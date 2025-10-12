package models

import (
	"slices"
	"time"
)

// Event represents an MCP event
type Event struct {
	// Source is the source of the event (e.g., openai, anthropic, langchain)
	Source string `json:"source"`

	// Type is the type of the event (e.g., context_update, model_request)
	Type string `json:"type"`

	// Timestamp is when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// Data contains the event data
	Data any `json:"data"`

	// AgentID is the identifier for the AI agent that generated this event
	AgentID string `json:"agent_id"`

	// SessionID is the identifier for the user session
	SessionID string `json:"session_id,omitempty"`
}

// ContextItem represents a single item in a context
type ContextItem struct {
	// ID is the unique identifier for this context item
	ID string `json:"id,omitempty" db:"id"`

	// ContextID is the ID of the context this item belongs to
	ContextID string `json:"context_id,omitempty" db:"context_id"`

	// Role is the role of this context item (e.g., user, assistant, system)
	Role string `json:"role" db:"role"`

	// Content is the text content of this context item
	Content string `json:"content" db:"content"`

	// Timestamp is when this context item was created
	Timestamp time.Time `json:"timestamp" db:"created_at"`

	// Tokens is the token count for this context item
	Tokens int `json:"tokens,omitempty" db:"token_count"`

	// Metadata contains additional information about this context item
	Metadata map[string]any `json:"metadata,omitempty" db:"metadata"`
}

// Context represents an AI model context
type Context struct {
	// ID is the unique identifier for this context
	ID string `json:"id" db:"id"`

	// Type is the type of context (e.g., conversation, task, etc.)
	Type string `json:"type" db:"type"`

	// Name is the display name of this context (not persisted to database - metadata field instead)
	Name string `json:"name,omitempty" db:"-"`

	// Description is a human-readable description of the context (not persisted to database - metadata field instead)
	Description string `json:"description,omitempty" db:"-"`

	// AgentID is the identifier for the AI agent that owns this context
	AgentID string `json:"agent_id,omitempty" db:"agent_id"`

	// TenantID is the tenant this context belongs to
	TenantID string `json:"tenant_id" db:"tenant_id"`

	// ModelID identifies which AI model this context is for
	ModelID string `json:"model_id,omitempty" db:"model_id"`

	// SessionID is the identifier for the user session
	SessionID string `json:"session_id,omitempty" db:"session_id"`

	// Content contains the actual context data
	Content []ContextItem `json:"content,omitempty" db:"-"`

	// Metadata contains additional information about the context
	Metadata map[string]any `json:"metadata,omitempty" db:"metadata"`

	// CreatedAt is when this context was created
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// UpdatedAt is when this context was last updated
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// ExpiresAt is when this context expires (if applicable)
	ExpiresAt time.Time `json:"expires_at,omitempty" db:"expires_at"`

	// MaxTokens is the maximum number of tokens this context can contain
	MaxTokens int `json:"max_tokens,omitempty" db:"max_tokens"`

	// CurrentTokens is the current token count for this context (maps to token_count in DB)
	CurrentTokens int `json:"current_tokens,omitempty" db:"token_count"`
}

// ContextUpdateOptions provides options for updating a context
type ContextUpdateOptions struct {
	// ReplaceContent indicates whether to replace the entire content or append to it
	ReplaceContent bool `json:"replace_content,omitempty"`

	// Truncate indicates whether to truncate the context if it exceeds the maximum tokens
	Truncate bool `json:"truncate,omitempty"`

	// TruncateStrategy defines the strategy for truncating the context
	TruncateStrategy string `json:"truncate_strategy,omitempty"`
}

// EventHandler is a function that processes an event
type EventHandler func(Event) error

// EventFilter defines criteria for filtering events
type EventFilter struct {
	Sources    []string  `json:"sources"`
	Types      []string  `json:"types"`
	AgentIDs   []string  `json:"agent_ids,omitempty"`
	SessionIDs []string  `json:"session_ids,omitempty"`
	After      time.Time `json:"after"`
	Before     time.Time `json:"before"`
}

// MatchEvent checks if an event matches the filter criteria
func (f *EventFilter) MatchEvent(event Event) bool {
	// Check sources
	if len(f.Sources) > 0 {
		if !slices.Contains(f.Sources, event.Source) {
			return false
		}
	}

	// Check types
	if len(f.Types) > 0 {
		if !slices.Contains(f.Types, event.Type) {
			return false
		}
	}

	// Check agent IDs
	if len(f.AgentIDs) > 0 {
		if !slices.Contains(f.AgentIDs, event.AgentID) {
			return false
		}
	}

	// Check session IDs
	if len(f.SessionIDs) > 0 {
		if !slices.Contains(f.SessionIDs, event.SessionID) {
			return false
		}
	}

	// Check timestamp
	if !f.After.IsZero() && event.Timestamp.Before(f.After) {
		return false
	}
	if !f.Before.IsZero() && event.Timestamp.After(f.Before) {
		return false
	}

	return true
}
