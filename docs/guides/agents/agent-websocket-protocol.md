<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:47:33
Verification Script: update-docs-parallel.sh
Batch: ab
-->

# Agent WebSocket Protocol Guide <!-- Source: pkg/models/websocket/binary.go -->

> **Purpose**: Comprehensive specification of the WebSocket protocol for AI agent communication <!-- Source: pkg/models/websocket/binary.go -->
> **Audience**: Developers implementing agent communication layers and protocol handlers
> **Scope**: Binary protocol, message formats, connection lifecycle, performance optimization <!-- Source: pkg/models/websocket/binary.go -->

## Overview

The Developer Mesh platform uses a high-performance WebSocket protocol for real-time communication between AI agents and the orchestration server. The protocol supports both JSON and binary formats, with automatic compression and efficient message routing. <!-- Source: pkg/models/websocket/binary.go -->

## Protocol Architecture

### Protocol Stack

```
┌────────────────────────────────────────────────────────────┐
│                    Application Layer                        │
│                 (Agent Business Logic)                      │
├────────────────────────────────────────────────────────────┤
│                    Message Layer                            │
│              (Serialization, Compression)                   │
├────────────────────────────────────────────────────────────┤
│                    Protocol Layer                           │
│           (Binary Header, Routing, Sequencing)              │
├────────────────────────────────────────────────────────────┤
│                    Transport Layer                          │
│                (WebSocket, TLS 1.3)                        │ <!-- Source: pkg/models/websocket/binary.go -->
└────────────────────────────────────────────────────────────┘
```

## Binary Protocol Specification <!-- Source: pkg/models/websocket/binary.go -->

### Header Format (24 bytes)

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                      Magic Number (0x4D435057)                |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|    Version    |     Type      |            Flags              |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                         Sequence ID                           |
|                           (64 bits)                           |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|            Method             |           Reserved            |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                         Data Size                             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### Field Descriptions

| Field | Size | Description |
|-------|------|-------------|
| Magic Number | 4 bytes | 0x4D435057 ("MCPW" in ASCII) |
| Version | 1 byte | Protocol version (currently 1) |
| Type | 1 byte | Message type (see Message Types) |
| Flags | 2 bytes | Control flags (compression, encryption, etc.) |
| Sequence ID | 8 bytes | Unique message sequence number |
| Method | 2 bytes | Method enumeration (see Methods) |
| Reserved | 2 bytes | Reserved for future use (must be 0) |
| Data Size | 4 bytes | Size of payload in bytes |

### Message Types

```go
const (
    MessageTypeRequest      = 0x00  // Client request
    MessageTypeResponse     = 0x01  // Server response
    MessageTypeNotification = 0x02  // Async notification
    MessageTypeError        = 0x03  // Error message
    MessageTypePing         = 0x04  // Keepalive ping
    MessageTypePong         = 0x05  // Keepalive pong
    MessageTypeBatch        = 0x06  // Batch messages
    MessageTypeProgress     = 0x07  // Progress update
)
```

### Control Flags

```go
const (
    FlagCompressed = 1 << 0  // Payload is compressed
    FlagEncrypted  = 1 << 1  // Payload is encrypted
    FlagBatch      = 1 << 2  // Contains multiple messages
    FlagPriority   = 1 << 3  // High priority message
    FlagStreaming  = 1 << 4  // Part of streaming response
)
```

## Agent Protocol Messages

