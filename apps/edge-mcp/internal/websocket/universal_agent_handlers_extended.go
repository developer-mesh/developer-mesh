package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/google/uuid"
)

// These handlers are methods on ExtendedServer to avoid type assertion issues

// HandleUniversalAgentRegister handles universal agent registration using the manifest system
func (es *ExtendedServer) HandleUniversalAgentRegister(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var registerParams struct {
		AgentType        string                 `json:"agent_type"` // ide, slack, monitoring, cicd, custom
		Name             string                 `json:"name"`
		Version          string                 `json:"version"`
		Description      string                 `json:"description"`
		Capabilities     models.JSONMap         `json:"capabilities"`
		Requirements     models.JSONMap         `json:"requirements"`
		RuntimeConfig    models.JSONMap         `json:"runtime_config"`
		ConnectionConfig models.JSONMap         `json:"connection_config"`
		AuthConfig       models.JSONMap         `json:"auth_config"`
		Metadata         map[string]interface{} `json:"metadata"`
		HealthCheckURL   string                 `json:"health_check_url"`
		Channels         []struct {
			Type     string         `json:"type"`
			Config   models.JSONMap `json:"config"`
			Priority int            `json:"priority"`
		} `json:"channels"`
	}

	if err := json.Unmarshal(params, &registerParams); err != nil {
		return nil, fmt.Errorf("invalid registration parameters: %w", err)
	}

	// Validate required fields
	if registerParams.AgentType == "" {
		return nil, fmt.Errorf("agent_type is required")
	}
	if registerParams.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	// Get organization ID from extended connection or tenant
	extConn := es.GetExtendedConnection(conn)
	orgID := extConn.OrganizationID
	if orgID == uuid.Nil && es.tenantRepo != nil {
		// Try to get from tenant
		if conn.TenantID != "" {
			tenant, err := es.tenantRepo.GetByTenantID(ctx, conn.TenantID)
			if err == nil && tenant != nil {
				// TenantConfig doesn't have OrganizationID directly
				// We'll need to get it from elsewhere or use a default
				// For now, generate a deterministic UUID from tenant ID
				orgID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(tenant.TenantID))
				extConn.OrganizationID = orgID
			}
		}

		if orgID == uuid.Nil {
			return nil, fmt.Errorf("organization ID not found in context")
		}
	}

	// Ensure we have a valid agent ID
	agentID := conn.AgentID
	if agentID == "" {
		agentID = fmt.Sprintf("%s-%s-%s", registerParams.AgentType, registerParams.Name, uuid.New().String()[:8])
		conn.AgentID = agentID
		es.logger.Info("Generated agent ID for universal registration", map[string]interface{}{
			"agent_id":      agentID,
			"agent_type":    registerParams.AgentType,
			"connection_id": conn.ID,
		})
	}

	// Get enhanced registry if available
	enhancedRegistry, ok := es.agentRegistry.(*EnhancedAgentRegistry)
	if !ok {
		// Fallback to standard registration
		es.logger.Warn("Enhanced registry not available, using standard registration", map[string]interface{}{
			"agent_type": registerParams.AgentType,
		})
		return es.Server.handleAgentRegister(ctx, conn, params) //nolint:staticcheck // Intentional call to base implementation
	}

	// Build registration request
	registration := &UniversalAgentRegistration{
		OrganizationID:    orgID,
		TenantID:          uuid.MustParse(conn.TenantID),
		AgentID:           agentID,
		AgentType:         registerParams.AgentType,
		InstanceID:        conn.ID,
		Name:              registerParams.Name,
		Version:           registerParams.Version,
		Description:       registerParams.Description,
		Token:             extConn.Token,
		Capabilities:      registerParams.Capabilities,
		Requirements:      registerParams.Requirements,
		RuntimeConfig:     registerParams.RuntimeConfig,
		ConnectionDetails: registerParams.ConnectionConfig,
		Metadata:          registerParams.Metadata,
		ConnectionID:      conn.ID,
		HealthCheckURL:    registerParams.HealthCheckURL,
	}

	// Add channels if specified
	for _, ch := range registerParams.Channels {
		registration.Channels = append(registration.Channels, ChannelConfig{
			Type:     ch.Type,
			Config:   ch.Config,
			Priority: ch.Priority,
		})
	}

	// Apply rate limiting for registration
	if es.rateLimiter != nil {
		agentLimiter, ok := es.rateLimiter.(*AgentRateLimiter)
		if ok {
			if err := agentLimiter.CheckAgentLimit(ctx, agentID, "register"); err != nil {
				es.logger.Warn("Agent registration rate limited", map[string]interface{}{
					"agent_id": agentID,
					"error":    err.Error(),
				})
				return nil, fmt.Errorf("rate limit exceeded: %w", err)
			}
		}
	}

	// Register the universal agent
	agentInfo, err := enhancedRegistry.RegisterUniversalAgent(ctx, registration)
	if err != nil {
		return nil, fmt.Errorf("failed to register universal agent: %w", err)
	}

	// Store agent info in extended connection
	extConn.AgentInfo = agentInfo

	// Send registration confirmation event
	es.broadcastAgentEvent(ctx, "agent.registered", map[string]interface{}{
		"agent_id":        agentInfo.AgentID,
		"agent_type":      agentInfo.AgentType,
		"manifest_id":     agentInfo.ManifestID,
		"registration_id": agentInfo.RegistrationID,
		"capabilities":    agentInfo.Capabilities,
		"channels":        agentInfo.Channels,
	})

	return map[string]interface{}{
		"manifest_id":     agentInfo.ManifestID,
		"registration_id": agentInfo.RegistrationID,
		"agent_id":        agentInfo.AgentID,
		"agent_type":      agentInfo.AgentType,
		"name":            agentInfo.Name,
		"version":         agentInfo.Version,
		"status":          agentInfo.Status,
		"health_status":   agentInfo.HealthStatus,
		"capabilities":    agentInfo.Capabilities,
		"channels":        agentInfo.Channels,
		"registered_at":   agentInfo.RegisteredAt.Format(time.RFC3339),
	}, nil
}

