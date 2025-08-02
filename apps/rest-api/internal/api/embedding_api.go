package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/agents"
	"github.com/developer-mesh/developer-mesh/pkg/common/util"
	"github.com/developer-mesh/developer-mesh/pkg/embedding"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// EmbeddingAPI handles all embedding-related endpoints
type EmbeddingAPI struct {
	embeddingService *embedding.ServiceV2
	agentService     *agents.Service
	logger           observability.Logger
}

// NewEmbeddingAPI creates a new embedding API handler
func NewEmbeddingAPI(embeddingService *embedding.ServiceV2, agentService *agents.Service, logger observability.Logger) *EmbeddingAPI {
	return &EmbeddingAPI{
		embeddingService: embeddingService,
		agentService:     agentService,
		logger:           logger,
	}
}

// RegisterRoutes registers all embedding routes
func (api *EmbeddingAPI) RegisterRoutes(router *gin.RouterGroup) {
	embeddings := router.Group("/embeddings")
	{
		// Core embedding operations
		embeddings.POST("", api.generateEmbedding)
		embeddings.POST("/batch", api.batchGenerateEmbeddings)
		embeddings.POST("/search", api.searchEmbeddings)
		embeddings.POST("/search/cross-model", api.crossModelSearch)

		// Provider management
		embeddings.GET("/providers/health", api.getProviderHealth)

		// Agent configuration
		agentsGroup := embeddings.Group("/agents")
		{
			agentsGroup.POST("", api.createAgentConfig)
			agentsGroup.GET("/:agentId", api.getAgentConfig)
			agentsGroup.PUT("/:agentId", api.updateAgentConfig)
			agentsGroup.GET("/:agentId/models", api.getAgentModels)
			agentsGroup.GET("/:agentId/costs", api.getAgentCosts)
		}
	}
}

// generateEmbedding godoc
// @Summary Generate an embedding
// @Description Generate a vector embedding for the provided text using agent-specific model selection
// @Tags embeddings
// @Accept json
// @Produce json
// @Param request body embedding.GenerateEmbeddingRequest true "Embedding request with text, agent_id, and optional parameters"
// @Success 200 {object} embedding.GenerateEmbeddingResponse "Generated embedding with metadata"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /embeddings [post]
func (api *EmbeddingAPI) generateEmbedding(c *gin.Context) {
	var req embedding.GenerateEmbeddingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrBadRequest,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	// Inject tenant ID from auth context
	if tenantID, err := util.GetTenantIDFromGinContext(c); err == nil {
		req.TenantID = tenantID
	}

	// Default task type if not specified
	if req.TaskType == "" {
		req.TaskType = agents.TaskTypeGeneralQA
	}

	resp, err := api.embeddingService.GenerateEmbedding(c.Request.Context(), req)
	if err != nil {
		api.logger.Error("Failed to generate embedding", map[string]any{
			"error":       err.Error(),
			"agent_id":    sanitizeLogValue(req.AgentID),
			"text_length": len(req.Text),
		})

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrInternalServer,
			Message: "Failed to generate embedding",
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// batchGenerateEmbeddings godoc
// @Summary Generate embeddings in batch
// @Description Generate multiple embeddings in a single request for efficiency
// @Tags embeddings
// @Accept json
// @Produce json
// @Param requests body []embedding.GenerateEmbeddingRequest true "Array of embedding requests"
// @Success 200 {object} map[string]interface{} "Batch generation results"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /embeddings/batch [post]
func (api *EmbeddingAPI) batchGenerateEmbeddings(c *gin.Context) {
	var reqs []embedding.GenerateEmbeddingRequest
	if err := c.ShouldBindJSON(&reqs); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrBadRequest,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	// Inject tenant ID for all requests
	if tenantID, err := util.GetTenantIDFromGinContext(c); err == nil {
		for i := range reqs {
			reqs[i].TenantID = tenantID
			if reqs[i].TaskType == "" {
				reqs[i].TaskType = agents.TaskTypeGeneralQA
			}
		}
	}

	resps, err := api.embeddingService.BatchGenerateEmbeddings(c.Request.Context(), reqs)
	if err != nil {
		api.logger.Error("Failed to batch generate embeddings", map[string]any{
			"error": err.Error(),
			"count": len(reqs),
		})
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrInternalServer,
			Message: "Failed to generate embeddings",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"embeddings": resps,
		"count":      len(resps),
	})
}

// searchEmbeddings godoc
// @Summary Search embeddings
// @Description Perform semantic search using vector similarity
// @Tags embeddings
// @Accept json
// @Produce json
// @Param request body embedding.SearchRequest true "Search query with text and optional filters"
// @Success 200 {object} embedding.SearchResponse "Search results with similarity scores"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /embeddings/search [post]
func (api *EmbeddingAPI) searchEmbeddings(c *gin.Context) {
	var req embedding.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrBadRequest,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	// Create search service adapter
	searchService := NewSearchServiceAdapter(api.embeddingService, api.logger)

	// Inject tenant ID from auth context if not provided
	if req.TenantID == uuid.Nil {
		if tenantID, err := util.GetTenantIDFromGinContext(c); err == nil {
			req.TenantID = tenantID
		}
	}

	// Perform search
	results, err := searchService.Search(c.Request.Context(), req)
	if err != nil {
		api.logger.Error("Search failed", map[string]any{
			"error":     err.Error(),
			"tenant_id": req.TenantID,
		})
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrInternalServer,
			Message: "Search operation failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"count":   len(results),
	})
}

