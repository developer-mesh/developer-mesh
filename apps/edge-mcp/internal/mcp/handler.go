package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/auth"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/cache"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/core"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/platform"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/developer-mesh/developer-mesh/pkg/models"
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
	tools          *tools.Registry
	cache          cache.Cache
	coreClient     *core.Client
	authenticator  auth.Authenticator
	sessions       map[string]*Session
	sessionsMu     sync.RWMutex
	logger         observability.Logger
	refreshManager *tools.RefreshManager

	// Request tracking for cancellation
	activeRequests map[interface{}]context.CancelFunc
	requestsMu     sync.RWMutex
}

// Session represents an MCP session
type Session struct {
	ID              string
	ConnectionID    string
	Initialized     bool
	TenantID        string
	EdgeMCPID       string
	CoreSession     string // Core Platform session ID for context sync
	CreatedAt       time.Time
	LastActivity    time.Time
	PassthroughAuth *models.PassthroughAuthBundle // User-specific credentials for pass-through
}

// NewHandler creates a new MCP handler
func NewHandler(
	toolRegistry *tools.Registry,
	cache cache.Cache,
	coreClient *core.Client,
	authenticator auth.Authenticator,
	logger observability.Logger,
) *Handler {
	h := &Handler{
		tools:          toolRegistry,
		cache:          cache,
		coreClient:     coreClient,
		authenticator:  authenticator,
		sessions:       make(map[string]*Session),
		logger:         logger,
		activeRequests: make(map[interface{}]context.CancelFunc),
	}

	// Setup refresh manager if core client is available
	if coreClient != nil {
		refreshConfig := tools.DefaultRefreshConfig()
		h.refreshManager = tools.NewRefreshManager(
			toolRegistry,
			refreshConfig,
			logger,
			func(ctx context.Context) ([]tools.ToolDefinition, error) {
				return coreClient.FetchRemoteTools(ctx)
			},
		)
		// Start automatic refresh
		h.refreshManager.Start(context.Background())
	}

	return h
}

