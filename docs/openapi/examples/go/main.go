package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	edgeMCPURL = "ws://localhost:8082/ws"
	apiKey     = "dev-admin-key-1234567890"
)

// MCPMessage represents a JSON-RPC message
type MCPMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func main() {
	// Create HTTP client with authentication
	header := http.Header{}
	header.Set("Authorization", "Bearer "+apiKey)

	// Connect to Edge MCP WebSocket
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, edgeMCPURL, &websocket.DialOptions{
		HTTPHeader: header,
	})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Initialize MCP session
	if err := initialize(ctx, conn); err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	// List available tools
	tools, err := listTools(ctx, conn)
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}
	fmt.Printf("Available tools: %d\n", len(tools))

	// Execute a tool (example: github_get_repository)
	result, err := callTool(ctx, conn, "github_get_repository", map[string]interface{}{
		"owner": "developer-mesh",
		"repo":  "developer-mesh",
	})
	if err != nil {
		log.Fatalf("Failed to call tool: %v", err)
	}
	fmt.Printf("Tool result: %s\n", result)

	// Batch execute multiple tools
	batchResult, err := batchCallTools(ctx, conn, []BatchToolCall{
		{
			ID:   "call-1",
			Name: "github_list_issues",
			Arguments: map[string]interface{}{
				"owner": "developer-mesh",
				"repo":  "developer-mesh",
				"state": "open",
			},
		},
		{
			ID:   "call-2",
			Name: "github_list_pull_requests",
			Arguments: map[string]interface{}{
				"owner": "developer-mesh",
				"repo":  "developer-mesh",
				"state": "open",
			},
		},
	}, true)
	if err != nil {
		log.Fatalf("Failed to batch call tools: %v", err)
	}
	fmt.Printf("Batch result: %d tools executed, %d succeeded, %d failed\n",
		len(batchResult.Results), batchResult.SuccessCount, batchResult.ErrorCount)
}

// initialize sends the initialize message
func initialize(ctx context.Context, conn *websocket.Conn) error {
	initMsg := MCPMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: json.RawMessage(`{
			"protocolVersion": "2025-06-18",
			"clientInfo": {
				"name": "go-example-client",
				"version": "1.0.0"
			}
		}`),
	}

	if err := wsjson.Write(ctx, conn, initMsg); err != nil {
		return fmt.Errorf("failed to send initialize: %w", err)
	}

	var response MCPMessage
	if err := wsjson.Read(ctx, conn, &response); err != nil {
		return fmt.Errorf("failed to read initialize response: %w", err)
	}

	if response.Error != nil {
		return fmt.Errorf("initialize error: %s", response.Error.Message)
	}

	// Send initialized confirmation
	confirmedMsg := MCPMessage{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "initialized",
		Params:  json.RawMessage(`{}`),
	}

	if err := wsjson.Write(ctx, conn, confirmedMsg); err != nil {
		return fmt.Errorf("failed to send initialized: %w", err)
	}

	return nil
}

// listTools retrieves all available tools
func listTools(ctx context.Context, conn *websocket.Conn) ([]map[string]interface{}, error) {
	msg := MCPMessage{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/list",
		Params:  json.RawMessage(`{}`),
	}

	if err := wsjson.Write(ctx, conn, msg); err != nil {
		return nil, fmt.Errorf("failed to send tools/list: %w", err)
	}

	var response MCPMessage
	if err := wsjson.Read(ctx, conn, &response); err != nil {
		return nil, fmt.Errorf("failed to read tools/list response: %w", err)
	}

	if response.Error != nil {
		return nil, fmt.Errorf("tools/list error: %s", response.Error.Message)
	}

	var result struct {
		Tools []map[string]interface{} `json:"tools"`
	}
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools: %w", err)
	}

	return result.Tools, nil
}

// callTool executes a single tool
func callTool(ctx context.Context, conn *websocket.Conn, name string, args map[string]interface{}) (string, error) {
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("failed to marshal arguments: %w", err)
	}

	params, err := json.Marshal(map[string]interface{}{
		"name":      name,
		"arguments": json.RawMessage(argsJSON),
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal params: %w", err)
	}

	msg := MCPMessage{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  json.RawMessage(params),
	}

	if err := wsjson.Write(ctx, conn, msg); err != nil {
		return "", fmt.Errorf("failed to send tools/call: %w", err)
	}

	var response MCPMessage
	if err := wsjson.Read(ctx, conn, &response); err != nil {
		return "", fmt.Errorf("failed to read tools/call response: %w", err)
	}

	if response.Error != nil {
		return "", fmt.Errorf("tools/call error: %s", response.Error.Message)
	}

	return string(response.Result), nil
}

// BatchToolCall represents a tool call in a batch request
type BatchToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// BatchResult represents the result of a batch call
type BatchResult struct {
	Results      []map[string]interface{} `json:"results"`
	DurationMS   float64                  `json:"duration_ms"`
	SuccessCount int                      `json:"success_count"`
	ErrorCount   int                      `json:"error_count"`
	Parallel     bool                     `json:"parallel"`
}

// batchCallTools executes multiple tools in parallel or sequentially
func batchCallTools(ctx context.Context, conn *websocket.Conn, tools []BatchToolCall, parallel bool) (*BatchResult, error) {
	params, err := json.Marshal(map[string]interface{}{
		"tools":    tools,
		"parallel": parallel,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	msg := MCPMessage{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/batch",
		Params:  json.RawMessage(params),
	}

	if err := wsjson.Write(ctx, conn, msg); err != nil {
		return nil, fmt.Errorf("failed to send tools/batch: %w", err)
	}

	var response MCPMessage
	if err := wsjson.Read(ctx, conn, &response); err != nil {
		return nil, fmt.Errorf("failed to read tools/batch response: %w", err)
	}

	if response.Error != nil {
		return nil, fmt.Errorf("tools/batch error: %s", response.Error.Message)
	}

	var result BatchResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse batch result: %w", err)
	}

	return &result, nil
}
