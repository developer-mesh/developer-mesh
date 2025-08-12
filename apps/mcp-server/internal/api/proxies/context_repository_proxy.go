package proxies

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/jmoiron/sqlx"
	// "github.com/lib/pq" // TODO: uncomment when ensureTablesExist is used
)

// ContextRepositoryProxy provides a database-backed implementation of ContextRepository
type ContextRepositoryProxy struct {
	db     *sqlx.DB
	logger observability.Logger
}

// NewContextRepositoryProxy creates a new context repository proxy
func NewContextRepositoryProxy(db *sqlx.DB, logger observability.Logger) repository.ContextRepository {
	if logger == nil {
		logger = observability.NewLogger("context-repository-proxy")
	}

	return &ContextRepositoryProxy{
		db:     db,
		logger: logger,
	}
}

// Create creates a new context
func (r *ContextRepositoryProxy) Create(ctx context.Context, contextObj *repository.Context) error {
	query := `
		INSERT INTO mcp.contexts (id, name, agent_id, session_id, status, properties, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			status = EXCLUDED.status,
			properties = EXCLUDED.properties,
			updated_at = EXCLUDED.updated_at
	`

	now := time.Now().Unix()
	contextObj.CreatedAt = now
	contextObj.UpdatedAt = now

	propsJSON, err := json.Marshal(contextObj.Properties)
	if err != nil {
		return fmt.Errorf("failed to marshal properties: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		contextObj.ID,
		contextObj.Name,
		contextObj.AgentID,
		contextObj.SessionID,
		contextObj.Status,
		propsJSON,
		contextObj.CreatedAt,
		contextObj.UpdatedAt,
	)

	if err != nil {
		r.logger.Error("Failed to create context", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextObj.ID,
		})
		return fmt.Errorf("failed to create context: %w", err)
	}

	return nil
}

// Get retrieves a context by ID
func (r *ContextRepositoryProxy) Get(ctx context.Context, id string) (*repository.Context, error) {
	query := `
		SELECT id, name, agent_id, session_id, status, properties, created_at, updated_at
		FROM mcp.contexts
		WHERE id = $1
	`

	var contextObj repository.Context
	var propsJSON []byte

	err := r.db.QueryRowxContext(ctx, query, id).Scan(
		&contextObj.ID,
		&contextObj.Name,
		&contextObj.AgentID,
		&contextObj.SessionID,
		&contextObj.Status,
		&propsJSON,
		&contextObj.CreatedAt,
		&contextObj.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("context not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get context: %w", err)
	}

	if len(propsJSON) > 0 {
		if err := json.Unmarshal(propsJSON, &contextObj.Properties); err != nil {
			r.logger.Warn("Failed to unmarshal context properties", map[string]interface{}{
				"error":      err.Error(),
				"context_id": id,
			})
			contextObj.Properties = make(map[string]interface{})
		}
	} else {
		contextObj.Properties = make(map[string]interface{})
	}

	return &contextObj, nil
}

// Update updates an existing context
func (r *ContextRepositoryProxy) Update(ctx context.Context, contextObj *repository.Context) error {
	query := `
		UPDATE mcp.contexts
		SET name = $2, status = $3, properties = $4, updated_at = $5
		WHERE id = $1
	`

	contextObj.UpdatedAt = time.Now().Unix()

	propsJSON, err := json.Marshal(contextObj.Properties)
	if err != nil {
		return fmt.Errorf("failed to marshal properties: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query,
		contextObj.ID,
		contextObj.Name,
		contextObj.Status,
		propsJSON,
		contextObj.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update context: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("context not found: %s", contextObj.ID)
	}

	return nil
}

// Delete deletes a context by ID
func (r *ContextRepositoryProxy) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM mcp.contexts WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete context: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("context not found: %s", id)
	}

	return nil
}

// List lists contexts with optional filtering
func (r *ContextRepositoryProxy) List(ctx context.Context, filter map[string]interface{}) ([]*repository.Context, error) {
	query := `
		SELECT id, name, agent_id, session_id, status, properties, created_at, updated_at
		FROM mcp.contexts
		WHERE 1=1
	`

	var args []interface{}
	argCount := 0

	// Add filters dynamically
	if agentID, ok := filter["agent_id"].(string); ok && agentID != "" {
		argCount++
		query += fmt.Sprintf(" AND agent_id = $%d", argCount)
		args = append(args, agentID)
	}

	if sessionID, ok := filter["session_id"].(string); ok && sessionID != "" {
		argCount++
		query += fmt.Sprintf(" AND session_id = $%d", argCount)
		args = append(args, sessionID)
	}

	if status, ok := filter["status"].(string); ok && status != "" {
		argCount++
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, status)
	}

	query += " ORDER BY created_at DESC LIMIT 100"

	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list contexts: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			r.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	var contexts []*repository.Context
	for rows.Next() {
		var contextObj repository.Context
		var propsJSON []byte

		err := rows.Scan(
			&contextObj.ID,
			&contextObj.Name,
			&contextObj.AgentID,
			&contextObj.SessionID,
			&contextObj.Status,
			&propsJSON,
			&contextObj.CreatedAt,
			&contextObj.UpdatedAt,
		)

		if err != nil {
			r.logger.Warn("Failed to scan context row", map[string]interface{}{
				"error": err.Error(),
			})
			continue
		}

		if len(propsJSON) > 0 {
			if err := json.Unmarshal(propsJSON, &contextObj.Properties); err != nil {
				r.logger.Warn("Failed to unmarshal context properties", map[string]interface{}{
					"error": err.Error(),
				})
				contextObj.Properties = make(map[string]interface{})
			}
		} else {
			contextObj.Properties = make(map[string]interface{})
		}

		contexts = append(contexts, &contextObj)
	}

	return contexts, nil
}

