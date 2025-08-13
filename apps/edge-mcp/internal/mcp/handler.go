package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/auth"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/cache"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/core"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/platform"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
)

// MCPMessage represents a JSON-RPC message in the MCP protocol
type MCPMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handler manages MCP protocol connections
type Handler struct {
	tools         *tools.Registry
	cache         cache.Cache
	coreClient    *core.Client
	authenticator auth.Authenticator
	sessions      map[string]*Session
	sessionsMu    sync.RWMutex
	logger        observability.Logger

	// Request tracking for cancellation
	activeRequests map[interface{}]context.CancelFunc
	requestsMu     sync.RWMutex
}

// Session represents an MCP session
type Session struct {
	ID           string
	ConnectionID string
	Initialized  bool
	TenantID     string
	EdgeMCPID    string
	CoreSession  string // Core Platform session ID for context sync
	CreatedAt    time.Time
	LastActivity time.Time
}

// NewHandler creates a new MCP handler
func NewHandler(
	toolRegistry *tools.Registry,
	cache cache.Cache,
	coreClient *core.Client,
	authenticator auth.Authenticator,
	logger observability.Logger,
) *Handler {
	return &Handler{
		tools:          toolRegistry,
		cache:          cache,
		coreClient:     coreClient,
		authenticator:  authenticator,
		sessions:       make(map[string]*Session),
		logger:         logger,
		activeRequests: make(map[interface{}]context.CancelFunc),
	}
}

// HandleConnection handles a WebSocket connection
func (h *Handler) HandleConnection(conn *websocket.Conn, r *http.Request) {
	sessionID := uuid.New().String()
	session := &Session{
		ID:           sessionID,
		ConnectionID: uuid.New().String(),
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	h.sessionsMu.Lock()
	h.sessions[sessionID] = session
	h.sessionsMu.Unlock()

	defer func() {
		h.sessionsMu.Lock()
		delete(h.sessions, sessionID)
		h.sessionsMu.Unlock()
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()

	// Create a context for this connection
	ctx := r.Context()

	// Start ping ticker to keep connection alive
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				if err := conn.Ping(ctx); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Message handling loop
	for {
		var msg MCPMessage
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
				h.logger.Error("WebSocket error", map[string]interface{}{
					"error": err.Error(),
				})
			}
			break
		}

		// Update activity
		h.sessionsMu.Lock()
		if s, exists := h.sessions[sessionID]; exists {
			s.LastActivity = time.Now()
		}
		h.sessionsMu.Unlock()

		// Handle message
		response, err := h.handleMessage(sessionID, &msg)
		if err != nil {
			response = &MCPMessage{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Error: &MCPError{
					Code:    -32603,
					Message: err.Error(),
				},
			}
		}

		if response != nil {
			if err := wsjson.Write(ctx, conn, response); err != nil {
				h.logger.Error("Failed to write response", map[string]interface{}{
					"error": err.Error(),
				})
				break
			}
		}
	}
}

// handleMessage processes an MCP message
func (h *Handler) handleMessage(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	switch msg.Method {
	case "initialize":
		return h.handleInitialize(sessionID, msg)
	case "initialized":
		return h.handleInitialized(sessionID, msg)
	case "ping":
		return h.handlePing(msg)
	case "shutdown":
		return h.handleShutdown(sessionID, msg)
	case "tools/list":
		return h.handleToolsList(sessionID, msg)
	case "tools/call":
		return h.handleToolCall(sessionID, msg)
	case "resources/list":
		return h.handleResourcesList(sessionID, msg)
	case "resources/read":
		return h.handleResourceRead(sessionID, msg)
	case "prompts/list":
		return h.handlePromptsList(sessionID, msg)
	case "logging/setLevel":
		return h.handleLoggingSetLevel(sessionID, msg)
	case "$/cancelRequest":
		return h.handleCancelRequest(sessionID, msg)
	default:
		return nil, fmt.Errorf("method not found: %s", msg.Method)
	}
}

