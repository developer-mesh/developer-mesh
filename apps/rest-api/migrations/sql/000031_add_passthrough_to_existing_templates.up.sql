-- Add passthrough authentication configuration to existing tool templates
-- This enables Edge MCP to pass user credentials directly to the tools

-- Update GitHub templates
UPDATE mcp.tool_templates 
SET default_config = jsonb_set(
    default_config,
    '{passthrough}',
    '{"mode": "optional", "supportedProviders": ["github"], "fallbackToService": true}'::jsonb
)
WHERE provider_name = 'github';

-- Update GitLab templates
UPDATE mcp.tool_templates 
SET default_config = jsonb_set(
    default_config,
    '{passthrough}',
    '{"mode": "optional", "supportedProviders": ["gitlab"], "fallbackToService": true}'::jsonb
)
WHERE provider_name = 'gitlab';

-- Update Jira templates
UPDATE mcp.tool_templates 
SET default_config = jsonb_set(
    default_config,
    '{passthrough}',
    '{"mode": "optional", "supportedProviders": ["jira", "atlassian"], "fallbackToService": true}'::jsonb
)
WHERE provider_name = 'jira';

-- Update Harness templates
UPDATE mcp.tool_templates 
SET default_config = jsonb_set(
    default_config,
    '{passthrough}',
    '{"mode": "optional", "supportedProviders": ["harness"], "fallbackToService": true}'::jsonb
)
WHERE provider_name = 'harness';

-- Update Confluence templates
UPDATE mcp.tool_templates 
SET default_config = jsonb_set(
    default_config,
    '{passthrough}',
    '{"mode": "optional", "supportedProviders": ["confluence", "atlassian"], "fallbackToService": true}'::jsonb
)
WHERE provider_name = 'confluence';

-- Update Artifactory templates
UPDATE mcp.tool_templates 
SET default_config = jsonb_set(
    default_config,
    '{passthrough}',
    '{"mode": "optional", "supportedProviders": ["artifactory", "jfrog"], "fallbackToService": true}'::jsonb
)
WHERE provider_name = 'artifactory';

-- Update Nexus templates
UPDATE mcp.tool_templates 
SET default_config = jsonb_set(
    default_config,
    '{passthrough}',
    '{"mode": "optional", "supportedProviders": ["nexus", "sonatype"], "fallbackToService": true}'::jsonb
)
WHERE provider_name = 'nexus';