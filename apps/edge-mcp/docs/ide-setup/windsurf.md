# Windsurf Configuration

## Setup Instructions

1. Create a `.windsurf` directory in your project root
2. Create `mcp-config.json` file with the configuration below
3. Restart Windsurf IDE

## Example Configuration

Create `.windsurf/mcp-config.json` in your project root:

```json
{
  "version": "1.0.0",
  "servers": {
    "edge-mcp": {
      "type": "local",
      "executable": "./apps/edge-mcp/bin/edge-mcp",
      "arguments": [
        "--port=8082",
        "--log-level=info"
      ],
      "environment": {
        "EDGE_MCP_API_KEY": "${env:WINDSURF_MCP_KEY}",
        "CORE_PLATFORM_URL": "${env:CORE_PLATFORM_URL}",
        "CORE_PLATFORM_API_KEY": "${env:CORE_API_KEY}",
        "TENANT_ID": "${env:TENANT_ID}"
      },
      "protocol": {
        "type": "websocket",
        "endpoint": "ws://localhost:8082/ws",
        "version": "2025-06-18"
      },
      "features": {
        "localTools": true,
        "remoteTools": true,
        "contextSync": true,
        "offlineMode": true
      },
      "security": {
        "commandSandboxing": true,
        "pathValidation": true,
        "environmentFiltering": true
      }
    }
  },
  "activeServer": "edge-mcp"
}
```

## Environment Setup

Configure in your shell profile:

```bash
export WINDSURF_MCP_KEY="your-api-key"
export CORE_PLATFORM_URL="https://api.devmesh.ai"  # Optional
export CORE_API_KEY="your-core-api-key"            # Optional
export TENANT_ID="your-tenant-id"                  # Optional
```

## Windsurf-Specific Features

- **Auto-connect**: Windsurf automatically starts Edge MCP
- **Security Indicators**: Shows sandboxing status in UI
- **Tool Discovery**: Automatic tool discovery from Core Platform
- **Offline Mode**: Full functionality without network

## Verification

Check Windsurf's MCP panel to verify:
1. Connection status shows "Connected"
2. Tools are listed and available
3. Security features are enabled (green indicators)