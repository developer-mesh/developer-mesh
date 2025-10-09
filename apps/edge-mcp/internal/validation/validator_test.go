package validation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	requestIDKey contextKey = "request_id"
	tenantIDKey  contextKey = "tenant_id"
	sessionIDKey contextKey = "session_id"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.Equal(t, 1024*1024, config.MaxParamsSize)
	assert.Equal(t, 256, config.MaxToolNameLen)
	assert.True(t, config.StrictValidation)
}

func TestNewValidator(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		config := &Config{
			MaxParamsSize:    500000,
			MaxToolNameLen:   128,
			StrictValidation: false,
		}
		logger := observability.NewStandardLogger("test")
		validator := NewValidator(config, logger)

		assert.NotNil(t, validator)
		assert.Equal(t, 500000, validator.maxParamsSize)
		assert.Equal(t, 128, validator.maxToolNameLen)
		assert.False(t, validator.strictValidation)
	})

	t.Run("with nil config uses defaults", func(t *testing.T) {
		logger := observability.NewStandardLogger("test")
		validator := NewValidator(nil, logger)

		assert.NotNil(t, validator)
		assert.Equal(t, 1024*1024, validator.maxParamsSize)
		assert.Equal(t, 256, validator.maxToolNameLen)
		assert.True(t, validator.strictValidation)
	})
}

