package clients

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// SecurityMiddleware provides HTTP middleware for security features
type SecurityMiddleware struct {
	securityManager *SecurityManager
	logger          observability.Logger
	config          SecurityMiddlewareConfig
}

// SecurityMiddlewareConfig defines middleware configuration
type SecurityMiddlewareConfig struct {
	// Security headers
	EnableSecurityHeaders bool
	CSPPolicy             string
	HSTSMaxAge            int

	// CORS configuration
	EnableCORS       bool
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int

	// Request validation
	ValidateInput  bool
	MaxRequestSize int64
	MaxHeaderSize  int

	// Authentication
	RequireAuth bool
	AuthHeader  string

	// Rate limiting
	EnableRateLimit bool

	// Audit logging
	EnableAudit bool
}

// DefaultSecurityMiddlewareConfig returns default middleware configuration
func DefaultSecurityMiddlewareConfig() SecurityMiddlewareConfig {
	return SecurityMiddlewareConfig{
		EnableSecurityHeaders: true,
		CSPPolicy:             "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'",
		HSTSMaxAge:            31536000, // 1 year
		EnableCORS:            true,
		AllowedOrigins:        []string{"*"},
		AllowedMethods:        []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:        []string{"Content-Type", "Authorization", "X-Correlation-ID"},
		ExposedHeaders:        []string{"X-Correlation-ID", "X-Request-ID"},
		AllowCredentials:      false,
		MaxAge:                3600,
		ValidateInput:         true,
		MaxRequestSize:        10 * 1024 * 1024, // 10MB
		MaxHeaderSize:         8192,
		RequireAuth:           true,
		AuthHeader:            "Authorization",
		EnableRateLimit:       true,
		EnableAudit:           true,
	}
}

// NewSecurityMiddleware creates a new security middleware
func NewSecurityMiddleware(manager *SecurityManager, config SecurityMiddlewareConfig, logger observability.Logger) *SecurityMiddleware {
	return &SecurityMiddleware{
		securityManager: manager,
		logger:          logger,
		config:          config,
	}
}

// WrapTransport wraps an HTTP transport with security features
func (m *SecurityMiddleware) WrapTransport(transport http.RoundTripper) http.RoundTripper {
	return &secureTransport{
		base:       transport,
		middleware: m,
	}
}

// secureTransport implements http.RoundTripper with security features
type secureTransport struct {
	base       http.RoundTripper
	middleware *SecurityMiddleware
}

