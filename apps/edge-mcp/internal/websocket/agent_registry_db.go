package websocket

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	agentRepo "github.com/developer-mesh/developer-mesh/pkg/repository/agent"
	"github.com/google/uuid"
)

// Constants for agent types and statuses that align with the database model
const (
	// Agent types
	AgentTypeStandard    = "standard"
	AgentTypeSpecialized = "specialized"

	// Agent statuses - these map to the database model expectations
	AgentStatusAvailable = "available" // Agent is online and ready for tasks
	AgentStatusBusy      = "busy"      // Agent is working on tasks
	AgentStatusOffline   = "offline"   // Agent is not connected
)

// DBAgentRegistry is a database-backed implementation of agent registry
type DBAgentRegistry struct {
	repo    agentRepo.Repository
	cache   cache.Cache
	logger  observability.Logger
	metrics observability.MetricsClient

	// In-memory cache for real-time operations
	onlineAgents sync.Map // connection ID -> agent ID for fast lookup
}

// NewDBAgentRegistry creates a new database-backed agent registry
func NewDBAgentRegistry(repo agentRepo.Repository, cache cache.Cache, logger observability.Logger, metrics observability.MetricsClient) *DBAgentRegistry {
	return &DBAgentRegistry{
		repo:    repo,
		cache:   cache,
		logger:  logger,
		metrics: metrics,
	}
}

