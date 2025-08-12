# MCP Server - Model Context Protocol Server

## Service Overview
The MCP Server is the core WebSocket server implementing the Model Context Protocol (MCP) 2025-06-18 specification for AI agent communication and orchestration. It provides:
- Full MCP protocol support over WebSocket
- JSON-RPC 2.0 message handling
- DevMesh-specific tools exposed as standard MCP tools
- Connection mode detection (Claude Code, IDE, Agent)
- Dynamic tool management and execution
- Context management for agent sessions
- Embedding operations via AWS Bedrock
- Real-time event streaming and resource updates
- Multi-agent task coordination

## Architecture
- **Protocol**: MCP 2025-06-18 over WebSocket
- **Format**: JSON-RPC 2.0
- **Port**: 8080 (configurable)
- **Framework**: Gin for HTTP, gorilla/websocket
- **Dependencies**: PostgreSQL, Redis, AWS Bedrock

## Key Components

### Core Engine (`internal/core/`)
- `Engine`: Main orchestration engine
- `ContextManager`: Manages agent context with S3 storage
- `AdapterContextBridge`: Bridges adapters with context
- `System`: System-level operations

### API Layer (`internal/api/`)
- `Server`: Main server with routing
- `WebSocketServer`: Handles WS connections
- `DynamicToolsAPI`: Tool management endpoints
- `EmbeddingProxy`: AWS Bedrock proxy

### Services (`internal/services/`)
- `ToolService`: Dynamic tool orchestration
- `DiscoveryService`: API discovery
- `ExecutionService`: Tool execution
- `HealthChecker`: Tool health monitoring
- `CredentialManager`: Secure credential handling

### Adapters (`internal/adapters/`)
- `OpenAPIAdapter`: OpenAPI spec handling
- Tool-specific adapters for execution

## MCP Protocol Implementation

### Protocol Version
- **Version**: 2025-06-18
- **Format**: JSON-RPC 2.0
- **Transport**: WebSocket

### Connection Initialization
```json
// Client initializes connection
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-06-18",
    "clientInfo": {
      "name": "client-name",
      "version": "1.0.0",
      "type": "ide"  // or "ci", "documentation", etc.
    }
  }
}

// Server responds with capabilities
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2025-06-18",
    "serverInfo": {
      "name": "devmesh-mcp-server",
      "version": "1.0.0"
    },
    "capabilities": {
      "tools": {},
      "resources": {
        "subscribe": true
      },
      "prompts": {},
      "logging": {}
    }
  }
}
```

### Connection Modes
The server detects and optimizes for different client types:

| Mode | Detection | Optimizations |
|------|-----------|---------------|
| **Claude Code** | User-Agent contains "Claude-Code" | Optimized responses, batch operations |
| **IDE** | X-IDE-Name header present | Rich error messages, debugging info |
| **Agent** | X-Agent-ID header present | Minimal responses, high throughput |
| **Standard MCP** | Default | Full MCP compliance |

## MCP Methods Supported

### Standard MCP Methods
- `initialize` - Initialize connection
- `initialized` - Confirm initialization
- `ping` - Heartbeat check
- `shutdown` - Graceful disconnect
- `$/cancelRequest` - Cancel in-flight request
- `tools/list` - List available tools
- `tools/call` - Execute a tool
- `resources/list` - List available resources
- `resources/read` - Read resource content
- `resources/subscribe` - Subscribe to resource updates
- `resources/unsubscribe` - Unsubscribe from updates
- `prompts/list` - List available prompts
- `prompts/get` - Get prompt content
- `logging/setLevel` - Set logging level
- `completion/complete` - Get completions

### DevMesh Extensions
- `x-devmesh/agent/register` - Register agent
- `x-devmesh/search/semantic` - Semantic search
- `x-devmesh/embeddings/generate` - Generate embeddings

## DevMesh Tools (MCP Tools)

All DevMesh functionality is exposed as standard MCP tools with the `devmesh.` namespace:

| Tool | Description | Arguments |
|------|-------------|-----------|  
| `devmesh.workflow.create` | Create new workflow | `name`, `description`, `steps` |
| `devmesh.workflow.execute` | Execute workflow | `workflow_id`, `parameters` |
| `devmesh.workflow.list` | List workflows | `status`, `limit` |
| `devmesh.task.create` | Create task | `title`, `type`, `priority` |
| `devmesh.task.assign` | Assign task | `task_id`, `agent_id` |
| `devmesh.task.complete` | Complete task | `task_id`, `result` |
| `devmesh.context.update` | Update context | `context` object |
| `devmesh.context.get` | Get context | none |

## MCP Resources

Resources use the `devmesh://` URI scheme:

| Resource URI | Description | Subscribe |
|-------------|-------------|-----------|  
| `devmesh://workflow/*` | Workflow details | ✓ |
| `devmesh://workflow/*/status` | Workflow status | ✓ |
| `devmesh://task/*` | Task details | ✓ |
| `devmesh://task/*/status` | Task status | ✓ |
| `devmesh://context/*` | Session context | ✓ |
| `devmesh://agent/*` | Agent information | ✓ |
| `devmesh://system/health` | System health | ✓ |
| `devmesh://system/metrics` | System metrics | ✓ |

## API Endpoints
- `GET /health` - Health check
- `GET /ws` - MCP WebSocket endpoint
- `POST /api/v1/tools` - Create tool (REST)
- `GET /api/v1/tools` - List tools (REST)
- `POST /api/v1/tools/:id/execute` - Execute tool (REST)
- `POST /api/v1/embeddings` - Generate embeddings

