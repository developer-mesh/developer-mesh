package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// ProtocolAdapter adapts custom protocol messages to MCP format
type ProtocolAdapter struct {
	logger    observability.Logger
	mu        sync.RWMutex
	sessions  map[string]*SessionInfo
	toolCache map[string]*ToolDefinition
}

// SessionInfo tracks session state
type SessionInfo struct {
	ID          string
	TenantID    string
	AgentID     string
	AgentType   string
	Initialized bool
	CreatedAt   time.Time
	LastPing    time.Time
}

// ToolDefinition represents a tool exposed via MCP
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Handler     ToolHandler            `json:"-"`
}

// ToolHandler processes tool calls
type ToolHandler func(ctx context.Context, args map[string]interface{}) (interface{}, error)

// NewProtocolAdapter creates a new protocol adapter
func NewProtocolAdapter(logger observability.Logger) *ProtocolAdapter {
	adapter := &ProtocolAdapter{
		logger:    logger,
		sessions:  make(map[string]*SessionInfo),
		toolCache: make(map[string]*ToolDefinition),
	}

	// Register custom protocol tools as MCP tools
	adapter.registerCustomTools()

	return adapter
}

// registerCustomTools registers custom protocol functionality as MCP tools
func (a *ProtocolAdapter) registerCustomTools() {
	// Agent management tools
	a.RegisterTool(&ToolDefinition{
		Name:        "agent.heartbeat",
		Description: "Send agent heartbeat",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"agent_id":  map[string]string{"type": "string"},
				"timestamp": map[string]string{"type": "number"},
			},
			"required": []string{"agent_id"},
		},
		Handler: a.handleAgentHeartbeat,
	})

	a.RegisterTool(&ToolDefinition{
		Name:        "agent.status",
		Description: "Get agent status",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"agent_id": map[string]string{"type": "string"},
			},
			"required": []string{"agent_id"},
		},
		Handler: a.handleAgentStatus,
	})

	// Workflow management tools
	a.RegisterTool(&ToolDefinition{
		Name:        "workflow.create",
		Description: "Create a new workflow",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":        map[string]string{"type": "string"},
				"description": map[string]string{"type": "string"},
				"steps":       map[string]string{"type": "array"},
			},
			"required": []string{"name", "steps"},
		},
		Handler: a.handleWorkflowCreate,
	})

	a.RegisterTool(&ToolDefinition{
		Name:        "workflow.execute",
		Description: "Execute a workflow",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workflow_id": map[string]string{"type": "string"},
				"input":       map[string]string{"type": "object"},
			},
			"required": []string{"workflow_id"},
		},
		Handler: a.handleWorkflowExecute,
	})

	a.RegisterTool(&ToolDefinition{
		Name:        "workflow.cancel",
		Description: "Cancel a workflow execution",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"execution_id": map[string]string{"type": "string"},
				"reason":       map[string]string{"type": "string"},
			},
			"required": []string{"execution_id"},
		},
		Handler: a.handleWorkflowCancel,
	})

	// Task management tools
	a.RegisterTool(&ToolDefinition{
		Name:        "task.create",
		Description: "Create a new task",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title":       map[string]string{"type": "string"},
				"description": map[string]string{"type": "string"},
				"priority":    map[string]string{"type": "string"},
				"agent_type":  map[string]string{"type": "string"},
			},
			"required": []string{"title"},
		},
		Handler: a.handleTaskCreate,
	})

	a.RegisterTool(&ToolDefinition{
		Name:        "task.assign",
		Description: "Assign a task to an agent",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task_id":  map[string]string{"type": "string"},
				"agent_id": map[string]string{"type": "string"},
			},
			"required": []string{"task_id", "agent_id"},
		},
		Handler: a.handleTaskAssign,
	})

	a.RegisterTool(&ToolDefinition{
		Name:        "task.complete",
		Description: "Mark a task as complete",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task_id": map[string]string{"type": "string"},
				"result":  map[string]string{"type": "object"},
			},
			"required": []string{"task_id"},
		},
		Handler: a.handleTaskComplete,
	})

	// Context management tools
	a.RegisterTool(&ToolDefinition{
		Name:        "context.update",
		Description: "Update context for a session",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]string{"type": "string"},
				"content":    map[string]string{"type": "string"},
				"metadata":   map[string]string{"type": "object"},
			},
			"required": []string{"session_id", "content"},
		},
		Handler: a.handleContextUpdate,
	})

	a.RegisterTool(&ToolDefinition{
		Name:        "context.append",
		Description: "Append to context",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]string{"type": "string"},
				"content":    map[string]string{"type": "string"},
			},
			"required": []string{"session_id", "content"},
		},
		Handler: a.handleContextAppend,
	})
}

