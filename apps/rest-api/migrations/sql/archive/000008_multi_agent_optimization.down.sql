-- Rollback: Multi-Agent Performance Optimization

-- Remove statistics
DROP STATISTICS IF EXISTS tasks_status_priority;
DROP STATISTICS IF EXISTS workflows_tenant_type;

-- Note: We don't rollback autovacuum settings or test data as they are harmless to leave