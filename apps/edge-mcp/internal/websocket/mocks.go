package websocket

import (
	"time"
)

// MockMetricsClient is a mock implementation of observability.MetricsClient for testing
type MockMetricsClient struct{}

// IncrementCounter is a no-op implementation
func (m *MockMetricsClient) IncrementCounter(name string, value float64) {}

// IncrementCounterWithLabels is a no-op implementation
func (m *MockMetricsClient) IncrementCounterWithLabels(name string, value float64, labels map[string]string) {
}

// RecordCounter is a no-op implementation
func (m *MockMetricsClient) RecordCounter(name string, value float64, labels map[string]string) {}

// RecordGauge is a no-op implementation
func (m *MockMetricsClient) RecordGauge(name string, value float64, labels map[string]string) {}

// RecordHistogram is a no-op implementation
func (m *MockMetricsClient) RecordHistogram(name string, value float64, labels map[string]string) {}

// RecordTimer is a no-op implementation
func (m *MockMetricsClient) RecordTimer(name string, duration time.Duration, labels map[string]string) {
}

// StartTimer is a no-op implementation
func (m *MockMetricsClient) StartTimer(name string, labels map[string]string) func() {
	return func() {}
}

// RecordCacheOperation is a no-op implementation
func (m *MockMetricsClient) RecordCacheOperation(operation string, success bool, durationSeconds float64) {
}

// RecordOperation is a no-op implementation
func (m *MockMetricsClient) RecordOperation(component string, operation string, success bool, durationSeconds float64, labels map[string]string) {
}

// RecordAPIOperation is a no-op implementation
func (m *MockMetricsClient) RecordAPIOperation(method string, endpoint string, success bool, durationSeconds float64) {
}

// Close is a no-op implementation
func (m *MockMetricsClient) Close() error { return nil }

// Add missing methods to implement observability.MetricsClient

// RecordEvent is a no-op implementation
func (m *MockMetricsClient) RecordEvent(source, eventType string) {}

// RecordLatency is a no-op implementation
func (m *MockMetricsClient) RecordLatency(operation string, duration time.Duration) {}

// RecordDatabaseOperation is a no-op implementation
func (m *MockMetricsClient) RecordDatabaseOperation(operation string, success bool, durationSeconds float64) {
}

// RecordDuration is a no-op implementation
func (m *MockMetricsClient) RecordDuration(name string, duration time.Duration) {}
