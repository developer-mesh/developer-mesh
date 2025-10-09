package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/cache"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/mcp"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter(healthChecker *HealthChecker) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	healthChecker.RegisterRoutes(router)
	return router
}

func TestNewHealthChecker(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	registry := tools.NewRegistry()
	memCache := cache.NewMemoryCache(100, 5*time.Minute)

	healthChecker := NewHealthChecker(
		registry,
		memCache,
		nil, // no core client
		nil, // no mcp handler
		logger,
		"1.0.0",
	)

	assert.NotNil(t, healthChecker)
	assert.Equal(t, "1.0.0", healthChecker.version)
	assert.Equal(t, 5*time.Second, healthChecker.cacheTTL)
	assert.NotNil(t, healthChecker.startTime)
}

func TestHealthChecker_Liveness(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	registry := tools.NewRegistry()
	memCache := cache.NewMemoryCache(100, 5*time.Minute)

	healthChecker := NewHealthChecker(
		registry,
		memCache,
		nil,
		nil,
		logger,
		"1.0.0",
	)

	router := setupTestRouter(healthChecker)

	req, _ := http.NewRequest("GET", "/health/live", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response LivenessResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, HealthStatusHealthy, response.Status)
	assert.True(t, response.Alive)
	assert.NotZero(t, response.Timestamp)
}

func TestHealthChecker_Readiness_AllHealthy(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	registry := tools.NewRegistry()
	memCache := cache.NewMemoryCache(100, 5*time.Minute)

	// Add some tools to the registry
	provider := &mockToolProvider{
		tools: []tools.ToolDefinition{
			{
				Name:        "test_tool",
				Description: "Test tool",
			},
		},
	}
	registry.Register(provider)

	// Create a minimal MCP handler to avoid unhealthy status
	mcpHandler := mcp.NewHandler(
		registry,
		memCache,
		nil, // coreClient
		nil, // authenticator
		logger,
		nil, // metrics not needed for health tests
		nil, // tracerProvider not needed for health tests
	)

	healthChecker := NewHealthChecker(
		registry,
		memCache,
		nil, // no core client (will be degraded, not unhealthy)
		mcpHandler,
		logger,
		"1.0.0",
	)

	router := setupTestRouter(healthChecker)

	req, _ := http.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should be OK, degraded because Core Platform is not available
	assert.Equal(t, http.StatusOK, w.Code)

	var response HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Overall status should be degraded because Core Platform degrades to unhealthy status
	// Actually, looking at the logic: Core Platform degraded + everything else healthy = degraded overall
	// But the logic says: if core platform is unhealthy and overall is healthy, set to degraded
	// In this case, Core Platform status is degraded (not unhealthy), so it doesn't affect overall
	assert.Equal(t, HealthStatusHealthy, response.Status) // Healthy because Core Platform is optional
	assert.Equal(t, "1.0.0", response.Version)
	assert.NotZero(t, response.Uptime)
	assert.NotZero(t, response.Timestamp)

	// Check components
	require.Contains(t, response.Components, "tool_registry")
	assert.Equal(t, HealthStatusHealthy, response.Components["tool_registry"].Status)

	require.Contains(t, response.Components, "cache")
	assert.Equal(t, HealthStatusHealthy, response.Components["cache"].Status)

	require.Contains(t, response.Components, "core_platform")
	assert.Equal(t, HealthStatusDegraded, response.Components["core_platform"].Status)

	require.Contains(t, response.Components, "mcp_handler")
	assert.Equal(t, HealthStatusHealthy, response.Components["mcp_handler"].Status)
}

