BEGIN;

-- Add full-text search columns
ALTER TABLE embeddings 
    ADD COLUMN IF NOT EXISTS content_tsvector tsvector,
    ADD COLUMN IF NOT EXISTS term_frequencies jsonb,
    ADD COLUMN IF NOT EXISTS document_length integer,
    ADD COLUMN IF NOT EXISTS idf_scores jsonb;

-- Create GIN indexes for full-text search
CREATE INDEX IF NOT EXISTS idx_embeddings_fts ON embeddings USING gin(content_tsvector);
CREATE INDEX IF NOT EXISTS idx_embeddings_trigram ON embeddings USING gin(content gin_trgm_ops);

-- Create function to update tsvector
CREATE OR REPLACE FUNCTION update_content_tsvector() RETURNS trigger AS $$
BEGIN
    NEW.content_tsvector := to_tsvector('english', NEW.content);
    NEW.document_length := array_length(string_to_array(NEW.content, ' '), 1);
    RETURN NEW;
END
$$ LANGUAGE plpgsql;

-- Create trigger
CREATE TRIGGER embeddings_tsvector_update BEFORE INSERT OR UPDATE ON embeddings
    FOR EACH ROW EXECUTE FUNCTION update_content_tsvector();

-- Create BM25 scoring function
CREATE OR REPLACE FUNCTION bm25_score(
    query_terms text[],
    doc_tsvector tsvector,
    doc_length integer,
    avg_doc_length float,
    total_docs integer,
    k1 float DEFAULT 1.2,
    b float DEFAULT 0.75
) RETURNS float AS $$
DECLARE
    score float := 0;
    term text;
    tf integer;
    df integer;
    idf float;
BEGIN
    FOREACH term IN ARRAY query_terms
    LOOP
        -- Get term frequency
        tf := ts_rank_cd(doc_tsvector, plainto_tsquery(term))::integer;
        
        -- Get document frequency (simplified - in production, use cached values)
        SELECT COUNT(*) INTO df FROM embeddings WHERE content_tsvector @@ plainto_tsquery(term);
        
        -- Calculate IDF
        idf := ln((total_docs - df + 0.5) / (df + 0.5));
        
        -- Calculate BM25 component
        score := score + (idf * tf * (k1 + 1)) / (tf + k1 * (1 - b + b * (doc_length / avg_doc_length)));
    END LOOP;
    
    RETURN score;
END;
$$ LANGUAGE plpgsql;

-- Create reciprocal rank fusion function
CREATE OR REPLACE FUNCTION reciprocal_rank_fusion(
    vector_scores float[],
    keyword_scores float[],
    vector_ids uuid[],
    keyword_ids uuid[],
    k integer DEFAULT 60
) RETURNS TABLE(id uuid, score float) AS $$
DECLARE
    i integer;
    combined_scores jsonb := '{}'::jsonb;
    result_id uuid;
BEGIN
    -- Process vector search results
    FOR i IN 1..array_length(vector_ids, 1)
    LOOP
        combined_scores := jsonb_set(
            combined_scores,
            array[vector_ids[i]::text],
            to_jsonb(COALESCE((combined_scores->>(vector_ids[i]::text))::float, 0) + 1.0 / (k + i))
        );
    END LOOP;
    
    -- Process keyword search results
    FOR i IN 1..array_length(keyword_ids, 1)
    LOOP
        combined_scores := jsonb_set(
            combined_scores,
            array[keyword_ids[i]::text],
            to_jsonb(COALESCE((combined_scores->>(keyword_ids[i]::text))::float, 0) + 1.0 / (k + i))
        );
    END LOOP;
    
    -- Return sorted results
    FOR result_id, score IN
        SELECT key::uuid, value::float
        FROM jsonb_each_text(combined_scores)
        ORDER BY value::float DESC
    LOOP
        RETURN NEXT;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Update existing embeddings with FTS data
UPDATE embeddings SET content = content WHERE content IS NOT NULL;

-- Create statistics table for BM25
CREATE TABLE IF NOT EXISTS embedding_statistics (
    id SERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL,
    total_documents INTEGER NOT NULL DEFAULT 0,
    avg_document_length FLOAT NOT NULL DEFAULT 0,
    term_document_frequencies JSONB DEFAULT '{}',
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenant(id) ON DELETE CASCADE
);

-- Create index on tenant_id
CREATE INDEX IF NOT EXISTS idx_embedding_statistics_tenant ON embedding_statistics(tenant_id);

-- Function to update statistics
CREATE OR REPLACE FUNCTION update_embedding_statistics(p_tenant_id UUID) RETURNS void AS $$
BEGIN
    INSERT INTO embedding_statistics (tenant_id, total_documents, avg_document_length)
    SELECT 
        p_tenant_id,
        COUNT(*),
        AVG(document_length)::float
    FROM embeddings
    WHERE tenant_id = p_tenant_id
    ON CONFLICT (tenant_id) DO UPDATE SET
        total_documents = EXCLUDED.total_documents,
        avg_document_length = EXCLUDED.avg_document_length,
        last_updated = CURRENT_TIMESTAMP;
END;
$$ LANGUAGE plpgsql;

COMMIT;