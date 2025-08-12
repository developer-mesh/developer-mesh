#!/bin/bash

# MCP Response Validation Test
# This script validates that responses contain the correct data structure

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

# Configuration
HOST="${HOST:-localhost}"
PORT="${PORT:-8080}"
WS_URL="ws://${HOST}:${PORT}/ws"
AUTH_HEADER="Authorization: Bearer dev-admin-key-1234567890"

echo "========================================="
echo "MCP Response Validation Test"
echo "========================================="
echo

# Test workflow list response structure
echo -e "${GREEN}[TEST]${NC} Validating workflow list response structure..."
RESPONSE=$(echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18"},"id":"1"}
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"devmesh.workflow.list","arguments":{}},"id":"2"}' | \
    websocat --header="$AUTH_HEADER" -n "$WS_URL" 2>/dev/null | tail -1)

# Extract and parse the text content
WORKFLOW_DATA=$(echo "$RESPONSE" | jq -r '.result.content[0].text' 2>/dev/null | jq . 2>/dev/null)

if [ $? -eq 0 ]; then
    # Check for expected fields
    if echo "$WORKFLOW_DATA" | jq -e '.workflows | length > 0' > /dev/null 2>&1; then
        echo -e "${GREEN}[✓]${NC} Workflow list contains valid workflows array"
        echo "  Found $(echo "$WORKFLOW_DATA" | jq '.workflows | length') workflows"
        echo "$WORKFLOW_DATA" | jq -r '.workflows[] | "  - \(.name) (\(.id))"'
    else
        echo -e "${RED}[✗]${NC} Workflow list missing workflows array"
    fi
else
    echo -e "${RED}[✗]${NC} Failed to parse workflow response"
fi

echo

# Test task creation response structure
echo -e "${GREEN}[TEST]${NC} Validating task creation response structure..."
RESPONSE=$(echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18"},"id":"1"}
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"devmesh.task.create","arguments":{"title":"Validation Test","type":"feature"}},"id":"2"}' | \
    websocat --header="$AUTH_HEADER" -n "$WS_URL" 2>/dev/null | tail -1)

TASK_DATA=$(echo "$RESPONSE" | jq -r '.result.content[0].text' 2>/dev/null | jq . 2>/dev/null)

if [ $? -eq 0 ]; then
    # Validate task fields
    TASK_ID=$(echo "$TASK_DATA" | jq -r '.id')
    TASK_TITLE=$(echo "$TASK_DATA" | jq -r '.title')
    TASK_STATUS=$(echo "$TASK_DATA" | jq -r '.status')
    
    if [[ -n "$TASK_ID" && "$TASK_TITLE" == "Validation Test" && "$TASK_STATUS" == "created" ]]; then
        echo -e "${GREEN}[✓]${NC} Task created with correct structure"
        echo "  Task ID: $TASK_ID"
        echo "  Title: $TASK_TITLE"
        echo "  Status: $TASK_STATUS"
    else
        echo -e "${RED}[✗]${NC} Task structure invalid"
    fi
else
    echo -e "${RED}[✗]${NC} Failed to parse task response"
fi

echo

# Test context get/update cycle
echo -e "${GREEN}[TEST]${NC} Validating context management cycle..."
SESSION_ID="test-$(date +%s)"

# First update context
UPDATE_RESPONSE=$(echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"'$SESSION_ID'"}},"id":"1"}
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"devmesh.context.update","arguments":{"context":{"test_key":"test_value","session":"'$SESSION_ID'"}}},"id":"2"}' | \
    websocat --header="$AUTH_HEADER" -n "$WS_URL" 2>/dev/null | tail -1)

if echo "$UPDATE_RESPONSE" | jq -e '.result.content[0].text' | grep -q "success"; then
    echo -e "${GREEN}[✓]${NC} Context update successful"
    
    # Now get the context
    GET_RESPONSE=$(echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"'$SESSION_ID'"}},"id":"1"}
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"devmesh.context.get","arguments":{}},"id":"2"}' | \
        websocat --header="$AUTH_HEADER" -n "$WS_URL" 2>/dev/null | tail -1)
    
    CONTEXT_DATA=$(echo "$GET_RESPONSE" | jq -r '.result.content[0].text' 2>/dev/null | jq . 2>/dev/null)
    
    if echo "$CONTEXT_DATA" | jq -e '.agent_id' > /dev/null 2>&1; then
        echo -e "${GREEN}[✓]${NC} Context retrieved with valid structure"
        echo "  Session ID: $(echo "$CONTEXT_DATA" | jq -r '.session_id')"
        echo "  Agent ID: $(echo "$CONTEXT_DATA" | jq -r '.agent_id')"
    else
        echo -e "${RED}[✗]${NC} Context structure invalid"
    fi
else
    echo -e "${RED}[✗]${NC} Context update failed"
fi

echo

# Test resource reading
echo -e "${GREEN}[TEST]${NC} Validating resource reading..."
RESOURCE_RESPONSE=$(echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18"},"id":"1"}
{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"devmesh://system/health"},"id":"2"}' | \
    websocat --header="$AUTH_HEADER" -n "$WS_URL" 2>/dev/null | tail -1)

HEALTH_DATA=$(echo "$RESOURCE_RESPONSE" | jq -r '.result.contents[0].text' 2>/dev/null | jq . 2>/dev/null)

if [ $? -eq 0 ]; then
    STATUS=$(echo "$HEALTH_DATA" | jq -r '.status')
    VERSION=$(echo "$HEALTH_DATA" | jq -r '.version')
    
    if [[ "$STATUS" == "healthy" && -n "$VERSION" ]]; then
        echo -e "${GREEN}[✓]${NC} System health resource valid"
        echo "  Status: $STATUS"
        echo "  Version: $VERSION"
        echo "  Active connections: $(echo "$HEALTH_DATA" | jq -r '.connections')"
    else
        echo -e "${RED}[✗]${NC} Health resource structure invalid"
    fi
else
    echo -e "${RED}[✗]${NC} Failed to parse health resource"
fi

echo
echo "========================================="
echo "Validation Test Complete"
echo "========================================="