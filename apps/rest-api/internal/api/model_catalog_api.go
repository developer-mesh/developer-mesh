package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/services"
	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/middleware"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// ModelCatalogAPI handles model catalog and tenant model management endpoints
type ModelCatalogAPI struct {
	modelService   services.ModelManagementService
	logger         observability.Logger
	authMiddleware *auth.Service
	rateLimiter    *middleware.RateLimiter
}

// NewModelCatalogAPI creates a new ModelCatalogAPI instance
func NewModelCatalogAPI(
	modelService services.ModelManagementService,
	authMiddleware *auth.Service,
	metrics observability.MetricsClient,
) *ModelCatalogAPI {
	logger := observability.NewStandardLogger("model_catalog_api")

	// Create rate limiter with custom config for model operations
	rateLimitConfig := middleware.RateLimitConfig{
		GlobalRPS:       500,  // 500 requests per second globally
		GlobalBurst:     1000, // Allow burst of 1000
		TenantRPS:       50,   // 50 requests per second per tenant
		TenantBurst:     100,  // Allow burst of 100 per tenant
		ToolRPS:         20,   // 20 requests per second per tool/endpoint
		ToolBurst:       40,   // Allow burst of 40 per endpoint
		CleanupInterval: 5 * time.Minute,
		MaxAge:          1 * time.Hour,
	}

	return &ModelCatalogAPI{
		modelService:   modelService,
		logger:         logger,
		authMiddleware: authMiddleware,
		rateLimiter:    middleware.NewRateLimiter(rateLimitConfig, logger, metrics),
	}
}

// RegisterRoutes registers the model catalog routes with authentication and rate limiting
func (api *ModelCatalogAPI) RegisterRoutes(router *gin.RouterGroup) {
	// Apply authentication middleware to all routes
	authRequired := api.authMiddleware.GinMiddleware(auth.TypeAPIKey)

	// Apply global rate limiting
	globalRateLimit := api.rateLimiter.GlobalLimit()

	// Apply tenant-aware rate limiting
	tenantRateLimit := api.rateLimiter.TenantLimit()

	// Model catalog endpoints (admin operations)
	catalog := router.Group("/embedding-models")
	catalog.Use(globalRateLimit, authRequired, tenantRateLimit)
	{
		// Read operations - higher rate limits
		catalog.GET("/catalog", api.ListCatalog)
		catalog.GET("/catalog/:id", api.GetModel)
		catalog.GET("/providers", api.ListProviders)

		// Write operations - apply adaptive rate limiting for safety
		adaptiveLimit := api.rateLimiter.AdaptiveLimit()
		catalog.POST("/catalog", adaptiveLimit, api.CreateModel)
		catalog.PUT("/catalog/:id", adaptiveLimit, api.UpdateModel)
		catalog.DELETE("/catalog/:id", adaptiveLimit, api.DeleteModel)
	}

	// Tenant model management (tenant-specific operations)
	tenant := router.Group("/tenant-models")
	tenant.Use(globalRateLimit, authRequired, tenantRateLimit)
	{
		// Read operations
		tenant.GET("", api.ListTenantModels)
		tenant.GET("/usage", api.GetUsageStats)
		tenant.GET("/quotas", api.GetQuotas)

		// Write operations with adaptive rate limiting
		adaptiveLimit := api.rateLimiter.AdaptiveLimit()
		tenant.POST("", adaptiveLimit, api.ConfigureTenantModel)
		tenant.PUT("/:model_id", adaptiveLimit, api.UpdateTenantModel)
		tenant.DELETE("/:model_id", adaptiveLimit, api.RemoveTenantModel)
		tenant.POST("/:model_id/set-default", adaptiveLimit, api.SetDefaultModel)
	}

	// Model selection endpoint (critical path - needs all rate limits)
	router.POST("/embedding-models/select",
		globalRateLimit,
		authRequired,
		tenantRateLimit,
		api.SelectModel)
}

// extractTenantID extracts the tenant ID from the authenticated user context
func (api *ModelCatalogAPI) extractTenantID(c *gin.Context) (uuid.UUID, error) {
	// Try to get from user context set by auth middleware
	user, exists := c.Get("user")
	if !exists {
		// Fallback to direct tenant_id in context (for backward compatibility)
		tenantIDStr := c.GetString("tenant_id")
		if tenantIDStr == "" {
			return uuid.Nil, fmt.Errorf("tenant ID not found in context")
		}
		return uuid.Parse(tenantIDStr)
	}

	userMap, ok := user.(map[string]interface{})
	if !ok {
		return uuid.Nil, fmt.Errorf("invalid user context format")
	}

	tenantIDStr, ok := userMap["tenant_id"].(string)
	if !ok || tenantIDStr == "" {
		return uuid.Nil, fmt.Errorf("tenant ID not found in user context")
	}

	return uuid.Parse(tenantIDStr)
}

