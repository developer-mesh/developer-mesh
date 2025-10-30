package security

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyFileChecksum(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a test file with known content
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("Hello, World!")
	err := os.WriteFile(testFile, testContent, 0644)
	require.NoError(t, err)

	// SHA256 of "Hello, World!" is:
	// dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f
	validChecksum := "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"
	invalidChecksum := "0000000000000000000000000000000000000000000000000000000000000000"

	tests := []struct {
		name        string
		filepath    string
		checksum    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid checksum",
			filepath:    testFile,
			checksum:    validChecksum,
			expectError: false,
		},
		{
			name:        "invalid checksum",
			filepath:    testFile,
			checksum:    invalidChecksum,
			expectError: true,
			errorMsg:    "checksum mismatch",
		},
		{
			name:        "nonexistent file",
			filepath:    filepath.Join(tmpDir, "nonexistent.txt"),
			checksum:    validChecksum,
			expectError: true,
			errorMsg:    "failed to open file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyFileChecksum(tt.filepath, tt.checksum)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestComputeFileChecksum(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name             string
		content          []byte
		expectedChecksum string
		expectError      bool
	}{
		{
			name:             "hello world",
			content:          []byte("Hello, World!"),
			expectedChecksum: "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f",
			expectError:      false,
		},
		{
			name:             "empty file",
			content:          []byte(""),
			expectedChecksum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			expectError:      false,
		},
		{
			name:             "binary data",
			content:          []byte{0x00, 0x01, 0x02, 0x03, 0xFF},
			expectedChecksum: "ff5d8507b6a72bee2debce2c0054798deaccdc5d8a1b945b6280ce8aa9cba52e",
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, tt.name+".bin")
			err := os.WriteFile(testFile, tt.content, 0644)
			require.NoError(t, err)

			checksum, err := ComputeFileChecksum(testFile)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedChecksum, checksum)
			}
		})
	}
}

func TestComputeFileChecksum_NonexistentFile(t *testing.T) {
	checksum, err := ComputeFileChecksum("/nonexistent/path/file.txt")
	assert.Error(t, err)
	assert.Empty(t, checksum)
	assert.Contains(t, err.Error(), "failed to open file")
}

func TestVerifyBytesChecksum(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		checksum    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid checksum",
			data:        []byte("Hello, World!"),
			checksum:    "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f",
			expectError: false,
		},
		{
			name:        "invalid checksum",
			data:        []byte("Hello, World!"),
			checksum:    "0000000000000000000000000000000000000000000000000000000000000000",
			expectError: true,
			errorMsg:    "checksum mismatch",
		},
		{
			name:        "empty data",
			data:        []byte(""),
			checksum:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyBytesChecksum(tt.data, tt.checksum)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestComputeBytesChecksum(t *testing.T) {
	tests := []struct {
		name             string
		data             []byte
		expectedChecksum string
	}{
		{
			name:             "hello world",
			data:             []byte("Hello, World!"),
			expectedChecksum: "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f",
		},
		{
			name:             "empty bytes",
			data:             []byte(""),
			expectedChecksum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:             "binary data",
			data:             []byte{0x00, 0x01, 0x02, 0x03, 0xFF},
			expectedChecksum: "ff5d8507b6a72bee2debce2c0054798deaccdc5d8a1b945b6280ce8aa9cba52e",
		},
		{
			name:             "large data",
			data:             make([]byte, 1024*1024), // 1MB of zeros
			expectedChecksum: "30e14955ebf1352266dc2ff8067e68104607e750abb9d3b36582b8af909fcb58",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checksum := ComputeBytesChecksum(tt.data)
			assert.Equal(t, tt.expectedChecksum, checksum)
		})
	}
}

func TestChecksumRoundTrip(t *testing.T) {
	// Test that computing and verifying works correctly
	testData := []byte("Test data for round trip verification")

	// Compute checksum
	checksum := ComputeBytesChecksum(testData)
	assert.NotEmpty(t, checksum)

	// Verify with the computed checksum
	err := VerifyBytesChecksum(testData, checksum)
	assert.NoError(t, err)

	// Verify file round trip
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "roundtrip.txt")
	err = os.WriteFile(testFile, testData, 0644)
	require.NoError(t, err)

	fileChecksum, err := ComputeFileChecksum(testFile)
	require.NoError(t, err)

	err = VerifyFileChecksum(testFile, fileChecksum)
	assert.NoError(t, err)

	// Checksums should match
	assert.Equal(t, checksum, fileChecksum)
}
