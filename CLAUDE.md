# Developer Mesh - AI Agent Orchestration Platform

## Project Overview
Developer Mesh is a production-ready platform for orchestrating multiple AI agents in DevOps workflows. It consists of:
- **MCP Server**: WebSocket server for real-time agent communication
- **REST API**: Dynamic tools integration and management
- **Worker**: Redis-based webhook and event processing
- **Shared Packages**: Common functionality in `/pkg`

## Architecture
- **Language**: Go 1.24+ with workspace support
- **Databases**: PostgreSQL 14+ with pgvector, Redis 7+
- **Message Queue**: Redis Streams (migrated from AWS SQS)
- **Cloud**: AWS (Bedrock, S3)
- **Protocols**: MCP (Model Context Protocol) over WebSocket, REST, gRPC

## Key Commands
- Build: `make build`
- Test: `make test`
- Lint: `make lint`
- Format: `make fmt`
- Pre-commit: `make pre-commit`
- Dev environment: `make dev`
- Docker: `docker-compose -f docker-compose.local.yml up`

## Project Structure
```
/apps
  /mcp-server     # WebSocket server for agent communication
  /rest-api       # REST API for tools and integrations
  /worker         # Redis worker for async processing
  /mockserver     # Mock server for testing
/pkg              # Shared packages
/migrations       # Database migrations
/configs          # Configuration files
/scripts          # Utility scripts
/docs             # Documentation
/test             # Test suites
```

## Development Workflow
1. **Before starting work**: Check branch with `git status`
2. **Before committing**: Run `make pre-commit`
3. **Testing**: Always write tests for new features
4. **Code style**: Follow Go idioms, use gofmt
5. **Security**: Use parameterized queries, validate inputs

## Go-Specific Patterns (This Project)

### Code Style
- **Indentation**: Tabs for Go (as per gofmt)
- **Import Groups**: stdlib, external, internal (blank line between)
- **Comments**: Package comments required, function comments for exported items
- **Line Length**: Follow gofmt, no hard limit
- **Error Handling**: Always wrap with context using `fmt.Errorf("context: %w", err)`

### Testing
- **Framework**: Always use testify/assert and testify/mock
- **Mocks**: Define in same test file or dedicated mock file
- **Table Tests**: Preferred for multiple scenarios
- **Coverage**: Minimum 80% coverage for new code
- **Test files**: In same package (not `_test` package)
- **Run tests**: `make test` or `go test ./...` in specific module

### Error Handling
```go
// Always wrap errors with context
if err != nil {
    return nil, fmt.Errorf("failed to query tools: %w", err)
}

// For deferred operations
defer func() {
    if err := rows.Close(); err != nil {
        s.logger.Warn("Failed to close rows", map[string]interface{}{
            "error": err.Error(),
        })
    }
}()
```

### Architecture Patterns
- **Context**: Always first parameter in functions
- **Defer Close**: Immediately after resource creation with error handling
- **Dependency Injection**: Via constructors
- **Interfaces**: Defined where used, not where implemented
- **Services**: Should be stateless
- **Circuit Breakers**: For external calls

### Database Patterns
- Use repository pattern (see pkg/repository)
- Always use sqlx over database/sql
- Named queries for clarity
- Scan into structs, not individual variables
- Handle sql.ErrNoRows explicitly

### Logging Standards
- Use structured logging via observability.Logger
- Never use fmt.Printf or println
- Log levels: Error (failures), Warn (recoverable), Info (important), Debug (detailed)
- Include context in log fields

### Security Practices
- **SQL**: Always parameterized queries, never string concatenation
- **API Keys**: Validate with regex `^[a-zA-Z0-9_-]+$`
- **Credentials**: Always encrypt using EncryptionService
- **Input Validation**: Required at handler level
- **No DEBUG prints**: Remove all debug statements before commit

### Common Gotchas to Avoid
- Don't ignore Close() errors in deferred functions
- Don't use panic except in main()
- Don't leave TODO comments - create issues instead
- Don't commit commented-out code
- Don't use magic numbers - create constants

## Current Focus Areas
- Redis Streams migration (completed)
- Dynamic tools implementation with enhanced discovery
- Multi-tenant embedding model management (completed)
- MCP (Model Context Protocol) migration (completed)
- Multi-agent orchestration improvements
- Security hardening
- Test coverage expansion

