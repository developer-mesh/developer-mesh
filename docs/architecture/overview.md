<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:36:40
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# Developer Mesh System Architecture

## Overview

Developer Mesh is an AI Agent Orchestration Platform that enables intelligent routing and coordination of multiple AI agents for DevOps workflows. Built using Go workspaces for modularity, it provides sophisticated multi-agent orchestration, real-time collaboration features, and production-ready AWS integrations.

## Architecture Principles

- **Microservices Architecture**: Three independent services communicating via APIs and events
- **AI-Native Design**: Built from the ground up for multi-agent orchestration and coordination
- **Clean Architecture**: Clear separation between business logic, adapters, and infrastructure
- **Event-Driven Design**: Real-time WebSocket communication with asynchronous task processing <!-- Source: pkg/models/websocket/binary.go -->
- **Go Workspace**: Monorepo with multiple modules for code sharing and independent deployment
- **Cloud-Native**: Production AWS integration with Bedrock, S3, and ElastiCache

## System Components

### 🔵 MCP Server (`apps/mcp-server`)

The MCP Server is the core AI agent orchestration hub with **Universal Agent Registration**:

- **Universal Agent Registry**: Manages any agent type (IDE, Slack, monitoring, CI/CD, custom)
- **Agent Manifest System**: Flexible configuration with capabilities, requirements, and auth
- **Organization Isolation**: Strict tenant separation with cross-org access control
- **Task Assignment Engine**: Intelligent routing with multiple strategies (capability-match, least-loaded, cost-optimized) <!-- Source: pkg/services/assignment_engine.go -->
- **Binary WebSocket Protocol**: High-performance communication with compression support <!-- Source: pkg/models/websocket/binary.go -->
- **Multi-Agent Collaboration**: Orchestrates complex workflows across multiple AI agents

Key Features:
- **Universal agent support** for any tool or service type
- **Capability-based discovery** across different agent types
- **Tenant isolation** with organization-level security boundaries
- **Cross-agent messaging** (IDE→Jira, Slack→IDE, Monitoring→Slack)
- Binary WebSocket protocol with automatic gzip compression (>1KB messages) <!-- Source: pkg/models/websocket/binary.go -->
- Real-time agent discovery and capability-based routing
- Workload tracking and dynamic load balancing
- Task delegation and collaboration patterns (MapReduce, parallel, pipeline)
- **Multi-level rate limiting** (per-agent, per-tenant, per-capability)
- **Circuit breaker patterns** for resilient agent communication with auto-recovery

### 🟢 REST API Service (`apps/rest-api`)

The REST API provides HTTP endpoints for external integrations:

- **Agent Management**: Register agents, query capabilities, monitor workload
- **Task Submission**: Submit tasks with routing preferences and requirements <!-- Source: pkg/services/assignment_engine.go -->
- **Embedding Operations**: Generate and search embeddings via AWS Bedrock
- **Tool Integration**: GitHub adapter for DevOps workflow automation

Key Features:
- All endpoints use `/api/v1/*` path prefix
- Multi-model embedding support (Titan, Cohere, Claude)
- Cost tracking and optimization for AI operations
- JWT and API key authentication
- Comprehensive Swagger/OpenAPI documentation

### 🟠 Worker Service (`apps/worker`)

The Worker handles distributed task processing:

- **Task Distribution**: Processes tasks assigned to AI agents
- **Embedding Pipeline**: Batch processing for vector embeddings
- **Notification Delivery**: Sends real-time updates via WebSocket <!-- Source: pkg/models/websocket/binary.go -->
- **Workflow Coordination**: Manages multi-step AI workflows

Key Features:
- Redis Streams for reliable event processing and task delivery
- Concurrent processing with agent workload awareness
- Cost tracking for AI model usage
- Dead letter queue for failed task handling

### 📦 Shared Libraries (`pkg/`)

Reusable packages across all services:

```
pkg/
├── adapters/       # External service integrations
├── common/         # Shared utilities and types
├── database/       # Database abstractions and migrations
├── embedding/      # Vector embedding services
├── models/         # Domain models and entities
├── observability/  # Logging, metrics, tracing
└── repository/     # Data access patterns
```

## Data Architecture

### Primary Storage

**PostgreSQL 14+**
- Relational data storage
- pgvector extension for embeddings and capability search
- JSONB for flexible schemas (agent manifests, requirements)
- Row-level security support
- **Agent manifest tables** for universal registration:
  - `agent_manifests`: Core agent definitions with capabilities
  - `agent_registrations`: Active agent instances
  - `agent_capabilities`: Capability registry with semantic search
  - `agent_channels`: Communication channel configurations

**Redis 7+**
- Response caching
- Session management
- Distributed locks
- Rate limiting counters

### Object Storage

**AWS S3 (Optional)**
- Large context storage
- File attachments
- Backup and archival

### Message Queue

**Redis Streams** <!-- Source: pkg/redis/streams_client.go -->
- Event distribution via streams
- Task queuing with consumer groups
- Dead letter queue handling
- Reliable message delivery
- Webhook event processing

## Collaboration Features

### CRDT-Based Collaborative Editing

The platform includes advanced CRDT (Conflict-free Replicated Data Type) implementations for real-time collaboration:

- **DocumentCRDT**: Collaborative text editing with fractional indexing
- **StateCRDT**: Distributed state management with path-based updates
- **Vector Clocks**: Causality tracking for distributed operations
- **Implemented CRDTs**:
  - GCounter (grow-only counter)
  - PNCounter (increment/decrement counter)
  - LWWRegister (last-write-wins register)
  - ORSet (observed-remove set)

### Binary WebSocket Protocol <!-- Source: pkg/models/websocket/binary.go -->

High-performance binary protocol for agent communication: <!-- Source: pkg/models/websocket/binary.go -->

```
Header (12 bytes):
┌─────────┬───────┬──────────────┬──────────────┬──────────┐
│ Version │ Flags │ Message Type │ Payload Size │ Reserved │
│ 1 byte  │ 1 byte│   2 bytes    │   4 bytes    │  4 bytes │
└─────────┴───────┴──────────────┴──────────────┴──────────┘

Features:
- Automatic gzip compression for messages > 1KB
- Message batching for improved throughput
- Buffer pooling for reduced GC pressure
- Max payload size: ~4GB
- Max decompressed size: 10MB (security limit)
```

## Data Flow Patterns

### 1. Universal Agent Registration Flow

```
Agent → WebSocket → MCP Server → Manifest Validation <!-- Source: pkg/models/websocket/binary.go -->
                         ↓
                 Organization Binding
                         ↓
                  Agent Registry → Database
                         ↓              ↓
                 Capability Index   Tenant Config
                         ↓              ↓
                 Task Router Update  Rate Limits <!-- Source: pkg/services/assignment_engine.go -->
                         ↓
                 Discovery Service
```

### 2. Task Assignment Flow

```
Task Request → REST API → Assignment Engine → Agent Selection <!-- Source: pkg/services/assignment_engine.go -->
                                    ↓
                            WebSocket Notification <!-- Source: pkg/models/websocket/binary.go -->
                                    ↓
                               Agent Processing
```

### 3. Multi-Agent Collaboration Flow

```
Initiator Agent → Task Delegation → Agent Discovery
                         ↓
                 Organization Filter ← Tenant Config
                         ↓
                 Capability Matching
                         ↓
                 Cross-Agent Routing → Message Broker
                         ↓                    ↓
                 Target Agent(s)     Rate Limiter/Circuit Breaker
                         ↓
                 Parallel Execution → Result Aggregation
```

### 5. Cross-Agent Message Flow

