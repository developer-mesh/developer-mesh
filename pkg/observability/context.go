package observability

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// Context keys for observability
type contextKey string

const (
	correlationIDKey contextKey = "correlation_id"
	causationIDKey   contextKey = "causation_id"
)

// GetCorrelationID gets the correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if v, ok := ctx.Value(correlationIDKey).(string); ok {
		return v
	}
	if v, ok := ctx.Value("correlation_id").(string); ok {
		return v
	}
	return ""
}

// GetCausationID gets the causation ID from context
func GetCausationID(ctx context.Context) string {
	if v, ok := ctx.Value(causationIDKey).(string); ok {
		return v
	}
	if v, ok := ctx.Value("causation_id").(string); ok {
		return v
	}
	return ""
}

// WithCorrelationID adds correlation ID to context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

// WithCausationID adds causation ID to context
func WithCausationID(ctx context.Context, causationID string) context.Context {
	return context.WithValue(ctx, causationIDKey, causationID)
}

// Context storage keys for additional metadata
const (
	tenantIDKey    contextKey = "tenant_id"
	userIDKey      contextKey = "user_id"
	requestIDKey   contextKey = "request_id"
	sessionIDKey   contextKey = "session_id"
	spanContextKey contextKey = "span_context"
	operationKey   contextKey = "operation"
)

// WithTenantID adds tenant ID to context
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// GetTenantID gets the tenant ID from context
func GetTenantID(ctx context.Context) string {
	if v, ok := ctx.Value(tenantIDKey).(string); ok {
		return v
	}
	if v, ok := ctx.Value("tenant_id").(string); ok {
		return v
	}
	return ""
}

// WithUserID adds user ID to context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// GetUserID gets the user ID from context
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(userIDKey).(string); ok {
		return v
	}
	if v, ok := ctx.Value("user_id").(string); ok {
		return v
	}
	return ""
}

// WithRequestID adds request ID to context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID gets the request ID from context
func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	if v, ok := ctx.Value("request_id").(string); ok {
		return v
	}
	return ""
}

// WithSessionID adds session ID to context
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

// GetSessionID gets the session ID from context
func GetSessionID(ctx context.Context) string {
	if v, ok := ctx.Value(sessionIDKey).(string); ok {
		return v
	}
	if v, ok := ctx.Value("session_id").(string); ok {
		return v
	}
	return ""
}

// WithOperation adds operation name to context
func WithOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, operationKey, operation)
}

// GetOperation gets the operation name from context
func GetOperation(ctx context.Context) string {
	if v, ok := ctx.Value(operationKey).(string); ok {
		return v
	}
	if v, ok := ctx.Value("operation").(string); ok {
		return v
	}
	return ""
}

// ExtractMetadata extracts all observability metadata from context
func ExtractMetadata(ctx context.Context) map[string]string {
	metadata := make(map[string]string)

	if correlationID := GetCorrelationID(ctx); correlationID != "" {
		metadata["correlation_id"] = correlationID
	}
	if causationID := GetCausationID(ctx); causationID != "" {
		metadata["causation_id"] = causationID
	}
	if tenantID := GetTenantID(ctx); tenantID != "" {
		metadata["tenant_id"] = tenantID
	}
	if userID := GetUserID(ctx); userID != "" {
		metadata["user_id"] = userID
	}
	if requestID := GetRequestID(ctx); requestID != "" {
		metadata["request_id"] = requestID
	}
	if sessionID := GetSessionID(ctx); sessionID != "" {
		metadata["session_id"] = sessionID
	}
	if operation := GetOperation(ctx); operation != "" {
		metadata["operation"] = operation
	}

	return metadata
}

// InjectMetadata injects metadata map into context
func InjectMetadata(ctx context.Context, metadata map[string]string) context.Context {
	for key, value := range metadata {
		switch key {
		case "correlation_id":
			ctx = WithCorrelationID(ctx, value)
		case "causation_id":
			ctx = WithCausationID(ctx, value)
		case "tenant_id":
			ctx = WithTenantID(ctx, value)
		case "user_id":
			ctx = WithUserID(ctx, value)
		case "request_id":
			ctx = WithRequestID(ctx, value)
		case "session_id":
			ctx = WithSessionID(ctx, value)
		case "operation":
			ctx = WithOperation(ctx, value)
		}
	}
	return ctx
}

