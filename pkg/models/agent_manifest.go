package models

import (
	"time"

	"github.com/google/uuid"
)

// AgentManifest represents a registered agent type with its capabilities
type AgentManifest struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	OrganizationID   uuid.UUID  `json:"organization_id" db:"organization_id"`
	AgentID          string     `json:"agent_id" db:"agent_id"`
	AgentType        string     `json:"agent_type" db:"agent_type"`
	Name             string     `json:"name" db:"name"`
	Version          string     `json:"version" db:"version"`
	Description      string     `json:"description" db:"description"`
	Capabilities     JSONMap    `json:"capabilities" db:"capabilities"`
	Requirements     JSONMap    `json:"requirements" db:"requirements"`
	ConnectionConfig JSONMap    `json:"connection_config" db:"connection_config"`
	AuthConfig       JSONMap    `json:"auth_config" db:"auth_config"`
	Metadata         JSONMap    `json:"metadata" db:"metadata"`
	Status           string     `json:"status" db:"status"`
	LastHeartbeat    *time.Time `json:"last_heartbeat" db:"last_heartbeat"`
	CreatedBy        *uuid.UUID `json:"created_by" db:"created_by"`
	UpdatedBy        *uuid.UUID `json:"updated_by" db:"updated_by"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}

// AgentRegistration represents an active instance of an agent
type AgentRegistration struct {
	ID                 uuid.UUID  `json:"id" db:"id"`
	ManifestID         uuid.UUID  `json:"manifest_id" db:"manifest_id"`
	TenantID           uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	InstanceID         string     `json:"instance_id" db:"instance_id"`
	RegistrationToken  string     `json:"registration_token" db:"registration_token"`
	RegistrationStatus string     `json:"registration_status" db:"registration_status"`
	ActivationDate     *time.Time `json:"activation_date" db:"activation_date"`
	ExpirationDate     *time.Time `json:"expiration_date" db:"expiration_date"`
	RuntimeConfig      JSONMap    `json:"runtime_config" db:"runtime_config"`
	ConnectionDetails  JSONMap    `json:"connection_details" db:"connection_details"`
	HealthStatus       string     `json:"health_status" db:"health_status"`
	HealthCheckURL     string     `json:"health_check_url" db:"health_check_url"`
	LastHealthCheck    *time.Time `json:"last_health_check" db:"last_health_check"`
	Metrics            JSONMap    `json:"metrics" db:"metrics"`
	CreatedAt          time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at" db:"updated_at"`
}

// ManifestCapability represents a specific capability of an agent manifest
type ManifestCapability struct {
	ID               uuid.UUID `json:"id" db:"id"`
	ManifestID       uuid.UUID `json:"manifest_id" db:"manifest_id"`
	CapabilityType   string    `json:"capability_type" db:"capability_type"`
	CapabilityName   string    `json:"capability_name" db:"capability_name"`
	CapabilityConfig JSONMap   `json:"capability_config" db:"capability_config"`
	Required         bool      `json:"required" db:"required"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
}

// AgentChannel represents a communication channel for an agent
type AgentChannel struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	RegistrationID uuid.UUID  `json:"registration_id" db:"registration_id"`
	ChannelType    string     `json:"channel_type" db:"channel_type"`
	ChannelConfig  JSONMap    `json:"channel_config" db:"channel_config"`
	Priority       int        `json:"priority" db:"priority"`
	Active         bool       `json:"active" db:"active"`
	LastMessageAt  *time.Time `json:"last_message_at" db:"last_message_at"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
}

// Agent Type constants
const (
	AgentTypeIDE        = "ide"
	AgentTypeSlack      = "slack"
	AgentTypeMonitoring = "monitoring"
	AgentTypeCI         = "ci"
	AgentTypeCustom     = "custom"
	AgentTypeWebhook    = "webhook"
	AgentTypeAPI        = "api"
)

// Manifest Status constants
const (
	ManifestStatusActive    = "active"
	ManifestStatusInactive  = "inactive"
	ManifestStatusPending   = "pending"
	ManifestStatusSuspended = "suspended"
	ManifestStatusArchived  = "archived"
)

