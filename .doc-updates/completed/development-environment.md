<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:41:31
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# Development Environment Setup

This guide provides a comprehensive setup for developing on the Developer Mesh AI Agent Orchestration Platform.

## Prerequisites

### Required Tools
- **Go 1.24+** - Required for workspace support (uses toolchain 1.24.2)
- **Docker 24+** & **Docker Compose v2** - For local services
- **Git 2.40+** - For version control
- **Make** - For build automation
- **AWS CLI v2** - For real AWS services
- **PostgreSQL client** - For database access

### Recommended Tools
- **VS Code** or **GoLand** - IDE with Go support
- **golangci-lint 1.61+** - For code quality
- **ripgrep (rg)** - For fast searching
- **jq** - For JSON processing
- **direnv** - For environment management (optional)
- **k9s** - For Kubernetes development (optional, not used in current deployment)
- **air** - For hot reload during development (optional)

## Quick Start (5 minutes)

### Option 1: Local Development with Real AWS

```bash
# Clone the repository
git clone https://github.com/developer-mesh/developer-mesh.git
cd developer-mesh

# Setup environment
cp .env.example .env
# Edit .env with your AWS credentials and settings

# One-command setup (starts PostgreSQL, Redis, runs migrations)
make dev-native

# Start ElastiCache tunnel (required for Redis)
./scripts/aws/connect-elasticache.sh  # Keep running in separate terminal

# Verify everything works
make health
make test-coverage  # Should be >85%
```

### Option 2: Docker Development

```bash
# Clone and setup
git clone https://github.com/developer-mesh/developer-mesh.git
cd developer-mesh

# Setup with Docker
make dev  # Starts all services in Docker

# Verify services
curl http://localhost:8080/health  # MCP Server
curl http://localhost:8081/health  # REST API
```

## Detailed Setup

### 1. Repository Setup

```bash
# Clone with submodules if any
git clone --recursive https://github.com/developer-mesh/developer-mesh.git
cd developer-mesh

# Setup git hooks
cp scripts/hooks/* .git/hooks/
chmod +x .git/hooks/*
```

### 2. Environment Configuration

```bash
# Copy environment template
cp .env.example .env

# Edit with your AWS credentials and settings
vim .env

# Required environment variables (from .env)
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=your-access-key
AWS_SECRET_ACCESS_KEY=your-secret-key

# S3 Configuration (IP-restricted bucket)
S3_BUCKET=sean-mcp-dev-contexts

# SQS Configuration
SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test

# Database (local PostgreSQL)
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_NAME=devops_mcp
DATABASE_USER=postgres
DATABASE_PASSWORD=postgres

# Redis (via SSH tunnel to ElastiCache)
REDIS_ADDR=127.0.0.1:6379  # Use 127.0.0.1, NOT localhost!
USE_SSH_TUNNEL_FOR_REDIS=true

# AI Models
BEDROCK_ENABLED=true
BEDROCK_SESSION_LIMIT=0.10  # $0.10 per session limit
GLOBAL_COST_LIMIT=10.0      # $10 daily limit

# Security
JWT_SECRET=dev-secret-change-in-prod
ADMIN_API_KEY=dev-admin-key
```

### 3. Go Workspace Setup

```bash
# Verify Go version
go version  # Should be 1.24+

# The workspace is already configured (go.work exists)
# Sync workspace modules
go work sync

# Download all dependencies
go mod download -x

# Verify workspace structure
go work edit -json | jq .

# Build all services to verify setup
make b  # Short alias for build
```

### 4. AWS Services Setup

```bash
# Configure AWS credentials
aws configure
# Enter your AWS Access Key ID, Secret Access Key, and region (us-east-1)

# Test AWS connectivity
./scripts/aws/test-aws-services.sh

# Start ElastiCache tunnel (REQUIRED for Redis)
# Keep this running in a separate terminal!
./scripts/aws/connect-elasticache.sh

# Verify S3 access (IP-restricted bucket)
aws s3 ls s3://sean-mcp-dev-contexts/

# Verify SQS access
aws sqs get-queue-attributes --queue-url https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test --attribute-names All

# Start local PostgreSQL
docker run -d \
  --name postgres \
  -p 5432:5432 \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=devops_mcp \
  postgres:15-alpine
```

### 5. Database Setup

