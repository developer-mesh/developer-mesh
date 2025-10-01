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

// TaskProvider provides task management tools
type TaskProvider struct {
	tasks   map[string]*Task
	tasksMu sync.RWMutex
}

// Task represents a task in the system
type Task struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	Type        string                 `json:"type,omitempty"`
	Priority    string                 `json:"priority,omitempty"`
	Status      string                 `json:"status"` // pending, assigned, in_progress, completed, failed
	AgentID     string                 `json:"agent_id,omitempty"`
	AgentType   string                 `json:"agent_type,omitempty"`
	Result      map[string]interface{} `json:"result,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	AssignedAt  *time.Time             `json:"assigned_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
}

// NewTaskProvider creates a new task provider
func NewTaskProvider() *TaskProvider {
	return &TaskProvider{
		tasks: make(map[string]*Task),
	}
}

// GetDefinitions returns the tool definitions for task management
func (p *TaskProvider) GetDefinitions() []tools.ToolDefinition {
	return []tools.ToolDefinition{
		{
			Name:        "task_list",
			Description: "List all tasks with optional filters",
			Category:    string(tools.CategoryTask),
			Tags:        []string{string(tools.CapabilityRead), string(tools.CapabilityList), string(tools.CapabilityFilter), string(tools.CapabilitySort), string(tools.CapabilityPaginate)},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by task status",
						"enum":        []string{"pending", "assigned", "in_progress", "completed", "failed"},
					},
					"priority": map[string]interface{}{
						"type":        "string",
						"description": "Filter by priority",
						"enum":        []string{"low", "medium", "high", "critical"},
					},
					"agent_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by assigned agent",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of tasks to return (default: 50, max: 100)",
						"minimum":     1,
						"maximum":     100,
					},
					"offset": map[string]interface{}{
						"type":        "integer",
						"description": "Number of tasks to skip for pagination (default: 0)",
						"minimum":     0,
					},
					"sort_by": map[string]interface{}{
						"type":        "string",
						"description": "Field to sort by",
						"enum":        []string{"id", "title", "priority", "status", "created_at"},
					},
					"sort_order": map[string]interface{}{
						"type":        "string",
						"description": "Sort order (default: desc)",
						"enum":        []string{"asc", "desc"},
					},
				},
			},
			Handler: p.handleList,
		},
		{
			Name:        "task_get",
			Description: "Get details of a specific task",
			Category:    string(tools.CategoryTask),
			Tags:        []string{string(tools.CapabilityRead)},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the task to retrieve",
					},
				},
				"required": []string{"task_id"},
			},
			Handler: p.handleGet,
		},
		{
			Name:        "task_get_batch",
			Description: "Get details of multiple tasks in a single operation",
			Category:    string(tools.CategoryTask),
			Tags:        []string{string(tools.CapabilityRead), string(tools.CapabilityBatch)},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_ids": map[string]interface{}{
						"type":        "array",
						"description": "IDs of tasks to retrieve (max 50)",
						"items": map[string]interface{}{
							"type": "string",
						},
						"maxItems": 50,
					},
				},
				"required": []string{"task_ids"},
			},
			Handler: p.handleGetBatch,
		},
		{
			Name:        "task_create",
			Description: "Create a new task",
			Category:    string(tools.CategoryTask),
			Tags:        []string{string(tools.CapabilityWrite), string(tools.CapabilityValidation), string(tools.CapabilityIdempotent)},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Title of the task",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Detailed description of the task",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Type of task",
						"enum":        []string{"code_review", "testing", "deployment", "analysis", "general"},
					},
					"priority": map[string]interface{}{
						"type":        "string",
						"description": "Priority level",
						"enum":        []string{"low", "medium", "high", "critical"},
					},
					"agent_type": map[string]interface{}{
						"type":        "string",
						"description": "Preferred agent type for this task",
					},
					"idempotency_key": map[string]interface{}{
						"type":        "string",
						"description": "Unique key for idempotent requests (prevents duplicate task creation)",
					},
				},
				"required": []string{"title"},
			},
			Handler: p.handleCreate,
		},
		{
			Name:        "task_assign",
			Description: "Assign a task to an agent",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the task to assign",
					},
					"agent_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the agent to assign to",
					},
				},
				"required": []string{"task_id", "agent_id"},
			},
			Handler: p.handleAssign,
		},
		{
			Name:        "task_complete",
			Description: "Mark a task as complete",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the task to complete",
					},
					"result": map[string]interface{}{
						"type":        "object",
						"description": "Result data from task completion",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Final status (completed or failed)",
						"enum":        []string{"completed", "failed"},
					},
				},
				"required": []string{"task_id"},
			},
			Handler: p.handleComplete,
		},
	}
}

