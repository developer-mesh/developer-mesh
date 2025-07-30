// Package audit provides audit logging capabilities for cache operations.
// It tracks all cache access and modifications for compliance and security monitoring.
package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
)

// EventType represents the type of audit event
type EventType string

const (
	// Cache operation events
	EventCacheGet      EventType = "cache.get"
	EventCacheSet      EventType = "cache.set"
	EventCacheDelete   EventType = "cache.delete"
	EventCacheClear    EventType = "cache.clear"
	EventCacheEviction EventType = "cache.eviction"

	// Security events
	EventAccessDenied EventType = "security.access_denied"
	EventEncryption   EventType = "security.encryption"
	EventDecryption   EventType = "security.decryption"
	EventKeyRotation  EventType = "security.key_rotation"

	// System events
	EventDegradedMode EventType = "system.degraded_mode"
	EventRecovery     EventType = "system.recovery"
	EventConfigChange EventType = "system.config_change"
)

// Result represents the outcome of an operation
type Result string

const (
	ResultSuccess Result = "success"
	ResultFailure Result = "failure"
	ResultPartial Result = "partial"
)

// AuditEvent represents a cache audit event for compliance logging
type AuditEvent struct {
	EventID   string                 `json:"event_id"`
	EventType EventType              `json:"event_type"`
	TenantID  uuid.UUID              `json:"tenant_id"`
	UserID    string                 `json:"user_id,omitempty"`
	Operation string                 `json:"operation"`
	Resource  string                 `json:"resource"`
	Result    Result                 `json:"result"`
	Error     string                 `json:"error,omitempty"`
	Duration  time.Duration          `json:"duration_ms"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`

	// Compliance fields
	IPAddress string `json:"ip_address,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// Logger provides audit logging capabilities
type Logger struct {
	logger    observability.Logger
	enabled   bool
	filter    EventFilter
	formatter EventFormatter
}

// EventFilter determines which events should be logged
type EventFilter func(event *AuditEvent) bool

// EventFormatter formats events for output
type EventFormatter func(event *AuditEvent) ([]byte, error)

// NewLogger creates a new audit logger
func NewLogger(logger observability.Logger, enabled bool) *Logger {
	if logger == nil {
		logger = observability.NewLogger("cache.audit")
	}

	return &Logger{
		logger:    logger,
		enabled:   enabled,
		filter:    defaultFilter,
		formatter: jsonFormatter,
	}
}

// SetFilter sets a custom event filter
func (l *Logger) SetFilter(filter EventFilter) {
	l.filter = filter
}

// SetFormatter sets a custom event formatter
func (l *Logger) SetFormatter(formatter EventFormatter) {
	l.formatter = formatter
}

// LogOperation logs a cache operation with timing
func (l *Logger) LogOperation(ctx context.Context, eventType EventType, operation string, resource string, start time.Time, err error) {
	if !l.enabled {
		return
	}

	duration := time.Since(start)
	result := ResultSuccess
	errorMsg := ""

	if err != nil {
		result = ResultFailure
		errorMsg = err.Error()
	}

	event := &AuditEvent{
		EventID:   uuid.New().String(),
		EventType: eventType,
		TenantID:  auth.GetTenantID(ctx),
		UserID:    auth.GetUserID(ctx),
		Operation: operation,
		Resource:  resource,
		Result:    result,
		Error:     errorMsg,
		Duration:  duration,
		Timestamp: time.Now().UTC(),
		RequestID: getRequestID(ctx),
		SessionID: getSessionID(ctx),
		IPAddress: getIPAddress(ctx),
		UserAgent: getUserAgent(ctx),
	}

	l.Log(event)
}

// LogSecurityEvent logs a security-related event
func (l *Logger) LogSecurityEvent(ctx context.Context, eventType EventType, resource string, metadata map[string]interface{}) {
	if !l.enabled {
		return
	}

	event := &AuditEvent{
		EventID:   uuid.New().String(),
		EventType: eventType,
		TenantID:  auth.GetTenantID(ctx),
		UserID:    auth.GetUserID(ctx),
		Operation: string(eventType),
		Resource:  resource,
		Result:    ResultSuccess,
		Metadata:  metadata,
		Timestamp: time.Now().UTC(),
		RequestID: getRequestID(ctx),
		SessionID: getSessionID(ctx),
		IPAddress: getIPAddress(ctx),
		UserAgent: getUserAgent(ctx),
	}

	l.Log(event)
}

// LogSystemEvent logs a system-level event
func (l *Logger) LogSystemEvent(eventType EventType, description string, metadata map[string]interface{}) {
	if !l.enabled {
		return
	}

	event := &AuditEvent{
		EventID:   uuid.New().String(),
		EventType: eventType,
		Operation: string(eventType),
		Resource:  "system",
		Result:    ResultSuccess,
		Metadata:  metadata,
		Timestamp: time.Now().UTC(),
	}

	if description != "" {
		if event.Metadata == nil {
			event.Metadata = make(map[string]interface{})
		}
		event.Metadata["description"] = description
	}

	l.Log(event)
}

// Log writes an audit event
func (l *Logger) Log(event *AuditEvent) {
	if !l.enabled || event == nil {
		return
	}

	// Apply filter
	if l.filter != nil && !l.filter(event) {
		return
	}

	// Format event
	data, err := l.formatter(event)
	if err != nil {
		l.logger.Error("Failed to format audit event", map[string]interface{}{
			"error":      err.Error(),
			"event_type": string(event.EventType),
		})
		return
	}

	// Log based on event type and result
	fields := map[string]interface{}{
		"audit":      true,
		"event_id":   event.EventID,
		"event_type": string(event.EventType),
		"tenant_id":  event.TenantID.String(),
		"result":     string(event.Result),
		"duration":   event.Duration.Milliseconds(),
	}

	if event.UserID != "" {
		fields["user_id"] = event.UserID
	}

	if event.Error != "" {
		fields["error"] = event.Error
	}

	// Use appropriate log level
	switch event.Result {
	case ResultFailure:
		l.logger.Error("Audit event", fields)
	case ResultPartial:
		l.logger.Warn("Audit event", fields)
	default:
		l.logger.Info("Audit event", fields)
	}

	// Also write the full formatted event
	l.logger.Info(string(data), map[string]interface{}{
		"audit_raw": true,
	})
}

// Helper functions

func defaultFilter(event *AuditEvent) bool {
	// Log all events by default
	return true
}

func jsonFormatter(event *AuditEvent) ([]byte, error) {
	return json.Marshal(event)
}

func getRequestID(ctx context.Context) string {
	if id, ok := ctx.Value("request_id").(string); ok {
		return id
	}
	return ""
}

func getSessionID(ctx context.Context) string {
	if id, ok := ctx.Value("session_id").(string); ok {
		return id
	}
	return ""
}

func getIPAddress(ctx context.Context) string {
	if ip, ok := ctx.Value("ip_address").(string); ok {
		return ip
	}
	return ""
}

func getUserAgent(ctx context.Context) string {
	if ua, ok := ctx.Value("user_agent").(string); ok {
		return ua
	}
	return ""
}

// ComplianceLogger wraps Logger with compliance-specific methods
type ComplianceLogger struct {
	*Logger
}

// NewComplianceLogger creates a logger for compliance requirements
func NewComplianceLogger(logger observability.Logger) *ComplianceLogger {
	return &ComplianceLogger{
		Logger: NewLogger(logger, true),
	}
}

// LogDataAccess logs data access for compliance
func (cl *ComplianceLogger) LogDataAccess(ctx context.Context, operation string, dataType string, recordCount int, sensitive bool) {
	// Log security event with metadata
	cl.LogSecurityEvent(ctx, EventCacheGet, dataType, map[string]interface{}{
		"operation":    operation,
		"data_type":    dataType,
		"record_count": recordCount,
		"sensitive":    sensitive,
	})
}

// LogDataModification logs data modifications for compliance
func (cl *ComplianceLogger) LogDataModification(ctx context.Context, operation string, dataType string, recordCount int, changeType string) {
	event := &AuditEvent{
		EventID:   uuid.New().String(),
		EventType: EventCacheSet,
		TenantID:  auth.GetTenantID(ctx),
		UserID:    auth.GetUserID(ctx),
		Operation: operation,
		Resource:  dataType,
		Result:    ResultSuccess,
		Metadata: map[string]interface{}{
			"data_type":    dataType,
			"record_count": recordCount,
			"change_type":  changeType,
		},
		Timestamp: time.Now().UTC(),
	}

	cl.Log(event)
}
