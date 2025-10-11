package context

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/core"
	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/repository"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// contextResponse wraps a context with HATEOAS links for API responses
type contextResponse struct {
	*models.Context
	Links map[string]string `json:"_links,omitempty"`
}

// API handles context-related API endpoints
type API struct {
	contextManager core.ContextManagerInterface
	logger         observability.Logger
	metricsClient  observability.MetricsClient
	db             *sqlx.DB
	modelRepo      repository.ModelRepository
}

// NewAPI creates a new context API handler
func NewAPI(
	contextManager core.ContextManagerInterface,
	logger observability.Logger,
	metricsClient observability.MetricsClient,
	db *sqlx.DB,
	modelRepo repository.ModelRepository,
) *API {
	if logger == nil {
		logger = observability.NewLogger("context_api")
	}

	return &API{
		contextManager: contextManager,
		logger:         logger,
		metricsClient:  metricsClient,
		db:             db,
		modelRepo:      modelRepo,
	}
}

// RegisterRoutes registers context API routes
func (api *API) RegisterRoutes(router *gin.RouterGroup) {
	contextRoutes := router.Group("/contexts")
	{
		contextRoutes.POST("", api.CreateContext)
		contextRoutes.GET("/:contextID", api.GetContext)
		contextRoutes.PUT("/:contextID", api.UpdateContext)
		contextRoutes.DELETE("/:contextID", api.DeleteContext)
		contextRoutes.GET("", api.ListContexts)
		contextRoutes.GET("/:contextID/summary", api.SummarizeContext)
		contextRoutes.POST("/:contextID/search", api.SearchInContext)
	}
}

