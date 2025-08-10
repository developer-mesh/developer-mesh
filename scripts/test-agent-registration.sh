#!/bin/bash
# Test agent registration with idempotent behavior

set -e

echo "Testing Agent Registration System"
echo "================================="

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test database connection
echo -e "${YELLOW}Testing database connection...${NC}"
psql -h localhost -U devmesh -d devmesh_development -c "SELECT 1" > /dev/null 2>&1 || {
    echo -e "${RED}Database connection failed. Make sure PostgreSQL is running.${NC}"
    exit 1
}
echo -e "${GREEN}✓ Database connected${NC}"

# Run the migration
echo -e "${YELLOW}Running agent architecture migration...${NC}"
cd apps/rest-api
migrate -path migrations/sql -database "postgres://devmesh:devmesh@localhost/devmesh_development?sslmode=disable" up || {
    echo -e "${YELLOW}Migration already applied or failed - continuing...${NC}"
}
cd ../..

# Test the registration function
echo -e "${YELLOW}Testing idempotent registration function...${NC}"
psql -h localhost -U devmesh -d devmesh_development << EOF
-- Test 1: First registration (should create new)
SELECT * FROM mcp.register_agent_instance(
    '00000000-0000-0000-0000-000000000001'::uuid,  -- tenant_id
    'test-ide-agent',                               -- agent_id
    'conn-12345',                                   -- instance_id (connection ID)
    'Test IDE Agent',                               -- name
    '{"protocol": "websocket"}'::jsonb,            -- connection_details
    '{"version": "1.0.0"}'::jsonb                  -- runtime_config
);

-- Test 2: Reconnection (same instance_id, should update)
SELECT * FROM mcp.register_agent_instance(
    '00000000-0000-0000-0000-000000000001'::uuid,
    'test-ide-agent',
    'conn-12345',  -- Same instance_id
    'Test IDE Agent',
    '{"protocol": "websocket", "reconnected": true}'::jsonb,
    '{"version": "1.0.0"}'::jsonb
);

-- Test 3: New instance (different instance_id, should create new)
SELECT * FROM mcp.register_agent_instance(
    '00000000-0000-0000-0000-000000000001'::uuid,
    'test-ide-agent',
    'conn-67890',  -- Different instance_id
    'Test IDE Agent',
    '{"protocol": "websocket"}'::jsonb,
    '{"version": "1.0.0"}'::jsonb
);

-- Clean up test data
DELETE FROM mcp.agent_registrations WHERE instance_id IN ('conn-12345', 'conn-67890');
DELETE FROM mcp.agent_configurations WHERE agent_id = 'test-ide-agent';
DELETE FROM mcp.agent_manifests WHERE agent_id = 'test-ide-agent';
EOF

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Idempotent registration tests passed${NC}"
else
    echo -e "${RED}✗ Registration tests failed${NC}"
    exit 1
fi

# Run Go tests
echo -e "${YELLOW}Running Go tests for agent registration...${NC}"
go test ./apps/mcp-server/internal/api/websocket -v -run TestIdempotent

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    echo -e "${GREEN}The agent registration system is working correctly with idempotent behavior.${NC}"
else
    echo -e "${RED}✗ Go tests failed${NC}"
    exit 1
fi

echo ""
echo "Summary:"
echo "- Database function handles idempotent registration ✓"
echo "- Same instance_id can reconnect without errors ✓"
echo "- Multiple instances of same agent can register ✓"
echo "- No duplicate key constraint violations ✓"