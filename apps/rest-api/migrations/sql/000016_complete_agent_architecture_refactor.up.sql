-- ================================================================
-- Complete Agent Architecture Refactor to Desired State
-- ================================================================
-- This migration completely refactors the agent system to properly separate:
-- 1. Agent Types (manifests) - What kinds of agents exist
-- 2. Agent Configurations - How tenants configure each agent type  
-- 3. Agent Instances (registrations) - Which instances are running
--
-- This fixes the duplicate key constraint issue and enables:
-- - Multiple instances of the same agent
-- - Graceful reconnections
-- - Kubernetes pod cycling
-- - Horizontal scaling
-- ================================================================

BEGIN;

-- ================================================================
-- STEP 1: Create New Architecture Tables
-- ================================================================

-- Agent configurations table (tenant-specific settings for each agent type)
CREATE TABLE IF NOT EXISTS mcp.agent_configurations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    manifest_id UUID NOT NULL REFERENCES mcp.agent_manifests(id) ON DELETE CASCADE,
    
    -- Configuration settings
    name VARCHAR(255) NOT NULL, -- Display name for this configuration
    enabled BOOLEAN DEFAULT true,
    configuration JSONB DEFAULT '{}',
    system_prompt TEXT,
    temperature FLOAT DEFAULT 0.7 CHECK (temperature >= 0 AND temperature <= 2),
    max_tokens INTEGER DEFAULT 4096,
    model_id VARCHAR(255),
    
    -- Workload management
    max_workload INTEGER DEFAULT 10,
    current_workload INTEGER DEFAULT 0,
    
    -- Metadata
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Unique constraint: one config per agent type per tenant
    UNIQUE(tenant_id, manifest_id)
);

-- Add indexes for performance
CREATE INDEX idx_agent_configurations_tenant ON mcp.agent_configurations(tenant_id);
CREATE INDEX idx_agent_configurations_manifest ON mcp.agent_configurations(manifest_id);
CREATE INDEX idx_agent_configurations_enabled ON mcp.agent_configurations(enabled) WHERE enabled = true;

-- ================================================================
-- STEP 2: Migrate Existing Data
-- ================================================================

-- First, ensure all agents have corresponding manifests
INSERT INTO mcp.agent_manifests (
    id,
    agent_id,
    agent_type,
    name,
    version,
    description,
    capabilities,
    status,
    created_at,
    updated_at
)
SELECT 
    uuid_generate_v4() as id,
    a.id::text as agent_id,
    COALESCE(a.type, 'standard') as agent_type,
    a.name,
    '1.0.0' as version,
    'Migrated from agents table' as description,
    jsonb_build_object('capabilities', a.capabilities) as capabilities,
    'active' as status,
    a.created_at,
    a.updated_at
FROM mcp.agents a
WHERE NOT EXISTS (
    SELECT 1 FROM mcp.agent_manifests m 
    WHERE m.agent_id = a.id::text
)
ON CONFLICT (agent_id) DO UPDATE
SET 
    updated_at = EXCLUDED.updated_at,
    capabilities = EXCLUDED.capabilities;

-- Create configurations from existing agents
INSERT INTO mcp.agent_configurations (
    id,
    tenant_id,
    manifest_id,
    name,
    enabled,
    configuration,
    system_prompt,
    temperature,
    max_tokens,
    model_id,
    max_workload,
    current_workload,
    created_at,
    updated_at
)
SELECT 
    a.id, -- Preserve the original agent ID as config ID for FK compatibility
    a.tenant_id,
    m.id as manifest_id,
    a.name,
    CASE WHEN a.status IN ('active', 'available') THEN true ELSE false END,
    COALESCE(a.configuration, '{}'::jsonb),
    a.system_prompt,
    a.temperature,
    a.max_tokens,
    a.model_id,
    a.max_workload,
    a.current_workload,
    a.created_at,
    a.updated_at
FROM mcp.agents a
JOIN mcp.agent_manifests m ON m.agent_id = a.id::text
ON CONFLICT (tenant_id, manifest_id) 
DO UPDATE SET
    name = EXCLUDED.name,
    configuration = EXCLUDED.configuration,
    updated_at = NOW();

