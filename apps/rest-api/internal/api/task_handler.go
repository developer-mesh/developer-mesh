package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository/interfaces"
	pkgservices "github.com/developer-mesh/developer-mesh/pkg/services"
)

// TaskHandler handles task management endpoints
type TaskHandler struct {
	taskService      pkgservices.TaskService
	assignmentEngine *pkgservices.AssignmentEngine
	logger           observability.Logger
	metricsClient    observability.MetricsClient
}

// NewTaskHandler creates a new task handler
func NewTaskHandler(
	taskService pkgservices.TaskService,
	assignmentEngine *pkgservices.AssignmentEngine,
	logger observability.Logger,
	metricsClient observability.MetricsClient,
) *TaskHandler {
	return &TaskHandler{
		taskService:      taskService,
		assignmentEngine: assignmentEngine,
		logger:           logger,
		metricsClient:    metricsClient,
	}
}

// RegisterRoutes registers all task API routes
func (h *TaskHandler) RegisterRoutes(router *gin.RouterGroup) {
	tasks := router.Group("/tasks")
	{
		// Task lifecycle
		tasks.POST("", h.CreateTask)
		tasks.GET("", h.ListTasks)
		tasks.GET("/:id", h.GetTask)
		tasks.PUT("/:id", h.UpdateTask)
		tasks.POST("/:id/assign", h.AssignTask)
		tasks.POST("/:id/complete", h.CompleteTask)
		tasks.POST("/:id/fail", h.FailTask)
		tasks.GET("/:id/subtasks", h.GetSubtasks)

		// Batch operations
		tasks.POST("/batch", h.CreateBatch)
		tasks.POST("/batch/get", h.GetBatch)

		// Auto-assignment
		tasks.POST("/:id/auto-assign", h.AutoAssignTask)
	}
}

// Request/Response types

// CreateTaskRequest represents a task creation request
type CreateTaskRequest struct {
	Type           string                 `json:"type" binding:"required"`
	Title          string                 `json:"title" binding:"required"`
	Description    string                 `json:"description"`
	Priority       models.TaskPriority    `json:"priority"`
	Parameters     map[string]interface{} `json:"parameters"`
	CreatedBy      string                 `json:"created_by"`
	IdempotencyKey string                 `json:"idempotency_key"`
	AutoAssign     bool                   `json:"auto_assign"`
	ParentTaskID   *uuid.UUID             `json:"parent_task_id"`
}

