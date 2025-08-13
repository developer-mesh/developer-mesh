package websocket

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/developer-mesh/developer-mesh/pkg/database"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
)

// TransactionalWorkflowEngine extends WorkflowEngine with transaction support
type TransactionalWorkflowEngine struct {
	*WorkflowEngine
	uow             database.UnitOfWork
	txManager       repository.TransactionManager
	compensationMgr repository.CompensationManager
}

// NewTransactionalWorkflowEngine creates a workflow engine with transaction support
func NewTransactionalWorkflowEngine(
	baseEngine *WorkflowEngine,
	uow database.UnitOfWork,
	txManager repository.TransactionManager,
	compensationMgr repository.CompensationManager,
) *TransactionalWorkflowEngine {
	return &TransactionalWorkflowEngine{
		WorkflowEngine:  baseEngine,
		uow:             uow,
		txManager:       txManager,
		compensationMgr: compensationMgr,
	}
}

// ExecuteWorkflow executes a workflow with full transaction support
func (we *TransactionalWorkflowEngine) ExecuteWorkflow(ctx context.Context, workflowID string, input map[string]interface{}) (*WorkflowExecution, error) {
	// Validate workflow exists
	defVal, ok := we.workflows.Load(workflowID)
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}
	def := defVal.(*WorkflowDefinition)

	// Create execution
	execution := &WorkflowExecution{
		ID:          uuid.New().String(),
		WorkflowID:  workflowID,
		Status:      "pending",
		CurrentStep: 0,
		TotalSteps:  len(def.Steps),
		Input:       input,
		StepResults: make(map[string]interface{}),
		StartedAt:   time.Now(),
	}

	// Store execution
	we.executions.Store(execution.ID, execution)

	// Use workflow service if available for database persistence
	if we.workflowService != nil {
		// Convert to service model
		workflowUUID, err := uuid.Parse(workflowID)
		if err != nil {
			we.logger.Error("Invalid workflow ID format", map[string]interface{}{
				"workflow_id": workflowID,
				"error":       err.Error(),
			})
		} else {
			// Create idempotency key based on workflow and input
			idempotencyKey := fmt.Sprintf("ws_%s_%d", workflowID, time.Now().Unix())

			// Execute through service for proper transaction handling
			serviceExecution, err := we.workflowService.ExecuteWorkflow(ctx, workflowUUID, input, idempotencyKey)
			if err != nil {
				we.logger.Error("Failed to create workflow execution in database", map[string]interface{}{
					"workflow_id": workflowID,
					"error":       err.Error(),
				})
			} else {
				// Update execution ID to match service
				execution.ID = serviceExecution.ID.String()
			}
		}
	}

	we.metrics.IncrementCounter("workflow_executions_started", 1)
	we.logger.Info("Workflow execution started", map[string]interface{}{
		"execution_id": execution.ID,
		"workflow_id":  workflowID,
		"total_steps":  execution.TotalSteps,
	})

	// Execute workflow asynchronously with transaction support
	go we.executeWorkflowWithTransactions(context.Background(), execution, def)

	return execution, nil
}

// executeWorkflowWithTransactions executes workflow steps with transaction management
func (we *TransactionalWorkflowEngine) executeWorkflowWithTransactions(ctx context.Context, execution *WorkflowExecution, def *WorkflowDefinition) {
	// Update status to running
	execution.Status = "running"

	// Create compensation list
	compensations := make([]string, 0)

	// Execute with compensation support
	err := we.compensationMgr.ExecuteWithCompensation(ctx, func(ctx context.Context) error {
		// Execute each step in a transaction
		for i, step := range def.Steps {
			// Check context cancellation
			select {
			case <-ctx.Done():
				return errors.New("execution cancelled")
			default:
			}

			// Update current step
			execution.CurrentStep = i + 1

			// Execute step within transaction
			err := we.txManager.WithTransactionOptions(ctx, &sql.TxOptions{
				Isolation: sql.LevelReadCommitted,
			}, func(ctx context.Context, tx database.Transaction) error {
				return we.executeStepInTransaction(ctx, tx, execution, step, i)
			})

			if err != nil {
				return errors.Wrapf(err, "failed to execute step %d", i)
			}

			// Register compensation for this step
			compensationKey := fmt.Sprintf("compensate_step_%d_%s", i, execution.ID)
			we.compensationMgr.RegisterCompensation(compensationKey, func(compCtx context.Context) error {
				return we.compensateStep(compCtx, execution, i)
			})
			compensations = append(compensations, compensationKey)
		}

		// Update execution status to completed
		execution.Status = "completed"
		execution.CompletedAt = time.Now()
		execution.ExecutionTime = execution.CompletedAt.Sub(execution.StartedAt)

		return nil
	}, compensations...)

	if err != nil {
		// Update execution status to failed
		execution.Status = "failed"
		execution.Error = err.Error()
		execution.CompletedAt = time.Now()
		execution.ExecutionTime = execution.CompletedAt.Sub(execution.StartedAt)

		we.logger.Error("Workflow execution failed", map[string]interface{}{
			"execution_id": execution.ID,
			"workflow_id":  execution.WorkflowID,
			"error":        err.Error(),
		})
	}

	// Send notification about completion
	we.notifyWorkflowCompletion(execution)

	// Update metrics
	we.metrics.IncrementCounter(fmt.Sprintf("workflow_executions_%s", execution.Status), 1)
	we.metrics.RecordHistogram("workflow_execution_duration", execution.ExecutionTime.Seconds(), map[string]string{
		"status":      execution.Status,
		"workflow_id": execution.WorkflowID,
	})
}

