// Copyright 2025 developer-mesh
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
)

// AgentRegistry manages agent registration and discovery
type AgentRegistry struct {
	agents       sync.Map // agent ID -> AgentInfo
	capabilities sync.Map // capability -> []agent IDs
	logger       observability.Logger
	metrics      observability.MetricsClient
}

// NewAgentRegistry creates a new agent registry
func NewAgentRegistry(logger observability.Logger, metrics observability.MetricsClient) *AgentRegistry {
	return &AgentRegistry{
		logger:  logger,
		metrics: metrics,
	}
}

// AgentRegistration represents agent registration request
type AgentRegistration struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Capabilities []string               `json:"capabilities"`
	Metadata     map[string]interface{} `json:"metadata"`
	ConnectionID string                 `json:"connection_id"`
	TenantID     string                 `json:"tenant_id"`
}

// AgentInfo represents registered agent information
type AgentInfo struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Capabilities []string               `json:"capabilities"`
	Metadata     map[string]interface{} `json:"metadata"`
	ConnectionID string                 `json:"connection_id"`
	TenantID     string                 `json:"tenant_id"`
	RegisteredAt time.Time              `json:"registered_at"`
	LastSeen     time.Time              `json:"last_seen"`
	Status       string                 `json:"status"` // online, offline, busy
	ActiveTasks  int                    `json:"active_tasks"`
	Health       string                 `json:"health"` // healthy, degraded, unhealthy
}

