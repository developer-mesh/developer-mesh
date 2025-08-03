# DBA Schema Consolidation Plan

## Executive Summary
As an expert DBA, I've identified significant gaps between the archived migrations (26 files) and the consolidated initial schema. The current initial schema is missing critical production features that evolved over 26 iterations.

## Current Situation
1. **Migration State**: Database is stuck at version 1 in a "dirty" state
2. **Schema Mismatch**: Column naming inconsistency (`type` vs `key_type`) causing authentication failures
3. **Missing Features**: ~40% of production features from archived migrations are not in the initial schema

## Critical Gaps Analysis

### 1. Data Type Mismatches
| Table | Current Schema | Required (from code/archives) | Impact |
|-------|----------------|-------------------------------|---------|
| api_keys | type VARCHAR(50) | key_type VARCHAR(50) | Auth failures |
| api_keys | Values: 'standard', 'service'... | Values: 'user', 'admin', 'agent'... | Invalid constraints |

### 2. Missing Tables (18 tables)
**High Priority - Core Functionality:**
- `tenant_config` - Multi-tenancy configuration
- `embedding_statistics` - Hybrid search support
- `task_delegation_history` - Audit trail
- `task_state_transitions` - State machine tracking
- `task_idempotency_keys` - Distributed system safety

**Medium Priority - Advanced Features:**
- `projection_matrices` - Cross-model embeddings
- `embedding_metrics` - Performance tracking
- `workspace_activities` - Collaboration audit
- `agent_configs` - Agent-specific settings
- `embedding_cache` - Performance optimization

**Materialized Views:**
- `agent_cost_analytics` - Cost tracking
- `active_embedding_models` - Model registry cache

### 3. Missing Columns
**embeddings table (8 columns):**
```
- agent_id VARCHAR(255)
- task_type VARCHAR(50)
- normalized_embedding vector(1536)
- cost_usd DECIMAL(10, 6)
- generation_time_ms INTEGER
- content_tsvector tsvector
- term_frequencies jsonb
- document_length integer
- idf_scores jsonb
```

**api_keys table (2 columns):**
```
- parent_key_id UUID
- allowed_services TEXT[]
```

**tasks table (3 columns):**
```
- auto_escalate BOOLEAN
- escalation_timeout INTERVAL
- max_delegations INTEGER
```

**workspaces table (5 columns):**
```
- max_members INTEGER
- max_storage_bytes BIGINT
- current_storage_bytes BIGINT
- max_documents INTEGER
- current_documents INTEGER
```

### 4. Missing Custom Types (6 types)
```sql
- task_status ENUM
- task_priority ENUM
- workflow_type ENUM
- workflow_status ENUM
- delegation_type ENUM
- member_role ENUM
```

### 5. Missing Functions (15 functions)
**Search Functions:**
- `bm25_score()` - Text ranking
- `reciprocal_rank_fusion()` - Hybrid search
- `search_embeddings()` - Vector search
- `update_content_tsvector()` - FTS maintenance
- `update_embedding_statistics()` - Stats maintenance

**Utility Functions:**
- `current_tenant_id()` - RLS support
- `validate_jsonb_keys()` - Data validation
- `jsonb_merge_recursive()` - Deep merge
- `refresh_cost_analytics()` - View refresh

### 6. Missing Indexes (22 indexes)
**Critical Performance Indexes:**
- Vector indexes with IVFFlat
- Full-text search GIN indexes
- Covering indexes for complex queries
- Partial indexes for active records

### 7. Missing Partitioning
- `embedding_metrics` by month
- Extended partitions for `audit_log`
- Performance settings for partitions

### 8. Missing Triggers (8 triggers)
- Audit trail triggers
- Validation triggers
- FTS update triggers

## Detailed Resolution Plan

### Phase 1: Schema Analysis & Planning (Current)
1. Complete gap analysis ✓
2. Create comprehensive plan ✓
3. Get stakeholder approval (pending)

### Phase 2: Schema Consolidation
1. **Update initial_schema.up.sql:**
   - Add all missing custom types at the beginning
   - Update api_keys table definition with correct columns
   - Add all missing tables in dependency order
   - Add all missing columns to existing tables
   - Add all functions in correct order
   - Add all indexes
   - Add all triggers
   - Add partitioning commands

2. **Update initial_schema.down.sql:**
   - Drop all objects in reverse order
   - Ensure clean rollback capability

### Phase 3: Migration State Resolution
**Option A - Clean Slate (Recommended for Dev):**
```sql
-- Drop and recreate database
DROP DATABASE IF EXISTS dev;
CREATE DATABASE dev;
-- Apply new initial schema
```

**Option B - Fix Dirty State (For preserving data):**
```sql
-- Force migration version
UPDATE schema_migrations SET dirty = false WHERE version = 1;
-- Or manually complete the migration
```

### Phase 4: Testing & Validation
1. Apply schema to fresh database
2. Verify all tables, columns, types exist
3. Test application connectivity
4. Verify authentication works
5. Test embedding functionality
6. Run integration tests

## Risk Assessment

### High Risks:
1. **Data Loss** - If Option A is used in environment with data
2. **Downtime** - Schema changes require service restart
3. **Compatibility** - Ensure Go code matches new schema

### Mitigation:
1. Full database backup before changes
2. Test in isolated environment first
3. Have rollback plan ready
4. Coordinate with development team

## Estimated Timeline
- Phase 1: Complete ✓
- Phase 2: 4-6 hours (updating schema files)
- Phase 3: 1 hour (fixing migration state)
- Phase 4: 2-3 hours (testing)

Total: 7-10 hours of work

## Recommendations

1. **Immediate Action**: Fix api_keys.type → key_type issue
2. **Short Term**: Complete schema consolidation this sprint
3. **Long Term**: Implement schema versioning strategy

## Next Steps

1. Review this plan with team
2. Decide on migration state resolution approach
3. Schedule implementation window
4. Assign resources for testing

## Appendix: Detailed Change List

### Custom Types to Add:
```sql
CREATE TYPE task_status AS ENUM ('pending', 'assigned', 'in_progress', 'completed', 'failed', 'cancelled', 'delegated');
CREATE TYPE task_priority AS ENUM ('low', 'medium', 'high', 'critical');
CREATE TYPE workflow_type AS ENUM ('sequential', 'parallel', 'conditional', 'loop', 'map_reduce', 'scatter_gather');
CREATE TYPE workflow_status AS ENUM ('draft', 'active', 'paused', 'completed', 'failed', 'archived');
CREATE TYPE delegation_type AS ENUM ('handoff', 'collaboration', 'supervision', 'consultation');
CREATE TYPE workspace_visibility AS ENUM ('private', 'team', 'organization', 'public');
CREATE TYPE member_role AS ENUM ('viewer', 'contributor', 'moderator', 'admin', 'owner');
```

### Critical Table Updates:
```sql
-- Fix api_keys table
ALTER TABLE mcp.api_keys 
  RENAME COLUMN type TO key_type;
  
-- Update constraint
ALTER TABLE mcp.api_keys 
  DROP CONSTRAINT api_keys_type_check,
  ADD CONSTRAINT api_keys_key_type_check 
  CHECK (key_type IN ('user', 'admin', 'agent', 'service', 'gateway'));
```

[Full detailed changes continue in appendix...]