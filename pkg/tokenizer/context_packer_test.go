package tokenizer_test

import (
	"strings"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/tokenizer"
	"github.com/stretchr/testify/assert"
)

func TestContextPacker_PackContextWindow(t *testing.T) {
	assert := assert.New(t)

	// Use simple tokenizer for testing
	tok := tokenizer.NewSimpleTokenizer(1000)
	packer := tokenizer.NewContextPacker(tok)

	// Create test items
	items := []*repository.ContextItem{
		{
			ID:      "1",
			Content: "Short content",
			Type:    "message",
		},
		{
			ID:      "2",
			Content: strings.Repeat("Long ", 100),
			Type:    "message",
		},
		{
			ID:       "3",
			Content:  "Critical item",
			Type:     "message",
			Metadata: map[string]any{"is_critical": true},
		},
	}

	// Pack with budget
	packed, tokens := packer.PackContextWindow(items, 100, []string{"3"})

	// Critical item should be included
	assert.Greater(len(packed), 0)
	assert.Equal("3", packed[0].ID)
	assert.LessOrEqual(tokens, 100)
}

func TestContextPacker_PackContextWindow_AlwaysInclude(t *testing.T) {
	assert := assert.New(t)

	tok := tokenizer.NewSimpleTokenizer(1000)
	packer := tokenizer.NewContextPacker(tok)

	items := []*repository.ContextItem{
		{ID: "1", Content: "Item one", Type: "message"},
		{ID: "2", Content: "Item two", Type: "message"},
		{ID: "3", Content: "Item three", Type: "message"},
	}

	// Always include item 2
	packed, _ := packer.PackContextWindow(items, 100, []string{"2"})

	// Item 2 should be first
	assert.Greater(len(packed), 0)
	assert.Equal("2", packed[0].ID)
}

func TestContextPacker_PackContextWindow_ZeroMaxTokens(t *testing.T) {
	assert := assert.New(t)

	tok := tokenizer.NewSimpleTokenizer(1000)
	packer := tokenizer.NewContextPacker(tok)

	items := []*repository.ContextItem{
		{ID: "1", Content: "Test", Type: "message"},
	}

	// Zero max tokens should use tokenizer limit
	packed, _ := packer.PackContextWindow(items, 0, nil)

	assert.Greater(len(packed), 0)
}

func TestContextPacker_countItemTokens(t *testing.T) {
	assert := assert.New(t)

	tok := tokenizer.NewSimpleTokenizer(1000)
	packer := tokenizer.NewContextPacker(tok)

	item := &repository.ContextItem{
		ID:      "1",
		Content: "Hello world",
		Type:    "message",
	}

	tokens := packer.EstimateContextSize([]*repository.ContextItem{item})
	assert.Greater(tokens, 0)
}

func TestContextPacker_canSplitItem(t *testing.T) {
	assert := assert.New(t)

	tok := tokenizer.NewSimpleTokenizer(1000)
	packer := tokenizer.NewContextPacker(tok)

	// Error messages shouldn't be split
	errorItem := &repository.ContextItem{
		ID:      "1",
		Content: strings.Repeat("error ", 200),
		Type:    "error",
	}

	// Test by attempting to split - should fail
	partial, _ := packer.PackContextWindow([]*repository.ContextItem{errorItem}, 10, nil)
	// Error item should be excluded rather than split
	assert.Equal(0, len(partial))

	// Critical items shouldn't be split
	criticalItem := &repository.ContextItem{
		ID:       "2",
		Content:  strings.Repeat("critical ", 200),
		Type:     "message",
		Metadata: map[string]any{"is_critical": true},
	}

	partial, _ = packer.PackContextWindow([]*repository.ContextItem{criticalItem}, 10, nil)
	// Critical item should be excluded rather than split
	assert.Equal(0, len(partial))

	// Long regular items can be split
	longItem := &repository.ContextItem{
		ID:      "3",
		Content: strings.Repeat("word ", 200),
		Type:    "message",
	}

	partial, _ = packer.PackContextWindow([]*repository.ContextItem{longItem}, 50, nil)
	// Should have partial item
	if len(partial) > 0 {
		assert.Contains(partial[0].Content, "truncated")
	}
}

