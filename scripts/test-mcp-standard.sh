#!/bin/bash

# MCP Standard Protocol Test Script
# Tests all standard MCP methods and DevMesh extensions

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

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Check if websocat is installed
if ! command -v websocat &> /dev/null; then
    print_error "websocat is not installed. Please install it first:"
    echo "  brew install websocat  # On macOS"
    echo "  cargo install websocat # Using Rust cargo"
    exit 1
fi

# Test function that sends a message and captures response
test_mcp_method() {
    local method_name="$1"
    local message="$2"
    local description="$3"
    
    print_status "Testing: $description"
    echo "Request: $message"
    
    response=$(echo "$message" | websocat -n1 --header="$AUTH_HEADER" "$WS_URL" 2>/dev/null || true)
    
    if [ -z "$response" ]; then
        print_error "No response received"
        return 1
    fi
    
    echo "Response: $response"
    
    # Check for error in response
    if echo "$response" | grep -q '"error"'; then
        print_warning "Response contains error"
    else
        print_status "✓ Success"
    fi
    
    echo "---"
    return 0
}

# Start testing
echo "========================================="
echo "MCP Standard Protocol Test Suite"
echo "Testing server at: $WS_URL"
echo "========================================="
echo

# Test 1: Initialize
print_status "Test 1: Initialize connection"
test_mcp_method "initialize" \
    '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test-client","version":"1.0.0","type":"testing"}},"id":"1"}' \
    "MCP Initialize"

# Test 2: Initialized confirmation
print_status "Test 2: Initialized confirmation"
test_mcp_method "initialized" \
    '{"jsonrpc":"2.0","method":"initialized","params":{},"id":"2"}' \
    "MCP Initialized Confirmation"

# Test 3: Ping
print_status "Test 3: Ping/Pong"
test_mcp_method "ping" \
    '{"jsonrpc":"2.0","method":"ping","params":{},"id":"3"}' \
    "MCP Ping"

# Test 4: List tools
print_status "Test 4: List available tools"
test_mcp_method "tools/list" \
    '{"jsonrpc":"2.0","method":"tools/list","params":{},"id":"4"}' \
    "List MCP Tools (including DevMesh tools)"

# Test 5: List resources
print_status "Test 5: List available resources"
test_mcp_method "resources/list" \
    '{"jsonrpc":"2.0","method":"resources/list","params":{},"id":"5"}' \
    "List MCP Resources"

# Test 6: Read a resource
print_status "Test 6: Read system health resource"
test_mcp_method "resources/read" \
    '{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"devmesh://system/health"},"id":"6"}' \
    "Read System Health Resource"

# Test 7: DevMesh tool - Context Update
print_status "Test 7: DevMesh Context Update"
test_mcp_method "tools/call" \
    '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"devmesh.context.update","arguments":{"context":{"test":"data","timestamp":"2025-01-01"}}},"id":"7"}' \
    "Update Context via DevMesh Tool"

# Test 8: DevMesh tool - Context Get
print_status "Test 8: DevMesh Context Get"
test_mcp_method "tools/call" \
    '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"devmesh.context.get","arguments":{}},"id":"8"}' \
    "Get Context via DevMesh Tool"

# Test 9: DevMesh tool - Workflow List
print_status "Test 9: DevMesh Workflow List"
test_mcp_method "tools/call" \
    '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"devmesh.workflow.list","arguments":{}},"id":"9"}' \
    "List Workflows via DevMesh Tool"

# Test 10: DevMesh tool - Task Create
print_status "Test 10: DevMesh Task Create"
test_mcp_method "tools/call" \
    '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"devmesh.task.create","arguments":{"title":"Test Task","type":"feature","priority":"medium"}},"id":"10"}' \
    "Create Task via DevMesh Tool"

# Test 11: Resource Subscribe
print_status "Test 11: Subscribe to resource updates"
test_mcp_method "resources/subscribe" \
    '{"jsonrpc":"2.0","method":"resources/subscribe","params":{"uri":"devmesh://system/health"},"id":"11"}' \
    "Subscribe to Resource Updates"

# Test 12: Resource Unsubscribe
print_status "Test 12: Unsubscribe from resource updates"
test_mcp_method "resources/unsubscribe" \
    '{"jsonrpc":"2.0","method":"resources/unsubscribe","params":{"uri":"devmesh://system/health"},"id":"12"}' \
    "Unsubscribe from Resource Updates"

# Test 13: Prompts List
print_status "Test 13: List prompts"
test_mcp_method "prompts/list" \
    '{"jsonrpc":"2.0","method":"prompts/list","params":{},"id":"13"}' \
    "List Available Prompts"

# Test 14: Logging Set Level
print_status "Test 14: Set logging level"
test_mcp_method "logging/setLevel" \
    '{"jsonrpc":"2.0","method":"logging/setLevel","params":{"level":"debug"},"id":"14"}' \
    "Set Logging Level"

# Test 15: Cancel Request
print_status "Test 15: Cancel request"
test_mcp_method "$/cancelRequest" \
    '{"jsonrpc":"2.0","method":"$/cancelRequest","params":{"id":"999"},"id":"15"}' \
    "Cancel Request"

# Test 16: Custom DevMesh Extension - Agent Register
print_status "Test 16: DevMesh Agent Registration"
test_mcp_method "x-devmesh/agent/register" \
    '{"jsonrpc":"2.0","method":"x-devmesh/agent/register","params":{"agent_id":"test-agent-001","agent_type":"testing","capabilities":["test","debug"]},"id":"16"}' \
    "Register Agent via DevMesh Extension"

# Test 17: Custom DevMesh Extension - Semantic Search
print_status "Test 17: DevMesh Semantic Search"
test_mcp_method "x-devmesh/search/semantic" \
    '{"jsonrpc":"2.0","method":"x-devmesh/search/semantic","params":{"query":"test query","limit":5},"id":"17"}' \
    "Semantic Search via DevMesh Extension"

# Test 18: Shutdown
print_status "Test 18: Shutdown connection"
test_mcp_method "shutdown" \
    '{"jsonrpc":"2.0","method":"shutdown","params":{},"id":"18"}' \
    "Graceful Shutdown"

echo
echo "========================================="
echo "Test Suite Complete"
echo "========================================="

# Test with different headers to check connection mode detection
echo
echo "Testing Connection Mode Detection..."
echo "---"

print_status "Testing Claude Code detection"
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18"},"id":"100"}' | \
    websocat -n1 --header="$AUTH_HEADER" --header="User-Agent: Claude-Code/1.0.0" "$WS_URL" 2>/dev/null || true

print_status "Testing IDE detection"
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18"},"id":"101"}' | \
    websocat -n1 --header="$AUTH_HEADER" --header="X-IDE-Name: VSCode" "$WS_URL" 2>/dev/null || true

print_status "Testing Agent detection"
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18"},"id":"102"}' | \
    websocat -n1 --header="$AUTH_HEADER" --header="X-Agent-ID: agent-123" "$WS_URL" 2>/dev/null || true

echo
print_status "All tests completed!"