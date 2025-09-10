# REST API - Data Management & Integration Service

## Service Overview
The REST API is the primary data management service providing:
- CRUD operations for all entities
- Dynamic tool discovery and management
- Multi-provider embedding generation
- Webhook ingestion and processing
- Agent and model configuration
- Semantic search capabilities

## Architecture
- **Protocol**: REST/HTTP
- **Port**: 8081 (configurable)
- **Framework**: Gin
- **Dependencies**: PostgreSQL, Redis, AWS/OpenAI/Google APIs

## Key Components

### API Handlers (`internal/api/`)
- `DynamicToolsAPI`: Tool discovery and management
- `EmbeddingAPI`: Multi-provider embeddings
- `AgentAPI`: Agent configuration
- `ModelAPI`: Model management
- `MCPAPI`: MCP protocol operations
- `WebhookServer`: Webhook ingestion

### Services (`internal/services/`)
- `DynamicToolsService`: Tool orchestration
- `EmbeddingService`: Provider routing
- `EnhancedDiscoveryService`: API discovery
- `AgentService`: Agent management
- `ModelService`: Model configuration

### Storage (`internal/storage/`)
- `DiscoveryPatternRepository`: Learning patterns
- `ToolRepository`: Tool persistence
- `AgentRepository`: Agent data
- `ModelRepository`: Model configs

## API Endpoints

### Dynamic Tools
- `POST /api/v1/tools` - Create tool with discovery
- `GET /api/v1/tools` - List tools
- `GET /api/v1/tools/:id` - Get tool details
- `DELETE /api/v1/tools/:id` - Delete tool
- `POST /api/v1/tools/:id/execute/:action` - Execute action
- `GET /api/v1/tools/:id/health` - Check health
- `POST /api/v1/tools/discover` - Start discovery
- `POST /api/v1/tools/discover-multiple` - Multi-API discovery

### Embeddings
- `POST /api/v1/embeddings` - Generate embeddings
- `POST /api/v1/embeddings/search` - Semantic search
- `GET /api/v1/embeddings/models` - List models
- `GET /api/v1/embeddings/providers` - List providers

### Webhooks
- `POST /api/webhooks/tools/:id` - Tool webhooks
- `POST /api/webhooks/github` - GitHub events
- `POST /api/webhooks/generic` - Generic webhooks

### Agent Management
- `POST /api/v1/agents` - Create agent
- `GET /api/v1/agents` - List agents
- `PUT /api/v1/agents/:id` - Update agent
- `DELETE /api/v1/agents/:id` - Delete agent

## Database Schema
```sql
-- Key tables
tool_configurations      -- Dynamic tools
tool_discovery_sessions  -- Discovery tracking
discovery_patterns       -- Learning system
agents                   -- Agent configs
models                   -- Model definitions
embeddings              -- Vector storage
webhook_events          -- Event queue
```

## Redis Usage
- **Streams**: Webhook event queue
- **Cache**: Tool specs, embeddings
- **Pub/Sub**: Real-time updates
- **Keys**: Rate limiting, deduplication

## Testing
```bash
# Run all tests
cd apps/rest-api && go test ./...

# Integration tests (needs DB)
go test -tags=integration ./...

# Specific service
go test ./internal/services/...
```

## Common Issues

### Tool Discovery Failures
- Check network connectivity
- Verify API endpoint is accessible
- Check discovery patterns table
- Review auth configuration

### Tool Action Execution
- **"Operation not found"**: System now intelligently resolves action names
  - Simple verbs (`get`, `list`) are mapped to full operation IDs
  - Uses parameters to determine correct operation (e.g., `repo` → `repos/get`)
  - Supports multiple formats: `repos/get`, `repos-get`, `repos_get`
- **Parameter mapping**: For GitHub tools, parameters may be nested under `"parameters"`
  - System automatically unwraps based on tool provider
  - Check logs for parameter structure being sent

### Webhook Processing
- Monitor Redis stream lag
- Check consumer group status
- Verify worker is running
- Check DLQ for failures

### Embedding Errors
- Verify provider API keys
- Check rate limits
- Monitor costs
- Verify model availability

## Security
- API key validation on all endpoints
- Encrypted credential storage
- SQL injection prevention
- Rate limiting per tenant
- Webhook signature validation

## Configuration
```yaml
# Key settings
api:
  port: 8081
  rate_limit: 100
  
database:
  max_connections: 100
  
redis:
  streams:
    webhook_events: "webhook_events"
    
embedding:
  providers:
    openai:
      enabled: true
      models: ["text-embedding-3-small"]
    bedrock:
      enabled: true
      region: "us-east-1"
```

## Performance Tuning
- Database connection pool: 100
- Redis connection pool: 50
- HTTP client timeout: 30s
- Batch embedding size: 100
- Cache TTL: 5m for specs

## Integration Points
- **MCP Server**: Via direct DB access
- **Worker**: Via Redis streams
- **External APIs**: Via dynamic tools
- **Webhooks**: Via HTTP endpoints

## Development Workflow
1. Add new endpoint in appropriate API handler
2. Implement service logic
3. Add repository methods if needed
4. Write unit and integration tests
5. Update OpenAPI spec
6. Test with curl/Postman

## Important Files
- `cmd/api/main.go` - Entry point
- `internal/api/server.go` - Server setup
- `internal/api/dynamic_tools_api.go` - Tools API
- `internal/services/dynamic_tools_service.go` - Core logic
- `internal/api/webhook_server.go` - Webhook handling

## Debugging
```bash
# Check health
curl http://localhost:8081/health

# List tools
curl -H "X-API-Key: $KEY" http://localhost:8081/api/v1/tools

# Monitor Redis
redis-cli xinfo stream webhook_events

# Check logs
docker-compose logs -f rest-api
```

## Error Patterns
```go
// Service layer errors
if err != nil {
    return nil, fmt.Errorf("failed to create tool: %w", err)
}

// API layer errors
if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{
        "error": "Failed to create tool",
        "details": err.Error(),
    })
    return
}
```

## Testing Patterns
- Mock service dependencies
- Use httptest for API tests
- Test error scenarios
- Verify JSON responses
- Check status codes

## Metrics
- `api.request.duration` - Request latency
- `api.request.count` - Request count
- `tools.discovery.duration` - Discovery time
- `embeddings.generation.count` - Embedding ops
- `webhooks.processed` - Webhook count

## Never Do
- Don't expose internal errors to API
- Don't skip input validation
- Don't ignore webhook signatures
- Don't cache sensitive data
- Don't bypass rate limits