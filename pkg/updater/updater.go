package updater

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/resilience"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/github"
	"github.com/developer-mesh/developer-mesh/pkg/utils"
	goGithub "github.com/google/go-github/v74/github"
)

// Updater orchestrates the auto-update process using existing infrastructure
type Updater struct {
	config *Config

	// Reuse existing components (Phase 2 principle: reuse before creating)
	githubClient   *github.ReleaseDownloader   // GitHub release operations
	circuitBreaker *resilience.CircuitBreaker  // Resilience for external calls
	logger         observability.Logger        // Structured logging
	metrics        observability.MetricsClient // Metrics collection

	// Current state
	currentVersion *Version
	lastCheck      time.Time
	lastCheckMu    sync.RWMutex

	// Background checker
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// New creates a new Updater instance using existing infrastructure
func New(config *Config, githubClient *goGithub.Client, logger observability.Logger, metrics observability.MetricsClient) (*Updater, error) {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Parse current version
	currentVersion, err := ParseVersion(config.CurrentVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse current version: %w", err)
	}

	// Create GitHub release downloader (Phase 1 component)
	releaseDownloader := github.NewReleaseDownloader(githubClient, logger)

	// Create circuit breaker for resilience (using existing pkg/resilience)
	cb := resilience.NewCircuitBreaker(
		"updater",
		config.CircuitBreaker,
		logger,
		metrics,
	)

	updater := &Updater{
		config:         config,
		githubClient:   releaseDownloader,
		circuitBreaker: cb,
		logger:         logger,
		metrics:        metrics,
		currentVersion: currentVersion,
		stopChan:       make(chan struct{}),
	}

	logger.Info("Updater initialized", map[string]interface{}{
		"current_version": currentVersion.String(),
		"channel":         config.Channel,
		"enabled":         config.Enabled,
		"repo":            fmt.Sprintf("%s/%s", config.Owner, config.Repo),
	})

	return updater, nil
}

// CheckForUpdate checks if a new update is available
// Uses circuit breaker and retry for resilience
func (u *Updater) CheckForUpdate(ctx context.Context) (*UpdateCheckResult, error) {
	u.logger.Debug("Checking for updates", map[string]interface{}{
		"current_version": u.currentVersion.String(),
		"channel":         u.config.Channel,
	})

	startTime := time.Now()

	// Use circuit breaker for external GitHub API call
	result, err := u.circuitBreaker.Execute(ctx, func() (interface{}, error) {
		// Use retry mechanism for resilience
		retryConfig := &utils.RetryConfig{
			MaxAttempts:  u.config.MaxRetries,
			InitialDelay: u.config.RetryDelay,
			MaxDelay:     u.config.RetryMaxDelay,
			Multiplier:   u.config.RetryMultiplier,
			JitterFactor: 0.1,
			RetryIf: func(err error) bool {
				// Retry on network errors and rate limits
				return utils.IsRetryableHTTPError(err)
			},
		}

		var release *goGithub.RepositoryRelease
		_, retryErr := utils.RetryWithBackoff(ctx, retryConfig, func() error {
			var err error
			release, err = u.githubClient.GetLatestRelease(ctx, u.config.Owner, u.config.Repo)
			return err
		})

		if retryErr != nil {
			return nil, retryErr
		}

		return release, nil
	})

	// Record metrics
	duration := time.Since(startTime)
	u.recordMetric("update_check_duration_seconds", duration.Seconds())

	if err != nil {
		u.recordMetric("update_check_failures_total", 1)
		u.logger.Error("Failed to check for updates", map[string]interface{}{
			"error":    err.Error(),
			"duration": duration.Seconds(),
		})
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}

	// Parse the release
	ghRelease := result.(*goGithub.RepositoryRelease)
	release, err := fromGitHubRelease(ghRelease)
	if err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	// Check if release is compatible with our channel
	if !release.IsCompatibleWith(u.config.Channel) {
		u.logger.Debug("Latest release not compatible with channel", map[string]interface{}{
			"version":    release.Version.String(),
			"prerelease": release.Prerelease,
			"channel":    u.config.Channel,
		})
		return &UpdateCheckResult{
			UpdateAvailable: false,
			CurrentVersion:  u.currentVersion,
			LatestVersion:   release.Version,
			Release:         release,
			CheckedAt:       time.Now(),
		}, nil
	}

	// Compare versions
	updateAvailable := release.Version.IsNewerThan(u.currentVersion)

	// Update last check time
	u.lastCheckMu.Lock()
	u.lastCheck = time.Now()
	u.lastCheckMu.Unlock()

	checkResult := &UpdateCheckResult{
		UpdateAvailable: updateAvailable,
		CurrentVersion:  u.currentVersion,
		LatestVersion:   release.Version,
		Release:         release,
		Changelog:       release.Body,
		CheckedAt:       time.Now(),
	}

	// Record metrics
	u.recordMetric("update_check_success_total", 1)
	if updateAvailable {
		u.recordMetric("update_available_total", 1)
		u.logger.Info("Update available", map[string]interface{}{
			"current_version": u.currentVersion.String(),
			"latest_version":  release.Version.String(),
		})
	} else {
		u.logger.Debug("No update available", map[string]interface{}{
			"current_version": u.currentVersion.String(),
			"latest_version":  release.Version.String(),
		})
	}

	return checkResult, nil
}

// DownloadUpdate downloads an update release
// Uses circuit breaker, retry, and checksum verification
func (u *Updater) DownloadUpdate(ctx context.Context, release *Release) (*DownloadResult, error) {
	// Generate expected asset name
	assetName, err := u.config.AssetName()
	if err != nil {
		return nil, fmt.Errorf("failed to generate asset name: %w", err)
	}

	u.logger.Info("Downloading update", map[string]interface{}{
		"version":    release.Version.String(),
		"asset_name": assetName,
	})

	// Find the asset in the release
	asset := release.FindAsset(assetName)
	if asset == nil {
		return nil, fmt.Errorf("asset %s not found in release %s", assetName, release.TagName)
	}

	startTime := time.Now()

	// Use circuit breaker for download
	result, err := u.circuitBreaker.Execute(ctx, func() (interface{}, error) {
		// Use retry mechanism
		retryConfig := &utils.RetryConfig{
			MaxAttempts:  u.config.MaxRetries,
			InitialDelay: u.config.RetryDelay,
			MaxDelay:     u.config.RetryMaxDelay,
			Multiplier:   u.config.RetryMultiplier,
			JitterFactor: 0.1,
			RetryIf: func(err error) bool {
				return utils.IsRetryableHTTPError(err)
			},
		}

		var data []byte
		var downloadedName string
		_, retryErr := utils.RetryWithBackoff(ctx, retryConfig, func() error {
			var err error
			data, downloadedName, err = u.githubClient.DownloadAsset(
				ctx,
				u.config.Owner,
				u.config.Repo,
				asset.ID,
			)
			return err
		})

		if retryErr != nil {
			return nil, retryErr
		}

		return &DownloadResult{
			Data:        data,
			AssetName:   downloadedName,
			Size:        len(data),
			DownloadAt:  time.Now(),
			DownloadURL: asset.DownloadURL,
		}, nil
	})

	// Record metrics
	duration := time.Since(startTime)
	u.recordMetric("update_download_duration_seconds", duration.Seconds())

	if err != nil {
		u.recordMetric("update_download_failures_total", 1)
		u.logger.Error("Failed to download update", map[string]interface{}{
			"error":      err.Error(),
			"asset_name": assetName,
			"duration":   duration.Seconds(),
		})
		return nil, fmt.Errorf("failed to download update: %w", err)
	}

	downloadResult := result.(*DownloadResult)

	// Verify checksum if enabled
	if u.config.VerifyChecksum {
		if err := u.verifyChecksum(ctx, release, downloadResult); err != nil {
			u.recordMetric("update_checksum_failures_total", 1)
			return nil, fmt.Errorf("checksum verification failed: %w", err)
		}
		u.recordMetric("update_checksum_success_total", 1)
		u.logger.Info("Checksum verification passed", map[string]interface{}{
			"asset_name": assetName,
		})
	}

	u.recordMetric("update_download_success_total", 1)
	u.recordMetric("update_download_bytes_total", float64(downloadResult.Size))

	u.logger.Info("Update downloaded successfully", map[string]interface{}{
		"asset_name": assetName,
		"size_bytes": downloadResult.Size,
		"duration":   duration.Seconds(),
	})

	return downloadResult, nil
}

// verifyChecksum verifies the downloaded data against the checksum file
func (u *Updater) verifyChecksum(ctx context.Context, release *Release, download *DownloadResult) error {
	// Find checksum asset
	checksumAssetName := download.AssetName + u.config.ChecksumAssetSuffix
	checksumAsset := release.FindAsset(checksumAssetName)
	if checksumAsset == nil {
		u.logger.Warn("Checksum asset not found, skipping verification", map[string]interface{}{
			"checksum_asset": checksumAssetName,
		})
		return nil // Don't fail if checksum file doesn't exist
	}

	// Download checksum file
	u.logger.Debug("Downloading checksum file", map[string]interface{}{
		"checksum_asset": checksumAssetName,
	})

	checksumData, _, err := u.githubClient.DownloadAsset(ctx, u.config.Owner, u.config.Repo, checksumAsset.ID)
	if err != nil {
		return fmt.Errorf("failed to download checksum file: %w", err)
	}

	expectedChecksum := string(checksumData)

	// Verify using pkg/security (Phase 1 component)
	if err := security.VerifyBytesChecksum(download.Data, expectedChecksum); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}

	download.Checksum = expectedChecksum
	return nil
}

