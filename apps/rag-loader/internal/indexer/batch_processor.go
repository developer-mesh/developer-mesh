// Package indexer implements batch processing for embeddings
package indexer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/pkg/embedding"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/rag/models"
	repoVector "github.com/developer-mesh/developer-mesh/pkg/repository/vector"
)

// BatchProcessorConfig configures the batch processor
type BatchProcessorConfig struct {
	BatchSize      int           // Number of chunks to process in each batch
	MaxConcurrency int           // Maximum number of concurrent batches
	RetryAttempts  int           // Number of retry attempts for failed chunks
	RetryDelay     time.Duration // Delay between retries
}

// DefaultBatchProcessorConfig returns default configuration
func DefaultBatchProcessorConfig() BatchProcessorConfig {
	return BatchProcessorConfig{
		BatchSize:      10,
		MaxConcurrency: 3,
		RetryAttempts:  3,
		RetryDelay:     time.Second,
	}
}

// BatchProcessor handles batch processing of document chunks for embedding generation
type BatchProcessor struct {
	config          BatchProcessorConfig
	embeddingClient *embedding.ContextEmbeddingClient
	vectorRepo      repoVector.Repository
	logger          observability.Logger
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(
	config BatchProcessorConfig,
	embeddingClient *embedding.ContextEmbeddingClient,
	vectorRepo repoVector.Repository,
	logger observability.Logger,
) *BatchProcessor {
	return &BatchProcessor{
		config:          config,
		embeddingClient: embeddingClient,
		vectorRepo:      vectorRepo,
		logger:          logger,
	}
}

// ChunkEmbeddingRequest represents a chunk to be embedded
type ChunkEmbeddingRequest struct {
	DocumentID uuid.UUID
	Chunk      *models.Chunk
	TenantID   uuid.UUID
}

// ChunkEmbeddingResult represents the result of processing a chunk
type ChunkEmbeddingResult struct {
	ChunkID     uuid.UUID
	EmbeddingID string
	Error       error
}

// ProcessChunks processes multiple chunks in batches
func (b *BatchProcessor) ProcessChunks(ctx context.Context, requests []ChunkEmbeddingRequest) ([]ChunkEmbeddingResult, error) {
	if len(requests) == 0 {
		return nil, nil
	}

	b.logger.Info("Starting batch chunk processing", map[string]interface{}{
		"total_chunks": len(requests),
		"batch_size":   b.config.BatchSize,
		"concurrency":  b.config.MaxConcurrency,
	})

	// Create batches
	batches := b.createBatches(requests)

	// Process batches with concurrency control
	results := make([]ChunkEmbeddingResult, 0, len(requests))
	resultsChan := make(chan []ChunkEmbeddingResult, len(batches))
	errorsChan := make(chan error, len(batches))

	// Use semaphore to limit concurrency
	sem := make(chan struct{}, b.config.MaxConcurrency)
	var wg sync.WaitGroup

	for i, batch := range batches {
		wg.Add(1)
		go func(batchNum int, batchRequests []ChunkEmbeddingRequest) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			b.logger.Debug("Processing batch", map[string]interface{}{
				"batch_number": batchNum,
				"batch_size":   len(batchRequests),
			})

			batchResults, err := b.processBatch(ctx, batchRequests)
			if err != nil {
				errorsChan <- fmt.Errorf("batch %d failed: %w", batchNum, err)
				return
			}

			resultsChan <- batchResults
		}(i, batch)
	}

	// Wait for all batches to complete
	go func() {
		wg.Wait()
		close(resultsChan)
		close(errorsChan)
	}()

	// Collect results
	var processingErrors []error
	for batchResults := range resultsChan {
		results = append(results, batchResults...)
	}

	// Collect errors
	for err := range errorsChan {
		processingErrors = append(processingErrors, err)
	}

	// Log summary
	successCount := 0
	for _, result := range results {
		if result.Error == nil {
			successCount++
		}
	}

	b.logger.Info("Batch processing completed", map[string]interface{}{
		"total_chunks": len(requests),
		"successful":   successCount,
		"failed":       len(results) - successCount,
		"batch_errors": len(processingErrors),
	})

	if len(processingErrors) > 0 {
		return results, fmt.Errorf("encountered %d batch errors", len(processingErrors))
	}

	return results, nil
}

