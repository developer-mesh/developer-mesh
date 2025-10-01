package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// ErrorCode represents standardized error codes for the system
type ErrorCode string

const (
	// Authentication and Authorization errors (1xxx)
	ErrorCodeAuthFailed       ErrorCode = "AUTH_FAILED"       // Authentication failed
	ErrorCodeUnauthorized     ErrorCode = "UNAUTHORIZED"      // Not authorized to perform action
	ErrorCodeTokenExpired     ErrorCode = "TOKEN_EXPIRED"     // Authentication token has expired
	ErrorCodeInvalidAPIKey    ErrorCode = "INVALID_API_KEY"   // API key is invalid or malformed
	ErrorCodePermissionDenied ErrorCode = "PERMISSION_DENIED" // User lacks required permissions
	ErrorCodeAccountSuspended ErrorCode = "ACCOUNT_SUSPENDED" // Account has been suspended
	ErrorCodeMFARequired      ErrorCode = "MFA_REQUIRED"      // Multi-factor authentication required

	// Resource errors (2xxx)
	ErrorCodeNotFound         ErrorCode = "NOT_FOUND"         // Resource not found
	ErrorCodeAlreadyExists    ErrorCode = "ALREADY_EXISTS"    // Resource already exists
	ErrorCodeConflict         ErrorCode = "CONFLICT"          // Resource conflict (e.g., version mismatch)
	ErrorCodeResourceLocked   ErrorCode = "RESOURCE_LOCKED"   // Resource is locked for modification
	ErrorCodeResourceDeleted  ErrorCode = "RESOURCE_DELETED"  // Resource has been deleted
	ErrorCodeResourceExpired  ErrorCode = "RESOURCE_EXPIRED"  // Resource has expired
	ErrorCodeDependencyFailed ErrorCode = "DEPENDENCY_FAILED" // Required dependency is unavailable

	// Rate limiting and quotas (3xxx)
	ErrorCodeRateLimit        ErrorCode = "RATE_LIMIT"        // Rate limit exceeded
	ErrorCodeQuotaExceeded    ErrorCode = "QUOTA_EXCEEDED"    // Quota exceeded
	ErrorCodeConcurrencyLimit ErrorCode = "CONCURRENCY_LIMIT" // Too many concurrent operations
	ErrorCodeBandwidthLimit   ErrorCode = "BANDWIDTH_LIMIT"   // Bandwidth limit exceeded
	ErrorCodeStorageLimit     ErrorCode = "STORAGE_LIMIT"     // Storage limit exceeded

	// Validation errors (4xxx)
	ErrorCodeValidation       ErrorCode = "VALIDATION_ERROR"  // Input validation failed
	ErrorCodeInvalidInput     ErrorCode = "INVALID_INPUT"     // Invalid input provided
	ErrorCodeMissingParameter ErrorCode = "MISSING_PARAMETER" // Required parameter missing
	ErrorCodeInvalidFormat    ErrorCode = "INVALID_FORMAT"    // Invalid format for input
	ErrorCodeOutOfRange       ErrorCode = "OUT_OF_RANGE"      // Value out of acceptable range
	ErrorCodeTypeMismatch     ErrorCode = "TYPE_MISMATCH"     // Type mismatch in input
	ErrorCodeSchemaViolation  ErrorCode = "SCHEMA_VIOLATION"  // Schema validation failed

	// Network and connectivity (5xxx)
	ErrorCodeTimeout          ErrorCode = "TIMEOUT"           // Operation timed out
	ErrorCodeNetworkError     ErrorCode = "NETWORK_ERROR"     // Network error occurred
	ErrorCodeConnectionFailed ErrorCode = "CONNECTION_FAILED" // Failed to establish connection
	ErrorCodeServiceOffline   ErrorCode = "SERVICE_OFFLINE"   // Service is offline
	ErrorCodeDNSResolution    ErrorCode = "DNS_RESOLUTION"    // DNS resolution failed
	ErrorCodeSSLError         ErrorCode = "SSL_ERROR"         // SSL/TLS error
	ErrorCodeProxyError       ErrorCode = "PROXY_ERROR"       // Proxy connection error

	// External service errors (6xxx)
	ErrorCodeUpstreamError    ErrorCode = "UPSTREAM_ERROR"    // Upstream service error
	ErrorCodeProviderError    ErrorCode = "PROVIDER_ERROR"    // External provider error
	ErrorCodeIntegrationError ErrorCode = "INTEGRATION_ERROR" // Integration failure
	ErrorCodeWebhookFailed    ErrorCode = "WEBHOOK_FAILED"    // Webhook delivery failed
	ErrorCodeAPIError         ErrorCode = "API_ERROR"         // External API error

	// System errors (7xxx)
	ErrorCodeInternal      ErrorCode = "INTERNAL_ERROR" // Internal server error
	ErrorCodeDatabaseError ErrorCode = "DATABASE_ERROR" // Database operation failed
	ErrorCodeCacheError    ErrorCode = "CACHE_ERROR"    // Cache operation failed
	ErrorCodeConfigError   ErrorCode = "CONFIG_ERROR"   // Configuration error
	ErrorCodeInitError     ErrorCode = "INIT_ERROR"     // Initialization failed
	ErrorCodeShutdown      ErrorCode = "SHUTDOWN"       // System is shutting down
	ErrorCodeMaintenance   ErrorCode = "MAINTENANCE"    // System under maintenance

	// Protocol and format errors (8xxx)
	ErrorCodeProtocolError    ErrorCode = "PROTOCOL_ERROR"      // Protocol violation
	ErrorCodeVersionMismatch  ErrorCode = "VERSION_MISMATCH"    // Version incompatibility
	ErrorCodeEncodingError    ErrorCode = "ENCODING_ERROR"      // Encoding/decoding failed
	ErrorCodeParsingError     ErrorCode = "PARSING_ERROR"       // Failed to parse data
	ErrorCodeSerializationErr ErrorCode = "SERIALIZATION_ERROR" // Serialization failed

	// Business logic errors (9xxx)
	ErrorCodeInvalidOperation ErrorCode = "INVALID_OPERATION"   // Operation not allowed
	ErrorCodePreconditionFail ErrorCode = "PRECONDITION_FAILED" // Precondition not met
	ErrorCodeStateError       ErrorCode = "STATE_ERROR"         // Invalid state for operation
	ErrorCodeWorkflowError    ErrorCode = "WORKFLOW_ERROR"      // Workflow execution failed
	ErrorCodePolicyViolation  ErrorCode = "POLICY_VIOLATION"    // Policy violation detected
)

