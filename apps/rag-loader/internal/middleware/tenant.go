package middleware

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/auth"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const tenantIDKey contextKey = "tenant_id"

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

// ExtractTenant validates JWT or API key and sets tenant context
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

		// Try API key authentication first (simpler)
		if tenantID, userID, err := tm.validateAPIKey(authHeader); err == nil {
			tm.setTenantContext(c, tenantID, userID)
			return
		}

		// Fall back to JWT authentication
		claims, err := tm.jwtValidator.ValidateJWT(authHeader)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid token or API key",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// Parse tenant ID from JWT claims
		tenantID, err := uuid.Parse(claims.TenantID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid tenant ID format",
			})
			c.Abort()
			return
		}

		// Parse user ID from JWT claims
		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid user ID format",
			})
			c.Abort()
			return
		}

		// Use common method to set tenant context
		tm.setTenantContext(c, tenantID, userID)
	}
}

// validateAPIKey validates an API key and returns tenant_id and user_id
func (tm *TenantMiddleware) validateAPIKey(authHeader string) (uuid.UUID, uuid.UUID, error) {
	// Extract API key from "Bearer <key>" format
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid authorization header format")
	}
	apiKey := parts[1]

	// Hash the API key to look up in database
	hasher := sha256.New()
	hasher.Write([]byte(apiKey))
	keyHash := hex.EncodeToString(hasher.Sum(nil))

	// Query database for API key
	var result struct {
		TenantID uuid.UUID `db:"tenant_id"`
		UserID   uuid.UUID `db:"user_id"`
		IsActive bool      `db:"is_active"`
	}

	err := tm.db.Get(&result, `
		SELECT tenant_id, user_id, is_active
		FROM mcp.api_keys
		WHERE key_hash = $1
	`, keyHash)

	if err == sql.ErrNoRows {
		return uuid.Nil, uuid.Nil, fmt.Errorf("API key not found")
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("database error: %w", err)
	}

	if !result.IsActive {
		return uuid.Nil, uuid.Nil, fmt.Errorf("API key is inactive")
	}

	// Update last_used_at and usage_count
	_, _ = tm.db.Exec(`
		UPDATE mcp.api_keys
		SET last_used_at = NOW(), usage_count = usage_count + 1
		WHERE key_hash = $1
	`, keyHash)

	return result.TenantID, result.UserID, nil
}

// setTenantContext sets the tenant context for the request
func (tm *TenantMiddleware) setTenantContext(c *gin.Context, tenantID, userID uuid.UUID) {
	// Check if tenant is active
	var isActive bool
	err := tm.db.Get(&isActive, `
		SELECT is_active
		FROM mcp.tenants
		WHERE id = $1
	`, tenantID)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "tenant not found",
		})
		c.Abort()
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to check tenant status",
			"details": err.Error(),
		})
		c.Abort()
		return
	}

	if !isActive {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "tenant is inactive",
		})
		c.Abort()
		return
	}

	// Set tenant context in Gin context for handlers
	c.Set("tenant_id", tenantID)
	c.Set("user_id", userID)

	// Set tenant in request context for database operations
	ctx := context.WithValue(c.Request.Context(), tenantIDKey, tenantID)
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
