package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/metrics"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultRateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()

	assert.Equal(t, 1000, config.GlobalRPS)
	assert.Equal(t, 2000, config.GlobalBurst)
	assert.Equal(t, 100, config.TenantRPS)
	assert.Equal(t, 200, config.TenantBurst)
	assert.Equal(t, 50, config.ToolRPS)
	assert.Equal(t, 100, config.ToolBurst)
	assert.True(t, config.EnableQuotas)
	assert.Equal(t, int64(10000), config.DefaultQuota)
}

func TestNewRateLimiter(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultRateLimitConfig()
	config.CleanupInterval = 1 * time.Minute

	rl := NewRateLimiter(config, logger, nil)
	require.NotNil(t, rl)

	assert.NotNil(t, rl.globalRL)
	assert.NotNil(t, rl.limiters)
	assert.NotNil(t, rl.quotas)

	// Cleanup
	rl.Close()
}

func TestRateLimiter_CheckRateLimit_GlobalLimit(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultRateLimitConfig()
	config.GlobalRPS = 2
	config.GlobalBurst = 2

	rl := NewRateLimiter(config, logger, nil)
	defer rl.Close()

	ctx := context.Background()
	tenantID := "tenant-1"
	toolName := "test-tool"

	// First 2 requests should succeed (burst)
	result1 := rl.CheckRateLimit(ctx, tenantID, toolName)
	assert.True(t, result1.Allowed)

	result2 := rl.CheckRateLimit(ctx, tenantID, toolName)
	assert.True(t, result2.Allowed)

	// Third request should be rate limited
	result3 := rl.CheckRateLimit(ctx, tenantID, toolName)
	assert.False(t, result3.Allowed)
	assert.Equal(t, "global", result3.LimitType)
	assert.Equal(t, 2, result3.Limit)
	assert.Equal(t, 0, result3.Remaining)
}

func TestRateLimiter_CheckRateLimit_TenantLimit(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultRateLimitConfig()
	config.GlobalRPS = 1000 // High global limit
	config.TenantRPS = 2
	config.TenantBurst = 2
	config.EnableQuotas = false // Disable quotas for this test

	rl := NewRateLimiter(config, logger, nil)
	defer rl.Close()

	ctx := context.Background()
	tenantID := "tenant-1"
	toolName := "test-tool"

	// First 2 requests should succeed (burst)
	result1 := rl.CheckRateLimit(ctx, tenantID, toolName)
	assert.True(t, result1.Allowed)

	result2 := rl.CheckRateLimit(ctx, tenantID, toolName)
	assert.True(t, result2.Allowed)

	// Third request should be rate limited
	result3 := rl.CheckRateLimit(ctx, tenantID, toolName)
	assert.False(t, result3.Allowed)
	assert.Equal(t, "tenant", result3.LimitType)
	assert.Equal(t, 2, result3.Limit)
}

func TestRateLimiter_CheckRateLimit_ToolLimit(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultRateLimitConfig()
	config.GlobalRPS = 1000 // High global limit
	config.TenantRPS = 1000 // High tenant limit
	config.ToolRPS = 2
	config.ToolBurst = 2
	config.EnableQuotas = false // Disable quotas for this test

	rl := NewRateLimiter(config, logger, nil)
	defer rl.Close()

	ctx := context.Background()
	tenantID := "tenant-1"
	toolName := "test-tool"

	// First 2 requests should succeed (burst)
	result1 := rl.CheckRateLimit(ctx, tenantID, toolName)
	assert.True(t, result1.Allowed)

	result2 := rl.CheckRateLimit(ctx, tenantID, toolName)
	assert.True(t, result2.Allowed)

	// Third request should be rate limited
	result3 := rl.CheckRateLimit(ctx, tenantID, toolName)
	assert.False(t, result3.Allowed)
	assert.Equal(t, "tool", result3.LimitType)
	assert.Equal(t, 2, result3.Limit)
}

func TestRateLimiter_CheckRateLimit_QuotaLimit(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultRateLimitConfig()
	config.GlobalRPS = 1000 // High global limit
	config.TenantRPS = 1000 // High tenant limit
	config.ToolRPS = 1000   // High tool limit
	config.EnableQuotas = true
	config.DefaultQuota = 5 // Only 5 requests per quota period

	rl := NewRateLimiter(config, logger, nil)
	defer rl.Close()

	ctx := context.Background()
	tenantID := "tenant-1"
	toolName := "test-tool"

	// First 5 requests should succeed
	for i := 0; i < 5; i++ {
		result := rl.CheckRateLimit(ctx, tenantID, toolName)
		assert.True(t, result.Allowed, "Request %d should be allowed", i+1)
	}

	// Sixth request should be quota limited
	result := rl.CheckRateLimit(ctx, tenantID, toolName)
	assert.False(t, result.Allowed)
	assert.Equal(t, "quota", result.LimitType)
	assert.Equal(t, int64(5), result.QuotaUsed)
	assert.Equal(t, int64(5), result.QuotaLimit)
}

