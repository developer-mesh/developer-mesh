-- Drop webhook_configs table
DROP TRIGGER IF EXISTS update_webhook_configs_updated_at ON webhook_configs;
DROP INDEX IF EXISTS idx_webhook_configs_enabled;
DROP INDEX IF EXISTS idx_webhook_configs_org_name;
DROP TABLE IF EXISTS webhook_configs;