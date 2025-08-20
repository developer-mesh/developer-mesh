# Authentication Configuration

## Overview

Developer Mesh supports multiple authentication methods:
- **JWT Tokens**: For user sessions and API access
- **API Keys**: For service-to-service and programmatic access
- **OAuth 2.0**: Interface defined, provider implementations pending

## JWT Configuration

### JWT Settings

```yaml
auth:
  jwt:
    # Secret key for signing tokens (minimum 32 characters)
    secret: ${JWT_SECRET}
    
    # Algorithm for token signing
    algorithm: "HS256"          # Options: HS256, HS384, HS512
    
    # Token expiration times
    expiration: 24h             # Access token expiration
    refresh_expiration: 7d      # Refresh token expiration
    
    # Token settings
    issuer: "developer-mesh"    # Token issuer
    audience: "api"             # Token audience
    
    # Refresh token settings
    refresh_enabled: true       # Enable refresh tokens
    refresh_rotation: true      # Rotate refresh tokens on use
```

### Environment Variables

```bash
# JWT Configuration
JWT_SECRET=your-secret-key-minimum-32-characters
JWT_EXPIRATION=24h
JWT_REFRESH_EXPIRATION=7d
JWT_ISSUER=developer-mesh
```

### JWT Token Structure

```json
{
  "sub": "user-id",
  "email": "user@example.com",
  "tenant_id": "00000000-0000-0000-0000-000000000001",
  "role": "admin",
  "scopes": ["read", "write", "admin"],
  "iat": 1234567890,
  "exp": 1234654290,
  "iss": "developer-mesh",
  "aud": "api"
}
```

## API Key Configuration

### API Key Settings

```yaml
auth:
  api_keys:
    # Header name for API key
    header: "X-API-Key"
    
    # Alternative header (fallback)
    alt_header: "Authorization"
    
    # Database storage
    enable_database: true       # Store keys in database
    cache_ttl: 5m              # Cache valid keys for 5 minutes
    
    # Key requirements
    min_length: 16             # Minimum key length
    key_prefix: "dm_"          # Key prefix (optional)
    
    # Key rotation
    rotation_enabled: true     # Enable key rotation
    rotation_period: 90d       # Rotation period
    rotation_warning: 7d       # Warning before expiration
```

### Static API Keys (Development Only)

```yaml
auth:
  api_keys:
    static_keys:
      # Admin key with full access
      "dev-admin-key-1234567890":
        role: "admin"
        scopes: ["read", "write", "admin"]
        tenant_id: "00000000-0000-0000-0000-000000000001"
        
      # Read-only key
      "dev-readonly-key-1234567890":
        role: "reader"
        scopes: ["read"]
        tenant_id: "00000000-0000-0000-0000-000000000001"
        
      # Service-specific keys
      "mcp-service-key":
        role: "service"
        scopes: ["read", "write"]
        tenant_id: "00000000-0000-0000-0000-000000000001"
        service: "mcp-server"
```

### Production API Keys

API keys in production are stored in the database:

```sql
-- API keys table
CREATE TABLE mcp.api_keys (
    id UUID PRIMARY KEY,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    key_prefix VARCHAR(8) NOT NULL,
    organization_id UUID NOT NULL,
    name VARCHAR(255),
    role VARCHAR(50),
    scopes TEXT[],
    expires_at TIMESTAMP,
    last_used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    created_by UUID,
    is_active BOOLEAN DEFAULT true
);
```

## Organization Authentication

### Registration Configuration

```yaml
auth:
  registration:
    # Registration settings
    enabled: true               # Allow new registrations
    require_email_verification: true
    email_verification_ttl: 24h
    
    # Password requirements
    password:
      min_length: 8
      require_uppercase: true
      require_lowercase: true
      require_number: true
      require_special: false
      
    # Organization settings
    organization:
      slug_pattern: "^[a-z0-9][a-z0-9-]{2,49}$"
      max_users: 5              # Default max users
      max_agents: 10            # Default max agents
      
    # Invitation settings
    invitation:
      enabled: true
      ttl: 7d                  # Invitation expiration
      max_pending: 10          # Max pending invitations
```

### User Roles

```yaml
auth:
  roles:
    # Role definitions
    owner:
      permissions: ["*"]       # All permissions
      can_invite: true
      can_manage_billing: true
      
    admin:
      permissions: ["read", "write", "invite", "manage_users"]
      can_invite: true
      can_manage_billing: false
      
    member:
      permissions: ["read", "write"]
      can_invite: false
      can_manage_billing: false
      
    readonly:
      permissions: ["read"]
      can_invite: false
      can_manage_billing: false
```

## Security Settings

### Rate Limiting

```yaml
auth:
  security:
    # Login rate limiting
    max_failed_attempts: 5     # Failed attempts before lockout
    lockout_duration: 15m      # Account lockout duration
    
    # Password reset
    password_reset:
      enabled: true
      token_ttl: 1h            # Reset token expiration
      rate_limit: 3            # Max reset requests per hour
      
    # Session management
    session:
      max_concurrent: 5        # Max concurrent sessions
      idle_timeout: 30m        # Session idle timeout
      absolute_timeout: 24h    # Maximum session duration
```

### CORS Configuration

```yaml
auth:
  cors:
    enabled: true
    allowed_origins:
      - "http://localhost:3000"
      - "https://app.example.com"
    allowed_methods:
      - GET
      - POST
      - PUT
      - DELETE
      - OPTIONS
    allowed_headers:
      - Authorization
      - Content-Type
      - X-API-Key
      - X-Request-ID
    exposed_headers:
      - X-RateLimit-Limit
      - X-RateLimit-Remaining
      - X-RateLimit-Reset
    allow_credentials: true
    max_age: 86400             # Preflight cache duration
```