-- Create active registrations for agents that have been seen recently
INSERT INTO mcp.agent_registrations (
    manifest_id,
    tenant_id,
    instance_id,
    registration_status,
    health_status,
    activation_date,
    last_health_check,
    created_at,
    updated_at
)
SELECT 
    m.id as manifest_id,
    a.tenant_id,
    'migrated-' || a.id::text as instance_id,
    'active' as registration_status,
    CASE 
        WHEN a.last_seen_at > NOW() - INTERVAL '5 minutes' THEN 'healthy'
        WHEN a.last_seen_at > NOW() - INTERVAL '1 hour' THEN 'degraded'
        ELSE 'unknown'
    END as health_status,
    COALESCE(a.created_at, NOW()) as activation_date,
    a.last_seen_at as last_health_check,
    a.created_at,
    a.updated_at
FROM mcp.agents a
JOIN mcp.agent_manifests m ON m.agent_id = a.id::text
WHERE a.last_seen_at IS NOT NULL
  AND a.last_seen_at > NOW() - INTERVAL '24 hours'
ON CONFLICT (tenant_id, instance_id) DO NOTHING;

-- ================================================================
-- STEP 3: Update Foreign Key References
-- ================================================================

-- Drop existing foreign key constraints and recreate them pointing to agent_configurations
-- This maintains compatibility while moving to the new model

-- contexts table
ALTER TABLE mcp.contexts 
    DROP CONSTRAINT IF EXISTS contexts_agent_id_fkey;
ALTER TABLE mcp.contexts 
    ADD CONSTRAINT contexts_agent_id_fkey 
    FOREIGN KEY (agent_id) REFERENCES mcp.agent_configurations(id) ON DELETE CASCADE;

-- agent_configs table 
ALTER TABLE mcp.agent_configs 
    DROP CONSTRAINT IF EXISTS agent_configs_agent_id_fkey;
ALTER TABLE mcp.agent_configs 
    ADD CONSTRAINT agent_configs_agent_id_fkey 
    FOREIGN KEY (agent_id) REFERENCES mcp.agent_configurations(id) ON DELETE CASCADE;

-- tasks table
ALTER TABLE mcp.tasks 
    DROP CONSTRAINT IF EXISTS tasks_agent_id_fkey;
ALTER TABLE mcp.tasks 
    ADD CONSTRAINT tasks_agent_id_fkey 
    FOREIGN KEY (agent_id) REFERENCES mcp.agent_configurations(id);

-- task_delegations table
ALTER TABLE mcp.task_delegations 
    DROP CONSTRAINT IF EXISTS task_delegations_from_agent_id_fkey;
ALTER TABLE mcp.task_delegations 
    ADD CONSTRAINT task_delegations_from_agent_id_fkey 
    FOREIGN KEY (from_agent_id) REFERENCES mcp.agent_configurations(id);
    
ALTER TABLE mcp.task_delegations 
    DROP CONSTRAINT IF EXISTS task_delegations_to_agent_id_fkey;
ALTER TABLE mcp.task_delegations 
    ADD CONSTRAINT task_delegations_to_agent_id_fkey 
    FOREIGN KEY (to_agent_id) REFERENCES mcp.agent_configurations(id);

-- task_delegation_history table
ALTER TABLE mcp.task_delegation_history 
    DROP CONSTRAINT IF EXISTS task_delegation_history_from_agent_id_fkey;
ALTER TABLE mcp.task_delegation_history 
    ADD CONSTRAINT task_delegation_history_from_agent_id_fkey 
    FOREIGN KEY (from_agent_id) REFERENCES mcp.agent_configurations(id);
    
ALTER TABLE mcp.task_delegation_history 
    DROP CONSTRAINT IF EXISTS task_delegation_history_to_agent_id_fkey;
ALTER TABLE mcp.task_delegation_history 
    ADD CONSTRAINT task_delegation_history_to_agent_id_fkey 
    FOREIGN KEY (to_agent_id) REFERENCES mcp.agent_configurations(id);

-- task_state_transitions table (renamed from task_status_history)
ALTER TABLE mcp.task_state_transitions 
    DROP CONSTRAINT IF EXISTS task_state_transitions_transitioned_by_fkey;
ALTER TABLE mcp.task_state_transitions 
    ADD CONSTRAINT task_state_transitions_transitioned_by_fkey 
    FOREIGN KEY (transitioned_by) REFERENCES mcp.agent_configurations(id);

-- workspace_activities table
ALTER TABLE mcp.workspace_activities 
    DROP CONSTRAINT IF EXISTS workspace_activities_actor_id_fkey;
ALTER TABLE mcp.workspace_activities 
    ADD CONSTRAINT workspace_activities_actor_id_fkey 
    FOREIGN KEY (actor_id) REFERENCES mcp.agent_configurations(id);

