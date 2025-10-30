package updater

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/security"
)

// BinaryReplacer handles atomic binary replacement with rollback capability
// Following Phase 3 principles: atomic operations, platform-specific handling, rollback support
type BinaryReplacer struct {
	logger observability.Logger
	config *Config
}

// NewBinaryReplacer creates a new binary replacer instance
func NewBinaryReplacer(config *Config, logger observability.Logger) *BinaryReplacer {
	return &BinaryReplacer{
		config: config,
		logger: logger,
	}
}

// ReplaceResult contains the result of a binary replacement operation
type ReplaceResult struct {
	OldPath        string
	NewPath        string
	BackupPath     string
	BackupCreated  bool
	ReplacedAt     time.Time
	NeedsRestart   bool
	RestartCommand []string
}

// ApplyUpdate performs atomic binary replacement with platform-specific handling
// This is the core Phase 3 implementation
func (r *BinaryReplacer) ApplyUpdate(ctx context.Context, download *DownloadResult) (*ReplaceResult, error) {
	r.logger.Info("Starting binary replacement", map[string]interface{}{
		"asset_name": download.AssetName,
		"size_bytes": download.Size,
		"platform":   r.config.Platform,
	})

	// 1. Determine current binary path
	currentBinary, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Resolve symlinks to get the actual binary path
	currentBinary, err = filepath.EvalSymlinks(currentBinary)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	r.logger.Debug("Resolved current binary path", map[string]interface{}{
		"path": currentBinary,
	})

	// 2. Write new binary to temporary location
	tempBinary, err := r.writeTemporaryBinary(ctx, download.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to write temporary binary: %w", err)
	}
	defer func() {
		// Clean up temp file if replacement fails
		if err := os.Remove(tempBinary); err != nil && !os.IsNotExist(err) {
			r.logger.Warn("Failed to remove temporary binary", map[string]interface{}{
				"path":  tempBinary,
				"error": err.Error(),
			})
		}
	}()

	r.logger.Debug("Wrote temporary binary", map[string]interface{}{
		"path": tempBinary,
	})

	// 3. Verify the new binary before replacing
	if err := r.verifyBinary(ctx, tempBinary); err != nil {
		return nil, fmt.Errorf("binary verification failed: %w", err)
	}

	r.logger.Info("Binary verification passed", nil)

	// 4. Set executable permissions (Unix/macOS)
	if err := r.setPlatformPermissions(tempBinary); err != nil {
		return nil, fmt.Errorf("failed to set permissions: %w", err)
	}

	// 5. Create backup of current binary
	backupPath, err := r.createBackup(currentBinary)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}

	r.logger.Info("Created backup", map[string]interface{}{
		"backup_path": backupPath,
	})

	// 6. Perform atomic replacement
	if err := r.atomicReplace(tempBinary, currentBinary); err != nil {
		// Attempt rollback
		r.logger.Error("Replacement failed, attempting rollback", map[string]interface{}{
			"error": err.Error(),
		})
		if rollbackErr := r.rollback(backupPath, currentBinary); rollbackErr != nil {
			return nil, fmt.Errorf("replacement failed and rollback failed: replace=%w, rollback=%w", err, rollbackErr)
		}
		return nil, fmt.Errorf("replacement failed (rolled back): %w", err)
	}

	r.logger.Info("Binary replacement successful", map[string]interface{}{
		"old_path":    currentBinary,
		"backup_path": backupPath,
	})

	// 7. Prepare restart command
	restartCmd := r.prepareRestartCommand(currentBinary)

	result := &ReplaceResult{
		OldPath:        currentBinary,
		NewPath:        currentBinary, // Same path, new content
		BackupPath:     backupPath,
		BackupCreated:  true,
		ReplacedAt:     time.Now(),
		NeedsRestart:   true,
		RestartCommand: restartCmd,
	}

	return result, nil
}

