// Package agent provides interfaces and implementations for agent entities
package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// RepositoryImpl implements the Repository interface for agents
type RepositoryImpl struct {
	db        *sqlx.DB
	tableName string
}

// isSQLite determines if the database is SQLite
func isSQLite(db *sqlx.DB) bool {
	// Check driver name first to avoid unnecessary queries
	driverName := db.DriverName()
	return driverName == "sqlite3"
}

// NewRepository creates a new agent repository
func NewRepository(db *sqlx.DB) Repository {
	// Determine the appropriate table name with schema prefix if needed
	var tableName string

	// Check the driver name to determine database type
	driverName := db.DriverName()
	switch driverName {
	case "sqlite3":
		tableName = "agents"
	case "postgres", "pgx":
		tableName = "mcp.agents"
	default:
		// For unknown drivers, try PostgreSQL version check
		var pgVersion string
		err := db.QueryRow("SELECT version()").Scan(&pgVersion)
		if err == nil && len(pgVersion) > 10 && pgVersion[:10] == "PostgreSQL" {
			tableName = "mcp.agents"
		} else {
			// Default to no schema for unknown databases
			tableName = "agents"
		}
	}

	return &RepositoryImpl{
		db:        db,
		tableName: tableName,
	}
}

// Create creates a new agent
func (r *RepositoryImpl) Create(ctx context.Context, agent *models.Agent) error {
	if agent == nil {
		return errors.New("agent cannot be nil")
	}

	// Validate required fields
	if agent.Name == "" {
		return errors.New("agent name cannot be empty")
	}

	// Check if we need to use a specific transaction from context
	// First try with string key
	tx, ok := ctx.Value("tx").(*sqlx.Tx)
	// If not found, try other common transaction keys
	if !ok || tx == nil {
		tx, ok = ctx.Value("TransactionKey").(*sqlx.Tx)
	}

	// Use appropriate placeholders based on database type
	var placeholders string
	if isSQLite(r.db) {
		placeholders = "?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?"
	} else {
		placeholders = "$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11"
	}

	query := fmt.Sprintf("INSERT INTO %s (id, name, tenant_id, model_id, type, status, capabilities, metadata, created_at, updated_at, last_seen_at) VALUES (%s)",
		r.tableName, placeholders)

	// Set default values if not provided
	if agent.Type == "" {
		agent.Type = "standard"
	}
	if agent.Status == "" {
		agent.Status = "offline"
	}
	if agent.ModelID == "" {
		agent.ModelID = "claude-sonnet-4" // Default to Claude Sonnet 4
	}

	// Convert capabilities to PostgreSQL array format
	// For SQLite, we'll store as JSON string
	var capabilitiesValue interface{}
	if isSQLite(r.db) {
		// SQLite doesn't support arrays, store as JSON
		capBytes, _ := json.Marshal(agent.Capabilities)
		capabilitiesValue = string(capBytes)
	} else {
		// PostgreSQL supports arrays directly
		capabilitiesValue = pq.Array(agent.Capabilities)
	}

	// Convert metadata to JSON
	var metadataValue interface{}
	if agent.Metadata != nil {
		metaBytes, _ := json.Marshal(agent.Metadata)
		metadataValue = string(metaBytes)
	} else {
		metadataValue = "{}"
	}

	// Use transaction if available
	var err error
	if ok && tx != nil {
		_, err = tx.ExecContext(ctx, query,
			agent.ID,
			agent.Name,
			agent.TenantID,
			agent.ModelID,
			agent.Type,
			agent.Status,
			capabilitiesValue,
			metadataValue,
			agent.CreatedAt,
			agent.UpdatedAt,
			agent.LastSeenAt,
		)
	} else {
		_, err = r.db.ExecContext(ctx, query,
			agent.ID,
			agent.Name,
			agent.TenantID,
			agent.ModelID,
			agent.Type,
			agent.Status,
			capabilitiesValue,
			metadataValue,
			agent.CreatedAt,
			agent.UpdatedAt,
			agent.LastSeenAt,
		)
	}

	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	return nil
}

