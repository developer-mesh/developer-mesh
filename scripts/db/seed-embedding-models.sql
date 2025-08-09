-- =====================================================================
-- Seed Data for Multi-Tenant Embedding Model Management
-- =====================================================================
-- This script populates initial data for testing the embedding model
-- management system. It includes model catalog entries and default
-- tenant configurations.

-- Ensure UUID extension is available
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create partition for current month if it doesn't exist
DO $$
DECLARE
    current_year TEXT := to_char(CURRENT_DATE, 'YYYY');
    current_month TEXT := to_char(CURRENT_DATE, 'MM');
    next_month DATE := date_trunc('month', CURRENT_DATE) + interval '1 month';
    partition_name TEXT := 'embedding_usage_tracking_' || current_year || '_' || current_month;
    start_date TEXT := to_char(date_trunc('month', CURRENT_DATE), 'YYYY-MM-DD');
    end_date TEXT := to_char(next_month, 'YYYY-MM-DD');
BEGIN
    -- Check if partition exists
    IF NOT EXISTS (
        SELECT 1 FROM pg_tables 
        WHERE schemaname = 'mcp' 
        AND tablename = partition_name
    ) THEN
        -- Create the partition
        EXECUTE format('CREATE TABLE mcp.%I PARTITION OF mcp.embedding_usage_tracking FOR VALUES FROM (%L) TO (%L)',
            partition_name, start_date, end_date);
        RAISE NOTICE 'Created partition %', partition_name;
    END IF;
END $$;

-- =====================================================================
-- 1. Ensure the models from migration are present (idempotent)
-- =====================================================================
-- Models are already inserted in the migration, but we'll ensure they're up to date

UPDATE mcp.embedding_model_catalog SET 
    is_available = true,
    cost_per_million_tokens = 0.02,
    updated_at = NOW()
WHERE model_id = 'amazon.titan-embed-text-v2:0';

-- =====================================================================
-- 2. Create Test Tenants if they don't exist
-- =====================================================================
-- We need some test tenant IDs for development
DO $$
DECLARE
    test_tenant_1 UUID := 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
    test_tenant_2 UUID := 'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12';
    default_org_id UUID := 'c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13';
    titan_v2_model_id UUID;
    titan_v1_model_id UUID;
    ada_model_id UUID;
