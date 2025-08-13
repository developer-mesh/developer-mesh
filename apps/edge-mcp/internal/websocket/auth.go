package websocket

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	ws "github.com/developer-mesh/developer-mesh/pkg/models/websocket"
)

// AuthenticatedMessage adds authentication to messages
type AuthenticatedMessage struct {
	Message   *ws.Message
	Signature string    // HMAC signature
	Timestamp time.Time // Anti-replay
}

// SessionKey is used for HMAC signatures per connection
type SessionKey struct {
	Key       []byte
	CreatedAt time.Time
	ExpiresAt time.Time
}

// SessionManager manages session keys for connections
type SessionManager struct {
	keys map[string]*SessionKey
	mu   sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		keys: make(map[string]*SessionKey),
	}
}

// GenerateSessionKey generates a new session key for a connection
func (sm *SessionManager) GenerateSessionKey(connectionID string) (*SessionKey, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Generate random key
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(time.Now().UnixNano() & 0xFF)
	}

	sessionKey := &SessionKey{
		Key:       key,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	sm.keys[connectionID] = sessionKey
	return sessionKey, nil
}

// GetSessionKey retrieves a session key for a connection
func (sm *SessionManager) GetSessionKey(connectionID string) (*SessionKey, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	key, ok := sm.keys[connectionID]
	if !ok {
		return nil, false
	}

	// Check expiration
	if time.Now().After(key.ExpiresAt) {
		return nil, false
	}

	return key, true
}

// RemoveSessionKey removes a session key
func (sm *SessionManager) RemoveSessionKey(connectionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.keys, connectionID)
}

// ValidateConnection performs initial authentication
func (s *Server) ValidateConnection(token string) (*auth.Claims, error) {
	// Check if auth service is available
	if s.auth == nil {
		// Fall back to local JWT validation if no auth service
		claims, err := s.ValidateJWT(token)
		if err == nil {
			return claims, nil
		}

		// Try local API key validation
		if s.ValidateAPIKey(token) {
			// Generate a unique UUID for this API key session
			userID := uuid.New().String()

			// Create claims for API key auth
			return &auth.Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Subject: userID,
				},
				TenantID: "default", // API keys could be mapped to tenants
				UserID:   userID,
				Scopes:   []string{"api:access"}, // Default scopes for API keys
			}, nil
		}

		return nil, errors.New("authentication failed")
	}

	// Use auth service for validation
	// Try to validate as JWT first
	if strings.Count(token, ".") == 2 { // Looks like a JWT
		user, err := s.auth.ValidateJWT(context.Background(), token)
		if err == nil {
			// Convert User to Claims
			return &auth.Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Subject: user.ID.String(),
				},
				TenantID: user.TenantID.String(),
				UserID:   user.ID.String(),
				Scopes:   user.Scopes,
			}, nil
		}
	}

	// Try as API key
	user, err := s.auth.ValidateAPIKey(context.Background(), token)
	if err == nil {
		// Convert User to Claims
		return &auth.Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject: user.ID.String(),
			},
			TenantID: user.TenantID.String(),
			UserID:   user.ID.String(),
			Scopes:   user.Scopes,
		}, nil
	}

	return nil, errors.New("authentication failed")
}

// ValidateJWT validates a JWT token
func (s *Server) ValidateJWT(tokenString string) (*auth.Claims, error) {
	// Parse and validate the token
	token, err := jwt.ParseWithClaims(tokenString, &auth.Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.getSigningKey(), nil
	})

	if err != nil {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(*auth.Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	// Check expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("token expired")
	}

	// Validate standard claims
	if claims.UserID == "" {
		return nil, errors.New("missing required user ID claim")
	}
	// Note: TenantID can be empty here as it may come from X-Tenant-ID header

	s.logger.Debug("JWT validated successfully", map[string]interface{}{
		"user_id":   claims.UserID,
		"tenant_id": claims.TenantID,
		"scopes":    claims.Scopes,
	})

	return claims, nil
}

// ValidateAPIKey validates an API key
func (s *Server) ValidateAPIKey(key string) bool {
	if len(s.config.Security.APIKeys) == 0 {
		return false
	}

	// Check if the key exists in our allowed list
	for _, validKey := range s.config.Security.APIKeys {
		if validKey == key {
			s.logger.Debug("API key validated successfully", map[string]interface{}{})
			return true
		}
	}

	return false
}

// CheckIPWhitelist validates IP address against whitelist
func (s *Server) CheckIPWhitelist(ip string) bool {
	// If no whitelist configured, allow all
	if len(s.config.Security.IPWhitelist) == 0 {
		return true
	}

	// Check if IP is in whitelist
	for _, allowedIP := range s.config.Security.IPWhitelist {
		if allowedIP == ip {
			return true
		}
	}

	s.logger.Warn("IP not in whitelist", map[string]interface{}{
		"ip": ip,
	})

	return false
}

