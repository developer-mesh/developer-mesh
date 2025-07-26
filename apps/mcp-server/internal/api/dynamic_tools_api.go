package api

import (
	"context"
	"net/http"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/developer-mesh/developer-mesh/pkg/tools/adapters"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DynamicToolsAPI handles dynamic tool management endpoints
type DynamicToolsAPI struct {
	toolService    *DynamicToolService
	logger         observability.Logger
	encryptionSvc  *security.EncryptionService
	healthCheckMgr *tools.HealthCheckManager
	openAPIAdapter *adapters.OpenAPIAdapter
	auditLogger    *auth.AuditLogger
}

// NewDynamicToolsAPI creates a new dynamic tools API handler
func NewDynamicToolsAPI(
	toolService *DynamicToolService,
	logger observability.Logger,
	encryptionSvc *security.EncryptionService,
	healthCheckMgr *tools.HealthCheckManager,
	auditLogger *auth.AuditLogger,
) *DynamicToolsAPI {
	return &DynamicToolsAPI{
		toolService:    toolService,
		logger:         logger,
		encryptionSvc:  encryptionSvc,
		healthCheckMgr: healthCheckMgr,
		openAPIAdapter: adapters.NewOpenAPIAdapter(logger),
		auditLogger:    auditLogger,
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

		// Health checks
		tools.GET("/:toolId/health", api.CheckHealth)
		tools.POST("/:toolId/health/refresh", api.RefreshHealth)

		// Execution
		tools.POST("/:toolId/execute/:action", api.ExecuteAction)
		tools.GET("/:toolId/actions", api.ListActions)

		// Credentials
		tools.PUT("/:toolId/credentials", api.UpdateCredentials)
	}
}

// ListTools lists all configured tools for the tenant
func (api *DynamicToolsAPI) ListTools(c *gin.Context) {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list tools"})
		return
	}

	// Optionally include health status
	if includeHealth {
		for i := range tools {
			if status, ok := api.healthCheckMgr.GetCachedStatus(c.Request.Context(), tools[i].Config); ok {
				tools[i].HealthStatus = status
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"tools": tools,
		"count": len(tools),
	})
}

// CreateTool creates a new tool configuration
func (api *DynamicToolsAPI) CreateTool(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
		return
	}

	var req CreateToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create tool config
	config := tools.ToolConfig{
		ID:               uuid.New().String(),
		TenantID:         tenantID,
		Name:             req.Name,
		BaseURL:          req.BaseURL,
		DocumentationURL: req.DocumentationURL,
		OpenAPIURL:       req.OpenAPIURL,
		Config:           req.Config,
		RetryPolicy:      req.RetryPolicy,
		HealthConfig:     req.HealthConfig,
	}

	// Handle credentials
	if req.Credentials != nil {
		// Encrypt credentials
		encrypted, err := api.encryptionSvc.EncryptCredential(
			req.Credentials.Token,
			tenantID,
		)
		if err != nil {
			api.logger.Error("Failed to encrypt credentials", map[string]interface{}{
				"error": err.Error(),
			})
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt credentials"})
			return
		}

		config.Credential = &models.TokenCredential{
			Type:         req.AuthType,
			Token:        string(encrypted),
			HeaderName:   req.Credentials.HeaderName,
			HeaderPrefix: req.Credentials.HeaderPrefix,
			QueryParam:   req.Credentials.QueryParam,
			Username:     req.Credentials.Username,
			Password:     req.Credentials.Password,
		}
	}

	// If OpenAPI URL provided, try to discover and generate tools
	if config.OpenAPIURL != "" {
		discovery, err := api.openAPIAdapter.DiscoverAPIs(c.Request.Context(), config)
		if err == nil && discovery.Status == tools.DiscoveryStatusSuccess {
			// Generate tools from spec
			generatedTools, err := api.openAPIAdapter.GenerateTools(config, discovery.OpenAPISpec)
			if err == nil {
				config.Config["generated_tools_count"] = len(generatedTools)
				config.Config["capabilities"] = discovery.Capabilities
			}
		}
	}

	// Save tool configuration
	tool, err := api.toolService.CreateTool(c.Request.Context(), config)
	if err != nil {
		api.logger.Error("Failed to create tool", map[string]interface{}{
			"error": err.Error(),
		})
		api.auditLogger.LogToolRegistration(c.Request.Context(), tenantID, config.ID, config.Name, false, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create tool"})
		return
	}

	// Log successful registration
	api.auditLogger.LogToolRegistration(c.Request.Context(), tenantID, tool.ID, tool.Name, true, nil)

	// Test connection
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if _, err := api.healthCheckMgr.CheckHealth(ctx, config, true); err != nil {
			api.logger.Warn("Initial health check failed", map[string]interface{}{
				"tool_id": tool.ID,
				"error":   err.Error(),
			})
		}
	}()

	c.JSON(http.StatusCreated, tool)
}

