package observability

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusMetricsClient implements MetricsClient interface using Prometheus
type PrometheusMetricsClient struct {
	namespace string
	subsystem string

	// Metric collectors
	counters   map[string]*prometheus.CounterVec
	gauges     map[string]*prometheus.GaugeVec
	histograms map[string]*prometheus.HistogramVec
	summaries  map[string]*prometheus.SummaryVec

	// Mutex for thread-safe operations
	mu sync.RWMutex

	// Common labels
	commonLabels prometheus.Labels
}

// NewPrometheusMetricsClient creates a new Prometheus metrics client
func NewPrometheusMetricsClient(namespace, subsystem string, commonLabels map[string]string) *PrometheusMetricsClient {
	labels := prometheus.Labels{}
	for k, v := range commonLabels {
		labels[k] = v
	}

	client := &PrometheusMetricsClient{
		namespace:    namespace,
		subsystem:    subsystem,
		counters:     make(map[string]*prometheus.CounterVec),
		gauges:       make(map[string]*prometheus.GaugeVec),
		histograms:   make(map[string]*prometheus.HistogramVec),
		summaries:    make(map[string]*prometheus.SummaryVec),
		commonLabels: labels,
	}

	// Register default metrics
	client.registerDefaultMetrics()

	return client
}

// registerDefaultMetrics registers commonly used metrics
func (c *PrometheusMetricsClient) registerDefaultMetrics() {
	// API operation metrics
	c.getOrCreateCounter("api_requests_total", "Total API requests", []string{"method", "endpoint", "status"})
	c.getOrCreateHistogram("api_request_duration_seconds", "API request duration", []string{"method", "endpoint"}, prometheus.DefBuckets)

	// Database operation metrics
	c.getOrCreateCounter("database_operations_total", "Total database operations", []string{"operation", "table", "status"})
	c.getOrCreateHistogram("database_operation_duration_seconds", "Database operation duration", []string{"operation", "table"}, prometheus.DefBuckets)

	// Cache operation metrics
	c.getOrCreateCounter("cache_operations_total", "Total cache operations", []string{"operation", "result"})
	c.getOrCreateHistogram("cache_operation_duration_seconds", "Cache operation duration", []string{"operation"}, prometheus.DefBuckets)

	// Circuit breaker metrics
	c.getOrCreateCounter("circuit_breaker_state_changes_total", "Circuit breaker state changes", []string{"name", "from", "to"})
	c.getOrCreateGauge("circuit_breaker_state", "Current circuit breaker state", []string{"name"})

	// Health check metrics
	c.getOrCreateGauge("health_check_status", "Health check status (1=healthy, 0=unhealthy)", []string{"component"})
	c.getOrCreateHistogram("health_check_duration_seconds", "Health check duration", []string{"component"}, prometheus.DefBuckets)

	// Embedding model metrics for multi-tenant management
	c.getOrCreateCounter("embedding_requests_total", "Total embedding requests", []string{"tenant_id", "model", "provider", "status"})
	c.getOrCreateHistogram("embedding_request_duration_seconds", "Embedding request duration", []string{"tenant_id", "model", "provider"}, prometheus.DefBuckets)
	c.getOrCreateCounter("embedding_tokens_total", "Total tokens processed", []string{"tenant_id", "model", "provider"})
	c.getOrCreateCounter("embedding_cost_usd_total", "Total cost in USD", []string{"tenant_id", "model", "provider"})
	c.getOrCreateGauge("embedding_quota_usage_ratio", "Quota usage ratio (0-1)", []string{"tenant_id", "quota_type"})
	c.getOrCreateCounter("embedding_model_switches_total", "Model switch count", []string{"tenant_id", "from_model", "to_model", "reason"})
	c.getOrCreateCounter("embedding_cache_hits_total", "Embedding cache hits", []string{"tenant_id", "model"})
	c.getOrCreateCounter("embedding_errors_total", "Embedding errors", []string{"tenant_id", "model", "error_type"})
}

// RecordCounter records a counter metric
func (c *PrometheusMetricsClient) RecordCounter(name string, value float64, labels map[string]string) {
	counter := c.getOrCreateCounter(name, fmt.Sprintf("Counter for %s", name), c.getLabelNames(labels))
	counter.With(c.mergeLabelValues(labels)).Add(value)
}

// RecordGauge records a gauge metric
func (c *PrometheusMetricsClient) RecordGauge(name string, value float64, labels map[string]string) {
	gauge := c.getOrCreateGauge(name, fmt.Sprintf("Gauge for %s", name), c.getLabelNames(labels))
	gauge.With(c.mergeLabelValues(labels)).Set(value)
}

// RecordHistogram records a histogram metric
func (c *PrometheusMetricsClient) RecordHistogram(name string, value float64, labels map[string]string) {
	histogram := c.getOrCreateHistogram(name, fmt.Sprintf("Histogram for %s", name), c.getLabelNames(labels), prometheus.DefBuckets)
	histogram.With(c.mergeLabelValues(labels)).Observe(value)
}

