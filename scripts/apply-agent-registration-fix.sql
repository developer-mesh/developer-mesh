-- Apply just the critical agent registration function
-- This creates an idempotent registration that solves duplicate key errors

-- First ensure the agent_configurations table exists
CREATE TABLE IF NOT EXISTS mcp.agent_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    agent_id TEXT NOT NULL,
    name TEXT,
    config JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, agent_id)
);

-- Create the idempotent registration function
CREATE OR REPLACE FUNCTION mcp.register_agent_instance(
    p_tenant_id UUID,
    p_agent_id TEXT,
    p_instance_id TEXT,
    p_name TEXT DEFAULT NULL,
    p_connection_details JSONB DEFAULT '{}',
    p_runtime_config JSONB DEFAULT '{}'
)
RETURNS TABLE (
    registration_id UUID,
    manifest_id UUID,
    config_id UUID,
    is_new BOOLEAN,
    message TEXT
) AS $$
DECLARE
    v_manifest_id UUID;
    v_config_id UUID;
    v_registration_id UUID;
    v_is_new BOOLEAN := FALSE;
    v_message TEXT;
BEGIN
    -- Step 1: Get or create manifest
    SELECT id INTO v_manifest_id 
    FROM mcp.agent_manifests 
    WHERE agent_id = p_agent_id;
    
    IF v_manifest_id IS NULL THEN
        INSERT INTO mcp.agent_manifests (
            agent_id, 
            agent_type, 
            name,
            status
        ) VALUES (
            p_agent_id,
            'standard',
            COALESCE(p_name, p_agent_id),
            'active'
        )
        RETURNING id INTO v_manifest_id;
    END IF;
    
    -- Step 2: Get or create configuration
    SELECT id INTO v_config_id
    FROM mcp.agent_configurations
    WHERE tenant_id = p_tenant_id 
    AND agent_id = p_agent_id;
    
    IF v_config_id IS NULL THEN
        INSERT INTO mcp.agent_configurations (
            tenant_id,
            agent_id,
            name,
            config
        ) VALUES (
            p_tenant_id,
            p_agent_id,
            COALESCE(p_name, p_agent_id),
            COALESCE(p_runtime_config, '{}')
        )
        RETURNING id INTO v_config_id;
    ELSE
        -- Update config if provided
        UPDATE mcp.agent_configurations
        SET config = COALESCE(p_runtime_config, config),
            updated_at = CURRENT_TIMESTAMP
        WHERE id = v_config_id;
    END IF;
    
    -- Step 3: Handle registration (idempotent by instance_id)
    SELECT id INTO v_registration_id
    FROM mcp.agent_registrations
    WHERE instance_id = p_instance_id;
    
    IF v_registration_id IS NULL THEN
        -- New registration
        INSERT INTO mcp.agent_registrations (
            manifest_id,
            tenant_id,
            instance_id,
            registration_status,
            connection_details,
            runtime_config,
            health_status
        ) VALUES (
            v_manifest_id,
            p_tenant_id,
            p_instance_id,
            'active',
            p_connection_details,
            p_runtime_config,
            'healthy'
        )
        RETURNING id INTO v_registration_id;
        
        v_is_new := TRUE;
        v_message := 'New registration created';
    ELSE
        -- Update existing registration (reconnection)
        UPDATE mcp.agent_registrations
        SET connection_details = p_connection_details,
            runtime_config = p_runtime_config,
            updated_at = CURRENT_TIMESTAMP,
            health_status = 'healthy',
            registration_status = 'active'
        WHERE id = v_registration_id;
        
        v_is_new := FALSE;
        v_message := 'Registration updated (reconnection)';
    END IF;
    
    -- Also update/insert into agents table for backward compatibility
    INSERT INTO mcp.agents (
        tenant_id,
        name,
        agent_type,
        status,
        connection_id,
        capabilities,
        metadata
    ) VALUES (
        p_tenant_id,
        COALESCE(p_name, p_agent_id),
        'standard',
        'active',
        p_instance_id,
        '{}',
        jsonb_build_object(
            'agent_id', p_agent_id,
            'instance_id', p_instance_id,
            'config_id', v_config_id,
            'manifest_id', v_manifest_id,
            'registration_id', v_registration_id
        )
    )
    ON CONFLICT (tenant_id, name) 
    DO UPDATE SET
        connection_id = EXCLUDED.connection_id,
        status = 'active',
        metadata = EXCLUDED.metadata,
        updated_at = CURRENT_TIMESTAMP;
    
    RETURN QUERY SELECT 
        v_registration_id,
        v_manifest_id,
        v_config_id,
        v_is_new,
        v_message;
END;
$$ LANGUAGE plpgsql;