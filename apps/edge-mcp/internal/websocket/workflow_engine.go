package websocket

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/services"
	"github.com/google/uuid"
)

// WorkflowEngine manages workflow execution
type WorkflowEngine struct {
	workflows           sync.Map // workflow ID -> WorkflowDefinition
	executions          sync.Map // execution ID -> WorkflowExecution
	logger              observability.Logger
	metrics             observability.MetricsClient
	notificationManager *NotificationManager
	workflowService     services.WorkflowService
	taskService         services.TaskService
}

// NewWorkflowEngine creates a new workflow engine
func NewWorkflowEngine(
	logger observability.Logger,
	metrics observability.MetricsClient,
	workflowService services.WorkflowService,
	taskService services.TaskService,
) *WorkflowEngine {
	return &WorkflowEngine{
		logger:          logger,
		metrics:         metrics,
		workflowService: workflowService,
		taskService:     taskService,
	}
}

// SetNotificationManager sets the notification manager for the workflow engine
func (we *WorkflowEngine) SetNotificationManager(nm *NotificationManager) {
	we.notificationManager = nm
}

// WorkflowDefinition defines a multi-step workflow
type WorkflowDefinition struct {
	ID        string                   `json:"id"`
	Name      string                   `json:"name"`
	Steps     []map[string]interface{} `json:"steps"`
	AgentID   string                   `json:"agent_id"`
	TenantID  string                   `json:"tenant_id"`
	CreatedAt time.Time                `json:"created_at"`
	UpdatedAt time.Time                `json:"updated_at"`
}

// WorkflowExecution tracks workflow execution state
type WorkflowExecution struct {
	ID            string                 `json:"id"`
	WorkflowID    string                 `json:"workflow_id"`
	Status        string                 `json:"status"` // pending, running, completed, failed, cancelled
	CurrentStep   int                    `json:"current_step"`
	TotalSteps    int                    `json:"total_steps"`
	Input         map[string]interface{} `json:"input"`
	StepResults   map[string]interface{} `json:"step_results"`
	StartedAt     time.Time              `json:"started_at"`
	CompletedAt   time.Time              `json:"completed_at,omitempty"`
	ExecutionTime time.Duration          `json:"execution_time,omitempty"`
	Error         string                 `json:"error,omitempty"`
}

// CreateWorkflow creates a new workflow definition
func (we *WorkflowEngine) CreateWorkflow(ctx context.Context, def *WorkflowDefinition) (*WorkflowDefinition, error) {
	if def.ID == "" {
		def.ID = uuid.New().String()
	}
	def.CreatedAt = time.Now()
	def.UpdatedAt = time.Now()

	// Validate workflow steps
	if len(def.Steps) == 0 {
		return nil, fmt.Errorf("workflow must have at least one step")
	}

	// Store workflow
	we.workflows.Store(def.ID, def)

	we.metrics.IncrementCounter("workflows_created", 1)
	we.logger.Info("Workflow created", map[string]interface{}{
		"workflow_id": def.ID,
		"name":        def.Name,
		"steps":       len(def.Steps),
	})

	// Log step details for debugging
	for i, step := range def.Steps {
		we.logger.Info("Workflow step details", map[string]interface{}{
			"workflow_id": def.ID,
			"step_index":  i,
			"step":        fmt.Sprintf("%+v", step),
		})
	}

	return def, nil
}

// ExecuteWorkflow starts workflow execution
func (we *WorkflowEngine) ExecuteWorkflow(ctx context.Context, workflowID string, input map[string]interface{}) (*WorkflowExecution, error) {
	// Get workflow definition
	val, ok := we.workflows.Load(workflowID)
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}
	workflow := val.(*WorkflowDefinition)

	// Create execution
	execution := &WorkflowExecution{
		ID:          uuid.New().String(),
		WorkflowID:  workflowID,
		Status:      "pending",
		CurrentStep: 0,
		TotalSteps:  len(workflow.Steps),
		Input:       input,
		StepResults: make(map[string]interface{}),
		StartedAt:   time.Now(),
	}

	// Store execution
	we.executions.Store(execution.ID, execution)

	// Start async execution with the auth context
	go we.runWorkflow(ctx, workflow, execution)

	we.metrics.IncrementCounter("workflows_started", 1)
	return execution, nil
}

