-- Initial schema setup with best practices
BEGIN;

-- Create schema if not exists
CREATE SCHEMA IF NOT EXISTS mcp;

-- Set search path for this transaction
SET search_path TO mcp, public;

-- UUID extension (standard for modern apps)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Models table (needed before agents)
CREATE TABLE IF NOT EXISTS models (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    provider VARCHAR(255),
    model_type VARCHAR(100),
    
    -- Metadata
    metadata JSONB NOT NULL DEFAULT '{}',
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_by UUID,
    
    -- Soft delete support
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    -- Constraints
    CONSTRAINT models_name_not_empty CHECK (length(trim(name)) > 0)
);

-- Indexes for models
CREATE INDEX IF NOT EXISTS idx_models_tenant_id ON models(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_models_provider ON models(provider) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_models_model_type ON models(model_type) WHERE deleted_at IS NULL;

-- Agents table
CREATE TABLE IF NOT EXISTS agents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    model_id UUID NOT NULL,
    
    -- Agent configuration
    config JSONB NOT NULL DEFAULT '{}',
    capabilities TEXT[] DEFAULT '{}',
    
    -- Metadata
    metadata JSONB NOT NULL DEFAULT '{}',
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_by UUID,
    
    -- Soft delete support
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    -- Constraints
    CONSTRAINT agents_name_not_empty CHECK (length(trim(name)) > 0),
    CONSTRAINT agents_model_fk FOREIGN KEY (model_id) REFERENCES models(id) ON DELETE RESTRICT
);

-- Indexes for agents
CREATE INDEX IF NOT EXISTS idx_agents_tenant_id ON agents(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_agents_model_id ON agents(model_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_agents_capabilities ON agents USING gin(capabilities) WHERE deleted_at IS NULL;

-- Base contexts table
CREATE TABLE IF NOT EXISTS contexts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    
    -- Metadata as JSONB for flexibility
    metadata JSONB NOT NULL DEFAULT '{}',
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_by UUID,
    
    -- Soft delete support
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    -- Constraints
    CONSTRAINT contexts_name_not_empty CHECK (length(trim(name)) > 0)
);

-- Indexes for contexts
CREATE INDEX IF NOT EXISTS idx_contexts_tenant_id ON contexts(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_contexts_created_at ON contexts(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_contexts_metadata ON contexts USING gin(metadata);

-- Update trigger for updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Drop trigger if exists and recreate
DROP TRIGGER IF EXISTS update_contexts_updated_at ON contexts;
CREATE TRIGGER update_contexts_updated_at BEFORE UPDATE
ON contexts FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Triggers for models table
DROP TRIGGER IF EXISTS update_models_updated_at ON models;
CREATE TRIGGER update_models_updated_at BEFORE UPDATE
ON models FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Triggers for agents table
DROP TRIGGER IF EXISTS update_agents_updated_at ON agents;
CREATE TRIGGER update_agents_updated_at BEFORE UPDATE
ON agents FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Add table comments for documentation
COMMENT ON TABLE contexts IS 'Stores context information for the MCP system';
COMMENT ON COLUMN contexts.metadata IS 'Flexible JSONB storage for context-specific data';
COMMENT ON COLUMN contexts.deleted_at IS 'Soft delete timestamp - NULL means active';

COMMIT;