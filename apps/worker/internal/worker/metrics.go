package worker

import (
	"context"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// MetricsCollector provides comprehensive metrics collection for webhook processing
type MetricsCollector struct {
	metrics observability.MetricsClient
	tracer  trace.Tracer
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(metrics observability.MetricsClient, tracer trace.Tracer) *MetricsCollector {
	return &MetricsCollector{
		metrics: metrics,
		tracer:  tracer,
	}
}

// RecordEventReceived records when an event is received
func (m *MetricsCollector) RecordEventReceived(ctx context.Context, eventType, toolID string) {
	_, span := m.tracer.Start(ctx, "webhook.event.received",
		trace.WithAttributes(
			attribute.String("event.type", eventType),
			attribute.String("tool.id", toolID),
		),
	)
	defer span.End()

	m.metrics.IncrementCounterWithLabels("webhook_events_received_total", 1, map[string]string{
		"event_type": eventType,
		"tool_id":    toolID,
	})
}

// RecordProcessingStart records when event processing starts
func (m *MetricsCollector) RecordProcessingStart(ctx context.Context, eventID, eventType, toolID string) (context.Context, trace.Span) {
	ctx, span := m.tracer.Start(ctx, "webhook.event.process",
		trace.WithAttributes(
			attribute.String("event.id", eventID),
			attribute.String("event.type", eventType),
			attribute.String("tool.id", toolID),
		),
	)

	m.metrics.IncrementCounterWithLabels("webhook_events_processing_total", 1, map[string]string{
		"event_type": eventType,
		"tool_id":    toolID,
	})

	return ctx, span
}

// RecordProcessingComplete records when event processing completes
func (m *MetricsCollector) RecordProcessingComplete(span trace.Span, eventType, toolID, status string, duration time.Duration) {
	span.SetAttributes(
		attribute.String("processing.status", status),
		attribute.Int64("processing.duration_ms", duration.Milliseconds()),
	)
	span.End()

	labels := map[string]string{
		"event_type": eventType,
		"tool_id":    toolID,
		"status":     status,
	}

	m.metrics.IncrementCounterWithLabels("webhook_events_processed_total", 1, labels)
	m.metrics.RecordHistogram("webhook_event_processing_duration_seconds", duration.Seconds(), labels)
}

// RecordRetryAttempt records a retry attempt
func (m *MetricsCollector) RecordRetryAttempt(ctx context.Context, eventID string, attempt int, reason string) {
	_, span := m.tracer.Start(ctx, "webhook.event.retry",
		trace.WithAttributes(
			attribute.String("event.id", eventID),
			attribute.Int("retry.attempt", attempt),
			attribute.String("retry.reason", reason),
		),
	)
	defer span.End()

	m.metrics.IncrementCounterWithLabels("webhook_retry_attempts_total", 1, map[string]string{
		"attempt": string(rune(attempt)),
		"reason":  reason,
	})
}

// RecordDLQEntry records when an event is sent to DLQ
func (m *MetricsCollector) RecordDLQEntry(ctx context.Context, eventID, eventType, reason string) {
	_, span := m.tracer.Start(ctx, "webhook.dlq.entry",
		trace.WithAttributes(
			attribute.String("event.id", eventID),
			attribute.String("event.type", eventType),
			attribute.String("dlq.reason", reason),
		),
	)
	defer span.End()

	m.metrics.IncrementCounterWithLabels("webhook_dlq_entries_total", 1, map[string]string{
		"event_type": eventType,
		"reason":     reason,
	})
}

// RecordDLQRetry records a DLQ retry attempt
func (m *MetricsCollector) RecordDLQRetry(ctx context.Context, eventID string, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}

	_, span := m.tracer.Start(ctx, "webhook.dlq.retry",
		trace.WithAttributes(
			attribute.String("event.id", eventID),
			attribute.String("retry.status", status),
		),
	)
	defer span.End()

	m.metrics.IncrementCounterWithLabels("webhook_dlq_retries_total", 1, map[string]string{
		"status": status,
	})
}

// RecordTransformation records event transformation metrics
func (m *MetricsCollector) RecordTransformation(ctx context.Context, eventID string, rulesApplied int, duration time.Duration) {
	_, span := m.tracer.Start(ctx, "webhook.event.transform",
		trace.WithAttributes(
			attribute.String("event.id", eventID),
			attribute.Int("transform.rules_applied", rulesApplied),
			attribute.Int64("transform.duration_ms", duration.Milliseconds()),
		),
	)
	defer span.End()

	m.metrics.RecordHistogram("webhook_transformation_duration_seconds", duration.Seconds(), map[string]string{
		"rules_count": string(rune(rulesApplied)),
	})
}

// RecordQueueDepth records the current queue depth
func (m *MetricsCollector) RecordQueueDepth(depth int64) {
	m.metrics.RecordGauge("webhook_queue_depth", float64(depth), nil)
}

// RecordDLQDepth records the current DLQ depth
func (m *MetricsCollector) RecordDLQDepth(depth int64) {
	m.metrics.RecordGauge("webhook_dlq_depth", float64(depth), nil)
}

// RecordHealthCheck records health check results
func (m *MetricsCollector) RecordHealthCheck(component string, healthy bool, latency time.Duration) {
	status := "healthy"
	if !healthy {
		status = "unhealthy"
	}

	m.metrics.IncrementCounterWithLabels("webhook_health_checks_total", 1, map[string]string{
		"component": component,
		"status":    status,
	})

	m.metrics.RecordHistogram("webhook_health_check_duration_seconds", latency.Seconds(), map[string]string{
		"component": component,
	})
}

// RecordMemoryUsage records memory usage metrics
func (m *MetricsCollector) RecordMemoryUsage(allocated, total, system uint64) {
	m.metrics.RecordGauge("webhook_memory_allocated_bytes", float64(allocated), nil)
	m.metrics.RecordGauge("webhook_memory_total_bytes", float64(total), nil)
	m.metrics.RecordGauge("webhook_memory_system_bytes", float64(system), nil)
}

// RecordGoroutineCount records the number of active goroutines
func (m *MetricsCollector) RecordGoroutineCount(count int) {
	m.metrics.RecordGauge("webhook_goroutines_total", float64(count), nil)
}
