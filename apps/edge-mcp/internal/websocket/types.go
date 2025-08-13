package websocket

import (
	"time"
)

// ContextStats represents statistics for a context
type ContextStats struct {
	TotalTokens     int
	MessageCount    int
	ToolInvocations int
	CreatedAt       time.Time
	LastAccessed    time.Time
}

// TruncatedContext represents a truncated context
type TruncatedContext struct {
	ID         string
	TokenCount int
}

// ToolExecutionStatus represents the status of a tool execution
type ToolExecutionStatus struct {
	ExecutionID string
	Status      string // "running", "completed", "failed", "cancelled"
	StartedAt   time.Time
	CompletedAt *time.Time
	Result      interface{}
	Error       string
}

// Tool represents a tool definition
type Tool struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}
