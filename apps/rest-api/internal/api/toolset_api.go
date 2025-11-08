package api

import (
	"net/http"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/services"
	"github.com/developer-mesh/developer-mesh/pkg/mcptools"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
)

// ToolsetAPI handles toolset-related endpoints
type ToolsetAPI struct {
	toolsetService *services.ToolsetService
	logger         observability.Logger
}

// NewToolsetAPI creates a new toolset API handler
func NewToolsetAPI(
	toolsetService *services.ToolsetService,
	logger observability.Logger,
) *ToolsetAPI {
	return &ToolsetAPI{
		toolsetService: toolsetService,
		logger:         logger,
	}
}

// RegisterRoutes registers toolset API routes
func (api *ToolsetAPI) RegisterRoutes(router *gin.RouterGroup) {
	toolsets := router.Group("/toolsets")
	{
		toolsets.GET("", api.ListToolsets)
		toolsets.GET("/:name/tools", api.GetToolsetTools)
	}
}

// ListToolsetsRequest defines the query parameters for listing toolsets
type ListToolsetsRequest struct {
	Provider string `form:"provider"`
}

// ListToolsetsResponse defines the response format for toolset list
type ListToolsetsResponse struct {
	Toolsets []ToolsetSummary `json:"toolsets"`
	Total    int              `json:"total"`
}

// ToolsetSummary provides a summary view of a toolset
type ToolsetSummary struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Provider    string   `json:"provider"`
	ToolCount   int      `json:"tool_count"`
	Tools       []string `json:"tools"`
	Tags        []string `json:"tags"`
	Enabled     bool     `json:"enabled"`
}

// ListToolsets handles GET /api/v1/toolsets
func (api *ToolsetAPI) ListToolsets(c *gin.Context) {
	// Get tenant from context (set by auth middleware)
	tenantID, exists := c.Get("tenant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant_id not found in context"})
		return
	}

	var req ListToolsetsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Auto-generate toolsets from dynamic tools
	toolsetInfos, err := api.toolsetService.AutoGenerateToolsets(c.Request.Context(), tenantID.(string))
	if err != nil {
		api.logger.Error("Failed to generate toolsets", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate toolsets"})
		return
	}

	// Filter by provider if requested
	var filteredToolsets []services.ToolsetInfo
	if req.Provider != "" {
		for _, info := range toolsetInfos {
			if info.Toolset.Provider == req.Provider {
				filteredToolsets = append(filteredToolsets, info)
			}
		}
	} else {
		filteredToolsets = toolsetInfos
	}

	// Convert to summary format
	summaries := make([]ToolsetSummary, len(filteredToolsets))
	for i, info := range filteredToolsets {
		summaries[i] = ToolsetSummary{
			Name:        info.Toolset.Name,
			DisplayName: info.Toolset.DisplayName,
			Description: info.Toolset.Description,
			Category:    info.Toolset.Category,
			Provider:    info.Toolset.Provider,
			ToolCount:   len(info.Tools),
			Tools:       info.Toolset.Tools,
			Tags:        info.Toolset.Tags,
			Enabled:     info.Toolset.Enabled,
		}
	}

	c.JSON(http.StatusOK, ListToolsetsResponse{
		Toolsets: summaries,
		Total:    len(summaries),
	})
}

// GetToolsetToolsRequest defines the query parameters
type GetToolsetToolsRequest struct {
	DetailLevel string `form:"detail_level" binding:"omitempty,oneof=name description full"`
}

// GetToolsetToolsResponse defines the response format
type GetToolsetToolsResponse struct {
	Toolset     string                   `json:"toolset"`
	DetailLevel string                   `json:"detail_level"`
	Tools       []map[string]interface{} `json:"tools"`
}

// GetToolsetTools handles GET /api/v1/toolsets/:name/tools
func (api *ToolsetAPI) GetToolsetTools(c *gin.Context) {
	// Get tenant from context
	tenantID, exists := c.Get("tenant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant_id not found in context"})
		return
	}

	toolsetName := c.Param("name")
	if toolsetName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "toolset name is required"})
		return
	}

	var req GetToolsetToolsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default to full detail level
	if req.DetailLevel == "" {
		req.DetailLevel = "full"
	}

	// Generate toolsets
	toolsetInfos, err := api.toolsetService.AutoGenerateToolsets(c.Request.Context(), tenantID.(string))
	if err != nil {
		api.logger.Error("Failed to generate toolsets", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate toolsets"})
		return
	}

	// Find the requested toolset
	var toolsetInfo *services.ToolsetInfo
	for i := range toolsetInfos {
		if toolsetInfos[i].Toolset.Name == toolsetName {
			toolsetInfo = &toolsetInfos[i]
			break
		}
	}

	if toolsetInfo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "toolset not found"})
		return
	}

	// Format tools based on detail level
	tools := make([]map[string]interface{}, len(toolsetInfo.Tools))
	detailLevel := mcptools.DetailLevel(req.DetailLevel)

	for i, tool := range toolsetInfo.Tools {
		switch detailLevel {
		case mcptools.DetailLevelName:
			tools[i] = map[string]interface{}{
				"name": tool.ToolName,
			}
		case mcptools.DetailLevelDescription:
			tools[i] = map[string]interface{}{
				"name":         tool.ToolName,
				"display_name": tool.DisplayName,
				"description":  api.generateToolDescription(tool),
			}
		case mcptools.DetailLevelFull:
			tools[i] = map[string]interface{}{
				"name":         tool.ToolName,
				"display_name": tool.DisplayName,
				"description":  api.generateToolDescription(tool),
				"input_schema": tool.InputSchema,
				"auth_type":    tool.AuthType,
				"base_url":     tool.BaseURL,
			}
		}
	}

	c.JSON(http.StatusOK, GetToolsetToolsResponse{
		Toolset:     toolsetName,
		DetailLevel: req.DetailLevel,
		Tools:       tools,
	})
}

// generateToolDescription creates a description for a tool
func (api *ToolsetAPI) generateToolDescription(tool *models.DynamicTool) string {
	if tool.DisplayName != "" {
		return tool.DisplayName
	}
	return tool.ToolName
}
