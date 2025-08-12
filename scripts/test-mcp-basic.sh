#!/bin/bash

# ============================================================================
# Basic MCP Protocol Test
# ============================================================================
# Tests the actual MCP methods implemented in the server
# ============================================================================

set -e

# Configuration
MCP_WS_URL="${MCP_WS_URL:-ws://localhost:8080/ws}"
API_KEY="${API_KEY:-dev-admin-key-1234567890}"
TENANT_ID="${TENANT_ID:-00000000-0000-0000-0000-000000000001}"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# MCP WebSocket communication
mcp_send() {
    local message="$1"
    local timeout="${2:-3}"
    
    # Compact JSON to single line
    local compact_message
    compact_message=$(echo "$message" | jq -c . 2>/dev/null || echo "$message")
    
    # Send message and capture response
    local response
    response=$( (printf "%s\n" "$compact_message"; sleep 1) | \
                websocat -t -n1 \
                --header="Authorization: Bearer ${API_KEY}" \
                --header="X-Tenant-ID: ${TENANT_ID}" \
                "$MCP_WS_URL" 2>/dev/null )
    
    echo "${response:-{}}"
}

# Test 1: MCP Initialize
test_initialize() {
    echo -e "\n${BOLD}1. Testing MCP Initialize${NC}"
    
    local msg=$(cat <<EOF
{
    "jsonrpc": "2.0",
    "method": "initialize",
    "params": {
        "protocolVersion": "1.0.0",
        "capabilities": {
            "tools": {
                "listChanged": true
            },
            "resources": {
                "listChanged": true,
                "subscribe": true
            },
            "prompts": {
                "listChanged": true
            }
        },
        "clientInfo": {
            "name": "test-client",
            "version": "1.0.0"
        }
    },
    "id": "init-$(date +%s)"
}
EOF
    )
    
    local response=$(mcp_send "$msg")
    
    if echo "$response" | grep -q '"result"'; then
        echo -e "${GREEN}✓ Initialize successful${NC}"
        
        # Extract server info
        local server_name=$(echo "$response" | jq -r '.result.serverInfo.name // "unknown"')
        local server_version=$(echo "$response" | jq -r '.result.serverInfo.version // "unknown"')
        echo -e "${GREEN}  Server: ${server_name} v${server_version}${NC}"
    else
        echo -e "${RED}✗ Initialize failed${NC}"
        echo -e "${YELLOW}Response: $response${NC}"
        return 1
    fi
}

# Test 2: List Tools
test_tools_list() {
    echo -e "\n${BOLD}2. Testing Tools List${NC}"
    
    local msg=$(cat <<EOF
{
    "jsonrpc": "2.0",
    "method": "tools/list",
    "params": {},
    "id": "tools-$(date +%s)"
}
EOF
    )
    
    local response=$(mcp_send "$msg")
    
    if echo "$response" | grep -q '"tools"'; then
        local tool_count=$(echo "$response" | jq '.result.tools | length')
        echo -e "${GREEN}✓ ${tool_count} tools available${NC}"
        
        # List first 3 tools
        echo -e "${CYAN}Sample tools:${NC}"
        echo "$response" | jq -r '.result.tools[0:3] | .[] | "  • \(.name): \(.description)"' 2>/dev/null | head -3
    else
        echo -e "${YELLOW}⚠ No tools found or error${NC}"
        echo -e "${YELLOW}Response: $response${NC}"
    fi
}

