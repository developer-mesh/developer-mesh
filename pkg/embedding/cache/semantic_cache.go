package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache/audit"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// SemanticCache implements similarity-based caching for embeddings.
// It provides intelligent caching of search results based on query similarity,
// using vector embeddings to find semantically similar queries.
//
// The cache uses Redis as the primary storage backend with circuit breaker
// protection and automatic fallback to in-memory storage during failures.
// It supports compression, vector similarity search, and comprehensive metrics.
//
// SemanticCache is safe for concurrent use by multiple goroutines.
type SemanticCache struct {
	redis        *ResilientRedisClient
	config       *Config
	normalizer   QueryNormalizer
	validator    *QueryValidator
	logger       observability.Logger
	safeLogger   *SafeLogger
	metrics      observability.MetricsClient
	mu           sync.RWMutex
	shutdownOnce sync.Once
	shuttingDown bool

	// Use sync.Map for concurrent access (project pattern)
	entries sync.Map // map[string]*CacheEntry

	// Atomic counters for stats
	hitCount  atomic.Int64
	missCount atomic.Int64

	// Compression and vector store support
	compressionService *CompressionService
	vectorStore        *VectorStore

	// Degraded mode support
	degradedModeCache *DegradedModeCache
	degradedMode      atomic.Bool
	recoveryStop      chan struct{}

	// Audit logging
	auditLogger *audit.Logger
}

// NewSemanticCache creates a new semantic cache instance with default configuration.
// It initializes the cache with Redis as the backend storage and sets up
// resilient operation with circuit breakers.
//
// Parameters:
//   - redisClient: Redis client for cache storage
//   - config: Cache configuration (uses defaults if nil)
//   - logger: Logger for observability (creates default if nil)
//
// Returns an error if the Redis client is nil or configuration is invalid.
func NewSemanticCache(
	redisClient *redis.Client,
	config *Config,
	logger observability.Logger,
) (*SemanticCache, error) {
	return NewSemanticCacheWithOptions(redisClient, config, logger, nil, "")
}

// NewSemanticCacheWithOptions creates a new semantic cache with vector store and compression
func NewSemanticCacheWithOptions(
	redisClient *redis.Client,
	config *Config,
	logger observability.Logger,
	vectorStore *VectorStore,
	encryptionKey string,
) (*SemanticCache, error) {
	if redisClient == nil {
		return nil, fmt.Errorf("redis client is required")
	}

	if config == nil {
		config = DefaultConfig()
	}

	// Validate config
	if config.SimilarityThreshold < 0 || config.SimilarityThreshold > 1 {
		return nil, fmt.Errorf("similarity threshold must be between 0 and 1")
	}
	if config.MaxCandidates <= 0 {
		config.MaxCandidates = 10
	}
	if config.TTL <= 0 {
		config.TTL = 24 * time.Hour
	}
	if config.Prefix == "" {
		config.Prefix = "semantic_cache"
	}

	if logger == nil {
		logger = observability.NewLogger("embedding.cache")
	}

	// Create safe logger for sensitive data handling
	safeLogger := NewSafeLogger(logger)

	// Create resilient Redis client
	var metrics observability.MetricsClient
	if config.EnableMetrics {
		metrics = observability.NewMetricsClient()
	}

	// Apply performance configuration
	if config.PerformanceConfig != nil {
		if err := config.PerformanceConfig.Validate(); err != nil {
			return nil, fmt.Errorf("invalid performance config: %w", err)
		}
		// Update config values from performance config
		config.MaxCandidates = config.PerformanceConfig.MaxCandidates
		config.SimilarityThreshold = config.PerformanceConfig.SimilarityThreshold
		config.EnableCompression = config.PerformanceConfig.CompressionThreshold > 0
	}

	resilientClient := NewResilientRedisClientWithConfig(redisClient, config.PerformanceConfig, logger, metrics)

	// Create compression service if encryption key provided
	var compressionService *CompressionService
	if encryptionKey != "" {
		compressionService = NewCompressionService(encryptionKey)
	}

	cache := &SemanticCache{
		redis:              resilientClient,
		config:             config,
		normalizer:         NewQueryNormalizer(),
		validator:          NewQueryValidator(),
		logger:             logger,
		safeLogger:         safeLogger,
		compressionService: compressionService,
		vectorStore:        vectorStore,
		recoveryStop:       make(chan struct{}),
	}

	if config.EnableMetrics {
		cache.metrics = observability.NewMetricsClient()
	}

	// Enable audit logging if configured
	if config.EnableAuditLogging {
		cache.auditLogger = audit.NewLogger(logger, true)
	}

	// Create degraded mode cache
	cache.degradedModeCache = NewDegradedModeCache(cache, logger)

	// Start recovery checker if degraded mode is supported
	if cache.degradedModeCache != nil {
		go func() {
			ticker := time.NewTicker(DefaultCheckPeriod)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					cache.checkAndRecoverFromDegradedMode(context.Background())
				case <-cache.recoveryStop:
					return
				}
			}
		}()
	}

	return cache, nil
}

