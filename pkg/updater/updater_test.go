package updater

import (
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/go-github/v74/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid configuration",
			config:      validTestConfig(),
			expectError: false,
		},
		{
			name: "missing owner",
			config: &Config{
				Repo:             "developer-mesh",
				CurrentVersion:   "0.0.9",
				Channel:          StableChannel,
				Platform:         "linux",
				Arch:             "amd64",
				AssetNamePattern: "edge-mcp-{{.OS}}-{{.Arch}}{{.Ext}}",
				CheckInterval:    24 * time.Hour,
				DownloadTimeout:  5 * time.Minute,
				ApplyTimeout:     30 * time.Second,
			},
			expectError: true,
			errorMsg:    "owner cannot be empty",
		},
		{
			name: "invalid version",
			config: &Config{
				Owner:            "developer-mesh",
				Repo:             "developer-mesh",
				CurrentVersion:   "invalid.version.format",
				Channel:          StableChannel,
				Platform:         "linux",
				Arch:             "amd64",
				AssetNamePattern: "edge-mcp-{{.OS}}-{{.Arch}}{{.Ext}}",
				CheckInterval:    24 * time.Hour,
				DownloadTimeout:  5 * time.Minute,
				ApplyTimeout:     30 * time.Second,
			},
			expectError: true,
			errorMsg:    "invalid version format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := observability.NewNoopLogger()
			githubClient := github.NewClient(nil)

			updater, err := New(tt.config, githubClient, logger, nil)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, updater)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, updater)
				assert.Equal(t, tt.config.CurrentVersion, updater.currentVersion.String())
			}
		})
	}
}

func TestCheckForUpdate_WithNewerVersion(t *testing.T) {
	// This is an integration-style test that would require mocking GitHub API
	// For now, we'll test the logic flow
	t.Skip("Requires GitHub API mocking - implement in integration tests")
}

func TestRelease_IsCompatibleWith(t *testing.T) {
	tests := []struct {
		name       string
		release    *Release
		channel    UpdateChannel
		compatible bool
	}{
		{
			name: "stable release with stable channel",
			release: &Release{
				Prerelease: false,
				Draft:      false,
			},
			channel:    StableChannel,
			compatible: true,
		},
		{
			name: "prerelease with stable channel",
			release: &Release{
				Prerelease: true,
				Draft:      false,
			},
			channel:    StableChannel,
			compatible: false,
		},
		{
			name: "prerelease with beta channel",
			release: &Release{
				Prerelease: true,
				Draft:      false,
			},
			channel:    BetaChannel,
			compatible: true,
		},
		{
			name: "draft release with any channel",
			release: &Release{
				Prerelease: false,
				Draft:      true,
			},
			channel:    LatestChannel,
			compatible: false,
		},
		{
			name: "stable release with latest channel",
			release: &Release{
				Prerelease: false,
				Draft:      false,
			},
			channel:    LatestChannel,
			compatible: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compatible := tt.release.IsCompatibleWith(tt.channel)
			assert.Equal(t, tt.compatible, compatible)
		})
	}
}

