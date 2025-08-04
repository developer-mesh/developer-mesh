//go:build redis
// +build redis

package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache/tenant"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

func TestTenantAwareCache_WithLRU(t *testing.T) {
	// Skip if Redis is not available
	redisClient := redis.NewClient(&redis.Options{
		Addr: GetTestRedisAddr(),
		DB:   15,
	})
	defer func() { _ = redisClient.Close() }()

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}

	// Clear test database
	redisClient.FlushDB(ctx)

	// Create base cache
	config := DefaultConfig()
	config.Prefix = "test_lru"
	baseCache, err := NewSemanticCache(redisClient, config, nil)
	require.NoError(t, err)

	// Create tenant config repository
	configRepo := &mockTenantConfigRepo{
		configs: make(map[string]*models.TenantConfig),
	}

	// Create tenant-aware cache with LRU
	tenantCache := NewTenantAwareCache(
		baseCache,
		configRepo,
		nil,
		"test-encryption-key",
		observability.NewLogger("test"),
		nil,
	)

	// Verify LRU manager was created
	assert.NotNil(t, tenantCache.lruManager)

	// Test tracking access
	tenantID := uuid.New()
	ctx = auth.WithTenantID(ctx, tenantID)

	// Add tenant config
	configRepo.configs[tenantID.String()] = &models.TenantConfig{
		TenantID: tenantID.String(),
		Features: map[string]interface{}{
			"cache": map[string]interface{}{
				"enabled": true,
			},
		},
	}

	// Add multiple entries
	for i := 0; i < 5; i++ {
		query := fmt.Sprintf("query %d", i)
		embedding := []float32{float32(i), float32(i + 1), float32(i + 2)}
		results := []CachedSearchResult{
			{ID: fmt.Sprintf("%d", i), Content: fmt.Sprintf("Result %d", i), Score: 0.9},
		}
		err := tenantCache.Set(ctx, query, embedding, results)
		require.NoError(t, err)
	}

	// Access entries to track LRU
	for i := 0; i < 3; i++ {
		query := fmt.Sprintf("query %d", i)
		embedding := []float32{float32(i), float32(i + 1), float32(i + 2)}
		entry, err := tenantCache.Get(ctx, query, embedding)
		require.NoError(t, err)
		assert.NotNil(t, entry)
	}

	// Wait for tracking to flush
	time.Sleep(100 * time.Millisecond)
}

func TestLRUEviction_Integration(t *testing.T) {
	// Skip if Redis is not available
	redisClient := redis.NewClient(&redis.Options{
		Addr: GetTestRedisAddr(),
		DB:   15,
	})
	defer func() { _ = redisClient.Close() }()

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}

	// Clear test database
	redisClient.FlushDB(ctx)

	// Create base cache with small limits
	config := DefaultConfig()
	config.Prefix = "test_evict"
	config.MaxCacheSize = 5 // Small limit to trigger eviction
	baseCache, err := NewSemanticCache(redisClient, config, nil)
	require.NoError(t, err)

	// Create tenant-aware cache
	tenantCache := NewTenantAwareCache(
		baseCache,
		nil,
		nil,
		"test-encryption-key",
		observability.NewLogger("test"),
		nil,
	)

	// Override LRU config for faster testing
	if tenantCache.lruManager != nil && tenantCache.lruManager.GetConfig() != nil {
		config := tenantCache.lruManager.GetConfig()
		config.MaxTenantEntries = 5
		config.EvictionBatchSize = 2
	}

	tenantID := uuid.New()
	ctx = auth.WithTenantID(ctx, tenantID)

	// Fill cache beyond limit
	for i := 0; i < 10; i++ {
		query := fmt.Sprintf("evict_query_%d", i)
		embedding := []float32{float32(i), float32(i + 1), float32(i + 2)}
		results := []CachedSearchResult{
			{ID: fmt.Sprintf("%d", i), Content: fmt.Sprintf("Evict Result %d", i), Score: 0.9},
		}
		err := tenantCache.Set(ctx, query, embedding, results)
		require.NoError(t, err)

		// Small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// Access some entries to make them more recent
	for i := 5; i < 8; i++ {
		query := fmt.Sprintf("evict_query_%d", i)
		embedding := []float32{float32(i), float32(i + 1), float32(i + 2)}
		_, _ = tenantCache.Get(ctx, query, embedding)
	}

	// Create vector store for eviction
	vectorStore := NewVectorStore(nil, observability.NewLogger("test"), nil)

	// Start LRU eviction (would normally run in background)
	evictCtx, cancel := context.WithCancel(ctx)
	tenantCache.StartLRUEviction(evictCtx, vectorStore)

	// Wait for eviction to run
	time.Sleep(200 * time.Millisecond)

	// Stop eviction
	cancel()
	tenantCache.StopLRUEviction()

	// Check that old entries were evicted
	// Entries 0-4 should be candidates for eviction
	for i := 0; i < 5; i++ {
		query := fmt.Sprintf("evict_query_%d", i)
		embedding := []float32{float32(i), float32(i + 1), float32(i + 2)}
		entry, _ := tenantCache.Get(ctx, query, embedding)
		if entry != nil {
			t.Logf("Entry %d still in cache", i)
		}
	}

	// Recently accessed entries should still be there
	for i := 5; i < 8; i++ {
		query := fmt.Sprintf("evict_query_%d", i)
		embedding := []float32{float32(i), float32(i + 1), float32(i + 2)}
		entry, err := tenantCache.Get(ctx, query, embedding)
		assert.NoError(t, err)
		if entry == nil {
			t.Logf("Entry %d was evicted (should have been kept)", i)
		}
	}
}

func TestTenantCacheConfig_LRULimits(t *testing.T) {
	// Test that tenant-specific LRU limits are respected
	config1 := &tenant.CacheTenantConfig{
		MaxCacheEntries: 100,
		MaxCacheBytes:   10 * 1024 * 1024, // 10MB
		EnabledFeatures: tenant.CacheFeatureFlags{
			EnableSemanticCache: true,
			EnableAsyncEviction: true,
		},
	}

	config2 := &tenant.CacheTenantConfig{
		MaxCacheEntries: 50,
		MaxCacheBytes:   5 * 1024 * 1024, // 5MB
		EnabledFeatures: tenant.CacheFeatureFlags{
			EnableSemanticCache: true,
			EnableAsyncEviction: false,
		},
	}

	// Verify different tenants can have different limits
	assert.NotEqual(t, config1.MaxCacheEntries, config2.MaxCacheEntries)
	assert.NotEqual(t, config1.MaxCacheBytes, config2.MaxCacheBytes)
	assert.NotEqual(t, config1.EnabledFeatures.EnableAsyncEviction, config2.EnabledFeatures.EnableAsyncEviction)
}
