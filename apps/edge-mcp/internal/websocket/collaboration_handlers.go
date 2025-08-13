package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/collaboration"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/google/uuid"
)

// Multi-agent collaboration handlers

// handleTaskCreateAutoAssign creates a task and automatically assigns it based on agent capabilities
func (s *Server) handleTaskCreateAutoAssign(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var createParams struct {
		Task struct {
			ID                   string                 `json:"id"`
			Title                string                 `json:"title"`
			Description          string                 `json:"description"`
			Type                 string                 `json:"type"`
			Priority             string                 `json:"priority"`
			RequiredCapabilities []string               `json:"required_capabilities"`
			Parameters           map[string]interface{} `json:"parameters"`
		} `json:"task"`
		AssignmentStrategy string `json:"assignment_strategy"` // capability_match, load_balance, round_robin
	}

	if err := json.Unmarshal(params, &createParams); err != nil {
		return nil, err
	}

	// Find agent with matching capabilities
	var bestAgent string
	if s.agentRegistry != nil && len(createParams.Task.RequiredCapabilities) > 0 {
		s.logger.Info("Looking for agents with capabilities", map[string]interface{}{
			"required_capabilities": createParams.Task.RequiredCapabilities,
			"tenant_id":             conn.TenantID,
			"requesting_agent":      conn.AgentID,
		})

		// Use DiscoverAgents to find agents with required capabilities
		// Don't exclude self - an agent should be able to assign tasks to itself if it has the capabilities
		agents, err := s.agentRegistry.DiscoverAgents(
			ctx,
			conn.TenantID,
			createParams.Task.RequiredCapabilities,
			false, // include self - agent can assign to itself if it has the capabilities
			conn.AgentID,
		)

		s.logger.Info("DiscoverAgents result", map[string]interface{}{
			"agent_count": len(agents),
			"error":       err,
			"agents":      agents,
		})
		if err == nil && len(agents) > 0 {
			// Select best agent based on assignment strategy
			switch createParams.AssignmentStrategy {
			case "load_balance":
				// Pick agent with least active tasks
				minTasks := int(^uint(0) >> 1) // Max int
				for _, agent := range agents {
					if activeTasks, ok := agent["active_tasks"].(int); ok && activeTasks < minTasks {
						if agentID, ok := agent["id"].(string); ok {
							bestAgent = agentID
							minTasks = activeTasks
						}
					}
				}
			case "round_robin":
				// For round robin, we'd need to track last assigned agent
				// For now, pick randomly from available agents
				if len(agents) > 0 {
					// Use task ID as seed for consistent assignment
					idx := 0
					if createParams.Task.ID != "" {
						// Simple hash of task ID to get index
						for _, c := range createParams.Task.ID {
							idx += int(c)
						}
						idx = idx % len(agents)
					}
					if agentID, ok := agents[idx]["id"].(string); ok {
						bestAgent = agentID
					}
				}
			default: // capability_match or fallback
				// Pick agent with most matching capabilities (best fit)
				maxCapScore := 0
				for _, agent := range agents {
					if caps, ok := agent["capabilities"].([]string); ok {
						// Count how many capabilities beyond required ones
						score := len(caps)
						if score > maxCapScore {
							if agentID, ok := agent["id"].(string); ok {
								bestAgent = agentID
								maxCapScore = score
							}
						}
					}
				}
				// If no scoring worked, fall back to first available
				if bestAgent == "" && len(agents) > 0 {
					if agentID, ok := agents[0]["id"].(string); ok {
						bestAgent = agentID
					}
				}
			}
		}
	}

	// If no agent found with capabilities, return error
	if bestAgent == "" {
		return nil, fmt.Errorf("no agent found with required capabilities: %v", createParams.Task.RequiredCapabilities)
	}

	// Check if taskService is available
	if s.taskService == nil {
		s.logger.Error("Task service not initialized", map[string]interface{}{
			"method": "handleTaskCreateAutoAssign",
		})
		return nil, fmt.Errorf("task service not initialized")
	}

	// Parse task ID if provided, otherwise generate new one
	var taskUUID uuid.UUID
	if createParams.Task.ID != "" {
		var err error
		taskUUID, err = uuid.Parse(createParams.Task.ID)
		if err != nil {
			taskUUID = uuid.New()
		}
	} else {
		taskUUID = uuid.New()
	}

	// Parse tenant ID
	tenantUUID, err := uuid.Parse(conn.TenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}

	// Convert priority string to TaskPriority
	priority := models.TaskPriorityNormal
	switch createParams.Task.Priority {
	case "low":
		priority = models.TaskPriorityLow
	case "high":
		priority = models.TaskPriorityHigh
	case "critical":
		priority = models.TaskPriorityCritical
	}

	// Create task in database
	task := &models.Task{
		ID:          taskUUID,
		Type:        createParams.Task.Type,
		Title:       createParams.Task.Title,
		Description: createParams.Task.Description,
		Parameters:  models.JSONMap(createParams.Task.Parameters),
		Priority:    priority,
		Status:      models.TaskStatusPending,
		CreatedBy:   conn.AgentID,
		TenantID:    tenantUUID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Version:     1,
	}

	// Add required capabilities to tags
	if len(createParams.Task.RequiredCapabilities) > 0 {
		task.Tags = createParams.Task.RequiredCapabilities
	}

	// Create task with idempotency key
	idempotencyKey := fmt.Sprintf("task-create-auto-%s", task.ID.String())
	s.logger.Info("Creating task in database", map[string]interface{}{
		"task_id":     task.ID.String(),
		"type":        task.Type,
		"title":       task.Title,
		"assigned_to": bestAgent,
	})
	err = s.taskService.Create(ctx, task, idempotencyKey)
	if err != nil {
		s.logger.Error("Failed to create task", map[string]interface{}{
			"task_id": task.ID.String(),
			"error":   err.Error(),
		})
		return nil, fmt.Errorf("failed to create task: %w", err)
	}
	s.logger.Info("Task created successfully", map[string]interface{}{
		"task_id": task.ID.String(),
	})

	// Auto-assign the task
	err = s.taskService.AssignTask(ctx, task.ID, bestAgent)
	if err != nil {
		return nil, fmt.Errorf("failed to assign task: %w", err)
	}

	// Send assignment notification to the selected agent
	s.mu.RLock()
	for _, c := range s.connections {
		if c.AgentID == bestAgent {
			notification := map[string]interface{}{
				"task_id":      task.ID.String(),
				"title":        task.Title,
				"capabilities": createParams.Task.RequiredCapabilities,
				"assigned_at":  time.Now().Format(time.RFC3339),
			}

			// Send directly as a notification
			msg := struct {
				Type   int                    `json:"type"`
				Method string                 `json:"method"`
				Params map[string]interface{} `json:"params"`
			}{
				Type:   2, // MessageTypeNotification
				Method: "task.assigned",
				Params: notification,
			}

			msgBytes, _ := json.Marshal(msg)
			c.send <- msgBytes
			break
		}
	}
	s.mu.RUnlock()

	return map[string]interface{}{
		"task_id":     task.ID.String(),
		"assigned_to": bestAgent,
		"status":      string(models.TaskStatusAssigned),
		"created_at":  task.CreatedAt.Format(time.RFC3339),
	}, nil
}

