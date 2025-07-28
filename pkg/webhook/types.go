package webhook

import (
	"time"
)

// WebhookEvent represents a webhook event
type WebhookEvent struct {
	EventId        string                 `json:"event_id"`
	TenantId       string                 `json:"tenant_id"`
	ToolId         string                 `json:"tool_id"`
	ToolType       string                 `json:"tool_type"`
	EventType      string                 `json:"event_type"`
	Version        int32                  `json:"version"`
	Timestamp      time.Time              `json:"timestamp"`
	Payload        map[string]interface{} `json:"payload"`
	Headers        map[string]string      `json:"headers"`
	ProcessingInfo *ProcessingInfo        `json:"processing_info,omitempty"`
	SecurityInfo   *SecurityInfo          `json:"security_info,omitempty"`
}

// ProcessingInfo contains processing metadata
type ProcessingInfo struct {
	RetryCount       int32             `json:"retry_count"`
	ProcessingTimeMs int64             `json:"processing_time_ms"`
	Status           string            `json:"status"`
	Error            string            `json:"error,omitempty"`
	GeneratedContext *GeneratedContext `json:"generated_context,omitempty"`
}

// GeneratedContext represents AI-generated context
type GeneratedContext struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SecurityInfo contains security-related information
type SecurityInfo struct {
	Signature    string `json:"signature"`
	ValidatedAt  int64  `json:"validated_at"`
	IsValid      bool   `json:"is_valid"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// DeduplicationInfo contains deduplication metadata
type DeduplicationInfo struct {
	MessageId       string `json:"message_id"`
	FirstSeenAt     int64  `json:"first_seen_at"`
	OccurrenceCount int32  `json:"occurrence_count"`
}

// Message represents a conversation message
type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// AgentContext represents the context data for an AI agent
type AgentContext struct {
	EventID             string                 `json:"event_id"`
	TenantID            string                 `json:"tenant_id"`
	ToolID              string                 `json:"tool_id"`
	ConversationHistory []Message              `json:"conversation_history"`
	Variables           map[string]interface{} `json:"variables"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
	AccessedAt          time.Time              `json:"accessed_at,omitempty"`
}

// ContextSearchCriteria defines search criteria for contexts
type ContextSearchCriteria struct {
	TenantID  string
	ToolID    string
	StartTime time.Time
	EndTime   time.Time
	Tags      []string
	MinScore  float64
}
