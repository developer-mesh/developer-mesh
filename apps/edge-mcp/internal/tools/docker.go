package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/executor"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// DockerTool provides Docker operations
type DockerTool struct {
	executor *executor.CommandExecutor
	logger   observability.Logger
}

// NewDockerTool creates a new Docker tool
func NewDockerTool(exec *executor.CommandExecutor, logger observability.Logger) *DockerTool {
	return &DockerTool{
		executor: exec,
		logger:   logger,
	}
}

// DockerContainer represents a container in the list
type DockerContainer struct {
	ID      string `json:"id"`
	Image   string `json:"image"`
	Command string `json:"command"`
	Created string `json:"created"`
	Status  string `json:"status"`
	Ports   string `json:"ports"`
	Names   string `json:"names"`
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
	// Parse arguments
	var params struct {
		Context    string            `json:"context"`
		Tag        string            `json:"tag,omitempty"`
		Dockerfile string            `json:"dockerfile,omitempty"`
		BuildArgs  map[string]string `json:"buildArgs,omitempty"`
		NoCache    bool              `json:"noCache,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Validate build context path
	if params.Context == "" {
		return nil, fmt.Errorf("build context is required")
	}

	// Clean and validate the path
	cleanPath := filepath.Clean(params.Context)
	if !t.executor.IsPathSafe(cleanPath) {
		return nil, fmt.Errorf("invalid build context path: %s", params.Context)
	}

	// Build docker command
	dockerArgs := []string{"build"}

	// Add tag if provided
	if params.Tag != "" {
		dockerArgs = append(dockerArgs, "-t", params.Tag)
	}

	// Add dockerfile if specified
	if params.Dockerfile != "" {
		dockerArgs = append(dockerArgs, "-f", params.Dockerfile)
	}

	// Add no-cache flag
	if params.NoCache {
		dockerArgs = append(dockerArgs, "--no-cache")
	}

	// Add build arguments
	for key, value := range params.BuildArgs {
		dockerArgs = append(dockerArgs, "--build-arg", fmt.Sprintf("%s=%s", key, value))
	}

	// Add the build context
	dockerArgs = append(dockerArgs, cleanPath)

	// Execute docker build
	result, err := t.executor.Execute(ctx, "docker", dockerArgs)
	if err != nil {
		return nil, fmt.Errorf("docker build failed: %w", err)
	}

	t.logger.Info("Docker build executed", map[string]interface{}{
		"context": params.Context,
		"tag":     params.Tag,
		"success": err == nil,
	})

	// Parse build output for image ID
	imageID := t.extractImageID(result.Stdout)

	return map[string]interface{}{
		"imageId": imageID,
		"tag":     params.Tag,
		"output":  result.Stdout,
	}, nil
}

func (t *DockerTool) handlePs(ctx context.Context, args json.RawMessage) (interface{}, error) {
	// Parse arguments
	var params struct {
		All    bool   `json:"all,omitempty"`
		Format string `json:"format,omitempty"`
	}
	if args != nil {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	// Build docker ps command with JSON output for easy parsing
	dockerArgs := []string{"ps", "--format", "json"}
	if params.All {
		dockerArgs = append(dockerArgs, "-a")
	}

	// Execute docker ps
	result, err := t.executor.Execute(ctx, "docker", dockerArgs)
	if err != nil {
		return nil, fmt.Errorf("docker ps failed: %w", err)
	}

	// Parse JSON output
	containers := t.parseDockerPs(result.Stdout)

	t.logger.Info("Docker ps executed", map[string]interface{}{
		"all":        params.All,
		"containers": len(containers),
	})

	return map[string]interface{}{
		"containers": containers,
	}, nil
}

// extractImageID extracts the image ID from docker build output
func (t *DockerTool) extractImageID(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Look for "Successfully built" or "writing image sha256:"
		if strings.Contains(line, "Successfully built ") {
			parts := strings.Split(line, "Successfully built ")
			if len(parts) > 1 {
				return strings.TrimSpace(parts[1])
			}
		}
		if strings.Contains(line, "writing image sha256:") {
			parts := strings.Split(line, "writing image sha256:")
			if len(parts) > 1 {
				// Extract just the short ID
				id := strings.TrimSpace(parts[1])
				if len(id) > 12 {
					return id[:12]
				}
				return id
			}
		}
	}
	return ""
}

// parseDockerPs parses docker ps JSON output
func (t *DockerTool) parseDockerPs(output string) []DockerContainer {
	containers := []DockerContainer{}

	// Each line is a JSON object
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		var container map[string]interface{}
		if err := json.Unmarshal([]byte(line), &container); err != nil {
			// If not JSON format, skip
			continue
		}

		// Extract fields from JSON
		dc := DockerContainer{}
		if id, ok := container["ID"].(string); ok {
			dc.ID = id
		}
		if image, ok := container["Image"].(string); ok {
			dc.Image = image
		}
		if command, ok := container["Command"].(string); ok {
			dc.Command = command
		}
		if created, ok := container["CreatedAt"].(string); ok {
			dc.Created = created
		}
		if status, ok := container["Status"].(string); ok {
			dc.Status = status
		}
		if ports, ok := container["Ports"].(string); ok {
			dc.Ports = ports
		}
		if names, ok := container["Names"].(string); ok {
			dc.Names = names
		}

		containers = append(containers, dc)
	}

	return containers
}
