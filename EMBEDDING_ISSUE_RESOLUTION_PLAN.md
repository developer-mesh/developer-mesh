# Embedding Service Resolution Plan

## Executive Summary
The embedding service is failing due to a critical schema mismatch between the database migration and the Go application code. The `agent_configs` table in the current schema uses `agent_id` as the primary key, while the Go code expects a separate `id` column with additional fields for versioning and configuration management.

## Root Cause Analysis

### Issue Details
- **Error**: `pq: column "id" does not exist` when calling `/api/v1/embeddings`
- **Location**: `pkg/agents/repository.go:140` - GetConfig() method
- **Impact**: Complete failure of embedding generation functionality

### Schema Comparison

#### Current Schema (Simplified)
```sql
CREATE TABLE agent_configs (
    agent_id UUID PRIMARY KEY,
    embedding_model_id UUID,
    embedding_config JSONB,
    cost_limit_usd DECIMAL(10, 2),
    rate_limit_per_minute INTEGER,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

#### Expected Schema (From Go Code)
```sql
CREATE TABLE agent_configs (
    id UUID PRIMARY KEY,
    agent_id UUID NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    embedding_strategy VARCHAR(50),
    model_preferences JSONB,
    constraints JSONB,
    fallback_behavior JSONB,
    metadata JSONB,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    created_by UUID
);
```

## Resolution Strategy

### Option 1: Update Database Schema (Recommended)
**Rationale**: The Go code implements comprehensive agent configuration with versioning, strategies, and constraints. This functionality should be preserved.

### Option 2: Simplify Go Code
**Rationale**: If the advanced features aren't needed, simplify the Go code to match the current schema.

## Implementation Plan (Option 1 - Recommended)

### Phase 1: Schema Analysis and Validation
1. **Review Archive Migration**: Examine `000005_multi_agent_embeddings.up.sql` for the complete schema
2. **Identify Dependencies**: Check for related tables and foreign keys
3. **Validate Go Code**: Ensure all repository methods align with the new schema

### Phase 2: Schema Update
1. **Update Initial Schema**: Modify `000001_initial_schema.up.sql` to include the complete agent_configs table
2. **Add Missing Columns**:
   - `id UUID PRIMARY KEY DEFAULT uuid_generate_v4()`
   - `version INTEGER NOT NULL DEFAULT 1`
   - `embedding_strategy VARCHAR(50)`
   - `model_preferences JSONB DEFAULT '{}'`
   - `constraints JSONB DEFAULT '{}'`
   - `fallback_behavior JSONB DEFAULT '{}'`
   - `metadata JSONB DEFAULT '{}'`
   - `is_active BOOLEAN DEFAULT true`
   - `created_by UUID`
3. **Update Indexes**: Add necessary indexes for performance
4. **Update Foreign Keys**: Ensure proper relationships

### Phase 3: Data Migration
1. **Handle Existing Data**: If any data exists, migrate it to the new schema
2. **Set Default Values**: Ensure all new columns have appropriate defaults

### Phase 4: Testing
1. **Unit Tests**: Update repository tests for the new schema
2. **Integration Tests**: Test the full embedding flow
3. **AWS Bedrock Verification**: Confirm embeddings are generated via Bedrock
4. **Error Handling**: Test edge cases and error scenarios

### Phase 5: Additional Fixes
1. **SQLite Detection**: Fix the SQLite version check that's running on PostgreSQL
2. **Prepared Statements**: Update any prepared statements referencing agent_configs
3. **Documentation**: Update API documentation for agent configuration

## Task Breakdown

### Immediate Tasks (High Priority)
1. ✅ Research embedding issues and create resolution plan
2. ⬜ Update agent_configs table schema in initial_schema.up.sql
3. ⬜ Test embedding endpoint with corrected schema
4. ⬜ Fix SQLite detection code trying to run on PostgreSQL

### Follow-up Tasks (Medium Priority)
5. ⬜ Add integration tests for embedding service
6. ⬜ Verify AWS Bedrock integration is working
7. ⬜ Update API documentation
8. ⬜ Add monitoring for embedding operations

### Long-term Tasks (Low Priority)
9. ⬜ Implement agent configuration versioning UI
10. ⬜ Add cost tracking for embeddings
11. ⬜ Implement fallback strategies for provider failures

## Risk Mitigation
1. **Backup Current State**: Before making changes, ensure backups exist
2. **Feature Flag**: Consider adding a feature flag for the new schema
3. **Rollback Plan**: Prepare down migration for quick rollback
4. **Monitoring**: Add alerts for embedding failures

## Success Criteria
1. Embedding endpoint returns successful response
2. AWS Bedrock is used for embedding generation
3. Agent configurations are properly stored and retrieved
4. No SQLite errors in PostgreSQL logs
5. All tests pass

## Timeline
- **Day 1**: Schema analysis and update (2-3 hours)
- **Day 2**: Testing and validation (2-3 hours)
- **Day 3**: Documentation and monitoring (1-2 hours)

## Dependencies
- Database access for schema updates
- AWS credentials for Bedrock testing
- Test agent configurations

## Next Steps
1. Review this plan with the team
2. Get approval for schema changes
3. Begin implementation following the task breakdown