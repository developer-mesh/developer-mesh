# RAG Loader Multi-Tenant Deployment Guide

## Overview

This guide covers deploying the multi-tenant RAG loader service. The RAG loader is now configured as a multi-tenant SaaS service with per-tenant credential isolation and API-driven configuration.

## Prerequisites

- Docker and Docker Compose installed
- PostgreSQL 14+ with pgvector extension
- Redis 7+
- AWS credentials (for Bedrock embeddings)
- Generated master encryption key

## Quick Start

### 1. Generate Encryption Keys

```bash
# Generate RAG master encryption key (32 bytes, base64 encoded)
openssl rand -base64 32

# Copy to .env.docker as RAG_MASTER_KEY
```

### 2. Configure Environment Variables

The RAG loader uses environment variables from `.env.docker`:

```bash
# Required Variables
RAG_MASTER_KEY=<your-generated-key>  # 32-byte base64 key
JWT_SECRET=docker-jwt-secret-change-in-production
RAG_API_ENABLED=true
RAG_API_PORT=8084

# AWS Configuration (for embeddings)
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=<your-aws-key>
AWS_SECRET_ACCESS_KEY=<your-aws-secret>
```

### 3. Start Services

```bash
# Start all services including RAG loader
docker-compose -f docker-compose.local.yml up -d

# Check RAG loader health
curl http://localhost:9094/health
```

### 4. Apply Migrations

Migrations are automatically applied on startup. To manually verify:

```bash
# Check migration status
docker-compose exec database psql -U devmesh -d devmesh_development \
  -c "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 5;"

# Expected migrations:
# - 000041_create_tenant_tables
# - 000042_add_row_level_security
# - 000043_create_mcp_tenants
```

## Configuration Changes from Single-Tenant

### Removed Configuration

**Before (Single-Tenant):**
```yaml
environment:
  - GITHUB_ACCESS_TOKEN=${GITHUB_ACCESS_TOKEN}
volumes:
  - ./apps/rag-loader/configs:/app/configs:ro
```

**After (Multi-Tenant):**
```yaml
environment:
  # GitHub token removed - now per-tenant via API
  - RAG_MASTER_KEY=${RAG_MASTER_KEY}
  - JWT_SECRET=${JWT_SECRET}
  - RAG_API_ENABLED=true
volumes:
  # Config mount removed - now database-driven
  - ./apps/rag-loader/migrations:/app/migrations:ro
```

### New Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `RAG_MASTER_KEY` | Master encryption key (32 bytes base64) | Yes | None |
| `JWT_SECRET` | JWT secret for token validation | Yes | None |
| `RAG_API_ENABLED` | Enable REST API for source management | Yes | `true` |
| `RAG_API_PORT` | API server port | No | `8084` |

## Security Configuration

### Master Encryption Key

The master key is used to derive tenant-specific encryption keys using HMAC-SHA256:

```go
tenantKey = SHA256(masterKey || tenantID || "RAG_TENANT_KEY_V1")
```

**Key Requirements:**
- Exactly 32 bytes (AES-256)
- Base64 encoded
- Unique per environment
- **NEVER commit production keys to git**

**Key Rotation:**
```bash
# 1. Generate new key
NEW_KEY=$(openssl rand -base64 32)

# 2. Run re-encryption script (future implementation)
go run scripts/rotate-rag-master-key.go --old-key="$OLD_KEY" --new-key="$NEW_KEY"

# 3. Update environment
export RAG_MASTER_KEY=$NEW_KEY
docker-compose restart rag-loader
```

### JWT Token Validation

The RAG loader validates JWT tokens with these claims:

```json
{
  "tenant_id": "uuid",
  "user_id": "uuid",
  "email": "user@example.com",
  "roles": ["user", "admin"],
  "exp": 1234567890,
  "iss": "devmesh"
}
```

**Token must:**
- Be signed with `JWT_SECRET`
- Include valid `tenant_id` (UUID format)
- Not be expired
- Come from configured issuer

## API Endpoints

The RAG loader exposes a REST API for source management:

### Health Check
```bash
GET http://localhost:9094/health
```

### Source Management
```bash
# Create source (requires JWT)
POST http://localhost:8084/api/v1/rag/sources
Authorization: Bearer <jwt-token>
Content-Type: application/json

{
  "source_id": "github-myorg",
  "source_type": "github_org",
  "config": {
    "org": "myorg",
    "base_url": "https://github.company.com",  // Optional: for GitHub Enterprise
    "include_archived": false
  },
  "credentials": {
    "token": "ghp_xxx"
  },
  "schedule": "0 */6 * * *",
  "description": "GitHub Enterprise organization"
}

# List sources (requires JWT)
GET http://localhost:8084/api/v1/rag/sources
Authorization: Bearer <jwt-token>

# Trigger manual sync
POST http://localhost:8084/api/v1/rag/sources/:id/sync
Authorization: Bearer <jwt-token>
```

## GitHub Enterprise Support