// RecordTimer records a timer metric (returns a function to stop the timer)
func (c *PrometheusMetricsClient) RecordTimer(name string, labels map[string]string) func() {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		histogram := c.getOrCreateHistogram(name, fmt.Sprintf("Timer for %s", name), c.getLabelNames(labels), prometheus.DefBuckets)
		histogram.With(c.mergeLabelValues(labels)).Observe(v)
	}))

	return func() {
		timer.ObserveDuration()
	}
}

// IncrementCounter increments a counter by 1
func (c *PrometheusMetricsClient) IncrementCounter(name string, value float64) {
	c.RecordCounter(name, value, nil)
}

// IncrementCounterWithLabels increments a counter with labels
func (c *PrometheusMetricsClient) IncrementCounterWithLabels(name string, value float64, labels map[string]string) {
	c.RecordCounter(name, value, labels)
}

// RecordDuration records a duration in seconds
func (c *PrometheusMetricsClient) RecordDuration(name string, duration time.Duration, labels map[string]string) {
	c.RecordHistogram(name, duration.Seconds(), labels)
}

// StartTimer starts a timer and returns a function to stop it
func (c *PrometheusMetricsClient) StartTimer(name string, labels map[string]string) func() {
	start := time.Now()
	return func() {
		c.RecordDuration(name, time.Since(start), labels)
	}
}

// RecordCacheOperation records a cache operation
func (c *PrometheusMetricsClient) RecordCacheOperation(operation string, hit bool, duration time.Duration) {
	result := "miss"
	if hit {
		result = "hit"
	}

	c.IncrementCounterWithLabels("cache_operations_total", 1, map[string]string{
		"operation": operation,
		"result":    result,
	})

	c.RecordDuration("cache_operation_duration_seconds", duration, map[string]string{
		"operation": operation,
	})
}

// RecordAPIOperation records an API operation
func (c *PrometheusMetricsClient) RecordAPIOperation(method, endpoint string, statusCode int, duration time.Duration) {
	labels := map[string]string{
		"method":   method,
		"endpoint": endpoint,
		"status":   fmt.Sprintf("%d", statusCode),
	}

	c.IncrementCounterWithLabels("api_requests_total", 1, labels)
	c.RecordDuration("api_request_duration_seconds", duration, map[string]string{
		"method":   method,
		"endpoint": endpoint,
	})
}

// RecordDatabaseOperation records a database operation
func (c *PrometheusMetricsClient) RecordDatabaseOperation(operation, table string, err error, duration time.Duration) {
	status := "success"
	if err != nil {
		status = "error"
	}

	labels := map[string]string{
		"operation": operation,
		"table":     table,
		"status":    status,
	}

	c.IncrementCounterWithLabels("database_operations_total", 1, labels)
	c.RecordDuration("database_operation_duration_seconds", duration, map[string]string{
		"operation": operation,
		"table":     table,
	})
}

// Helper methods

func (c *PrometheusMetricsClient) getOrCreateCounter(name, help string, labels []string) *prometheus.CounterVec {
	c.mu.RLock()
	if counter, exists := c.counters[name]; exists {
		c.mu.RUnlock()
		return counter
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if counter, exists := c.counters[name]; exists {
		return counter
	}

	counter := promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: c.namespace,
		Subsystem: c.subsystem,
		Name:      name,
		Help:      help,
	}, labels)

	c.counters[name] = counter
	return counter
}

func (c *PrometheusMetricsClient) getOrCreateGauge(name, help string, labels []string) *prometheus.GaugeVec {
	c.mu.RLock()
	if gauge, exists := c.gauges[name]; exists {
		c.mu.RUnlock()
		return gauge
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if gauge, exists := c.gauges[name]; exists {
		return gauge
	}

	gauge := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: c.namespace,
		Subsystem: c.subsystem,
		Name:      name,
		Help:      help,
	}, labels)

	c.gauges[name] = gauge
	return gauge
}

func (c *PrometheusMetricsClient) getOrCreateHistogram(name, help string, labels []string, buckets []float64) *prometheus.HistogramVec {
	c.mu.RLock()
	if histogram, exists := c.histograms[name]; exists {
		c.mu.RUnlock()
		return histogram
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if histogram, exists := c.histograms[name]; exists {
		return histogram
	}

	histogram := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: c.namespace,
		Subsystem: c.subsystem,
		Name:      name,
		Help:      help,
		Buckets:   buckets,
	}, labels)

	c.histograms[name] = histogram
	return histogram
}

func (c *PrometheusMetricsClient) getLabelNames(labels map[string]string) []string {
	if labels == nil {
		return []string{}
	}

	names := make([]string, 0, len(labels))
	for name := range labels {
		names = append(names, name)
	}
	return names
}

func (c *PrometheusMetricsClient) mergeLabelValues(labels map[string]string) prometheus.Labels {
	merged := prometheus.Labels{}

	// Add common labels first
	for k, v := range c.commonLabels {
		merged[k] = v
	}

	// Override with specific labels
	for k, v := range labels {
		merged[k] = v
	}

	return merged
}

