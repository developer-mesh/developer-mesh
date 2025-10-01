package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorResponse_BasicConstruction(t *testing.T) {
	tests := []struct {
		name         string
		code         ErrorCode
		message      string
		wantCategory ErrorCategory
		wantSeverity ErrorSeverity
	}{
		{
			name:         "auth error",
			code:         ErrorCodeAuthFailed,
			message:      "Invalid credentials",
			wantCategory: CategoryAuth,
			wantSeverity: SeverityError,
		},
		{
			name:         "not found error",
			code:         ErrorCodeNotFound,
			message:      "Repository not found",
			wantCategory: CategoryResource,
			wantSeverity: SeverityError,
		},
		{
			name:         "rate limit error",
			code:         ErrorCodeRateLimit,
			message:      "API rate limit exceeded",
			wantCategory: CategoryRateLimit,
			wantSeverity: SeverityError,
		},
		{
			name:         "validation error",
			code:         ErrorCodeValidation,
			message:      "Invalid input format",
			wantCategory: CategoryValidation,
			wantSeverity: SeverityError,
		},
		{
			name:         "network error",
			code:         ErrorCodeTimeout,
			message:      "Connection timeout",
			wantCategory: CategoryNetwork,
			wantSeverity: SeverityError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewErrorResponse(tt.code, tt.message)

			assert.Equal(t, tt.code, err.Code)
			assert.Equal(t, tt.message, err.Message)
			assert.Equal(t, tt.wantCategory, err.Category)
			assert.Equal(t, tt.wantSeverity, err.Severity)
			assert.NotZero(t, err.Timestamp)
			assert.NotNil(t, err.Metadata)
		})
	}
}

func TestErrorResponse_FluentAPI(t *testing.T) {
	requestID := "req-123"
	operation := "github.createIssue"
	details := "Missing required field 'title'"
	suggestion := "Provide a title for the issue"

	err := NewErrorResponse(ErrorCodeValidation, "Validation failed").
		WithDetails(details).
		WithRequestID(requestID).
		WithOperation(operation).
		WithSeverity(SeverityWarning).
		WithSuggestion(suggestion).
		WithMetadata("field", "title").
		WithMetadata("required", true)

	assert.Equal(t, ErrorCodeValidation, err.Code)
	assert.Equal(t, details, err.Details)
	assert.Equal(t, requestID, err.RequestID)
	assert.Equal(t, operation, err.Operation)
	assert.Equal(t, SeverityWarning, err.Severity)
	assert.Equal(t, suggestion, err.Suggestion)
	assert.Equal(t, "title", err.Metadata["field"])
	assert.Equal(t, true, err.Metadata["required"])
}

func TestErrorResponse_WithResource(t *testing.T) {
	resource := ResourceInfo{
		Type:  "repository",
		ID:    "123",
		Name:  "test-repo",
		Owner: "testuser",
		State: "private",
		Attributes: map[string]interface{}{
			"stars": 42,
		},
	}

	err := NewErrorResponse(ErrorCodeNotFound, "Repository not found").
		WithResource(resource)

	assert.NotNil(t, err.Resource)
	assert.Equal(t, "repository", err.Resource.Type)
	assert.Equal(t, "123", err.Resource.ID)
	assert.Equal(t, "test-repo", err.Resource.Name)
	assert.Equal(t, "testuser", err.Resource.Owner)
	assert.Equal(t, "private", err.Resource.State)
	assert.Equal(t, 42, err.Resource.Attributes["stars"])
}

