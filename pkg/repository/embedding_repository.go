package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/developer-mesh/developer-mesh/pkg/database"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository/vector"
)

// EmbeddingRepositoryImpl implements VectorAPIRepository
type EmbeddingRepositoryImpl struct {
	db       *sqlx.DB
	vectorDB *database.VectorDatabase
	logger   observability.Logger
}

// NewEmbeddingRepository creates a new EmbeddingRepository instance
func NewEmbeddingRepository(db *sqlx.DB) VectorAPIRepository {
	// Create a logger that implements the observability.Logger interface
	logger := observability.NewStandardLogger("embedding_repository")

	// Initialize the vector database
	vectorDB, err := database.NewVectorDatabase(db, nil, logger)
	if err != nil {
		logger.Error("Failed to create vector database", map[string]any{"error": err})
		// We still create the repository, but operations using vectorDB will fail
	}

	return &EmbeddingRepositoryImpl{
		db:       db,
		vectorDB: vectorDB,
		logger:   logger,
	}
}

// StoreEmbedding implements VectorAPIRepository.StoreEmbedding
func (r *EmbeddingRepositoryImpl) StoreEmbedding(ctx context.Context, embedding *Embedding) error {
	if embedding == nil {
		return errors.New("embedding cannot be nil")
	}

	// Ensure we have a timestamp
	if embedding.CreatedAt.IsZero() {
		embedding.CreatedAt = time.Now()
	}

	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize vector database: %w", err)
	}

	// Format the vector to pgvector format using the vector database
	vectorStr, err := r.vectorDB.CreateVector(ctx, embedding.Embedding)
	if err != nil {
		return fmt.Errorf("failed to format vector: %w", err)
	}

	// Now store the embedding in the database
	query := `INSERT INTO mcp.embeddings
		(id, context_id, content_index, content, embedding, vector_dimensions, model_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
		context_id = $2, content_index = $3, content = $4, embedding = $5,
		vector_dimensions = $6, model_id = $7`

	// Execute the database query using the transaction from the vector database
	// This ensures that vector operations are atomic
	err = r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, query,
			embedding.ID,
			embedding.ContextID,
			embedding.ContentIndex,
			embedding.Text,
			vectorStr,
			len(embedding.Embedding), // vector dimensions
			embedding.ModelID,
			embedding.CreatedAt,
		)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to store embedding: %w", err)
	}

	return nil
}

// SearchEmbeddings implements VectorAPIRepository.SearchEmbeddings
func (r *EmbeddingRepositoryImpl) SearchEmbeddings(
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

	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return nil, errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize vector database: %w", err)
	}

	// Format the query vector for PostgreSQL
	vectorStr, err := r.vectorDB.CreateVector(ctx, queryVector)
	if err != nil {
		return nil, fmt.Errorf("failed to format vector: %w", err)
	}

	// Build search query with vector similarity
	query := `
		SELECT id, context_id, content_index, content, embedding::text, vector_dimensions, model_id, created_at,
		       (1 - (embedding <=> $1::vector)) as similarity
		FROM mcp.embeddings
		WHERE context_id = $2
	`

	// Add model filter if provided
	args := []any{vectorStr, contextID}
	if modelID != "" {
		query += " AND model_id = $3"
		args = append(args, modelID)
	}

	// Add similarity threshold if provided
	if similarityThreshold > 0 {
		query += fmt.Sprintf(" AND (1 - (embedding <=> $1::vector)) >= %f", similarityThreshold)
	}

	// Order by similarity and add limit
	query += " ORDER BY similarity DESC LIMIT $" + fmt.Sprintf("%d", len(args)+1)
	args = append(args, limit)

	// Initialize embeddings slice and use the Transaction method to access the database
	embeddings := make([]*Embedding, 0)
	err = r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		rows, err := tx.QueryContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("failed to search embeddings: %w", err)
		}
		defer func() {
			if err := rows.Close(); err != nil {
				// Embedding repository - log but don't fail
				_ = err
			}
		}()

		// Process results
		for rows.Next() {
			var (
				id, contextID            string
				contentIndex, dimensions int
				text, embStr, modelID    string
				createdAt                time.Time
				similarity               float64
			)

			if err := rows.Scan(
				&id,
				&contextID,
				&contentIndex,
				&text,
				&embStr,
				&dimensions,
				&modelID,
				&createdAt,
				&similarity,
			); err != nil {
				return fmt.Errorf("failed to scan embedding: %w", err)
			}

			embedding := &Embedding{
				ID:           id,
				ContextID:    contextID,
				ContentIndex: contentIndex,
				Text:         text,
				ModelID:      modelID,
				CreatedAt:    createdAt,
				// We'll need to convert the embedding string to float32 array
				// For now, we'll just leave it empty since we often don't need the actual embedding values
				Embedding: []float32{},
			}

			// Add metadata with the similarity score
			embedding.Metadata = map[string]any{
				"similarity": similarity,
			}

			embeddings = append(embeddings, embedding)
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("error iterating embeddings: %w", err)
		}

		return nil
	})

	// Check for transaction error
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return embeddings, nil
}