// handleCreate creates a new task
func (p *TaskProvider) handleCreate(ctx context.Context, args json.RawMessage) (interface{}, error) {
	rb := NewResponseBuilder()

	// Check rate limit
	allowed, rateLimitStatus := GlobalRateLimiter.CheckAndConsume("task_create")
	if !allowed {
		return rb.ErrorWithMetadata(
			fmt.Errorf("rate limit exceeded, please retry after %v", time.Until(rateLimitStatus.Reset)),
			&ResponseMetadata{
				RateLimitStatus: rateLimitStatus,
			},
		), nil
	}

	var params struct {
		Title          string `json:"title"`
		Description    string `json:"description,omitempty"`
		Type           string `json:"type,omitempty"`
		Priority       string `json:"priority,omitempty"`
		AgentType      string `json:"agent_type,omitempty"`
		IdempotencyKey string `json:"idempotency_key,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return rb.Error(fmt.Errorf("invalid parameters: %w", err)), nil
	}

	if params.Title == "" {
		return rb.Error(fmt.Errorf("title is required")), nil
	}

	// Check idempotency
	if params.IdempotencyKey != "" {
		if cachedResponse, found := CheckIdempotency(params.IdempotencyKey); found {
			return cachedResponse, nil
		}
	}

	// Set defaults
	if params.Type == "" {
		params.Type = "general"
	}
	if params.Priority == "" {
		params.Priority = "medium"
	}

	task := &Task{
		ID:          uuid.New().String(),
		Title:       params.Title,
		Description: params.Description,
		Type:        params.Type,
		Priority:    params.Priority,
		AgentType:   params.AgentType,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}

	p.tasksMu.Lock()
	p.tasks[task.ID] = task
	p.tasksMu.Unlock()

	result := map[string]interface{}{
		"id":         task.ID,
		"title":      task.Title,
		"type":       task.Type,
		"priority":   task.Priority,
		"status":     task.Status,
		"created_at": task.CreatedAt.Format(time.RFC3339),
		"message":    fmt.Sprintf("Task created: %s", task.Title),
	}

	// Store for idempotency
	if params.IdempotencyKey != "" {
		response := rb.SuccessWithMetadata(
			result,
			&ResponseMetadata{
				IdempotencyKey:  params.IdempotencyKey,
				RateLimitStatus: rateLimitStatus,
			},
			"task_assign", "task_get", "task_list",
		)
		StoreIdempotentResponse(params.IdempotencyKey, response)
		return response, nil
	}

	return rb.SuccessWithMetadata(
		result,
		&ResponseMetadata{
			RateLimitStatus: rateLimitStatus,
		},
		"task_assign", "task_get", "task_list",
	), nil
}

// handleAssign assigns a task to an agent
func (p *TaskProvider) handleAssign(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		TaskID  string `json:"task_id"`
		AgentID string `json:"agent_id"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.TaskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}
	if params.AgentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}

	p.tasksMu.Lock()
	defer p.tasksMu.Unlock()

	task, exists := p.tasks[params.TaskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", params.TaskID)
	}

	if task.Status != "pending" && task.Status != "assigned" {
		return nil, fmt.Errorf("cannot assign task in status: %s", task.Status)
	}

	now := time.Now()
	task.AgentID = params.AgentID
	task.Status = "assigned"
	task.AssignedAt = &now

	return map[string]interface{}{
		"task_id":     task.ID,
		"agent_id":    task.AgentID,
		"status":      task.Status,
		"assigned_at": task.AssignedAt.Format(time.RFC3339),
		"message":     fmt.Sprintf("Task '%s' assigned to agent %s", task.Title, params.AgentID),
	}, nil
}

