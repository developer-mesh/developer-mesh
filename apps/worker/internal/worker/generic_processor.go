package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/jmoiron/sqlx"
)

// GenericWebhookProcessor processes webhook events for any tool
type GenericWebhookProcessor struct {
	logger           observability.Logger
	metrics          observability.MetricsClient
	metricsCollector *MetricsCollector
	db               *sqlx.DB
	toolRepo         repository.DynamicToolRepository
	eventRepo        repository.WebhookEventRepository
	configExtractor  ToolConfigExtractor
	transformer      EventTransformer
	retryHandler     *RetryHandler
}

// NewGenericWebhookProcessor creates a new generic webhook processor
func NewGenericWebhookProcessor(
	logger observability.Logger,
	metrics observability.MetricsClient,
	db *sqlx.DB,
	queueClient *queue.Client,
) (*GenericWebhookProcessor, error) {
	if logger == nil {
		logger = observability.NewLogger("webhook-processor")
	}
	if metrics == nil {
		metrics = observability.NewMetricsClient()
	}

	toolRepo := repository.NewDynamicToolRepository(db)
	eventRepo := repository.NewWebhookEventRepository(db)

	// Create metrics collector
	tracer := observability.GetTracer()
	metricsCollector := NewMetricsCollector(metrics, tracer)

	// Create DLQ handler
	dlqHandler := NewDLQHandler(db, logger, metrics, queueClient)

	// Create retry handler with DLQ
	retryHandler := NewRetryHandler(nil, logger, dlqHandler, metricsCollector)

	return &GenericWebhookProcessor{
		logger:           logger,
		metrics:          metrics,
		metricsCollector: metricsCollector,
		db:               db,
		toolRepo:         toolRepo,
		eventRepo:        eventRepo,
		configExtractor:  NewToolConfigExtractor(toolRepo, logger),
		transformer:      NewEventTransformer(logger),
		retryHandler:     retryHandler,
	}, nil
}

// ProcessEvent processes a webhook event generically based on tool configuration
func (p *GenericWebhookProcessor) ProcessEvent(ctx context.Context, event queue.Event) error {
	// Wrap the actual processing with retry logic
	return p.retryHandler.ExecuteWithRetry(ctx, event, func() error {
		return p.processEventInternal(ctx, event)
	})
}

