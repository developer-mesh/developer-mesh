package cache_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache/lru"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Benchmark cache Get operations
func BenchmarkCache_Get(b *testing.B) {
	ctx := setupBenchmark(b)
	tenantCache, tenantID := setupTenantCache(b)

	// Pre-populate cache
	for i := 0; i < 100; i++ {
		query := fmt.Sprintf("benchmark query %d", i)
		embedding := makeBenchmarkEmbedding(i)
		results := makeBenchmarkResults(i, 5)

		err := tenantCache.Set(ctx, query, embedding, results)
		require.NoError(b, err)
	}

	b.ResetTimer()

	b.Run("CacheHit", func(b *testing.B) {
		ctx := auth.WithTenantID(context.Background(), tenantID)
		query := "benchmark query 50"
		embedding := makeBenchmarkEmbedding(50)

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				entry, err := tenantCache.Get(ctx, query, embedding)
				if err != nil {
					b.Fatal(err)
				}
				if entry == nil {
					b.Fatal("expected cache hit")
				}
			}
		})
	})

	b.Run("CacheMiss", func(b *testing.B) {
		ctx := auth.WithTenantID(context.Background(), tenantID)

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				query := fmt.Sprintf("non-existent query %d", i)
				embedding := makeBenchmarkEmbedding(i)

				entry, err := tenantCache.Get(ctx, query, embedding)
				if err != nil {
					b.Fatal(err)
				}
				if entry != nil {
					b.Fatal("expected cache miss")
				}
				i++
			}
		})
	})
}

// Benchmark cache Set operations
func BenchmarkCache_Set(b *testing.B) {
	ctx := setupBenchmark(b)
	tenantCache, _ := setupTenantCache(b)

	b.Run("SmallPayload", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				query := fmt.Sprintf("benchmark set query %d", i)
				embedding := makeBenchmarkEmbedding(i)
				results := makeBenchmarkResults(i, 1) // 1 result

				err := tenantCache.Set(ctx, query, embedding, results)
				if err != nil {
					b.Fatal(err)
				}
				i++
			}
		})
	})

	b.Run("LargePayload", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				query := fmt.Sprintf("benchmark large query %d", i)
				embedding := makeBenchmarkEmbedding(i)
				results := makeBenchmarkResults(i, 50) // 50 results

				err := tenantCache.Set(ctx, query, embedding, results)
				if err != nil {
					b.Fatal(err)
				}
				i++
			}
		})
	})
}

// Benchmark encryption operations
func BenchmarkCache_GetWithEncryption(b *testing.B) {
	ctx := setupBenchmark(b)
	tenantCache, tenantID := setupTenantCache(b)

	// Pre-populate with encrypted data
	for i := 0; i < 50; i++ {
		query := fmt.Sprintf("encrypted query %d", i)
		embedding := makeBenchmarkEmbedding(i)
		results := []cache.CachedSearchResult{
			{
				ID:      fmt.Sprintf("doc-%d", i),
				Content: "Encrypted content",
				Score:   0.95,
				Metadata: map[string]interface{}{
					"api_key": fmt.Sprintf("sk-secret-key-%d", i),
					"secret":  fmt.Sprintf("sensitive-data-%d", i),
				},
			},
		}

		err := tenantCache.Set(ctx, query, embedding, results)
		require.NoError(b, err)
	}

	b.ResetTimer()

	ctx = auth.WithTenantID(context.Background(), tenantID)

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			query := fmt.Sprintf("encrypted query %d", i%50)
			embedding := makeBenchmarkEmbedding(i % 50)

			entry, err := tenantCache.Get(ctx, query, embedding)
			if err != nil {
				b.Fatal(err)
			}
			if entry == nil {
				b.Fatal("expected cache hit")
			}
			// Verify decryption occurred
			if _, ok := entry.Metadata["decrypted_data"]; !ok {
				b.Fatal("expected decrypted data")
			}
			i++
		}
	})
}

// Benchmark LRU access tracking
func BenchmarkLRU_TrackAccess(b *testing.B) {
	redisClient := setupRedis(b)
	config := lru.DefaultConfig()
	manager := lru.NewManager(redisClient, config, "bench", nil, nil)

	tenantID := uuid.New()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench:key:%d", i%1000)
			manager.TrackAccess(tenantID, key)
			i++
		}
	})
}

