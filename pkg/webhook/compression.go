package webhook

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

// CompressionType represents the type of compression used
type CompressionType string

const (
	CompressionGzip CompressionType = "gzip"
	CompressionZstd CompressionType = "zstd"
)

// SemanticCompressionService implements intelligent compression that preserves semantic structure
type SemanticCompressionService struct {
	compressionType  CompressionType
	compressionLevel int
	encoder          *zstd.Encoder
	decoder          *zstd.Decoder
}

// NewSemanticCompressionService creates a new compression service
func NewSemanticCompressionService(compressionType CompressionType, level int) (*SemanticCompressionService, error) {
	svc := &SemanticCompressionService{
		compressionType:  compressionType,
		compressionLevel: level,
	}

	if compressionType == CompressionZstd {
		// Initialize Zstd encoder/decoder
		encoder, err := zstd.NewWriter(nil,
			zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(level)))
		if err != nil {
			return nil, fmt.Errorf("failed to create zstd encoder: %w", err)
		}
		svc.encoder = encoder

		decoder, err := zstd.NewReader(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create zstd decoder: %w", err)
		}
		svc.decoder = decoder
	}

	return svc, nil
}

// CompressWithSemantics compresses data while preserving semantic structure
func (s *SemanticCompressionService) CompressWithSemantics(data []byte) ([]byte, float64, error) {
	// First, try to parse as JSON to apply semantic compression
	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err == nil {
		// Apply semantic compression techniques
		optimized := s.optimizeJSON(jsonData)

		// Re-marshal with compact format
		compactJSON, err := json.Marshal(optimized)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal optimized JSON: %w", err)
		}

		data = compactJSON
	}

	// Apply compression
	var compressed []byte
	var err error

	switch s.compressionType {
	case CompressionGzip:
		compressed, err = s.compressGzip(data)
	case CompressionZstd:
		compressed, err = s.compressZstd(data)
	default:
		return nil, 0, fmt.Errorf("unsupported compression type: %s", s.compressionType)
	}

	if err != nil {
		return nil, 0, err
	}

	// Calculate compression ratio
	ratio := float64(len(compressed)) / float64(len(data))

	return compressed, ratio, nil
}

// Compress is a simplified compression method that wraps CompressWithSemantics
func (s *SemanticCompressionService) Compress(data interface{}) ([]byte, error) {
	// Marshal the data to bytes
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	compressed, _, err := s.CompressWithSemantics(jsonData)
	return compressed, err
}

// Decompress decompresses data
func (s *SemanticCompressionService) Decompress(data []byte) ([]byte, error) {
	switch s.compressionType {
	case CompressionGzip:
		return s.decompressGzip(data)
	case CompressionZstd:
		return s.decompressZstd(data)
	default:
		return nil, fmt.Errorf("unsupported compression type: %s", s.compressionType)
	}
}

// optimizeJSON applies semantic compression to JSON data
func (s *SemanticCompressionService) optimizeJSON(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		// Remove null values and empty strings
		optimized := make(map[string]interface{})
		for key, value := range v {
			if value != nil {
				if str, ok := value.(string); ok && str == "" {
					continue // Skip empty strings
				}
				// Recursively optimize nested structures
				optimized[key] = s.optimizeJSON(value)
			}
		}
		return optimized

	case []interface{}:
		// Optimize arrays
		optimized := make([]interface{}, 0, len(v))
		for _, item := range v {
			opt := s.optimizeJSON(item)
			if opt != nil {
				optimized = append(optimized, opt)
			}
		}
		return optimized

	default:
		// Return as-is for primitive types
		return v
	}
}

// compressGzip compresses data using gzip
func (s *SemanticCompressionService) compressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buf, s.compressionLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip writer: %w", err)
	}

	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("failed to write gzip data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// decompressGzip decompresses gzip data
func (s *SemanticCompressionService) decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() { _ = reader.Close() }()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read gzip data: %w", err)
	}

	return decompressed, nil
}

// compressZstd compresses data using Zstandard
func (s *SemanticCompressionService) compressZstd(data []byte) ([]byte, error) {
	return s.encoder.EncodeAll(data, nil), nil
}

// decompressZstd decompresses Zstandard data
func (s *SemanticCompressionService) decompressZstd(data []byte) ([]byte, error) {
	return s.decoder.DecodeAll(data, nil)
}

// CompressionStats tracks compression statistics
type CompressionStats struct {
	TotalCompressed   int64
	TotalDecompressed int64
	TotalSpaceSaved   int64
	AverageRatio      float64
	CompressionErrors int64
}

// GetStats returns compression statistics
func (s *SemanticCompressionService) GetStats() *CompressionStats {
	// This would be implemented with actual metrics tracking
	return &CompressionStats{
		AverageRatio: 0.4, // Typical compression ratio
	}
}