// GetTool gets a specific tool configuration
func (api *DynamicToolsAPI) GetTool(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	tool, err := api.toolService.GetTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err == ErrToolNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tool"})
		return
	}

	// Include health status
	if status, ok := api.healthCheckMgr.GetCachedStatus(c.Request.Context(), tool.Config); ok {
		tool.HealthStatus = status
	}

	c.JSON(http.StatusOK, tool)
}

// UpdateTool updates a tool configuration
func (api *DynamicToolsAPI) UpdateTool(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	var req UpdateToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing tool
	existing, err := api.toolService.GetTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err == ErrToolNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tool"})
		return
	}

	// Update fields
	if req.Name != "" {
		existing.Config.Name = req.Name
	}
	if req.BaseURL != "" {
		existing.Config.BaseURL = req.BaseURL
	}
	if req.DocumentationURL != "" {
		existing.Config.DocumentationURL = req.DocumentationURL
	}
	if req.OpenAPIURL != "" {
		existing.Config.OpenAPIURL = req.OpenAPIURL
	}
	if req.Config != nil {
		for k, v := range req.Config {
			existing.Config.Config[k] = v
		}
	}
	if req.RetryPolicy != nil {
		existing.Config.RetryPolicy = req.RetryPolicy
	}
	if req.HealthConfig != nil {
		existing.Config.HealthConfig = req.HealthConfig
	}

	// Update tool
	updated, err := api.toolService.UpdateTool(c.Request.Context(), existing.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update tool"})
		return
	}

	// Invalidate health cache
	api.healthCheckMgr.InvalidateCache(c.Request.Context(), updated.Config)

	c.JSON(http.StatusOK, updated)
}

// DeleteTool deletes a tool configuration
func (api *DynamicToolsAPI) DeleteTool(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	err := api.toolService.DeleteTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err == ErrToolNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete tool"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// DiscoverTool initiates tool discovery
func (api *DynamicToolsAPI) DiscoverTool(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	var req DiscoverToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create temporary config for discovery
	config := tools.ToolConfig{
		TenantID:   tenantID,
		BaseURL:    req.BaseURL,
		OpenAPIURL: req.OpenAPIURL,
		Config:     req.Hints,
	}

	// Handle authentication if provided
	if req.AuthType != "" && req.Credentials != nil {
		config.Credential = &models.TokenCredential{
			Type:         req.AuthType,
			Token:        req.Credentials.Token,
			HeaderName:   req.Credentials.HeaderName,
			HeaderPrefix: req.Credentials.HeaderPrefix,
			QueryParam:   req.Credentials.QueryParam,
			Username:     req.Credentials.Username,
			Password:     req.Credentials.Password,
		}
	}

	// Start discovery
	session, err := api.toolService.StartDiscovery(c.Request.Context(), config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start discovery"})
		return
	}

	// Run discovery asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		result, err := api.openAPIAdapter.DiscoverAPIs(ctx, config)
		if err != nil {
			api.toolService.UpdateDiscoverySession(ctx, session.ID, tools.DiscoveryStatusFailed, nil, err)
			return
		}

		api.toolService.UpdateDiscoverySession(ctx, session.ID, result.Status, result, nil)
	}()

	c.JSON(http.StatusAccepted, session)
}

