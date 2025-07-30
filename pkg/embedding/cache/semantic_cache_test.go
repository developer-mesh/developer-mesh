package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestCache(t *testing.T) (*SemanticCache, *miniredis.Miniredis, func()) {
	// Create miniredis instance
	mr, err := miniredis.Run()
	require.NoError(t, err)

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Create cache config
	config := &Config{
		SimilarityThreshold: 0.95,
		TTL:                 time.Hour,
		MaxCandidates:       10,
		MaxCacheSize:        100,
		Prefix:              "test_cache",
		EnableMetrics:       false,
		EnableCompression:   false,
	}

	// Create cache
	cache, err := NewSemanticCache(client, config, observability.NewNoopLogger())
	require.NoError(t, err)

	cleanup := func() {
		_ = client.Close()
		mr.Close()
	}

	return cache, mr, cleanup
}

func TestNewSemanticCache(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		cache, _, cleanup := setupTestCache(t)
		defer cleanup()

		assert.NotNil(t, cache)
		assert.NotNil(t, cache.redis)
		assert.NotNil(t, cache.config)
		assert.NotNil(t, cache.normalizer)
	})

	t.Run("nil redis client", func(t *testing.T) {
		cache, err := NewSemanticCache(nil, nil, nil)
		assert.Error(t, err)
		assert.Nil(t, cache)
		assert.Contains(t, err.Error(), "redis client is required")
	})

	t.Run("invalid similarity threshold", func(t *testing.T) {
		mr, err := miniredis.Run()
		require.NoError(t, err)
		defer mr.Close()

		client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		defer func() { _ = client.Close() }()

		config := &Config{
			SimilarityThreshold: 1.5, // Invalid
		}

		cache, err := NewSemanticCache(client, config, nil)
		assert.Error(t, err)
		assert.Nil(t, cache)
		assert.Contains(t, err.Error(), "similarity threshold must be between 0 and 1")
	})

	t.Run("default config", func(t *testing.T) {
		mr, err := miniredis.Run()
		require.NoError(t, err)
		defer mr.Close()

		client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		defer func() { _ = client.Close() }()

		cache, err := NewSemanticCache(client, nil, nil)
		require.NoError(t, err)

		assert.Equal(t, float32(0.95), cache.config.SimilarityThreshold)
		assert.Equal(t, 24*time.Hour, cache.config.TTL)
		assert.Equal(t, 10, cache.config.MaxCandidates)
		assert.Equal(t, "semantic_cache", cache.config.Prefix)
	})
}

func TestSemanticCache_SetAndGet(t *testing.T) {
	cache, _, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("set and get exact match", func(t *testing.T) {
		query := "How to implement Redis cache?"
		embedding := []float32{0.1, 0.2, 0.3}
		results := []CachedSearchResult{
			{
				ID:      "1",
				Content: "Redis implementation guide",
				Score:   0.9,
			},
			{
				ID:      "2",
				Content: "Cache patterns",
				Score:   0.8,
			},
		}

		// Set cache entry
		err := cache.Set(ctx, query, embedding, results)
		require.NoError(t, err)

		// Get exact match (without embedding)
		entry, err := cache.Get(ctx, query, nil)
		require.NoError(t, err)
		require.NotNil(t, entry)

		assert.Equal(t, query, entry.Query)
		assert.Len(t, entry.Results, 2)
		assert.Equal(t, results[0].ID, entry.Results[0].ID)
		assert.Equal(t, results[1].ID, entry.Results[1].ID)
	})

	t.Run("get with no embedding returns nil for miss", func(t *testing.T) {
		entry, err := cache.Get(ctx, "non-existent query", nil)
		assert.NoError(t, err)
		assert.Nil(t, entry)
	})

	t.Run("normalized query matching", func(t *testing.T) {
		query1 := "How to IMPLEMENT redis CACHE?"
		query2 := "how to implement Redis cache"
		embedding := []float32{0.1, 0.2, 0.3}
		results := []CachedSearchResult{{ID: "1", Content: "Test", Score: 0.9}}

		// Set with query1
		err := cache.Set(ctx, query1, embedding, results)
		require.NoError(t, err)

		// Get with query2 (different case/spacing)
		entry, err := cache.Get(ctx, query2, nil)
		require.NoError(t, err)
		require.NotNil(t, entry)

		assert.Len(t, entry.Results, 1)
	})

	t.Run("access stats update", func(t *testing.T) {
		query := "test query for stats"
		embedding := []float32{0.1, 0.2}
		results := []CachedSearchResult{{ID: "1", Content: "Test", Score: 0.9}}

		// Set cache entry
		err := cache.Set(ctx, query, embedding, results)
		require.NoError(t, err)

		// First access
		entry1, err := cache.Get(ctx, query, nil)
		require.NoError(t, err)
		require.NotNil(t, entry1)
		assert.Equal(t, 1, entry1.HitCount) // Shows 1 because updateAccessStats updates in-place

		// Second access
		entry2, err := cache.Get(ctx, query, nil)
		require.NoError(t, err)
		require.NotNil(t, entry2)
		assert.Equal(t, 2, entry2.HitCount) // Now shows 2 from both accesses

		// Verify access time was updated
		assert.True(t, entry2.LastAccessedAt.After(entry1.CachedAt))
	})
}

