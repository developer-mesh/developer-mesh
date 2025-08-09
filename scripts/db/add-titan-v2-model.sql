-- Add Amazon Titan Embed Text v2:0 model to embedding_models table
-- This model is required for the greenfield embedding service

INSERT INTO embedding_models (
    provider,
    model_name,
    model_id,
    dimensions,
    max_tokens,
    cost_per_1m_tokens,
    is_active,
    created_at,
    updated_at
) VALUES (
    'amazon',
    'titan-embed-text-v2',
    'amazon.titan-embed-text-v2:0',
    1024,  -- v2 has 1024 dimensions
    8192,
    0.02,  -- $0.02 per 1M tokens
    true,
    NOW(),
    NOW()
) ON CONFLICT (provider, model_name) 
DO UPDATE SET
    model_id = EXCLUDED.model_id,
    dimensions = EXCLUDED.dimensions,
    is_active = EXCLUDED.is_active,
    updated_at = NOW();

-- Verify the model was added
SELECT provider, model_name, model_id, dimensions, is_active 
FROM embedding_models 
WHERE model_id = 'amazon.titan-embed-text-v2:0';