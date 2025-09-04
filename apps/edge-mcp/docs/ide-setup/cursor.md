# Cursor Configuration

## Prerequisites

1. DevMesh account with organization API key (obtained during registration)
2. Edge MCP installed and in PATH
3. Your personal access tokens for services you want to use (GitHub, AWS, etc.) - optional

## Setup

Cursor requires Edge MCP to be running separately.

### Step 1: Start Edge MCP

```bash
# Set DevMesh credentials
export DEV_MESH_URL="https://api.devmesh.io"
export DEV_MESH_API_KEY="devmesh_xxx..."  # Your API key from organization registration

# Optional: Set personal access tokens for pass-through auth
export GITHUB_TOKEN="ghp_your_personal_access_token"
export AWS_ACCESS_KEY_ID="AKIAIOSFODNN7EXAMPLE"
export AWS_SECRET_ACCESS_KEY="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"

# Start Edge MCP
edge-mcp --port 8082
```

**Note**: Your organization's tenant ID is automatically determined from your API key. You no longer need to provide it separately.

### Step 2: Configure Cursor

Create `.cursor/mcp.json` in your project root:

```json
{
  "mcp": {
    "servers": [
      {
        "name": "devmesh",
        "type": "websocket",
        "url": "ws://localhost:8082/ws"
      }
    ]
  }
}
```

### Step 3: Restart Cursor

Restart Cursor for the configuration to take effect.

## Verification

1. Check Edge MCP is running: `ps aux | grep edge-mcp`
2. Look for the MCP indicator in Cursor's status bar
3. Tools should appear automatically in Cursor's command palette

## Available Tools

Tools are dynamically discovered from your DevMesh tenant. Common tools include:

- **GitHub**: Full API access for repos, PRs, issues, workflows
- **AWS**: S3, Lambda, EC2, CloudWatch, Bedrock
- **Slack**: Messaging and channel management
- **Jira**: Issue tracking and project management
- Plus any custom tools configured in your tenant

## Advanced: Auto-start Edge MCP

To have Cursor automatically start Edge MCP, create a task:

`.vscode/tasks.json`:
```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "Start Edge MCP",
      "type": "shell",
      "command": "edge-mcp",
      "args": ["--port", "8082"],
      "options": {
        "env": {
          "DEV_MESH_URL": "https://api.devmesh.ai",
          "DEV_MESH_API_KEY": "your-api-key",
          "TENANT_ID": "your-tenant-id"
        }
      },
      "isBackground": true,
      "problemMatcher": []
    }
  ]
}
```

## Troubleshooting

### Connection failed
- Ensure Edge MCP is running: `edge-mcp --port 8082`
- Check port 8082 is not in use: `lsof -i :8082`
- Verify WebSocket URL in configuration

### Authentication errors
- Check environment variables are set correctly
- Verify API key and tenant ID match DevMesh dashboard
- Look at Edge MCP logs for detailed error messages

### Tools not available
- Restart Cursor after configuration changes
- Check your DevMesh tenant has tools configured
- Verify service credentials in DevMesh dashboard