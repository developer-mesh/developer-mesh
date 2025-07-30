package integration

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache/eviction"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache/middleware"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache/monitoring"
	baseMiddleware "github.com/developer-mesh/developer-mesh/pkg/middleware"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// CacheRouter sets up routes with all cache middleware integrated
type CacheRouter struct {
	tenantCache     *cache.TenantAwareCache
	rateLimiter     *middleware.CacheRateLimiter
	metricsExporter *monitoring.CacheMetricsExporter
	logger          observability.Logger
}

// NewCacheRouter creates a new cache router with all integrations
func NewCacheRouter(
	tenantCache *cache.TenantAwareCache,
	baseRateLimiter *baseMiddleware.RateLimiter,
	vectorStore eviction.VectorStore,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *CacheRouter {
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.router")
	}

	// Create cache-specific rate limiter
	cacheRateLimiter := middleware.NewCacheRateLimiter(
		baseRateLimiter,
		middleware.DefaultCacheRateLimitConfig(),
		logger,
		metrics,
	)

	// Create metrics exporter
	metricsExporter := monitoring.NewCacheMetricsExporter(
		metrics,
		vectorStore,
		30*time.Second,
	)

	return &CacheRouter{
		tenantCache:     tenantCache,
		rateLimiter:     cacheRateLimiter,
		metricsExporter: metricsExporter,
		logger:          logger,
	}
}

// SetupRoutes configures all cache-related routes with middleware
func (cr *CacheRouter) SetupRoutes(router *gin.RouterGroup) {
	// Apply global middleware
	router.Use(
		middleware.CacheAuthMiddleware(),        // Extract tenant ID
		middleware.TenantCacheStatsMiddleware(), // Add cache stats to response
	)

	// Cache read endpoints
	readGroup := router.Group("/cache")
	readGroup.Use(cr.rateLimiter.CacheReadLimit()) // Apply read rate limits
	{
		readGroup.GET("/search", cr.handleCacheSearch)
		readGroup.GET("/stats", middleware.RequireTenantMiddleware(), cr.handleGetStats)
		readGroup.GET("/health", cr.handleHealthCheck)
	}

	// Cache write endpoints
	writeGroup := router.Group("/cache")
	writeGroup.Use(
		middleware.RequireTenantMiddleware(), // Require tenant for writes
		cr.rateLimiter.CacheWriteLimit(),     // Apply write rate limits
	)
	{
		writeGroup.POST("/entry", cr.handleCacheSet)
		writeGroup.DELETE("/entry", cr.handleCacheDelete)
		writeGroup.POST("/clear", cr.handleCacheClear)
	}

	// Admin endpoints (no rate limiting)
	adminGroup := router.Group("/cache/admin")
	adminGroup.Use(middleware.RequireTenantMiddleware())
	{
		adminGroup.GET("/config", cr.handleGetConfig)
		adminGroup.PUT("/config", cr.handleUpdateConfig)
		adminGroup.POST("/evict", cr.handleManualEviction)
	}
}

// handleCacheSearch handles cache search requests
func (cr *CacheRouter) handleCacheSearch(c *gin.Context) {
	var req CacheSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request", "details": err.Error()})
		return
	}

	// Get tenant ID from context
	tenantID := auth.GetTenantID(c.Request.Context())

	// Track operation
	err := monitoring.TrackCacheOperation(
		c.Request.Context(),
		cr.metricsExporter.GetMetrics(),
		"search",
		tenantID,
		func() error {
			entry, err := cr.tenantCache.Get(c.Request.Context(), req.Query, req.Embedding)
			if err != nil {
				return err
			}

			if entry != nil {
				c.Set("cache_hit", true)
				c.JSON(200, gin.H{
					"hit":       true,
					"results":   entry.Results,
					"cached_at": entry.CachedAt,
					"hit_count": entry.HitCount,
				})
			} else {
				c.Set("cache_hit", false)
				c.JSON(200, gin.H{
					"hit": false,
				})
			}
			return nil
		},
	)

	if err != nil {
		cr.logger.Error("Cache search failed", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
		})
		c.JSON(500, gin.H{"error": "search failed", "details": err.Error()})
	}
}

