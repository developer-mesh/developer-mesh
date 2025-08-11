package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache/integration"
	cache_middleware "github.com/developer-mesh/developer-mesh/pkg/embedding/cache/middleware"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache/monitoring"
	"github.com/developer-mesh/developer-mesh/pkg/middleware"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

func TestCacheIntegration_WithMiddleware(t *testing.T) {
	// Skip if Redis is not available
	redisClient := redis.NewClient(&redis.Options{
		Addr: cache.GetTestRedisAddr(),
		DB:   15,
	})
	defer func() { _ = redisClient.Close() }()

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}

	// Clear test database
	if err := redisClient.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("Failed to flush Redis: %v", err)
	}

	// Setup cache
	config := cache.DefaultConfig()
	config.Prefix = "test_integration"
	baseCache, err := cache.NewSemanticCache(redisClient, config, nil)
	require.NoError(t, err)

	// Create tenant config repository
	configRepo := &mockTenantConfigRepo{
		configs: make(map[string]*models.TenantConfig),
	}

	// Create tenant-aware cache
	tenantCache := cache.NewTenantAwareCache(
		baseCache,
		configRepo,
		nil,
		"test-encryption-key",
		observability.NewLogger("test"),
		nil,
	)

	// Create base rate limiter
	rlConfig := middleware.DefaultRateLimitConfig()
	baseRateLimiter := middleware.NewRateLimiter(
		rlConfig,
		observability.NewLogger("test"),
		nil,
	)

	// Create router with all integrations
	cacheRouter := integration.NewCacheRouter(
		tenantCache,
		baseRateLimiter,
		nil, // vector store
		observability.NewLogger("test"),
		nil, // metrics
	)

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	cacheRouter.SetupRoutes(api)

	// Test tenant
	tenantID := uuid.New()
	configRepo.configs[tenantID.String()] = &models.TenantConfig{
		TenantID: tenantID.String(),
		Features: map[string]interface{}{
			"cache": map[string]interface{}{
				"enabled": true,
			},
		},
	}

	t.Run("AuthMiddleware", func(t *testing.T) {
		// Test with X-Tenant-ID header
		req := httptest.NewRequest("GET", "/api/v1/cache/health", nil)
		req.Header.Set("X-Tenant-ID", tenantID.String())
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, tenantID.String(), w.Header().Get("X-Tenant-ID"))
	})

	t.Run("RequireTenantMiddleware", func(t *testing.T) {
		// Test without tenant ID
		payload := map[string]interface{}{
			"query":     "test",
			"embedding": []float32{1, 2, 3},
			"results":   []interface{}{},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/cache/entry", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		assert.Equal(t, "tenant authentication required", resp["error"])
	})

	t.Run("CacheSetAndGet", func(t *testing.T) {
		// Set cache entry
		setPayload := map[string]interface{}{
			"query":     "integration test query",
			"embedding": []float32{1.0, 2.0, 3.0},
			"results": []map[string]interface{}{
				{
					"id":      "1",
					"content": "Test result",
					"score":   0.95,
				},
			},
		}
		setBody, _ := json.Marshal(setPayload)

		setReq := httptest.NewRequest("POST", "/api/v1/cache/entry", bytes.NewBuffer(setBody))
		setReq.Header.Set("Content-Type", "application/json")
		setReq.Header.Set("X-Tenant-ID", tenantID.String())
		setW := httptest.NewRecorder()

		router.ServeHTTP(setW, setReq)
		assert.Equal(t, http.StatusOK, setW.Code)

		// Get cache entry
		getPayload := map[string]interface{}{
			"query":     "integration test query",
			"embedding": []float32{1.0, 2.0, 3.0},
		}
		getBody, _ := json.Marshal(getPayload)

		getReq := httptest.NewRequest("GET", "/api/v1/cache/search", bytes.NewBuffer(getBody))
		getReq.Header.Set("Content-Type", "application/json")
		getReq.Header.Set("X-Tenant-ID", tenantID.String())
		getW := httptest.NewRecorder()

		router.ServeHTTP(getW, getReq)
		assert.Equal(t, http.StatusOK, getW.Code)

		var getResp map[string]interface{}
		if err := json.Unmarshal(getW.Body.Bytes(), &getResp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		assert.True(t, getResp["hit"].(bool))
		assert.Equal(t, "true", getW.Header().Get("X-Cache-Hit"))
	})

	t.Run("GetStats", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/cache/stats", nil)
		req.Header.Set("X-Tenant-ID", tenantID.String())
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		assert.Contains(t, resp, "cache_stats")
		assert.Contains(t, resp, "rate_limit_stats")
	})
}

