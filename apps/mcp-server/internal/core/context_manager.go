package core

import (
	"context"
	"errors"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/models"
)

// ErrContextNotFound is returned when a context is not found
var ErrContextNotFound = errors.New("context not found")

// ContextManager is a wrapper around the internal context manager
// This allows us to test core functionality without directly exposing internal implementations
type ContextManager struct {
	db          interface{}
	cache       cache.Cache
	subscribers map[string][]func(models.Event)
}

// TruncateStrategy defines the strategy for truncating a context
type TruncateStrategy string

// NewContextManager creates a new context manager
func NewContextManager(db interface{}, cache cache.Cache) *ContextManager {
	return &ContextManager{
		db:          db,
		cache:       cache,
		subscribers: make(map[string][]func(models.Event)),
	}
}

// CreateContext creates a new context
func (cm *ContextManager) CreateContext(ctx context.Context, contextData *models.Context) (*models.Context, error) {
	// Use the real context manager if available, otherwise just return the context data
	// with some minimal processing for tests

	// For test purposes, ensure ID exists
	if contextData.ID == "" {
		contextData.ID = "test-id"
	}

	// Set timestamps
	now := time.Now()
	contextData.CreatedAt = now
	contextData.UpdatedAt = now

	// Set default values if needed
	if contextData.MaxTokens == 0 {
		contextData.MaxTokens = 4000
	}

	// Initialize content if nil
	if contextData.Content == nil {
		contextData.Content = []models.ContextItem{}
	}

	// Calculate tokens
	if contextData.CurrentTokens == 0 && len(contextData.Content) > 0 {
		for _, item := range contextData.Content {
			contextData.CurrentTokens += item.Tokens
		}
	}

	// If mockDB is provided, simulate database operations
	if mockDB, ok := cm.db.(*MockDB); ok {
		err := mockDB.CreateContext(ctx, contextData)
		if err != nil {
			return nil, err
		}
	}

	// Cache the context
	if cm.cache != nil {
		_ = cm.cache.Set(ctx, "context:"+contextData.ID, contextData, 24*time.Hour)
	}

	return contextData, nil
}

// GetContext retrieves a context by ID
func (cm *ContextManager) GetContext(ctx context.Context, contextID string) (*models.Context, error) {
	// Try to get from cache first
	if cm.cache != nil {
		var contextData models.Context
		err := cm.cache.Get(ctx, "context:"+contextID, &contextData)
		if err == nil {
			return &contextData, nil
		}
	}

	// If mockDB is provided, get from database
	if mockDB, ok := cm.db.(*MockDB); ok {
		return mockDB.GetContext(ctx, contextID)
	}

	// For tests that don't provide mock implementations, return a dummy context
	if mockDatabase, ok := cm.db.(*MockDB); ok {
		return mockDatabase.GetContext(ctx, contextID)
	}

	return nil, ErrContextNotFound
}