func TestSemanticCache_Delete(t *testing.T) {
	cache, _, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	query := "test query to delete"
	embedding := []float32{0.1, 0.2, 0.3}
	results := []CachedSearchResult{{ID: "1", Content: "Test", Score: 0.9}}

	// Set cache entry
	err := cache.Set(ctx, query, embedding, results)
	require.NoError(t, err)

	// Verify it exists
	entry, err := cache.Get(ctx, query, nil)
	require.NoError(t, err)
	require.NotNil(t, entry)

	// Delete it
	err = cache.Delete(ctx, query)
	require.NoError(t, err)

	// Verify it's gone
	entry, err = cache.Get(ctx, query, nil)
	assert.NoError(t, err)
	assert.Nil(t, entry)
}

func TestSemanticCache_Clear(t *testing.T) {
	cache, mr, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	// Add multiple entries
	for i := 0; i < 5; i++ {
		query := fmt.Sprintf("test query %d", i)
		results := []CachedSearchResult{{ID: fmt.Sprintf("%d", i), Content: "Test", Score: 0.9}}
		err := cache.Set(ctx, query, nil, results)
		require.NoError(t, err)
	}

	// Clear cache
	err := cache.Clear(ctx)
	require.NoError(t, err)

	// Also clear miniredis for testing
	mr.FlushAll()

	// Verify all entries are gone
	for i := 0; i < 5; i++ {
		query := fmt.Sprintf("test query %d", i)
		entry, err := cache.Get(ctx, query, nil)
		assert.NoError(t, err)
		assert.Nil(t, entry)
	}

	// Verify stats are reset
	assert.Equal(t, int64(0), cache.hitCount.Load())
	// We had 5 misses from checking the deleted entries
	assert.Equal(t, int64(5), cache.missCount.Load())
}

func TestSemanticCache_Stats(t *testing.T) {
	cache, _, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	// Add some entries
	for i := 0; i < 3; i++ {
		query := fmt.Sprintf("test query %d", i)
		results := make([]CachedSearchResult, i+1)
		for j := 0; j <= i; j++ {
			results[j] = CachedSearchResult{
				ID:      fmt.Sprintf("%d-%d", i, j),
				Content: "Test",
				Score:   0.9,
			}
		}
		err := cache.Set(ctx, query, nil, results)
		require.NoError(t, err)
	}

	// Generate some hits and misses
	_, _ = cache.Get(ctx, "test query 0", nil) // Hit
	_, _ = cache.Get(ctx, "test query 1", nil) // Hit
	_, _ = cache.Get(ctx, "test query 1", nil) // Hit again
	_, _ = cache.Get(ctx, "non-existent", nil) // Miss
	_, _ = cache.Get(ctx, "another miss", nil) // Miss

	// Get stats
	stats, err := cache.Stats(ctx)
	require.NoError(t, err)

	assert.Equal(t, 3, stats.TotalEntries)
	assert.Equal(t, 3, stats.TotalHits)
	assert.Equal(t, 2, stats.TotalMisses)
	assert.InDelta(t, 0.6, stats.HitRate, 0.01) // 3/5 = 0.6
	assert.Greater(t, stats.AverageResultsPerEntry, 0.0)
	assert.NotZero(t, stats.Timestamp)
}

