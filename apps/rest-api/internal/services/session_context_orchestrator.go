package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/core"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/services"
)

// SessionContextOrchestrator coordinates session and context creation
// Following industry best practices for orchestration (Option C):
// - Maintains Single Responsibility Principle
// - Loose coupling via interfaces
// - Transaction-ready with rollback
// - Similar to patterns in Kubernetes, Docker Compose, Terraform
type SessionContextOrchestrator interface {
	// CreateSessionWithContext creates both a session and its linked context atomically
	// Returns the session with context_id populated and the created context
	CreateSessionWithContext(ctx context.Context, req *models.CreateSessionRequest) (*models.EdgeMCPSession, *models.Context, error)

	// TerminateSessionWithCleanup terminates session and cleans up associated resources
	// Includes virtual agent cleanup following ephemeral resource lifecycle
	TerminateSessionWithCleanup(ctx context.Context, sessionID string, reason string) error
}

// sessionContextOrchestrator implementation
type sessionContextOrchestrator struct {
	sessionService services.SessionService
	contextManager core.ContextManagerInterface
	sessionRepo    repository.SessionRepository
	logger         observability.Logger
	metrics        observability.MetricsClient
}

// SessionContextOrchestratorConfig holds configuration for the orchestrator
type SessionContextOrchestratorConfig struct {
	SessionService services.SessionService
	ContextManager core.ContextManagerInterface
	SessionRepo    repository.SessionRepository
	Logger         observability.Logger
	Metrics        observability.MetricsClient
}

// NewSessionContextOrchestrator creates a new session-context orchestrator
func NewSessionContextOrchestrator(config SessionContextOrchestratorConfig) SessionContextOrchestrator {
	return &sessionContextOrchestrator{
		sessionService: config.SessionService,
		contextManager: config.ContextManager,
		sessionRepo:    config.SessionRepo,
		logger:         config.Logger,
		metrics:        config.Metrics,
	}
}