// GenerateRequestID generates a unique request ID
func GenerateRequestID() string {
	return uuid.New().String()
}

// LoggerFromContext creates a logger with fields from context
func LoggerFromContext(ctx context.Context, baseLogger Logger) Logger {
	fields := make(map[string]interface{})

	if requestID := GetRequestID(ctx); requestID != "" {
		fields["request_id"] = requestID
	}

	if tenantID := GetTenantID(ctx); tenantID != "" {
		fields["tenant_id"] = tenantID
	}

	if sessionID := GetSessionID(ctx); sessionID != "" {
		fields["session_id"] = sessionID
	}

	if correlationID := GetCorrelationID(ctx); correlationID != "" {
		fields["correlation_id"] = correlationID
	}

	if operation := GetOperation(ctx); operation != "" {
		fields["operation"] = operation
	}

	if len(fields) > 0 {
		return baseLogger.With(fields)
	}

	return baseLogger
}

// SampledLogger wraps a logger to implement log sampling for high volume scenarios
type SampledLogger struct {
	logger     Logger
	sampleRate float64        // Percentage of logs to keep (0.0 to 1.0)
	counter    *atomic.Uint64 // Thread-safe counter for sampling decision
}

// NewSampledLogger creates a new sampled logger
// sampleRate: 1.0 means log everything, 0.1 means log 10% of messages
func NewSampledLogger(logger Logger, sampleRate float64) *SampledLogger {
	if sampleRate < 0.0 {
		sampleRate = 0.0
	}
	if sampleRate > 1.0 {
		sampleRate = 1.0
	}

	return &SampledLogger{
		logger:     logger,
		sampleRate: sampleRate,
		counter:    &atomic.Uint64{},
	}
}

// shouldLog determines if this log message should be emitted
func (l *SampledLogger) shouldLog(level LogLevel) bool {
	// Always log errors and fatals regardless of sampling
	if level == LogLevelError || level == LogLevelFatal {
		return true
	}

	// If sample rate is 1.0, always log
	if l.sampleRate >= 1.0 {
		return true
	}

	// Use deterministic sampling based on counter
	count := l.counter.Add(1)
	threshold := uint64(1.0 / l.sampleRate)
	return count%threshold == 0
}

// Debug logs a debug message with sampling
func (l *SampledLogger) Debug(msg string, fields map[string]interface{}) {
	if l.shouldLog(LogLevelDebug) {
		l.logger.Debug(msg, fields)
	}
}

// Info logs an info message with sampling
func (l *SampledLogger) Info(msg string, fields map[string]interface{}) {
	if l.shouldLog(LogLevelInfo) {
		l.logger.Info(msg, fields)
	}
}

// Warn logs a warning message with sampling
func (l *SampledLogger) Warn(msg string, fields map[string]interface{}) {
	if l.shouldLog(LogLevelWarn) {
		l.logger.Warn(msg, fields)
	}
}

// Error logs an error message (always, no sampling)
func (l *SampledLogger) Error(msg string, fields map[string]interface{}) {
	l.logger.Error(msg, fields)
}

// Fatal logs a fatal message (always, no sampling)
func (l *SampledLogger) Fatal(msg string, fields map[string]interface{}) {
	l.logger.Fatal(msg, fields)
}

// Debugf logs a formatted debug message with sampling
func (l *SampledLogger) Debugf(format string, args ...interface{}) {
	if l.shouldLog(LogLevelDebug) {
		l.logger.Debugf(format, args...)
	}
}

// Infof logs a formatted info message with sampling
func (l *SampledLogger) Infof(format string, args ...interface{}) {
	if l.shouldLog(LogLevelInfo) {
		l.logger.Infof(format, args...)
	}
}

