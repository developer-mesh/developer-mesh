package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/services"
	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	pkgservices "github.com/developer-mesh/developer-mesh/pkg/services"
)

// SessionHandler handles edge MCP session management endpoints
type SessionHandler struct {
	sessionService  pkgservices.SessionService
	orchestrator    services.SessionContextOrchestrator
	logger          observability.Logger
	metricsClient   observability.MetricsClient
	auditLogger     *auth.AuditLogger
	useOrchestrator bool // Feature flag to enable orchestration
}

// NewSessionHandler creates a new session handler
func NewSessionHandler(
	sessionService pkgservices.SessionService,
	orchestrator services.SessionContextOrchestrator,
	logger observability.Logger,
	metricsClient observability.MetricsClient,
	auditLogger *auth.AuditLogger,
) *SessionHandler {
	return &SessionHandler{
		sessionService:  sessionService,
		orchestrator:    orchestrator,
		logger:          logger,
		metricsClient:   metricsClient,
		auditLogger:     auditLogger,
		useOrchestrator: orchestrator != nil, // Enable if orchestrator provided
	}
}

// RegisterRoutes registers all session API routes
func (h *SessionHandler) RegisterRoutes(router *gin.RouterGroup) {
	sessions := router.Group("/sessions")
	{
		// Session lifecycle
		sessions.POST("", h.CreateSession)
		sessions.GET("/:sessionId", h.GetSession)
		sessions.POST("/:sessionId/refresh", h.RefreshSession)
		sessions.DELETE("/:sessionId", h.TerminateSession)
		sessions.POST("/:sessionId/validate", h.ValidateSession)

		// Session queries
		sessions.GET("", h.ListSessions)
		sessions.GET("/active", h.ListActiveSessions)
		sessions.GET("/metrics", h.GetMetrics)

		// Tool execution tracking
		sessions.POST("/:sessionId/tools/execute", h.RecordToolExecution)
		sessions.GET("/:sessionId/tools/executions", h.GetToolExecutions)

		// Activity tracking
		sessions.POST("/:sessionId/activity", h.UpdateActivity)
	}
}

