-- Add function to pad embeddings to 4096 dimensions for pgvector storage
-- This allows storing embeddings of various dimensions in a single column

CREATE OR REPLACE FUNCTION mcp.pad_embedding(p_embedding DOUBLE PRECISION[])
RETURNS vector(4096)
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
    v_input_dims INTEGER;
    v_padded DOUBLE PRECISION[4096];
BEGIN
    -- Get input dimensions
    v_input_dims := array_length(p_embedding, 1);
    
    -- Validate input dimensions
    IF v_input_dims IS NULL OR v_input_dims = 0 THEN
        RAISE EXCEPTION 'Input embedding cannot be empty';
    END IF;
    
    IF v_input_dims > 4096 THEN
        RAISE EXCEPTION 'Input embedding dimensions % exceed maximum of 4096', v_input_dims;
    END IF;
    
    -- Initialize padded array with zeros
    v_padded := array_fill(0.0::DOUBLE PRECISION, ARRAY[4096]);
    
    -- Copy input values to padded array
    FOR i IN 1..v_input_dims LOOP
        v_padded[i] := p_embedding[i];
    END LOOP;
    
    -- Convert to vector type and return
    RETURN v_padded::vector(4096);
END;
$$;

COMMENT ON FUNCTION mcp.pad_embedding IS 'Pads embeddings to 4096 dimensions for unified storage in pgvector';