#!/bin/bash
# Deployment Configuration Validation Script for RAG Loader Multi-Tenant
# This script validates that all required configuration is in place

set -e

echo "🔍 RAG Loader Multi-Tenant Deployment Validation"
echo "=================================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

ERRORS=0
WARNINGS=0

# Function to check if variable is set
check_var() {
    local var_name=$1
    local var_value=$2
    local required=$3

    if [ -z "$var_value" ]; then
        if [ "$required" = "true" ]; then
            echo -e "${RED}✗${NC} $var_name is NOT set (REQUIRED)"
            ((ERRORS++))
        else
            echo -e "${YELLOW}⚠${NC} $var_name is not set (optional)"
            ((WARNINGS++))
        fi
        return 1
    else
        echo -e "${GREEN}✓${NC} $var_name is set"
        return 0
    fi
}

# Function to validate base64 key length
validate_master_key() {
    local key=$1
    if [ -z "$key" ]; then
        echo -e "${RED}✗${NC} RAG_MASTER_KEY is not set"
        ((ERRORS++))
        return 1
    fi

    # Decode and check length
    local decoded_length=$(echo "$key" | base64 -d 2>/dev/null | wc -c | tr -d ' ')
    if [ "$decoded_length" -ne 32 ]; then
        echo -e "${RED}✗${NC} RAG_MASTER_KEY is invalid (decoded length: $decoded_length bytes, expected: 32)"
        ((ERRORS++))
        return 1
    fi

    echo -e "${GREEN}✓${NC} RAG_MASTER_KEY is valid (32 bytes)"
    return 0
}

echo "1. Checking Environment Variables"
echo "----------------------------------"

# Load environment from .env.docker
if [ -f ".env.docker" ]; then
    export $(grep -v '^#' .env.docker | grep -v '^$' | xargs)
    echo -e "${GREEN}✓${NC} Loaded .env.docker"
else
    echo -e "${RED}✗${NC} .env.docker not found"
    ((ERRORS++))
fi

# Check required variables
echo ""
echo "Required Variables:"
validate_master_key "$RAG_MASTER_KEY"
check_var "JWT_SECRET" "$JWT_SECRET" "true"
check_var "RAG_API_ENABLED" "$RAG_API_ENABLED" "true"
check_var "RAG_API_PORT" "$RAG_API_PORT" "true"

echo ""
echo "AWS Configuration:"
check_var "AWS_REGION" "$AWS_REGION" "true"
check_var "AWS_ACCESS_KEY_ID" "$AWS_ACCESS_KEY_ID" "true"
check_var "AWS_SECRET_ACCESS_KEY" "$AWS_SECRET_ACCESS_KEY" "true"

echo ""
echo "2. Checking Docker Compose Configuration"
echo "-----------------------------------------"

# Validate docker-compose
if docker-compose -f docker-compose.local.yml config --services | grep -q rag-loader; then
    echo -e "${GREEN}✓${NC} docker-compose.local.yml is valid"
else
    echo -e "${RED}✗${NC} docker-compose.local.yml validation failed"
    ((ERRORS++))
fi

# Check if RAG loader service is properly configured
RAG_CONFIG=$(docker-compose -f docker-compose.local.yml config 2>&1 | grep -A 30 "rag-loader:")

if echo "$RAG_CONFIG" | grep -q "RAG_API_ENABLED"; then
    echo -e "${GREEN}✓${NC} RAG_API_ENABLED found in docker-compose"
else
    echo -e "${RED}✗${NC} RAG_API_ENABLED not found in docker-compose"
    ((ERRORS++))
fi

# Check in source file instead of interpolated config
if grep -q "RAG_MASTER_KEY=" docker-compose.local.yml; then
    echo -e "${GREEN}✓${NC} RAG_MASTER_KEY found in docker-compose"
else
    echo -e "${RED}✗${NC} RAG_MASTER_KEY not found in docker-compose"
    ((ERRORS++))
fi

# Check that GITHUB_ACCESS_TOKEN is NOT in rag-loader config
if echo "$RAG_CONFIG" | grep -q "GITHUB_ACCESS_TOKEN"; then
    echo -e "${RED}✗${NC} GITHUB_ACCESS_TOKEN found in rag-loader (should be removed)"
    ((ERRORS++))