func TestHealthChecker_Readiness_NoTools(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	registry := tools.NewRegistry()
	memCache := cache.NewMemoryCache(100, 5*time.Minute)

	// Create a minimal MCP handler to avoid unhealthy status
	mcpHandler := mcp.NewHandler(
		registry,
		memCache,
		nil, // coreClient
		nil, // authenticator
		logger,
		nil, // metrics not needed for health tests
		nil, // tracerProvider not needed for health tests
	)

	healthChecker := NewHealthChecker(
		registry,
		memCache,
		nil,
		mcpHandler,
		logger,
		"1.0.0",
	)

	router := setupTestRouter(healthChecker)

	req, _ := http.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should be OK even with no tools (degraded)
	assert.Equal(t, http.StatusOK, w.Code)

	var response HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, HealthStatusDegraded, response.Status)

	// Check tool registry component
	require.Contains(t, response.Components, "tool_registry")
	assert.Equal(t, HealthStatusDegraded, response.Components["tool_registry"].Status)
	assert.Contains(t, response.Components["tool_registry"].Message, "No tools registered")
}

func TestHealthChecker_Readiness_CacheHealthy(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	registry := tools.NewRegistry()
	memCache := cache.NewMemoryCache(100, 5*time.Minute)

	// Add a tool
	provider := &mockToolProvider{
		tools: []tools.ToolDefinition{
			{Name: "test_tool", Description: "Test tool"},
		},
	}
	registry.Register(provider)

	healthChecker := NewHealthChecker(
		registry,
		memCache,
		nil,
		nil,
		logger,
		"1.0.0",
	)

	router := setupTestRouter(healthChecker)

	req, _ := http.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Check cache component
	require.Contains(t, response.Components, "cache")
	assert.Equal(t, HealthStatusHealthy, response.Components["cache"].Status)
	assert.Contains(t, response.Components["cache"].Message, "operational")
}

func TestHealthChecker_Readiness_Caching(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	registry := tools.NewRegistry()
	memCache := cache.NewMemoryCache(100, 5*time.Minute)

	// Add a tool
	provider := &mockToolProvider{
		tools: []tools.ToolDefinition{
			{Name: "test_tool", Description: "Test tool"},
		},
	}
	registry.Register(provider)

	healthChecker := NewHealthChecker(
		registry,
		memCache,
		nil,
		nil,
		logger,
		"1.0.0",
	)

	router := setupTestRouter(healthChecker)

	// First request
	req1, _ := http.NewRequest("GET", "/health/ready", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	var response1 HealthResponse
	err := json.Unmarshal(w1.Body.Bytes(), &response1)
	require.NoError(t, err)

	timestamp1 := response1.Timestamp

	// Second request within cache TTL (should be cached)
	time.Sleep(100 * time.Millisecond)
	req2, _ := http.NewRequest("GET", "/health/ready", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	var response2 HealthResponse
	err = json.Unmarshal(w2.Body.Bytes(), &response2)
	require.NoError(t, err)

	timestamp2 := response2.Timestamp

	// Timestamps should be the same (cached response)
	assert.Equal(t, timestamp1, timestamp2)

	// Wait for cache to expire
	time.Sleep(6 * time.Second)

	// Third request after cache expiry
	req3, _ := http.NewRequest("GET", "/health/ready", nil)
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	var response3 HealthResponse
	err = json.Unmarshal(w3.Body.Bytes(), &response3)
	require.NoError(t, err)

	timestamp3 := response3.Timestamp

	// Timestamp should be different (fresh response)
	assert.NotEqual(t, timestamp1, timestamp3)
}

func TestHealthChecker_CheckToolRegistry(t *testing.T) {
	logger := observability.NewStandardLogger("test")

	tests := []struct {
		name          string
		registry      *tools.Registry
		setupRegistry func(*tools.Registry)
		wantStatus    HealthStatus
		wantMessage   string
	}{
		{
			name:        "nil registry",
			registry:    nil,
			wantStatus:  HealthStatusUnhealthy,
			wantMessage: "not initialized",
		},
		{
			name:        "empty registry",
			registry:    tools.NewRegistry(),
			wantStatus:  HealthStatusDegraded,
			wantMessage: "No tools registered",
		},
		{
			name:     "registry with tools",
			registry: tools.NewRegistry(),
			setupRegistry: func(r *tools.Registry) {
				provider := &mockToolProvider{
					tools: []tools.ToolDefinition{
						{Name: "tool1", Description: "Tool 1"},
						{Name: "tool2", Description: "Tool 2"},
					},
				}
				r.Register(provider)
			},
			wantStatus:  HealthStatusHealthy,
			wantMessage: "operational",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupRegistry != nil && tt.registry != nil {
				tt.setupRegistry(tt.registry)
			}

			healthChecker := &HealthChecker{
				toolRegistry: tt.registry,
				logger:       logger,
			}

			result := healthChecker.checkToolRegistry()
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Contains(t, result.Message, tt.wantMessage)

			if tt.wantStatus == HealthStatusHealthy && tt.registry != nil {
				assert.Contains(t, result.Details, "tool_count")
			}
		})
	}
}

func TestHealthChecker_CheckCache(t *testing.T) {
	logger := observability.NewStandardLogger("test")

	tests := []struct {
		name        string
		cache       cache.Cache
		wantStatus  HealthStatus
		wantMessage string
	}{
		{
			name:        "nil cache",
			cache:       nil,
			wantStatus:  HealthStatusUnhealthy,
			wantMessage: "not initialized",
		},
		{
			name:        "working cache",
			cache:       cache.NewMemoryCache(100, 5*time.Minute),
			wantStatus:  HealthStatusHealthy,
			wantMessage: "operational",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healthChecker := &HealthChecker{
				cache:  tt.cache,
				logger: logger,
			}

			result := healthChecker.checkCache()
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Contains(t, result.Message, tt.wantMessage)

			if tt.wantStatus == HealthStatusHealthy {
				assert.Contains(t, result.Details, "size")
			}
		})
	}
}