// UpdateTaskRequest represents a task update request
type UpdateTaskRequest struct {
	Status      *models.TaskStatus     `json:"status"`
	Priority    *models.TaskPriority   `json:"priority"`
	Title       *string                `json:"title"`
	Description *string                `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// AssignTaskRequest represents a task assignment request
type AssignTaskRequest struct {
	AgentID string `json:"agent_id" binding:"required"`
	Reason  string `json:"reason"`
}

// CompleteTaskRequest represents a task completion request
type CompleteTaskRequest struct {
	AgentID string                 `json:"agent_id"`
	Result  map[string]interface{} `json:"result"`
}

// FailTaskRequest represents a task failure request
type FailTaskRequest struct {
	AgentID string                 `json:"agent_id"`
	Error   string                 `json:"error" binding:"required"`
	Reason  string                 `json:"reason"`
	Detail  map[string]interface{} `json:"detail"`
}

// CreateBatchRequest represents a batch task creation request
type CreateBatchRequest struct {
	Tasks []CreateTaskRequest `json:"tasks" binding:"required"`
}

// GetBatchRequest represents a batch task retrieval request
type GetBatchRequest struct {
	TaskIDs []uuid.UUID `json:"task_ids" binding:"required"`
}

// TaskErrorResponse represents an error response
type TaskErrorResponse struct {
	Error   string                 `json:"error"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// Handler implementations

// CreateTask creates a new task and optionally auto-assigns it
// @Summary Create a new task
// @Description Creates a new task in the system, optionally with auto-assignment
// @Tags Tasks
// @Accept json
// @Produce json
// @Param request body CreateTaskRequest true "Task creation request"
// @Success 201 {object} models.Task
// @Failure 400 {object} TaskErrorResponse
// @Failure 401 {object} TaskErrorResponse
// @Failure 500 {object} TaskErrorResponse
// @Router /api/v1/tasks [post]
func (h *TaskHandler) CreateTask(c *gin.Context) {
	start := time.Now()

	// Parse request
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.recordMetric("task.create.error", 1, map[string]string{"error": "invalid_request"})
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Get tenant from context (set by auth middleware)
	tenantIDStr := c.GetString("tenant_id")
	if tenantIDStr == "" {
		h.recordMetric("task.create.error", 1, map[string]string{"error": "missing_tenant"})
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		h.recordMetric("task.create.error", 1, map[string]string{"error": "invalid_tenant"})
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant_id format"})
		return
	}

	// Create task
	task := &models.Task{
		TenantID:     tenantID,
		Type:         req.Type,
		Title:        req.Title,
		Description:  req.Description,
		Priority:     req.Priority,
		Parameters:   req.Parameters,
		CreatedBy:    req.CreatedBy,
		ParentTaskID: req.ParentTaskID,
	}

	// Create task (modifies task in place)
	if err := h.taskService.Create(c.Request.Context(), task, req.IdempotencyKey); err != nil {
		h.logger.Error("Failed to create task", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
		})
		h.recordMetric("task.create.error", 1, map[string]string{"error": "create_failed"})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	// Auto-assign if requested
	if req.AutoAssign {
		agent, assignErr := h.assignmentEngine.FindBestAgent(c.Request.Context(), task)
		if assignErr == nil && agent != nil {
			if err := h.taskService.AssignTask(c.Request.Context(), task.ID, agent.ID); err == nil {
				task.AssignedTo = &agent.ID
				h.logger.Info("Task auto-assigned", map[string]interface{}{
					"task_id":  task.ID.String(),
					"agent_id": agent.ID,
				})
			}
		}
	}

	// Record success metrics
	h.recordMetric("task.create.success", 1, map[string]string{
		"type":     task.Type,
		"priority": string(task.Priority),
	})
	h.recordMetric("task.create.duration_ms", float64(time.Since(start).Milliseconds()), nil)

	// Audit log using regular logger
	h.logger.Info("Task created", map[string]interface{}{
		"action":      "task.create",
		"task_id":     task.ID.String(),
		"tenant_id":   tenantID.String(),
		"type":        task.Type,
		"priority":    task.Priority,
		"auto_assign": req.AutoAssign,
	})

	c.JSON(http.StatusCreated, task)
}

// ListTasks lists tasks with optional filtering
// @Summary List tasks
// @Description Lists tasks with optional filtering by status, priority, agent, etc.
// @Tags Tasks
// @Accept json
// @Produce json
// @Param status query string false "Filter by status"
// @Param priority query string false "Filter by priority"
// @Param agent_id query string false "Filter by assigned agent"
// @Param limit query int false "Limit results"
// @Param offset query int false "Offset for pagination"
// @Success 200 {array} models.Task
// @Failure 400 {object} TaskErrorResponse
// @Failure 500 {object} TaskErrorResponse
// @Router /api/v1/tasks [get]
func (h *TaskHandler) ListTasks(c *gin.Context) {
	start := time.Now()

	// Get tenant from context
	tenantIDStr := c.GetString("tenant_id")
	if tenantIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
		return
	}

	_, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant_id format"})
		return
	}

	// Build filters from query parameters
	filters := interfaces.TaskFilters{
		Limit:  50, // default
		Offset: 0,  // default
	}

	if status := c.Query("status"); status != "" {
		filters.Status = []string{status}
	}
	if priority := c.Query("priority"); priority != "" {
		filters.Priority = []string{priority}
	}
	if agentID := c.Query("agent_id"); agentID != "" {
		filters.AssignedTo = &agentID
	}

	// Get pagination parameters
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := parseInt(limitStr); err == nil && l > 0 {
			filters.Limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := parseInt(offsetStr); err == nil && o >= 0 {
			filters.Offset = o
		}
	}

	// Search tasks with filters
	tasks, err := h.taskService.SearchTasks(c.Request.Context(), "", filters)
	if err != nil {
		h.logger.Error("Failed to list tasks", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantIDStr,
		})
		h.recordMetric("task.list.error", 1, map[string]string{"error": "list_failed"})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list tasks"})
		return
	}

	// Record success metrics
	h.recordMetric("task.list.success", 1, nil)
	h.recordMetric("task.list.duration_ms", float64(time.Since(start).Milliseconds()), nil)
	h.recordMetric("task.list.count", float64(len(tasks)), nil)

	c.JSON(http.StatusOK, gin.H{
		"tasks":  tasks,
		"count":  len(tasks),
		"limit":  filters.Limit,
		"offset": filters.Offset,
	})
}