// EnhancedAuth performs comprehensive authentication checks
func (s *Server) EnhancedAuth(ctx context.Context, token string, clientIP string) (*auth.Claims, error) {
	// Check IP whitelist first
	if !s.CheckIPWhitelist(clientIP) {
		s.metricsCollector.RecordError("auth_ip_blocked")
		return nil, errors.New("IP address not allowed")
	}

	// Check IP rate limiting
	if s.ipRateLimiter != nil && !s.ipRateLimiter.Allow(clientIP) {
		s.metricsCollector.RecordError("auth_rate_limit_ip")
		return nil, errors.New("rate limit exceeded for IP")
	}

	// Validate authentication token
	claims, err := s.ValidateConnection(token)
	if err != nil {
		s.metricsCollector.RecordError("auth_failed")
		return nil, err
	}

	// Check user/tenant rate limiting
	userKey := fmt.Sprintf("user:%s", claims.UserID)
	tenantKey := fmt.Sprintf("tenant:%s", claims.TenantID)

	// User rate limiting
	if s.ipRateLimiter != nil && !s.ipRateLimiter.Allow(userKey) {
		s.metricsCollector.RecordError("auth_rate_limit_user")
		return nil, errors.New("rate limit exceeded for user")
	}

	// Tenant rate limiting
	if s.ipRateLimiter != nil && !s.ipRateLimiter.Allow(tenantKey) {
		s.metricsCollector.RecordError("auth_rate_limit_tenant")
		return nil, errors.New("rate limit exceeded for tenant")
	}

	// Check role-based connection limits
	if err := s.checkConnectionLimits(claims); err != nil {
		return nil, err
	}

	// Audit log authentication event
	s.logger.Info("Authentication successful", map[string]interface{}{
		"user_id":   claims.UserID,
		"tenant_id": claims.TenantID,
		"client_ip": clientIP,
		"auth_type": "enhanced",
		"timestamp": time.Now().UTC(),
	})

	// Record success metrics
	if s.metrics != nil {
		s.metrics.IncrementCounterWithLabels("auth_success", 1, map[string]string{
			"tenant_id": claims.TenantID,
			"auth_type": "enhanced",
		})
	}

	return claims, nil
}

// checkConnectionLimits verifies role-based connection limits
func (s *Server) checkConnectionLimits(claims *auth.Claims) error {
	// Count current connections for this user
	userConnections := 0
	tenantConnections := 0

	s.mu.RLock()
	for _, conn := range s.connections {
		if conn.state != nil && conn.state.Claims != nil {
			if conn.state.Claims.UserID == claims.UserID {
				userConnections++
			}
			if conn.state.Claims.TenantID == claims.TenantID {
				tenantConnections++
			}
		}
	}
	s.mu.RUnlock()

	// Define role-based limits
	userLimit := 5    // Default user limit
	tenantLimit := 50 // Default tenant limit

	// Check for admin role
	for _, scope := range claims.Scopes {
		if scope == "admin" {
			userLimit = 20    // Higher limit for admins
			tenantLimit = 200 // Higher tenant limit
			break
		}
	}

	// Check user limit
	if userConnections >= userLimit {
		s.metricsCollector.RecordError("auth_user_connection_limit")
		return fmt.Errorf("user connection limit exceeded (%d/%d)", userConnections, userLimit)
	}

	// Check tenant limit
	if tenantConnections >= tenantLimit {
		s.metricsCollector.RecordError("auth_tenant_connection_limit")
		return fmt.Errorf("tenant connection limit exceeded (%d/%d)", tenantConnections, tenantLimit)
	}

	return nil
}