-- agent_events table (from migration 008)
ALTER TABLE mcp.agent_events 
    DROP CONSTRAINT IF EXISTS agent_events_agent_id_fkey;
ALTER TABLE mcp.agent_events 
    ADD CONSTRAINT agent_events_agent_id_fkey 
    FOREIGN KEY (agent_id) REFERENCES mcp.agent_configurations(id) ON DELETE CASCADE;

-- agent_sessions table (from migration 008)
ALTER TABLE mcp.agent_sessions 
    DROP CONSTRAINT IF EXISTS agent_sessions_agent_id_fkey;
ALTER TABLE mcp.agent_sessions 
    ADD CONSTRAINT agent_sessions_agent_id_fkey 
    FOREIGN KEY (agent_id) REFERENCES mcp.agent_configurations(id) ON DELETE CASCADE;

-- agent_registry table (from migration 008)
ALTER TABLE mcp.agent_registry 
    DROP CONSTRAINT IF EXISTS agent_registry_agent_id_fkey;
ALTER TABLE mcp.agent_registry 
    ADD CONSTRAINT agent_registry_agent_id_fkey 
    FOREIGN KEY (agent_id) REFERENCES mcp.agent_configurations(id) ON DELETE CASCADE;

-- agent_embedding_preferences table (from migration 015)
ALTER TABLE mcp.agent_embedding_preferences 
    DROP CONSTRAINT IF EXISTS agent_embedding_preferences_agent_id_fkey;
ALTER TABLE mcp.agent_embedding_preferences 
    ADD CONSTRAINT agent_embedding_preferences_agent_id_fkey 
    FOREIGN KEY (agent_id) REFERENCES mcp.agent_configurations(id) ON DELETE CASCADE;

-- embedding_usage_tracking table (from migration 015)
ALTER TABLE mcp.embedding_usage_tracking 
    DROP CONSTRAINT IF EXISTS embedding_usage_tracking_agent_id_fkey;
ALTER TABLE mcp.embedding_usage_tracking 
    ADD CONSTRAINT embedding_usage_tracking_agent_id_fkey 
    FOREIGN KEY (agent_id) REFERENCES mcp.agent_configurations(id);

-- ================================================================
-- STEP 4: Create Helper Functions for Idempotent Operations
-- ================================================================

-- Function to register or update an agent instance (handles reconnections)
CREATE OR REPLACE FUNCTION mcp.register_agent_instance(
    p_tenant_id UUID,
    p_agent_id TEXT,           -- The deterministic agent ID (e.g., "ide-agent")
    p_instance_id TEXT,         -- The unique instance ID (e.g., WebSocket connection ID)
    p_name TEXT DEFAULT NULL,
    p_connection_details JSONB DEFAULT '{}',
    p_runtime_config JSONB DEFAULT '{}'
)
RETURNS TABLE(
    registration_id UUID,
    manifest_id UUID,
    config_id UUID,
    is_new BOOLEAN,
    message TEXT
) 
LANGUAGE plpgsql
AS $$
DECLARE
    v_manifest_id UUID;
    v_config_id UUID;
    v_registration_id UUID;
    v_existing_reg_id UUID;
    v_is_new BOOLEAN := FALSE;