// Warnf logs a formatted warning message with sampling
func (l *SampledLogger) Warnf(format string, args ...interface{}) {
	if l.shouldLog(LogLevelWarn) {
		l.logger.Warnf(format, args...)
	}
}

// Errorf logs a formatted error message (always, no sampling)
func (l *SampledLogger) Errorf(format string, args ...interface{}) {
	l.logger.Errorf(format, args...)
}

// Fatalf logs a formatted fatal message (always, no sampling)
func (l *SampledLogger) Fatalf(format string, args ...interface{}) {
	l.logger.Fatalf(format, args...)
}

// WithPrefix returns a new sampled logger with the given prefix
func (l *SampledLogger) WithPrefix(prefix string) Logger {
	return &SampledLogger{
		logger:     l.logger.WithPrefix(prefix),
		sampleRate: l.sampleRate,
		counter:    l.counter,
	}
}

// With returns a new sampled logger with the given fields
func (l *SampledLogger) With(fields map[string]interface{}) Logger {
	return &SampledLogger{
		logger:     l.logger.With(fields),
		sampleRate: l.sampleRate,
		counter:    l.counter,
	}
}

// PerformanceLogger wraps a logger to add performance metrics
type PerformanceLogger struct {
	logger Logger
}

// NewPerformanceLogger creates a new performance logger
func NewPerformanceLogger(logger Logger) *PerformanceLogger {
	return &PerformanceLogger{
		logger: logger,
	}
}

// LogWithDuration logs a message with duration metrics
func (l *PerformanceLogger) LogWithDuration(level LogLevel, msg string, duration time.Duration, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}

	// Add performance metrics
	fields["duration_ms"] = duration.Milliseconds()
	fields["duration_us"] = duration.Microseconds()

	switch level {
	case LogLevelDebug:
		l.logger.Debug(msg, fields)
	case LogLevelInfo:
		l.logger.Info(msg, fields)
	case LogLevelWarn:
		l.logger.Warn(msg, fields)
	case LogLevelError:
		l.logger.Error(msg, fields)
	case LogLevelFatal:
		l.logger.Fatal(msg, fields)
	}
}

// StartTimer returns a function that logs the duration when called
func (l *PerformanceLogger) StartTimer(msg string, level LogLevel) func(fields map[string]interface{}) {
	start := time.Now()
	return func(fields map[string]interface{}) {
		duration := time.Since(start)
		l.LogWithDuration(level, msg, duration, fields)
	}
}

// Delegate all standard logging methods to the wrapped logger
func (l *PerformanceLogger) Debug(msg string, fields map[string]interface{}) {
	l.logger.Debug(msg, fields)
}

func (l *PerformanceLogger) Info(msg string, fields map[string]interface{}) {
	l.logger.Info(msg, fields)
}

func (l *PerformanceLogger) Warn(msg string, fields map[string]interface{}) {
	l.logger.Warn(msg, fields)
}

func (l *PerformanceLogger) Error(msg string, fields map[string]interface{}) {
	l.logger.Error(msg, fields)
}

func (l *PerformanceLogger) Fatal(msg string, fields map[string]interface{}) {
	l.logger.Fatal(msg, fields)
}

func (l *PerformanceLogger) Debugf(format string, args ...interface{}) {
	l.logger.Debugf(format, args...)
}

func (l *PerformanceLogger) Infof(format string, args ...interface{}) {
	l.logger.Infof(format, args...)
}

func (l *PerformanceLogger) Warnf(format string, args ...interface{}) {
	l.logger.Warnf(format, args...)
}

func (l *PerformanceLogger) Errorf(format string, args ...interface{}) {
	l.logger.Errorf(format, args...)
}

func (l *PerformanceLogger) Fatalf(format string, args ...interface{}) {
	l.logger.Fatalf(format, args...)
}

func (l *PerformanceLogger) WithPrefix(prefix string) Logger {
	return &PerformanceLogger{
		logger: l.logger.WithPrefix(prefix),
	}
}

func (l *PerformanceLogger) With(fields map[string]interface{}) Logger {
	return &PerformanceLogger{
		logger: l.logger.With(fields),
	}
}