// AuditLog records authentication events
type AuthAuditLog struct {
	Timestamp   time.Time              `json:"timestamp"`
	EventType   string                 `json:"event_type"`
	UserID      string                 `json:"user_id"`
	TenantID    string                 `json:"tenant_id"`
	ClientIP    string                 `json:"client_ip"`
	Success     bool                   `json:"success"`
	ErrorReason string                 `json:"error_reason,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// LogAuthEvent logs authentication events for audit trail
func (s *Server) LogAuthEvent(event AuthAuditLog) {
	// Convert to structured log
	s.logger.Info("Authentication audit event", map[string]interface{}{
		"event":     event.EventType,
		"user_id":   event.UserID,
		"tenant_id": event.TenantID,
		"client_ip": event.ClientIP,
		"success":   event.Success,
		"error":     event.ErrorReason,
		"metadata":  event.Metadata,
		"timestamp": event.Timestamp,
	})

	// Record metrics
	successStr := "false"
	if event.Success {
		successStr = "true"
	}

	if s.metrics != nil {
		s.metrics.IncrementCounterWithLabels("auth_audit_events", 1, map[string]string{
			"event_type": event.EventType,
			"success":    successStr,
		})
	}
}

// getSigningKey retrieves the JWT signing key
func (s *Server) getSigningKey() []byte {
	// In production, this would come from secure configuration
	// Check for configured JWT secret
	if s.config.Security.JWTSecret != "" {
		return []byte(s.config.Security.JWTSecret)
	}

	// Fallback to environment variable or default
	// This should be properly configured in production
	return []byte("developer-mesh-jwt-secret-key")
}

// SignMessage creates HMAC signature for a message
func (c *Connection) SignMessage(msg []byte) string {
	sessionKey, ok := c.hub.sessionManager.GetSessionKey(c.ID)
	if !ok {
		return ""
	}

	h := hmac.New(sha256.New, sessionKey.Key)
	h.Write(msg)
	h.Write([]byte(c.ID))
	if _, err := fmt.Fprintf(h, "%d", time.Now().Unix()); err != nil {
		// Hash write error is unlikely but log it
		c.hub.logger.Debug("Error writing to hash", map[string]interface{}{
			"error": err.Error(),
		})
	}

	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// VerifyMessage validates HMAC signature
func (c *Connection) VerifyMessage(msg []byte, signature string) error {
	expected := c.SignMessage(msg)

	// Constant-time comparison
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return errors.New("invalid signature")
	}

	return nil
}

// RateLimiter is already defined in connection.go, but let's add configuration
type RateLimiterConfig struct {
	Rate    float64 // Requests per second
	Burst   float64 // Burst capacity
	PerIP   bool    // Enable per-IP rate limiting
	PerUser bool    // Enable per-user rate limiting
}

// DefaultRateLimiterConfig returns default rate limiter configuration
func DefaultRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		Rate:    1000.0 / 60.0, // 1000 per minute
		Burst:   100,
		PerIP:   true,
		PerUser: true,
	}
}

// IPRateLimiter manages rate limiting per IP address
type IPRateLimiter struct {
	limiters map[string]*RateLimiter
	mu       sync.RWMutex
	config   *RateLimiterConfig
}

// NewIPRateLimiter creates a new IP-based rate limiter
func NewIPRateLimiter(config *RateLimiterConfig) *IPRateLimiter {
	return &IPRateLimiter{
		limiters: make(map[string]*RateLimiter),
		config:   config,
	}
}

// Allow checks if a request from an IP is allowed
func (r *IPRateLimiter) Allow(ip string) bool {
	r.mu.Lock()
	limiter, ok := r.limiters[ip]
	if !ok {
		limiter = NewRateLimiter(r.config.Rate, r.config.Burst)
		r.limiters[ip] = limiter
	}
	r.mu.Unlock()

	return limiter.Allow()
}

// Cleanup removes old rate limiters
func (r *IPRateLimiter) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// In production, implement cleanup of inactive IPs
	// For now, just clear if too many entries
	if len(r.limiters) > 10000 {
		r.limiters = make(map[string]*RateLimiter)
	}
}

// AntiReplayCache prevents replay attacks
type AntiReplayCache struct {
	seen map[string]time.Time
	mu   sync.RWMutex
	ttl  time.Duration
}

// NewAntiReplayCache creates a new anti-replay cache
func NewAntiReplayCache(ttl time.Duration) *AntiReplayCache {
	cache := &AntiReplayCache{
		seen: make(map[string]time.Time),
		ttl:  ttl,
	}

	// Start cleanup goroutine
	go cache.cleanup()

	return cache
}

// Check verifies if a message ID has been seen
func (c *AntiReplayCache) Check(messageID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.seen[messageID]; ok {
		return false // Already seen, reject
	}

	c.seen[messageID] = time.Now()
	return true
}

// cleanup removes expired entries
func (c *AntiReplayCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for id, timestamp := range c.seen {
			if now.Sub(timestamp) > c.ttl {
				delete(c.seen, id)
			}
		}
		c.mu.Unlock()
	}
}

// Enhanced auth features are already integrated into the Server struct in server.go:
// - sessionManager  *SessionManager
// - ipRateLimiter   *IPRateLimiter
// - antiReplayCache *AntiReplayCache

// ValidateMessageAuth validates authentication for a single message
func (s *Server) ValidateMessageAuth(conn *Connection, msg *AuthenticatedMessage) error {
	// Check timestamp (prevent old messages)
	if time.Since(msg.Timestamp) > 5*time.Minute {
		s.metricsCollector.RecordError("auth")
		return errors.New("message too old")
	}

	// Check anti-replay
	if s.antiReplayCache != nil && !s.antiReplayCache.Check(msg.Message.ID) {
		s.metricsCollector.RecordError("auth")
		return errors.New("duplicate message")
	}

	// Verify signature if HMAC is enabled
	if s.config.Security.HMACSignatures && msg.Signature != "" {
		msgBytes, err := json.Marshal(msg.Message)
		if err != nil {
			return err
		}

		if err := conn.VerifyMessage(msgBytes, msg.Signature); err != nil {
			return err
		}
	}

	return nil
}

// Security configuration
type SecurityConfig struct {
	RequireAuth    bool     // Require authentication
	HMACSignatures bool     // Enable HMAC signatures
	AllowedOrigins []string // CORS allowed origins
	MaxFrameSize   int64    // Maximum WebSocket frame size
	EnableTLS      bool     // Require TLS
	MinTLSVersion  string   // Minimum TLS version
	JWTSecret      string   // JWT signing secret
	APIKeys        []string // Valid API keys
	IPWhitelist    []string // Allowed IP addresses (empty = allow all)
}
