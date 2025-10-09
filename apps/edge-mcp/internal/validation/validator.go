package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/xeipuuv/gojsonschema"
)

// Validator provides comprehensive input validation for Edge MCP
type Validator struct {
	logger           observability.Logger
	maxParamsSize    int  // Maximum size of params in bytes
	maxToolNameLen   int  // Maximum length of tool name
	strictValidation bool // Enable strict validation mode
}

// ValidationError represents a validation error with details
type ValidationError struct {
	Field   string
	Message string
	Code    string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error in field '%s': %s", e.Field, e.Message)
}

// ValidationResult contains the result of validation
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

// Config holds validator configuration
type Config struct {
	MaxParamsSize    int  // Maximum size of params in bytes (default: 1MB)
	MaxToolNameLen   int  // Maximum length of tool name (default: 256)
	StrictValidation bool // Enable strict validation mode (default: true)
}

// DefaultConfig returns a validator configuration with secure defaults
func DefaultConfig() *Config {
	return &Config{
		MaxParamsSize:    1024 * 1024, // 1MB
		MaxToolNameLen:   256,
		StrictValidation: true,
	}
}

// NewValidator creates a new validator with given configuration
func NewValidator(config *Config, logger observability.Logger) *Validator {
	if config == nil {
		config = DefaultConfig()
	}

	return &Validator{
		logger:           logger,
		maxParamsSize:    config.MaxParamsSize,
		maxToolNameLen:   config.MaxToolNameLen,
		strictValidation: config.StrictValidation,
	}
}

// ValidateJSONRPCMessage validates a JSON-RPC 2.0 message structure
func (v *Validator) ValidateJSONRPCMessage(msg map[string]interface{}) error {
	// Check jsonrpc version
	jsonrpc, ok := msg["jsonrpc"].(string)
	if !ok {
		return &ValidationError{
			Field:   "jsonrpc",
			Message: "field is required and must be a string",
			Code:    "MISSING_JSONRPC_VERSION",
		}
	}
	if jsonrpc != "2.0" {
		return &ValidationError{
			Field:   "jsonrpc",
			Message: fmt.Sprintf("must be '2.0', got '%s'", jsonrpc),
			Code:    "INVALID_JSONRPC_VERSION",
		}
	}

	// Check if it's a request or response
	if method, hasMethod := msg["method"]; hasMethod {
		// It's a request - validate request-specific fields
		if err := v.validateJSONRPCRequest(msg, method); err != nil {
			return err
		}
	} else if _, hasResult := msg["result"]; hasResult {
		// It's a response - validate response-specific fields
		if err := v.validateJSONRPCResponse(msg); err != nil {
			return err
		}
	} else if _, hasError := msg["error"]; hasError {
		// It's an error response
		if err := v.validateJSONRPCErrorResponse(msg); err != nil {
			return err
		}
	} else {
		return &ValidationError{
			Field:   "method/result/error",
			Message: "message must contain either 'method', 'result', or 'error' field",
			Code:    "INVALID_MESSAGE_TYPE",
		}
	}

	return nil
}

func (v *Validator) validateJSONRPCRequest(msg map[string]interface{}, method interface{}) error {
	// Validate method
	methodStr, ok := method.(string)
	if !ok {
		return &ValidationError{
			Field:   "method",
			Message: "must be a string",
			Code:    "INVALID_METHOD_TYPE",
		}
	}

	if methodStr == "" {
		return &ValidationError{
			Field:   "method",
			Message: "cannot be empty",
			Code:    "EMPTY_METHOD",
		}
	}

	// Validate method name format (prevent injection)
	if err := v.ValidateMethodName(methodStr); err != nil {
		return err
	}

	// ID is optional for notifications, but if present must be string, number, or null
	if id, hasID := msg["id"]; hasID {
		if err := v.validateJSONRPCID(id); err != nil {
			return err
		}
	}

	// Validate params if present
	if params, hasParams := msg["params"]; hasParams {
		if err := v.validateJSONRPCParams(params); err != nil {
			return err
		}
	}

	return nil
}

func (v *Validator) validateJSONRPCResponse(msg map[string]interface{}) error {
	// Response must have ID
	id, hasID := msg["id"]
	if !hasID {
		return &ValidationError{
			Field:   "id",
			Message: "is required for responses",
			Code:    "MISSING_RESPONSE_ID",
		}
	}

	if err := v.validateJSONRPCID(id); err != nil {
		return err
	}

	return nil
}

