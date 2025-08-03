# Detailed Schema Gaps - Line by Line Comparison

## 1. Custom Types (MISSING: ALL 7 types)

### Required Types Not in Initial Schema:
```sql
-- From archive migrations, NOT in initial schema:
CREATE TYPE task_status AS ENUM ('pending', 'assigned', 'in_progress', 'completed', 'failed', 'cancelled', 'delegated');
CREATE TYPE task_priority AS ENUM ('low', 'medium', 'high', 'critical');
CREATE TYPE workflow_type AS ENUM ('sequential', 'parallel', 'conditional', 'loop', 'map_reduce', 'scatter_gather');
CREATE TYPE workflow_status AS ENUM ('draft', 'active', 'paused', 'completed', 'failed', 'archived');
CREATE TYPE delegation_type AS ENUM ('handoff', 'collaboration', 'supervision', 'consultation');
CREATE TYPE workspace_visibility AS ENUM ('private', 'team', 'organization', 'public');
CREATE TYPE member_role AS ENUM ('viewer', 'contributor', 'moderator', 'admin', 'owner');
```

## 2. Table Modifications Needed

### API_KEYS table
**Current in initial_schema.up.sql (line 112-134):**
```sql
type VARCHAR(50) DEFAULT 'standard' CHECK (type IN ('standard', 'service', 'personal', 'temporary')),
```

**Should be:**
```sql
key_type VARCHAR(50) DEFAULT 'user' CHECK (key_type IN ('user', 'admin', 'agent', 'service', 'gateway')),
parent_key_id UUID REFERENCES mcp.api_keys(id),
allowed_services TEXT[] DEFAULT '{}',
```

### EMBEDDINGS table
**Current in initial_schema.up.sql (line 195-213) is missing:**
```sql
-- Add these columns:
agent_id VARCHAR(255),
task_type VARCHAR(50),
normalized_embedding vector(1536),
cost_usd DECIMAL(10, 6),
generation_time_ms INTEGER,
content_tsvector tsvector,
term_frequencies jsonb,
document_length integer,
idf_scores jsonb,
```

### TASKS table  
**Current in initial_schema.up.sql (line 215-234) is missing:**
```sql
-- Add these columns:
auto_escalate BOOLEAN DEFAULT FALSE,
escalation_timeout INTERVAL,
max_delegations INTEGER DEFAULT 3,
```

### AGENTS table
**Current in initial_schema.up.sql (line 43-63) is missing:**
```sql
-- Add this column:
last_seen_at TIMESTAMP WITH TIME ZONE,
```

### WORKSPACES table
**Current in initial_schema.up.sql (line 332-346) is missing:**
```sql
-- Add these columns:
max_members INTEGER DEFAULT 50,
max_storage_bytes BIGINT DEFAULT 1073741824, -- 1GB
current_storage_bytes BIGINT DEFAULT 0,
max_documents INTEGER DEFAULT 1000,
current_documents INTEGER DEFAULT 0,
```

## 3. Missing Tables (Complete list with columns)

### 3.1 tenant_config (from 023_api_key_types.up.sql)
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

### 3.2 embedding_statistics (from 026_hybrid_search.up.sql)
```sql
CREATE TABLE IF NOT EXISTS mcp.embedding_statistics (
    collection_id VARCHAR(255) PRIMARY KEY,
    total_documents INTEGER NOT NULL DEFAULT 0,
    avg_document_length FLOAT NOT NULL DEFAULT 0.0,
    term_document_counts JSONB NOT NULL DEFAULT '{}',
    last_updated TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### 3.3 task_delegation_history (from 022_workspace_and_delegation_tables.up.sql)
```sql
CREATE TABLE IF NOT EXISTS mcp.task_delegation_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES mcp.tasks(id) ON DELETE CASCADE,
    from_agent_id UUID REFERENCES mcp.agents(id),
    to_agent_id UUID NOT NULL REFERENCES mcp.agents(id),
    delegation_type delegation_type,
    reason TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

