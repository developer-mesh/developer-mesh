package cache

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/redis/go-redis/v9"
)

const (
	// CompressionThreshold defines minimum size for compression (1KB)
	CompressionThreshold = 1024

	// DefaultL1MaxItems defines default L1 cache size (100MB / ~10KB per item)
	DefaultL1MaxItems = 10000

	// DefaultL1TTL defines default L1 cache TTL
	DefaultL1TTL = 5 * time.Minute

	// DefaultL2TTL defines default L2 Redis TTL
	DefaultL2TTL = 1 * time.Hour

	// RedisConnectTimeout defines connection timeout for Redis
	RedisConnectTimeout = 5 * time.Second

	// RedisOperationTimeout defines operation timeout for Redis
	RedisOperationTimeout = 2 * time.Second
)

// TieredCacheConfig defines configuration for two-tier caching
type TieredCacheConfig struct {
	// L1 Memory Cache (always enabled)
	L1MaxItems int
	L1TTL      time.Duration

	// L2 Redis Cache (optional)
	RedisEnabled        bool
	RedisURL            string
	RedisConnectTimeout time.Duration
	RedisFallbackMode   bool // Continue without Redis if unavailable
	L2TTL               time.Duration

	// Compression
	EnableCompression    bool
	CompressionThreshold int

	// Logger
	Logger observability.Logger
}

// TieredCache implements a two-tier caching strategy:
// - L1: In-memory cache (always enabled, fast access)
// - L2: Redis cache (optional, distributed)
type TieredCache struct {
	// L1 cache (always present)
	l1 Cache

	// L2 cache (optional)
	redis        *redis.Client
	redisEnabled bool

	// Configuration
	config *TieredCacheConfig
	logger observability.Logger

	// Statistics
	stats *CacheStats

	// State
	mu              sync.RWMutex
	redisHealthy    bool
	lastHealthCheck time.Time
}

// CacheStats tracks cache performance metrics
type CacheStats struct {
	// L1 stats
	l1Hits   atomic.Int64
	l1Misses atomic.Int64

	// L2 stats
	l2Hits   atomic.Int64
	l2Misses atomic.Int64

	// Overall stats
	totalRequests atomic.Int64
	totalErrors   atomic.Int64

	// Compression stats
	compressionSaved atomic.Int64
	compressionCount atomic.Int64
}

// NewTieredCache creates a new two-tier cache
func NewTieredCache(config *TieredCacheConfig) (*TieredCache, error) {
	if config == nil {
		config = DefaultTieredCacheConfig()
	}

	// Apply defaults
	if config.L1MaxItems == 0 {
		config.L1MaxItems = DefaultL1MaxItems
	}
	if config.L1TTL == 0 {
		config.L1TTL = DefaultL1TTL
	}
	if config.L2TTL == 0 {
		config.L2TTL = DefaultL2TTL
	}
	if config.RedisConnectTimeout == 0 {
		config.RedisConnectTimeout = RedisConnectTimeout
	}
	if config.CompressionThreshold == 0 {
		config.CompressionThreshold = CompressionThreshold
	}

	// Create L1 memory cache (always enabled)
	l1 := NewMemoryCache(config.L1MaxItems, config.L1TTL)

	tc := &TieredCache{
		l1:     l1,
		config: config,
		logger: config.Logger,
		stats:  &CacheStats{},
	}

	// Initialize Redis if enabled
	if config.RedisEnabled && config.RedisURL != "" {
		if err := tc.initRedis(); err != nil {
			if config.RedisFallbackMode {
				tc.logger.Warn("Redis initialization failed, falling back to memory-only mode", map[string]interface{}{
					"error": err.Error(),
				})
			} else {
				return nil, fmt.Errorf("redis initialization failed: %w", err)
			}
		}
	}

	return tc, nil
}

