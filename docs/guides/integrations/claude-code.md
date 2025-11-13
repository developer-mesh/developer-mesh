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

### 1. Install Edge MCP Binary

**Install via Go:**
```bash
cd apps/edge-mcp
go build -o ~/.local/bin/edge-mcp ./cmd/server
```

**Or download pre-built binary:**
```bash
# Download from releases (replace VERSION)
curl -L -o ~/.local/bin/edge-mcp \
  https://github.com/developer-mesh/developer-mesh/releases/download/VERSION/edge-mcp
chmod +x ~/.local/bin/edge-mcp
```

### 2. Configure Edge MCP for Claude Code (stdio mode)

Claude Code communicates with Edge MCP using stdio mode. Configure it using `~/.claude/mcp_servers.json`:

**Location:** `~/.claude/mcp_servers.json` (Linux/macOS) or `%APPDATA%\.claude\mcp_servers.json` (Windows)

```json
{
  "mcpServers": {
    "devmesh": {
      "command": "edge-mcp",
      "args": ["--stdio"],
      "description": "Developer Mesh - DevOps Tool Integration"
    }
  }
}
```

**⚠️ Important:** Due to a Claude Code bug ([Issue #1254](https://github.com/anthropics/claude-code/issues/1254)), environment variables in `mcp_servers.json` are not passed to MCP servers. Use the config file workaround below instead.

### 3. Create Edge MCP Configuration File

Since environment variables don't work in stdio mode, create a configuration file with your credentials:

**Location:** `~/.edge-mcp.yaml`

```yaml
# Edge MCP Configuration for Claude Code
core:
  url: "http://localhost:8081"      # REST API URL
  api_key: "your-api-key-here"      # Your DevMesh API key

auth:
  api_key: "your-api-key-here"      # Same as core.api_key

server:
  port: 8082
```

**Configuration priority:**
1. Environment variables (highest) - Used by Docker/K8s deployments
2. `~/.edge-mcp.yaml` - Local development (Claude Code workaround)
3. `configs/config.yaml` - Base configuration
4. Defaults (lowest)

### 4. Start Required Services

Edge MCP needs the REST API running to fetch dynamic tools:

**Using Docker Compose (Recommended for local development):**
```bash
docker-compose -f docker-compose.local.yml up rest-api database redis
```

**Or start services individually:**
```bash
# Start PostgreSQL
docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=devmesh postgres:14

# Start Redis
docker run -d -p 6379:6379 redis:7

# Start REST API
cd apps/rest-api
go run cmd/server/main.go
```

### 5. Verify Connection (Alternative: WebSocket Mode)

If you prefer WebSocket mode for remote deployments, you can also configure Claude Code to connect via WebSocket:

**WebSocket Configuration** (`~/.claude/mcp.json`):

```json
{
  "mcpServers": {
    "edge-mcp": {
      "transport": "websocket",
      "url": "ws://localhost:8082/ws",
      "headers": {
        "Authorization": "Bearer your-api-key-here"
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

For WebSocket mode, start Edge MCP in server mode:
```bash
edge-mcp --port 8082
```

### 6. Verify Connection

Start Claude Code and verify the connection:

```bash
claude
```

In Claude Code, use the `/mcp` command to verify the connection:
```
/mcp
```

You should see:
- `Connected to devmesh` - Indicates successful connection
- 169+ tools available (27 built-in tools + 140+ dynamic tools from REST API)

List available tools:
```
/tools
```

You should see tools like:
- `mcp__devmesh__github_*` (140+ GitHub operations)
- `mcp__devmesh__harness_*` (Harness Platform operations)
- `devmesh_agent_assign`, `devmesh_task_create`, etc. (Built-in orchestration)
- And many more...

## Configuration

### Configuration File (`~/.edge-mcp.yaml`)

**Primary method for Claude Code stdio mode.** Create this file with your settings:

```yaml
# Core Platform connection (required for dynamic tools)
core:
  url: "http://localhost:8081"
  api_key: "your-api-key-here"
  edge_mcp_id: "edge-local-dev"  # Optional: instance identifier

# Authentication (should match core.api_key)
auth:
  api_key: "your-api-key-here"

# Server configuration
server:
  port: 8082

# Rate limiting (optional - defaults shown)
rate_limit:
  global_rps: 1000
  global_burst: 2000
  tenant_rps: 100
  tenant_burst: 200
  tool_rps: 50
  tool_burst: 100

# Auto-updater (optional)
updater:
  enabled: true
  channel: "stable"
  auto_download: true
  auto_apply: false
```

**Config file search locations** (checked in order):
1. `./edge-mcp.yaml` - Current directory
2. `~/.edge-mcp.yaml` - Home directory (recommended)
3. `~/.config/edge-mcp/config.yaml` - XDG config
4. `/etc/edge-mcp/config.yaml` - System-wide

### Environment Variables

**For Docker/Kubernetes deployments only.** Environment variables override config file values:

```bash
# Core Platform integration (required for dynamic tools)
export DEV_MESH_URL=http://localhost:8081    # Core Platform URL
export DEV_MESH_API_KEY=your-core-api-key    # Core Platform API key
export EDGE_MCP_ID=edge-mcp-01              # Unique Edge MCP instance ID

# Server configuration
export EDGE_MCP_PORT=8082                    # Server port (default: 8082)

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

**⚠️ Note:** Environment variables in `mcp_servers.json` are **not passed** to edge-mcp due to Claude Code bug [#1254](https://github.com/anthropics/claude-code/issues/1254). Use the config file instead for Claude Code stdio mode.

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

### Only Seeing 27 Tools (Missing Dynamic Tools)

**Problem:** Claude Code only shows 27 built-in tools instead of 169+ tools

**Root Cause:** Edge MCP is not connecting to REST API to fetch dynamic tools (GitHub, Harness, etc.)

**Solutions:**

1. **Verify config file exists and has credentials:**
   ```bash
   cat ~/.edge-mcp.yaml
   ```
   Should show your `core.url` and `auth.api_key`

2. **Check edge-mcp debug output:**

   Restart Claude Code's MCP connection (`/mcp` command) and check the debug output.

   You should see:
   ```
   [edge-mcp] Loaded base config from: configs/config.yaml
   [edge-mcp] Merging user config from: /Users/you/.edge-mcp.yaml
   [edge-mcp] Resolved configuration:
   [edge-mcp]   Core URL: http://localhost:8081
   [edge-mcp]   Auth API Key: adm_...KaQ
   ```

   If you see empty URL or API key, the config file is not being loaded.

3. **Verify REST API is running and healthy:**
   ```bash
   curl http://localhost:8081/health
   ```
   Should return: `{"status":"healthy",...}`

4. **Check REST API has tools:**
   ```bash
   curl -H "Authorization: Bearer your-api-key" \
     http://localhost:8081/api/v1/tools | jq '.count'
   ```
   Should show 169 or more tools

5. **Common config file issues:**
   - **Wrong location**: Must be `~/.edge-mcp.yaml` (not `~/.edge-mcp.yml`)
   - **YAML syntax error**: Validate with `yamllint ~/.edge-mcp.yaml`
   - **Wrong API key format**: Must match the format `adm_...` or configured format
   - **Wrong URL**: Must include http:// or https:// protocol

### Connection Issues (stdio mode)

**Problem:** Claude Code shows "Failed to connect to devmesh"

**Solutions:**

1. **Verify edge-mcp binary is in PATH:**
   ```bash
   which edge-mcp
   ```
   Should show: `/Users/you/.local/bin/edge-mcp` or similar

2. **Check mcp_servers.json configuration:**
   ```bash
   cat ~/.claude/mcp_servers.json
   ```
   Should have:
   ```json
   {
     "mcpServers": {
       "devmesh": {
         "command": "edge-mcp",
         "args": ["--stdio"]
       }
     }
   }
   ```

3. **Test edge-mcp manually:**
   ```bash
   edge-mcp --stdio
   ```
   Should start and wait for stdin input (Ctrl+C to exit)

4. **Check Claude Code MCP logs:**
   Look for error messages in Claude Code's developer console

### Connection Issues (WebSocket mode)

**Problem:** Claude Code cannot connect to Edge MCP via WebSocket

**Solutions:**

1. **Verify Edge MCP is running in WebSocket mode:**
   ```bash
   edge-mcp --port 8082
   ```

2. **Check WebSocket endpoint:**
   ```bash
   curl http://localhost:8082/health/ready
   ```
   Should return: `{"status":"healthy",...}`

3. **Test WebSocket connectivity:**
   ```bash
   websocat ws://localhost:8082/ws
   ```

4. **Verify API key in headers:**
   Check `~/.claude/mcp.json` has correct Bearer token

5. **Check Edge MCP logs:**
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
