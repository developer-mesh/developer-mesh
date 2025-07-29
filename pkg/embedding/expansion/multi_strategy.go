package expansion

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// MultiStrategyExpander combines multiple expansion strategies
type MultiStrategyExpander struct {
	strategies map[ExpansionType]QueryExpander
	config     *Config
	logger     observability.Logger
	metrics    observability.MetricsClient
}

// Config for multi-strategy expander
type Config struct {
	DefaultMaxExpansions int
	EnabledStrategies    []ExpansionType
	Timeout              time.Duration
	CacheEnabled         bool
	CacheTTL             time.Duration
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultMaxExpansions: 10,
		EnabledStrategies: []ExpansionType{
			ExpansionTypeSynonym,
			ExpansionTypeHyDE,
			ExpansionTypeDecompose,
		},
		Timeout:      5 * time.Second,
		CacheEnabled: true,
		CacheTTL:     1 * time.Hour,
	}
}

// NewMultiStrategyExpander creates a new multi-strategy expander
func NewMultiStrategyExpander(llmClient LLMClient, config *Config, logger observability.Logger) *MultiStrategyExpander {
	if config == nil {
		config = DefaultConfig()
	}
	if logger == nil {
		logger = observability.NewLogger("expansion.multi")
	}

	expander := &MultiStrategyExpander{
		strategies: make(map[ExpansionType]QueryExpander),
		config:     config,
		logger:     logger,
		metrics:    observability.NewMetricsClient(),
	}

	// Initialize strategies
	expander.strategies[ExpansionTypeSynonym] = NewSynonymExpander(llmClient, logger)
	expander.strategies[ExpansionTypeHyDE] = NewHyDEExpander(llmClient, logger)
	expander.strategies[ExpansionTypeDecompose] = NewDecompositionExpander(llmClient, logger)

	return expander
}

// Expand applies multiple expansion strategies
func (m *MultiStrategyExpander) Expand(ctx context.Context, query string, opts *ExpansionOptions) (*ExpandedQuery, error) {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "expansion.multi_strategy")
	defer span.End()

	span.SetAttribute("query", query)

	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	start := time.Now()
	defer func() {
		m.metrics.RecordHistogram("expansion.multi.duration", time.Since(start).Seconds(), nil)
	}()

	if err := ValidateQuery(query); err != nil {
		m.metrics.IncrementCounter("expansion.multi.error", 1.0)
		return nil, err
	}

	// Set defaults if not provided
	if opts == nil {
		opts = &ExpansionOptions{
			MaxExpansions:   m.config.DefaultMaxExpansions,
			IncludeOriginal: true,
			ExpansionTypes:  m.config.EnabledStrategies,
		}
	}

	expanded := &ExpandedQuery{
		Original:   query,
		Expansions: []QueryVariation{},
	}

	// Include original query if requested
	if opts.IncludeOriginal {
		expanded.Expansions = append(expanded.Expansions, QueryVariation{
			Text:   query,
			Type:   "original",
			Weight: 1.0,
			Metadata: map[string]interface{}{
				"is_original": true,
			},
		})
	}

	// Apply strategies in parallel
	type strategyResult struct {
		expansions []QueryVariation
		strategy   ExpansionType
		err        error
	}

	resultChan := make(chan strategyResult, len(opts.ExpansionTypes))
	var wg sync.WaitGroup

	for _, strategyType := range opts.ExpansionTypes {
		strategy, ok := m.strategies[strategyType]
		if !ok {
			m.logger.Warn("Unknown strategy type", map[string]interface{}{
				"strategy": strategyType,
			})
			continue
		}

		wg.Add(1)
		go func(st ExpansionType, s QueryExpander) {
			defer wg.Done()

			strategyStart := time.Now()
			strategyExpanded, err := s.Expand(ctx, query, opts)

			duration := time.Since(strategyStart).Seconds()
			m.metrics.RecordHistogram(fmt.Sprintf("expansion.%s.duration", st), duration, nil)

			if err != nil {
				m.logger.Error("Strategy expansion failed", map[string]interface{}{
					"strategy": st,
					"error":    err.Error(),
					"duration": duration,
				})
				m.metrics.IncrementCounter(fmt.Sprintf("expansion.%s.error", st), 1.0)
				resultChan <- strategyResult{strategy: st, err: err}
				return
			}

			m.metrics.IncrementCounter(fmt.Sprintf("expansion.%s.success", st), 1.0)
			resultChan <- strategyResult{
				expansions: strategyExpanded.Expansions,
				strategy:   st,
			}
		}(strategyType, strategy)
	}

	// Wait for all strategies to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	strategyExpansions := make(map[ExpansionType][]QueryVariation)
	for result := range resultChan {
		if result.err == nil && len(result.expansions) > 0 {
			strategyExpansions[result.strategy] = result.expansions
		}
	}

	// Merge expansions with proper weighting
	mergedExpansions := m.mergeExpansions(strategyExpansions)
	expanded.Expansions = append(expanded.Expansions, mergedExpansions...)

	// Deduplicate
	expanded.Expansions = m.deduplicateExpansions(expanded.Expansions)

	// Apply limit if specified
	if opts.MaxExpansions > 0 && len(expanded.Expansions) > opts.MaxExpansions {
		// Sort by weight and take top N
		sort.Slice(expanded.Expansions, func(i, j int) bool {
			return expanded.Expansions[i].Weight > expanded.Expansions[j].Weight
		})
		expanded.Expansions = expanded.Expansions[:opts.MaxExpansions]
	}

	span.SetAttribute("total_expansions", len(expanded.Expansions))
	m.metrics.RecordHistogram("expansion.multi.result_count", float64(len(expanded.Expansions)), nil)

	m.logger.Info("Multi-strategy expansion completed", map[string]interface{}{
		"query":            query,
		"strategies_used":  len(strategyExpansions),
		"total_expansions": len(expanded.Expansions),
	})

	return expanded, nil
}

// mergeExpansions combines expansions from different strategies
func (m *MultiStrategyExpander) mergeExpansions(strategyExpansions map[ExpansionType][]QueryVariation) []QueryVariation {
	var merged []QueryVariation

	// Strategy weights (can be made configurable)
	strategyWeights := map[ExpansionType]float32{
		ExpansionTypeSynonym:   1.0,
		ExpansionTypeHyDE:      0.7,
		ExpansionTypeDecompose: 0.8,
	}

	for strategy, expansions := range strategyExpansions {
		weight, exists := strategyWeights[strategy]
		if !exists {
			weight = 0.5
		}

		for _, exp := range expansions {
			// Apply strategy weight to expansion weight
			exp.Weight *= weight
			merged = append(merged, exp)
		}
	}

	return merged
}

// deduplicateExpansions removes duplicate expansions, keeping highest weight
func (m *MultiStrategyExpander) deduplicateExpansions(expansions []QueryVariation) []QueryVariation {
	// Map to track best weight for each unique text
	bestByText := make(map[string]QueryVariation)

	for _, exp := range expansions {
		key := normalizeText(exp.Text)
		if existing, exists := bestByText[key]; exists {
			// Keep the one with higher weight
			if exp.Weight > existing.Weight {
				bestByText[key] = exp
			}
		} else {
			bestByText[key] = exp
		}
	}

	// Convert back to slice
	deduped := make([]QueryVariation, 0, len(bestByText))
	for _, exp := range bestByText {
		deduped = append(deduped, exp)
	}

	return deduped
}

// normalizeText normalizes text for deduplication
func normalizeText(text string) string {
	// Simple normalization - can be enhanced
	return strings.ToLower(strings.TrimSpace(text))
}
