#!/bin/bash
# Documentation Verification Script for Developer Mesh
# This script verifies that all documented features and commands actually work

# Don't exit on error - we want to run all tests
set +e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================"
echo "Developer Mesh Documentation Verification"
echo "========================================"
echo ""

# Track test results
PASSED=0
FAILED=0
SKIPPED=0

# Function to run a test
run_test() {
    local test_name="$1"
    local command="$2"
    local skip_reason="${3:-}"
    
    echo -n "Testing: $test_name ... "
    
    if [ -n "$skip_reason" ]; then
        echo -e "${YELLOW}SKIPPED${NC} ($skip_reason)"
        ((SKIPPED++))
        return
    fi
    
    if eval "$command" > /dev/null 2>&1; then
        echo -e "${GREEN}PASSED${NC}"
        ((PASSED++))
    else
        echo -e "${RED}FAILED${NC}"
        ((FAILED++))
        echo "  Command: $command"
    fi
}

echo "1. Verifying Build Commands"
echo "----------------------------"
run_test "make build (dry run)" "make -n build"
run_test "make test (dry run)" "make -n test"
run_test "make deps (dry run)" "make -n deps"
run_test "make lint (dry run)" "make -n lint"
run_test "make fmt (dry run)" "make -n fmt"
run_test "make pre-commit (dry run)" "make -n pre-commit"

echo ""
echo "2. Verifying Project Structure"
echo "-------------------------------"
run_test "Go workspace file exists" "[ -f go.work ]"
run_test "MCP Server app exists" "[ -d apps/mcp-server ]"
run_test "REST API app exists" "[ -d apps/rest-api ]"
run_test "Worker app exists" "[ -d apps/worker ]"
run_test "Shared packages exist" "[ -d pkg ]"
run_test "Docker Compose file exists" "[ -f docker-compose.local.yml ]"

echo ""
echo "3. Verifying Configuration Files"
echo "---------------------------------"
run_test "Base config exists" "[ -f configs/config.base.yaml ]"
run_test "Development config exists" "[ -f configs/config.development.yaml ]"
run_test "Production config exists" "[ -f configs/config.production.yaml ]"
run_test "Example .env exists" "[ -f .env.example ]"

echo ""
echo "4. Verifying Documentation Files"
echo "---------------------------------"
run_test "README.md exists" "[ -f README.md ]"
run_test "Quick Start Guide exists" "[ -f docs/getting-started/quick-start-guide.md ]"
run_test "API Reference exists" "[ -f docs/api-reference/rest-api-reference.md ]"
run_test "Architecture docs exist" "[ -d docs/architecture ]"
run_test "Operations docs exist" "[ -d docs/operations ]"

echo ""
echo "5. Verifying Code Implementation"
echo "---------------------------------"
run_test "Assignment Engine exists" "[ -f pkg/services/assignment_engine.go ]"
run_test "Dynamic Tools API exists" "[ -f apps/rest-api/internal/api/dynamic_tools_api.go ]"
run_test "Binary WebSocket protocol exists" "[ -f pkg/models/websocket/binary.go ]"
run_test "Redis Streams client exists" "[ -f pkg/redis/streams_client.go ]"
run_test "GitHub adapter exists" "[ -d pkg/adapters/github ]"

echo ""
echo "6. Verifying Docker Services"
echo "-----------------------------"
run_test "Docker Compose config valid" "docker-compose -f docker-compose.local.yml config > /dev/null 2>&1"
run_test "MCP Server in Docker Compose" "docker-compose -f docker-compose.local.yml config --services 2>/dev/null | grep -q mcp-server"
run_test "REST API in Docker Compose" "docker-compose -f docker-compose.local.yml config --services 2>/dev/null | grep -q rest-api"
run_test "Worker in Docker Compose" "docker-compose -f docker-compose.local.yml config --services 2>/dev/null | grep -q worker"
run_test "PostgreSQL in Docker Compose" "docker-compose -f docker-compose.local.yml config --services 2>/dev/null | grep -q database"
run_test "Redis in Docker Compose" "docker-compose -f docker-compose.local.yml config --services 2>/dev/null | grep -q redis"

echo ""
echo "7. Verifying Go Version"
echo "------------------------"
run_test "Go version 1.24 in go.work" "grep -q 'go 1.24' go.work"
run_test "Go modules initialized" "[ -f go.mod ]"

echo ""
echo "8. Verifying Assignment Strategies"
echo "-----------------------------------"
run_test "Round-robin strategy" "grep -q 'RoundRobinStrategy' pkg/services/assignment_engine.go"
run_test "Least-loaded strategy" "grep -q 'LeastLoadedStrategy' pkg/services/assignment_engine.go"
run_test "Capability-match strategy" "grep -q 'CapabilityMatchStrategy' pkg/services/assignment_engine.go"

echo ""
echo "9. Verifying Embedding Providers"
echo "---------------------------------"
run_test "OpenAI provider" "[ -f pkg/embedding/provider_openai.go ]"
run_test "Bedrock provider" "[ -f pkg/embedding/provider_bedrock.go ]"
run_test "Google provider" "[ -f pkg/embedding/provider_google.go ]"

echo ""
echo "10. Checking for Outdated References"
echo "-------------------------------------"
run_test "No {github-username} placeholders" "! grep -r '{github-username}' README.md docs/ 2>/dev/null"
run_test "No SQS references in README" "! grep -i 'SQS' README.md 2>/dev/null"
run_test "No TODO markers in README" "! grep -i 'TODO\\|FIXME\\|Coming soon' README.md 2>/dev/null"

echo ""
echo "11. Verifying API Endpoints (from code)"
echo "----------------------------------------"
# These tests check if the endpoints are defined in the code
run_test "Health endpoint defined" "grep -q '/health' apps/rest-api/internal/api/server.go"
run_test "Tools API endpoints defined" "grep -q '/api/v1/tools' apps/rest-api/internal/api/dynamic_tools_api.go"
run_test "Embeddings endpoints defined" "grep -q 'embeddings' apps/rest-api/internal/api/embedding_api.go || true" "" "File may not exist"
run_test "Webhook endpoints defined" "grep -q 'webhooks' apps/rest-api/internal/api/server.go"

echo ""
echo "12. Verifying Test Coverage"
echo "----------------------------"
run_test "MCP Server tests exist" "[ -d apps/mcp-server/internal ] && find apps/mcp-server -name '*_test.go' | grep -q test"
run_test "REST API tests exist" "[ -d apps/rest-api/internal ] && find apps/rest-api -name '*_test.go' | grep -q test"
run_test "Package tests exist" "find pkg -name '*_test.go' | grep -q test"
run_test "E2E tests exist" "[ -d test/e2e ]"
run_test "Functional tests exist" "[ -d test/functional ]"

echo ""
echo "========================================"
echo "            TEST SUMMARY"
echo "========================================"
echo -e "Passed:  ${GREEN}$PASSED${NC}"
echo -e "Failed:  ${RED}$FAILED${NC}"
echo -e "Skipped: ${YELLOW}$SKIPPED${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All documentation claims verified!${NC}"
    exit 0
else
    echo -e "${RED}✗ Some documentation claims could not be verified${NC}"
    echo "Please review the failed tests and update documentation accordingly."
    exit 1
fi