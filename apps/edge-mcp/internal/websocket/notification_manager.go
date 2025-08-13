package websocket

import (
	"context"
	"sync"
	"time"

	ws "github.com/developer-mesh/developer-mesh/pkg/models/websocket"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
)

// NotificationManager handles broadcasting notifications to WebSocket connections
type NotificationManager struct {
	connections         map[string]*Connection // connection ID -> connection
	subscribers         map[string][]string    // topic -> connection IDs
	mu                  sync.RWMutex
	logger              observability.Logger
	metrics             observability.MetricsClient
	bufferSize          int
	dropStrategy        string               // "oldest" or "newest"
	subscriptionManager *SubscriptionManager // Reference to subscription manager
}

// NewNotificationManager creates a new notification manager
func NewNotificationManager(logger observability.Logger, metrics observability.MetricsClient) *NotificationManager {
	return &NotificationManager{
		connections:  make(map[string]*Connection),
		subscribers:  make(map[string][]string),
		logger:       logger,
		metrics:      metrics,
		bufferSize:   1000,
		dropStrategy: "oldest",
	}
}

// SetSubscriptionManager sets the subscription manager for resource-based subscriptions
func (nm *NotificationManager) SetSubscriptionManager(sm *SubscriptionManager) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.subscriptionManager = sm
}

// RegisterConnection adds a connection to the notification manager
func (nm *NotificationManager) RegisterConnection(conn *Connection) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.connections[conn.ID] = conn
}

// UnregisterConnection removes a connection from the notification manager
func (nm *NotificationManager) UnregisterConnection(connID string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	delete(nm.connections, connID)

	// Remove from all subscriptions
	for topic, subs := range nm.subscribers {
		newSubs := make([]string, 0, len(subs))
		for _, sub := range subs {
			if sub != connID {
				newSubs = append(newSubs, sub)
			}
		}
		nm.subscribers[topic] = newSubs
	}
}

// Subscribe adds a connection to a topic
func (nm *NotificationManager) Subscribe(connID, topic string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if subs, ok := nm.subscribers[topic]; ok {
		// Check if already subscribed
		for _, sub := range subs {
			if sub == connID {
				return
			}
		}
		nm.subscribers[topic] = append(subs, connID)
	} else {
		nm.subscribers[topic] = []string{connID}
	}
}

// Unsubscribe removes a connection from a topic
func (nm *NotificationManager) Unsubscribe(connID, topic string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if subs, ok := nm.subscribers[topic]; ok {
		newSubs := make([]string, 0, len(subs))
		for _, sub := range subs {
			if sub != connID {
				newSubs = append(newSubs, sub)
			}
		}
		nm.subscribers[topic] = newSubs
	}
}

// SendNotification sends a notification to a specific connection
func (nm *NotificationManager) SendNotification(ctx context.Context, connID string, method string, params interface{}) error {
	nm.mu.RLock()
	conn, ok := nm.connections[connID]
	nm.mu.RUnlock()

	if !ok {
		return ErrConnectionNotFound
	}

	msg := &ws.Message{
		ID:     uuid.New().String(),
		Type:   ws.MessageTypeNotification,
		Method: method,
		Params: params,
	}

	return conn.SendMessage(msg)
}

// BroadcastNotification sends a notification to all connections subscribed to a topic
func (nm *NotificationManager) BroadcastNotification(ctx context.Context, topic string, method string, params interface{}) {
	nm.mu.RLock()
	// Get subscribers from internal map
	subs := nm.subscribers[topic]

	// Also get subscribers from subscription manager if available
	var resourceSubs []string
	if nm.subscriptionManager != nil {
		// The topic is used as resource name for subscription manager
		subscriptions := nm.subscriptionManager.GetSubscriptions(topic)
		for _, sub := range subscriptions {
			resourceSubs = append(resourceSubs, sub.ConnectionID)
		}
	}
	nm.mu.RUnlock()

	nm.logger.Debug("BroadcastNotification checking subscribers", map[string]interface{}{
		"topic":                    topic,
		"method":                   method,
		"internal_subscribers":     len(subs),
		"resource_subscribers":     len(resourceSubs),
		"has_subscription_manager": nm.subscriptionManager != nil,
	})

	// Combine both subscriber lists and deduplicate
	allSubs := make(map[string]bool)
	for _, connID := range subs {
		allSubs[connID] = true
	}
	for _, connID := range resourceSubs {
		allSubs[connID] = true
	}

	if len(allSubs) == 0 {
		return
	}

	// Send to all subscribers
	var wg sync.WaitGroup
	for connID := range allSubs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if err := nm.SendNotification(ctx, id, method, params); err != nil {
				nm.logger.Debug("Failed to send notification", map[string]interface{}{
					"connection_id": id,
					"method":        method,
					"error":         err.Error(),
				})
			}
		}(connID)
	}

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All notifications sent
	case <-time.After(5 * time.Second):
		nm.logger.Warn("Timeout broadcasting notifications", map[string]interface{}{
			"topic":  topic,
			"method": method,
		})
	}
}

