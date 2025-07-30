package lru

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EvictionPolicy defines the interface for cache eviction policies
type EvictionPolicy interface {
	ShouldEvict(ctx context.Context, tenantID uuid.UUID, stats TenantStats) bool
	GetEvictionTarget(ctx context.Context, tenantID uuid.UUID, stats TenantStats) int
}

// TenantStats contains cache statistics for a tenant
type TenantStats struct {
	EntryCount   int
	TotalBytes   int64
	LastEviction time.Time
	HitRate      float64
}

// SizeBasedPolicy evicts based on entry count and byte size
type SizeBasedPolicy struct {
	maxEntries int
	maxBytes   int64
}

// NewSizeBasedPolicy creates a new size-based eviction policy
func NewSizeBasedPolicy(maxEntries int, maxBytes int64) *SizeBasedPolicy {
	return &SizeBasedPolicy{
		maxEntries: maxEntries,
		maxBytes:   maxBytes,
	}
}

// ShouldEvict checks if eviction is needed based on size limits
func (p *SizeBasedPolicy) ShouldEvict(ctx context.Context, tenantID uuid.UUID, stats TenantStats) bool {
	return stats.EntryCount > p.maxEntries || stats.TotalBytes > p.maxBytes
}

// GetEvictionTarget calculates the target entry count after eviction
func (p *SizeBasedPolicy) GetEvictionTarget(ctx context.Context, tenantID uuid.UUID, stats TenantStats) int {
	// Evict 10% when over limit
	if stats.EntryCount > p.maxEntries {
		return int(float64(p.maxEntries) * 0.9)
	}

	if stats.TotalBytes > p.maxBytes {
		// Estimate entries to remove based on average size
		if stats.EntryCount == 0 {
			return 0
		}
		avgSize := stats.TotalBytes / int64(stats.EntryCount)
		if avgSize == 0 {
			avgSize = 1024 // Default to 1KB if no size info
		}
		bytesToRemove := stats.TotalBytes - int64(float64(p.maxBytes)*0.9)
		entriesToRemove := int(bytesToRemove / avgSize)
		return stats.EntryCount - entriesToRemove
	}

	return stats.EntryCount
}

// AdaptivePolicy adjusts eviction based on hit rate
type AdaptivePolicy struct {
	base       EvictionPolicy
	minHitRate float64
	config     *Config
}

// NewAdaptivePolicy creates a new adaptive eviction policy
func NewAdaptivePolicy(base EvictionPolicy, minHitRate float64, config *Config) *AdaptivePolicy {
	return &AdaptivePolicy{
		base:       base,
		minHitRate: minHitRate,
		config:     config,
	}
}

// ShouldEvict checks if eviction is needed with adaptive thresholds
func (p *AdaptivePolicy) ShouldEvict(ctx context.Context, tenantID uuid.UUID, stats TenantStats) bool {
	// More aggressive eviction for low hit rates
	if stats.HitRate < p.minHitRate {
		// Evict if over 80% of max capacity when hit rate is low
		maxEntries := p.config.MaxTenantEntries
		if sizePolicy, ok := p.base.(*SizeBasedPolicy); ok {
			maxEntries = sizePolicy.maxEntries
		}
		return stats.EntryCount > int(float64(maxEntries)*0.8)
	}
	return p.base.ShouldEvict(ctx, tenantID, stats)
}

// GetEvictionTarget calculates target with adaptive adjustments
func (p *AdaptivePolicy) GetEvictionTarget(ctx context.Context, tenantID uuid.UUID, stats TenantStats) int {
	// For low hit rates, evict more aggressively
	if stats.HitRate < p.minHitRate {
		maxEntries := p.config.MaxTenantEntries
		if sizePolicy, ok := p.base.(*SizeBasedPolicy); ok {
			maxEntries = sizePolicy.maxEntries
		}
		// Target 70% of max when hit rate is low
		return int(float64(maxEntries) * 0.7)
	}
	return p.base.GetEvictionTarget(ctx, tenantID, stats)
}

// TimeBasedPolicy evicts entries older than a certain age
type TimeBasedPolicy struct {
	maxAge        time.Duration
	checkInterval time.Duration
}

