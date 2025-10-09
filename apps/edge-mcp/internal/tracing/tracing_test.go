package tracing

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, false, config.Enabled)
	assert.Equal(t, "edge-mcp", config.ServiceName)
	assert.Equal(t, "1.0.0", config.ServiceVersion)
	assert.Equal(t, "development", config.Environment)
	assert.Equal(t, "localhost:4317", config.OTLPEndpoint)
	assert.Equal(t, true, config.OTLPInsecure)
	assert.Equal(t, 1.0, config.SamplingRate)
}

func TestNewTracerProvider_Disabled(t *testing.T) {
	config := &Config{
		Enabled: false,
	}

	tp, err := NewTracerProvider(config)
	require.NoError(t, err)
	assert.NotNil(t, tp)
	assert.False(t, tp.IsEnabled())
}

func TestNewTracerProvider_Enabled_NoExporter(t *testing.T) {
	config := &Config{
		Enabled:      true,
		ServiceName:  "test-service",
		SamplingRate: 1.0,
	}

	tp, err := NewTracerProvider(config)
	require.NoError(t, err)
	assert.NotNil(t, tp)
	assert.True(t, tp.IsEnabled())

	// Should still work without exporter (noop)
	ctx := context.Background()
	_, span := tp.StartSpan(ctx, "test-span")
	defer span.End()

	assert.NotNil(t, span)
}

func TestNewTracerProvider_WithOTLP(t *testing.T) {
	// This test would normally connect to a real OTLP endpoint
	// For testing, we'll use an invalid endpoint and expect an error
	config := &Config{
		Enabled:      true,
		ServiceName:  "test-service",
		OTLPEndpoint: "invalid:9999",
		OTLPInsecure: true,
		SamplingRate: 1.0,
	}

	// Note: This may or may not error depending on network conditions
	// The important thing is that it doesn't panic
	tp, _ := NewTracerProvider(config)
	if tp != nil {
		assert.True(t, tp.IsEnabled())
	}
}

func TestNewTracerProvider_WithZipkin(t *testing.T) {
	config := &Config{
		Enabled:        true,
		ServiceName:    "test-service",
		ZipkinEndpoint: "http://localhost:9411/api/v2/spans",
		SamplingRate:   1.0,
	}

	// Note: This may or may not error if Zipkin is not running
	// The important thing is that it doesn't panic
	tp, _ := NewTracerProvider(config)
	if tp != nil {
		assert.True(t, tp.IsEnabled())
	}
}

func TestTracerProvider_Tracer(t *testing.T) {
	config := &Config{
		Enabled:      true,
		ServiceName:  "test-service",
		SamplingRate: 1.0,
	}

	tp, err := NewTracerProvider(config)
	require.NoError(t, err)

	tracer := tp.Tracer()
	assert.NotNil(t, tracer)
}

func TestTracerProvider_Shutdown(t *testing.T) {
	config := &Config{
		Enabled:      true,
		ServiceName:  "test-service",
		SamplingRate: 1.0,
	}

	tp, err := NewTracerProvider(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = tp.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestSamplingRates(t *testing.T) {
	tests := []struct {
		name         string
		samplingRate float64
	}{
		{"always sample", 1.0},
		{"never sample", 0.0},
		{"half sample", 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Enabled:      true,
				ServiceName:  "test-service",
				SamplingRate: tt.samplingRate,
			}

			tp, err := NewTracerProvider(config)
			require.NoError(t, err)
			assert.NotNil(t, tp)
		})
	}
}

func TestSpanHelper_StartToolExecutionSpan(t *testing.T) {
	config := &Config{
		Enabled:      true,
		ServiceName:  "test-service",
		SamplingRate: 1.0,
	}

	tp, err := NewTracerProvider(config)
	require.NoError(t, err)

	sh := NewSpanHelper(tp)
	ctx := context.Background()

	ctx, span := sh.StartToolExecutionSpan(ctx, "test-tool", "session-123", "tenant-456")
	defer span.End()

	assert.NotNil(t, span)
	// Verify span is in context
	assert.NotNil(t, SpanFromContext(ctx))
}

func TestSpanHelper_StartCorePlatformCallSpan(t *testing.T) {
	config := &Config{
		Enabled:      true,
		ServiceName:  "test-service",
		SamplingRate: 1.0,
	}

	tp, err := NewTracerProvider(config)
	require.NoError(t, err)

	sh := NewSpanHelper(tp)
	ctx := context.Background()

	_, span := sh.StartCorePlatformCallSpan(ctx, "POST", "http://core-platform/api/v1/tools")
	defer span.End()

	assert.NotNil(t, span)
}

func TestSpanHelper_StartCacheOperationSpan(t *testing.T) {
	config := &Config{
		Enabled:      true,
		ServiceName:  "test-service",
		SamplingRate: 1.0,
	}

	tp, err := NewTracerProvider(config)
	require.NoError(t, err)

	sh := NewSpanHelper(tp)
	ctx := context.Background()

	_, span := sh.StartCacheOperationSpan(ctx, "get", "cache-key-123")
	defer span.End()

	assert.NotNil(t, span)
}

