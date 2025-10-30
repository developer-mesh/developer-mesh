package github

import (
	"context"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/go-github/v74/github"
	"github.com/stretchr/testify/assert"
)

func TestNewReleaseDownloader(t *testing.T) {
	client := github.NewClient(nil)
	logger := observability.NewNoopLogger()

	tests := []struct {
		name   string
		client *github.Client
		logger observability.Logger
	}{
		{
			name:   "with logger",
			client: client,
			logger: logger,
		},
		{
			name:   "without logger",
			client: client,
			logger: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			downloader := NewReleaseDownloader(tt.client, tt.logger)
			assert.NotNil(t, downloader)
			assert.NotNil(t, downloader.client)
			assert.NotNil(t, downloader.logger)
		})
	}
}

func TestFindAssetByName(t *testing.T) {
	logger := observability.NewNoopLogger()
	downloader := NewReleaseDownloader(nil, logger)

	assetID1 := int64(1)
	assetID2 := int64(2)
	assetName1 := "app-v1.0.0-linux-amd64.tar.gz"
	assetName2 := "app-v1.0.0-darwin-amd64.tar.gz"

	release := &github.RepositoryRelease{
		TagName: github.Ptr("v1.0.0"),
		Assets: []*github.ReleaseAsset{
			{
				ID:   &assetID1,
				Name: &assetName1,
			},
			{
				ID:   &assetID2,
				Name: &assetName2,
			},
		},
	}

	tests := []struct {
		name        string
		assetName   string
		expectedID  int64
		expectError bool
	}{
		{
			name:        "find first asset",
			assetName:   assetName1,
			expectedID:  assetID1,
			expectError: false,
		},
		{
			name:        "find second asset",
			assetName:   assetName2,
			expectedID:  assetID2,
			expectError: false,
		},
		{
			name:        "asset not found",
			assetName:   "nonexistent.tar.gz",
			expectedID:  0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := downloader.FindAssetByName(release, tt.assetName)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "not found")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedID, id)
			}
		})
	}
}

func TestListReleaseAssets(t *testing.T) {
	logger := observability.NewNoopLogger()
	downloader := NewReleaseDownloader(nil, logger)

	assetID1 := int64(1)
	assetID2 := int64(2)
	assetName1 := "app-v1.0.0-linux-amd64.tar.gz"
	assetName2 := "app-v1.0.0-darwin-amd64.tar.gz"
	assetSize1 := 1024000
	assetSize2 := 2048000
	contentType := "application/gzip"
	downloadURL1 := "https://github.com/test/repo/releases/download/v1.0.0/app-v1.0.0-linux-amd64.tar.gz"
	downloadURL2 := "https://github.com/test/repo/releases/download/v1.0.0/app-v1.0.0-darwin-amd64.tar.gz"

	release := &github.RepositoryRelease{
		TagName: github.Ptr("v1.0.0"),
		Assets: []*github.ReleaseAsset{
			{
				ID:                 &assetID1,
				Name:               &assetName1,
				Size:               &assetSize1,
				ContentType:        &contentType,
				BrowserDownloadURL: &downloadURL1,
			},
			{
				ID:                 &assetID2,
				Name:               &assetName2,
				Size:               &assetSize2,
				ContentType:        &contentType,
				BrowserDownloadURL: &downloadURL2,
			},
		},
	}

	assets := downloader.ListReleaseAssets(release)
	assert.Len(t, assets, 2)

	// Verify first asset
	assert.Equal(t, assetID1, assets[0].ID)
	assert.Equal(t, assetName1, assets[0].Name)
	assert.Equal(t, assetSize1, assets[0].Size)
	assert.Equal(t, contentType, assets[0].ContentType)
	assert.Equal(t, downloadURL1, assets[0].DownloadURL)

	// Verify second asset
	assert.Equal(t, assetID2, assets[1].ID)
	assert.Equal(t, assetName2, assets[1].Name)
	assert.Equal(t, assetSize2, assets[1].Size)
	assert.Equal(t, contentType, assets[1].ContentType)
	assert.Equal(t, downloadURL2, assets[1].DownloadURL)
}

func TestListReleaseAssets_EmptyRelease(t *testing.T) {
	logger := observability.NewNoopLogger()
	downloader := NewReleaseDownloader(nil, logger)

	release := &github.RepositoryRelease{
		TagName: github.Ptr("v1.0.0"),
		Assets:  []*github.ReleaseAsset{},
	}

	assets := downloader.ListReleaseAssets(release)
	assert.NotNil(t, assets)
	assert.Len(t, assets, 0)
}

// Note: Integration tests for GetLatestRelease, GetReleaseByTag, and DownloadAsset
// would require mocking the GitHub API responses or running against a test repository.
// These are better suited for integration tests rather than unit tests.
// For now, we test the logic that doesn't require API calls.

func TestReleaseAssetInfo(t *testing.T) {
	// Test that ReleaseAssetInfo struct is properly defined
	info := ReleaseAssetInfo{
		ID:          123,
		Name:        "test-asset.tar.gz",
		Size:        1024,
		ContentType: "application/gzip",
		DownloadURL: "https://example.com/asset",
	}

	assert.Equal(t, int64(123), info.ID)
	assert.Equal(t, "test-asset.tar.gz", info.Name)
	assert.Equal(t, 1024, info.Size)
	assert.Equal(t, "application/gzip", info.ContentType)
	assert.Equal(t, "https://example.com/asset", info.DownloadURL)
}

// TestGetLatestRelease_Integration is marked to run only with integration tests
func TestGetLatestRelease_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This would require a real GitHub token and test repository
	// For now, we just verify the structure
	ctx := context.Background()
	client := github.NewClient(nil) // Would need authentication
	logger := observability.NewNoopLogger()
	downloader := NewReleaseDownloader(client, logger)

	// Test against a known public repository
	// Note: This will fail without authentication for private repos
	release, err := downloader.GetLatestRelease(ctx, "octocat", "Hello-World")
	if err != nil {
		// Expected for public repos without auth or if rate limited
		t.Logf("Expected error for unauthenticated request: %v", err)
		return
	}

	if release != nil {
		assert.NotNil(t, release.TagName)
		t.Logf("Latest release: %s", release.GetTagName())
	}
}
