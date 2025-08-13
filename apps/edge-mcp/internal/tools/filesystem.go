package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// FileSystemTool provides file system operations
type FileSystemTool struct {
	basePath string // Optional: restrict to a base path
}

// NewFileSystemTool creates a new file system tool
func NewFileSystemTool() *FileSystemTool {
	return &FileSystemTool{}
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

	content, err := os.ReadFile(params.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

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

	if err := os.WriteFile(params.Path, []byte(params.Content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

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

	return map[string]interface{}{
		"files": files,
		"count": len(files),
	}, nil
}
