#!/bin/bash
# Environment Validation Script for Developer Mesh
# This script validates that all required environment variables and dependencies are properly configured

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Counters
ERRORS=0
WARNINGS=0

# Helper functions
error() {
    echo -e "${RED}❌ ERROR: $1${NC}"
    ERRORS=$((ERRORS + 1))
}

warning() {
    echo -e "${YELLOW}⚠️  WARNING: $1${NC}"
    WARNINGS=$((WARNINGS + 1))
}

success() {
    echo -e "${GREEN}✅ $1${NC}"
}

info() {
    echo -e "ℹ️  $1"
}

# Check if we're in the right directory
if [ ! -f "Makefile" ] || [ ! -d "apps" ]; then
    error "Must run from project root directory"
    exit 1
fi

echo "🔍 Validating Developer Mesh Environment"
echo "========================================"
echo ""

# 1. Check required tools
echo "1. Checking required tools..."
REQUIRED_TOOLS=(
    "docker:Docker is required for local development"
    "docker-compose:Docker Compose is required for service orchestration"
    "psql:PostgreSQL client is required for database operations"
    "redis-cli:Redis client is required for cache operations"
    "curl:curl is required for health checks"
    "jq:jq is recommended for JSON parsing"
)

for tool_desc in "${REQUIRED_TOOLS[@]}"; do
    IFS=':' read -r tool description <<< "$tool_desc"
    if command -v "$tool" >/dev/null 2>&1; then
        success "$tool found"
    else
        if [[ "$tool" == "jq" ]]; then
            warning "$description"
        else
            error "$description"
        fi
    fi
done

# 2. Check environment variables based on current environment
echo ""
echo "2. Checking environment variables..."

# Determine environment
ENVIRONMENT="${ENVIRONMENT:-development}"
info "Current environment: $ENVIRONMENT"

# Common required variables
COMMON_VARS=(
    "ADMIN_API_KEY:API key for authentication"
    "AWS_REGION:AWS region for services"
)

# Environment-specific variables
if [ -f ".env.local" ] || [ "$ENVIRONMENT" = "local" ]; then
    info "Local/Docker environment detected"
    REQUIRED_VARS=(
        "${COMMON_VARS[@]}"
    )
else
    info "AWS environment detected"
    REQUIRED_VARS=(
        "${COMMON_VARS[@]}"
        "DATABASE_HOST:Database host (should be localhost for tunnel)"
        "DATABASE_USER:Database username"
        "DATABASE_PASSWORD:Database password"
        "DATABASE_NAME:Database name"
        "SSH_KEY_PATH:SSH key path for tunnels"
        "NAT_INSTANCE_IP:NAT instance IP for SSH tunnels"
        "RDS_ENDPOINT:RDS endpoint for tunnel"
        "ELASTICACHE_ENDPOINT:ElastiCache endpoint for tunnel"
        "AWS_ACCESS_KEY_ID:AWS access key"
        "AWS_SECRET_ACCESS_KEY:AWS secret key"
    )
fi

for var_desc in "${REQUIRED_VARS[@]}"; do
    IFS=':' read -r var description <<< "$var_desc"
    if [ -n "${!var}" ]; then
        if [[ "$var" == *"PASSWORD"* ]] || [[ "$var" == *"SECRET"* ]] || [[ "$var" == *"KEY"* ]]; then
            success "$var is set (hidden)"
        else
            success "$var = ${!var}"
        fi
    else
        error "$var is not set - $description"
    fi
done

# 3. Check Docker services (if applicable)
echo ""
echo "3. Checking Docker services..."

if [ "$ENVIRONMENT" = "local" ] || [ -f ".env.local" ]; then
    if docker info >/dev/null 2>&1; then
        success "Docker daemon is running"
        
        # Check if docker-compose file exists
        if [ -f "docker-compose.local.yml" ]; then
            success "docker-compose.local.yml found"
        else
            error "docker-compose.local.yml not found"
        fi
    else
        error "Docker daemon is not running"
    fi
