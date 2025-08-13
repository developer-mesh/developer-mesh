package websocket

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/resilience"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// AgentCircuitBreaker extends circuit breaker functionality for agents
type AgentCircuitBreaker struct {
	// Circuit breakers per agent
	agentBreakers sync.Map // agent_id -> *resilience.CircuitBreaker

	// Circuit breakers per capability
	capabilityBreakers sync.Map // capability -> *resilience.CircuitBreaker

	// Circuit breakers per tenant
	tenantBreakers sync.Map // tenant_id -> *resilience.CircuitBreaker

	// Circuit breakers per channel type
	channelBreakers sync.Map // channel_type -> *resilience.CircuitBreaker

	// Components
	manifestRepo repository.AgentManifestRepository
	orgRepo      repository.OrganizationRepository
	logger       observability.Logger
	metrics      observability.MetricsClient

	// Configuration
	defaultConfig    resilience.CircuitBreakerConfig
	agentConfig      resilience.CircuitBreakerConfig
	capabilityConfig resilience.CircuitBreakerConfig
	tenantConfig     resilience.CircuitBreakerConfig
	channelConfig    resilience.CircuitBreakerConfig
}

// AgentCircuitBreakerConfig contains configuration for agent circuit breakers
type AgentCircuitBreakerConfig struct {
	// Default configuration for all breakers
	DefaultConfig resilience.CircuitBreakerConfig

	// Specific configurations
	AgentConfig      resilience.CircuitBreakerConfig
	CapabilityConfig resilience.CircuitBreakerConfig
	TenantConfig     resilience.CircuitBreakerConfig
	ChannelConfig    resilience.CircuitBreakerConfig
}

// DefaultAgentCircuitBreakerConfig returns sensible defaults
func DefaultAgentCircuitBreakerConfig() *AgentCircuitBreakerConfig {
	return &AgentCircuitBreakerConfig{
		DefaultConfig: resilience.CircuitBreakerConfig{
			FailureThreshold:    5,
			FailureRatio:        0.6,
			ResetTimeout:        30 * time.Second,
			SuccessThreshold:    2,
			TimeoutThreshold:    5 * time.Second,
			MaxRequestsHalfOpen: 5,
			MinimumRequestCount: 10,
		},
		AgentConfig: resilience.CircuitBreakerConfig{
			FailureThreshold:    3,
			FailureRatio:        0.5,
			ResetTimeout:        20 * time.Second,
			SuccessThreshold:    2,
			TimeoutThreshold:    10 * time.Second,
			MaxRequestsHalfOpen: 3,
			MinimumRequestCount: 5,
		},
		CapabilityConfig: resilience.CircuitBreakerConfig{
			FailureThreshold:    10,
			FailureRatio:        0.7,
			ResetTimeout:        60 * time.Second,
			SuccessThreshold:    3,
			TimeoutThreshold:    15 * time.Second,
			MaxRequestsHalfOpen: 10,
			MinimumRequestCount: 20,
		},
		TenantConfig: resilience.CircuitBreakerConfig{
			FailureThreshold:    20,
			FailureRatio:        0.8,
			ResetTimeout:        120 * time.Second,
			SuccessThreshold:    5,
			TimeoutThreshold:    30 * time.Second,
			MaxRequestsHalfOpen: 15,
			MinimumRequestCount: 30,
		},
		ChannelConfig: resilience.CircuitBreakerConfig{
			FailureThreshold:    5,
			FailureRatio:        0.6,
			ResetTimeout:        30 * time.Second,
			SuccessThreshold:    2,
			TimeoutThreshold:    5 * time.Second,
			MaxRequestsHalfOpen: 5,
			MinimumRequestCount: 10,
		},
	}
}

// NewAgentCircuitBreaker creates a new agent circuit breaker manager
func NewAgentCircuitBreaker(
	manifestRepo repository.AgentManifestRepository,
	orgRepo repository.OrganizationRepository,
	logger observability.Logger,
	metrics observability.MetricsClient,
	config *AgentCircuitBreakerConfig,
) *AgentCircuitBreaker {
	if config == nil {
		config = DefaultAgentCircuitBreakerConfig()
	}

	return &AgentCircuitBreaker{
		manifestRepo:     manifestRepo,
		orgRepo:          orgRepo,
		logger:           logger,
		metrics:          metrics,
		defaultConfig:    config.DefaultConfig,
		agentConfig:      config.AgentConfig,
		capabilityConfig: config.CapabilityConfig,
		tenantConfig:     config.TenantConfig,
		channelConfig:    config.ChannelConfig,
	}
}

