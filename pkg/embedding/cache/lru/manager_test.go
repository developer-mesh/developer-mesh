package lru

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache/eviction"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// MockRedisClient is a mock implementation of RedisClient
type MockRedisClient struct {
	mock.Mock
	client *redis.Client
}

func (m *MockRedisClient) Execute(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	args := m.Called(ctx, fn)
	if args.Get(0) == nil {
		return fn()
	}
	return args.Get(0), args.Error(1)
}

func (m *MockRedisClient) GetClient() *redis.Client {
	return m.client
}

// MockVectorStore is a mock implementation of VectorStore
type MockVectorStore struct {
	mock.Mock
}

func (m *MockVectorStore) GetTenantCacheStats(ctx context.Context, tenantID uuid.UUID) (*eviction.TenantCacheStats, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*eviction.TenantCacheStats), args.Error(1)
}

func (m *MockVectorStore) GetLRUEntries(ctx context.Context, tenantID uuid.UUID, limit int) ([]eviction.LRUEntry, error) {
	args := m.Called(ctx, tenantID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]eviction.LRUEntry), args.Error(1)
}

func (m *MockVectorStore) DeleteCacheEntry(ctx context.Context, tenantID uuid.UUID, cacheKey string) error {
	args := m.Called(ctx, tenantID, cacheKey)
	return args.Error(0)
}

func (m *MockVectorStore) GetTenantsWithCache(ctx context.Context) ([]uuid.UUID, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]uuid.UUID), args.Error(1)
}

