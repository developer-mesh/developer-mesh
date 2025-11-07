package mcptools

import (
	"context"
	"encoding/json"
)

// ToolDefinition defines a tool with all its metadata
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Handler     ToolHandler            `json:"-"`

	// Enhanced metadata for AI agents
	Category string   `json:"category,omitempty"` // Primary category (repository, issues, ci/cd, etc.)
	Tags     []string `json:"tags,omitempty"`     // Tags for capabilities (read, write, delete, etc.)

	// Relationships and compatibility
	Prerequisites    []string         `json:"prerequisites,omitempty"`      // Tools that must be executed before this tool
	CommonlyUsedWith []string         `json:"commonly_used_with,omitempty"` // Tools frequently used together
	NextSteps        []string         `json:"next_steps,omitempty"`         // Recommended follow-up tools
	Alternatives     []string         `json:"alternatives,omitempty"`       // Alternative tools that can be used instead
	ConflictsWith    []string         `json:"conflicts_with,omitempty"`     // Tools that should not be used together
	IOCompatibility  *IOCompatibility `json:"io_compatibility,omitempty"`   // Input/output type information
}

// ToolHandler is a function that executes a tool
type ToolHandler func(ctx context.Context, args json.RawMessage) (interface{}, error)

// ToolRelationship defines relationships between tools
type ToolRelationship struct {
	// Prerequisites are tools that must be executed before this tool
	Prerequisites []string `json:"prerequisites,omitempty"`

	// CommonlyUsedWith are tools frequently used together with this tool
	CommonlyUsedWith []string `json:"commonly_used_with,omitempty"`

	// NextSteps are recommended follow-up tools after this tool
	NextSteps []string `json:"next_steps,omitempty"`

	// Alternatives are tools that can be used instead of this tool
	Alternatives []string `json:"alternatives,omitempty"`

	// ConflictsWith are tools that should not be used with this tool
	ConflictsWith []string `json:"conflicts_with,omitempty"`
}

// IOCompatibility defines input/output type compatibility between tools
type IOCompatibility struct {
	InputType  DataType `json:"input_type"`
	OutputType DataType `json:"output_type"`
}

// DataType represents the type of data a tool accepts or produces
type DataType struct {
	Format      string                 `json:"format"`               // e.g., "json", "text", "binary"
	Schema      string                 `json:"schema"`               // e.g., "issue", "pull_request", "workflow"
	ContentType string                 `json:"content_type"`         // MIME type
	Properties  map[string]interface{} `json:"properties,omitempty"` // Additional schema properties
}

// WorkflowTemplate represents a suggested workflow of tools
type WorkflowTemplate struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Steps       []WorkflowStep `json:"steps"`
	Tags        []string       `json:"tags"`
}

// WorkflowStep represents a single step in a workflow
type WorkflowStep struct {
	Order       int                    `json:"order"`
	ToolName    string                 `json:"tool_name"`
	Description string                 `json:"description"`
	Required    bool                   `json:"required"`
	Condition   string                 `json:"condition,omitempty"`  // Condition for executing this step
	InputFrom   string                 `json:"input_from,omitempty"` // Previous step to get input from
	Parameters  map[string]interface{} `json:"parameters,omitempty"` // Default parameters
}
