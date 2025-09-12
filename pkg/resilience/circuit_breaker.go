package resilience

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/pkg/errors"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

// Circuit breaker states
const (
	CircuitBreakerClosed   CircuitBreakerState = iota // Normal operation, requests allowed
	CircuitBreakerOpen                                // Tripped, requests blocked
	CircuitBreakerHalfOpen                            // Testing if service is healthy
)

// Circuit breaker errors
var (
	ErrCircuitBreakerOpen    = errors.New("circuit breaker is open")
	ErrCircuitBreakerTimeout = errors.New("circuit breaker timeout")
	ErrMaxRequestsExceeded   = errors.New("max requests exceeded in half-open state")
)

// String returns the string representation of the circuit breaker state
func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerClosed:
		return "closed"
	case CircuitBreakerOpen:
		return "open"
	case CircuitBreakerHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig holds configuration for a circuit breaker
type CircuitBreakerConfig struct {
	FailureThreshold    int           // Number of failures before tripping
	FailureRatio        float64       // Failure ratio threshold (0.0-1.0)
	ResetTimeout        time.Duration // Time before attempting retry
	SuccessThreshold    int           // Number of successes needed to close circuit
	TimeoutThreshold    time.Duration // Request timeout threshold
	MaxRequestsHalfOpen int           // Max requests in half-open state
	MinimumRequestCount int           // Minimum requests before evaluating failure ratio
}

// CircuitBreaker implements the circuit breaker pattern with production features
type CircuitBreaker struct {
	name            string
	config          CircuitBreakerConfig
	state           atomic.Value // CircuitBreakerState
	counts          atomic.Value // *Counts
	lastFailureTime atomic.Value // time.Time
	lastStateChange atomic.Value // time.Time

	// Concurrent request tracking for half-open state
	halfOpenRequests atomic.Int32

	// Synchronization
	mutex sync.RWMutex

	// Observability
	logger  observability.Logger
	metrics observability.MetricsClient
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(name string, config CircuitBreakerConfig, logger observability.Logger, metrics observability.MetricsClient) *CircuitBreaker {
	// Apply defaults if not set
	if config.FailureThreshold == 0 {
		config.FailureThreshold = 5
	}
	if config.FailureRatio == 0 {
		config.FailureRatio = 0.6
	}
	if config.ResetTimeout == 0 {
		config.ResetTimeout = 30 * time.Second
	}
	if config.SuccessThreshold == 0 {
		config.SuccessThreshold = 2
	}
	if config.TimeoutThreshold == 0 {
		config.TimeoutThreshold = 5 * time.Second
	}
	if config.MaxRequestsHalfOpen == 0 {
		config.MaxRequestsHalfOpen = 5
	}
	if config.MinimumRequestCount == 0 {
		config.MinimumRequestCount = 10
	}

	cb := &CircuitBreaker{
		name:    name,
		config:  config,
		logger:  logger,
		metrics: metrics,
	}

	// Initialize atomic values
	cb.state.Store(CircuitBreakerClosed)
	initialCounts := NewCounts()
	cb.counts.Store(&initialCounts)
	cb.lastFailureTime.Store(time.Time{})
	cb.lastStateChange.Store(time.Now())

	// Record initial state metric
	cb.recordStateMetric(CircuitBreakerClosed)

	return cb
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	start := time.Now()

	// Check if we can execute based on current state
	if err := cb.canExecute(); err != nil {
		cb.recordFailure()
		cb.recordMetrics("rejected", false, time.Since(start))
		cb.logger.Error("Circuit breaker execution rejected", map[string]interface{}{
			"error": err.Error(),
			"state": cb.getState().String(),
			"name":  cb.name,
		})
		return nil, errors.Wrap(err, "circuit breaker execution failed")
	}

	// For half-open state, track concurrent requests
	if cb.getState() == CircuitBreakerHalfOpen {
		cb.halfOpenRequests.Add(1)
		defer cb.halfOpenRequests.Add(-1)
	}

	// Create a channel for the result
	type result struct {
		value interface{}
		err   error
	}
	resultChan := make(chan result, 1)

	// Execute function with timeout
	go func() {
		value, err := fn()
		resultChan <- result{value: value, err: err}
	}()

	// Wait for result or timeout
	select {
	case <-ctx.Done():
		cb.recordFailure()
		cb.recordMetrics("timeout", false, time.Since(start))
		return nil, errors.Wrap(ctx.Err(), "context cancelled")

	case <-time.After(cb.config.TimeoutThreshold):
		cb.recordFailure()
		cb.recordMetrics("timeout", false, time.Since(start))
		return nil, ErrCircuitBreakerTimeout

	case res := <-resultChan:
		if res.err != nil {
			cb.recordFailure()
			cb.recordMetrics("failure", false, time.Since(start))
			cb.logger.Error("Circuit breaker execution failed", map[string]interface{}{
				"error": res.err.Error(),
				"state": cb.getState().String(),
				"name":  cb.name,
			})
			return nil, errors.Wrap(res.err, "circuit breaker execution failed")
		}

		cb.recordSuccess()
		cb.recordMetrics("success", true, time.Since(start))
		return res.value, nil
	}
}

// canExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) canExecute() error {
	state := cb.getState()

	switch state {
	case CircuitBreakerClosed:
		return nil

	case CircuitBreakerOpen:
		// Check if we should transition to half-open
		lastFailure := cb.lastFailureTime.Load().(time.Time)
		if time.Since(lastFailure) > cb.config.ResetTimeout {
			cb.transitionTo(CircuitBreakerHalfOpen)
			return nil
		}
		return ErrCircuitBreakerOpen

	case CircuitBreakerHalfOpen:
		// Check if we've exceeded max requests in half-open state
		if int(cb.halfOpenRequests.Load()) >= cb.config.MaxRequestsHalfOpen {
			return ErrMaxRequestsExceeded
		}
		return nil

	default:
		return fmt.Errorf("unknown circuit breaker state: %v", state)
	}
}

