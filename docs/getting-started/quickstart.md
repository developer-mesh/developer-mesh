# Edge MCP Quick Start Guide

Get up and running with Edge MCP in 5 minutes. This guide covers the fastest path from zero to executing your first DevOps automation with AI agents.

## 🚀 5-Minute Setup

### Prerequisites

- Docker and Docker Compose installed
- 2GB available RAM
- Ports 8085, 8081, 5432, 6379 available

### Step 1: Clone and Start (2 minutes)

```bash
# Clone repository (if you haven't already)
git clone https://github.com/developer-mesh/developer-mesh.git
cd developer-mesh

# Start all services with Docker Compose
docker-compose -f docker-compose.local.yml up -d

# Verify services are healthy (wait ~30 seconds for initialization)
curl http://localhost:8085/health/ready
```

**Expected output:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime_seconds": 15,
  "components": {
    "tool_registry": "healthy",
    "cache": "healthy",
    "mcp_handler": "healthy"
  }
}
```

### Step 2: Verify Installation (1 minute)

```bash
# Test Edge MCP WebSocket endpoint
curl -H "Authorization: Bearer dev-admin-key-1234567890" \
  http://localhost:8085/health

# Test Core Platform REST API
curl -H "X-API-Key: dev-admin-key-1234567890" \
  http://localhost:8081/health

# Check logs (optional)
docker-compose -f docker-compose.local.yml logs -f edge-mcp
```

### Step 3: Connect Your AI Client (2 minutes)

Choose your preferred AI coding assistant:

#### Option A: Claude Code

Create `~/.claude/mcp.json`:

```json
{
  "mcpServers": {
    "edge-mcp": {
      "transport": "websocket",
      "url": "ws://localhost:8085/ws",
      "headers": {
        "Authorization": "Bearer dev-admin-key-1234567890"
      },
      "description": "Edge MCP - DevOps Automation",
      "supportsStreaming": true
    }
  }
}
```

Verify connection:
```bash
claude
# Then type: /tools list
```

#### Option B: Cursor IDE

Add to Cursor settings (`Settings > Features > Claude > MCP Servers`):

```json
{
  "edge-mcp": {
    "command": "ws://localhost:8085/ws",
    "args": [],
    "env": {
      "EDGE_MCP_AUTH": "Bearer dev-admin-key-1234567890"
    }
  }
}
```

#### Option C: Generic MCP Client

```bash
# Install websocat for testing
brew install websocat  # macOS
# or: apt-get install websocat  # Linux

# Connect to Edge MCP
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test","version":"1.0.0"}}}' | \
  websocat --header="Authorization: Bearer dev-admin-key-1234567890" \
  ws://localhost:8085/ws
```

---

## 🎯 Common Use Cases

### Use Case 1: GitHub Repository Analysis

**Scenario:** Get repository information and list recent issues

**With Claude Code:**
```
You: Analyze the developer-mesh/developer-mesh repository on GitHub
     and show me the 5 most recent open issues
```

**Direct API Call:**
```bash
# Using websocat to call tools directly
echo '{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "github_get_repository",
    "arguments": {
      "owner": "developer-mesh",
      "repo": "developer-mesh"
    }
  }
}' | websocat --header="Authorization: Bearer dev-admin-key-1234567890" \
             --header="X-GitHub-Token: YOUR_GITHUB_TOKEN" \
             ws://localhost:8085/ws
```

**What Edge MCP Does:**
1. Authenticates your request
2. Routes to GitHub API via passthrough authentication
3. Returns structured repository data
4. AI agent analyzes and presents insights

### Use Case 2: Batch DevOps Operations

**Scenario:** Check multiple services in parallel

**With AI Agent:**
```
You: For the developer-mesh organization, get repository info,
     list open pull requests, and check CI/CD workflow status
```

**Direct API Call (Batch):**
```bash
echo '{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/batch",
  "params": {
    "tools": [
      {
        "id": "repo-1",
        "name": "github_get_repository",
        "arguments": {"owner": "developer-mesh", "repo": "developer-mesh"}
      },
      {
        "id": "prs-1",
        "name": "github_list_pull_requests",
        "arguments": {"owner": "developer-mesh", "repo": "developer-mesh", "state": "open"}
      },
      {
        "id": "workflows-1",
        "name": "github_list_workflows",
        "arguments": {"owner": "developer-mesh", "repo": "developer-mesh"}
      }
    ],
    "parallel": true
  }
}' | websocat --header="Authorization: Bearer dev-admin-key-1234567890" \
             --header="X-GitHub-Token: YOUR_GITHUB_TOKEN" \
             ws://localhost:8085/ws