// ListCatalog handles GET /api/v1/embedding-models/catalog
func (api *ModelCatalogAPI) ListCatalog(c *gin.Context) {
	// Extract query parameters for filtering
	provider := c.Query("provider")
	modelType := c.Query("type")
	availableOnly := c.DefaultQuery("available_only", "true") == "true"

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	// Create filter
	filter := &services.ModelFilter{
		AvailableOnly: availableOnly,
		Offset:        offset,
		Limit:         limit,
	}
	if provider != "" {
		filter.Provider = &provider
	}
	if modelType != "" {
		filter.ModelType = &modelType
	}

	models, total, err := api.modelService.ListModels(c.Request.Context(), filter)
	if err != nil {
		api.logger.Error("Failed to list models", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve model catalog",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"models": models,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
			"pages": (total + limit - 1) / limit,
		},
	})
}

// GetModel handles GET /api/v1/embedding-models/catalog/:id
func (api *ModelCatalogAPI) GetModel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid model ID format",
		})
		return
	}

	model, err := api.modelService.GetModel(c.Request.Context(), id)
	if err != nil {
		api.logger.Error("Failed to get model", map[string]interface{}{
			"model_id": id,
			"error":    err.Error(),
		})
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Model not found",
		})
		return
	}

	c.JSON(http.StatusOK, model)
}

// CreateModel handles POST /api/v1/embedding-models/catalog
func (api *ModelCatalogAPI) CreateModel(c *gin.Context) {
	var req services.CreateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Validate required fields
	if req.Provider == "" || req.ModelName == "" || req.ModelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Provider, model_name, and model_id are required",
		})
		return
	}

	if req.Dimensions <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Dimensions must be positive",
		})
		return
	}

	model, err := api.modelService.CreateModel(c.Request.Context(), &req)
	if err != nil {
		api.logger.Error("Failed to create model", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create model",
		})
		return
	}

	c.JSON(http.StatusCreated, model)
}

// UpdateModel handles PUT /api/v1/embedding-models/catalog/:id
func (api *ModelCatalogAPI) UpdateModel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid model ID format",
		})
		return
	}

	var req services.UpdateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	model, err := api.modelService.UpdateModel(c.Request.Context(), id, &req)
	if err != nil {
		api.logger.Error("Failed to update model", map[string]interface{}{
			"model_id": id,
			"error":    err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update model",
		})
		return
	}

	c.JSON(http.StatusOK, model)
}

// DeleteModel handles DELETE /api/v1/embedding-models/catalog/:id
func (api *ModelCatalogAPI) DeleteModel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid model ID format",
		})
		return
	}

	err = api.modelService.DeleteModel(c.Request.Context(), id)
	if err != nil {
		api.logger.Error("Failed to delete model", map[string]interface{}{
			"model_id": id,
			"error":    err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete model",
		})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// ListProviders handles GET /api/v1/embedding-models/providers
func (api *ModelCatalogAPI) ListProviders(c *gin.Context) {
	providers, err := api.modelService.ListProviders(c.Request.Context())
	if err != nil {
		api.logger.Error("Failed to list providers", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve providers",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"providers": providers,
	})
}

// ListTenantModels handles GET /api/v1/tenant-models
func (api *ModelCatalogAPI) ListTenantModels(c *gin.Context) {
	// Extract tenant ID from authenticated context
	tenantID, err := api.extractTenantID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": err.Error(),
		})
		return
	}

	enabledOnly := c.DefaultQuery("enabled_only", "false") == "true"

	models, err := api.modelService.ListTenantModels(c.Request.Context(), tenantID, enabledOnly)
	if err != nil {
		api.logger.Error("Failed to list tenant models", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve tenant models",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"models": models,
	})
}

