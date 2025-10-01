package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// ToolDefinition defines a tool
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Handler     ToolHandler            `json:"-"`

	// Enhanced metadata for AI agents
	Category string   `json:"category,omitempty"` // Primary category (repository, issues, ci/cd, etc.)
	Tags     []string `json:"tags,omitempty"`     // Tags for capabilities (read, write, delete, etc.)
}

// ToolHandler is a function that executes a tool
type ToolHandler func(ctx context.Context, args json.RawMessage) (interface{}, error)

// Registry manages tools
type Registry struct {
	tools map[string]ToolDefinition
	mu    sync.RWMutex
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]ToolDefinition),
	}
}

// Register registers a tool provider
func (r *Registry) Register(provider interface{ GetDefinitions() []ToolDefinition }) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, def := range provider.GetDefinitions() {
		r.tools[def.Name] = def
	}
}

// RegisterRemote registers a remote tool
func (r *Registry) RegisterRemote(tool ToolDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools[tool.Name] = tool
}

// ListAll returns all tools
func (r *Registry) ListAll() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ListByCategory returns tools filtered by category
func (r *Registry) ListByCategory(category string) []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]ToolDefinition, 0)
	for _, tool := range r.tools {
		if tool.Category == category {
			tools = append(tools, tool)
		}
	}
	return tools
}

// ListByTags returns tools that have all specified tags
func (r *Registry) ListByTags(tags []string) []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]ToolDefinition, 0)
	for _, tool := range r.tools {
		if hasAllTags(tool.Tags, tags) {
			tools = append(tools, tool)
		}
	}
	return tools
}

// ListWithFilter returns tools matching the filter criteria
func (r *Registry) ListWithFilter(category string, tags []string) []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]ToolDefinition, 0)
	for _, tool := range r.tools {
		// Check category if specified
		if category != "" && tool.Category != category {
			continue
		}

		// Check tags if specified
		if len(tags) > 0 && !hasAllTags(tool.Tags, tags) {
			continue
		}

		tools = append(tools, tool)
	}
	return tools
}

// hasAllTags checks if tool tags contain all required tags
func hasAllTags(toolTags []string, requiredTags []string) bool {
	tagMap := make(map[string]bool)
	for _, tag := range toolTags {
		tagMap[tag] = true
	}

	for _, tag := range requiredTags {
		if !tagMap[tag] {
			return false
		}
	}
	return true
}

// Execute executes a tool
func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (interface{}, error) {
	r.mu.RLock()
	tool, exists := r.tools[name]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	if tool.Handler == nil {
		return nil, fmt.Errorf("tool %s has no handler", name)
	}

	return tool.Handler(ctx, args)
}

// Count returns the number of registered tools
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// Size returns the size (same as Count for compatibility)
func (r *Registry) Size() int {
	return r.Count()
}
