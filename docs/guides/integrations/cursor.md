# Cursor IDE Integration Guide

This guide explains how to integrate Edge MCP with Cursor, the AI-first code editor.

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Quick Start](#quick-start)
4. [Configuration](#configuration)
5. [Usage Examples](#usage-examples)
6. [Advanced Configuration](#advanced-configuration)
7. [Troubleshooting](#troubleshooting)

## Overview

Cursor supports the Model Context Protocol (MCP) for extending its AI capabilities with custom tools. Edge MCP provides:

- **DevOps Integration**: GitHub, Harness, and CI/CD tools
- **Agent Orchestration**: Multi-agent task delegation and workflow management
- **Context Management**: Session-based context tracking
- **Batch Operations**: Execute multiple tools efficiently
- **Semantic Errors**: AI-friendly error messages with recovery guidance

## Prerequisites

### Required

- Cursor IDE (v0.40.0+)
- Edge MCP server running locally or remotely
- Valid API key for Edge MCP authentication

### Optional

- Core Platform connection (for dynamic tool discovery from GitHub, Harness)
- Redis (for distributed caching in multi-instance deployments)

## Quick Start

### 1. Start Edge MCP Server

**Local Development:**
```bash
cd apps/edge-mcp
export EDGE_MCP_API_KEY=dev-admin-key-1234567890
go run cmd/server/main.go
```

**Docker:**
```bash
docker-compose -f docker-compose.local.yml up edge-mcp
```

**Kubernetes:**
```bash
helm install edge-mcp ./deployments/k8s/helm/edge-mcp \
  --set config.auth.apiKey="your-api-key"
```

### 2. Configure MCP in Cursor

Cursor stores MCP configuration in its settings. There are two ways to configure:

#### Option A: Cursor Settings UI

1. Open Cursor Settings (`Cmd+,` on macOS, `Ctrl+,` on Windows/Linux)
2. Search for "MCP"
3. Click "Edit in settings.json"
4. Add Edge MCP configuration:

```json
{
  "mcp.servers": {
    "edge-mcp": {
      "transport": "websocket",
      "url": "ws://localhost:8082/ws",
      "headers": {
        "Authorization": "Bearer dev-admin-key-1234567890"
      },
      "description": "DevOps tools via Edge MCP",
      "enabled": true
    }
  }
}
```

#### Option B: Direct Config File Edit

**Location:**
- macOS: `~/Library/Application Support/Cursor/User/settings.json`
- Linux: `~/.config/Cursor/User/settings.json`
- Windows: `%APPDATA%\Cursor\User\settings.json`

Add the MCP configuration as shown above.

### 3. Reload Cursor

After configuration, reload Cursor:
- Press `Cmd+Shift+P` (macOS) or `Ctrl+Shift+P` (Windows/Linux)
- Type "Reload Window"
- Press Enter

### 4. Verify Connection

1. Open Cursor's AI chat (`Cmd+L` or `Ctrl+L`)
2. Type: "List available MCP tools"
3. Cursor should show tools from Edge MCP

## Configuration

### Basic Configuration

```json
{
  "mcp.servers": {
    "edge-mcp": {
      "transport": "websocket",
      "url": "ws://localhost:8082/ws",
      "headers": {
        "Authorization": "Bearer dev-admin-key-1234567890"
      },
      "description": "Edge MCP - DevOps Integration",
      "enabled": true,
      "autoReconnect": true,
      "reconnectDelay": 5000
    }
  }
}
```

### With Passthrough Authentication

Provide service-specific credentials for GitHub, Harness, etc.:

```json
{
  "mcp.servers": {
    "edge-mcp": {
      "transport": "websocket",
      "url": "ws://localhost:8082/ws",
      "headers": {
        "Authorization": "Bearer dev-admin-key-1234567890",
        "X-GitHub-Token": "ghp_yourPersonalAccessToken",
        "X-Harness-API-Key": "pat.yourHarnessAPIKey",
        "X-Harness-Account-ID": "your-account-id"
      },
      "description": "Edge MCP with GitHub & Harness credentials",
      "enabled": true
    }
  }
}
```

### Environment-Based Configuration

For team setups, use environment variables:

```json
{
  "mcp.servers": {
    "edge-mcp": {
      "transport": "websocket",
      "url": "${env:EDGE_MCP_URL}",
      "headers": {
        "Authorization": "Bearer ${env:EDGE_MCP_API_KEY}",
        "X-GitHub-Token": "${env:GITHUB_TOKEN}",
        "X-Harness-API-Key": "${env:HARNESS_API_KEY}"
      },
      "enabled": true
    }
  }
}
```

Then set environment variables before launching Cursor:
```bash
export EDGE_MCP_URL=ws://localhost:8082/ws
export EDGE_MCP_API_KEY=dev-admin-key-1234567890
export GITHUB_TOKEN=ghp_yourToken
export HARNESS_API_KEY=pat.yourHarnessKey
cursor .
```

### Multiple Edge MCP Instances

Connect to multiple Edge MCP instances (e.g., dev and production):

```json
{
  "mcp.servers": {
    "edge-mcp-dev": {
      "transport": "websocket",
      "url": "ws://localhost:8082/ws",
      "headers": {
        "Authorization": "Bearer dev-api-key"
      },
      "description": "Edge MCP - Development",
      "enabled": true
    },
    "edge-mcp-prod": {
      "transport": "websocket",
      "url": "wss://edge-mcp.company.com/ws",
      "headers": {
        "Authorization": "Bearer prod-api-key"
      },
      "description": "Edge MCP - Production",
      "enabled": true
    }
  }
}
```

## Usage Examples

### Example 1: GitHub Repository Analysis

Open Cursor's AI chat and ask:

```
Analyze the developer-mesh/developer-mesh repository on GitHub.
Show me the recent commits, open issues, and pull requests.
```

Cursor will:
1. Use `github_get_repository` to fetch repo info
2. Use `github_list_commits` for recent commits
3. Use `github_list_issues` for open issues
4. Use `github_list_pull_requests` for PRs
5. Compile a comprehensive analysis

### Example 2: Create GitHub Issue from Current File

With a file open in Cursor:

```
Create a GitHub issue about the bug on line 42 of this file
in the developer-mesh/developer-mesh repository.
Title: "Fix null pointer dereference in handler.go"
```

Cursor will:
1. Extract context from the current file
2. Use `github_create_issue` with relevant details
3. Confirm issue creation with link

### Example 3: Harness Pipeline Status

```
Show me the status of all Harness pipelines
in the "developer-mesh" project
```

Cursor will:
1. Use `harness_pipelines_list` to fetch pipelines
2. Use `harness_executions_list` for recent executions
3. Present a formatted status report

### Example 4: Multi-Step Workflow

```
1. List all open issues in developer-mesh/developer-mesh with label "bug"
2. For each issue, create a task and assign it to an agent
3. Show me the task assignment summary
```

Cursor will:
1. Use `github_list_issues` with label filter
2. Use `devmesh_task_create` for each issue
3. Use `devmesh_agent_assign` to assign tasks
4. Compile and present the summary

### Example 5: Context-Aware Operations

```
Remember: I'm working on the developer-mesh/developer-mesh repository
```

Later in the conversation:
```
List all PRs authored by me
```

Cursor will use the stored context to automatically fill in the repository details.

## Advanced Configuration

### IDE Detection

Edge MCP automatically detects Cursor IDE via:
- `User-Agent` header containing "Cursor"
- `X-IDE-Name: Cursor` header

This enables IDE-specific optimizations:
- Enhanced code context awareness
- File operation batching
- Workspace-aware tool suggestions

### Connection Timeouts

Configure timeouts for stability:

```json
{
  "mcp.servers": {
    "edge-mcp": {
      "transport": "websocket",
      "url": "ws://localhost:8082/ws",
      "headers": {
        "Authorization": "Bearer dev-admin-key-1234567890"
      },
      "connectionTimeout": 30000,    // 30s connection timeout
      "requestTimeout": 120000,       // 2min request timeout
      "pingInterval": 30000,          // 30s ping interval
      "autoReconnect": true,
      "reconnectDelay": 5000,         // 5s reconnect delay
      "maxReconnectAttempts": 10
    }
  }
}
```

### Workspace Integration

For workspace-specific configuration, create `.cursor/settings.json` in your project:

```json
{
  "mcp.servers": {
    "edge-mcp": {
      "transport": "websocket",
      "url": "ws://localhost:8082/ws",
      "headers": {
        "Authorization": "Bearer ${workspaceFolder}/.edge-mcp-key",
        "X-Workspace": "${workspaceFolderBasename}"
      }
    }
  }
}
```

This allows per-project API keys and workspace identification.

### Custom Tool Categories

Filter tools by category in Cursor:

```
Show me all GitHub repository tools
```

Cursor will filter tools with category "repository" from Edge MCP.

Available categories:
- `repository` - Repository operations
- `issues` - Issue management
- `pull_requests` - PR operations
- `ci_cd` - CI/CD and pipelines
- `workflow` - Workflow orchestration
- `agent` - Agent management
- And many more...

## Troubleshooting

### Connection Issues

**Problem:** Cursor cannot connect to Edge MCP

**Diagnosis:**
1. Check Cursor's Output panel (View → Output → MCP)
2. Look for connection errors

**Solutions:**
1. Verify Edge MCP is running:
   ```bash
   curl http://localhost:8082/health/ready
   ```

2. Test WebSocket connectivity:
   ```bash
   websocat ws://localhost:8082/ws
   ```

3. Check API key format and validity

4. Verify URL format (must be `ws://` or `wss://`)

5. Check firewall/antivirus settings (may block WebSocket)

### Authentication Errors

**Problem:** 401 Unauthorized or 403 Forbidden

**Solutions:**
1. Verify API key is correctly formatted:
   - Pattern: `^[a-zA-Z0-9_-]+$`
   - Example: `dev-admin-key-1234567890`

2. Check header format:
   ```json
   "headers": {
     "Authorization": "Bearer your-api-key"
   }
   ```
   NOT: `"Authorization": "your-api-key"` (missing "Bearer")

3. Ensure API key has not expired (check Edge MCP logs)

4. Verify API key is set in Edge MCP environment:
   ```bash
   echo $EDGE_MCP_API_KEY
   ```

### Tools Not Appearing

**Problem:** No tools visible in Cursor

**Solutions:**
1. Reload Cursor window (`Cmd+Shift+P` → "Reload Window")

2. Check MCP server status in Cursor:
   - Open Output panel (View → Output)
   - Select "MCP" from dropdown
   - Look for "Connected to edge-mcp" message

3. Verify tools are loaded in Edge MCP:
   ```bash
   curl -H "Authorization: Bearer dev-admin-key-1234567890" \
        http://localhost:8082/ws  # Connection test
   ```

4. Check Edge MCP logs for tool loading errors:
   ```bash
   docker-compose logs edge-mcp | grep "tool"
   ```

### Slow Tool Execution

**Problem:** Tools take a long time to execute

**Solutions:**
1. Enable caching:
   ```bash
   export REDIS_ENABLED=true
   export REDIS_URL=redis://localhost:6379
   ```

2. Use batch operations for multiple independent calls:
   ```
   Get info for repositories A, B, and C in parallel
   ```
   Edge MCP will execute these in parallel via `tools/batch`.

3. Check network latency to Core Platform (if using dynamic tools)

4. Review Edge MCP metrics:
   ```bash
   curl http://localhost:8082/metrics | grep tool_execution_duration
   ```

### Rate Limiting

**Problem:** Getting 429 Too Many Requests errors

**Solutions:**
1. Check rate limit in error response:
   ```json
   {
     "error": {
       "code": 429,
       "message": "Rate limit exceeded",
       "data": {
         "retry_after": 5.2,
         "limit": "100 requests/sec",
         "quota_remaining": 0
       }
     }
   }
   ```

2. Increase limits for local development:
   ```bash
   export EDGE_MCP_TENANT_RPS=500
   export EDGE_MCP_TOOL_RPS=200
   ```

3. Implement batch operations to reduce request count

4. Space out requests in Cursor (avoid rapid-fire queries)

### Passthrough Auth Not Working

**Problem:** GitHub/Harness tools fail with authentication errors

**Solutions:**
1. Verify passthrough credentials are set:
   ```json
   "headers": {
     "Authorization": "Bearer dev-admin-key-1234567890",
     "X-GitHub-Token": "ghp_...",
     "X-Harness-API-Key": "pat..."
   }
   ```

2. Test credentials directly:
   ```bash
   # Test GitHub token
   curl -H "Authorization: token ghp_..." https://api.github.com/user

   # Test Harness API key
   curl -H "x-api-key: pat..." https://app.harness.io/gateway/ng/api/version
   ```

3. Check Edge MCP logs for passthrough auth extraction:
   ```bash
   docker-compose logs edge-mcp | grep "passthrough"
   ```

4. Ensure tokens have correct scopes:
   - GitHub: `repo`, `read:org`, `read:user`
   - Harness: Account-level API key with appropriate permissions

### WebSocket Connection Drops

**Problem:** Connection drops periodically

**Solutions:**
1. Verify keepalive is working (Edge MCP sends pings every 30s)

2. Check proxy/firewall timeout settings

3. Increase timeouts in Cursor config:
   ```json
   "pingInterval": 30000,
   "requestTimeout": 180000
   ```

4. Use `wss://` (secure WebSocket) for production

5. Check Edge MCP logs for disconnection reasons:
   ```bash
   docker-compose logs edge-mcp | grep -i "disconnect\|close"
   ```

## Next Steps

- **Explore Tool Categories**: Use tool categories to discover relevant tools
- **Create Workflows**: Build multi-step workflows combining tools
- **Agent Delegation**: Use agent assignment for complex tasks
- **Production Deployment**: Deploy Edge MCP with HA and monitoring

## Related Documentation

- [Claude Code Integration](./claude-code.md)
- [Generic MCP Client Guide](./generic-mcp-client.md)
- [Troubleshooting Guide](./troubleshooting.md)
- [OpenAPI Specification](../openapi/edge-mcp.yaml)
- [Kubernetes Deployment](../../deployments/k8s/README.md)
