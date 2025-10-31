-- Drop cache system
DROP VIEW IF EXISTS mcp.cache_performance;

DROP FUNCTION IF EXISTS mcp.cleanup_expired_cache();
DROP FUNCTION IF EXISTS mcp.find_similar_cache_entries(UUID, vector(1536), FLOAT, INTEGER);
DROP FUNCTION IF EXISTS mcp.update_cache_stats(UUID, BOOLEAN, NUMERIC, BIGINT, INTEGER);
DROP FUNCTION IF EXISTS mcp.get_or_create_cache_entry(VARCHAR, UUID, VARCHAR, VARCHAR, VARCHAR, JSONB, vector(1536), JSONB, INTEGER);
DROP FUNCTION IF EXISTS mcp.update_expires_at() CASCADE;

DROP TABLE IF EXISTS mcp.cache_statistics;
DROP TABLE IF EXISTS mcp.cache_entries;