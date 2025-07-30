package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

func TestIntegration_CacheLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Setup Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr: cache.GetTestRedisAddr(),
		DB:   3, // Use DB 3 for integration tests
	})
	defer func() { _ = redisClient.Close() }()

	// Clear test database
	err := redisClient.FlushDB(context.Background()).Err()
	require.NoError(t, err)

	// Create cache
	config := cache.DefaultConfig()
	config.Prefix = "integration_test"
	logger := observability.NewLogger("integration_test")

	c, err := cache.NewSemanticCache(redisClient, config, logger)
	require.NoError(t, err)

	// Create lifecycle manager
	lifecycle := cache.NewLifecycle(c, logger)

	// Start lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = lifecycle.Start(ctx)
	require.NoError(t, err)

	// Test cache operations
	testQuery := "integration test query"
	testEmbedding := []float32{0.1, 0.2, 0.3}
	testResults := []cache.CachedSearchResult{
		{ID: "1", Content: "Test result", Score: 0.9},
	}

	// Set and get
	err = c.Set(ctx, testQuery, testEmbedding, testResults)
	require.NoError(t, err)

	entry, err := c.Get(ctx, testQuery, testEmbedding)
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, testQuery, entry.Query)

	// Test stats
	stats, err := c.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.TotalEntries)
	assert.Equal(t, 1, stats.TotalHits)

	// Test graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	err = lifecycle.Shutdown(shutdownCtx)
	require.NoError(t, err)

	// Verify cache is shutting down
	assert.True(t, lifecycle.IsShuttingDown())
}

func TestIntegration_CacheWarmer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Setup Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr: cache.GetTestRedisAddr(),
		DB:   3,
	})
	defer func() { _ = redisClient.Close() }()

	// Clear test database
	err := redisClient.FlushDB(context.Background()).Err()
	require.NoError(t, err)

	// Create cache
	config := cache.DefaultConfig()
	config.Prefix = "warmer_test"
	logger := observability.NewLogger("warmer_test")

	c, err := cache.NewSemanticCache(redisClient, config, logger)
	require.NoError(t, err)

	// Create search executor mock
	searchExecutor := func(ctx context.Context, query string) ([]cache.CachedSearchResult, error) {
		return []cache.CachedSearchResult{
			{ID: "warmed-1", Content: "Warmed result for " + query, Score: 0.95},
		}, nil
	}

	// Create warmer
	warmer := cache.NewCacheWarmer(c, searchExecutor, logger)

	// Test warming
	ctx := context.Background()
	queries := []string{
		"warm query 1",
		"warm query 2",
		"warm query 3",
	}

	results, err := warmer.Warm(ctx, queries)
	require.NoError(t, err)
	assert.Len(t, results, 3)

	// Verify all queries were warmed
	for i, result := range results {
		assert.True(t, result.Success)
		assert.Equal(t, queries[i], result.Query)
		assert.Equal(t, 1, result.ResultCount)
		assert.False(t, result.FromCache) // First time, not from cache
	}

	// Run again - should be from cache
	results2, err := warmer.Warm(ctx, queries)
	require.NoError(t, err)

	for _, result := range results2 {
		assert.True(t, result.Success)
		assert.True(t, result.FromCache) // Second time, from cache
	}
}

func TestIntegration_CacheAnalytics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Setup Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr: cache.GetTestRedisAddr(),
		DB:   3,
	})
	defer func() { _ = redisClient.Close() }()

	// Clear test database
	err := redisClient.FlushDB(context.Background()).Err()
	require.NoError(t, err)

	// Create cache
	config := cache.DefaultConfig()
	config.Prefix = "analytics_test"
	logger := observability.NewLogger("analytics_test")

	c, err := cache.NewSemanticCache(redisClient, config, logger)
	require.NoError(t, err)

	// Populate cache with data
	ctx := context.Background()
	queries := []struct {
		query     string
		hits      int
		embedding []float32
	}{
		{"machine learning model", 10, []float32{0.1, 0.2, 0.3}},
		{"deep learning neural network", 8, []float32{0.2, 0.3, 0.4}},
		{"machine learning algorithm", 6, []float32{0.1, 0.25, 0.35}},
		{"data science pipeline", 4, []float32{0.3, 0.4, 0.5}},
		{"artificial intelligence", 2, []float32{0.15, 0.25, 0.35}},
	}

	// Set and simulate hits
	for _, q := range queries {
		results := []cache.CachedSearchResult{
			{ID: "1", Content: "Result for " + q.query, Score: 0.9},
		}
		err := c.Set(ctx, q.query, q.embedding, results)
		require.NoError(t, err)

		// Simulate hits
		for i := 0; i < q.hits; i++ {
			_, _ = c.Get(ctx, q.query, q.embedding)
		}
	}

	// Create analytics
	analytics := cache.NewCacheAnalytics(c, logger)

	// Test query pattern analysis
	patterns, err := analytics.AnalyzeQueryPatterns(ctx)
	require.NoError(t, err)

	// "machine" and "learning" should be the most common terms
	assert.Greater(t, patterns["machine"], patterns["data"])
	assert.Greater(t, patterns["learning"], patterns["science"])

	// Test efficiency analysis
	report, err := analytics.AnalyzeCacheEfficiency(ctx)
	require.NoError(t, err)

	assert.Greater(t, report.HitRate, 0.0)
	assert.Equal(t, 5, report.CacheSize)
	assert.Greater(t, report.AverageHitsPerEntry, 0.0)
}

func TestIntegration_PrometheusMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Setup Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr: cache.GetTestRedisAddr(),
		DB:   3,
	})
	defer func() { _ = redisClient.Close() }()

	// Clear test database
	err := redisClient.FlushDB(context.Background()).Err()
	require.NoError(t, err)

	// Create cache with metrics enabled
	config := cache.DefaultConfig()
	config.Prefix = "metrics_test"
	config.EnableMetrics = true
	logger := observability.NewLogger("metrics_test")

	c, err := cache.NewSemanticCache(redisClient, config, logger)
	require.NoError(t, err)

	// Perform operations
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		query := "metrics test query"
		embedding := []float32{float32(i) * 0.1, 0.2, 0.3}
		results := []cache.CachedSearchResult{
			{ID: "1", Content: "Test", Score: 0.9},
		}

		_ = c.Set(ctx, query, embedding, results)
		_, _ = c.Get(ctx, query, embedding)
	}

	// Export metrics
	metricsData, err := c.ExportStats(ctx, "prometheus")
	require.NoError(t, err)
	require.NotEmpty(t, metricsData)

	// Verify Prometheus format
	metricsStr := string(metricsData)
	assert.Contains(t, metricsStr, "# HELP semantic_cache_entries_total")
	assert.Contains(t, metricsStr, "# TYPE semantic_cache_entries_total gauge")
	assert.Contains(t, metricsStr, "# HELP semantic_cache_hits_total")
	assert.Contains(t, metricsStr, "# TYPE semantic_cache_hits_total counter")
}