// Get retrieves an agent by ID
func (r *RepositoryImpl) Get(ctx context.Context, id string) (*models.Agent, error) {
	if id == "" {
		return nil, errors.New("id cannot be empty")
	}

	// Check if we need to use a specific transaction from context
	// First try with string key
	tx, ok := ctx.Value("tx").(*sqlx.Tx)
	// If not found, try other common transaction keys
	if !ok || tx == nil {
		tx, ok = ctx.Value("TransactionKey").(*sqlx.Tx)
	}

	// Use appropriate placeholder based on database type
	var placeholder string
	if isSQLite(r.db) {
		placeholder = "?"
	} else {
		placeholder = "$1"
	}

	query := fmt.Sprintf("SELECT id, name, tenant_id, model_id, type, status, capabilities, metadata, created_at, updated_at, last_seen_at FROM %s WHERE id = %s",
		r.tableName, placeholder)

	var agent models.Agent
	var metadataJSON string
	var err error

	// We need to manually scan because of the array and JSON types
	var row *sql.Row
	if ok && tx != nil {
		row = tx.QueryRowContext(ctx, query, id)
	} else {
		row = r.db.QueryRowContext(ctx, query, id)
	}

	if isSQLite(r.db) {
		// SQLite stores capabilities as JSON string
		var capabilitiesJSON string
		err = row.Scan(
			&agent.ID,
			&agent.Name,
			&agent.TenantID,
			&agent.ModelID,
			&agent.Type,
			&agent.Status,
			&capabilitiesJSON,
			&metadataJSON,
			&agent.CreatedAt,
			&agent.UpdatedAt,
			&agent.LastSeenAt,
		)
		if err == nil && capabilitiesJSON != "" {
			if err := json.Unmarshal([]byte(capabilitiesJSON), &agent.Capabilities); err != nil {
				// Log error but continue - capabilities are optional
				fmt.Printf("failed to unmarshal capabilities for agent %s: %v\n", agent.ID, err)
			}
		}
	} else {
		// PostgreSQL with array support
		err = row.Scan(
			&agent.ID,
			&agent.Name,
			&agent.TenantID,
			&agent.ModelID,
			&agent.Type,
			&agent.Status,
			pq.Array(&agent.Capabilities),
			&metadataJSON,
			&agent.CreatedAt,
			&agent.UpdatedAt,
			&agent.LastSeenAt,
		)
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found, return nil without error
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// Parse metadata JSON
	if metadataJSON != "" && metadataJSON != "{}" {
		if err := json.Unmarshal([]byte(metadataJSON), &agent.Metadata); err != nil {
			// Log but don't fail on metadata parse error
			agent.Metadata = make(map[string]interface{})
		}
	} else {
		agent.Metadata = make(map[string]interface{})
	}

	return &agent, nil
}

// List retrieves agents based on filter criteria
func (r *RepositoryImpl) List(ctx context.Context, filter Filter) ([]*models.Agent, error) {
	baseQuery := fmt.Sprintf("SELECT id, name, tenant_id, model_id, type, status, capabilities, metadata, created_at, updated_at, last_seen_at FROM %s", r.tableName)

	// Build the WHERE clause based on filters
	whereClause := ""
	var args []any

	// Check if we need to use a specific transaction from context
	// First try with string key
	tx, ok := ctx.Value("tx").(*sqlx.Tx)
	// If not found, try other common transaction keys
	if !ok || tx == nil {
		tx, ok = ctx.Value("TransactionKey").(*sqlx.Tx)
	}

	if filter != nil {
		argCount := 1
		for key, value := range filter {
			if whereClause == "" {
				whereClause = " WHERE"
			} else {
				whereClause += " AND"
			}

			// For SQLite, always use ? without numbering
			if isSQLite(r.db) {
				whereClause += fmt.Sprintf(" %s = ?", key)
			} else {
				whereClause += fmt.Sprintf(" %s = $%d", key, argCount)
			}

			args = append(args, value)
			argCount++
		}
	}

	// Order by name as a default sort
	query := baseQuery + whereClause + " ORDER BY name ASC"

	var rows *sql.Rows
	var err error

	// Use transaction if available
	if ok && tx != nil {
		rows, err = tx.QueryContext(ctx, query, args...)
	} else {
		rows, err = r.db.QueryContext(ctx, query, args...)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Log the error but don't fail the operation
			fmt.Printf("failed to close rows: %v\n", err)
		}
	}()

	var agents []*models.Agent
	for rows.Next() {
		agent := &models.Agent{}
		var metadataJSON string

		if isSQLite(r.db) {
			// SQLite stores capabilities as JSON string
			var capabilitiesJSON string
			err = rows.Scan(
				&agent.ID,
				&agent.Name,
				&agent.TenantID,
				&agent.ModelID,
				&agent.Type,
				&agent.Status,
				&capabilitiesJSON,
				&metadataJSON,
				&agent.CreatedAt,
				&agent.UpdatedAt,
				&agent.LastSeenAt,
			)
			if err == nil && capabilitiesJSON != "" {
				if err := json.Unmarshal([]byte(capabilitiesJSON), &agent.Capabilities); err != nil {
					// Log error but continue - capabilities are optional
					fmt.Printf("failed to unmarshal capabilities for agent %s: %v\n", agent.ID, err)
				}
			}
		} else {
			// PostgreSQL with array support
			err = rows.Scan(
				&agent.ID,
				&agent.Name,
				&agent.TenantID,
				&agent.ModelID,
				&agent.Type,
				&agent.Status,
				pq.Array(&agent.Capabilities),
				&metadataJSON,
				&agent.CreatedAt,
				&agent.UpdatedAt,
				&agent.LastSeenAt,
			)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to scan agent row: %w", err)
		}

		// Parse metadata JSON
		if metadataJSON != "" && metadataJSON != "{}" {
			if err := json.Unmarshal([]byte(metadataJSON), &agent.Metadata); err != nil {
				// Log but don't fail on metadata parse error
				agent.Metadata = make(map[string]interface{})
			}
		} else {
			agent.Metadata = make(map[string]interface{})
		}

		agents = append(agents, agent)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating agent rows: %w", err)
	}

	return agents, nil
}

