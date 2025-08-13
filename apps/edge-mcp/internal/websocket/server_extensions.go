package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/google/uuid"
)

// ExtendedConnection adds fields needed for universal agent support
type ExtendedConnection struct {
	*Connection
	OrganizationID uuid.UUID
	Token          string
	AgentInfo      interface{} // Can be *AgentInfo or *UniversalAgentInfo
	subscriptions  sync.Map    // subscription -> bool
}

// IsSubscribedTo checks if connection is subscribed to a topic
func (ec *ExtendedConnection) IsSubscribedTo(topic string) bool {
	_, subscribed := ec.subscriptions.Load(topic)
	return subscribed
}

// Subscribe adds a subscription
func (ec *ExtendedConnection) Subscribe(topic string) {
	ec.subscriptions.Store(topic, true)
}

// Unsubscribe removes a subscription
func (ec *ExtendedConnection) Unsubscribe(topic string) {
	ec.subscriptions.Delete(topic)
}

// ExtendedServer adds fields needed for universal agent support
type ExtendedServer struct {
	*Server

	// Additional repositories
	tenantRepo   repository.TenantConfigRepository
	orgRepo      repository.OrganizationRepository
	manifestRepo repository.AgentManifestRepository

	// Enhanced components
	rateLimiter    interface{} // Can be *auth.RateLimiter or *AgentRateLimiter
	circuitBreaker interface{} // Can be *resilience.CircuitBreaker or *AgentCircuitBreaker
	messageBroker  *AgentMessageBroker

	// Connection management
	connectionsMu  sync.RWMutex
	extConnections map[string]*ExtendedConnection
}

// SetTenantRepo sets the tenant repository
func (es *ExtendedServer) SetTenantRepo(repo repository.TenantConfigRepository) {
	es.tenantRepo = repo
}

// SetOrgRepo sets the organization repository
func (es *ExtendedServer) SetOrgRepo(repo repository.OrganizationRepository) {
	es.orgRepo = repo
}

// SetManifestRepo sets the manifest repository
func (es *ExtendedServer) SetManifestRepo(repo repository.AgentManifestRepository) {
	es.manifestRepo = repo
}

// SetRateLimiter sets the rate limiter (can be standard or agent-specific)
func (es *ExtendedServer) SetRateLimiter(limiter interface{}) {
	es.rateLimiter = limiter
}

// SetCircuitBreaker sets the circuit breaker (can be standard or agent-specific)
func (es *ExtendedServer) SetCircuitBreaker(breaker interface{}) {
	es.circuitBreaker = breaker
}

// SetMessageBroker sets the agent message broker
func (es *ExtendedServer) SetMessageBroker(broker *AgentMessageBroker) {
	es.messageBroker = broker
}

// GetExtendedConnection converts a standard connection to extended
func (es *ExtendedServer) GetExtendedConnection(conn *Connection) *ExtendedConnection {
	es.connectionsMu.RLock()
	defer es.connectionsMu.RUnlock()

	if extConn, ok := es.extConnections[conn.ID]; ok {
		return extConn
	}

	// Create new extended connection
	extConn := &ExtendedConnection{
		Connection: conn,
	}

	es.connectionsMu.Lock()
	es.extConnections[conn.ID] = extConn
	es.connectionsMu.Unlock()

	return extConn
}

// NewExtendedServer creates a server with universal agent support
func NewExtendedServer(base *Server) *ExtendedServer {
	return &ExtendedServer{
		Server:         base,
		extConnections: make(map[string]*ExtendedConnection),
	}
}

// canAccessAgent checks if a connection can access a universal agent
func (es *ExtendedServer) canAccessAgent(ctx context.Context, conn *Connection, agent *UniversalAgentInfo) bool {
	// Get extended connection
	extConn := es.GetExtendedConnection(conn)

	// Check organization isolation
	if extConn.OrganizationID != uuid.Nil && es.manifestRepo != nil {
		// Get agent's organization from manifest
		manifest, err := es.manifestRepo.GetManifestByAgentID(ctx, agent.AgentID)
		if err != nil {
			return false
		}

		// Check if same organization
		if manifest.OrganizationID != extConn.OrganizationID {
			// Check if cross-organization access is allowed
			if es.orgRepo != nil {
				org, err := es.orgRepo.GetByID(ctx, extConn.OrganizationID)
				if err != nil || org == nil || org.IsStrictlyIsolated() {
					es.logger.Debug("Cross-organization access blocked", map[string]interface{}{
						"source_org": extConn.OrganizationID,
						"target_org": manifest.OrganizationID,
						"agent_id":   agent.AgentID,
					})
					return false
				}
			}
		}
	}

	return true
}

// broadcastAgentEvent broadcasts an event to all connected agents
func (es *ExtendedServer) broadcastAgentEvent(ctx context.Context, eventType string, data map[string]interface{}) {
	// Broadcast event to subscribers
	event := map[string]interface{}{
		"type":      eventType,
		"timestamp": time.Now().Format(time.RFC3339),
		"data":      data,
	}

	// Marshal event to JSON
	eventBytes, err := json.Marshal(event)
	if err != nil {
		es.logger.Error("Failed to marshal event", map[string]interface{}{
			"event_type": eventType,
			"error":      err.Error(),
		})
		return
	}

	// Send to all connected agents subscribed to agent events
	es.connectionsMu.RLock()
	defer es.connectionsMu.RUnlock()

	for _, extConn := range es.extConnections {
		if extConn.IsSubscribedTo("agent.events") {
			select {
			case extConn.send <- eventBytes:
			default:
				// Connection buffer full, skip
			}
		}
	}
}