// RoundTrip executes an HTTP request with security features
func (t *secureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	startTime := time.Now()

	// Extract context values
	ctx := req.Context()
	tenantID := extractTenantID(ctx)
	userID := extractUserID(ctx)
	correlationID := extractCorrelationID(ctx)

	// Add security headers to request
	t.addSecurityHeaders(req)

	// Validate request
	if t.middleware.config.ValidateInput {
		if err := t.validateRequest(req); err != nil {
			t.middleware.logger.Warn("Request validation failed", map[string]interface{}{
				"error":          err.Error(),
				"correlation_id": correlationID,
			})
			return nil, err
		}
	}

	// Check rate limit
	if t.middleware.config.EnableRateLimit && tenantID != "" {
		if err := t.middleware.securityManager.CheckRateLimit(ctx, tenantID); err != nil {
			t.middleware.logger.Warn("Rate limit exceeded", map[string]interface{}{
				"tenant_id":      tenantID,
				"correlation_id": correlationID,
			})

			// Return 429 response
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Status:     "429 Too Many Requests",
				Header: http.Header{
					"Retry-After": []string{"60"},
				},
				Request: req,
			}, nil
		}
	}

	// Add authentication if required
	if t.middleware.config.RequireAuth {
		if err := t.addAuthentication(req, ctx); err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
	}

	// Encrypt sensitive data if needed
	if req.Body != nil && t.shouldEncryptBody(req) {
		if err := t.encryptRequestBody(req); err != nil {
			return nil, fmt.Errorf("failed to encrypt request: %w", err)
		}
	}

	// Execute request
	resp, err := t.base.RoundTrip(req)

	// Audit log the request
	if t.middleware.config.EnableAudit {
		t.auditRequest(req, resp, err, time.Since(startTime), userID, tenantID)
	}

	// Check for threats
	if resp != nil && t.middleware.securityManager.threatDetector != nil {
		activity := map[string]interface{}{
			"endpoint":    req.URL.Path,
			"method":      req.Method,
			"status_code": resp.StatusCode,
			"error":       err != nil || resp.StatusCode >= 400,
			"duration":    time.Since(startTime).Milliseconds(),
		}

		if threat := t.middleware.securityManager.DetectThreat(ctx, userID, tenantID, activity); threat != nil {
			t.middleware.logger.Warn("Threat detected", map[string]interface{}{
				"threat_type":    threat.Type,
				"severity":       threat.Severity,
				"user_id":        userID,
				"tenant_id":      tenantID,
				"correlation_id": correlationID,
			})

			if threat.Action == "block" {
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Status:     "403 Forbidden - Security Threat Detected",
					Request:    req,
				}, nil
			}
		}
	}

	// Process response
	if resp != nil {
		// Add security headers to response
		t.addResponseSecurityHeaders(resp)

		// Decrypt response if needed
		if t.shouldDecryptBody(resp) {
			if err := t.decryptResponseBody(resp); err != nil {
				t.middleware.logger.Error("Failed to decrypt response", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}
	}

	return resp, err
}

// addSecurityHeaders adds security headers to request
func (t *secureTransport) addSecurityHeaders(req *http.Request) {
	// Add correlation ID if not present
	if req.Header.Get("X-Correlation-ID") == "" {
		if correlationID := extractCorrelationID(req.Context()); correlationID != "" {
			req.Header.Set("X-Correlation-ID", correlationID)
		}
	}

	// Add security headers
	req.Header.Set("X-Content-Type-Options", "nosniff")
	req.Header.Set("X-Frame-Options", "DENY")
	req.Header.Set("X-XSS-Protection", "1; mode=block")

	// Add user agent if not present
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "MCP-Security-Client/1.0")
	}
}

// addResponseSecurityHeaders adds security headers to response
func (t *secureTransport) addResponseSecurityHeaders(resp *http.Response) {
	if t.middleware.config.EnableSecurityHeaders {
		// Content Security Policy
		if t.middleware.config.CSPPolicy != "" {
			resp.Header.Set("Content-Security-Policy", t.middleware.config.CSPPolicy)
		}

		// Strict Transport Security
		if t.middleware.config.HSTSMaxAge > 0 {
			resp.Header.Set("Strict-Transport-Security",
				fmt.Sprintf("max-age=%d; includeSubDomains", t.middleware.config.HSTSMaxAge))
		}

		// Other security headers
		resp.Header.Set("X-Content-Type-Options", "nosniff")
		resp.Header.Set("X-Frame-Options", "DENY")
		resp.Header.Set("X-XSS-Protection", "1; mode=block")
		resp.Header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		resp.Header.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
	}

	// CORS headers
	if t.middleware.config.EnableCORS {
		t.addCORSHeaders(resp)
	}
}

