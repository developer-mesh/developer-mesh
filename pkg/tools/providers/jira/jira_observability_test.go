package jira

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJiraObservabilityManager(t *testing.T) {
	logger := &observability.NoopLogger{}
	config := GetDefaultJiraObservabilityConfig()

	observabilityMgr := NewJiraObservabilityManager(config, logger)

	assert.NotNil(t, observabilityMgr)
	assert.Equal(t, config, observabilityMgr.config)
	assert.Equal(t, logger, observabilityMgr.logger)
	assert.NotNil(t, observabilityMgr.healthStatus)
	assert.False(t, observabilityMgr.healthStatus.Healthy) // Initially false until first health check
}

func TestJiraObservabilityManager_StartOperation(t *testing.T) {
	logger := &observability.NoopLogger{}
	config := GetDefaultJiraObservabilityConfig()
	config.EnableMetrics = true

	observabilityMgr := NewJiraObservabilityManager(config, logger)
	ctx := context.Background()

	// Start an operation
	newCtx, finishFunc := observabilityMgr.StartOperation(ctx, "test_operation")

	assert.NotNil(t, newCtx)
	assert.NotNil(t, finishFunc)

	// Finish the operation successfully
	finishFunc(nil)

	// Finish with an error
	testErr := errors.New("test error")
	finishFunc(testErr)
}

func TestJiraObservabilityManager_StartOperationWithDebugMode(t *testing.T) {
	logger := &observability.NoopLogger{}
	config := GetDefaultJiraObservabilityConfig()
	config.DebugMode = true

	observabilityMgr := NewJiraObservabilityManager(config, logger)
	ctx := context.Background()

	// Start an operation with debug mode enabled
	newCtx, finishFunc := observabilityMgr.StartOperation(ctx, "debug_test_operation")

	assert.NotNil(t, newCtx)
	assert.NotNil(t, finishFunc)
	assert.True(t, observabilityMgr.IsDebugMode())
	assert.NotNil(t, observabilityMgr.debugLogger)

	// Finish the operation
	finishFunc(nil)
}

func TestJiraObservabilityManager_CategorizeError(t *testing.T) {
	logger := &observability.NoopLogger{}
	config := GetDefaultJiraObservabilityConfig()
	config.EnableErrorTracking = true

	observabilityMgr := NewJiraObservabilityManager(config, logger)

	tests := []struct {
		name                string
		err                 error
		expectedType        JiraErrorType
		expectedRecoverable bool
	}{
		{
			name:                "timeout error",
			err:                 context.DeadlineExceeded,
			expectedType:        ErrorTypeTimeout,
			expectedRecoverable: true,
		},
		{
			name:                "authentication error",
			err:                 errors.New("401 unauthorized"),
			expectedType:        ErrorTypeAuthentication,
			expectedRecoverable: false,
		},
		{
			name:                "authorization error",
			err:                 errors.New("403 forbidden"),
			expectedType:        ErrorTypeAuthorization,
			expectedRecoverable: false,
		},
		{
			name:                "not found error",
			err:                 errors.New("404 not found"),
			expectedType:        ErrorTypeNotFound,
			expectedRecoverable: false,
		},
		{
			name:                "rate limit error",
			err:                 errors.New("429 too many requests"),
			expectedType:        ErrorTypeRateLimit,
			expectedRecoverable: true,
		},
		{
			name:                "validation error",
			err:                 errors.New("400 bad request validation failed"),
			expectedType:        ErrorTypeValidation,
			expectedRecoverable: false,
		},
		{
			name:                "server error",
			err:                 errors.New("500 internal server error"),
			expectedType:        ErrorTypeServerError,
			expectedRecoverable: true,
		},
		{
			name:                "network error",
			err:                 errors.New("network connection timeout"),
			expectedType:        ErrorTypeNetwork,
			expectedRecoverable: true,
		},
		{
			name:                "quota exceeded error",
			err:                 errors.New("quota exceeded limit reached"),
			expectedType:        ErrorTypeQuotaExceeded,
			expectedRecoverable: true,
		},
		{
			name:                "configuration error",
			err:                 errors.New("configuration file missing"),
			expectedType:        ErrorTypeConfiguration,
			expectedRecoverable: false,
		},
		{
			name:                "unknown error",
			err:                 errors.New("some random error"),
			expectedType:        ErrorTypeUnknown,
			expectedRecoverable: false,
		},
		{
			name:         "nil error",
			err:          nil,
			expectedType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jiraErr := observabilityMgr.CategorizeError(tt.err, "test_operation", time.Second)

			if tt.err == nil {
				assert.Nil(t, jiraErr)
				return
			}

			require.NotNil(t, jiraErr)
			assert.Equal(t, string(tt.expectedType), string(jiraErr.Type))
			assert.Equal(t, tt.expectedRecoverable, jiraErr.Recoverable)
			assert.Equal(t, "test_operation", jiraErr.Operation)
			assert.Equal(t, time.Second, jiraErr.Duration)
			assert.Equal(t, tt.err, jiraErr.OriginalErr)
			assert.NotZero(t, jiraErr.Timestamp)

			// Test that error message is properly formatted
			errMsg := jiraErr.Error()
			assert.Contains(t, errMsg, string(tt.expectedType))
		})
	}
}

