package models

import (
	"time"

	"github.com/google/uuid"
)

// AttributionLevel represents the level of cost attribution
type AttributionLevel string

const (
	AttributionLevelUser    AttributionLevel = "user"     // User-level attribution (strongest)
	AttributionLevelEdgeMCP AttributionLevel = "edge_mcp" // Machine identity attribution
	AttributionLevelTenant  AttributionLevel = "tenant"   // Tenant-level attribution (weakest)
	AttributionLevelSystem  AttributionLevel = "system"   // System-level (rare)
)

// AgentAttribution provides hierarchical cost and audit attribution
type AgentAttribution struct {
	Level        AttributionLevel `json:"level"`                  // Attribution level
	PrimaryID    string           `json:"primary_id"`             // Primary identifier for attribution
	SecondaryID  *string          `json:"secondary_id,omitempty"` // Secondary identifier (e.g., edge_mcp_id when user is primary)
	CostCenter   string           `json:"cost_center"`            // Who accumulates the costs
	BillableUnit string           `json:"billable_unit"`          // Who gets charged (usually tenant_id)
	UserID       *uuid.UUID       `json:"user_id,omitempty"`      // User ID if available
	EdgeMCPID    *string          `json:"edge_mcp_id,omitempty"`  // Edge MCP machine identity
	SessionID    *string          `json:"session_id,omitempty"`   // Session identifier
}

// Agent represents an AI agent in the system
type Agent struct {
	ID           string                 `json:"id" db:"id"`
	TenantID     uuid.UUID              `json:"tenant_id" db:"tenant_id"`
	Name         string                 `json:"name" db:"name"`
	ModelID      string                 `json:"model_id" db:"model_id"`
	Type         string                 `json:"type" db:"type"`
	Status       string                 `json:"status" db:"status"` // available, busy, offline
	Capabilities []string               `json:"capabilities" db:"capabilities"`
	Metadata     map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt    time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at" db:"updated_at"`
	LastSeenAt   *time.Time             `json:"last_seen_at" db:"last_seen_at"`

	// Attribution for virtual/session agents (not stored in DB, computed)
	Attribution *AgentAttribution `json:"attribution,omitempty" db:"-"`
}

// ResolveAttribution creates attribution metadata from session information
// Implements hierarchical attribution following 2025 best practices
func ResolveAttribution(session *EdgeMCPSession) AgentAttribution {
	if session == nil {
		return AgentAttribution{
			Level:        AttributionLevelSystem,
			PrimaryID:    "system",
			CostCenter:   "system",
			BillableUnit: "system",
		}
	}

	// Level 1: User attribution (strongest) - for user-authenticated sessions
	if session.UserID != nil {
		edgeMCPID := session.EdgeMCPID
		sessionID := session.SessionID
		return AgentAttribution{
			Level:        AttributionLevelUser,
			PrimaryID:    session.UserID.String(),
			SecondaryID:  &edgeMCPID,
			CostCenter:   session.UserID.String(),
			BillableUnit: session.TenantID.String(),
			UserID:       session.UserID,
			EdgeMCPID:    &edgeMCPID,
			SessionID:    &sessionID,
		}
	}

	// Level 2: Edge MCP attribution (machine identity) - for service accounts
	if session.EdgeMCPID != "" {
		edgeMCPID := session.EdgeMCPID
		sessionID := session.SessionID
		return AgentAttribution{
			Level:        AttributionLevelEdgeMCP,
			PrimaryID:    session.EdgeMCPID,
			SecondaryID:  nil,
			CostCenter:   session.EdgeMCPID,
			BillableUnit: session.TenantID.String(),
			UserID:       nil,
			EdgeMCPID:    &edgeMCPID,
			SessionID:    &sessionID,
		}
	}

	// Level 3: Tenant attribution (fallback)
	return AgentAttribution{
		Level:        AttributionLevelTenant,
		PrimaryID:    session.TenantID.String(),
		SecondaryID:  nil,
		CostCenter:   session.TenantID.String(),
		BillableUnit: session.TenantID.String(),
		UserID:       nil,
		EdgeMCPID:    nil,
		SessionID:    nil,
	}
}
