package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	namespace = "edge_mcp"
)

// Metrics holds all Prometheus metrics for the Edge MCP service
type Metrics struct {
	// Tool execution metrics
	ToolExecutionDuration *prometheus.HistogramVec
	ToolExecutionTotal    *prometheus.CounterVec
	ToolExecutionErrors   *prometheus.CounterVec

	// Connection metrics
	ActiveConnections prometheus.Gauge
	ConnectionsTotal  prometheus.Counter

	// Error metrics
	ErrorsTotal *prometheus.CounterVec

	// Cache metrics
	CacheOperationsTotal *prometheus.CounterVec
	CacheHitRatio        prometheus.Gauge

	// Request metrics
	RequestsTotal    *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	RequestsInFlight prometheus.Gauge

	// Session metrics
	ActiveSessions  prometheus.Gauge
	SessionsTotal   prometheus.Counter
	SessionDuration prometheus.Histogram

	// Message metrics
	MessagesSent     *prometheus.CounterVec
	MessagesReceived *prometheus.CounterVec
}

// New creates a new Metrics instance with all collectors registered
func New() *Metrics {
	m := &Metrics{
		// Tool execution duration histogram with custom buckets optimized for tool execution
		// Buckets: 10ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s, 30s
		ToolExecutionDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "tool_execution_duration_seconds",
				Help:      "Duration of tool execution in seconds",
				Buckets:   []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
			},
			[]string{"tool_name", "status"},
		),

		// Tool execution counter by tool name and status
		ToolExecutionTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "tool_execution_total",
				Help:      "Total number of tool executions",
			},
			[]string{"tool_name", "status"},
		),

		// Tool execution errors by tool name and error type
		ToolExecutionErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "tool_execution_errors_total",
				Help:      "Total number of tool execution errors",
			},
			[]string{"tool_name", "error_type"},
		),

		// Active WebSocket connections
		ActiveConnections: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "active_connections",
				Help:      "Number of active WebSocket connections",
			},
		),

		// Total connections counter
		ConnectionsTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "connections_total",
				Help:      "Total number of WebSocket connections established",
			},
		),

		// Error rate counter by error type
		ErrorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "errors_total",
				Help:      "Total number of errors by type",
			},
			[]string{"error_type", "error_code"},
		),

		// Cache operations counter by operation type and result
		CacheOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "cache_operations_total",
				Help:      "Total number of cache operations",
			},
			[]string{"operation", "result"},
		),

		// Cache hit ratio gauge (0-1 representing percentage)
		CacheHitRatio: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "cache_hit_ratio",
				Help:      "Cache hit ratio (0-1)",
			},
		),

		// Request rate by tool
		RequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "requests_total",
				Help:      "Total number of requests by tool",
			},
			[]string{"tool_name", "method"},
		),

		// Request duration histogram
		RequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "request_duration_seconds",
				Help:      "Duration of requests in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"tool_name", "method"},
		),

		// In-flight requests gauge
		RequestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "requests_in_flight",
				Help:      "Number of requests currently being processed",
			},
		),

		// Active sessions gauge
		ActiveSessions: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "active_sessions",
				Help:      "Number of active MCP sessions",
			},
		),

		// Total sessions counter
		SessionsTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "sessions_total",
				Help:      "Total number of MCP sessions created",
			},
		),

		// Session duration histogram
		SessionDuration: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "session_duration_seconds",
				Help:      "Duration of MCP sessions in seconds",
				Buckets:   []float64{60, 300, 600, 1800, 3600, 7200, 14400, 28800}, // 1m, 5m, 10m, 30m, 1h, 2h, 4h, 8h
			},
		),

		// Messages sent counter
		MessagesSent: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "messages_sent_total",
				Help:      "Total number of messages sent",
			},
			[]string{"message_type"},
		),

		// Messages received counter
		MessagesReceived: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "messages_received_total",
				Help:      "Total number of messages received",
			},
			[]string{"message_type"},
		),
	}

	return m
}

// RecordToolExecution records a tool execution with duration and status
func (m *Metrics) RecordToolExecution(toolName string, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}

	m.ToolExecutionDuration.WithLabelValues(toolName, status).Observe(duration.Seconds())
	m.ToolExecutionTotal.WithLabelValues(toolName, status).Inc()
}

// RecordToolExecutionError records a tool execution error with error type
func (m *Metrics) RecordToolExecutionError(toolName, errorType string) {
	m.ToolExecutionErrors.WithLabelValues(toolName, errorType).Inc()
}

// RecordConnectionStart records when a new connection is established
func (m *Metrics) RecordConnectionStart() {
	m.ConnectionsTotal.Inc()
	m.ActiveConnections.Inc()
}

// RecordConnectionEnd records when a connection is closed
func (m *Metrics) RecordConnectionEnd() {
	m.ActiveConnections.Dec()
}

// RecordError records an error by type and code
func (m *Metrics) RecordError(errorType, errorCode string) {
	m.ErrorsTotal.WithLabelValues(errorType, errorCode).Inc()
}

// RecordCacheOperation records a cache operation
func (m *Metrics) RecordCacheOperation(operation, result string) {
	m.CacheOperationsTotal.WithLabelValues(operation, result).Inc()
}

// UpdateCacheHitRatio updates the cache hit ratio gauge
// hits and total should be cumulative counts since service start
func (m *Metrics) UpdateCacheHitRatio(hits, total int) {
	if total > 0 {
		ratio := float64(hits) / float64(total)
		m.CacheHitRatio.Set(ratio)
	}
}

// RecordRequest records a request by tool and method
func (m *Metrics) RecordRequest(toolName, method string) {
	m.RequestsTotal.WithLabelValues(toolName, method).Inc()
}

// RecordRequestDuration records request duration
func (m *Metrics) RecordRequestDuration(toolName, method string, duration time.Duration) {
	m.RequestDuration.WithLabelValues(toolName, method).Observe(duration.Seconds())
}

// RecordRequestStart increments in-flight requests
func (m *Metrics) RecordRequestStart() {
	m.RequestsInFlight.Inc()
}

// RecordRequestEnd decrements in-flight requests
func (m *Metrics) RecordRequestEnd() {
	m.RequestsInFlight.Dec()
}

// RecordSessionStart records when a new session is created
func (m *Metrics) RecordSessionStart() {
	m.SessionsTotal.Inc()
	m.ActiveSessions.Inc()
}

// RecordSessionEnd records when a session ends with its duration
func (m *Metrics) RecordSessionEnd(duration time.Duration) {
	m.ActiveSessions.Dec()
	m.SessionDuration.Observe(duration.Seconds())
}

// RecordMessageSent records a sent message by type
func (m *Metrics) RecordMessageSent(messageType string) {
	m.MessagesSent.WithLabelValues(messageType).Inc()
}

// RecordMessageReceived records a received message by type
func (m *Metrics) RecordMessageReceived(messageType string) {
	m.MessagesReceived.WithLabelValues(messageType).Inc()
}

// StartToolExecutionTimer returns a function that when called, records the tool execution duration
// Usage: defer m.StartToolExecutionTimer(toolName)()
func (m *Metrics) StartToolExecutionTimer(toolName string) func(error) {
	start := time.Now()
	return func(err error) {
		m.RecordToolExecution(toolName, time.Since(start), err)
	}
}

// StartRequestTimer returns a function that when called, records the request duration
// Usage: defer m.StartRequestTimer(toolName, method)()
func (m *Metrics) StartRequestTimer(toolName, method string) func() {
	start := time.Now()
	return func() {
		m.RecordRequestDuration(toolName, method, time.Since(start))
	}
}
