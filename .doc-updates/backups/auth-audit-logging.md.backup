# Authentication Audit Logging Guide

> **Status**: Implementation Guide
> **Complexity**: Medium
> **Estimated Effort**: 1 week
> **Dependencies**: Existing auth system, PostgreSQL, observability package

## Overview

This guide provides comprehensive instructions for implementing audit logging for the Developer Mesh authentication and authorization system. Audit logging is crucial for security compliance, debugging authentication issues, and maintaining a forensic trail of all authentication events.

## Current State

Currently, the platform has basic logging through the observability package but lacks dedicated audit logging for authentication events. This guide outlines how to implement comprehensive audit logging.

```go
// Current: Basic logging
logger.Info("User authenticated", map[string]interface{}{
    "user_id": userID,
})

// Goal: Comprehensive audit logging
auditLogger.LogAuthEvent(ctx, AuthEvent{
    Type:      EventTypeLogin,
    UserID:    userID,
    TenantID:  tenantID,
    Result:    AuthResultSuccess,
    Metadata:  metadata,
})
```

## Architecture

### High-Level Design
```
┌─────────────┐     ┌────────────┐     ┌──────────────┐
│   Auth      │────▶│   Audit    │────▶│  PostgreSQL  │
│  Providers  │     │   Logger   │     │   (Audit)    │
└─────────────┘     └────────────┘     └──────────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │ Observability│
                    │  (Logging)   │
                    └──────────────┘
```

## Implementation

### 1. Audit Event Types

#### 1.1 Define Event Types
```go
// pkg/auth/audit/types.go
package audit

import (
    "time"
    "github.com/google/uuid"
)

// EventType represents the type of authentication event
type EventType string

const (
    // Authentication events
    EventTypeLogin           EventType = "auth.login"
    EventTypeLogout          EventType = "auth.logout"
    EventTypeTokenGenerated  EventType = "auth.token.generated"
    EventTypeTokenValidated  EventType = "auth.token.validated"
    EventTypeTokenRevoked    EventType = "auth.token.revoked"
    EventTypeTokenRefreshed  EventType = "auth.token.refreshed"
    
    // API Key events
    EventTypeAPIKeyCreated   EventType = "auth.apikey.created"
    EventTypeAPIKeyUsed      EventType = "auth.apikey.used"
    EventTypeAPIKeyRotated   EventType = "auth.apikey.rotated"
    EventTypeAPIKeyRevoked   EventType = "auth.apikey.revoked"
    
    // OAuth events
    EventTypeOAuthInitiated  EventType = "auth.oauth.initiated"
    EventTypeOAuthCallback   EventType = "auth.oauth.callback"
    EventTypeOAuthLinked     EventType = "auth.oauth.linked"
    
    // Authorization events
    EventTypePermissionCheck EventType = "authz.permission.check"
    EventTypePolicyUpdated   EventType = "authz.policy.updated"
    EventTypeRoleAssigned    EventType = "authz.role.assigned"
    EventTypeRoleRevoked     EventType = "authz.role.revoked"
    
    // Security events
    EventTypeRateLimitExceeded EventType = "security.ratelimit.exceeded"
    EventTypeInvalidToken      EventType = "security.invalid.token"
    EventTypeSuspiciousActivity EventType = "security.suspicious.activity"
)

// AuthResult represents the result of an authentication attempt
type AuthResult string

const (
    AuthResultSuccess AuthResult = "success"
    AuthResultFailure AuthResult = "failure"
    AuthResultError   AuthResult = "error"
)

// AuditEvent represents a single audit log entry
type AuditEvent struct {
    ID           uuid.UUID              `json:"id" db:"id"`
    EventType    EventType              `json:"event_type" db:"event_type"`
    Timestamp    time.Time              `json:"timestamp" db:"timestamp"`
    UserID       *uuid.UUID             `json:"user_id,omitempty" db:"user_id"`
    TenantID     uuid.UUID              `json:"tenant_id" db:"tenant_id"`
    SessionID    *string                `json:"session_id,omitempty" db:"session_id"`
    IPAddress    string                 `json:"ip_address" db:"ip_address"`
    UserAgent    string                 `json:"user_agent" db:"user_agent"`
    Resource     string                 `json:"resource,omitempty" db:"resource"`
    Action       string                 `json:"action,omitempty" db:"action"`
    Result       AuthResult             `json:"result" db:"result"`
    ErrorMessage *string                `json:"error_message,omitempty" db:"error_message"`
    Metadata     map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
    
    // Compliance fields
    DataClassification string `json:"data_classification" db:"data_classification"`
    ComplianceFlags    []string `json:"compliance_flags,omitempty" db:"compliance_flags"`
}
```

