package clients

import (
	"context"
	"fmt"
	"sync"
	"time"

	pkgerrors "github.com/developer-mesh/developer-mesh/pkg/errors"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// RecoveryAction represents an action to take for error recovery
type RecoveryAction int

const (
	// RecoveryActionNone indicates no recovery action needed
	RecoveryActionNone RecoveryAction = iota
	// RecoveryActionRetry indicates the operation should be retried
	RecoveryActionRetry
	// RecoveryActionCircuitBreak indicates circuit breaker should open
	RecoveryActionCircuitBreak
	// RecoveryActionFallback indicates fallback should be used
	RecoveryActionFallback
	// RecoveryActionRefreshCache indicates cache should be refreshed
	RecoveryActionRefreshCache
	// RecoveryActionReconnect indicates connection should be re-established
	RecoveryActionReconnect
	// RecoveryActionEscalate indicates error should be escalated
	RecoveryActionEscalate
)

// RecoveryStrategy defines how to recover from an error
type RecoveryStrategy struct {
	Action            RecoveryAction
	Delay             time.Duration
	MaxAttempts       int
	BackoffMultiplier float64
	FallbackEnabled   bool
	NotifyOps         bool
}

// ErrorRecoveryManager manages error recovery procedures
type ErrorRecoveryManager struct {
	mu                sync.RWMutex
	logger            observability.Logger
	strategies        map[string]*RecoveryStrategy
	recoveryHistory   []RecoveryEvent
	maxHistorySize    int
	circuitBreakers   map[string]*CircuitBreaker
	fallbackRegistry  *FallbackRegistry
	reconnectHandlers map[string]func() error
}

// RecoveryEvent represents a recovery attempt
type RecoveryEvent struct {
	Timestamp     time.Time
	ErrorCode     string
	ErrorClass    pkgerrors.ErrorClass
	Action        RecoveryAction
	Success       bool
	Duration      time.Duration
	CorrelationID string
}

// NewErrorRecoveryManager creates a new error recovery manager
func NewErrorRecoveryManager(logger observability.Logger) *ErrorRecoveryManager {
	manager := &ErrorRecoveryManager{
		logger:            logger,
		strategies:        make(map[string]*RecoveryStrategy),
		recoveryHistory:   make([]RecoveryEvent, 0, 100),
		maxHistorySize:    100,
		circuitBreakers:   make(map[string]*CircuitBreaker),
		reconnectHandlers: make(map[string]func() error),
	}

	// Initialize default recovery strategies
	manager.initializeDefaultStrategies()

	return manager
}

// initializeDefaultStrategies sets up default recovery strategies for common errors
func (m *ErrorRecoveryManager) initializeDefaultStrategies() {
	// Transient errors - retry with exponential backoff
	m.strategies["REST_REQUEST_ERROR"] = &RecoveryStrategy{
		Action:            RecoveryActionRetry,
		Delay:             1 * time.Second,
		MaxAttempts:       3,
		BackoffMultiplier: 2.0,
		FallbackEnabled:   true,
	}

	// Circuit breaker errors - wait and use fallback
	m.strategies["REST_CIRCUIT_OPEN"] = &RecoveryStrategy{
		Action:          RecoveryActionFallback,
		Delay:           30 * time.Second,
		MaxAttempts:     1,
		FallbackEnabled: true,
		NotifyOps:       true,
	}

	// Rate limiting - wait with retry-after
	m.strategies["REST_HTTP_429"] = &RecoveryStrategy{
		Action:            RecoveryActionRetry,
		Delay:             5 * time.Second,
		MaxAttempts:       5,
		BackoffMultiplier: 1.0, // Linear for rate limiting
		FallbackEnabled:   false,
	}

	// Timeout errors - retry with longer timeout
	m.strategies["REST_TIMEOUT"] = &RecoveryStrategy{
		Action:            RecoveryActionRetry,
		Delay:             2 * time.Second,
		MaxAttempts:       2,
		BackoffMultiplier: 1.5,
		FallbackEnabled:   true,
	}

	// Connection errors - reconnect and retry
	m.strategies["REST_CONNECTION_ERROR"] = &RecoveryStrategy{
		Action:            RecoveryActionReconnect,
		Delay:             3 * time.Second,
		MaxAttempts:       3,
		BackoffMultiplier: 2.0,
		FallbackEnabled:   true,
	}

	// Server errors (5xx) - circuit break after multiple failures
	m.strategies["REST_HTTP_500"] = &RecoveryStrategy{
		Action:            RecoveryActionCircuitBreak,
		Delay:             5 * time.Second,
		MaxAttempts:       3,
		BackoffMultiplier: 2.0,
		FallbackEnabled:   true,
		NotifyOps:         true,
	}
}

// HandleError analyzes an error and determines the recovery action
func (m *ErrorRecoveryManager) HandleError(ctx context.Context, err error) (*RecoveryStrategy, error) {
	if err == nil {
		return nil, nil
	}

	// Extract error details
	var errorCode string
	var errorClass pkgerrors.ErrorClass
	var correlationID string

	if classifiedErr, ok := err.(*pkgerrors.ClassifiedError); ok {
		errorCode = classifiedErr.Code
		errorClass = classifiedErr.Class
		correlationID = classifiedErr.CorrelationID
	} else {
		// Unclassified error
		errorCode = "UNKNOWN"
		errorClass = pkgerrors.ClassUnknown
	}

	// Get recovery strategy
	m.mu.RLock()
	strategy, exists := m.strategies[errorCode]
	m.mu.RUnlock()

	if !exists {
		// Use default strategy based on error class
		strategy = m.getDefaultStrategyForClass(errorClass)
	}

	// Log recovery attempt
	m.logRecoveryAttempt(errorCode, errorClass, strategy.Action, correlationID)

	return strategy, nil
}

// ExecuteRecovery executes the recovery strategy for an error
func (m *ErrorRecoveryManager) ExecuteRecovery(ctx context.Context, strategy *RecoveryStrategy, operation func() error) error {
	if strategy == nil {
		return fmt.Errorf("no recovery strategy provided")
	}

	startTime := time.Now()
	var lastErr error

	for attempt := 0; attempt < strategy.MaxAttempts; attempt++ {
		// Apply recovery action
		switch strategy.Action {
		case RecoveryActionRetry:
			// Calculate delay with backoff
			delay := strategy.Delay
			for i := 0; i < attempt; i++ {
				delay = time.Duration(float64(delay) * strategy.BackoffMultiplier)
			}

			m.logger.Info("Retrying operation", map[string]interface{}{
				"attempt": attempt + 1,
				"delay":   delay.String(),
			})

			time.Sleep(delay)

		case RecoveryActionReconnect:
			// Execute reconnection handlers
			m.executeReconnectHandlers()

		case RecoveryActionCircuitBreak:
			// Open circuit breaker for the service
			m.openCircuitBreaker("rest-api", 30*time.Second)

		case RecoveryActionRefreshCache:
			// Clear caches to force refresh
			m.logger.Info("Refreshing cache due to error recovery", nil)

		case RecoveryActionFallback:
			// Fallback is handled at a higher level
			m.logger.Info("Using fallback due to error recovery", nil)
			return nil
		}

		// Retry the operation
		if err := operation(); err != nil {
			lastErr = err
			continue
		}

		// Success - record recovery
		m.recordRecoverySuccess(strategy.Action, time.Since(startTime))
		return nil
	}

	// All attempts failed
	m.recordRecoveryFailure(strategy.Action, time.Since(startTime))

	// Escalate if configured
	if strategy.NotifyOps {
		m.escalateToOps(lastErr, strategy)
	}

	return lastErr
}

// executeReconnectHandlers runs all registered reconnection handlers
func (m *ErrorRecoveryManager) executeReconnectHandlers() {
	m.mu.RLock()
	handlers := make([]func() error, 0, len(m.reconnectHandlers))
	for _, handler := range m.reconnectHandlers {
		handlers = append(handlers, handler)
	}
	m.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(); err != nil {
			m.logger.Warn("Reconnection handler failed", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
}

// openCircuitBreaker opens a circuit breaker for a service
func (m *ErrorRecoveryManager) openCircuitBreaker(service string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cb, exists := m.circuitBreakers[service]; exists {
		cb.mu.Lock()
		cb.state = "open"
		cb.nextRetryTime = time.Now().Add(duration)
		cb.mu.Unlock()

		m.logger.Warn("Circuit breaker opened", map[string]interface{}{
			"service":  service,
			"duration": duration.String(),
		})
	}
}

// getDefaultStrategyForClass returns a default strategy based on error class
func (m *ErrorRecoveryManager) getDefaultStrategyForClass(class pkgerrors.ErrorClass) *RecoveryStrategy {
	switch class {
	case pkgerrors.ClassTransient, pkgerrors.ClassTimeout:
		return &RecoveryStrategy{
			Action:            RecoveryActionRetry,
			Delay:             1 * time.Second,
			MaxAttempts:       3,
			BackoffMultiplier: 2.0,
			FallbackEnabled:   true,
		}
	case pkgerrors.ClassCircuitBreaker:
		return &RecoveryStrategy{
			Action:          RecoveryActionFallback,
			Delay:           30 * time.Second,
			MaxAttempts:     1,
			FallbackEnabled: true,
		}
	case pkgerrors.ClassRateLimited:
		return &RecoveryStrategy{
			Action:            RecoveryActionRetry,
			Delay:             5 * time.Second,
			MaxAttempts:       5,
			BackoffMultiplier: 1.0,
		}
	default:
		// Non-retryable by default
		return &RecoveryStrategy{
			Action:      RecoveryActionNone,
			MaxAttempts: 0,
		}
	}
}

// RegisterReconnectHandler registers a handler to be called on reconnection
func (m *ErrorRecoveryManager) RegisterReconnectHandler(name string, handler func() error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconnectHandlers[name] = handler
}

// SetFallbackRegistry sets the fallback registry for recovery
func (m *ErrorRecoveryManager) SetFallbackRegistry(registry *FallbackRegistry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fallbackRegistry = registry
}

// logRecoveryAttempt logs a recovery attempt
func (m *ErrorRecoveryManager) logRecoveryAttempt(errorCode string, errorClass pkgerrors.ErrorClass, action RecoveryAction, correlationID string) {
	event := RecoveryEvent{
		Timestamp:     time.Now(),
		ErrorCode:     errorCode,
		ErrorClass:    errorClass,
		Action:        action,
		CorrelationID: correlationID,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.recoveryHistory = append(m.recoveryHistory, event)

	// Trim history if needed
	if len(m.recoveryHistory) > m.maxHistorySize {
		m.recoveryHistory = m.recoveryHistory[len(m.recoveryHistory)-m.maxHistorySize:]
	}
}

// recordRecoverySuccess records a successful recovery
func (m *ErrorRecoveryManager) recordRecoverySuccess(action RecoveryAction, duration time.Duration) {
	m.logger.Info("Recovery successful", map[string]interface{}{
		"action":   action,
		"duration": duration.String(),
	})

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.recoveryHistory) > 0 {
		m.recoveryHistory[len(m.recoveryHistory)-1].Success = true
		m.recoveryHistory[len(m.recoveryHistory)-1].Duration = duration
	}
}

// recordRecoveryFailure records a failed recovery
func (m *ErrorRecoveryManager) recordRecoveryFailure(action RecoveryAction, duration time.Duration) {
	m.logger.Error("Recovery failed", map[string]interface{}{
		"action":   action,
		"duration": duration.String(),
	})

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.recoveryHistory) > 0 {
		m.recoveryHistory[len(m.recoveryHistory)-1].Success = false
		m.recoveryHistory[len(m.recoveryHistory)-1].Duration = duration
	}
}

// escalateToOps notifies operations team about critical errors
func (m *ErrorRecoveryManager) escalateToOps(err error, strategy *RecoveryStrategy) {
	m.logger.Error("CRITICAL: Error escalated to operations", map[string]interface{}{
		"error":    err.Error(),
		"strategy": strategy,
		"action":   "Manual intervention may be required",
	})

	// In production, this would trigger alerts via PagerDuty, Slack, etc.
}

// GetRecoveryStats returns statistics about recovery attempts
func (m *ErrorRecoveryManager) GetRecoveryStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	successCount := 0
	failureCount := 0
	actionCounts := make(map[RecoveryAction]int)

	for _, event := range m.recoveryHistory {
		if event.Success {
			successCount++
		} else {
			failureCount++
		}
		actionCounts[event.Action]++
	}

	return map[string]interface{}{
		"total_attempts":   len(m.recoveryHistory),
		"successful":       successCount,
		"failed":           failureCount,
		"success_rate":     float64(successCount) / float64(len(m.recoveryHistory)),
		"action_breakdown": actionCounts,
		"history_size":     len(m.recoveryHistory),
	}
}
