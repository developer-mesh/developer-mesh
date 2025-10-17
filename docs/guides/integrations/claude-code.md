# Claude Code Integration Guide

This guide explains how to integrate Edge MCP with Claude Code, Anthropic's official CLI tool for Claude.

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Quick Start](#quick-start)
4. [Configuration](#configuration)
5. [Usage Examples](#usage-examples)
6. [Advanced Configuration](#advanced-configuration)
7. [Troubleshooting](#troubleshooting)

## Overview

Claude Code automatically detects Edge MCP as an MCP server when properly configured. Edge MCP provides:

- **200+ DevOps Tools**: GitHub, Harness, and built-in agent orchestration tools
- **Intelligent Tool Discovery**: Categorized tools with AI-friendly metadata
- **Batch Execution**: Execute multiple tools in parallel
- **Response Streaming**: Automatic streaming for large responses (>32KB)
- **Rate Limiting**: Per-tenant and per-tool rate limits
- **Error Recovery**: Semantic errors with recovery suggestions

## Prerequisites

### Required

- Claude Code CLI installed
- Edge MCP server running locally or remotely
- Valid API key for authentication

### Optional

- Core Platform connection (for dynamic tool discovery)
- Redis (for distributed caching in production)

## Quick Start

### 1. Start Edge MCP Server

**Local Development:**
```bash
cd apps/edge-mcp
go run cmd/server/main.go
```

**Using Docker:**
```bash
docker-compose -f docker-compose.local.yml up edge-mcp
```

**Kubernetes:**
```bash
helm install edge-mcp deployments/k8s/helm/edge-mcp/
```

### 2. Configure MCP Server in Claude Code

Claude Code uses MCP configuration files to discover servers. Create or update your MCP configuration file:

**Location:** `~/.claude/mcp.json` (Linux/macOS) or `%APPDATA%\Claude\mcp.json` (Windows)

```json
{
  "mcpServers": {
    "edge-mcp": {
      "transport": "websocket",
      "url": "ws://localhost:8082/ws",
      "headers": {
        "Authorization": "Bearer dev-admin-key-1234567890"
      },
      "description": "Edge MCP - DevOps Tool Integration",
      "supportsStreaming": true,
      "capabilities": {
        "tools": true,
        "resources": true,
        "prompts": false
      }
    }
  }
}
```

### 3. Verify Connection

Start Claude Code and verify the connection:

```bash
claude
```

In Claude Code, list available tools:
```
/tools list
```

You should see tools like:
- `github_get_repository`
- `github_list_issues`
- `harness_pipelines_list`
- `devmesh_agent_assign`
- And many more...

## Configuration

### Environment Variables

Edge MCP can be configured via environment variables:

```bash
# Server configuration
export EDGE_MCP_PORT=8082                    # Server port (default: 8082)
export EDGE_MCP_API_KEY=your-api-key-here    # Authentication key

# Core Platform integration (optional)
export DEV_MESH_URL=http://localhost:8081    # Core Platform URL
export DEV_MESH_API_KEY=your-core-api-key    # Core Platform API key
export EDGE_MCP_ID=edge-mcp-01              # Unique Edge MCP instance ID

# Rate limiting
export EDGE_MCP_GLOBAL_RPS=1000             # Global requests/sec
export EDGE_MCP_TENANT_RPS=100              # Per-tenant requests/sec
export EDGE_MCP_TOOL_RPS=50                 # Per-tool requests/sec

# Redis cache (optional, for production)
export REDIS_ENABLED=true
export REDIS_URL=redis://localhost:6379

# Tracing (optional)
export TRACING_ENABLED=true
export OTLP_ENDPOINT=localhost:4317         # Jaeger OTLP endpoint
```

### API Key Authentication

Edge MCP supports two authentication methods:

1. **Bearer Token (Recommended):**
   ```json
   "headers": {
     "Authorization": "Bearer your-api-key"
   }
   ```

2. **API Key Header:**
   ```json
   "headers": {
     "X-API-Key": "your-api-key"
   }
   ```

### Passthrough Authentication

For GitHub and Harness tools, you can provide service-specific credentials:

```json
{
  "mcpServers": {
    "edge-mcp": {
      "transport": "websocket",
      "url": "ws://localhost:8082/ws",
      "headers": {
        "Authorization": "Bearer dev-admin-key-1234567890",
        "X-GitHub-Token": "ghp_yourGitHubToken",
        "X-Harness-API-Key": "your-harness-key",
        "X-Harness-Account-ID": "your-account-id"
      }
    }
  }
}
```

Edge MCP will use these credentials when calling GitHub/Harness APIs.

## Usage Examples

### Example 1: List GitHub Repositories

```
claude> List all repositories in the developer-mesh organization
```

Claude Code will automatically:
1. Discover the `github_list_repositories` tool
2. Call the tool with appropriate parameters
3. Format and display the results

### Example 2: Batch Operations

```
claude> Get GitHub repository info for developer-mesh/developer-mesh
       and list all open issues in that repo
```

Claude Code will:
1. Execute `github_get_repository` and `github_list_issues` in parallel
2. Combine the results intelligently
3. Present a unified response

### Example 3: Workflow Orchestration

```
claude> Create a new GitHub issue titled "Bug: Login fails"
       in developer-mesh/developer-mesh, then assign it to an agent for analysis
```

Claude Code will:
1. Call `github_create_issue`
2. Call `devmesh_agent_assign` with the issue details
3. Report the task assignment result

### Example 4: Using Context

```
claude> Remember my current project is developer-mesh/developer-mesh
```

Later:
```
claude> List open pull requests
```

Claude Code will use the stored context to fill in the repository details automatically.

## Advanced Configuration

### Connection Mode Detection

Edge MCP automatically detects Claude Code and optimizes its behavior:

- **Client Detection:** Via `User-Agent: Claude-Code/*` header
- **Optimizations:** Multi-file operations, enhanced error messages
- **Streaming:** Automatic for responses >32KB

### Timeout Configuration

Configure timeouts for long-running operations:

```json
{
  "mcpServers": {
    "edge-mcp": {
      "transport": "websocket",
      "url": "ws://localhost:8082/ws",
      "headers": {
        "Authorization": "Bearer dev-admin-key-1234567890"
      },
      "timeout": 60000,           // Connection timeout (ms)
      "requestTimeout": 120000    // Request timeout (ms)
    }
  }
}
```

### Reconnection Strategy

Claude Code automatically handles reconnections. Edge MCP supports:

- **Keepalive Pings:** Server sends pings every 30s
- **Session Persistence:** Sessions maintained for 24 hours (configurable)
- **Graceful Reconnect:** Automatic session restoration

### Local vs. Remote Deployment

**Local Development:**
```json
{
  "url": "ws://localhost:8082/ws"
}
```

**Remote Server:**
```json
{
  "url": "wss://edge-mcp.your-domain.com/ws"
}
```

**Kubernetes (Port Forward):**
```bash
kubectl port-forward -n edge-mcp svc/edge-mcp 8082:8082
```

Then use `ws://localhost:8082/ws` in configuration.

### TLS/SSL Configuration

For production deployments with TLS:

```json
{
  "mcpServers": {
    "edge-mcp": {
      "transport": "websocket",
      "url": "wss://edge-mcp.your-domain.com/ws",
      "headers": {
        "Authorization": "Bearer your-production-api-key"
      },
      "tlsVerify": true
    }
  }
}
```

## Troubleshooting

### Connection Issues

**Problem:** Claude Code cannot connect to Edge MCP

**Solutions:**
1. Verify Edge MCP is running:
   ```bash
   curl http://localhost:8082/health/ready
   ```
   Should return: `{"status":"healthy",...}`

2. Check WebSocket connectivity:
   ```bash
   websocat ws://localhost:8082/ws
   ```

3. Verify API key is correct in configuration

4. Check Edge MCP logs:
   ```bash
   # If running locally
   docker-compose logs edge-mcp

   # If running in Kubernetes
   kubectl logs -n edge-mcp deployment/edge-mcp
   ```

### Authentication Errors

**Problem:** 401 Unauthorized or 403 Forbidden

**Solutions:**
1. Verify API key format (alphanumeric + hyphen/underscore only)
2. Check header format:
   - Bearer token: `Authorization: Bearer <key>`
   - API key: `X-API-Key: <key>`
3. Ensure API key has not expired (if using Core Platform)

### Rate Limiting

**Problem:** 429 Too Many Requests

**Solutions:**
1. Check rate limit headers in error response:
   ```json
   {
     "error": {
       "code": 429,
       "message": "Rate limit exceeded",
       "data": {
         "retry_after": 5.2,
         "limit": "100 requests/sec"
       }
     }
   }
   ```

2. Increase rate limits (for local development):
   ```bash
   export EDGE_MCP_TENANT_RPS=500
   export EDGE_MCP_TOOL_RPS=200
   ```

3. Implement exponential backoff in workflows

### Tool Not Found

**Problem:** Tool execution fails with "tool not found"

**Solutions:**
1. List available tools:
   ```
   /tools list
   ```

2. Search for similar tools:
   ```
   /tools search <keyword>
   ```

3. Check if Core Platform is connected (for dynamic tools):
   ```bash
   curl http://localhost:8082/health/ready
   ```
   Look for `"core_platform": "healthy"` in response

4. Refresh tool registry (if Core Platform was recently connected):
   - Restart Edge MCP or wait for automatic refresh (every 5 minutes)

### Slow Performance

**Problem:** Tool execution is slow

**Solutions:**
1. Enable Redis cache:
   ```bash
   export REDIS_ENABLED=true
   export REDIS_URL=redis://localhost:6379
   ```

2. Check cache hit rate:
   ```bash
   curl http://localhost:8082/metrics | grep cache_hit
   ```

3. Use batch execution for multiple independent operations:
   ```
   claude> Get info for 5 GitHub repos in parallel
   ```

4. Enable distributed tracing to identify bottlenecks:
   ```bash
   export TRACING_ENABLED=true
   export ZIPKIN_ENDPOINT=http://localhost:9411/api/v2/spans
   ```

### Connection Drops

**Problem:** Connection drops after period of inactivity

**Solutions:**
1. Keepalive is enabled by default (30s interval)
2. Check firewall/proxy timeout settings
3. Verify WebSocket connection is not being proxied incorrectly
4. Use `wss://` (WebSocket Secure) for production deployments

## Next Steps

- **Custom Tools:** Learn how to add custom tools to Edge MCP
- **Workflow Templates:** Use workflow templates for common operations
- **Multi-Agent Orchestration:** Leverage agent assignment for complex tasks
- **Production Deployment:** Deploy Edge MCP to Kubernetes with HA

## Related Documentation

- [Generic MCP Client Guide](./generic-mcp-client.md)
- [Troubleshooting Guide](./troubleshooting.md)
- [OpenAPI Specification](../openapi/edge-mcp.yaml)
- [Kubernetes Deployment Guide](../../deployments/k8s/README.md)
