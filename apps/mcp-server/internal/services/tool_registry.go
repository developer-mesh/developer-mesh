package services

import (
	"context"
	"fmt"
	"sync"

	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/adapters/openapi"
	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/core/tool"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// ToolRegistry manages the registration and execution of dynamic tools
type ToolRegistry struct {
	toolService    *ToolService
	openAPIAdapter *openapi.OpenAPIAdapter
	healthChecker  *HealthChecker
	logger         observability.Logger

	// Cache for loaded tools per tenant
	mu         sync.RWMutex
	toolsCache map[string]map[string][]*tool.DynamicTool // tenantID -> toolName -> tools
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(
	toolService *ToolService,
	healthChecker *HealthChecker,
	logger observability.Logger,
) *ToolRegistry {
	return &ToolRegistry{
		toolService:    toolService,
		openAPIAdapter: openapi.NewOpenAPIAdapter(),
		healthChecker:  healthChecker,
		logger:         logger,
		toolsCache:     make(map[string]map[string][]*tool.DynamicTool),
	}
}

// RegisterTool registers a new tool for a tenant
func (r *ToolRegistry) RegisterTool(ctx context.Context, tenantID string, config *tool.ToolConfig, createdBy string) (*tool.DiscoveryResult, error) {
	// First, discover the OpenAPI spec
	result, err := r.openAPIAdapter.DiscoverAPIs(ctx, *config)
	if err != nil {
		return nil, fmt.Errorf("failed to discover APIs: %w", err)
	}

	if result.Status != "success" || result.OpenAPISpec == nil {
		return result, fmt.Errorf("API discovery failed: %s", result.Error)
	}

	// Store the discovered OpenAPI URL in config
	if len(result.DiscoveredURLs) > 0 {
		config.Config["openapi_url"] = result.DiscoveredURLs[0]
	}

	// Create the tool configuration in database
	if err := r.toolService.CreateTool(ctx, tenantID, config, createdBy); err != nil {
		return nil, fmt.Errorf("failed to create tool: %w", err)
	}

	// Clear cache for this tenant to force reload
	r.clearTenantCache(tenantID)

	// Test the connection
	if err := r.openAPIAdapter.TestConnection(ctx, *config); err != nil {
		r.logger.Warn("Tool registered but connection test failed", map[string]interface{}{
			"tool_name": config.Name,
			"error":     err.Error(),
		})
	}

	return result, nil
}

// GetTool retrieves a specific tool for a tenant
func (r *ToolRegistry) GetTool(ctx context.Context, tenantID, toolName string) (*tool.ToolConfig, error) {
	return r.toolService.GetTool(ctx, tenantID, toolName)
}

// GetToolByType retrieves a tool by type (for migration compatibility)
func (r *ToolRegistry) GetToolByType(ctx context.Context, tenantID, toolType string) (*tool.ToolConfig, error) {
	return r.toolService.GetToolByType(ctx, tenantID, toolType)
}

// GetToolForTenant retrieves a tool with loaded actions for a tenant
func (r *ToolRegistry) GetToolForTenant(ctx context.Context, tenantID, toolName string) (*tool.ToolConfig, error) {
	config, err := r.toolService.GetTool(ctx, tenantID, toolName)
	if err != nil {
		return nil, err
	}

	// Check health status from cache
	health := r.healthChecker.GetCachedHealth(config.ID)
	if health != nil {
		config.HealthStatus = "healthy"
		if !health.IsHealthy {
			config.HealthStatus = "unhealthy"
		}
		config.LastHealthCheck = &health.LastChecked
	}

	return config, nil
}

// ListToolsForTenant returns all tools for a tenant
func (r *ToolRegistry) ListToolsForTenant(ctx context.Context, tenantID string) ([]*tool.ToolConfig, error) {
	return r.toolService.ListTools(ctx, tenantID)
}

// GetToolActions returns the available actions for a tool
func (r *ToolRegistry) GetToolActions(ctx context.Context, tenantID, toolName string) ([]*tool.DynamicTool, error) {
	// Check cache first
	r.mu.RLock()
	if tenantTools, ok := r.toolsCache[tenantID]; ok {
		if tools, ok := tenantTools[toolName]; ok {
			r.mu.RUnlock()
			return tools, nil
		}
	}
	r.mu.RUnlock()

	// Load from OpenAPI spec
	config, err := r.toolService.GetTool(ctx, tenantID, toolName)
	if err != nil {
		return nil, err
	}

	// Discover and load OpenAPI spec
	result, err := r.openAPIAdapter.DiscoverAPIs(ctx, *config)
	if err != nil {
		return nil, fmt.Errorf("failed to discover APIs: %w", err)
	}

	if result.OpenAPISpec == nil {
		return nil, fmt.Errorf("no OpenAPI specification found for tool")
	}

	// Generate tools from spec
	tools, err := r.openAPIAdapter.GenerateTools(*config, result.OpenAPISpec)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tools: %w", err)
	}

	// Cache the tools
	r.mu.Lock()
	if r.toolsCache[tenantID] == nil {
		r.toolsCache[tenantID] = make(map[string][]*tool.DynamicTool)
	}
	r.toolsCache[tenantID][toolName] = tools
	r.mu.Unlock()

	return tools, nil
}

// GetToolAction returns a specific action for a tool
func (r *ToolRegistry) GetToolAction(ctx context.Context, tenantID, toolName, actionName string) (*tool.DynamicTool, error) {
	actions, err := r.GetToolActions(ctx, tenantID, toolName)
	if err != nil {
		return nil, err
	}

	for _, action := range actions {
		if action.Name == actionName || action.OperationID == actionName {
			return action, nil
		}
	}

	return nil, fmt.Errorf("action %s not found for tool %s", actionName, toolName)
}

// UpdateTool updates a tool configuration
func (r *ToolRegistry) UpdateTool(ctx context.Context, tenantID, toolName string, updates map[string]interface{}) error {
	if err := r.toolService.UpdateTool(ctx, tenantID, toolName, updates); err != nil {
		return err
	}

	// Clear cache for this tenant
	r.clearTenantCache(tenantID)

	return nil
}

// DeleteTool removes a tool for a tenant
func (r *ToolRegistry) DeleteTool(ctx context.Context, tenantID, toolName string) error {
	if err := r.toolService.DeleteTool(ctx, tenantID, toolName); err != nil {
		return err
	}

	// Clear cache for this tenant
	r.clearTenantCache(tenantID)

	return nil
}

// TestConnection tests the connection to a tool
func (r *ToolRegistry) TestConnection(ctx context.Context, tenantID, toolName string) (*tool.HealthStatus, error) {
	config, err := r.toolService.GetTool(ctx, tenantID, toolName)
	if err != nil {
		return nil, err
	}

	// Force a fresh health check
	health := r.healthChecker.CheckHealth(ctx, config)

	// Update in database
	if err := r.toolService.UpdateHealthStatus(ctx, config.ID, health); err != nil {
		r.logger.Error("Failed to update health status", map[string]interface{}{
			"error":   err.Error(),
			"tool_id": config.ID,
		})
	}

	return health, nil
}

// GetOpenAPIAdapter returns the OpenAPI adapter for direct use
func (r *ToolRegistry) GetOpenAPIAdapter() *openapi.OpenAPIAdapter {
	return r.openAPIAdapter
}

// clearTenantCache clears the tool cache for a specific tenant
func (r *ToolRegistry) clearTenantCache(tenantID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.toolsCache, tenantID)
}
