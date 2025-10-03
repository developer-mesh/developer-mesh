# Windsurf IDE Integration Guide

This guide explains how to integrate Edge MCP with Windsurf, the AI-powered code editor by Codeium.

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Quick Start](#quick-start)
4. [Configuration](#configuration)
5. [Usage Examples](#usage-examples)
6. [Advanced Configuration](#advanced-configuration)
7. [Troubleshooting](#troubleshooting)

## Overview

Windsurf supports MCP (Model Context Protocol) for extending AI capabilities with external tools and services. Edge MCP provides:

- **200+ DevOps Tools**: Seamless GitHub, Harness, and workflow integration
- **Tool Discovery**: AI-friendly categorization and search
- **Batch Execution**: Parallel tool execution for performance
- **Intelligent Errors**: Semantic error messages with recovery steps
- **Session Context**: Persistent context across interactions

## Prerequisites

### Required

- Windsurf IDE (latest version recommended)
- Edge MCP server (local or remote)
- API key for Edge MCP authentication

### Optional

- Core Platform integration (for dynamic GitHub/Harness tools)
- Redis (for distributed caching in production deployments)
- OpenTelemetry collector (for distributed tracing)

## Quick Start

### 1. Launch Edge MCP Server

**Local Development:**
```bash
cd apps/edge-mcp
export EDGE_MCP_API_KEY=dev-admin-key-1234567890
go run cmd/server/main.go
```

Server starts on `ws://localhost:8082/ws` by default.

**Using Docker:**
```bash
docker-compose -f docker-compose.local.yml up -d edge-mcp
```

**Kubernetes Deployment:**
```bash
# Port forward to local machine
kubectl port-forward -n edge-mcp svc/edge-mcp 8082:8082
```

### 2. Configure MCP in Windsurf

Windsurf uses JSON configuration files for MCP servers.

**Configuration File Location:**
- macOS: `~/Library/Application Support/Windsurf/User/mcp.json`
- Linux: `~/.config/Windsurf/User/mcp.json`
- Windows: `%APPDATA%\Windsurf\User\mcp.json`

**Create or update `mcp.json`:**

```json
{
  "mcpServers": {
    "edge-mcp": {
      "transport": "websocket",
      "endpoint": "ws://localhost:8082/ws",
      "authentication": {
        "type": "bearer",
        "token": "dev-admin-key-1234567890"
      },
      "metadata": {
        "name": "Edge MCP",
        "description": "DevOps tools and agent orchestration",
        "version": "1.0.0"
      },
      "capabilities": {
        "tools": true,
        "resources": true,
        "prompts": false
      },
      "enabled": true
    }
  }
}
```

### 3. Restart Windsurf

After configuration:
1. Save the `mcp.json` file
2. Restart Windsurf completely (not just reload window)
3. Windsurf will automatically connect to Edge MCP on startup

### 4. Verify Connection

1. Open Windsurf's AI chat panel
2. Type: `@edge-mcp list tools`
3. Windsurf should display available Edge MCP tools

Or check the status in Windsurf:
- Open Command Palette (`Cmd+Shift+P` or `Ctrl+Shift+P`)
- Type "MCP: Show Status"
- Verify "edge-mcp" is connected

## Configuration

### Basic Configuration

Minimal configuration for local development:

```json
{
  "mcpServers": {
    "edge-mcp": {
      "transport": "websocket",
      "endpoint": "ws://localhost:8082/ws",
      "authentication": {
        "type": "bearer",
        "token": "dev-admin-key-1234567890"
      },
      "enabled": true
    }
  }
}
```

### With API Key Header Authentication

Alternative authentication method using API key header:

```json
{
  "mcpServers": {
    "edge-mcp": {
      "transport": "websocket",
      "endpoint": "ws://localhost:8082/ws",
      "authentication": {
        "type": "header",
        "headers": {
          "X-API-Key": "dev-admin-key-1234567890"
        }
      },
      "enabled": true
    }
  }
}
```

### With Passthrough Authentication

Configure service-specific credentials for GitHub, Harness, etc.:

```json
{
  "mcpServers": {
    "edge-mcp": {
      "transport": "websocket",
      "endpoint": "ws://localhost:8082/ws",
      "authentication": {
        "type": "bearer",
        "token": "dev-admin-key-1234567890"
      },
      "additionalHeaders": {
        "X-GitHub-Token": "ghp_yourGitHubPersonalAccessToken",
        "X-Harness-API-Key": "pat.yourHarnessAPIKey",
        "X-Harness-Account-ID": "your-harness-account-id"
      },
      "enabled": true
    }
  }
}
```

Edge MCP will use these credentials when calling GitHub/Harness APIs.

### Production Configuration

For production deployment with TLS/SSL:

```json
{
  "mcpServers": {
    "edge-mcp-prod": {
      "transport": "websocket",
      "endpoint": "wss://edge-mcp.company.com/ws",
      "authentication": {
        "type": "bearer",
        "token": "${env:EDGE_MCP_API_KEY}"
      },
      "metadata": {
        "name": "Edge MCP - Production",
        "description": "Production Edge MCP instance",
        "version": "1.0.0"
      },
      "connectionOptions": {
        "tlsVerify": true,
        "timeout": 30000,
        "reconnect": true,
        "reconnectDelay": 5000,
        "maxReconnectAttempts": 10
      },
      "enabled": true
    }
  }
}
```

**Note:** `${env:VARIABLE_NAME}` syntax references environment variables.

### Multiple Environment Setup

Connect to different Edge MCP instances for different environments:

```json
{
  "mcpServers": {
    "edge-mcp-local": {
      "transport": "websocket",
      "endpoint": "ws://localhost:8082/ws",
      "authentication": {
        "type": "bearer",
        "token": "dev-admin-key-1234567890"
      },
      "metadata": {
        "name": "Edge MCP - Local",
        "description": "Local development instance"
      },
      "enabled": true
    },
    "edge-mcp-staging": {
      "transport": "websocket",
      "endpoint": "wss://edge-mcp-staging.company.com/ws",
      "authentication": {
        "type": "bearer",
        "token": "${env:EDGE_MCP_STAGING_KEY}"
      },
      "metadata": {
        "name": "Edge MCP - Staging",
        "description": "Staging environment"
      },
      "enabled": false
    },
    "edge-mcp-prod": {
      "transport": "websocket",
      "endpoint": "wss://edge-mcp.company.com/ws",
      "authentication": {
        "type": "bearer",
        "token": "${env:EDGE_MCP_PROD_KEY}"
      },
      "metadata": {
        "name": "Edge MCP - Production",
        "description": "Production environment"
      },
      "enabled": false
    }
  }
}
```

Enable/disable environments as needed by changing the `enabled` flag.

## Usage Examples

### Example 1: GitHub Repository Information

In Windsurf AI chat:

```
@edge-mcp Get detailed information about the developer-mesh/developer-mesh repository on GitHub
```

Windsurf will:
1. Call `github_get_repository` tool
2. Format and display repository details
3. Show stars, forks, issues, last updated, etc.

### Example 2: List and Filter Issues

```
@edge-mcp Show me all open bugs in developer-mesh/developer-mesh repository
```

Windsurf will:
1. Use `github_list_issues` with filters
2. Filter by label "bug" and state "open"
3. Display a formatted list of issues

### Example 3: Create GitHub Issue

```
@edge-mcp Create a new issue in developer-mesh/developer-mesh titled "Add rate limiting to webhook handler" with description "Implement per-tenant rate limiting for webhook processing to prevent abuse"
```

Windsurf will:
1. Execute `github_create_issue`
2. Return the created issue URL and number
3. Confirm creation success

### Example 4: Batch Operations

```
@edge-mcp Get repository info, list open PRs, and list recent commits for developer-mesh/developer-mesh
```

Windsurf will:
1. Execute tools in parallel via `tools/batch`
2. Combine results intelligently
3. Present unified view of all data

### Example 5: Harness Pipeline Status

```
@edge-mcp Show me the latest execution status of all pipelines in the "developer-mesh" Harness project
```

Windsurf will:
1. Use `harness_pipelines_list` to fetch pipelines
2. Use `harness_executions_list` for recent executions
3. Format status report with success/failure counts

### Example 6: Agent Task Assignment

```
@edge-mcp Create a code review task and assign it to an agent with high priority
```

Windsurf will:
1. Use `devmesh_task_create` to create task
2. Use `devmesh_agent_assign` to assign to available agent
3. Return task ID and assignment confirmation

### Example 7: Context Management

```
@edge-mcp Remember that I'm working on the developer-mesh/developer-mesh repository
```

Later:
```
@edge-mcp List my recent commits
```

Windsurf will use stored context to infer repository details automatically.

## Advanced Configuration

### Connection Options

Fine-tune connection behavior:

```json
{
  "mcpServers": {
    "edge-mcp": {
      "transport": "websocket",
      "endpoint": "ws://localhost:8082/ws",
      "authentication": {
        "type": "bearer",
        "token": "dev-admin-key-1234567890"
      },
      "connectionOptions": {
        "timeout": 30000,               // Connection timeout (30s)
        "reconnect": true,               // Auto-reconnect on disconnect
        "reconnectDelay": 5000,          // Wait 5s before reconnect
        "maxReconnectAttempts": 10,      // Max 10 reconnect attempts
        "pingInterval": 30000,           // Send ping every 30s
        "pongTimeout": 10000,            // Expect pong within 10s
        "requestTimeout": 120000         // Request timeout (2min)
      },
      "enabled": true
    }
  }
}
```

### Tool Filtering

Filter tools by category or tags:

```json
{
  "mcpServers": {
    "edge-mcp": {
      "transport": "websocket",
      "endpoint": "ws://localhost:8082/ws",
      "authentication": {
        "type": "bearer",
        "token": "dev-admin-key-1234567890"
      },
      "toolFilters": {
        "includeCategories": ["repository", "issues", "ci_cd"],
        "excludeCategories": ["admin"],
        "includeTags": ["read", "write"],
        "excludeTags": ["delete"]
      },
      "enabled": true
    }
  }
}
```

This configuration:
- Only shows repository, issues, and CI/CD tools
- Hides admin tools
- Shows read and write operations
- Hides delete operations

### Workspace-Specific Configuration

Create `.windsurf/mcp.json` in your workspace root for project-specific settings:

```json
{
  "mcpServers": {
    "edge-mcp": {
      "transport": "websocket",
      "endpoint": "ws://localhost:8082/ws",
      "authentication": {
        "type": "bearer",
        "token": "${file:.edge-mcp-key}"
      },
      "additionalHeaders": {
        "X-Workspace": "developer-mesh",
        "X-Project-ID": "proj-12345"
      },
      "enabled": true
    }
  }
}
```

**Note:** `${file:path}` syntax reads token from file.

### Logging and Debugging

Enable detailed logging for troubleshooting:

```json
{
  "mcpServers": {
    "edge-mcp": {
      "transport": "websocket",
      "endpoint": "ws://localhost:8082/ws",
      "authentication": {
        "type": "bearer",
        "token": "dev-admin-key-1234567890"
      },
      "debug": {
        "enabled": true,
        "logLevel": "debug",          // debug, info, warn, error
        "logMessages": true,           // Log all MCP messages
        "logHeaders": false            // Don't log headers (contains auth)
      },
      "enabled": true
    }
  }
}
```

View logs in Windsurf:
- Open Output panel (View → Output)
- Select "MCP Client" from dropdown
- Look for edge-mcp logs

## Troubleshooting

### Connection Fails

**Problem:** Windsurf cannot connect to Edge MCP

**Diagnosis:**
1. Check Windsurf output logs (View → Output → MCP Client)
2. Look for connection error messages

**Solutions:**
1. Verify Edge MCP is running:
   ```bash
   curl http://localhost:8082/health/ready
   ```
   Expected: `{"status":"healthy",...}`

2. Test WebSocket endpoint:
   ```bash
   websocat ws://localhost:8082/ws
   ```
   Should connect without errors.

3. Verify `mcp.json` syntax:
   ```bash
   # Validate JSON
   cat ~/Library/Application\ Support/Windsurf/User/mcp.json | jq .
   ```

4. Check firewall/antivirus settings (may block WebSocket)

5. Verify API key format:
   - Must match `^[a-zA-Z0-9_-]+$`
   - Example: `dev-admin-key-1234567890`

### Authentication Errors

**Problem:** 401 Unauthorized or 403 Forbidden

**Solutions:**
1. Verify authentication configuration:
   ```json
   "authentication": {
     "type": "bearer",
     "token": "dev-admin-key-1234567890"
   }
   ```

2. Check Edge MCP API key environment variable:
   ```bash
   echo $EDGE_MCP_API_KEY
   ```

3. Verify API key has not expired (check Edge MCP logs)

4. Test authentication manually:
   ```bash
   curl -H "Authorization: Bearer dev-admin-key-1234567890" \
        http://localhost:8082/health/ready
   ```

### No Tools Visible

**Problem:** Windsurf shows no tools from Edge MCP

**Solutions:**
1. Check MCP server status:
   - Command Palette → "MCP: Show Status"
   - Verify "edge-mcp" is "Connected"

2. Restart Windsurf completely (full application restart)

3. Verify Edge MCP has tools loaded:
   ```bash
   # Connect with websocat and send tools/list
   echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test","version":"1.0.0"}}}' | websocat ws://localhost:8082/ws
   ```

4. Check Edge MCP logs for tool loading errors:
   ```bash
   docker-compose logs edge-mcp | grep -i "tool\|error"
   ```

5. Verify tool filters (if configured) aren't excluding all tools

### Slow Performance

**Problem:** Tool execution is slow in Windsurf

**Solutions:**
1. Enable Redis caching:
   ```bash
   export REDIS_ENABLED=true
   export REDIS_URL=redis://localhost:6379
   ```

2. Use batch operations instead of sequential calls:
   ```
   @edge-mcp Get info for repos A, B, and C
   ```
   (Edge MCP will batch these automatically)

3. Check network latency (especially for remote Edge MCP):
   ```bash
   time curl http://edge-mcp.company.com/health/live
   ```

4. Review Edge MCP performance metrics:
   ```bash
   curl http://localhost:8082/metrics | grep tool_execution_duration
   ```

5. Enable distributed tracing to identify bottlenecks:
   ```bash
   export TRACING_ENABLED=true
   export ZIPKIN_ENDPOINT=http://localhost:9411/api/v2/spans
   ```

### Rate Limiting Issues

**Problem:** Getting 429 Too Many Requests errors

**Solutions:**
1. Check rate limit details in error message:
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
   export EDGE_MCP_GLOBAL_RPS=2000
   ```

3. Implement request batching to reduce total requests

4. Space out requests (avoid rapid sequential calls)

### Passthrough Auth Fails

**Problem:** GitHub/Harness tools fail with auth errors

**Solutions:**
1. Verify passthrough headers are configured:
   ```json
   "additionalHeaders": {
     "X-GitHub-Token": "ghp_...",
     "X-Harness-API-Key": "pat..."
   }
   ```

2. Test credentials directly:
   ```bash
   # GitHub
   curl -H "Authorization: token ghp_..." \
        https://api.github.com/user

   # Harness
   curl -H "x-api-key: pat..." \
        https://app.harness.io/gateway/ng/api/version
   ```

3. Check token scopes:
   - **GitHub:** `repo`, `read:org`, `read:user`
   - **Harness:** Account-level API key with appropriate permissions

4. Review Edge MCP logs for passthrough auth processing:
   ```bash
   docker-compose logs edge-mcp | grep "passthrough"
   ```

### Connection Drops Frequently

**Problem:** WebSocket connection drops and reconnects often

**Solutions:**
1. Increase timeout values:
   ```json
   "connectionOptions": {
     "timeout": 60000,
     "pingInterval": 20000,
     "pongTimeout": 15000
   }
   ```

2. Check proxy/firewall timeout settings

3. Use secure WebSocket (`wss://`) for production

4. Verify network stability (ping Edge MCP server)

5. Check Edge MCP logs for disconnect reasons:
   ```bash
   docker-compose logs edge-mcp | grep -i "disconnect\|websocket.*close"
   ```

## Next Steps

- **Explore Tool Categories**: Discover tools by category (repository, issues, ci_cd, etc.)
- **Build Workflows**: Combine tools into multi-step workflows
- **Agent Orchestration**: Use agent assignment for complex tasks
- **Production Deployment**: Deploy Edge MCP with Kubernetes, monitoring, and HA

## Related Documentation

- [Claude Code Integration](./claude-code.md)
- [Cursor Integration](./cursor.md)
- [Generic MCP Client Guide](./generic-mcp-client.md)
- [Troubleshooting Guide](./troubleshooting.md)
- [OpenAPI Specification](../openapi/edge-mcp.yaml)
- [Kubernetes Deployment](../../deployments/k8s/README.md)
