-- Fix Agent Config Trigger - Remove Non-Existent Column Reference
-- This migration fixes the broken trigger that references custom_models column

DROP TRIGGER IF EXISTS create_agent_config_on_insert ON mcp.agents;
DROP FUNCTION IF EXISTS mcp.create_default_agent_config();

CREATE OR REPLACE FUNCTION mcp.create_default_agent_config()
RETURNS TRIGGER AS $$
DECLARE
    config_id UUID;
BEGIN
    -- Skip if agent already has a config
    IF EXISTS (SELECT 1 FROM mcp.agent_configs WHERE agent_id = NEW.id) THEN
        RETURN NEW;
    END IF;
    
    -- Create minimal config (no custom_models column)
    INSERT INTO mcp.agent_configs (
        id, agent_id, version, embedding_strategy, model_preferences,
        constraints, fallback_behavior, metadata, is_active, 
        created_at, updated_at, created_by
    ) VALUES (
        gen_random_uuid(),
        NEW.id,
        1,
        'balanced',
        CASE NEW.type
            WHEN 'ide' THEN '[{"task_type": "code_analysis", "primary_models": ["bedrock:amazon.titan-embed-text-v1"], "fallback_models": ["openai:text-embedding-3-small"], "weight": 1.0}]'::jsonb
            WHEN 'slack' THEN '[{"task_type": "general_qa", "primary_models": ["openai:text-embedding-3-small"], "fallback_models": [], "weight": 1.0}]'::jsonb
            ELSE '[{"task_type": "general_qa", "primary_models": ["bedrock:amazon.titan-embed-text-v1"], "fallback_models": [], "weight": 1.0}]'::jsonb
        END,
        '{}'::jsonb,  -- constraints
        '{}'::jsonb,  -- fallback_behavior
        '{"auto_created": true}'::jsonb,  -- metadata
        true,
        CURRENT_TIMESTAMP,
        CURRENT_TIMESTAMP,
        '00000000-0000-0000-0000-000000000000'::UUID
    ) ON CONFLICT (agent_id, version) DO NOTHING
    RETURNING id INTO config_id;
    
    RETURN NEW;
EXCEPTION WHEN OTHERS THEN
    RAISE WARNING 'Failed to create default config for agent %: %', NEW.id, SQLERRM;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER create_agent_config_on_insert
    AFTER INSERT ON mcp.agents
    FOR EACH ROW
    EXECUTE FUNCTION mcp.create_default_agent_config();
