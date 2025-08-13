package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// FileSystemTool provides file system operations with security
type FileSystemTool struct {
	basePath     string   // Base path for operations
	allowedPaths []string // List of allowed paths
	logger       observability.Logger
}

// NewFileSystemTool creates a new file system tool with security constraints
func NewFileSystemTool(basePath string, logger observability.Logger) *FileSystemTool {
	// Default to current working directory if not specified
	if basePath == "" {
		basePath, _ = os.Getwd()
	}

	return &FileSystemTool{
		basePath:     basePath,
		allowedPaths: []string{basePath},
		logger:       logger,
	}
}

// isPathSafe validates that a path is within allowed directories
func (t *FileSystemTool) isPathSafe(path string) bool {
	// Clean the path to prevent traversal attacks
	cleaned := filepath.Clean(path)

	// Reject paths with .. to prevent traversal
	if strings.Contains(cleaned, "..") {
		return false
	}

	// Make path absolute
	absPath, err := filepath.Abs(cleaned)
	if err != nil {
		return false
	}

	// Check if path is within allowed directories
	for _, allowed := range t.allowedPaths {
		absAllowed, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}

		// Check if path is within allowed directory
		if strings.HasPrefix(absPath, absAllowed) {
			return true
		}
	}

	return false
}

// GetDefinitions returns tool definitions
func (t *FileSystemTool) GetDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "fs.read_file",
			Description: "Read the contents of a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to read",
					},
				},
				"required": []string{"path"},
			},
			Handler: t.handleReadFile,
		},
		{
			Name:        "fs.write_file",
			Description: "Write content to a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to write",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Content to write to the file",
					},
				},
				"required": []string{"path", "content"},
			},
			Handler: t.handleWriteFile,
		},
		{
			Name:        "fs.list_directory",
			Description: "List contents of a directory",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the directory",
					},
				},
				"required": []string{"path"},
			},
			Handler: t.handleListDirectory,
		},
	}
}

func (t *FileSystemTool) handleReadFile(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Validate path security
	if !t.isPathSafe(params.Path) {
		t.logger.Warn("Blocked unsafe path access attempt", map[string]interface{}{
			"path": params.Path,
		})
		return nil, fmt.Errorf("access denied: path outside allowed directory")
	}

	content, err := os.ReadFile(params.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	t.logger.Info("File read successfully", map[string]interface{}{
		"path": params.Path,
		"size": len(content),
	})

	return map[string]interface{}{
		"content": string(content),
		"size":    len(content),
	}, nil
}

func (t *FileSystemTool) handleWriteFile(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Validate path security
	if !t.isPathSafe(params.Path) {
		t.logger.Warn("Blocked unsafe write attempt", map[string]interface{}{
			"path": params.Path,
		})
		return nil, fmt.Errorf("access denied: path outside allowed directory")
	}

	// Don't allow writing to sensitive files
	sensitiveFiles := []string{".ssh", ".git/config", ".env", "id_rsa", "id_ed25519"}
	for _, sensitive := range sensitiveFiles {
		if strings.Contains(params.Path, sensitive) {
			t.logger.Warn("Blocked write to sensitive file", map[string]interface{}{
				"path": params.Path,
			})
			return nil, fmt.Errorf("access denied: cannot write to sensitive files")
		}
	}

	if err := os.WriteFile(params.Path, []byte(params.Content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	t.logger.Info("File written successfully", map[string]interface{}{
		"path": params.Path,
		"size": len(params.Content),
	})

	return map[string]interface{}{
		"success": true,
		"size":    len(params.Content),
	}, nil
}

func (t *FileSystemTool) handleListDirectory(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Validate path security
	if !t.isPathSafe(params.Path) {
		t.logger.Warn("Blocked unsafe directory list attempt", map[string]interface{}{
			"path": params.Path,
		})
		return nil, fmt.Errorf("access denied: path outside allowed directory")
	}

	entries, err := os.ReadDir(params.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	files := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		info, _ := entry.Info()
		files = append(files, map[string]interface{}{
			"name": entry.Name(),
			"type": map[bool]string{true: "directory", false: "file"}[entry.IsDir()],
			"size": info.Size(),
			"mode": info.Mode().String(),
		})
	}

	t.logger.Info("Directory listed", map[string]interface{}{
		"path":  params.Path,
		"count": len(files),
	})

	return map[string]interface{}{
		"files": files,
		"count": len(files),
	}, nil
}