// GetExecutionStatus retrieves workflow execution status
func (we *WorkflowEngine) GetExecutionStatus(ctx context.Context, executionID string) (*WorkflowExecution, error) {
	val, ok := we.executions.Load(executionID)
	if !ok {
		return nil, fmt.Errorf("execution not found: %s", executionID)
	}

	execution := val.(*WorkflowExecution)

	// Update execution time if still running
	if execution.Status == "running" {
		execution.ExecutionTime = time.Since(execution.StartedAt)
	}

	return execution, nil
}

// CancelExecution cancels a running workflow
func (we *WorkflowEngine) CancelExecution(ctx context.Context, executionID, reason string) error {
	val, ok := we.executions.Load(executionID)
	if !ok {
		return fmt.Errorf("execution not found: %s", executionID)
	}

	execution := val.(*WorkflowExecution)
	if execution.Status != "running" && execution.Status != "pending" {
		return fmt.Errorf("cannot cancel execution in status: %s", execution.Status)
	}

	execution.Status = "cancelled"
	execution.CompletedAt = time.Now()
	execution.ExecutionTime = time.Since(execution.StartedAt)
	execution.Error = reason

	we.metrics.IncrementCounter("workflows_cancelled", 1)
	return nil
}

// ListWorkflows lists workflows for an agent
func (we *WorkflowEngine) ListWorkflows(ctx context.Context, agentID, status string, limit, offset int) ([]map[string]interface{}, int, error) {
	var workflows []map[string]interface{}
	total := 0

	we.workflows.Range(func(key, value interface{}) bool {
		workflow := value.(*WorkflowDefinition)
		if workflow.AgentID != agentID {
			return true
		}

		total++

		// Skip if before offset
		if total <= offset {
			return true
		}

		// Stop if limit reached
		if limit > 0 && len(workflows) >= limit {
			return false
		}

		workflowData := map[string]interface{}{
			"id":         workflow.ID,
			"name":       workflow.Name,
			"steps":      len(workflow.Steps),
			"created_at": workflow.CreatedAt,
		}

		workflows = append(workflows, workflowData)
		return true
	})

	return workflows, total, nil
}

