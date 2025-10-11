-- =====================================================================
-- Configure AWS Bedrock Titan Embeddings for DevMesh
-- =====================================================================
-- This script adds AWS Bedrock Titan models and configures them for
-- the default tenant (00000000-0000-0000-0000-000000000001)
-- =====================================================================

\echo 'Configuring AWS Bedrock Titan embedding models...'

-- 1. Add AWS Bedrock Titan models to the catalog
INSERT INTO mcp.embedding_model_catalog (
    id, model_id, provider, model_name, model_version,
    max_tokens, dimensions, cost_per_million_tokens,
    model_type, supports_binary, supports_dimensionality_reduction,
    min_dimensions, is_available, requires_api_key,
    capabilities, created_at, updated_at
) VALUES
    -- Titan Text Embeddings V2 (Latest, recommended)
    (
        gen_random_uuid(),
        'amazon.titan-embed-text-v2:0',
        'aws_bedrock',
        'Titan Text Embeddings V2',
        'v2',
        8192,  -- max tokens
        1024,  -- dimensions (with dimensionality reduction)
        0.0200,  -- $0.02 per million tokens
        'text',
        false,
        true,  -- supports dimensionality reduction (256-1024)
        256,   -- min dimensions
        true,
        true,  -- requires AWS credentials
        '{"normalization": "l2", "similarity_metric": "cosine", "supports_batching": true, "max_batch_size": 128}'::jsonb,
        NOW(),
        NOW()
    ),
    -- Titan Text Embeddings V1 (Fallback with larger dimensions)
    (
        gen_random_uuid(),
        'amazon.titan-embed-text-v1',
        'aws_bedrock',
        'Titan Text Embeddings V1',
        'v1',
        8192,
        1536,  -- larger dimensions (fixed, no reduction)
        0.0200,
        'text',
        false,
        false,  -- does not support dimensionality reduction
        1536,
        true,
        true,
        '{"normalization": "l2", "similarity_metric": "cosine", "supports_batching": true, "max_batch_size": 128}'::jsonb,
        NOW(),
        NOW()
    )
ON CONFLICT (model_id) DO UPDATE SET
    is_available = true,
    cost_per_million_tokens = EXCLUDED.cost_per_million_tokens,
    max_tokens = EXCLUDED.max_tokens,
    dimensions = EXCLUDED.dimensions,
    capabilities = EXCLUDED.capabilities,
    updated_at = NOW();

\echo 'Added Titan models to catalog'

-- 2. Configure the default tenant to use Titan V2
DO $$
DECLARE
    titan_v2_id UUID;
    titan_v1_id UUID;
    target_tenant_id UUID := '00000000-0000-0000-0000-000000000001';
BEGIN
    -- Get Titan model IDs
    SELECT id INTO titan_v2_id
    FROM mcp.embedding_model_catalog
    WHERE model_id = 'amazon.titan-embed-text-v2:0';

    SELECT id INTO titan_v1_id
    FROM mcp.embedding_model_catalog
    WHERE model_id = 'amazon.titan-embed-text-v1';

    IF titan_v2_id IS NULL THEN
        RAISE EXCEPTION 'Titan V2 model not found in catalog';
    END IF;

    -- Configure Titan V2 as primary model
    INSERT INTO mcp.tenant_embedding_models (
        tenant_id,
        model_id,
        is_enabled,
        is_default,
        monthly_token_limit,
        daily_token_limit,
        monthly_request_limit,
        priority,
        created_at,
        updated_at
    ) VALUES (
        target_tenant_id,
        titan_v2_id,
        true,        -- enabled
        true,        -- default model for this tenant
        10000000,    -- 10M tokens per month
        1000000,     -- 1M tokens per day
        100000,      -- 100k requests per month
        100,         -- high priority
        NOW(),
        NOW()
    )
    ON CONFLICT (tenant_id, model_id) DO UPDATE SET
        is_enabled = true,
        is_default = true,
        priority = EXCLUDED.priority,
        monthly_token_limit = EXCLUDED.monthly_token_limit,
        daily_token_limit = EXCLUDED.daily_token_limit,
        monthly_request_limit = EXCLUDED.monthly_request_limit,
        updated_at = NOW();

    RAISE NOTICE 'Configured Titan V2 (%) as default for tenant %', titan_v2_id, target_tenant_id;

    -- Configure Titan V1 as fallback (if exists)
    IF titan_v1_id IS NOT NULL THEN
        INSERT INTO mcp.tenant_embedding_models (
            tenant_id,
            model_id,
            is_enabled,
            is_default,
            monthly_token_limit,
            daily_token_limit,
            priority,
            created_at,
            updated_at
        ) VALUES (
            target_tenant_id,
            titan_v1_id,
            true,        -- enabled as fallback
            false,       -- not default
            5000000,     -- 5M tokens per month
            500000,      -- 500k tokens per day
            50,          -- lower priority
            NOW(),
            NOW()
        )
        ON CONFLICT (tenant_id, model_id) DO UPDATE SET
            is_enabled = true,
            priority = EXCLUDED.priority,
            updated_at = NOW();

        RAISE NOTICE 'Configured Titan V1 (%) as fallback for tenant %', titan_v1_id, target_tenant_id;
    END IF;
END $$;

\echo 'Configured tenant embedding models'

-- 3. Verify configuration
\echo ''
\echo 'Configuration Summary:'
\echo '====================='

SELECT
    c.model_id as "Model",
    c.provider as "Provider",
    c.dimensions as "Dimensions",
    c.cost_per_million_tokens as "Cost per 1M tokens",
    CASE WHEN tm.is_default THEN 'Yes' ELSE 'No' END as "Default",
    tm.priority as "Priority",
    tm.monthly_token_limit as "Monthly Token Limit",
    tm.daily_token_limit as "Daily Token Limit"
FROM mcp.embedding_model_catalog c
JOIN mcp.tenant_embedding_models tm ON c.id = tm.model_id
WHERE c.provider = 'aws_bedrock'
    AND tm.tenant_id = '00000000-0000-0000-0000-000000000001'
    AND tm.is_enabled = true
ORDER BY tm.priority DESC;

\echo ''
\echo 'Configuration complete! AWS Bedrock Titan models are ready to use.'
\echo 'Tenant ID: 00000000-0000-0000-0000-000000000001'
\echo ''
\echo 'Next steps:'
\echo '  1. Ensure AWS credentials are configured (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION)'
\echo '  2. Create a context and add content to trigger embedding generation'
\echo '  3. Check the embedding_queue and worker logs to verify processing'
