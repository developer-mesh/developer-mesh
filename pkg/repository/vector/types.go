// Package vector provides interfaces and types for vector embeddings
package vector

import (
	"context"
	"time"
)

// Filter defines a filter map for repository operations
// This avoids importing pkg/repository to prevent import cycles
type Filter map[string]any

// FilterFromContextID creates a filter for context ID
func FilterFromContextID(contextID string) Filter {
	return Filter{"context_id": contextID}
}

// FilterFromModelAndContext creates a filter for model ID and context ID
func FilterFromModelAndContext(contextID, modelID string) Filter {
	return Filter{
		"context_id": contextID,
		"model_id":   modelID,
	}
}

// Embedding represents a vector embedding with its metadata
type Embedding struct {
	ID           string         `json:"id" db:"id"`
	ContextID    string         `json:"context_id" db:"context_id"`
	ContentIndex int            `json:"content_index" db:"content_index"`
	Text         string         `json:"text" db:"content"` // db column is 'content', not 'text'
	Embedding    []float32      `json:"embedding" db:"embedding"`
	ModelID      string         `json:"model_id" db:"model_id"`
	CreatedAt    time.Time      `json:"created_at" db:"created_at"`
	Metadata     map[string]any `json:"metadata" db:"metadata"`
}

// Repository defines the interface for vector embedding operations
// It follows the generic repository pattern while preserving vector-specific operations
type Repository interface {
	// Core repository methods - aligned with generic Repository[T] interface
	// Create stores a new embedding
	Create(ctx context.Context, embedding *Embedding) error
	// Get retrieves an embedding by its ID
	Get(ctx context.Context, id string) (*Embedding, error)
	// List retrieves embeddings matching the provided filter
	List(ctx context.Context, filter Filter) ([]*Embedding, error)
	// Update modifies an existing embedding
	Update(ctx context.Context, embedding *Embedding) error
	// Delete removes an embedding by its ID
	Delete(ctx context.Context, id string) error

	// Vector-specific methods
	// StoreEmbedding stores a vector embedding (alias for Create method)
	StoreEmbedding(ctx context.Context, embedding *Embedding) error

	// SearchEmbeddings performs a vector search with various filter options
	SearchEmbeddings(
		ctx context.Context,
		queryVector []float32,
		contextID string,
		modelID string,
		limit int,
		similarityThreshold float64,
	) ([]*Embedding, error)

	// SearchEmbeddings_Legacy performs a legacy vector search
	SearchEmbeddings_Legacy(
		ctx context.Context,
		queryVector []float32,
		contextID string,
		limit int,
	) ([]*Embedding, error)

	// GetContextEmbeddings retrieves all embeddings for a context
	GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error)

	// DeleteContextEmbeddings deletes all embeddings for a context
	DeleteContextEmbeddings(ctx context.Context, contextID string) error

	// GetEmbeddingsByModel retrieves all embeddings for a context and model
	GetEmbeddingsByModel(
		ctx context.Context,
		contextID string,
		modelID string,
	) ([]*Embedding, error)

	// GetSupportedModels returns a list of models with embeddings
	GetSupportedModels(ctx context.Context) ([]string, error)

	// DeleteModelEmbeddings deletes all embeddings for a specific model in a context
	DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error

	// Context-specific embedding operations (Story 2.1)
	// StoreContextEmbedding stores an embedding and links it to a context with metadata
	StoreContextEmbedding(ctx context.Context, contextID string, embedding *Embedding, sequence int, importance float64) (string, error)
	// GetContextEmbeddingsBySequence retrieves embeddings for a context within a sequence range
	GetContextEmbeddingsBySequence(ctx context.Context, contextID string, startSeq int, endSeq int) ([]*Embedding, error)
	// UpdateEmbeddingImportance updates the importance score for an embedding
	UpdateEmbeddingImportance(ctx context.Context, embeddingID string, importance float64) error
}
