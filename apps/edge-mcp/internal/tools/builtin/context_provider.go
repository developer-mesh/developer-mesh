package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/core"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
)

// CorePlatformClient defines the interface for Core Platform operations
// This avoids circular imports while allowing context provider to delegate
// Note: Matches the actual core.Client implementation
type CorePlatformClient interface {
	CreateSession(ctx context.Context, clientName string, clientType string) (string, error)
	GetContext(ctx context.Context, contextID string) (map[string]interface{}, error)
	UpdateContext(ctx context.Context, contextID string, update map[string]interface{}) error
	AppendContext(ctx context.Context, contextID string, data map[string]interface{}) error
	SearchContext(ctx context.Context, contextID string, query string, limit int) ([]interface{}, error)
}

// ContextProvider provides context management tools
type ContextProvider struct {
	sessions   map[string]*SessionContext
	sessionsMu sync.RWMutex
	coreClient CorePlatformClient // Optional: for Core Platform delegation
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

// NewContextProviderWithClient creates a context provider with Core Platform client
func NewContextProviderWithClient(coreClient *core.Client) *ContextProvider {
	return &ContextProvider{
		sessions:   make(map[string]*SessionContext),
		coreClient: coreClient,
	}
}

// GetDefinitions returns the tool definitions for context management
func (p *ContextProvider) GetDefinitions() []tools.ToolDefinition {
	return []tools.ToolDefinition{
		{
			Name:        "context_create",
			Description: "Create a new context session",
			Category:    string(tools.CategoryContext),
			Tags:        []string{string(tools.CapabilityWrite)},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"client_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the client creating the context",
					},
					"client_type": map[string]interface{}{
						"type":        "string",
						"description": "Type of client (e.g., 'claude-code', 'ide', 'agent')",
					},
				},
				"required": []string{"client_name"},
			},
			Handler: p.handleCreate,
		},
		{
			Name:        "context_list",
			Description: "List all active sessions",
			Category:    string(tools.CategoryContext),
			Tags:        []string{string(tools.CapabilityRead), string(tools.CapabilityList)},
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
			Category:    string(tools.CategoryContext),
			Tags:        []string{string(tools.CapabilityRead)},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Session identifier",
					},
					"context_id": map[string]interface{}{
						"type":        "string",
						"description": "Context identifier (for Core Platform delegation)",
					},
				},
				"required": []string{},
			},
			Handler: p.handleGet,
		},
		{
			Name:        "context_update",
			Description: "Update context for a session",
			Category:    string(tools.CategoryContext),
			Tags:        []string{string(tools.CapabilityWrite)},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Session identifier (for standalone mode)",
					},
					"context_id": map[string]interface{}{
						"type":        "string",
						"description": "Context identifier (for Core Platform delegation)",
					},
					"context": map[string]interface{}{
						"type":        "object",
						"description": "Context data to set",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Text content to add to context (for semantic operations)",
					},
					"role": map[string]interface{}{
						"type":        "string",
						"description": "Role of the content (user, assistant, system)",
						"enum":        []string{"user", "assistant", "system"},
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
				"required": []string{},
			},
			Handler: p.handleUpdate,
		},
		{
			Name:        "context_append",
			Description: "Append to context array or add new context values",
			Category:    string(tools.CategoryContext),
			Tags:        []string{string(tools.CapabilityWrite)},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Session identifier (for standalone mode)",
					},
					"context_id": map[string]interface{}{
						"type":        "string",
						"description": "Context identifier (for Core Platform delegation)",
					},
					"key": map[string]interface{}{
						"type":        "string",
						"description": "Context key to append to",
					},
					"value": map[string]interface{}{
						"description": "Value to append",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Text content to append (for semantic operations)",
					},
					"role": map[string]interface{}{
						"type":        "string",
						"description": "Role of the content (user, assistant, system)",
						"enum":        []string{"user", "assistant", "system"},
					},
					"idempotency_key": map[string]interface{}{
						"type":        "string",
						"description": "Unique key for idempotent requests (prevents duplicate appends)",
					},
				},
				"required": []string{},
			},
			Handler: p.handleAppend,
		},
		{
			Name:        "context_search",
			Description: "Perform semantic search across context items",
			Category:    string(tools.CategoryContext),
			Tags:        []string{string(tools.CapabilityRead), string(tools.CapabilitySearch)},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"context_id": map[string]interface{}{
						"type":        "string",
						"description": "Context identifier to search within",
					},
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query for semantic matching",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of results to return (default: 10)",
						"minimum":     1,
						"maximum":     100,
					},
				},
				"required": []string{"context_id", "query"},
			},
			Handler: p.handleSearch,
		},
		{
			Name:        "context_compact",
			Description: "Compact context to reduce size using various strategies",
			Category:    string(tools.CategoryContext),
			Tags:        []string{string(tools.CapabilityWrite)},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"context_id": map[string]interface{}{
						"type":        "string",
						"description": "Context identifier to compact",
					},
					"strategy": map[string]interface{}{
						"type":        "string",
						"description": "Compaction strategy to use",
						"enum":        []string{"summarize", "prune", "semantic", "sliding", "tool_clear"},
						"default":     "summarize",
					},
				},
				"required": []string{"context_id"},
			},
			Handler: p.handleCompact,
		},
		{
			Name:        "context_delete",
			Description: "Delete a context session",
			Category:    string(tools.CategoryContext),
			Tags:        []string{string(tools.CapabilityDelete)},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Session identifier (for standalone mode)",
					},
					"context_id": map[string]interface{}{
						"type":        "string",
						"description": "Context identifier (for Core Platform delegation)",
					},
				},
				"required": []string{},
			},
			Handler: p.handleDelete,
		},
	}
}

