package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/developer-mesh/developer-mesh/pkg/adapters/mcp"
	"github.com/developer-mesh/developer-mesh/pkg/adapters/mcp/resources"
	"github.com/developer-mesh/developer-mesh/pkg/clients"
	"github.com/developer-mesh/developer-mesh/pkg/feature"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// MCPMessage represents a JSON-RPC 2.0 message for the Model Context Protocol
type MCPMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC 2.0 error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCP Error codes as per JSON-RPC 2.0 specification
const (
	MCPErrorParseError     = -32700
	MCPErrorInvalidRequest = -32600
	MCPErrorMethodNotFound = -32601
	MCPErrorInvalidParams  = -32602
	MCPErrorInternalError  = -32603
)

// MCPSession represents an active MCP session
type MCPSession struct {
	ID        string
	TenantID  string
	AgentID   string
	CreatedAt time.Time
}

// MCPProtocolHandler handles MCP protocol messages
type MCPProtocolHandler struct {
	restAPIClient    clients.RESTAPIClient
	sessions         map[string]*MCPSession
	sessionsMu       sync.RWMutex
	logger           observability.Logger
	protocolAdapter  *mcp.ProtocolAdapter
	resourceProvider *resources.ResourceProvider
	// Performance optimizations
	toolsCache      *ToolsCache
	toolNameCache   map[string]map[string]string // tenant_id -> tool_name -> tool_id
	toolNameCacheMu sync.RWMutex
	metrics         observability.MetricsClient
	telemetry       *MCPTelemetry
	// Resilience
	circuitBreakers *ToolCircuitBreakerManager
}

// NewMCPProtocolHandler creates a new MCP protocol handler
func NewMCPProtocolHandler(
	restClient clients.RESTAPIClient,
	logger observability.Logger,
) *MCPProtocolHandler {
	return &MCPProtocolHandler{
		restAPIClient:    restClient,
		sessions:         make(map[string]*MCPSession),
		logger:           logger,
		protocolAdapter:  mcp.NewProtocolAdapter(logger),
		resourceProvider: resources.NewResourceProvider(logger),
		toolsCache:       NewToolsCache(5 * time.Minute), // 5 minute TTL
		toolNameCache:    make(map[string]map[string]string),
		telemetry:        NewMCPTelemetry(logger),
		circuitBreakers:  NewToolCircuitBreakerManager(logger),
	}
}

// SetMetricsClient sets the metrics client for telemetry
func (h *MCPProtocolHandler) SetMetricsClient(metrics observability.MetricsClient) {
	h.metrics = metrics
	if h.telemetry != nil {
		h.telemetry.SetMetricsClient(metrics)
	}
}

// resolveToolNameToID resolves a tool display name to its UUID for a specific tenant
// This ensures tenant isolation - a tenant can only access their own tools
func (h *MCPProtocolHandler) resolveToolNameToID(ctx context.Context, tenantID, toolName string) (string, error) {
	// Check cache first
	h.toolNameCacheMu.RLock()
	if tenantCache, exists := h.toolNameCache[tenantID]; exists {
		if toolID, found := tenantCache[toolName]; found {
			h.toolNameCacheMu.RUnlock()
			h.logger.Debug("Tool name resolved from cache", map[string]interface{}{
				"tenant_id": tenantID,
				"tool_name": toolName,
				"tool_id":   toolID,
			})
			return toolID, nil
		}
	}
	h.toolNameCacheMu.RUnlock()

	// Not in cache, fetch all tools for this tenant
	tools, err := h.restAPIClient.ListTools(ctx, tenantID)
	if err != nil {
		return "", fmt.Errorf("failed to list tools for tenant: %w", err)
	}

	// Build the cache for this tenant
	h.toolNameCacheMu.Lock()
	if h.toolNameCache[tenantID] == nil {
		h.toolNameCache[tenantID] = make(map[string]string)
	}

	var foundID string
	for _, tool := range tools {
		// Use display name if available, otherwise tool name
		name := tool.DisplayName
		if name == "" {
			name = tool.ToolName
		}
		h.toolNameCache[tenantID][name] = tool.ID

		// Check if this is the tool we're looking for
		if name == toolName {
			foundID = tool.ID
		}
	}
	h.toolNameCacheMu.Unlock()

	if foundID == "" {
		return "", fmt.Errorf("tool '%s' not found for tenant '%s'", toolName, tenantID)
	}

	h.logger.Info("Tool name resolved and cached", map[string]interface{}{
		"tenant_id":          tenantID,
		"tool_name":          toolName,
		"tool_id":            foundID,
		"total_tools_cached": len(tools),
	})

	return foundID, nil
}

// HandleMessage processes an MCP protocol message
func (h *MCPProtocolHandler) HandleMessage(conn *websocket.Conn, connID string, tenantID string, message []byte) error {
	startTime := time.Now()

	var msg MCPMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		h.logger.Error("Failed to parse MCP message", map[string]interface{}{
			"error":         err.Error(),
			"connection_id": connID,
		})
		h.recordTelemetry("parse_error", time.Since(startTime), false)
		return h.sendError(conn, nil, MCPErrorParseError, "Parse error")
	}

	h.logger.Debug("Handling MCP method", map[string]interface{}{
		"method":        msg.Method,
		"id":            msg.ID,
		"connection_id": connID,
	})

	// Route to appropriate handler based on method
	switch msg.Method {
	// Core MCP methods
	case "initialize":
		return h.handleInitialize(conn, connID, tenantID, msg)
	case "initialized":
		// Client confirmation after initialize - just acknowledge
		return h.sendResult(conn, msg.ID, map[string]interface{}{"status": "ok"})
	case "ping":
		return h.handlePing(conn, connID, tenantID, msg)
	case "shutdown":
		return h.handleShutdown(conn, connID, tenantID, msg)
	case "cancel", "$/cancelRequest":
		return h.handleCancelRequest(conn, connID, tenantID, msg)

	// Tools methods
	case "tools/list":
		return h.handleToolsList(conn, connID, tenantID, msg)
	case "tools/call":
		return h.handleToolCall(conn, connID, tenantID, msg)

	// Resources methods
	case "resources/list":
		return h.handleResourcesList(conn, connID, tenantID, msg)
	case "resources/read":
		return h.handleResourceRead(conn, connID, tenantID, msg)
	case "resources/subscribe":
		return h.handleResourceSubscribe(conn, connID, tenantID, msg)
	case "resources/unsubscribe":
		return h.handleResourceUnsubscribe(conn, connID, tenantID, msg)

	// Prompts methods
	case "prompts/list":
		return h.handlePromptsList(conn, connID, tenantID, msg)
	case "prompts/get":
		return h.handlePromptGet(conn, connID, tenantID, msg)
	case "prompts/run":
		return h.handlePromptRun(conn, connID, tenantID, msg)

	// Completion methods
	case "completion/complete":
		return h.handleCompletionComplete(conn, connID, tenantID, msg)
	case "sampling/createMessage":
		return h.handleSamplingCreateMessage(conn, connID, tenantID, msg)

	// Logging methods
	case "logging/setLevel":
		return h.handleLoggingSetLevel(conn, connID, tenantID, msg)

	// Custom DevMesh extensions (x- prefix for extensions)
	case "x-devmesh/agent/register":
		return h.handleAgentRegister(conn, connID, tenantID, msg)
	case "x-devmesh/agent/health":
		return h.handleAgentHealth(conn, connID, tenantID, msg)
	case "x-devmesh/context/update":
		return h.handleContextUpdate(conn, connID, tenantID, msg)
	case "x-devmesh/search/semantic":
		return h.handleSemanticSearch(conn, connID, tenantID, msg)
	case "x-devmesh/tools/batch":
		return h.handleToolsBatch(conn, connID, tenantID, msg)

	default:
		return h.sendError(conn, msg.ID, MCPErrorMethodNotFound, fmt.Sprintf("Method not found: %s", msg.Method))
	}
}