else
    info "Skipping Docker checks for AWS environment"
fi

# 4. Check network connectivity
echo ""
echo "4. Checking network connectivity..."

# Check localhost ports
PORTS_TO_CHECK=(
    "8080:MCP Server"
    "8081:REST API"
    "5432:PostgreSQL"
    "6379:Redis"
)

for port_desc in "${PORTS_TO_CHECK[@]}"; do
    IFS=':' read -r port service <<< "$port_desc"
    if nc -z localhost "$port" 2>/dev/null; then
        success "Port $port is accessible ($service)"
    else
        info "Port $port is not in use ($service) - service may not be running"
    fi
done

# 5. Check configuration files
echo ""
echo "5. Checking configuration files..."

CONFIG_FILES=(
    "configs/config.base.yaml"
    "configs/config.development.yaml"
    "configs/config.docker.yaml"
)

for config in "${CONFIG_FILES[@]}"; do
    if [ -f "$config" ]; then
        success "$config exists"
    else
        error "$config not found"
    fi
done

# 6. Check test configuration
echo ""
echo "6. Checking E2E test configuration..."

if [ -f "test/e2e/.env.local" ]; then
    success "test/e2e/.env.local exists"
elif [ -f "test/e2e/.env" ]; then
    success "test/e2e/.env exists"
else
    warning "No E2E test configuration found - run 'make test-e2e-setup'"
fi

# Check if ginkgo is installed
if command -v ginkgo >/dev/null 2>&1; then
    success "ginkgo test runner found"
else
    warning "ginkgo not installed - E2E tests will use 'go test' fallback"
fi

# 7. AWS specific checks
if [ "$USE_REAL_AWS" = "true" ] || [ "$ENVIRONMENT" != "local" ]; then
    echo ""
    echo "7. Checking AWS configuration..."
    
    # Check AWS CLI
    if command -v aws >/dev/null 2>&1; then
        success "AWS CLI found"
        
        # Check AWS credentials
        if aws sts get-caller-identity >/dev/null 2>&1; then
            success "AWS credentials are valid"
        else
            error "AWS credentials are not valid or not configured"
        fi
    else
        warning "AWS CLI not found - some operations may fail"
    fi
    
    # Check SSH key for tunnels
    if [ -n "$SSH_KEY_PATH" ]; then
        SSH_KEY_EXPANDED=$(eval echo "$SSH_KEY_PATH")
        if [ -f "$SSH_KEY_EXPANDED" ]; then
            success "SSH key found at $SSH_KEY_EXPANDED"
            
            # Check permissions
            PERMS=$(stat -f "%OLp" "$SSH_KEY_EXPANDED" 2>/dev/null || stat -c "%a" "$SSH_KEY_EXPANDED" 2>/dev/null)
            if [ "$PERMS" = "600" ] || [ "$PERMS" = "400" ]; then
                success "SSH key has correct permissions ($PERMS)"
            else
                warning "SSH key permissions are $PERMS (should be 600 or 400)"
            fi
        else
            error "SSH key not found at $SSH_KEY_EXPANDED"
        fi
    fi
fi

# Summary
echo ""
echo "========================================"
echo "Validation Summary:"
echo "  Errors:   $ERRORS"
echo "  Warnings: $WARNINGS"

if [ $ERRORS -eq 0 ]; then
    if [ $WARNINGS -eq 0 ]; then
        success "Environment validation passed!"
    else
        success "Environment validation passed with warnings"
    fi
    exit 0
else
    error "Environment validation failed!"
    echo ""
    echo "To fix these issues:"
    echo "  1. Ensure all required tools are installed"
    echo "  2. Check your .env file has all required variables"
    echo "  3. For Docker environment: run 'make env-local'"
    echo "  4. For AWS environment: run 'make env-aws'"
    echo "  5. Run 'make test-e2e-setup' to configure E2E tests"
    exit 1
fi