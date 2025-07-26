package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
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
	metricsClient  observability.MetricsClient
	encryptionSvc  *security.EncryptionService
	healthCheckMgr *tools.HealthCheckManager
	openAPIAdapter *adapters.OpenAPIAdapter
	auditLogger    *auth.AuditLogger
}

// NewDynamicToolsAPI creates a new dynamic tools API handler
func NewDynamicToolsAPI(
	toolService *DynamicToolService,
	logger observability.Logger,
	metricsClient observability.MetricsClient,
	encryptionSvc *security.EncryptionService,
	healthCheckMgr *tools.HealthCheckManager,
	auditLogger *auth.AuditLogger,
) *DynamicToolsAPI {
	return &DynamicToolsAPI{
		toolService:    toolService,
		logger:         logger,
		metricsClient:  metricsClient,
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
			api.metricsClient.RecordCounter("dynamic_tools_api_requests", 1, map[string]string{
				"operation": "list_tools",
				"status":    "error",
				"tenant_id": tenantID,
			})
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list tools"})
		return
	}

	// Optionally include health status
	if includeHealth {
		for i := range tools {
			if status, ok := api.healthCheckMgr.GetCachedStatus(c.Request.Context(), tools[i].InternalConfig); ok {
				tools[i].HealthStatus = status
			}
		}
	}

	// Record success metric
	if api.metricsClient != nil {
		duration := time.Since(start).Seconds()
		api.metricsClient.RecordCounter("dynamic_tools_api_requests", 1, map[string]string{
			"operation": "list_tools",
			"status":    "success",
			"tenant_id": tenantID,
		})
		api.metricsClient.RecordHistogram("dynamic_tools_api_duration_seconds", duration, map[string]string{
			"operation": "list_tools",
		})
		api.metricsClient.RecordGauge("dynamic_tools_count", float64(len(tools)), map[string]string{
			"tenant_id": tenantID,
			"status":    status,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"tools": tools,
		"count": len(tools),
	})
}

// CreateTool creates a new tool configuration
func (api *DynamicToolsAPI) CreateTool(c *gin.Context) {
	start := time.Now()
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

	// Auto-detect provider if not specified
	provider := req.Provider
	if provider == "" {
		if strings.Contains(req.BaseURL, "github.com") {
			provider = "github"
		} else if strings.Contains(req.BaseURL, "gitlab.com") {
			provider = "gitlab"
		} else if strings.Contains(req.BaseURL, "bitbucket.org") {
			provider = "bitbucket"
		} else {
			provider = "custom"
		}
	}

	// Set default passthrough config if not provided
	passthroughConfig := req.PassthroughConfig
	if passthroughConfig == nil {
		passthroughConfig = &PassthroughConfig{
			Mode:              "optional",
			FallbackToService: true,
		}
	}

	// Create tool config
	config := tools.ToolConfig{
		ID:                uuid.New().String(),
		TenantID:          tenantID,
		Name:              req.Name,
		BaseURL:           req.BaseURL,
		DocumentationURL:  req.DocumentationURL,
		OpenAPIURL:        req.OpenAPIURL,
		Config:            req.Config,
		RetryPolicy:       req.RetryPolicy,
		HealthConfig:      req.HealthConfig,
		Provider:          provider,
		PassthroughConfig: (*tools.PassthroughConfig)(passthroughConfig),
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
				if config.Config == nil {
					config.Config = make(map[string]interface{})
				}
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
		// Record failure metric
		if api.metricsClient != nil {
			api.metricsClient.RecordCounter("dynamic_tools_api_requests", 1, map[string]string{
				"operation": "create_tool",
				"status":    "error",
				"tenant_id": tenantID,
			})
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create tool"})
		return
	}

	// Log successful registration
	api.auditLogger.LogToolRegistration(c.Request.Context(), tenantID, tool.ID, tool.Name, true, nil)

	// Record success metric
	if api.metricsClient != nil {
		duration := time.Since(start).Seconds()
		api.metricsClient.RecordCounter("dynamic_tools_api_requests", 1, map[string]string{
			"operation": "create_tool",
			"status":    "success",
			"tenant_id": tenantID,
			"tool_name": tool.Name,
		})
		api.metricsClient.RecordHistogram("dynamic_tools_api_duration_seconds", duration, map[string]string{
			"operation": "create_tool",
		})
	}

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
		if err == ErrDynamicToolNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tool"})
		return
	}

	// Include health status
	if status, ok := api.healthCheckMgr.GetCachedStatus(c.Request.Context(), tool.InternalConfig); ok {
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

	// Get existing tool (bypass cache to ensure fresh data)
	existing, err := api.toolService.GetTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err == ErrDynamicToolNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tool"})
		return
	}

	// Ensure config map exists
	if existing.InternalConfig.Config == nil {
		existing.InternalConfig.Config = make(map[string]interface{})
	}

	// Update fields
	if req.Name != "" {
		existing.InternalConfig.Name = req.Name
	}
	if req.BaseURL != "" {
		existing.InternalConfig.BaseURL = req.BaseURL
	}
	if req.DocumentationURL != "" {
		existing.InternalConfig.DocumentationURL = req.DocumentationURL
	}
	if req.OpenAPIURL != "" {
		existing.InternalConfig.OpenAPIURL = req.OpenAPIURL
	}
	if req.Config != nil {
		for k, v := range req.Config {
			existing.InternalConfig.Config[k] = v
		}
	}
	if req.RetryPolicy != nil {
		existing.InternalConfig.RetryPolicy = req.RetryPolicy
	}
	if req.HealthConfig != nil {
		existing.InternalConfig.HealthConfig = req.HealthConfig
	}
	if req.PassthroughConfig != nil {
		existing.InternalConfig.PassthroughConfig = (*tools.PassthroughConfig)(req.PassthroughConfig)
	}

	// Update tool
	updated, err := api.toolService.UpdateTool(c.Request.Context(), existing.InternalConfig)
	if err != nil {
		api.logger.Error("Failed to update tool", map[string]interface{}{
			"tool_id": toolID,
			"error":   err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update tool"})
		return
	}

	// Invalidate health cache
	if err := api.healthCheckMgr.InvalidateCache(c.Request.Context(), updated.InternalConfig); err != nil {
		// Log error but don't fail the request
		api.logger.Debugf("failed to invalidate health cache: %v", err)
	}

	c.JSON(http.StatusOK, updated)
}

// DeleteTool deletes a tool configuration
func (api *DynamicToolsAPI) DeleteTool(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	err := api.toolService.DeleteTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err == ErrDynamicToolNotFound {
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
			if updateErr := api.toolService.UpdateDiscoverySession(ctx, session.ID, tools.DiscoveryStatusFailed, nil, err); updateErr != nil {
				api.logger.Errorf("failed to update discovery session: %v", updateErr)
			}
			return
		}

		if err := api.toolService.UpdateDiscoverySession(ctx, session.ID, result.Status, result, nil); err != nil {
			api.logger.Errorf("failed to update discovery session: %v", err)
		}
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
		if err == ErrDynamicToolNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tool"})
		return
	}

	// Check if cached result is available
	force := c.Query("force") == "true"

	status, err := api.healthCheckMgr.CheckHealth(c.Request.Context(), tool.InternalConfig, force)
	if err != nil {
		// Record health check failure
		if api.metricsClient != nil {
			api.metricsClient.RecordCounter("dynamic_tools_health_checks", 1, map[string]string{
				"tool_id":   toolID,
				"tool_name": tool.Name,
				"status":    "error",
				"tenant_id": tenantID,
			})
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "health check failed", "details": err.Error()})
		return
	}

	// Record health check success and tool health status
	if api.metricsClient != nil {
		healthStatusStr := "healthy"
		if !status.IsHealthy {
			healthStatusStr = "unhealthy"
		}
		api.metricsClient.RecordCounter("dynamic_tools_health_checks", 1, map[string]string{
			"tool_id":       toolID,
			"tool_name":     tool.Name,
			"status":        "success",
			"health_status": healthStatusStr,
			"tenant_id":     tenantID,
		})
		healthValue := 1.0
		if !status.IsHealthy {
			healthValue = 0.0
		}
		api.metricsClient.RecordGauge("dynamic_tools_health_status", healthValue, map[string]string{
			"tool_id":   toolID,
			"tool_name": tool.Name,
			"tenant_id": tenantID,
		})
	}

	c.JSON(http.StatusOK, status)
}

// RefreshHealth forces a health check refresh
func (api *DynamicToolsAPI) RefreshHealth(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	tool, err := api.toolService.GetTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err == ErrDynamicToolNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tool"})
		return
	}

	// Force health check
	status, err := api.healthCheckMgr.CheckHealth(c.Request.Context(), tool.InternalConfig, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "health check failed", "details": err.Error()})
		return
	}

	// Update tool status in database
	if err := api.toolService.UpdateHealthStatus(c.Request.Context(), tenantID, toolID, status); err != nil {
		// Log error but don't fail the response
		api.logger.Errorf("failed to update health status: %v", err)
	}

	c.JSON(http.StatusOK, status)
}

