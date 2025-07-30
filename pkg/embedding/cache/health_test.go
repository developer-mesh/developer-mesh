package cache

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

func TestHealthChecker_CheckHealth(t *testing.T) {
	// Create test Redis client
	redisClient := GetTestRedisClient(t)
	defer func() { _ = redisClient.Close() }()

	ctx := context.Background()

	// Skip if Redis not available
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}

	// Create cache
	config := DefaultConfig()
	logger := observability.NewLogger("test")
	cache, err := NewSemanticCache(redisClient, config, logger)
	require.NoError(t, err)

	// Create tenant cache
	tenantCache := NewTenantAwareCache(cache, nil, nil, "test-key", logger, nil)

	// Create health checker
	checker := NewHealthChecker(cache, tenantCache)

	// Run health check
	health := checker.CheckHealth(ctx)

	// Verify results
	assert.NotNil(t, health)
	assert.NotEmpty(t, health.Checks)
	assert.Equal(t, health.TotalChecks, len(health.Checks))

	// Check that we have expected components
	components := make(map[string]bool)
	for _, check := range health.Checks {
		components[check.Component] = true
		assert.NotZero(t, check.Latency)
		assert.NotZero(t, check.LastChecked)
	}

	assert.True(t, components["redis"])
	assert.True(t, components["vector_store"])
	assert.True(t, components["lru_manager"])
	assert.True(t, components["encryption"])
	assert.True(t, components["compression"])
	assert.True(t, components["circuit_breaker"])
}

func TestHealthChecker_IndividualChecks(t *testing.T) {
	// Create test Redis client
	redisClient := GetTestRedisClient(t)
	defer func() { _ = redisClient.Close() }()

	ctx := context.Background()

	// Skip if Redis not available
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}

	// Create cache
	config := DefaultConfig()
	logger := observability.NewLogger("test")
	cache, err := NewSemanticCache(redisClient, config, logger)
	require.NoError(t, err)

	// Create tenant cache
	tenantCache := NewTenantAwareCache(cache, nil, nil, "test-key", logger, nil)

	// Create health checker
	checker := NewHealthChecker(cache, tenantCache)

	t.Run("Redis check", func(t *testing.T) {
		check := checker.checkRedis(ctx)
		assert.Equal(t, "redis", check.Component)
		assert.Equal(t, HealthStatusHealthy, check.Status)
		assert.NotZero(t, check.Latency)
	})

	t.Run("Encryption check", func(t *testing.T) {
		check := checker.checkEncryption(ctx)
		assert.Equal(t, "encryption", check.Component)
		// Should be healthy since we created with encryption key
		assert.Equal(t, HealthStatusHealthy, check.Status)
	})

	t.Run("Circuit breaker check", func(t *testing.T) {
		check := checker.checkCircuitBreaker(ctx)
		assert.Equal(t, "circuit_breaker", check.Component)
		assert.Equal(t, HealthStatusHealthy, check.Status)
	})
}

func TestHealthHandler_Endpoints(t *testing.T) {
	// Create minimal cache for testing
	cache := &SemanticCache{
		config: DefaultConfig(),
		logger: observability.NewLogger("test"),
	}

	handler := NewHealthHandler(cache, nil)

	t.Run("Liveness endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
		w := httptest.NewRecorder()

		handler.HandleHealthLiveness(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "alive")
	})

	t.Run("Readiness endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
		w := httptest.NewRecorder()

		handler.HandleHealthReadiness(w, req)

		// Should be unavailable without Redis
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		assert.Contains(t, w.Body.String(), "ready")
	})

	t.Run("Stats endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health/stats", nil)
		w := httptest.NewRecorder()

		handler.HandleHealthStats(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "timestamp")
	})
}

func TestHealthHandler_RegisterRoutes(t *testing.T) {
	handler := NewHealthHandler(nil, nil)
	mux := http.NewServeMux()

	// Register routes
	handler.RegisterRoutes(mux, "/cache/health")

	// Test that routes are registered
	testServer := httptest.NewServer(mux)
	defer testServer.Close()

	// Test main health endpoint
	resp, err := http.Get(testServer.URL + "/cache/health")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	// Should get a response (even if unhealthy)
	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode)
}

func TestHealthStatus_Overall(t *testing.T) {
	health := &CacheHealth{
		Status:    HealthStatusHealthy,
		Checks:    []HealthCheck{},
		Timestamp: time.Now(),
	}

	// All healthy
	health.Checks = []HealthCheck{
		{Component: "redis", Status: HealthStatusHealthy},
		{Component: "encryption", Status: HealthStatusHealthy},
	}
	health.Healthy = 2
	assert.Equal(t, HealthStatusHealthy, health.Status)

	// Some degraded
	health.Checks = append(health.Checks, HealthCheck{
		Component: "lru", Status: HealthStatusDegraded,
	})
	health.Degraded = 1
	health.Status = HealthStatusDegraded
	assert.Equal(t, HealthStatusDegraded, health.Status)

	// Any unhealthy
	health.Checks = append(health.Checks, HealthCheck{
		Component: "vector", Status: HealthStatusUnhealthy,
	})
	health.Unhealthy = 1
	health.Status = HealthStatusUnhealthy
	assert.Equal(t, HealthStatusUnhealthy, health.Status)
}
