package core

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/interfaces"
	"github.com/developer-mesh/developer-mesh/pkg/models"
)

// AdapterContextBridge connects adapters with the context manager for managing context-aware tool interactions
type AdapterContextBridge struct {
	contextManager interfaces.ContextManager
	adapters       map[string]interfaces.Adapter
}

// NewAdapterContextBridge creates a new adapter-context bridge
func NewAdapterContextBridge(contextManager interfaces.ContextManager, adapters map[string]interfaces.Adapter) *AdapterContextBridge {
	return &AdapterContextBridge{
		contextManager: contextManager,
		adapters:       adapters,
	}
}

// ExecuteToolAction executes a tool action with context awareness
func (b *AdapterContextBridge) ExecuteToolAction(ctx context.Context, contextID string, tool string, action string, params map[string]interface{}) (interface{}, error) {
	// Get the adapter
	adapter, exists := b.adapters[tool]
	if !exists {
		return nil, fmt.Errorf("tool not found: %s", tool)
	}

	// Validate that the context exists
	_, err := b.contextManager.GetContext(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("failed to get context: %w", err)
	}

	// Record the tool request in the context
	requestItem := models.ContextItem{
		Role:    "tool_request",
		Content: fmt.Sprintf("%s.%s(%+v)", tool, action, params),
		Tokens:  1, // Will be calculated properly in production
		Metadata: map[string]interface{}{
			"tool":   tool,
			"action": action,
			"params": params,
		},
		Timestamp: time.Now(),
	}

	// Update the context with the request
	updateData := &models.Context{
		Content: []models.ContextItem{requestItem},
	}

	err = b.contextManager.UpdateContext(ctx, contextID, updateData)
	if err != nil {
		return nil, fmt.Errorf("failed to update context with request: %w", err)
	}

	// Execute the action - type assert to ActionAdapter interface
	actionAdapter, ok := adapter.(interface {
		ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error)
	})
	if !ok {
		return nil, fmt.Errorf("adapter %s does not support ExecuteAction", tool)
	}
	result, err := actionAdapter.ExecuteAction(ctx, contextID, action, params)

	// Record the tool response in the context
	responseContent := "Error executing tool action"
	if err == nil {
		responseBytes, _ := json.Marshal(result)
		responseContent = string(responseBytes)
	}

	responseItem := models.ContextItem{
		Role:    "tool_response",
		Content: responseContent,
		Tokens:  1, // Will be calculated properly in production
		Metadata: map[string]interface{}{
			"tool":   tool,
			"action": action,
			"status": err == nil,
			"result": result,
			"error":  err != nil,
		},
		Timestamp: time.Now(),
	}

	// Update the context with the response
	updateData = &models.Context{
		Content: []models.ContextItem{responseItem},
	}

	err2 := b.contextManager.UpdateContext(ctx, contextID, updateData)
	if err2 != nil {
		// Log but continue with original error if present
		fmt.Printf("Failed to update context with response: %v\n", err2)
	}

	// Return the original result and error
	return result, err
}

