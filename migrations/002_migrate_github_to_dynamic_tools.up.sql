-- Migration: Migrate GitHub Configurations to Dynamic Tools
-- Version: 002
-- Date: 2025-01-27
-- Description: Migrates existing GitHub configurations to the dynamic tools system

BEGIN;

-- Create audit table for migration tracking
CREATE TABLE IF NOT EXISTS migration_audit (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    migration_version VARCHAR(50) NOT NULL,
    tenant_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL,
    details JSONB,
    executed_at TIMESTAMP DEFAULT NOW()
);

-- Create encryption function for credentials
CREATE OR REPLACE FUNCTION encrypt_credential(
    p_tenant_id UUID,
    p_token TEXT
) RETURNS BYTEA AS $$
DECLARE
    v_key BYTEA;
    v_nonce BYTEA;
    v_encrypted BYTEA;
BEGIN
    -- Generate tenant-specific key using SHA256
    -- In production, this should use a proper key derivation function
    v_key := decode(md5(p_tenant_id::text || 'encryption_salt_change_in_prod'), 'hex');
    
    -- For this migration, we'll store as base64 encoded
    -- In production, use proper encryption
    RETURN encode(p_token::bytea, 'base64')::bytea;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Check if we have any GitHub configurations to migrate
DO $$
DECLARE
    v_count INTEGER;
BEGIN
    -- Check if tenant_credentials table exists
    IF EXISTS (SELECT 1 FROM information_schema.tables 
               WHERE table_schema = 'public' 
               AND table_name = 'tenant_credentials') THEN
        
        -- Count GitHub configurations
        SELECT COUNT(*) INTO v_count
        FROM tenant_credentials
        WHERE github_token IS NOT NULL AND github_token != '';
        
        RAISE NOTICE 'Found % GitHub configurations to migrate', v_count;
    ELSE
        RAISE NOTICE 'No tenant_credentials table found - skipping migration';
    END IF;
END $$;

-- Migrate GitHub credentials if they exist
DO $$
DECLARE
    v_tenant RECORD;
    v_success_count INT := 0;
    v_failure_count INT := 0;
    v_tool_id UUID;
