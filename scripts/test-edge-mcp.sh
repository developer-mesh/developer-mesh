#!/bin/bash
set -e

# Edge MCP Test Script
# Tests MCP protocol implementation and tool execution

echo "🧪 Edge MCP Test Suite"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Configuration
EDGE_MCP_PORT="${EDGE_MCP_PORT:-8082}"
EDGE_MCP_URL="ws://localhost:${EDGE_MCP_PORT}/ws"
API_KEY="${EDGE_MCP_API_KEY:-test-edge-key-123}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Function to test WebSocket connection
test_websocket_connection() {
    echo -e "\n${YELLOW}Test 1: WebSocket Connection${NC}"
    
    # Test basic connection
    response=$(echo '{"jsonrpc":"2.0","method":"ping","id":1}' | \
        websocat --one-message --text \
        --header="Authorization: Bearer ${API_KEY}" \
        "${EDGE_MCP_URL}" 2>/dev/null || echo "CONNECTION_FAILED")
    
    if [[ "$response" == "CONNECTION_FAILED" ]]; then
        echo -e "${RED}✗ Failed to connect to Edge MCP${NC}"
        ((TESTS_FAILED++))
        return 1
    else
        echo -e "${GREEN}✓ WebSocket connection successful${NC}"
        ((TESTS_PASSED++))
        return 0
    fi
}

# Function to test MCP initialization
test_mcp_initialization() {
    echo -e "\n${YELLOW}Test 2: MCP Protocol Initialization${NC}"
    
    # Send initialize request
    init_request='{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test-client","version":"1.0.0"}},"id":"init-1"}'
    
    response=$(echo "$init_request" | \
        websocat --one-message --text \
        --header="Authorization: Bearer ${API_KEY}" \
        "${EDGE_MCP_URL}" 2>/dev/null)
    
    # Check for expected response fields
    if echo "$response" | jq -e '.result.protocolVersion == "2025-06-18"' >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Protocol initialization successful${NC}"
        ((TESTS_PASSED++))
        
        # Extract capabilities
        echo "  Capabilities:"
        echo "$response" | jq '.result.capabilities' | head -10
        return 0
    else
        echo -e "${RED}✗ Protocol initialization failed${NC}"
        echo "  Response: $response"
        ((TESTS_FAILED++))
        return 1
    fi
}

# Function to test tools listing
test_tools_list() {
    echo -e "\n${YELLOW}Test 3: Tools Listing${NC}"
    
    # Create a temporary file for multi-message session
    session_file=$(mktemp)
    
    # Write session commands
    cat > "$session_file" << 'EOF'
{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test-client","version":"1.0.0"}},"id":"init-1"}
{"jsonrpc":"2.0","method":"initialized","params":{},"id":"init-2"}
{"jsonrpc":"2.0","method":"tools/list","id":"tools-1"}
EOF
    
    # Run session and capture last response
    response=$(cat "$session_file" | \
        websocat --text \
        --header="Authorization: Bearer ${API_KEY}" \
        "${EDGE_MCP_URL}" 2>/dev/null | tail -1)
    
    rm -f "$session_file"
    
    # Check for tools in response
    if echo "$response" | jq -e '.result.tools | length > 0' >/dev/null 2>&1; then
        tool_count=$(echo "$response" | jq '.result.tools | length')
        echo -e "${GREEN}✓ Tools list retrieved: $tool_count tools available${NC}"
        ((TESTS_PASSED++))
        
        # List first few tools
        echo "  Available tools:"
        echo "$response" | jq -r '.result.tools[:3] | .[] | "    - \(.name): \(.description)"'
        return 0
    else
        echo -e "${RED}✗ Failed to retrieve tools list${NC}"
        echo "  Response: $response"
        ((TESTS_FAILED++))
        return 1
    fi
}

# Function to test filesystem tool
test_filesystem_tool() {
    echo -e "\n${YELLOW}Test 4: Filesystem Tool Execution${NC}"
    
    # Create a temporary test file
    test_dir="/tmp/edge-mcp-test-$$"
    mkdir -p "$test_dir"
    echo "Edge MCP Test Content" > "$test_dir/test.txt"
    
    # Create session file
    session_file=$(mktemp)
    
    cat > "$session_file" << EOF
{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test-client","version":"1.0.0"}},"id":"init-1"}
{"jsonrpc":"2.0","method":"initialized","params":{},"id":"init-2"}
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"fs.read_file","arguments":{"path":"$test_dir/test.txt"}},"id":"read-1"}
EOF
    
    # Run session
    response=$(cat "$session_file" | \
        websocat --text \
        --header="Authorization: Bearer ${API_KEY}" \
        "${EDGE_MCP_URL}" 2>/dev/null | tail -1)
    
    rm -f "$session_file"
    rm -rf "$test_dir"
    
    # Check if file was read successfully
    if echo "$response" | jq -e '.result.content' >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Filesystem tool executed successfully${NC}"
        ((TESTS_PASSED++))
        return 0
    else
        echo -e "${RED}✗ Filesystem tool execution failed${NC}"
        echo "  Response: $response"
        ((TESTS_FAILED++))
        return 1
    fi
}

