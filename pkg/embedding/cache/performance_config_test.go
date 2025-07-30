package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetPerformanceProfile(t *testing.T) {
	tests := []struct {
		name    string
		profile PerformanceProfile
		checks  func(t *testing.T, config *PerformanceConfig)
	}{
		{
			name:    "LowLatency profile",
			profile: ProfileLowLatency,
			checks: func(t *testing.T, config *PerformanceConfig) {
				assert.Equal(t, 500*time.Millisecond, config.GetTimeout)
				assert.Equal(t, 10, config.BatchSize)
				assert.Equal(t, "none", config.CompressionLevel)
				assert.Equal(t, 5, config.MaxCandidates)
				assert.Equal(t, float32(0.95), config.SimilarityThreshold)
				assert.True(t, config.CircuitBreakerEnabled)
				assert.Equal(t, 3, config.CircuitBreakerThreshold)
				assert.Equal(t, 2, config.RetryMaxAttempts)
			},
		},
		{
			name:    "HighThroughput profile",
			profile: ProfileHighThroughput,
			checks: func(t *testing.T, config *PerformanceConfig) {
				assert.Equal(t, 3*time.Second, config.GetTimeout)
				assert.Equal(t, 100, config.BatchSize)
				assert.Equal(t, "best", config.CompressionLevel)
				assert.Equal(t, 20, config.MaxCandidates)
				assert.Equal(t, float32(0.90), config.SimilarityThreshold)
				assert.True(t, config.CircuitBreakerEnabled)
				assert.Equal(t, 10, config.CircuitBreakerThreshold)
				assert.Equal(t, 5, config.RetryMaxAttempts)
			},
		},
		{
			name:    "Balanced profile",
			profile: ProfileBalanced,
			checks: func(t *testing.T, config *PerformanceConfig) {
				assert.Equal(t, 1*time.Second, config.GetTimeout)
				assert.Equal(t, 50, config.BatchSize)
				assert.Equal(t, "fast", config.CompressionLevel)
				assert.Equal(t, 10, config.MaxCandidates)
				assert.Equal(t, float32(0.93), config.SimilarityThreshold)
				assert.True(t, config.CircuitBreakerEnabled)
				assert.Equal(t, 5, config.CircuitBreakerThreshold)
				assert.Equal(t, 3, config.RetryMaxAttempts)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := GetPerformanceProfile(tt.profile)
			assert.NotNil(t, config)
			tt.checks(t, config)
		})
	}
}

func TestPerformanceConfig_Validate(t *testing.T) {
	// Test with zero values
	config := &PerformanceConfig{}
	err := config.Validate()
	assert.NoError(t, err)

	// Check defaults were applied
	assert.Equal(t, 1*time.Second, config.GetTimeout)
	assert.Equal(t, 2*time.Second, config.SetTimeout)
	assert.Equal(t, 1*time.Second, config.DeleteTimeout)
	assert.Equal(t, 50, config.BatchSize)
	assert.Equal(t, 200*time.Millisecond, config.FlushInterval)
	assert.Equal(t, 4096, config.CompressionThreshold)
	assert.Equal(t, 10, config.MaxCandidates)
	// SimilarityThreshold should be set by Validate when it's 0

	// Test with invalid similarity threshold
	config = &PerformanceConfig{
		SimilarityThreshold: 1.5,
	}
	err = config.Validate()
	assert.NoError(t, err)
	assert.Equal(t, float32(0.93), config.SimilarityThreshold)

	// Test with zero similarity threshold
	config = &PerformanceConfig{
		SimilarityThreshold: 0,
	}
	err = config.Validate()
	assert.NoError(t, err)
	assert.Equal(t, float32(0.93), config.SimilarityThreshold)

	// Test with negative values
	config = &PerformanceConfig{
		BatchSize:            -1,
		RetryMaxAttempts:     -1,
		CompressionThreshold: 0,
	}
	err = config.Validate()
	assert.NoError(t, err)
	assert.Equal(t, 50, config.BatchSize)
	assert.Equal(t, 3, config.RetryMaxAttempts)
	assert.Equal(t, 4096, config.CompressionThreshold)
}

func TestConfig_ApplyPerformanceProfile(t *testing.T) {
	config := DefaultConfig()
	originalSimilarity := config.SimilarityThreshold
	originalCandidates := config.MaxCandidates

	// Apply low latency profile
	config.ApplyPerformanceProfile(ProfileLowLatency)

	// Check that values were updated
	assert.NotEqual(t, originalSimilarity, config.SimilarityThreshold)
	assert.NotEqual(t, originalCandidates, config.MaxCandidates)
	assert.Equal(t, float32(0.95), config.SimilarityThreshold)
	assert.Equal(t, 5, config.MaxCandidates)
	assert.NotNil(t, config.RedisPoolConfig)

	// Apply high throughput profile
	config.ApplyPerformanceProfile(ProfileHighThroughput)
	assert.Equal(t, float32(0.90), config.SimilarityThreshold)
	assert.Equal(t, 20, config.MaxCandidates)
}
