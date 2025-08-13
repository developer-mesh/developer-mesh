package websocket

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/google/uuid"
)

// AgentRateLimiter extends auth.RateLimiter with agent-specific capabilities
type AgentRateLimiter struct {
	*auth.RateLimiter // Embed base rate limiter to reuse its methods

	// Agent-specific components
	tenantRepo repository.TenantConfigRepository
	orgRepo    repository.OrganizationRepository
	logger     observability.Logger
	metrics    observability.MetricsClient

	// Enhanced tracking for agents
	agentLimits      sync.Map // agent_id -> *AgentRateLimit
	tenantLimits     sync.Map // tenant_id -> *TenantRateLimit
	capabilityLimits sync.Map // capability -> *CapabilityRateLimit

	// Configuration
	defaultAgentRPS      int
	defaultTenantRPS     int
	defaultCapabilityRPS int
	burstMultiplier      float64
}

// AgentRateLimiterConfig extends the base configuration
type AgentRateLimiterConfig struct {
	*auth.RateLimiterConfig // Embed base config

	// Agent-specific settings
	DefaultAgentRPS      int
	DefaultTenantRPS     int
	DefaultCapabilityRPS int
	BurstMultiplier      float64
}

// DefaultAgentRateLimiterConfig returns sensible defaults
func DefaultAgentRateLimiterConfig() *AgentRateLimiterConfig {
	return &AgentRateLimiterConfig{
		RateLimiterConfig:    auth.DefaultRateLimiterConfig(),
		DefaultAgentRPS:      10,
		DefaultTenantRPS:     100,
		DefaultCapabilityRPS: 50,
		BurstMultiplier:      2.0,
	}
}

// NewAgentRateLimiter creates a rate limiter for agents
func NewAgentRateLimiter(
	cache cache.Cache,
	logger observability.Logger,
	metrics observability.MetricsClient,
	tenantRepo repository.TenantConfigRepository,
	orgRepo repository.OrganizationRepository,
	config *AgentRateLimiterConfig,
) *AgentRateLimiter {
	if config == nil {
		config = DefaultAgentRateLimiterConfig()
	}

	// Create base rate limiter
	baseRateLimiter := auth.NewRateLimiter(cache, logger, config.RateLimiterConfig)

	return &AgentRateLimiter{
		RateLimiter:          baseRateLimiter,
		tenantRepo:           tenantRepo,
		orgRepo:              orgRepo,
		logger:               logger,
		metrics:              metrics,
		defaultAgentRPS:      config.DefaultAgentRPS,
		defaultTenantRPS:     config.DefaultTenantRPS,
		defaultCapabilityRPS: config.DefaultCapabilityRPS,
		burstMultiplier:      config.BurstMultiplier,
	}
}

// CheckAgentLimit checks rate limits for a specific agent
func (arl *AgentRateLimiter) CheckAgentLimit(ctx context.Context, agentID string, operation string) error {
	// First check base rate limit (uses identifier)
	if err := arl.RateLimiter.CheckLimit(ctx, fmt.Sprintf("agent:%s:%s", agentID, operation)); err != nil { //nolint:staticcheck // Explicit method call
		arl.metrics.IncrementCounter("agent_rate_limit_exceeded", 1)
		return err
	}

	// Get or create agent-specific limit
	limit := arl.getOrCreateAgentLimit(agentID)

	// Check if within RPS limit
	if !limit.AllowRequest() {
		arl.metrics.IncrementCounter("agent_rps_exceeded", 1)
		return fmt.Errorf("agent rate limit exceeded: %d requests per second", limit.RPS)
	}

	// Check burst limit
	if limit.CurrentBurst >= limit.MaxBurst {
		arl.metrics.IncrementCounter("agent_burst_exceeded", 1)
		return fmt.Errorf("agent burst limit exceeded: max %d", limit.MaxBurst)
	}

	return nil
}

// CheckTenantLimit checks rate limits for a tenant
func (arl *AgentRateLimiter) CheckTenantLimit(ctx context.Context, tenantID uuid.UUID, operation string) error {
	// Get tenant configuration
	tenantConfig, err := arl.tenantRepo.GetByTenantID(ctx, tenantID.String())
	if err != nil {
		// Use defaults if config not found
		return arl.checkDefaultTenantLimit(ctx, tenantID, operation)
	}

	// Check configured limits
	rateLimitConfig, hasOverride := tenantConfig.GetRateLimitForEndpoint(operation)
	if hasOverride {
		return arl.checkConfiguredLimit(ctx, tenantID.String(), operation, rateLimitConfig.RequestsPerMinute)
	}

	// Fall back to default tenant limits
	return arl.checkDefaultTenantLimit(ctx, tenantID, operation)
}

