package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/middleware"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// CacheRateLimiter provides rate limiting specifically for cache operations
type CacheRateLimiter struct {
	baseRateLimiter *middleware.RateLimiter
	cacheLimiters   map[string]*rate.Limiter
	mu              sync.RWMutex
	config          CacheRateLimitConfig
	logger          observability.Logger
	metrics         observability.MetricsClient
}

// CacheRateLimitConfig defines cache-specific rate limit configuration
type CacheRateLimitConfig struct {
	// Cache-specific limits
	CacheReadRPS    int // Reads per second per tenant
	CacheReadBurst  int // Burst size for reads
	CacheWriteRPS   int // Writes per second per tenant
	CacheWriteBurst int // Burst size for writes

	// Cleanup
	CleanupInterval time.Duration
	MaxAge          time.Duration
}

// DefaultCacheRateLimitConfig returns default cache rate limit configuration
func DefaultCacheRateLimitConfig() CacheRateLimitConfig {
	return CacheRateLimitConfig{
		CacheReadRPS:    100,
		CacheReadBurst:  200,
		CacheWriteRPS:   50,
		CacheWriteBurst: 100,
		CleanupInterval: 5 * time.Minute,
		MaxAge:          1 * time.Hour,
	}
}

// NewCacheRateLimiter creates a new cache-aware rate limiter
func NewCacheRateLimiter(
	baseRateLimiter *middleware.RateLimiter,
	config CacheRateLimitConfig,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *CacheRateLimiter {
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.ratelimit")
	}
	if metrics == nil {
		metrics = observability.NewMetricsClient()
	}

	rl := &CacheRateLimiter{
		baseRateLimiter: baseRateLimiter,
		cacheLimiters:   make(map[string]*rate.Limiter),
		config:          config,
		logger:          logger,
		metrics:         metrics,
	}

	// Start cleanup routine
	go rl.cleanupRoutine()

	return rl
}

// CacheReadLimit applies rate limiting for cache read operations
func (rl *CacheRateLimiter) CacheReadLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := auth.GetTenantID(c.Request.Context())
		if tenantID == uuid.Nil {
			c.Next()
			return
		}

		key := fmt.Sprintf("cache:read:%s", tenantID.String())
		limiter := rl.getLimiter(key, rl.config.CacheReadRPS, rl.config.CacheReadBurst)

		if !limiter.Allow() {
			rl.recordRateLimitHit("cache_read", c.Request.URL.Path, tenantID)
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rl.config.CacheReadRPS))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Second).Unix()))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "cache read rate limit exceeded",
				"retry_after": 1,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// CacheWriteLimit applies rate limiting for cache write operations
func (rl *CacheRateLimiter) CacheWriteLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := auth.GetTenantID(c.Request.Context())
		if tenantID == uuid.Nil {
			c.Next()
			return
		}

		key := fmt.Sprintf("cache:write:%s", tenantID.String())
		limiter := rl.getLimiter(key, rl.config.CacheWriteRPS, rl.config.CacheWriteBurst)

		if !limiter.Allow() {
			rl.recordRateLimitHit("cache_write", c.Request.URL.Path, tenantID)
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rl.config.CacheWriteRPS))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Second).Unix()))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "cache write rate limit exceeded",
				"retry_after": 1,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Allow checks if a cache operation is allowed for a tenant
func (rl *CacheRateLimiter) Allow(tenantID uuid.UUID, operation string) bool {
	var rps, burst int
	switch operation {
	case "read":
		rps = rl.config.CacheReadRPS
		burst = rl.config.CacheReadBurst
	case "write":
		rps = rl.config.CacheWriteRPS
		burst = rl.config.CacheWriteBurst
	default:
		return true // Unknown operation, allow by default
	}

	key := fmt.Sprintf("cache:%s:%s", operation, tenantID.String())
	limiter := rl.getLimiter(key, rps, burst)
	return limiter.Allow()
}

// getLimiter gets or creates a rate limiter for a key
func (rl *CacheRateLimiter) getLimiter(key string, rps, burst int) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.cacheLimiters[key]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	// Create new limiter
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists := rl.cacheLimiters[key]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(rate.Limit(rps), burst)
	rl.cacheLimiters[key] = limiter
	return limiter
}

// recordRateLimitHit records a rate limit hit in metrics
func (rl *CacheRateLimiter) recordRateLimitHit(limitType, path string, tenantID uuid.UUID) {
	rl.metrics.IncrementCounterWithLabels("cache.rate_limit.hit", 1, map[string]string{
		"type":      limitType,
		"path":      path,
		"tenant_id": tenantID.String(),
	})

	rl.logger.Warn("Cache rate limit hit", map[string]interface{}{
		"type":      limitType,
		"path":      path,
		"tenant_id": tenantID.String(),
	})
}

// cleanupRoutine periodically cleans up old limiters
func (rl *CacheRateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

// cleanup removes old limiters
func (rl *CacheRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// For simplicity, just clear all limiters periodically
	// In production, you'd track last access time
	oldCount := len(rl.cacheLimiters)
	rl.cacheLimiters = make(map[string]*rate.Limiter)

	rl.logger.Debug("Cache rate limiter cleanup completed", map[string]interface{}{
		"removed_limiters": oldCount,
	})
}

// GetStats returns rate limiter statistics
func (rl *CacheRateLimiter) GetStats() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return map[string]interface{}{
		"active_limiters": len(rl.cacheLimiters),
		"config": map[string]interface{}{
			"read_rps":    rl.config.CacheReadRPS,
			"read_burst":  rl.config.CacheReadBurst,
			"write_rps":   rl.config.CacheWriteRPS,
			"write_burst": rl.config.CacheWriteBurst,
		},
	}
}
