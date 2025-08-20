# MCP Architecture Fix - Generic Tool Bridging

## Current Problem
Claude Code is configured with "devmesh" which points to Edge MCP server (has GitHub/Docker tools).
The DevMesh MCP server (localhost:8080) is a separate server with its own tools.
Claude Code can only use tools from ONE server at a time.

## Solution: Generic MCP-to-MCP Proxy

### Architecture
```
Claude Code 
    ↓
DevMesh MCP Server (localhost:8080)
    ├── Native DevMesh Tools (workflow, task, context)
    └── MCP Proxy → Edge MCP Server
                      ├── GitHub tools
                      ├── Docker tools
                      └── Filesystem tools
```

### Implementation Approach

1. **Dynamic Tool Discovery**
   - DevMesh queries Edge MCP for its tool list
   - Prefixes Edge tools with `edge_` namespace
   - Merges with native DevMesh tools

2. **Generic Tool Proxy**
   - Any tool starting with `edge_` gets proxied to Edge MCP
   - No hardcoding of specific tools
   - Transparent pass-through of arguments and results

3. **Configuration in claude_desktop_config.json**
   ```json
   {
     "mcpServers": {
       "devmesh": {
         "command": "ws",
         "args": ["ws://localhost:8080/ws"],
         "env": {
           "EDGE_MCP_PATH": "/usr/local/bin/edge-mcp"
         }
       }
     }
   }
   ```

### Benefits
- Single connection point for Claude Code
- All tools available through one interface
- No hardcoding of specific tools
- Easy to add new Edge MCP tools
- Maintains separation of concerns

### Alternative: Direct Edge MCP Integration
If Edge MCP is installed locally, DevMesh could:
1. Shell out to `edge-mcp tools list` to get available tools
2. Shell out to `edge-mcp tools execute` to run them
3. Present them as native MCP tools with `edge_` prefix

This keeps DevMesh generic and doesn't require WebSocket proxying.