// GetTask retrieves a specific task by ID
// @Summary Get task by ID
// @Description Retrieves detailed information about a specific task
// @Tags Tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} models.Task
// @Failure 400 {object} TaskErrorResponse
// @Failure 404 {object} TaskErrorResponse
// @Failure 500 {object} TaskErrorResponse
// @Router /api/v1/tasks/{id} [get]
func (h *TaskHandler) GetTask(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID format"})
		return
	}

	task, err := h.taskService.Get(c.Request.Context(), taskID)
	if err != nil {
		h.logger.Warn("Task not found", map[string]interface{}{
			"task_id": taskID.String(),
			"error":   err.Error(),
		})
		h.recordMetric("task.get.error", 1, map[string]string{"error": "not_found"})
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	h.recordMetric("task.get.success", 1, nil)
	c.JSON(http.StatusOK, task)
}

// UpdateTask updates an existing task
// @Summary Update task
// @Description Updates task properties such as status, priority, title, etc.
// @Tags Tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Param request body UpdateTaskRequest true "Task update request"
// @Success 200 {object} models.Task
// @Failure 400 {object} TaskErrorResponse
// @Failure 404 {object} TaskErrorResponse
// @Failure 500 {object} TaskErrorResponse
// @Router /api/v1/tasks/{id} [put]
func (h *TaskHandler) UpdateTask(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID format"})
		return
	}

	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Get existing task
	task, err := h.taskService.Get(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	// Update fields if provided
	if req.Status != nil {
		task.Status = *req.Status
	}
	if req.Priority != nil {
		task.Priority = *req.Priority
	}
	if req.Title != nil {
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Parameters != nil {
		task.Parameters = req.Parameters
	}

	// Update task (modifies in place)
	if err := h.taskService.Update(c.Request.Context(), task); err != nil {
		h.logger.Error("Failed to update task", map[string]interface{}{
			"error":   err.Error(),
			"task_id": taskID.String(),
		})
		h.recordMetric("task.update.error", 1, map[string]string{"error": "update_failed"})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update task"})
		return
	}

	// Get updated task to return
	updatedTask, err := h.taskService.Get(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve updated task"})
		return
	}

	h.recordMetric("task.update.success", 1, nil)

	// Audit log using regular logger
	h.logger.Info("Task updated", map[string]interface{}{
		"action":   "task.update",
		"task_id":  taskID.String(),
		"status":   task.Status,
		"priority": task.Priority,
	})

	c.JSON(http.StatusOK, updatedTask)
}

// AssignTask manually assigns a task to a specific agent
// @Summary Assign task to agent
// @Description Manually assigns a task to a specific agent
// @Tags Tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Param request body AssignTaskRequest true "Assignment request"
// @Success 200 {object} models.Task
// @Failure 400 {object} TaskErrorResponse
// @Failure 404 {object} TaskErrorResponse
// @Failure 500 {object} TaskErrorResponse
// @Router /api/v1/tasks/{id}/assign [post]
func (h *TaskHandler) AssignTask(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID format"})
		return
	}

	var req AssignTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Assign task
	if err := h.taskService.AssignTask(c.Request.Context(), taskID, req.AgentID); err != nil {
		h.logger.Error("Failed to assign task", map[string]interface{}{
			"error":    err.Error(),
			"task_id":  taskID.String(),
			"agent_id": req.AgentID,
		})
		h.recordMetric("task.assign.error", 1, map[string]string{"error": "assign_failed"})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to assign task"})
		return
	}

	// Get updated task
	task, err := h.taskService.Get(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve updated task"})
		return
	}

	h.recordMetric("task.assign.success", 1, nil)
	h.logger.Info("Task assigned", map[string]interface{}{
		"action":   "task.assign",
		"task_id":  taskID.String(),
		"agent_id": req.AgentID,
		"reason":   req.Reason,
	})

	c.JSON(http.StatusOK, task)
}