### 1. Agent Registration

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": 0,
  "method": "agent.register",
  "params": {
    "agent_id": "agent-123",
    "agent_type": "code-analyzer",
    "capabilities": [
      {
        "name": "code_review",
        "confidence": 0.95,
        "languages": ["go", "python", "javascript"],
        "specialties": ["security", "performance"]
      },
      {
        "name": "bug_detection",
        "confidence": 0.88,
        "patterns": ["memory-leak", "race-condition", "sql-injection"]
      }
    ],
    "model": {
      "provider": "openai",
      "name": "gpt-4",
      "version": "1106-preview"
    },
    "resources": {
      "max_concurrent_tasks": 10,
      "max_memory_mb": 4096,
      "gpu_required": false
    },
    "metadata": {
      "version": "1.2.0",
      "deployment": "production",
      "region": "us-east-1"
    }
  }
}
```

### 2. Heartbeat

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440001",
  "type": 0,
  "method": "agent.heartbeat",
  "params": {
    "agent_id": "agent-123",
    "status": "healthy",
    "metrics": {
      "active_tasks": 3,
      "completed_tasks": 147,
      "failed_tasks": 2,
      "avg_task_duration_ms": 2340,
      "cpu_usage_percent": 45.2,
      "memory_usage_mb": 1823,
      "uptime_seconds": 86400
    },
    "workload": {
      "current_load": 0.6,
      "queue_depth": 2,
      "processing_rate": 0.5
    }
  }
}
```

### 3. Task Assignment

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440002",
  "type": 2,
  "method": "task.assigned",
  "params": {
    "task_id": "task-456",
    "type": "code_review",
    "priority": 2,
    "deadline": "2024-12-25T15:30:00Z",
    "context": {
      "repository": "github.com/org/repo",
      "pull_request": 123,
      "files": ["src/main.go", "src/handler.go"],
      "commit": "a1b2c3d4"
    },
    "requirements": {
      "focus_areas": ["security", "performance"],
      "severity_threshold": "medium",
      "include_suggestions": true
    },
    "routing_metadata": {
      "assigned_by": "capability-router",
      "score": 0.92,
      "alternative_agents": ["agent-456", "agent-789"]
    }
  }
}
```

### 4. Task Progress

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440003",
  "type": 7,
  "method": "task.progress",
  "params": {
    "task_id": "task-456",
    "agent_id": "agent-123",
    "progress": 0.65,
    "status": "analyzing",
    "current_step": "security-scan",
    "steps_completed": ["syntax-check", "dependency-analysis"],
    "estimated_completion": "2024-12-25T15:28:00Z",
    "intermediate_results": {
      "issues_found": 3,
      "files_analyzed": 15,
      "patterns_detected": ["sql-injection-risk", "weak-encryption"]
    }
  }
}
```

### 5. Task Completion

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440004",
  "type": 0,
  "method": "task.complete",
  "params": {
    "task_id": "task-456",
    "agent_id": "agent-123",
    "status": "completed",
    "duration_ms": 2150,
    "result": {
      "summary": "Found 3 security issues and 2 performance concerns",
      "findings": [
        {
          "type": "security",
          "severity": "high",
          "file": "src/handler.go",
          "line": 45,
          "description": "Potential SQL injection vulnerability",
          "suggestion": "Use parameterized queries"
        }
      ],
      "metrics": {
        "total_lines": 1523,
        "issues_by_severity": {
          "high": 1,
          "medium": 2,
          "low": 2
        }
      }
    },
    "resource_usage": {
      "cpu_seconds": 2.1,
      "memory_peak_mb": 256,
      "tokens_used": 4523
    }
  }
}
```

### 6. Capability Update

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440005",
  "type": 0,
  "method": "agent.capability.update",
  "params": {
    "agent_id": "agent-123",
    "operation": "add",
    "capability": {
      "name": "rust_analysis",
      "confidence": 0.75,
      "languages": ["rust"],
      "specialties": ["memory-safety", "concurrency"],
      "learning_status": "training",
      "training_progress": 0.82
    },
    "reason": "Model fine-tuning completed for Rust"
  }
}
```

