package redis

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// ErrCircuitOpen is returned when the circuit breaker is open
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	// StateClosed means the circuit is functioning normally
	StateClosed CircuitState = iota
	// StateOpen means the circuit is open due to failures
	StateOpen
	// StateHalfOpen means the circuit is testing if the service has recovered
	StateHalfOpen
)

// CircuitBreakerConfig contains configuration for circuit breaker
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of failures before opening the circuit
	FailureThreshold int
	// SuccessThreshold is the number of successes in half-open state before closing
	SuccessThreshold int
	// Timeout is how long to wait before attempting to close the circuit
	Timeout time.Duration
	// MaxTimeout is the maximum timeout after repeated failures
	MaxTimeout time.Duration
	// TimeoutMultiplier increases timeout after each failure
	TimeoutMultiplier float64
}

// DefaultCircuitBreakerConfig returns default circuit breaker configuration
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		FailureThreshold:  5,
		SuccessThreshold:  2,
		Timeout:           30 * time.Second,
		MaxTimeout:        5 * time.Minute,
		TimeoutMultiplier: 2.0,
	}
}

// CircuitBreaker implements the circuit breaker pattern for Redis operations
type CircuitBreaker struct {
	config *CircuitBreakerConfig
	logger observability.Logger

	mu              sync.RWMutex
	state           CircuitState
	failures        int
	successes       int
	lastFailureTime time.Time
	currentTimeout  time.Duration
	generation      uint64 // Prevents race conditions during state transitions
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig, logger observability.Logger) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	return &CircuitBreaker{
		config:         config,
		logger:         logger,
		state:          StateClosed,
		currentTimeout: config.Timeout,
	}
}

// Execute runs a function through the circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, operation func() error) error {
	// Check context before execution
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	generation, err := cb.beforeRequest()
	if err != nil {
		return err
	}

	err = operation()
	cb.afterRequest(generation, err)

	return err
}

// ExecuteWithResult runs a function that returns a value through the circuit breaker
func (cb *CircuitBreaker) ExecuteWithResult(ctx context.Context, operation func() (interface{}, error)) (interface{}, error) {
	// Check context before execution
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	generation, err := cb.beforeRequest()
	if err != nil {
		return nil, err
	}

	result, err := operation()
	cb.afterRequest(generation, err)

	return result, err
}

// beforeRequest checks if the circuit allows the request
func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	generation := cb.generation

	switch cb.state {
	case StateClosed:
		// Normal operation
		return generation, nil

	case StateOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailureTime) > cb.currentTimeout {
			cb.state = StateHalfOpen
			cb.successes = 0
			cb.generation++
			cb.logger.Info("Circuit breaker transitioning to half-open", map[string]interface{}{
				"timeout": cb.currentTimeout,
			})
			return cb.generation, nil
		}
		return generation, fmt.Errorf("circuit breaker is open")

	case StateHalfOpen:
		// Allow request to test if service has recovered
		return generation, nil

	default:
		return generation, fmt.Errorf("unknown circuit breaker state: %v", cb.state)
	}
}

// afterRequest updates the circuit breaker state after a request
func (cb *CircuitBreaker) afterRequest(generation uint64, err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Ignore if generation has changed (another goroutine changed state)
	if generation != cb.generation {
		return
	}

	if err == nil {
		cb.onSuccess()
	} else {
		cb.onFailure()
	}
}

// onSuccess handles a successful request
func (cb *CircuitBreaker) onSuccess() {
	switch cb.state {
	case StateClosed:
		// Reset failure count on success
		cb.failures = 0

	case StateHalfOpen:
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			// Transition to closed
			cb.state = StateClosed
			cb.failures = 0
			cb.successes = 0
			cb.currentTimeout = cb.config.Timeout // Reset timeout
			cb.generation++

			cb.logger.Info("Circuit breaker closed after recovery", map[string]interface{}{
				"successes": cb.successes,
			})
		}
	}
}

// onFailure handles a failed request
func (cb *CircuitBreaker) onFailure() {
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		cb.failures++
		if cb.failures >= cb.config.FailureThreshold {
			// Transition to open
			cb.state = StateOpen
			cb.generation++

			cb.logger.Error("Circuit breaker opened due to failures", map[string]interface{}{
				"failures": cb.failures,
				"timeout":  cb.currentTimeout,
			})
		}

	case StateHalfOpen:
		// Failure in half-open state, back to open
		cb.state = StateOpen
		cb.successes = 0
		cb.generation++

		// Increase timeout with backoff
		cb.currentTimeout = time.Duration(float64(cb.currentTimeout) * cb.config.TimeoutMultiplier)
		if cb.currentTimeout > cb.config.MaxTimeout {
			cb.currentTimeout = cb.config.MaxTimeout
		}

		cb.logger.Error("Circuit breaker reopened after half-open test failed", map[string]interface{}{
			"new_timeout": cb.currentTimeout,
		})
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns statistics about the circuit breaker
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	stats := map[string]interface{}{
		"state":           cb.state.String(),
		"failures":        cb.failures,
		"successes":       cb.successes,
		"current_timeout": cb.currentTimeout,
		"generation":      cb.generation,
	}

	if !cb.lastFailureTime.IsZero() {
		stats["last_failure"] = cb.lastFailureTime
		stats["time_since_failure"] = time.Since(cb.lastFailureTime)
	}

	return stats
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
	cb.currentTimeout = cb.config.Timeout
	cb.generation++

	cb.logger.Info("Circuit breaker manually reset", nil)
}

// String returns the string representation of a circuit state
func (cs CircuitState) String() string {
	switch cs {
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

// CircuitBreakerManager manages multiple circuit breakers for different tools/endpoints
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	config   *CircuitBreakerConfig
	logger   observability.Logger
	mu       sync.RWMutex
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager(config *CircuitBreakerConfig, logger observability.Logger) *CircuitBreakerManager {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
		config:   config,
		logger:   logger,
	}
}

// GetBreaker returns a circuit breaker for a specific key (e.g., tool ID)
func (m *CircuitBreakerManager) GetBreaker(key string) *CircuitBreaker {
	m.mu.RLock()
	breaker, exists := m.breakers[key]
	m.mu.RUnlock()

	if exists {
		return breaker
	}

	// Create new breaker
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if breaker, exists := m.breakers[key]; exists {
		return breaker
	}

	breaker = NewCircuitBreaker(m.config, m.logger)
	m.breakers[key] = breaker

	return breaker
}

// GetAllStats returns statistics for all circuit breakers
func (m *CircuitBreakerManager) GetAllStats() map[string]map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]map[string]interface{})
	for key, breaker := range m.breakers {
		stats[key] = breaker.GetStats()
	}

	return stats
}

// ResetAll resets all circuit breakers
func (m *CircuitBreakerManager) ResetAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, breaker := range m.breakers {
		breaker.Reset()
	}
}

// ResetBreaker resets a specific circuit breaker
func (m *CircuitBreakerManager) ResetBreaker(key string) error {
	m.mu.RLock()
	breaker, exists := m.breakers[key]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("circuit breaker not found: %s", key)
	}

	breaker.Reset()
	return nil
}
