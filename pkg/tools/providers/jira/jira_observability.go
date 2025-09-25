package jira

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// JiraObservabilityConfig holds Jira-specific observability settings
type JiraObservabilityConfig struct {
	// Debug mode enables verbose logging
	DebugMode bool `yaml:"debug_mode" json:"debug_mode"`

	// Health check configuration
	HealthCheckTimeout  time.Duration `yaml:"health_check_timeout" json:"health_check_timeout"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval" json:"health_check_interval"`

	// Metrics configuration
	EnableMetrics    bool   `yaml:"enable_metrics" json:"enable_metrics"`
	MetricsNamespace string `yaml:"metrics_namespace" json:"metrics_namespace"`

	// Error handling configuration
	EnableErrorTracking bool `yaml:"enable_error_tracking" json:"enable_error_tracking"`
	MaxErrorStackDepth  int  `yaml:"max_error_stack_depth" json:"max_error_stack_depth"`
}

// JiraErrorType represents different categories of Jira errors
type JiraErrorType string

const (
	ErrorTypeAuthentication JiraErrorType = "authentication"
	ErrorTypeAuthorization  JiraErrorType = "authorization"
	ErrorTypeNetwork        JiraErrorType = "network"
	ErrorTypeRateLimit      JiraErrorType = "rate_limit"
	ErrorTypeValidation     JiraErrorType = "validation"
	ErrorTypeNotFound       JiraErrorType = "not_found"
	ErrorTypeServerError    JiraErrorType = "server_error"
	ErrorTypeTimeout        JiraErrorType = "timeout"
	ErrorTypeConfiguration  JiraErrorType = "configuration"
	ErrorTypeQuotaExceeded  JiraErrorType = "quota_exceeded"
	ErrorTypeUnknown        JiraErrorType = "unknown"
)

// JiraError represents a categorized error with additional context
type JiraError struct {
	Type        JiraErrorType          `json:"type"`
	Code        string                 `json:"code,omitempty"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	HTTPStatus  int                    `json:"http_status,omitempty"`
	Recoverable bool                   `json:"recoverable"`
	OriginalErr error                  `json:"-"`
	Timestamp   time.Time              `json:"timestamp"`
	Operation   string                 `json:"operation,omitempty"`
	Duration    time.Duration          `json:"duration,omitempty"`
}

func (e *JiraError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("%s (%s): %s", e.Type, e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *JiraError) Unwrap() error {
	return e.OriginalErr
}

// JiraObservabilityManager provides comprehensive observability for Jira operations
type JiraObservabilityManager struct {
	config      JiraObservabilityConfig
	logger      observability.Logger
	metrics     observability.MetricsClient
	startSpan   observability.StartSpanFunc
	debugLogger observability.Logger

	// Health check state
	lastHealthCheck time.Time
	healthStatus    HealthStatus
}

// HealthStatus represents the health status of the Jira provider
type HealthStatus struct {
	Healthy      bool                   `json:"healthy"`
	LastChecked  time.Time              `json:"last_checked"`
	ResponseTime time.Duration          `json:"response_time"`
	Version      string                 `json:"version,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
	Errors       []string               `json:"errors,omitempty"`
}

// NewJiraObservabilityManager creates a new Jira observability manager
func NewJiraObservabilityManager(config JiraObservabilityConfig, logger observability.Logger) *JiraObservabilityManager {
	// Get observability components from the global defaults
	metrics := observability.DefaultMetricsClient
	startSpan := observability.DefaultStartSpan

	// Create debug logger with prefix if debug mode is enabled
	var debugLogger observability.Logger
	if config.DebugMode {
		debugLogger = logger.WithPrefix("[JIRA-DEBUG]")
	}

	return &JiraObservabilityManager{
		config:      config,
		logger:      logger,
		metrics:     metrics,
		startSpan:   startSpan,
		debugLogger: debugLogger,
		healthStatus: HealthStatus{
			Healthy: false,
			Details: make(map[string]interface{}),
		},
	}
}

