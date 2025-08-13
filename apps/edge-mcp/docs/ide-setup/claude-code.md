# Claude Code Configuration

## Setup Instructions

1. Create a `.claude` directory in your project root
2. Create `mcp.json` file with the configuration below
3. Restart Claude Code to apply changes

## Example Configuration

Create `.claude/mcp.json` in your project root:

```json
{
  "mcpServers": {
    "edge-mcp": {
      "command": "./apps/edge-mcp/bin/edge-mcp",
      "args": [
        "--port", "8082",
        "--log-level", "info"
      ],
      "env": {
        "EDGE_MCP_API_KEY": "${EDGE_MCP_API_KEY}",
        "CORE_PLATFORM_URL": "${CORE_PLATFORM_URL:-}",
        "CORE_PLATFORM_API_KEY": "${CORE_PLATFORM_API_KEY:-}",
        "TENANT_ID": "${TENANT_ID:-default}"
      },
      "description": "Edge MCP - Local development tools with secure execution"
    }
  },
  "defaultServer": "edge-mcp"
}
```

## Environment Variables

Set these in your shell before starting Claude Code:

```bash
export EDGE_MCP_API_KEY="your-api-key"
export CORE_PLATFORM_URL="https://api.devmesh.ai"  # Optional
export CORE_PLATFORM_API_KEY="your-core-api-key"   # Optional
export TENANT_ID="your-tenant-id"                  # Optional
```

## Verify Connection

After setup, you should see Edge MCP tools available in Claude Code:
- Git operations (status, diff, log, branch)
- Docker commands (build, ps)
- Shell execution (with security restrictions)
- File system operations

## Troubleshooting

1. **Connection Failed**: Check that Edge MCP is built and executable
2. **Tools Not Showing**: Restart Claude Code after configuration
3. **Authentication Error**: Verify your API keys are set correctly