package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/google/uuid"
)

// WorkflowProvider provides workflow management tools
type WorkflowProvider struct {
	workflows   map[string]*Workflow
	executions  map[string]*WorkflowExecution
	workflowsMu sync.RWMutex
}

// Workflow represents a workflow definition
type Workflow struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Steps       []WorkflowStep         `json:"steps"`
	Config      map[string]interface{} `json:"config,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

// WorkflowStep represents a step in a workflow
type WorkflowStep struct {
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	Config    map[string]interface{} `json:"config,omitempty"`
	DependsOn []string               `json:"depends_on,omitempty"`
}

// WorkflowExecution represents a workflow execution instance
type WorkflowExecution struct {
	ID         string                 `json:"id"`
	WorkflowID string                 `json:"workflow_id"`
	Status     string                 `json:"status"` // pending, running, completed, failed, cancelled
	Input      map[string]interface{} `json:"input,omitempty"`
	Output     map[string]interface{} `json:"output,omitempty"`
	StartedAt  time.Time              `json:"started_at"`
	EndedAt    *time.Time             `json:"ended_at,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

// NewWorkflowProvider creates a new workflow provider
func NewWorkflowProvider() *WorkflowProvider {
	return &WorkflowProvider{
		workflows:  make(map[string]*Workflow),
		executions: make(map[string]*WorkflowExecution),
	}
}

// GetDefinitions returns the tool definitions for workflow management
func (p *WorkflowProvider) GetDefinitions() []tools.ToolDefinition {
	return []tools.ToolDefinition{
		{
			Name:        "workflow_list",
			Description: "List all workflow definitions",
			Category:    string(tools.CategoryWorkflow),
			Tags:        []string{string(tools.CapabilityRead), string(tools.CapabilityList), string(tools.CapabilityFilter), string(tools.CapabilitySort), string(tools.CapabilityPaginate)},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of workflows to return (default: 50, max: 100)",
						"minimum":     1,
						"maximum":     100,
					},
					"offset": map[string]interface{}{
						"type":        "integer",
						"description": "Number of workflows to skip for pagination (default: 0)",
						"minimum":     0,
					},
					"sort_by": map[string]interface{}{
						"type":        "string",
						"description": "Field to sort by",
						"enum":        []string{"id", "name", "created_at"},
					},
					"sort_order": map[string]interface{}{
						"type":        "string",
						"description": "Sort order (default: asc)",
						"enum":        []string{"asc", "desc"},
					},
				},
			},
			Handler: p.handleList,
		},
		{
			Name:        "workflow_get",
			Description: "Get details of a specific workflow",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workflow_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the workflow to retrieve",
					},
				},
				"required": []string{"workflow_id"},
			},
			Handler: p.handleGet,
		},
		{
			Name:        "workflow_execution_list",
			Description: "List all workflow executions with optional filters",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workflow_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by workflow ID",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by execution status",
						"enum":        []string{"pending", "running", "completed", "failed", "cancelled"},
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of executions to return (default: 50, max: 100)",
						"minimum":     1,
						"maximum":     100,
					},
					"offset": map[string]interface{}{
						"type":        "integer",
						"description": "Number of executions to skip for pagination (default: 0)",
						"minimum":     0,
					},
					"sort_by": map[string]interface{}{
						"type":        "string",
						"description": "Field to sort by",
						"enum":        []string{"id", "workflow_id", "status", "started_at"},
					},
					"sort_order": map[string]interface{}{
						"type":        "string",
						"description": "Sort order (default: desc)",
						"enum":        []string{"asc", "desc"},
					},
				},
			},
			Handler: p.handleExecutionList,
		},
		{
			Name:        "workflow_execution_get",
			Description: "Get status of a specific workflow execution",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"execution_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the execution to retrieve",
					},
				},
				"required": []string{"execution_id"},
			},
			Handler: p.handleExecutionGet,
		},
		{
			Name:        "workflow_create",
			Description: "Create a new workflow definition",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the workflow",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Description of the workflow",
					},
					"steps": map[string]interface{}{
						"type":        "array",
						"description": "List of workflow steps",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"name": map[string]interface{}{
									"type": "string",
								},
								"type": map[string]interface{}{
									"type": "string",
								},
								"config": map[string]interface{}{
									"type": "object",
								},
								"depends_on": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type": "string",
									},
								},
							},
							"required": []string{"name", "type"},
						},
					},
					"config": map[string]interface{}{
						"type":        "object",
						"description": "Workflow configuration",
					},
					"idempotency_key": map[string]interface{}{
						"type":        "string",
						"description": "Unique key for idempotent requests (prevents duplicate workflow creation)",
					},
				},
				"required": []string{"name", "steps"},
			},
			Handler: p.handleCreate,
		},
		{
			Name:        "workflow_execute",
			Description: "Execute a workflow",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workflow_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the workflow to execute",
					},
					"input": map[string]interface{}{
						"type":        "object",
						"description": "Input parameters for the workflow",
					},
					"idempotency_key": map[string]interface{}{
						"type":        "string",
						"description": "Unique key for idempotent requests (prevents duplicate executions)",
					},
				},
				"required": []string{"workflow_id"},
			},
			Handler: p.handleExecute,
		},
		{
			Name:        "workflow_cancel",
			Description: "Cancel a running workflow execution",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"execution_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the execution to cancel",
					},
				},
				"required": []string{"execution_id"},
			},
			Handler: p.handleCancel,
		},
	}
}

