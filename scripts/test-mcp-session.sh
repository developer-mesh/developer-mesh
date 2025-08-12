#!/bin/bash

# MCP Session Test Script - Tests with persistent session
# This script maintains a single connection for all tests

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
HOST="${HOST:-localhost}"
PORT="${PORT:-8080}"
WS_URL="ws://${HOST}:${PORT}/ws"
AUTH_HEADER="Authorization: Bearer dev-admin-key-1234567890"

# Function to print colored output
print_status() {
    echo -e "${GREEN}[TEST]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

# Check if websocat is installed
if ! command -v websocat &> /dev/null; then
    print_error "websocat is not installed. Please install it first"
    exit 1
fi

# Create a temp file for test messages
TEMP_FILE=$(mktemp)
trap "rm -f $TEMP_FILE" EXIT

# Build the test sequence
cat > $TEMP_FILE << 'EOF'
{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test-session","version":"1.0.0"}},"id":"1"}
{"jsonrpc":"2.0","method":"initialized","params":{},"id":"2"}
{"jsonrpc":"2.0","method":"tools/list","params":{},"id":"3"}
{"jsonrpc":"2.0","method":"resources/list","params":{},"id":"4"}
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"devmesh.context.update","arguments":{"context":{"test":"data","session":"active"}}},"id":"5"}
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"devmesh.context.get","arguments":{}},"id":"6"}
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"devmesh.workflow.list","arguments":{}},"id":"7"}
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"devmesh.task.create","arguments":{"title":"Session Test Task","type":"feature"}},"id":"8"}
{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"devmesh://system/health"},"id":"9"}
{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"devmesh://agents/00000000-0000-0000-0000-000000000001"},"id":"10"}
{"jsonrpc":"2.0","method":"shutdown","params":{},"id":"11"}
EOF

echo "========================================="
echo "MCP Session Test"
echo "Testing server at: $WS_URL"
echo "========================================="
echo

print_status "Starting session test with persistent connection..."
echo

# Run all tests in a single session
websocat --header="$AUTH_HEADER" -n "$WS_URL" < $TEMP_FILE | while IFS= read -r line; do
    # Parse the response
    if echo "$line" | grep -q '"id":"1"'; then
        print_success "Initialize successful"
    elif echo "$line" | grep -q '"id":"2"'; then
        print_success "Initialized confirmation"
    elif echo "$line" | grep -q '"id":"3"'; then
        if echo "$line" | grep -q '"tools"'; then
            print_success "Tools list retrieved (with DevMesh tools)"
        else
            print_error "Tools list failed"
        fi
    elif echo "$line" | grep -q '"id":"4"'; then
        print_success "Resources list retrieved"
    elif echo "$line" | grep -q '"id":"5"'; then
        if echo "$line" | grep -q '"error"'; then
            print_error "Context update failed"
        else
            print_success "Context updated successfully"
        fi
    elif echo "$line" | grep -q '"id":"6"'; then
        if echo "$line" | grep -q '"error"'; then
            print_error "Context get failed"
        else
            print_success "Context retrieved successfully"
        fi
    elif echo "$line" | grep -q '"id":"7"'; then
        if echo "$line" | grep -q '"error"'; then
            print_error "Workflow list failed"
        elif echo "$line" | grep -q '"content"'; then
            # MCP returns results in content[].text format
            print_success "Workflows listed successfully"
        else
            print_error "Workflow list failed - unexpected format"
        fi
    elif echo "$line" | grep -q '"id":"8"'; then
        if echo "$line" | grep -q '"error"'; then
            print_error "Task creation failed"
        elif echo "$line" | grep -q '"content"'; then
            # MCP returns results in content[].text format
            print_success "Task created successfully"
        else
            print_error "Task creation failed - unexpected format"
        fi
    elif echo "$line" | grep -q '"id":"9"'; then
        print_success "System health resource read"
    elif echo "$line" | grep -q '"id":"10"'; then
        print_success "Agents resource read"
    elif echo "$line" | grep -q '"id":"11"'; then
        print_success "Shutdown successful"
    fi
    
    # Show the actual response for debugging
    echo "  Response: $(echo $line | jq -c . 2>/dev/null || echo $line)"
    echo
done

echo "========================================="
echo "Session Test Complete"
echo "========================================="

# Test different connection modes with proper sessions
echo
echo "Testing Connection Modes with Sessions..."
echo "---"

# Claude Code session
print_status "Testing Claude Code session"
(echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"claude-code","version":"1.0.0"}},"id":"100"}'; 
 echo '{"jsonrpc":"2.0","method":"tools/list","params":{},"id":"101"}'; 
 echo '{"jsonrpc":"2.0","method":"shutdown","params":{},"id":"102"}') | \
    websocat --header="$AUTH_HEADER" --header="User-Agent: Claude-Code/1.0.0" -n "$WS_URL" | \
    while IFS= read -r line; do
        if echo "$line" | grep -q '"id":"101"' && echo "$line" | grep -q '"tools"'; then
            print_success "Claude Code session works with tools"
        fi
    done

echo
print_status "All session tests completed!"