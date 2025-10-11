// Story 1.3: SemanticContextManager Interface
// Package repository provides data access interfaces for the DevMesh platform

package repository

import (
	"context"
	"time"
)

// CompactionStrategy defines how to compact context
type CompactionStrategy string

const (
	// CompactionSummarize uses LLM to create summaries of old context
	CompactionSummarize CompactionStrategy = "summarize"
	// CompactionPrune removes low-importance items
	CompactionPrune CompactionStrategy = "prune"
	// CompactionSemantic uses semantic similarity to deduplicate
	CompactionSemantic CompactionStrategy = "semantic"
	// CompactionSliding uses sliding window approach
	CompactionSliding CompactionStrategy = "sliding"
	// CompactionToolClear clears old tool execution results
	CompactionToolClear CompactionStrategy = "tool_clear"
)

// RetrievalOptions configures how context is retrieved
type RetrievalOptions struct {
	// IncludeEmbeddings whether to include embedding vectors in response
	IncludeEmbeddings bool
	// MaxTokens maximum tokens to return
	MaxTokens int
	// RelevanceQuery query for semantic retrieval
	RelevanceQuery string
	// TimeRange optional time window filter
	TimeRange *TimeRange
	// MinSimilarity minimum similarity threshold for semantic search
	MinSimilarity float64
}

// TimeRange specifies a time window
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// ContextUpdate represents an update to context
type ContextUpdate struct {
	// Role the role of the message (user, assistant, system, tool)
	Role string
	// Content the actual content
	Content string
	// Metadata additional metadata for the update
	Metadata map[string]interface{}
}

// CreateContextRequest contains data for creating a new context
type CreateContextRequest struct {
	// Name human-readable name for the context
	Name string
	// AgentID the agent this context belongs to
	AgentID string
	// SessionID the session this context is associated with
	SessionID string
	// Properties additional properties
	Properties map[string]interface{}
	// MaxTokens maximum tokens for this context
	MaxTokens int
}

// SemanticContextManager manages context with semantic awareness
// This interface extends basic context management with semantic search,
// intelligent compaction, and embedding-based retrieval capabilities.
type SemanticContextManager interface {
	// Core CRUD with semantic awareness

	// CreateContext creates a new context with semantic capabilities
	CreateContext(ctx context.Context, req *CreateContextRequest) (*Context, error)

	// GetContext retrieves a context with optional semantic filtering
	GetContext(ctx context.Context, contextID string, opts *RetrievalOptions) (*Context, error)

	// UpdateContext updates context and generates embeddings automatically
	UpdateContext(ctx context.Context, contextID string, update *ContextUpdate) error

	// DeleteContext removes a context and its associated embeddings
	DeleteContext(ctx context.Context, contextID string) error

	// Semantic operations

	// SearchContext performs semantic search within a context
	// Returns items ranked by relevance to the query
	SearchContext(ctx context.Context, query string, contextID string, limit int) ([]*ContextItem, error)

	// CompactContext applies compaction strategy to reduce context size
	CompactContext(ctx context.Context, contextID string, strategy CompactionStrategy) error

	// GetRelevantContext retrieves semantically relevant context items
	// Uses embedding similarity to pack most relevant items within token budget
	GetRelevantContext(ctx context.Context, contextID string, query string, maxTokens int) (*Context, error)

	// Lifecycle management

	// PromoteToHot moves context to hot tier (fast access)
	PromoteToHot(ctx context.Context, contextID string) error

	// ArchiveToCold moves context to cold storage (archival)
	ArchiveToCold(ctx context.Context, contextID string) error

	// Security & Compliance

	// AuditContextAccess logs access to context for compliance
	AuditContextAccess(ctx context.Context, contextID string, operation string) error

	// ValidateContextIntegrity verifies context data integrity
	ValidateContextIntegrity(ctx context.Context, contextID string) error
}
