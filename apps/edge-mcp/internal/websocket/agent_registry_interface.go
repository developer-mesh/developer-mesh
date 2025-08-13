package websocket

import (
	"context"
	"time"
)

// AgentRegistryInterface defines the common interface for agent registries
type AgentRegistryInterface interface {
	// RegisterAgent registers a new agent
	RegisterAgent(ctx context.Context, reg *AgentRegistration) (*AgentInfo, error)

	// DiscoverAgents finds agents with required capabilities
	DiscoverAgents(ctx context.Context, tenantID string, requiredCapabilities []string, excludeSelf bool, selfID string) ([]map[string]interface{}, error)

	// DelegateTask delegates a task to another agent
	DelegateTask(ctx context.Context, fromAgentID, toAgentID string, task map[string]interface{}, timeout time.Duration) (*DelegationResult, error)

	// InitiateCollaboration starts multi-agent collaboration
	InitiateCollaboration(ctx context.Context, initiatorID string, agentIDs []string, task map[string]interface{}, strategy string) (*CollaborationSession, error)

	// GetAgentStatus retrieves agent status
	GetAgentStatus(ctx context.Context, agentID string) (*AgentInfo, error)

	// UpdateAgentStatus updates agent status
	UpdateAgentStatus(ctx context.Context, agentID, status string, metadata map[string]interface{}) error

	// RemoveAgent removes an agent from registry
	RemoveAgent(agentID string) error

	// RemoveAgentByConnection removes an agent when connection is closed (DB registry specific)
	RemoveAgentByConnection(connectionID string) error
}
