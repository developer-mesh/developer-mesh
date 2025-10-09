package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewTieredCache_MemoryOnly tests cache creation without Redis
func TestNewTieredCache_MemoryOnly(t *testing.T) {
	config := &TieredCacheConfig{
		RedisEnabled: false,
		Logger:       observability.NewNoopLogger(),
	}

	cache, err := NewTieredCache(config)
	require.NoError(t, err)
	require.NotNil(t, cache)
	assert.NotNil(t, cache.l1)
	assert.Nil(t, cache.redis)
	assert.False(t, cache.redisEnabled)
}

// TestNewTieredCache_WithRedis tests cache creation with Redis
func TestNewTieredCache_WithRedis(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Redis integration test in short mode")
	}

	// Try to connect to local Redis
	redisURL := "redis://localhost:6379/15" // Use DB 15 for testing

	config := &TieredCacheConfig{
		RedisEnabled:        true,
		RedisURL:            redisURL,
		RedisFallbackMode:   true,
		RedisConnectTimeout: 2 * time.Second,
		Logger:              observability.NewNoopLogger(),
	}

	cache, err := NewTieredCache(config)
	require.NoError(t, err)
	require.NotNil(t, cache)
	defer func() { _ = cache.Close() }()

	// Redis might not be available in local dev
	// Cache should still work in memory-only mode
	assert.NotNil(t, cache.l1)
}

// TestNewTieredCache_InvalidRedisURL tests handling of invalid Redis URL
func TestNewTieredCache_InvalidRedisURL(t *testing.T) {
	config := &TieredCacheConfig{
		RedisEnabled:      true,
		RedisURL:          "invalid://url",
		RedisFallbackMode: true,
		Logger:            observability.NewNoopLogger(),
	}

	cache, err := NewTieredCache(config)
	require.NoError(t, err) // Should succeed due to fallback mode
	require.NotNil(t, cache)
	assert.False(t, cache.redisEnabled)
}

// TestNewTieredCache_DefaultConfig tests default configuration
func TestNewTieredCache_DefaultConfig(t *testing.T) {
	cache, err := NewTieredCache(nil)
	require.NoError(t, err)
	require.NotNil(t, cache)

	assert.Equal(t, DefaultL1MaxItems, cache.config.L1MaxItems)
	assert.Equal(t, DefaultL1TTL, cache.config.L1TTL)
	assert.Equal(t, DefaultL2TTL, cache.config.L2TTL)
	assert.True(t, cache.config.EnableCompression)
	assert.Equal(t, CompressionThreshold, cache.config.CompressionThreshold)
}