// NewTimeBasedPolicy creates a new time-based eviction policy
func NewTimeBasedPolicy(maxAge, checkInterval time.Duration) *TimeBasedPolicy {
	return &TimeBasedPolicy{
		maxAge:        maxAge,
		checkInterval: checkInterval,
	}
}

// ShouldEvict checks if eviction is needed based on time
func (p *TimeBasedPolicy) ShouldEvict(ctx context.Context, tenantID uuid.UUID, stats TenantStats) bool {
	// Run time-based eviction at check intervals
	return time.Since(stats.LastEviction) > p.checkInterval
}

// GetEvictionTarget returns the current count (time-based doesn't change count)
func (p *TimeBasedPolicy) GetEvictionTarget(ctx context.Context, tenantID uuid.UUID, stats TenantStats) int {
	// Time-based policy doesn't set a target count
	// It should be used with a different eviction mechanism
	return stats.EntryCount
}

// CompositePolicy combines multiple policies
type CompositePolicy struct {
	policies []EvictionPolicy
}

// NewCompositePolicy creates a policy that combines multiple policies
func NewCompositePolicy(policies ...EvictionPolicy) *CompositePolicy {
	return &CompositePolicy{
		policies: policies,
	}
}

// ShouldEvict returns true if any policy says to evict
func (p *CompositePolicy) ShouldEvict(ctx context.Context, tenantID uuid.UUID, stats TenantStats) bool {
	for _, policy := range p.policies {
		if policy.ShouldEvict(ctx, tenantID, stats) {
			return true
		}
	}
	return false
}

// GetEvictionTarget returns the most restrictive target
func (p *CompositePolicy) GetEvictionTarget(ctx context.Context, tenantID uuid.UUID, stats TenantStats) int {
	minTarget := stats.EntryCount
	for _, policy := range p.policies {
		target := policy.GetEvictionTarget(ctx, tenantID, stats)
		if target < minTarget {
			minTarget = target
		}
	}
	return minTarget
}

// TenantQuotaPolicy enforces per-tenant quotas
type TenantQuotaPolicy struct {
	quotas map[uuid.UUID]TenantQuota
	mu     sync.RWMutex
}

// TenantQuota defines quota limits for a tenant
type TenantQuota struct {
	MaxEntries int
	MaxBytes   int64
}

// NewTenantQuotaPolicy creates a new tenant quota policy
func NewTenantQuotaPolicy() *TenantQuotaPolicy {
	return &TenantQuotaPolicy{
		quotas: make(map[uuid.UUID]TenantQuota),
	}
}

// SetQuota sets the quota for a tenant
func (p *TenantQuotaPolicy) SetQuota(tenantID uuid.UUID, quota TenantQuota) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quotas[tenantID] = quota
}

// GetQuota retrieves the quota for a tenant
func (p *TenantQuotaPolicy) GetQuota(tenantID uuid.UUID) (TenantQuota, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	quota, ok := p.quotas[tenantID]
	return quota, ok
}

// ShouldEvict checks if tenant exceeds quota
func (p *TenantQuotaPolicy) ShouldEvict(ctx context.Context, tenantID uuid.UUID, stats TenantStats) bool {
	quota, ok := p.GetQuota(tenantID)
	if !ok {
		return false // No quota set, no eviction
	}
	return stats.EntryCount > quota.MaxEntries || stats.TotalBytes > quota.MaxBytes
}

// GetEvictionTarget calculates target based on quota
func (p *TenantQuotaPolicy) GetEvictionTarget(ctx context.Context, tenantID uuid.UUID, stats TenantStats) int {
	quota, ok := p.GetQuota(tenantID)
	if !ok {
		return stats.EntryCount
	}

	// Target 90% of quota
	if stats.EntryCount > quota.MaxEntries {
		return int(float64(quota.MaxEntries) * 0.9)
	}

	if stats.TotalBytes > quota.MaxBytes {
		avgSize := stats.TotalBytes / int64(stats.EntryCount)
		if avgSize == 0 {
			avgSize = 1024
		}
		bytesToRemove := stats.TotalBytes - int64(float64(quota.MaxBytes)*0.9)
		entriesToRemove := int(bytesToRemove / avgSize)
		return stats.EntryCount - entriesToRemove
	}

	return stats.EntryCount
}
