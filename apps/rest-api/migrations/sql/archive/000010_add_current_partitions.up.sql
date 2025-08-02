-- Add partitions for current time period

BEGIN;

-- Set search path to mcp schema
SET search_path TO mcp, public;

-- Tasks partitions for June 2025
CREATE TABLE IF NOT EXISTS tasks_2025_06 PARTITION OF tasks FOR VALUES FROM ('2025-06-01') TO ('2025-07-01');
CREATE TABLE IF NOT EXISTS tasks_2025_07 PARTITION OF tasks FOR VALUES FROM ('2025-07-01') TO ('2025-08-01');

-- Apply performance settings to new partitions
ALTER TABLE tasks_2025_06 SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02,
    autovacuum_vacuum_cost_delay = 10,
    autovacuum_vacuum_cost_limit = 1000
);

ALTER TABLE tasks_2025_07 SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02,
    autovacuum_vacuum_cost_delay = 10,
    autovacuum_vacuum_cost_limit = 1000
);

-- Audit log partitions for June 2025
CREATE TABLE IF NOT EXISTS audit_log_2025_06 PARTITION OF audit_log FOR VALUES FROM ('2025-06-01') TO ('2025-07-01');
CREATE TABLE IF NOT EXISTS audit_log_2025_07 PARTITION OF audit_log FOR VALUES FROM ('2025-07-01') TO ('2025-08-01');

COMMIT;