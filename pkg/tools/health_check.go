package tools

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// HealthCheckManager manages health checks for tools
type HealthCheckManager struct {
	cache   cache.Cache
	handler OpenAPIHandler
	logger  observability.Logger
	metrics observability.MetricsClient
	mu      sync.RWMutex
	checks  map[string]*healthCheckEntry
}

// healthCheckEntry stores health check state
type healthCheckEntry struct {
	config     ToolConfig
	lastCheck  time.Time
	lastStatus HealthStatus
	checking   bool
	checkingMu sync.Mutex
}

// NewHealthCheckManager creates a new health check manager
func NewHealthCheckManager(
	cache cache.Cache,
	handler OpenAPIHandler,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *HealthCheckManager {
	return &HealthCheckManager{
		cache:   cache,
		handler: handler,
		logger:  logger,
		metrics: metrics,
		checks:  make(map[string]*healthCheckEntry),
	}
}

// CheckHealth performs a health check for a tool
func (m *HealthCheckManager) CheckHealth(ctx context.Context, config ToolConfig, force bool) (*HealthStatus, error) {
	// Apply default health config if not set
	if config.HealthConfig == nil {
		config.HealthConfig = &HealthCheckConfig{
			Mode:           "on_demand",
			HealthEndpoint: "/health",
			CheckTimeout:   30 * time.Second,
			CacheDuration:  5 * time.Minute,
			StaleThreshold: 10 * time.Minute,
		}
	}

	// Generate cache key
	cacheKey := fmt.Sprintf("health:%s:%s", config.TenantID, config.ID)

	// Check cache if not forcing
	if !force {
		var cachedStatus HealthStatus
		if err := m.cache.Get(ctx, cacheKey, &cachedStatus); err == nil {
			// Check if cache is still valid
			if time.Since(cachedStatus.LastChecked) < config.HealthConfig.CacheDuration {
				return &cachedStatus, nil
			}
		}
	}

	// Get or create check entry
	m.mu.Lock()
	entry, exists := m.checks[config.ID]
	if !exists {
		entry = &healthCheckEntry{
			config: config,
		}
		m.checks[config.ID] = entry
	}
	m.mu.Unlock()

	// Prevent concurrent checks for the same tool
	entry.checkingMu.Lock()
	if entry.checking {
		entry.checkingMu.Unlock()
		// Return last known status
		return &entry.lastStatus, nil
	}
	entry.checking = true
	entry.checkingMu.Unlock()

	defer func() {
		entry.checkingMu.Lock()
		entry.checking = false
		entry.checkingMu.Unlock()
	}()

	// Record check start time
	startTime := time.Now()

	// Perform health check
	status := m.performHealthCheck(ctx, config)

	// Record metrics
	m.recordHealthMetrics(config, status, time.Since(startTime))

	// Update entry
	entry.lastCheck = time.Now()
	entry.lastStatus = *status

	// Cache the result
	if err := m.cache.Set(ctx, cacheKey, status, config.HealthConfig.CacheDuration); err != nil {
		m.logger.Warn("Failed to cache health status", map[string]interface{}{
			"tool_id": config.ID,
			"error":   err.Error(),
		})
	}

	return status, nil
}

// performHealthCheck performs the actual health check
func (m *HealthCheckManager) performHealthCheck(ctx context.Context, config ToolConfig) *HealthStatus {
	// Create timeout context
	checkCtx, cancel := context.WithTimeout(ctx, config.HealthConfig.CheckTimeout)
	defer cancel()

	startTime := time.Now()

	// Test connection using the handler
	err := m.handler.TestConnection(checkCtx, config)
	responseTime := int(time.Since(startTime).Milliseconds())

	if err != nil {
		return &HealthStatus{
			IsHealthy:    false,
			LastChecked:  time.Now(),
			ResponseTime: responseTime,
			Error:        err.Error(),
		}
	}

	// Try to get version and additional details
	details := make(map[string]interface{})

	// If handler supports extended health checks, use them
	if healthChecker, ok := m.handler.(ExtendedHealthChecker); ok {
		version, capabilities, checkErr := healthChecker.GetHealthDetails(checkCtx, config)
		if checkErr == nil {
			details["version"] = version
			details["capabilities"] = capabilities
		}
	}

	return &HealthStatus{
		IsHealthy:    true,
		LastChecked:  time.Now(),
		ResponseTime: responseTime,
		Details:      details,
	}
}

// recordHealthMetrics records health check metrics
func (m *HealthCheckManager) recordHealthMetrics(config ToolConfig, status *HealthStatus, duration time.Duration) {
	// Record check duration
	m.metrics.RecordHistogram("tool_health_check_duration", duration.Seconds(), map[string]string{
		"tool_name": config.Name,
		"tenant_id": config.TenantID,
		"healthy":   fmt.Sprintf("%t", status.IsHealthy),
	})

	// Record health status
	healthValue := 0.0
	if status.IsHealthy {
		healthValue = 1.0
	}
	m.metrics.RecordGauge("tool_availability", healthValue, map[string]string{
		"tool_name": config.Name,
		"tool_id":   config.ID,
		"tenant_id": config.TenantID,
	})

	// Record response time
	if status.ResponseTime > 0 {
		m.metrics.RecordHistogram("tool_response_time", float64(status.ResponseTime)/1000.0, map[string]string{
			"tool_name": config.Name,
			"tenant_id": config.TenantID,
		})
	}
}

// GetCachedStatus returns cached health status without performing a check
func (m *HealthCheckManager) GetCachedStatus(ctx context.Context, config ToolConfig) (*HealthStatus, bool) {
	cacheKey := fmt.Sprintf("health:%s:%s", config.TenantID, config.ID)

	var status HealthStatus
	if err := m.cache.Get(ctx, cacheKey, &status); err == nil {
		// Check if stale
		staleThreshold := 10 * time.Minute // default
		if config.HealthConfig != nil && config.HealthConfig.StaleThreshold > 0 {
			staleThreshold = config.HealthConfig.StaleThreshold
		}
		isStale := time.Since(status.LastChecked) > staleThreshold
		return &status, !isStale
	}

	// Check in-memory
	m.mu.RLock()
	entry, exists := m.checks[config.ID]
	m.mu.RUnlock()

	if exists && !entry.lastStatus.LastChecked.IsZero() {
		isStale := time.Since(entry.lastStatus.LastChecked) > config.HealthConfig.StaleThreshold
		return &entry.lastStatus, !isStale
	}

	return nil, false
}

// InvalidateCache invalidates cached health status
func (m *HealthCheckManager) InvalidateCache(ctx context.Context, config ToolConfig) error {
	cacheKey := fmt.Sprintf("health:%s:%s", config.TenantID, config.ID)
	return m.cache.Delete(ctx, cacheKey)
}

// StartPeriodicChecks starts periodic health checks for tools configured with periodic mode
func (m *HealthCheckManager) StartPeriodicChecks(ctx context.Context) {
	// This would be implemented if we supported periodic health checks
	// For MVP, we're using on-demand checks only
}

// ExtendedHealthChecker interface for plugins that support extended health info
type ExtendedHealthChecker interface {
	GetHealthDetails(ctx context.Context, config ToolConfig) (version string, capabilities []string, err error)
}

// HealthCheckResult represents the result of a batch health check
type HealthCheckResult struct {
	ToolID   string       `json:"tool_id"`
	ToolName string       `json:"tool_name"`
	Status   HealthStatus `json:"status"`
	Error    string       `json:"error,omitempty"`
}

// CheckMultiple performs health checks for multiple tools
func (m *HealthCheckManager) CheckMultiple(ctx context.Context, configs []ToolConfig) []HealthCheckResult {
	results := make([]HealthCheckResult, len(configs))

	// Use goroutines for parallel checking
	var wg sync.WaitGroup
	wg.Add(len(configs))

	for i, config := range configs {
		go func(idx int, cfg ToolConfig) {
			defer wg.Done()

			status, err := m.CheckHealth(ctx, cfg, false)
			result := HealthCheckResult{
				ToolID:   cfg.ID,
				ToolName: cfg.Name,
			}

			if err != nil {
				result.Error = err.Error()
				result.Status = HealthStatus{
					IsHealthy:   false,
					LastChecked: time.Now(),
					Error:       err.Error(),
				}
			} else {
				result.Status = *status
			}

			results[idx] = result
		}(i, config)
	}

	wg.Wait()
	return results
}
