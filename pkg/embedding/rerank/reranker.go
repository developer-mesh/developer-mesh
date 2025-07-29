package rerank

import (
	"context"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// SearchResult represents a search result to be reranked
type SearchResult struct {
	ID       string                 `json:"id"`
	Content  string                 `json:"content"`
	Score    float32                `json:"score"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Reranker re-scores and reorders search results
type Reranker interface {
	Rerank(ctx context.Context, query string, results []SearchResult, opts *RerankOptions) ([]SearchResult, error)
	GetName() string
	Close() error
}

// RerankOptions configures reranking behavior
type RerankOptions struct {
	TopK            int     // Return top K results
	Model           string  // Model to use for reranking
	IncludeScores   bool    // Include scores in metadata
	DiversityFactor float64 // For MMR diversity (0-1)
	MaxConcurrency  int     // Max concurrent operations
}

// MultiStageReranker applies multiple rerankers in sequence
type MultiStageReranker struct {
	stages  []RerankStage
	logger  observability.Logger
	metrics observability.MetricsClient
}

// RerankStage represents a single stage in multi-stage reranking
type RerankStage struct {
	Reranker Reranker
	TopK     int     // How many to pass to next stage
	Weight   float64 // Weight for this stage's scores
}

// NewMultiStageReranker creates a new multi-stage reranker
func NewMultiStageReranker(stages []RerankStage, logger observability.Logger) *MultiStageReranker {
	if logger == nil {
		logger = observability.NewLogger("rerank.multistage")
	}
	return &MultiStageReranker{
		stages:  stages,
		logger:  logger,
		metrics: observability.NewMetricsClient(),
	}
}

// Rerank applies multiple reranking stages in sequence
func (m *MultiStageReranker) Rerank(ctx context.Context, query string, results []SearchResult, opts *RerankOptions) ([]SearchResult, error) {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "rerank.multistage")
	defer span.End()

	span.SetAttribute("num_stages", len(m.stages))
	span.SetAttribute("input_count", len(results))

	start := time.Now()
	defer func() {
		m.metrics.RecordHistogram("rerank.multistage.duration", time.Since(start).Seconds(), nil)
	}()

	currentResults := results

	for i, stage := range m.stages {
		stageStart := time.Now()

		m.logger.Debug("Executing rerank stage", map[string]interface{}{
			"stage":    i,
			"reranker": stage.Reranker.GetName(),
			"input":    len(currentResults),
			"top_k":    stage.TopK,
		})

		// Apply reranker
		stageOpts := *opts
		if stage.TopK > 0 {
			stageOpts.TopK = stage.TopK
		}

		reranked, err := stage.Reranker.Rerank(ctx, query, currentResults, &stageOpts)
		if err != nil {
			m.logger.Error("Rerank stage failed", map[string]interface{}{
				"stage":    i,
				"reranker": stage.Reranker.GetName(),
				"error":    err.Error(),
			})
			return nil, fmt.Errorf("rerank stage %d (%s) failed: %w", i, stage.Reranker.GetName(), err)
		}

		// Track metrics
		m.metrics.RecordHistogram(
			fmt.Sprintf("rerank.stage.%s.duration", stage.Reranker.GetName()),
			time.Since(stageStart).Seconds(),
			map[string]string{"stage": fmt.Sprintf("%d", i)},
		)

		m.logger.Debug("Rerank stage completed", map[string]interface{}{
			"stage":    i,
			"reranker": stage.Reranker.GetName(),
			"output":   len(reranked),
			"duration": time.Since(stageStart),
		})

		currentResults = reranked
	}

	span.SetAttribute("output_count", len(currentResults))
	return currentResults, nil
}

// GetName returns the name of the multi-stage reranker
func (m *MultiStageReranker) GetName() string {
	return "multistage"
}

// Close cleans up resources for all stages
func (m *MultiStageReranker) Close() error {
	for _, stage := range m.stages {
		if err := stage.Reranker.Close(); err != nil {
			m.logger.Warn("Failed to close reranker", map[string]interface{}{
				"reranker": stage.Reranker.GetName(),
				"error":    err.Error(),
			})
		}
	}
	return nil
}
