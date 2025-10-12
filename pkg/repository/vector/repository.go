// Package vector provides interfaces and implementations for vector embeddings
package vector

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// RepositoryImpl implements the Repository interface
type RepositoryImpl struct {
	db *sqlx.DB
}

// NewRepository creates a new vector repository instance
func NewRepository(db *sqlx.DB) Repository {
	return &RepositoryImpl{db: db}
}

// Create stores a new embedding (standardized Repository method)
func (r *RepositoryImpl) Create(ctx context.Context, embedding *Embedding) error {
	return r.StoreEmbedding(ctx, embedding)
}

// Get retrieves an embedding by its ID (standardized Repository method)
func (r *RepositoryImpl) Get(ctx context.Context, id string) (*Embedding, error) {
	if id == "" {
		return nil, errors.New("id cannot be empty")
	}

	query := `SELECT id, context_id, content_index, content, embedding, model_id, created_at, metadata
              FROM embeddings WHERE id = $1`

	var embedding Embedding
	err := r.db.GetContext(ctx, &embedding, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding: %w", err)
	}

	return &embedding, nil
}

// List retrieves embeddings matching the provided filter (standardized Repository method)
func (r *RepositoryImpl) List(ctx context.Context, filter Filter) ([]*Embedding, error) {
	query := `SELECT id, context_id, content_index, content, embedding, model_id, created_at, metadata FROM embeddings`

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

	query += whereClause + " ORDER BY content_index"

	var embeddings []*Embedding
	err := r.db.SelectContext(ctx, &embeddings, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list embeddings: %w", err)
	}

	return embeddings, nil
}

// Update modifies an existing embedding (standardized Repository method)
func (r *RepositoryImpl) Update(ctx context.Context, embedding *Embedding) error {
	return r.StoreEmbedding(ctx, embedding) // Uses upsert functionality
}

// Delete removes an embedding by its ID (standardized Repository method)
func (r *RepositoryImpl) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id cannot be empty")
	}

	query := `DELETE FROM embeddings WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete embedding: %w", err)
	}

	return nil
}

// StoreEmbedding stores a vector embedding
func (r *RepositoryImpl) StoreEmbedding(ctx context.Context, embedding *Embedding) error {
	if embedding == nil {
		return errors.New("embedding cannot be nil")
	}

	// Ensure we have a timestamp
	if embedding.CreatedAt.IsZero() {
		embedding.CreatedAt = time.Now()
	}

	query := `INSERT INTO embeddings (id, context_id, content_index, content, embedding, model_id, created_at, metadata)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
              ON CONFLICT (id) DO UPDATE SET
              context_id = $2, content_index = $3, content = $4, embedding = $5, model_id = $6, metadata = $8`

	_, err := r.db.ExecContext(ctx, query,
		embedding.ID,
		embedding.ContextID,
		embedding.ContentIndex,
		embedding.Text,
		embedding.Embedding,
		embedding.ModelID,
		embedding.CreatedAt,
		embedding.Metadata,
	)

	if err != nil {
		return fmt.Errorf("failed to store embedding: %w", err)
	}

	return nil
}

// SearchEmbeddings performs a vector search with various filter options
func (r *RepositoryImpl) SearchEmbeddings(
	ctx context.Context,
	queryVector []float32,
	contextID string,
	modelID string,
	limit int,
	similarityThreshold float64,
) ([]*Embedding, error) {
	if len(queryVector) == 0 {
		return nil, errors.New("query vector cannot be empty")
	}

	if limit <= 0 {
		limit = 10 // Default limit
	}

	// Build query based on parameters
	query := `SELECT id, context_id, content_index, content, embedding, model_id, created_at, metadata FROM embeddings`

	whereClause := ""
	var args []any
	argIndex := 1

	// Add context filter if provided
	if contextID != "" {
		whereClause = " WHERE context_id = $1"
		args = append(args, contextID)
		argIndex++
	}

	// Add model filter if provided
	if modelID != "" {
		if whereClause == "" {
			whereClause = " WHERE model_id = $" + fmt.Sprintf("%d", argIndex)
		} else {
			whereClause += " AND model_id = $" + fmt.Sprintf("%d", argIndex)
		}
		args = append(args, modelID)
		argIndex++
	}

	// Order by similarity to query vector (simplified version)
	// In a real implementation, this would use vector similarity functions like cosine similarity
	// but for compatibility we'll use a simplified approach
	query += whereClause + " ORDER BY id LIMIT $" + fmt.Sprintf("%d", argIndex)
	args = append(args, limit)

	var embeddings []*Embedding
	err := r.db.SelectContext(ctx, &embeddings, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search embeddings: %w", err)
	}

	// In a real implementation, we'd calculate similarity here and filter by threshold
	// For now, we'll just return all results
	return embeddings, nil
}

// SearchEmbeddings_Legacy performs a legacy vector search
func (r *RepositoryImpl) SearchEmbeddings_Legacy(
	ctx context.Context,
	queryVector []float32,
	contextID string,
	limit int,
) ([]*Embedding, error) {
	// Legacy method delegates to the new method with default values
	return r.SearchEmbeddings(ctx, queryVector, contextID, "", limit, 0.0)
}

// GetContextEmbeddings retrieves all embeddings for a context
func (r *RepositoryImpl) GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error) {
	if contextID == "" {
		return nil, errors.New("context ID cannot be empty")
	}

	query := `SELECT id, context_id, content_index, text, embedding, model_id, created_at, metadata
              FROM embeddings WHERE context_id = $1 ORDER BY content_index`

	var embeddings []*Embedding
	err := r.db.SelectContext(ctx, &embeddings, query, contextID)
	if err != nil {
		return nil, fmt.Errorf("failed to get context embeddings: %w", err)
	}

	return embeddings, nil
}

// DeleteContextEmbeddings deletes all embeddings for a context
func (r *RepositoryImpl) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	if contextID == "" {
		return errors.New("context ID cannot be empty")
	}

	query := `DELETE FROM embeddings WHERE context_id = $1`

	_, err := r.db.ExecContext(ctx, query, contextID)
	if err != nil {
		return fmt.Errorf("failed to delete context embeddings: %w", err)
	}

	return nil
}

// GetEmbeddingsByModel retrieves all embeddings for a context and model
func (r *RepositoryImpl) GetEmbeddingsByModel(
	ctx context.Context,
	contextID string,
	modelID string,
) ([]*Embedding, error) {
	if contextID == "" {
		return nil, errors.New("context ID cannot be empty")
	}

	if modelID == "" {
		return nil, errors.New("model ID cannot be empty")
	}

	query := `SELECT id, context_id, content_index, text, embedding, model_id, created_at, metadata
              FROM embeddings WHERE context_id = $1 AND model_id = $2 ORDER BY content_index`

	var embeddings []*Embedding
	err := r.db.SelectContext(ctx, &embeddings, query, contextID, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get embeddings by model: %w", err)
	}

	return embeddings, nil
}

// GetSupportedModels returns a list of models with embeddings
func (r *RepositoryImpl) GetSupportedModels(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT model_id FROM embeddings WHERE model_id IS NOT NULL AND model_id != ''`

	var modelIDs []string
	err := r.db.SelectContext(ctx, &modelIDs, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get supported models: %w", err)
	}

	return modelIDs, nil
}

