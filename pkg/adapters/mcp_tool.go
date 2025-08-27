package adapters

// MCPTool represents a tool exposed via MCP protocol
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