// recordSuccess records a successful execution
func (cb *CircuitBreaker) recordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	counts := cb.getCounts()
	counts.RecordSuccess()
	cb.counts.Store(counts)

	state := cb.getState()

	// Check if we should transition states
	switch state {
	case CircuitBreakerHalfOpen:
		if counts.ConsecutiveSuccesses >= cb.config.SuccessThreshold {
			cb.transitionTo(CircuitBreakerClosed)
		}
	}
}

// recordFailure records a failed execution
func (cb *CircuitBreaker) recordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	counts := cb.getCounts()
	counts.RecordFailure()
	cb.counts.Store(counts)
	cb.lastFailureTime.Store(time.Now())

	state := cb.getState()

	// Check if we should transition states
	switch state {
	case CircuitBreakerClosed:
		// Check failure threshold
		if counts.ConsecutiveFailures >= cb.config.FailureThreshold {
			cb.transitionTo(CircuitBreakerOpen)
		} else if counts.Requests >= cb.config.MinimumRequestCount {
			// Check failure ratio
			failureRatio := float64(counts.Failures) / float64(counts.Requests)
			if failureRatio >= cb.config.FailureRatio {
				cb.transitionTo(CircuitBreakerOpen)
			}
		}

	case CircuitBreakerHalfOpen:
		// Any failure in half-open state trips the breaker
		cb.transitionTo(CircuitBreakerOpen)
	}
}

// transitionTo transitions the circuit breaker to a new state
func (cb *CircuitBreaker) transitionTo(newState CircuitBreakerState) {
	oldState := cb.getState()
	if oldState == newState {
		return
	}

	cb.state.Store(newState)
	cb.lastStateChange.Store(time.Now())

	// Reset counts when transitioning to half-open
	if newState == CircuitBreakerHalfOpen {
		newCounts := NewCounts()
		cb.counts.Store(&newCounts)
		cb.halfOpenRequests.Store(0)
	}

	// Log state change
	cb.logger.Info("Circuit breaker state changed", map[string]interface{}{
		"name": cb.name,
		"from": oldState.String(),
		"to":   newState.String(),
	})

	// Record metrics
	cb.recordStateChangeMetric(oldState, newState)
	cb.recordStateMetric(newState)
}

// Helper methods

func (cb *CircuitBreaker) getState() CircuitBreakerState {
	return cb.state.Load().(CircuitBreakerState)
}

func (cb *CircuitBreaker) getCounts() *Counts {
	counts := cb.counts.Load().(*Counts)
	// Return a copy to avoid race conditions
	copy := NewCounts()
	copy.Requests = counts.Requests
	copy.Successes = counts.Successes
	copy.Failures = counts.Failures
	copy.ConsecutiveSuccesses = counts.ConsecutiveSuccesses
	copy.ConsecutiveFailures = counts.ConsecutiveFailures
	copy.TotalSuccesses = counts.TotalSuccesses
	copy.TotalFailures = counts.TotalFailures
	copy.Timeout = counts.Timeout
	copy.ShortCircuited = counts.ShortCircuited
	copy.Rejected = counts.Rejected
	copy.LastSuccess = counts.LastSuccess
	copy.LastFailure = counts.LastFailure
	copy.LastTimeout = counts.LastTimeout
	return &copy
}

// Metrics recording methods

func (cb *CircuitBreaker) recordMetrics(result string, success bool, duration time.Duration) {
	if cb.metrics == nil {
		return
	}

	labels := map[string]string{
		"name":   cb.name,
		"state":  cb.getState().String(),
		"status": result,
	}

	// Record request count
	cb.metrics.IncrementCounterWithLabels("circuit_breaker_requests_total", 1, labels)

	// Record duration
	cb.metrics.RecordHistogram("circuit_breaker_request_duration_seconds", duration.Seconds(), labels)

	// Record success/failure
	if success {
		cb.metrics.IncrementCounterWithLabels("circuit_breaker_successes_total", 1, labels)
	} else {
		cb.metrics.IncrementCounterWithLabels("circuit_breaker_failures_total", 1, labels)
	}
}