### 7. Collaboration Request

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440006",
  "type": 0,
  "method": "collaboration.request",
  "params": {
    "task_id": "task-789",
    "requesting_agent": "agent-123",
    "collaboration_type": "consensus",
    "required_agents": ["agent-456", "agent-789"],
    "coordination_pattern": "voting",
    "timeout_ms": 30000,
    "context": {
      "decision": "deployment-approval",
      "artifacts": ["deployment-plan.yaml", "risk-assessment.md"],
      "criteria": {
        "security_score": ">= 0.9",
        "performance_impact": "< 5%",
        "rollback_ready": true
      }
    }
  }
}
```

### 8. State Synchronization (CRDT)

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440007",
  "type": 2,
  "method": "state.sync",
  "params": {
    "agent_id": "agent-123",
    "vector_clock": {
      "agent-123": 42,
      "agent-456": 38,
      "agent-789": 40
    },
    "operations": [
      {
        "type": "g-counter",
        "key": "tasks_completed",
        "increment": 1,
        "actor": "agent-123"
      },
      {
        "type": "or-set",
        "key": "active_collaborations",
        "operation": "add",
        "element": "collab-xyz",
        "unique_id": "550e8400-e29b-41d4-a716-446655440008"
      }
    ],
    "merkle_root": "a1b2c3d4e5f6789012345678901234567890abcd"
  }
}
```

## Connection Lifecycle

### 1. Connection Establishment

```sequence
Agent -> Server: WebSocket Upgrade Request (Sec-WebSocket-Protocol: mcp.v1) <!-- Source: pkg/models/websocket/binary.go -->
Server -> Agent: 101 Switching Protocols (Sec-WebSocket-Protocol: mcp.v1) <!-- Source: pkg/models/websocket/binary.go -->
Agent -> Server: agent.register
Server -> Agent: registration.confirmed
Agent -> Server: agent.heartbeat (periodic)
```

**IMPORTANT**: All WebSocket clients MUST request the `mcp.v1` subprotocol during the handshake. Connections without this subprotocol will be rejected with a 426 Upgrade Required error. <!-- Source: pkg/models/websocket/binary.go -->

### 2. Authentication Flow

```go
// WebSocket dial options with required subprotocol <!-- Source: pkg/models/websocket/binary.go -->
dialOpts := &websocket.DialOptions{ <!-- Source: pkg/models/websocket/binary.go -->
    Subprotocols: []string{"mcp.v1"}, // REQUIRED
    HTTPHeader: http.Header{
        "Authorization": []string{"Bearer " + apiKey},
        "X-Agent-ID":    []string{agentID},
        "X-Agent-Type":  []string{agentType},
        "X-Protocol":    []string{"mcpw-binary-v1"},
    },
}

// TLS configuration
tlsConfig := &tls.Config{
    MinVersion:               tls.VersionTLS13,
    PreferServerCipherSuites: true,
    CipherSuites: []uint16{
        tls.TLS_AES_256_GCM_SHA384,
        tls.TLS_CHACHA20_POLY1305_SHA256,
    },
}
```

### 3. Graceful Shutdown

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440009",
  "type": 0,
  "method": "agent.shutdown",
  "params": {
    "agent_id": "agent-123",
    "reason": "scheduled_maintenance",
    "graceful_timeout_ms": 30000,
    "transfer_tasks": true,
    "final_state": {
      "completed_tasks": 1523,
      "active_tasks": [],
      "pending_transfers": ["task-890", "task-891"]
    }
  }
}
```

## Binary Protocol Implementation <!-- Source: pkg/models/websocket/binary.go -->

### Encoding Example

```go
func EncodeMessage(msg *Message) ([]byte, error) {
    // Create header
    header := &BinaryHeader{
        Magic:      MagicNumber,
        Version:    1,
        Type:       msg.Type.ToByte(),
        Flags:      0,
        SequenceID: msg.SequenceID,
        Method:     StringToMethod(msg.Method),
    }
    
    // Serialize payload
    payload, err := json.Marshal(msg.Params)
    if err != nil {
        return nil, err
    }
    
    // Compress if beneficial
    if len(payload) > 1024 {
        compressed := compress(payload)
        if len(compressed) < len(payload) {
            payload = compressed
            header.SetFlag(FlagCompressed)
        }
    }
    
    header.DataSize = uint32(len(payload))
    
    // Write to buffer
    buf := new(bytes.Buffer)
    if err := WriteBinaryHeader(buf, header); err != nil {
        return nil, err
    }
    
    if _, err := buf.Write(payload); err != nil {
        return nil, err
    }
    
    return buf.Bytes(), nil
}
```

### Decoding Example

```go
func DecodeMessage(data []byte) (*Message, error) {
    reader := bytes.NewReader(data)
    
    // Parse header
    header, err := ParseBinaryHeader(reader)
    if err != nil {
        return nil, err
    }
    
    // Read payload
    payload := make([]byte, header.DataSize)
    if _, err := io.ReadFull(reader, payload); err != nil {
        return nil, err
    }
    
    // Decompress if needed
    if header.IsCompressed() {
        payload, err = decompress(payload)
        if err != nil {
            return nil, err
        }
    }
    
    // Create message
    msg := &Message{
        Type:       MessageTypeFromByte(header.Type),
        Method:     MethodToString(header.Method),
        SequenceID: header.SequenceID,
    }
    
    // Deserialize params based on method
    if err := json.Unmarshal(payload, &msg.Params); err != nil {
        return nil, err
    }
    
    return msg, nil
}
```

## Performance Optimization

### 1. Message Batching

```go
type BatchMessage struct {
    Messages []Message `json:"messages"`
    Count    int       `json:"count"`
}