// Search searches for text within a context using vector similarity
func (r *ContextRepositoryProxy) Search(ctx context.Context, contextID, query string) ([]repository.ContextItem, error) {
	// For now, return a simple text search. In production, this would use pgvector
	searchQuery := `
		SELECT id, context_id, content, type, metadata
		FROM mcp.context_items
		WHERE context_id = $1 AND content ILIKE $2
		ORDER BY created_at DESC
		LIMIT 20
	`

	searchPattern := "%" + query + "%"
	rows, err := r.db.QueryxContext(ctx, searchQuery, contextID, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search context: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			r.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	var items []repository.ContextItem
	for rows.Next() {
		var item repository.ContextItem
		var metadataJSON []byte

		err := rows.Scan(
			&item.ID,
			&item.ContextID,
			&item.Content,
			&item.Type,
			&metadataJSON,
		)

		if err != nil {
			r.logger.Warn("Failed to scan context item", map[string]interface{}{
				"error": err.Error(),
			})
			continue
		}

		// Calculate a simple relevance score based on match position
		score := 1.0
		if idx := len(item.Content) - len(query); idx > 0 {
			score = 1.0 - (float64(idx) / float64(len(item.Content)))
		}
		item.Score = score

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &item.Metadata); err != nil {
				item.Metadata = make(map[string]interface{})
			}
		} else {
			item.Metadata = make(map[string]interface{})
		}

		items = append(items, item)
	}

	return items, nil
}

// Summarize generates a summary of a context
func (r *ContextRepositoryProxy) Summarize(ctx context.Context, contextID string) (string, error) {
	// Retrieve recent context items
	query := `
		SELECT content, type
		FROM mcp.context_items
		WHERE context_id = $1
		ORDER BY created_at DESC
		LIMIT 10
	`

	rows, err := r.db.QueryxContext(ctx, query, contextID)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve context items: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			r.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	var contents []string
	for rows.Next() {
		var content, itemType string
		if err := rows.Scan(&content, &itemType); err != nil {
			continue
		}
		contents = append(contents, fmt.Sprintf("[%s] %s", itemType, content))
	}

	if len(contents) == 0 {
		return "No context items found", nil
	}

	// Simple summary: combine the first few items
	summary := fmt.Sprintf("Context Summary (last %d items):\n", len(contents))
	for i, content := range contents {
		if i >= 5 { // Limit summary to 5 items
			summary += fmt.Sprintf("... and %d more items", len(contents)-5)
			break
		}
		// Truncate long content
		if len(content) > 100 {
			content = content[:100] + "..."
		}
		summary += fmt.Sprintf("- %s\n", content)
	}

	return summary, nil
}

// ensureTablesExist ensures the required tables exist
// TODO: This function is not currently used. Tables should be created via migrations.
// Keeping for reference in case manual table creation is needed.
/*
func (r *ContextRepositoryProxy) ensureTablesExist(ctx context.Context) error {
	// Create contexts table if it doesn't exist
	contextTableQuery := `
		CREATE TABLE IF NOT EXISTS mcp.contexts (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			agent_id VARCHAR(255) NOT NULL,
			session_id VARCHAR(255) NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'active',
			properties JSONB,
			created_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL,
			INDEX idx_agent_id (agent_id),
			INDEX idx_session_id (session_id),
			INDEX idx_status (status)
		)
	`

	// Create context_items table if it doesn't exist
	itemsTableQuery := `
		CREATE TABLE IF NOT EXISTS mcp.context_items (
			id VARCHAR(255) PRIMARY KEY,
			context_id VARCHAR(255) NOT NULL REFERENCES mcp.contexts(id) ON DELETE CASCADE,
			content TEXT NOT NULL,
			type VARCHAR(50) NOT NULL,
			metadata JSONB,
			embedding vector(1536),
			created_at BIGINT NOT NULL,
			INDEX idx_context_id (context_id),
			INDEX idx_type (type)
		)
	`

	// Execute table creation queries
	if _, err := r.db.ExecContext(ctx, contextTableQuery); err != nil {
		// PostgreSQL doesn't support IF NOT EXISTS with INDEX, so ignore duplicate index errors
		if pqErr, ok := err.(*pq.Error); !ok || pqErr.Code != "42P07" {
			r.logger.Warn("Context table may already exist", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	if _, err := r.db.ExecContext(ctx, itemsTableQuery); err != nil {
		if pqErr, ok := err.(*pq.Error); !ok || pqErr.Code != "42P07" {
			r.logger.Warn("Context items table may already exist", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	return nil
}
*/