func TestHealthChecker_CheckCorePlatform(t *testing.T) {
	logger := observability.NewStandardLogger("test")

	tests := []struct {
		name        string
		coreClient  interface{} // Using interface{} since we can't easily mock core.Client
		wantStatus  HealthStatus
		wantMessage string
	}{
		{
			name:        "no core client (standalone)",
			coreClient:  nil,
			wantStatus:  HealthStatusDegraded,
			wantMessage: "standalone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healthChecker := &HealthChecker{
				coreClient: nil, // We don't test with real core client here
				logger:     logger,
			}

			result := healthChecker.checkCorePlatform()
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Contains(t, result.Message, tt.wantMessage)
		})
	}
}

func TestHealthChecker_CheckMCPHandler(t *testing.T) {
	logger := observability.NewStandardLogger("test")

	tests := []struct {
		name        string
		mcpHandler  *mcp.Handler
		wantStatus  HealthStatus
		wantMessage string
	}{
		{
			name:        "nil mcp handler",
			mcpHandler:  nil,
			wantStatus:  HealthStatusUnhealthy,
			wantMessage: "not initialized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healthChecker := &HealthChecker{
				mcpHandler: tt.mcpHandler,
				logger:     logger,
			}

			result := healthChecker.checkMCPHandler()
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Contains(t, result.Message, tt.wantMessage)
		})
	}
}

