package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/services"
	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/gin-gonic/gin"
)

// DynamicToolsAPI handles dynamic tool management endpoints
type DynamicToolsAPI struct {
	toolService   services.DynamicToolsServiceInterface
	logger        observability.Logger
	metricsClient observability.MetricsClient
	auditLogger   *auth.AuditLogger
}

// NewDynamicToolsAPI creates a new dynamic tools API handler
func NewDynamicToolsAPI(
	toolService services.DynamicToolsServiceInterface,
	logger observability.Logger,
	metricsClient observability.MetricsClient,
	auditLogger *auth.AuditLogger,
) *DynamicToolsAPI {
	return &DynamicToolsAPI{
		toolService:   toolService,
		logger:        logger,
		metricsClient: metricsClient,
		auditLogger:   auditLogger,
	}
}

// RegisterRoutes registers all dynamic tool API routes
func (api *DynamicToolsAPI) RegisterRoutes(router *gin.RouterGroup) {
	tools := router.Group("/tools")
	{
		// Tool management
		tools.GET("", api.ListTools)
		tools.POST("", api.CreateTool)
		tools.GET("/:toolId", api.GetTool)
		tools.PUT("/:toolId", api.UpdateTool)
		tools.DELETE("/:toolId", api.DeleteTool)

		// Discovery
		tools.POST("/discover", api.DiscoverTool)
		tools.GET("/discover/:sessionId", api.GetDiscoverySession)
		tools.POST("/discover/:sessionId/confirm", api.ConfirmDiscovery)

		// Multi-API Discovery
		tools.POST("/discover-multiple", api.DiscoverMultipleAPIs)
		tools.POST("/discover-multiple/create", api.CreateToolsFromMultipleAPIs)

		// Health checks
		tools.GET("/:toolId/health", api.CheckHealth)
		tools.POST("/:toolId/health/refresh", api.RefreshHealth)

		// Execution
		tools.POST("/:toolId/execute/:action", api.ExecuteAction)
		tools.GET("/:toolId/actions", api.ListActions)

		// Credentials
		tools.PUT("/:toolId/credentials", api.UpdateCredentials)

		// Webhook configuration
		tools.GET("/:toolId/webhook", api.GetWebhookConfig)
	}
}

