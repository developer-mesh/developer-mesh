package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// contextKey is a type for context keys to avoid collisions
type contextKey string

const (
	// UserContextKey is the key for storing user in context
	UserContextKey contextKey = "auth_user"
)

// GinMiddleware creates a Gin middleware for authentication
func (s *Service) GinMiddleware(authTypes ...Type) gin.HandlerFunc {
	// Default to API key auth if no types specified
	if len(authTypes) == 0 {
		authTypes = []Type{TypeAPIKey}
	}

	return func(c *gin.Context) {
		var user *User
		var err error

		// Try each auth type in order
		for _, authType := range authTypes {
			switch authType {
			case TypeAPIKey:
				// Check Authorization header first
				authHeader := c.GetHeader("Authorization")
				if authHeader != "" {
					// Handle "Bearer <token>" format
					if strings.HasPrefix(authHeader, "Bearer ") {
						apiKey := strings.TrimPrefix(authHeader, "Bearer ")
						user, err = s.ValidateAPIKey(c.Request.Context(), apiKey)
					} else {
						// Direct API key
						user, err = s.ValidateAPIKey(c.Request.Context(), authHeader)
					}
				}

				// Check custom API key header if Authorization header didn't work
				if user == nil && s.config.APIKeyHeader != "" {
					apiKey := c.GetHeader(s.config.APIKeyHeader)
					if apiKey != "" {
						user, err = s.ValidateAPIKey(c.Request.Context(), apiKey)
					}
				}

			case TypeJWT:
				authHeader := c.GetHeader("Authorization")
				if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
					token := strings.TrimPrefix(authHeader, "Bearer ")
					user, err = s.ValidateJWT(c.Request.Context(), token)
				}

			case TypeNone:
				// No authentication required
				c.Next()
				return
			}

			// If we found a valid user, break out of the loop
			if user != nil {
				break
			}
		}

		// If no valid authentication found
		if user == nil {
			s.logger.Warn("Authentication failed", map[string]interface{}{
				"error": err,
				"ip":    c.ClientIP(),
				"path":  c.Request.URL.Path,
			})

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			c.Abort()
			return
		}

		// Store user in context
		c.Set(string(UserContextKey), user)

		// For compatibility with existing code that expects "user" key
		c.Set("user", map[string]interface{}{
			"id":        user.ID,
			"tenant_id": user.TenantID,
			"email":     user.Email,
			"scopes":    user.Scopes,
			"auth_type": string(user.AuthType),
		})

		// Also set individual keys for backward compatibility
		c.Set("user_id", user.ID)
		c.Set("tenant_id", user.TenantID)

		// Log successful authentication
		s.logger.Debug("Authentication successful", map[string]interface{}{
			"user_id":   user.ID,
			"tenant_id": user.TenantID,
			"auth_type": string(user.AuthType),
			"path":      c.Request.URL.Path,
		})

		c.Next()
	}
}

// RequireScopes creates a middleware that checks for required scopes
func (s *Service) RequireScopes(scopes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user from context
		userInterface, exists := c.Get(string(UserContextKey))
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			c.Abort()
			return
		}

		user, ok := userInterface.(*User)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid user context",
			})
			c.Abort()
			return
		}

		// Check scopes
		if err := s.AuthorizeScopes(user, scopes); err != nil {
			s.logger.Warn("Authorization failed", map[string]interface{}{
				"user_id":         user.ID,
				"required_scopes": scopes,
				"user_scopes":     user.Scopes,
				"error":           err,
			})

			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient permissions",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUserFromContext extracts the authenticated user from the Gin context
func GetUserFromContext(c *gin.Context) (*User, bool) {
	userInterface, exists := c.Get(string(UserContextKey))
	if !exists {
		return nil, false
	}

	user, ok := userInterface.(*User)
	return user, ok
}

// GetTenantFromContext extracts the tenant ID from the Gin context
func GetTenantFromContext(c *gin.Context) (string, bool) {
	// Try to get from user first
	if user, ok := GetUserFromContext(c); ok {
		return user.TenantID.String(), true
	}

	// Fall back to direct tenant_id key
	if tenantID, exists := c.Get("tenant_id"); exists {
		switch v := tenantID.(type) {
		case uuid.UUID:
			return v.String(), true
		case string:
			return v, true
		}
	}

	// Check X-Tenant-ID header as last resort
	if tid := c.GetHeader("X-Tenant-ID"); tid != "" {
		return tid, true
	}

	return "", false
}

// StandardMiddleware returns standard HTTP middleware for authentication
func (s *Service) StandardMiddleware(authTypes ...Type) func(http.Handler) http.Handler {
	// Default to API key auth if no types specified
	if len(authTypes) == 0 {
		authTypes = []Type{TypeAPIKey}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var user *User
			var err error

			// Try each auth type in order
			for _, authType := range authTypes {
				switch authType {
				case TypeAPIKey:
					// Check Authorization header first
					authHeader := r.Header.Get("Authorization")
					if authHeader != "" {
						// Handle "Bearer <token>" format
						if strings.HasPrefix(authHeader, "Bearer ") {
							apiKey := strings.TrimPrefix(authHeader, "Bearer ")
							user, err = s.ValidateAPIKey(r.Context(), apiKey)
						} else {
							// Direct API key
							user, err = s.ValidateAPIKey(r.Context(), authHeader)
						}
					}

					// Check custom API key header
					if user == nil && s.config.APIKeyHeader != "" {
						apiKey := r.Header.Get(s.config.APIKeyHeader)
						if apiKey != "" {
							user, err = s.ValidateAPIKey(r.Context(), apiKey)
						}
					}

				case TypeJWT:
					authHeader := r.Header.Get("Authorization")
					if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
						token := strings.TrimPrefix(authHeader, "Bearer ")
						user, err = s.ValidateJWT(r.Context(), token)
					}

				case TypeNone:
					// No authentication required
					next.ServeHTTP(w, r)
					return
				}

				// If we found a valid user, break
				if user != nil {
					break
				}
			}

			// If no valid authentication found
			if user == nil {
				s.logger.Warn("Authentication failed", map[string]interface{}{
					"error": err,
					"ip":    r.RemoteAddr,
					"path":  r.URL.Path,
				})

				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			// Store user in request context
			ctx := context.WithValue(r.Context(), UserContextKey, user)

			// Continue with authenticated request
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserFromRequest extracts the authenticated user from the request context
func GetUserFromRequest(r *http.Request) (*User, bool) {
	user, ok := r.Context().Value(UserContextKey).(*User)
	return user, ok
}
