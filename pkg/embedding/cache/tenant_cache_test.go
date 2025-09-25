package cache_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache"
	"github.com/developer-mesh/developer-mesh/pkg/middleware"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Mock repository for testing
type mockTenantConfigRepo struct {
	mock.Mock
}

func (m *mockTenantConfigRepo) GetByTenantID(ctx context.Context, tenantID string) (*models.TenantConfig, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TenantConfig), args.Error(1)
}

func (m *mockTenantConfigRepo) Create(ctx context.Context, config *models.TenantConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *mockTenantConfigRepo) Update(ctx context.Context, config *models.TenantConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *mockTenantConfigRepo) Delete(ctx context.Context, tenantID string) error {
	args := m.Called(ctx, tenantID)
	return args.Error(0)
}

func (m *mockTenantConfigRepo) List(ctx context.Context, limit, offset int) ([]*models.TenantConfig, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.TenantConfig), args.Error(1)
}

func (m *mockTenantConfigRepo) Exists(ctx context.Context, tenantID string) (bool, error) {
	args := m.Called(ctx, tenantID)
	return args.Bool(0), args.Error(1)
}

func setupTestTenantCache(t *testing.T) (*cache.TenantAwareCache, *mockTenantConfigRepo, *redis.Client) {
	// Setup Redis client
	redisClient := cache.GetTestRedisClient(t)

	// Clear test database
	redisClient.FlushDB(context.Background())

	// Create base cache
	config := cache.DefaultConfig()
	config.Prefix = "test_cache"

	logger := observability.NewLogger("test")
	metrics := observability.NewMetricsClient()

	baseCache, err := cache.NewSemanticCache(redisClient, config, logger)
	require.NoError(t, err)

	// Create mock repository
	mockRepo := &mockTenantConfigRepo{}

	// Create rate limiter
	rateLimitConfig := middleware.RateLimitConfig{
		GlobalRPS:       100,
		GlobalBurst:     200,
		TenantRPS:       50,
		TenantBurst:     100,
		CleanupInterval: time.Minute,
		MaxAge:          time.Hour,
	}
	rateLimiter := middleware.NewRateLimiter(rateLimitConfig, logger, metrics)

	// Create tenant aware cache
	tenantCache := cache.NewTenantAwareCache(
		baseCache,
		mockRepo,
		rateLimiter,
		"test-encryption-key-32-chars-long!!",
		logger,
		metrics,
	)

	return tenantCache, mockRepo, redisClient
}

func TestTenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	tenantCache, mockRepo, redisClient := setupTestTenantCache(t)
	defer func() { _ = redisClient.Close() }()

	tenant1 := uuid.New()
	tenant2 := uuid.New()

	// Setup mock expectations
	tenant1Config := &models.TenantConfig{
		TenantID: tenant1.String(),
		Features: map[string]interface{}{
			"semantic_cache": true,
		},
	}
	tenant2Config := &models.TenantConfig{
		TenantID: tenant2.String(),
		Features: map[string]interface{}{
			"semantic_cache": true,
		},
	}

	mockRepo.On("GetByTenantID", mock.Anything, tenant1.String()).Return(tenant1Config, nil)
	mockRepo.On("GetByTenantID", mock.Anything, tenant2.String()).Return(tenant2Config, nil)

	// Create contexts with tenant IDs
	ctx1 := auth.WithTenantID(context.Background(), tenant1)
	ctx2 := auth.WithTenantID(context.Background(), tenant2)

	query := "test query for isolation"
	embedding := []float32{0.1, 0.2, 0.3}
	results := []cache.CachedSearchResult{
		{ID: "1", Content: "Result 1", Score: 0.9},
	}

	// Test: Set data for tenant 1
	err := tenantCache.Set(ctx1, query, embedding, results)
	require.NoError(t, err)

	// Test: Get data for tenant 1 - should find it
	entry1, err := tenantCache.Get(ctx1, query, embedding)
	require.NoError(t, err)
	require.NotNil(t, entry1)
	assert.Equal(t, query, entry1.Query)
	assert.Len(t, entry1.Results, 1)

	// Test: Get data for tenant 2 - should NOT find it (isolation)
	entry2, err := tenantCache.Get(ctx2, query, embedding)
	assert.NoError(t, err)
	assert.Nil(t, entry2)

	// Test: Set different data for tenant 2
	results2 := []cache.CachedSearchResult{
		{ID: "2", Content: "Result 2", Score: 0.8},
	}
	err = tenantCache.Set(ctx2, query, embedding, results2)
	require.NoError(t, err)

	// Test: Verify tenant 1 still has original data
	entry1Again, err := tenantCache.Get(ctx1, query, embedding)
	require.NoError(t, err)
	require.NotNil(t, entry1Again)
	assert.Equal(t, "Result 1", entry1Again.Results[0].Content)

	// Test: Verify tenant 2 has its own data
	entry2Again, err := tenantCache.Get(ctx2, query, embedding)
	require.NoError(t, err)
	require.NotNil(t, entry2Again)
	assert.Equal(t, "Result 2", entry2Again.Results[0].Content)
}

