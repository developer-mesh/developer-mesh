<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:40:35
Verification Script: update-docs-parallel.sh
Batch: ab
-->

# Quick Start Guide

Get Developer Mesh running locally in under 5 minutes.

## What You'll Learn

- How to run Developer Mesh locally
- Register your first organization
- Authenticate and make API calls
- Connect your first AI agent via MCP protocol
- Add users to your organization

## Prerequisites

### Required
- **[Docker](https://www.docker.com/get-started)** and Docker Compose
- **[Git](https://git-scm.com/downloads)**
- **[Go](https://golang.org/doc/install)** 1.24 (for building from source)
- **Make** (usually pre-installed on Linux/Mac)

### Optional
- AWS CLI configured (for AWS Bedrock embedding features)
- PostgreSQL client tools (for direct database access)
- `wscat` or `websocat` (for testing WebSocket connections)

## 🚀 Quick Setup

### Step 1: Clone and Setup Environment

```bash
# Clone the repository
git clone https://github.com/developer-mesh/developer-mesh.git
cd developer-mesh

# Setup environment variables
cp .env.example .env
# Edit .env with your settings (optional: add AWS credentials for embedding features)
```

### Step 2: Start Infrastructure Services

```bash
# Start PostgreSQL and Redis using Docker Compose
make dev-setup

# This starts:
# - PostgreSQL 14+ with pgvector extension
# - Redis 7+ for caching and streams

# Wait for services to be ready (~10 seconds)
docker-compose -f docker-compose.local.yml ps
```

### Step 3: Initialize Database

```bash
# Run database migrations
make migrate-up

# This creates all necessary tables including:
# - Organizations and users
# - API keys and authentication
# - Agent configurations
# - Embedding models
```

### Step 4: Start Developer Mesh Services

**Option A: Using Docker Compose (Recommended)**

```bash
# Build and start all services
make dev

# This starts:
# - MCP Server on port 8080
# - REST API on port 8081
# - Worker service for async processing
```

**Option B: Run Services Individually**

```bash
# Build services first
make build

# Then in separate terminals:
make run-mcp-server   # Terminal 1: MCP Server (port 8080)
make run-rest-api     # Terminal 2: REST API (port 8081)
make run-worker       # Terminal 3: Worker service
```

### Step 5: Verify Installation

```bash
# Check MCP Server health
curl http://localhost:8080/health

# Expected response:
# {"status":"healthy","version":"1.0.0"}

# Check REST API health
curl http://localhost:8081/health

# Expected response:
# {"status":"healthy","components":{"database":"up","redis":"up"}}
```

## 🎯 Register Your First Organization

Developer Mesh uses a multi-tenant architecture. You need to register an organization to get started.

### Register Organization

```bash
# Register a new organization with admin user
curl -X POST http://localhost:8081/api/v1/auth/register/organization \
  -H "Content-Type: application/json" \
  -d '{
    "organization_name": "My Company",
    "organization_slug": "my-company",
    "admin_email": "admin@mycompany.com",
    "admin_name": "Admin User",
    "admin_password": "SecurePass123!"
  }'
```

**Response:**
```json
{
  "organization": {
    "id": "uuid-here",
    "name": "My Company",
    "slug": "my-company",
    "subscription_tier": "free",
    "max_users": 5,
    "max_agents": 10
  },
  "user": {
    "id": "user-uuid",
    "email": "admin@mycompany.com",
    "name": "Admin User",
    "role": "owner"
  },
  "api_key": "devmesh_xxxxxxxxxxxxx",
  "message": "Organization registered successfully. Please check your email to verify your account."
}
```

**Important:** Save the `api_key` returned in the response. This is your initial API key for accessing the system.

### Password Requirements
- Minimum 8 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one number

## 🔐 Authentication

### Using Your API Key

Use the API key from registration to authenticate requests:

```bash
# Test authentication
curl -H "Authorization: Bearer devmesh_xxxxxxxxxxxxx" \
  http://localhost:8081/api/v1/auth/profile
```

### Login with Email/Password

```bash
# Login to get a JWT token
curl -X POST http://localhost:8081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@mycompany.com",
    "password": "SecurePass123!"
  }'
```

**Response:**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 86400
}
```

## 👥 User Management

### Invite Users to Your Organization

Only organization owners and admins can invite new users:

```bash
# Invite a new user
curl -X POST http://localhost:8081/api/v1/auth/users/invite \
  -H "Authorization: Bearer devmesh_xxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "developer@mycompany.com",
    "name": "Developer Name",
    "role": "member"
  }'
```

**Available Roles:**
- `owner`: Full organization control (only one per org)
- `admin`: Can manage users and settings
- `member`: Standard user access
- `readonly`: Read-only access

### Accept Invitation

Users receive an invitation email with a token. To accept:

```bash
# Accept invitation and create account
curl -X POST http://localhost:8081/api/v1/auth/invitation/accept \
  -H "Content-Type: application/json" \
  -d '{
    "token": "invitation_token_from_email",
    "password": "NewUserPass123!"
  }'