func TestErrorResponse_WithRetryStrategy(t *testing.T) {
	strategy := RetryStrategy{
		Retryable:      true,
		MaxAttempts:    5,
		BackoffType:    "exponential",
		InitialDelay:   2 * time.Second,
		MaxDelay:       60 * time.Second,
		RetryCondition: "status_code == 503",
	}

	err := NewErrorResponse(ErrorCodeServiceOffline, "Service temporarily unavailable").
		WithRetryStrategy(strategy)

	assert.NotNil(t, err.RetryStrategy)
	assert.True(t, err.RetryStrategy.Retryable)
	assert.Equal(t, 5, err.RetryStrategy.MaxAttempts)
	assert.Equal(t, "exponential", err.RetryStrategy.BackoffType)
	assert.Equal(t, 2*time.Second, err.RetryStrategy.InitialDelay)
	assert.Equal(t, 60*time.Second, err.RetryStrategy.MaxDelay)
}

func TestErrorResponse_WithRateLimitInfo(t *testing.T) {
	reset := time.Now().Add(15 * time.Minute)
	retryAfter := 15 * time.Minute

	err := NewErrorResponse(ErrorCodeRateLimit, "Rate limit exceeded").
		WithRetryAfter(retryAfter).
		WithRateLimitInfo(RateLimitInfo{
			Limit:     5000,
			Remaining: 0,
			Reset:     reset,
			Window:    1 * time.Hour,
			Scope:     "user",
			TierInfo:  "free tier - upgrade for higher limits",
		})

	assert.NotNil(t, err.RetryAfter)
	assert.Equal(t, retryAfter, *err.RetryAfter)
	assert.NotNil(t, err.RateLimitInfo)
	assert.Equal(t, 5000, err.RateLimitInfo.Limit)
	assert.Equal(t, 0, err.RateLimitInfo.Remaining)
	assert.Equal(t, reset, err.RateLimitInfo.Reset)
	assert.Equal(t, 1*time.Hour, err.RateLimitInfo.Window)
	assert.Equal(t, "user", err.RateLimitInfo.Scope)
}

func TestErrorResponse_WithRecoverySteps(t *testing.T) {
	steps := []RecoveryStep{
		{
			Order:       1,
			Action:      "refresh_token",
			Description: "Refresh the authentication token",
			Tool:        "auth.refreshToken",
			Parameters: map[string]interface{}{
				"grant_type": "refresh_token",
			},
			Optional: false,
		},
		{
			Order:       2,
			Action:      "retry_request",
			Description: "Retry the original request with new token",
			Tool:        "request.retry",
			Optional:    false,
		},
		{
			Order:       3,
			Action:      "fallback",
			Description: "Use fallback authentication method",
			Tool:        "auth.fallback",
			Optional:    true,
		},
	}

	err := NewErrorResponse(ErrorCodeTokenExpired, "Authentication token expired").
		WithRecoverySteps(steps)

	assert.Len(t, err.RecoverySteps, 3)
	assert.Equal(t, "refresh_token", err.RecoverySteps[0].Action)
	assert.Equal(t, "auth.refreshToken", err.RecoverySteps[0].Tool)
	assert.False(t, err.RecoverySteps[0].Optional)
	assert.True(t, err.RecoverySteps[2].Optional)
}

func TestErrorResponse_WithInnerError(t *testing.T) {
	innerErr := NewErrorResponse(ErrorCodeDatabaseError, "Connection pool exhausted").
		WithDetails("No available database connections")

	outerErr := NewErrorResponse(ErrorCodeInternal, "Failed to process request").
		WithInnerError(innerErr)

	assert.NotNil(t, outerErr.InnerError)
	assert.Equal(t, ErrorCodeDatabaseError, outerErr.InnerError.Code)
	assert.Equal(t, "Connection pool exhausted", outerErr.InnerError.Message)
}