// writeTemporaryBinary writes the downloaded binary to a temporary file
func (r *BinaryReplacer) writeTemporaryBinary(ctx context.Context, data []byte) (string, error) {
	// Create temp file in same directory as current binary for atomic rename
	currentBinary, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	binDir := filepath.Dir(currentBinary)
	tempFile, err := os.CreateTemp(binDir, "edge-mcp-update-*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	tempPath := tempFile.Name()

	// Write data
	if _, err := tempFile.Write(data); err != nil {
		if closeErr := tempFile.Close(); closeErr != nil {
			r.logger.Warn("Failed to close temp file during error cleanup", map[string]interface{}{
				"error": closeErr.Error(),
			})
		}
		if removeErr := os.Remove(tempPath); removeErr != nil {
			r.logger.Warn("Failed to remove temp file during error cleanup", map[string]interface{}{
				"error": removeErr.Error(),
			})
		}
		return "", fmt.Errorf("failed to write data: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		if removeErr := os.Remove(tempPath); removeErr != nil {
			r.logger.Warn("Failed to remove temp file during error cleanup", map[string]interface{}{
				"error": removeErr.Error(),
			})
		}
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	return tempPath, nil
}

// verifyBinary performs basic verification on the new binary
func (r *BinaryReplacer) verifyBinary(ctx context.Context, path string) error {
	// Check file exists and is readable
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat binary: %w", err)
	}

	// Check file size is reasonable (not empty, not too small)
	if info.Size() < 1024 { // Binaries should be at least 1KB
		return fmt.Errorf("binary file too small: %d bytes", info.Size())
	}

	// Verify checksum if available
	if r.config.VerifyChecksum {
		// Checksum verification already done in DownloadUpdate
		r.logger.Debug("Checksum verification already performed during download", nil)
	}

	// Platform-specific verification
	if runtime.GOOS != "windows" {
		// On Unix, verify it's a binary file (has executable bit or will have it set)
		// We'll set permissions next, so just log for now
		r.logger.Debug("Binary verification complete", map[string]interface{}{
			"size_bytes": info.Size(),
		})
	}

	return nil
}

// setPlatformPermissions sets executable permissions based on platform
func (r *BinaryReplacer) setPlatformPermissions(path string) error {
	switch runtime.GOOS {
	case "windows":
		// Windows doesn't need chmod, .exe extension is enough
		r.logger.Debug("Windows platform, no chmod needed", nil)
		return nil

	case "darwin", "linux":
		// Unix-like: Set executable permissions (0755 = rwxr-xr-x)
		if err := os.Chmod(path, 0755); err != nil {
			return fmt.Errorf("failed to chmod: %w", err)
		}
		r.logger.Debug("Set executable permissions (0755)", map[string]interface{}{
			"path": path,
		})
		return nil

	default:
		// Unknown platform, try to set executable anyway
		r.logger.Warn("Unknown platform, attempting chmod anyway", map[string]interface{}{
			"platform": runtime.GOOS,
		})
		if err := os.Chmod(path, 0755); err != nil {
			// Don't fail on unknown platforms
			r.logger.Warn("Chmod failed on unknown platform", map[string]interface{}{
				"error": err.Error(),
			})
		}
		return nil
	}
}

// createBackup creates a backup of the current binary
func (r *BinaryReplacer) createBackup(currentPath string) (string, error) {
	// Determine backup path
	backupPath := r.config.BackupPath
	if backupPath == "" {
		// Default: current binary path with .backup extension
		backupPath = currentPath + ".backup"
	}

	// Create backup directory if it doesn't exist
	backupDir := filepath.Dir(backupPath)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Remove old backup if it exists
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to remove old backup: %w", err)
	}

	// Copy current binary to backup location
	// Use rename for efficiency (atomic on same filesystem)
	if err := copyFile(currentPath, backupPath); err != nil {
		return "", fmt.Errorf("failed to copy to backup: %w", err)
	}

	// Preserve permissions
	if runtime.GOOS != "windows" {
		if err := os.Chmod(backupPath, 0755); err != nil {
			r.logger.Warn("Failed to set backup permissions", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	return backupPath, nil
}

// atomicReplace performs atomic replacement of the binary
func (r *BinaryReplacer) atomicReplace(newPath, currentPath string) error {
	// On most filesystems, rename is atomic
	// This is the safest way to replace a binary
	if err := os.Rename(newPath, currentPath); err != nil {
		return fmt.Errorf("atomic rename failed: %w", err)
	}

	r.logger.Debug("Atomic rename completed", map[string]interface{}{
		"from": newPath,
		"to":   currentPath,
	})

	return nil
}

// rollback restores the backup binary
func (r *BinaryReplacer) rollback(backupPath, currentPath string) error {
	r.logger.Warn("Rolling back to backup", map[string]interface{}{
		"backup_path":  backupPath,
		"current_path": currentPath,
	})

	if err := os.Rename(backupPath, currentPath); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	r.logger.Info("Rollback successful", nil)
	return nil
}

// CleanupBackup removes the backup file after a successful update
func (r *BinaryReplacer) CleanupBackup(backupPath string) error {
	if backupPath == "" {
		return nil
	}

	r.logger.Info("Cleaning up backup", map[string]interface{}{
		"path": backupPath,
	})

	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove backup: %w", err)
	}

	return nil
}

// VerifyUpdate verifies that the update was successful after restart
// This should be called on startup to check if we just updated
func (r *BinaryReplacer) VerifyUpdate(ctx context.Context) error {
	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks
	currentBinary, err = filepath.EvalSymlinks(currentBinary)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	// Check if backup exists
	backupPath := r.config.BackupPath
	if backupPath == "" {
		backupPath = currentBinary + ".backup"
	}

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		// No backup exists, no update to verify
		return nil
	}

	r.logger.Info("Update verification: backup found, verifying new binary", map[string]interface{}{
		"backup_path": backupPath,
	})

	// Verify the current binary is valid
	if err := r.verifyBinary(ctx, currentBinary); err != nil {
		// Binary is invalid, rollback
		r.logger.Error("New binary verification failed, rolling back", map[string]interface{}{
			"error": err.Error(),
		})
		if rollbackErr := r.rollback(backupPath, currentBinary); rollbackErr != nil {
			return fmt.Errorf("verification failed and rollback failed: %w", rollbackErr)
		}
		return fmt.Errorf("update verification failed, rolled back: %w", err)
	}

	// Verify checksum if we have it stored
	// In a real implementation, you might store the expected checksum
	// and verify it here

	r.logger.Info("Update verification passed, cleaning up backup", nil)

	// Update successful, clean up backup
	if err := r.CleanupBackup(backupPath); err != nil {
		r.logger.Warn("Failed to clean up backup", map[string]interface{}{
			"error": err.Error(),
		})
	}

	return nil
}

// prepareRestartCommand prepares the command to restart the application
func (r *BinaryReplacer) prepareRestartCommand(binaryPath string) []string {
	// Get current process arguments
	args := os.Args[1:] // Exclude the binary path itself

	r.logger.Debug("Prepared restart command", map[string]interface{}{
		"binary": binaryPath,
		"args":   args,
	})

	return append([]string{binaryPath}, args...)
}

// Restart restarts the current process with the new binary
// WARNING: This will terminate the current process!
func (r *BinaryReplacer) Restart(result *ReplaceResult) error {
	if !result.NeedsRestart {
		return nil
	}

	r.logger.Info("Restarting process with new binary", map[string]interface{}{
		"command": result.RestartCommand,
	})

	// Platform-specific restart
	switch runtime.GOOS {
	case "windows":
		return r.restartWindows(result.RestartCommand)
	default:
		return r.restartUnix(result.RestartCommand)
	}
}

// restartUnix restarts the process on Unix-like systems
func (r *BinaryReplacer) restartUnix(command []string) error {
	// Create new process
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Start new process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start new process: %w", err)
	}

	r.logger.Info("New process started, exiting current process", map[string]interface{}{
		"new_pid": cmd.Process.Pid,
	})

	// Exit current process
	os.Exit(0)
	return nil
}

// restartWindows restarts the process on Windows
func (r *BinaryReplacer) restartWindows(command []string) error {
	// On Windows, we need to use a different approach
	// because the .exe file is locked while running

	// Start new process detached
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start new process: %w", err)
	}

	r.logger.Info("New process started (Windows), exiting current process", map[string]interface{}{
		"new_pid": cmd.Process.Pid,
	})

	// Exit current process
	os.Exit(0)
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceData, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source: %w", err)
	}

	if err := os.WriteFile(dst, sourceData, 0755); err != nil {
		return fmt.Errorf("failed to write destination: %w", err)
	}

	return nil
}

// ComputeChecksumForFile computes SHA256 checksum for a file
// Useful for verification and debugging
func ComputeChecksumForFile(path string) (string, error) {
	return security.ComputeFileChecksum(path)
}
