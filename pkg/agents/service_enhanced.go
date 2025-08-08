package agents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// EnhancedService provides comprehensive agent lifecycle management
type EnhancedService struct {
	repo          *EnhancedRepository
	configRepo    Repository // For agent configs
	validator     *validator.Validate
	cache         *sync.Map
	eventBus      EventPublisher
	healthMonitor *HealthMonitor
}

// HealthMonitor monitors agent health in the background
type HealthMonitor struct {
	service  *EnhancedService
	interval time.Duration
	done     chan struct{}
	wg       sync.WaitGroup
}

// NewEnhancedService creates a new enhanced agent service
func NewEnhancedService(repo *EnhancedRepository, configRepo Repository, eventBus EventPublisher) *EnhancedService {
	s := &EnhancedService{
		repo:       repo,
		configRepo: configRepo,
		validator:  validator.New(),
		cache:      &sync.Map{},
		eventBus:   eventBus,
	}

	// Start health monitor
	s.healthMonitor = &HealthMonitor{
		service:  s,
		interval: 30 * time.Second,
		done:     make(chan struct{}),
	}
	s.healthMonitor.Start()

	return s
}

// RegisterAgent registers a new agent with automatic configuration
func (s *EnhancedService) RegisterAgent(ctx context.Context, req RegisterAgentRequest) (*Agent, error) {
	// Validate request
	if err := s.validator.Struct(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Generate deterministic ID if identifier provided
	var agentID uuid.UUID
	if req.Identifier != "" {
		agentID = s.generateDeterministicID(req.Type, req.TenantID, req.Identifier)

		// Check if agent already exists (idempotent)
		existing, err := s.repo.GetAgent(ctx, agentID)
		if err == nil && existing != nil {
			// Agent exists, handle re-registration
			return s.handleReregistration(ctx, existing, req)
		}
	} else {
		agentID = uuid.New()
	}

	// Create agent
	agent := &Agent{
		ID:            agentID,
		TenantID:      req.TenantID,
		Name:          req.Name,
		Type:          req.Type,
		State:         StatePending,
		StateReason:   "Initial registration",
		ModelID:       req.ModelID,
		Capabilities:  req.Capabilities,
		Configuration: req.Config,
		SystemPrompt:  req.SystemPrompt,
		Temperature:   req.Temperature,
		MaxTokens:     req.MaxTokens,
		Metadata:      req.Metadata,
		HealthStatus:  map[string]interface{}{"status": "initializing"},
	}

	// Set defaults
	if agent.Temperature == 0 {
		agent.Temperature = 0.7
	}
	if agent.MaxTokens == 0 {
		agent.MaxTokens = 4096
	}
	if agent.Configuration == nil {
		agent.Configuration = make(map[string]interface{})
	}
	if agent.Metadata == nil {
		agent.Metadata = make(map[string]interface{})
	}

	// Create agent in database
	if err := s.repo.CreateAgent(ctx, agent); err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// The database trigger will automatically create default configuration
	// No need to explicitly create it here to avoid duplicate key errors

	// Start async configuration process
	go s.configureAgent(context.Background(), agentID)

	// Publish event
	if s.eventBus != nil {
		event := ConfigUpdatedEvent{
			AgentID:   agentID.String(),
			Timestamp: time.Now(),
		}
		_ = s.eventBus.PublishConfigUpdated(ctx, event)
	}

	// Cache the agent
	s.cache.Store(agentID, agent)

	return agent, nil
}

// GetAgent retrieves an agent by ID
func (s *EnhancedService) GetAgent(ctx context.Context, id uuid.UUID) (*Agent, error) {
	// Check cache first
	if cached, ok := s.cache.Load(id); ok {
		return cached.(*Agent), nil
	}

	// Get from repository
	agent, err := s.repo.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}

	// Update cache
	s.cache.Store(id, agent)

	return agent, nil
}

// UpdateAgent updates an agent's configuration
func (s *EnhancedService) UpdateAgent(ctx context.Context, id uuid.UUID, update UpdateAgentRequest) (*Agent, error) {
	// Validate update
	if err := s.validator.Struct(update); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Update in repository
	agent, err := s.repo.UpdateAgent(ctx, id, &update)
	if err != nil {
		return nil, err
	}

	// Update cache
	s.cache.Store(id, agent)

	// Publish event
	if s.eventBus != nil {
		event := ConfigUpdatedEvent{
			AgentID:   id.String(),
			Timestamp: time.Now(),
		}
		_ = s.eventBus.PublishConfigUpdated(ctx, event)
	}

	return agent, nil
}

