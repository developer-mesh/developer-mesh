package mcp

import (
	"errors"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestSemanticError_Creation(t *testing.T) {
	tests := []struct {
		name         string
		err          *models.ErrorResponse
		expectedCode models.ErrorCode
	}{
		{
			name:         "Protocol Error",
			err:          NewProtocolError("initialize", "Invalid version", "Version 1.0 not supported"),
			expectedCode: models.ErrorCodeProtocolError,
		},
		{
			name:         "Auth Error",
			err:          NewAuthError("Invalid API key"),
			expectedCode: models.ErrorCodeAuthFailed,
		},
		{
			name:         "Tool Execution Error",
			err:          NewToolExecutionError("github_list_repos", errors.New("connection timeout")),
			expectedCode: models.ErrorCodeIntegrationError,
		},
		{
			name:         "Not Found Error",
			err:          NewNotFoundError("repository", "my-repo"),
			expectedCode: models.ErrorCodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedCode, tt.err.Code)
			assert.NotEmpty(t, tt.err.Suggestion, "Expected suggestion to be present")
			assert.NotEmpty(t, tt.err.Error(), "Expected error message")
		})
	}
}

func TestSemanticError_ToMCPError(t *testing.T) {
	// Test protocol error conversion
	protocolErr := NewProtocolError("test", "Test error", "Details")
	mcpErr := ToMCPError(protocolErr)
	assert.Equal(t, -32600, mcpErr.Code)
	assert.Contains(t, mcpErr.Message, "Test error")
}

func TestSemanticError_WithMetadata(t *testing.T) {
	err := NewToolExecutionError("test_tool", errors.New("failed")).
		WithRequestID("req-123").
		WithMetadata("attempt", 3).
		WithMetadata("duration_ms", 1500)

	assert.Equal(t, "req-123", err.RequestID)
	assert.Equal(t, 3, err.Metadata["attempt"])
	assert.Equal(t, 1500, err.Metadata["duration_ms"])

	// Test MCP conversion includes metadata
	mcpErr := ToMCPError(err)
	data := mcpErr.Data.(map[string]interface{})
	assert.Equal(t, 3, data["attempt"])
	assert.Equal(t, 1500, data["duration_ms"])
}

func TestSemanticError_RecoveryInformation(t *testing.T) {
	err := NewToolExecutionError("test_tool", errors.New("failed"))

	// Verify recovery steps exist
	assert.NotEmpty(t, err.RecoverySteps, "Expected recovery steps")
	assert.NotEmpty(t, err.Suggestion, "Expected suggestion")

	// Verify retry strategy
	assert.NotNil(t, err.RetryStrategy, "Expected retry strategy")
	assert.True(t, err.RetryStrategy.Retryable, "Expected error to be retryable")
}

func TestSemanticError_AlternativeTools(t *testing.T) {
	alternatives := []string{"alt_tool1", "alt_tool2"}
	err := NewToolExecutionErrorWithAlternatives("test_tool", errors.New("failed"), alternatives)

	assert.Equal(t, alternatives, err.AlternativeTools, "Expected alternative tools")

	// Verify alternatives appear in recovery steps
	hasAlternative := false
	for _, step := range err.RecoverySteps {
		if step.Action == "use_alternative" {
			hasAlternative = true
			break
		}
	}
	assert.True(t, hasAlternative, "Expected alternative tool step")
}

func TestSemanticError_MCPErrorCodes(t *testing.T) {
	// Test MCP error codes
	testCases := []struct {
		err  *models.ErrorResponse
		code int
	}{
		{NewProtocolError("test", "msg", "details"), -32600},
		{NewAuthError("msg"), -32001},
		{NewValidationError("field", "msg"), -32602},
		{NewNotFoundError("resource", "id"), -32002},
	}

	for _, tc := range testCases {
		mcpErr := ToMCPError(tc.err)
		assert.Equal(t, tc.code, mcpErr.Code,
			"Error code %s should map to MCP code %d", tc.err.Code, tc.code)
	}
}

func TestSemanticError_AIFriendlyFeatures(t *testing.T) {
	err := NewToolExecutionError("complex_tool", errors.New("failed")).
		WithMetadata("request", map[string]interface{}{
			"method": "POST",
			"url":    "https://api.example.com",
		})

	// Verify nested metadata is preserved
	assert.NotNil(t, err.Metadata["request"])

	// Test MCP conversion includes AI-friendly fields
	mcpErr := ToMCPError(err)
	data := mcpErr.Data.(map[string]interface{})

	// Verify essential AI-friendly fields
	assert.NotNil(t, data["code"], "Expected error code")
	assert.NotNil(t, data["category"], "Expected category")
	assert.NotNil(t, data["severity"], "Expected severity")
	assert.NotNil(t, data["suggestion"], "Expected suggestion")
	assert.NotNil(t, data["recovery_steps"], "Expected recovery steps")
}
