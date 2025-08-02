-- Drop core MCP tables

BEGIN;

SET search_path TO mcp, public;

DROP TABLE IF EXISTS context_items CASCADE;
DROP TABLE IF EXISTS events CASCADE;
DROP TABLE IF EXISTS integrations CASCADE;

COMMIT;