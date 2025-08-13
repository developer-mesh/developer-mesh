package websocket

import (
	"go.opentelemetry.io/otel/trace"
)

// NoOpSpan is a no-op implementation of observability.Span
type NoOpSpan struct{}

// End ends the span
func (s *NoOpSpan) End() {}

// SetStatus sets the status of the span
func (s *NoOpSpan) SetStatus(code int, message string) {}

// SetAttribute sets an attribute on the span
func (s *NoOpSpan) SetAttribute(key string, value interface{}) {}

// RecordError records an error on the span
func (s *NoOpSpan) RecordError(err error) {}

// AddEvent adds an event to the span
func (s *NoOpSpan) AddEvent(name string, attributes map[string]interface{}) {}

// SpanContext returns the span context
func (s *NoOpSpan) SpanContext() trace.SpanContext {
	return trace.SpanContext{}
}

// TracerProvider returns the tracer provider
func (s *NoOpSpan) TracerProvider() trace.TracerProvider {
	return nil
}