### 2. Audit Logger Implementation

#### 2.1 Core Audit Logger
```go
// pkg/auth/audit/logger.go
package audit

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/google/uuid"
    "github.com/jmoiron/sqlx"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
)

type AuditLogger interface {
    LogEvent(ctx context.Context, event AuditEvent) error
    LogAuthEvent(ctx context.Context, eventType EventType, userID *uuid.UUID, result AuthResult, metadata map[string]interface{}) error
    QueryEvents(ctx context.Context, filter EventFilter) ([]AuditEvent, error)
}

type auditLogger struct {
    db       *sqlx.DB
    logger   observability.Logger
    tracer   observability.Tracer
    enricher *EventEnricher
}

func NewAuditLogger(db *sqlx.DB, logger observability.Logger, tracer observability.Tracer) AuditLogger {
    return &auditLogger{
        db:       db,
        logger:   logger,
        tracer:   tracer,
        enricher: NewEventEnricher(),
    }
}

func (a *auditLogger) LogEvent(ctx context.Context, event AuditEvent) error {
    ctx, span := a.tracer.Start(ctx, "AuditLogger.LogEvent")
    defer span.End()
    
    // Set ID and timestamp if not provided
    if event.ID == uuid.Nil {
        event.ID = uuid.New()
    }
    if event.Timestamp.IsZero() {
        event.Timestamp = time.Now().UTC()
    }
    
    // Enrich event with context data
    a.enricher.Enrich(ctx, &event)
    
    // Serialize metadata
    metadataJSON, err := json.Marshal(event.Metadata)
    if err != nil {
        return fmt.Errorf("failed to marshal metadata: %w", err)
    }
    
    // Insert into database
    query := `
        INSERT INTO audit_logs (
            id, event_type, timestamp, user_id, tenant_id, session_id,
            ip_address, user_agent, resource, action, result,
            error_message, metadata, data_classification, compliance_flags
        ) VALUES (
            :id, :event_type, :timestamp, :user_id, :tenant_id, :session_id,
            :ip_address, :user_agent, :resource, :action, :result,
            :error_message, :metadata, :data_classification, :compliance_flags
        )
    `
    
    _, err = a.db.NamedExecContext(ctx, query, map[string]interface{}{
        "id":                  event.ID,
        "event_type":          event.EventType,
        "timestamp":           event.Timestamp,
        "user_id":             event.UserID,
        "tenant_id":           event.TenantID,
        "session_id":          event.SessionID,
        "ip_address":          event.IPAddress,
        "user_agent":          event.UserAgent,
        "resource":            event.Resource,
        "action":              event.Action,
        "result":              event.Result,
        "error_message":       event.ErrorMessage,
        "metadata":            metadataJSON,
        "data_classification": event.DataClassification,
        "compliance_flags":    event.ComplianceFlags,
    })
    
    if err != nil {
        // Log to observability even if DB write fails
        a.logger.Error("Failed to write audit log", map[string]interface{}{
            "error":      err.Error(),
            "event_type": event.EventType,
            "user_id":    event.UserID,
        })
        return fmt.Errorf("failed to insert audit log: %w", err)
    }
    
    // Also log to observability for real-time monitoring
    a.logToObservability(event)
    
    return nil
}

func (a *auditLogger) logToObservability(event AuditEvent) {
    logData := map[string]interface{}{
        "audit_event_id":   event.ID.String(),
        "event_type":       string(event.EventType),
        "result":           string(event.Result),
        "tenant_id":        event.TenantID.String(),
        "ip_address":       event.IPAddress,
        "data_classification": event.DataClassification,
    }
    
    if event.UserID != nil {
        logData["user_id"] = event.UserID.String()
    }
    
    if event.ErrorMessage != nil {
        logData["error"] = *event.ErrorMessage
    }
    
    // Add selected metadata fields
    for k, v := range event.Metadata {
        if k == "provider" || k == "method" || k == "duration_ms" {
            logData[k] = v
        }
    }
    
    switch event.Result {
    case AuthResultSuccess:
        a.logger.Info("Authentication audit event", logData)
    case AuthResultFailure:
        a.logger.Warn("Authentication audit event", logData)
    case AuthResultError:
        a.logger.Error("Authentication audit event", logData)
    }
}
```