```

**Benefits:**
- All three operations execute in parallel (faster)
- Single WebSocket request
- Partial results returned even if one operation fails

### Use Case 3: AI Agent Task Orchestration

**Scenario:** Delegate complex tasks to specialized AI agents

**With Claude Code:**
```
You: Assign a code review task for PR #123 to the code-review agent
```

**Direct API Call:**
```bash
echo '{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "devmesh_task_create",
    "arguments": {
      "title": "Review PR #123",
      "type": "code_review",
      "priority": "high",
      "agent_type": "code-review"
    }
  }
}' | websocat --header="Authorization: Bearer dev-admin-key-1234567890" \
             ws://localhost:8085/ws
```

**What Happens:**
1. Task created in DevMesh
2. Assignment engine selects best code-review agent
3. Agent receives task notification
4. Results tracked in workflow context

### Use Case 4: Harness Pipeline Monitoring

**Scenario:** Check deployment pipeline status

**With AI Agent:**
```
You: What's the status of my Harness pipelines in the production project?
```

**Direct API Call:**
```bash
echo '{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "tools/call",
  "params": {
    "name": "harness_pipelines_list",
    "arguments": {
      "projectIdentifier": "production",
      "orgIdentifier": "default"
    }
  }
}' | websocat --header="Authorization: Bearer dev-admin-key-1234567890" \
             --header="X-Harness-API-Key: YOUR_HARNESS_KEY" \
             --header="X-Harness-Account-ID: YOUR_ACCOUNT_ID" \
             ws://localhost:8085/ws
```

**Supported Harness Operations:**
- List/get pipelines
- Execute pipelines
- Get execution status
- List GitOps applications
- Manage feature flags
- View security scans (STO)

### Use Case 5: Context-Aware Workflows

**Scenario:** Maintain conversation context across tool calls

**Example Workflow:**
```
1. You: Get repository info for developer-mesh/developer-mesh
   (Edge MCP stores repository details in session context)

2. You: Now show me issues in that repo
   (Edge MCP uses stored context - you don't need to repeat repo name)

3. You: Create a task to fix the highest priority issue
   (Edge MCP references context for repo and issue details)
```

**API Implementation:**
```bash
# First call - stores context
echo '{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "tools/call",
  "params": {
    "name": "github_get_repository",
    "arguments": {"owner": "developer-mesh", "repo": "developer-mesh"}
  }
}' | websocat --header="Authorization: Bearer dev-admin-key-1234567890" ws://localhost:8085/ws

