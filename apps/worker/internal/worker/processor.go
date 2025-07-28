package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
)

// contextKey is a type for context keys
type contextKey string

const (
	// authContextKey is the key for auth context in context.Value
	authContextKey contextKey = "auth_context"
	// tenantIDKey is the key for tenant ID in context.Value
	tenantIDKey contextKey = "tenant_id"
)

// EventProcessor handles GitHub webhook events
type EventProcessor struct {
	logger       observability.Logger
	metrics      observability.MetricsClient
	mutex        sync.RWMutex
	successCount int64
	failureCount int64
	processors   map[string]EventTypeProcessor
}

// EventTypeProcessor defines the interface for type-specific event processors
type EventTypeProcessor interface {
	Process(ctx context.Context, event queue.SQSEvent, payload map[string]interface{}) error
}

// NewEventProcessor creates a new processor for GitHub webhook events
func NewEventProcessor(logger observability.Logger, metrics observability.MetricsClient) *EventProcessor {
	if logger == nil {
		logger = observability.NewLogger("github-webhook-processor")
	}
	if metrics == nil {
		metrics = observability.NewMetricsClient()
	}

	p := &EventProcessor{
		logger:     logger,
		metrics:    metrics,
		mutex:      sync.RWMutex{},
		processors: make(map[string]EventTypeProcessor),
	}

	// Register processors for each supported event type
	p.registerProcessors()
	return p
}

// registerProcessors registers handlers for each GitHub webhook event type
func (p *EventProcessor) registerProcessors() {
	p.processors["push"] = &PushProcessor{BaseProcessor: BaseProcessor{Logger: p.logger, Metrics: p.metrics}}
	p.processors["pull_request"] = &PullRequestProcessor{BaseProcessor: BaseProcessor{Logger: p.logger, Metrics: p.metrics}}
	p.processors["issues"] = &IssuesProcessor{BaseProcessor: BaseProcessor{Logger: p.logger, Metrics: p.metrics}}
	p.processors["issue_comment"] = &IssueCommentProcessor{BaseProcessor: BaseProcessor{Logger: p.logger, Metrics: p.metrics}}
	p.processors["repository"] = &RepositoryProcessor{BaseProcessor: BaseProcessor{Logger: p.logger, Metrics: p.metrics}}
	p.processors["release"] = &ReleaseProcessor{BaseProcessor: BaseProcessor{Logger: p.logger, Metrics: p.metrics}}
	p.processors["workflow_run"] = &WorkflowRunProcessor{BaseProcessor: BaseProcessor{Logger: p.logger, Metrics: p.metrics}}
	p.processors["workflow_job"] = &WorkflowJobProcessor{BaseProcessor: BaseProcessor{Logger: p.logger, Metrics: p.metrics}}
	p.processors["check_run"] = &CheckRunProcessor{BaseProcessor: BaseProcessor{Logger: p.logger, Metrics: p.metrics}}
	p.processors["deployment"] = &DeploymentProcessor{BaseProcessor: BaseProcessor{Logger: p.logger, Metrics: p.metrics}}
	p.processors["deployment_status"] = &DeploymentStatusProcessor{BaseProcessor: BaseProcessor{Logger: p.logger, Metrics: p.metrics}}
	p.processors["dependabot_alert"] = &DependabotAlertProcessor{BaseProcessor: BaseProcessor{Logger: p.logger, Metrics: p.metrics}}
	// Default processor for handling any other event types
	p.processors["default"] = &DefaultProcessor{BaseProcessor: BaseProcessor{Logger: p.logger, Metrics: p.metrics}}
}

