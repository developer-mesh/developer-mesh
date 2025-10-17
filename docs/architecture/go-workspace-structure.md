# Go Workspace Structure

## Overview

Developer Mesh uses Go 1.24's workspace feature to organize the codebase as a monorepo with multiple modules. This architecture provides strong module boundaries while enabling code sharing, making it ideal for the AI agent orchestration platform's microservices architecture.

## Why Go Workspaces?

Go workspaces solve several challenges in multi-module projects:

- **Local Development**: Work on multiple modules simultaneously without replace directives
- **Dependency Management**: Shared dependencies are managed consistently
- **Code Sharing**: Common packages can be imported across modules
- **Build Optimization**: Only affected modules need rebuilding
- **Version Independence**: Modules can have different dependency versions when needed

## Project Structure

```
developer-mesh/
├── go.work                    # Workspace definition
├── go.work.sum               # Workspace checksums
├── Makefile                  # Build automation
├── apps/                     # Application modules (5 services)
│   ├── edge-mcp/            # MCP gateway server (WebSocket)
│   │   ├── go.mod           # Module: edge-mcp
│   │   ├── cmd/server/      # Entry point
│   │   └── internal/        # Private packages
│   │       ├── api/         # MCP protocol handlers
│   │       ├── handlers/    # Tool execution handlers
│   │       ├── tools/       # Tool registry
│   │       └── config/      # Configuration
│   ├── rest-api/            # REST API service
│   │   ├── go.mod           # Module: rest-api
│   │   ├── cmd/api/         # Entry point
│   │   ├── internal/        # Private packages
│   │   │   ├── api/         # HTTP handlers
│   │   │   ├── service/     # Business logic
│   │   │   └── config/      # Configuration
│   │   └── migrations/      # Database migrations
│   ├── worker/              # Distributed task processor
│   │   ├── go.mod           # Module: worker
│   │   ├── cmd/worker/      # Entry point
│   │   └── internal/        # Private packages
│   │       ├── processors/  # Event processors
│   │       └── config/      # Configuration
│   ├── rag-loader/          # RAG loader for code indexing
│   │   ├── go.mod           # Module: rag-loader
│   │   ├── cmd/loader/      # Entry point
│   │   ├── internal/        # Private packages
│   │   │   ├── github/      # GitHub integration
│   │   │   ├── chunker/     # Code chunking
│   │   │   └── loader/      # Loading logic
│   │   └── migrations/      # Database migrations
│   └── mockserver/          # Testing mock server
│       ├── go.mod           # Module: mockserver
│       └── cmd/mockserver/  # Entry point
└── pkg/                     # Shared packages (50 packages)
    ├── adapters/            # Tool integrations (GitHub, etc.)
    ├── agents/              # Agent management interfaces
    ├── auth/                # Authentication/authorization
    ├── aws/                 # AWS service clients (Bedrock, S3)
    ├── cache/               # Multi-level caching (L1, L2)
    ├── chunking/            # Code-aware content chunking
    ├── circuitbreaker/      # Circuit breaker patterns
    ├── client/              # API clients
    ├── clients/             # Service clients
    ├── collaboration/       # CRDT implementations
    ├── common/              # Shared utilities
    ├── config/              # Configuration management
    ├── core/                # Core business logic
    ├── database/            # Database abstractions
    ├── embedding/           # Vector embeddings (multi-provider)
    ├── errors/              # Error handling
    ├── events/              # Event-driven architecture
    ├── feature/             # Feature flags
    ├── health/              # Health checks
    ├── intelligence/        # AI intelligence services
    ├── interfaces/          # Shared interfaces
    ├── metrics/             # Metrics collection (Prometheus)
    ├── middleware/          # HTTP/WebSocket middleware
    ├── models/              # Domain models
    ├── observability/       # Logging, metrics, tracing
    ├── protocol/            # Protocol implementations (MCP)
    ├── queue/               # Queue abstractions
    ├── rag/                 # RAG-specific logic
    ├── redis/               # Redis client and streams
    ├── relationship/        # Relationship management
    ├── repository/          # Data access patterns
    ├── resilience/          # Resilience patterns
    ├── retry/               # Retry logic with backoff
    ├── rules/               # Business rules engine
    ├── safety/              # Safety checks
    ├── search/              # Search implementations
    ├── security/            # Security utilities (encryption)
    ├── services/            # Business services
    ├── storage/             # Storage abstractions
    ├── tests/               # Test utilities
    ├── testutil/            # Testing helpers
    ├── tokenizer/           # Token counting
    ├── tools/               # Tool management
    ├── util/                # General utilities
    ├── utils/               # Additional utilities
    ├── webhook/             # Webhook handling
    └── worker/              # Worker pool implementations
```

## Workspace Configuration

### go.work File

```go
go 1.24.0

use (
    ./apps/edge-mcp
    ./apps/mockserver
    ./apps/rag-loader
    ./apps/rest-api
    ./apps/worker
    ./pkg
)
```

**Note**: The workspace does NOT include a `test/e2e` module. Test code exists in the `test/` directory but is not a separate Go module.

### Module Names

