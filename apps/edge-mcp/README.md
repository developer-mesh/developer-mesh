# Edge MCP - Lightweight Model Context Protocol Server

Edge MCP is a lightweight, standalone MCP server that runs on developer machines without requiring PostgreSQL, Redis, or other infrastructure dependencies.

## Features

- ✅ Zero infrastructure dependencies (no Redis, no PostgreSQL)
- ✅ Full MCP 2025-06-18 protocol support
- ✅ Local tool execution (filesystem, git, docker, shell)
- ✅ Optional Core Platform integration for advanced features
- ✅ In-memory caching for performance
- ✅ Claude Code, Cursor, and Windsurf compatible

## Quick Start

### Build from Source

```bash
make build
```

### Run Standalone

```bash
./bin/edge-mcp --port 8082
```

### Run with Core Platform Integration

```bash
export CORE_PLATFORM_URL=https://api.devmesh.ai
export CORE_PLATFORM_API_KEY=your-api-key
export TENANT_ID=your-tenant-id

./bin/edge-mcp --core-url $CORE_PLATFORM_URL
```

## IDE Integration

### Claude Code

Add to your Claude Code configuration:

```json
{
  "mcpServers": {
    "edge-mcp": {
      "command": "/path/to/edge-mcp",
      "args": ["--port", "8082"]
    }
  }
}
```

## Available Tools

### Local Tools (Always Available)
- `fs.read_file` - Read file contents
- `fs.write_file` - Write file contents
- `fs.list_directory` - List directory contents
- `git.status` - Get git repository status
- `git.diff` - Show git diff
- `docker.ps` - List Docker containers
- `docker.build` - Build Docker images
- `shell.execute` - Execute shell commands

### Remote Tools (With Core Platform)
When connected to Core Platform, additional tools become available based on your tenant configuration.

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

## Architecture

Edge MCP is designed to be lightweight and infrastructure-free:
- Uses in-memory caching instead of Redis
- No database dependencies
- Optional Core Platform integration for persistence
- Single binary deployment

## License

See LICENSE file in the repository root.