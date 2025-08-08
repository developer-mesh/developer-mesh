// Package agents provides comprehensive agent lifecycle management
package agents

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AgentState represents the lifecycle state of an agent
type AgentState string

const (
	StatePending     AgentState = "pending"     // Initial registration, awaiting configuration
	StateConfiguring AgentState = "configuring" // Configuration in progress
	StateValidating  AgentState = "validating"  // Validating capabilities and connections
	StateReady       AgentState = "ready"       // Ready for activation
	StateActive      AgentState = "active"      // Fully operational
	StateDegraded    AgentState = "degraded"    // Partial failure, some capabilities unavailable
	StateSuspended   AgentState = "suspended"   // Temporarily disabled by admin
	StateTerminating AgentState = "terminating" // Cleanup in progress
	StateTerminated  AgentState = "terminated"  // Fully removed (terminal state)
)

// String returns the string representation of the agent state
func (s AgentState) String() string {
	return string(s)
}

// IsValid checks if the state is valid
func (s AgentState) IsValid() bool {
	switch s {
	case StatePending, StateConfiguring, StateValidating, StateReady,
		StateActive, StateDegraded, StateSuspended, StateTerminating, StateTerminated:
		return true
	}
	return false
}

// IsOperational returns true if the agent can handle requests
func (s AgentState) IsOperational() bool {
	return s == StateActive || s == StateDegraded
}

// IsTerminal returns true if this is a terminal state
func (s AgentState) IsTerminal() bool {
	return s == StateTerminated
}

// Scan implements sql.Scanner for database operations
func (s *AgentState) Scan(value interface{}) error {
	if value == nil {
		*s = StatePending
		return nil
	}
	switch v := value.(type) {
	case string:
		*s = AgentState(v)
	case []byte:
		*s = AgentState(v)
	default:
		return fmt.Errorf("cannot scan %T into AgentState", value)
	}
	if !s.IsValid() {
		return fmt.Errorf("invalid agent state: %s", *s)
	}
	return nil
}

// Value implements driver.Valuer for database operations
func (s AgentState) Value() (driver.Value, error) {
	if !s.IsValid() {
		return nil, fmt.Errorf("invalid agent state: %s", s)
	}
	return string(s), nil
}

// StateTransitions defines valid state transitions
var StateTransitions = map[AgentState][]AgentState{
	StatePending:     {StateConfiguring, StateTerminating},
	StateConfiguring: {StateValidating, StatePending, StateTerminating},
	StateValidating:  {StateReady, StateConfiguring, StateTerminating},
	StateReady:       {StateActive, StateValidating, StateTerminating},
	StateActive:      {StateDegraded, StateSuspended, StateTerminating},
	StateDegraded:    {StateActive, StateSuspended, StateTerminating},
	StateSuspended:   {StateActive, StateTerminating},
	StateTerminating: {StateTerminated},
	StateTerminated:  {}, // Terminal state, no transitions
}

// CanTransitionTo checks if a transition from current state to target is valid
func (s AgentState) CanTransitionTo(target AgentState) bool {
	validTargets, exists := StateTransitions[s]
	if !exists {
		return false
	}
	for _, valid := range validTargets {
		if valid == target {
			return true
		}
	}
	return false
}