The RAG loader supports both GitHub.com and self-hosted GitHub Enterprise instances.

### Configuration

Add the optional `base_url` field to your source configuration to use GitHub Enterprise:

**GitHub.com (default):**
```json
{
  "source_type": "github_org",
  "config": {
    "org": "my-organization"
  },
  "credentials": {
    "token": "ghp_token"
  }
}
```

**GitHub Enterprise Server:**
```json
{
  "source_type": "github_org",
  "config": {
    "org": "internal-engineering",
    "base_url": "https://github.company.com",
    "include_archived": false
  },
  "credentials": {
    "token": "ghp_enterprise_token"
  }
}
```

### Supported URL Formats

| Input | Normalized To | Notes |
|-------|--------------|-------|
| _empty_ | `api.github.com` | Default (GitHub.com) |
| `https://github.com` | `api.github.com` | Explicit GitHub.com |
| `https://github.company.com` | `https://github.company.com/api/v3/` | Auto-appends API path |
| `https://github.company.com/` | `https://github.company.com/api/v3/` | Strips trailing slash |

### Authentication for GitHub Enterprise

1. Navigate to your Enterprise instance: `https://github.company.com/settings/tokens`
2. Generate Personal Access Token with required scopes:
   - `repo` - Full control of private repositories
   - `read:org` - Read organization data (for org-wide crawling)
3. Store token securely in credentials

### Example Configurations

#### Organization-wide (Enterprise)
```bash
curl -X POST http://localhost:8084/api/v1/rag/sources \
  -H "Authorization: Bearer JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "source_id": "enterprise-repos",
    "source_type": "github_org",
    "config": {
      "org": "engineering",
      "base_url": "https://github.company.com",
      "include_forks": true,
      "include_patterns": ["*.md", "docs/**"],
      "exclude_patterns": ["vendor/**"]
    },
    "credentials": {
      "token": "ghp_enterprise_token"
    }
  }'
```

#### Single Repository (Enterprise)
```bash
curl -X POST http://localhost:8084/api/v1/rag/sources \
  -H "Authorization: Bearer JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "source_id": "api-docs",
    "source_type": "github_repo",
    "config": {
      "owner": "platform",
      "repo": "documentation",
      "branch": "main",
      "base_url": "https://github.company.com"
    },
    "credentials": {
      "token": "ghp_enterprise_token"
    }
  }'
```

### Troubleshooting GitHub Enterprise

#### "Failed to access organization"
- **Cause**: Invalid token or insufficient permissions
- **Solution**: Verify token has `read:org` scope and hasn't expired
```bash
# Test token manually
curl -H "Authorization: token YOUR_TOKEN" \
  https://github.company.com/api/v3/user
```

#### "Failed to create GitHub Enterprise client"
- **Cause**: Invalid base_url format
- **Solution**: Ensure URL starts with `https://` and is valid
```bash
# Test connectivity
curl -I https://github.company.com
```

#### "Connection timeout"
- **Cause**: Network/firewall blocking access
- **Solution**: Verify HTTPS access to Enterprise host, check firewall rules

### Network Requirements

Ensure the RAG loader can access:
- GitHub Enterprise Server on port 443 (HTTPS)
- Valid DNS resolution for Enterprise domain
- Valid TLS/SSL certificates

For proxy environments:
```bash
export HTTPS_PROXY=https://proxy.company.com:8443
export NO_PROXY=localhost,127.0.0.1,.company.com
```

## Database Schema

### Multi-Tenant Tables

```sql
-- Tenant configuration
rag.tenant_sources (
  tenant_id UUID,           -- Foreign key to mcp.tenants
  source_id VARCHAR,        -- Unique per tenant
  source_type VARCHAR,      -- github_org, github_repo, etc.
  config JSONB,            -- Source-specific config
  schedule VARCHAR,         -- Cron expression
  enabled BOOLEAN
)

-- Encrypted credentials (per-tenant)
rag.tenant_source_credentials (
  tenant_id UUID,
  source_id VARCHAR,
  credential_type VARCHAR,  -- token, api_key, oauth, etc.
  encrypted_value TEXT,     -- AES-256-GCM encrypted
  expires_at TIMESTAMP
)

-- Indexed documents
rag.tenant_documents (
  tenant_id UUID,
  source_id VARCHAR,
  document_id VARCHAR,
  content TEXT,
  embedding VECTOR(1536),
  metadata JSONB
)

-- Sync job tracking
rag.tenant_sync_jobs (
  tenant_id UUID,
  source_id VARCHAR,
  job_type VARCHAR,
  status VARCHAR,
  documents_processed INTEGER
)
```

### Row Level Security

All tables have RLS enabled:

```sql
-- Set tenant context before queries
SELECT rag.set_current_tenant('<tenant-uuid>');

-- All queries are automatically filtered
SELECT * FROM rag.tenant_sources;  -- Only returns current tenant's data
```

## Monitoring

### Health Checks

