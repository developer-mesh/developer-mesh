package auth

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// DynamicToolAuditEvent extends AuditEvent for dynamic tool operations
type DynamicToolAuditEvent struct {
	AuditEvent
	ToolID     string                 `json:"tool_id,omitempty"`
	ToolName   string                 `json:"tool_name,omitempty"`
	Action     string                 `json:"action,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Result     interface{}            `json:"result,omitempty"`
	Duration   int64                  `json:"duration_ms,omitempty"`
}

// LogToolRegistration logs a tool registration event
func (al *AuditLogger) LogToolRegistration(ctx context.Context, tenantID, toolID, toolName string, success bool, err error) {
	event := DynamicToolAuditEvent{
		AuditEvent: AuditEvent{
			Timestamp: time.Now(),
			EventType: "tool_registration",
			TenantID:  tenantID,
			Success:   success,
		},
		ToolID:   toolID,
		ToolName: toolName,
	}

	if err != nil {
		event.Error = err.Error()
	}

	al.logger.Info("AUDIT: Tool registration", map[string]interface{}{
		"event_type": event.EventType,
		"tenant_id":  event.TenantID,
		"tool_id":    event.ToolID,
		"tool_name":  event.ToolName,
		"success":    event.Success,
		"error":      event.Error,
	})
}

// LogToolDiscovery logs a tool discovery event
func (al *AuditLogger) LogToolDiscovery(ctx context.Context, tenantID, baseURL string, discovered []string, err error) {
	event := DynamicToolAuditEvent{
		AuditEvent: AuditEvent{
			Timestamp: time.Now(),
			EventType: "tool_discovery",
			TenantID:  tenantID,
			Success:   err == nil,
			Metadata: map[string]interface{}{
				"base_url":        baseURL,
				"discovered_urls": discovered,
			},
		},
	}

	if err != nil {
		event.Error = err.Error()
	}

	al.logger.Info("AUDIT: Tool discovery", map[string]interface{}{
		"event_type":       event.EventType,
		"tenant_id":        event.TenantID,
		"base_url":         baseURL,
		"discovered_count": len(discovered),
		"success":          event.Success,
		"error":            event.Error,
	})
}

// LogToolExecution logs a tool execution event
func (al *AuditLogger) LogToolExecution(ctx context.Context, tenantID, toolID, action string, params map[string]interface{}, result interface{}, duration time.Duration, err error, metadata map[string]interface{}) {
	event := DynamicToolAuditEvent{
		AuditEvent: AuditEvent{
			Timestamp: time.Now(),
			EventType: "tool_execution",
			TenantID:  tenantID,
			Success:   err == nil,
			Metadata:  metadata,
		},
		ToolID:     toolID,
		Action:     action,
		Parameters: params,
		Result:     result,
		Duration:   duration.Milliseconds(),
	}

	if err != nil {
		event.Error = err.Error()
	}

	al.logger.Info("AUDIT: Tool execution", map[string]interface{}{
		"event_type":  event.EventType,
		"tenant_id":   event.TenantID,
		"tool_id":     event.ToolID,
		"action":      event.Action,
		"duration_ms": event.Duration,
		"success":     event.Success,
		"error":       event.Error,
	})
}

// LogToolHealthCheck logs a health check event
func (al *AuditLogger) LogToolHealthCheck(ctx context.Context, tenantID, toolID string, healthy bool, responseTime int, err error) {
	event := DynamicToolAuditEvent{
		AuditEvent: AuditEvent{
			Timestamp: time.Now(),
			EventType: "tool_health_check",
			TenantID:  tenantID,
			Success:   healthy,
			Metadata: map[string]interface{}{
				"response_time_ms": responseTime,
			},
		},
		ToolID: toolID,
	}

	if err != nil {
		event.Error = err.Error()
	}

	al.logger.Info("AUDIT: Tool health check", map[string]interface{}{
		"event_type":       event.EventType,
		"tenant_id":        event.TenantID,
		"tool_id":          event.ToolID,
		"healthy":          healthy,
		"response_time_ms": responseTime,
		"error":            event.Error,
	})
}

// LogToolCredentialUpdate logs a credential update event
func (al *AuditLogger) LogToolCredentialUpdate(ctx context.Context, tenantID, toolID, userID string, success bool, err error) {
	event := DynamicToolAuditEvent{
		AuditEvent: AuditEvent{
			Timestamp: time.Now(),
			EventType: "tool_credential_update",
			TenantID:  tenantID,
			UserID:    userID,
			Success:   success,
		},
		ToolID: toolID,
	}

	if err != nil {
		event.Error = err.Error()
	}

	al.logger.Info("AUDIT: Tool credential update", map[string]interface{}{
		"event_type": event.EventType,
		"tenant_id":  event.TenantID,
		"tool_id":    event.ToolID,
		"user_id":    event.UserID,
		"success":    event.Success,
		"error":      event.Error,
	})
}

// DynamicToolAuditMiddleware creates a middleware for auditing dynamic tool operations
func DynamicToolAuditMiddleware(auditLogger *AuditLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip if not a tool-related endpoint
		if !isToolEndpoint(c.Request.URL.Path) {
			c.Next()
			return
		}

		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// Extract context
		tenantID := c.GetString("tenant_id")
		userID := c.GetString("user_id")
		toolID := c.Param("toolId")
		action := c.Param("action")

		// Process request
		c.Next()

		// Log based on endpoint and result
		duration := time.Since(start)
		status := c.Writer.Status()
		success := status >= 200 && status < 300

		// Create base event
		event := DynamicToolAuditEvent{
			AuditEvent: AuditEvent{
				Timestamp: time.Now(),
				EventType: "tool_api_request",
				TenantID:  tenantID,
				UserID:    userID,
				Success:   success,
				IPAddress: c.ClientIP(),
				UserAgent: c.Request.UserAgent(),
				Metadata: map[string]interface{}{
					"method":     method,
					"path":       path,
					"status":     status,
					"request_id": c.GetString("request_id"),
				},
			},
			ToolID:   toolID,
			Action:   action,
			Duration: duration.Milliseconds(),
		}

		// Add any errors
		if len(c.Errors) > 0 {
			event.Error = c.Errors.String()
		}

		// Log the event
		auditLogger.logger.Info("AUDIT: Tool API request", map[string]interface{}{
			"event_type":  event.EventType,
			"tenant_id":   event.TenantID,
			"user_id":     event.UserID,
			"tool_id":     event.ToolID,
			"method":      method,
			"path":        path,
			"status":      status,
			"duration_ms": event.Duration,
			"success":     event.Success,
			"ip_address":  event.IPAddress,
		})
	}
}

// isToolEndpoint checks if the path is a tool-related endpoint
func isToolEndpoint(path string) bool {
	toolPaths := []string{
		"/api/v1/tools",
		"/api/v1/dynamic-tools",
		"/tools",
	}

	for _, prefix := range toolPaths {
		if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}
