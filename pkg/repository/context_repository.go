package repository

import (
	"context"
	"time"
)

// Context represents a persistent context object
type Context struct {
	ID         string         `json:"id" db:"id"`
	TenantID   string         `json:"tenant_id" db:"tenant_id"`
	Name       string         `json:"name" db:"name"`
	AgentID    string         `json:"agent_id" db:"agent_id"`
	SessionID  string         `json:"session_id" db:"session_id"`
	Status     string         `json:"status" db:"status"`
	Properties map[string]any `json:"properties" db:"properties"`
	CreatedAt  int64          `json:"created_at" db:"created_at"`
	UpdatedAt  int64          `json:"updated_at" db:"updated_at"`
}

// ContextItem represents an item in a context
type ContextItem struct {
	ID        string         `json:"id" db:"id"`
	ContextID string         `json:"context_id" db:"context_id"`
	Content   string         `json:"content" db:"content"`
	Type      string         `json:"type" db:"type"`
	Score     float64        `json:"score" db:"score"`
	Metadata  map[string]any `json:"metadata" db:"metadata"`
}

// Story 1.2: New Types for Semantic Context
// ContextEmbeddingLink represents the relationship between context and embedding
type ContextEmbeddingLink struct {
	ID              string    `json:"id" db:"id"`
	ContextID       string    `json:"context_id" db:"context_id"`
	EmbeddingID     string    `json:"embedding_id" db:"embedding_id"`
	ChunkSequence   int       `json:"chunk_sequence" db:"chunk_sequence"`
	ImportanceScore float64   `json:"importance_score" db:"importance_score"`
	IsSummary       bool      `json:"is_summary" db:"is_summary"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
}

// ContextRepository defines methods for interacting with contexts
type ContextRepository interface {
	// Create creates a new context
	Create(ctx context.Context, contextObj *Context) error

	// Get retrieves a context by ID
	Get(ctx context.Context, id string) (*Context, error)

	// Update updates an existing context
	Update(ctx context.Context, contextObj *Context) error

	// Delete deletes a context by ID
	Delete(ctx context.Context, id string) error

	// List lists contexts with optional filtering
	List(ctx context.Context, filter map[string]any) ([]*Context, error)

	// Search searches for text within a context
	Search(ctx context.Context, contextID, query string) ([]ContextItem, error)

	// Summarize generates a summary of a context
	Summarize(ctx context.Context, contextID string) (string, error)

	// Story 1.2: Extended Context Repository Interface for Semantic Operations

	// AddContextItem adds a single item to a context
	AddContextItem(ctx context.Context, contextID string, item *ContextItem) error

	// GetContextItems retrieves all items for a context
	GetContextItems(ctx context.Context, contextID string) ([]*ContextItem, error)

	// UpdateContextItem updates an existing context item
	UpdateContextItem(ctx context.Context, item *ContextItem) error

	// UpdateCompactionMetadata updates compaction tracking information
	UpdateCompactionMetadata(ctx context.Context, contextID string, strategy string, lastCompactedAt time.Time) error

	// GetContextsNeedingCompaction returns contexts that need compaction based on threshold
	GetContextsNeedingCompaction(ctx context.Context, threshold int) ([]*Context, error)

	// LinkEmbeddingToContext creates a link between an embedding and a context
	LinkEmbeddingToContext(ctx context.Context, contextID string, embeddingID string, sequence int, importance float64) error

	// GetContextEmbeddingLinks retrieves all embedding links for a context
	GetContextEmbeddingLinks(ctx context.Context, contextID string) ([]ContextEmbeddingLink, error)
}
