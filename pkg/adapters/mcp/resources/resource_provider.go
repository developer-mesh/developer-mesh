package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// ResourceProvider provides MCP resources for read-only access
type ResourceProvider struct {
	logger    observability.Logger
	mu        sync.RWMutex
	resources map[string]Resource
	handlers  map[string]ResourceHandler
}

// Resource represents an MCP resource
type Resource struct {
	URI         string                 `json:"uri"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	MimeType    string                 `json:"mimeType"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ResourceHandler retrieves resource content
type ResourceHandler func(ctx context.Context, uri string) (interface{}, error)

// NewResourceProvider creates a new resource provider
func NewResourceProvider(logger observability.Logger) *ResourceProvider {
	provider := &ResourceProvider{
		logger:    logger,
		resources: make(map[string]Resource),
		handlers:  make(map[string]ResourceHandler),
	}

	// Register default resources
	provider.registerDefaultResources()

	return provider
}

// registerDefaultResources registers the default set of resources
func (p *ResourceProvider) registerDefaultResources() {
	// Workflow resources
	p.RegisterResource(Resource{
		URI:         "workflow/*",
		Name:        "Workflow",
		Description: "Access workflow information",
		MimeType:    "application/json",
	}, p.handleWorkflowResource)

	p.RegisterResource(Resource{
		URI:         "workflow/*/status",
		Name:        "Workflow Status",
		Description: "Get workflow execution status",
		MimeType:    "application/json",
	}, p.handleWorkflowStatusResource)

	p.RegisterResource(Resource{
		URI:         "workflow/*/execution/*",
		Name:        "Workflow Execution",
		Description: "Get specific workflow execution details",
		MimeType:    "application/json",
	}, p.handleWorkflowExecutionResource)

	// Task resources
	p.RegisterResource(Resource{
		URI:         "task/*",
		Name:        "Task",
		Description: "Access task information",
		MimeType:    "application/json",
	}, p.handleTaskResource)

	p.RegisterResource(Resource{
		URI:         "task/*/status",
		Name:        "Task Status",
		Description: "Get task status",
		MimeType:    "application/json",
	}, p.handleTaskStatusResource)

	p.RegisterResource(Resource{
		URI:         "task/*/results",
		Name:        "Task Results",
		Description: "Get task execution results",
		MimeType:    "application/json",
	}, p.handleTaskResultsResource)

	// Context resources
	p.RegisterResource(Resource{
		URI:         "context/*",
		Name:        "Context",
		Description: "Access session context",
		MimeType:    "text/plain",
	}, p.handleContextResource)

	p.RegisterResource(Resource{
		URI:         "context/*/metadata",
		Name:        "Context Metadata",
		Description: "Get context metadata",
		MimeType:    "application/json",
	}, p.handleContextMetadataResource)

	// Agent resources
	p.RegisterResource(Resource{
		URI:         "agent/*",
		Name:        "Agent",
		Description: "Access agent information",
		MimeType:    "application/json",
	}, p.handleAgentResource)

	p.RegisterResource(Resource{
		URI:         "agent/*/capabilities",
		Name:        "Agent Capabilities",
		Description: "Get agent capabilities",
		MimeType:    "application/json",
	}, p.handleAgentCapabilitiesResource)

	// System resources
	p.RegisterResource(Resource{
		URI:         "system/health",
		Name:        "System Health",
		Description: "Get system health status",
		MimeType:    "application/json",
	}, p.handleSystemHealthResource)

	p.RegisterResource(Resource{
		URI:         "system/metrics",
		Name:        "System Metrics",
		Description: "Get system metrics",
		MimeType:    "application/json",
	}, p.handleSystemMetricsResource)
}

// RegisterResource registers a new resource with its handler
func (p *ResourceProvider) RegisterResource(resource Resource, handler ResourceHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.resources[resource.URI] = resource
	p.handlers[resource.URI] = handler

	p.logger.Debug("Registered MCP resource", map[string]interface{}{
		"uri":      resource.URI,
		"name":     resource.Name,
		"mimeType": resource.MimeType,
	})
}

// ListResources returns all available resources
func (p *ResourceProvider) ListResources() []Resource {
	p.mu.RLock()
	defer p.mu.RUnlock()

	resources := make([]Resource, 0, len(p.resources))
	for _, resource := range p.resources {
		resources = append(resources, resource)
	}

	return resources
}

// ReadResource reads a resource by URI
func (p *ResourceProvider) ReadResource(ctx context.Context, uri string) (interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Try exact match first
	if handler, exists := p.handlers[uri]; exists {
		return handler(ctx, uri)
	}

	// Try pattern matching for wildcards
	for pattern, handler := range p.handlers {
		if matchesPattern(pattern, uri) {
			return handler(ctx, uri)
		}
	}

	return nil, fmt.Errorf("resource not found: %s", uri)
}

// matchesPattern checks if a URI matches a pattern with wildcards
func matchesPattern(pattern, uri string) bool {
	// Simple wildcard matching - replace * with regex equivalent
	patternParts := strings.Split(pattern, "/")
	uriParts := strings.Split(uri, "/")

	if len(patternParts) != len(uriParts) {
		return false
	}

	for i, part := range patternParts {
		if part != "*" && part != uriParts[i] {
			return false
		}
	}

	return true
}

// Resource handler implementations

func (p *ResourceProvider) handleWorkflowResource(ctx context.Context, uri string) (interface{}, error) {
	// Extract workflow ID from URI
	parts := strings.Split(uri, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid workflow URI: %s", uri)
	}

	workflowID := parts[1]

	// TODO: Integrate with actual workflow service
	return map[string]interface{}{
		"workflow_id": workflowID,
		"name":        fmt.Sprintf("Workflow %s", workflowID),
		"status":      "active",
		"steps":       []interface{}{},
		"created_at":  "2024-01-01T00:00:00Z",
	}, nil
}

func (p *ResourceProvider) handleWorkflowStatusResource(ctx context.Context, uri string) (interface{}, error) {
	parts := strings.Split(uri, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid workflow status URI: %s", uri)
	}

	workflowID := parts[1]

	return map[string]interface{}{
		"workflow_id":     workflowID,
		"status":          "running",
		"current_step":    2,
		"total_steps":     5,
		"completion_rate": 0.4,
	}, nil
}

func (p *ResourceProvider) handleWorkflowExecutionResource(ctx context.Context, uri string) (interface{}, error) {
	parts := strings.Split(uri, "/")
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid workflow execution URI: %s", uri)
	}

	workflowID := parts[1]
	executionID := parts[3]

	return map[string]interface{}{
		"workflow_id":  workflowID,
		"execution_id": executionID,
		"status":       "completed",
		"started_at":   "2024-01-01T00:00:00Z",
		"completed_at": "2024-01-01T00:15:00Z",
		"result":       map[string]interface{}{},
	}, nil
}