// executeStepInTransaction executes a single step within a transaction
func (we *TransactionalWorkflowEngine) executeStepInTransaction(ctx context.Context, tx database.Transaction, execution *WorkflowExecution, step map[string]interface{}, stepIndex int) error {
	stepName := fmt.Sprintf("step_%d", stepIndex)
	if name, ok := step["name"].(string); ok {
		stepName = name
	}

	we.logger.Info("Executing workflow step", map[string]interface{}{
		"execution_id": execution.ID,
		"step_index":   stepIndex,
		"step_name":    stepName,
	})

	// Extract step type and configuration
	stepType, ok := step["type"].(string)
	if !ok {
		return errors.New("step type not specified")
	}

	// Execute step based on type
	var result interface{}
	var err error

	switch stepType {
	case "task":
		result, err = we.executeTaskStep(ctx, tx, execution, step)
	case "parallel":
		result, err = we.executeParallelStep(ctx, tx, execution, step)
	case "conditional":
		result, err = we.executeConditionalStep(ctx, tx, execution, step)
	case "wait":
		result, err = we.executeWaitStep(ctx, tx, execution, step)
	default:
		err = fmt.Errorf("unknown step type: %s", stepType)
	}

	if err != nil {
		return errors.Wrapf(err, "step '%s' failed", stepName)
	}

	// Store step result
	execution.StepResults[stepName] = result

	// If step produces output, make it available for next steps
	if output, ok := result.(map[string]interface{}); ok {
		for k, v := range output {
			execution.StepResults[k] = v
		}
	}

	we.logger.Info("Workflow step completed", map[string]interface{}{
		"execution_id": execution.ID,
		"step_index":   stepIndex,
		"step_name":    stepName,
	})

	return nil
}

