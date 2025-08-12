package prompts

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Service defines the interface for prompt operations
type Service interface {
	ListPrompts(tenantID string, category string, tags []string) ([]*models.Prompt, error)
	GetPrompt(tenantID, name string) (*models.Prompt, error)
	CreatePrompt(tenantID string, req *models.PromptCreateRequest) (*models.Prompt, error)
	UpdatePrompt(tenantID, name string, req *models.PromptUpdateRequest) (*models.Prompt, error)
	DeletePrompt(tenantID, name string) error
	RenderPrompt(tenantID, name string, arguments map[string]interface{}) (*models.PromptRenderResponse, error)
}

// API handles prompt-related HTTP endpoints
type API struct {
	service Service
	logger  observability.Logger
}

// NewAPI creates a new prompt API handler
func NewAPI(service Service, logger observability.Logger) *API {
	return &API{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes registers all prompt-related routes
func (api *API) RegisterRoutes(router *gin.RouterGroup) {
	prompts := router.Group("/prompts")
	{
		prompts.GET("", api.ListPrompts)
		prompts.GET("/:name", api.GetPrompt)
		prompts.POST("", api.CreatePrompt)
		prompts.PUT("/:name", api.UpdatePrompt)
		prompts.DELETE("/:name", api.DeletePrompt)
		prompts.POST("/:name/render", api.RenderPrompt)
	}
}

// ListPrompts handles GET /api/v1/prompts
func (api *API) ListPrompts(c *gin.Context) {
	tenantID := auth.GetTenantID(c.Request.Context())
	if tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	// Parse query parameters
	category := c.Query("category")
	tags := c.QueryArray("tags")

	prompts, err := api.service.ListPrompts(tenantID.String(), category, tags)
	if err != nil {
		api.logger.Error("Failed to list prompts", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list prompts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"prompts": prompts})
}

// GetPrompt handles GET /api/v1/prompts/:name
func (api *API) GetPrompt(c *gin.Context) {
	tenantID := auth.GetTenantID(c.Request.Context())
	if tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	prompt, err := api.service.GetPrompt(tenantID.String(), name)
	if err != nil {
		api.logger.Error("Failed to get prompt", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
			"name":      name,
		})
		c.JSON(http.StatusNotFound, gin.H{"error": "Prompt not found"})
		return
	}

	c.JSON(http.StatusOK, prompt)
}

// CreatePrompt handles POST /api/v1/prompts
func (api *API) CreatePrompt(c *gin.Context) {
	tenantID := auth.GetTenantID(c.Request.Context())
	if tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	var req models.PromptCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate required fields
	if req.Name == "" || req.Template == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and template are required"})
		return
	}

	prompt, err := api.service.CreatePrompt(tenantID.String(), &req)
	if err != nil {
		api.logger.Error("Failed to create prompt", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
			"name":      req.Name,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create prompt"})
		return
	}

	c.JSON(http.StatusCreated, prompt)
}

// UpdatePrompt handles PUT /api/v1/prompts/:name
func (api *API) UpdatePrompt(c *gin.Context) {
	tenantID := auth.GetTenantID(c.Request.Context())
	if tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	var req models.PromptUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	prompt, err := api.service.UpdatePrompt(tenantID.String(), name, &req)
	if err != nil {
		api.logger.Error("Failed to update prompt", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
			"name":      name,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update prompt"})
		return
	}

	c.JSON(http.StatusOK, prompt)
}

// DeletePrompt handles DELETE /api/v1/prompts/:name
func (api *API) DeletePrompt(c *gin.Context) {
	tenantID := auth.GetTenantID(c.Request.Context())
	if tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	err := api.service.DeletePrompt(tenantID.String(), name)
	if err != nil {
		api.logger.Error("Failed to delete prompt", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
			"name":      name,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete prompt"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// RenderPrompt handles POST /api/v1/prompts/:name/render
func (api *API) RenderPrompt(c *gin.Context) {
	tenantID := auth.GetTenantID(c.Request.Context())
	if tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	var req struct {
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// For now, do a simple template rendering
	// In production, this would be handled by the service layer
	prompt, err := api.service.GetPrompt(tenantID.String(), name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Prompt not found"})
		return
	}

	// Simple template variable replacement
	rendered := prompt.Template
	for key, value := range req.Arguments {
		placeholder := fmt.Sprintf("{{%s}}", key)
		rendered = strings.ReplaceAll(rendered, placeholder, fmt.Sprintf("%v", value))
	}

	response := &models.PromptRenderResponse{
		Messages: []models.PromptMessage{
			{
				Role:    "user",
				Content: rendered,
			},
		},
	}

	c.JSON(http.StatusOK, response)
}
