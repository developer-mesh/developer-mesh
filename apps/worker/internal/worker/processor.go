package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/jmoiron/sqlx"
)

// EventProcessor handles webhook events using the generic processor
type EventProcessor struct {
	genericProcessor           WebhookEventProcessor
	contextEmbeddingProcessor  *ContextEmbeddingProcessor
	packageEnrichmentProcessor *PackageEnrichmentProcessor
	githubReleaseHandler       *GitHubReleaseHandler
	artifactoryWebhookHandler  *ArtifactoryWebhookHandler
	logger                     observability.Logger
	metrics                    observability.MetricsClient
}

// NewEventProcessor creates a new processor for webhook events
func NewEventProcessor(logger observability.Logger, metrics observability.MetricsClient, db *sqlx.DB, queueClient *queue.Client) (*EventProcessor, error) {
	if logger == nil {
		logger = observability.NewLogger("webhook-processor")
	}
	if metrics == nil {
		metrics = observability.NewMetricsClient()
	}

	processor := &EventProcessor{
		logger:  logger,
		metrics: metrics,
	}

	// Initialize the generic processor
	genericProcessor, err := NewGenericWebhookProcessor(logger, metrics, db, queueClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create generic processor: %w", err)
	}
	processor.genericProcessor = genericProcessor

	return processor, nil
}

// ProcessSQSEvent processes a webhook event from SQS (for backward compatibility)
func (p *EventProcessor) ProcessSQSEvent(ctx context.Context, event queue.SQSEvent) error {
	// Convert SQSEvent to Event
	queueEvent := queue.Event{
		EventID:     event.DeliveryID,
		EventType:   event.EventType,
		RepoName:    event.RepoName,
		SenderName:  event.SenderName,
		Payload:     event.Payload,
		AuthContext: event.AuthContext,
		Timestamp:   time.Now(),                   // SQSEvent doesn't have timestamp
		Metadata:    make(map[string]interface{}), // SQSEvent doesn't have metadata
	}

	return p.ProcessEvent(ctx, queueEvent)
}

// ProcessEvent processes a webhook event
func (p *EventProcessor) ProcessEvent(ctx context.Context, event queue.Event) error {
	// Route based on event type
	switch event.EventType {
	case "context.items.created":
		if p.contextEmbeddingProcessor != nil {
			return p.contextEmbeddingProcessor.ProcessEvent(ctx, event)
		}
		// Fall through to generic processor if context processor not configured
		p.logger.Debug("Context embedding processor not configured, skipping event", map[string]interface{}{
			"event_type": event.EventType,
		})
		return nil

	case "package.enrichment":
		if p.packageEnrichmentProcessor != nil {
			return p.packageEnrichmentProcessor.ProcessEvent(ctx, event)
		}
		// Fall through to generic processor if enrichment processor not configured
		p.logger.Debug("Package enrichment processor not configured, skipping event", map[string]interface{}{
			"event_type": event.EventType,
		})
		return nil

	case "release", "github.release":
		if p.githubReleaseHandler != nil {
			return p.githubReleaseHandler.Handle(ctx, event)
		}
		// Fall through to generic processor if release handler not configured
		p.logger.Debug("GitHub release handler not configured, using generic processor", map[string]interface{}{
			"event_type": event.EventType,
		})

	case "artifactory", "artifactory.deployed", "artifact.deployed":
		if p.artifactoryWebhookHandler != nil {
			return p.artifactoryWebhookHandler.Handle(ctx, event)
		}
		// Fall through to generic processor if Artifactory handler not configured
		p.logger.Debug("Artifactory webhook handler not configured, using generic processor", map[string]interface{}{
			"event_type": event.EventType,
		})
	}

	// Default to generic processor for all other events
	if p.genericProcessor == nil {
		return fmt.Errorf("processor not initialized")
	}

	return p.genericProcessor.ProcessEvent(ctx, event)
}

// SetContextEmbeddingProcessor sets the context embedding processor
func (p *EventProcessor) SetContextEmbeddingProcessor(processor *ContextEmbeddingProcessor) {
	p.contextEmbeddingProcessor = processor
}

// SetPackageEnrichmentProcessor sets the package enrichment processor
func (p *EventProcessor) SetPackageEnrichmentProcessor(processor *PackageEnrichmentProcessor) {
	p.packageEnrichmentProcessor = processor
}

// SetGitHubReleaseHandler sets the GitHub release handler
func (p *EventProcessor) SetGitHubReleaseHandler(handler *GitHubReleaseHandler) {
	p.githubReleaseHandler = handler
}

// SetArtifactoryWebhookHandler sets the Artifactory webhook handler
func (p *EventProcessor) SetArtifactoryWebhookHandler(handler *ArtifactoryWebhookHandler) {
	p.artifactoryWebhookHandler = handler
}