// ExecuteAgentOperation executes an operation with agent-specific circuit breaker
func (acb *AgentCircuitBreaker) ExecuteAgentOperation(ctx context.Context, agentID string, operation func() (interface{}, error)) (interface{}, error) {
	breaker := acb.getOrCreateAgentBreaker(agentID)

	result, err := breaker.Execute(ctx, operation)

	// Record agent-specific metrics
	breakerMetrics := breaker.GetMetrics()
	state := "unknown"
	if s, ok := breakerMetrics["state"].(string); ok {
		state = s
	}

	acb.metrics.RecordCounter("agent_circuit_breaker_calls", 1, map[string]string{
		"agent_id": agentID,
		"state":    state,
		"success":  fmt.Sprintf("%v", err == nil),
	})

	if err != nil {
		// Check if we should mark agent as unhealthy
		if state == "open" {
			acb.markAgentUnhealthy(ctx, agentID)
		}
		return nil, errors.Wrapf(err, "agent %s circuit breaker triggered", agentID)
	}

	return result, nil
}

// ExecuteCapabilityOperation executes an operation with capability-specific circuit breaker
func (acb *AgentCircuitBreaker) ExecuteCapabilityOperation(ctx context.Context, capability string, agentID string, operation func() (interface{}, error)) (interface{}, error) {
	// Use both capability and agent breakers
	capBreaker := acb.getOrCreateCapabilityBreaker(capability)
	agentBreaker := acb.getOrCreateAgentBreaker(agentID)

	// Check both breakers
	capMetrics := capBreaker.GetMetrics()
	capState := "unknown"
	if s, ok := capMetrics["state"].(string); ok {
		capState = s
	}

	agentMetrics := agentBreaker.GetMetrics()
	agentState := "unknown"
	if s, ok := agentMetrics["state"].(string); ok {
		agentState = s
	}

	if capState == "open" {
		acb.metrics.RecordCounter("capability_circuit_breaker_open", 1, map[string]string{
			"capability": capability,
		})
		return nil, fmt.Errorf("capability %s circuit breaker is open", capability)
	}

	if agentState == "open" {
		acb.metrics.RecordCounter("agent_circuit_breaker_open", 1, map[string]string{
			"agent_id": agentID,
		})
		return nil, fmt.Errorf("agent %s circuit breaker is open", agentID)
	}

	// Execute with capability breaker (it will track the agent breaker internally)
	result, err := capBreaker.Execute(ctx, func() (interface{}, error) {
		// Also track in agent breaker
		return agentBreaker.Execute(ctx, operation)
	})

	if err != nil {
		return nil, errors.Wrapf(err, "capability %s operation failed", capability)
	}

	return result, nil
}

// ExecuteTenantOperation executes an operation with tenant-specific circuit breaker
func (acb *AgentCircuitBreaker) ExecuteTenantOperation(ctx context.Context, tenantID uuid.UUID, operation func() (interface{}, error)) (interface{}, error) {
	breaker := acb.getOrCreateTenantBreaker(tenantID)

	result, err := breaker.Execute(ctx, operation)

	if err != nil {
		// Check if we should alert on tenant-wide issues
		metrics := breaker.GetMetrics()
		if state, ok := metrics["state"].(string); ok && state == "open" {
			acb.alertTenantIssue(ctx, tenantID)
		}
		return nil, errors.Wrapf(err, "tenant %s circuit breaker triggered", tenantID)
	}

	return result, nil
}

// ExecuteChannelOperation executes an operation with channel-specific circuit breaker
func (acb *AgentCircuitBreaker) ExecuteChannelOperation(ctx context.Context, channelType string, operation func() (interface{}, error)) (interface{}, error) {
	breaker := acb.getOrCreateChannelBreaker(channelType)

	result, err := breaker.Execute(ctx, operation)

	if err != nil {
		// Check if we should switch to backup channel
		metrics := breaker.GetMetrics()
		if state, ok := metrics["state"].(string); ok && state == "open" {
			acb.handleChannelFailure(ctx, channelType)
		}
		return nil, errors.Wrapf(err, "channel %s circuit breaker triggered", channelType)
	}

	return result, nil
}