// Registration Status constants
const (
	RegistrationStatusPending = "pending"
	RegistrationStatusActive  = "active"
	RegistrationStatusExpired = "expired"
	RegistrationStatusRevoked = "revoked"
	RegistrationStatusFailed  = "failed"
)

// Registration Health Status constants
const (
	RegistrationHealthHealthy   = "healthy"
	RegistrationHealthUnhealthy = "unhealthy"
	RegistrationHealthDegraded  = "degraded"
	RegistrationHealthUnknown   = "unknown"
)

// Channel Type constants
const (
	ChannelTypeWebSocket = "websocket"
	ChannelTypeHTTP      = "http"
	ChannelTypeGRPC      = "grpc"
	ChannelTypeRedis     = "redis"
	ChannelTypeKafka     = "kafka"
	ChannelTypeSQS       = "sqs"
)

// Capability Type constants
const (
	CapabilityTypeCommand      = "command"
	CapabilityTypeQuery        = "query"
	CapabilityTypeNotification = "notification"
	CapabilityTypeIntegration  = "integration"
	CapabilityTypeAutomation   = "automation"
	CapabilityTypeMonitoring   = "monitoring"
)

// IsActive checks if the agent manifest is active
func (am *AgentManifest) IsActive() bool {
	return am.Status == ManifestStatusActive
}

// IsHealthy checks if the agent registration is healthy
func (ar *AgentRegistration) IsHealthy() bool {
	return ar.HealthStatus == RegistrationHealthHealthy
}

// IsExpired checks if the registration has expired
func (ar *AgentRegistration) IsExpired() bool {
	if ar.ExpirationDate == nil {
		return false
	}
	return ar.ExpirationDate.Before(time.Now())
}

// NeedsHealthCheck checks if a health check is due
func (ar *AgentRegistration) NeedsHealthCheck(interval time.Duration) bool {
	if ar.LastHealthCheck == nil {
		return true
	}
	return time.Since(*ar.LastHealthCheck) > interval
}

// HasCapability checks if the manifest has a specific capability
func (am *AgentManifest) HasCapability(capabilityType, capabilityName string) bool {
	if am.Capabilities == nil {
		return false
	}

	// Check if capabilities is an array
	if caps, ok := am.Capabilities["capabilities"].([]interface{}); ok {
		for _, cap := range caps {
			if capMap, ok := cap.(map[string]interface{}); ok {
				if capMap["type"] == capabilityType && capMap["name"] == capabilityName {
					return true
				}
			}
		}
	}

	return false
}

// GetRequirement retrieves a specific requirement value
func (am *AgentManifest) GetRequirement(key string) (interface{}, bool) {
	if am.Requirements == nil {
		return nil, false
	}
	val, exists := am.Requirements[key]
	return val, exists
}

// IsHighPriority checks if the channel is high priority
func (ac *AgentChannel) IsHighPriority() bool {
	return ac.Priority >= 100
}

// GetConnectionParam retrieves a connection parameter
func (am *AgentManifest) GetConnectionParam(key string) (interface{}, bool) {
	if am.ConnectionConfig == nil {
		return nil, false
	}
	val, exists := am.ConnectionConfig[key]
	return val, exists
}

// GetAuthParam retrieves an authentication parameter
func (am *AgentManifest) GetAuthParam(key string) (interface{}, bool) {
	if am.AuthConfig == nil {
		return nil, false
	}
	val, exists := am.AuthConfig[key]
	return val, exists
}

// GetRuntimeParam retrieves a runtime configuration parameter
func (ar *AgentRegistration) GetRuntimeParam(key string) (interface{}, bool) {
	if ar.RuntimeConfig == nil {
		return nil, false
	}
	val, exists := ar.RuntimeConfig[key]
	return val, exists
}

// GetMetric retrieves a specific metric value
func (ar *AgentRegistration) GetMetric(key string) (interface{}, bool) {
	if ar.Metrics == nil {
		return nil, false
	}
	val, exists := ar.Metrics[key]
	return val, exists
}
