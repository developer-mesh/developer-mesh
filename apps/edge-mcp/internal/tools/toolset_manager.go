package tools

import (
	"context"
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/mcptools"
)

// ToolsetManager manages toolset enablement and tool generation
// This bridges between pkg/mcptools types and the internal Registry
type ToolsetManager struct {
	toolsets map[string]mcptools.Toolset // All registered toolsets
	registry *Registry                    // Reference to the main tool registry

	// Default toolsets that are enabled by default
	defaultToolsets []string
}

// NewToolsetManager creates a new toolset manager
func NewToolsetManager(registry *Registry) *ToolsetManager {
	return &ToolsetManager{
		toolsets: make(map[string]mcptools.Toolset),
		registry: registry,
		defaultToolsets: []string{
			"devmesh_context",
			"devmesh_workflow",
			"devmesh_task",
		},
	}
}

// RegisterToolset registers a new toolset
func (tm *ToolsetManager) RegisterToolset(toolset mcptools.Toolset) error {
	if toolset.Name == "" {
		return fmt.Errorf("toolset name cannot be empty")
	}

	if toolset.ToolGenerator == nil {
		return fmt.Errorf("toolset %s must have a tool generator", toolset.Name)
	}

	// Check if this is a default toolset
	for _, defaultName := range tm.defaultToolsets {
		if toolset.Name == defaultName {
			toolset.Enabled = true
			break
		}
	}

	tm.toolsets[toolset.Name] = toolset
	return nil
}

// RegisterProvider registers all toolsets from a provider
func (tm *ToolsetManager) RegisterProvider(provider mcptools.ToolsetProvider) error {
	toolsets := provider.GetToolsets()
	for _, toolset := range toolsets {
		if err := tm.RegisterToolset(toolset); err != nil {
			return fmt.Errorf("failed to register toolset %s: %w", toolset.Name, err)
		}
	}
	return nil
}

// EnableToolset enables a toolset and registers its tools
func (tm *ToolsetManager) EnableToolset(ctx context.Context, name string) error {
	toolset, exists := tm.toolsets[name]
	if !exists {
		return fmt.Errorf("toolset not found: %s", name)
	}

	if toolset.Enabled {
		return nil // Already enabled
	}

	// Generate full tool definitions
	mcpToolDefs, err := toolset.ToolGenerator(mcptools.DetailLevelFull, nil)
	if err != nil {
		return fmt.Errorf("failed to generate tools for toolset %s: %w", name, err)
	}

	// Convert to internal ToolDefinition and register each tool
	for _, mcpTool := range mcpToolDefs {
		internalTool := convertMCPToInternalToolDef(mcpTool)
		tm.registry.RegisterRemote(internalTool)
	}

	// Mark as enabled
	toolset.Enabled = true
	tm.toolsets[name] = toolset

	return nil
}

// DisableToolset disables a toolset and removes its tools
func (tm *ToolsetManager) DisableToolset(ctx context.Context, name string) error {
	toolset, exists := tm.toolsets[name]
	if !exists {
		return fmt.Errorf("toolset not found: %s", name)
	}

	if !toolset.Enabled {
		return nil // Already disabled
	}

	// Note: We don't currently have a way to unregister tools from the registry
	// This would need to be added to the Registry struct
	// For now, we just mark as disabled
	toolset.Enabled = false
	tm.toolsets[name] = toolset

	return nil
}

// EnableToolsets enables multiple toolsets
func (tm *ToolsetManager) EnableToolsets(ctx context.Context, names []string) error {
	for _, name := range names {
		if err := tm.EnableToolset(ctx, name); err != nil {
			return err
		}
	}
	return nil
}

// DisableToolsets disables multiple toolsets
func (tm *ToolsetManager) DisableToolsets(ctx context.Context, names []string) error {
	for _, name := range names {
		if err := tm.DisableToolset(ctx, name); err != nil {
			return err
		}
	}
	return nil
}

// GetToolset returns a toolset by name
func (tm *ToolsetManager) GetToolset(name string) (mcptools.Toolset, bool) {
	toolset, exists := tm.toolsets[name]
	return toolset, exists
}

// ListToolsets returns all registered toolsets
func (tm *ToolsetManager) ListToolsets() []mcptools.Toolset {
	toolsets := make([]mcptools.Toolset, 0, len(tm.toolsets))
	for _, toolset := range tm.toolsets {
		toolsets = append(toolsets, toolset)
	}
	return toolsets
}

// ListEnabledToolsets returns only enabled toolsets
func (tm *ToolsetManager) ListEnabledToolsets() []mcptools.Toolset {
	toolsets := make([]mcptools.Toolset, 0)
	for _, toolset := range tm.toolsets {
		if toolset.Enabled {
			toolsets = append(toolsets, toolset)
		}
	}
	return toolsets
}

// ListToolsetsByProvider returns toolsets filtered by provider
func (tm *ToolsetManager) ListToolsetsByProvider(provider string) []mcptools.Toolset {
	toolsets := make([]mcptools.Toolset, 0)
	for _, toolset := range tm.toolsets {
		if toolset.Provider == provider {
			toolsets = append(toolsets, toolset)
		}
	}
	return toolsets
}

// GenerateToolDefinitions generates tool definitions for a toolset with specified detail level
func (tm *ToolsetManager) GenerateToolDefinitions(toolsetName string, detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
	toolset, exists := tm.toolsets[toolsetName]
	if !exists {
		return nil, fmt.Errorf("toolset not found: %s", toolsetName)
	}

	return toolset.ToolGenerator(detailLevel, actions)
}

// convertMCPToInternalToolDef converts a pkg/mcptools.ToolDefinition to internal ToolDefinition
func convertMCPToInternalToolDef(mcpTool mcptools.ToolDefinition) ToolDefinition {
	return ToolDefinition{
		Name:             mcpTool.Name,
		Description:      mcpTool.Description,
		InputSchema:      mcpTool.InputSchema,
		Handler:          ToolHandler(mcpTool.Handler), // Cast the handler - signatures are compatible
		Category:         mcpTool.Category,
		Tags:             mcpTool.Tags,
		Prerequisites:    mcpTool.Prerequisites,
		CommonlyUsedWith: mcpTool.CommonlyUsedWith,
		NextSteps:        mcpTool.NextSteps,
		Alternatives:     mcpTool.Alternatives,
		ConflictsWith:    mcpTool.ConflictsWith,
		IOCompatibility:  convertMCPIOCompat(mcpTool.IOCompatibility),
	}
}

// convertMCPIOCompat converts pkg/mcptools.IOCompatibility to internal IOCompatibility
func convertMCPIOCompat(mcpIO *mcptools.IOCompatibility) *IOCompatibility {
	if mcpIO == nil {
		return nil
	}

	return &IOCompatibility{
		InputType: DataType{
			Format:      mcpIO.InputType.Format,
			Schema:      mcpIO.InputType.Schema,
			ContentType: mcpIO.InputType.ContentType,
			Properties:  mcpIO.InputType.Properties,
		},
		OutputType: DataType{
			Format:      mcpIO.OutputType.Format,
			Schema:      mcpIO.OutputType.Schema,
			ContentType: mcpIO.OutputType.ContentType,
			Properties:  mcpIO.OutputType.Properties,
		},
	}
}
