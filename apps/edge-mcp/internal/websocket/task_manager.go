package websocket

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
)

// TaskManager manages background tasks
type TaskManager struct {
	tasks   sync.Map // task ID -> Task
	logger  observability.Logger
	metrics observability.MetricsClient
}

// NewTaskManager creates a new task manager
func NewTaskManager(logger observability.Logger, metrics observability.MetricsClient) *TaskManager {
	return &TaskManager{
		logger:  logger,
		metrics: metrics,
	}
}

// TaskDefinition defines a task
type TaskDefinition struct {
	Type       string                 `json:"type"`
	Parameters map[string]interface{} `json:"parameters"`
	Priority   string                 `json:"priority"` // high, normal, low
	MaxRetries int                    `json:"max_retries"`
	Timeout    time.Duration          `json:"timeout"`
	AgentID    string                 `json:"agent_id"`
	TenantID   string                 `json:"tenant_id"`
}

// Task represents a background task
type Task struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Parameters  map[string]interface{} `json:"parameters"`
	Status      string                 `json:"status"` // pending, running, completed, failed, cancelled
	Priority    string                 `json:"priority"`
	Progress    float64                `json:"progress"` // 0-100
	Result      interface{}            `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	AgentID     string                 `json:"agent_id"`
	TenantID    string                 `json:"tenant_id"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   time.Time              `json:"started_at,omitempty"`
	CompletedAt time.Time              `json:"completed_at,omitempty"`
	UpdatedAt   time.Time              `json:"updated_at,omitempty"`
	Attempts    int                    `json:"attempts"`
	MaxRetries  int                    `json:"max_retries"`
	Timeout     time.Duration          `json:"timeout"`
	AssignedTo  string                 `json:"assigned_to,omitempty"`
	Duration    time.Duration          `json:"duration,omitempty"`
}

// CreateTask creates a new background task
func (tm *TaskManager) CreateTask(ctx context.Context, def *TaskDefinition) (*Task, error) {
	task := &Task{
		ID:         uuid.New().String(),
		Type:       def.Type,
		Parameters: def.Parameters,
		Status:     "pending",
		Priority:   def.Priority,
		Progress:   0,
		AgentID:    def.AgentID,
		TenantID:   def.TenantID,
		CreatedAt:  time.Now(),
		Attempts:   0,
		MaxRetries: def.MaxRetries,
		Timeout:    def.Timeout,
	}

	if task.Priority == "" {
		task.Priority = "normal"
	}

	// Store task
	tm.tasks.Store(task.ID, task)

	// Start async execution
	go tm.executeTask(ctx, task)

	tm.metrics.IncrementCounter("tasks_created", 1)
	tm.logger.Info("Task created", map[string]interface{}{
		"task_id":  task.ID,
		"type":     task.Type,
		"priority": task.Priority,
	})

	return task, nil
}

// GetTask retrieves a task
func (tm *TaskManager) GetTask(ctx context.Context, taskID string) (*Task, error) {
	val, ok := tm.tasks.Load(taskID)
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	return val.(*Task), nil
}

// CancelTask cancels a task
func (tm *TaskManager) CancelTask(ctx context.Context, taskID, reason string) error {
	val, ok := tm.tasks.Load(taskID)
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task := val.(*Task)
	if task.Status != "pending" && task.Status != "running" {
		return fmt.Errorf("cannot cancel task in status: %s", task.Status)
	}

	task.Status = "cancelled"
	task.Error = reason
	task.CompletedAt = time.Now()

	tm.metrics.IncrementCounter("tasks_cancelled", 1)
	return nil
}

// ListTasks lists tasks for an agent
func (tm *TaskManager) ListTasks(ctx context.Context, agentID, status, taskType string, limit, offset int) ([]*Task, int, error) {
	var tasks []*Task
	total := 0

	tm.tasks.Range(func(key, value interface{}) bool {
		task := value.(*Task)

		// Filter by agent
		if task.AgentID != agentID {
			return true
		}

		// Filter by status
		if status != "" && task.Status != status {
			return true
		}

		// Filter by type
		if taskType != "" && task.Type != taskType {
			return true
		}

		total++

		// Skip if before offset
		if total <= offset {
			return true
		}

		// Stop if limit reached
		if limit > 0 && len(tasks) >= limit {
			return false
		}

		tasks = append(tasks, task)
		return true
	})

	return tasks, total, nil
}

