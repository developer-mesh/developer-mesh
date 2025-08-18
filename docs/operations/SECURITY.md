<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:34:45
Verification Script: update-docs-parallel.sh
Batch: ad
-->

# Security Guide

## Overview
This guide covers the security features implemented in Developer Mesh and best practices for deployment.

## Implementation Status

### ✅ Implemented Security Features
- **TLS 1.2/1.3 Support**: Full TLS configuration with secure cipher suites
- **Authentication**: JWT and API key authentication
- **Basic RBAC**: In-memory role-based access control (admin/user/viewer)
- **Audit Logging**: Comprehensive audit trail for all security events
- **Rate Limiting**: Per-user and per-IP rate limiting with burst support
- **CORS**: Configurable cross-origin resource sharing

### ⚠️ Configuration Required
- TLS certificates must be provided for production
- JWT secrets must be configured
- API keys must be generated and stored securely

### ❌ Not Implemented (Future Roadmap)
- Kubernetes deployment and network policies
- WAF/CDN integration
- HashiCorp Vault integration
- Casbin-based RBAC (currently uses simpler in-memory system)
- Compliance tooling (GDPR, HIPAA, SOC2)
- Container security scanning
- Security headers middleware

## Quick Security Checklist

- [x] TLS 1.3 support available (requires certificate configuration)
- [x] Authentication system (JWT/API Keys) implemented
- [x] Audit logging implemented and enabled by default
- [ ] Network policies (requires Kubernetes deployment)
- [ ] External secrets management (basic env vars only)
- [x] Rate limiting implemented and configurable
- [ ] WAF rules (requires external WAF service)
- [x] Basic RBAC implemented (not Casbin)
- [x] Metrics/monitoring available at /metrics
- [ ] Security scanning (requires CI/CD setup)

## Security Architecture

### Current Implementation (Docker Compose)

```
┌─────────────────────────────────────────────────┐
│            External Access (Port 8080/8081)     │
├─────────────────────────────────────────────────┤
│         Optional: Reverse Proxy/TLS             │
│         (nginx, Caddy, or cloud LB)            │
├─────────────────────────────────────────────────┤
│    ┌─────────┐  ┌─────────┐  ┌─────────┐     │
│    │   MCP   │  │  REST   │  │ Worker  │     │
│    │ Server  │  │   API   │  │         │     │
│    │  :8080  │  │  :8081  │  │ (async) │     │
│    └────┬────┘  └────┬────┘  └────┬────┘     │
├─────────┴────────────┴─────────────┴───────────┤
│          Application Security Layer             │
│    - JWT/API Key Auth                          │
│    - Rate Limiting                             │
│    - Audit Logging                             │
│    - Basic RBAC                                │
├─────────────────────────────────────────────────┤
│    ┌─────────┐  ┌─────────┐  ┌─────────┐     │
│    │Postgres │  │  Redis  │  │   S3    │     │
│    │ (local) │  │(tunnel) │  │  (AWS)  │     │
│    └─────────┘  └─────────┘  └─────────┘     │
└─────────────────────────────────────────────────┘
```

### Future Architecture (Kubernetes)

The following represents a future deployment architecture that is not currently implemented:

```
[Kubernetes architecture diagram - NOT IMPLEMENTED]
Would include: WAF/CDN → Load Balancer → Ingress → 
Service Mesh → Pods → Managed Services
```

## Network Security

### Current Implementation

**Docker Compose Network Isolation**:
- Services communicate via Docker bridge network
- Only exposed ports: 8080 (MCP Server), 8081 (REST API)
- Redis accessed via SSH tunnel for production
- PostgreSQL uses standard port with password auth

### Production Recommendations

1. **Reverse Proxy**: Deploy nginx/Caddy for TLS termination
2. **Firewall Rules**: Configure host firewall (iptables/ufw)
3. **SSH Tunnel**: Use for Redis access in production
4. **VPN/Private Network**: Isolate backend services

### Future: Kubernetes Network Policies

**Note**: The following Kubernetes configurations are examples for future implementation:

```yaml
# EXAMPLE ONLY - Not currently implemented
# This would be used in a Kubernetes deployment
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: mcp-server-ingress
spec:
  podSelector:
    matchLabels:
      app: mcp-server
  # ... rest of policy
```

### TLS Configuration (Implemented)

**Application TLS Support** (`pkg/security/tls/config.go`):
```go
// Actual implementation in codebase
type Config struct {
    Enabled            bool
    MinVersion         string    // "1.2" or "1.3"
    CertFile          string
    KeyFile           string
    ClientAuth        string    // "none", "request", "require", "verify"
    ClientCAFile      string
    SessionTickets    bool
    EnableHTTP2       bool
    StrictSNI         bool
    OCSPStapling      bool
}

// Secure cipher suites configured by default
CipherSuites: []uint16{
    tls.TLS_AES_128_GCM_SHA256,
    tls.TLS_AES_256_GCM_SHA384,
    tls.TLS_CHACHA20_POLY1305_SHA256,
    // ... more secure ciphers
}
```

**To Enable TLS**:
1. Generate or obtain TLS certificates
2. Configure in environment:
   ```bash
   API_TLS_ENABLED=true
   API_TLS_CERT_FILE=/path/to/cert.pem
   API_TLS_KEY_FILE=/path/to/key.pem
   ```

## Data Security

### Encryption at Rest

**Current Implementation**:
- **Application-Level Credential Encryption** (`pkg/security/encryption.go`):
  - AES-256-GCM encryption with per-tenant key derivation
  - All API keys and secrets encrypted before database storage
  - Each tenant has unique encryption key derived from master key + tenant ID + salt
  - PBKDF2 with 10,000 iterations for key derivation
  - Forward secrecy through unique salt per encryption operation
  - Required environment variable:
    - `ENCRYPTION_MASTER_KEY` for all services
- PostgreSQL: Standard encryption (depends on deployment)
- Redis: Data not encrypted at rest by default
- S3: Server-side encryption enabled (SSE-S3)

**Production Recommendations**:
1. Enable PostgreSQL transparent data encryption (TDE) if using RDS
2. Use encrypted EBS volumes for self-managed databases
3. Enable Redis persistence encryption if required
4. Use KMS for S3 encryption keys

### PostgreSQL Security Configuration
```sql
-- Enforce SSL connections (add to postgresql.conf)
ssl = on
ssl_min_protocol_version = 'TLSv1.2'

-- Create user with limited privileges
CREATE USER mcp_app WITH PASSWORD 'secure_password';
GRANT CONNECT ON DATABASE mcp TO mcp_app;
GRANT USAGE ON SCHEMA public TO mcp_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO mcp_app;
```

### Encryption in Transit (Implemented)

**TLS Configuration** (`pkg/security/tls/config.go`):
```go
// Actual implementation with hardware acceleration
func (c *Config) ToTLSConfig() (*tls.Config, error) {
    tlsConfig := &tls.Config{
        MinVersion:         minVersion,
        MaxVersion:         maxVersion,
        CipherSuites:       cipherSuites,
        CurvePreferences:   curves,
        SessionTicketsDisabled: !c.SessionTickets,
        ClientAuth:         clientAuthType,
    }
    
    // Hardware acceleration detection
    if hasAESNI() {
        // Prioritize AES-GCM ciphers
    }
    
    return tlsConfig, nil
}
```

**Database Connections**:
- PostgreSQL: SSL mode configurable via `DATABASE_SSL_MODE`
- Redis: TLS support via `CACHE_TLS_ENABLED`
- S3: HTTPS enforced by AWS SDK

## Secrets Management

### Current Implementation

**Environment Variables**:
- Secrets loaded from environment variables
- `.env` file support for local development
- No external secret management integration

**Best Practices**:
1. Never commit `.env` files (already in .gitignore)
2. Use strong, unique passwords for each environment
3. Rotate API keys and JWT secrets regularly
4. Set restrictive file permissions on `.env` files

### Production Deployment