// handleComplete marks a task as complete
func (p *TaskProvider) handleComplete(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		TaskID string                 `json:"task_id"`
		Result map[string]interface{} `json:"result,omitempty"`
		Status string                 `json:"status,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.TaskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	// Default to completed if not specified
	if params.Status == "" {
		params.Status = "completed"
	}

	if params.Status != "completed" && params.Status != "failed" {
		return nil, fmt.Errorf("invalid status: %s (must be 'completed' or 'failed')", params.Status)
	}

	p.tasksMu.Lock()
	defer p.tasksMu.Unlock()

	task, exists := p.tasks[params.TaskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", params.TaskID)
	}

	if task.Status == "completed" || task.Status == "failed" {
		return nil, fmt.Errorf("task already finished with status: %s", task.Status)
	}

	now := time.Now()
	task.Status = params.Status
	task.Result = params.Result
	task.CompletedAt = &now

	return map[string]interface{}{
		"task_id":      task.ID,
		"title":        task.Title,
		"status":       task.Status,
		"completed_at": task.CompletedAt.Format(time.RFC3339),
		"message":      fmt.Sprintf("Task '%s' marked as %s", task.Title, params.Status),
	}, nil
}

// handleList returns a list of tasks with optional filters
func (p *TaskProvider) handleList(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Status    string `json:"status,omitempty"`
		Priority  string `json:"priority,omitempty"`
		AgentID   string `json:"agent_id,omitempty"`
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
		params.SortOrder = "desc" // Default to desc for tasks (newest first)
	}

	p.tasksMu.RLock()
	defer p.tasksMu.RUnlock()

	tasks := make([]map[string]interface{}, 0)
	for _, task := range p.tasks {
		// Apply filters
		if params.Status != "" && task.Status != params.Status {
			continue
		}
		if params.Priority != "" && task.Priority != params.Priority {
			continue
		}
		if params.AgentID != "" && task.AgentID != params.AgentID {
			continue
		}

		taskData := map[string]interface{}{
			"id":          task.ID,
			"title":       task.Title,
			"description": task.Description,
			"type":        task.Type,
			"priority":    task.Priority,
			"status":      task.Status,
			"created_at":  task.CreatedAt.Format(time.RFC3339),
		}
		if task.AgentID != "" {
			taskData["agent_id"] = task.AgentID
		}
		if task.AssignedAt != nil {
			taskData["assigned_at"] = task.AssignedAt.Format(time.RFC3339)
		}
		if task.CompletedAt != nil {
			taskData["completed_at"] = task.CompletedAt.Format(time.RFC3339)
		}
		tasks = append(tasks, taskData)
	}

	// Sort tasks if requested
	if params.SortBy != "" {
		sort.Slice(tasks, func(i, j int) bool {
			var less bool
			switch params.SortBy {
			case "id":
				less = tasks[i]["id"].(string) < tasks[j]["id"].(string)
			case "title":
				less = tasks[i]["title"].(string) < tasks[j]["title"].(string)
			case "priority":
				// Custom priority ordering: critical > high > medium > low
				priorityOrder := map[string]int{"critical": 4, "high": 3, "medium": 2, "low": 1}
				iPriority := priorityOrder[tasks[i]["priority"].(string)]
				jPriority := priorityOrder[tasks[j]["priority"].(string)]
				less = iPriority < jPriority
			case "status":
				less = tasks[i]["status"].(string) < tasks[j]["status"].(string)
			case "created_at":
				less = tasks[i]["created_at"].(string) < tasks[j]["created_at"].(string)
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
	totalCount := len(tasks)
	start := params.Offset
	end := params.Offset + params.Limit
	if start > totalCount {
		start = totalCount
	}
	if end > totalCount {
		end = totalCount
	}
	paginatedTasks := tasks[start:end]

	return map[string]interface{}{
		"tasks":       paginatedTasks,
		"count":       len(paginatedTasks),
		"total_count": totalCount,
		"offset":      params.Offset,
		"limit":       params.Limit,
	}, nil
}

// handleGet returns details of a specific task
func (p *TaskProvider) handleGet(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		TaskID string `json:"task_id"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.TaskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	p.tasksMu.RLock()
	task, exists := p.tasks[params.TaskID]
	p.tasksMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("task not found: %s", params.TaskID)
	}

	result := map[string]interface{}{
		"id":          task.ID,
		"title":       task.Title,
		"description": task.Description,
		"type":        task.Type,
		"priority":    task.Priority,
		"status":      task.Status,
		"agent_id":    task.AgentID,
		"agent_type":  task.AgentType,
		"result":      task.Result,
		"created_at":  task.CreatedAt.Format(time.RFC3339),
	}
	if task.AssignedAt != nil {
		result["assigned_at"] = task.AssignedAt.Format(time.RFC3339)
	}
	if task.CompletedAt != nil {
		result["completed_at"] = task.CompletedAt.Format(time.RFC3339)
		result["duration_seconds"] = int(task.CompletedAt.Sub(task.CreatedAt).Seconds())
	}

	return result, nil
}

// handleGetBatch returns details of multiple tasks
func (p *TaskProvider) handleGetBatch(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		TaskIDs []string `json:"task_ids"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if len(params.TaskIDs) == 0 {
		return nil, fmt.Errorf("task_ids is required and cannot be empty")
	}

	if len(params.TaskIDs) > 50 {
		return nil, fmt.Errorf("cannot retrieve more than 50 tasks at once")
	}

	p.tasksMu.RLock()
	defer p.tasksMu.RUnlock()

	tasks := make([]map[string]interface{}, 0, len(params.TaskIDs))
	notFound := []string{}

	for _, taskID := range params.TaskIDs {
		task, exists := p.tasks[taskID]
		if !exists {
			notFound = append(notFound, taskID)
			continue
		}

		taskData := map[string]interface{}{
			"id":          task.ID,
			"title":       task.Title,
			"description": task.Description,
			"type":        task.Type,
			"priority":    task.Priority,
			"status":      task.Status,
			"agent_id":    task.AgentID,
			"agent_type":  task.AgentType,
			"result":      task.Result,
			"created_at":  task.CreatedAt.Format(time.RFC3339),
		}
		if task.AssignedAt != nil {
			taskData["assigned_at"] = task.AssignedAt.Format(time.RFC3339)
		}
		if task.CompletedAt != nil {
			taskData["completed_at"] = task.CompletedAt.Format(time.RFC3339)
			taskData["duration_seconds"] = int(task.CompletedAt.Sub(task.CreatedAt).Seconds())
		}
		tasks = append(tasks, taskData)
	}

	result := map[string]interface{}{
		"tasks":           tasks,
		"found_count":     len(tasks),
		"total_requested": len(params.TaskIDs),
	}

	if len(notFound) > 0 {
		result["not_found"] = notFound
	}

	return result, nil
}
