package clients

import (
	"fmt"
	"sync"
	"time"

	pkgerrors "github.com/developer-mesh/developer-mesh/pkg/errors"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// ErrorMetrics tracks comprehensive error metrics
type ErrorMetrics struct {
	mu     sync.RWMutex
	logger observability.Logger

	// Error counts by classification
	errorsByClass     map[pkgerrors.ErrorClass]int64
	errorsByCode      map[string]int64
	errorsByOperation map[string]int64

	// Time-based metrics
	errorsPerMinute  []int64 // Circular buffer for last 60 minutes
	errorsPerHour    []int64 // Circular buffer for last 24 hours
	currentMinute    int
	currentHour      int
	lastMinuteUpdate time.Time
	lastHourUpdate   time.Time

	// Error patterns
	consecutiveErrors int
	lastErrorTime     time.Time
	errorRate         float64

	// Recovery metrics
	recoveryAttempts  int64
	recoverySuccesses int64
	recoveryFailures  int64
	avgRecoveryTime   time.Duration

	// Circuit breaker metrics
	circuitOpens     int64
	circuitCloses    int64
	circuitHalfOpens int64
	timeInOpenState  time.Duration
	lastCircuitOpen  time.Time

	// Fallback metrics
	fallbackActivations int64
	fallbackSuccesses   int64
	fallbackFailures    int64

	// Performance impact
	totalErrorLatency time.Duration
	maxErrorLatency   time.Duration
	avgErrorLatency   time.Duration

	// Alert thresholds
	errorRateThreshold   float64
	consecutiveThreshold int
	alertCallback        func(alert ErrorAlert)
}

// ErrorAlert represents an alert condition
type ErrorAlert struct {
	Type         string
	Severity     string
	Message      string
	Threshold    interface{}
	CurrentValue interface{}
	Timestamp    time.Time
	Details      map[string]interface{}
}

// NewErrorMetrics creates a new error metrics tracker
func NewErrorMetrics(logger observability.Logger) *ErrorMetrics {
	return &ErrorMetrics{
		logger:               logger,
		errorsByClass:        make(map[pkgerrors.ErrorClass]int64),
		errorsByCode:         make(map[string]int64),
		errorsByOperation:    make(map[string]int64),
		errorsPerMinute:      make([]int64, 60),
		errorsPerHour:        make([]int64, 24),
		lastMinuteUpdate:     time.Now(),
		lastHourUpdate:       time.Now(),
		errorRateThreshold:   10.0, // 10 errors per minute triggers alert
		consecutiveThreshold: 5,    // 5 consecutive errors triggers alert
	}
}

// RecordError records an error occurrence
func (m *ErrorMetrics) RecordError(err error, operation string, latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update time-based buckets
	m.updateTimeBuckets()

	// Extract error details
	var errorCode string
	var errorClass pkgerrors.ErrorClass

	if classifiedErr, ok := err.(*pkgerrors.ClassifiedError); ok {
		errorCode = classifiedErr.Code
		errorClass = classifiedErr.Class
	} else {
		errorCode = "UNKNOWN"
		errorClass = pkgerrors.ClassUnknown
	}

	// Update counters
	m.errorsByClass[errorClass]++
	m.errorsByCode[errorCode]++
	m.errorsByOperation[operation]++
	m.errorsPerMinute[m.currentMinute]++

	// Update consecutive errors
	if time.Since(m.lastErrorTime) < 10*time.Second {
		m.consecutiveErrors++
	} else {
		m.consecutiveErrors = 1
	}
	m.lastErrorTime = time.Now()

	// Update latency metrics
	m.totalErrorLatency += latency
	if latency > m.maxErrorLatency {
		m.maxErrorLatency = latency
	}

	// Calculate error rate (errors per minute)
	m.calculateErrorRate()

	// Check alert conditions
	m.checkAlertConditions(errorCode, errorClass, operation)

	// Log detailed metrics periodically
	if time.Since(m.lastMinuteUpdate) > time.Minute {
		m.logMetricsSummary()
	}
}

// RecordRecovery records a recovery attempt
func (m *ErrorMetrics) RecordRecovery(success bool, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.recoveryAttempts++
	if success {
		m.recoverySuccesses++
	} else {
		m.recoveryFailures++
	}

	// Update average recovery time
	if m.avgRecoveryTime == 0 {
		m.avgRecoveryTime = duration
	} else {
		m.avgRecoveryTime = (m.avgRecoveryTime + duration) / 2
	}
}

// RecordCircuitBreakerEvent records circuit breaker state changes
func (m *ErrorMetrics) RecordCircuitBreakerEvent(event string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch event {
	case "open":
		m.circuitOpens++
		m.lastCircuitOpen = time.Now()
	case "close":
		m.circuitCloses++
		if !m.lastCircuitOpen.IsZero() {
			m.timeInOpenState += time.Since(m.lastCircuitOpen)
		}
	case "half-open":
		m.circuitHalfOpens++
	}
}

// RecordFallback records fallback usage
func (m *ErrorMetrics) RecordFallback(success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.fallbackActivations++
	if success {
		m.fallbackSuccesses++
	} else {
		m.fallbackFailures++
	}
}

// updateTimeBuckets updates the time-based error buckets
func (m *ErrorMetrics) updateTimeBuckets() {
	now := time.Now()

	// Update minute bucket
	minutesSince := int(now.Sub(m.lastMinuteUpdate).Minutes())
	if minutesSince > 0 {
		// Clear old buckets
		for i := 1; i <= minutesSince && i < 60; i++ {
			idx := (m.currentMinute + i) % 60
			m.errorsPerMinute[idx] = 0
		}
		m.currentMinute = (m.currentMinute + minutesSince) % 60
		m.lastMinuteUpdate = now
	}

	// Update hour bucket
	hoursSince := int(now.Sub(m.lastHourUpdate).Hours())
	if hoursSince > 0 {
		// Clear old buckets
		for i := 1; i <= hoursSince && i < 24; i++ {
			idx := (m.currentHour + i) % 24
			m.errorsPerHour[idx] = 0
		}
		m.currentHour = (m.currentHour + hoursSince) % 24
		m.lastHourUpdate = now

		// Sum up errors in current hour from minute buckets
		hourErrors := int64(0)
		for _, count := range m.errorsPerMinute {
			hourErrors += count
		}
		m.errorsPerHour[m.currentHour] = hourErrors
	}
}

// calculateErrorRate calculates the current error rate
func (m *ErrorMetrics) calculateErrorRate() {
	// Calculate errors in last minute
	totalErrors := int64(0)
	for _, count := range m.errorsPerMinute {
		totalErrors += count
	}
	m.errorRate = float64(totalErrors) / 60.0 // Errors per second
}

// checkAlertConditions checks if any alert conditions are met
func (m *ErrorMetrics) checkAlertConditions(errorCode string, errorClass pkgerrors.ErrorClass, operation string) {
	alerts := []ErrorAlert{}

	// Check error rate threshold
	if m.errorRate > m.errorRateThreshold/60.0 {
		alerts = append(alerts, ErrorAlert{
			Type:         "ERROR_RATE_HIGH",
			Severity:     "WARNING",
			Message:      fmt.Sprintf("Error rate exceeded threshold: %.2f errors/sec", m.errorRate),
			Threshold:    m.errorRateThreshold,
			CurrentValue: m.errorRate * 60, // Convert to per minute
			Timestamp:    time.Now(),
			Details: map[string]interface{}{
				"operation": operation,
			},
		})
	}

	// Check consecutive errors threshold
	if m.consecutiveErrors >= m.consecutiveThreshold {
		alerts = append(alerts, ErrorAlert{
			Type:         "CONSECUTIVE_ERRORS",
			Severity:     "ERROR",
			Message:      fmt.Sprintf("Consecutive errors detected: %d errors", m.consecutiveErrors),
			Threshold:    m.consecutiveThreshold,
			CurrentValue: m.consecutiveErrors,
			Timestamp:    time.Now(),
			Details: map[string]interface{}{
				"error_code":  errorCode,
				"error_class": errorClass,
				"operation":   operation,
			},
		})
	}

	// Check circuit breaker opens
	if m.circuitOpens > 5 {
		alerts = append(alerts, ErrorAlert{
			Type:         "CIRCUIT_BREAKER_UNSTABLE",
			Severity:     "ERROR",
			Message:      fmt.Sprintf("Circuit breaker opened %d times", m.circuitOpens),
			Threshold:    5,
			CurrentValue: m.circuitOpens,
			Timestamp:    time.Now(),
			Details: map[string]interface{}{
				"time_in_open_state": m.timeInOpenState.String(),
			},
		})
	}

	// Send alerts
	for _, alert := range alerts {
		if m.alertCallback != nil {
			m.alertCallback(alert)
		}
		m.logger.Error("Error alert triggered", map[string]interface{}{
			"type":     alert.Type,
			"severity": alert.Severity,
			"message":  alert.Message,
			"details":  alert.Details,
		})
	}
}

// logMetricsSummary logs a summary of error metrics
func (m *ErrorMetrics) logMetricsSummary() {
	totalErrors := int64(0)
	for _, count := range m.errorsByClass {
		totalErrors += count
	}

	m.logger.Info("Error metrics summary", map[string]interface{}{
		"total_errors":          totalErrors,
		"error_rate":            fmt.Sprintf("%.2f/min", m.errorRate*60),
		"consecutive_errors":    m.consecutiveErrors,
		"recovery_success_rate": float64(m.recoverySuccesses) / float64(m.recoveryAttempts),
		"avg_recovery_time":     m.avgRecoveryTime.String(),
		"circuit_opens":         m.circuitOpens,
		"fallback_success_rate": float64(m.fallbackSuccesses) / float64(m.fallbackActivations),
		"avg_error_latency":     m.avgErrorLatency.String(),
		"max_error_latency":     m.maxErrorLatency.String(),
	})
}

// SetAlertCallback sets a callback for error alerts
func (m *ErrorMetrics) SetAlertCallback(callback func(ErrorAlert)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertCallback = callback
}

// GetMetricsSummary returns a summary of all error metrics
func (m *ErrorMetrics) GetMetricsSummary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalErrors := int64(0)
	for _, count := range m.errorsByClass {
		totalErrors += count
	}

	recoveryRate := float64(0)
	if m.recoveryAttempts > 0 {
		recoveryRate = float64(m.recoverySuccesses) / float64(m.recoveryAttempts)
	}

	fallbackRate := float64(0)
	if m.fallbackActivations > 0 {
		fallbackRate = float64(m.fallbackSuccesses) / float64(m.fallbackActivations)
	}

	// Calculate average latency
	if totalErrors > 0 {
		m.avgErrorLatency = m.totalErrorLatency / time.Duration(totalErrors)
	}

	return map[string]interface{}{
		"total_errors":          totalErrors,
		"errors_by_class":       m.errorsByClass,
		"errors_by_code":        m.errorsByCode,
		"errors_by_operation":   m.errorsByOperation,
		"error_rate_per_minute": m.errorRate * 60,
		"consecutive_errors":    m.consecutiveErrors,
		"recovery": map[string]interface{}{
			"attempts":          m.recoveryAttempts,
			"successes":         m.recoverySuccesses,
			"failures":          m.recoveryFailures,
			"success_rate":      recoveryRate,
			"avg_recovery_time": m.avgRecoveryTime.String(),
		},
		"circuit_breaker": map[string]interface{}{
			"opens":              m.circuitOpens,
			"closes":             m.circuitCloses,
			"half_opens":         m.circuitHalfOpens,
			"time_in_open_state": m.timeInOpenState.String(),
		},
		"fallback": map[string]interface{}{
			"activations":  m.fallbackActivations,
			"successes":    m.fallbackSuccesses,
			"failures":     m.fallbackFailures,
			"success_rate": fallbackRate,
		},
		"latency": map[string]interface{}{
			"avg": m.avgErrorLatency.String(),
			"max": m.maxErrorLatency.String(),
		},
	}
}

// Reset resets all metrics
func (m *ErrorMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.errorsByClass = make(map[pkgerrors.ErrorClass]int64)
	m.errorsByCode = make(map[string]int64)
	m.errorsByOperation = make(map[string]int64)
	m.errorsPerMinute = make([]int64, 60)
	m.errorsPerHour = make([]int64, 24)
	m.consecutiveErrors = 0
	m.recoveryAttempts = 0
	m.recoverySuccesses = 0
	m.recoveryFailures = 0
	m.circuitOpens = 0
	m.circuitCloses = 0
	m.circuitHalfOpens = 0
	m.fallbackActivations = 0
	m.fallbackSuccesses = 0
	m.fallbackFailures = 0
	m.totalErrorLatency = 0
	m.maxErrorLatency = 0
	m.avgErrorLatency = 0
	m.timeInOpenState = 0
}
