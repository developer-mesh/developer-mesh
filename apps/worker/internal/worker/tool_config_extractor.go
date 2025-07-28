package worker

import (
	"context"
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
)

// ToolConfigExtractorImpl extracts tool configuration from events
type ToolConfigExtractorImpl struct {
	toolRepo repository.DynamicToolRepository
	logger   observability.Logger
}

// NewToolConfigExtractor creates a new tool config extractor
func NewToolConfigExtractor(toolRepo repository.DynamicToolRepository, logger observability.Logger) ToolConfigExtractor {
	return &ToolConfigExtractorImpl{
		toolRepo: toolRepo,
		logger:   logger,
	}
}

// ExtractToolConfig extracts the tool configuration from an event
func (e *ToolConfigExtractorImpl) ExtractToolConfig(ctx context.Context, event queue.Event) (*models.DynamicTool, error) {
	// Extract tool ID from event metadata
	if event.Metadata == nil {
		return nil, fmt.Errorf("event metadata is nil")
	}

	toolID, ok := event.Metadata["tool_id"].(string)
	if !ok || toolID == "" {
		return nil, fmt.Errorf("tool_id not found in event metadata")
	}

	// Fetch tool configuration from repository
	tool, err := e.toolRepo.GetByID(ctx, toolID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tool configuration: %w", err)
	}

	if tool == nil {
		return nil, fmt.Errorf("tool not found: %s", toolID)
	}

	// Validate tool is active
	if tool.Status != "active" {
		return nil, fmt.Errorf("tool is not active: %s (status: %s)", toolID, tool.Status)
	}

	// Validate webhook config exists and is enabled
	if tool.WebhookConfig == nil || !tool.WebhookConfig.Enabled {
		return nil, fmt.Errorf("webhook config is disabled for tool: %s", toolID)
	}

	e.logger.Debug("Extracted tool config", map[string]interface{}{
		"tool_id":   toolID,
		"tool_name": tool.ToolName,
		"provider":  tool.Provider,
		"tenant_id": tool.TenantID,
	})

	return tool, nil
}
