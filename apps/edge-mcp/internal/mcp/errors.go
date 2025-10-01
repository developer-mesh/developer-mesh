package mcp

import (
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/models"
)

// Global error templates instance
var errorTemplates = NewErrorTemplates()

// Legacy type aliases for backward compatibility
// These map to the new error codes from pkg/models
type ErrorType string

const (
	ErrorTypeProtocol      ErrorType = "PROTOCOL_ERROR"
	ErrorTypeAuth          ErrorType = "AUTH_ERROR"
	ErrorTypeToolExecution ErrorType = "TOOL_EXECUTION_ERROR"
	ErrorTypeTimeout       ErrorType = "TIMEOUT_ERROR"
	ErrorTypeRateLimit     ErrorType = "RATE_LIMIT_ERROR"
	ErrorTypeNetwork       ErrorType = "NETWORK_ERROR"
	ErrorTypeValidation    ErrorType = "VALIDATION_ERROR"
	ErrorTypeNotFound      ErrorType = "NOT_FOUND"
	ErrorTypeInternal      ErrorType = "INTERNAL_ERROR"
)

// ToMCPError converts ErrorResponse to MCP protocol error format
func ToMCPError(err *models.ErrorResponse) *MCPError {
	code := -32603 // Internal error default

	// Map error codes to JSON-RPC error codes
	switch err.Code {
	case models.ErrorCodeProtocolError, models.ErrorCodeVersionMismatch:
		code = -32600 // Invalid request
	case models.ErrorCodeParsingError:
		code = -32700 // Parse error
	case models.ErrorCodeValidation, models.ErrorCodeInvalidInput, models.ErrorCodeMissingParameter:
		code = -32602 // Invalid params
	case models.ErrorCodeAuthFailed, models.ErrorCodeUnauthorized, models.ErrorCodeTokenExpired:
		code = -32001 // Custom auth error
	case models.ErrorCodeNotFound:
		code = -32002 // Custom not found
	case models.ErrorCodeRateLimit, models.ErrorCodeQuotaExceeded:
		code = -32003 // Custom rate limit
	case models.ErrorCodeTimeout:
		code = -32004 // Custom timeout
	case models.ErrorCodeInvalidOperation:
		code = -32601 // Method not found
	}

	// Build data payload with AI-friendly information
	data := map[string]interface{}{
		"code":     string(err.Code),
		"category": string(err.Category),
		"severity": string(err.Severity),
	}

	if err.Suggestion != "" {
		data["suggestion"] = err.Suggestion
	}

	if len(err.RecoverySteps) > 0 {
		data["recovery_steps"] = err.RecoverySteps
	}

	if len(err.AlternativeTools) > 0 {
		data["alternative_tools"] = err.AlternativeTools
	}

	if err.RetryAfter != nil {
		data["retry_after_seconds"] = err.RetryAfter.Seconds()
	}

	if err.RateLimitInfo != nil {
		data["rate_limit"] = map[string]interface{}{
			"limit":     err.RateLimitInfo.Limit,
			"remaining": err.RateLimitInfo.Remaining,
			"reset":     err.RateLimitInfo.Reset,
		}
	}

	if err.RetryStrategy != nil {
		data["retry_strategy"] = map[string]interface{}{
			"retryable":     err.RetryStrategy.Retryable,
			"max_attempts":  err.RetryStrategy.MaxAttempts,
			"backoff_type":  err.RetryStrategy.BackoffType,
			"initial_delay": err.RetryStrategy.InitialDelay.Seconds(),
		}
	}

	if err.Resource != nil {
		data["resource"] = map[string]interface{}{
			"type": err.Resource.Type,
			"id":   err.Resource.ID,
			"name": err.Resource.Name,
		}
	}

	if err.Documentation != "" {
		data["documentation"] = err.Documentation
	}

	// Include metadata
	if err.Metadata != nil {
		for k, v := range err.Metadata {
			data[k] = v
		}
	}

	return &MCPError{
		Code:    code,
		Message: err.Error(),
		Data:    data,
	}
}

// Convenience functions that use error templates
// These maintain backward compatibility while using the new semantic errors

// NewProtocolError creates a protocol error
func NewProtocolError(operation, message string, details string) *models.ErrorResponse {
	return models.NewErrorResponse(models.ErrorCodeProtocolError, message).
		WithDetails(details).
		WithOperation(operation).
		WithSuggestion("Check that your client implements the MCP protocol correctly").
		WithDocumentation("https://developer-mesh.io/docs/mcp-protocol")
}

// NewAuthError creates an authentication error
func NewAuthError(message string) *models.ErrorResponse {
	return errorTemplates.AuthenticationFailed(message)
}

// NewToolExecutionError creates a tool execution error
func NewToolExecutionError(toolName string, err error) *models.ErrorResponse {
	return errorTemplates.ToolExecutionFailed(toolName, err, nil)
}

// NewToolExecutionErrorWithAlternatives creates a tool execution error with alternative tools
func NewToolExecutionErrorWithAlternatives(toolName string, err error, alternatives []string) *models.ErrorResponse {
	return errorTemplates.ToolExecutionFailed(toolName, err, alternatives)
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(operation string, timeout fmt.Stringer) *models.ErrorResponse {
	// Convert fmt.Stringer to time.Duration if possible
	// For backward compatibility, we'll parse the string
	var duration interface{}
	if timeout != nil {
		duration = timeout.String()
	}

	return models.NewErrorResponse(models.ErrorCodeTimeout, fmt.Sprintf("Operation '%s' timed out", operation)).
		WithOperation(operation).
		WithMetadata("timeout", duration).
		WithSuggestion("Try again with a longer timeout or reduce the request size")
}

// NewRateLimitError creates a rate limit error
func NewRateLimitError(retryAfter fmt.Stringer) *models.ErrorResponse {
	// For backward compatibility, we create a basic rate limit error
	return models.NewErrorResponse(models.ErrorCodeRateLimit, "Rate limit exceeded").
		WithSuggestion(fmt.Sprintf("Wait %v before retrying", retryAfter))
}

// NewValidationError creates a validation error
func NewValidationError(field, message string) *models.ErrorResponse {
	return models.NewValidationError(field, message)
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource, identifier string) *models.ErrorResponse {
	return models.NewNotFoundError(resource, identifier)
}
