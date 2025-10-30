package updater

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBinaryReplacer_ComponentWorkflow(t *testing.T) {
	// Test the individual components of binary replacement workflow
	// Full integration test would require building actual binaries and cannot
	// be tested with os.Executable() in unit tests
	logger := observability.NewNoopLogger()

	tests := []struct {
		name          string
		setupFunc     func(t *testing.T) (string, *DownloadResult, *Config)
		expectError   bool
		errorContains string
	}{
		{
			name: "valid binary workflow",
			setupFunc: func(t *testing.T) (string, *DownloadResult, *Config) {
				testDir := t.TempDir()

				// Create "new" binary content (larger than 1KB for verification)
				newContent := make([]byte, 2048)
				copy(newContent, []byte("new binary content v1.1.0"))
				download := &DownloadResult{
					Data:       newContent,
					AssetName:  "edge-mcp-test",
					Size:       len(newContent),
					DownloadAt: time.Now(),
				}

				config := DefaultConfig()
				config.BackupPath = filepath.Join(testDir, "edge-mcp.backup")

				return testDir, download, config
			},
			expectError: false,
		},
		{
			name: "invalid binary - too small",
			setupFunc: func(t *testing.T) (string, *DownloadResult, *Config) {
				testDir := t.TempDir()

				// Create invalid new binary (too small)
				newContent := []byte("x") // Only 1 byte
				download := &DownloadResult{
					Data:       newContent,
					AssetName:  "edge-mcp-test",
					Size:       len(newContent),
					DownloadAt: time.Now(),
				}

				config := DefaultConfig()
				config.BackupPath = filepath.Join(testDir, "edge-mcp.backup")

				return testDir, download, config
			},
			expectError:   true,
			errorContains: "binary file too small",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, download, config := tt.setupFunc(t)
			replacer := NewBinaryReplacer(config, logger)
			ctx := context.Background()

			// Test write temporary binary
			tempPath, err := replacer.writeTemporaryBinary(ctx, download.Data)
			if err != nil && tt.expectError {
				return
			}
			require.NoError(t, err)
			defer func() {
				if removeErr := os.Remove(tempPath); removeErr != nil {
					t.Logf("Failed to remove temp file: %v", removeErr)
				}
			}()

			// Test verification
			err = replacer.verifyBinary(ctx, tempPath)
			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}
			require.NoError(t, err)

			// Test set permissions
			err = replacer.setPlatformPermissions(tempPath)
			require.NoError(t, err)

			// Verify permissions were set correctly
			if runtime.GOOS != "windows" {
				info, err := os.Stat(tempPath)
				require.NoError(t, err)
				assert.True(t, info.Mode()&0111 != 0, "Should have executable permission")
			}

			// Verify file exists and is readable
			assert.FileExists(t, tempPath)
			content, err := os.ReadFile(tempPath)
			require.NoError(t, err)
			assert.Equal(t, download.Data, content, "Written content should match original")
		})
	}
}

func TestBinaryReplacer_CreateBackup(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultConfig()

	testDir := t.TempDir()

	// Create a test binary
	testBinary := filepath.Join(testDir, "test-binary")
	originalContent := []byte("original binary content")
	err := os.WriteFile(testBinary, originalContent, 0755)
	require.NoError(t, err)

	// Set backup path
	config.BackupPath = filepath.Join(testDir, "test-binary.backup")

	replacer := NewBinaryReplacer(config, logger)

	// Create backup
	backupPath, err := replacer.createBackup(testBinary)
	require.NoError(t, err)
	assert.Equal(t, config.BackupPath, backupPath)

	// Verify backup exists
	assert.FileExists(t, backupPath)

	// Verify backup content
	backupContent, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, backupContent)

	// Verify permissions on Unix
	if runtime.GOOS != "windows" {
		info, err := os.Stat(backupPath)
		require.NoError(t, err)
		assert.True(t, info.Mode()&0111 != 0, "Backup should be executable")
	}
}

func TestBinaryReplacer_AtomicReplace(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultConfig()

	testDir := t.TempDir()

	// Create source and target files
	sourcePath := filepath.Join(testDir, "source")
	targetPath := filepath.Join(testDir, "target")

	sourceContent := []byte("new content")
	targetContent := []byte("old content")

	err := os.WriteFile(sourcePath, sourceContent, 0755)
	require.NoError(t, err)

	err = os.WriteFile(targetPath, targetContent, 0755)
	require.NoError(t, err)

	replacer := NewBinaryReplacer(config, logger)

	// Perform atomic replace
	err = replacer.atomicReplace(sourcePath, targetPath)
	require.NoError(t, err)

	// Verify target now has source content
	actualContent, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	assert.Equal(t, sourceContent, actualContent)

	// Verify source no longer exists
	_, err = os.Stat(sourcePath)
	assert.True(t, os.IsNotExist(err), "Source should be moved, not copied")
}