func TestCacheRateLimiter_Allow(t *testing.T) {
	config := cache_middleware.DefaultCacheRateLimitConfig()
	config.CacheReadRPS = 2
	config.CacheReadBurst = 2

	rateLimiter := cache_middleware.NewCacheRateLimiter(
		nil,
		config,
		nil,
		nil,
	)

	tenantID := uuid.New()

	// Should allow initial requests
	assert.True(t, rateLimiter.Allow(tenantID, "read"))
	assert.True(t, rateLimiter.Allow(tenantID, "read"))

	// Should block after burst is exhausted
	assert.False(t, rateLimiter.Allow(tenantID, "read"))

	// Different operation should have separate limit
	assert.True(t, rateLimiter.Allow(tenantID, "write"))
}

func TestMetricsIntegration(t *testing.T) {
	// Test that metrics are properly recorded
	metrics := &mockMetricsClient{
		counters:   make(map[string]float64),
		histograms: make(map[string][]float64),
		gauges:     make(map[string]float64),
	}

	ctx := context.Background()
	tenantID := uuid.New()

	// Test operation tracking
	err := monitoring.TrackCacheOperation(
		ctx,
		metrics,
		"test_op",
		tenantID,
		func() error {
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	)

	require.NoError(t, err)

	// Check metrics were recorded
	assert.Contains(t, metrics.histograms, "cache.operation.duration")
	assert.Greater(t, metrics.histograms["cache.operation.duration"][0], 0.01)

	assert.Contains(t, metrics.counters, "cache.operation.count")
	assert.Equal(t, float64(1), metrics.counters["cache.operation.count"])
}

// Mock implementations for testing

type mockTenantConfigRepo struct {
	configs map[string]*models.TenantConfig
}

func (m *mockTenantConfigRepo) GetByTenantID(ctx context.Context, tenantID string) (*models.TenantConfig, error) {
	if config, ok := m.configs[tenantID]; ok {
		return config, nil
	}
	return nil, nil
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

type mockMetricsClient struct {
	counters   map[string]float64
	histograms map[string][]float64
	gauges     map[string]float64
	mu         sync.Mutex
}

func (m *mockMetricsClient) IncrementCounter(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name] += value
}

func (m *mockMetricsClient) IncrementCounterWithLabels(name string, value float64, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name] += value
}

func (m *mockMetricsClient) RecordHistogram(name string, value float64, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.histograms[name] = append(m.histograms[name], value)
}

func (m *mockMetricsClient) SetGauge(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[name] = value
}

func (m *mockMetricsClient) SetGaugeWithLabels(name string, value float64, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[name] = value
}

// Additional methods to implement observability.MetricsClient interface
func (m *mockMetricsClient) RecordEvent(source, eventType string)                   {}
func (m *mockMetricsClient) RecordLatency(operation string, duration time.Duration) {}
func (m *mockMetricsClient) RecordCounter(name string, value float64, labels map[string]string) {
	m.IncrementCounterWithLabels(name, value, labels)
}
func (m *mockMetricsClient) RecordGauge(name string, value float64, labels map[string]string) {
	m.SetGaugeWithLabels(name, value, labels)
}
func (m *mockMetricsClient) RecordTimer(name string, duration time.Duration, labels map[string]string) {
}
func (m *mockMetricsClient) RecordCacheOperation(operation string, success bool, durationSeconds float64) {
}
func (m *mockMetricsClient) RecordOperation(component string, operation string, success bool, durationSeconds float64, labels map[string]string) {
}
func (m *mockMetricsClient) RecordAPIOperation(api string, operation string, success bool, durationSeconds float64) {
}
func (m *mockMetricsClient) RecordDatabaseOperation(operation string, success bool, durationSeconds float64) {
}
func (m *mockMetricsClient) StartTimer(name string, labels map[string]string) func() {
	return func() {}
}
func (m *mockMetricsClient) RecordDuration(name string, duration time.Duration) {}
func (m *mockMetricsClient) Close() error                                       { return nil }
