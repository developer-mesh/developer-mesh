# Quick Start Guide

Get Developer Mesh running locally in under 5 minutes.

## Prerequisites

### Option A: Using Pre-built Images (Recommended)
- **[Docker](https://www.docker.com/get-started)** and Docker Compose
- **[Git](https://git-scm.com/downloads)** (to clone configuration files)

### Option B: Building from Source
- **[Go](https://golang.org/doc/install)** 1.24 or later
- **[Docker](https://www.docker.com/get-started)** and Docker Compose
- **[Git](https://git-scm.com/downloads)**
- **Make** (usually pre-installed on Linux/Mac)

### Optional (for full features)
- AWS CLI configured (for S3/SQS features)
- PostgreSQL client tools (for direct DB access)

## 🚀 Quick Setup

Choose one of the following options:

### Option A: Using Pre-built Images (Recommended) 🐳

This is the fastest way to get started with Developer Mesh.

```bash
# Clone the repository for configuration files
git clone https://github.com/developer-mesh/developer-mesh.git
cd developer-mesh

# Create environment file
cp .env.example .env
# Edit .env with your settings (API tokens, passwords, etc.)

# Pull the latest images (replace 'your-github-username' with the repo owner)
GITHUB_USERNAME=your-github-username ./scripts/pull-images.sh

# Start all services using production docker-compose
docker-compose -f docker-compose.prod.yml up -d

# Check service health
docker-compose -f docker-compose.prod.yml ps
docker-compose -f docker-compose.prod.yml logs -f --tail=100
```

Services will be available at:
- MCP Server: http://localhost:8080
- REST API: http://localhost:8081
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000

### Option B: Building from Source 🔨

For development or customization:

```bash
# Clone the repository
git clone https://github.com/developer-mesh/developer-mesh.git
cd developer-mesh

# Copy configuration template
cp config.yaml.example config.yaml
# Edit config.yaml with your settings (especially API tokens)

# Start infrastructure services
make dev-setup

# Wait for services to be ready (usually ~10 seconds)
# Check logs if needed
make docker-compose-logs

# Note: This command runs:
# - PostgreSQL 17 with pgvector extension
# - Redis 7 Alpine
# - Connects to real AWS services (SQS, S3, Bedrock)
# - Requires AWS credentials configured
```

Then continue with the appropriate steps:

#### For Pre-built Images (Option A):

The database is automatically initialized when using `docker-compose.prod.yml`. 
Skip to the [Verify Installation](#verify-installation) section below.

#### For Building from Source (Option B):

### 3. Initialize Database

```bash
# Install golang-migrate if not already installed
# macOS: brew install golang-migrate
# Linux: see https://github.com/golang-migrate/migrate

# Run database migrations
make migrate-up

# This creates tables and indexes including pgvector
# Migrations are located in apps/rest-api/migrations/sql/
```

### 4. Build and Run Services

**Option A: Run All Services (Recommended)**

```bash
# First build all services
make build

# Then in separate terminal windows:

# Terminal 1 - MCP Server (port 8080)
make run-mcp-server

# Terminal 2 - REST API (port 8081)
make run-rest-api

# Terminal 3 - Worker (processes SQS messages)
make run-worker
```

**Option B: Run with Docker Compose**

```bash
# Build and run all services
make dev

# This automatically:
# - Builds all service containers
# - Starts PostgreSQL, Redis
# - Creates SQS queues
# - Runs all three services
# - Exposes ports 8080 (MCP) and 8081 (REST API)
```

### 5. Verify Installation

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

### 6. API Documentation

Swagger UI is available for both services:

- **MCP Server Swagger**: http://localhost:8080/swagger/index.html
- **REST API Swagger**: http://localhost:8081/swagger/index.html

Note: Generate/update Swagger docs with:
```bash
make swagger
```

## 🎯 First API Call

Create your first context:

```bash
# Create a context
curl -X POST http://localhost:8081/api/v1/contexts \
  -H "Content-Type: application/json" \
  -H "X-API-Key: change-this-admin-key" \
  -d '{
    "name": "My First Context",
    "type": "conversation",
    "metadata": {
      "description": "Testing Developer Mesh"
    }
  }'

# Response includes the created context with ID
# Note: Update the API key to match your config.yaml
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

### 2. Vector Search Test

```bash
# Create embedding
curl -X POST http://localhost:8081/api/embeddings \
  -H "Content-Type: application/json" \
  -H "X-API-Key: change-this-admin-key" \
  -d '{
    "agent_id": "my-agent",
    "text": "DevOps automation with AI",
    "context_id": "your-context-id"
  }'

# Search embeddings
curl -X POST http://localhost:8081/api/embeddings/search \
  -H "Content-Type: application/json" \
  -H "X-API-Key: change-this-reader-key" \
  -d '{
    "agent_id": "my-agent",
    "query": "AI-powered DevOps tools",
    "limit": 5
  }'
```

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
# Check Go version (must be 1.24+)
go version

# If using older version, update Go:
# macOS: brew upgrade go
# Linux: Follow https://golang.org/doc/install
```

### AWS SQS Issues

```bash
# Check SQS connectivity (real AWS)
aws sqs list-queues --region us-east-1

# Verify queue exists
aws sqs get-queue-attributes \
  --queue-url https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test \
  --attribute-names All
```

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

**Congratulations!** You now have a fully functional Developer Mesh development environment. 🎉