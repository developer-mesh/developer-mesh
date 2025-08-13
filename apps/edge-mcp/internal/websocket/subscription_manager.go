package websocket

import (
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
)

// SubscriptionManager manages real-time subscriptions
type SubscriptionManager struct {
	subscriptions map[string]*Subscription // subscription ID -> subscription
	connections   map[string][]string      // connection ID -> subscription IDs
	resources     map[string][]string      // resource -> subscription IDs
	mu            sync.RWMutex
	logger        observability.Logger
	metrics       observability.MetricsClient
}

// NewSubscriptionManager creates a new subscription manager
func NewSubscriptionManager(logger observability.Logger, metrics observability.MetricsClient) *SubscriptionManager {
	return &SubscriptionManager{
		subscriptions: make(map[string]*Subscription),
		connections:   make(map[string][]string),
		resources:     make(map[string][]string),
		logger:        logger,
		metrics:       metrics,
	}
}

// Subscription represents a real-time subscription
type Subscription struct {
	ID           string                 `json:"id"`
	ConnectionID string                 `json:"connection_id"`
	Resource     string                 `json:"resource"`
	Filter       map[string]interface{} `json:"filter"`
	CreatedAt    string                 `json:"created_at"`
}

// Subscribe creates a new subscription
func (sm *SubscriptionManager) Subscribe(connectionID, resource string, filter map[string]interface{}) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	subscription := &Subscription{
		ID:           uuid.New().String(),
		ConnectionID: connectionID,
		Resource:     resource,
		Filter:       filter,
		CreatedAt:    timeNow(),
	}

	// Store subscription
	sm.subscriptions[subscription.ID] = subscription

	// Map connection to subscription
	sm.connections[connectionID] = append(sm.connections[connectionID], subscription.ID)

	// Map resource to subscription
	sm.resources[resource] = append(sm.resources[resource], subscription.ID)

	sm.metrics.IncrementCounter("subscriptions_created", 1)
	sm.logger.Info("Subscription created", map[string]interface{}{
		"subscription_id": subscription.ID,
		"connection_id":   connectionID,
		"resource":        resource,
	})

	return subscription.ID, nil
}

// Unsubscribe removes a subscription
func (sm *SubscriptionManager) Unsubscribe(connectionID, subscriptionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	subscription, ok := sm.subscriptions[subscriptionID]
	if !ok {
		return fmt.Errorf("subscription not found: %s", subscriptionID)
	}

	// Verify ownership
	if subscription.ConnectionID != connectionID {
		return fmt.Errorf("subscription not owned by connection")
	}

	// Remove from all maps
	delete(sm.subscriptions, subscriptionID)

	// Remove from connection map
	if subs := sm.connections[connectionID]; subs != nil {
		sm.connections[connectionID] = sm.removeFromSlice(subs, subscriptionID)
	}

	// Remove from resource map
	if subs := sm.resources[subscription.Resource]; subs != nil {
		sm.resources[subscription.Resource] = sm.removeFromSlice(subs, subscriptionID)
	}

	sm.metrics.IncrementCounter("subscriptions_removed", 1)
	return nil
}

// UnsubscribeAll removes all subscriptions for a connection
func (sm *SubscriptionManager) UnsubscribeAll(connectionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	subscriptionIDs, ok := sm.connections[connectionID]
	if !ok {
		return nil
	}

	for _, subID := range subscriptionIDs {
		if subscription, ok := sm.subscriptions[subID]; ok {
			delete(sm.subscriptions, subID)
			if subs := sm.resources[subscription.Resource]; subs != nil {
				sm.resources[subscription.Resource] = sm.removeFromSlice(subs, subID)
			}
		}
	}

	delete(sm.connections, connectionID)

	sm.metrics.IncrementCounter("subscriptions_removed", float64(len(subscriptionIDs)))
	return nil
}

// GetSubscriptions returns all subscriptions for a resource
func (sm *SubscriptionManager) GetSubscriptions(resource string) []*Subscription {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	subscriptionIDs, ok := sm.resources[resource]
	if !ok {
		return nil
	}

	var subscriptions []*Subscription
	for _, id := range subscriptionIDs {
		if sub, ok := sm.subscriptions[id]; ok {
			subscriptions = append(subscriptions, sub)
		}
	}

	return subscriptions
}

