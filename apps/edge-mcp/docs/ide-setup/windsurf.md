# Windsurf Configuration

## Prerequisites

1. DevMesh account with organization API key (obtained during registration)
2. Edge MCP installed and in PATH
3. Your personal access tokens for services you want to use (GitHub, AWS, etc.) - optional

## Configuration

Create `.windsurf/mcp-config.json` in your project root:

```json
{
  "version": "1.0.0",
  "servers": {
    "devmesh": {
      "type": "local",
      "executable": "edge-mcp",
      "arguments": ["--port=8082"],
      "environment": {
        "DEV_MESH_URL": "https://api.devmesh.io",
        "DEV_MESH_API_KEY": "devmesh_xxx...",
        
        // Optional: Personal access tokens for pass-through auth
        "GITHUB_TOKEN": "ghp_your_personal_access_token",
        "AWS_ACCESS_KEY_ID": "AKIAIOSFODNN7EXAMPLE",
        "AWS_SECRET_ACCESS_KEY": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
      }
    }
  },
  "activeServer": "devmesh"
}
```

**Note**: Your organization's tenant ID is automatically determined from your API key. You no longer need to provide it separately.

Replace `devmesh_xxx...` with your actual API key obtained during organization registration.

## Verification

After restarting Windsurf:

1. Check the MCP panel shows "Connected"
2. Tools should be listed automatically (no manual configuration needed)
3. Test with a simple command

## Available Tools

Tools are dynamically discovered from your DevMesh tenant. Common tools include:

- **GitHub**: Complete API access for all GitHub operations
- **AWS**: Full AWS service integration (S3, Lambda, EC2, etc.)
- **Slack**: Messaging, channels, and workspace management
- **Jira**: Issues, projects, and sprint management
- Plus any custom tools configured in your tenant

## Features

Windsurf provides excellent MCP integration:

- **Auto-start**: Automatically starts Edge MCP when you open a project
- **Tool Discovery**: Automatically discovers available tools via MCP protocol
- **Status Indicators**: Shows connection status in the UI
- **Error Reporting**: Clear error messages for troubleshooting

## Alternative: Environment Variables

If you prefer not to hardcode credentials, use environment variables:

```json
{
  "version": "1.0.0",
  "servers": {
    "devmesh": {
      "type": "local",
      "executable": "edge-mcp",
      "arguments": ["--port=8082"],
      "environment": {
        "DEV_MESH_URL": "${env:DEV_MESH_URL}",
        "DEV_MESH_API_KEY": "${env:DEV_MESH_API_KEY}"
      }
    }
  },
  "activeServer": "devmesh"
}
```

Then set environment variables before starting Windsurf:

```bash
export DEV_MESH_URL="https://api.devmesh.ai"
export DEV_MESH_API_KEY="devmesh_xxx..."  # Your API key from registration
```

## Troubleshooting

### Edge MCP not found
- Ensure Edge MCP is in PATH: `which edge-mcp`
- Or use full path in configuration: `"executable": "/usr/local/bin/edge-mcp"`

### Authentication failed
- Verify API key from DevMesh dashboard (should start with `devmesh_`)
- Check Core Platform URL is correct
- Look at Windsurf's output panel for detailed errors

### Tools not appearing
- Check your DevMesh tenant configuration
- Verify service credentials are added in DevMesh dashboard
- Restart Windsurf after configuration changes