func TestHealthChecker_RegisterRoutes(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	registry := tools.NewRegistry()
	memCache := cache.NewMemoryCache(100, 5*time.Minute)

	// Add a tool to make registry healthy
	provider := &mockToolProvider{
		tools: []tools.ToolDefinition{
			{Name: "test_tool", Description: "Test tool"},
		},
	}
	registry.Register(provider)

	// Create MCP handler
	mcpHandler := mcp.NewHandler(
		registry,
		memCache,
		nil, // coreClient
		nil, // authenticator
		logger,
		nil, // metrics not needed for health tests
		nil, // tracerProvider not needed for health tests
	)

	healthChecker := NewHealthChecker(
		registry,
		memCache,
		nil,
		mcpHandler,
		logger,
		"1.0.0",
	)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	healthChecker.RegisterRoutes(router)

	// Test that routes are registered
	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "liveness endpoint exists",
			path:       "/health/live",
			wantStatus: http.StatusOK,
		},
		{
			name:       "readiness endpoint exists",
			path:       "/health/ready",
			wantStatus: http.StatusOK, // OK because we have tools and mcp handler
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestHealthChecker_UptimeCalculation(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	registry := tools.NewRegistry()
	memCache := cache.NewMemoryCache(100, 5*time.Minute)

	// Create health checker with a known start time
	startTime := time.Now()
	healthChecker := NewHealthChecker(
		registry,
		memCache,
		nil,
		nil,
		logger,
		"1.0.0",
	)
	healthChecker.startTime = startTime

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Check readiness
	response := healthChecker.checkReadiness()

	// Uptime should be greater than 0 and less than 1 second
	assert.Greater(t, response.Uptime, 0.0)
	assert.Less(t, response.Uptime, 1.0)
}

// Mock tool provider for testing
type mockToolProvider struct {
	tools []tools.ToolDefinition
}

func (m *mockToolProvider) GetDefinitions() []tools.ToolDefinition {
	return m.tools
}

// Benchmark tests
func BenchmarkHealthChecker_Liveness(b *testing.B) {
	logger := observability.NewStandardLogger("test")
	registry := tools.NewRegistry()
	memCache := cache.NewMemoryCache(100, 5*time.Minute)

	healthChecker := NewHealthChecker(
		registry,
		memCache,
		nil,
		nil,
		logger,
		"1.0.0",
	)

	router := setupTestRouter(healthChecker)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/health/live", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

func BenchmarkHealthChecker_Readiness(b *testing.B) {
	logger := observability.NewStandardLogger("test")
	registry := tools.NewRegistry()
	memCache := cache.NewMemoryCache(100, 5*time.Minute)

	// Add some tools
	provider := &mockToolProvider{
		tools: []tools.ToolDefinition{
			{Name: "tool1", Description: "Tool 1"},
			{Name: "tool2", Description: "Tool 2"},
		},
	}
	registry.Register(provider)

	healthChecker := NewHealthChecker(
		registry,
		memCache,
		nil,
		nil,
		logger,
		"1.0.0",
	)

	router := setupTestRouter(healthChecker)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/health/ready", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

func BenchmarkHealthChecker_ReadinessCached(b *testing.B) {
	logger := observability.NewStandardLogger("test")
	registry := tools.NewRegistry()
	memCache := cache.NewMemoryCache(100, 5*time.Minute)

	// Add some tools
	provider := &mockToolProvider{
		tools: []tools.ToolDefinition{
			{Name: "tool1", Description: "Tool 1"},
		},
	}
	registry.Register(provider)

	healthChecker := NewHealthChecker(
		registry,
		memCache,
		nil,
		nil,
		logger,
		"1.0.0",
	)

	router := setupTestRouter(healthChecker)

	// Prime the cache
	req, _ := http.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/health/ready", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// Startup probe tests
func TestHealthChecker_Startup_InProgress(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	registry := tools.NewRegistry()
	memCache := cache.NewMemoryCache(100, 5*time.Minute)

	// No tools registered yet - startup should be in progress
	healthChecker := NewHealthChecker(
		registry,
		memCache,
		nil,
		nil,
		logger,
		"1.0.0",
	)

	router := setupTestRouter(healthChecker)

	req, _ := http.NewRequest("GET", "/health/startup", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 503 when startup is in progress
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response StartupResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, StartupStateInProgress, response.State)
	assert.NotZero(t, response.Timestamp)
	assert.Contains(t, response.Components, "tool_loading")
	assert.Equal(t, HealthStatusUnhealthy, response.Components["tool_loading"].Status)
}

func TestHealthChecker_Startup_Complete(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	registry := tools.NewRegistry()
	memCache := cache.NewMemoryCache(100, 5*time.Minute)

	// Add tools
	provider := &mockToolProvider{
		tools: []tools.ToolDefinition{
			{Name: "test_tool", Description: "Test tool"},
		},
	}
	registry.Register(provider)

	// Create mock config
	mockConfig := map[string]interface{}{
		"server": map[string]interface{}{"port": 8082},
	}

	// Create mock authenticator
	mockAuth := struct{}{}

	healthChecker := NewHealthChecker(
		registry,
		memCache,
		nil,
		nil,
		logger,
		"1.0.0",
	)

	healthChecker.SetConfig(mockConfig)
	healthChecker.SetAuthenticator(mockAuth)

	// Mark startup as complete
	metrics := map[string]interface{}{
		"builtin_tools": 1,
		"remote_tools":  0,
		"total_tools":   1,
	}
	healthChecker.MarkStartupComplete(metrics)

	router := setupTestRouter(healthChecker)

	req, _ := http.NewRequest("GET", "/health/startup", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 200 when startup is complete
	assert.Equal(t, http.StatusOK, w.Code)

	var response StartupResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, StartupStateComplete, response.State)
	assert.NotZero(t, response.Timestamp)
	assert.Greater(t, response.StartupDuration, 0.0)
	assert.NotNil(t, response.Metrics)
	// JSON unmarshaling converts numbers to float64
	assert.Equal(t, float64(1), response.Metrics["builtin_tools"])
	assert.Equal(t, float64(0), response.Metrics["remote_tools"])
	assert.Equal(t, float64(1), response.Metrics["total_tools"])
}

func TestHealthChecker_Startup_AlwaysSucceedsAfterComplete(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	registry := tools.NewRegistry()
	memCache := cache.NewMemoryCache(100, 5*time.Minute)

	// Add tools
	provider := &mockToolProvider{
		tools: []tools.ToolDefinition{
			{Name: "test_tool", Description: "Test tool"},
		},
	}
	registry.Register(provider)

	healthChecker := NewHealthChecker(
		registry,
		memCache,
		nil,
		nil,
		logger,
		"1.0.0",
	)

	healthChecker.SetConfig(map[string]interface{}{})
	healthChecker.SetAuthenticator(struct{}{})

	// Mark startup as complete
	healthChecker.MarkStartupComplete(map[string]interface{}{
		"tools": 1,
	})

	router := setupTestRouter(healthChecker)

	// Make multiple requests - all should succeed
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("GET", "/health/startup", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)

		var response StartupResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, StartupStateComplete, response.State)
	}
}

func TestHealthChecker_CheckToolLoading(t *testing.T) {
	logger := observability.NewStandardLogger("test")

	tests := []struct {
		name        string
		registry    *tools.Registry
		setup       func(*tools.Registry)
		wantStatus  HealthStatus
		wantMessage string
	}{
		{
			name:        "nil registry",
			registry:    nil,
			wantStatus:  HealthStatusUnhealthy,
			wantMessage: "not initialized",
		},
		{
			name:        "no tools loaded",
			registry:    tools.NewRegistry(),
			wantStatus:  HealthStatusUnhealthy,
			wantMessage: "No tools loaded",
		},
		{
			name:     "tools loaded successfully",
			registry: tools.NewRegistry(),
			setup: func(r *tools.Registry) {
				provider := &mockToolProvider{
					tools: []tools.ToolDefinition{
						{Name: "tool1", Description: "Tool 1"},
						{Name: "tool2", Description: "Tool 2"},
					},
				}
				r.Register(provider)
			},
			wantStatus:  HealthStatusHealthy,
			wantMessage: "successfully loaded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil && tt.registry != nil {
				tt.setup(tt.registry)
			}

			healthChecker := &HealthChecker{
				toolRegistry: tt.registry,
				logger:       logger,
			}

			result := healthChecker.checkToolLoading()
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Contains(t, result.Message, tt.wantMessage)

			if tt.wantStatus == HealthStatusHealthy {
				assert.Contains(t, result.Details, "tool_count")
			}
		})
	}
}

