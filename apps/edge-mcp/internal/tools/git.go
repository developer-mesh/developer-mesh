package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/executor"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// GitTool provides Git operations
type GitTool struct {
	executor *executor.CommandExecutor
	logger   observability.Logger
}

// NewGitTool creates a new Git tool
func NewGitTool(exec *executor.CommandExecutor, logger observability.Logger) *GitTool {
	return &GitTool{
		executor: exec,
		logger:   logger,
	}
}

// GitStatus represents the parsed git status output
type GitStatus struct {
	Branch     string   `json:"branch"`
	Modified   []string `json:"modified"`
	Untracked  []string `json:"untracked"`
	Staged     []string `json:"staged"`
	Deleted    []string `json:"deleted"`
	HasChanges bool     `json:"has_changes"`
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
	// Parse arguments
	var params struct {
		Path string `json:"path,omitempty"`
	}
	if args != nil {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	// Validate path if provided
	if params.Path != "" && !t.executor.IsPathSafe(params.Path) {
		return nil, fmt.Errorf("invalid path: %s", params.Path)
	}

	// Execute git status with porcelain format for parsing
	result, err := t.executor.Execute(ctx, "git", []string{"status", "--porcelain=v2", "--branch"})
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w", err)
	}

	// Parse the output
	status := t.parseGitStatus(result.Stdout)

	t.logger.Info("Git status executed", map[string]interface{}{
		"branch":      status.Branch,
		"modified":    len(status.Modified),
		"untracked":   len(status.Untracked),
		"staged":      len(status.Staged),
		"has_changes": status.HasChanges,
	})

	return status, nil
}

func (t *GitTool) handleDiff(ctx context.Context, args json.RawMessage) (interface{}, error) {
	// Parse arguments
	var params struct {
		Path   string `json:"path,omitempty"`
		Staged bool   `json:"staged,omitempty"`
	}
	if args != nil {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	// Build git diff command
	gitArgs := []string{"diff", "--unified=3"}
	if params.Staged {
		gitArgs = append(gitArgs, "--staged")
	}
	if params.Path != "" {
		if !t.executor.IsPathSafe(params.Path) {
			return nil, fmt.Errorf("invalid path: %s", params.Path)
		}
		gitArgs = append(gitArgs, "--", params.Path)
	}

	// Execute git diff
	result, err := t.executor.Execute(ctx, "git", gitArgs)
	if err != nil {
		// No changes is not an error for diff
		if result != nil && result.ExitCode == 0 {
			return map[string]string{"diff": ""}, nil
		}
		return nil, fmt.Errorf("git diff failed: %w", err)
	}

	return map[string]string{"diff": result.Stdout}, nil
}

// parseGitStatus parses git status --porcelain=v2 output
func (t *GitTool) parseGitStatus(output string) *GitStatus {
	status := &GitStatus{
		Modified:  []string{},
		Untracked: []string{},
		Staged:    []string{},
		Deleted:   []string{},
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		// Parse branch information
		if strings.HasPrefix(line, "# branch.head ") {
			status.Branch = strings.TrimPrefix(line, "# branch.head ")
			continue
		}

		// Parse file status
		if strings.HasPrefix(line, "1 ") || strings.HasPrefix(line, "2 ") {
			// Format: 1 XY sub mH mI mW hH hI path
			parts := strings.Fields(line)
			if len(parts) >= 9 {
				xy := parts[1]
				path := parts[8]

				// X = index, Y = worktree
				switch xy[0] {
				case 'M', 'A', 'D', 'R', 'C':
					status.Staged = append(status.Staged, path)
				}

				switch xy[1] {
				case 'M':
					status.Modified = append(status.Modified, path)
				case 'D':
					status.Deleted = append(status.Deleted, path)
				}
			}
		} else if strings.HasPrefix(line, "? ") {
			// Untracked file
			path := strings.TrimPrefix(line, "? ")
			status.Untracked = append(status.Untracked, path)
		}
	}

	status.HasChanges = len(status.Modified) > 0 || len(status.Untracked) > 0 ||
		len(status.Staged) > 0 || len(status.Deleted) > 0

	return status
}
