#!/bin/bash
# Migration script from SQS to Redis

set -e

echo "=== DevOps MCP: SQS to Redis Migration Script ==="
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_step() {
    echo -e "${GREEN}[STEP]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running in production
if [[ "$ENVIRONMENT" == "production" ]]; then
    print_warning "Running in production mode. Are you sure you want to continue? (yes/no)"
    read -r response
    if [[ "$response" != "yes" ]]; then
        echo "Migration cancelled."
        exit 0
    fi
fi

# Step 1: Check current configuration
print_step "Checking current queue configuration..."
CURRENT_QUEUE_TYPE=${QUEUE_TYPE:-sqs}
echo "Current queue type: $CURRENT_QUEUE_TYPE"

if [[ "$CURRENT_QUEUE_TYPE" == "redis" ]]; then
    print_warning "Already using Redis. Migration may not be needed."
fi

# Step 2: Verify Redis connectivity
print_step "Verifying Redis connectivity..."
REDIS_ADDRESS=${REDIS_ADDRESS:-localhost:6379}
echo "Redis address: $REDIS_ADDRESS"

# Test Redis connection
if redis-cli -h ${REDIS_ADDRESS%:*} -p ${REDIS_ADDRESS#*:} ping > /dev/null 2>&1; then
    echo "✓ Redis is accessible"
else
    print_error "Cannot connect to Redis at $REDIS_ADDRESS"
    exit 1
fi

# Step 3: Create backup of current configuration
print_step "Creating configuration backup..."
CONFIG_BACKUP_DIR="config-backup-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$CONFIG_BACKUP_DIR"

# Backup environment variables
env | grep -E "(SQS|AWS|QUEUE)" > "$CONFIG_BACKUP_DIR/env-vars.txt" || true
echo "✓ Environment variables backed up to $CONFIG_BACKUP_DIR/env-vars.txt"

# Step 4: Update environment variables
print_step "Updating environment variables..."

# Create new environment file
cat > .env.redis << EOF
# Queue Configuration
QUEUE_TYPE=redis

# Redis Configuration
REDIS_ADDRESS=${REDIS_ADDRESS}
REDIS_PASSWORD=${REDIS_PASSWORD}
REDIS_TLS_ENABLED=${REDIS_TLS_ENABLED:-false}

# Redis Streams Configuration
REDIS_STREAM_NAME=webhook-events
REDIS_CONSUMER_GROUP=webhook-processors

# Worker Configuration
WORKER_COUNT=${WORKER_COUNT:-10}
WORKER_BATCH_SIZE=${WORKER_BATCH_SIZE:-100}

# Disable SQS (kept for rollback)
# SQS_QUEUE_URL=${SQS_QUEUE_URL}
# AWS_REGION=${AWS_REGION}
EOF

echo "✓ New configuration saved to .env.redis"

# Step 5: Test with dry run
print_step "Testing configuration (dry run)..."
export QUEUE_TYPE=redis
export DRY_RUN=true

# Test webhook handler
echo "Testing webhook handler with Redis..."
# Add your test command here

# Test worker with Redis
echo "Testing worker with Redis..."
# Add your test command here

# Step 6: Migration checklist
print_step "Pre-migration checklist:"
echo ""
echo "[ ] 1. All tests pass with Redis configuration"
echo "[ ] 2. Monitoring dashboards updated"
echo "[ ] 3. Alerts configured for Redis"
echo "[ ] 4. Team notified of migration"
echo "[ ] 5. Rollback plan reviewed"
echo ""

print_warning "Complete the checklist above before proceeding with production migration."
echo ""

# Step 7: Apply migration
echo "To apply the migration:"
echo "1. Source the new environment: source .env.redis"
echo "2. Restart webhook handlers"
echo "3. Stop SQS workers"
echo "4. Start Redis workers"
echo "5. Monitor for errors"
echo ""

# Step 8: Verification commands
print_step "Verification commands:"
echo ""
echo "# Check Redis stream status:"
echo "redis-cli XINFO STREAM webhook-events"
echo ""
echo "# Monitor Redis stream:"
echo "redis-cli XREAD BLOCK 0 STREAMS webhook-events $"
echo ""
echo "# Check consumer group:"
echo "redis-cli XINFO GROUPS webhook-events"
echo ""

# Step 9: Rollback instructions
print_step "Rollback instructions (if needed):"
echo ""
echo "1. export QUEUE_TYPE=sqs"
echo "2. Restart all services"
echo "3. Restore from backup: source $CONFIG_BACKUP_DIR/env-vars.txt"
echo ""

print_step "Migration preparation complete!"
echo ""
echo "Review the instructions above and proceed when ready."