// ErrorSeverity indicates the severity level of an error
type ErrorSeverity string

const (
	SeverityInfo     ErrorSeverity = "INFO"     // Informational, not an actual error
	SeverityWarning  ErrorSeverity = "WARNING"  // Warning, operation succeeded with issues
	SeverityError    ErrorSeverity = "ERROR"    // Error, operation failed but recoverable
	SeverityCritical ErrorSeverity = "CRITICAL" // Critical error, system-level impact
	SeverityFatal    ErrorSeverity = "FATAL"    // Fatal error, requires immediate attention
)

// ErrorCategory groups related error types
type ErrorCategory string

const (
	CategoryAuth       ErrorCategory = "AUTHENTICATION"
	CategoryResource   ErrorCategory = "RESOURCE"
	CategoryRateLimit  ErrorCategory = "RATE_LIMIT"
	CategoryValidation ErrorCategory = "VALIDATION"
	CategoryNetwork    ErrorCategory = "NETWORK"
	CategoryExternal   ErrorCategory = "EXTERNAL_SERVICE"
	CategorySystem     ErrorCategory = "SYSTEM"
	CategoryProtocol   ErrorCategory = "PROTOCOL"
	CategoryBusiness   ErrorCategory = "BUSINESS_LOGIC"
)

// ErrorResponse represents a comprehensive error response for AI agents
type ErrorResponse struct {
	// Core error information
	Code      ErrorCode     `json:"code"`              // Standardized error code
	Message   string        `json:"message"`           // Human-readable error message
	Details   string        `json:"details,omitempty"` // Detailed error description
	Category  ErrorCategory `json:"category"`          // Error category for grouping
	Severity  ErrorSeverity `json:"severity"`          // Error severity level
	Timestamp time.Time     `json:"timestamp"`         // When the error occurred

	// Context information
	RequestID string `json:"request_id,omitempty"` // Request identifier for tracing
	Operation string `json:"operation,omitempty"`  // Operation that failed
	Service   string `json:"service,omitempty"`    // Service that generated error
	Version   string `json:"version,omitempty"`    // API/protocol version

	// Affected resources
	Resource         *ResourceInfo  `json:"resource,omitempty"`          // Primary affected resource
	RelatedResources []ResourceInfo `json:"related_resources,omitempty"` // Related affected resources

	// Recovery information
	Suggestion       string         `json:"suggestion,omitempty"`        // Suggested action to resolve
	Documentation    string         `json:"documentation,omitempty"`     // Link to relevant documentation
	AlternativeTools []string       `json:"alternative_tools,omitempty"` // Alternative tools to try
	RecoverySteps    []RecoveryStep `json:"recovery_steps,omitempty"`    // Steps to recover
	RetryStrategy    *RetryStrategy `json:"retry_strategy,omitempty"`    // Retry guidance

	// Rate limiting information
	RetryAfter    *time.Duration `json:"retry_after,omitempty"`     // When to retry (for rate limits)
	RateLimitInfo *RateLimitInfo `json:"rate_limit_info,omitempty"` // Rate limit details

	// Additional metadata
	Metadata   map[string]interface{} `json:"metadata,omitempty"`    // Additional error metadata
	StackTrace []string               `json:"stack_trace,omitempty"` // Stack trace (debug mode only)
	InnerError *ErrorResponse         `json:"inner_error,omitempty"` // Nested error for error chains
}