#### 2.2 Event Enricher
```go
// pkg/auth/audit/enricher.go
package audit

import (
    "context"
    "net"
    "strings"
    
    "github.com/gin-gonic/gin"
)

type EventEnricher struct {
    geoIP GeoIPProvider // Optional geo-location
}

func NewEventEnricher() *EventEnricher {
    return &EventEnricher{}
}

func (e *EventEnricher) Enrich(ctx context.Context, event *AuditEvent) {
    // Extract from Gin context if available
    if ginCtx, ok := ctx.(*gin.Context); ok {
        e.enrichFromGin(ginCtx, event)
    }
    
    // Extract from context values
    e.enrichFromContext(ctx, event)
    
    // Add compliance classifications
    e.addComplianceData(event)
}

func (e *EventEnricher) enrichFromGin(c *gin.Context, event *AuditEvent) {
    // Get IP address
    if event.IPAddress == "" {
        event.IPAddress = e.getClientIP(c)
    }
    
    // Get user agent
    if event.UserAgent == "" {
        event.UserAgent = c.GetHeader("User-Agent")
    }
    
    // Get session ID from header or cookie
    if event.SessionID == nil {
        if sessionID := c.GetHeader("X-Session-ID"); sessionID != "" {
            event.SessionID = &sessionID
        }
    }
    
    // Add request metadata
    if event.Metadata == nil {
        event.Metadata = make(map[string]interface{})
    }
    
    event.Metadata["request_id"] = c.GetString("request_id")
    event.Metadata["method"] = c.Request.Method
    event.Metadata["path"] = c.Request.URL.Path
}

func (e *EventEnricher) getClientIP(c *gin.Context) string {
    // Check X-Forwarded-For header
    if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
        ips := strings.Split(xff, ",")
        if len(ips) > 0 {
            return strings.TrimSpace(ips[0])
        }
    }
    
    // Check X-Real-IP header
    if xri := c.GetHeader("X-Real-IP"); xri != "" {
        return xri
    }
    
    // Fall back to remote address
    ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
    return ip
}

func (e *EventEnricher) enrichFromContext(ctx context.Context, event *AuditEvent) {
    // Extract tenant ID if not set
    if event.TenantID == uuid.Nil {
        if tenantID, ok := ctx.Value("tenant_id").(uuid.UUID); ok {
            event.TenantID = tenantID
        }
    }
    
    // Extract trace ID for correlation
    if traceID := observability.GetTraceID(ctx); traceID != "" {
        if event.Metadata == nil {
            event.Metadata = make(map[string]interface{})
        }
        event.Metadata["trace_id"] = traceID
    }
}

func (e *EventEnricher) addComplianceData(event *AuditEvent) {
    // Set data classification based on event type
    switch event.EventType {
    case EventTypeLogin, EventTypeLogout, EventTypeTokenGenerated:
        event.DataClassification = "CONFIDENTIAL"
    case EventTypePermissionCheck:
        event.DataClassification = "INTERNAL"
    default:
        event.DataClassification = "PUBLIC"
    }
    
    // Add compliance flags
    event.ComplianceFlags = []string{}
    
    // Check if event contains PII
    if event.UserID != nil || strings.Contains(string(event.EventType), "user") {
        event.ComplianceFlags = append(event.ComplianceFlags, "PII")
    }
    
    // Check if event is security-related
    if strings.HasPrefix(string(event.EventType), "security.") {
        event.ComplianceFlags = append(event.ComplianceFlags, "SECURITY_EVENT")
    }
}
```

### 3. Database Schema

