// Package vector provides interfaces and implementations for vector embeddings
package vector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/developer-mesh/developer-mesh/pkg/database"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// RepositoryImpl implements the Repository interface
type RepositoryImpl struct {
	db       *sqlx.DB
	vectorDB *database.VectorDatabase
	logger   observability.Logger
}

// NewRepository creates a new vector repository instance
func NewRepository(db *sqlx.DB) Repository {
	logger := observability.NewStandardLogger("vector_repository")

	// Initialize the vector database for pgvector operations
	vectorDB, err := database.NewVectorDatabase(db, nil, logger)
	if err != nil {
		logger.Error("Failed to create vector database", map[string]interface{}{
			"error": err.Error(),
		})
		// We still create the repository, but operations using vectorDB will fail
	}

	return &RepositoryImpl{
		db:       db,
		vectorDB: vectorDB,
		logger:   logger,
	}
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

// StoreEmbedding stores a vector embedding in mcp.embeddings
func (r *RepositoryImpl) StoreEmbedding(ctx context.Context, embedding *Embedding) error {
	if embedding == nil {
		return errors.New("embedding cannot be nil")
	}

	// Ensure we have a timestamp
	if embedding.CreatedAt.IsZero() {
		embedding.CreatedAt = time.Now()
	}

	// Initialize the vector database for pgvector operations
	if r.vectorDB == nil {
		return errors.New("vector database not initialized")
	}

	if err := r.vectorDB.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize vector database: %w", err)
	}

	// Convert []float32 to pgvector format using VectorDatabase
	vectorStr, err := r.vectorDB.CreateVector(ctx, embedding.Embedding)
	if err != nil {
		return fmt.Errorf("failed to convert embedding to pgvector format: %w", err)
	}

	// Extract tenant_id from ContextID (assumes UUID format)
	// For RAG embeddings, ContextID is actually the tenant_id since they're not session-based
	tenantID := embedding.ContextID

	// For RAG embeddings (source_type=rag), context_id should be NULL since there's no session context
	// The foreign key constraint requires context_id to reference mcp.contexts(id)
	var contextIDValue interface{}
	if embedding.Metadata != nil {
		if sourceType, ok := embedding.Metadata["source_type"].(string); ok && sourceType == "rag" {
			contextIDValue = nil // NULL for RAG embeddings
		} else {
			contextIDValue = embedding.ContextID // Use ContextID for session-based embeddings
		}
	} else {
		contextIDValue = embedding.ContextID
	}

	// Calculate content hash for deduplication
	hash := sha256.Sum256([]byte(embedding.Text))
	contentHash := hex.EncodeToString(hash[:])

	// Parse model provider and name from ModelID
	// Format: "provider/model-name" or just "model-name"
	provider, modelName := parseModelID(embedding.ModelID)

	// Look up model UUID from mcp.embedding_models
	modelUUID, modelDimensions, err := r.lookupModelUUID(ctx, provider, modelName)
	if err != nil {
		return fmt.Errorf("failed to lookup model UUID: %w", err)
	}

	// Marshal metadata to JSON for JSONB column
	var metadataJSON []byte
	if embedding.Metadata != nil {
		metadataJSON, err = json.Marshal(embedding.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	} else {
		metadataJSON = []byte("{}")
	}

	// Select the appropriate vector column based on model dimensions
	var vectorColumn string
	switch modelDimensions {
	case 1024:
		vectorColumn = "embedding_1024"
	case 1536:
		vectorColumn = "vector"
	case 4096:
		vectorColumn = "embedding"
	default:
		return fmt.Errorf("unsupported embedding dimensions: %d (supported: 1024, 1536, 4096)", modelDimensions)
	}

	// Insert into mcp.embeddings with all required fields
	query := fmt.Sprintf(`INSERT INTO mcp.embeddings (
		id, tenant_id, context_id, content_index, chunk_index, content, content_hash,
		model_id, model_provider, model_name, model_dimensions, %s, created_at, metadata
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
	)
	ON CONFLICT (id) DO UPDATE SET
		tenant_id = $2, context_id = $3, content_index = $4, chunk_index = $5,
		content = $6, content_hash = $7, model_id = $8, model_provider = $9,
		model_name = $10, model_dimensions = $11, %s = $12, metadata = $14`, vectorColumn, vectorColumn)

	// Use vectorDB transaction for atomic pgvector operations
	err = r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, query,
			embedding.ID,
			tenantID,
			contextIDValue,
			embedding.ContentIndex,
			0, // chunk_index - default to 0 for non-chunk embeddings
			embedding.Text,
			contentHash,
			modelUUID,
			provider,
			modelName,
			modelDimensions,
			vectorStr,
			embedding.CreatedAt,
			metadataJSON,
		)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to store embedding in mcp.embeddings: %w", err)
	}

	return nil
}

// lookupModelUUID looks up the UUID for a model from mcp.embedding_models
func (r *RepositoryImpl) lookupModelUUID(ctx context.Context, provider, modelName string) (string, int, error) {
	var modelUUID string
	var dimensions int

	query := `SELECT id, dimensions FROM mcp.embedding_models WHERE provider = $1 AND model_name = $2`

	err := r.db.QueryRowContext(ctx, query, provider, modelName).Scan(&modelUUID, &dimensions)
	if err != nil {
		return "", 0, fmt.Errorf("model not found in catalog (provider=%s, model=%s): %w", provider, modelName, err)
	}

	return modelUUID, dimensions, nil
}

// parseModelID parses a model ID string into provider and model name
// Supports formats: "provider/model-name", "model-name"
func parseModelID(modelID string) (provider, modelName string) {
	// Try to split by "/"
	parts := strings.Split(modelID, "/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	// If no "/" found, try to infer provider from model name prefix
	if strings.HasPrefix(modelID, "amazon.") {
		return "bedrock", modelID
	}
	if strings.HasPrefix(modelID, "cohere.") {
		return "bedrock", modelID
	}
	if strings.HasPrefix(modelID, "anthropic.") {
		return "bedrock", modelID
	}
	if strings.HasPrefix(modelID, "text-embedding") {
		return "openai", modelID
	}

	// Default: assume bedrock provider (most common for RAG loader)
	return "bedrock", modelID
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