// SearchEmbeddings_Legacy implements VectorAPIRepository.SearchEmbeddings_Legacy
func (r *EmbeddingRepositoryImpl) SearchEmbeddings_Legacy(
	ctx context.Context,
	queryVector []float32,
	contextID string,
	limit int,
) ([]*Embedding, error) {
	// Legacy method delegates to the new method with default values
	return r.SearchEmbeddings(ctx, queryVector, contextID, "", limit, 0.0)
}

// GetContextEmbeddings implements VectorAPIRepository.GetContextEmbeddings
func (r *EmbeddingRepositoryImpl) GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error) {
	if contextID == "" {
		return nil, errors.New("context ID cannot be empty")
	}

	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return nil, errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize vector database: %w", err)
	}

	query := `
		SELECT id, context_id, content_index, content, embedding::text, vector_dimensions, model_id, created_at
		FROM mcp.embeddings
		WHERE context_id = $1
		ORDER BY content_index
	`

	// Initialize embeddings slice and use the Transaction method
	embeddings := make([]*Embedding, 0)
	err := r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		rows, err := tx.QueryContext(ctx, query, contextID)
		if err != nil {
			return fmt.Errorf("failed to query context embeddings: %w", err)
		}
		defer func() {
			if err := rows.Close(); err != nil {
				// Embedding repository - log but don't fail
				_ = err
			}
		}()

		// Process results
		for rows.Next() {
			var (
				id, contextID            string
				contentIndex, dimensions int
				text, embStr, modelID    string
				createdAt                time.Time
			)

			if err := rows.Scan(
				&id,
				&contextID,
				&contentIndex,
				&text,
				&embStr,
				&dimensions,
				&modelID,
				&createdAt,
			); err != nil {
				return fmt.Errorf("failed to scan embedding: %w", err)
			}

			embedding := &Embedding{
				ID:           id,
				ContextID:    contextID,
				ContentIndex: contentIndex,
				Text:         text,
				ModelID:      modelID,
				CreatedAt:    createdAt,
				// We'll leave the embedding empty unless specifically needed
				Embedding: []float32{},
			}

			embeddings = append(embeddings, embedding)
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("error iterating embeddings: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return embeddings, nil
}

// DeleteContextEmbeddings implements VectorAPIRepository.DeleteContextEmbeddings
func (r *EmbeddingRepositoryImpl) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	if contextID == "" {
		return errors.New("context ID cannot be empty")
	}

	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize vector database: %w", err)
	}

	query := `DELETE FROM mcp.embeddings WHERE context_id = $1`

	err := r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, query, contextID)
		if err != nil {
			return fmt.Errorf("failed to delete context embeddings: %w", err)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	return nil
}

