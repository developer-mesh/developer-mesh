package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Tool represents a registered tool with its metadata
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	Provider    string                 `json:"provider"`
	Enabled     bool                   `json:"enabled"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ToolHandler defines the interface for tool execution
type ToolHandler interface {
	Execute(ctx context.Context, params map[string]interface{}) (interface{}, error)
	GetSchema() map[string]interface{}
	Validate(params map[string]interface{}) error
}

// Registry manages tool registration and execution
type Registry struct {
	tools    map[string]*Tool
	handlers map[string]ToolHandler
	logger   observability.Logger
	mu       sync.RWMutex
}

// NewRegistry creates a new tool registry
func NewRegistry(logger observability.Logger) *Registry {
	return &Registry{
		tools:    make(map[string]*Tool),
		handlers: make(map[string]ToolHandler),
		logger:   logger,
	}
}

// Register adds a new tool to the registry
func (r *Registry) Register(tool *Tool, handler ToolHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tool.Name == "" {
		return fmt.Errorf("tool name is required")
	}

	if _, exists := r.tools[tool.Name]; exists {
		return fmt.Errorf("tool %s already registered", tool.Name)
	}

	if handler == nil {
		return fmt.Errorf("tool handler is required")
	}

	// Set schema from handler if not provided
	if tool.Schema == nil {
		tool.Schema = handler.GetSchema()
	}

	r.tools[tool.Name] = tool
	r.handlers[tool.Name] = handler

	r.logger.Info("Tool registered", map[string]interface{}{
		"tool":     tool.Name,
		"provider": tool.Provider,
		"version":  tool.Version,
	})

	return nil
}

// Unregister removes a tool from the registry
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		return fmt.Errorf("tool %s not found", name)
	}

	delete(r.tools, name)
	delete(r.handlers, name)

	r.logger.Info("Tool unregistered", map[string]interface{}{
		"tool": name,
	})

	return nil
}

// Get returns a tool by name
func (r *Registry) Get(name string) (*Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}

	return tool, nil
}

// List returns all registered tools
func (r *Registry) List() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]*Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}

	return tools
}

// Execute runs a tool with the given parameters
func (r *Registry) Execute(ctx context.Context, name string, params map[string]interface{}) (interface{}, error) {
	r.mu.RLock()
	tool, exists := r.tools[name]
	handler, handlerExists := r.handlers[name]
	r.mu.RUnlock()

	if !exists || !handlerExists {
		return nil, fmt.Errorf("tool %s not found", name)
	}

	if !tool.Enabled {
		return nil, fmt.Errorf("tool %s is disabled", name)
	}

	// Validate parameters
	if err := handler.Validate(params); err != nil {
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	// Execute tool
	result, err := handler.Execute(ctx, params)
	if err != nil {
		r.logger.Error("Tool execution failed", map[string]interface{}{
			"tool":  name,
			"error": err.Error(),
		})
		return nil, err
	}

	r.logger.Info("Tool executed successfully", map[string]interface{}{
		"tool": name,
	})

	return result, nil
}

// Enable enables a tool
func (r *Registry) Enable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tool, exists := r.tools[name]
	if !exists {
		return fmt.Errorf("tool %s not found", name)
	}

	tool.Enabled = true
	return nil
}

// Disable disables a tool
func (r *Registry) Disable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tool, exists := r.tools[name]
	if !exists {
		return fmt.Errorf("tool %s not found", name)
	}

	tool.Enabled = false
	return nil
}

// RegisterBuiltinTools is deprecated - all tools are now managed via REST API
// This function is kept for backward compatibility but does nothing
func (r *Registry) RegisterBuiltinTools() error {
	// No builtin tools are registered anymore
	// All tools are dynamically loaded from the REST API
	r.logger.Info("Builtin tools registration skipped - using REST API for tool discovery", nil)
	return nil
}

// ProxyToolHandler proxies tool execution to the REST API
type ProxyToolHandler struct {
	name string
	// restClient interface{} // TODO: Add REST API client when proxy is implemented
	// logger     observability.Logger // TODO: Add logging when methods are implemented
}

func (h *ProxyToolHandler) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// This handler would proxy to REST API
	// Currently returns an error indicating tools should be accessed via REST API
	return nil, fmt.Errorf("tool %s should be executed via REST API proxy", h.name)
}

func (h *ProxyToolHandler) GetSchema() map[string]interface{} {
	// Schema is retrieved from REST API
	return map[string]interface{}{
		"type":        "object",
		"description": "Tool schema available via REST API",
	}
}

func (h *ProxyToolHandler) Validate(params map[string]interface{}) error {
	// Validation is handled by REST API
	return nil
}
