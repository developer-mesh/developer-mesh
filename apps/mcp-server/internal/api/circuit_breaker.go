package api

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

// ToolCircuitBreaker implements circuit breaking for tool calls
type ToolCircuitBreaker struct {
	mu     sync.RWMutex
	logger observability.Logger

	// State tracking
	state           CircuitBreakerState
	failures        uint64
	successes       uint64
	lastFailureTime time.Time
	lastStateChange time.Time

	// Configuration
	maxFailures      uint64
	timeout          time.Duration
	halfOpenRequests uint64

	// Metrics
	totalRequests  uint64
	totalFailures  uint64
	totalSuccesses uint64
	tripsCount     uint64
}

// NewToolCircuitBreaker creates a new circuit breaker for tools
func NewToolCircuitBreaker(logger observability.Logger) *ToolCircuitBreaker {
	return &ToolCircuitBreaker{
		logger:           logger,
		state:            CircuitClosed,
		maxFailures:      5,
		timeout:          30 * time.Second,
		halfOpenRequests: 3,
	}
}

// Call executes a function with circuit breaker protection
func (cb *ToolCircuitBreaker) Call(ctx context.Context, toolName string, fn func() (interface{}, error)) (interface{}, error) {
	cb.mu.Lock()
	state := cb.state
	cb.mu.Unlock()

	atomic.AddUint64(&cb.totalRequests, 1)

	switch state {
	case CircuitOpen:
		return cb.handleOpenCircuit(toolName)
	case CircuitHalfOpen:
		return cb.handleHalfOpenCircuit(ctx, toolName, fn)
	default:
		return cb.handleClosedCircuit(ctx, toolName, fn)
	}
}

// handleOpenCircuit handles calls when circuit is open
func (cb *ToolCircuitBreaker) handleOpenCircuit(toolName string) (interface{}, error) {
	cb.mu.RLock()
	timeSinceOpen := time.Since(cb.lastStateChange)
	timeout := cb.timeout
	cb.mu.RUnlock()

	if timeSinceOpen > timeout {
		// Try half-open
		cb.mu.Lock()
		cb.state = CircuitHalfOpen
		cb.lastStateChange = time.Now()
		cb.successes = 0
		cb.mu.Unlock()

		cb.logger.Info("Circuit breaker entering half-open state", map[string]interface{}{
			"tool": toolName,
		})

		return nil, fmt.Errorf("circuit breaker is half-open, retrying tool %s", toolName)
	}

	return nil, fmt.Errorf("circuit breaker is open for tool %s", toolName)
}

// handleHalfOpenCircuit handles calls when circuit is half-open
func (cb *ToolCircuitBreaker) handleHalfOpenCircuit(ctx context.Context, toolName string, fn func() (interface{}, error)) (interface{}, error) {
	result, err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		// Failure in half-open, back to open
		cb.state = CircuitOpen
		cb.lastStateChange = time.Now()
		cb.lastFailureTime = time.Now()
		atomic.AddUint64(&cb.failures, 1)
		atomic.AddUint64(&cb.totalFailures, 1)

		cb.logger.Warn("Circuit breaker reopened due to failure", map[string]interface{}{
			"tool":  toolName,
			"error": err.Error(),
		})

		return nil, err
	}

	// Success in half-open
	atomic.AddUint64(&cb.successes, 1)
	atomic.AddUint64(&cb.totalSuccesses, 1)

	// Check if we can close the circuit
	if cb.successes >= cb.halfOpenRequests {
		cb.state = CircuitClosed
		cb.lastStateChange = time.Now()
		cb.failures = 0
		cb.successes = 0

		cb.logger.Info("Circuit breaker closed after successful recovery", map[string]interface{}{
			"tool": toolName,
		})
	}

	return result, nil
}

// handleClosedCircuit handles calls when circuit is closed
func (cb *ToolCircuitBreaker) handleClosedCircuit(ctx context.Context, toolName string, fn func() (interface{}, error)) (interface{}, error) {
	result, err := fn()

	if err != nil {
		cb.mu.Lock()
		atomic.AddUint64(&cb.failures, 1)
		atomic.AddUint64(&cb.totalFailures, 1)
		cb.lastFailureTime = time.Now()

		// Check if we should trip the circuit
		if cb.failures >= cb.maxFailures {
			cb.state = CircuitOpen
			cb.lastStateChange = time.Now()
			atomic.AddUint64(&cb.tripsCount, 1)

			cb.logger.Error("Circuit breaker tripped", map[string]interface{}{
				"tool":     toolName,
				"failures": cb.failures,
			})
		}
		cb.mu.Unlock()

		return nil, err
	}

	// Success - reset failure count
	cb.mu.Lock()
	cb.failures = 0
	atomic.AddUint64(&cb.totalSuccesses, 1)
	cb.mu.Unlock()

	return result, nil
}

// GetState returns the current state of the circuit breaker
func (cb *ToolCircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetMetrics returns circuit breaker metrics
func (cb *ToolCircuitBreaker) GetMetrics() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	stateStr := "unknown"
	switch cb.state {
	case CircuitClosed:
		stateStr = "closed"
	case CircuitOpen:
		stateStr = "open"
	case CircuitHalfOpen:
		stateStr = "half_open"
	}

	return map[string]interface{}{
		"state":            stateStr,
		"total_requests":   cb.totalRequests,
		"total_failures":   cb.totalFailures,
		"total_successes":  cb.totalSuccesses,
		"trips_count":      cb.tripsCount,
		"current_failures": cb.failures,
	}
}

// Reset resets the circuit breaker
func (cb *ToolCircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitClosed
	cb.failures = 0
	cb.successes = 0
	cb.lastStateChange = time.Now()

	cb.logger.Info("Circuit breaker reset", nil)
}

// ToolCircuitBreakerManager manages circuit breakers for multiple tools
type ToolCircuitBreakerManager struct {
	mu       sync.RWMutex
	breakers map[string]*ToolCircuitBreaker
	logger   observability.Logger
}

// NewToolCircuitBreakerManager creates a new circuit breaker manager
func NewToolCircuitBreakerManager(logger observability.Logger) *ToolCircuitBreakerManager {
	return &ToolCircuitBreakerManager{
		breakers: make(map[string]*ToolCircuitBreaker),
		logger:   logger,
	}
}

// GetBreaker gets or creates a circuit breaker for a tool
func (m *ToolCircuitBreakerManager) GetBreaker(toolName string) *ToolCircuitBreaker {
	m.mu.RLock()
	breaker, exists := m.breakers[toolName]
	m.mu.RUnlock()

	if exists {
		return breaker
	}

	// Create new breaker
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check after acquiring write lock
	if breaker, exists = m.breakers[toolName]; exists {
		return breaker
	}

	breaker = NewToolCircuitBreaker(m.logger)
	m.breakers[toolName] = breaker
	return breaker
}

// GetAllMetrics returns metrics for all circuit breakers
func (m *ToolCircuitBreakerManager) GetAllMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make(map[string]interface{})
	for toolName, breaker := range m.breakers {
		metrics[toolName] = breaker.GetMetrics()
	}
	return metrics
}

// ResetAll resets all circuit breakers
func (m *ToolCircuitBreakerManager) ResetAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, breaker := range m.breakers {
		breaker.Reset()
	}
}