// AutoAssignTask automatically assigns a task using the assignment engine
// @Summary Auto-assign task
// @Description Automatically assigns a task to the best available agent using the assignment engine
// @Tags Tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} TaskErrorResponse
// @Failure 404 {object} TaskErrorResponse
// @Failure 503 {object} TaskErrorResponse "No agent available"
// @Router /api/v1/tasks/{id}/auto-assign [post]
func (h *TaskHandler) AutoAssignTask(c *gin.Context) {
	start := time.Now()

	taskIDStr := c.Param("id")
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID format"})
		return
	}

	// Get task
	task, err := h.taskService.Get(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	// Find best agent (strategy determined internally by assignment engine)
	agent, err := h.assignmentEngine.FindBestAgent(c.Request.Context(), task)
	if err != nil {
		h.logger.Warn("No agent available for task", map[string]interface{}{
			"task_id": taskID.String(),
			"error":   err.Error(),
		})
		h.recordMetric("task.auto_assign.error", 1, map[string]string{"error": "no_agent"})
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no agent available"})
		return
	}

	// Assign task
	if err := h.taskService.AssignTask(c.Request.Context(), task.ID, agent.ID); err != nil {
		h.logger.Error("Failed to auto-assign task", map[string]interface{}{
			"error":    err.Error(),
			"task_id":  taskID.String(),
			"agent_id": agent.ID,
		})
		h.recordMetric("task.auto_assign.error", 1, map[string]string{"error": "assign_failed"})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to assign task"})
		return
	}

	h.recordMetric("task.auto_assign.success", 1, nil)
	h.recordMetric("task.auto_assign.duration_ms", float64(time.Since(start).Milliseconds()), nil)

	h.logger.Info("Task auto-assigned", map[string]interface{}{
		"action":     "task.auto_assign",
		"task_id":    taskID.String(),
		"agent_id":   agent.ID,
		"agent_type": agent.Type,
	})

	c.JSON(http.StatusOK, gin.H{
		"task_id":     task.ID,
		"agent_id":    agent.ID,
		"agent_name":  agent.Name,
		"agent_type":  agent.Type,
		"assigned_at": time.Now(),
	})
}

// CompleteTask marks a task as completed
// @Summary Complete task
// @Description Marks a task as completed with optional result data
// @Tags Tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Param request body CompleteTaskRequest false "Completion details"
// @Success 200 {object} models.Task
// @Failure 400 {object} TaskErrorResponse
// @Failure 404 {object} TaskErrorResponse
// @Failure 500 {object} TaskErrorResponse
// @Router /api/v1/tasks/{id}/complete [post]
func (h *TaskHandler) CompleteTask(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID format"})
		return
	}

	var req CompleteTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// It's okay if there's no body - use defaults
		req.AgentID = ""
		req.Result = nil
	}

	// Get agent ID from request or context
	agentID := req.AgentID
	if agentID == "" {
		agentID = c.GetString("agent_id") // May be empty, that's okay
	}

	// Complete task
	if err := h.taskService.CompleteTask(c.Request.Context(), taskID, agentID, req.Result); err != nil {
		h.logger.Error("Failed to complete task", map[string]interface{}{
			"error":   err.Error(),
			"task_id": taskID.String(),
		})
		h.recordMetric("task.complete.error", 1, map[string]string{"error": "complete_failed"})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to complete task"})
		return
	}

	// Get updated task
	task, err := h.taskService.Get(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve updated task"})
		return
	}

	h.recordMetric("task.complete.success", 1, nil)
	h.logger.Info("Task completed", map[string]interface{}{
		"action":     "task.complete",
		"task_id":    taskID.String(),
		"agent_id":   agentID,
		"has_result": req.Result != nil,
	})

	c.JSON(http.StatusOK, task)
}

