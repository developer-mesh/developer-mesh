package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/metrics"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"golang.org/x/time/rate"
)

// RateLimitConfig defines rate limiting configuration for Edge MCP
type RateLimitConfig struct {
	// Global limits (all requests)
	GlobalRPS   int // Requests per second globally
	GlobalBurst int // Burst size globally

	// Per-tenant limits
	TenantRPS   int // Requests per second per tenant
	TenantBurst int // Burst size per tenant

	// Per-tool limits
	ToolRPS   int // Requests per second per tool
	ToolBurst int // Burst size per tool

	// Quota management
	EnableQuotas       bool          // Enable quota tracking
	QuotaResetInterval time.Duration // How often quotas reset (e.g., daily, monthly)
	DefaultQuota       int64         // Default quota per tenant

	// Cleanup
	CleanupInterval time.Duration // How often to clean up old limiters
	MaxAge          time.Duration // Maximum age for unused limiters
}

// DefaultRateLimitConfig returns default rate limit configuration for Edge MCP
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		GlobalRPS:          1000,
		GlobalBurst:        2000,
		TenantRPS:          100,
		TenantBurst:        200,
		ToolRPS:            50,
		ToolBurst:          100,
		EnableQuotas:       true,
		QuotaResetInterval: 24 * time.Hour, // Daily reset
		DefaultQuota:       10000,          // 10K requests per day
		CleanupInterval:    5 * time.Minute,
		MaxAge:             1 * time.Hour,
	}
}

// rateLimiterEntry holds a rate limiter and its metadata
type rateLimiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// quotaEntry tracks quota usage for a tenant
type quotaEntry struct {
	used    int64
	limit   int64
	resetAt time.Time
	mu      sync.Mutex
}

// RateLimiter manages rate limiting for Edge MCP WebSocket connections
type RateLimiter struct {
	config    RateLimitConfig
	limiters  map[string]*rateLimiterEntry
	quotas    map[string]*quotaEntry
	mu        sync.RWMutex
	logger    observability.Logger
	metrics   *metrics.Metrics
	globalRL  *rate.Limiter
	stopClean chan struct{}
	closed    bool
	closeMu   sync.Mutex
	wg        sync.WaitGroup
}

// RateLimitResult contains the result of a rate limit check
type RateLimitResult struct {
	Allowed      bool
	LimitType    string // "global", "tenant", "tool", "quota"
	Limit        int
	Remaining    int
	ResetAt      time.Time
	RetryAfter   time.Duration
	QuotaUsed    int64
	QuotaLimit   int64
	QuotaResetAt time.Time
}

// NewRateLimiter creates a new rate limiter for Edge MCP
func NewRateLimiter(config RateLimitConfig, logger observability.Logger, metricsCollector *metrics.Metrics) *RateLimiter {
	rl := &RateLimiter{
		config:    config,
		limiters:  make(map[string]*rateLimiterEntry),
		quotas:    make(map[string]*quotaEntry),
		logger:    logger,
		metrics:   metricsCollector,
		globalRL:  rate.NewLimiter(rate.Limit(config.GlobalRPS), config.GlobalBurst),
		stopClean: make(chan struct{}),
	}

	// Start cleanup routine
	rl.wg.Add(1)
	go rl.cleanupRoutine()

	return rl
}

// CheckRateLimit checks if a request should be allowed based on rate limits
// Returns a RateLimitResult with details about the decision
func (rl *RateLimiter) CheckRateLimit(ctx context.Context, tenantID, toolName string) *RateLimitResult {
	// Check global rate limit first
	if !rl.globalRL.Allow() {
		rl.recordRateLimitHit("global", toolName)
		return &RateLimitResult{
			Allowed:    false,
			LimitType:  "global",
			Limit:      rl.config.GlobalRPS,
			Remaining:  0,
			ResetAt:    time.Now().Add(time.Second),
			RetryAfter: time.Second,
		}
	}

	// Check tenant quota if enabled
	if rl.config.EnableQuotas && tenantID != "" {
		quotaResult := rl.checkQuota(tenantID)
		if !quotaResult.Allowed {
			rl.recordRateLimitHit("quota", toolName)
			return quotaResult
		}
	}

	// Check per-tenant rate limit
	if tenantID != "" {
		tenantKey := fmt.Sprintf("tenant:%s", tenantID)
		tenantLimiter := rl.getLimiter(tenantKey, rl.config.TenantRPS, rl.config.TenantBurst)

		if !tenantLimiter.Allow() {
			rl.recordRateLimitHit("tenant", toolName)
			remaining := int(tenantLimiter.Tokens())
			return &RateLimitResult{
				Allowed:    false,
				LimitType:  "tenant",
				Limit:      rl.config.TenantRPS,
				Remaining:  remaining,
				ResetAt:    time.Now().Add(time.Second),
				RetryAfter: time.Second,
			}
		}
	}

	// Check per-tool rate limit
	if toolName != "" && tenantID != "" {
		toolKey := fmt.Sprintf("tool:%s:%s", tenantID, toolName)
		toolLimiter := rl.getLimiter(toolKey, rl.config.ToolRPS, rl.config.ToolBurst)

		if !toolLimiter.Allow() {
			rl.recordRateLimitHit("tool", toolName)
			remaining := int(toolLimiter.Tokens())
			return &RateLimitResult{
				Allowed:    false,
				LimitType:  "tool",
				Limit:      rl.config.ToolRPS,
				Remaining:  remaining,
				ResetAt:    time.Now().Add(time.Second),
				RetryAfter: time.Second,
			}
		}
	}

	// All checks passed
	return &RateLimitResult{
		Allowed:   true,
		LimitType: "none",
	}
}

