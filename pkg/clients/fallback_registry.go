package clients

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// FallbackRegistry provides cached tool information for graceful degradation
type FallbackRegistry struct {
	mu              sync.RWMutex
	tools           map[string]*models.DynamicTool
	lastUpdate      time.Time
	maxAge          time.Duration
	logger          observability.Logger
	persistencePath string
}

// NewFallbackRegistry creates a new fallback registry
func NewFallbackRegistry(logger observability.Logger, persistencePath string) *FallbackRegistry {
	return &FallbackRegistry{
		tools:           make(map[string]*models.DynamicTool),
		maxAge:          24 * time.Hour, // Keep fallback data for 24 hours
		logger:          logger,
		persistencePath: persistencePath,
	}
}

// UpdateTools updates the fallback registry with current tool information
func (r *FallbackRegistry) UpdateTools(tools []*models.DynamicTool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear existing tools
	r.tools = make(map[string]*models.DynamicTool)

	// Add new tools
	for _, tool := range tools {
		if tool != nil && tool.ID != "" {
			r.tools[tool.ID] = tool
		}
	}

	r.lastUpdate = time.Now()

	// Persist to disk for recovery after restart
	if r.persistencePath != "" {
		if err := r.persistToDisk(); err != nil {
			r.logger.Warn("Failed to persist fallback registry", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	r.logger.Info("Updated fallback registry", map[string]interface{}{
		"tool_count": len(r.tools),
		"timestamp":  r.lastUpdate.Format(time.RFC3339),
	})

	return nil
}

// GetTools returns cached tools from the fallback registry
func (r *FallbackRegistry) GetTools() ([]*models.DynamicTool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check if data is too old
	if time.Since(r.lastUpdate) > r.maxAge {
		r.logger.Warn("Fallback registry data is stale", map[string]interface{}{
			"age":     time.Since(r.lastUpdate).String(),
			"max_age": r.maxAge.String(),
		})
		return nil, false
	}

	// Convert map to slice
	tools := make([]*models.DynamicTool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}

	r.logger.Info("Serving tools from fallback registry", map[string]interface{}{
		"tool_count": len(tools),
		"age":        time.Since(r.lastUpdate).String(),
	})

	return tools, true
}

// GetTool returns a specific tool from the fallback registry
func (r *FallbackRegistry) GetTool(toolID string) (*models.DynamicTool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check if data is too old
	if time.Since(r.lastUpdate) > r.maxAge {
		return nil, false
	}

	tool, exists := r.tools[toolID]
	if !exists {
		return nil, false
	}

	r.logger.Debug("Serving tool from fallback registry", map[string]interface{}{
		"tool_id": toolID,
		"age":     time.Since(r.lastUpdate).String(),
	})

	return tool, true
}

// IsStale returns true if the fallback data is older than maxAge
func (r *FallbackRegistry) IsStale() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return time.Since(r.lastUpdate) > r.maxAge
}

// GetAge returns how old the fallback data is
func (r *FallbackRegistry) GetAge() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return time.Since(r.lastUpdate)
}

// persistToDisk saves the registry to disk for recovery
func (r *FallbackRegistry) persistToDisk() error {
	// Implementation would save to file
	// For now, this is a placeholder
	return nil
}

// FallbackClient wraps a REST client with fallback capabilities
type FallbackClient struct {
	primary  RESTAPIClient
	fallback *FallbackRegistry
	logger   observability.Logger
	metrics  *FallbackMetrics
}

// FallbackMetrics tracks fallback usage
type FallbackMetrics struct {
	mu                 sync.RWMutex
	FallbackHits       int64
	FallbackMisses     int64
	PrimaryFailures    int64
	LastFallbackUse    time.Time
	LastPrimarySuccess time.Time
}

// NewFallbackClient creates a new client with fallback capabilities
func NewFallbackClient(primary RESTAPIClient, logger observability.Logger) *FallbackClient {
	return &FallbackClient{
		primary:  primary,
		fallback: NewFallbackRegistry(logger, ""),
		logger:   logger,
		metrics:  &FallbackMetrics{},
	}
}

// ListTools attempts to list tools with fallback
func (c *FallbackClient) ListTools(ctx context.Context, tenantID string) ([]*models.DynamicTool, error) {
	// Try primary source first
	tools, err := c.primary.ListTools(ctx, tenantID)
	if err == nil {
		// Update fallback registry with fresh data
		_ = c.fallback.UpdateTools(tools)
		c.metrics.mu.Lock()
		c.metrics.LastPrimarySuccess = time.Now()
		c.metrics.mu.Unlock()
		return tools, nil
	}

	// Primary failed, log the error
	c.logger.Warn("Primary REST API failed, attempting fallback", map[string]interface{}{
		"error":     err.Error(),
		"tenant_id": tenantID,
	})

	c.metrics.mu.Lock()
	c.metrics.PrimaryFailures++
	c.metrics.mu.Unlock()

	// Try fallback registry
	if fallbackTools, ok := c.fallback.GetTools(); ok {
		c.metrics.mu.Lock()
		c.metrics.FallbackHits++
		c.metrics.LastFallbackUse = time.Now()
		c.metrics.mu.Unlock()

		c.logger.Info("Using fallback registry for tool list", map[string]interface{}{
			"tool_count": len(fallbackTools),
			"age":        c.fallback.GetAge().String(),
		})

		return fallbackTools, nil
	}

	c.metrics.mu.Lock()
	c.metrics.FallbackMisses++
	c.metrics.mu.Unlock()

	// Both primary and fallback failed
	return nil, fmt.Errorf("both primary and fallback sources failed: %w", err)
}

// GetTool attempts to get a tool with fallback
func (c *FallbackClient) GetTool(ctx context.Context, tenantID, toolID string) (*models.DynamicTool, error) {
	// Try primary source first
	tool, err := c.primary.GetTool(ctx, tenantID, toolID)
	if err == nil {
		c.metrics.mu.Lock()
		c.metrics.LastPrimarySuccess = time.Now()
		c.metrics.mu.Unlock()
		return tool, nil
	}

	// Primary failed
	c.metrics.mu.Lock()
	c.metrics.PrimaryFailures++
	c.metrics.mu.Unlock()

	// Try fallback registry
	if fallbackTool, ok := c.fallback.GetTool(toolID); ok {
		c.metrics.mu.Lock()
		c.metrics.FallbackHits++
		c.metrics.LastFallbackUse = time.Now()
		c.metrics.mu.Unlock()

		c.logger.Info("Using fallback registry for tool", map[string]interface{}{
			"tool_id": toolID,
			"age":     c.fallback.GetAge().String(),
		})

		return fallbackTool, nil
	}

	c.metrics.mu.Lock()
	c.metrics.FallbackMisses++
	c.metrics.mu.Unlock()

	return nil, fmt.Errorf("tool not found in primary or fallback: %w", err)
}

// ExecuteTool always uses primary (no fallback for execution)
func (c *FallbackClient) ExecuteTool(ctx context.Context, tenantID, toolID, action string, params map[string]interface{}) (*models.ToolExecutionResponse, error) {
	// Tool execution cannot use fallback - must go through primary
	result, err := c.primary.ExecuteTool(ctx, tenantID, toolID, action, params)
	if err != nil {
		c.metrics.mu.Lock()
		c.metrics.PrimaryFailures++
		c.metrics.mu.Unlock()
	} else {
		c.metrics.mu.Lock()
		c.metrics.LastPrimarySuccess = time.Now()
		c.metrics.mu.Unlock()
	}
	return result, err
}

// GetToolHealth uses primary only
func (c *FallbackClient) GetToolHealth(ctx context.Context, tenantID, toolID string) (*models.HealthStatus, error) {
	return c.primary.GetToolHealth(ctx, tenantID, toolID)
}

// HealthCheck checks primary health
func (c *FallbackClient) HealthCheck(ctx context.Context) error {
	return c.primary.HealthCheck(ctx)
}

// GetMetrics returns combined metrics
func (c *FallbackClient) GetMetrics() ClientMetrics {
	primaryMetrics := c.primary.GetMetrics()

	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()

	// Add fallback metrics to primary metrics
	if primaryMetrics.Metadata == nil {
		primaryMetrics.Metadata = make(map[string]interface{})
	}

	primaryMetrics.Metadata["fallback_hits"] = c.metrics.FallbackHits
	primaryMetrics.Metadata["fallback_misses"] = c.metrics.FallbackMisses
	primaryMetrics.Metadata["primary_failures_since_fallback"] = c.metrics.PrimaryFailures
	primaryMetrics.Metadata["last_fallback_use"] = c.metrics.LastFallbackUse
	primaryMetrics.Metadata["last_primary_success"] = c.metrics.LastPrimarySuccess
	primaryMetrics.Metadata["fallback_age"] = c.fallback.GetAge().String()
	primaryMetrics.Metadata["fallback_stale"] = c.fallback.IsStale()

	return primaryMetrics
}

// Close closes the client
func (c *FallbackClient) Close() error {
	return c.primary.Close()
}

// GetFallbackMetrics returns detailed fallback metrics
func (c *FallbackClient) GetFallbackMetrics() map[string]interface{} {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()

	return map[string]interface{}{
		"fallback_hits":        c.metrics.FallbackHits,
		"fallback_misses":      c.metrics.FallbackMisses,
		"primary_failures":     c.metrics.PrimaryFailures,
		"last_fallback_use":    c.metrics.LastFallbackUse,
		"last_primary_success": c.metrics.LastPrimarySuccess,
		"fallback_age":         c.fallback.GetAge().String(),
		"fallback_stale":       c.fallback.IsStale(),
		"fallback_tool_count":  len(c.fallback.tools),
	}
}