// GetEmbeddingsByModel implements VectorAPIRepository.GetEmbeddingsByModel
func (r *EmbeddingRepositoryImpl) GetEmbeddingsByModel(
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

	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return nil, errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize vector database: %w", err)
	}

	query := `
		SELECT id, context_id, content_index, content, embedding::text, vector_dimensions, model_id, created_at
		FROM mcp.embeddings
		WHERE context_id = $1 AND model_id = $2
		ORDER BY content_index
	`

	// Initialize embeddings slice and use the Transaction method
	embeddings := make([]*Embedding, 0)
	err := r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		rows, err := tx.QueryContext(ctx, query, contextID, modelID)
		if err != nil {
			return fmt.Errorf("failed to query embeddings by model: %w", err)
		}
		defer func() {
			if err := rows.Close(); err != nil {
				// Embedding repository - log but don't fail
				_ = err
			}
		}()

		// Process results
		for rows.Next() {
			var (
				id, contextID            string
				contentIndex, dimensions int
				text, embStr, modelID    string
				createdAt                time.Time
			)

			if err := rows.Scan(
				&id,
				&contextID,
				&contentIndex,
				&text,
				&embStr,
				&dimensions,
				&modelID,
				&createdAt,
			); err != nil {
				return fmt.Errorf("failed to scan embedding: %w", err)
			}

			embedding := &Embedding{
				ID:           id,
				ContextID:    contextID,
				ContentIndex: contentIndex,
				Text:         text,
				ModelID:      modelID,
				CreatedAt:    createdAt,
				// We'll leave the embedding empty unless specifically needed
				Embedding: []float32{},
			}

			embeddings = append(embeddings, embedding)
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("error iterating embeddings: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return embeddings, nil
}

// GetSupportedModels implements VectorAPIRepository.GetSupportedModels
func (r *EmbeddingRepositoryImpl) GetSupportedModels(ctx context.Context) ([]string, error) {
	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return nil, errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize vector database: %w", err)
	}

	query := `SELECT DISTINCT model_id FROM mcp.embeddings WHERE model_id IS NOT NULL AND model_id != ''`

	var modelIDs []string
	err := r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		rows, err := tx.QueryContext(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to query supported models: %w", err)
		}
		defer func() {
			if err := rows.Close(); err != nil {
				// Embedding repository - log but don't fail
				_ = err
			}
		}()

		// Process results
		for rows.Next() {
			var modelID string
			if err := rows.Scan(&modelID); err != nil {
				return fmt.Errorf("failed to scan model ID: %w", err)
			}
			modelIDs = append(modelIDs, modelID)
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("error iterating model IDs: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return modelIDs, nil
}

// DeleteModelEmbeddings implements VectorAPIRepository.DeleteModelEmbeddings
func (r *EmbeddingRepositoryImpl) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	if contextID == "" {
		return errors.New("context ID cannot be empty")
	}

	if modelID == "" {
		return errors.New("model ID cannot be empty")
	}

	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize vector database: %w", err)
	}

	query := `DELETE FROM mcp.embeddings WHERE context_id = $1 AND model_id = $2`

	err := r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, query, contextID, modelID)
		if err != nil {
			return fmt.Errorf("failed to delete model embeddings: %w", err)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	return nil
}

// GetEmbeddingByID implements VectorAPIRepository.GetEmbeddingByID
func (r *EmbeddingRepositoryImpl) GetEmbeddingByID(ctx context.Context, id string) (*Embedding, error) {
	if id == "" {
		return nil, errors.New("embedding ID cannot be empty")
	}

	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return nil, errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize vector database: %w", err)
	}

	// Create query to fetch embedding by ID
	query := `
		SELECT id, context_id, content_index, content, embedding::text, vector_dimensions, model_id, created_at
		FROM mcp.embeddings
		WHERE id = $1
	`

	var embedding *Embedding
	err := r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		var (
			id, contextID            string
			contentIndex, dimensions int
			text, embStr, modelID    string
			createdAt                time.Time
		)

		row := tx.QueryRowContext(ctx, query, id)
		if err := row.Scan(
			&id,
			&contextID,
			&contentIndex,
			&text,
			&embStr,
			&dimensions,
			&modelID,
			&createdAt,
		); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil // Not found, will return nil embedding
			}
			return fmt.Errorf("failed to scan embedding: %w", err)
		}

		// Create embedding from data
		embedding = &Embedding{
			ID:           id,
			ContextID:    contextID,
			ContentIndex: contentIndex,
			Text:         text,
			ModelID:      modelID,
			CreatedAt:    createdAt,
			// We'll leave the embedding empty unless specifically needed
			Embedding: []float32{},
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return embedding, nil
}