// TransitionState transitions an agent to a new state
func (s *EnhancedService) TransitionState(ctx context.Context, id uuid.UUID, targetState AgentState, reason string, initiatedBy uuid.UUID) error {
	// Get current agent
	agent, err := s.GetAgent(ctx, id)
	if err != nil {
		return err
	}

	// Validate transition
	if !agent.CanTransitionTo(targetState) {
		return fmt.Errorf("cannot transition from %s to %s", agent.State, targetState)
	}

	// Execute pre-transition hooks
	if err := s.executePreTransitionHooks(ctx, agent, targetState); err != nil {
		return fmt.Errorf("pre-transition hook failed: %w", err)
	}

	// Perform transition
	req := &StateTransitionRequest{
		TargetState: targetState,
		Reason:      reason,
		InitiatedBy: initiatedBy,
	}

	if err := s.repo.TransitionState(ctx, id, req); err != nil {
		return err
	}

	// Update cache
	agent.State = targetState
	agent.StateReason = reason
	agent.StateChangedAt = time.Now()
	agent.StateChangedBy = &initiatedBy
	s.cache.Store(id, agent)

	// Execute post-transition hooks
	go s.executePostTransitionHooks(context.Background(), agent, targetState)

	return nil
}

// ActivateAgent activates an agent
func (s *EnhancedService) ActivateAgent(ctx context.Context, id uuid.UUID, initiatedBy uuid.UUID) error {
	agent, err := s.GetAgent(ctx, id)
	if err != nil {
		return err
	}

	// Validate agent is ready
	if agent.State != StateReady {
		// Try to validate first
		if agent.State == StateValidating {
			if err := s.validateAgent(ctx, agent); err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}
		} else {
			return fmt.Errorf("agent must be in ready state, current: %s", agent.State)
		}
	}

	// Transition to active
	return s.TransitionState(ctx, id, StateActive, "Agent activated", initiatedBy)
}

// SuspendAgent suspends an agent
func (s *EnhancedService) SuspendAgent(ctx context.Context, id uuid.UUID, reason string, initiatedBy uuid.UUID) error {
	return s.TransitionState(ctx, id, StateSuspended, reason, initiatedBy)
}

// TerminateAgent terminates an agent
func (s *EnhancedService) TerminateAgent(ctx context.Context, id uuid.UUID, reason string, initiatedBy uuid.UUID) error {
	// First transition to terminating
	if err := s.TransitionState(ctx, id, StateTerminating, reason, initiatedBy); err != nil {
		return err
	}

	// Perform cleanup
	go s.cleanupAgent(context.Background(), id, initiatedBy)

	return nil
}

// ListAgents lists agents with filtering
func (s *EnhancedService) ListAgents(ctx context.Context, filter AgentFilter) ([]*Agent, error) {
	return s.repo.ListAgents(ctx, filter)
}

// GetAgentEvents retrieves events for an agent
func (s *EnhancedService) GetAgentEvents(ctx context.Context, agentID uuid.UUID, limit int) ([]*AgentEvent, error) {
	filter := EventFilter{
		AgentID: &agentID,
		Limit:   limit,
	}
	return s.repo.GetEvents(ctx, filter)
}

// UpdateAgentHealth updates an agent's health status
func (s *EnhancedService) UpdateAgentHealth(ctx context.Context, id uuid.UUID, health map[string]interface{}) error {
	if err := s.repo.UpdateHealth(ctx, id, health); err != nil {
		return err
	}

	// Check if agent should transition to degraded
	if status, ok := health["status"].(string); ok && status == "unhealthy" {
		agent, err := s.GetAgent(ctx, id)
		if err == nil && agent.State == StateActive {
			_ = s.TransitionState(ctx, id, StateDegraded, "Health check failed", uuid.Nil)
		}
	}

	return nil
}

// Helper methods

func (s *EnhancedService) generateDeterministicID(agentType string, tenantID uuid.UUID, identifier string) uuid.UUID {
	// Create deterministic ID from type, tenant, and identifier
	data := fmt.Sprintf("%s-%s-%s", agentType, tenantID.String(), identifier)
	hash := sha256.Sum256([]byte(data))
	hashStr := hex.EncodeToString(hash[:])

	// Format as UUID
	uuidStr := fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		hashStr[0:8],
		hashStr[8:12],
		hashStr[12:16],
		hashStr[16:20],
		hashStr[20:32],
	)

	id, _ := uuid.Parse(uuidStr)
	return id
}

