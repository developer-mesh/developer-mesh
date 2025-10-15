// Package resilience provides rate limiting for API protection
package resilience

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"golang.org/x/time/rate"
)

var (
	// ErrRateLimitExceeded is returned when rate limit is exceeded
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

// RateLimiterConfig configures rate limiter behavior
type RateLimiterConfig struct {
	// RequestsPerSecond is the sustained request rate
	RequestsPerSecond float64

	// BurstSize is the maximum burst size
	BurstSize int

	// Per-source rate limits
	PerSourceLimit *SourceRateLimits
}

// SourceRateLimits defines rate limits per source type
type SourceRateLimits struct {
	GitHub     float64 // requests/second
	Web        float64
	S3         float64
	Confluence float64
	Jira       float64
}

// DefaultRateLimiterConfig returns sensible defaults
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		RequestsPerSecond: 100, // 100 requests per second
		BurstSize:         50,  // Allow bursts up to 50
		PerSourceLimit: &SourceRateLimits{
			GitHub:     10, // GitHub API has rate limits
			Web:        20, // Web crawling
			S3:         50, // S3 is more generous
			Confluence: 10, // Confluence API limits
			Jira:       10, // Jira API limits
		},
	}
}

// RateLimiter manages request rate limiting
type RateLimiter struct {
	config         RateLimiterConfig
	globalLimiter  *rate.Limiter
	sourceLimiters map[string]*rate.Limiter
	logger         observability.Logger

	mu sync.RWMutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config RateLimiterConfig, logger observability.Logger) *RateLimiter {
	rl := &RateLimiter{
		config:         config,
		globalLimiter:  rate.NewLimiter(rate.Limit(config.RequestsPerSecond), config.BurstSize),
		sourceLimiters: make(map[string]*rate.Limiter),
		logger:         logger.WithPrefix("rate-limiter"),
	}

	// Initialize per-source limiters
	if config.PerSourceLimit != nil {
		rl.sourceLimiters["github"] = rate.NewLimiter(rate.Limit(config.PerSourceLimit.GitHub), int(config.PerSourceLimit.GitHub))
		rl.sourceLimiters["web"] = rate.NewLimiter(rate.Limit(config.PerSourceLimit.Web), int(config.PerSourceLimit.Web))
		rl.sourceLimiters["s3"] = rate.NewLimiter(rate.Limit(config.PerSourceLimit.S3), int(config.PerSourceLimit.S3))
		rl.sourceLimiters["confluence"] = rate.NewLimiter(rate.Limit(config.PerSourceLimit.Confluence), int(config.PerSourceLimit.Confluence))
		rl.sourceLimiters["jira"] = rate.NewLimiter(rate.Limit(config.PerSourceLimit.Jira), int(config.PerSourceLimit.Jira))
	}

	return rl
}

// Allow checks if a request is allowed under rate limits
func (rl *RateLimiter) Allow() bool {
	return rl.globalLimiter.Allow()
}

// AllowN checks if n requests are allowed under rate limits
func (rl *RateLimiter) AllowN(n int) bool {
	return rl.globalLimiter.AllowN(time.Now(), n)
}

// Wait blocks until a request is allowed (with context)
func (rl *RateLimiter) Wait(ctx context.Context) error {
	return rl.globalLimiter.Wait(ctx)
}

// WaitForSource blocks until a request is allowed for specific source
func (rl *RateLimiter) WaitForSource(ctx context.Context, sourceType string) error {
	// Wait on global limiter first
	if err := rl.globalLimiter.Wait(ctx); err != nil {
		return err
	}

	// Then wait on source-specific limiter if exists
	rl.mu.RLock()
	limiter, exists := rl.sourceLimiters[sourceType]
	rl.mu.RUnlock()

	if exists {
		return limiter.Wait(ctx)
	}

	return nil
}

// AllowForSource checks if a request is allowed for specific source
func (rl *RateLimiter) AllowForSource(sourceType string) bool {
	// Check global limit first
	if !rl.globalLimiter.Allow() {
		return false
	}

	// Check source-specific limit
	rl.mu.RLock()
	limiter, exists := rl.sourceLimiters[sourceType]
	rl.mu.RUnlock()

	if exists {
		return limiter.Allow()
	}

	return true
}

// TokenBucketConfig configures token bucket rate limiter
type TokenBucketConfig struct {
	// Capacity is the maximum number of tokens
	Capacity int

	// RefillRate is tokens per second
	RefillRate float64

	// InitialTokens is the starting number of tokens
	InitialTokens int
}

// TokenBucket implements a token bucket rate limiter
type TokenBucket struct {
	capacity   int
	tokens     float64
	refillRate float64
	lastRefill time.Time
	logger     observability.Logger

	mu sync.Mutex
}

