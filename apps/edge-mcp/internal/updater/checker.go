package updater

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/config"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/updater"
	goGithub "github.com/google/go-github/v74/github"
)

// BackgroundChecker manages background update checking
type BackgroundChecker struct {
	config   *config.UpdaterConfig
	updater  *updater.Updater
	logger   observability.Logger
	stopChan chan struct{}
	wg       sync.WaitGroup
	ticker   *time.Ticker
	mu       sync.RWMutex

	// Status
	isRunning     bool
	lastCheckTime time.Time
	lastResult    *updater.UpdateCheckResult
	lastError     error
}

// NewBackgroundChecker creates a new background update checker
func NewBackgroundChecker(
	cfg *config.UpdaterConfig,
	currentVersion string,
	githubClient *goGithub.Client,
	logger observability.Logger,
	metrics observability.MetricsClient,
) (*BackgroundChecker, error) {
	if !cfg.Enabled {
		logger.Info("Auto-update disabled", nil)
		return nil, nil
	}

	// Create updater config with full defaults
	updaterConfig := updater.DefaultConfig()

	// Override with edge-mcp specific settings
	updaterConfig.Owner = cfg.GitHubOwner
	updaterConfig.Repo = cfg.GitHubRepo
	updaterConfig.Channel = updater.UpdateChannel(cfg.Channel)
	updaterConfig.CurrentVersion = currentVersion
	updaterConfig.CheckInterval = cfg.CheckInterval
	updaterConfig.AutoDownload = cfg.AutoDownload
	updaterConfig.AutoApply = cfg.AutoApply
	updaterConfig.Enabled = cfg.Enabled

	// Create updater instance
	u, err := updater.New(updaterConfig, githubClient, logger, metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to create updater: %w", err)
	}

	checker := &BackgroundChecker{
		config:   cfg,
		updater:  u,
		logger:   logger,
		stopChan: make(chan struct{}),
	}

	logger.Info("Background update checker initialized", map[string]interface{}{
		"check_interval": cfg.CheckInterval.String(),
		"channel":        cfg.Channel,
		"auto_download":  cfg.AutoDownload,
		"auto_apply":     cfg.AutoApply,
	})

	return checker, nil
}

// Start begins background update checking
func (bc *BackgroundChecker) Start(ctx context.Context) {
	if bc == nil {
		return
	}

	bc.mu.Lock()
	if bc.isRunning {
		bc.mu.Unlock()
		bc.logger.Warn("Background checker already running", nil)
		return
	}
	bc.isRunning = true
	bc.ticker = time.NewTicker(bc.config.CheckInterval)
	bc.stopChan = make(chan struct{}) // Recreate stop channel for restart
	bc.mu.Unlock()

	bc.wg.Add(1)
	go bc.run(ctx)

	bc.logger.Info("Background update checker started", map[string]interface{}{
		"check_interval": bc.config.CheckInterval.String(),
	})
}

// run is the main background loop
func (bc *BackgroundChecker) run(ctx context.Context) {
	defer bc.wg.Done()
	defer bc.ticker.Stop()

	// Perform initial check after a short delay
	initialCheckTimer := time.NewTimer(30 * time.Second)
	defer initialCheckTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			bc.logger.Info("Background checker context cancelled", nil)
			return

		case <-bc.stopChan:
			bc.logger.Info("Background checker stopped", nil)
			return

		case <-initialCheckTimer.C:
			bc.performCheck(ctx)

		case <-bc.ticker.C:
			bc.performCheck(ctx)
		}
	}
}

