package errors

import (
	"context"
	"fmt"
	"time"
)

// ErrorClass represents the classification of an error
type ErrorClass int

const (
	// ClassUnknown indicates an unclassified error
	ClassUnknown ErrorClass = iota
	// ClassTransient indicates a temporary error that may be retried
	ClassTransient
	// ClassPermanent indicates a permanent error that should not be retried
	ClassPermanent
	// ClassRateLimited indicates rate limiting
	ClassRateLimited
	// ClassTimeout indicates a timeout error
	ClassTimeout
	// ClassCircuitBreaker indicates circuit breaker is open
	ClassCircuitBreaker
	// ClassValidation indicates input validation error
	ClassValidation
	// ClassAuthentication indicates auth failure
	ClassAuthentication
	// ClassNotFound indicates resource not found
	ClassNotFound
	// ClassConflict indicates a conflict (e.g., concurrent modification)
	ClassConflict
)

// RetryStrategy defines how to retry an operation
type RetryStrategy struct {
	// ShouldRetry indicates if the error is retryable
	ShouldRetry bool `json:"should_retry"`
	// MaxAttempts is the maximum number of retry attempts
	MaxAttempts int `json:"max_attempts"`
	// BaseDelay is the initial delay between retries
	BaseDelay time.Duration `json:"base_delay"`
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration `json:"max_delay"`
	// BackoffMultiplier for exponential backoff
	BackoffMultiplier float64 `json:"backoff_multiplier"`
	// RetryAfter specific time to retry (for rate limiting)
	RetryAfter *time.Time `json:"retry_after,omitempty"`
}

// ClassifiedError is an error with classification and retry information
type ClassifiedError struct {
	// Core error information
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Class   ErrorClass  `json:"class"`
	Details interface{} `json:"details,omitempty"`

	// Context information
	Service       string            `json:"service"`
	Operation     string            `json:"operation"`
	CorrelationID string            `json:"correlation_id,omitempty"`
	Timestamp     time.Time         `json:"timestamp"`
	Metadata      map[string]string `json:"metadata,omitempty"`

	// Retry information
	Retry *RetryStrategy `json:"retry,omitempty"`

	// Original error for unwrapping
	cause error
}

// Error implements the error interface
func (e *ClassifiedError) Error() string {
	if e.CorrelationID != "" {
		return fmt.Sprintf("[%s] %s: %s (correlation_id: %s)",
			e.Code, e.Operation, e.Message, e.CorrelationID)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Code, e.Operation, e.Message)
}

// Unwrap returns the underlying error
func (e *ClassifiedError) Unwrap() error {
	return e.cause
}

// IsRetryable returns true if the error should be retried
func (e *ClassifiedError) IsRetryable() bool {
	return e.Retry != nil && e.Retry.ShouldRetry
}

// GetRetryDelay calculates the retry delay for a given attempt
func (e *ClassifiedError) GetRetryDelay(attempt int) time.Duration {
	if e.Retry == nil || !e.Retry.ShouldRetry {
		return 0
	}

	// If we have a specific retry-after time, use it
	if e.Retry.RetryAfter != nil {
		return time.Until(*e.Retry.RetryAfter)
	}

	// Calculate exponential backoff
	delay := e.Retry.BaseDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * e.Retry.BackoffMultiplier)
		if delay > e.Retry.MaxDelay {
			delay = e.Retry.MaxDelay
			break
		}
	}

	return delay
}

// New creates a new classified error
func New(code string, message string, class ErrorClass) *ClassifiedError {
	return &ClassifiedError{
		Code:      code,
		Message:   message,
		Class:     class,
		Timestamp: time.Now(),
		Retry:     getDefaultRetryStrategy(class),
	}
}

// Wrap wraps an existing error with classification
func Wrap(err error, code string, class ErrorClass) *ClassifiedError {
	if err == nil {
		return nil
	}

	// If already a classified error, preserve the chain
	if ce, ok := err.(*ClassifiedError); ok {
		return &ClassifiedError{
			Code:      code,
			Message:   ce.Message,
			Class:     class,
			Details:   ce.Details,
			Service:   ce.Service,
			Operation: ce.Operation,
			Timestamp: time.Now(),
			Metadata:  ce.Metadata,
			Retry:     getDefaultRetryStrategy(class),
			cause:     err,
		}
	}

	return &ClassifiedError{
		Code:      code,
		Message:   err.Error(),
		Class:     class,
		Timestamp: time.Now(),
		Retry:     getDefaultRetryStrategy(class),
		cause:     err,
	}
}

