package model_catalog

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// ModelCatalogRepositoryImpl implements ModelCatalogRepository
type ModelCatalogRepositoryImpl struct {
	db     *sqlx.DB
	logger observability.Logger
}

// NewModelCatalogRepository creates a new ModelCatalogRepository instance
func NewModelCatalogRepository(db *sqlx.DB) ModelCatalogRepository {
	return &ModelCatalogRepositoryImpl{
		db:     db,
		logger: observability.NewStandardLogger("model_catalog_repository"),
	}
}

// GetByID retrieves a model by its UUID
func (r *ModelCatalogRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*EmbeddingModel, error) {
	query := `
		SELECT id, provider, model_name, model_id, model_version, dimensions, max_tokens,
		       supports_binary, supports_dimensionality_reduction, min_dimensions, model_type,
		       cost_per_million_tokens, cost_per_million_chars, is_available, is_deprecated,
		       deprecation_date, minimum_tier, requires_api_key, provider_config,
		       capabilities, performance_metrics, created_at, updated_at
		FROM mcp.embedding_model_catalog
		WHERE id = $1`

	var model EmbeddingModel
	err := r.db.GetContext(ctx, &model, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("model not found with id %s: %w", id, err)
		}
		return nil, fmt.Errorf("failed to get model by id: %w", err)
	}

	return &model, nil
}

// GetByModelID retrieves a model by its model_id
func (r *ModelCatalogRepositoryImpl) GetByModelID(ctx context.Context, modelID string) (*EmbeddingModel, error) {
	query := `
		SELECT id, provider, model_name, model_id, model_version, dimensions, max_tokens,
		       supports_binary, supports_dimensionality_reduction, min_dimensions, model_type,
		       cost_per_million_tokens, cost_per_million_chars, is_available, is_deprecated,
		       deprecation_date, minimum_tier, requires_api_key, provider_config,
		       capabilities, performance_metrics, created_at, updated_at
		FROM mcp.embedding_model_catalog
		WHERE model_id = $1`

	var model EmbeddingModel
	err := r.db.GetContext(ctx, &model, query, modelID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("model not found with model_id %s: %w", modelID, err)
		}
		return nil, fmt.Errorf("failed to get model by model_id: %w", err)
	}

	return &model, nil
}

// ListAvailable returns all available (non-deprecated) models
func (r *ModelCatalogRepositoryImpl) ListAvailable(ctx context.Context, filter *ModelFilter) ([]*EmbeddingModel, error) {
	query := `
		SELECT id, provider, model_name, model_id, model_version, dimensions, max_tokens,
		       supports_binary, supports_dimensionality_reduction, min_dimensions, model_type,
		       cost_per_million_tokens, cost_per_million_chars, is_available, is_deprecated,
		       deprecation_date, minimum_tier, requires_api_key, provider_config,
		       capabilities, performance_metrics, created_at, updated_at
		FROM mcp.embedding_model_catalog
		WHERE is_available = true AND is_deprecated = false`

	args := []interface{}{}
	argNum := 1

	if filter != nil {
		if filter.Provider != nil {
			query += fmt.Sprintf(" AND provider = $%d", argNum)
			args = append(args, *filter.Provider)
			argNum++
		}
		if filter.ModelType != nil {
			query += fmt.Sprintf(" AND model_type = $%d", argNum)
			args = append(args, *filter.ModelType)
			argNum++
		}
		if filter.MinimumTier != nil {
			query += fmt.Sprintf(" AND minimum_tier = $%d", argNum)
			args = append(args, *filter.MinimumTier)
		}
	}

	query += " ORDER BY provider, model_name"

	var models []*EmbeddingModel
	err := r.db.SelectContext(ctx, &models, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list available models: %w", err)
	}

	return models, nil
}