**Docker Compose**:
```bash
# Create .env.production with secrets
JWT_SECRET=$(openssl rand -base64 32)
API_KEY_ADMIN=$(openssl rand -hex 32)
DB_PASSWORD=$(openssl rand -base64 24)

# Deploy with secrets
docker-compose --env-file .env.production up -d
```

**EC2 Deployment**:
```bash
# Store secrets in AWS Systems Manager Parameter Store
aws ssm put-parameter --name /mcp/jwt-secret --value "$JWT_SECRET" --type SecureString
aws ssm put-parameter --name /mcp/api-key --value "$API_KEY" --type SecureString

# Retrieve in application
JWT_SECRET=$(aws ssm get-parameter --name /mcp/jwt-secret --with-decryption --query 'Parameter.Value' --output text)
```

### Future: External Secret Management

The following integrations are planned but not implemented:
- HashiCorp Vault integration
- Kubernetes Secrets with Sealed Secrets
- AWS Secrets Manager integration

### Environment Variable Security (Implemented)

**Actual Implementation** (`pkg/config/env.go`):
```go
// LoadEnv loads environment variables with validation
func LoadEnv() error {
    // Load .env file if it exists
    if err := godotenv.Load(); err != nil {
        // It's okay if .env doesn't exist in production
        if !os.IsNotExist(err) {
            return fmt.Errorf("error loading .env file: %w", err)
        }
    }
    
    // Validate required variables
    required := []string{
        "DATABASE_URL",
        "JWT_SECRET",
        "REDIS_ADDR",
    }
    
    for _, key := range required {
        if os.Getenv(key) == "" {
            return fmt.Errorf("required environment variable %s not set", key)
        }
    }
    
    return nil
}
```

**Security Notes**:
- Secrets are not cleared after reading (consideration for future)
- No secret rotation mechanism built-in
- Logs sanitize sensitive values

## Access Control (RBAC)

### Current Implementation (Basic RBAC)

**In-Memory Authorization** (`pkg/auth/production_authorizer.go`):
```go
// Actual implementation - NOT using Casbin
type productionAuthorizer struct {
    policies map[string]Policy
    mu       sync.RWMutex
}

// Default roles configured
const (
    RoleAdmin  = "admin"   // Full access
    RoleUser   = "user"    // Read/write contexts, execute tools
    RoleViewer = "viewer"  // Read-only access
)

// Example policy check
func (a *productionAuthorizer) Authorize(ctx context.Context, subject, resource, action string) error {
    // Check user role
    userRole := getUserRole(subject)
    
    // Apply role-based permissions
    switch userRole {
    case RoleAdmin:
        return nil // Admin has full access
    case RoleUser:
        if resource == "contexts" || resource == "tools" {
            return nil
        }
    case RoleViewer:
        if action == "read" {
            return nil
        }
    }
    
    return ErrUnauthorized
}
```

### Future: Kubernetes RBAC

Kubernetes RBAC would be configured when deploying to K8s (not currently supported).

### API Key Management (Implemented)

**API Key Service** (`pkg/auth/api_key_service.go`):
```go
// Actual implementation with database storage
type APIKey struct {
    ID          string
    Name        string
    KeyHash     string    // bcrypt hash
    Permissions []string  // Role-based permissions
    CreatedAt   time.Time
    ExpiresAt   *time.Time
    LastUsedAt  *time.Time
}

// Create new API key
func (s *apiKeyService) CreateAPIKey(ctx context.Context, name string, permissions []string) (*APIKey, string, error) {
    // Generate secure random key
    rawKey := generateSecureKey(32)
    
    // Hash the key
    hashedKey, err := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)
    
    // Store in database
    apiKey := &APIKey{
        ID:          uuid.New().String(),
        Name:        name,
        KeyHash:     string(hashedKey),
        Permissions: permissions,
        CreatedAt:   time.Now(),
    }
    
    // Return key only once
    return apiKey, rawKey, nil
}
```

### JWT Authentication (Implemented)