## MCP Protocol Implementation (Complete)

### Overview
DevMesh fully implements the Model Context Protocol (MCP) 2025-06-18 specification for standardized AI agent communication. The platform exposes all DevMesh capabilities through standard MCP tools and resources.

### MCP Protocol Details
- **Version**: 2025-06-18 (Industry Standard)
- **Format**: JSON-RPC 2.0 over WebSocket
- **Endpoint**: `ws://localhost:8080/ws` (WebSocket)
- **Authentication**: Bearer token via Authorization header
- **Connection Modes**: Claude Code, IDE, Agent, Standard MCP

### Quick Start - Connect with MCP Client
```bash
# Using websocat (for testing)
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"my-client","version":"1.0.0"}},"id":"1"}' | \
  websocat --header="Authorization: Bearer YOUR_API_KEY" ws://localhost:8080/ws

# The server will respond with capabilities
```

### Connection Initialization
```json
// 1. Initialize connection
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-06-18",
    "clientInfo": {
      "name": "your-client",
      "version": "1.0.0"
    }
  }
}

// 2. Server responds with capabilities
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2025-06-18",
    "serverInfo": {
      "name": "developer-mesh-mcp",
      "version": "1.0.0"
    },
    "capabilities": {
      "tools": {"listChanged": true},
      "resources": {"subscribe": true, "listChanged": true},
      "prompts": {"listChanged": true}
    }
  }
}

// 3. Confirm initialization (required)
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "initialized",
  "params": {}
}
```

### DevMesh Tools Available via MCP

All DevMesh features are exposed as standard MCP tools. Use `tools/list` to discover them dynamically.

#### Core DevMesh Tools
| Tool Name | Description | Key Parameters |
|-----------|-------------|----------------|
| `devmesh_agent_assign` | Assign task to specialized AI agent | `agent_type`, `task`, `priority` |
| `devmesh_context_update` | Update session context | `context`, `merge` |
| `devmesh_context_get` | Retrieve current context | `keys` (optional) |
| `devmesh_search_semantic` | Semantic search across codebase | `query`, `limit`, `filters` |
| `devmesh_workflow_execute` | Execute predefined workflow | `workflow_id`, `parameters` |
| `devmesh_workflow_list` | List available workflows | `category`, `tags` |
| `devmesh_task_create` | Create new task | `title`, `type`, `priority` |
| `devmesh_task_status` | Get/update task status | `task_id`, `status` |

#### Legacy Protocol Tools (Still Available)
| Tool Name | Description |
|-----------|-------------|
| `agent_heartbeat` | Send agent heartbeat |
| `agent_status` | Get agent status |
| `workflow_create` | Create new workflow |
| `workflow_cancel` | Cancel workflow execution |
| `task_assign` | Assign task to agent |
| `task_complete` | Mark task as complete |
| `context_update` | Update context |
| `context_append` | Append to context |

### MCP Resources

DevMesh exposes system state through MCP resources. Use `resources/list` to discover available resources.

#### DevMesh Resources (devmesh:// URI scheme)
| Resource URI | Description | Content Type |
|-------------|-------------|--------------|
| `devmesh://agents/{tenant_id}` | List of registered AI agents | `application/json` |
| `devmesh://workflows/{tenant_id}` | Available workflows | `application/json` |
| `devmesh://context/{session_id}` | Current session context | `application/json` |
| `devmesh://tasks/{tenant_id}` | Active tasks in system | `application/json` |
| `devmesh://tools/{tenant_id}` | Available tools and configs | `application/json` |
| `devmesh://system/health` | System health and metrics | `application/json` |
| `devmesh://session/{id}/info` | Session information | `application/json` |

#### Standard Resources
| Resource URI | Description |
|-------------|-------------|
| `agent/*` | Agent information |
| `agent/*/capabilities` | Agent capabilities |
| `workflow/*` | Workflow details |
| `workflow/*/status` | Workflow execution status |
| `task/*` | Task information |
| `task/*/status` | Task status |
| `context/*` | Session context |
| `system/metrics` | System metrics |

### Example MCP Operations