// CheckCapabilityLimit checks rate limits for capability usage
func (arl *AgentRateLimiter) CheckCapabilityLimit(ctx context.Context, capability string, agentID string) error {
	// Create composite key for capability + agent
	key := fmt.Sprintf("capability:%s:agent:%s", capability, agentID)

	// Use base rate limiter for initial check
	if err := arl.RateLimiter.CheckLimit(ctx, key); err != nil { //nolint:staticcheck // Explicit method call
		arl.metrics.IncrementCounter("capability_rate_limit_exceeded", 1)
		return err
	}

	// Get or create capability-specific limit
	limit := arl.getOrCreateCapabilityLimit(capability)

	// Check if within RPS limit
	if !limit.AllowRequest() {
		arl.metrics.IncrementCounter("capability_rps_exceeded", 1)
		return fmt.Errorf("capability rate limit exceeded for %s: %d requests per second", capability, limit.RPS)
	}

	// Track capability usage per agent
	limit.TrackAgent(agentID)

	// Check if too many agents using this capability
	if limit.GetActiveAgentCount() > limit.MaxConcurrentAgents {
		arl.metrics.IncrementCounter("capability_agent_limit_exceeded", 1)
		return fmt.Errorf("too many agents using capability %s: max %d", capability, limit.MaxConcurrentAgents)
	}

	return nil
}

// CheckOrganizationLimit checks rate limits at organization level
func (arl *AgentRateLimiter) CheckOrganizationLimit(ctx context.Context, orgID uuid.UUID) error {
	// Get organization
	org, err := arl.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("organization not found: %w", err)
	}

	// Check isolation mode
	if org.IsStrictlyIsolated() {
		// Strict isolation may have different limits
		return arl.checkStrictIsolationLimit(ctx, orgID)
	}

	// Standard organization limit check
	key := fmt.Sprintf("org:%s", orgID)
	return arl.RateLimiter.CheckLimit(ctx, key) //nolint:staticcheck // Explicit method call
}

// RecordAgentRequest records a request for rate limiting
func (arl *AgentRateLimiter) RecordAgentRequest(ctx context.Context, agentID string, operation string, success bool) {
	// Record in base rate limiter
	arl.RateLimiter.RecordAttempt(ctx, fmt.Sprintf("agent:%s:%s", agentID, operation), success) //nolint:staticcheck // Explicit method call

	// Update agent-specific tracking
	if limit, ok := arl.agentLimits.Load(agentID); ok {
		agentLimit := limit.(*AgentRateLimit)
		agentLimit.RecordRequest(success)

		// Update metrics
		arl.metrics.RecordGauge("agent_request_rate", float64(agentLimit.GetCurrentRPS()), map[string]string{
			"agent_id":  agentID,
			"operation": operation,
		})
	}
}

// GetAgentLimitStatus returns current limit status for an agent
func (arl *AgentRateLimiter) GetAgentLimitStatus(agentID string) *AgentLimitStatus {
	limit, ok := arl.agentLimits.Load(agentID)
	if !ok {
		return &AgentLimitStatus{
			AgentID:        agentID,
			CurrentRPS:     0,
			MaxRPS:         arl.defaultAgentRPS,
			BurstRemaining: int(float64(arl.defaultAgentRPS) * arl.burstMultiplier),
		}
	}

	agentLimit := limit.(*AgentRateLimit)
	return &AgentLimitStatus{
		AgentID:        agentID,
		CurrentRPS:     agentLimit.GetCurrentRPS(),
		MaxRPS:         agentLimit.RPS,
		BurstRemaining: agentLimit.MaxBurst - agentLimit.CurrentBurst,
		WindowResetAt:  agentLimit.WindowEnd,
	}
}

// Helper methods

func (arl *AgentRateLimiter) getOrCreateAgentLimit(agentID string) *AgentRateLimit {
	val, _ := arl.agentLimits.LoadOrStore(agentID, &AgentRateLimit{
		AgentID:     agentID,
		RPS:         arl.defaultAgentRPS,
		MaxBurst:    int(float64(arl.defaultAgentRPS) * arl.burstMultiplier),
		WindowStart: time.Now(),
		WindowEnd:   time.Now().Add(time.Second),
	})
	return val.(*AgentRateLimit)
}

func (arl *AgentRateLimiter) getOrCreateCapabilityLimit(capability string) *CapabilityRateLimit {
	val, _ := arl.capabilityLimits.LoadOrStore(capability, &CapabilityRateLimit{
		Capability:          capability,
		RPS:                 arl.defaultCapabilityRPS,
		MaxConcurrentAgents: 10, // Default max concurrent agents
		ActiveAgents:        make(map[string]time.Time),
		mu:                  sync.RWMutex{},
		WindowStart:         time.Now(),
		WindowEnd:           time.Now().Add(time.Second),
	})
	return val.(*CapabilityRateLimit)
}

func (arl *AgentRateLimiter) checkDefaultTenantLimit(ctx context.Context, tenantID uuid.UUID, operation string) error {
	key := fmt.Sprintf("tenant:%s:%s", tenantID, operation)

	// Get or create tenant limit
	val, _ := arl.tenantLimits.LoadOrStore(tenantID.String(), &TenantRateLimit{
		TenantID:    tenantID,
		RPS:         arl.defaultTenantRPS,
		WindowStart: time.Now(),
		WindowEnd:   time.Now().Add(time.Second),
	})

	limit := val.(*TenantRateLimit)
	if !limit.AllowRequest() {
		return fmt.Errorf("tenant rate limit exceeded: %d requests per second", limit.RPS)
	}

	return arl.RateLimiter.CheckLimit(ctx, key) //nolint:staticcheck // Explicit method call
}