// executeTask runs a task asynchronously
func (tm *TaskManager) executeTask(ctx context.Context, task *Task) {
	// Wait based on priority
	switch task.Priority {
	case "low":
		time.Sleep(2 * time.Second)
	case "normal":
		time.Sleep(500 * time.Millisecond)
	case "high":
		// Execute immediately
	}

	task.Status = "running"
	task.StartedAt = time.Now()
	task.Attempts++

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, task.Timeout)
	defer cancel()

	// Execute task based on type (simplified - in production would call actual handlers)
	err := tm.runTaskLogic(timeoutCtx, task)

	if err != nil {
		task.Error = err.Error()

		// Check if should retry
		if task.Attempts < task.MaxRetries {
			task.Status = "pending"
			tm.logger.Info("Task failed, retrying", map[string]interface{}{
				"task_id": task.ID,
				"attempt": task.Attempts,
				"error":   err.Error(),
			})

			// Exponential backoff
			time.Sleep(time.Duration(task.Attempts) * time.Second)
			go tm.executeTask(ctx, task)
			return
		}

		task.Status = "failed"
		tm.metrics.IncrementCounter("tasks_failed", 1)
	} else {
		task.Status = "completed"
		task.Progress = 100
		tm.metrics.IncrementCounter("tasks_completed", 1)
	}

	task.CompletedAt = time.Now()

	tm.logger.Info("Task completed", map[string]interface{}{
		"task_id":  task.ID,
		"status":   task.Status,
		"duration": task.CompletedAt.Sub(task.StartedAt).Seconds(),
	})
}

// runTaskLogic executes the actual task logic
func (tm *TaskManager) runTaskLogic(ctx context.Context, task *Task) error {
	// Simulate task execution with progress updates
	steps := 10
	for i := 0; i < steps; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("task timeout")
		default:
			// Update progress
			task.Progress = float64(i+1) / float64(steps) * 100
			time.Sleep(200 * time.Millisecond)

			// Check if cancelled
			if task.Status == "cancelled" {
				return fmt.Errorf("task cancelled")
			}
		}
	}

	// Set result based on task type
	switch task.Type {
	case "analysis":
		task.Result = map[string]interface{}{
			"findings": []string{"Finding 1", "Finding 2"},
			"score":    85,
		}
	case "generation":
		task.Result = map[string]interface{}{
			"output": "Generated content",
			"tokens": 1500,
		}
	default:
		task.Result = map[string]interface{}{
			"message": fmt.Sprintf("Task %s completed successfully", task.Type),
		}
	}

	return nil
}

// UpdateTaskProgress updates task progress with optional result
func (tm *TaskManager) UpdateTaskProgress(ctx context.Context, taskID, agentID string, progress int, result map[string]interface{}) (*Task, error) {
	val, ok := tm.tasks.Load(taskID)
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	task := val.(*Task)

	// Verify agent has permission to update
	if task.AgentID != agentID && task.AssignedTo != agentID {
		return nil, fmt.Errorf("agent %s not authorized to update task %s", agentID, taskID)
	}

	if task.Status != "running" {
		return nil, fmt.Errorf("task is not running")
	}

	task.Progress = float64(progress)
	task.UpdatedAt = time.Now()

	if result != nil {
		task.Result = result
	}

	tm.tasks.Store(taskID, task)

	if tm.metrics != nil {
		tm.metrics.RecordGauge("task.progress", float64(progress), map[string]string{
			"task_id": taskID,
			"type":    task.Type,
		})
	}

	return task, nil
}

// DelegateTask delegates a task to another agent
func (tm *TaskManager) DelegateTask(ctx context.Context, taskID, fromAgentID, toAgentID, reason string) (*DelegationResult, error) {
	val, ok := tm.tasks.Load(taskID)
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	task := val.(*Task)

	// Verify current agent owns the task
	if task.AssignedTo != fromAgentID {
		return nil, fmt.Errorf("agent %s is not assigned to task %s", fromAgentID, taskID)
	}

	// Update task assignment
	task.AssignedTo = toAgentID
	task.UpdatedAt = time.Now()

	tm.tasks.Store(taskID, task)

	delegation := &DelegationResult{
		TaskID:      taskID,
		DelegatedAt: time.Now(),
	}

	tm.logger.Info("Task delegated", map[string]interface{}{
		"task_id":    taskID,
		"from_agent": fromAgentID,
		"to_agent":   toAgentID,
		"reason":     reason,
	})

	return delegation, nil
}

