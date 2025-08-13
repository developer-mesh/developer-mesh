package websocket

import (
	"context"

	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/core"

	"github.com/developer-mesh/developer-mesh/pkg/models"
)

// contextManagerAdapter adapts core.ContextManagerInterface to websocket.ContextManager
type contextManagerAdapter struct {
	coreManager core.ContextManagerInterface
}

// NewContextManagerAdapter creates a new adapter
func NewContextManagerAdapter(coreManager core.ContextManagerInterface) ContextManager {
	return &contextManagerAdapter{
		coreManager: coreManager,
	}
}

// GetContext implements websocket.ContextManager
func (a *contextManagerAdapter) GetContext(ctx context.Context, contextID string) (*models.Context, error) {
	return a.coreManager.GetContext(ctx, contextID)
}

// UpdateContext implements websocket.ContextManager with simplified signature
func (a *contextManagerAdapter) UpdateContext(ctx context.Context, contextID string, content string) (*models.Context, error) {
	// Create a context update with the new content
	updateData := &models.Context{
		Content: []models.ContextItem{
			{
				Content: content,
				Role:    "user",
			},
		},
	}

	// Use default options
	options := &models.ContextUpdateOptions{
		Truncate: false,
	}

	return a.coreManager.UpdateContext(ctx, contextID, updateData, options)
}

// TruncateContext implements websocket.ContextManager
func (a *contextManagerAdapter) TruncateContext(ctx context.Context, contextID string, maxTokens int, preserveRecent bool) (*TruncatedContext, int, error) {
	// Get current context
	context, err := a.coreManager.GetContext(ctx, contextID)
	if err != nil {
		return nil, 0, err
	}

	// Calculate tokens to remove
	removedTokens := 0
	if context.CurrentTokens > maxTokens {
		removedTokens = context.CurrentTokens - maxTokens
	}

	// Update context with truncation
	updateData := &models.Context{
		Content: context.Content, // This would be truncated in a real implementation
	}

	options := &models.ContextUpdateOptions{
		Truncate:         true,
		TruncateStrategy: "keep_recent",
	}

	updatedContext, err := a.coreManager.UpdateContext(ctx, contextID, updateData, options)
	if err != nil {
		return nil, 0, err
	}

	return &TruncatedContext{
		ID:         updatedContext.ID,
		TokenCount: maxTokens,
	}, removedTokens, nil
}

// CreateContext implements websocket.ContextManager
func (a *contextManagerAdapter) CreateContext(ctx context.Context, agentID, tenantID, name, content, modelID string) (*models.Context, error) {
	// Create a new context
	newContext := &models.Context{
		Name:     name,
		AgentID:  agentID,
		TenantID: tenantID,
		ModelID:  modelID,
		Content: []models.ContextItem{
			{
				Content: content,
				Role:    "system",
			},
		},
	}

	return a.coreManager.CreateContext(ctx, newContext)
}

// AppendToContext appends content to an existing context
func (a *contextManagerAdapter) AppendToContext(ctx context.Context, contextID string, content string) (*models.Context, error) {
	// Get current context
	currentContext, err := a.coreManager.GetContext(ctx, contextID)
	if err != nil {
		return nil, err
	}

	// Append new content
	currentContext.Content = append(currentContext.Content, models.ContextItem{
		Content: content,
		Role:    "user",
	})

	// Update context
	options := &models.ContextUpdateOptions{
		Truncate: false,
	}

	return a.coreManager.UpdateContext(ctx, contextID, currentContext, options)
}

// GetContextStats returns statistics for a context
func (a *contextManagerAdapter) GetContextStats(ctx context.Context, contextID string) (*ContextStats, error) {
	// Get context to calculate stats
	context, err := a.coreManager.GetContext(ctx, contextID)
	if err != nil {
		return nil, err
	}

	// Calculate stats
	messageCount := len(context.Content)
	toolInvocations := 0

	// Count tool invocations (simplified - would parse content in real implementation)
	for _, item := range context.Content {
		if item.Role == "tool" {
			toolInvocations++
		}
	}

	return &ContextStats{
		TotalTokens:     context.CurrentTokens,
		MessageCount:    messageCount,
		ToolInvocations: toolInvocations,
		CreatedAt:       context.CreatedAt,
		LastAccessed:    context.UpdatedAt,
	}, nil
}