# Update context manually if needed
echo '{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "context.update",
  "params": {
    "context": {
      "current_repository": "developer-mesh/developer-mesh",
      "current_branch": "main"
    }
  }
}' | websocat --header="Authorization: Bearer dev-admin-key-1234567890" ws://localhost:8085/ws
```

---

## 📋 Available Tools

Edge MCP provides 200+ tools across multiple categories:

### Built-in Tools (Always Available)

| Tool Name | Description | Category |
|-----------|-------------|----------|
| `devmesh_agent_assign` | Assign tasks to AI agents | agent_management |
| `devmesh_task_create` | Create new tasks | task_management |
| `devmesh_task_status` | Get/update task status | task_management |
| `devmesh_workflow_execute` | Execute predefined workflows | workflow_management |
| `devmesh_workflow_list` | List available workflows | workflow_management |
| `devmesh_context_update` | Update session context | context_management |
| `devmesh_context_get` | Get current context | context_management |

### GitHub Tools (Requires GitHub Token)

| Tool Category | Example Tools | Count |
|---------------|---------------|-------|
| Repository | `github_get_repository`, `github_list_repositories` | 15+ |
| Issues | `github_list_issues`, `github_create_issue` | 10+ |
| Pull Requests | `github_list_pull_requests`, `github_get_pull_request` | 12+ |
| Workflows | `github_list_workflows`, `github_run_workflow` | 8+ |
| Releases | `github_list_releases`, `github_create_release` | 6+ |

### Harness Tools (Requires Harness API Key)

| Tool Category | Example Tools | Count |
|---------------|---------------|-------|
| Pipelines | `harness_pipelines_list`, `harness_pipelines_execute` | 8+ |
| GitOps | `harness_gitops_applications_list`, `harness_gitops_applications_sync` | 10+ |
| Feature Flags | `harness_featureflags_list`, `harness_featureflags_toggle` | 6+ |
| STO (Security) | `harness_sto_scans_list`, `harness_sto_vulnerabilities_list` | 5+ |
| Cost Management | `harness_ccm_costs_overview`, `harness_ccm_recommendations_list` | 8+ |

**Discover All Tools:**
```bash
# List all available tools
echo '{
  "jsonrpc": "2.0",
  "id": 8,
  "method": "tools/list"
}' | websocat --header="Authorization: Bearer dev-admin-key-1234567890" ws://localhost:8085/ws
```

---

## ❓ Frequently Asked Questions

### Q: Do I need Core Platform for Edge MCP to work?

**A:** No! Edge MCP has two modes:

- **Standalone Mode (Default)**: Works independently with built-in tools (agent, workflow, task, context management)
- **Connected Mode (Optional)**: Connects to Core Platform for GitHub, Harness, and other dynamic tools

For basic AI agent orchestration, standalone mode is sufficient. For DevOps tool integrations, configure Core Platform.

### Q: What are the default API keys for development?

**A:** For local development with Docker Compose:

- **Edge MCP API Key:** `dev-admin-key-1234567890`
- **Core Platform Admin Key:** `dev-admin-key-1234567890`
- **Core Platform Reader Key:** `dev-readonly-key-1234567890`

**⚠️ WARNING:** Never use these keys in production! Generate secure keys for production deployments.

### Q: How do I add GitHub/Harness credentials?

**A:** Use passthrough authentication headers:

**Claude Code (`~/.claude/mcp.json`):**
```json
{
  "mcpServers": {
    "edge-mcp": {
      "url": "ws://localhost:8085/ws",
      "headers": {
        "Authorization": "Bearer dev-admin-key-1234567890",
        "X-GitHub-Token": "ghp_yourPersonalAccessToken",
        "X-Harness-API-Key": "pat.yourHarnessAPIKey",
        "X-Harness-Account-ID": "yourAccountID"
      }
    }
  }
}
```

**Environment Variables (for Edge MCP):**
```bash
export GITHUB_TOKEN=ghp_yourPersonalAccessToken
export HARNESS_API_KEY=pat.yourHarnessAPIKey
export HARNESS_ACCOUNT_ID=yourAccountID
```

### Q: What if a tool call fails?

**A:** Edge MCP provides semantic error messages with recovery steps:

**Example Error Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 9,
  "error": {
    "code": -32001,
    "message": "Tool not found: github_invalid_tool",
    "data": {
      "error_code": "TOOL_NOT_FOUND",
      "severity": "ERROR",
      "recovery_steps": [
        "List available tools using tools/list method",
        "Check tool name spelling (use fuzzy search)",
        "Verify tool category matches your use case"
      ],
      "alternatives": [
        "github_get_repository",
        "github_list_repositories",
        "github_search_repositories"
      ]
    }
  }
}
```

**Common Errors and Solutions:**

| Error Code | Meaning | Solution |
|------------|---------|----------|
| `TOOL_NOT_FOUND` | Tool doesn't exist | Use `/tools list` to find correct tool name |
| `AUTHENTICATION_FAILED` | Invalid API key | Check Authorization header |
| `RATE_LIMIT_EXCEEDED` | Too many requests | Wait for `retry_after` seconds |
| `PARAMETER_VALIDATION_FAILED` | Invalid parameters | Check parameter types in error message |
| `PERMISSION_DENIED` | Missing permissions | Verify GitHub/Harness token has required scopes |

### Q: How do I enable Redis for production?

**A:** Configure Redis environment variables:

```bash
# In your .env or docker-compose.yml
export REDIS_ENABLED=true
export REDIS_URL=redis://redis:6379

# For Kubernetes
export REDIS_URL=redis://redis.default.svc.cluster.local:6379

# Start Edge MCP
docker-compose -f docker-compose.local.yml up edge-mcp
```

Edge MCP will automatically use Redis for L2 caching if available, or fall back to memory-only mode.

### Q: Can I run Edge MCP without Docker?

**A:** Yes! Run from source:

```bash
cd apps/edge-mcp

# Set required environment variables
export EDGE_MCP_API_KEY=your-api-key
export EDGE_MCP_PORT=8082

# Optional: Connect to Core Platform
export DEV_MESH_URL=http://localhost:8081
export DEV_MESH_API_KEY=your-core-api-key

# Run Edge MCP
go run cmd/server/main.go

# Or build and run binary
go build -o edge-mcp cmd/server/main.go
./edge-mcp
```

### Q: How do I monitor Edge MCP in production?

**A:** Use built-in observability endpoints:

```bash
# Health checks (Kubernetes probes)
curl http://localhost:8085/health/live      # Liveness probe
curl http://localhost:8085/health/ready     # Readiness probe
curl http://localhost:8085/health/startup   # Startup probe

# Prometheus metrics
curl http://localhost:8085/metrics

# Key metrics:
# - edge_mcp_tool_execution_duration_seconds (histogram)
# - edge_mcp_active_connections (gauge)
# - edge_mcp_errors_total (counter)
# - edge_mcp_cache_hit_ratio (gauge)
# - edge_mcp_rate_limited_total (counter)
```

Configure Prometheus scraping:
```yaml
scrape_configs:
  - job_name: 'edge-mcp'
    static_configs:
      - targets: ['localhost:8085']
    metrics_path: /metrics
```

### Q: What's the difference between Edge MCP and Core Platform?

**A:**

| Component | Purpose | When to Use |
|-----------|---------|-------------|
| **Edge MCP** | WebSocket MCP server for AI clients | Always required - direct client connection |
| **Core Platform** | REST API for dynamic tool discovery | Optional - only needed for GitHub/Harness tools |

**Architecture:**
```
AI Client (Claude Code, Cursor)
    ↓ WebSocket (MCP Protocol)
Edge MCP (port 8082/8085)
    ↓ HTTP (optional)
Core Platform (port 8081)
    ↓ HTTP
External APIs (GitHub, Harness)
```

### Q: How do I update Edge MCP?

**A:** Pull latest changes and rebuild:

```bash
# Stop services
docker-compose -f docker-compose.local.yml down

# Pull latest code
git pull origin main

# Rebuild images
docker-compose -f docker-compose.local.yml build edge-mcp

# Start with fresh state
docker-compose -f docker-compose.local.yml up -d

# Verify update
curl http://localhost:8085/health
```

### Q: Can I use Edge MCP with custom tools?

**A:** Yes! Edge MCP supports dynamic tool integration:

1. **Add tool to Core Platform** via REST API
2. **Edge MCP auto-discovers** new tools on startup
3. **AI clients see new tools** in tools/list

See [Dynamic Tools API Documentation](./dynamic_tools_api.md) for details.

---

## 🎬 Video Walkthrough Script

**Duration:** 5 minutes
**Target Audience:** Developers new to Edge MCP
**Format:** Screen recording with narration

### Scene 1: Introduction (30 seconds)

**[Screen: Terminal with empty workspace]**

> "Hi! In the next 5 minutes, I'll show you how to set up Edge MCP and execute your first AI-powered DevOps automation. Edge MCP is a WebSocket server that implements the Model Context Protocol, enabling AI agents like Claude to interact with GitHub, Harness, and other DevOps tools."

### Scene 2: Installation (90 seconds)

**[Screen: Clone repository]**

```bash
git clone https://github.com/developer-mesh/developer-mesh.git
cd developer-mesh
```

> "First, clone the repository. Edge MCP is part of the Developer Mesh platform."

**[Screen: Start Docker Compose]**

```bash
docker-compose -f docker-compose.local.yml up -d
```

> "Start all services with Docker Compose. This launches Edge MCP on port 8085, the Core Platform on 8081, and supporting services like PostgreSQL and Redis."

**[Screen: Health check]**

```bash
curl http://localhost:8085/health/ready
```

> "Wait about 30 seconds for initialization, then verify Edge MCP is healthy. You should see a JSON response with status 'healthy' and all components reporting healthy."

### Scene 3: Configuration (60 seconds)

**[Screen: Open text editor with ~/.claude/mcp.json]**

```json
{
  "mcpServers": {
    "edge-mcp": {
      "transport": "websocket",
      "url": "ws://localhost:8085/ws",
      "headers": {
        "Authorization": "Bearer dev-admin-key-1234567890"
      }
    }
  }
}
```

> "Configure your AI client - I'm using Claude Code. Create an MCP configuration file that points to Edge MCP's WebSocket endpoint. Use the development API key for local testing."

**[Screen: Add GitHub token to config]**

```json
"headers": {
  "Authorization": "Bearer dev-admin-key-1234567890",
  "X-GitHub-Token": "ghp_yourtoken"
}
```

> "Add your GitHub token as a passthrough header. This lets Edge MCP access your GitHub repositories on your behalf."

### Scene 4: First Tool Call (90 seconds)

**[Screen: Claude Code terminal]**

```
claude
```

> "Start Claude Code. It automatically connects to Edge MCP."

**[Screen: List tools]**

```
/tools list
```

> "List available tools. You'll see 200+ tools including GitHub operations, Harness pipelines, and built-in agent orchestration tools."

**[Screen: Execute tool]**