// FailTask marks a task as failed
// @Summary Fail task
// @Description Marks a task as failed with error details
// @Tags Tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Param request body FailTaskRequest true "Failure details"
// @Success 200 {object} models.Task
// @Failure 400 {object} TaskErrorResponse
// @Failure 404 {object} TaskErrorResponse
// @Failure 500 {object} TaskErrorResponse
// @Router /api/v1/tasks/{id}/fail [post]
func (h *TaskHandler) FailTask(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID format"})
		return
	}

	var req FailTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Get agent ID from request or context
	agentID := req.AgentID
	if agentID == "" {
		agentID = c.GetString("agent_id") // May be empty, that's okay
	}

	// Fail task
	if err := h.taskService.FailTask(c.Request.Context(), taskID, agentID, req.Error); err != nil {
		h.logger.Error("Failed to mark task as failed", map[string]interface{}{
			"error":   err.Error(),
			"task_id": taskID.String(),
		})
		h.recordMetric("task.fail.error", 1, map[string]string{"error": "fail_operation_failed"})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fail task"})
		return
	}

	// Get updated task
	task, err := h.taskService.Get(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve updated task"})
		return
	}

	h.recordMetric("task.fail.success", 1, nil)
	h.logger.Warn("Task marked as failed", map[string]interface{}{
		"action":   "task.fail",
		"task_id":  taskID.String(),
		"agent_id": agentID,
		"error":    req.Error,
		"reason":   req.Reason,
	})

	c.JSON(http.StatusOK, task)
}

// GetSubtasks retrieves all subtasks of a parent task
// @Summary Get subtasks
// @Description Retrieves all subtasks for a given parent task
// @Tags Tasks
// @Accept json
// @Produce json
// @Param id path string true "Parent Task ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} TaskErrorResponse
// @Failure 500 {object} TaskErrorResponse
// @Router /api/v1/tasks/{id}/subtasks [get]
func (h *TaskHandler) GetSubtasks(c *gin.Context) {
	parentTaskIDStr := c.Param("id")
	parentTaskID, err := uuid.Parse(parentTaskIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID format"})
		return
	}

	// Get task tree which includes subtasks
	taskTree, err := h.taskService.GetTaskTree(c.Request.Context(), parentTaskID)
	if err != nil {
		h.logger.Error("Failed to get task tree", map[string]interface{}{
			"error":          err.Error(),
			"parent_task_id": parentTaskID.String(),
		})
		h.recordMetric("task.subtasks.error", 1, map[string]string{"error": "get_tree_failed"})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get subtasks"})
		return
	}

	// Extract subtasks from tree
	// TaskTree.Children is a map[uuid.UUID][]*Task mapping parent IDs to their children
	var subtasks []*models.Task
	if taskTree != nil && taskTree.Children != nil {
		// Get the children for the requested parent task ID
		if children, ok := taskTree.Children[parentTaskID]; ok {
			subtasks = children
		} else {
			subtasks = make([]*models.Task, 0)
		}
	} else {
		subtasks = make([]*models.Task, 0)
	}

	h.recordMetric("task.subtasks.success", 1, nil)
	h.recordMetric("task.subtasks.count", float64(len(subtasks)), nil)

	c.JSON(http.StatusOK, gin.H{
		"parent_task_id": parentTaskID,
		"subtasks":       subtasks,
		"count":          len(subtasks),
	})
}