// AcceptTask allows an agent to accept a task
func (tm *TaskManager) AcceptTask(ctx context.Context, taskID, agentID string, estimatedDuration time.Duration) (*Task, error) {
	val, ok := tm.tasks.Load(taskID)
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	task := val.(*Task)

	if task.Status != "pending" {
		return nil, fmt.Errorf("task is not pending")
	}

	task.AssignedTo = agentID
	task.Status = "accepted"
	task.UpdatedAt = time.Now()

	tm.tasks.Store(taskID, task)

	tm.metrics.IncrementCounter("tasks_accepted", 1)

	return task, nil
}

// CompleteTask marks a task as completed
func (tm *TaskManager) CompleteTask(ctx context.Context, taskID, agentID string, result map[string]interface{}) (*Task, error) {
	val, ok := tm.tasks.Load(taskID)
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	task := val.(*Task)

	// Verify agent has permission
	if task.AssignedTo != agentID && task.AgentID != agentID {
		return nil, fmt.Errorf("agent %s not authorized to complete task %s", agentID, taskID)
	}

	task.Status = "completed"
	task.Progress = 100
	task.Result = result
	task.CompletedAt = time.Now()
	task.Duration = task.CompletedAt.Sub(task.StartedAt)

	tm.tasks.Store(taskID, task)

	tm.metrics.IncrementCounter("tasks_completed", 1)

	return task, nil
}

// FailTask marks a task as failed
func (tm *TaskManager) FailTask(ctx context.Context, taskID, agentID, errorMsg, reason string) (*Task, error) {
	val, ok := tm.tasks.Load(taskID)
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	task := val.(*Task)

	// Verify agent has permission
	if task.AssignedTo != agentID && task.AgentID != agentID {
		return nil, fmt.Errorf("agent %s not authorized to fail task %s", agentID, taskID)
	}

	task.Status = "failed"
	task.Error = errorMsg
	task.CompletedAt = time.Now()
	task.Duration = task.CompletedAt.Sub(task.StartedAt)

	tm.tasks.Store(taskID, task)

	tm.metrics.IncrementCounter("tasks_failed", 1)

	return task, nil
}

// CreateDistributedTask creates a task that can be distributed across multiple agents
func (tm *TaskManager) CreateDistributedTask(ctx context.Context, def *DistributedTaskDefinition) (*Task, error) {
	// Create main task
	mainTask := &Task{
		ID:         uuid.New().String(),
		Type:       def.Type,
		Parameters: def.Parameters,
		Status:     "pending",
		Priority:   "normal",
		Progress:   0,
		AgentID:    def.AgentID,
		TenantID:   def.TenantID,
		CreatedAt:  time.Now(),
		Attempts:   0,
		MaxRetries: 3,
		Timeout:    30 * time.Second,
	}

	// Initialize Parameters if nil
	if mainTask.Parameters == nil {
		mainTask.Parameters = make(map[string]interface{})
	}

	// Store distributed task metadata
	mainTask.Parameters["_distributed"] = true
	mainTask.Parameters["_subtasks"] = def.Subtasks
	mainTask.Parameters["_strategy"] = def.Strategy
	mainTask.Parameters["_max_workers"] = def.MaxWorkers

	tm.tasks.Store(mainTask.ID, mainTask)

	// Create subtasks
	subtaskIDs := []string{}
	for _, subtaskDef := range def.Subtasks {
		subtask := &Task{
			ID:         uuid.New().String(),
			Type:       def.Type,
			Parameters: subtaskDef,
			Status:     "pending",
			Priority:   "normal",
			Progress:   0,
			AgentID:    def.AgentID,
			TenantID:   def.TenantID,
			CreatedAt:  time.Now(),
			Attempts:   0,
			MaxRetries: 3,
			Timeout:    30 * time.Second,
		}
		subtask.Parameters["_parent_task"] = mainTask.ID

		tm.tasks.Store(subtask.ID, subtask)
		subtaskIDs = append(subtaskIDs, subtask.ID)
	}

	mainTask.Parameters["_subtask_ids"] = subtaskIDs

	tm.logger.Info("Distributed task created", map[string]interface{}{
		"task_id":       mainTask.ID,
		"type":          def.Type,
		"subtask_count": len(def.Subtasks),
		"strategy":      def.Strategy,
	})

	return mainTask, nil
}

// DistributedTaskDefinition defines a distributed task
type DistributedTaskDefinition struct {
	Type       string                   `json:"type"`
	Parameters map[string]interface{}   `json:"parameters"`
	Subtasks   []map[string]interface{} `json:"subtasks"`
	Strategy   string                   `json:"strategy"` // parallel, sequential
	MaxWorkers int                      `json:"max_workers"`
	AgentID    string                   `json:"agent_id"`
	TenantID   string                   `json:"tenant_id"`
}
