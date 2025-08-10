-- Revert to the original view from migration 16

DROP VIEW IF EXISTS mcp.agents CASCADE;

-- Recreate original view from migration 16
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

-- Recreate original triggers

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

COMMENT ON VIEW mcp.agents IS 'Backward compatibility view - use agent_configurations directly for new code';