// CreateBatch creates multiple tasks in a single operation
// @Summary Create batch tasks
// @Description Creates multiple tasks in a single batch operation
// @Tags Tasks
// @Accept json
// @Produce json
// @Param request body CreateBatchRequest true "Batch creation request"
// @Success 201 {array} models.Task
// @Failure 400 {object} TaskErrorResponse
// @Failure 500 {object} TaskErrorResponse
// @Router /api/v1/tasks/batch [post]
func (h *TaskHandler) CreateBatch(c *gin.Context) {
	start := time.Now()

	var req CreateBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Get tenant from context
	tenantIDStr := c.GetString("tenant_id")
	if tenantIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant_id format"})
		return
	}

	// Convert request tasks to models
	tasks := make([]*models.Task, len(req.Tasks))
	for i, taskReq := range req.Tasks {
		tasks[i] = &models.Task{
			TenantID:     tenantID,
			Type:         taskReq.Type,
			Title:        taskReq.Title,
			Description:  taskReq.Description,
			Priority:     taskReq.Priority,
			Parameters:   taskReq.Parameters,
			CreatedBy:    taskReq.CreatedBy,
			ParentTaskID: taskReq.ParentTaskID,
		}
	}

	// Create tasks in batch (modifies tasks in place)
	if err := h.taskService.CreateBatch(c.Request.Context(), tasks); err != nil {
		h.logger.Error("Failed to create batch tasks", map[string]interface{}{
			"error":     err.Error(),
			"count":     len(tasks),
			"tenant_id": tenantID.String(),
		})
		h.recordMetric("task.batch.create.error", 1, map[string]string{"error": "batch_create_failed"})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create batch tasks"})
		return
	}

	h.recordMetric("task.batch.create.success", 1, nil)
	h.recordMetric("task.batch.create.count", float64(len(tasks)), nil)
	h.recordMetric("task.batch.create.duration_ms", float64(time.Since(start).Milliseconds()), nil)

	h.logger.Info("Batch tasks created", map[string]interface{}{
		"action":    "task.batch.create",
		"count":     len(tasks),
		"tenant_id": tenantID.String(),
	})

	c.JSON(http.StatusCreated, gin.H{
		"tasks": tasks,
		"count": len(tasks),
	})
}

// GetBatch retrieves multiple tasks by IDs in a single operation
// @Summary Get batch tasks
// @Description Retrieves multiple tasks by their IDs in a single batch operation
// @Tags Tasks
// @Accept json
// @Produce json
// @Param request body GetBatchRequest true "Batch retrieval request"
// @Success 200 {array} models.Task
// @Failure 400 {object} TaskErrorResponse
// @Failure 500 {object} TaskErrorResponse
// @Router /api/v1/tasks/batch/get [post]
func (h *TaskHandler) GetBatch(c *gin.Context) {
	start := time.Now()

	var req GetBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Get tasks in batch
	tasks, err := h.taskService.GetBatch(c.Request.Context(), req.TaskIDs)
	if err != nil {
		h.logger.Error("Failed to get batch tasks", map[string]interface{}{
			"error": err.Error(),
			"count": len(req.TaskIDs),
		})
		h.recordMetric("task.batch.get.error", 1, map[string]string{"error": "batch_get_failed"})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get batch tasks"})
		return
	}

	h.recordMetric("task.batch.get.success", 1, nil)
	h.recordMetric("task.batch.get.count", float64(len(tasks)), nil)
	h.recordMetric("task.batch.get.duration_ms", float64(time.Since(start).Milliseconds()), nil)

	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"count": len(tasks),
	})
}

// Helper methods

// recordMetric records a metric with optional tags
func (h *TaskHandler) recordMetric(name string, value float64, tags map[string]string) {
	if h.metricsClient == nil {
		return
	}

	// Add common tags
	if tags == nil {
		tags = make(map[string]string)
	}
	tags["handler"] = "task"

	h.metricsClient.RecordCounter(name, value, tags)
}

// parseInt safely parses an integer parameter
func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}