// Update updates an existing agent
func (r *RepositoryImpl) Update(ctx context.Context, agent *models.Agent) error {
	if agent == nil {
		return errors.New("agent cannot be nil")
	}

	if agent.ID == "" {
		return errors.New("agent ID cannot be empty")
	}

	// Check if we need to use a specific transaction from context
	// First try with string key
	tx, ok := ctx.Value("tx").(*sqlx.Tx)
	// If not found, try other common transaction keys
	if !ok || tx == nil {
		tx, ok = ctx.Value("TransactionKey").(*sqlx.Tx)
	}

	// Convert capabilities to appropriate format
	var capabilitiesValue interface{}
	if isSQLite(r.db) {
		capBytes, _ := json.Marshal(agent.Capabilities)
		capabilitiesValue = string(capBytes)
	} else {
		capabilitiesValue = pq.Array(agent.Capabilities)
	}

	// Convert metadata to JSON
	var metadataValue interface{}
	if agent.Metadata != nil {
		metaBytes, _ := json.Marshal(agent.Metadata)
		metadataValue = string(metaBytes)
	} else {
		metadataValue = "{}"
	}

	// Choose the appropriate placeholders based on database type
	var placeholders string
	if isSQLite(r.db) {
		placeholders = "name = ?, tenant_id = ?, model_id = ?, type = ?, status = ?, capabilities = ?, metadata = ?, updated_at = ?, last_seen_at = ? WHERE id = ?"
	} else {
		placeholders = "name = $2, tenant_id = $3, model_id = $4, type = $5, status = $6, capabilities = $7, metadata = $8, updated_at = $9, last_seen_at = $10 WHERE id = $1"
	}

	query := fmt.Sprintf("UPDATE %s SET %s", r.tableName, placeholders)

	// Set updated_at to current time
	agent.UpdatedAt = time.Now()

	var args []any
	if isSQLite(r.db) {
		// SQLite uses placeholders in order of appearance
		args = []any{agent.Name, agent.TenantID, agent.ModelID, agent.Type, agent.Status, capabilitiesValue, metadataValue, agent.UpdatedAt, agent.LastSeenAt, agent.ID}
	} else {
		// PostgreSQL uses numbered placeholders
		args = []any{agent.ID, agent.Name, agent.TenantID, agent.ModelID, agent.Type, agent.Status, capabilitiesValue, metadataValue, agent.UpdatedAt, agent.LastSeenAt}
	}

	var result sql.Result
	var err error

	// Use transaction if available
	if ok && tx != nil {
		result, err = tx.ExecContext(ctx, query, args...)
	} else {
		result, err = r.db.ExecContext(ctx, query, args...)
	}

	if err != nil {
		return fmt.Errorf("failed to update agent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("agent not found")
	}

	return nil
}

// Delete deletes an agent by ID
func (r *RepositoryImpl) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id cannot be empty")
	}

	// Check if we need to use a specific transaction from context
	// First try with string key
	tx, ok := ctx.Value("tx").(*sqlx.Tx)
	// If not found, try other common transaction keys
	if !ok || tx == nil {
		tx, ok = ctx.Value("TransactionKey").(*sqlx.Tx)
	}

	// Use appropriate placeholder based on database type
	var placeholder string
	if isSQLite(r.db) {
		placeholder = "?"
	} else {
		placeholder = "$1"
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE id = %s", r.tableName, placeholder)

	var result sql.Result
	var err error

	// Use transaction if available
	if ok && tx != nil {
		result, err = tx.ExecContext(ctx, query, id)
	} else {
		result, err = r.db.ExecContext(ctx, query, id)
	}

	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("agent not found")
	}

	return nil
}