// DeleteEmbedding implements VectorAPIRepository.DeleteEmbedding
func (r *EmbeddingRepositoryImpl) DeleteEmbedding(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("embedding ID cannot be empty")
	}

	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize vector database: %w", err)
	}

	// Create delete query
	query := `DELETE FROM mcp.embeddings WHERE id = $1`

	err := r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, query, id)
		if err != nil {
			return fmt.Errorf("failed to delete embedding: %w", err)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	return nil
}

// BatchDeleteEmbeddings implements VectorAPIRepository.BatchDeleteEmbeddings
func (r *EmbeddingRepositoryImpl) BatchDeleteEmbeddings(ctx context.Context, ids []string) error {
	// Check if there are any IDs to delete
	if len(ids) == 0 {
		return nil // Nothing to do
	}

	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize vector database: %w", err)
	}

	// Create parameterized query with placeholders for all IDs
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf("DELETE FROM mcp.embeddings WHERE id IN (%s)", strings.Join(placeholders, ","))

	err := r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("failed to batch delete embeddings: %w", err)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	return nil
}

// The following methods implement the standard Repository[Embedding] interface

// Create implements Repository[Embedding].Create
func (r *EmbeddingRepositoryImpl) Create(ctx context.Context, embedding *vector.Embedding) error {
	// Delegate to StoreEmbedding for backward compatibility
	return r.StoreEmbedding(ctx, embedding)
}

// Get implements Repository[Embedding].Get
func (r *EmbeddingRepositoryImpl) Get(ctx context.Context, id string) (*vector.Embedding, error) {
	// Delegate to GetEmbeddingByID for backward compatibility
	return r.GetEmbeddingByID(ctx, id)
}

// List implements Repository[Embedding].List
func (r *EmbeddingRepositoryImpl) List(ctx context.Context, filter vector.Filter) ([]*vector.Embedding, error) {
	// Extract filters from the map
	var contextID, modelID string

	if filter != nil {
		if contextIDVal, ok := filter["context_id"]; ok {
			if contextIDStr, ok := contextIDVal.(string); ok {
				contextID = contextIDStr
			}
		}

		if modelIDVal, ok := filter["model_id"]; ok {
			if modelIDStr, ok := modelIDVal.(string); ok {
				modelID = modelIDStr
			}
		}
	}

	// Use specific retrieval methods based on filter contents
	if contextID != "" {
		if modelID != "" {
			// If we have both context and model ID
			return r.GetEmbeddingsByModel(ctx, contextID, modelID)
		}
		// If we only have context ID
		return r.GetContextEmbeddings(ctx, contextID)
	}

	// If no specific filters, return empty result for now
	// A complete implementation would retrieve all embeddings
	r.logger.Warn("List without context_id not implemented", nil)
	return []*vector.Embedding{}, nil
}

// Update implements Repository[Embedding].Update
func (r *EmbeddingRepositoryImpl) Update(ctx context.Context, embedding *vector.Embedding) error {
	// Delegate to StoreEmbedding for backward compatibility
	return r.StoreEmbedding(ctx, embedding)
}

// Delete implements Repository[Embedding].Delete
func (r *EmbeddingRepositoryImpl) Delete(ctx context.Context, id string) error {
	// Delegate to DeleteEmbedding for backward compatibility
	return r.DeleteEmbedding(ctx, id)
}

// Story 2.1: Context-Specific Embedding Methods

