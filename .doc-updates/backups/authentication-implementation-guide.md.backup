# Authentication Implementation Guide

This guide provides comprehensive documentation for implementing authentication in the Developer Mesh platform, covering all authentication methods, rate limiting, metrics, and testing patterns.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Authentication Methods](#authentication-methods)
3. [Rate Limiting](#rate-limiting)
4. [Metrics and Monitoring](#metrics-and-monitoring)
5. [Implementation Patterns](#implementation-patterns)
6. [Testing Strategies](#testing-strategies)
7. [Security Best Practices](#security-best-practices)

## Architecture Overview

The Developer Mesh platform uses a centralized authentication service (`pkg/auth`) that provides:

- Two authentication methods (API keys, JWT)
- OAuth interfaces (not implemented)
- Middleware for both Gin and standard HTTP handlers
- Rate limiting integration
- Metrics collection
- Caching for performance
- Basic tenant-based access control (not Casbin RBAC)

### Key Components

```
pkg/
├── auth/                     # Core authentication package
│   ├── auth.go              # Main authentication service
│   ├── middleware.go        # HTTP middleware implementations
│   ├── credential_context.go # Context management for credentials
│   └── credential_middleware.go # Credential extraction middleware
├── resilience/              # Rate limiting and circuit breakers
│   ├── rate_limiter.go     # Token bucket rate limiter
│   └── rate_limiter_methods.go
└── observability/           # Metrics and monitoring
    └── metrics.go          # Metrics client implementation
```

## Authentication Methods

### 1. API Key Authentication

The primary authentication method using static or database-backed API keys.

#### Implementation

```go
// Create auth service with API key support
config := auth.DefaultConfig()
config.EnableAPIKeys = true
config.APIKeyHeader = "X-API-Key"
authService := auth.NewService(config, db, cache, logger)

// Initialize default API keys from configuration
authService.InitializeDefaultAPIKeys(map[string]string{
    "test-key": "read",     // Read-only access
    "admin-key": "admin",   // Full admin access
})

// Apply middleware to routes
router.Use(authService.GinMiddleware(auth.TypeAPIKey))
```

#### API Key Validation Flow

1. Check `Authorization: Bearer <key>` header
2. Check custom API key header (e.g., `X-API-Key`)
3. Validate against cache (if enabled)
4. Validate against in-memory storage
5. Validate against database
6. Update last-used timestamp
7. Cache result for performance

### 2. JWT Authentication

Token-based authentication for session management.

#### Implementation

```go
// Configure JWT authentication
config := auth.DefaultConfig()
config.EnableJWT = true
config.JWTSecret = os.Getenv("JWT_SECRET")
config.JWTExpiration = 24 * time.Hour

// Generate JWT for user
user := &auth.User{
    ID:       "user-123",
    TenantID: "tenant-456",
    Email:    "user@example.com",
    Scopes:   []string{"read", "write"},
}
token, err := authService.GenerateJWT(ctx, user)

// Apply JWT middleware
router.Use(authService.GinMiddleware(auth.TypeJWT))
```

### 3. GitHub App Authentication (Adapter-specific)

**Note**: GitHub App authentication is implemented separately in the GitHub adapter, not as part of the core auth package.

#### Implementation

```go
// pkg/adapters/github/auth/provider.go
// This is NOT part of the main auth package
import "github.com/developer-mesh/developer-mesh/pkg/adapters/github/auth"

appProvider, err := auth.NewAppProvider(
    appID,
    privateKeyPEM,
    installationID,
    logger,
)

// Automatic token refresh
token, err := appProvider.GetToken(ctx)

// Set auth headers on requests
err := appProvider.SetAuthHeaders(req)
```

#### Token Refresh Flow

1. Check if current token is valid
2. Generate JWT using app private key
3. Request installation token from GitHub
4. Cache token until expiry
5. Automatically refresh before expiry

### 4. OAuth Authentication (Not Implemented)

**Status**: Only interfaces exist, no concrete OAuth providers are implemented.

```go
// Interface exists but no implementations
type OAuthProvider interface {
    GetAuthorizationURL(state string) (string, error)
    ExchangeCode(ctx context.Context, code string) (*OAuthToken, error)
    RefreshToken(ctx context.Context, refreshToken string) (*OAuthToken, error)
    ValidateToken(ctx context.Context, token string) (*User, error)
}

// No Google, GitHub OAuth, or other providers implemented
```

## Rate Limiting

### Token Bucket Implementation

```go
// pkg/resilience/rate_limiter.go
limiterConfig := RateLimiterConfig{
    Limit:       100,           // 100 requests
    Period:      time.Minute,   // per minute
    BurstFactor: 3,            // allow burst up to 300
}

manager := NewRateLimiterManager(map[string]RateLimiterConfig{
    "api":     {Limit: 1000, Period: time.Minute},
    "webhook": {Limit: 100, Period: time.Minute},
    "auth":    {Limit: 50, Period: time.Minute},
})

// Use in middleware
limiter := manager.GetRateLimiter("api")
if !limiter.Allow() {
    c.JSON(429, gin.H{"error": "rate limit exceeded"})
    return
}
```

### Integration with Authentication

```go
// Custom middleware with rate limiting per tenant
func RateLimitMiddleware(manager *RateLimiterManager) gin.HandlerFunc {
    return func(c *gin.Context) {
        user, _ := auth.GetUserFromContext(c)
        
        // Create tenant-specific rate limiter
        limiterName := fmt.Sprintf("tenant:%s", user.TenantID)
        limiter := manager.GetRateLimiter(limiterName)
        
        if !limiter.Allow() {
            metrics.RecordCounter("rate_limit_exceeded", 1, map[string]string{
                "tenant_id": user.TenantID,
                "endpoint":  c.Request.URL.Path,
            })
            c.JSON(429, gin.H{"error": "rate limit exceeded"})
            c.Abort()
            return
        }
        
        c.Next()
    }
}
```

## Metrics and Monitoring

### Metrics Collection

```go
// Record authentication attempts
metrics.RecordCounter("auth_attempts_total", 1, map[string]string{
    "method": "api_key",
    "status": "success",
})

// Record authentication latency
startTime := time.Now()
// ... authentication logic ...
metrics.RecordHistogram("auth_duration_seconds", time.Since(startTime).Seconds(), map[string]string{
    "method": "api_key",
})

// Record rate limit metrics
metrics.RecordCounter("rate_limit_checks_total", 1, map[string]string{
    "limiter": "api",
    "allowed": strconv.FormatBool(allowed),
})
```

### Recommended Metrics

1. **Authentication Metrics**
   - `auth_attempts_total` - Counter of authentication attempts
   - `auth_duration_seconds` - Histogram of authentication duration
   - `auth_failures_total` - Counter of failed authentications
   - `active_sessions_gauge` - Gauge of active JWT sessions

2. **Rate Limiting Metrics**
   - `rate_limit_checks_total` - Counter of rate limit checks
   - `rate_limit_exceeded_total` - Counter of rate limit violations
   - `rate_limit_tokens_gauge` - Gauge of available tokens

3. **API Key Metrics**
   - `api_key_usage_total` - Counter of API key usage
   - `api_key_last_used_timestamp` - Gauge of last usage time

## Implementation Patterns

### 1. Middleware Stack

```go
// Recommended middleware order
router := gin.New()
router.Use(
    gin.Recovery(),
    RequestIDMiddleware(),        // Add request ID
    LoggingMiddleware(logger),    // Log requests
    authService.GinMiddleware(),  // Authentication
    RateLimitMiddleware(manager), // Rate limiting
    MetricsMiddleware(metrics),   // Collect metrics
)
```

### 2. Context Management

```go
// Store user in context
c.Set(string(auth.UserContextKey), user)
c.Set("tenant_id", user.TenantID)

// Retrieve user from context
user, ok := auth.GetUserFromContext(c)
if !ok {
    c.JSON(401, gin.H{"error": "unauthorized"})
    return
}

// Tenant-based filtering
tenantID, _ := auth.GetTenantFromContext(c)
query = query.Where("tenant_id = ?", tenantID)
```

### 3. Scope-Based Authorization

```go
// Define route with required scopes
router.POST("/api/v1/admin/users", 
    authService.GinMiddleware(auth.TypeAPIKey),
    authService.RequireScopes("admin", "write"),
    handleCreateUser,
)

// Note: Authorization uses simple in-memory policies,
// not Casbin RBAC as might be mentioned elsewhere

// Check scopes in handler
func handleCreateUser(c *gin.Context) {
    user, _ := auth.GetUserFromContext(c)
    
    // Additional authorization logic
    if !canCreateUsers(user) {
        c.JSON(403, gin.H{"error": "forbidden"})
        return
    }
    
    // ... handle request ...
}
```

### 4. Webhook Authentication

```go
// GitHub webhook signature verification
func GitHubWebhookMiddleware(secret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        signature := c.GetHeader("X-Hub-Signature-256")
        
        body, _ := io.ReadAll(c.Request.Body)
        c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
        
        if !verifySignature(body, signature, secret) {
            c.JSON(401, gin.H{"error": "invalid signature"})
            c.Abort()
            return
        }
        
        c.Next()
    }
}
```

## Testing Strategies

### 1. Unit Testing Authentication

```go
func TestAPIKeyAuthentication(t *testing.T) {
    // Setup
    config := auth.DefaultConfig()
    authService := auth.NewService(config, nil, nil, logger)
    authService.InitializeDefaultAPIKeys(map[string]string{
        "test-key": "read",
    })
    
    // Test valid key
    user, err := authService.ValidateAPIKey(ctx, "test-key")
    assert.NoError(t, err)
    assert.Equal(t, "system", user.UserID)
    assert.Contains(t, user.Scopes, "read")
    
    // Test invalid key
    _, err = authService.ValidateAPIKey(ctx, "invalid-key")
    assert.Equal(t, auth.ErrInvalidAPIKey, err)
}
```

### 2. Integration Testing

```go
func TestAuthenticationIntegration(t *testing.T) {
    // Setup test server
    router := setupTestRouter()
    
    // Test with valid authentication
    req := httptest.NewRequest("GET", "/api/v1/models", nil)
    req.Header.Set("Authorization", "Bearer test-key")
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    
    assert.Equal(t, 200, w.Code)
    
    // Test without authentication
    req = httptest.NewRequest("GET", "/api/v1/models", nil)
    w = httptest.NewRecorder()
    router.ServeHTTP(w, req)
    
    assert.Equal(t, 401, w.Code)
}
```

### 3. Functional Testing (Ginkgo)

```go
var _ = Describe("Authentication", func() {
    var (
        validClient   *client.MCPClient
        invalidClient *client.MCPClient
    )
    
    BeforeEach(func() {
        validClient = client.NewMCPClient(
            ServerURL,
            "test-admin-api-key",
            client.WithTenantID("test-tenant"),
        )
        
        invalidClient = client.NewMCPClient(
            ServerURL,
            "invalid-key",
            client.WithTenantID("test-tenant"),
        )
    })
    
    It("should accept valid API key", func() {
        resp, err := validClient.Get(ctx, "/api/v1/tools")
        Expect(err).NotTo(HaveOccurred())
        Expect(resp.StatusCode).To(Equal(http.StatusOK))
    })
    
    It("should reject invalid API key", func() {
        resp, err := invalidClient.Get(ctx, "/api/v1/tools")
        Expect(err).NotTo(HaveOccurred())
        Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
    })
})
```

### 4. Rate Limiting Tests

```go
func TestRateLimiting(t *testing.T) {
    limiter := NewRateLimiter("test", RateLimiterConfig{
        Limit:  5,
        Period: time.Second,
    })
    
    // Should allow initial requests
    for i := 0; i < 5; i++ {
        assert.True(t, limiter.Allow())
    }
    
    // Should block after limit
    assert.False(t, limiter.Allow())
    
    // Should allow after period
    time.Sleep(time.Second)
    assert.True(t, limiter.Allow())
}
```

## Security Best Practices

### 1. API Key Management

- Generate cryptographically secure keys
- Store hashed keys in database
- Implement key rotation policies
- Log key usage for auditing
- Set expiration dates for keys

### 2. JWT Security

- Use strong signing secrets (256+ bits)
- Implement short expiration times
- Include minimal claims in tokens
- Validate all claims on each request
- Implement token revocation

### 3. Rate Limiting Strategies

- Implement per-tenant limits
- Use sliding window algorithms
- Provide rate limit headers in responses
- Implement backoff strategies
- Monitor for abuse patterns

### 4. Monitoring and Alerting

- Alert on authentication failures
- Monitor rate limit violations
- Track API key usage patterns
- Log all authentication events
- Implement anomaly detection

### 5. Defense in Depth

```go
// Multiple layers of security
router.Use(
    IPWhitelistMiddleware(allowedIPs),     // IP filtering
    authService.GinMiddleware(),           // Authentication
    RateLimitMiddleware(manager),          // Rate limiting
    authService.RequireScopes("read"),     // Authorization
    AuditLogMiddleware(auditLogger),       // Audit logging
)
```

## Common Patterns and Examples

### 1. Custom Authentication Provider

```go
type CustomAuthProvider struct {
    BaseAuthProvider
    customToken string
}

func (p *CustomAuthProvider) GetToken(ctx context.Context) (string, error) {
    // Custom token logic
    return p.customToken, nil
}

func (p *CustomAuthProvider) SetAuthHeaders(req *http.Request) error {
    req.Header.Set("X-Custom-Auth", p.customToken)
    return nil
}
```

### 2. Multi-Tenant Rate Limiting

```go
func NewTenantRateLimiter(config map[string]RateLimiterConfig) *TenantRateLimiter {
    return &TenantRateLimiter{
        limiters: make(map[string]*RateLimiter),
        defaults: RateLimiterConfig{
            Limit:  100,
            Period: time.Minute,
        },
    }
}

func (t *TenantRateLimiter) CheckLimit(tenantID string) bool {
    limiter := t.getOrCreateLimiter(tenantID)
    return limiter.Allow()
}
```

### 3. Authentication Caching

```go
func CachedAuthMiddleware(authService *auth.Service, cache cache.Cache) gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        cacheKey := fmt.Sprintf("auth:%s", hash(token))
        
        // Check cache
        var cachedUser auth.User
        if err := cache.Get(c, cacheKey, &cachedUser); err == nil {
            c.Set(string(auth.UserContextKey), &cachedUser)
            c.Next()
            return
        }
        
        // Validate and cache
        authService.GinMiddleware()(c)
        
        if user, ok := auth.GetUserFromContext(c); ok {
            cache.Set(c, cacheKey, user, 5*time.Minute)
        }
    }
}
```

## Troubleshooting

### Common Issues

1. **401 Unauthorized**
   - Check API key format and headers
   - Verify key is active and not expired
   - Check tenant ID matches

2. **429 Rate Limit Exceeded**
   - Check rate limit configuration
   - Verify tenant-specific limits
   - Implement exponential backoff

3. **403 Forbidden**
   - Verify user has required scopes
   - Check tenant access permissions
   - Validate webhook signatures

### Debug Logging

```go
// Enable debug logging for authentication
logger := observability.NewLogger("auth")
logger.SetLevel("debug")

// Log all authentication attempts
authService = auth.NewService(config, db, cache, logger)
```

## Current Limitations and Future Work

### Not Yet Implemented:
1. **OAuth Providers**: Only interfaces exist, no concrete implementations
2. **Casbin RBAC**: Currently uses simple in-memory authorization
3. **External Identity Providers**: No SAML, OIDC, or external IdP support
4. **Token Revocation**: JWT tokens cannot be revoked before expiry
5. **Multi-factor Authentication**: Not supported

### What Works Today:
1. **API Key Authentication**: Full support with database backing
2. **JWT Authentication**: Token generation and validation
3. **Basic Authorization**: Scope-based access control
4. **Rate Limiting**: Per-tenant and per-endpoint limits
5. **Metrics and Monitoring**: Authentication metrics via Prometheus

## Conclusion

This guide documents the current authentication implementation in Developer Mesh. While the foundation is solid with API key and JWT support, several advanced features (OAuth, Casbin RBAC, external IdPs) remain unimplemented. Always verify feature availability in the actual codebase before relying on documentation.

For the most accurate implementation status, see `docs/guides/auth-implementation-status.md`.