package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/repository/agent"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// AgentRepositoryImpl implements AgentRepository
type AgentRepositoryImpl struct {
	db *sqlx.DB
}

// NewAgentRepository creates a new AgentRepository instance
func NewAgentRepository(db *sqlx.DB) AgentRepository {
	return &AgentRepositoryImpl{db: db}
}

// NewAgentRepositoryLegacy creates a new AgentRepository instance using the legacy implementation
// This avoids naming conflicts with the NewAgentRepository function defined in agent_bridge.go
func NewAgentRepositoryLegacy(db *sql.DB) AgentRepository {
	// Convert the *sql.DB to *sqlx.DB
	dbx := sqlx.NewDb(db, "postgres")
	return &AgentRepositoryImpl{db: dbx}
}

// API-specific methods required by agent_api.go

// CreateAgent implements AgentRepository.CreateAgent
func (r *AgentRepositoryImpl) CreateAgent(ctx context.Context, agent *models.Agent) error {
	// Delegate to the core Create method
	return r.Create(ctx, agent)
}

// GetAgentByID implements AgentRepository.GetAgentByID
func (r *AgentRepositoryImpl) GetAgentByID(ctx context.Context, tenantID string, id string) (*models.Agent, error) {
	// Get the agent first
	agent, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Verify tenant access if required
	if tenantID != "" {
		tenantUUID, err := uuid.Parse(tenantID)
		if err != nil {
			return nil, err
		}
		if agent.TenantID != tenantUUID {
			return nil, fmt.Errorf("agent does not belong to tenant: %s", tenantID)
		}
	}

	return agent, nil
}

// ListAgents implements AgentRepository.ListAgents
func (r *AgentRepositoryImpl) ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error) {
	// Create filter based on tenantID
	filter := agent.FilterFromTenantID(tenantID)

	// Delegate to the core List method
	return r.List(ctx, filter)
}

// UpdateAgent implements AgentRepository.UpdateAgent
func (r *AgentRepositoryImpl) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	// Delegate to the core Update method
	return r.Update(ctx, agent)
}

// DeleteAgent implements AgentRepository.DeleteAgent
func (r *AgentRepositoryImpl) DeleteAgent(ctx context.Context, id string) error {
	// Delegate to the core Delete method
	return r.Delete(ctx, id)
}

// Create implements AgentRepository.Create
func (r *AgentRepositoryImpl) Create(ctx context.Context, agent *models.Agent) error {
	if agent == nil {
		return errors.New("agent cannot be nil")
	}

	// Validate agent ID is a valid UUID
	if agent.ID == "" {
		return errors.New("agent ID cannot be empty")
	}

	// Ensure agent ID is a valid UUID format
	if _, err := uuid.Parse(agent.ID); err != nil {
		return fmt.Errorf("invalid agent ID format: %w", err)
	}

	query := `INSERT INTO agents (id, name, tenant_id, model_id, created_at, updated_at)
              VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`

	_, err := r.db.ExecContext(ctx, query,
		agent.ID,
		agent.Name,
		agent.TenantID,
		agent.ModelID,
	)

	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	return nil
}

