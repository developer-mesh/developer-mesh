package tenant_models

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// TenantModelsRepositoryImpl implements TenantModelsRepository
type TenantModelsRepositoryImpl struct {
	db     *sqlx.DB
	logger observability.Logger
}

// NewTenantModelsRepository creates a new TenantModelsRepository instance
func NewTenantModelsRepository(db *sqlx.DB) TenantModelsRepository {
	return &TenantModelsRepositoryImpl{
		db:     db,
		logger: observability.NewStandardLogger("tenant_models_repository"),
	}
}

// GetTenantModel retrieves a specific tenant-model configuration
func (r *TenantModelsRepositoryImpl) GetTenantModel(ctx context.Context, tenantID, modelID uuid.UUID) (*TenantEmbeddingModel, error) {
	query := `
		SELECT id, tenant_id, model_id, is_enabled, is_default,
		       custom_cost_per_million_tokens, custom_cost_per_million_chars,
		       monthly_token_limit, daily_token_limit, monthly_request_limit,
		       priority, fallback_model_id, agent_preferences,
		       created_at, updated_at, created_by
		FROM mcp.tenant_embedding_models
		WHERE tenant_id = $1 AND model_id = $2`

	var model TenantEmbeddingModel
	err := r.db.GetContext(ctx, &model, query, tenantID, modelID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tenant model configuration not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get tenant model: %w", err)
	}

	return &model, nil
}

// ListTenantModels returns all models configured for a tenant
func (r *TenantModelsRepositoryImpl) ListTenantModels(ctx context.Context, tenantID uuid.UUID, enabledOnly bool) ([]*TenantEmbeddingModel, error) {
	query := `
		SELECT id, tenant_id, model_id, is_enabled, is_default,
		       custom_cost_per_million_tokens, custom_cost_per_million_chars,
		       monthly_token_limit, daily_token_limit, monthly_request_limit,
		       priority, fallback_model_id, agent_preferences,
		       created_at, updated_at, created_by
		FROM mcp.tenant_embedding_models
		WHERE tenant_id = $1`

	if enabledOnly {
		query += " AND is_enabled = true"
	}
	query += " ORDER BY priority DESC, is_default DESC"

	var models []*TenantEmbeddingModel
	err := r.db.SelectContext(ctx, &models, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tenant models: %w", err)
	}

	return models, nil
}

// GetDefaultModel returns the default model for a tenant
func (r *TenantModelsRepositoryImpl) GetDefaultModel(ctx context.Context, tenantID uuid.UUID) (*TenantEmbeddingModel, error) {
	query := `
		SELECT id, tenant_id, model_id, is_enabled, is_default,
		       custom_cost_per_million_tokens, custom_cost_per_million_chars,
		       monthly_token_limit, daily_token_limit, monthly_request_limit,
		       priority, fallback_model_id, agent_preferences,
		       created_at, updated_at, created_by
		FROM mcp.tenant_embedding_models
		WHERE tenant_id = $1 AND is_default = true AND is_enabled = true
		ORDER BY priority DESC
		LIMIT 1`

	var model TenantEmbeddingModel
	err := r.db.GetContext(ctx, &model, query, tenantID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no default model found for tenant: %w", err)
		}
		return nil, fmt.Errorf("failed to get default model: %w", err)
	}

	return &model, nil
}

