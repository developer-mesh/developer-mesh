-- =====================================================================
-- Fix Agent Configs Table - Handle Duplicate Keys and NULL Values
-- Version: 1.0.0
-- Author: Principal Engineer
-- Description: Fixes issues with agent_configs table constraints
-- =====================================================================

-- Make created_by nullable to allow system-created configs
ALTER TABLE mcp.agent_configs 
ALTER COLUMN created_by DROP NOT NULL;

-- Add ON CONFLICT handling to the trigger function
CREATE OR REPLACE FUNCTION mcp.create_default_agent_config()
RETURNS TRIGGER AS $$
DECLARE
    template_record RECORD;
    config_id UUID;
BEGIN
    -- Skip if agent already has a config
    IF EXISTS (SELECT 1 FROM mcp.agent_configs WHERE agent_id = NEW.id) THEN
        RETURN NEW;
    END IF;
    
    -- Find default template for this agent type
    SELECT * INTO template_record
    FROM mcp.agent_config_templates
    WHERE agent_type = NEW.type 
    AND is_default = true 
    AND is_active = true
    LIMIT 1;
    
    -- If no template found, create minimal config
    IF template_record IS NULL THEN
        INSERT INTO mcp.agent_configs (
            id, agent_id, embedding_strategy, model_preferences,
            custom_models, is_active, created_at, updated_at, created_by
        ) VALUES (
            gen_random_uuid(),
            NEW.id,
            'balanced'::mcp.embedding_strategy,
            '[
                {
                    "task_type": "general_qa",
                    "primary_models": ["bedrock:amazon.titan-embed-text-v1"],
                    "fallback_models": [],
                    "weight": 1.0
                }
            ]'::jsonb,
            '[]'::jsonb,
            true,
            CURRENT_TIMESTAMP,
            CURRENT_TIMESTAMP,
            '00000000-0000-0000-0000-000000000000'::UUID -- System user
        ) ON CONFLICT (agent_id) DO NOTHING -- Prevent duplicate key errors
        RETURNING id INTO config_id;
    ELSE
        -- Create config from template
        INSERT INTO mcp.agent_configs (
            id, agent_id, embedding_strategy, model_preferences,
            custom_models, is_active, created_at, updated_at, created_by
        ) VALUES (
            gen_random_uuid(),
            NEW.id,
            template_record.embedding_strategy,
            template_record.model_preferences,
            '[]'::jsonb,
            true,
            CURRENT_TIMESTAMP,
            CURRENT_TIMESTAMP,
            '00000000-0000-0000-0000-000000000000'::UUID -- System user
        ) ON CONFLICT (agent_id) DO NOTHING -- Prevent duplicate key errors
        RETURNING id INTO config_id;
    END IF;
    
    -- Only record event if config was actually created
    IF config_id IS NOT NULL THEN
        INSERT INTO mcp.agent_events (
            agent_id, tenant_id, event_type, payload
        ) VALUES (
            NEW.id, NEW.tenant_id, 'config_created',
            jsonb_build_object('config_id', config_id, 'auto_created', true)
        );
        
        -- Automatically transition to configuring state
        UPDATE mcp.agents 
        SET state = 'configuring'::mcp.agent_state,
            state_reason = 'Auto-configuration initiated'
        WHERE id = NEW.id;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Add index to prevent duplicate configs
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_configs_agent_id ON mcp.agent_configs(agent_id);

-- Update existing NULL created_by values
UPDATE mcp.agent_configs 
SET created_by = '00000000-0000-0000-0000-000000000000'::UUID 
WHERE created_by IS NULL;