// recordMetric records a metric if metrics client is available
func (u *Updater) recordMetric(name string, value float64) {
	if u.metrics != nil {
		labels := map[string]string{
			"version": u.currentVersion.String(),
			"channel": string(u.config.Channel),
		}
		u.metrics.RecordGauge(name, value, labels)
	}
}

// ApplyUpdate applies a downloaded update by replacing the binary
// This is the Phase 3 binary replacement with rollback capability
func (u *Updater) ApplyUpdate(ctx context.Context, download *DownloadResult) (*ReplaceResult, error) {
	u.logger.Info("Applying update", map[string]interface{}{
		"asset_name": download.AssetName,
		"size_bytes": download.Size,
	})

	startTime := time.Now()

	// Create binary replacer
	replacer := NewBinaryReplacer(u.config, u.logger)

	// Apply the update with timeout
	applyCtx, cancel := context.WithTimeout(ctx, u.config.ApplyTimeout)
	defer cancel()

	result, err := replacer.ApplyUpdate(applyCtx, download)
	if err != nil {
		u.recordMetric("update_apply_failures_total", 1)
		u.logger.Error("Failed to apply update", map[string]interface{}{
			"error":    err.Error(),
			"duration": time.Since(startTime).Seconds(),
		})
		return nil, fmt.Errorf("failed to apply update: %w", err)
	}

	// Record metrics
	u.recordMetric("update_apply_success_total", 1)
	u.recordMetric("update_apply_duration_seconds", time.Since(startTime).Seconds())

	u.logger.Info("Update applied successfully", map[string]interface{}{
		"backup_path":   result.BackupPath,
		"needs_restart": result.NeedsRestart,
		"duration":      time.Since(startTime).Seconds(),
	})

	return result, nil
}

// VerifyUpdateOnStartup verifies that an update was successful after restart
// This should be called early in the application startup
func (u *Updater) VerifyUpdateOnStartup(ctx context.Context) error {
	if !u.config.RollbackEnabled {
		u.logger.Debug("Rollback not enabled, skipping update verification", nil)
		return nil
	}

	replacer := NewBinaryReplacer(u.config, u.logger)

	if err := replacer.VerifyUpdate(ctx); err != nil {
		u.recordMetric("update_verification_failures_total", 1)
		return fmt.Errorf("update verification failed: %w", err)
	}

	u.recordMetric("update_verification_success_total", 1)
	return nil
}

// GetLastCheckTime returns the time of the last update check
func (u *Updater) GetLastCheckTime() time.Time {
	u.lastCheckMu.RLock()
	defer u.lastCheckMu.RUnlock()
	return u.lastCheck
}

// Close gracefully shuts down the updater
func (u *Updater) Close() error {
	u.logger.Info("Shutting down updater", nil)

	// Stop background checker if running
	close(u.stopChan)
	u.wg.Wait()

	u.logger.Info("Updater shutdown complete", nil)
	return nil
}