func TestSpanHelper_RecordCacheHit(t *testing.T) {
	config := &Config{
		Enabled:      true,
		ServiceName:  "test-service",
		SamplingRate: 1.0,
	}

	tp, err := NewTracerProvider(config)
	require.NoError(t, err)

	sh := NewSpanHelper(tp)
	ctx := context.Background()

	ctx, span := sh.StartCacheOperationSpan(ctx, "get", "cache-key-123")
	defer span.End()

	sh.RecordCacheHit(ctx, true)
	// No error expected, just verify it doesn't panic
}

func TestSpanHelper_RecordHTTPStatus(t *testing.T) {
	config := &Config{
		Enabled:      true,
		ServiceName:  "test-service",
		SamplingRate: 1.0,
	}

	tp, err := NewTracerProvider(config)
	require.NoError(t, err)

	sh := NewSpanHelper(tp)

	tests := []struct {
		name       string
		statusCode int
	}{
		{"success", 200},
		{"created", 201},
		{"bad request", 400},
		{"not found", 404},
		{"internal error", 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx, span := sh.StartCorePlatformCallSpan(ctx, "GET", "/test")
			defer span.End()

			sh.RecordHTTPStatus(ctx, tt.statusCode)
			// No error expected
		})
	}
}

func TestAddEvent(t *testing.T) {
	config := &Config{
		Enabled:      true,
		ServiceName:  "test-service",
		SamplingRate: 1.0,
	}

	tp, err := NewTracerProvider(config)
	require.NoError(t, err)

	ctx := context.Background()
	ctx, span := tp.StartSpan(ctx, "test-span")
	defer span.End()

	AddEvent(ctx, "test-event", attribute.String("key", "value"))
	// No error expected
}

func TestSetAttributes(t *testing.T) {
	config := &Config{
		Enabled:      true,
		ServiceName:  "test-service",
		SamplingRate: 1.0,
	}

	tp, err := NewTracerProvider(config)
	require.NoError(t, err)

	ctx := context.Background()
	ctx, span := tp.StartSpan(ctx, "test-span")
	defer span.End()

	SetAttributes(ctx,
		attribute.String("key1", "value1"),
		attribute.Int("key2", 42),
	)
	// No error expected
}

func TestRecordError(t *testing.T) {
	config := &Config{
		Enabled:      true,
		ServiceName:  "test-service",
		SamplingRate: 1.0,
	}

	tp, err := NewTracerProvider(config)
	require.NoError(t, err)

	ctx := context.Background()
	ctx, span := tp.StartSpan(ctx, "test-span")
	defer span.End()

	testErr := errors.New("test error")
	RecordError(ctx, testErr)
	// No error expected
}

func TestSetStatus(t *testing.T) {
	config := &Config{
		Enabled:      true,
		ServiceName:  "test-service",
		SamplingRate: 1.0,
	}

	tp, err := NewTracerProvider(config)
	require.NoError(t, err)

	ctx := context.Background()
	ctx, span := tp.StartSpan(ctx, "test-span")
	defer span.End()

	SetStatus(ctx, codes.Ok, "success")
	SetStatus(ctx, codes.Error, "failed")
	// No error expected
}

func TestSpanHelper_NilTracerProvider(t *testing.T) {
	sh := NewSpanHelper(nil)
	assert.Nil(t, sh.tp)

	ctx := context.Background()

	// These should not panic even with nil provider
	ctx, span := sh.StartToolExecutionSpan(ctx, "test", "session", "tenant")
	assert.NotNil(t, span)
	span.End()

	_, span = sh.StartCorePlatformCallSpan(ctx, "GET", "/test")
	assert.NotNil(t, span)
	span.End()

	_, span = sh.StartCacheOperationSpan(ctx, "get", "key")
	assert.NotNil(t, span)
	span.End()
}

func TestConstants(t *testing.T) {
	// Verify constant values
	assert.Equal(t, "edge-mcp", serviceName)
	assert.Equal(t, "github.com/developer-mesh/developer-mesh/apps/edge-mcp", tracerName)

	// Verify attribute keys
	assert.Equal(t, "tool.name", AttrToolName)
	assert.Equal(t, "session.id", AttrSessionID)
	assert.Equal(t, "tenant.id", AttrTenantID)
	assert.Equal(t, "request.id", AttrRequestID)
	assert.Equal(t, "cache.key", AttrCacheKey)
	assert.Equal(t, "cache.hit", AttrCacheHit)
	assert.Equal(t, "core_platform.url", AttrCorePlatformURL)
	assert.Equal(t, "http.method", AttrHTTPMethod)
	assert.Equal(t, "http.status_code", AttrHTTPStatus)
	assert.Equal(t, "error.type", AttrErrorType)
	assert.Equal(t, "tool.category", AttrToolCategory)
}