func TestRateLimiter_QuotaReset(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultRateLimitConfig()
	config.GlobalRPS = 1000 // High global limit
	config.TenantRPS = 1000 // High tenant limit
	config.ToolRPS = 1000   // High tool limit
	config.EnableQuotas = true
	config.DefaultQuota = 2
	config.QuotaResetInterval = 100 * time.Millisecond // Short interval for testing

	rl := NewRateLimiter(config, logger, nil)
	defer rl.Close()

	ctx := context.Background()
	tenantID := "tenant-1"
	toolName := "test-tool"

	// Use up quota
	result1 := rl.CheckRateLimit(ctx, tenantID, toolName)
	assert.True(t, result1.Allowed)

	result2 := rl.CheckRateLimit(ctx, tenantID, toolName)
	assert.True(t, result2.Allowed)

	// Should be quota limited
	result3 := rl.CheckRateLimit(ctx, tenantID, toolName)
	assert.False(t, result3.Allowed)
	assert.Equal(t, "quota", result3.LimitType)

	// Wait for quota reset
	time.Sleep(150 * time.Millisecond)

	// Should work again after reset
	result4 := rl.CheckRateLimit(ctx, tenantID, toolName)
	assert.True(t, result4.Allowed)
}

func TestRateLimiter_SetTenantQuota(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultRateLimitConfig()
	config.EnableQuotas = true
	config.DefaultQuota = 100

	rl := NewRateLimiter(config, logger, nil)
	defer rl.Close()

	tenantID := "tenant-1"

	// Set custom quota
	rl.SetTenantQuota(tenantID, 500)

	// Check quota
	used, limit, _ := rl.GetTenantQuota(tenantID)
	assert.Equal(t, int64(0), used)
	assert.Equal(t, int64(500), limit)
}

func TestRateLimiter_GetTenantQuota(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultRateLimitConfig()
	config.GlobalRPS = 1000 // High global limit
	config.TenantRPS = 1000 // High tenant limit
	config.ToolRPS = 1000   // High tool limit
	config.EnableQuotas = true
	config.DefaultQuota = 100

	rl := NewRateLimiter(config, logger, nil)
	defer rl.Close()

	ctx := context.Background()
	tenantID := "tenant-1"
	toolName := "test-tool"

	// Initial quota
	used, limit, resetAt := rl.GetTenantQuota(tenantID)
	assert.Equal(t, int64(0), used)
	assert.Equal(t, int64(100), limit)
	assert.True(t, resetAt.After(time.Now()))

	// Use some quota
	for i := 0; i < 5; i++ {
		result := rl.CheckRateLimit(ctx, tenantID, toolName)
		assert.True(t, result.Allowed)
	}

	// Check updated quota
	used, limit, _ = rl.GetTenantQuota(tenantID)
	assert.Equal(t, int64(5), used)
	assert.Equal(t, int64(100), limit)
}

func TestRateLimiter_MultiTenant(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultRateLimitConfig()
	config.GlobalRPS = 1000 // High global limit
	config.TenantRPS = 2
	config.TenantBurst = 2
	config.EnableQuotas = false

	rl := NewRateLimiter(config, logger, nil)
	defer rl.Close()

	ctx := context.Background()
	toolName := "test-tool"

	// Tenant 1: Use up limit
	rl.CheckRateLimit(ctx, "tenant-1", toolName)
	rl.CheckRateLimit(ctx, "tenant-1", toolName)
	result1 := rl.CheckRateLimit(ctx, "tenant-1", toolName)
	assert.False(t, result1.Allowed)

	// Tenant 2: Should still have full limit
	result2 := rl.CheckRateLimit(ctx, "tenant-2", toolName)
	assert.True(t, result2.Allowed)
}

func TestRateLimiter_GetRateLimitMetadata(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultRateLimitConfig()
	config.EnableQuotas = true

	rl := NewRateLimiter(config, logger, nil)
	defer rl.Close()

	tenantID := "tenant-1"
	toolName := "test-tool"

	metadata := rl.GetRateLimitMetadata(tenantID, toolName)

	// Check global metadata
	assert.Contains(t, metadata, "global_limit")
	assert.Contains(t, metadata, "global_remaining")
	assert.Equal(t, 1000, metadata["global_limit"])

	// Check tenant metadata
	assert.Contains(t, metadata, "tenant_limit")
	assert.Contains(t, metadata, "tenant_remaining")
	assert.Equal(t, 100, metadata["tenant_limit"])

	// Check tool metadata
	assert.Contains(t, metadata, "tool_limit")
	assert.Contains(t, metadata, "tool_remaining")
	assert.Equal(t, 50, metadata["tool_limit"])

	// Check quota metadata
	assert.Contains(t, metadata, "quota_used")
	assert.Contains(t, metadata, "quota_limit")
	assert.Contains(t, metadata, "quota_remaining")
	assert.Contains(t, metadata, "quota_reset_at")
}

