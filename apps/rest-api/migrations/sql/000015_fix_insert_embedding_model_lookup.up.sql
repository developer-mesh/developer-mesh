-- Fix insert_embedding function to check both model_id and model_name
-- This allows compatibility with code that passes either field
-- Also update all model_id values to match model_name for consistency

-- First, update all model_id values to match model_name
UPDATE mcp.embedding_models 
SET model_id = model_name 
WHERE model_id IS NULL OR model_id = '';

CREATE OR REPLACE FUNCTION mcp.insert_embedding(
    p_context_id UUID,
    p_content TEXT,
    p_embedding FLOAT[],
    p_model_name TEXT,
    p_tenant_id UUID,
    p_metadata JSONB DEFAULT '{}',
    p_content_index INTEGER DEFAULT 0,
    p_chunk_index INTEGER DEFAULT 0,
    p_configured_dimensions INTEGER DEFAULT NULL
) RETURNS UUID AS $$
DECLARE
    v_id UUID;
    v_model_id UUID;
    v_model_provider VARCHAR(50);
    v_model_name VARCHAR(100);
    v_dimensions INTEGER;
    v_supports_reduction BOOLEAN;
    v_min_dimensions INTEGER;
    v_padded_embedding vector(4096);
    v_actual_dimensions INTEGER;
    v_content_hash VARCHAR(64);
BEGIN
    -- Get model info - check both model_id and model_name for compatibility
    SELECT id, provider, model_name, dimensions, supports_dimensionality_reduction, min_dimensions
    INTO v_model_id, v_model_provider, v_model_name, v_dimensions, v_supports_reduction, v_min_dimensions
    FROM mcp.embedding_models
    WHERE (model_id = p_model_name OR model_name = p_model_name)
    AND is_active = true
    LIMIT 1;
    
    IF v_model_id IS NULL THEN
        RAISE EXCEPTION 'Model % not found or inactive', p_model_name;
    END IF;
    
    -- Determine actual dimensions
    v_actual_dimensions := COALESCE(p_configured_dimensions, v_dimensions);
    
    -- Validate configured dimensions if provided
    IF p_configured_dimensions IS NOT NULL THEN
        IF NOT v_supports_reduction THEN
            RAISE EXCEPTION 'Model % does not support dimension reduction', p_model_name;
        END IF;
        IF p_configured_dimensions < v_min_dimensions OR p_configured_dimensions > v_dimensions THEN
            RAISE EXCEPTION 'Configured dimensions % outside valid range [%, %] for model %', 
                p_configured_dimensions, v_min_dimensions, v_dimensions, p_model_name;
        END IF;
    END IF;
    
    -- Validate embedding dimensions
    IF array_length(p_embedding, 1) != v_actual_dimensions THEN
        RAISE EXCEPTION 'Embedding dimensions % do not match expected dimensions %', 
            array_length(p_embedding, 1), v_actual_dimensions;
    END IF;
    
    -- Calculate content hash
    v_content_hash := encode(sha256(p_content::bytea), 'hex');
    
    -- Pad embedding to 4096 dimensions
    v_padded_embedding := mcp.pad_embedding(p_embedding);
    
    -- Insert the embedding (including model_name)
    INSERT INTO mcp.embeddings (
        context_id, content, embedding, model_id, model_provider, model_name, model_dimensions,
        tenant_id, metadata, content_index, chunk_index, content_hash, configured_dimensions
    ) VALUES (
        p_context_id, p_content, v_padded_embedding, v_model_id, v_model_provider, v_model_name, v_actual_dimensions,
        p_tenant_id, p_metadata, p_content_index, p_chunk_index, v_content_hash, p_configured_dimensions
    ) RETURNING id INTO v_id;
    
    RETURN v_id;
END;
$$ LANGUAGE plpgsql;