func (v *Validator) validateJSONRPCErrorResponse(msg map[string]interface{}) error {
	// Error response must have ID
	id, hasID := msg["id"]
	if !hasID {
		return &ValidationError{
			Field:   "id",
			Message: "is required for error responses",
			Code:    "MISSING_ERROR_ID",
		}
	}

	if err := v.validateJSONRPCID(id); err != nil {
		return err
	}

	// Validate error object
	errObj, ok := msg["error"].(map[string]interface{})
	if !ok {
		return &ValidationError{
			Field:   "error",
			Message: "must be an object",
			Code:    "INVALID_ERROR_TYPE",
		}
	}

	// Error must have code and message
	if _, hasCode := errObj["code"]; !hasCode {
		return &ValidationError{
			Field:   "error.code",
			Message: "is required",
			Code:    "MISSING_ERROR_CODE",
		}
	}

	if _, hasMessage := errObj["message"]; !hasMessage {
		return &ValidationError{
			Field:   "error.message",
			Message: "is required",
			Code:    "MISSING_ERROR_MESSAGE",
		}
	}

	return nil
}

func (v *Validator) validateJSONRPCID(id interface{}) error {
	// ID must be string, number, or null (per JSON-RPC spec)
	switch id.(type) {
	case string, float64, int, int64, nil:
		return nil
	default:
		return &ValidationError{
			Field:   "id",
			Message: "must be a string, number, or null",
			Code:    "INVALID_ID_TYPE",
		}
	}
}

func (v *Validator) validateJSONRPCParams(params interface{}) error {
	// Params must be object or array (per JSON-RPC spec)
	switch p := params.(type) {
	case map[string]interface{}:
		// Check size limit
		paramBytes, err := json.Marshal(p)
		if err != nil {
			return &ValidationError{
				Field:   "params",
				Message: fmt.Sprintf("failed to serialize: %v", err),
				Code:    "INVALID_PARAMS_FORMAT",
			}
		}
		if len(paramBytes) > v.maxParamsSize {
			return &ValidationError{
				Field:   "params",
				Message: fmt.Sprintf("size exceeds maximum of %d bytes", v.maxParamsSize),
				Code:    "PARAMS_TOO_LARGE",
			}
		}
		return nil
	case []interface{}:
		// Check size limit
		paramBytes, err := json.Marshal(p)
		if err != nil {
			return &ValidationError{
				Field:   "params",
				Message: fmt.Sprintf("failed to serialize: %v", err),
				Code:    "INVALID_PARAMS_FORMAT",
			}
		}
		if len(paramBytes) > v.maxParamsSize {
			return &ValidationError{
				Field:   "params",
				Message: fmt.Sprintf("size exceeds maximum of %d bytes", v.maxParamsSize),
				Code:    "PARAMS_TOO_LARGE",
			}
		}
		return nil
	case json.RawMessage:
		// Already in raw form, check size
		if len(p) > v.maxParamsSize {
			return &ValidationError{
				Field:   "params",
				Message: fmt.Sprintf("size exceeds maximum of %d bytes", v.maxParamsSize),
				Code:    "PARAMS_TOO_LARGE",
			}
		}
		return nil
	default:
		return &ValidationError{
			Field:   "params",
			Message: "must be an object or array",
			Code:    "INVALID_PARAMS_TYPE",
		}
	}
}

// ValidateMethodName validates MCP method name format and prevents injection
func (v *Validator) ValidateMethodName(method string) error {
	// Check length
	if len(method) == 0 {
		return &ValidationError{
			Field:   "method",
			Message: "cannot be empty",
			Code:    "EMPTY_METHOD_NAME",
		}
	}

	if len(method) > 128 {
		return &ValidationError{
			Field:   "method",
			Message: "exceeds maximum length of 128 characters",
			Code:    "METHOD_NAME_TOO_LONG",
		}
	}

	// Prevent control characters (check first for better error messages)
	for _, r := range method {
		if unicode.IsControl(r) {
			return &ValidationError{
				Field:   "method",
				Message: "contains control characters",
				Code:    "METHOD_CONTAINS_CONTROL_CHARS",
			}
		}
	}

	// Valid MCP methods: alphanumeric, slash, dollar sign, underscore, dot, hyphen
	// Examples: "initialize", "tools/list", "$/cancelRequest", "resources/read"
	validMethodPattern := regexp.MustCompile(`^[\$a-zA-Z0-9/_.-]+$`)
	if !validMethodPattern.MatchString(method) {
		return &ValidationError{
			Field:   "method",
			Message: "contains invalid characters (allowed: alphanumeric, /, $, _, ., -)",
			Code:    "INVALID_METHOD_FORMAT",
		}
	}

	return nil
}

