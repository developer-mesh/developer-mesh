-- Remove added columns

BEGIN;

SET search_path TO mcp, public;

ALTER TABLE tasks DROP COLUMN IF EXISTS version;
ALTER TABLE workflows DROP COLUMN IF EXISTS version;
ALTER TABLE workspaces DROP COLUMN IF EXISTS version;

COMMIT;