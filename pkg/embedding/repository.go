package embedding

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type Repository struct {
	db      *sql.DB
	logger  observability.Logger
	metrics observability.MetricsClient
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{
		db:      db,
		logger:  observability.NewLogger("embedding.repository"),
		metrics: observability.NewMetricsClient(), // Production metrics client
	}
}

// NewRepositoryWithObservability creates a repository with custom observability components
func NewRepositoryWithObservability(db *sql.DB, logger observability.Logger, metrics observability.MetricsClient) *Repository {
	return &Repository{
		db:      db,
		logger:  logger,
		metrics: metrics,
	}
}

// InsertEmbedding inserts a new embedding with automatic padding
func (r *Repository) InsertEmbedding(ctx context.Context, req InsertRequest) (uuid.UUID, error) {
	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "embedding.repository.insert")
	defer span.End()

	span.SetAttribute("operation", "insert_embedding")
	span.SetAttribute("model", req.ModelName)
	span.SetAttribute("tenant_id", req.TenantID.String())
	span.SetAttribute("embedding_dimensions", len(req.Embedding))

	// Extract tenant/user context for logging
	tenantID := auth.GetTenantID(ctx)
	userID := auth.GetUserID(ctx)
	correlationID := observability.GetCorrelationID(ctx)

	// Log the operation with context
	r.logger.Info("Inserting embedding", map[string]interface{}{
		"tenant_id":      tenantID,
		"user_id":        userID,
		"correlation_id": correlationID,
		"context_id":     req.ContextID,
		"model":          req.ModelName,
		"content_size":   len(req.Content),
		"dimensions":     len(req.Embedding),
	})

	// Track metrics
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		labels := map[string]string{
			"model":  req.ModelName,
			"tenant": req.TenantID.String(),
		}
		r.metrics.RecordHistogram("embedding.repository.insert.duration", duration.Seconds(), labels)
		r.metrics.IncrementCounter("embedding.repository.insert.total", 1.0)
	}()

	var id uuid.UUID

	err := r.db.QueryRowContext(ctx, `
        SELECT mcp.insert_embedding($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `,
		req.ContextID,
		req.Content,
		pq.Array(req.Embedding),
		req.ModelName,
		req.TenantID,
		req.Metadata,
		req.ContentIndex,
		req.ChunkIndex,
		req.ConfiguredDimensions,
	).Scan(&id)

	if err != nil {
		r.metrics.IncrementCounter("embedding.repository.insert.error", 1.0)
		r.logger.Error("Failed to insert embedding", map[string]interface{}{
			"error":          err.Error(),
			"tenant_id":      tenantID,
			"correlation_id": correlationID,
			"context_id":     req.ContextID,
			"model":          req.ModelName,
		})
		span.RecordError(err)
		span.SetStatus(500, "Failed to insert embedding")
		return uuid.Nil, fmt.Errorf("failed to insert embedding: %w", err)
	}

	r.logger.Debug("Successfully inserted embedding", map[string]interface{}{
		"embedding_id":   id,
		"tenant_id":      tenantID,
		"correlation_id": correlationID,
		"context_id":     req.ContextID,
	})

	return id, nil
}