#### 1. List Available Tools
```json
// Request
{
  "jsonrpc": "2.0",
  "id": "tools-1",
  "method": "tools/list"
}

// Response includes all DevMesh tools
{
  "jsonrpc": "2.0",
  "id": "tools-1",
  "result": {
    "tools": [
      {
        "name": "devmesh_agent_assign",
        "description": "Assign task to specialized AI agent",
        "inputSchema": {...}
      },
      // ... more tools
    ]
  }
}
```

#### 2. Execute a DevMesh Tool
```json
// Create a task
{
  "jsonrpc": "2.0",
  "id": "task-1",
  "method": "tools/call",
  "params": {
    "name": "devmesh_task_create",
    "arguments": {
      "title": "Review PR #123",
      "type": "code_review",
      "priority": "high"
    }
  }
}

// Response
{
  "jsonrpc": "2.0",
  "id": "task-1",
  "result": {
    "content": [{
      "type": "text",
      "text": "{\"id\":\"task-123\",\"status\":\"created\",...}"
    }]
  }
}
```

#### 3. Read System Health Resource
```json
// Request
{
  "jsonrpc": "2.0",
  "id": "health-1",
  "method": "resources/read",
  "params": {
    "uri": "devmesh://system/health"
  }
}

// Response with health metrics
{
  "jsonrpc": "2.0",
  "id": "health-1",
  "result": {
    "contents": [{
      "uri": "devmesh://system/health",
      "mimeType": "application/json",
      "text": "{\"status\":\"healthy\",\"connections\":5,...}"
    }]
  }
}
```

### Connection Mode Detection

DevMesh automatically detects the type of client connecting:

| Client Type | Detection Method | Special Features |
|------------|------------------|------------------|
| **Claude Code** | User-Agent: `Claude-Code/*` or Header: `X-Claude-Code-Version` | Optimized for multi-file operations |
| **IDE** | User-Agent contains `VSCode`, `Cursor` or Header: `X-IDE-Name` | IDE-specific features |
| **Agent** | Header: `X-Agent-ID` or `X-Agent-Type` | Persistent connections |
| **Standard MCP** | Default for any MCP client | Full MCP compliance |

### Implementation Files

| Component | Location | Description |
|-----------|----------|-------------|
| **MCP Handler** | `/apps/mcp-server/internal/api/mcp_protocol.go` | Core MCP protocol implementation |
| **WebSocket Server** | `/apps/mcp-server/internal/api/websocket/server.go` | WebSocket handling and routing |
| **Connection Manager** | `/apps/mcp-server/internal/api/websocket/connection.go` | Connection state management |
| **Protocol Adapter** | `/pkg/adapters/mcp/protocol_adapter.go` | Legacy protocol conversion |
| **Resource Provider** | `/pkg/adapters/mcp/resources/resource_provider.go` | Resource management |

### Testing MCP Connection

```bash
# Quick test with websocat
./scripts/test-mcp-standard.sh      # Full test suite
./scripts/test-mcp-session.sh       # Session-based tests
./scripts/test-mcp-validation.sh    # Response validation

# Manual testing with websocat
websocat --header="Authorization: Bearer dev-admin-key-1234567890" \
  ws://localhost:8080/ws

# Then send:
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test","version":"1.0.0"}}}
{"jsonrpc":"2.0","id":2,"method":"initialized","params":{}}
{"jsonrpc":"2.0","id":3,"method":"tools/list"}
```

### MCP Compliance

DevMesh fully implements the MCP 2025-06-18 specification:
- ✅ All required methods implemented
- ✅ Standard error codes (JSON-RPC 2.0)
- ✅ Tool discovery and execution
- ✅ Resource listing and reading
- ✅ Subscription support
- ✅ Session management
- ✅ Graceful shutdown

## Testing Guidelines
- Unit tests: In same package as code
- Integration tests: In `/test/functional`
- E2E tests: In `/test/e2e`
- Run specific service tests: `cd apps/SERVICE && go test ./...`
- Coverage: Aim for >80% on new code

## Database
- **PostgreSQL**: Main data store with pgvector for embeddings
- **Redis**: Caching, pub/sub, and streams for webhooks
- Migrations: `make migrate-up` / `make migrate-down`
- Schema: See `/migrations` directory

## Security Considerations
- **API Keys**: Use regex validation `^[a-zA-Z0-9_-]+$`
- **SQL**: Always use parameterized queries
- **Credentials**: Encrypt with `pkg/security/EncryptionService`
- **Auth**: Bearer tokens, API keys, OAuth2 supported
- **Input Validation**: Required for all user inputs

