package github

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/go-github/v74/github"
)

// ReleaseDownloader handles GitHub release operations for internal use.
// This is separate from the MCP tool handlers and designed for service-level operations
// like the auto-updater.
type ReleaseDownloader struct {
	client *github.Client
	logger observability.Logger
}

// NewReleaseDownloader creates a new ReleaseDownloader with the provided GitHub client.
func NewReleaseDownloader(client *github.Client, logger observability.Logger) *ReleaseDownloader {
	if logger == nil {
		logger = observability.NewNoopLogger()
	}
	return &ReleaseDownloader{
		client: client,
		logger: logger,
	}
}

// GetLatestRelease retrieves the latest published release for a repository.
// It only returns published, non-draft, non-prerelease versions.
func (r *ReleaseDownloader) GetLatestRelease(ctx context.Context, owner, repo string) (*github.RepositoryRelease, error) {
	r.logger.Debug("Fetching latest release", map[string]interface{}{
		"owner": owner,
		"repo":  repo,
	})

	release, resp, err := r.client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		r.logger.Error("Failed to get latest release", map[string]interface{}{
			"error": err.Error(),
			"owner": owner,
			"repo":  repo,
		})
		return nil, fmt.Errorf("failed to get latest release for %s/%s: %w", owner, repo, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d for latest release", resp.StatusCode)
	}

	r.logger.Info("Retrieved latest release", map[string]interface{}{
		"owner":   owner,
		"repo":    repo,
		"version": release.GetTagName(),
	})

	return release, nil
}

// GetReleaseByTag retrieves a specific release by its tag name.
func (r *ReleaseDownloader) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*github.RepositoryRelease, error) {
	r.logger.Debug("Fetching release by tag", map[string]interface{}{
		"owner": owner,
		"repo":  repo,
		"tag":   tag,
	})

	release, resp, err := r.client.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		r.logger.Error("Failed to get release by tag", map[string]interface{}{
			"error": err.Error(),
			"owner": owner,
			"repo":  repo,
			"tag":   tag,
		})
		return nil, fmt.Errorf("failed to get release %s for %s/%s: %w", tag, owner, repo, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d for release %s", resp.StatusCode, tag)
	}

	r.logger.Info("Retrieved release by tag", map[string]interface{}{
		"owner": owner,
		"repo":  repo,
		"tag":   tag,
	})

	return release, nil
}

// DownloadAsset downloads a release asset by its ID.
// Returns the asset data as bytes.
func (r *ReleaseDownloader) DownloadAsset(ctx context.Context, owner, repo string, assetID int64) ([]byte, string, error) {
	r.logger.Debug("Downloading release asset", map[string]interface{}{
		"owner":    owner,
		"repo":     repo,
		"asset_id": assetID,
	})

	// First, get asset metadata to retrieve the name
	asset, resp, err := r.client.Repositories.GetReleaseAsset(ctx, owner, repo, assetID)
	if err != nil {
		r.logger.Error("Failed to get asset metadata", map[string]interface{}{
			"error":    err.Error(),
			"owner":    owner,
			"repo":     repo,
			"asset_id": assetID,
		})
		return nil, "", fmt.Errorf("failed to get asset metadata: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected status code %d for asset metadata", resp.StatusCode)
	}

	assetName := asset.GetName()

	// Download the asset content
	rc, redirectURL, err := r.client.Repositories.DownloadReleaseAsset(ctx, owner, repo, assetID, http.DefaultClient)
	if err != nil {
		r.logger.Error("Failed to download asset", map[string]interface{}{
			"error":      err.Error(),
			"owner":      owner,
			"repo":       repo,
			"asset_id":   assetID,
			"asset_name": assetName,
		})
		return nil, "", fmt.Errorf("failed to download asset %s: %w", assetName, err)
	}

	// If we got a redirect URL, download from there
	if redirectURL != "" {
		r.logger.Debug("Following redirect for asset download", map[string]interface{}{
			"redirect_url": redirectURL,
			"asset_name":   assetName,
		})

		httpResp, err := http.DefaultClient.Get(redirectURL)
		if err != nil {
			return nil, "", fmt.Errorf("failed to download from redirect URL: %w", err)
		}
		defer func() {
			if closeErr := httpResp.Body.Close(); closeErr != nil {
				r.logger.Warn("Failed to close response body", map[string]interface{}{
					"error": closeErr.Error(),
				})
			}
		}()

		data, err := io.ReadAll(httpResp.Body)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read asset data from redirect: %w", err)
		}

		r.logger.Info("Downloaded asset via redirect", map[string]interface{}{
			"asset_name": assetName,
			"size_bytes": len(data),
		})

		return data, assetName, nil
	}

	// Otherwise read from the reader
	if rc != nil {
		defer func() {
			if closeErr := rc.Close(); closeErr != nil {
				r.logger.Warn("Failed to close asset reader", map[string]interface{}{
					"error": closeErr.Error(),
				})
			}
		}()

		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read asset data: %w", err)
		}

		r.logger.Info("Downloaded asset", map[string]interface{}{
			"asset_name": assetName,
			"size_bytes": len(data),
		})

		return data, assetName, nil
	}

	return nil, "", fmt.Errorf("no data received for asset %s", assetName)
}

// FindAssetByName finds a release asset by name within a release.
// Returns the asset ID if found, or an error if not found.
func (r *ReleaseDownloader) FindAssetByName(release *github.RepositoryRelease, assetName string) (int64, error) {
	for _, asset := range release.Assets {
		if asset.GetName() == assetName {
			return asset.GetID(), nil
		}
	}
	return 0, fmt.Errorf("asset %s not found in release %s", assetName, release.GetTagName())
}

// ListReleaseAssets returns a list of all assets for a release.
func (r *ReleaseDownloader) ListReleaseAssets(release *github.RepositoryRelease) []ReleaseAssetInfo {
	assets := make([]ReleaseAssetInfo, 0, len(release.Assets))
	for _, asset := range release.Assets {
		assets = append(assets, ReleaseAssetInfo{
			ID:          asset.GetID(),
			Name:        asset.GetName(),
			Size:        asset.GetSize(),
			ContentType: asset.GetContentType(),
			DownloadURL: asset.GetBrowserDownloadURL(),
		})
	}
	return assets
}

// ReleaseAssetInfo contains metadata about a release asset.
type ReleaseAssetInfo struct {
	ID          int64
	Name        string
	Size        int
	ContentType string
	DownloadURL string
}
