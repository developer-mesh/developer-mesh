package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/core/tool"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ExecutionResult represents the result of a tool execution
type ExecutionResult struct {
	Result        interface{} `json:"result"`
	ExecutionTime int         `json:"execution_time_ms"`
	RetryAttempts int         `json:"retry_attempts"`
}

// ExecutionService handles tool action execution
type ExecutionService struct {
	db           *sqlx.DB
	toolRegistry *ToolRegistry
	retryHandler *RetryHandler
	httpClient   *http.Client
	logger       observability.Logger
}

// NewExecutionService creates a new execution service
func NewExecutionService(
	db *sqlx.DB,
	toolRegistry *ToolRegistry,
	retryHandler *RetryHandler,
	logger observability.Logger,
) *ExecutionService {
	return &ExecutionService{
		db:           db,
		toolRegistry: toolRegistry,
		retryHandler: retryHandler,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// ExecuteToolAction executes a specific action for a tool
func (s *ExecutionService) ExecuteToolAction(
	ctx context.Context,
	tenantID string,
	toolName string,
	actionName string,
	parameters map[string]interface{},
	executedBy string,
) (*ExecutionResult, error) {
	start := time.Now()
	executionID := uuid.New().String()

	// Get tool configuration
	toolConfig, err := s.toolRegistry.GetTool(ctx, tenantID, toolName)
	if err != nil {
		return nil, fmt.Errorf("tool not found: %w", err)
	}

	// Get the specific action
	action, err := s.toolRegistry.GetToolAction(ctx, tenantID, toolName, actionName)
	if err != nil {
		return nil, fmt.Errorf("action not found: %w", err)
	}

	// Validate parameters
	if err := s.validateParameters(action, parameters); err != nil {
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	// Log execution start
	if err := s.logExecutionStart(ctx, executionID, toolConfig.ID, tenantID, actionName, parameters, executedBy); err != nil {
		s.logger.Error("Failed to log execution start", map[string]interface{}{
			"error":        err.Error(),
			"execution_id": executionID,
		})
	}

	// Build the request
	req, err := s.buildRequest(ctx, toolConfig, action, parameters)
	if err != nil {
		s.updateExecutionStatus(ctx, executionID, "failed", err.Error(), 0)
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	// Execute with retry logic
	var result interface{}
	var retryCount int

	policy := toolConfig.RetryPolicy
	if policy == nil {
		// Default policy
		policy = &tool.ToolRetryPolicy{
			MaxAttempts:      3,
			InitialDelay:     1 * time.Second,
			MaxDelay:         30 * time.Second,
			Multiplier:       2.0,
			Jitter:           0.1,
			RetryOnTimeout:   true,
			RetryOnRateLimit: true,
		}
	}

	result, err = s.retryHandler.ExecuteWithRetry(
		ctx,
		toolName,
		actionName,
		policy,
		func() (interface{}, error) {
			retryCount++
			return s.executeHTTPRequest(req.Clone(ctx))
		},
	)

	executionTime := int(time.Since(start).Milliseconds())

	// Update execution log
	status := "success"
	var errorMsg string
	if err != nil {
		status = "failed"
		errorMsg = err.Error()
	}

	s.updateExecutionStatus(ctx, executionID, status, errorMsg, executionTime)

	if err != nil {
		return nil, err
	}

	return &ExecutionResult{
		Result:        result,
		ExecutionTime: executionTime,
		RetryAttempts: retryCount - 1, // Don't count the initial attempt
	}, nil
}

// buildRequest creates an HTTP request for the tool action
func (s *ExecutionService) buildRequest(
	ctx context.Context,
	config *tool.ToolConfig,
	action *tool.DynamicTool,
	parameters map[string]interface{},
) (*http.Request, error) {
	// Build URL
	url := config.BaseURL + action.Path

	// Replace path parameters
	for key, value := range parameters {
		placeholder := fmt.Sprintf("{%s}", key)
		if strings.Contains(url, placeholder) {
			url = strings.Replace(url, placeholder, fmt.Sprintf("%v", value), 1)
			// Remove from parameters so it's not sent in body/query
			delete(parameters, key)
		}
	}

	// Create request based on method
	var req *http.Request
	var err error

	switch strings.ToUpper(action.Method) {
	case "GET", "DELETE":
		req, err = http.NewRequestWithContext(ctx, action.Method, url, nil)
		if err != nil {
			return nil, err
		}

		// Add query parameters
		if len(parameters) > 0 {
			q := req.URL.Query()
			for key, value := range parameters {
				q.Add(key, fmt.Sprintf("%v", value))
			}
			req.URL.RawQuery = q.Encode()
		}

	case "POST", "PUT", "PATCH":
		// Marshal parameters as JSON body
		body, err := json.Marshal(parameters)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}

		req, err = http.NewRequestWithContext(ctx, action.Method, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

	default:
		return nil, fmt.Errorf("unsupported HTTP method: %s", action.Method)
	}

	// Add authentication
	adapter := s.toolRegistry.GetOpenAPIAdapter()
	if err := adapter.AuthenticateRequest(req, config.Credential); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Add standard headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "DevOps-MCP/1.0")

	return req, nil
}

// executeHTTPRequest executes the HTTP request and handles the response
func (s *ExecutionService) executeHTTPRequest(req *http.Request) (interface{}, error) {
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, s.retryHandler.ClassifyHTTPError(nil, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		httpErr := s.retryHandler.ClassifyHTTPError(resp, nil)
		if httpErr != nil {
			return nil, httpErr
		}

		// Try to parse error response
		var errorResp map[string]interface{}
		if err := json.Unmarshal(body, &errorResp); err == nil {
			return nil, fmt.Errorf("API error %d: %v", resp.StatusCode, errorResp)
		}

		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Parse successful response
	if len(body) == 0 {
		return map[string]interface{}{
			"status": "success",
			"code":   resp.StatusCode,
		}, nil
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		// If not JSON, return as string
		return string(body), nil
	}

	return result, nil
}

// validateParameters validates action parameters
func (s *ExecutionService) validateParameters(action *tool.DynamicTool, parameters map[string]interface{}) error {
	if action.Parameters == nil {
		return nil
	}

	// Check required parameters
	for _, required := range action.Parameters.Required {
		if _, exists := parameters[required]; !exists {
			return fmt.Errorf("missing required parameter: %s", required)
		}
	}

	// Validate parameter types
	for name, schema := range action.Parameters.Properties {
		value, exists := parameters[name]
		if !exists {
			continue
		}

		// Basic type validation
		if err := s.validateType(name, value, schema.Type); err != nil {
			return err
		}
	}

	return nil
}

// validateType performs basic type validation
func (s *ExecutionService) validateType(name string, value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("parameter %s must be a string", name)
		}
	case "number", "integer":
		switch value.(type) {
		case int, int32, int64, float32, float64:
			// Valid number types
		default:
			return fmt.Errorf("parameter %s must be a number", name)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("parameter %s must be a boolean", name)
		}
	case "array":
		switch value.(type) {
		case []interface{}, []string, []int:
			// Valid array types
		default:
			return fmt.Errorf("parameter %s must be an array", name)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("parameter %s must be an object", name)
		}
	}

	return nil
}

// logExecutionStart logs the start of a tool execution
func (s *ExecutionService) logExecutionStart(
	ctx context.Context,
	executionID string,
	toolConfigID string,
	tenantID string,
	action string,
	parameters map[string]interface{},
	executedBy string,
) error {
	paramsJSON, err := json.Marshal(parameters)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO tool_executions 
		(id, tool_config_id, tenant_id, action, parameters, status, executed_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err = s.db.ExecContext(ctx, query,
		executionID, toolConfigID, tenantID, action, paramsJSON, "started", executedBy,
	)

	return err
}

// updateExecutionStatus updates the status of a tool execution
func (s *ExecutionService) updateExecutionStatus(
	ctx context.Context,
	executionID string,
	status string,
	errorMsg string,
	responseTime int,
) {
	query := `
		UPDATE tool_executions 
		SET status = $1, error = $2, response_time_ms = $3, executed_at = NOW()
		WHERE id = $4`

	_, err := s.db.ExecContext(ctx, query, status, errorMsg, responseTime, executionID)
	if err != nil {
		s.logger.Error("Failed to update execution status", map[string]interface{}{
			"error":        err.Error(),
			"execution_id": executionID,
		})
	}
}

// GetExecutionHistory retrieves execution history for a tool
func (s *ExecutionService) GetExecutionHistory(
	ctx context.Context,
	tenantID string,
	toolName string,
	limit int,
) ([]map[string]interface{}, error) {
	// Get tool config
	toolConfig, err := s.toolRegistry.GetTool(ctx, tenantID, toolName)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, action, parameters, status, error, response_time_ms, 
		       retry_count, executed_at, executed_by
		FROM tool_executions
		WHERE tool_config_id = $1 AND tenant_id = $2
		ORDER BY executed_at DESC
		LIMIT $3`

	rows, err := s.db.QueryContext(ctx, query, toolConfig.ID, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	results := []map[string]interface{}{}

	for rows.Next() {
		var (
			id           string
			action       string
			parameters   json.RawMessage
			status       string
			errorMsg     sql.NullString
			responseTime sql.NullInt64
			retryCount   int
			executedAt   time.Time
			executedBy   sql.NullString
		)

		err := rows.Scan(&id, &action, &parameters, &status, &errorMsg,
			&responseTime, &retryCount, &executedAt, &executedBy)
		if err != nil {
			continue
		}

		result := map[string]interface{}{
			"id":          id,
			"action":      action,
			"status":      status,
			"retry_count": retryCount,
			"executed_at": executedAt,
		}

		if errorMsg.Valid {
			result["error"] = errorMsg.String
		}
		if responseTime.Valid {
			result["response_time_ms"] = responseTime.Int64
		}
		if executedBy.Valid {
			result["executed_by"] = executedBy.String
		}

		// Parse parameters
		var params map[string]interface{}
		if err := json.Unmarshal(parameters, &params); err == nil {
			result["parameters"] = params
		}

		results = append(results, result)
	}

	return results, nil
}
