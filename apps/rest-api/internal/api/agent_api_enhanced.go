package api

import (
	"net/http"

	"github.com/developer-mesh/developer-mesh/pkg/agents"
	"github.com/developer-mesh/developer-mesh/pkg/common/util"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// EnhancedAgentAPI handles agent management endpoints with full lifecycle support
type EnhancedAgentAPI struct {
	service *agents.EnhancedService
	logger  observability.Logger
}

// NewEnhancedAgentAPI creates a new enhanced agent API
func NewEnhancedAgentAPI(service *agents.EnhancedService, logger observability.Logger) *EnhancedAgentAPI {
	return &EnhancedAgentAPI{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes registers enhanced agent endpoints
func (a *EnhancedAgentAPI) RegisterRoutes(router *gin.RouterGroup) {
	agents := router.Group("/agents")

	// CRUD operations
	agents.POST("", a.registerAgent)
	agents.GET("", a.listAgents)
	agents.GET("/:id", a.getAgent)
	agents.PUT("/:id", a.updateAgent)
	agents.DELETE("/:id", a.deleteAgent)

	// State management
	agents.POST("/:id/activate", a.activateAgent)
	agents.POST("/:id/suspend", a.suspendAgent)
	agents.POST("/:id/terminate", a.terminateAgent)
	agents.POST("/:id/transition", a.transitionState)

	// Health and monitoring
	agents.POST("/:id/health", a.updateHealth)
	agents.GET("/:id/events", a.getEvents)
}

// registerAgent creates a new agent with lifecycle management
func (a *EnhancedAgentAPI) registerAgent(c *gin.Context) {
	var req agents.RegisterAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get tenant ID from context
	tenantID := util.GetTenantIDFromContext(c)
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}

	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant id format"})
		return
	}
	req.TenantID = tenantUUID

	// Register agent
	agent, err := a.service.RegisterAgent(c.Request.Context(), req)
	if err != nil {
		a.logger.Error("Failed to register agent", map[string]interface{}{
			"error":      err.Error(),
			"tenant_id":  tenantID,
			"agent_type": req.Type,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, agent)
}

// getAgent retrieves an agent by ID
func (a *EnhancedAgentAPI) getAgent(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent id required"})
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id format"})
		return
	}

	// Get tenant ID for validation
	tenantID := util.GetTenantIDFromContext(c)
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}

	agent, err := a.service.GetAgent(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	// Validate tenant access
	if agent.TenantID.String() != tenantID {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized"})
		return
	}

	c.JSON(http.StatusOK, agent)
}

// listAgents lists agents with filtering
func (a *EnhancedAgentAPI) listAgents(c *gin.Context) {
	tenantID := util.GetTenantIDFromContext(c)
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}

	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant id format"})
		return
	}

	// Build filter from query parameters
	filter := agents.AgentFilter{
		TenantID: &tenantUUID,
	}

	// Add optional filters
	if state := c.Query("state"); state != "" {
		agentState := agents.AgentState(state)
		filter.State = &agentState
	}

	if agentType := c.Query("type"); agentType != "" {
		filter.Type = &agentType
	}

	if available := c.Query("available"); available == "true" {
		isAvailable := true
		filter.IsAvailable = &isAvailable
	}

	agents, err := a.service.ListAgents(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"agents": agents})
}

// updateAgent updates an agent
func (a *EnhancedAgentAPI) updateAgent(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent id required"})
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id format"})
		return
	}

	var update agents.UpdateAgentRequest
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get tenant ID for validation
	tenantID := util.GetTenantIDFromContext(c)
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}

	// Verify agent belongs to tenant
	existing, err := a.service.GetAgent(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	if existing.TenantID.String() != tenantID {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized"})
		return
	}

	// Update agent
	agent, err := a.service.UpdateAgent(c.Request.Context(), id, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, agent)
}

// deleteAgent terminates an agent
func (a *EnhancedAgentAPI) deleteAgent(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent id required"})
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id format"})
		return
	}

	// Get user ID for audit
	userID := uuid.Nil // Would get from auth context

	// Terminate agent
	err = a.service.TerminateAgent(c.Request.Context(), id, "Deleted via API", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// activateAgent activates an agent
func (a *EnhancedAgentAPI) activateAgent(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id format"})
		return
	}

	userID := uuid.Nil // Would get from auth context

	err = a.service.ActivateAgent(c.Request.Context(), id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "agent activated"})
}

// suspendAgent suspends an agent
func (a *EnhancedAgentAPI) suspendAgent(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id format"})
		return
	}

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := uuid.Nil // Would get from auth context

	err = a.service.SuspendAgent(c.Request.Context(), id, req.Reason, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "agent suspended"})
}

// terminateAgent terminates an agent
func (a *EnhancedAgentAPI) terminateAgent(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id format"})
		return
	}

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := uuid.Nil // Would get from auth context

	err = a.service.TerminateAgent(c.Request.Context(), id, req.Reason, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "agent terminated"})
}

// transitionState transitions an agent to a new state
func (a *EnhancedAgentAPI) transitionState(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id format"})
		return
	}

	var req struct {
		TargetState string `json:"target_state" binding:"required"`
		Reason      string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := uuid.Nil // Would get from auth context

	err = a.service.TransitionState(
		c.Request.Context(),
		id,
		agents.AgentState(req.TargetState),
		req.Reason,
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "state transitioned"})
}

// updateHealth updates agent health metrics
func (a *EnhancedAgentAPI) updateHealth(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id format"})
		return
	}

	var health map[string]interface{}
	if err := c.ShouldBindJSON(&health); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = a.service.UpdateAgentHealth(c.Request.Context(), id, health)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "health updated"})
}

// getEvents retrieves agent events
func (a *EnhancedAgentAPI) getEvents(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id format"})
		return
	}

	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		// Parse limit
		limit = 100 // Default
	}

	events, err := a.service.GetAgentEvents(c.Request.Context(), id, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"events": events})
}