// handleInitialize handles the MCP initialize request
func (h *MCPProtocolHandler) handleInitialize(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	// Parse initialize params
	var params struct {
		ProtocolVersion string                 `json:"protocolVersion"`
		ClientInfo      map[string]interface{} `json:"clientInfo"`
	}
	if msg.Params != nil {
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			h.logger.Warn("Failed to parse initialize params", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// Initialize session in protocol adapter
	adapterSession, err := h.protocolAdapter.InitializeSession(connID, tenantID, params.ClientInfo)
	if err != nil {
		h.logger.Error("Failed to initialize adapter session", map[string]interface{}{
			"error":         err.Error(),
			"connection_id": connID,
		})
		return h.sendError(conn, msg.ID, MCPErrorInternalError, "Failed to initialize session")
	}

	// Create or update session
	h.sessionsMu.Lock()
	session := &MCPSession{
		ID:        connID,
		TenantID:  tenantID,
		AgentID:   adapterSession.AgentID,
		CreatedAt: time.Now(),
	}
	h.sessions[connID] = session
	h.sessionsMu.Unlock()

	h.logger.Info("MCP session initialized", map[string]interface{}{
		"connection_id":    connID,
		"tenant_id":        tenantID,
		"agent_id":         adapterSession.AgentID,
		"agent_type":       adapterSession.AgentType,
		"protocol_version": params.ProtocolVersion,
	})

	// Return capabilities
	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"protocolVersion": "2025-06-18",
		"serverInfo": map[string]interface{}{
			"name":    "developer-mesh-mcp",
			"version": "1.0.0",
		},
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{
				"listChanged": true,
			},
			"resources": map[string]interface{}{
				"subscribe":   true,
				"listChanged": true,
			},
			"prompts": map[string]interface{}{
				"listChanged": true,
			},
		},
	})
}

// handleToolsList handles the tools/list request
func (h *MCPProtocolHandler) handleToolsList(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	startTime := time.Now()
	defer func() {
		h.recordTelemetry("tools_list", time.Since(startTime), true)
	}()

	ctx := context.Background()

	// Get session
	session := h.getSession(connID)
	if session == nil {
		h.recordTelemetry("tools_list", time.Since(startTime), false)
		return h.sendError(conn, msg.ID, MCPErrorInvalidRequest, "Session not initialized")
	}

	// Check cache first
	if h.toolsCache != nil {
		if cachedTools, ok := h.toolsCache.Get(); ok {
			h.logger.Debug("Using cached tools list", map[string]interface{}{
				"count": len(cachedTools),
			})
			return h.sendResponse(conn, msg.ID, map[string]interface{}{
				"tools": cachedTools,
			})
		}
	}

	// Skip legacy adapter tools - we've fully migrated to MCP
	// Get both dynamic tools and organization tools from REST API
	tools, err := h.restAPIClient.ListTools(ctx, tenantID)
	if err != nil {
		h.logger.Error("Failed to list dynamic tools", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID,
		})
		// Don't fail completely - continue with DevMesh tools
		tools = nil
	}

	// Also get organization tools if standard tools are enabled
	if feature.IsEnabled(feature.EnableStandardTools) {
		// TODO: Call organization tools endpoint when available
		// For now, this is a placeholder for the expanded organization tools
		// We'll need to extract organization ID from the connection context
		h.logger.Debug("Standard tools enabled", map[string]interface{}{
			"tenant_id": tenantID,
		})
	}

	// Create tools list with just DevMesh tools and dynamic tools
	mcpTools := make([]map[string]interface{}, 0, len(tools)+10)

	// Add DevMesh-specific tools as standard MCP tools
	devMeshTools := []map[string]interface{}{
		{
			"name":        "devmesh_agent_assign",
			"description": "Assign a task to a specialized AI agent",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"agent_type": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"code_review", "security", "performance", "documentation", "testing"},
						"description": "Type of specialized agent",
					},
					"task": map[string]interface{}{
						"type":        "string",
						"description": "Task description or URL",
					},
					"priority": map[string]interface{}{
						"type":    "string",
						"enum":    []string{"low", "medium", "high"},
						"default": "medium",
					},
					"context": map[string]interface{}{
						"type":                 "object",
						"description":          "Additional context for the task",
						"additionalProperties": true,
					},
				},
				"required": []string{"agent_type", "task"},
			},
		},
		{
			"name":        "devmesh_context_update",
			"description": "Update session context with new information",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"context": map[string]interface{}{
						"type":                 "object",
						"description":          "Context data to store",
						"additionalProperties": true,
					},
					"merge": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether to merge with existing context or replace",
						"default":     true,
					},
				},
				"required": []string{"context"},
			},
		},
		{
			"name":        "devmesh_context_get",
			"description": "Retrieve current session context",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"keys": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Specific keys to retrieve (optional, returns all if not specified)",
					},
				},
			},
		},
		{
			"name":        "devmesh_search_semantic",
			"description": "Semantic search across codebase and documentation",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query",
					},
					"limit": map[string]interface{}{
						"type":    "integer",
						"minimum": 1,
						"maximum": 100,
						"default": 10,
					},
					"filters": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"file_types": map[string]interface{}{
								"type":        "array",
								"items":       map[string]interface{}{"type": "string"},
								"description": "Filter by file extensions (e.g., ['.go', '.js'])",
							},
							"paths": map[string]interface{}{
								"type":        "array",
								"items":       map[string]interface{}{"type": "string"},
								"description": "Filter by path patterns",
							},
							"min_score": map[string]interface{}{
								"type":        "number",
								"minimum":     0,
								"maximum":     1,
								"description": "Minimum similarity score",
							},
						},
					},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "devmesh_workflow_execute",
			"description": "Execute a predefined workflow",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workflow_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the workflow to execute",
					},
					"parameters": map[string]interface{}{
						"type":                 "object",
						"description":          "Workflow parameters",
						"additionalProperties": true,
					},
					"async": map[string]interface{}{
						"type":        "boolean",
						"description": "Execute asynchronously",
						"default":     false,
					},
				},
				"required": []string{"workflow_id"},
			},
		},
		{
			"name":        "devmesh_workflow_list",
			"description": "List available workflows",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"category": map[string]interface{}{
						"type":        "string",
						"description": "Filter by workflow category",
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Filter by tags",
					},
				},
			},
		},
		{
			"name":        "devmesh_task_create",
			"description": "Create a new task",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Task title",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Task description",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"bug", "feature", "refactor", "test", "documentation"},
						"description": "Task type",
					},
					"priority": map[string]interface{}{
						"type":    "string",
						"enum":    []string{"low", "medium", "high", "critical"},
						"default": "medium",
					},
					"assignee": map[string]interface{}{
						"type":        "string",
						"description": "Agent ID to assign the task to",
					},
				},
				"required": []string{"title", "type"},
			},
		},
		{
			"name":        "devmesh_task_status",
			"description": "Get or update task status",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "Task ID",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"pending", "in_progress", "blocked", "completed", "cancelled"},
						"description": "New status (optional, just queries if not provided)",
					},
					"notes": map[string]interface{}{
						"type":        "string",
						"description": "Status update notes",
					},
				},
				"required": []string{"task_id"},
			},
		},
	}

	// Add DevMesh tools to the list
	mcpTools = append(mcpTools, devMeshTools...)

	// Transform dynamic tools to MCP format
	for _, tool := range tools {
		// Generate minimal inputSchema to reduce context usage
		// This creates tool-specific schemas based on naming patterns
		inputSchema := h.generateMinimalInputSchema(tool.ToolName)

		// Use tool name for consistency
		name := tool.ToolName

		// Get tool description
		description := tool.DisplayName
		if description == "" {
			description = fmt.Sprintf("%s integration", name)
		}

		mcpTools = append(mcpTools, map[string]interface{}{
			"name":        name,
			"description": description,
			"inputSchema": inputSchema,
		})
	}

	// Cache the tools list
	if h.toolsCache != nil {
		convertedTools := make([]interface{}, len(mcpTools))
		for i, tool := range mcpTools {
			convertedTools[i] = tool
		}
		h.toolsCache.Set(convertedTools)
	}

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"tools": mcpTools,
	})
}