func TestErrorResponse_IsRetryable(t *testing.T) {
	tests := []struct {
		name         string
		code         ErrorCode
		withStrategy bool
		retryable    bool
		want         bool
	}{
		{
			name: "timeout is retryable by default",
			code: ErrorCodeTimeout,
			want: true,
		},
		{
			name: "network error is retryable by default",
			code: ErrorCodeNetworkError,
			want: true,
		},
		{
			name: "rate limit is retryable by default",
			code: ErrorCodeRateLimit,
			want: true,
		},
		{
			name: "auth failed is not retryable by default",
			code: ErrorCodeAuthFailed,
			want: false,
		},
		{
			name: "validation error is not retryable by default",
			code: ErrorCodeValidation,
			want: false,
		},
		{
			name:         "explicit strategy overrides default",
			code:         ErrorCodeAuthFailed,
			withStrategy: true,
			retryable:    true,
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewErrorResponse(tt.code, "test error")
			if tt.withStrategy {
				err = err.WithRetryStrategy(RetryStrategy{Retryable: tt.retryable})
			}
			assert.Equal(t, tt.want, err.IsRetryable())
		})
	}
}

func TestErrorResponse_GetSeverityPriority(t *testing.T) {
	tests := []struct {
		severity ErrorSeverity
		want     int
	}{
		{SeverityInfo, 1},
		{SeverityWarning, 2},
		{SeverityError, 3},
		{SeverityCritical, 4},
		{SeverityFatal, 5},
		{ErrorSeverity("unknown"), 3}, // default
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			err := NewErrorResponse(ErrorCodeInternal, "test").
				WithSeverity(tt.severity)
			assert.Equal(t, tt.want, err.GetSeverityPriority())
		})
	}
}

