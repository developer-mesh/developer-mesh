# Edge MCP - Gateway to DevMesh Platform

Edge MCP is a lightweight MCP server that connects your IDE to the DevMesh Platform, providing secure access to GitHub, AWS, Slack, Jira, and other tools without managing credentials locally.

## Features

- ✅ **Zero Infrastructure** - No local Redis, PostgreSQL, or databases needed
- ✅ **Full MCP 2025-06-18 Protocol** - Industry-standard protocol implementation
- ✅ **Pass-Through Authentication** - Secure credential management via DevMesh Platform
- ✅ **Dynamic Tool Discovery** - Automatically discovers available tools from your tenant
- ✅ **Multi-Tenant Support** - Isolated workspaces with per-tenant tool configurations
- ✅ **IDE Compatible** - Works with Claude Code, Cursor, Windsurf, and any MCP client
- ✅ **Enterprise Security** - Centralized credential management, audit logging, usage tracking
- ✅ **Circuit Breaker** - Resilient connection handling with automatic retries

## Installation

> ⚠️ **Note**: Binary releases are not available yet as the project hasn't created any release tags. You can either build from source or use nightly builds.

### Option 1: Build from Source (Recommended)

```bash
# Clone repository
git clone https://github.com/developer-mesh/developer-mesh.git
cd developer-mesh/apps/edge-mcp

# Build
go build -o edge-mcp ./cmd/server

# Run
./edge-mcp --port 8082
```

### Option 2: Download Nightly Builds

Nightly builds are created automatically from the main branch. These are development builds and may be unstable.

