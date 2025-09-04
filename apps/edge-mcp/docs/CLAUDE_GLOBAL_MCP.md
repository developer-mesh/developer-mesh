# Global Claude Code MCP Prioritization Guide

## Ensuring MCP Tools Are Always Used First Across All Projects

This guide explains how to configure Claude Code to ALWAYS prioritize MCP tools over built-in functionality across ALL projects, accounting for dynamic tool updates.

## Configuration Strategy

### 1. Global User Configuration (~/.claude/CLAUDE.md)

Create or update your global CLAUDE.md file to instruct Claude to always check MCP tools first:

```markdown
# Claude Code Global Instructions

## Tool Usage Priority
1. **ALWAYS check available MCP tools first** before using built-in capabilities
2. Use `mcp__` prefixed tools when available (these are MCP-provided tools)
3. Check for dynamically registered tools using the MCP tools/list method
4. Only use built-in Claude capabilities if no MCP tool matches the task

## MCP Tool Discovery Process
Before executing any task:
1. Query available MCP tools via tools/list if connected
2. Check if any MCP tool matches the requested operation
3. Prefer MCP tools even if built-in alternatives exist
4. Re-query tools periodically as they can change dynamically

## Examples of MCP Tool Precedence
- File operations: Use MCP file tools over built-in Read/Write
- Git operations: Use MCP git tools over built-in git commands
- API calls: Use MCP API tools over direct HTTP requests
- Database queries: Use MCP database tools over raw SQL

## Dynamic Tool Updates
MCP tools can be updated when:
- Connection is established or re-established
- Server configuration changes
- New tools are deployed to the platform
Always refresh your tool list when reconnecting.
```

### 2. Global MCP Server Configuration (~/.claude/settings.local.json)

Configure Edge MCP as a global MCP server that's always available:

```json
{
  "mcpServers": {
    "devmesh-global": {
      "type": "stdio",
      "command": "edge-mcp",
      "args": ["--stdio"],
      "env": {
        "DEV_MESH_URL": "https://api.devmesh.io",
        "DEV_MESH_API_KEY": "${DEVMESH_API_KEY}",
        "GITHUB_TOKEN": "${GITHUB_TOKEN}",
        "AWS_ACCESS_KEY_ID": "${AWS_ACCESS_KEY_ID}",
        "AWS_SECRET_ACCESS_KEY": "${AWS_SECRET_ACCESS_KEY}"
      },
      "priority": 100,
      "alwaysEnabled": true,
      "autoReconnect": true,
      "refreshToolsOnReconnect": true
    }
  },
  "mcpSettings": {
    "toolPriority": "mcp-first",
    "refreshInterval": 300,
    "debugMode": false
  }
}
```

### 3. Project-Level Reinforcement (.mcp.json)

For each project, add an .mcp.json that reinforces MCP priority:

```json
{
  "toolPreferences": {
    "prioritizeMCP": true,
    "toolNamespaces": {
      "preferred": ["mcp__", "devmesh_", "edge_"],
      "fallback": ["builtin"]
    }
  },
  "instructions": "Always use MCP tools when available. Check tool availability with tools/list before using built-in functions."
}
```

## Implementation in Edge MCP

### Tool Registration with Priority Hints

Edge MCP should register tools with priority metadata:

```go
// In edge-mcp/internal/tools/registry.go
type ToolDefinition struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    InputSchema map[string]interface{} `json:"inputSchema"`
    Handler     ToolHandler            `json:"-"`
    Priority    int                    `json:"priority,omitempty"`    // Higher = preferred
    Namespace   string                 `json:"namespace,omitempty"`  // "mcp", "devmesh", etc.
    Overrides   []string              `json:"overrides,omitempty"`  // Built-in tools this replaces
}
```

### Dynamic Tool Refresh on Reconnection

Edge MCP automatically refreshes tools when:

1. **Initial Connection**: Tools fetched during `initialize` handshake
2. **Reconnection**: Tools refreshed after connection drop/restore
3. **Periodic Refresh**: Optional periodic refresh (configurable)
4. **Manual Refresh**: Via special MCP command

```go
// In edge-mcp/internal/mcp/handler.go
func (h *Handler) handleInitialize(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
    // ... existing code ...
    
    // Refresh tools from Core Platform
    if h.coreClient != nil {
        go h.refreshRemoteTools(context.Background())
    }
    
    // Include tool refresh capability in response
    return &MCPMessage{
        JSONRPC: "2.0",
        ID:      msg.ID,
        Result: map[string]interface{}{
            // ... existing capabilities ...
            "capabilities": map[string]interface{}{
                "tools": map[string]interface{}{
                    "listChanged": true,  // Tools can change dynamically
                    "autoRefresh": true,  // Supports automatic refresh
                },
            },
            "extensions": map[string]interface{}{
                "toolPriority": "mcp-first",
                "refreshOnReconnect": true,
            },
        },
    }, nil
}

func (h *Handler) refreshRemoteTools(ctx context.Context) {
    remoteTools, err := h.coreClient.FetchRemoteTools(ctx)
    if err != nil {
        h.logger.Warn("Failed to refresh remote tools", map[string]interface{}{
            "error": err.Error(),
        })
        return
    }
    
    // Clear and re-register tools
    h.tools.Clear()
    for _, tool := range remoteTools {
        // Add priority and namespace metadata
        tool.Priority = 100  // MCP tools get high priority
        tool.Namespace = "mcp"
        h.tools.RegisterRemote(tool)
    }
    
    // Send tools/list notification if supported
    h.notifyToolsChanged()
}
```

