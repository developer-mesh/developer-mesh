package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/cache"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/core"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/mcp"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
)

// HealthStatus represents the overall health status
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// StartupState represents the startup state of the application
type StartupState string

const (
	StartupStateNotStarted StartupState = "not_started"
	StartupStateInProgress StartupState = "in_progress"
	StartupStateComplete   StartupState = "complete"
	StartupStateFailed     StartupState = "failed"
)

// ComponentHealth represents the health of a single component
type ComponentHealth struct {
	Status  HealthStatus           `json:"status"`
	Message string                 `json:"message,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// HealthResponse represents the full health check response
type HealthResponse struct {
	Status     HealthStatus               `json:"status"`
	Timestamp  time.Time                  `json:"timestamp"`
	Version    string                     `json:"version"`
	Uptime     float64                    `json:"uptime_seconds"`
	Components map[string]ComponentHealth `json:"components,omitempty"`
}

// LivenessResponse represents a simple liveness check response
type LivenessResponse struct {
	Status    HealthStatus `json:"status"`
	Timestamp time.Time    `json:"timestamp"`
	Alive     bool         `json:"alive"`
}

// StartupResponse represents the startup probe response
type StartupResponse struct {
	State           StartupState               `json:"state"`
	Timestamp       time.Time                  `json:"timestamp"`
	StartupDuration float64                    `json:"startup_duration_seconds,omitempty"`
	Components      map[string]ComponentHealth `json:"components,omitempty"`
	Metrics         map[string]interface{}     `json:"metrics,omitempty"`
}

// HealthChecker manages health checks for Edge MCP
type HealthChecker struct {
	toolRegistry    *tools.Registry
	cache           cache.Cache
	coreClient      *core.Client
	mcpHandler      *mcp.Handler
	logger          observability.Logger
	version         string
	config          interface{} // Store config for validation
	authenticator   interface{} // Store authenticator for validation
	startTime       time.Time
	mu              sync.RWMutex
	lastReadiness   *HealthResponse
	lastCheck       time.Time
	cacheTTL        time.Duration
	startupState    StartupState
	startupComplete time.Time
	startupMetrics  map[string]interface{}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(
	toolRegistry *tools.Registry,
	cache cache.Cache,
	coreClient *core.Client,
	mcpHandler *mcp.Handler,
	logger observability.Logger,
	version string,
) *HealthChecker {
	return &HealthChecker{
		toolRegistry:   toolRegistry,
		cache:          cache,
		coreClient:     coreClient,
		mcpHandler:     mcpHandler,
		logger:         logger,
		version:        version,
		startTime:      time.Now(),
		cacheTTL:       5 * time.Second, // Cache readiness checks for 5 seconds
		startupState:   StartupStateInProgress,
		startupMetrics: make(map[string]interface{}),
	}
}

// SetConfig sets the configuration for validation
func (h *HealthChecker) SetConfig(config interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.config = config
}

// SetAuthenticator sets the authenticator for validation
func (h *HealthChecker) SetAuthenticator(auth interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.authenticator = auth
}

// Liveness returns a simple liveness check (Kubernetes liveness probe)
// This should only fail if the application is completely dead
func (h *HealthChecker) Liveness(c *gin.Context) {
	response := LivenessResponse{
		Status:    HealthStatusHealthy,
		Timestamp: time.Now(),
		Alive:     true,
	}

	c.JSON(http.StatusOK, response)
}

// Readiness returns a detailed readiness check (Kubernetes readiness probe)
// This checks if the application is ready to serve traffic
func (h *HealthChecker) Readiness(c *gin.Context) {
	// Check cache first to avoid excessive health checks
	h.mu.RLock()
	if h.lastReadiness != nil && time.Since(h.lastCheck) < h.cacheTTL {
		cached := h.lastReadiness
		h.mu.RUnlock()

		status := http.StatusOK
		switch cached.Status {
		case HealthStatusUnhealthy:
			status = http.StatusServiceUnavailable
		case HealthStatusDegraded:
			status = http.StatusOK // Still serve traffic when degraded
		}

		c.JSON(status, cached)
		return
	}
	h.mu.RUnlock()

	// Perform health check
	response := h.checkReadiness()

	// Update cache
	h.mu.Lock()
	h.lastReadiness = response
	h.lastCheck = time.Now()
	h.mu.Unlock()

	// Determine HTTP status code
	status := http.StatusOK
	switch response.Status {
	case HealthStatusUnhealthy:
		status = http.StatusServiceUnavailable
	case HealthStatusDegraded:
		status = http.StatusOK // Still serve traffic when degraded
	}

	c.JSON(status, response)
}

// checkReadiness performs the actual readiness health check
func (h *HealthChecker) checkReadiness() *HealthResponse {
	components := make(map[string]ComponentHealth)
	overallStatus := HealthStatusHealthy

	// Check tool registry
	registryHealth := h.checkToolRegistry()
	components["tool_registry"] = registryHealth
	if registryHealth.Status == HealthStatusUnhealthy {
		overallStatus = HealthStatusUnhealthy
	} else if registryHealth.Status == HealthStatusDegraded && overallStatus == HealthStatusHealthy {
		overallStatus = HealthStatusDegraded
	}

	// Check cache
	cacheHealth := h.checkCache()
	components["cache"] = cacheHealth
	if cacheHealth.Status == HealthStatusUnhealthy {
		overallStatus = HealthStatusUnhealthy
	} else if cacheHealth.Status == HealthStatusDegraded && overallStatus == HealthStatusHealthy {
		overallStatus = HealthStatusDegraded
	}

	// Check Core Platform connectivity (optional, degraded if unavailable)
	corePlatformHealth := h.checkCorePlatform()
	components["core_platform"] = corePlatformHealth
	// Core Platform is optional, so only mark as degraded if unavailable
	if corePlatformHealth.Status == HealthStatusUnhealthy && overallStatus == HealthStatusHealthy {
		overallStatus = HealthStatusDegraded
	}

	// Check MCP handler (sessions, connections)
	mcpHealth := h.checkMCPHandler()
	components["mcp_handler"] = mcpHealth
	if mcpHealth.Status == HealthStatusUnhealthy {
		overallStatus = HealthStatusUnhealthy
	} else if mcpHealth.Status == HealthStatusDegraded && overallStatus == HealthStatusHealthy {
		overallStatus = HealthStatusDegraded
	}

	return &HealthResponse{
		Status:     overallStatus,
		Timestamp:  time.Now(),
		Version:    h.version,
		Uptime:     time.Since(h.startTime).Seconds(),
		Components: components,
	}
}

// checkToolRegistry checks the health of the tool registry
func (h *HealthChecker) checkToolRegistry() ComponentHealth {
	if h.toolRegistry == nil {
		return ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "Tool registry is not initialized",
		}
	}

	toolCount := h.toolRegistry.Count()

	if toolCount == 0 {
		return ComponentHealth{
			Status:  HealthStatusDegraded,
			Message: "No tools registered",
			Details: map[string]interface{}{
				"tool_count": toolCount,
			},
		}
	}

	return ComponentHealth{
		Status:  HealthStatusHealthy,
		Message: "Tool registry operational",
		Details: map[string]interface{}{
			"tool_count": toolCount,
		},
	}
}

// checkCache checks the health of the cache
func (h *HealthChecker) checkCache() ComponentHealth {
	if h.cache == nil {
		return ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "Cache is not initialized",
		}
	}

	// Try a simple cache operation to verify it's working
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	testKey := "health_check_" + time.Now().Format("20060102150405")
	testValue := "test"

	// Try to set a value
	if err := h.cache.Set(ctx, testKey, testValue, 5*time.Second); err != nil {
		return ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "Cache write operation failed",
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}

	// Try to get the value
	var retrieved string
	if err := h.cache.Get(ctx, testKey, &retrieved); err != nil {
		return ComponentHealth{
			Status:  HealthStatusDegraded,
			Message: "Cache read operation failed",
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}

	// Clean up test key
	_ = h.cache.Delete(ctx, testKey)

	cacheSize := h.cache.Size()
	return ComponentHealth{
		Status:  HealthStatusHealthy,
		Message: "Cache operational",
		Details: map[string]interface{}{
			"size": cacheSize,
		},
	}
}

// checkCorePlatform checks the health of Core Platform connectivity
func (h *HealthChecker) checkCorePlatform() ComponentHealth {
	if h.coreClient == nil {
		return ComponentHealth{
			Status:  HealthStatusDegraded,
			Message: "Running in standalone mode (no Core Platform)",
			Details: map[string]interface{}{
				"standalone": true,
			},
		}
	}

	// Check if core client is connected
	// The core client tracks connection status internally
	// We'll attempt a simple connectivity check with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Try to fetch remote tools as a health check
	// This verifies both connectivity and API functionality
	_, err := h.coreClient.FetchRemoteTools(ctx)
	if err != nil {
		return ComponentHealth{
			Status:  HealthStatusDegraded,
			Message: "Core Platform connectivity degraded",
			Details: map[string]interface{}{
				"error":      err.Error(),
				"standalone": false,
			},
		}
	}

	return ComponentHealth{
		Status:  HealthStatusHealthy,
		Message: "Core Platform connected",
		Details: map[string]interface{}{
			"standalone": false,
		},
	}
}

// checkMCPHandler checks the health of the MCP handler
func (h *HealthChecker) checkMCPHandler() ComponentHealth {
	if h.mcpHandler == nil {
		return ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "MCP handler is not initialized",
		}
	}

	// MCP handler is considered healthy if it exists
	// In the future, we could check for things like:
	// - Number of active sessions
	// - Number of active connections
	// - Error rates
	// But for now, just verify it's initialized
	return ComponentHealth{
		Status:  HealthStatusHealthy,
		Message: "MCP handler operational",
		Details: map[string]interface{}{
			"initialized": true,
		},
	}
}

// Startup returns the startup probe status (Kubernetes startup probe)
// This checks if the application has successfully completed startup
func (h *HealthChecker) Startup(c *gin.Context) {
	h.mu.RLock()
	state := h.startupState
	h.mu.RUnlock()

	// If startup is already complete, always return success
	if state == StartupStateComplete {
		h.mu.RLock()
		response := &StartupResponse{
			State:           StartupStateComplete,
			Timestamp:       time.Now(),
			StartupDuration: h.startupComplete.Sub(h.startTime).Seconds(),
			Metrics:         h.startupMetrics,
		}
		h.mu.RUnlock()

		c.JSON(http.StatusOK, response)
		return
	}

	// Perform startup check
	response := h.checkStartup()

	// Determine HTTP status code
	status := http.StatusOK
	switch response.State {
	case StartupStateFailed, StartupStateInProgress:
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, response)
}

// checkStartup performs the actual startup health check
func (h *HealthChecker) checkStartup() *StartupResponse {
	components := make(map[string]ComponentHealth)
	allHealthy := true

	// Check tool loading
	toolHealth := h.checkToolLoading()
	components["tool_loading"] = toolHealth
	if toolHealth.Status != HealthStatusHealthy {
		allHealthy = false
	}

	// Check authentication setup
	authHealth := h.checkAuthenticationSetup()
	components["authentication"] = authHealth
	if authHealth.Status != HealthStatusHealthy {
		allHealthy = false
	}

	// Check cache initialization
	cacheHealth := h.checkCacheInitialization()
	components["cache"] = cacheHealth
	if cacheHealth.Status != HealthStatusHealthy {
		allHealthy = false
	}

	// Check configuration validation
	configHealth := h.checkConfigurationValidation()
	components["configuration"] = configHealth
	if configHealth.Status != HealthStatusHealthy {
		allHealthy = false
	}

	// Determine startup state
	state := StartupStateInProgress
	if allHealthy {
		state = StartupStateComplete
	}

	return &StartupResponse{
		State:      state,
		Timestamp:  time.Now(),
		Components: components,
	}
}

// checkToolLoading checks if tools have been successfully loaded
func (h *HealthChecker) checkToolLoading() ComponentHealth {
	if h.toolRegistry == nil {
		return ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "Tool registry not initialized",
		}
	}

	toolCount := h.toolRegistry.Count()
	if toolCount == 0 {
		return ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "No tools loaded",
			Details: map[string]interface{}{
				"tool_count": toolCount,
			},
		}
	}

	return ComponentHealth{
		Status:  HealthStatusHealthy,
		Message: "Tools successfully loaded",
		Details: map[string]interface{}{
			"tool_count": toolCount,
		},
	}
}

// checkAuthenticationSetup verifies authentication is properly configured
func (h *HealthChecker) checkAuthenticationSetup() ComponentHealth {
	h.mu.RLock()
	auth := h.authenticator
	h.mu.RUnlock()

	if auth == nil {
		return ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "Authenticator not initialized",
		}
	}

	return ComponentHealth{
		Status:  HealthStatusHealthy,
		Message: "Authentication configured",
		Details: map[string]interface{}{
			"initialized": true,
		},
	}
}

// checkCacheInitialization verifies cache is initialized and operational
func (h *HealthChecker) checkCacheInitialization() ComponentHealth {
	if h.cache == nil {
		return ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "Cache not initialized",
		}
	}

	// Try a simple cache operation
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	testKey := "startup_check_" + time.Now().Format("20060102150405")
	testValue := "startup_test"

	if err := h.cache.Set(ctx, testKey, testValue, 5*time.Second); err != nil {
		return ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "Cache initialization failed",
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}

	_ = h.cache.Delete(ctx, testKey)

	return ComponentHealth{
		Status:  HealthStatusHealthy,
		Message: "Cache initialized and operational",
		Details: map[string]interface{}{
			"operational": true,
		},
	}
}

// checkConfigurationValidation validates required configuration
func (h *HealthChecker) checkConfigurationValidation() ComponentHealth {
	h.mu.RLock()
	cfg := h.config
	h.mu.RUnlock()

	if cfg == nil {
		return ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "Configuration not loaded",
		}
	}

	// Configuration is loaded and validated during startup
	// If we got here, configuration is valid
	return ComponentHealth{
		Status:  HealthStatusHealthy,
		Message: "Configuration validated",
		Details: map[string]interface{}{
			"validated": true,
		},
	}
}

// MarkStartupComplete marks startup as complete and logs metrics
func (h *HealthChecker) MarkStartupComplete(metrics map[string]interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.startupState = StartupStateComplete
	h.startupComplete = time.Now()
	h.startupMetrics = metrics

	duration := h.startupComplete.Sub(h.startTime).Seconds()

	// Log startup metrics
	logFields := map[string]interface{}{
		"startup_duration_seconds": duration,
		"state":                    string(StartupStateComplete),
	}
	for k, v := range metrics {
		logFields[k] = v
	}

	h.logger.Info("Startup complete", logFields)
}

// RegisterRoutes registers health check routes with the Gin router
func (h *HealthChecker) RegisterRoutes(router *gin.Engine) {
	health := router.Group("/health")
	{
		// Liveness probe - checks if the application is alive
		// This should only fail if the app is completely dead
		health.GET("/live", h.Liveness)

		// Readiness probe - checks if the application is ready to serve traffic
		// This checks all dependencies and returns 503 if not ready
		health.GET("/ready", h.Readiness)

		// Startup probe - checks if the application has completed startup
		// This is used by Kubernetes to know when the app has finished initializing
		health.GET("/startup", h.Startup)
	}
}