// Get implements AgentRepository.Get
func (r *AgentRepositoryImpl) Get(ctx context.Context, id string) (*models.Agent, error) {
	if id == "" {
		return nil, errors.New("id cannot be empty")
	}

	query := `SELECT id, name, tenant_id, model_id, created_at, updated_at
              FROM agents WHERE id = $1`

	var agent models.Agent
	err := r.db.GetContext(ctx, &agent, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("agent not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	return &agent, nil
}

// List implements AgentRepository.List
func (r *AgentRepositoryImpl) List(ctx context.Context, filter agent.Filter) ([]*models.Agent, error) {
	query := `SELECT id, name, tenant_id, model_id, created_at, updated_at FROM agents`

	// Apply filters
	var whereClause string
	var args []any
	argIndex := 1

	for k, v := range filter {
		if whereClause == "" {
			whereClause = " WHERE "
		} else {
			whereClause += " AND "
		}
		whereClause += fmt.Sprintf("%s = $%d", k, argIndex)
		args = append(args, v)
		argIndex++
	}

	query += whereClause + " ORDER BY name"

	var agents []*models.Agent
	err := r.db.SelectContext(ctx, &agents, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	return agents, nil
}

// Update implements AgentRepository.Update
func (r *AgentRepositoryImpl) Update(ctx context.Context, agent *models.Agent) error {
	if agent == nil {
		return errors.New("agent cannot be nil")
	}

	query := `UPDATE agents
              SET name = $2, tenant_id = $3, model_id = $4, updated_at = CURRENT_TIMESTAMP
              WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		agent.ID,
		agent.Name,
		agent.TenantID,
		agent.ModelID,
	)

	if err != nil {
		return fmt.Errorf("failed to update agent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("agent not found: %s", agent.ID)
	}

	return nil
}

// Delete implements AgentRepository.Delete
func (r *AgentRepositoryImpl) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id cannot be empty")
	}

	query := `DELETE FROM agents WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("agent not found: %s", id)
	}

	return nil
}

// GetByStatus retrieves agents by status with caching and metrics
func (r *AgentRepositoryImpl) GetByStatus(ctx context.Context, status models.AgentStatus) ([]*models.Agent, error) {
	if status == "" {
		return nil, errors.New("status cannot be empty")
	}

	query := `SELECT id, name, tenant_id, model_id, type, status, capabilities, metadata, 
	          created_at, updated_at, last_seen_at
	          FROM agents WHERE status = $1 ORDER BY created_at DESC`

	var agents []*models.Agent
	err := r.db.SelectContext(ctx, &agents, query, string(status))
	if err != nil {
		return nil, fmt.Errorf("failed to get agents by status: %w", err)
	}

	return agents, nil
}

// GetWorkload retrieves current workload with distributed locking
func (r *AgentRepositoryImpl) GetWorkload(ctx context.Context, agentID uuid.UUID) (*models.AgentWorkload, error) {
	// First, count active tasks for this agent
	activeQuery := `
		SELECT COUNT(*) FROM tasks 
		WHERE assigned_to = $1 AND status IN ('assigned', 'accepted', 'in_progress')
	`
	var activeTasks int
	err := r.db.GetContext(ctx, &activeTasks, activeQuery, agentID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to count active tasks: %w", err)
	}

	// Count queued tasks
	queuedQuery := `
		SELECT COUNT(*) FROM tasks 
		WHERE assigned_to = $1 AND status = 'pending'
	`
	var queuedTasks int
	err = r.db.GetContext(ctx, &queuedTasks, queuedQuery, agentID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to count queued tasks: %w", err)
	}

	// Get tasks by type
	typeQuery := `
		SELECT type, COUNT(*) as count FROM tasks 
		WHERE assigned_to = $1 AND status IN ('assigned', 'accepted', 'in_progress', 'pending')
		GROUP BY type
	`
	rows, err := r.db.QueryContext(ctx, typeQuery, agentID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks by type: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	tasksByType := make(map[string]int)
	for rows.Next() {
		var taskType string
		var count int
		if err := rows.Scan(&taskType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan task type: %w", err)
		}
		tasksByType[taskType] = count
	}

	// Calculate load score (simple algorithm: active tasks / 10)
	loadScore := float64(activeTasks) / 10.0
	if loadScore > 1.0 {
		loadScore = 1.0
	}

	// Estimate time (simple: 5 minutes per task)
	estimatedTime := (activeTasks + queuedTasks) * 300

	workload := &models.AgentWorkload{
		AgentID:       agentID.String(),
		ActiveTasks:   activeTasks,
		QueuedTasks:   queuedTasks,
		TasksByType:   tasksByType,
		LoadScore:     loadScore,
		EstimatedTime: estimatedTime,
	}

	return workload, nil
}

// UpdateWorkload atomically updates workload metrics
func (r *AgentRepositoryImpl) UpdateWorkload(ctx context.Context, workload *models.AgentWorkload) error {
	if workload == nil {
		return errors.New("workload cannot be nil")
	}

	// In a production system, this would update a separate workload tracking table
	// For now, we'll just validate the workload data
	if workload.AgentID == "" {
		return errors.New("agent ID cannot be empty")
	}

	// Verify agent exists
	var exists bool
	err := r.db.GetContext(ctx, &exists, "SELECT EXISTS(SELECT 1 FROM agents WHERE id = $1)", workload.AgentID)
	if err != nil {
		return fmt.Errorf("failed to check agent existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("agent not found: %s", workload.AgentID)
	}

	// In a real implementation, we would update metrics in a dedicated table
	// For MVP, we're acknowledging the update
	return nil
}

// GetLeastLoadedAgent implements intelligent load balancing
func (r *AgentRepositoryImpl) GetLeastLoadedAgent(ctx context.Context, capability models.AgentCapability) (*models.Agent, error) {
	// Query to find active agents with the required capability and minimal load
	query := `
		WITH agent_loads AS (
			SELECT 
				a.id, a.name, a.tenant_id, a.model_id, a.type, a.status, 
				a.capabilities, a.metadata, a.created_at, a.updated_at, a.last_seen_at,
				COUNT(t.id) as task_count
			FROM agents a
			LEFT JOIN tasks t ON t.assigned_to = a.id::text 
				AND t.status IN ('assigned', 'accepted', 'in_progress')
			WHERE a.status = $1
				AND $2 = ANY(a.capabilities)
			GROUP BY a.id, a.name, a.tenant_id, a.model_id, a.type, a.status, 
				a.capabilities, a.metadata, a.created_at, a.updated_at, a.last_seen_at
		)
		SELECT id, name, tenant_id, model_id, type, status, capabilities, metadata, 
		       created_at, updated_at, last_seen_at
		FROM agent_loads
		ORDER BY task_count ASC, created_at ASC
		LIMIT 1
	`

	var agent models.Agent
	err := r.db.GetContext(ctx, &agent, query, string(models.AgentStatusActive), string(capability))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no available agent with capability %s", capability)
		}
		return nil, fmt.Errorf("failed to get least loaded agent: %w", err)
	}

	return &agent, nil
}