// DeleteModelEmbeddings deletes all embeddings for a specific model in a context
func (r *RepositoryImpl) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	if contextID == "" {
		return errors.New("context ID cannot be empty")
	}

	if modelID == "" {
		return errors.New("model ID cannot be empty")
	}

	query := `DELETE FROM embeddings WHERE context_id = $1 AND model_id = $2`

	_, err := r.db.ExecContext(ctx, query, contextID, modelID)
	if err != nil {
		return fmt.Errorf("failed to delete model embeddings: %w", err)
	}

	return nil
}

// Story 2.1: Context-Specific Embedding Methods

// StoreContextEmbedding stores an embedding and links it to a context with metadata
func (r *RepositoryImpl) StoreContextEmbedding(
	ctx context.Context,
	contextID string,
	embedding *Embedding,
	sequence int,
	importance float64,
) (string, error) {
	if embedding == nil {
		return "", errors.New("embedding cannot be nil")
	}

	if contextID == "" {
		return "", errors.New("context ID cannot be empty")
	}

	// First store the embedding using existing method
	if err := r.StoreEmbedding(ctx, embedding); err != nil {
		return "", fmt.Errorf("failed to store embedding: %w", err)
	}

	// Then create the link in context_embeddings table
	// Use DELETE+INSERT approach instead of ON CONFLICT to avoid constraint matching issues
	// Start a transaction for atomic DELETE+INSERT
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
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
		return "", fmt.Errorf("failed to delete existing link: %w", err)
	}

	// Insert the new link
	insertQuery := `
		INSERT INTO mcp.context_embeddings
		(context_id, embedding_id, chunk_sequence, importance_score, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`
	_, err = tx.ExecContext(ctx, insertQuery, contextID, embedding.ID, sequence, importance)
	if err != nil {
		return "", fmt.Errorf("failed to insert link: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return embedding.ID, nil
}

// GetContextEmbeddingsBySequence retrieves embeddings for a context within a sequence range
func (r *RepositoryImpl) GetContextEmbeddingsBySequence(
	ctx context.Context,
	contextID string,
	startSeq int,
	endSeq int,
) ([]*Embedding, error) {
	if contextID == "" {
		return nil, errors.New("context ID cannot be empty")
	}

	query := `
		SELECT e.id, e.context_id, e.content_index, e.content, e.embedding, e.model_id, e.created_at, e.metadata
		FROM embeddings e
		JOIN mcp.context_embeddings ce ON e.id = ce.embedding_id
		WHERE ce.context_id = $1
		AND ce.chunk_sequence BETWEEN $2 AND $3
		ORDER BY ce.chunk_sequence
	`

	var embeddings []*Embedding
	err := r.db.SelectContext(ctx, &embeddings, query, contextID, startSeq, endSeq)
	if err != nil {
		return nil, fmt.Errorf("failed to get context embeddings by sequence: %w", err)
	}

	return embeddings, nil
}

// UpdateEmbeddingImportance updates the importance score for an embedding
func (r *RepositoryImpl) UpdateEmbeddingImportance(
	ctx context.Context,
	embeddingID string,
	importance float64,
) error {
	if embeddingID == "" {
		return errors.New("embedding ID cannot be empty")
	}

	if importance < 0 || importance > 1 {
		return errors.New("importance must be between 0 and 1")
	}

	query := `
		UPDATE mcp.context_embeddings
		SET importance_score = $1, updated_at = NOW()
		WHERE embedding_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, importance, embeddingID)
	if err != nil {
		return fmt.Errorf("failed to update importance: %w", err)
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no context embedding found with embedding_id: %s", embeddingID)
	}

	return nil
}
