#!/bin/bash

# Test cache functionality directly

API_URL="http://localhost:8081"
API_KEY="dev-admin-key-1234567890"
TENANT_ID="00000000-0000-0000-0000-000000000001"
TOOL_ID="fadd1612-21ac-4825-96c8-5cd66cdf6d91"  # GitHub tool

echo "Testing cache functionality..."

# Make first request (should be cache miss)
echo -e "\n1. First request (cold cache):"
RESPONSE1=$(curl -s -X POST "${API_URL}/api/v1/tools/${TOOL_ID}/execute" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Tenant-ID: ${TENANT_ID}" \
  -H "Content-Type: application/json" \
  -d '{
    "action": "repos/get",
    "parameters": {
      "owner": "golang",
      "repo": "go"
    }
  }')

echo "$RESPONSE1" | jq -r '.from_cache, .cache_hit' 2>/dev/null | grep -q true && echo "Cache HIT" || echo "Cache MISS"

# Wait a moment
sleep 1

# Make second request (should be cache hit)
echo -e "\n2. Second request (warm cache):"
RESPONSE2=$(curl -s -X POST "${API_URL}/api/v1/tools/${TOOL_ID}/execute" \
  -H "X-API-Key: ${API_KEY}" \
  -H "X-Tenant-ID: ${TENANT_ID}" \
  -H "Content-Type: application/json" \
  -d '{
    "action": "repos/get",
    "parameters": {
      "owner": "golang",
      "repo": "go"
    }
  }')

echo "$RESPONSE2" | jq -r '.from_cache, .cache_hit' 2>/dev/null | grep -q true && echo "Cache HIT" || echo "Cache MISS"

# Check database
echo -e "\n3. Cache entries in database:"
docker exec devops-mcp-database-1 psql -U devmesh -d devmesh_development -c \
  "SELECT tool_id, action, hit_count FROM mcp.cache_entries WHERE tenant_id = '${TENANT_ID}';" 2>/dev/null

echo -e "\nDone!"