# Edge MCP Integration Guides

This directory contains comprehensive integration guides for connecting various AI code editors and MCP clients to Edge MCP.

## Available Guides

### AI Code Editors

- **[Claude Code Integration](./claude-code.md)** - Integrate Edge MCP with Claude Code, Anthropic's official CLI
- **[Cursor Integration](./cursor.md)** - Connect Edge MCP to Cursor, the AI-first code editor
- **[Windsurf Integration](./windsurf.md)** - Set up Edge MCP with Windsurf IDE by Codeium

### Generic Clients

- **[Generic MCP Client Guide](./generic-mcp-client.md)** - Integrate any MCP-compatible client using the standard protocol

### Troubleshooting

- **[Troubleshooting Guide](./troubleshooting.md)** - Comprehensive troubleshooting for all Edge MCP integrations

## Quick Start

### 1. Start Edge MCP

```bash
# Local development
cd apps/edge-mcp
export EDGE_MCP_API_KEY=dev-admin-key-1234567890
go run cmd/server/main.go

# Docker
docker-compose -f docker-compose.local.yml up edge-mcp

# Kubernetes
helm install edge-mcp ./deployments/k8s/helm/edge-mcp
```

### 2. Choose Your Integration

Select the integration guide for your preferred tool:

| Tool | Best For | Guide |
|------|----------|-------|
| Claude Code | CLI-based AI coding | [claude-code.md](./claude-code.md) |
| Cursor | Full IDE with AI pair programming | [cursor.md](./cursor.md) |
| Windsurf | Codeium-powered IDE | [windsurf.md](./windsurf.md) |
| Custom Client | Building custom integrations | [generic-mcp-client.md](./generic-mcp-client.md) |

### 3. Configure Your Client

Each guide includes:
- Prerequisites and requirements
- Step-by-step configuration
- Usage examples
- Advanced configuration options
- Platform-specific troubleshooting

### 4. Verify Connection

After configuration, verify Edge MCP connection:

1. Check Edge MCP is running:
   ```bash
   curl http://localhost:8082/health/ready
   ```

2. List available tools in your client
3. Execute a test tool (e.g., `github_get_repository`)

## Common Configuration

### Authentication

All clients require an API key for authentication:

```json
{
  "headers": {
    "Authorization": "Bearer dev-admin-key-1234567890"
  }
}
```

### WebSocket URL

- **Local Development:** `ws://localhost:8082/ws`
- **Production (TLS):** `wss://edge-mcp.your-domain.com/ws`
- **Kubernetes (Port Forward):** `ws://localhost:8082/ws`

### Passthrough Authentication

For GitHub and Harness tools, provide service-specific credentials:

```json
{
  "headers": {
    "Authorization": "Bearer dev-admin-key-1234567890",
    "X-GitHub-Token": "ghp_yourGitHubToken",
    "X-Harness-API-Key": "pat.yourHarnessKey"
  }
}
```

## Features Available

All integrations provide access to:

- **200+ DevOps Tools**: GitHub, Harness, and built-in orchestration
- **Tool Categories**: Repository, Issues, CI/CD, Workflows, Agents, and more
- **Batch Execution**: Run multiple tools in parallel
- **Response Streaming**: Automatic for large responses (>32KB)
- **Context Management**: Session-based context tracking
- **Rate Limiting**: Per-tenant and per-tool limits with quotas
- **Semantic Errors**: AI-friendly error messages with recovery steps

## Architecture

```
┌─────────────┐
│ AI Client   │ (Claude Code, Cursor, Windsurf, Custom)
└──────┬──────┘
       │ WebSocket (JSON-RPC 2.0)
       │ ws://localhost:8082/ws
       ▼
┌─────────────┐
│  Edge MCP   │
└──────┬──────┘
       │
       ├─► Built-in Tools (Agent, Workflow, Task, Context)
       │
       └─► Core Platform ─► Dynamic Tools (GitHub, Harness)
```

## Environment Variables

Configure Edge MCP behavior:

```bash
# Server
export EDGE_MCP_PORT=8082
export EDGE_MCP_API_KEY=your-api-key

# Core Platform (optional)
export DEV_MESH_URL=http://localhost:8081
export DEV_MESH_API_KEY=your-core-api-key

# Rate Limiting
export EDGE_MCP_TENANT_RPS=100
export EDGE_MCP_TOOL_RPS=50

# Caching (optional)
export REDIS_ENABLED=true
export REDIS_URL=redis://localhost:6379

# Tracing (optional)
export TRACING_ENABLED=true
export OTLP_ENDPOINT=localhost:4317
```

## Deployment Scenarios

### Local Development

- Single developer
- No Redis required (memory-only cache)
- Hot reload during development

### Team Development

- Multiple developers
- Shared Edge MCP instance
- Redis for distributed caching
- Port forwarding from Kubernetes

### Production

- Kubernetes deployment with HA
- Redis cluster for caching
- TLS/SSL (wss://)
- Horizontal pod autoscaling
- Prometheus monitoring

See [Kubernetes Deployment Guide](../../deployments/k8s/README.md) for production setup.

## Troubleshooting

If you encounter issues:

1. **Check Edge MCP Health:**
   ```bash
   curl http://localhost:8082/health/ready
   ```

2. **View Logs:**
   ```bash
   docker-compose logs -f edge-mcp
   ```

3. **Test WebSocket:**
   ```bash
   websocat ws://localhost:8082/ws
   ```

4. **Consult Troubleshooting Guide:**
   See [troubleshooting.md](./troubleshooting.md) for comprehensive solutions

## Support

- **Documentation:** [Edge MCP Docs](../)
- **API Reference:** [OpenAPI Spec](../openapi/edge-mcp.yaml)
- **Issues:** GitHub Issues
- **Examples:** [Example Clients](../openapi/examples/)

## Next Steps

After setting up your integration:

1. **Explore Tools:** Use `tools/list` to discover available tools
2. **Tool Categories:** Filter tools by category (repository, issues, ci_cd, etc.)
3. **Batch Operations:** Combine multiple tools for efficiency
4. **Agent Orchestration:** Delegate complex tasks to agents
5. **Context Management:** Use session context for stateful interactions

## Related Documentation

- [OpenAPI Specification](../openapi/edge-mcp.yaml)
- [Kubernetes Deployment](../../deployments/k8s/README.md)
- [Error Handling](../error-handling.md)
- [Tool Usage Examples](../tool-usage-examples.md)
- [Cache Configuration](../deployment/cache-configuration.md)
