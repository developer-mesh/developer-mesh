package observability

import (
	"time"
)

// Default metrics implementation
type metricsClient struct {
	enabled bool
	labels  map[string]string
}

// MetricsOptions contains configuration options for creating a metrics client
type MetricsOptions struct {
	Enabled bool
	Labels  map[string]string
}

// NOTE: DefaultMetricsClient is now defined in observability.go
// This comment remains to document the initialization

// NewMetricsClient creates a new metrics client with default options
func NewMetricsClient() MetricsClient {
	return NewMetricsClientWithOptions(MetricsOptions{
		Enabled: true,
		Labels:  map[string]string{},
	})
}

// NewMetricsClientWithOptions creates a new metrics client with specific options
func NewMetricsClientWithOptions(options MetricsOptions) MetricsClient {
	return &metricsClient{
		enabled: options.Enabled,
		labels:  options.Labels,
	}
}

// IncrementCounter increments a counter metric by a given value (legacy version without labels)
func (m *metricsClient) IncrementCounter(name string, value float64) {
	if !m.enabled {
		return
	}
	m.RecordCounter(name, value, m.labels)
}

// IncrementCounterWithLabels increments a counter metric by a given value with custom labels
func (m *metricsClient) IncrementCounterWithLabels(name string, value float64, labels map[string]string) {
	if !m.enabled {
		return
	}
	// If labels are provided, use them; otherwise, use default labels
	effectiveLabels := m.labels
	if labels != nil {
		effectiveLabels = labels
	}
	m.RecordCounter(name, value, effectiveLabels)
}

// RecordDuration records a duration metric
func (m *metricsClient) RecordDuration(name string, duration time.Duration) {
	if !m.enabled {
		return
	}
	m.RecordHistogram(name, duration.Seconds(), m.labels)
}

// RecordEvent records an event metric
func (m *metricsClient) RecordEvent(source, eventType string) {
	if !m.enabled {
		return
	}

	labels := map[string]string{
		"source":     source,
		"event_type": eventType,
	}

	m.RecordCounter("events_total", 1.0, labels)
}

// RecordLatency records a latency metric
func (m *metricsClient) RecordLatency(operation string, duration time.Duration) {
	if !m.enabled {
		return
	}

	labels := map[string]string{
		"operation": operation,
	}

	m.RecordTimer(operation+"_latency", duration, labels)
}

// RecordCounter increments a counter metric
func (m *metricsClient) RecordCounter(name string, value float64, labels map[string]string) {
	if !m.enabled {
		return
	}

	// Placeholder for recording a counter metric
	// In a production implementation, this would use a metrics library like Prometheus
}

// RecordGauge records a gauge metric
func (m *metricsClient) RecordGauge(name string, value float64, labels map[string]string) {
	if !m.enabled {
		return
	}

	// Placeholder for recording a gauge metric
}

// RecordHistogram records a histogram metric
func (m *metricsClient) RecordHistogram(name string, value float64, labels map[string]string) {
	if !m.enabled {
		return
	}

	// Placeholder for recording a histogram metric
}

// RecordTimer records a timer metric
func (m *metricsClient) RecordTimer(name string, duration time.Duration, labels map[string]string) {
	if !m.enabled {
		return
	}

	m.RecordHistogram(name+"_seconds", duration.Seconds(), labels)
}

// RecordCacheOperation records cache operation metrics
func (m *metricsClient) RecordCacheOperation(operation string, success bool, durationSeconds float64) {
	if !m.enabled {
		return
	}

	labels := map[string]string{
		"operation": operation,
		"success":   stringFromBool(success),
	}

	m.RecordCounter("cache_operations_total", 1.0, labels)
	m.RecordHistogram("cache_operation_duration_seconds", durationSeconds, labels)
}

// RecordAPIOperation records API operation metrics
func (m *metricsClient) RecordAPIOperation(api string, operation string, success bool, durationSeconds float64) {
	if !m.enabled {
		return
	}

	labels := map[string]string{
		"api":       api,
		"operation": operation,
		"success":   stringFromBool(success),
	}

	m.RecordCounter("api_operations_total", 1.0, labels)
	m.RecordHistogram("api_operation_duration_seconds", durationSeconds, labels)
}

// RecordDatabaseOperation records database operation metrics
func (m *metricsClient) RecordDatabaseOperation(operation string, success bool, durationSeconds float64) {
	if !m.enabled {
		return
	}

	labels := map[string]string{
		"operation": operation,
		"success":   stringFromBool(success),
	}

	m.RecordCounter("database_operations_total", 1.0, labels)
	m.RecordHistogram("database_operation_duration_seconds", durationSeconds, labels)
}

// RecordOperation records operation metrics for adapters and other components
func (m *metricsClient) RecordOperation(component string, operation string, success bool, durationSeconds float64, labels map[string]string) {
	if !m.enabled {
		return
	}

	// Merge the base labels with the operation-specific ones
	mergedLabels := map[string]string{
		"component": component,
		"operation": operation,
		"success":   stringFromBool(success),
	}

	for k, v := range labels {
		mergedLabels[k] = v
	}

	m.RecordCounter("operations_total", 1.0, mergedLabels)
	m.RecordHistogram("operation_duration_seconds", durationSeconds, mergedLabels)
}

// StartTimer starts a timer metric
func (m *metricsClient) StartTimer(name string, labels map[string]string) func() {
	if !m.enabled {
		return func() {}
	}

	startTime := time.Now()
	return func() {
		duration := time.Since(startTime)
		m.RecordTimer(name, duration, labels)
	}
}

// Close closes the metrics client and returns any error
func (m *metricsClient) Close() error {
	// Placeholder for cleanup
	return nil
}

// Helper function to convert bool to string
func stringFromBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