// DelegationResult represents task delegation result
type DelegationResult struct {
	ID          string                 `json:"id"`
	FromAgentID string                 `json:"from_agent_id"`
	ToAgentID   string                 `json:"to_agent_id"`
	TaskID      string                 `json:"task_id"`
	Status      string                 `json:"status"` // "accepted", "rejected", "completed", "failed", "timeout"
	Result      interface{}            `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	DelegatedAt time.Time              `json:"delegated_at"`
}

// CollaborationSession represents multi-agent collaboration
type CollaborationSession struct {
	ID           string                 `json:"id"`
	InitiatorID  string                 `json:"initiator_id"`
	Agents       []string               `json:"agents"`
	AgentIDs     []string               `json:"agent_ids"` // Alternative field name for compatibility
	Task         map[string]interface{} `json:"task"`
	Strategy     string                 `json:"strategy"` // "round-robin", "parallel", "hierarchical", "consensus"
	Status       string                 `json:"status"`   // "active", "completed", "failed", "cancelled"
	Priority     string                 `json:"priority"` // "low", "medium", "high", "critical"
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	InitiatedAt  time.Time              `json:"initiated_at"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
	Results      map[string]interface{} `json:"results,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
}

// RegisterAgent registers a new agent
func (ar *AgentRegistry) RegisterAgent(ctx context.Context, reg *AgentRegistration) (*AgentInfo, error) {
	agent := &AgentInfo{
		ID:           reg.ID,
		Name:         reg.Name,
		Capabilities: reg.Capabilities,
		Metadata:     reg.Metadata,
		ConnectionID: reg.ConnectionID,
		TenantID:     reg.TenantID,
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
		Status:       "online",
		ActiveTasks:  0,
		Health:       "healthy",
	}

	// Store agent
	ar.agents.Store(agent.ID, agent)

	// Index capabilities
	for _, capability := range agent.Capabilities {
		ar.logger.Info("Indexing capability", map[string]interface{}{
			"capability": capability,
			"agent_id":   agent.ID,
		})
		ar.addCapability(capability, agent.ID)
	}

	ar.metrics.IncrementCounter("agents_registered", 1)
	ar.logger.Info("Agent registered", map[string]interface{}{
		"agent_id":     agent.ID,
		"name":         agent.Name,
		"capabilities": agent.Capabilities,
	})

	return agent, nil
}

// DiscoverAgents finds agents with required capabilities
func (ar *AgentRegistry) DiscoverAgents(ctx context.Context, tenantID string, requiredCapabilities []string, excludeSelf bool, selfID string) ([]map[string]interface{}, error) {
	ar.logger.Info("DiscoverAgents called", map[string]interface{}{
		"tenant_id":             tenantID,
		"required_capabilities": requiredCapabilities,
		"exclude_self":          excludeSelf,
		"self_id":               selfID,
	})

	// Debug: Print all registered agents
	ar.agents.Range(func(key, value interface{}) bool {
		agent := value.(*AgentInfo)
		ar.logger.Info("Registered agent", map[string]interface{}{
			"agent_id":     agent.ID,
			"capabilities": agent.Capabilities,
			"status":       agent.Status,
			"tenant_id":    agent.TenantID,
		})
		return true
	})

	// Find agents with all required capabilities
	agentMap := make(map[string]*AgentInfo)

	for _, capability := range requiredCapabilities {
		ar.logger.Info("Looking up capability", map[string]interface{}{
			"capability": capability,
		})

		val, ok := ar.capabilities.Load(capability)
		if !ok {
			ar.logger.Info("No agents found with capability", map[string]interface{}{
				"capability": capability,
			})

			// Debug: Print all capability indexes
			ar.logger.Info("Current capability index:", map[string]interface{}{})
			ar.capabilities.Range(func(key, value interface{}) bool {
				ar.logger.Info("Capability index entry", map[string]interface{}{
					"capability": key,
					"agents":     value,
				})
				return true
			})

			return []map[string]interface{}{}, nil // No agents with this capability
		}

		agentIDs := val.([]string)
		ar.logger.Info("Agents with capability", map[string]interface{}{
			"capability": capability,
			"agents":     agentIDs,
		})

		for _, agentID := range agentIDs {
			if excludeSelf && agentID == selfID {
				ar.logger.Debug("Excluding self", map[string]interface{}{
					"agent_id": agentID,
				})
				continue
			}

			if agentVal, ok := ar.agents.Load(agentID); ok {
				agent := agentVal.(*AgentInfo)
				ar.logger.Debug("Checking agent", map[string]interface{}{
					"agent_id":     agentID,
					"tenant_id":    agent.TenantID,
					"req_tenant":   tenantID,
					"status":       agent.Status,
					"capabilities": agent.Capabilities,
				})
				if agent.TenantID == tenantID && agent.Status == "online" {
					agentMap[agentID] = agent
				}
			}
		}
	}

	// Convert to result format
	var result []map[string]interface{}
	for _, agent := range agentMap {
		// Check if agent has ALL required capabilities
		hasAll := true
		for _, reqCap := range requiredCapabilities {
			found := false
			for _, agentCap := range agent.Capabilities {
				if agentCap == reqCap {
					found = true
					break
				}
			}
			if !found {
				hasAll = false
				break
			}
		}

		if hasAll {
			result = append(result, map[string]interface{}{
				"id":           agent.ID,
				"name":         agent.Name,
				"capabilities": agent.Capabilities,
				"status":       agent.Status,
				"health":       agent.Health,
				"active_tasks": agent.ActiveTasks,
			})
		}
	}

	return result, nil
}

// DelegateTask delegates a task to another agent
func (ar *AgentRegistry) DelegateTask(ctx context.Context, fromAgentID, toAgentID string, task map[string]interface{}, timeout time.Duration) (*DelegationResult, error) {
	// Verify target agent exists and is online
	val, ok := ar.agents.Load(toAgentID)
	if !ok {
		return nil, fmt.Errorf("target agent not found: %s", toAgentID)
	}

	targetAgent := val.(*AgentInfo)
	if targetAgent.Status != "online" {
		return nil, fmt.Errorf("target agent is not online: %s", targetAgent.Status)
	}

	// Create delegation result
	result := &DelegationResult{
		ID:          uuid.New().String(),
		FromAgentID: fromAgentID,
		ToAgentID:   toAgentID,
		TaskID:      uuid.New().String(),
		Status:      "accepted",
		StartedAt:   time.Now(),
		DelegatedAt: time.Now(),
		Metadata:    make(map[string]interface{}),
	}

	// Increment target agent's task count
	targetAgent.ActiveTasks++

	ar.metrics.IncrementCounter("tasks_delegated", 1)
	ar.logger.Info("Task delegated", map[string]interface{}{
		"task_id":    result.TaskID,
		"from_agent": fromAgentID,
		"to_agent":   toAgentID,
	})

	return result, nil
}

// InitiateCollaboration starts multi-agent collaboration
func (ar *AgentRegistry) InitiateCollaboration(ctx context.Context, initiatorID string, agentIDs []string, task map[string]interface{}, strategy string) (*CollaborationSession, error) {
	// Verify all agents exist and are online
	for _, agentID := range agentIDs {
		val, ok := ar.agents.Load(agentID)
		if !ok {
			return nil, fmt.Errorf("agent not found: %s", agentID)
		}

		agent := val.(*AgentInfo)
		if agent.Status != "online" {
			return nil, fmt.Errorf("agent %s is not online", agentID)
		}
	}

	// Create collaboration session
	now := time.Now()

	// Extract priority from task metadata if provided, default to "medium"
	priority := "medium"
	if p, ok := task["priority"].(string); ok {
		priority = p
	}

	// Extract initial status from task metadata if provided, default to "active"
	status := "active"
	if s, ok := task["status"].(string); ok {
		status = s
	}

	session := &CollaborationSession{
		ID:          uuid.New().String(),
		InitiatorID: initiatorID,
		Agents:      append([]string{initiatorID}, agentIDs...),
		AgentIDs:    agentIDs,
		Task:        task,
		Strategy:    strategy,
		Status:      status,
		Priority:    priority,
		CreatedAt:   now,
		UpdatedAt:   now,
		InitiatedAt: now,
		Results:     make(map[string]interface{}),
		Metadata:    make(map[string]interface{}),
	}

	ar.metrics.IncrementCounter("collaborations_initiated", 1)
	ar.logger.Info("Collaboration initiated", map[string]interface{}{
		"collaboration_id": session.ID,
		"initiator":        initiatorID,
		"agents":           agentIDs,
		"strategy":         strategy,
	})

	return session, nil
}

// GetAgentStatus retrieves agent status
func (ar *AgentRegistry) GetAgentStatus(ctx context.Context, agentID string) (*AgentInfo, error) {
	val, ok := ar.agents.Load(agentID)
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}

	agent := val.(*AgentInfo)

	// Update last seen
	agent.LastSeen = time.Now()

	return agent, nil
}

// UpdateAgentStatus updates agent status
func (ar *AgentRegistry) UpdateAgentStatus(ctx context.Context, agentID, status string, metadata map[string]interface{}) error {
	val, ok := ar.agents.Load(agentID)
	if !ok {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	agent := val.(*AgentInfo)
	agent.Status = status
	agent.LastSeen = time.Now()

	// Update metadata if provided
	if metadata != nil {
		if agent.Metadata == nil {
			agent.Metadata = make(map[string]interface{})
		}
		for k, v := range metadata {
			agent.Metadata[k] = v
		}
	}

	ar.agents.Store(agentID, agent)

	ar.metrics.IncrementCounter(fmt.Sprintf("agent_status_%s", status), 1)
	return nil
}

// RemoveAgent removes an agent from registry
func (ar *AgentRegistry) RemoveAgent(agentID string) error {
	val, ok := ar.agents.Load(agentID)
	if !ok {
		return nil // Already removed
	}

	agent := val.(*AgentInfo)

	// Remove from capabilities index
	for _, capability := range agent.Capabilities {
		ar.removeCapability(capability, agentID)
	}

	// Remove agent
	ar.agents.Delete(agentID)

	ar.metrics.IncrementCounter("agents_removed", 1)
	return nil
}

// Helper methods

func (ar *AgentRegistry) addCapability(capability, agentID string) {
	val, ok := ar.capabilities.Load(capability)
	var agentIDs []string
	if ok {
		agentIDs = val.([]string)
	}

	// Add if not already present
	found := false
	for _, id := range agentIDs {
		if id == agentID {
			found = true
			break
		}
	}

	if !found {
		agentIDs = append(agentIDs, agentID)
		ar.capabilities.Store(capability, agentIDs)
		ar.logger.Info("Added agent to capability index", map[string]interface{}{
			"capability":   capability,
			"agent_id":     agentID,
			"total_agents": len(agentIDs),
		})
	}
}

func (ar *AgentRegistry) removeCapability(capability, agentID string) {
	val, ok := ar.capabilities.Load(capability)
	if !ok {
		return
	}

	agentIDs := val.([]string)
	newIDs := make([]string, 0, len(agentIDs))

	for _, id := range agentIDs {
		if id != agentID {
			newIDs = append(newIDs, id)
		}
	}

	if len(newIDs) > 0 {
		ar.capabilities.Store(capability, newIDs)
	} else {
		ar.capabilities.Delete(capability)
	}
}

// RemoveAgentByConnection removes an agent when connection is closed
// For in-memory registry, we don't track by connection ID, so this is a no-op
func (ar *AgentRegistry) RemoveAgentByConnection(connectionID string) error {
	// In-memory registry doesn't track agents by connection ID
	// This method is mainly for the database-backed implementation
	return nil
}
