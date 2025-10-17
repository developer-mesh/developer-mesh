# Developer Mesh System Architecture

## Overview

Developer Mesh is an AI Agent Orchestration Platform that enables intelligent routing and coordination of multiple AI agents for DevOps workflows. Built using Go workspaces for modularity, it provides sophisticated multi-agent orchestration, real-time collaboration features, and production-ready AWS integrations.

## Architecture Principles

- **Microservices Architecture**: Multiple independent services communicating via APIs and events
- **AI-Native Design**: Built from the ground up for multi-agent orchestration and coordination
- **Clean Architecture**: Clear separation between business logic, adapters, and infrastructure
- **Event-Driven Design**: Real-time WebSocket communication with asynchronous task processing
- **Go Workspace**: Monorepo with multiple modules for code sharing and independent deployment
- **Cloud-Native**: Production AWS integration with Bedrock, S3, and ElastiCache
- **MCP Protocol**: Full Model Context Protocol 2025-06-18 implementation

## System Components

### 🔵 Edge MCP Server (`apps/edge-mcp`)

Edge MCP is a lightweight MCP server that acts as a gateway to the DevMesh Platform:

- **MCP 2025-06-18 Protocol**: Industry-standard Model Context Protocol over WebSocket (JSON-RPC 2.0)
- **Connection Modes**: Auto-detection for Claude Code, IDEs, and agents
- **Zero Infrastructure**: No direct database/Redis dependencies - API-based sync with Core Platform
- **Pass-Through Authentication**: Secure credential management via DevMesh Platform
- **Dynamic Tool Discovery**: Automatically discovers available tools from tenant
- **Multi-Tenant Support**: Isolated workspaces with per-tenant configurations
- **Circuit Breaker**: Resilient connection handling with automatic retries
- **Command Execution Security**: Process isolation, timeout enforcement, command allowlisting

Key Features:
- **Universal agent support** for any tool or service type
- **Capability-based discovery** across different agent types
- **Tenant isolation** with organization-level security boundaries
- **Real-time agent discovery** via MCP tools/list
- **Resource subscriptions** for workflows and tasks
- **Binary Protocol Support**: Optional compressed messages for efficiency
- **Heartbeat Monitoring**: Automatic reconnection handling

### 🟢 REST API Service (`apps/rest-api`)

The REST API provides HTTP endpoints for external integrations:

- **Tool Management**: Register and configure dynamic tools
- **Organization Management**: Multi-tenant organization registration and configuration
- **Agent Management**: Register agents, query capabilities, monitor workload
- **Task Submission**: Submit tasks with routing preferences and requirements
- **Embedding Operations**: Generate and search embeddings via AWS Bedrock
- **Webhook Processing**: Receive and queue webhook events for async processing
- **Session Management**: Edge MCP session lifecycle and authentication

Key Features:
- All endpoints use `/api/v1/*` path prefix
- Multi-model embedding support (AWS Bedrock, OpenAI, Google, Anthropic)
- Cost tracking and optimization for AI operations
- JWT and API key authentication (multiple key types: admin, gateway, agent, user)
- Comprehensive Swagger/OpenAPI documentation
- Per-tenant credential encryption with AES-256-GCM
- Redis Streams for webhook event processing

### 🟠 Worker Service (`apps/worker`)

The Worker handles distributed task processing:

- **Webhook Event Processing**: Consumes events from Redis Streams
- **Task Distribution**: Processes tasks assigned to AI agents
- **Embedding Pipeline**: Batch processing for vector embeddings
- **Notification Delivery**: Sends real-time updates via WebSocket
- **Dead Letter Queue**: Handles failed webhook processing

Key Features:
- Redis Streams with consumer groups for reliable event processing
- Concurrent processing with configurable worker pools
- Automatic retry with exponential backoff
- Health monitoring and metrics

### 🔷 RAG Loader Service (`apps/rag-loader`)