## OAuth 2.0 Configuration (Planned)

**Note**: OAuth provider implementations are not yet available. The interface is defined for future implementation.

```yaml
auth:
  oauth2:
    enabled: false              # Not yet implemented
    providers:
      github:
        client_id: ${GITHUB_CLIENT_ID}
        client_secret: ${GITHUB_CLIENT_SECRET}
        redirect_uri: "https://app.example.com/auth/github/callback"
        scopes: ["user:email", "read:org"]
        
      google:
        client_id: ${GOOGLE_CLIENT_ID}
        client_secret: ${GOOGLE_CLIENT_SECRET}
        redirect_uri: "https://app.example.com/auth/google/callback"
        scopes: ["email", "profile"]
```

## Multi-Factor Authentication (Planned)

```yaml
auth:
  mfa:
    enabled: false              # Not yet implemented
    methods:
      totp:
        enabled: true
        issuer: "Developer Mesh"
        
      sms:
        enabled: false
        provider: "twilio"
        
      email:
        enabled: true
        
    # Backup codes
    backup_codes:
      enabled: true
      count: 10
      length: 8
```

## Audit Logging

```yaml
auth:
  audit:
    enabled: true
    
    # Events to log
    events:
      - login_success
      - login_failure
      - logout
      - password_change
      - api_key_created
      - api_key_revoked
      - permission_change
      - account_locked
      
    # Log destinations
    destinations:
      database: true
      file: true
      file_path: "/var/log/devmesh/audit.log"
      
    # Log retention
    retention:
      database: 90d
      file: 30d
```

## Authentication Middleware Configuration

### Middleware Settings

```yaml
auth:
  middleware:
    # Token validation
    validate_token: true
    validate_signature: true
    validate_expiration: true
    
    # Clock skew tolerance
    clock_skew: 5m
    
    # Cache settings
    cache:
      enabled: true
      ttl: 5m
      negative_ttl: 30s        # Cache invalid tokens briefly
      
    # Public endpoints (no auth required)
    public_endpoints:
      - /health
      - /metrics
      - /api/v1/auth/login
      - /api/v1/auth/register/organization
      - /api/v1/auth/password/reset
```

## Environment-Specific Configuration

### Development

```yaml
auth:
  # Relaxed settings for development
  require_auth: false           # Optional authentication
  jwt:
    secret: "dev-jwt-secret-minimum-32-characters"
    expiration: 1h              # Shorter for testing
  api_keys:
    static_keys:                # Use static keys
      "dev-key": {...}
  cors:
    allowed_origins: ["*"]      # Allow all origins
```

### Production

```yaml
auth:
  # Strict settings for production
  require_auth: true            # Always require auth
  jwt:
    secret: ${JWT_SECRET}       # From environment
    expiration: 24h
  api_keys:
    enable_database: true       # Database-only keys
    static_keys: {}             # No static keys
  cors:
    allowed_origins:            # Specific origins only
      - "https://app.example.com"
  security:
    max_failed_attempts: 3      # Stricter limits
    lockout_duration: 30m
```

## Testing Authentication

### Generate Test JWT

```bash
# Generate JWT secret
export JWT_SECRET=$(openssl rand -base64 32)

# Create test token (requires jwt-cli)
jwt encode \
  --secret "$JWT_SECRET" \
  --iss "developer-mesh" \
  --sub "test-user" \
  --exp "+24h" \
  '{"tenant_id":"00000000-0000-0000-0000-000000000001","role":"admin"}'
```

### Test API Key

```bash
# Test with API key
curl -H "X-API-Key: dev-admin-key-1234567890" \
  http://localhost:8081/api/v1/contexts

# Test with JWT
curl -H "Authorization: Bearer $JWT_TOKEN" \
  http://localhost:8081/api/v1/contexts
```

## Troubleshooting

### Authentication Failures

```bash
# Check JWT secret is set
echo $JWT_SECRET | wc -c  # Should be >= 32

# Verify API key exists
curl -H "X-API-Key: your-key" http://localhost:8081/health

# Check auth logs
grep -i auth /var/log/devmesh/app.log
```

### Token Issues

```bash
# Decode JWT token
echo $JWT_TOKEN | jwt decode -

# Verify token signature
jwt verify --secret "$JWT_SECRET" "$JWT_TOKEN"

# Check token expiration
jwt decode "$JWT_TOKEN" | jq .exp
```

## Security Best Practices

1. **Use strong secrets**: Minimum 32 characters for JWT secret
2. **Rotate keys regularly**: Implement key rotation policy
3. **Use HTTPS only**: Never send auth tokens over HTTP in production
4. **Implement rate limiting**: Prevent brute force attacks
5. **Enable audit logging**: Track all authentication events
6. **Use short token expiration**: Balance security and usability
7. **Implement refresh tokens**: For better user experience
8. **Validate on every request**: Don't trust cached authentication
9. **Use database for API keys**: Never hardcode in production
10. **Monitor failed attempts**: Detect and respond to attacks

## Related Documentation

- [Environment Variables Reference](../ENVIRONMENT_VARIABLES.md)
- [Organization Setup Guide](../guides/organization-setup-guide.md)
- [API Key Management](../operations/api-key-management.md)
- [Security Best Practices](../security/)