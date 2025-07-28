package webhook

import (
	"context"
	"runtime"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// MetricsCollector interface for collecting custom metrics
type MetricsCollector interface {
	CollectMetrics()
}

// MonitoringService provides comprehensive monitoring capabilities
type MonitoringService struct {
	logger          observability.Logger
	metricsInterval time.Duration
	stopCh          chan struct{}
	collectors      []MetricsCollector
}

// NewMonitoringService creates a new monitoring service
func NewMonitoringService(logger observability.Logger, metricsInterval time.Duration) *MonitoringService {
	if metricsInterval == 0 {
		metricsInterval = 30 * time.Second
	}

	return &MonitoringService{
		logger:          logger,
		metricsInterval: metricsInterval,
		stopCh:          make(chan struct{}),
		collectors:      make([]MetricsCollector, 0),
	}
}

// RegisterCollector registers a metrics collector
func (m *MonitoringService) RegisterCollector(collector MetricsCollector) {
	m.collectors = append(m.collectors, collector)
}

// Start starts the monitoring service
func (m *MonitoringService) Start(ctx context.Context) {
	go m.collectSystemMetrics(ctx)
	go m.runCollectors(ctx)
}

// Stop stops the monitoring service
func (m *MonitoringService) Stop() {
	close(m.stopCh)
}

// collectSystemMetrics collects system-level metrics
func (m *MonitoringService) collectSystemMetrics(ctx context.Context) {
	ticker := time.NewTicker(m.metricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			// Update memory usage
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			// Update system metrics
			m.logger.Debug("System metrics", map[string]interface{}{
				"goroutines":   runtime.NumGoroutine(),
				"heap_alloc":   memStats.HeapAlloc,
				"stack_inuse":  memStats.StackInuse,
				"total_memory": memStats.Sys,
			})
		}
	}
}

// runCollectors runs registered collectors
func (m *MonitoringService) runCollectors(ctx context.Context) {
	ticker := time.NewTicker(m.metricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			for _, collector := range m.collectors {
				collector.CollectMetrics()
			}
		}
	}
}

// InstrumentedWebhookProcessor wraps webhook processor with monitoring
type InstrumentedWebhookProcessor struct {
	processor       EventProcessor
	logger          observability.Logger
	metrics         *WebhookMetricsAdapter
	tracer          observability.Tracer
	securityService *SecurityService
}

// NewInstrumentedWebhookProcessor creates an instrumented processor
func NewInstrumentedWebhookProcessor(
	processor EventProcessor,
	logger observability.Logger,
	metrics observability.MetricsClient,
	tracer observability.Tracer,
	security *SecurityService,
) *InstrumentedWebhookProcessor {
	return &InstrumentedWebhookProcessor{
		processor:       processor,
		logger:          logger,
		metrics:         NewWebhookMetricsAdapter(metrics),
		tracer:          tracer,
		securityService: security,
	}
}

// ProcessEvent processes an event with full instrumentation
func (p *InstrumentedWebhookProcessor) ProcessEvent(ctx context.Context, event *WebhookEvent) error {
	start := time.Now()

	// Record event received
	p.metrics.RecordWebhookReceived(event.TenantId, event.ToolId, event.EventType)

	// Start tracing span
	ctx, span := p.tracer.StartSpan(ctx, "webhook.process")
	defer span.End()

	// Add event attributes to span
	span.SetAttribute("webhook.event_id", event.EventId)
	span.SetAttribute("webhook.tenant_id", event.TenantId)
	span.SetAttribute("webhook.tool_id", event.ToolId)
	span.SetAttribute("webhook.event_type", event.EventType)
	span.SetAttribute("webhook.version", event.Version)

	// Security audit log
	if p.securityService != nil {
		p.securityService.AuditLog(event, "event_received", "success", nil)
	}

	// Process the event
	err := p.processor.ProcessEvent(ctx, event)

	// Record metrics
	duration := time.Since(start)
	if err != nil {
		p.metrics.RecordWebhookFailed(event.TenantId, event.ToolId, event.EventType, getErrorType(err))
		span.RecordError(err)
		span.SetStatus(2, "Failed to process webhook event") // 2 = Error

		p.logger.Error("Failed to process webhook event", map[string]interface{}{
			"event_id":    event.EventId,
			"tenant_id":   event.TenantId,
			"tool_id":     event.ToolId,
			"event_type":  event.EventType,
			"error":       err.Error(),
			"duration_ms": duration.Milliseconds(),
		})

		if p.securityService != nil {
			p.securityService.AuditLog(event, "event_processed", "failure", map[string]interface{}{
				"error":       err.Error(),
				"duration_ms": duration.Milliseconds(),
			})
		}
	} else {
		p.metrics.RecordWebhookProcessed(event.TenantId, event.ToolId, event.EventType, duration)
		span.SetStatus(1, "Success") // 1 = OK

		p.logger.Debug("Successfully processed webhook event", map[string]interface{}{
			"event_id":    event.EventId,
			"tenant_id":   event.TenantId,
			"tool_id":     event.ToolId,
			"event_type":  event.EventType,
			"duration_ms": duration.Milliseconds(),
		})

		if p.securityService != nil {
			p.securityService.AuditLog(event, "event_processed", "success", map[string]interface{}{
				"duration_ms": duration.Milliseconds(),
			})
		}
	}

	return err
}