## Dynamic Tools System
- **Discovery**: Automatic API discovery with learning
- **Formats**: OpenAPI, Swagger, custom JSON
- **Auth**: Universal authentication support
- **Health**: Automatic health monitoring
- **Testing**: Use mockserver for tool testing
- **Operation Resolution**: Intelligent mapping of action names to OpenAPI operations
  - Handles simple verbs (`get`, `list`, `create`) and full operation IDs
  - Context-aware resolution using provided parameters
  - Supports multiple naming conventions (slash/hyphen/underscore)
  - Automatic disambiguation when multiple operations match

## Edge MCP Session Management
- **Session Lifecycle**: Create, refresh, validate, terminate sessions
- **Authentication**: JWT + refresh tokens with passthrough auth encryption
- **Storage**: PostgreSQL persistence with automatic expiry
- **Caching**: Redis L1 cache for performance
- **Tool Tracking**: Complete audit trail of tool executions per session
- **Multi-tenant**: Per-tenant session limits and isolation
- **Database Tables**:
  - `mcp.edge_mcp_sessions`: Session storage with metadata
  - `mcp.session_tool_executions`: Tool execution audit trail
- **REST API Endpoints**:
  - `POST /api/v1/sessions` - Create new session
  - `GET /api/v1/sessions/:id` - Get session details
  - `POST /api/v1/sessions/:id/refresh` - Refresh session TTL
  - `DELETE /api/v1/sessions/:id` - Terminate session
  - `POST /api/v1/sessions/:id/validate` - Validate session status
  - `GET /api/v1/sessions` - List sessions with filtering
  - `GET /api/v1/sessions/active` - List active sessions
  - `GET /api/v1/sessions/metrics` - Session metrics
  - `POST /api/v1/sessions/:id/tools/execute` - Record tool execution
  - `GET /api/v1/sessions/:id/tools/executions` - Get execution history
- **Key Services**:
  - `SessionService`: Business logic and orchestration
  - `SessionRepository`: Database operations
  - `SessionHandler`: REST API handlers
- **Configuration**:
  - Default TTL: 24 hours (configurable)
  - Max sessions per tenant: 100 (configurable)
  - Idle timeout: 30 minutes (configurable)

## Multi-Tenant Embedding Model Management
- **Model Catalog**: Global registry of all available embedding models
- **Tenant Configuration**: Per-tenant model access and limits
- **Agent Preferences**: Fine-grained model selection per agent
- **Usage Tracking**: Comprehensive usage and cost tracking
- **Quota Management**: Monthly/daily token and request limits
- **Model Selection**: Intelligent model selection based on tenant/agent/task
- **Database Tables**:
  - `mcp.embedding_model_catalog`: Global model registry
  - `mcp.tenant_embedding_models`: Tenant-specific configurations
  - `mcp.agent_embedding_preferences`: Agent-level preferences
  - `mcp.embedding_usage_tracking`: Usage and cost tracking
- **Key Services**:
  - `ModelManagementService`: Core model selection and quota logic
  - `ModelCatalogRepository`: CRUD for model catalog
  - `TenantModelsRepository`: Tenant model configurations
  - `EmbeddingUsageRepository`: Usage tracking and reporting
- **Test Data**: Run `scripts/db/seed-embedding-models.sql` to populate test tenants

## Webhook Processing
- **Producer**: REST API receives webhooks
- **Queue**: Redis Streams with consumer groups
- **Worker**: Processes events asynchronously
- **DLQ**: Dead letter queue for failed messages
- **Monitoring**: Prometheus metrics for all stages

## Performance Optimization
- **Circuit Breakers**: For external API calls
- **Connection Pooling**: Database and Redis
- **Caching**: Redis with TTL management
- **Compression**: Binary WebSocket protocol
- **Batch Processing**: For bulk operations

## Error Handling
- **Logging**: Structured logging with `pkg/observability`
- **Metrics**: Prometheus for monitoring
- **Tracing**: OpenTelemetry for distributed tracing
- **Alerts**: Based on error rates and latencies

## Git Workflow
- Feature branches: `feature/description`
- Commits: Clear, concise messages
- PRs: Detailed description with test plan
- Reviews: Required before merge to main

