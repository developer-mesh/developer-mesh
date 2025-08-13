# Cursor Configuration

## Setup Instructions

1. Create a `.cursor` directory in your project root
2. Create `mcp.json` file with the configuration below
3. Restart Cursor to apply changes

## Example Configuration

Create `.cursor/mcp.json` in your project root:

```json
{
  "mcp": {
    "servers": [
      {
        "name": "edge-mcp",
        "type": "websocket",
        "url": "ws://localhost:8082/ws",
        "apiKey": "${CURSOR_MCP_API_KEY}",
        "description": "Edge MCP - Secure local tool execution",
        "capabilities": {
          "tools": true,
          "resources": true,
          "prompts": false
        },
        "tools": [
          "git.status",
          "git.diff",
          "git.log",
          "git.branch",
          "docker.build",
          "docker.ps",
          "shell.execute",
          "filesystem.read",
          "filesystem.write",
          "filesystem.list"
        ]
      }
    ],
    "defaultServer": "edge-mcp",
    "autoConnect": true
  }
}
```

## Environment Variables

Set these before starting Cursor:

```bash
export CURSOR_MCP_API_KEY="your-api-key"
```

## Starting Edge MCP

Before using in Cursor, start Edge MCP:

```bash
./apps/edge-mcp/bin/edge-mcp --port 8082
```

## Features Available

- **Git Integration**: Full git operations with parsed output
- **Docker Support**: Build and manage containers
- **Shell Commands**: Execute allowed commands securely
- **File Operations**: Read/write files with validation

## Security Notes

Edge MCP enforces strict security:
- Commands like `rm`, `sudo`, `chmod` are blocked
- Path traversal is prevented
- Environment variables are filtered
- All operations have timeouts