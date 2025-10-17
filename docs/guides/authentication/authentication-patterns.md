<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:30:55
Verification Script: update-docs-parallel.sh
Batch: ab
-->

# Authentication Patterns and Best Practices

## Overview
This guide demonstrates authentication patterns implemented in Developer Mesh.

## Authentication Methods

### 1. API Key Authentication
Best for: Service-to-service communication, CI/CD pipelines, long-lived integrations

```bash
# API keys are generated during organization registration
# You receive an API key when registering:
curl -X POST http://localhost:8081/api/v1/auth/register/organization \
  -H "Content-Type: application/json" \
  -d '{
    "organization_name": "My Company",
    "organization_slug": "my-company",
    "admin_email": "admin@company.com",
    "admin_name": "Admin",
    "admin_password": "SecurePass123!"
  }'
# Response includes: "api_key": "devmesh_xxxxx"

# Use the API key
curl -X GET http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer devmesh_xxxxx"
```

### 2. JWT Token Authentication
Best for: User sessions, web applications, mobile apps

```javascript
// Example: Browser-based authentication
async function login(email, password) {
  const response = await fetch('http://localhost:8081/api/v1/auth/login', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ email, password }),  // Note: uses email, not username
  });
  
  const data = await response.json();
  
  // Store tokens securely
  localStorage.setItem('access_token', data.access_token);
  localStorage.setItem('refresh_token', data.refresh_token);
  
  // Set default header for all requests
  axios.defaults.headers.common['Authorization'] = `Bearer ${data.access_token}`;
}

// Note: Token refresh endpoint is not yet implemented
// The /api/v1/auth/refresh endpoint returns 501 Not Implemented
// For now, users must login again when token expires
```

### 3. Organization User Invitations
Best for: Adding team members to your organization

```python
# Example: Invite users to organization
import requests

class OrganizationManager:
    def __init__(self, api_key):
        self.api_key = api_key
        self.base_url = "http://localhost:8081"
    
    def invite_user(self, email, name, role='member'):
        """Invite a user to the organization"""
        response = requests.post(
            f"{self.base_url}/api/v1/auth/users/invite",
            headers={
                'Authorization': f'Bearer {self.api_key}',
                'Content-Type': 'application/json'
            },
            json={
                'email': email,
                'name': name,
                'role': role  # 'admin', 'member', or 'readonly'
            }
        )
        return response.json()
    
    # Note: OAuth endpoints are not implemented
    # The following is planned but not functional:
    # def exchange_code_for_token(self, code):
    #     response = requests.post(f"{self.base_url}/oauth/token", data={
    #         'grant_type': 'authorization_code',
    #         'code': code,
    #         'redirect_uri': self.redirect_uri,
    #         'client_id': self.client_id,
    #         'client_secret': self.client_secret
    #     })
    #     return response.json()
```

### 4. Multi-Tenant Authentication
Best for: SaaS applications, enterprise deployments

```go
// Example: Tenant isolation middleware
func TenantIsolationMiddleware(authManager *auth.Manager) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Extract tenant from auth
        claims, exists := c.Get("claims")
        if !exists {
            c.JSON(401, gin.H{"error": "unauthorized"})
            c.Abort()
            return
        }
        
        tenantID := claims.(*auth.Claims).TenantID
        
        // Validate tenant access
        resource := c.Param("resource")
        if !authManager.HasTenantAccess(c.Request.Context(), tenantID, resource) {
            c.JSON(403, gin.H{"error": "forbidden"})
            c.Abort()
            return
        }
        
        // Add tenant to context for downstream use
        c.Set("tenant_id", tenantID)
        ctx := context.WithValue(c.Request.Context(), "tenant_id", tenantID)
        c.Request = c.Request.WithContext(ctx)
        
        c.Next()
    }
}
```

## Security Best Practices

### 1. Token Rotation
```bash
# Rotate API keys periodically
curl -X POST http://localhost:8081/api/v1/auth/keys/{key_id}/rotate \
  -H "Authorization: Bearer $ADMIN_JWT"
```