**JWT Service** (`pkg/auth/jwt_service.go`):
```go
// Generate JWT tokens with claims
func (s *jwtService) GenerateToken(userID string, role string) (string, error) {
    claims := jwt.MapClaims{
        "sub":  userID,
        "role": role,
        "exp":  time.Now().Add(s.expiration).Unix(),
        "iat":  time.Now().Unix(),
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(s.secret))
}
```

## Security Scanning

### Current Security Testing

**Manual Security Checks**:
```bash
# Check for known vulnerabilities in dependencies
go list -json -deps ./... | nancy sleuth

# Static analysis
go vet ./...
staticcheck ./...

# Check for hardcoded secrets
gitleaks detect --source . -v
```

### Recommended Security Tools

1. **Dependency Scanning**:
   ```bash
   # Install nancy
   go install github.com/sonatype-nexus-community/nancy@latest
   
   # Run vulnerability check
   go list -json -deps ./... | nancy sleuth
   ```

2. **SAST (Static Application Security Testing)**:
   ```bash
   # Install gosec
   go install github.com/securego/gosec/v2/cmd/gosec@latest
   
   # Run security scan
   gosec -fmt sarif -out results.sarif ./...
   ```

3. **Container Scanning** (if using Docker):
   ```bash
   # Scan Docker image
   docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
     aquasec/trivy image developer-mesh:latest
   ```

### Future: CI/CD Security Integration

GitHub Actions workflows for automated security scanning are planned but not implemented.

### Dependency Management (Current Process)

```bash
# Regular dependency updates
go get -u ./...
go mod tidy
go mod verify

# Manual vulnerability checking
go list -json -deps ./... | nancy sleuth

# Review and test updates
make test
make lint
```

**Best Practices**:
1. Review changelogs before updating
2. Test thoroughly after updates
3. Use specific versions, not latest
4. Monitor security advisories

## Audit Logging (Implemented)

### Audit Logger Implementation (`pkg/auth/audit_logger.go`)

```go
// Actual implementation
type AuditEvent struct {
    Timestamp    time.Time              `json:"timestamp"`
    EventType    string                 `json:"event_type"`
    Subject      string                 `json:"subject"`
    Action       string                 `json:"action"`
    Resource     string                 `json:"resource"`
    Result       string                 `json:"result"`
    ErrorMessage string                 `json:"error,omitempty"`
    Metadata     map[string]interface{} `json:"metadata,omitempty"`
    TraceID      string                 `json:"trace_id,omitempty"`
}

// Events tracked:
- Authentication attempts (success/failure)
- API key creation/revocation
- Authorization decisions
- Rate limit violations
- Policy changes
- Role assignments
```

### Viewing Audit Logs

```bash
# View audit logs in Docker Compose
docker-compose logs -f mcp-server | grep "audit_event"

# Parse JSON logs with jq
docker-compose logs mcp-server | grep "audit_event" | jq '.'

# Filter by event type
docker-compose logs mcp-server | grep "audit_event" | jq 'select(.event_type=="auth_failed")'
```

### Log Retention

**Current**: Logs written to stdout/stderr, retention depends on deployment:
- Docker: Configure log driver and rotation
- EC2: Use CloudWatch Logs or similar
- Local: Redirect to file with logrotate

### Production Log Configuration

**Docker Compose Logging**:
```yaml
# docker-compose.production.yml
services:
  mcp-server:
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "10"
        labels: "service=mcp-server"
```

**CloudWatch Logs (EC2)**:
```bash
# Install CloudWatch agent
sudo yum install -y amazon-cloudwatch-agent

# Configure log streams
cat > /opt/aws/amazon-cloudwatch-agent/etc/config.json <<EOF
{
  "logs": {
    "logs_collected": {
      "files": {
        "collect_list": [
          {
            "file_path": "/var/log/mcp/*.log",
            "log_group_name": "mcp-audit-logs",
            "log_stream_name": "{instance_id}",
            "retention_in_days": 90
          }
        ]
      }
    }
  }
}
EOF
```

## Compliance Considerations

### Current Security Controls

Developer Mesh implements several security controls that support compliance:

1. **Access Control**
   - JWT and API key authentication ✅
   - Basic role-based access (admin/user/viewer) ✅
   - Session expiration configurable ✅
   - MFA not implemented ❌

2. **Audit Trail**
   - Comprehensive audit logging ✅
   - Structured JSON logs ✅
   - Log retention (deployment-specific) ✅
   - SIEM integration not built-in ❌

3. **Data Security**
   - TLS 1.2/1.3 support ✅
   - Encrypted passwords (bcrypt) ✅
   - Encryption at rest (deployment-specific) ⚠️
   - Data anonymization not implemented ❌

### Compliance Gaps

**Not Implemented**:
- GDPR data export/deletion tools
- HIPAA-specific controls
- SOC2 evidence collection
- Privacy controls (PII handling)
- Data retention policies
- Consent management

### Recommendations for Compliance

1. **SOC2**: 
   - Deploy monitoring stack
   - Implement change control process
   - Document security procedures
   - Regular security assessments

2. **GDPR**:
   - Build data export functionality
   - Implement right-to-deletion
   - Add consent tracking
   - Document data flows

3. **HIPAA**:
   - Enable encryption everywhere
   - Implement PHI access controls
   - Enhanced audit logging
   - BAA with cloud providers

## Security Headers

### Current Implementation

**Note**: Security headers middleware is not currently implemented but recommended.

### Recommended Implementation

Add to your reverse proxy (nginx example):
```nginx
# /etc/nginx/conf.d/security-headers.conf
add_header X-Content-Type-Options "nosniff" always;
add_header X-Frame-Options "DENY" always;
add_header X-XSS-Protection "1; mode=block" always;
add_header Referrer-Policy "strict-origin-when-cross-origin" always;
add_header Permissions-Policy "geolocation=(), microphone=(), camera=()" always;

# Only add HSTS if using HTTPS
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

# Adjust CSP based on your needs
add_header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline';" always;
```

### Future Enhancement

Security headers middleware could be added to the application:
```go
// Example implementation (not currently in codebase)
func SecurityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Add security headers
        c.Next()
    }
}
```

## Incident Response

### Basic Response Procedures

1. **Detection**
   - Monitor logs for security events
   - Check audit logs for anomalies
   - Review metrics for unusual patterns

2. **Initial Response**
   ```bash
   # Check active connections
   docker-compose exec mcp-server netstat -an
   
   # Review recent logs
   docker-compose logs --tail 1000 mcp-server | grep -E "error|fail|denied"
   
   # Check system resources
   docker stats
   ```

3. **Containment**
   ```bash
   # Block suspicious IP (example)
   iptables -A INPUT -s SUSPICIOUS_IP -j DROP
   
   # Disable compromised API key
   # Update database or configuration
   
   # Scale down if under attack
   docker-compose scale mcp-server=0
   ```

4. **Investigation**
   - Review audit logs for root cause
   - Check for unauthorized access
   - Analyze traffic patterns
   - Preserve evidence (logs, configs)

5. **Recovery**
   - Apply security patches
   - Rotate credentials
   - Restore from clean backup
   - Monitor for reoccurrence

### Emergency Procedures

**Service Isolation**:
```bash
# Stop all services
docker-compose stop

# Start only essential services
docker-compose up -d postgres redis
docker-compose up -d mcp-server
```

**Credential Rotation**:
1. Generate new JWT secret
2. Update API keys
3. Rotate database passwords
4. Update .env and restart services

## Security Best Practices

### Development Security

1. **Code Security** (Implemented practices):
   ```go
   // Parameterized queries (from codebase)
   query := `SELECT id, key_hash, permissions FROM api_keys WHERE id = $1`
   err := db.QueryRow(query, keyID).Scan(&id, &hash, &perms)
   
   // Input validation (from codebase)
   if req.Name == "" {
       return nil, errors.New("name is required")
   }
   
   // Password hashing (from codebase)
   hashedKey, err := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)
   ```

2. **Error Handling**:
   ```go
   // Don't expose internal errors
   if err != nil {
       logger.Error("Database error", zap.Error(err))
       return nil, errors.New("internal server error")
   }
   ```