// Get retrieves cached results for a query using exact match or similarity search.
// It first attempts an exact match on the normalized query, then falls back to
// vector similarity search if an embedding is provided.
//
// The method automatically handles degraded mode operation when Redis is unavailable,
// falling back to an in-memory cache with reduced capacity.
//
// Parameters:
//   - ctx: Context for cancellation and tracing
//   - query: The search query to look up
//   - queryEmbedding: Optional embedding vector for similarity search
//
// Returns:
//   - *CacheEntry: The cached entry if found, nil if not found
//   - error: Always nil (errors are logged but don't fail the operation)
//
// Cache hits update access statistics and LRU tracking for eviction.
func (c *SemanticCache) Get(ctx context.Context, query string, queryEmbedding []float32) (*CacheEntry, error) {
	// Check if shutting down
	if c.IsShuttingDown() {
		return nil, nil
	}

	// Use degraded mode if active - but only if not already in a degraded mode call
	// Check context to prevent infinite recursion
	if _, fromDegraded := ctx.Value(contextKeyFromDegraded).(bool); !fromDegraded {
		if c.degradedMode.Load() && c.degradedModeCache != nil {
			return c.degradedModeCache.Get(ctx, query, queryEmbedding)
		}
	}

	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "semantic_cache.get")
	defer span.End()

	// Track operation start time for audit
	start := time.Now()
	var auditErr error
	defer func() {
		if c.auditLogger != nil {
			c.auditLogger.LogOperation(ctx, audit.EventCacheGet, "get", query, start, auditErr)
		}
	}()

	// Validate query
	if err := c.validator.Validate(query); err != nil {
		c.recordMiss(ctx, "invalid_query")
		auditErr = err
		return nil, nil // Don't fail on validation errors, just miss
	}

	// Sanitize query
	query = c.validator.Sanitize(query)

	// Normalize query
	normalized := c.normalizer.Normalize(query)
	span.SetAttribute("normalized_query", normalized)

	// If normalized query is empty, return nil
	if normalized == "" {
		c.recordMiss(ctx, "empty_normalized")
		return nil, nil
	}

	// Try exact match first
	key := c.getCacheKey(normalized)
	entry, err := c.getExactMatch(ctx, normalized)
	if err == nil && entry != nil {
		c.recordHit(ctx, "exact")
		updatedEntry, updateErr := c.updateAccessStats(ctx, key, entry)
		if updateErr != nil {
			c.logger.Warn("Failed to update access stats", map[string]interface{}{
				"error": updateErr.Error(),
			})
			return entry, nil
		}
		return updatedEntry, nil
	}

	// If no embedding provided, this is a cache miss
	if len(queryEmbedding) == 0 {
		c.recordMiss(ctx, "no_embedding")
		return nil, nil
	}

	// Search for similar cached queries
	candidates, err := c.searchSimilarQueries(ctx, queryEmbedding, c.config.MaxCandidates)
	if err != nil {
		c.logger.Error("Failed to search similar queries", map[string]interface{}{
			"error": err.Error(),
			"query": query,
		})
		c.recordMiss(ctx, "search_error")
		return nil, nil // Don't fail on cache errors
	}

	// Find best match above threshold
	for _, candidate := range candidates {
		if candidate.Similarity >= c.config.SimilarityThreshold {
			entry, err := c.getCacheEntry(ctx, candidate.CacheKey)
			if err == nil && entry != nil {
				c.recordHit(ctx, "similarity")
				updatedEntry, updateErr := c.updateAccessStats(ctx, candidate.CacheKey, entry)
				if updateErr != nil {
					c.logger.Warn("Failed to update access stats", map[string]interface{}{
						"error": updateErr.Error(),
					})
					return entry, nil
				}
				return updatedEntry, nil
			}
		}
	}

	c.recordMiss(ctx, "no_match")
	return nil, nil
}