func TestJiraObservabilityManager_CategorizeExistingJiraError(t *testing.T) {
	logger := &observability.NoopLogger{}
	config := GetDefaultJiraObservabilityConfig()

	observabilityMgr := NewJiraObservabilityManager(config, logger)

	// Create an existing JiraError
	existingErr := &JiraError{
		Type:        ErrorTypeAuthentication,
		Message:     "Original error",
		Recoverable: false,
	}

	result := observabilityMgr.CategorizeError(existingErr, "new_operation", 2*time.Second)

	assert.Equal(t, existingErr, result)
	assert.Equal(t, "new_operation", result.Operation) // Should update operation
	assert.Equal(t, 2*time.Second, result.Duration)    // Should update duration
}

func TestJiraObservabilityManager_RecordHTTPMetrics(t *testing.T) {
	logger := &observability.NoopLogger{}
	config := GetDefaultJiraObservabilityConfig()
	config.EnableMetrics = true

	observabilityMgr := NewJiraObservabilityManager(config, logger)

	// Test recording HTTP metrics
	observabilityMgr.RecordHTTPMetrics("GET", "/rest/api/3/issue", 200, time.Millisecond*100)
	observabilityMgr.RecordHTTPMetrics("POST", "/rest/api/3/issue", 201, time.Millisecond*200)
	observabilityMgr.RecordHTTPMetrics("GET", "/rest/api/3/issue", 404, time.Millisecond*50)
	observabilityMgr.RecordHTTPMetrics("POST", "/rest/api/3/issue", 500, time.Millisecond*300)

	// Test with metrics disabled
	config.EnableMetrics = false
	observabilityMgr.config = config
	observabilityMgr.RecordHTTPMetrics("GET", "/rest/api/3/issue", 200, time.Millisecond*100)
}

func TestJiraObservabilityManager_PerformHealthCheck(t *testing.T) {
	logger := &observability.NoopLogger{}
	config := GetDefaultJiraObservabilityConfig()
	config.HealthCheckTimeout = time.Second

	observabilityMgr := NewJiraObservabilityManager(config, logger)

	t.Run("successful health check", func(t *testing.T) {
		healthCheckFunc := func(ctx context.Context) error {
			return nil
		}

		status := observabilityMgr.PerformHealthCheck(context.Background(), healthCheckFunc)

		assert.True(t, status.Healthy)
		assert.Empty(t, status.Errors)
		assert.NotZero(t, status.LastChecked)
		assert.Greater(t, status.ResponseTime, time.Duration(0))
	})

	t.Run("failed health check", func(t *testing.T) {
		expectedErr := errors.New("health check failed")
		healthCheckFunc := func(ctx context.Context) error {
			return expectedErr
		}

		status := observabilityMgr.PerformHealthCheck(context.Background(), healthCheckFunc)

		assert.False(t, status.Healthy)
		assert.Len(t, status.Errors, 1)
		assert.Equal(t, expectedErr.Error(), status.Errors[0])
		assert.NotZero(t, status.LastChecked)
		assert.Greater(t, status.ResponseTime, time.Duration(0))

		// Check categorization details
		assert.Contains(t, status.Details, "error_type")
		assert.Contains(t, status.Details, "recoverable")
	})

	t.Run("health check timeout", func(t *testing.T) {
		healthCheckFunc := func(ctx context.Context) error {
			// Simulate long operation that respects context cancellation
			select {
			case <-ctx.Done():
				return ctx.Err() // Return context error on timeout
			case <-time.After(2 * time.Second):
				return nil
			}
		}

		status := observabilityMgr.PerformHealthCheck(context.Background(), healthCheckFunc)

		assert.False(t, status.Healthy)
		assert.NotEmpty(t, status.Errors)
		assert.Contains(t, status.Details, "error_type")
	})
}

func TestJiraObservabilityManager_GetHealthStatus(t *testing.T) {
	logger := &observability.NoopLogger{}
	config := GetDefaultJiraObservabilityConfig()

	observabilityMgr := NewJiraObservabilityManager(config, logger)

	// Initially should return default status
	status := observabilityMgr.GetHealthStatus()
	assert.False(t, status.Healthy) // Initially false

	// Perform a health check to update status
	healthCheckFunc := func(ctx context.Context) error {
		return nil
	}
	observabilityMgr.PerformHealthCheck(context.Background(), healthCheckFunc)

	// Now should return updated status
	status = observabilityMgr.GetHealthStatus()
	assert.True(t, status.Healthy)
	assert.NotZero(t, status.LastChecked)
}