```bash
# Install migrate tool if needed
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Run migrations
migrate -path migrations -database "postgresql://postgres:postgres@localhost:5432/devops_mcp?sslmode=disable" up

# Verify database
psql -h localhost -U postgres -d devops_mcp -c '\dt'

# Should see tables:
# - schema_migrations
# - users
# - api_keys
# - contexts
# - tools
# - And more...
```

### 6. Build & Run

```bash
# Build all services (using short aliases)
make b

# Run tests to ensure everything works
make t              # Run all tests
make test-coverage  # Must be >85%

# Lint code (must pass with 0 errors)
make lint

# Run services locally
# Terminal 1: MCP Server (WebSocket) <!-- Source: pkg/models/websocket/binary.go -->
cd apps/mcp-server && go run cmd/server/main.go

# Terminal 2: REST API
cd apps/rest-api && go run cmd/api/main.go

# Terminal 3: Worker
cd apps/worker && go run cmd/worker/main.go

# Or use the all-in-one command
make dev-native  # Runs all services
```

## IDE Setup

### VS Code Configuration

```json
// .vscode/settings.json
{
    "go.useLanguageServer": true,
    "go.lintTool": "golangci-lint",
    "go.lintFlags": ["--fast"],
    "go.testFlags": ["-v", "-race"],
    "go.testTimeout": "30s",
    "go.buildTags": "integration",
    "gopls": {
        "experimentalWorkspaceModule": true,
        "ui.semanticTokens": true,
        "ui.completion.usePlaceholders": true
    },
    "files.associations": {
        "*.go.tmpl": "go",
        "go.work": "go.work"
    }
}
```

### Essential Extensions

#### VS Code
- **golang.go** - Official Go support
- **ms-vscode-remote.remote-containers** - Dev containers
- **ms-azuretools.vscode-docker** - Docker support
- **mtxr.sqltools** - Database explorer
- **humao.rest-client** - API testing
- **redhat.vscode-yaml** - YAML support
- **eamodio.gitlens** - Git superpowers

#### GoLand
- Built-in Go support
- Database tools
- HTTP client
- Docker integration

## Project Structure

```
developer-mesh/
├── apps/                    # Application modules
│   ├── mcp-server/         # MCP protocol server
│   │   ├── cmd/            # Entry points
│   │   ├── internal/       # Private packages
│   │   └── go.mod         # Module definition
│   ├── rest-api/          # REST API service
│   └── worker/            # Async worker service
├── pkg/                    # Shared packages
│   ├── adapters/          # Tool adapters
│   ├── models/            # Domain models
│   └── observability/     # Logging, metrics, tracing
├── configs/               # Configuration files
├── scripts/              # Development scripts
├── test/                 # E2E tests
└── go.work              # Workspace definition
```

### Working with Modules

```bash
# Add new dependency to specific module
cd apps/mcp-server
go get github.com/some/package

# Run tests for specific module
cd apps/rest-api
go test -v ./...

# Update all modules
go work sync

# Add new module to workspace
go work use ./apps/new-service
```

## Development Workflow

### 1. Essential Commands (from CLAUDE.md)

```bash
# Pre-session setup (REQUIRED)
./scripts/aws/connect-elasticache.sh      # Keep running in separate terminal
./scripts/aws/test-aws-services.sh        # Verify AWS connectivity

# Development workflow
make b                   # Build all services
make t                   # Run tests (must pass)
make test-coverage       # Test coverage (must be >85%)
make lint                # Must show 0 errors
make pre-commit          # Run before EVERY commit

# Quick checks
grep -r "TODO" pkg/ apps/ --include="*.go"  # Must return NOTHING
make health              # Check service health
```

### 2. Feature Development

```bash
# Create feature branch
git checkout -b feature/my-feature

# Make changes with TDD
# 1. Write failing test
vim internal/feature_test.go

# 2. Run test (should fail)
go test ./internal -run TestFeature

# 3. Implement feature (NO TODOs!)
vim internal/feature.go

# 4. Run test (should pass)
go test ./internal -run TestFeature

# 5. Run all checks
make pre-commit  # Must pass before commit!
```

### 3. Debugging

