-- ==============================================================================
-- Migration: 000007_openapi_cache (DOWN)
-- Description: Remove OpenAPI specification caching table
-- Author: System
-- Date: 2025-08-06
-- ==============================================================================

BEGIN;

-- Drop the table (this will also drop all associated indexes, triggers, and constraints)
DROP TABLE IF EXISTS mcp.openapi_cache CASCADE;

COMMIT;