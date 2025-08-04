# Final Schema Gap Analysis - Initial Schema vs Application Requirements

## Overview

After implementing AWS Bedrock embeddings and reviewing the application code, here's a comprehensive analysis of remaining schema gaps.

## Critical Gaps (Blocking Features)

### 1. Dynamic Tools System Tables

The application expects these tables but they're not in the schema:

#### `tool_configurations`
```sql
-- Referenced in: internal/services/dynamic_tools_service.go
CREATE TABLE IF NOT EXISTS mcp.tool_configurations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    base_url TEXT NOT NULL,
    auth_config JSONB DEFAULT '{}',
    api_spec JSONB,
    discovered_endpoints JSONB DEFAULT '[]',
    health_check_config JSONB DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, name)
);
```

#### `tool_discovery_sessions`
```sql
-- Referenced in: internal/services/dynamic_tools_service.go:369
CREATE TABLE IF NOT EXISTS mcp.tool_discovery_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tool_id UUID REFERENCES mcp.tool_configurations(id),
    status VARCHAR(50) DEFAULT 'pending',
    discovered_endpoints INTEGER DEFAULT 0,
    error_message TEXT,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP
);
```

#### `tool_discovery_patterns`
```sql
-- Referenced in: internal/storage/discovery_repository.go:26
CREATE TABLE IF NOT EXISTS mcp.tool_discovery_patterns (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    domain VARCHAR(255) UNIQUE NOT NULL,
    successful_paths JSONB DEFAULT '[]',
    auth_method VARCHAR(50),
    api_format VARCHAR(50),
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    success_count INTEGER DEFAULT 0
);
```

#### `tool_executions`
```sql
-- Referenced in: internal/services/dynamic_tools_service.go:605
CREATE TABLE IF NOT EXISTS mcp.tool_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tool_id UUID REFERENCES mcp.tool_configurations(id),
    action VARCHAR(255) NOT NULL,
    input_data JSONB DEFAULT '{}',
    output_data JSONB,
    status VARCHAR(50) DEFAULT 'pending',
    error_message TEXT,
    executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    duration_ms INTEGER
);
```

## Non-Critical Gaps (Performance/Future Features)

### 2. Missing Indexes for Optimization

#### Composite Indexes for Common Queries
```sql
-- Tenant + Status queries (common pattern)
CREATE INDEX idx_agents_tenant_status ON mcp.agents(tenant_id, status);
CREATE INDEX idx_tasks_tenant_status ON mcp.tasks(tenant_id, status);
CREATE INDEX idx_contexts_tenant_status ON mcp.contexts(tenant_id, status);

-- Timestamp-based queries
CREATE INDEX idx_embeddings_created_desc ON mcp.embeddings(created_at DESC);
CREATE INDEX idx_tasks_deadline ON mcp.tasks(deadline) WHERE deadline IS NOT NULL;

-- JSON path indexes
CREATE INDEX idx_agents_capabilities ON mcp.agents USING gin(capabilities);
CREATE INDEX idx_models_capabilities ON mcp.models USING gin(capabilities);
```

### 3. Missing Monitoring Tables

#### Performance Metrics
```sql
-- Query performance tracking
CREATE TABLE IF NOT EXISTS mcp.query_metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    query_hash VARCHAR(64) NOT NULL,
    query_type VARCHAR(50) NOT NULL,
    execution_time_ms INTEGER NOT NULL,
    rows_returned INTEGER,
    tenant_id UUID,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- API endpoint metrics
CREATE TABLE IF NOT EXISTS mcp.endpoint_metrics (
    endpoint VARCHAR(255) NOT NULL,
    method VARCHAR(10) NOT NULL,
    status_code INTEGER NOT NULL,
    response_time_ms INTEGER NOT NULL,
    tenant_id UUID,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 4. Missing Business Logic Tables

#### Rate Limiting
```sql
-- Per-tenant rate limit tracking
CREATE TABLE IF NOT EXISTS mcp.rate_limit_buckets (
    tenant_id UUID NOT NULL,
    bucket_key VARCHAR(255) NOT NULL,
    tokens INTEGER NOT NULL,
    last_refill TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, bucket_key)
);
```

#### Billing/Usage
```sql
-- Usage tracking for billing
CREATE TABLE IF NOT EXISTS mcp.usage_metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id UUID,
    quantity DECIMAL(10, 4) NOT NULL,
    unit VARCHAR(20) NOT NULL,
    cost_usd DECIMAL(10, 6),
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## Schema Inconsistencies

### 1. Column Type Mismatches
- `agents.model_id` is VARCHAR(255) but should potentially be UUID for consistency
- Some JSONB columns lack proper validation constraints

### 2. Missing Check Constraints
```sql
-- Add validation constraints
ALTER TABLE mcp.agents 
    ADD CONSTRAINT check_temperature_range 
    CHECK (temperature >= 0 AND temperature <= 2);

ALTER TABLE mcp.agents 
    ADD CONSTRAINT check_workload 
    CHECK (current_workload >= 0 AND current_workload <= max_workload);
```

### 3. Missing Foreign Key Constraints
```sql
-- Task delegation relationships
ALTER TABLE mcp.task_delegations 
    ADD CONSTRAINT fk_task_id 
    FOREIGN KEY (task_id) 
    REFERENCES mcp.tasks(id) ON DELETE CASCADE;
```

## Recommendations

### Immediate (Blocking Features)
1. **Add Dynamic Tools Tables**: Required for tool discovery feature
   - Priority: HIGH
   - Impact: REST API endpoints won't function without these

2. **Add Missing Indexes**: For query performance
   - Priority: MEDIUM
   - Impact: Slow queries under load

### Short Term (1-2 weeks)
1. **Add Monitoring Tables**: For observability
   - Priority: MEDIUM
   - Impact: Limited debugging capability

2. **Add Check Constraints**: For data integrity
   - Priority: MEDIUM
   - Impact: Potential data quality issues

### Long Term (1+ month)
1. **Refactor Column Types**: For consistency
   - Priority: LOW
   - Impact: Technical debt

2. **Add Usage/Billing Tables**: For monetization
   - Priority: LOW
   - Impact: Can't track usage for billing

## Migration Strategy

### Option 1: Add to Initial Schema (Recommended for new deployments)
- Add all missing tables to `000001_initial_schema.up.sql`
- Ensures clean deployments have complete schema

### Option 2: Create New Migration (Recommended for existing deployments)
- Create `000002_dynamic_tools.up.sql` for tool tables
- Create `000003_performance_indexes.up.sql` for indexes
- Allows incremental updates without breaking existing deployments

## Summary

The current schema successfully supports:
- ✅ Core authentication and multi-tenancy
- ✅ AWS Bedrock embeddings with multi-agent support
- ✅ Task management and workflows
- ✅ Basic monitoring with audit logs

The schema lacks:
- ❌ Dynamic tools tables (CRITICAL)
- ❌ Performance optimization indexes
- ❌ Advanced monitoring tables
- ❌ Usage tracking for billing

**Recommendation**: Create a new migration file `000002_dynamic_tools.up.sql` to add the critical missing tables for the dynamic tools feature. This will unblock the REST API functionality while maintaining backward compatibility.