// CreateSessionWithContext creates a session and context atomically
func (o *sessionContextOrchestrator) CreateSessionWithContext(
	ctx context.Context,
	req *models.CreateSessionRequest,
) (*models.EdgeMCPSession, *models.Context, error) {
	startTime := time.Now()
	defer func() {
		if o.metrics != nil {
			o.metrics.RecordHistogram("session_context.create.duration", time.Since(startTime).Seconds(), nil)
		}
	}()

	o.logger.Info("Starting session-context orchestration", map[string]interface{}{
		"tenant_id":   req.TenantID,
		"edge_mcp_id": req.EdgeMCPID,
		"client_type": req.ClientType,
		"client_name": req.ClientName,
	})

	// Step 1: Create the session via SessionService
	session, err := o.sessionService.CreateSession(ctx, req)
	if err != nil {
		o.recordMetric("session_context.create.error", 1, map[string]string{"step": "session_create"})
		return nil, nil, errors.Wrap(err, "orchestrator: failed to create session")
	}

	o.logger.Info("Session created, creating virtual agent", map[string]interface{}{
		"session_id":   session.SessionID,
		"session_uuid": session.ID,
		"tenant_id":    session.TenantID,
	})

	// Step 1.5: Create virtual agent for this session (JIT provisioning)
	// Following 2025 best practices for ephemeral agent identities
	virtualAgentID := o.generateVirtualAgentID(session)
	attribution := models.ResolveAttribution(session)

	virtualAgent := &models.Agent{
		ID:           virtualAgentID,
		TenantID:     session.TenantID,
		Name:         o.generateVirtualAgentName(req, session),
		Type:         "virtual_session_agent",
		Status:       "available",
		Capabilities: []string{"embedding", "context_management"},
		Metadata: map[string]interface{}{
			"session_id":        session.SessionID,
			"session_uuid":      session.ID.String(),
			"edge_mcp_id":       session.EdgeMCPID,
			"client_type":       string(req.ClientType),
			"client_name":       req.ClientName,
			"ephemeral":         true,
			"auto_created":      true,
			"created_by":        "session_orchestrator",
			"attribution_level": string(attribution.Level),
			"cost_center":       attribution.CostCenter,
			"billable_unit":     attribution.BillableUnit,
		},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Attribution: &attribution,
	}

	// Store attribution metadata in agent metadata for persistence
	if attribution.UserID != nil {
		virtualAgent.Metadata["user_id"] = attribution.UserID.String()
	}
	if attribution.EdgeMCPID != nil {
		virtualAgent.Metadata["edge_mcp_id_attr"] = *attribution.EdgeMCPID
	}
	if attribution.SessionID != nil {
		virtualAgent.Metadata["session_id_attr"] = *attribution.SessionID
	}

	o.logger.Info("Virtual agent created for session", map[string]interface{}{
		"agent_id":          virtualAgent.ID,
		"session_id":        session.SessionID,
		"attribution_level": string(attribution.Level),
		"cost_center":       attribution.CostCenter,
	})

	// Step 2: Create the context via ContextManager
	// Build context with session linkage and virtual agent
	// Note: agent_id is stored in metadata as virtual agents are ephemeral and not persisted to agent_configurations table
	contextToCreate := &models.Context{
		Type:          "conversation", // Default type for session contexts
		TenantID:      session.TenantID.String(),
		SessionID:     session.SessionID, // Link via session_id string
		AgentID:       "",                // Leave empty - virtual agent stored in metadata
		ModelID:       "",                // Will be set when model is selected
		MaxTokens:     100000,            // Default max tokens
		CurrentTokens: 0,
		Metadata: map[string]any{
			"session_uuid":       session.ID.String(),
			"edge_mcp_id":        session.EdgeMCPID,
			"client_name":        req.ClientName,
			"client_type":        string(req.ClientType),
			"created_by":         "session_context_orchestrator",
			"orchestration_ts":   time.Now().Unix(),
			"virtual_agent_id":   virtualAgent.ID,           // Virtual agent ID (ephemeral)
			"virtual_agent_name": virtualAgent.Name,         // Virtual agent name for logs
			"attribution_level":  string(attribution.Level), // Cost attribution level
			"cost_center":        attribution.CostCenter,    // Who accumulates costs
			"billable_unit":      attribution.BillableUnit,  // Who gets charged
		},
	}

	// CreateContext generates UUID if not provided
	createdContext, err := o.contextManager.CreateContext(ctx, contextToCreate)
	if err != nil {
		// Rollback: Delete the session we just created
		o.logger.Error("Context creation failed, rolling back session", map[string]interface{}{
			"session_id": session.SessionID,
			"error":      err.Error(),
		})

		if deleteErr := o.rollbackSession(ctx, session.SessionID); deleteErr != nil {
			o.logger.Error("Failed to rollback session after context creation failure", map[string]interface{}{
				"session_id":   session.SessionID,
				"delete_error": deleteErr.Error(),
				"orig_error":   err.Error(),
			})
		}

		o.recordMetric("session_context.create.error", 1, map[string]string{"step": "context_create"})
		return nil, nil, errors.Wrap(err, "orchestrator: failed to create context")
	}

	o.logger.Info("Context created, linking to session", map[string]interface{}{
		"session_id": session.SessionID,
		"context_id": createdContext.ID,
	})

	// Step 3: Link context_id back to session
	// Parse context UUID
	contextUUID, err := uuid.Parse(createdContext.ID)
	if err != nil {
		// Rollback: Delete both session and context
		o.logger.Error("Failed to parse context UUID, rolling back", map[string]interface{}{
			"session_id": session.SessionID,
			"context_id": createdContext.ID,
			"error":      err.Error(),
		})

		_ = o.rollbackSession(ctx, session.SessionID)
		_ = o.contextManager.DeleteContext(ctx, createdContext.ID)

		o.recordMetric("session_context.create.error", 1, map[string]string{"step": "uuid_parse"})
		return nil, nil, errors.Wrap(err, "orchestrator: failed to parse context UUID")
	}

	// Update session with context_id
	session.ContextID = &contextUUID
	if err := o.sessionRepo.UpdateSession(ctx, session); err != nil {
		// Rollback: Delete both session and context
		o.logger.Error("Failed to link context to session, rolling back", map[string]interface{}{
			"session_id": session.SessionID,
			"context_id": createdContext.ID,
			"error":      err.Error(),
		})

		_ = o.rollbackSession(ctx, session.SessionID)
		_ = o.contextManager.DeleteContext(ctx, createdContext.ID)

		o.recordMetric("session_context.create.error", 1, map[string]string{"step": "session_update"})
		return nil, nil, errors.Wrap(err, "orchestrator: failed to link context to session")
	}

	// Success! Log and emit metrics
	o.logger.Info("Session-context orchestration complete", map[string]interface{}{
		"session_id":   session.SessionID,
		"session_uuid": session.ID,
		"context_id":   createdContext.ID,
		"tenant_id":    session.TenantID,
		"duration_ms":  time.Since(startTime).Milliseconds(),
	})

	o.recordMetric("session_context.create.success", 1, map[string]string{
		"client_type": string(req.ClientType),
	})

	return session, createdContext, nil
}