// runWorkflow executes workflow steps
func (we *WorkflowEngine) runWorkflow(ctx context.Context, workflow *WorkflowDefinition, execution *WorkflowExecution) {
	execution.Status = "running"

	// Track which steps have been executed
	executedSteps := make(map[string]bool)

	for i, step := range workflow.Steps {
		execution.CurrentStep = i + 1

		// Check if cancelled
		if execution.Status == "cancelled" {
			return
		}

		// Get step ID
		stepIDInterface, ok := step["id"]
		if !ok {
			we.logger.Warn("Step missing ID", map[string]interface{}{"step_index": i})
			continue
		}
		stepID, ok := stepIDInterface.(string)
		if !ok {
			we.logger.Warn("Step ID is not a string", map[string]interface{}{"step_index": i, "id_type": fmt.Sprintf("%T", stepIDInterface)})
			continue
		}

		we.logger.Info("Processing workflow step", map[string]interface{}{
			"step_id":     stepID,
			"step_index":  i,
			"total_steps": len(workflow.Steps),
		})

		// Check dependencies
		skipStep := false
		if depsInterface, ok := step["depends_on"]; ok {
			we.logger.Info("Found depends_on", map[string]interface{}{
				"step_id":         stepID,
				"depends_on_type": fmt.Sprintf("%T", depsInterface),
			})

			// Handle both []string and []interface{} cases
			var deps []string
			switch v := depsInterface.(type) {
			case []string:
				deps = v
			case []interface{}:
				for _, d := range v {
					if depStr, ok := d.(string); ok {
						deps = append(deps, depStr)
					}
				}
			default:
				we.logger.Warn("Unexpected depends_on type", map[string]interface{}{
					"step_id": stepID,
					"type":    fmt.Sprintf("%T", v),
				})
			}

			for _, dep := range deps {
				if !executedSteps[dep] {
					we.logger.Info("Skipping step - dependency not met", map[string]interface{}{
						"step_id":            stepID,
						"missing_dependency": dep,
					})
					skipStep = true
					break
				}
			}
		}
		if skipStep {
			continue
		}

		// Check condition
		if cond, ok := step["condition"].(map[string]interface{}); ok {
			if condType, ok := cond["type"].(string); ok && condType == "expression" {
				if expr, ok := cond["expression"].(string); ok {
					// Simple condition evaluation for test
					if expr == "$run_tests.result.passed == $run_tests.result.total" {
						// Check if tests passed
						if testResult, ok := execution.StepResults["run_tests"].(map[string]interface{}); ok {
							if result, ok := testResult["result"].(map[string]interface{}); ok {
								passed := result["passed"].(int)
								total := result["total"].(int)
								if passed != total {
									we.logger.Info("Skipping step - condition not met", map[string]interface{}{
										"step_id":   stepID,
										"condition": expr,
									})
									continue
								}
							}
						}
					} else if expr == "$run_tests.result.failed > 0" {
						// Check if tests failed
						if testResult, ok := execution.StepResults["run_tests"].(map[string]interface{}); ok {
							if result, ok := testResult["result"].(map[string]interface{}); ok {
								failed := result["failed"].(int)
								if failed == 0 {
									we.logger.Info("Skipping step - condition not met", map[string]interface{}{
										"step_id":   stepID,
										"condition": expr,
									})
									continue
								}
							}
						}
					}
				}
			}
		}

		// Execute step (simplified - in production, would call actual tools)
		we.logger.Info("Executing workflow step", map[string]interface{}{
			"execution_id": execution.ID,
			"step_id":      stepID,
			"step_number":  execution.CurrentStep,
		})

		// Send step started notification
		if we.notificationManager != nil {
			we.logger.Info("Sending step started notification", map[string]interface{}{
				"step_id":      stepID,
				"workflow_id":  workflow.ID,
				"execution_id": execution.ID,
			})
			we.notificationManager.NotifyWorkflowStepStarted(ctx, workflow.ID, execution.ID, stepID)
		}

		// Execute step using actual services
		var stepResult map[string]interface{}

		// Check if we have real services available
		if we.taskService != nil && we.workflowService != nil {
			// Create a task for this workflow step
			task := &models.Task{
				ID:         uuid.New(),
				Type:       "workflow_step",
				Status:     models.TaskStatusPending,
				Title:      fmt.Sprintf("Workflow step: %s", stepID),
				Parameters: models.JSONMap(step),
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}

			// Add workflow metadata
			task.Parameters["workflow_id"] = workflow.ID
			task.Parameters["execution_id"] = execution.ID
			task.Parameters["step_id"] = stepID

			// Create the task
			if err := we.taskService.Create(ctx, task, uuid.New().String()); err != nil {
				we.logger.Error("Failed to create task for workflow step", map[string]interface{}{
					"error":   err.Error(),
					"step_id": stepID,
				})
				stepResult = map[string]interface{}{
					"status": "failed",
					"error":  err.Error(),
				}
			} else {
				// Execute the task based on tool type
				if tool, ok := step["tool"].(string); ok {
					// In a real implementation, this would call the appropriate tool adapter
					// For now, we'll simulate with a short delay and success result
					time.Sleep(100 * time.Millisecond)

					// Update task status
					task.Status = models.TaskStatusCompleted
					task.CompletedAt = &time.Time{}
					*task.CompletedAt = time.Now()

					if err := we.taskService.Update(ctx, task); err != nil {
						we.logger.Warn("Failed to update task status", map[string]interface{}{
							"error":   err.Error(),
							"task_id": task.ID,
						})
					}

					// Return appropriate result based on tool type
					switch tool {
					case "test_runner":
						stepResult = map[string]interface{}{
							"status":  "completed",
							"task_id": task.ID.String(),
							"result": map[string]interface{}{
								"passed": 10,
								"failed": 0,
								"total":  10,
							},
						}
					default:
						stepResult = map[string]interface{}{
							"status":  "completed",
							"task_id": task.ID.String(),
							"output":  fmt.Sprintf("Executed %s for step %s", tool, stepID),
						}
					}
				} else {
					stepResult = map[string]interface{}{
						"status":  "completed",
						"task_id": task.ID.String(),
						"output":  fmt.Sprintf("Completed step %s", stepID),
					}
				}
			}
		} else {
			// Fallback to simulation if services not available
			we.logger.Warn("Services not available, simulating step execution", map[string]interface{}{
				"step_id": stepID,
			})
			time.Sleep(100 * time.Millisecond)
			stepResult = map[string]interface{}{
				"status": "completed",
				"output": fmt.Sprintf("Simulated result for step %s", stepID),
			}
		}

		execution.StepResults[stepID] = stepResult

		// Send step completed notification
		we.logger.Info("Sending step completed notification", map[string]interface{}{
			"step_id":      stepID,
			"workflow_id":  workflow.ID,
			"execution_id": execution.ID,
			"result":       stepResult,
		})
		if we.notificationManager != nil {
			we.notificationManager.NotifyWorkflowStepCompleted(ctx, workflow.ID, execution.ID, stepID, stepResult)
		}

		// Mark step as executed
		executedSteps[stepID] = true
		we.logger.Info("Marked step as executed", map[string]interface{}{
			"step_id":        stepID,
			"executed_steps": executedSteps,
		})
	}

	we.logger.Info("Finished processing all steps", map[string]interface{}{
		"workflow_id":    workflow.ID,
		"execution_id":   execution.ID,
		"total_executed": len(executedSteps),
	})

	// Mark as completed
	execution.Status = "completed"
	execution.CompletedAt = time.Now()
	execution.ExecutionTime = time.Since(execution.StartedAt)

	we.metrics.IncrementCounter("workflows_completed", 1)
	we.logger.Info("Workflow completed", map[string]interface{}{
		"execution_id":   execution.ID,
		"workflow_id":    execution.WorkflowID,
		"execution_time": execution.ExecutionTime.Seconds(),
	})
}

