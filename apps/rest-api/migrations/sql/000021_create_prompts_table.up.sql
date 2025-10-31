-- Create prompts table for MCP protocol support
CREATE TABLE IF NOT EXISTS mcp.prompts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    arguments JSONB DEFAULT '[]',
    template TEXT NOT NULL,
    category VARCHAR(100),
    tags TEXT[] DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensure unique name per tenant
    CONSTRAINT unique_tenant_prompt_name UNIQUE(tenant_id, name)
);

-- Create indexes for efficient querying
CREATE INDEX idx_prompts_tenant_id ON mcp.prompts(tenant_id);
CREATE INDEX idx_prompts_name ON mcp.prompts(name);
CREATE INDEX idx_prompts_category ON mcp.prompts(category);
CREATE INDEX idx_prompts_tags ON mcp.prompts USING GIN(tags);
CREATE INDEX idx_prompts_metadata ON mcp.prompts USING GIN(metadata);
CREATE INDEX idx_prompts_created_at ON mcp.prompts(created_at DESC);

-- Create prompt usage tracking table
CREATE TABLE IF NOT EXISTS mcp.prompt_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    prompt_id UUID NOT NULL REFERENCES mcp.prompts(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL,
    arguments JSONB,
    rendered_content TEXT,
    used_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for usage tracking
CREATE INDEX idx_prompt_usage_tenant_id ON mcp.prompt_usage(tenant_id);
CREATE INDEX idx_prompt_usage_prompt_id ON mcp.prompt_usage(prompt_id);
CREATE INDEX idx_prompt_usage_agent_id ON mcp.prompt_usage(agent_id);
CREATE INDEX idx_prompt_usage_used_at ON mcp.prompt_usage(used_at DESC);

-- Create trigger to update the updated_at timestamp
CREATE OR REPLACE FUNCTION mcp.update_prompt_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_prompt_updated_at
    BEFORE UPDATE ON mcp.prompts
    FOR EACH ROW
    EXECUTE FUNCTION mcp.update_prompt_updated_at();

-- Add comments for documentation
COMMENT ON TABLE mcp.prompts IS 'Stores prompt templates for MCP protocol';
COMMENT ON TABLE mcp.prompt_usage IS 'Tracks usage of prompts by agents';
COMMENT ON COLUMN mcp.prompts.arguments IS 'JSON array of argument definitions [{name, description, required, default}]';
COMMENT ON COLUMN mcp.prompts.template IS 'Prompt template with {{variable}} placeholders';
COMMENT ON COLUMN mcp.prompts.category IS 'Category for organizing prompts (e.g., coding, analysis, creative)';
COMMENT ON COLUMN mcp.prompt_usage.rendered_content IS 'The actual prompt after template rendering';