// handleTaskCreateDistributed creates a distributed task that can be split across agents
func (s *Server) handleTaskCreateDistributed(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	startTime := time.Now()
	defer func() {
		// Record operation latency
		if s.metricsCollector != nil {
			s.metricsCollector.RecordMessage("collaboration", "task.create.distributed", conn.TenantID, time.Since(startTime))
		}
	}()

	var createParams struct {
		Title                string                   `json:"title"`
		Description          string                   `json:"description"`
		Type                 string                   `json:"type"`
		Priority             string                   `json:"priority"`
		Parameters           map[string]interface{}   `json:"parameters"`
		Subtasks             []map[string]interface{} `json:"subtasks"`
		Strategy             string                   `json:"strategy"` // parallel, sequential, pipeline
		TargetAgents         []string                 `json:"target_agents"`
		RequiredCapabilities []string                 `json:"required_capabilities"`
		MaxRetries           int                      `json:"max_retries"`
		TimeoutSeconds       int                      `json:"timeout_seconds"`
	}

	if err := json.Unmarshal(params, &createParams); err != nil {
		return nil, err
	}

	// Use task service if available
	if s.taskService != nil {
		// Create main task
		task := &models.Task{
			ID:             uuid.New(),
			TenantID:       conn.GetTenantUUID(),
			Type:           createParams.Type,
			Status:         models.TaskStatusPending,
			Priority:       models.TaskPriority(createParams.Priority),
			CreatedBy:      conn.AgentID,
			Title:          createParams.Title,
			Description:    createParams.Description,
			Parameters:     models.JSONMap(createParams.Parameters),
			MaxRetries:     createParams.MaxRetries,
			TimeoutSeconds: createParams.TimeoutSeconds,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		// Store distribution strategy in parameters
		task.Parameters["distribution_strategy"] = createParams.Strategy
		task.Parameters["target_agents"] = createParams.TargetAgents
		task.Parameters["required_capabilities"] = createParams.RequiredCapabilities

		// Create task first
		if err := s.taskService.Create(ctx, task, uuid.New().String()); err != nil {
			return nil, fmt.Errorf("failed to create distributed task: %w", err)
		}

		// Create subtasks
		subtaskIDs := []string{}
		for _, st := range createParams.Subtasks {
			subtask := &models.Task{
				ID:             uuid.New(),
				TenantID:       conn.GetTenantUUID(),
				Type:           st["type"].(string),
				Status:         models.TaskStatusPending,
				Priority:       models.TaskPriority(st["priority"].(string)),
				Title:          st["title"].(string),
				Parameters:     models.JSONMap(st["parameters"].(map[string]interface{})),
				MaxRetries:     createParams.MaxRetries,
				TimeoutSeconds: createParams.TimeoutSeconds,
				ParentTaskID:   &task.ID,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}
			if err := s.taskService.Create(ctx, subtask, uuid.New().String()); err != nil {
				// Log error but continue
				s.logger.Error("Failed to create subtask", map[string]interface{}{
					"error":      err.Error(),
					"parent_id":  task.ID,
					"subtask_id": subtask.ID,
				})
				continue
			}
			subtaskIDs = append(subtaskIDs, subtask.ID.String())

			// Subscribe to subtask notifications
			if s.notificationManager != nil {
				s.notificationManager.Subscribe(conn.ID, fmt.Sprintf("task:%s", subtask.ID))
			}
		}

		// Subscribe to main task notifications
		if s.notificationManager != nil {
			s.notificationManager.Subscribe(conn.ID, fmt.Sprintf("task:%s", task.ID))
		}

		return map[string]interface{}{
			"task_id":       task.ID.String(),
			"type":          task.Type,
			"status":        task.Status,
			"priority":      task.Priority,
			"subtask_count": len(subtaskIDs),
			"subtask_ids":   subtaskIDs,
			"strategy":      createParams.Strategy,
			"target_agents": createParams.TargetAgents,
			"created_at":    task.CreatedAt.Format(time.RFC3339),
		}, nil
	}

	// Service not initialized
	return nil, fmt.Errorf("task service not initialized")
}

// handleTaskDelegate delegates a task to another agent
func (s *Server) handleTaskDelegate(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	startTime := time.Now()
	defer func() {
		// Record operation latency
		if s.metricsCollector != nil {
			s.metricsCollector.RecordMessage("collaboration", "task.delegate", conn.TenantID, time.Since(startTime))
		}
	}()

	var delegateParams struct {
		TaskID         string                 `json:"task_id"`
		ToAgentID      string                 `json:"to_agent_id"`
		Reason         string                 `json:"reason"`
		DelegationType string                 `json:"delegation_type"` // manual, automatic, failover, load_balance
		Metadata       map[string]interface{} `json:"metadata"`
	}

	if err := json.Unmarshal(params, &delegateParams); err != nil {
		return nil, err
	}

	taskID, err := uuid.Parse(delegateParams.TaskID)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID: %w", err)
	}

	if s.taskService != nil {
		// Get task first to ensure it exists and get its CreatedAt for partitioned FK
		task, err := s.taskService.Get(ctx, taskID)
		if err != nil {
			return nil, fmt.Errorf("failed to get task: %w", err)
		}

		// Verify the current agent owns the task or has permission to delegate
		if task.AssignedTo != nil && *task.AssignedTo != conn.AgentID && task.CreatedBy != conn.AgentID {
			return nil, fmt.Errorf("agent %s is not authorized to delegate task %s", conn.AgentID, taskID)
		}

		// Parse delegation type
		var delegationType models.DelegationType
		switch delegateParams.DelegationType {
		case "automatic":
			delegationType = models.DelegationAutomatic
		case "failover":
			delegationType = models.DelegationFailover
		case "load_balance":
			delegationType = models.DelegationLoadBalance
		default:
			delegationType = models.DelegationManual
		}

		delegation := &models.TaskDelegation{
			ID:             uuid.New(),
			TaskID:         taskID,
			TaskCreatedAt:  task.CreatedAt, // Required for partitioned table FK
			FromAgentID:    conn.AgentID,
			ToAgentID:      delegateParams.ToAgentID,
			Reason:         delegateParams.Reason,
			DelegationType: delegationType,
			Metadata:       models.JSONMap(delegateParams.Metadata),
			DelegatedAt:    time.Now(),
		}

		if err := s.taskService.DelegateTask(ctx, delegation); err != nil {
			return nil, fmt.Errorf("failed to delegate task: %w", err)
		}

		// Record delegation metrics
		if s.metricsCollector != nil {
			s.metricsCollector.RecordTaskDelegated(conn.TenantID, conn.AgentID, delegateParams.ToAgentID, delegateParams.DelegationType)
		}

		// Notify target agent
		if s.notificationManager != nil {
			notification := map[string]interface{}{
				"type":         "task.delegated",
				"task_id":      taskID.String(),
				"from_agent":   conn.AgentID,
				"to_agent":     delegateParams.ToAgentID,
				"reason":       delegateParams.Reason,
				"delegated_at": delegation.DelegatedAt.Format(time.RFC3339),
			}
			if err := s.notificationManager.NotifyAgent(ctx, delegateParams.ToAgentID, notification); err != nil {
				s.logger.Warn("Failed to notify agent of task delegation", map[string]interface{}{
					"agent_id": delegateParams.ToAgentID,
					"task_id":  taskID.String(),
					"error":    err.Error(),
				})
			}
		}

		return map[string]interface{}{
			"delegation_id":   delegation.ID.String(),
			"task_id":         taskID.String(),
			"from_agent":      conn.AgentID,
			"to_agent":        delegateParams.ToAgentID,
			"delegation_type": delegateParams.DelegationType,
			"delegated_at":    delegation.DelegatedAt.Format(time.RFC3339),
		}, nil
	}

	// Service not initialized
	return nil, fmt.Errorf("task service not initialized")
}