// StoreContextEmbedding stores an embedding and links it to a context with metadata
func (r *EmbeddingRepositoryImpl) StoreContextEmbedding(
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

	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return "", errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return "", fmt.Errorf("failed to initialize vector database: %w", err)
	}

	// First store the embedding using existing method
	if err := r.StoreEmbedding(ctx, embedding); err != nil {
		return "", fmt.Errorf("failed to store embedding: %w", err)
	}

	// Then create the link in context_embeddings table
	// Use DELETE+INSERT approach instead of ON CONFLICT to avoid constraint matching issues
	err := r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		// Delete any existing entry for this context_id and chunk_sequence
		deleteQuery := `DELETE FROM mcp.context_embeddings WHERE context_id = $1 AND chunk_sequence = $2`
		_, err := tx.ExecContext(ctx, deleteQuery, contextID, sequence)
		if err != nil {
			return fmt.Errorf("failed to delete existing link: %w", err)
		}

		// Insert the new link
		insertQuery := `
			INSERT INTO mcp.context_embeddings
			(context_id, embedding_id, chunk_sequence, importance_score, created_at, updated_at)
			VALUES ($1, $2, $3, $4, NOW(), NOW())
		`
		_, err = tx.ExecContext(ctx, insertQuery, contextID, embedding.ID, sequence, importance)
		return err
	})

	if err != nil {
		return "", fmt.Errorf("failed to link embedding to context: %w", err)
	}

	return embedding.ID, nil
}

// GetContextEmbeddingsBySequence retrieves embeddings for a context within a sequence range
func (r *EmbeddingRepositoryImpl) GetContextEmbeddingsBySequence(
	ctx context.Context,
	contextID string,
	startSeq int,
	endSeq int,
) ([]*Embedding, error) {
	if contextID == "" {
		return nil, errors.New("context ID cannot be empty")
	}

	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return nil, errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize vector database: %w", err)
	}

	query := `
		SELECT e.id, e.context_id, e.content_index, e.content, e.embedding::text,
		       e.vector_dimensions, e.model_id, e.created_at
		FROM mcp.embeddings e
		JOIN mcp.context_embeddings ce ON e.id = ce.embedding_id
		WHERE ce.context_id = $1
		AND ce.chunk_sequence BETWEEN $2 AND $3
		ORDER BY ce.chunk_sequence
	`

	embeddings := make([]*Embedding, 0)
	err := r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		rows, err := tx.QueryContext(ctx, query, contextID, startSeq, endSeq)
		if err != nil {
			return fmt.Errorf("failed to query context embeddings: %w", err)
		}
		defer func() {
			if err := rows.Close(); err != nil {
				r.logger.Warn("Failed to close rows", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}()

		// Process results
		for rows.Next() {
			var (
				id, contextID            string
				contentIndex, dimensions int
				text, embStr, modelID    string
				createdAt                time.Time
			)

			if err := rows.Scan(
				&id,
				&contextID,
				&contentIndex,
				&text,
				&embStr,
				&dimensions,
				&modelID,
				&createdAt,
			); err != nil {
				return fmt.Errorf("failed to scan embedding: %w", err)
			}

			embedding := &Embedding{
				ID:           id,
				ContextID:    contextID,
				ContentIndex: contentIndex,
				Text:         text,
				ModelID:      modelID,
				CreatedAt:    createdAt,
				// We'll leave the embedding empty unless specifically needed
				Embedding: []float32{},
			}

			embeddings = append(embeddings, embedding)
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("error iterating embeddings: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return embeddings, nil
}

// UpdateEmbeddingImportance updates the importance score for an embedding
func (r *EmbeddingRepositoryImpl) UpdateEmbeddingImportance(
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

	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize vector database: %w", err)
	}

	query := `
		UPDATE mcp.context_embeddings
		SET importance_score = $1, updated_at = NOW()
		WHERE embedding_id = $2
	`

	var rowsAffected int64
	err := r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		result, err := tx.ExecContext(ctx, query, importance, embeddingID)
		if err != nil {
			return fmt.Errorf("failed to update importance: %w", err)
		}

		// Check if any rows were affected
		rowsAffected, err = result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no context embedding found with embedding_id: %s", embeddingID)
	}

	return nil
}
