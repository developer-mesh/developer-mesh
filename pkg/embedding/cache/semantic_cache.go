package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// SemanticCache implements similarity-based caching for embeddings
type SemanticCache struct {
	redis        *redis.Client
	config       *Config
	normalizer   QueryNormalizer
	logger       observability.Logger
	metrics      observability.MetricsClient
	mu           sync.RWMutex
	
	// Cache statistics
	stats struct {
		hits   int64
		misses int64
	}
}

// NewSemanticCache creates a new semantic cache instance
func NewSemanticCache(
	redisClient *redis.Client,
	config *Config,
	logger observability.Logger,
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
	
	cache := &SemanticCache{
		redis:      redisClient,
		config:     config,
		normalizer: NewQueryNormalizer(),
		logger:     logger,
	}
	
	if config.EnableMetrics {
		cache.metrics = observability.NewMetricsClient()
	}
	
	return cache, nil
}

// Get retrieves cached results for a query
func (c *SemanticCache) Get(ctx context.Context, query string, queryEmbedding []float32) (*CacheEntry, error) {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "semantic_cache.get")
	defer span.End()
	
	// Normalize query
	normalized := c.normalizer.Normalize(query)
	span.SetAttribute("normalized_query", normalized)
	
	// If normalized query is empty, return nil
	if normalized == "" {
		c.recordMiss(ctx, "empty_normalized")
		return nil, nil
	}
	
	// Try exact match first
	entry, err := c.getExactMatch(ctx, normalized)
	if err == nil && entry != nil {
		c.recordHit(ctx, "exact")
		c.updateAccessStats(ctx, entry)
		return entry, nil
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
				c.updateAccessStats(ctx, entry)
				return entry, nil
			}
		}
	}
	
	c.recordMiss(ctx, "no_match")
	return nil, nil
}

// Set stores query results in cache
func (c *SemanticCache) Set(ctx context.Context, query string, queryEmbedding []float32, results []CachedSearchResult) error {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "semantic_cache.set")
	defer span.End()
	
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
			"result_count": len(results),
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
	
	err = c.redis.Set(ctx, key, data, c.config.TTL).Err()
	if err != nil {
		return fmt.Errorf("failed to store in Redis: %w", err)
	}
	
	// Store embedding for similarity search (if provided)
	if len(queryEmbedding) > 0 {
		err = c.storeCacheEmbedding(ctx, normalized, queryEmbedding, key)
		if err != nil {
			// Log error but don't fail - exact match will still work
			c.logger.Warn("Failed to store embedding for similarity search", map[string]interface{}{
				"error": err.Error(),
				"query": query,
			})
		}
	}
	
	// Check cache size and evict if necessary
	go c.evictIfNecessary(context.Background())
	
	return nil
}

// Delete removes a specific query from cache
func (c *SemanticCache) Delete(ctx context.Context, query string) error {
	normalized := c.normalizer.Normalize(query)
	key := c.getCacheKey(normalized)
	
	// Delete from Redis
	err := c.redis.Del(ctx, key).Err()
	if err != nil {
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

// Clear removes all entries from the cache
func (c *SemanticCache) Clear(ctx context.Context) error {
	pattern := fmt.Sprintf("%s:*", c.config.Prefix)
	
	// Use SCAN to avoid blocking Redis
	iter := c.redis.Scan(ctx, 0, pattern, 100).Iterator()
	var keys []string
	
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		
		// Delete in batches
		if len(keys) >= 1000 {
			if err := c.redis.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("failed to delete batch: %w", err)
			}
			keys = keys[:0]
		}
	}
	
	// Delete remaining keys
	if len(keys) > 0 {
		if err := c.redis.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("failed to delete final batch: %w", err)
		}
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
	
	// Reset stats
	c.mu.Lock()
	c.stats.hits = 0
	c.stats.misses = 0
	c.mu.Unlock()
	
	return nil
}

// Helper methods

func (c *SemanticCache) getCacheKey(normalized string) string {
	return fmt.Sprintf("%s:query:%s", c.config.Prefix, normalized)
}

