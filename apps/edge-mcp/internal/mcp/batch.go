package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// BatchConfig configures batch execution behavior
type BatchConfig struct {
	MaxBatchSize            int           // Maximum number of tools in one batch (default: 10)
	EnableParallelExecution bool          // Execute tools in parallel (default: true)
	ContinueOnError         bool          // Continue executing remaining tools after a failure (default: true)
	Timeout                 time.Duration // Maximum time for batch execution (default: 5 minutes)
	MaxConcurrency          int           // Maximum concurrent tool executions (default: 5)
}

// DefaultBatchConfig returns the default batch configuration
func DefaultBatchConfig() *BatchConfig {
	return &BatchConfig{
		MaxBatchSize:            10,
		EnableParallelExecution: true,
		ContinueOnError:         true,
		Timeout:                 5 * time.Minute,
		MaxConcurrency:          5,
	}
}

// BatchToolCall represents a single tool call in a batch
type BatchToolCall struct {
	ID        string          `json:"id,omitempty"`        // Optional client-provided ID for tracking
	Name      string          `json:"name"`                // Tool name
	Arguments json.RawMessage `json:"arguments,omitempty"` // Tool arguments
}

// BatchRequest represents a batch of tool calls
type BatchRequest struct {
	Tools    []BatchToolCall `json:"tools"`              // Tools to execute
	Parallel *bool           `json:"parallel,omitempty"` // Override parallel execution (nil = use config default)
}

// BatchToolResult represents the result of a single tool execution
type BatchToolResult struct {
	ID       string        `json:"id,omitempty"`     // Matches the tool call ID if provided
	Name     string        `json:"name"`             // Tool name
	Status   string        `json:"status"`           // "success" or "error"
	Result   interface{}   `json:"result,omitempty"` // Tool result (on success)
	Error    string        `json:"error,omitempty"`  // Error message (on failure)
	Duration time.Duration `json:"duration_ms"`      // Execution duration in milliseconds
	Index    int           `json:"index"`            // Position in batch
}

// BatchResponse represents the response for a batch execution
type BatchResponse struct {
	Results       []BatchToolResult `json:"results"`        // Results for each tool
	TotalDuration time.Duration     `json:"total_duration"` // Total batch execution time
	SuccessCount  int               `json:"success_count"`  // Number of successful executions
	ErrorCount    int               `json:"error_count"`    // Number of failed executions
	Parallel      bool              `json:"parallel"`       // Whether execution was parallel
}

// BatchExecutor executes batches of tool calls
type BatchExecutor struct {
	registry *tools.Registry
	config   *BatchConfig
	logger   observability.Logger
}

// NewBatchExecutor creates a new batch executor
func NewBatchExecutor(registry *tools.Registry, config *BatchConfig, logger observability.Logger) *BatchExecutor {
	if config == nil {
		config = DefaultBatchConfig()
	}
	return &BatchExecutor{
		registry: registry,
		config:   config,
		logger:   logger,
	}
}

// Execute executes a batch of tool calls
func (b *BatchExecutor) Execute(ctx context.Context, request *BatchRequest) (*BatchResponse, error) {
	// Validate batch size
	if len(request.Tools) == 0 {
		return nil, fmt.Errorf("batch is empty: at least one tool call is required")
	}

	if len(request.Tools) > b.config.MaxBatchSize {
		return nil, fmt.Errorf("batch size %d exceeds maximum %d", len(request.Tools), b.config.MaxBatchSize)
	}

	// Determine execution mode (parallel or sequential)
	parallel := b.config.EnableParallelExecution
	if request.Parallel != nil {
		parallel = *request.Parallel
	}

	// Create batch context with timeout
	batchCtx, cancel := context.WithTimeout(ctx, b.config.Timeout)
	defer cancel()

	// Track batch execution time
	batchStart := time.Now()

	// Execute tools
	var results []BatchToolResult
	if parallel {
		results = b.executeParallel(batchCtx, request.Tools)
	} else {
		results = b.executeSequential(batchCtx, request.Tools)
	}

	// Calculate summary
	successCount := 0
	errorCount := 0
	for _, result := range results {
		if result.Status == "success" {
			successCount++
		} else {
			errorCount++
		}
	}

	response := &BatchResponse{
		Results:       results,
		TotalDuration: time.Since(batchStart),
		SuccessCount:  successCount,
		ErrorCount:    errorCount,
		Parallel:      parallel,
	}

	b.logger.Info("Batch execution completed", map[string]interface{}{
		"total_tools":   len(request.Tools),
		"success_count": successCount,
		"error_count":   errorCount,
		"duration_ms":   response.TotalDuration.Milliseconds(),
		"parallel":      parallel,
	})

	return response, nil
}

