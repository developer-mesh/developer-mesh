package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	pkgservices "github.com/developer-mesh/developer-mesh/pkg/services"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// EnhancedToolsAPI handles enhanced tool management with templated and dynamic tools
type EnhancedToolsAPI struct {
	dynamicToolsAPI *DynamicToolsAPI
	toolRegistry    *pkgservices.EnhancedToolRegistry
	templateRepo    repository.ToolTemplateRepository
	db              *sqlx.DB
	logger          observability.Logger
	metricsClient   observability.MetricsClient
	auditLogger     *auth.AuditLogger
}

// NewEnhancedToolsAPI creates a new enhanced tools API handler
func NewEnhancedToolsAPI(
	dynamicToolsAPI *DynamicToolsAPI,
	toolRegistry *pkgservices.EnhancedToolRegistry,
	templateRepo repository.ToolTemplateRepository,
	db *sqlx.DB,
	logger observability.Logger,
	metricsClient observability.MetricsClient,
	auditLogger *auth.AuditLogger,
) *EnhancedToolsAPI {
	return &EnhancedToolsAPI{
		dynamicToolsAPI: dynamicToolsAPI,
		toolRegistry:    toolRegistry,
		templateRepo:    templateRepo,
		db:              db,
		logger:          logger,
		metricsClient:   metricsClient,
		auditLogger:     auditLogger,
	}
}

// RegisterRoutes registers enhanced tool API routes
func (api *EnhancedToolsAPI) RegisterRoutes(router *gin.RouterGroup) {
	// Organization-specific tool management
	orgs := router.Group("/organizations/:orgId")
	{
		orgs.GET("/tools", api.ListOrganizationTools)
		orgs.POST("/tools", api.CreateOrganizationTool)
		orgs.GET("/tools/:toolId", api.GetOrganizationTool)
		orgs.PUT("/tools/:toolId", api.UpdateOrganizationTool)
		orgs.DELETE("/tools/:toolId", api.DeleteOrganizationTool)
		orgs.POST("/tools/:toolId/execute", api.ExecuteOrganizationTool)
	}

	// Tool templates (platform-wide)
	templates := router.Group("/tool-templates")
	{
		templates.GET("", api.ListTemplates)
		templates.GET("/:templateId", api.GetTemplate)
		templates.GET("/search", api.SearchTemplates)
	}

	// Enhanced discovery with template detection
	router.POST("/tools/discover-enhanced", api.EnhancedDiscovery)

	// Migration endpoint
	router.POST("/tools/migrate/:toolId", api.MigrateDynamicTool)
}

// GetToolsForTenant returns all tools available for a tenant (used internally)
func (api *EnhancedToolsAPI) GetToolsForTenant(ctx context.Context, tenantID string) ([]interface{}, error) {
	return api.toolRegistry.GetToolsForTenant(ctx, tenantID)
}

// ExecuteToolInternal executes a tool operation (used internally by DynamicToolsAPI)
func (api *EnhancedToolsAPI) ExecuteToolInternal(ctx context.Context, tenantID, toolID, action string, params map[string]interface{}) (interface{}, error) {
	return api.toolRegistry.ExecuteTool(ctx, tenantID, toolID, action, params)
}

// ExecuteToolInternalWithPassthrough executes a tool operation with passthrough auth (used internally by DynamicToolsAPI)
func (api *EnhancedToolsAPI) ExecuteToolInternalWithPassthrough(
	ctx context.Context,
	tenantID, toolID, action string,
	params map[string]interface{},
	passthroughAuth *models.PassthroughAuthBundle,
) (interface{}, error) {
	return api.toolRegistry.ExecuteToolWithPassthrough(ctx, tenantID, toolID, action, params, passthroughAuth)
}

