# WebSocket Client Requirements

## Critical Requirements for MCP WebSocket Connections

### 1. Subprotocol Requirement

**All WebSocket clients MUST request the `mcp.v1` subprotocol during connection.**

Without this subprotocol, the server will reject the connection with:
- **HTTP 426 Upgrade Required** error
- Error message: "Subprotocol required"

### 2. Implementation Examples

#### Go (using coder/websocket)
```go
import "github.com/coder/websocket"

dialOpts := &websocket.DialOptions{
    Subprotocols: []string{"mcp.v1"},  // REQUIRED
    HTTPHeader: http.Header{
        "Authorization": []string{"Bearer " + apiKey},
    },
}

conn, _, err := websocket.Dial(ctx, wsURL, dialOpts)
```

#### JavaScript/TypeScript
```javascript
const ws = new WebSocket('wss://mcp.dev-mesh.io/ws', ['mcp.v1']);
ws.setRequestHeader('Authorization', `Bearer ${apiKey}`);
```

#### Python
```python
import websockets

headers = {
    "Authorization": f"Bearer {api_key}"
}

async with websockets.connect(
    'wss://mcp.dev-mesh.io/ws',
    subprotocols=['mcp.v1'],
    extra_headers=headers
) as websocket:
    # Your code here
```

#### cURL (for testing)
```bash
curl -i -N \
    -H "Connection: Upgrade" \
    -H "Upgrade: websocket" \
    -H "Sec-WebSocket-Version: 13" \
    -H "Sec-WebSocket-Key: $(openssl rand -base64 16)" \
    -H "Sec-WebSocket-Protocol: mcp.v1" \
    -H "Authorization: Bearer YOUR_API_KEY" \
    https://mcp.dev-mesh.io/ws
```

### 3. Nginx/Proxy Configuration

If you're proxying WebSocket connections, ensure you forward the protocol headers:

```nginx
location /ws {
    proxy_pass http://backend;
    
    # Standard WebSocket headers
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    
    # IMPORTANT: Forward the subprotocol header
    proxy_set_header Sec-WebSocket-Protocol $http_sec_websocket_protocol;
    proxy_set_header Sec-WebSocket-Version $http_sec_websocket_version;
    proxy_set_header Sec-WebSocket-Key $http_sec_websocket_key;
    
    # Forward auth
    proxy_set_header Authorization $http_authorization;
}
```

### 4. Troubleshooting

#### Error: 426 Upgrade Required
- **Cause**: Missing `mcp.v1` subprotocol in the request
- **Fix**: Add `Subprotocols: ["mcp.v1"]` to your WebSocket dial options

#### Error: 401 Unauthorized
- **Cause**: Missing or invalid authentication
- **Fix**: Ensure you're sending `Authorization: Bearer <token>` header

#### Error: Connection drops immediately
- **Cause**: Protocol mismatch or missing headers
- **Fix**: Verify all required headers are being sent

### 5. Server Response

When successful, the server will respond with:
```
HTTP/1.1 101 Switching Protocols
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Accept: <calculated value>
Sec-WebSocket-Protocol: mcp.v1
```

### 6. Additional Resources

- [Full WebSocket Protocol Guide](./guides/agent-websocket-protocol.md)
- [Binary Protocol Specification](./examples/binary-websocket-protocol.md)
- [E2E Test Examples](../test/e2e/README.md)