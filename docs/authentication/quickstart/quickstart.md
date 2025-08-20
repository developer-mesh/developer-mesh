<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:39:41
Verification Script: update-docs-parallel.sh
Batch: ab
-->

# Authentication Quick Start Guide

Get up and running with Developer Mesh authentication in 5 minutes.

## Prerequisites

- Go 1.24
- Docker and Docker Compose
- Access to PostgreSQL and Redis (or use Docker Compose)

## Quick Start

### 1. Clone and Setup

```bash
# Clone the repository
git clone https://github.com/developer-mesh/developer-mesh.git
cd developer-mesh

# Start dependencies
docker-compose up -d postgres redis

# Run database migrations
make migrate-up
```

### 2. Basic Authentication Setup

Create a minimal authentication service:

```go
package main

import (
    "log"
    
    "github.com/developer-mesh/developer-mesh/pkg/auth"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
    "github.com/gin-gonic/gin"
)

func main() {
    // Create logger
    logger := observability.NewLogger("api")
    
    // Create auth service with defaults
    authConfig := auth.DefaultConfig()
    authConfig.JWTSecret = "your-secret-key-minimum-32-chars"
    authService := auth.NewService(authConfig, nil, nil, logger)
    
    // Add a test API key
    authService.InitializeDefaultAPIKeys(map[string]string{
        "test-key-1234567890": "admin",
    })
    
    // Create Gin router with auth
    router := gin.Default()
    router.Use(authService.GinMiddleware())
    
    // Protected endpoint
    router.GET("/api/v1/hello", func(c *gin.Context) {
        user, _ := auth.GetUserFromContext(c)
        c.JSON(200, gin.H{
            "message": "Hello, authenticated user!",
            "user_id": user.ID,
            "tenant":  user.TenantID,
        })
    })
    
    // Start server
    log.Fatal(router.Run(":8080"))
}
```

### 3. Test Authentication

```bash
# Test without auth (should fail)
curl http://localhost:8080/api/v1/hello
# Response: {"error":"Authentication required"}

# Test with API key
curl -H "Authorization: Bearer test-key-1234567890" \
     http://localhost:8080/api/v1/hello
# Response: {"message":"Hello, authenticated user!","user_id":"system","tenant":"default"}
```

## Enhanced Authentication Setup

Developer Mesh provides both API key and JWT token authentication with dedicated endpoints for user management.

### 1. Full Configuration

```go
package main

import (
    "log"
    "time"
    
    "github.com/developer-mesh/developer-mesh/pkg/auth"
    "github.com/developer-mesh/developer-mesh/pkg/common/cache"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
    "github.com/gin-gonic/gin"
    "github.com/jmoiron/sqlx"
)

func main() {
    // Setup dependencies
    db, _ := sqlx.Connect("postgres", "postgres://localhost/devops_mcp")
    cache := cache.NewRedisCache("localhost:6379", 0)
    logger := observability.NewLogger("api")
    metrics := observability.NewPrometheusMetricsClient()
    
    // Create enhanced auth configuration
    config := &auth.AuthSystemConfig{
        Service: &auth.ServiceConfig{
            JWTSecret:         "your-production-secret-min-32-chars",
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
            "prod-key-minimum-16-chars": {
                Role:     "admin",
                Scopes:   []string{"read", "write", "admin"},
                TenantID: "tenant-123",
            },
        },
    }
    
    // Setup enhanced authentication
    authMiddleware, err := auth.SetupAuthenticationWithConfig(
        config, db, cache, logger, metrics,
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // Create router
    router := gin.Default()
    
    // Apply enhanced auth middleware
    router.Use(authMiddleware)
    
    // Add your application routes here
    
    // Start server
    log.Fatal(router.Run(":8080"))
}

func setupRoutes(router *gin.Engine) {
    v1 := router.Group("/api/v1")
    
    // All endpoints are protected by default
    v1.GET("/profile", func(c *gin.Context) {
        user, _ := auth.GetUserFromContext(c)
        c.JSON(200, user)
    })
    
    // Example: Check user role
    v1.GET("/admin/users", func(c *gin.Context) {
        user, _ := auth.GetUserFromContext(c)
        if user.Role != "admin" {
            c.JSON(403, gin.H{"error": "insufficient permissions"})
            return
        }
        // Handle admin request
    })
}
```

### 2. Testing Enhanced Features