func (p *ResourceProvider) handleTaskResource(ctx context.Context, uri string) (interface{}, error) {
	parts := strings.Split(uri, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid task URI: %s", uri)
	}

	taskID := parts[1]

	return map[string]interface{}{
		"task_id":     taskID,
		"title":       fmt.Sprintf("Task %s", taskID),
		"description": "Task description",
		"status":      "in_progress",
		"priority":    "medium",
		"agent_id":    "agent-123",
		"created_at":  "2024-01-01T00:00:00Z",
	}, nil
}

func (p *ResourceProvider) handleTaskStatusResource(ctx context.Context, uri string) (interface{}, error) {
	parts := strings.Split(uri, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid task status URI: %s", uri)
	}

	taskID := parts[1]

	return map[string]interface{}{
		"task_id":  taskID,
		"status":   "in_progress",
		"progress": 0.65,
		"message":  "Processing data...",
	}, nil
}

func (p *ResourceProvider) handleTaskResultsResource(ctx context.Context, uri string) (interface{}, error) {
	parts := strings.Split(uri, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid task results URI: %s", uri)
	}

	taskID := parts[1]

	return map[string]interface{}{
		"task_id": taskID,
		"results": []interface{}{
			map[string]interface{}{
				"type": "text",
				"data": "Task completed successfully",
			},
		},
		"completed_at": "2024-01-01T00:30:00Z",
	}, nil
}

