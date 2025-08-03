#!/bin/bash
# Script to recreate database with new schema

echo "=== Database Recreation Script ==="
echo "This will drop and recreate the database with the new schema"
echo ""

# Configuration
DB_HOST="localhost"
DB_PORT="5432"
DB_USER="dev"
DB_PASS="dev"
DB_NAME="dev"
MIGRATION_PATH="/Users/seancorkum/projects/devops-mcp/apps/rest-api/migrations/sql"

# Step 1: Drop and recreate database
echo "Step 1: Dropping and recreating database..."
PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d postgres <<EOF
DROP DATABASE IF EXISTS $DB_NAME;
CREATE DATABASE $DB_NAME;
EOF

if [ $? -ne 0 ]; then
    echo "Error: Failed to recreate database"
    exit 1
fi

echo "Database recreated successfully"

# Step 2: Apply initial schema
echo ""
echo "Step 2: Applying initial schema..."
PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME < "$MIGRATION_PATH/000001_initial_schema.up.sql"

if [ $? -ne 0 ]; then
    echo "Error: Failed to apply initial schema"
    exit 1
fi

echo "Initial schema applied successfully"

# Step 3: Update migration version
echo ""
echo "Step 3: Setting migration version..."
PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME <<EOF
CREATE TABLE IF NOT EXISTS schema_migrations (
    version bigint not null primary key,
    dirty boolean not null default false
);
INSERT INTO schema_migrations (version, dirty) VALUES (1, false);
EOF

if [ $? -ne 0 ]; then
    echo "Error: Failed to set migration version"
    exit 1
fi

echo "Migration version set successfully"

# Step 4: Verify schema
echo ""
echo "Step 4: Verifying schema..."
echo "Checking key tables and columns..."

PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME <<EOF
-- Check api_keys has key_type column
SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_schema = 'mcp' 
  AND table_name = 'api_keys' 
  AND column_name = 'key_type';

-- Count custom types
SELECT COUNT(*) as custom_types_count
FROM pg_type t
JOIN pg_namespace n ON t.typnamespace = n.oid
WHERE n.nspname = 'mcp' AND t.typtype = 'e';

-- Count tables
SELECT COUNT(*) as table_count
FROM information_schema.tables 
WHERE table_schema = 'mcp';

-- Count functions
SELECT COUNT(*) as function_count
FROM information_schema.routines 
WHERE routine_schema = 'mcp';
EOF

echo ""
echo "=== Database recreation complete ==="
echo "The database has been recreated with the updated schema."
echo "- API keys table now has 'key_type' column instead of 'type'"
echo "- All missing tables, columns, and functions have been added"
echo ""
echo "Next steps:"
echo "1. Restart the application services"
echo "2. Test authentication with the embedding endpoint"