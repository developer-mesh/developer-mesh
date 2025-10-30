package updater

import (
	"os"
	"path/filepath"
	"strings"
)

// IsDevelopmentMode detects if the application is running in development mode.
// This is used to disable auto-updates during local development.
func IsDevelopmentMode() bool {
	// Check environment variable first
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		env = strings.ToLower(env)
		return env == "dev" || env == "development" || env == "local"
	}

	// Check APP_ENV
	if env := os.Getenv("APP_ENV"); env != "" {
		env = strings.ToLower(env)
		return env == "dev" || env == "development" || env == "local"
	}

	// Check GO_ENV
	if env := os.Getenv("GO_ENV"); env != "" {
		env = strings.ToLower(env)
		return env == "dev" || env == "development" || env == "local"
	}

	// Check if running from go run (executable is in temp directory)
	exe, err := os.Executable()
	if err == nil {
		// go run creates temp executables
		if strings.Contains(exe, os.TempDir()) {
			return true
		}
		// Check if running from a source directory
		if strings.Contains(exe, "/go-build") {
			return true
		}
	}

	// Check if .git directory exists (indicates development)
	if _, err := os.Stat(".git"); err == nil {
		return true
	}

	// Check common development directory patterns
	workDir, err := os.Getwd()
	if err == nil {
		workDir = strings.ToLower(workDir)
		devPatterns := []string{
			"/development/",
			"/dev/",
			"/projects/",
			"/workspace/",
			"/src/",
			"/go/src/",
		}
		for _, pattern := range devPatterns {
			if strings.Contains(workDir, pattern) {
				return true
			}
		}
	}

	return false
}

// IsProductionMode returns true if running in production mode.
func IsProductionMode() bool {
	// Check environment variable
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		env = strings.ToLower(env)
		return env == "prod" || env == "production"
	}

	// Check APP_ENV
	if env := os.Getenv("APP_ENV"); env != "" {
		env = strings.ToLower(env)
		return env == "prod" || env == "production"
	}

	// Check GO_ENV
	if env := os.Getenv("GO_ENV"); env != "" {
		env = strings.ToLower(env)
		return env == "prod" || env == "production"
	}

	// If not explicitly development, assume production for safety
	return !IsDevelopmentMode()
}

// ShouldEnableAutoUpdate determines if auto-update should be enabled.
// Auto-update is disabled in development mode and can be explicitly disabled.
func ShouldEnableAutoUpdate() bool {
	// Check if explicitly disabled (highest priority)
	if disabled := os.Getenv("DISABLE_AUTO_UPDATE"); disabled != "" {
		disabled = strings.ToLower(disabled)
		if disabled == "true" || disabled == "1" || disabled == "yes" {
			return false
		}
	}

	// Check if explicitly enabled (overrides environment-based detection)
	if enabled := os.Getenv("ENABLE_AUTO_UPDATE"); enabled != "" {
		enabled = strings.ToLower(enabled)
		return enabled == "true" || enabled == "1" || enabled == "yes"
	}

	// Disable in development mode (only if not explicitly set)
	if IsDevelopmentMode() {
		return false
	}

	// Default: enabled in production, disabled in development
	return IsProductionMode()
}

// GetExecutablePath returns the path to the current executable.
// This is useful for determining where to replace the binary during updates.
func GetExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	// Resolve symlinks
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe, nil // Return original if we can't resolve
	}

	return resolved, nil
}

// IsWritableExecutable checks if the current executable can be replaced.
// Returns true if we have write permissions to the executable directory.
func IsWritableExecutable() bool {
	exe, err := GetExecutablePath()
	if err != nil {
		return false
	}

	dir := filepath.Dir(exe)

	// Try to create a temporary file in the executable's directory
	tmpFile := filepath.Join(dir, ".update_test_"+filepath.Base(exe))
	f, err := os.Create(tmpFile)
	if err != nil {
		return false
	}

	// Always cleanup, even if Close fails
	_ = f.Close()          // Explicitly ignore error - we only care if we can write
	_ = os.Remove(tmpFile) // Explicitly ignore error - cleanup is best-effort

	return true
}
