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
	// Performance optimizations
	toolsCache *ToolsCache
	metrics    observability.MetricsClient
	telemetry  *MCPTelemetry
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

	// First check if this is a custom protocol tool handled by the adapter
	if strings.HasPrefix(params.Name, "agent.") ||
		strings.HasPrefix(params.Name, "workflow.") ||
		strings.HasPrefix(params.Name, "task.") ||
		strings.HasPrefix(params.Name, "context.") {

		// Get circuit breaker for this tool
		breaker := h.circuitBreakers.GetBreaker(params.Name)

		// Execute via protocol adapter with circuit breaker protection
		resultInterface, err := breaker.Call(ctx, params.Name, func() (interface{}, error) {
			return h.protocolAdapter.ExecuteTool(ctx, params.Name, params.Arguments)
		})

		if err != nil {
			h.logger.Error("Adapter tool execution failed", map[string]interface{}{
				"tool":      params.Name,
				"error":     err.Error(),
				"tenant_id": tenantID,
			})
			h.recordTelemetry(fmt.Sprintf("tools_call.%s", params.Name), time.Since(startTime), false)
			return h.sendError(conn, msg.ID, MCPErrorInternalError, fmt.Sprintf("Tool execution failed: %v", err))
		}

		result := resultInterface

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