// ListAll returns all models including deprecated ones
func (r *ModelCatalogRepositoryImpl) ListAll(ctx context.Context, filter *ModelFilter) ([]*EmbeddingModel, error) {
	query := `
		SELECT id, provider, model_name, model_id, model_version, dimensions, max_tokens,
		       supports_binary, supports_dimensionality_reduction, min_dimensions, model_type,
		       cost_per_million_tokens, cost_per_million_chars, is_available, is_deprecated,
		       deprecation_date, minimum_tier, requires_api_key, provider_config,
		       capabilities, performance_metrics, created_at, updated_at
		FROM mcp.embedding_model_catalog
		WHERE 1=1`

	args := []interface{}{}
	argNum := 1

	if filter != nil {
		if filter.Provider != nil {
			query += fmt.Sprintf(" AND provider = $%d", argNum)
			args = append(args, *filter.Provider)
			argNum++
		}
		if filter.ModelType != nil {
			query += fmt.Sprintf(" AND model_type = $%d", argNum)
			args = append(args, *filter.ModelType)
			argNum++
		}
		if filter.IsAvailable != nil {
			query += fmt.Sprintf(" AND is_available = $%d", argNum)
			args = append(args, *filter.IsAvailable)
			argNum++
		}
		if filter.IsDeprecated != nil {
			query += fmt.Sprintf(" AND is_deprecated = $%d", argNum)
			args = append(args, *filter.IsDeprecated)
			argNum++
		}
		if filter.MinimumTier != nil {
			query += fmt.Sprintf(" AND minimum_tier = $%d", argNum)
			args = append(args, *filter.MinimumTier)
		}
	}

	query += " ORDER BY provider, model_name"

	var models []*EmbeddingModel
	err := r.db.SelectContext(ctx, &models, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list all models: %w", err)
	}

	return models, nil
}

// Create adds a new model to the catalog
func (r *ModelCatalogRepositoryImpl) Create(ctx context.Context, model *EmbeddingModel) error {
	if model.ID == uuid.Nil {
		model.ID = uuid.New()
	}
	if model.CreatedAt.IsZero() {
		model.CreatedAt = time.Now()
	}
	if model.UpdatedAt.IsZero() {
		model.UpdatedAt = time.Now()
	}

	query := `
		INSERT INTO mcp.embedding_model_catalog (
			id, provider, model_name, model_id, model_version, dimensions, max_tokens,
			supports_binary, supports_dimensionality_reduction, min_dimensions, model_type,
			cost_per_million_tokens, cost_per_million_chars, is_available, is_deprecated,
			deprecation_date, minimum_tier, requires_api_key, provider_config,
			capabilities, performance_metrics, created_at, updated_at
		) VALUES (
			:id, :provider, :model_name, :model_id, :model_version, :dimensions, :max_tokens,
			:supports_binary, :supports_dimensionality_reduction, :min_dimensions, :model_type,
			:cost_per_million_tokens, :cost_per_million_chars, :is_available, :is_deprecated,
			:deprecation_date, :minimum_tier, :requires_api_key, :provider_config,
			:capabilities, :performance_metrics, :created_at, :updated_at
		)`

	_, err := r.db.NamedExecContext(ctx, query, model)
	if err != nil {
		return fmt.Errorf("failed to create model: %w", err)
	}

	r.logger.Info("Model created", map[string]interface{}{
		"model_id": model.ModelID,
		"provider": model.Provider,
	})

	return nil
}

// Update modifies an existing model
func (r *ModelCatalogRepositoryImpl) Update(ctx context.Context, model *EmbeddingModel) error {
	model.UpdatedAt = time.Now()

	query := `
		UPDATE mcp.embedding_model_catalog SET
			provider = :provider,
			model_name = :model_name,
			model_id = :model_id,
			model_version = :model_version,
			dimensions = :dimensions,
			max_tokens = :max_tokens,
			supports_binary = :supports_binary,
			supports_dimensionality_reduction = :supports_dimensionality_reduction,
			min_dimensions = :min_dimensions,
			model_type = :model_type,
			cost_per_million_tokens = :cost_per_million_tokens,
			cost_per_million_chars = :cost_per_million_chars,
			is_available = :is_available,
			is_deprecated = :is_deprecated,
			deprecation_date = :deprecation_date,
			minimum_tier = :minimum_tier,
			requires_api_key = :requires_api_key,
			provider_config = :provider_config,
			capabilities = :capabilities,
			performance_metrics = :performance_metrics,
			updated_at = :updated_at
		WHERE id = :id`

	result, err := r.db.NamedExecContext(ctx, query, model)
	if err != nil {
		return fmt.Errorf("failed to update model: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("model not found with id %s", model.ID)
	}

	r.logger.Info("Model updated", map[string]interface{}{
		"model_id": model.ModelID,
		"provider": model.Provider,
	})

	return nil
}

// Delete removes a model from the catalog
func (r *ModelCatalogRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM mcp.embedding_model_catalog WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete model: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("model not found with id %s", id)
	}

	r.logger.Info("Model deleted", map[string]interface{}{
		"id": id,
	})

	return nil
}

