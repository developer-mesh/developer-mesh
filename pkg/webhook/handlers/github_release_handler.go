package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/google/uuid"
)

// GitHubReleaseHandler processes GitHub release webhook events
type GitHubReleaseHandler struct {
	releaseRepo repository.PackageReleaseRepository
	queueClient *queue.Client
	logger      observability.Logger
	metrics     observability.MetricsClient
}

// NewGitHubReleaseHandler creates a new GitHub release handler
func NewGitHubReleaseHandler(
	releaseRepo repository.PackageReleaseRepository,
	queueClient *queue.Client,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *GitHubReleaseHandler {
	if logger == nil {
		logger = observability.NewLogger("github-release-handler")
	}
	if metrics == nil {
		metrics = observability.NewMetricsClient()
	}

	return &GitHubReleaseHandler{
		releaseRepo: releaseRepo,
		queueClient: queueClient,
		logger:      logger,
		metrics:     metrics,
	}
}

// GitHubReleasePayload represents the GitHub release webhook payload
type GitHubReleasePayload struct {
	Action     string           `json:"action"`
	Release    GitHubRelease    `json:"release"`
	Repository GitHubRepository `json:"repository"`
	Sender     GitHubUser       `json:"sender"`
}

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	ID          int64         `json:"id"`
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	CreatedAt   string        `json:"created_at"`
	PublishedAt string        `json:"published_at"`
	Author      GitHubUser    `json:"author"`
	Assets      []GitHubAsset `json:"assets"`
	TarballURL  string        `json:"tarball_url"`
	ZipballURL  string        `json:"zipball_url"`
}

// GitHubAsset represents a release asset
type GitHubAsset struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	ContentType string     `json:"content_type"`
	Size        int64      `json:"size"`
	DownloadURL string     `json:"browser_download_url"`
	State       string     `json:"state"`
	Uploader    GitHubUser `json:"uploader"`
}

// GitHubRepository represents a GitHub repository
type GitHubRepository struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	FullName    string        `json:"full_name"`
	Description string        `json:"description"`
	Homepage    string        `json:"homepage"`
	License     GitHubLicense `json:"license"`
	Language    string        `json:"language"`
}

// GitHubLicense represents a repository license
type GitHubLicense struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	SPDX string `json:"spdx_id"`
}

// GitHubUser represents a GitHub user
type GitHubUser struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
}