Multi-tenant RAG (Retrieval-Augmented Generation) loader for codebase indexing:

- **GitHub Integration**: Loads repositories for semantic search
- **Multi-Org Support**: Per-organization repository configurations
- **Chunking & Embedding**: Intelligent code chunking with embedding generation
- **pgvector Storage**: Stores embeddings for fast similarity search
- **Incremental Updates**: Tracks and processes only changed files

Key Features:
- Support for multiple GitHub organizations per tenant
- File type filtering and gitignore respect
- Batch embedding generation for cost optimization
- Complete audit trail of loaded repositories

### 🎭 Mock Server (`apps/mockserver`)

Testing infrastructure for development and CI/CD:

- **API Mocking**: Mock external service responses
- **Tool Testing**: Test dynamic tool integrations
- **Development Support**: Local testing without external dependencies

### 📦 Shared Libraries (`pkg/`)

Reusable packages across all services:

```
pkg/
├── adapters/           # External service integrations (GitHub, AWS, etc.)
├── agents/             # Agent management and coordination
├── auth/               # Authentication and authorization
├── aws/                # AWS service integrations (Bedrock, S3)
├── cache/              # Multi-tier caching (L1 memory, L2 Redis)
├── chunking/           # Code chunking for RAG
├── circuitbreaker/     # Circuit breaker patterns
├── client/             # API clients
├── clients/            # Service clients
├── collaboration/      # CRDT-based collaborative editing
├── common/             # Shared utilities and types
├── config/             # Configuration management
├── core/               # Core domain logic
├── database/           # Database abstractions and utilities
├── embedding/          # Vector embedding services (multi-provider)
├── errors/             # Error handling and wrapping
├── events/             # Event publishing and handling
├── feature/            # Feature flags
├── health/             # Health check implementations
├── intelligence/       # AI intelligence services
├── interfaces/         # Shared interfaces
├── metrics/            # Metrics collection (Prometheus)
├── middleware/         # HTTP/WebSocket middleware
├── models/             # Domain models and entities
├── observability/      # Logging, metrics, tracing (OpenTelemetry)
├── protocol/           # Protocol implementations (MCP, etc.)
├── queue/              # Queue abstractions
├── rag/                # RAG-specific logic
├── redis/              # Redis client and streams (streams_client.go)
├── relationship/       # Relationship management
├── repository/         # Data access patterns
├── resilience/         # Resilience patterns (retry, timeout)
├── retry/              # Retry logic with exponential backoff
├── rules/              # Business rules engine
├── safety/             # Safety checks and validation
├── search/             # Search implementations
├── security/           # Security utilities (encryption, etc.)
├── services/           # Business services (assignment_engine.go)
├── storage/            # Storage abstractions
├── tests/              # Test utilities
├── testutil/           # Testing helpers
├── tokenizer/          # Token counting and management
├── tools/              # Tool management
├── util/               # General utilities
├── utils/              # Additional utilities
├── webhook/            # Webhook handling
└── worker/             # Worker pool implementations
```

## Data Architecture

### Primary Storage

**PostgreSQL 14+**
- Relational data storage with ACID guarantees
- pgvector extension for embeddings and semantic search
- JSONB for flexible schemas (agent manifests, tool configurations)
- Row-level security support for multi-tenancy
- Schema: `mcp.*` for all tables

**Key Tables**:
- **Organizations & Auth**:
  - `organizations`: Multi-tenant organization registry
  - `users`: User accounts with role-based access
  - `api_keys`: Multiple key types (admin, gateway, agent, user)

- **Agent Management**:
  - `agent_manifests`: Core agent definitions with capabilities
  - `agent_registrations`: Active agent instances
  - `agent_capabilities`: Capability registry with semantic search
  - `agent_channels`: Communication channel configurations

- **Dynamic Tools**:
  - `tool_configurations`: Tool registry with auth configurations
  - `tool_operations`: OpenAPI operations cache
  - `organization_tools`: Per-organization tool assignments

