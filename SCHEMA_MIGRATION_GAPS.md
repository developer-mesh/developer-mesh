# Schema Migration Gap Analysis

## Critical Missing Items in Initial Schema

### 1. API Keys Table - Missing Columns
The current schema has `type` but the code expects `key_type`. Additionally missing:
```sql
-- Current has: type VARCHAR(50) DEFAULT 'standard' CHECK (type IN ('standard', 'service', 'personal', 'temporary'))
-- Should have:
key_type VARCHAR(50) NOT NULL DEFAULT 'user' CHECK (key_type IN ('user', 'admin', 'agent', 'service', 'gateway')),
parent_key_id UUID REFERENCES mcp.api_keys(id),
allowed_services TEXT[] DEFAULT '{}'
```

### 2. Missing Custom Types
```sql
-- Task management types
CREATE TYPE task_status AS ENUM ('pending', 'assigned', 'in_progress', 'completed', 'failed', 'cancelled', 'delegated');
CREATE TYPE task_priority AS ENUM ('low', 'medium', 'high', 'critical');

-- Workflow types
CREATE TYPE workflow_type AS ENUM ('sequential', 'parallel', 'conditional', 'loop', 'map_reduce', 'scatter_gather');
CREATE TYPE workflow_status AS ENUM ('draft', 'active', 'paused', 'completed', 'failed', 'archived');

-- Delegation types
CREATE TYPE delegation_type AS ENUM ('handoff', 'collaboration', 'supervision', 'consultation');

-- Workspace types
CREATE TYPE workspace_visibility AS ENUM ('private', 'team', 'organization', 'public');
CREATE TYPE member_role AS ENUM ('viewer', 'contributor', 'moderator', 'admin', 'owner');
```

### 3. Hybrid Search Features (from migration 026)
```sql
-- Add to embeddings table
ALTER TABLE embeddings ADD COLUMN content_tsvector tsvector;
ALTER TABLE embeddings ADD COLUMN term_frequencies jsonb;
ALTER TABLE embeddings ADD COLUMN document_length integer;
ALTER TABLE embeddings ADD COLUMN idf_scores jsonb;

-- Add indexes
CREATE INDEX idx_embeddings_fts ON embeddings USING gin(content_tsvector);

-- Add statistics table
CREATE TABLE embedding_statistics (
    collection_id VARCHAR(255) PRIMARY KEY,
    total_documents INTEGER NOT NULL DEFAULT 0,
    avg_document_length FLOAT NOT NULL DEFAULT 0.0,
    term_document_counts JSONB NOT NULL DEFAULT '{}',
    last_updated TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### 4. Advanced Task Management Tables
```sql
-- Task delegation history
CREATE TABLE task_delegation_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    from_agent_id UUID REFERENCES agents(id),
    to_agent_id UUID NOT NULL REFERENCES agents(id),
    delegation_type VARCHAR(50),
    reason TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Task state transitions
CREATE TABLE task_state_transitions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    from_status task_status,
    to_status task_status NOT NULL,
    transitioned_by UUID REFERENCES agents(id),
    reason TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Idempotency support
CREATE TABLE task_idempotency_keys (
    idempotency_key VARCHAR(255) PRIMARY KEY,
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);
```

### 5. Tenant Configuration Table
```sql
CREATE TABLE IF NOT EXISTS mcp.tenant_config (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID UNIQUE NOT NULL,
    rate_limit_config JSONB NOT NULL DEFAULT '{}',
    service_tokens JSONB DEFAULT '{}',
    allowed_origins TEXT[] DEFAULT '{}',
    features JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

### 6. Missing Functions
```sql
-- BM25 scoring
CREATE OR REPLACE FUNCTION bm25_score(
    term_freq FLOAT,
    doc_freq INTEGER,
    total_docs INTEGER,
    doc_length INTEGER,
    avg_doc_length FLOAT,
    k1 FLOAT DEFAULT 1.2,
    b FLOAT DEFAULT 0.75
) RETURNS FLOAT AS $$
BEGIN
    RETURN ((term_freq * (k1 + 1)) / 
            (term_freq + k1 * (1 - b + b * (doc_length / avg_doc_length)))) *
           ln((total_docs - doc_freq + 0.5) / (doc_freq + 0.5));
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Reciprocal rank fusion
CREATE OR REPLACE FUNCTION reciprocal_rank_fusion(
    vector_scores FLOAT[],
    text_scores FLOAT[],
    k INTEGER DEFAULT 60
) RETURNS TABLE(idx INTEGER, score FLOAT) AS $$
BEGIN
    -- Implementation for combining vector and text scores
END;
$$ LANGUAGE plpgsql;
```

### 7. Missing Columns in Existing Tables

**embeddings table:**
```sql
ALTER TABLE embeddings 
ADD COLUMN agent_id VARCHAR(255),
ADD COLUMN task_type VARCHAR(50),
ADD COLUMN normalized_embedding vector(1536),
ADD COLUMN cost_usd DECIMAL(10, 6),
ADD COLUMN generation_time_ms INTEGER;
```

**tasks table:**
```sql
ALTER TABLE tasks 
ADD COLUMN auto_escalate BOOLEAN DEFAULT FALSE,
ADD COLUMN escalation_timeout INTERVAL,
ADD COLUMN max_delegations INTEGER DEFAULT 3;
```

**agents table:**
```sql
ALTER TABLE agents 
ADD COLUMN last_seen_at TIMESTAMP WITH TIME ZONE;
```

### 8. Missing Indexes
```sql
-- Performance indexes
CREATE INDEX idx_embeddings_agent_id ON embeddings(agent_id);
CREATE INDEX idx_embeddings_task_type ON embeddings(task_type);
CREATE INDEX idx_task_delegation_history_task ON task_delegation_history(task_id);
CREATE INDEX idx_task_state_transitions_task ON task_state_transitions(task_id);
CREATE INDEX idx_task_idempotency_expires ON task_idempotency_keys(expires_at);
```

## Action Items

1. **Update Initial Schema**: Add all missing tables, columns, types, and functions
2. **Fix API Keys Table**: Rename `type` to `key_type` with correct values
3. **Add Hybrid Search**: Include BM25 scoring and full-text search features
4. **Add Advanced Features**: Include task delegation, idempotency, and tenant config
5. **Create Migration Strategy**: Since the DB is in a dirty state, we may need to:
   - Fix the dirty migration state
   - Create a new migration that adds missing features
   - Or recreate the database with the complete schema

## Database Migration State Issue

Current state:
- Database is at migration version 1 in a "dirty" state
- This prevents new migrations from running
- Need to either:
  1. Force clean the migration state
  2. Drop and recreate the database
  3. Manually apply the missing schema changes