package agents

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// EnhancedRepository provides production-grade agent persistence with retry logic
type EnhancedRepository struct {
	db         *sqlx.DB
	schema     string
	maxRetries int
	retryDelay time.Duration
}

// NewEnhancedRepository creates a new enhanced repository
func NewEnhancedRepository(db *sqlx.DB, schema string) *EnhancedRepository {
	if schema == "" {
		schema = "mcp"
	}
	return &EnhancedRepository{
		db:         db,
		schema:     schema,
		maxRetries: 3,
		retryDelay: 100 * time.Millisecond,
	}
}

// CreateAgent creates a new agent with automatic state initialization
func (r *EnhancedRepository) CreateAgent(ctx context.Context, agent *Agent) error {
	// Set defaults
	if agent.ID == uuid.Nil {
		agent.ID = uuid.New()
	}
	if agent.State == "" {
		agent.State = StatePending
	}
	if agent.Version == 0 {
		agent.Version = 1
	}
	if agent.MaxWorkload == 0 {
		agent.MaxWorkload = 10
	}
	if agent.Temperature == 0 {
		agent.Temperature = 0.7
	}
	if agent.MaxTokens == 0 {
		agent.MaxTokens = 4096
	}

	now := time.Now()
	agent.CreatedAt = now
	agent.UpdatedAt = now
	agent.StateChangedAt = now

	// Convert slices and maps to database types
	configJSON, err := json.Marshal(agent.Configuration)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	healthJSON, err := json.Marshal(agent.HealthStatus)
	if err != nil {
		return fmt.Errorf("failed to marshal health status: %w", err)
	}

	metadataJSON, err := json.Marshal(agent.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s.agents (
			id, tenant_id, name, type, state, state_reason, state_changed_at,
			model_id, capabilities, configuration, system_prompt,
			temperature, max_tokens, current_workload, max_workload,
			health_status, metadata, version, activation_count,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
		)
	`, r.schema)

	err = r.executeWithRetry(ctx, func() error {
		_, err := r.db.ExecContext(
			ctx, query,
			agent.ID, agent.TenantID, agent.Name, agent.Type,
			agent.State, agent.StateReason, agent.StateChangedAt,
			agent.ModelID, pq.Array(agent.Capabilities), configJSON,
			agent.SystemPrompt, agent.Temperature, agent.MaxTokens,
			agent.CurrentWorkload, agent.MaxWorkload,
			healthJSON, metadataJSON, agent.Version, agent.ActivationCount,
			agent.CreatedAt, agent.UpdatedAt,
		)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Record creation event
	event := &AgentEvent{
		ID:           uuid.New(),
		AgentID:      agent.ID,
		TenantID:     agent.TenantID,
		EventType:    "agent.created",
		EventVersion: "1.0.0",
		ToState:      &agent.State,
		Payload: map[string]interface{}{
			"name": agent.Name,
			"type": agent.Type,
		},
		CreatedAt: now,
	}

	_ = r.RecordEvent(ctx, event) // Best effort event recording

	return nil
}

// GetAgent retrieves an agent by ID
func (r *EnhancedRepository) GetAgent(ctx context.Context, id uuid.UUID) (*Agent, error) {
	query := fmt.Sprintf(`
		SELECT 
			id, tenant_id, name, type, state, state_reason, state_changed_at, state_changed_by,
			model_id, capabilities, configuration, system_prompt, temperature, max_tokens,
			current_workload, max_workload, health_status, health_checked_at,
			last_error, last_error_at, retry_count, version, activation_count,
			last_seen_at, metadata, created_at, updated_at
		FROM %s.agents
		WHERE id = $1
	`, r.schema)

	var agent Agent
	var configJSON, healthJSON, metadataJSON []byte
	var capabilities pq.StringArray

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&agent.ID, &agent.TenantID, &agent.Name, &agent.Type,
		&agent.State, &agent.StateReason, &agent.StateChangedAt, &agent.StateChangedBy,
		&agent.ModelID, &capabilities, &configJSON, &agent.SystemPrompt,
		&agent.Temperature, &agent.MaxTokens, &agent.CurrentWorkload, &agent.MaxWorkload,
		&healthJSON, &agent.HealthCheckedAt, &agent.LastError, &agent.LastErrorAt,
		&agent.RetryCount, &agent.Version, &agent.ActivationCount,
		&agent.LastSeenAt, &metadataJSON, &agent.CreatedAt, &agent.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("agent not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// Convert database types back
	agent.Capabilities = []string(capabilities)

	if err := json.Unmarshal(configJSON, &agent.Configuration); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	if err := json.Unmarshal(healthJSON, &agent.HealthStatus); err != nil {
		return nil, fmt.Errorf("failed to unmarshal health status: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &agent.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &agent, nil
}

// UpdateAgent updates an existing agent
func (r *EnhancedRepository) UpdateAgent(ctx context.Context, id uuid.UUID, update *UpdateAgentRequest) (*Agent, error) {
	// Get current agent
	current, err := r.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if update.Name != nil {
		current.Name = *update.Name
	}
	if update.ModelID != nil {
		current.ModelID = *update.ModelID
	}
	if update.Capabilities != nil {
		current.Capabilities = *update.Capabilities
	}
	if update.Config != nil {
		current.Configuration = *update.Config
	}
	if update.SystemPrompt != nil {
		current.SystemPrompt = *update.SystemPrompt
	}
	if update.Temperature != nil {
		current.Temperature = *update.Temperature
	}
	if update.MaxTokens != nil {
		current.MaxTokens = *update.MaxTokens
	}
	if update.MaxWorkload != nil {
		current.MaxWorkload = *update.MaxWorkload
	}
	if update.Metadata != nil {
		current.Metadata = *update.Metadata
	}

	current.UpdatedAt = time.Now()
	current.Version++

	// Convert to JSON
	configJSON, _ := json.Marshal(current.Configuration)
	metadataJSON, _ := json.Marshal(current.Metadata)

	query := fmt.Sprintf(`
		UPDATE %s.agents SET
			name = $2, model_id = $3, capabilities = $4, configuration = $5,
			system_prompt = $6, temperature = $7, max_tokens = $8, max_workload = $9,
			metadata = $10, version = $11, updated_at = $12
		WHERE id = $1
		RETURNING version
	`, r.schema)

	err = r.executeWithRetry(ctx, func() error {
		return r.db.QueryRowContext(
			ctx, query,
			id, current.Name, current.ModelID, pq.Array(current.Capabilities),
			configJSON, current.SystemPrompt, current.Temperature, current.MaxTokens,
			current.MaxWorkload, metadataJSON, current.Version, current.UpdatedAt,
		).Scan(&current.Version)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	return current, nil
}

// TransitionState transitions an agent to a new state
func (r *EnhancedRepository) TransitionState(ctx context.Context, id uuid.UUID, req *StateTransitionRequest) error {
	// Start transaction
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Get current state with lock
	var currentState AgentState
	var tenantID uuid.UUID
	query := fmt.Sprintf(`
		SELECT state, tenant_id FROM %s.agents 
		WHERE id = $1 
		FOR UPDATE
	`, r.schema)

	err = tx.QueryRowContext(ctx, query, id).Scan(&currentState, &tenantID)
	if err != nil {
		return fmt.Errorf("failed to get current state: %w", err)
	}

	// Validate transition
	if !currentState.CanTransitionTo(req.TargetState) {
		return fmt.Errorf("invalid transition from %s to %s", currentState, req.TargetState)
	}

	// Update state
	updateQuery := fmt.Sprintf(`
		UPDATE %s.agents SET
			state = $2, state_reason = $3, state_changed_at = $4,
			state_changed_by = $5, version = version + 1
		WHERE id = $1
	`, r.schema)

	now := time.Now()
	_, err = tx.ExecContext(
		ctx, updateQuery,
		id, req.TargetState, req.Reason, now, req.InitiatedBy,
	)
	if err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	// Record event (within transaction)
	event := &AgentEvent{
		ID:           uuid.New(),
		AgentID:      id,
		TenantID:     tenantID,
		EventType:    "state.transition",
		EventVersion: "1.0.0",
		FromState:    &currentState,
		ToState:      &req.TargetState,
		Payload: map[string]interface{}{
			"reason": req.Reason,
		},
		InitiatedBy: &req.InitiatedBy,
		CreatedAt:   now,
	}

	if err := r.recordEventTx(ctx, tx, event); err != nil {
		return fmt.Errorf("failed to record event: %w", err)
	}

	// Handle special state transitions
	if req.TargetState == StateActive {
		// Increment activation count
		_, err = tx.ExecContext(ctx,
			fmt.Sprintf("UPDATE %s.agents SET activation_count = activation_count + 1 WHERE id = $1", r.schema),
			id,
		)
		if err != nil {
			return fmt.Errorf("failed to update activation count: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ListAgents lists agents with filtering
func (r *EnhancedRepository) ListAgents(ctx context.Context, filter AgentFilter) ([]*Agent, error) {
	query := fmt.Sprintf(`
		SELECT 
			id, tenant_id, name, type, state, state_reason, state_changed_at,
			model_id, capabilities, configuration, system_prompt, temperature, max_tokens,
			current_workload, max_workload, health_status, version, activation_count,
			metadata, created_at, updated_at
		FROM %s.agents
		WHERE 1=1
	`, r.schema)

	args := []interface{}{}
	argCount := 0

	// Apply filters
	if filter.TenantID != nil {
		argCount++
		query += fmt.Sprintf(" AND tenant_id = $%d", argCount)
		args = append(args, *filter.TenantID)
	}

	if filter.Type != nil {
		argCount++
		query += fmt.Sprintf(" AND type = $%d", argCount)
		args = append(args, *filter.Type)
	}

	if filter.State != nil {
		argCount++
		query += fmt.Sprintf(" AND state = $%d", argCount)
		args = append(args, *filter.State)
	}

	if len(filter.States) > 0 {
		argCount++
		query += fmt.Sprintf(" AND state = ANY($%d)", argCount)
		states := make([]string, len(filter.States))
		for i, s := range filter.States {
			states[i] = string(s)
		}
		args = append(args, pq.Array(states))
	}

	if filter.IsAvailable != nil && *filter.IsAvailable {
		query += " AND state IN ('active', 'degraded') AND current_workload < max_workload"
	}

	// Order and pagination
	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, filter.Limit)
	}

	if filter.Offset > 0 {
		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	agents := []*Agent{}
	for rows.Next() {
		var agent Agent
		var configJSON, healthJSON, metadataJSON []byte
		var capabilities pq.StringArray

		err := rows.Scan(
			&agent.ID, &agent.TenantID, &agent.Name, &agent.Type,
			&agent.State, &agent.StateReason, &agent.StateChangedAt,
			&agent.ModelID, &capabilities, &configJSON, &agent.SystemPrompt,
			&agent.Temperature, &agent.MaxTokens, &agent.CurrentWorkload, &agent.MaxWorkload,
			&healthJSON, &agent.Version, &agent.ActivationCount,
			&metadataJSON, &agent.CreatedAt, &agent.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent: %w", err)
		}

		agent.Capabilities = []string(capabilities)
		_ = json.Unmarshal(configJSON, &agent.Configuration)
		_ = json.Unmarshal(healthJSON, &agent.HealthStatus)
		_ = json.Unmarshal(metadataJSON, &agent.Metadata)

		agents = append(agents, &agent)
	}

	return agents, nil
}

// RecordEvent records an agent event
func (r *EnhancedRepository) RecordEvent(ctx context.Context, event *AgentEvent) error {
	return r.recordEventTx(ctx, r.db, event)
}

// recordEventTx records an event within a transaction
func (r *EnhancedRepository) recordEventTx(ctx context.Context, tx sqlx.ExtContext, event *AgentEvent) error {
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if event.EventVersion == "" {
		event.EventVersion = "1.0.0"
	}

	payloadJSON, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s.agent_events (
			id, agent_id, tenant_id, event_type, event_version,
			from_state, to_state, payload, error_message, error_code,
			initiated_by, correlation_id, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, r.schema)

	_, err = tx.ExecContext(
		ctx, query,
		event.ID, event.AgentID, event.TenantID, event.EventType, event.EventVersion,
		event.FromState, event.ToState, payloadJSON, event.ErrorMessage, event.ErrorCode,
		event.InitiatedBy, event.CorrelationID, event.CreatedAt,
	)

	return err
}

// GetEvents retrieves events for an agent
func (r *EnhancedRepository) GetEvents(ctx context.Context, filter EventFilter) ([]*AgentEvent, error) {
	query := fmt.Sprintf(`
		SELECT 
			id, agent_id, tenant_id, event_type, event_version,
			from_state, to_state, payload, error_message, error_code,
			initiated_by, correlation_id, created_at
		FROM %s.agent_events
		WHERE 1=1
	`, r.schema)

	args := []interface{}{}
	argCount := 0

	if filter.AgentID != nil {
		argCount++
		query += fmt.Sprintf(" AND agent_id = $%d", argCount)
		args = append(args, *filter.AgentID)
	}

	if filter.TenantID != nil {
		argCount++
		query += fmt.Sprintf(" AND tenant_id = $%d", argCount)
		args = append(args, *filter.TenantID)
	}

	if filter.EventType != nil {
		argCount++
		query += fmt.Sprintf(" AND event_type = $%d", argCount)
		args = append(args, *filter.EventType)
	}

	if filter.StartTime != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at >= $%d", argCount)
		args = append(args, *filter.StartTime)
	}

	if filter.EndTime != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at <= $%d", argCount)
		args = append(args, *filter.EndTime)
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, filter.Limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	events := []*AgentEvent{}
	for rows.Next() {
		var event AgentEvent
		var payloadJSON []byte

		err := rows.Scan(
			&event.ID, &event.AgentID, &event.TenantID, &event.EventType, &event.EventVersion,
			&event.FromState, &event.ToState, &payloadJSON, &event.ErrorMessage, &event.ErrorCode,
			&event.InitiatedBy, &event.CorrelationID, &event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		_ = json.Unmarshal(payloadJSON, &event.Payload)
		events = append(events, &event)
	}

	return events, nil
}

// UpdateHealth updates agent health status
func (r *EnhancedRepository) UpdateHealth(ctx context.Context, id uuid.UUID, health map[string]interface{}) error {
	healthJSON, err := json.Marshal(health)
	if err != nil {
		return fmt.Errorf("failed to marshal health: %w", err)
	}

	query := fmt.Sprintf(`
		UPDATE %s.agents 
		SET health_status = $2, health_checked_at = $3
		WHERE id = $1
	`, r.schema)

	_, err = r.db.ExecContext(ctx, query, id, healthJSON, time.Now())
	return err
}

// executeWithRetry executes a function with retry logic
func (r *EnhancedRepository) executeWithRetry(ctx context.Context, fn func() error) error {
	var lastErr error

	for i := 0; i < r.maxRetries; i++ {
		if err := fn(); err != nil {
			lastErr = err

			// Check if error is retryable
			if !isRetryableError(err) {
				return err
			}

			// Wait before retry
			select {
			case <-time.After(r.retryDelay * time.Duration(i+1)):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error) bool {
	// Add database-specific retry logic here
	// For example, deadlock errors, connection errors, etc.
	return false // Conservative: don't retry by default
}