// WithContext adds context information to the error
func (e *ClassifiedError) WithContext(ctx context.Context, service, operation string) *ClassifiedError {
	e.Service = service
	e.Operation = operation

	// Extract correlation ID from context if available
	if correlationID, ok := ctx.Value("correlation_id").(string); ok {
		e.CorrelationID = correlationID
	}

	return e
}

// WithDetails adds additional details to the error
func (e *ClassifiedError) WithDetails(details interface{}) *ClassifiedError {
	e.Details = details
	return e
}

// WithMetadata adds metadata to the error
func (e *ClassifiedError) WithMetadata(key, value string) *ClassifiedError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]string)
	}
	e.Metadata[key] = value
	return e
}

// WithRetryStrategy sets a custom retry strategy
func (e *ClassifiedError) WithRetryStrategy(retry *RetryStrategy) *ClassifiedError {
	e.Retry = retry
	return e
}

// getDefaultRetryStrategy returns default retry strategy based on error class
func getDefaultRetryStrategy(class ErrorClass) *RetryStrategy {
	switch class {
	case ClassTransient:
		return &RetryStrategy{
			ShouldRetry:       true,
			MaxAttempts:       3,
			BaseDelay:         1 * time.Second,
			MaxDelay:          30 * time.Second,
			BackoffMultiplier: 2.0,
		}
	case ClassTimeout:
		return &RetryStrategy{
			ShouldRetry:       true,
			MaxAttempts:       2,
			BaseDelay:         2 * time.Second,
			MaxDelay:          10 * time.Second,
			BackoffMultiplier: 1.5,
		}
	case ClassRateLimited:
		return &RetryStrategy{
			ShouldRetry:       true,
			MaxAttempts:       5,
			BaseDelay:         5 * time.Second,
			MaxDelay:          60 * time.Second,
			BackoffMultiplier: 1.0, // Linear backoff for rate limiting
		}
	case ClassCircuitBreaker:
		return &RetryStrategy{
			ShouldRetry:       true,
			MaxAttempts:       1,
			BaseDelay:         30 * time.Second,
			MaxDelay:          30 * time.Second,
			BackoffMultiplier: 1.0,
		}
	default:
		// Non-retryable by default
		return &RetryStrategy{
			ShouldRetry: false,
		}
	}
}

// ClassifyHTTPError classifies an HTTP status code
func ClassifyHTTPError(statusCode int) ErrorClass {
	switch {
	case statusCode >= 400 && statusCode < 404:
		return ClassValidation
	case statusCode == 401:
		return ClassAuthentication
	case statusCode == 403:
		return ClassAuthentication
	case statusCode == 404:
		return ClassNotFound
	case statusCode == 409:
		return ClassConflict
	case statusCode == 429:
		return ClassRateLimited
	case statusCode >= 500 && statusCode < 600:
		return ClassTransient
	case statusCode == 503:
		return ClassCircuitBreaker
	case statusCode == 504:
		return ClassTimeout
	default:
		return ClassUnknown
	}
}

// IsTransient returns true if the error is transient and may be retried
func IsTransient(err error) bool {
	if ce, ok := err.(*ClassifiedError); ok {
		return ce.Class == ClassTransient || ce.Class == ClassTimeout
	}
	return false
}

// IsRateLimited returns true if the error is due to rate limiting
func IsRateLimited(err error) bool {
	if ce, ok := err.(*ClassifiedError); ok {
		return ce.Class == ClassRateLimited
	}
	return false
}

// IsCircuitBreakerOpen returns true if the error is due to circuit breaker
func IsCircuitBreakerOpen(err error) bool {
	if ce, ok := err.(*ClassifiedError); ok {
		return ce.Class == ClassCircuitBreaker
	}
	return false
}

// IsValidationError returns true if the error is a validation error
func IsValidationError(err error) bool {
	if ce, ok := err.(*ClassifiedError); ok {
		return ce.Class == ClassValidation
	}
	return false
}

// IsAuthenticationError returns true if the error is an authentication error
func IsAuthenticationError(err error) bool {
	if ce, ok := err.(*ClassifiedError); ok {
		return ce.Class == ClassAuthentication
	}
	return false
}