// Set stores query results in cache with the associated embedding.
// The entry is stored with a TTL and can be retrieved later using exact
// match or similarity search.
//
// The method automatically compresses large entries and handles degraded
// mode operation by falling back to in-memory storage when Redis fails.
//
// Parameters:
//   - ctx: Context for cancellation and tracing
//   - query: The search query to cache
//   - queryEmbedding: Optional embedding vector for similarity matching
//   - results: The search results to cache
//
// Returns an error if:
//   - The cache is shutting down
//   - Query validation fails
//   - Marshaling fails
//
// Note: Redis storage failures trigger degraded mode but don't return errors.
func (c *SemanticCache) Set(ctx context.Context, query string, queryEmbedding []float32, results []CachedSearchResult) error {
	// Check if shutting down
	if c.IsShuttingDown() {
		return fmt.Errorf("cache is shutting down")
	}

	// Use degraded mode if active - but only if not already in a degraded mode call
	// Check context to prevent infinite recursion
	if _, fromDegraded := ctx.Value(contextKeyFromDegraded).(bool); !fromDegraded {
		if c.degradedMode.Load() && c.degradedModeCache != nil {
			return c.degradedModeCache.Set(ctx, query, queryEmbedding, results)
		}
	}

	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "semantic_cache.set")
	defer span.End()

	// Track operation start time for audit
	start := time.Now()
	var auditErr error
	defer func() {
		if c.auditLogger != nil {
			c.auditLogger.LogOperation(ctx, audit.EventCacheSet, "set", query, start, auditErr)
		}
	}()

	// Validate query
	if err := c.validator.Validate(query); err != nil {
		auditErr = err
		return fmt.Errorf("invalid query: %w", err)
	}

	// Sanitize query
	query = c.validator.Sanitize(query)

	normalized := c.normalizer.Normalize(query)

	// Don't cache empty normalized queries
	if normalized == "" {
		return nil
	}

	entry := &CacheEntry{
		Query:           query,
		NormalizedQuery: normalized,
		Embedding:       queryEmbedding,
		Results:         results,
		CachedAt:        time.Now(),
		HitCount:        0,
		LastAccessedAt:  time.Now(),
		TTL:             c.config.TTL,
		Metadata: map[string]interface{}{
			"result_count":  len(results),
			"has_embedding": len(queryEmbedding) > 0,
		},
	}

	// Store in Redis
	key := c.getCacheKey(normalized)
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	// Use compression if enabled and data is large
	if c.config.EnableCompression && len(data) > 1024 {
		data, err = c.compress(data)
		if err != nil {
			c.logger.Warn("Failed to compress cache entry", map[string]interface{}{
				"error": err.Error(),
				"size":  len(data),
			})
			// Continue without compression
		}
	}

	err = c.redis.Set(ctx, key, data, c.config.TTL)
	if err != nil {
		// Enter degraded mode on Redis errors
		c.enterDegradedMode("Redis SET failed", err)

		// If we have a degraded mode cache, use it as fallback
		if c.degradedModeCache != nil {
			// Retry with degraded mode
			return c.degradedModeCache.Set(ctx, query, queryEmbedding, results)
		}

		return fmt.Errorf("failed to store in Redis: %w", err)
	}

	// Store embedding for similarity search (if provided)
	if len(queryEmbedding) > 0 {
		err = c.storeCacheEmbedding(ctx, normalized, queryEmbedding, key)
		if err != nil {
			// Log error but don't fail - exact match will still work
			c.safeLogger.Warn("Failed to store embedding for similarity search", map[string]interface{}{
				"error": err.Error(),
				"query": query,
			})
		}
	}

	// Check cache size and evict if necessary
	go c.evictIfNecessary(context.Background())

	return nil
}