- **Embedding System**:
  - `embedding_model_catalog`: Global model registry (OpenAI, Bedrock, Google, Anthropic)
  - `tenant_embedding_models`: Per-tenant model configurations
  - `agent_embedding_preferences`: Agent-specific model preferences
  - `embedding_usage_tracking`: Usage and cost tracking

- **RAG System**:
  - `github_repositories`: Tracked repositories per organization
  - `repository_files`: Indexed file metadata
  - `code_embeddings`: Vector embeddings for semantic search

- **Webhook System**:
  - `webhook_events`: Event log
  - `webhook_dlq`: Dead letter queue for failed events

**Redis 7+**
- L2 cache for API responses and embeddings
- Session management for Edge MCP connections
- Distributed locks for coordination
- Rate limiting counters (per-agent, per-tenant, per-capability)
- Redis Streams for event distribution

### Object Storage

**AWS S3 (Optional)**
- Large context storage
- File attachments
- Backup and archival

### Message Queue

**Redis Streams**
- Webhook event processing via `webhook_events` stream
- Consumer groups for parallel processing
- Dead letter queue handling
- Reliable message delivery with acknowledgments
- Implementation: `pkg/redis/streams_client.go`

## MCP Protocol Implementation

### Protocol Details

Developer Mesh implements the Model Context Protocol 2025-06-18 specification:

- **Format**: JSON-RPC 2.0 over WebSocket
- **Endpoint**: `ws://localhost:8080/ws` (Edge MCP)
- **Authentication**: Bearer token via Authorization header
- **Connection Modes**: Claude Code, IDE, Agent, Standard MCP

### Connection Flow

```
1. Client → initialize (protocolVersion: "2025-06-18")
2. Server → capabilities (tools, resources, prompts)
3. Client → initialized
4. Client → tools/list (discover available tools)
5. Client → tools/call (execute tools)
6. Client → resources/read (read system state)
```

### DevMesh Tools Exposed via MCP

All DevMesh functionality is exposed as standard MCP tools:

- `devmesh.task.create` - Create tasks
- `devmesh.task.assign` - Assign tasks to agents
- `devmesh.task.status` - Get task status
- `devmesh.agent.assign` - Assign work to agents
- `devmesh.context.update` - Update session context
- `devmesh.context.get` - Get current context
- `devmesh.search.semantic` - Semantic search across codebase
- `devmesh.workflow.execute` - Execute predefined workflows
- `devmesh.workflow.list` - List available workflows

Plus all dynamically registered tools from configured integrations.

### DevMesh Resources

System state exposed through MCP resources:

- `devmesh://system/health` - System health and metrics
- `devmesh://agents/{tenant_id}` - List of registered agents
- `devmesh://workflows/{tenant_id}` - Available workflows
- `devmesh://context/{session_id}` - Current session context
- `devmesh://tasks/{tenant_id}` - Active tasks
- `devmesh://tools/{tenant_id}` - Available tools and configs

## Data Flow Patterns

### 1. Agent Connection Flow (MCP)

```
IDE/Agent → WebSocket → Edge MCP → initialize handshake
                           ↓
                    DevMesh Platform Auth
                           ↓
                    Tool Discovery (tools/list)
                           ↓
                    Resource Subscriptions
                           ↓
                    Ready for tool execution
```

### 2. Tool Execution Flow

```
Client → tools/call → Edge MCP → REST API → Tool Adapter
                                     ↓
                              Auth & Validation
                                     ↓
                              External Service
                                     ↓
                              Response → Client
```

### 3. Webhook Processing Flow

```
External Service → REST API → Redis Streams → Worker
                                 ↓                ↓
                           Acknowledge      Process Event
                                                  ↓
                                            Update Database
                                                  ↓
                                         Notify via WebSocket
```

### 4. RAG Loader Flow