func TestValidateJSONRPCMessage(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	validator := NewValidator(DefaultConfig(), logger)

	tests := []struct {
		name    string
		msg     map[string]interface{}
		wantErr bool
		errCode string
	}{
		{
			name: "valid request",
			msg: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "1",
				"method":  "initialize",
				"params": map[string]interface{}{
					"protocolVersion": "2025-06-18",
				},
			},
			wantErr: false,
		},
		{
			name: "valid notification (no ID)",
			msg: map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "initialized",
				"params":  map[string]interface{}{},
			},
			wantErr: false,
		},
		{
			name: "valid response",
			msg: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "1",
				"result": map[string]interface{}{
					"status": "ok",
				},
			},
			wantErr: false,
		},
		{
			name: "valid error response",
			msg: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "1",
				"error": map[string]interface{}{
					"code":    -32600,
					"message": "Invalid request",
				},
			},
			wantErr: false,
		},
		{
			name: "missing jsonrpc",
			msg: map[string]interface{}{
				"id":     "1",
				"method": "initialize",
			},
			wantErr: true,
			errCode: "MISSING_JSONRPC_VERSION",
		},
		{
			name: "invalid jsonrpc version",
			msg: map[string]interface{}{
				"jsonrpc": "1.0",
				"id":      "1",
				"method":  "initialize",
			},
			wantErr: true,
			errCode: "INVALID_JSONRPC_VERSION",
		},
		{
			name: "missing method and result and error",
			msg: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "1",
			},
			wantErr: true,
			errCode: "INVALID_MESSAGE_TYPE",
		},
		{
			name: "empty method",
			msg: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "1",
				"method":  "",
			},
			wantErr: true,
			errCode: "EMPTY_METHOD",
		},
		{
			name: "invalid method type",
			msg: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "1",
				"method":  123,
			},
			wantErr: true,
			errCode: "INVALID_METHOD_TYPE",
		},
		{
			name: "invalid ID type",
			msg: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      map[string]interface{}{"foo": "bar"},
				"method":  "initialize",
			},
			wantErr: true,
			errCode: "INVALID_ID_TYPE",
		},
		{
			name: "response missing ID",
			msg: map[string]interface{}{
				"jsonrpc": "2.0",
				"result":  map[string]interface{}{"status": "ok"},
			},
			wantErr: true,
			errCode: "MISSING_RESPONSE_ID",
		},
		{
			name: "error response missing error code",
			msg: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "1",
				"error": map[string]interface{}{
					"message": "Error occurred",
				},
			},
			wantErr: true,
			errCode: "MISSING_ERROR_CODE",
		},
		{
			name: "error response missing error message",
			msg: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "1",
				"error": map[string]interface{}{
					"code": -32600,
				},
			},
			wantErr: true,
			errCode: "MISSING_ERROR_MESSAGE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateJSONRPCMessage(tt.msg)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errCode != "" {
					var ve *ValidationError
					if assert.ErrorAs(t, err, &ve) {
						assert.Equal(t, tt.errCode, ve.Code)
					}
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMethodName(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	validator := NewValidator(DefaultConfig(), logger)

	tests := []struct {
		name    string
		method  string
		wantErr bool
		errCode string
	}{
		{"valid simple method", "initialize", false, ""},
		{"valid method with slash", "tools/list", false, ""},
		{"valid method with dollar", "$/cancelRequest", false, ""},
		{"valid method with underscore", "resource_read", false, ""},
		{"valid method with dot", "context.update", false, ""},
		{"valid method with hyphen", "agent-assign", false, ""},
		{"empty method", "", true, "EMPTY_METHOD_NAME"},
		{"method too long", strings.Repeat("a", 129), true, "METHOD_NAME_TOO_LONG"},
		{"method with invalid chars", "method<script>", true, "INVALID_METHOD_FORMAT"},
		{"method with control char", "method\x00name", true, "METHOD_CONTAINS_CONTROL_CHARS"},
		{"method with spaces", "method name", true, "INVALID_METHOD_FORMAT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateMethodName(tt.method)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errCode != "" {
					var ve *ValidationError
					if assert.ErrorAs(t, err, &ve) {
						assert.Equal(t, tt.errCode, ve.Code)
					}
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMCPProtocolVersion(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	validator := NewValidator(DefaultConfig(), logger)

	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{"valid 2024-11-05", "2024-11-05", false},
		{"valid 2025-03-26", "2025-03-26", false},
		{"valid 2025-06-18", "2025-06-18", false},
		{"invalid version", "1.0", true},
		{"empty version", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateMCPProtocolVersion(tt.version)
			if tt.wantErr {
				require.Error(t, err)
				var ve *ValidationError
				if assert.ErrorAs(t, err, &ve) {
					assert.Equal(t, "UNSUPPORTED_PROTOCOL_VERSION", ve.Code)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateToolName(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	validator := NewValidator(DefaultConfig(), logger)

	tests := []struct {
		name     string
		toolName string
		wantErr  bool
		errCode  string
	}{
		{"valid simple name", "github_list_repos", false, ""},
		{"valid with colon", "harness:pipeline:execute", false, ""},
		{"valid with dot", "devmesh.agent.assign", false, ""},
		{"valid with hyphen", "agent-heartbeat", false, ""},
		{"empty name", "", true, "EMPTY_TOOL_NAME"},
		{"name too long", strings.Repeat("a", 257), true, "TOOL_NAME_TOO_LONG"},
		{"invalid chars", "tool<script>", true, "INVALID_TOOL_NAME_FORMAT"},
		{"control char", "tool\x00name", true, "TOOL_NAME_CONTAINS_CONTROL_CHARS"},
		{"spaces", "tool name", true, "INVALID_TOOL_NAME_FORMAT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateToolName(tt.toolName)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errCode != "" {
					var ve *ValidationError
					if assert.ErrorAs(t, err, &ve) {
						assert.Equal(t, tt.errCode, ve.Code)
					}
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateToolArguments(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	validator := NewValidator(DefaultConfig(), logger)
	ctx := context.Background()

	t.Run("no schema - valid JSON", func(t *testing.T) {
		args := json.RawMessage(`{"owner":"test","repo":"myrepo"}`)
		err := validator.ValidateToolArguments(ctx, "test_tool", args, nil)
		assert.NoError(t, err)
	})

	t.Run("no schema - invalid JSON", func(t *testing.T) {
		args := json.RawMessage(`{invalid json}`)
		err := validator.ValidateToolArguments(ctx, "test_tool", args, nil)
		require.Error(t, err)
		var ve *ValidationError
		if assert.ErrorAs(t, err, &ve) {
			assert.Equal(t, "INVALID_ARGUMENTS_JSON", ve.Code)
		}
	})

	t.Run("with schema - valid arguments", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type": "string",
				},
				"repo": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []interface{}{"owner", "repo"},
		}
		args := json.RawMessage(`{"owner":"test","repo":"myrepo"}`)
		err := validator.ValidateToolArguments(ctx, "test_tool", args, schema)
		assert.NoError(t, err)
	})

	t.Run("with schema - missing required field", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type": "string",
				},
				"repo": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []interface{}{"owner", "repo"},
		}
		args := json.RawMessage(`{"owner":"test"}`)
		err := validator.ValidateToolArguments(ctx, "test_tool", args, schema)
		require.Error(t, err)
		var ve *ValidationError
		if assert.ErrorAs(t, err, &ve) {
			assert.Equal(t, "ARGUMENTS_SCHEMA_MISMATCH", ve.Code)
			assert.Contains(t, ve.Message, "repo")
		}
	})

	t.Run("with schema - wrong type", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"count": map[string]interface{}{
					"type": "number",
				},
			},
		}
		args := json.RawMessage(`{"count":"not a number"}`)
		err := validator.ValidateToolArguments(ctx, "test_tool", args, schema)
		require.Error(t, err)
		var ve *ValidationError
		if assert.ErrorAs(t, err, &ve) {
			assert.Equal(t, "ARGUMENTS_SCHEMA_MISMATCH", ve.Code)
		}
	})
}

func TestSanitizeString(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	validator := NewValidator(DefaultConfig(), logger)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean string",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "string with control chars",
			input:    "hello\x00world\x01test",
			expected: "helloworldtest",
		},
		{
			name:     "string with tabs and newlines preserved",
			input:    "hello\tworld\ntest",
			expected: "hello\tworld\ntest",
		},
		{
			name:     "string with leading/trailing spaces",
			input:    "  hello world  ",
			expected: "hello world",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   \t\n   ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.SanitizeString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateClientInfo(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	validator := NewValidator(DefaultConfig(), logger)

	tests := []struct {
		name       string
		clientInfo map[string]interface{}
		wantErr    bool
		errCode    string
	}{
		{
			name: "valid client info",
			clientInfo: map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "valid with optional type",
			clientInfo: map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
				"type":    "ide",
			},
			wantErr: false,
		},
		{
			name:       "missing name",
			clientInfo: map[string]interface{}{"version": "1.0.0"},
			wantErr:    true,
			errCode:    "MISSING_CLIENT_NAME",
		},
		{
			name:       "missing version",
			clientInfo: map[string]interface{}{"name": "test-client"},
			wantErr:    true,
			errCode:    "MISSING_CLIENT_VERSION",
		},
		{
			name:       "empty name",
			clientInfo: map[string]interface{}{"name": "", "version": "1.0.0"},
			wantErr:    true,
			errCode:    "EMPTY_CLIENT_NAME",
		},
		{
			name:       "empty version",
			clientInfo: map[string]interface{}{"name": "test-client", "version": ""},
			wantErr:    true,
			errCode:    "EMPTY_CLIENT_VERSION",
		},
		{
			name:       "name too long",
			clientInfo: map[string]interface{}{"name": strings.Repeat("a", 257), "version": "1.0.0"},
			wantErr:    true,
			errCode:    "CLIENT_NAME_TOO_LONG",
		},
		{
			name:       "version too long",
			clientInfo: map[string]interface{}{"name": "test-client", "version": strings.Repeat("1", 129)},
			wantErr:    true,
			errCode:    "CLIENT_VERSION_TOO_LONG",
		},
		{
			name:       "name not a string",
			clientInfo: map[string]interface{}{"name": 123, "version": "1.0.0"},
			wantErr:    true,
			errCode:    "INVALID_CLIENT_NAME_TYPE",
		},
		{
			name:       "version not a string",
			clientInfo: map[string]interface{}{"name": "test-client", "version": 1.0},
			wantErr:    true,
			errCode:    "INVALID_CLIENT_VERSION_TYPE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateClientInfo(tt.clientInfo)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errCode != "" {
					var ve *ValidationError
					if assert.ErrorAs(t, err, &ve) {
						assert.Equal(t, tt.errCode, ve.Code)
					}
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateParams_SizeLimit(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	config := &Config{
		MaxParamsSize:    100, // Very small for testing
		MaxToolNameLen:   256,
		StrictValidation: true,
	}
	validator := NewValidator(config, logger)

	t.Run("params exceeds size limit", func(t *testing.T) {
		// Create a large params object
		largeParams := map[string]interface{}{
			"data": strings.Repeat("a", 200),
		}

		err := validator.validateJSONRPCParams(largeParams)
		require.Error(t, err)
		var ve *ValidationError
		if assert.ErrorAs(t, err, &ve) {
			assert.Equal(t, "PARAMS_TOO_LARGE", ve.Code)
		}
	})

	t.Run("params within size limit", func(t *testing.T) {
		smallParams := map[string]interface{}{
			"data": "small",
		}

		err := validator.validateJSONRPCParams(smallParams)
		assert.NoError(t, err)
	})
}

func TestLogValidationFailure(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	validator := NewValidator(DefaultConfig(), logger)

	ctx := context.Background()
	ctx = context.WithValue(ctx, requestIDKey, "test-request-123")
	ctx = context.WithValue(ctx, tenantIDKey, "tenant-456")
	ctx = context.WithValue(ctx, sessionIDKey, "session-789")

	err := &ValidationError{
		Field:   "test_field",
		Message: "test error",
		Code:    "TEST_ERROR",
	}

	details := map[string]interface{}{
		"method": "test_method",
	}

	// Should not panic
	validator.LogValidationFailure(ctx, err, details)
}

func TestToErrorResponse(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	validator := NewValidator(DefaultConfig(), logger)

	t.Run("validation error", func(t *testing.T) {
		ve := &ValidationError{
			Field:   "test_field",
			Message: "test error",
			Code:    "TEST_ERROR",
		}

		errResp := validator.ToErrorResponse(ve, "test_operation")
		assert.NotNil(t, errResp)
		assert.Equal(t, "TEST_ERROR", string(errResp.Code))
		assert.Contains(t, errResp.Message, "test error")
		assert.Equal(t, "test_operation", errResp.Operation)
		assert.NotNil(t, errResp.RetryStrategy)
		assert.False(t, errResp.RetryStrategy.Retryable)
		assert.Equal(t, "test_field", errResp.Metadata["field"])
	})

	t.Run("generic error", func(t *testing.T) {
		err := assert.AnError

		errResp := validator.ToErrorResponse(err, "test_operation")
		assert.NotNil(t, errResp)
		assert.Equal(t, string(models.ErrorCodeValidation), string(errResp.Code))
		assert.Contains(t, errResp.Message, "Validation failed")
		assert.NotNil(t, errResp.RetryStrategy)
		assert.False(t, errResp.RetryStrategy.Retryable)
	})
}

func TestValidationError_Error(t *testing.T) {
	ve := &ValidationError{
		Field:   "test_field",
		Message: "test message",
		Code:    "TEST_CODE",
	}

	expected := "validation error in field 'test_field': test message"
	assert.Equal(t, expected, ve.Error())
}