// DefaultTieredCacheConfig returns default configuration
func DefaultTieredCacheConfig() *TieredCacheConfig {
	return &TieredCacheConfig{
		L1MaxItems:           DefaultL1MaxItems,
		L1TTL:                DefaultL1TTL,
		RedisEnabled:         false,
		RedisURL:             "",
		RedisConnectTimeout:  RedisConnectTimeout,
		RedisFallbackMode:    true,
		L2TTL:                DefaultL2TTL,
		EnableCompression:    true,
		CompressionThreshold: CompressionThreshold,
		Logger:               observability.NewNoopLogger(),
	}
}

// initRedis initializes Redis connection with timeout
func (tc *TieredCache) initRedis() error {
	opt, err := redis.ParseURL(tc.config.RedisURL)
	if err != nil {
		return fmt.Errorf("invalid Redis URL: %w", err)
	}

	// Set timeouts
	opt.DialTimeout = tc.config.RedisConnectTimeout
	opt.ReadTimeout = RedisOperationTimeout
	opt.WriteTimeout = RedisOperationTimeout

	// Create client
	tc.redis = redis.NewClient(opt)

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), tc.config.RedisConnectTimeout)
	defer cancel()

	if err := tc.redis.Ping(ctx).Err(); err != nil {
		_ = tc.redis.Close()
		tc.redis = nil
		return fmt.Errorf("redis ping failed: %w", err)
	}

	tc.redisEnabled = true
	tc.redisHealthy = true
	tc.lastHealthCheck = time.Now()

	tc.logger.Info("Redis cache initialized successfully", map[string]interface{}{
		"url": tc.config.RedisURL,
	})

	return nil
}

// Get retrieves a value from the cache (tries L1 first, then L2)
func (tc *TieredCache) Get(ctx context.Context, key string, value interface{}) error {
	tc.stats.totalRequests.Add(1)

	// Try L1 cache first
	if err := tc.l1.Get(ctx, key, value); err == nil {
		tc.stats.l1Hits.Add(1)
		return nil
	}
	tc.stats.l1Misses.Add(1)

	// Try L2 (Redis) if enabled and healthy
	if !tc.isRedisAvailable() {
		return fmt.Errorf("key not found: %s", key)
	}

	// Get from Redis
	redisKey := tc.makeRedisKey(key)
	ctx, cancel := context.WithTimeout(ctx, RedisOperationTimeout)
	defer cancel()

	data, err := tc.redis.Get(ctx, redisKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			tc.stats.l2Misses.Add(1)
			return fmt.Errorf("key not found: %s", key)
		}

		// Redis error - mark as unhealthy if repeated failures
		tc.handleRedisError(err)
		tc.stats.totalErrors.Add(1)
		return fmt.Errorf("key not found: %s", key)
	}

	tc.stats.l2Hits.Add(1)

	// Decompress if needed
	if tc.config.EnableCompression && tc.isCompressed(data) {
		var err error
		data, err = tc.decompress(data)
		if err != nil {
			tc.stats.totalErrors.Add(1)
			return fmt.Errorf("failed to decompress value: %w", err)
		}
	}

	// Unmarshal into value
	if err := json.Unmarshal(data, value); err != nil {
		tc.stats.totalErrors.Add(1)
		return fmt.Errorf("failed to unmarshal value: %w", err)
	}

	// Populate L1 cache for next access (write-through)
	go func() {
		_ = tc.l1.Set(context.Background(), key, value, tc.config.L1TTL)
	}()

	return nil
}

// Set stores a value in the cache (writes to both L1 and L2)
func (tc *TieredCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// Store in L1 cache (always succeeds)
	if err := tc.l1.Set(ctx, key, value, ttl); err != nil {
		tc.stats.totalErrors.Add(1)
		return fmt.Errorf("failed to set L1 cache: %w", err)
	}

	// Store in L2 (Redis) asynchronously if enabled
	if tc.isRedisAvailable() {
		go tc.setRedisAsync(key, value, ttl)
	}

	return nil
}