// GetDiscoverySession gets the status of a discovery session
func (api *DynamicToolsAPI) GetDiscoverySession(c *gin.Context) {
	sessionID := c.Param("sessionId")

	session, err := api.toolService.GetDiscoverySession(c.Request.Context(), sessionID)
	if err != nil {
		if err == ErrSessionNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get session"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// ConfirmDiscovery confirms and saves a discovered tool
func (api *DynamicToolsAPI) ConfirmDiscovery(c *gin.Context) {
	sessionID := c.Param("sessionId")

	var req ConfirmDiscoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get session
	session, err := api.toolService.GetDiscoverySession(c.Request.Context(), sessionID)
	if err != nil {
		if err == ErrSessionNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get session"})
		return
	}

	// Check session status
	if session.Status != tools.DiscoveryStatusSuccess && session.Status != tools.DiscoveryStatusPartial {
		c.JSON(http.StatusBadRequest, gin.H{"error": "discovery not successful"})
		return
	}

	// Create tool from discovery
	tool, err := api.toolService.CreateToolFromDiscovery(c.Request.Context(), session, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create tool"})
		return
	}

	c.JSON(http.StatusCreated, tool)
}

// CheckHealth checks the health of a tool
func (api *DynamicToolsAPI) CheckHealth(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	tool, err := api.toolService.GetTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err == ErrToolNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tool"})
		return
	}

	// Check if cached result is available
	force := c.Query("force") == "true"

	status, err := api.healthCheckMgr.CheckHealth(c.Request.Context(), tool.Config, force)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "health check failed", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// RefreshHealth forces a health check refresh
func (api *DynamicToolsAPI) RefreshHealth(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	tool, err := api.toolService.GetTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err == ErrToolNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tool"})
		return
	}

	// Force health check
	status, err := api.healthCheckMgr.CheckHealth(c.Request.Context(), tool.Config, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "health check failed", "details": err.Error()})
		return
	}

	// Update tool status in database
	api.toolService.UpdateHealthStatus(c.Request.Context(), tenantID, toolID, status)

	c.JSON(http.StatusOK, status)
}

// ExecuteAction executes a tool action
func (api *DynamicToolsAPI) ExecuteAction(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")
	action := c.Param("action")

	var params map[string]interface{}
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get tool
	tool, err := api.toolService.GetTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err == ErrToolNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tool"})
		return
	}

	// Execute action
	result, err := api.toolService.ExecuteAction(c.Request.Context(), tool, action, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "execution failed", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ListActions lists available actions for a tool
func (api *DynamicToolsAPI) ListActions(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	tool, err := api.toolService.GetTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err == ErrToolNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tool"})
		return
	}

	// Get available actions
	actions, err := api.toolService.GetAvailableActions(c.Request.Context(), tool)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get actions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tool_id": toolID,
		"actions": actions,
		"count":   len(actions),
	})
}

// UpdateCredentials updates tool credentials
func (api *DynamicToolsAPI) UpdateCredentials(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	var req UpdateCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing tool
	tool, err := api.toolService.GetTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err == ErrToolNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tool"})
		return
	}

	// Encrypt new credentials
	encrypted, err := api.encryptionSvc.EncryptCredential(req.Credentials.Token, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt credentials"})
		return
	}

	// Update credentials
	tool.Config.Credential = &models.TokenCredential{
		Type:         req.AuthType,
		Token:        string(encrypted),
		HeaderName:   req.Credentials.HeaderName,
		HeaderPrefix: req.Credentials.HeaderPrefix,
		QueryParam:   req.Credentials.QueryParam,
		Username:     req.Credentials.Username,
		Password:     req.Credentials.Password,
	}

	// Save updated tool
	if _, err := api.toolService.UpdateTool(c.Request.Context(), tool.Config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update tool"})
		return
	}

	// Test new credentials
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		api.healthCheckMgr.CheckHealth(ctx, tool.Config, true)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "credentials updated successfully"})
}