func TestErrorResponse_ErrorInterface(t *testing.T) {
	tests := []struct {
		name string
		err  *ErrorResponse
		want string
	}{
		{
			name: "with details",
			err: NewErrorResponse(ErrorCodeAuthFailed, "Authentication failed").
				WithDetails("Invalid API key"),
			want: "[AUTH_FAILED] Authentication failed: Invalid API key",
		},
		{
			name: "without details",
			err:  NewErrorResponse(ErrorCodeNotFound, "Resource not found"),
			want: "[NOT_FOUND] Resource not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

func TestErrorResponse_ToJSON(t *testing.T) {
	err := NewErrorResponse(ErrorCodeValidation, "Validation failed").
		WithDetails("Missing required field").
		WithRequestID("req-123").
		WithSeverity(SeverityWarning).
		WithMetadata("field", "name")

	jsonData, jsonErr := err.ToJSON()
	require.NoError(t, jsonErr)
	assert.NotEmpty(t, jsonData)

	// Verify it's valid JSON
	var parsed map[string]interface{}
	parseErr := json.Unmarshal(jsonData, &parsed)
	require.NoError(t, parseErr)

	assert.Equal(t, "VALIDATION_ERROR", parsed["code"])
	assert.Equal(t, "Validation failed", parsed["message"])
	assert.Equal(t, "Missing required field", parsed["details"])
	assert.Equal(t, "req-123", parsed["request_id"])
	assert.Equal(t, "WARNING", parsed["severity"])
}

func TestNewAuthError(t *testing.T) {
	err := NewAuthError("Invalid API key")

	assert.Equal(t, ErrorCodeAuthFailed, err.Code)
	assert.Equal(t, "Invalid API key", err.Message)
	assert.Equal(t, CategoryAuth, err.Category)
	assert.Contains(t, err.Suggestion, "check your credentials")
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("repository", "org/repo")

	assert.Equal(t, ErrorCodeNotFound, err.Code)
	assert.Equal(t, "repository not found", err.Message)
	assert.NotNil(t, err.Resource)
	assert.Equal(t, "repository", err.Resource.Type)
	assert.Equal(t, "org/repo", err.Resource.ID)
	assert.Contains(t, err.Suggestion, "Verify that the repository exists")
}

func TestNewRateLimitError(t *testing.T) {
	reset := time.Now().Add(30 * time.Minute)
	err := NewRateLimitError(5000, 0, reset)

	assert.Equal(t, ErrorCodeRateLimit, err.Code)
	assert.Equal(t, "Rate limit exceeded", err.Message)
	assert.NotNil(t, err.RetryAfter)
	assert.NotNil(t, err.RateLimitInfo)
	assert.Equal(t, 5000, err.RateLimitInfo.Limit)
	assert.Equal(t, 0, err.RateLimitInfo.Remaining)
	assert.Equal(t, reset, err.RateLimitInfo.Reset)
	assert.Contains(t, err.Suggestion, "Wait")
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("email", "Invalid email format")

	assert.Equal(t, ErrorCodeValidation, err.Code)
	assert.Contains(t, err.Message, "email")
	assert.Equal(t, "Invalid email format", err.Details)
	assert.Equal(t, SeverityWarning, err.Severity)
	assert.Equal(t, "email", err.Metadata["field"])
}

func TestNewTimeoutError(t *testing.T) {
	err := NewTimeoutError("fetchRepository", 30*time.Second)

	assert.Equal(t, ErrorCodeTimeout, err.Code)
	assert.Contains(t, err.Message, "fetchRepository")
	assert.Equal(t, "fetchRepository", err.Operation)
	assert.NotNil(t, err.RetryStrategy)
	assert.True(t, err.RetryStrategy.Retryable)
	assert.Equal(t, 3, err.RetryStrategy.MaxAttempts)
	assert.Equal(t, "exponential", err.RetryStrategy.BackoffType)
}

func TestGetCategoryForCode(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected ErrorCategory
	}{
		// Auth errors
		{ErrorCodeAuthFailed, CategoryAuth},
		{ErrorCodeTokenExpired, CategoryAuth},
		{ErrorCodePermissionDenied, CategoryAuth},

		// Resource errors
		{ErrorCodeNotFound, CategoryResource},
		{ErrorCodeAlreadyExists, CategoryResource},
		{ErrorCodeResourceLocked, CategoryResource},

		// Rate limit errors
		{ErrorCodeRateLimit, CategoryRateLimit},
		{ErrorCodeQuotaExceeded, CategoryRateLimit},
		{ErrorCodeConcurrencyLimit, CategoryRateLimit},

		// Validation errors
		{ErrorCodeValidation, CategoryValidation},
		{ErrorCodeInvalidInput, CategoryValidation},
		{ErrorCodeMissingParameter, CategoryValidation},

		// Network errors
		{ErrorCodeTimeout, CategoryNetwork},
		{ErrorCodeNetworkError, CategoryNetwork},
		{ErrorCodeConnectionFailed, CategoryNetwork},

		// External service errors
		{ErrorCodeUpstreamError, CategoryExternal},
		{ErrorCodeProviderError, CategoryExternal},
		{ErrorCodeWebhookFailed, CategoryExternal},

		// System errors
		{ErrorCodeInternal, CategorySystem},
		{ErrorCodeDatabaseError, CategorySystem},
		{ErrorCodeCacheError, CategorySystem},

		// Protocol errors
		{ErrorCodeProtocolError, CategoryProtocol},
		{ErrorCodeVersionMismatch, CategoryProtocol},
		{ErrorCodeEncodingError, CategoryProtocol},

		// Business logic errors (default)
		{ErrorCodeInvalidOperation, CategoryBusiness},
		{ErrorCodeWorkflowError, CategoryBusiness},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			category := getCategoryForCode(tt.code)
			assert.Equal(t, tt.expected, category)
		})
	}
}