// addCORSHeaders adds CORS headers to response
func (t *secureTransport) addCORSHeaders(resp *http.Response) {
	// Allowed origins
	if len(t.middleware.config.AllowedOrigins) > 0 {
		origin := resp.Request.Header.Get("Origin")
		if t.isOriginAllowed(origin) {
			resp.Header.Set("Access-Control-Allow-Origin", origin)
		}
	}

	// Allowed methods
	if len(t.middleware.config.AllowedMethods) > 0 {
		resp.Header.Set("Access-Control-Allow-Methods",
			strings.Join(t.middleware.config.AllowedMethods, ", "))
	}

	// Allowed headers
	if len(t.middleware.config.AllowedHeaders) > 0 {
		resp.Header.Set("Access-Control-Allow-Headers",
			strings.Join(t.middleware.config.AllowedHeaders, ", "))
	}

	// Exposed headers
	if len(t.middleware.config.ExposedHeaders) > 0 {
		resp.Header.Set("Access-Control-Expose-Headers",
			strings.Join(t.middleware.config.ExposedHeaders, ", "))
	}

	// Credentials
	if t.middleware.config.AllowCredentials {
		resp.Header.Set("Access-Control-Allow-Credentials", "true")
	}

	// Max age
	if t.middleware.config.MaxAge > 0 {
		resp.Header.Set("Access-Control-Max-Age",
			fmt.Sprintf("%d", t.middleware.config.MaxAge))
	}
}

// isOriginAllowed checks if an origin is allowed
func (t *secureTransport) isOriginAllowed(origin string) bool {
	for _, allowed := range t.middleware.config.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
		// Support wildcard subdomains
		if strings.HasPrefix(allowed, "*.") {
			domain := strings.TrimPrefix(allowed, "*")
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}
	return false
}

// validateRequest validates the HTTP request
func (t *secureTransport) validateRequest(req *http.Request) error {
	// Check request size
	if req.ContentLength > 0 && req.ContentLength > t.middleware.config.MaxRequestSize {
		return fmt.Errorf("request size %d exceeds maximum %d",
			req.ContentLength, t.middleware.config.MaxRequestSize)
	}

	// Validate headers
	totalHeaderSize := 0
	for key, values := range req.Header {
		for _, value := range values {
			totalHeaderSize += len(key) + len(value)

			// Check for header injection
			if strings.ContainsAny(value, "\r\n") {
				return fmt.Errorf("potential header injection in %s", key)
			}
		}
	}

	if totalHeaderSize > t.middleware.config.MaxHeaderSize {
		return fmt.Errorf("header size %d exceeds maximum %d",
			totalHeaderSize, t.middleware.config.MaxHeaderSize)
	}

	// Validate URL parameters
	if req.URL != nil {
		for key, values := range req.URL.Query() {
			for _, value := range values {
				if err := t.middleware.securityManager.ValidateInput(key, value); err != nil {
					return fmt.Errorf("invalid query parameter %s: %w", key, err)
				}
			}
		}
	}

	return nil
}

// addAuthentication adds authentication to the request
func (t *secureTransport) addAuthentication(req *http.Request, ctx context.Context) error {
	// Check if auth header already present
	if req.Header.Get(t.middleware.config.AuthHeader) != "" {
		return nil
	}

	// Get token from context or create new one
	token := extractToken(ctx)
	if token == "" {
		// Get or create token for user
		userID := extractUserID(ctx)
		tenantID := extractTenantID(ctx)

		if userID == "" || tenantID == "" {
			return fmt.Errorf("missing user or tenant ID for authentication")
		}

		// Create token
		tokenObj, err := t.middleware.securityManager.CreateToken(ctx, userID, tenantID, []string{"api:access"})
		if err != nil {
			return fmt.Errorf("failed to create token: %w", err)
		}

		token = tokenObj.Value
	}

	// Add auth header
	req.Header.Set(t.middleware.config.AuthHeader, "Bearer "+token)

	return nil
}

// shouldEncryptBody determines if request body should be encrypted
func (t *secureTransport) shouldEncryptBody(req *http.Request) bool {
	// Check if encryption is enabled
	if t.middleware.securityManager.encryptor == nil {
		return false
	}

	// Check content type
	contentType := req.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "application/xml") ||
		strings.Contains(contentType, "text/") {
		return true
	}

	// Check for sensitive endpoints
	sensitiveEndpoints := []string{
		"/api/v1/credentials",
		"/api/v1/tokens",
		"/api/v1/keys",
		"/api/v1/secrets",
	}

	for _, endpoint := range sensitiveEndpoints {
		if strings.Contains(req.URL.Path, endpoint) {
			return true
		}
	}

	return false
}

