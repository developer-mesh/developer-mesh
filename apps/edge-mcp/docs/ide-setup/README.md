# IDE Setup Guide for Edge MCP

Connect your IDE to DevMesh Platform through Edge MCP for access to GitHub, AWS, Slack, and more.

## Prerequisites

1. **DevMesh Account**: Register your organization and get an API key
2. **Organization API Key**: Obtained during organization registration (`devmesh_xxx...`)

## Quick Start

### Step 1: Install Edge MCP

```bash
# Build from source (until releases are available)
git clone https://github.com/developer-mesh/developer-mesh.git
cd developer-mesh/apps/edge-mcp
go build -o edge-mcp ./cmd/server
sudo mv edge-mcp /usr/local/bin/  # Add to PATH
```

### Step 2: Register Your Organization (if not done already)

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
# Save the api_key from the response!
```

### Step 3: Set DevMesh Credentials

```bash
# Add to your shell profile (~/.bashrc, ~/.zshrc, etc.)
export DEV_MESH_URL="https://api.devmesh.io"
export DEV_MESH_API_KEY="devmesh_xxx..."  # Your API key from registration

# Note: Tenant ID is no longer needed - it's automatically determined from your API key
```

### Step 4: Configure Your IDE

Choose your IDE:
- [Claude Code](./claude-code.md) - `.claude/mcp.json`
- [Cursor](./cursor.md) - `.cursor/mcp.json`
- [Windsurf](./windsurf.md) - `.windsurf/mcp-config.json`

## How Authentication Works

```
Your IDE → Edge MCP → DevMesh Platform → GitHub/AWS/Slack
         ↑            ↑                   ↑
     No creds    DevMesh API Key    Stored service credentials
```

**Key Points:**
- You never provide GitHub tokens, AWS keys, etc. to Edge MCP
- DevMesh Platform stores and manages all service credentials
- Edge MCP only needs your DevMesh API key to authenticate
- All tool calls are proxied through DevMesh with proper credentials

## Example IDE Configurations

### Claude Code (`.claude/mcp.json`)
```json
{
  "mcpServers": {
    "devmesh": {
      "command": "edge-mcp",
      "args": ["--port", "8082"],
      "env": {
        "DEV_MESH_URL": "${DEV_MESH_URL}",
        "DEV_MESH_API_KEY": "${DEV_MESH_API_KEY}"
      }
    }
  }
}
```

### Cursor (Start Edge MCP separately)
```bash
# Terminal 1: Start Edge MCP
edge-mcp --port 8082
```

```json
// .cursor/mcp.json
{
  "mcp": {
    "servers": [{
      "name": "devmesh",
      "type": "websocket",
      "url": "ws://localhost:8082/ws"
    }]
  }
}
```

### Windsurf (`.windsurf/mcp-config.json`)
```json
{
  "servers": {
    "devmesh": {
      "type": "local",
      "executable": "edge-mcp",
      "arguments": ["--port=8082"],
      "environment": {
        "DEV_MESH_URL": "https://api.devmesh.io",
        "DEV_MESH_API_KEY": "devmesh_xxx..."
      }
    }
  }
}
```

## How Tool Discovery Works

Edge MCP implements automatic tool discovery through the MCP protocol:

1. **IDE connects** to Edge MCP via WebSocket
2. **Edge MCP authenticates** with DevMesh Platform
3. **DevMesh returns** available tools for your tenant
4. **IDE discovers** tools via `tools/list` MCP method
5. **Tools appear** automatically in your IDE

**No manual configuration needed!** Your available tools depend on:
- Your tenant's tool configuration in DevMesh
- Service credentials configured in DevMesh
- Your subscription tier and limits

## Security Features

### Credential Security
- **Zero local storage**: No service credentials stored on your machine
- **Encrypted transport**: All communications use TLS/HTTPS
- **Token rotation**: Update credentials in DevMesh without changing local config
- **Audit logging**: All API calls logged with full attribution

### Execution Security
- **Tenant isolation**: Complete separation between tenants
- **Usage limits**: Configurable rate limits and quotas
- **Permission scoping**: Fine-grained tool permissions per user
- **Timeout enforcement**: All operations have configurable timeouts

## Troubleshooting

### Edge MCP won't start
```bash
# Check if installed
which edge-mcp

# Build from source if needed
cd apps/edge-mcp && go build -o edge-mcp ./cmd/server
```

### IDE can't connect
```bash
# Check if Edge MCP is running
ps aux | grep edge-mcp

# Check port availability
lsof -i :8082

# Test WebSocket endpoint
curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" http://localhost:8082/ws
```

### Tools not showing up
1. Restart your IDE after configuration
2. Check Edge MCP logs for errors
3. Verify MCP protocol version compatibility

## Common Issues

### "Authentication failed"
- Verify your DevMesh API key is correct (should start with `devmesh_`)
- Ensure your organization account is active
- Check environment variables are exported: `echo $DEV_MESH_API_KEY`
- Note: Tenant ID is automatically determined from your API key

### "No tools available"
- Check your tenant has tools configured in DevMesh
- Verify service credentials are added in DevMesh dashboard
- Restart your IDE after configuration changes

### "Tool execution failed"
- Check the specific service credentials in DevMesh
- Verify you have permissions for the requested operation
- Check usage limits haven't been exceeded

## Support

- **Documentation**: [docs.devmesh.ai](https://docs.devmesh.ai)
- **Dashboard**: [app.devmesh.ai](https://app.devmesh.ai)
- **Support**: support@devmesh.ai