// HandleConnection handles a WebSocket connection
func (h *Handler) HandleConnection(conn *websocket.Conn, r *http.Request) {
	sessionID := uuid.New().String()

	// Extract passthrough authentication from headers
	passthroughAuth := h.extractPassthroughAuth(r)

	session := &Session{
		ID:              sessionID,
		ConnectionID:    uuid.New().String(),
		CreatedAt:       time.Now(),
		LastActivity:    time.Now(),
		PassthroughAuth: passthroughAuth,
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
	// Use background context, not request context which gets cancelled after upgrade
	ctx := context.Background()

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

// HandleStdio handles MCP protocol over stdin/stdout for Claude Code integration
func (h *Handler) HandleStdio() {
	sessionID := uuid.New().String()

	// Log all environment variables that might contain tokens (for debugging)
	// Write to stderr so it appears in Claude logs
	fmt.Fprintf(os.Stderr, "[edge-mcp] Checking environment variables:\n")
	fmt.Fprintf(os.Stderr, "  HARNESS_TOKEN: %v\n", os.Getenv("HARNESS_TOKEN") != "")
	fmt.Fprintf(os.Stderr, "  HARNESS_API_KEY: %v\n", os.Getenv("HARNESS_API_KEY") != "")
	fmt.Fprintf(os.Stderr, "  GITHUB_TOKEN: %v (len=%d)\n", os.Getenv("GITHUB_TOKEN") != "", len(os.Getenv("GITHUB_TOKEN")))
	fmt.Fprintf(os.Stderr, "  AWS_ACCESS_KEY_ID: %v\n", os.Getenv("AWS_ACCESS_KEY_ID") != "")
	fmt.Fprintf(os.Stderr, "  DEV_MESH_URL: %s\n", os.Getenv("DEV_MESH_URL"))
	fmt.Fprintf(os.Stderr, "  DEV_MESH_API_KEY: %v\n", os.Getenv("DEV_MESH_API_KEY") != "")

	h.logger.Info("Checking environment variables for tokens", map[string]interface{}{
		"has_HARNESS_TOKEN":    os.Getenv("HARNESS_TOKEN") != "",
		"has_HARNESS_API_KEY":  os.Getenv("HARNESS_API_KEY") != "",
		"has_GITHUB_TOKEN":     os.Getenv("GITHUB_TOKEN") != "",
		"has_AWS_ACCESS_KEY":   os.Getenv("AWS_ACCESS_KEY_ID") != "",
		"has_DEV_MESH_URL":     os.Getenv("DEV_MESH_URL") != "",
		"has_DEV_MESH_API_KEY": os.Getenv("DEV_MESH_API_KEY") != "",
	})

	// Extract passthrough authentication from environment variables
	passthroughAuth := h.extractPassthroughAuthFromEnv()

	session := &Session{
		ID:              sessionID,
		ConnectionID:    uuid.New().String(),
		CreatedAt:       time.Now(),
		LastActivity:    time.Now(),
		PassthroughAuth: passthroughAuth,
	}

	h.sessionsMu.Lock()
	h.sessions[sessionID] = session
	h.sessionsMu.Unlock()

	defer func() {
		h.sessionsMu.Lock()
		delete(h.sessions, sessionID)
		h.sessionsMu.Unlock()
	}()

	// Create JSON encoder/decoder for stdio
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	// Message handling loop
	for {
		var msg MCPMessage
		if err := decoder.Decode(&msg); err != nil {
			// EOF is expected when stdin is closed
			if err == io.EOF || strings.Contains(err.Error(), "file already closed") {
				h.logger.Debug("Stdio connection closed", nil)
				break
			}
			h.logger.Error("Failed to decode message from stdin", map[string]interface{}{
				"error": err.Error(),
			})
			// Send error response
			errorResponse := &MCPMessage{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &MCPError{
					Code:    -32700,
					Message: "Parse error",
					Data:    err.Error(),
				},
			}
			_ = encoder.Encode(errorResponse)
			continue
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

		// Check for shutdown
		if msg.Method == "shutdown" {
			if response != nil {
				_ = encoder.Encode(response)
			}
			h.logger.Info("Received shutdown request, exiting stdio mode", nil)
			break
		}

		if response != nil {
			if err := encoder.Encode(response); err != nil {
				h.logger.Error("Failed to write response to stdout", map[string]interface{}{
					"error": err.Error(),
				})
				break
			}
		}
	}
}

// extractPassthroughAuthFromEnv extracts passthrough auth from environment variables
func (h *Handler) extractPassthroughAuthFromEnv() *models.PassthroughAuthBundle {
	bundle := &models.PassthroughAuthBundle{
		Credentials: make(map[string]*models.PassthroughCredential),
	}

	// Check for common service tokens in environment
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		bundle.Credentials["github"] = &models.PassthroughCredential{
			Type:  "bearer",
			Token: token,
		}
		h.logger.Info("Found GitHub passthrough token in environment", map[string]interface{}{
			"token_len": len(token),
		})
	}

	if accessKey := os.Getenv("AWS_ACCESS_KEY_ID"); accessKey != "" {
		if secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY"); secretKey != "" {
			bundle.Credentials["aws"] = &models.PassthroughCredential{
				Type: "aws_signature",
				Properties: map[string]string{
					"access_key_id":     accessKey,
					"secret_access_key": secretKey,
				},
			}
			if region := os.Getenv("AWS_REGION"); region != "" {
				bundle.Credentials["aws"].Properties["region"] = region
			}
			h.logger.Debug("Found AWS passthrough credentials in environment", nil)
		}
	}

	// Check for Harness token (try both possible env var names)
	harnessToken := os.Getenv("HARNESS_TOKEN")
	fmt.Fprintf(os.Stderr, "[edge-mcp] Checking HARNESS_TOKEN: found=%v, len=%d\n", harnessToken != "", len(harnessToken))
	h.logger.Info("Checking HARNESS_TOKEN environment variable", map[string]interface{}{
		"found":     harnessToken != "",
		"token_len": len(harnessToken),
	})
	if harnessToken == "" {
		harnessToken = os.Getenv("HARNESS_API_KEY")
		fmt.Fprintf(os.Stderr, "[edge-mcp] Checking HARNESS_API_KEY: found=%v, len=%d\n", harnessToken != "", len(harnessToken))
		h.logger.Info("Checking HARNESS_API_KEY environment variable", map[string]interface{}{
			"found":     harnessToken != "",
			"token_len": len(harnessToken),
		})
	}
	if harnessToken != "" {
		bundle.Credentials["harness"] = &models.PassthroughCredential{
			Type:  "api_key",
			Token: harnessToken,
		}
		fmt.Fprintf(os.Stderr, "[edge-mcp] Added Harness token to passthrough bundle (len=%d)\n", len(harnessToken))
		h.logger.Info("Found Harness passthrough token in environment", map[string]interface{}{
			"token_len": len(harnessToken),
		})
	} else {
		fmt.Fprintf(os.Stderr, "[edge-mcp] WARNING: No Harness token found in environment\n")
		h.logger.Warn("No Harness token found in environment", nil)
	}

	if len(bundle.Credentials) == 0 {
		return nil
	}

	// Debug: Log what credentials are in the bundle
	fmt.Fprintf(os.Stderr, "[edge-mcp] Passthrough bundle contains %d credentials:\n", len(bundle.Credentials))
	for provider, cred := range bundle.Credentials {
		tokenLen := 0
		if cred.Token != "" {
			tokenLen = len(cred.Token)
		}
		fmt.Fprintf(os.Stderr, "  - %s: type=%s, token_len=%d\n", provider, cred.Type, tokenLen)
	}

	return bundle
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

	// Verify protocol version - accept all known MCP protocol versions
	supportedVersions := []string{
		"2024-11-05", // Original Claude Code version
		"2025-03-26", // March 2025 release with OAuth improvements
		"2025-06-18", // Latest version with structured outputs
	}
	versionSupported := false
	for _, v := range supportedVersions {
		if params.ProtocolVersion == v {
			versionSupported = true
			break
		}
	}
	if !versionSupported {
		return nil, fmt.Errorf("unsupported protocol version: %s (supported: %v)", params.ProtocolVersion, supportedVersions)
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

			// Trigger tool refresh on new connection
			if h.refreshManager != nil {
				go func() {
					h.logger.Debug("Refreshing tools on new connection", map[string]interface{}{
						"client": params.ClientInfo.Name,
					})
					h.refreshManager.OnReconnect(context.Background())
				}()
			}
		}
	}
	h.sessionsMu.Unlock()

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"protocolVersion": params.ProtocolVersion,
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

	// Add passthrough auth to context if available
	h.sessionsMu.RLock()
	session := h.sessions[sessionID]
	var passthroughAuth *models.PassthroughAuthBundle
	if session != nil && session.PassthroughAuth != nil {
		passthroughAuth = session.PassthroughAuth
		// Add passthrough auth to context for remote tool execution
		ctx = context.WithValue(ctx, core.PassthroughAuthKey, passthroughAuth)
		h.logger.Debug("Added passthrough auth to tool execution context", map[string]interface{}{
			"tool":             params.Name,
			"has_passthrough":  true,
			"credential_count": len(passthroughAuth.Credentials),
		})
	}
	h.sessionsMu.RUnlock()

	// Track the request for potential cancellation (only if ID is present)
	if msg.ID != nil {
		h.trackRequest(msg.ID, cancel)
		defer h.untrackRequest(msg.ID)
	}

	// Execute tool with cancellable context (includes passthrough auth if available)
	result, err := h.tools.Execute(ctx, params.Name, params.Arguments)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	// Record execution with Core Platform if connected
	if h.coreClient != nil {
		coreSessionID := ""
		if session != nil {
			coreSessionID = session.CoreSession
		}

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

// extractPassthroughAuth extracts user-specific credentials from request headers and environment
func (h *Handler) extractPassthroughAuth(r *http.Request) *models.PassthroughAuthBundle {
	bundle := &models.PassthroughAuthBundle{
		Credentials:   make(map[string]*models.PassthroughCredential),
		CustomHeaders: make(map[string]string),
	}

	// Extract GitHub Personal Access Token (from header or environment)
	token := r.Header.Get("X-GitHub-Token")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
		if token == "" {
			token = os.Getenv("GITHUB_PAT") // Alternative env var
		}
	}
	if token != "" {
		bundle.Credentials["github"] = &models.PassthroughCredential{
			Type:  "bearer",
			Token: token,
		}
		h.logger.Debug("Found GitHub passthrough token", nil)
	}

	// Extract generic user token (can be used for any service)
	userToken := r.Header.Get("X-User-Token")
	if userToken == "" {
		userToken = os.Getenv("USER_TOKEN")
	}
	if userToken != "" {
		bundle.Credentials["*"] = &models.PassthroughCredential{
			Type:  "bearer",
			Token: userToken,
		}
		h.logger.Debug("Found generic user passthrough token", nil)
	}

	// Extract AWS credentials (from headers or environment)
	accessKey := r.Header.Get("X-AWS-Access-Key")
	if accessKey == "" {
		accessKey = os.Getenv("AWS_ACCESS_KEY_ID")
	}

	if accessKey != "" {
		awsCred := &models.PassthroughCredential{
			Type:       "aws_signature",
			Properties: make(map[string]string),
		}
		awsCred.Properties["access_key"] = accessKey

		secretKey := r.Header.Get("X-AWS-Secret-Key")
		if secretKey == "" {
			secretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
		}
		if secretKey != "" {
			awsCred.Properties["secret_key"] = secretKey
		}

		sessionToken := r.Header.Get("X-AWS-Session-Token")
		if sessionToken == "" {
			sessionToken = os.Getenv("AWS_SESSION_TOKEN")
		}
		if sessionToken != "" {
			awsCred.Properties["session_token"] = sessionToken
		}

		region := r.Header.Get("X-AWS-Region")
		if region == "" {
			region = os.Getenv("AWS_REGION")
			if region == "" {
				region = os.Getenv("AWS_DEFAULT_REGION")
			}
		}
		if region != "" {
			awsCred.Properties["region"] = region
		}

		bundle.Credentials["aws"] = awsCred
		h.logger.Debug("Found AWS passthrough credentials", nil)
	}

	// Extract service-specific tokens (pattern: X-Service-{ServiceName}-Token)
	for key, values := range r.Header {
		if strings.HasPrefix(key, "X-Service-") && strings.HasSuffix(key, "-Token") && len(values) > 0 {
			// Extract service name from header
			// e.g., "X-Service-Slack-Token" -> "slack"
			serviceName := strings.ToLower(
				strings.TrimSuffix(
					strings.TrimPrefix(key, "X-Service-"),
					"-Token",
				),
			)
			bundle.Credentials[serviceName] = &models.PassthroughCredential{
				Type:  "bearer",
				Token: values[0],
			}
			h.logger.Debug("Found service-specific passthrough token", map[string]interface{}{
				"service": serviceName,
			})
		}
	}

	// Extract custom headers for advanced use cases
	for key, values := range r.Header {
		if strings.HasPrefix(key, "X-Custom-Auth-") && len(values) > 0 {
			customKey := strings.TrimPrefix(key, "X-Custom-Auth-")
			bundle.CustomHeaders[customKey] = values[0]
		}
	}

	// Check for common service tokens in environment variables
	commonServices := map[string][]string{
		"slack":     {"SLACK_TOKEN", "SLACK_API_TOKEN"},
		"jira":      {"JIRA_TOKEN", "JIRA_API_TOKEN", "ATLASSIAN_TOKEN"},
		"gitlab":    {"GITLAB_TOKEN", "GITLAB_PAT"},
		"bitbucket": {"BITBUCKET_TOKEN", "BITBUCKET_APP_PASSWORD"},
		"discord":   {"DISCORD_TOKEN", "DISCORD_BOT_TOKEN"},
		"openai":    {"OPENAI_API_KEY", "OPENAI_TOKEN"},
		"anthropic": {"ANTHROPIC_API_KEY", "CLAUDE_API_KEY"},
	}

	for service, envVars := range commonServices {
		// Skip if we already have a credential for this service
		if _, exists := bundle.Credentials[service]; exists {
			continue
		}

		// Check each possible environment variable
		for _, envVar := range envVars {
			if token := os.Getenv(envVar); token != "" {
				bundle.Credentials[service] = &models.PassthroughCredential{
					Type:  "bearer",
					Token: token,
				}
				h.logger.Debug("Found service token from environment", map[string]interface{}{
					"service": service,
					"env_var": envVar,
				})
				break
			}
		}
	}

	// If no passthrough credentials were found, return nil
	if len(bundle.Credentials) == 0 && len(bundle.CustomHeaders) == 0 {
		return nil
	}

	h.logger.Info("Extracted passthrough authentication", map[string]interface{}{
		"credentials_count": len(bundle.Credentials),
		"custom_headers":    len(bundle.CustomHeaders),
	})

	return bundle
}