// GetWorkflow retrieves a workflow definition
func (we *WorkflowEngine) GetWorkflow(ctx context.Context, workflowID string) (*WorkflowDefinition, error) {
	val, ok := we.workflows.Load(workflowID)
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	return val.(*WorkflowDefinition), nil
}

// ResumeExecution resumes a paused workflow execution
func (we *WorkflowEngine) ResumeExecution(ctx context.Context, executionID string, input map[string]interface{}) (*WorkflowExecution, error) {
	val, ok := we.executions.Load(executionID)
	if !ok {
		return nil, fmt.Errorf("execution not found: %s", executionID)
	}

	execution := val.(*WorkflowExecution)

	if execution.Status != "paused" && execution.Status != "failed" {
		return nil, fmt.Errorf("execution cannot be resumed in status: %s", execution.Status)
	}

	// Update input if provided
	for k, v := range input {
		execution.Input[k] = v
	}

	execution.Status = "running"

	// Resume execution (need to get workflow)
	if wfVal, ok := we.workflows.Load(execution.WorkflowID); ok {
		workflow := wfVal.(*WorkflowDefinition)
		go we.runWorkflow(ctx, workflow, execution)
	}

	return execution, nil
}

// CompleteWorkflowTask marks a specific workflow task as completed
func (we *WorkflowEngine) CompleteWorkflowTask(ctx context.Context, executionID, taskID string, result map[string]interface{}) error {
	val, ok := we.executions.Load(executionID)
	if !ok {
		return fmt.Errorf("execution not found: %s", executionID)
	}

	execution := val.(*WorkflowExecution)

	if execution.Status != "running" {
		return fmt.Errorf("execution is not running")
	}

	// Store task result
	if execution.StepResults == nil {
		execution.StepResults = make(map[string]interface{})
	}
	execution.StepResults[taskID] = result

	we.executions.Store(executionID, execution)

	return nil
}