func (c *SemanticCache) getExactMatch(ctx context.Context, normalized string) (*CacheEntry, error) {
	key := c.getCacheKey(normalized)
	data, err := c.redis.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	
	// Decompress if needed
	if c.config.EnableCompression && c.isCompressed(data) {
		data, err = c.decompress(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress: %w", err)
		}
	}
	
	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}
	
	return &entry, nil
}

func (c *SemanticCache) getCacheEntry(ctx context.Context, key string) (*CacheEntry, error) {
	data, err := c.redis.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	
	// Decompress if needed
	if c.config.EnableCompression && c.isCompressed(data) {
		data, err = c.decompress(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress: %w", err)
		}
	}
	
	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}
	
	return &entry, nil
}

func (c *SemanticCache) updateAccessStats(ctx context.Context, entry *CacheEntry) {
	entry.HitCount++
	entry.LastAccessedAt = time.Now()
	
	// Update in Redis
	key := c.getCacheKey(entry.NormalizedQuery)
	data, err := json.Marshal(entry)
	if err != nil {
		c.logger.Error("Failed to marshal updated entry", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	// Compress if needed
	if c.config.EnableCompression && len(data) > 1024 {
		data, _ = c.compress(data)
	}
	
	// Reset TTL on access
	err = c.redis.Set(ctx, key, data, c.config.TTL).Err()
	if err != nil {
		c.logger.Error("Failed to update access stats", map[string]interface{}{
			"error": err.Error(),
		})
	}
}

func (c *SemanticCache) recordHit(ctx context.Context, hitType string) {
	c.mu.Lock()
	c.stats.hits++
	c.mu.Unlock()
	
	if c.metrics != nil {
		c.metrics.IncrementCounterWithLabels("semantic_cache.hit", 1, map[string]string{
			"type": hitType,
		})
	}
}

func (c *SemanticCache) recordMiss(ctx context.Context, missType string) {
	c.mu.Lock()
	c.stats.misses++
	c.mu.Unlock()
	
	if c.metrics != nil {
		c.metrics.IncrementCounterWithLabels("semantic_cache.miss", 1, map[string]string{
			"type": missType,
		})
	}
}

// Compression helpers (stub implementations)
func (c *SemanticCache) compress(data []byte) ([]byte, error) {
	// TODO: Implement compression (e.g., using gzip)
	return data, nil
}

func (c *SemanticCache) decompress(data []byte) ([]byte, error) {
	// TODO: Implement decompression
	return data, nil
}

func (c *SemanticCache) isCompressed(data []byte) bool {
	// TODO: Check compression header
	return false
}

// Similarity search stubs (to be implemented with vector DB)
func (c *SemanticCache) searchSimilarQueries(ctx context.Context, embedding []float32, limit int) ([]SimilarQuery, error) {
	// TODO: Implement similarity search using vector DB or Redis vector search
	// For now, return empty results
	return []SimilarQuery{}, nil
}

func (c *SemanticCache) storeCacheEmbedding(ctx context.Context, query string, embedding []float32, cacheKey string) error {
	// TODO: Store embedding in vector index
	return nil
}

func (c *SemanticCache) deleteCacheEmbedding(ctx context.Context, query string) error {
	// TODO: Delete from vector index
	return nil
}

func (c *SemanticCache) clearSimilarityIndex(ctx context.Context) error {
	// TODO: Clear vector index
	return nil
}

// Eviction helper
func (c *SemanticCache) evictIfNecessary(ctx context.Context) {
	if c.config.MaxCacheSize <= 0 {
		return
	}
	
	// Count entries
	pattern := fmt.Sprintf("%s:query:*", c.config.Prefix)
	count, err := c.redis.Eval(ctx, `
		local count = 0
		local cursor = "0"
		repeat
			local result = redis.call("SCAN", cursor, "MATCH", ARGV[1], "COUNT", 100)
			cursor = result[1]
			count = count + #result[2]
		until cursor == "0"
		return count
	`, []string{}, pattern).Int()
	
	if err != nil {
		c.logger.Error("Failed to count cache entries", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	if count <= c.config.MaxCacheSize {
		return
	}
	
	// TODO: Implement LRU eviction
	c.logger.Warn("Cache size exceeded, eviction needed", map[string]interface{}{
		"current_size": count,
		"max_size":     c.config.MaxCacheSize,
	})
}