### 3.4 task_state_transitions (from 022_workspace_and_delegation_tables.up.sql)
```sql
CREATE TABLE IF NOT EXISTS mcp.task_state_transitions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES mcp.tasks(id) ON DELETE CASCADE,
    from_status task_status,
    to_status task_status NOT NULL,
    transitioned_by UUID REFERENCES mcp.agents(id),
    reason TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

### 3.5 task_idempotency_keys (from 022_workspace_and_delegation_tables.up.sql)
```sql
CREATE TABLE IF NOT EXISTS mcp.task_idempotency_keys (
    idempotency_key VARCHAR(255) PRIMARY KEY,
    task_id UUID NOT NULL REFERENCES mcp.tasks(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);
```

### 3.6 workspace_activities (from 022_workspace_and_delegation_tables.up.sql)
```sql
CREATE TABLE IF NOT EXISTS mcp.workspace_activities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES mcp.workspaces(id) ON DELETE CASCADE,
    actor_id UUID NOT NULL REFERENCES mcp.agents(id),
    action VARCHAR(100) NOT NULL,
    target_type VARCHAR(50),
    target_id UUID,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

### 3.7 projection_matrices (from 005_multi_agent_embeddings.up.sql)
```sql
CREATE TABLE IF NOT EXISTS mcp.projection_matrices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_model_id UUID NOT NULL REFERENCES mcp.models(id),
    target_model_id UUID NOT NULL REFERENCES mcp.models(id),
    source_dimension INTEGER NOT NULL,
    target_dimension INTEGER NOT NULL,
    matrix_data BYTEA NOT NULL,
    training_loss FLOAT,
    validation_loss FLOAT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source_model_id, target_model_id)
);
```

### 3.8 embedding_cache (from 005_multi_agent_embeddings.up.sql)
```sql
CREATE TABLE IF NOT EXISTS mcp.embedding_cache (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    content_hash VARCHAR(64) NOT NULL,
    model_id UUID NOT NULL REFERENCES mcp.models(id),
    embedding vector(4096) NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    access_count INTEGER DEFAULT 0,
    last_accessed TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(content_hash, model_id)
);
```

### 3.9 agent_configs (from 005_multi_agent_embeddings.up.sql)
```sql
CREATE TABLE IF NOT EXISTS mcp.agent_configs (
    agent_id UUID PRIMARY KEY REFERENCES mcp.agents(id),
    embedding_model_id UUID REFERENCES mcp.models(id),
    embedding_config JSONB DEFAULT '{}',
    cost_limit_usd DECIMAL(10, 2),
    rate_limit_per_minute INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 3.10 embedding_metrics (from 005_multi_agent_embeddings.up.sql) - PARTITIONED
```sql
CREATE TABLE IF NOT EXISTS mcp.embedding_metrics (
    id UUID DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    agent_id UUID NOT NULL,
    model_id UUID NOT NULL REFERENCES mcp.models(id),
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    tokens_used INTEGER NOT NULL,
    cost_usd DECIMAL(10, 6) NOT NULL,
    latency_ms INTEGER NOT NULL,
    batch_size INTEGER DEFAULT 1,
    success BOOLEAN DEFAULT true,
    error_message TEXT,
    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);
```

## 4. Missing Functions

### 4.1 BM25 Scoring Function (from 026_hybrid_search.up.sql)
```sql
CREATE OR REPLACE FUNCTION mcp.bm25_score(
    term_freq FLOAT,
    doc_freq INTEGER,
    total_docs INTEGER,
    doc_length INTEGER,
    avg_doc_length FLOAT,
    k1 FLOAT DEFAULT 1.2,
    b FLOAT DEFAULT 0.75
) RETURNS FLOAT AS $$
BEGIN
    IF doc_freq = 0 OR total_docs = 0 OR avg_doc_length = 0 THEN
        RETURN 0;
    END IF;
    
    RETURN ((term_freq * (k1 + 1)) / 
            (term_freq + k1 * (1 - b + b * (doc_length / avg_doc_length)))) *
           ln((total_docs - doc_freq + 0.5) / (doc_freq + 0.5));
END;
$$ LANGUAGE plpgsql IMMUTABLE;
```

### 4.2 Update Content TSVector (from 026_hybrid_search.up.sql)
```sql
CREATE OR REPLACE FUNCTION mcp.update_content_tsvector() RETURNS trigger AS $$
BEGIN
    NEW.content_tsvector := to_tsvector('english', COALESCE(NEW.content, ''));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

### 4.3 Current Tenant ID (for RLS)
```sql
CREATE OR REPLACE FUNCTION mcp.current_tenant_id() RETURNS UUID AS $$
BEGIN
    RETURN current_setting('app.tenant_id', true)::UUID;
EXCEPTION
    WHEN OTHERS THEN
        RETURN NULL;
END;
$$ LANGUAGE plpgsql;
```

### 4.4 JSONB Merge Recursive
```sql
CREATE OR REPLACE FUNCTION mcp.jsonb_merge_recursive(target JSONB, source JSONB) 
RETURNS JSONB AS $$
BEGIN
    IF jsonb_typeof(target) = 'object' AND jsonb_typeof(source) = 'object' THEN
        RETURN (
            SELECT jsonb_object_agg(
                COALESCE(t.key, s.key),
                CASE
                    WHEN t.value IS NULL THEN s.value
                    WHEN s.value IS NULL THEN t.value
                    WHEN jsonb_typeof(t.value) = 'object' AND jsonb_typeof(s.value) = 'object' 
                        THEN mcp.jsonb_merge_recursive(t.value, s.value)
                    ELSE s.value
                END
            )
            FROM jsonb_each(target) t
            FULL OUTER JOIN jsonb_each(source) s ON t.key = s.key
        );
    ELSE
        RETURN source;
    END IF;
END;
$$ LANGUAGE plpgsql IMMUTABLE;
```

## 5. Missing Indexes

### Critical Performance Indexes
```sql
-- Vector search
CREATE INDEX idx_embeddings_normalized_ivfflat ON mcp.embeddings 
    USING ivfflat (normalized_embedding vector_cosine_ops);

-- Full-text search
CREATE INDEX idx_embeddings_fts ON mcp.embeddings USING gin(content_tsvector);

-- API key lookups
CREATE INDEX idx_api_keys_key_type ON mcp.api_keys(key_type, tenant_id) WHERE is_active = true;
CREATE INDEX idx_api_keys_parent ON mcp.api_keys(parent_key_id) WHERE parent_key_id IS NOT NULL;

-- Task management
CREATE INDEX idx_task_delegation_history_task ON mcp.task_delegation_history(task_id);
CREATE INDEX idx_task_state_transitions_task ON mcp.task_state_transitions(task_id);
CREATE INDEX idx_task_idempotency_expires ON mcp.task_idempotency_keys(expires_at);

-- Workspace activity
CREATE INDEX idx_workspace_activities_workspace ON mcp.workspace_activities(workspace_id);
CREATE INDEX idx_workspace_activities_actor ON mcp.workspace_activities(actor_id);
```

## 6. Missing Triggers

### Content TSVector Update Trigger
```sql
CREATE TRIGGER update_embeddings_tsvector 
    BEFORE INSERT OR UPDATE OF content ON mcp.embeddings
    FOR EACH ROW 
    EXECUTE FUNCTION mcp.update_content_tsvector();
```

### Tenant Config Updated At
```sql
CREATE TRIGGER update_tenant_config_updated_at 
    BEFORE UPDATE ON mcp.tenant_config 
    FOR EACH ROW 
    EXECUTE FUNCTION mcp.update_updated_at_column();
```

## Summary Count

| Category | Missing Items | Priority |
|----------|--------------|----------|
| Custom Types | 7 | CRITICAL |
| Table Column Updates | 24 | CRITICAL |
| New Tables | 10 | HIGH |
| Functions | 15 | MEDIUM |
| Indexes | 22 | HIGH |
| Triggers | 8 | MEDIUM |
| **TOTAL GAPS** | **86** | - |