// handleToolCall handles the tools/call request
func (h *MCPProtocolHandler) handleToolCall(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	startTime := time.Now()
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		h.recordTelemetry("tools_call", time.Since(startTime), false)
		return h.sendError(conn, msg.ID, MCPErrorInvalidParams, "Invalid params")
	}

	defer func() {
		h.recordTelemetry(fmt.Sprintf("tools_call.%s", params.Name), time.Since(startTime), true)
	}()

	ctx := context.Background()

	// Get session
	session := h.getSession(connID)
	if session == nil {
		return h.sendError(conn, msg.ID, MCPErrorInvalidRequest, "Session not initialized")
	}

	h.logger.Info("Executing tool via MCP", map[string]interface{}{
		"tool":      params.Name,
		"tenant_id": tenantID,
	})

	// Route based on tool namespace
	if strings.HasPrefix(params.Name, "devmesh.") {
		// Handle DevMesh namespace tools
		return h.handleDevMeshTool(conn, connID, tenantID, msg, params.Name, params.Arguments)
	}

	// Legacy adapter tools are deprecated - we've fully migrated to MCP
	// All tools should use either the devmesh. namespace or be dynamic tools

	// Otherwise use dynamic tools via REST API
	// Extract tool name and action from the params
	// Format could be "toolName" or "toolName.action"
	toolName := params.Name
	action := "execute" // default action
	if idx := strings.LastIndex(params.Name, "."); idx != -1 && !strings.HasPrefix(params.Name, "devmesh.") {
		toolName = params.Name[:idx]
		action = params.Name[idx+1:]
	}

	// Resolve tool name to UUID with tenant isolation
	toolID, err := h.resolveToolNameToID(ctx, tenantID, toolName)
	if err != nil {
		h.logger.Error("Failed to resolve tool name", map[string]interface{}{
			"tool_name": toolName,
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		h.recordTelemetry(fmt.Sprintf("tools_call.%s", params.Name), time.Since(startTime), false)
		return h.sendError(conn, msg.ID, MCPErrorInvalidParams, fmt.Sprintf("Tool '%s' not found", toolName))
	}

	// Get circuit breaker for this tool
	breaker := h.circuitBreakers.GetBreaker(toolID)

	// Execute via existing tool execution endpoint with circuit breaker protection
	resultInterface, err := breaker.Call(ctx, toolID, func() (interface{}, error) {
		return h.restAPIClient.ExecuteTool(
			ctx,
			tenantID,
			toolID,
			action,
			params.Arguments,
		)
	})

	if err != nil {
		h.logger.Error("Tool execution failed", map[string]interface{}{
			"tool":      params.Name,
			"error":     err.Error(),
			"tenant_id": tenantID,
		})
		h.recordTelemetry(fmt.Sprintf("tools_call.%s", params.Name), time.Since(startTime), false)
		return h.sendError(conn, msg.ID, MCPErrorInternalError, fmt.Sprintf("Tool execution failed: %v", err))
	}

	result := resultInterface.(*clients.ToolExecutionResult)

	// Return in MCP format
	// Format the response based on what's available
	var responseText string
	if result.Result != nil && result.Result.Body != nil {
		// Convert body to string representation
		if bodyStr, ok := result.Result.Body.(string); ok {
			responseText = bodyStr
		} else {
			// Marshal body to JSON string
			bodyBytes, _ := json.Marshal(result.Result.Body)
			responseText = string(bodyBytes)
		}
	} else if result.Error != nil {
		responseText = fmt.Sprintf("Error: %s", result.Error.Error())
	} else if result.Result != nil {
		responseText = fmt.Sprintf("Tool executed successfully (status: %d)", result.Result.StatusCode)
	} else {
		responseText = "Tool execution completed"
	}

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": responseText,
			},
		},
	})
}

// handleDevMeshTool routes and executes DevMesh namespace tools
func (h *MCPProtocolHandler) handleDevMeshTool(conn *websocket.Conn, connID, tenantID string, msg MCPMessage, toolName string, args map[string]interface{}) error {
	switch toolName {
	case "devmesh_agent_assign":
		return h.executeAgentAssign(conn, connID, tenantID, msg, args)
	case "devmesh_context_update":
		return h.executeContextUpdate(conn, connID, tenantID, msg, args)
	case "devmesh_context_get":
		return h.executeContextGet(conn, connID, tenantID, msg, args)
	case "devmesh_search_semantic":
		return h.executeSemanticSearch(conn, connID, tenantID, msg, args)
	case "devmesh_workflow_execute":
		return h.executeWorkflowExecute(conn, connID, tenantID, msg, args)
	case "devmesh_workflow_list":
		return h.executeWorkflowList(conn, connID, tenantID, msg, args)
	case "devmesh_task_create":
		return h.executeTaskCreate(conn, connID, tenantID, msg, args)
	case "devmesh_task_status":
		return h.executeTaskStatus(conn, connID, tenantID, msg, args)
	default:
		return h.sendError(conn, msg.ID, MCPErrorMethodNotFound, fmt.Sprintf("Unknown DevMesh tool: %s", toolName))
	}
}