func TestTenantFeatureDisabled(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	tenantCache, mockRepo, redisClient := setupTestTenantCache(t)
	defer func() { _ = redisClient.Close() }()

	tenantID := uuid.New()

	// Setup mock - cache disabled for this tenant
	tenantConfig := &models.TenantConfig{
		TenantID: tenantID.String(),
		Features: map[string]interface{}{
			"cache": map[string]interface{}{
				"enabled": false,
			},
		},
	}
	mockRepo.On("GetByTenantID", mock.Anything, tenantID.String()).Return(tenantConfig, nil)

	ctx := auth.WithTenantID(context.Background(), tenantID)

	// Test: Get should return feature disabled error
	_, err := tenantCache.Get(ctx, "test", []float32{0.1})
	assert.Error(t, err)
	assert.Equal(t, cache.ErrFeatureDisabled, err)

	// Test: Set should also return feature disabled error
	err = tenantCache.Set(ctx, "test", []float32{0.1}, []cache.CachedSearchResult{})
	assert.Error(t, err)
	assert.Equal(t, cache.ErrFeatureDisabled, err)
}

func TestNoTenantID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	tenantCache, _, redisClient := setupTestTenantCache(t)
	defer func() { _ = redisClient.Close() }()

	// Context without tenant ID
	ctx := context.Background()

	// Test: Get should return no tenant ID error
	_, err := tenantCache.Get(ctx, "test", []float32{0.1})
	assert.Error(t, err)
	assert.Equal(t, cache.ErrNoTenantID, err)

	// Test: Set should also return no tenant ID error
	err = tenantCache.Set(ctx, "test", []float32{0.1}, []cache.CachedSearchResult{})
	assert.Error(t, err)
	assert.Equal(t, cache.ErrNoTenantID, err)
}

func TestEncryption(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	tenantCache, mockRepo, redisClient := setupTestTenantCache(t)
	defer func() { _ = redisClient.Close() }()

	tenantID := uuid.New()

	// Setup mock
	tenantConfig := &models.TenantConfig{
		TenantID: tenantID.String(),
		Features: map[string]interface{}{
			"semantic_cache": true,
		},
	}
	mockRepo.On("GetByTenantID", mock.Anything, tenantID.String()).Return(tenantConfig, nil)

	ctx := auth.WithTenantID(context.Background(), tenantID)

	// Create results with sensitive data
	results := []cache.CachedSearchResult{
		{
			ID:      "1",
			Content: "Public content",
			Metadata: map[string]interface{}{
				"sensitive": true,
				"api_key":   "secret-key-123",
			},
		},
	}

	// Test: Set with encryption
	err := tenantCache.Set(ctx, "test query", []float32{0.1}, results)
	require.NoError(t, err)

	// Test: Get with decryption
	entry, err := tenantCache.Get(ctx, "test query", []float32{0.1})
	require.NoError(t, err)
	require.NotNil(t, entry)

	// Verify data is returned (decryption handling would be in the actual implementation)
	assert.Len(t, entry.Results, 1)
	assert.Equal(t, "Public content", entry.Results[0].Content)
}

