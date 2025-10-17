# Organization Setup and User Management Guide

This guide walks you through setting up your organization in Developer Mesh and managing users.

## Table of Contents

1. [Initial Setup](#initial-setup)
2. [Organization Registration](#organization-registration)
3. [Authentication](#authentication)
4. [User Management](#user-management)
5. [API Key Management](#api-key-management)
6. [Common Scenarios](#common-scenarios)
7. [Troubleshooting](#troubleshooting)

## Initial Setup

### Prerequisites

Before you begin, ensure you have:

1. Developer Mesh running locally (see [Quick Start Guide](./quick-start-guide.md))
2. Access to the REST API on port 8081
3. A valid email address for the admin account

### Verify Services

```bash
# Check REST API is running
curl http://localhost:8081/health

# Expected response:
# {"status":"healthy","components":{"database":"up","redis":"up"}}
```

## Organization Registration

Every user in Developer Mesh belongs to an organization. The first step is registering your organization.

### Register Your Organization

```bash
curl -X POST http://localhost:8081/api/v1/auth/register/organization \
  -H "Content-Type: application/json" \
  -d '{
    "organization_name": "Acme Corporation",
    "organization_slug": "acme-corp",
    "admin_email": "admin@acme.com",
    "admin_name": "John Doe",
    "admin_password": "SecurePass123!",
    "company_size": "50-200",
    "industry": "Technology",
    "use_case": "AI-powered DevOps automation"
  }'
```

### Understanding the Response

The registration response contains three critical pieces of information:

```json
{
  "organization": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Acme Corporation",
    "slug": "acme-corp",
    "subscription_tier": "free",
    "max_users": 5,
    "max_agents": 10
  },
  "user": {
    "id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
    "email": "admin@acme.com",
    "name": "John Doe",
    "role": "owner",
    "email_verified": false
  },
  "api_key": "devmesh_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0",
  "message": "Organization registered successfully. Please check your email to verify your account."
}
```

**Important:** Save the `api_key` immediately. This is your master API key and won't be shown again.

### Organization Limits

Each subscription tier has limits:

| Tier | Max Users | Max Agents | Max API Keys |
|------|-----------|------------|--------------|
| Free | 5 | 10 | 10 |
| Starter | 25 | 50 | 50 |
| Pro | 100 | 200 | 200 |
| Enterprise | Unlimited | Unlimited | Unlimited |

## Authentication

Developer Mesh supports two authentication methods:

### Method 1: API Key Authentication

Use the API key from registration:

```bash
# Using Authorization header
curl -H "Authorization: Bearer devmesh_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0" \
  http://localhost:8081/api/v1/some-endpoint

# Using X-API-Key header
curl -H "X-API-Key: devmesh_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0" \
  http://localhost:8081/api/v1/some-endpoint
```

### Method 2: JWT Token Authentication

Login with email/password to get a JWT token:

```bash
# Login
curl -X POST http://localhost:8081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@acme.com",
    "password": "SecurePass123!"
  }'

# Response includes JWT token
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 86400
}

# Use the JWT token
curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  http://localhost:8081/api/v1/some-endpoint
```

## User Management

### Inviting Users

Only organization owners and admins can invite new users:

```bash
# Invite a developer
curl -X POST http://localhost:8081/api/v1/users/invite \
  -H "Authorization: Bearer devmesh_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "developer@acme.com",
    "name": "Jane Smith",
    "role": "member"
  }'

# Invite an admin
curl -X POST http://localhost:8081/api/v1/users/invite \
  -H "Authorization: Bearer devmesh_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "lead@acme.com",
    "name": "Bob Johnson",
    "role": "admin"
  }'
```

### User Roles

| Role | Can Invite Users | Can Manage Settings | Can Delete Org | Typical Use |
|------|-----------------|--------------------|--------------| ------------|
| `owner` | ✅ | ✅ | ✅ | Organization founder |
| `admin` | ✅ | ✅ | ❌ | Team leads, managers |
| `member` | ❌ | ❌ | ❌ | Developers, users |
| `readonly` | ❌ | ❌ | ❌ | Auditors, viewers |

### Accepting Invitations

When a user is invited, they receive an email with an invitation token. To accept:

```bash
# Accept invitation and set password
curl -X POST http://localhost:8081/api/v1/auth/invitation/accept \
  -H "Content-Type: application/json" \
  -d '{
    "token": "inv_a1b2c3d4e5f6g7h8",
    "password": "MySecurePass456!"
  }'
```

The response includes login tokens for immediate access.

## API Key Management

### API Key Structure

Developer Mesh API keys follow this format:
- Prefix: `devmesh_` (8 characters)
- Random token: 64 hexadecimal characters
- Total length: 72 characters

Example: `devmesh_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6a7b8c9d0`

### API Key Scopes

API keys can have different scopes:

| Scope | Description |
|-------|-------------|
| `read` | Read access to resources |
| `write` | Create and modify resources |
| `admin` | Administrative operations |

### Security Best Practices

1. **Never share API keys** - Treat them like passwords
2. **Use environment variables** - Don't hardcode keys in code
3. **Rotate keys regularly** - Generate new keys periodically
4. **Use minimal scopes** - Only grant necessary permissions
5. **Monitor usage** - Check for unusual activity

## Common Scenarios

### Scenario 1: Setting Up a Development Team

```bash
# 1. Register organization
curl -X POST http://localhost:8081/api/v1/auth/register/organization \
  -d '{"organization_name":"Dev Team","organization_slug":"dev-team",...}'

# 2. Save the API key from response
export API_KEY="devmesh_xxxxx"

# 3. Invite team members
for email in alice@team.com bob@team.com charlie@team.com; do
  curl -X POST http://localhost:8081/api/v1/users/invite \
    -H "Authorization: Bearer $API_KEY" \
    -d "{\"email\":\"$email\",\"role\":\"member\"}"
done

# 4. Invite team lead as admin
curl -X POST http://localhost:8081/api/v1/users/invite \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"email":"lead@team.com","role":"admin"}'
```

### Scenario 2: CI/CD Integration

For CI/CD pipelines, use a dedicated API key:

```yaml
# GitHub Actions example
name: Deploy with Developer Mesh
env:
  DEVMESH_API_KEY: ${{ secrets.DEVMESH_API_KEY }}
  DEVMESH_API_URL: https://api.devmesh.io

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Trigger Developer Mesh Workflow
        run: |
          curl -X POST $DEVMESH_API_URL/api/v1/workflows/deploy \
            -H "Authorization: Bearer $DEVMESH_API_KEY" \
            -d '{"environment":"production"}'
```

## Troubleshooting

### Common Issues

#### "Organization slug already exists"

The slug must be unique across all organizations. Try a different slug:

```bash
# Add a unique identifier
"organization_slug": "acme-corp-2024"

# Or use department/region
"organization_slug": "acme-engineering"
```

#### "Password validation failed"

Ensure your password meets all requirements:
- ✅ At least 8 characters
- ✅ Contains uppercase letter (A-Z)
- ✅ Contains lowercase letter (a-z)
- ✅ Contains number (0-9)

Good examples:
- `SecurePass123!`
- `MyP@ssw0rd2024`
- `DevMesh2024Admin`

#### "Email already registered"

Each email can only be associated with one user account. Options:
1. Use a different email address
2. Use email aliases (e.g., `admin+dev@company.com`)
3. Contact support to resolve duplicate accounts

#### "Insufficient permissions to invite users"

Only `owner` and `admin` roles can invite users. Check your role:

```bash
# Login and check your user info
curl -X POST http://localhost:8081/api/v1/auth/login \
  -d '{"email":"your@email.com","password":"YourPass123!"}'

# The response includes your role
```

### Getting Help

If you encounter issues not covered here:

1. Check the [API Reference](../reference/api/authentication-api-reference.md)
2. Review server logs: `docker-compose logs rest-api`
3. Search [GitHub Issues](https://github.com/developer-mesh/developer-mesh/issues)
4. Ask in [Discussions](https://github.com/developer-mesh/developer-mesh/discussions)

## Next Steps

After setting up your organization:

1. [Connect AI Agents](./ai-agent-orchestration.md) - Set up your first AI agent
2. [Configure Tools](../reference/api/dynamic_tools_api.md) - Add DevOps tools
3. [Create Workflows](./multi-agent-collaboration.md) - Build automation workflows
4. [Monitor Usage](./cost-optimization-guide.md) - Track costs and usage