3. **Secure Defaults**:
   - TLS 1.2 minimum
   - Bcrypt for passwords
   - UUID for identifiers
   - Secure random for tokens

### Deployment Security

1. **Docker Security**:
   ```dockerfile
   # Current Dockerfile uses good practices:
   FROM golang:1.24-alpine AS builder
   # ... build stage ...
   
   FROM alpine:latest
   RUN apk --no-cache add ca-certificates
   # Non-root user recommended (not implemented)
   ```

2. **Environment Configuration**:
   ```bash
   # Use strong secrets
   JWT_SECRET=$(openssl rand -base64 32)
   API_KEY=$(openssl rand -hex 32)
   
   # Restrict file permissions
   chmod 600 .env.production
   
   # Use read-only mounts where possible
   docker run -v /config:/config:ro ...
   ```

3. **Network Security**:
   - Only expose necessary ports
   - Use reverse proxy for TLS
   - Enable firewall rules
   - Monitor for anomalies

## Security Testing

### Manual Security Testing

1. **API Security Testing**:
   ```bash
   # Test authentication
   curl -X POST http://localhost:8081/api/v1/auth/login \
     -d '{"username":"test","password":"wrong"}' \
     -H "Content-Type: application/json"
   # Should return 401
   
   # Test rate limiting
   for i in {1..200}; do
     curl -X GET http://localhost:8081/api/v1/contexts \
       -H "Authorization: Bearer $TOKEN"
   done
   # Should get rate limited after threshold
   ```

2. **WebSocket Security**: <!-- Source: pkg/models/websocket/binary.go -->
   ```bash
   # Test without auth
   wscat -c ws://localhost:8080/ws -s mcp.v1
   # Should disconnect without valid auth
   
   # Test with valid auth
   wscat -c ws://localhost:8080/ws -s mcp.v1 \
     -H "Authorization: Bearer $TOKEN"
   ```

3. **Dependency Scanning**:
   ```bash
   # Check for known vulnerabilities
   go list -json -deps ./... | nancy sleuth
   
   # Review outdated dependencies
   go list -u -m all
   ```

### Security Metrics (Actual)

| Metric | Implementation | Status |
|--------|----------------|---------|
| TLS Support | 1.2/1.3 | ✅ Configured |
| Authentication | JWT + API Keys | ✅ Implemented |
| Authorization | Basic RBAC | ✅ Implemented |
| Audit Logging | JSON structured | ✅ Implemented |
| Rate Limiting | Token bucket | ✅ Implemented |
| Input Validation | Per-endpoint | ✅ Implemented |
| Secret Storage | Environment vars | ⚠️ Basic |
| Vulnerability Scanning | Manual | ⚠️ No automation |

## Security Checklist

### Implemented ✅
- [x] TLS 1.2+ support available
- [x] Authentication required for all endpoints
- [x] Audit logging enabled by default
- [x] Rate limiting configured
- [x] Input validation on all endpoints
- [x] Secure password hashing (bcrypt)
- [x] CORS configuration
- [x] Basic RBAC implementation

### Configuration Required ⚠️
- [ ] TLS certificates for production
- [ ] Strong JWT secrets and API keys
- [ ] Strong encryption key (32+ characters):
  - [ ] `ENCRYPTION_MASTER_KEY` for all services
  - [ ] Generate with: `openssl rand -base64 32`
- [ ] Database encryption at rest
- [ ] Log retention and rotation
- [ ] Firewall rules
- [ ] Reverse proxy for production

### Not Implemented ❌
- [ ] Automated security scanning
- [ ] External secrets management
- [ ] Security headers middleware
- [ ] Container security policies
- [ ] Network segmentation (K8s)
- [ ] SIEM integration
- [ ] Compliance tooling
- [ ] MFA support

### Deployment Checklist

**Before Production**:
1. Generate strong secrets
2. Configure TLS certificates
3. Set up reverse proxy
4. Configure firewall rules
5. Enable log aggregation
6. Set up monitoring/alerting
7. Document incident response
8. Review security configuration
9. Perform security testing
