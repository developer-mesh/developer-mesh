#!/bin/bash

# Test cache through WebSocket to see actual response structure

MCP_WS_URL="ws://localhost:8080/ws"
API_KEY="dev-admin-key-1234567890"
TENANT_ID="00000000-0000-0000-0000-000000000001"

echo "Testing cache through WebSocket..."

# Create a test message
MESSAGE=$(cat <<EOF
{
  "type": 0,
  "id": "$(uuidgen | tr '[:upper:]' '[:lower:]')",
  "method": "tool.execute",
  "params": {
    "agent_id": "test-agent-123",
    "tool_id": "fadd1612-21ac-4825-96c8-5cd66cdf6d91",
    "action": "repos/get",
    "parameters": {
      "owner": "golang",
      "repo": "go"
    }
  }
}
EOF
)

echo "Sending WebSocket request..."
echo "$MESSAGE" | jq -c .

# Send and get response
RESPONSE=$( (echo "$MESSAGE" | jq -c .; sleep 2) | \
  websocat -t -n1 --header="X-API-Key: ${API_KEY}" "$MCP_WS_URL" 2>/dev/null )

echo -e "\nWebSocket Response:"
echo "$RESPONSE" | jq .

# Check for cache indicators
echo -e "\nCache metadata check:"
echo "$RESPONSE" | jq '.from_cache, .cache_hit, .result.from_cache, .result.cache_hit' 2>/dev/null

echo -e "\nDone!"