// ListOrganizationTools lists all tools for an organization
func (api *EnhancedToolsAPI) ListOrganizationTools(c *gin.Context) {
	orgID := c.Param("orgId")
	tenantID := c.GetString("tenant_id")

	// For now, use tenant as org if org is not specified
	if orgID == "" {
		orgID = tenantID
	}

	tools, err := api.toolRegistry.GetToolsForTenant(c.Request.Context(), tenantID)
	if err != nil {
		api.logger.Error("Failed to list organization tools", map[string]interface{}{
			"org_id":    orgID,
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tools"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tools": tools,
		"count": len(tools),
	})
}

// CreateOrganizationTool creates a new tool instance for an organization
func (api *EnhancedToolsAPI) CreateOrganizationTool(c *gin.Context) {
	orgID := c.Param("orgId")
	tenantID := c.GetString("tenant_id")

	var req CreateOrganizationToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Look up organization ID from slug
	orgUUID, err := api.getOrganizationIDFromSlug(c.Request.Context(), orgID)
	if err != nil {
		api.logger.Error("Failed to resolve organization", map[string]interface{}{
			"org_slug": orgID,
			"error":    err.Error(),
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization"})
		return
	}

	// Determine which flow to use
	var templateName string

	// Priority 1: Explicit template ID or name
	if req.TemplateID != "" {
		templateName = req.TemplateID
	} else if req.TemplateName != "" {
		templateName = req.TemplateName
	} else if req.BaseURL != "" && api.isKnownProviderURL(req.BaseURL) {
		// Priority 2: Detect from URL
		templateName = api.getProviderNameFromURL(req.BaseURL)
	}

	// Use the appropriate name field
	toolName := req.InstanceName
	if toolName == "" {
		toolName = req.Name
	}

	// Merge config fields
	config := req.Config
	if req.InstanceConfig != nil {
		if config == nil {
			config = req.InstanceConfig
		} else {
			// Merge instance_config into config
			for k, v := range req.InstanceConfig {
				config[k] = v
			}
		}
	}

	if templateName != "" {
		// Add display_name to config if provided
		if req.DisplayName != "" {
			if config == nil {
				config = make(map[string]interface{})
			}
			config["display_name"] = req.DisplayName
		}

		// Use template-based creation
		tool, err := api.toolRegistry.InstantiateToolForOrg(
			c.Request.Context(),
			orgUUID,
			tenantID,
			templateName,
			toolName,
			req.Credentials,
			config,
		)
		if err != nil {
			api.logger.Error("Failed to create organization tool from template", map[string]interface{}{
				"org_id":   orgID,
				"template": templateName,
				"error":    err.Error(),
			})
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, tool)
		return
	}

	// Fall back to dynamic tool creation if no template
	if req.BaseURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Either template_id/template_name or base_url is required"})
		return
	}

	api.createDynamicToolFallback(c, req)
}

// GetOrganizationTool gets a specific organization tool
func (api *EnhancedToolsAPI) GetOrganizationTool(c *gin.Context) {
	orgID := c.Param("orgId")
	toolID := c.Param("toolId")
	tenantID := c.GetString("tenant_id")

	// Try to get as organization tool first
	ctx := c.Request.Context()
	tool, err := api.getToolByID(ctx, tenantID, toolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
		return
	}

	_ = orgID // Use orgID for authorization in the future
	c.JSON(http.StatusOK, tool)
}

// UpdateOrganizationTool updates an organization tool
func (api *EnhancedToolsAPI) UpdateOrganizationTool(c *gin.Context) {
	// Check for refresh action in query params
	action := c.Query("action")
	if action == "refresh" {
		api.RefreshOrganizationTool(c)
		return
	}

	// Regular update not yet implemented
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Tool update not implemented. Use ?action=refresh to refresh tool capabilities"})
}

// RefreshOrganizationTool refreshes an organization tool with latest provider capabilities
func (api *EnhancedToolsAPI) RefreshOrganizationTool(c *gin.Context) {
	orgID := c.Param("orgId")
	toolID := c.Param("toolId")

	// Refresh the tool using the registry
	err := api.toolRegistry.RefreshOrganizationTool(
		c.Request.Context(),
		orgID,
		toolID,
	)
	if err != nil {
		api.logger.Error("Failed to refresh organization tool", map[string]interface{}{
			"org_id":  orgID,
			"tool_id": toolID,
			"error":   err.Error(),
		})

		if err.Error() == "tool does not belong to organization" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh tool"})
		return
	}

	// Return success with updated tool info
	c.JSON(http.StatusOK, gin.H{
		"message": "Tool refreshed successfully",
		"tool_id": toolID,
		"org_id":  orgID,
	})
}

// DeleteOrganizationTool deletes an organization tool
func (api *EnhancedToolsAPI) DeleteOrganizationTool(c *gin.Context) {
	orgID := c.Param("orgId")
	toolID := c.Param("toolId")
	tenantID := c.GetString("tenant_id")

	_ = orgID // For future org-level authorization

	// Create organization tool repository
	orgToolRepo := repository.NewOrganizationToolRepository(api.db)

	// Try to delete as organization tool first
	err := orgToolRepo.Delete(c.Request.Context(), toolID)
	if err == nil {
		c.Status(http.StatusNoContent)
		return
	}

	// If not found in organization tools, try dynamic tools
	err = api.dynamicToolsAPI.toolService.DeleteTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

// ExecuteOrganizationTool executes an action on an organization tool
func (api *EnhancedToolsAPI) ExecuteOrganizationTool(c *gin.Context) {
	orgID := c.Param("orgId")
	toolID := c.Param("toolId")
	tenantID := c.GetString("tenant_id")

	_ = orgID // For future org-level authorization

	var req models.ToolExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := api.toolRegistry.ExecuteTool(
		c.Request.Context(),
		tenantID,
		toolID,
		req.Action,
		req.Parameters,
	)
	if err != nil {
		api.logger.Error("Failed to execute tool", map[string]interface{}{
			"tool_id": toolID,
			"action":  req.Action,
			"error":   err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ListTemplates lists available tool templates
func (api *EnhancedToolsAPI) ListTemplates(c *gin.Context) {
	category := c.Query("category")

	ctx := c.Request.Context()
	var templates []*models.ToolTemplate
	var err error

	if category != "" {
		templates, err = api.templateRepo.ListByCategory(ctx, category)
	} else {
		templates, err = api.templateRepo.List(ctx)
	}

	if err != nil {
		api.logger.Error("Failed to list templates", map[string]interface{}{
			"category": category,
			"error":    err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list templates"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"count":     len(templates),
	})
}

// GetTemplate gets a specific template
func (api *EnhancedToolsAPI) GetTemplate(c *gin.Context) {
	templateID := c.Param("templateId")

	template, err := api.templateRepo.GetByID(c.Request.Context(), templateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	c.JSON(http.StatusOK, template)
}

// SearchTemplates searches for templates
func (api *EnhancedToolsAPI) SearchTemplates(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	// For now, list all and filter in memory
	// TODO: Implement proper search in repository
	templates, err := api.templateRepo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search templates"})
		return
	}

	var results []*models.ToolTemplate
	query = strings.ToLower(query)
	for _, t := range templates {
		if strings.Contains(strings.ToLower(t.DisplayName), query) ||
			strings.Contains(strings.ToLower(t.Description), query) ||
			strings.Contains(strings.ToLower(t.ProviderName), query) {
			results = append(results, t)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": results,
		"count":     len(results),
		"query":     query,
	})
}

// EnhancedDiscovery performs discovery with template detection
func (api *EnhancedToolsAPI) EnhancedDiscovery(c *gin.Context) {
	var req DiscoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")

	// Check if this is a known provider
	if api.isKnownProviderURL(req.BaseURL) {
		providerName := api.getProviderNameFromURL(req.BaseURL)

		// Get template info
		template, err := api.templateRepo.GetByProviderName(c.Request.Context(), providerName)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{
				"discovery_type": "template",
				"provider":       providerName,
				"template":       template,
				"message":        fmt.Sprintf("This is a known %s API. You can create it directly using the template.", providerName),
			})
			return
		}
	}

	// Fall back to dynamic discovery
	config := tools.ToolConfig{
		TenantID:   tenantID,
		BaseURL:    req.BaseURL,
		Credential: req.Credential,
		Config:     req.DiscoveryHints,
	}

	session, err := api.dynamicToolsAPI.toolService.StartDiscovery(c.Request.Context(), config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start discovery"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"discovery_type": "dynamic",
		"session":        session,
	})
}

// MigrateDynamicTool migrates a dynamic tool to a template-based tool
func (api *EnhancedToolsAPI) MigrateDynamicTool(c *gin.Context) {
	toolID := c.Param("toolId")
	tenantID := c.GetString("tenant_id")

	// Verify the tool exists and belongs to tenant
	tool, err := api.dynamicToolsAPI.toolService.GetTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
		return
	}

	// Attempt migration
	orgTool, err := api.toolRegistry.MigrateFromDynamic(c.Request.Context(), tool.ID)
	if err != nil {
		api.logger.Error("Failed to migrate tool", map[string]interface{}{
			"tool_id": toolID,
			"error":   err.Error(),
		})
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Migration failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Tool migrated successfully",
		"migrated_tool": orgTool,
	})
}

// Helper methods

func (api *EnhancedToolsAPI) isKnownProviderURL(url string) bool {
	knownDomains := []string{
		"github.com", "api.github.com",
		"gitlab.com", "gitlab.example.com",
		"app.harness.io", "harness.io",
		"atlassian.net", "jira.com", "confluence.com",
		"dev.azure.com", "azure.com",
		"circleci.com",
		"jenkins.io",
	}

	for _, domain := range knownDomains {
		if strings.Contains(url, domain) {
			return true
		}
	}
	return false
}

func (api *EnhancedToolsAPI) getProviderNameFromURL(url string) string {
	// Map URLs to provider names
	if strings.Contains(url, "github.com") || strings.Contains(url, "api.github.com") {
		return "github"
	}
	if strings.Contains(url, "gitlab.com") {
		return "gitlab"
	}
	if strings.Contains(url, "harness.io") {
		return "harness"
	}
	if strings.Contains(url, "atlassian.net") || strings.Contains(url, "jira.com") {
		return "jira"
	}
	if strings.Contains(url, "confluence.com") {
		return "confluence"
	}
	if strings.Contains(url, "dev.azure.com") {
		return "azure-devops"
	}
	if strings.Contains(url, "circleci.com") {
		return "circleci"
	}
	if strings.Contains(url, "jenkins.io") {
		return "jenkins"
	}
	return ""
}

func (api *EnhancedToolsAPI) getToolByID(ctx context.Context, tenantID, toolID string) (interface{}, error) {
	// Try to get from enhanced registry (handles both org tools and dynamic tools)
	tools, err := api.toolRegistry.GetToolsForTenant(ctx, tenantID)
	if err == nil {
		for _, tool := range tools {
			// Check if tool matches by ID
			if orgTool, ok := tool.(*models.OrganizationTool); ok && orgTool.ID == toolID {
				return orgTool, nil
			}
			if dynTool, ok := tool.(*models.DynamicTool); ok && dynTool.ID == toolID {
				return dynTool, nil
			}
		}
	}

	return nil, fmt.Errorf("tool not found")
}

func (api *EnhancedToolsAPI) createDynamicToolFallback(c *gin.Context, req CreateOrganizationToolRequest) {
	tenantID := c.GetString("tenant_id")

	// Convert to dynamic tool request
	toolConfig := tools.ToolConfig{
		TenantID:         tenantID,
		Name:             req.Name,
		BaseURL:          req.BaseURL,
		DocumentationURL: req.DocumentationURL,
		OpenAPIURL:       req.OpenAPIURL,
		Config:           req.Config,
		Credential:       convertToTokenCredential(req.Credentials),
	}

	tool, err := api.dynamicToolsAPI.toolService.CreateTool(c.Request.Context(), tenantID, toolConfig)
	if err != nil {
		api.logger.Error("Failed to create dynamic tool", map[string]interface{}{
			"tenant_id": tenantID,
			"name":      req.Name,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, tool)
}

func convertToTokenCredential(creds map[string]string) *models.TokenCredential {
	if creds == nil {
		return nil
	}

	// Try to determine credential type
	if token, ok := creds["token"]; ok {
		return &models.TokenCredential{
			Type:  "bearer",
			Token: token,
		}
	}
	if apiKey, ok := creds["api_key"]; ok {
		return &models.TokenCredential{
			Type:  "api_key",
			Token: apiKey,
		}
	}
	if pat, ok := creds["personal_access_token"]; ok {
		return &models.TokenCredential{
			Type:  "bearer",
			Token: pat,
		}
	}

	// Default to bearer token if we have any value
	for _, v := range creds {
		return &models.TokenCredential{
			Type:  "bearer",
			Token: v,
		}
	}

	return nil
}

// getOrganizationIDFromSlug looks up an organization's UUID by its slug
func (api *EnhancedToolsAPI) getOrganizationIDFromSlug(ctx context.Context, slug string) (string, error) {
	// Query the organizations table
	var orgID string
	query := `SELECT id FROM mcp.organizations WHERE slug = $1`

	err := api.db.QueryRowContext(ctx, query, slug).Scan(&orgID)
	if err != nil {
		return "", fmt.Errorf("organization not found: %w", err)
	}

	return orgID, nil
}

// Request types

type CreateOrganizationToolRequest struct {
	// Legacy fields for backward compatibility
	Name             string `json:"name"`
	BaseURL          string `json:"base_url"`
	DocumentationURL string `json:"documentation_url,omitempty"`
	OpenAPIURL       string `json:"openapi_url,omitempty"`

	// Standard tools fields
	TemplateID   string `json:"template_id,omitempty"`
	TemplateName string `json:"template_name,omitempty"`
	InstanceName string `json:"instance_name"`
	DisplayName  string `json:"display_name,omitempty"`
	Description  string `json:"description,omitempty"`

	// Common fields
	Config          map[string]interface{} `json:"config,omitempty"`
	InstanceConfig  map[string]interface{} `json:"instance_config,omitempty"`
	Credentials     map[string]string      `json:"credentials,omitempty"`
	EnabledFeatures map[string]interface{} `json:"enabled_features,omitempty"`
	Tags            []string               `json:"tags,omitempty"`
	AuthType        string                 `json:"auth_type,omitempty"`
}
