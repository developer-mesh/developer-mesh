package rerank

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/embedding/providers"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/resilience"
	"github.com/developer-mesh/developer-mesh/pkg/retry"
	"golang.org/x/sync/semaphore"
)

// CrossEncoderReranker uses a cross-encoder model for reranking with resilience patterns
type CrossEncoderReranker struct {
	provider    providers.RerankProvider
	config      *CrossEncoderConfig
	breaker     *resilience.CircuitBreaker
	retryPolicy retry.Policy
	semaphore   *semaphore.Weighted
	logger      observability.Logger
	metrics     observability.MetricsClient
}

// CrossEncoderConfig configures the cross-encoder reranker
type CrossEncoderConfig struct {
	Model              string
	BatchSize          int
	MaxConcurrency     int
	TimeoutPerBatch    time.Duration
	CircuitBreakerName string
}

// NewCrossEncoderReranker creates a new cross-encoder reranker
func NewCrossEncoderReranker(
	provider providers.RerankProvider,
	config *CrossEncoderConfig,
	logger observability.Logger,
	metrics observability.MetricsClient,
) (*CrossEncoderReranker, error) {
	if provider == nil {
		return nil, fmt.Errorf("provider is required")
	}
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	// Set defaults
	if config.BatchSize <= 0 {
		config.BatchSize = 10
	}
	if config.MaxConcurrency <= 0 {
		config.MaxConcurrency = 3
	}
	if config.TimeoutPerBatch == 0 {
		config.TimeoutPerBatch = 5 * time.Second
	}
	if config.CircuitBreakerName == "" {
		config.CircuitBreakerName = fmt.Sprintf("reranker_%s", config.Model)
	}
	if logger == nil {
		logger = observability.NewLogger("rerank.cross_encoder")
	}
	if metrics == nil {
		metrics = observability.NewMetricsClient()
	}

	// Create circuit breaker
	breaker := resilience.NewCircuitBreaker(config.CircuitBreakerName, resilience.CircuitBreakerConfig{
		FailureThreshold:    5,
		FailureRatio:        0.5,
		ResetTimeout:        30 * time.Second,
		SuccessThreshold:    2,
		MaxRequestsHalfOpen: 10,
		TimeoutThreshold:    10 * time.Second,
	}, logger, metrics)

	// Create retry policy
	retryPolicy := retry.NewExponentialBackoff(retry.Config{
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     2 * time.Second,
		MaxElapsedTime:  30 * time.Second,
		Multiplier:      2.0,
		MaxRetries:      3,
	})

	return &CrossEncoderReranker{
		provider:    provider,
		config:      config,
		breaker:     breaker,
		retryPolicy: retryPolicy,
		semaphore:   semaphore.NewWeighted(int64(config.MaxConcurrency)),
		logger:      logger,
		metrics:     metrics,
	}, nil
}