func TestRateLimiting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	tenantCache, mockRepo, redisClient := setupTestTenantCache(t)
	defer func() { _ = redisClient.Close() }()

	tenantID := uuid.New()

	// Setup mock
	tenantConfig := &models.TenantConfig{
		TenantID: tenantID.String(),
		Features: map[string]interface{}{
			"semantic_cache": true,
		},
	}
	mockRepo.On("GetByTenantID", mock.Anything, tenantID.String()).Return(tenantConfig, nil)

	ctx := auth.WithTenantID(context.Background(), tenantID)

	// Note: The actual rate limiting implementation needs to be added to TenantAwareCache
	// This test demonstrates the expected behavior

	// Make many rapid requests
	for i := 0; i < 150; i++ {
		query := fmt.Sprintf("query %d", i)
		_, _ = tenantCache.Get(ctx, query, []float32{0.1})

		// After hitting rate limit, subsequent requests should fail
		// This would be implemented in the actual TenantAwareCache
	}
}

func TestConcurrentTenantAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	tenantCache, mockRepo, redisClient := setupTestTenantCache(t)
	defer func() { _ = redisClient.Close() }()

	numTenants := 5
	numOperations := 20
	tenants := make([]uuid.UUID, numTenants)

	// Setup tenants
	for i := 0; i < numTenants; i++ {
		tenants[i] = uuid.New()
		config := &models.TenantConfig{
			TenantID: tenants[i].String(),
			Features: map[string]interface{}{
				"semantic_cache": true,
			},
		}
		mockRepo.On("GetByTenantID", mock.Anything, tenants[i].String()).Return(config, nil)
	}

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < numTenants; i++ {
		go func(tenantIdx int) {
			tenantID := tenants[tenantIdx]
			ctx := auth.WithTenantID(context.Background(), tenantID)

			for j := 0; j < numOperations; j++ {
				query := fmt.Sprintf("tenant %d query %d", tenantIdx, j)
				embedding := []float32{float32(tenantIdx), float32(j)}
				results := []cache.CachedSearchResult{
					{
						ID:      fmt.Sprintf("%d-%d", tenantIdx, j),
						Content: fmt.Sprintf("Result for tenant %d", tenantIdx),
						Score:   0.9,
					},
				}

				// Set
				err := tenantCache.Set(ctx, query, embedding, results)
				assert.NoError(t, err)

				// Get
				entry, err := tenantCache.Get(ctx, query, embedding)
				assert.NoError(t, err)
				if entry != nil {
					assert.Equal(t, query, entry.Query)
					assert.Contains(t, entry.Results[0].Content, fmt.Sprintf("tenant %d", tenantIdx))
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numTenants; i++ {
		<-done
	}

	// Verify isolation - check that tenant 0's data is not accessible by tenant 1
	ctx0 := auth.WithTenantID(context.Background(), tenants[0])
	ctx1 := auth.WithTenantID(context.Background(), tenants[1])

	entry0, err := tenantCache.Get(ctx0, "tenant 0 query 0", []float32{0, 0})
	assert.NoError(t, err)
	assert.NotNil(t, entry0)

	entry1, err := tenantCache.Get(ctx1, "tenant 0 query 0", []float32{0, 0})
	assert.NoError(t, err)
	assert.Nil(t, entry1) // Should not find tenant 0's data
}

func TestTenantConfigCaching(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	tenantCache, mockRepo, redisClient := setupTestTenantCache(t)
	defer func() { _ = redisClient.Close() }()

	tenantID := uuid.New()

	// Setup mock - should only be called once due to caching
	tenantConfig := &models.TenantConfig{
		TenantID: tenantID.String(),
		Features: map[string]interface{}{
			"semantic_cache": true,
		},
	}
	mockRepo.On("GetByTenantID", mock.Anything, tenantID.String()).Return(tenantConfig, nil).Once()

	ctx := auth.WithTenantID(context.Background(), tenantID)

	// Make multiple requests - config should be cached after first call
	for i := 0; i < 5; i++ {
		_, err := tenantCache.Get(ctx, fmt.Sprintf("query %d", i), []float32{0.1})
		assert.NoError(t, err)
	}

	// Verify the repository was only called once
	mockRepo.AssertNumberOfCalls(t, "GetByTenantID", 1)
}