// setRedisAsync stores a value in Redis asynchronously (best-effort)
func (tc *TieredCache) setRedisAsync(key string, value interface{}, ttl time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), RedisOperationTimeout)
	defer cancel()

	// Marshal value
	data, err := json.Marshal(value)
	if err != nil {
		tc.logger.Warn("Failed to marshal value for Redis", map[string]interface{}{
			"error": err.Error(),
			"key":   key,
		})
		tc.stats.totalErrors.Add(1)
		return
	}

	// Compress if needed
	if tc.config.EnableCompression && len(data) >= tc.config.CompressionThreshold {
		compressed, err := tc.compress(data)
		if err != nil {
			tc.logger.Warn("Failed to compress value", map[string]interface{}{
				"error": err.Error(),
				"key":   key,
			})
		} else {
			saved := len(data) - len(compressed)
			tc.stats.compressionSaved.Add(int64(saved))
			tc.stats.compressionCount.Add(1)
			data = compressed
		}
	}

	// Store in Redis
	redisKey := tc.makeRedisKey(key)
	if ttl == 0 {
		ttl = tc.config.L2TTL
	}

	if err := tc.redis.Set(ctx, redisKey, data, ttl).Err(); err != nil {
		tc.handleRedisError(err)
		tc.logger.Warn("Failed to set Redis cache", map[string]interface{}{
			"error": err.Error(),
			"key":   key,
		})
		tc.stats.totalErrors.Add(1)
	}
}

// Delete removes a key from both caches
func (tc *TieredCache) Delete(ctx context.Context, key string) error {
	// Delete from L1
	_ = tc.l1.Delete(ctx, key)

	// Delete from L2 (Redis) if enabled
	if tc.isRedisAvailable() {
		redisKey := tc.makeRedisKey(key)
		ctx, cancel := context.WithTimeout(ctx, RedisOperationTimeout)
		defer cancel()

		if err := tc.redis.Del(ctx, redisKey).Err(); err != nil {
			tc.handleRedisError(err)
			tc.logger.Warn("Failed to delete from Redis", map[string]interface{}{
				"error": err.Error(),
				"key":   key,
			})
		}
	}

	return nil
}

// Size returns the size of L1 cache
func (tc *TieredCache) Size() int {
	return tc.l1.Size()
}

// InvalidatePattern invalidates all keys matching a pattern
func (tc *TieredCache) InvalidatePattern(ctx context.Context, pattern string) error {
	// Note: L1 memory cache doesn't support pattern-based invalidation
	// For L1, we'd need to iterate all keys (not implemented for simplicity)

	// Invalidate in Redis if enabled
	if tc.isRedisAvailable() {
		redisPattern := tc.makeRedisKey(pattern + "*")
		ctx, cancel := context.WithTimeout(ctx, RedisOperationTimeout*10)
		defer cancel()

		iter := tc.redis.Scan(ctx, 0, redisPattern, 100).Iterator()
		for iter.Next(ctx) {
			if err := tc.redis.Del(ctx, iter.Val()).Err(); err != nil {
				tc.logger.Warn("Failed to delete Redis key", map[string]interface{}{
					"error": err.Error(),
					"key":   iter.Val(),
				})
			}
		}

		if err := iter.Err(); err != nil {
			return fmt.Errorf("redis scan failed: %w", err)
		}
	}

	return nil
}

// WarmCache pre-loads frequently accessed keys into L1 cache
func (tc *TieredCache) WarmCache(ctx context.Context, keys []string) error {
	if !tc.isRedisAvailable() {
		return nil // Nothing to warm if Redis is not available
	}

	tc.logger.Info("Warming cache", map[string]interface{}{
		"keys": len(keys),
	})

	warmed := 0
	for _, key := range keys {
		var value interface{}
		if err := tc.Get(ctx, key, &value); err == nil {
			warmed++
		}
	}

	tc.logger.Info("Cache warming complete", map[string]interface{}{
		"total":  len(keys),
		"warmed": warmed,
	})

	return nil
}