#### 3.1 Audit Log Table
```sql
-- migrations/create_audit_logs.sql
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    user_id UUID,
    tenant_id UUID NOT NULL,
    session_id VARCHAR(255),
    ip_address INET NOT NULL,
    user_agent TEXT,
    resource VARCHAR(255),
    action VARCHAR(100),
    result VARCHAR(20) NOT NULL,
    error_message TEXT,
    metadata JSONB,
    data_classification VARCHAR(50) NOT NULL DEFAULT 'PUBLIC',
    compliance_flags TEXT[],
    
    -- Indexes for querying
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Performance indexes
CREATE INDEX idx_audit_logs_timestamp ON audit_logs(timestamp DESC);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_audit_logs_tenant_id ON audit_logs(tenant_id);
CREATE INDEX idx_audit_logs_event_type ON audit_logs(event_type);
CREATE INDEX idx_audit_logs_result ON audit_logs(result);
CREATE INDEX idx_audit_logs_ip_address ON audit_logs(ip_address);

-- Composite indexes for common queries
CREATE INDEX idx_audit_logs_user_tenant_time ON audit_logs(user_id, tenant_id, timestamp DESC);
CREATE INDEX idx_audit_logs_tenant_type_time ON audit_logs(tenant_id, event_type, timestamp DESC);

-- Partitioning for large-scale deployments (optional)
-- Partition by month for better performance
CREATE TABLE audit_logs_2024_01 PARTITION OF audit_logs
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');
```

### 4. Integration with Auth Providers

#### 4.1 JWT Provider Integration
```go
// Update pkg/auth/jwt_provider.go
func (p *JWTProvider) GenerateToken(user *User) (string, error) {
    // ... existing code ...
    
    // Log token generation
    p.auditLogger.LogAuthEvent(context.Background(), 
        EventTypeTokenGenerated,
        &user.ID,
        AuthResultSuccess,
        map[string]interface{}{
            "provider": "jwt",
            "expires_in": p.expiresIn.Seconds(),
            "scopes": user.Scopes,
        },
    )
    
    return tokenString, nil
}

func (p *JWTProvider) ValidateToken(tokenString string) (*Claims, error) {
    // ... existing validation code ...
    
    if err != nil {
        // Log validation failure
        p.auditLogger.LogAuthEvent(context.Background(),
            EventTypeTokenValidated,
            nil,
            AuthResultFailure,
            map[string]interface{}{
                "provider": "jwt",
                "error": err.Error(),
            },
        )
        return nil, err
    }
    
    // Log successful validation
    userID, _ := uuid.Parse(claims.UserID)
    p.auditLogger.LogAuthEvent(context.Background(),
        EventTypeTokenValidated,
        &userID,
        AuthResultSuccess,
        map[string]interface{}{
            "provider": "jwt",
            "token_id": claims.ID,
        },
    )
    
    return claims, nil
}
```

#### 4.2 API Key Provider Integration
```go
// Update pkg/auth/apikey_provider.go
func (p *APIKeyProvider) Authenticate(ctx context.Context, key string) (*User, error) {
    // ... existing code ...
    
    apiKey, err := p.service.ValidateKey(ctx, key)
    if err != nil {
        // Log authentication failure
        p.auditLogger.LogEvent(ctx, AuditEvent{
            EventType: EventTypeAPIKeyUsed,
            Result:    AuthResultFailure,
            ErrorMessage: &err.Error(),
            Metadata: map[string]interface{}{
                "provider": "apikey",
                "key_hint": key[:8] + "...", // Log only hint
            },
        })
        return nil, err
    }
    
    // Log successful authentication
    p.auditLogger.LogEvent(ctx, AuditEvent{
        EventType: EventTypeAPIKeyUsed,
        UserID:    &apiKey.UserID,
        TenantID:  apiKey.TenantID,
        Result:    AuthResultSuccess,
        Metadata: map[string]interface{}{
            "provider": "apikey",
            "key_id": apiKey.ID,
            "key_name": apiKey.Name,
        },
    })
    
    return user, nil
}
```

### 5. Query and Analysis