```
Source Agent → Message Broker → Capability Router
                     ↓                 ↓
              Organization Check   Target Discovery
                     ↓                 ↓
               Rate Limiting      Available Agents
                     ↓                 ↓
              Circuit Breaker     Load Balancing
                     ↓                 ↓
               Redis Stream   →  Target Agent <!-- Source: pkg/redis/streams_client.go -->
                                      ↓
                               Message Handler
```

### 4. Vector Embedding Flow

```
Content → Bedrock API → Embedding Generation
                              ↓
                         Cost Tracking
                              ↓
                    pgvector Storage → Similarity Search
```

## Integration Patterns

### Adapter Pattern

All external integrations use the adapter pattern:

```go
type ToolAdapter interface {
    Execute(ctx context.Context, action string, params map[string]interface{}) (interface{}, error)
    GetCapabilities() []Capability
}
```

Benefits:
- Isolation of external dependencies
- Consistent interface across tools
- Easy testing with mocks
- Gradual migration support

### Repository Pattern

Data access follows the repository pattern:

```go
type Repository[T any] interface {
    Create(ctx context.Context, entity T) (T, error)
    Get(ctx context.Context, id string) (T, error)
    List(ctx context.Context, filter Filter) ([]T, error)
    Update(ctx context.Context, entity T) (T, error)
    Delete(ctx context.Context, id string) error
}
```

## Security Architecture

### Authentication & Authorization

- **JWT Tokens**: Stateless authentication (implemented)
- **API Keys**: Multi-type keys (admin, gateway, agent, user) with different privileges
- **OAuth 2.0**: Third-party integrations (interface defined, no provider implementations yet)
- **RBAC**: Role-based access control (Casbin planned, not yet implemented)
- **Organization Isolation**: Automatic tenant separation at all levels

### Tenant Isolation

- **Strict Mode**: Complete isolation between organizations
- **Agent Discovery**: Filtered by organization automatically
- **Message Routing**: Cross-org communication blocked by default
- **Rate Limiting**: Per-tenant limits with custom configuration
- **Audit Logging**: All cross-org attempts logged

### Data Protection

- **Encryption at Rest**: Database and S3 encryption
- **Encryption in Transit**: TLS 1.3 minimum
- **Secrets Management**: AWS Secrets Manager integration
- **Audit Logging**: All access logged and monitored

## Observability

### Metrics (Prometheus)

- Request rates and latencies
- Error rates by endpoint
- Queue depths and processing times
- Resource utilization

### Tracing (OpenTelemetry)

- Distributed request tracing
- Cross-service correlation
- Performance bottleneck identification

### Logging (Structured)

- JSON-formatted logs
- Contextual information
- Log aggregation support
- Configurable log levels

## Deployment Architecture

### Local Development

```yaml
docker-compose:
  - postgres (with pgvector)
  - redis
  - Services (hot reload)
```

### Production (AWS)

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   ALB/NLB   │────▶│  ECS Tasks  │────▶│     RDS     │
└─────────────┘     └─────────────┘     └─────────────┘
                           │                     
                           ▼                     
                    ┌─────────────┐     ┌─────────────┐
                    │Redis Streams│     │ ElastiCache │
                    └─────────────┘     └─────────────┘
