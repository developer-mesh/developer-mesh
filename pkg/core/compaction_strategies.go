// Story 4.1: Compaction Strategies
// Package core provides core business logic for DevMesh platform

package core

import (
	"context"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
)

// CompactionExecutor handles different compaction strategies
type CompactionExecutor struct {
	contextRepo   repository.ContextRepository
	embeddingRepo repository.VectorAPIRepository
	logger        observability.Logger
}

// NewCompactionExecutor creates a new compaction executor
func NewCompactionExecutor(
	contextRepo repository.ContextRepository,
	embeddingRepo repository.VectorAPIRepository,
	logger observability.Logger,
) *CompactionExecutor {
	return &CompactionExecutor{
		contextRepo:   contextRepo,
		embeddingRepo: embeddingRepo,
		logger:        logger,
	}
}

// ExecuteCompaction runs the specified compaction strategy
func (e *CompactionExecutor) ExecuteCompaction(
	ctx context.Context,
	contextID string,
	strategy repository.CompactionStrategy,
) error {
	switch strategy {
	case repository.CompactionToolClear:
		return e.compactToolClear(ctx, contextID)
	case repository.CompactionPrune:
		return e.compactPrune(ctx, contextID)
	case repository.CompactionSliding:
		return e.compactSliding(ctx, contextID)
	case repository.CompactionSummarize:
		return e.compactSummarize(ctx, contextID)
	default:
		return fmt.Errorf("unknown compaction strategy: %s", strategy)
	}
}

// compactToolClear removes tool execution results
func (e *CompactionExecutor) compactToolClear(ctx context.Context, contextID string) error {
	items, err := e.contextRepo.GetContextItems(ctx, contextID)
	if err != nil {
		return fmt.Errorf("failed to get context items: %w", err)
	}

	compactedCount := 0
	for _, item := range items {
		// Check if this is a tool result
		if item.Type == "tool_result" || item.Type == "function_result" {
			// Check if it's old enough to clear (> 10 messages ago)
			if isOldToolResult(item, items) {
				// Mark as compacted
				if item.Metadata == nil {
					item.Metadata = make(map[string]interface{})
				}
				item.Metadata["compacted"] = true
				item.Metadata["original_content_length"] = len(item.Content)

				// Clear the content but keep metadata
				toolName := "unknown"
				if name, ok := item.Metadata["tool_name"].(string); ok {
					toolName = name
				}
				item.Content = fmt.Sprintf("[Tool result cleared: %s]", toolName)

				if err := e.contextRepo.UpdateContextItem(ctx, item); err != nil {
					e.logger.Warn("Failed to compact tool result", map[string]interface{}{
						"item_id": item.ID,
						"error":   err.Error(),
					})
				} else {
					compactedCount++
				}
			}
		}
	}

	// Update compaction metadata
	if err := e.contextRepo.UpdateCompactionMetadata(ctx, contextID, "tool_clear", time.Now()); err != nil {
		return fmt.Errorf("failed to update compaction metadata: %w", err)
	}

	e.logger.Info("Tool clear compaction completed", map[string]interface{}{
		"context_id":      contextID,
		"items_compacted": compactedCount,
	})

	return nil
}

// compactPrune removes low-importance items
func (e *CompactionExecutor) compactPrune(ctx context.Context, contextID string) error {
	items, err := e.contextRepo.GetContextItems(ctx, contextID)
	if err != nil {
		return fmt.Errorf("failed to get context items: %w", err)
	}

	// Get embeddings to check importance scores
	links, err := e.contextRepo.GetContextEmbeddingLinks(ctx, contextID)
	if err != nil {
		return fmt.Errorf("failed to get embedding links: %w", err)
	}

	// Create importance map
	importanceMap := make(map[string]float64)
	for _, link := range links {
		importanceMap[link.EmbeddingID] = link.ImportanceScore
	}

	prunedCount := 0
	for _, item := range items {
		importance := 0.5 // Default importance

		// Check if we have importance score
		if item.Metadata != nil {
			if embeddingID, ok := item.Metadata["embedding_id"].(string); ok {
				if score, exists := importanceMap[embeddingID]; exists {
					importance = score
				}
			}
		}

		// Prune if importance is below threshold
		if importance < 0.3 && !isProtectedItem(item) {
			// Delete the item
			if err := e.contextRepo.Delete(ctx, item.ID); err != nil {
				e.logger.Warn("Failed to prune item", map[string]interface{}{
					"item_id": item.ID,
					"error":   err.Error(),
				})
			} else {
				prunedCount++
			}
		}
	}

	if err := e.contextRepo.UpdateCompactionMetadata(ctx, contextID, "prune", time.Now()); err != nil {
		return fmt.Errorf("failed to update compaction metadata: %w", err)
	}

	e.logger.Info("Prune compaction completed", map[string]interface{}{
		"context_id":   contextID,
		"items_pruned": prunedCount,
	})

	return nil
}

