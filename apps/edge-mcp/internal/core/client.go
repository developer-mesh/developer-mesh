package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Client connects Edge MCP to Core Platform for state synchronization
// Edge MCP maintains only in-memory state and syncs through this API client
// No direct Redis or database connections are used
type Client struct {
	baseURL    string
	tenantID   string
	edgeMCPID  string
	apiKey     string
	httpClient *http.Client
	logger     observability.Logger

	// Connection status
	connected       bool
	lastError       error
	lastHealthCheck time.Time

	// Circuit breaker for resilience
	failureCount  int
	maxFailures   int
	backoffTime   time.Duration
	nextRetryTime time.Time
	mu            sync.RWMutex
}

// NewClient creates a new Core Platform client
func NewClient(baseURL, apiKey, tenantID, edgeMCPID string, logger observability.Logger) *Client {
	return &Client{
		baseURL:   baseURL,
		apiKey:    apiKey,
		tenantID:  tenantID,
		edgeMCPID: edgeMCPID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:      logger,
		maxFailures: 3,
		backoffTime: 5 * time.Second,
	}
}

// AuthRequest represents authentication request
type AuthRequest struct {
	EdgeMCPID string `json:"edge_mcp_id"`
	APIKey    string `json:"api_key"`
	TenantID  string `json:"tenant_id"`
}

// AuthResponse represents authentication response
type AuthResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Message string `json:"message,omitempty"`
}

// AuthenticateWithCore authenticates with the Core Platform
func (c *Client) AuthenticateWithCore(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check circuit breaker
	if time.Now().Before(c.nextRetryTime) {
		return fmt.Errorf("circuit breaker open, retry after %v", c.nextRetryTime)
	}

	// If no URL configured, work in offline mode
	if c.baseURL == "" {
		c.logger.Info("No Core Platform URL configured, running in offline mode", nil)
		c.connected = false
		return nil
	}

	// Prepare authentication request
	authReq := AuthRequest{
		EdgeMCPID: c.edgeMCPID,
		APIKey:    c.apiKey,
		TenantID:  c.tenantID,
	}

	// Make authentication request
	resp, err := c.doRequest(ctx, "POST", "/api/v1/auth/edge-mcp", authReq)
	if err != nil {
		c.handleFailure(err)
		return fmt.Errorf("authentication failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, string(body))
		c.handleFailure(err)
		return err
	}

	// Parse response
	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		c.handleFailure(err)
		return fmt.Errorf("failed to decode auth response: %w", err)
	}

	if !authResp.Success {
		err := fmt.Errorf("authentication failed: %s", authResp.Message)
		c.handleFailure(err)
		return err
	}

	// Authentication successful
	c.connected = true
	c.lastError = nil
	c.failureCount = 0
	c.lastHealthCheck = time.Now()

	c.logger.Info("Successfully authenticated with Core Platform", map[string]interface{}{
		"tenant_id":   c.tenantID,
		"edge_mcp_id": c.edgeMCPID,
	})

	return nil
}

// handleFailure updates circuit breaker state on failure
func (c *Client) handleFailure(err error) {
	c.lastError = err
	c.failureCount++
	c.connected = false

	if c.failureCount >= c.maxFailures {
		c.nextRetryTime = time.Now().Add(c.backoffTime)
		c.logger.Warn("Circuit breaker opened", map[string]interface{}{
			"failures":    c.failureCount,
			"retry_after": c.nextRetryTime,
			"error":       err.Error(),
		})
	}
}