else
    echo -e "${GREEN}✓${NC} GITHUB_ACCESS_TOKEN correctly removed from rag-loader"
fi

echo ""
echo "3. Checking Migration Files"
echo "----------------------------"

MIGRATIONS_DIR="apps/rag-loader/migrations"
if [ -d "$MIGRATIONS_DIR" ]; then
    echo -e "${GREEN}✓${NC} Migrations directory exists"

    # Check for required migrations
    REQUIRED_MIGRATIONS=("000041_create_tenant_tables" "000042_add_row_level_security" "000043_create_mcp_tenants")
    for migration in "${REQUIRED_MIGRATIONS[@]}"; do
        if ls "$MIGRATIONS_DIR"/${migration}* >/dev/null 2>&1; then
            echo -e "${GREEN}✓${NC} Migration $migration found"
        else
            echo -e "${RED}✗${NC} Migration $migration not found"
            ((ERRORS++))
        fi
    done
else
    echo -e "${RED}✗${NC} Migrations directory not found: $MIGRATIONS_DIR"
    ((ERRORS++))
fi

echo ""
echo "4. Checking Database Connection (if running)"
echo "---------------------------------------------"

if docker-compose -f docker-compose.local.yml ps database 2>/dev/null | grep -q "Up"; then
    echo -e "${GREEN}✓${NC} Database container is running"

    # Test connection
    if docker-compose -f docker-compose.local.yml exec -T database pg_isready -U devmesh -d devmesh_development >/dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} Database is accepting connections"

        # Check if mcp.tenants table exists
        if docker-compose -f docker-compose.local.yml exec -T database psql -U devmesh -d devmesh_development -tAc "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'mcp' AND table_name = 'tenants');" 2>/dev/null | grep -q "t"; then
            echo -e "${GREEN}✓${NC} mcp.tenants table exists"
        else
            echo -e "${YELLOW}⚠${NC} mcp.tenants table does not exist (run migrations)"
            ((WARNINGS++))
        fi
    else
        echo -e "${YELLOW}⚠${NC} Database is not ready (this is OK if not started)"
        ((WARNINGS++))
    fi
else
    echo -e "${YELLOW}⚠${NC} Database container is not running (this is OK)"
    ((WARNINGS++))
fi

echo ""
echo "5. Security Checks"
echo "------------------"

# Check if production keys are being used
if [ "$RAG_MASTER_KEY" = "K5UjoD45dEV/PehMDwar9ORfItM39KtUg5dT+HymK2A=" ]; then
    echo -e "${YELLOW}⚠${NC} Using default RAG_MASTER_KEY (generate new key for production)"
    ((WARNINGS++))
else
    echo -e "${GREEN}✓${NC} Using custom RAG_MASTER_KEY"
fi

if [ "$JWT_SECRET" = "docker-jwt-secret-change-in-production" ] || [ "$JWT_SECRET" = "dev-jwt-secret-minimum-32-characters" ]; then
    echo -e "${YELLOW}⚠${NC} Using default JWT_SECRET (change for production)"
    ((WARNINGS++))
else
    echo -e "${GREEN}✓${NC} Using custom JWT_SECRET"
fi

# Check JWT secret length
if [ ${#JWT_SECRET} -lt 32 ]; then
    echo -e "${RED}✗${NC} JWT_SECRET is too short (${#JWT_SECRET} chars, minimum 32)"
    ((ERRORS++))
else
    echo -e "${GREEN}✓${NC} JWT_SECRET length is acceptable (${#JWT_SECRET} chars)"
fi

echo ""
echo "=================================================="
echo "Validation Results:"
echo ""
echo -e "Errors:   ${RED}$ERRORS${NC}"
echo -e "Warnings: ${YELLOW}$WARNINGS${NC}"
echo ""

if [ $ERRORS -eq 0 ]; then
    echo -e "${GREEN}✓ Deployment configuration is valid!${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Review warnings above"
    echo "2. For production: Generate unique keys"
    echo "3. Start services: docker-compose -f docker-compose.local.yml up -d"
    echo "4. Apply migrations (automatic on startup)"
    echo "5. Test API: curl http://localhost:9094/health"
    exit 0
else
    echo -e "${RED}✗ Deployment configuration has errors!${NC}"
    echo ""
    echo "Please fix the errors above before deploying."
    exit 1
fi