func (m *MockVectorStore) CleanupStaleEntries(ctx context.Context, staleDuration time.Duration) (int64, error) {
	args := m.Called(ctx, staleDuration)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockVectorStore) GetGlobalCacheStats(ctx context.Context) (map[string]interface{}, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func TestManager_EvictForTenant(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	prefix := "test"

	// Use miniredis for testing
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	// Setup Redis client mock
	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer func() { _ = redisClient.Close() }()

	mockRedis := &MockRedisClient{client: redisClient}
	mockRedis.On("Execute", mock.Anything, mock.Anything).Return(nil, nil)

	// Create manager
	config := &Config{
		MaxTenantEntries:  100,
		EvictionBatchSize: 10,
	}
	manager := NewManager(mockRedis, config, prefix, nil, nil)

	t.Run("NoEvictionNeeded", func(t *testing.T) {
		// Mock key count less than target
		mockRedis.On("Execute", mock.Anything, mock.Anything).Return(int64(50), nil).Once()

		err := manager.EvictForTenant(ctx, tenantID, 100)
		assert.NoError(t, err)
	})

	t.Run("EvictionSuccess", func(t *testing.T) {
		// Mock key count exceeds target
		mockRedis.On("Execute", mock.Anything, mock.Anything).Return(int64(150), nil).Once()

		// Mock getting LRU candidates
		candidates := []string{"key1", "key2", "key3"}
		mockRedis.On("Execute", mock.Anything, mock.Anything).Return(candidates, nil).Once()

		// Mock batch delete
		mockRedis.On("Execute", mock.Anything, mock.Anything).Return(nil, nil).Once()

		err := manager.EvictForTenant(ctx, tenantID, 100)
		assert.NoError(t, err)
	})
}

func TestEvictionPolicies(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()

	t.Run("SizeBasedPolicy", func(t *testing.T) {
		policy := NewSizeBasedPolicy(100, 1024*1024) // 100 entries, 1MB

		// Under limits
		stats := TenantStats{
			EntryCount: 50,
			TotalBytes: 512 * 1024, // 512KB
		}
		assert.False(t, policy.ShouldEvict(ctx, tenantID, stats))

		// Over entry limit
		stats.EntryCount = 150
		assert.True(t, policy.ShouldEvict(ctx, tenantID, stats))
		assert.Equal(t, 90, policy.GetEvictionTarget(ctx, tenantID, stats))

		// Over byte limit
		stats = TenantStats{
			EntryCount: 50,
			TotalBytes: 2 * 1024 * 1024, // 2MB
		}
		assert.True(t, policy.ShouldEvict(ctx, tenantID, stats))
		target := policy.GetEvictionTarget(ctx, tenantID, stats)
		assert.Less(t, target, 50)
	})

	t.Run("AdaptivePolicy", func(t *testing.T) {
		basePolicy := NewSizeBasedPolicy(100, 1024*1024)
		config := DefaultConfig()
		adaptivePolicy := NewAdaptivePolicy(basePolicy, 0.5, config)

		// High hit rate - use base policy
		stats := TenantStats{
			EntryCount: 90,
			HitRate:    0.8,
		}
		assert.False(t, adaptivePolicy.ShouldEvict(ctx, tenantID, stats))

		// Low hit rate - more aggressive
		stats.HitRate = 0.3
		stats.EntryCount = 85
		assert.True(t, adaptivePolicy.ShouldEvict(ctx, tenantID, stats))
		assert.Equal(t, 70, adaptivePolicy.GetEvictionTarget(ctx, tenantID, stats))
	})

	t.Run("TimeBasedPolicy", func(t *testing.T) {
		policy := NewTimeBasedPolicy(24*time.Hour, 1*time.Hour)

		// Recent eviction
		stats := TenantStats{
			LastEviction: time.Now().Add(-30 * time.Minute),
		}
		assert.False(t, policy.ShouldEvict(ctx, tenantID, stats))

		// Old eviction
		stats.LastEviction = time.Now().Add(-2 * time.Hour)
		assert.True(t, policy.ShouldEvict(ctx, tenantID, stats))
	})

	t.Run("CompositePolicy", func(t *testing.T) {
		sizePolicy := NewSizeBasedPolicy(100, 1024*1024)
		timePolicy := NewTimeBasedPolicy(24*time.Hour, 1*time.Hour)
		composite := NewCompositePolicy(sizePolicy, timePolicy)

		// Neither policy triggers
		stats := TenantStats{
			EntryCount:   50,
			TotalBytes:   512 * 1024,
			LastEviction: time.Now(),
		}
		assert.False(t, composite.ShouldEvict(ctx, tenantID, stats))

		// Size policy triggers
		stats.EntryCount = 150
		assert.True(t, composite.ShouldEvict(ctx, tenantID, stats))

		// Time policy triggers
		stats.EntryCount = 50
		stats.LastEviction = time.Now().Add(-2 * time.Hour)
		assert.True(t, composite.ShouldEvict(ctx, tenantID, stats))
	})

	t.Run("TenantQuotaPolicy", func(t *testing.T) {
		policy := NewTenantQuotaPolicy()

		// Set quota for tenant
		policy.SetQuota(tenantID, TenantQuota{
			MaxEntries: 50,
			MaxBytes:   500 * 1024,
		})

		// Under quota
		stats := TenantStats{
			EntryCount: 30,
			TotalBytes: 300 * 1024,
		}
		assert.False(t, policy.ShouldEvict(ctx, tenantID, stats))

		// Over quota
		stats.EntryCount = 60
		assert.True(t, policy.ShouldEvict(ctx, tenantID, stats))
		assert.Equal(t, 45, policy.GetEvictionTarget(ctx, tenantID, stats))

		// No quota set for different tenant
		otherTenant := uuid.New()
		assert.False(t, policy.ShouldEvict(ctx, otherTenant, stats))
	})
}

func TestLRUManager_Integration(t *testing.T) {
	// Skip if Redis is not available
	redisClient := GetTestRedisClient(t)
	defer func() { _ = redisClient.Close() }()

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}

	// Clear test database
	redisClient.FlushDB(ctx)

	// Create real Redis wrapper (would need to implement this)
	// For now, using mock
	mockRedis := &MockRedisClient{client: redisClient}
	mockRedis.On("Execute", mock.Anything, mock.Anything).Return(nil, nil)

	// Create manager with real components
	config := &Config{
		MaxTenantEntries:  10,
		EvictionBatchSize: 5,
		EvictionInterval:  100 * time.Millisecond,
		TrackingBatchSize: 5,
		FlushInterval:     50 * time.Millisecond,
	}

	logger := observability.NewLogger("test")
	metrics := observability.NewMetricsClient()

	manager := NewManager(mockRedis, config, "test", logger, metrics)

	// Test tracking access
	tenantID := uuid.New()
	for i := 0; i < 15; i++ {
		key := fmt.Sprintf("key%d", i)
		manager.TrackAccess(tenantID, key)
	}

	// Wait for flush
	time.Sleep(100 * time.Millisecond)

	// Mock vector store
	mockVectorStore := &MockVectorStore{}
	mockVectorStore.On("GetTenantsWithCache", mock.Anything).Return([]uuid.UUID{tenantID}, nil)
	mockVectorStore.On("GetTenantCacheStats", mock.Anything, tenantID).Return(&eviction.TenantCacheStats{
		TenantID:   tenantID,
		EntryCount: 15,
		TotalHits:  100,
	}, nil)

	// Run eviction cycle
	runCtx, cancel := context.WithCancel(ctx)
	go manager.Run(runCtx, mockVectorStore)

	// Wait for eviction
	time.Sleep(200 * time.Millisecond)

	// Stop manager
	cancel()
	err := manager.Stop(ctx)
	assert.NoError(t, err)

	// Verify eviction was attempted
	mockVectorStore.AssertCalled(t, "GetTenantCacheStats", mock.Anything, tenantID)
}