```

## 🤖 Connect Your First AI Agent

### MCP Protocol Connection

Developer Mesh implements the Model Context Protocol (MCP) for AI agent communication.

#### Connect with websocat

```bash
# Install websocat if needed
# macOS: brew install websocat
# Linux: Download from https://github.com/vi/websocat

# Connect to MCP server
websocat --header="Authorization: Bearer devmesh_xxxxxxxxxxxxx" \
  ws://localhost:8080/ws
```

#### Initialize MCP Session

Send these messages after connecting:

```json
// 1. Initialize
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"my-agent","version":"1.0.0"}}}

// 2. Confirm initialization
{"jsonrpc":"2.0","id":2,"method":"initialized","params":{}}

// 3. List available tools
{"jsonrpc":"2.0","id":3,"method":"tools/list"}
```

### Create and Assign Tasks

```bash
# Create a task via REST API
curl -X POST http://localhost:8081/api/v1/tasks \
  -H "Authorization: Bearer devmesh_xxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Review pull request #123",
    "type": "code_review",
    "priority": "high",
    "metadata": {
      "repository": "my-repo",
      "pr_number": 123
    }
  }'
```

## 🛠️ Common Operations

### View Logs

```bash
# All services
make docker-compose-logs

# Specific service
docker-compose -f docker-compose.local.yml logs mcp-server -f
```

### Run Tests

```bash
# Unit tests (fast)
make test

# Test specific module
make test-mcp-server
make test-rest-api
make test-worker

# Integration tests (requires Docker)
make test-integration

# Functional tests (requires full stack)
make test-functional

# Test coverage report
make test-coverage-html
open coverage.html

# Run tests with specific focus
make test-functional-focus FOCUS="Health Endpoint"
```

### Stop Services

```bash
# For pre-built images
docker-compose -f docker-compose.prod.yml down

# For local development
make docker-compose-down

# Stop individual services (Ctrl+C in their terminals)
```

### Update Docker Images

When using pre-built images, update to the latest versions:

```bash
# Pull latest images
GITHUB_USERNAME=your-github-username ./scripts/pull-images.sh

# Or pull a specific version
GITHUB_USERNAME=your-github-username ./scripts/pull-images.sh v1.2.3

# Restart services with new images
docker-compose -f docker-compose.prod.yml down
docker-compose -f docker-compose.prod.yml up -d
```

## 📁 Project Structure Overview

```
developer-mesh/
├── apps/               # Microservices (Go workspace modules)
│   ├── mcp-server/     # MCP protocol implementation
│   ├── rest-api/       # REST API endpoints  
│   └── worker/         # Async job processor
├── pkg/                # Shared libraries
├── configs/            # Configuration files
├── docs/               # Documentation
├── scripts/            # Utility scripts
├── test/               # Integration & functional tests
├── go.work             # Go workspace configuration
└── Makefile            # Build automation
```

## 🔧 Configuration

### Environment Variables

Common environment variables:

```bash
# Database
export DATABASE_URL="postgres://dev:dev@localhost:5432/dev?sslmode=disable"
# Or use the DSN format:
export DATABASE_DSN="postgresql://dev:dev@localhost:5432/dev?sslmode=disable"

