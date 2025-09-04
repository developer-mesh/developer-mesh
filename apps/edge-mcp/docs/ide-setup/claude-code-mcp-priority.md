# Claude Code MCP Priority Configuration

This guide ensures Claude Code ALWAYS uses MCP tools first across ALL your projects.

## Quick Setup (3 Steps)

### Step 1: Create Global CLAUDE.md Instructions

Create or update `~/.claude/CLAUDE.md`:

```markdown
# Global Instructions for Claude Code

## CRITICAL: Tool Usage Priority
1. **ALWAYS** check and use MCP tools first before ANY built-in function
2. Query `tools/list` at the start of EVERY session to see available MCP tools
3. Use tools prefixed with `mcp__` or `devmesh_` when available
4. Only use built-in tools (Read, Write, Bash, etc.) if NO MCP alternative exists
5. Re-query tools after reconnections as they update dynamically

## Examples of Required MCP Tool Usage
- GitHub operations: MUST use `mcp__devmesh__github_*` tools
- File operations: Check for MCP file tools before using built-in Read/Write
- API calls: Use MCP API tools instead of direct HTTP requests
- Database: Use MCP database tools instead of raw SQL

## Dynamic Tool Awareness
MCP tools can change when:
- Starting a new session
- After connection drops/reconnects  
- When server configuration updates
- Periodically (every 5 minutes)

ALWAYS refresh your tool awareness with tools/list when starting tasks.
```

### Step 2: Configure Global MCP Server

Add Edge MCP as a global server in `~/.claude/settings.local.json`:

```json
{
  "mcpServers": {
    "devmesh": {
      "type": "stdio",
      "command": "edge-mcp",
      "args": ["--stdio"],
      "env": {
        "DEV_MESH_URL": "${DEV_MESH_URL:-http://localhost:8081}",
        "DEV_MESH_API_KEY": "${DEV_MESH_API_KEY}",
        "GITHUB_TOKEN": "${GITHUB_TOKEN}",
        "AWS_ACCESS_KEY_ID": "${AWS_ACCESS_KEY_ID}",
        "AWS_SECRET_ACCESS_KEY": "${AWS_SECRET_ACCESS_KEY}"
      }
    }
  }
}
```

### Step 3: Set Environment Variables

Add to your shell profile (`~/.zshrc` or `~/.bashrc`):

```bash
# DevMesh MCP Configuration
export DEV_MESH_URL="http://localhost:8081"  # or https://api.devmesh.io
export DEV_MESH_API_KEY="devmesh_your_api_key_here"

# Pass-through Authentication (optional but recommended)
export GITHUB_TOKEN="ghp_your_github_token"
export AWS_ACCESS_KEY_ID="your_aws_key"
export AWS_SECRET_ACCESS_KEY="your_aws_secret"
```

## Verification

### Test MCP Priority

1. Start Claude Code:
```bash
claude --mcp-debug  # Enable debug mode to see tool selection
```

2. Ask Claude to perform a GitHub operation:
```
"List the open issues in this repository"
```

3. Verify in logs that MCP tool was used:
```bash
# Check Claude used the MCP tool, not built-in
tail -f ~/.claude/logs/latest.log | grep "mcp__devmesh__github"
```

### Check Available Tools

Ask Claude directly:
```
"Show me all available MCP tools using tools/list"
```

Expected response should show DevMesh tools like:
- `mcp__devmesh__github_repos`
- `mcp__devmesh__github_issues`
- `mcp__devmesh__github_pulls`
- etc.

## Project-Specific Reinforcement

For critical projects, add `.claude.json` in the project root:

```json
{
  "instructions": "CRITICAL: Use MCP tools exclusively. Query tools/list before any operation. Never use built-in tools if MCP alternative exists.",
  "mcpServers": {
    "devmesh": {
      "type": "stdio",
      "command": "edge-mcp",
      "args": ["--stdio", "--log-level", "debug"],
      "env": {
        "DEV_MESH_URL": "${DEV_MESH_URL}",
        "DEV_MESH_API_KEY": "${DEV_MESH_API_KEY}",
        "TOOL_PRIORITY": "mcp-only"
      }
    }
  }
}
```

## Advanced Configuration

### Tool Priority Scoring

Create `~/.claude/tool-priority.json`:

```json
{
  "toolPriority": {
    "rules": [
      {
        "pattern": "github.*",
        "prefer": "mcp__devmesh__github_*",
        "weight": 100
      },
      {
        "pattern": "file.*",
        "prefer": "mcp__file_*",
        "weight": 90
      },
      {
        "pattern": "api.*",
        "prefer": "mcp__api_*",
        "weight": 85
      }
    ],
    "defaultStrategy": "mcp-first",
    "fallbackToBuiltin": true
  }
}
```

### Monitoring Tool Usage

Enable detailed logging to monitor which tools Claude uses:

```bash
# Create a monitoring script
cat > ~/bin/claude-mcp-monitor.sh << 'EOF'
#!/bin/bash
echo "Monitoring Claude MCP tool usage..."
tail -f ~/.claude/logs/latest.log | while read line; do
  if echo "$line" | grep -q "tool:.*mcp__"; then
    echo "[✓] MCP Tool Used: $line"
  elif echo "$line" | grep -q "tool:.*builtin"; then
    echo "[✗] Built-in Tool Used: $line"
  fi
done
EOF

chmod +x ~/bin/claude-mcp-monitor.sh
```

## Troubleshooting

### Claude Not Using MCP Tools

1. **Check CLAUDE.md exists and is loaded:**
```bash
ls -la ~/.claude/CLAUDE.md
# Should show the file exists
```

2. **Verify MCP server is running:**
```bash
ps aux | grep edge-mcp
# Should show edge-mcp process
```

3. **Check tool registration:**
```bash
# In Claude Code, ask:
"Run tools/list and show me all available tools with their namespaces"
```

4. **Enable debug logging:**
```bash
export CLAUDE_MCP_DEBUG=true
claude --mcp-debug
```

### Tools Not Refreshing

1. **Check refresh configuration:**
```json
// In ~/.claude/settings.local.json
{
  "mcpServers": {
    "devmesh": {
      // ... existing config ...
      "refreshInterval": 300,  // 5 minutes
      "autoRefresh": true
    }
  }
}
```

2. **Force manual refresh:**
```
"Refresh the MCP tool list using tools/list"
```

### Performance Issues

If checking MCP tools first causes delays:

1. **Enable tool caching:**
```json
{
  "mcpSettings": {
    "toolCache": {
      "enabled": true,
      "ttl": 60  // Cache for 60 seconds
    }
  }
}
```

2. **Use tool namespaces for faster matching:**
```json
{
  "toolNamespaces": {
    "priority": ["mcp__", "devmesh_"],
    "ignore": ["deprecated_", "test_"]
  }
}
```

## Best Practices

1. **Always Set Global Instructions**: The `~/.claude/CLAUDE.md` file is critical
2. **Use Clear Tool Naming**: Prefix MCP tools with `mcp__` for clarity
3. **Monitor Tool Usage**: Regularly check logs to ensure MCP priority
4. **Handle Failures Gracefully**: Configure fallback behavior
5. **Document Tool Mappings**: Keep a list of built-in vs MCP alternatives

## Example Scenarios

### Scenario 1: GitHub PR Creation

**Without MCP Priority:**
```
Claude might use: Built-in git commands + manual API calls
Result: PR created as bot account, no passthrough auth
```

**With MCP Priority:**
```
Claude will use: mcp__devmesh__github_pulls tool
Result: PR created as YOU with your GitHub token
```

### Scenario 2: File Operations

**Without MCP Priority:**
```
Claude might use: Built-in Read/Write tools
Result: Limited to local filesystem
```

**With MCP Priority:**
```
Claude will use: MCP file tools if available
Result: Can access remote files, cloud storage, etc.
```

### Scenario 3: Database Queries

**Without MCP Priority:**
```
Claude might use: Raw SQL via Bash
Result: No connection pooling, no query optimization
```

**With MCP Priority:**
```
Claude will use: MCP database tools
Result: Optimized queries, connection pooling, audit trail
```

## Conclusion

With this configuration:
1. Claude Code will ALWAYS check MCP tools first
2. Your global CLAUDE.md enforces this behavior
3. Tool refresh ensures up-to-date capabilities
4. Pass-through auth enables personal attribution
5. Monitoring helps verify correct behavior

This setup works across ALL projects automatically!