package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

func TestDegradedMode(t *testing.T) {
	ctx := context.Background()
	logger := observability.NewLogger("test")

	t.Run("AutomaticFailover", func(t *testing.T) {
		// Create a Redis client that will fail
		redisClient := redis.NewClient(&redis.Options{
			Addr:         "invalid:6379",
			DialTimeout:  100 * time.Millisecond,
			ReadTimeout:  100 * time.Millisecond,
			WriteTimeout: 100 * time.Millisecond,
		})

		// Create cache with degraded mode support
		cache, err := NewSemanticCache(redisClient, DefaultConfig(), logger)
		require.NoError(t, err)
		defer func() {
			_ = cache.Shutdown(ctx)
		}()

		// Test data
		query := "test query"
		embedding := []float32{0.1, 0.2, 0.3}
		results := []CachedSearchResult{
			{ID: "1", Score: 0.9, Content: "test content"},
		}

		// Set should trigger degraded mode due to Redis failure
		err = cache.Set(ctx, query, embedding, results)
		assert.NoError(t, err) // Should succeed using fallback

		// Verify cache is in degraded mode
		assert.True(t, cache.degradedMode.Load())

		// Get should work in degraded mode
		entry, err := cache.Get(ctx, query, embedding)
		assert.NoError(t, err)
		assert.NotNil(t, entry)
		assert.Equal(t, query, entry.Query)
		assert.Len(t, entry.Results, 1)
	})

	t.Run("RecoveryFromDegradedMode", func(t *testing.T) {
		// Start with a working Redis
		redisClient := redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
			DB:   15, // Use test DB
		})

		// Ensure Redis is accessible
		err := redisClient.Ping(ctx).Err()
		if err != nil {
			t.Skip("Redis not available")
		}

		// Create cache
		config := DefaultConfig()
		config.Prefix = "test_degraded"
		cache, err := NewSemanticCache(redisClient, config, logger)
		require.NoError(t, err)
		defer func() {
			_ = cache.Shutdown(ctx)
		}()

		// Clear test data
		_ = cache.Clear(ctx)

		// Force degraded mode
		cache.degradedMode.Store(true)

		// Verify recovery checker restores normal mode
		cache.checkAndRecoverFromDegradedMode(ctx)
		time.Sleep(100 * time.Millisecond)

		assert.False(t, cache.degradedMode.Load())
	})

	t.Run("FallbackCacheLimits", func(t *testing.T) {
		// Create fallback cache with small limit
		fallback := NewFallbackCache(5, 1*time.Hour, logger, nil)

		// Add entries up to limit
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("key%d", i)
			entry := &CacheEntry{
				Query: fmt.Sprintf("query%d", i),
			}
			err := fallback.Set(ctx, key, entry)
			assert.NoError(t, err)
		}

		// Check that only 5 entries remain (LRU eviction)
		count := 0
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("key%d", i)
			entry, _ := fallback.Get(ctx, key)
			if entry != nil {
				count++
			}
		}
		assert.Equal(t, 5, count)
	})
}

func TestDegradedModeCache(t *testing.T) {
	ctx := context.Background()
	logger := observability.NewLogger("test")

	t.Run("NilPrimaryCache", func(t *testing.T) {
		// Create degraded mode cache with nil primary
		degradedCache := NewDegradedModeCache(nil, logger)
		assert.True(t, degradedCache.IsInDegradedMode())

		// Should use fallback for all operations
		query := "test query"
		embedding := []float32{0.1, 0.2, 0.3}
		results := []CachedSearchResult{
			{ID: "1", Score: 0.9},
		}

		err := degradedCache.Set(ctx, query, embedding, results)
		assert.NoError(t, err)

		entry, err := degradedCache.Get(ctx, query, embedding)
		assert.NoError(t, err)
		assert.NotNil(t, entry)
	})

	t.Run("HealthCheckRecovery", func(t *testing.T) {
		// Mock primary cache
		primary := &SemanticCache{
			redis: &ResilientRedisClient{
				client: redis.NewClient(&redis.Options{
					Addr: "localhost:6379",
					DB:   15,
				}),
			},
			config: DefaultConfig(),
		}

		degradedCache := NewDegradedModeCache(primary, logger)

		// Start in degraded mode
		degradedCache.enterDegradedMode()
		assert.True(t, degradedCache.IsInDegradedMode())

		// Health check should recover if Redis is available
		if primary.redis.Health(ctx) == nil {
			degradedCache.checkPrimaryHealth(ctx)
			assert.False(t, degradedCache.IsInDegradedMode())
		}
	})
}

// TestRedisFallbackScenarios tests various Redis failure scenarios
func TestRedisFallbackScenarios(t *testing.T) {
	ctx := context.Background()
	logger := observability.NewLogger("test")

	scenarios := []struct {
		name        string
		setupRedis  func() *redis.Client
		expectError bool
	}{
		{
			name: "ConnectionRefused",
			setupRedis: func() *redis.Client {
				return redis.NewClient(&redis.Options{
					Addr:        "localhost:9999", // Invalid port
					DialTimeout: 100 * time.Millisecond,
				})
			},
		},
		{
			name: "Timeout",
			setupRedis: func() *redis.Client {
				return redis.NewClient(&redis.Options{
					Addr:        "10.255.255.1:6379", // Non-routable IP
					DialTimeout: 100 * time.Millisecond,
				})
			},
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			redisClient := sc.setupRedis()
			cache, err := NewSemanticCache(redisClient, DefaultConfig(), logger)
			require.NoError(t, err)
			defer func() {
				_ = cache.Shutdown(ctx)
			}()

			// Operations should succeed despite Redis failure
			err = cache.Set(ctx, "query", []float32{0.1}, []CachedSearchResult{{ID: "1"}})
			assert.NoError(t, err)
			assert.True(t, cache.degradedMode.Load())
		})
	}
}
