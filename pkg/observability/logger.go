// Package observability provides unified logging, metrics, and tracing
// capabilities for the developer-mesh workspace. It follows a consistent
// approach to observability across all services and components.
package observability

import (
	"fmt"
	"log"
	"os"
	"time"

	commonLogging "github.com/developer-mesh/developer-mesh/pkg/common/logging"
)

// StandardLogger is a logger implementation that uses the standard log package
type StandardLogger struct {
	prefix        string
	level         LogLevel
	logger        *log.Logger
	contextFields map[string]interface{} // Stored fields for all logs from this logger
}

// NewStandardLogger creates a new StandardLogger with the given prefix
func NewStandardLogger(prefix string) Logger {
	// Create a logger that writes to stderr instead of stdout
	// This is crucial for MCP servers using stdio transport
	return &StandardLogger{
		prefix: prefix,
		level:  LogLevelInfo, // Default to INFO level
		logger: log.New(os.Stderr, "", 0),
	}
}

// WithLevel returns a new logger with the specified log level
func (l *StandardLogger) WithLevel(level LogLevel) *StandardLogger {
	return &StandardLogger{
		prefix:        l.prefix,
		level:         level,
		logger:        l.logger,
		contextFields: l.contextFields,
	}
}

// Debug logs a debug message
func (l *StandardLogger) Debug(msg string, fields map[string]interface{}) {
	if l.levelEnabled(LogLevelDebug) {
		l.log(LogLevelDebug, msg, fields)
	}
}

// Info logs an info message
func (l *StandardLogger) Info(msg string, fields map[string]interface{}) {
	if l.levelEnabled(LogLevelInfo) {
		l.log(LogLevelInfo, msg, fields)
	}
}

// Warn logs a warning message
func (l *StandardLogger) Warn(msg string, fields map[string]interface{}) {
	if l.levelEnabled(LogLevelWarn) {
		l.log(LogLevelWarn, msg, fields)
	}
}

// Error logs an error message
func (l *StandardLogger) Error(msg string, fields map[string]interface{}) {
	l.log(LogLevelError, msg, fields)
}

// Fatal logs a fatal message and exits
func (l *StandardLogger) Fatal(msg string, fields map[string]interface{}) {
	l.log(LogLevelFatal, msg, fields)
	os.Exit(1)
}

// WithPrefix returns a new logger with the given prefix
func (l *StandardLogger) WithPrefix(prefix string) Logger {
	// Preserve context fields when creating logger with new prefix
	return &StandardLogger{
		prefix:        prefix,
		level:         l.level,
		logger:        l.logger,
		contextFields: l.contextFields,
	}
}

// With returns a new logger with the given fields
func (l *StandardLogger) With(fields map[string]interface{}) Logger {
	// Merge existing context fields with new fields
	mergedFields := make(map[string]interface{})

	// Copy existing context fields
	if l.contextFields != nil {
		for k, v := range l.contextFields {
			mergedFields[k] = v
		}
	}

	// Add new fields (overwrites existing if duplicate keys)
	for k, v := range fields {
		mergedFields[k] = v
	}

	return &StandardLogger{
		prefix:        l.prefix,
		level:         l.level,
		logger:        l.logger,
		contextFields: mergedFields,
	}
}

// formatFields formats fields as a string
func (l *StandardLogger) formatFields(fields map[string]interface{}) string {
	if len(fields) == 0 {
		return ""
	}

	// Format the fields as key=value pairs
	result := ""
	for k, v := range fields {
		result += fmt.Sprintf(" %s=%v", k, v)
	}
	return result
}

// levelEnabled checks if the given log level is enabled
func (l *StandardLogger) levelEnabled(level LogLevel) bool {
	// Define log level hierarchy
	levelHierarchy := map[LogLevel]int{
		LogLevelDebug: 0,
		LogLevelInfo:  1,
		LogLevelWarn:  2,
		LogLevelError: 3,
		LogLevelFatal: 4,
	}

	// Check if the current level is equal to or greater than the minimum level
	return levelHierarchy[level] >= levelHierarchy[l.level]
}