func (cb *CircuitBreaker) recordStateChangeMetric(from, to CircuitBreakerState) {
	if cb.metrics == nil {
		return
	}

	labels := map[string]string{
		"name": cb.name,
		"from": from.String(),
		"to":   to.String(),
	}
	cb.metrics.IncrementCounterWithLabels("circuit_breaker_state_changes_total", 1, labels)
}

func (cb *CircuitBreaker) recordStateMetric(state CircuitBreakerState) {
	// Record current state as a gauge (0=closed, 1=open, 2=half-open)
	if cb.metrics != nil {
		labels := map[string]string{
			"name": cb.name,
		}
		cb.metrics.RecordGauge("circuit_breaker_current_state", float64(state), labels)
	}
}

// GetMetrics returns current circuit breaker metrics
func (cb *CircuitBreaker) GetMetrics() map[string]interface{} {
	counts := cb.getCounts()
	state := cb.getState()
	lastStateChange := cb.lastStateChange.Load().(time.Time)
	lastFailure := cb.lastFailureTime.Load().(time.Time)

	return map[string]interface{}{
		"name":                    cb.name,
		"state":                   state.String(),
		"requests":                counts.Requests,
		"successes":               counts.Successes,
		"failures":                counts.Failures,
		"total_successes":         counts.TotalSuccesses,
		"total_failures":          counts.TotalFailures,
		"consecutive_successes":   counts.ConsecutiveSuccesses,
		"consecutive_failures":    counts.ConsecutiveFailures,
		"timeout":                 counts.Timeout,
		"rejected":                counts.Rejected,
		"short_circuited":         counts.ShortCircuited,
		"last_state_change":       lastStateChange,
		"last_failure":            lastFailure,
		"time_since_last_failure": time.Since(lastFailure).Seconds(),
	}
}

// Reset manually resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.transitionTo(CircuitBreakerClosed)
	resetCounts := NewCounts()
	cb.counts.Store(&resetCounts)
	cb.halfOpenRequests.Store(0)

	cb.logger.Info("Circuit breaker manually reset", map[string]interface{}{
		"name": cb.name,
	})
}

// CircuitBreakerManager manages multiple circuit breakers
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	mutex    sync.RWMutex
	logger   observability.Logger
	metrics  observability.MetricsClient
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager(logger observability.Logger, metrics observability.MetricsClient, defaultConfigs map[string]CircuitBreakerConfig) *CircuitBreakerManager {
	manager := &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
		logger:   logger,
		metrics:  metrics,
	}

	// Create circuit breakers from default configs
	for name, config := range defaultConfigs {
		manager.breakers[name] = NewCircuitBreaker(name, config, logger, metrics)
	}

	return manager
}

// GetCircuitBreaker gets a circuit breaker by name, creating it if it doesn't exist
func (m *CircuitBreakerManager) GetCircuitBreaker(name string) *CircuitBreaker {
	m.mutex.RLock()
	breaker, exists := m.breakers[name]
	m.mutex.RUnlock()

	if exists {
		return breaker
	}

	// Use a default configuration if the circuit breaker doesn't exist
	defaultConfig := CircuitBreakerConfig{
		FailureThreshold:    5,
		FailureRatio:        0.6,
		ResetTimeout:        30 * time.Second,
		SuccessThreshold:    2,
		TimeoutThreshold:    5 * time.Second,
		MaxRequestsHalfOpen: 5,
		MinimumRequestCount: 10,
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check again in case it was created while we were waiting for the lock
	breaker, exists = m.breakers[name]
	if exists {
		return breaker
	}

	// Create a new circuit breaker
	breaker = NewCircuitBreaker(name, defaultConfig, m.logger, m.metrics)
	m.breakers[name] = breaker

	return breaker
}

// Execute executes a function using the named circuit breaker
func (m *CircuitBreakerManager) Execute(ctx context.Context, name string, fn func() (interface{}, error)) (interface{}, error) {
	breaker := m.GetCircuitBreaker(name)
	return breaker.Execute(ctx, fn)
}

// GetAllMetrics returns metrics for all circuit breakers
func (m *CircuitBreakerManager) GetAllMetrics() map[string]map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	metrics := make(map[string]map[string]interface{})
	for name, breaker := range m.breakers {
		metrics[name] = breaker.GetMetrics()
	}

	return metrics
}

// ResetAll resets all circuit breakers to closed state
func (m *CircuitBreakerManager) ResetAll() {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, breaker := range m.breakers {
		breaker.Reset()
	}
}