Modules use full GitHub paths with replace directives:

```go
// apps/edge-mcp/go.mod
module github.com/developer-mesh/developer-mesh/apps/edge-mcp

// apps/rest-api/go.mod
module github.com/developer-mesh/developer-mesh/apps/rest-api

// apps/worker/go.mod
module github.com/developer-mesh/developer-mesh/apps/worker

// apps/rag-loader/go.mod
module github.com/developer-mesh/developer-mesh/apps/rag-loader

// Each module includes:
replace github.com/developer-mesh/developer-mesh/pkg => ../../pkg
```

## Dependency Flow

```
┌─────────────┐   ┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│  edge-mcp   │   │  rest-api   │   │   worker    │   │ rag-loader  │
└──────┬──────┘   └──────┬──────┘   └──────┬──────┘   └──────┬──────┘
       │                 │                   │                 │
       └─────────────────┴───────────────────┴─────────────────┘
                                   │
                             ┌─────▼─────┐
                             │    pkg    │
                             │(50 pkgs)  │
                             └───────────┘
```

Rules:
1. Applications (`apps/*`) can import from `pkg`
2. Applications cannot import from each other
3. `pkg` cannot import from applications
4. External dependencies are managed per module

## Working with the Workspace

### Initial Setup

```bash
# Clone repository
git clone https://github.com/developer-mesh/developer-mesh.git
cd developer-mesh

# Sync workspace dependencies
go work sync

# Or use Makefile
make deps
```

### Building Modules

```bash
# Build all applications
make build

# Build specific application
make build-edge-mcp
make build-rest-api
make build-worker
make build-rag-loader

# Build from module directory
cd apps/edge-mcp
go build -o edge-mcp ./cmd/server
```

### Testing

```bash
# Test all modules
make test

# Test specific module
cd apps/rest-api
go test ./...

# Test with coverage
make test-coverage

# Integration tests (in test/ directory, not a module)
make test-integration
```

### Adding Dependencies

```bash
# Add to specific module
cd apps/rest-api
go get github.com/some/package

# Sync workspace
cd ../..
go work sync
```

## Module Organization

### Application Modules (`apps/*`)

Each application follows a consistent structure:

```
apps/service-name/
├── go.mod              # Module definition
├── cmd/                # Entry points
│   └── server/         # Main binary
│       └── main.go
├── internal/           # Private packages
│   ├── api/            # API layer
│   ├── config/         # Configuration
│   ├── handlers/       # Request handlers
│   └── service/        # Business logic
├── migrations/         # Database migrations (if applicable)
└── tests/              # Module-specific tests
```

### Shared Packages (`pkg/*`)

The 50 shared packages provide comprehensive functionality for AI orchestration:

#### Core AI Orchestration Packages
```
pkg/
├── agents/              # Agent management
│   └── interfaces.go   # AgentManager interface
├── services/           # Business services
│   ├── interfaces.go   # Service contracts
│   ├── assignment_engine.go # Task routing algorithms
│   └── notification_service.go # Real-time updates
├── collaboration/      # Real-time collaboration
│   ├── crdt/          # CRDT implementations
│   │   ├── gcounter.go    # Grow-only counter
│   │   ├── pncounter.go   # PN counter
│   │   ├── lwwregister.go # LWW register
│   │   └── orset.go       # OR-Set
│   ├── document_crdt.go   # Collaborative editing
│   └── state_crdt.go      # State synchronization
└── embedding/         # Vector embeddings
    ├── bedrock/       # AWS Bedrock provider
    ├── embedder.go    # Embedding interface
    └── router.go      # Multi-provider routing
```

#### Infrastructure Packages
```
pkg/
├── aws/               # AWS service clients
│   ├── bedrock.go    # Bedrock AI models
│   └── s3.go         # Context storage
├── cache/            # Multi-level caching
│   ├── memory.go     # L1 in-memory cache
│   ├── redis.go      # L2 distributed cache
│   └── multi.go      # Cache hierarchy
├── circuitbreaker/   # Circuit breaker pattern
├── resilience/       # Fault tolerance
│   ├── retry.go      # Retry logic
│   └── timeout.go    # Timeout handling
└── observability/    # Monitoring
    ├── logger.go     # Structured logging
    ├── metrics.go    # Prometheus metrics
    └── tracer.go     # OpenTelemetry tracing
```

#### Data & Protocol Packages
```
pkg/
├── models/           # Core domain models
│   ├── agent.go     # AI agent entities
│   ├── task.go      # Task definitions
│   ├── workflow.go  # Workflow orchestration
│   └── embedding.go # Vector embeddings
├── repository/      # Data access layer
│   ├── agent/       # Agent repository
│   ├── task/        # Task repository
│   └── vector/      # Vector storage
├── protocol/        # Protocol implementations
│   └── mcp/         # MCP protocol handlers
└── redis/           # Redis operations
    └── streams_client.go # Redis Streams client
```

## Best Practices

### 1. Module Independence

Keep modules loosely coupled:
- Define interfaces in `pkg`
- Implement in application modules
- Use dependency injection

### 2. Internal Packages