// CreateContext godoc
// @Summary Create a new context
// @Description Create a new conversation context for an AI agent with optional metadata and initial content
// @Tags contexts
// @Accept json
// @Produce json
// @Param context body models.Context true "Context data including agent_id, session_id, content, and metadata"
// @Success 201 {object} map[string]interface{} "Created context with HATEOAS links and request tracing"
// @Failure 400 {object} map[string]interface{} "Invalid request body"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /contexts [post]
func (api *API) CreateContext(c *gin.Context) {
	var contextData models.Context

	if err := c.ShouldBindJSON(&contextData); err != nil {
		api.logger.Warn("Invalid request body for create context", map[string]any{
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Extract tenant ID directly from context (set by auth middleware)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists {
		api.logger.Warn("No tenant ID in context", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}

	// Handle both string and uuid.UUID types
	var tenantID string
	switch v := tenantIDRaw.(type) {
	case string:
		tenantID = v
	case interface{ String() string }:
		tenantID = v.String()
	default:
		api.logger.Warn("Invalid tenant_id type", map[string]interface{}{
			"type": fmt.Sprintf("%T", v),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid tenant id format"})
		return
	}

	if tenantID == "" {
		api.logger.Warn("Empty tenant ID", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}

	// Set the tenant ID in the context data
	contextData.TenantID = tenantID

	result, err := api.contextManager.CreateContext(c.Request.Context(), &contextData)
	if err != nil {
		api.logger.Error("Failed to create context", map[string]any{
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record metric
	if api.metricsClient != nil {
		api.metricsClient.RecordCounter("context_api_requests", 1, map[string]string{
			"operation": "create_context",
			"status":    "success",
		})
	}

	// Create response with HATEOAS links and request tracing
	response := &contextResponse{
		Context: result,
		Links: map[string]string{
			"self":    "/api/v1/contexts/" + result.ID,
			"summary": "/api/v1/contexts/" + result.ID + "/summary",
			"search":  "/api/v1/contexts/" + result.ID + "/search",
		},
	}

	// Include request ID for distributed tracing
	c.JSON(http.StatusCreated, gin.H{
		"data":       response,
		"request_id": c.GetString("RequestID"), // Set by middleware
		"timestamp":  time.Now().UTC(),
	})
}

// GetContext godoc
// @Summary Get a context by ID
// @Description Retrieve an existing context by its ID with optional content inclusion
// @Tags contexts
// @Accept json
// @Produce json
// @Param contextID path string true "Context ID"
// @Param include_content query boolean false "Include full content in response (default: true)"
// @Success 200 {object} map[string]interface{} "Context data with HATEOAS links"
// @Failure 400 {object} map[string]interface{} "Invalid parameters"
// @Failure 401 {object} map[string]interface{} "Unauthorized or tenant mismatch"
// @Failure 404 {object} map[string]interface{} "Context not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /contexts/{contextID} [get]
func (api *API) GetContext(c *gin.Context) {
	contextID := c.Param("contextID")

	// Extract tenant ID from the request context
	userInfo, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userMap, ok := userInfo.(map[string]any)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user context"})
		return
	}

	requestTenantID, ok := userMap["tenant_id"].(string)
	if !ok || requestTenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}

	// Read query parameters for content options
	includeContent := true
	if includeContentParam := c.Query("include_content"); includeContentParam != "" {
		var err error
		includeContent, err = strconv.ParseBool(includeContentParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid include_content parameter"})
			return
		}
	}

	result, err := api.contextManager.GetContext(c.Request.Context(), contextID)
	if err != nil {
		api.logger.Warn("Failed to get context", map[string]any{
			"error":      err.Error(),
			"context_id": sanitizeLogValue(contextID),
		})
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Validate tenant access - check if the context belongs to the requesting tenant
	// We need to fetch the model to check its tenant ID
	// We'll try to get the model with the requesting tenant ID
	model, err := api.modelRepo.GetModelByID(c.Request.Context(), requestTenantID, result.ModelID)
	if err != nil {
		api.logger.Error("Failed to fetch model for tenant validation", map[string]any{
			"error":      err.Error(),
			"model_id":   result.ModelID,
			"context_id": sanitizeLogValue(contextID),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate tenant access"})
		return
	}

	// If model is nil, it means either the model doesn't exist or doesn't belong to this tenant
	// GetModelByID already checks tenant ownership internally
	if model == nil {
		api.logger.Warn("Cross-tenant access attempt blocked", map[string]any{
			"context_id":        sanitizeLogValue(contextID),
			"request_tenant_id": requestTenantID,
			"model_id":          result.ModelID,
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Optionally remove content for lighter responses
	if !includeContent {
		result.Content = []models.ContextItem{}
	}

	// Record metric
	if api.metricsClient != nil {
		api.metricsClient.RecordCounter("context_api_requests", 1, map[string]string{
			"operation": "get_context",
			"status":    "success",
		})
	}

	// Add HATEOAS links
	response := &contextResponse{
		Context: result,
		Links: map[string]string{
			"self":    "/api/v1/contexts/" + result.ID,
			"summary": "/api/v1/contexts/" + result.ID + "/summary",
			"search":  "/api/v1/contexts/" + result.ID + "/search",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       response,
		"request_id": c.GetString("RequestID"),
		"timestamp":  time.Now().UTC(),
	})
}

// UpdateContext godoc
// @Summary Update an existing context
// @Description Update an existing context with new messages or replace content entirely
// @Tags contexts
// @Accept json
// @Produce json
// @Param contextID path string true "Context ID"
// @Param request body object true "Update request with content array and optional options"
// @Success 200 {object} map[string]interface{} "Updated context with HATEOAS links"
// @Failure 400 {object} map[string]interface{} "Invalid request body or missing content"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 404 {object} map[string]interface{} "Context not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /contexts/{contextID} [put]
func (api *API) UpdateContext(c *gin.Context) {
	contextID := c.Param("contextID")

	var updateRequest struct {
		Content []models.ContextItem         `json:"content"`
		Options *models.ContextUpdateOptions `json:"options,omitempty"`
	}

	// Bind the request body once into the typed struct
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		api.logger.Warn("Invalid request body for update context", map[string]any{
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if updateRequest.Content == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}

	result, err := api.contextManager.UpdateContext(c.Request.Context(), contextID, &models.Context{Content: updateRequest.Content}, updateRequest.Options)
	if err != nil {
		api.logger.Error("Failed to update context", map[string]any{
			"error":      err.Error(),
			"context_id": sanitizeLogValue(contextID),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Debug log the result
	api.logger.Debug("UpdateContext handler - received result", map[string]any{
		"context_id":    result.ID,
		"name":          result.Name,
		"agent_id":      result.AgentID,
		"model_id":      result.ModelID,
		"result_is_nil": result == nil,
		"has_content":   len(result.Content),
	})

	// Record metric
	if api.metricsClient != nil {
		api.metricsClient.RecordCounter("context_api_requests", 1, map[string]string{
			"operation": "update_context",
			"status":    "success",
		})
	}

	// Add HATEOAS links
	response := &contextResponse{
		Context: result,
		Links: map[string]string{
			"self":    "/api/v1/contexts/" + result.ID,
			"summary": "/api/v1/contexts/" + result.ID + "/summary",
			"search":  "/api/v1/contexts/" + result.ID + "/search",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       response,
		"request_id": c.GetString("RequestID"),
		"timestamp":  time.Now().UTC(),
	})
}

// DeleteContext godoc
// @Summary Delete a context
// @Description Delete an existing context and all associated data
// @Tags contexts
// @Accept json
// @Produce json
// @Param contextID path string true "Context ID"
// @Success 200 {object} map[string]interface{} "Deletion confirmation"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 404 {object} map[string]interface{} "Context not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /contexts/{contextID} [delete]
func (api *API) DeleteContext(c *gin.Context) {
	contextID := c.Param("contextID")

	err := api.contextManager.DeleteContext(c.Request.Context(), contextID)
	if err != nil {
		api.logger.Error("Failed to delete context", map[string]any{
			"error":      err.Error(),
			"context_id": sanitizeLogValue(contextID),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record metric
	if api.metricsClient != nil {
		api.metricsClient.RecordCounter("context_api_requests", 1, map[string]string{
			"operation": "delete_context",
			"status":    "success",
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "context deleted"})
}

// ListContexts godoc
// @Summary List contexts for an agent
// @Description List all contexts for a specific agent with optional session filtering
// @Tags contexts
// @Accept json
// @Produce json
// @Param agent_id query string true "Agent ID"
// @Param session_id query string false "Filter by session ID"
// @Param limit query integer false "Maximum number of contexts to return"
// @Success 200 {array} map[string]interface{} "List of contexts with HATEOAS links"
// @Failure 400 {object} map[string]interface{} "Invalid parameters"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /contexts [get]
func (api *API) ListContexts(c *gin.Context) {
	agentID := c.Query("agent_id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id is required"})
		return
	}

	sessionID := c.Query("session_id")

	// Parse limit from query
	limit := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit parameter"})
			return
		}
	}

	options := map[string]any{}
	if limit > 0 {
		options["limit"] = limit
	}

	result, err := api.contextManager.ListContexts(c.Request.Context(), agentID, sessionID, options)
	if err != nil {
		api.logger.Error("Failed to list contexts", map[string]any{
			"error":    err.Error(),
			"agent_id": agentID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record metric
	if api.metricsClient != nil {
		api.metricsClient.RecordCounter("context_api_requests", 1, map[string]string{
			"operation": "list_contexts",
			"status":    "success",
		})
	}

	// Create response with contexts and their links
	response := make([]map[string]any, len(result))
	for i, ctx := range result {
		links := map[string]string{
			"self":    "/api/v1/contexts/" + ctx.ID,
			"summary": "/api/v1/contexts/" + ctx.ID + "/summary",
			"search":  "/api/v1/contexts/" + ctx.ID + "/search",
		}
		response[i] = map[string]any{
			"context": ctx,
			"_links":  links,
		}
	}

	c.JSON(http.StatusOK, response)
}

// SummarizeContext godoc
// @Summary Generate a summary of a context
// @Description Generate an AI-powered summary of the conversation context
// @Tags contexts
// @Accept json
// @Produce json
// @Param contextID path string true "Context ID"
// @Success 200 {object} map[string]interface{} "Summary of the context"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 404 {object} map[string]interface{} "Context not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /contexts/{contextID}/summary [get]
func (api *API) SummarizeContext(c *gin.Context) {
	contextID := c.Param("contextID")

	result, err := api.contextManager.SummarizeContext(c.Request.Context(), contextID)
	if err != nil {
		api.logger.Error("Failed to summarize context", map[string]any{
			"error":      err.Error(),
			"context_id": sanitizeLogValue(contextID),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record metric
	if api.metricsClient != nil {
		api.metricsClient.RecordCounter("context_api_requests", 1, map[string]string{
			"operation": "summarize_context",
			"status":    "success",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"context_id": contextID,
		"summary":    result,
		"_links": map[string]string{
			"self":    "/api/v1/contexts/" + contextID,
			"context": "/api/v1/contexts/" + contextID,
		},
	})
}

// SearchInContext godoc
// @Summary Search within a context
// @Description Search for specific content within a conversation context
// @Tags contexts
// @Accept json
// @Produce json
// @Param contextID path string true "Context ID"
// @Param request body object true "Search query"
// @Success 200 {object} map[string]interface{} "Search results with matched messages"
// @Failure 400 {object} map[string]interface{} "Invalid request body"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 404 {object} map[string]interface{} "Context not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /contexts/{contextID}/search [post]
func (api *API) SearchInContext(c *gin.Context) {
	contextID := c.Param("contextID")

	var request struct {
		Query string `json:"query"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		api.logger.Warn("Invalid request body for search in context", map[string]any{
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := api.contextManager.SearchInContext(c.Request.Context(), contextID, request.Query)
	if err != nil {
		api.logger.Error("Failed to search in context", map[string]any{
			"error":      err.Error(),
			"context_id": sanitizeLogValue(contextID),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record metric
	if api.metricsClient != nil {
		api.metricsClient.RecordCounter("context_api_requests", 1, map[string]string{
			"operation": "search_in_context",
			"status":    "success",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"context_id": contextID,
		"query":      request.Query,
		"results":    result,
		"_links": map[string]string{
			"self":    "/api/v1/contexts/" + contextID + "/search",
			"context": "/api/v1/contexts/" + contextID,
		},
	})
}

// sanitizeLogValue removes newlines and carriage returns from user input to prevent log injection
func sanitizeLogValue(input string) string {
	// Remove newlines, carriage returns, and other control characters
	sanitized := strings.ReplaceAll(input, "\n", "\\n")
	sanitized = strings.ReplaceAll(sanitized, "\r", "\\r")
	sanitized = strings.ReplaceAll(sanitized, "\t", "\\t")
	// Limit length to prevent excessive log sizes
	if len(sanitized) > 100 {
		sanitized = sanitized[:100] + "..."
	}
	return sanitized
}
