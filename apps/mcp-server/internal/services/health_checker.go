package services

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/core/tool"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// HealthCheckCache represents a cached health check result
type HealthCheckCache struct {
	Status    *tool.HealthStatus
	ExpiresAt time.Time
}

// HealthChecker manages health checks for tools
type HealthChecker struct {
	httpClient    *http.Client
	logger        observability.Logger
	cacheDuration time.Duration
	checkTimeout  time.Duration

	// Cache for health check results
	mu    sync.RWMutex
	cache map[string]*HealthCheckCache // toolID -> cached result
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(logger observability.Logger) *HealthChecker {
	return &HealthChecker{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger:        logger,
		cacheDuration: 5 * time.Minute, // Default cache duration
		checkTimeout:  5 * time.Second, // Default check timeout
		cache:         make(map[string]*HealthCheckCache),
	}
}

// CheckHealth performs a health check on a tool
func (h *HealthChecker) CheckHealth(ctx context.Context, config *tool.ToolConfig) *tool.HealthStatus {
	// Check cache first
	if cached := h.GetCachedHealth(config.ID); cached != nil && !h.isHealthStale(cached) {
		cached.WasCached = true
		return cached
	}

	// Perform fresh health check
	start := time.Now()

	// Create context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, h.checkTimeout)
	defer cancel()

	// Create health check request
	req, err := http.NewRequestWithContext(checkCtx, "GET", config.BaseURL, nil)
	if err != nil {
		status := &tool.HealthStatus{
			IsHealthy:    false,
			LastChecked:  time.Now(),
			ResponseTime: int(time.Since(start).Milliseconds()),
			Error:        fmt.Sprintf("failed to create request: %v", err),
			WasCached:    false,
		}
		h.cacheHealthStatus(config.ID, status)
		return status
	}

	// Add authentication if available
	if config.Credential != nil {
		switch config.Credential.Type {
		case "bearer":
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Credential.Token))
		case "api_key":
			headerName := config.Credential.HeaderName
			if headerName == "" {
				headerName = "X-API-Key"
			}
			req.Header.Set(headerName, config.Credential.APIKey)
		case "basic":
			req.SetBasicAuth(config.Credential.Username, config.Credential.Password)
		}
	}

	// Execute request
	resp, err := h.httpClient.Do(req)
	responseTime := int(time.Since(start).Milliseconds())

	if err != nil {
		status := &tool.HealthStatus{
			IsHealthy:    false,
			LastChecked:  time.Now(),
			ResponseTime: responseTime,
			Error:        fmt.Sprintf("request failed: %v", err),
			WasCached:    false,
		}
		h.cacheHealthStatus(config.ID, status)
		return status
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response status
	isHealthy := resp.StatusCode >= 200 && resp.StatusCode < 400
	var errorMsg string
	if !isHealthy {
		errorMsg = fmt.Sprintf("unhealthy status code: %d", resp.StatusCode)
	}

	// Try to extract version from headers
	version := resp.Header.Get("X-API-Version")
	if version == "" {
		version = resp.Header.Get("X-Version")
	}

	status := &tool.HealthStatus{
		IsHealthy:    isHealthy,
		LastChecked:  time.Now(),
		ResponseTime: responseTime,
		Error:        errorMsg,
		Version:      version,
		WasCached:    false,
	}

	// Cache the result
	h.cacheHealthStatus(config.ID, status)

	h.logger.Debug("Health check completed", map[string]interface{}{
		"tool_id":       config.ID,
		"tool_name":     config.Name,
		"is_healthy":    isHealthy,
		"response_time": responseTime,
		"cached":        false,
	})

	return status
}

// GetCachedHealth returns a cached health status if available
func (h *HealthChecker) GetCachedHealth(toolID string) *tool.HealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()

	cached, exists := h.cache[toolID]
	if !exists || cached == nil {
		return nil
	}

	// Check if cache is expired
	if time.Now().After(cached.ExpiresAt) {
		return nil
	}

	// Return a copy to avoid mutations
	status := *cached.Status
	status.WasCached = true
	return &status
}

// IsStale checks if a health status is stale
func (h *HealthChecker) IsStale(status *tool.HealthStatus) bool {
	if status == nil {
		return true
	}

	// Consider stale if older than cache duration
	return time.Since(status.LastChecked) > h.cacheDuration
}

// cacheHealthStatus stores a health status in the cache
func (h *HealthChecker) cacheHealthStatus(toolID string, status *tool.HealthStatus) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cache[toolID] = &HealthCheckCache{
		Status:    status,
		ExpiresAt: time.Now().Add(h.cacheDuration),
	}
}

// ClearCache removes all cached health statuses
func (h *HealthChecker) ClearCache() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cache = make(map[string]*HealthCheckCache)
}

// ClearToolCache removes cached health status for a specific tool
func (h *HealthChecker) ClearToolCache(toolID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.cache, toolID)
}

// SetCacheDuration updates the cache duration
func (h *HealthChecker) SetCacheDuration(duration time.Duration) {
	h.cacheDuration = duration
}

// SetCheckTimeout updates the health check timeout
func (h *HealthChecker) SetCheckTimeout(timeout time.Duration) {
	h.checkTimeout = timeout
}

// StartPeriodicCleanup starts a goroutine that periodically cleans expired cache entries
func (h *HealthChecker) StartPeriodicCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.cleanupExpiredCache()
			}
		}
	}()
}

// cleanupExpiredCache removes expired entries from the cache
func (h *HealthChecker) cleanupExpiredCache() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	for toolID, cached := range h.cache {
		if now.After(cached.ExpiresAt) {
			delete(h.cache, toolID)
		}
	}
}

// GetCacheStats returns statistics about the health check cache
func (h *HealthChecker) GetCacheStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	validCount := 0
	now := time.Now()

	for _, cached := range h.cache {
		if now.Before(cached.ExpiresAt) {
			validCount++
		}
	}

	return map[string]interface{}{
		"total_entries":  len(h.cache),
		"valid_entries":  validCount,
		"cache_duration": h.cacheDuration.String(),
		"check_timeout":  h.checkTimeout.String(),
	}
}

// isHealthStale checks if a health status is stale
func (h *HealthChecker) isHealthStale(status *tool.HealthStatus) bool {
	// Consider stale if older than cache duration
	return time.Since(status.LastChecked) > h.cacheDuration
}