// handleCreate creates a new workflow
func (p *WorkflowProvider) handleCreate(ctx context.Context, args json.RawMessage) (interface{}, error) {
	rb := NewResponseBuilder()

	// Check rate limit
	allowed, rateLimitStatus := GlobalRateLimiter.CheckAndConsume("workflow_create")
	if !allowed {
		return rb.ErrorWithMetadata(
			fmt.Errorf("rate limit exceeded, please retry after %v", time.Until(rateLimitStatus.Reset)),
			&ResponseMetadata{
				RateLimitStatus: rateLimitStatus,
			},
		), nil
	}

	var params struct {
		Name           string                 `json:"name"`
		Description    string                 `json:"description,omitempty"`
		Steps          []WorkflowStep         `json:"steps"`
		Config         map[string]interface{} `json:"config,omitempty"`
		IdempotencyKey string                 `json:"idempotency_key,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return rb.Error(fmt.Errorf("invalid parameters: %w", err)), nil
	}

	if params.Name == "" {
		return rb.Error(fmt.Errorf("name is required")), nil
	}
	if len(params.Steps) == 0 {
		return rb.Error(fmt.Errorf("at least one step is required")), nil
	}

	// Check idempotency
	if params.IdempotencyKey != "" {
		if cachedResponse, found := CheckIdempotency(params.IdempotencyKey); found {
			return cachedResponse, nil
		}
	}

	workflow := &Workflow{
		ID:          uuid.New().String(),
		Name:        params.Name,
		Description: params.Description,
		Steps:       params.Steps,
		Config:      params.Config,
		CreatedAt:   time.Now(),
	}

	p.workflowsMu.Lock()
	p.workflows[workflow.ID] = workflow
	p.workflowsMu.Unlock()

	result := map[string]interface{}{
		"id":          workflow.ID,
		"name":        workflow.Name,
		"description": workflow.Description,
		"steps_count": len(workflow.Steps),
		"created_at":  workflow.CreatedAt.Format(time.RFC3339),
		"message":     fmt.Sprintf("Workflow '%s' created successfully", workflow.Name),
	}

	// Store for idempotency
	if params.IdempotencyKey != "" {
		response := rb.SuccessWithMetadata(
			result,
			&ResponseMetadata{
				IdempotencyKey:  params.IdempotencyKey,
				RateLimitStatus: rateLimitStatus,
			},
			"workflow_execute", "workflow_get",
		)
		StoreIdempotentResponse(params.IdempotencyKey, response)
		return response, nil
	}

	return rb.SuccessWithMetadata(
		result,
		&ResponseMetadata{
			RateLimitStatus: rateLimitStatus,
		},
		"workflow_execute", "workflow_get",
	), nil
}

// handleExecute executes a workflow
func (p *WorkflowProvider) handleExecute(ctx context.Context, args json.RawMessage) (interface{}, error) {
	rb := NewResponseBuilder()

	// Check rate limit
	allowed, rateLimitStatus := GlobalRateLimiter.CheckAndConsume("workflow_execute")
	if !allowed {
		return rb.ErrorWithMetadata(
			fmt.Errorf("rate limit exceeded, please retry after %v", time.Until(rateLimitStatus.Reset)),
			&ResponseMetadata{
				RateLimitStatus: rateLimitStatus,
			},
		), nil
	}

	var params struct {
		WorkflowID     string                 `json:"workflow_id"`
		Input          map[string]interface{} `json:"input,omitempty"`
		IdempotencyKey string                 `json:"idempotency_key,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.WorkflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}

	p.workflowsMu.RLock()
	workflow, exists := p.workflows[params.WorkflowID]
	p.workflowsMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("workflow not found: %s", params.WorkflowID)
	}

	execution := &WorkflowExecution{
		ID:         uuid.New().String(),
		WorkflowID: params.WorkflowID,
		Status:     "running",
		Input:      params.Input,
		StartedAt:  time.Now(),
	}

	p.workflowsMu.Lock()
	p.executions[execution.ID] = execution
	p.workflowsMu.Unlock()

	// In a real implementation, this would execute the workflow asynchronously
	// For now, we'll simulate immediate completion
	go func() {
		time.Sleep(100 * time.Millisecond) // Simulate processing

		p.workflowsMu.Lock()
		now := time.Now()
		execution.Status = "completed"
		execution.EndedAt = &now
		execution.Output = map[string]interface{}{
			"result":         "success",
			"steps_executed": len(workflow.Steps),
		}
		p.workflowsMu.Unlock()
	}()

	return map[string]interface{}{
		"execution_id": execution.ID,
		"workflow_id":  execution.WorkflowID,
		"status":       execution.Status,
		"started_at":   execution.StartedAt.Format(time.RFC3339),
		"message":      fmt.Sprintf("Workflow execution started: %s", execution.ID),
	}, nil
}