// CreateSession creates a new edge MCP session
// @Summary Create a new session
// @Description Creates a new edge MCP session for authentication and tracking
// @Tags Sessions
// @Accept json
// @Produce json
// @Param request body models.CreateSessionRequest true "Session creation request"
// @Success 201 {object} models.SessionResponse
// @Failure 400 {object} SessionErrorResponse
// @Failure 401 {object} SessionErrorResponse
// @Failure 429 {object} SessionErrorResponse "Session limit exceeded"
// @Failure 500 {object} SessionErrorResponse
// @Router /api/v1/sessions [post]
func (h *SessionHandler) CreateSession(c *gin.Context) {
	start := time.Now()

	// Parse request
	var req models.CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.recordMetric(c, "session.create.error", 1, map[string]string{"error": "invalid_request"})
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Get tenant from context (set by auth middleware)
	tenantIDStr := c.GetString("tenant_id")
	if tenantIDStr == "" {
		h.recordMetric(c, "session.create.error", 1, map[string]string{"error": "missing_tenant"})
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		h.recordMetric(c, "session.create.error", 1, map[string]string{"error": "invalid_tenant"})
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant_id"})
		return
	}

	// Set tenant ID in request
	req.TenantID = tenantID

	// Get user ID if available
	if userIDStr := c.GetString("user_id"); userIDStr != "" {
		if userID, err := uuid.Parse(userIDStr); err == nil {
			req.UserID = &userID
		}
	}

	// Add connection metadata from request context
	if req.Metadata == nil {
		req.Metadata = make(map[string]interface{})
	}
	req.Metadata["ip_address"] = c.ClientIP()
	req.Metadata["user_agent"] = c.GetHeader("User-Agent")
	req.Metadata["protocol"] = "REST"

	// Create session (with context if orchestrator available)
	var session *models.EdgeMCPSession
	var context *models.Context

	if h.useOrchestrator {
		// Use orchestrator to create both session and context atomically
		h.logger.Info("Creating session with context via orchestrator", map[string]interface{}{
			"tenant_id":   tenantID,
			"client_type": req.ClientType,
		})
		session, context, err = h.orchestrator.CreateSessionWithContext(c.Request.Context(), &req)
	} else {
		// Fallback to session-only creation
		h.logger.Warn("Orchestrator not available, creating session only (context not linked)", map[string]interface{}{
			"tenant_id":   tenantID,
			"client_type": req.ClientType,
		})
		session, err = h.sessionService.CreateSession(c.Request.Context(), &req)
	}

	if err != nil {
		h.logger.Error("Failed to create session", map[string]interface{}{
			"tenant_id":        tenantID,
			"error":            err.Error(),
			"use_orchestrator": h.useOrchestrator,
		})
		h.recordMetric(c, "session.create.error", 1, map[string]string{"error": "service_error"})

		// Check for specific errors
		if err.Error() == "session limit exceeded" {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Session limit exceeded for tenant"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	// Audit log
	if h.auditLogger != nil {
		auditMetadata := map[string]interface{}{
			"session_id":        session.SessionID,
			"edge_mcp_id":       session.EdgeMCPID,
			"orchestrator_used": h.useOrchestrator,
		}
		if context != nil {
			auditMetadata["context_id"] = context.ID
			auditMetadata["context_created"] = true
		}

		h.auditLogger.LogAuthAttempt(c.Request.Context(), auth.AuditEvent{
			EventType: "session.create",
			TenantID:  tenantID.String(),
			UserID:    c.GetString("user_id"),
			AuthType:  "session",
			Success:   true,
			IPAddress: c.ClientIP(),
			UserAgent: c.GetHeader("User-Agent"),
			Metadata:  auditMetadata,
		})
	}

	// Record metrics
	h.recordMetric(c, "session.create.success", 1, map[string]string{
		"client_type":       string(req.ClientType),
		"orchestrator_used": strconv.FormatBool(h.useOrchestrator),
		"context_created":   strconv.FormatBool(context != nil),
	})
	h.recordMetric(c, "session.create.duration", time.Since(start).Seconds(), nil)

	// Build response with context info if available
	response := models.NewSessionResponse(session)
	if context != nil {
		// Add context_id to response metadata or as a new field
		// The SessionResponse includes ContextID via session.ContextID
		h.logger.Info("Session created with linked context", map[string]interface{}{
			"session_id": session.SessionID,
			"context_id": context.ID,
		})
	}

	// Return response
	c.JSON(http.StatusCreated, response)
}

// GetSession retrieves a session by ID
// @Summary Get session details
// @Description Retrieves details of a specific session
// @Tags Sessions
// @Accept json
// @Produce json
// @Param sessionId path string true "Session ID"
// @Success 200 {object} models.SessionResponse
// @Failure 404 {object} SessionErrorResponse
// @Failure 500 {object} SessionErrorResponse
// @Router /api/v1/sessions/{sessionId} [get]
func (h *SessionHandler) GetSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId required"})
		return
	}

	session, err := h.sessionService.GetSession(c.Request.Context(), sessionID)
	if err != nil {
		if err.Error() == "session not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}
		if err.Error() == "session has expired" {
			c.JSON(http.StatusGone, gin.H{"error": "Session has expired"})
			return
		}

		h.logger.Error("Failed to get session", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve session"})
		return
	}

	// Verify tenant access
	tenantIDStr := c.GetString("tenant_id")
	if tenantIDStr != "" {
		if tenantID, err := uuid.Parse(tenantIDStr); err == nil {
			if session.TenantID != tenantID {
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
				return
			}
		}
	}

	c.JSON(http.StatusOK, models.NewSessionResponse(session))
}

// RefreshSession extends the session expiry
// @Summary Refresh session
// @Description Extends the expiry time of an active session
// @Tags Sessions
// @Accept json
// @Produce json
// @Param sessionId path string true "Session ID"
// @Success 200 {object} models.SessionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} SessionErrorResponse
// @Failure 500 {object} SessionErrorResponse
// @Router /api/v1/sessions/{sessionId}/refresh [post]
func (h *SessionHandler) RefreshSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId required"})
		return
	}

	session, err := h.sessionService.RefreshSession(c.Request.Context(), sessionID)
	if err != nil {
		if err.Error() == "session not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}
		if err.Error() == "cannot refresh non-active session" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		h.logger.Error("Failed to refresh session", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh session"})
		return
	}

	// Audit log
	if h.auditLogger != nil {
		h.auditLogger.LogAuthAttempt(c.Request.Context(), auth.AuditEvent{
			EventType: "session.refresh",
			TenantID:  session.TenantID.String(),
			UserID:    c.GetString("user_id"),
			AuthType:  "session",
			Success:   true,
			IPAddress: c.ClientIP(),
			UserAgent: c.GetHeader("User-Agent"),
			Metadata: map[string]interface{}{
				"session_id": sessionID,
			},
		})
	}

	h.recordMetric(c, "session.refresh", 1, nil)

	c.JSON(http.StatusOK, models.NewSessionResponse(session))
}