#### 5.1 Query Interface
```go
// pkg/auth/audit/query.go
package audit

import (
    "time"
    "github.com/google/uuid"
)

type EventFilter struct {
    StartTime    *time.Time
    EndTime      *time.Time
    UserID       *uuid.UUID
    TenantID     *uuid.UUID
    EventTypes   []EventType
    Results      []AuthResult
    IPAddress    *string
    Resource     *string
    Limit        int
    Offset       int
}

func (a *auditLogger) QueryEvents(ctx context.Context, filter EventFilter) ([]AuditEvent, error) {
    ctx, span := a.tracer.Start(ctx, "AuditLogger.QueryEvents")
    defer span.End()
    
    query := `
        SELECT id, event_type, timestamp, user_id, tenant_id, session_id,
               ip_address, user_agent, resource, action, result,
               error_message, metadata, data_classification, compliance_flags
        FROM audit_logs
        WHERE 1=1
    `
    
    args := map[string]interface{}{}
    
    if filter.StartTime != nil {
        query += " AND timestamp >= :start_time"
        args["start_time"] = *filter.StartTime
    }
    
    if filter.EndTime != nil {
        query += " AND timestamp <= :end_time"
        args["end_time"] = *filter.EndTime
    }
    
    if filter.UserID != nil {
        query += " AND user_id = :user_id"
        args["user_id"] = *filter.UserID
    }
    
    if filter.TenantID != nil {
        query += " AND tenant_id = :tenant_id"
        args["tenant_id"] = *filter.TenantID
    }
    
    if len(filter.EventTypes) > 0 {
        query += " AND event_type = ANY(:event_types)"
        args["event_types"] = filter.EventTypes
    }
    
    query += " ORDER BY timestamp DESC LIMIT :limit OFFSET :offset"
    args["limit"] = filter.Limit
    args["offset"] = filter.Offset
    
    rows, err := a.db.NamedQueryContext(ctx, query, args)
    if err != nil {
        return nil, fmt.Errorf("failed to query audit logs: %w", err)
    }
    defer rows.Close()
    
    var events []AuditEvent
    for rows.Next() {
        var event AuditEvent
        if err := rows.StructScan(&event); err != nil {
            return nil, fmt.Errorf("failed to scan audit log: %w", err)
        }
        events = append(events, event)
    }
    
    return events, nil
}
```

#### 5.2 Analytics Queries
```go
// pkg/auth/audit/analytics.go
package audit

type AuditAnalytics struct {
    db     *sqlx.DB
    logger observability.Logger
}

func (a *AuditAnalytics) GetLoginAttemptsByHour(tenantID uuid.UUID, hours int) ([]HourlyStats, error) {
    query := `
        SELECT 
            date_trunc('hour', timestamp) as hour,
            COUNT(*) FILTER (WHERE result = 'success') as success_count,
            COUNT(*) FILTER (WHERE result = 'failure') as failure_count,
            COUNT(DISTINCT user_id) as unique_users
        FROM audit_logs
        WHERE tenant_id = $1
            AND event_type = $2
            AND timestamp >= NOW() - INTERVAL '%d hours'
        GROUP BY hour
        ORDER BY hour DESC
    `
    
    // ... execute and return
}

func (a *AuditAnalytics) GetSuspiciousActivity(tenantID uuid.UUID) ([]SuspiciousEvent, error) {
    query := `
        WITH failed_logins AS (
            SELECT 
                ip_address,
                COUNT(*) as failure_count,
                MAX(timestamp) as last_attempt
            FROM audit_logs
            WHERE tenant_id = $1
                AND event_type IN ('auth.login', 'auth.token.validated')
                AND result = 'failure'
                AND timestamp >= NOW() - INTERVAL '1 hour'
            GROUP BY ip_address
            HAVING COUNT(*) >= 5
        )
        SELECT 
            al.*,
            fl.failure_count
        FROM audit_logs al
        JOIN failed_logins fl ON al.ip_address = fl.ip_address
        WHERE al.tenant_id = $1
            AND al.timestamp >= NOW() - INTERVAL '1 hour'
        ORDER BY al.timestamp DESC
    `
    
    // ... execute and return
}
```

### 6. Compliance and Retention

