package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/google/uuid"
)

// ArtifactoryWebhookHandler processes JFrog Artifactory webhook events
type ArtifactoryWebhookHandler struct {
	releaseRepo       repository.PackageReleaseRepository
	artifactoryClient *ArtifactoryClient
	releaseMatcher    *GitHubReleaseMatcher
	queueClient       *queue.Client
	logger            observability.Logger
	metrics           observability.MetricsClient
}

// NewArtifactoryWebhookHandler creates a new Artifactory webhook handler
func NewArtifactoryWebhookHandler(
	releaseRepo repository.PackageReleaseRepository,
	artifactoryClient *ArtifactoryClient,
	queueClient *queue.Client,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *ArtifactoryWebhookHandler {
	if logger == nil {
		logger = observability.NewLogger("artifactory-webhook-handler")
	}
	if metrics == nil {
		metrics = observability.NewMetricsClient()
	}

	return &ArtifactoryWebhookHandler{
		releaseRepo:       releaseRepo,
		artifactoryClient: artifactoryClient,
		releaseMatcher:    NewGitHubReleaseMatcher(releaseRepo, logger),
		queueClient:       queueClient,
		logger:            logger,
		metrics:           metrics,
	}
}

// ArtifactoryWebhookPayload represents the Artifactory webhook payload
type ArtifactoryWebhookPayload struct {
	Domain    string               `json:"domain"`
	EventType string               `json:"event_type"`
	Timestamp int64                `json:"timestamp"`
	Data      ArtifactoryEventData `json:"data"`
}

// ArtifactoryEventData contains the artifact details
type ArtifactoryEventData struct {
	RepoPath   RepoPath               `json:"repoPath"`
	Name       string                 `json:"name"`
	Path       string                 `json:"path"`
	Properties map[string]interface{} `json:"properties"`
	Size       int64                  `json:"size"`
	SHA1       string                 `json:"sha1"`
	SHA256     string                 `json:"sha256"`
	MD5        string                 `json:"md5"`
	Created    int64                  `json:"created"`
	CreatedBy  string                 `json:"created_by"`
	Modified   int64                  `json:"modified"`
	ModifiedBy string                 `json:"modified_by"`
}

// RepoPath represents the repository path structure
type RepoPath struct {
	RepoKey string `json:"repoKey"`
	Path    string `json:"path"`
}

// PackageInfo represents parsed package information from artifact path
type PackageInfo struct {
	Name       string
	Version    string
	Type       models.PackageType
	GroupID    string // For Maven
	ArtifactID string // For Maven
}

// Handle processes an Artifactory webhook event
func (h *ArtifactoryWebhookHandler) Handle(ctx context.Context, event queue.Event) error {
	start := time.Now()
	defer func() {
		h.metrics.RecordHistogram("webhook_artifactory_duration_seconds", time.Since(start).Seconds(), nil)
	}()

	// Parse the payload
	var payload ArtifactoryWebhookPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		h.logger.Error("Failed to parse Artifactory webhook payload", map[string]interface{}{
			"event_id": event.EventID,
			"error":    err.Error(),
		})
		h.metrics.IncrementCounter("webhook_artifactory_parse_errors_total", 1)
		return fmt.Errorf("failed to parse Artifactory webhook payload: %w", err)
	}

	// Only process deployed (published) events
	if payload.EventType != "deployed" && payload.EventType != "artifact.deployed" {
		h.logger.Debug("Skipping non-deployed artifact event", map[string]interface{}{
			"event_id":   event.EventID,
			"event_type": payload.EventType,
		})
		return nil
	}

	// Extract tenant ID from auth context
	tenantID, err := h.extractTenantID(event)
	if err != nil {
		h.logger.Error("Failed to extract tenant ID", map[string]interface{}{
			"event_id": event.EventID,
			"error":    err.Error(),
		})
		return fmt.Errorf("failed to extract tenant ID: %w", err)
	}

	h.logger.Info("Processing Artifactory webhook", map[string]interface{}{
		"event_id":   event.EventID,
		"tenant_id":  tenantID,
		"repo_key":   payload.Data.RepoPath.RepoKey,
		"path":       payload.Data.Path,
		"event_type": payload.EventType,
	})

	// Parse package information from path
	packageInfo := h.parseArtifactPath(payload.Data.Path)
	if packageInfo == nil {
		h.logger.Warn("Could not parse package information from path", map[string]interface{}{
			"path": payload.Data.Path,
		})
		return nil // Not an error, just can't process this artifact
	}

	// Try to match with existing GitHub release
	githubRelease, err := h.releaseMatcher.FindMatchingRelease(ctx, tenantID, packageInfo)
	if err != nil {
		h.logger.Warn("Failed to find matching GitHub release", map[string]interface{}{
			"package": packageInfo.Name,
			"version": packageInfo.Version,
			"error":   err.Error(),
		})
		// Continue processing even if we can't match to GitHub
	}

	// Fetch additional metadata from Artifactory if client is configured
	var metadata map[string]interface{}
	if h.artifactoryClient != nil {
		metadata, err = h.artifactoryClient.GetArtifactProperties(ctx, payload.Data.RepoPath.RepoKey, payload.Data.Path)
		if err != nil {
			h.logger.Warn("Failed to fetch Artifactory metadata", map[string]interface{}{
				"repo_key": payload.Data.RepoPath.RepoKey,
				"path":     payload.Data.Path,
				"error":    err.Error(),
			})
		}
	}

	// Update or create package release record
	var releaseID uuid.UUID
	if githubRelease != nil {
		// Update existing GitHub release with Artifactory information
		if err := h.updateReleaseWithArtifactory(ctx, githubRelease, payload, packageInfo, metadata); err != nil {
			h.logger.Error("Failed to update release with Artifactory data", map[string]interface{}{
				"release_id": githubRelease.ID,
				"error":      err.Error(),
			})
			h.metrics.IncrementCounter("webhook_artifactory_storage_errors_total", 1)
			return fmt.Errorf("failed to update release with Artifactory data: %w", err)
		}
		releaseID = githubRelease.ID
	} else {
		// Create new release record from Artifactory data
		release, err := h.createReleaseFromArtifactory(ctx, tenantID, payload, packageInfo, metadata)
		if err != nil {
			h.logger.Error("Failed to create release from Artifactory data", map[string]interface{}{
				"package": packageInfo.Name,
				"version": packageInfo.Version,
				"error":   err.Error(),
			})
			h.metrics.IncrementCounter("webhook_artifactory_storage_errors_total", 1)
			return fmt.Errorf("failed to create release from Artifactory data: %w", err)
		}
		releaseID = release.ID
	}

	h.metrics.IncrementCounterWithLabels("webhook_artifactory_processed_total", 1, map[string]string{
		"repo_key":  payload.Data.RepoPath.RepoKey,
		"tenant_id": tenantID.String(),
	})

	// Publish enrichment event for Phase 3 processing (if queue client available)
	if h.queueClient != nil {
		if err := h.publishEnrichmentEvent(ctx, releaseID, tenantID, event.AuthContext); err != nil {
			// Log warning but don't fail the entire operation
			h.logger.Warn("Failed to publish enrichment event", map[string]interface{}{
				"release_id": releaseID,
				"error":      err.Error(),
			})
		}
	}

	return nil
}

