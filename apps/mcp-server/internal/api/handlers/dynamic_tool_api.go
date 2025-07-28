package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/core/tool"
	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/services"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DynamicToolAPI handles dynamic tool operations
type DynamicToolAPI struct {
	toolRegistry     *services.ToolRegistry
	discoveryService *services.DiscoveryService
	executionService *services.ExecutionService
	logger           observability.Logger
}

// NewDynamicToolAPI creates a new dynamic tool API handler
func NewDynamicToolAPI(
	toolRegistry *services.ToolRegistry,
	discoveryService *services.DiscoveryService,
	executionService *services.ExecutionService,
	logger observability.Logger,
) *DynamicToolAPI {
	return &DynamicToolAPI{
		toolRegistry:     toolRegistry,
		discoveryService: discoveryService,
		executionService: executionService,
		logger:           logger,
	}
}

// RegisterRoutes registers all dynamic tool routes
func (api *DynamicToolAPI) RegisterRoutes(router *gin.RouterGroup) {
	// All tools are now dynamic - no hardcoded paths or legacy compatibility needed
	tools := router.Group("/tools")

	// Collection endpoints
	tools.GET("", api.listTools)
	tools.POST("", api.registerTool)

	// Discovery endpoints
	tools.POST("/discover", api.startDiscovery)
	tools.POST("/discover/:session_id/confirm", api.confirmDiscovery)

	// Single tool endpoints
	tools.GET("/:tool", api.getToolDetails)
	tools.PUT("/:tool", api.updateTool)
	tools.DELETE("/:tool", api.deleteTool)
	tools.POST("/:tool/test", api.testConnection)

	// Action endpoints
	tools.GET("/:tool/actions", api.listToolActions)
	tools.GET("/:tool/actions/:action", api.getActionDetails)
	tools.POST("/:tool/actions/:action", api.executeAction)

	// Query endpoint
	tools.POST("/:tool/queries", api.queryToolData)
}

