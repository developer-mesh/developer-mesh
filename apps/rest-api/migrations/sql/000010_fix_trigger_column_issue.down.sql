-- Rollback trigger fix
-- This would restore the previous (broken) trigger, but we won't do that
-- Instead, just drop the trigger and function

DROP TRIGGER IF EXISTS create_agent_config_on_insert ON mcp.agents;
DROP FUNCTION IF EXISTS mcp.create_default_agent_config();
