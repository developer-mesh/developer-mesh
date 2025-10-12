package builtin

import (
	"context"
	"encoding/json"
	"fmt"
)

// handleSearch performs semantic search across context items
func (p *ContextProvider) handleSearch(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		ContextID string `json:"context_id"`
		Query     string `json:"query"`
		Limit     int    `json:"limit,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.ContextID == "" {
		return nil, fmt.Errorf("context_id is required")
	}
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// Default limit
	if params.Limit == 0 {
		params.Limit = 10
	}

	// Search requires Core Platform client (semantic search with embeddings)
	if p.coreClient == nil {
		return nil, fmt.Errorf("context_search requires Core Platform connection for semantic search")
	}

	result, err := p.coreClient.SearchContext(ctx, params.ContextID, params.Query, params.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search context via Core Platform: %w", err)
	}

	return result, nil
}

// handleCompact compacts context to reduce size using various strategies
func (p *ContextProvider) handleCompact(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		ContextID string `json:"context_id"`
		Strategy  string `json:"strategy,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.ContextID == "" {
		return nil, fmt.Errorf("context_id is required")
	}

	// Default strategy
	if params.Strategy == "" {
		params.Strategy = "summarize"
	}

	// Compaction is not yet implemented in Core Platform REST API
	// This will be available when the full semantic context manager is exposed
	return nil, fmt.Errorf("context_compact is not yet available via Core Platform REST API - use REST API /api/v1/contexts/:id/compact endpoint directly")
}

// handleDelete deletes a context session
func (p *ContextProvider) handleDelete(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		SessionID string `json:"session_id,omitempty"`
		ContextID string `json:"context_id,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Core Platform delete is not yet implemented via client
	// This will be available when the full semantic context manager is exposed
	if params.ContextID != "" {
		return nil, fmt.Errorf("context_delete for Core Platform contexts is not yet available - use REST API /api/v1/contexts/:id endpoint directly")
	}

	// Standalone mode: delete from in-memory storage
	if params.SessionID == "" {
		return nil, fmt.Errorf("either session_id or context_id is required")
	}

	p.sessionsMu.Lock()
	defer p.sessionsMu.Unlock()

	_, exists := p.sessions[params.SessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", params.SessionID)
	}

	delete(p.sessions, params.SessionID)

	return map[string]interface{}{
		"success":    true,
		"session_id": params.SessionID,
		"backend":    "standalone",
		"message":    "Session deleted successfully",
	}, nil
}