// createBatches splits requests into batches of configured size
func (b *BatchProcessor) createBatches(requests []ChunkEmbeddingRequest) [][]ChunkEmbeddingRequest {
	var batches [][]ChunkEmbeddingRequest

	for i := 0; i < len(requests); i += b.config.BatchSize {
		end := i + b.config.BatchSize
		if end > len(requests) {
			end = len(requests)
		}
		batches = append(batches, requests[i:end])
	}

	return batches
}

// processBatch processes a single batch of chunks
func (b *BatchProcessor) processBatch(ctx context.Context, requests []ChunkEmbeddingRequest) ([]ChunkEmbeddingResult, error) {
	results := make([]ChunkEmbeddingResult, len(requests))

	for i, req := range requests {
		result := b.processChunkWithRetry(ctx, req)
		results[i] = result
	}

	return results, nil
}

// processChunkWithRetry processes a single chunk with retry logic
func (b *BatchProcessor) processChunkWithRetry(ctx context.Context, req ChunkEmbeddingRequest) ChunkEmbeddingResult {
	var lastErr error

	for attempt := 0; attempt <= b.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			b.logger.Debug("Retrying chunk processing", map[string]interface{}{
				"chunk_id": req.Chunk.ID,
				"attempt":  attempt,
			})

			// Wait before retrying
			select {
			case <-time.After(b.config.RetryDelay * time.Duration(attempt)):
			case <-ctx.Done():
				return ChunkEmbeddingResult{
					ChunkID: req.Chunk.ID,
					Error:   ctx.Err(),
				}
			}
		}

		embeddingID, err := b.processChunk(ctx, req)
		if err == nil {
			return ChunkEmbeddingResult{
				ChunkID:     req.Chunk.ID,
				EmbeddingID: embeddingID,
				Error:       nil,
			}
		}

		lastErr = err
	}

	return ChunkEmbeddingResult{
		ChunkID: req.Chunk.ID,
		Error:   fmt.Errorf("failed after %d attempts: %w", b.config.RetryAttempts+1, lastErr),
	}
}

// processChunk processes a single chunk
func (b *BatchProcessor) processChunk(ctx context.Context, req ChunkEmbeddingRequest) (string, error) {
	// Generate embedding
	vector, modelUsed, err := b.embeddingClient.EmbedContent(
		ctx,
		req.Chunk.Content,
		"", // Use default model for tenant
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Create embedding record
	emb := &repoVector.Embedding{
		ID:        uuid.New().String(),
		ContextID: req.TenantID.String(), // Use tenant ID as context
		Text:      req.Chunk.Content,
		Embedding: vector,
		ModelID:   modelUsed,
		CreatedAt: time.Now(),
		Metadata: map[string]interface{}{
			"document_id": req.DocumentID.String(),
			"chunk_index": req.Chunk.ChunkIndex,
			"source_type": "rag",
			"chunk_id":    req.Chunk.ID.String(),
		},
	}

	// Store embedding
	if err := b.vectorRepo.StoreEmbedding(ctx, emb); err != nil {
		return "", fmt.Errorf("failed to store embedding: %w", err)
	}

	return emb.ID, nil
}

// ProcessBatchStats returns statistics about batch processing
type ProcessBatchStats struct {
	TotalChunks      int
	SuccessfulChunks int
	FailedChunks     int
	AverageTime      time.Duration
	TotalTime        time.Duration
}

// GetStats calculates statistics from results
func (b *BatchProcessor) GetStats(results []ChunkEmbeddingResult, totalTime time.Duration) ProcessBatchStats {
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
