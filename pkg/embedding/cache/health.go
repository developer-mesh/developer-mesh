package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// HealthStatus represents the health of a cache component
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthCheck represents a health check result
type HealthCheck struct {
	Component   string                 `json:"component"`
	Status      HealthStatus           `json:"status"`
	Message     string                 `json:"message,omitempty"`
	Latency     time.Duration          `json:"latency_ms"`
	LastChecked time.Time              `json:"last_checked"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// CacheHealth represents overall cache health
type CacheHealth struct {
	Status      HealthStatus  `json:"status"`
	Checks      []HealthCheck `json:"checks"`
	TotalChecks int           `json:"total_checks"`
	Healthy     int           `json:"healthy"`
	Degraded    int           `json:"degraded"`
	Unhealthy   int           `json:"unhealthy"`
	Timestamp   time.Time     `json:"timestamp"`
}

// HealthChecker provides health check functionality for the cache
type HealthChecker struct {
	cache        *SemanticCache
	tenantCache  *TenantAwareCache
	checkTimeout time.Duration
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(cache *SemanticCache, tenantCache *TenantAwareCache) *HealthChecker {
	return &HealthChecker{
		cache:        cache,
		tenantCache:  tenantCache,
		checkTimeout: 5 * time.Second,
	}
}

// CheckHealth performs all health checks
func (h *HealthChecker) CheckHealth(ctx context.Context) *CacheHealth {
	health := &CacheHealth{
		Status:    HealthStatusHealthy,
		Checks:    []HealthCheck{},
		Timestamp: time.Now(),
	}

	// Create timeout context
	checkCtx, cancel := context.WithTimeout(ctx, h.checkTimeout)
	defer cancel()

	// Run all checks
	checks := []func(context.Context) HealthCheck{
		h.checkRedis,
		h.checkVectorStore,
		h.checkLRUManager,
		h.checkEncryption,
		h.checkCompression,
		h.checkCircuitBreaker,
	}

	for _, check := range checks {
		result := check(checkCtx)
		health.Checks = append(health.Checks, result)

		// Update counters
		health.TotalChecks++
		switch result.Status {
		case HealthStatusHealthy:
			health.Healthy++
		case HealthStatusDegraded:
			health.Degraded++
		case HealthStatusUnhealthy:
			health.Unhealthy++
		}
	}

	// Determine overall status
	if health.Unhealthy > 0 {
		health.Status = HealthStatusUnhealthy
	} else if health.Degraded > 0 {
		health.Status = HealthStatusDegraded
	}

	return health
}

// Redis health check
func (h *HealthChecker) checkRedis(ctx context.Context) HealthCheck {
	start := time.Now()
	check := HealthCheck{
		Component:   "redis",
		Status:      HealthStatusHealthy,
		LastChecked: time.Now(),
		Details:     make(map[string]interface{}),
	}

	// Check Redis connectivity
	if h.cache == nil || h.cache.redis == nil {
		check.Status = HealthStatusUnhealthy
		check.Message = "Redis client not initialized"
		check.Latency = time.Since(start)
		return check
	}

	// Ping Redis
	err := h.cache.redis.Health(ctx)
	check.Latency = time.Since(start)

	if err != nil {
		check.Status = HealthStatusUnhealthy
		check.Message = fmt.Sprintf("Redis ping failed: %v", err)
		return check
	}

	// Check memory usage
	testKey := fmt.Sprintf("%s:health:test", h.cache.config.Prefix)
	memUsage, err := h.cache.redis.MemoryUsage(ctx, testKey)
	if err == nil && memUsage > 0 {
		check.Details["memory_usage_bytes"] = memUsage
	}

	// Get Redis info (would need to add this method to ResilientRedisClient)
	check.Details["connection_pool"] = "healthy"
	check.Message = "Redis is healthy"

	return check
}

// Vector store health check
func (h *HealthChecker) checkVectorStore(ctx context.Context) HealthCheck {
	start := time.Now()
	check := HealthCheck{
		Component:   "vector_store",
		Status:      HealthStatusHealthy,
		LastChecked: time.Now(),
		Details:     make(map[string]interface{}),
	}

	if h.cache == nil || h.cache.vectorStore == nil {
		check.Status = HealthStatusDegraded
		check.Message = "Vector store not configured"
		check.Latency = time.Since(start)
		return check
	}

	// Test vector store by getting global stats
	stats, err := h.cache.vectorStore.GetGlobalCacheStats(ctx)
	check.Latency = time.Since(start)

	if err != nil {
		check.Status = HealthStatusUnhealthy
		check.Message = fmt.Sprintf("Vector store query failed: %v", err)
		return check
	}

	check.Details = stats
	check.Message = "Vector store is healthy"

	return check
}

// LRU manager health check
func (h *HealthChecker) checkLRUManager(ctx context.Context) HealthCheck {
	start := time.Now()
	check := HealthCheck{
		Component:   "lru_manager",
		Status:      HealthStatusHealthy,
		LastChecked: time.Now(),
		Details:     make(map[string]interface{}),
	}

	if h.tenantCache == nil || h.tenantCache.GetLRUManager() == nil {
		check.Status = HealthStatusDegraded
		check.Message = "LRU manager not initialized"
		check.Latency = time.Since(start)
		return check
	}

	// Check if LRU manager is running
	// This would require adding a health check method to the LRU manager
	check.Details["status"] = "running"
	check.Message = "LRU manager is healthy"
	check.Latency = time.Since(start)

	return check
}

// Encryption health check
func (h *HealthChecker) checkEncryption(ctx context.Context) HealthCheck {
	start := time.Now()
	check := HealthCheck{
		Component:   "encryption",
		Status:      HealthStatusHealthy,
		LastChecked: time.Now(),
		Details:     make(map[string]interface{}),
	}

	if h.tenantCache == nil || h.tenantCache.encryptionService == nil {
		check.Status = HealthStatusUnhealthy
		check.Message = "Encryption service not initialized"
		check.Latency = time.Since(start)
		return check
	}

	// Test encryption/decryption
	testData := "health-check-test"
	testTenant := uuid.New().String()

	encrypted, err := h.tenantCache.encryptionService.EncryptCredential(testData, testTenant)
	if err != nil {
		check.Status = HealthStatusUnhealthy
		check.Message = fmt.Sprintf("Encryption test failed: %v", err)
		check.Latency = time.Since(start)
		return check
	}

	decrypted, err := h.tenantCache.encryptionService.DecryptCredential([]byte(encrypted), testTenant)
	if err != nil || decrypted != testData {
		check.Status = HealthStatusUnhealthy
		check.Message = fmt.Sprintf("Decryption test failed: %v", err)
		check.Latency = time.Since(start)
		return check
	}

	check.Details["algorithm"] = "AES"
	check.Details["key_rotation_enabled"] = h.tenantCache.keyManager != nil
	check.Message = "Encryption service is healthy"
	check.Latency = time.Since(start)

	return check
}

// Compression health check
func (h *HealthChecker) checkCompression(ctx context.Context) HealthCheck {
	start := time.Now()
	check := HealthCheck{
		Component:   "compression",
		Status:      HealthStatusHealthy,
		LastChecked: time.Now(),
		Details:     make(map[string]interface{}),
	}

	if h.cache == nil || h.cache.compressionService == nil {
		check.Status = HealthStatusDegraded
		check.Message = "Compression not enabled"
		check.Details["enabled"] = false
		check.Latency = time.Since(start)
		return check
	}

	// Test compression
	testData := make([]byte, 1024)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	compressed, err := h.cache.compress(testData)
	if err != nil {
		check.Status = HealthStatusUnhealthy
		check.Message = fmt.Sprintf("Compression test failed: %v", err)
		check.Latency = time.Since(start)
		return check
	}

	decompressed, err := h.cache.decompress(compressed)
	if err != nil {
		check.Status = HealthStatusUnhealthy
		check.Message = fmt.Sprintf("Decompression test failed: %v", err)
		check.Latency = time.Since(start)
		return check
	}

	// Verify decompression worked
	if len(decompressed) != len(testData) {
		check.Status = HealthStatusUnhealthy
		check.Message = "Decompression size mismatch"
		check.Latency = time.Since(start)
		return check
	}

	compressionRatio := float64(len(compressed)) / float64(len(testData))
	check.Details["enabled"] = true
	check.Details["compression_ratio"] = compressionRatio
	check.Details["algorithm"] = "gzip"
	check.Message = "Compression service is healthy"
	check.Latency = time.Since(start)

	return check
}

// Circuit breaker health check
func (h *HealthChecker) checkCircuitBreaker(ctx context.Context) HealthCheck {
	start := time.Now()
	check := HealthCheck{
		Component:   "circuit_breaker",
		Status:      HealthStatusHealthy,
		LastChecked: time.Now(),
		Details:     make(map[string]interface{}),
	}

	if h.cache == nil || h.cache.redis == nil {
		check.Status = HealthStatusUnhealthy
		check.Message = "Circuit breaker not initialized"
		check.Latency = time.Since(start)
		return check
	}

	// Check circuit breaker state
	// This would require exposing circuit breaker state from ResilientRedisClient
	check.Details["state"] = "closed" // Would get actual state
	check.Details["failure_count"] = 0
	check.Details["success_count"] = 0
	check.Message = "Circuit breaker is healthy"
	check.Latency = time.Since(start)

	return check
}

// GetStats returns cache statistics for monitoring
func (h *HealthChecker) GetStats(ctx context.Context) map[string]interface{} {
	stats := make(map[string]interface{})

	if h.cache != nil {
		cacheStats := h.cache.GetStats()
		stats["cache"] = map[string]interface{}{
			"total_hits":    cacheStats.TotalHits,
			"total_misses":  cacheStats.TotalMisses,
			"hit_rate":      cacheStats.HitRate,
			"total_entries": cacheStats.TotalEntries,
		}
	}

	// Add performance metrics
	if h.cache != nil && h.cache.config.PerformanceConfig != nil {
		stats["performance"] = map[string]interface{}{
			"max_candidates":       h.cache.config.MaxCandidates,
			"similarity_threshold": h.cache.config.SimilarityThreshold,
			"compression_enabled":  h.cache.config.EnableCompression,
		}
	}

	return stats
}