// listTools returns all tools for the authenticated tenant
func (api *DynamicToolAPI) listTools(c *gin.Context) {
	// Get tenant from context
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant ID"})
		return
	}

	tools, err := api.toolRegistry.ListToolsForTenant(c.Request.Context(), tenantID)
	if err != nil {
		api.logger.Error("Failed to list tools", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve tools",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// Convert to API response format
	response := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		response = append(response, map[string]interface{}{
			"id":                tool.ID,
			"name":              tool.Name,
			"type":              tool.Type,
			"display_name":      tool.DisplayName,
			"status":            tool.Status,
			"health_status":     tool.HealthStatus,
			"last_health_check": tool.LastHealthCheck,
			"_links": map[string]string{
				"self":    "/api/v1/tools/" + tool.Name,
				"test":    "/api/v1/tools/" + tool.Name + "/test",
				"actions": "/api/v1/tools/" + tool.Name + "/actions",
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"tools": response,
		"total": len(response),
	})
}

// registerTool creates a new tool for the tenant
func (api *DynamicToolAPI) registerTool(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant ID"})
		return
	}

	var request struct {
		Name             string                  `json:"name" binding:"required"`
		DisplayName      string                  `json:"display_name"`
		BaseURL          string                  `json:"base_url" binding:"required"`
		DocumentationURL string                  `json:"documentation_url,omitempty"`
		OpenAPIURL       string                  `json:"openapi_url,omitempty"`
		AuthConfig       map[string]interface{}  `json:"auth_config" binding:"required"`
		RetryPolicy      *tool.ToolRetryPolicy   `json:"retry_policy,omitempty"`
		HealthCheck      *tool.HealthCheckConfig `json:"health_check,omitempty"`
		Config           map[string]interface{}  `json:"config,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
			"code":  "INVALID_REQUEST",
			"details": map[string]interface{}{
				"validation_errors": err.Error(),
			},
		})
		return
	}

	// Build credential from auth config
	credential, err := api.buildCredential(request.AuthConfig)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid authentication configuration",
			"code":  "INVALID_AUTH_CONFIG",
			"details": map[string]interface{}{
				"reason": err.Error(),
			},
		})
		return
	}

	// Build tool config
	config := &tool.ToolConfig{
		TenantID:         tenantID,
		Type:             "openapi", // Default type for dynamic tools
		Name:             request.Name,
		DisplayName:      request.DisplayName,
		BaseURL:          request.BaseURL,
		DocumentationURL: request.DocumentationURL,
		OpenAPIURL:       request.OpenAPIURL,
		Config:           request.Config,
		Credential:       credential,
		RetryPolicy:      request.RetryPolicy,
		Status:           "active",
		HealthStatus:     "unknown",
	}

	if config.Config == nil {
		config.Config = make(map[string]interface{})
	}

	// Store URLs in config for persistence
	config.Config["base_url"] = request.BaseURL
	if request.DocumentationURL != "" {
		config.Config["documentation_url"] = request.DocumentationURL
	}
	if request.OpenAPIURL != "" {
		config.Config["openapi_url"] = request.OpenAPIURL
	}

	// Register the tool
	userID := c.GetString("user_id")
	result, err := api.toolRegistry.RegisterTool(c.Request.Context(), tenantID, config, userID)
	if err != nil {
		api.logger.Error("Failed to register tool", map[string]interface{}{
			"error":     err.Error(),
			"tool_name": request.Name,
			"tenant_id": tenantID,
		})

		// Check for specific errors
		if strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Tool with this name already exists",
				"code":  "DUPLICATE_TOOL",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to register tool",
			"code":  "REGISTRATION_FAILED",
			"details": map[string]interface{}{
				"reason": err.Error(),
			},
		})
		return
	}

	// Build response
	response := gin.H{
		"id":               config.ID,
		"name":             config.Name,
		"status":           config.Status,
		"discovery_method": "automatic",
		"_links": map[string]string{
			"self":    "/api/v1/tools/" + config.Name,
			"test":    "/api/v1/tools/" + config.Name + "/test",
			"execute": "/api/v1/tools/" + config.Name + "/execute",
		},
	}

	if result != nil {
		response["capabilities"] = result.Capabilities
		response["tools_created"] = len(result.Capabilities)
		if len(result.DiscoveredURLs) > 0 {
			response["openapi_url"] = result.DiscoveredURLs[0]
		}
	}

	c.JSON(http.StatusCreated, response)
}

// startDiscovery initiates a tool discovery session
func (api *DynamicToolAPI) startDiscovery(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant ID"})
		return
	}

	var request struct {
		BaseURL string                 `json:"base_url" binding:"required"`
		Hints   map[string]interface{} `json:"hints,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
			"code":  "INVALID_REQUEST",
		})
		return
	}

	// Create discovery session
	sessionID := uuid.New().String()
	session, err := api.discoveryService.StartDiscovery(c.Request.Context(), tenantID, sessionID, request.BaseURL, request.Hints)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to start discovery",
			"code":  "DISCOVERY_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id":  session.SessionID,
		"status":      session.Status,
		"suggestions": session.Suggestions,
		"expires_in":  int(time.Until(session.ExpiresAt).Seconds()),
	})
}

