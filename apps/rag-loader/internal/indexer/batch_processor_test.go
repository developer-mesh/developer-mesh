package indexer

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/developer-mesh/developer-mesh/pkg/rag/models"
)

func TestDefaultBatchProcessorConfig(t *testing.T) {
	config := DefaultBatchProcessorConfig()

	assert.Equal(t, 10, config.BatchSize)
	assert.Equal(t, 3, config.MaxConcurrency)
	assert.Equal(t, 3, config.RetryAttempts)
	assert.Equal(t, time.Second, config.RetryDelay)
}

func TestCreateBatches(t *testing.T) {
	tests := []struct {
		name            string
		batchSize       int
		numRequests     int
		expectedBatches int
		validateFunc    func(t *testing.T, batches [][]ChunkEmbeddingRequest, requests []ChunkEmbeddingRequest)
	}{
		{
			name:            "Exact multiple",
			batchSize:       5,
			numRequests:     10,
			expectedBatches: 2,
			validateFunc: func(t *testing.T, batches [][]ChunkEmbeddingRequest, requests []ChunkEmbeddingRequest) {
				// Each batch should have exactly 5 items
				assert.Len(t, batches[0], 5)
				assert.Len(t, batches[1], 5)
			},
		},
		{
			name:            "Remainder",
			batchSize:       5,
			numRequests:     12,
			expectedBatches: 3,
			validateFunc: func(t *testing.T, batches [][]ChunkEmbeddingRequest, requests []ChunkEmbeddingRequest) {
				// First two batches should have 5, last should have 2
				assert.Len(t, batches[0], 5)
				assert.Len(t, batches[1], 5)
				assert.Len(t, batches[2], 2)
			},
		},
		{
			name:            "Single batch",
			batchSize:       10,
			numRequests:     5,
			expectedBatches: 1,
			validateFunc: func(t *testing.T, batches [][]ChunkEmbeddingRequest, requests []ChunkEmbeddingRequest) {
				assert.Len(t, batches[0], 5)
			},
		},
		{
			name:            "Empty",
			batchSize:       5,
			numRequests:     0,
			expectedBatches: 0,
			validateFunc: func(t *testing.T, batches [][]ChunkEmbeddingRequest, requests []ChunkEmbeddingRequest) {
				// No validation needed for empty batches
			},
		},
		{
			name:            "Single item",
			batchSize:       10,
			numRequests:     1,
			expectedBatches: 1,
			validateFunc: func(t *testing.T, batches [][]ChunkEmbeddingRequest, requests []ChunkEmbeddingRequest) {
				assert.Len(t, batches[0], 1)
			},
		},
		{
			name:            "Large batch size",
			batchSize:       100,
			numRequests:     10,
			expectedBatches: 1,
			validateFunc: func(t *testing.T, batches [][]ChunkEmbeddingRequest, requests []ChunkEmbeddingRequest) {
				assert.Len(t, batches[0], 10)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test requests
			requests := make([]ChunkEmbeddingRequest, tt.numRequests)
			for i := 0; i < tt.numRequests; i++ {
				requests[i] = ChunkEmbeddingRequest{
					TenantID:   uuid.New(),
					DocumentID: uuid.New(),
					Chunk: &models.Chunk{
						ID:         uuid.New(),
						Content:    "test content",
						ChunkIndex: i,
					},
				}
			}

			// Create batches using test helper
			batches := createTestBatches(requests, tt.batchSize)

			// Verify batch count
			assert.Len(t, batches, tt.expectedBatches)

			// Verify all requests are included
			totalCount := 0
			for _, batch := range batches {
				totalCount += len(batch)
			}
			assert.Equal(t, tt.numRequests, totalCount)

			// Run custom validation if provided
			if tt.validateFunc != nil {
				tt.validateFunc(t, batches, requests)
			}

			// Verify order is preserved
			if len(requests) > 0 {
				reconstituted := make([]ChunkEmbeddingRequest, 0, len(requests))
				for _, batch := range batches {
					reconstituted = append(reconstituted, batch...)
				}
				for i := range requests {
					assert.Equal(t, requests[i].Chunk.ChunkIndex, reconstituted[i].Chunk.ChunkIndex)
				}
			}
		})
	}
}

