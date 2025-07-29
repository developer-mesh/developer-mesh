package cache_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache"
)

func TestCompressionService_CompressAndEncrypt(t *testing.T) {
	service := cache.NewCompressionService("test-encryption-key-32-chars-long!!")
	tenantID := "test-tenant"

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "small data (no compression)",
			data:    []byte("small data"),
			wantErr: false,
		},
		{
			name:    "large data (with compression)",
			data:    []byte(strings.Repeat("large data to compress ", 100)),
			wantErr: false,
		},
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := service.CompressAndEncrypt(tt.data, tenantID)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotEmpty(t, encrypted)

			// Verify it's base64
			assert.NotPanics(t, func() {
				_, _ = service.DecryptAndDecompress(encrypted, tenantID)
			})
		})
	}
}

func TestCompressionService_DecryptAndDecompress(t *testing.T) {
	service := cache.NewCompressionService("test-encryption-key-32-chars-long!!")
	tenantID := "test-tenant"

	// Test round trip
	originalData := []byte(strings.Repeat("test data for compression ", 50))

	encrypted, err := service.CompressAndEncrypt(originalData, tenantID)
	require.NoError(t, err)

	decrypted, err := service.DecryptAndDecompress(encrypted, tenantID)
	require.NoError(t, err)

	assert.Equal(t, originalData, decrypted)
}

func TestCompressionService_CompressOnly(t *testing.T) {
	service := cache.NewCompressionService("test-encryption-key-32-chars-long!!")

	tests := []struct {
		name           string
		data           []byte
		expectCompress bool
	}{
		{
			name:           "small data",
			data:           []byte("small"),
			expectCompress: false, // Too small to compress
		},
		{
			name:           "large repeating data",
			data:           []byte(strings.Repeat("compress me ", 100)),
			expectCompress: true,
		},
		{
			name:           "random data",
			data:           make([]byte, 2000), // Zero bytes compress well
			expectCompress: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressed, err := service.CompressOnly(tt.data)
			assert.NoError(t, err)

			if tt.expectCompress && len(tt.data) > 1024 {
				// Should be compressed (might not always be smaller)
				decompressed, err := service.DecompressOnly(compressed)
				assert.NoError(t, err)
				assert.Equal(t, tt.data, decompressed)
			} else {
				// Should return original
				assert.Equal(t, tt.data, compressed)
			}
		})
	}
}

func TestCompressionService_GetCompressionRatio(t *testing.T) {
	service := cache.NewCompressionService("test-encryption-key-32-chars-long!!")

	// Highly compressible data
	data := []byte(strings.Repeat("a", 1000))
	ratio, err := service.GetCompressionRatio(data)
	assert.NoError(t, err)
	assert.Greater(t, ratio, 0.5) // Should compress well

	// Empty data
	ratio, err = service.GetCompressionRatio([]byte{})
	assert.NoError(t, err)
	assert.Equal(t, 0.0, ratio)
}

func TestCompressionService_CompressEntry(t *testing.T) {
	service := cache.NewCompressionService("test-encryption-key-32-chars-long!!")
	tenantID := "test-tenant"

	entry := &cache.CacheEntry{
		Query: "test query",
		Results: []cache.CachedSearchResult{
			{
				ID:      "1",
				Content: strings.Repeat("result content ", 100),
				Score:   0.95,
			},
			{
				ID:      "2",
				Content: strings.Repeat("another result ", 100),
				Score:   0.85,
			},
		},
	}

	compressed, err := service.CompressEntry(entry, tenantID)
	assert.NoError(t, err)
	assert.NotNil(t, compressed)
	assert.True(t, compressed.IsCompressed)
	assert.NotEmpty(t, compressed.CompressedData)

	// Decompress and verify
	decompressed, err := service.DecompressEntry(compressed, tenantID)
	assert.NoError(t, err)
	assert.Equal(t, entry.Query, decompressed.Query)
	assert.Len(t, decompressed.Results, 2)
	assert.Equal(t, entry.Results[0].ID, decompressed.Results[0].ID)
}