func BatchMessages(messages []Message) *Message {
    return &Message{
        Type:   MessageTypeBatch,
        Method: "batch",
        Params: BatchMessage{
            Messages: messages,
            Count:    len(messages),
        },
    }
}
```

### 2. Compression Strategy

```go
// Adaptive compression based on payload characteristics
func ShouldCompress(payload []byte) bool {
    // Don't compress small payloads
    if len(payload) < 1024 {
        return false
    }
    
    // Estimate compression ratio
    sample := payload[:min(len(payload), 1024)]
    entropy := calculateEntropy(sample)
    
    // High entropy = less compressible
    return entropy < 6.5
}
```

### 3. Connection Pooling

```go
type ConnectionPool struct {
    connections sync.Map
    maxPerAgent int
    mu          sync.RWMutex
}

func (p *ConnectionPool) GetConnection(agentID string) (*Connection, error) {
    // Try to reuse existing connection
    if conn, ok := p.connections.Load(agentID); ok {
        if c := conn.(*Connection); c.IsActive() {
            return c, nil
        }
    }
    
    // Create new connection
    return p.createConnection(agentID)
}
```

## Error Handling

### Error Codes

| Code | Name | Description |
|------|------|-------------|
| 4000 | Invalid Message | Malformed message structure |
| 4001 | Authentication Failed | Invalid credentials or token |
| 4002 | Rate Limited | Too many requests |
| 4003 | Server Error | Internal server error |
| 4004 | Method Not Found | Unknown method |
| 4005 | Invalid Params | Invalid method parameters |
| 4006 | Operation Cancelled | Task or operation cancelled |
| 4007 | Context Too Large | Message exceeds size limit |
| 4008 | Conflict | State conflict detected |

### Error Response Format

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440010",
  "type": 3,
  "error": {
    "code": 4005,
    "message": "Invalid parameters for method 'task.complete'",
    "data": {
      "field": "result.findings",
      "reason": "Required field missing",
      "expected": "array",
      "received": "null"
    }
  }
}
```

## Security Considerations

### 1. Message Authentication

```go
// HMAC-based message authentication
func SignMessage(msg []byte, secret []byte) []byte {
    h := hmac.New(sha256.New, secret)
    h.Write(msg)
    return h.Sum(nil)
}

func VerifyMessage(msg []byte, signature []byte, secret []byte) bool {
    expected := SignMessage(msg, secret)
    return hmac.Equal(signature, expected)
}
```

### 2. Rate Limiting

```go
type RateLimiter struct {
    limits map[string]*rate.Limiter
    mu     sync.RWMutex
}

func (r *RateLimiter) Allow(agentID string) bool {
    r.mu.RLock()
    limiter, exists := r.limits[agentID]
    r.mu.RUnlock()
    
    if !exists {
        r.mu.Lock()
        limiter = rate.NewLimiter(rate.Every(100*time.Millisecond), 100)
        r.limits[agentID] = limiter
        r.mu.Unlock()
    }
    
    return limiter.Allow()
}
```