BEGIN
    -- Insert test organization if not exists
    INSERT INTO mcp.organizations (id, name, slug, isolation_mode)
    VALUES (default_org_id, 'Test Organization', 'test-org', 'strict')
    ON CONFLICT (id) DO NOTHING;

    -- Insert test tenant entries in organization_tenants
    INSERT INTO mcp.organization_tenants (organization_id, tenant_id, tenant_name, tenant_type)
    VALUES 
        (default_org_id, test_tenant_1, 'Development Tenant', 'development'),
        (default_org_id, test_tenant_2, 'Production Tenant', 'production')
    ON CONFLICT (organization_id, tenant_id) DO NOTHING;

    -- Get model IDs
    SELECT id INTO titan_v2_model_id FROM mcp.embedding_model_catalog 
    WHERE model_id = 'amazon.titan-embed-text-v2:0';
    
    SELECT id INTO titan_v1_model_id FROM mcp.embedding_model_catalog 
    WHERE model_id = 'amazon.titan-embed-text-v1';
    
    SELECT id INTO ada_model_id FROM mcp.embedding_model_catalog 
    WHERE model_id = 'text-embedding-ada-002';

    -- =====================================================================
    -- 3. Configure Models for Test Tenants
    -- =====================================================================
    
    -- Development Tenant Configuration (test_tenant_1)
    -- Enable multiple models with Titan v2 as default
    INSERT INTO mcp.tenant_embedding_models (
        tenant_id, model_id, is_enabled, is_default, 
        monthly_token_limit, daily_token_limit,
        priority, created_at, updated_at
    ) VALUES 
        -- Titan v2 as default
        (test_tenant_1, titan_v2_model_id, true, true, 
         10000000, 1000000, 100, NOW(), NOW()),
        -- Titan v1 as fallback
        (test_tenant_1, titan_v1_model_id, true, false, 
         5000000, 500000, 50, NOW(), NOW()),
        -- Ada as secondary option
        (test_tenant_1, ada_model_id, true, false, 
         5000000, 500000, 75, NOW(), NOW())
    ON CONFLICT (tenant_id, model_id) 
    DO UPDATE SET 
        is_enabled = EXCLUDED.is_enabled,
        is_default = EXCLUDED.is_default,
        priority = EXCLUDED.priority,
        updated_at = NOW();

    -- Production Tenant Configuration (test_tenant_2)
    -- More restrictive limits, only Titan v2
    INSERT INTO mcp.tenant_embedding_models (
        tenant_id, model_id, is_enabled, is_default, 
        monthly_token_limit, daily_token_limit, monthly_request_limit,
        priority, created_at, updated_at
    ) VALUES 
        (test_tenant_2, titan_v2_model_id, true, true, 
         50000000, 5000000, 100000, 100, NOW(), NOW())
    ON CONFLICT (tenant_id, model_id) 
    DO UPDATE SET 
        is_enabled = EXCLUDED.is_enabled,
        is_default = EXCLUDED.is_default,
        monthly_token_limit = EXCLUDED.monthly_token_limit,
        daily_token_limit = EXCLUDED.daily_token_limit,
        updated_at = NOW();

    -- =====================================================================
    -- 4. Create Test Agents if needed
    -- =====================================================================
    INSERT INTO mcp.agents (
        id, tenant_id, name, type, status,
        created_at, updated_at
    ) VALUES 
        ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14', test_tenant_1, 
         'IDE Agent Dev', 'ide', 'active', NOW(), NOW()),
        ('e0eebc99-9c0b-4ef8-bb6d-6bb9bd380a15', test_tenant_2, 
         'IDE Agent Prod', 'ide', 'active', NOW(), NOW())
    ON CONFLICT (id) DO NOTHING;

    -- =====================================================================
    -- 5. Add Agent Preferences (optional)
    -- =====================================================================
    INSERT INTO mcp.agent_embedding_preferences (
        tenant_id, agent_id, 
        selection_strategy, primary_model_id,
        max_cost_per_request, monthly_budget,
        created_at, updated_at
    ) VALUES 
        (test_tenant_1, 'd0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14',
         'cost_optimized', titan_v2_model_id,
         0.001, 10.00, NOW(), NOW())
    ON CONFLICT (tenant_id, agent_id) DO NOTHING;

    -- =====================================================================
    -- 6. Add some sample usage data for testing quotas
    -- =====================================================================
    -- Add minimal usage to show the system is working
    INSERT INTO mcp.embedding_usage_tracking (
        id, tenant_id, agent_id, model_id,
        tokens_used, embeddings_generated,
        actual_cost, billed_cost,
        latency_ms, task_type,
        created_at
    ) VALUES 
        (uuid_generate_v4(), test_tenant_1, 'd0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14', 
         titan_v2_model_id, 1000, 1, 0.00002, 0.00002, 45, 'code_analysis', NOW()),
        (uuid_generate_v4(), test_tenant_1, 'd0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14', 
         titan_v2_model_id, 2000, 1, 0.00004, 0.00004, 50, 'documentation', NOW() - INTERVAL '1 hour')
    ON CONFLICT DO NOTHING;

END $$;

-- =====================================================================
-- 7. Verify Setup
-- =====================================================================
-- Show configured models for test tenants
SELECT 
    ot.tenant_name,
    c.model_id as model,
    c.provider,
    tm.is_enabled,
    tm.is_default,
    tm.priority,
    tm.monthly_token_limit,
    tm.daily_token_limit
FROM mcp.tenant_embedding_models tm
JOIN mcp.embedding_model_catalog c ON c.id = tm.model_id
JOIN mcp.organization_tenants ot ON ot.tenant_id = tm.tenant_id
ORDER BY ot.tenant_name, tm.priority DESC;

-- Test the model selection function
SELECT * FROM mcp.get_embedding_model_for_request(
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'::UUID,  -- test_tenant_1
    NULL,  -- no specific agent
    NULL,  -- no task type
    NULL   -- no requested model
);

-- Show usage summary
SELECT 
    tenant_id,
    COUNT(*) as request_count,
    SUM(tokens_used) as total_tokens,
    SUM(actual_cost) as total_cost
FROM mcp.embedding_usage_tracking
WHERE tenant_id IN (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'::UUID,
    'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12'::UUID
)
GROUP BY tenant_id;