# Function to test security restrictions
test_security_restrictions() {
    echo -e "\n${YELLOW}Test 5: Security Restrictions${NC}"
    
    # Try to read a file outside allowed paths
    session_file=$(mktemp)
    
    cat > "$session_file" << 'EOF'
{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test-client","version":"1.0.0"}},"id":"init-1"}
{"jsonrpc":"2.0","method":"initialized","params":{},"id":"init-2"}
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"fs.read_file","arguments":{"path":"/etc/passwd"}},"id":"read-1"}
EOF
    
    # Run session
    response=$(cat "$session_file" | \
        websocat --text \
        --header="Authorization: Bearer ${API_KEY}" \
        "${EDGE_MCP_URL}" 2>/dev/null | tail -1)
    
    rm -f "$session_file"
    
    # Check if access was denied
    if echo "$response" | jq -e '.error' >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Security restrictions working correctly${NC}"
        echo "  Access denied for /etc/passwd as expected"
        ((TESTS_PASSED++))
        return 0
    else
        echo -e "${RED}✗ Security restriction test failed - access should have been denied${NC}"
        echo "  Response: $response"
        ((TESTS_FAILED++))
        return 1
    fi
}

# Function to test resources listing
test_resources_list() {
    echo -e "\n${YELLOW}Test 6: Resources Listing${NC}"
    
    session_file=$(mktemp)
    
    cat > "$session_file" << 'EOF'
{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test-client","version":"1.0.0"}},"id":"init-1"}
{"jsonrpc":"2.0","method":"initialized","params":{},"id":"init-2"}
{"jsonrpc":"2.0","method":"resources/list","id":"resources-1"}
EOF
    
    response=$(cat "$session_file" | \
        websocat --text \
        --header="Authorization: Bearer ${API_KEY}" \
        "${EDGE_MCP_URL}" 2>/dev/null | tail -1)
    
    rm -f "$session_file"
    
    if echo "$response" | jq -e '.result.resources | length > 0' >/dev/null 2>&1; then
        resource_count=$(echo "$response" | jq '.result.resources | length')
        echo -e "${GREEN}✓ Resources list retrieved: $resource_count resources available${NC}"
        ((TESTS_PASSED++))
        
        echo "  Available resources:"
        echo "$response" | jq -r '.result.resources[] | "    - \(.uri): \(.name)"'
        return 0
    else
        echo -e "${RED}✗ Failed to retrieve resources list${NC}"
        echo "  Response: $response"
        ((TESTS_FAILED++))
        return 1
    fi
}

# Function to test logging level setting
test_logging_level() {
    echo -e "\n${YELLOW}Test 7: Logging Level Configuration${NC}"
    
    session_file=$(mktemp)
    
    cat > "$session_file" << 'EOF'
{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test-client","version":"1.0.0"}},"id":"init-1"}
{"jsonrpc":"2.0","method":"initialized","params":{},"id":"init-2"}
{"jsonrpc":"2.0","method":"logging/setLevel","params":{"level":"debug"},"id":"log-1"}
EOF
    
    response=$(cat "$session_file" | \
        websocat --text \
        --header="Authorization: Bearer ${API_KEY}" \
        "${EDGE_MCP_URL}" 2>/dev/null | tail -1)
    
    rm -f "$session_file"
    
    if echo "$response" | jq -e '.result' >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Logging level set successfully${NC}"
        ((TESTS_PASSED++))
        return 0
    else
        echo -e "${RED}✗ Failed to set logging level${NC}"
        echo "  Response: $response"
        ((TESTS_FAILED++))
        return 1
    fi
}

# Main execution
main() {
    echo "Starting Edge MCP tests on port $EDGE_MCP_PORT"
    echo ""
    
    # Check if websocat is installed
    if ! command -v websocat &> /dev/null; then
        echo -e "${RED}Error: websocat is required but not installed${NC}"
        echo "Install with: brew install websocat (macOS) or cargo install websocat"
        exit 1
    fi
    
    # Check if jq is installed
    if ! command -v jq &> /dev/null; then
        echo -e "${RED}Error: jq is required but not installed${NC}"
        echo "Install with: brew install jq (macOS) or apt-get install jq"
        exit 1
    fi
    
    # Check if Edge MCP is running
    if ! nc -z localhost "$EDGE_MCP_PORT" 2>/dev/null; then
        echo -e "${YELLOW}Warning: Edge MCP doesn't appear to be running on port $EDGE_MCP_PORT${NC}"
        echo "Starting Edge MCP..."
        
        # Try to start Edge MCP in background
        cd /Users/seancorkum/projects/devops-mcp/apps/edge-mcp
        ./edge-mcp --port="$EDGE_MCP_PORT" --api-key="$API_KEY" --log-level=debug > edge-mcp.log 2>&1 &
        EDGE_MCP_PID=$!
        
        echo "Waiting for Edge MCP to start (PID: $EDGE_MCP_PID)..."
        sleep 3
        
        if ! nc -z localhost "$EDGE_MCP_PORT" 2>/dev/null; then
            echo -e "${RED}Failed to start Edge MCP${NC}"
            exit 1
        fi
    fi
    
    # Run tests
    test_websocket_connection
    test_mcp_initialization
    test_tools_list
    test_filesystem_tool
    test_security_restrictions
    test_resources_list
    test_logging_level
    
    # Summary
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo -e "${YELLOW}Test Summary${NC}"
    echo -e "  ${GREEN}Passed: $TESTS_PASSED${NC}"
    echo -e "  ${RED}Failed: $TESTS_FAILED${NC}"
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "\n${GREEN}✅ All tests passed!${NC}"
        exit 0
    else
        echo -e "\n${RED}❌ Some tests failed${NC}"
        exit 1
    fi
}

# Cleanup function
cleanup() {
    if [ ! -z "$EDGE_MCP_PID" ]; then
        echo -e "\n${YELLOW}Stopping Edge MCP (PID: $EDGE_MCP_PID)${NC}"
        kill "$EDGE_MCP_PID" 2>/dev/null || true
    fi
}

# Set trap for cleanup
trap cleanup EXIT

# Run main function
main "$@"