// performCheck performs a single update check
func (bc *BackgroundChecker) performCheck(ctx context.Context) {
	bc.mu.Lock()
	bc.lastCheckTime = time.Now()
	bc.mu.Unlock()

	bc.logger.Debug("Checking for updates", nil)

	// Check for updates with timeout
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	result, err := bc.updater.CheckForUpdate(checkCtx)

	bc.mu.Lock()
	bc.lastResult = result
	bc.lastError = err
	bc.mu.Unlock()

	if err != nil {
		bc.logger.Error("Update check failed", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	if result.UpdateAvailable {
		bc.logger.Info("Update available", map[string]interface{}{
			"current_version": result.CurrentVersion.String(),
			"latest_version":  result.LatestVersion.String(),
			"auto_download":   bc.config.AutoDownload,
		})

		// Auto-download if enabled
		if bc.config.AutoDownload {
			bc.downloadUpdate(ctx, result)
		}
	} else {
		bc.logger.Debug("No update available", map[string]interface{}{
			"current_version": result.CurrentVersion.String(),
			"latest_version":  result.LatestVersion.String(),
		})
	}
}

// downloadUpdate downloads an available update
func (bc *BackgroundChecker) downloadUpdate(ctx context.Context, result *updater.UpdateCheckResult) {
	bc.logger.Info("Downloading update", map[string]interface{}{
		"version": result.LatestVersion.String(),
	})

	downloadCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	downloadResult, err := bc.updater.DownloadUpdate(downloadCtx, result.Release)
	if err != nil {
		bc.logger.Error("Failed to download update", map[string]interface{}{
			"error":   err.Error(),
			"version": result.LatestVersion.String(),
		})
		return
	}

	bc.logger.Info("Update downloaded successfully", map[string]interface{}{
		"version":    result.LatestVersion.String(),
		"size_bytes": downloadResult.Size,
		"auto_apply": bc.config.AutoApply,
	})

	// Auto-apply if enabled (this replaces the binary but requires restart)
	if bc.config.AutoApply {
		bc.applyUpdate(ctx, downloadResult)
	} else {
		bc.logger.Info("Update ready to apply. Restart edge-mcp to apply the update.", map[string]interface{}{
			"version": result.LatestVersion.String(),
		})
	}
}

// applyUpdate applies a downloaded update
func (bc *BackgroundChecker) applyUpdate(ctx context.Context, download *updater.DownloadResult) {
	bc.logger.Info("Applying update", map[string]interface{}{
		"asset_name": download.AssetName,
	})

	applyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	replaceResult, err := bc.updater.ApplyUpdate(applyCtx, download)
	if err != nil {
		bc.logger.Error("Failed to apply update", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	bc.logger.Info("Update applied successfully. Restart required.", map[string]interface{}{
		"backup_path":   replaceResult.BackupPath,
		"needs_restart": replaceResult.NeedsRestart,
	})
}

// Stop gracefully stops the background checker
func (bc *BackgroundChecker) Stop() {
	if bc == nil {
		return
	}

	bc.mu.Lock()
	if !bc.isRunning {
		bc.mu.Unlock()
		return
	}
	bc.isRunning = false
	bc.mu.Unlock()

	bc.logger.Info("Stopping background update checker", nil)
	close(bc.stopChan)
	bc.wg.Wait()
	bc.logger.Info("Background update checker stopped", nil)
}

// GetStatus returns the current status of the update checker
func (bc *BackgroundChecker) GetStatus() *Status {
	if bc == nil {
		return &Status{
			Enabled: false,
		}
	}

	bc.mu.RLock()
	defer bc.mu.RUnlock()

	status := &Status{
		Enabled:       true,
		Running:       bc.isRunning,
		LastCheckTime: bc.lastCheckTime,
		CheckInterval: bc.config.CheckInterval,
		Channel:       bc.config.Channel,
	}

	if bc.lastResult != nil {
		status.UpdateAvailable = bc.lastResult.UpdateAvailable
		if bc.lastResult.CurrentVersion != nil {
			status.CurrentVersion = bc.lastResult.CurrentVersion.String()
		}
		if bc.lastResult.LatestVersion != nil {
			status.LatestVersion = bc.lastResult.LatestVersion.String()
		}
	}

	if bc.lastError != nil {
		status.LastError = bc.lastError.Error()
	}

	return status
}

// Status represents the current status of the update checker
type Status struct {
	Enabled         bool          `json:"enabled"`
	Running         bool          `json:"running"`
	LastCheckTime   time.Time     `json:"last_check_time"`
	CheckInterval   time.Duration `json:"check_interval"`
	Channel         string        `json:"channel"`
	UpdateAvailable bool          `json:"update_available"`
	CurrentVersion  string        `json:"current_version,omitempty"`
	LatestVersion   string        `json:"latest_version,omitempty"`
	LastError       string        `json:"last_error,omitempty"`
}