// ResourceInfo describes an affected resource
type ResourceInfo struct {
	Type       string                 `json:"type"`                 // Resource type (e.g., "repository", "issue")
	ID         string                 `json:"id,omitempty"`         // Resource identifier
	Name       string                 `json:"name,omitempty"`       // Resource name
	Path       string                 `json:"path,omitempty"`       // Resource path or location
	Owner      string                 `json:"owner,omitempty"`      // Resource owner
	State      string                 `json:"state,omitempty"`      // Current resource state
	Attributes map[string]interface{} `json:"attributes,omitempty"` // Additional attributes
}

// RecoveryStep represents a step in error recovery
type RecoveryStep struct {
	Order       int                    `json:"order"`                // Step order
	Action      string                 `json:"action"`               // Action to take
	Description string                 `json:"description"`          // Detailed description
	Tool        string                 `json:"tool,omitempty"`       // Tool to use for this step
	Parameters  map[string]interface{} `json:"parameters,omitempty"` // Parameters for the action
	Optional    bool                   `json:"optional,omitempty"`   // Whether step is optional
}

// RetryStrategy provides retry guidance
type RetryStrategy struct {
	Retryable      bool          `json:"retryable"`                 // Whether error is retryable
	MaxAttempts    int           `json:"max_attempts,omitempty"`    // Maximum retry attempts
	BackoffType    string        `json:"backoff_type,omitempty"`    // Backoff strategy (exponential, linear)
	InitialDelay   time.Duration `json:"initial_delay,omitempty"`   // Initial retry delay
	MaxDelay       time.Duration `json:"max_delay,omitempty"`       // Maximum retry delay
	RetryCondition string        `json:"retry_condition,omitempty"` // Condition for retry
}

// RateLimitInfo provides rate limit details
type RateLimitInfo struct {
	Limit     int           `json:"limit"`               // Rate limit
	Remaining int           `json:"remaining"`           // Remaining requests
	Reset     time.Time     `json:"reset"`               // When limit resets
	Window    time.Duration `json:"window"`              // Rate limit window
	Scope     string        `json:"scope,omitempty"`     // Scope of rate limit
	TierInfo  string        `json:"tier_info,omitempty"` // Information about tier/plan
}

// Error implements the error interface
func (e *ErrorResponse) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// ToJSON converts error response to JSON
func (e *ErrorResponse) ToJSON() ([]byte, error) {
	return json.MarshalIndent(e, "", "  ")
}

