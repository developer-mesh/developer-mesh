package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
)

// ContextProvider provides context management tools
type ContextProvider struct {
	sessions   map[string]*SessionContext
	sessionsMu sync.RWMutex
}

// SessionContext represents context for a session
type SessionContext struct {
	SessionID string                 `json:"session_id"`
	Context   map[string]interface{} `json:"context"`
	UpdatedAt time.Time              `json:"updated_at"`
	Version   int                    `json:"version"`
}

// NewContextProvider creates a new context provider
func NewContextProvider() *ContextProvider {
	return &ContextProvider{
		sessions: make(map[string]*SessionContext),
	}
}

// GetDefinitions returns the tool definitions for context management
func (p *ContextProvider) GetDefinitions() []tools.ToolDefinition {
	return []tools.ToolDefinition{
		{
			Name:        "context_list",
			Description: "List all active sessions",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of sessions to return (default: 50, max: 100)",
						"minimum":     1,
						"maximum":     100,
					},
					"offset": map[string]interface{}{
						"type":        "integer",
						"description": "Number of sessions to skip for pagination (default: 0)",
						"minimum":     0,
					},
					"sort_by": map[string]interface{}{
						"type":        "string",
						"description": "Field to sort by",
						"enum":        []string{"session_id", "version", "updated_at"},
					},
					"sort_order": map[string]interface{}{
						"type":        "string",
						"description": "Sort order (default: desc)",
						"enum":        []string{"asc", "desc"},
					},
				},
			},
			Handler: p.handleList,
		},
		{
			Name:        "context_get",
			Description: "Get current context for a session",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Session identifier",
					},
				},
				"required": []string{"session_id"},
			},
			Handler: p.handleGet,
		},
		{
			Name:        "context_update",
			Description: "Update context for a session",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Session identifier",
					},
					"context": map[string]interface{}{
						"type":        "object",
						"description": "Context data to set",
					},
					"merge": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, merge with existing context; if false, replace",
						"default":     false,
					},
					"idempotency_key": map[string]interface{}{
						"type":        "string",
						"description": "Unique key for idempotent requests (prevents duplicate updates)",
					},
				},
				"required": []string{"session_id", "context"},
			},
			Handler: p.handleUpdate,
		},
		{
			Name:        "context_append",
			Description: "Append to context array or add new context values",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Session identifier",
					},
					"key": map[string]interface{}{
						"type":        "string",
						"description": "Context key to append to",
					},
					"value": map[string]interface{}{
						"description": "Value to append",
					},
					"idempotency_key": map[string]interface{}{
						"type":        "string",
						"description": "Unique key for idempotent requests (prevents duplicate appends)",
					},
				},
				"required": []string{"session_id", "key", "value"},
			},
			Handler: p.handleAppend,
		},
	}
}

