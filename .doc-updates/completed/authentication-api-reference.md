<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:27:57
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# Authentication API Reference

## Overview

The Developer Mesh uses middleware-based authentication for API access. Authentication is handled through API keys and JWT tokens validated on each request, with basic rate limiting support.

## Table of Contents

1. [Authentication Endpoints](#authentication-endpoints)
2. [Configuration API](#configuration-api)
3. [Types and Models](#types-and-models)
4. [Middleware Functions](#middleware-functions)
5. [Rate Limiting API](#rate-limiting-api)
6. [Metrics API](#metrics-api)
7. [Error Codes](#error-codes)

## Authentication Methods

Authentication is handled through middleware on all protected endpoints. There are no dedicated authentication endpoints.

### API Key Authentication

API keys can be provided in two ways:

**Authorization Header:**
```
Authorization: Bearer <api-key>
```

**X-API-Key Header:**
```
X-API-Key: <api-key>
```

### JWT Token Authentication

JWT tokens are validated when provided:

```
Authorization: Bearer <jwt-token>
```

**Note**: The system validates JWT tokens but does not provide endpoints to generate them. JWT tokens must be obtained through external means.

### Rate Limiting

When rate limited, the following headers are returned:

```
X-RateLimit-Remaining: 0
Retry-After: <seconds>
```

**Status Code**: 429 Too Many Requests

### Authentication Errors

**Missing Authentication (401):**
```json
{
  "error": "unauthorized"
}
```

**Invalid API Key (401):**
```json
{
  "error": "Invalid API key"
}
```

**Rate Limited (429):**
```json
{
  "token": "string",
  "token_type": "api_key|jwt"
}
```

**Response:**
```json
{
  "revoked": true,
  "revoked_at": "2024-01-01T00:00:00Z"
}
```

## Configuration API

### SetupAuthenticationWithConfig
Initialize authentication system with custom configuration.

```go
func SetupAuthenticationWithConfig(
    config *AuthSystemConfig,
    db *sqlx.DB,
    cache cache.Cache,
    logger observability.Logger,
    metrics observability.MetricsClient,
) (*AuthMiddleware, error)
```

**Parameters:**
- `config`: Complete authentication system configuration
- `db`: Database connection (optional, can be nil)
- `cache`: Cache implementation for rate limiting and performance
- `logger`: Logger instance for audit and debug logging
- `metrics`: Metrics client for monitoring

**Example:**
```go
config := &auth.AuthSystemConfig{
    Service: &auth.ServiceConfig{
        JWTSecret:         "your-secret-key",
        JWTExpiration:     24 * time.Hour,
        EnableAPIKeys:     true,
        EnableJWT:         true,
        CacheEnabled:      true,
        MaxFailedAttempts: 5,
        LockoutDuration:   15 * time.Minute,
    },
    RateLimiter: &auth.RateLimiterConfig{
        MaxAttempts:   100,
        WindowSize:    1 * time.Minute,
        LockoutPeriod: 15 * time.Minute,
    },
    APIKeys: map[string]auth.APIKeySettings{
        "prod-key-123": {
            Role:     "admin",
            Scopes:   []string{"read", "write", "admin"},
            TenantID: "tenant-1",
        },
    },
}

middleware, err := auth.SetupAuthenticationWithConfig(
    config, db, cache, logger, metrics,
)
```

### AddAPIKey
Add API key at runtime.

```go
func (s *Service) AddAPIKey(key string, settings APIKeySettings) error
```

**Parameters:**
- `key`: The API key string (minimum 16 characters)
- `settings`: API key configuration including role, scopes, and tenant

**Example:**
```go
err := authService.AddAPIKey("prod-key-minimum-16-chars", auth.APIKeySettings{
    Role:      "admin",
    Scopes:    []string{"read", "write", "admin"},
    TenantID:  "tenant-123",
    ExpiresIn: "30d", // Optional expiration
})
```

## Types and Models

### AuthSystemConfig
Complete authentication system configuration.

```go
type AuthSystemConfig struct {
    Service      *ServiceConfig
    RateLimiter  *RateLimiterConfig
    APIKeys      map[string]APIKeySettings
}
```

### ServiceConfig
Core authentication service configuration.

```go
type ServiceConfig struct {
    // JWT Configuration
    JWTSecret         string
    JWTExpiration     time.Duration
    
    // API Key Configuration
    APIKeyHeader      string // Default: "X-API-Key"
    EnableAPIKeys     bool
    EnableJWT         bool
    
    // Cache Configuration
    CacheEnabled      bool
    CacheTTL          time.Duration
    
    // Security Configuration
    MaxFailedAttempts int
    LockoutDuration   time.Duration
}
```

### RateLimiterConfig
Rate limiting configuration.

```go
type RateLimiterConfig struct {
    MaxAttempts   int           // Max requests per window
    WindowSize    time.Duration // Time window for counting
    LockoutPeriod time.Duration // Lockout duration after limit
}
```

### APIKeySettings
API key configuration.

```go
type APIKeySettings struct {
    Role      string   // User role (admin, user, etc.)
    Scopes    []string // Permission scopes
    TenantID  string   // Tenant identifier
    ExpiresIn string   // Duration string (e.g., "30d")
}
```

### User
Authenticated user information.

```go
type User struct {
    ID       string   `json:"id"`
    TenantID string   `json:"tenant_id"`
    Email    string   `json:"email,omitempty"`
    Scopes   []string `json:"scopes,omitempty"`
    AuthType Type     `json:"auth_type"`
}
```

### AuditEvent
Authentication audit event.

```go
type AuditEvent struct {
    Timestamp time.Time `json:"timestamp"`
    UserID    string    `json:"user_id,omitempty"`
    TenantID  string    `json:"tenant_id,omitempty"`
    AuthType  string    `json:"auth_type"`
    Success   bool      `json:"success"`
    IPAddress string    `json:"ip_address"`
    UserAgent string    `json:"user_agent,omitempty"`
    Error     string    `json:"error,omitempty"`
    Metadata  map[string]interface{} `json:"metadata,omitempty"`
}
```

## Middleware Functions

### GinMiddleware
Authentication middleware for Gin framework.

```go
func (m *AuthMiddleware) GinMiddleware() gin.HandlerFunc
```

**Features:**
- Automatic rate limiting for auth endpoints
- API key and JWT validation
- User context injection
- Metrics collection
- Audit logging

**Example:**
```go
router := gin.New()
router.Use(authMiddleware.GinMiddleware())

// Access user in handler
func handler(c *gin.Context) {
    user, ok := auth.GetUserFromContext(c)
    if !ok {
        c.JSON(401, gin.H{"error": "unauthorized"})
        return
    }
}
```

### StandardMiddleware
Authentication middleware for standard HTTP handlers.

```go
func (s *Service) StandardMiddleware(authTypes ...Type) func(http.Handler) http.Handler
```

**Example:**
```go
mux := http.NewServeMux()
authMiddleware := authService.StandardMiddleware(auth.TypeAPIKey, auth.TypeJWT)
mux.Handle("/api/", authMiddleware(apiHandler))
```

### RequireScopes
Scope-based authorization middleware.

```go
func (s *Service) RequireScopes(scopes ...string) gin.HandlerFunc
```

**Example:**
```go
adminRoutes := router.Group("/admin")
adminRoutes.Use(
    authMiddleware.GinMiddleware(),
    authService.RequireScopes("admin"),
)
```

## Rate Limiting API

### CheckLimit
Check if request should be rate limited.

```go
func (rl *RateLimiter) CheckLimit(ctx context.Context, identifier string) error
```

**Returns:**
- `nil`: Request allowed
- `error`: Request should be rate limited

### RecordAttempt
Record authentication attempt for rate limiting.

```go
func (rl *RateLimiter) RecordAttempt(ctx context.Context, identifier string, success bool)
```

**Parameters:**
- `identifier`: Client identifier (IP, user ID, etc.)
- `success`: Whether authentication succeeded

### GetLockoutPeriod
Get configured lockout period.

```go
func (rl *RateLimiter) GetLockoutPeriod() time.Duration
```

## Metrics API

### RecordAuthAttempt
Record authentication attempt metric.

```go
func (m *MetricsCollector) RecordAuthAttempt(
    ctx context.Context,
    authType string,
    success bool,
    duration time.Duration,
)
```

### RecordRateLimitExceeded
Record rate limit violation.

```go
func (m *MetricsCollector) RecordRateLimitExceeded(
    ctx context.Context,
    identifier string,
)
```

### Available Metrics

| Metric Name | Type | Labels | Description |
|------------|------|--------|-------------|
| `auth_attempts_total` | Counter | `type`, `status` | Total authentication attempts |
| `auth_duration_seconds` | Histogram | `type` | Authentication processing time |
| `auth_rate_limit_exceeded_total` | Counter | `identifier` | Rate limit violations |
| `auth_active_sessions` | Gauge | `type` | Active authenticated sessions |
| `auth_api_keys_total` | Gauge | `status` | Total API keys by status |

## Error Codes

### Authentication Errors

| Error | HTTP Status | Description |
|-------|-------------|-------------|
| `ErrInvalidCredentials` | 401 | Invalid API key or JWT |
| `ErrTokenExpired` | 401 | JWT token has expired |
| `ErrNoAPIKey` | 401 | No API key provided |
| `ErrInvalidAPIKey` | 401 | API key format invalid |
| `ErrInsufficientScope` | 403 | Missing required scopes |
| `ErrUnauthorized` | 401 | Generic unauthorized |

### Rate Limiting Errors

| Error | HTTP Status | Description |
|-------|-------------|-------------|
| `rate limit exceeded: locked out` | 429 | Client is locked out |
| `rate limit exceeded: too many attempts` | 429 | Too many requests |

### Response Examples

**401 Unauthorized:**
```json
{
  "error": "Authentication required",
  "code": "UNAUTHORIZED",
  "request_id": "req-123"
}
```

**403 Forbidden:**
```json
{
  "error": "Insufficient permissions",
  "code": "FORBIDDEN",
  "required_scopes": ["admin"],
  "user_scopes": ["read"]
}
```

**429 Too Many Requests:**
```json
{
  "error": "Too many authentication attempts",
  "code": "RATE_LIMITED",
  "retry_after": 300
}
```

## Security Best Practices

1. **API Key Security**
   - Use minimum 16-character keys
   - Rotate keys regularly
   - Never log full API keys
   - Use HTTPS only

2. **Rate Limiting**
   - Configure per-tenant limits
   - Monitor rate limit metrics
   - Implement gradual backoff

3. **Monitoring**
   - Track failed auth attempts
   - Alert on unusual patterns
   - Review audit logs regularly

4. **Configuration**
   - Use environment variables for secrets
   - Enable caching for performance
   - Configure appropriate timeouts
