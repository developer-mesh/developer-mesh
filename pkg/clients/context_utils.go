package clients

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ContextKey is a type for context keys
type ContextKey string

const (
	// ContextKeyCorrelationID is the key for correlation ID in context
	ContextKeyCorrelationID ContextKey = "correlation_id"
	// ContextKeyOperation is the key for operation name in context
	ContextKeyOperation ContextKey = "operation"
	// ContextKeyTenantID is the key for tenant ID in context
	ContextKeyTenantID ContextKey = "tenant_id"
	// ContextKeyAgentID is the key for agent ID in context
	ContextKeyAgentID ContextKey = "agent_id"
)

// TimeoutConfig defines timeout settings for different operations
type TimeoutConfig struct {
	// Default timeout for general operations
	Default time.Duration
	// HealthCheck timeout for health check operations
	HealthCheck time.Duration
	// ListTools timeout for listing tools
	ListTools time.Duration
	// ExecuteTool timeout for tool execution
	ExecuteTool time.Duration
	// LongRunning timeout for long-running operations
	LongRunning time.Duration
}

// DefaultTimeoutConfig returns default timeout configuration
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		Default:     30 * time.Second,
		HealthCheck: 5 * time.Second,
		ListTools:   10 * time.Second,
		ExecuteTool: 60 * time.Second,
		LongRunning: 5 * time.Minute,
	}
}

// WithCorrelationID adds a correlation ID to the context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	if correlationID == "" {
		correlationID = uuid.New().String()
	}
	return context.WithValue(ctx, ContextKeyCorrelationID, correlationID)
}

// GetCorrelationID retrieves the correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if val := ctx.Value(ContextKeyCorrelationID); val != nil {
		if id, ok := val.(string); ok {
			return id
		}
	}
	return uuid.New().String()
}

// WithOperation adds an operation name to the context
func WithOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, ContextKeyOperation, operation)
}

// GetOperation retrieves the operation name from context
func GetOperation(ctx context.Context) string {
	if val := ctx.Value(ContextKeyOperation); val != nil {
		if op, ok := val.(string); ok {
			return op
		}
	}
	return "unknown"
}

// WithTimeout creates a context with timeout based on operation type
func WithTimeout(ctx context.Context, operation string, config TimeoutConfig) (context.Context, context.CancelFunc) {
	var timeout time.Duration

	switch operation {
	case "health.check":
		timeout = config.HealthCheck
	case "tool.list":
		timeout = config.ListTools
	case "tool.execute":
		timeout = config.ExecuteTool
	case "long.running":
		timeout = config.LongRunning
	default:
		timeout = config.Default
	}

	return context.WithTimeout(ctx, timeout)
}

// EnrichContext adds standard metadata to the context
func EnrichContext(ctx context.Context, tenantID, agentID, operation string) context.Context {
	ctx = WithCorrelationID(ctx, GetCorrelationID(ctx))
	ctx = WithOperation(ctx, operation)

	if tenantID != "" {
		ctx = context.WithValue(ctx, ContextKeyTenantID, tenantID)
	}
	if agentID != "" {
		ctx = context.WithValue(ctx, ContextKeyAgentID, agentID)
	}

	return ctx
}

// ExtractContextMetadata extracts all relevant metadata from context
func ExtractContextMetadata(ctx context.Context) map[string]string {
	metadata := make(map[string]string)

	if correlationID := GetCorrelationID(ctx); correlationID != "" {
		metadata["correlation_id"] = correlationID
	}

	if operation := GetOperation(ctx); operation != "" {
		metadata["operation"] = operation
	}

	if val := ctx.Value(ContextKeyTenantID); val != nil {
		if tenantID, ok := val.(string); ok && tenantID != "" {
			metadata["tenant_id"] = tenantID
		}
	}

	if val := ctx.Value(ContextKeyAgentID); val != nil {
		if agentID, ok := val.(string); ok && agentID != "" {
			metadata["agent_id"] = agentID
		}
	}

	// Check if context has a deadline
	if deadline, ok := ctx.Deadline(); ok {
		metadata["deadline"] = deadline.Format(time.RFC3339)
		metadata["timeout_remaining"] = time.Until(deadline).String()
	}

	return metadata
}
