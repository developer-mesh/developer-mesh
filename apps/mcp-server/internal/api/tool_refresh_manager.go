package api

import (
	"context"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/clients"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// ToolRefreshManager manages automatic refresh of tool definitions
type ToolRefreshManager struct {
	client          clients.RESTAPIClient
	handler         *MCPProtocolHandler
	refreshInterval time.Duration
	logger          observability.Logger

	stopCh      chan struct{}
	wg          sync.WaitGroup
	lastRefresh time.Time
	mu          sync.RWMutex

	// Track refresh metrics
	refreshCount uint64
	failureCount uint64
	lastError    error
	isRunning    bool
}

// NewToolRefreshManager creates a new tool refresh manager
func NewToolRefreshManager(
	client clients.RESTAPIClient,
	handler *MCPProtocolHandler,
	interval time.Duration,
	logger observability.Logger,
) *ToolRefreshManager {
	if interval == 0 {
		interval = 5 * time.Minute // Default interval
	}
	return &ToolRefreshManager{
		client:          client,
		handler:         handler,
		refreshInterval: interval,
		logger:          logger,
		stopCh:          make(chan struct{}),
	}
}

// Start begins the automatic refresh process
func (m *ToolRefreshManager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.isRunning {
		m.mu.Unlock()
		return nil // Already running
	}
	m.isRunning = true
	m.mu.Unlock()

	m.wg.Add(1)
	go m.refreshLoop(ctx)

	m.logger.Info("Tool refresh manager started", map[string]interface{}{
		"interval": m.refreshInterval.String(),
	})
	return nil
}

// Stop gracefully stops the refresh manager
func (m *ToolRefreshManager) Stop() {
	m.mu.Lock()
	if !m.isRunning {
		m.mu.Unlock()
		return
	}
	m.isRunning = false
	m.mu.Unlock()

	close(m.stopCh)
	m.wg.Wait()

	m.logger.Info("Tool refresh manager stopped", map[string]interface{}{
		"refresh_count": m.refreshCount,
		"failure_count": m.failureCount,
	})
}

// refreshLoop runs the periodic refresh cycle
func (m *ToolRefreshManager) refreshLoop(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(m.refreshInterval)
	defer ticker.Stop()

	// Initial refresh
	m.refresh(ctx)

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("Refresh loop stopped due to context cancellation", nil)
			return
		case <-m.stopCh:
			m.logger.Info("Refresh loop stopped", nil)
			return
		case <-ticker.C:
			m.refresh(ctx)
		}
	}
}

// refresh performs a tool cache refresh
func (m *ToolRefreshManager) refresh(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	startTime := time.Now()

	// Clear the tools cache to force refresh on next request
	if m.handler.toolsCache != nil {
		m.handler.toolsCache.Clear()
		m.logger.Debug("Tool cache cleared", nil)
	}

	// Clear tool name cache to ensure fresh mappings
	m.handler.toolNameCacheMu.Lock()
	oldCacheSize := len(m.handler.toolNameCache)
	m.handler.toolNameCache = make(map[string]map[string]string)
	m.handler.toolNameCacheMu.Unlock()

	m.lastRefresh = time.Now()
	m.refreshCount++

	// Optionally pre-fetch tools to warm the cache
	// This is done in a non-blocking way to avoid delays
	// Only warm cache if we're not in test mode (client is real)
	if m.client != nil {
		go m.warmCache(ctx)
	}

	m.logger.Info("Tools cache refreshed", map[string]interface{}{
		"timestamp":      m.lastRefresh,
		"duration":       time.Since(startTime).String(),
		"refresh_count":  m.refreshCount,
		"old_cache_size": oldCacheSize,
	})
}

// warmCache pre-fetches tools to warm the cache
func (m *ToolRefreshManager) warmCache(ctx context.Context) {
	// Use a short timeout for warming
	warmCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Try to fetch tools for common tenants
	// This is optional and failures are not critical
	if m.client != nil {
		if _, err := m.client.ListTools(warmCtx, "default-tenant"); err != nil {
			m.logger.Debug("Failed to warm cache for default tenant", map[string]interface{}{
				"error": err.Error(),
			})
			m.mu.Lock()
			m.failureCount++
			m.lastError = err
			m.mu.Unlock()
		}
	}
}

// ForceRefresh triggers an immediate refresh
func (m *ToolRefreshManager) ForceRefresh(ctx context.Context) error {
	m.logger.Info("Force refresh requested", nil)
	m.refresh(ctx)
	return nil
}

// GetStatus returns the current status of the refresh manager
func (m *ToolRefreshManager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := map[string]interface{}{
		"is_running":       m.isRunning,
		"refresh_interval": m.refreshInterval.String(),
		"refresh_count":    m.refreshCount,
		"failure_count":    m.failureCount,
		"last_refresh":     m.lastRefresh.Format(time.RFC3339),
	}

	if m.lastError != nil {
		status["last_error"] = m.lastError.Error()
	}

	if !m.lastRefresh.IsZero() {
		status["next_refresh"] = m.lastRefresh.Add(m.refreshInterval).Format(time.RFC3339)
		status["time_until_refresh"] = time.Until(m.lastRefresh.Add(m.refreshInterval)).String()
	}

	return status
}

// SetInterval updates the refresh interval
func (m *ToolRefreshManager) SetInterval(interval time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldInterval := m.refreshInterval
	m.refreshInterval = interval

	m.logger.Info("Refresh interval updated", map[string]interface{}{
		"old_interval": oldInterval.String(),
		"new_interval": interval.String(),
	})
}
