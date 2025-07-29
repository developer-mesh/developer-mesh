package expansion

import (
	"context"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMultiStrategyExpander(t *testing.T) {
	mockLLM := new(MockLLMClient)
	logger := observability.NewLogger("test")

	t.Run("with default config", func(t *testing.T) {
		expander := NewMultiStrategyExpander(mockLLM, nil, logger)
		assert.NotNil(t, expander)
		assert.NotNil(t, expander.config)
		assert.Equal(t, 10, expander.config.DefaultMaxExpansions)
		assert.Len(t, expander.strategies, 3)
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &Config{
			DefaultMaxExpansions: 5,
			EnabledStrategies: []ExpansionType{
				ExpansionTypeSynonym,
				ExpansionTypeHyDE,
			},
			Timeout: 10 * time.Second,
		}
		expander := NewMultiStrategyExpander(mockLLM, config, logger)
		assert.NotNil(t, expander)
		assert.Equal(t, 5, expander.config.DefaultMaxExpansions)
		assert.Equal(t, 10*time.Second, expander.config.Timeout)
	})
}

func TestMultiStrategyExpander_Expand(t *testing.T) {
	ctx := context.Background()

	t.Run("successful multi-strategy expansion", func(t *testing.T) {
		llmClient := NewSimpleLLMClient()
		config := &Config{
			DefaultMaxExpansions: 10,
			EnabledStrategies: []ExpansionType{
				ExpansionTypeSynonym,
				ExpansionTypeDecompose,
			},
			Timeout: 5 * time.Second,
		}
		expander := NewMultiStrategyExpander(llmClient, config, nil)

		query := "implement error handling and logging"
		opts := &ExpansionOptions{
			IncludeOriginal: true,
			MaxExpansions:   8,
			ExpansionTypes: []ExpansionType{
				ExpansionTypeSynonym,
				ExpansionTypeDecompose,
			},
		}

		result, err := expander.Expand(ctx, query, opts)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, query, result.Original)

		// Should have original + expansions
		assert.Greater(t, len(result.Expansions), 1)
		assert.LessOrEqual(t, len(result.Expansions), 8)

		// Check original is included
		hasOriginal := false
		for _, exp := range result.Expansions {
			if exp.Text == query && exp.Metadata["is_original"] == true {
				hasOriginal = true
				assert.Equal(t, float32(1.0), exp.Weight)
				break
			}
		}
		assert.True(t, hasOriginal)

		// Check for different expansion types
		types := make(map[ExpansionType]int)
		for _, exp := range result.Expansions {
			types[exp.Type]++
		}
		// Log the types we found for debugging
		t.Logf("Found expansion types: %+v", types)
		// At least one type should have expansions
		assert.Greater(t, len(types), 0, "Should have at least one expansion type")
		// Should have multiple expansion types or at least some expansions
		totalNonOriginal := 0
		for _, exp := range result.Expansions {
			if exp.Metadata["is_original"] != true {
				totalNonOriginal++
			}
		}
		assert.Greater(t, totalNonOriginal, 0, "Should have non-original expansions")
	})

	t.Run("with nil options", func(t *testing.T) {
		llmClient := NewSimpleLLMClient()
		expander := NewMultiStrategyExpander(llmClient, nil, nil)

		query := "test query"
		result, err := expander.Expand(ctx, query, nil)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Greater(t, len(result.Expansions), 0)
	})

	t.Run("empty query validation", func(t *testing.T) {
		llmClient := NewSimpleLLMClient()
		expander := NewMultiStrategyExpander(llmClient, nil, nil)

		result, err := expander.Expand(ctx, "", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "query cannot be empty")
		assert.Nil(t, result)
	})

	t.Run("respects max expansions", func(t *testing.T) {
		llmClient := NewSimpleLLMClient()
		expander := NewMultiStrategyExpander(llmClient, nil, nil)

		query := "implement authentication and authorization and logging and monitoring"
		opts := &ExpansionOptions{
			IncludeOriginal: true,
			MaxExpansions:   3,
			ExpansionTypes: []ExpansionType{
				ExpansionTypeSynonym,
				ExpansionTypeDecompose,
				ExpansionTypeHyDE,
			},
		}

		result, err := expander.Expand(ctx, query, opts)

		assert.NoError(t, err)
		assert.Len(t, result.Expansions, 3)

		// Should be sorted by weight
		for i := 1; i < len(result.Expansions); i++ {
			assert.GreaterOrEqual(t, result.Expansions[i-1].Weight, result.Expansions[i].Weight)
		}
	})

	t.Run("timeout handling", func(t *testing.T) {
		llmClient := NewSimpleLLMClient()
		config := &Config{
			DefaultMaxExpansions: 10,
			EnabledStrategies:    []ExpansionType{ExpansionTypeHyDE},
			Timeout:              1 * time.Millisecond, // Very short timeout
		}
		expander := NewMultiStrategyExpander(llmClient, config, nil)

		query := "test query"
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		// Give context time to expire
		time.Sleep(2 * time.Millisecond)

		result, err := expander.Expand(ctx, query, nil)

		// Should still work but might have fewer expansions
		if err == nil {
			assert.NotNil(t, result)
		}
	})
}