func TestErrorResponse_ComplexScenario(t *testing.T) {
	// Simulate a complex error scenario with nested errors and full context
	dbErr := NewErrorResponse(ErrorCodeDatabaseError, "Connection timeout").
		WithDetails("Failed to connect to primary database after 30 seconds").
		WithMetadata("host", "db-primary.example.com").
		WithMetadata("port", 5432)

	cacheErr := NewErrorResponse(ErrorCodeCacheError, "Cache unavailable").
		WithDetails("Redis connection refused")

	mainErr := NewErrorResponse(ErrorCodeInternal, "Failed to fetch user profile").
		WithRequestID("req-abc123").
		WithOperation("user.getProfile").
		WithService("user-service").
		WithVersion("v2.1.0").
		WithSeverity(SeverityCritical).
		WithInnerError(dbErr).
		WithResource(ResourceInfo{
			Type:  "user",
			ID:    "user-456",
			Name:  "john.doe",
			State: "active",
		}).
		WithSuggestion("The service is experiencing database issues. Please try again in a few minutes.").
		WithDocumentation("https://docs.example.com/errors/internal-error").
		WithRecoverySteps([]RecoveryStep{
			{
				Order:       1,
				Action:      "wait",
				Description: "Wait 30 seconds for the database to recover",
				Optional:    false,
			},
			{
				Order:       2,
				Action:      "retry",
				Description: "Retry the request",
				Tool:        "request.retry",
				Parameters: map[string]interface{}{
					"max_attempts": 3,
					"backoff":      "exponential",
				},
				Optional: false,
			},
			{
				Order:       3,
				Action:      "use_cache",
				Description: "Try to fetch from cache if available",
				Tool:        "cache.get",
				Optional:    true,
			},
		}).
		WithRetryStrategy(RetryStrategy{
			Retryable:    true,
			MaxAttempts:  5,
			BackoffType:  "exponential",
			InitialDelay: 5 * time.Second,
			MaxDelay:     2 * time.Minute,
		}).
		WithMetadata("cache_error", cacheErr.Error()).
		WithMetadata("fallback_available", false)

	// Verify the complex error structure
	assert.Equal(t, ErrorCodeInternal, mainErr.Code)
	assert.Equal(t, "req-abc123", mainErr.RequestID)
	assert.Equal(t, "user.getProfile", mainErr.Operation)
	assert.Equal(t, "user-service", mainErr.Service)
	assert.Equal(t, SeverityCritical, mainErr.Severity)
	assert.NotNil(t, mainErr.InnerError)
	assert.Equal(t, ErrorCodeDatabaseError, mainErr.InnerError.Code)
	assert.NotNil(t, mainErr.Resource)
	assert.Equal(t, "user", mainErr.Resource.Type)
	assert.Len(t, mainErr.RecoverySteps, 3)
	assert.True(t, mainErr.IsRetryable())

	// Test JSON serialization of complex error
	jsonData, err := mainErr.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Verify it can be parsed back
	var parsed ErrorResponse
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)
	assert.Equal(t, mainErr.Code, parsed.Code)
	assert.Equal(t, mainErr.RequestID, parsed.RequestID)
}

func TestErrorCodes_Completeness(t *testing.T) {
	// Test that all error codes have proper categorization
	errorCodes := []ErrorCode{
		ErrorCodeAuthFailed, ErrorCodeUnauthorized, ErrorCodeTokenExpired,
		ErrorCodeNotFound, ErrorCodeAlreadyExists, ErrorCodeConflict,
		ErrorCodeRateLimit, ErrorCodeQuotaExceeded,
		ErrorCodeValidation, ErrorCodeInvalidInput,
		ErrorCodeTimeout, ErrorCodeNetworkError,
		ErrorCodeUpstreamError, ErrorCodeProviderError,
		ErrorCodeInternal, ErrorCodeDatabaseError,
		ErrorCodeProtocolError, ErrorCodeVersionMismatch,
		ErrorCodeInvalidOperation, ErrorCodeWorkflowError,
	}

	for _, code := range errorCodes {
		t.Run(string(code), func(t *testing.T) {
			err := NewErrorResponse(code, "test")
			assert.NotEmpty(t, err.Category, "Error code %s should have a category", code)
			assert.NotEmpty(t, err.Code, "Error code should be set")
			assert.NotZero(t, err.Timestamp, "Timestamp should be set")
		})
	}
}