// CreateAgent implements the API-specific method
func (r *RepositoryImpl) CreateAgent(ctx context.Context, agent *models.Agent) error {
	return r.Create(ctx, agent)
}

// GetAgentByID implements the API-specific method
func (r *RepositoryImpl) GetAgentByID(ctx context.Context, tenantID string, id string) (*models.Agent, error) {
	// Get agent by ID first
	agent, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// If found, verify tenant ID matches
	if agent != nil && tenantID != "" {
		tenantUUID, err := uuid.Parse(tenantID)
		if err != nil {
			return nil, err
		}
		if agent.TenantID != tenantUUID {
			return nil, errors.New("agent not found for tenant")
		}
	}

	return agent, nil
}

// ListAgents implements the API-specific method
func (r *RepositoryImpl) ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error) {
	filter := FilterFromTenantID(tenantID)
	return r.List(ctx, filter)
}

// UpdateAgent implements the API-specific method
func (r *RepositoryImpl) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	return r.Update(ctx, agent)
}

// DeleteAgent implements the API-specific method
func (r *RepositoryImpl) DeleteAgent(ctx context.Context, id string) error {
	return r.Delete(ctx, id)
}

// GetByStatus implements the Repository interface
func (r *RepositoryImpl) GetByStatus(ctx context.Context, status models.AgentStatus) ([]*models.Agent, error) {
	if status == "" {
		return nil, errors.New("status cannot be empty")
	}

	// Check if we need to use a specific transaction from context
	tx, ok := ctx.Value("tx").(*sqlx.Tx)
	if !ok || tx == nil {
		tx, ok = ctx.Value("TransactionKey").(*sqlx.Tx)
	}

	// Use appropriate placeholder based on database type
	var placeholder string
	if isSQLite(r.db) {
		placeholder = "?"
	} else {
		placeholder = "$1"
	}

	query := fmt.Sprintf("SELECT id, name, tenant_id, model_id, type, status, capabilities, metadata, created_at, updated_at, last_seen_at FROM %s WHERE status = %s ORDER BY created_at DESC",
		r.tableName, placeholder)

	var rows *sql.Rows
	var err error

	// Use transaction if available
	if ok && tx != nil {
		rows, err = tx.QueryContext(ctx, query, string(status))
	} else {
		rows, err = r.db.QueryContext(ctx, query, string(status))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get agents by status: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Log the error but don't fail the operation
			fmt.Printf("failed to close rows: %v\n", err)
		}
	}()

	var agents []*models.Agent
	for rows.Next() {
		agent := &models.Agent{}
		var metadataJSON string

		if isSQLite(r.db) {
			// SQLite stores capabilities as JSON string
			var capabilitiesJSON string
			err = rows.Scan(
				&agent.ID,
				&agent.Name,
				&agent.TenantID,
				&agent.ModelID,
				&agent.Type,
				&agent.Status,
				&capabilitiesJSON,
				&metadataJSON,
				&agent.CreatedAt,
				&agent.UpdatedAt,
				&agent.LastSeenAt,
			)
			if err == nil && capabilitiesJSON != "" {
				if err := json.Unmarshal([]byte(capabilitiesJSON), &agent.Capabilities); err != nil {
					// Log error but continue - capabilities are optional
					fmt.Printf("failed to unmarshal capabilities for agent %s: %v\n", agent.ID, err)
				}
			}
		} else {
			// PostgreSQL with array support
			err = rows.Scan(
				&agent.ID,
				&agent.Name,
				&agent.TenantID,
				&agent.ModelID,
				&agent.Type,
				&agent.Status,
				pq.Array(&agent.Capabilities),
				&metadataJSON,
				&agent.CreatedAt,
				&agent.UpdatedAt,
				&agent.LastSeenAt,
			)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to scan agent row: %w", err)
		}

		// Parse metadata JSON
		if metadataJSON != "" && metadataJSON != "{}" {
			if err := json.Unmarshal([]byte(metadataJSON), &agent.Metadata); err != nil {
				// Log but don't fail on metadata parse error
				agent.Metadata = make(map[string]interface{})
			}
		} else {
			agent.Metadata = make(map[string]interface{})
		}

		agents = append(agents, agent)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating agent rows: %w", err)
	}

	return agents, nil
}

