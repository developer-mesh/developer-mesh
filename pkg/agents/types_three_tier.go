package agents

import (
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// Three-Tier Architecture Types
// ============================================================================

// AgentManifest represents an agent type/blueprint definition
// This is the top-level definition that describes what an agent can do
type AgentManifest struct {
	ID           uuid.UUID              `json:"id" db:"id"`
	AgentID      string                 `json:"agent_id" db:"agent_id"`         // Unique identifier like "ide-agent", "slack-bot"
	AgentType    string                 `json:"agent_type" db:"agent_type"`     // Category: "ide", "chat", "automation"
	Name         string                 `json:"name" db:"name"`                 // Human-readable name
	Description  string                 `json:"description" db:"description"`   // What this agent does
	Version      string                 `json:"version" db:"version"`           // Semantic version
	Capabilities map[string]interface{} `json:"capabilities" db:"capabilities"` // What the agent can do
	Requirements string                 `json:"requirements" db:"requirements"` // System requirements
	Metadata     map[string]interface{} `json:"metadata" db:"metadata"`         // Additional metadata
	Status       string                 `json:"status" db:"status"`             // active, deprecated, beta
	CreatedAt    time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at" db:"updated_at"`
}

// AgentConfiguration represents a tenant-specific configuration of an agent manifest
// This is how a specific tenant has configured an agent type for their use
type AgentConfiguration struct {
	ID              uuid.UUID              `json:"id" db:"id"`
	TenantID        uuid.UUID              `json:"tenant_id" db:"tenant_id"`
	ManifestID      uuid.UUID              `json:"manifest_id" db:"manifest_id"`           // References the agent type
	Name            string                 `json:"name" db:"name"`                         // Tenant's name for this config
	Enabled         bool                   `json:"enabled" db:"enabled"`                   // Is this config active?
	Configuration   map[string]interface{} `json:"configuration" db:"configuration"`       // Tenant-specific settings
	SystemPrompt    string                 `json:"system_prompt" db:"system_prompt"`       // LLM system prompt
	Temperature     float64                `json:"temperature" db:"temperature"`           // LLM temperature
	MaxTokens       int                    `json:"max_tokens" db:"max_tokens"`             // LLM max tokens
	ModelID         uuid.UUID              `json:"model_id" db:"model_id"`                 // Which model to use
	MaxWorkload     int                    `json:"max_workload" db:"max_workload"`         // Max concurrent tasks
	CurrentWorkload int                    `json:"current_workload" db:"current_workload"` // Current task count
	CreatedAt       time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at" db:"updated_at"`
}

// AgentRegistration represents an active instance of an agent
// This is a running instance - could be multiple per configuration (e.g., multiple IDE instances)
type AgentRegistration struct {
	ID                 uuid.UUID              `json:"id" db:"id"`
	ManifestID         uuid.UUID              `json:"manifest_id" db:"manifest_id"`
	TenantID           uuid.UUID              `json:"tenant_id" db:"tenant_id"`
	InstanceID         string                 `json:"instance_id" db:"instance_id"`                 // Unique instance identifier
	RegistrationStatus string                 `json:"registration_status" db:"registration_status"` // active, inactive
	HealthStatus       string                 `json:"health_status" db:"health_status"`             // healthy, degraded, unknown, disconnected
	ConnectionDetails  map[string]interface{} `json:"connection_details" db:"connection_details"`   // WebSocket ID, IP, etc.
	RuntimeConfig      map[string]interface{} `json:"runtime_config" db:"runtime_config"`           // Runtime configuration
	ActivationDate     *time.Time             `json:"activation_date" db:"activation_date"`         // When activated
	DeactivationDate   *time.Time             `json:"deactivation_date" db:"deactivation_date"`     // When deactivated
	LastHealthCheck    *time.Time             `json:"last_health_check" db:"last_health_check"`     // Last health ping
	FailureCount       int                    `json:"failure_count" db:"failure_count"`             // Consecutive failures
	CreatedAt          time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at" db:"updated_at"`

	// Denormalized fields from manifest for convenience
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`
}

// ============================================================================
// Query Results and Filters
// ============================================================================

// RegistrationResult is returned when registering an agent instance
type RegistrationResult struct {
	RegistrationID uuid.UUID `json:"registration_id"`
	ManifestID     uuid.UUID `json:"manifest_id"`
	ConfigID       uuid.UUID `json:"config_id"`
	IsNew          bool      `json:"is_new"`
	Message        string    `json:"message"`
}

// AvailableAgent represents an agent that's ready to take work
type AvailableAgent struct {
	// Configuration info
	ConfigID        uuid.UUID `json:"config_id"`
	ConfigName      string    `json:"config_name"`
	ModelID         uuid.UUID `json:"model_id"`
	SystemPrompt    string    `json:"system_prompt"`
	Temperature     float64   `json:"temperature"`
	MaxTokens       int       `json:"max_tokens"`
	CurrentWorkload int       `json:"current_workload"`
	MaxWorkload     int       `json:"max_workload"`

	// Manifest info
	ManifestID   uuid.UUID              `json:"manifest_id"`
	AgentID      string                 `json:"agent_id"`
	AgentType    string                 `json:"agent_type"`
	Capabilities map[string]interface{} `json:"capabilities"`
	Version      string                 `json:"version"`

	// Instance info
	ActiveInstances   int     `json:"active_instances"`
	HealthyInstances  int     `json:"healthy_instances"`
	AvailabilityScore float64 `json:"availability_score"` // Calculated score for selection
}

// AgentMetrics contains metrics for an agent configuration
type AgentMetrics struct {
	ConfigID             uuid.UUID  `json:"config_id"`
	Name                 string     `json:"name"`
	CurrentWorkload      int        `json:"current_workload"`
	MaxWorkload          int        `json:"max_workload"`
	TotalRegistrations   int        `json:"total_registrations"`
	HealthyRegistrations int        `json:"healthy_registrations"`
	ActiveRegistrations  int        `json:"active_registrations"`
	AvgFailureCount      int        `json:"avg_failure_count"`
	LastActivity         *time.Time `json:"last_activity"`
	WorkloadUtilization  float64    `json:"workload_utilization"`
	HealthRate           float64    `json:"health_rate"`
}

// ============================================================================
// Filter Types
// ============================================================================

// ManifestFilter filters agent manifests
type ManifestFilter struct {
	AgentType string `json:"agent_type"`
	Status    string `json:"status"`
}

// ConfigurationFilter filters agent configurations
type ConfigurationFilter struct {
	Enabled    *bool      `json:"enabled"`
	ManifestID *uuid.UUID `json:"manifest_id"`
}

// RegistrationFilter filters agent registrations
type RegistrationFilter struct {
	ManifestID         *uuid.UUID `json:"manifest_id"`
	RegistrationStatus string     `json:"registration_status"`
	HealthStatus       string     `json:"health_status"`
}

// ============================================================================
// Health and Status Types
// ============================================================================

// HealthStatus represents the health of an agent registration
type HealthStatus string

const (
	HealthStatusHealthy      HealthStatus = "healthy"
	HealthStatusDegraded     HealthStatus = "degraded"
	HealthStatusUnknown      HealthStatus = "unknown"
	HealthStatusDisconnected HealthStatus = "disconnected"
)

// RegistrationStatus represents the status of an agent registration
type RegistrationStatus string

const (
	RegistrationStatusActive   RegistrationStatus = "active"
	RegistrationStatusInactive RegistrationStatus = "inactive"
)

// ManifestStatus represents the status of an agent manifest
type ManifestStatus string

const (
	ManifestStatusActive     ManifestStatus = "active"
	ManifestStatusDeprecated ManifestStatus = "deprecated"
	ManifestStatusBeta       ManifestStatus = "beta"
)

// ============================================================================
// Event Types
// ============================================================================

// ThreeTierAgentEvent represents an event in the three-tier agent lifecycle
type ThreeTierAgentEvent struct {
	ID           uuid.UUID              `json:"id" db:"id"`
	AgentID      uuid.UUID              `json:"agent_id" db:"agent_id"` // References configuration ID
	TenantID     uuid.UUID              `json:"tenant_id" db:"tenant_id"`
	EventType    string                 `json:"event_type" db:"event_type"` // registration, health_check, task_assigned, etc.
	EventVersion string                 `json:"event_version" db:"event_version"`
	Payload      map[string]interface{} `json:"payload" db:"payload"`
	InitiatedBy  *uuid.UUID             `json:"initiated_by" db:"initiated_by"`
	CreatedAt    time.Time              `json:"created_at" db:"created_at"`
}