// ListTools lists all configured tools for the tenant
// @Summary List dynamic tools
// @Description Lists all configured dynamic tools for the authenticated tenant
// @Tags Dynamic Tools
// @Accept json
// @Produce json
// @Param status query string false "Filter by status (active, inactive)"
// @Param include_health query bool false "Include health status information"
// @Success 200 {object} ListToolsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tools [get]
func (api *DynamicToolsAPI) ListTools(c *gin.Context) {
	start := time.Now()
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
		return
	}

	// Query parameters
	status := c.Query("status")
	includeHealth := c.Query("include_health") == "true"

	tools, err := api.toolService.ListTools(c.Request.Context(), tenantID, status)
	if err != nil {
		api.logger.Error("Failed to list tools", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		// Record failure metric
		if api.metricsClient != nil {
			api.metricsClient.IncrementCounterWithLabels("api.tools.list.error", 1, map[string]string{
				"tenant_id": tenantID,
			})
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tools"})
		return
	}

	// Record success metric
	if api.metricsClient != nil {
		api.metricsClient.RecordHistogram("api.tools.list.duration", float64(time.Since(start).Milliseconds()), map[string]string{
			"tenant_id": tenantID,
		})
	}

	// Check health status if requested
	if includeHealth {
		for range tools {
			// This would trigger health checks in parallel
			// For now, we'll use cached health status
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"tools": tools,
		"count": len(tools),
	})
}

// GetTool gets a specific tool by ID
// @Summary Get a dynamic tool
// @Description Retrieves details of a specific dynamic tool
// @Tags Dynamic Tools
// @Accept json
// @Produce json
// @Param toolId path string true "Tool ID"
// @Success 200 {object} models.DynamicTool
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tools/{toolId} [get]
func (api *DynamicToolsAPI) GetTool(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	tool, err := api.toolService.GetTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err.Error() == "tool not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
			return
		}
		api.logger.Error("Failed to get tool", map[string]interface{}{
			"tenant_id": tenantID,
			"tool_id":   toolID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get tool"})
		return
	}

	c.JSON(http.StatusOK, tool)
}

// CreateTool creates a new dynamic tool
// @Summary Create a dynamic tool
// @Description Creates a new dynamic tool with automatic discovery
// @Tags Dynamic Tools
// @Accept json
// @Produce json
// @Param tool body CreateToolRequest true "Tool configuration"
// @Success 201 {object} models.DynamicTool
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tools [post]
func (api *DynamicToolsAPI) CreateTool(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	_ = c.GetString("user_id")

	var req CreateToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if req.Name == "" || req.BaseURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and base_url are required"})
		return
	}

	// Convert to ToolConfig
	config := tools.ToolConfig{
		TenantID:          tenantID,
		Name:              req.Name,
		BaseURL:           req.BaseURL,
		DocumentationURL:  req.DocumentationURL,
		OpenAPIURL:        req.OpenAPIURL,
		Config:            req.Config,
		Credential:        req.Credential,
		Provider:          req.Provider,
		PassthroughConfig: (*tools.PassthroughConfig)(req.PassthroughConfig),
	}

	// Add discovery hints if provided
	if req.DiscoveryHints != nil {
		if config.Config == nil {
			config.Config = make(map[string]interface{})
		}
		config.Config["discovery_hints"] = req.DiscoveryHints
	}

	// Create tool with discovery
	tool, err := api.toolService.CreateTool(c.Request.Context(), tenantID, config)
	if err != nil {
		api.logger.Error("Failed to create tool", map[string]interface{}{
			"tenant_id": tenantID,
			"tool_name": req.Name,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Audit log
	if api.auditLogger != nil {
		api.auditLogger.LogToolRegistration(c.Request.Context(), tenantID, tool.ID, tool.ToolName, true, nil)
	}

	c.JSON(http.StatusCreated, tool)
}

// UpdateTool updates an existing tool
// @Summary Update a dynamic tool
// @Description Updates an existing dynamic tool configuration
// @Tags Dynamic Tools
// @Accept json
// @Produce json
// @Param toolId path string true "Tool ID"
// @Param tool body UpdateToolRequest true "Updated tool configuration"
// @Success 200 {object} models.DynamicTool
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tools/{toolId} [put]
func (api *DynamicToolsAPI) UpdateTool(c *gin.Context) {
	// Implementation similar to CreateTool
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

// DeleteTool deletes a tool
// @Summary Delete a dynamic tool
// @Description Deletes a dynamic tool configuration
// @Tags Dynamic Tools
// @Accept json
// @Produce json
// @Param toolId path string true "Tool ID"
// @Success 204 "No content"
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tools/{toolId} [delete]
func (api *DynamicToolsAPI) DeleteTool(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")
	_ = c.GetString("user_id")

	err := api.toolService.DeleteTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err.Error() == "tool not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
			return
		}
		api.logger.Error("Failed to delete tool", map[string]interface{}{
			"tenant_id": tenantID,
			"tool_id":   toolID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete tool"})
		return
	}

	// Audit log
	if api.auditLogger != nil {
		api.auditLogger.LogToolRegistration(c.Request.Context(), tenantID, toolID, "tool.delete", true, nil)
	}

	c.Status(http.StatusNoContent)
}

// DiscoverTool starts a tool discovery session
// @Summary Discover a tool
// @Description Starts a discovery session to find OpenAPI specifications
// @Tags Dynamic Tools
// @Accept json
// @Produce json
// @Param discovery body DiscoveryRequest true "Discovery configuration"
// @Success 200 {object} models.DiscoverySession
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tools/discover [post]
func (api *DynamicToolsAPI) DiscoverTool(c *gin.Context) {
	var req DiscoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
		return
	}

	// Create tool config from request
	config := tools.ToolConfig{
		TenantID:   tenantID,
		BaseURL:    req.BaseURL,
		Credential: req.Credential,
		Config:     req.DiscoveryHints,
	}

	// Start discovery session
	session, err := api.toolService.StartDiscovery(c.Request.Context(), config)
	if err != nil {
		api.logger.Error("Failed to start discovery", map[string]interface{}{
			"tenant_id": tenantID,
			"base_url":  req.BaseURL,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start discovery"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// GetDiscoverySession gets discovery session status
func (api *DynamicToolsAPI) GetDiscoverySession(c *gin.Context) {
	sessionID := c.Param("sessionId")

	session, err := api.toolService.GetDiscoverySession(c.Request.Context(), sessionID)
	if err != nil {
		if err.Error() == "discovery session not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get discovery session"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// ConfirmDiscovery confirms a discovery session
func (api *DynamicToolsAPI) ConfirmDiscovery(c *gin.Context) {
	sessionID := c.Param("sessionId")

	var req CreateToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create tool config
	config := tools.ToolConfig{
		Name:              req.Name,
		BaseURL:           req.BaseURL,
		DocumentationURL:  req.DocumentationURL,
		OpenAPIURL:        req.OpenAPIURL,
		Config:            req.Config,
		Credential:        req.Credential,
		Provider:          req.Provider,
		PassthroughConfig: (*tools.PassthroughConfig)(req.PassthroughConfig),
	}

	// Confirm discovery and create tool
	tool, err := api.toolService.ConfirmDiscovery(c.Request.Context(), sessionID, config)
	if err != nil {
		api.logger.Error("Failed to confirm discovery", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tool)
}

// CheckHealth checks tool health
// @Summary Check tool health
// @Description Checks the health status of a dynamic tool
// @Tags Dynamic Tools
// @Accept json
// @Produce json
// @Param toolId path string true "Tool ID"
// @Success 200 {object} models.ToolHealthStatus
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tools/{toolId}/health [get]
func (api *DynamicToolsAPI) CheckHealth(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	health, err := api.toolService.CheckToolHealth(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err.Error() == "tool not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
			return
		}
		api.logger.Error("Failed to check tool health", map[string]interface{}{
			"tenant_id": tenantID,
			"tool_id":   toolID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check health"})
		return
	}

	c.JSON(http.StatusOK, health)
}

// RefreshHealth refreshes tool health status
func (api *DynamicToolsAPI) RefreshHealth(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	health, err := api.toolService.RefreshToolHealth(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err.Error() == "tool not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
			return
		}
		api.logger.Error("Failed to refresh tool health", map[string]interface{}{
			"tenant_id": tenantID,
			"tool_id":   toolID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh health"})
		return
	}

	c.JSON(http.StatusOK, health)
}

// ListActions lists available actions for a tool
func (api *DynamicToolsAPI) ListActions(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
		return
	}

	actions, err := api.toolService.ListToolActions(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err.Error() == "tool not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		api.logger.Error("Failed to list tool actions", map[string]interface{}{
			"tenant_id": tenantID,
			"tool_id":   toolID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list actions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tool_id": toolID,
		"actions": actions,
		"count":   len(actions),
	})
}

// ExecuteAction executes a tool action
func (api *DynamicToolsAPI) ExecuteAction(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")
	action := c.Param("action")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
		return
	}

	var req models.ToolExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Use action from URL if not in body
	if req.Action == "" {
		req.Action = action
	}

	// Execute the action
	result, err := api.toolService.ExecuteToolAction(
		c.Request.Context(),
		tenantID,
		toolID,
		req.Action,
		req.Parameters,
	)
	if err != nil {
		if err.Error() == "tool not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		api.logger.Error("Failed to execute tool action", map[string]interface{}{
			"tenant_id": tenantID,
			"tool_id":   toolID,
			"action":    action,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// UpdateCredentials updates tool credentials
func (api *DynamicToolsAPI) UpdateCredentials(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
		return
	}

	var creds models.TokenCredential
	if err := c.ShouldBindJSON(&creds); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update credentials
	err := api.toolService.UpdateToolCredentials(c.Request.Context(), tenantID, toolID, &creds)
	if err != nil {
		if err.Error() == "tool not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		api.logger.Error("Failed to update tool credentials", map[string]interface{}{
			"tenant_id": tenantID,
			"tool_id":   toolID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update credentials"})
		return
	}

	// Audit log
	if api.auditLogger != nil {
		api.auditLogger.LogToolCredentialUpdate(c.Request.Context(), tenantID, toolID, c.GetString("user_id"), true, nil)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Credentials updated successfully",
		"tool_id": toolID,
	})
}

// DiscoverMultipleAPIs discovers all APIs from a portal URL
// @Summary Discover multiple APIs from a portal
// @Description Discovers all available APIs from an API portal like Harness, AWS, Azure, etc.
// @Tags Dynamic Tools
// @Accept json
// @Produce json
// @Param request body MultiAPIDiscoveryRequest true "Discovery request"
// @Success 200 {object} adapters.MultiAPIDiscoveryResult
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tools/discover-multiple [post]
func (api *DynamicToolsAPI) DiscoverMultipleAPIs(c *gin.Context) {
	start := time.Now()
	tenantID := c.GetString("tenant_id")

	var req MultiAPIDiscoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set timeout
	timeout := 5 * time.Minute
	if req.DiscoveryTimeout > 0 {
		timeout = time.Duration(req.DiscoveryTimeout) * time.Second
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()

	// Perform multi-API discovery
	result, err := api.toolService.DiscoverMultipleAPIs(ctx, req.PortalURL)
	if err != nil {
		api.logger.Error("Multi-API discovery failed", map[string]interface{}{
			"tenant_id":  tenantID,
			"portal_url": req.PortalURL,
			"error":      err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Discovery failed"})
		return
	}

	// Record metrics
	if api.metricsClient != nil {
		api.metricsClient.RecordHistogram("api.tools.discover_multiple.duration", float64(time.Since(start).Milliseconds()), map[string]string{
			"tenant_id": tenantID,
			"portal":    req.PortalURL,
		})
		api.metricsClient.IncrementCounterWithLabels("api.tools.discover_multiple.apis_found", float64(len(result.DiscoveredAPIs)), map[string]string{
			"tenant_id": tenantID,
			"portal":    req.PortalURL,
		})
	}

	// Audit log can be added here when AuditLogger supports LogAction method

	c.JSON(http.StatusOK, result)
}

// CreateToolsFromMultipleAPIs creates tools from multi-API discovery results
// @Summary Create tools from multiple discovered APIs
// @Description Creates multiple tools from APIs discovered from a portal
// @Tags Dynamic Tools
// @Accept json
// @Produce json
// @Param request body CreateToolsFromMultipleAPIsRequest true "Create tools request"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tools/discover-multiple/create [post]
func (api *DynamicToolsAPI) CreateToolsFromMultipleAPIs(c *gin.Context) {
	start := time.Now()
	tenantID := c.GetString("tenant_id")
	_ = c.GetString("user_id") // For future audit logging

	var req CreateToolsFromMultipleAPIsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// First, perform discovery
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	discoveryResult, err := api.toolService.DiscoverMultipleAPIs(ctx, req.PortalURL)
	if err != nil {
		api.logger.Error("Multi-API discovery failed", map[string]interface{}{
			"tenant_id":  tenantID,
			"portal_url": req.PortalURL,
			"error":      err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Discovery failed"})
		return
	}

	// Check if any APIs were discovered
	if len(discoveryResult.DiscoveredAPIs) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error":            "No APIs found",
			"discovery_result": discoveryResult,
		})
		return
	}

	// Create base tool config
	baseConfig := tools.ToolConfig{
		Name:              req.NamePrefix,
		BaseURL:           req.PortalURL,
		Credential:        req.Credential,
		Provider:          req.Provider,
		PassthroughConfig: (*tools.PassthroughConfig)(req.PassthroughConfig),
		Config:            req.Config,
	}

	// Create tools from discovered APIs
	createdTools, err := api.toolService.CreateToolsFromMultipleAPIs(ctx, tenantID, discoveryResult, baseConfig)
	if err != nil {
		api.logger.Error("Failed to create tools from discovered APIs", map[string]interface{}{
			"tenant_id":  tenantID,
			"portal_url": req.PortalURL,
			"error":      err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":            "Failed to create tools",
			"discovery_result": discoveryResult,
		})
		return
	}

	// Record metrics
	if api.metricsClient != nil {
		api.metricsClient.RecordHistogram("api.tools.create_multiple.duration", float64(time.Since(start).Milliseconds()), map[string]string{
			"tenant_id": tenantID,
			"portal":    req.PortalURL,
		})
		api.metricsClient.IncrementCounterWithLabels("api.tools.create_multiple.tools_created", float64(len(createdTools)), map[string]string{
			"tenant_id": tenantID,
			"portal":    req.PortalURL,
		})
	}

	// Audit log can be added here when AuditLogger supports LogAction method

	c.JSON(http.StatusCreated, gin.H{
		"tools_created":    createdTools,
		"count":            len(createdTools),
		"discovery_result": discoveryResult,
	})
}

// Request/Response types

type CreateToolRequest struct {
	Name              string                    `json:"name" binding:"required"`
	BaseURL           string                    `json:"base_url" binding:"required"`
	DocumentationURL  string                    `json:"documentation_url,omitempty"`
	OpenAPIURL        string                    `json:"openapi_url,omitempty"`
	Config            map[string]interface{}    `json:"config,omitempty"`
	Credential        *models.TokenCredential   `json:"credential,omitempty"`
	Provider          string                    `json:"provider,omitempty"`
	PassthroughConfig *models.PassthroughConfig `json:"passthrough_config,omitempty"`
	DiscoveryHints    map[string]interface{}    `json:"discovery_hints,omitempty"`
}

type UpdateToolRequest struct {
	DisplayName       string                    `json:"display_name,omitempty"`
	Config            map[string]interface{}    `json:"config,omitempty"`
	PassthroughConfig *models.PassthroughConfig `json:"passthrough_config,omitempty"`
}

type DiscoveryRequest struct {
	BaseURL        string                  `json:"base_url" binding:"required"`
	DiscoveryHints map[string]interface{}  `json:"discovery_hints,omitempty"`
	Credential     *models.TokenCredential `json:"credential,omitempty"`
}

type ListToolsResponse struct {
	Tools []*models.DynamicTool `json:"tools"`
	Count int                   `json:"count"`
}

type MultiAPIDiscoveryRequest struct {
	PortalURL        string                  `json:"portal_url" binding:"required"`
	DiscoveryTimeout int                     `json:"discovery_timeout,omitempty"` // seconds
	Credential       *models.TokenCredential `json:"credential,omitempty"`
}

type CreateToolsFromMultipleAPIsRequest struct {
	PortalURL         string                    `json:"portal_url" binding:"required"`
	NamePrefix        string                    `json:"name_prefix" binding:"required"`
	AutoCreate        bool                      `json:"auto_create"`
	Credential        *models.TokenCredential   `json:"credential,omitempty"`
	Provider          string                    `json:"provider,omitempty"`
	PassthroughConfig *models.PassthroughConfig `json:"passthrough_config,omitempty"`
	Config            map[string]interface{}    `json:"config,omitempty"`
}

// GetWebhookConfig returns the webhook configuration for a tool
// @Summary Get webhook configuration
// @Description Returns webhook configuration including URL and authentication details
// @Tags Dynamic Tools
// @Accept json
// @Produce json
// @Param toolId path string true "Tool ID"
// @Success 200 {object} WebhookConfigResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tools/{toolId}/webhook [get]
func (api *DynamicToolsAPI) GetWebhookConfig(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
		return
	}

	// Get tool with webhook config
	tool, err := api.toolService.GetTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
		return
	}

	// Get base URL from request
	scheme := "https"
	if c.Request.TLS == nil {
		scheme = "http"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

	// Build webhook response
	response := gin.H{
		"tool_id":     toolID,
		"tool_name":   tool.ToolName,
		"webhook_url": fmt.Sprintf("%s/api/webhooks/tools/%s", baseURL, toolID),
		"enabled":     false,
	}

	if tool.WebhookConfig != nil {
		response["enabled"] = tool.WebhookConfig.Enabled
		response["auth_type"] = tool.WebhookConfig.AuthType

		if tool.WebhookConfig.SignatureHeader != "" {
			response["signature_header"] = tool.WebhookConfig.SignatureHeader
		}

		if tool.WebhookConfig.SignatureAlgorithm != "" {
			response["signature_algorithm"] = tool.WebhookConfig.SignatureAlgorithm
		}

		if len(tool.WebhookConfig.Events) > 0 {
			events := make([]string, 0, len(tool.WebhookConfig.Events))
			for _, e := range tool.WebhookConfig.Events {
				events = append(events, e.EventType)
			}
			response["supported_events"] = events
		}

		// Add setup instructions
		switch tool.WebhookConfig.AuthType {
		case "hmac":
			response["setup_instructions"] = []string{
				fmt.Sprintf("1. Configure your %s to send webhooks to: %s/api/webhooks/tools/%s", tool.ToolName, baseURL, toolID),
				"2. Set up HMAC signing with the provided secret",
				fmt.Sprintf("3. Include the signature in the '%s' header", tool.WebhookConfig.SignatureHeader),
			}
		case "bearer":
			response["setup_instructions"] = []string{
				fmt.Sprintf("1. Configure your %s to send webhooks to: %s/api/webhooks/tools/%s", tool.ToolName, baseURL, toolID),
				"2. Include the authentication token in the Authorization header as 'Bearer <token>'",
			}
		case "basic":
			response["setup_instructions"] = []string{
				fmt.Sprintf("1. Configure your %s to send webhooks to: %s/api/webhooks/tools/%s", tool.ToolName, baseURL, toolID),
				"2. Use HTTP Basic Authentication with the provided credentials",
			}
		default:
			response["setup_instructions"] = []string{
				fmt.Sprintf("1. Configure your %s to send webhooks to: %s/api/webhooks/tools/%s", tool.ToolName, baseURL, toolID),
			}
		}
	}

	c.JSON(http.StatusOK, response)
}

// WebhookConfigResponse represents the webhook configuration response
type WebhookConfigResponse struct {
	ToolID             string   `json:"tool_id"`
	ToolName           string   `json:"tool_name"`
	WebhookURL         string   `json:"webhook_url"`
	Enabled            bool     `json:"enabled"`
	AuthType           string   `json:"auth_type,omitempty"`
	SignatureHeader    string   `json:"signature_header,omitempty"`
	SignatureAlgorithm string   `json:"signature_algorithm,omitempty"`
	SupportedEvents    []string `json:"supported_events,omitempty"`
	SetupInstructions  []string `json:"setup_instructions,omitempty"`
}