// SearchEmbeddings performs similarity search with optional metadata filtering
func (r *Repository) SearchEmbeddings(ctx context.Context, req SearchRequest) ([]EmbeddingSearchResult, error) {
	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "embedding.repository.search")
	defer span.End()

	span.SetAttribute("operation", "search_embeddings")
	span.SetAttribute("model", req.ModelName)
	span.SetAttribute("tenant_id", req.TenantID.String())
	span.SetAttribute("limit", req.Limit)
	span.SetAttribute("threshold", req.Threshold)

	// Extract context for logging
	tenantID := auth.GetTenantID(ctx)
	correlationID := observability.GetCorrelationID(ctx)

	// Log the operation
	r.logger.Info("Searching embeddings", map[string]interface{}{
		"tenant_id":           tenantID,
		"correlation_id":      correlationID,
		"model":               req.ModelName,
		"limit":               req.Limit,
		"threshold":           req.Threshold,
		"has_context_id":      req.ContextID != nil,
		"has_metadata_filter": req.MetadataFilter != nil,
	})

	// Track metrics
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		labels := map[string]string{
			"model":  req.ModelName,
			"tenant": req.TenantID.String(),
		}
		r.metrics.RecordHistogram("embedding.repository.search.duration", duration.Seconds(), labels)
		r.metrics.IncrementCounter("embedding.repository.search.total", 1.0)
	}()

	rows, err := r.db.QueryContext(ctx, `
        SELECT * FROM mcp.search_embeddings($1, $2, $3, $4, $5, $6, $7)
    `,
		pq.Array(req.QueryEmbedding),
		req.ModelName,
		req.TenantID,
		req.ContextID, // can be nil
		req.Limit,
		req.Threshold,
		req.MetadataFilter, // JSONB filter
	)
	if err != nil {
		r.metrics.IncrementCounter("embedding.repository.search.error", 1.0)
		r.logger.Error("Failed to search embeddings", map[string]interface{}{
			"error":          err.Error(),
			"tenant_id":      tenantID,
			"correlation_id": correlationID,
			"model":          req.ModelName,
		})
		span.RecordError(err)
		span.SetStatus(500, "Failed to search embeddings")
		return nil, fmt.Errorf("failed to search embeddings: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			r.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	var results []EmbeddingSearchResult
	for rows.Next() {
		var r EmbeddingSearchResult
		if err := rows.Scan(&r.ID, &r.ContextID, &r.Content, &r.Similarity, &r.Metadata, &r.ModelProvider); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("error iterating results: %w", err)
	}

	r.logger.Debug("Search completed", map[string]interface{}{
		"result_count":   len(results),
		"tenant_id":      tenantID,
		"correlation_id": correlationID,
	})

	return results, nil
}

// GetAvailableModels retrieves available embedding models
func (r *Repository) GetAvailableModels(ctx context.Context, filter ModelFilter) ([]Model, error) {
	query := `
        SELECT provider, model_name, model_version, dimensions, max_tokens,
               model_type, supports_dimensionality_reduction, min_dimensions, is_active
        FROM mcp.get_available_models($1, $2)
    `

	rows, err := r.db.QueryContext(ctx, query, filter.Provider, filter.ModelType)
	if err != nil {
		return nil, fmt.Errorf("failed to get available models: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Embedding repository - log but don't fail
			_ = err
		}
	}()

	var models []Model
	for rows.Next() {
		var m Model
		err := rows.Scan(
			&m.Provider,
			&m.ModelName,
			&m.ModelVersion,
			&m.Dimensions,
			&m.MaxTokens,
			&m.ModelType,
			&m.SupportsDimensionalityReduction,
			&m.MinDimensions,
			&m.IsActive,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan model: %w", err)
		}
		models = append(models, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating models: %w", err)
	}

	return models, nil
}

// GetModelByName retrieves a model by name
func (r *Repository) GetModelByName(ctx context.Context, modelName string) (*Model, error) {
	var model Model

	err := r.db.QueryRowContext(ctx, `
        SELECT id, provider, model_name, model_version, dimensions,
               max_tokens, supports_binary, supports_dimensionality_reduction,
               min_dimensions, cost_per_million_tokens, model_id, model_type,
               is_active, capabilities, created_at
        FROM mcp.embedding_models
        WHERE model_name = $1 AND is_active = true
        LIMIT 1
    `, modelName).Scan(
		&model.ID,
		&model.Provider,
		&model.ModelName,
		&model.ModelVersion,
		&model.Dimensions,
		&model.MaxTokens,
		&model.SupportsBinary,
		&model.SupportsDimensionalityReduction,
		&model.MinDimensions,
		&model.CostPerMillionTokens,
		&model.ModelID,
		&model.ModelType,
		&model.IsActive,
		&model.Capabilities,
		&model.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("model not found: %s", modelName)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	return &model, nil
}

// GetExistingEmbedding checks if an embedding already exists for the given content
func (r *Repository) GetExistingEmbedding(ctx context.Context, contentHash string, modelName string, tenantID uuid.UUID) (*uuid.UUID, error) {
	var embeddingID uuid.UUID

	query := `
		SELECT e.id 
		FROM mcp.embeddings e
		JOIN mcp.embedding_models m ON e.model_id = m.id
		WHERE e.content_hash = $1 
		AND (m.model_name = $2 OR m.model_id = $2)
		AND e.tenant_id = $3
		LIMIT 1
	`

	err := r.db.QueryRowContext(ctx, query, contentHash, modelName, tenantID).Scan(&embeddingID)
	if err == sql.ErrNoRows {
		return nil, nil // No existing embedding found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing embedding: %w", err)
	}

	r.logger.Debug("Found existing embedding", map[string]interface{}{
		"embedding_id": embeddingID,
		"content_hash": contentHash,
		"model":        modelName,
		"tenant_id":    tenantID,
	})

	return &embeddingID, nil
}

// CalculateContentHash generates a SHA256 hash of the content
func CalculateContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// GetEmbeddingsByContext retrieves all embeddings for a context
func (r *Repository) GetEmbeddingsByContext(ctx context.Context, contextID, tenantID uuid.UUID) ([]Embedding, error) {
	query := `
        SELECT id, context_id, content_index, chunk_index, content,
               content_hash, content_tokens, model_id, model_provider,
               model_name, model_dimensions, configured_dimensions,
               processing_time_ms, embedding_created_at, magnitude, 
               tenant_id, metadata, created_at, updated_at
        FROM mcp.embeddings
        WHERE context_id = $1 AND tenant_id = $2
        ORDER BY content_index, chunk_index
    `

	rows, err := r.db.QueryContext(ctx, query, contextID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to query embeddings: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Embedding repository - log but don't fail
			_ = err
		}
	}()

	var embeddings []Embedding
	for rows.Next() {
		var e Embedding
		err := rows.Scan(
			&e.ID,
			&e.ContextID,
			&e.ContentIndex,
			&e.ChunkIndex,
			&e.Content,
			&e.ContentHash,
			&e.ContentTokens,
			&e.ModelID,
			&e.ModelProvider,
			&e.ModelName,
			&e.ModelDimensions,
			&e.ConfiguredDimensions,
			&e.ProcessingTimeMS,
			&e.EmbeddingCreatedAt,
			&e.Magnitude,
			&e.TenantID,
			&e.Metadata,
			&e.CreatedAt,
			&e.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan embedding: %w", err)
		}
		embeddings = append(embeddings, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating embeddings: %w", err)
	}

	return embeddings, nil
}