// crossModelSearch godoc
// @Summary Cross-model semantic search
// @Description Search across embeddings from different models using unified similarity
// @Tags embeddings
// @Accept json
// @Produce json
// @Param request body embedding.CrossModelSearchRequest true "Cross-model search request"
// @Success 200 {object} map[string]interface{} "Cross-model search results"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /embeddings/search/cross-model [post]
func (api *EmbeddingAPI) crossModelSearch(c *gin.Context) {
	var req embedding.CrossModelSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrBadRequest,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	// Create search service adapter
	searchService := NewSearchServiceAdapter(api.embeddingService, api.logger)

	// Convert to standard search request for adapter
	searchReq := embedding.SearchRequest{
		QueryEmbedding: req.QueryEmbedding,
		ModelName:      req.SearchModel,
		TenantID:       req.TenantID,
		ContextID:      req.ContextID,
		Limit:          req.Limit,
		Threshold:      float64(req.MinSimilarity),
	}

	// Add metadata filter if provided
	if len(req.MetadataFilter) > 0 {
		filterJSON, err := json.Marshal(req.MetadataFilter)
		if err == nil {
			searchReq.MetadataFilter = filterJSON
		}
	}

	// Perform search
	results, err := searchService.Search(c.Request.Context(), searchReq)
	if err != nil {
		api.logger.Error("Cross-model search failed", map[string]any{
			"error":     err.Error(),
			"tenant_id": req.TenantID,
		})
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrInternalServer,
			Message: "Cross-model search operation failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results":      results,
		"count":        len(results),
		"search_model": req.SearchModel,
	})
}

// getProviderHealth godoc
// @Summary Get embedding provider health
// @Description Check the health status of all configured embedding providers
// @Tags embeddings
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Provider health status"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /embeddings/providers/health [get]
func (api *EmbeddingAPI) getProviderHealth(c *gin.Context) {
	health := api.embeddingService.GetProviderHealth(c.Request.Context())

	c.JSON(http.StatusOK, gin.H{
		"providers": health,
		"timestamp": time.Now().UTC(),
	})
}

// Agent configuration endpoints

// createAgentConfig godoc
// @Summary Create agent embedding configuration
// @Description Configure embedding preferences for a specific AI agent
// @Tags embeddings
// @Accept json
// @Produce json
// @Param config body agents.AgentConfig true "Agent configuration with model preferences"
// @Success 201 {object} agents.AgentConfig "Created agent configuration"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /embeddings/agents [post]
func (api *EmbeddingAPI) createAgentConfig(c *gin.Context) {
	var config agents.AgentConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrBadRequest,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	// Set defaults
	config.ID = uuid.New()
	config.IsActive = true
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	if err := api.agentService.CreateConfig(c.Request.Context(), &config); err != nil {
		api.logger.Error("Failed to create agent config", map[string]any{
			"error":    err.Error(),
			"agent_id": sanitizeLogValue(config.AgentID),
		})
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrInternalServer,
			Message: "Failed to create agent configuration",
		})
		return
	}

	c.JSON(http.StatusCreated, config)
}