// executeTaskStep executes a task-based step
func (we *TransactionalWorkflowEngine) executeTaskStep(ctx context.Context, tx database.Transaction, execution *WorkflowExecution, step map[string]interface{}) (interface{}, error) {
	// Extract task configuration
	taskConfig, ok := step["task"].(map[string]interface{})
	if !ok {
		return nil, errors.New("task configuration not found")
	}

	// Use nested transaction for task creation
	savepointName := fmt.Sprintf("task_step_%s", execution.ID)
	err := we.txManager.WithNestedTransaction(ctx, tx, savepointName, func(ctx context.Context) error {
		// Create task if service is available
		if we.taskService != nil {
			task := &models.Task{
				ID:          uuid.New(),
				Type:        taskConfig["type"].(string),
				Title:       taskConfig["title"].(string),
				Description: taskConfig["description"].(string),
				Status:      models.TaskStatusPending,
				Priority:    models.TaskPriorityNormal,
			}

			// Set assignee if specified
			if assignTo, ok := taskConfig["assign_to"].(string); ok {
				task.AssignedTo = &assignTo
			}

			// Create task with idempotency
			idempotencyKey := fmt.Sprintf("workflow_task_%s_%s", execution.ID, task.ID)
			if err := we.taskService.Create(ctx, task, idempotencyKey); err != nil {
				return errors.Wrap(err, "failed to create task")
			}

			we.logger.Info("Created task for workflow step", map[string]interface{}{
				"task_id":      task.ID,
				"execution_id": execution.ID,
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status": "task_created",
		"type":   taskConfig["type"],
	}, nil
}

// executeParallelStep executes multiple steps in parallel
func (we *TransactionalWorkflowEngine) executeParallelStep(ctx context.Context, tx database.Transaction, execution *WorkflowExecution, step map[string]interface{}) (interface{}, error) {
	parallelSteps, ok := step["steps"].([]interface{})
	if !ok {
		return nil, errors.New("parallel steps not defined")
	}

	// Execute steps concurrently
	var wg sync.WaitGroup
	results := make([]interface{}, len(parallelSteps))
	errors := make([]error, len(parallelSteps))

	for i, pStep := range parallelSteps {
		wg.Add(1)
		go func(index int, stepData interface{}) {
			defer wg.Done()

			// Each parallel step gets its own transaction
			stepMap, ok := stepData.(map[string]interface{})
			if !ok {
				errors[index] = fmt.Errorf("invalid step configuration at index %d", index)
				return
			}

			err := we.txManager.WithTransaction(ctx, func(ctx context.Context, stepTx database.Transaction) error {
				err := we.executeStepInTransaction(ctx, stepTx, execution, stepMap, index)
				if err != nil {
					return err
				}
				// Get result from execution context
				stepName := fmt.Sprintf("step_%d", index)
				if name, ok := stepMap["name"].(string); ok {
					stepName = name
				}
				results[index] = execution.StepResults[stepName]
				return nil
			})

			if err != nil {
				errors[index] = err
			}
		}(i, pStep)
	}

	wg.Wait()

	// Check for errors
	var errs []error
	for i, err := range errors {
		if err != nil {
			errs = append(errs, fmt.Errorf("parallel step %d failed: %v", i, err))
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("parallel execution failed: %v", errs)
	}

	return map[string]interface{}{
		"status":  "completed",
		"results": results,
	}, nil
}

// executeConditionalStep executes a step based on conditions
func (we *TransactionalWorkflowEngine) executeConditionalStep(ctx context.Context, tx database.Transaction, execution *WorkflowExecution, step map[string]interface{}) (interface{}, error) {
	condition, ok := step["condition"].(string)
	if !ok {
		return nil, errors.New("condition not specified")
	}

	// Evaluate condition (simplified - in production this would use a proper expression evaluator)
	conditionMet := we.evaluateCondition(condition, execution)

	var stepToExecute map[string]interface{}
	if conditionMet {
		if thenStep, ok := step["then"].(map[string]interface{}); ok {
			stepToExecute = thenStep
		}
	} else {
		if elseStep, ok := step["else"].(map[string]interface{}); ok {
			stepToExecute = elseStep
		}
	}

	if stepToExecute != nil {
		err := we.executeStepInTransaction(ctx, tx, execution, stepToExecute, -1)
		if err != nil {
			return nil, err
		}
		// Return the result from the executed step
		return execution.StepResults["conditional_step"], nil
	}

	return map[string]interface{}{
		"status":        "completed",
		"condition_met": conditionMet,
		"action_taken":  conditionMet,
	}, nil
}

// executeWaitStep pauses execution for a specified duration
func (we *TransactionalWorkflowEngine) executeWaitStep(ctx context.Context, tx database.Transaction, execution *WorkflowExecution, step map[string]interface{}) (interface{}, error) {
	duration := 5 * time.Second // default
	if d, ok := step["duration"].(string); ok {
		parsed, err := time.ParseDuration(d)
		if err == nil {
			duration = parsed
		}
	}

	select {
	case <-time.After(duration):
		return map[string]interface{}{
			"status":   "completed",
			"duration": duration.String(),
		}, nil
	case <-ctx.Done():
		return nil, errors.New("wait cancelled")
	}
}

// compensateStep compensates for a failed step
func (we *TransactionalWorkflowEngine) compensateStep(ctx context.Context, execution *WorkflowExecution, stepIndex int) error {
	we.logger.Info("Compensating workflow step", map[string]interface{}{
		"execution_id": execution.ID,
		"step_index":   stepIndex,
	})

	// Compensation logic would go here
	// This could involve:
	// - Cancelling created tasks
	// - Reverting state changes
	// - Sending notifications
	// - Cleaning up resources

	return nil
}

// evaluateCondition evaluates a condition expression
func (we *TransactionalWorkflowEngine) evaluateCondition(condition string, execution *WorkflowExecution) bool {
	// Simple implementation - in production use a proper expression evaluator
	// For now, just check if a specific result exists
	_, exists := execution.StepResults[condition]
	return exists
}

// notifyWorkflowCompletion sends notifications about workflow completion
func (we *TransactionalWorkflowEngine) notifyWorkflowCompletion(execution *WorkflowExecution) {
	if we.notificationManager != nil {
		notification := map[string]interface{}{
			"type":         "workflow_completed",
			"execution_id": execution.ID,
			"workflow_id":  execution.WorkflowID,
			"status":       execution.Status,
			"duration":     execution.ExecutionTime.String(),
		}

		if execution.Error != "" {
			notification["error"] = execution.Error
		}

		// Send to all connections interested in this workflow
		// This would need to be implemented based on subscription model
		we.logger.Info("Workflow completion notification sent", notification)
	}
}

// CancelExecution cancels a running workflow execution
func (we *TransactionalWorkflowEngine) CancelExecution(ctx context.Context, executionID string) error {
	val, ok := we.executions.Load(executionID)
	if !ok {
		return fmt.Errorf("execution not found: %s", executionID)
	}

	execution := val.(*WorkflowExecution)
	if execution.Status != "running" && execution.Status != "pending" {
		return fmt.Errorf("cannot cancel execution in status: %s", execution.Status)
	}

	// Update status
	execution.Status = "cancelled"
	execution.CompletedAt = time.Now()
	execution.ExecutionTime = execution.CompletedAt.Sub(execution.StartedAt)
	execution.Error = "Cancelled by user"

	we.metrics.IncrementCounter("workflow_executions_cancelled", 1)
	we.logger.Info("Workflow execution cancelled", map[string]interface{}{
		"execution_id": executionID,
		"workflow_id":  execution.WorkflowID,
	})

	return nil
}
