package builtin

import "github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"

// EnhancedToolDefinition extends the base tool definition with Anthropic patterns
type EnhancedToolDefinition struct {
	tools.ToolDefinition
	Metadata ToolMetadata `json:"metadata"`
}

// GetEnhancedAgentTools returns agent tools with enhanced metadata
func GetEnhancedAgentTools() []EnhancedToolDefinition {
	return []EnhancedToolDefinition{
		{
			ToolDefinition: tools.ToolDefinition{
				Name:        "agent_heartbeat",
				Description: "Send agent heartbeat to maintain connection status. Agents are marked offline after 5 minutes without heartbeat.",
			},
			Metadata: ToolMetadata{
				NextTools: []string{"agent_status", "agent_list", "task_assign"},
				RateLimit: &RateLimitInfo{
					RequestsPerMinute: 600,
					BurstSize:         20,
					Description:       "High frequency allowed for maintaining agent liveness",
				},
				Limits: map[string]interface{}{
					"timeout_minutes":       5,
					"max_agents_per_tenant": 1000,
				},
				Examples: []Example{
					{
						Description: "Send online heartbeat for agent",
						Input: map[string]interface{}{
							"agent_id": "agent-123",
							"status":   "online",
						},
					},
				},
			},
		},
		{
			ToolDefinition: tools.ToolDefinition{
				Name:        "agent_list",
				Description: "List all registered agents with pagination and filtering. Returns online/offline status based on heartbeat recency.",
			},
			Metadata: ToolMetadata{
				NextTools: []string{"agent_status", "task_assign", "agent_heartbeat"},
				RateLimit: &RateLimitInfo{
					RequestsPerMinute: 100,
					BurstSize:         10,
					Description:       "Standard rate limit for list operations",
				},
				Limits: map[string]interface{}{
					"max_limit":     100,
					"default_limit": 50,
					"max_offset":    10000,
				},
				Examples: []Example{
					{
						Description: "List online agents with pagination",
						Input: map[string]interface{}{
							"status": "online",
							"limit":  10,
							"offset": 0,
						},
					},
				},
				AdvancedParams: []string{"sort_by", "sort_order"},
			},
		},
	}
}

// GetEnhancedWorkflowTools returns workflow tools with enhanced metadata
func GetEnhancedWorkflowTools() []EnhancedToolDefinition {
	return []EnhancedToolDefinition{
		{
			ToolDefinition: tools.ToolDefinition{
				Name:        "workflow_create",
				Description: "Create a new workflow definition. Maximum 100 steps per workflow.",
			},
			Metadata: ToolMetadata{
				NextTools: []string{"workflow_execute", "workflow_get", "workflow_list"},
				RateLimit: &RateLimitInfo{
					RequestsPerMinute: 60,
					BurstSize:         10,
					Description:       "Moderate limit for workflow creation",
				},
				Limits: map[string]interface{}{
					"max_steps":       100,
					"max_name_length": 255,
					"max_description": 1000,
				},
				Examples: []Example{
					{
						Description: "Create deployment workflow",
						Input: map[string]interface{}{
							"name":        "deploy-service",
							"description": "Deploy microservice to production",
							"steps": []map[string]interface{}{
								{
									"name": "build",
									"type": "action",
								},
								{
									"name":       "test",
									"type":       "action",
									"depends_on": []string{"build"},
								},
							},
						},
					},
				},
			},
		},
		{
			ToolDefinition: tools.ToolDefinition{
				Name:        "workflow_execute",
				Description: "Execute a workflow. Currently simulates execution with 100ms delay.",
			},
			Metadata: ToolMetadata{
				NextTools: []string{"workflow_execution_get", "workflow_execution_list", "workflow_cancel"},
				RateLimit: &RateLimitInfo{
					RequestsPerMinute: 60,
					BurstSize:         10,
					Description:       "Controlled rate to prevent workflow flooding",
				},
				Limits: map[string]interface{}{
					"simulation_delay_ms": 100,
					"max_concurrent":      10,
				},
				Examples: []Example{
					{
						Description: "Execute workflow with input parameters",
						Input: map[string]interface{}{
							"workflow_id": "workflow-123",
							"input": map[string]interface{}{
								"environment": "production",
								"version":     "1.2.3",
							},
						},
					},
				},
			},
		},
	}
}