### 2. Scope-Based Access Control
```yaml
# Define minimal scopes for each service
services:
  ai_agent:
    scopes:
      - contexts:read
      - contexts:write
      - tools:execute:github
  
  monitoring:
    scopes:
      - contexts:read
      - metrics:read
  
  admin:
    scopes:
      - "*:*"  # Full access
```

### 3. Rate Limiting Configuration
```json
{
  "rate_limits": {
    "default": {
      "requests_per_minute": 60,
      "burst": 120
    },
    "authenticated": {
      "requests_per_minute": 1000,
      "burst": 3000
    },
    "premium": {
      "requests_per_minute": 10000,
      "burst": 30000
    }
  }
}
```

## Common Integration Patterns

### 1. CI/CD Pipeline Integration
```yaml
# .github/workflows/deploy.yml
name: Deploy with MCP
on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Deploy context to MCP
        env:
          MCP_API_KEY: ${{ secrets.MCP_API_KEY }}
          MCP_TENANT_ID: ${{ secrets.MCP_TENANT_ID }}
        run: |
          curl -X POST https://api.mcp.example.com/api/v1/contexts \
            -H "X-API-Key: $MCP_API_KEY" \
            -H "X-Tenant-ID: $MCP_TENANT_ID" \
            -H "Content-Type: application/json" \
            -d @deployment-context.json
```

### 2. SDK Authentication
```typescript
// TypeScript SDK example
import { MCPClient } from '@developer-mesh/sdk';

const client = new MCPClient({
  baseURL: 'https://api.mcp.example.com',
  auth: {
    type: 'api-key',
    apiKey: process.env.MCP_API_KEY,
  },
  tenantId: process.env.MCP_TENANT_ID,
  retryConfig: {
    maxRetries: 3,
    backoff: 'exponential',
  },
});

// Automatic token refresh for JWT auth
const jwtClient = new MCPClient({
  baseURL: 'https://api.mcp.example.com',
  auth: {
    type: 'jwt',
    username: 'user@example.com',
    password: 'secure-password',
    autoRefresh: true,
  },
});
```

## Troubleshooting Authentication Issues

### Common Errors and Solutions

| Error | Cause | Solution |
|-------|-------|----------|
| `401: Invalid API Key` | Expired or revoked key | Generate new API key |
| `403: Insufficient Scope` | Missing required permissions | Add required scopes to key |
| `429: Rate Limit Exceeded` | Too many requests | Implement exponential backoff |
| `401: Token Expired` | JWT token expired | Refresh token or re-authenticate |

### Debug Authentication
```bash
# Validate JWT token
curl -X POST http://localhost:8081/api/v1/auth/validate \
  -H "Authorization: Bearer $TOKEN"

# Check API key permissions
curl -X GET http://localhost:8081/api/v1/auth/keys/{key_id} \
  -H "Authorization: Bearer $ADMIN_JWT"

# View rate limit status
curl -I http://localhost:8081/api/v1/contexts \
  -H "X-API-Key: $API_KEY"
# Check headers: X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset
```

## Migration Guide

### Migrating from Basic Auth to Enhanced Auth

1. **Generate API Keys for Services**
```bash
# For each service using basic auth
for service in ai-agent monitoring analytics; do
  curl -X POST http://localhost:8081/api/v1/auth/keys \
    -H "Authorization: Bearer $ADMIN_JWT" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"$service-migration\",
      \"scopes\": [\"contexts:read\", \"contexts:write\"],
      \"tenant_id\": \"$service-tenant\"
    }"
done
```

2. **Update Service Configurations**
```yaml
# Before
auth:
  type: basic
  username: service
  password: password

# After
auth:
  type: api-key
  api_key: ${MCP_API_KEY}
  tenant_id: ${MCP_TENANT_ID}
```

3. **Test and Validate**
```bash
# Test new authentication
curl -X GET http://localhost:8081/api/v1/health \
  -H "X-API-Key: $NEW_API_KEY"
```

## Next Steps

- Review [API Reference](/docs/api-reference/authentication-api-reference.md) for complete endpoint documentation
- Set up [Monitoring](/docs/operations/authentication-operations-guide.md) for auth metrics