// checkQuota checks if a tenant has quota remaining
func (rl *RateLimiter) checkQuota(tenantID string) *RateLimitResult {
	rl.mu.Lock()
	quota, exists := rl.quotas[tenantID]
	if !exists {
		// Create new quota entry
		quota = &quotaEntry{
			used:    0,
			limit:   rl.config.DefaultQuota,
			resetAt: time.Now().Add(rl.config.QuotaResetInterval),
		}
		rl.quotas[tenantID] = quota
	}
	rl.mu.Unlock()

	quota.mu.Lock()
	defer quota.mu.Unlock()

	// Check if quota needs to be reset
	if time.Now().After(quota.resetAt) {
		quota.used = 0
		quota.resetAt = time.Now().Add(rl.config.QuotaResetInterval)
	}

	// Check if quota exceeded
	if quota.used >= quota.limit {
		return &RateLimitResult{
			Allowed:      false,
			LimitType:    "quota",
			QuotaUsed:    quota.used,
			QuotaLimit:   quota.limit,
			QuotaResetAt: quota.resetAt,
			RetryAfter:   time.Until(quota.resetAt),
		}
	}

	// Increment quota usage
	quota.used++

	return &RateLimitResult{
		Allowed:      true,
		LimitType:    "quota",
		QuotaUsed:    quota.used,
		QuotaLimit:   quota.limit,
		QuotaResetAt: quota.resetAt,
	}
}

// getLimiter gets or creates a rate limiter for a key
func (rl *RateLimiter) getLimiter(key string, rps, burst int) *rate.Limiter {
	rl.mu.RLock()
	entry, exists := rl.limiters[key]
	rl.mu.RUnlock()

	if exists {
		// Update last access
		rl.mu.Lock()
		entry.lastAccess = time.Now()
		rl.mu.Unlock()
		return entry.limiter
	}

	// Create new limiter
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if entry, exists := rl.limiters[key]; exists {
		entry.lastAccess = time.Now()
		return entry.limiter
	}

	limiter := rate.NewLimiter(rate.Limit(rps), burst)
	rl.limiters[key] = &rateLimiterEntry{
		limiter:    limiter,
		lastAccess: time.Now(),
	}

	return limiter
}

// cleanupRoutine periodically cleans up old limiters
func (rl *RateLimiter) cleanupRoutine() {
	defer rl.wg.Done()

	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopClean:
			return
		}
	}
}

// cleanup removes old limiters and resets expired quotas
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Cleanup old rate limiters
	for key, entry := range rl.limiters {
		if now.Sub(entry.lastAccess) > rl.config.MaxAge {
			delete(rl.limiters, key)
		}
	}

	// Cleanup expired quotas
	for key, quota := range rl.quotas {
		quota.mu.Lock()
		if now.After(quota.resetAt) {
			quota.used = 0
			quota.resetAt = now.Add(rl.config.QuotaResetInterval)
		}
		quota.mu.Unlock()

		// Remove if unused for long time
		if quota.used == 0 && now.Sub(quota.resetAt.Add(-rl.config.QuotaResetInterval)) > rl.config.MaxAge {
			delete(rl.quotas, key)
		}
	}

	rl.logger.Debug("Rate limiter cleanup completed", map[string]interface{}{
		"remaining_limiters": len(rl.limiters),
		"remaining_quotas":   len(rl.quotas),
	})
}

// recordRateLimitHit records a rate limit hit metric
func (rl *RateLimiter) recordRateLimitHit(limitType, toolName string) {
	if rl.metrics != nil {
		rl.metrics.RecordError("rate_limit", limitType)
	}

	rl.logger.Warn("Rate limit exceeded", map[string]interface{}{
		"limit_type": limitType,
		"tool_name":  toolName,
	})
}

