package embedding

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Provider constants
const (
	ProviderOpenAI = "openai"
	ProviderVoyage = "voyage" // Anthropic's partner
	ProviderAmazon = "amazon"
	ProviderCohere = "cohere" // Available on Bedrock
	ProviderGoogle = "google"
)

// Model type constants
const (
	ModelTypeText       = "text"
	ModelTypeCode       = "code"
	ModelTypeMultimodal = "multimodal"
)

// Model represents an embedding model
type Model struct {
	ID                              uuid.UUID       `json:"id" db:"id"`
	Provider                        string          `json:"provider" db:"provider"`
	ModelName                       string          `json:"model_name" db:"model_name"`
	ModelVersion                    *string         `json:"model_version,omitempty" db:"model_version"`
	Dimensions                      int             `json:"dimensions" db:"dimensions"`
	MaxTokens                       *int            `json:"max_tokens,omitempty" db:"max_tokens"`
	SupportsBinary                  bool            `json:"supports_binary" db:"supports_binary"`
	SupportsDimensionalityReduction bool            `json:"supports_dimensionality_reduction" db:"supports_dimensionality_reduction"`
	MinDimensions                   *int            `json:"min_dimensions,omitempty" db:"min_dimensions"`
	CostPerMillionTokens            *float64        `json:"cost_per_million_tokens,omitempty" db:"cost_per_million_tokens"`
	ModelID                         *string         `json:"model_id,omitempty" db:"model_id"` // For Bedrock models
	ModelType                       *string         `json:"model_type,omitempty" db:"model_type"`
	IsActive                        bool            `json:"is_active" db:"is_active"`
	Capabilities                    json.RawMessage `json:"capabilities" db:"capabilities"`
	CreatedAt                       time.Time       `json:"created_at" db:"created_at"`
}

// Embedding represents a stored embedding
type Embedding struct {
	ID                   uuid.UUID       `json:"id" db:"id"`
	ContextID            uuid.UUID       `json:"context_id" db:"context_id"`
	ContentIndex         int             `json:"content_index" db:"content_index"`
	ChunkIndex           int             `json:"chunk_index" db:"chunk_index"`
	Content              string          `json:"content" db:"content"`
	ContentHash          string          `json:"content_hash" db:"content_hash"`
	ContentTokens        *int            `json:"content_tokens,omitempty" db:"content_tokens"`
	ModelID              uuid.UUID       `json:"model_id" db:"model_id"`
	ModelProvider        string          `json:"model_provider" db:"model_provider"`
	ModelName            string          `json:"model_name" db:"model_name"`
	ModelDimensions      int             `json:"model_dimensions" db:"model_dimensions"`
	ConfiguredDimensions *int            `json:"configured_dimensions,omitempty" db:"configured_dimensions"`
	ProcessingTimeMS     *int            `json:"processing_time_ms,omitempty" db:"processing_time_ms"`
	EmbeddingCreatedAt   time.Time       `json:"embedding_created_at" db:"embedding_created_at"`
	Magnitude            float64         `json:"magnitude" db:"magnitude"`
	TenantID             uuid.UUID       `json:"tenant_id" db:"tenant_id"`
	Metadata             json.RawMessage `json:"metadata" db:"metadata"`
	CreatedAt            time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at" db:"updated_at"`
}

// InsertRequest represents a request to insert an embedding
type InsertRequest struct {
	ContextID            *uuid.UUID      `json:"context_id,omitempty"` // Optional context reference
	Content              string          `json:"content"`
	Embedding            []float32       `json:"embedding"`
	ModelName            string          `json:"model_name"`
	TenantID             uuid.UUID       `json:"tenant_id"`
	Metadata             json.RawMessage `json:"metadata,omitempty"`
	ContentIndex         int             `json:"content_index"`
	ChunkIndex           int             `json:"chunk_index"`
	ConfiguredDimensions *int            `json:"configured_dimensions,omitempty"` // For models that support reduction
}

// SearchRequest represents a similarity search request
type SearchRequest struct {
	QueryEmbedding []float32       `json:"query_embedding"`
	ModelName      string          `json:"model_name"`
	TenantID       uuid.UUID       `json:"tenant_id"`
	ContextID      *uuid.UUID      `json:"context_id,omitempty"`
	Limit          int             `json:"limit"`
	Threshold      float64         `json:"threshold"`
	MetadataFilter json.RawMessage `json:"metadata_filter,omitempty"` // JSONB filter
}

// EmbeddingSearchResult represents a search result
type EmbeddingSearchResult struct {
	ID            uuid.UUID       `json:"id" db:"id"`
	ContextID     uuid.UUID       `json:"context_id" db:"context_id"`
	Content       string          `json:"content" db:"content"`
	Similarity    float64         `json:"similarity" db:"similarity"`
	Metadata      json.RawMessage `json:"metadata" db:"metadata"`
	ModelProvider string          `json:"model_provider" db:"model_provider"`
}

// ModelFilter for querying available models
type ModelFilter struct {
	Provider  *string `json:"provider,omitempty"`
	ModelType *string `json:"model_type,omitempty"`
	IsActive  *bool   `json:"is_active,omitempty"`
}
