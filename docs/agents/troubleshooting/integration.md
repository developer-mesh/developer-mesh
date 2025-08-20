<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:44:16
Verification Script: update-docs-parallel.sh
Batch: ab
-->

# Agent Integration Troubleshooting Guide

> **Purpose**: Comprehensive troubleshooting guide for common AI agent integration issues
> **Audience**: Developers and operators debugging agent integration problems
> **Scope**: Common errors, diagnostic tools, debugging techniques, and solutions

## Table of Contents

1. [Quick Diagnostics](#quick-diagnostics)
2. [Connection Issues](#connection-issues)
3. [Authentication Problems](#authentication-problems)
4. [Registration Failures](#registration-failures)
5. [Task Processing Errors](#task-processing-errors)
6. [Performance Issues](#performance-issues)
7. [State Synchronization Problems](#state-synchronization-problems)
8. [Binary Protocol Issues](#binary-protocol-issues) <!-- Source: pkg/models/websocket/binary.go -->
9. [Model Integration Errors](#model-integration-errors)
10. [Debugging Tools](#debugging-tools)
11. [Common Error Codes](#common-error-codes)
12. [Advanced Troubleshooting](#advanced-troubleshooting)

## Quick Diagnostics

### Agent Health Check Script

```bash
#!/bin/bash
# Quick agent diagnostics script

echo "=== Agent Integration Diagnostics ==="
echo

# Check MCP server connectivity
echo "1. Checking MCP server connectivity..."
if curl -s -o /dev/null -w "%{http_code}" https://mcp.example.com/health | grep -q "200"; then
    echo "✓ MCP server is reachable"
else
    echo "✗ Cannot reach MCP server"
fi

# Check WebSocket endpoint <!-- Source: pkg/models/websocket/binary.go -->
echo "2. Checking WebSocket endpoint..." <!-- Source: pkg/models/websocket/binary.go -->
if wscat -c wss://mcp.example.com/ws 2>&1 | grep -q "Connected"; then
    echo "✓ WebSocket endpoint is accessible" <!-- Source: pkg/models/websocket/binary.go -->
else
    echo "✗ WebSocket connection failed" <!-- Source: pkg/models/websocket/binary.go -->
fi

# Check API key validity
echo "3. Checking API authentication..."
if curl -H "Authorization: Bearer $MCP_API_KEY" https://mcp.example.com/api/v1/agents 2>&1 | grep -q "200"; then
    echo "✓ API key is valid"
else
    echo "✗ API authentication failed"
fi

# Check agent process
echo "4. Checking agent process..."
if pgrep -f "mcp-agent" > /dev/null; then
    echo "✓ Agent process is running"
else
    echo "✗ Agent process not found"
fi

# Check logs for errors
echo "5. Checking recent errors..."
ERROR_COUNT=$(tail -n 1000 /var/log/mcp-agent/agent.log | grep -c "ERROR")
echo "Found $ERROR_COUNT errors in last 1000 log lines"
```

### Common Symptoms Checklist

| Symptom | Possible Causes | Quick Fix |
|---------|----------------|-----------|
| Agent won't start | Invalid config, missing dependencies | Check config validation |
| Connection drops frequently | Network issues, timeout settings | Increase timeouts |
| Tasks not received | Registration failed, wrong capabilities | Verify registration |
| High latency | Network, overloaded agent | Check metrics |
| Memory leaks | Task accumulation, no cleanup | Enable GC logging |

## Connection Issues

### Problem: WebSocket Connection Fails <!-- Source: pkg/models/websocket/binary.go -->

**Symptoms:**
- `websocket: bad handshake` error <!-- Source: pkg/models/websocket/binary.go -->
- `connection refused` errors
- Immediate disconnection after connect

**Diagnosis:**
```go
// Enable connection debugging
conn, _, err := websocket.DefaultDialer.Dial(serverURL, headers) <!-- Source: pkg/models/websocket/binary.go -->
if err != nil {
    log.Printf("Connection error: %v", err)
    
    // Check specific error types
    if websocket.IsCloseError(err, websocket.CloseNormalClosure) { <!-- Source: pkg/models/websocket/binary.go -->
        log.Println("Server closed connection normally")
    } else if websocket.IsUnexpectedCloseError(err) { <!-- Source: pkg/models/websocket/binary.go -->
        log.Println("Unexpected connection close")
    }
}
```

**Solutions:**

1. **Check URL format:**
```go
// Correct formats
serverURL := "wss://mcp.example.com/ws"  // Production
serverURL := "ws://localhost:8080/ws"    // Local development

// Common mistakes
serverURL := "https://..."  // Wrong protocol
serverURL := "wss://.../"    // Missing /ws path
```

2. **Verify TLS certificates:**
```go
// For self-signed certificates in development
dialer := websocket.Dialer{ <!-- Source: pkg/models/websocket/binary.go -->
    TLSClientConfig: &tls.Config{
        InsecureSkipVerify: true, // Development only!
    },
}
```

3. **Check network connectivity:**
```bash
# Test WebSocket connectivity <!-- Source: pkg/models/websocket/binary.go -->
wscat -c wss://mcp.example.com/ws -H "Authorization: Bearer $API_KEY"

# Check DNS resolution
nslookup mcp.example.com

# Test port accessibility
nc -zv mcp.example.com 443
```

### Problem: Frequent Disconnections

**Symptoms:**
- Connection drops every few minutes
- `ping timeout` errors
- Reconnection loops

**Solutions:**

1. **Implement proper keepalive:**
```go
func (a *Agent) startHeartbeat(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            if err := a.sendPing(); err != nil {
                log.Printf("Ping failed: %v", err)
                a.reconnect()
                return
            }
        case <-ctx.Done():
            return
        }
    }
}
```

2. **Configure connection timeouts:**
```go
dialer := websocket.Dialer{ <!-- Source: pkg/models/websocket/binary.go -->
    HandshakeTimeout: 45 * time.Second,
    // Enable keepalive at TCP level
    NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
        d := net.Dialer{
            KeepAlive: 30 * time.Second,
        }
        return d.DialContext(ctx, network, addr)
    },
}
```

3. **Implement exponential backoff reconnection:**
```go
func (a *Agent) reconnectWithBackoff() {
    backoff := time.Second
    maxBackoff := 5 * time.Minute
    
    for {
        if err := a.connect(); err == nil {
            log.Println("Reconnected successfully")
            return
        }
        
        log.Printf("Reconnection failed, waiting %v", backoff)
        time.Sleep(backoff)
        
        backoff *= 2
        if backoff > maxBackoff {
            backoff = maxBackoff
        }
    }
}
```

## Authentication Problems

### Problem: API Key Rejected

**Symptoms:**
- `401 Unauthorized` errors
- `invalid api key` messages
- Registration immediately fails

**Diagnosis:**
```bash
# Test API key
curl -H "Authorization: Bearer $MCP_API_KEY" \
     https://mcp.example.com/api/v1/agents

# Check key format
echo $MCP_API_KEY | base64 -d  # If base64 encoded
```

**Solutions:**

1. **Verify API key format:**
```go
// Check for common issues
apiKey := strings.TrimSpace(os.Getenv("MCP_API_KEY"))
if apiKey == "" {
    log.Fatal("MCP_API_KEY environment variable not set")
}

// Remove any accidental quotes
apiKey = strings.Trim(apiKey, "\"'")

// Validate format (example: UUID format)
if _, err := uuid.Parse(apiKey); err != nil {
    log.Fatalf("Invalid API key format: %v", err)
}
```

2. **Set correct headers:**
```go
headers := http.Header{
    "Authorization": []string{fmt.Sprintf("Bearer %s", apiKey)},
    "X-Agent-ID":    []string{agentID},
    "X-Agent-Type":  []string{agentType},
}
```

3. **Handle token rotation:**
```go
func (a *Agent) refreshToken() error {
    // Request new token
    newToken, err := a.requestNewToken()
    if err != nil {
        return err
    }
    
    // Update connection headers
    a.conn.SetHeader("Authorization", fmt.Sprintf("Bearer %s", newToken))
    
    // Store for future use
    a.apiKey = newToken
    
    return nil
}
```

## Registration Failures

### Problem: Agent Registration Rejected

**Symptoms:**
- `registration failed` errors
- Agent not appearing in dashboard
- No tasks received after connection

**Diagnosis:**
```go
// Add registration debugging
func (a *Agent) register() error {
    msg := RegistrationMessage{
        AgentID:      a.ID,
        AgentType:    a.Type,
        Capabilities: a.Capabilities,
    }
    
    log.Printf("Registering agent: %+v", msg)
    
    if err := a.sendMessage(msg); err != nil {
        return fmt.Errorf("send registration failed: %w", err)
    }
    
    // Wait for confirmation
    timeout := time.NewTimer(10 * time.Second)
    defer timeout.Stop()
    
    select {
    case resp := <-a.responseChan:
        if resp.Error != nil {
            return fmt.Errorf("registration rejected: %s", resp.Error.Message)
        }
        log.Println("Registration confirmed")
        return nil
    case <-timeout.C:
        return errors.New("registration timeout")
    }
}
```

**Solutions:**

1. **Validate capabilities:**
```go
func validateCapabilities(caps []Capability) error {
    if len(caps) == 0 {
        return errors.New("no capabilities defined")
    }
    
    for i, cap := range caps {
        if cap.Name == "" {
            return fmt.Errorf("capability %d missing name", i)
        }
        if cap.Confidence < 0 || cap.Confidence > 1 {
            return fmt.Errorf("capability %s confidence out of range: %f", 
                cap.Name, cap.Confidence)
        }
    }
    
    return nil
}
```

2. **Check for duplicate agent IDs:**
```go
// Generate unique agent ID
agentID := fmt.Sprintf("%s-%s-%s", 
    agentType, 
    hostname, 
    uuid.New().String()[:8])
```

3. **Verify message format:**
```go
// Use correct registration format
registration := map[string]interface{}{
    "method": "agent.register",
    "params": map[string]interface{}{
        "agent_id":   agentID,
        "agent_type": agentType,
        "capabilities": capabilities,
        "metadata": map[string]interface{}{
            "version": version,
            "sdk":     "go-1.2.0",
        },
    },
}
```

## Task Processing Errors

### Problem: Tasks Timing Out

**Symptoms:**
- Tasks marked as failed due to timeout
- Agent appears stuck
- No response sent for tasks

**Diagnosis:**
```go
func (a *Agent) processTask(ctx context.Context, task Task) error {
    // Add timeout tracking
    ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
    defer cancel()
    
    done := make(chan error, 1)
    
    go func() {
        done <- a.actualProcessTask(ctx, task)
    }()
    
    select {
    case err := <-done:
        return err
    case <-ctx.Done():
        log.Printf("Task %s timed out after %v", task.ID, 5*time.Minute)
        return ctx.Err()
    }
}
```

**Solutions:**

1. **Implement progress reporting:**
```go
func (a *Agent) processWithProgress(ctx context.Context, task Task) error {
    progressTicker := time.NewTicker(30 * time.Second)
    defer progressTicker.Stop()
    
    progress := 0.0
    
    for {
        select {
        case <-progressTicker.C:
            progress += 0.1
            a.reportProgress(task.ID, progress)
        case <-ctx.Done():
            return ctx.Err()
        default:
            // Process task...
            if done {
                return a.completeTask(task.ID, result)
            }
        }
    }
}
```

2. **Handle long-running tasks:**
```go
// For tasks that may exceed timeout
func (a *Agent) handleLongTask(task Task) error {
    // Request extended timeout
    if err := a.requestExtension(task.ID, 10*time.Minute); err != nil {
        return err
    }
    
    // Process in chunks
    for i, chunk := range task.Chunks {
        if err := a.processChunk(chunk); err != nil {
            return err
        }
        
        // Report progress
        progress := float64(i+1) / float64(len(task.Chunks))
        a.reportProgress(task.ID, progress)
    }
    
    return nil
}
```

### Problem: Task Results Rejected

**Symptoms:**
- `invalid result format` errors
- Tasks marked as failed despite processing
- Result size limit exceeded

**Solutions:**

1. **Validate result format:**
```go
func validateTaskResult(result interface{}) error {
    // Check size
    data, err := json.Marshal(result)
    if err != nil {
        return fmt.Errorf("result not serializable: %w", err)
    }
    
    if len(data) > 10*1024*1024 { // 10MB limit
        return errors.New("result exceeds size limit")
    }
    
    // Validate structure
    var resultMap map[string]interface{}
    if err := json.Unmarshal(data, &resultMap); err != nil {
        return fmt.Errorf("result must be JSON object: %w", err)
    }
    
    return nil
}
```

2. **Handle large results:**
```go
func (a *Agent) submitLargeResult(taskID string, result interface{}) error {
    data, _ := json.Marshal(result)
    
    if len(data) > 5*1024*1024 { // 5MB threshold
        // Upload to S3
        url, err := a.uploadToS3(taskID, data)
        if err != nil {
            return err
        }
        
        // Submit reference
        return a.completeTask(taskID, map[string]interface{}{
            "type": "large_result",
            "url":  url,
            "size": len(data),
        })
    }
    
    return a.completeTask(taskID, result)
}
```

## Performance Issues

### Problem: High Memory Usage

**Symptoms:**
- Agent consuming excessive memory
- Out of memory errors
- Gradual performance degradation

**Diagnosis:**
```go
import (
    "runtime"
    "runtime/debug"
    "runtime/pprof"
)

func diagnoseMemory() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    log.Printf("Alloc = %v MB", m.Alloc / 1024 / 1024)
    log.Printf("TotalAlloc = %v MB", m.TotalAlloc / 1024 / 1024)
    log.Printf("Sys = %v MB", m.Sys / 1024 / 1024)
    log.Printf("NumGC = %v", m.NumGC)
    
    // Force GC and print stats
    runtime.GC()
    debug.FreeOSMemory()
    
    // Create heap profile
    f, _ := os.Create("heap.prof")
    pprof.WriteHeapProfile(f)
    f.Close()
}
```

**Solutions:**

1. **Fix memory leaks:**
```go
// Common leak: goroutine accumulation
type Agent struct {
    tasks    chan Task
    shutdown chan struct{}
    wg       sync.WaitGroup
}

func (a *Agent) processTask(task Task) {
    a.wg.Add(1)
    go func() {
        defer a.wg.Done()
        
        // Use context for cancellation
        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()
        
        select {
        case <-a.shutdown:
            return
        default:
            // Process task
        }
    }()
}

func (a *Agent) Shutdown() {
    close(a.shutdown)
    a.wg.Wait() // Wait for all goroutines
}
```

2. **Implement memory limits:**
```go
func enforceMemoryLimit(limit uint64) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        var m runtime.MemStats
        runtime.ReadMemStats(&m)
        
        if m.Alloc > limit {
            log.Printf("Memory limit exceeded: %v > %v", m.Alloc, limit)
            
            // Force GC
            runtime.GC()
            debug.FreeOSMemory()
            
            // If still over limit, reject new tasks
            if m.Alloc > limit {
                a.pauseTaskProcessing()
            }
        }
    }
}
```

### Problem: High CPU Usage

**Symptoms:**
- 100% CPU utilization
- Slow response times
- System becoming unresponsive

**Solutions:**

1. **Profile CPU usage:**
```go
// Enable CPU profiling
func enableCPUProfile() {
    f, _ := os.Create("cpu.prof")
    pprof.StartCPUProfile(f)
    
    // Run for 30 seconds
    time.Sleep(30 * time.Second)
    
    pprof.StopCPUProfile()
    f.Close()
    
    // Analyze: go tool pprof cpu.prof
}
```

2. **Optimize hot paths:**
```go
// Before: Inefficient JSON parsing in loop
for _, item := range items {
    var data map[string]interface{}
    json.Unmarshal([]byte(item), &data) // Slow!
    process(data)
}

// After: Parse once, reuse decoder
decoder := json.NewDecoder(reader)
for decoder.More() {
    var data map[string]interface{}
    if err := decoder.Decode(&data); err != nil {
        continue
    }
    process(data)
}
```

## State Synchronization Problems

### Problem: CRDT State Conflicts

**Symptoms:**
- Inconsistent state across agents
- State updates lost
- Merge conflicts

**Diagnosis:**
```go
func diagnoseCRDTState(store *CRDTStore) {
    // Check vector clocks
    clock := store.GetVectorClock()
    log.Printf("Vector clock: %+v", clock)
    
    // Check for concurrent updates
    conflicts := store.GetConflicts()
    if len(conflicts) > 0 {
        log.Printf("Found %d conflicts", len(conflicts))
        for _, c := range conflicts {
            log.Printf("Conflict: %+v", c)
        }
    }
    
    // Verify merkle tree
    root := store.GetMerkleRoot()
    log.Printf("Merkle root: %x", root)
}
```

**Solutions:**

1. **Implement conflict resolution:**
```go
func (s *CRDTStore) resolveConflicts() {
    conflicts := s.detectConflicts()
    
    for _, conflict := range conflicts {
        switch conflict.Type {
        case "counter":
            // Counters: sum all increments
            total := 0
            for _, value := range conflict.Values {
                total += value.(int)
            }
            s.set(conflict.Key, total)
            
        case "set":
            // Sets: union all elements
            union := make(map[string]bool)
            for _, value := range conflict.Values {
                for _, elem := range value.([]string) {
                    union[elem] = true
                }
            }
            s.set(conflict.Key, keys(union))
            
        case "lww":
            // Last-write-wins: newest timestamp
            var newest interface{}
            var newestTime time.Time
            for _, entry := range conflict.Values {
                e := entry.(LWWEntry)
                if e.Timestamp.After(newestTime) {
                    newest = e.Value
                    newestTime = e.Timestamp
                }
            }
            s.set(conflict.Key, newest)
        }
    }
}
```

2. **Periodic sync verification:**
```go
func (a *Agent) verifySyncState() error {
    // Get local state hash
    localHash := a.state.Hash()
    
    // Request remote state hash
    remoteHash, err := a.requestStateHash()
    if err != nil {
        return err
    }
    
    if !bytes.Equal(localHash, remoteHash) {
        log.Println("State divergence detected")
        
        // Request full sync
        return a.requestFullSync()
    }
    
    return nil
}
```

## Binary Protocol Issues <!-- Source: pkg/models/websocket/binary.go -->

### Problem: Binary Message Corruption

**Symptoms:**
- `invalid magic number` errors
- Checksum failures
- Random disconnections

**Diagnosis:**
```go
func debugBinaryMessage(data []byte) {
    if len(data) < 24 {
        log.Printf("Message too short: %d bytes", len(data))
        return
    }
    
    // Check magic number
    magic := binary.BigEndian.Uint32(data[0:4])
    log.Printf("Magic: 0x%08X (expected: 0x%08X)", magic, MagicNumber)
    
    // Check version
    version := data[4]
    log.Printf("Version: %d", version)
    
    // Check flags
    flags := binary.BigEndian.Uint16(data[6:8])
    log.Printf("Flags: 0x%04X (compressed: %v)", 
        flags, flags&FlagCompressed != 0)
    
    // Dump hex
    log.Printf("Header hex: %x", data[:24])
}
```

**Solutions:**

1. **Implement message validation:**
```go
func validateBinaryMessage(data []byte) error {
    if len(data) < 24 {
        return errors.New("message too short")
    }
    
    header, err := ParseBinaryHeader(bytes.NewReader(data[:24]))
    if err != nil {
        return fmt.Errorf("header parse failed: %w", err)
    }
    
    // Verify payload size
    if len(data) != 24+int(header.DataSize) {
        return fmt.Errorf("size mismatch: header says %d, got %d", 
            header.DataSize, len(data)-24)
    }
    
    // Verify checksum if present
    if header.Flags&FlagChecksum != 0 {
        payload := data[24:]
        checksum := crc32.ChecksumIEEE(payload)
        // Compare with embedded checksum...
    }
    
    return nil
}
```

2. **Handle compression errors:**
```go
func decompressPayload(data []byte, flags uint16) ([]byte, error) {
    if flags&FlagCompressed == 0 {
        return data, nil
    }
    
    reader, err := gzip.NewReader(bytes.NewReader(data))
    if err != nil {
        // Try other compression formats
        if zr, err := zlib.NewReader(bytes.NewReader(data)); err == nil {
            return io.ReadAll(zr)
        }
        return nil, fmt.Errorf("decompression failed: %w", err)
    }
    
    return io.ReadAll(reader)
}
```

## Model Integration Errors

### Problem: Model API Failures

**Symptoms:**
- `model not available` errors
- Timeouts on model calls
- Inconsistent responses

**Solutions:**

1. **Implement model fallbacks:**
```go
type ModelManager struct {
    primary   Model
    fallbacks []Model
}

func (m *ModelManager) Complete(ctx context.Context, prompt string) (string, error) {
    // Try primary model
    result, err := m.primary.Complete(ctx, prompt)
    if err == nil {
        return result, nil
    }
    
    log.Printf("Primary model failed: %v", err)
    
    // Try fallbacks
    for _, fallback := range m.fallbacks {
        result, err := fallback.Complete(ctx, prompt)
        if err == nil {
            log.Printf("Fallback %s succeeded", fallback.Name())
            return result, nil
        }
    }
    
    return "", errors.New("all models failed")
}
```

2. **Handle rate limits:**
```go
type RateLimitedModel struct {
    model   Model
    limiter *rate.Limiter
    backoff time.Duration
}

func (m *RateLimitedModel) Complete(ctx context.Context, prompt string) (string, error) {
    // Wait for rate limit
    if err := m.limiter.Wait(ctx); err != nil {
        return "", err
    }
    
    result, err := m.model.Complete(ctx, prompt)
    
    // Handle rate limit error
    if isRateLimitError(err) {
        log.Printf("Rate limited, backing off %v", m.backoff)
        time.Sleep(m.backoff)
        m.backoff *= 2
        
        // Retry once
        return m.model.Complete(ctx, prompt)
    }
    
    // Reset backoff on success
    if err == nil {
        m.backoff = time.Second
    }
    
    return result, err
}
```

## Debugging Tools

### WebSocket Debugging Proxy <!-- Source: pkg/models/websocket/binary.go -->

```go
// Simple WebSocket debugging proxy <!-- Source: pkg/models/websocket/binary.go -->
func debugProxy(listenAddr, targetAddr string) {
    http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        // Connect to target
        targetConn, _, err := websocket.DefaultDialer.Dial(targetAddr, nil) <!-- Source: pkg/models/websocket/binary.go -->
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        defer targetConn.Close()
        
        // Upgrade client connection
        upgrader := websocket.Upgrader{} <!-- Source: pkg/models/websocket/binary.go -->
        clientConn, err := upgrader.Upgrade(w, r, nil)
        if err != nil {
            return
        }
        defer clientConn.Close()
        
        // Proxy messages with logging
        go proxyMessages(clientConn, targetConn, "client->server")
        proxyMessages(targetConn, clientConn, "server->client")
    })
    
    log.Fatal(http.ListenAndServe(listenAddr, nil))
}

func proxyMessages(from, to *websocket.Conn, direction string) { <!-- Source: pkg/models/websocket/binary.go -->
    for {
        msgType, data, err := from.ReadMessage()
        if err != nil {
            return
        }
        
        log.Printf("[%s] Type: %d, Size: %d", direction, msgType, len(data))
        
        if msgType == websocket.TextMessage { <!-- Source: pkg/models/websocket/binary.go -->
            log.Printf("[%s] Text: %s", direction, string(data))
        } else {
            log.Printf("[%s] Binary: %x", direction, data[:min(len(data), 32)])
        }
        
        if err := to.WriteMessage(msgType, data); err != nil {
            return
        }
    }
}
```

### Agent Debug Mode

```go
type DebugAgent struct {
    *Agent
    debugLog *log.Logger
}

func (d *DebugAgent) EnableDebugMode() {
    // Create debug log
    f, _ := os.Create("agent-debug.log")
    d.debugLog = log.New(f, "[DEBUG] ", log.LstdFlags|log.Lmicroseconds)
    
    // Hook into message flow
    d.OnSend = func(msg Message) {
        d.debugLog.Printf("SEND: %+v", msg)
    }
    
    d.OnReceive = func(msg Message) {
        d.debugLog.Printf("RECV: %+v", msg)
    }
    
    // Add performance tracking
    d.OnTaskStart = func(task Task) {
        d.debugLog.Printf("TASK START: %s", task.ID)
    }
    
    d.OnTaskComplete = func(task Task, duration time.Duration) {
        d.debugLog.Printf("TASK COMPLETE: %s in %v", task.ID, duration)
    }
}
```

## Common Error Codes

### MCP Error Code Reference

| Code | Name | Description | Common Causes | Solution |
|------|------|-------------|---------------|----------|
| 4000 | Invalid Message | Malformed message | JSON syntax error, wrong format | Validate message structure |
| 4001 | Auth Failed | Authentication error | Invalid API key, expired token | Check credentials |
| 4002 | Rate Limited | Too many requests | Exceeding rate limits | Implement backoff |
| 4003 | Server Error | Internal error | Server bug, overload | Retry with backoff |
| 4004 | Method Not Found | Unknown method | Typo, version mismatch | Check method name |
| 4005 | Invalid Params | Bad parameters | Missing required fields | Validate parameters |
| 4006 | Operation Cancelled | Task cancelled | Timeout, manual cancel | Handle cancellation |
| 4007 | Context Too Large | Size limit exceeded | Large payload | Compress or chunk data |
| 4008 | Conflict | State conflict | Concurrent updates | Implement CRDT resolution |

### Handling Specific Errors

```go
func handleMCPError(err error) error {
    var mcpErr *MCPError
    if !errors.As(err, &mcpErr) {
        return err
    }
    
    switch mcpErr.Code {
    case 4001: // Auth failed
        // Refresh token
        if err := refreshAuthToken(); err != nil {
            return err
        }
        return RetryableError{Err: mcpErr}
        
    case 4002: // Rate limited
        // Parse retry-after header
        retryAfter := parseRetryAfter(mcpErr.Data)
        time.Sleep(retryAfter)
        return RetryableError{Err: mcpErr}
        
    case 4007: // Context too large
        // Enable compression
        enableCompression()
        return RetryableError{Err: mcpErr}
        
    default:
        return mcpErr
    }
}
```

## Advanced Troubleshooting

### Distributed Tracing

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

func setupTracing() {
    // Configure Jaeger exporter
    exp, _ := jaeger.New(
        jaeger.WithCollectorEndpoint(
            jaeger.WithEndpoint("http://jaeger:14268/api/traces"),
        ),
    )
    
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exp),
        trace.WithResource(resource.NewWithAttributes(
            semconv.ServiceNameKey.String("mcp-agent"),
        )),
    )
    
    otel.SetTracerProvider(tp)
}

// Trace agent operations
func (a *Agent) processTaskWithTracing(ctx context.Context, task Task) error {
    tracer := otel.Tracer("agent")
    
    ctx, span := tracer.Start(ctx, "process_task",
        trace.WithAttributes(
            attribute.String("task.id", task.ID),
            attribute.String("task.type", task.Type),
        ),
    )
    defer span.End()
    
    // Trace each phase
    _, setupSpan := tracer.Start(ctx, "task.setup")
    // Setup...
    setupSpan.End()
    
    _, processSpan := tracer.Start(ctx, "task.process")
    result, err := a.process(ctx, task)
    if err != nil {
        processSpan.RecordError(err)
    }
    processSpan.End()
    
    return err
}
```

### Network Packet Capture

```bash
# Capture WebSocket traffic <!-- Source: pkg/models/websocket/binary.go -->
sudo tcpdump -i any -w agent.pcap 'port 8080'

# Analyze with Wireshark
wireshark agent.pcap

# Filter WebSocket frames <!-- Source: pkg/models/websocket/binary.go -->
ws.opcode == 1  # Text frames
ws.opcode == 2  # Binary frames
ws.opcode == 8  # Close frames
```

### Production Debugging Checklist

1. **Check infrastructure:**
   - [ ] MCP server health
   - [ ] Network connectivity
   - [ ] DNS resolution
   - [ ] Firewall rules
   - [ ] Load balancer config

2. **Verify configuration:**
   - [ ] API credentials
   - [ ] WebSocket URL <!-- Source: pkg/models/websocket/binary.go -->
   - [ ] TLS certificates
   - [ ] Timeout settings
   - [ ] Resource limits

3. **Review logs:**
   - [ ] Agent logs
   - [ ] Server logs
   - [ ] Network traces
   - [ ] Error patterns
   - [ ] Performance metrics

4. **Test isolation:**
   - [ ] Run minimal agent
   - [ ] Test with curl/wscat
   - [ ] Check with different models
   - [ ] Try different regions
   - [ ] Verify with SDK examples

## Prevention Best Practices

1. **Implement comprehensive logging**
2. **Add metrics and monitoring**
3. **Use circuit breakers**
4. **Test failure scenarios**
5. **Document error handling**
6. **Regular health checks**
7. **Gradual rollouts**
8. **Backup configurations**

## Getting Help

1. **Collect diagnostic information:**
   - Agent logs (last 1000 lines)
   - Error messages and stack traces
   - Configuration (sanitized)
   - Network traces if applicable
   - Performance metrics

2. **Check resources:**
   - [MCP Documentation](https://docs.mcp.dev)
   - [Agent SDK Issues](https://github.com/developer-mesh/developer-mesh/issues)
   - [Community Forum](https://forum.mcp.dev)
   - [Status Page](https://status.mcp.dev)

3. **Report issues:**
   - Use issue templates
   - Include diagnostic data
   - Provide reproduction steps
   - Mention SDK version
   - Tag appropriately

## Next Steps

1. Review [Agent WebSocket Protocol](./agent-websocket-protocol.md) for protocol details <!-- Source: pkg/models/websocket/binary.go -->
2. See [Agent SDK Guide](./agent-sdk-guide.md) for proper implementation
3. Check [Agent Integration Examples](./agent-integration-examples.md) for working code