func TestRelease_FindAsset(t *testing.T) {
	release := &Release{
		Assets: []ReleaseAsset{
			{Name: "edge-mcp-linux-amd64", ID: 1},
			{Name: "edge-mcp-darwin-amd64", ID: 2},
			{Name: "edge-mcp-windows-amd64.exe", ID: 3},
		},
	}

	tests := []struct {
		name       string
		assetName  string
		found      bool
		expectedID int64
	}{
		{
			name:       "existing linux asset",
			assetName:  "edge-mcp-linux-amd64",
			found:      true,
			expectedID: 1,
		},
		{
			name:       "existing windows asset",
			assetName:  "edge-mcp-windows-amd64.exe",
			found:      true,
			expectedID: 3,
		},
		{
			name:      "non-existent asset",
			assetName: "edge-mcp-freebsd-amd64",
			found:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset := release.FindAsset(tt.assetName)
			if tt.found {
				require.NotNil(t, asset)
				assert.Equal(t, tt.expectedID, asset.ID)
				assert.Equal(t, tt.assetName, asset.Name)
			} else {
				assert.Nil(t, asset)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorField  string
	}{
		{
			name:        "valid config",
			config:      validTestConfig(),
			expectError: false,
		},
		{
			name: "empty owner",
			config: &Config{
				Repo:             "developer-mesh",
				CurrentVersion:   "0.0.9",
				Platform:         "linux",
				Arch:             "amd64",
				AssetNamePattern: "edge-mcp-{{.OS}}-{{.Arch}}{{.Ext}}",
				CheckInterval:    24 * time.Hour,
				DownloadTimeout:  5 * time.Minute,
				ApplyTimeout:     30 * time.Second,
			},
			expectError: true,
			errorField:  "Owner",
		},
		{
			name: "invalid version format",
			config: &Config{
				Owner:            "developer-mesh",
				Repo:             "developer-mesh",
				CurrentVersion:   "not-a-version",
				Platform:         "linux",
				Arch:             "amd64",
				AssetNamePattern: "edge-mcp-{{.OS}}-{{.Arch}}{{.Ext}}",
				CheckInterval:    24 * time.Hour,
				DownloadTimeout:  5 * time.Minute,
				ApplyTimeout:     30 * time.Second,
			},
			expectError: true,
			errorField:  "CurrentVersion",
		},
		{
			name: "negative check interval",
			config: &Config{
				Owner:            "developer-mesh",
				Repo:             "developer-mesh",
				CurrentVersion:   "0.0.9",
				Platform:         "linux",
				Arch:             "amd64",
				AssetNamePattern: "edge-mcp-{{.OS}}-{{.Arch}}{{.Ext}}",
				CheckInterval:    -1 * time.Hour,
				DownloadTimeout:  5 * time.Minute,
				ApplyTimeout:     30 * time.Second,
			},
			expectError: true,
			errorField:  "CheckInterval",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				require.Error(t, err)
				var configErr ErrInvalidConfig
				require.ErrorAs(t, err, &configErr)
				assert.Equal(t, tt.errorField, configErr.Field)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_AssetName(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected string
	}{
		{
			name: "linux amd64",
			config: &Config{
				Platform:         "linux",
				Arch:             "amd64",
				AssetNamePattern: "edge-mcp-{{.OS}}-{{.Arch}}{{.Ext}}",
			},
			expected: "edge-mcp-linux-amd64",
		},
		{
			name: "windows amd64",
			config: &Config{
				Platform:         "windows",
				Arch:             "amd64",
				AssetNamePattern: "edge-mcp-{{.OS}}-{{.Arch}}{{.Ext}}",
			},
			expected: "edge-mcp-windows-amd64.exe",
		},
		{
			name: "darwin arm64",
			config: &Config{
				Platform:         "darwin",
				Arch:             "arm64",
				AssetNamePattern: "edge-mcp-{{.OS}}-{{.Arch}}{{.Ext}}",
			},
			expected: "edge-mcp-darwin-arm64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assetName, err := tt.config.AssetName()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, assetName)
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, "developer-mesh", config.Owner)
	assert.Equal(t, "developer-mesh", config.Repo)
	assert.Equal(t, StableChannel, config.Channel)
	assert.Equal(t, 24*time.Hour, config.CheckInterval)
	assert.True(t, config.AutoDownload)
	assert.False(t, config.AutoApply) // Safety: manual approval by default
	assert.True(t, config.VerifyChecksum)
	assert.True(t, config.RollbackEnabled)
	assert.NotEmpty(t, config.Platform)
	assert.NotEmpty(t, config.Arch)
}

func TestUpdater_GetLastCheckTime(t *testing.T) {
	config := validTestConfig()
	logger := observability.NewNoopLogger()
	githubClient := github.NewClient(nil)

	updater, err := New(config, githubClient, logger, nil)
	require.NoError(t, err)

	// Initially should be zero
	lastCheck := updater.GetLastCheckTime()
	assert.True(t, lastCheck.IsZero())

	// After manual update (would normally happen in CheckForUpdate)
	now := time.Now()
	updater.lastCheckMu.Lock()
	updater.lastCheck = now
	updater.lastCheckMu.Unlock()

	lastCheck = updater.GetLastCheckTime()
	assert.Equal(t, now.Unix(), lastCheck.Unix())
}

func TestUpdater_Close(t *testing.T) {
	config := validTestConfig()
	logger := observability.NewNoopLogger()
	githubClient := github.NewClient(nil)

	updater, err := New(config, githubClient, logger, nil)
	require.NoError(t, err)

	err = updater.Close()
	assert.NoError(t, err)
}

// Helper functions

func validTestConfig() *Config {
	return &Config{
		Owner:               "developer-mesh",
		Repo:                "developer-mesh",
		CurrentVersion:      "0.0.9",
		Channel:             StableChannel,
		Platform:            "linux",
		Arch:                "amd64",
		AssetNamePattern:    "edge-mcp-{{.OS}}-{{.Arch}}{{.Ext}}",
		CheckInterval:       24 * time.Hour,
		ChecksumAssetSuffix: ".sha256",
		VerifyChecksum:      true,
		MaxRetries:          3,
		RetryDelay:          2 * time.Second,
		RetryMaxDelay:       30 * time.Second,
		RetryMultiplier:     2.0,
		DownloadTimeout:     5 * time.Minute,
		ApplyTimeout:        30 * time.Second,
		RollbackEnabled:     true,
		Enabled:             true,
	}
}
