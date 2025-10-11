// Story 3.2: Token Counting and Context Packing
package tokenizer

import (
	"fmt"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/repository"
)

// ContextPacker packs context items within token budget
type ContextPacker struct {
	tokenizer Tokenizer
}

// NewContextPacker creates a new context packer
func NewContextPacker(tokenizer Tokenizer) *ContextPacker {
	if tokenizer == nil {
		// Use default simple tokenizer if none provided
		tokenizer = NewSimpleTokenizer(8192)
	}
	return &ContextPacker{
		tokenizer: tokenizer,
	}
}

// PackContextWindow packs items into token budget
func (p *ContextPacker) PackContextWindow(
	rankedItems []*repository.ContextItem,
	maxTokens int,
	alwaysInclude []string,
) ([]*repository.ContextItem, int) {
	if maxTokens <= 0 {
		maxTokens = p.tokenizer.GetTokenLimit()
	}

	packed := make([]*repository.ContextItem, 0)
	currentTokens := 0

	// Create map for quick lookup of always-include items
	alwaysIncludeMap := make(map[string]bool)
	for _, id := range alwaysInclude {
		alwaysIncludeMap[id] = true
	}

	// First pass: add always-include items
	for _, item := range rankedItems {
		if alwaysIncludeMap[item.ID] {
			tokens := p.countItemTokens(item)
			if currentTokens+tokens <= maxTokens {
				packed = append(packed, item)
				currentTokens += tokens
				delete(alwaysIncludeMap, item.ID)
			}
		}
	}

	// Second pass: add remaining items by rank
	for _, item := range rankedItems {
		// Skip if already packed
		alreadyPacked := false
		for _, packedItem := range packed {
			if packedItem.ID == item.ID {
				alreadyPacked = true
				break
			}
		}
		if alreadyPacked {
			continue
		}

		// Calculate tokens for this item
		tokens := p.countItemTokens(item)

		// Check if it fits
		if currentTokens+tokens <= maxTokens {
			packed = append(packed, item)
			currentTokens += tokens
		} else {
			// Try to fit partial content if possible
			if p.canSplitItem(item) {
				partialItem, partialTokens := p.splitItem(item, maxTokens-currentTokens)
				if partialItem != nil {
					packed = append(packed, partialItem)
					currentTokens += partialTokens
				}
			}
			break // No more space
		}
	}

	return packed, currentTokens
}

// countItemTokens counts tokens in a context item
func (p *ContextPacker) countItemTokens(item *repository.ContextItem) int {
	// Format as it would appear in context
	formatted := p.formatContextItem(item)

	// Use existing tokenizer
	return p.tokenizer.CountTokens(formatted)
}

// formatContextItem formats item for context
func (p *ContextPacker) formatContextItem(item *repository.ContextItem) string {
	var parts []string

	// Add role/type prefix
	if item.Type != "" {
		parts = append(parts, fmt.Sprintf("[%s]", item.Type))
	}

	// Add content
	parts = append(parts, item.Content)

	// Add important metadata
	if item.Metadata != nil {
		if timestamp, ok := item.Metadata["timestamp"].(string); ok {
			parts = append(parts, fmt.Sprintf("(at %s)", timestamp))
		}
	}

	return strings.Join(parts, " ")
}

// canSplitItem checks if item can be split
func (p *ContextPacker) canSplitItem(item *repository.ContextItem) bool {
	// Don't split error messages or critical items
	if item.Type == "error" {
		return false
	}

	if item.Metadata != nil {
		if critical, ok := item.Metadata["is_critical"].(bool); ok && critical {
			return false
		}
	}

	// Only split if content is long enough
	return len(item.Content) > 500
}

// splitItem splits an item to fit token budget
func (p *ContextPacker) splitItem(item *repository.ContextItem, maxTokens int) (*repository.ContextItem, int) {
	// Try different split ratios
	ratios := []float64{0.75, 0.5, 0.25}

	for _, ratio := range ratios {
		splitPoint := int(float64(len(item.Content)) * ratio)
		truncated := item.Content[:splitPoint] + "... [truncated]"

		// Create partial item with copy of metadata
		metadataCopy := make(map[string]any)
		if item.Metadata != nil {
			for k, v := range item.Metadata {
				metadataCopy[k] = v
			}
		}
		metadataCopy["truncated"] = true
		metadataCopy["original_length"] = len(item.Content)

		partialItem := &repository.ContextItem{
			ID:        item.ID + "_partial",
			ContextID: item.ContextID,
			Content:   truncated,
			Type:      item.Type,
			Score:     item.Score,
			Metadata:  metadataCopy,
		}

		tokens := p.countItemTokens(partialItem)
		if tokens <= maxTokens {
			return partialItem, tokens
		}
	}

	return nil, 0
}

// EstimateContextSize estimates total tokens for a set of items
func (p *ContextPacker) EstimateContextSize(items []*repository.ContextItem) int {
	total := 0
	for _, item := range items {
		total += p.countItemTokens(item)
	}
	return total
}

// GetTokenBudgetUtilization returns the ratio of used tokens to max tokens
func (p *ContextPacker) GetTokenBudgetUtilization(usedTokens, maxTokens int) float64 {
	if maxTokens <= 0 {
		return 0.0
	}
	return float64(usedTokens) / float64(maxTokens)
}