## Testing

### Unit Tests
```bash
# Run all tests
cd apps/mcp-server && go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/core/...

# Test MCP protocol handler
go test ./internal/api -run TestMCPProtocol
```

### MCP Protocol Testing
```bash
# Test standard MCP methods
./scripts/test-mcp-standard.sh

# Test with persistent session
./scripts/test-mcp-session.sh

# Validate response structures
./scripts/test-mcp-validation.sh

# Test with wscat
wscat -c ws://localhost:8080/ws
> {"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}
> {"jsonrpc":"2.0","id":2,"method":"tools/list"}
> {"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"devmesh.workflow.list","arguments":{}}}
```

## Common Issues

### WebSocket Connection Drops
- Check heartbeat configuration
- Verify client implements ping/pong
- Check proxy timeout settings

### Context Manager S3 Errors
- Verify AWS credentials
- Check S3 bucket permissions
- Ensure bucket exists in region

### Tool Execution Failures
- Check tool health status
- Verify credentials are valid
- Check network connectivity

## Security Considerations
- All WebSocket connections require authentication
- Tool credentials are encrypted at rest
- API keys validated with regex pattern
- Circuit breakers for external calls

## Configuration
```yaml
# Key settings in config.yaml
api:
  host: "0.0.0.0"
  port: 8080
  cors:
    enabled: true
    
websocket:
  heartbeat_interval: 30s
  max_message_size: 1048576
  
tools:
  health_check_interval: 5m
  execution_timeout: 30s
```

## Debugging Tips

### MCP Connection Testing
```bash
# Test MCP connection with websocat
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18"},"id":1}' | \
  websocat -n1 ws://localhost:8080/ws

# Test as Claude Code client
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18"},"id":1}' | \
  websocat -n1 --header="User-Agent: Claude-Code/1.0.0" ws://localhost:8080/ws

# Monitor logs
docker-compose logs -f mcp-server | grep MCP

# Check tool health
curl http://localhost:8080/api/v1/tools/:id/health
```

### Common MCP Issues
1. **"Method not found" errors**: Check protocol version compatibility
2. **Connection drops**: Ensure proper ping/pong handling
3. **Tool execution failures**: Verify tool name includes `devmesh.` namespace
4. **Resource not found**: Check URI uses `devmesh://` scheme

## Performance Tuning
- Connection pool size: 100 (database)
- WebSocket buffer size: 1MB
- Context cache TTL: 5 minutes
- Health check parallelism: 10

## Integration Points
- **REST API**: Via REST client factory
- **PostgreSQL**: Main data store
- **Redis**: Caching and pub/sub
- **AWS Bedrock**: Embeddings
- **S3**: Context storage

## Development Workflow
1. Make changes in `internal/` directories
2. Run `go test ./...` to verify
3. Update API tests if endpoints change
4. Test WebSocket with mock clients
5. Verify health endpoints work

## Important Files

### MCP Implementation
- `internal/api/mcp_protocol.go` - Main MCP protocol handler
- `internal/api/websocket/connection.go` - Connection mode detection
- `internal/api/websocket/server.go` - WebSocket server with MCP routing
- `pkg/adapters/mcp/protocol_adapter.go` - Protocol conversion layer
- `pkg/adapters/mcp/resources/resource_provider.go` - MCP resource provider

### Core Server Files
- `cmd/server/main.go` - Entry point
- `internal/api/server.go` - Server setup
- `internal/core/engine.go` - Core logic
- `internal/api/config.go` - Configuration

## Metrics & Monitoring
- WebSocket connections: `mcp.websocket.connections`
- Tool executions: `mcp.tools.executions`
- Context operations: `mcp.context.operations`
- Embedding requests: `mcp.embeddings.requests`

## Error Patterns
```go
// Always wrap errors with context
if err != nil {
    return fmt.Errorf("failed to execute tool %s: %w", toolID, err)
}

// Log WebSocket errors
if err := ws.WriteMessage(messageType, data); err != nil {
    s.logger.Error("Failed to write WebSocket message", map[string]interface{}{
        "error": err.Error(),
        "agent_id": agentID,
    })
}
```

## Testing Patterns
- Mock WebSocket connections
- Use testify for assertions
- Test tool execution with mocks
- Verify context persistence
- Check error scenarios

## MCP Implementation Details

### Message Router
The MCP handler routes messages based on JSON-RPC method:
```go
// In mcp_protocol.go
switch msg.Method {
    case "initialize":
        return h.handleInitialize(conn, connID, tenantID, msg)
    case "tools/call":
        return h.handleToolCall(conn, connID, tenantID, msg)
    // ... other methods
}
```

### Tool Execution
DevMesh tools are executed through the standard MCP tools/call method:
```go
func (h *MCPHandler) executeDevMeshTool(toolName string, args map[string]interface{}) (interface{}, error) {
    switch toolName {
    case "devmesh.workflow.create":
        return h.createWorkflow(args)
    case "devmesh.task.create":
        return h.createTask(args)
    // ... other tools
    }
}
```

### Session Management
- Sessions persist across the WebSocket connection
- Context is maintained per connection
- Agent registration happens via initialize method
- Graceful shutdown preserves session state

## Never Do
- Don't store credentials in logs
- Don't skip WebSocket ping/pong
- Don't ignore close errors
- Don't hardcode AWS regions
- Don't bypass auth checks
- Don't mix MCP and custom protocol messages
- Don't return raw errors to MCP clients (wrap in proper error response)