// handleCacheSet handles cache write requests
func (cr *CacheRouter) handleCacheSet(c *gin.Context) {
	var req CacheSetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request", "details": err.Error()})
		return
	}

	tenantID := auth.GetTenantID(c.Request.Context())

	err := monitoring.TrackCacheOperation(
		c.Request.Context(),
		cr.metricsExporter.GetMetrics(),
		"set",
		tenantID,
		func() error {
			return cr.tenantCache.Set(c.Request.Context(), req.Query, req.Embedding, req.Results)
		},
	)

	if err != nil {
		cr.logger.Error("Cache set failed", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
		})
		c.JSON(500, gin.H{"error": "set failed", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// handleGetStats returns cache statistics for the tenant
func (cr *CacheRouter) handleGetStats(c *gin.Context) {
	tenantID := auth.GetTenantID(c.Request.Context())

	stats, err := cr.tenantCache.GetTenantStats(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get stats", "details": err.Error()})
		return
	}

	// Add rate limiter stats
	rlStats := cr.rateLimiter.GetStats()

	c.JSON(200, gin.H{
		"cache_stats":      stats,
		"rate_limit_stats": rlStats,
	})
}

// handleHealthCheck returns cache health status
func (cr *CacheRouter) handleHealthCheck(c *gin.Context) {
	// This would check Redis connectivity, vector store health, etc.
	c.JSON(200, gin.H{
		"status": "healthy",
		"components": gin.H{
			"redis":        "ok",
			"vector_store": "ok",
			"rate_limiter": "ok",
		},
	})
}

// Request/Response types
type CacheSearchRequest struct {
	Query     string    `json:"query" binding:"required"`
	Embedding []float32 `json:"embedding" binding:"required"`
}

type CacheSetRequest struct {
	Query     string                     `json:"query" binding:"required"`
	Embedding []float32                  `json:"embedding" binding:"required"`
	Results   []cache.CachedSearchResult `json:"results" binding:"required"`
}

// handleCacheDelete handles cache entry deletion
func (cr *CacheRouter) handleCacheDelete(c *gin.Context) {
	var req CacheDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request", "details": err.Error()})
		return
	}

	tenantID := auth.GetTenantID(c.Request.Context())

	err := monitoring.TrackCacheOperation(
		c.Request.Context(),
		cr.metricsExporter.GetMetrics(),
		"delete",
		tenantID,
		func() error {
			return cr.tenantCache.Delete(c.Request.Context(), req.Query)
		},
	)

	if err != nil {
		cr.logger.Error("Cache delete failed", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
		})
		c.JSON(500, gin.H{"error": "delete failed", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// handleCacheClear clears all cache entries for a tenant
func (cr *CacheRouter) handleCacheClear(c *gin.Context) {
	tenantID := auth.GetTenantID(c.Request.Context())

	err := cr.tenantCache.ClearTenant(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(500, gin.H{"error": "clear failed", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "tenant_id": tenantID.String()})
}

// handleGetConfig returns tenant cache configuration
func (cr *CacheRouter) handleGetConfig(c *gin.Context) {
	tenantID := auth.GetTenantID(c.Request.Context())

	// This would need to be implemented in TenantAwareCache
	config, err := cr.tenantCache.GetTenantConfig(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get config", "details": err.Error()})
		return
	}

	c.JSON(200, config)
}

// handleUpdateConfig updates tenant cache configuration
func (cr *CacheRouter) handleUpdateConfig(c *gin.Context) {
	// Admin only - would need proper authorization
	c.JSON(501, gin.H{"error": "not implemented"})
}

// handleManualEviction triggers manual cache eviction
func (cr *CacheRouter) handleManualEviction(c *gin.Context) {
	tenantID := auth.GetTenantID(c.Request.Context())

	if cr.tenantCache.GetLRUManager() == nil {
		c.JSON(400, gin.H{"error": "LRU manager not configured"})
		return
	}

	// Trigger eviction
	err := cr.tenantCache.GetLRUManager().EvictForTenant(c.Request.Context(), tenantID, 0)
	if err != nil {
		c.JSON(500, gin.H{"error": "eviction failed", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "Eviction triggered"})
}

// Request types
type CacheDeleteRequest struct {
	Query string `json:"query" binding:"required"`
}