// log logs a message with the given level
func (l *StandardLogger) log(level LogLevel, msg string, fields map[string]interface{}) {
	// Get current timestamp
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")

	// Create log prefix with timestamp, level, and logger prefix
	logPrefix := fmt.Sprintf("%s [%s] [%s]", timestamp, level, l.prefix)

	// Merge context fields with method fields
	mergedFields := make(map[string]interface{})

	// Add context fields first
	if l.contextFields != nil {
		for k, v := range l.contextFields {
			mergedFields[k] = v
		}
	}

	// Add method fields (overwrites context fields if duplicate keys)
	for k, v := range fields {
		mergedFields[k] = v
	}

	// Format the fields
	fieldsStr := l.formatFields(mergedFields)

	// Log the message using the logger instance (writes to stderr)
	l.logger.Printf("%s %s%s", logPrefix, msg, fieldsStr)

	// Exit if fatal
	if level == LogLevelFatal {
		os.Exit(1)
	}
}

// Debugf logs a formatted debug message
func (l *StandardLogger) Debugf(format string, args ...interface{}) {
	if l.levelEnabled(LogLevelDebug) {
		l.log(LogLevelDebug, fmt.Sprintf(format, args...), nil)
	}
}

// Infof logs a formatted info message
func (l *StandardLogger) Infof(format string, args ...interface{}) {
	if l.levelEnabled(LogLevelInfo) {
		l.log(LogLevelInfo, fmt.Sprintf(format, args...), nil)
	}
}

// Warnf logs a formatted warning message
func (l *StandardLogger) Warnf(format string, args ...interface{}) {
	if l.levelEnabled(LogLevelWarn) {
		l.log(LogLevelWarn, fmt.Sprintf(format, args...), nil)
	}
}

// Errorf logs a formatted error message
func (l *StandardLogger) Errorf(format string, args ...interface{}) {
	l.log(LogLevelError, fmt.Sprintf(format, args...), nil)
}

// Fatalf logs a formatted fatal message and exits
func (l *StandardLogger) Fatalf(format string, args ...interface{}) {
	// Fatal always logs regardless of level
	l.log(LogLevelFatal, fmt.Sprintf(format, args...), nil)
}

// NoopLogger is a logger that does nothing
type NoopLogger struct{}

// Debug implements Logger.Debug
func (l *NoopLogger) Debug(msg string, fields map[string]interface{}) {}

// Info implements Logger.Info
func (l *NoopLogger) Info(msg string, fields map[string]interface{}) {}

// Warn implements Logger.Warn
func (l *NoopLogger) Warn(msg string, fields map[string]interface{}) {}

// Error implements Logger.Error
func (l *NoopLogger) Error(msg string, fields map[string]interface{}) {}

// Fatal implements Logger.Fatal
func (l *NoopLogger) Fatal(msg string, fields map[string]interface{}) {}

// Debugf implements Logger.Debugf
func (l *NoopLogger) Debugf(format string, args ...interface{}) {}

// Infof implements Logger.Infof
func (l *NoopLogger) Infof(format string, args ...interface{}) {}

// Warnf implements Logger.Warnf
func (l *NoopLogger) Warnf(format string, args ...interface{}) {}

// Errorf implements Logger.Errorf
func (l *NoopLogger) Errorf(format string, args ...interface{}) {}

// Fatalf implements Logger.Fatalf
func (l *NoopLogger) Fatalf(format string, args ...interface{}) {}

// WithPrefix implements Logger.WithPrefix
func (l *NoopLogger) WithPrefix(prefix string) Logger {
	return l
}

// With implements Logger.With
func (l *NoopLogger) With(fields map[string]interface{}) Logger {
	return l
}

// NewNoopLogger creates a new NoopLogger
func NewNoopLogger() Logger {
	return &NoopLogger{}
}

// NewLogger creates a new logger with the given prefix
// This is the primary logger factory function used throughout the codebase
// It returns the standard Logger interface type (not a pointer to an implementation)
func NewLogger(prefix string) Logger {
	if prefix == "" {
		prefix = "default"
	}
	return NewStandardLogger(prefix)
}

// NewLoggerFromCommon converts a pkg/common/logging.Logger to the observability.Logger interface
// This handles the pointer vs. interface compatibility issue
func NewLoggerFromCommon(commonLogger *commonLogging.Logger) Logger {
	// If common logger is nil, return a noop logger
	if commonLogger == nil {
		return NewNoopLogger()
	}

	// Create a standard logger with the same prefix
	// This is a simplification - in a real implementation we might want to delegate
	return NewStandardLogger("common-adapter")
}