// GetToolID returns the tool ID
func (p *InstrumentedWebhookProcessor) GetToolID() string {
	return p.processor.GetToolID()
}

// getErrorType categorizes errors for metrics
func getErrorType(err error) string {
	if err == nil {
		return "none"
	}

	// Import error types from appropriate packages
	if err.Error() == "circuit breaker is open" {
		return "circuit_open"
	}
	if err.Error() == "storage: key not found" {
		return "not_found"
	}

	if isTimeoutError(err) {
		return "timeout"
	}
	if isNetworkError(err) {
		return "network"
	}
	return "unknown"
}

// isTimeoutError checks if error is timeout-related
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	// Check for context deadline exceeded
	if err == context.DeadlineExceeded {
		return true
	}
	// Add other timeout error checks as needed
	return false
}

// isNetworkError checks if error is network-related
func isNetworkError(err error) bool {
	// Add network error detection logic
	return false
}

// WebhookMetricsAdapter adapts webhook metrics to observability.MetricsClient
type WebhookMetricsAdapter struct {
	metrics observability.MetricsClient
}

// NewWebhookMetricsAdapter creates a new webhook metrics adapter
func NewWebhookMetricsAdapter(metrics observability.MetricsClient) *WebhookMetricsAdapter {
	return &WebhookMetricsAdapter{
		metrics: metrics,
	}
}

// RecordWebhookReceived records a webhook received event
func (w *WebhookMetricsAdapter) RecordWebhookReceived(tenantID, toolID, eventType string) {
	w.metrics.RecordCounter("webhook_events_received_total", 1, map[string]string{
		"tenant_id":  tenantID,
		"tool_id":    toolID,
		"event_type": eventType,
	})
}

// RecordWebhookProcessed records a successfully processed webhook
func (w *WebhookMetricsAdapter) RecordWebhookProcessed(tenantID, toolID, eventType string, duration time.Duration) {
	w.metrics.RecordCounter("webhook_events_processed_total", 1, map[string]string{
		"tenant_id":  tenantID,
		"tool_id":    toolID,
		"event_type": eventType,
	})
	w.metrics.RecordTimer("webhook_event_processing_duration_seconds", duration, map[string]string{
		"tenant_id":  tenantID,
		"tool_id":    toolID,
		"event_type": eventType,
	})
}

// RecordWebhookFailed records a failed webhook
func (w *WebhookMetricsAdapter) RecordWebhookFailed(tenantID, toolID, eventType, errorType string) {
	w.metrics.RecordCounter("webhook_events_failed_total", 1, map[string]string{
		"tenant_id":  tenantID,
		"tool_id":    toolID,
		"event_type": eventType,
		"error_type": errorType,
	})
}

// RecordDuplicateDetected records duplicate detection
func (w *WebhookMetricsAdapter) RecordDuplicateDetected(tenantID, toolID, eventType string) {
	w.metrics.RecordCounter("webhook_duplicates_detected_total", 1, map[string]string{
		"tenant_id":  tenantID,
		"tool_id":    toolID,
		"event_type": eventType,
	})
}

// RecordSecurityViolation records a security violation
func (w *WebhookMetricsAdapter) RecordSecurityViolation(violationType, toolID string) {
	w.metrics.RecordCounter("webhook_security_violations_total", 1, map[string]string{
		"violation_type": violationType,
		"tool_id":        toolID,
	})
}