#### 6.1 Data Retention Policy
```go
// pkg/auth/audit/retention.go
package audit

type RetentionPolicy struct {
    db       *sqlx.DB
    logger   observability.Logger
    policies map[string]RetentionRule
}

type RetentionRule struct {
    DataClassification string
    RetentionDays      int
    ArchiveAfterDays   int
}

func NewRetentionPolicy(db *sqlx.DB) *RetentionPolicy {
    return &RetentionPolicy{
        db: db,
        policies: map[string]RetentionRule{
            "CONFIDENTIAL": {
                DataClassification: "CONFIDENTIAL",
                RetentionDays:      365 * 2, // 2 years
                ArchiveAfterDays:   90,
            },
            "INTERNAL": {
                DataClassification: "INTERNAL",
                RetentionDays:      365, // 1 year
                ArchiveAfterDays:   30,
            },
            "PUBLIC": {
                DataClassification: "PUBLIC",
                RetentionDays:      90,
                ArchiveAfterDays:   0, // No archive
            },
        },
    }
}

func (r *RetentionPolicy) ApplyRetentionPolicies(ctx context.Context) error {
    for classification, rule := range r.policies {
        // Archive old logs
        if rule.ArchiveAfterDays > 0 {
            if err := r.archiveLogs(ctx, classification, rule.ArchiveAfterDays); err != nil {
                return err
            }
        }
        
        // Delete expired logs
        if err := r.deleteLogs(ctx, classification, rule.RetentionDays); err != nil {
            return err
        }
    }
    
    return nil
}
```

### 7. Security Considerations

#### 7.1 Log Sanitization
```go
// pkg/auth/audit/sanitizer.go
package audit

import (
    "regexp"
    "strings"
)

type LogSanitizer struct {
    patterns map[string]*regexp.Regexp
}

func NewLogSanitizer() *LogSanitizer {
    return &LogSanitizer{
        patterns: map[string]*regexp.Regexp{
            "api_key":     regexp.MustCompile(`(?i)(api[_-]?key|apikey)[\s:=]+([a-zA-Z0-9\-_]+)`),
            "password":    regexp.MustCompile(`(?i)(password|passwd|pwd)[\s:=]+([^\s]+)`),
            "token":       regexp.MustCompile(`(?i)(token|jwt|bearer)[\s:=]+([a-zA-Z0-9\-_.]+)`),
            "credit_card": regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`),
        },
    }
}

func (s *LogSanitizer) Sanitize(data map[string]interface{}) map[string]interface{} {
    sanitized := make(map[string]interface{})
    
    for key, value := range data {
        switch v := value.(type) {
        case string:
            sanitized[key] = s.sanitizeString(v)
        case map[string]interface{}:
            sanitized[key] = s.Sanitize(v)
        default:
            sanitized[key] = value
        }
    }
    
    return sanitized
}

func (s *LogSanitizer) sanitizeString(str string) string {
    result := str
    
    for name, pattern := range s.patterns {
        result = pattern.ReplaceAllStringFunc(result, func(match string) string {
            parts := pattern.FindStringSubmatch(match)
            if len(parts) > 2 {
                // Keep the key part, redact the value
                return parts[1] + "=[REDACTED-" + strings.ToUpper(name) + "]"
            }
            return "[REDACTED]"
        })
    }
    
    return result
}
```

### 8. API Endpoints for Audit Logs

#### 8.1 REST API Handlers
```go
// apps/rest-api/internal/api/audit_handlers.go
package api

// GetAuditLogs godoc
// @Summary Get audit logs
// @Description Query audit logs with filters
// @Tags audit
// @Accept json
// @Produce json
// @Param start_time query string false "Start time (RFC3339)"
// @Param end_time query string false "End time (RFC3339)"
// @Param user_id query string false "User ID"
// @Param event_type query string false "Event type"
// @Param result query string false "Result (success/failure/error)"
// @Param limit query int false "Limit" default(100)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} AuditLogsResponse
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /audit/logs [get]
func (h *AuditHandler) GetAuditLogs(c *gin.Context) {
    // Parse query parameters
    filter := audit.EventFilter{
        Limit:  100,
        Offset: 0,
    }
    
    if startTime := c.Query("start_time"); startTime != "" {
        t, _ := time.Parse(time.RFC3339, startTime)
        filter.StartTime = &t
    }
    
    if userID := c.Query("user_id"); userID != "" {
        uid, _ := uuid.Parse(userID)
        filter.UserID = &uid
    }
    
    // Apply tenant isolation
    tenantID := c.GetString("tenant_id")
    tid, _ := uuid.Parse(tenantID)
    filter.TenantID = &tid
    
    // Query logs
    events, err := h.auditLogger.QueryEvents(c.Request.Context(), filter)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "events": events,
        "count":  len(events),
        "filter": filter,
    })
}