func TestSemanticCache_HitMissRecording(t *testing.T) {
	cache, _, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	// Enable metrics for this test
	cache.config.EnableMetrics = true
	cache.metrics = observability.NewMetricsClient()

	// Generate hits and misses
	cache.recordHit(ctx, "exact")
	cache.recordHit(ctx, "similarity")
	cache.recordMiss(ctx, "no_embedding")
	cache.recordMiss(ctx, "no_match")

	// Check internal stats
	cache.mu.RLock()
	assert.Equal(t, int64(2), cache.hitCount.Load())
	assert.Equal(t, int64(2), cache.missCount.Load())
	cache.mu.RUnlock()
}

func TestSemanticCache_TTL(t *testing.T) {
	// Create cache with short TTL
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = client.Close() }()

	config := &Config{
		SimilarityThreshold: 0.95,
		TTL:                 100 * time.Millisecond, // Very short TTL
		Prefix:              "test_cache",
	}

	cache, err := NewSemanticCache(client, config, nil)
	require.NoError(t, err)

	ctx := context.Background()

	// Set entry
	query := "test ttl"
	results := []CachedSearchResult{{ID: "1", Content: "Test", Score: 0.9}}
	err = cache.Set(ctx, query, nil, results)
	require.NoError(t, err)

	// Verify it exists
	entry, err := cache.Get(ctx, query, nil)
	require.NoError(t, err)
	require.NotNil(t, entry)

	// Wait for TTL to expire
	mr.FastForward(200 * time.Millisecond)

	// Verify it's gone
	entry, err = cache.Get(ctx, query, nil)
	assert.NoError(t, err)
	assert.Nil(t, entry)
}

func TestSemanticCache_EdgeCases(t *testing.T) {
	cache, _, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("empty query", func(t *testing.T) {
		err := cache.Set(ctx, "", nil, []CachedSearchResult{})
		assert.Error(t, err) // Now validation rejects empty queries
		assert.Contains(t, err.Error(), "empty")

		entry, err := cache.Get(ctx, "", nil)
		assert.NoError(t, err)
		assert.Nil(t, entry)
	})

	t.Run("empty results", func(t *testing.T) {
		query := "query with no results"
		err := cache.Set(ctx, query, nil, []CachedSearchResult{})
		assert.NoError(t, err)

		entry, err := cache.Get(ctx, query, nil)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.Len(t, entry.Results, 0)
	})

	t.Run("very large embedding", func(t *testing.T) {
		query := "query with large embedding"
		embedding := make([]float32, 1000)
		for i := range embedding {
			embedding[i] = float32(i) / 1000.0
		}

		results := []CachedSearchResult{{ID: "1", Content: "Test", Score: 0.9}}
		err := cache.Set(ctx, query, embedding, results)
		assert.NoError(t, err)

		entry, err := cache.Get(ctx, query, nil)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.Len(t, entry.Embedding, 1000)
	})
}

func BenchmarkSemanticCache_Set(b *testing.B) {
	cache, _, cleanup := setupTestCache(&testing.T{})
	defer cleanup()

	ctx := context.Background()
	results := []CachedSearchResult{
		{ID: "1", Content: "Test content 1", Score: 0.9},
		{ID: "2", Content: "Test content 2", Score: 0.8},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		query := fmt.Sprintf("benchmark query %d", i)
		_ = cache.Set(ctx, query, nil, results)
	}
}

func BenchmarkSemanticCache_Get(b *testing.B) {
	cache, _, cleanup := setupTestCache(&testing.T{})
	defer cleanup()

	ctx := context.Background()

	// Pre-populate cache
	query := "benchmark query"
	results := []CachedSearchResult{
		{ID: "1", Content: "Test content 1", Score: 0.9},
		{ID: "2", Content: "Test content 2", Score: 0.8},
	}
	_ = cache.Set(ctx, query, nil, results)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(ctx, query, nil)
	}
}
