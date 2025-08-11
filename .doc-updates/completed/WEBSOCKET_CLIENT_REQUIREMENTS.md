<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:43:04
Verification Script: update-docs-parallel.sh
Batch: ad
-->

# WebSocket Client Requirements <!-- Source: pkg/models/websocket/binary.go -->

## Critical Requirements for MCP WebSocket Connections <!-- Source: pkg/models/websocket/binary.go -->

### 1. Subprotocol Requirement

**All WebSocket clients MUST request the `mcp.v1` subprotocol during connection.** <!-- Source: pkg/models/websocket/binary.go -->

Without this subprotocol, the server will reject the connection with:
- **HTTP 426 Upgrade Required** error
- Error message: "Subprotocol required"

### 2. Implementation Examples

#### Go (using coder/websocket) <!-- Source: pkg/models/websocket/binary.go -->
```go
import "github.com/coder/websocket" <!-- Source: pkg/models/websocket/binary.go -->

dialOpts := &websocket.DialOptions{ <!-- Source: pkg/models/websocket/binary.go -->
    Subprotocols: []string{"mcp.v1"},  // REQUIRED
    HTTPHeader: http.Header{
        "Authorization": []string{"Bearer " + apiKey},
    },
}

conn, _, err := websocket.Dial(ctx, wsURL, dialOpts) <!-- Source: pkg/models/websocket/binary.go -->
```

#### JavaScript/TypeScript
```javascript
const ws = new WebSocket('wss://mcp.dev-mesh.io/ws', ['mcp.v1']); <!-- Source: pkg/models/websocket/binary.go -->
ws.setRequestHeader('Authorization', `Bearer ${apiKey}`);
```

#### Python
```python
import websockets <!-- Source: pkg/models/websocket/binary.go -->

headers = {
    "Authorization": f"Bearer {api_key}"
}

async with websockets.connect( <!-- Source: pkg/models/websocket/binary.go -->
    'wss://mcp.dev-mesh.io/ws',
    subprotocols=['mcp.v1'],
    extra_headers=headers
) as websocket: <!-- Source: pkg/models/websocket/binary.go -->
    # Your code here
```

#### cURL (for testing)
```bash
curl -i -N \
    -H "Connection: Upgrade" \
    -H "Upgrade: websocket" \ <!-- Source: pkg/models/websocket/binary.go -->
    -H "Sec-WebSocket-Version: 13" \ <!-- Source: pkg/models/websocket/binary.go -->
    -H "Sec-WebSocket-Key: $(openssl rand -base64 16)" \ <!-- Source: pkg/models/websocket/binary.go -->
    -H "Sec-WebSocket-Protocol: mcp.v1" \ <!-- Source: pkg/models/websocket/binary.go -->
    -H "Authorization: Bearer YOUR_API_KEY" \
    https://mcp.dev-mesh.io/ws
```

### 3. Nginx/Proxy Configuration

If you're proxying WebSocket connections, ensure you forward the protocol headers: <!-- Source: pkg/models/websocket/binary.go -->

```nginx
location /ws {
    proxy_pass http://backend;
    
    # Standard WebSocket headers <!-- Source: pkg/models/websocket/binary.go -->
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    
    # IMPORTANT: Forward the subprotocol header
    proxy_set_header Sec-WebSocket-Protocol $http_sec_websocket_protocol; <!-- Source: pkg/models/websocket/binary.go -->
    proxy_set_header Sec-WebSocket-Version $http_sec_websocket_version; <!-- Source: pkg/models/websocket/binary.go -->
    proxy_set_header Sec-WebSocket-Key $http_sec_websocket_key; <!-- Source: pkg/models/websocket/binary.go -->
    
    # Forward auth
    proxy_set_header Authorization $http_authorization;
}
```

### 4. Troubleshooting

#### Error: 426 Upgrade Required
- **Cause**: Missing `mcp.v1` subprotocol in the request
- **Fix**: Add `Subprotocols: ["mcp.v1"]` to your WebSocket dial options <!-- Source: pkg/models/websocket/binary.go -->

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
Upgrade: websocket <!-- Source: pkg/models/websocket/binary.go -->
Connection: Upgrade
Sec-WebSocket-Accept: <calculated value> <!-- Source: pkg/models/websocket/binary.go -->
Sec-WebSocket-Protocol: mcp.v1 <!-- Source: pkg/models/websocket/binary.go -->
```

### 6. Additional Resources

- [Full WebSocket Protocol Guide](./guides/agent-websocket-protocol.md) <!-- Source: pkg/models/websocket/binary.go -->
- [Binary Protocol Specification](./examples/binary-websocket-protocol.md) <!-- Source: pkg/models/websocket/binary.go -->
