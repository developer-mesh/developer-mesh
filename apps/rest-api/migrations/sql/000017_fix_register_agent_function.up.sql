-- Fix the ambiguous column reference in register_agent_instance function

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
    
    -- Get or create configuration (fix: use table alias to avoid ambiguity)
    SELECT ac.id INTO v_config_id
    FROM mcp.agent_configurations ac
    WHERE ac.tenant_id = p_tenant_id AND ac.manifest_id = v_manifest_id;
    
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
    
    -- Check for existing registration with this instance_id
    SELECT id INTO v_existing_reg_id
    FROM mcp.agent_registrations
    WHERE instance_id = p_instance_id;
    
    IF v_existing_reg_id IS NULL THEN
        -- Create new registration
        INSERT INTO mcp.agent_registrations (
            manifest_id,
            tenant_id,
            instance_id,
            registration_status,
            health_status,
            connection_details,
            runtime_config
        ) VALUES (
            v_manifest_id,
            p_tenant_id,
            p_instance_id,
            'active',
            'healthy',
            p_connection_details,
            p_runtime_config
        )
        RETURNING id INTO v_registration_id;
        
        v_is_new := TRUE;
    ELSE
        -- Update existing registration (reconnection)
        UPDATE mcp.agent_registrations
        SET connection_details = p_connection_details,
            runtime_config = p_runtime_config,
            health_status = 'healthy',
            registration_status = 'active',
            last_health_check = CURRENT_TIMESTAMP,
            updated_at = CURRENT_TIMESTAMP
        WHERE id = v_existing_reg_id
        RETURNING id INTO v_registration_id;
        
        v_is_new := FALSE;
    END IF;
    
    -- Note: We don't insert into mcp.agents view here
    -- The view is automatically populated from the underlying tables
    -- (agent_configurations joined with agent_manifests)
    
    -- Return the results
    RETURN QUERY SELECT 
        v_registration_id,
        v_manifest_id,
        v_config_id,
        v_is_new,
        CASE 
            WHEN v_is_new THEN 'New registration created'
            ELSE 'Registration updated (reconnection)'
        END;
END;
$$;