```

### Kubernetes (Future)

- Helm charts for deployment
- Horizontal pod autoscaling
- Service mesh integration
- GitOps workflow

## Performance Considerations

### AI Agent Performance

1. **Task Routing**: Optimized routing decisions with cached capabilities <!-- Source: pkg/services/assignment_engine.go -->
2. **Binary Protocol**: Significant message size reduction with compression <!-- Source: pkg/models/websocket/binary.go -->
3. **Connection Pooling**: Reusable WebSocket connections per agent <!-- Source: pkg/models/websocket/binary.go -->
4. **Workload Balancing**: Real-time load distribution across agents

### Multi-Level Caching

1. **Memory Cache**: Hot embeddings and agent capabilities
2. **Redis Cache**: Distributed cache for agent state and embeddings
3. **Database Cache**: Persistent storage with pgvector indexes
4. **Cost Cache**: Model pricing data for routing decisions

### Embedding Optimization

1. **Batch Processing**: Reduce API calls to Bedrock
2. **Provider Failover**: Automatic switching on rate limits
3. **Quality/Cost Trade-offs**: Configurable routing strategies
4. **Cache Hit Rates**: Minimize regeneration costs

### Scalability

1. **Agent Scaling**: Designed for high concurrency with multiple AI agents
2. **Task Parallelization**: MapReduce patterns for large workloads
3. **Circuit Breakers**: Prevent cascade failures
4. **Stream Partitioning**: Distribute load across Redis consumer groups

## Resilience Patterns

### Circuit Breakers
Prevent cascading failures from external services

### Retry Logic
Exponential backoff with jitter

### Bulkheads
Isolate failures to specific components

### Health Checks
Liveness and readiness probes

## Universal Agent Registration System

### Architecture Components

1. **Agent Manifest System**:
   - Flexible agent definitions with type, capabilities, requirements
   - Dynamic configuration without code changes
   - Version tracking and compatibility management
   - Authentication configuration per agent type

2. **Enhanced Registry**:
   - Extends existing DBAgentRegistry through embedding
   - Backward compatible with existing agents
   - Universal agent support without breaking changes
   - Real-time health and status tracking

3. **Rate Limiting Architecture**:
   - **Per-Agent**: Individual agent limits (10 RPS default)
   - **Per-Tenant**: Organization-wide limits (100 RPS default)
   - **Per-Capability**: Capability-specific limits (50 RPS default)
   - **Burst Capacity**: 1.5x multiplier for traffic spikes
   - **Sliding Windows**: Accurate rate tracking

4. **Circuit Breaker System**:
   - **Agent Breakers**: Trip after 3 failures, 20s recovery
   - **Capability Breakers**: Trip after 10 failures, 60s recovery
   - **Tenant Breakers**: Trip after 20 failures, 120s recovery
   - **Channel Breakers**: For communication channels
   - **Auto-Recovery**: Half-open state for gradual recovery

5. **Message Broker**:
   - Redis Streams for reliable delivery <!-- Source: pkg/redis/streams_client.go -->
   - Worker pools with consumer groups
   - Priority queuing (1-10 scale)
   - Dead letter queue for failures
   - Capability-based routing

### Supported Agent Types

- **IDE Agents**: VS Code, IntelliJ, Neovim (code assistance)
- **Slack Agents**: Notifications, alerts, team coordination
- **Monitoring Agents**: Prometheus, DataDog (metrics, health)
- **CI/CD Agents**: Jenkins, GitHub Actions (builds, deployments)
- **Custom Agents**: Any tool with WebSocket support <!-- Source: pkg/models/websocket/binary.go -->

## Future Architecture Considerations

1. **Advanced AI Orchestration**:
   - Hierarchical agent organizations with delegation
   - Learning-based task routing from historical data <!-- Source: pkg/services/assignment_engine.go -->
   - Agent capability evolution and specialization
   - Multi-modal agent support (text, voice, video)

2. **Enhanced Collaboration**:
   - Full CRDT delta synchronization
   - Conflict resolution strategies
   - Real-time collaborative debugging
   - Agent consensus mechanisms

3. **Enterprise Features**:
   - Casbin RBAC implementation (planned, not yet implemented)
   - OAuth provider integrations (interface defined, no implementations yet)
   - Advanced audit logging
   - Multi-tenant agent isolation

4. **Performance Enhancements**:
   - GPU-accelerated embeddings
   - Edge agent deployment
   - Predictive task scheduling
   - Adaptive compression algorithms

5. **Integration Expansion**:
   - Additional DevOps tool adapters
   - Cloud provider agnostic design
   - Kubernetes operator for agents
   - GitOps workflow automation

## References

- [Go Workspace Structure](go-workspace-structure.md)
- [Adapter Pattern Implementation](adapter-pattern.md)
- [API Documentation](../api-reference/vector-search-api.md)