// handleInitialize handles the initialize request
func (h *Handler) handleInitialize(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
		ClientInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Type    string `json:"type,omitempty"`
		} `json:"clientInfo"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid initialize params: %w", err)
	}

	// Verify protocol version
	if params.ProtocolVersion != "2025-06-18" {
		return nil, fmt.Errorf("unsupported protocol version: %s", params.ProtocolVersion)
	}

	// Update session
	h.sessionsMu.Lock()
	if session, exists := h.sessions[sessionID]; exists {
		session.Initialized = true

		// If connected to Core Platform, create a linked session
		if h.coreClient != nil {
			coreSessionID, err := h.coreClient.CreateSession(
				context.Background(),
				params.ClientInfo.Name,
				params.ClientInfo.Type,
			)
			if err != nil {
				h.logger.Warn("Failed to create Core Platform session", map[string]interface{}{
					"error": err.Error(),
				})
			} else {
				session.CoreSession = coreSessionID
			}
		}
	}
	h.sessionsMu.Unlock()

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2025-06-18",
			"serverInfo": map[string]interface{}{
				"name":    "edge-mcp",
				"version": "1.0.0",
			},
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{
					"listChanged": true,
				},
				"resources": map[string]interface{}{
					"subscribe":   false, // Edge MCP doesn't support subscriptions
					"listChanged": false,
				},
				"prompts": map[string]interface{}{},
				"logging": map[string]interface{}{},
			},
		},
	}, nil
}

// handleInitialized handles the initialized notification
func (h *Handler) handleInitialized(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	// Client confirms initialization complete
	h.sessionsMu.Lock()
	if session, exists := h.sessions[sessionID]; exists {
		session.Initialized = true
	}
	h.sessionsMu.Unlock()

	// No response for notifications
	return nil, nil
}

// handlePing handles ping requests
func (h *Handler) handlePing(msg *MCPMessage) (*MCPMessage, error) {
	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  map[string]interface{}{},
	}, nil
}

// handleShutdown handles shutdown requests
func (h *Handler) handleShutdown(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	// Clean up session
	h.sessionsMu.Lock()
	if session, exists := h.sessions[sessionID]; exists {
		// If connected to Core Platform, close the linked session
		if h.coreClient != nil && session.CoreSession != "" {
			_ = h.coreClient.CloseSession(context.Background(), session.CoreSession)
		}
	}
	delete(h.sessions, sessionID)
	h.sessionsMu.Unlock()

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  map[string]interface{}{},
	}, nil
}

// handleToolsList handles tools/list requests
func (h *Handler) handleToolsList(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	tools := h.tools.ListAll()

	toolList := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		toolList = append(toolList, map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		})
	}

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"tools": toolList,
		},
	}, nil
}

// handleToolCall handles tools/call requests
func (h *Handler) handleToolCall(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid tool call params: %w", err)
	}

	// CRITICAL: Handle context operations specially for sync with Core Platform
	if params.Name == "context.update" || params.Name == "context.append" || params.Name == "context.get" {
		return h.handleContextOperation(sessionID, msg.ID, params.Name, params.Arguments)
	}

	// Create cancellable context for tool execution
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure cancel is always called to prevent context leak

	// Track the request for potential cancellation (only if ID is present)
	if msg.ID != nil {
		h.trackRequest(msg.ID, cancel)
		defer h.untrackRequest(msg.ID)
	}

	// Execute tool with cancellable context
	result, err := h.tools.Execute(ctx, params.Name, params.Arguments)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	// Record execution with Core Platform if connected
	if h.coreClient != nil {
		h.sessionsMu.RLock()
		session := h.sessions[sessionID]
		coreSessionID := ""
		if session != nil {
			coreSessionID = session.CoreSession
		}
		h.sessionsMu.RUnlock()

		if coreSessionID != "" {
			_ = h.coreClient.RecordToolExecution(
				context.Background(),
				coreSessionID,
				params.Name,
				params.Arguments,
				result,
			)
		}
	}

	// Format result as MCP content
	content := []map[string]interface{}{
		{
			"type": "text",
			"text": fmt.Sprintf("%v", result),
		},
	}

	// If result is already structured, use it directly
	if resultMap, ok := result.(map[string]interface{}); ok {
		if resultContent, ok := resultMap["content"]; ok {
			content = resultContent.([]map[string]interface{})
		}
	}

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"content": content,
		},
	}, nil
}

// handleContextOperation handles context sync with Core Platform
func (h *Handler) handleContextOperation(sessionID string, msgID interface{}, operation string, args json.RawMessage) (*MCPMessage, error) {
	// If not connected to Core Platform, return error
	if h.coreClient == nil {
		return nil, fmt.Errorf("context operations require Core Platform connection")
	}

	h.sessionsMu.RLock()
	session := h.sessions[sessionID]
	coreContextID := ""
	if session != nil {
		coreContextID = session.CoreSession
	}
	h.sessionsMu.RUnlock()

	if coreContextID == "" {
		return nil, fmt.Errorf("no active Core Platform session")
	}

	var result interface{}
	var err error

	switch operation {
	case "context.update":
		var contextUpdate map[string]interface{}
		if err := json.Unmarshal(args, &contextUpdate); err != nil {
			return nil, fmt.Errorf("invalid context update: %w", err)
		}

		err = h.coreClient.UpdateContext(context.Background(), coreContextID, contextUpdate)
		if err == nil {
			// Cache locally for performance
			_ = h.cache.Set(context.Background(), fmt.Sprintf("context:%s", sessionID), contextUpdate, 5*time.Minute)
			result = map[string]interface{}{"success": true}
		}

	case "context.get":
		// Try cache first
		var cached map[string]interface{}
		if err := h.cache.Get(context.Background(), fmt.Sprintf("context:%s", sessionID), &cached); err == nil {
			result = cached
		} else {
			// Fetch from Core Platform
			result, err = h.coreClient.GetContext(context.Background(), coreContextID)
			if err == nil {
				// Cache the result
				_ = h.cache.Set(context.Background(), fmt.Sprintf("context:%s", sessionID), result, 5*time.Minute)
			}
		}

	case "context.append":
		var appendData map[string]interface{}
		if err := json.Unmarshal(args, &appendData); err != nil {
			return nil, fmt.Errorf("invalid append data: %w", err)
		}

		err = h.coreClient.AppendContext(context.Background(), coreContextID, appendData)
		if err == nil {
			result = map[string]interface{}{"success": true}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("context operation failed: %w", err)
	}

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msgID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("%v", result),
				},
			},
		},
	}, nil
}

// handleResourcesList handles resources/list requests
func (h *Handler) handleResourcesList(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	resources := []map[string]interface{}{
		{
			"uri":         "edge://system/info",
			"name":        "System Information",
			"description": "Edge MCP system information",
			"mimeType":    "application/json",
		},
		{
			"uri":         "edge://platform/info",
			"name":        "Platform Information",
			"description": "Operating system and platform capabilities",
			"mimeType":    "application/json",
		},
		{
			"uri":         "edge://tools/list",
			"name":        "Available Tools",
			"description": "List of available tools",
			"mimeType":    "application/json",
		},
	}

	// Add Core Platform resources if connected
	if h.coreClient != nil {
		resources = append(resources, map[string]interface{}{
			"uri":         "core://connection/status",
			"name":        "Core Connection Status",
			"description": "Status of Core Platform connection",
			"mimeType":    "application/json",
		})
	}

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"resources": resources,
		},
	}, nil
}

// handleResourceRead handles resources/read requests
func (h *Handler) handleResourceRead(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	var params struct {
		URI string `json:"uri"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid resource read params: %w", err)
	}

	var content interface{}

	switch params.URI {
	case "edge://system/info":
		content = map[string]interface{}{
			"version":        "1.0.0",
			"core_connected": h.coreClient != nil,
			"tools_count":    h.tools.Count(),
			"cache_size":     h.cache.Size(),
		}

	case "edge://platform/info":
		content = platform.GetInfo()

	case "edge://tools/list":
		tools := h.tools.ListAll()
		toolNames := make([]string, 0, len(tools))
		for _, tool := range tools {
			toolNames = append(toolNames, tool.Name)
		}
		content = toolNames

	case "core://connection/status":
		if h.coreClient != nil {
			content = h.coreClient.GetStatus()
		} else {
			content = map[string]interface{}{
				"connected": false,
				"error":     "Core Platform not configured",
			}
		}

	default:
		return nil, fmt.Errorf("resource not found: %s", params.URI)
	}

	contentJSON, _ := json.Marshal(content)

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"uri":      params.URI,
					"mimeType": "application/json",
					"text":     string(contentJSON),
				},
			},
		},
	}, nil
}