// handleTaskAccept accepts a delegated task
func (s *Server) handleTaskAccept(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var acceptParams struct {
		TaskID   string `json:"task_id"`
		Comments string `json:"comments"`
	}

	if err := json.Unmarshal(params, &acceptParams); err != nil {
		return nil, err
	}

	taskID, err := uuid.Parse(acceptParams.TaskID)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID: %w", err)
	}

	if s.taskService != nil {
		if err := s.taskService.AcceptTask(ctx, taskID, conn.AgentID); err != nil {
			return nil, fmt.Errorf("failed to accept task: %w", err)
		}

		// Get task details
		task, err := s.taskService.Get(ctx, taskID)
		if err != nil {
			return nil, fmt.Errorf("failed to get task: %w", err)
		}

		// Notify original creator
		if s.notificationManager != nil {
			notification := map[string]interface{}{
				"type":        "task.accepted",
				"task_id":     taskID.String(),
				"accepted_by": conn.AgentID,
				"accepted_at": time.Now().Format(time.RFC3339),
			}
			if err := s.notificationManager.NotifyAgent(ctx, task.CreatedBy, notification); err != nil {
				s.logger.Warn("Failed to notify task creator of acceptance", map[string]interface{}{
					"creator_id": task.CreatedBy,
					"task_id":    taskID.String(),
					"error":      err.Error(),
				})
			}
		}

		return map[string]interface{}{
			"task_id":     taskID.String(),
			"status":      task.Status,
			"accepted_by": conn.AgentID,
			"accepted_at": time.Now().Format(time.RFC3339),
		}, nil
	}

	// Service not initialized
	return nil, fmt.Errorf("task service not initialized")
}

// handleTaskComplete marks a task as completed
func (s *Server) handleTaskComplete(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	startTime := time.Now()
	defer func() {
		// Record operation latency
		if s.metricsCollector != nil {
			s.metricsCollector.RecordMessage("collaboration", "task.complete", conn.TenantID, time.Since(startTime))
		}
	}()

	var completeParams struct {
		TaskID string                 `json:"task_id"`
		Result map[string]interface{} `json:"result"`
	}

	if err := json.Unmarshal(params, &completeParams); err != nil {
		return nil, err
	}

	taskID, err := uuid.Parse(completeParams.TaskID)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID: %w", err)
	}

	if s.taskService != nil {
		if err := s.taskService.CompleteTask(ctx, taskID, conn.AgentID, completeParams.Result); err != nil {
			return nil, fmt.Errorf("failed to complete task: %w", err)
		}

		// Get task details
		task, err := s.taskService.Get(ctx, taskID)
		if err != nil {
			return nil, fmt.Errorf("failed to get task: %w", err)
		}

		// Record task completion metrics
		if s.metricsCollector != nil && task.StartedAt != nil {
			duration := time.Since(*task.StartedAt)
			s.metricsCollector.RecordTaskCompleted(conn.TenantID, conn.AgentID, "completed", duration)
		}

		// Notify task creator and parent task owner
		if s.notificationManager != nil {
			notification := map[string]interface{}{
				"type":         "task.completed",
				"task_id":      taskID.String(),
				"completed_by": conn.AgentID,
				"result":       completeParams.Result,
				"completed_at": time.Now().Format(time.RFC3339),
			}
			if err := s.notificationManager.NotifyAgent(ctx, task.CreatedBy, notification); err != nil {
				s.logger.Warn("Failed to notify task creator of completion", map[string]interface{}{
					"creator_id": task.CreatedBy,
					"task_id":    taskID.String(),
					"error":      err.Error(),
				})
			}

			// If this is a subtask, notify parent task owner
			if task.ParentTaskID != nil {
				s.notificationManager.BroadcastNotification(ctx, fmt.Sprintf("task:%s", task.ParentTaskID.String()), "task.subtask_completed", notification)
			}
		}

		return map[string]interface{}{
			"task_id":      taskID.String(),
			"status":       task.Status,
			"completed_by": conn.AgentID,
			"completed_at": time.Now().Format(time.RFC3339),
		}, nil
	}

	// Service not initialized
	return nil, fmt.Errorf("task service not initialized")
}