```bash
# Run with debug logging
LOG_LEVEL=debug go run ./cmd/server

# Use delve debugger
dlv debug ./cmd/server

# Debug WebSocket connections <!-- Source: pkg/models/websocket/binary.go -->
wscat -c ws://localhost:8080/v1/mcp

# View structured logs with jq
go run ./cmd/server 2>&1 | jq '.'

# Debug with distributed tracing
# 1. Start Jaeger
docker run -d -p 16686:16686 jaegertracing/all-in-one
# 2. Enable tracing
ENABLE_TRACING=true TRACE_SAMPLING_RATE=1.0 go run ./cmd/server
# 3. View traces at http://localhost:16686
```

### 4. Testing Requirements

```bash
# Unit tests (must have >85% coverage)
go test -cover ./...

# Integration tests with real AWS
make test-integration

# Test with cost limits
BEDROCK_SESSION_LIMIT=0.01 go test ./pkg/services/...

# Benchmark WebSocket performance <!-- Source: pkg/models/websocket/binary.go -->
go test -bench=BenchmarkWebSocket ./pkg/api/websocket/... <!-- Source: pkg/models/websocket/binary.go -->

# No TODOs allowed!
! grep -r "TODO" pkg/ apps/ --include="*.go"
```

## Code Standards

### Style Guide

```go
// Package names: lowercase, no underscores
package adapters

// Interface names: end with -er when possible
type Adapter interface {
    Execute(ctx context.Context, req Request) (Response, error)
}

// Struct initialization: use field names
config := &Config{
    URL:     "https://api.example.com",
    Timeout: 30 * time.Second,
}

// Error handling: wrap with context
if err != nil {
    return fmt.Errorf("execute request: %w", err)
}

// Context usage: always first parameter
func (s *Service) Process(ctx context.Context, data []byte) error {
    // Check context cancellation
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }
    // Process...
}
```

### Testing Standards

```go
// Table-driven tests
func TestAdapter_Execute(t *testing.T) {
    tests := []struct {
        name    string
        input   Request
        want    Response
        wantErr bool
    }{
        {
            name:  "valid request",
            input: Request{ID: "123"},
            want:  Response{Status: "success"},
        },
        {
            name:    "invalid request",
            input:   Request{},
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Parallel safe
            t.Parallel()
            
            // Test implementation
            got, err := adapter.Execute(context.Background(), tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("Execute() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Local Development Tips

### Critical Requirements (NO EXCEPTIONS)

```bash
# 1. ALWAYS use real AWS services in production
# LocalStack exists for local testing but production requires real S3, SQS, Bedrock

# 2. ElastiCache tunnel MUST be running
./scripts/aws/connect-elasticache.sh  # Keep open!

# 3. Use 127.0.0.1 for Redis, NOT localhost
REDIS_ADDR=127.0.0.1:6379  # Correct
REDIS_ADDR=localhost:6379   # WRONG!

# 4. No TODOs in code
grep -r "TODO" pkg/ apps/ --include="*.go"  # Must be empty

# 5. No nil services
# Bad:  authorizer := nil
# Good: authorizer := auth.NewProductionAuthorizer(config)
```

### Hot Reload (Optional)

```bash
# Install air for hot reload (not currently configured in project)
go install github.com/cosmtrek/air@latest

# Create .air.toml in each app directory
cat > apps/mcp-server/.air.toml << 'EOF'
root = "."
tmp_dir = "tmp"