// publishEnrichmentEvent publishes an event to trigger package enrichment
func (h *ArtifactoryWebhookHandler) publishEnrichmentEvent(ctx context.Context, releaseID uuid.UUID, tenantID uuid.UUID, authContext *queue.EventAuthContext) error {
	// Create enrichment event payload
	payload, err := json.Marshal(map[string]interface{}{
		"release_id": releaseID.String(),
		"tenant_id":  tenantID.String(),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal enrichment event: %w", err)
	}

	// Publish to queue
	enrichmentEvent := queue.Event{
		EventID:     uuid.New().String(),
		EventType:   "package.enrichment",
		Payload:     payload,
		AuthContext: authContext,
		Timestamp:   time.Now(),
		Metadata: map[string]interface{}{
			"source":     "artifactory_webhook_handler",
			"release_id": releaseID.String(),
		},
	}

	if err := h.queueClient.EnqueueEvent(ctx, enrichmentEvent); err != nil {
		h.metrics.IncrementCounter("webhook_enrichment_event_errors_total", 1)
		return fmt.Errorf("failed to enqueue enrichment event: %w", err)
	}

	h.logger.Debug("Published enrichment event", map[string]interface{}{
		"event_id":   enrichmentEvent.EventID,
		"release_id": releaseID,
	})

	return nil
}

// parseArtifactPath extracts package information from the artifact path
func (h *ArtifactoryWebhookHandler) parseArtifactPath(path string) *PackageInfo {
	ext := filepath.Ext(path)
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")

	switch ext {
	case ".jar", ".war", ".pom":
		return h.parseMavenPath(parts)
	case ".tgz", ".tar.gz":
		// Check for NPM format
		if strings.Contains(path, "/-/") {
			return h.parseNPMPath(parts)
		}
		return h.parseGenericPath(parts)
	case ".whl", ".egg", ".tar.bz2":
		return h.parsePythonPath(parts)
	case ".nupkg":
		return h.parseNuGetPath(parts)
	default:
		// Try generic parsing
		return h.parseGenericPath(parts)
	}
}

// parseMavenPath parses Maven coordinate format: /groupId/artifactId/version/artifactId-version.jar
func (h *ArtifactoryWebhookHandler) parseMavenPath(parts []string) *PackageInfo {
	if len(parts) < 3 {
		return nil
	}

	// Maven path: com/example/myapp/1.0.0/myapp-1.0.0.jar
	version := parts[len(parts)-2]
	artifactID := parts[len(parts)-3]

	// GroupID is everything before artifactID
	var groupID string
	if len(parts) > 3 {
		groupID = strings.Join(parts[:len(parts)-3], ".")
	}

	packageName := artifactID
	if groupID != "" {
		packageName = fmt.Sprintf("%s:%s", groupID, artifactID)
	}

	return &PackageInfo{
		Name:       packageName,
		Version:    version,
		Type:       models.PackageTypeMaven,
		GroupID:    groupID,
		ArtifactID: artifactID,
	}
}

// parseNPMPath parses NPM format: /packageName/-/packageName-version.tgz
func (h *ArtifactoryWebhookHandler) parseNPMPath(parts []string) *PackageInfo {
	// NPM format: @scope/package/-/package-1.0.0.tgz or package/-/package-1.0.0.tgz
	filename := parts[len(parts)-1]

	// Extract version from filename: package-1.0.0.tgz -> 1.0.0
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Find last hyphen to separate name from version
	lastHyphen := strings.LastIndex(name, "-")
	if lastHyphen == -1 {
		return nil
	}

	packageName := name[:lastHyphen]
	version := name[lastHyphen+1:]

	// Handle scoped packages
	if len(parts) > 3 && strings.HasPrefix(parts[0], "@") {
		packageName = parts[0] + "/" + packageName
	}

	return &PackageInfo{
		Name:    packageName,
		Version: version,
		Type:    models.PackageTypeNPM,
	}
}

// parsePythonPath parses Python package format
func (h *ArtifactoryWebhookHandler) parsePythonPath(parts []string) *PackageInfo {
	filename := parts[len(parts)-1]

	// Python format: package-1.0.0.tar.gz or package-1.0.0-py3-none-any.whl
	// Remove extension(s)
	name := filename
	for ext := filepath.Ext(name); ext != ""; ext = filepath.Ext(name) {
		name = strings.TrimSuffix(name, ext)
	}

	// Find version separator (usually after package name with hyphen)
	lastHyphen := strings.LastIndex(name, "-")
	if lastHyphen == -1 {
		return nil
	}

	packageName := name[:lastHyphen]
	version := name[lastHyphen+1:]

	return &PackageInfo{
		Name:    packageName,
		Version: version,
		Type:    models.PackageTypePython,
	}
}

// parseNuGetPath parses NuGet package format
func (h *ArtifactoryWebhookHandler) parseNuGetPath(parts []string) *PackageInfo {
	filename := parts[len(parts)-1]

	// NuGet format: PackageName.1.0.0.nupkg
	name := strings.TrimSuffix(filename, ".nupkg")

	// Find last dot to separate name from version
	lastDot := strings.LastIndex(name, ".")
	if lastDot == -1 {
		return nil
	}

	packageName := name[:lastDot]
	version := name[lastDot+1:]

	return &PackageInfo{
		Name:    packageName,
		Version: version,
		Type:    models.PackageTypeGo, // NuGet mapped to generic for now
	}
}

// parseGenericPath attempts generic parsing
func (h *ArtifactoryWebhookHandler) parseGenericPath(parts []string) *PackageInfo {
	if len(parts) < 2 {
		return nil
	}

	// Try to extract version from second-to-last part (common pattern)
	version := parts[len(parts)-2]
	packageName := parts[len(parts)-3]

	if len(parts) < 3 {
		// Fallback: use filename
		packageName = parts[len(parts)-1]
	}

	return &PackageInfo{
		Name:    packageName,
		Version: version,
		Type:    models.PackageTypeGeneric,
	}
}

// updateReleaseWithArtifactory updates an existing GitHub release with Artifactory information
func (h *ArtifactoryWebhookHandler) updateReleaseWithArtifactory(
	ctx context.Context,
	release *models.PackageRelease,
	payload ArtifactoryWebhookPayload,
	packageInfo *PackageInfo,
	metadata map[string]interface{},
) error {
	// Update Artifactory path
	artifactoryPath := fmt.Sprintf("%s/%s", payload.Data.RepoPath.RepoKey, payload.Data.Path)
	release.ArtifactoryPath = &artifactoryPath

	// Add Artifactory metadata to release metadata
	if release.Metadata == nil {
		release.Metadata = make(models.JSONMap)
	}

	release.Metadata["artifactory_repo"] = payload.Data.RepoPath.RepoKey
	release.Metadata["artifactory_path"] = payload.Data.Path
	release.Metadata["artifactory_deployed_at"] = time.Unix(payload.Timestamp/1000, 0).Format(time.RFC3339)
	release.Metadata["artifactory_deployed_by"] = payload.Data.CreatedBy

	if metadata != nil {
		release.Metadata["artifactory_metadata"] = metadata
	}

	// Update the release
	if err := h.releaseRepo.Update(ctx, release); err != nil {
		return fmt.Errorf("failed to update release: %w", err)
	}

	h.logger.Info("Updated release with Artifactory information", map[string]interface{}{
		"release_id":       release.ID,
		"package":          release.PackageName,
		"version":          release.Version,
		"artifactory_path": artifactoryPath,
	})

	// Create or update asset record
	asset := &models.PackageAsset{
		ReleaseID:      release.ID,
		Name:           payload.Data.Name,
		SizeBytes:      &payload.Data.Size,
		ArtifactoryURL: strPtr(fmt.Sprintf("https://artifactory.example.com/%s/%s", payload.Data.RepoPath.RepoKey, payload.Data.Path)),
		SHA256Checksum: strPtr(payload.Data.SHA256),
		SHA1Checksum:   strPtr(payload.Data.SHA1),
		MD5Checksum:    strPtr(payload.Data.MD5),
		Metadata: models.JSONMap{
			"artifactory_repo": payload.Data.RepoPath.RepoKey,
			"created_by":       payload.Data.CreatedBy,
			"modified_by":      payload.Data.ModifiedBy,
		},
	}

	if err := h.releaseRepo.CreateAsset(ctx, asset); err != nil {
		h.logger.Warn("Failed to create Artifactory asset record", map[string]interface{}{
			"release_id": release.ID,
			"asset_name": asset.Name,
			"error":      err.Error(),
		})
	}

	return nil
}

// createReleaseFromArtifactory creates a new release record from Artifactory data
func (h *ArtifactoryWebhookHandler) createReleaseFromArtifactory(
	ctx context.Context,
	tenantID uuid.UUID,
	payload ArtifactoryWebhookPayload,
	packageInfo *PackageInfo,
	metadata map[string]interface{},
) (*models.PackageRelease, error) {
	// Parse published timestamp
	publishedAt := time.Unix(payload.Timestamp/1000, 0)

	artifactoryPath := fmt.Sprintf("%s/%s", payload.Data.RepoPath.RepoKey, payload.Data.Path)

	// Create release record
	release := &models.PackageRelease{
		TenantID:        tenantID,
		RepositoryName:  payload.Data.RepoPath.RepoKey, // Use repo key as repository name
		PackageName:     packageInfo.Name,
		Version:         packageInfo.Version,
		PublishedAt:     publishedAt,
		AuthorLogin:     strPtr(payload.Data.CreatedBy),
		ArtifactoryPath: &artifactoryPath,
		PackageType:     string(packageInfo.Type),
		Metadata: models.JSONMap{
			"source":                  "artifactory",
			"artifactory_repo":        payload.Data.RepoPath.RepoKey,
			"artifactory_path":        payload.Data.Path,
			"artifactory_deployed_at": publishedAt.Format(time.RFC3339),
			"artifactory_deployed_by": payload.Data.CreatedBy,
		},
	}

	// Add Maven-specific metadata
	if packageInfo.Type == models.PackageTypeMaven && packageInfo.GroupID != "" {
		release.Metadata["maven_group_id"] = packageInfo.GroupID
		release.Metadata["maven_artifact_id"] = packageInfo.ArtifactID
	}

	// Add custom metadata if available
	if metadata != nil {
		release.Metadata["artifactory_metadata"] = metadata
	}

	// Parse semantic version if possible
	version := parseVersion(packageInfo.Version)
	if version != nil {
		release.VersionMajor = intPtr(version.Major)
		release.VersionMinor = intPtr(version.Minor)
		release.VersionPatch = intPtr(version.Patch)
		release.Prerelease = version.Prerelease
	}

	// Store the release
	if err := h.releaseRepo.Create(ctx, release); err != nil {
		return nil, fmt.Errorf("failed to create release: %w", err)
	}

	h.logger.Info("Created release from Artifactory data", map[string]interface{}{
		"release_id": release.ID,
		"package":    release.PackageName,
		"version":    release.Version,
		"repo_key":   payload.Data.RepoPath.RepoKey,
	})

	// Create asset record
	asset := &models.PackageAsset{
		ReleaseID:      release.ID,
		Name:           payload.Data.Name,
		SizeBytes:      &payload.Data.Size,
		ArtifactoryURL: strPtr(fmt.Sprintf("https://artifactory.example.com/%s/%s", payload.Data.RepoPath.RepoKey, payload.Data.Path)),
		SHA256Checksum: strPtr(payload.Data.SHA256),
		SHA1Checksum:   strPtr(payload.Data.SHA1),
		MD5Checksum:    strPtr(payload.Data.MD5),
		Metadata: models.JSONMap{
			"artifactory_repo": payload.Data.RepoPath.RepoKey,
			"created_by":       payload.Data.CreatedBy,
			"modified_by":      payload.Data.ModifiedBy,
		},
	}

	if err := h.releaseRepo.CreateAsset(ctx, asset); err != nil {
		h.logger.Warn("Failed to create Artifactory asset record", map[string]interface{}{
			"release_id": release.ID,
			"asset_name": asset.Name,
			"error":      err.Error(),
		})
	}

	return release, nil
}

// extractTenantID extracts the tenant ID from the event auth context
func (h *ArtifactoryWebhookHandler) extractTenantID(event queue.Event) (uuid.UUID, error) {
	// Try to get tenant ID from auth context
	if event.AuthContext != nil {
		if event.AuthContext.TenantID != "" {
			return uuid.Parse(event.AuthContext.TenantID)
		}
	}

	// Default to system tenant if not specified
	return uuid.Parse("00000000-0000-0000-0000-000000000001")
}
