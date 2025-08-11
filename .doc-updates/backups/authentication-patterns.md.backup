# Authentication Patterns and Best Practices

## Overview
This guide demonstrates common authentication patterns using the Developer Mesh enhanced authentication system.

## Authentication Methods

### 1. API Key Authentication
Best for: Service-to-service communication, CI/CD pipelines, long-lived integrations

```bash
# Generate an API key
curl -X POST http://localhost:8081/api/v1/auth/keys \
  -H "Authorization: Bearer $ADMIN_JWT" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "ci-pipeline",
    "scopes": ["contexts:read", "tools:execute"],
    "tenant_id": "ci-tenant",
    "expires_at": "2026-01-01T00:00:00Z"
  }'

# Use the API key
curl -X GET http://localhost:8081/api/v1/contexts \
  -H "X-API-Key: mcp_k_..." \
  -H "X-Tenant-ID: ci-tenant"
```

### 2. JWT Token Authentication
Best for: User sessions, web applications, mobile apps

```javascript
// Example: Browser-based authentication
async function login(username, password) {
  const response = await fetch('http://localhost:8081/api/v1/auth/login', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ username, password }),
  });
  
  const data = await response.json();
  
  // Store tokens securely
  localStorage.setItem('access_token', data.access_token);
  localStorage.setItem('refresh_token', data.refresh_token);
  
  // Set default header for all requests
  axios.defaults.headers.common['Authorization'] = `Bearer ${data.access_token}`;
}

// Refresh token when expired
async function refreshToken() {
  const refreshToken = localStorage.getItem('refresh_token');
  const response = await fetch('http://localhost:8081/api/v1/auth/refresh', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ refresh_token: refreshToken }),
  });
  
  const data = await response.json();
  localStorage.setItem('access_token', data.access_token);
  return data.access_token;
}
```

### 3. OAuth2 Integration
Best for: Third-party integrations, social login

```python
# Example: Python OAuth2 client
import requests
from urllib.parse import urlencode

class MCPOAuth2Client:
    def __init__(self, client_id, client_secret, redirect_uri):
        self.client_id = client_id
        self.client_secret = client_secret
        self.redirect_uri = redirect_uri
        self.base_url = "http://localhost:8081"
    
    def get_authorization_url(self, state):
        params = {
            'client_id': self.client_id,
            'redirect_uri': self.redirect_uri,
            'response_type': 'code',
            'scope': 'contexts:read contexts:write',
            'state': state
        }
        return f"{self.base_url}/oauth/authorize?{urlencode(params)}"
    
    def exchange_code_for_token(self, code):
        response = requests.post(f"{self.base_url}/oauth/token", data={
            'grant_type': 'authorization_code',
            'code': code,
            'redirect_uri': self.redirect_uri,
            'client_id': self.client_id,
            'client_secret': self.client_secret
        })
        return response.json()
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
- Implement [Security Best Practices](/docs/SECURITY.md) for production