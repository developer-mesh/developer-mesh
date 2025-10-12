package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tracing"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/utils"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// contextKey is a type for context keys to avoid collisions
type contextKey string

// PassthroughAuthKey is the context key for passthrough authentication
const PassthroughAuthKey contextKey = "passthrough-auth"

// getMapKeys returns the keys of a string map as a slice
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// normalizeClientType maps various client type inputs to allowed database values
func normalizeClientType(clientName, clientType string) string {
	// Database accepts: 'claude-code', 'ide', 'agent', 'cli'

	// Check client name first (Claude Code specific detection)
	nameLower := strings.ToLower(clientName)
	if strings.Contains(nameLower, "claude") && strings.Contains(nameLower, "code") {
		return "claude-code"
	}
	if strings.Contains(nameLower, "claude-code") {
		return "claude-code"
	}

	// Normalize client type
	typeLower := strings.ToLower(clientType)
	switch typeLower {
	case "claude-code", "claude_code", "claudecode":
		return "claude-code"
	case "ide", "editor", "vscode", "cursor":
		return "ide"
	case "agent", "ai", "bot":
		return "agent"
	case "cli", "terminal", "command-line", "commandline":
		return "cli"
	case "":
		// Empty type - try to infer from name
		if strings.Contains(nameLower, "ide") || strings.Contains(nameLower, "editor") {
			return "ide"
		}
		if strings.Contains(nameLower, "agent") || strings.Contains(nameLower, "bot") {
			return "agent"
		}
		// Default to CLI for empty type
		return "cli"
	default:
		// Unknown type - default to CLI
		return "cli"
	}
}

// Client connects Edge MCP to Core Platform for state synchronization
// Edge MCP maintains only in-memory state and syncs through this API client
// No direct Redis or database connections are used
type Client struct {
	baseURL     string
	tenantID    string // Retrieved from Core Platform after authentication
	edgeMCPID   string
	apiKey      string
	httpClient  *http.Client
	logger      observability.Logger
	retryConfig *utils.RetryConfig // Retry configuration for external calls
	spanHelper  *tracing.SpanHelper

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

	// Tool ID mapping for execution
	toolIDMap map[string]string // Maps tool name to tool ID

	// Session and context tracking
	currentSessionID string // Current session ID
	currentContextID string // Current context ID (linked to session)
}

// NewClient creates a new Core Platform client
func NewClient(baseURL, apiKey, edgeMCPID string, logger observability.Logger, tracerProvider *tracing.TracerProvider) *Client {
	var spanHelper *tracing.SpanHelper
	if tracerProvider != nil {
		spanHelper = tracing.NewSpanHelper(tracerProvider)
	}

	// Setup default retry configuration
	retryConfig := &utils.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2,
		JitterFactor: 0.1,
		RetryIf: func(err error) bool {
			// Retry on network errors and specific HTTP status codes
			var httpErr utils.HTTPError
			if errors.As(err, &httpErr) {
				return httpErr.IsRetryable()
			}

			// Retry on timeout
			if errors.Is(err, context.DeadlineExceeded) {
				return true
			}

			// Retry on temporary network errors
			var netErr utils.NetworkError
			return errors.As(err, &netErr)
		},
	}

	return &Client{
		baseURL:   baseURL,
		apiKey:    apiKey,
		edgeMCPID: edgeMCPID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:      logger,
		retryConfig: retryConfig,
		spanHelper:  spanHelper,
		maxFailures: 3,
		backoffTime: 5 * time.Second,
		toolIDMap:   make(map[string]string),
	}
}

// AuthRequest represents authentication request
type AuthRequest struct {
	EdgeMCPID string `json:"edge_mcp_id"`
	APIKey    string `json:"api_key"`
	// TenantID not needed - Core Platform extracts it from APIKey
}