// ValidateMCPProtocolVersion validates MCP protocol version
func (v *Validator) ValidateMCPProtocolVersion(version string) error {
	supportedVersions := []string{
		"2024-11-05",
		"2025-03-26",
		"2025-06-18",
	}

	for _, supported := range supportedVersions {
		if version == supported {
			return nil
		}
	}

	return &ValidationError{
		Field:   "protocolVersion",
		Message: fmt.Sprintf("unsupported version '%s', supported: %v", version, supportedVersions),
		Code:    "UNSUPPORTED_PROTOCOL_VERSION",
	}
}

// ValidateToolName validates tool name format and prevents injection
func (v *Validator) ValidateToolName(toolName string) error {
	// Check length
	if len(toolName) == 0 {
		return &ValidationError{
			Field:   "name",
			Message: "tool name cannot be empty",
			Code:    "EMPTY_TOOL_NAME",
		}
	}

	if len(toolName) > v.maxToolNameLen {
		return &ValidationError{
			Field:   "name",
			Message: fmt.Sprintf("tool name exceeds maximum length of %d characters", v.maxToolNameLen),
			Code:    "TOOL_NAME_TOO_LONG",
		}
	}

	// Prevent control characters (check first for better error messages)
	for _, r := range toolName {
		if unicode.IsControl(r) {
			return &ValidationError{
				Field:   "name",
				Message: "tool name contains control characters",
				Code:    "TOOL_NAME_CONTAINS_CONTROL_CHARS",
			}
		}
	}

	// Valid tool names: alphanumeric, underscore, hyphen, dot, colon
	// Examples: "github_list_repos", "devmesh_agent_assign", "harness:pipeline:execute"
	validToolPattern := regexp.MustCompile(`^[a-zA-Z0-9_:.-]+$`)
	if !validToolPattern.MatchString(toolName) {
		return &ValidationError{
			Field:   "name",
			Message: "tool name contains invalid characters (allowed: alphanumeric, _, -, ., :)",
			Code:    "INVALID_TOOL_NAME_FORMAT",
		}
	}

	return nil
}

// ValidateToolArguments validates tool arguments against JSON schema
func (v *Validator) ValidateToolArguments(
	ctx context.Context,
	toolName string,
	arguments json.RawMessage,
	schema map[string]interface{},
) error {
	// Create request-scoped logger
	logger := v.logger
	if reqLogger, ok := ctx.Value("logger").(observability.Logger); ok {
		logger = reqLogger
	}

	// If no schema provided, just validate it's valid JSON
	if len(schema) == 0 {
		var temp interface{}
		if err := json.Unmarshal(arguments, &temp); err != nil {
			return &ValidationError{
				Field:   "arguments",
				Message: fmt.Sprintf("invalid JSON: %v", err),
				Code:    "INVALID_ARGUMENTS_JSON",
			}
		}
		return nil
	}

	// Validate against JSON schema
	schemaLoader := gojsonschema.NewGoLoader(schema)
	argsLoader := gojsonschema.NewBytesLoader(arguments)

	result, err := gojsonschema.Validate(schemaLoader, argsLoader)
	if err != nil {
		logger.Error("JSON schema validation failed", map[string]interface{}{
			"tool":  toolName,
			"error": err.Error(),
		})
		return &ValidationError{
			Field:   "arguments",
			Message: fmt.Sprintf("schema validation error: %v", err),
			Code:    "SCHEMA_VALIDATION_ERROR",
		}
	}

	if !result.Valid() {
		// Collect all validation errors
		var errorMessages []string
		for _, err := range result.Errors() {
			errorMessages = append(errorMessages, err.String())
		}

		logger.Warn("Tool arguments failed schema validation", map[string]interface{}{
			"tool":   toolName,
			"errors": errorMessages,
		})

		return &ValidationError{
			Field:   "arguments",
			Message: fmt.Sprintf("schema validation failed: %s", strings.Join(errorMessages, "; ")),
			Code:    "ARGUMENTS_SCHEMA_MISMATCH",
		}
	}

	return nil
}

// SanitizeString sanitizes a string to prevent injection attacks
func (v *Validator) SanitizeString(input string) string {
	// Remove control characters
	var builder strings.Builder
	for _, r := range input {
		if !unicode.IsControl(r) || r == '\n' || r == '\r' || r == '\t' {
			builder.WriteRune(r)
		}
	}
	sanitized := builder.String()

	// Trim whitespace
	sanitized = strings.TrimSpace(sanitized)

	return sanitized
}

