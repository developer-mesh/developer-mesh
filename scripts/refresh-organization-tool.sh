#!/bin/bash

# Script to refresh an organization tool with the latest provider capabilities
# This updates the tool template with current provider operations

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8081}"
ORG_ID="${ORG_ID:-$1}"
TOOL_ID="${TOOL_ID:-$2}"
API_KEY="${API_KEY:-dev-admin-key-1234567890}"

# Check arguments
if [ -z "$ORG_ID" ] || [ -z "$TOOL_ID" ]; then
    echo "Usage: $0 <organization_id> <tool_id>"
    echo "Or set ORG_ID and TOOL_ID environment variables"
    exit 1
fi

echo "Refreshing organization tool..."
echo "Organization ID: $ORG_ID"
echo "Tool ID: $TOOL_ID"
echo ""

# Make the refresh request
response=$(curl -s -X PUT \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    "${BASE_URL}/api/v1/organizations/${ORG_ID}/tools/${TOOL_ID}?action=refresh")

# Check if request was successful
if echo "$response" | grep -q '"message":"Tool refreshed successfully"'; then
    echo "✅ Tool refreshed successfully!"
    echo "$response" | jq '.' 2>/dev/null || echo "$response"
else
    echo "❌ Failed to refresh tool"
    echo "$response" | jq '.' 2>/dev/null || echo "$response"
    exit 1
fi

echo ""
echo "To verify the refresh, list the tool's operations:"
echo "curl -H 'Authorization: Bearer $API_KEY' '${BASE_URL}/api/v1/organizations/${ORG_ID}/tools/${TOOL_ID}/actions'"