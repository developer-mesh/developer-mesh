package cache

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogger implements observability.Logger for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields map[string]interface{})         {}
func (m *mockLogger) Info(msg string, fields map[string]interface{})          {}
func (m *mockLogger) Warn(msg string, fields map[string]interface{})          {}
func (m *mockLogger) Error(msg string, fields map[string]interface{})         {}
func (m *mockLogger) Fatal(msg string, fields map[string]interface{})         {}
func (m *mockLogger) Debugf(format string, args ...interface{})               {}
func (m *mockLogger) Infof(format string, args ...interface{})                {}
func (m *mockLogger) Warnf(format string, args ...interface{})                {}
func (m *mockLogger) Errorf(format string, args ...interface{})               {}
func (m *mockLogger) Fatalf(format string, args ...interface{})               {}
func (m *mockLogger) WithPrefix(prefix string) observability.Logger           { return m }
func (m *mockLogger) With(fields map[string]interface{}) observability.Logger { return m }

func setupTestService(t *testing.T) (*Service, *miniredis.Miniredis, sqlmock.Sqlmock) {
	// Setup mock Redis
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Setup mock DB
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	})

	sqlxDB := sqlx.NewDb(db, "postgres")

	// Create mock logger
	logger := &mockLogger{}

	// Create service
	service := NewService(sqlxDB, redisClient, logger)

	return service, mr, mock
}