// TestTieredCache_GetSet_L1Only tests basic get/set operations with L1 only
func TestTieredCache_GetSet_L1Only(t *testing.T) {
	cache, err := NewTieredCache(&TieredCacheConfig{
		RedisEnabled: false,
		L1MaxItems:   100,
		L1TTL:        5 * time.Minute,
		Logger:       observability.NewNoopLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Set a value
	testData := map[string]interface{}{
		"key":   "value",
		"count": 42,
	}

	err = cache.Set(ctx, "test-key", testData, 0)
	require.NoError(t, err)

	// Get the value
	var result map[string]interface{}
	err = cache.Get(ctx, "test-key", &result)
	require.NoError(t, err)
	assert.Equal(t, "value", result["key"])
	// Memory cache stores direct values, not JSON-marshaled, so type is preserved
	assert.Equal(t, 42, result["count"])

	// Check stats
	stats := cache.GetStats()
	assert.Equal(t, int64(1), stats["l1_hits"])
	assert.Equal(t, int64(0), stats["l1_misses"])
	assert.Equal(t, 1.0, stats["l1_hit_rate"])
}

// TestTieredCache_GetNotFound tests cache miss
func TestTieredCache_GetNotFound(t *testing.T) {
	cache, err := NewTieredCache(&TieredCacheConfig{
		RedisEnabled: false,
		Logger:       observability.NewNoopLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	var result map[string]interface{}

	err = cache.Get(ctx, "nonexistent-key", &result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")

	// Check stats
	stats := cache.GetStats()
	assert.Equal(t, int64(0), stats["l1_hits"])
	assert.Equal(t, int64(1), stats["l1_misses"])
}

// TestTieredCache_Delete tests key deletion
func TestTieredCache_Delete(t *testing.T) {
	cache, err := NewTieredCache(&TieredCacheConfig{
		RedisEnabled: false,
		Logger:       observability.NewNoopLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Set and then delete
	testData := map[string]string{"test": "data"}
	err = cache.Set(ctx, "delete-key", testData, 0)
	require.NoError(t, err)

	err = cache.Delete(ctx, "delete-key")
	require.NoError(t, err)

	// Verify deletion
	var result map[string]string
	err = cache.Get(ctx, "delete-key", &result)
	assert.Error(t, err)
}

// TestTieredCache_Expiration tests TTL expiration
func TestTieredCache_Expiration(t *testing.T) {
	cache, err := NewTieredCache(&TieredCacheConfig{
		RedisEnabled: false,
		L1TTL:        50 * time.Millisecond, // Default TTL for the cache
		Logger:       observability.NewNoopLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Set with short TTL
	testData := map[string]string{"test": "data"}
	err = cache.Set(ctx, "expire-key", testData, 50*time.Millisecond)
	require.NoError(t, err)

	// Should be available immediately
	var result1 map[string]string
	err = cache.Get(ctx, "expire-key", &result1)
	require.NoError(t, err)
	assert.Equal(t, "data", result1["test"])

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	var result2 map[string]string
	err = cache.Get(ctx, "expire-key", &result2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key")
}

// TestTieredCache_Compression tests compression functionality
func TestTieredCache_Compression(t *testing.T) {
	cache, err := NewTieredCache(&TieredCacheConfig{
		RedisEnabled:         false,
		EnableCompression:    true,
		CompressionThreshold: 100, // Low threshold for testing
		Logger:               observability.NewNoopLogger(),
	})
	require.NoError(t, err)

	// Create large data > threshold
	largeData := make(map[string]interface{})
	for i := 0; i < 50; i++ {
		largeData[fmt.Sprintf("key_%d", i)] = "This is a long value to ensure data is above compression threshold"
	}

	// Test compression
	originalJSON, err := json.Marshal(largeData)
	require.NoError(t, err)

	compressed, err := cache.compress(originalJSON)
	require.NoError(t, err)
	assert.Less(t, len(compressed), len(originalJSON), "Compressed data should be smaller")

	// Test decompression
	decompressed, err := cache.decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, originalJSON, decompressed)

	// Test isCompressed
	assert.True(t, cache.isCompressed(compressed))
	assert.False(t, cache.isCompressed(originalJSON))
}

// TestTieredCache_Size tests cache size tracking
func TestTieredCache_Size(t *testing.T) {
	cache, err := NewTieredCache(&TieredCacheConfig{
		RedisEnabled: false,
		L1MaxItems:   10,
		Logger:       observability.NewNoopLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Add multiple items
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		data := map[string]int{"value": i}
		err = cache.Set(ctx, key, data, 0)
		require.NoError(t, err)
	}

	// Check size
	size := cache.Size()
	assert.Equal(t, 5, size)

	// Check stats
	stats := cache.GetStats()
	assert.Equal(t, 5, stats["l1_size"])
}

// TestTieredCache_GetStats tests statistics collection
func TestTieredCache_GetStats(t *testing.T) {
	cache, err := NewTieredCache(&TieredCacheConfig{
		RedisEnabled: false,
		Logger:       observability.NewNoopLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Perform various operations
	testData := map[string]string{"test": "data"}

	// Set and Get (hit) - first get should be a hit from L1
	err = cache.Set(ctx, "key1", testData, 0)
	require.NoError(t, err)

	var result map[string]string
	err = cache.Get(ctx, "key1", &result)
	require.NoError(t, err)

	// Get non-existent (miss) - should increment both l1_misses and total_requests
	err = cache.Get(ctx, "key2", &result)
	require.Error(t, err)

	// Get stats
	stats := cache.GetStats()

	// Verify stats
	assert.Equal(t, int64(1), stats["l1_hits"], "L1 hits should be 1")
	assert.Equal(t, int64(1), stats["l1_misses"], "L1 misses should be 1")
	assert.Equal(t, 0.5, stats["l1_hit_rate"], "L1 hit rate should be 0.5")
	assert.Equal(t, int64(2), stats["total_requests"], "Total requests should be 2")
	assert.False(t, stats["l2_enabled"].(bool))
	assert.False(t, stats["l2_healthy"].(bool))
}

// TestTieredCache_ConcurrentAccess tests thread-safety
func TestTieredCache_ConcurrentAccess(t *testing.T) {
	cache, err := NewTieredCache(&TieredCacheConfig{
		RedisEnabled: false,
		L1MaxItems:   1000,
		Logger:       observability.NewNoopLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	iterations := 100

	// Concurrent writes
	done := make(chan bool, iterations)
	for i := 0; i < iterations; i++ {
		go func(index int) {
			key := fmt.Sprintf("key-%d", index)
			data := map[string]int{"value": index}
			_ = cache.Set(ctx, key, data, 0)
			done <- true
		}(i)
	}

	// Wait for all writes
	for i := 0; i < iterations; i++ {
		<-done
	}

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		go func(index int) {
			key := fmt.Sprintf("key-%d", index)
			var result map[string]int
			_ = cache.Get(ctx, key, &result)
			done <- true
		}(i)
	}

	// Wait for all reads
	for i := 0; i < iterations; i++ {
		<-done
	}

	// Verify stats are consistent
	stats := cache.GetStats()
	assert.Greater(t, stats["l1_hits"].(int64), int64(0))
	assert.Equal(t, int64(iterations), stats["l1_hits"].(int64)+stats["l1_misses"].(int64))
}

// TestTieredCache_WarmCache tests cache warming
func TestTieredCache_WarmCache(t *testing.T) {
	cache, err := NewTieredCache(&TieredCacheConfig{
		RedisEnabled: false,
		Logger:       observability.NewNoopLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Pre-populate some keys
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("warm-key-%d", i)
		data := map[string]int{"value": i}
		_ = cache.Set(ctx, key, data, 0)
	}

	// Warm cache with these keys
	keys := []string{"warm-key-0", "warm-key-1", "warm-key-2", "nonexistent"}
	err = cache.WarmCache(ctx, keys)
	require.NoError(t, err)

	// Cache should have these keys loaded (already in L1)
	// This is more relevant when L2 (Redis) is enabled
}

// TestTieredCache_InvalidatePattern tests pattern-based invalidation
func TestTieredCache_InvalidatePattern(t *testing.T) {
	cache, err := NewTieredCache(&TieredCacheConfig{
		RedisEnabled: false,
		Logger:       observability.NewNoopLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Set multiple keys with pattern
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("user:123:session:%d", i)
		data := map[string]int{"value": i}
		_ = cache.Set(ctx, key, data, 0)
	}

	// Invalidate pattern (note: L1 doesn't support pattern invalidation)
	err = cache.InvalidatePattern(ctx, "user:123:")
	require.NoError(t, err)

	// Without Redis, this doesn't actually invalidate L1
	// This test is more relevant when L2 (Redis) is enabled
}

// TestTieredCache_Close tests graceful shutdown
func TestTieredCache_Close(t *testing.T) {
	cache, err := NewTieredCache(&TieredCacheConfig{
		RedisEnabled: false,
		Logger:       observability.NewNoopLogger(),
	})
	require.NoError(t, err)

	err = cache.Close()
	assert.NoError(t, err)
}

// TestTieredCache_MakeRedisKey tests Redis key generation
func TestTieredCache_MakeRedisKey(t *testing.T) {
	cache, err := NewTieredCache(&TieredCacheConfig{
		RedisEnabled: false,
		Logger:       observability.NewNoopLogger(),
	})
	require.NoError(t, err)

	key := cache.makeRedisKey("test-key")
	assert.Equal(t, "edge_mcp:cache:test-key", key)
	assert.Contains(t, key, "edge_mcp:cache:")
}

// TestDefaultTieredCacheConfig tests default configuration values
func TestDefaultTieredCacheConfig(t *testing.T) {
	config := DefaultTieredCacheConfig()

	assert.Equal(t, DefaultL1MaxItems, config.L1MaxItems)
	assert.Equal(t, DefaultL1TTL, config.L1TTL)
	assert.Equal(t, DefaultL2TTL, config.L2TTL)
	assert.Equal(t, RedisConnectTimeout, config.RedisConnectTimeout)
	assert.Equal(t, CompressionThreshold, config.CompressionThreshold)
	assert.True(t, config.RedisFallbackMode)
	assert.True(t, config.EnableCompression)
	assert.False(t, config.RedisEnabled)
}

// TestTieredCache_IsRedisHealthy tests Redis health status
func TestTieredCache_IsRedisHealthy(t *testing.T) {
	cache, err := NewTieredCache(&TieredCacheConfig{
		RedisEnabled: false,
		Logger:       observability.NewNoopLogger(),
	})
	require.NoError(t, err)

	// Should be false when Redis is not enabled
	assert.False(t, cache.IsRedisHealthy())
}

// Benchmark tests
func BenchmarkTieredCache_GetL1(b *testing.B) {
	cache, _ := NewTieredCache(&TieredCacheConfig{
		RedisEnabled: false,
		Logger:       observability.NewNoopLogger(),
	})

	ctx := context.Background()
	testData := map[string]string{"test": "data"}
	_ = cache.Set(ctx, "bench-key", testData, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result map[string]string
		_ = cache.Get(ctx, "bench-key", &result)
	}
}

func BenchmarkTieredCache_Set(b *testing.B) {
	cache, _ := NewTieredCache(&TieredCacheConfig{
		RedisEnabled: false,
		Logger:       observability.NewNoopLogger(),
	})

	ctx := context.Background()
	testData := map[string]string{"test": "data"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench-key-%d", i)
		_ = cache.Set(ctx, key, testData, 0)
	}
}

func BenchmarkTieredCache_Compression(b *testing.B) {
	cache, _ := NewTieredCache(&TieredCacheConfig{
		RedisEnabled:         false,
		EnableCompression:    true,
		CompressionThreshold: 100,
		Logger:               observability.NewNoopLogger(),
	})

	// Create compressible data
	data := make([]byte, 10000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compressed, _ := cache.compress(data)
		_, _ = cache.decompress(compressed)
	}
}

// Integration test with real Redis (only runs when Redis is available)
func TestTieredCache_WithRedis_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Redis integration test in short mode")
	}

	// Try to connect to local Redis
	redisURL := "redis://localhost:6379/15" // Use DB 15 for testing
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping integration test")
	}
	defer func() { _ = client.Close() }()

	// Clear test database
	client.FlushDB(ctx)

	// Create cache with Redis enabled
	cache, err := NewTieredCache(&TieredCacheConfig{
		RedisEnabled:        true,
		RedisURL:            redisURL,
		RedisFallbackMode:   false,
		RedisConnectTimeout: 5 * time.Second,
		L1TTL:               100 * time.Millisecond,
		L2TTL:               5 * time.Second,
		EnableCompression:   true,
		Logger:              observability.NewNoopLogger(),
	})
	require.NoError(t, err)
	defer func() { _ = cache.Close() }()

	// Verify Redis is enabled and healthy
	assert.True(t, cache.redisEnabled)
	assert.True(t, cache.IsRedisHealthy())

	// Test L1 and L2 interaction
	testData := map[string]interface{}{
		"key":   "value",
		"count": 42,
	}

	// Set value (should go to both L1 and L2)
	err = cache.Set(ctx, "redis-test-key", testData, 0)
	require.NoError(t, err)

	// Wait for async Redis write
	time.Sleep(50 * time.Millisecond)

	// Get from L1 (should hit L1)
	var result1 map[string]interface{}
	err = cache.Get(ctx, "redis-test-key", &result1)
	require.NoError(t, err)
	assert.Equal(t, "value", result1["key"])

	// Clear L1 cache by waiting for expiration
	time.Sleep(150 * time.Millisecond)

	// Get again (should hit L2 and populate L1)
	var result2 map[string]interface{}
	err = cache.Get(ctx, "redis-test-key", &result2)
	require.NoError(t, err)
	assert.Equal(t, "value", result2["key"])

	// Check stats
	stats := cache.GetStats()
	assert.True(t, stats["l2_enabled"].(bool))
	assert.True(t, stats["l2_healthy"].(bool))
	assert.Greater(t, stats["l2_hits"].(int64), int64(0))

	// Test pattern invalidation
	err = cache.InvalidatePattern(ctx, "redis-test-")
	require.NoError(t, err)

	// Key should be gone from Redis
	time.Sleep(50 * time.Millisecond)
	val, err := client.Get(ctx, "edge_mcp:cache:redis-test-key").Result()
	assert.Equal(t, redis.Nil, err)
	assert.Empty(t, val)
}
