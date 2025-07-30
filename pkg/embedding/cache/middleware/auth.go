package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
)

// CacheAuthMiddleware extracts tenant ID from auth token and adds it to context
func CacheAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract tenant from existing auth
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// Try X-Tenant-ID header as fallback
			tenantIDStr := c.GetHeader("X-Tenant-ID")
			if tenantIDStr != "" {
				tenantID, err := uuid.Parse(tenantIDStr)
				if err == nil {
					ctx := auth.WithTenantID(c.Request.Context(), tenantID)
					c.Request = c.Request.WithContext(ctx)
				}
			}
			c.Next()
			return
		}

		// For now, skip token validation in cache middleware
		// The main auth middleware should have already validated and set the tenant ID
		// This is just a fallback for development/testing

		c.Next()
	}
}

// RequireTenantMiddleware ensures a tenant ID is present in the context
func RequireTenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := auth.GetTenantID(c.Request.Context())
		if tenantID == uuid.Nil {
			c.JSON(401, gin.H{
				"error": "tenant authentication required",
				"code":  "TENANT_AUTH_REQUIRED",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// TenantCacheStatsMiddleware adds tenant cache stats to response headers
func TenantCacheStatsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// After request processing, add cache stats if available
		if cacheHit, exists := c.Get("cache_hit"); exists {
			c.Header("X-Cache-Hit", formatBool(cacheHit.(bool)))
		}

		if hitRate, exists := c.Get("cache_hit_rate"); exists {
			c.Header("X-Cache-Hit-Rate", formatFloat(hitRate.(float64)))
		}

		if tenantID := auth.GetTenantID(c.Request.Context()); tenantID != uuid.Nil {
			c.Header("X-Tenant-ID", tenantID.String())
		}
	}
}

func formatBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%.2f", f)
}