func TestHealthChecker_CheckAuthenticationSetup(t *testing.T) {
	logger := observability.NewStandardLogger("test")

	tests := []struct {
		name          string
		authenticator interface{}
		wantStatus    HealthStatus
		wantMessage   string
	}{
		{
			name:          "nil authenticator",
			authenticator: nil,
			wantStatus:    HealthStatusUnhealthy,
			wantMessage:   "not initialized",
		},
		{
			name:          "authenticator configured",
			authenticator: struct{}{}, // Mock authenticator
			wantStatus:    HealthStatusHealthy,
			wantMessage:   "configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healthChecker := &HealthChecker{
				authenticator: tt.authenticator,
				logger:        logger,
			}

			result := healthChecker.checkAuthenticationSetup()
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Contains(t, result.Message, tt.wantMessage)
		})
	}
}

func TestHealthChecker_CheckCacheInitialization(t *testing.T) {
	logger := observability.NewStandardLogger("test")

	tests := []struct {
		name        string
		cache       cache.Cache
		wantStatus  HealthStatus
		wantMessage string
	}{
		{
			name:        "nil cache",
			cache:       nil,
			wantStatus:  HealthStatusUnhealthy,
			wantMessage: "not initialized",
		},
		{
			name:        "cache operational",
			cache:       cache.NewMemoryCache(100, 5*time.Minute),
			wantStatus:  HealthStatusHealthy,
			wantMessage: "operational",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healthChecker := &HealthChecker{
				cache:  tt.cache,
				logger: logger,
			}

			result := healthChecker.checkCacheInitialization()
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Contains(t, result.Message, tt.wantMessage)
		})
	}
}