// Benchmark LRU eviction
func BenchmarkLRU_Eviction(b *testing.B) {
	ctx := context.Background()
	redisClient := setupRedis(b)
	config := lru.DefaultConfig()
	config.EvictionBatchSize = 100

	manager := lru.NewManager(redisClient, config, "bench", nil, nil)

	for _, entries := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("%d_entries", entries), func(b *testing.B) {
			tenantID := uuid.New()

			// Pre-populate Redis with entries
			pipe := redisClient.GetClient().Pipeline()
			scoreKey := fmt.Sprintf("bench:lru:{%s}", tenantID.String())

			for i := 0; i < entries; i++ {
				key := fmt.Sprintf("bench:{%s}:q:%d", tenantID.String(), i)
				pipe.Set(ctx, key, "test data", 0)
				pipe.ZAdd(ctx, scoreKey, redis.Z{
					Score:  float64(time.Now().Add(-time.Duration(i) * time.Second).Unix()),
					Member: key,
				})
			}
			_, err := pipe.Exec(ctx)
			require.NoError(b, err)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Evict 10% of entries
				targetCount := entries - (entries / 10)
				err := manager.EvictForTenant(ctx, tenantID, int64(targetCount)*1024)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Benchmark concurrent operations
func BenchmarkCache_Concurrent(b *testing.B) {
	ctx := setupBenchmark(b)
	tenantCache, _ := setupTenantCache(b)

	// Mix of operations
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			switch i % 3 {
			case 0: // Set
				query := fmt.Sprintf("concurrent query %d", i)
				embedding := makeBenchmarkEmbedding(i)
				results := makeBenchmarkResults(i, 3)

				err := tenantCache.Set(ctx, query, embedding, results)
				if err != nil {
					b.Fatal(err)
				}

			case 1: // Get (hit)
				query := fmt.Sprintf("concurrent query %d", i/2)
				embedding := makeBenchmarkEmbedding(i / 2)

				_, err := tenantCache.Get(ctx, query, embedding)
				if err != nil {
					b.Fatal(err)
				}

			case 2: // Get (miss)
				query := fmt.Sprintf("missing query %d", i)
				embedding := makeBenchmarkEmbedding(i)

				_, err := tenantCache.Get(ctx, query, embedding)
				if err != nil {
					b.Fatal(err)
				}
			}
			i++
		}
	})
}

// Benchmark memory usage
func BenchmarkCache_Memory(b *testing.B) {
	ctx := setupBenchmark(b)
	tenantCache, _ := setupTenantCache(b)

	b.Run("1000_entries", func(b *testing.B) {
		for i := 0; i < 1000; i++ {
			query := fmt.Sprintf("memory test %d", i)
			embedding := makeBenchmarkEmbedding(i)
			results := makeBenchmarkResults(i, 10)

			err := tenantCache.Set(ctx, query, embedding, results)
			require.NoError(b, err)
		}

		b.ReportAllocs()
	})
}

// Benchmark similarity search performance
func BenchmarkCache_SimilaritySearch(b *testing.B) {
	redisClient := setupRedis(b)
	config := cache.DefaultConfig()
	config.Prefix = "bench_sim"

	baseCache, err := cache.NewSemanticCache(redisClient.GetClient(), config, nil)
	require.NoError(b, err)

	ctx := context.Background()

	// Pre-populate with queries
	baseEmbedding := makeBenchmarkEmbedding(0)
	for i := 0; i < 1000; i++ {
		query := fmt.Sprintf("similar query %d", i)
		embedding := make([]float32, len(baseEmbedding))
		copy(embedding, baseEmbedding)

		// Slightly modify embedding
		for j := 0; j < 10; j++ {
			embedding[j] = baseEmbedding[j] + float32(i)*0.0001
		}

		results := makeBenchmarkResults(i, 5)
		_ = baseCache.Set(ctx, query, embedding, results)
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
		_, _ = baseCache.Get(ctx, "test query", embedding)
	}
}

// Benchmark tenant isolation performance
func BenchmarkCache_TenantIsolation(b *testing.B) {
	redisClient := setupRedis(b)
	config := cache.DefaultConfig()
	config.Prefix = "bench_tenant"

	baseCache, err := cache.NewSemanticCache(redisClient.GetClient(), config, nil)
	require.NoError(b, err)

	numTenants := 10
	tenants := make([]uuid.UUID, numTenants)
	for i := range tenants {
		tenants[i] = uuid.New()
	}

	embeddings := make([][]float32, 100)
	for i := range embeddings {
		embeddings[i] = makeBenchmarkEmbedding(i)
	}
	results := makeBenchmarkResults(0, 5)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			tenantID := tenants[i%numTenants]
			ctx := auth.WithTenantID(context.Background(), tenantID)

			query := fmt.Sprintf("tenant query %d", i%100)
			embedding := embeddings[i%len(embeddings)]

			if i%3 == 0 {
				_ = baseCache.Set(ctx, query, embedding, results)
			} else {
				_, _ = baseCache.Get(ctx, query, embedding)
			}
			i++
		}
	})
}