// handleTaskFail marks a task as failed
func (s *Server) handleTaskFail(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var failParams struct {
		TaskID string `json:"task_id"`
		Error  string `json:"error"`
		Retry  bool   `json:"retry"`
	}

	if err := json.Unmarshal(params, &failParams); err != nil {
		return nil, err
	}

	taskID, err := uuid.Parse(failParams.TaskID)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID: %w", err)
	}

	if s.taskService != nil {
		if err := s.taskService.FailTask(ctx, taskID, conn.AgentID, failParams.Error); err != nil {
			return nil, fmt.Errorf("failed to fail task: %w", err)
		}

		// Get task details
		task, err := s.taskService.Get(ctx, taskID)
		if err != nil {
			return nil, fmt.Errorf("failed to get task: %w", err)
		}

		// Notify task creator
		if s.notificationManager != nil {
			notification := map[string]interface{}{
				"type":      "task.failed",
				"task_id":   taskID.String(),
				"failed_by": conn.AgentID,
				"error":     failParams.Error,
				"retry":     failParams.Retry,
				"failed_at": time.Now().Format(time.RFC3339),
			}
			if err := s.notificationManager.NotifyAgent(ctx, task.CreatedBy, notification); err != nil {
				s.logger.Warn("Failed to notify task creator of failure", map[string]interface{}{
					"creator_id": task.CreatedBy,
					"task_id":    taskID.String(),
					"error":      err.Error(),
				})
			}
		}

		return map[string]interface{}{
			"task_id":       taskID.String(),
			"status":        task.Status,
			"error":         failParams.Error,
			"retry_enabled": failParams.Retry,
			"failed_at":     time.Now().Format(time.RFC3339),
		}, nil
	}

	// Service not initialized
	return nil, fmt.Errorf("task service not initialized")
}

// handleTaskSubmitResult submits partial results for a long-running task
func (s *Server) handleTaskSubmitResult(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var submitParams struct {
		TaskID   string                 `json:"task_id"`
		Result   map[string]interface{} `json:"result"`
		Progress float64                `json:"progress"` // 0-100
		Message  string                 `json:"message"`
	}

	if err := json.Unmarshal(params, &submitParams); err != nil {
		return nil, err
	}

	taskID, err := uuid.Parse(submitParams.TaskID)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID: %w", err)
	}

	// Notify subscribers about progress
	if s.notificationManager != nil {
		notification := map[string]interface{}{
			"type":           "task.progress",
			"task_id":        taskID.String(),
			"progress":       submitParams.Progress,
			"message":        submitParams.Message,
			"partial_result": submitParams.Result,
			"reported_by":    conn.AgentID,
			"timestamp":      time.Now().Format(time.RFC3339),
		}
		s.notificationManager.BroadcastNotification(ctx, fmt.Sprintf("task:%s", taskID.String()), "task.progress", notification)
	}

	return map[string]interface{}{
		"task_id":      taskID.String(),
		"progress":     submitParams.Progress,
		"submitted_at": time.Now().Format(time.RFC3339),
	}, nil
}

