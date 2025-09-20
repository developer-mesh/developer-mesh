package builtin

import (
	"sync"
	"time"
)

// RateLimiter provides simple rate limiting for tools
type RateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*TokenBucket
	configs map[string]*RateLimitInfo
}

// TokenBucket implements a simple token bucket algorithm
type TokenBucket struct {
	tokens     int
	maxTokens  int
	refillRate int // tokens per minute
	lastRefill time.Time
}

// GlobalRateLimiter is a singleton rate limiter for all tools
var GlobalRateLimiter = NewRateLimiter()

// GlobalIdempotencyStore is a singleton for idempotency
var GlobalIdempotencyStore = NewIdempotencyStore()

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*TokenBucket),
		configs: make(map[string]*RateLimitInfo),
	}

	// Initialize default configs
	rl.initializeConfigs()

	// Start refill goroutine
	go rl.refillBuckets()

	return rl
}

// initializeConfigs sets up rate limit configurations for each tool
func (rl *RateLimiter) initializeConfigs() {
	// Use the existing GetRateLimitForTool configurations
	tools := []string{
		"agent_heartbeat", "agent_list", "agent_status",
		"workflow_create", "workflow_execute", "workflow_list", "workflow_get",
		"workflow_execution_list", "workflow_execution_get", "workflow_cancel",
		"task_create", "task_assign", "task_complete", "task_list", "task_get", "task_get_batch",
		"context_update", "context_append", "context_get", "context_list",
		"template_list", "template_get", "template_instantiate",
	}

	for _, tool := range tools {
		config := GetRateLimitForTool(tool)
		rl.configs[tool] = config
		rl.buckets[tool] = &TokenBucket{
			tokens:     config.BurstSize,
			maxTokens:  config.BurstSize,
			refillRate: config.RequestsPerMinute,
			lastRefill: time.Now(),
		}
	}
}

// CheckAndConsume checks if a request can proceed and consumes a token
func (rl *RateLimiter) CheckAndConsume(toolName string) (allowed bool, status *RateLimitStatus) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.buckets[toolName]
	if !exists {
		// Create default bucket if not exists
		config := GetRateLimitForTool(toolName)
		bucket = &TokenBucket{
			tokens:     config.BurstSize,
			maxTokens:  config.BurstSize,
			refillRate: config.RequestsPerMinute,
			lastRefill: time.Now(),
		}
		rl.buckets[toolName] = bucket
		rl.configs[toolName] = config
	}

	// Refill tokens based on time elapsed
	rl.refillBucket(bucket)

	// Check if token available
	if bucket.tokens > 0 {
		bucket.tokens--
		allowed = true
	} else {
		allowed = false
	}

	// Calculate when bucket will reset (have tokens again)
	resetTime := time.Now().Add(time.Minute / time.Duration(bucket.refillRate))

	status = &RateLimitStatus{
		Remaining: bucket.tokens,
		Reset:     resetTime,
		Limit:     bucket.maxTokens,
	}

	return allowed, status
}

// refillBucket refills tokens based on time elapsed
func (rl *RateLimiter) refillBucket(bucket *TokenBucket) {
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill)

	// Calculate tokens to add (rate is per minute)
	tokensToAdd := int(elapsed.Minutes() * float64(bucket.refillRate))

	if tokensToAdd > 0 {
		bucket.tokens = min(bucket.tokens+tokensToAdd, bucket.maxTokens)
		bucket.lastRefill = now
	}
}

// refillBuckets periodically refills all buckets
func (rl *RateLimiter) refillBuckets() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		for _, bucket := range rl.buckets {
			rl.refillBucket(bucket)
		}
		rl.mu.Unlock()
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CheckIdempotency checks if this is a duplicate request
func CheckIdempotency(key string) (interface{}, bool) {
	if key == "" {
		return nil, false
	}
	return GlobalIdempotencyStore.Get(key)
}

// StoreIdempotentResponse stores a response for idempotency
func StoreIdempotentResponse(key string, response interface{}) {
	if key == "" {
		return
	}
	GlobalIdempotencyStore.Set(key, response, 5*time.Minute)
}