func TestBinaryReplacer_Rollback(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultConfig()

	testDir := t.TempDir()

	// Create backup and current files
	backupPath := filepath.Join(testDir, "backup")
	currentPath := filepath.Join(testDir, "current")

	backupContent := []byte("backup content")
	badContent := []byte("corrupted content")

	err := os.WriteFile(backupPath, backupContent, 0755)
	require.NoError(t, err)

	err = os.WriteFile(currentPath, badContent, 0755)
	require.NoError(t, err)

	replacer := NewBinaryReplacer(config, logger)

	// Perform rollback
	err = replacer.rollback(backupPath, currentPath)
	require.NoError(t, err)

	// Verify current now has backup content
	actualContent, err := os.ReadFile(currentPath)
	require.NoError(t, err)
	assert.Equal(t, backupContent, actualContent)

	// Verify backup no longer exists (moved to current)
	_, err = os.Stat(backupPath)
	assert.True(t, os.IsNotExist(err), "Backup should be moved to current")
}

func TestBinaryReplacer_CleanupBackup(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultConfig()

	testDir := t.TempDir()

	// Create a backup file
	backupPath := filepath.Join(testDir, "test-backup")
	err := os.WriteFile(backupPath, []byte("backup"), 0755)
	require.NoError(t, err)

	replacer := NewBinaryReplacer(config, logger)

	// Cleanup backup
	err = replacer.CleanupBackup(backupPath)
	require.NoError(t, err)

	// Verify backup is deleted
	_, err = os.Stat(backupPath)
	assert.True(t, os.IsNotExist(err), "Backup should be deleted")

	// Test cleanup on non-existent file (should not error)
	err = replacer.CleanupBackup(backupPath)
	require.NoError(t, err)
}

func TestBinaryReplacer_VerifyBinary(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultConfig()
	config.VerifyChecksum = false // Disable checksum for basic tests

	replacer := NewBinaryReplacer(config, logger)
	ctx := context.Background()

	tests := []struct {
		name          string
		setupFunc     func(t *testing.T) string
		expectError   bool
		errorContains string
	}{
		{
			name: "valid binary",
			setupFunc: func(t *testing.T) string {
				testDir := t.TempDir()
				binaryPath := filepath.Join(testDir, "valid-binary")
				// Create a file larger than 1KB
				content := make([]byte, 2048)
				for i := range content {
					content[i] = byte(i % 256)
				}
				err := os.WriteFile(binaryPath, content, 0755)
				require.NoError(t, err)
				return binaryPath
			},
			expectError: false,
		},
		{
			name: "binary too small",
			setupFunc: func(t *testing.T) string {
				testDir := t.TempDir()
				binaryPath := filepath.Join(testDir, "tiny-binary")
				err := os.WriteFile(binaryPath, []byte("tiny"), 0755)
				require.NoError(t, err)
				return binaryPath
			},
			expectError:   true,
			errorContains: "binary file too small",
		},
		{
			name: "binary doesn't exist",
			setupFunc: func(t *testing.T) string {
				return "/nonexistent/path/to/binary"
			},
			expectError:   true,
			errorContains: "failed to stat binary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binaryPath := tt.setupFunc(t)

			err := replacer.verifyBinary(ctx, binaryPath)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestBinaryReplacer_PrepareRestartCommand(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultConfig()

	replacer := NewBinaryReplacer(config, logger)

	// Save original args
	originalArgs := os.Args

	// Test with mock args
	os.Args = []string{"/path/to/binary", "--config", "test.yaml", "--verbose"}

	binaryPath := "/path/to/new/binary"
	cmd := replacer.prepareRestartCommand(binaryPath)

	// Restore original args
	os.Args = originalArgs

	// Verify command structure
	require.Len(t, cmd, 4)
	assert.Equal(t, binaryPath, cmd[0])
	assert.Equal(t, "--config", cmd[1])
	assert.Equal(t, "test.yaml", cmd[2])
	assert.Equal(t, "--verbose", cmd[3])
}

func TestComputeChecksumForFile(t *testing.T) {
	testDir := t.TempDir()

	// Create test file with known content
	testFile := filepath.Join(testDir, "test-file")
	content := []byte("test content for checksum")
	err := os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)

	// Compute checksum
	checksum, err := ComputeChecksumForFile(testFile)
	require.NoError(t, err)
	assert.NotEmpty(t, checksum)

	// Verify checksum is consistent
	checksum2, err := ComputeChecksumForFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, checksum, checksum2, "Checksum should be deterministic")

	// Verify checksum is hex string (64 chars for SHA256)
	assert.Len(t, checksum, 64, "SHA256 checksum should be 64 hex characters")
}

func TestCopyFile(t *testing.T) {
	testDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(testDir, "source")
	srcContent := []byte("source file content")
	err := os.WriteFile(srcPath, srcContent, 0644)
	require.NoError(t, err)

	// Copy file
	dstPath := filepath.Join(testDir, "destination")
	err = copyFile(srcPath, dstPath)
	require.NoError(t, err)

	// Verify destination exists
	assert.FileExists(t, dstPath)

	// Verify content is same
	dstContent, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, srcContent, dstContent)

	// Verify source still exists (copy, not move)
	assert.FileExists(t, srcPath)

	// Verify permissions on Unix
	if runtime.GOOS != "windows" {
		info, err := os.Stat(dstPath)
		require.NoError(t, err)
		assert.True(t, info.Mode()&0111 != 0, "Destination should be executable (0755)")
	}
}