// RegisterAgent registers a new agent in the database or updates an existing one
func (ar *DBAgentRegistry) RegisterAgent(ctx context.Context, reg *AgentRegistration) (*AgentInfo, error) {
	// Validate agent ID
	if reg.ID == "" {
		return nil, fmt.Errorf("agent ID cannot be empty")
	}

	// Validate agent ID is a valid UUID
	if _, err := uuid.Parse(reg.ID); err != nil {
		ar.logger.Warn("Invalid agent ID format, generating new UUID", map[string]interface{}{
			"original_id": reg.ID,
			"error":       err.Error(),
		})
		reg.ID = uuid.New().String()
	}

	// Validate tenant ID
	if reg.TenantID == "" {
		return nil, fmt.Errorf("tenant ID cannot be empty")
	}

	// Ensure we don't use zero UUID
	zeroUUID := "00000000-0000-0000-0000-000000000000"
	if reg.ID == zeroUUID {
		// Generate a new UUID instead
		reg.ID = uuid.New().String()
		ar.logger.Info("Replaced zero UUID with new agent ID", map[string]interface{}{
			"new_agent_id": reg.ID,
			"name":         reg.Name,
		})
	}

	// Parse tenant UUID
	tenantUUID, err := uuid.Parse(reg.TenantID)
	if err != nil {
		ar.logger.Error("Failed to parse tenant ID", map[string]interface{}{
			"tenant_id": reg.TenantID,
			"error":     err.Error(),
		})
		return nil, fmt.Errorf("invalid tenant ID '%s': %w", reg.TenantID, err)
	}

	// Create agent model
	now := time.Now()
	agent := &models.Agent{
		ID:           reg.ID,
		TenantID:     tenantUUID,
		Name:         reg.Name,
		ModelID:      "claude-sonnet-4", // Default model ID - Latest Anthropic Sonnet (May 2025)
		Type:         AgentTypeStandard, // Default type
		Status:       AgentStatusAvailable,
		Capabilities: reg.Capabilities,
		Metadata:     reg.Metadata,
		LastSeenAt:   &now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Check if agent already exists
	existingAgent, getErr := ar.repo.Get(ctx, agent.ID)
	if getErr == nil && existingAgent != nil {
		// Agent exists, update it
		existingAgent.Name = agent.Name
		existingAgent.Capabilities = agent.Capabilities
		existingAgent.Metadata = agent.Metadata
		existingAgent.Status = agent.Status
		existingAgent.LastSeenAt = agent.LastSeenAt
		existingAgent.UpdatedAt = agent.UpdatedAt

		if err := ar.repo.Update(ctx, existingAgent); err != nil {
			return nil, fmt.Errorf("failed to update agent: %w", err)
		}

		ar.logger.Info("Agent updated", map[string]interface{}{
			"agent_id":     agent.ID,
			"name":         agent.Name,
			"capabilities": agent.Capabilities,
		})

		// Use the updated agent for the rest of the method
		agent = existingAgent
	} else {
		// Agent doesn't exist, create it
		if err := ar.repo.Create(ctx, agent); err != nil {
			return nil, fmt.Errorf("failed to register agent: %w", err)
		}
	}

	// Cache the agent
	cacheKey := fmt.Sprintf("agent:%s", agent.ID)
	if ar.cache != nil {
		if err := ar.cache.Set(ctx, cacheKey, agent, 5*time.Minute); err != nil {
			ar.logger.Error("Failed to cache agent", map[string]interface{}{
				"agent_id": agent.ID,
				"error":    err.Error(),
			})
		}
	}

	// Track online status in memory
	ar.onlineAgents.Store(reg.ConnectionID, agent.ID)

	// Convert to AgentInfo
	agentInfo := &AgentInfo{
		ID:           agent.ID,
		Name:         agent.Name,
		Capabilities: reg.Capabilities,
		Metadata:     reg.Metadata,
		ConnectionID: reg.ConnectionID,
		TenantID:     reg.TenantID,
		RegisteredAt: agent.CreatedAt,
		LastSeen:     *agent.LastSeenAt,
		Status:       "online",
		ActiveTasks:  0,
		Health:       "healthy",
	}

	if getErr == nil && existingAgent != nil {
		ar.metrics.IncrementCounter("agents_updated", 1)
	} else {
		ar.metrics.IncrementCounter("agents_registered", 1)
		ar.logger.Info("Agent registered in database", map[string]interface{}{
			"agent_id":     agent.ID,
			"name":         agent.Name,
			"capabilities": agent.Capabilities,
		})
	}

	return agentInfo, nil
}

// DiscoverAgents finds agents with required capabilities from the database
func (ar *DBAgentRegistry) DiscoverAgents(ctx context.Context, tenantID string, requiredCapabilities []string, excludeSelf bool, selfID string) ([]map[string]interface{}, error) {
	ar.logger.Info("DiscoverAgents called", map[string]interface{}{
		"tenant_id":             tenantID,
		"required_capabilities": requiredCapabilities,
		"exclude_self":          excludeSelf,
		"self_id":               selfID,
	})

	// Get all agents for the tenant
	agents, err := ar.repo.ListAgents(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	// Filter agents by capabilities and online status
	var result []map[string]interface{}
	for _, agent := range agents {
		// Skip self if requested
		if excludeSelf && agent.ID == selfID {
			continue
		}

		// Check if agent is online (check in-memory map)
		isOnline := false
		ar.onlineAgents.Range(func(connID, agentID interface{}) bool {
			if agentID.(string) == agent.ID {
				isOnline = true
				return false // Stop iteration
			}
			return true
		})

		if !isOnline || agent.Status != AgentStatusAvailable {
			continue
		}

		// Check if agent has all required capabilities
		hasAll := true
		for _, reqCap := range requiredCapabilities {
			found := false
			for _, agentCap := range agent.Capabilities {
				if string(agentCap) == reqCap {
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
			// Get workload info
			agentUUID, err := uuid.Parse(agent.ID)
			if err != nil {
				ar.logger.Error("Failed to parse agent ID", map[string]interface{}{
					"agent_id": agent.ID,
					"error":    err.Error(),
				})
				continue
			}
			workload, _ := ar.repo.GetWorkload(ctx, agentUUID)
			activeTasks := 0
			if workload != nil {
				activeTasks = workload.ActiveTasks
			}

			// Convert capabilities to string array
			capStrings := make([]string, len(agent.Capabilities))
			for i, cap := range agent.Capabilities {
				capStrings[i] = string(cap)
			}

			result = append(result, map[string]interface{}{
				"id":           agent.ID,
				"name":         agent.Name,
				"capabilities": capStrings,
				"status":       "online",
				"health":       "healthy", // TODO: Implement health check
				"active_tasks": activeTasks,
			})
		}
	}

	ar.logger.Info("Discovered agents", map[string]interface{}{
		"count": len(result),
	})

	return result, nil
}

// DelegateTask delegates a task to another agent
func (ar *DBAgentRegistry) DelegateTask(ctx context.Context, fromAgentID, toAgentID string, task map[string]interface{}, timeout time.Duration) (*DelegationResult, error) {
	// Get target agent from database
	agent, err := ar.repo.Get(ctx, toAgentID)
	if err != nil {
		return nil, fmt.Errorf("target agent not found: %w", err)
	}

	// Check if agent is online
	isOnline := false
	ar.onlineAgents.Range(func(connID, agentID interface{}) bool {
		if agentID.(string) == toAgentID {
			isOnline = true
			return false
		}
		return true
	})

	if !isOnline || agent.Status != AgentStatusAvailable {
		return nil, fmt.Errorf("target agent is not available")
	}

	// Update agent workload
	agentUUID, err := uuid.Parse(agent.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid agent ID '%s': %w", agent.ID, err)
	}
	workload, err := ar.repo.GetWorkload(ctx, agentUUID)
	if err != nil {
		// Create new workload entry
		workload = &models.AgentWorkload{
			AgentID:     agent.ID,
			ActiveTasks: 0,
			QueuedTasks: 0,
			TasksByType: make(map[string]int),
			LoadScore:   0.0,
		}
	}

	workload.ActiveTasks++
	workload.LoadScore = float64(workload.ActiveTasks) / 10.0 // Assume max 10 tasks
	if err := ar.repo.UpdateWorkload(ctx, workload); err != nil {
		ar.logger.Warn("Failed to update agent workload", map[string]interface{}{
			"agent_id": toAgentID,
			"error":    err.Error(),
		})
	}

	// Create delegation result
	result := &DelegationResult{
		TaskID:      uuid.New().String(),
		Status:      "delegated",
		DelegatedAt: time.Now(),
	}

	ar.metrics.IncrementCounter("tasks_delegated", 1)
	ar.logger.Info("Task delegated", map[string]interface{}{
		"task_id":    result.TaskID,
		"from_agent": fromAgentID,
		"to_agent":   toAgentID,
	})

	return result, nil
}

// InitiateCollaboration starts multi-agent collaboration
func (ar *DBAgentRegistry) InitiateCollaboration(ctx context.Context, initiatorID string, agentIDs []string, task map[string]interface{}, strategy string) (*CollaborationSession, error) {
	// Verify all agents exist and are online
	for _, agentID := range agentIDs {
		agent, err := ar.repo.Get(ctx, agentID)
		if err != nil {
			return nil, fmt.Errorf("agent not found: %s", agentID)
		}

		// Check if online
		isOnline := false
		ar.onlineAgents.Range(func(connID, aID interface{}) bool {
			if aID.(string) == agentID {
				isOnline = true
				return false
			}
			return true
		})

		if !isOnline || agent.Status != AgentStatusAvailable {
			return nil, fmt.Errorf("agent %s is not available", agentID)
		}
	}

	// Create collaboration session
	session := &CollaborationSession{
		ID:          uuid.New().String(),
		Agents:      append([]string{initiatorID}, agentIDs...),
		Strategy:    strategy,
		Status:      "initiated",
		InitiatedAt: time.Now(),
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

// GetAgentStatus retrieves agent status from database
func (ar *DBAgentRegistry) GetAgentStatus(ctx context.Context, agentID string) (*AgentInfo, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("agent:%s", agentID)
	if ar.cache != nil {
		var agent models.Agent
		err := ar.cache.Get(ctx, cacheKey, &agent)
		if err == nil {
			return ar.modelToAgentInfo(&agent), nil
		}
		// Cache miss or error, continue to database
	}

	// Get from database
	agent, err := ar.repo.Get(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	// Update last seen
	now := time.Now()
	agent.LastSeenAt = &now
	if err := ar.repo.Update(ctx, agent); err != nil {
		ar.logger.Warn("Failed to update agent last seen", map[string]interface{}{
			"agent_id": agentID,
			"error":    err.Error(),
		})
	}

	// Cache the agent
	if ar.cache != nil {
		if err := ar.cache.Set(ctx, cacheKey, agent, 5*time.Minute); err != nil {
			ar.logger.Error("Failed to cache agent", map[string]interface{}{
				"agent_id": agent.ID,
				"error":    err.Error(),
			})
		}
	}

	return ar.modelToAgentInfo(agent), nil
}

// UpdateAgentStatus updates agent status in database
func (ar *DBAgentRegistry) UpdateAgentStatus(ctx context.Context, agentID, status string, metadata map[string]interface{}) error {
	agent, err := ar.repo.Get(ctx, agentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	// Update status
	switch status {
	case "online", "available":
		agent.Status = AgentStatusAvailable
	case "busy":
		agent.Status = AgentStatusBusy
	case "offline":
		agent.Status = AgentStatusOffline
	default:
		agent.Status = AgentStatusOffline
	}

	now := time.Now()
	agent.LastSeenAt = &now

	// Update metadata if provided
	if metadata != nil {
		if agent.Metadata == nil {
			agent.Metadata = make(map[string]interface{})
		}
		for k, v := range metadata {
			agent.Metadata[k] = v
		}
	}

	if err := ar.repo.Update(ctx, agent); err != nil {
		return fmt.Errorf("failed to update agent status: %w", err)
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("agent:%s", agentID)
	if ar.cache != nil {
		if err := ar.cache.Delete(ctx, cacheKey); err != nil {
			ar.logger.Error("Failed to delete agent from cache", map[string]interface{}{
				"agent_id": agentID,
				"error":    err.Error(),
			})
		}
	}

	ar.metrics.IncrementCounter(fmt.Sprintf("agent_status_%s", status), 1)
	return nil
}

// RemoveAgent removes an agent from database and cleans up
func (ar *DBAgentRegistry) RemoveAgent(agentID string) error {
	ctx := context.Background()

	// Remove from database
	if err := ar.repo.Delete(ctx, agentID); err != nil {
		return fmt.Errorf("failed to remove agent: %w", err)
	}

	// Remove from online agents map
	ar.onlineAgents.Range(func(connID, aID interface{}) bool {
		if aID.(string) == agentID {
			ar.onlineAgents.Delete(connID)
			return false
		}
		return true
	})

	// Invalidate cache
	cacheKey := fmt.Sprintf("agent:%s", agentID)
	if ar.cache != nil {
		if err := ar.cache.Delete(ctx, cacheKey); err != nil {
			ar.logger.Error("Failed to delete agent from cache", map[string]interface{}{
				"agent_id": agentID,
				"error":    err.Error(),
			})
		}
	}

	ar.metrics.IncrementCounter("agents_removed", 1)
	ar.logger.Info("Agent removed", map[string]interface{}{
		"agent_id": agentID,
	})

	return nil
}

// RemoveAgentByConnection removes an agent when connection is closed
func (ar *DBAgentRegistry) RemoveAgentByConnection(connectionID string) error {
	// Find agent ID by connection
	var agentID string
	if val, ok := ar.onlineAgents.Load(connectionID); ok {
		agentID = val.(string)
		ar.onlineAgents.Delete(connectionID)
	} else {
		return nil // No agent for this connection
	}

	// Update agent status to offline
	ctx := context.Background()
	return ar.UpdateAgentStatus(ctx, agentID, "offline", nil)
}

// Helper method to convert models.Agent to AgentInfo
func (ar *DBAgentRegistry) modelToAgentInfo(agent *models.Agent) *AgentInfo {
	// Convert metadata
	metadata := make(map[string]interface{})
	if agent.Metadata != nil {
		for k, v := range agent.Metadata {
			metadata[k] = v
		}
	}

	// Determine status string
	status := "offline"
	switch agent.Status {
	case AgentStatusAvailable:
		status = "online"
	case AgentStatusBusy:
		status = "busy"
	}

	// Get workload info
	activeTasks := 0
	agentUUID, err := uuid.Parse(agent.ID)
	if err != nil {
		ar.logger.Error("Failed to parse agent ID in modelToAgentInfo", map[string]interface{}{
			"agent_id": agent.ID,
			"error":    err.Error(),
		})
		// Use zero value for activeTasks if we can't parse the ID
	} else {
		workload, _ := ar.repo.GetWorkload(context.Background(), agentUUID)
		if workload != nil {
			activeTasks = workload.ActiveTasks
		}
	}

	return &AgentInfo{
		ID:           agent.ID,
		Name:         agent.Name,
		Capabilities: agent.Capabilities,
		Metadata:     metadata,
		TenantID:     agent.TenantID.String(),
		RegisteredAt: agent.CreatedAt,
		LastSeen:     *agent.LastSeenAt,
		Status:       status,
		ActiveTasks:  activeTasks,
		Health:       "healthy", // TODO: Implement proper health status
	}
}