// compactSliding implements sliding window compaction
func (e *CompactionExecutor) compactSliding(ctx context.Context, contextID string) error {
	items, err := e.contextRepo.GetContextItems(ctx, contextID)
	if err != nil {
		return fmt.Errorf("failed to get context items: %w", err)
	}

	const recentWindowSize = 20 // Keep last 20 items in full

	if len(items) <= recentWindowSize {
		return nil // Nothing to compact
	}

	// Items to compact are those outside the recent window
	itemsToCompact := items[:len(items)-recentWindowSize]

	for _, item := range itemsToCompact {
		// Create summary metadata
		if item.Metadata == nil {
			item.Metadata = make(map[string]interface{})
		}
		item.Metadata["compacted"] = true
		item.Metadata["compaction_strategy"] = "sliding"
		item.Metadata["original_length"] = len(item.Content)

		// Keep only first 100 characters
		if len(item.Content) > 100 {
			item.Content = item.Content[:100] + "..."
		}

		if err := e.contextRepo.UpdateContextItem(ctx, item); err != nil {
			e.logger.Warn("Failed to compact item", map[string]interface{}{
				"item_id": item.ID,
				"error":   err.Error(),
			})
		}
	}

	if err := e.contextRepo.UpdateCompactionMetadata(ctx, contextID, "sliding", time.Now()); err != nil {
		return fmt.Errorf("failed to update compaction metadata: %w", err)
	}

	e.logger.Info("Sliding window compaction completed", map[string]interface{}{
		"context_id":         contextID,
		"items_compacted":    len(itemsToCompact),
		"recent_window_size": recentWindowSize,
	})

	return nil
}

// compactSummarize uses LLM to summarize (placeholder - requires LLM integration)
func (e *CompactionExecutor) compactSummarize(ctx context.Context, contextID string) error {
	// This will be implemented when LLM service is integrated
	// For now, just update metadata
	if err := e.contextRepo.UpdateCompactionMetadata(ctx, contextID, "summarize", time.Now()); err != nil {
		return fmt.Errorf("failed to update compaction metadata: %w", err)
	}

	e.logger.Info("Summarize compaction placeholder", map[string]interface{}{
		"context_id": contextID,
		"note":       "LLM summarization not yet implemented",
	})

	return nil
}

// Helper functions

// isOldToolResult checks if a tool result is old enough to clear
func isOldToolResult(item *repository.ContextItem, allItems []*repository.ContextItem) bool {
	// Find position of this item
	position := -1
	for i, it := range allItems {
		if it.ID == item.ID {
			position = i
			break
		}
	}

	// Consider old if more than 10 items after it
	return position >= 0 && len(allItems)-position > 10
}

// isProtectedItem checks if an item should be protected from pruning
func isProtectedItem(item *repository.ContextItem) bool {
	// Never prune errors or critical items
	if item.Type == "error" {
		return true
	}

	if item.Metadata != nil {
		if critical, ok := item.Metadata["is_critical"].(bool); ok && critical {
			return true
		}

		// Protect recent items (last hour)
		if createdAt, ok := item.Metadata["created_at"].(time.Time); ok {
			if time.Since(createdAt) < time.Hour {
				return true
			}
		}
	}

	return false
}
