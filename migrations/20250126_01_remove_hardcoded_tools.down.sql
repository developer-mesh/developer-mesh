-- Rollback: Restore hardcoded tool configurations
-- Note: This is a destructive rollback - data cannot be fully restored

-- Re-add tool_type column if it was removed
DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'tool_configurations' 
        AND column_name = 'tool_type'
    ) THEN
        ALTER TABLE tool_configurations 
        ADD COLUMN tool_type VARCHAR(50);
        
        -- Try to restore tool_type from config if it was saved
        UPDATE tool_configurations 
        SET tool_type = config->>'legacy_tool_type'
        WHERE config ? 'legacy_tool_type';
        
        -- Remove the legacy marker
        UPDATE tool_configurations
        SET config = config - 'legacy_tool_type'
        WHERE config ? 'legacy_tool_type';
    END IF;
END $$;

-- Note: We cannot restore dropped tables and their data
-- This would need to be restored from backups

-- Remove migration record
DELETE FROM schema_migrations WHERE version = '20250126_01';