// handlePromptsList handles prompts/list requests
func (h *Handler) handlePromptsList(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	// Edge MCP doesn't provide prompts
	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"prompts": []interface{}{},
		},
	}, nil
}

// handleLoggingSetLevel handles logging/setLevel requests
func (h *Handler) handleLoggingSetLevel(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	var params struct {
		Level string `json:"level"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid logging params: %w", err)
	}

	// Map MCP log levels to observability log levels
	levelMap := map[string]observability.LogLevel{
		"debug":   observability.LogLevelDebug,
		"info":    observability.LogLevelInfo,
		"warning": observability.LogLevelWarn,
		"warn":    observability.LogLevelWarn,
		"error":   observability.LogLevelError,
	}

	newLevel, ok := levelMap[params.Level]
	if !ok {
		return nil, fmt.Errorf("invalid log level: %s", params.Level)
	}

	// Create a new logger with the specified level if StandardLogger
	if stdLogger, ok := h.logger.(*observability.StandardLogger); ok {
		h.logger = stdLogger.WithLevel(newLevel)
		h.logger.Info("Log level changed", map[string]interface{}{
			"new_level": params.Level,
		})
	}

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  map[string]interface{}{},
	}, nil
}

// handleCancelRequest handles $/cancelRequest requests
func (h *Handler) handleCancelRequest(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	var params struct {
		ID interface{} `json:"id"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid cancel params: %w", err)
	}

	// Look up and cancel the request
	h.requestsMu.Lock()
	cancel, exists := h.activeRequests[params.ID]
	if exists {
		delete(h.activeRequests, params.ID)
	}
	h.requestsMu.Unlock()

	if exists {
		// Cancel the request context
		cancel()
		h.logger.Info("Request cancelled", map[string]interface{}{
			"request_id": params.ID,
			"session_id": sessionID,
		})
	} else {
		h.logger.Warn("Request not found for cancellation", map[string]interface{}{
			"request_id": params.ID,
			"session_id": sessionID,
		})
	}

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  map[string]interface{}{},
	}, nil
}

// trackRequest registers a request for potential cancellation
func (h *Handler) trackRequest(id interface{}, cancel context.CancelFunc) {
	h.requestsMu.Lock()
	h.activeRequests[id] = cancel
	h.requestsMu.Unlock()
}

// untrackRequest removes a request from tracking
func (h *Handler) untrackRequest(id interface{}) {
	h.requestsMu.Lock()
	delete(h.activeRequests, id)
	h.requestsMu.Unlock()
}
