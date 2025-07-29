package cache

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

func TestMigrationHelper(t *testing.T) {
	// Setup Redis client for testing
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use test database
	})
	defer redisClient.Close()

	// Clear test database
	redisClient.FlushDB(context.Background())

	// Create legacy cache
	legacyConfig := DefaultConfig()
	legacyConfig.Prefix = "test_legacy"
	legacyCache, err := NewSemanticCache(redisClient, legacyConfig, nil)
	require.NoError(t, err)

	// Create tenant cache
	tenantConfig := DefaultConfig()
	tenantConfig.Prefix = "test_tenant"
	tenantBaseCache, err := NewSemanticCache(redisClient, tenantConfig, nil)
	require.NoError(t, err)

	tenantCache := NewTenantAwareCache(
		tenantBaseCache,
		nil,
		nil,
		"test-encryption-key",
		observability.NewLogger("test"),
		nil,
	)

	// Create migration helper
	migrationHelper := NewMigrationHelper(
		legacyCache,
		tenantCache,
		observability.NewLogger("test"),
		nil,
	)

	t.Run("DualWrite", func(t *testing.T) {
		tenantID := uuid.New()
		ctx := auth.WithTenantID(context.Background(), tenantID)

		query := "migration test query"
		embedding := []float32{1, 2, 3}
		results := []CachedSearchResult{
			{ID: "1", Content: "Migration Result", Score: 0.95},
		}

		// Set through migration helper
		err = migrationHelper.Set(ctx, query, embedding, results)
		require.NoError(t, err)

		// Should be in both caches
		// Check legacy cache
		legacyEntry, err := legacyCache.Get(ctx, query, embedding)
		require.NoError(t, err)
		assert.NotNil(t, legacyEntry)
		assert.Equal(t, "Migration Result", legacyEntry.Results[0].Content)

		// Check tenant cache
		tenantEntry, err := tenantCache.Get(ctx, query, embedding)
		require.NoError(t, err)
		assert.NotNil(t, tenantEntry)
		assert.Equal(t, "Migration Result", tenantEntry.Results[0].Content)
	})

	t.Run("DualRead_TenantHit", func(t *testing.T) {
		tenantID := uuid.New()
		ctx := auth.WithTenantID(context.Background(), tenantID)

		query := "tenant hit query"
		embedding := []float32{4, 5, 6}
		results := []CachedSearchResult{
			{ID: "2", Content: "Tenant Hit", Score: 0.9},
		}

		// Set only in tenant cache
		err = tenantCache.Set(ctx, query, embedding, results)
		require.NoError(t, err)

		// Get through migration helper
		entry, err := migrationHelper.Get(ctx, query, embedding)
		require.NoError(t, err)
		assert.NotNil(t, entry)
		assert.Equal(t, "Tenant Hit", entry.Results[0].Content)
	})

	t.Run("DualRead_LegacyFallback", func(t *testing.T) {
		tenantID := uuid.New()
		ctx := auth.WithTenantID(context.Background(), tenantID)

		query := "legacy fallback query"
		embedding := []float32{7, 8, 9}
		results := []CachedSearchResult{
			{ID: "3", Content: "Legacy Fallback", Score: 0.85},
		}

		// Set only in legacy cache
		err = legacyCache.Set(ctx, query, embedding, results)
		require.NoError(t, err)

		// Get through migration helper
		entry, err := migrationHelper.Get(ctx, query, embedding)
		require.NoError(t, err)
		assert.NotNil(t, entry)
		assert.Equal(t, "Legacy Fallback", entry.Results[0].Content)

		// Wait a bit for async copy
		time.Sleep(100 * time.Millisecond)

		// Should now also be in tenant cache
		tenantEntry, err := tenantCache.Get(ctx, query, embedding)
		require.NoError(t, err)
		assert.NotNil(t, tenantEntry)
		assert.Equal(t, "Legacy Fallback", tenantEntry.Results[0].Content)
	})

	t.Run("Delete", func(t *testing.T) {
		tenantID := uuid.New()
		ctx := auth.WithTenantID(context.Background(), tenantID)

		query := "delete test query"
		embedding := []float32{10, 11, 12}
		results := []CachedSearchResult{
			{ID: "4", Content: "To Delete", Score: 0.8},
		}

		// Set through migration helper
		err = migrationHelper.Set(ctx, query, embedding, results)
		require.NoError(t, err)

		// Delete through migration helper
		err = migrationHelper.Delete(ctx, query)
		require.NoError(t, err)

		// Should be gone from both caches
		legacyEntry, err := legacyCache.Get(ctx, query, embedding)
		assert.NoError(t, err)
		assert.Nil(t, legacyEntry)

		tenantEntry, err := tenantCache.Get(ctx, query, embedding)
		assert.NoError(t, err)
		assert.Nil(t, tenantEntry)
	})

	t.Run("NoTenantID", func(t *testing.T) {
		// Context without tenant ID
		ctx := context.Background()

		query := "no tenant query"
		embedding := []float32{13, 14, 15}
		results := []CachedSearchResult{
			{ID: "5", Content: "No Tenant", Score: 0.75},
		}

		// Set through migration helper
		err = migrationHelper.Set(ctx, query, embedding, results)
		require.NoError(t, err)

		// Should only be in legacy cache
		legacyEntry, err := legacyCache.Get(ctx, query, embedding)
		require.NoError(t, err)
		assert.NotNil(t, legacyEntry)
		assert.Equal(t, "No Tenant", legacyEntry.Results[0].Content)

		// Not in tenant cache (no tenant ID)
		tenantID := uuid.New()
		tenantCtx := auth.WithTenantID(ctx, tenantID)
		tenantEntry, err := tenantCache.Get(tenantCtx, query, embedding)
		assert.NoError(t, err)
		assert.Nil(t, tenantEntry)
	})
}