// generateMinimalInputSchema creates a minimal inputSchema for MCP compatibility
// This analyzes the tool name and creates a generic schema with common parameters
func (h *MCPProtocolHandler) generateMinimalInputSchema(toolName string) map[string]interface{} {
	// Create base schema structure
	schema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []string{},
	}

	properties := schema["properties"].(map[string]interface{})

	// Parse tool name to determine resource type and operation
	// Examples: github_repos, github_issues, gitlab_merge_requests
	parts := strings.Split(toolName, "_")

	// Common parameters for most operations
	if len(parts) >= 2 {
		provider := parts[0] // e.g., "github", "gitlab", "bitbucket"

		// Add common repository parameters for source control tools
		if provider == "github" || provider == "gitlab" || provider == "bitbucket" {
			properties["owner"] = map[string]interface{}{
				"type":        "string",
				"description": "Repository owner or organization",
			}
			properties["repo"] = map[string]interface{}{
				"type":        "string",
				"description": "Repository name",
			}

			// Check for specific resource types
			if len(parts) > 1 {
				resource := parts[len(parts)-1]

				switch resource {
				case "issues", "issue":
					properties["issue_number"] = map[string]interface{}{
						"type":        "integer",
						"description": "Issue number",
					}
				case "pulls", "pull", "pr", "merge":
					properties["pull_number"] = map[string]interface{}{
						"type":        "integer",
						"description": "Pull request number",
					}
				case "branches", "branch":
					properties["branch"] = map[string]interface{}{
						"type":        "string",
						"description": "Branch name",
					}
				}
			}
		}

		// Add action parameter for all tools
		properties["action"] = map[string]interface{}{
			"type":        "string",
			"description": "Action to perform",
			"enum":        []string{"list", "get", "create", "update", "delete"},
		}

		// Add generic parameters object for additional tool-specific params
		properties["parameters"] = map[string]interface{}{
			"type":                 "object",
			"description":          "Additional parameters specific to the action",
			"additionalProperties": true,
		}
	} else {
		// For unrecognized tool patterns, provide a completely generic schema
		properties["action"] = map[string]interface{}{
			"type":        "string",
			"description": "Action to perform",
		}
		properties["parameters"] = map[string]interface{}{
			"type":                 "object",
			"description":          "Parameters for the action",
			"additionalProperties": true,
		}
	}

	return schema
}

// DevMesh tool execution implementations

func (h *MCPProtocolHandler) executeAgentAssign(conn *websocket.Conn, connID, tenantID string, msg MCPMessage, args map[string]interface{}) error {
	// Extract arguments
	agentType, _ := args["agent_type"].(string)
	task, _ := args["task"].(string)
	priority, _ := args["priority"].(string)
	// context, _ := args["context"].(map[string]interface{}) // TODO: Use context when implementing actual logic

	if priority == "" {
		priority = "medium"
	}

	// TODO: Implement actual agent assignment logic
	h.logger.Info("Agent assignment requested", map[string]interface{}{
		"agent_type": agentType,
		"task":       task,
		"priority":   priority,
		"tenant_id":  tenantID,
	})

	// Mock response
	result := map[string]interface{}{
		"assigned": true,
		"agent_id": fmt.Sprintf("agent-%s-%d", agentType, time.Now().Unix()),
		"task_id":  fmt.Sprintf("task-%d", time.Now().Unix()),
		"status":   "assigned",
		"message":  fmt.Sprintf("Task assigned to %s agent", agentType),
	}

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": jsonString(result),
			},
		},
	})
}

func (h *MCPProtocolHandler) executeContextUpdate(conn *websocket.Conn, connID, tenantID string, msg MCPMessage, args map[string]interface{}) error {
	ctx := context.Background()
	contextData, _ := args["context"].(map[string]interface{})
	// merge, _ := args["merge"].(bool) // TODO: Implement merge logic

	// Use the protocol adapter's handleContextUpdate method via ExecuteTool
	result, err := h.protocolAdapter.ExecuteTool(ctx, "context.update", map[string]interface{}{
		"session_id": connID,
		"context":    contextData,
	})

	if err != nil {
		return h.sendError(conn, msg.ID, MCPErrorInternalError, fmt.Sprintf("Context update failed: %v", err))
	}

	// Convert result to string if needed
	var resultText string
	if resultStr, ok := result.(string); ok {
		resultText = resultStr
	} else {
		resultText = "Context updated successfully"
	}

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": resultText,
			},
		},
	})
}

func (h *MCPProtocolHandler) executeContextGet(conn *websocket.Conn, connID, tenantID string, msg MCPMessage, args map[string]interface{}) error {
	// keys, _ := args["keys"].([]interface{}) // TODO: Implement key filtering

	// Get session info which contains context
	session := h.protocolAdapter.GetSession(connID)

	var contextData map[string]interface{}
	if session != nil {
		contextData = map[string]interface{}{
			"session_id":  session.ID,
			"agent_id":    session.AgentID,
			"agent_type":  session.AgentType,
			"tenant_id":   session.TenantID,
			"initialized": session.Initialized,
		}
	} else {
		// Return empty context if session not found
		contextData = map[string]interface{}{
			"session_id": connID,
			"message":    "No context available",
		}
	}

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": jsonString(contextData),
			},
		},
	})
}

func (h *MCPProtocolHandler) executeSemanticSearch(conn *websocket.Conn, connID, tenantID string, msg MCPMessage, args map[string]interface{}) error {
	query, _ := args["query"].(string)
	limit, _ := args["limit"].(float64)
	// filters, _ := args["filters"].(map[string]interface{}) // TODO: Implement filters

	if limit == 0 {
		limit = 10
	}

	// TODO: Implement actual semantic search via embeddings
	h.logger.Info("Semantic search requested", map[string]interface{}{
		"query":     query,
		"limit":     limit,
		"tenant_id": tenantID,
	})

	// Mock response
	results := []map[string]interface{}{
		{
			"file":       "example.go",
			"line":       42,
			"content":    "// Example matching content",
			"score":      0.95,
			"highlights": []string{query},
		},
	}

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": jsonString(map[string]interface{}{
					"results": results,
					"total":   len(results),
				}),
			},
		},
	})
}

func (h *MCPProtocolHandler) executeWorkflowExecute(conn *websocket.Conn, connID, tenantID string, msg MCPMessage, args map[string]interface{}) error {
	workflowID, _ := args["workflow_id"].(string)
	// parameters, _ := args["parameters"].(map[string]interface{}) // TODO: Use parameters
	async, _ := args["async"].(bool)

	// TODO: Implement actual workflow execution
	h.logger.Info("Workflow execution requested", map[string]interface{}{
		"workflow_id": workflowID,
		"async":       async,
		"tenant_id":   tenantID,
	})

	// Mock response
	result := map[string]interface{}{
		"execution_id": fmt.Sprintf("exec-%d", time.Now().Unix()),
		"workflow_id":  workflowID,
		"status":       "started",
		"async":        async,
	}

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": jsonString(result),
			},
		},
	})
}