// TerminateSessionWithCleanup orchestrates session termination with resource cleanup
// Following 2025 best practices for graceful shutdown and resource lifecycle
func (o *sessionContextOrchestrator) TerminateSessionWithCleanup(ctx context.Context, sessionID string, reason string) error {
	startTime := time.Now()
	defer func() {
		if o.metrics != nil {
			o.metrics.RecordHistogram("session_context.terminate.duration", time.Since(startTime).Seconds(), nil)
		}
	}()

	o.logger.Info("Starting session termination with cleanup", map[string]interface{}{
		"session_id": sessionID,
		"reason":     reason,
	})

	// Step 1: Terminate the session (marks as terminated in DB, invalidates cache)
	if err := o.sessionService.TerminateSession(ctx, sessionID, reason); err != nil {
		o.recordMetric("session_context.terminate.error", 1, map[string]string{"step": "session_terminate"})
		return errors.Wrap(err, "orchestrator: failed to terminate session")
	}

	o.logger.Info("Session terminated, cleaning up virtual agent", map[string]interface{}{
		"session_id": sessionID,
	})

	// Step 2: Cleanup virtual agent (ephemeral, logs lifecycle completion)
	if err := o.cleanupVirtualAgent(ctx, sessionID); err != nil {
		// Log error but don't fail - agent cleanup is best-effort for ephemeral agents
		o.logger.Warn("Virtual agent cleanup encountered error", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
	}

	// Success! Log and emit metrics
	o.logger.Info("Session termination with cleanup complete", map[string]interface{}{
		"session_id":  sessionID,
		"reason":      reason,
		"duration_ms": time.Since(startTime).Milliseconds(),
	})

	o.recordMetric("session_context.terminate.success", 1, map[string]string{
		"reason": reason,
	})

	return nil
}

// cleanupVirtualAgent cleans up the virtual agent when session terminates
// Following 2025 best practices for ephemeral resource cleanup
func (o *sessionContextOrchestrator) cleanupVirtualAgent(ctx context.Context, sessionID string) error {
	// Virtual agents are ephemeral and tied to session lifecycle
	// They are not persisted to database, so no cleanup needed
	// This method is provided for future extensibility if we add:
	// - Agent persistence
	// - Agent repository
	// - Cleanup hooks

	virtualAgentID := "virtual-agent-" + sessionID

	o.logger.Info("Virtual agent lifecycle complete", map[string]interface{}{
		"agent_id":   virtualAgentID,
		"session_id": sessionID,
		"reason":     "session_terminated",
	})

	o.recordMetric("virtual_agent.cleanup", 1, map[string]string{
		"reason": "session_terminated",
	})

	// Future: If agent repository is available and agents are persisted
	// if o.agentRepo != nil {
	//     return o.agentRepo.DeleteAgent(ctx, virtualAgentID)
	// }

	return nil
}

// rollbackSession attempts to delete a session (rollback operation)
func (o *sessionContextOrchestrator) rollbackSession(ctx context.Context, sessionID string) error {
	if err := o.sessionService.TerminateSession(ctx, sessionID, "orchestration_rollback"); err != nil {
		return errors.Wrap(err, "failed to rollback session")
	}
	return nil
}

// generateVirtualAgentID generates a unique UUID for the virtual session agent
// Following 2025 best practices: UUIDs for database compatibility
func (o *sessionContextOrchestrator) generateVirtualAgentID(session *models.EdgeMCPSession) string {
	// Generate a new UUID for the virtual agent
	// This ensures compatibility with agents table UUID primary key
	return uuid.New().String()
}

// generateVirtualAgentName generates a human-readable name for the virtual agent
// Includes client information for easy identification in logs and dashboards
func (o *sessionContextOrchestrator) generateVirtualAgentName(req *models.CreateSessionRequest, session *models.EdgeMCPSession) string {
	if req.ClientName != "" {
		return req.ClientName + " Session Agent"
	}
	// Fallback to client type if name not provided
	return string(req.ClientType) + " Session Agent"
}

// recordMetric records a metric if metrics client is available
func (o *sessionContextOrchestrator) recordMetric(name string, value interface{}, labels map[string]string) {
	if o.metrics == nil {
		return
	}

	// Add common labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["service"] = "session_context_orchestrator"

	// Record based on type
	switch v := value.(type) {
	case float64:
		o.metrics.RecordHistogram(name, v, labels)
	case time.Duration:
		o.metrics.RecordHistogram(name, v.Seconds(), labels)
	case int:
		o.metrics.IncrementCounterWithLabels(name, float64(v), labels)
	}
}