```
GitHub Trigger → RAG Loader → Clone Repository
                                     ↓
                              Filter Files
                                     ↓
                              Chunk Code
                                     ↓
                         Generate Embeddings (Bedrock)
                                     ↓
                         Store in pgvector
                                     ↓
                         Available for Semantic Search
```

### 5. Embedding Generation Flow

```
Content → Model Selection Engine → Check Tenant Quotas
                  ↓                         ↓
           Circuit Breaker             Failover Logic
                  ↓                         ↓
           Provider API (Bedrock)    Alternative Provider
                  ↓
           Cost Tracking
                  ↓
           pgvector Storage
```

## Security Architecture

### Authentication & Authorization

- **JWT Tokens**: Stateless authentication for API access
- **API Keys**: Multiple types with different privileges:
  - Admin keys: Full platform access
  - Gateway keys: Service-to-service auth
  - Agent keys: Agent-specific permissions
  - User keys: User-scoped access
- **OAuth 2.0**: Interface defined for third-party integrations (no provider implementations yet)
- **Organization Isolation**: Automatic tenant separation at all levels

### Credential Management

- **Per-Tenant Encryption**: AES-256-GCM with unique keys per tenant
- **Key Derivation**: PBKDF2 for deriving encryption keys from master key
- **Forward Secrecy**: Unique salt/nonce per encryption operation
- **Authenticated Encryption**: GCM mode prevents tampering
- **Secrets Management**: Integration with AWS Secrets Manager

### Tenant Isolation

- **Strict Mode**: Complete isolation between organizations
- **Agent Discovery**: Filtered by organization automatically
- **Database Row-Level Security**: PostgreSQL RLS for data isolation
- **Rate Limiting**: Per-tenant limits with custom configuration
- **Audit Logging**: All cross-tenant attempts logged

### Data Protection

- **Encryption at Rest**: Database encryption, S3 encryption
- **Encryption in Transit**: TLS 1.3 minimum
- **Credential Encryption**: All API keys and tokens encrypted at rest
- **Audit Logging**: Comprehensive logging of all operations

## Observability

### Metrics (Prometheus)

- Request rates and latencies per endpoint
- Error rates by service and endpoint
- Queue depths and processing times
- Agent connection counts and health
- Embedding generation costs and latencies
- Circuit breaker states
- Cache hit rates (L1 and L2)

### Tracing (OpenTelemetry)

- Distributed request tracing across services
- Cross-service correlation with trace IDs
- Performance bottleneck identification
- Webhook processing traces

### Logging (Structured)

- JSON-formatted logs via `pkg/observability`
- Contextual information (trace ID, tenant ID, user ID)
- Log aggregation support
- Configurable log levels (Error, Warn, Info, Debug)

## Deployment Architecture

### Local Development

```yaml
docker-compose.local.yml:
  - postgres:latest (with pgvector extension)
  - redis:latest
  - edge-mcp (port 8080)
  - rest-api (port 8081)
  - worker (background)
  - rag-loader (on-demand)
```

### Production (AWS)

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   ALB/NLB   │────▶│  ECS Tasks  │────▶│  RDS        │
│  (TLS)      │     │  (Fargate)  │     │(PostgreSQL) │
└─────────────┘     └─────────────┘     └─────────────┘
                           │
                           ▼
                    ┌─────────────┐     ┌─────────────┐
                    │Redis Streams│     │ ElastiCache │
                    │  (Events)   │     │   (Redis)   │
                    └─────────────┘     └─────────────┘
                           │
                           ▼
                    ┌─────────────┐     ┌─────────────┐
                    │AWS Bedrock  │     │     S3      │
                    │(Embeddings) │     │  (Storage)  │
                    └─────────────┘     └─────────────┘