// handleCancel cancels a workflow execution
func (p *WorkflowProvider) handleCancel(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		ExecutionID string `json:"execution_id"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.ExecutionID == "" {
		return nil, fmt.Errorf("execution_id is required")
	}

	p.workflowsMu.Lock()
	defer p.workflowsMu.Unlock()

	execution, exists := p.executions[params.ExecutionID]
	if !exists {
		return nil, fmt.Errorf("execution not found: %s", params.ExecutionID)
	}

	if execution.Status != "running" && execution.Status != "pending" {
		return nil, fmt.Errorf("cannot cancel execution in status: %s", execution.Status)
	}

	now := time.Now()
	execution.Status = "cancelled"
	execution.EndedAt = &now

	return map[string]interface{}{
		"execution_id": execution.ID,
		"status":       execution.Status,
		"message":      fmt.Sprintf("Workflow execution cancelled: %s", execution.ID),
	}, nil
}

// handleList returns a list of all workflows
func (p *WorkflowProvider) handleList(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Limit     int    `json:"limit,omitempty"`
		Offset    int    `json:"offset,omitempty"`
		SortBy    string `json:"sort_by,omitempty"`
		SortOrder string `json:"sort_order,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Set defaults
	if params.Limit <= 0 {
		params.Limit = 50
	} else if params.Limit > 100 {
		params.Limit = 100
	}
	if params.Offset < 0 {
		params.Offset = 0
	}
	if params.SortOrder == "" {
		params.SortOrder = "asc"
	}

	p.workflowsMu.RLock()
	defer p.workflowsMu.RUnlock()

	workflows := make([]map[string]interface{}, 0, len(p.workflows))
	for _, workflow := range p.workflows {
		workflows = append(workflows, map[string]interface{}{
			"id":          workflow.ID,
			"name":        workflow.Name,
			"description": workflow.Description,
			"steps_count": len(workflow.Steps),
			"created_at":  workflow.CreatedAt.Format(time.RFC3339),
		})
	}

	// Sort workflows if requested
	if params.SortBy != "" {
		sort.Slice(workflows, func(i, j int) bool {
			var less bool
			switch params.SortBy {
			case "id":
				less = workflows[i]["id"].(string) < workflows[j]["id"].(string)
			case "name":
				less = workflows[i]["name"].(string) < workflows[j]["name"].(string)
			case "created_at":
				less = workflows[i]["created_at"].(string) < workflows[j]["created_at"].(string)
			default:
				return false
			}
			if params.SortOrder == "desc" {
				return !less
			}
			return less
		})
	}

	// Apply pagination
	totalCount := len(workflows)
	start := params.Offset
	end := params.Offset + params.Limit
	if start > totalCount {
		start = totalCount
	}
	if end > totalCount {
		end = totalCount
	}
	paginatedWorkflows := workflows[start:end]

	return map[string]interface{}{
		"workflows":   paginatedWorkflows,
		"count":       len(paginatedWorkflows),
		"total_count": totalCount,
		"offset":      params.Offset,
		"limit":       params.Limit,
	}, nil
}