```bash
# Service health
curl http://localhost:9094/health

# Expected response:
{
  "status": "healthy",
  "database": "connected",
  "redis": "connected"
}
```

### Metrics

Prometheus metrics available at `http://localhost:9094/metrics`:

- `rag_sync_jobs_total` - Total sync jobs by status
- `rag_documents_indexed_total` - Total documents indexed
- `rag_credential_operations_total` - Encryption/decryption operations
- `rag_api_requests_total` - API requests by endpoint

### Logs

```bash
# View RAG loader logs
docker-compose logs -f rag-loader

# Filter for errors
docker-compose logs rag-loader | grep ERROR

# View specific tenant activity
docker-compose logs rag-loader | grep "tenant_id=xxx"
```

## Troubleshooting

### Service Won't Start

**Symptom:** Container exits immediately

**Check:**
```bash
# View logs
docker-compose logs rag-loader

# Common issues:
# 1. Missing RAG_MASTER_KEY
# 2. Invalid base64 in master key
# 3. Database connection failed
# 4. Migration failures
```

**Solution:**
```bash
# Verify environment variables
docker-compose config | grep -A 20 rag-loader

# Check database connectivity
docker-compose exec rag-loader nc -zv database 5432

# Verify migrations
docker-compose exec database psql -U devmesh -d devmesh_development \
  -c "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1;"
```

### API Returns 401 Unauthorized

**Symptom:** All API requests fail with 401

**Check:**
```bash
# Verify JWT_SECRET matches between services
docker-compose exec rag-loader env | grep JWT_SECRET
docker-compose exec rest-api env | grep JWT_SECRET

# Test token generation
curl -X POST http://localhost:8081/api/v1/auth/token \
  -H "Authorization: Bearer dev-admin-key-1234567890"
```

### Credentials Cannot Be Decrypted

**Symptom:** Sync jobs fail with "decryption failed"

**Check:**
```bash
# Verify master key is set
docker-compose exec rag-loader env | grep RAG_MASTER_KEY

# Check key length
echo $RAG_MASTER_KEY | base64 -d | wc -c  # Should be 32
```

**Solution:**
- Ensure RAG_MASTER_KEY is same across all instances
- Verify credentials were encrypted with same tenant_id
- Check for key rotation without re-encryption

### Tenant Cannot See Their Data

**Symptom:** API returns empty results for tenant

**Check:**
```bash
# Verify tenant exists and is active
docker-compose exec database psql -U devmesh -d devmesh_development \
  -c "SELECT id, name, is_active FROM mcp.tenants WHERE id = '<tenant-uuid>';"

# Check RLS is working
docker-compose exec database psql -U devmesh -d devmesh_development <<EOF
SELECT rag.set_current_tenant('<tenant-uuid>');
SELECT COUNT(*) FROM rag.tenant_sources WHERE tenant_id = '<tenant-uuid>';
EOF
```

## Rollback Plan

If critical issues occur:

### 1. Disable API
```bash
# Stop accepting new requests
docker-compose stop rag-loader
```

### 2. Preserve Data
```bash
# Backup before rollback
docker-compose exec database pg_dump -U devmesh devmesh_development \
  --schema=rag --file=/tmp/rag_backup.sql

# Copy to host
docker cp $(docker-compose ps -q database):/tmp/rag_backup.sql ./
```

### 3. Rollback Migration (if needed)
```bash
# Revert to previous migration
migrate -database "postgresql://devmesh:devmesh@localhost:5432/devmesh_development?sslmode=disable" \
  -path apps/rag-loader/migrations down 1
```

### 4. Revert Code
```bash
# Switch to previous version
git checkout <previous-tag>
docker-compose build rag-loader
docker-compose up -d rag-loader
```

## Production Deployment Checklist

- [ ] Generate unique production `RAG_MASTER_KEY`
- [ ] Configure secure `JWT_SECRET` (min 32 characters)
- [ ] Remove default API keys
- [ ] Enable database SSL (`DATABASE_SSL_MODE=require`)
- [ ] Configure AWS credentials with minimum IAM permissions
- [ ] Set up database backups
- [ ] Enable audit logging
- [ ] Configure monitoring and alerts
- [ ] Test credential encryption/decryption
- [ ] Verify tenant isolation
- [ ] Load test with multiple tenants
- [ ] Document emergency procedures
- [ ] Train operations team
- [ ] Set up on-call rotation

## Additional Resources

- [Multi-Tenant Implementation Guide](../../docs/rag-loader-multi-tenant-implementation.md)
- [API Integration Tests](./internal/api/README_TESTS.md)
- [Security Best Practices](../../docs/security/tenant-isolation.md)
- [Operations Guide](../../docs/rag-loader-operations.md)

## Support

For issues or questions:
- GitHub Issues: https://github.com/developer-mesh/developer-mesh/issues
- Operations Runbook: docs/rag-loader-operations.md
- Security Incidents: Follow incident response plan