// GetStats returns current rate limiter statistics
func (rl *RateLimiter) GetStats() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	tenantCount := 0
	toolCount := 0
	totalQuotaUsed := int64(0)

	for key := range rl.limiters {
		if len(key) > 7 && key[:7] == "tenant:" {
			tenantCount++
		} else if len(key) > 5 && key[:5] == "tool:" {
			toolCount++
		}
	}

	for _, quota := range rl.quotas {
		quota.mu.Lock()
		totalQuotaUsed += quota.used
		quota.mu.Unlock()
	}

	return map[string]interface{}{
		"total_limiters":   len(rl.limiters),
		"tenant_limiters":  tenantCount,
		"tool_limiters":    toolCount,
		"total_quotas":     len(rl.quotas),
		"total_quota_used": totalQuotaUsed,
		"config":           rl.config,
	}
}

// SetTenantQuota sets a custom quota for a specific tenant
func (rl *RateLimiter) SetTenantQuota(tenantID string, limit int64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	quota, exists := rl.quotas[tenantID]
	if !exists {
		quota = &quotaEntry{
			used:    0,
			limit:   limit,
			resetAt: time.Now().Add(rl.config.QuotaResetInterval),
		}
		rl.quotas[tenantID] = quota
	} else {
		quota.mu.Lock()
		quota.limit = limit
		quota.mu.Unlock()
	}

	rl.logger.Info("Tenant quota updated", map[string]interface{}{
		"tenant_id": tenantID,
		"limit":     limit,
	})
}

// GetTenantQuota returns the current quota usage for a tenant
func (rl *RateLimiter) GetTenantQuota(tenantID string) (used, limit int64, resetAt time.Time) {
	rl.mu.RLock()
	quota, exists := rl.quotas[tenantID]
	rl.mu.RUnlock()

	if !exists {
		return 0, rl.config.DefaultQuota, time.Now().Add(rl.config.QuotaResetInterval)
	}

	quota.mu.Lock()
	defer quota.mu.Unlock()

	return quota.used, quota.limit, quota.resetAt
}

// GetRateLimitMetadata returns rate limit metadata for including in JSON-RPC responses
// This provides clients with visibility into their rate limit status
func (rl *RateLimiter) GetRateLimitMetadata(tenantID, toolName string) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Add global rate limit info
	globalTokens := int(rl.globalRL.Tokens())
	metadata["global_limit"] = rl.config.GlobalRPS
	metadata["global_remaining"] = globalTokens

	// Add tenant rate limit info
	if tenantID != "" {
		tenantKey := fmt.Sprintf("tenant:%s", tenantID)
		rl.mu.RLock()
		entry, exists := rl.limiters[tenantKey]
		rl.mu.RUnlock()

		if exists {
			tenantRemaining := int(entry.limiter.Tokens())
			metadata["tenant_limit"] = rl.config.TenantRPS
			metadata["tenant_remaining"] = tenantRemaining
		} else {
			metadata["tenant_limit"] = rl.config.TenantRPS
			metadata["tenant_remaining"] = rl.config.TenantRPS
		}
	}

	// Add tool rate limit info
	if toolName != "" && tenantID != "" {
		toolKey := fmt.Sprintf("tool:%s:%s", tenantID, toolName)
		rl.mu.RLock()
		entry, exists := rl.limiters[toolKey]
		rl.mu.RUnlock()

		if exists {
			toolRemaining := int(entry.limiter.Tokens())
			metadata["tool_limit"] = rl.config.ToolRPS
			metadata["tool_remaining"] = toolRemaining
		} else {
			metadata["tool_limit"] = rl.config.ToolRPS
			metadata["tool_remaining"] = rl.config.ToolRPS
		}
	}

	// Add quota info if enabled
	if rl.config.EnableQuotas && tenantID != "" {
		used, limit, resetAt := rl.GetTenantQuota(tenantID)
		metadata["quota_used"] = used
		metadata["quota_limit"] = limit
		metadata["quota_remaining"] = limit - used
		metadata["quota_reset_at"] = resetAt.Unix()
	}

	return metadata
}

// CreateRateLimitError creates a JSON-RPC error response for rate limit exceeded
func (rl *RateLimiter) CreateRateLimitError(result *RateLimitResult) map[string]interface{} {
	errorData := map[string]interface{}{
		"limit_type":  result.LimitType,
		"limit":       result.Limit,
		"remaining":   result.Remaining,
		"reset_at":    result.ResetAt.Unix(),
		"retry_after": int(result.RetryAfter.Seconds()),
	}

	if result.LimitType == "quota" {
		errorData["quota_used"] = result.QuotaUsed
		errorData["quota_limit"] = result.QuotaLimit
		errorData["quota_reset_at"] = result.QuotaResetAt.Unix()
	}

	return errorData
}

// Close gracefully shuts down the rate limiter
func (rl *RateLimiter) Close() {
	rl.closeMu.Lock()
	defer rl.closeMu.Unlock()

	if rl.closed {
		return
	}

	close(rl.stopClean)
	rl.wg.Wait()
	rl.closed = true

	rl.logger.Info("Rate limiter shutdown complete", map[string]interface{}{
		"total_limiters": len(rl.limiters),
		"total_quotas":   len(rl.quotas),
	})
}