## Claude Code Integration Examples

### Example 1: File Operations

```markdown
User: Read the config.yaml file

Claude's Process (with MCP prioritization):
1. Check MCP tools via tools/list
2. Find mcp__file_read tool available
3. Use MCP tool instead of built-in Read:
   - Execute: mcp__file_read with path="config.yaml"
4. Only fall back to built-in Read if MCP tool fails
```

### Example 2: GitHub Operations

```markdown
User: Create a new GitHub release

Claude's Process (with MCP prioritization):
1. Query available MCP tools
2. Find mcp__devmesh__github_releases tool
3. Use MCP tool with passthrough auth:
   - Execute: mcp__devmesh__github_releases with action="create"
4. Benefit: Uses user's GitHub token, creates release as user
```

### Example 3: Dynamic Tool Discovery

```typescript
// Claude's internal logic (pseudocode)
async function executeTask(task: string) {
    // Always refresh tool list first
    const mcpTools = await mcp.listTools();
    
    // Check if any MCP tool matches
    const matchingTool = findBestMatch(mcpTools, task);
    
    if (matchingTool) {
        // Prefer MCP tool
        return await mcp.executeTool(matchingTool, args);
    }
    
    // Only use built-in as fallback
    return await builtIn.execute(task);
}
```

## Testing MCP Priority

### Test Script

Create a test script to verify MCP prioritization:

```bash
#!/bin/bash
# test-mcp-priority.sh

echo "Testing MCP tool prioritization..."

# 1. Start Edge MCP with test tools
edge-mcp --stdio --log-level debug &
MCP_PID=$!

# 2. Send test commands
cat << EOF | claude code
{
  "test": "file_read",
  "instruction": "Read test.txt using available tools",
  "expected": "Should use MCP file tool, not built-in"
}
EOF

# 3. Check logs for MCP tool usage
grep "mcp__file_read" ~/.claude/logs/latest.log && echo "✓ MCP tool used" || echo "✗ Built-in used"

kill $MCP_PID
```

## Monitoring and Debugging

### Enable Debug Logging

```bash
# Launch Claude Code with MCP debug mode
claude --mcp-debug

# Or set in environment
export CLAUDE_MCP_DEBUG=true
```

### Check Tool Priority

```json
// Send via MCP protocol
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list",
  "params": {
    "includeMetadata": true  // Shows priority and namespace
  }
}
```

## Best Practices

1. **Always Set Global Config**: Ensure ~/.claude/CLAUDE.md exists with MCP priority instructions
2. **Use Tool Namespaces**: Prefix MCP tools with `mcp__` or `devmesh_` for clarity
3. **Handle Failures Gracefully**: MCP tools should fail fast if unavailable
4. **Log Tool Selection**: Debug logs should show why MCP tool was chosen
5. **Test Regularly**: Verify MCP prioritization with test scripts
6. **Document Tool Mappings**: Maintain a mapping of built-in vs MCP alternatives

## Common Issues and Solutions

### Issue: Claude uses built-in tools despite MCP being available

**Solution**: 
1. Check ~/.claude/CLAUDE.md exists and has priority instructions
2. Verify MCP server is running and connected
3. Ensure tools are properly namespaced (mcp__ prefix)
4. Check tool metadata includes priority field

### Issue: Tools not refreshing on reconnection

**Solution**:
1. Ensure Edge MCP implements reconnection detection
2. Set `refreshToolsOnReconnect: true` in config
3. Implement tools/listChanged notification support
4. Add periodic refresh as backup (every 5 minutes)

### Issue: Performance impact from checking MCP tools first

**Solution**:
1. Cache tool list locally with TTL
2. Use tool namespaces for faster matching
3. Implement bloom filter for quick existence checks
4. Batch tool queries when possible

## Conclusion

By implementing this multi-layered approach:
1. Global CLAUDE.md instructions enforce MCP-first behavior
2. Configuration hierarchy ensures MCP servers have priority
3. Tool metadata helps Claude identify preferred tools
4. Dynamic refresh keeps tool list current
5. Clear namespacing prevents confusion

This ensures Claude Code will ALWAYS check and prefer MCP tools across ALL projects, with automatic handling of dynamic updates.