func (p *ResourceProvider) handleContextResource(ctx context.Context, uri string) (interface{}, error) {
	parts := strings.Split(uri, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid context URI: %s", uri)
	}

	sessionID := parts[1]

	// Return context as text
	return fmt.Sprintf("Context for session %s\n\nThis is the current context content.", sessionID), nil
}

func (p *ResourceProvider) handleContextMetadataResource(ctx context.Context, uri string) (interface{}, error) {
	parts := strings.Split(uri, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid context metadata URI: %s", uri)
	}

	sessionID := parts[1]

	return map[string]interface{}{
		"session_id":    sessionID,
		"token_count":   1500,
		"max_tokens":    4096,
		"last_modified": "2024-01-01T00:00:00Z",
	}, nil
}

func (p *ResourceProvider) handleAgentResource(ctx context.Context, uri string) (interface{}, error) {
	parts := strings.Split(uri, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid agent URI: %s", uri)
	}

	agentID := parts[1]

	return map[string]interface{}{
		"agent_id":      agentID,
		"agent_type":    "ide",
		"status":        "online",
		"version":       "1.0.0",
		"registered_at": "2024-01-01T00:00:00Z",
	}, nil
}

func (p *ResourceProvider) handleAgentCapabilitiesResource(ctx context.Context, uri string) (interface{}, error) {
	parts := strings.Split(uri, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid agent capabilities URI: %s", uri)
	}

	agentID := parts[1]

	return map[string]interface{}{
		"agent_id": agentID,
		"capabilities": []string{
			"code_completion",
			"code_analysis",
			"debugging",
			"testing",
		},
		"supported_languages": []string{
			"go",
			"python",
			"javascript",
			"typescript",
		},
	}, nil
}

func (p *ResourceProvider) handleSystemHealthResource(ctx context.Context, uri string) (interface{}, error) {
	return map[string]interface{}{
		"status":  "healthy",
		"uptime":  86400,
		"version": "1.0.0",
		"services": map[string]interface{}{
			"database": "healthy",
			"redis":    "healthy",
			"api":      "healthy",
		},
	}, nil
}

func (p *ResourceProvider) handleSystemMetricsResource(ctx context.Context, uri string) (interface{}, error) {
	return map[string]interface{}{
		"connections":      42,
		"active_workflows": 5,
		"active_tasks":     12,
		"messages_per_sec": 150,
		"cpu_usage":        0.35,
		"memory_usage":     0.62,
	}, nil
}

// ConvertToMCPResourceList converts resources to MCP resources/list response format
func (p *ResourceProvider) ConvertToMCPResourceList() map[string]interface{} {
	resources := p.ListResources()

	mcpResources := make([]map[string]interface{}, 0, len(resources))
	for _, resource := range resources {
		mcpResource := map[string]interface{}{
			"uri":         resource.URI,
			"name":        resource.Name,
			"description": resource.Description,
			"mimeType":    resource.MimeType,
		}
		if resource.Metadata != nil {
			mcpResource["metadata"] = resource.Metadata
		}
		mcpResources = append(mcpResources, mcpResource)
	}

	return map[string]interface{}{
		"resources": mcpResources,
	}
}

// ConvertToMCPResourceRead converts resource content to MCP resources/read response format
func (p *ResourceProvider) ConvertToMCPResourceRead(uri string, content interface{}) map[string]interface{} {
	// Determine content type based on content
	var contentItems []map[string]interface{}

	switch v := content.(type) {
	case string:
		// Text content
		contentItems = append(contentItems, map[string]interface{}{
			"type": "text",
			"text": v,
		})
	case map[string]interface{}, []interface{}:
		// JSON content - convert to formatted string
		jsonBytes, _ := json.MarshalIndent(v, "", "  ")
		contentItems = append(contentItems, map[string]interface{}{
			"type": "text",
			"text": string(jsonBytes),
		})
	default:
		// Default to string representation
		contentItems = append(contentItems, map[string]interface{}{
			"type": "text",
			"text": fmt.Sprintf("%v", v),
		})
	}

	return map[string]interface{}{
		"contents": contentItems,
	}
}
