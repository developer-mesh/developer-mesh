-- Add provider and passthrough configuration to tool configurations
ALTER TABLE tool_configurations ADD COLUMN IF NOT EXISTS provider VARCHAR(50);
ALTER TABLE tool_configurations ADD COLUMN IF NOT EXISTS passthrough_config JSONB DEFAULT '{"mode": "optional", "fallback_to_service": true}';

-- Update existing tools with provider based on their config
UPDATE tool_configurations 
SET provider = 'github' 
WHERE config->>'base_url' LIKE '%github.com%' AND provider IS NULL;

UPDATE tool_configurations 
SET provider = 'gitlab' 
WHERE config->>'base_url' LIKE '%gitlab.com%' AND provider IS NULL;

-- Add index for provider lookups
CREATE INDEX IF NOT EXISTS idx_tool_configurations_provider ON tool_configurations(provider);

-- Add comment explaining the passthrough_config structure
COMMENT ON COLUMN tool_configurations.passthrough_config IS 'Configuration for user token passthrough. Structure: {"mode": "optional|required|disabled", "fallback_to_service": boolean}';