1. Go to [GitHub Actions](https://github.com/developer-mesh/developer-mesh/actions/workflows/edge-mcp-ci.yml)
2. Click on the latest successful workflow run
3. Download the artifact for your platform from the "Artifacts" section
4. Extract and run the binary

### Option 3: Future Binary Releases

Once release tags are created, you'll be able to:

#### Quick Install Scripts (Coming Soon)
```bash
# Unix/Linux/macOS
curl -sSL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/apps/edge-mcp/install.sh | bash

# Windows (PowerShell as Administrator)
iwr -useb https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/apps/edge-mcp/install.ps1 | iex
```

#### Direct Downloads (Coming Soon)
Binary releases will be available at:
- macOS Apple Silicon: `edge-mcp-darwin-arm64.tar.gz`
- macOS Intel: `edge-mcp-darwin-amd64.tar.gz`
- Linux x64: `edge-mcp-linux-amd64.tar.gz`
- Linux ARM64: `edge-mcp-linux-arm64.tar.gz`
- Windows x64: `edge-mcp-windows-amd64.exe.zip`
- Windows ARM64: `edge-mcp-windows-arm64.exe.zip`


## Uninstallation

### Unix/Linux/macOS
```bash
# Remove binary
sudo rm -f /usr/local/bin/edge-mcp

# Or if installed elsewhere
which edge-mcp | xargs sudo rm -f
```

### Windows
```powershell
# Remove from Program Files
Remove-Item -Path "$env:ProgramFiles\edge-mcp" -Recurse -Force

# Remove from PATH (requires admin)
$path = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine)
$newPath = ($path.Split(';') | Where-Object { $_ -ne "$env:ProgramFiles\edge-mcp" }) -join ';'
[Environment]::SetEnvironmentVariable("Path", $newPath, [EnvironmentVariableTarget]::Machine)
```

## Quick Start

### 1. Register Your Organization

Register at DevMesh to get your API key:

```bash
curl -X POST https://api.devmesh.io/api/v1/auth/register/organization \
  -H "Content-Type: application/json" \
  -d '{
    "organization_name": "Your Company",
    "organization_slug": "your-company",
    "admin_email": "admin@company.com",
    "admin_name": "Your Name",
    "admin_password": "SecurePass123"
  }'
```

Save the `api_key` from the response - this is your authentication credential.

### 2. Configure and Run

```bash
# Set your DevMesh credentials
export DEV_MESH_URL=https://api.devmesh.io
export DEV_MESH_API_KEY=devmesh_xxx...   # Your API key from registration
# Note: Tenant ID is automatically determined from your API key

# Run Edge MCP
edge-mcp --port 8082
```

### 3. Configure Your IDE

See [IDE Setup Guide](#ide-integration) below.

## How It Works

Edge MCP acts as a secure gateway between your IDE and the DevMesh Platform:

```
┌─────────┐      MCP       ┌──────────┐      HTTPS      ┌─────────────┐
│   IDE   │ ◄──────────► │ Edge MCP │ ◄─────────────► │  DevMesh    │
│(Claude, │   WebSocket   │          │   Authenticated │  Platform   │
│ Cursor) │               │          │                 │             │
└─────────┘               └──────────┘                 └─────────────┘
                                                              │
                                                              ▼
                                                    ┌──────────────────┐
                                                    │ GitHub, AWS,     │
                                                    │ Slack, Jira, etc │
                                                    └──────────────────┘
```

**Key Points:**
- Your IDE connects to Edge MCP via WebSocket (MCP protocol)
- Edge MCP authenticates with DevMesh Platform using your API key
- DevMesh Platform stores and manages all actual service credentials (GitHub tokens, AWS keys, etc.)
- When you call a tool, Edge MCP proxies the request to DevMesh, which executes it with the appropriate credentials

## IDE Integration

Edge MCP works with any MCP-compatible IDE. See detailed setup guides:

- 📘 **[Claude Code Setup](./docs/ide-setup/claude-code.md)**
- 📗 **[Cursor Setup](./docs/ide-setup/cursor.md)**  
- 📙 **[Windsurf Setup](./docs/ide-setup/windsurf.md)**
- 📚 **[All IDE Configurations](./docs/ide-setup/README.md)**

### Quick Example (Claude Code)

```json
{
  "mcpServers": {
    "edge-mcp": {
      "command": "./apps/edge-mcp/bin/edge-mcp",
      "args": ["--port", "8082"]
    }
  }
}
```

For complete configuration with all options, see the [IDE setup guides](./docs/ide-setup/).

## Available Tools

Edge MCP dynamically discovers tools from your DevMesh tenant. The exact tools available depend on your tenant configuration.

### Common Tool Categories

#### 📦 Source Control
- **GitHub** - Manage repos, PRs, issues, workflows
- **GitLab** - Similar capabilities for GitLab
- **Bitbucket** - Atlassian integration

#### ☁️ Cloud Platforms
- **AWS** - S3, Lambda, EC2, CloudWatch, Bedrock
- **Google Cloud** - GCP services
- **Azure** - Microsoft cloud services

#### 💬 Communication
- **Slack** - Send messages, manage channels
- **Discord** - Bot operations
- **Email** - Send notifications

#### 📋 Project Management
- **Jira** - Issues, sprints, projects
- **Linear** - Modern issue tracking
- **Notion** - Documentation and wikis

#### 🔧 DevOps
- **Docker Hub** - Image management
- **Kubernetes** - Cluster operations
- **Terraform** - Infrastructure as code

#### 🤖 AI/ML
- **OpenAI** - GPT models
- **Anthropic** - Claude models
- **AWS Bedrock** - Multiple AI models

### Tool Discovery

Tools are discovered automatically when Edge MCP connects to DevMesh:

```bash
# View available tools in your tenant
curl -H "X-API-Key: your-api-key" \
  https://api.devmesh.ai/api/v1/tools?tenant_id=your-tenant-id
```

Your IDE will automatically see all available tools through MCP protocol discovery - no configuration needed!

## Pass-Through Authentication

### Overview

Pass-through authentication allows you to provide your personal access tokens (GitHub PAT, AWS credentials, etc.) to Edge MCP, which forwards them to DevMesh Platform. This enables actions to be performed as YOU, with your identity and permissions, rather than using shared service credentials.

### How to Provide Your Tokens

Edge MCP automatically detects personal access tokens from environment variables:

```bash
# Set your personal tokens before starting your IDE
export GITHUB_TOKEN="ghp_your_personal_access_token"
export AWS_ACCESS_KEY_ID="AKIAIOSFODNN7EXAMPLE"
export AWS_SECRET_ACCESS_KEY="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
export SLACK_TOKEN="xoxb-your-slack-token"
export JIRA_TOKEN="your-jira-api-token"
```

### Supported Services

Edge MCP automatically detects tokens for these services:

| Service | Environment Variables | Token Type |
|---------|----------------------|------------|
| GitHub | `GITHUB_TOKEN`, `GITHUB_PAT` | Personal Access Token |
| AWS | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN` | IAM Credentials |
| Slack | `SLACK_TOKEN`, `SLACK_API_TOKEN` | Bot/User Token |
| Jira | `JIRA_TOKEN`, `JIRA_API_TOKEN`, `ATLASSIAN_TOKEN` | API Token |
| GitLab | `GITLAB_TOKEN`, `GITLAB_PAT` | Personal Access Token |
| Bitbucket | `BITBUCKET_TOKEN`, `BITBUCKET_APP_PASSWORD` | App Password |
| Discord | `DISCORD_TOKEN`, `DISCORD_BOT_TOKEN` | Bot Token |

### How Credentials Work

Edge MCP uses a **three-tier authentication model**:

1. **IDE → Edge MCP**: Optional authentication using `EDGE_MCP_API_KEY`
2. **Edge MCP → DevMesh**: Required authentication using `DEV_MESH_API_KEY`
3. **DevMesh → Services**: DevMesh uses stored credentials for each service

```
┌──────────┐                    ┌──────────┐                    ┌──────────┐
│   IDE    │                    │ Edge MCP │                    │ DevMesh  │
│          │◄──────────────────►│          │◄──────────────────►│ Platform │
└──────────┘  Optional API Key  └──────────┘  Required API Key  └──────────┘
                                                                       │
                                                          Stored Service Credentials
                                                                       ▼
                                                              ┌────────────────┐
                                                              │ GitHub: token  │
                                                              │ AWS: key/secret│
                                                              │ Slack: token   │
                                                              └────────────────┘
```

### Benefits of Pass-Through Authentication

#### With Personal Tokens (Recommended)
- **Personal Attribution**: Actions show as performed by YOU
- **Respect Personal Limits**: Uses your personal rate limits and quotas
- **Audit Compliance**: Full traceability to individual users
- **Permission Scoping**: Limited to what YOU can access
- **Contribution Credit**: GitHub commits count toward your profile

#### Without Personal Tokens (Fallback)
- **Service Account**: Actions performed by DevMesh service account
- **Shared Limits**: Uses shared rate limits
- **Generic Attribution**: Shows as "DevMesh Bot" or similar
- **Broader Permissions**: May have access you don't personally have

### Security Benefits

- **No Local Credential Storage**: Tokens only in environment variables
- **Session-Only**: Tokens held in memory only during active session
- **Encrypted Transport**: All credentials sent over TLS/HTTPS
- **No Logging**: Tokens are never written to logs
- **Rotation Support**: Update tokens anytime without config changes

### Setting Up Credentials

1. **In DevMesh Dashboard**:
   - Add service credentials (GitHub tokens, AWS keys, etc.)
   - Configure which tools are available to your tenant
   - Set usage limits and permissions

2. **In Edge MCP**:
   - Only provide DevMesh API key and tenant ID
   - Edge MCP automatically discovers available tools
   - No service credentials needed locally

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DEV_MESH_URL` | Yes | DevMesh Platform endpoint (usually `https://api.devmesh.ai`) |
| `DEV_MESH_API_KEY` | Yes | Your DevMesh API key from dashboard (contains tenant information) |
| `EDGE_MCP_API_KEY` | No | Optional API key to secure IDE→Edge connection |
| `EDGE_MCP_ID` | No | Unique identifier for this Edge instance (auto-generated) |

## Testing

```bash
# Run tests
make test

# Test WebSocket connection
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}' | \
  websocat ws://localhost:8082/ws
```

## Building for Production

```bash
# Build for all platforms
make build-all

# Build Docker image
make docker-build
```

## Security Model

Edge MCP implements defense-in-depth with multiple security layers:

### 🔒 Command Execution Security
1. **Process Isolation** - Each command runs in its own process group
2. **Timeout Enforcement** - All commands have mandatory timeouts
3. **Command Allowlisting** - Only approved commands can execute
4. **Path Sandboxing** - File operations restricted to allowed directories
5. **No Shell Expansion** - Commands execute directly without shell interpretation
6. **Argument Validation** - Blocks injection attempts and dangerous patterns

### 🛡️ Data Protection
- **Environment Filtering** - Sensitive variables (API keys, tokens) are filtered
- **Credential Encryption** - All stored credentials use AES-256 encryption
- **Audit Logging** - All operations are logged with structured data

## Architecture

Edge MCP is designed as a lightweight, standalone MCP server with zero infrastructure dependencies:

### Infrastructure Independence
- **No Direct Database Access**: Edge MCP does not connect to PostgreSQL
- **No Direct Redis Access**: Edge MCP does not connect to Redis
- **In-Memory Only**: All state is maintained in-memory
- **API-Based Sync**: When connected to Core Platform, state synchronization happens via REST API, not direct infrastructure connections

Edge MCP architecture:

```
┌─────────────────┐
│   MCP Client    │ (Claude Code, Cursor, etc.)
└────────┬────────┘
         │ WebSocket (MCP Protocol)
┌────────▼────────┐
│   Edge MCP      │
│  ┌───────────┐  │
│  │ MCP Handler│ │
│  └─────┬─────┘  │
│  ┌─────▼─────┐  │
│  │Tool Registry│ │
│  └─────┬─────┘  │
│  ┌─────▼─────┐  │
│  │ Executor  │  │ ← Security Layer
│  └───────────┘  │
└─────────────────┘
         │ Optional
┌────────▼────────┐
│  Core Platform  │ (Advanced features, remote tools)
└─────────────────┘
```

### Key Components
- **MCP Handler** - Implements MCP 2025-06-18 protocol
- **Tool Registry** - Manages local and remote tools
- **Command Executor** - Secure command execution with sandboxing
- **Core Client** - Optional integration with DevMesh platform

## License

See LICENSE file in the repository root.