# Testing Pass-Through Authentication

## Quick Test

Run the automated test script:

```bash
cd apps/edge-mcp
./scripts/test-passthrough-auth.sh
```

This script will:
1. Set test environment variables
2. Start Edge MCP with debug logging
3. Check if passthrough tokens are detected
4. Show relevant log entries

## Manual Testing

### 1. Build Edge MCP

```bash
cd apps/edge-mcp
go build -o edge-mcp ./cmd/server
```

### 2. Set Your Personal Tokens

```bash
# Required: DevMesh credentials
export DEV_MESH_URL="https://api.devmesh.io"
export DEV_MESH_API_KEY="devmesh_xxx..."  # Your API key from organization registration
# Note: Tenant ID is automatically determined from your API key

# Your personal tokens
export GITHUB_TOKEN="ghp_your_real_github_token"
export AWS_ACCESS_KEY_ID="your_real_aws_key"
export AWS_SECRET_ACCESS_KEY="your_real_aws_secret"
```

### 3. Run Edge MCP with Debug Logging

```bash
./edge-mcp --port 8082 --log-level debug
```

### 4. Look for These Log Messages

✅ **Successful extraction:**
```
INFO: Extracted passthrough authentication {"credentials_count": 3, "custom_headers": 0}
DEBUG: Found GitHub passthrough token
DEBUG: Found AWS passthrough credentials
```

✅ **Token forwarding (when executing tools):**
```
DEBUG: Added passthrough auth to tool execution context {"tool": "github.create_pr", "has_passthrough": true}
DEBUG: Including passthrough auth in tool execution {"tool": "github.create_pr", "credential_count": 3}
```

❌ **No tokens detected:**
```
# No "Extracted passthrough authentication" message appears
```

## Testing with IDE

### Claude Code Test

1. Create `.claude/mcp.json`:
```json
{
  "mcpServers": {
    "edge-mcp-test": {
      "command": "./edge-mcp",
      "args": ["--port", "8082", "--log-level", "debug"],
      "env": {
        "DEV_MESH_URL": "${DEV_MESH_URL}",
        "DEV_MESH_API_KEY": "${DEV_MESH_API_KEY}",
        "GITHUB_TOKEN": "${GITHUB_TOKEN}"
      }
    }
  }
}
```

2. Start Claude Code and connect
3. Execute a GitHub tool
4. Check the created PR/issue shows YOUR username

### Verification Steps

1. **GitHub**: Created PRs/issues should show your username
2. **AWS**: Resources should be created in your account
3. **Slack**: Messages should show as sent by you
4. **Audit Logs**: DevMesh dashboard should show your identity

## Common Issues

### Tokens Not Detected

**Symptom**: No "Extracted passthrough authentication" in logs

**Solutions**:
- Verify environment variables are exported: `echo $GITHUB_TOKEN`
- Restart Edge MCP after setting variables
- Check token format (GitHub tokens start with `ghp_`)

### Tokens Not Forwarded

**Symptom**: Actions still performed as service account

**Solutions**:
- Check "Added passthrough auth to tool execution context" appears
- Verify Core Platform connection is active
- Check tool supports passthrough (some may not)

### Authentication Failures

**Symptom**: "401 Unauthorized" or "403 Forbidden" errors

**Solutions**:
- Verify token is valid and not expired
- Check token has required scopes/permissions
- Test token directly with service API

## Debug Commands

```bash
# Check if Edge MCP sees your tokens
./edge-mcp --port 8082 --log-level debug 2>&1 | grep -E "(passthrough|token|credential)"

# Test WebSocket connection with headers
curl -i -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "X-GitHub-Token: ghp_test123" \
  http://localhost:8082/ws

# Monitor Core Platform requests
./edge-mcp --port 8082 --log-level debug 2>&1 | grep -E "(Including passthrough|POST /api/v1/tools/execute)"
```

## Expected Behavior

### With Pass-Through Auth

```json
// Tool execution includes passthrough
{
  "tool": "github.create_pr",
  "arguments": {...},
  "passthrough_auth": {
    "credentials": {
      "github": {
        "type": "bearer",
        "token": "ghp_..."
      }
    }
  }
}
```

Result: PR created by YOUR GitHub account

### Without Pass-Through Auth

```json
// Tool execution without passthrough
{
  "tool": "github.create_pr",
  "arguments": {...}
}
```

Result: PR created by DevMesh service account

## Security Testing

### Token Masking

Verify tokens are masked in logs:
- Full tokens should NEVER appear in logs
- Only "Found X token" messages should appear
- Token values should be masked as `***`

### Memory Only

Verify tokens are not persisted:
1. Stop Edge MCP
2. Check no token values in any files
3. Restart Edge MCP - tokens should be gone

### Session Isolation

Test with multiple connections:
1. Connect with Token A
2. Connect with Token B
3. Verify each session uses its own tokens