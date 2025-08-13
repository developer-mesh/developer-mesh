#!/bin/bash
# Test script to verify Edge MCP platform detection and adaptation

set -e

echo "=== Testing Edge MCP Platform Detection ==="
echo

# Start Edge MCP in background and capture its output
echo "Starting Edge MCP..."
./edge-mcp --port=8083 --log-level=debug --work-dir=/tmp > edge-mcp.log 2>&1 &
EDGE_PID=$!

# Give it time to start
sleep 2

# Check if it's running
if ! kill -0 $EDGE_PID 2>/dev/null; then
    echo "❌ Edge MCP failed to start"
    cat edge-mcp.log
    exit 1
fi

echo "✅ Edge MCP started successfully (PID: $EDGE_PID)"
echo

# Check platform detection in logs
echo "Platform detection from logs:"
grep "Edge MCP starting" edge-mcp.log | head -1
echo

# Test MCP connection and get platform info
echo "Testing MCP connection and platform info..."
cat << 'EOF' | websocat --text ws://localhost:8083/ws 2>/dev/null || true
{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test-client","version":"1.0.0"}},"id":1}
{"jsonrpc":"2.0","method":"initialized","params":{},"id":2}
{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"edge://platform/info"},"id":3}
{"jsonrpc":"2.0","method":"shutdown","params":{},"id":4}
EOF

echo
echo "Checking command executor initialization..."
grep "CommandExecutor initialized" edge-mcp.log | head -1
echo

# Clean up
echo "Cleaning up..."
kill $EDGE_PID 2>/dev/null || true
wait $EDGE_PID 2>/dev/null || true
rm -f edge-mcp.log

echo "✅ Platform detection test completed"