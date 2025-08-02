BEGIN;

-- Drop tenant configuration table
DROP TABLE IF EXISTS mcp.tenant_config;

-- Remove columns from api_keys
ALTER TABLE mcp.api_keys 
DROP COLUMN IF EXISTS key_type,
DROP COLUMN IF EXISTS parent_key_id,
DROP COLUMN IF EXISTS allowed_services;

-- Drop indexes
DROP INDEX IF EXISTS mcp.idx_api_keys_type;
DROP INDEX IF EXISTS mcp.idx_api_keys_parent;

COMMIT;