// confirmDiscovery confirms a discovery selection
func (api *DynamicToolAPI) confirmDiscovery(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant ID"})
		return
	}
	sessionID := c.Param("session_id")

	var request struct {
		SelectedURL string                 `json:"selected_url" binding:"required"`
		AuthToken   string                 `json:"auth_token" binding:"required"`
		ToolName    string                 `json:"tool_name" binding:"required"`
		AuthType    string                 `json:"auth_type,omitempty"`
		AuthConfig  map[string]interface{} `json:"auth_config,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
			"code":  "INVALID_REQUEST",
		})
		return
	}

	// Confirm discovery and create tool
	config, err := api.discoveryService.ConfirmDiscovery(
		c.Request.Context(),
		tenantID,
		sessionID,
		request.SelectedURL,
		request.ToolName,
		request.AuthToken,
		request.AuthConfig,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to confirm discovery",
			"code":  "CONFIRMATION_FAILED",
			"details": map[string]interface{}{
				"reason": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":     config.ID,
		"name":   config.Name,
		"status": config.Status,
		"_links": map[string]string{
			"self":    "/api/v1/tools/" + config.Name,
			"test":    "/api/v1/tools/" + config.Name + "/test",
			"actions": "/api/v1/tools/" + config.Name + "/actions",
		},
	})
}

// getToolDetails returns details about a specific tool
func (api *DynamicToolAPI) getToolDetails(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant ID"})
		return
	}
	toolName := c.Param("tool")

	tool, err := api.toolRegistry.GetToolForTenant(c.Request.Context(), tenantID, toolName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Tool not found",
			"code":  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                tool.ID,
		"name":              tool.Name,
		"type":              tool.Type,
		"display_name":      tool.DisplayName,
		"base_url":          tool.BaseURL,
		"status":            tool.Status,
		"health_status":     tool.HealthStatus,
		"last_health_check": tool.LastHealthCheck,
		"retry_policy":      tool.RetryPolicy,
		"_links": map[string]string{
			"self":    "/api/v1/tools/" + tool.Name,
			"test":    "/api/v1/tools/" + tool.Name + "/test",
			"actions": "/api/v1/tools/" + tool.Name + "/actions",
		},
	})
}

// updateTool updates a tool configuration
func (api *DynamicToolAPI) updateTool(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant ID"})
		return
	}
	toolName := c.Param("tool")

	var request map[string]interface{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
			"code":  "INVALID_REQUEST",
		})
		return
	}

	// Handle credential updates
	if authConfig, ok := request["auth_config"].(map[string]interface{}); ok {
		credential, err := api.buildCredential(authConfig)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid authentication configuration",
				"code":  "INVALID_AUTH_CONFIG",
			})
			return
		}
		request["credential"] = credential
		delete(request, "auth_config")
	}

	err := api.toolRegistry.UpdateTool(c.Request.Context(), tenantID, toolName, request)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Tool not found",
				"code":  "NOT_FOUND",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update tool",
			"code":  "UPDATE_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Tool updated successfully",
		"_links": map[string]string{
			"self": "/api/v1/tools/" + toolName,
		},
	})
}

// deleteTool removes a tool
func (api *DynamicToolAPI) deleteTool(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant ID"})
		return
	}
	toolName := c.Param("tool")

	err := api.toolRegistry.DeleteTool(c.Request.Context(), tenantID, toolName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Tool not found",
				"code":  "NOT_FOUND",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete tool",
			"code":  "DELETE_FAILED",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// testConnection tests tool connectivity
func (api *DynamicToolAPI) testConnection(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant ID"})
		return
	}
	toolName := c.Param("tool")

	health, err := api.toolRegistry.TestConnection(c.Request.Context(), tenantID, toolName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Tool not found",
			"code":  "NOT_FOUND",
		})
		return
	}

	status := "healthy"
	if !health.IsHealthy {
		status = "unhealthy"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":           status,
		"response_time_ms": health.ResponseTime,
		"version":          health.Version,
		"last_checked":     health.LastChecked,
		"cached":           health.WasCached,
		"error":            health.Error,
	})
}

// listToolActions returns available actions for a tool
func (api *DynamicToolAPI) listToolActions(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant ID"})
		return
	}
	toolName := c.Param("tool")

	actions, err := api.toolRegistry.GetToolActions(c.Request.Context(), tenantID, toolName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Tool not found",
			"code":  "NOT_FOUND",
		})
		return
	}

	// Convert to API response format
	response := make([]map[string]interface{}, 0, len(actions))
	for _, action := range actions {
		response = append(response, map[string]interface{}{
			"name":        action.Name,
			"description": action.Description,
			"method":      action.Method,
			"path":        action.Path,
			"parameters":  action.Parameters,
			"_links": map[string]string{
				"execute": "/api/v1/tools/" + toolName + "/actions/" + action.Name,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"tool":    toolName,
		"actions": response,
		"total":   len(response),
	})
}

// getActionDetails returns details about a specific action
func (api *DynamicToolAPI) getActionDetails(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant ID"})
		return
	}
	toolName := c.Param("tool")
	actionName := c.Param("action")

	action, err := api.toolRegistry.GetToolAction(c.Request.Context(), tenantID, toolName, actionName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Action not found",
			"code":  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":        action.Name,
		"description": action.Description,
		"method":      action.Method,
		"path":        action.Path,
		"parameters":  action.Parameters,
		"returns":     action.Returns,
		"_links": map[string]string{
			"execute": "/api/v1/tools/" + toolName + "/actions/" + action.Name,
		},
	})
}

// executeAction executes a tool action
func (api *DynamicToolAPI) executeAction(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant ID"})
		return
	}
	toolName := c.Param("tool")
	actionName := c.Param("action")

	var request struct {
		ContextID  string                 `json:"context_id"`
		Parameters map[string]interface{} `json:"parameters"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
			"code":  "INVALID_REQUEST",
		})
		return
	}

	// Execute the action
	result, err := api.executionService.ExecuteToolAction(
		c.Request.Context(),
		tenantID,
		toolName,
		actionName,
		request.Parameters,
		c.GetString("user_id"),
	)

	if err != nil {
		// Handle specific error types
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Tool or action not found",
				"code":  "NOT_FOUND",
			})
			return
		}

		if strings.Contains(err.Error(), "validation") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid parameters",
				"code":  "VALIDATION_ERROR",
				"details": map[string]interface{}{
					"reason": err.Error(),
				},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Action execution failed",
			"code":  "EXECUTION_FAILED",
			"details": map[string]interface{}{
				"reason": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result":            result.Result,
		"execution_time_ms": result.ExecutionTime,
		"retry_attempts":    result.RetryAttempts,
	})
}

