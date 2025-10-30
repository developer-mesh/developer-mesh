package updater

import (
	"time"

	"github.com/google/go-github/v74/github"
)

// UpdateChannel represents the release channel to follow
type UpdateChannel string

const (
	// StableChannel tracks only stable releases (no prereleases)
	StableChannel UpdateChannel = "stable"
	// BetaChannel tracks beta/rc releases
	BetaChannel UpdateChannel = "beta"
	// LatestChannel tracks all releases including prereleases
	LatestChannel UpdateChannel = "latest"
)

// Release represents a parsed GitHub release with update-relevant information
type Release struct {
	Version     *Version
	TagName     string
	PublishedAt time.Time
	Assets      []ReleaseAsset
	Body        string
	HTMLURL     string
	Prerelease  bool
	Draft       bool
}

// ReleaseAsset represents a downloadable asset from a release
type ReleaseAsset struct {
	ID          int64
	Name        string
	Size        int
	ContentType string
	DownloadURL string
}

// UpdateCheckResult contains the result of an update check
type UpdateCheckResult struct {
	UpdateAvailable bool
	CurrentVersion  *Version
	LatestVersion   *Version
	Release         *Release
	Changelog       string
	CheckedAt       time.Time
}

// DownloadResult contains the result of a release download
type DownloadResult struct {
	Data        []byte
	AssetName   string
	Checksum    string
	Size        int
	DownloadAt  time.Time
	DownloadURL string
}

// UpdateStatus represents the state of an update operation
type UpdateStatus string

const (
	// UpdateStatusChecking is checking for updates
	UpdateStatusChecking UpdateStatus = "checking"
	// UpdateStatusAvailable means an update is available
	UpdateStatusAvailable UpdateStatus = "available"
	// UpdateStatusDownloading is downloading the update
	UpdateStatusDownloading UpdateStatus = "downloading"
	// UpdateStatusVerifying is verifying the downloaded file
	UpdateStatusVerifying UpdateStatus = "verifying"
	// UpdateStatusReady means update is ready to apply
	UpdateStatusReady UpdateStatus = "ready"
	// UpdateStatusApplying is applying the update
	UpdateStatusApplying UpdateStatus = "applying"
	// UpdateStatusComplete means update completed successfully
	UpdateStatusComplete UpdateStatus = "complete"
	// UpdateStatusFailed means update failed
	UpdateStatusFailed UpdateStatus = "failed"
	// UpdateStatusSkipped means update was skipped
	UpdateStatusSkipped UpdateStatus = "skipped"
)

// UpdateEvent represents an event during the update process
type UpdateEvent struct {
	Status    UpdateStatus
	Message   string
	Progress  float64 // 0.0 to 1.0
	Error     error
	Timestamp time.Time
}

// fromGitHubRelease converts a GitHub release to our internal Release type
func fromGitHubRelease(ghRelease *github.RepositoryRelease) (*Release, error) {
	version, err := ParseVersion(ghRelease.GetTagName())
	if err != nil {
		return nil, err
	}

	release := &Release{
		Version:     version,
		TagName:     ghRelease.GetTagName(),
		PublishedAt: ghRelease.GetPublishedAt().Time,
		Body:        ghRelease.GetBody(),
		HTMLURL:     ghRelease.GetHTMLURL(),
		Prerelease:  ghRelease.GetPrerelease(),
		Draft:       ghRelease.GetDraft(),
		Assets:      make([]ReleaseAsset, 0, len(ghRelease.Assets)),
	}

	for _, asset := range ghRelease.Assets {
		release.Assets = append(release.Assets, ReleaseAsset{
			ID:          asset.GetID(),
			Name:        asset.GetName(),
			Size:        asset.GetSize(),
			ContentType: asset.GetContentType(),
			DownloadURL: asset.GetBrowserDownloadURL(),
		})
	}

	return release, nil
}

// IsCompatibleWith returns true if this release is compatible with the given channel
func (r *Release) IsCompatibleWith(channel UpdateChannel) bool {
	switch channel {
	case StableChannel:
		return !r.Prerelease && !r.Draft
	case BetaChannel:
		return !r.Draft // Includes both stable and prerelease
	case LatestChannel:
		return !r.Draft // All non-draft releases
	default:
		return false
	}
}

// FindAsset finds a release asset by name
func (r *Release) FindAsset(name string) *ReleaseAsset {
	for i := range r.Assets {
		if r.Assets[i].Name == name {
			return &r.Assets[i]
		}
	}
	return nil
}
