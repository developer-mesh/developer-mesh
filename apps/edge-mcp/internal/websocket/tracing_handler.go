package websocket

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	ws "github.com/developer-mesh/developer-mesh/pkg/models/websocket"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// TracingHandler wraps handlers with distributed tracing
type TracingHandler struct {
	tracer  observability.StartSpanFunc
	metrics observability.MetricsClient
	logger  observability.Logger
}

// NewTracingHandler creates a new tracing handler wrapper
func NewTracingHandler(tracer observability.StartSpanFunc, metrics observability.MetricsClient, logger observability.Logger) *TracingHandler {
	return &TracingHandler{
		tracer:  tracer,
		metrics: metrics,
		logger:  logger,
	}
}

// HandleWithTracing wraps a handler function with tracing and metrics
func (h *TracingHandler) HandleWithTracing(ctx context.Context, method string, handler func(context.Context) error) error {
	// Start span
	ctx, span := h.tracer(ctx, fmt.Sprintf("websocket.%s", method))
	defer span.End()

	// Record start time for metrics
	start := time.Now()

	// Add span attributes
	span.SetAttribute("websocket.method", method)
	if connectionID := ctx.Value("connection_id"); connectionID != nil {
		span.SetAttribute("websocket.connection_id", connectionID)
	}
	if tenantID := ctx.Value("tenant_id"); tenantID != nil {
		span.SetAttribute("tenant_id", tenantID)
	}
	if agentID := ctx.Value("agent_id"); agentID != nil {
		span.SetAttribute("agent_id", agentID)
	}

	// Execute handler
	err := handler(ctx)

	// Record duration
	duration := time.Since(start)

	// Update span based on result
	if err != nil {
		span.RecordError(err)
		span.SetStatus(1, err.Error()) // 1 = error status

		// Log error
		h.logger.Error("WebSocket handler error", map[string]interface{}{
			"method":   method,
			"error":    err.Error(),
			"duration": duration.Milliseconds(),
		})

		// Record error metrics
		h.recordMetrics(method, false, duration)
	} else {
		span.SetStatus(0, "") // 0 = ok status

		// Record success metrics
		h.recordMetrics(method, true, duration)
	}

	return err
}

// HandleMessageWithTracing wraps message handlers with tracing
func (h *TracingHandler) HandleMessageWithTracing(ctx context.Context, msg *ws.Message, handler func(context.Context, *ws.Message) (*ws.Message, error)) (*ws.Message, error) {
	method := msg.Method
	if method == "" {
		method = "unknown"
	}

	// Start span
	ctx, span := h.tracer(ctx, fmt.Sprintf("websocket.message.%s", method))
	defer span.End()

	// Record start time
	start := time.Now()

	// Add span attributes
	span.SetAttribute("websocket.method", method)
	span.SetAttribute("websocket.message_id", msg.ID)
	span.SetAttribute("websocket.message_type", int(msg.Type))

	// Add connection context
	if connectionID := ctx.Value("connection_id"); connectionID != nil {
		span.SetAttribute("websocket.connection_id", connectionID)
	}
	if tenantID := ctx.Value("tenant_id"); tenantID != nil {
		span.SetAttribute("tenant_id", tenantID)
	}
	if agentID := ctx.Value("agent_id"); agentID != nil {
		span.SetAttribute("agent_id", agentID)
	}

	// Execute handler
	response, err := handler(ctx, msg)

	// Record duration
	duration := time.Since(start)

	// Update span based on result
	if err != nil {
		span.RecordError(err)
		span.SetStatus(1, err.Error()) // 1 = error status

		// Log error
		h.logger.Error("WebSocket message handler error", map[string]interface{}{
			"method":     method,
			"message_id": msg.ID,
			"error":      err.Error(),
			"duration":   duration.Milliseconds(),
		})

		// Record error metrics
		h.recordMessageMetrics(method, false, duration)
	} else {
		span.SetStatus(0, "") // 0 = ok status

		// Record success metrics
		h.recordMessageMetrics(method, true, duration)
	}

	return response, err
}

