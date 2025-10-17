# Edge MCP Integration Troubleshooting Guide

This comprehensive troubleshooting guide covers common issues when integrating with Edge MCP from any client (Claude Code, Cursor, Windsurf, or custom MCP clients).

## Table of Contents

1. [Quick Diagnostics](#quick-diagnostics)
2. [Connection Issues](#connection-issues)
3. [Authentication Problems](#authentication-problems)
4. [Tool Execution Errors](#tool-execution-errors)
5. [Performance Issues](#performance-issues)
6. [Rate Limiting](#rate-limiting)
7. [Protocol Errors](#protocol-errors)
8. [Network and Firewall](#network-and-firewall)
9. [Platform-Specific Issues](#platform-specific-issues)
10. [Advanced Debugging](#advanced-debugging)

## Quick Diagnostics

Before diving into specific issues, run these quick diagnostic checks:

### 1. Verify Edge MCP is Running

```bash
curl http://localhost:8082/health/ready
```

**Expected Response:**
```json
{
  "status": "healthy",
  "uptime_seconds": 3600,
  "components": {
    "tool_registry": "healthy",
    "cache": "healthy",
    "core_platform": "healthy",
    "mcp_handler": "healthy"
  },
  "tools_loaded": 215
}
```

**Unhealthy Response:**
```json
{
  "status": "unhealthy",
  "components": {
    "tool_registry": "unhealthy",
    "error": "no tools loaded"
  }
}
```

### 2. Test WebSocket Connection

```bash
websocat ws://localhost:8082/ws
```

Should connect without immediate disconnect. Type Ctrl+C to exit.

### 3. Check Edge MCP Logs

**Docker:**
```bash
docker-compose logs -f edge-mcp
```

**Kubernetes:**
```bash
kubectl logs -n edge-mcp deployment/edge-mcp -f
```

**Local:**
Check console output where Edge MCP is running.

### 4. Verify API Key

```bash
# Test with curl
curl -H "Authorization: Bearer dev-admin-key-1234567890" \
     http://localhost:8082/health/ready
```

Should return healthy status if API key is valid.

## Connection Issues

### Cannot Connect to WebSocket

**Symptoms:**
- Client shows "Connection refused" or "Cannot connect to ws://localhost:8082/ws"
- WebSocket upgrade fails
- Immediate disconnect after connection

**Diagnosis:**

1. **Check if Edge MCP is listening:**
   ```bash
   lsof -i :8082
   # or
   netstat -an | grep 8082
   ```

2. **Verify WebSocket endpoint:**
   ```bash
   curl -i http://localhost:8082/ws
   ```
   Should return upgrade-related headers or 426 Upgrade Required.

**Solutions:**

1. **Edge MCP not running:**
   ```bash
   # Start Edge MCP
   cd apps/edge-mcp
   go run cmd/server/main.go
   ```

2. **Wrong port:**
   - Default port is 8082
   - Check `EDGE_MCP_PORT` environment variable
   - Update client configuration to match server port

3. **Wrong URL format:**
   - Use `ws://` for HTTP (local development)
   - Use `wss://` for HTTPS (production)
   - Include `/ws` path: `ws://localhost:8082/ws`

4. **Firewall blocking:**
   ```bash
   # On macOS
   sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add /path/to/edge-mcp

   # On Linux (iptables)
   sudo iptables -I INPUT -p tcp --dport 8082 -j ACCEPT
   ```

### Connection Drops Immediately

**Symptoms:**
- Connection established then immediately closes
- "WebSocket closed with code 1008" or similar
- No error message

**Diagnosis:**

Check Edge MCP logs for the reason:
```bash
docker-compose logs edge-mcp | grep -i "websocket\|close\|disconnect"
```

**Common Causes:**

1. **Missing Authentication:**
   ```json
   // Correct
   "headers": {
     "Authorization": "Bearer dev-admin-key-1234567890"
   }

   // Wrong - missing auth header
   "headers": {}
   ```

2. **Invalid API Key Format:**
   - Must match pattern: `^[a-zA-Z0-9_-]+$`
   - Example valid: `dev-admin-key-1234567890`
   - Example invalid: `dev@admin!key` (contains @ and !)

3. **Protocol Version Mismatch:**
   - Always use `protocolVersion: "2025-06-18"` in initialize message

### Connection Drops After Period of Inactivity

**Symptoms:**
- Connection works initially
- Drops after 30-60 seconds of inactivity
- Needs manual reconnection

**Diagnosis:**

1. **Check keepalive settings:**
   - Edge MCP sends ping every 30 seconds
   - Client should respond with pong (or WebSocket library handles)

2. **Check for proxy/firewall timeouts:**
   ```bash
   # Test long-lived connection
   websocat ws://localhost:8082/ws &
   sleep 120
   # Connection should still be alive
   ```

**Solutions:**

1. **Ensure client handles ping/pong:**
   - Most WebSocket libraries handle this automatically
   - If implementing manually, respond to "ping" method with "pong"

2. **Adjust proxy timeouts:**
   - nginx: `proxy_read_timeout 300s;`
   - Apache: `ProxyTimeout 300`
   - AWS ALB: Target group attributes → Timeout 300s

3. **Enable reconnection in client:**
   ```json
   "connectionOptions": {
     "autoReconnect": true,
     "reconnectDelay": 5000,
     "maxReconnectAttempts": 10
   }
   ```

## Authentication Problems

### 401 Unauthorized

**Symptoms:**
- HTTP 401 response when connecting
- "Unauthorized" error message
- Connection rejected immediately

**Diagnosis:**

1. **Check authentication header format:**
   ```bash
   # Test with curl
   curl -H "Authorization: Bearer dev-admin-key-1234567890" \
        http://localhost:8082/health/ready
   ```

2. **Check Edge MCP logs:**
   ```bash
   docker-compose logs edge-mcp | grep -i "auth\|unauthorized"
   ```

**Solutions:**

1. **Missing "Bearer" prefix:**
   ```json
   // Correct
   "Authorization": "Bearer dev-admin-key-1234567890"

   // Wrong
   "Authorization": "dev-admin-key-1234567890"
   ```

2. **Wrong header name:**
   ```json
   // Option 1 (Recommended)
   "Authorization": "Bearer <api-key>"

   // Option 2
   "X-API-Key": "<api-key>"

   // Wrong
   "Api-Key": "<api-key>"
   ```

3. **API key not configured in Edge MCP:**
   ```bash
   # Set API key environment variable
   export EDGE_MCP_API_KEY=dev-admin-key-1234567890

   # Restart Edge MCP
   ```

4. **Special characters in API key:**
   - Use only alphanumeric, hyphen (-), and underscore (_)
   - Generate new key: `openssl rand -hex 16`

### 403 Forbidden

**Symptoms:**
- HTTP 403 response
- "Forbidden" or "Access denied" error
- Connection established but operations fail

**Diagnosis:**

1. **Check if API key has expired (if using Core Platform):**
   ```bash
   curl -H "Authorization: Bearer <api-key>" \
        http://localhost:8081/api/v1/auth/validate
   ```

2. **Check tenant permissions:**
   - Verify tenant has access to requested resources

**Solutions:**

1. **Renew expired API key:**
   - Contact administrator for new API key
   - Update client configuration

2. **Check tool permissions:**
   - Some tools require specific permissions
   - Verify tenant has required scopes

### Passthrough Authentication Fails

**Symptoms:**
- GitHub/Harness tools return authentication errors
- "Invalid credentials" for external services
- Edge MCP authentication works, but tool calls fail

**Diagnosis:**

1. **Verify passthrough credentials are sent:**
   ```bash
   # Check Edge MCP logs for passthrough auth extraction
   docker-compose logs edge-mcp | grep "passthrough"
   ```

2. **Test credentials directly:**
   ```bash
   # Test GitHub token
   curl -H "Authorization: token ghp_yourToken" \
        https://api.github.com/user

   # Test Harness API key
   curl -H "x-api-key: pat.yourKey" \
        https://app.harness.io/gateway/ng/api/version
   ```

**Solutions:**

1. **Add passthrough headers to client config:**
   ```json
   "headers": {
     "Authorization": "Bearer dev-admin-key-1234567890",
     "X-GitHub-Token": "ghp_yourPersonalAccessToken",
     "X-Harness-API-Key": "pat.yourHarnessAPIKey",
     "X-Harness-Account-ID": "your-account-id"
   }
   ```

2. **Verify token scopes:**
   - **GitHub:** Needs `repo`, `read:org`, `read:user` scopes
   - **Harness:** Account-level API key with appropriate permissions

3. **Check token expiration:**
   - GitHub PATs can expire
   - Harness API keys can be rotated/revoked

4. **Use environment variables (for security):**
   ```json
   "headers": {
     "X-GitHub-Token": "${env:GITHUB_TOKEN}"
   }
   ```

## Tool Execution Errors

### Tool Not Found (404)

**Symptoms:**
- Error: "Tool 'tool_name' not found"
- 404 error code
- Tool appears in `tools/list` but fails when called

**Diagnosis:**

1. **List available tools:**
   ```bash
   # Using websocat
   echo '{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}' | \
     websocat ws://localhost:8082/ws
   ```

2. **Check tool name (case-sensitive):**
   - Correct: `github_get_repository`
   - Wrong: `GitHub_Get_Repository` or `github-get-repository`

**Solutions:**

1. **Use exact tool name from `tools/list`:**
   - Tool names are case-sensitive
   - Use underscores, not hyphens or spaces

2. **Check if Core Platform is connected (for dynamic tools):**
   ```bash
   curl http://localhost:8082/health/ready | jq '.components.core_platform'
   ```

3. **Refresh tool registry:**
   - Restart Edge MCP
   - Or wait for automatic refresh (every 5 minutes)

4. **Search for similar tools:**
   ```json
   {
     "jsonrpc": "2.0",
     "id": 10,
     "method": "tools/search",
     "params": {
       "keyword": "repository"
     }
   }
   ```

### Invalid Parameters (400)

**Symptoms:**
- Error: "Invalid parameters"
- 400 Bad Request
- "Missing required parameter" or "Invalid parameter type"

**Diagnosis:**

1. **Check tool schema:**
   ```bash
   # List tools to see inputSchema
   echo '{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}' | \
     websocat ws://localhost:8082/ws | jq '.result.tools[] | select(.name == "github_get_repository")'
   ```

2. **Validate parameters against schema:**
   - Check required parameters
   - Verify parameter types (string, number, boolean, etc.)

**Solutions:**

1. **Include all required parameters:**
   ```json
   // Correct
   {
     "name": "github_get_repository",
     "arguments": {
       "owner": "developer-mesh",
       "repo": "developer-mesh"
     }
   }

   // Wrong - missing required parameter
   {
     "name": "github_get_repository",
     "arguments": {
       "repo": "developer-mesh"
     }
   }
   ```

2. **Use correct parameter types:**
   ```json
   // Correct
   {"limit": 10}

   // Wrong
   {"limit": "10"}  // Should be number, not string
   ```

3. **Check parameter names (case-sensitive):**
   ```json
   // Correct
   {"owner": "developer-mesh"}

   // Wrong
   {"Owner": "developer-mesh"}
   ```

### Tool Execution Timeout

**Symptoms:**
- Operation times out after 2 minutes (default)
- Error: "Request timeout"
- No response received

**Diagnosis:**

1. **Check if tool is long-running:**
   - Some tools (like large file operations) can take time

2. **Check Edge MCP metrics:**
   ```bash
   curl http://localhost:8082/metrics | grep tool_execution_duration
   ```

**Solutions:**

1. **Increase client timeout:**
   ```json
   "connectionOptions": {
     "requestTimeout": 300000  // 5 minutes
   }
   ```

2. **Use batch operations with streaming:**
   - For multiple slow operations, use `tools/batch` with `parallel: true`

3. **Check Core Platform/external service latency:**
   - Tool may be waiting on external service response

## Performance Issues

### Slow Tool Execution

**Symptoms:**
- Tools take longer than expected
- Inconsistent response times
- Higher latency than baseline

**Diagnosis:**

1. **Check cache hit rate:**
   ```bash
   curl http://localhost:8082/metrics | grep cache_hit
   ```

2. **Check tool execution metrics:**
   ```bash
   curl http://localhost:8082/metrics | grep tool_execution_duration
   ```

3. **Enable distributed tracing:**
   ```bash
   export TRACING_ENABLED=true
   export ZIPKIN_ENDPOINT=http://localhost:9411/api/v2/spans
   ```

**Solutions:**

1. **Enable Redis caching:**
   ```bash
   export REDIS_ENABLED=true
   export REDIS_URL=redis://localhost:6379
   ```

2. **Use batch operations:**
   ```json
   {
     "method": "tools/batch",
     "params": {
       "tools": [...],
       "parallel": true
     }
   }
   ```

3. **Check network latency to Core Platform:**
   ```bash
   time curl http://localhost:8081/health
   ```

4. **Review Edge MCP resource usage:**
   ```bash
   docker stats edge-mcp
   ```

### High Memory Usage

**Symptoms:**
- Edge MCP using excessive memory
- Out of memory errors
- Slowdown over time

**Diagnosis:**

1. **Check memory usage:**
   ```bash
   docker stats edge-mcp --no-stream
   ```

2. **Check number of active sessions:**
   ```bash
   curl http://localhost:8082/metrics | grep active_connections
   ```

**Solutions:**

1. **Enable cache size limits:**
   ```bash
   export MEMORY_CACHE_SIZE_MB=100
   ```

2. **Reduce session TTL:**
   ```bash
   export SESSION_TTL=12h  # Default is 24h
   ```

3. **Limit concurrent connections:**
   ```bash
   export MAX_CONNECTIONS=100
   ```

4. **Enable cache eviction:**
   - LRU eviction enabled by default
   - Check cache configuration

## Rate Limiting

### 429 Too Many Requests

**Symptoms:**
- Error code 429
- "Rate limit exceeded" message
- Response includes `retry_after` field

**Diagnosis:**

1. **Check error response for rate limit details:**
   ```json
   {
     "error": {
       "code": 429,
       "message": "Rate limit exceeded",
       "data": {
         "retry_after": 5.2,
         "limit": "100 requests/sec",
         "quota_remaining": 0,
         "quota_reset_at": "2025-01-15T11:00:00Z"
       }
     }
   }
   ```

2. **Check current rate limits:**
   ```bash
   curl http://localhost:8082/metrics | grep rate_limit
   ```

**Solutions:**

1. **Implement exponential backoff:**
   ```python
   import time

   retry_after = error_data.get('retry_after', 1.0)
   time.sleep(retry_after)
   # Retry request
   ```

2. **Increase rate limits (for local development):**
   ```bash
   export EDGE_MCP_GLOBAL_RPS=2000
   export EDGE_MCP_TENANT_RPS=500
   export EDGE_MCP_TOOL_RPS=200
   ```

3. **Use batch operations to reduce request count:**
   - Combine multiple tool calls into one `tools/batch` request

4. **Implement request queuing in client:**
   - Queue requests locally
   - Send at controlled rate

### Quota Exceeded

**Symptoms:**
- Daily/monthly quota exhausted
- Error message includes quota information
- All requests failing with 429

**Diagnosis:**

1. **Check quota status:**
   ```json
   {
     "error": {
       "data": {
         "quota_remaining": 0,
         "quota_limit": 10000,
         "quota_reset_at": "2025-01-16T00:00:00Z"
       }
     }
   }
   ```

**Solutions:**

1. **Wait for quota reset:**
   - Check `quota_reset_at` timestamp
   - Quotas typically reset daily

2. **Request quota increase:**
   - Contact administrator for higher quota
   - Adjust `DEFAULT_QUOTA` environment variable

3. **Optimize tool usage:**
   - Cache results locally
   - Reduce redundant calls
   - Use more specific queries

## Protocol Errors

### Invalid JSON-RPC

**Symptoms:**
- Error: "Invalid request"
- Parse errors
- Protocol violations

**Diagnosis:**

1. **Validate JSON-RPC message format:**
   ```json
   {
     "jsonrpc": "2.0",      // Required, must be "2.0"
     "id": 1,               // Required for requests
     "method": "method",    // Required for requests
     "params": {}           // Optional
   }
   ```

**Solutions:**

1. **Ensure `jsonrpc: "2.0"` is present:**
   ```json
   // Correct
   {"jsonrpc": "2.0", "id": 1, "method": "tools/list"}

   // Wrong - missing jsonrpc
   {"id": 1, "method": "tools/list"}
   ```

2. **Use proper ID format:**
   - Can be string or number
   - Must be unique per request
   - Required for requests (optional for notifications)

3. **Check method name format:**
   - Must match MCP specification
   - Case-sensitive

### Protocol Version Mismatch

**Symptoms:**
- "Protocol version not supported"
- Initialization fails
- Incompatibility errors

**Diagnosis:**

1. **Check initialize message:**
   ```json
   {
     "params": {
       "protocolVersion": "2025-06-18"  // Must match
     }
   }
   ```

**Solutions:**

1. **Use supported protocol version:**
   - Edge MCP supports: `2025-06-18`
   - Update client to use correct version

2. **Check Edge MCP version:**
   ```bash
   curl http://localhost:8082/health/ready | jq '.version'
   ```

## Network and Firewall

### Proxy Issues

**Symptoms:**
- Connection fails in corporate network
- Works without proxy, fails with proxy
- WebSocket upgrade blocked

**Diagnosis:**

1. **Check proxy configuration:**
   ```bash
   echo $HTTP_PROXY
   echo $HTTPS_PROXY
   ```

2. **Test direct connection:**
   ```bash
   curl --noproxy localhost http://localhost:8082/health
   ```

**Solutions:**

1. **Configure proxy bypass:**
   ```bash
   export NO_PROXY=localhost,127.0.0.1
   ```

2. **Use proxy with WebSocket support:**
   - Ensure proxy supports HTTP/1.1 Upgrade header
   - Configure proxy to allow WebSocket (ws://, wss://)

3. **Use VPN or direct connection:**
   - Some corporate proxies block WebSocket
   - Use VPN or direct connection as workaround

### SSL/TLS Issues (wss://)

**Symptoms:**
- "Certificate verification failed"
- "SSL handshake failed"
- Works with ws:// but not wss://

**Diagnosis:**

1. **Test TLS connection:**
   ```bash
   openssl s_client -connect edge-mcp.company.com:443
   ```

2. **Check certificate validity:**
   ```bash
   curl -v https://edge-mcp.company.com/health
   ```

**Solutions:**

1. **Verify certificate is valid:**
   - Not expired
   - Matches domain name
   - Signed by trusted CA

2. **Install certificate in client:**
   - Add custom CA to system trust store
   - Or disable certificate verification (development only)

3. **Use self-signed certificate for development:**
   ```json
   "connectionOptions": {
     "tlsVerify": false  // Development only!
   }
   ```

## Platform-Specific Issues

### Claude Code

**Issue:** Tools not appearing in Claude Code

**Solutions:**
1. Verify `~/.claude/mcp.json` configuration
2. Restart Claude Code completely
3. Check Claude Code logs: View → Output → MCP

**Issue:** Authentication fails

**Solutions:**
1. Use Bearer token format: `Authorization: Bearer <key>`
2. Verify JSON syntax in mcp.json

### Cursor

**Issue:** MCP server not connecting

**Solutions:**
1. Check Settings → MCP configuration
2. Reload window: Cmd+Shift+P → "Reload Window"
3. Check Output panel: View → Output → MCP Client

**Issue:** Environment variables not resolving

**Solutions:**
1. Launch Cursor from terminal with env vars set
2. Or use absolute paths/values instead of ${env:VAR}

### Windsurf

**Issue:** Connection established but no tools

**Solutions:**
1. Verify `~/Library/Application Support/Windsurf/User/mcp.json`
2. Full restart (not just reload)
3. Check tool filters if configured

**Issue:** Authentication using header format

**Solutions:**
Use Windsurf's auth format:
```json
"authentication": {
  "type": "bearer",
  "token": "your-api-key"
}
```

## Advanced Debugging

### Enable Verbose Logging

**Edge MCP:**
```bash
export LOG_LEVEL=debug
go run cmd/server/main.go
```

**Client (example for Go):**
```go
websocket.Dial(ctx, url, &websocket.DialOptions{
    HTTPHeader: header,
    CompressionMode: websocket.CompressionContextTakeover,
})
```

### Capture WebSocket Traffic

**Using websocat with logging:**
```bash
websocat -v ws://localhost:8082/ws 2>&1 | tee websocket.log
```

**Using browser DevTools:**
1. Open browser DevTools
2. Network tab → WS filter
3. Click WebSocket connection
4. View Messages tab

### Distributed Tracing

Enable tracing for end-to-end visibility:

```bash
# Start Jaeger
docker run -d -p 16686:16686 -p 4317:4317 jaegertracing/all-in-one:latest

# Configure Edge MCP
export TRACING_ENABLED=true
export OTLP_ENDPOINT=localhost:4317

# View traces
open http://localhost:16686
```

### Health Check Monitoring

Monitor Edge MCP health continuously:

```bash
# Watch health status
watch -n 5 'curl -s http://localhost:8082/health/ready | jq'

# Check specific component
curl http://localhost:8082/health/ready | jq '.components.tool_registry'
```

### Metrics Analysis

Review Prometheus metrics:

```bash
# Get all metrics
curl http://localhost:8082/metrics

# Filter specific metrics
curl http://localhost:8082/metrics | grep tool_execution

# Monitor rate limits
curl http://localhost:8082/metrics | grep rate_limit
```

## Getting Help

If you're still experiencing issues after trying these solutions:

1. **Check Edge MCP logs** for detailed error messages
2. **Enable debug logging** to capture more information
3. **Collect diagnostic information:**
   - Edge MCP version
   - Client type and version
   - Network configuration
   - Error messages and logs
4. **Open GitHub issue** with diagnostic information

## Related Documentation

- [Claude Code Integration](./claude-code.md)
- [Cursor Integration](./cursor.md)
- [Windsurf Integration](./windsurf.md)
- [Generic MCP Client](./generic-mcp-client.md)
- [OpenAPI Specification](../openapi/edge-mcp.yaml)
- [Kubernetes Deployment](../../deployments/k8s/README.md)
