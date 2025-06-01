-- Initial schema setup with best practices
BEGIN;

-- Create schema if not exists
CREATE SCHEMA IF NOT EXISTS mcp;

-- Set search path for this transaction
SET search_path TO mcp, public;

-- UUID extension (standard for modern apps)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Base contexts table
CREATE TABLE contexts (
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
CREATE INDEX idx_contexts_tenant_id ON contexts(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_contexts_created_at ON contexts(created_at DESC);
CREATE INDEX idx_contexts_metadata ON contexts USING gin(metadata);

-- Update trigger for updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_contexts_updated_at BEFORE UPDATE
ON contexts FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Add table comments for documentation
COMMENT ON TABLE contexts IS 'Stores context information for the MCP system';
COMMENT ON COLUMN contexts.metadata IS 'Flexible JSONB storage for context-specific data';
COMMENT ON COLUMN contexts.deleted_at IS 'Soft delete timestamp - NULL means active';

COMMIT;