### 3. Connection Security

- **TLS 1.3**: All connections must use TLS 1.3 or higher
- **Certificate Pinning**: Optional certificate pinning for high-security deployments
- **IP Allowlisting**: Restrict connections to known agent IPs
- **Token Rotation**: Support for rotating authentication tokens

## Monitoring and Metrics

### Connection Metrics

```go
var (
    wsConnections = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "websocket_connections_active", <!-- Source: pkg/models/websocket/binary.go -->
            Help: "Active WebSocket connections", <!-- Source: pkg/models/websocket/binary.go -->
        },
        []string{"agent_type", "protocol"},
    )
    
    wsMessages = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "websocket_messages_total", <!-- Source: pkg/models/websocket/binary.go -->
            Help: "Total WebSocket messages", <!-- Source: pkg/models/websocket/binary.go -->
        },
        []string{"type", "method", "status"},
    )
    
    wsLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "websocket_message_latency_seconds", <!-- Source: pkg/models/websocket/binary.go -->
            Help: "Message processing latency",
            Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
        },
        []string{"method"},
    )
)
```

### Protocol Analytics

```go
type ProtocolAnalytics struct {
    MessageCounts    map[string]int64
    CompressionRatio float64
    ErrorRates       map[int]float64
    AvgMessageSize   int64
    PeakConnections  int
}

func (a *ProtocolAnalytics) RecordMessage(msg *Message, compressed bool, size int) {
    a.MessageCounts[msg.Method]++
    a.updateCompressionRatio(compressed, size)
    a.updateAvgSize(size)
}
```

## Best Practices

### 1. Connection Management

- Implement exponential backoff for reconnection
- Use connection pooling for multiple agents
- Monitor connection health with heartbeats
- Gracefully handle network interruptions

### 2. Message Design

- Keep messages focused and single-purpose
- Use appropriate message types
- Include idempotency keys for critical operations
- Version your message schemas

### 3. Performance

- Batch small messages when possible
- Use binary protocol for high-frequency communication <!-- Source: pkg/models/websocket/binary.go -->
- Implement client-side caching
- Monitor and optimize compression ratios

### 4. Error Handling

- Implement retry logic with backoff
- Log all errors with context
- Use structured error responses
- Monitor error rates and patterns

## Troubleshooting

### Common Issues

1. **Connection Drops**
   ```bash
   # Check WebSocket timeout settings <!-- Source: pkg/models/websocket/binary.go -->
   # Verify keepalive configuration
   # Monitor network stability
   ```

2. **High Latency**
   ```bash
   # Enable message timing logs
   # Check compression overhead
   # Monitor server processing time
   ```

3. **Message Loss**
   ```bash
   # Verify sequence numbers
   # Check for network issues
   # Enable message acknowledgments
   ```

4. **Protocol Errors**
   ```bash
   # Validate message format
   # Check protocol version compatibility
   # Review error logs
   ```

## Protocol Evolution

### Version Negotiation

```go
// Client announces supported versions
supportedVersions := []uint8{1, 2}

// Server selects highest mutual version
selectedVersion := negotiateVersion(supportedVersions)
```

### Backward Compatibility

- New fields must be optional
- Deprecated fields marked but not removed
- Version-specific handlers for migration
- Clear upgrade paths documented

## Next Steps

1. Review [Agent Integration Complete Guide](./ai-agent-integration-complete.md)
2. Explore [Building Custom AI Agents](./building-custom-ai-agents.md)
3. See [Agent Integration Examples](./agent-integration-examples.md)
4. Check [Agent Integration Troubleshooting](./agent-integration-troubleshooting.md)

## Resources

- [WebSocket RFC 6455](https://tools.ietf.org/html/rfc6455) <!-- Source: pkg/models/websocket/binary.go -->
- [Protocol Buffers Documentation](https://developers.google.com/protocol-buffers)
- [CRDT Specifications](https://crdt.tech/)