// GetToolData gets data from a tool with context awareness
func (b *AdapterContextBridge) GetToolData(ctx context.Context, contextID string, tool string, query interface{}) (interface{}, error) {
	// Get the adapter
	adapter, exists := b.adapters[tool]
	if !exists {
		return nil, fmt.Errorf("tool not found: %s", tool)
	}

	// Validate that the context exists
	_, err := b.contextManager.GetContext(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("failed to get context: %w", err)
	}

	// Record the data request in the context
	queryBytes, _ := json.Marshal(query)
	requestItem := models.ContextItem{
		Role:    "data_request",
		Content: fmt.Sprintf("%s.getData(%s)", tool, string(queryBytes)),
		Tokens:  1, // Will be calculated properly in production
		Metadata: map[string]interface{}{
			"tool":  tool,
			"query": query,
		},
		Timestamp: time.Now(),
	}

	// Update the context with the request
	updateData := &models.Context{
		Content: []models.ContextItem{requestItem},
	}

	err = b.contextManager.UpdateContext(ctx, contextID, updateData)
	if err != nil {
		return nil, fmt.Errorf("failed to update context with request: %w", err)
	}

	// Convert query to map for ExecuteAction since GetData is not available in interfaces.Adapter
	queryMap := make(map[string]interface{})
	if queryBytes, err := json.Marshal(query); err == nil {
		_ = json.Unmarshal(queryBytes, &queryMap)
	}

	// Use ExecuteAction with a "getData" action as a workaround
	// Type assert to ActionAdapter interface
	actionAdapter, ok := adapter.(interface {
		ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error)
	})
	if !ok {
		return nil, fmt.Errorf("adapter %s does not support ExecuteAction", tool)
	}
	result, err := actionAdapter.ExecuteAction(ctx, contextID, "getData", queryMap)

	// Record the data response in the context
	responseContent := "Error getting tool data"
	if err == nil {
		responseBytes, _ := json.Marshal(result)
		responseContent = string(responseBytes)
	}

	responseItem := models.ContextItem{
		Role:    "data_response",
		Content: responseContent,
		Tokens:  1, // Will be calculated properly in production
		Metadata: map[string]interface{}{
			"tool":   tool,
			"status": err == nil,
			"result": result,
			"error":  err != nil,
		},
		Timestamp: time.Now(),
	}

	// Update the context with the response
	updateData = &models.Context{
		Content: []models.ContextItem{responseItem},
	}

	err2 := b.contextManager.UpdateContext(ctx, contextID, updateData)
	if err2 != nil {
		// Log but continue with original error if present
		fmt.Printf("Failed to update context with response: %v\n", err2)
	}

	// Return the original result and error
	return result, err
}

// HandleToolWebhook handles a webhook from a tool
func (b *AdapterContextBridge) HandleToolWebhook(ctx context.Context, tool string, eventType string, payload []byte) error {
	// Get the adapter
	adapter, exists := b.adapters[tool]
	if !exists {
		return fmt.Errorf("tool not found: %s", tool)
	}

	// Extract context IDs from the payload if present
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err == nil {
		// Check if the payload contains context IDs
		if metadata, ok := data["metadata"].(map[string]interface{}); ok {
			if contextIDs, ok := metadata["context_ids"].([]interface{}); ok {
				// Record the webhook in each context
				for _, contextID := range contextIDs {
					if cid, ok := contextID.(string); ok {
						// Validate that the context exists
						_, err := b.contextManager.GetContext(ctx, cid)
						if err != nil {
							fmt.Printf("Failed to get context %s: %v\n", cid, err)
							continue
						}

						// Record the webhook in the context
						webhookItem := models.ContextItem{
							Role:    "webhook",
							Content: fmt.Sprintf("Webhook from %s: %s", tool, eventType),
							Tokens:  1, // Will be calculated properly in production
							Metadata: map[string]interface{}{
								"tool":       tool,
								"event_type": eventType,
								"payload":    data,
							},
							Timestamp: time.Now(),
						}

						// Update the context with the webhook
						updateData := &models.Context{
							Content: []models.ContextItem{webhookItem},
						}

						err = b.contextManager.UpdateContext(ctx, cid, updateData)
						if err != nil {
							fmt.Printf("Failed to update context %s with webhook: %v\n", cid, err)
						}
					}
				}
			}
		}
	}

	// Handle the webhook with the adapter
	// Type assert to WebhookAdapter interface
	webhookAdapter, ok := adapter.(interface {
		HandleWebhook(ctx context.Context, eventType string, payload []byte) error
	})
	if !ok {
		return fmt.Errorf("adapter %s does not support HandleWebhook", tool)
	}
	return webhookAdapter.HandleWebhook(ctx, eventType, payload)
}
