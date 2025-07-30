package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache/tenant"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// mockTenantConfigRepo implements repository.TenantConfigRepository for testing
type mockTenantConfigRepo struct {
	configs map[string]*models.TenantConfig
}

func (m *mockTenantConfigRepo) GetByTenantID(ctx context.Context, tenantID string) (*models.TenantConfig, error) {
	if config, ok := m.configs[tenantID]; ok {
		return config, nil
	}
	return nil, ErrTenantNotFound
}

func (m *mockTenantConfigRepo) Create(ctx context.Context, config *models.TenantConfig) error {
	m.configs[config.TenantID] = config
	return nil
}

func (m *mockTenantConfigRepo) Update(ctx context.Context, config *models.TenantConfig) error {
	m.configs[config.TenantID] = config
	return nil
}

func (m *mockTenantConfigRepo) Delete(ctx context.Context, tenantID string) error {
	delete(m.configs, tenantID)
	return nil
}

func (m *mockTenantConfigRepo) List(ctx context.Context, limit, offset int) ([]*models.TenantConfig, error) {
	var configs []*models.TenantConfig
	for _, config := range m.configs {
		configs = append(configs, config)
	}
	return configs, nil
}

func (m *mockTenantConfigRepo) Exists(ctx context.Context, tenantID string) (bool, error) {
	_, exists := m.configs[tenantID]
	return exists, nil
}

func TestTenantIsolation(t *testing.T) {
	// Setup Redis client for testing
	redisClient := GetTestRedisClient(t)
	defer func() { _ = redisClient.Close() }()

	// Clear test database
	if err := redisClient.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("Failed to flush Redis: %v", err)
	}

	// Create base cache
	config := DefaultConfig()
	config.Prefix = "test_tenant"
	baseCache, err := NewSemanticCache(redisClient, config, nil)
	require.NoError(t, err)

	// Create tenant config repository
	configRepo := &mockTenantConfigRepo{
		configs: make(map[string]*models.TenantConfig),
	}

	// Create tenant-aware cache
	cache := NewTenantAwareCache(
		baseCache,
		configRepo,
		nil, // No rate limiter for tests
		"test-encryption-key",
		observability.NewLogger("test"),
		nil, // No metrics for tests
	)

	// Test with real auth context
	tenant1 := uuid.New()
	tenant2 := uuid.New()
	ctx1 := auth.WithTenantID(context.Background(), tenant1)
	ctx2 := auth.WithTenantID(context.Background(), tenant2)

	// Create tenant configs
	configRepo.configs[tenant1.String()] = &models.TenantConfig{
		TenantID: tenant1.String(),
		Features: map[string]interface{}{
			"cache": map[string]interface{}{
				"enabled": true,
			},
		},
	}
	configRepo.configs[tenant2.String()] = &models.TenantConfig{
		TenantID: tenant2.String(),
		Features: map[string]interface{}{
			"cache": map[string]interface{}{
				"enabled": true,
			},
		},
	}

	// Test data
	query := "test query"
	embedding := []float32{1, 2, 3}
	results := []CachedSearchResult{
		{ID: "1", Content: "Result 1", Score: 0.95},
	}

	// Set in tenant 1
	err = cache.Set(ctx1, query, embedding, results)
	require.NoError(t, err)

	// Should find in tenant 1
	entry, err := cache.Get(ctx1, query, embedding)
	require.NoError(t, err)
	assert.NotNil(t, entry)
	assert.Equal(t, query, entry.Query)

	// Should not find in tenant 2
	entry, err = cache.Get(ctx2, query, embedding)
	assert.NoError(t, err) // No error, just cache miss
	assert.Nil(t, entry)

	// Set same query in tenant 2 with different results
	results2 := []CachedSearchResult{
		{ID: "2", Content: "Different Result", Score: 0.85},
	}
	err = cache.Set(ctx2, query, embedding, results2)
	require.NoError(t, err)

	// Verify each tenant sees their own data
	entry1, err := cache.Get(ctx1, query, embedding)
	require.NoError(t, err)
	assert.Equal(t, "Result 1", entry1.Results[0].Content)

	entry2, err := cache.Get(ctx2, query, embedding)
	require.NoError(t, err)
	assert.Equal(t, "Different Result", entry2.Results[0].Content)
}

func TestCacheModes(t *testing.T) {
	// Setup Redis client for testing
	redisClient := GetTestRedisClient(t)
	defer func() { _ = redisClient.Close() }()

	// Clear test database
	if err := redisClient.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("Failed to flush Redis: %v", err)
	}

	// Create base cache
	config := DefaultConfig()
	config.Prefix = "test_mode"
	baseCache, err := NewSemanticCache(redisClient, config, nil)
	require.NoError(t, err)

	// Create tenant-aware cache
	cache := NewTenantAwareCache(
		baseCache,
		nil, // No config repo for this test
		nil, // No rate limiter
		"test-encryption-key",
		observability.NewLogger("test"),
		nil, // No metrics
	)

	// Test data
	query := "legacy query"
	embedding := []float32{1, 2, 3}
	results := []CachedSearchResult{
		{ID: "1", Content: "Legacy Result", Score: 0.95},
	}

	t.Run("NoTenantID", func(t *testing.T) {
		// Context without tenant ID
		ctx := context.Background()

		// Should fail without tenant ID (cache is always tenant-isolated)
		err = cache.Set(ctx, query, embedding, results)
		require.Error(t, err)
		assert.Equal(t, ErrNoTenantID, err)

		entry, err := cache.Get(ctx, query, embedding)
		require.Error(t, err)
		assert.Equal(t, ErrNoTenantID, err)
		assert.Nil(t, entry)
	})

	t.Run("WithTenantID", func(t *testing.T) {
		// Cache is always tenant-isolated now

		// Context without tenant ID
		ctx := context.Background()

		// Should fail without tenant ID in tenant-only mode
		err = cache.Set(ctx, query, embedding, results)
		assert.Equal(t, ErrNoTenantID, err)

		entry, err := cache.Get(ctx, query, embedding)
		assert.Equal(t, ErrNoTenantID, err)
		assert.Nil(t, entry)
	})
}

