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

## Current Focus Areas
- Redis Streams migration (completed)
- Dynamic tools implementation with enhanced discovery
- Multi-tenant embedding model management (completed)
- MCP (Model Context Protocol) migration (completed)
- Multi-agent orchestration improvements
- Security hardening
- Test coverage expansion

## MCP Protocol Migration (Completed)

### Overview
The platform has been migrated from a dual-protocol system (custom + MCP) to MCP-only for all agent communication. This provides a standardized, JSON-RPC 2.0-based protocol for AI agent interaction.

### MCP Protocol Details
- **Version**: 2024-11-05
- **Format**: JSON-RPC 2.0 over WebSocket
- **Connection**: WebSocket at `/ws` endpoint

### Agent Connection via MCP
```json
// Initialize connection
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "clientInfo": {
      "name": "agent-name",
      "version": "1.0.0",
      "type": "ide"  // or "ci", "documentation", etc.
    }
  }
}
```

### Custom Protocol Features as MCP Tools
Previous custom protocol features are now exposed as MCP tools:

| Custom Protocol | MCP Tool | Description |
|----------------|----------|-------------|
| `agent.register` | `initialize` | Agent registration via MCP initialize |
| `agent.heartbeat` | `agent.heartbeat` | Heartbeat tool |
| `workflow.create` | `workflow.create` | Create workflow tool |
| `workflow.execute` | `workflow.execute` | Execute workflow tool |
| `task.create` | `task.create` | Create task tool |
| `task.assign` | `task.assign` | Assign task tool |
| `task.complete` | `task.complete` | Complete task tool |
| `context.update` | `context.update` | Update context tool |

### MCP Resources
Read-only access to system state via resources:

| Resource URI | Description |
|-------------|-------------|
| `workflow/*` | Workflow information |
| `workflow/*/status` | Workflow execution status |
| `task/*` | Task information |
| `task/*/status` | Task status |
| `context/*` | Session context |
| `agent/*` | Agent information |
| `system/health` | System health status |

### Example MCP Operations

#### List Available Tools
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list"
}
```

#### Execute a Tool
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "workflow.create",
    "arguments": {
      "name": "deployment-workflow",
      "steps": [...]
    }
  }
}
```

#### Read a Resource
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "resources/read",
  "params": {
    "uri": "task/task-123/status"
  }
}
```

### Implementation Details
- **Protocol Adapter**: `/pkg/adapters/mcp/protocol_adapter.go` - Converts custom protocol features to MCP
- **Resource Provider**: `/pkg/adapters/mcp/resources/resource_provider.go` - Provides MCP resources
- **MCP Handler**: `/apps/mcp-server/internal/api/mcp_protocol.go` - Main MCP protocol handler
- **Migration Tests**: `/apps/mcp-server/internal/api/websocket/mcp_migration_test.go`

### Testing MCP Connection
```bash
# Test MCP connection
wscat -c ws://localhost:8080/ws

# Send initialize message
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test-agent","version":"1.0.0"}}}

# List available tools
{"jsonrpc":"2.0","id":2,"method":"tools/list"}

# Execute a tool
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"agent.heartbeat","arguments":{"agent_id":"test-agent"}}}
```

### Backward Compatibility
The system maintains dual-protocol support during transition:
- MCP messages (containing `"jsonrpc":"2.0"`) are routed to MCP handler
- Custom protocol messages are still processed but deprecated
- Full migration to MCP-only is recommended

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