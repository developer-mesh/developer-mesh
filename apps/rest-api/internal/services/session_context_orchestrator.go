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

	o.logger.Info("Session created, creating context", map[string]interface{}{
		"session_id":   session.SessionID,
		"session_uuid": session.ID,
		"tenant_id":    session.TenantID,
	})

	// Step 2: Create the context via ContextManager
	// Build context with session linkage
	contextToCreate := &models.Context{
		Type:          "conversation", // Default type for session contexts
		TenantID:      session.TenantID.String(),
		SessionID:     session.SessionID, // Link via session_id string
		AgentID:       "",                // Will be set when agent is assigned
		ModelID:       "",                // Will be set when model is selected
		MaxTokens:     100000,            // Default max tokens
		CurrentTokens: 0,
		Metadata: map[string]any{
			"session_uuid":     session.ID.String(),
			"edge_mcp_id":      session.EdgeMCPID,
			"client_name":      req.ClientName,
			"client_type":      string(req.ClientType),
			"created_by":       "session_context_orchestrator",
			"orchestration_ts": time.Now().Unix(),
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

// rollbackSession attempts to delete a session (rollback operation)
func (o *sessionContextOrchestrator) rollbackSession(ctx context.Context, sessionID string) error {
	if err := o.sessionService.TerminateSession(ctx, sessionID, "orchestration_rollback"); err != nil {
		return errors.Wrap(err, "failed to rollback session")
	}
	return nil
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