// Benchmark cache stats collection
func BenchmarkCache_Stats(b *testing.B) {
	redisClient := setupRedis(b)
	config := cache.DefaultConfig()
	config.Prefix = "bench_stats"

	baseCache, err := cache.NewSemanticCache(redisClient.GetClient(), config, nil)
	require.NoError(b, err)

	ctx := context.Background()

	// Pre-populate with many entries
	for i := 0; i < 10000; i++ {
		query := fmt.Sprintf("stats query %d", i)
		embedding := makeBenchmarkEmbedding(i)
		results := makeBenchmarkResults(i, 3)
		_ = baseCache.Set(ctx, query, embedding, results)

		// Simulate some hits
		if i%10 == 0 {
			for j := 0; j < i%5; j++ {
				_, _ = baseCache.Get(ctx, query, embedding)
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = baseCache.GetStats()
	}
}

// Benchmark getting top queries
func BenchmarkCache_GetTopQueries(b *testing.B) {
	ctx := setupBenchmark(b)
	tenantCache, _ := setupTenantCache(b)

	// Pre-populate with entries having different hit counts
	for i := 0; i < 1000; i++ {
		query := fmt.Sprintf("top query %d", i)
		embedding := makeBenchmarkEmbedding(i)
		results := makeBenchmarkResults(i, 3)
		_ = tenantCache.Set(ctx, query, embedding, results)

		// Simulate varying hit counts
		hits := (1000 - i) / 10
		for j := 0; j < hits; j++ {
			_, _ = tenantCache.Get(ctx, query, embedding)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Since GetTopQueries is not implemented in TenantAwareCache,
		// we'll use GetStats instead for now
		stats := tenantCache.GetStats()
		_ = stats
	}
}

// Helper functions

func setupBenchmark(b *testing.B) context.Context {
	b.Helper()
	return auth.WithTenantID(context.Background(), uuid.New())
}

func setupRedis(b *testing.B) lru.RedisClient {
	b.Helper()

	client := redis.NewClient(&redis.Options{
		Addr: cache.GetTestRedisAddr(),
		DB:   15, // Test DB
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		b.Skip("Redis not available")
	}

	// Clear test DB
	client.FlushDB(context.Background())

	b.Cleanup(func() {
		_ = client.Close()
	})

	return &mockRedisClient{client}
}

func setupTenantCache(b *testing.B) (*cache.TenantAwareCache, uuid.UUID) {
	b.Helper()

	redisClient := setupRedis(b)
	config := cache.DefaultConfig()
	config.Prefix = "bench"

	baseCache, err := cache.NewSemanticCache(redisClient.GetClient(), config, nil)
	require.NoError(b, err)

	tenantID := uuid.New()
	configRepo := &mockConfigRepo{
		configs: map[string]*models.TenantConfig{
			tenantID.String(): {
				TenantID: tenantID.String(),
				Features: map[string]interface{}{
					"cache": map[string]interface{}{
						"enabled": true,
					},
				},
			},
		},
	}

	tenantCache := cache.NewTenantAwareCache(
		baseCache,
		configRepo,
		nil,
		"bench-encryption-key",
		observability.NewLogger("bench"),
		nil,
	)

	return tenantCache, tenantID
}

func makeBenchmarkEmbedding(seed int) []float32 {
	embedding := make([]float32, 384) // Standard embedding size
	for i := range embedding {
		embedding[i] = float32((seed+i)%100) / 100.0
	}
	return embedding
}

func makeBenchmarkResults(seed, count int) []cache.CachedSearchResult {
	results := make([]cache.CachedSearchResult, count)
	for i := 0; i < count; i++ {
		results[i] = cache.CachedSearchResult{
			ID:      fmt.Sprintf("doc-%d-%d", seed, i),
			Content: fmt.Sprintf("Result content for query %d, result %d", seed, i),
			Score:   float32(100-i) / 100.0,
			Metadata: map[string]interface{}{
				"source": "benchmark",
				"index":  i,
			},
		}
	}
	return results
}

// Mock implementations for benchmarks

type mockRedisClient struct {
	*redis.Client
}

func (m *mockRedisClient) GetClient() *redis.Client {
	return m.Client
}

func (m *mockRedisClient) Execute(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	return fn()
}

func (m *mockRedisClient) Get(ctx context.Context, key string) (string, error) {
	return m.Client.Get(ctx, key).Result()
}

func (m *mockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return m.Client.Set(ctx, key, value, expiration).Err()
}

func (m *mockRedisClient) Del(ctx context.Context, keys ...string) error {
	return m.Client.Del(ctx, keys...).Err()
}

func (m *mockRedisClient) Scan(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error) {
	cmd := m.Client.Scan(ctx, cursor, match, count)
	return cmd.Result()
}

func (m *mockRedisClient) Pipeline() redis.Pipeliner {
	return m.Client.Pipeline()
}

type mockConfigRepo struct {
	configs map[string]*models.TenantConfig
}

func (m *mockConfigRepo) GetByTenantID(ctx context.Context, tenantID string) (*models.TenantConfig, error) {
	if config, ok := m.configs[tenantID]; ok {
		return config, nil
	}
	return nil, nil
}

func (m *mockConfigRepo) Create(ctx context.Context, config *models.TenantConfig) error {
	m.configs[config.TenantID] = config
	return nil
}

func (m *mockConfigRepo) Update(ctx context.Context, config *models.TenantConfig) error {
	m.configs[config.TenantID] = config
	return nil
}

func (m *mockConfigRepo) Delete(ctx context.Context, tenantID string) error {
	delete(m.configs, tenantID)
	return nil
}

func (m *mockConfigRepo) List(ctx context.Context, limit, offset int) ([]*models.TenantConfig, error) {
	var configs []*models.TenantConfig
	for _, config := range m.configs {
		configs = append(configs, config)
	}
	return configs, nil
}

func (m *mockConfigRepo) Exists(ctx context.Context, tenantID string) (bool, error) {
	_, exists := m.configs[tenantID]
	return exists, nil
}