// Delete removes a specific query from cache.
// It removes both the cache entry and any associated vector embeddings
// used for similarity search.
//
// Parameters:
//   - ctx: Context for cancellation
//   - query: The query to remove from cache
//
// Returns an error if the Redis deletion fails.
func (c *SemanticCache) Delete(ctx context.Context, query string) error {
	// Audit logging
	start := time.Now()
	var auditErr error
	defer func() {
		if c.auditLogger != nil {
			c.auditLogger.LogOperation(ctx, audit.EventCacheDelete, "delete", query, start, auditErr)
		}
	}()

	normalized := c.normalizer.Normalize(query)
	key := c.getCacheKey(normalized)

	// Delete from Redis
	err := c.redis.Del(ctx, key)
	if err != nil {
		// Enter degraded mode on Redis errors
		c.enterDegradedMode("Redis DEL failed", err)
		auditErr = err
		return fmt.Errorf("failed to delete from Redis: %w", err)
	}

	// Delete from similarity index
	err = c.deleteCacheEmbedding(ctx, normalized)
	if err != nil {
		c.logger.Warn("Failed to delete embedding", map[string]interface{}{
			"error": err.Error(),
			"query": query,
		})
	}

	return nil
}

// Clear removes all entries from the cache.
// It uses Redis SCAN to avoid blocking the server while deleting large
// numbers of keys. Entries are deleted in batches of 1000.
//
// This operation also clears the similarity index and resets all statistics.
//
// Parameters:
//   - ctx: Context for cancellation
//
// Returns an error if the scan or deletion operations fail.
func (c *SemanticCache) Clear(ctx context.Context) error {
	// Audit logging for compliance
	start := time.Now()
	var auditErr error
	var deletedCount int
	defer func() {
		if c.auditLogger != nil {
			metadata := map[string]interface{}{
				"entries_deleted": deletedCount,
			}
			c.auditLogger.LogOperation(ctx, audit.EventCacheClear, "clear", "all", start, auditErr)
			if deletedCount > 0 {
				c.auditLogger.LogSecurityEvent(ctx, audit.EventCacheClear, "cache", metadata)
			}
		}
	}()

	pattern := fmt.Sprintf("%s:*", c.config.Prefix)

	// Use SCAN to avoid blocking Redis
	iter := c.redis.Scan(ctx, 0, pattern, 100).Iterator()
	var keys []string

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())

		// Delete in batches
		if len(keys) >= 1000 {
			if err := c.redis.Del(ctx, keys...); err != nil {
				auditErr = err
				return fmt.Errorf("failed to delete batch: %w", err)
			}
			deletedCount += len(keys)
			keys = keys[:0]
		}
	}

	// Delete remaining keys
	if len(keys) > 0 {
		if err := c.redis.Del(ctx, keys...); err != nil {
			auditErr = err
			return fmt.Errorf("failed to delete final batch: %w", err)
		}
		deletedCount += len(keys)
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("scan error: %w", err)
	}

	// Clear similarity index
	if err := c.clearSimilarityIndex(ctx); err != nil {
		c.logger.Warn("Failed to clear similarity index", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Reset stats atomically
	c.hitCount.Store(0)
	c.missCount.Store(0)

	// Clear local cache
	c.entries.Range(func(key, value interface{}) bool {
		c.entries.Delete(key)
		return true
	})

	return nil
}

// Helper methods

func (c *SemanticCache) getCacheKey(normalized string) string {
	return fmt.Sprintf("%s:query:%s", c.config.Prefix, SanitizeRedisKey(normalized))
}

func (c *SemanticCache) getExactMatch(ctx context.Context, normalized string) (*CacheEntry, error) {
	key := c.getCacheKey(normalized)
	data, err := c.redis.Get(ctx, key)
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		// Enter degraded mode on Redis errors
		c.enterDegradedMode("Redis GET failed", err)
		return nil, err
	}

	dataBytes := []byte(data)

	// Decompress if needed
	if c.config.EnableCompression && c.isCompressed(dataBytes) {
		dataBytes, err = c.decompress(dataBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress: %w", err)
		}
	}

	var entry CacheEntry
	if err := json.Unmarshal(dataBytes, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	return &entry, nil
}

func (c *SemanticCache) getCacheEntry(ctx context.Context, key string) (*CacheEntry, error) {
	data, err := c.redis.Get(ctx, key)
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		// Enter degraded mode on Redis errors
		c.enterDegradedMode("Redis GET failed", err)
		return nil, err
	}

	dataBytes := []byte(data)

	// Decompress if needed
	if c.config.EnableCompression && c.isCompressed(dataBytes) {
		dataBytes, err = c.decompress(dataBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress: %w", err)
		}
	}

	var entry CacheEntry
	if err := json.Unmarshal(dataBytes, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	return &entry, nil
}

// Thread-safe update using copy-on-write pattern
func (c *SemanticCache) updateAccessStats(ctx context.Context, key string, entry *CacheEntry) (*CacheEntry, error) {
	// Create a copy to avoid race conditions
	updatedEntry := &CacheEntry{
		Query:           entry.Query,
		NormalizedQuery: entry.NormalizedQuery,
		Embedding:       entry.Embedding,
		Results:         entry.Results,
		CachedAt:        entry.CachedAt,
		LastAccessedAt:  time.Now(),
		HitCount:        entry.HitCount + 1,
		TTL:             entry.TTL,
		Metadata:        entry.Metadata,
	}

	// Update in Redis atomically
	data, err := json.Marshal(updatedEntry)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal entry: %w", err)
	}

	// Compress if needed
	if c.config.EnableCompression && len(data) > 1024 {
		data, err = c.compress(data)
		if err != nil {
			return nil, fmt.Errorf("failed to compress: %w", err)
		}
	}

	// Use resilient client with retry
	err = c.redis.Set(ctx, key, data, entry.TTL)
	if err != nil {
		// Enter degraded mode on Redis errors
		c.enterDegradedMode("Redis SET failed during stats update", err)
		return nil, fmt.Errorf("failed to update access stats: %w", err)
	}

	// Update local cache
	c.entries.Store(key, updatedEntry)

	return updatedEntry, nil
}

