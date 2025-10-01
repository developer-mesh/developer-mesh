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

// Registry manages tools
type Registry struct {
	tools               map[string]ToolDefinition
	relationshipManager *RelationshipManager
	mu                  sync.RWMutex
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools:               make(map[string]ToolDefinition),
		relationshipManager: NewRelationshipManager(),
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
		// Return a special error type that the handler can recognize and enhance
		return nil, &ToolNotFoundError{ToolName: name}
	}

	if tool.Handler == nil {
		return nil, &ToolConfigError{ToolName: name, Reason: "no handler configured"}
	}

	return tool.Handler(ctx, args)
}

// ToolNotFoundError indicates a tool was not found
type ToolNotFoundError struct {
	ToolName string
}

func (e *ToolNotFoundError) Error() string {
	return fmt.Sprintf("tool not found: %s", e.ToolName)
}

// ToolConfigError indicates a tool configuration error
type ToolConfigError struct {
	ToolName string
	Reason   string
}

func (e *ToolConfigError) Error() string {
	return fmt.Sprintf("tool %s configuration error: %s", e.ToolName, e.Reason)
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) (ToolDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, exists := r.tools[name]
	return tool, exists
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

// GetRelationships returns the relationship manager
func (r *Registry) GetRelationships() *RelationshipManager {
	return r.relationshipManager
}

// ValidateToolDependencies checks if all prerequisites for a tool are available
func (r *Registry) ValidateToolDependencies(toolName string) (bool, []string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	availableTools := make([]string, 0, len(r.tools))
	for name := range r.tools {
		availableTools = append(availableTools, name)
	}

	return r.relationshipManager.ValidateDependencies(toolName, availableTools)
}

// CheckToolCompatibility checks if two tools are compatible based on I/O types
func (r *Registry) CheckToolCompatibility(sourceTool, targetTool string) (bool, string) {
	return r.relationshipManager.CheckCompatibility(sourceTool, targetTool)
}

// SuggestNextTools suggests the next tools to use based on the current tool
func (r *Registry) SuggestNextTools(currentTool string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	suggestions := r.relationshipManager.SuggestNextTools(currentTool)

	// Filter suggestions to only include available tools
	available := make([]string, 0, len(suggestions))
	for _, tool := range suggestions {
		if _, exists := r.tools[tool]; exists {
			available = append(available, tool)
		}
	}

	return available
}

// GetAlternativeTools returns alternative tools that can be used instead
func (r *Registry) GetAlternativeTools(toolName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	alternatives := r.relationshipManager.GetAlternatives(toolName)

	// Filter alternatives to only include available tools
	available := make([]string, 0, len(alternatives))
	for _, tool := range alternatives {
		if _, exists := r.tools[tool]; exists {
			available = append(available, tool)
		}
	}

	return available
}

// CheckToolConflicts checks if two tools conflict with each other
func (r *Registry) CheckToolConflicts(tool1, tool2 string) bool {
	return r.relationshipManager.CheckConflicts(tool1, tool2)
}

// GetWorkflowsForCategory returns workflows for a specific category
func (r *Registry) GetWorkflowsForCategory(category string) []WorkflowTemplate {
	return r.relationshipManager.GetWorkflowsForCategory(category)
}

// GetWorkflowsWithTool returns workflows that include a specific tool
func (r *Registry) GetWorkflowsWithTool(toolName string) []WorkflowTemplate {
	return r.relationshipManager.GetWorkflowsWithTool(toolName)
}

// ValidateWorkflow validates that all tools in a workflow are available
func (r *Registry) ValidateWorkflow(workflow WorkflowTemplate) (bool, []string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	availableTools := make([]string, 0, len(r.tools))
	for name := range r.tools {
		availableTools = append(availableTools, name)
	}

	return r.relationshipManager.ValidateWorkflow(workflow, availableTools)
}

// GetAllWorkflows returns all workflow templates
func (r *Registry) GetAllWorkflows() []WorkflowTemplate {
	return r.relationshipManager.GetAllWorkflows()
}

// EnrichToolWithRelationships adds relationship information to a tool definition
func (r *Registry) EnrichToolWithRelationships(tool *ToolDefinition) {
	if rel, exists := r.relationshipManager.GetRelationship(tool.Name); exists {
		tool.Prerequisites = rel.Prerequisites
		tool.CommonlyUsedWith = rel.CommonlyUsedWith
		tool.NextSteps = rel.NextSteps
		tool.Alternatives = rel.Alternatives
		tool.ConflictsWith = rel.ConflictsWith
	}

	if io, exists := r.relationshipManager.GetIOCompatibility(tool.Name); exists {
		tool.IOCompatibility = io
	}
}

// ListAllEnriched returns all tools with relationship information
func (r *Registry) ListAllEnriched() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		enrichedTool := tool
		r.EnrichToolWithRelationships(&enrichedTool)
		tools = append(tools, enrichedTool)
	}
	return tools
}
