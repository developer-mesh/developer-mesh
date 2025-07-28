package services

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/core/tool"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// RetryableError represents an error that can be retried
type RetryableError struct {
	StatusCode  int
	ErrorType   string
	RetryAfter  time.Duration
	OriginalErr error
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("%s: %v", e.ErrorType, e.OriginalErr)
}

// RetryHandler manages retry logic for tool executions
type RetryHandler struct {
	logger observability.Logger
}

// NewRetryHandler creates a new retry handler
func NewRetryHandler(logger observability.Logger) *RetryHandler {
	return &RetryHandler{
		logger: logger,
	}
}

// ExecuteWithRetry executes a function with retry logic based on the policy
func (r *RetryHandler) ExecuteWithRetry(
	ctx context.Context,
	toolName string,
	operationName string,
	policy *tool.ToolRetryPolicy,
	fn func() (interface{}, error),
) (interface{}, error) {
	if policy == nil {
		// Default policy if none specified
		policy = &tool.ToolRetryPolicy{
			MaxAttempts:      3,
			InitialDelay:     1 * time.Second,
			MaxDelay:         30 * time.Second,
			Multiplier:       2.0,
			Jitter:           0.1,
			RetryOnTimeout:   true,
			RetryOnRateLimit: true,
		}
	}

	var lastErr error
	delay := policy.InitialDelay

	for attempt := 0; attempt < policy.MaxAttempts; attempt++ {
		// Check context before each attempt
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled: %w", err)
		}

		// Execute the function
		result, err := fn()

		// Success - return immediately
		if err == nil {
			if attempt > 0 {
				r.logger.Info("Operation succeeded after retry", map[string]interface{}{
					"tool":      toolName,
					"operation": operationName,
					"attempt":   attempt + 1,
				})
			}
			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		if !r.isRetryable(err, policy) {
			r.logger.Debug("Non-retryable error encountered", map[string]interface{}{
				"tool":      toolName,
				"operation": operationName,
				"error":     err.Error(),
			})
			return nil, err
		}

		// Don't retry if this was the last attempt
		if attempt == policy.MaxAttempts-1 {
			break
		}

		// Calculate next delay
		nextDelay := r.calculateDelay(delay, policy, err)

		r.logger.Info("Retrying operation", map[string]interface{}{
			"tool":      toolName,
			"operation": operationName,
			"attempt":   attempt + 1,
			"delay_ms":  nextDelay.Milliseconds(),
			"error":     err.Error(),
		})

		// Wait before next attempt
		select {
		case <-time.After(nextDelay):
			// Continue to next attempt
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
		}

		// Update delay for next iteration
		delay = time.Duration(float64(delay) * policy.Multiplier)
		if delay > policy.MaxDelay {
			delay = policy.MaxDelay
		}
	}

	return nil, fmt.Errorf("operation failed after %d attempts: %w", policy.MaxAttempts, lastErr)
}

// isRetryable determines if an error should trigger a retry
func (r *RetryHandler) isRetryable(err error, policy *tool.ToolRetryPolicy) bool {
	if err == nil {
		return false
	}

	// Check for retryable error types
	if retryErr, ok := err.(*RetryableError); ok {
		switch retryErr.ErrorType {
		case "rate_limit":
			return policy.RetryOnRateLimit
		case "timeout":
			return policy.RetryOnTimeout
		case "server_error":
			return true // Always retry 5xx errors
		case "network":
			return true // Always retry network errors
		}

		// Check status code
		if retryErr.StatusCode >= 500 && retryErr.StatusCode < 600 {
			return true // Retry all 5xx errors
		}
		if retryErr.StatusCode == 429 {
			return policy.RetryOnRateLimit // Rate limit
		}
		if retryErr.StatusCode == 408 {
			return policy.RetryOnTimeout // Request timeout
		}
	}

	// Check error message for specific patterns
	errMsg := strings.ToLower(err.Error())

	// Network-related errors
	networkErrors := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"no such host",
		"network is unreachable",
		"i/o timeout",
		"tls handshake timeout",
	}
	for _, pattern := range networkErrors {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	// Timeout errors
	if policy.RetryOnTimeout {
		timeoutErrors := []string{
			"timeout",
			"deadline exceeded",
			"context deadline exceeded",
		}
		for _, pattern := range timeoutErrors {
			if strings.Contains(errMsg, pattern) {
				return true
			}
		}
	}

	// Check custom retryable errors
	for _, retryableError := range policy.RetryableErrors {
		if strings.Contains(errMsg, strings.ToLower(retryableError)) {
			return true
		}
	}

	return false
}

