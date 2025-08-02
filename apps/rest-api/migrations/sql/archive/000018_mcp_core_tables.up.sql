-- Create core MCP tables required by the application

BEGIN;

-- Set search path to mcp schema
SET search_path TO mcp, public;

-- Events table
CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source VARCHAR(255) NOT NULL,
    type VARCHAR(255) NOT NULL,
    data JSONB NOT NULL DEFAULT '{}',
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for events
CREATE INDEX IF NOT EXISTS idx_events_source ON events(source);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp DESC);

-- Context items table
CREATE TABLE IF NOT EXISTS context_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    context_id UUID NOT NULL,
    role VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    tokens INTEGER DEFAULT 0,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    metadata JSONB NOT NULL DEFAULT '{}',
    
    -- Foreign key to contexts table
    CONSTRAINT context_items_context_fk FOREIGN KEY (context_id) REFERENCES contexts(id) ON DELETE CASCADE
);

-- Indexes for context_items
CREATE INDEX IF NOT EXISTS idx_context_items_context_id ON context_items(context_id);
CREATE INDEX IF NOT EXISTS idx_context_items_timestamp ON context_items(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_context_items_role ON context_items(role);

-- Integrations table
CREATE TABLE IF NOT EXISTS integrations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    type VARCHAR(100) NOT NULL,
    config JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(50) DEFAULT 'active',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for integrations
CREATE INDEX IF NOT EXISTS idx_integrations_type ON integrations(type);
CREATE INDEX IF NOT EXISTS idx_integrations_status ON integrations(status);

-- Create update triggers
DROP TRIGGER IF EXISTS update_integrations_updated_at ON integrations;
CREATE TRIGGER update_integrations_updated_at BEFORE UPDATE
ON integrations FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;