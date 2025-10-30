package security

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// VerifyFileChecksum verifies that a file matches the expected SHA256 checksum.
// This is essential for ensuring downloaded files haven't been tampered with.
func VerifyFileChecksum(filepath, expectedSHA256 string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		// Best effort close - we already read the file successfully
		_ = file.Close()
	}()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return fmt.Errorf("failed to compute checksum: %w", err)
	}

	computed := hex.EncodeToString(h.Sum(nil))
	if computed != expectedSHA256 {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSHA256, computed)
	}

	return nil
}

// ComputeFileChecksum calculates the SHA256 checksum of a file.
// Useful for generating checksums for release artifacts.
func ComputeFileChecksum(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		// Best effort close - we already read the file successfully
		_ = file.Close()
	}()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("failed to compute checksum: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyBytesChecksum verifies that a byte slice matches the expected SHA256 checksum.
// Useful for verifying downloaded content before writing to disk.
func VerifyBytesChecksum(data []byte, expectedSHA256 string) error {
	h := sha256.New()
	if _, err := h.Write(data); err != nil {
		return fmt.Errorf("failed to compute checksum: %w", err)
	}

	computed := hex.EncodeToString(h.Sum(nil))
	if computed != expectedSHA256 {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSHA256, computed)
	}

	return nil
}

// ComputeBytesChecksum calculates the SHA256 checksum of a byte slice.
func ComputeBytesChecksum(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