// Handle processes a GitHub release webhook event
func (h *GitHubReleaseHandler) Handle(ctx context.Context, event queue.Event) error {
	start := time.Now()
	defer func() {
		h.metrics.RecordHistogram("webhook_github_release_duration_seconds", time.Since(start).Seconds(), nil)
	}()

	// Parse the payload
	var payload GitHubReleasePayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		h.logger.Error("Failed to parse GitHub release payload", map[string]interface{}{
			"event_id": event.EventID,
			"error":    err.Error(),
		})
		h.metrics.IncrementCounter("webhook_github_release_parse_errors_total", 1)
		return fmt.Errorf("failed to parse GitHub release payload: %w", err)
	}

	// Only process published releases (not drafts or prereleases being saved)
	if payload.Action != "published" && payload.Action != "released" {
		h.logger.Debug("Skipping non-published release event", map[string]interface{}{
			"event_id": event.EventID,
			"action":   payload.Action,
		})
		return nil
	}

	// Skip draft releases
	if payload.Release.Draft {
		h.logger.Debug("Skipping draft release", map[string]interface{}{
			"event_id": event.EventID,
			"tag_name": payload.Release.TagName,
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

	h.logger.Info("Processing GitHub release", map[string]interface{}{
		"event_id":   event.EventID,
		"tenant_id":  tenantID,
		"repository": payload.Repository.FullName,
		"tag_name":   payload.Release.TagName,
		"action":     payload.Action,
	})

	// Parse version information
	version := h.parseVersion(payload.Release.TagName)

	// Determine package name (use repo name if not specified in release)
	packageName := payload.Repository.Name
	if payload.Release.Name != "" && payload.Release.Name != payload.Release.TagName {
		packageName = payload.Release.Name
	}

	// Parse release notes
	releaseNotes := h.parseReleaseNotes(payload.Release.Body)

	// Parse published timestamp
	publishedAt, err := time.Parse(time.RFC3339, payload.Release.PublishedAt)
	if err != nil {
		h.logger.Warn("Failed to parse published timestamp, using current time", map[string]interface{}{
			"published_at": payload.Release.PublishedAt,
			"error":        err.Error(),
		})
		publishedAt = time.Now()
	}

	// Create package release record
	release := &models.PackageRelease{
		TenantID:         tenantID,
		RepositoryName:   payload.Repository.FullName,
		PackageName:      packageName,
		Version:          payload.Release.TagName,
		VersionMajor:     intPtr(version.Major),
		VersionMinor:     intPtr(version.Minor),
		VersionPatch:     intPtr(version.Patch),
		Prerelease:       version.Prerelease,
		IsBreakingChange: releaseNotes.HasBreakingChange,
		ReleaseNotes:     strPtr(payload.Release.Body),
		Changelog:        nil, // TODO: Extract from release notes
		PublishedAt:      publishedAt,
		AuthorLogin:      strPtr(payload.Release.Author.Login),
		GitHubReleaseID:  &payload.Release.ID,
		PackageType:      "generic", // TODO: Detect from repository
		Description:      strPtr(payload.Repository.Description),
		Homepage:         strPtr(payload.Repository.Homepage),
		Metadata: models.JSONMap{
			"github_url":    fmt.Sprintf("https://github.com/%s/releases/tag/%s", payload.Repository.FullName, payload.Release.TagName),
			"is_prerelease": payload.Release.Prerelease,
			"language":      payload.Repository.Language,
			"tarball_url":   payload.Release.TarballURL,
			"zipball_url":   payload.Release.ZipballURL,
		},
	}

	if payload.Repository.License.Name != "" {
		release.License = strPtr(payload.Repository.License.Name)
	}

	// Store the release
	if err := h.releaseRepo.Create(ctx, release); err != nil {
		h.logger.Error("Failed to store package release", map[string]interface{}{
			"event_id":   event.EventID,
			"repository": payload.Repository.FullName,
			"version":    payload.Release.TagName,
			"error":      err.Error(),
		})
		h.metrics.IncrementCounter("webhook_github_release_storage_errors_total", 1)
		return fmt.Errorf("failed to store package release: %w", err)
	}

	h.logger.Info("Stored package release", map[string]interface{}{
		"release_id": release.ID,
		"package":    release.PackageName,
		"version":    release.Version,
		"repository": release.RepositoryName,
	})

	// Store assets
	for _, asset := range payload.Release.Assets {
		if err := h.storeAsset(ctx, release.ID, asset); err != nil {
			h.logger.Warn("Failed to store asset", map[string]interface{}{
				"release_id": release.ID,
				"asset_name": asset.Name,
				"error":      err.Error(),
			})
			// Continue processing other assets
		}
	}

	// Parse and store breaking changes as API changes
	if releaseNotes.HasBreakingChange {
		for _, breakingChange := range releaseNotes.BreakingChanges {
			apiChange := &models.PackageAPIChange{
				ReleaseID:      release.ID,
				ChangeType:     models.APIChangeModified,
				APISignature:   breakingChange,
				Description:    strPtr(breakingChange),
				Breaking:       true,
				MigrationGuide: releaseNotes.MigrationGuide,
			}
			if err := h.releaseRepo.CreateAPIChange(ctx, apiChange); err != nil {
				h.logger.Warn("Failed to store API change", map[string]interface{}{
					"release_id": release.ID,
					"error":      err.Error(),
				})
			}
		}
	}

	h.metrics.IncrementCounterWithLabels("webhook_github_release_processed_total", 1, map[string]string{
		"repository": payload.Repository.FullName,
		"tenant_id":  tenantID.String(),
	})

	// Publish enrichment event for Phase 3 processing (if queue client available)
	if h.queueClient != nil {
		if err := h.publishEnrichmentEvent(ctx, release.ID, tenantID, event.AuthContext); err != nil {
			// Log warning but don't fail the entire operation
			h.logger.Warn("Failed to publish enrichment event", map[string]interface{}{
				"release_id": release.ID,
				"error":      err.Error(),
			})
		}
	}

	return nil
}

// publishEnrichmentEvent publishes an event to trigger package enrichment
func (h *GitHubReleaseHandler) publishEnrichmentEvent(ctx context.Context, releaseID uuid.UUID, tenantID uuid.UUID, authContext *queue.EventAuthContext) error {
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
			"source":     "github_release_handler",
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

// parseVersion extracts semantic version information from a tag
func (h *GitHubReleaseHandler) parseVersion(tag string) *models.VersionInfo {
	// Remove common prefixes
	tag = strings.TrimPrefix(tag, "v")
	tag = strings.TrimPrefix(tag, "version-")
	tag = strings.TrimPrefix(tag, "release-")

	// Parse semantic version using regex
	semverRegex := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-(.+))?`)
	matches := semverRegex.FindStringSubmatch(tag)

	version := &models.VersionInfo{
		Raw: tag,
	}

	if len(matches) >= 4 {
		version.Major = parseInt(matches[1])
		version.Minor = parseInt(matches[2])
		version.Patch = parseInt(matches[3])

		if len(matches) > 4 && matches[4] != "" {
			version.Prerelease = &matches[4]
		}
	}

	return version
}

// parseReleaseNotes extracts structured information from release notes
func (h *GitHubReleaseHandler) parseReleaseNotes(body string) *models.ParsedReleaseNotes {
	notes := &models.ParsedReleaseNotes{
		RawNotes:          body,
		HasBreakingChange: false,
		BreakingChanges:   []string{},
		NewFeatures:       []string{},
		BugFixes:          []string{},
	}

	if body == "" {
		return notes
	}

	// Convert to lowercase for case-insensitive matching
	lowerBody := strings.ToLower(body)

	// Check for breaking changes
	if strings.Contains(lowerBody, "breaking") || strings.Contains(lowerBody, "breaking change") {
		notes.HasBreakingChange = true
		notes.BreakingChanges = h.extractSection(body, []string{"breaking changes", "breaking"})
	}

	// Extract new features
	notes.NewFeatures = h.extractSection(body, []string{"features", "new features", "what's new", "added"})

	// Extract bug fixes
	notes.BugFixes = h.extractSection(body, []string{"fixes", "bug fixes", "fixed", "bugfixes"})

	// Extract migration guide
	migrationSections := h.extractSection(body, []string{"migration", "upgrade guide", "upgrading", "migration guide"})
	if len(migrationSections) > 0 {
		migration := strings.Join(migrationSections, "\n")
		notes.MigrationGuide = &migration
	}

	return notes
}

// extractSection extracts bullet points or content under specific section headers
func (h *GitHubReleaseHandler) extractSection(body string, headers []string) []string {
	var items []string

	lines := strings.Split(body, "\n")
	inSection := false
	sectionDepth := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		// Check if we're entering a target section
		for _, header := range headers {
			if strings.HasPrefix(lower, "#") && strings.Contains(lower, strings.ToLower(header)) {
				inSection = true
				// Count the number of # to determine depth
				sectionDepth = strings.Count(trimmed, "#")
				break
			}
		}

		if inSection {
			// Check if we've entered a new section at the same or higher level
			if strings.HasPrefix(trimmed, "#") {
				currentDepth := strings.Count(trimmed, "#")
				if currentDepth <= sectionDepth && !strings.Contains(lower, strings.ToLower(headers[0])) {
					// We've entered a different section
					break
				}
			}

			// Extract bullet points or numbered items
			if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") || regexp.MustCompile(`^\d+\.`).MatchString(trimmed) {
				// Remove bullet/number and trim
				item := regexp.MustCompile(`^[-*]\s+|^\d+\.\s+`).ReplaceAllString(trimmed, "")
				if item != "" {
					items = append(items, item)
				}
			}
		}
	}

	return items
}

// storeAsset stores a release asset
func (h *GitHubReleaseHandler) storeAsset(ctx context.Context, releaseID uuid.UUID, asset GitHubAsset) error {
	packageAsset := &models.PackageAsset{
		ReleaseID:   releaseID,
		Name:        asset.Name,
		ContentType: strPtr(asset.ContentType),
		SizeBytes:   &asset.Size,
		DownloadURL: strPtr(asset.DownloadURL),
		Metadata: models.JSONMap{
			"github_asset_id": asset.ID,
			"state":           asset.State,
			"uploader":        asset.Uploader.Login,
		},
	}

	return h.releaseRepo.CreateAsset(ctx, packageAsset)
}

// extractTenantID extracts the tenant ID from the event auth context
func (h *GitHubReleaseHandler) extractTenantID(event queue.Event) (uuid.UUID, error) {
	// Try to get tenant ID from auth context
	if event.AuthContext != nil {
		if event.AuthContext.TenantID != "" {
			return uuid.Parse(event.AuthContext.TenantID)
		}
	}

	// Default to a system tenant if not specified (for backward compatibility)
	// In production, this should be properly configured
	return uuid.Parse("00000000-0000-0000-0000-000000000001")
}

// Helper functions
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func parseInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

func intPtr(i int) *int {
	return &i
}