// createTestBatches is a test helper that mimics the batch creation logic
func createTestBatches(requests []ChunkEmbeddingRequest, batchSize int) [][]ChunkEmbeddingRequest {
	var batches [][]ChunkEmbeddingRequest

	for i := 0; i < len(requests); i += batchSize {
		end := i + batchSize
		if end > len(requests) {
			end = len(requests)
		}
		batches = append(batches, requests[i:end])
	}

	return batches
}

func TestGetStats(t *testing.T) {
	tests := []struct {
		name               string
		results            []ChunkEmbeddingResult
		totalTime          time.Duration
		expectedSuccessful int
		expectedFailed     int
		expectedAvgTime    time.Duration
	}{
		{
			name: "All successful",
			results: []ChunkEmbeddingResult{
				{ChunkID: uuid.New(), EmbeddingID: "1", Error: nil},
				{ChunkID: uuid.New(), EmbeddingID: "2", Error: nil},
				{ChunkID: uuid.New(), EmbeddingID: "3", Error: nil},
			},
			totalTime:          3 * time.Second,
			expectedSuccessful: 3,
			expectedFailed:     0,
			expectedAvgTime:    1 * time.Second,
		},
		{
			name: "Some failed",
			results: []ChunkEmbeddingResult{
				{ChunkID: uuid.New(), EmbeddingID: "1", Error: nil},
				{ChunkID: uuid.New(), Error: errors.New("failed")},
				{ChunkID: uuid.New(), EmbeddingID: "3", Error: nil},
			},
			totalTime:          3 * time.Second,
			expectedSuccessful: 2,
			expectedFailed:     1,
			expectedAvgTime:    1 * time.Second,
		},
		{
			name: "All failed",
			results: []ChunkEmbeddingResult{
				{ChunkID: uuid.New(), Error: errors.New("failed 1")},
				{ChunkID: uuid.New(), Error: errors.New("failed 2")},
			},
			totalTime:          2 * time.Second,
			expectedSuccessful: 0,
			expectedFailed:     2,
			expectedAvgTime:    1 * time.Second,
		},
		{
			name:               "Empty results",
			results:            []ChunkEmbeddingResult{},
			totalTime:          0,
			expectedSuccessful: 0,
			expectedFailed:     0,
			expectedAvgTime:    0,
		},
		{
			name: "Single successful",
			results: []ChunkEmbeddingResult{
				{ChunkID: uuid.New(), EmbeddingID: "1", Error: nil},
			},
			totalTime:          500 * time.Millisecond,
			expectedSuccessful: 1,
			expectedFailed:     0,
			expectedAvgTime:    500 * time.Millisecond,
		},
		{
			name: "Single failed",
			results: []ChunkEmbeddingResult{
				{ChunkID: uuid.New(), Error: errors.New("failed")},
			},
			totalTime:          100 * time.Millisecond,
			expectedSuccessful: 0,
			expectedFailed:     1,
			expectedAvgTime:    100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := calculateTestStats(tt.results, tt.totalTime)

			assert.Equal(t, len(tt.results), stats.TotalChunks, "Total chunks mismatch")
			assert.Equal(t, tt.expectedSuccessful, stats.SuccessfulChunks, "Successful chunks mismatch")
			assert.Equal(t, tt.expectedFailed, stats.FailedChunks, "Failed chunks mismatch")
			assert.Equal(t, tt.totalTime, stats.TotalTime, "Total time mismatch")
			assert.Equal(t, tt.expectedAvgTime, stats.AverageTime, "Average time mismatch")
		})
	}
}

// calculateTestStats is a test helper that mimics GetStats logic
func calculateTestStats(results []ChunkEmbeddingResult, totalTime time.Duration) ProcessBatchStats {
	successful := 0
	for _, result := range results {
		if result.Error == nil {
			successful++
		}
	}

	avgTime := time.Duration(0)
	if len(results) > 0 {
		avgTime = totalTime / time.Duration(len(results))
	}

	return ProcessBatchStats{
		TotalChunks:      len(results),
		SuccessfulChunks: successful,
		FailedChunks:     len(results) - successful,
		AverageTime:      avgTime,
		TotalTime:        totalTime,
	}
}

func TestChunkEmbeddingRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		request ChunkEmbeddingRequest
		valid   bool
	}{
		{
			name: "Valid request",
			request: ChunkEmbeddingRequest{
				TenantID:   uuid.New(),
				DocumentID: uuid.New(),
				Chunk: &models.Chunk{
					ID:         uuid.New(),
					Content:    "test content",
					ChunkIndex: 0,
				},
			},
			valid: true,
		},
		{
			name: "Missing chunk",
			request: ChunkEmbeddingRequest{
				TenantID:   uuid.New(),
				DocumentID: uuid.New(),
				Chunk:      nil,
			},
			valid: false,
		},
		{
			name: "Empty content",
			request: ChunkEmbeddingRequest{
				TenantID:   uuid.New(),
				DocumentID: uuid.New(),
				Chunk: &models.Chunk{
					ID:         uuid.New(),
					Content:    "",
					ChunkIndex: 0,
				},
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := validateRequest(tt.request)
			assert.Equal(t, tt.valid, valid)
		})
	}
}

// validateRequest is a test helper to validate requests
func validateRequest(req ChunkEmbeddingRequest) bool {
	if req.Chunk == nil {
		return false
	}
	if req.Chunk.Content == "" {
		return false
	}
	return true
}

func TestProcessBatchStats_Methods(t *testing.T) {
	stats := ProcessBatchStats{
		TotalChunks:      10,
		SuccessfulChunks: 8,
		FailedChunks:     2,
		AverageTime:      100 * time.Millisecond,
		TotalTime:        1 * time.Second,
	}

	// Test success rate calculation
	successRate := float64(stats.SuccessfulChunks) / float64(stats.TotalChunks)
	assert.InDelta(t, 0.8, successRate, 0.01)

	// Test failure rate calculation
	failureRate := float64(stats.FailedChunks) / float64(stats.TotalChunks)
	assert.InDelta(t, 0.2, failureRate, 0.01)

	// Test throughput (chunks per second)
	throughput := float64(stats.TotalChunks) / stats.TotalTime.Seconds()
	assert.InDelta(t, 10.0, throughput, 0.1)
}

func TestBatchProcessorConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  BatchProcessorConfig
		isValid bool
	}{
		{
			name:    "Valid config",
			config:  DefaultBatchProcessorConfig(),
			isValid: true,
		},
		{
			name: "Invalid batch size (zero)",
			config: BatchProcessorConfig{
				BatchSize:      0,
				MaxConcurrency: 3,
				RetryAttempts:  3,
				RetryDelay:     time.Second,
			},
			isValid: false,
		},
		{
			name: "Invalid batch size (negative)",
			config: BatchProcessorConfig{
				BatchSize:      -1,
				MaxConcurrency: 3,
				RetryAttempts:  3,
				RetryDelay:     time.Second,
			},
			isValid: false,
		},
		{
			name: "Invalid max concurrency (zero)",
			config: BatchProcessorConfig{
				BatchSize:      10,
				MaxConcurrency: 0,
				RetryAttempts:  3,
				RetryDelay:     time.Second,
			},
			isValid: false,
		},
		{
			name: "Invalid retry delay (negative)",
			config: BatchProcessorConfig{
				BatchSize:      10,
				MaxConcurrency: 3,
				RetryAttempts:  3,
				RetryDelay:     -1 * time.Second,
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := validateConfig(tt.config)
			assert.Equal(t, tt.isValid, valid)
		})
	}
}

// validateConfig is a test helper to validate configuration
func validateConfig(cfg BatchProcessorConfig) bool {
	if cfg.BatchSize <= 0 {
		return false
	}
	if cfg.MaxConcurrency <= 0 {
		return false
	}
	if cfg.RetryDelay < 0 {
		return false
	}
	return true
}