func (c *SemanticCache) recordHit(ctx context.Context, hitType string) {
	c.hitCount.Add(1)

	if c.metrics != nil {
		c.metrics.IncrementCounterWithLabels("semantic_cache.hit", 1, map[string]string{
			"type": hitType,
		})
	}
}

func (c *SemanticCache) recordMiss(ctx context.Context, missType string) {
	c.missCount.Add(1)

	if c.metrics != nil {
		c.metrics.IncrementCounterWithLabels("semantic_cache.miss", 1, map[string]string{
			"type": missType,
		})
	}
}

// Compression helpers use the existing CompressionService
func (c *SemanticCache) compress(data []byte) ([]byte, error) {
	// Use the existing compression service for data > 1KB
	if c.compressionService != nil {
		return c.compressionService.CompressOnly(data)
	}
	return data, nil
}

func (c *SemanticCache) decompress(data []byte) ([]byte, error) {
	// Use the existing compression service
	if c.compressionService != nil {
		return c.compressionService.DecompressOnly(data)
	}
	return data, nil
}

func (c *SemanticCache) isCompressed(data []byte) bool {
	// Check for gzip magic bytes: 0x1f, 0x8b
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}

// searchSimilarQueries uses the vector store to find similar cached queries
func (c *SemanticCache) searchSimilarQueries(ctx context.Context, embedding []float32, limit int) ([]SimilarQuery, error) {
	if c.vectorStore == nil {
		return []SimilarQuery{}, nil // No vector store configured
	}

	// Use tenant ID from context (required for all operations)
	tenantID := auth.GetTenantID(ctx)
	if tenantID == uuid.Nil {
		return nil, ErrNoTenantID
	}

	// Call vectorStore.FindSimilarQueries with threshold 0.8
	results, err := c.vectorStore.FindSimilarQueries(ctx, tenantID, embedding, 0.8, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search similar queries: %w", err)
	}

	// Convert SimilarQueryResult to SimilarQuery
	similarQueries := make([]SimilarQuery, 0, len(results))
	for _, result := range results {
		similarQueries = append(similarQueries, SimilarQuery{
			Query:      result.QueryHash, // This is the normalized query
			CacheKey:   result.CacheKey,
			Similarity: result.Similarity,
		})
	}

	return similarQueries, nil
}

