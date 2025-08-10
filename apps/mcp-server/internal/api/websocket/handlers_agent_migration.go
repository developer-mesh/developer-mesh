package websocket

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// handleAgentRegisterIdempotent is the new idempotent registration handler
// It uses the proper architecture with manifests, configurations, and registrations
func (s *Server) handleAgentRegisterIdempotent(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	// Check if we have the enhanced registry
	enhancedRegistry, ok := s.agentRegistry.(*EnhancedAgentRegistry)
	if !ok {
		// Fall back to old handler if not using enhanced registry
		return s.handleAgentRegister(ctx, conn, params)
	}

	// Parse the registration parameters
	var registerParams struct {
		AgentID          string                 `json:"agent_id"`
		AgentType        string                 `json:"agent_type"`
		Name             string                 `json:"name"`
		Version          string                 `json:"version"`
		Description      string                 `json:"description"`
		Capabilities     []string               `json:"capabilities"`
		Requirements     map[string]interface{} `json:"requirements"`
		ModelPreferences map[string]interface{} `json:"model_preferences"`
		Metadata         map[string]interface{} `json:"metadata"`
		Channels         []struct {
			Type     string                 `json:"type"`
			Config   map[string]interface{} `json:"config"`
			Priority int                    `json:"priority"`
		} `json:"channels"`
		Auth struct {
			APIKey   string `json:"api_key"`
			TenantID string `json:"tenant_id"`
		} `json:"auth"`
		HealthCheckURL string `json:"health_check_url"`
	}

	if err := json.Unmarshal(params, &registerParams); err != nil {
		return nil, fmt.Errorf("invalid registration parameters: %w", err)
	}

	// Determine tenant ID
	tenantID := conn.TenantID
	if registerParams.Auth.TenantID != "" {
		tenantID = registerParams.Auth.TenantID
	}

	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}

	// Use connection ID as instance ID for idempotency
	instanceID := conn.ID

	// Build the universal registration request
	registration := &UniversalAgentRegistration{
		TenantID:    tenantUUID,
		AgentID:     registerParams.AgentID,
		AgentType:   registerParams.AgentType,
		InstanceID:  instanceID, // This is key for idempotency
		Name:        registerParams.Name,
		Version:     registerParams.Version,
		Description: registerParams.Description,
		Token:       registerParams.Auth.APIKey,
		Capabilities: map[string]interface{}{
			"capabilities": registerParams.Capabilities,
		},
		Requirements: registerParams.Requirements,
		RuntimeConfig: map[string]interface{}{
			"model_preferences": registerParams.ModelPreferences,
			"version":           registerParams.Version,
		},
		ConnectionDetails: map[string]interface{}{
			"connection_id": conn.ID,
			"protocol":      "websocket",
			"connected_at":  conn.CreatedAt,
		},
		Metadata:       registerParams.Metadata,
		ConnectionID:   conn.ID,
		HealthCheckURL: registerParams.HealthCheckURL,
	}

	// Add channels if specified
	for _, ch := range registerParams.Channels {
		registration.Channels = append(registration.Channels, ChannelConfig{
			Type:     ch.Type,
			Config:   ch.Config,
			Priority: ch.Priority,
		})
	}

	// Register using the enhanced registry (idempotent)
	agentInfo, err := enhancedRegistry.RegisterUniversalAgent(ctx, registration)
	if err != nil {
		s.logger.Error("Failed to register agent", map[string]interface{}{
			"error":       err.Error(),
			"agent_id":    registerParams.AgentID,
			"instance_id": instanceID,
			"tenant_id":   tenantID,
		})
		return nil, fmt.Errorf("failed to register agent: %w", err)
	}

	// Update connection with agent info
	conn.AgentID = agentInfo.AgentID

	// Log successful registration
	s.logger.Info("Agent registered successfully (idempotent)", map[string]interface{}{
		"agent_id":        agentInfo.AgentID,
		"instance_id":     instanceID,
		"tenant_id":       tenantID,
		"registration_id": agentInfo.RegistrationID,
		"manifest_id":     agentInfo.ManifestID,
		"is_reconnection": agentInfo.RegisteredAt.Before(conn.CreatedAt),
	})

	// Return success response
	return map[string]interface{}{
		"status":          "success",
		"agent_id":        agentInfo.AgentID,
		"instance_id":     instanceID,
		"registration_id": agentInfo.RegistrationID,
		"manifest_id":     agentInfo.ManifestID,
		"name":            agentInfo.Name,
		"version":         agentInfo.Version,
		"capabilities":    registerParams.Capabilities,
		"registered_at":   agentInfo.RegisteredAt.Format("2006-01-02T15:04:05Z"),
	}, nil
}

// handleAgentHeartbeatProper handles periodic health checks from agents
func (s *Server) handleAgentHeartbeatProper(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	// Check if we have enhanced registry
	enhancedRegistry, ok := s.agentRegistry.(*EnhancedAgentRegistry)
	if !ok {
		// No heartbeat handling in old registry
		return map[string]interface{}{
			"status": "acknowledged",
		}, nil
	}

	var heartbeatParams struct {
		Metrics map[string]interface{} `json:"metrics"`
		Status  string                 `json:"status"`
	}

	if err := json.Unmarshal(params, &heartbeatParams); err != nil {
		// Default to healthy if no params
		heartbeatParams.Status = "healthy"
	}

	// Update agent health status using the enhanced registry
	if conn.AgentID != "" {
		if err := enhancedRegistry.UpdateAgentHealth(ctx, conn.AgentID, &AgentHealthUpdate{
			Status:  heartbeatParams.Status,
			Metrics: heartbeatParams.Metrics,
		}); err != nil {
			s.logger.Warn("Failed to update agent health", map[string]interface{}{
				"error":       err.Error(),
				"agent_id":    conn.AgentID,
				"instance_id": conn.ID,
			})
		}
	}

	// Log the heartbeat
	s.logger.Debug("Agent heartbeat received", map[string]interface{}{
		"agent_id":    conn.AgentID,
		"instance_id": conn.ID,
		"status":      heartbeatParams.Status,
		"metrics":     heartbeatParams.Metrics,
	})

	return map[string]interface{}{
		"status":    "acknowledged",
		"timestamp": ctx.Value("timestamp"),
	}, nil
}

// OnAgentDisconnect handles agent disconnection
// This is called when a WebSocket connection is closed
func (s *Server) OnAgentDisconnect(ctx context.Context, conn *Connection) {
	// Check if we have enhanced registry
	enhancedRegistry, ok := s.agentRegistry.(*EnhancedAgentRegistry)
	if !ok {
		return
	}

	// Mark the registration as inactive
	if conn.AgentID != "" {
		// Update the registration status in the database
		if err := enhancedRegistry.UpdateAgentHealth(ctx, conn.AgentID, &AgentHealthUpdate{
			Status:  "disconnected",
			Message: "WebSocket connection closed",
		}); err != nil {
			s.logger.Warn("Failed to update agent status on disconnect", map[string]interface{}{
				"agent_id":    conn.AgentID,
				"instance_id": conn.ID,
				"error":       err.Error(),
			})
		}

		s.logger.Info("Agent disconnected and marked inactive", map[string]interface{}{
			"agent_id":    conn.AgentID,
			"instance_id": conn.ID,
		})
	}
}