// CreateTenantModel adds a model configuration for a tenant
func (r *TenantModelsRepositoryImpl) CreateTenantModel(ctx context.Context, model *TenantEmbeddingModel) error {
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
		INSERT INTO mcp.tenant_embedding_models (
			id, tenant_id, model_id, is_enabled, is_default,
			custom_cost_per_million_tokens, custom_cost_per_million_chars,
			monthly_token_limit, daily_token_limit, monthly_request_limit,
			priority, fallback_model_id, agent_preferences,
			created_at, updated_at, created_by
		) VALUES (
			:id, :tenant_id, :model_id, :is_enabled, :is_default,
			:custom_cost_per_million_tokens, :custom_cost_per_million_chars,
			:monthly_token_limit, :daily_token_limit, :monthly_request_limit,
			:priority, :fallback_model_id, :agent_preferences,
			:created_at, :updated_at, :created_by
		)`

	_, err := r.db.NamedExecContext(ctx, query, model)
	if err != nil {
		return fmt.Errorf("failed to create tenant model: %w", err)
	}

	r.logger.Info("Tenant model created", map[string]interface{}{
		"tenant_id": model.TenantID,
		"model_id":  model.ModelID,
	})

	return nil
}

// UpdateTenantModel updates a tenant's model configuration
func (r *TenantModelsRepositoryImpl) UpdateTenantModel(ctx context.Context, model *TenantEmbeddingModel) error {
	model.UpdatedAt = time.Now()

	query := `
		UPDATE mcp.tenant_embedding_models SET
			is_enabled = :is_enabled,
			is_default = :is_default,
			custom_cost_per_million_tokens = :custom_cost_per_million_tokens,
			custom_cost_per_million_chars = :custom_cost_per_million_chars,
			monthly_token_limit = :monthly_token_limit,
			daily_token_limit = :daily_token_limit,
			monthly_request_limit = :monthly_request_limit,
			priority = :priority,
			fallback_model_id = :fallback_model_id,
			agent_preferences = :agent_preferences,
			updated_at = :updated_at
		WHERE id = :id`

	result, err := r.db.NamedExecContext(ctx, query, model)
	if err != nil {
		return fmt.Errorf("failed to update tenant model: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("tenant model not found with id %s", model.ID)
	}

	r.logger.Info("Tenant model updated", map[string]interface{}{
		"id": model.ID,
	})

	return nil
}

// DeleteTenantModel removes a model configuration for a tenant
func (r *TenantModelsRepositoryImpl) DeleteTenantModel(ctx context.Context, tenantID, modelID uuid.UUID) error {
	query := `DELETE FROM mcp.tenant_embedding_models WHERE tenant_id = $1 AND model_id = $2`

	result, err := r.db.ExecContext(ctx, query, tenantID, modelID)
	if err != nil {
		return fmt.Errorf("failed to delete tenant model: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("tenant model not found")
	}

	r.logger.Info("Tenant model deleted", map[string]interface{}{
		"tenant_id": tenantID,
		"model_id":  modelID,
	})

	return nil
}

// SetDefaultModel sets a model as the default for a tenant
func (r *TenantModelsRepositoryImpl) SetDefaultModel(ctx context.Context, tenantID, modelID uuid.UUID) error {
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

	// First, unset any existing default
	_, err = tx.ExecContext(ctx,
		`UPDATE mcp.tenant_embedding_models SET is_default = false WHERE tenant_id = $1`,
		tenantID)
	if err != nil {
		return fmt.Errorf("failed to unset existing default: %w", err)
	}

	// Then set the new default
	result, err := tx.ExecContext(ctx,
		`UPDATE mcp.tenant_embedding_models SET is_default = true, updated_at = $1 
		 WHERE tenant_id = $2 AND model_id = $3`,
		time.Now(), tenantID, modelID)
	if err != nil {
		return fmt.Errorf("failed to set default model: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("tenant model not found")
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.logger.Info("Default model set", map[string]interface{}{
		"tenant_id": tenantID,
		"model_id":  modelID,
	})

	return nil
}

// UpdatePriority updates the priority of a tenant's model
func (r *TenantModelsRepositoryImpl) UpdatePriority(ctx context.Context, tenantID, modelID uuid.UUID, priority int) error {
	query := `
		UPDATE mcp.tenant_embedding_models 
		SET priority = $1, updated_at = $2
		WHERE tenant_id = $3 AND model_id = $4`

	result, err := r.db.ExecContext(ctx, query, priority, time.Now(), tenantID, modelID)
	if err != nil {
		return fmt.Errorf("failed to update priority: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("tenant model not found")
	}

	return nil
}

// GetModelForRequest selects the best model for a request using the SQL function
func (r *TenantModelsRepositoryImpl) GetModelForRequest(ctx context.Context, tenantID uuid.UUID, agentID *uuid.UUID, taskType, requestedModel *string) (*ModelSelection, error) {
	query := `
		SELECT * FROM mcp.get_embedding_model_for_request($1, $2, $3, $4)`

	var selection ModelSelection
	err := r.db.QueryRowContext(ctx, query, tenantID, agentID, taskType, requestedModel).Scan(
		&selection.ModelID,
		&selection.ModelIdentifier,
		&selection.Provider,
		&selection.Dimensions,
		&selection.CostPerMillionTokens,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no model available for tenant: %w", err)
		}
		return nil, fmt.Errorf("failed to get model for request: %w", err)
	}

	// Query additional info from tenant_embedding_models
	var tenantModel TenantEmbeddingModel
	err = r.db.GetContext(ctx, &tenantModel,
		`SELECT is_default, priority FROM mcp.tenant_embedding_models 
		 WHERE tenant_id = $1 AND model_id = $2`,
		tenantID, selection.ModelID)
	if err == nil {
		selection.IsDefault = tenantModel.IsDefault
		selection.Priority = tenantModel.Priority
	}

	return &selection, nil
}

// CheckUsageLimits verifies if a tenant is within usage limits for a model
func (r *TenantModelsRepositoryImpl) CheckUsageLimits(ctx context.Context, tenantID, modelID uuid.UUID) (*UsageStatus, error) {
	// Get the limits
	var limits struct {
		MonthlyTokenLimit   *int64 `db:"monthly_token_limit"`
		DailyTokenLimit     *int64 `db:"daily_token_limit"`
		MonthlyRequestLimit *int   `db:"monthly_request_limit"`
	}
	err := r.db.GetContext(ctx, &limits,
		`SELECT monthly_token_limit, daily_token_limit, monthly_request_limit
		 FROM mcp.tenant_embedding_models
		 WHERE tenant_id = $1 AND model_id = $2`,
		tenantID, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage limits: %w", err)
	}

	// Get current usage
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var monthlyUsage struct {
		TokensUsed   int64 `db:"tokens_used"`
		RequestCount int   `db:"request_count"`
	}
	err = r.db.GetContext(ctx, &monthlyUsage,
		`SELECT COALESCE(SUM(tokens_used), 0) as tokens_used, 
		        COALESCE(COUNT(*), 0) as request_count
		 FROM mcp.embedding_usage_tracking
		 WHERE tenant_id = $1 AND model_id = $2 AND created_at >= $3`,
		tenantID, modelID, startOfMonth)
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly usage: %w", err)
	}

	var dailyTokens int64
	err = r.db.GetContext(ctx, &dailyTokens,
		`SELECT COALESCE(SUM(tokens_used), 0)
		 FROM mcp.embedding_usage_tracking
		 WHERE tenant_id = $1 AND model_id = $2 AND created_at >= $3`,
		tenantID, modelID, startOfDay)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily usage: %w", err)
	}

	status := &UsageStatus{
		MonthlyTokensUsed:   monthlyUsage.TokensUsed,
		DailyTokensUsed:     dailyTokens,
		MonthlyRequestsUsed: monthlyUsage.RequestCount,
		IsWithinLimits:      true,
	}

	// Set limits and check if within bounds
	if limits.MonthlyTokenLimit != nil {
		status.MonthlyTokenLimit = *limits.MonthlyTokenLimit
		if status.MonthlyTokensUsed >= status.MonthlyTokenLimit {
			status.IsWithinLimits = false
		}
	}

	if limits.DailyTokenLimit != nil {
		status.DailyTokenLimit = *limits.DailyTokenLimit
		if status.DailyTokensUsed >= status.DailyTokenLimit {
			status.IsWithinLimits = false
		}
	}

	if limits.MonthlyRequestLimit != nil {
		status.MonthlyRequestLimit = *limits.MonthlyRequestLimit
		if status.MonthlyRequestsUsed >= status.MonthlyRequestLimit {
			status.IsWithinLimits = false
		}
	}

	return status, nil
}

// UpdateUsageLimits updates the usage limits for a tenant's model
func (r *TenantModelsRepositoryImpl) UpdateUsageLimits(ctx context.Context, tenantID, modelID uuid.UUID, monthlyTokens, dailyTokens *int64, monthlyRequests *int) error {
	query := `
		UPDATE mcp.tenant_embedding_models 
		SET monthly_token_limit = $1, daily_token_limit = $2, monthly_request_limit = $3, updated_at = $4
		WHERE tenant_id = $5 AND model_id = $6`

	result, err := r.db.ExecContext(ctx, query, monthlyTokens, dailyTokens, monthlyRequests, time.Now(), tenantID, modelID)
	if err != nil {
		return fmt.Errorf("failed to update usage limits: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("tenant model not found")
	}

	return nil
}

// BulkEnableModels enables multiple models for a tenant
func (r *TenantModelsRepositoryImpl) BulkEnableModels(ctx context.Context, tenantID uuid.UUID, modelIDs []uuid.UUID) error {
	if len(modelIDs) == 0 {
		return nil
	}

	query := `
		UPDATE mcp.tenant_embedding_models 
		SET is_enabled = true, updated_at = $1
		WHERE tenant_id = $2 AND model_id = ANY($3)`

	_, err := r.db.ExecContext(ctx, query, time.Now(), tenantID, modelIDs)
	if err != nil {
		return fmt.Errorf("failed to bulk enable models: %w", err)
	}

	r.logger.Info("Models enabled", map[string]interface{}{
		"tenant_id": tenantID,
		"count":     len(modelIDs),
	})

	return nil
}

// BulkDisableModels disables multiple models for a tenant
func (r *TenantModelsRepositoryImpl) BulkDisableModels(ctx context.Context, tenantID uuid.UUID, modelIDs []uuid.UUID) error {
	if len(modelIDs) == 0 {
		return nil
	}

	query := `
		UPDATE mcp.tenant_embedding_models 
		SET is_enabled = false, updated_at = $1
		WHERE tenant_id = $2 AND model_id = ANY($3)`

	_, err := r.db.ExecContext(ctx, query, time.Now(), tenantID, modelIDs)
	if err != nil {
		return fmt.Errorf("failed to bulk disable models: %w", err)
	}

	r.logger.Info("Models disabled", map[string]interface{}{
		"tenant_id": tenantID,
		"count":     len(modelIDs),
	})

	return nil
}
