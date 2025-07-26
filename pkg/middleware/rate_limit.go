package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	limiters map[string]*rateLimiterEntry
	mu       sync.RWMutex
	config   RateLimitConfig
	logger   observability.Logger
	metrics  observability.MetricsClient
}

// RateLimitConfig defines rate limiting configuration
type RateLimitConfig struct {
	// Global limits
	GlobalRPS   int // Requests per second globally
	GlobalBurst int // Burst size globally

	// Per-tenant limits
	TenantRPS   int // Requests per second per tenant
	TenantBurst int // Burst size per tenant

	// Per-tool limits
	ToolRPS   int // Requests per second per tool
	ToolBurst int // Burst size per tool

	// Cleanup
	CleanupInterval time.Duration // How often to clean up old limiters
	MaxAge          time.Duration // Maximum age for unused limiters
}

// rateLimiterEntry holds a rate limiter and its last access time
type rateLimiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// DefaultRateLimitConfig returns default rate limit configuration
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		GlobalRPS:       1000,
		GlobalBurst:     2000,
		TenantRPS:       100,
		TenantBurst:     200,
		ToolRPS:         50,
		ToolBurst:       100,
		CleanupInterval: 5 * time.Minute,
		MaxAge:          1 * time.Hour,
	}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config RateLimitConfig, logger observability.Logger, metrics observability.MetricsClient) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*rateLimiterEntry),
		config:   config,
		logger:   logger,
		metrics:  metrics,
	}

	// Start cleanup routine
	go rl.cleanupRoutine()

	return rl
}

// GlobalLimit applies global rate limiting
func (rl *RateLimiter) GlobalLimit() gin.HandlerFunc {
	limiter := rate.NewLimiter(rate.Limit(rl.config.GlobalRPS), rl.config.GlobalBurst)

	return func(c *gin.Context) {
		if !limiter.Allow() {
			rl.recordRateLimitHit("global", c.Request.URL.Path)
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rl.config.GlobalRPS))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Second).Unix()))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "global rate limit exceeded",
				"retry_after": 1,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// TenantLimit applies per-tenant rate limiting
func (rl *RateLimiter) TenantLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		if tenantID == "" {
			tenantID = c.GetHeader("X-Tenant-ID")
		}

		if tenantID == "" {
			// No tenant ID, skip rate limiting
			c.Next()
			return
		}

		key := fmt.Sprintf("tenant:%s", tenantID)
		limiter := rl.getLimiter(key, rl.config.TenantRPS, rl.config.TenantBurst)

		if !limiter.Allow() {
			rl.recordRateLimitHit("tenant", c.Request.URL.Path)
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rl.config.TenantRPS))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Second).Unix()))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "tenant rate limit exceeded",
				"retry_after": 1,
			})
			c.Abort()
			return
		}

		// Add rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rl.config.TenantRPS))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", int(limiter.Tokens())))

		c.Next()
	}
}

// ToolLimit applies per-tool rate limiting
func (rl *RateLimiter) ToolLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		if tenantID == "" {
			tenantID = c.GetHeader("X-Tenant-ID")
		}

		toolID := c.Param("toolId")
		if toolID == "" {
			// No tool ID in path, skip
			c.Next()
			return
		}

		key := fmt.Sprintf("tool:%s:%s", tenantID, toolID)
		limiter := rl.getLimiter(key, rl.config.ToolRPS, rl.config.ToolBurst)

		if !limiter.Allow() {
			rl.recordRateLimitHit("tool", c.Request.URL.Path)
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rl.config.ToolRPS))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Second).Unix()))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "tool rate limit exceeded",
				"retry_after": 1,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AdaptiveLimit applies adaptive rate limiting based on error rates
func (rl *RateLimiter) AdaptiveLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// This is a placeholder for adaptive rate limiting
		// In production, this would monitor error rates and adjust limits
		c.Next()

		// After request, check response status
		status := c.Writer.Status()
		if status >= 500 {
			// Server error - might want to back off
			tenantID := c.GetString("tenant_id")
			if tenantID != "" {
				key := fmt.Sprintf("tenant:%s", tenantID)
				rl.recordError(key)
			}
		}
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
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

// cleanup removes old limiters
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, entry := range rl.limiters {
		if now.Sub(entry.lastAccess) > rl.config.MaxAge {
			delete(rl.limiters, key)
		}
	}

	rl.logger.Debug("Rate limiter cleanup completed", map[string]interface{}{
		"remaining_limiters": len(rl.limiters),
	})
}

// recordRateLimitHit records a rate limit hit metric
func (rl *RateLimiter) recordRateLimitHit(limitType, path string) {
	if rl.metrics != nil {
		rl.metrics.IncrementCounterWithLabels("rate_limit_hits", 1.0, map[string]string{
			"type": limitType,
			"path": path,
		})
	}
}

// recordError records an error for adaptive limiting
func (rl *RateLimiter) recordError(key string) {
	// In a production system, this would track error rates
	// and potentially adjust rate limits dynamically
	if rl.metrics != nil {
		rl.metrics.IncrementCounterWithLabels("rate_limit_errors", 1.0, map[string]string{
			"key": key,
		})
	}
}

// GetStats returns current rate limiter statistics
func (rl *RateLimiter) GetStats() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	tenantCount := 0
	toolCount := 0

	for key := range rl.limiters {
		if len(key) > 7 && key[:7] == "tenant:" {
			tenantCount++
		} else if len(key) > 5 && key[:5] == "tool:" {
			toolCount++
		}
	}

	return map[string]interface{}{
		"total_limiters":  len(rl.limiters),
		"tenant_limiters": tenantCount,
		"tool_limiters":   toolCount,
		"config":          rl.config,
	}
}