```bash
# Test rate limiting
for i in {1..101}; do
  curl -H "Authorization: Bearer invalid-key" \
       http://localhost:8080/api/v1/profile
done
# After 100 requests: {"error":"Too many authentication attempts"}

# Check rate limit headers
curl -i -H "Authorization: Bearer prod-key-minimum-16-chars" \
     http://localhost:8080/api/v1/profile
# When rate limited, you'll see:
# X-RateLimit-Remaining: 0
# Retry-After: 900

# Login to get JWT tokens
curl -X POST http://localhost:8081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@company.com", "password": "SecurePass123!"}'

# Use the JWT token from the response
curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
     http://localhost:8081/api/v1/profile
```

## Common Integration Patterns

### 1. Multi-Tenant API

```go
func handleListResources(c *gin.Context) {
    // Get tenant from auth context
    tenantID, ok := auth.GetTenantFromContext(c)
    if !ok {
        c.JSON(400, gin.H{"error": "missing tenant"})
        return
    }
    
    // Filter resources by tenant
    resources, err := db.Query(
        "SELECT * FROM resources WHERE tenant_id = $1",
        tenantID,
    )
    
    c.JSON(200, resources)
}
```

### 2. Scope-Based Authorization

```go
// Middleware for scope checking
func requireScopes(scopes ...string) gin.HandlerFunc {
    return func(c *gin.Context) {
        user, ok := auth.GetUserFromContext(c)
        if !ok {
            c.JSON(401, gin.H{"error": "unauthorized"})
            c.Abort()
            return
        }
        
        // Check if user has required scopes
        for _, required := range scopes {
            hasScope := false
            for _, userScope := range user.Scopes {
                if userScope == required {
                    hasScope = true
                    break
                }
            }
            if !hasScope {
                c.JSON(403, gin.H{
                    "error": "insufficient permissions",
                    "required": scopes,
                })
                c.Abort()
                return
            }
        }
        
        c.Next()
    }
}

// Usage
router.DELETE("/api/v1/users/:id", 
    authMiddleware.GinMiddleware(),
    requireScopes("admin", "user:delete"),
    handleDeleteUser,
)
```

### 3. Custom Authentication Flow

```go
func handleLogin(c *gin.Context) {
    var req struct {
        Username string `json:"username"`
        Password string `json:"password"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "invalid request"})
        return
    }
    
    // Validate credentials (implement your logic)
    user, err := validateUserCredentials(req.Username, req.Password)
    if err != nil {
        c.JSON(401, gin.H{"error": "invalid credentials"})
        return
    }
    
    // Generate JWT token
    authUser := &auth.User{
        ID:       user.ID,
        TenantID: user.TenantID,
        Email:    user.Email,
        Scopes:   user.Scopes,
    }
    
    token, err := authService.GenerateJWT(c.Request.Context(), authUser)
    if err != nil {
        c.JSON(500, gin.H{"error": "token generation failed"})
        return
    }
    
    c.JSON(200, gin.H{
        "access_token": token,
        "token_type":   "Bearer",
        "expires_in":   86400, // 24 hours
    })
}
```

### 4. Webhook Authentication

```go
func handleGitHubWebhook(c *gin.Context) {
    // Verify webhook signature
    signature := c.GetHeader("X-Hub-Signature-256")
    body, _ := io.ReadAll(c.Request.Body)
    c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
    
    if !verifyGitHubSignature(body, signature, webhookSecret) {
        c.JSON(401, gin.H{"error": "invalid signature"})
        return
    }
    
    // Process webhook
    var event GitHubEvent
    if err := c.ShouldBindJSON(&event); err != nil {
        c.JSON(400, gin.H{"error": "invalid payload"})
        return
    }
    
    // Handle event...
}
```

## Production Checklist

Before going to production:

- [ ] Use strong JWT secret (32+ characters)
- [ ] Enable HTTPS/TLS
- [ ] Configure rate limiting per tenant
- [ ] Set up monitoring and alerting
- [ ] Enable audit logging
- [ ] Implement key rotation policy
- [ ] Configure database connection pooling
- [ ] Set up Redis for distributed rate limiting
- [ ] Test failover scenarios
- [ ] Document API key distribution process

## Next Steps

1. Review the [full authentication documentation](../developer/authentication-implementation-guide.md)
2. Set up [monitoring and metrics](../operations/authentication-operations-guide.md)
3. Configure [production deployment](../api-reference/authentication-api-reference.md)
4. Implement [custom auth providers](../examples/custom-auth-provider.md)

## Getting Help

- Check the [troubleshooting guide](../operations/authentication-operations-guide.md#troubleshooting)
- Review [example implementations](../examples/)
- Submit issues at [GitHub Issues](https://github.com/developer-mesh/developer-mesh/issues)

<!-- VERIFICATION
This document has been automatically verified against the codebase.
Last verification: 2025-08-11 14:39:41
All features mentioned have been confirmed to exist in the code.
-->