// RegisterTool registers a tool handler
func (a *ProtocolAdapter) RegisterTool(tool *ToolDefinition) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.toolCache[tool.Name] = tool
}

// GetTools returns all registered tools in MCP format
func (a *ProtocolAdapter) GetTools() []map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tools := make([]map[string]interface{}, 0, len(a.toolCache))
	for _, tool := range a.toolCache {
		tools = append(tools, map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		})
	}

	return tools
}

// ExecuteTool executes a tool by name
func (a *ProtocolAdapter) ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	a.mu.RLock()
	tool, exists := a.toolCache[name]
	a.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	if tool.Handler == nil {
		return nil, fmt.Errorf("tool handler not implemented: %s", name)
	}

	return tool.Handler(ctx, args)
}

// ConvertCustomToMCP converts custom protocol message to MCP format
func (a *ProtocolAdapter) ConvertCustomToMCP(customMsg map[string]interface{}) (map[string]interface{}, error) {
	msgType, ok := customMsg["type"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid message type")
	}

	// Generate a unique ID for the MCP message
	msgID := uuid.New().String()

	// Map custom protocol to MCP
	switch msgType {
	case "agent.register":
		return a.convertAgentRegister(msgID, customMsg)
	case "agent.heartbeat":
		return a.convertToToolCall(msgID, "agent.heartbeat", customMsg["payload"])
	case "workflow.create":
		return a.convertToToolCall(msgID, "workflow.create", customMsg["payload"])
	case "workflow.execute":
		return a.convertToToolCall(msgID, "workflow.execute", customMsg["payload"])
	case "task.create":
		return a.convertToToolCall(msgID, "task.create", customMsg["payload"])
	case "task.assign":
		return a.convertToToolCall(msgID, "task.assign", customMsg["payload"])
	case "task.complete":
		return a.convertToToolCall(msgID, "task.complete", customMsg["payload"])
	case "context.update":
		return a.convertToToolCall(msgID, "context.update", customMsg["payload"])
	default:
		return nil, fmt.Errorf("unsupported message type: %s", msgType)
	}
}

// convertAgentRegister converts agent registration to MCP initialize
func (a *ProtocolAdapter) convertAgentRegister(msgID string, customMsg map[string]interface{}) (map[string]interface{}, error) {
	payload, ok := customMsg["payload"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid payload")
	}

	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      msgID,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2025-06-18",
			"clientInfo": map[string]interface{}{
				"name":    payload["agent_id"],
				"type":    payload["agent_type"],
				"version": "1.0.0",
			},
		},
	}, nil
}

// convertToToolCall converts a custom message to MCP tool call
func (a *ProtocolAdapter) convertToToolCall(msgID string, toolName string, payload interface{}) (map[string]interface{}, error) {
	args, ok := payload.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid payload for tool call")
	}

	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      msgID,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": args,
		},
	}, nil
}

// Tool handler implementations

func (a *ProtocolAdapter) handleAgentHeartbeat(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	agentID, _ := args["agent_id"].(string)

	a.mu.Lock()
	if session, exists := a.sessions[agentID]; exists {
		session.LastPing = time.Now()
	}
	a.mu.Unlock()

	return map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
	}, nil
}