// queryToolData executes a query against a tool
func (api *DynamicToolAPI) queryToolData(c *gin.Context) {
	// This would be implemented similarly to executeAction
	// but for data queries rather than actions
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Query endpoint not yet implemented",
		"code":  "NOT_IMPLEMENTED",
	})
}

// buildCredential creates a credential object from auth config
func (api *DynamicToolAPI) buildCredential(authConfig map[string]interface{}) (*tool.TokenCredential, error) {
	authType, ok := authConfig["type"].(string)
	if !ok {
		return nil, fmt.Errorf("auth type is required")
	}

	cred := &tool.TokenCredential{
		Type: authType,
	}

	switch authType {
	case "bearer", "token":
		token, ok := authConfig["token"].(string)
		if !ok {
			return nil, fmt.Errorf("token is required for %s auth", authType)
		}
		cred.Token = token

		if headerName, ok := authConfig["header_name"].(string); ok {
			cred.HeaderName = headerName
		}
		if headerPrefix, ok := authConfig["header_prefix"].(string); ok {
			cred.HeaderPrefix = headerPrefix
		}

	case "api_key":
		apiKey, ok := authConfig["api_key"].(string)
		if !ok {
			return nil, fmt.Errorf("api_key is required for api_key auth")
		}
		cred.APIKey = apiKey

		if headerName, ok := authConfig["header_name"].(string); ok {
			cred.HeaderName = headerName
		}

	case "basic":
		username, ok1 := authConfig["username"].(string)
		password, ok2 := authConfig["password"].(string)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("username and password are required for basic auth")
		}
		cred.Username = username
		cred.Password = password

	default:
		return nil, fmt.Errorf("unsupported auth type: %s", authType)
	}

	return cred, nil
}

// GetToolRegistry returns the tool registry (for testing)
func (api *DynamicToolAPI) GetToolRegistry() *services.ToolRegistry {
	return api.toolRegistry
}

// GetExecutionService returns the execution service (for testing)
func (api *DynamicToolAPI) GetExecutionService() *services.ExecutionService {
	return api.executionService
}