// GetWorkload implements the Repository interface
func (r *RepositoryImpl) GetWorkload(ctx context.Context, agentID uuid.UUID) (*models.AgentWorkload, error) {
	// For now, return a simple workload calculation
	// In production, this would query actual task counts and metrics
	return &models.AgentWorkload{
		AgentID:       agentID.String(),
		ActiveTasks:   0,
		QueuedTasks:   0,
		TasksByType:   make(map[string]int),
		LoadScore:     0.0,
		EstimatedTime: 0,
	}, nil
}

// UpdateWorkload implements the Repository interface
func (r *RepositoryImpl) UpdateWorkload(ctx context.Context, workload *models.AgentWorkload) error {
	if workload == nil {
		return errors.New("workload cannot be nil")
	}
	// For MVP, this is a no-op as we don't track workload in a separate table yet
	return nil
}

// GetLeastLoadedAgent implements the Repository interface
func (r *RepositoryImpl) GetLeastLoadedAgent(ctx context.Context, capability models.AgentCapability) (*models.Agent, error) {
	// Check if we need to use a specific transaction from context
	tx, ok := ctx.Value("tx").(*sqlx.Tx)
	if !ok || tx == nil {
		tx, ok = ctx.Value("TransactionKey").(*sqlx.Tx)
	}

	// For MVP, we'll just get the first active agent
	// In production, this would perform complex load balancing queries
	var placeholder string
	if isSQLite(r.db) {
		placeholder = "?"
	} else {
		placeholder = "$1"
	}

	query := fmt.Sprintf("SELECT id, name, tenant_id, model_id, type, status, capabilities, metadata, created_at, updated_at, last_seen_at FROM %s WHERE status = %s ORDER BY name ASC LIMIT 1",
		r.tableName, placeholder)

	var agent models.Agent
	var metadataJSON string
	var err error

	// We need to manually scan because of the array and JSON types
	var row *sql.Row
	if ok && tx != nil {
		row = tx.QueryRowContext(ctx, query, string(models.AgentStatusActive))
	} else {
		row = r.db.QueryRowContext(ctx, query, string(models.AgentStatusActive))
	}

	if isSQLite(r.db) {
		// SQLite stores capabilities as JSON string
		var capabilitiesJSON string
		err = row.Scan(
			&agent.ID,
			&agent.Name,
			&agent.TenantID,
			&agent.ModelID,
			&agent.Type,
			&agent.Status,
			&capabilitiesJSON,
			&metadataJSON,
			&agent.CreatedAt,
			&agent.UpdatedAt,
			&agent.LastSeenAt,
		)
		if err == nil && capabilitiesJSON != "" {
			if err := json.Unmarshal([]byte(capabilitiesJSON), &agent.Capabilities); err != nil {
				// Log error but continue - capabilities are optional
				fmt.Printf("failed to unmarshal capabilities for agent %s: %v\n", agent.ID, err)
			}
		}
	} else {
		// PostgreSQL with array support
		err = row.Scan(
			&agent.ID,
			&agent.Name,
			&agent.TenantID,
			&agent.ModelID,
			&agent.Type,
			&agent.Status,
			pq.Array(&agent.Capabilities),
			&metadataJSON,
			&agent.CreatedAt,
			&agent.UpdatedAt,
			&agent.LastSeenAt,
		)
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no available agent with capability %s", capability)
		}
		return nil, fmt.Errorf("failed to get least loaded agent: %w", err)
	}

	// Parse metadata JSON
	if metadataJSON != "" && metadataJSON != "{}" {
		if err := json.Unmarshal([]byte(metadataJSON), &agent.Metadata); err != nil {
			// Log but don't fail on metadata parse error
			agent.Metadata = make(map[string]interface{})
		}
	} else {
		agent.Metadata = make(map[string]interface{})
	}

	return &agent, nil
}