// StartOperation creates a new operation span and returns timing function
func (jom *JiraObservabilityManager) StartOperation(ctx context.Context, operation string) (context.Context, func(error)) {
	start := time.Now()

	// Start distributed trace span
	var span observability.Span
	if jom.startSpan != nil {
		ctx, span = jom.startSpan(ctx, operation)
	}

	// Debug logging if enabled
	if jom.debugLogger != nil {
		jom.debugLogger.Debug("Starting Jira operation", map[string]interface{}{
			"operation": operation,
			"timestamp": start,
		})
	}

	// Record operation start metric
	if jom.metrics != nil && jom.config.EnableMetrics {
		jom.metrics.IncrementCounterWithLabels("jira_operations_started_total", 1, map[string]string{
			"operation": operation,
		})
	}

	return ctx, func(err error) {
		duration := time.Since(start)
		success := err == nil

		// End span
		if span != nil {
			if err != nil {
				span.RecordError(err)
				span.SetStatus(2, err.Error()) // Error status
			} else {
				span.SetStatus(1, "OK") // Success status
			}
			span.End()
		}

		// Record metrics
		if jom.metrics != nil && jom.config.EnableMetrics {
			labels := map[string]string{
				"operation": operation,
				"success":   fmt.Sprintf("%t", success),
			}

			jom.metrics.RecordOperation("jira", operation, success, duration.Seconds(), labels)
			jom.metrics.RecordTimer("jira_operation_duration", duration, labels)
			jom.metrics.IncrementCounterWithLabels("jira_operations_completed_total", 1, labels)
		}

		// Debug logging
		if jom.debugLogger != nil {
			fields := map[string]interface{}{
				"operation": operation,
				"duration":  duration,
				"success":   success,
			}

			if err != nil {
				fields["error"] = err.Error()
				if jiraErr, ok := err.(*JiraError); ok {
					fields["error_type"] = string(jiraErr.Type)
					fields["error_code"] = jiraErr.Code
					fields["http_status"] = jiraErr.HTTPStatus
					fields["recoverable"] = jiraErr.Recoverable
				}
			}

			if success {
				jom.debugLogger.Debug("Completed Jira operation", fields)
			} else {
				jom.debugLogger.Warn("Failed Jira operation", fields)
			}
		}

		// Log operation completion
		if success {
			jom.logger.Info("Jira operation completed", map[string]interface{}{
				"operation": operation,
				"duration":  duration,
			})
		} else {
			jom.logger.Error("Jira operation failed", map[string]interface{}{
				"operation": operation,
				"duration":  duration,
				"error":     err.Error(),
			})
		}
	}
}

// CategorizeError categorizes an error and returns a structured JiraError
func (jom *JiraObservabilityManager) CategorizeError(err error, operation string, duration time.Duration) *JiraError {
	if err == nil {
		return nil
	}

	// Check if it's already a JiraError
	if jiraErr, ok := err.(*JiraError); ok {
		if jiraErr.Operation == "" {
			jiraErr.Operation = operation
		}
		if jiraErr.Duration == 0 {
			jiraErr.Duration = duration
		}
		return jiraErr
	}

	jiraErr := &JiraError{
		Message:     err.Error(),
		OriginalErr: err,
		Timestamp:   time.Now(),
		Operation:   operation,
		Duration:    duration,
		Details:     make(map[string]interface{}),
	}

	// Categorize based on error content and type
	errMsg := err.Error()

	switch {
	case errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled):
		jiraErr.Type = ErrorTypeTimeout
		jiraErr.Recoverable = true

	case containsAny(errMsg, []string{"401", "unauthorized", "authentication"}):
		jiraErr.Type = ErrorTypeAuthentication
		jiraErr.HTTPStatus = 401
		jiraErr.Recoverable = false

	case containsAny(errMsg, []string{"403", "forbidden", "access denied"}):
		jiraErr.Type = ErrorTypeAuthorization
		jiraErr.HTTPStatus = 403
		jiraErr.Recoverable = false

	case containsAny(errMsg, []string{"404", "not found"}):
		jiraErr.Type = ErrorTypeNotFound
		jiraErr.HTTPStatus = 404
		jiraErr.Recoverable = false

	case containsAny(errMsg, []string{"429", "rate limit", "too many requests"}):
		jiraErr.Type = ErrorTypeRateLimit
		jiraErr.HTTPStatus = 429
		jiraErr.Recoverable = true

	case containsAny(errMsg, []string{"400", "bad request", "validation", "invalid"}):
		jiraErr.Type = ErrorTypeValidation
		jiraErr.HTTPStatus = 400
		jiraErr.Recoverable = false

	case containsAny(errMsg, []string{"500", "502", "503", "504", "server error", "internal error"}):
		jiraErr.Type = ErrorTypeServerError
		jiraErr.HTTPStatus = 500
		jiraErr.Recoverable = true

	case containsAny(errMsg, []string{"network", "connection", "dns", "timeout"}):
		jiraErr.Type = ErrorTypeNetwork
		jiraErr.Recoverable = true

	case containsAny(errMsg, []string{"quota", "exceeded", "limit reached"}):
		jiraErr.Type = ErrorTypeQuotaExceeded
		jiraErr.Recoverable = true

	case containsAny(errMsg, []string{"configuration", "config", "setting"}):
		jiraErr.Type = ErrorTypeConfiguration
		jiraErr.Recoverable = false

	default:
		jiraErr.Type = ErrorTypeUnknown
		jiraErr.Recoverable = false
	}

	// Record error metrics
	if jom.metrics != nil && jom.config.EnableMetrics && jom.config.EnableErrorTracking {
		labels := map[string]string{
			"operation":   operation,
			"error_type":  string(jiraErr.Type),
			"recoverable": fmt.Sprintf("%t", jiraErr.Recoverable),
		}

		if jiraErr.HTTPStatus > 0 {
			labels["http_status"] = fmt.Sprintf("%d", jiraErr.HTTPStatus)
		}

		jom.metrics.IncrementCounterWithLabels("jira_errors_total", 1, labels)
	}

	return jiraErr
}

