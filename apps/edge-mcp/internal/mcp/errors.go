package mcp

import (
	"fmt"
	"time"
)

// ErrorType represents categories of errors
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

// StructuredError provides rich error context
type StructuredError struct {
	Type       ErrorType              `json:"type"`
	Message    string                 `json:"message"`
	Details    string                 `json:"details,omitempty"`
	Operation  string                 `json:"operation,omitempty"`
	RequestID  string                 `json:"request_id,omitempty"`
	Suggestion string                 `json:"suggestion,omitempty"`
	RetryAfter *time.Duration         `json:"retry_after,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	wrapped    error
}

// Error implements the error interface
func (e *StructuredError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Type, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap returns the wrapped error
func (e *StructuredError) Unwrap() error {
	return e.wrapped
}

// NewProtocolError creates a protocol error
func NewProtocolError(operation, message string, details string) *StructuredError {
	return &StructuredError{
		Type:       ErrorTypeProtocol,
		Message:    message,
		Details:    details,
		Operation:  operation,
		Suggestion: "Check that your client implements the MCP protocol correctly",
	}
}

// NewAuthError creates an authentication error
func NewAuthError(message string) *StructuredError {
	return &StructuredError{
		Type:       ErrorTypeAuth,
		Message:    message,
		Operation:  "authenticate",
		Suggestion: "Verify your API key or credentials are correct",
	}
}

// NewToolExecutionError creates a tool execution error
func NewToolExecutionError(toolName string, err error) *StructuredError {
	return &StructuredError{
		Type:       ErrorTypeToolExecution,
		Message:    fmt.Sprintf("Tool '%s' execution failed", toolName),
		Details:    err.Error(),
		Operation:  fmt.Sprintf("tools/call:%s", toolName),
		Suggestion: "Check tool parameters and try again",
		wrapped:    err,
		Metadata: map[string]interface{}{
			"tool": toolName,
		},
	}
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(operation string, timeout time.Duration) *StructuredError {
	return &StructuredError{
		Type:       ErrorTypeTimeout,
		Message:    fmt.Sprintf("Operation timed out after %v", timeout),
		Operation:  operation,
		Suggestion: fmt.Sprintf("Try again with a longer timeout or smaller request"),
		Metadata: map[string]interface{}{
			"timeout_seconds": timeout.Seconds(),
		},
	}
}

// NewRateLimitError creates a rate limit error
func NewRateLimitError(retryAfter time.Duration) *StructuredError {
	return &StructuredError{
		Type:       ErrorTypeRateLimit,
		Message:    "Rate limit exceeded",
		RetryAfter: &retryAfter,
		Suggestion: fmt.Sprintf("Wait %v before retrying", retryAfter),
		Metadata: map[string]interface{}{
			"retry_after_seconds": retryAfter.Seconds(),
		},
	}
}

// NewValidationError creates a validation error
func NewValidationError(field, message string) *StructuredError {
	return &StructuredError{
		Type:       ErrorTypeValidation,
		Message:    fmt.Sprintf("Validation failed for field '%s'", field),
		Details:    message,
		Suggestion: "Check the input parameters against the schema",
		Metadata: map[string]interface{}{
			"field": field,
		},
	}
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource, identifier string) *StructuredError {
	return &StructuredError{
		Type:       ErrorTypeNotFound,
		Message:    fmt.Sprintf("%s not found", resource),
		Details:    fmt.Sprintf("No %s with identifier '%s' exists", resource, identifier),
		Suggestion: fmt.Sprintf("Check that the %s exists and you have access", resource),
		Metadata: map[string]interface{}{
			"resource":   resource,
			"identifier": identifier,
		},
	}
}

// ToMCPError converts to MCP error format
func (e *StructuredError) ToMCPError() *MCPError {
	code := -32603 // Internal error default

	switch e.Type {
	case ErrorTypeProtocol:
		code = -32600 // Invalid request
	case ErrorTypeAuth:
		code = -32001 // Custom auth error
	case ErrorTypeValidation:
		code = -32602 // Invalid params
	case ErrorTypeNotFound:
		code = -32002 // Custom not found
	case ErrorTypeRateLimit:
		code = -32003 // Custom rate limit
	case ErrorTypeTimeout:
		code = -32004 // Custom timeout
	}

	data := map[string]interface{}{
		"type":       e.Type,
		"suggestion": e.Suggestion,
	}

	if e.RetryAfter != nil {
		data["retry_after"] = e.RetryAfter.Seconds()
	}

	if e.Metadata != nil {
		for k, v := range e.Metadata {
			data[k] = v
		}
	}

	return &MCPError{
		Code:    code,
		Message: e.Error(),
		Data:    data,
	}
}

// WithRequestID adds request ID to error
func (e *StructuredError) WithRequestID(requestID string) *StructuredError {
	e.RequestID = requestID
	return e
}

// WithMetadata adds metadata to error
func (e *StructuredError) WithMetadata(key string, value interface{}) *StructuredError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}