func (h *MCPProtocolHandler) executeWorkflowList(conn *websocket.Conn, connID, tenantID string, msg MCPMessage, args map[string]interface{}) error {
	category, _ := args["category"].(string)
	// tags, _ := args["tags"].([]interface{}) // TODO: Implement tag filtering

	// TODO: Implement actual workflow listing
	h.logger.Info("Workflow list requested", map[string]interface{}{
		"category":  category,
		"tenant_id": tenantID,
	})

	// Mock response
	workflows := []map[string]interface{}{
		{
			"id":          "wf-deploy",
			"name":        "Deploy Application",
			"description": "Deploy application to production",
			"category":    "deployment",
			"tags":        []string{"production", "ci/cd"},
		},
		{
			"id":          "wf-test",
			"name":        "Run Tests",
			"description": "Execute test suite",
			"category":    "testing",
			"tags":        []string{"test", "ci"},
		},
	}

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": jsonString(map[string]interface{}{
					"workflows": workflows,
					"total":     len(workflows),
				}),
			},
		},
	})
}

func (h *MCPProtocolHandler) executeTaskCreate(conn *websocket.Conn, connID, tenantID string, msg MCPMessage, args map[string]interface{}) error {
	title, _ := args["title"].(string)
	description, _ := args["description"].(string)
	taskType, _ := args["type"].(string)
	priority, _ := args["priority"].(string)
	assignee, _ := args["assignee"].(string)

	if priority == "" {
		priority = "medium"
	}

	// TODO: Implement actual task creation
	h.logger.Info("Task creation requested", map[string]interface{}{
		"title":     title,
		"type":      taskType,
		"priority":  priority,
		"tenant_id": tenantID,
	})

	// Mock response
	task := map[string]interface{}{
		"id":          fmt.Sprintf("task-%d", time.Now().Unix()),
		"title":       title,
		"description": description,
		"type":        taskType,
		"priority":    priority,
		"assignee":    assignee,
		"status":      "created",
		"created_at":  time.Now().Format(time.RFC3339),
	}

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": jsonString(task),
			},
		},
	})
}

func (h *MCPProtocolHandler) executeTaskStatus(conn *websocket.Conn, connID, tenantID string, msg MCPMessage, args map[string]interface{}) error {
	taskID, _ := args["task_id"].(string)
	status, _ := args["status"].(string)
	notes, _ := args["notes"].(string)

	// TODO: Implement actual task status management
	h.logger.Info("Task status requested", map[string]interface{}{
		"task_id":   taskID,
		"status":    status,
		"tenant_id": tenantID,
	})

	// Mock response
	result := map[string]interface{}{
		"task_id":    taskID,
		"status":     status,
		"notes":      notes,
		"updated_at": time.Now().Format(time.RFC3339),
	}

	if status == "" {
		// Just querying status
		result["status"] = "in_progress"
		result["message"] = "Task is currently in progress"
	} else {
		// Updating status
		result["message"] = "Task status updated successfully"
	}

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": jsonString(result),
			},
		},
	})
}

// Helper function to convert interface to JSON string
func jsonString(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

// handleResourcesList handles the resources/list request
func (h *MCPProtocolHandler) handleResourcesList(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	// Get standard resources from the resource provider
	standardResources := h.resourceProvider.ConvertToMCPResourceList()

	// Add DevMesh-specific resources
	devMeshResources := []map[string]interface{}{
		{
			"uri":         fmt.Sprintf("devmesh://agents/%s", tenantID),
			"name":        "Registered Agents",
			"description": "List of all registered AI agents in the tenant",
			"mimeType":    "application/json",
		},
		{
			"uri":         fmt.Sprintf("devmesh://workflows/%s", tenantID),
			"name":        "Available Workflows",
			"description": "List of configured workflows for the tenant",
			"mimeType":    "application/json",
		},
		{
			"uri":         fmt.Sprintf("devmesh://context/%s", connID),
			"name":        "Session Context",
			"description": "Current session context data",
			"mimeType":    "application/json",
		},
		{
			"uri":         fmt.Sprintf("devmesh://tasks/%s", tenantID),
			"name":        "Active Tasks",
			"description": "List of active tasks in the system",
			"mimeType":    "application/json",
		},
		{
			"uri":         fmt.Sprintf("devmesh://tools/%s", tenantID),
			"name":        "Available Tools",
			"description": "List of all available tools and their configurations",
			"mimeType":    "application/json",
		},
		{
			"uri":         "devmesh://system/health",
			"name":        "System Health",
			"description": "Current system health status and metrics",
			"mimeType":    "application/json",
		},
		{
			"uri":         fmt.Sprintf("devmesh://session/%s/info", connID),
			"name":        "Session Information",
			"description": "Detailed information about the current session",
			"mimeType":    "application/json",
		},
	}

	// Combine resources
	var allResources []map[string]interface{}

	// Add standard resources if they exist
	if resources, ok := standardResources["resources"].([]map[string]interface{}); ok {
		allResources = append(allResources, resources...)
	}

	// Add DevMesh resources
	allResources = append(allResources, devMeshResources...)

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"resources": allResources,
	})
}

// handleResourceRead handles the resources/read request
func (h *MCPProtocolHandler) handleResourceRead(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return h.sendError(conn, msg.ID, MCPErrorInvalidParams, "Invalid params")
	}

	ctx := context.Background()

	// Check if this is a DevMesh resource
	if strings.HasPrefix(params.URI, "devmesh://") {
		return h.handleDevMeshResourceRead(conn, connID, tenantID, msg, params.URI)
	}

	// Otherwise use the standard resource provider
	content, err := h.resourceProvider.ReadResource(ctx, params.URI)
	if err != nil {
		h.logger.Warn("Failed to read resource", map[string]interface{}{
			"uri":   params.URI,
			"error": err.Error(),
		})
		return h.sendError(conn, msg.ID, MCPErrorMethodNotFound, fmt.Sprintf("Resource not found: %s", params.URI))
	}

	// Convert to MCP format
	response := h.resourceProvider.ConvertToMCPResourceRead(params.URI, content)
	return h.sendResult(conn, msg.ID, response)
}