// Agent represents a complete agent with lifecycle management
type Agent struct {
	// Core identification
	ID       uuid.UUID `json:"id" db:"id"`
	TenantID uuid.UUID `json:"tenant_id" db:"tenant_id"`
	Name     string    `json:"name" db:"name"`
	Type     string    `json:"type" db:"type"`

	// State management
	State          AgentState `json:"state" db:"state"`
	StateReason    string     `json:"state_reason" db:"state_reason"`
	StateChangedAt time.Time  `json:"state_changed_at" db:"state_changed_at"`
	StateChangedBy *uuid.UUID `json:"state_changed_by,omitempty" db:"state_changed_by"`

	// Configuration
	ModelID       string                 `json:"model_id,omitempty" db:"model_id"`
	Capabilities  []string               `json:"capabilities" db:"capabilities"`
	Configuration map[string]interface{} `json:"configuration" db:"configuration"`
	SystemPrompt  string                 `json:"system_prompt,omitempty" db:"system_prompt"`
	Temperature   float64                `json:"temperature" db:"temperature"`
	MaxTokens     int                    `json:"max_tokens" db:"max_tokens"`

	// Health and monitoring
	HealthStatus    map[string]interface{} `json:"health_status" db:"health_status"`
	HealthCheckedAt *time.Time             `json:"health_checked_at,omitempty" db:"health_checked_at"`
	LastError       string                 `json:"last_error,omitempty" db:"last_error"`
	LastErrorAt     *time.Time             `json:"last_error_at,omitempty" db:"last_error_at"`
	RetryCount      int                    `json:"retry_count" db:"retry_count"`

	// Workload management
	CurrentWorkload int `json:"current_workload" db:"current_workload"`
	MaxWorkload     int `json:"max_workload" db:"max_workload"`

	// Operational metrics
	Version         int                    `json:"version" db:"version"`
	ActivationCount int                    `json:"activation_count" db:"activation_count"`
	LastSeenAt      *time.Time             `json:"last_seen_at,omitempty" db:"last_seen_at"`
	Metadata        map[string]interface{} `json:"metadata" db:"metadata"`

	// Timestamps
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// IsAvailable checks if the agent can accept new work
func (a *Agent) IsAvailable() bool {
	return a.State.IsOperational() && a.CurrentWorkload < a.MaxWorkload
}

// CanTransitionTo checks if the agent can transition to the target state
func (a *Agent) CanTransitionTo(target AgentState) bool {
	return a.State.CanTransitionTo(target)
}

// AgentEvent represents a state change or significant event in agent lifecycle
type AgentEvent struct {
	ID            uuid.UUID              `json:"id" db:"id"`
	AgentID       uuid.UUID              `json:"agent_id" db:"agent_id"`
	TenantID      uuid.UUID              `json:"tenant_id" db:"tenant_id"`
	EventType     string                 `json:"event_type" db:"event_type"`
	EventVersion  string                 `json:"event_version" db:"event_version"`
	FromState     *AgentState            `json:"from_state,omitempty" db:"from_state"`
	ToState       *AgentState            `json:"to_state,omitempty" db:"to_state"`
	Payload       map[string]interface{} `json:"payload" db:"payload"`
	ErrorMessage  string                 `json:"error_message,omitempty" db:"error_message"`
	ErrorCode     string                 `json:"error_code,omitempty" db:"error_code"`
	InitiatedBy   *uuid.UUID             `json:"initiated_by,omitempty" db:"initiated_by"`
	CorrelationID *uuid.UUID             `json:"correlation_id,omitempty" db:"correlation_id"`
	CreatedAt     time.Time              `json:"created_at" db:"created_at"`
}

// AgentSession represents an active connection session for an agent
type AgentSession struct {
	ID                 uuid.UUID              `json:"id" db:"id"`
	AgentID            uuid.UUID              `json:"agent_id" db:"agent_id"`
	TenantID           uuid.UUID              `json:"tenant_id" db:"tenant_id"`
	SessionToken       string                 `json:"session_token" db:"session_token"`
	ConnectionType     string                 `json:"connection_type" db:"connection_type"`
	ConnectionMetadata map[string]interface{} `json:"connection_metadata" db:"connection_metadata"`
	IPAddress          string                 `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent          string                 `json:"user_agent,omitempty" db:"user_agent"`
	StartedAt          time.Time              `json:"started_at" db:"started_at"`
	LastActivityAt     time.Time              `json:"last_activity_at" db:"last_activity_at"`
	EndedAt            *time.Time             `json:"ended_at,omitempty" db:"ended_at"`
	EndReason          string                 `json:"end_reason,omitempty" db:"end_reason"`
	Metrics            map[string]interface{} `json:"metrics" db:"metrics"`
}

// IsActive returns true if the session is still active
func (s *AgentSession) IsActive() bool {
	return s.EndedAt == nil
}

// AgentCapability represents a capability that an agent possesses
type AgentCapability struct {
	ID                 uuid.UUID              `json:"id" db:"id"`
	AgentID            uuid.UUID              `json:"agent_id" db:"agent_id"`
	CapabilityName     string                 `json:"capability_name" db:"capability_name"`
	CapabilityVersion  string                 `json:"capability_version,omitempty" db:"capability_version"`
	CapabilityType     string                 `json:"capability_type" db:"capability_type"`
	Configuration      map[string]interface{} `json:"configuration" db:"configuration"`
	IsEnabled          bool                   `json:"is_enabled" db:"is_enabled"`
	Priority           int                    `json:"priority" db:"priority"`
	Dependencies       []string               `json:"dependencies" db:"dependencies"`
	HealthEndpoint     string                 `json:"health_endpoint,omitempty" db:"health_endpoint"`
	LastHealthCheck    *time.Time             `json:"last_health_check,omitempty" db:"last_health_check"`
	HealthStatus       string                 `json:"health_status" db:"health_status"`
	PerformanceMetrics map[string]interface{} `json:"performance_metrics" db:"performance_metrics"`
	ValidatedAt        *time.Time             `json:"validated_at,omitempty" db:"validated_at"`
	ValidationErrors   map[string]interface{} `json:"validation_errors,omitempty" db:"validation_errors"`
}

// AgentHealthMetric represents a health metric for monitoring
type AgentHealthMetric struct {
	AgentID    uuid.UUID              `json:"agent_id" db:"agent_id"`
	TenantID   uuid.UUID              `json:"tenant_id" db:"tenant_id"`
	MetricType string                 `json:"metric_type" db:"metric_type"`
	MetricName string                 `json:"metric_name" db:"metric_name"`
	Value      float64                `json:"value" db:"value"`
	Unit       string                 `json:"unit,omitempty" db:"unit"`
	Tags       map[string]interface{} `json:"tags" db:"tags"`
	Metadata   map[string]interface{} `json:"metadata" db:"metadata"`
	RecordedAt time.Time              `json:"recorded_at" db:"recorded_at"`
}

// RegisterAgentRequest represents a request to register a new agent
type RegisterAgentRequest struct {
	Name         string                 `json:"name" validate:"required,min=3,max=255"`
	Type         string                 `json:"type" validate:"required,oneof=ide slack cicd monitoring custom"`
	TenantID     uuid.UUID              `json:"tenant_id" validate:"required"`
	Identifier   string                 `json:"identifier,omitempty"` // For deterministic ID generation
	ModelID      string                 `json:"model_id,omitempty"`
	Capabilities []string               `json:"capabilities,omitempty"`
	Config       map[string]interface{} `json:"configuration,omitempty"`
	SystemPrompt string                 `json:"system_prompt,omitempty"`
	Temperature  float64                `json:"temperature,omitempty" validate:"min=0,max=2"`
	MaxTokens    int                    `json:"max_tokens,omitempty" validate:"min=1,max=32768"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateAgentRequest represents a request to update an agent
type UpdateAgentRequest struct {
	Name         *string                 `json:"name,omitempty" validate:"omitempty,min=3,max=255"`
	ModelID      *string                 `json:"model_id,omitempty"`
	Capabilities *[]string               `json:"capabilities,omitempty"`
	Config       *map[string]interface{} `json:"configuration,omitempty"`
	SystemPrompt *string                 `json:"system_prompt,omitempty"`
	Temperature  *float64                `json:"temperature,omitempty" validate:"omitempty,min=0,max=2"`
	MaxTokens    *int                    `json:"max_tokens,omitempty" validate:"omitempty,min=1,max=32768"`
	MaxWorkload  *int                    `json:"max_workload,omitempty" validate:"omitempty,min=1,max=1000"`
	Metadata     *map[string]interface{} `json:"metadata,omitempty"`
}

// StateTransitionRequest represents a request to transition agent state
type StateTransitionRequest struct {
	TargetState AgentState `json:"target_state" validate:"required"`
	Reason      string     `json:"reason" validate:"required,min=3,max=500"`
	InitiatedBy uuid.UUID  `json:"initiated_by" validate:"required"`
}

// AgentFilter provides filtering options for listing agents
type AgentFilter struct {
	TenantID     *uuid.UUID   `json:"tenant_id,omitempty"`
	Type         *string      `json:"type,omitempty"`
	State        *AgentState  `json:"state,omitempty"`
	States       []AgentState `json:"states,omitempty"`
	Capabilities []string     `json:"capabilities,omitempty"`
	IsAvailable  *bool        `json:"is_available,omitempty"`
	Limit        int          `json:"limit,omitempty"`
	Offset       int          `json:"offset,omitempty"`
}

// EventFilter provides filtering options for querying events
type EventFilter struct {
	AgentID       *uuid.UUID  `json:"agent_id,omitempty"`
	TenantID      *uuid.UUID  `json:"tenant_id,omitempty"`
	EventType     *string     `json:"event_type,omitempty"`
	FromState     *AgentState `json:"from_state,omitempty"`
	ToState       *AgentState `json:"to_state,omitempty"`
	CorrelationID *uuid.UUID  `json:"correlation_id,omitempty"`
	StartTime     *time.Time  `json:"start_time,omitempty"`
	EndTime       *time.Time  `json:"end_time,omitempty"`
	Limit         int         `json:"limit,omitempty"`
	Offset        int         `json:"offset,omitempty"`
}