// SendToAll sends a notification to all connected clients
func (nm *NotificationManager) SendToAll(ctx context.Context, method string, params interface{}) {
	nm.mu.RLock()
	connIDs := make([]string, 0, len(nm.connections))
	for id := range nm.connections {
		connIDs = append(connIDs, id)
	}
	nm.mu.RUnlock()

	for _, connID := range connIDs {
		_ = nm.SendNotification(ctx, connID, method, params)
	}
}

// Workflow-specific notification methods

// NotifyWorkflowStepStarted sends a workflow.step_started notification
func (nm *NotificationManager) NotifyWorkflowStepStarted(ctx context.Context, workflowID, executionID, stepID string) {
	params := map[string]interface{}{
		"step_id":      stepID,
		"workflow_id":  workflowID,
		"execution_id": executionID,
		"timestamp":    time.Now().Format(time.RFC3339),
	}

	nm.BroadcastNotification(ctx, "workflow:"+workflowID, "workflow.step_started", params)
}

// NotifyWorkflowStepCompleted sends a workflow.step_completed notification
func (nm *NotificationManager) NotifyWorkflowStepCompleted(ctx context.Context, workflowID, executionID, stepID string, result interface{}) {
	params := map[string]interface{}{
		"step_id":      stepID,
		"workflow_id":  workflowID,
		"execution_id": executionID,
		"result":       result,
		"timestamp":    time.Now().Format(time.RFC3339),
	}

	nm.BroadcastNotification(ctx, "workflow:"+workflowID, "workflow.step_completed", params)
}

// NotifyWorkflowTransactionEvent sends a workflow.transaction_event notification
func (nm *NotificationManager) NotifyWorkflowTransactionEvent(ctx context.Context, workflowID, transactionID, event string) {
	params := map[string]interface{}{
		"event":          event,
		"workflow_id":    workflowID,
		"transaction_id": transactionID,
		"timestamp":      time.Now().Format(time.RFC3339),
	}

	nm.BroadcastNotification(ctx, "workflow:"+workflowID, "workflow.transaction_event", params)
}

// NotifyToolProgress sends a tool.progress notification
func (nm *NotificationManager) NotifyToolProgress(ctx context.Context, connID string, percentage float64, message string, operation string, estimatedTime *int) {
	params := map[string]interface{}{
		"percentage": percentage,
		"message":    message,
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	if percentage > 0 {
		params["current_operation"] = operation
	}

	if estimatedTime != nil {
		params["estimated_time_remaining"] = *estimatedTime
	}

	_ = nm.SendNotification(ctx, connID, "tool.progress", params)
}

// NotifyContextChunk sends a context.chunk notification
func (nm *NotificationManager) NotifyContextChunk(ctx context.Context, connID string, bytes, chunkNumber, totalChunks int) {
	params := map[string]interface{}{
		"bytes":        bytes,
		"chunk_number": chunkNumber,
	}

	if totalChunks > 0 {
		params["total_chunks"] = totalChunks
	}

	_ = nm.SendNotification(ctx, connID, "context.chunk", params)
}

// NotifyAgentStatusChanged sends an agent.status_changed notification
func (nm *NotificationManager) NotifyAgentStatusChanged(ctx context.Context, agentID, name, status string, activity, currentTask interface{}) {
	params := map[string]interface{}{
		"agent_id":  agentID,
		"name":      name,
		"status":    status,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	if activity != nil {
		params["activity"] = activity
	}

	if currentTask != nil {
		params["current_task"] = currentTask
	}

	nm.logger.Info("NotifyAgentStatusChanged broadcasting", map[string]interface{}{
		"topic":    "agent.status",
		"method":   "agent.status_changed",
		"agent_id": agentID,
		"name":     name,
		"status":   status,
	})

	nm.BroadcastNotification(ctx, "agent.status", "agent.status_changed", params)
}

// NotifyAgent sends a notification to a specific agent by finding their connection
func (nm *NotificationManager) NotifyAgent(ctx context.Context, agentID string, notification interface{}) error {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	// Find connection for the agent
	for connID, conn := range nm.connections {
		if conn.AgentID == agentID {
			// Send notification to the agent's connection
			method := "agent.notification"
			if notif, ok := notification.(map[string]interface{}); ok {
				if notifType, ok := notif["type"].(string); ok {
					method = notifType
				}
			}
			return nm.SendNotification(ctx, connID, method, notification)
		}
	}

	return ErrConnectionNotFound
}

// Custom errors
var (
	ErrConnectionNotFound = &ws.Error{
		Code:    ws.ErrCodeInvalidParams,
		Message: "Connection not found",
	}
)