func (s *EnhancedService) handleReregistration(ctx context.Context, existing *Agent, req RegisterAgentRequest) (*Agent, error) {
	// Update last seen
	now := time.Now()
	existing.LastSeenAt = &now

	// If agent is terminated, reject
	if existing.State == StateTerminated {
		return nil, fmt.Errorf("agent is terminated and cannot be reregistered")
	}

	// If agent is in a bad state, try to recover
	if existing.State == StateTerminating {
		// Cancel termination
		_ = s.TransitionState(ctx, existing.ID, StateSuspended, "Reregistration cancelled termination", uuid.Nil)
	}

	// Update configuration if provided
	if req.Config != nil {
		existing.Configuration = req.Config
	}
	if req.Capabilities != nil {
		existing.Capabilities = req.Capabilities
	}

	// Save updates
	update := UpdateAgentRequest{
		Config:       &existing.Configuration,
		Capabilities: &existing.Capabilities,
		Metadata:     &existing.Metadata,
	}

	return s.UpdateAgent(ctx, existing.ID, update)
}

func (s *EnhancedService) configureAgent(ctx context.Context, agentID uuid.UUID) {
	// Simulate configuration process
	time.Sleep(1 * time.Second)

	// Transition to validating
	_ = s.TransitionState(ctx, agentID, StateValidating, "Configuration complete", uuid.Nil)

	// Validate
	agent, err := s.GetAgent(ctx, agentID)
	if err == nil {
		if err := s.validateAgent(ctx, agent); err == nil {
			// Transition to ready
			_ = s.TransitionState(ctx, agentID, StateReady, "Validation successful", uuid.Nil)
		}
	}
}

func (s *EnhancedService) validateAgent(ctx context.Context, agent *Agent) error {
	// Validate agent configuration
	if agent.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	if agent.Type == "" {
		return fmt.Errorf("agent type is required")
	}
	if len(agent.Capabilities) == 0 {
		return fmt.Errorf("agent must have at least one capability")
	}

	// Check if config exists
	config, err := s.configRepo.GetConfig(ctx, agent.ID.String())
	if err != nil {
		return fmt.Errorf("agent configuration not found")
	}
	if config == nil || !config.IsActive {
		return fmt.Errorf("agent configuration is not active")
	}

	return nil
}

func (s *EnhancedService) cleanupAgent(ctx context.Context, agentID uuid.UUID, initiatedBy uuid.UUID) {
	// Perform cleanup tasks
	time.Sleep(2 * time.Second)

	// Remove from cache
	s.cache.Delete(agentID)

	// Transition to terminated
	_ = s.TransitionState(ctx, agentID, StateTerminated, "Cleanup complete", initiatedBy)
}

func (s *EnhancedService) executePreTransitionHooks(ctx context.Context, agent *Agent, targetState AgentState) error {
	// Execute any pre-transition validation or preparation
	switch targetState {
	case StateActive:
		// Ensure agent has valid configuration
		return s.validateAgent(ctx, agent)
	case StateTerminating:
		// Check if agent has active sessions
		// Would check sessions here
		return nil
	}
	return nil
}

func (s *EnhancedService) executePostTransitionHooks(ctx context.Context, agent *Agent, newState AgentState) {
	// Execute any post-transition actions
	switch newState {
	case StateActive:
		// Start health monitoring
		s.startHealthMonitoring(ctx, agent.ID)
	case StateTerminated:
		// Cleanup resources
		s.cache.Delete(agent.ID)
	}
}

func (s *EnhancedService) startHealthMonitoring(ctx context.Context, agentID uuid.UUID) {
	// Would implement health monitoring here
}

// Shutdown gracefully shuts down the service
func (s *EnhancedService) Shutdown() {
	if s.healthMonitor != nil {
		close(s.healthMonitor.done)
		s.healthMonitor.wg.Wait()
	}
}

// HealthMonitor implementation

func (h *HealthMonitor) Start() {
	h.wg.Add(1)
	go h.monitor()
}

func (h *HealthMonitor) monitor() {
	defer h.wg.Done()

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.checkAllAgents()
		case <-h.done:
			return
		}
	}
}

func (h *HealthMonitor) checkAllAgents() {
	// Get all active agents
	filter := AgentFilter{
		States: []AgentState{StateActive, StateDegraded},
	}

	agents, err := h.service.ListAgents(context.Background(), filter)
	if err != nil {
		return
	}

	for _, agent := range agents {
		// Check agent health
		health := map[string]interface{}{
			"status":     "healthy",
			"checked_at": time.Now(),
		}

		// Would perform actual health checks here

		_ = h.service.UpdateAgentHealth(context.Background(), agent.ID, health)
	}
}
