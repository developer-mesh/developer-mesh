-- Drop prompt-related tables and functions
DROP TRIGGER IF EXISTS trigger_update_prompt_updated_at ON mcp.prompts;
DROP FUNCTION IF EXISTS mcp.update_prompt_updated_at();
DROP TABLE IF EXISTS mcp.prompt_usage;
DROP TABLE IF EXISTS mcp.prompts;