// encryptRequestBody encrypts the request body
func (t *secureTransport) encryptRequestBody(req *http.Request) error {
	// Read body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	_ = req.Body.Close()

	// Encrypt
	encrypted, err := t.middleware.securityManager.EncryptData(body)
	if err != nil {
		return err
	}

	// Set new body
	req.Body = io.NopCloser(strings.NewReader(string(encrypted)))
	req.ContentLength = int64(len(encrypted))
	req.Header.Set("X-Encrypted", "true")

	return nil
}

// shouldDecryptBody determines if response body should be decrypted
func (t *secureTransport) shouldDecryptBody(resp *http.Response) bool {
	return resp.Header.Get("X-Encrypted") == "true"
}

// decryptResponseBody decrypts the response body
func (t *secureTransport) decryptResponseBody(resp *http.Response) error {
	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()

	// Decrypt
	decrypted, err := t.middleware.securityManager.DecryptData(body)
	if err != nil {
		return err
	}

	// Set new body
	resp.Body = io.NopCloser(strings.NewReader(string(decrypted)))
	resp.ContentLength = int64(len(decrypted))
	resp.Header.Del("X-Encrypted")

	return nil
}

// auditRequest logs the request for audit
func (t *secureTransport) auditRequest(req *http.Request, resp *http.Response, err error, duration time.Duration, userID, tenantID string) {
	details := map[string]interface{}{
		"method":      req.Method,
		"url":         req.URL.String(),
		"duration_ms": duration.Milliseconds(),
		"user_agent":  req.Header.Get("User-Agent"),
		"remote_addr": req.RemoteAddr,
	}

	if resp != nil {
		details["status_code"] = resp.StatusCode
	}

	if err != nil {
		details["error"] = err.Error()
	}

	eventType := "api_request"
	if err != nil || (resp != nil && resp.StatusCode >= 400) {
		eventType = "api_error"
	}

	t.middleware.securityManager.auditLog(eventType, userID, tenantID, details)
}

// Helper functions to extract context values
func extractTenantID(ctx context.Context) string {
	if val := ctx.Value(ContextKeyTenantID); val != nil {
		if id, ok := val.(string); ok {
			return id
		}
	}
	return ""
}

func extractUserID(ctx context.Context) string {
	if val := ctx.Value(ContextKeyAgentID); val != nil {
		if id, ok := val.(string); ok {
			return id
		}
	}
	return ""
}

func extractCorrelationID(ctx context.Context) string {
	if val := ctx.Value(ContextKeyCorrelationID); val != nil {
		if id, ok := val.(string); ok {
			return id
		}
	}
	return ""
}

func extractToken(ctx context.Context) string {
	if val := ctx.Value("auth_token"); val != nil {
		if token, ok := val.(string); ok {
			return token
		}
	}
	return ""
}

// CreateSecureHTTPClient creates an HTTP client with security middleware
func CreateSecureHTTPClient(manager *SecurityManager, config SecurityMiddlewareConfig, logger observability.Logger) *http.Client {
	// Create base transport
	baseTransport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		ForceAttemptHTTP2:   true,
	}

	// Create security middleware
	middleware := NewSecurityMiddleware(manager, config, logger)

	// Wrap transport with security
	secureTransport := middleware.WrapTransport(baseTransport)

	// Create HTTP client
	return &http.Client{
		Transport: secureTransport,
		Timeout:   30 * time.Second,
	}
}

// SecurityContext enriches context with security information
func SecurityContext(ctx context.Context, tenantID, userID, correlationID string) context.Context {
	ctx = context.WithValue(ctx, ContextKeyTenantID, tenantID)
	ctx = context.WithValue(ctx, ContextKeyAgentID, userID)
	ctx = context.WithValue(ctx, ContextKeyCorrelationID, correlationID)
	return ctx
}