// handleCreate creates a new context session
func (p *ContextProvider) handleCreate(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		ClientName string `json:"client_name"`
		ClientType string `json:"client_type,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.ClientName == "" {
		return nil, fmt.Errorf("client_name is required")
	}

	// If Core Platform client is available, delegate to it
	if p.coreClient != nil {
		sessionID, err := p.coreClient.CreateSession(ctx, params.ClientName, params.ClientType)
		if err != nil {
			return nil, fmt.Errorf("failed to create context via Core Platform: %w", err)
		}

		// Get the actual context_id that was linked to this session
		// The Core Platform client stores this after session creation
		contextID := sessionID // Default to session_id if not available
		if coreClient, ok := p.coreClient.(interface{ GetCurrentContextID() string }); ok {
			if ctxID := coreClient.GetCurrentContextID(); ctxID != "" {
				contextID = ctxID
			}
		}

		return map[string]interface{}{
			"success":     true,
			"session_id":  sessionID,
			"context_id":  contextID,
			"client_name": params.ClientName,
			"client_type": params.ClientType,
			"backend":     "core_platform",
		}, nil
	}

	// Standalone mode: create in-memory session
	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())

	p.sessionsMu.Lock()
	p.sessions[sessionID] = &SessionContext{
		SessionID: sessionID,
		Context:   make(map[string]interface{}),
		UpdatedAt: time.Now(),
		Version:   1,
	}
	p.sessionsMu.Unlock()

	return map[string]interface{}{
		"success":     true,
		"session_id":  sessionID,
		"client_name": params.ClientName,
		"client_type": params.ClientType,
		"backend":     "standalone",
	}, nil
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
		SessionID      string                 `json:"session_id,omitempty"`
		ContextID      string                 `json:"context_id,omitempty"`
		Context        map[string]interface{} `json:"context,omitempty"`
		Content        string                 `json:"content,omitempty"`
		Role           string                 `json:"role,omitempty"`
		Merge          bool                   `json:"merge,omitempty"`
		IdempotencyKey string                 `json:"idempotency_key,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return rb.Error(fmt.Errorf("invalid parameters: %w", err)), nil
	}

	// If Core Platform client is available and context_id provided, delegate to it
	if p.coreClient != nil && params.ContextID != "" {
		// Build update map for semantic context
		update := make(map[string]interface{})
		if params.Content != "" {
			update["content"] = params.Content
			if params.Role != "" {
				update["role"] = params.Role
			} else {
				update["role"] = "user"
			}
		} else if params.Context != nil {
			update = params.Context
		} else {
			return rb.Error(fmt.Errorf("either 'content' or 'context' is required")), nil
		}

		err := p.coreClient.UpdateContext(ctx, params.ContextID, update)
		if err != nil {
			return rb.Error(fmt.Errorf("failed to update context via Core Platform: %w", err)), nil
		}

		result := map[string]interface{}{
			"success":           true,
			"context_id":        params.ContextID,
			"backend":           "core_platform",
			"embedding_enabled": true,
		}

		if params.IdempotencyKey != "" {
			response := rb.SuccessWithMetadata(
				result,
				&ResponseMetadata{
					IdempotencyKey:  params.IdempotencyKey,
					RateLimitStatus: rateLimitStatus,
				},
				"context_get", "context_search",
			)
			StoreIdempotentResponse(params.IdempotencyKey, response)
			return response, nil
		}

		return rb.SuccessWithMetadata(
			result,
			&ResponseMetadata{
				RateLimitStatus: rateLimitStatus,
			},
			"context_get", "context_search",
		), nil
	}

	// Standalone mode: use in-memory storage
	if params.SessionID == "" {
		return rb.Error(fmt.Errorf("session_id is required for standalone mode")), nil
	}
	if params.Context == nil {
		return rb.Error(fmt.Errorf("context is required for standalone mode")), nil
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
		"backend":      "standalone",
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
		SessionID string `json:"session_id,omitempty"`
		ContextID string `json:"context_id,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// If Core Platform client is available and context_id provided, delegate to it
	if p.coreClient != nil && params.ContextID != "" {
		result, err := p.coreClient.GetContext(ctx, params.ContextID)
		if err != nil {
			return nil, fmt.Errorf("failed to get context via Core Platform: %w", err)
		}

		return result, nil
	}

	// Standalone mode: use in-memory storage
	if params.SessionID == "" {
		return nil, fmt.Errorf("session_id is required for standalone mode")
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
