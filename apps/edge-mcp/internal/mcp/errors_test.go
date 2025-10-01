package mcp

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStructuredError_Creation(t *testing.T) {
	tests := []struct {
		name     string
		err      *StructuredError
		expected string
	}{
		{
			name:     "Protocol Error",
			err:      NewProtocolError("initialize", "Invalid version", "Version 1.0 not supported"),
			expected: "[PROTOCOL_ERROR] Invalid version: Version 1.0 not supported",
		},
		{
			name:     "Auth Error",
			err:      NewAuthError("Invalid API key"),
			expected: "[AUTH_ERROR] Invalid API key",
		},
		{
			name:     "Tool Execution Error",
			err:      NewToolExecutionError("github_list_repos", errors.New("connection timeout")),
			expected: "[TOOL_EXECUTION_ERROR] Tool 'github_list_repos' execution failed: connection timeout",
		},
		{
			name:     "Not Found Error",
			err:      NewNotFoundError("repository", "my-repo"),
			expected: "[NOT_FOUND] repository not found: No repository with identifier 'my-repo' exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
			assert.NotEmpty(t, tt.err.Suggestion)
		})
	}
}

func TestStructuredError_ToMCPError(t *testing.T) {
	// Test protocol error conversion
	protocolErr := NewProtocolError("test", "Test error", "Details")
	mcpErr := protocolErr.ToMCPError()
	assert.Equal(t, -32600, mcpErr.Code)
	assert.Contains(t, mcpErr.Message, "PROTOCOL_ERROR")

	// Test rate limit error with retry after
	retryAfter := 60 * time.Second
	rateLimitErr := NewRateLimitError(retryAfter)
	mcpErr = rateLimitErr.ToMCPError()
	assert.Equal(t, -32003, mcpErr.Code)
	data := mcpErr.Data.(map[string]interface{})
	assert.Equal(t, 60.0, data["retry_after"])
}

func TestStructuredError_WithMetadata(t *testing.T) {
	err := NewToolExecutionError("test_tool", errors.New("failed")).
		WithRequestID("req-123").
		WithMetadata("attempt", 3).
		WithMetadata("duration_ms", 1500)

	assert.Equal(t, "req-123", err.RequestID)
	assert.Equal(t, 3, err.Metadata["attempt"])
	assert.Equal(t, 1500, err.Metadata["duration_ms"])

	// Test MCP conversion includes metadata
	mcpErr := err.ToMCPError()
	data := mcpErr.Data.(map[string]interface{})
	assert.Equal(t, 3, data["attempt"])
	assert.Equal(t, 1500, data["duration_ms"])
}

func TestStructuredError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	structuredErr := NewToolExecutionError("tool", originalErr)

	unwrapped := structuredErr.Unwrap()
	assert.Equal(t, originalErr, unwrapped)

	// Test with errors.Is
	assert.True(t, errors.Is(structuredErr, originalErr))
}

func TestStructuredError_ErrorTypes(t *testing.T) {
	// Test timeout error
	timeoutErr := NewTimeoutError("database_query", 30*time.Second)
	assert.Equal(t, ErrorTypeTimeout, timeoutErr.Type)
	assert.Contains(t, timeoutErr.Error(), "timed out after 30s")
	assert.NotNil(t, timeoutErr.Metadata["timeout_seconds"])

	// Test validation error
	validationErr := NewValidationError("email", "Invalid email format")
	assert.Equal(t, ErrorTypeValidation, validationErr.Type)
	assert.Contains(t, validationErr.Error(), "email")
	assert.Contains(t, validationErr.Details, "Invalid email format")

	// Test MCP error codes
	testCases := []struct {
		err  *StructuredError
		code int
	}{
		{NewProtocolError("test", "msg", "details"), -32600},
		{NewAuthError("msg"), -32001},
		{NewValidationError("field", "msg"), -32602},
		{NewNotFoundError("resource", "id"), -32002},
		{NewRateLimitError(1 * time.Second), -32003},
		{NewTimeoutError("op", 1*time.Second), -32004},
	}

	for _, tc := range testCases {
		mcpErr := tc.err.ToMCPError()
		assert.Equal(t, tc.code, mcpErr.Code,
			"Error type %s should have code %d", tc.err.Type, tc.code)
	}
}

func TestStructuredError_RateLimitWithRetryAfter(t *testing.T) {
	retryAfter := 30 * time.Second
	err := NewRateLimitError(retryAfter)

	assert.NotNil(t, err.RetryAfter)
	assert.Equal(t, retryAfter, *err.RetryAfter)
	assert.Contains(t, err.Suggestion, "Wait 30s")

	// Test MCP conversion includes retry_after
	mcpErr := err.ToMCPError()
	data := mcpErr.Data.(map[string]interface{})
	assert.Equal(t, 30.0, data["retry_after_seconds"])
}

func TestStructuredError_ComplexMetadata(t *testing.T) {
	err := NewToolExecutionError("complex_tool", errors.New("failed"))
	err.WithMetadata("request", map[string]interface{}{
		"method": "POST",
		"url":    "https://api.example.com",
		"body":   map[string]string{"key": "value"},
	})
	err.WithMetadata("response", map[string]interface{}{
		"status": 500,
		"error":  "Internal Server Error",
	})

	// Verify nested metadata is preserved
	request := err.Metadata["request"].(map[string]interface{})
	assert.Equal(t, "POST", request["method"])

	response := err.Metadata["response"].(map[string]interface{})
	assert.Equal(t, 500, response["status"])
}