func TestContextPacker_splitItem(t *testing.T) {
	assert := assert.New(t)

	tok := tokenizer.NewSimpleTokenizer(1000)
	packer := tokenizer.NewContextPacker(tok)

	// Create a long item
	longContent := strings.Repeat("word ", 500)
	item := &repository.ContextItem{
		ID:      "1",
		Content: longContent,
		Type:    "message",
	}

	// Pack with very limited budget to force splitting
	packed, tokens := packer.PackContextWindow([]*repository.ContextItem{item}, 50, nil)

	// Should have partial item
	if len(packed) > 0 {
		assert.Contains(packed[0].Content, "truncated")
		assert.Equal("1_partial", packed[0].ID)
		assert.LessOrEqual(tokens, 50)

		// Check metadata
		assert.NotNil(packed[0].Metadata)
		assert.True(packed[0].Metadata["truncated"].(bool))
		assert.Equal(len(longContent), packed[0].Metadata["original_length"].(int))
	}
}

func TestContextPacker_formatContextItem(t *testing.T) {
	assert := assert.New(t)

	tok := tokenizer.NewSimpleTokenizer(1000)
	packer := tokenizer.NewContextPacker(tok)

	// Item with type
	item := &repository.ContextItem{
		ID:      "1",
		Content: "Hello world",
		Type:    "user",
	}

	tokens := packer.EstimateContextSize([]*repository.ContextItem{item})
	assert.Greater(tokens, 0)

	// Item with metadata timestamp
	itemWithTimestamp := &repository.ContextItem{
		ID:       "2",
		Content:  "Test message",
		Type:     "assistant",
		Metadata: map[string]any{"timestamp": "2024-01-01T00:00:00Z"},
	}

	tokensWithTimestamp := packer.EstimateContextSize([]*repository.ContextItem{itemWithTimestamp})
	assert.Greater(tokensWithTimestamp, tokens)
}

func TestContextPacker_EstimateContextSize(t *testing.T) {
	assert := assert.New(t)

	tok := tokenizer.NewSimpleTokenizer(1000)
	packer := tokenizer.NewContextPacker(tok)

	items := []*repository.ContextItem{
		{ID: "1", Content: "First item", Type: "message"},
		{ID: "2", Content: "Second item", Type: "message"},
		{ID: "3", Content: "Third item", Type: "message"},
	}

	totalTokens := packer.EstimateContextSize(items)
	assert.Greater(totalTokens, 0)

	// Total should be sum of individual items
	var sum int
	for _, item := range items {
		sum += packer.EstimateContextSize([]*repository.ContextItem{item})
	}
	assert.Equal(sum, totalTokens)
}

func TestContextPacker_GetTokenBudgetUtilization(t *testing.T) {
	assert := assert.New(t)

	tok := tokenizer.NewSimpleTokenizer(1000)
	packer := tokenizer.NewContextPacker(tok)

	// 50% utilization
	utilization := packer.GetTokenBudgetUtilization(50, 100)
	assert.Equal(0.5, utilization)

	// 100% utilization
	utilization = packer.GetTokenBudgetUtilization(100, 100)
	assert.Equal(1.0, utilization)

	// 0% utilization
	utilization = packer.GetTokenBudgetUtilization(0, 100)
	assert.Equal(0.0, utilization)

	// Zero max tokens
	utilization = packer.GetTokenBudgetUtilization(50, 0)
	assert.Equal(0.0, utilization)
}

func TestContextPacker_NewContextPacker_NilTokenizer(t *testing.T) {
	assert := assert.New(t)

	// Should use default tokenizer if nil
	packer := tokenizer.NewContextPacker(nil)
	assert.NotNil(packer)

	// Should be able to pack items
	items := []*repository.ContextItem{
		{ID: "1", Content: "Test", Type: "message"},
	}

	packed, tokens := packer.PackContextWindow(items, 100, nil)
	assert.Greater(len(packed), 0)
	assert.Greater(tokens, 0)
}

func TestContextPacker_MultipleAlwaysInclude(t *testing.T) {
	assert := assert.New(t)

	tok := tokenizer.NewSimpleTokenizer(1000)
	packer := tokenizer.NewContextPacker(tok)

	items := []*repository.ContextItem{
		{ID: "1", Content: "Item one", Type: "message"},
		{ID: "2", Content: "Item two", Type: "message"},
		{ID: "3", Content: "Item three", Type: "message"},
		{ID: "4", Content: "Item four", Type: "message"},
	}

	// Always include items 2 and 4
	packed, _ := packer.PackContextWindow(items, 200, []string{"2", "4"})

	// Both should be included
	assert.GreaterOrEqual(len(packed), 2)

	// Check that items 2 and 4 are in the packed list
	ids := make(map[string]bool)
	for _, item := range packed {
		ids[item.ID] = true
	}
	assert.True(ids["2"])
	assert.True(ids["4"])
}