// RecordEmbeddingOperation records an embedding operation with comprehensive metrics
func (c *PrometheusMetricsClient) RecordEmbeddingOperation(tenantID, model, provider string, success bool, tokens int, cost float64, duration time.Duration) {
	status := "success"
	if !success {
		status = "failure"
	}

	labels := map[string]string{
		"tenant_id": tenantID,
		"model":     model,
		"provider":  provider,
	}

	// Record request count
	c.IncrementCounterWithLabels("embedding_requests_total", 1, map[string]string{
		"tenant_id": tenantID,
		"model":     model,
		"provider":  provider,
		"status":    status,
	})

	// Record duration
	c.RecordDuration("embedding_request_duration_seconds", duration, labels)

	// Record tokens if successful
	if success && tokens > 0 {
		c.RecordCounter("embedding_tokens_total", float64(tokens), labels)
	}

	// Record cost if provided
	if cost > 0 {
		c.RecordCounter("embedding_cost_usd_total", cost, labels)
	}
}

// RecordEmbeddingQuotaUsage records quota usage for a tenant
func (c *PrometheusMetricsClient) RecordEmbeddingQuotaUsage(tenantID string, quotaType string, used, limit int64) {
	if limit > 0 {
		ratio := float64(used) / float64(limit)
		c.RecordGauge("embedding_quota_usage_ratio", ratio, map[string]string{
			"tenant_id":  tenantID,
			"quota_type": quotaType,
		})
	}
}

// RecordEmbeddingModelSwitch records when a model switch occurs
func (c *PrometheusMetricsClient) RecordEmbeddingModelSwitch(tenantID, fromModel, toModel, reason string) {
	c.IncrementCounterWithLabels("embedding_model_switches_total", 1, map[string]string{
		"tenant_id":  tenantID,
		"from_model": fromModel,
		"to_model":   toModel,
		"reason":     reason,
	})
}

// RecordEmbeddingCacheHit records cache hit for embeddings
func (c *PrometheusMetricsClient) RecordEmbeddingCacheHit(tenantID, model string) {
	c.IncrementCounterWithLabels("embedding_cache_hits_total", 1, map[string]string{
		"tenant_id": tenantID,
		"model":     model,
	})
}

// RecordEmbeddingError records embedding errors
func (c *PrometheusMetricsClient) RecordEmbeddingError(tenantID, model, errorType string) {
	c.IncrementCounterWithLabels("embedding_errors_total", 1, map[string]string{
		"tenant_id":  tenantID,
		"model":      model,
		"error_type": errorType,
	})
}

// WebSocketMetrics provides WebSocket-specific metrics
type WebSocketMetrics struct {
	ConnectionsTotal      prometheus.Counter
	ConnectionsActive     prometheus.Gauge
	MessagesSent          prometheus.Counter
	MessagesReceived      prometheus.Counter
	MessageLatency        prometheus.Histogram
	TasksCreated          prometheus.Counter
	TasksCompleted        prometheus.Counter
	TasksFailed           prometheus.Counter
	WorkflowsExecuted     prometheus.Counter
	WorkflowDuration      prometheus.Histogram
	DatabaseQueryDuration prometheus.Histogram
	CacheHitRate          prometheus.Gauge
}

// NewWebSocketMetrics creates WebSocket-specific metrics
func NewWebSocketMetrics(namespace string) *WebSocketMetrics {
	return &WebSocketMetrics{
		ConnectionsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "websocket",
			Name:      "connections_total",
			Help:      "Total number of WebSocket connections",
		}),
		ConnectionsActive: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "websocket",
			Name:      "connections_active",
			Help:      "Number of active WebSocket connections",
		}),
		MessagesSent: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "websocket",
			Name:      "messages_sent_total",
			Help:      "Total number of messages sent",
		}),
		MessagesReceived: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "websocket",
			Name:      "messages_received_total",
			Help:      "Total number of messages received",
		}),
		MessageLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "websocket",
			Name:      "message_latency_seconds",
			Help:      "Message processing latency",
			Buckets:   prometheus.DefBuckets,
		}),
		TasksCreated: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "websocket",
			Name:      "tasks_created_total",
			Help:      "Total number of tasks created",
		}),
		TasksCompleted: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "websocket",
			Name:      "tasks_completed_total",
			Help:      "Total number of tasks completed",
		}),
		TasksFailed: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "websocket",
			Name:      "tasks_failed_total",
			Help:      "Total number of tasks failed",
		}),
		WorkflowsExecuted: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "websocket",
			Name:      "workflows_executed_total",
			Help:      "Total number of workflows executed",
		}),
		WorkflowDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "websocket",
			Name:      "workflow_duration_seconds",
			Help:      "Workflow execution duration",
			Buckets:   prometheus.DefBuckets,
		}),
		DatabaseQueryDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "websocket",
			Name:      "database_query_duration_seconds",
			Help:      "Database query duration",
			Buckets:   prometheus.DefBuckets,
		}),
		CacheHitRate: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "websocket",
			Name:      "cache_hit_rate",
			Help:      "Cache hit rate percentage",
		}),
	}
}
