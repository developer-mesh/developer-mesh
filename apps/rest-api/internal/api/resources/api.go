package resources

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Service defines the interface for resource operations
type Service interface {
	ListResources(tenantID string, filter map[string]interface{}) ([]*models.Resource, error)
	GetResource(tenantID, uri string) (*models.Resource, error)
	CreateResource(tenantID string, req *models.ResourceCreateRequest) (*models.Resource, error)
	UpdateResource(tenantID, uri string, req *models.ResourceUpdateRequest) (*models.Resource, error)
	DeleteResource(tenantID, uri string) error
	Subscribe(tenantID, resourceID, agentID string, events []string) (*models.ResourceSubscription, error)
	Unsubscribe(tenantID, subscriptionID string) error
}

// API handles resource-related HTTP endpoints
type API struct {
	service Service
	logger  observability.Logger
}

// NewAPI creates a new resource API handler
func NewAPI(service Service, logger observability.Logger) *API {
	return &API{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes registers all resource-related routes
func (api *API) RegisterRoutes(router *gin.RouterGroup) {
	resources := router.Group("/resources")
	{
		resources.GET("", api.ListResources)
		resources.GET("/:uri", api.GetResource)
		resources.POST("", api.CreateResource)
		resources.PUT("/:uri", api.UpdateResource)
		resources.DELETE("/:uri", api.DeleteResource)
		resources.POST("/subscribe", api.Subscribe)
		resources.DELETE("/subscriptions/:id", api.Unsubscribe)
	}
}

// ListResources handles GET /api/v1/resources
func (api *API) ListResources(c *gin.Context) {
	tenantID := auth.GetTenantID(c.Request.Context())
	if tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	// Parse filter from query params
	filter := make(map[string]interface{})
	if tags := c.QueryArray("tags"); len(tags) > 0 {
		filter["tags"] = tags
	}
	if mimeType := c.Query("mime_type"); mimeType != "" {
		filter["mime_type"] = mimeType
	}

	resources, err := api.service.ListResources(tenantID.String(), filter)
	if err != nil {
		api.logger.Error("Failed to list resources", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list resources"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"resources": resources})
}

// GetResource handles GET /api/v1/resources/:uri
func (api *API) GetResource(c *gin.Context) {
	tenantID := auth.GetTenantID(c.Request.Context())
	if tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	uri := c.Param("uri")
	if uri == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uri is required"})
		return
	}

	resource, err := api.service.GetResource(tenantID.String(), uri)
	if err != nil {
		api.logger.Error("Failed to get resource", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
			"uri":       uri,
		})
		c.JSON(http.StatusNotFound, gin.H{"error": "Resource not found"})
		return
	}

	// Return resource content in MCP format
	c.JSON(http.StatusOK, models.ResourceContent{
		URI:      resource.URI,
		MimeType: resource.MimeType,
		Content:  resource.Content,
	})
}

// CreateResource handles POST /api/v1/resources
func (api *API) CreateResource(c *gin.Context) {
	tenantID := auth.GetTenantID(c.Request.Context())
	if tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	var req models.ResourceCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate required fields
	if req.URI == "" || req.Name == "" || req.MimeType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uri, name, and mimeType are required"})
		return
	}

	resource, err := api.service.CreateResource(tenantID.String(), &req)
	if err != nil {
		api.logger.Error("Failed to create resource", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
			"uri":       req.URI,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create resource"})
		return
	}

	c.JSON(http.StatusCreated, resource)
}

// UpdateResource handles PUT /api/v1/resources/:uri
func (api *API) UpdateResource(c *gin.Context) {
	tenantID := auth.GetTenantID(c.Request.Context())
	if tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	uri := c.Param("uri")
	if uri == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uri is required"})
		return
	}

	var req models.ResourceUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resource, err := api.service.UpdateResource(tenantID.String(), uri, &req)
	if err != nil {
		api.logger.Error("Failed to update resource", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
			"uri":       uri,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update resource"})
		return
	}

	c.JSON(http.StatusOK, resource)
}

// DeleteResource handles DELETE /api/v1/resources/:uri
func (api *API) DeleteResource(c *gin.Context) {
	tenantID := auth.GetTenantID(c.Request.Context())
	if tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	uri := c.Param("uri")
	if uri == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uri is required"})
		return
	}

	err := api.service.DeleteResource(tenantID.String(), uri)
	if err != nil {
		api.logger.Error("Failed to delete resource", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
			"uri":       uri,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete resource"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// Subscribe handles POST /api/v1/resources/subscribe
func (api *API) Subscribe(c *gin.Context) {
	tenantID := auth.GetTenantID(c.Request.Context())
	if tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	var req struct {
		ResourceID string   `json:"resource_id"`
		AgentID    string   `json:"agent_id"`
		Events     []string `json:"events"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	subscription, err := api.service.Subscribe(tenantID.String(), req.ResourceID, req.AgentID, req.Events)
	if err != nil {
		api.logger.Error("Failed to subscribe to resource", map[string]interface{}{
			"error":       err.Error(),
			"tenant_id":   tenantID.String(),
			"resource_id": req.ResourceID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to subscribe"})
		return
	}

	c.JSON(http.StatusCreated, subscription)
}

// Unsubscribe handles DELETE /api/v1/resources/subscriptions/:id
func (api *API) Unsubscribe(c *gin.Context) {
	tenantID := auth.GetTenantID(c.Request.Context())
	if tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	subscriptionID := c.Param("id")
	if subscriptionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subscription id is required"})
		return
	}

	err := api.service.Unsubscribe(tenantID.String(), subscriptionID)
	if err != nil {
		api.logger.Error("Failed to unsubscribe from resource", map[string]interface{}{
			"error":           err.Error(),
			"tenant_id":       tenantID.String(),
			"subscription_id": subscriptionID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unsubscribe"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
