-- Seed test agent configuration data
-- This script creates test data for agent configurations

-- First, ensure we have a test agent  
-- Note: agents table requires tenant_id
INSERT INTO mcp.agents (id, tenant_id, name, type, status, configuration, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000001'::uuid,
    '00000000-0000-0000-0000-000000000001'::uuid,  -- Default tenant ID
    'test-agent',
    'embedding',
    'available',
    '{"description": "Test agent for embedding operations"}'::jsonb,
    NOW(),
    NOW()
) ON CONFLICT (id) DO UPDATE SET
    updated_at = NOW();

-- Create a test agent configuration
INSERT INTO mcp.agent_configs (
    id,
    agent_id,
    version,
    embedding_strategy,
    model_preferences,
    constraints,
    fallback_behavior,
    metadata,
    is_active,
    created_at,
    updated_at,
    created_by
) VALUES (
    uuid_generate_v4(),
    '00000000-0000-0000-0000-000000000001'::uuid,
    1,
    'balanced',
    '["text-embedding-3-small", "amazon.titan-embed-text-v2:0", "text-embedding-ada-002"]'::jsonb,
    '{
        "max_tokens_per_request": 8000,
        "max_cost_per_day": 10.0,
        "preferred_dimensions": 1536,
        "allow_dimension_reduction": true
    }'::jsonb,
    '{
        "enabled": true,
        "max_retries": 3,
        "retry_delay": "1s",
        "use_cache_on_failure": true
    }'::jsonb,
    '{
        "created_by": "system",
        "purpose": "test configuration"
    }'::jsonb,
    true,
    NOW(),
    NOW(),
    '00000000-0000-0000-0000-000000000000'::uuid  -- System user UUID
) ON CONFLICT (agent_id, version) DO UPDATE SET
    updated_at = NOW();

-- Create a second test agent for testing multiple configurations
INSERT INTO mcp.agents (id, tenant_id, name, type, status, configuration, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000002'::uuid,
    '00000000-0000-0000-0000-000000000001'::uuid,  -- Default tenant ID
    'production-agent',
    'embedding',
    'available',
    '{"description": "Production agent for embedding operations"}'::jsonb,
    NOW(),
    NOW()
) ON CONFLICT (id) DO UPDATE SET
    updated_at = NOW();

-- Create production agent configuration
INSERT INTO mcp.agent_configs (
    id,
    agent_id,
    version,
    embedding_strategy,
    model_preferences,
    constraints,
    fallback_behavior,
    metadata,
    is_active,
    created_at,
    updated_at,
    created_by
) VALUES (
    uuid_generate_v4(),
    '00000000-0000-0000-0000-000000000002'::uuid,
    1,
    'quality',
    '["text-embedding-3-large", "text-embedding-3-small", "amazon.titan-embed-text-v2:0"]'::jsonb,
    '{
        "max_tokens_per_request": 8191,
        "max_cost_per_day": 50.0,
        "preferred_dimensions": 3072,
        "allow_dimension_reduction": false
    }'::jsonb,
    '{
        "enabled": true,
        "max_retries": 5,
        "retry_delay": "2s",
        "use_cache_on_failure": true
    }'::jsonb,
    '{
        "created_by": "system",
        "purpose": "production configuration",
        "environment": "production"
    }'::jsonb,
    true,
    NOW(),
    NOW(),
    '00000000-0000-0000-0000-000000000000'::uuid  -- System user UUID
) ON CONFLICT (agent_id, version) DO UPDATE SET
    updated_at = NOW();

-- Verify the data was inserted
SELECT 
    a.name as agent_name,
    ac.version,
    ac.embedding_strategy,
    ac.model_preferences->>'primary_models' as primary_models,
    ac.is_active
FROM mcp.agent_configs ac
JOIN mcp.agents a ON a.id = ac.agent_id
ORDER BY a.name, ac.version;