// handleDevMeshResourceRead handles reading DevMesh-specific resources
func (h *MCPProtocolHandler) handleDevMeshResourceRead(conn *websocket.Conn, connID, tenantID string, msg MCPMessage, uri string) error {
	// Parse DevMesh URI
	resourcePath := strings.TrimPrefix(uri, "devmesh://")
	parts := strings.Split(resourcePath, "/")

	if len(parts) == 0 {
		return h.sendError(conn, msg.ID, MCPErrorInvalidParams, "Invalid DevMesh resource URI")
	}

	var content interface{}

	switch parts[0] {
	case "agents":
		// Return list of registered agents
		content = h.getRegisteredAgents(tenantID)

	case "workflows":
		// Return available workflows
		content = h.getAvailableWorkflows(tenantID)

	case "context":
		// Return session context
		if len(parts) > 1 {
			content = h.getSessionContext(parts[1])
		} else {
			content = h.getSessionContext(connID)
		}

	case "tasks":
		// Return active tasks
		content = h.getActiveTasks(tenantID)

	case "tools":
		// Return available tools
		content = h.getAvailableTools(tenantID)

	case "system":
		if len(parts) > 1 && parts[1] == "health" {
			// Return system health
			content = h.getSystemHealth()
		} else {
			return h.sendError(conn, msg.ID, MCPErrorInvalidParams, fmt.Sprintf("Unknown system resource: %s", resourcePath))
		}

	case "session":
		if len(parts) > 2 && parts[2] == "info" {
			// Return session information
			content = h.getSessionInfo(parts[1])
		} else {
			return h.sendError(conn, msg.ID, MCPErrorInvalidParams, fmt.Sprintf("Unknown session resource: %s", resourcePath))
		}

	default:
		return h.sendError(conn, msg.ID, MCPErrorMethodNotFound, fmt.Sprintf("Unknown DevMesh resource: %s", parts[0]))
	}

	// Convert content to JSON string
	var text string
	if contentStr, ok := content.(string); ok {
		text = contentStr
	} else {
		contentBytes, _ := json.MarshalIndent(content, "", "  ")
		text = string(contentBytes)
	}

	// Return in MCP format
	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"uri":      uri,
				"mimeType": "application/json",
				"text":     text,
			},
		},
	})
}

// Helper methods for resource data retrieval

func (h *MCPProtocolHandler) getRegisteredAgents(tenantID string) interface{} {
	// TODO: Implement actual agent retrieval from database
	return []map[string]interface{}{
		{
			"id":           "agent-code-review-001",
			"type":         "code_review",
			"status":       "active",
			"capabilities": []string{"code_analysis", "security_scan", "best_practices"},
			"last_active":  time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
		},
		{
			"id":           "agent-security-001",
			"type":         "security",
			"status":       "active",
			"capabilities": []string{"vulnerability_scan", "dependency_check", "secrets_detection"},
			"last_active":  time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
		},
	}
}

func (h *MCPProtocolHandler) getAvailableWorkflows(tenantID string) interface{} {
	// TODO: Implement actual workflow retrieval
	return []map[string]interface{}{
		{
			"id":          "wf-ci-cd",
			"name":        "CI/CD Pipeline",
			"description": "Complete CI/CD workflow",
			"steps":       5,
			"status":      "available",
		},
		{
			"id":          "wf-code-review",
			"name":        "Automated Code Review",
			"description": "Multi-agent code review process",
			"steps":       3,
			"status":      "available",
		},
	}
}

func (h *MCPProtocolHandler) getSessionContext(sessionID string) interface{} {
	// Try to get session from protocol adapter
	session := h.protocolAdapter.GetSession(sessionID)

	if session != nil {
		return map[string]interface{}{
			"session_id":  session.ID,
			"agent_id":    session.AgentID,
			"agent_type":  session.AgentType,
			"tenant_id":   session.TenantID,
			"initialized": session.Initialized,
			"created_at":  time.Now().Format(time.RFC3339),
		}
	}

	// Return default context
	return map[string]interface{}{
		"session_id":  sessionID,
		"created_at":  time.Now().Format(time.RFC3339),
		"environment": "development",
		"features":    []string{"mcp", "devmesh"},
	}
}

func (h *MCPProtocolHandler) getActiveTasks(tenantID string) interface{} {
	// TODO: Implement actual task retrieval
	return []map[string]interface{}{
		{
			"id":       "task-001",
			"title":    "Review PR #123",
			"type":     "code_review",
			"status":   "in_progress",
			"assignee": "agent-code-review-001",
			"priority": "high",
		},
		{
			"id":       "task-002",
			"title":    "Security scan for deployment",
			"type":     "security",
			"status":   "pending",
			"assignee": "agent-security-001",
			"priority": "medium",
		},
	}
}

func (h *MCPProtocolHandler) getAvailableTools(tenantID string) interface{} {
	ctx := context.Background()
	// Try to get from REST API client
	if tools, err := h.restAPIClient.ListTools(ctx, tenantID); err == nil {
		return tools
	}

	// Return default tools
	return []map[string]interface{}{
		{
			"name":        "github",
			"type":        "api",
			"status":      "active",
			"description": "GitHub API integration",
		},
		{
			"name":        "docker",
			"type":        "cli",
			"status":      "active",
			"description": "Docker container management",
		},
	}
}

func (h *MCPProtocolHandler) getSystemHealth() interface{} {
	// Get metrics from telemetry
	metrics := h.GetMetrics()

	return map[string]interface{}{
		"status":         "healthy",
		"timestamp":      time.Now().Format(time.RFC3339),
		"version":        "1.0.0",
		"uptime_seconds": time.Since(time.Now().Add(-24 * time.Hour)).Seconds(),
		"connections":    len(h.sessions),
		"metrics":        metrics,
	}
}

func (h *MCPProtocolHandler) getSessionInfo(sessionID string) interface{} {
	session := h.getSession(sessionID)
	if session == nil {
		return map[string]interface{}{
			"error": "Session not found",
		}
	}

	return map[string]interface{}{
		"id":         session.ID,
		"tenant_id":  session.TenantID,
		"agent_id":   session.AgentID,
		"created_at": session.CreatedAt.Format(time.RFC3339),
		"duration":   time.Since(session.CreatedAt).String(),
	}
}

// handlePromptsList handles the prompts/list request
func (h *MCPProtocolHandler) handlePromptsList(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	// Prompts will be implemented via REST API proxy
	// For now, return empty list
	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"prompts": []interface{}{},
	})
}

// handlePromptGet handles the prompts/get request
func (h *MCPProtocolHandler) handlePromptGet(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return h.sendError(conn, msg.ID, MCPErrorInvalidParams, "Invalid params")
	}

	// Prompts will be implemented via REST API proxy
	// For now, return error
	return h.sendError(conn, msg.ID, MCPErrorMethodNotFound, "Prompts not yet implemented")
}

// Helper methods

// getSession retrieves a session by connection ID
func (h *MCPProtocolHandler) getSession(connID string) *MCPSession {
	h.sessionsMu.RLock()
	defer h.sessionsMu.RUnlock()
	return h.sessions[connID]
}

// removeSession removes a session when connection closes
func (h *MCPProtocolHandler) RemoveSession(connID string) {
	h.sessionsMu.Lock()
	defer h.sessionsMu.Unlock()
	delete(h.sessions, connID)
}

// sendResult sends a successful result response
func (h *MCPProtocolHandler) sendResult(conn *websocket.Conn, id interface{}, result interface{}) error {
	msg := MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return conn.Write(context.Background(), websocket.MessageText, data)
}

// sendResponse is an alias for sendResult for compatibility
func (h *MCPProtocolHandler) sendResponse(conn *websocket.Conn, id interface{}, result interface{}) error {
	return h.sendResult(conn, id, result)
}

