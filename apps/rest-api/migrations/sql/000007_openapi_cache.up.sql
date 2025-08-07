-- ==============================================================================
-- Migration: 000007_openapi_cache
-- Description: Add OpenAPI specification caching table
-- Author: System
-- Date: 2025-08-06
-- ==============================================================================

BEGIN;

-- ==============================================================================
-- OPENAPI CACHE TABLE
-- ==============================================================================

-- Create OpenAPI cache table for storing fetched specifications
-- This table is used by pkg/repository/openapi_cache_repository.go
CREATE TABLE IF NOT EXISTS mcp.openapi_cache (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    url TEXT NOT NULL,
    spec_hash VARCHAR(64),
    spec_data JSONB NOT NULL,
    version VARCHAR(50),
    title VARCHAR(255),
    discovered_actions TEXT[],  -- Array of discovered action names
    cache_expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Unique constraint for URL and hash combination
    CONSTRAINT unique_url_cache UNIQUE (url, spec_hash)
);

-- Create indexes for efficient lookups
CREATE INDEX IF NOT EXISTS idx_openapi_cache_url ON mcp.openapi_cache(url);
CREATE INDEX IF NOT EXISTS idx_openapi_cache_expires ON mcp.openapi_cache(cache_expires_at);
CREATE INDEX IF NOT EXISTS idx_openapi_cache_created ON mcp.openapi_cache(created_at DESC);

-- Add trigger for updated_at
CREATE TRIGGER update_openapi_cache_updated_at 
    BEFORE UPDATE ON mcp.openapi_cache 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Add comment explaining the table
COMMENT ON TABLE mcp.openapi_cache IS 'Cache for OpenAPI specifications fetched from external APIs';
COMMENT ON COLUMN mcp.openapi_cache.url IS 'URL of the OpenAPI specification';
COMMENT ON COLUMN mcp.openapi_cache.spec_hash IS 'SHA256 hash of the spec for change detection';
COMMENT ON COLUMN mcp.openapi_cache.spec_data IS 'Full OpenAPI specification as JSON';
COMMENT ON COLUMN mcp.openapi_cache.version IS 'API version from the spec';
COMMENT ON COLUMN mcp.openapi_cache.title IS 'API title from the spec';
COMMENT ON COLUMN mcp.openapi_cache.discovered_actions IS 'Array of discovered action IDs/operations';
COMMENT ON COLUMN mcp.openapi_cache.cache_expires_at IS 'When this cached entry expires';

COMMIT;