package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

func TestFallbackCache_Basic(t *testing.T) {
	ctx := context.Background()
	logger := observability.NewLogger("test")
	cache := NewFallbackCache(10, 1*time.Minute, logger, nil)

	// Test Set and Get
	entry := &CacheEntry{
		Query:    "test query",
		Results:  []CachedSearchResult{{ID: "1", Score: 0.9}},
		CachedAt: time.Now(),
	}

	err := cache.Set(ctx, "test-key", entry)
	assert.NoError(t, err)

	// Get the entry
	retrieved, err := cache.Get(ctx, "test-key")
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "test query", retrieved.Query)
	assert.Equal(t, 1, retrieved.HitCount)

	// Get non-existent entry
	missing, err := cache.Get(ctx, "missing-key")
	assert.NoError(t, err)
	assert.Nil(t, missing)

	// Delete entry
	err = cache.Delete(ctx, "test-key")
	assert.NoError(t, err)

	// Verify deletion
	deleted, err := cache.Get(ctx, "test-key")
	assert.NoError(t, err)
	assert.Nil(t, deleted)
}

func TestFallbackCache_LRU(t *testing.T) {
	ctx := context.Background()
	logger := observability.NewLogger("test")
	cache := NewFallbackCache(3, 1*time.Minute, logger, nil)

	// Fill cache to capacity
	for i := 0; i < 3; i++ {
		entry := &CacheEntry{
			Query: fmt.Sprintf("query-%d", i),
		}
		err := cache.Set(ctx, fmt.Sprintf("key-%d", i), entry)
		assert.NoError(t, err)
	}

	// Access key-0 to make it most recently used
	_, err := cache.Get(ctx, "key-0")
	assert.NoError(t, err)

	// Add one more entry - should evict key-1 (LRU)
	newEntry := &CacheEntry{Query: "new-query"}
	err = cache.Set(ctx, "key-new", newEntry)
	assert.NoError(t, err)

	// Verify key-1 was evicted
	evicted, err := cache.Get(ctx, "key-1")
	assert.NoError(t, err)
	assert.Nil(t, evicted)

	// Verify others still exist
	for _, key := range []string{"key-0", "key-2", "key-new"} {
		entry, err := cache.Get(ctx, key)
		assert.NoError(t, err)
		assert.NotNil(t, entry, "Key %s should still exist", key)
	}
}

func TestFallbackCache_Expiration(t *testing.T) {
	ctx := context.Background()
	logger := observability.NewLogger("test")
	// Use very short TTL for testing
	cache := NewFallbackCache(10, 100*time.Millisecond, logger, nil)

	entry := &CacheEntry{Query: "test"}
	err := cache.Set(ctx, "test-key", entry)
	assert.NoError(t, err)

	// Should exist immediately
	retrieved, err := cache.Get(ctx, "test-key")
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	expired, err := cache.Get(ctx, "test-key")
	assert.NoError(t, err)
	assert.Nil(t, expired)
}

func TestFallbackCache_Stats(t *testing.T) {
	ctx := context.Background()
	logger := observability.NewLogger("test")
	cache := NewFallbackCache(10, 1*time.Minute, logger, nil)

	// Generate some activity
	entry := &CacheEntry{Query: "test"}
	err := cache.Set(ctx, "key1", entry)
	assert.NoError(t, err)
	err = cache.Set(ctx, "key2", entry)
	assert.NoError(t, err)

	_, err = cache.Get(ctx, "key1") // hit
	assert.NoError(t, err)
	_, err = cache.Get(ctx, "key1") // hit
	assert.NoError(t, err)
	_, err = cache.Get(ctx, "missing") // miss
	assert.NoError(t, err)
	_, err = cache.Get(ctx, "missing2") // miss
	assert.NoError(t, err)

	stats := cache.GetStats()
	assert.Equal(t, "in-memory", stats["type"])
	assert.Equal(t, 2, stats["entries"])
	assert.Equal(t, int64(2), stats["hits"])
	assert.Equal(t, int64(2), stats["misses"])
	assert.Equal(t, 0.5, stats["hit_rate"])
}

func TestDegradedModeCache_Fallback(t *testing.T) {
	ctx := context.Background()
	logger := observability.NewLogger("test")

	// Create degraded cache with nil primary (simulates complete failure)
	degradedCache := NewDegradedModeCache(nil, logger)

	// Should use fallback since primary has no Redis
	entry := []CachedSearchResult{{ID: "1", Score: 0.9}}
	err := degradedCache.Set(ctx, "test-query", nil, entry)
	assert.NoError(t, err)

	// Should be in degraded mode
	assert.True(t, degradedCache.IsInDegradedMode())

	// Should retrieve from fallback
	retrieved, err := degradedCache.Get(ctx, "test-query", nil)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "test-query", retrieved.Query)
}

func TestDegradedModeCache_Recovery(t *testing.T) {
	ctx := context.Background()
	logger := observability.NewLogger("test")

	// Create working primary cache
	redisClient := GetTestRedisClient(t)
	defer func() { _ = redisClient.Close() }()

	// Skip if Redis not available
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}

	primary, err := NewSemanticCache(redisClient, DefaultConfig(), logger)
	require.NoError(t, err)

	degradedCache := NewDegradedModeCache(primary, logger)

	// Should use primary when healthy
	entry := []CachedSearchResult{{ID: "1", Score: 0.9}}
	err = degradedCache.Set(ctx, "test-query", nil, entry)
	assert.NoError(t, err)
	assert.False(t, degradedCache.IsInDegradedMode())

	// Simulate primary failure by closing Redis
	err = primary.redis.Close()
	assert.NoError(t, err)

	// Next operation should switch to degraded mode
	err = degradedCache.Set(ctx, "test-query-2", nil, entry)
	assert.NoError(t, err)
	assert.True(t, degradedCache.IsInDegradedMode())

	// Stats should show degraded mode
	stats := degradedCache.GetStats()
	assert.True(t, stats["degraded_mode"].(bool))
}

func TestAccessList_Operations(t *testing.T) {
	al := &accessList{}

	// Test add
	al.add("key1")
	al.add("key2")
	al.add("key3")
	assert.Equal(t, 3, al.size)
	assert.Equal(t, "key3", al.head.key)
	assert.Equal(t, "key1", al.tail.key)

	// Test moveToFront
	al.moveToFront("key1")
	assert.Equal(t, "key1", al.head.key)
	assert.Equal(t, "key2", al.tail.key)

	// Test remove
	al.remove("key2")
	assert.Equal(t, 2, al.size)
	assert.Equal(t, "key3", al.tail.key)

	// Test removeTail
	removed := al.removeTail()
	assert.Equal(t, "key3", removed)
	assert.Equal(t, 1, al.size)

	// Test clear
	al.clear()
	assert.Equal(t, 0, al.size)
	assert.Nil(t, al.head)
	assert.Nil(t, al.tail)
}
