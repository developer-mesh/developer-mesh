package worker

import (
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/webhook/handlers"
)

// ArtifactoryWebhookHandler is a type alias for the pkg handler
type ArtifactoryWebhookHandler = handlers.ArtifactoryWebhookHandler

// NewArtifactoryWebhookHandler creates a new Artifactory webhook handler
func NewArtifactoryWebhookHandler(
	releaseRepo repository.PackageReleaseRepository,
	artifactoryURL string,
	artifactoryAPIKey string,
	queueClient *queue.Client,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *ArtifactoryWebhookHandler {
	// Create the Artifactory client
	artifactoryClient := handlers.NewArtifactoryClient(artifactoryURL, artifactoryAPIKey, logger)

	// Create and return the handler
	return handlers.NewArtifactoryWebhookHandler(releaseRepo, artifactoryClient, queueClient, logger, metrics)
}
