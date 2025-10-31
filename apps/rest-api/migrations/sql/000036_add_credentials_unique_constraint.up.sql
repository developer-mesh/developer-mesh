-- Migration: Add unique constraint to tenant_source_credentials
-- This ensures ON CONFLICT works properly for credential upserts
-- Required for proper credential management and deduplication

-- Add unique constraint if it doesn't already exist
DO $$
BEGIN
    -- Check if the constraint already exists
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'tenant_source_credentials_tenant_source_type_key'
        AND conrelid = 'rag.tenant_source_credentials'::regclass
    ) THEN
        -- Add the unique constraint
        ALTER TABLE rag.tenant_source_credentials
        ADD CONSTRAINT tenant_source_credentials_tenant_source_type_key
        UNIQUE (tenant_id, source_id, credential_type);
    END IF;
END $$;

-- Comment on the constraint for documentation
COMMENT ON CONSTRAINT tenant_source_credentials_tenant_source_type_key
ON rag.tenant_source_credentials IS
'Ensures each tenant can only have one credential of each type per source. Required for ON CONFLICT operations.';