// GetStats returns current cache statistics
func (tc *TieredCache) GetStats() map[string]interface{} {
	l1Hits := tc.stats.l1Hits.Load()
	l1Misses := tc.stats.l1Misses.Load()
	l2Hits := tc.stats.l2Hits.Load()
	l2Misses := tc.stats.l2Misses.Load()

	l1Total := l1Hits + l1Misses
	l2Total := l2Hits + l2Misses

	l1HitRate := 0.0
	if l1Total > 0 {
		l1HitRate = float64(l1Hits) / float64(l1Total)
	}

	l2HitRate := 0.0
	if l2Total > 0 {
		l2HitRate = float64(l2Hits) / float64(l2Total)
	}

	return map[string]interface{}{
		"l1_hits":           l1Hits,
		"l1_misses":         l1Misses,
		"l1_hit_rate":       l1HitRate,
		"l1_size":           tc.l1.Size(),
		"l2_enabled":        tc.redisEnabled,
		"l2_healthy":        tc.redisHealthy,
		"l2_hits":           l2Hits,
		"l2_misses":         l2Misses,
		"l2_hit_rate":       l2HitRate,
		"total_requests":    tc.stats.totalRequests.Load(),
		"total_errors":      tc.stats.totalErrors.Load(),
		"compression_saved": tc.stats.compressionSaved.Load(),
		"compression_count": tc.stats.compressionCount.Load(),
	}
}

// IsRedisHealthy returns the current health status of Redis
func (tc *TieredCache) IsRedisHealthy() bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.redisHealthy
}

// Close closes the cache and Redis connection
func (tc *TieredCache) Close() error {
	if tc.redis != nil {
		return tc.redis.Close()
	}
	return nil
}

// isRedisAvailable checks if Redis is enabled and healthy
func (tc *TieredCache) isRedisAvailable() bool {
	if !tc.redisEnabled || tc.redis == nil {
		return false
	}

	tc.mu.RLock()
	healthy := tc.redisHealthy
	lastCheck := tc.lastHealthCheck
	tc.mu.RUnlock()

	// Recheck health every 30 seconds
	if time.Since(lastCheck) > 30*time.Second {
		go tc.checkRedisHealth()
	}

	return healthy
}

// checkRedisHealth performs a health check on Redis
func (tc *TieredCache) checkRedisHealth() {
	ctx, cancel := context.WithTimeout(context.Background(), RedisConnectTimeout)
	defer cancel()

	err := tc.redis.Ping(ctx).Err()

	tc.mu.Lock()
	tc.redisHealthy = (err == nil)
	tc.lastHealthCheck = time.Now()
	tc.mu.Unlock()

	if err != nil {
		tc.logger.Warn("Redis health check failed", map[string]interface{}{
			"error": err.Error(),
		})
	}
}

// handleRedisError handles Redis errors and updates health status
func (tc *TieredCache) handleRedisError(err error) {
	if err == nil || err == redis.Nil {
		return
	}

	tc.mu.Lock()
	tc.redisHealthy = false
	tc.mu.Unlock()

	// Schedule a health check
	go func() {
		time.Sleep(5 * time.Second)
		tc.checkRedisHealth()
	}()
}

// makeRedisKey creates a Redis key with namespace
func (tc *TieredCache) makeRedisKey(key string) string {
	return "edge_mcp:cache:" + key
}

// compress compresses data using gzip
func (tc *TieredCache) compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	// gzip.NewWriter will write the magic header automatically
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// decompress decompresses gzip data
func (tc *TieredCache) decompress(data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()

	return io.ReadAll(gz)
}

// isCompressed checks if data is gzip compressed
func (tc *TieredCache) isCompressed(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}
