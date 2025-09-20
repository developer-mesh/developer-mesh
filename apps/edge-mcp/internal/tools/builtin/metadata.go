package builtin

import (
	"sync"
	"time"
)

// ToolMetadata provides enhanced metadata for tools following Anthropic patterns
type ToolMetadata struct {
	// Tool chaining hints - which tools commonly follow this one
	NextTools []string `json:"next_tools,omitempty"`

	// Rate limiting information
	RateLimit *RateLimitInfo `json:"rate_limit,omitempty"`

	// Capability boundaries
	Limits map[string]interface{} `json:"limits,omitempty"`

	// Examples of usage
	Examples []Example `json:"examples,omitempty"`

	// Advanced parameter groups
	AdvancedParams []string `json:"advanced_params,omitempty"`
}

// RateLimitInfo describes rate limiting for a tool
type RateLimitInfo struct {
	RequestsPerMinute int    `json:"requests_per_minute"`
	BurstSize         int    `json:"burst_size"`
	Description       string `json:"description"`
}

// Example shows a sample usage of a tool
type Example struct {
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output,omitempty"`
}

// StandardResponse wraps tool responses with metadata
type StandardResponse struct {
	Success   bool              `json:"success"`
	Data      interface{}       `json:"data,omitempty"`
	Error     string            `json:"error,omitempty"`
	Metadata  *ResponseMetadata `json:"metadata"`
	NextSteps []string          `json:"next_steps,omitempty"`
}

// ResponseMetadata provides operation metadata
type ResponseMetadata struct {
	RequestID       string           `json:"request_id"`
	Timestamp       time.Time        `json:"timestamp"`
	Duration        string           `json:"duration,omitempty"`
	IdempotencyKey  string           `json:"idempotency_key,omitempty"`
	RateLimitStatus *RateLimitStatus `json:"rate_limit_status,omitempty"`
}

// RateLimitStatus shows current rate limit state
type RateLimitStatus struct {
	Remaining int       `json:"remaining"`
	Reset     time.Time `json:"reset"`
	Limit     int       `json:"limit"`
}

// IdempotencyStore tracks idempotent requests
type IdempotencyStore struct {
	mu      sync.RWMutex
	entries map[string]*IdempotencyEntry
}

// IdempotencyEntry stores a previous request result
type IdempotencyEntry struct {
	Key       string
	Response  interface{}
	CreatedAt time.Time
	ExpiresAt time.Time
}

// NewIdempotencyStore creates a new idempotency store
func NewIdempotencyStore() *IdempotencyStore {
	store := &IdempotencyStore{
		entries: make(map[string]*IdempotencyEntry),
	}

	// Start cleanup goroutine
	go store.cleanupExpired()

	return store
}

// Get retrieves a cached idempotent response
func (s *IdempotencyStore) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.entries[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	return entry.Response, true
}

// Set stores an idempotent response
func (s *IdempotencyStore) Set(key string, response interface{}, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.entries[key] = &IdempotencyEntry{
		Key:       key,
		Response:  response,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
}

// cleanupExpired removes expired entries periodically
func (s *IdempotencyStore) cleanupExpired() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for key, entry := range s.entries {
			if now.After(entry.ExpiresAt) {
				delete(s.entries, key)
			}
		}
		s.mu.Unlock()
	}
}

// WorkflowTemplate defines a pre-configured workflow pattern
type WorkflowTemplate struct {
	ID                string                 `json:"id"`
	Name              string                 `json:"name"`
	Description       string                 `json:"description"`
	Category          string                 `json:"category"`
	Steps             []WorkflowTemplateStep `json:"steps"`
	RequiredVariables []TemplateVariable     `json:"required_variables,omitempty"`
	OptionalVariables []TemplateVariable     `json:"optional_variables,omitempty"`
}

// TemplateVariable defines a variable for a workflow template
type TemplateVariable struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"` // string, number, boolean, array, object
	Description string      `json:"description"`
	Default     interface{} `json:"default,omitempty"`
	Example     interface{} `json:"example,omitempty"`
}

// WorkflowTemplateStep defines a step in a workflow template
type WorkflowTemplateStep struct {
	Tool        string                 `json:"tool"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	DependsOn   []string               `json:"depends_on,omitempty"`
	OutputVar   string                 `json:"output_var,omitempty"`
}

// Common workflow templates
var WorkflowTemplates = []WorkflowTemplate{
	{
		ID:          "deploy-with-review",
		Name:        "Deploy with Code Review",
		Description: "Create task, assign for review, then deploy",
		Category:    "deployment",
		RequiredVariables: []TemplateVariable{
			{
				Name:        "deployment_title",
				Type:        "string",
				Description: "Title for the deployment task",
				Example:     "Deploy API v2.0 to production",
			},
			{
				Name:        "reviewer_id",
				Type:        "string",
				Description: "Agent ID of the code reviewer",
				Example:     "agent-senior-dev-1",
			},
			{
				Name:        "deployment_workflow_id",
				Type:        "string",
				Description: "ID of the deployment workflow to execute",
				Example:     "workflow-deploy-prod",
			},
		},
		Steps: []WorkflowTemplateStep{
			{
				Tool:        "task_create",
				Description: "Create deployment task",
				Input: map[string]interface{}{
					"title":    "${deployment_title}",
					"type":     "deployment",
					"priority": "high",
				},
				OutputVar: "task_id",
			},
			{
				Tool:        "task_assign",
				Description: "Assign to reviewer",
				Input: map[string]interface{}{
					"task_id":  "${task_id}",
					"agent_id": "${reviewer_id}",
				},
				DependsOn: []string{"task_create"},
			},
			{
				Tool:        "workflow_execute",
				Description: "Execute deployment workflow",
				Input: map[string]interface{}{
					"workflow_id": "${deployment_workflow_id}",
				},
				DependsOn: []string{"task_assign"},
			},
		},
	},
	{
		ID:          "batch-task-processing",
		Name:        "Batch Task Processing",
		Description: "Create multiple tasks and assign to available agents",
		Category:    "task-management",
		RequiredVariables: []TemplateVariable{
			{
				Name:        "task_title",
				Type:        "string",
				Description: "Title for the tasks to create",
				Example:     "Process data batch",
			},
		},
		OptionalVariables: []TemplateVariable{
			{
				Name:        "task_priority",
				Type:        "string",
				Description: "Priority level for tasks",
				Default:     "medium",
				Example:     "high",
			},
		},
		Steps: []WorkflowTemplateStep{
			{
				Tool:        "agent_list",
				Description: "Get available agents",
				Input: map[string]interface{}{
					"status": "online",
					"limit":  10,
				},
				OutputVar: "available_agents",
			},
			{
				Tool:        "task_create",
				Description: "Create tasks from batch",
				Input: map[string]interface{}{
					"title":    "${task_title}",
					"type":     "processing",
					"priority": "${task_priority}",
				},
				OutputVar: "task_ids",
			},
		},
	},
}
