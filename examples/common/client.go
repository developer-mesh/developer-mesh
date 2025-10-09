package common

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// MCPClient represents a connection to Edge MCP
type MCPClient struct {
	conn      *websocket.Conn
	ctx       context.Context
	cancel    context.CancelFunc
	messageID int
	mu        sync.Mutex
}

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
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    string                 `json:"category,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// BatchToolCall represents a tool call in a batch request
type BatchToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// BatchResult represents the result of a batch call
type BatchResult struct {
	Results      []BatchToolResult `json:"results"`
	DurationMS   float64           `json:"duration_ms"`
	SuccessCount int               `json:"success_count"`
	ErrorCount   int               `json:"error_count"`
	Parallel     bool              `json:"parallel"`
}

// BatchToolResult represents a single tool result in a batch
type BatchToolResult struct {
	ID         string          `json:"id"`
	Status     string          `json:"status"`
	Result     json.RawMessage `json:"result,omitempty"`
	Error      *MCPError       `json:"error,omitempty"`
	DurationMS float64         `json:"duration_ms"`
	Index      int             `json:"index"`
}

// Config holds the MCP client configuration
type Config struct {
	URL             string
	APIKey          string
	ConnectTimeout  time.Duration
	RequestTimeout  time.Duration
	PassthroughAuth map[string]string // Additional auth headers for passthrough
}

// DefaultConfig returns the default configuration from environment variables
func DefaultConfig() *Config {
	url := os.Getenv("EDGE_MCP_URL")
	if url == "" {
		url = "ws://localhost:8085/ws"
	}

	apiKey := os.Getenv("EDGE_MCP_API_KEY")
	if apiKey == "" {
		apiKey = "devmesh_ab80cbb2438dbb43339c0e3317ab2fc6dd0e046f3b50360df06abb5bae31a210"
	}

	passthroughAuth := make(map[string]string)
	if githubToken := os.Getenv("GITHUB_TOKEN"); githubToken != "" {
		passthroughAuth["X-GitHub-Token"] = githubToken
	}
	if harnessKey := os.Getenv("HARNESS_API_KEY"); harnessKey != "" {
		passthroughAuth["X-Harness-API-Key"] = harnessKey
	}
	if harnessAccount := os.Getenv("HARNESS_ACCOUNT_ID"); harnessAccount != "" {
		passthroughAuth["X-Harness-Account-ID"] = harnessAccount
	}

	return &Config{
		URL:             url,
		APIKey:          apiKey,
		ConnectTimeout:  10 * time.Second,
		RequestTimeout:  30 * time.Second,
		PassthroughAuth: passthroughAuth,
	}
}

// NewClient creates a new MCP client and connects to Edge MCP
func NewClient(config *Config) (*MCPClient, error) {
	if config == nil {
		config = DefaultConfig()
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.ConnectTimeout)
	defer cancel()

	// Build headers
	header := http.Header{}
	header.Set("Authorization", "Bearer "+config.APIKey)
	for k, v := range config.PassthroughAuth {
		header.Set(k, v)
	}

	// Connect to WebSocket
	conn, _, err := websocket.Dial(ctx, config.URL, &websocket.DialOptions{
		HTTPHeader: header,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Edge MCP: %w", err)
	}

	clientCtx, clientCancel := context.WithCancel(context.Background())
	client := &MCPClient{
		conn:      conn,
		ctx:       clientCtx,
		cancel:    clientCancel,
		messageID: 0,
	}

	// Initialize MCP session
	if err := client.Initialize(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to initialize session: %w", err)
	}

	return client, nil
}

// Close closes the MCP client connection
func (c *MCPClient) Close() error {
	c.cancel()
	return c.conn.Close(websocket.StatusNormalClosure, "client closing")
}

// nextMessageID returns the next message ID
func (c *MCPClient) nextMessageID() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messageID++
	return c.messageID
}

// sendMessage sends a JSON-RPC message and waits for the response
func (c *MCPClient) sendMessage(method string, params interface{}) (*MCPMessage, error) {
	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	// Marshal params
	var paramsJSON json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
		paramsJSON = b
	}

	// Send request
	msg := MCPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  method,
		Params:  paramsJSON,
	}

	if err := wsjson.Write(ctx, c.conn, msg); err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Read response
	var response MCPMessage
	if err := wsjson.Read(ctx, c.conn, &response); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if response.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", response.Error.Code, response.Error.Message)
	}

	return &response, nil
}

// Initialize performs the MCP initialization handshake
func (c *MCPClient) Initialize() error {
	// Send initialize
	initParams := map[string]interface{}{
		"protocolVersion": "2025-06-18",
		"clientInfo": map[string]string{
			"name":    "edge-mcp-example-client",
			"version": "1.0.0",
		},
	}

	_, err := c.sendMessage("initialize", initParams)
	if err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	// Send initialized confirmation
	_, err = c.sendMessage("initialized", map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("initialized confirmation failed: %w", err)
	}

	return nil
}

// ListTools retrieves all available tools
func (c *MCPClient) ListTools(ctx context.Context) ([]Tool, error) {
	response, err := c.sendMessage("tools/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools: %w", err)
	}

	return result.Tools, nil
}

// CallTool executes a single tool
func (c *MCPClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (json.RawMessage, error) {
	params := map[string]interface{}{
		"name":      name,
		"arguments": args,
	}

	response, err := c.sendMessage("tools/call", params)
	if err != nil {
		return nil, err
	}

	// Extract content from result
	var result struct {
		Content []struct {
			Type string          `json:"type"`
			Text json.RawMessage `json:"text,omitempty"`
		} `json:"content"`
	}
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse result: %w", err)
	}

	if len(result.Content) == 0 {
		return nil, fmt.Errorf("no content in response")
	}

	return result.Content[0].Text, nil
}

// BatchCallTools executes multiple tools in parallel or sequentially
func (c *MCPClient) BatchCallTools(ctx context.Context, tools []BatchToolCall, parallel bool) (*BatchResult, error) {
	params := map[string]interface{}{
		"tools":    tools,
		"parallel": parallel,
	}

	response, err := c.sendMessage("tools/batch", params)
	if err != nil {
		return nil, err
	}

	var result BatchResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse batch result: %w", err)
	}

	return &result, nil
}

// UpdateContext updates the session context
func (c *MCPClient) UpdateContext(ctx context.Context, contextData map[string]interface{}, merge bool) error {
	params := map[string]interface{}{
		"context": contextData,
		"merge":   merge,
	}

	_, err := c.sendMessage("context/update", params)
	return err
}

// GetContext retrieves the current session context
func (c *MCPClient) GetContext(ctx context.Context) (map[string]interface{}, error) {
	response, err := c.sendMessage("context/get", map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse context: %w", err)
	}

	return result, nil
}