func TestFeatureFlags(t *testing.T) {
	// Setup Redis client for testing
	redisClient := GetTestRedisClient(t)
	defer func() { _ = redisClient.Close() }()

	// Clear test database
	if err := redisClient.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("Failed to flush Redis: %v", err)
	}

	// Create base cache
	config := DefaultConfig()
	config.Prefix = "test_features"
	baseCache, err := NewSemanticCache(redisClient, config, nil)
	require.NoError(t, err)

	// Create tenant config repository
	configRepo := &mockTenantConfigRepo{
		configs: make(map[string]*models.TenantConfig),
	}

	// Create tenant-aware cache
	cache := NewTenantAwareCache(
		baseCache,
		configRepo,
		nil,
		"test-encryption-key",
		observability.NewLogger("test"),
		nil,
	)

	tenantID := uuid.New()
	ctx := auth.WithTenantID(context.Background(), tenantID)

	// Test with cache disabled
	configRepo.configs[tenantID.String()] = &models.TenantConfig{
		TenantID: tenantID.String(),
		Features: map[string]interface{}{
			"cache": map[string]interface{}{
				"enabled": false,
			},
		},
	}

	query := "test query"
	embedding := []float32{1, 2, 3}
	results := []CachedSearchResult{{ID: "1", Content: "Test", Score: 0.9}}

	err = cache.Set(ctx, query, embedding, results)
	assert.Equal(t, ErrFeatureDisabled, err)

	entry, err := cache.Get(ctx, query, embedding)
	assert.Equal(t, ErrFeatureDisabled, err)
	assert.Nil(t, entry)
}

func TestTenantConfigParsing(t *testing.T) {
	baseConfig := &models.TenantConfig{
		TenantID: "test-tenant",
		Features: map[string]interface{}{
			"cache": map[string]interface{}{
				"max_entries":     float64(5000),
				"max_bytes":       float64(50 * 1024 * 1024),
				"ttl_seconds":     float64(1800),
				"enabled":         false,
				"cache_warming":   true,
				"async_eviction":  false,
				"metrics_enabled": true,
			},
		},
	}

	cacheConfig := tenant.ParseFromTenantConfig(baseConfig)

	assert.Equal(t, 5000, cacheConfig.MaxCacheEntries)
	assert.Equal(t, int64(50*1024*1024), cacheConfig.MaxCacheBytes)
	assert.Equal(t, 30*time.Minute, cacheConfig.CacheTTLOverride)
	assert.False(t, cacheConfig.EnabledFeatures.EnableSemanticCache)
	assert.True(t, cacheConfig.EnabledFeatures.EnableCacheWarming)
	assert.False(t, cacheConfig.EnabledFeatures.EnableAsyncEviction)
	assert.True(t, cacheConfig.EnabledFeatures.EnableMetrics)
}

func TestClearTenant(t *testing.T) {
	// Setup Redis client for testing
	redisClient := GetTestRedisClient(t)
	defer func() { _ = redisClient.Close() }()

	// Clear test database
	if err := redisClient.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("Failed to flush Redis: %v", err)
	}

	// Create base cache
	config := DefaultConfig()
	config.Prefix = "test_clear"
	baseCache, err := NewSemanticCache(redisClient, config, nil)
	require.NoError(t, err)

	// Create tenant-aware cache
	cache := NewTenantAwareCache(
		baseCache,
		nil,
		nil,
		"test-encryption-key",
		observability.NewLogger("test"),
		nil,
	)

	tenantID := uuid.New()
	ctx := auth.WithTenantID(context.Background(), tenantID)

	// Add multiple entries for the tenant
	for i := 0; i < 5; i++ {
		query := fmt.Sprintf("query %d", i)
		embedding := []float32{float32(i), float32(i + 1), float32(i + 2)}
		results := []CachedSearchResult{
			{ID: fmt.Sprintf("%d", i), Content: fmt.Sprintf("Result %d", i), Score: 0.9},
		}
		err = cache.Set(ctx, query, embedding, results)
		require.NoError(t, err)
	}

	// Verify entries exist
	entry, err := cache.Get(ctx, "query 0", []float32{0, 1, 2})
	require.NoError(t, err)
	assert.NotNil(t, entry)

	// Clear tenant
	err = cache.ClearTenant(ctx, tenantID)
	require.NoError(t, err)

	// Verify entries are gone
	entry, err = cache.Get(ctx, "query 0", []float32{0, 1, 2})
	assert.NoError(t, err)
	assert.Nil(t, entry)
}