// GetEnhancedTaskTools returns task tools with enhanced metadata
func GetEnhancedTaskTools() []EnhancedToolDefinition {
	return []EnhancedToolDefinition{
		{
			ToolDefinition: tools.ToolDefinition{
				Name:        "task_create",
				Description: "Create a new task with specified type and priority.",
			},
			Metadata: ToolMetadata{
				NextTools: []string{"task_assign", "task_get", "task_list"},
				RateLimit: &RateLimitInfo{
					RequestsPerMinute: 120,
					BurstSize:         20,
					Description:       "Standard limit for task creation",
				},
				Limits: map[string]interface{}{
					"max_title_length":       255,
					"max_description_length": 2000,
				},
				Examples: []Example{
					{
						Description: "Create high-priority deployment task",
						Input: map[string]interface{}{
							"title":       "Deploy v2.0 to production",
							"type":        "deployment",
							"priority":    "high",
							"description": "Deploy new version with database migrations",
						},
					},
				},
			},
		},
		{
			ToolDefinition: tools.ToolDefinition{
				Name:        "task_get_batch",
				Description: "Get details of multiple tasks in a single operation. Maximum 50 tasks per request.",
			},
			Metadata: ToolMetadata{
				NextTools: []string{"task_assign", "task_complete", "task_list"},
				RateLimit: &RateLimitInfo{
					RequestsPerMinute: 30,
					BurstSize:         5,
					Description:       "Lower limit for batch operations to prevent overload",
				},
				Limits: map[string]interface{}{
					"max_tasks_per_request": 50,
				},
				Examples: []Example{
					{
						Description: "Get multiple tasks by ID",
						Input: map[string]interface{}{
							"task_ids": []string{"task-1", "task-2", "task-3"},
						},
					},
				},
			},
		},
		{
			ToolDefinition: tools.ToolDefinition{
				Name:        "task_complete",
				Description: "Mark a task as completed or failed with optional result data.",
			},
			Metadata: ToolMetadata{
				NextTools: []string{"task_list", "task_create", "workflow_execute"},
				RateLimit: &RateLimitInfo{
					RequestsPerMinute: 100,
					BurstSize:         10,
					Description:       "Standard rate limit",
				},
				Limits: map[string]interface{}{
					"max_result_size_kb": 100,
				},
				Examples: []Example{
					{
						Description: "Complete task with success result",
						Input: map[string]interface{}{
							"task_id": "task-123",
							"status":  "completed",
							"result": map[string]interface{}{
								"deployment_url": "https://app.example.com",
								"version":        "2.0.1",
							},
						},
					},
				},
			},
		},
	}
}

// GetEnhancedContextTools returns context tools with enhanced metadata
func GetEnhancedContextTools() []EnhancedToolDefinition {
	return []EnhancedToolDefinition{
		{
			ToolDefinition: tools.ToolDefinition{
				Name:        "context_update",
				Description: "Update session context with merge options. Use 'merge: true' for partial updates.",
			},
			Metadata: ToolMetadata{
				NextTools: []string{"context_get", "context_append", "workflow_execute"},
				RateLimit: &RateLimitInfo{
					RequestsPerMinute: 300,
					BurstSize:         50,
					Description:       "High limit for context management",
				},
				Limits: map[string]interface{}{
					"max_context_size_kb": 1000,
					"max_key_length":      100,
					"max_depth":           10,
				},
				Examples: []Example{
					{
						Description: "Update context with merge",
						Input: map[string]interface{}{
							"session_id": "session-123",
							"context": map[string]interface{}{
								"user_id": "user-456",
								"preferences": map[string]interface{}{
									"theme": "dark",
								},
							},
							"merge": true,
						},
					},
				},
				AdvancedParams: []string{"merge"},
			},
		},
		{
			ToolDefinition: tools.ToolDefinition{
				Name:        "context_append",
				Description: "Append values to context arrays or create new entries. Handles type coercion automatically.",
			},
			Metadata: ToolMetadata{
				NextTools: []string{"context_get", "context_list"},
				RateLimit: &RateLimitInfo{
					RequestsPerMinute: 300,
					BurstSize:         50,
					Description:       "High limit for context management",
				},
				Limits: map[string]interface{}{
					"max_array_size":    1000,
					"max_value_size_kb": 100,
				},
				Examples: []Example{
					{
						Description: "Append to existing array",
						Input: map[string]interface{}{
							"session_id": "session-123",
							"key":        "events",
							"value": map[string]interface{}{
								"type":      "click",
								"timestamp": "2024-01-20T10:00:00Z",
							},
						},
					},
				},
			},
		},
	}
}