// handleWorkflowCreateCollaborative creates a collaborative workflow
func (s *Server) handleWorkflowCreateCollaborative(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	// Add user context for authorization
	ctx = auth.WithUserID(ctx, conn.AgentID)
	ctx = auth.WithTenantID(ctx, conn.GetTenantUUID())

	startTime := time.Now()
	defer func() {
		// Record operation latency
		if s.metricsCollector != nil {
			s.metricsCollector.RecordMessage("collaboration", "workflow.create", conn.TenantID, time.Since(startTime))
		}
	}()

	var workflowParams struct {
		Name             string                   `json:"name"`
		Description      string                   `json:"description"`
		Steps            []map[string]interface{} `json:"steps"`
		Agents           []string                 `json:"agents"`
		CoordinationMode string                   `json:"coordination_mode"` // centralized, distributed, consensus
		DecisionStrategy string                   `json:"decision_strategy"` // majority, unanimous, weighted
		Strategy         string                   `json:"strategy"`          // sequential, parallel, conditional, collaborative
		TimeoutSeconds   int                      `json:"timeout_seconds"`
		MaxRetries       int                      `json:"max_retries"`
	}

	if err := json.Unmarshal(params, &workflowParams); err != nil {
		return nil, err
	}

	// Check if workflowService is available
	if s.workflowService == nil {
		return nil, fmt.Errorf("workflow service not initialized")
	}

	{
		// Convert agents array to object with agent IDs as keys
		agentsMap := make(models.JSONMap)

		// If no agents specified, use the creating agent as default
		if len(workflowParams.Agents) == 0 {
			// Add the creating agent as a participant
			agentsMap[conn.AgentID] = map[string]interface{}{
				"id":   conn.AgentID,
				"role": "participant",
			}
		} else {
			for _, agentID := range workflowParams.Agents {
				agentsMap[agentID] = map[string]interface{}{
					"id":   agentID,
					"role": "participant",
				}
			}
		}

		// Determine workflow type from strategy parameter
		workflowType := models.WorkflowTypeCollaborative
		if workflowParams.Strategy != "" {
			switch workflowParams.Strategy {
			case "sequential":
				workflowType = models.WorkflowTypeSequential
			case "parallel":
				workflowType = models.WorkflowTypeParallel
			case "conditional":
				workflowType = models.WorkflowTypeConditional
			case "collaborative":
				workflowType = models.WorkflowTypeCollaborative
			default:
				// Keep default as collaborative
			}
		}

		workflow := &models.Workflow{
			ID:          uuid.New(),
			TenantID:    conn.GetTenantUUID(),
			Name:        workflowParams.Name,
			Description: workflowParams.Description,
			Type:        workflowType,
			IsActive:    true,
			Agents:      agentsMap, // Initialize Agents as JSONMap object
			Config: models.JSONMap{
				"coordination_mode": workflowParams.CoordinationMode,
				"decision_strategy": workflowParams.DecisionStrategy,
				"agents":            workflowParams.Agents,
				"timeout_seconds":   workflowParams.TimeoutSeconds,
				"max_retries":       workflowParams.MaxRetries,
				"strategy":          workflowParams.Strategy,
			},
			Steps:     models.WorkflowSteps{}, // Initialize Steps array
			CreatedBy: conn.AgentID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Convert steps to WorkflowStep objects
		steps := make(models.WorkflowSteps, 0, len(workflowParams.Steps))
		s.logger.Info("Processing workflow steps", map[string]interface{}{
			"step_count": len(workflowParams.Steps),
			"steps":      workflowParams.Steps,
		})
		for i, stepData := range workflowParams.Steps {
			// Use provided ID or generate new one
			stepID := ""
			if id, ok := stepData["id"].(string); ok {
				stepID = id
			} else {
				stepID = uuid.New().String()
			}

			// Safely extract required fields with defaults
			name, ok := stepData["name"].(string)
			if !ok || name == "" {
				s.logger.Error("Step missing required name field", map[string]interface{}{
					"step_index": i,
					"step_data":  stepData,
				})
				return nil, fmt.Errorf("step %d missing required name field", i)
			}

			// Type can default based on agent_capability if not provided
			stepType, ok := stepData["type"].(string)
			if !ok || stepType == "" {
				// Check if agent_capability is provided (for E2E test compatibility)
				if agentCap, ok := stepData["agent_capability"].(string); ok {
					stepType = agentCap
				} else {
					stepType = "task" // Default type
				}
			}

			// Config can be empty map if not provided
			config, ok := stepData["config"].(map[string]interface{})
			if !ok {
				config = make(map[string]interface{})
				// If timeout is provided at step level, add it to config
				if timeout, ok := stepData["timeout"].(float64); ok {
					config["timeout"] = int(timeout)
				}
			}

			// Extract action or default to execute
			action, ok := stepData["action"].(string)
			if !ok || action == "" {
				action = "execute" // Default action
			}

			// Extract input or default to empty map
			input, ok := stepData["input"].(map[string]interface{})
			if !ok {
				input = make(map[string]interface{})
			}

			// Extract agent_id if provided
			agentID, _ := stepData["agent_id"].(string)

			step := models.WorkflowStep{
				ID:          stepID,
				Name:        name,
				Type:        stepType,
				Action:      action,
				AgentID:     agentID,
				Input:       input,
				Config:      config,
				Description: "", // Optional field
			}

			// Parse dependencies
			if deps, ok := stepData["dependencies"].([]interface{}); ok {
				dependencies := make([]string, 0, len(deps))
				for _, dep := range deps {
					if depStr, ok := dep.(string); ok {
						dependencies = append(dependencies, depStr)
					}
				}
				step.Dependencies = dependencies
			}

			// Note: The existing WorkflowStep struct doesn't have AssignedAgents field
			// Store it in the Config for now
			if agents, ok := stepData["agents"].([]interface{}); ok {
				assignedAgents := make([]string, 0, len(agents))
				for _, agent := range agents {
					if agentStr, ok := agent.(string); ok {
						assignedAgents = append(assignedAgents, agentStr)
					}
				}
				if step.Config == nil {
					step.Config = make(map[string]interface{})
				}
				step.Config["assigned_agents"] = assignedAgents
			}

			// Store order in config as well since WorkflowStep doesn't have Order field
			if step.Config == nil {
				step.Config = make(map[string]interface{})
			}
			step.Config["order"] = i + 1

			steps = append(steps, step)
		}

		// Assign steps directly - it will be serialized as a JSONB array
		workflow.Steps = steps
		s.logger.Info("Storing steps in workflow", map[string]interface{}{
			"steps_count": len(steps),
			"steps":       steps,
		})

		// Debug: verify steps are stored
		s.logger.Info("Steps stored successfully", map[string]interface{}{
			"stored_count": len(workflow.Steps),
		})

		// Log complete workflow object before creation
		s.logger.Info("Creating workflow with data", map[string]interface{}{
			"workflow_id":  workflow.ID,
			"tenant_id":    workflow.TenantID,
			"name":         workflow.Name,
			"type":         workflow.Type,
			"agents_count": len(workflow.Agents),
			"agents":       workflow.Agents,
			"steps_count":  len(workflow.Steps),
			"config":       workflow.Config,
			"created_by":   workflow.CreatedBy,
		})

		if err := s.workflowService.CreateWorkflow(ctx, workflow); err != nil {
			s.logger.Error("Failed to create workflow", map[string]interface{}{
				"error":         err.Error(),
				"workflow_name": workflow.Name,
				"workflow_type": workflow.Type,
			})
			return nil, fmt.Errorf("failed to create collaborative workflow: %w", err)
		}

		// Record workflow creation metrics
		if s.metricsCollector != nil {
			s.metricsCollector.RecordWorkflowStarted(conn.TenantID, "collaborative")
		}

		// Subscribe all participating agents
		if s.notificationManager != nil {
			// If no agents were specified, subscribe the creating agent
			if len(workflowParams.Agents) == 0 {
				s.notificationManager.Subscribe(conn.AgentID, fmt.Sprintf("workflow:%s", workflow.ID))
			} else {
				for _, agentID := range workflowParams.Agents {
					s.notificationManager.Subscribe(agentID, fmt.Sprintf("workflow:%s", workflow.ID))
				}
			}
		}

		return map[string]interface{}{
			"workflow_id":       workflow.ID.String(),
			"name":              workflow.Name,
			"type":              workflow.Type,
			"is_active":         workflow.IsActive,
			"coordination_mode": workflowParams.CoordinationMode,
			"agents":            workflowParams.Agents,
			"step_count":        len(workflow.Steps),
			"created_at":        workflow.CreatedAt.Format(time.RFC3339),
		}, nil
	}
}

// handleWorkflowExecuteCollaborative executes a collaborative workflow
func (s *Server) handleWorkflowExecuteCollaborative(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	// Add user context for authorization
	ctx = auth.WithUserID(ctx, conn.AgentID)
	ctx = auth.WithTenantID(ctx, conn.GetTenantUUID())

	var execParams struct {
		WorkflowID string                 `json:"workflow_id"`
		Input      map[string]interface{} `json:"input"`
		Context    map[string]interface{} `json:"context"`
		Stream     bool                   `json:"stream"`
	}

	if err := json.Unmarshal(params, &execParams); err != nil {
		return nil, err
	}

	workflowID, err := uuid.Parse(execParams.WorkflowID)
	if err != nil {
		return nil, fmt.Errorf("invalid workflow ID: %w", err)
	}

	// Check if workflowService is available
	if s.workflowService == nil {
		return nil, fmt.Errorf("workflow service not initialized")
	}

	{
		// Prepare context for workflow execution
		executionContext := models.JSONMap(execParams.Context)
		if executionContext == nil {
			executionContext = make(models.JSONMap)
		}
		executionContext["input"] = execParams.Input

		// Execute workflow - this creates the execution
		execution, err := s.workflowService.ExecuteWorkflow(ctx, workflowID, executionContext, uuid.New().String())
		if err != nil {
			return nil, fmt.Errorf("failed to execute collaborative workflow: %w", err)
		}

		// Subscribe to execution notifications if streaming
		if execParams.Stream && s.notificationManager != nil {
			s.notificationManager.Subscribe(conn.ID, fmt.Sprintf("execution:%s", execution.ID))
		}

		return map[string]interface{}{
			"execution_id": execution.ID.String(),
			"workflow_id":  workflowID.String(),
			"status":       execution.Status,
			"initiated_by": conn.AgentID,
			"started_at":   execution.StartedAt.Format(time.RFC3339),
			"streaming":    execParams.Stream,
		}, nil
	}
}

// handleWorkflowGet retrieves workflow details
func (s *Server) handleWorkflowGet(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	// Add user context for authorization
	ctx = auth.WithUserID(ctx, conn.AgentID)
	ctx = auth.WithTenantID(ctx, conn.GetTenantUUID())

	var getParams struct {
		WorkflowID string `json:"workflow_id"`
	}

	if err := json.Unmarshal(params, &getParams); err != nil {
		return nil, err
	}

	workflowID, err := uuid.Parse(getParams.WorkflowID)
	if err != nil {
		return nil, fmt.Errorf("invalid workflow ID: %w", err)
	}

	// Check if workflowService is available
	if s.workflowService == nil {
		return nil, fmt.Errorf("workflow service not initialized")
	}

	{
		workflow, err := s.workflowService.GetWorkflow(ctx, workflowID)
		if err != nil {
			return nil, fmt.Errorf("failed to get workflow: %w", err)
		}

		// Convert steps to response format
		steps := make([]map[string]interface{}, 0, len(workflow.Steps))
		for _, step := range workflow.Steps {
			stepMap := map[string]interface{}{
				"id":           step.ID,
				"name":         step.Name,
				"type":         step.Type,
				"config":       step.Config,
				"dependencies": step.Dependencies,
			}
			if step.Description != "" {
				stepMap["description"] = step.Description
			}
			steps = append(steps, stepMap)
		}

		return map[string]interface{}{
			"workflow_id": workflow.ID.String(),
			"name":        workflow.Name,
			"description": workflow.Description,
			"type":        workflow.Type,
			"is_active":   workflow.IsActive,
			"config":      workflow.Config,
			"steps":       steps,
			"created_by":  workflow.CreatedBy,
			"created_at":  workflow.CreatedAt.Format(time.RFC3339),
			"updated_at":  workflow.UpdatedAt.Format(time.RFC3339),
		}, nil
	}
}

// handleWorkflowResume resumes a paused workflow execution
func (s *Server) handleWorkflowResume(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	// Add user context for authorization
	ctx = auth.WithUserID(ctx, conn.AgentID)
	ctx = auth.WithTenantID(ctx, conn.GetTenantUUID())

	var resumeParams struct {
		ExecutionID string                 `json:"execution_id"`
		Input       map[string]interface{} `json:"input"`
		Force       bool                   `json:"force"`
	}

	if err := json.Unmarshal(params, &resumeParams); err != nil {
		return nil, err
	}

	executionID, err := uuid.Parse(resumeParams.ExecutionID)
	if err != nil {
		return nil, fmt.Errorf("invalid execution ID: %w", err)
	}

	// Check if workflowService is available
	if s.workflowService == nil {
		return nil, fmt.Errorf("workflow service not initialized")
	}

	{
		if err := s.workflowService.ResumeExecution(ctx, executionID); err != nil {
			return nil, fmt.Errorf("failed to resume workflow: %w", err)
		}

		// Get execution details
		execution, err := s.workflowService.GetExecution(ctx, executionID)
		if err != nil {
			return nil, fmt.Errorf("failed to resume workflow: %w", err)
		}

		return map[string]interface{}{
			"execution_id": execution.ID.String(),
			"workflow_id":  execution.WorkflowID.String(),
			"status":       execution.Status,
			"resumed_at":   time.Now().Format(time.RFC3339),
			"resumed_by":   conn.AgentID,
		}, nil
	}
}

// handleWorkflowCompleteTask completes a specific task in a workflow
func (s *Server) handleWorkflowCompleteTask(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	// Add user context for authorization
	ctx = auth.WithUserID(ctx, conn.AgentID)
	ctx = auth.WithTenantID(ctx, conn.GetTenantUUID())

	var completeParams struct {
		ExecutionID string                 `json:"execution_id"`
		StepID      string                 `json:"step_id"`
		Result      map[string]interface{} `json:"result"`
		NextStep    string                 `json:"next_step"` // For conditional workflows
	}

	if err := json.Unmarshal(params, &completeParams); err != nil {
		return nil, err
	}

	executionID, err := uuid.Parse(completeParams.ExecutionID)
	if err != nil {
		return nil, fmt.Errorf("invalid execution ID: %w", err)
	}

	// Check if workflowService is available
	if s.workflowService == nil {
		return nil, fmt.Errorf("workflow service not initialized")
	}

	{
		// Get execution and update step status
		execution, err := s.workflowService.GetExecution(ctx, executionID)
		if err != nil {
			return nil, fmt.Errorf("failed to get execution: %w", err)
		}

		// Update step status
		if execution.StepStatuses == nil {
			execution.StepStatuses = make(map[string]*models.StepStatus)
		}

		now := time.Now()
		if stepStatus, exists := execution.StepStatuses[completeParams.StepID]; exists {
			stepStatus.Status = "completed"
			stepStatus.CompletedAt = &now
			stepStatus.Output = models.JSONMap{"result": completeParams.Result}
		} else {
			execution.StepStatuses[completeParams.StepID] = &models.StepStatus{
				StepID:      completeParams.StepID,
				Status:      "completed",
				StartedAt:   &now,
				CompletedAt: &now,
				Output:      models.JSONMap{"result": completeParams.Result},
			}
		}

		// Update execution
		if err := s.workflowService.UpdateExecution(ctx, execution); err != nil {
			return nil, fmt.Errorf("failed to update workflow execution: %w", err)
		}

		// Notify other agents in the workflow
		if s.notificationManager != nil {
			notification := map[string]interface{}{
				"type":         "workflow.step_completed",
				"execution_id": executionID.String(),
				"step_id":      completeParams.StepID,
				"completed_by": conn.AgentID,
				"result":       completeParams.Result,
				"next_step":    completeParams.NextStep,
				"timestamp":    time.Now().Format(time.RFC3339),
			}
			s.notificationManager.BroadcastNotification(ctx, fmt.Sprintf("execution:%s", executionID), "workflow.step_completed", notification)
		}

		return map[string]interface{}{
			"execution_id": executionID.String(),
			"step_id":      completeParams.StepID,
			"completed_by": conn.AgentID,
			"completed_at": time.Now().Format(time.RFC3339),
		}, nil
	}
}

// handleAgentUpdateStatus updates an agent's status
func (s *Server) handleAgentUpdateStatus(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var statusParams struct {
		Status       string                 `json:"status"` // available, busy, offline
		Capabilities []string               `json:"capabilities"`
		Metadata     map[string]interface{} `json:"metadata"`
		Health       map[string]interface{} `json:"health"`
	}

	if err := json.Unmarshal(params, &statusParams); err != nil {
		return nil, err
	}

	// Update agent status in registry
	if s.agentRegistry != nil {
		if err := s.agentRegistry.UpdateAgentStatus(ctx, conn.AgentID, statusParams.Status, statusParams.Metadata); err != nil {
			return nil, fmt.Errorf("failed to update agent status: %w", err)
		}

		// Note: Capabilities would need to be updated separately through re-registration
		// as UpdateAgentStatus only updates status and metadata
	}

	// Broadcast status update to interested parties
	if s.notificationManager != nil {
		notification := map[string]interface{}{
			"type":         "agent.status_changed",
			"agent_id":     conn.AgentID,
			"status":       statusParams.Status,
			"capabilities": statusParams.Capabilities,
			"health":       statusParams.Health,
			"timestamp":    time.Now().Format(time.RFC3339),
		}
		s.notificationManager.BroadcastNotification(ctx, "agent.status", "agent.status_changed", notification)
	}

	return map[string]interface{}{
		"agent_id":   conn.AgentID,
		"status":     statusParams.Status,
		"updated_at": time.Now().Format(time.RFC3339),
	}, nil
}

// handleDocumentCreateShared creates a shared document for collaboration
func (s *Server) handleDocumentCreateShared(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var docParams struct {
		Title       string                 `json:"title"`
		Type        string                 `json:"type"` // text, code, diagram, data
		Content     string                 `json:"content"`
		Permissions map[string]interface{} `json:"permissions"`
		Workspace   string                 `json:"workspace"`
		WorkspaceID string                 `json:"workspace_id"`
	}

	if err := json.Unmarshal(params, &docParams); err != nil {
		return nil, err
	}

	// Check if documentService is available
	if s.documentService == nil {
		return nil, fmt.Errorf("document service not initialized")
	}

	// Support both "workspace" and "workspace_id" fields
	workspaceStr := docParams.Workspace
	if workspaceStr == "" {
		workspaceStr = docParams.WorkspaceID
	}
	workspaceID, err := uuid.Parse(workspaceStr)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace ID: %w", err)
	}

	doc := &models.Document{
		ID:          uuid.New(),
		TenantID:    conn.GetTenantUUID(),
		WorkspaceID: &workspaceID,
		Title:       docParams.Title,
		Type:        docParams.Type,
		Content:     docParams.Content,
		CreatedBy:   conn.AgentID,
		Version:     1,
		Permissions: models.JSONMap(docParams.Permissions),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Convert to SharedDocument for the service
	sharedDoc := &models.SharedDocument{
		ID:          doc.ID,
		TenantID:    doc.TenantID,
		WorkspaceID: *doc.WorkspaceID,
		Title:       doc.Title,
		Type:        doc.Type,
		Content:     doc.Content,
		CreatedBy:   doc.CreatedBy,
		Metadata:    doc.Permissions, // Store permissions in metadata
		CreatedAt:   doc.CreatedAt,
		UpdatedAt:   doc.UpdatedAt,
		Version:     1,
	}

	// Create document using the service
	if err := s.documentService.Create(ctx, sharedDoc); err != nil {
		return nil, fmt.Errorf("failed to create document: %w", err)
	}

	// Notify workspace members (excluding the creator)
	if s.workspaceManager != nil && workspaceStr != "" {
		// Use the workspace manager's broadcast method with exclusion
		s.workspaceManager.broadcastEvent(workspaceStr, "document_created", map[string]interface{}{
			"document_id":   doc.ID.String(),
			"title":         doc.Title,
			"doc_type":      doc.Type,
			"created_by":    conn.AgentID,
			"exclude_agent": conn.AgentID, // Production pattern: exclude creator
		})
	}

	return map[string]interface{}{
		"document_id":  doc.ID.String(),
		"title":        doc.Title,
		"type":         doc.Type,
		"version":      doc.Version,
		"created_by":   doc.CreatedBy,
		"workspace_id": workspaceStr,
		"created_at":   doc.CreatedAt.Format(time.RFC3339),
	}, nil
}

// handleDocumentUpdate updates a shared document
func (s *Server) handleDocumentUpdate(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var updateParams struct {
		DocumentID string                 `json:"document_id"`
		Content    string                 `json:"content"`
		Metadata   map[string]interface{} `json:"metadata"`
	}

	if err := json.Unmarshal(params, &updateParams); err != nil {
		return nil, err
	}

	docID, err := uuid.Parse(updateParams.DocumentID)
	if err != nil {
		return nil, fmt.Errorf("invalid document ID: %w", err)
	}

	// Check if documentService is available
	if s.documentService == nil {
		return nil, fmt.Errorf("document service not initialized")
	}

	// Get the existing document first
	doc, err := s.documentService.Get(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	// Update the content
	doc.Content = updateParams.Content
	doc.UpdatedAt = time.Now()

	// Update document using the service
	if err := s.documentService.Update(ctx, doc); err != nil {
		return nil, fmt.Errorf("failed to update document: %w", err)
	}

	// Notify collaborators
	if s.notificationManager != nil {
		notification := map[string]interface{}{
			"type":        "document.updated",
			"document_id": docID.String(),
			"updated_by":  conn.AgentID,
			"version":     doc.Version,
			"timestamp":   doc.UpdatedAt.Format(time.RFC3339),
		}
		s.notificationManager.BroadcastNotification(ctx, fmt.Sprintf("document:%s", docID), "document.updated", notification)
	}

	return map[string]interface{}{
		"document_id": docID.String(),
		"version":     doc.Version,
		"updated_by":  conn.AgentID,
		"updated_at":  doc.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// handleDocumentApplyChange applies a change to a document (for CRDT)
func (s *Server) handleDocumentApplyChange(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var changeParams struct {
		DocumentID string                 `json:"document_id"`
		ChangeType string                 `json:"change_type"` // insert, delete, format
		Position   int                    `json:"position"`
		Content    string                 `json:"content"`
		Length     int                    `json:"length"`
		Metadata   map[string]interface{} `json:"metadata"`
	}

	if err := json.Unmarshal(params, &changeParams); err != nil {
		return nil, err
	}

	docID, err := uuid.Parse(changeParams.DocumentID)
	if err != nil {
		return nil, fmt.Errorf("invalid document ID: %w", err)
	}

	// Check if documentService is available
	if s.documentService == nil {
		return nil, fmt.Errorf("document service not initialized")
	}

	// Create a document operation for collaborative editing
	// Map the change parameters to the DocumentOperation structure
	operationData := map[string]interface{}{
		"position": changeParams.Position,
		"content":  changeParams.Content,
		"length":   changeParams.Length,
		"metadata": changeParams.Metadata,
	}

	operation := &collaboration.DocumentOperation{
		ID:         uuid.New(),
		DocumentID: docID,
		AgentID:    conn.AgentID,
		Type:       changeParams.ChangeType,
		Value:      operationData,
		AppliedAt:  time.Now(),
	}

	if err := s.documentService.ApplyOperation(ctx, docID, operation); err != nil {
		return nil, fmt.Errorf("failed to apply document operation: %w", err)
	}

	// Broadcast change to all collaborators for real-time sync
	if s.notificationManager != nil {
		notification := map[string]interface{}{
			"type":        "document.change",
			"document_id": docID.String(),
			"change_id":   operation.ID.String(),
			"change_type": operation.Type,
			"position":    changeParams.Position,
			"content":     changeParams.Content,
			"length":      changeParams.Length,
			"agent_id":    conn.AgentID,
			"timestamp":   operation.AppliedAt.Format(time.RFC3339),
		}
		s.notificationManager.BroadcastNotification(ctx, fmt.Sprintf("document:%s", docID), "document.change", notification)
	}

	return map[string]interface{}{
		"document_id": docID.String(),
		"change_id":   operation.ID.String(),
		"applied_at":  operation.AppliedAt.Format(time.RFC3339),
		"applied_by":  conn.AgentID,
	}, nil
}

// handleWorkspaceGetState retrieves the current state of a workspace
func (s *Server) handleWorkspaceGetState(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var stateParams struct {
		WorkspaceID string `json:"workspace_id"`
	}

	if err := json.Unmarshal(params, &stateParams); err != nil {
		return nil, err
	}

	workspaceID, err := uuid.Parse(stateParams.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace ID: %w", err)
	}

	// Check if workspaceService is available
	if s.workspaceService == nil {
		return nil, fmt.Errorf("workspace service not initialized")
	}

	// Get workspace state
	workspace, err := s.workspaceService.Get(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace: %w", err)
	}

	return map[string]interface{}{
		"workspace_id":   workspace.ID.String(),
		"name":           workspace.Name,
		"type":           workspace.Type,
		"state":          workspace.Metadata, // Assuming state is stored in metadata
		"active_members": len(workspace.Members),
		"documents":      0, // Would need to query documents separately
		"updated_at":     workspace.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// handleWorkspaceUpdateState updates the shared state of a workspace
func (s *Server) handleWorkspaceUpdateState(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var updateParams struct {
		WorkspaceID string                 `json:"workspace_id"`
		State       map[string]interface{} `json:"state"`
		Merge       bool                   `json:"merge"` // Merge with existing state or replace
	}

	if err := json.Unmarshal(params, &updateParams); err != nil {
		return nil, err
	}

	workspaceID, err := uuid.Parse(updateParams.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace ID: %w", err)
	}

	if s.workspaceService != nil {
		// Create state operation
		var opType string
		if updateParams.Merge {
			opType = "merge"
		} else {
			opType = "set"
		}

		stateOp := &models.StateOperation{
			Type:  opType,
			Path:  "/",
			Value: models.JSONMap(updateParams.State),
		}

		if err := s.workspaceService.UpdateState(ctx, workspaceID, stateOp); err != nil {
			return nil, fmt.Errorf("failed to update workspace state: %w", err)
		}

		// Notify workspace members of state change
		if s.notificationManager != nil {
			notification := map[string]interface{}{
				"type":         "workspace.state_changed",
				"workspace_id": workspaceID.String(),
				"updated_by":   conn.AgentID,
				"state":        updateParams.State,
				"timestamp":    time.Now().Format(time.RFC3339),
			}
			s.notificationManager.BroadcastNotification(ctx, fmt.Sprintf("workspace:%s", workspaceID), "workspace.state_changed", notification)
		}

		return map[string]interface{}{
			"workspace_id": workspaceID.String(),
			"state":        updateParams.State,
			"updated_by":   conn.AgentID,
			"updated_at":   time.Now().Format(time.RFC3339),
			"version":      1, // TODO: Implement proper versioning
		}, nil
	}

	// Service not initialized
	return nil, fmt.Errorf("workspace service not initialized")
}
