package middleware

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/auth"
)

// TenantMiddleware handles tenant context extraction and validation
type TenantMiddleware struct {
	db           *sqlx.DB
	jwtValidator *auth.JWTValidator
}

// NewTenantMiddleware creates a new tenant middleware instance
func NewTenantMiddleware(db *sqlx.DB, jwtValidator *auth.JWTValidator) *TenantMiddleware {
	return &TenantMiddleware{
		db:           db,
		jwtValidator: jwtValidator,
	}
}

// ExtractTenant validates JWT and sets tenant context
// This middleware MUST be used on all tenant-aware endpoints
func (tm *TenantMiddleware) ExtractTenant() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "missing authorization header",
			})
			c.Abort()
			return
		}

		// Extract and validate JWT claims
		claims, err := tm.jwtValidator.ValidateJWT(authHeader)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid token",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// Parse tenant ID from claims
		tenantID, err := uuid.Parse(claims.TenantID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid tenant ID format",
			})
			c.Abort()
			return
		}

		// Parse user ID from claims
		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid user ID format",
			})
			c.Abort()
			return
		}

		// Verify tenant exists and is active in database
		var active bool
		err = tm.db.Get(&active,
			"SELECT is_active FROM mcp.tenants WHERE id = $1",
			tenantID)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "tenant not found",
			})
			c.Abort()
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to verify tenant",
			})
			c.Abort()
			return
		}
		if !active {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "tenant not authorized",
			})
			c.Abort()
			return
		}

		// Set tenant context in Gin context for handlers
		c.Set("tenant_id", tenantID)
		c.Set("user_id", userID)
		c.Set("user_email", claims.Email)
		c.Set("user_roles", claims.Roles)

		// Set tenant in request context for database operations
		ctx := context.WithValue(c.Request.Context(), "tenant_id", tenantID)
		c.Request = c.Request.WithContext(ctx)

		// Set database tenant for Row Level Security
		if err := SetDatabaseTenant(tm.db, tenantID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "failed to set tenant context",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// SetDatabaseTenant sets the tenant context for database queries
// This function calls the PostgreSQL function rag.set_current_tenant()
// which enables Row Level Security policies to enforce tenant isolation
func SetDatabaseTenant(db *sqlx.DB, tenantID uuid.UUID) error {
	_, err := db.Exec("SELECT rag.set_current_tenant($1)", tenantID)
	if err != nil {
		return fmt.Errorf("failed to set database tenant: %w", err)
	}
	return nil
}