// AuthResponse represents authentication response
type AuthResponse struct {
	Success  bool   `json:"success"`
	Token    string `json:"token,omitempty"`
	Message  string `json:"message,omitempty"`
	TenantID string `json:"tenant_id,omitempty"` // Core Platform returns the tenant_id
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

	// Authentication successful - store the tenant_id from response
	if authResp.TenantID != "" {
		c.tenantID = authResp.TenantID
	}

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

// FetchRemoteTools fetches available tools from Core Platform with retry logic
func (c *Client) FetchRemoteTools(ctx context.Context) ([]tools.ToolDefinition, error) {
	if !c.connected {
		return []tools.ToolDefinition{}, nil // Return empty list in offline mode
	}

	var result []tools.ToolDefinition

	// Use retry logic for fetching tools
	retryResult, err := utils.RetryWithBackoff(ctx, c.retryConfig, func() error {
		// Make request to fetch tools
		resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/tools?tenant_id=%s&edge_mcp=true", c.tenantID), nil)
		if err != nil {
			// Wrap as network error to make it retryable
			return utils.NetworkError{Message: err.Error()}
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
			// Return HTTPError to enable proper retry logic
			return utils.HTTPError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("Failed to fetch tools: %s", string(body)),
			}
		}

		// Parse response
		var toolsResp struct {
			Tools []struct {
				ID          string                 `json:"id"` // Tool ID for execution
				Name        string                 `json:"tool_name"`
				DisplayName string                 `json:"display_name"`
				Description string                 `json:"description"`
				Config      map[string]interface{} `json:"config"`
				Schema      map[string]interface{} `json:"schema"` // Direct schema field (if present)
			} `json:"tools"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&toolsResp); err != nil {
			return fmt.Errorf("failed to decode tools response: %w", err)
		}

		// Convert to ToolDefinition
		definitions := make([]tools.ToolDefinition, 0, len(toolsResp.Tools))
		for _, t := range toolsResp.Tools {
			description := t.Description
			if description == "" && t.DisplayName != "" {
				description = t.DisplayName + " integration"
			}

			// Try to get schema from config first (where we store generated schemas)
			// then fall back to direct schema field
			var inputSchema map[string]interface{}
			if t.Config != nil {
				if configSchema, ok := t.Config["schema"].(map[string]interface{}); ok {
					inputSchema = configSchema
				}
			}
			if inputSchema == nil && t.Schema != nil {
				inputSchema = t.Schema
			}

			// Store the tool ID mapping for execution
			c.mu.Lock()
			c.toolIDMap[t.Name] = t.ID
			c.mu.Unlock()

			c.logger.Debug("Registering tool with ID mapping", map[string]interface{}{
				"tool_name": t.Name,
				"tool_id":   t.ID,
			})

			definitions = append(definitions, tools.ToolDefinition{
				Name:        t.Name,
				Description: description,
				InputSchema: inputSchema,
				// Handler will be a proxy handler that calls Core Platform
				// Note: Passthrough auth will be injected at execution time
				Handler: c.createProxyHandler(t.Name, t.ID),
			})
		}

		result = definitions
		return nil
	})

	if err != nil {
		c.logger.Error("Failed to fetch remote tools after retries", map[string]interface{}{
			"attempts":       retryResult.Attempts,
			"total_duration": retryResult.TotalDuration.Seconds(),
			"error":          err.Error(),
		})
		return nil, err
	}

	c.logger.Info("Successfully fetched remote tools", map[string]interface{}{
		"count":    len(result),
		"attempts": retryResult.Attempts,
	})

	return result, nil
}

// createProxyHandler creates a handler that proxies to Core Platform
func (c *Client) createProxyHandler(toolName string, toolID string) tools.ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		if !c.connected {
			return nil, fmt.Errorf("not connected to Core Platform")
		}

		// Extract passthrough auth from context if available
		var passthroughAuth *models.PassthroughAuthBundle
		if auth := ctx.Value(PassthroughAuthKey); auth != nil {
			passthroughAuth, _ = auth.(*models.PassthroughAuthBundle)
		}

		// Parse arguments to extract action and parameters
		var parsedArgs map[string]interface{}
		if err := json.Unmarshal(args, &parsedArgs); err != nil {
			c.logger.Warn("Failed to parse arguments, using empty parameters", map[string]interface{}{
				"error": err.Error(),
			})
			parsedArgs = make(map[string]interface{})
		}

		// For organization tools, the action might be derived from the tool name
		// Tool names are in format: github-devmesh_repos_list or github-devmesh-repos-list
		var action string
		var parameters map[string]interface{}

		// Handle Harness tools specially
		if strings.HasPrefix(toolName, "harness_") {
			// Extract the operation from Harness tool name
			// e.g., "harness_pipelines_list" -> "pipelines/list"
			operationName := strings.TrimPrefix(toolName, "harness_")

			// The REST API will handle the conversion to OpenAPI operation IDs
			// We send it in the format the REST API expects
			action = operationName

			c.logger.Debug("Extracted Harness operation", map[string]interface{}{
				"tool_name": toolName,
				"operation": action,
			})
		} else if strings.Contains(toolName, "_") || strings.Contains(toolName, "-") {
			// Try to extract the operation part from the tool name
			// Look for common operation prefixes (GitHub tools)
			operationPrefixes := []string{"repos_", "issues_", "pulls_", "actions_", "releases_", "repos-", "issues-", "pulls-", "actions-", "releases-"}
			for _, prefix := range operationPrefixes {
				if idx := strings.LastIndex(toolName, prefix); idx != -1 {
					action = toolName[idx:]
					// For GitHub Actions operations, preserve the full action with prefix
					// The provider will handle the normalization
					if strings.HasPrefix(action, "actions-") || strings.HasPrefix(action, "actions_") {
						// Keep the full action including the "actions-" prefix
						// Just normalize underscores to hyphens if present
						if strings.HasPrefix(action, "actions_") {
							action = strings.Replace(action, "actions_", "actions-", 1)
						}
						// Keep action as-is: "actions-list-workflows" or "actions-trigger-workflow"
					} else {
						// For other operations, convert to slash format
						// e.g., "repos-list" -> "repos/list"
						action = strings.Replace(action, "-", "/", 1)
						action = strings.Replace(action, "_", "/", 1)
					}
					break
				}
			}
		}

		// If action wasn't extracted from tool name, check arguments
		if action == "" {
			if actionArg, ok := parsedArgs["action"].(string); ok {
				action = actionArg
				delete(parsedArgs, "action")
			}
		}

		// Handle parameters - merge top-level GitHub fields with nested parameters
		if params, ok := parsedArgs["parameters"].(map[string]interface{}); ok {
			parameters = params
			// For GitHub tools, also include owner, repo, and other top-level fields
			// These are passed at the top level by MCP but need to be in parameters for REST API
			for key, value := range parsedArgs {
				if key != "parameters" && key != "action" {
					// Include fields like owner, repo, branch, pull_number, issue_number
					if key == "owner" || key == "repo" || key == "branch" ||
						key == "pull_number" || key == "issue_number" || key == "ref" {
						parameters[key] = value
					}
				}
			}
		} else {
			// Use all arguments as parameters (action was already removed if present)
			parameters = parsedArgs
		}

		// Build the payload
		payload := map[string]interface{}{
			"parameters": parameters,
		}

		// Only add action if we have one
		if action != "" {
			payload["action"] = action
			c.logger.Debug("Executing tool with extracted action", map[string]interface{}{
				"tool":       toolName,
				"tool_id":    toolID,
				"action":     action,
				"param_keys": getMapKeys(parameters),
			})
		} else {
			c.logger.Debug("Executing tool without explicit action", map[string]interface{}{
				"tool":       toolName,
				"tool_id":    toolID,
				"param_keys": getMapKeys(parameters),
			})
		}

		// Include passthrough auth if available
		if passthroughAuth != nil {
			payload["passthrough_auth"] = passthroughAuth

			// Log detailed passthrough auth info
			authInfo := map[string]interface{}{
				"tool":             toolName,
				"has_passthrough":  true,
				"credential_count": len(passthroughAuth.Credentials),
			}

			// Log each credential provider and token length (without exposing tokens)
			for provider, cred := range passthroughAuth.Credentials {
				if cred != nil {
					authInfo[provider+"_token_len"] = len(cred.Token)
				}
			}

			c.logger.Info("Including passthrough auth in tool execution", authInfo)
		}

		// Use the correct endpoint with tool ID
		endpoint := fmt.Sprintf("/api/v1/tools/%s/execute", toolID)
		c.logger.Debug("Executing tool via REST API", map[string]interface{}{
			"tool_name": toolName,
			"tool_id":   toolID,
			"endpoint":  endpoint,
		})
		resp, err := c.doRequest(ctx, "POST", endpoint, payload)
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

	// Normalize client_type to match database constraints
	// Database accepts: 'claude-code', 'ide', 'agent', 'cli'
	normalizedType := normalizeClientType(clientName, clientType)

	// Register session with Core Platform
	payload := map[string]interface{}{
		"session_id":  sessionID,
		"edge_mcp_id": c.edgeMCPID,
		"tenant_id":   c.tenantID,
		"client_name": clientName,
		"client_type": normalizedType,
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

	// Parse response to extract context_id
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		var sessionResp struct {
			SessionID string  `json:"session_id"`
			ContextID *string `json:"context_id"` // UUID from server, can be null
		}

		if err := json.NewDecoder(resp.Body).Decode(&sessionResp); err != nil {
			c.logger.Warn("Failed to parse session response", map[string]interface{}{
				"error": err.Error(),
			})
			return sessionID, nil
		}

		// Store session and context IDs
		c.mu.Lock()
		c.currentSessionID = sessionResp.SessionID
		if sessionResp.ContextID != nil {
			c.currentContextID = *sessionResp.ContextID
		}
		c.mu.Unlock()

		contextID := ""
		if sessionResp.ContextID != nil {
			contextID = *sessionResp.ContextID
		}

		c.logger.Info("Session registered with Core Platform", map[string]interface{}{
			"session_id": sessionResp.SessionID,
			"context_id": contextID,
			"linked":     contextID != "",
		})

		// Return the session_id from server (should match what we generated)
		return sessionResp.SessionID, nil
	}

	c.logger.Warn("Unexpected status code from session creation", map[string]interface{}{
		"status_code": resp.StatusCode,
	})

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
func (c *Client) UpdateContext(ctx context.Context, contextID string, contextData map[string]interface{}) error {
	if !c.connected {
		return nil // Skip in offline mode
	}

	// Transform contextData to match REST API expected format
	// API expects: {"content": [{"role": "...", "content": "...", "type": "text"}]}
	var payload map[string]interface{}

	// Check if contextData has "content" and "role" fields (from context_provider)
	if content, hasContent := contextData["content"].(string); hasContent {
		role := "user" // Default role
		if r, hasRole := contextData["role"].(string); hasRole {
			role = r
		}

		// Build proper ContextItem structure
		contextItem := map[string]interface{}{
			"role":    role,
			"content": content,
			"type":    "text",
		}

		payload = map[string]interface{}{
			"content": []interface{}{contextItem},
		}
	} else {
		// If not in expected format, wrap as-is
		payload = contextData
	}

	resp, err := c.doRequest(ctx, "PUT", fmt.Sprintf("/api/v1/contexts/%s", contextID), payload)
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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("context update failed, status %d: %s", resp.StatusCode, string(body))
	}

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

// SearchContext performs semantic search within a context
func (c *Client) SearchContext(ctx context.Context, contextID string, query string, limit int) ([]interface{}, error) {
	if !c.connected {
		return nil, fmt.Errorf("not connected to Core Platform")
	}

	payload := map[string]interface{}{
		"query": query,
	}

	if limit > 0 {
		payload["limit"] = limit
	}

	resp, err := c.doRequest(ctx, "POST", fmt.Sprintf("/api/v1/contexts/%s/search", contextID), payload)
	if err != nil {
		return nil, fmt.Errorf("failed to search context: %w", err)
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
		return nil, fmt.Errorf("context search failed, status %d: %s", resp.StatusCode, string(body))
	}

	var searchResp struct {
		ContextID string        `json:"context_id"`
		Query     string        `json:"query"`
		Results   []interface{} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	return searchResp.Results, nil
}

// GetStatus returns the connection status
func (c *Client) GetStatus() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"connected":   c.connected,
		"base_url":    c.baseURL,
		"tenant_id":   c.tenantID,
		"edge_mcp_id": c.edgeMCPID,
		"session_id":  c.currentSessionID,
		"context_id":  c.currentContextID,
		"last_error": func() string {
			if c.lastError != nil {
				return c.lastError.Error()
			}
			return ""
		}(),
	}
}

// GetCurrentContextID returns the current context ID (linked to the session)
func (c *Client) GetCurrentContextID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentContextID
}

// doRequest performs an authenticated HTTP request
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	// Start distributed tracing span for Core Platform call
	var span trace.Span
	url := c.baseURL + path
	if c.spanHelper != nil {
		ctx, span = c.spanHelper.StartCorePlatformCallSpan(ctx, method, url)
		defer span.End()
	}

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			if span != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to marshal request body")
			}
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to create request")
		}
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "HTTP request failed")
		}
		return nil, err
	}

	// Record HTTP status in span
	if span != nil && c.spanHelper != nil {
		c.spanHelper.RecordHTTPStatus(ctx, resp.StatusCode)
	}

	return resp, nil
}
