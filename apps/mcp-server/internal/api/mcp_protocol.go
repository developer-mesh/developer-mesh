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
	}
}

// HandleMessage processes an MCP protocol message
func (h *MCPProtocolHandler) HandleMessage(conn *websocket.Conn, connID string, tenantID string, message []byte) error {
	var msg MCPMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		h.logger.Error("Failed to parse MCP message", map[string]interface{}{
			"error":         err.Error(),
			"connection_id": connID,
		})
		return h.sendError(conn, nil, MCPErrorParseError, "Parse error")
	}

	h.logger.Debug("Handling MCP method", map[string]interface{}{
		"method":        msg.Method,
		"id":            msg.ID,
		"connection_id": connID,
	})

	// Route to appropriate handler based on method
	switch msg.Method {
	case "initialize":
		return h.handleInitialize(conn, connID, tenantID, msg)
	case "tools/list":
		return h.handleToolsList(conn, connID, tenantID, msg)
	case "tools/call":
		return h.handleToolCall(conn, connID, tenantID, msg)
	case "resources/list":
		return h.handleResourcesList(conn, connID, tenantID, msg)
	case "resources/read":
		return h.handleResourceRead(conn, connID, tenantID, msg)
	case "prompts/list":
		return h.handlePromptsList(conn, connID, tenantID, msg)
	case "prompts/get":
		return h.handlePromptGet(conn, connID, tenantID, msg)
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
		"protocolVersion": "2024-11-05",
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
	ctx := context.Background()

	// Get session
	session := h.getSession(connID)
	if session == nil {
		return h.sendError(conn, msg.ID, MCPErrorInvalidRequest, "Session not initialized")
	}

	// Get custom protocol tools from adapter
	adapterTools := h.protocolAdapter.GetTools()

	// Use existing dynamic tool discovery
	tools, err := h.restAPIClient.ListTools(ctx, tenantID)
	if err != nil {
		h.logger.Error("Failed to list tools", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID,
		})
		// Don't fail completely - just use adapter tools
		tools = nil
	}

	// Combine adapter tools and dynamic tools
	mcpTools := make([]map[string]interface{}, 0, len(tools)+len(adapterTools))

	// Add adapter tools first (custom protocol tools as MCP tools)
	mcpTools = append(mcpTools, adapterTools...)

	// Transform dynamic tools to MCP format
	for _, tool := range tools {
		// Use the OpenAPI spec or API spec as input schema if available
		var inputSchema interface{}
		if tool.APISpec != nil {
			inputSchema = tool.APISpec
		} else if tool.OpenAPISpec != nil {
			inputSchema = tool.OpenAPISpec
		}

		// Use display name or tool name
		name := tool.DisplayName
		if name == "" {
			name = tool.ToolName
		}

		mcpTools = append(mcpTools, map[string]interface{}{
			"name":        name,
			"description": fmt.Sprintf("%s tool", name),
			"inputSchema": inputSchema,
		})
	}

	return h.sendResult(conn, msg.ID, map[string]interface{}{
		"tools": mcpTools,
	})
}

// handleToolCall handles the tools/call request
func (h *MCPProtocolHandler) handleToolCall(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return h.sendError(conn, msg.ID, MCPErrorInvalidParams, "Invalid params")
	}

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

	// First check if this is a custom protocol tool handled by the adapter
	if strings.HasPrefix(params.Name, "agent.") ||
		strings.HasPrefix(params.Name, "workflow.") ||
		strings.HasPrefix(params.Name, "task.") ||
		strings.HasPrefix(params.Name, "context.") {
		// Execute via protocol adapter
		result, err := h.protocolAdapter.ExecuteTool(ctx, params.Name, params.Arguments)
		if err != nil {
			h.logger.Error("Adapter tool execution failed", map[string]interface{}{
				"tool":      params.Name,
				"error":     err.Error(),
				"tenant_id": tenantID,
			})
			return h.sendError(conn, msg.ID, MCPErrorInternalError, fmt.Sprintf("Tool execution failed: %v", err))
		}

		// Convert result to MCP format
		var responseText string
		if resultStr, ok := result.(string); ok {
			responseText = resultStr
		} else {
			resultBytes, _ := json.Marshal(result)
			responseText = string(resultBytes)
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

	// Otherwise use dynamic tools via REST API
	// Extract tool ID and action from the name
	// Format could be "toolName" or "toolName.action"
	toolID := params.Name
	action := "execute" // default action
	if idx := strings.LastIndex(params.Name, "."); idx != -1 {
		toolID = params.Name[:idx]
		action = params.Name[idx+1:]
	}

	// Execute via existing tool execution endpoint
	result, err := h.restAPIClient.ExecuteTool(
		ctx,
		tenantID,
		toolID,
		action,
		params.Arguments,
	)

	if err != nil {
		h.logger.Error("Tool execution failed", map[string]interface{}{
			"tool":      params.Name,
			"error":     err.Error(),
			"tenant_id": tenantID,
		})
		return h.sendError(conn, msg.ID, MCPErrorInternalError, fmt.Sprintf("Tool execution failed: %v", err))
	}

	// Return in MCP format
	// Format the response based on what's available
	var responseText string
	if result.Body != nil {
		// Convert body to string representation
		if bodyStr, ok := result.Body.(string); ok {
			responseText = bodyStr
		} else {
			// Marshal body to JSON string
			bodyBytes, _ := json.Marshal(result.Body)
			responseText = string(bodyBytes)
		}
	} else if result.Error != "" {
		responseText = fmt.Sprintf("Error: %s", result.Error)
	} else {
		responseText = fmt.Sprintf("Tool executed successfully (status: %d)", result.StatusCode)
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

// handleResourcesList handles the resources/list request
func (h *MCPProtocolHandler) handleResourcesList(conn *websocket.Conn, connID, tenantID string, msg MCPMessage) error {
	// Get resources from the resource provider
	resourceList := h.resourceProvider.ConvertToMCPResourceList()
	return h.sendResult(conn, msg.ID, resourceList)
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

	// Read the resource
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