// GetAgentBreakerStatus returns the status of an agent's circuit breaker
func (acb *AgentCircuitBreaker) GetAgentBreakerStatus(agentID string) *CircuitBreakerStatus {
	breaker := acb.getOrCreateAgentBreaker(agentID)
	metrics := breaker.GetMetrics()

	state := "unknown"
	if s, ok := metrics["state"].(string); ok {
		state = s
	}

	failures := 0
	if f, ok := metrics["failures"].(int); ok {
		failures = f
	}

	successes := 0
	if s, ok := metrics["successes"].(int); ok {
		successes = s
	}

	return &CircuitBreakerStatus{
		Name:        fmt.Sprintf("agent:%s", agentID),
		State:       state,
		Failures:    failures,
		Successes:   successes,
		LastFailure: time.Now(), // We don't have access to this
		LastChange:  time.Now(), // We don't have access to this
		IsHealthy:   state != "open",
	}
}

// GetCapabilityBreakerStatus returns the status of a capability's circuit breaker
func (acb *AgentCircuitBreaker) GetCapabilityBreakerStatus(capability string) *CircuitBreakerStatus {
	breaker := acb.getOrCreateCapabilityBreaker(capability)
	metrics := breaker.GetMetrics()

	state := "unknown"
	if s, ok := metrics["state"].(string); ok {
		state = s
	}

	failures := 0
	if f, ok := metrics["failures"].(int); ok {
		failures = f
	}

	successes := 0
	if s, ok := metrics["successes"].(int); ok {
		successes = s
	}

	return &CircuitBreakerStatus{
		Name:        fmt.Sprintf("capability:%s", capability),
		State:       state,
		Failures:    failures,
		Successes:   successes,
		LastFailure: time.Now(), // We don't have access to this
		LastChange:  time.Now(), // We don't have access to this
		IsHealthy:   state != "open",
	}
}

// ResetAgentBreaker resets the circuit breaker for an agent
func (acb *AgentCircuitBreaker) ResetAgentBreaker(agentID string) {
	if val, ok := acb.agentBreakers.Load(agentID); ok {
		breaker := val.(*resilience.CircuitBreaker)
		breaker.Reset()

		acb.logger.Info("Reset agent circuit breaker", map[string]interface{}{
			"agent_id": agentID,
		})
	}
}

// ResetCapabilityBreaker resets the circuit breaker for a capability
func (acb *AgentCircuitBreaker) ResetCapabilityBreaker(capability string) {
	if val, ok := acb.capabilityBreakers.Load(capability); ok {
		breaker := val.(*resilience.CircuitBreaker)
		breaker.Reset()

		acb.logger.Info("Reset capability circuit breaker", map[string]interface{}{
			"capability": capability,
		})
	}
}

// Helper methods

func (acb *AgentCircuitBreaker) getOrCreateAgentBreaker(agentID string) *resilience.CircuitBreaker {
	val, _ := acb.agentBreakers.LoadOrStore(agentID,
		resilience.NewCircuitBreaker(
			fmt.Sprintf("agent:%s", agentID),
			acb.agentConfig,
			acb.logger,
			acb.metrics,
		),
	)
	return val.(*resilience.CircuitBreaker)
}

func (acb *AgentCircuitBreaker) getOrCreateCapabilityBreaker(capability string) *resilience.CircuitBreaker {
	val, _ := acb.capabilityBreakers.LoadOrStore(capability,
		resilience.NewCircuitBreaker(
			fmt.Sprintf("capability:%s", capability),
			acb.capabilityConfig,
			acb.logger,
			acb.metrics,
		),
	)
	return val.(*resilience.CircuitBreaker)
}

func (acb *AgentCircuitBreaker) getOrCreateTenantBreaker(tenantID uuid.UUID) *resilience.CircuitBreaker {
	key := tenantID.String()
	val, _ := acb.tenantBreakers.LoadOrStore(key,
		resilience.NewCircuitBreaker(
			fmt.Sprintf("tenant:%s", key),
			acb.tenantConfig,
			acb.logger,
			acb.metrics,
		),
	)
	return val.(*resilience.CircuitBreaker)
}

func (acb *AgentCircuitBreaker) getOrCreateChannelBreaker(channelType string) *resilience.CircuitBreaker {
	val, _ := acb.channelBreakers.LoadOrStore(channelType,
		resilience.NewCircuitBreaker(
			fmt.Sprintf("channel:%s", channelType),
			acb.channelConfig,
			acb.logger,
			acb.metrics,
		),
	)
	return val.(*resilience.CircuitBreaker)
}

