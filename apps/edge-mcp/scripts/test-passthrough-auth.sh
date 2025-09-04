#!/bin/bash

# Test script for Edge MCP pass-through authentication
# This script tests that Edge MCP correctly extracts and forwards user tokens

set -e

echo "=== Edge MCP Pass-Through Authentication Test ==="
echo

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if Edge MCP is built
if [ ! -f "./edge-mcp" ] && [ ! -f "./bin/edge-mcp" ]; then
    echo -e "${RED}Error: Edge MCP binary not found. Please build first with 'go build -o edge-mcp ./cmd/server'${NC}"
    exit 1
fi

# Set test environment variables
export DEV_MESH_URL="https://api.devmesh.ai"
export DEV_MESH_API_KEY="test-api-key"
export TENANT_ID="test-tenant"

# Set test passthrough tokens
export GITHUB_TOKEN="ghp_test_github_token_123456"
export AWS_ACCESS_KEY_ID="AKIATEST123456"
export AWS_SECRET_ACCESS_KEY="test_secret_key_123456"
export SLACK_TOKEN="xoxb-test-slack-token"
export JIRA_TOKEN="test-jira-token"

echo "Test Environment Variables Set:"
echo "  GITHUB_TOKEN: ${GITHUB_TOKEN:0:10}..."
echo "  AWS_ACCESS_KEY_ID: ${AWS_ACCESS_KEY_ID:0:10}..."
echo "  SLACK_TOKEN: ${SLACK_TOKEN:0:10}..."
echo "  JIRA_TOKEN: ${JIRA_TOKEN:0:10}..."
echo

# Start Edge MCP in background with debug logging
echo "Starting Edge MCP with debug logging..."
if [ -f "./edge-mcp" ]; then
    ./edge-mcp --port 8082 --log-level debug > edge-mcp-test.log 2>&1 &
else
    ./bin/edge-mcp --port 8082 --log-level debug > edge-mcp-test.log 2>&1 &
fi
EDGE_PID=$!

# Give Edge MCP time to start
sleep 3

# Check if Edge MCP started successfully
if ! kill -0 $EDGE_PID 2>/dev/null; then
    echo -e "${RED}Error: Edge MCP failed to start. Check edge-mcp-test.log${NC}"
    cat edge-mcp-test.log
    exit 1
fi

echo -e "${GREEN}Edge MCP started successfully (PID: $EDGE_PID)${NC}"
echo

# Test WebSocket connection and check for passthrough auth extraction
echo "Testing WebSocket connection..."

# Create a test WebSocket request
cat > test-mcp-request.json << EOF
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test-client","version":"1.0.0"}}}
EOF

# Use websocat or curl to test (prefer websocat if available)
if command -v websocat &> /dev/null; then
    echo "Using websocat to test connection..."
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test","version":"1.0.0"}}}' | \
        websocat -n1 ws://localhost:8082/ws > test-response.json 2>/dev/null || true
else
    echo "Using curl to test connection..."
    curl -i -N \
        -H "Connection: Upgrade" \
        -H "Upgrade: websocket" \
        -H "X-GitHub-Token: ${GITHUB_TOKEN}" \
        -H "X-AWS-Access-Key: ${AWS_ACCESS_KEY_ID}" \
        http://localhost:8082/ws 2>/dev/null || true
fi

# Check logs for passthrough auth extraction
echo
echo "Checking Edge MCP logs for passthrough authentication..."
echo

# Look for passthrough auth messages in log
if grep -q "Extracted passthrough authentication" edge-mcp-test.log; then
    echo -e "${GREEN}✓ Pass-through authentication extraction detected${NC}"
    
    # Check for specific tokens
    if grep -q "Found GitHub passthrough token" edge-mcp-test.log; then
        echo -e "${GREEN}  ✓ GitHub token detected${NC}"
    else
        echo -e "${YELLOW}  ⚠ GitHub token not detected${NC}"
    fi
    
    if grep -q "Found AWS passthrough credentials" edge-mcp-test.log; then
        echo -e "${GREEN}  ✓ AWS credentials detected${NC}"
    else
        echo -e "${YELLOW}  ⚠ AWS credentials not detected${NC}"
    fi
    
    if grep -q "Found service token from environment" edge-mcp-test.log; then
        echo -e "${GREEN}  ✓ Service tokens detected from environment${NC}"
    else
        echo -e "${YELLOW}  ⚠ Service tokens not detected from environment${NC}"
    fi
    
    # Show credential count
    CRED_COUNT=$(grep "credentials_count" edge-mcp-test.log | tail -1 | grep -o '"credentials_count":[0-9]*' | cut -d: -f2)
    if [ ! -z "$CRED_COUNT" ]; then
        echo -e "${GREEN}  ✓ Total credentials extracted: $CRED_COUNT${NC}"
    fi
else
    echo -e "${RED}✗ Pass-through authentication NOT detected${NC}"
    echo "  This may indicate the feature is not working correctly."
fi

echo
echo "Relevant log entries:"
echo "---------------------"
grep -E "(passthrough|token|credential|auth)" edge-mcp-test.log | tail -20 || echo "No auth-related logs found"

# Cleanup
echo
echo "Cleaning up..."
kill $EDGE_PID 2>/dev/null || true
wait $EDGE_PID 2>/dev/null || true

echo
echo "=== Test Complete ==="
echo
echo "To manually verify:"
echo "1. Check edge-mcp-test.log for detailed logs"
echo "2. Set real tokens and test with your IDE"
echo "3. Verify actions are performed as your user (not service account)"
echo

# Show summary
if grep -q "Extracted passthrough authentication" edge-mcp-test.log; then
    echo -e "${GREEN}Result: Pass-through authentication is WORKING${NC}"
    exit 0
else
    echo -e "${RED}Result: Pass-through authentication may NOT be working${NC}"
    echo "Check edge-mcp-test.log for details"
    exit 1
fi