func TestHealthChecker_CheckConfigurationValidation(t *testing.T) {
	logger := observability.NewStandardLogger("test")

	tests := []struct {
		name        string
		config      interface{}
		wantStatus  HealthStatus
		wantMessage string
	}{
		{
			name:        "nil config",
			config:      nil,
			wantStatus:  HealthStatusUnhealthy,
			wantMessage: "not loaded",
		},
		{
			name: "config validated",
			config: map[string]interface{}{
				"server": map[string]interface{}{"port": 8082},
			},
			wantStatus:  HealthStatusHealthy,
			wantMessage: "validated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healthChecker := &HealthChecker{
				config: tt.config,
				logger: logger,
			}

			result := healthChecker.checkConfigurationValidation()
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Contains(t, result.Message, tt.wantMessage)
		})
	}
}

func TestHealthChecker_MarkStartupComplete(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	registry := tools.NewRegistry()
	memCache := cache.NewMemoryCache(100, 5*time.Minute)

	healthChecker := NewHealthChecker(
		registry,
		memCache,
		nil,
		nil,
		logger,
		"1.0.0",
	)

	// Initial state should be in progress
	assert.Equal(t, StartupStateInProgress, healthChecker.startupState)

	metrics := map[string]interface{}{
		"builtin_tools": 5,
		"remote_tools":  3,
		"total_tools":   8,
	}

	// Mark startup as complete
	healthChecker.MarkStartupComplete(metrics)

	// State should be complete
	assert.Equal(t, StartupStateComplete, healthChecker.startupState)
	assert.NotZero(t, healthChecker.startupComplete)
	assert.Equal(t, metrics, healthChecker.startupMetrics)
}

func TestHealthChecker_StartupRoute_Registered(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	registry := tools.NewRegistry()
	memCache := cache.NewMemoryCache(100, 5*time.Minute)

	// Add tools
	provider := &mockToolProvider{
		tools: []tools.ToolDefinition{
			{Name: "test_tool", Description: "Test tool"},
		},
	}
	registry.Register(provider)

	healthChecker := NewHealthChecker(
		registry,
		memCache,
		nil,
		nil,
		logger,
		"1.0.0",
	)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	healthChecker.RegisterRoutes(router)

	// Test startup endpoint exists
	req, _ := http.NewRequest("GET", "/health/startup", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should not be 404
	assert.NotEqual(t, http.StatusNotFound, w.Code)

	// Should be either 200 (complete) or 503 (in progress)
	assert.Contains(t, []int{http.StatusOK, http.StatusServiceUnavailable}, w.Code)
}