func (a *ProtocolAdapter) handleAgentStatus(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	agentID, _ := args["agent_id"].(string)

	a.mu.RLock()
	session, exists := a.sessions[agentID]
	a.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}

	return map[string]interface{}{
		"agent_id":   session.AgentID,
		"agent_type": session.AgentType,
		"status":     "online",
		"last_ping":  session.LastPing.Unix(),
		"created_at": session.CreatedAt.Unix(),
	}, nil
}

func (a *ProtocolAdapter) handleWorkflowCreate(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// This would integrate with the workflow engine
	workflowID := uuid.New().String()

	return map[string]interface{}{
		"workflow_id": workflowID,
		"status":      "created",
		"created_at":  time.Now().Unix(),
	}, nil
}

func (a *ProtocolAdapter) handleWorkflowExecute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	workflowID, _ := args["workflow_id"].(string)
	executionID := uuid.New().String()

	return map[string]interface{}{
		"execution_id": executionID,
		"workflow_id":  workflowID,
		"status":       "running",
		"started_at":   time.Now().Unix(),
	}, nil
}

func (a *ProtocolAdapter) handleWorkflowCancel(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	executionID, _ := args["execution_id"].(string)
	reason, _ := args["reason"].(string)

	return map[string]interface{}{
		"execution_id": executionID,
		"status":       "cancelled",
		"reason":       reason,
		"cancelled_at": time.Now().Unix(),
	}, nil
}

func (a *ProtocolAdapter) handleTaskCreate(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	taskID := uuid.New().String()

	return map[string]interface{}{
		"task_id":    taskID,
		"status":     "created",
		"created_at": time.Now().Unix(),
	}, nil
}

func (a *ProtocolAdapter) handleTaskAssign(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	taskID, _ := args["task_id"].(string)
	agentID, _ := args["agent_id"].(string)

	return map[string]interface{}{
		"task_id":     taskID,
		"agent_id":    agentID,
		"status":      "assigned",
		"assigned_at": time.Now().Unix(),
	}, nil
}

func (a *ProtocolAdapter) handleTaskComplete(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	taskID, _ := args["task_id"].(string)
	result := args["result"]

	return map[string]interface{}{
		"task_id":      taskID,
		"status":       "completed",
		"result":       result,
		"completed_at": time.Now().Unix(),
	}, nil
}

func (a *ProtocolAdapter) handleContextUpdate(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessionID, _ := args["session_id"].(string)
	content, _ := args["content"].(string)

	return map[string]interface{}{
		"session_id":  sessionID,
		"status":      "updated",
		"content_len": len(content),
		"updated_at":  time.Now().Unix(),
	}, nil
}

func (a *ProtocolAdapter) handleContextAppend(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessionID, _ := args["session_id"].(string)
	content, _ := args["content"].(string)

	return map[string]interface{}{
		"session_id":   sessionID,
		"status":       "appended",
		"appended_len": len(content),
		"updated_at":   time.Now().Unix(),
	}, nil
}

// InitializeSession initializes an MCP session
func (a *ProtocolAdapter) InitializeSession(connID, tenantID string, clientInfo map[string]interface{}) (*SessionInfo, error) {
	agentID := uuid.New().String()
	if id, ok := clientInfo["name"].(string); ok {
		agentID = id
	}

	agentType := "generic"
	if t, ok := clientInfo["type"].(string); ok {
		agentType = t
	}

	session := &SessionInfo{
		ID:          connID,
		TenantID:    tenantID,
		AgentID:     agentID,
		AgentType:   agentType,
		Initialized: true,
		CreatedAt:   time.Now(),
		LastPing:    time.Now(),
	}

	a.mu.Lock()
	a.sessions[connID] = session
	a.mu.Unlock()

	a.logger.Info("MCP session initialized", map[string]interface{}{
		"connection_id": connID,
		"tenant_id":     tenantID,
		"agent_id":      agentID,
		"agent_type":    agentType,
	})

	return session, nil
}

// GetSession retrieves session information
func (a *ProtocolAdapter) GetSession(connID string) *SessionInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.sessions[connID]
}

// RemoveSession removes a session
func (a *ProtocolAdapter) RemoveSession(connID string) {
	a.mu.Lock()
	delete(a.sessions, connID)
	a.mu.Unlock()
}