// GetAuditAnalytics godoc
// @Summary Get audit analytics
// @Description Get authentication analytics and statistics
// @Tags audit
// @Produce json
// @Success 200 {object} AuditAnalyticsResponse
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /audit/analytics [get]
func (h *AuditHandler) GetAuditAnalytics(c *gin.Context) {
    tenantID, _ := uuid.Parse(c.GetString("tenant_id"))
    
    // Get various analytics
    loginStats, _ := h.analytics.GetLoginAttemptsByHour(tenantID, 24)
    suspiciousActivity, _ := h.analytics.GetSuspiciousActivity(tenantID)
    
    c.JSON(http.StatusOK, gin.H{
        "login_stats": loginStats,
        "suspicious_activity": suspiciousActivity,
        "generated_at": time.Now(),
    })
}
```

## Testing

### 9.1 Unit Tests
```go
func TestAuditLogger(t *testing.T) {
    db := setupTestDB(t)
    logger := observability.NewLogger("test")
    tracer := observability.NewTracer("test")
    
    auditLogger := NewAuditLogger(db, logger, tracer)
    
    t.Run("LogEvent", func(t *testing.T) {
        ctx := context.Background()
        userID := uuid.New()
        
        event := AuditEvent{
            EventType: EventTypeLogin,
            UserID:    &userID,
            TenantID:  uuid.New(),
            Result:    AuthResultSuccess,
            IPAddress: "192.168.1.1",
            UserAgent: "Test/1.0",
            Metadata: map[string]interface{}{
                "provider": "jwt",
            },
        }
        
        err := auditLogger.LogEvent(ctx, event)
        assert.NoError(t, err)
        
        // Verify event was logged
        events, err := auditLogger.QueryEvents(ctx, EventFilter{
            UserID: &userID,
            Limit:  1,
        })
        
        assert.NoError(t, err)
        assert.Len(t, events, 1)
        assert.Equal(t, EventTypeLogin, events[0].EventType)
    })
}
```

## Monitoring and Alerts

### 10.1 Metrics
```go
// Prometheus metrics for audit logging
var (
    auditEventCounter = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "auth_audit_events_total",
            Help: "Total number of audit events",
        },
        []string{"event_type", "result", "tenant_id"},
    )
    
    auditEventDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "auth_audit_event_duration_seconds",
            Help: "Time taken to log audit event",
        },
        []string{"event_type"},
    )
)
```

### 10.2 Alert Rules
```yaml
# Prometheus alert rules
groups:
  - name: auth_audit_alerts
    rules:
      - alert: HighAuthFailureRate
        expr: |
          rate(auth_audit_events_total{result="failure"}[5m]) > 10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: High authentication failure rate
          
      - alert: SuspiciousLoginActivity
        expr: |
          increase(auth_audit_events_total{event_type="security.suspicious.activity"}[1h]) > 5
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: Suspicious login activity detected
```

## Best Practices

1. **Always Log**: Log all authentication events, both success and failure
2. **Context Propagation**: Always pass context through the call chain
3. **Sanitization**: Sanitize sensitive data before logging
4. **Structured Logging**: Use structured logging for easy querying
5. **Compliance**: Follow data retention policies
6. **Performance**: Use async logging for high-throughput scenarios
7. **Security**: Encrypt audit logs at rest and in transit
8. **Monitoring**: Set up alerts for suspicious patterns

## Troubleshooting

### Common Issues

1. **Missing Events**
   - Check if audit logger is properly initialized
   - Verify context is passed correctly
   - Check database connectivity

2. **Performance Impact**
   - Enable async logging
   - Implement batching for high volume
   - Consider partitioning strategy

3. **Storage Growth**
   - Implement retention policies
   - Archive old logs to S3
   - Use compression for archived logs

## Next Steps

1. Implement audit logger interface
2. Add database migrations
3. Integrate with existing auth providers
4. Create API endpoints
5. Set up monitoring and alerts
6. Configure retention policies
7. Performance testing
8. Security audit

## Resources

- [NIST SP 800-92](https://csrc.nist.gov/publications/detail/sp/800-92/final) - Guide to Security Log Management
- [OWASP Logging Guide](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Vocabulary_Cheat_Sheet.html)
- [PCI DSS Logging Requirements](https://www.pcisecuritystandards.org/)