// HandleNotificationWithTracing wraps notification handlers with tracing
func (h *TracingHandler) HandleNotificationWithTracing(ctx context.Context, method string, params interface{}, handler func(context.Context, interface{}) error) error {
	// Start span
	ctx, span := h.tracer(ctx, fmt.Sprintf("websocket.notification.%s", method))
	defer span.End()

	// Record start time
	start := time.Now()

	// Add span attributes
	span.SetAttribute("websocket.notification_type", method)

	// Execute handler
	err := handler(ctx, params)

	// Record duration
	duration := time.Since(start)

	// Update span based on result
	if err != nil {
		span.RecordError(err)
		span.SetStatus(1, err.Error()) // 1 = error status

		// Record error metrics
		h.recordNotificationMetrics(method, false, duration)
	} else {
		span.SetStatus(0, "") // 0 = ok status

		// Record success metrics
		h.recordNotificationMetrics(method, true, duration)
	}

	return err
}

// recordMetrics records handler metrics
func (h *TracingHandler) recordMetrics(method string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "error"
	}

	// Record counter
	h.metrics.IncrementCounterWithLabels("websocket_handler_total", 1, map[string]string{
		"method": method,
		"status": status,
	})

	// Record histogram
	h.metrics.RecordHistogram("websocket_handler_duration_seconds", duration.Seconds(), map[string]string{
		"method": method,
	})
}

// recordMessageMetrics records message handler metrics
func (h *TracingHandler) recordMessageMetrics(method string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "error"
	}

	// Record counter
	h.metrics.IncrementCounterWithLabels("websocket_message_handled_total", 1, map[string]string{
		"method": method,
		"status": status,
	})

	// Record histogram
	h.metrics.RecordHistogram("websocket_message_duration_seconds", duration.Seconds(), map[string]string{
		"method": method,
	})
}

// recordNotificationMetrics records notification metrics
func (h *TracingHandler) recordNotificationMetrics(method string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "error"
	}

	// Record counter
	h.metrics.IncrementCounterWithLabels("websocket_notification_total", 1, map[string]string{
		"type":   method,
		"status": status,
	})

	// Record histogram
	h.metrics.RecordHistogram("websocket_notification_duration_seconds", duration.Seconds(), map[string]string{
		"type": method,
	})
}

// WrapHandler wraps a simple handler function with tracing
func (h *TracingHandler) WrapHandler(method string, handler func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		return h.HandleWithTracing(ctx, method, handler)
	}
}

// WrapMessageHandler wraps a message handler function with tracing
func (h *TracingHandler) WrapMessageHandler(handler func(context.Context, *ws.Message) (*ws.Message, error)) func(context.Context, *ws.Message) (*ws.Message, error) {
	return func(ctx context.Context, msg *ws.Message) (*ws.Message, error) {
		return h.HandleMessageWithTracing(ctx, msg, handler)
	}
}

// CreateRequestContext creates a context with request metadata
func CreateRequestContext(ctx context.Context, connectionID string, tenantID, agentID *uuid.UUID) context.Context {
	ctx = context.WithValue(ctx, contextKeyConnectionID, connectionID)

	if tenantID != nil {
		ctx = context.WithValue(ctx, contextKeyTenantID, tenantID.String())
	}

	if agentID != nil {
		ctx = context.WithValue(ctx, contextKeyAgentID, agentID.String())
	}

	// Add correlation ID
	correlationID := uuid.New().String()
	ctx = context.WithValue(ctx, contextKey("correlation_id"), correlationID)

	return ctx
}

// ExtractTraceContext extracts trace context from WebSocket message headers
func ExtractTraceContext(ctx context.Context, msg *ws.Message) context.Context {
	// In a real implementation, this would extract W3C trace context headers
	// from the message metadata if available

	// For now, just return the context as-is
	return ctx
}