## Environment Variables
- Development: `.env.development`
- Docker: `.env.docker`
- Production: Never commit, use secrets manager
- Required vars: See `configs/config.base.yaml`

## Common Issues & Solutions
1. **Import errors**: Run `go work sync`
2. **Test failures**: Check Redis/Postgres are running
3. **Lint errors**: Run `make fmt` then `make lint`
4. **Docker issues**: `docker-compose down -v` and restart

## Code Quality Standards
- No DEBUG print statements in production code
- All exported functions must have comments
- Error messages should be actionable
- Avoid magic numbers, use named constants
- Prefer dependency injection over globals

## Integration Points
- **GitHub**: Via dynamic tools API
- **AWS Bedrock**: Multiple embedding models
- **Vector Search**: pgvector for semantic search
- **Monitoring**: Prometheus + Grafana stack

## When Making Changes
- Update tests for modified code
- Update documentation if behavior changes
- Check for security implications
- Consider backward compatibility
- Add metrics for new features

## Quick Debug Commands
```bash
# Check service health
curl http://localhost:8080/health  # MCP
curl http://localhost:8081/health  # REST API

# View logs
docker-compose logs -f mcp-server
docker-compose logs -f rest-api
docker-compose logs -f worker

# Database queries
psql -h localhost -U devmesh -d devmesh_development

# Redis monitoring
redis-cli monitor
redis-cli xinfo groups webhook_events
```

## 🚀 Productivity Shortcuts

### Quick Testing
```bash
# Test current module (auto-detects from pwd)
if [[ $PWD == *"mcp-server"* ]]; then go test ./...; \
elif [[ $PWD == *"rest-api"* ]]; then go test ./...; \
elif [[ $PWD == *"worker"* ]]; then go test ./...; \
else make test; fi

# Run specific test with coverage
go test -cover -run TestName ./path/to/package
```

### Common Workflows
```bash
# Full pre-commit flow
make pre-commit && git add -A && git commit -m "feat: description"

# Quick PR creation
gh pr create --fill

# Update feature branch
git stash && git checkout main && git pull && git checkout - && git rebase main && git stash pop
```

### Service-Specific Commands
```bash
# Restart specific service
docker-compose restart mcp-server  # or rest-api, worker

# Tail specific service logs
docker-compose logs -f --tail=100 rest-api

# Check Redis queue depth
redis-cli xlen webhook_events

# Quick DB query
psql -h localhost -U devmesh -d devmesh_development -c "SELECT COUNT(*) FROM tool_configurations;"
```

### Emergency Fixes
```bash
# Clear stuck Redis stream
redis-cli DEL webhook_events

# Reset consumer group
redis-cli XGROUP DESTROY webhook_events webhook_workers
redis-cli XGROUP CREATE webhook_events webhook_workers 0

# Kill stuck processes
pkill -f "mcp-server|rest-api|worker"
```

## 🔧 Technical Implementation Details

### Operation Resolver (`pkg/tools/operation_resolver.go`)
The OperationResolver provides intelligent mapping between simple action names and OpenAPI operation IDs:

**Resolution Strategies** (applied in order):
1. **Direct Lookup** - Exact match for operation ID
2. **Contextual Resolution** - Uses parameters to infer resource type:
   - `owner`/`repo` → `repos/*` operations
   - `issue_number` → `issues/*` operations
   - `pull_number` → `pulls/*` operations
3. **Simple Name Mapping** - Extracts action verbs (`get`, `list`, `create`)
4. **Fuzzy Matching** - Handles format variations (`/`, `-`, `_`)
5. **Disambiguation Scoring** - Selects best match when multiple candidates exist

**Integration Points**:
- `DynamicToolAdapter`: Builds mappings when OpenAPI spec is loaded
- `findOperationWithContext`: Passes parameters as context for resolution
- `ExecuteAction`: Uses resolved operation for API calls

**Example Resolution Flow**:
```
Input: action="get", params={"owner": "org", "repo": "myrepo"}
↓ Context detection: has "repo" parameter
↓ Resource inference: "repos"
↓ Candidate generation: ["repos/get", "repos-get", "repos_get"]
↓ Operation lookup: found "repos/get"
Output: OpenAPI operation for GET /repos/{owner}/{repo}
```