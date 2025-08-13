# IDE Setup Guide for Edge MCP

Edge MCP works with any MCP-compatible IDE. This directory contains configuration examples for popular IDEs.

## Supported IDEs

| IDE | Configuration File | Documentation |
|-----|-------------------|---------------|
| Claude Code | `.claude/mcp.json` | [claude-code.md](./claude-code.md) |
| Cursor | `.cursor/mcp.json` | [cursor.md](./cursor.md) |
| Windsurf | `.windsurf/mcp-config.json` | [windsurf.md](./windsurf.md) |

## Quick Setup

1. **Build Edge MCP**
   ```bash
   cd apps/edge-mcp
   make build
   ```

2. **Choose your IDE configuration** from the guides above

3. **Set environment variables** (optional for Core Platform integration)
   ```bash
   export EDGE_MCP_API_KEY="your-api-key"
   export CORE_PLATFORM_URL="https://api.devmesh.ai"
   export CORE_PLATFORM_API_KEY="your-core-api-key"
   export TENANT_ID="your-tenant-id"
   ```

4. **Start your IDE** and verify Edge MCP tools are available

## Common Configuration Options

All IDEs support these configuration options:

| Option | Description | Default |
|--------|-------------|---------|
| `port` | WebSocket port | 8082 |
| `log-level` | Logging verbosity (debug, info, warn, error) | info |
| `core-url` | Core Platform URL (optional) | - |
| `work-dir` | Working directory for commands | Current directory |

## Troubleshooting

### Connection Issues
- Ensure Edge MCP is built: `make build`
- Check port availability: `lsof -i :8082`
- Verify WebSocket endpoint: `ws://localhost:8082/ws`

### Authentication Errors
- Verify API key is set in environment
- Check Core Platform connectivity (if configured)

### Tools Not Available
- Restart your IDE after configuration changes
- Check Edge MCP logs: `tail -f edge-mcp.log`
- Verify MCP protocol version compatibility

## Security Notes

Edge MCP enforces strict security by default:
- ❌ Dangerous commands blocked (`rm`, `sudo`, `chmod`)
- ✅ Safe commands allowed (`ls`, `cat`, `grep`, `git`)
- 🔒 Path sandboxing prevents directory traversal
- 🛡️ Environment variables filtered for sensitive data
- ⏱️ All operations have timeouts

## Need Help?

- Check the main [Edge MCP README](../../README.md)
- Review security model documentation
- Open an issue in the DevMesh repository