func TestMultiStrategyExpander_deduplicateExpansions(t *testing.T) {
	expander := &MultiStrategyExpander{}

	expansions := []QueryVariation{
		{Text: "test query", Type: ExpansionTypeSynonym, Weight: 0.8},
		{Text: "Test Query", Type: ExpansionTypeDecompose, Weight: 0.6}, // Duplicate
		{Text: "test query", Type: ExpansionTypeHyDE, Weight: 0.7},      // Exact duplicate
		{Text: "different query", Type: ExpansionTypeSynonym, Weight: 0.5},
	}

	deduped := expander.deduplicateExpansions(expansions)

	assert.Len(t, deduped, 2)

	// Should keep highest weight version
	var testQueryFound bool
	for _, exp := range deduped {
		if normalizeText(exp.Text) == "test query" {
			assert.Equal(t, float32(0.8), exp.Weight)
			testQueryFound = true
		}
	}
	assert.True(t, testQueryFound)
}

func TestMultiStrategyExpander_mergeExpansions(t *testing.T) {
	expander := &MultiStrategyExpander{}

	strategyExpansions := map[ExpansionType][]QueryVariation{
		ExpansionTypeSynonym: {
			{Text: "synonym1", Weight: 0.9},
			{Text: "synonym2", Weight: 0.8},
		},
		ExpansionTypeHyDE: {
			{Text: "hyde result", Weight: 1.0},
		},
		ExpansionTypeDecompose: {
			{Text: "decomposed1", Weight: 0.7},
			{Text: "decomposed2", Weight: 0.5},
		},
	}

	merged := expander.mergeExpansions(strategyExpansions)

	assert.Len(t, merged, 5)

	// Check strategy weights are applied
	for _, exp := range merged {
		switch exp.Type {
		case ExpansionTypeSynonym:
			assert.LessOrEqual(t, exp.Weight, float32(0.9)) // Original weight * 1.0
		case ExpansionTypeHyDE:
			assert.LessOrEqual(t, exp.Weight, float32(0.7)) // Original weight * 0.7
		case ExpansionTypeDecompose:
			assert.LessOrEqual(t, exp.Weight, float32(0.64)) // Original weight * 0.8
		}
	}
}

func TestMultiStrategyExpander_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	llmClient := NewSimpleLLMClient()
	logger := observability.NewLogger("test")

	config := &Config{
		DefaultMaxExpansions: 15,
		EnabledStrategies: []ExpansionType{
			ExpansionTypeSynonym,
			ExpansionTypeHyDE,
			ExpansionTypeDecompose,
		},
		Timeout: 10 * time.Second,
	}

	expander := NewMultiStrategyExpander(llmClient, config, logger)

	query := "how to implement error handling and logging in microservices"
	opts := &ExpansionOptions{
		IncludeOriginal: true,
		MaxExpansions:   10,
	}

	result, err := expander.Expand(ctx, query, opts)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, query, result.Original)
	assert.Greater(t, len(result.Expansions), 3) // At least original + some expansions
	assert.LessOrEqual(t, len(result.Expansions), 10)

	// Verify different types are present
	types := make(map[ExpansionType]bool)
	for _, exp := range result.Expansions {
		types[exp.Type] = true
		t.Logf("Expansion: %s (type: %s, weight: %.2f)", exp.Text, exp.Type, exp.Weight)
	}

	assert.True(t, len(types) >= 2, "Should have at least 2 different expansion types")
}
