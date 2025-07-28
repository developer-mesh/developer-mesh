// Package observability provides unified observability functionality for the MCP system.
// It consolidates logging, metrics, and tracing into a cohesive interface.
package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Config holds the configuration for all observability components
type Config struct {
	Tracing TracingConfig `json:"tracing,omitempty"`
	Metrics MetricsConfig `json:"metrics,omitempty"`
	Logging LoggingConfig `json:"logging,omitempty"`
}

// TracingConfig holds the configuration for tracing
type TracingConfig struct {
	// Enabled indicates whether tracing is enabled
	Enabled     bool   `json:"enabled"`
	ServiceName string `json:"service_name,omitempty"`
	Environment string `json:"environment,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
}

// MetricsConfig holds the configuration for metrics
type MetricsConfig struct {
	// Enabled indicates whether metrics collection is enabled
	Enabled      bool          `json:"enabled" mapstructure:"enabled"`
	Type         string        `json:"type,omitempty" mapstructure:"type"`
	Endpoint     string        `json:"endpoint,omitempty" mapstructure:"endpoint"`
	Namespace    string        `json:"namespace,omitempty" mapstructure:"namespace"`
	PushGateway  string        `json:"push_gateway,omitempty" mapstructure:"push_gateway"`
	PushInterval time.Duration `json:"push_interval,omitempty" mapstructure:"push_interval"`
}

// LoggingConfig holds the configuration for logging
type LoggingConfig struct {
	// Level is the minimum log level to emit
	Level  string `json:"level,omitempty"`
	Format string `json:"format,omitempty"`
	// Output is the output destination (stdout, file)
	Output string `json:"output,omitempty"`
	// FilePath is the path to the log file if Output is "file"
	FilePath string `json:"file_path,omitempty"`
}

// LogLevel defines log message severity
type LogLevel string

// Log levels
const (
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
	LogLevelFatal LogLevel = "FATAL"
)

// Logger defines the interface for logging
type Logger interface {
	// Core logging methods with fields
	Debug(msg string, fields map[string]interface{})
	Info(msg string, fields map[string]interface{})
	Warn(msg string, fields map[string]interface{})
	Error(msg string, fields map[string]interface{})
	Fatal(msg string, fields map[string]interface{})

	// Formatted logging methods
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})

	// Context methods
	WithPrefix(prefix string) Logger
	With(fields map[string]interface{}) Logger
}

// MetricsClient defines the interface for metrics collection
type MetricsClient interface {
	// Core metrics recording methods
	RecordEvent(source, eventType string)
	RecordLatency(operation string, duration time.Duration)
	RecordCounter(name string, value float64, labels map[string]string)
	RecordGauge(name string, value float64, labels map[string]string)
	RecordHistogram(name string, value float64, labels map[string]string)
	RecordTimer(name string, duration time.Duration, labels map[string]string)

	// Operation-specific metrics
	RecordCacheOperation(operation string, success bool, durationSeconds float64)
	RecordOperation(component string, operation string, success bool, durationSeconds float64, labels map[string]string)
	RecordAPIOperation(api string, operation string, success bool, durationSeconds float64)
	RecordDatabaseOperation(operation string, success bool, durationSeconds float64)

	// Convenience methods
	StartTimer(name string, labels map[string]string) func()
	// IncrementCounter is the standard method for incrementing counters
	// For backward compatibility with internal/observability, this version doesn't require labels
	IncrementCounter(name string, value float64)
	// IncrementCounterWithLabels is the preferred method with labels support
	IncrementCounterWithLabels(name string, value float64, labels map[string]string)
	RecordDuration(name string, duration time.Duration)

	// Lifecycle management
	Close() error
}

// Span represents a trace span
type Span interface {
	End()
	SetAttribute(key string, value interface{})
	AddEvent(name string, attributes map[string]interface{})
	RecordError(err error)
	SetStatus(code int, description string)
	SpanContext() trace.SpanContext
	TracerProvider() trace.TracerProvider
}

// StartSpanFunc is a function that creates and starts a new span
type StartSpanFunc func(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, Span)

// Tracer defines the interface for distributed tracing
type Tracer interface {
	StartSpan(ctx context.Context, name string) (context.Context, Span)
}
