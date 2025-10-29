package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/services"
	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	pkgrepository "github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/repository/credential"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	githubprovider "github.com/developer-mesh/developer-mesh/pkg/tools/providers/github"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/harness"
	"github.com/gin-gonic/gin"
)

// DynamicToolsAPI handles dynamic tool management endpoints
type DynamicToolsAPI struct {
	toolService       services.DynamicToolsServiceInterface
	logger            observability.Logger
	metricsClient     observability.MetricsClient
	auditLogger       *auth.AuditLogger
	enhancedToolsAPI  *EnhancedToolsAPI                        // Optional enhanced tools integration
	templateRepo      pkgrepository.ToolTemplateRepository     // For expanding organization tools
	orgToolRepo       pkgrepository.OrganizationToolRepository // For updating org tools with permissions
	encryptionService *security.EncryptionService              // For decrypting credentials
	credentialRepo    credential.Repository                    // For accessing user credentials
	toolFilterService *services.ToolFilterService              // For filtering tools based on configuration
}

// NewDynamicToolsAPI creates a new dynamic tools API handler
func NewDynamicToolsAPI(
	toolService services.DynamicToolsServiceInterface,
	logger observability.Logger,
	metricsClient observability.MetricsClient,
	auditLogger *auth.AuditLogger,
	templateRepo pkgrepository.ToolTemplateRepository,
	orgToolRepo pkgrepository.OrganizationToolRepository,
	encryptionService *security.EncryptionService,
	credentialRepo credential.Repository,
	toolFilterService *services.ToolFilterService,
) *DynamicToolsAPI {
	return &DynamicToolsAPI{
		toolService:       toolService,
		logger:            logger,
		metricsClient:     metricsClient,
		auditLogger:       auditLogger,
		templateRepo:      templateRepo,
		orgToolRepo:       orgToolRepo,
		encryptionService: encryptionService,
		credentialRepo:    credentialRepo,
		toolFilterService: toolFilterService,
	}
}

