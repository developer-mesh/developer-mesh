package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/jmoiron/sqlx"
)

// ResolutionLearner learns from successful operation resolutions and improves future matching
// This is completely tool-agnostic and learns patterns for ANY API
type ResolutionLearner struct {
	db            *sqlx.DB
	logger        observability.Logger
	learningCache map[string]*OperationHistory
	cacheMu       sync.RWMutex
}

// NewResolutionLearner creates a new resolution learner
func NewResolutionLearner(db *sqlx.DB, logger observability.Logger) *ResolutionLearner {
	return &ResolutionLearner{
		db:            db,
		logger:        logger,
		learningCache: make(map[string]*OperationHistory),
	}
}

// OperationHistory tracks successful resolutions for learning
type OperationHistory struct {
	ToolID      string                       `json:"tool_id"`
	LastUpdated time.Time                    `json:"last_updated"`
	Resolutions map[string]*ResolutionRecord `json:"resolutions"`
}

// ResolutionRecord records a successful operation resolution
type ResolutionRecord struct {
	Action            string           `json:"action"`
	ResolvedOperation string           `json:"resolved_operation"`
	SuccessCount      int              `json:"success_count"`
	FailureCount      int              `json:"failure_count"`
	LastSuccess       time.Time        `json:"last_success"`
	LastFailure       time.Time        `json:"last_failure,omitempty"`
	AverageLatency    int64            `json:"avg_latency_ms"`
	ContextPatterns   []ContextPattern `json:"context_patterns"`
	ParameterPatterns map[string]int   `json:"parameter_patterns"` // param name -> frequency
	ErrorPatterns     map[string]int   `json:"error_patterns,omitempty"`
}

// ContextPattern captures patterns in successful contexts
type ContextPattern struct {
	Parameters []string  `json:"parameters"`
	Frequency  int       `json:"frequency"`
	LastSeen   time.Time `json:"last_seen"`
}

// RecordResolution records a successful or failed resolution attempt
func (l *ResolutionLearner) RecordResolution(
	ctx context.Context,
	toolID string,
	action string,
	resolvedOperation string,
	context map[string]interface{},
	success bool,
	latencyMs int64,
	errorMsg string,
) error {
	// Load or create history
	history, err := l.loadHistory(ctx, toolID)
	if err != nil {
		l.logger.Warn("Failed to load resolution history", map[string]interface{}{
			"tool_id": toolID,
			"error":   err.Error(),
		})
		history = &OperationHistory{
			ToolID:      toolID,
			LastUpdated: time.Now(),
			Resolutions: make(map[string]*ResolutionRecord),
		}
	}

	// Create key for this resolution
	key := fmt.Sprintf("%s:%s", action, resolvedOperation)

	// Get or create resolution record
	record, exists := history.Resolutions[key]
	if !exists {
		record = &ResolutionRecord{
			Action:            action,
			ResolvedOperation: resolvedOperation,
			ParameterPatterns: make(map[string]int),
			ErrorPatterns:     make(map[string]int),
			ContextPatterns:   []ContextPattern{},
		}
		history.Resolutions[key] = record
	}

	// Update statistics
	if success {
		record.SuccessCount++
		record.LastSuccess = time.Now()

		// Update average latency
		if record.AverageLatency == 0 {
			record.AverageLatency = latencyMs
		} else {
			// Running average
			record.AverageLatency = (record.AverageLatency*(int64(record.SuccessCount-1)) + latencyMs) / int64(record.SuccessCount)
		}

		// Record parameter patterns
		for param := range context {
			// Skip internal parameters
			if !strings.HasPrefix(param, "__") {
				record.ParameterPatterns[param]++
			}
		}

		// Record context pattern
		l.recordContextPattern(record, context)

	} else {
		record.FailureCount++
		record.LastFailure = time.Now()

		// Record error pattern
		if errorMsg != "" {
			// Extract error type from message
			errorType := l.extractErrorType(errorMsg)
			record.ErrorPatterns[errorType]++
		}
	}

	// Update cache
	l.cacheMu.Lock()
	l.learningCache[toolID] = history
	l.cacheMu.Unlock()

	// Persist to database
	return l.saveHistory(ctx, toolID, history)
}

// GetResolutionHints provides hints for resolving an action based on learned patterns
func (l *ResolutionLearner) GetResolutionHints(
	ctx context.Context,
	toolID string,
	action string,
	context map[string]interface{},
) map[string]float64 {
	hints := make(map[string]float64)

	// Load history
	history, err := l.loadHistory(ctx, toolID)
	if err != nil || history == nil {
		return hints
	}

	// Score each known resolution for this action
	for _, record := range history.Resolutions {
		if record.Action != action {
			continue
		}

		// Calculate confidence score based on historical performance
		confidence := l.calculateConfidence(record, context)
		if confidence > 0 {
			hints[record.ResolvedOperation] = confidence
		}
	}

	l.logger.Debug("Generated resolution hints", map[string]interface{}{
		"tool_id":     toolID,
		"action":      action,
		"hints_count": len(hints),
		"hints":       hints,
	})

	return hints
}

