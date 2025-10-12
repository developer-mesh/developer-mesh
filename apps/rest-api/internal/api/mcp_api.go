package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/gin-gonic/gin"
)

// ContextManagerInterface defines the interface for context management
type ContextManagerInterface interface {
	CreateContext(ctx context.Context, context *models.Context) (*models.Context, error)
	GetContext(ctx context.Context, contextID string) (*models.Context, error)
	UpdateContext(ctx context.Context, contextID string, context *models.Context, options *models.ContextUpdateOptions) (*models.Context, error)
	DeleteContext(ctx context.Context, contextID string) error
	ListContexts(ctx context.Context, agentID, sessionID string, options map[string]any) ([]*models.Context, error)
	SearchInContext(ctx context.Context, contextID, query string) ([]models.ContextItem, error)
	SummarizeContext(ctx context.Context, contextID string) (string, error)
}

// MCPAPI handles the MCP-specific API endpoints
type MCPAPI struct {
	contextManager     ContextManagerInterface
	semanticContextMgr repository.SemanticContextManager // Optional semantic context manager
}

// NewMCPAPI creates a new MCP API handler
func NewMCPAPI(contextManager any) *MCPAPI {
	return &MCPAPI{
		contextManager:     contextManager.(ContextManagerInterface),
		semanticContextMgr: nil, // Will be set via SetSemanticContextManager if available
	}
}

// SetSemanticContextManager sets the optional semantic context manager
func (api *MCPAPI) SetSemanticContextManager(mgr repository.SemanticContextManager) {
	api.semanticContextMgr = mgr
}

// RegisterRoutes registers all MCP API routes
func (api *MCPAPI) RegisterRoutes(router *gin.RouterGroup) {
	mcpRoutes := router.Group("/mcp")
	{
		mcpRoutes.POST("/context", api.createContext)
		mcpRoutes.GET("/context/:id", api.getContext)
		mcpRoutes.PUT("/context/:id", api.updateContext)
		mcpRoutes.DELETE("/context/:id", api.deleteContext)
		mcpRoutes.GET("/contexts", api.listContexts)
		mcpRoutes.POST("/context/:id/search", api.searchContext)
		mcpRoutes.GET("/context/:id/summary", api.summarizeContext)
		// Semantic context management endpoints
		mcpRoutes.POST("/context/:id/compact", api.compactContext)
	}
}

// createContext creates a new context
func (api *MCPAPI) createContext(c *gin.Context) {
	var request models.Context
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := api.contextManager.CreateContext(c.Request.Context(), &request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "context created", "id": request.ID})
}

// getContext retrieves a context by ID with optional semantic retrieval
func (api *MCPAPI) getContext(c *gin.Context) {
	contextID := c.Param("id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context ID is required"})
		return
	}

	// Check for semantic retrieval parameters
	relevanceQuery := c.Query("relevant_to")
	maxTokensStr := c.Query("max_tokens")

	// If semantic context manager is available and semantic retrieval is requested
	if api.semanticContextMgr != nil && relevanceQuery != "" {
		maxTokens := 4000 // Default
		if maxTokensStr != "" {
			if mt, err := parseIntParam(maxTokensStr); err == nil {
				maxTokens = mt
			}
		}

		contextData, err := api.semanticContextMgr.GetRelevantContext(
			c.Request.Context(),
			contextID,
			relevanceQuery,
			maxTokens,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"context":  contextData,
			"semantic": true,
			"mode":     "semantic_retrieval",
		})
		return
	}

	// Fall back to regular retrieval
	contextData, err := api.contextManager.GetContext(c.Request.Context(), contextID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"context":  contextData,
		"semantic": false,
	})
}

// updateContext updates an existing context
func (api *MCPAPI) updateContext(c *gin.Context) {
	contextID := c.Param("id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context ID is required"})
		return
	}

	// Parse request body
	var updateRequest struct {
		Content []models.ContextItem         `json:"content"`
		Options *models.ContextUpdateOptions `json:"options,omitempty"`
	}

	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	// Get the current context
	currentContext, err := api.contextManager.GetContext(c.Request.Context(), contextID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "context not found: " + err.Error()})
		return
	}

	// Ensure we have options
	if updateRequest.Options == nil {
		updateRequest.Options = &models.ContextUpdateOptions{}
	}

	// Update content if provided
	if updateRequest.Content != nil {
		currentContext.Content = updateRequest.Content
	}

	// Update the context in the database
	updatedContext, err := api.contextManager.UpdateContext(
		c.Request.Context(),
		contextID,
		currentContext,
		updateRequest.Options,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update context: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, updatedContext)
}

// deleteContext deletes a context
func (api *MCPAPI) deleteContext(c *gin.Context) {
	contextID := c.Param("id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context ID is required"})
		return
	}

	err := api.contextManager.DeleteContext(c.Request.Context(), contextID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// listContexts returns a list of contexts for an agent
func (api *MCPAPI) listContexts(c *gin.Context) {
	agentID := c.Query("agent_id")
	sessionID := c.Query("session_id")

	contexts, err := api.contextManager.ListContexts(c.Request.Context(), agentID, sessionID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"contexts": contexts})
}

// searchContext searches for text within a context with optional semantic search
func (api *MCPAPI) searchContext(c *gin.Context) {
	contextID := c.Param("id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context ID is required"})
		return
	}

	var request struct {
		Query    string `json:"query" binding:"required"`
		Limit    int    `json:"limit,omitempty"`
		Semantic bool   `json:"semantic,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set default limit
	if request.Limit == 0 {
		request.Limit = 10
	}

	// If semantic search is requested and manager is available
	if request.Semantic && api.semanticContextMgr != nil {
		results, err := api.semanticContextMgr.SearchContext(
			c.Request.Context(),
			request.Query,
			contextID,
			request.Limit,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"results":  results,
			"count":    len(results),
			"semantic": true,
		})
		return
	}

	// Fall back to regular text search
	results, err := api.contextManager.SearchInContext(c.Request.Context(), contextID, request.Query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results":  results,
		"count":    len(results),
		"semantic": false,
	})
}

// summarizeContext generates a summary of a context
func (api *MCPAPI) summarizeContext(c *gin.Context) {
	contextID := c.Param("id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context ID is required"})
		return
	}

	summary, err := api.contextManager.SummarizeContext(c.Request.Context(), contextID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"summary": summary})
}

// compactContext applies compaction strategy to a context
func (api *MCPAPI) compactContext(c *gin.Context) {
	contextID := c.Param("id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context ID is required"})
		return
	}

	var req struct {
		Strategy string `json:"strategy" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	// Check if semantic context manager is available
	if api.semanticContextMgr == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "semantic context management not available"})
		return
	}

	// Validate strategy
	strategy := repository.CompactionStrategy(req.Strategy)
	validStrategies := map[repository.CompactionStrategy]bool{
		repository.CompactionSummarize: true,
		repository.CompactionPrune:     true,
		repository.CompactionSemantic:  true,
		repository.CompactionSliding:   true,
		repository.CompactionToolClear: true,
	}

	if !validStrategies[strategy] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid strategy, must be one of: summarize, prune, semantic, sliding, tool_clear",
		})
		return
	}

	// Execute compaction
	if err := api.semanticContextMgr.CompactContext(c.Request.Context(), contextID, strategy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "compacted",
		"context_id": contextID,
		"strategy":   req.Strategy,
	})
}

// parseIntParam safely parses an integer parameter
func parseIntParam(value string) (int, error) {
	var result int
	_, err := fmt.Sscanf(value, "%d", &result)
	return result, err
}
