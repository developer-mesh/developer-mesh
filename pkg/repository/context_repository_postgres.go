// Package repository provides data access implementations
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// PostgresContextRepository implements ContextRepository using PostgreSQL
type PostgresContextRepository struct {
	db *sqlx.DB
}

// NewPostgresContextRepository creates a new PostgreSQL context repository
func NewPostgresContextRepository(db *sqlx.DB) ContextRepository {
	return &PostgresContextRepository{db: db}
}

// Create creates a new context
func (r *PostgresContextRepository) Create(ctx context.Context, contextObj *Context) error {
	propertiesJSON, err := json.Marshal(contextObj.Properties)
	if err != nil {
		return fmt.Errorf("failed to marshal properties: %w", err)
	}

	query := `
		INSERT INTO mcp.contexts (id, tenant_id, type, agent_id, model_id, token_count, max_tokens, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err = r.db.ExecContext(ctx, query,
		contextObj.ID,
		contextObj.AgentID, // Using agent_id as tenant_id for now
		"conversation",     // Default type
		contextObj.AgentID,
		nil, // model_id
		0,   // token_count
		0,   // max_tokens
		propertiesJSON,
		time.Now(),
		time.Now(),
	)

	return err
}

// Get retrieves a context by ID
func (r *PostgresContextRepository) Get(ctx context.Context, id string) (*Context, error) {
	var dbContext struct {
		ID        string          `db:"id"`
		TenantID  string          `db:"tenant_id"`
		AgentID   *string         `db:"agent_id"`
		Metadata  json.RawMessage `db:"metadata"`
		CreatedAt time.Time       `db:"created_at"`
		UpdatedAt time.Time       `db:"updated_at"`
	}

	query := `SELECT id, tenant_id, agent_id, metadata, created_at, updated_at FROM mcp.contexts WHERE id = $1`
	err := r.db.GetContext(ctx, &dbContext, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("context not found: %s", id)
		}
		return nil, err
	}

	result := &Context{
		ID:        dbContext.ID,
		AgentID:   dbContext.TenantID,
		SessionID: "",
		Status:    "active",
		CreatedAt: dbContext.CreatedAt.Unix(),
		UpdatedAt: dbContext.UpdatedAt.Unix(),
	}

	if dbContext.AgentID != nil {
		result.AgentID = *dbContext.AgentID
	}

	if len(dbContext.Metadata) > 0 {
		if err := json.Unmarshal(dbContext.Metadata, &result.Properties); err != nil {
			// Log but don't fail
			result.Properties = make(map[string]any)
		}
	}

	return result, nil
}

// Update updates an existing context
func (r *PostgresContextRepository) Update(ctx context.Context, contextObj *Context) error {
	propertiesJSON, err := json.Marshal(contextObj.Properties)
	if err != nil {
		return fmt.Errorf("failed to marshal properties: %w", err)
	}

	query := `UPDATE mcp.contexts SET metadata = $1, updated_at = $2 WHERE id = $3`
	_, err = r.db.ExecContext(ctx, query, propertiesJSON, time.Now(), contextObj.ID)
	return err
}

// Delete deletes a context by ID
func (r *PostgresContextRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM mcp.contexts WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// List lists contexts with optional filtering
func (r *PostgresContextRepository) List(ctx context.Context, filter map[string]any) ([]*Context, error) {
	query := `SELECT id, tenant_id, agent_id, metadata, created_at, updated_at FROM mcp.contexts WHERE 1=1`
	args := []any{}

	if agentID, ok := filter["agent_id"].(string); ok && agentID != "" {
		query += " AND agent_id = $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, agentID)
	}

	if sessionID, ok := filter["session_id"].(string); ok && sessionID != "" {
		query += " AND session_id = $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, sessionID)
	}

	query += " ORDER BY created_at DESC LIMIT 100"

	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []*Context
	for rows.Next() {
		var dbContext struct {
			ID        string          `db:"id"`
			TenantID  string          `db:"tenant_id"`
			AgentID   *string         `db:"agent_id"`
			Metadata  json.RawMessage `db:"metadata"`
			CreatedAt time.Time       `db:"created_at"`
			UpdatedAt time.Time       `db:"updated_at"`
		}

		if err := rows.StructScan(&dbContext); err != nil {
			continue
		}

		result := &Context{
			ID:        dbContext.ID,
			AgentID:   dbContext.TenantID,
			Status:    "active",
			CreatedAt: dbContext.CreatedAt.Unix(),
			UpdatedAt: dbContext.UpdatedAt.Unix(),
		}

		if dbContext.AgentID != nil {
			result.AgentID = *dbContext.AgentID
		}

		if len(dbContext.Metadata) > 0 {
			_ = json.Unmarshal(dbContext.Metadata, &result.Properties)
		}

		results = append(results, result)
	}

	return results, nil
}

// Search searches for text within a context
func (r *PostgresContextRepository) Search(ctx context.Context, contextID, query string) ([]ContextItem, error) {
	sqlQuery := `
		SELECT id, context_id, content, type, metadata
		FROM mcp.context_items
		WHERE context_id = $1 AND content ILIKE $2
		ORDER BY sequence_number DESC
		LIMIT 50
	`

	rows, err := r.db.QueryxContext(ctx, sqlQuery, contextID, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []ContextItem
	for rows.Next() {
		var item struct {
			ID        string          `db:"id"`
			ContextID string          `db:"context_id"`
			Content   string          `db:"content"`
			Type      string          `db:"type"`
			Metadata  json.RawMessage `db:"metadata"`
		}

		if err := rows.StructScan(&item); err != nil {
			continue
		}

		result := ContextItem{
			ID:        item.ID,
			ContextID: item.ContextID,
			Content:   item.Content,
			Type:      item.Type,
		}

		if len(item.Metadata) > 0 {
			_ = json.Unmarshal(item.Metadata, &result.Metadata)
		}

		results = append(results, result)
	}

	return results, nil
}

// Summarize generates a summary of a context
func (r *PostgresContextRepository) Summarize(ctx context.Context, contextID string) (string, error) {
	// Simple implementation - returns item count
	var count int
	query := `SELECT COUNT(*) FROM mcp.context_items WHERE context_id = $1`
	err := r.db.GetContext(ctx, &count, query, contextID)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Context %s contains %d items", contextID, count), nil
}

// AddContextItem adds a single item to a context
func (r *PostgresContextRepository) AddContextItem(ctx context.Context, contextID string, item *ContextItem) error {
	metadataJSON := []byte("{}")
	if item.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(item.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	// Get current max sequence number
	var maxSeq sql.NullInt64
	seqQuery := `SELECT MAX(sequence_number) FROM mcp.context_items WHERE context_id = $1`
	_ = r.db.GetContext(ctx, &maxSeq, seqQuery, contextID)

	nextSeq := 0
	if maxSeq.Valid {
		nextSeq = int(maxSeq.Int64) + 1
	}

	query := `
		INSERT INTO mcp.context_items (id, context_id, type, role, content, token_count, sequence_number, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.ExecContext(ctx, query,
		item.ID,
		contextID,
		item.Type,
		"assistant", // Default role
		item.Content,
		0, // token_count
		nextSeq,
		metadataJSON,
		time.Now(),
	)

	return err
}

// GetContextItems retrieves all items for a context
func (r *PostgresContextRepository) GetContextItems(ctx context.Context, contextID string) ([]*ContextItem, error) {
	query := `
		SELECT id, context_id, content, type, metadata
		FROM mcp.context_items
		WHERE context_id = $1
		ORDER BY sequence_number ASC
	`

	rows, err := r.db.QueryxContext(ctx, query, contextID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []*ContextItem
	for rows.Next() {
		var item struct {
			ID        string          `db:"id"`
			ContextID string          `db:"context_id"`
			Content   string          `db:"content"`
			Type      string          `db:"type"`
			Metadata  json.RawMessage `db:"metadata"`
		}

		if err := rows.StructScan(&item); err != nil {
			continue
		}

		result := &ContextItem{
			ID:        item.ID,
			ContextID: item.ContextID,
			Content:   item.Content,
			Type:      item.Type,
		}

		if len(item.Metadata) > 0 {
			_ = json.Unmarshal(item.Metadata, &result.Metadata)
		}

		results = append(results, result)
	}

	return results, nil
}

// UpdateContextItem updates an existing context item
func (r *PostgresContextRepository) UpdateContextItem(ctx context.Context, item *ContextItem) error {
	metadataJSON, err := json.Marshal(item.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `UPDATE mcp.context_items SET content = $1, metadata = $2 WHERE id = $3`
	_, err = r.db.ExecContext(ctx, query, item.Content, metadataJSON, item.ID)
	return err
}

// UpdateCompactionMetadata updates compaction tracking information
func (r *PostgresContextRepository) UpdateCompactionMetadata(ctx context.Context, contextID string, strategy string, lastCompactedAt time.Time) error {
	// Store in context metadata
	query := `
		UPDATE mcp.contexts
		SET metadata = jsonb_set(
			COALESCE(metadata, '{}'::jsonb),
			'{compaction}',
			jsonb_build_object(
				'strategy', $2::text,
				'last_compacted_at', $3::text
			)
		),
		updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query, contextID, strategy, lastCompactedAt.Format(time.RFC3339))
	return err
}

// GetContextsNeedingCompaction returns contexts that need compaction based on threshold
func (r *PostgresContextRepository) GetContextsNeedingCompaction(ctx context.Context, threshold int) ([]*Context, error) {
	query := `
		SELECT c.id, c.tenant_id, c.agent_id, c.metadata, c.created_at, c.updated_at
		FROM mcp.contexts c
		JOIN (
			SELECT context_id, COUNT(*) as item_count
			FROM mcp.context_items
			GROUP BY context_id
			HAVING COUNT(*) > $1
		) items ON c.id = items.context_id
		ORDER BY items.item_count DESC
		LIMIT 100
	`

	rows, err := r.db.QueryxContext(ctx, query, threshold)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []*Context
	for rows.Next() {
		var dbContext struct {
			ID        string          `db:"id"`
			TenantID  string          `db:"tenant_id"`
			AgentID   *string         `db:"agent_id"`
			Metadata  json.RawMessage `db:"metadata"`
			CreatedAt time.Time       `db:"created_at"`
			UpdatedAt time.Time       `db:"updated_at"`
		}

		if err := rows.StructScan(&dbContext); err != nil {
			continue
		}

		result := &Context{
			ID:        dbContext.ID,
			AgentID:   dbContext.TenantID,
			Status:    "active",
			CreatedAt: dbContext.CreatedAt.Unix(),
			UpdatedAt: dbContext.UpdatedAt.Unix(),
		}

		if dbContext.AgentID != nil {
			result.AgentID = *dbContext.AgentID
		}

		results = append(results, result)
	}

	return results, nil
}

// LinkEmbeddingToContext creates a link between an embedding and a context
func (r *PostgresContextRepository) LinkEmbeddingToContext(ctx context.Context, contextID string, embeddingID string, sequence int, importance float64) error {
	// Use DELETE+INSERT approach instead of ON CONFLICT to avoid constraint matching issues
	// Start a transaction for atomic DELETE+INSERT
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Delete any existing entry for this context_id and chunk_sequence
	deleteQuery := `DELETE FROM mcp.context_embeddings WHERE context_id = $1 AND chunk_sequence = $2`
	_, err = tx.ExecContext(ctx, deleteQuery, contextID, sequence)
	if err != nil {
		return fmt.Errorf("failed to delete existing link: %w", err)
	}

	// Insert the new link
	insertQuery := `
		INSERT INTO mcp.context_embeddings (id, context_id, embedding_id, chunk_sequence, importance_score, is_summary, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, false, NOW())
	`
	_, err = tx.ExecContext(ctx, insertQuery, contextID, embeddingID, sequence, importance)
	if err != nil {
		return fmt.Errorf("failed to insert link: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetContextEmbeddingLinks retrieves all embedding links for a context
func (r *PostgresContextRepository) GetContextEmbeddingLinks(ctx context.Context, contextID string) ([]ContextEmbeddingLink, error) {
	query := `
		SELECT id, context_id, embedding_id, chunk_sequence, importance_score, is_summary, created_at
		FROM mcp.context_embeddings
		WHERE context_id = $1
		ORDER BY chunk_sequence ASC
	`

	rows, err := r.db.QueryxContext(ctx, query, contextID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []ContextEmbeddingLink
	for rows.Next() {
		var link ContextEmbeddingLink
		if err := rows.StructScan(&link); err != nil {
			continue
		}
		results = append(results, link)
	}

	return results, nil
}