func TestGetOrCompute_CacheMiss(t *testing.T) {
	service, _, mock := setupTestService(t)
	ctx := context.Background()

	req := &ExecutionRequest{
		TenantID:   "test-tenant",
		ToolID:     "test-tool",
		Action:     "test-action",
		Parameters: map[string]interface{}{"key": "value"},
		TTLSeconds: 3600,
	}

	// Mock database query for cache lookup (returns no rows for cache miss)
	mock.ExpectQuery("SELECT \\* FROM mcp.get_or_create_cache_entry").
		WithArgs(sqlmock.AnyArg(), "test-tenant", "test-tool", "test-action", sqlmock.AnyArg(), 3600).
		WillReturnRows(sqlmock.NewRows([]string{"key_hash", "tenant_id", "response_data", "from_cache", "hit_count", "created_at"}))

	// Mock database exec for cache storage
	mock.ExpectExec("SELECT \\* FROM mcp.get_or_create_cache_entry").
		WithArgs(sqlmock.AnyArg(), "test-tenant", "test-tool", "test-action", sqlmock.AnyArg(), sqlmock.AnyArg(), 3600).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock update stats
	mock.ExpectExec("SELECT mcp.update_cache_stats").
		WithArgs("test-tenant", false, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Compute function that returns a ToolExecutionResponse
	computeFn := func(ctx context.Context) (interface{}, error) {
		return &models.ToolExecutionResponse{
			Success:    true,
			StatusCode: 200,
			Body:       map[string]interface{}{"result": "computed"},
			ExecutedAt: time.Now(),
		}, nil
	}

	result, err := service.GetOrCompute(ctx, req, computeFn)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check that it's a ToolExecutionResponse with cache metadata
	toolResp, ok := result.(*models.ToolExecutionResponse)
	assert.True(t, ok)
	assert.True(t, toolResp.Success)
	assert.False(t, toolResp.FromCache)
	assert.False(t, toolResp.CacheHit)
}

func TestGetOrCompute_RedisHit(t *testing.T) {
	service, mr, _ := setupTestService(t)
	ctx := context.Background()

	req := &ExecutionRequest{
		TenantID:   "test-tenant",
		ToolID:     "test-tool",
		Action:     "test-action",
		Parameters: map[string]interface{}{"key": "value"},
		TTLSeconds: 3600,
	}

	// Pre-populate Redis with cached response
	cachedResponse := &models.ToolExecutionResponse{
		Success:    true,
		StatusCode: 200,
		Body:       map[string]interface{}{"result": "cached"},
		ExecutedAt: time.Now(),
	}
	cachedJSON, _ := json.Marshal(cachedResponse)

	cacheKey := service.generateCacheKey(req)
	redisKey := "cache:test-tenant:" + cacheKey
	err := mr.Set(redisKey, string(cachedJSON))
	require.NoError(t, err)

	// Compute function should not be called
	computeFn := func(ctx context.Context) (interface{}, error) {
		t.Fatal("Compute function should not be called on cache hit")
		return nil, nil
	}

	result, err := service.GetOrCompute(ctx, req, computeFn)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check that it's a ToolExecutionResponse with cache metadata
	toolResp, ok := result.(*models.ToolExecutionResponse)
	assert.True(t, ok)
	assert.True(t, toolResp.Success)
	assert.True(t, toolResp.FromCache)
	assert.True(t, toolResp.CacheHit)
	assert.Equal(t, "L1_redis", toolResp.CacheLevel)
}

func TestGetOrCompute_PostgreSQLHit(t *testing.T) {
	service, _, mock := setupTestService(t)
	ctx := context.Background()

	req := &ExecutionRequest{
		TenantID:   "test-tenant",
		ToolID:     "test-tool",
		Action:     "test-action",
		Parameters: map[string]interface{}{"key": "value"},
		TTLSeconds: 3600,
	}

	// Prepare cached response
	cachedResponse := &models.ToolExecutionResponse{
		Success:    true,
		StatusCode: 200,
		Body:       map[string]interface{}{"result": "cached_pg"},
		ExecutedAt: time.Now(),
	}
	cachedJSON, _ := json.Marshal(cachedResponse)

	// Mock database query for cache hit
	rows := sqlmock.NewRows([]string{"key_hash", "tenant_id", "response_data", "from_cache", "hit_count", "created_at"}).
		AddRow("test-hash", "test-tenant", cachedJSON, true, 5, time.Now())

	mock.ExpectQuery("SELECT \\* FROM mcp.get_or_create_cache_entry").
		WithArgs(sqlmock.AnyArg(), "test-tenant", "test-tool", "test-action", sqlmock.AnyArg(), 3600).
		WillReturnRows(rows)

	// Mock update stats
	mock.ExpectExec("SELECT mcp.update_cache_stats").
		WithArgs("test-tenant", true, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Compute function should not be called
	computeFn := func(ctx context.Context) (interface{}, error) {
		t.Fatal("Compute function should not be called on cache hit")
		return nil, nil
	}

	result, err := service.GetOrCompute(ctx, req, computeFn)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check that it's a ToolExecutionResponse with cache metadata
	toolResp, ok := result.(*models.ToolExecutionResponse)
	assert.True(t, ok)
	assert.True(t, toolResp.Success)
	assert.True(t, toolResp.FromCache)
	assert.True(t, toolResp.CacheHit)
	assert.Equal(t, "L2_postgres", toolResp.CacheLevel)
	assert.Equal(t, 5, toolResp.HitCount)
}

func TestGenerateCacheKey(t *testing.T) {
	service, _, _ := setupTestService(t)

	req1 := &ExecutionRequest{
		TenantID:   "tenant1",
		ToolID:     "tool1",
		Action:     "action1",
		Parameters: map[string]interface{}{"key": "value"},
	}

	req2 := &ExecutionRequest{
		TenantID:   "tenant1",
		ToolID:     "tool1",
		Action:     "action1",
		Parameters: map[string]interface{}{"key": "value"},
	}

	req3 := &ExecutionRequest{
		TenantID:   "tenant1",
		ToolID:     "tool1",
		Action:     "action1",
		Parameters: map[string]interface{}{"key": "different"},
	}

	key1 := service.generateCacheKey(req1)
	key2 := service.generateCacheKey(req2)
	key3 := service.generateCacheKey(req3)

	// Same requests should generate same key
	assert.Equal(t, key1, key2)
	// Different parameters should generate different key
	assert.NotEqual(t, key1, key3)
}

func TestGetOrCompute_NilRedis(t *testing.T) {
	// Setup service with nil Redis
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	})

	sqlxDB := sqlx.NewDb(db, "postgres")
	logger := &mockLogger{}

	// Create service with nil Redis client
	service := NewService(sqlxDB, nil, logger)
	ctx := context.Background()

	req := &ExecutionRequest{
		TenantID:   "test-tenant",
		ToolID:     "test-tool",
		Action:     "test-action",
		Parameters: map[string]interface{}{"key": "value"},
		TTLSeconds: 3600,
	}

	// Mock database query for cache lookup (returns no rows for cache miss)
	mock.ExpectQuery("SELECT \\* FROM mcp.get_or_create_cache_entry").
		WithArgs(sqlmock.AnyArg(), "test-tenant", "test-tool", "test-action", sqlmock.AnyArg(), 3600).
		WillReturnRows(sqlmock.NewRows([]string{"key_hash", "tenant_id", "response_data", "from_cache", "hit_count", "created_at"}))

	// Mock database exec for cache storage
	mock.ExpectExec("SELECT \\* FROM mcp.get_or_create_cache_entry").
		WithArgs(sqlmock.AnyArg(), "test-tenant", "test-tool", "test-action", sqlmock.AnyArg(), sqlmock.AnyArg(), 3600).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock update stats
	mock.ExpectExec("SELECT mcp.update_cache_stats").
		WithArgs("test-tenant", false, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Compute function
	computeFn := func(ctx context.Context) (interface{}, error) {
		return &models.ToolExecutionResponse{
			Success:    true,
			StatusCode: 200,
			Body:       map[string]interface{}{"result": "computed"},
			ExecutedAt: time.Now(),
		}, nil
	}

	// Should work without Redis
	result, err := service.GetOrCompute(ctx, req, computeFn)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestInvalidatePattern(t *testing.T) {
	service, mr, mock := setupTestService(t)
	ctx := context.Background()

	// Set some Redis keys
	err := mr.Set("cache:test-tenant:pattern1", "value1")
	require.NoError(t, err)
	err = mr.Set("cache:test-tenant:pattern2", "value2")
	require.NoError(t, err)
	err = mr.Set("cache:other-tenant:pattern1", "value3")
	require.NoError(t, err)

	// Mock database update
	mock.ExpectExec("UPDATE mcp.cache_entries").
		WithArgs("test-tenant", "pattern%").
		WillReturnResult(sqlmock.NewResult(0, 2))

	err = service.InvalidatePattern(ctx, "test-tenant", "pattern")
	assert.NoError(t, err)

	// Check that only test-tenant keys were deleted
	assert.False(t, mr.Exists("cache:test-tenant:pattern1"))
	assert.False(t, mr.Exists("cache:test-tenant:pattern2"))
	assert.True(t, mr.Exists("cache:other-tenant:pattern1"))
}