// ExecuteAction executes a tool action
func (api *DynamicToolsAPI) ExecuteAction(c *gin.Context) {
	start := time.Now()
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
		if err == ErrDynamicToolNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tool"})
		return
	}

	// Check for passthrough token
	ctx := c.Request.Context()
	authMethod := "service_account"

	if passthroughToken, ok := auth.GetPassthroughToken(ctx); ok && tool.Provider != "" {
		// Validate provider matches
		if passthroughToken.Provider == tool.Provider {
			// Add user credentials to context
			var userCreds *models.ToolCredentials
			switch tool.Provider {
			case "github":
				userCreds = &models.ToolCredentials{
					GitHub: &models.TokenCredential{
						Type:  "bearer",
						Token: passthroughToken.Token,
					},
				}
			case "gitlab":
				userCreds = &models.ToolCredentials{
					GitLab: &models.TokenCredential{
						Type:  "bearer",
						Token: passthroughToken.Token,
					},
				}
			default:
				// For custom providers, use generic credential
				userCreds = &models.ToolCredentials{
					Custom: map[string]*models.TokenCredential{
						tool.Provider: {
							Type:  "bearer",
							Token: passthroughToken.Token,
						},
					},
				}
			}

			ctx = auth.WithToolCredentials(ctx, userCreds)
			authMethod = "passthrough"

			api.logger.Info("Using passthrough token for tool execution", map[string]interface{}{
				"tool_id":  toolID,
				"provider": tool.Provider,
			})
		} else if tool.PassthroughConfig != nil && tool.PassthroughConfig.Mode == "required" {
			// Passthrough required but provider mismatch
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "provider mismatch",
				"details": fmt.Sprintf("tool requires %s credentials but %s token provided", tool.Provider, passthroughToken.Provider),
			})
			return
		}
	} else if tool.PassthroughConfig != nil && tool.PassthroughConfig.Mode == "required" {
		// Passthrough required but no token provided
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "passthrough token required",
			"details": "this tool requires user authentication via X-User-Token header",
		})
		return
	}

	// Execute action with updated context
	result, err := api.toolService.ExecuteAction(ctx, tool, action, params)
	executionDuration := time.Since(start)

	// Audit log the execution with metadata
	auditMetadata := map[string]interface{}{
		"auth_method": authMethod,
		"tool_name":   tool.Name,
		"provider":    tool.Provider,
	}
	api.auditLogger.LogToolExecution(ctx, tenantID, toolID, action, params, result, executionDuration, err, auditMetadata)

	if err != nil {
		// Record failure metric
		if api.metricsClient != nil {
			api.metricsClient.RecordCounter("dynamic_tools_executions", 1, map[string]string{
				"tool_id":     toolID,
				"tool_name":   tool.Name,
				"action":      action,
				"status":      "error",
				"tenant_id":   tenantID,
				"auth_method": authMethod,
			})
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "execution failed", "details": err.Error()})
		return
	}

	// Record success metric
	if api.metricsClient != nil {
		api.metricsClient.RecordCounter("dynamic_tools_executions", 1, map[string]string{
			"tool_id":     toolID,
			"tool_name":   tool.Name,
			"action":      action,
			"status":      "success",
			"tenant_id":   tenantID,
			"auth_method": authMethod,
		})
		api.metricsClient.RecordHistogram("dynamic_tools_execution_duration_seconds", executionDuration.Seconds(), map[string]string{
			"tool_name": tool.Name,
			"action":    action,
		})
	}

	c.JSON(http.StatusOK, result)
}

// ListActions lists available actions for a tool
func (api *DynamicToolsAPI) ListActions(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	tool, err := api.toolService.GetTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err == ErrDynamicToolNotFound {
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
		if err == ErrDynamicToolNotFound {
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
	tool.InternalConfig.Credential = &models.TokenCredential{
		Type:         req.AuthType,
		Token:        string(encrypted),
		HeaderName:   req.Credentials.HeaderName,
		HeaderPrefix: req.Credentials.HeaderPrefix,
		QueryParam:   req.Credentials.QueryParam,
		Username:     req.Credentials.Username,
		Password:     req.Credentials.Password,
	}

	// Save updated tool
	if _, err := api.toolService.UpdateTool(c.Request.Context(), tool.InternalConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update tool"})
		return
	}

	// Test new credentials
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if _, err := api.healthCheckMgr.CheckHealth(ctx, tool.InternalConfig, true); err != nil {
			api.logger.Errorf("failed to run health check: %v", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "credentials updated successfully"})
}
