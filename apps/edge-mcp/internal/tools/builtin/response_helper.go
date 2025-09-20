package builtin

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ResponseBuilder helps create standardized responses
type ResponseBuilder struct {
	startTime      time.Time
	requestID      string
	idempotencyKey string
}

// NewResponseBuilder creates a new response builder
func NewResponseBuilder() *ResponseBuilder {
	return &ResponseBuilder{
		startTime: time.Now(),
		requestID: uuid.New().String(),
	}
}

// WithIdempotencyKey sets the idempotency key
func (rb *ResponseBuilder) WithIdempotencyKey(key string) *ResponseBuilder {
	rb.idempotencyKey = key
	return rb
}

// Success creates a successful response with metadata
func (rb *ResponseBuilder) Success(data interface{}, nextSteps ...string) *StandardResponse {
	duration := time.Since(rb.startTime)

	return &StandardResponse{
		Success: true,
		Data:    data,
		Metadata: &ResponseMetadata{
			RequestID:      rb.requestID,
			Timestamp:      rb.startTime,
			Duration:       fmt.Sprintf("%dms", duration.Milliseconds()),
			IdempotencyKey: rb.idempotencyKey,
		},
		NextSteps: nextSteps,
	}
}

// Error creates an error response with metadata
func (rb *ResponseBuilder) Error(err error) *StandardResponse {
	duration := time.Since(rb.startTime)

	return &StandardResponse{
		Success: false,
		Error:   err.Error(),
		Metadata: &ResponseMetadata{
			RequestID:      rb.requestID,
			Timestamp:      rb.startTime,
			Duration:       fmt.Sprintf("%dms", duration.Milliseconds()),
			IdempotencyKey: rb.idempotencyKey,
		},
	}
}

// SuccessWithMetadata creates a successful response with custom metadata
func (rb *ResponseBuilder) SuccessWithMetadata(data interface{}, metadata *ResponseMetadata, nextSteps ...string) *StandardResponse {
	duration := time.Since(rb.startTime)

	// Merge custom metadata with default values
	if metadata == nil {
		metadata = &ResponseMetadata{}
	}
	metadata.RequestID = rb.requestID
	metadata.Timestamp = rb.startTime
	metadata.Duration = fmt.Sprintf("%dms", duration.Milliseconds())
	if metadata.IdempotencyKey == "" {
		metadata.IdempotencyKey = rb.idempotencyKey
	}

	return &StandardResponse{
		Success:   true,
		Data:      data,
		Metadata:  metadata,
		NextSteps: nextSteps,
	}
}

// ErrorWithMetadata creates an error response with custom metadata
func (rb *ResponseBuilder) ErrorWithMetadata(err error, metadata *ResponseMetadata) *StandardResponse {
	duration := time.Since(rb.startTime)

	// Merge custom metadata with default values
	if metadata == nil {
		metadata = &ResponseMetadata{}
	}
	metadata.RequestID = rb.requestID
	metadata.Timestamp = rb.startTime
	metadata.Duration = fmt.Sprintf("%dms", duration.Milliseconds())
	if metadata.IdempotencyKey == "" {
		metadata.IdempotencyKey = rb.idempotencyKey
	}

	return &StandardResponse{
		Success:  false,
		Error:    err.Error(),
		Metadata: metadata,
	}
}

// SuggestNextTools returns appropriate next tool suggestions based on the current tool
func SuggestNextTools(currentTool string, context map[string]interface{}) []string {
	suggestions := make([]string, 0)

	switch currentTool {
	case "workflow_create":
		suggestions = append(suggestions, "workflow_execute", "workflow_get")

	case "workflow_execute":
		suggestions = append(suggestions, "workflow_execution_get", "workflow_execution_list")

	case "task_create":
		suggestions = append(suggestions, "task_assign", "task_get", "task_list")

	case "task_assign":
		suggestions = append(suggestions, "task_complete", "task_get", "agent_status")

	case "agent_heartbeat":
		suggestions = append(suggestions, "agent_status", "agent_list")

	case "context_update":
		suggestions = append(suggestions, "context_get", "context_append")

	case "context_append":
		suggestions = append(suggestions, "context_get", "context_list")

	case "workflow_list":
		if count, ok := context["count"].(int); ok && count > 0 {
			suggestions = append(suggestions, "workflow_get", "workflow_execute")
		} else {
			suggestions = append(suggestions, "workflow_create")
		}

	case "task_list":
		if count, ok := context["count"].(int); ok && count > 0 {
			suggestions = append(suggestions, "task_get", "task_assign", "task_get_batch")
		} else {
			suggestions = append(suggestions, "task_create")
		}

	case "agent_list":
		if count, ok := context["count"].(int); ok && count > 0 {
			suggestions = append(suggestions, "agent_status", "task_assign")
		} else {
			suggestions = append(suggestions, "agent_heartbeat")
		}

	case "workflow_execution_list":
		suggestions = append(suggestions, "workflow_execution_get", "workflow_cancel")

	case "task_get_batch":
		suggestions = append(suggestions, "task_assign", "task_complete")

	case "task_complete":
		suggestions = append(suggestions, "task_list", "task_create")

	case "workflow_cancel":
		suggestions = append(suggestions, "workflow_execution_get", "workflow_list")
	}

	return suggestions
}

// GetRateLimitForTool returns rate limit information for a specific tool
func GetRateLimitForTool(toolName string) *RateLimitInfo {
	// Default rate limits - in production these would be configurable
	rateLimits := map[string]*RateLimitInfo{
		"agent_heartbeat": {
			RequestsPerMinute: 600, // High frequency for heartbeats
			BurstSize:         20,
			Description:       "Higher limit for maintaining agent liveness",
		},
		"workflow_execute": {
			RequestsPerMinute: 60,
			BurstSize:         10,
			Description:       "Moderate limit to prevent workflow flooding",
		},
		"task_create": {
			RequestsPerMinute: 120,
			BurstSize:         20,
			Description:       "Standard limit for task creation",
		},
		"task_get_batch": {
			RequestsPerMinute: 30,
			BurstSize:         5,
			Description:       "Lower limit for batch operations",
		},
		"context_update": {
			RequestsPerMinute: 300,
			BurstSize:         50,
			Description:       "High limit for context management",
		},
	}

	// Default rate limit for tools not explicitly configured
	defaultLimit := &RateLimitInfo{
		RequestsPerMinute: 100,
		BurstSize:         10,
		Description:       "Standard rate limit",
	}

	if limit, exists := rateLimits[toolName]; exists {
		return limit
	}

	return defaultLimit
}

// GetCapabilityLimits returns documented limits for a tool
func GetCapabilityLimits(toolName string) map[string]interface{} {
	limits := make(map[string]interface{})

	switch toolName {
	case "task_get_batch":
		limits["max_tasks"] = 50
		limits["description"] = "Maximum 50 tasks per batch request"

	case "agent_list", "workflow_list", "task_list", "context_list":
		limits["max_limit"] = 100
		limits["default_limit"] = 50
		limits["max_offset"] = 10000
		limits["description"] = "Pagination limits for list operations"

	case "agent_heartbeat":
		limits["timeout_minutes"] = 5
		limits["description"] = "Agent marked offline after 5 minutes without heartbeat"

	case "workflow_execute":
		limits["simulation_delay_ms"] = 100
		limits["description"] = "Simulated execution completes after 100ms"

	case "context_append":
		limits["max_array_size"] = 1000
		limits["description"] = "Maximum array size for appended values"

	case "workflow_create":
		limits["max_steps"] = 100
		limits["description"] = "Maximum 100 steps per workflow"
	}

	return limits
}