[build]
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main ./cmd/server"
  delay = 1000
  exclude_dir = ["tmp", "vendor"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html"]
  kill_delay = "0s"
  log = "build-errors.log"
  send_interrupt = false
  stop_on_error = true

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  time = false

[misc]
  clean_on_exit = false
EOF

# Run with hot reload
cd apps/mcp-server && air
```

### Database Management

```bash
# Connect to local database
psql -h localhost -U postgres -d devops_mcp

# Common queries
-- List all contexts
SELECT id, name, created_at FROM contexts ORDER BY created_at DESC LIMIT 10;

-- Check API keys
SELECT key_hash, name, last_used_at FROM api_keys;

-- View embeddings
SELECT id, model, dimensions, created_at FROM embeddings LIMIT 5;
```

### Working with Docker Images

#### Using Pre-built Images for Development

```bash
# Pull specific version for testing
GITHUB_USERNAME=your-github-username ./scripts/pull-images.sh v1.2.3

# Run a specific service with pre-built image
docker run -it --rm \
  -e DATABASE_URL=postgres://dev:dev@host.docker.internal:5432/dev \
  -p 8080:8080 \
  ghcr.io/${GITHUB_USERNAME}/developer-mesh-mcp-server:latest

# Override configuration
docker run -it --rm \
  -v $(pwd)/config.yaml:/app/config.yaml \
  ghcr.io/${GITHUB_USERNAME}/developer-mesh-rest-api:latest
```

#### Building Images Locally

```bash
# Build single service
make docker-build-mcp-server

# Build all services with proper metadata
make docker-build-all VERSION=dev

# Build with custom tags
docker build \
  --build-arg VERSION=$(git describe --tags --always) \
  --build-arg COMMIT_SHA=$(git rev-parse HEAD) \
  --build-arg BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
  -t developer-mesh-local:dev \
  -f apps/mcp-server/Dockerfile .
```

#### Testing with Different Image Versions

```bash
# Compare behavior between versions
VERSION=v1.2.2 docker-compose -f docker-compose.prod.yml up -d
# Test...
docker-compose -f docker-compose.prod.yml down

VERSION=v1.2.3 docker-compose -f docker-compose.prod.yml up -d
# Test...
```

### Performance Profiling

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof

# Memory profiling
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof

# Trace execution
go test -trace=trace.out
go tool trace trace.out
```

### Troubleshooting Common Issues

```bash
# ElastiCache connection issues
# Error: dial tcp [::1]:6379: connect: connection refused
# Fix: Use 127.0.0.1:6379, NOT localhost:6379

# AWS credentials issues
# Error: NoCredentialProviders
# Fix:
aws configure list  # Check configuration
aws sts get-caller-identity  # Verify credentials

# S3 access denied
# Error: AccessDenied for bucket sean-mcp-dev-contexts
# Fix: Ensure your IP is whitelisted in bucket policy

# Module issues
go clean -modcache
go work sync
go mod download -x

# Port conflicts
lsof -i :8080
kill -9 $(lsof -t -i:8080)

# Database connection issues
# Check PostgreSQL is running
docker ps | grep postgres
# Restart if needed
docker restart postgres
```

## Security Considerations

### Secret Management

```bash
# Never commit secrets
echo '.env' >> .gitignore

# Use environment variables
export GITHUB_TOKEN=$(pass show github/token)

# Or use direnv (if installed)
echo 'export GITHUB_TOKEN="..."' >> .envrc
direnv allow
```

### Dependency Security

```bash
# Check for vulnerabilities
go list -json -m all | nancy sleuth

# Or use govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...

# Update dependencies safely (per module)
cd apps/mcp-server && go get -u ./... && go mod tidy
cd apps/rest-api && go get -u ./... && go mod tidy
cd apps/worker && go get -u ./... && go mod tidy

# Sync workspace after updates
go work sync

# Run all tests to verify updates
make test-coverage  # Must maintain >85%
```

## Next Steps

1. **Verify Setup**: Run `make pre-commit` to ensure everything is configured correctly
2. **Run Integration Tests**: `make test-integration` with real AWS services
3. **Review Architecture**: See [System Overview](../architecture/system-overview.md)
4. **Understand AI Features**: Read [AI Agent Orchestration](../ai-agents/ai-agent-orchestration.md)
5. **Check WebSocket Protocol**: See [MCP Protocol Reference](../api/mcp-protocol-reference.md) <!-- Source: pkg/models/websocket/binary.go -->

## Important References

- [CLAUDE.md](../../CLAUDE.md) - Essential implementation rules and commands
- [Configuration Guide](../operations/configuration-guide.md) - Environment setup details
- [Production Deployment](../deployment/production-deployment.md) - AWS deployment guide
- [Observability Architecture](../guides/observability-architecture.md) - Monitoring setup

## Frequently Encountered Issues

1. **Redis Connection Refused**
   - Ensure ElastiCache tunnel is running: `./scripts/aws/connect-elasticache.sh`
   - Use `127.0.0.1:6379 (Redis)` not `localhost:6379 (Redis)`

2. **S3 Access Denied**
   - Check IP whitelist in bucket policy
   - Verify AWS credentials: `aws sts get-caller-identity`

3. **Test Coverage Below 85%**
   - Run `make test-coverage` to see uncovered code
   - Add tests before implementing features

4. **Lint Errors**
   - Run `make lint` to see all issues
   - Fix all errors before committing

---

*For production deployment, see [Production Deployment Guide](../deployment/production-deployment.md)*