func (arl *AgentRateLimiter) checkConfiguredLimit(ctx context.Context, tenantID, operation string, rpm int) error {
	// Convert RPM to RPS (currently unused, but kept for future implementation)
	// TODO: Implement actual rate limiting based on RPS
	// rps := rpm / 60
	// if rps < 1 {
	// 	rps = 1
	// }

	key := fmt.Sprintf("tenant:%s:%s", tenantID, operation)

	// Check against configured limit
	// This is simplified - in production you'd want sliding windows
	return arl.RateLimiter.CheckLimit(ctx, key) //nolint:staticcheck // Explicit method call
}

func (arl *AgentRateLimiter) checkStrictIsolationLimit(ctx context.Context, orgID uuid.UUID) error {
	// Strict isolation has tighter limits
	key := fmt.Sprintf("org:strict:%s", orgID)
	return arl.RateLimiter.CheckLimit(ctx, key) //nolint:staticcheck // Explicit method call
}

// Rate limit structures

// AgentRateLimit tracks rate limits for an individual agent
type AgentRateLimit struct {
	AgentID      string
	RPS          int
	CurrentCount int
	CurrentBurst int
	MaxBurst     int
	WindowStart  time.Time
	WindowEnd    time.Time
	mu           sync.Mutex
}

func (arl *AgentRateLimit) AllowRequest() bool {
	arl.mu.Lock()
	defer arl.mu.Unlock()

	now := time.Now()
	if now.After(arl.WindowEnd) {
		// Reset window
		arl.CurrentCount = 0
		arl.CurrentBurst = 0
		arl.WindowStart = now
		arl.WindowEnd = now.Add(time.Second)
	}

	if arl.CurrentCount >= arl.RPS {
		// Try burst
		if arl.CurrentBurst < arl.MaxBurst {
			arl.CurrentBurst++
			return true
		}
		return false
	}

	arl.CurrentCount++
	return true
}

func (arl *AgentRateLimit) RecordRequest(success bool) {
	arl.mu.Lock()
	defer arl.mu.Unlock()

	if !success && arl.CurrentBurst > 0 {
		// Reduce burst on failure
		arl.CurrentBurst--
	}
}

func (arl *AgentRateLimit) GetCurrentRPS() int {
	arl.mu.Lock()
	defer arl.mu.Unlock()
	return arl.CurrentCount
}

// TenantRateLimit tracks rate limits for a tenant
type TenantRateLimit struct {
	TenantID     uuid.UUID
	RPS          int
	CurrentCount int
	WindowStart  time.Time
	WindowEnd    time.Time
	mu           sync.Mutex
}

func (trl *TenantRateLimit) AllowRequest() bool {
	trl.mu.Lock()
	defer trl.mu.Unlock()

	now := time.Now()
	if now.After(trl.WindowEnd) {
		// Reset window
		trl.CurrentCount = 0
		trl.WindowStart = now
		trl.WindowEnd = now.Add(time.Second)
	}

	if trl.CurrentCount >= trl.RPS {
		return false
	}

	trl.CurrentCount++
	return true
}

// CapabilityRateLimit tracks rate limits for capability usage
type CapabilityRateLimit struct {
	Capability          string
	RPS                 int
	CurrentCount        int
	MaxConcurrentAgents int
	ActiveAgents        map[string]time.Time
	WindowStart         time.Time
	WindowEnd           time.Time
	mu                  sync.RWMutex
}

func (crl *CapabilityRateLimit) AllowRequest() bool {
	crl.mu.Lock()
	defer crl.mu.Unlock()

	now := time.Now()
	if now.After(crl.WindowEnd) {
		// Reset window
		crl.CurrentCount = 0
		crl.WindowStart = now
		crl.WindowEnd = now.Add(time.Second)

		// Clean up expired agents
		for agentID, lastSeen := range crl.ActiveAgents {
			if now.Sub(lastSeen) > time.Minute {
				delete(crl.ActiveAgents, agentID)
			}
		}
	}

	if crl.CurrentCount >= crl.RPS {
		return false
	}

	crl.CurrentCount++
	return true
}

func (crl *CapabilityRateLimit) TrackAgent(agentID string) {
	crl.mu.Lock()
	defer crl.mu.Unlock()
	crl.ActiveAgents[agentID] = time.Now()
}

func (crl *CapabilityRateLimit) GetActiveAgentCount() int {
	crl.mu.RLock()
	defer crl.mu.RUnlock()
	return len(crl.ActiveAgents)
}

// AgentLimitStatus contains current limit status
type AgentLimitStatus struct {
	AgentID        string
	CurrentRPS     int
	MaxRPS         int
	BurstRemaining int
	WindowResetAt  time.Time
}