// IsRetryable checks if the error is retryable
func (e *ErrorResponse) IsRetryable() bool {
	if e.RetryStrategy != nil {
		return e.RetryStrategy.Retryable
	}

	// Default retry logic based on error codes
	switch e.Code {
	case ErrorCodeTimeout, ErrorCodeNetworkError, ErrorCodeConnectionFailed,
		ErrorCodeServiceOffline, ErrorCodeRateLimit, ErrorCodeConcurrencyLimit:
		return true
	case ErrorCodeInternal, ErrorCodeDatabaseError, ErrorCodeCacheError:
		return true // Usually retryable after brief delay
	default:
		return false
	}
}

// GetSeverityPriority returns numeric priority for severity (higher = more severe)
func (e *ErrorResponse) GetSeverityPriority() int {
	switch e.Severity {
	case SeverityInfo:
		return 1
	case SeverityWarning:
		return 2
	case SeverityError:
		return 3
	case SeverityCritical:
		return 4
	case SeverityFatal:
		return 5
	default:
		return 3 // Default to error level
	}
}

// NewErrorResponse creates a new error response with defaults
func NewErrorResponse(code ErrorCode, message string) *ErrorResponse {
	return &ErrorResponse{
		Code:      code,
		Message:   message,
		Severity:  SeverityError,
		Category:  getCategoryForCode(code),
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
}

// Helper function to determine category from error code
func getCategoryForCode(code ErrorCode) ErrorCategory {
	switch code {
	case ErrorCodeAuthFailed, ErrorCodeUnauthorized, ErrorCodeTokenExpired,
		ErrorCodeInvalidAPIKey, ErrorCodePermissionDenied, ErrorCodeAccountSuspended, ErrorCodeMFARequired:
		return CategoryAuth
	case ErrorCodeNotFound, ErrorCodeAlreadyExists, ErrorCodeConflict,
		ErrorCodeResourceLocked, ErrorCodeResourceDeleted, ErrorCodeResourceExpired:
		return CategoryResource
	case ErrorCodeRateLimit, ErrorCodeQuotaExceeded, ErrorCodeConcurrencyLimit,
		ErrorCodeBandwidthLimit, ErrorCodeStorageLimit:
		return CategoryRateLimit
	case ErrorCodeValidation, ErrorCodeInvalidInput, ErrorCodeMissingParameter,
		ErrorCodeInvalidFormat, ErrorCodeOutOfRange, ErrorCodeTypeMismatch:
		return CategoryValidation
	case ErrorCodeTimeout, ErrorCodeNetworkError, ErrorCodeConnectionFailed,
		ErrorCodeServiceOffline, ErrorCodeDNSResolution, ErrorCodeSSLError:
		return CategoryNetwork
	case ErrorCodeUpstreamError, ErrorCodeProviderError, ErrorCodeIntegrationError,
		ErrorCodeWebhookFailed, ErrorCodeAPIError:
		return CategoryExternal
	case ErrorCodeInternal, ErrorCodeDatabaseError, ErrorCodeCacheError,
		ErrorCodeConfigError, ErrorCodeInitError, ErrorCodeShutdown:
		return CategorySystem
	case ErrorCodeProtocolError, ErrorCodeVersionMismatch, ErrorCodeEncodingError,
		ErrorCodeParsingError, ErrorCodeSerializationErr:
		return CategoryProtocol
	default:
		return CategoryBusiness
	}
}

// Builder methods for fluent API

// WithDetails adds details to the error
func (e *ErrorResponse) WithDetails(details string) *ErrorResponse {
	e.Details = details
	return e
}

// WithSeverity sets the error severity
func (e *ErrorResponse) WithSeverity(severity ErrorSeverity) *ErrorResponse {
	e.Severity = severity
	return e
}

// WithRequestID adds request ID for tracing
func (e *ErrorResponse) WithRequestID(requestID string) *ErrorResponse {
	e.RequestID = requestID
	return e
}

// WithOperation adds the operation context
func (e *ErrorResponse) WithOperation(operation string) *ErrorResponse {
	e.Operation = operation
	return e
}

// WithService adds the service context
func (e *ErrorResponse) WithService(service string) *ErrorResponse {
	e.Service = service
	return e
}

// WithVersion adds the version context
func (e *ErrorResponse) WithVersion(version string) *ErrorResponse {
	e.Version = version
	return e
}

// WithResource adds affected resource information
func (e *ErrorResponse) WithResource(resource ResourceInfo) *ErrorResponse {
	e.Resource = &resource
	return e
}

// WithSuggestion adds a recovery suggestion
func (e *ErrorResponse) WithSuggestion(suggestion string) *ErrorResponse {
	e.Suggestion = suggestion
	return e
}

// WithDocumentation adds documentation link
func (e *ErrorResponse) WithDocumentation(documentation string) *ErrorResponse {
	e.Documentation = documentation
	return e
}

// WithRetryAfter sets retry after duration
func (e *ErrorResponse) WithRetryAfter(duration time.Duration) *ErrorResponse {
	e.RetryAfter = &duration
	return e
}

// WithRateLimitInfo adds rate limit information
func (e *ErrorResponse) WithRateLimitInfo(info RateLimitInfo) *ErrorResponse {
	e.RateLimitInfo = &info
	return e
}

// WithMetadata adds metadata key-value pair
func (e *ErrorResponse) WithMetadata(key string, value interface{}) *ErrorResponse {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// WithRecoverySteps adds recovery steps
func (e *ErrorResponse) WithRecoverySteps(steps []RecoveryStep) *ErrorResponse {
	e.RecoverySteps = steps
	return e
}

// WithRetryStrategy adds retry strategy
func (e *ErrorResponse) WithRetryStrategy(strategy RetryStrategy) *ErrorResponse {
	e.RetryStrategy = &strategy
	return e
}

// WithInnerError adds nested error for error chains
func (e *ErrorResponse) WithInnerError(inner *ErrorResponse) *ErrorResponse {
	e.InnerError = inner
	return e
}

// Common error constructors for convenience

// NewAuthError creates an authentication error
func NewAuthError(message string) *ErrorResponse {
	return NewErrorResponse(ErrorCodeAuthFailed, message).
		WithSuggestion("Please check your credentials and try again")
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resourceType, resourceID string) *ErrorResponse {
	return NewErrorResponse(ErrorCodeNotFound, fmt.Sprintf("%s not found", resourceType)).
		WithResource(ResourceInfo{
			Type: resourceType,
			ID:   resourceID,
		}).
		WithSuggestion(fmt.Sprintf("Verify that the %s exists and you have access", resourceType))
}

// NewRateLimitError creates a rate limit error
func NewRateLimitError(limit, remaining int, reset time.Time) *ErrorResponse {
	retryAfter := time.Until(reset)
	return NewErrorResponse(ErrorCodeRateLimit, "Rate limit exceeded").
		WithRetryAfter(retryAfter).
		WithRateLimitInfo(RateLimitInfo{
			Limit:     limit,
			Remaining: remaining,
			Reset:     reset,
		}).
		WithSuggestion(fmt.Sprintf("Wait %v before retrying", retryAfter.Round(time.Second)))
}

// NewValidationError creates a validation error
func NewValidationError(field, issue string) *ErrorResponse {
	return NewErrorResponse(ErrorCodeValidation, fmt.Sprintf("Validation failed for field '%s'", field)).
		WithDetails(issue).
		WithMetadata("field", field).
		WithSeverity(SeverityWarning).
		WithSuggestion("Check the input format and constraints for this field")
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(operation string, timeout time.Duration) *ErrorResponse {
	return NewErrorResponse(ErrorCodeTimeout, fmt.Sprintf("Operation '%s' timed out", operation)).
		WithOperation(operation).
		WithMetadata("timeout_seconds", timeout.Seconds()).
		WithRetryStrategy(RetryStrategy{
			Retryable:    true,
			MaxAttempts:  3,
			BackoffType:  "exponential",
			InitialDelay: 1 * time.Second,
			MaxDelay:     30 * time.Second,
		}).
		WithSuggestion("Try again with a longer timeout or reduce the request size")
}