BEGIN
    -- Check if source table exists
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables 
                   WHERE table_schema = 'public' 
                   AND table_name = 'tenant_credentials') THEN
        RAISE NOTICE 'No tenant_credentials table found - nothing to migrate';
        RETURN;
    END IF;
    
    -- Migrate each tenant's GitHub configuration
    FOR v_tenant IN 
        SELECT 
            t.id as tenant_id,
            t.name as tenant_name,
            tc.github_token,
            tc.github_webhook_secret,
            tc.github_enterprise_url
        FROM tenants t
        INNER JOIN tenant_credentials tc ON t.id = tc.tenant_id
        WHERE tc.github_token IS NOT NULL 
        AND tc.github_token != ''
    LOOP
        BEGIN
            v_tool_id := gen_random_uuid();
            
            -- Determine tool type and base URL
            DECLARE
                v_tool_type VARCHAR(50);
                v_base_url TEXT;
                v_tool_name VARCHAR(100);
                v_display_name VARCHAR(200);
            BEGIN
                IF v_tenant.github_enterprise_url IS NOT NULL AND v_tenant.github_enterprise_url != '' THEN
                    v_tool_type := 'github-enterprise';
                    v_base_url := v_tenant.github_enterprise_url;
                    v_tool_name := 'github-enterprise-default';
                    v_display_name := 'GitHub Enterprise';
                ELSE
                    v_tool_type := 'github';
                    v_base_url := 'https://api.github.com';
                    v_tool_name := 'github-default';
                    v_display_name := 'GitHub';
                END IF;
                
                -- Check if tool already exists for this tenant
                IF EXISTS (SELECT 1 FROM tool_configurations 
                          WHERE tenant_id = v_tenant.tenant_id 
                          AND tool_name = v_tool_name) THEN
                    RAISE NOTICE 'Tool % already exists for tenant %, skipping', v_tool_name, v_tenant.tenant_id;
                    CONTINUE;
                END IF;
                
                -- Insert tool configuration
                INSERT INTO tool_configurations (
                    id,
                    tenant_id,
                    tool_name,
                    display_name,
                    config,
                    credentials_encrypted,
                    auth_type,
                    retry_policy,
                    status,
                    health_status,
                    created_by
                ) VALUES (
                    v_tool_id,
                    v_tenant.tenant_id,
                    v_tool_name,
                    v_display_name,
                    jsonb_build_object(
                        'base_url', v_base_url,
                        'api_version', 'v3',
                        'webhook_secret', v_tenant.github_webhook_secret,
                        'migrated_from', 'static_config',
                        'migration_time', NOW(),
                        'is_enterprise', (v_tool_type = 'github-enterprise'),
                        'tool_type', v_tool_type
                    ),
                    encrypt_credential(v_tenant.tenant_id, v_tenant.github_token),
                    'token',
                    jsonb_build_object(
                        'max_attempts', 3,
                        'initial_delay', '1s',
                        'max_delay', '30s',
                        'multiplier', 2.0,
                        'jitter', 0.1,
                        'retry_on_timeout', true,
                        'retry_on_rate_limit', true
                    ),
                    'active',
                    'unknown',
                    'migration-002'
                );
                
                v_success_count := v_success_count + 1;
                
                -- Log success
                INSERT INTO migration_audit (
                    migration_version, 
                    tenant_id, 
                    status,
                    details
                ) VALUES (
                    '002_migrate_github',
                    v_tenant.tenant_id,
                    'success',
                    jsonb_build_object(
                        'tool_id', v_tool_id,
                        'tool_name', v_tool_name,
                        'tool_type', v_tool_type
                    )
                );
            END;
            
        EXCEPTION WHEN OTHERS THEN
            v_failure_count := v_failure_count + 1;
            
            -- Log failure
            INSERT INTO migration_audit (
                migration_version,
                tenant_id,
                status,
                details
            ) VALUES (
                '002_migrate_github',
                v_tenant.tenant_id,
                'failed',
                jsonb_build_object(
                    'error', SQLERRM,
                    'error_detail', SQLSTATE
                )
            );
            
            RAISE WARNING 'Failed to migrate GitHub for tenant %: %', v_tenant.tenant_id, SQLERRM;
        END;
    END LOOP;
    
    -- Log summary
    RAISE NOTICE 'Migration completed: % successful, % failed', v_success_count, v_failure_count;
    
    -- If any failures, raise exception to rollback
    IF v_failure_count > 0 THEN
        RAISE EXCEPTION 'Migration failed for % tenants', v_failure_count;
    END IF;
END $$;

-- Create view for GitHub tools
CREATE OR REPLACE VIEW github_tools AS
SELECT 
    tc.id,
    tc.tenant_id,
    tc.tool_name,
    tc.display_name,
    tc.config->>'base_url' as base_url,
    tc.config->>'is_enterprise' as is_enterprise,
    tc.status,
    tc.health_status,
    tc.last_health_check,
    tc.created_at,
    tc.updated_at
FROM tool_configurations tc
WHERE tc.config->>'tool_type' IN ('github', 'github-enterprise')
   OR tc.tool_name LIKE 'github%';

-- Add indexes for performance
CREATE INDEX IF NOT EXISTS idx_tool_config_type 
ON tool_configurations ((config->>'tool_type'));

CREATE INDEX IF NOT EXISTS idx_tool_config_base_url 
ON tool_configurations ((config->>'base_url'));

-- Verify migration
DO $$
DECLARE
    v_migrated_count INTEGER;
    v_total_expected INTEGER;
BEGIN
    -- Count migrated tools
    SELECT COUNT(*) INTO v_migrated_count
    FROM tool_configurations
    WHERE created_by = 'migration-002';
    
    -- Count expected migrations (if source table exists)
    IF EXISTS (SELECT 1 FROM information_schema.tables 
               WHERE table_schema = 'public' 
               AND table_name = 'tenant_credentials') THEN
        
        SELECT COUNT(*) INTO v_total_expected
        FROM tenant_credentials
        WHERE github_token IS NOT NULL AND github_token != '';
        
        RAISE NOTICE 'Migration verification: % tools migrated out of % expected', 
                     v_migrated_count, v_total_expected;
        
        -- Check for any missing migrations
        IF v_migrated_count < v_total_expected THEN
            RAISE WARNING 'Some GitHub configurations were not migrated. Check migration_audit table for details.';
        END IF;
    END IF;
END $$;

-- Grant permissions
GRANT SELECT ON github_tools TO PUBLIC;
GRANT SELECT, INSERT, UPDATE ON migration_audit TO PUBLIC;

COMMIT;