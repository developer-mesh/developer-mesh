package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// DockerTool provides Docker operations
type DockerTool struct{}

// NewDockerTool creates a new Docker tool
func NewDockerTool() *DockerTool {
	return &DockerTool{}
}

// GetDefinitions returns tool definitions
func (t *DockerTool) GetDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "docker.build",
			Description: "Build a Docker image",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"context": map[string]interface{}{
						"type":        "string",
						"description": "Build context path",
					},
					"tag": map[string]interface{}{
						"type":        "string",
						"description": "Image tag",
					},
				},
				"required": []string{"context"},
			},
			Handler: t.handleBuild,
		},
		{
			Name:        "docker.ps",
			Description: "List Docker containers",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"all": map[string]interface{}{
						"type":        "boolean",
						"description": "Show all containers (default shows just running)",
					},
				},
			},
			Handler: t.handlePs,
		},
	}
}

func (t *DockerTool) handleBuild(ctx context.Context, args json.RawMessage) (interface{}, error) {
	// TODO: Implement docker build using exec.Command
	return map[string]interface{}{
		"status": "TODO: Implement docker build",
	}, fmt.Errorf("docker.build not yet implemented")
}

func (t *DockerTool) handlePs(ctx context.Context, args json.RawMessage) (interface{}, error) {
	// TODO: Implement docker ps using exec.Command
	return map[string]interface{}{
		"containers": "TODO: Implement docker ps",
	}, fmt.Errorf("docker.ps not yet implemented")
}
