package resilience

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

func TestRateLimiter_Allow(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := RateLimiterConfig{
		RequestsPerSecond: 10,
		BurstSize:         5,
	}
	rl := NewRateLimiter(config, logger)

	// Should allow requests up to burst size immediately
	for i := 0; i < 5; i++ {
		assert.True(t, rl.Allow())
	}

	// Next request might be denied (depends on timing)
	// This is expected behavior for token bucket
}

func TestRateLimiter_Wait(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := RateLimiterConfig{
		RequestsPerSecond: 100,
		BurstSize:         10,
	}
	rl := NewRateLimiter(config, logger)

	ctx := context.Background()

	// Should wait successfully
	err := rl.Wait(ctx)
	assert.NoError(t, err)
}

func TestRateLimiter_WaitWithTimeout(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := RateLimiterConfig{
		RequestsPerSecond: 1,
		BurstSize:         1,
	}
	rl := NewRateLimiter(config, logger)

	// Consume the burst
	assert.True(t, rl.Allow())

	// Context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := rl.Wait(ctx)
	assert.Error(t, err)
	// The error might be context.DeadlineExceeded or a rate limiter error
	// Just check that an error occurred
}

func TestRateLimiter_PerSource(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := RateLimiterConfig{
		RequestsPerSecond: 100,
		BurstSize:         50,
		PerSourceLimit: &SourceRateLimits{
			GitHub: 10,
			Web:    20,
		},
	}
	rl := NewRateLimiter(config, logger)

	// Should allow requests for different sources
	assert.True(t, rl.AllowForSource("github"))
	assert.True(t, rl.AllowForSource("web"))
	assert.True(t, rl.AllowForSource("unknown")) // Unknown sources use global limit
}

func TestTokenBucket_Take(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := TokenBucketConfig{
		Capacity:      10,
		RefillRate:    1.0,
		InitialTokens: 10,
	}
	tb := NewTokenBucket(config, logger)

	// Should take tokens successfully
	assert.True(t, tb.Take(5))
	assert.Equal(t, 5, tb.Available())

	// Should fail when not enough tokens
	assert.False(t, tb.Take(10))

	// Should still have remaining tokens
	assert.Equal(t, 5, tb.Available())
}

func TestTokenBucket_Refill(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := TokenBucketConfig{
		Capacity:      10,
		RefillRate:    10.0, // 10 tokens per second
		InitialTokens: 10,   // Start with full bucket
	}
	tb := NewTokenBucket(config, logger)

	// Start with full bucket
	assert.Equal(t, 10, tb.Available())

	// Consume all tokens
	assert.True(t, tb.Take(10))
	assert.Equal(t, 0, tb.Available())

	// Wait for refill
	time.Sleep(200 * time.Millisecond)

	// Should have refilled some tokens (approximately 2)
	available := tb.Available()
	assert.Greater(t, available, 0)
	assert.LessOrEqual(t, available, 10)
}

func TestTokenBucket_TakeWait(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := TokenBucketConfig{
		Capacity:      10,
		RefillRate:    10.0,
		InitialTokens: 10,
	}
	tb := NewTokenBucket(config, logger)

	// Consume all tokens first
	assert.True(t, tb.Take(10))

	ctx := context.Background()

	// Should wait for tokens to become available
	start := time.Now()
	err := tb.TakeWait(ctx, 2)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Greater(t, duration, 100*time.Millisecond) // Should have waited
}

func TestTokenBucket_TakeWaitWithTimeout(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := TokenBucketConfig{
		Capacity:      10,
		RefillRate:    1.0, // Slow refill
		InitialTokens: 10,
	}
	tb := NewTokenBucket(config, logger)

	// Consume all tokens
	assert.True(t, tb.Take(10))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Should timeout waiting for tokens (needs 5 tokens, refills at 1/sec, timeout is 50ms)
	err := tb.TakeWait(ctx, 5)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestAdaptiveRateLimiter_AdjustRate(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := AdaptiveRateLimiterConfig{
		InitialRate:    100,
		MinRate:        10,
		MaxRate:        200,
		BurstSize:      50,
		AdjustInterval: 100 * time.Millisecond,
	}
	arl := NewAdaptiveRateLimiter(config, logger)

	initialRate := arl.CurrentRate()
	assert.Equal(t, 100.0, initialRate)

	// Simulate high error rate - should decrease rate
	arl.AdjustRate(0.2, 0.5) // 20% errors, 0.5s latency
	assert.Less(t, arl.CurrentRate(), initialRate)

	// Reset for next test
	arl = NewAdaptiveRateLimiter(config, logger)

	// Simulate healthy system - should increase rate
	arl.AdjustRate(0.001, 0.1) // 0.1% errors, 0.1s latency
	assert.Greater(t, arl.CurrentRate(), initialRate)
}

func TestAdaptiveRateLimiter_RateBounds(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := AdaptiveRateLimiterConfig{
		InitialRate:    100,
		MinRate:        50,
		MaxRate:        150,
		BurstSize:      50,
		AdjustInterval: 100 * time.Millisecond,
	}
	arl := NewAdaptiveRateLimiter(config, logger)

	// Try to decrease below min
	for i := 0; i < 10; i++ {
		arl.AdjustRate(1.0, 10.0) // Very high errors and latency
	}
	assert.GreaterOrEqual(t, arl.CurrentRate(), 50.0)

	// Reset
	arl = NewAdaptiveRateLimiter(config, logger)

	// Try to increase above max
	for i := 0; i < 10; i++ {
		arl.AdjustRate(0.0, 0.0) // Perfect performance
	}
	assert.LessOrEqual(t, arl.CurrentRate(), 150.0)
}

func BenchmarkRateLimiter_Allow(b *testing.B) {
	logger := observability.NewNoopLogger()
	config := RateLimiterConfig{
		RequestsPerSecond: 1000,
		BurstSize:         500,
	}
	rl := NewRateLimiter(config, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Allow()
	}
}

func BenchmarkTokenBucket_Take(b *testing.B) {
	logger := observability.NewNoopLogger()
	config := TokenBucketConfig{
		Capacity:      1000,
		RefillRate:    1000.0,
		InitialTokens: 1000,
	}
	tb := NewTokenBucket(config, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tb.Take(1)
	}
}