// ProcessSQSEvent processes a GitHub webhook event from SQS
func (p *EventProcessor) ProcessSQSEvent(ctx context.Context, event queue.SQSEvent) error {
	start := time.Now()

	// Log event with auth context
	logFields := map[string]interface{}{
		"delivery_id": event.DeliveryID,
		"event_type":  event.EventType,
		"repo":        event.RepoName,
		"sender":      event.SenderName,
	}

	// Add auth context to logs if present
	if event.AuthContext != nil {
		logFields["tenant_id"] = event.AuthContext.TenantID
		logFields["principal_id"] = event.AuthContext.PrincipalID
		logFields["principal_type"] = event.AuthContext.PrincipalType
		if event.AuthContext.InstallationID != nil {
			logFields["installation_id"] = *event.AuthContext.InstallationID
		}
	}

	p.logger.Info("Processing webhook event", logFields)

	// Record metrics for this event
	p.metrics.IncrementCounterWithLabels("webhook_events_received_total", 1, map[string]string{
		"event_type": event.EventType,
		"repo":       event.RepoName,
	})

	// Extract and validate payload
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		p.incrementFailureCount()
		p.logger.Error("Failed to unmarshal payload", map[string]interface{}{
			"delivery_id": event.DeliveryID,
			"error":       err.Error(),
		})
		p.metrics.IncrementCounterWithLabels("webhook_events_failed_total", 1, map[string]string{
			"event_type": event.EventType,
			"reason":     "parse_failure",
		})
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Log payload keys for debugging
	p.logger.Debug("Webhook payload keys", map[string]interface{}{
		"delivery_id": event.DeliveryID,
		"keys":        keys(payload),
	})

	// Process the event with the appropriate handler
	err := p.processEvent(ctx, event, payload)

	// Record metrics
	duration := time.Since(start)
	p.metrics.RecordHistogram("webhook_event_processing_duration_seconds", duration.Seconds(), map[string]string{
		"event_type": event.EventType,
		"success":    fmt.Sprintf("%t", err == nil),
	})

	if err != nil {
		p.incrementFailureCount()
		p.logger.Error("Failed to process event", map[string]interface{}{
			"delivery_id": event.DeliveryID,
			"event_type":  event.EventType,
			"error":       err.Error(),
			"duration_ms": float64(duration.Milliseconds()),
		})
		return err
	}

	p.incrementSuccessCount()
	p.logger.Info("Successfully processed event", map[string]interface{}{
		"delivery_id":   event.DeliveryID,
		"event_type":    event.EventType,
		"duration_ms":   float64(duration.Milliseconds()),
		"success_count": p.getSuccessCount(),
		"failure_count": p.getFailureCount(),
	})
	return nil
}

// ProcessEvent processes a GitHub webhook event
// Returns error if processing fails (to trigger retry).
func ProcessEvent(event queue.Event) error {
	// Convert Event to SQSEvent for backward compatibility
	sqsEvent := queue.SQSEvent{
		DeliveryID:  event.EventID,
		EventType:   event.EventType,
		RepoName:    event.RepoName,
		SenderName:  event.SenderName,
		Payload:     event.Payload,
		AuthContext: event.AuthContext,
	}

	ctx := context.Background()
	processor := NewEventProcessor(nil, nil)
	return processor.ProcessSQSEvent(ctx, sqsEvent)
}

// ProcessSQSEvent contains the actual business logic for handling webhook events.
// Returns error if processing fails (to trigger SQS retry).
// This function remains for backward compatibility
func ProcessSQSEvent(event queue.SQSEvent) error {
	ctx := context.Background()
	processor := NewEventProcessor(nil, nil)
	return processor.ProcessSQSEvent(ctx, event)
}

// processEvent selects the appropriate event processor based on event type
func (p *EventProcessor) processEvent(ctx context.Context, event queue.SQSEvent, payload map[string]interface{}) error {
	processor, exists := p.processors[event.EventType]
	if !exists {
		p.logger.Warn("No specific processor for event type, using default", map[string]interface{}{
			"event_type": event.EventType,
		})
		processor = p.processors["default"]
	}

	// Add auth context to the processing context
	if event.AuthContext != nil {
		ctx = context.WithValue(ctx, authContextKey, event.AuthContext)
		ctx = context.WithValue(ctx, tenantIDKey, event.AuthContext.TenantID)
	}

	// Process with appropriate handler
	return processor.Process(ctx, event, payload)
}

// Helper methods for thread-safe counters
func (p *EventProcessor) incrementSuccessCount() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.successCount++
}

func (p *EventProcessor) incrementFailureCount() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.failureCount++
}

func (p *EventProcessor) getSuccessCount() int64 {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.successCount
}

func (p *EventProcessor) getFailureCount() int64 {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.failureCount
}

func keys(m map[string]interface{}) []string {
	var k []string
	for key := range m {
		k = append(k, key)
	}
	return k
}
