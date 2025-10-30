package updater

import (
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/resilience"
)

// Config holds configuration for the updater
type Config struct {
	// GitHub repository information
	Owner string // Repository owner (e.g., "developer-mesh")
	Repo  string // Repository name (e.g., "developer-mesh")

	// Update channel configuration
	Channel UpdateChannel // stable, beta, or latest

	// Current version
	CurrentVersion string // Current running version (e.g., "0.0.9")

	// Update check configuration
	CheckInterval time.Duration // How often to check for updates
	AutoDownload  bool          // Automatically download available updates
	AutoApply     bool          // Automatically apply downloaded updates

	// Platform configuration
	Platform string // Target platform (e.g., "linux", "darwin", "windows")
	Arch     string // Target architecture (e.g., "amd64", "arm64")

	// Asset naming pattern
	// Use Go template syntax: {{.OS}} {{.Arch}} {{.Ext}}
	// Example: "edge-mcp-{{.OS}}-{{.Arch}}{{.Ext}}"
	AssetNamePattern string

	// Checksum configuration
	ChecksumAssetSuffix string // Suffix for checksum files (e.g., ".sha256")
	VerifyChecksum      bool   // Whether to verify checksums

	// Circuit breaker configuration
	CircuitBreaker resilience.CircuitBreakerConfig

	// Retry configuration
	MaxRetries      int           // Maximum number of retry attempts
	RetryDelay      time.Duration // Initial delay between retries
	RetryMaxDelay   time.Duration // Maximum delay between retries
	RetryMultiplier float64       // Multiplier for exponential backoff

	// Timeout configuration
	DownloadTimeout time.Duration // Timeout for downloading releases
	ApplyTimeout    time.Duration // Timeout for applying updates

	// Safety configuration
	RequireSignature bool   // Require GPG signature verification (future)
	BackupPath       string // Path to store backup of current binary
	RollbackEnabled  bool   // Enable automatic rollback on failure

	// Feature flags
	Enabled bool // Master switch to enable/disable auto-update
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Owner:               "developer-mesh",
		Repo:                "developer-mesh",
		Channel:             StableChannel,
		CheckInterval:       24 * time.Hour,
		AutoDownload:        true,
		AutoApply:           false, // Require manual approval for safety
		Platform:            detectPlatform(),
		Arch:                detectArch(),
		AssetNamePattern:    "edge-mcp-{{.OS}}-{{.Arch}}{{.Ext}}",
		ChecksumAssetSuffix: ".sha256",
		VerifyChecksum:      true,
		CircuitBreaker: resilience.CircuitBreakerConfig{
			FailureThreshold:    3,
			FailureRatio:        0.6,
			ResetTimeout:        60 * time.Second,
			SuccessThreshold:    2,
			TimeoutThreshold:    30 * time.Second,
			MaxRequestsHalfOpen: 3,
			MinimumRequestCount: 5,
		},
		MaxRetries:      3,
		RetryDelay:      2 * time.Second,
		RetryMaxDelay:   30 * time.Second,
		RetryMultiplier: 2.0,
		DownloadTimeout: 5 * time.Minute,
		ApplyTimeout:    30 * time.Second,
		BackupPath:      "", // Will be set automatically
		RollbackEnabled: true,
		Enabled:         ShouldEnableAutoUpdate(),
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Owner == "" {
		return ErrInvalidConfig{Field: "Owner", Message: "owner cannot be empty"}
	}
	if c.Repo == "" {
		return ErrInvalidConfig{Field: "Repo", Message: "repo cannot be empty"}
	}
	if c.CurrentVersion == "" {
		return ErrInvalidConfig{Field: "CurrentVersion", Message: "current version cannot be empty"}
	}
	if c.Platform == "" {
		return ErrInvalidConfig{Field: "Platform", Message: "platform cannot be empty"}
	}
	if c.Arch == "" {
		return ErrInvalidConfig{Field: "Arch", Message: "architecture cannot be empty"}
	}
	if c.AssetNamePattern == "" {
		return ErrInvalidConfig{Field: "AssetNamePattern", Message: "asset name pattern cannot be empty"}
	}
	if c.CheckInterval <= 0 {
		return ErrInvalidConfig{Field: "CheckInterval", Message: "check interval must be positive"}
	}
	if c.DownloadTimeout <= 0 {
		return ErrInvalidConfig{Field: "DownloadTimeout", Message: "download timeout must be positive"}
	}
	if c.ApplyTimeout <= 0 {
		return ErrInvalidConfig{Field: "ApplyTimeout", Message: "apply timeout must be positive"}
	}

	// Validate version can be parsed
	if _, err := ParseVersion(c.CurrentVersion); err != nil {
		return ErrInvalidConfig{Field: "CurrentVersion", Message: "invalid version format: " + err.Error()}
	}

	return nil
}

// ErrInvalidConfig represents a configuration validation error
type ErrInvalidConfig struct {
	Field   string
	Message string
}

func (e ErrInvalidConfig) Error() string {
	return "invalid config field '" + e.Field + "': " + e.Message
}

// AssetName generates the asset name for the current platform/arch
func (c *Config) AssetName() (string, error) {
	return generateAssetName(c.AssetNamePattern, c.Platform, c.Arch)
}