// NewTokenBucket creates a new token bucket
func NewTokenBucket(config TokenBucketConfig, logger observability.Logger) *TokenBucket {
	initialTokens := config.InitialTokens
	if initialTokens > config.Capacity {
		initialTokens = config.Capacity
	}
	if initialTokens == 0 {
		initialTokens = config.Capacity
	}

	return &TokenBucket{
		capacity:   config.Capacity,
		tokens:     float64(initialTokens),
		refillRate: config.RefillRate,
		lastRefill: time.Now(),
		logger:     logger.WithPrefix("token-bucket"),
	}
}

// Take attempts to take n tokens from the bucket
func (tb *TokenBucket) Take(n int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens >= float64(n) {
		tb.tokens -= float64(n)
		return true
	}

	tb.logger.Debug("Rate limit exceeded", map[string]interface{}{
		"requested": n,
		"available": tb.tokens,
	})

	return false
}

// TakeWait blocks until n tokens are available
func (tb *TokenBucket) TakeWait(ctx context.Context, n int) error {
	for {
		if tb.Take(n) {
			return nil
		}

		// Calculate wait time
		tb.mu.Lock()
		needed := float64(n) - tb.tokens
		waitTime := time.Duration(needed/tb.refillRate) * time.Second
		tb.mu.Unlock()

		// Wait with context
		select {
		case <-time.After(waitTime):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// refill adds tokens based on elapsed time
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	tokensToAdd := elapsed.Seconds() * tb.refillRate
	tb.tokens += tokensToAdd

	if tb.tokens > float64(tb.capacity) {
		tb.tokens = float64(tb.capacity)
	}

	tb.lastRefill = now
}

// Available returns the number of available tokens
func (tb *TokenBucket) Available() int {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()
	return int(tb.tokens)
}

// Capacity returns the bucket capacity
func (tb *TokenBucket) Capacity() int {
	return tb.capacity
}

// AdaptiveRateLimiter adjusts rate limits based on system load
type AdaptiveRateLimiter struct {
	baseLimiter    *RateLimiter
	currentRate    float64
	minRate        float64
	maxRate        float64
	adjustInterval time.Duration
	logger         observability.Logger

	mu sync.RWMutex
}

// AdaptiveRateLimiterConfig configures adaptive rate limiter
type AdaptiveRateLimiterConfig struct {
	InitialRate    float64
	MinRate        float64
	MaxRate        float64
	BurstSize      int
	AdjustInterval time.Duration
}

// NewAdaptiveRateLimiter creates an adaptive rate limiter
func NewAdaptiveRateLimiter(config AdaptiveRateLimiterConfig, logger observability.Logger) *AdaptiveRateLimiter {
	baseConfig := RateLimiterConfig{
		RequestsPerSecond: config.InitialRate,
		BurstSize:         config.BurstSize,
	}

	return &AdaptiveRateLimiter{
		baseLimiter:    NewRateLimiter(baseConfig, logger),
		currentRate:    config.InitialRate,
		minRate:        config.MinRate,
		maxRate:        config.MaxRate,
		adjustInterval: config.AdjustInterval,
		logger:         logger.WithPrefix("adaptive-limiter"),
	}
}

// Allow checks if a request is allowed
func (arl *AdaptiveRateLimiter) Allow() bool {
	return arl.baseLimiter.Allow()
}

// AdjustRate adjusts the rate based on system metrics
func (arl *AdaptiveRateLimiter) AdjustRate(errorRate, latency float64) {
	arl.mu.Lock()
	defer arl.mu.Unlock()

	oldRate := arl.currentRate
	newRate := arl.currentRate

	// Decrease rate if error rate is high
	if errorRate > 0.1 {
		newRate = arl.currentRate * 0.9
	}

	// Decrease rate if latency is high
	if latency > 1.0 {
		newRate = arl.currentRate * 0.95
	}

	// Increase rate if system is healthy
	if errorRate < 0.01 && latency < 0.5 {
		newRate = arl.currentRate * 1.05
	}

	// Clamp to min/max
	if newRate < arl.minRate {
		newRate = arl.minRate
	}
	if newRate > arl.maxRate {
		newRate = arl.maxRate
	}

	// Update if changed significantly
	if newRate != oldRate {
		arl.currentRate = newRate
		arl.baseLimiter.globalLimiter.SetLimit(rate.Limit(newRate))

		arl.logger.Info("Rate limit adjusted", map[string]interface{}{
			"old_rate":   oldRate,
			"new_rate":   newRate,
			"error_rate": errorRate,
			"latency":    latency,
		})
	}
}

// CurrentRate returns the current rate limit
func (arl *AdaptiveRateLimiter) CurrentRate() float64 {
	arl.mu.RLock()
	defer arl.mu.RUnlock()
	return arl.currentRate
}