# Test 3: Call a Tool
test_tool_call() {
    echo -e "\n${BOLD}3. Testing Tool Call${NC}"
    
    # First get the list of tools to find a valid tool name
    local list_msg=$(cat <<EOF
{
    "jsonrpc": "2.0",
    "method": "tools/list",
    "params": {},
    "id": "list-for-call-$(date +%s)"
}
EOF
    )
    
    local list_response=$(mcp_send "$list_msg")
    local first_tool=$(echo "$list_response" | jq -r '.result.tools[0].name // "unknown"')
    
    if [ "$first_tool" = "unknown" ]; then
        echo -e "${YELLOW}⚠ No tools available to test${NC}"
        return
    fi
    
    echo -e "${CYAN}Calling tool: ${first_tool}${NC}"
    
    local msg=$(cat <<EOF
{
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
        "name": "${first_tool}",
        "arguments": {}
    },
    "id": "call-$(date +%s)"
}
EOF
    )
    
    local response=$(mcp_send "$msg")
    
    if echo "$response" | grep -q '"result"'; then
        echo -e "${GREEN}✓ Tool call successful${NC}"
    elif echo "$response" | grep -q '"error"'; then
        local error_msg=$(echo "$response" | jq -r '.error.message // "unknown error"')
        echo -e "${YELLOW}⚠ Tool call returned error: ${error_msg}${NC}"
    else
        echo -e "${RED}✗ Tool call failed${NC}"
        echo -e "${YELLOW}Response: $response${NC}"
    fi
}

# Test 4: List Resources
test_resources_list() {
    echo -e "\n${BOLD}4. Testing Resources List${NC}"
    
    local msg=$(cat <<EOF
{
    "jsonrpc": "2.0",
    "method": "resources/list",
    "params": {},
    "id": "resources-$(date +%s)"
}
EOF
    )
    
    local response=$(mcp_send "$msg")
    
    if echo "$response" | grep -q '"resources"'; then
        local resource_count=$(echo "$response" | jq '.result.resources | length')
        echo -e "${GREEN}✓ ${resource_count} resources available${NC}"
        
        if [ "$resource_count" -gt 0 ]; then
            echo -e "${CYAN}Sample resources:${NC}"
            echo "$response" | jq -r '.result.resources[0:3] | .[] | "  • \(.uri): \(.name)"' 2>/dev/null | head -3
        fi
    else
        echo -e "${YELLOW}⚠ No resources found or not implemented${NC}"
    fi
}

# Test 5: List Prompts
test_prompts_list() {
    echo -e "\n${BOLD}5. Testing Prompts List${NC}"
    
    local msg=$(cat <<EOF
{
    "jsonrpc": "2.0",
    "method": "prompts/list",
    "params": {},
    "id": "prompts-$(date +%s)"
}
EOF
    )
    
    local response=$(mcp_send "$msg")
    
    if echo "$response" | grep -q '"prompts"'; then
        local prompt_count=$(echo "$response" | jq '.result.prompts | length')
        echo -e "${GREEN}✓ ${prompt_count} prompts available${NC}"
        
        if [ "$prompt_count" -gt 0 ]; then
            echo -e "${CYAN}Sample prompts:${NC}"
            echo "$response" | jq -r '.result.prompts[0:3] | .[] | "  • \(.name): \(.description)"' 2>/dev/null | head -3
        fi
    else
        echo -e "${YELLOW}⚠ No prompts found or not implemented${NC}"
    fi
}

# Main test execution
main() {
    echo -e "${BOLD}${CYAN}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BOLD}${CYAN}║         Basic MCP Protocol Test Suite                     ║${NC}"
    echo -e "${BOLD}${CYAN}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${CYAN}Server: ${MCP_WS_URL}${NC}"
    echo -e "${CYAN}Tenant: ${TENANT_ID}${NC}"
    echo ""
    
    # Pre-flight check
    echo -e "${YELLOW}Performing pre-flight checks...${NC}"
    if ! curl -f -s "http://localhost:8080/health" > /dev/null; then
        echo -e "${RED}✗ MCP Server not responding${NC}"
        echo "Run: docker-compose up mcp-server"
        exit 1
    fi
    echo -e "${GREEN}✓ MCP Server healthy${NC}"
    
    if ! command -v websocat &> /dev/null; then
        echo -e "${RED}✗ websocat not installed${NC}"
        echo "Install: brew install websocat"
        exit 1
    fi
    echo -e "${GREEN}✓ websocat available${NC}"
    
    # Run tests
    test_initialize
    test_tools_list
    test_tool_call
    test_resources_list
    test_prompts_list
    
    echo -e "\n${BOLD}${GREEN}═══ Test Complete ═══${NC}"
    echo -e "${GREEN}✓ All basic MCP methods tested${NC}"
}

# Run the test
main "$@"