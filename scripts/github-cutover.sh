#!/bin/bash

# GitHub Tools Cutover Script
# This script performs the cutover from static GitHub implementation to dynamic tools

set -euo pipefail

# Configuration
BACKUP_DIR="/tmp/github-cutover-backup-$(date +%Y%m%d-%H%M%S)"
DB_NAME="${DB_NAME:-devops_mcp}"
DB_USER="${DB_USER:-postgres}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging function
log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Pre-flight checks
preflight_check() {
    log "Running pre-flight checks..."
    
    # Check database connection
    if ! psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1" >/dev/null 2>&1; then
        error "Cannot connect to database"
        exit 1
    fi
    
    # Check if migration is needed
    local github_count=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT COUNT(*) FROM tenant_credentials 
        WHERE github_token IS NOT NULL AND github_token != ''
    " 2>/dev/null || echo "0")
    
    if [ "$github_count" -eq "0" ]; then
        warning "No GitHub configurations found to migrate"
    else
        log "Found $github_count GitHub configurations to migrate"
    fi
    
    # Check if target tables exist
    if ! psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
        SELECT 1 FROM tool_configurations LIMIT 1
    " >/dev/null 2>&1; then
        error "tool_configurations table does not exist. Run migration 001 first."
        exit 1
    fi
    
    log "Pre-flight checks passed"
}

# Create backup
create_backup() {
    log "Creating backup in $BACKUP_DIR..."
    mkdir -p "$BACKUP_DIR"
    
    # Backup relevant tables
    tables=("tenant_credentials" "tool_configurations" "tenants")
    
    for table in "${tables[@]}"; do
        if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "\d $table" >/dev/null 2>&1; then
            pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
                --table="$table" \
                --data-only \
                --file="$BACKUP_DIR/${table}.sql"
            log "Backed up table: $table"
        fi
    done
    
    log "Backup completed"
}

# Run migration
run_migration() {
    log "Running GitHub migration..."
    
    # Execute migration SQL
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
        -f "migrations/002_migrate_github_to_dynamic_tools.up.sql"
    
    if [ $? -eq 0 ]; then
        log "Migration completed successfully"
    else
        error "Migration failed"
        return 1
    fi
}

# Verify migration
verify_migration() {
    log "Verifying migration..."
    
    # Check migrated tools count
    local migrated_count=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT COUNT(*) FROM tool_configurations 
        WHERE created_by = 'migration-002'
    ")
    
    log "Migrated $migrated_count GitHub tools"
    
    # Check for any failed migrations
    local failed_count=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT COUNT(*) FROM migration_audit 
        WHERE migration_version = '002_migrate_github' 
        AND status = 'failed'
    " 2>/dev/null || echo "0")
    
    if [ "$failed_count" -gt "0" ]; then
        warning "Found $failed_count failed migrations. Check migration_audit table for details."
        
        # Show failed migrations
        psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
            SELECT tenant_id, details->>'error' as error
            FROM migration_audit 
            WHERE migration_version = '002_migrate_github' 
            AND status = 'failed'
        "
    fi
    
    # Test a migrated tool
    local test_tool=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT tool_name FROM tool_configurations 
        WHERE created_by = 'migration-002' 
        LIMIT 1
    " | xargs)
    
    if [ -n "$test_tool" ]; then
        log "Sample migrated tool: $test_tool"
    fi
}

# Update application configuration
update_app_config() {
    log "Updating application configuration..."
    
    # This would typically update config files or environment variables
    # For now, we'll just log what should be done
    
    cat <<EOF

================================================================================
MANUAL STEPS REQUIRED:
================================================================================

1. Update your application configuration to use the new dynamic tools API:
   - Remove any static GitHub configuration
   - Ensure the dynamic tools endpoints are enabled

2. Update environment variables:
   - Remove GITHUB_TOKEN, GITHUB_WEBHOOK_SECRET if set globally
   - Ensure ENCRYPTION_MASTER_KEY is set for credential encryption

3. Deploy the new application version with dynamic tools support

4. Monitor logs for any compatibility issues with the new endpoints

================================================================================

EOF
}

# Rollback function
rollback() {
    error "Rolling back migration..."
    
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
        -f "migrations/002_migrate_github_to_dynamic_tools.down.sql"
    
    if [ $? -eq 0 ]; then
        log "Rollback completed"
    else
        error "Rollback failed! Manual intervention required."
        error "Backup files are in: $BACKUP_DIR"
    fi
}

# Main execution
main() {
    log "Starting GitHub cutover process..."
    
    # Run pre-flight checks
    preflight_check
    
    # Create backup
    create_backup
    
    # Run migration with error handling
    if run_migration; then
        verify_migration
        update_app_config
        
        log "GitHub cutover completed successfully!"
        log "Backup files saved in: $BACKUP_DIR"
    else
        rollback
        exit 1
    fi
}

# Handle script interruption
trap 'error "Script interrupted"; rollback; exit 1' INT TERM

# Run main function
main "$@"