// sendError sends an error response
func (h *MCPProtocolHandler) sendError(conn *websocket.Conn, id interface{}, code int, message string) error {
	msg := MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return conn.Write(context.Background(), websocket.MessageText, data)
}

// IsMCPMessage checks if a message is an MCP protocol message
func IsMCPMessage(message []byte) bool {
	// Quick check for JSON-RPC 2.0 signature
	return strings.Contains(string(message), `"jsonrpc":"2.0"`) ||
		strings.Contains(string(message), `"jsonrpc": "2.0"`)
}

// recordTelemetry records telemetry for MCP operations
func (h *MCPProtocolHandler) recordTelemetry(method string, duration time.Duration, success bool) {
	if h.telemetry != nil {
		h.telemetry.Record(method, duration, success)
	}
	if h.metrics != nil {
		h.metrics.IncrementCounter(fmt.Sprintf("mcp.method.%s", method), 1)
		h.metrics.RecordDuration(fmt.Sprintf("mcp.latency.%s", method), duration)
		if !success {
			h.metrics.IncrementCounter(fmt.Sprintf("mcp.errors.%s", method), 1)
		}
	}
}

// ToolsCache implements a simple TTL cache for tools list
type ToolsCache struct {
	mu         sync.RWMutex
	tools      []interface{}
	lastUpdate time.Time
	ttl        time.Duration
}

// NewToolsCache creates a new tools cache
func NewToolsCache(ttl time.Duration) *ToolsCache {
	return &ToolsCache{
		ttl: ttl,
	}
}

// Get retrieves tools from cache if valid
func (tc *ToolsCache) Get() ([]interface{}, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	if time.Since(tc.lastUpdate) > tc.ttl {
		return nil, false
	}
	return tc.tools, len(tc.tools) > 0
}

// Set updates the cache with new tools
func (tc *ToolsCache) Set(tools []interface{}) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.tools = tools
	tc.lastUpdate = time.Now()
}

// MCPTelemetry tracks MCP protocol metrics
type MCPTelemetry struct {
	mu      sync.RWMutex
	logger  observability.Logger
	metrics observability.MetricsClient

	// Tracking data
	methodCounts  map[string]uint64
	methodLatency map[string][]time.Duration
	errorCounts   map[string]uint64
	totalMessages uint64
	totalErrors   uint64
}

// NewMCPTelemetry creates a new telemetry tracker
func NewMCPTelemetry(logger observability.Logger) *MCPTelemetry {
	return &MCPTelemetry{
		logger:        logger,
		methodCounts:  make(map[string]uint64),
		methodLatency: make(map[string][]time.Duration),
		errorCounts:   make(map[string]uint64),
	}
}

// SetMetricsClient sets the metrics client
func (mt *MCPTelemetry) SetMetricsClient(metrics observability.MetricsClient) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	mt.metrics = metrics
}

// Record records telemetry for a method
func (mt *MCPTelemetry) Record(method string, duration time.Duration, success bool) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	mt.methodCounts[method]++
	mt.totalMessages++

	// Track latency
	if _, exists := mt.methodLatency[method]; !exists {
		mt.methodLatency[method] = make([]time.Duration, 0, 100)
	}
	mt.methodLatency[method] = append(mt.methodLatency[method], duration)

	// Keep bounded
	if len(mt.methodLatency[method]) > 100 {
		mt.methodLatency[method] = mt.methodLatency[method][1:]
	}

	if !success {
		mt.errorCounts[method]++
		mt.totalErrors++
	}
}

// GetStats returns current telemetry statistics
func (mt *MCPTelemetry) GetStats() map[string]interface{} {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	stats := map[string]interface{}{
		"total_messages": mt.totalMessages,
		"total_errors":   mt.totalErrors,
		"method_counts":  mt.methodCounts,
		"error_counts":   mt.errorCounts,
	}

	// Calculate average latencies
	avgLatencies := make(map[string]float64)
	for method, latencies := range mt.methodLatency {
		if len(latencies) > 0 {
			var total time.Duration
			for _, l := range latencies {
				total += l
			}
			avgLatencies[method] = float64(total.Milliseconds()) / float64(len(latencies))
		}
	}
	stats["avg_latency_ms"] = avgLatencies

	return stats
}

// GetMetrics returns comprehensive MCP handler metrics
func (h *MCPProtocolHandler) GetMetrics() map[string]interface{} {
	metrics := map[string]interface{}{
		"sessions_count": len(h.sessions),
	}

	// Add telemetry stats
	if h.telemetry != nil {
		metrics["telemetry"] = h.telemetry.GetStats()
	}

	// Add circuit breaker metrics
	if h.circuitBreakers != nil {
		metrics["circuit_breakers"] = h.circuitBreakers.GetAllMetrics()
	}

	// Add cache metrics
	if h.toolsCache != nil {
		tools, cached := h.toolsCache.Get()
		metrics["tools_cache"] = map[string]interface{}{
			"cached":      cached,
			"tools_count": len(tools),
		}
	}

	return metrics
}

// Missing standard MCP method implementations

// handlePing handles ping requests for keep-alive
func (h *MCPProtocolHandler) handlePing(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	// Simple ping response
	return h.sendResult(conn, msg.ID, map[string]interface{}{"pong": true})
}

// handleShutdown handles graceful shutdown requests
func (h *MCPProtocolHandler) handleShutdown(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	// Acknowledge shutdown request
	if err := h.sendResult(conn, msg.ID, map[string]interface{}{"status": "shutting_down"}); err != nil {
		return err
	}

	// Log the shutdown request
	h.logger.Info("MCP shutdown requested", map[string]interface{}{
		"connection_id": connID,
		"tenant_id":     tenantID,
	})

	// Remove session
	h.RemoveSession(connID)

	// Close connection gracefully
	return conn.Close(websocket.StatusNormalClosure, "Server shutting down")
}

// handleCancelRequest handles request cancellation
func (h *MCPProtocolHandler) handleCancelRequest(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	// Parse the request ID to cancel
	var params struct {
		RequestID interface{} `json:"id"`
	}

	if msg.Params != nil {
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return h.sendError(conn, msg.ID, MCPErrorInvalidParams, "Invalid cancel request params")
		}
	}

	// TODO: Implement actual cancellation logic for in-flight requests
	// For now, acknowledge the request
	h.logger.Info("Cancel request received", map[string]interface{}{
		"connection_id": connID,
		"request_id":    params.RequestID,
	})

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"cancelled": false,
		"reason":    "Cancellation not yet implemented",
	})
}

// handleResourceSubscribe handles resource subscription requests
func (h *MCPProtocolHandler) handleResourceSubscribe(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	var params struct {
		URI string `json:"uri"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return h.sendError(conn, msg.ID, MCPErrorInvalidParams, "Invalid subscription params")
	}

	// TODO: Implement actual subscription logic
	h.logger.Info("Resource subscription requested", map[string]interface{}{
		"connection_id": connID,
		"uri":           params.URI,
	})

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"subscribed": true,
		"uri":        params.URI,
	})
}

// handleResourceUnsubscribe handles resource unsubscription requests
func (h *MCPProtocolHandler) handleResourceUnsubscribe(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	var params struct {
		URI string `json:"uri"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return h.sendError(conn, msg.ID, MCPErrorInvalidParams, "Invalid unsubscription params")
	}

	// TODO: Implement actual unsubscription logic
	h.logger.Info("Resource unsubscription requested", map[string]interface{}{
		"connection_id": connID,
		"uri":           params.URI,
	})

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"unsubscribed": true,
		"uri":          params.URI,
	})
}

