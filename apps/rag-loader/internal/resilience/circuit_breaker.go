// Package resilience provides circuit breaker and retry logic for external dependencies
package resilience

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

var (
	// ErrCircuitOpen is returned when the circuit breaker is open
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// ErrMaxRetriesExceeded is returned when max retries are exceeded
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	// StateClosed means the circuit is closed and requests flow normally
	StateClosed CircuitBreakerState = iota

	// StateOpen means the circuit is open and requests are blocked
	StateOpen

	// StateHalfOpen means the circuit is testing if the service recovered
	StateHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
	// MaxFailures is the number of failures before opening
	MaxFailures int

	// ResetTimeout is how long to wait before attempting to close
	ResetTimeout time.Duration

	// HalfOpenMaxRequests is max requests allowed in half-open state
	HalfOpenMaxRequests int

	// FailureThreshold is the failure rate threshold (0-1)
	FailureThreshold float64

	// MinimumRequestCount is the minimum requests before checking threshold
	MinimumRequestCount int
}

// DefaultCircuitBreakerConfig returns sensible defaults
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		MaxFailures:         5,
		ResetTimeout:        60 * time.Second,
		HalfOpenMaxRequests: 3,
		FailureThreshold:    0.5,
		MinimumRequestCount: 10,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config      CircuitBreakerConfig
	state       CircuitBreakerState
	failures    int
	successes   int
	requests    int
	lastAttempt time.Time
	logger      observability.Logger

	mu sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig, logger observability.Logger) *CircuitBreaker {
	if config.MaxFailures <= 0 {
		config.MaxFailures = 5
	}
	if config.ResetTimeout <= 0 {
		config.ResetTimeout = 60 * time.Second
	}
	if config.HalfOpenMaxRequests <= 0 {
		config.HalfOpenMaxRequests = 3
	}

	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
		logger: logger.WithPrefix("circuit-breaker"),
	}
}

// Execute runs the given function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	// Check if circuit is open
	if !cb.allow() {
		return ErrCircuitOpen
	}

	// Execute function
	err := fn()

	// Record result
	cb.recordResult(err == nil)

	return err
}

// allow checks if the request should be allowed
func (cb *CircuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true

	case StateOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastAttempt) > cb.config.ResetTimeout {
			cb.setState(StateHalfOpen)
			cb.logger.Info("Circuit breaker transitioning to half-open", nil)
			return true
		}
		return false

	case StateHalfOpen:
		// Allow limited requests in half-open state
		if cb.requests < cb.config.HalfOpenMaxRequests {
			return true
		}
		return false

	default:
		return false
	}
}

// recordResult records the result of an execution
func (cb *CircuitBreaker) recordResult(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.requests++
	cb.lastAttempt = time.Now()

	if success {
		cb.successes++
		cb.onSuccess()
	} else {
		cb.failures++
		cb.onFailure()
	}
}

// onSuccess handles a successful execution
func (cb *CircuitBreaker) onSuccess() {
	switch cb.state {
	case StateHalfOpen:
		// If we got enough successes in half-open, close the circuit
		if cb.successes >= cb.config.HalfOpenMaxRequests {
			cb.setState(StateClosed)
			cb.reset()
			cb.logger.Info("Circuit breaker closed after successful recovery", nil)
		}

	case StateClosed:
		// Track success for failure rate calculation
		if cb.requests >= cb.config.MinimumRequestCount {
			failureRate := float64(cb.failures) / float64(cb.requests)
			if failureRate < cb.config.FailureThreshold {
				// Reset counters if we're well below threshold
				cb.reset()
			}
		}
	}
}

// onFailure handles a failed execution
func (cb *CircuitBreaker) onFailure() {
	switch cb.state {
	case StateHalfOpen:
		// Immediate open on failure in half-open
		cb.setState(StateOpen)
		cb.logger.Warn("Circuit breaker re-opened after failure", map[string]interface{}{
			"failures": cb.failures,
		})

	case StateClosed:
		// Check if we should open
		if cb.failures >= cb.config.MaxFailures {
			cb.setState(StateOpen)
			cb.logger.Warn("Circuit breaker opened", map[string]interface{}{
				"failures": cb.failures,
			})
		} else if cb.requests >= cb.config.MinimumRequestCount {
			// Check failure rate
			failureRate := float64(cb.failures) / float64(cb.requests)
			if failureRate >= cb.config.FailureThreshold {
				cb.setState(StateOpen)
				cb.logger.Warn("Circuit breaker opened due to failure rate", map[string]interface{}{
					"failure_rate": failureRate,
					"threshold":    cb.config.FailureThreshold,
				})
			}
		}
	}
}

// setState changes the circuit breaker state
func (cb *CircuitBreaker) setState(state CircuitBreakerState) {
	cb.state = state
}

// reset resets the circuit breaker counters
func (cb *CircuitBreaker) reset() {
	cb.failures = 0
	cb.successes = 0
	cb.requests = 0
}

// State returns the current circuit breaker state
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats returns circuit breaker statistics
func (cb *CircuitBreaker) Stats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	failureRate := 0.0
	if cb.requests > 0 {
		failureRate = float64(cb.failures) / float64(cb.requests)
	}

	return map[string]interface{}{
		"state":        cb.state.String(),
		"failures":     cb.failures,
		"successes":    cb.successes,
		"requests":     cb.requests,
		"failure_rate": failureRate,
		"last_attempt": cb.lastAttempt,
	}
}

// RetryConfig configures retry behavior
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int

	// InitialDelay is the initial delay before first retry
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// Multiplier is the backoff multiplier
	Multiplier float64

	// RetryableErrors are errors that should trigger retries
	RetryableErrors []error
}

// DefaultRetryConfig returns sensible defaults
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

// RetryWithBackoff executes a function with exponential backoff retry logic
func RetryWithBackoff(ctx context.Context, config RetryConfig, logger observability.Logger, fn func() error) error {
	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Execute function
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !isRetryable(err, config.RetryableErrors) {
			logger.Debug("Error not retryable", map[string]interface{}{
				"error": err.Error(),
			})
			return fmt.Errorf("non-retryable error: %w", err)
		}

		// Last attempt, don't delay
		if attempt == config.MaxRetries {
			break
		}

		// Log retry
		logger.Warn("Retrying after error", map[string]interface{}{
			"attempt":      attempt + 1,
			"max_attempts": config.MaxRetries,
			"delay":        delay,
			"error":        err.Error(),
		})

		// Wait with context cancellation
		select {
		case <-time.After(delay):
			// Calculate next delay
			delay = time.Duration(float64(delay) * config.Multiplier)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("%w: %v", ErrMaxRetriesExceeded, lastErr)
}

// isRetryable checks if an error should trigger a retry
func isRetryable(err error, retryableErrors []error) bool {
	if len(retryableErrors) == 0 {
		// If no specific errors configured, retry all errors
		return true
	}

	for _, retryableErr := range retryableErrors {
		if errors.Is(err, retryableErr) {
			return true
		}
	}

	return false
}