// HandleUniversalAgentDiscover discovers agents using capability-based routing
func (es *ExtendedServer) HandleUniversalAgentDiscover(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var discoverParams struct {
		AgentType            string   `json:"agent_type,omitempty"`
		RequiredCapabilities []string `json:"required_capabilities,omitempty"`
		OnlyHealthy          bool     `json:"only_healthy"`
		ExcludeSelf          bool     `json:"exclude_self"`
	}

	if err := json.Unmarshal(params, &discoverParams); err != nil {
		return nil, fmt.Errorf("invalid discover parameters: %w", err)
	}

	// Get enhanced registry
	enhancedRegistry, ok := es.agentRegistry.(*EnhancedAgentRegistry)
	if !ok {
		// Fallback to standard discovery
		return es.Server.handleAgentDiscover(ctx, conn, params) //nolint:staticcheck // Intentional call to base implementation
	}

	// Get extended connection
	extConn := es.GetExtendedConnection(conn)

	// Build filter
	filter := &UniversalAgentFilter{
		OrganizationID:       extConn.OrganizationID,
		TenantID:             uuid.MustParse(conn.TenantID),
		AgentType:            discoverParams.AgentType,
		RequiredCapabilities: discoverParams.RequiredCapabilities,
		OnlyHealthy:          discoverParams.OnlyHealthy,
		ExcludeSelf:          discoverParams.ExcludeSelf,
		SelfID:               conn.AgentID,
	}

	// Apply organization isolation
	if extConn.OrganizationID != uuid.Nil && es.orgRepo != nil {
		// Check if organization enforces strict isolation
		org, err := es.orgRepo.GetByID(ctx, extConn.OrganizationID)
		if err == nil && org != nil && org.IsStrictlyIsolated() {
			// Only discover agents within the same organization
			filter.OrganizationID = extConn.OrganizationID
			es.logger.Debug("Enforcing strict organization isolation for discovery", map[string]interface{}{
				"org_id": extConn.OrganizationID,
			})
		}
	}

	// Discover agents
	agents, err := enhancedRegistry.DiscoverUniversalAgents(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to discover agents: %w", err)
	}

	// Format response
	var results []map[string]interface{}
	for _, agent := range agents {
		// Skip cross-organization agents if not allowed
		if !es.canAccessAgent(ctx, conn, agent) {
			continue
		}

		results = append(results, map[string]interface{}{
			"agent_id":      agent.AgentID,
			"agent_type":    agent.AgentType,
			"name":          agent.Name,
			"version":       agent.Version,
			"status":        agent.Status,
			"health_status": agent.HealthStatus,
			"capabilities":  agent.Capabilities,
			"channels":      agent.Channels,
			"last_seen":     agent.LastSeen.Format(time.RFC3339),
		})
	}

	return map[string]interface{}{
		"agents": results,
		"count":  len(results),
	}, nil
}

