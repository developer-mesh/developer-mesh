package tools

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// HealthCheckScheduler manages scheduled health checks for tools
type HealthCheckScheduler struct {
	manager     *HealthCheckManager
	db          HealthCheckDB // Database interface
	logger      observability.Logger
	interval    time.Duration
	mu          sync.RWMutex
	running     bool
	stopCh      chan struct{}
	toolConfigs map[string]ToolConfig // Cache of tools to check
}

// HealthCheckDB defines the database interface for health checks
type HealthCheckDB interface {
	GetActiveToolsForHealthCheck(ctx context.Context) ([]ToolConfig, error)
	UpdateToolHealthStatus(ctx context.Context, tenantID, toolID string, status *HealthStatus) error
}

// NewHealthCheckScheduler creates a new health check scheduler
func NewHealthCheckScheduler(
	manager *HealthCheckManager,
	db HealthCheckDB,
	logger observability.Logger,
	interval time.Duration,
) *HealthCheckScheduler {
	return &HealthCheckScheduler{
		manager:     manager,
		db:          db,
		logger:      logger,
		interval:    interval,
		stopCh:      make(chan struct{}),
		toolConfigs: make(map[string]ToolConfig),
	}
}

// Start begins the scheduled health check process
func (s *HealthCheckScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.mu.Unlock()

	s.logger.Info("Starting health check scheduler", map[string]interface{}{
		"interval": s.interval.String(),
	})

	// Initial load of tools
	if err := s.loadTools(ctx); err != nil {
		s.logger.Error("Failed to load tools for health check", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Start the scheduler
	go s.run(ctx)

	return nil
}

// Stop stops the scheduled health check process
func (s *HealthCheckScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.logger.Info("Stopping health check scheduler", nil)
	close(s.stopCh)
	s.running = false
}

// run is the main scheduler loop
func (s *HealthCheckScheduler) run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Also reload tools periodically
	reloadTicker := time.NewTicker(5 * time.Minute)
	defer reloadTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Health check scheduler context cancelled", nil)
			return
		case <-s.stopCh:
			s.logger.Info("Health check scheduler stopped", nil)
			return
		case <-reloadTicker.C:
			// Reload tools
			if err := s.loadTools(ctx); err != nil {
				s.logger.Error("Failed to reload tools", map[string]interface{}{
					"error": err.Error(),
				})
			}
		case <-ticker.C:
			// Perform health checks
			s.performHealthChecks(ctx)
		}
	}
}

// performHealthChecks runs health checks for all configured tools
func (s *HealthCheckScheduler) performHealthChecks(ctx context.Context) {
	s.mu.RLock()
	configs := make([]ToolConfig, 0, len(s.toolConfigs))
	for _, config := range s.toolConfigs {
		configs = append(configs, config)
	}
	s.mu.RUnlock()

	s.logger.Debug("Performing scheduled health checks", map[string]interface{}{
		"tool_count": len(configs),
	})

	// Check tools in parallel with bounded concurrency
	sem := make(chan struct{}, 10) // Limit to 10 concurrent checks
	var wg sync.WaitGroup

	for _, config := range configs {
		// Skip if health check is disabled for this tool
		if config.HealthConfig != nil && config.HealthConfig.Mode == "disabled" {
			continue
		}

		wg.Add(1)
		go func(cfg ToolConfig) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			// Create timeout context for individual check
			checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			// Perform health check
			status, err := s.manager.CheckHealth(checkCtx, cfg, false)
			if err != nil {
				s.logger.Error("Scheduled health check failed", map[string]interface{}{
					"tool_id":   cfg.ID,
					"tool_name": cfg.Name,
					"error":     err.Error(),
				})
				return
			}

			// Log result
			s.logger.Debug("Scheduled health check completed", map[string]interface{}{
				"tool_id":       cfg.ID,
				"tool_name":     cfg.Name,
				"is_healthy":    status.IsHealthy,
				"response_time": status.ResponseTime,
			})

			// Update database with health status
			s.updateHealthStatus(cfg.TenantID, cfg.ID, status)
		}(config)
	}

	wg.Wait()
}

// loadTools loads tools that need health checking from the database
func (s *HealthCheckScheduler) loadTools(ctx context.Context) error {
	s.logger.Info("Loading tools for health check scheduler", nil)

	// Query database for active tools with health checks enabled
	tools, err := s.db.GetActiveToolsForHealthCheck(ctx)
	if err != nil {
		return fmt.Errorf("failed to load tools from database: %w", err)
	}

	// Update the toolConfigs map
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing configs
	s.toolConfigs = make(map[string]ToolConfig)

	// Add new configs
	for _, tool := range tools {
		// Only add tools with health check enabled
		if tool.HealthConfig == nil || tool.HealthConfig.Mode == "disabled" {
			continue
		}

		s.toolConfigs[tool.ID] = tool
	}

	s.logger.Info("Loaded tools for health check", map[string]interface{}{
		"count": len(s.toolConfigs),
	})

	return nil
}

// updateHealthStatus updates the health status in the database
func (s *HealthCheckScheduler) updateHealthStatus(tenantID, toolID string, status *HealthStatus) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.db.UpdateToolHealthStatus(ctx, tenantID, toolID, status); err != nil {
		s.logger.Error("Failed to update health status in database", map[string]interface{}{
			"tenant_id":  tenantID,
			"tool_id":    toolID,
			"is_healthy": status.IsHealthy,
			"error":      err.Error(),
		})
	} else {
		s.logger.Debug("Updated health status in database", map[string]interface{}{
			"tenant_id":  tenantID,
			"tool_id":    toolID,
			"is_healthy": status.IsHealthy,
			"last_check": status.LastChecked,
		})
	}
}

// AddTool adds a tool to be health checked
func (s *HealthCheckScheduler) AddTool(config ToolConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.toolConfigs[config.ID] = config
	s.logger.Info("Added tool to health check scheduler", map[string]interface{}{
		"tool_id":   config.ID,
		"tool_name": config.Name,
	})
}

// RemoveTool removes a tool from health checking
func (s *HealthCheckScheduler) RemoveTool(toolID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if config, exists := s.toolConfigs[toolID]; exists {
		delete(s.toolConfigs, toolID)
		s.logger.Info("Removed tool from health check scheduler", map[string]interface{}{
			"tool_id":   toolID,
			"tool_name": config.Name,
		})
	}
}

// GetScheduledTools returns the list of tools being health checked
func (s *HealthCheckScheduler) GetScheduledTools() []ToolConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]ToolConfig, 0, len(s.toolConfigs))
	for _, config := range s.toolConfigs {
		tools = append(tools, config)
	}

	return tools
}
