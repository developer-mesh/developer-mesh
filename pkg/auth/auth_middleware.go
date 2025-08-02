package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// ContextKeyIPAddress is the context key for IP address
	ContextKeyIPAddress contextKey = "ip_address"
	// ContextKeyUserAgent is the context key for user agent
	ContextKeyUserAgent contextKey = "user_agent"
)

// AuthMiddleware wraps the auth service with production features
type AuthMiddleware struct {
	service     *Service
	rateLimiter *RateLimiter
	metrics     *MetricsCollector
	audit       *AuditLogger
}

// NewAuthMiddleware creates middleware with rate limiting and metrics
func NewAuthMiddleware(service *Service, rateLimiter *RateLimiter, metrics *MetricsCollector, audit *AuditLogger) *AuthMiddleware {
	return &AuthMiddleware{
		service:     service,
		rateLimiter: rateLimiter,
		metrics:     metrics,
		audit:       audit,
	}
}

// ValidateAPIKeyWithMetrics validates an API key with rate limiting and metrics
func (m *AuthMiddleware) ValidateAPIKeyWithMetrics(ctx context.Context, apiKey string) (*User, error) {
	start := time.Now()

	// Extract IP address from context
	ipAddress, _ := ctx.Value(ContextKeyIPAddress).(string)
	if ipAddress == "" {
		ipAddress = "unknown"
	}

	// Create identifier for rate limiting
	identifier := "apikey:" + truncateKey(apiKey, 8)

	// Check rate limit first
	if err := m.rateLimiter.CheckLimit(ctx, identifier); err != nil {
		m.metrics.RecordRateLimitExceeded(ctx, identifier)
		m.audit.LogRateLimitExceeded(ctx, identifier, ipAddress)
		return nil, err
	}

	// Call base implementation
	user, err := m.service.ValidateAPIKey(ctx, apiKey)

	// Record metrics
	duration := time.Since(start)
	success := err == nil
	m.metrics.RecordAuthAttempt(ctx, "api_key", success, duration)

	// Record attempt for rate limiting
	m.rateLimiter.RecordAttempt(ctx, identifier, success)

	// Audit log
	auditEvent := AuditEvent{
		AuthType:  "api_key",
		Success:   success,
		IPAddress: ipAddress,
	}
	if user != nil {
		auditEvent.UserID = user.ID.String()
		auditEvent.TenantID = user.TenantID.String()
	}
	if err != nil {
		auditEvent.Error = err.Error()
	}
	m.audit.LogAuthAttempt(ctx, auditEvent)

	return user, err
}

// ValidateJWTWithMetrics validates a JWT with rate limiting and metrics
func (m *AuthMiddleware) ValidateJWTWithMetrics(ctx context.Context, tokenString string) (*User, error) {
	start := time.Now()

	// Extract IP address from context
	ipAddress, _ := ctx.Value(ContextKeyIPAddress).(string)
	if ipAddress == "" {
		ipAddress = "unknown"
	}

	// For JWT, use IP address for rate limiting
	identifier := "jwt:" + ipAddress

	// Check rate limit first
	if err := m.rateLimiter.CheckLimit(ctx, identifier); err != nil {
		m.metrics.RecordRateLimitExceeded(ctx, identifier)
		m.audit.LogRateLimitExceeded(ctx, identifier, ipAddress)
		return nil, err
	}

	// Call base implementation
	user, err := m.service.ValidateJWT(ctx, tokenString)

	// Record metrics
	duration := time.Since(start)
	success := err == nil
	m.metrics.RecordAuthAttempt(ctx, "jwt", success, duration)

	// Record attempt for rate limiting
	m.rateLimiter.RecordAttempt(ctx, identifier, success)

	// Audit log
	auditEvent := AuditEvent{
		AuthType:  "jwt",
		Success:   success,
		IPAddress: ipAddress,
	}
	if user != nil {
		auditEvent.UserID = user.ID.String()
		auditEvent.TenantID = user.TenantID.String()
	}
	if err != nil {
		auditEvent.Error = err.Error()
	}
	m.audit.LogAuthAttempt(ctx, auditEvent)

	return user, err
}

