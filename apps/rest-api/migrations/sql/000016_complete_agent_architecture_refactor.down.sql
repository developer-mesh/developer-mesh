-- ================================================================
-- Rollback: Restore Original Agent Architecture
-- ================================================================

BEGIN;

-- Drop views and triggers
DROP VIEW IF EXISTS mcp.agents CASCADE;
DROP FUNCTION IF EXISTS mcp.agents_view_insert() CASCADE;
DROP FUNCTION IF EXISTS mcp.agents_view_update() CASCADE;
DROP FUNCTION IF EXISTS mcp.agents_view_delete() CASCADE;
DROP FUNCTION IF EXISTS mcp.register_agent_instance CASCADE;

-- Restore original agents table
ALTER TABLE IF EXISTS mcp.agents_deprecated RENAME TO agents;

-- Restore all foreign key constraints to point back to agents table
ALTER TABLE mcp.contexts 
    DROP CONSTRAINT IF EXISTS contexts_agent_id_fkey;
ALTER TABLE mcp.contexts 
    ADD CONSTRAINT contexts_agent_id_fkey 
    FOREIGN KEY (agent_id) REFERENCES mcp.agents(id) ON DELETE CASCADE;

ALTER TABLE mcp.agent_configs 
    DROP CONSTRAINT IF EXISTS agent_configs_agent_id_fkey;
ALTER TABLE mcp.agent_configs 
    ADD CONSTRAINT agent_configs_agent_id_fkey 
    FOREIGN KEY (agent_id) REFERENCES mcp.agents(id);

ALTER TABLE mcp.tasks 
    DROP CONSTRAINT IF EXISTS tasks_agent_id_fkey;
ALTER TABLE mcp.tasks 
    ADD CONSTRAINT tasks_agent_id_fkey 
    FOREIGN KEY (agent_id) REFERENCES mcp.agents(id);

ALTER TABLE mcp.task_delegations 
    DROP CONSTRAINT IF EXISTS task_delegations_from_agent_id_fkey;
ALTER TABLE mcp.task_delegations 
    ADD CONSTRAINT task_delegations_from_agent_id_fkey 
    FOREIGN KEY (from_agent_id) REFERENCES mcp.agents(id);

ALTER TABLE mcp.task_delegations 
    DROP CONSTRAINT IF EXISTS task_delegations_to_agent_id_fkey;
ALTER TABLE mcp.task_delegations 
    ADD CONSTRAINT task_delegations_to_agent_id_fkey 
    FOREIGN KEY (to_agent_id) REFERENCES mcp.agents(id);

ALTER TABLE mcp.task_delegation_history 
    DROP CONSTRAINT IF EXISTS task_delegation_history_from_agent_id_fkey;
ALTER TABLE mcp.task_delegation_history 
    ADD CONSTRAINT task_delegation_history_from_agent_id_fkey 
    FOREIGN KEY (from_agent_id) REFERENCES mcp.agents(id);

ALTER TABLE mcp.task_delegation_history 
    DROP CONSTRAINT IF EXISTS task_delegation_history_to_agent_id_fkey;
ALTER TABLE mcp.task_delegation_history 
    ADD CONSTRAINT task_delegation_history_to_agent_id_fkey 
    FOREIGN KEY (to_agent_id) REFERENCES mcp.agents(id);

ALTER TABLE mcp.task_status_history 
    DROP CONSTRAINT IF EXISTS task_status_history_transitioned_by_fkey;
ALTER TABLE mcp.task_status_history 
    ADD CONSTRAINT task_status_history_transitioned_by_fkey 
    FOREIGN KEY (transitioned_by) REFERENCES mcp.agents(id);

ALTER TABLE mcp.workspace_activities 
    DROP CONSTRAINT IF EXISTS workspace_activities_actor_id_fkey;
ALTER TABLE mcp.workspace_activities 
    ADD CONSTRAINT workspace_activities_actor_id_fkey 
    FOREIGN KEY (actor_id) REFERENCES mcp.agents(id);

ALTER TABLE mcp.agent_events 
    DROP CONSTRAINT IF EXISTS agent_events_agent_id_fkey;
ALTER TABLE mcp.agent_events 
    ADD CONSTRAINT agent_events_agent_id_fkey 
    FOREIGN KEY (agent_id) REFERENCES mcp.agents(id) ON DELETE CASCADE;

ALTER TABLE mcp.agent_sessions 
    DROP CONSTRAINT IF EXISTS agent_sessions_agent_id_fkey;
ALTER TABLE mcp.agent_sessions 
    ADD CONSTRAINT agent_sessions_agent_id_fkey 
    FOREIGN KEY (agent_id) REFERENCES mcp.agents(id) ON DELETE CASCADE;

ALTER TABLE mcp.agent_registry 
    DROP CONSTRAINT IF EXISTS agent_registry_agent_id_fkey;
ALTER TABLE mcp.agent_registry 
    ADD CONSTRAINT agent_registry_agent_id_fkey 
    FOREIGN KEY (agent_id) REFERENCES mcp.agents(id) ON DELETE CASCADE;

ALTER TABLE mcp.agent_embedding_preferences 
    DROP CONSTRAINT IF EXISTS agent_embedding_preferences_agent_id_fkey;
ALTER TABLE mcp.agent_embedding_preferences 
    ADD CONSTRAINT agent_embedding_preferences_agent_id_fkey 
    FOREIGN KEY (agent_id) REFERENCES mcp.agents(id) ON DELETE CASCADE;

ALTER TABLE mcp.embedding_usage_tracking 
    DROP CONSTRAINT IF EXISTS embedding_usage_tracking_agent_id_fkey;
ALTER TABLE mcp.embedding_usage_tracking 
    ADD CONSTRAINT embedding_usage_tracking_agent_id_fkey 
    FOREIGN KEY (agent_id) REFERENCES mcp.agents(id);

-- Drop new tables
DROP TABLE IF EXISTS mcp.agent_configurations CASCADE;

COMMIT;