// calculateDelay calculates the delay before the next retry attempt
func (r *RetryHandler) calculateDelay(baseDelay time.Duration, policy *tool.ToolRetryPolicy, err error) time.Duration {
	delay := baseDelay

	// Check if error specifies a retry-after duration
	if retryErr, ok := err.(*RetryableError); ok && retryErr.RetryAfter > 0 {
		delay = retryErr.RetryAfter
	}

	// Apply jitter to avoid thundering herd
	if policy.Jitter > 0 {
		jitter := float64(delay) * policy.Jitter
		// Random value between -jitter and +jitter
		jitterValue := (rand.Float64()*2 - 1) * jitter
		delay = time.Duration(float64(delay) + jitterValue)
	}

	// Ensure delay doesn't exceed max
	if delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}

	// Ensure delay is at least 1ms
	if delay < time.Millisecond {
		delay = time.Millisecond
	}

	return delay
}

// ClassifyHTTPError converts an HTTP response to a retryable error if appropriate
func (r *RetryHandler) ClassifyHTTPError(resp *http.Response, err error) error {
	if err != nil {
		// Network error
		return &RetryableError{
			ErrorType:   "network",
			OriginalErr: err,
		}
	}

	if resp == nil {
		return fmt.Errorf("no response received")
	}

	// Check status code
	switch {
	case resp.StatusCode >= 500:
		retryErr := &RetryableError{
			StatusCode:  resp.StatusCode,
			ErrorType:   "server_error",
			OriginalErr: fmt.Errorf("server error: %s", resp.Status),
		}

		// Check for Retry-After header
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			// Try to parse as seconds
			if seconds, err := time.ParseDuration(retryAfter + "s"); err == nil {
				retryErr.RetryAfter = seconds
			}
		}

		return retryErr

	case resp.StatusCode == 429:
		retryErr := &RetryableError{
			StatusCode:  resp.StatusCode,
			ErrorType:   "rate_limit",
			OriginalErr: fmt.Errorf("rate limit exceeded"),
		}

		// Check for Retry-After header
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if seconds, err := time.ParseDuration(retryAfter + "s"); err == nil {
				retryErr.RetryAfter = seconds
			}
		} else {
			// Default rate limit retry after 1 minute
			retryErr.RetryAfter = 60 * time.Second
		}

		return retryErr

	case resp.StatusCode == 408:
		return &RetryableError{
			StatusCode:  resp.StatusCode,
			ErrorType:   "timeout",
			OriginalErr: fmt.Errorf("request timeout"),
		}

	case resp.StatusCode >= 400:
		// Client errors are generally not retryable
		return fmt.Errorf("client error: %s", resp.Status)

	default:
		// Success or redirect - not an error
		return nil
	}
}

// ExponentialBackoff calculates exponential backoff with jitter
func (r *RetryHandler) ExponentialBackoff(attempt int, baseDelay time.Duration, maxDelay time.Duration, jitter float64) time.Duration {
	if attempt <= 0 {
		return baseDelay
	}

	// Calculate exponential delay
	delay := baseDelay * time.Duration(math.Pow(2, float64(attempt-1)))

	// Cap at max delay
	if delay > maxDelay {
		delay = maxDelay
	}

	// Apply jitter
	if jitter > 0 {
		jitterValue := float64(delay) * jitter * (rand.Float64()*2 - 1)
		delay = time.Duration(float64(delay) + jitterValue)
	}

	return delay
}