// Rerank reorders search results using a cross-encoder model
func (c *CrossEncoderReranker) Rerank(ctx context.Context, query string, results []SearchResult, opts *RerankOptions) ([]SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}

	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "rerank.cross_encoder")
	defer span.End()

	span.SetAttribute("model", c.config.Model)
	span.SetAttribute("input_count", len(results))
	span.SetAttribute("batch_size", c.config.BatchSize)

	start := time.Now()
	defer func() {
		c.metrics.RecordHistogram("rerank.cross_encoder.duration", time.Since(start).Seconds(),
			map[string]string{"model": c.config.Model})
	}()

	c.logger.Info("Starting cross-encoder reranking", map[string]interface{}{
		"query":       query,
		"num_results": len(results),
		"model":       c.config.Model,
	})

	// Process in batches to avoid overwhelming the API
	batches := c.createBatches(results, c.config.BatchSize)
	allReranked := make([]SearchResult, 0, len(results))

	for batchIdx, batch := range batches {
		// Acquire semaphore to limit concurrency
		if err := c.semaphore.Acquire(ctx, 1); err != nil {
			return nil, fmt.Errorf("failed to acquire semaphore: %w", err)
		}

		batchReranked, err := c.processBatchWithRetry(ctx, query, batch, batchIdx)
		c.semaphore.Release(1)

		if err != nil {
			// Log error but continue with other batches (graceful degradation)
			c.logger.Error("Batch reranking failed", map[string]interface{}{
				"batch": batchIdx,
				"error": err.Error(),
			})
			c.metrics.IncrementCounter("rerank.cross_encoder.batch_failure", 1.0)

			// Use original results for failed batch
			allReranked = append(allReranked, batch...)
		} else {
			allReranked = append(allReranked, batchReranked...)
		}
	}

	// Sort by score
	sort.Slice(allReranked, func(i, j int) bool {
		return allReranked[i].Score > allReranked[j].Score
	})

	// Return top K if specified
	if opts != nil && opts.TopK > 0 && opts.TopK < len(allReranked) {
		allReranked = allReranked[:opts.TopK]
	}

	span.SetAttribute("output_count", len(allReranked))
	c.metrics.IncrementCounter("rerank.cross_encoder.success", 1.0)

	return allReranked, nil
}

// processBatchWithRetry processes a batch of results with retry logic
func (c *CrossEncoderReranker) processBatchWithRetry(ctx context.Context, query string, batch []SearchResult, batchIdx int) ([]SearchResult, error) {
	var rerankedBatch []SearchResult
	var lastErr error

	// Use retry policy with circuit breaker
	err := c.retryPolicy.Execute(ctx, func(ctx context.Context) error {
		// Check circuit breaker
		result, err := c.breaker.Execute(ctx, func() (interface{}, error) {
			// Create timeout context for this batch
			batchCtx, cancel := context.WithTimeout(ctx, c.config.TimeoutPerBatch)
			defer cancel()

			// Prepare documents
			documents := make([]string, len(batch))
			for i, result := range batch {
				documents[i] = result.Content
			}

			// Call provider
			resp, err := c.provider.Rerank(batchCtx, providers.RerankRequest{
				Query:     query,
				Documents: documents,
				Model:     c.config.Model,
			})
			if err != nil {
				lastErr = err
				return nil, err
			}

			// Apply scores to results
			reranked := make([]SearchResult, len(batch))
			for i, result := range batch {
				reranked[i] = result

				// Find matching score from response
				for _, rerankResult := range resp.Results {
					if rerankResult.Index == i {
						reranked[i].Score = float32(rerankResult.Score)
						if reranked[i].Metadata == nil {
							reranked[i].Metadata = make(map[string]interface{})
						}
						reranked[i].Metadata["original_score"] = result.Score
						reranked[i].Metadata["rerank_model"] = c.config.Model
						reranked[i].Metadata["reranked"] = true
						break
					}
				}
			}

			return reranked, nil
		})

		if err != nil {
			return err
		}

		rerankedBatch = result.([]SearchResult)
		return nil
	})

	if err != nil {
		c.metrics.IncrementCounter("rerank.cross_encoder.batch_failure", 1.0)
		return nil, fmt.Errorf("batch reranking failed after retries: %w", lastErr)
	}

	return rerankedBatch, nil
}

// createBatches splits results into batches
func (c *CrossEncoderReranker) createBatches(results []SearchResult, batchSize int) [][]SearchResult {
	var batches [][]SearchResult

	for i := 0; i < len(results); i += batchSize {
		end := i + batchSize
		if end > len(results) {
			end = len(results)
		}
		batches = append(batches, results[i:end])
	}

	return batches
}

// GetName returns the name of the reranker
func (c *CrossEncoderReranker) GetName() string {
	return fmt.Sprintf("cross_encoder_%s", c.config.Model)
}

// Close cleans up resources
func (c *CrossEncoderReranker) Close() error {
	// Circuit breaker doesn't need explicit closing in our implementation
	return nil
}
