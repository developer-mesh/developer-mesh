package utils

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxAttempts     int              // Maximum number of attempts (including first try)
	InitialDelay    time.Duration    // Initial delay between retries
	MaxDelay        time.Duration    // Maximum delay between retries
	Multiplier      float64          // Multiplier for exponential backoff
	JitterFactor    float64          // Jitter factor (0-1) to randomize delays
	RetryableErrors []error          // Specific errors that trigger retry
	RetryIf         func(error) bool // Custom function to determine if retry
}

// DefaultRetryConfig returns sensible defaults
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0.1,
	}
}

// RetryResult contains retry execution details
type RetryResult struct {
	Attempts      int
	TotalDuration time.Duration
	LastError     error
}

// RetryableError interface for errors that know if they're retryable
type RetryableError interface {
	error
	IsRetryable() bool
}

// RetryWithBackoff executes a function with exponential backoff retry
func RetryWithBackoff(ctx context.Context, config *RetryConfig, fn func() error) (*RetryResult, error) {
	if config == nil {
		config = DefaultRetryConfig()
	}

	result := &RetryResult{}
	startTime := time.Now()

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		result.Attempts = attempt

		// Execute the function
		err := fn()

		// Success - return immediately
		if err == nil {
			result.TotalDuration = time.Since(startTime)
			return result, nil
		}

		result.LastError = err

		// Check if we should retry
		if attempt == config.MaxAttempts {
			// No more retries
			break
		}

		if !shouldRetry(err, config) {
			// Error is not retryable
			break
		}

		// Calculate delay with exponential backoff
		delay := calculateDelay(attempt, config)

		// Wait with context cancellation support
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			// Context cancelled
			result.TotalDuration = time.Since(startTime)
			return result, fmt.Errorf("retry cancelled: %w", ctx.Err())
		}
	}

	result.TotalDuration = time.Since(startTime)
	return result, fmt.Errorf("all %d attempts failed: %w", result.Attempts, result.LastError)
}

// shouldRetry determines if an error warrants a retry
func shouldRetry(err error, config *RetryConfig) bool {
	// Check custom retry function first
	if config.RetryIf != nil {
		return config.RetryIf(err)
	}

	// Check if error implements RetryableError
	var retryable RetryableError
	if errors.As(err, &retryable) {
		return retryable.IsRetryable()
	}

	// Check specific retryable errors
	for _, retryableErr := range config.RetryableErrors {
		if errors.Is(err, retryableErr) {
			return true
		}
	}

	// Default: don't retry
	return false
}

// calculateDelay calculates the delay for the next retry attempt
func calculateDelay(attempt int, config *RetryConfig) time.Duration {
	// Calculate base delay with exponential backoff
	delay := float64(config.InitialDelay) * math.Pow(config.Multiplier, float64(attempt-1))

	// Apply maximum delay cap
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}

	// Add jitter to prevent thundering herd
	if config.JitterFactor > 0 {
		jitter := delay * config.JitterFactor * (rand.Float64()*2 - 1) // -jitter to +jitter
		delay += jitter
	}

	// Ensure delay is not negative
	if delay < 0 {
		delay = 0
	}

	return time.Duration(delay)
}

// Common retryable errors
var (
	ErrTimeout            = errors.New("operation timeout")
	ErrRateLimit          = errors.New("rate limit exceeded")
	ErrServiceUnavailable = errors.New("service temporarily unavailable")
)

// NetworkError represents a retryable network error
type NetworkError struct {
	Message string
}

func (e NetworkError) Error() string {
	return fmt.Sprintf("network error: %s", e.Message)
}

func (e NetworkError) IsRetryable() bool {
	return true
}

// HTTPError represents an HTTP error with status code
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

func (e HTTPError) IsRetryable() bool {
	// Retry on server errors and specific client errors
	switch e.StatusCode {
	case 429, // Too Many Requests
		502, // Bad Gateway
		503, // Service Unavailable
		504: // Gateway Timeout
		return true
	default:
		return e.StatusCode >= 500
	}
}

// IsRetryableHTTPError checks if an error is a retryable HTTP error
func IsRetryableHTTPError(err error) bool {
	// Check if error implements RetryableError interface
	var retryable RetryableError
	if errors.As(err, &retryable) {
		return retryable.IsRetryable()
	}

	// Check for specific error types
	var httpErr HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.IsRetryable()
	}

	var netErr NetworkError
	if errors.As(err, &netErr) {
		return netErr.IsRetryable()
	}

	// Check for common retryable errors
	if errors.Is(err, ErrTimeout) || errors.Is(err, ErrRateLimit) || errors.Is(err, ErrServiceUnavailable) {
		return true
	}

	return false
}