// HandleAgentMessage handles message routing between agents
func (es *ExtendedServer) HandleAgentMessage(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var msgParams struct {
		TargetAgentID    string                 `json:"target_agent_id,omitempty"`
		TargetAgentType  string                 `json:"target_agent_type,omitempty"`
		TargetCapability string                 `json:"target_capability,omitempty"`
		MessageType      string                 `json:"message_type"`
		Payload          map[string]interface{} `json:"payload"`
		Context          map[string]interface{} `json:"context,omitempty"`
		Priority         int                    `json:"priority"`
		TTL              int64                  `json:"ttl,omitempty"`
	}

	if err := json.Unmarshal(params, &msgParams); err != nil {
		return nil, fmt.Errorf("invalid message parameters: %w", err)
	}

	// Validate message
	if msgParams.MessageType == "" {
		return nil, fmt.Errorf("message_type is required")
	}

	// Get agent type from extended connection
	extConn := es.GetExtendedConnection(conn)
	sourceAgentType := "standard"
	if info, ok := extConn.AgentInfo.(*UniversalAgentInfo); ok {
		sourceAgentType = info.AgentType
	}

	// Create agent message
	msg := &AgentMessage{
		ID:               uuid.New().String(),
		SourceAgentID:    conn.AgentID,
		SourceAgentType:  sourceAgentType,
		TargetAgentID:    msgParams.TargetAgentID,
		TargetAgentType:  msgParams.TargetAgentType,
		TargetCapability: msgParams.TargetCapability,
		MessageType:      msgParams.MessageType,
		Payload:          msgParams.Payload,
		Context:          msgParams.Context,
		Priority:         msgParams.Priority,
		TTL:              msgParams.TTL,
		Timestamp:        time.Now(),
	}

	// Apply rate limiting
	if es.rateLimiter != nil {
		agentLimiter, ok := es.rateLimiter.(*AgentRateLimiter)
		if ok {
			if err := agentLimiter.CheckAgentLimit(ctx, conn.AgentID, "message"); err != nil {
				return nil, fmt.Errorf("rate limit exceeded: %w", err)
			}
		}
	}

	// Send message through broker
	if es.messageBroker != nil {
		if err := es.messageBroker.SendMessage(ctx, msg); err != nil {
			return nil, fmt.Errorf("failed to send message: %w", err)
		}
	} else {
		return nil, fmt.Errorf("message broker not available")
	}

	// Record metrics
	if agentLimiter, ok := es.rateLimiter.(*AgentRateLimiter); ok {
		agentLimiter.RecordAgentRequest(ctx, conn.AgentID, "message", true)
	}

	return map[string]interface{}{
		"message_id":        msg.ID,
		"status":            "queued",
		"timestamp":         msg.Timestamp.Format(time.RFC3339),
		"target_agent":      msg.TargetAgentID,
		"target_capability": msg.TargetCapability,
	}, nil
}

// HandleAgentHealth handles agent health status updates
func (es *ExtendedServer) HandleAgentHealth(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var healthParams struct {
		Status  string                 `json:"status"` // healthy, degraded, unhealthy
		Message string                 `json:"message,omitempty"`
		Metrics map[string]interface{} `json:"metrics,omitempty"`
	}

	if err := json.Unmarshal(params, &healthParams); err != nil {
		return nil, fmt.Errorf("invalid health parameters: %w", err)
	}

	// Validate status
	validStatuses := map[string]bool{
		"healthy":   true,
		"degraded":  true,
		"unhealthy": true,
	}

	if !validStatuses[healthParams.Status] {
		return nil, fmt.Errorf("invalid health status: %s", healthParams.Status)
	}

	// Get enhanced registry
	enhancedRegistry, ok := es.agentRegistry.(*EnhancedAgentRegistry)
	if !ok {
		// Fallback to standard status update
		return es.Server.handleAgentUpdateStatus(ctx, conn, params) //nolint:staticcheck // Intentional call to base implementation
	}

	// Update health
	healthUpdate := &AgentHealthUpdate{
		Status:  healthParams.Status,
		Message: healthParams.Message,
		Metrics: healthParams.Metrics,
	}

	if err := enhancedRegistry.UpdateAgentHealth(ctx, conn.AgentID, healthUpdate); err != nil {
		return nil, fmt.Errorf("failed to update health: %w", err)
	}

	// Update circuit breaker state if unhealthy
	if es.circuitBreaker != nil {
		agentBreaker, ok := es.circuitBreaker.(*AgentCircuitBreaker)
		if ok && healthParams.Status == "unhealthy" {
			// Reset circuit breaker if agent is unhealthy
			agentBreaker.ResetAgentBreaker(conn.AgentID)
		}
	}

	// Broadcast health update event
	es.broadcastAgentEvent(ctx, "agent.health_updated", map[string]interface{}{
		"agent_id": conn.AgentID,
		"status":   healthParams.Status,
		"message":  healthParams.Message,
		"metrics":  healthParams.Metrics,
	})

	return map[string]interface{}{
		"status":     "updated",
		"agent_id":   conn.AgentID,
		"health":     healthParams.Status,
		"updated_at": time.Now().Format(time.RFC3339),
	}, nil
}