// TerminateSession terminates an active session
// @Summary Terminate session
// @Description Terminates an active session
// @Tags Sessions
// @Accept json
// @Produce json
// @Param sessionId path string true "Session ID"
// @Param reason body object false "Termination reason"
// @Success 204 "No Content"
// @Failure 404 {object} SessionErrorResponse
// @Failure 500 {object} SessionErrorResponse
// @Router /api/v1/sessions/{sessionId} [delete]
func (h *SessionHandler) TerminateSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId required"})
		return
	}

	// Get termination reason from body if provided
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)

	reason := body.Reason
	if reason == "" {
		reason = "User requested termination"
	}

	err := h.sessionService.TerminateSession(c.Request.Context(), sessionID, reason)
	if err != nil {
		if err.Error() == "session not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}

		h.logger.Error("Failed to terminate session", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to terminate session"})
		return
	}

	// Audit log
	if h.auditLogger != nil {
		h.auditLogger.LogAuthAttempt(c.Request.Context(), auth.AuditEvent{
			EventType: "session.terminate",
			TenantID:  c.GetString("tenant_id"),
			UserID:    c.GetString("user_id"),
			AuthType:  "session",
			Success:   true,
			IPAddress: c.ClientIP(),
			UserAgent: c.GetHeader("User-Agent"),
			Metadata: map[string]interface{}{
				"session_id": sessionID,
				"reason":     reason,
			},
		})
	}

	h.recordMetric(c, "session.terminate", 1, map[string]string{"reason": reason})

	c.Status(http.StatusNoContent)
}

// ValidateSession validates a session and returns its details if valid
// @Summary Validate session
// @Description Validates a session and returns its details if valid
// @Tags Sessions
// @Accept json
// @Produce json
// @Param sessionId path string true "Session ID"
// @Success 200 {object} models.SessionResponse
// @Failure 400 {object} SessionErrorResponse
// @Failure 401 {object} SessionErrorResponse "Session invalid or expired"
// @Failure 404 {object} SessionErrorResponse
// @Failure 500 {object} SessionErrorResponse
// @Router /api/v1/sessions/{sessionId}/validate [post]
func (h *SessionHandler) ValidateSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId required"})
		return
	}

	session, err := h.sessionService.ValidateSession(c.Request.Context(), sessionID)
	if err != nil {
		if err.Error() == "session not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}
		if err.Error() == "session has expired" || err.Error() == "session is not active" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		h.logger.Error("Failed to validate session", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate session"})
		return
	}

	h.recordMetric(c, "session.validate", 1, map[string]string{
		"status": string(session.Status),
	})

	c.JSON(http.StatusOK, models.NewSessionResponse(session))
}

// ListSessions lists sessions with filtering
// @Summary List sessions
// @Description Lists sessions with optional filtering
// @Tags Sessions
// @Accept json
// @Produce json
// @Param status query string false "Filter by status"
// @Param client_type query string false "Filter by client type"
// @Param limit query int false "Limit results (default 100)"
// @Param offset query int false "Offset for pagination"
// @Success 200 {object} ListSessionsResponse
// @Failure 500 {object} SessionErrorResponse
// @Router /api/v1/sessions [get]
func (h *SessionHandler) ListSessions(c *gin.Context) {
	tenantIDStr := c.GetString("tenant_id")
	if tenantIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant_id"})
		return
	}

	// Build filter
	filter := &models.SessionFilter{
		TenantID: &tenantID,
	}

	// Parse query parameters
	if status := c.Query("status"); status != "" {
		s := models.SessionStatus(status)
		filter.Status = &s
	}

	if clientType := c.Query("client_type"); clientType != "" {
		ct := models.ClientType(clientType)
		filter.ClientType = &ct
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = offset
		}
	}

	sessions, err := h.sessionService.ListSessions(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list sessions", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list sessions"})
		return
	}

	// Convert to responses
	responses := make([]*models.SessionResponse, len(sessions))
	for i, session := range sessions {
		responses[i] = models.NewSessionResponse(session)
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": responses,
		"count":    len(responses),
	})
}

// ListActiveSessions lists all active sessions for the tenant
// @Summary List active sessions
// @Description Lists all active sessions for the authenticated tenant
// @Tags Sessions
// @Accept json
// @Produce json
// @Success 200 {object} ListSessionsResponse
// @Failure 500 {object} SessionErrorResponse
// @Router /api/v1/sessions/active [get]
func (h *SessionHandler) ListActiveSessions(c *gin.Context) {
	tenantIDStr := c.GetString("tenant_id")
	if tenantIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant_id"})
		return
	}

	sessions, err := h.sessionService.ListActiveSessions(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to list active sessions", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list active sessions"})
		return
	}

	// Convert to responses
	responses := make([]*models.SessionResponse, len(sessions))
	for i, session := range sessions {
		responses[i] = models.NewSessionResponse(session)
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": responses,
		"count":    len(responses),
	})
}