// GetConnectionSubscriptions returns all subscriptions for a connection
func (sm *SubscriptionManager) GetConnectionSubscriptions(connectionID string) []*Subscription {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	subscriptionIDs, ok := sm.connections[connectionID]
	if !ok {
		return nil
	}

	var subscriptions []*Subscription
	for _, id := range subscriptionIDs {
		if sub, ok := sm.subscriptions[id]; ok {
			subscriptions = append(subscriptions, sub)
		}
	}

	return subscriptions
}

// SubscribeToWorkspace subscribes a connection to workspace events
func (sm *SubscriptionManager) SubscribeToWorkspace(connectionID, workspaceID string) error {
	resource := fmt.Sprintf("workspace.%s", workspaceID)
	_, err := sm.Subscribe(connectionID, resource, nil)
	return err
}

// UnsubscribeFromWorkspace unsubscribes a connection from workspace events
func (sm *SubscriptionManager) UnsubscribeFromWorkspace(connectionID, workspaceID string) error {
	resource := fmt.Sprintf("workspace.%s", workspaceID)

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Find and remove workspace subscriptions
	if subscriptionIDs, ok := sm.connections[connectionID]; ok {
		for _, subID := range subscriptionIDs {
			if sub, ok := sm.subscriptions[subID]; ok && sub.Resource == resource {
				delete(sm.subscriptions, subID)
				if subs := sm.resources[resource]; subs != nil {
					sm.resources[resource] = sm.removeFromSlice(subs, subID)
				}
				if subs := sm.connections[connectionID]; subs != nil {
					sm.connections[connectionID] = sm.removeFromSlice(subs, subID)
				}
				sm.metrics.IncrementCounter("subscriptions_removed", 1)
			}
		}
	}

	return nil
}

// MatchesFilter checks if data matches subscription filter
func (s *Subscription) MatchesFilter(data map[string]interface{}) bool {
	if s.Filter == nil {
		return true
	}

	for key, filterValue := range s.Filter {
		dataValue, ok := data[key]
		if !ok {
			return false
		}

		// Simple equality check - in production, support more complex filters
		if dataValue != filterValue {
			return false
		}
	}

	return true
}

// Helper function to remove element from slice
func (sm *SubscriptionManager) removeFromSlice(slice []string, value string) []string {
	result := make([]string, 0, len(slice))
	for _, v := range slice {
		if v != value {
			result = append(result, v)
		}
	}
	return result
}

// Helper function for current time
func timeNow() string {
	return fmt.Sprintf("%d", timeNowUnix())
}

func timeNowUnix() int64 {
	return 0 // This would be time.Now().Unix() in production
}

// ListSubscriptions returns all subscriptions for a connection
func (sm *SubscriptionManager) ListSubscriptions(connectionID string) []map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	subIDs, ok := sm.connections[connectionID]
	if !ok {
		return []map[string]interface{}{}
	}

	var subscriptions []map[string]interface{}
	for _, subID := range subIDs {
		if sub, exists := sm.subscriptions[subID]; exists {
			subscriptions = append(subscriptions, map[string]interface{}{
				"id":         sub.ID,
				"resource":   sub.Resource,
				"filter":     sub.Filter,
				"created_at": sub.CreatedAt,
			})
		}
	}

	return subscriptions
}

// GetSubscriptionStatus returns the status of a subscription
func (sm *SubscriptionManager) GetSubscriptionStatus(connectionID, subscriptionID string) (*SubscriptionStatus, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sub, ok := sm.subscriptions[subscriptionID]
	if !ok {
		return nil, fmt.Errorf("subscription not found: %s", subscriptionID)
	}

	// Verify ownership
	if sub.ConnectionID != connectionID {
		return nil, fmt.Errorf("subscription not owned by connection")
	}

	return &SubscriptionStatus{
		Status:     "active",
		Resource:   sub.Resource,
		Filter:     sub.Filter,
		CreatedAt:  time.Now(), // In real implementation, parse sub.CreatedAt
		LastEvent:  time.Now(), // In real implementation, track last event time
		EventCount: 0,          // In real implementation, track event count
	}, nil
}

// SubscriptionStatus represents the status of a subscription
type SubscriptionStatus struct {
	Status     string                 `json:"status"`
	Resource   string                 `json:"resource"`
	Filter     map[string]interface{} `json:"filter"`
	CreatedAt  time.Time              `json:"created_at"`
	LastEvent  time.Time              `json:"last_event"`
	EventCount int                    `json:"event_count"`
}