```

## Performance Considerations

### Multi-Level Caching

1. **L1 Cache (Memory)**: Hot embeddings and tool operations (<1ms)
2. **L2 Cache (Redis)**: Distributed cache with TTL management (<10ms)
3. **Database (PostgreSQL)**: Persistent storage with indexes
4. **Cost Cache**: Model pricing data for routing decisions

### Embedding Optimization

1. **Batch Processing**: Reduce API calls to providers
2. **Provider Failover**: Automatic switching on rate limits or failures
3. **Quality/Cost Trade-offs**: Configurable model selection strategies
4. **Cache Hit Rates**: Minimize regeneration costs
5. **Circuit Breakers**: Prevent cascade failures

### Scalability

1. **Horizontal Scaling**: Multiple instances behind load balancer
2. **Worker Parallelization**: Consumer groups for parallel processing
3. **Connection Pooling**: Database and Redis connection pools
4. **Stream Partitioning**: Distribute load across Redis consumer groups

## Resilience Patterns

### Circuit Breakers
- Prevent cascading failures from external services
- Configurable failure thresholds and recovery times
- Implementation in `pkg/circuitbreaker/`

### Retry Logic
- Exponential backoff with jitter
- Configurable max attempts
- Implementation in `pkg/retry/`

### Bulkheads
- Isolate failures to specific components
- Separate connection pools per service

### Health Checks
- Liveness probes: Service is running
- Readiness probes: Service can handle traffic
- Database connectivity checks
- Redis connectivity checks

## Collaboration Features

### CRDT-Based Collaborative Editing

Advanced CRDT (Conflict-free Replicated Data Type) implementations in `pkg/collaboration/`:

- **DocumentCRDT**: Collaborative text editing with fractional indexing
- **StateCRDT**: Distributed state management with path-based updates
- **Vector Clocks**: Causality tracking for distributed operations
- **Implemented CRDTs**:
  - GCounter: Grow-only counter
  - PNCounter: Increment/decrement counter
  - LWWRegister: Last-write-wins register
  - ORSet: Observed-remove set

## Migration Path

Database migrations are managed per service:

- **REST API**: `apps/rest-api/migrations/sql/`
- **RAG Loader**: `apps/rag-loader/migrations/`
- **Migration Tool**: `pkg/database/migration/`

Migrations use golang-migrate with up/down SQL files. Run with:
```bash
make migrate-up    # Apply migrations
make migrate-down  # Rollback migrations
```

## Future Considerations

1. **Advanced AI Orchestration**:
   - Hierarchical agent organizations
   - Learning-based task routing from historical data
   - Agent capability evolution
   - Multi-modal agent support

2. **Enhanced Collaboration**:
   - Full CRDT delta synchronization
   - Conflict resolution strategies
   - Real-time collaborative debugging

3. **Enterprise Features**:
   - Casbin RBAC (planned)
   - OAuth provider integrations (interface defined)
   - Advanced audit logging
   - SAML/SSO support

4. **Performance Enhancements**:
   - GPU-accelerated embeddings
   - Edge agent deployment
   - Predictive task scheduling
   - Adaptive compression

5. **Integration Expansion**:
   - Additional DevOps tool adapters
   - Cloud provider agnostic design
   - Kubernetes operator
   - GitOps workflow automation

## References

- [Go Workspace Structure](go-workspace-structure.md) - Multi-module organization
- [Multi-Agent Embedding Architecture](multi-agent-embedding-architecture.md) - Embedding system design
- [Universal Agent Architecture](universal-agent-architecture.md) - Agent registration and coordination
- [Package Dependencies](package-dependencies.md) - Module relationships
- [MCP Protocol Documentation](../reference/mcp-protocol/MCP_PROTOCOL.md) - Complete protocol details
- [REST API Reference](../reference/api/rest-api-reference.md) - HTTP API documentation
- [Edge MCP Documentation](../../apps/edge-mcp/README.md) - Edge MCP gateway details

---

**Document Status**: Verified against codebase
**Last Updated**: 2025-10-17
**Verified Components**: All file paths, table schemas, and service descriptions