// processEventInternal contains the actual event processing logic
func (p *GenericWebhookProcessor) processEventInternal(ctx context.Context, event queue.Event) error {
	start := time.Now()

	// Extract tool configuration from event
	tool, err := p.configExtractor.ExtractToolConfig(ctx, event)
	if err != nil {
		p.logger.Error("Failed to extract tool config", map[string]interface{}{
			"event_id": event.EventID,
			"error":    err.Error(),
		})
		p.recordMetrics(nil, event.EventType, "error", time.Since(start))
		return fmt.Errorf("failed to extract tool config: %w", err)
	}

	// Start distributed tracing span
	toolID := ""
	if tool != nil {
		toolID = tool.ID
	}
	ctx, span := p.metricsCollector.RecordProcessingStart(ctx, event.EventID, event.EventType, toolID)

	// Log with tool context
	p.logWithContext("info", "Processing webhook event", event, tool, map[string]interface{}{
		"processing_start": start,
	})

	// Validate event
	if err := p.ValidateEvent(event); err != nil {
		p.logWithContext("error", "Event validation failed", event, tool, map[string]interface{}{
			"error": err.Error(),
		})
		p.recordMetrics(tool, event.EventType, "validation_failed", time.Since(start))
		return fmt.Errorf("event validation failed: %w", err)
	}

	// Update event status to processing
	if err := p.updateEventStatus(ctx, event.EventID, "processing", ""); err != nil {
		p.logWithContext("warn", "Failed to update event status", event, tool, map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Get processing mode
	mode := p.getProcessingMode(tool, event.EventType)

	// Process based on mode
	err = p.processWithMode(ctx, event, tool, mode)

	// Update final status
	if err != nil {
		if statusErr := p.updateEventStatus(ctx, event.EventID, "failed", err.Error()); statusErr != nil {
			p.logger.Warn("Failed to update event status to failed", map[string]interface{}{
				"event_id": event.EventID,
				"error":    statusErr.Error(),
			})
		}
		p.logWithContext("error", "Event processing failed", event, tool, map[string]interface{}{
			"error":       err.Error(),
			"duration_ms": time.Since(start).Milliseconds(),
		})
		p.recordMetrics(tool, event.EventType, "failed", time.Since(start))
		p.metricsCollector.RecordProcessingComplete(span, event.EventType, toolID, "failed", time.Since(start))
		return err
	}

	if statusErr := p.updateEventStatus(ctx, event.EventID, "completed", ""); statusErr != nil {
		p.logger.Warn("Failed to update event status to completed", map[string]interface{}{
			"event_id": event.EventID,
			"error":    statusErr.Error(),
		})
	}
	p.logWithContext("info", "Event processed successfully", event, tool, map[string]interface{}{
		"duration_ms": time.Since(start).Milliseconds(),
	})
	p.recordMetrics(tool, event.EventType, "success", time.Since(start))
	p.metricsCollector.RecordProcessingComplete(span, event.EventType, toolID, "success", time.Since(start))

	return nil
}

// ValidateEvent validates the event has required fields
func (p *GenericWebhookProcessor) ValidateEvent(event queue.Event) error {
	if event.EventID == "" {
		return fmt.Errorf("event ID is required")
	}
	if event.EventType == "" {
		return fmt.Errorf("event type is required")
	}
	if len(event.Payload) == 0 {
		return fmt.Errorf("event payload is required")
	}

	// Validate JSON payload
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("invalid JSON payload: %w", err)
	}

	return nil
}

// GetProcessingMode returns the processing mode
func (p *GenericWebhookProcessor) GetProcessingMode() string {
	return string(ModeStoreOnly) // Default mode
}

// processWithMode processes the event based on the configured mode
func (p *GenericWebhookProcessor) processWithMode(
	ctx context.Context,
	event queue.Event,
	tool *models.DynamicTool,
	mode ProcessingMode,
) error {
	switch mode {
	case ModeStoreOnly:
		return p.storeEvent(ctx, event, tool)

	case ModeStoreAndForward:
		if err := p.storeEvent(ctx, event, tool); err != nil {
			return err
		}
		return p.forwardEvent(ctx, event, tool)

	case ModeTransformAndStore:
		transformedEvent, err := p.transformEvent(ctx, event, tool)
		if err != nil {
			return err
		}
		return p.storeEvent(ctx, transformedEvent, tool)

	default:
		return fmt.Errorf("unknown processing mode: %s", mode)
	}
}

// storeEvent stores the event (already stored by REST API, this updates metadata)
func (p *GenericWebhookProcessor) storeEvent(_ context.Context, event queue.Event, tool *models.DynamicTool) error {
	// Event is already stored by the REST API webhook handler
	// Here we just log that it was processed
	p.logger.Debug("Event stored", map[string]interface{}{
		"event_id":  event.EventID,
		"tool_id":   tool.ID,
		"tool_name": tool.ToolName,
	})
	return nil
}

// forwardEvent forwards the event to another service
func (p *GenericWebhookProcessor) forwardEvent(_ context.Context, event queue.Event, tool *models.DynamicTool) error {
	// TODO: Implement forwarding logic based on tool configuration
	p.logger.Info("Event forwarding not yet implemented", map[string]interface{}{
		"event_id": event.EventID,
		"tool_id":  tool.ID,
	})
	return nil
}

// transformEvent transforms the event based on tool configuration
func (p *GenericWebhookProcessor) transformEvent(ctx context.Context, event queue.Event, tool *models.DynamicTool) (queue.Event, error) {
	if tool.WebhookConfig == nil {
		return event, nil
	}

	// Find transformation rules for this event type
	for _, eventConfig := range tool.WebhookConfig.Events {
		if eventConfig.EventType == event.EventType && len(eventConfig.TransformRules) > 0 {
			start := time.Now()
			transformedEvent, err := p.transformer.Transform(event, eventConfig.TransformRules)

			// Record transformation metrics
			p.metricsCollector.RecordTransformation(ctx, event.EventID, len(eventConfig.TransformRules), time.Since(start))

			return transformedEvent, err
		}
	}

	return event, nil
}

// getProcessingMode determines the processing mode for the event
func (p *GenericWebhookProcessor) getProcessingMode(tool *models.DynamicTool, eventType string) ProcessingMode {
	if tool.WebhookConfig == nil {
		return ModeStoreOnly
	}

	// Check event-specific mode
	for _, event := range tool.WebhookConfig.Events {
		if event.EventType == eventType && event.ProcessingMode != "" {
			return ProcessingMode(event.ProcessingMode)
		}
	}

	// Use default mode from webhook config
	if tool.WebhookConfig.DefaultProcessingMode != "" {
		return ProcessingMode(tool.WebhookConfig.DefaultProcessingMode)
	}

	return ModeStoreOnly
}

// updateEventStatus updates the status of a webhook event
func (p *GenericWebhookProcessor) updateEventStatus(ctx context.Context, eventID, status, errorMsg string) error {
	if p.eventRepo == nil {
		return nil // Skip if eventRepo not configured (e.g., in tests)
	}
	processedAt := time.Now()
	return p.eventRepo.UpdateStatus(ctx, eventID, status, &processedAt, errorMsg)
}

// logWithContext logs with tool and event context
func (p *GenericWebhookProcessor) logWithContext(
	level string,
	message string,
	event queue.Event,
	tool *models.DynamicTool,
	extra map[string]interface{},
) {
	fields := map[string]interface{}{
		"event_id":    event.EventID,
		"event_type":  event.EventType,
		"tool_id":     tool.ID,
		"tool_name":   tool.ToolName,
		"provider":    tool.Provider,
		"tenant_id":   tool.TenantID,
		"received_at": event.Timestamp,
	}

	// Merge extra fields
	for k, v := range extra {
		fields[k] = v
	}

	switch level {
	case "info":
		p.logger.Info(message, fields)
	case "warn":
		p.logger.Warn(message, fields)
	case "error":
		p.logger.Error(message, fields)
	case "debug":
		p.logger.Debug(message, fields)
	}
}

// recordMetrics records processing metrics
func (p *GenericWebhookProcessor) recordMetrics(tool *models.DynamicTool, eventType, status string, duration time.Duration) {
	labels := map[string]string{
		"event_type": eventType,
		"status":     status,
	}

	if tool != nil {
		labels["tool_id"] = tool.ID
		labels["tool_name"] = tool.ToolName
		labels["provider"] = tool.Provider
	}

	p.metrics.IncrementCounterWithLabels("webhook_events_processed_total", 1, labels)
	p.metrics.RecordHistogram("webhook_event_processing_duration_seconds", duration.Seconds(), labels)
}
