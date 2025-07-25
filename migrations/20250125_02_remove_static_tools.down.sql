-- Rollback: Remove Static Tool Implementations
-- Version: 20250125_02
-- Description: Rollback of static tool removal (re-creates placeholder tables)

-- Note: This is a destructive rollback that cannot restore original data
-- Only creates empty tables/views as placeholders

-- Create placeholder static tool tables
CREATE TABLE IF NOT EXISTS github_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS gitlab_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sonarqube_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS jfrog_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Drop the dynamic tools view
DROP VIEW IF EXISTS dynamic_tools;

-- Update migration metadata
UPDATE migration_metadata 
SET rollback_at = NOW(), 
    status = 'rolled_back' 
WHERE version = '20250125_02';