// ValidateClientInfo validates client information in initialize request
func (v *Validator) ValidateClientInfo(clientInfo map[string]interface{}) error {
	// Name is required
	name, hasName := clientInfo["name"]
	if !hasName {
		return &ValidationError{
			Field:   "clientInfo.name",
			Message: "is required",
			Code:    "MISSING_CLIENT_NAME",
		}
	}

	nameStr, ok := name.(string)
	if !ok {
		return &ValidationError{
			Field:   "clientInfo.name",
			Message: "must be a string",
			Code:    "INVALID_CLIENT_NAME_TYPE",
		}
	}

	if len(nameStr) == 0 {
		return &ValidationError{
			Field:   "clientInfo.name",
			Message: "cannot be empty",
			Code:    "EMPTY_CLIENT_NAME",
		}
	}

	if len(nameStr) > 256 {
		return &ValidationError{
			Field:   "clientInfo.name",
			Message: "exceeds maximum length of 256 characters",
			Code:    "CLIENT_NAME_TOO_LONG",
		}
	}

	// Version is required
	version, hasVersion := clientInfo["version"]
	if !hasVersion {
		return &ValidationError{
			Field:   "clientInfo.version",
			Message: "is required",
			Code:    "MISSING_CLIENT_VERSION",
		}
	}

	versionStr, ok := version.(string)
	if !ok {
		return &ValidationError{
			Field:   "clientInfo.version",
			Message: "must be a string",
			Code:    "INVALID_CLIENT_VERSION_TYPE",
		}
	}

	if len(versionStr) == 0 {
		return &ValidationError{
			Field:   "clientInfo.version",
			Message: "cannot be empty",
			Code:    "EMPTY_CLIENT_VERSION",
		}
	}

	if len(versionStr) > 128 {
		return &ValidationError{
			Field:   "clientInfo.version",
			Message: "exceeds maximum length of 128 characters",
			Code:    "CLIENT_VERSION_TOO_LONG",
		}
	}

	return nil
}

// LogValidationFailure logs a validation failure with full context
func (v *Validator) LogValidationFailure(ctx context.Context, err error, details map[string]interface{}) {
	// Extract validation error if available
	var code string
	var field string

	if ve, ok := err.(*ValidationError); ok {
		code = ve.Code
		field = ve.Field
	} else if ve, ok := err.(*models.ErrorResponse); ok {
		code = string(ve.Code)
	}

	// Merge details with error info
	logFields := make(map[string]interface{})
	for k, v := range details {
		logFields[k] = v
	}
	logFields["error"] = err.Error()
	if code != "" {
		logFields["error_code"] = code
	}
	if field != "" {
		logFields["field"] = field
	}

	// Add context fields if available
	if reqID, ok := ctx.Value("request_id").(string); ok {
		logFields["request_id"] = reqID
	}
	if tenantID, ok := ctx.Value("tenant_id").(string); ok {
		logFields["tenant_id"] = tenantID
	}
	if sessionID, ok := ctx.Value("session_id").(string); ok {
		logFields["session_id"] = sessionID
	}

	// Log as warning (not error, since validation failures are expected)
	v.logger.Warn("Request validation failed", logFields)
}

// ToErrorResponse converts a validation error to an ErrorResponse
func (v *Validator) ToErrorResponse(err error, operation string) *models.ErrorResponse {
	if ve, ok := err.(*ValidationError); ok {
		return &models.ErrorResponse{
			Code:      models.ErrorCode(ve.Code),
			Message:   fmt.Sprintf("Validation failed: %s", ve.Message),
			Details:   fmt.Sprintf("Field: %s", ve.Field),
			Category:  models.CategoryValidation,
			Severity:  models.SeverityWarning,
			Operation: operation,
			Metadata: map[string]interface{}{
				"field": ve.Field,
			},
			RetryStrategy: &models.RetryStrategy{
				Retryable: false,
			},
		}
	}

	// Generic validation error
	return &models.ErrorResponse{
		Code:      models.ErrorCodeValidation,
		Message:   fmt.Sprintf("Validation failed: %v", err),
		Category:  models.CategoryValidation,
		Severity:  models.SeverityWarning,
		Operation: operation,
		RetryStrategy: &models.RetryStrategy{
			Retryable: false,
		},
	}
}
