// Package tracing provides distributed tracing capabilities using OpenTelemetry
package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	// Service name for tracing
	serviceName = "edge-mcp"

	// Tracer name
	tracerName = "github.com/developer-mesh/developer-mesh/apps/edge-mcp"

	// Attribute keys
	AttrToolName        = "tool.name"
	AttrSessionID       = "session.id"
	AttrTenantID        = "tenant.id"
	AttrRequestID       = "request.id"
	AttrCacheKey        = "cache.key"
	AttrCacheHit        = "cache.hit"
	AttrCorePlatformURL = "core_platform.url"
	AttrHTTPMethod      = "http.method"
	AttrHTTPStatus      = "http.status_code"
	AttrErrorType       = "error.type"
	AttrToolCategory    = "tool.category"
)

// Config holds tracing configuration
type Config struct {
	Enabled        bool
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string        // OTLP endpoint (e.g., "localhost:4317" for gRPC)
	OTLPInsecure   bool          // Whether to use insecure connection
	SamplingRate   float64       // Sampling rate (0.0 to 1.0)
	ExportTimeout  time.Duration // Timeout for exporting traces
	JaegerEndpoint string        // Optional Jaeger endpoint (deprecated, use OTLP)
	ZipkinEndpoint string        // Optional Zipkin endpoint
}

// DefaultConfig returns default tracing configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:        false,
		ServiceName:    serviceName,
		ServiceVersion: "1.0.0",
		Environment:    "development",
		OTLPEndpoint:   "localhost:4317",
		OTLPInsecure:   true,
		SamplingRate:   1.0, // Sample all traces by default
		ExportTimeout:  30 * time.Second,
	}
}

// TracerProvider manages OpenTelemetry tracing
type TracerProvider struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
	config   *Config
}

// NewTracerProvider creates a new tracer provider
func NewTracerProvider(config *Config) (*TracerProvider, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if !config.Enabled {
		// Return a noop provider
		return &TracerProvider{
			provider: sdktrace.NewTracerProvider(),
			tracer:   otel.Tracer(tracerName),
			config:   config,
		}, nil
	}

	// Create resource with service information
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
			attribute.String("environment", config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter based on configuration
	var spanExporter sdktrace.SpanExporter

	// Priority: Zipkin > OTLP (if both are configured, use Zipkin)
	if config.ZipkinEndpoint != "" {
		spanExporter, err = zipkin.New(config.ZipkinEndpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to create Zipkin exporter: %w", err)
		}
	} else if config.OTLPEndpoint != "" {
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(config.OTLPEndpoint),
			otlptracegrpc.WithTimeout(config.ExportTimeout),
		}
		if config.OTLPInsecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}

		spanExporter, err = otlptracegrpc.New(context.Background(), opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
	}

	// Create sampler based on sampling rate
	var sampler sdktrace.Sampler
	if config.SamplingRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if config.SamplingRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(config.SamplingRate)
	}

	// Create tracer provider
	var opts []sdktrace.TracerProviderOption
	opts = append(opts,
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	if spanExporter != nil {
		opts = append(opts, sdktrace.WithBatcher(spanExporter))
	}

	provider := sdktrace.NewTracerProvider(opts...)

	// Set global tracer provider
	otel.SetTracerProvider(provider)

	// Set global propagator to propagate trace context
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &TracerProvider{
		provider: provider,
		tracer:   provider.Tracer(tracerName),
		config:   config,
	}, nil
}

// Shutdown gracefully shuts down the tracer provider
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if tp.provider == nil {
		return nil
	}
	return tp.provider.Shutdown(ctx)
}

// Tracer returns the tracer
func (tp *TracerProvider) Tracer() trace.Tracer {
	return tp.tracer
}

// IsEnabled returns whether tracing is enabled
func (tp *TracerProvider) IsEnabled() bool {
	return tp.config.Enabled
}

// StartSpan starts a new span with the given name and options
func (tp *TracerProvider) StartSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if !tp.config.Enabled {
		return ctx, trace.SpanFromContext(ctx)
	}
	return tp.tracer.Start(ctx, spanName, opts...)
}

// SpanFromContext returns the current span from the context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// AddEvent adds an event to the current span
func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetAttributes sets attributes on the current span
func SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// RecordError records an error on the current span
func RecordError(ctx context.Context, err error, opts ...trace.EventOption) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err, opts...)
	span.SetStatus(codes.Error, err.Error())
}

// SetStatus sets the status of the current span
func SetStatus(ctx context.Context, code codes.Code, description string) {
	span := trace.SpanFromContext(ctx)
	span.SetStatus(code, description)
}

// SpanHelper provides convenient methods for span management
type SpanHelper struct {
	tp *TracerProvider
}

// NewSpanHelper creates a new span helper
func NewSpanHelper(tp *TracerProvider) *SpanHelper {
	return &SpanHelper{tp: tp}
}

// StartToolExecutionSpan starts a span for tool execution
func (sh *SpanHelper) StartToolExecutionSpan(ctx context.Context, toolName string, sessionID string, tenantID string) (context.Context, trace.Span) {
	if sh == nil || sh.tp == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	ctx, span := sh.tp.StartSpan(ctx, fmt.Sprintf("tool.execute.%s", toolName),
		trace.WithSpanKind(trace.SpanKindServer),
	)

	span.SetAttributes(
		attribute.String(AttrToolName, toolName),
		attribute.String(AttrSessionID, sessionID),
		attribute.String(AttrTenantID, tenantID),
	)

	return ctx, span
}

// StartCorePlatformCallSpan starts a span for Core Platform API call
func (sh *SpanHelper) StartCorePlatformCallSpan(ctx context.Context, method string, url string) (context.Context, trace.Span) {
	if sh == nil || sh.tp == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	ctx, span := sh.tp.StartSpan(ctx, fmt.Sprintf("core_platform.%s", method),
		trace.WithSpanKind(trace.SpanKindClient),
	)

	span.SetAttributes(
		attribute.String(AttrHTTPMethod, method),
		attribute.String(AttrCorePlatformURL, url),
	)

	return ctx, span
}

// StartCacheOperationSpan starts a span for cache operation
func (sh *SpanHelper) StartCacheOperationSpan(ctx context.Context, operation string, key string) (context.Context, trace.Span) {
	if sh == nil || sh.tp == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	ctx, span := sh.tp.StartSpan(ctx, fmt.Sprintf("cache.%s", operation),
		trace.WithSpanKind(trace.SpanKindInternal),
	)

	span.SetAttributes(
		attribute.String(AttrCacheKey, key),
	)

	return ctx, span
}

// RecordCacheHit records cache hit/miss
func (sh *SpanHelper) RecordCacheHit(ctx context.Context, hit bool) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.Bool(AttrCacheHit, hit))
}

// RecordHTTPStatus records HTTP status code
func (sh *SpanHelper) RecordHTTPStatus(ctx context.Context, statusCode int) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.Int(AttrHTTPStatus, statusCode))

	// Set span status based on HTTP status
	if statusCode >= 400 {
		if statusCode >= 500 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
		} else {
			// Client errors are not necessarily span errors
			span.SetStatus(codes.Unset, "")
		}
	}
}