// executeParallel executes tools in parallel with concurrency control
func (b *BatchExecutor) executeParallel(ctx context.Context, tools []BatchToolCall) []BatchToolResult {
	results := make([]BatchToolResult, len(tools))
	var wg sync.WaitGroup

	// Create semaphore for concurrency control
	semaphore := make(chan struct{}, b.config.MaxConcurrency)

	// Create mutex for safe result updates
	var resultMu sync.Mutex

	for i, tool := range tools {
		wg.Add(1)
		go func(index int, toolCall BatchToolCall) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				// Context cancelled, record error
				resultMu.Lock()
				results[index] = BatchToolResult{
					ID:     toolCall.ID,
					Name:   toolCall.Name,
					Status: "error",
					Error:  "batch execution timeout or cancelled",
					Index:  index,
				}
				resultMu.Unlock()
				return
			}

			// Execute tool
			result := b.executeSingleTool(ctx, toolCall, index)

			// Store result safely
			resultMu.Lock()
			results[index] = result
			resultMu.Unlock()
		}(i, tool)
	}

	wg.Wait()
	return results
}

// executeSequential executes tools in sequence
func (b *BatchExecutor) executeSequential(ctx context.Context, tools []BatchToolCall) []BatchToolResult {
	results := make([]BatchToolResult, 0, len(tools))

	for i, tool := range tools {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			// Context cancelled, stop execution
			results = append(results, BatchToolResult{
				ID:     tool.ID,
				Name:   tool.Name,
				Status: "error",
				Error:  "batch execution timeout or cancelled",
				Index:  i,
			})

			// If continue on error is disabled, stop here
			if !b.config.ContinueOnError {
				break
			}
			continue
		default:
		}

		// Execute tool
		result := b.executeSingleTool(ctx, tool, i)
		results = append(results, result)

		// If continue on error is disabled and tool failed, stop
		if !b.config.ContinueOnError && result.Status == "error" {
			b.logger.Warn("Stopping batch execution due to error", map[string]interface{}{
				"tool":  tool.Name,
				"error": result.Error,
				"index": i,
			})
			break
		}
	}

	return results
}

// executeSingleTool executes a single tool and returns the result
func (b *BatchExecutor) executeSingleTool(ctx context.Context, toolCall BatchToolCall, index int) BatchToolResult {
	start := time.Now()

	result := BatchToolResult{
		ID:    toolCall.ID,
		Name:  toolCall.Name,
		Index: index,
	}

	// Execute tool
	toolResult, err := b.registry.Execute(ctx, toolCall.Name, toolCall.Arguments)
	duration := time.Since(start)
	result.Duration = duration

	if err != nil {
		result.Status = "error"
		result.Error = err.Error()

		b.logger.Debug("Tool execution failed in batch", map[string]interface{}{
			"tool":        toolCall.Name,
			"index":       index,
			"error":       err.Error(),
			"duration_ms": duration.Milliseconds(),
		})
	} else {
		result.Status = "success"
		result.Result = toolResult

		b.logger.Debug("Tool execution succeeded in batch", map[string]interface{}{
			"tool":        toolCall.Name,
			"index":       index,
			"duration_ms": duration.Milliseconds(),
		})
	}

	return result
}

// ValidateBatchRequest validates a batch request
func (b *BatchExecutor) ValidateBatchRequest(request *BatchRequest) error {
	if request == nil {
		return fmt.Errorf("batch request is nil")
	}

	if len(request.Tools) == 0 {
		return fmt.Errorf("batch is empty: at least one tool call is required")
	}

	if len(request.Tools) > b.config.MaxBatchSize {
		return fmt.Errorf("batch size %d exceeds maximum %d", len(request.Tools), b.config.MaxBatchSize)
	}

	// Validate each tool call
	for i, tool := range request.Tools {
		if tool.Name == "" {
			return fmt.Errorf("tool at index %d has empty name", i)
		}
	}

	return nil
}