// UpdateContext updates an existing context
func (cm *ContextManager) UpdateContext(ctx context.Context, contextID string, updateData *models.Context, options *models.ContextUpdateOptions) (*models.Context, error) {
	// Get existing context
	existingContext, err := cm.GetContext(ctx, contextID)
	if err != nil {
		return nil, err
	}

	// Update fields
	if updateData.AgentID != "" {
		existingContext.AgentID = updateData.AgentID
	}

	if updateData.SessionID != "" {
		existingContext.SessionID = updateData.SessionID
	}

	// Update metadata
	if updateData.Metadata != nil {
		if existingContext.Metadata == nil {
			existingContext.Metadata = make(map[string]interface{})
		}

		for k, v := range updateData.Metadata {
			existingContext.Metadata[k] = v
		}
	}

	// Handle content updates
	if updateData.Content != nil {
		// If ReplaceContent is true, replace the entire content
		if options != nil && options.ReplaceContent {
			existingContext.Content = updateData.Content

			// Recalculate token count
			existingContext.CurrentTokens = 0
			for _, item := range existingContext.Content {
				existingContext.CurrentTokens += item.Tokens
			}
		} else {
			// Add new content items
			for _, item := range updateData.Content {
				existingContext.Content = append(existingContext.Content, item)
				existingContext.CurrentTokens += item.Tokens
			}
		}
	}
	// Update timestamp
	existingContext.UpdatedAt = time.Now()

	// Check if context needs truncation
	if options != nil && options.Truncate && existingContext.CurrentTokens > existingContext.MaxTokens {
		// For tests, simulate truncation by removing oldest items
		if existingContext.CurrentTokens > existingContext.MaxTokens {
			tokensToRemove := existingContext.CurrentTokens - existingContext.MaxTokens
			removed := 0
			removeCount := 0

			for i := 0; i < len(existingContext.Content) && removed < tokensToRemove; i++ {
				removed += existingContext.Content[i].Tokens
				removeCount++
			}

			if removeCount > 0 {
				existingContext.Content = existingContext.Content[removeCount:]
				existingContext.CurrentTokens -= removed
			}
		}
	}

	// If mockDB is provided, update in database
	if mockDB, ok := cm.db.(*MockDB); ok {
		err := mockDB.UpdateContext(ctx, existingContext)
		if err != nil {
			return nil, err
		}
	}

	// Cache the updated context
	if cm.cache != nil {
		_ = cm.cache.Set(ctx, "context:"+contextID, existingContext, 24*time.Hour)
	}

	return existingContext, nil
}

// DeleteContext deletes a context
func (cm *ContextManager) DeleteContext(ctx context.Context, contextID string) error {
	// Get context to ensure it exists
	_, err := cm.GetContext(ctx, contextID)
	if err != nil {
		return err
	}

	// If mockDB is provided, delete from database
	if mockDB, ok := cm.db.(*MockDB); ok {
		err := mockDB.DeleteContext(ctx, contextID)
		if err != nil {
			return err
		}
	}

	// Delete from cache
	if cm.cache != nil {
		_ = cm.cache.Delete(ctx, "context:"+contextID)
	}

	return nil
}

// ListContexts lists contexts for an agent
func (cm *ContextManager) ListContexts(ctx context.Context, agentID string, sessionID string, options map[string]interface{}) ([]*models.Context, error) {
	// If mockDB is provided, list from database
	if mockDB, ok := cm.db.(*MockDB); ok {
		return mockDB.ListContexts(ctx, agentID, sessionID, options)
	}

	// For tests that don't provide mock implementations, return an empty list
	return []*models.Context{}, nil
}

// SummarizeContext generates a summary of a context
func (cm *ContextManager) SummarizeContext(ctx context.Context, contextID string) (string, error) {
	// Get context
	_, err := cm.GetContext(ctx, contextID)
	if err != nil {
		return "", err
	}

	// Simple summary implementation for tests
	return "This is a context summary", nil
}

// SearchInContext searches for text within a context
func (cm *ContextManager) SearchInContext(ctx context.Context, contextID string, query string) ([]models.ContextItem, error) {
	// Get context
	contextData, err := cm.GetContext(ctx, contextID)
	if err != nil {
		return nil, err
	}

	// Simple search implementation for tests
	results := append([]models.ContextItem(nil), contextData.Content...)

	return results, nil
}

// Subscribe subscribes to context events
func (cm *ContextManager) Subscribe(eventType string, handler func(models.Event)) {
	if cm.subscribers[eventType] == nil {
		cm.subscribers[eventType] = make([]func(models.Event), 0)
	}

	cm.subscribers[eventType] = append(cm.subscribers[eventType], handler)
}

// Constants for truncation strategies
const (
	// TruncateOldestFirst truncates the oldest items first
	TruncateOldestFirst = "oldest_first"

	// TruncatePreservingUser truncates by removing assistant responses while preserving user messages
	TruncatePreservingUser = "preserving_user"

	// TruncateRelevanceBased truncates based on relevance to the current conversation
	TruncateRelevanceBased = "relevance_based"
)