// handleGet returns details of a specific workflow
func (p *WorkflowProvider) handleGet(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		WorkflowID string `json:"workflow_id"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.WorkflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}

	p.workflowsMu.RLock()
	workflow, exists := p.workflows[params.WorkflowID]
	p.workflowsMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("workflow not found: %s", params.WorkflowID)
	}

	return map[string]interface{}{
		"id":          workflow.ID,
		"name":        workflow.Name,
		"description": workflow.Description,
		"steps":       workflow.Steps,
		"config":      workflow.Config,
		"created_at":  workflow.CreatedAt.Format(time.RFC3339),
	}, nil
}

// handleExecutionList returns a list of workflow executions
func (p *WorkflowProvider) handleExecutionList(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		WorkflowID string `json:"workflow_id,omitempty"`
		Status     string `json:"status,omitempty"`
		Limit      int    `json:"limit,omitempty"`
		Offset     int    `json:"offset,omitempty"`
		SortBy     string `json:"sort_by,omitempty"`
		SortOrder  string `json:"sort_order,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Set defaults
	if params.Limit <= 0 {
		params.Limit = 50
	} else if params.Limit > 100 {
		params.Limit = 100
	}
	if params.Offset < 0 {
		params.Offset = 0
	}
	if params.SortOrder == "" {
		params.SortOrder = "desc" // Default to desc for executions (newest first)
	}

	p.workflowsMu.RLock()
	defer p.workflowsMu.RUnlock()

	executions := make([]map[string]interface{}, 0)
	for _, execution := range p.executions {
		// Apply filters
		if params.WorkflowID != "" && execution.WorkflowID != params.WorkflowID {
			continue
		}
		if params.Status != "" && execution.Status != params.Status {
			continue
		}

		exec := map[string]interface{}{
			"id":          execution.ID,
			"workflow_id": execution.WorkflowID,
			"status":      execution.Status,
			"started_at":  execution.StartedAt.Format(time.RFC3339),
		}

		// Add workflow name if available
		if workflow, exists := p.workflows[execution.WorkflowID]; exists {
			exec["workflow_name"] = workflow.Name
		}

		if execution.EndedAt != nil {
			exec["ended_at"] = execution.EndedAt.Format(time.RFC3339)
			exec["duration_seconds"] = int(execution.EndedAt.Sub(execution.StartedAt).Seconds())
		}
		if execution.Error != "" {
			exec["error"] = execution.Error
		}
		executions = append(executions, exec)
	}

	// Sort executions if requested
	if params.SortBy != "" {
		sort.Slice(executions, func(i, j int) bool {
			var less bool
			switch params.SortBy {
			case "id":
				less = executions[i]["id"].(string) < executions[j]["id"].(string)
			case "workflow_id":
				less = executions[i]["workflow_id"].(string) < executions[j]["workflow_id"].(string)
			case "status":
				less = executions[i]["status"].(string) < executions[j]["status"].(string)
			case "started_at":
				less = executions[i]["started_at"].(string) < executions[j]["started_at"].(string)
			default:
				return false
			}
			if params.SortOrder == "desc" {
				return !less
			}
			return less
		})
	}

	// Apply pagination
	totalCount := len(executions)
	start := params.Offset
	end := params.Offset + params.Limit
	if start > totalCount {
		start = totalCount
	}
	if end > totalCount {
		end = totalCount
	}
	paginatedExecutions := executions[start:end]

	return map[string]interface{}{
		"executions":  paginatedExecutions,
		"count":       len(paginatedExecutions),
		"total_count": totalCount,
		"offset":      params.Offset,
		"limit":       params.Limit,
	}, nil
}

// handleExecutionGet returns details of a specific execution
func (p *WorkflowProvider) handleExecutionGet(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		ExecutionID string `json:"execution_id"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.ExecutionID == "" {
		return nil, fmt.Errorf("execution_id is required")
	}

	p.workflowsMu.RLock()
	execution, exists := p.executions[params.ExecutionID]
	p.workflowsMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("execution not found: %s", params.ExecutionID)
	}

	result := map[string]interface{}{
		"id":          execution.ID,
		"workflow_id": execution.WorkflowID,
		"status":      execution.Status,
		"input":       execution.Input,
		"output":      execution.Output,
		"started_at":  execution.StartedAt.Format(time.RFC3339),
	}
	if execution.EndedAt != nil {
		result["ended_at"] = execution.EndedAt.Format(time.RFC3339)
		result["duration_seconds"] = int(execution.EndedAt.Sub(execution.StartedAt).Seconds())
	}
	if execution.Error != "" {
		result["error"] = execution.Error
	}

	return result, nil
}
