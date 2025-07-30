package cache_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

func setupBenchmarkCache(b *testing.B) *cache.SemanticCache {
	// Setup Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: cache.GetTestRedisAddr(),
		DB:   2, // Use DB 2 for benchmarks
	})

	// Clear benchmark database
	redisClient.FlushDB(context.Background())

	// Create cache with optimized config
	config := &cache.Config{
		SimilarityThreshold: 0.95,
		TTL:                 3600,
		MaxCandidates:       10,
		MaxCacheSize:        100000,
		Prefix:              "bench",
		EnableMetrics:       false, // Disable metrics for benchmarks
		EnableCompression:   false, // Disable compression for raw performance
	}

	logger := observability.NewLogger("benchmark")

	c, err := cache.NewSemanticCache(redisClient, config, logger)
	require.NoError(b, err)

	return c
}

func generateEmbedding() []float32 {
	// Generate a 1536-dimensional embedding (OpenAI ada-002 size)
	embedding := make([]float32, 1536)
	for i := range embedding {
		embedding[i] = float32(i) / 1536.0
	}
	return embedding
}

func generateResults(n int) []cache.CachedSearchResult {
	results := make([]cache.CachedSearchResult, n)
	for i := 0; i < n; i++ {
		results[i] = cache.CachedSearchResult{
			ID:      fmt.Sprintf("doc-%d", i),
			Content: fmt.Sprintf("This is document %d with some content that might be useful", i),
			Score:   float32(n-i) / float32(n),
			Metadata: map[string]interface{}{
				"source": "benchmark",
				"index":  i,
			},
		}
	}
	return results
}

func BenchmarkCacheSet(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-generate data
	embeddings := make([][]float32, 1000)
	for i := range embeddings {
		embeddings[i] = generateEmbedding()
		// Slightly vary embeddings
		embeddings[i][0] = float32(i) / 1000.0
	}
	results := generateResults(10)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			query := fmt.Sprintf("benchmark query %d", i)
			embedding := embeddings[i%len(embeddings)]
			_ = cache.Set(ctx, query, embedding, results)
			i++
		}
	})
}

func BenchmarkCacheGet(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate cache
	numQueries := 1000
	embeddings := make([][]float32, numQueries)
	for i := 0; i < numQueries; i++ {
		query := fmt.Sprintf("query %d", i)
		embedding := generateEmbedding()
		embedding[0] = float32(i) / float32(numQueries)
		embeddings[i] = embedding

		results := generateResults(10)
		err := cache.Set(ctx, query, embedding, results)
		require.NoError(b, err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			query := fmt.Sprintf("query %d", i%numQueries)
			embedding := embeddings[i%numQueries]
			_, _ = cache.Get(ctx, query, embedding)
			i++
		}
	})
}

func BenchmarkCacheGetMiss(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate with different queries
	for i := 0; i < 100; i++ {
		query := fmt.Sprintf("existing query %d", i)
		embedding := generateEmbedding()
		results := generateResults(10)
		_ = cache.Set(ctx, query, embedding, results)
	}

	// Different embeddings for misses
	missEmbeddings := make([][]float32, 100)
	for i := range missEmbeddings {
		missEmbeddings[i] = generateEmbedding()
		missEmbeddings[i][0] = float32(1000+i) / 1100.0 // Different range
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			query := fmt.Sprintf("missing query %d", i)
			embedding := missEmbeddings[i%len(missEmbeddings)]
			_, _ = cache.Get(ctx, query, embedding)
			i++
		}
	})
}