Use `internal/` for private code:
```go
// Only accessible within the module
apps/edge-mcp/internal/handlers/tools.go

// Cannot import from another module
apps/rest-api/internal/service/
```

### 3. Shared Types

Place shared types in `pkg/models`:
```go
// pkg/models/agent.go
type Agent struct {
    ID           string
    Type         string
    Capabilities []string
    Status       AgentStatus
    CreatedAt    time.Time
}
```

### 4. Interface Definitions

Define interfaces where they're used:
```go
// pkg/repository/interfaces.go
type Repository[T any] interface {
    Create(ctx context.Context, entity T) (T, error)
    Get(ctx context.Context, id string) (T, error)
    List(ctx context.Context, filter Filter) ([]T, error)
    Update(ctx context.Context, entity T) (T, error)
    Delete(ctx context.Context, id string) error
}
```

### 5. Configuration

Standardize configuration across modules:
```go
// Use pkg/config for consistent config loading
cfg, err := config.Load()
```

## Common Patterns

### Service Interface Pattern

Define contracts for AI services:

```go
// pkg/services/interfaces.go
type AssignmentEngine interface {
    AssignTask(ctx context.Context, task *models.Task) (*models.Agent, error)
    GetStrategy() AssignmentStrategy
    SelectBestAgent(ctx context.Context, candidates []*models.Agent) (*models.Agent, error)
}

type NotificationService interface {
    NotifyTaskAssigned(ctx context.Context, agentID string, task interface{}) error
    BroadcastToAgents(ctx context.Context, agentIDs []string, message interface{}) error
}
```

### Repository Pattern for Agents

Manage AI agent persistence:

```go
// pkg/repository/agent/interfaces.go
type Repository interface {
    Create(ctx context.Context, agent *models.Agent) error
    GetByStatus(ctx context.Context, status models.AgentStatus) ([]*models.Agent, error)
    GetWorkload(ctx context.Context, agentID uuid.UUID) (*models.AgentWorkload, error)
    GetLeastLoadedAgent(ctx context.Context, capability models.AgentCapability) (*models.Agent, error)
}
```

### Provider Pattern for Embeddings

Support multiple AI providers:

```go
// pkg/embedding/provider.go
type Provider interface {
    GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
    GetModel() string
    GetCost() float64
}

// pkg/embedding/router.go
type Router struct {
    providers []Provider
    strategy  RoutingStrategy
}

func (r *Router) Route(ctx context.Context, text string) ([]float32, error) {
    provider := r.strategy.SelectProvider(r.providers)
    return provider.GenerateEmbedding(ctx, text)
}
```

## Migration Notes

When migrating from a single module:

1. **Create Module Structure**: Set up `apps/` directories
2. **Move Code**: Relocate packages to appropriate modules
3. **Update Imports**: Change import paths
4. **Add go.mod**: Initialize modules with full paths
5. **Update go.work**: Include all modules
6. **Test Thoroughly**: Ensure no circular dependencies

## Troubleshooting

### Module Not Found

```bash
# Ensure module is in workspace
go work use ./apps/new-module

# Sync dependencies
go work sync
```

### Import Conflicts

```bash
# Use full module paths in go.mod
module github.com/developer-mesh/developer-mesh/apps/service-name

# Not simple names
module service-name  # Avoid this
```

### Circular Dependencies

```bash
# Check dependency graph
go mod graph | grep -E "app.*app"

# Refactor to use interfaces in pkg/
```

### Build Errors After Sync

```bash
# Clean and rebuild
go clean -modcache
go work sync
make build
```

## Performance Optimizations

### Build Caching

Go workspace enables intelligent caching:
- Only modified modules are rebuilt
- Shared pkg changes trigger dependent rebuilds
- CI/CD can cache per-module builds

### Parallel Builds

```bash
# Build all apps in parallel
make -j4 build

# Or manually
cd apps/edge-mcp && go build ./... &
cd apps/rest-api && go build ./... &
wait
```

## Future Considerations

1. **Module Expansion**:
   - Separate SDK module for external developers
   - Provider-specific modules for different AI models
   - Shared AI utilities package

2. **Testing Infrastructure**:
   - Consider test/ as a proper module for shared test utilities
   - Module-specific integration test packages
   - Shared test fixtures in pkg/testutil

3. **Performance Optimization**:
   - Module-aware build caching for faster CI/CD
   - Selective module testing based on git changes
   - Binary size optimization per service

4. **Developer Experience**:
   - Code generation for new modules
   - Module templates for common patterns
   - Automated dependency updates per module

## References

- [Go Workspaces Documentation](https://go.dev/doc/tutorial/workspaces)
- [Go Modules Reference](https://go.dev/ref/mod)
- [System Overview](system-overview.md) - Complete system architecture
- [Package Dependencies](package-dependencies.md) - Detailed dependency graph
- [Multi-Agent Embedding Architecture](multi-agent-embedding-architecture.md) - Embedding system design

---

**Document Status**: Verified against codebase
**Last Updated**: 2025-10-17
**Verified Components**: All module names, file paths, and package counts
**Workspace File**: Verified against actual go.work