// calculateConfidence calculates confidence score for a resolution
func (l *ResolutionLearner) calculateConfidence(record *ResolutionRecord, context map[string]interface{}) float64 {
	confidence := 0.0

	// Base confidence from success ratio
	totalAttempts := record.SuccessCount + record.FailureCount
	if totalAttempts > 0 {
		successRatio := float64(record.SuccessCount) / float64(totalAttempts)
		confidence += successRatio * 50 // Up to 50 points from success ratio
	}

	// Boost for recent successes
	if time.Since(record.LastSuccess) < 24*time.Hour {
		confidence += 20
	} else if time.Since(record.LastSuccess) < 7*24*time.Hour {
		confidence += 10
	}

	// Penalty for recent failures
	if record.LastFailure.After(record.LastSuccess) {
		confidence -= 30
	}

	// Context similarity scoring
	contextScore := l.scoreContextSimilarity(record, context)
	confidence += contextScore * 30 // Up to 30 points from context match

	// Volume bonus - more usage means more confidence
	if record.SuccessCount > 100 {
		confidence += 20
	} else if record.SuccessCount > 50 {
		confidence += 15
	} else if record.SuccessCount > 10 {
		confidence += 10
	}

	// Cap confidence at 100
	if confidence > 100 {
		confidence = 100
	} else if confidence < 0 {
		confidence = 0
	}

	return confidence
}

// scoreContextSimilarity scores how similar the current context is to historical patterns
func (l *ResolutionLearner) scoreContextSimilarity(record *ResolutionRecord, context map[string]interface{}) float64 {
	if len(record.ParameterPatterns) == 0 {
		return 0
	}

	// Count matching parameters
	matches := 0
	totalWeight := 0

	for param, frequency := range record.ParameterPatterns {
		totalWeight += frequency
		if _, exists := context[param]; exists {
			matches += frequency
		}
	}

	if totalWeight == 0 {
		return 0
	}

	return float64(matches) / float64(totalWeight)
}

// recordContextPattern records a context pattern for learning
func (l *ResolutionLearner) recordContextPattern(record *ResolutionRecord, context map[string]interface{}) {
	// Extract parameter names (excluding internal ones)
	var params []string
	for param := range context {
		if !strings.HasPrefix(param, "__") {
			params = append(params, param)
		}
	}

	// Sort for consistent comparison
	sort.Strings(params)

	// Check if this pattern exists
	patternKey := strings.Join(params, ",")
	found := false

	for i, pattern := range record.ContextPatterns {
		existingKey := strings.Join(pattern.Parameters, ",")
		if existingKey == patternKey {
			record.ContextPatterns[i].Frequency++
			record.ContextPatterns[i].LastSeen = time.Now()
			found = true
			break
		}
	}

	if !found {
		record.ContextPatterns = append(record.ContextPatterns, ContextPattern{
			Parameters: params,
			Frequency:  1,
			LastSeen:   time.Now(),
		})
	}

	// Keep only top 10 patterns
	if len(record.ContextPatterns) > 10 {
		sort.Slice(record.ContextPatterns, func(i, j int) bool {
			return record.ContextPatterns[i].Frequency > record.ContextPatterns[j].Frequency
		})
		record.ContextPatterns = record.ContextPatterns[:10]
	}
}

// extractErrorType extracts a generic error type from an error message
func (l *ResolutionLearner) extractErrorType(errorMsg string) string {
	// Common error patterns (tool-agnostic)
	patterns := map[string][]string{
		"missing_parameter": {"missing", "required parameter", "parameter missing"},
		"invalid_parameter": {"invalid", "incorrect", "malformed"},
		"not_found":         {"not found", "404", "does not exist"},
		"unauthorized":      {"unauthorized", "401", "authentication", "permission denied"},
		"rate_limit":        {"rate limit", "429", "too many requests"},
		"timeout":           {"timeout", "timed out", "deadline exceeded"},
		"network":           {"connection", "network", "unreachable"},
		"parsing":           {"parse", "unmarshal", "decode", "invalid json"},
	}

	errorLower := strings.ToLower(errorMsg)

	for errorType, indicators := range patterns {
		for _, indicator := range indicators {
			if strings.Contains(errorLower, indicator) {
				return errorType
			}
		}
	}

	return "unknown"
}