BEGIN
    -- Get or create manifest
    SELECT id INTO v_manifest_id
    FROM mcp.agent_manifests
    WHERE agent_id = p_agent_id;
    
    IF v_manifest_id IS NULL THEN
        -- Create new manifest if it doesn't exist
        INSERT INTO mcp.agent_manifests (
            agent_id,
            agent_type,
            name,
            version,
            status
        ) VALUES (
            p_agent_id,
            'dynamic',
            COALESCE(p_name, p_agent_id),
            '1.0.0',
            'active'
        )
        RETURNING id INTO v_manifest_id;
    END IF;
    
    -- Get or create configuration
    SELECT id INTO v_config_id
    FROM mcp.agent_configurations
    WHERE tenant_id = p_tenant_id AND agent_configurations.manifest_id = v_manifest_id;
    
    IF v_config_id IS NULL THEN
        INSERT INTO mcp.agent_configurations (
            tenant_id,
            manifest_id,
            name,
            enabled
        ) VALUES (
            p_tenant_id,
            v_manifest_id,
            COALESCE(p_name, p_agent_id),
            true
        )
        RETURNING id INTO v_config_id;
    END IF;
    
    -- Check for existing registration
    SELECT id INTO v_existing_reg_id
    FROM mcp.agent_registrations
    WHERE tenant_id = p_tenant_id AND instance_id = p_instance_id;
    
    IF v_existing_reg_id IS NOT NULL THEN
        -- Update existing registration (reconnection scenario)
        UPDATE mcp.agent_registrations
        SET 
            manifest_id = v_manifest_id,
            registration_status = 'active',
            health_status = 'healthy',
            connection_details = p_connection_details,
            runtime_config = p_runtime_config,
            last_health_check = NOW(),
            updated_at = NOW()
        WHERE id = v_existing_reg_id;
        
        v_registration_id := v_existing_reg_id;
        v_is_new := FALSE;
        
        RETURN QUERY SELECT 
            v_registration_id,
            v_manifest_id,
            v_config_id,
            v_is_new,
            'Registration updated (reconnection)'::TEXT;
    ELSE
        -- Create new registration
        INSERT INTO mcp.agent_registrations (
            manifest_id,
            tenant_id,
            instance_id,
            registration_status,
            health_status,
            connection_details,
            runtime_config,
            activation_date
        ) VALUES (
            v_manifest_id,
            p_tenant_id,
            p_instance_id,
            'active',
            'healthy',
            p_connection_details,
            p_runtime_config,
            NOW()
        )
        ON CONFLICT (tenant_id, instance_id) DO UPDATE
        SET
            registration_status = 'active',
            health_status = 'healthy',
            last_health_check = NOW()
        RETURNING id INTO v_registration_id;
        
        v_is_new := TRUE;
        
        RETURN QUERY SELECT 
            v_registration_id,
            v_manifest_id,
            v_config_id,
            v_is_new,
            'New registration created'::TEXT;
    END IF;
END;
$$;

-- ================================================================
-- STEP 5: Create Compatibility View
-- ================================================================

-- Rename old agents table
ALTER TABLE mcp.agents RENAME TO agents_deprecated;

-- Create view that mimics old agents table for backward compatibility
CREATE OR REPLACE VIEW mcp.agents AS
SELECT 
    ac.id,
    ac.tenant_id,
    ac.name,
    am.agent_type as type,
    ac.model_id,
    -- Extract capabilities array from JSONB
    COALESCE(
        ARRAY(SELECT jsonb_array_elements_text(am.capabilities->'capabilities')),
        ARRAY[]::TEXT[]
    ) as capabilities,
    -- Derive status from active registrations
    CASE 
        WHEN EXISTS (
            SELECT 1 FROM mcp.agent_registrations ar 
            WHERE ar.manifest_id = ac.manifest_id 
              AND ar.tenant_id = ac.tenant_id
              AND ar.health_status = 'healthy'
              AND ar.registration_status = 'active'
        ) THEN 'active'
        WHEN ac.enabled THEN 'available'
        ELSE 'offline'
    END as status,
    ac.configuration,
    ac.system_prompt,
    ac.temperature,
    ac.max_tokens,
    ac.current_workload,
    ac.max_workload,
    -- Get last activity from registrations
    (SELECT MAX(ar.last_health_check)
     FROM mcp.agent_registrations ar
     WHERE ar.manifest_id = ac.manifest_id 
       AND ar.tenant_id = ac.tenant_id) as last_task_assigned_at,
    (SELECT MAX(ar.last_health_check)
     FROM mcp.agent_registrations ar
     WHERE ar.manifest_id = ac.manifest_id 
       AND ar.tenant_id = ac.tenant_id) as last_seen_at,
    ac.created_at,
    ac.updated_at
FROM mcp.agent_configurations ac
JOIN mcp.agent_manifests am ON am.id = ac.manifest_id;

-- ================================================================
-- STEP 6: Create Triggers for View Operations
-- ================================================================

-- INSERT trigger
CREATE OR REPLACE FUNCTION mcp.agents_view_insert()
RETURNS TRIGGER 
LANGUAGE plpgsql
AS $$
DECLARE
    v_result RECORD;
BEGIN
    -- Register the agent using the new system
    SELECT * INTO v_result
    FROM mcp.register_agent_instance(
        NEW.tenant_id,
        COALESCE(NEW.id::text, gen_random_uuid()::text),
        'view-' || COALESCE(NEW.id::text, gen_random_uuid()::text),
        NEW.name,
        jsonb_build_object('source', 'legacy_view'),
        jsonb_build_object('capabilities', NEW.capabilities)
    );
    
    -- Update configuration
    UPDATE mcp.agent_configurations
    SET 
        name = NEW.name,
        model_id = NEW.model_id,
        system_prompt = NEW.system_prompt,
        temperature = NEW.temperature,
        max_tokens = NEW.max_tokens,
        configuration = NEW.configuration,
        current_workload = COALESCE(NEW.current_workload, 0),
        max_workload = COALESCE(NEW.max_workload, 10)
    WHERE id = v_result.config_id;
    
    -- Set NEW.id to the config_id for compatibility
    NEW.id := v_result.config_id;
    
    RETURN NEW;
