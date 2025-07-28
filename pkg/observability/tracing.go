// Package observability provides unified observability functionality for the MCP system.
// It consolidates logging, metrics, and tracing into a cohesive interface.
package observability

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// otelSpanWrapper wraps an OpenTelemetry span to implement the Span interface
type otelSpanWrapper struct {
	span trace.Span
}

// End implements Span.End
func (o *otelSpanWrapper) End() {
	o.span.End()
}

// SetStatus implements Span.SetStatus
func (o *otelSpanWrapper) SetStatus(code int, description string) {
	// Convert int code to codes.Code
	var statusCode codes.Code
	switch code {
	case 1:
		statusCode = codes.Ok
	case 2:
		statusCode = codes.Error
	default:
		statusCode = codes.Unset
	}
	o.span.SetStatus(statusCode, description)
}

// SetAttribute implements Span.SetAttribute
func (o *otelSpanWrapper) SetAttribute(key string, value interface{}) {
	// Convert the value to an appropriate attribute.KeyValue based on type
	switch v := value.(type) {
	case string:
		o.span.SetAttributes(attribute.String(key, v))
	case int:
		o.span.SetAttributes(attribute.Int(key, v))
	case int64:
		o.span.SetAttributes(attribute.Int64(key, v))
	case float64:
		o.span.SetAttributes(attribute.Float64(key, v))
	case bool:
		o.span.SetAttributes(attribute.Bool(key, v))
	case []attribute.KeyValue:
		o.span.SetAttributes(v...)
	default:
		o.span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", v)))
	}
}

// AddEvent implements Span.AddEvent
func (o *otelSpanWrapper) AddEvent(name string, attributes map[string]interface{}) {
	// Convert attributes map to attribute.KeyValue slice
	attrs := make([]attribute.KeyValue, 0, len(attributes))
	for k, v := range attributes {
		switch val := v.(type) {
		case string:
			attrs = append(attrs, attribute.String(k, val))
		case int:
			attrs = append(attrs, attribute.Int(k, val))
		case int64:
			attrs = append(attrs, attribute.Int64(k, val))
		case float64:
			attrs = append(attrs, attribute.Float64(k, val))
		case bool:
			attrs = append(attrs, attribute.Bool(k, val))
		default:
			attrs = append(attrs, attribute.String(k, fmt.Sprintf("%v", val)))
		}
	}
	o.span.AddEvent(name, trace.WithAttributes(attrs...))
}

// RecordError implements Span.RecordError
func (o *otelSpanWrapper) RecordError(err error) {
	o.span.RecordError(err)
}

// SpanContext implements Span.SpanContext
func (o *otelSpanWrapper) SpanContext() trace.SpanContext {
	return o.span.SpanContext()
}

// TracerProvider implements Span.TracerProvider
func (o *otelSpanWrapper) TracerProvider() trace.TracerProvider {
	return o.span.TracerProvider()
}

// Constants for span attribute keys
const (
	// ContextOperationAttributeKey is the attribute key for context operations
	ContextOperationAttributeKey = attribute.Key("context.operation")

	// ContextModelAttributeKey is the attribute key for context model IDs
	ContextModelAttributeKey = attribute.Key("context.model_id")

	// ContextTokensAttributeKey is the attribute key for context token counts
	ContextTokensAttributeKey = attribute.Key("context.tokens")

	// VectorOperationAttributeKey is the attribute key for vector operations
	VectorOperationAttributeKey = attribute.Key("vector.operation")

	// ToolNameAttributeKey is the attribute key for tool names
	ToolNameAttributeKey = attribute.Key("tool.name")

	// ToolActionAttributeKey is the attribute key for tool actions
	ToolActionAttributeKey = attribute.Key("tool.action")
)

// InitTracing initializes OpenTelemetry tracing
func InitTracing(cfg TracingConfig) (func(), error) {
	if !cfg.Enabled {
		log.Println("Tracing is disabled")
		return func() {}, nil
	}

	if cfg.ServiceName == "" {
		cfg.ServiceName = "mcp-server"
	}

	if cfg.Environment == "" {
		cfg.Environment = "development"
	}

	if cfg.Endpoint == "" {
		cfg.Endpoint = "localhost:4317"
	}

	ctx := context.Background()

	// Create OTLP exporter with endpoint
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create trace provider
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	// Set global trace provider
	otel.SetTracerProvider(tracerProvider)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Create tracer
	tracer := otel.Tracer(cfg.ServiceName)

	// Store tracer in a package-level variable
	SetTracer(tracer)

	log.Printf("Tracing initialized with service name: %s, environment: %s", cfg.ServiceName, cfg.Environment)

	// Return a cleanup function
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracerProvider.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}, nil
}

// Package level variables for tracing
var (
	globalTracer     trace.Tracer
	globalTracerInit bool
)

// SetTracer sets the global tracer
func SetTracer(t trace.Tracer) {
	globalTracer = t
	globalTracerInit = true
}

// GetTracer returns the global tracer
func GetTracer() trace.Tracer {
	if !globalTracerInit {
		log.Println("Warning: Tracing not initialized, operations will not be traced")
		return noop.NewTracerProvider().Tracer("")
	}
	return globalTracer
}

// StartSpan starts a new span and returns the wrapped span and context
func StartSpan(ctx context.Context, name string) (context.Context, Span) {
	ctx, otelSpan := GetTracer().Start(ctx, name)
	return ctx, &otelSpanWrapper{span: otelSpan}
}

// AddSpanEvent adds an event to the current span
func AddSpanEvent(ctx context.Context, name string, attributes ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attributes...))
}

// SetSpanStatus sets the status of the current span
func SetSpanStatus(ctx context.Context, err error) {
	if err == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
}

// AddSpanAttributes adds attributes to the current span
func AddSpanAttributes(ctx context.Context, attributes ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attributes...)
}

// RecordError records an error to the current span
func RecordError(ctx context.Context, err error, options ...trace.EventOption) {
	if err == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	span.RecordError(err, options...)
}

// TraceContext wraps span operations for a specific context
func TraceContext(ctx context.Context, operation string, modelID string) (context.Context, Span) {
	ctx, span := StartSpan(ctx, "context."+operation)
	otelSpan, ok := span.(*otelSpanWrapper)
	if ok {
		otelSpan.span.SetAttributes(
			ContextOperationAttributeKey.String(operation),
			ContextModelAttributeKey.String(modelID),
		)
	}
	return ctx, span
}

// TraceVector wraps span operations for vector operations
func TraceVector(ctx context.Context, operation string) (context.Context, Span) {
	ctx, span := StartSpan(ctx, "vector."+operation)
	otelSpan, ok := span.(*otelSpanWrapper)
	if ok {
		otelSpan.span.SetAttributes(
			VectorOperationAttributeKey.String(operation),
		)
	}
	return ctx, span
}

// TraceTool wraps span operations for tool operations
func TraceTool(ctx context.Context, tool string, action string) (context.Context, Span) {
	ctx, span := StartSpan(ctx, "tool."+action)
	otelSpan, ok := span.(*otelSpanWrapper)
	if ok {
		otelSpan.span.SetAttributes(
			ToolNameAttributeKey.String(tool),
			ToolActionAttributeKey.String(action),
		)
	}
	return ctx, span
}

// tracerAdapter wraps the global tracer to implement the Tracer interface
type tracerAdapter struct{}

// NewTracer creates a new tracer adapter
func NewTracer() Tracer {
	return &tracerAdapter{}
}

// StartSpan implements the Tracer interface
func (t *tracerAdapter) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	return StartSpan(ctx, name)
}