func TestJiraObservabilityManager_GetObservabilityMetrics(t *testing.T) {
	logger := &observability.NoopLogger{}
	config := GetDefaultJiraObservabilityConfig()
	config.DebugMode = true
	config.EnableMetrics = true
	config.EnableErrorTracking = true

	observabilityMgr := NewJiraObservabilityManager(config, logger)

	metrics := observabilityMgr.GetObservabilityMetrics()

	assert.Contains(t, metrics, "debug_mode")
	assert.Contains(t, metrics, "metrics_enabled")
	assert.Contains(t, metrics, "error_tracking_enabled")
	assert.Contains(t, metrics, "health_check_timeout")
	assert.Contains(t, metrics, "health_check_interval")
	assert.Contains(t, metrics, "current_health_status")

	assert.Equal(t, true, metrics["debug_mode"])
	assert.Equal(t, true, metrics["metrics_enabled"])
	assert.Equal(t, true, metrics["error_tracking_enabled"])
}

func TestJiraError_Error(t *testing.T) {
	tests := []struct {
		name        string
		jiraError   *JiraError
		expectedMsg string
	}{
		{
			name: "error with code",
			jiraError: &JiraError{
				Type:    ErrorTypeAuthentication,
				Code:    "AUTH001",
				Message: "Authentication failed",
			},
			expectedMsg: "authentication (AUTH001): Authentication failed",
		},
		{
			name: "error without code",
			jiraError: &JiraError{
				Type:    ErrorTypeValidation,
				Message: "Validation error",
			},
			expectedMsg: "validation: Validation error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedMsg, tt.jiraError.Error())
		})
	}
}

func TestJiraError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	jiraErr := &JiraError{
		Type:        ErrorTypeNetwork,
		Message:     "Network error",
		OriginalErr: originalErr,
	}

	unwrapped := jiraErr.Unwrap()
	assert.Equal(t, originalErr, unwrapped)
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name       string
		s          string
		substrings []string
		expected   bool
	}{
		{
			name:       "matches first substring",
			s:          "This is a test message",
			substrings: []string{"test", "demo", "sample"},
			expected:   true,
		},
		{
			name:       "matches case insensitive",
			s:          "This is a TEST message",
			substrings: []string{"test", "demo"},
			expected:   true,
		},
		{
			name:       "no match",
			s:          "This is a message",
			substrings: []string{"demo", "sample"},
			expected:   false,
		},
		{
			name:       "empty string",
			s:          "",
			substrings: []string{"test"},
			expected:   false,
		},
		{
			name:       "empty substrings",
			s:          "test message",
			substrings: []string{},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsAny(tt.s, tt.substrings)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDefaultJiraObservabilityConfig(t *testing.T) {
	config := GetDefaultJiraObservabilityConfig()

	assert.False(t, config.DebugMode)
	assert.Equal(t, 30*time.Second, config.HealthCheckTimeout)
	assert.Equal(t, 5*time.Minute, config.HealthCheckInterval)
	assert.True(t, config.EnableMetrics)
	assert.Equal(t, "jira", config.MetricsNamespace)
	assert.True(t, config.EnableErrorTracking)
	assert.Equal(t, 10, config.MaxErrorStackDepth)
}

func TestJiraObservabilityManager_Integration(t *testing.T) {
	logger := &observability.NoopLogger{}
	config := GetDefaultJiraObservabilityConfig()
	config.DebugMode = true
	config.EnableMetrics = true
	config.EnableErrorTracking = true

	observabilityMgr := NewJiraObservabilityManager(config, logger)
	ctx := context.Background()

	// Simulate a complete operation lifecycle
	newCtx, finishFunc := observabilityMgr.StartOperation(ctx, "integration_test_operation")
	assert.NotNil(t, newCtx)
	assert.NotNil(t, finishFunc)

	// Simulate HTTP request
	observabilityMgr.RecordHTTPMetrics("GET", "/rest/api/3/issue/TEST-123", 200, time.Millisecond*150)

	// Simulate an error scenario
	testErr := errors.New("500 internal server error")
	categorizedErr := observabilityMgr.CategorizeError(testErr, "integration_test_operation", time.Millisecond*150)

	require.NotNil(t, categorizedErr)
	assert.Equal(t, ErrorTypeServerError, categorizedErr.Type)
	assert.True(t, categorizedErr.Recoverable)

	// Finish operation with the categorized error
	finishFunc(categorizedErr)

	// Perform a health check
	healthCheckFunc := func(ctx context.Context) error {
		return nil
	}
	status := observabilityMgr.PerformHealthCheck(ctx, healthCheckFunc)
	assert.True(t, status.Healthy)

	// Get metrics
	metrics := observabilityMgr.GetObservabilityMetrics()
	assert.NotEmpty(t, metrics)

	// Verify debug mode
	assert.True(t, observabilityMgr.IsDebugMode())
}