// handleUpdate updates the context for a session
func (p *ContextProvider) handleUpdate(ctx context.Context, args json.RawMessage) (interface{}, error) {
	rb := NewResponseBuilder()

	// Check rate limit
	allowed, rateLimitStatus := GlobalRateLimiter.CheckAndConsume("context_update")
	if !allowed {
		return rb.ErrorWithMetadata(
			fmt.Errorf("rate limit exceeded, please retry after %v", time.Until(rateLimitStatus.Reset)),
			&ResponseMetadata{
				RateLimitStatus: rateLimitStatus,
			},
		), nil
	}

	var params struct {
		SessionID      string                 `json:"session_id"`
		Context        map[string]interface{} `json:"context"`
		Merge          bool                   `json:"merge,omitempty"`
		IdempotencyKey string                 `json:"idempotency_key,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return rb.Error(fmt.Errorf("invalid parameters: %w", err)), nil
	}

	if params.SessionID == "" {
		return rb.Error(fmt.Errorf("session_id is required")), nil
	}
	if params.Context == nil {
		return rb.Error(fmt.Errorf("context is required")), nil
	}

	// Check idempotency
	if params.IdempotencyKey != "" {
		if cachedResponse, found := CheckIdempotency(params.IdempotencyKey); found {
			return cachedResponse, nil
		}
	}

	p.sessionsMu.Lock()
	defer p.sessionsMu.Unlock()

	session, exists := p.sessions[params.SessionID]
	if !exists {
		session = &SessionContext{
			SessionID: params.SessionID,
			Context:   make(map[string]interface{}),
			Version:   0,
		}
		p.sessions[params.SessionID] = session
	}

	// Update or merge context
	if params.Merge && session.Context != nil {
		// Merge new context with existing
		for key, value := range params.Context {
			session.Context[key] = value
		}
	} else {
		// Replace context entirely
		session.Context = params.Context
	}

	session.UpdatedAt = time.Now()
	session.Version++

	result := map[string]interface{}{
		"session_id":   session.SessionID,
		"version":      session.Version,
		"updated_at":   session.UpdatedAt.Format(time.RFC3339),
		"context_size": len(session.Context),
		"message":      fmt.Sprintf("Context updated for session %s", params.SessionID),
	}

	// Store for idempotency
	if params.IdempotencyKey != "" {
		response := rb.SuccessWithMetadata(
			result,
			&ResponseMetadata{
				IdempotencyKey:  params.IdempotencyKey,
				RateLimitStatus: rateLimitStatus,
			},
			"context_get", "context_append",
		)
		StoreIdempotentResponse(params.IdempotencyKey, response)
		return response, nil
	}

	return rb.SuccessWithMetadata(
		result,
		&ResponseMetadata{
			RateLimitStatus: rateLimitStatus,
		},
		"context_get", "context_append",
	), nil
}

// handleAppend appends to context
func (p *ContextProvider) handleAppend(ctx context.Context, args json.RawMessage) (interface{}, error) {
	rb := NewResponseBuilder()

	// Check rate limit
	allowed, rateLimitStatus := GlobalRateLimiter.CheckAndConsume("context_append")
	if !allowed {
		return rb.ErrorWithMetadata(
			fmt.Errorf("rate limit exceeded, please retry after %v", time.Until(rateLimitStatus.Reset)),
			&ResponseMetadata{
				RateLimitStatus: rateLimitStatus,
			},
		), nil
	}

	var params struct {
		SessionID      string      `json:"session_id"`
		Key            string      `json:"key"`
		Value          interface{} `json:"value"`
		IdempotencyKey string      `json:"idempotency_key,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	if params.Key == "" {
		return nil, fmt.Errorf("key is required")
	}
	if params.Value == nil {
		return nil, fmt.Errorf("value is required")
	}

	p.sessionsMu.Lock()
	defer p.sessionsMu.Unlock()

	session, exists := p.sessions[params.SessionID]
	if !exists {
		session = &SessionContext{
			SessionID: params.SessionID,
			Context:   make(map[string]interface{}),
			Version:   0,
		}
		p.sessions[params.SessionID] = session
	}

	// Handle appending based on existing value type
	existingValue, hasKey := session.Context[params.Key]

	if !hasKey {
		// Key doesn't exist, create new entry
		session.Context[params.Key] = params.Value
	} else {
		// Key exists, append based on type
		switch existing := existingValue.(type) {
		case []interface{}:
			// Existing is array, append to it
			session.Context[params.Key] = append(existing, params.Value)
		case string:
			// Existing is string, create array with both
			session.Context[params.Key] = []interface{}{existing, params.Value}
		default:
			// For other types, create array with both values
			session.Context[params.Key] = []interface{}{existing, params.Value}
		}
	}

	session.UpdatedAt = time.Now()
	session.Version++

	return map[string]interface{}{
		"session_id": session.SessionID,
		"key":        params.Key,
		"version":    session.Version,
		"updated_at": session.UpdatedAt.Format(time.RFC3339),
		"message":    fmt.Sprintf("Value appended to context key '%s'", params.Key),
	}, nil
}

// handleList returns a list of all active sessions
func (p *ContextProvider) handleList(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Limit     int    `json:"limit,omitempty"`
		Offset    int    `json:"offset,omitempty"`
		SortBy    string `json:"sort_by,omitempty"`
		SortOrder string `json:"sort_order,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Set defaults
	if params.Limit <= 0 {
		params.Limit = 50
	} else if params.Limit > 100 {
		params.Limit = 100
	}
	if params.Offset < 0 {
		params.Offset = 0
	}
	if params.SortOrder == "" {
		params.SortOrder = "desc" // Default to desc (most recent first)
	}

	p.sessionsMu.RLock()
	defer p.sessionsMu.RUnlock()

	sessions := make([]map[string]interface{}, 0, len(p.sessions))
	for _, session := range p.sessions {
		sessions = append(sessions, map[string]interface{}{
			"session_id":   session.SessionID,
			"version":      session.Version,
			"updated_at":   session.UpdatedAt.Format(time.RFC3339),
			"context_size": len(session.Context),
		})
	}

	// Sort sessions if requested
	if params.SortBy != "" {
		sort.Slice(sessions, func(i, j int) bool {
			var less bool
			switch params.SortBy {
			case "session_id":
				less = sessions[i]["session_id"].(string) < sessions[j]["session_id"].(string)
			case "version":
				less = sessions[i]["version"].(int) < sessions[j]["version"].(int)
			case "updated_at":
				less = sessions[i]["updated_at"].(string) < sessions[j]["updated_at"].(string)
			default:
				return false
			}
			if params.SortOrder == "desc" {
				return !less
			}
			return less
		})
	}

	// Apply pagination
	totalCount := len(sessions)
	start := params.Offset
	end := params.Offset + params.Limit
	if start > totalCount {
		start = totalCount
	}
	if end > totalCount {
		end = totalCount
	}
	paginatedSessions := sessions[start:end]

	return map[string]interface{}{
		"sessions":    paginatedSessions,
		"count":       len(paginatedSessions),
		"total_count": totalCount,
		"offset":      params.Offset,
		"limit":       params.Limit,
	}, nil
}

// handleGet retrieves the current context for a session
func (p *ContextProvider) handleGet(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		SessionID string `json:"session_id"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	p.sessionsMu.RLock()
	session, exists := p.sessions[params.SessionID]
	p.sessionsMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", params.SessionID)
	}

	// Return a copy to prevent external modifications
	contextCopy := make(map[string]interface{})
	for k, v := range session.Context {
		contextCopy[k] = v
	}

	return map[string]interface{}{
		"session_id": session.SessionID,
		"context":    contextCopy,
		"version":    session.Version,
		"updated_at": session.UpdatedAt.Format(time.RFC3339),
	}, nil
}