// getAgentConfig godoc
// @Summary Get agent embedding configuration
// @Description Retrieve embedding configuration for a specific agent
// @Tags embeddings
// @Accept json
// @Produce json
// @Param agentId path string true "Agent ID"
// @Success 200 {object} agents.AgentConfig "Agent configuration"
// @Failure 404 {object} ErrorResponse "Agent configuration not found"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /embeddings/agents/{agentId} [get]
func (api *EmbeddingAPI) getAgentConfig(c *gin.Context) {
	agentID := c.Param("agentId")

	config, err := api.agentService.GetConfig(c.Request.Context(), agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    ErrNotFound,
			Message: "Agent configuration not found",
		})
		return
	}

	c.JSON(http.StatusOK, config)
}

// updateAgentConfig godoc
// @Summary Update agent embedding configuration
// @Description Update embedding preferences and model selection for an agent
// @Tags embeddings
// @Accept json
// @Produce json
// @Param agentId path string true "Agent ID"
// @Param update body agents.ConfigUpdateRequest true "Configuration updates"
// @Success 200 {object} agents.AgentConfig "Updated agent configuration"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /embeddings/agents/{agentId} [put]
func (api *EmbeddingAPI) updateAgentConfig(c *gin.Context) {
	agentID := c.Param("agentId")

	var update agents.ConfigUpdateRequest
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrBadRequest,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	config, err := api.agentService.UpdateConfig(c.Request.Context(), agentID, &update)
	if err != nil {
		api.logger.Error("Failed to update agent config", map[string]any{
			"error":    err.Error(),
			"agent_id": sanitizeLogValue(agentID),
		})
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrInternalServer,
			Message: "Failed to update agent configuration",
		})
		return
	}

	c.JSON(http.StatusOK, config)
}

// getAgentModels godoc
// @Summary Get agent model assignments
// @Description Get primary and fallback models for an agent based on task type
// @Tags embeddings
// @Accept json
// @Produce json
// @Param agentId path string true "Agent ID"
// @Param task_type query string false "Task type (default: general_qa)"
// @Success 200 {object} map[string]interface{} "Model assignments"
// @Failure 404 {object} ErrorResponse "Agent configuration not found"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /embeddings/agents/{agentId}/models [get]
func (api *EmbeddingAPI) getAgentModels(c *gin.Context) {
	agentID := c.Param("agentId")
	taskTypeStr := c.DefaultQuery("task_type", string(agents.TaskTypeGeneralQA))
	taskType := agents.TaskType(taskTypeStr)

	primary, fallback, err := api.agentService.GetModelsForAgent(c.Request.Context(), agentID, taskType)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    ErrNotFound,
			Message: "Agent configuration not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agent_id":        agentID,
		"task_type":       taskType,
		"primary_models":  primary,
		"fallback_models": fallback,
	})
}

// getAgentCosts godoc
// @Summary Get agent embedding costs
// @Description Retrieve cost metrics for agent's embedding usage
// @Tags embeddings
// @Accept json
// @Produce json
// @Param agentId path string true "Agent ID"
// @Param period_days query string false "Period in days (default: 30, max: 365)"
// @Success 200 {object} map[string]interface{} "Cost metrics"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /embeddings/agents/{agentId}/costs [get]
func (api *EmbeddingAPI) getAgentCosts(c *gin.Context) {
	agentID := c.Param("agentId")
	periodDays := c.DefaultQuery("period_days", "30")

	// Convert to duration
	days := 30
	if d, err := strconv.Atoi(periodDays); err == nil && d > 0 && d <= 365 {
		days = d
	}
	period := time.Duration(days) * 24 * time.Hour

	// Create metrics adapter
	metricsAdapter := NewMetricsRepositoryAdapter(api.embeddingService, api.logger)

	// Get cost summary
	costs, err := metricsAdapter.GetAgentCosts(c.Request.Context(), agentID, period)
	if err != nil {
		api.logger.Error("Failed to get agent costs", map[string]any{
			"error":    err.Error(),
			"agent_id": sanitizeLogValue(agentID),
		})
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrInternalServer,
			Message: "Failed to retrieve cost metrics",
		})
		return
	}

	c.JSON(http.StatusOK, costs)
}

// Use the common ErrorResponse from errors.go

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
