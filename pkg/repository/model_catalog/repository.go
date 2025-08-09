package model_catalog

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// EmbeddingModel represents a model in the catalog
type EmbeddingModel struct {
	ID                              uuid.UUID  `db:"id" json:"id"`
	Provider                        string     `db:"provider" json:"provider"`
	ModelName                       string     `db:"model_name" json:"model_name"`
	ModelID                         string     `db:"model_id" json:"model_id"`
	ModelVersion                    *string    `db:"model_version" json:"model_version,omitempty"`
	Dimensions                      int        `db:"dimensions" json:"dimensions"`
	MaxTokens                       *int       `db:"max_tokens" json:"max_tokens,omitempty"`
	SupportsBinary                  bool       `db:"supports_binary" json:"supports_binary"`
	SupportsDimensionalityReduction bool       `db:"supports_dimensionality_reduction" json:"supports_dimensionality_reduction"`
	MinDimensions                   *int       `db:"min_dimensions" json:"min_dimensions,omitempty"`
	ModelType                       string     `db:"model_type" json:"model_type"`
	CostPerMillionTokens            *float64   `db:"cost_per_million_tokens" json:"cost_per_million_tokens,omitempty"`
	CostPerMillionChars             *float64   `db:"cost_per_million_chars" json:"cost_per_million_chars,omitempty"`
	IsAvailable                     bool       `db:"is_available" json:"is_available"`
	IsDeprecated                    bool       `db:"is_deprecated" json:"is_deprecated"`
	DeprecationDate                 *time.Time `db:"deprecation_date" json:"deprecation_date,omitempty"`
	MinimumTier                     *string    `db:"minimum_tier" json:"minimum_tier,omitempty"`
	RequiresAPIKey                  bool       `db:"requires_api_key" json:"requires_api_key"`
	ProviderConfig                  []byte     `db:"provider_config" json:"provider_config,omitempty"`
	Capabilities                    []byte     `db:"capabilities" json:"capabilities,omitempty"`
	PerformanceMetrics              []byte     `db:"performance_metrics" json:"performance_metrics,omitempty"`
	CreatedAt                       time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt                       time.Time  `db:"updated_at" json:"updated_at"`
}

// ModelFilter defines filtering options for model queries
type ModelFilter struct {
	Provider     *string `json:"provider,omitempty"`
	ModelType    *string `json:"model_type,omitempty"`
	IsAvailable  *bool   `json:"is_available,omitempty"`
	IsDeprecated *bool   `json:"is_deprecated,omitempty"`
	MinimumTier  *string `json:"minimum_tier,omitempty"`
}

// ModelCatalogRepository defines the interface for model catalog operations
type ModelCatalogRepository interface {
	// GetByID retrieves a model by its UUID
	GetByID(ctx context.Context, id uuid.UUID) (*EmbeddingModel, error)

	// GetByModelID retrieves a model by its model_id (e.g., "amazon.titan-embed-text-v2:0")
	GetByModelID(ctx context.Context, modelID string) (*EmbeddingModel, error)

	// ListAvailable returns all available (non-deprecated) models
	ListAvailable(ctx context.Context, filter *ModelFilter) ([]*EmbeddingModel, error)

	// ListAll returns all models including deprecated ones
	ListAll(ctx context.Context, filter *ModelFilter) ([]*EmbeddingModel, error)

	// Create adds a new model to the catalog
	Create(ctx context.Context, model *EmbeddingModel) error

	// Update modifies an existing model
	Update(ctx context.Context, model *EmbeddingModel) error

	// Delete removes a model from the catalog
	Delete(ctx context.Context, id uuid.UUID) error

	// SetAvailability updates the availability status of a model
	SetAvailability(ctx context.Context, id uuid.UUID, isAvailable bool) error

	// MarkDeprecated marks a model as deprecated
	MarkDeprecated(ctx context.Context, id uuid.UUID, deprecationDate *time.Time) error

	// GetProviders returns a list of unique providers
	GetProviders(ctx context.Context) ([]string, error)

	// BulkUpsert inserts or updates multiple models
	BulkUpsert(ctx context.Context, models []*EmbeddingModel) error
}