func (acb *AgentCircuitBreaker) markAgentUnhealthy(ctx context.Context, agentID string) {
	// Update agent health in manifest repository
	registrations, err := acb.getRegistrationsByAgentID(ctx, agentID)
	if err == nil && len(registrations) > 0 {
		for _, regID := range registrations {
			if err := acb.manifestRepo.UpdateRegistrationHealth(ctx, regID, "unhealthy"); err != nil {
				acb.logger.Error("Failed to mark agent unhealthy", map[string]interface{}{
					"agent_id": agentID,
					"error":    err.Error(),
				})
			}
		}
	}

	acb.metrics.RecordCounter("agents_marked_unhealthy", 1, map[string]string{
		"agent_id": agentID,
	})
}

func (acb *AgentCircuitBreaker) alertTenantIssue(ctx context.Context, tenantID uuid.UUID) {
	// Log and emit metrics for tenant-wide issues
	acb.logger.Error("Tenant circuit breaker open", map[string]interface{}{
		"tenant_id": tenantID,
	})

	acb.metrics.RecordCounter("tenant_circuit_breaker_alerts", 1, map[string]string{
		"tenant_id": tenantID.String(),
	})

	// Could also send alerts to monitoring systems here
}

func (acb *AgentCircuitBreaker) handleChannelFailure(ctx context.Context, channelType string) {
	// Log channel failure
	acb.logger.Error("Channel circuit breaker open", map[string]interface{}{
		"channel_type": channelType,
	})

	acb.metrics.RecordCounter("channel_circuit_breaker_failures", 1, map[string]string{
		"channel_type": channelType,
	})

	// Could implement channel failover logic here
}

func (acb *AgentCircuitBreaker) getRegistrationsByAgentID(ctx context.Context, agentID string) ([]uuid.UUID, error) {
	manifest, err := acb.manifestRepo.GetManifestByAgentID(ctx, agentID)
	if err != nil {
		return nil, err
	}

	registrations, err := acb.manifestRepo.ListRegistrationsByManifest(ctx, manifest.ID)
	if err != nil {
		return nil, err
	}

	// Note: registrations is []models.AgentRegistration, not []uuid.UUID
	// We need to extract the IDs
	var ids []uuid.UUID
	for _, reg := range registrations {
		ids = append(ids, reg.ID)
	}

	return ids, nil
}

// CircuitBreakerStatus contains the status of a circuit breaker
type CircuitBreakerStatus struct {
	Name        string
	State       string
	Failures    int
	Successes   int
	LastFailure time.Time
	LastChange  time.Time
	IsHealthy   bool
}

// CircuitBreakerMetrics contains aggregated metrics for circuit breakers
type CircuitBreakerMetrics struct {
	TotalBreakers     int
	OpenBreakers      int
	HalfOpenBreakers  int
	ClosedBreakers    int
	TotalFailures     int64
	TotalSuccesses    int64
	HealthyPercentage float64
}

// GetMetrics returns aggregated metrics for all circuit breakers
func (acb *AgentCircuitBreaker) GetMetrics() *CircuitBreakerMetrics {
	metrics := &CircuitBreakerMetrics{}

	// Count agent breakers
	acb.agentBreakers.Range(func(key, value interface{}) bool {
		breaker := value.(*resilience.CircuitBreaker)
		metrics.TotalBreakers++

		bMetrics := breaker.GetMetrics()
		if state, ok := bMetrics["state"].(string); ok {
			switch state {
			case "open":
				metrics.OpenBreakers++
			case "half-open":
				metrics.HalfOpenBreakers++
			case "closed":
				metrics.ClosedBreakers++
			}
		}

		if failures, ok := bMetrics["failures"].(int); ok {
			metrics.TotalFailures += int64(failures)
		}
		if successes, ok := bMetrics["successes"].(int); ok {
			metrics.TotalSuccesses += int64(successes)
		}

		return true
	})

	// Count capability breakers
	acb.capabilityBreakers.Range(func(key, value interface{}) bool {
		breaker := value.(*resilience.CircuitBreaker)
		metrics.TotalBreakers++

		bMetrics := breaker.GetMetrics()
		if state, ok := bMetrics["state"].(string); ok {
			switch state {
			case "open":
				metrics.OpenBreakers++
			case "half-open":
				metrics.HalfOpenBreakers++
			case "closed":
				metrics.ClosedBreakers++
			}
		}

		if failures, ok := bMetrics["failures"].(int); ok {
			metrics.TotalFailures += int64(failures)
		}
		if successes, ok := bMetrics["successes"].(int); ok {
			metrics.TotalSuccesses += int64(successes)
		}

		return true
	})

	// Calculate healthy percentage
	if metrics.TotalBreakers > 0 {
		metrics.HealthyPercentage = float64(metrics.ClosedBreakers) / float64(metrics.TotalBreakers) * 100
	}

	return metrics
}