END;
$$;

CREATE TRIGGER agents_view_insert_trigger
    INSTEAD OF INSERT ON mcp.agents
    FOR EACH ROW
    EXECUTE FUNCTION mcp.agents_view_insert();

-- UPDATE trigger
CREATE OR REPLACE FUNCTION mcp.agents_view_update()
RETURNS TRIGGER 
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE mcp.agent_configurations
    SET 
        name = NEW.name,
        model_id = NEW.model_id,
        system_prompt = NEW.system_prompt,
        temperature = NEW.temperature,
        max_tokens = NEW.max_tokens,
        configuration = NEW.configuration,
        current_workload = NEW.current_workload,
        max_workload = NEW.max_workload,
        updated_at = NOW()
    WHERE id = NEW.id;
    
    RETURN NEW;
END;
$$;

CREATE TRIGGER agents_view_update_trigger
    INSTEAD OF UPDATE ON mcp.agents
    FOR EACH ROW
    EXECUTE FUNCTION mcp.agents_view_update();

-- DELETE trigger
CREATE OR REPLACE FUNCTION mcp.agents_view_delete()
RETURNS TRIGGER 
LANGUAGE plpgsql
AS $$
BEGIN
    -- Soft delete by disabling configuration
    UPDATE mcp.agent_configurations
    SET 
        enabled = false,
        updated_at = NOW()
    WHERE id = OLD.id;
    
    -- Mark all registrations as inactive
    UPDATE mcp.agent_registrations
    SET 
        registration_status = 'inactive',
        health_status = 'disconnected',
        updated_at = NOW()
    WHERE manifest_id = (
        SELECT manifest_id FROM mcp.agent_configurations WHERE id = OLD.id
    )
    AND tenant_id = OLD.tenant_id;
    
    RETURN OLD;
END;
$$;

CREATE TRIGGER agents_view_delete_trigger
    INSTEAD OF DELETE ON mcp.agents
    FOR EACH ROW
    EXECUTE FUNCTION mcp.agents_view_delete();

-- ================================================================
-- STEP 7: Add Indexes and Constraints
-- ================================================================

-- Add trigger to update timestamps
CREATE TRIGGER update_agent_configurations_timestamp
    BEFORE UPDATE ON mcp.agent_configurations
    FOR EACH ROW
    EXECUTE FUNCTION mcp.update_agent_manifest_timestamp();

-- Add comments for documentation
COMMENT ON TABLE mcp.agent_configurations IS 'Tenant-specific configuration for each agent type - replaces the old agents table';
COMMENT ON TABLE mcp.agent_manifests IS 'Agent type definitions (e.g., IDE agent, Slack bot) - the blueprint';
COMMENT ON TABLE mcp.agent_registrations IS 'Active instances of agents - supports multiple instances per configuration';
COMMENT ON VIEW mcp.agents IS 'Backward compatibility view - use agent_configurations directly for new code';
COMMENT ON FUNCTION mcp.register_agent_instance IS 'Idempotent agent registration - handles reconnections, pod restarts, and multiple instances';

-- ================================================================
-- STEP 8: Grant Permissions
-- ================================================================

-- Grant permissions on new objects (adjust roles as needed)
-- Grant permissions if application_role exists
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'application_role') THEN
        GRANT SELECT, INSERT, UPDATE ON mcp.agent_configurations TO application_role;
        GRANT SELECT, INSERT, UPDATE ON mcp.agent_registrations TO application_role;
        GRANT EXECUTE ON FUNCTION mcp.register_agent_instance TO application_role;
    END IF;
END
$$;

COMMIT;

-- ================================================================
-- Post-Migration Notes:
-- ================================================================
-- 1. The old agents table is renamed to agents_deprecated (not dropped)
-- 2. All foreign keys now point to agent_configurations
-- 3. The agents view provides backward compatibility
-- 4. New code should use register_agent_instance() function
-- 5. After verifying everything works, agents_deprecated can be dropped
-- ================================================================