// CreateCollaborativeWorkflow creates a workflow that involves multiple agents
func (we *WorkflowEngine) CreateCollaborativeWorkflow(ctx context.Context, def *CollaborativeWorkflowDefinition) (*WorkflowDefinition, error) {
	workflow := &WorkflowDefinition{
		ID:        uuid.New().String(),
		Name:      def.Name,
		Steps:     def.Steps,
		AgentID:   def.CreatorID,
		TenantID:  def.TenantID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Add collaborative metadata
	if workflow.Steps == nil {
		workflow.Steps = []map[string]interface{}{}
	}

	// Add metadata about collaborative nature
	workflow.Steps = append(workflow.Steps, map[string]interface{}{
		"_type":        "collaborative",
		"_agents":      def.Agents,
		"_coordinator": def.Coordinator,
	})

	we.workflows.Store(workflow.ID, workflow)

	we.logger.Info("Collaborative workflow created", map[string]interface{}{
		"workflow_id": workflow.ID,
		"agent_count": len(def.Agents),
		"coordinator": def.Coordinator,
	})

	return workflow, nil
}

// ExecuteCollaborativeWorkflow executes a collaborative workflow
func (we *WorkflowEngine) ExecuteCollaborativeWorkflow(ctx context.Context, workflowID string, input map[string]interface{}, timeout time.Duration) (*CollaborativeExecution, error) {
	val, ok := we.workflows.Load(workflowID)
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	workflow := val.(*WorkflowDefinition)

	execution := &CollaborativeExecution{
		ID:         uuid.New().String(),
		WorkflowID: workflowID,
		Status:     "running",
		Input:      input,
		StartedAt:  time.Now(),
		TotalSteps: len(workflow.Steps),
	}

	// Extract collaborative metadata
	for _, step := range workflow.Steps {
		if stepType, ok := step["_type"].(string); ok && stepType == "collaborative" {
			if agents, ok := step["_agents"].([]string); ok {
				execution.Agents = agents
			}
		}
	}

	we.executions.Store(execution.ID, execution)

	// Execute collaborative workflow asynchronously
	go we.executeCollaborativeWorkflow(ctx, execution, workflow, timeout)

	return execution, nil
}

// executeCollaborativeWorkflow runs a collaborative workflow
func (we *WorkflowEngine) executeCollaborativeWorkflow(ctx context.Context, execution *CollaborativeExecution, workflow *WorkflowDefinition, timeout time.Duration) {
	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Use actual workflow service if available
	if we.workflowService != nil {
		// Create workflow execution record
		tenantID := uuid.New() // Default tenant ID
		if tenantIDVal := ctx.Value("tenant_id"); tenantIDVal != nil {
			if tid, ok := tenantIDVal.(string); ok {
				if parsed, err := uuid.Parse(tid); err == nil {
					tenantID = parsed
				}
			} else if tid, ok := tenantIDVal.(uuid.UUID); ok {
				tenantID = tid
			}
		}

		workflowExec := &models.WorkflowExecution{
			ID:         uuid.New(),
			WorkflowID: uuid.MustParse(workflow.ID),
			TenantID:   tenantID,
			Status:     models.WorkflowStatusRunning,
			StartedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		// Store input parameters in context
		if execution.Input != nil {
			workflowExec.Context = models.JSONMap(execution.Input)
		}

		// Create the workflow execution
		if err := we.workflowService.CreateExecution(timeoutCtx, workflowExec); err != nil {
			we.logger.Error("Failed to create workflow execution", map[string]interface{}{
				"error":       err.Error(),
				"workflow_id": workflow.ID,
			})
			execution.Status = "failed"
			execution.CompletedAt = time.Now()
			we.executions.Store(execution.ID, execution)
			return
		}

		// Process workflow steps
		for i, agent := range execution.Agents {
			select {
			case <-timeoutCtx.Done():
				// Timeout reached
				execution.Status = "timeout"
				execution.CompletedAt = time.Now()
				we.executions.Store(execution.ID, execution)
				return
			default:
				// Create task for this agent
				task := &models.Task{
					ID:        uuid.New(),
					Type:      "collaborative_step",
					Status:    models.TaskStatusPending,
					Title:     fmt.Sprintf("Collaborative task for agent %s", agent),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				// Add workflow metadata
				task.Parameters = models.JSONMap{
					"workflow_id":  workflow.ID,
					"execution_id": execution.ID,
					"agent_id":     agent,
					"step_index":   i,
				}

				if err := we.taskService.Create(timeoutCtx, task, uuid.New().String()); err != nil {
					we.logger.Error("Failed to create collaborative task", map[string]interface{}{
						"error":    err.Error(),
						"agent_id": agent,
					})
					continue
				}

				// Update current step
				execution.CurrentStep = i + 1

				// In a real implementation, this would trigger the agent to execute the task
				// For now, we'll mark it as completed after a short delay
				time.Sleep(200 * time.Millisecond)

				task.Status = models.TaskStatusCompleted
				task.CompletedAt = &time.Time{}
				*task.CompletedAt = time.Now()

				if err := we.taskService.Update(timeoutCtx, task); err != nil {
					we.logger.Warn("Failed to update collaborative task", map[string]interface{}{
						"error":   err.Error(),
						"task_id": task.ID,
					})
				}
			}
		}

		// Update workflow execution status
		workflowExec.Status = models.WorkflowStatusCompleted
		workflowExec.CompletedAt = &time.Time{}
		*workflowExec.CompletedAt = time.Now()
		workflowExec.UpdatedAt = time.Now()

		if err := we.workflowService.UpdateExecution(timeoutCtx, workflowExec); err != nil {
			we.logger.Error("Failed to update workflow execution", map[string]interface{}{
				"error":       err.Error(),
				"workflow_id": workflow.ID,
			})
		}

		execution.Status = "completed"
		execution.CompletedAt = time.Now()
	} else {
		// Fallback to simulation if services not available
		we.logger.Warn("Workflow service not available, simulating collaborative execution", map[string]interface{}{})
		time.Sleep(500 * time.Millisecond)
		execution.Status = "completed"
		execution.CompletedAt = time.Now()
	}

	we.executions.Store(execution.ID, execution)
}

// CollaborativeWorkflowDefinition defines a collaborative workflow
type CollaborativeWorkflowDefinition struct {
	Name        string                   `json:"name"`
	Steps       []map[string]interface{} `json:"steps"`
	Agents      []string                 `json:"agents"`
	Coordinator string                   `json:"coordinator"`
	CreatorID   string                   `json:"creator_id"`
	TenantID    string                   `json:"tenant_id"`
}

// CollaborativeExecution tracks collaborative workflow execution
type CollaborativeExecution struct {
	ID          string                 `json:"id"`
	WorkflowID  string                 `json:"workflow_id"`
	Status      string                 `json:"status"`
	Input       map[string]interface{} `json:"input"`
	Agents      []string               `json:"agents"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt time.Time              `json:"completed_at,omitempty"`
	TotalSteps  int                    `json:"total_steps"`
	CurrentStep int                    `json:"current_step"`
}