func (c *SemanticCache) storeCacheEmbedding(ctx context.Context, query string, embedding []float32, cacheKey string) error {
	if c.vectorStore == nil {
		return nil // No vector store configured
	}

	tenantID := auth.GetTenantID(ctx)
	if tenantID == uuid.Nil {
		return ErrNoTenantID
	}

	queryHash := c.normalizer.Normalize(query) // Use existing normalizer

	return c.vectorStore.StoreCacheEmbedding(ctx, tenantID, cacheKey, queryHash, embedding)
}

func (c *SemanticCache) deleteCacheEmbedding(ctx context.Context, query string) error {
	if c.vectorStore == nil {
		return nil // No vector store configured
	}

	tenantID := auth.GetTenantID(ctx)
	if tenantID == uuid.Nil {
		return ErrNoTenantID
	}

	// Generate the cache key to delete from vector store
	normalized := c.normalizer.Normalize(query)
	cacheKey := c.getCacheKey(normalized)

	// Delete from vector store
	return c.vectorStore.DeleteCacheEntry(ctx, tenantID, cacheKey)
}

func (c *SemanticCache) clearSimilarityIndex(ctx context.Context) error {
	if c.vectorStore == nil {
		return nil // No vector store configured
	}

	tenantID := auth.GetTenantID(ctx)
	if tenantID == uuid.Nil {
		return ErrNoTenantID
	}

	// Clear all embeddings for this tenant - need to implement this
	// For now, return nil as this is not a critical operation
	return nil
}

