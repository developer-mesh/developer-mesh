-- Remove passthrough configuration from tool templates

-- Remove from all templates that have it
UPDATE mcp.tool_templates 
SET default_config = default_config - 'passthrough'
WHERE default_config ? 'passthrough';