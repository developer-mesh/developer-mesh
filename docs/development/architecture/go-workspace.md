<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:35:09
Verification Script: update-docs-parallel.sh
Batch: aa
-->

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
├── apps/                     # Application modules
│   ├── mcp-server/          # AI Agent orchestration server
│   │   ├── go.mod           # Module: mcp-server
│   │   ├── cmd/server/      # Entry point
│   │   └── internal/        # Private packages
│   │       ├── api/         # WebSocket & HTTP handlers <!-- Source: pkg/models/websocket/binary.go -->
│   │       │   └── websocket/ # Binary protocol, agent registry <!-- Source: pkg/models/websocket/binary.go -->
│   │       ├── config/      # Configuration
│   │       └── core/        # Agent orchestration logic
│   ├── rest-api/            # REST API service
│   │   ├── go.mod           # Module: rest-api
│   │   ├── cmd/api/         # Entry point
│   │   └── internal/        # Private packages
│   ├── worker/              # Distributed task processor
│   │   ├── go.mod           # Module: worker
│   │   ├── cmd/worker/      # Entry point
│   │   └── internal/        # Private packages
│   └── mockserver/          # Testing mock server
│       ├── go.mod           # Module: mockserver
│       └── cmd/mockserver/  # Entry point
└── pkg/                     # Shared packages (33 total)
    ├── adapters/            # Tool integrations (GitHub, etc.)
    ├── agents/              # Agent management interfaces
    ├── api/                 # API types and handlers
    ├── auth/                # Authentication/authorization
    ├── aws/                 # AWS service clients
    ├── cache/               # Multi-level caching
    ├── chunking/            # Code-aware content chunking
    ├── collaboration/       # CRDT implementations
    ├── common/              # Shared utilities
    ├── config/              # Configuration management
    ├── core/                # Core business logic
    ├── database/            # Database abstractions
    ├── embedding/           # Vector embeddings (Bedrock)
    ├── events/              # Event-driven architecture
    ├── models/              # Domain models
    ├── observability/       # Logging, metrics, tracing
    ├── queue/               # Redis Streams for event processing
    ├── repository/          # Data access patterns
    ├── resilience/          # Circuit breakers, retries
    ├── services/            # Business services
    └── ...                  # 14 more packages
```

## Workspace Configuration

### go.work File

```go
go 1.24

use (
    ./apps/edge-mcp
    ./apps/mcp-server
    ./apps/mockserver
    ./apps/rest-api
    ./apps/worker
    ./pkg
    ./test/e2e
)
```

### Module Names

Modules use full GitHub paths with replace directives:

```go
// apps/mcp-server/go.mod
module github.com/developer-mesh/developer-mesh/apps/mcp-server

// apps/rest-api/go.mod
module github.com/developer-mesh/developer-mesh/apps/rest-api

// apps/worker/go.mod
module github.com/developer-mesh/developer-mesh/apps/worker

// Each module includes:
replace github.com/developer-mesh/developer-mesh/pkg => ../../pkg
```

## Dependency Flow

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ mcp-server  │     │  rest-api   │     │   worker    │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                     │
       └───────────────────┴─────────────────────┘
                           │
                     ┌─────▼─────┐
                     │    pkg    │
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
make sync
```

### Building Modules

```bash
# Build all applications
make build

# Build specific application
make build-mcp-server
make build-rest-api
make build-worker

# Build from module directory
cd apps/mcp-server
go build -o mcp-server ./cmd/server
```

### Testing

```bash
# Test all modules
make test

# Test specific module
make test-mcp-server

# Test with coverage
make test-coverage

# Integration tests
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
│   ├── adapters/       # External integrations
│   ├── api/            # API layer
│   ├── config/         # Configuration
│   └── core/           # Business logic
└── tests/              # Integration tests
```

### Shared Packages (`pkg/*`)

The 33 shared packages provide comprehensive functionality for AI orchestration:

#### Core AI Orchestration Packages
```
pkg/
├── agents/              # Agent management
│   └── interfaces.go   # AgentManager interface
├── services/           # Business services
│   ├── interfaces.go   # Service contracts
│   ├── assignment_engine.go # Task routing algorithms <!-- Source: pkg/services/assignment_engine.go -->
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
├── common/
│   └── cache/        # Multi-level caching
│       ├── memory.go # In-memory cache
│       ├── redis.go  # Distributed cache
│       └── multi.go  # Cache hierarchy
├── resilience/       # Fault tolerance
│   ├── circuitbreaker.go # Circuit breaker
│   ├── retry.go          # Retry logic
│   └── ratelimit.go      # Rate limiting
└── observability/    # Monitoring
    ├── logger.go     # Structured logging
    ├── metrics.go    # Prometheus metrics
    └── tracer.go     # OpenTelemetry tracing
```

#### Domain Model Packages
```
pkg/
├── models/           # Core domain models
│   ├── agent.go     # AI agent entities
│   ├── task.go      # Task definitions
│   ├── workflow.go  # Workflow orchestration
│   └── embedding.go # Vector embeddings
└── repository/      # Data access layer
    ├── agent/       # Agent repository
    ├── task/        # Task repository
    └── vector/      # Vector storage
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
apps/mcp-server/internal/core/engine.go

// Cannot import from another module
apps/rest-api/internal/adapters/
```

### 3. Shared Types

Place shared types in `pkg/models`:
```go
// pkg/models/context.go
type Context struct {
    ID        string
    Name      string
    CreatedAt time.Time
}
```

### 4. Interface Definitions

Define interfaces where they're used:
```go
// pkg/repository/interfaces.go
type Repository[T any] interface {
    Create(ctx context.Context, entity T) (T, error)
    Get(ctx context.Context, id string) (T, error)
}
```

### 5. Configuration

Standardize configuration across modules:
```go
// Use viper for consistent config loading
cfg, err := config.Load()
```

## Common Patterns

### Service Interface Pattern

Define contracts for AI services:

```go
// pkg/services/interfaces.go
type AssignmentEngine interface { <!-- Source: pkg/services/assignment_engine.go -->
    AssignTask(ctx context.Context, task *models.Task) (*models.Agent, error)
    GetStrategy() AssignmentStrategy
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
4. **Add go.mod**: Initialize modules with simple names
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
# Use simple module names
module mcp-server  # Good
module github.com/user/mcp-server  # Can cause conflicts
```

### Circular Dependencies

```bash
# Check dependency graph
go mod graph | grep circular

# Refactor to use interfaces
```

## Future Considerations

1. **AI-Specific Packages**: 
   - Separate module for agent SDK (`sdk/agent`)
   - Provider-specific modules for different AI models
   - Shared AI utilities package

2. **Performance Optimization**:
   - Module-aware build caching for faster CI/CD
   - Selective module testing based on changes
   - Binary size optimization per service

3. **Extensibility**:
   - Plugin system for new AI providers
   - Dynamic module loading for agents
   - Standardized adapter interfaces

4. **Developer Experience**:
   - Code generation for new adapters
   - Module templates for common patterns
   - Automated dependency updates

## References

- [Go Workspaces Documentation](https://go.dev/doc/tutorial/workspaces)
- [Go Modules Reference](https://go.dev/ref/mod)
- [Adapter Pattern](adapter-pattern.md)