// GetMetrics retrieves session metrics for the tenant
// @Summary Get session metrics
// @Description Retrieves aggregated session metrics for the tenant
// @Tags Sessions
// @Accept json
// @Produce json
// @Param since query string false "Metrics since (RFC3339 timestamp)"
// @Success 200 {object} models.SessionMetrics
// @Failure 500 {object} SessionErrorResponse
// @Router /api/v1/sessions/metrics [get]
func (h *SessionHandler) GetMetrics(c *gin.Context) {
	tenantIDStr := c.GetString("tenant_id")
	if tenantIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant_id"})
		return
	}

	// Parse since parameter
	since := time.Now().AddDate(0, 0, -30) // Default to last 30 days
	if sinceStr := c.Query("since"); sinceStr != "" {
		if parsedTime, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsedTime
		}
	}

	metrics, err := h.sessionService.GetSessionMetrics(c.Request.Context(), tenantID, since)
	if err != nil {
		h.logger.Error("Failed to get session metrics", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get metrics"})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

// RecordToolExecution records a tool execution in the session
// @Summary Record tool execution
// @Description Records a tool execution for audit and metrics
// @Tags Sessions
// @Accept json
// @Produce json
// @Param sessionId path string true "Session ID"
// @Param execution body models.SessionToolExecutionRequest true "Tool execution details"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} SessionErrorResponse
// @Failure 500 {object} SessionErrorResponse
// @Router /api/v1/sessions/{sessionId}/tools/execute [post]
func (h *SessionHandler) RecordToolExecution(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId required"})
		return
	}

	var req models.SessionToolExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	err := h.sessionService.RecordToolExecution(c.Request.Context(), sessionID, &req)
	if err != nil {
		if err.Error() == "invalid session" || err.Error() == "session not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found or invalid"})
			return
		}

		h.logger.Error("Failed to record tool execution", map[string]interface{}{
			"session_id": sessionID,
			"tool_name":  req.ToolName,
			"error":      err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record tool execution"})
		return
	}

	h.recordMetric(c, "tool_execution.recorded", 1, map[string]string{
		"tool_name": req.ToolName,
	})

	c.Status(http.StatusNoContent)
}

// GetToolExecutions retrieves tool executions for a session
// @Summary Get tool executions
// @Description Retrieves tool execution history for a session
// @Tags Sessions
// @Accept json
// @Produce json
// @Param sessionId path string true "Session ID"
// @Param limit query int false "Limit results (default 100)"
// @Success 200 {object} ListToolExecutionsResponse
// @Failure 404 {object} SessionErrorResponse
// @Failure 500 {object} SessionErrorResponse
// @Router /api/v1/sessions/{sessionId}/tools/executions [get]
func (h *SessionHandler) GetToolExecutions(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId required"})
		return
	}

	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	executions, err := h.sessionService.GetSessionToolExecutions(c.Request.Context(), sessionID, limit)
	if err != nil {
		if err.Error() == "session not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}

		h.logger.Error("Failed to get tool executions", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get tool executions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"executions": executions,
		"count":      len(executions),
	})
}

// UpdateActivity updates the session's last activity timestamp
// @Summary Update session activity
// @Description Updates the last activity timestamp for a session
// @Tags Sessions
// @Accept json
// @Produce json
// @Param sessionId path string true "Session ID"
// @Success 204 "No Content"
// @Failure 404 {object} SessionErrorResponse
// @Failure 500 {object} SessionErrorResponse
// @Router /api/v1/sessions/{sessionId}/activity [post]
func (h *SessionHandler) UpdateActivity(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId required"})
		return
	}

	err := h.sessionService.UpdateSessionActivity(c.Request.Context(), sessionID)
	if err != nil {
		if err.Error() == "session not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}
		if err.Error() == "session has expired" {
			c.JSON(http.StatusGone, gin.H{"error": "Session has expired"})
			return
		}

		h.logger.Error("Failed to update session activity", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update activity"})
		return
	}

	c.Status(http.StatusNoContent)
}

// Helper methods

func (h *SessionHandler) recordMetric(c *gin.Context, name string, value interface{}, labels map[string]string) {
	if h.metricsClient == nil {
		return
	}

	// Add common labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["endpoint"] = c.Request.URL.Path
	labels["method"] = c.Request.Method

	// Record based on type
	switch v := value.(type) {
	case float64:
		h.metricsClient.RecordHistogram(name, v, labels)
	case int:
		h.metricsClient.IncrementCounterWithLabels(name, float64(v), labels)
	}
}

// Response types for documentation

type ListSessionsResponse struct {
	Sessions []*models.SessionResponse `json:"sessions"`
	Count    int                       `json:"count"`
}

type ListToolExecutionsResponse struct {
	Executions []*models.SessionToolExecution `json:"executions"`
	Count      int                            `json:"count"`
}

// SessionErrorResponse represents an error response for session endpoints
type SessionErrorResponse struct {
	Error   string                 `json:"error"`
	Details map[string]interface{} `json:"details,omitempty"`
}
