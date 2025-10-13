package worker

import (
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/webhook/handlers"
)

// GitHubReleaseHandler is a type alias for the package-level handler
type GitHubReleaseHandler = handlers.GitHubReleaseHandler

// NewGitHubReleaseHandler creates a new GitHub release handler
func NewGitHubReleaseHandler(
	releaseRepo repository.PackageReleaseRepository,
	queueClient *queue.Client,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *GitHubReleaseHandler {
	return handlers.NewGitHubReleaseHandler(releaseRepo, queueClient, logger, metrics)
}
