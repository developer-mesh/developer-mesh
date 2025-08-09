package tenant_models

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TenantEmbeddingModel represents a tenant's configuration for an embedding model
type TenantEmbeddingModel struct {
	ID                         uuid.UUID  `db:"id" json:"id"`
	TenantID                   uuid.UUID  `db:"tenant_id" json:"tenant_id"`
	ModelID                    uuid.UUID  `db:"model_id" json:"model_id"`
	IsEnabled                  bool       `db:"is_enabled" json:"is_enabled"`
	IsDefault                  bool       `db:"is_default" json:"is_default"`
	CustomCostPerMillionTokens *float64   `db:"custom_cost_per_million_tokens" json:"custom_cost_per_million_tokens,omitempty"`
	CustomCostPerMillionChars  *float64   `db:"custom_cost_per_million_chars" json:"custom_cost_per_million_chars,omitempty"`
	MonthlyTokenLimit          *int64     `db:"monthly_token_limit" json:"monthly_token_limit,omitempty"`
	DailyTokenLimit            *int64     `db:"daily_token_limit" json:"daily_token_limit,omitempty"`
	MonthlyRequestLimit        *int       `db:"monthly_request_limit" json:"monthly_request_limit,omitempty"`
	Priority                   int        `db:"priority" json:"priority"`
	FallbackModelID            *uuid.UUID `db:"fallback_model_id" json:"fallback_model_id,omitempty"`
	AgentPreferences           []byte     `db:"agent_preferences" json:"agent_preferences,omitempty"`
	CreatedAt                  time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt                  time.Time  `db:"updated_at" json:"updated_at"`
	CreatedBy                  *uuid.UUID `db:"created_by" json:"created_by,omitempty"`
}

// ModelSelection represents the result of model selection for a request
type ModelSelection struct {
	ModelID              uuid.UUID `json:"model_id"`
	ModelIdentifier      string    `json:"model_identifier"`
	Provider             string    `json:"provider"`
	Dimensions           int       `json:"dimensions"`
	CostPerMillionTokens float64   `json:"cost_per_million_tokens"`
	IsDefault            bool      `json:"is_default"`
	Priority             int       `json:"priority"`
}

// UsageStatus represents current usage against limits
type UsageStatus struct {
	MonthlyTokensUsed   int64 `json:"monthly_tokens_used"`
	MonthlyTokenLimit   int64 `json:"monthly_token_limit"`
	DailyTokensUsed     int64 `json:"daily_tokens_used"`
	DailyTokenLimit     int64 `json:"daily_token_limit"`
	MonthlyRequestsUsed int   `json:"monthly_requests_used"`
	MonthlyRequestLimit int   `json:"monthly_request_limit"`
	IsWithinLimits      bool  `json:"is_within_limits"`
}

// TenantModelsRepository defines the interface for tenant model operations
type TenantModelsRepository interface {
	// GetTenantModel retrieves a specific tenant-model configuration
	GetTenantModel(ctx context.Context, tenantID, modelID uuid.UUID) (*TenantEmbeddingModel, error)

	// ListTenantModels returns all models configured for a tenant
	ListTenantModels(ctx context.Context, tenantID uuid.UUID, enabledOnly bool) ([]*TenantEmbeddingModel, error)

	// GetDefaultModel returns the default model for a tenant
	GetDefaultModel(ctx context.Context, tenantID uuid.UUID) (*TenantEmbeddingModel, error)

	// CreateTenantModel adds a model configuration for a tenant
	CreateTenantModel(ctx context.Context, model *TenantEmbeddingModel) error

	// UpdateTenantModel updates a tenant's model configuration
	UpdateTenantModel(ctx context.Context, model *TenantEmbeddingModel) error

	// DeleteTenantModel removes a model configuration for a tenant
	DeleteTenantModel(ctx context.Context, tenantID, modelID uuid.UUID) error

	// SetDefaultModel sets a model as the default for a tenant
	SetDefaultModel(ctx context.Context, tenantID, modelID uuid.UUID) error

	// UpdatePriority updates the priority of a tenant's model
	UpdatePriority(ctx context.Context, tenantID, modelID uuid.UUID, priority int) error

	// GetModelForRequest selects the best model for a request using the SQL function
	GetModelForRequest(ctx context.Context, tenantID uuid.UUID, agentID *uuid.UUID, taskType, requestedModel *string) (*ModelSelection, error)

	// CheckUsageLimits verifies if a tenant is within usage limits for a model
	CheckUsageLimits(ctx context.Context, tenantID, modelID uuid.UUID) (*UsageStatus, error)

	// UpdateUsageLimits updates the usage limits for a tenant's model
	UpdateUsageLimits(ctx context.Context, tenantID, modelID uuid.UUID, monthlyTokens, dailyTokens *int64, monthlyRequests *int) error

	// BulkEnableModels enables multiple models for a tenant
	BulkEnableModels(ctx context.Context, tenantID uuid.UUID, modelIDs []uuid.UUID) error

	// BulkDisableModels disables multiple models for a tenant
	BulkDisableModels(ctx context.Context, tenantID uuid.UUID, modelIDs []uuid.UUID) error
}