// FetchRemoteTools fetches available tools from Core Platform
func (c *Client) FetchRemoteTools(ctx context.Context) ([]tools.ToolDefinition, error) {
	if !c.connected {
		return []tools.ToolDefinition{}, nil // Return empty list in offline mode
	}

	// Make request to fetch tools
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/tools?tenant_id=%s&edge_mcp=true", c.tenantID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tools: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch tools, status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var toolsResp struct {
		Tools []struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			Schema      map[string]interface{} `json:"schema"`
		} `json:"tools"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&toolsResp); err != nil {
		return nil, fmt.Errorf("failed to decode tools response: %w", err)
	}

	// Convert to ToolDefinition
	definitions := make([]tools.ToolDefinition, 0, len(toolsResp.Tools))
	for _, t := range toolsResp.Tools {
		definitions = append(definitions, tools.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Schema,
			// Handler will be a proxy handler that calls Core Platform
			Handler: c.createProxyHandler(t.Name),
		})
	}

	c.logger.Info("Fetched remote tools from Core Platform", map[string]interface{}{
		"count": len(definitions),
	})

	return definitions, nil
}

// createProxyHandler creates a handler that proxies to Core Platform
func (c *Client) createProxyHandler(toolName string) tools.ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		if !c.connected {
			return nil, fmt.Errorf("not connected to Core Platform")
		}

		// Execute tool via Core Platform
		payload := map[string]interface{}{
			"tool":      toolName,
			"arguments": args,
		}

		resp, err := c.doRequest(ctx, "POST", "/api/v1/tools/execute", payload)
		if err != nil {
			return nil, fmt.Errorf("failed to execute remote tool: %w", err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				c.logger.Warn("Failed to close response body", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("remote tool execution failed, status %d: %s", resp.StatusCode, string(body))
		}

		var result interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode result: %w", err)
		}

		return result, nil
	}
}

// CreateSession creates a new session on Core Platform
func (c *Client) CreateSession(ctx context.Context, clientName, clientType string) (string, error) {
	// Generate local session ID for offline mode
	sessionID := fmt.Sprintf("session-%d-%s", time.Now().Unix(), c.edgeMCPID)

	if !c.connected {
		return sessionID, nil // Work offline
	}

	// Register session with Core Platform
	payload := map[string]interface{}{
		"session_id":  sessionID,
		"edge_mcp_id": c.edgeMCPID,
		"tenant_id":   c.tenantID,
		"client_name": clientName,
		"client_type": clientType,
	}

	resp, err := c.doRequest(ctx, "POST", "/api/v1/sessions", payload)
	if err != nil {
		// Log error but continue with local session
		c.logger.Warn("Failed to register session with Core Platform", map[string]interface{}{
			"error": err.Error(),
		})
		return sessionID, nil
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	if resp.StatusCode == http.StatusOK {
		c.logger.Info("Session registered with Core Platform", map[string]interface{}{
			"session_id": sessionID,
		})
	}

	return sessionID, nil
}

// CloseSession closes a session on Core Platform
func (c *Client) CloseSession(ctx context.Context, sessionID string) error {
	if !c.connected {
		return nil
	}

	resp, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/api/v1/sessions/%s", sessionID), nil)
	if err != nil {
		return fmt.Errorf("failed to close session: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	return nil
}

// RecordToolExecution records a tool execution on Core Platform
func (c *Client) RecordToolExecution(ctx context.Context, sessionID, toolName string, args json.RawMessage, result interface{}) error {
	if !c.connected {
		return nil // Skip recording in offline mode
	}

	payload := map[string]interface{}{
		"session_id": sessionID,
		"tool_name":  toolName,
		"arguments":  args,
		"result":     result,
		"timestamp":  time.Now().Unix(),
	}

	resp, err := c.doRequest(ctx, "POST", "/api/v1/executions", payload)
	if err != nil {
		// Log but don't fail
		c.logger.Debug("Failed to record tool execution", map[string]interface{}{
			"error": err.Error(),
		})
		return nil
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	return nil
}

// UpdateContext updates context on Core Platform
func (c *Client) UpdateContext(ctx context.Context, sessionID string, contextData map[string]interface{}) error {
	if !c.connected {
		return nil // Skip in offline mode
	}

	payload := map[string]interface{}{
		"context": contextData,
	}

	resp, err := c.doRequest(ctx, "PUT", fmt.Sprintf("/api/v1/context/%s", sessionID), payload)
	if err != nil {
		return fmt.Errorf("failed to update context: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	return nil
}

// GetContext retrieves context from Core Platform
func (c *Client) GetContext(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	if !c.connected {
		return map[string]interface{}{}, nil
	}

	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/context/%s", sessionID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get context: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	var context map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&context); err != nil {
		return nil, fmt.Errorf("failed to decode context: %w", err)
	}

	return context, nil
}

// AppendContext appends to context on Core Platform
func (c *Client) AppendContext(ctx context.Context, sessionID string, appendData map[string]interface{}) error {
	if !c.connected {
		return nil
	}

	payload := map[string]interface{}{
		"append": appendData,
	}

	resp, err := c.doRequest(ctx, "PATCH", fmt.Sprintf("/api/v1/context/%s", sessionID), payload)
	if err != nil {
		return fmt.Errorf("failed to append context: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	return nil
}

// GetStatus returns the connection status
func (c *Client) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"connected":   c.connected,
		"base_url":    c.baseURL,
		"tenant_id":   c.tenantID,
		"edge_mcp_id": c.edgeMCPID,
		"last_error": func() string {
			if c.lastError != nil {
				return c.lastError.Error()
			}
			return ""
		}(),
	}
}

// doRequest performs an authenticated HTTP request
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	return c.httpClient.Do(req)
}
