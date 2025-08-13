# Edge MCP - Lightweight Model Context Protocol Server

Edge MCP is a secure, lightweight, standalone MCP server that runs on developer machines without requiring PostgreSQL, Redis, or other infrastructure dependencies. It provides local tool execution with enterprise-grade security controls.

## Features

- ✅ **Zero Infrastructure** - No Redis, PostgreSQL, or external dependencies
- ✅ **Full MCP 2025-06-18 Protocol** - Complete protocol implementation
- ✅ **Secure Local Tools** - Sandboxed execution of git, docker, and shell commands
- ✅ **Multi-Layer Security** - Command allowlisting, path validation, process isolation
- ✅ **Optional Core Platform** - Connect to DevMesh for advanced features
- ✅ **IDE Compatible** - Works with Claude Code, Cursor, Windsurf, and any MCP client
- ✅ **Offline Mode** - Full functionality without network connection
- ✅ **Circuit Breaker** - Resilient Core Platform integration

## Installation

### Quick Install (Recommended)

#### Unix/Linux/macOS
```bash
curl -sSL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/apps/edge-mcp/install.sh | bash
```

#### Windows (PowerShell as Administrator)
```powershell
iwr -useb https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/apps/edge-mcp/install.ps1 | iex
```

### Download Pre-built Binaries

Download the latest release for your platform from the [releases page](https://github.com/developer-mesh/developer-mesh/releases?q=edge-mcp&expanded=true).

| Platform | Architecture | Download |
|----------|-------------|----------|
| macOS | Apple Silicon (M1/M2/M3) | [edge-mcp-darwin-arm64.tar.gz](https://github.com/developer-mesh/developer-mesh/releases/latest/download/edge-mcp-darwin-arm64.tar.gz) |
| macOS | Intel | [edge-mcp-darwin-amd64.tar.gz](https://github.com/developer-mesh/developer-mesh/releases/latest/download/edge-mcp-darwin-amd64.tar.gz) |
| Linux | x64 | [edge-mcp-linux-amd64.tar.gz](https://github.com/developer-mesh/developer-mesh/releases/latest/download/edge-mcp-linux-amd64.tar.gz) |
| Linux | ARM64 | [edge-mcp-linux-arm64.tar.gz](https://github.com/developer-mesh/developer-mesh/releases/latest/download/edge-mcp-linux-arm64.tar.gz) |
| Windows | x64 | [edge-mcp-windows-amd64.exe.zip](https://github.com/developer-mesh/developer-mesh/releases/latest/download/edge-mcp-windows-amd64.exe.zip) |
| Windows | ARM64 | [edge-mcp-windows-arm64.exe.zip](https://github.com/developer-mesh/developer-mesh/releases/latest/download/edge-mcp-windows-arm64.exe.zip) |

### Manual Installation

#### macOS/Linux
```bash
# Download for your platform (example: macOS Apple Silicon)
curl -L https://github.com/developer-mesh/developer-mesh/releases/latest/download/edge-mcp-darwin-arm64.tar.gz -o edge-mcp.tar.gz

# Extract
tar -xzf edge-mcp.tar.gz

# Make executable
chmod +x edge-mcp-darwin-arm64

# Move to PATH (optional)
sudo mv edge-mcp-darwin-arm64 /usr/local/bin/edge-mcp

# Verify installation
edge-mcp --version
```

#### Windows
```powershell
# Download (example: Windows x64)
Invoke-WebRequest -Uri "https://github.com/developer-mesh/developer-mesh/releases/latest/download/edge-mcp-windows-amd64.exe.zip" -OutFile "edge-mcp.zip"

# Extract
Expand-Archive -Path "edge-mcp.zip" -DestinationPath .

# Move to Program Files (optional)
New-Item -ItemType Directory -Path "$env:ProgramFiles\edge-mcp" -Force
Move-Item edge-mcp-windows-amd64.exe "$env:ProgramFiles\edge-mcp\edge-mcp.exe"

# Add to PATH (optional, requires admin)
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";$env:ProgramFiles\edge-mcp", [EnvironmentVariableTarget]::Machine)

# Verify installation (restart terminal first if you added to PATH)
edge-mcp --version
```

### Build from Source

```bash
# Clone repository
git clone https://github.com/developer-mesh/developer-mesh.git
cd developer-mesh/apps/edge-mcp

# Build
go build -o edge-mcp ./cmd/server

# Run
./edge-mcp --port 8082
```

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

### Run Standalone

```bash
# Run with default settings
edge-mcp

# Run on specific port
edge-mcp --port 8082

# Run with debug logging
edge-mcp --log-level debug
```

### Run with Core Platform Integration

```bash
# Set environment variables
export CORE_PLATFORM_URL=https://api.devmesh.ai
export CORE_PLATFORM_API_KEY=your-api-key
export TENANT_ID=your-tenant-id

# Run with Core Platform connection
edge-mcp --core-url $CORE_PLATFORM_URL
```

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

### 🔧 Local Tools (Always Available)

#### Git Operations
- **`git.status`** - Get repository status with parsed output
  - Returns: branch, modified files, staged files, untracked files
- **`git.diff`** - Show changes with optional staging
  - Parameters: `path`, `staged` (boolean)
- **`git.log`** - View commit history
  - Parameters: `limit`, `format`, `since`, `until`
- **`git.branch`** - Manage branches
  - Parameters: `list`, `create`, `delete`, `switch`

#### Docker Operations
- **`docker.build`** - Build Docker images securely
  - Parameters: `context`, `tag`, `dockerfile`, `buildArgs`, `noCache`
  - Security: Path validation on build context
- **`docker.ps`** - List containers with JSON output
  - Parameters: `all` (boolean)
  - Returns: Structured container information

#### Shell Execution (Highly Secured)
- **`shell.execute`** - Execute allowed shell commands
  - Parameters: `command`, `args`, `cwd`, `env`
  - Security Features:
    - ❌ Blocked: `rm`, `sudo`, `chmod`, `chown`, `kill`, `shutdown`
    - ✅ Allowed: `ls`, `cat`, `grep`, `find`, `echo`, `pwd`, `go`, `make`, `npm`
    - No shell interpretation (prevents injection)
    - Path sandboxing
    - Environment variable filtering
    - Argument validation

#### File System Operations
- **`filesystem.read`** - Read file contents
- **`filesystem.write`** - Write file contents  
- **`filesystem.list`** - List directory contents
- **`filesystem.delete`** - Delete files (with validation)

### 🌐 Remote Tools (With Core Platform)

When connected to Core Platform, Edge MCP becomes a gateway to ALL DevMesh tools:

- **GitHub** - Full GitHub API (repos, PRs, issues, actions)
- **AWS** - S3, Lambda, CloudWatch, Bedrock
- **Slack** - Send messages, manage channels
- **Jira** - Create/update issues, manage sprints
- **Custom Tools** - Any tool configured in your tenant

Edge MCP automatically discovers and proxies these tools from Core Platform, providing:
- Unified authentication
- Centralized configuration
- Usage tracking and limits
- Audit logging

**How it works**: Edge MCP fetches available tools from Core Platform and creates local proxy handlers. When you call a remote tool, Edge MCP forwards the request to Core Platform, which executes it with proper credentials and returns the result.

## Configuration

Environment variables:
- `EDGE_MCP_API_KEY` - API key for client authentication
- `CORE_PLATFORM_URL` - Core Platform URL (optional)
- `CORE_PLATFORM_API_KEY` - Core Platform API key
- `TENANT_ID` - Your tenant ID
- `EDGE_MCP_ID` - Edge MCP identifier (auto-generated if not set)

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