// GinMiddleware returns a Gin middleware that uses the enhanced auth service
func (m *AuthMiddleware) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First apply rate limiting for auth endpoints
		if isAuthEndpoint(c.Request.URL.Path) {
			identifier := getIdentifier(c.Request)
			if err := m.rateLimiter.CheckLimit(c.Request.Context(), identifier); err != nil {
				m.service.logger.Warn("Rate limit exceeded", map[string]interface{}{
					"identifier": identifier,
					"path":       c.Request.URL.Path,
					"error":      err.Error(),
				})
				c.Header("X-RateLimit-Remaining", "0")
				c.Header("Retry-After", fmt.Sprintf("%.0f", m.rateLimiter.GetLockoutPeriod().Seconds()))
				c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many authentication attempts"})
				c.Abort()
				return
			}
		}

		// Perform authentication
		var user *User
		var err error

		// Try API key first
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey := strings.TrimPrefix(authHeader, "Bearer ")
				user, err = m.service.ValidateAPIKey(c.Request.Context(), apiKey)
			} else {
				user, err = m.service.ValidateAPIKey(c.Request.Context(), authHeader)
			}
		}

		// Try custom API key header if Authorization header didn't work
		if user == nil && m.service.config.APIKeyHeader != "" {
			apiKey := c.GetHeader(m.service.config.APIKeyHeader)
			if apiKey != "" {
				// Add context with enhanced values
				ctx := context.WithValue(c.Request.Context(), ContextKeyIPAddress, c.ClientIP())
				ctx = context.WithValue(ctx, ContextKeyUserAgent, c.Request.UserAgent())
				user, err = m.ValidateAPIKeyWithMetrics(ctx, apiKey)
			}
		}

		// Try JWT if API key failed
		if user == nil && authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			user, err = m.service.ValidateJWT(c.Request.Context(), token)
		}

		// Handle authentication failure
		if user == nil {
			m.service.logger.Warn("Authentication failed", map[string]interface{}{
				"error": err,
				"ip":    c.ClientIP(),
				"path":  c.Request.URL.Path,
			})

			// Record failed attempt for rate limiting on auth endpoints
			if isAuthEndpoint(c.Request.URL.Path) {
				identifier := getIdentifier(c.Request)
				m.rateLimiter.RecordAttempt(c.Request.Context(), identifier, false)
			}

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			c.Abort()
			return
		}

		// Store user in context
		c.Set(string(UserContextKey), user)

		// For compatibility with existing code
		c.Set("user", map[string]interface{}{
			"id":        user.ID,
			"tenant_id": user.TenantID,
			"email":     user.Email,
			"scopes":    user.Scopes,
			"auth_type": string(user.AuthType),
		})

		c.Set("user_id", user.ID)
		c.Set("tenant_id", user.TenantID)

		c.Next()

		// Record attempt if auth endpoint and response was written
		if isAuthEndpoint(c.Request.URL.Path) && c.Writer.Written() {
			identifier := getIdentifier(c.Request)
			success := c.Writer.Status() < 400
			m.rateLimiter.RecordAttempt(c.Request.Context(), identifier, success)
		}
	}
}

// HTTPAuthMiddleware creates HTTP middleware that adds IP to context
func HTTPAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract IP address
			ip := r.RemoteAddr
			if forwardedIP := r.Header.Get("X-Forwarded-For"); forwardedIP != "" {
				ips := strings.Split(forwardedIP, ",")
				if len(ips) > 0 {
					ip = strings.TrimSpace(ips[0])
				}
			}

			// Add to context
			ctx := context.WithValue(r.Context(), ContextKeyIPAddress, ip)
			ctx = context.WithValue(ctx, ContextKeyUserAgent, r.Header.Get("User-Agent"))

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Helper function to safely truncate keys
func truncateKey(key string, length int) string {
	if len(key) <= length {
		return key
	}
	return key[:length]
}

// isAuthEndpoint checks if path is an auth endpoint for rate limiting
func isAuthEndpoint(path string) bool {
	authPaths := []string{"/auth/", "/login", "/token", "/oauth/"}
	for _, authPath := range authPaths {
		if strings.Contains(path, authPath) {
			return true
		}
	}
	return false
}

// getIdentifier extracts client identifier for rate limiting
func getIdentifier(req *http.Request) string {
	// Use IP address as identifier
	ip := req.RemoteAddr
	if forwardedIP := req.Header.Get("X-Forwarded-For"); forwardedIP != "" {
		ips := strings.Split(forwardedIP, ",")
		if len(ips) > 0 {
			ip = strings.TrimSpace(ips[0])
		}
	}
	return "ip:" + ip
}