// SetAvailability updates the availability status of a model
func (r *ModelCatalogRepositoryImpl) SetAvailability(ctx context.Context, id uuid.UUID, isAvailable bool) error {
	query := `
		UPDATE mcp.embedding_model_catalog 
		SET is_available = $1, updated_at = $2
		WHERE id = $3`

	result, err := r.db.ExecContext(ctx, query, isAvailable, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to set model availability: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("model not found with id %s", id)
	}

	r.logger.Info("Model availability updated", map[string]interface{}{
		"id":           id,
		"is_available": isAvailable,
	})

	return nil
}

// MarkDeprecated marks a model as deprecated
func (r *ModelCatalogRepositoryImpl) MarkDeprecated(ctx context.Context, id uuid.UUID, deprecationDate *time.Time) error {
	if deprecationDate == nil {
		now := time.Now()
		deprecationDate = &now
	}

	query := `
		UPDATE mcp.embedding_model_catalog 
		SET is_deprecated = true, deprecation_date = $1, updated_at = $2
		WHERE id = $3`

	result, err := r.db.ExecContext(ctx, query, deprecationDate, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to mark model as deprecated: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("model not found with id %s", id)
	}

	r.logger.Info("Model marked as deprecated", map[string]interface{}{
		"id":               id,
		"deprecation_date": deprecationDate,
	})

	return nil
}

// GetProviders returns a list of unique providers
func (r *ModelCatalogRepositoryImpl) GetProviders(ctx context.Context) ([]string, error) {
	query := `
		SELECT DISTINCT provider 
		FROM mcp.embedding_model_catalog 
		WHERE is_available = true 
		ORDER BY provider`

	var providers []string
	err := r.db.SelectContext(ctx, &providers, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get providers: %w", err)
	}

	return providers, nil
}

// BulkUpsert inserts or updates multiple models
func (r *ModelCatalogRepositoryImpl) BulkUpsert(ctx context.Context, models []*EmbeddingModel) error {
	if len(models) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			r.logger.Warn("Failed to rollback transaction", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	query := `
		INSERT INTO mcp.embedding_model_catalog (
			id, provider, model_name, model_id, model_version, dimensions, max_tokens,
			supports_binary, supports_dimensionality_reduction, min_dimensions, model_type,
			cost_per_million_tokens, cost_per_million_chars, is_available, is_deprecated,
			deprecation_date, minimum_tier, requires_api_key, provider_config,
			capabilities, performance_metrics, created_at, updated_at
		) VALUES (
			:id, :provider, :model_name, :model_id, :model_version, :dimensions, :max_tokens,
			:supports_binary, :supports_dimensionality_reduction, :min_dimensions, :model_type,
			:cost_per_million_tokens, :cost_per_million_chars, :is_available, :is_deprecated,
			:deprecation_date, :minimum_tier, :requires_api_key, :provider_config,
			:capabilities, :performance_metrics, :created_at, :updated_at
		)
		ON CONFLICT (provider, model_name) DO UPDATE SET
			model_id = EXCLUDED.model_id,
			model_version = EXCLUDED.model_version,
			dimensions = EXCLUDED.dimensions,
			max_tokens = EXCLUDED.max_tokens,
			supports_binary = EXCLUDED.supports_binary,
			supports_dimensionality_reduction = EXCLUDED.supports_dimensionality_reduction,
			min_dimensions = EXCLUDED.min_dimensions,
			model_type = EXCLUDED.model_type,
			cost_per_million_tokens = EXCLUDED.cost_per_million_tokens,
			cost_per_million_chars = EXCLUDED.cost_per_million_chars,
			is_available = EXCLUDED.is_available,
			is_deprecated = EXCLUDED.is_deprecated,
			deprecation_date = EXCLUDED.deprecation_date,
			minimum_tier = EXCLUDED.minimum_tier,
			requires_api_key = EXCLUDED.requires_api_key,
			provider_config = EXCLUDED.provider_config,
			capabilities = EXCLUDED.capabilities,
			performance_metrics = EXCLUDED.performance_metrics,
			updated_at = EXCLUDED.updated_at`

	for _, model := range models {
		if model.ID == uuid.Nil {
			model.ID = uuid.New()
		}
		if model.CreatedAt.IsZero() {
			model.CreatedAt = time.Now()
		}
		model.UpdatedAt = time.Now()

		_, err := tx.NamedExecContext(ctx, query, model)
		if err != nil {
			return fmt.Errorf("failed to upsert model %s: %w", model.ModelID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.logger.Info("Bulk upsert completed", map[string]interface{}{
		"count": len(models),
	})

	return nil
}