func BenchmarkSimilaritySearch(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate with queries
	baseEmbedding := generateEmbedding()
	for i := 0; i < 1000; i++ {
		query := fmt.Sprintf("similar query %d", i)
		embedding := make([]float32, len(baseEmbedding))
		copy(embedding, baseEmbedding)

		// Slightly modify embedding
		for j := 0; j < 10; j++ {
			embedding[j] = baseEmbedding[j] + float32(i)*0.0001
		}

		results := generateResults(5)
		_ = cache.Set(ctx, query, embedding, results)
	}

	// Create test embeddings with varying similarity
	testEmbeddings := make([][]float32, 10)
	for i := range testEmbeddings {
		testEmbeddings[i] = make([]float32, len(baseEmbedding))
		copy(testEmbeddings[i], baseEmbedding)

		// Vary similarity
		for j := 0; j < i*10; j++ {
			testEmbeddings[i][j] = baseEmbedding[j] + float32(i)*0.001
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		embedding := testEmbeddings[i%len(testEmbeddings)]
		_, _ = cache.Get(ctx, "test query", embedding)
	}
}

func BenchmarkConcurrentAccess(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 100; i++ {
		query := fmt.Sprintf("concurrent query %d", i)
		embedding := generateEmbedding()
		embedding[0] = float32(i) / 100.0
		results := generateResults(5)
		_ = cache.Set(ctx, query, embedding, results)
	}

	embeddings := make([][]float32, 100)
	for i := range embeddings {
		embeddings[i] = generateEmbedding()
		embeddings[i][0] = float32(i) / 100.0
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			idx := i % 100
			if i%2 == 0 {
				// Read operation
				query := fmt.Sprintf("concurrent query %d", idx)
				_, _ = cache.Get(ctx, query, embeddings[idx])
			} else {
				// Write operation
				query := fmt.Sprintf("new concurrent query %d", i)
				results := generateResults(5)
				_ = cache.Set(ctx, query, embeddings[idx], results)
			}
			i++
		}
	})
}

func BenchmarkLargeResults(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Test with varying result sizes
	resultSizes := []int{1, 10, 50, 100}

	for _, size := range resultSizes {
		b.Run(fmt.Sprintf("results_%d", size), func(b *testing.B) {
			results := generateResults(size)
			embedding := generateEmbedding()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				query := fmt.Sprintf("large result query %d", i)
				_ = cache.Set(ctx, query, embedding, results)
			}
		})
	}
}

func BenchmarkTenantIsolation(b *testing.B) {
	// This would benchmark the tenant-aware cache
	// For now, we'll benchmark with different key prefixes to simulate tenants
	cache := setupBenchmarkCache(b)

	numTenants := 10
	tenants := make([]uuid.UUID, numTenants)
	for i := range tenants {
		tenants[i] = uuid.New()
	}

	embeddings := make([][]float32, 100)
	for i := range embeddings {
		embeddings[i] = generateEmbedding()
		embeddings[i][0] = float32(i) / 100.0
	}
	results := generateResults(5)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			tenantID := tenants[i%numTenants]
			ctx := auth.WithTenantID(context.Background(), tenantID)

			query := fmt.Sprintf("tenant query %d", i%100)
			embedding := embeddings[i%len(embeddings)]

			if i%3 == 0 {
				_ = cache.Set(ctx, query, embedding, results)
			} else {
				_, _ = cache.Get(ctx, query, embedding)
			}
			i++
		}
	})
}

func BenchmarkStats(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate with many entries
	for i := 0; i < 10000; i++ {
		query := fmt.Sprintf("stats query %d", i)
		embedding := generateEmbedding()
		embedding[0] = float32(i) / 10000.0
		results := generateResults(3)
		_ = cache.Set(ctx, query, embedding, results)

		// Simulate some hits
		if i%10 == 0 {
			for j := 0; j < i%5; j++ {
				_, _ = cache.Get(ctx, query, embedding)
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Stats(ctx)
	}
}

func BenchmarkGetTopQueries(b *testing.B) {
	cache := setupBenchmarkCache(b)
	ctx := context.Background()

	// Pre-populate with entries having different hit counts
	for i := 0; i < 1000; i++ {
		query := fmt.Sprintf("top query %d", i)
		embedding := generateEmbedding()
		embedding[0] = float32(i) / 1000.0
		results := generateResults(3)
		_ = cache.Set(ctx, query, embedding, results)

		// Simulate varying hit counts
		hits := (1000 - i) / 10
		for j := 0; j < hits; j++ {
			_, _ = cache.Get(ctx, query, embedding)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.GetTopQueries(ctx, 10)
	}
}