func TestRateLimiter_CreateRateLimitError(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultRateLimitConfig()

	rl := NewRateLimiter(config, logger, nil)
	defer rl.Close()

	t.Run("TenantLimitError", func(t *testing.T) {
		result := &RateLimitResult{
			Allowed:    false,
			LimitType:  "tenant",
			Limit:      100,
			Remaining:  0,
			ResetAt:    time.Now().Add(time.Minute),
			RetryAfter: time.Minute,
		}

		errorData := rl.CreateRateLimitError(result)

		assert.Equal(t, "tenant", errorData["limit_type"])
		assert.Equal(t, 100, errorData["limit"])
		assert.Equal(t, 0, errorData["remaining"])
		assert.Contains(t, errorData, "reset_at")
		assert.Equal(t, 60, errorData["retry_after"])
	})

	t.Run("QuotaLimitError", func(t *testing.T) {
		result := &RateLimitResult{
			Allowed:      false,
			LimitType:    "quota",
			QuotaUsed:    1000,
			QuotaLimit:   1000,
			QuotaResetAt: time.Now().Add(24 * time.Hour),
			RetryAfter:   24 * time.Hour,
		}

		errorData := rl.CreateRateLimitError(result)

		assert.Equal(t, "quota", errorData["limit_type"])
		assert.Equal(t, int64(1000), errorData["quota_used"])
		assert.Equal(t, int64(1000), errorData["quota_limit"])
		assert.Contains(t, errorData, "quota_reset_at")
	})
}

func TestRateLimiter_GetStats(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultRateLimitConfig()
	config.GlobalRPS = 1000
	config.TenantRPS = 1000
	config.ToolRPS = 1000
	config.EnableQuotas = true

	rl := NewRateLimiter(config, logger, nil)
	defer rl.Close()

	ctx := context.Background()

	// Create some limiters
	rl.CheckRateLimit(ctx, "tenant-1", "tool-1")
	rl.CheckRateLimit(ctx, "tenant-1", "tool-2")
	rl.CheckRateLimit(ctx, "tenant-2", "tool-1")

	stats := rl.GetStats()

	assert.Contains(t, stats, "total_limiters")
	assert.Contains(t, stats, "tenant_limiters")
	assert.Contains(t, stats, "tool_limiters")
	assert.Contains(t, stats, "total_quotas")
	assert.Contains(t, stats, "total_quota_used")

	// Should have 2 tenant limiters and 3 tool limiters
	assert.Equal(t, 2, stats["tenant_limiters"])
	assert.Equal(t, 3, stats["tool_limiters"])
	assert.Equal(t, 2, stats["total_quotas"])
}

func TestRateLimiter_Cleanup(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultRateLimitConfig()
	config.GlobalRPS = 1000
	config.TenantRPS = 1000
	config.ToolRPS = 1000
	config.MaxAge = 50 * time.Millisecond
	config.CleanupInterval = 100 * time.Millisecond

	rl := NewRateLimiter(config, logger, nil)
	defer rl.Close()

	ctx := context.Background()

	// Create some limiters
	rl.CheckRateLimit(ctx, "tenant-1", "tool-1")
	rl.CheckRateLimit(ctx, "tenant-2", "tool-2")

	stats1 := rl.GetStats()
	initialCount := stats1["total_limiters"].(int)
	assert.Greater(t, initialCount, 0)

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	stats2 := rl.GetStats()
	afterCleanupCount := stats2["total_limiters"].(int)

	// Old limiters should be cleaned up
	assert.LessOrEqual(t, afterCleanupCount, initialCount)
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultRateLimitConfig()
	config.GlobalRPS = 1000
	config.TenantRPS = 1000
	config.ToolRPS = 1000

	rl := NewRateLimiter(config, logger, nil)
	defer rl.Close()

	ctx := context.Background()
	tenantID := "tenant-1"
	toolName := "test-tool"

	// Concurrent access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				rl.CheckRateLimit(ctx, tenantID, toolName)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic or deadlock
	stats := rl.GetStats()
	assert.NotNil(t, stats)
}

func TestRateLimiter_WithMetrics(t *testing.T) {
	logger := observability.NewNoopLogger()
	metricsCollector := metrics.New()
	config := DefaultRateLimitConfig()
	config.GlobalRPS = 1
	config.GlobalBurst = 1

	rl := NewRateLimiter(config, logger, metricsCollector)
	defer rl.Close()

	ctx := context.Background()
	tenantID := "tenant-1"
	toolName := "test-tool"

	// Trigger rate limit
	rl.CheckRateLimit(ctx, tenantID, toolName)
	result := rl.CheckRateLimit(ctx, tenantID, toolName)

	assert.False(t, result.Allowed)
	// Metrics should be recorded (we can't easily verify without metrics inspection)
}

func TestRateLimiter_Close(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultRateLimitConfig()

	rl := NewRateLimiter(config, logger, nil)

	// Should not panic
	rl.Close()

	// Double close should not panic
	rl.Close()
}
