package cache

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"github.com/developer-mesh/developer-mesh/pkg/security"
)

// CompressionService handles compression and encryption for cache data
type CompressionService struct {
	encryptionService *security.EncryptionService
	compressionLevel  int
	minSizeBytes      int
}

// NewCompressionService creates a new compression service
func NewCompressionService(encryptionKey string) *CompressionService {
	return &CompressionService{
		encryptionService: security.NewEncryptionService(encryptionKey),
		compressionLevel:  gzip.BestSpeed, // Fast compression for cache
		minSizeBytes:      1024,           // Only compress if > 1KB
	}
}

// CompressAndEncrypt compresses then encrypts data
func (c *CompressionService) CompressAndEncrypt(data []byte, tenantID string) (string, error) {
	// Skip compression for small data
	if len(data) < c.minSizeBytes {
		encrypted, err := c.encryptionService.EncryptCredential(string(data), tenantID)
		if err != nil {
			return "", fmt.Errorf("encryption failed: %w", err)
		}
		return base64.StdEncoding.EncodeToString(encrypted), nil
	}

	// Compress
	compressed, err := c.compress(data)
	if err != nil {
		return "", fmt.Errorf("compression failed: %w", err)
	}

	// Check if compression actually reduced size
	if len(compressed) >= len(data) {
		// Compression didn't help, encrypt original
		encrypted, err := c.encryptionService.EncryptCredential(string(data), tenantID)
		if err != nil {
			return "", fmt.Errorf("encryption failed: %w", err)
		}
		return base64.StdEncoding.EncodeToString(encrypted), nil
	}

	// Encrypt compressed data
	encrypted, err := c.encryptionService.EncryptCredential(string(compressed), tenantID)
	if err != nil {
		return "", fmt.Errorf("encryption failed: %w", err)
	}

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// DecryptAndDecompress decrypts then decompresses data
func (c *CompressionService) DecryptAndDecompress(encryptedBase64, tenantID string) ([]byte, error) {
	// Decode base64
	encrypted, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %w", err)
	}

	// Decrypt
	decrypted, err := c.encryptionService.DecryptCredential(encrypted, tenantID)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	// Check if compressed (gzip magic bytes)
	data := []byte(decrypted)
	if !c.isCompressed(data) {
		return data, nil
	}

	// Decompress
	decompressed, err := c.decompress(data)
	if err != nil {
		// If decompression fails, data might not be compressed
		// Return as-is
		return data, nil
	}

	return decompressed, nil
}

// CompressOnly compresses data without encryption
func (c *CompressionService) CompressOnly(data []byte) ([]byte, error) {
	if len(data) < c.minSizeBytes {
		return data, nil
	}

	compressed, err := c.compress(data)
	if err != nil {
		return nil, err
	}

	// Return original if compression didn't help
	if len(compressed) >= len(data) {
		return data, nil
	}

	return compressed, nil
}

// DecompressOnly decompresses data without decryption
func (c *CompressionService) DecompressOnly(data []byte) ([]byte, error) {
	if !c.isCompressed(data) {
		return data, nil
	}

	return c.decompress(data)
}

func (c *CompressionService) compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, c.compressionLevel)
	if err != nil {
		return nil, err
	}

	if _, err := gz.Write(data); err != nil {
		_ = gz.Close()
		return nil, fmt.Errorf("compression write failed: %w", err)
	}

	if err := gz.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (c *CompressionService) decompress(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = gz.Close()
	}()

	// Limit reader to prevent decompression bombs
	limitedReader := io.LimitReader(gz, 100*1024*1024) // 100MB max
	return io.ReadAll(limitedReader)
}

func (c *CompressionService) isCompressed(data []byte) bool {
	// Check for gzip magic bytes
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}

// GetCompressionRatio returns the compression ratio for given data
func (c *CompressionService) GetCompressionRatio(data []byte) (float64, error) {
	if len(data) == 0 {
		return 0, nil
	}

	compressed, err := c.compress(data)
	if err != nil {
		return 0, err
	}

	ratio := 1.0 - (float64(len(compressed)) / float64(len(data)))
	return ratio, nil
}

// SetCompressionLevel updates the compression level
func (c *CompressionService) SetCompressionLevel(level int) error {
	if level < gzip.NoCompression || level > gzip.BestCompression {
		return fmt.Errorf("invalid compression level: %d", level)
	}
	c.compressionLevel = level
	return nil
}

// SetMinSize updates the minimum size for compression
func (c *CompressionService) SetMinSize(minSize int) {
	c.minSizeBytes = minSize
}

// CompressedCacheEntry wraps a cache entry with compression support
type CompressedCacheEntry struct {
	*CacheEntry
	CompressedData string `json:"compressed_data,omitempty"`
	IsCompressed   bool   `json:"is_compressed"`
}

// CompressEntry compresses a cache entry's results
func (c *CompressionService) CompressEntry(entry *CacheEntry, tenantID string) (*CompressedCacheEntry, error) {
	// Serialize results for compression
	resultsData, err := marshalResults(entry.Results)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal results: %w", err)
	}

	// Compress and encrypt
	compressed, err := c.CompressAndEncrypt(resultsData, tenantID)
	if err != nil {
		return nil, err
	}

	return &CompressedCacheEntry{
		CacheEntry:     entry,
		CompressedData: compressed,
		IsCompressed:   true,
	}, nil
}

// DecompressEntry decompresses a cache entry's results
func (c *CompressionService) DecompressEntry(entry *CompressedCacheEntry, tenantID string) (*CacheEntry, error) {
	if !entry.IsCompressed || entry.CompressedData == "" {
		return entry.CacheEntry, nil
	}

	// Decrypt and decompress
	data, err := c.DecryptAndDecompress(entry.CompressedData, tenantID)
	if err != nil {
		return nil, err
	}

	// Unmarshal results
	results, err := unmarshalResults(data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal results: %w", err)
	}

	// Create new entry with decompressed results
	decompressed := *entry.CacheEntry
	decompressed.Results = results

	return &decompressed, nil
}

// marshalResults serializes cache results to JSON
func marshalResults(results []CachedSearchResult) ([]byte, error) {
	return json.Marshal(results)
}

// unmarshalResults deserializes cache results from JSON
func unmarshalResults(data []byte) ([]CachedSearchResult, error) {
	var results []CachedSearchResult
	err := json.Unmarshal(data, &results)
	return results, err
}