// handlePromptRun handles prompt execution requests
func (h *MCPProtocolHandler) handlePromptRun(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return h.sendError(conn, msg.ID, MCPErrorInvalidParams, "Invalid prompt run params")
	}

	// TODO: Implement actual prompt execution via LLM integration
	h.logger.Info("Prompt run requested", map[string]interface{}{
		"connection_id": connID,
		"prompt_name":   params.Name,
	})

	// For now, return a placeholder response
	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"role":    "assistant",
				"content": "Prompt execution not yet implemented",
			},
		},
	})
}

// handleCompletionComplete handles text completion requests
func (h *MCPProtocolHandler) handleCompletionComplete(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	var params struct {
		Ref      map[string]interface{} `json:"ref"`
		Argument map[string]interface{} `json:"argument"`
	}

	if msg.Params != nil {
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return h.sendError(conn, msg.ID, MCPErrorInvalidParams, "Invalid completion params")
		}
	}

	// TODO: Implement actual LLM completion
	h.logger.Info("Completion requested", map[string]interface{}{
		"connection_id": connID,
	})

	// For now, return empty completion
	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"completion": map[string]interface{}{
			"values":  []string{},
			"total":   0,
			"hasMore": false,
		},
	})
}

// handleSamplingCreateMessage handles message sampling/generation requests
func (h *MCPProtocolHandler) handleSamplingCreateMessage(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	var params struct {
		Messages     []map[string]interface{} `json:"messages"`
		ModelHint    string                   `json:"modelHint"`
		SystemPrompt string                   `json:"systemPrompt"`
		MaxTokens    int                      `json:"maxTokens"`
	}

	if msg.Params != nil {
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return h.sendError(conn, msg.ID, MCPErrorInvalidParams, "Invalid sampling params")
		}
	}

	// TODO: Implement actual message generation via LLM
	h.logger.Info("Message sampling requested", map[string]interface{}{
		"connection_id": connID,
		"model_hint":    params.ModelHint,
	})

	// For now, return a placeholder response
	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"role": "assistant",
		"content": map[string]interface{}{
			"type": "text",
			"text": "Message sampling not yet implemented",
		},
		"model":      "placeholder",
		"stopReason": "end_turn",
	})
}

// handleLoggingSetLevel handles logging level changes
func (h *MCPProtocolHandler) handleLoggingSetLevel(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	var params struct {
		Level string `json:"level"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return h.sendError(conn, msg.ID, MCPErrorInvalidParams, "Invalid logging params")
	}

	// Validate log level
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if !validLevels[params.Level] {
		return h.sendError(conn, msg.ID, MCPErrorInvalidParams, fmt.Sprintf("Invalid log level: %s", params.Level))
	}

	// TODO: Actually change the logging level for this session
	h.logger.Info("Logging level change requested", map[string]interface{}{
		"connection_id": connID,
		"new_level":     params.Level,
	})

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"level": params.Level,
	})
}

// DevMesh extension handlers

// handleAgentRegister handles agent registration
func (h *MCPProtocolHandler) handleAgentRegister(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	var params struct {
		AgentID      string   `json:"agent_id"`
		AgentType    string   `json:"agent_type"`
		Capabilities []string `json:"capabilities"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return h.sendError(conn, msg.ID, MCPErrorInvalidParams, "Invalid agent registration params")
	}

	// Register agent via protocol adapter - use InitializeSession which is available
	_, err := h.protocolAdapter.InitializeSession(connID, tenantID, map[string]interface{}{
		"agent_id":     params.AgentID,
		"agent_type":   params.AgentType,
		"capabilities": params.Capabilities,
	})

	if err != nil {
		return h.sendError(conn, msg.ID, MCPErrorInternalError, fmt.Sprintf("Agent registration failed: %v", err))
	}

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"registered": true,
		"agent_id":   params.AgentID,
	})
}

// handleAgentHealth handles agent health checks
func (h *MCPProtocolHandler) handleAgentHealth(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	// Simple health check response
	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	})
}

// handleContextUpdate handles context updates
func (h *MCPProtocolHandler) handleContextUpdate(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	var params struct {
		Context map[string]interface{} `json:"context"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return h.sendError(conn, msg.ID, MCPErrorInvalidParams, "Invalid context update params")
	}

	// Update context via protocol adapter ExecuteTool
	ctx := context.Background()
	result, err := h.protocolAdapter.ExecuteTool(ctx, "context.update", map[string]interface{}{
		"session_id": connID,
		"context":    params.Context,
	})

	if err != nil {
		return h.sendError(conn, msg.ID, MCPErrorInternalError, fmt.Sprintf("Context update failed: %v", err))
	}

	h.logger.Info("Context updated", map[string]interface{}{
		"connection_id": connID,
		"result":        result,
	})

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"updated": true,
	})
}

// handleSemanticSearch handles semantic search requests
func (h *MCPProtocolHandler) handleSemanticSearch(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	var params struct {
		Query   string                 `json:"query"`
		Limit   int                    `json:"limit"`
		Filters map[string]interface{} `json:"filters"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return h.sendError(conn, msg.ID, MCPErrorInvalidParams, "Invalid search params")
	}

	// Default limit
	if params.Limit == 0 {
		params.Limit = 10
	}

	// TODO: Implement actual semantic search via embeddings
	h.logger.Info("Semantic search requested", map[string]interface{}{
		"connection_id": connID,
		"query":         params.Query,
		"limit":         params.Limit,
	})

	// For now, return empty results
	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"results": []interface{}{},
		"total":   0,
	})
}

// handleToolsBatch handles batch tool execution
func (h *MCPProtocolHandler) handleToolsBatch(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	var params struct {
		Calls []struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		} `json:"calls"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return h.sendError(conn, msg.ID, MCPErrorInvalidParams, "Invalid batch params")
	}

	// Execute tools in batch
	results := make([]map[string]interface{}, 0, len(params.Calls))
	ctx := context.Background()

	for _, call := range params.Calls {
		// Execute each tool (simplified for now)
		h.logger.Info("Executing batch tool", map[string]interface{}{
			"tool": call.Name,
		})

		// Try to execute via protocol adapter
		result, err := h.protocolAdapter.ExecuteTool(ctx, call.Name, call.Arguments)
		if err != nil {
			results = append(results, map[string]interface{}{
				"error": err.Error(),
				"tool":  call.Name,
			})
		} else {
			results = append(results, map[string]interface{}{
				"result": result,
				"tool":   call.Name,
			})
		}
	}

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"results": results,
	})
}
