<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:27:57
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# Authentication API Reference

## Overview

Developer Mesh provides a complete authentication and authorization system with organization registration, user management, and multi-tenant support.

## Table of Contents

1. [Authentication Endpoints](#authentication-endpoints)
2. [Organization Management](#organization-management)
3. [User Management](#user-management)
4. [Authentication Methods](#authentication-methods)
5. [Error Codes](#error-codes)

## Authentication Endpoints

### Public Endpoints (No Authentication Required)

These endpoints are accessible without authentication:

#### POST /api/v1/auth/register/organization

Register a new organization with an admin user.

**Request Body:**
```json
{
  "organization_name": "string",  // Required, 3-100 chars
  "organization_slug": "string",  // Required, 3-50 chars, lowercase + hyphens
  "admin_email": "string",        // Required, valid email
  "admin_name": "string",         // Required, 2-100 chars
  "admin_password": "string",     // Required, min 8 chars, uppercase + lowercase + number
  "company_size": "string",       // Optional
  "industry": "string",           // Optional
  "use_case": "string"            // Optional
}
```

**Success Response (201 Created):**
```json
{
  "organization": {
    "id": "uuid",
    "name": "string",
    "slug": "string",
    "subscription_tier": "free",
    "max_users": 5,
    "max_agents": 10
  },
  "user": {
    "id": "uuid",
    "email": "string",
    "name": "string",
    "role": "owner",
    "email_verified": false
  },
  "api_key": "devmesh_xxxxx",
  "message": "Organization registered successfully. Please check your email to verify your account."
}
```

**Error Responses:**
- `400 Bad Request`: Invalid slug format or password validation failed
- `409 Conflict`: Organization slug or email already exists

#### POST /api/v1/auth/login

Authenticate user with email and password.

**Request Body:**
```json
{
  "email": "string",
  "password": "string"
}
```

**Success Response (200 OK):**
```json
{
  "access_token": "string",
  "refresh_token": "string",
  "token_type": "Bearer",
  "expires_in": 86400,
  "user": {
    "id": "uuid",
    "email": "string",
    "name": "string",
    "role": "string",
    "organization_id": "uuid"
  }
}
```

**Error Response (401 Unauthorized):**
```json
{
  "error": "Invalid email or password"
}
```

#### POST /api/v1/auth/invitation/accept

Accept an invitation and create user account.

**Request Body:**
```json
{
  "token": "string",      // Invitation token from email
  "password": "string"    // New user's password
}
```

**Success Response (201 Created):**
```json
{
  "access_token": "string",
  "refresh_token": "string",
  "token_type": "Bearer",
  "expires_in": 86400,
  "user": {
    "id": "uuid",
    "email": "string",
    "name": "string",
    "role": "string"
  }
}
```

#### POST /api/v1/auth/edge-mcp

Authenticate Edge MCP instances.

**Request Body:**
```json
{
  "edge_mcp_id": "string",
  "api_key": "string"
}
```

**Success Response (200 OK):**
```json
{
  "success": true,
  "token": "string",
  "tenant_id": "uuid"
}
```

### Protected Endpoints (Authentication Required)

#### POST /api/v1/auth/users/invite

Invite a new user to your organization. Requires `admin` or `owner` role.

**Request Body:**
```json
{
  "email": "string",
  "name": "string",
  "role": "admin|member|readonly"  // Cannot invite as owner
}
```

**Success Response (200 OK):**
```json
{
  "message": "Invitation sent successfully",
  "email": "string"
}
```

**Error Responses:**
- `403 Forbidden`: Insufficient permissions
- `409 Conflict`: User already exists or invitation already sent

## Authentication Methods

### API Key Authentication

API keys are generated during organization registration and can be used for authentication:

**Authorization Header:**
```
Authorization: Bearer devmesh_xxxxxxxxxxxxx
```

**X-API-Key Header:**
```
X-API-Key: devmesh_xxxxxxxxxxxxx
```

### JWT Token Authentication

JWT tokens are obtained through the login endpoint:

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

### User Roles and Permissions

| Role | Description | Permissions |
|------|-------------|-------------|
| `owner` | Organization owner | Full control, can delete organization |
| `admin` | Administrator | Manage users, settings, API keys |
| `member` | Standard user | Access resources, create/modify own items |
| `readonly` | Read-only user | View resources only |

## Organization Management

### Organization Tiers

| Tier | Max Users | Max Agents | Description |
|------|-----------|------------|-------------|
| `free` | 5 | 10 | Default tier for new organizations |
| `starter` | 25 | 50 | Small teams |
| `pro` | 100 | 200 | Growing organizations |
| `enterprise` | Unlimited | Unlimited | Large enterprises |

## Error Codes

### Authentication Errors

| Status Code | Error | Description |
|-------------|-------|-------------|
| 401 | `unauthorized` | Missing or invalid authentication |
| 401 | `Invalid email or password` | Login credentials incorrect |
| 403 | `insufficient permissions` | User lacks required role/scope |
| 409 | `organization slug already exists` | Slug taken during registration |
| 409 | `email already registered` | Email already in use |
| 429 | `Too many authentication attempts` | Rate limit exceeded |

### Password Validation Rules

- Minimum 8 characters
- Must contain at least one uppercase letter (A-Z)
- Must contain at least one lowercase letter (a-z)
- Must contain at least one number (0-9)

### Organization Slug Rules

- 3-50 characters
- Lowercase letters, numbers, and hyphens only
- Must start with a letter or number
- Format: `^[a-z0-9][a-z0-9-]{2,49}$`

## Placeholder Endpoints (Not Yet Implemented)

The following endpoints return "Not implemented yet" (501):

- `POST /api/v1/auth/refresh` - Refresh JWT token
- `POST /api/v1/auth/logout` - Logout user
- `POST /api/v1/auth/password/reset` - Request password reset
- `POST /api/v1/auth/password/reset/confirm` - Confirm password reset
- `POST /api/v1/auth/email/verify` - Verify email address
- `POST /api/v1/auth/email/resend` - Resend verification email
- `GET /api/v1/auth/invitation/:token` - Get invitation details
- `GET /api/v1/auth/users` - List organization users
- `PUT /api/v1/auth/users/:id/role` - Update user role
- `DELETE /api/v1/auth/users/:id` - Remove user from organization
- `GET /api/v1/auth/organization` - Get organization details
- `PUT /api/v1/auth/organization` - Update organization
- `GET /api/v1/auth/organization/usage` - Get usage statistics
- `GET /api/v1/auth/profile` - Get user profile
- `PUT /api/v1/auth/profile` - Update user profile
- `POST /api/v1/auth/profile/password` - Change password
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