// Eviction helper
func (c *SemanticCache) evictIfNecessary(ctx context.Context) {
	RecoverMiddleware(c.logger, "evict_if_necessary")(func() {
		if c.config.MaxCacheSize <= 0 {
			return
		}

		// Count entries
		pattern := fmt.Sprintf("%s:query:*", c.config.Prefix)
		countInterface, err := c.redis.Eval(ctx, `
		local count = 0
		local cursor = "0"
		repeat
			local result = redis.call("SCAN", cursor, "MATCH", ARGV[1], "COUNT", 100)
			cursor = result[1]
			count = count + #result[2]
		until cursor == "0"
		return count
	`, []string{}, pattern)

		var count int
		if err == nil {
			switch v := countInterface.(type) {
			case int64:
				count = int(v)
			case int:
				count = v
			default:
				count = 0
			}
		}

		if err != nil {
			c.logger.Error("Failed to count cache entries", map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		if count <= c.config.MaxCacheSize {
			return
		}

		// LRU eviction is now handled by the LRU manager in tenant_cache.go
		// The eviction runs asynchronously via StartLRUEviction()
		c.logger.Warn("Cache size exceeded, eviction handled by LRU manager", map[string]interface{}{
			"current_size": count,
			"max_size":     c.config.MaxCacheSize,
		})
	})
}

// Shutdown gracefully shuts down the cache.
// It stops background operations, flushes metrics, and closes the Redis connection.
// This method should be called when the cache is no longer needed.
//
// Shutdown is idempotent and can be called multiple times safely.
//
// Parameters:
//   - ctx: Context for shutdown operations
//
// Returns an error if closing the Redis connection fails.
func (c *SemanticCache) Shutdown(ctx context.Context) error {
	var err error
	c.shutdownOnce.Do(func() {
		c.logger.Info("Shutting down semantic cache", map[string]interface{}{})

		// Mark as shutting down
		c.mu.Lock()
		c.shuttingDown = true
		c.mu.Unlock()

		// Stop recovery checker
		if c.recoveryStop != nil {
			close(c.recoveryStop)
		}

		// Flush any pending metrics
		if c.metrics != nil {
			// Most metrics clients have a Flush or Close method
			// Since we don't have the exact interface, we'll log instead
			c.logger.Info("Flushing metrics", map[string]interface{}{})
		}

		// Close Redis connection
		if c.redis != nil {
			if closeErr := c.redis.Close(); closeErr != nil {
				err = fmt.Errorf("error closing Redis connection: %w", closeErr)
			}
		}

		c.logger.Info("Semantic cache shutdown complete", map[string]interface{}{})
	})

	return err
}

// IsShuttingDown returns true if the cache is shutting down
func (c *SemanticCache) IsShuttingDown() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.shuttingDown
}

// GetBatch retrieves multiple entries from cache in a single operation.
// Each query is processed independently, and errors for individual queries
// are logged but don't fail the entire batch.
//
// Parameters:
//   - ctx: Context for cancellation
//   - queries: List of queries to look up
//   - embeddings: Optional embeddings for each query (can be nil or shorter than queries)
//
// Returns:
//   - []*CacheEntry: Results for each query (nil for cache misses)
//   - error: Always nil
func (c *SemanticCache) GetBatch(ctx context.Context, queries []string, embeddings [][]float32) ([]*CacheEntry, error) {
	results := make([]*CacheEntry, len(queries))

	// Process each query
	for i, query := range queries {
		var embedding []float32
		if i < len(embeddings) {
			embedding = embeddings[i]
		}

		entry, err := c.Get(ctx, query, embedding)
		if err != nil {
			// Log error but continue with other queries
			c.logger.Warn("Failed to get cache entry in batch", map[string]interface{}{
				"query": query,
				"error": err.Error(),
			})
		}
		results[i] = entry
	}

	return results, nil
}

// GetStats returns current cache statistics.
// It provides metrics on cache performance including hit rate,
// total operations, and current entry count.
//
// The statistics are calculated from atomic counters and are
// safe to call concurrently.
//
// Returns a CacheStats struct with current metrics.
func (c *SemanticCache) GetStats() *CacheStats {
	hits := c.hitCount.Load()
	misses := c.missCount.Load()
	total := hits + misses

	stats := &CacheStats{
		TotalHits:   int(hits),
		TotalMisses: int(misses),
		HitRate:     0.0,
		Timestamp:   time.Now(),
	}

	if total > 0 {
		stats.HitRate = float64(hits) / float64(total)
	}

	// Count entries in local cache
	entryCount := 0
	c.entries.Range(func(key, value interface{}) bool {
		entryCount++
		return true
	})
	stats.TotalEntries = entryCount

	return stats
}

// enterDegradedMode switches the cache to degraded mode
func (c *SemanticCache) enterDegradedMode(reason string, err error) {
	// Only log once per degraded mode entry
	if !c.degradedMode.Swap(true) {
		c.logger.Error("Entering degraded mode", map[string]interface{}{
			"reason": reason,
			"error":  err.Error(),
		})

		// Record metric
		if c.metrics != nil {
			c.metrics.IncrementCounterWithLabels("semantic_cache.degraded_mode", 1, map[string]string{
				"reason": reason,
			})
		}

		// Audit log system event
		if c.auditLogger != nil {
			c.auditLogger.LogSystemEvent(audit.EventDegradedMode, reason, map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
}

// exitDegradedMode switches the cache back to normal mode
func (c *SemanticCache) exitDegradedMode() {
	if c.degradedMode.Swap(false) {
		c.logger.Info("Exiting degraded mode - Redis connection restored", nil)

		// Record metric
		if c.metrics != nil {
			c.metrics.IncrementCounterWithLabels("semantic_cache.degraded_mode_exit", 1, nil)
		}

		// Audit log recovery event
		if c.auditLogger != nil {
			c.auditLogger.LogSystemEvent(audit.EventRecovery, "Redis connection restored", nil)
		}
	}
}

// checkAndRecoverFromDegradedMode periodically checks if Redis is healthy again
func (c *SemanticCache) checkAndRecoverFromDegradedMode(ctx context.Context) {
	if !c.degradedMode.Load() {
		return
	}

	// Try a simple Redis operation
	testCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	err := c.redis.Health(testCtx)
	if err == nil {
		c.exitDegradedMode()
	}
}

// marshalEntry marshals a cache entry with optional compression
func (c *SemanticCache) marshalEntry(entry *CacheEntry) ([]byte, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	// Use compression if enabled and data is large
	if c.config.EnableCompression && len(data) > 1024 {
		compressed, err := c.compress(data)
		if err != nil {
			c.logger.Warn("Failed to compress cache entry", map[string]interface{}{
				"error": err.Error(),
				"size":  len(data),
			})
			// Return uncompressed data on compression failure
			return data, nil
		}
		return compressed, nil
	}

	return data, nil
}

// unmarshalEntry unmarshals a cache entry with optional decompression
func (c *SemanticCache) unmarshalEntry(data []byte) (*CacheEntry, error) {
	// Decompress if needed
	if c.config.EnableCompression && c.isCompressed(data) {
		decompressed, err := c.decompress(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress: %w", err)
		}
		data = decompressed
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	return &entry, nil
}