// RecordHTTPMetrics records HTTP-specific metrics
func (jom *JiraObservabilityManager) RecordHTTPMetrics(method, endpoint string, statusCode int, duration time.Duration) {
	if jom.metrics == nil || !jom.config.EnableMetrics {
		return
	}

	labels := map[string]string{
		"method":      method,
		"endpoint":    endpoint,
		"status_code": fmt.Sprintf("%d", statusCode),
		"success":     fmt.Sprintf("%t", statusCode < 400),
	}

	jom.metrics.RecordTimer("jira_http_request_duration", duration, labels)
	jom.metrics.IncrementCounterWithLabels("jira_http_requests_total", 1, labels)

	if statusCode >= 400 {
		jom.metrics.IncrementCounterWithLabels("jira_http_errors_total", 1, labels)
	}
}

// PerformHealthCheck performs a comprehensive health check
func (jom *JiraObservabilityManager) PerformHealthCheck(ctx context.Context, healthCheckFunc func(context.Context) error) HealthStatus {
	start := time.Now()

	// Create timeout context for health check
	if jom.config.HealthCheckTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, jom.config.HealthCheckTimeout)
		defer cancel()
	}

	status := HealthStatus{
		LastChecked: start,
		Details:     make(map[string]interface{}),
	}

	// Perform the health check
	err := healthCheckFunc(ctx)
	status.ResponseTime = time.Since(start)

	if err != nil {
		status.Healthy = false
		status.Errors = []string{err.Error()}

		// Categorize the health check error
		jiraErr := jom.CategorizeError(err, "health_check", status.ResponseTime)
		status.Details["error_type"] = string(jiraErr.Type)
		status.Details["recoverable"] = jiraErr.Recoverable

		if jom.debugLogger != nil {
			jom.debugLogger.Error("Health check failed", map[string]interface{}{
				"error":         err.Error(),
				"error_type":    string(jiraErr.Type),
				"response_time": status.ResponseTime,
				"recoverable":   jiraErr.Recoverable,
			})
		}
	} else {
		status.Healthy = true

		if jom.debugLogger != nil {
			jom.debugLogger.Debug("Health check successful", map[string]interface{}{
				"response_time": status.ResponseTime,
			})
		}
	}

	// Record health check metrics
	if jom.metrics != nil && jom.config.EnableMetrics {
		labels := map[string]string{
			"healthy": fmt.Sprintf("%t", status.Healthy),
		}

		jom.metrics.IncrementCounterWithLabels("jira_health_checks_total", 1, labels)
		jom.metrics.RecordTimer("jira_health_check_duration", status.ResponseTime, labels)

		if status.Healthy {
			jom.metrics.RecordGauge("jira_health_status", 1, labels)
		} else {
			jom.metrics.RecordGauge("jira_health_status", 0, labels)
		}
	}

	// Update cached status
	jom.healthStatus = status
	jom.lastHealthCheck = start

	return status
}

// GetHealthStatus returns the current health status
func (jom *JiraObservabilityManager) GetHealthStatus() HealthStatus {
	return jom.healthStatus
}

// IsDebugMode returns whether debug mode is enabled
func (jom *JiraObservabilityManager) IsDebugMode() bool {
	return jom.config.DebugMode
}

// GetObservabilityMetrics returns observability-specific metrics
func (jom *JiraObservabilityManager) GetObservabilityMetrics() map[string]interface{} {
	metrics := map[string]interface{}{
		"debug_mode":             jom.config.DebugMode,
		"metrics_enabled":        jom.config.EnableMetrics,
		"error_tracking_enabled": jom.config.EnableErrorTracking,
		"health_check_timeout":   jom.config.HealthCheckTimeout,
		"health_check_interval":  jom.config.HealthCheckInterval,
		"last_health_check":      jom.lastHealthCheck,
		"current_health_status":  jom.healthStatus.Healthy,
	}

	if !jom.lastHealthCheck.IsZero() {
		metrics["time_since_last_health_check"] = time.Since(jom.lastHealthCheck)
	}

	return metrics
}

// GetDefaultJiraObservabilityConfig returns default observability configuration
func GetDefaultJiraObservabilityConfig() JiraObservabilityConfig {
	return JiraObservabilityConfig{
		DebugMode:           false,
		HealthCheckTimeout:  30 * time.Second,
		HealthCheckInterval: 5 * time.Minute,
		EnableMetrics:       true,
		MetricsNamespace:    "jira",
		EnableErrorTracking: true,
		MaxErrorStackDepth:  10,
	}
}

// Helper function to check if string contains any of the given substrings
func containsAny(s string, substrings []string) bool {
	s = strings.ToLower(s)
	for _, substr := range substrings {
		if strings.Contains(s, strings.ToLower(substr)) {
			return true
		}
	}
	return false
}
