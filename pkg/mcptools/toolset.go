package mcptools

import (
	"context"
	"encoding/json"
	"fmt"
)

// Toolset represents a logical grouping of related tools
// This follows GitHub's MCP server pattern of organizing tools into functional groups
type Toolset struct {
	Name        string   `json:"name"`                  // Unique toolset identifier (e.g., "github_repos")
	DisplayName string   `json:"display_name"`          // Human-readable name (e.g., "GitHub Repositories")
	Description string   `json:"description"`           // What this toolset does
	Category    string   `json:"category"`              // Primary category
	Icon        string   `json:"icon,omitempty"`        // Optional emoji icon
	Enabled     bool     `json:"enabled"`               // Whether this toolset is currently enabled
	Tools       []string `json:"tools"`                 // List of tool names in this toolset
	Tags        []string `json:"tags,omitempty"`        // Capability tags
	Version     string   `json:"version,omitempty"`     // Toolset version
	Provider    string   `json:"provider,omitempty"`    // Provider name (e.g., "github", "harness")

	// Tool generation function - generates tool definitions on demand
	// This enables lazy loading: only generate tool definitions when needed
	ToolGenerator ToolsetGenerator `json:"-"`
}

// ToolsetGenerator is a function that generates tool definitions for a toolset
// Parameters:
//   - detailLevel: "name" (just names), "description" (name+description), "full" (complete schema)
//   - actions: optional list of specific actions to generate (nil = all actions)
type ToolsetGenerator func(detailLevel DetailLevel, actions []string) ([]ToolDefinition, error)

// DetailLevel controls how much information is returned for tools
type DetailLevel string

const (
	DetailLevelName        DetailLevel = "name"        // Just tool name
	DetailLevelDescription DetailLevel = "description" // Name + description
	DetailLevelFull        DetailLevel = "full"        // Complete tool definition with schema
)

// ToolsetProvider defines the interface for providing toolsets
type ToolsetProvider interface {
	// GetToolsets returns all toolsets provided by this provider
	GetToolsets() []Toolset

	// GetToolsetByName returns a specific toolset by name
	GetToolsetByName(name string) (Toolset, bool)
}

// ToolsetAction represents an action within a toolset
// This is used for toolsets that support multiple actions (e.g., get, list, create, update, delete)
type ToolsetAction struct {
	Name        string                 `json:"name"`           // Action name (e.g., "get", "list", "create")
	Description string                 `json:"description"`    // What this action does
	Parameters  map[string]interface{} `json:"parameters"`     // JSON schema for parameters
	Handler     ToolsetActionHandler   `json:"-"`              // Handler function
	Tags        []string               `json:"tags,omitempty"` // Capability tags for this action
}

// ToolsetActionHandler is a function that executes a toolset action
type ToolsetActionHandler func(ctx context.Context, action string, args json.RawMessage) (interface{}, error)

// UnifiedToolHandler creates a unified tool handler for action-based toolsets
// This wraps a ToolsetActionHandler and routes to the appropriate action
func UnifiedToolHandler(handler ToolsetActionHandler) ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		// Parse arguments to extract action
		var params struct {
			Action string `json:"action"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		if params.Action == "" {
			return nil, fmt.Errorf("action parameter is required")
		}

		// Execute the action
		return handler(ctx, params.Action, args)
	}
}