// loadHistory loads resolution history from database
func (l *ResolutionLearner) loadHistory(ctx context.Context, toolID string) (*OperationHistory, error) {
	// Check cache first
	l.cacheMu.RLock()
	if cached, exists := l.learningCache[toolID]; exists {
		// Cache hit if less than 5 minutes old
		if time.Since(cached.LastUpdated) < 5*time.Minute {
			l.cacheMu.RUnlock()
			return cached, nil
		}
	}
	l.cacheMu.RUnlock()

	// Load from database
	var tool models.DynamicTool
	query := `
		SELECT id, metadata 
		FROM tool_configurations 
		WHERE id = $1
	`

	err := l.db.GetContext(ctx, &tool, query, toolID)
	if err != nil {
		return nil, fmt.Errorf("failed to load tool: %w", err)
	}

	// Extract learning history from metadata
	if tool.Metadata == nil {
		return nil, nil
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(*tool.Metadata, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Look for resolution_history in metadata
	if historyData, exists := metadata["resolution_history"]; exists {
		historyJSON, err := json.Marshal(historyData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal history: %w", err)
		}

		var history OperationHistory
		if err := json.Unmarshal(historyJSON, &history); err != nil {
			return nil, fmt.Errorf("failed to unmarshal history: %w", err)
		}

		// Update cache
		l.cacheMu.Lock()
		l.learningCache[toolID] = &history
		l.cacheMu.Unlock()

		return &history, nil
	}

	return nil, nil
}

// saveHistory saves resolution history to database
func (l *ResolutionLearner) saveHistory(ctx context.Context, toolID string, history *OperationHistory) error {
	// Load current metadata
	var metadata map[string]interface{}
	query := `
		SELECT metadata 
		FROM tool_configurations 
		WHERE id = $1
	`

	var metadataJSON *json.RawMessage
	err := l.db.GetContext(ctx, &metadataJSON, query, toolID)
	if err != nil {
		return fmt.Errorf("failed to load tool metadata: %w", err)
	}

	if metadataJSON != nil {
		if err := json.Unmarshal(*metadataJSON, &metadata); err != nil {
			// If unmarshal fails, start fresh
			metadata = make(map[string]interface{})
		}
	} else {
		metadata = make(map[string]interface{})
	}

	// Update resolution history
	history.LastUpdated = time.Now()
	metadata["resolution_history"] = history

	// Save back to database
	updatedJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	updateQuery := `
		UPDATE tool_configurations 
		SET metadata = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`

	_, err = l.db.ExecContext(ctx, updateQuery, updatedJSON, toolID)
	if err != nil {
		return fmt.Errorf("failed to update tool metadata: %w", err)
	}

	l.logger.Debug("Saved resolution history", map[string]interface{}{
		"tool_id":     toolID,
		"resolutions": len(history.Resolutions),
	})

	return nil
}

// PruneOldHistory removes old learning data to prevent unbounded growth
func (l *ResolutionLearner) PruneOldHistory(ctx context.Context, olderThan time.Duration) error {
	query := `
		SELECT id, metadata 
		FROM tool_configurations 
		WHERE metadata IS NOT NULL
	`

	rows, err := l.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query tools: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			l.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	pruned := 0
	cutoff := time.Now().Add(-olderThan)

	for rows.Next() {
		var toolID string
		var metadataJSON *json.RawMessage

		if err := rows.Scan(&toolID, &metadataJSON); err != nil {
			continue
		}

		if metadataJSON == nil {
			continue
		}

		var metadata map[string]interface{}
		if err := json.Unmarshal(*metadataJSON, &metadata); err != nil {
			continue
		}

		// Check for resolution history
		if historyData, exists := metadata["resolution_history"]; exists {
			historyJSON, _ := json.Marshal(historyData)
			var history OperationHistory
			if err := json.Unmarshal(historyJSON, &history); err != nil {
				continue
			}

			// Prune old resolutions
			updated := false
			for key, record := range history.Resolutions {
				// Remove if no success in cutoff period and low success rate
				if record.LastSuccess.Before(cutoff) {
					totalAttempts := record.SuccessCount + record.FailureCount
					successRate := float64(record.SuccessCount) / float64(totalAttempts)
					if successRate < 0.5 || totalAttempts < 5 {
						delete(history.Resolutions, key)
						updated = true
						pruned++
					}
				}
			}

			if updated {
				// Save updated history
				metadata["resolution_history"] = history
				updatedJSON, _ := json.Marshal(metadata)

				updateQuery := `
					UPDATE tool_configurations 
					SET metadata = $1
					WHERE id = $2
				`
				if _, err := l.db.ExecContext(ctx, updateQuery, updatedJSON, toolID); err != nil {
					l.logger.Warn("Failed to update resolution history", map[string]interface{}{
						"error":  err.Error(),
						"toolID": toolID,
					})
				}
			}
		}
	}

	l.logger.Info("Pruned old resolution history", map[string]interface{}{
		"pruned_count": pruned,
		"cutoff":       cutoff,
	})

	return nil
}
