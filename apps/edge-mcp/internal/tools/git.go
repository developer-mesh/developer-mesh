package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// GitTool provides Git operations
type GitTool struct{}

// NewGitTool creates a new Git tool
func NewGitTool() *GitTool {
	return &GitTool{}
}

// GetDefinitions returns tool definitions
func (t *GitTool) GetDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "git.status",
			Description: "Get Git repository status",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Repository path (optional, defaults to current directory)",
					},
				},
			},
			Handler: t.handleStatus,
		},
		{
			Name:        "git.diff",
			Description: "Show Git diff",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Repository path",
					},
				},
			},
			Handler: t.handleDiff,
		},
	}
}

func (t *GitTool) handleStatus(ctx context.Context, args json.RawMessage) (interface{}, error) {
	// TODO: Implement git status using exec.Command
	return map[string]interface{}{
		"status": "TODO: Implement git status",
	}, fmt.Errorf("git.status not yet implemented")
}

func (t *GitTool) handleDiff(ctx context.Context, args json.RawMessage) (interface{}, error) {
	// TODO: Implement git diff using exec.Command
	return map[string]interface{}{
		"diff": "TODO: Implement git diff",
	}, fmt.Errorf("git.diff not yet implemented")
}