// ConfigureTenantModel handles POST /api/v1/tenant-models
func (api *ModelCatalogAPI) ConfigureTenantModel(c *gin.Context) {
	tenantIDStr := c.GetString("tenant_id")
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Tenant ID not found in context",
		})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid tenant ID format",
		})
		return
	}

	var req services.ConfigureTenantModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Validate model ID
	if req.ModelID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Model ID is required",
		})
		return
	}

	result, err := api.modelService.ConfigureTenantModel(c.Request.Context(), tenantID, &req)
	if err != nil {
		api.logger.Error("Failed to configure tenant model", map[string]interface{}{
			"tenant_id": tenantID,
			"model_id":  req.ModelID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to configure tenant model",
		})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// UpdateTenantModel handles PUT /api/v1/tenant-models/:model_id
func (api *ModelCatalogAPI) UpdateTenantModel(c *gin.Context) {
	tenantIDStr := c.GetString("tenant_id")
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Tenant ID not found in context",
		})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid tenant ID format",
		})
		return
	}

	modelIDStr := c.Param("model_id")
	modelID, err := uuid.Parse(modelIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid model ID format",
		})
		return
	}

	var req services.UpdateTenantModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	result, err := api.modelService.UpdateTenantModel(c.Request.Context(), tenantID, modelID, &req)
	if err != nil {
		api.logger.Error("Failed to update tenant model", map[string]interface{}{
			"tenant_id": tenantID,
			"model_id":  modelID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update tenant model",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// RemoveTenantModel handles DELETE /api/v1/tenant-models/:model_id
func (api *ModelCatalogAPI) RemoveTenantModel(c *gin.Context) {
	tenantIDStr := c.GetString("tenant_id")
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Tenant ID not found in context",
		})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid tenant ID format",
		})
		return
	}

	modelIDStr := c.Param("model_id")
	modelID, err := uuid.Parse(modelIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid model ID format",
		})
		return
	}

	err = api.modelService.RemoveTenantModel(c.Request.Context(), tenantID, modelID)
	if err != nil {
		api.logger.Error("Failed to remove tenant model", map[string]interface{}{
			"tenant_id": tenantID,
			"model_id":  modelID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to remove tenant model",
		})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// SetDefaultModel handles POST /api/v1/tenant-models/:model_id/set-default
func (api *ModelCatalogAPI) SetDefaultModel(c *gin.Context) {
	tenantIDStr := c.GetString("tenant_id")
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Tenant ID not found in context",
		})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid tenant ID format",
		})
		return
	}

	modelIDStr := c.Param("model_id")
	modelID, err := uuid.Parse(modelIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid model ID format",
		})
		return
	}

	err = api.modelService.SetDefaultModel(c.Request.Context(), tenantID, modelID)
	if err != nil {
		api.logger.Error("Failed to set default model", map[string]interface{}{
			"tenant_id": tenantID,
			"model_id":  modelID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to set default model",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Default model updated successfully",
	})
}

// GetUsageStats handles GET /api/v1/tenant-models/usage
func (api *ModelCatalogAPI) GetUsageStats(c *gin.Context) {
	tenantIDStr := c.GetString("tenant_id")
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Tenant ID not found in context",
		})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid tenant ID format",
		})
		return
	}

	// Optional model ID filter
	modelIDStr := c.Query("model_id")
	var modelID *uuid.UUID
	if modelIDStr != "" {
		id, err := uuid.Parse(modelIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid model ID format",
			})
			return
		}
		modelID = &id
	}

	period := c.DefaultQuery("period", "month")

	stats, err := api.modelService.GetUsageStats(c.Request.Context(), tenantID, modelID, period)
	if err != nil {
		api.logger.Error("Failed to get usage stats", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve usage statistics",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetQuotas handles GET /api/v1/tenant-models/quotas
func (api *ModelCatalogAPI) GetQuotas(c *gin.Context) {
	tenantIDStr := c.GetString("tenant_id")
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Tenant ID not found in context",
		})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid tenant ID format",
		})
		return
	}

	quotas, err := api.modelService.GetTenantQuotas(c.Request.Context(), tenantID)
	if err != nil {
		api.logger.Error("Failed to get quotas", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve quotas",
		})
		return
	}

	c.JSON(http.StatusOK, quotas)
}

// SelectModel handles POST /api/v1/embedding-models/select
func (api *ModelCatalogAPI) SelectModel(c *gin.Context) {
	tenantIDStr := c.GetString("tenant_id")
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Tenant ID not found in context",
		})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid tenant ID format",
		})
		return
	}

	var req services.ModelSelectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	req.TenantID = tenantID

	result, err := api.modelService.SelectModelForRequest(c.Request.Context(), &req)
	if err != nil {
		api.logger.Error("Failed to select model", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to select model",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}