```
You: Get repository information for developer-mesh/developer-mesh
```

> "Ask Claude to get repository information. Watch as Edge MCP routes the request through the GitHub API and returns structured data."

**[Screen: Show JSON response in terminal]**

> "Edge MCP returns comprehensive repository details - stars, forks, open issues, language breakdown, and more. All formatted for the AI agent to analyze and present insights."

### Scene 5: Batch Operations (60 seconds)

**[Screen: Complex query]**

```
You: For developer-mesh/developer-mesh, get the repository info,
     list the 5 most recent issues, and show active pull requests
```

> "Now let's do something more complex. Ask for multiple pieces of information at once."

**[Screen: Show parallel execution logs]**

> "Edge MCP automatically batches these into parallel tool calls. All three operations execute simultaneously, reducing latency from 3 seconds to under 1 second."

**[Screen: Show formatted results]**

> "Claude receives all the data and presents it in a unified, readable format. This is the power of AI agent orchestration - complex operations made simple."

### Scene 6: Conclusion (30 seconds)

**[Screen: Terminal with summary]**

> "That's it! In 5 minutes, you've installed Edge MCP, connected an AI client, and executed DevOps automation. You now have access to 200+ tools for GitHub, Harness, and agent orchestration."

**[Screen: Documentation links]**

> "For advanced features like batch operations, context management, and custom tools, check out the full documentation at github.com/developer-mesh/developer-mesh. Thanks for watching!"

---

## 🔗 Next Steps

### Beginner Path

1. ✅ **Complete Quick Start** (you are here)
2. 📖 **Read Integration Guides**
   - [Claude Code Integration](./integrations/claude-code.md)
   - [Cursor Integration](./integrations/cursor.md)
   - [Generic MCP Client](./integrations/generic-mcp-client.md)
3. 🔧 **Explore Tools**
   - Use `/tools list` in your AI client
   - Filter by category (repository, issues, ci_cd, etc.)
   - Read [Tool Usage Examples](./tool-usage-examples.md)

### Intermediate Path

4. 🎯 **Try Common Use Cases**
   - GitHub repository analysis
   - Harness pipeline monitoring
   - Multi-agent task delegation
5. 🔄 **Use Batch Operations**
   - Combine multiple tools in one request
   - Learn parallel vs sequential execution
6. 🧠 **Leverage Context Management**
   - Store workflow state in session context
   - Reference previous tool results

### Advanced Path

7. 🏗️ **Deploy to Production**
   - [Kubernetes Deployment Guide](../deployments/k8s/README.md)
   - Configure Redis for distributed caching
   - Enable distributed tracing (OpenTelemetry)
8. 🔐 **Security Hardening**
   - Generate production API keys
   - Configure rate limiting per tenant
   - Enable TLS/SSL (wss://)
9. 📊 **Monitoring & Observability**
   - Set up Prometheus scraping
   - Configure Grafana dashboards
   - Integrate with Jaeger for tracing

---

## 📚 Related Documentation

- **[OpenAPI Specification](./openapi/edge-mcp.yaml)** - Complete API reference
- **[Error Handling Guide](./error-handling.md)** - Semantic errors and recovery
- **[Tool Usage Examples](./tool-usage-examples.md)** - Detailed examples for all tools
- **[Kubernetes Deployment](../deployments/k8s/README.md)** - Production deployment guide
- **[Troubleshooting Guide](./integrations/troubleshooting.md)** - Common issues and solutions
- **[Local Development Guide](./LOCAL_DEVELOPMENT.md)** - Development environment setup

---

## 🆘 Getting Help

### Documentation

- **GitHub Repository:** https://github.com/developer-mesh/developer-mesh
- **Issues:** https://github.com/developer-mesh/developer-mesh/issues
- **Discussions:** https://github.com/developer-mesh/developer-mesh/discussions

### Quick Diagnostics

```bash
# Check all service health
docker-compose -f docker-compose.local.yml ps

# View logs
docker-compose -f docker-compose.local.yml logs -f edge-mcp

# Test WebSocket connection
websocat ws://localhost:8085/ws

# Verify Core Platform
curl -H "X-API-Key: dev-admin-key-1234567890" \
  http://localhost:8081/api/v1/tools
```

### Common Issues

See [Troubleshooting Guide](./integrations/troubleshooting.md) for detailed solutions to:
- Connection errors
- Authentication failures
- Tool execution errors
- Rate limiting issues
- Performance problems

---

**Ready to automate your DevOps workflows with AI?** Start with the 5-minute setup above and explore the powerful capabilities of Edge MCP! 🚀