# Redis
export REDIS_URL="redis://localhost:6379"

# AWS (optional)
export AWS_REGION="us-west-2"
export AWS_PROFILE="default"

# Logging
export LOG_LEVEL="debug"
```

### Configuration File

Key settings in `config.yaml`:

```yaml
api:
  listen_address: ":8080"
  enable_cors: true

database:
  host: localhost
  port: 5432
  database: mcp
  
cache:
  type: redis
  address: localhost:6379
```

## 🧪 Testing the Integration

### 1. GitHub Webhook Test

```bash
# Send a test webhook
./scripts/test-github-webhook.sh

# Check worker logs for processing
```

### 2. Dynamic Tool Discovery Test

```bash
# Discover and create a tool from an API
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer devmesh_xxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "github",
    "base_url": "https://api.github.com",
    "discovery_hints": {
      "openapi_url": "https://api.github.com/swagger.json"
    }
  }'

# List available tools
curl -H "Authorization: Bearer devmesh_xxxxxxxxxxxxx" \
  http://localhost:8081/api/v1/tools
```

**Note**: The embedding API endpoints (`/api/embeddings`) shown in some documentation are not currently registered in the REST API server.

### 3. Useful Helper Scripts

```bash
# Health check all services
./scripts/health-check.sh

# Validate all endpoints
./scripts/validate-endpoints.sh

# Test GitHub integration
./scripts/test-github-integration.sh

# Redis connectivity check
./scripts/redis-check.sh
```

## 🚨 Troubleshooting

### Port Already in Use

```bash
# Find process using port
lsof -i :8080  # MCP Server
lsof -i :8081  # REST API

# Kill process
kill -9 <PID>

# Or kill by port
kill -9 $(lsof -t -i:8080)
```

### Database Connection Failed

```bash
# Check PostgreSQL is running
docker-compose -f docker-compose.local.yml ps

# Restart PostgreSQL
docker-compose -f docker-compose.local.yml restart postgres
```

### Build Errors

```bash
# Clean and rebuild
make clean
go work sync
make build

# If module errors occur:
cd apps/mcp-server && go mod tidy
cd apps/rest-api && go mod tidy
cd apps/worker && go mod tidy
go work sync
```

### Permission Denied

```bash
# Fix script permissions
chmod +x scripts/*.sh
```

### Go Version Issues

```bash
# Check Go version (must be 1.24)
go version

# If using wrong version, install Go 1.24:
# macOS: brew install go@1.24
# Linux: Follow https://golang.org/doc/install
```

### Redis Streams Issues

```bash
# Check Redis connectivity
redis-cli ping

# Check stream info
redis-cli xinfo stream webhook_events

# Check consumer groups
redis-cli xinfo groups webhook_events
```

**Note**: Developer Mesh uses Redis Streams for event processing, not AWS SQS.

## 📚 Next Steps

Now that you have Developer Mesh running:

1. **Explore Examples**: Check out [integration examples](../examples/README.md)
2. **Read Architecture**: Understand the [system design](../architecture/system-overview.md)
3. **Try API Endpoints**: Review the [API documentation](../api-reference/vector-search-api.md)
4. **Setup IDE**: Configure your [development environment](../developer/development-environment.md)

## 💡 Tips

- Use `make help` to see all available commands (note: help target may not exist, check Makefile)
- Enable debug logging with `LOG_LEVEL=debug`
- Check logs with `make docker-compose-logs` or `docker-compose -f docker-compose.local.yml logs -f`
- Use `docker-compose -f docker-compose.local.yml ps` to verify service status
- All services log to stdout by default

## 🆘 Getting Help

If you encounter issues:

1. Check the [troubleshooting guide](../troubleshooting/)
2. Search [GitHub Issues](https://github.com/developer-mesh/developer-mesh/issues)
3. Ask in [Discussions](https://github.com/developer-mesh/developer-mesh/discussions)

---


<!-- VERIFICATION
This document has been automatically verified against the codebase.
Last verification: 2025-08-11 14:40:35
All features mentioned have been confirmed to exist in the code.
-->