// SetEnhancedToolsAPI sets the enhanced tools API for integration
func (api *DynamicToolsAPI) SetEnhancedToolsAPI(enhancedAPI *EnhancedToolsAPI) {
	api.enhancedToolsAPI = enhancedAPI
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
		tools.POST("/:toolId/execute", api.ExecuteAction)
		tools.POST("/:toolId/execute/:action", api.ExecuteAction) // Keep for backward compatibility
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
	isEdgeMCP := c.Query("edge_mcp") == "true"

	// Get dynamic tools
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

	// Also get organization tools if enhanced tools API is available
	if api.enhancedToolsAPI != nil {
		orgTools, err := api.enhancedToolsAPI.GetToolsForTenant(c.Request.Context(), tenantID)
		if err != nil {
			api.logger.Warn("Failed to get organization tools", map[string]interface{}{
				"tenant_id": tenantID,
				"error":     err.Error(),
			})
		} else {
			api.logger.Info("Retrieved organization tools", map[string]interface{}{
				"tenant_id": tenantID,
				"count":     len(orgTools),
			})

			// Convert organization tools to dynamic tool format for consistency
			for _, orgTool := range orgTools {
				// Check if it's an OrganizationTool struct
				if ot, ok := orgTool.(*models.OrganizationTool); ok {
					api.logger.Debug("Processing organization tool", map[string]interface{}{
						"tool_id":       ot.ID,
						"instance_name": ot.InstanceName,
						"template_id":   ot.TemplateID,
						"has_creds":     len(ot.CredentialsEncrypted) > 0,
						"has_config":    ot.InstanceConfig != nil,
					})

					// Discover permissions for tools if not already done (for authenticated user)
					userID := c.GetString("user_id")
					if userID != "" && api.shouldDiscoverPermissions(c.Request.Context(), tenantID, userID, ot) {
						api.logger.Info("Tool needs permission discovery for user", map[string]interface{}{
							"tool_id":       ot.ID,
							"instance_name": ot.InstanceName,
							"user_id":       userID,
						})
						api.discoverAndStorePermissions(c.Request.Context(), tenantID, userID, ot)
					} else {
						api.logger.Debug("Tool does not need permission discovery", map[string]interface{}{
							"tool_id":       ot.ID,
							"instance_name": ot.InstanceName,
							"user_id":       userID,
						})
					}

					// Check if we should expand this tool based on its template
					if api.shouldExpandTool(ot) {
						// Expand the organization tool into multiple operation-specific tools
						// Pass userID for permission filtering
						expandedTools := api.expandOrganizationTool(c.Request.Context(), ot, userID)
						if len(expandedTools) > 0 {
							tools = append(tools, expandedTools...)
							api.logger.Info("Expanded organization tool into operations", map[string]interface{}{
								"tool_id":         ot.ID,
								"tool_name":       ot.InstanceName,
								"template_id":     ot.TemplateID,
								"operation_count": len(expandedTools),
							})
						} else {
							// Fall back to single tool if expansion fails
							api.logger.Warn("Failed to expand organization tool, using single tool", map[string]interface{}{
								"tool_id":     ot.ID,
								"tool_name":   ot.InstanceName,
								"template_id": ot.TemplateID,
							})
							tools = append(tools, api.createSingleToolFromOrgTool(ot, tenantID))
						}
					} else {
						// Create a single tool representation for non-expandable tools
						tools = append(tools, api.createSingleToolFromOrgTool(ot, tenantID))
					}
				} else if dt, ok := orgTool.(*models.DynamicTool); ok {
					// It's already a dynamic tool, just add it
					tools = append(tools, dt)
				}
			}
		}
	}

	// Apply tool filtering for Edge MCP clients if filter service is available
	if isEdgeMCP && api.toolFilterService != nil {
		// Extract tool names for filtering
		toolNames := make([]string, len(tools))
		for i, tool := range tools {
			toolNames[i] = tool.ToolName
		}

		// Apply filtering
		filteredNames := api.toolFilterService.FilterTools(toolNames)

		// Create a set of filtered names for O(1) lookup
		filteredSet := make(map[string]bool)
		for _, name := range filteredNames {
			filteredSet[name] = true
		}

		// Filter the tools array
		filteredTools := make([]*models.DynamicTool, 0, len(filteredNames))
		for _, tool := range tools {
			if filteredSet[tool.ToolName] {
				filteredTools = append(filteredTools, tool)
			}
		}

		// Replace tools with filtered list
		originalCount := len(tools)
		tools = filteredTools

		api.logger.Info("Applied tool filtering for Edge MCP", map[string]interface{}{
			"tenant_id":      tenantID,
			"original_count": originalCount,
			"filtered_count": len(tools),
			"reduction":      originalCount - len(tools),
		})
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

	// For edge-mcp, return a lightweight response without schemas or specs
	if isEdgeMCP {
		lightweightTools := make([]map[string]interface{}, 0, len(tools))
		for _, tool := range tools {
			// Create minimal tool representation with basic inputSchema
			lightTool := map[string]interface{}{
				"id":           tool.ID, // Include ID for execution
				"tool_name":    tool.ToolName,
				"display_name": tool.DisplayName,
				"description":  tool.DisplayName + " integration",
			}

			// Add minimal inputSchema for MCP compatibility
			// This is a generic schema that works for most GitHub operations
			inputSchema := api.generateMinimalInputSchema(tool.ToolName)
			if inputSchema != nil {
				lightTool["schema"] = inputSchema
			}

			// Add absolutely minimal config - just metadata, no schemas
			if tool.Config != nil {
				minimalConfig := make(map[string]interface{})
				for k, v := range tool.Config {
					// Only include basic metadata fields
					if k == "group_name" || k == "parent_api" || k == "spec_url" {
						minimalConfig[k] = v
					}
				}
				if len(minimalConfig) > 0 {
					lightTool["config"] = minimalConfig
				}
			}

			lightweightTools = append(lightweightTools, lightTool)
		}

		c.JSON(http.StatusOK, gin.H{
			"tools": lightweightTools,
			"count": len(lightweightTools),
		})
		return
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

	// Debug logging to understand the issue
	if tenantIDRaw, exists := c.Get("tenant_id"); exists {
		api.logger.Info("Debug: tenant_id from context", map[string]interface{}{
			"raw_type":     fmt.Sprintf("%T", tenantIDRaw),
			"raw_value":    fmt.Sprintf("%v", tenantIDRaw),
			"string_value": tenantID,
			"exists":       exists,
		})
	}

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

	// Check tenant_id
	if tenantID == "" {
		api.logger.Error("tenant_id is empty", map[string]interface{}{
			"path":   c.Request.URL.Path,
			"method": c.Request.Method,
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
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
		GroupOperations:   req.GroupOperations,
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

		// Check if it's a duplicate tool error
		if strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, gin.H{
				"error":      err.Error(),
				"suggestion": "A tool with this name already exists. Please use a different name or delete the existing tool first.",
			})
			return
		}

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
	userID := c.GetString("user_id")
	toolID := c.Param("toolId")
	encodedAction := c.Param("action")

	// URL-decode the action to handle encoded slashes and special characters
	action, decodeErr := url.QueryUnescape(encodedAction)
	if decodeErr != nil {
		// Fallback to using the raw value if decoding fails
		action = encodedAction
	}

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
		return
	}

	// Store the original tool ID for organization tool detection
	originalToolID := toolID

	// Variable to store extracted operation (defined at function scope)
	var extractedOperation string

	// Check if this is an expanded tool ID (format: parent_id_operation)
	// Check if this is an expanded tool (has UUID followed by operation)
	// Format: {uuid}-{operation} where uuid is 36 chars
	if len(toolID) > 36 && toolID[36] == '-' {
		// This is an expanded tool, extract the parent ID and operation
		parentToolID := toolID[:36]
		extractedOperation = toolID[37:]

		// Override the action with the operation name if not provided
		if action == "" || action == "execute" {
			action = extractedOperation
		}

		// Use the parent tool ID for execution
		toolID = parentToolID

		api.logger.Debug("Handling expanded tool execution", map[string]interface{}{
			"original_tool_id": originalToolID,
			"parent_tool_id":   parentToolID,
			"operation":        extractedOperation,
			"action":           action,
		})
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

	// Add pagination defaults for operations that can return large responses
	needsPagination := false
	defaultPerPage := 30 // Reasonable default to prevent token limit issues

	// Check if this is a list, search, or get-multiple operation
	if strings.Contains(req.Action, "list") || strings.Contains(req.Action, "_list") ||
		strings.Contains(req.Action, "search") || strings.Contains(req.Action, "_search") ||
		strings.Contains(req.Action, "get_all") || strings.Contains(req.Action, "fetch_all") {
		needsPagination = true
	}

	// Special cases for specific GitHub operations that return arrays
	paginatedOps := []string{
		"get_issue_comments", "get_issue_events", "get_pull_request_files",
		"get_pull_request_reviews", "get_pull_request_review_comments",
		"list_workflow_runs", "list_workflow_jobs", "list_artifacts",
		"list_commits", "list_branches", "list_tags", "list_releases",
		"list_notifications", "list_gists", "get_team_members",
		"discussion_comments_get", "get_issue_timeline",
	}

	for _, op := range paginatedOps {
		if strings.Contains(req.Action, op) {
			needsPagination = true
			break
		}
	}

	if needsPagination {
		if req.Parameters == nil {
			req.Parameters = make(map[string]interface{})
		}

		// Set reasonable defaults for MCP clients to prevent token limit issues
		if _, hasPerPage := req.Parameters["per_page"]; !hasPerPage {
			// Use smaller page size for operations that typically have larger payloads
			if strings.Contains(req.Action, "diff") || strings.Contains(req.Action, "files") {
				req.Parameters["per_page"] = 20
				defaultPerPage = 20
			} else if strings.Contains(req.Action, "search") {
				req.Parameters["per_page"] = 25 // Search results are often larger
				defaultPerPage = 25
			} else {
				req.Parameters["per_page"] = defaultPerPage
			}
		}
		if _, hasPage := req.Parameters["page"]; !hasPage {
			req.Parameters["page"] = 1
		}

		// Add limit parameter for operations that use it instead of per_page
		if _, hasLimit := req.Parameters["limit"]; !hasLimit {
			req.Parameters["limit"] = defaultPerPage
		}

		api.logger.Debug("Added pagination defaults", map[string]interface{}{
			"action":   req.Action,
			"per_page": req.Parameters["per_page"],
			"page":     req.Parameters["page"],
			"limit":    req.Parameters["limit"],
		})
	}

	// Log what we received
	paramKeys := make([]string, 0, len(req.Parameters))
	for k := range req.Parameters {
		paramKeys = append(paramKeys, k)
	}
	// Log passthrough auth details if present
	if req.PassthroughAuth != nil {
		authInfo := map[string]interface{}{
			"tenant_id":  tenantID,
			"tool_id":    toolID,
			"action":     req.Action,
			"params":     req.Parameters,
			"param_keys": paramKeys,
		}

		// Log credential details
		if req.PassthroughAuth.Credentials != nil {
			for provider, cred := range req.PassthroughAuth.Credentials {
				if cred != nil {
					authInfo[provider+"_token_len"] = len(cred.Token)
					if len(cred.Token) > 0 {
						authInfo[provider+"_preview"] = cred.Token[:min(10, len(cred.Token))] + "..."
					}
				}
			}
		}

		api.logger.Info("Executing tool action with passthrough", authInfo)
	} else {
		api.logger.Info("Executing tool action without passthrough", map[string]interface{}{
			"tenant_id":  tenantID,
			"tool_id":    toolID,
			"action":     req.Action,
			"params":     req.Parameters,
			"param_keys": paramKeys,
		})
	}

	// Extract passthrough authentication from headers if not in body
	if req.PassthroughAuth == nil {
		req.PassthroughAuth = api.extractPassthroughAuth(c)
	}

	// Determine whether to use passthrough or standard execution
	var result interface{}
	var err error

	// Check if this is an organization tool (expanded or not)
	// Use the original tool ID for detection since we may have modified toolID
	isOrganizationTool := false
	var parentToolID string
	var operationName string

	// First check if this looks like an expanded tool (provider_operation format)
	// Check if tool ID starts with any known provider prefix
	looksLikeProviderTool := false
	if api.enhancedToolsAPI != nil && strings.Contains(originalToolID, "_") {
		// Extract potential provider name
		parts := strings.SplitN(originalToolID, "_", 2)
		if len(parts) > 0 {
			providerName := parts[0]
			// Check against known providers
			knownProviders := []string{"github", "harness", "gitlab", "bitbucket", "jira", "slack", "aws", "azure", "gcp", "snyk", "sonarqube", "artifactory", "jenkins", "confluence"}
			for _, known := range knownProviders {
				if providerName == known {
					looksLikeProviderTool = true
					break
				}
			}
		}
	}

	if looksLikeProviderTool {
		api.logger.Info("Checking for expanded organization tool", map[string]interface{}{
			"tool_id":   originalToolID,
			"tenant_id": tenantID,
		})
		// This might be an expanded organization tool
		// Try to find it in our expanded tools list
		orgTools, getErr := api.enhancedToolsAPI.GetToolsForTenant(c.Request.Context(), tenantID)
		if getErr == nil && len(orgTools) > 0 {
			api.logger.Info("Got organization tools for tenant", map[string]interface{}{
				"count":     len(orgTools),
				"tenant_id": tenantID,
			})
			// Check each org tool and its potential expansions
			for _, orgTool := range orgTools {
				if ot, ok := orgTool.(*models.OrganizationTool); ok && api.shouldExpandTool(ot) {
					// Get the expanded tools for this org tool (with user permission filtering)
					expandedTools := api.expandOrganizationTool(c.Request.Context(), ot, userID)
					for _, expTool := range expandedTools {
						if expTool.ID == originalToolID {
							// Found it! Extract the parent tool ID and operation
							if expTool.Config != nil {
								if pid, ok := expTool.Config["parent_tool_id"].(string); ok {
									parentToolID = pid
								}
								if op, ok := expTool.Config["operation"].(string); ok {
									operationName = op
								}
							}
							isOrganizationTool = true
							toolID = parentToolID // Use the parent tool ID for execution
							extractedOperation = operationName
							api.logger.Info("Found expanded organization tool", map[string]interface{}{
								"tool_id":        originalToolID,
								"parent_tool_id": parentToolID,
								"operation":      operationName,
							})
							break
						}
					}
					if isOrganizationTool {
						break
					}
				}
			}
		}
	}

	// If not found as expanded tool, check if it's a direct organization tool
	if !isOrganizationTool && api.enhancedToolsAPI != nil {
		// Check if this is a non-expanded organization tool
		// Try to get it from the enhanced registry first
		tools, getErr := api.enhancedToolsAPI.GetToolsForTenant(c.Request.Context(), tenantID)
		if getErr == nil {
			for _, t := range tools {
				if orgTool, ok := t.(*models.OrganizationTool); ok && orgTool.ID == toolID {
					isOrganizationTool = true
					break
				}
			}
		}
	}

	// Always attempt to load user credentials for any tool that might need authentication
	// All authentication is now tied to the user's DevMesh API key
	// Extract provider from tool ID pattern: {provider}_{operation}
	if userID != "" {
		// Map of valid service types for quick lookup and validation
		validServiceTypes := map[string]models.ServiceType{
			"github":      models.ServiceTypeGitHub,
			"harness":     models.ServiceTypeHarness,
			"gitlab":      models.ServiceTypeGitLab,
			"bitbucket":   models.ServiceTypeBitbucket,
			"jira":        models.ServiceTypeJira,
			"slack":       models.ServiceTypeSlack,
			"aws":         models.ServiceTypeAWS,
			"azure":       models.ServiceTypeAzure,
			"gcp":         models.ServiceTypeGCP,
			"snyk":        models.ServiceTypeSnyk,
			"sonarqube":   models.ServiceTypeSonarQube,
			"artifactory": models.ServiceTypeArtifactory,
			"jenkins":     models.ServiceTypeJenkins,
			"confluence":  models.ServiceTypeConfluence,
		}

		// Extract potential provider name from tool ID
		// Pattern: "github_create_pull_request" -> "github"
		// Pattern: "harness_pipelines_list" -> "harness"
		parts := strings.SplitN(originalToolID, "_", 2)
		if len(parts) > 0 {
			providerName := parts[0]

			// Check if this is a valid service type we support
			if serviceType, isValid := validServiceTypes[providerName]; isValid {
				// Attempt to load user credentials for this provider
				userCred, credErr := api.credentialRepo.Get(c.Request.Context(), tenantID, userID, serviceType)
				if credErr == nil && userCred != nil {
					// Decrypt credentials
					decryptedCreds, decryptErr := api.encryptionService.DecryptCredential(userCred.EncryptedCredentials, tenantID)
					if decryptErr == nil {
						// Create passthrough auth bundle with user credentials
						req.PassthroughAuth = &models.PassthroughAuthBundle{
							Credentials: make(map[string]*models.PassthroughCredential),
						}

						// Parse decrypted credentials (they're JSON)
						var credMap map[string]interface{}
						if jsonErr := json.Unmarshal([]byte(decryptedCreds), &credMap); jsonErr == nil {
							// Extract token from credentials (supports multiple formats)
							var token string
							if t, ok := credMap["token"].(string); ok {
								token = t
							} else if t, ok := credMap["access_token"].(string); ok {
								token = t
							} else if t, ok := credMap["api_key"].(string); ok {
								token = t
							}

							if token != "" {
								req.PassthroughAuth.Credentials[providerName] = &models.PassthroughCredential{
									Token: token,
									Type:  "bearer",
								}

								api.logger.Info("Loaded user credentials for tool execution", map[string]interface{}{
									"provider":     providerName,
									"service_type": string(serviceType),
									"user_id":      userID,
									"tool_id":      originalToolID,
									"has_token":    token != "",
									"token_len":    len(token),
								})
							}
						}
					} else {
						api.logger.Warn("Failed to decrypt user credentials", map[string]interface{}{
							"provider": providerName,
							"user_id":  userID,
							"tool_id":  originalToolID,
							"error":    decryptErr.Error(),
						})
					}
				}
				// If credentials not found, continue without them
				// The tool execution will fail later if auth is required
			}
		}
	}

	// Route to appropriate execution handler
	if isOrganizationTool && api.enhancedToolsAPI != nil {
		// Use the extracted operation if available, otherwise use req.Action
		actionToUse := req.Action
		if extractedOperation != "" && extractedOperation != "execute" {
			// We extracted a specific operation from the tool ID (e.g., "issues_list")
			actionToUse = extractedOperation
			api.logger.Info("Using extracted operation for organization tool", map[string]interface{}{
				"original_action":     req.Action,
				"extracted_operation": extractedOperation,
				"tool_id":             toolID,
			})
		}

		// Check if we have passthrough auth for organization tools
		if req.PassthroughAuth != nil {
			// Use passthrough execution for organization tools
			result, err = api.enhancedToolsAPI.ExecuteToolInternalWithPassthrough(
				c.Request.Context(),
				tenantID,
				toolID,
				actionToUse,
				req.Parameters,
				req.PassthroughAuth,
			)
		} else {
			// Use standard execution for organization tools
			result, err = api.enhancedToolsAPI.ExecuteToolInternal(
				c.Request.Context(),
				tenantID,
				toolID,
				actionToUse,
				req.Parameters,
			)
		}
	} else if req.PassthroughAuth != nil {
		// Use passthrough execution for dynamic tools
		result, err = api.toolService.ExecuteToolActionWithPassthrough(
			c.Request.Context(),
			tenantID,
			toolID,
			req.Action,
			req.Parameters,
			req.PassthroughAuth,
		)
	} else {
		// Use standard execution for dynamic tools
		result, err = api.toolService.ExecuteToolAction(
			c.Request.Context(),
			tenantID,
			toolID,
			req.Action,
			req.Parameters,
		)
	}

	if err != nil {
		if err.Error() == "tool not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		api.logger.Error("Failed to execute tool action", map[string]interface{}{
			"tenant_id":       tenantID,
			"tool_id":         toolID,
			"action":          action,
			"has_passthrough": req.PassthroughAuth != nil,
			"error":           err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// extractPassthroughAuth extracts passthrough authentication from request headers
func (api *DynamicToolsAPI) extractPassthroughAuth(c *gin.Context) *models.PassthroughAuthBundle {
	// Check for passthrough token in headers
	passthroughToken := c.GetHeader("X-Passthrough-Token")
	if passthroughToken == "" {
		// Try alternative headers
		passthroughToken = c.GetHeader("X-User-Token")
		if passthroughToken == "" {
			passthroughToken = c.GetHeader("X-GitHub-Token") // Tool-specific headers
		}
	}

	// Check for agent context
	agentType := c.GetHeader("X-Agent-Type")
	if agentType == "" {
		agentType = c.GetString("agent_type") // From middleware
	}

	// If no passthrough data found, return nil
	if passthroughToken == "" && agentType == "" {
		return nil
	}

	// Build passthrough bundle
	bundle := &models.PassthroughAuthBundle{
		Credentials: make(map[string]*models.PassthroughCredential),
	}

	// Add token if present
	if passthroughToken != "" {
		bundle.Credentials["*"] = &models.PassthroughCredential{
			Type:  "bearer",
			Token: passthroughToken,
		}
	}

	// Add agent context if present
	if agentType != "" {
		bundle.AgentContext = &models.AgentContext{
			AgentType:   agentType,
			AgentID:     c.GetString("agent_id"),
			UserID:      c.GetString("user_id"),
			SessionID:   c.GetString("session_id"),
			Environment: c.GetString("environment"),
		}
	}

	// Check for OAuth token
	if authHeader := c.GetHeader("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		// Only use if it's different from the API key
		if token != c.GetString("api_key") {
			bundle.OAuthTokens = map[string]*models.OAuthToken{
				"*": {
					AccessToken: token,
					TokenType:   "Bearer",
				},
			}
		}
	}

	return bundle
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
	GroupOperations   bool                      `json:"group_operations,omitempty"`
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

	if tool.WebhookConfig != nil && len(*tool.WebhookConfig) > 0 {
		// Unmarshal the webhook config
		var webhookConfig models.ToolWebhookConfig
		if err := json.Unmarshal(*tool.WebhookConfig, &webhookConfig); err == nil {
			response["enabled"] = webhookConfig.Enabled
			response["auth_type"] = webhookConfig.AuthType

			if webhookConfig.SignatureHeader != "" {
				response["signature_header"] = webhookConfig.SignatureHeader
			}

			if webhookConfig.SignatureAlgorithm != "" {
				response["signature_algorithm"] = webhookConfig.SignatureAlgorithm
			}

			if len(webhookConfig.Events) > 0 {
				events := make([]string, 0, len(webhookConfig.Events))
				for _, e := range webhookConfig.Events {
					events = append(events, e.EventType)
				}
				response["supported_events"] = events
			}

			// Add setup instructions
			switch webhookConfig.AuthType {
			case "hmac":
				response["setup_instructions"] = []string{
					fmt.Sprintf("1. Configure your %s to send webhooks to: %s/api/webhooks/tools/%s", tool.ToolName, baseURL, toolID),
					"2. Set up HMAC signing with the provided secret",
					fmt.Sprintf("3. Include the signature in the '%s' header", webhookConfig.SignatureHeader),
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
	}

	c.JSON(http.StatusOK, response)
}

// generateMinimalInputSchema creates a minimal inputSchema for MCP compatibility
// This analyzes the tool name and creates a generic schema with common parameters
func (api *DynamicToolsAPI) generateMinimalInputSchema(toolName string) map[string]interface{} {
	// Create base schema structure
	schema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []string{},
	}

	properties := schema["properties"].(map[string]interface{})
	required := []string{}

	// Parse tool name to determine resource type and operation
	// Examples: github_repos, github_issues, github_pull_requests
	parts := strings.Split(toolName, "_")

	// Common parameters for most operations
	if len(parts) >= 2 {
		provider := parts[0] // e.g., "github", "gitlab", etc.

		// Add common repository parameters for source control tools
		if provider == "github" || provider == "gitlab" || provider == "bitbucket" {
			properties["owner"] = map[string]interface{}{
				"type":        "string",
				"description": "Repository owner or organization",
			}
			properties["repo"] = map[string]interface{}{
				"type":        "string",
				"description": "Repository name",
			}

			// Check for specific resource types
			if len(parts) > 1 {
				resource := parts[len(parts)-1]

				switch resource {
				case "issues", "issue":
					properties["issue_number"] = map[string]interface{}{
						"type":        "integer",
						"description": "Issue number",
					}
				case "pulls", "pull_requests", "pr":
					properties["pull_number"] = map[string]interface{}{
						"type":        "integer",
						"description": "Pull request number",
					}
				case "branches", "branch":
					properties["branch"] = map[string]interface{}{
						"type":        "string",
						"description": "Branch name",
					}
				}
			}
		}

		// Add action parameter for all tools
		properties["action"] = map[string]interface{}{
			"type":        "string",
			"description": "Action to perform",
			"enum":        []string{"list", "get", "create", "update", "delete"},
		}

		// Add generic parameters object for additional tool-specific params
		properties["parameters"] = map[string]interface{}{
			"type":                 "object",
			"description":          "Additional parameters specific to the action",
			"additionalProperties": true,
		}
	} else {
		// For unrecognized tool patterns, provide a completely generic schema
		properties["action"] = map[string]interface{}{
			"type":        "string",
			"description": "Action to perform",
		}
		properties["parameters"] = map[string]interface{}{
			"type":                 "object",
			"description":          "Parameters for the action",
			"additionalProperties": true,
		}
	}

	// Keep required array minimal - most params should be optional
	schema["required"] = required

	return schema
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

// shouldExpandTool determines if an organization tool should be expanded into multiple operations
func (api *DynamicToolsAPI) shouldExpandTool(ot *models.OrganizationTool) bool {
	// Expand any organization tool that has a template
	// Templates contain operation mappings that define individual tools
	return ot.TemplateID != ""
}

// expandOrganizationTool expands an organization tool into multiple operation-specific tools
// userID is optional - if provided, operations will be filtered based on user's permissions
func (api *DynamicToolsAPI) expandOrganizationTool(ctx context.Context, ot *models.OrganizationTool, userID string) []*models.DynamicTool {
	var expandedTools []*models.DynamicTool

	api.logger.Info("Starting organization tool expansion", map[string]interface{}{
		"org_tool_id":   ot.ID,
		"template_id":   ot.TemplateID,
		"instance_name": ot.InstanceName,
		"has_config":    ot.InstanceConfig != nil,
		"user_id":       userID,
	})

	// Skip if no template ID
	if ot.TemplateID == "" {
		api.logger.Debug("No template ID, skipping expansion", map[string]interface{}{
			"org_tool_id": ot.ID,
		})
		return expandedTools
	}

	// Fetch the template to get operation mappings
	template, err := api.templateRepo.GetByID(ctx, ot.TemplateID)
	if err != nil {
		api.logger.Warn("Failed to fetch template for organization tool", map[string]interface{}{
			"tool_id":     ot.ID,
			"template_id": ot.TemplateID,
			"error":       err.Error(),
		})
		return expandedTools
	}

	api.logger.Debug("Template fetched", map[string]interface{}{
		"provider_name":   template.ProviderName,
		"operation_count": len(template.OperationMappings),
	})

	// Get the base URL - prefer instance config over template default
	baseURL := ""
	if ot.InstanceConfig != nil {
		if url, ok := ot.InstanceConfig["base_url"].(string); ok {
			baseURL = url
		}
	}
	// Fall back to template default if not in instance config
	if baseURL == "" && template.DefaultConfig.BaseURL != "" {
		baseURL = template.DefaultConfig.BaseURL
	}

	// Extract tool definitions from AI definitions if available
	toolDefs := make(map[string]struct {
		displayName string
		description string
		category    string
	})

	// Parse AI definitions to get descriptive info and schemas for each operation
	var aiDefMap map[string]map[string]interface{}
	if template.AIDefinitions != nil {
		// Parse as array of AI tool definitions
		var aiDefs []map[string]interface{}
		if err := json.Unmarshal(*template.AIDefinitions, &aiDefs); err == nil {
			aiDefMap = make(map[string]map[string]interface{})
			for _, def := range aiDefs {
				if name, ok := def["name"].(string); ok {
					aiDefMap[name] = def
					// Also populate the legacy toolDefs map
					if desc, ok := def["description"].(string); ok {
						if cat, ok := def["category"].(string); ok {
							toolDefs[name] = struct {
								displayName string
								description string
								category    string
							}{
								displayName: name,
								description: desc,
								category:    cat,
							}
						}
					}
				}
			}
		}
	}

	// Retrieve user permissions if userID is provided
	var discoveredPermissions map[string]interface{}
	if userID != "" {
		// Map provider name to service type
		var serviceType models.ServiceType
		switch template.ProviderName {
		case "github":
			serviceType = models.ServiceTypeGitHub
		case "harness":
			serviceType = models.ServiceTypeHarness
		case "gitlab":
			serviceType = models.ServiceTypeGitLab
		case "bitbucket":
			serviceType = models.ServiceTypeBitbucket
		case "jira":
			serviceType = models.ServiceTypeJira
		case "slack":
			serviceType = models.ServiceTypeSlack
		default:
			// Unknown provider, no filtering
			serviceType = ""
		}

		// Get user credentials if service type is supported
		if serviceType != "" {
			userCred, err := api.credentialRepo.Get(ctx, ot.TenantID, userID, serviceType)
			if err == nil && userCred != nil && userCred.Metadata != nil {
				// Extract permissions from user credential metadata
				if perms, ok := userCred.Metadata["permissions"].(map[string]interface{}); ok {
					discoveredPermissions = perms
					api.logger.Info("Found user permissions for tool filtering", map[string]interface{}{
						"tool_id":           ot.ID,
						"user_id":           userID,
						"provider":          template.ProviderName,
						"permissions_count": len(perms),
					})
				} else {
					api.logger.Debug("No permissions found in user credentials", map[string]interface{}{
						"tool_id":  ot.ID,
						"user_id":  userID,
						"provider": template.ProviderName,
					})
				}
			} else if err != nil {
				api.logger.Debug("Could not retrieve user credentials for filtering", map[string]interface{}{
					"tool_id":  ot.ID,
					"user_id":  userID,
					"provider": template.ProviderName,
					"error":    err.Error(),
				})
			}
		}
	}

	operationCount := 0
	skippedCount := 0

	// Create a tool for each operation in the template
	for operationName, mapping := range template.OperationMappings {
		// Filter operations based on user permissions if available
		if discoveredPermissions != nil {
			if !api.isOperationAllowed(template.ProviderName, operationName, discoveredPermissions) {
				api.logger.Debug("Skipping operation due to insufficient user permissions", map[string]interface{}{
					"operation": operationName,
					"tool_id":   ot.ID,
					"user_id":   userID,
					"provider":  template.ProviderName,
				})
				skippedCount++
				continue
			}
		}
		operationCount++
		// Generate display name from operation name
		displayName := operationName
		description := fmt.Sprintf("Execute %s operation", operationName)
		category := "general"

		// Try to get better description from AI definitions
		if aiDefMap != nil {
			if aiDef, ok := aiDefMap[operationName]; ok {
				if desc, ok := aiDef["description"].(string); ok && desc != "" {
					description = desc
				}
				if cat, ok := aiDef["category"].(string); ok && cat != "" {
					category = cat
				}
			}
		}

		// Try to extract better names from operation groups
		for _, group := range template.OperationGroups {
			for _, op := range group.Operations {
				if op == operationName {
					category = strings.ToLower(strings.ReplaceAll(group.Name, " ", "_"))
					break
				}
			}
		}

		// Format the operation name for tool naming
		// Keep snake_case and use underscores for consistency (e.g., "ci/pipelines/list" -> "ci_pipelines_list")
		operationNameForTool := strings.ReplaceAll(operationName, "/", "_")
		operationNameForTool = strings.ReplaceAll(operationNameForTool, "-", "_")

		// Create tool name with provider prefix (e.g., "github_list_repositories")
		// Use the template provider name for consistency
		toolName := fmt.Sprintf("%s_%s", template.ProviderName, operationNameForTool)

		// Create display name
		if ot.DisplayName != "" {
			displayName = fmt.Sprintf("%s - %s", ot.DisplayName, operationName)
		}

		// Try to get the input schema from AI definitions
		var inputSchema map[string]interface{}
		if aiDefMap != nil {
			if aiDef, ok := aiDefMap[operationName]; ok {
				if schema, ok := aiDef["inputSchema"].(map[string]interface{}); ok {
					inputSchema = schema
				}
			}
		}

		// If no schema from AI definitions, build basic schema from operation parameters
		if inputSchema == nil && (len(mapping.RequiredParams) > 0 || len(mapping.OptionalParams) > 0) {
			properties := make(map[string]interface{})
			for _, param := range mapping.RequiredParams {
				properties[param] = map[string]interface{}{
					"type":        "string",
					"description": fmt.Sprintf("Required parameter: %s", param),
				}
			}
			for _, param := range mapping.OptionalParams {
				properties[param] = map[string]interface{}{
					"type":        "string",
					"description": fmt.Sprintf("Optional parameter: %s", param),
				}
			}
			inputSchema = map[string]interface{}{
				"type":       "object",
				"properties": properties,
				"required":   mapping.RequiredParams,
			}
		}

		// Get operation metadata if provider is GitHub
		var metadata *json.RawMessage
		if template.ProviderName == "github" {
			// Import the github provider package to get metadata
			if metadataFunc := api.getGitHubOperationMetadata(operationName); metadataFunc != nil {
				metadataBytes, _ := json.Marshal(metadataFunc)
				metadata = (*json.RawMessage)(&metadataBytes)
			}
		}

		// Create the dynamic tool
		tool := &models.DynamicTool{
			ID:          fmt.Sprintf("%s_%s", template.ProviderName, operationNameForTool),
			TenantID:    ot.TenantID,
			ToolName:    toolName,
			DisplayName: displayName,
			BaseURL:     baseURL,
			InputSchema: inputSchema,
			Status:      ot.Status,
			ToolType:    "organization_tool_operation",
			Config: map[string]interface{}{
				"type":           "organization_tool_operation",
				"parent_tool_id": ot.ID,
				"template_id":    ot.TemplateID,
				"org_id":         ot.OrganizationID,
				"operation":      operationName,
				"category":       category,
				"method":         mapping.Method,
				"path":           mapping.PathTemplate,
				"provider":       template.ProviderName,
			},
			IsActive:    ot.IsActive,
			Description: &description,
			Metadata:    metadata,
		}

		// Add encrypted credentials if present
		if len(ot.CredentialsEncrypted) > 0 {
			tool.CredentialsEncrypted = ot.CredentialsEncrypted
		}

		expandedTools = append(expandedTools, tool)
	}

	// Log summary of expansion results
	if template.ProviderName == "harness" {
		api.logger.Info("Completed Harness tool expansion", map[string]interface{}{
			"tool_id":          ot.ID,
			"total_operations": len(template.OperationMappings),
			"expanded_count":   operationCount,
			"skipped_count":    skippedCount,
			"final_tools":      len(expandedTools),
		})
	}

	return expandedTools
}

// createSingleToolFromOrgTool creates a single dynamic tool representation from an organization tool
func (api *DynamicToolsAPI) createSingleToolFromOrgTool(ot *models.OrganizationTool, tenantID string) *models.DynamicTool {
	// Extract base URL from instance config if available
	baseURL := "https://api.github.com" // Default for now
	if ot.InstanceConfig != nil {
		if url, ok := ot.InstanceConfig["base_url"].(string); ok {
			baseURL = url
		}
	}

	// Create a dynamic tool representation
	tool := &models.DynamicTool{
		ID:          ot.ID,
		TenantID:    tenantID,
		ToolName:    ot.InstanceName,
		DisplayName: ot.DisplayName,
		BaseURL:     baseURL,
		Status:      ot.Status,
		ToolType:    "organization_tool",
		Config: map[string]interface{}{
			"type":        "organization_tool",
			"template_id": ot.TemplateID,
			"org_id":      ot.OrganizationID,
		},
		IsActive: ot.IsActive,
	}

	// Add encrypted credentials if present
	if len(ot.CredentialsEncrypted) > 0 {
		tool.CredentialsEncrypted = ot.CredentialsEncrypted
	}

	return tool
}

// getGitHubOperationMetadata retrieves metadata for a GitHub operation
func (api *DynamicToolsAPI) getGitHubOperationMetadata(operationName string) map[string]interface{} {
	metadata := githubprovider.GetOperationMetadata(operationName)

	// Add response example to metadata
	if example := githubprovider.GetOperationResponseExample(operationName); example != nil {
		metadata["response_example"] = example
	}

	return metadata
}

// shouldDiscoverPermissions checks if we should discover permissions for a user's credentials
func (api *DynamicToolsAPI) shouldDiscoverPermissions(ctx context.Context, tenantID, userID string, ot *models.OrganizationTool) bool {
	// Only discover permissions for tools with templates
	if ot.TemplateID == "" {
		return false
	}

	// Check if template exists and get provider name
	template, err := api.templateRepo.GetByID(ctx, ot.TemplateID)
	if err != nil || template == nil {
		return false
	}

	// Map provider name to service type
	var serviceType models.ServiceType
	switch template.ProviderName {
	case "github":
		serviceType = models.ServiceTypeGitHub
	case "harness":
		serviceType = models.ServiceTypeHarness
	case "gitlab":
		serviceType = models.ServiceTypeGitLab
	case "bitbucket":
		serviceType = models.ServiceTypeBitbucket
	case "jira":
		serviceType = models.ServiceTypeJira
	case "slack":
		serviceType = models.ServiceTypeSlack
	default:
		// Unknown provider, skip discovery
		return false
	}

	// Check if user has credentials for this provider
	userCred, err := api.credentialRepo.Get(ctx, tenantID, userID, serviceType)
	if err != nil || userCred == nil {
		return false // User doesn't have credentials for this provider
	}

	// Check if we already have recent permissions in user's credential metadata
	if userCred.Metadata != nil {
		if perms, ok := userCred.Metadata["permissions"]; ok && perms != nil {
			// Check if permissions are recent (within 24 hours)
			if discoveredAt, ok := userCred.Metadata["permissions_discovered_at"].(string); ok {
				if parsed, err := time.Parse(time.RFC3339, discoveredAt); err == nil {
					if time.Since(parsed) < 24*time.Hour {
						return false // Have recent permissions
					}
				}
			}
		}
	}

	return true
}

// discoverAndStorePermissions discovers and stores permissions for a user's credentials
func (api *DynamicToolsAPI) discoverAndStorePermissions(ctx context.Context, tenantID, userID string, ot *models.OrganizationTool) {
	// Get template to determine provider
	template, err := api.templateRepo.GetByID(ctx, ot.TemplateID)
	if err != nil || template == nil {
		api.logger.Error("Failed to fetch template for permission discovery", map[string]interface{}{
			"tool_id":     ot.ID,
			"template_id": ot.TemplateID,
			"error":       err,
		})
		return
	}

	// Map provider name to service type
	var serviceType models.ServiceType
	switch template.ProviderName {
	case "github":
		serviceType = models.ServiceTypeGitHub
	case "harness":
		serviceType = models.ServiceTypeHarness
	case "gitlab":
		serviceType = models.ServiceTypeGitLab
	case "bitbucket":
		serviceType = models.ServiceTypeBitbucket
	default:
		api.logger.Warn("Unsupported provider for permission discovery", map[string]interface{}{
			"provider": template.ProviderName,
		})
		return
	}

	api.logger.Info("Starting permission discovery for user", map[string]interface{}{
		"tool_id":      ot.ID,
		"tool_name":    ot.InstanceName,
		"provider":     template.ProviderName,
		"user_id":      userID,
		"service_type": serviceType,
	})

	// Get user's credentials
	userCred, err := api.credentialRepo.Get(ctx, tenantID, userID, serviceType)
	if err != nil {
		api.logger.Error("Failed to get user credentials for permission discovery", map[string]interface{}{
			"user_id":      userID,
			"service_type": serviceType,
			"error":        err.Error(),
		})
		return
	}

	// Decrypt user credentials
	decrypted, err := api.encryptionService.DecryptCredential(userCred.EncryptedCredentials, tenantID)
	if err != nil {
		api.logger.Error("Failed to decrypt user credentials for permission discovery", map[string]interface{}{
			"user_id": userID,
			"error":   err.Error(),
		})
		return
	}

	// Parse credentials
	var credentials map[string]string
	if err := json.Unmarshal([]byte(decrypted), &credentials); err != nil {
		// Fallback: assume single token
		credentials = map[string]string{
			"token":   decrypted,
			"api_key": decrypted,
		}
	}

	// Extract token/API key
	token := ""
	if key, ok := credentials["token"]; ok {
		token = key
	} else if key, ok := credentials["api_key"]; ok {
		token = key
	} else if key, ok := credentials["personal_access_token"]; ok {
		token = key
	}

	if token == "" {
		api.logger.Warn("No token found in user credentials", map[string]interface{}{
			"user_id": userID,
		})
		return
	}

	// Discover permissions based on provider
	var permissionsMap map[string]interface{}

	switch template.ProviderName {
	case "harness":
		// Use Harness-specific discoverer
		discoverer := harness.NewHarnessPermissionDiscoverer(api.logger)
		permissions, err := discoverer.DiscoverPermissions(ctx, token)
		if err != nil {
			api.logger.Error("Failed to discover Harness permissions", map[string]interface{}{
				"user_id": userID,
				"error":   err.Error(),
			})
			return
		}

		api.logger.Info("Discovered Harness permissions", map[string]interface{}{
			"user_id":         userID,
			"enabled_modules": permissions.EnabledModules,
			"scope_count":     len(permissions.Scopes),
			"resource_access": len(permissions.ResourceAccess),
		})

		permissionsMap = map[string]interface{}{
			"enabled_modules": permissions.EnabledModules,
			"resource_access": permissions.ResourceAccess,
			"project_access":  permissions.ProjectAccess,
			"org_access":      permissions.OrgAccess,
			"scopes":          permissions.Scopes,
		}

	case "github":
		// Use generic discoverer for GitHub
		genericDiscoverer := tools.NewPermissionDiscoverer(api.logger)
		permissions, err := genericDiscoverer.DiscoverPermissions(ctx, "https://api.github.com", token, "bearer")
		if err != nil {
			api.logger.Error("Failed to discover GitHub permissions", map[string]interface{}{
				"user_id": userID,
				"error":   err.Error(),
			})
			return
		}

		api.logger.Info("Discovered GitHub permissions", map[string]interface{}{
			"user_id":     userID,
			"scopes":      permissions.Scopes,
			"scope_count": len(permissions.Scopes),
		})

		permissionsMap = map[string]interface{}{
			"scopes":      permissions.Scopes,
			"user_info":   permissions.UserInfo,
			"raw_headers": permissions.RawHeaders,
		}

	default:
		api.logger.Warn("No permission discoverer for provider", map[string]interface{}{
			"provider": template.ProviderName,
		})
		return
	}

	// Store permissions in user credential metadata
	if userCred.Metadata == nil {
		userCred.Metadata = make(map[string]interface{})
	}

	userCred.Metadata["permissions"] = permissionsMap
	userCred.Metadata["permissions_discovered_at"] = time.Now().UTC().Format(time.RFC3339)
	userCred.Metadata["provider"] = template.ProviderName

	// Update user credentials in database
	if err := api.credentialRepo.Update(ctx, userCred); err != nil {
		api.logger.Error("Failed to update user credentials with permissions", map[string]interface{}{
			"user_id": userID,
			"error":   err.Error(),
		})
	} else {
		api.logger.Info("Successfully stored discovered permissions for user", map[string]interface{}{
			"user_id":      userID,
			"provider":     template.ProviderName,
			"service_type": serviceType,
		})
	}
}

// isOperationAllowed checks if an operation is allowed based on discovered permissions
// Handles different providers (harness, github, etc.)
func (api *DynamicToolsAPI) isOperationAllowed(provider string, operationName string, permissions map[string]interface{}) bool {
	// Handle different providers
	switch provider {
	case "harness":
		return api.isHarnessOperationAllowed(operationName, permissions)
	case "github":
		return api.isGitHubOperationAllowed(operationName, permissions)
	default:
		// For unknown providers, allow by default (no filtering)
		return true
	}
}

// isHarnessOperationAllowed checks if a Harness operation is allowed based on discovered permissions
func (api *DynamicToolsAPI) isHarnessOperationAllowed(operationName string, permissions map[string]interface{}) bool {
	// Extract enabled modules from permissions
	enabledModules, ok := permissions["enabled_modules"].(map[string]interface{})
	if !ok || enabledModules == nil {
		// If no modules info, allow by default (backward compatibility)
		return true
	}

	// Extract resource access permissions
	resourceAccess, _ := permissions["resource_access"].(map[string]interface{})

	// Map operation names to required modules and permissions
	// Parse operation name to determine module (e.g., "pipelines/list" -> "pipeline" module)
	parts := strings.Split(operationName, "/")
	if len(parts) == 0 {
		return true // Allow if we can't parse
	}

	// Determine module from operation prefix
	moduleRequired := ""
	switch parts[0] {
	case "pipelines":
		moduleRequired = "pipeline"
	case "projects":
		moduleRequired = "project"
	case "connectors":
		moduleRequired = "connector"
	case "gitops":
		moduleRequired = "gitops"
	case "sto":
		moduleRequired = "sto"
	case "ccm":
		moduleRequired = "ccm"
	case "cv":
		moduleRequired = "cv"
	case "chaos":
		moduleRequired = "chaos"
	case "featureflags", "cf":
		moduleRequired = "cf"
	case "iacm":
		moduleRequired = "iacm"
	case "ssca":
		moduleRequired = "ssca"
	case "idp":
		moduleRequired = "idp"
	case "services":
		moduleRequired = "service"
	case "environments":
		moduleRequired = "environment"
	case "infrastructures":
		moduleRequired = "infrastructure"
	case "templates":
		moduleRequired = "template"
	case "secrets":
		moduleRequired = "secret"
	case "delegates":
		moduleRequired = "delegate"
	case "approvals":
		moduleRequired = "approval"
	case "notifications":
		moduleRequired = "notification"
	case "webhooks":
		moduleRequired = "webhook"
	case "variables":
		moduleRequired = "variable"
	case "triggers":
		moduleRequired = "trigger"
	case "inputsets":
		moduleRequired = "inputset"
	case "executions":
		moduleRequired = "execution"
	case "governance":
		moduleRequired = "governance"
	case "rbac", "roles", "usergroups", "permissions":
		moduleRequired = "rbacpolicy"
	case "audit":
		moduleRequired = "audit"
	case "dashboards":
		moduleRequired = "dashboard"
	case "logs":
		moduleRequired = "logs"
	case "licenses":
		moduleRequired = "license"
	case "account", "orgs":
		// Basic platform operations, generally allowed
		return true
	default:
		// If we don't recognize the module, check if it's a general operation
		// Allow by default for backward compatibility
		return true
	}

	// Check if the required module is enabled
	if moduleRequired != "" {
		if enabled, ok := enabledModules[moduleRequired].(bool); ok && !enabled {
			api.logger.Debug("Module not enabled for operation", map[string]interface{}{
				"operation": operationName,
				"module":    moduleRequired,
			})
			return false
		}
	}

	// Additional permission checks for specific operations
	if len(parts) >= 2 && resourceAccess != nil {
		operation := parts[1]
		resource := parts[0]

		// Check specific resource permissions
		if perms, ok := resourceAccess[resource].([]interface{}); ok {
			// Check if operation requires specific permission
			requiredPerm := ""
			switch operation {
			case "create":
				requiredPerm = "create"
			case "update", "edit":
				requiredPerm = "update"
			case "delete", "remove":
				requiredPerm = "delete"
			case "list", "get", "read":
				requiredPerm = "view"
			case "execute", "run":
				requiredPerm = "execute"
			}

			if requiredPerm != "" {
				hasPermission := false
				for _, p := range perms {
					if perm, ok := p.(string); ok && (perm == requiredPerm || perm == "*") {
						hasPermission = true
						break
					}
				}

				if !hasPermission {
					api.logger.Debug("Insufficient permissions for operation", map[string]interface{}{
						"operation":     operationName,
						"required_perm": requiredPerm,
						"resource":      resource,
					})
					return false
				}
			}
		}
	}

	return true
}

// isGitHubOperationAllowed checks if a GitHub operation is allowed based on discovered scopes
func (api *DynamicToolsAPI) isGitHubOperationAllowed(operationName string, permissions map[string]interface{}) bool {
	// Extract scopes from permissions
	scopes, ok := permissions["scopes"].([]interface{})
	if !ok || scopes == nil {
		// If no scopes info, allow by default (backward compatibility)
		return true
	}

	// Convert scopes to string array for easier checking
	scopeSet := make(map[string]bool)
	for _, scope := range scopes {
		if scopeStr, ok := scope.(string); ok {
			scopeSet[scopeStr] = true
		}
	}

	// Parse operation name to determine required scope
	// GitHub operations follow patterns like: repos/get, issues/create, pulls/list
	parts := strings.Split(operationName, "/")
	if len(parts) < 2 {
		// If we can't parse, allow by default
		return true
	}

	resource := parts[0]
	operation := parts[1]

	// Map GitHub resources and operations to required scopes
	requiredScope := ""
	switch resource {
	case "repos", "repository":
		// Repository operations require repo scope
		if operation == "create" || operation == "update" || operation == "delete" {
			requiredScope = "repo"
		} else {
			// Read operations might work with public_repo or repo
			if scopeSet["repo"] || scopeSet["public_repo"] {
				return true
			}
		}
	case "issues":
		// Issues typically need repo scope
		requiredScope = "repo"
	case "pulls", "pull_request", "pull_requests":
		// Pull requests need repo scope
		requiredScope = "repo"
	case "actions", "workflow", "workflows":
		// GitHub Actions need workflow or repo scope
		if scopeSet["workflow"] || scopeSet["repo"] {
			return true
		}
	case "gists", "gist":
		// Gists need gist scope
		requiredScope = "gist"
	case "user", "users":
		// User operations might need user scope for writes
		if operation == "update" {
			requiredScope = "user"
		} else {
			// Read operations typically work without special scope
			return true
		}
	case "orgs", "organization", "organizations":
		// Org operations need various scopes
		if operation == "update" || operation == "create" {
			requiredScope = "admin:org"
		} else {
			// Read operations typically work with read:org or admin:org
			if scopeSet["read:org"] || scopeSet["admin:org"] {
				return true
			}
		}
	case "teams", "team":
		// Team operations need read:org or admin:org
		if operation == "create" || operation == "update" || operation == "delete" {
			requiredScope = "admin:org"
		} else {
			if scopeSet["read:org"] || scopeSet["admin:org"] {
				return true
			}
		}
	default:
		// For unknown resources, check if user has general repo access
		if scopeSet["repo"] {
			return true
		}
		// Allow by default if we don't recognize the resource
		return true
	}

	// Check if required scope is present
	if requiredScope != "" {
		if scopeSet[requiredScope] {
			return true
		}
		api.logger.Debug("Insufficient GitHub scopes for operation", map[string]interface{}{
			"operation":        operationName,
			"required_scope":   requiredScope,
			"available_scopes": scopes,
		})
		return false
	}

	return true
}
