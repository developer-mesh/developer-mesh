package worker

import (
	"context"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
)

// WebhookEventProcessor defines the interface for processing webhook events
type WebhookEventProcessor interface {
	// ProcessEvent processes a webhook event
	ProcessEvent(ctx context.Context, event queue.Event) error

	// ValidateEvent validates an event before processing
	ValidateEvent(event queue.Event) error

	// GetProcessingMode returns the processing mode for this processor
	GetProcessingMode() string
}

// ProcessingMode defines how events should be processed
type ProcessingMode string

const (
	// ModeStoreOnly just stores the event
	ModeStoreOnly ProcessingMode = "store_only"

	// ModeStoreAndForward stores and forwards to another service
	ModeStoreAndForward ProcessingMode = "store_and_forward"

	// ModeTransformAndStore transforms the event then stores
	ModeTransformAndStore ProcessingMode = "transform_and_store"
)

// ToolConfigExtractor extracts tool configuration from events
type ToolConfigExtractor interface {
	// ExtractToolConfig extracts the tool configuration from an event
	ExtractToolConfig(ctx context.Context, event queue.Event) (*models.DynamicTool, error)
}

// EventRouter routes events to appropriate processors
type EventRouter interface {
	// GetProcessingMode determines the processing mode for an event
	GetProcessingMode(tool *models.DynamicTool, eventType string) ProcessingMode

	// RouteEvent routes an event to the appropriate processor
	RouteEvent(ctx context.Context, event queue.Event, tool *models.DynamicTool) error
}

// EventTransformer transforms events based on rules
type EventTransformer interface {
	// Transform applies transformation rules to an event
	Transform(event queue.Event, rules map[string]interface{}) (queue.Event, error)
}
