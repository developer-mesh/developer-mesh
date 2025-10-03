package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultBatchConfig tests the default configuration
func TestDefaultBatchConfig(t *testing.T) {
	config := DefaultBatchConfig()

	assert.Equal(t, 10, config.MaxBatchSize, "Default max batch size should be 10")
	assert.True(t, config.EnableParallelExecution, "Parallel execution should be enabled by default")
	assert.True(t, config.ContinueOnError, "Continue on error should be enabled by default")
	assert.Equal(t, 5*time.Minute, config.Timeout, "Default timeout should be 5 minutes")
	assert.Equal(t, 5, config.MaxConcurrency, "Default max concurrency should be 5")
}

// TestBatchExecutor_ValidateBatchRequest tests batch request validation
func TestBatchExecutor_ValidateBatchRequest(t *testing.T) {
	registry := tools.NewRegistry()
	logger := observability.NewNoopLogger()
	executor := NewBatchExecutor(registry, nil, logger)

	tests := []struct {
		name      string
		request   *BatchRequest
		expectErr bool
		errMsg    string
	}{
		{
			name:      "nil request",
			request:   nil,
			expectErr: true,
			errMsg:    "batch request is nil",
		},
		{
			name: "empty batch",
			request: &BatchRequest{
				Tools: []BatchToolCall{},
			},
			expectErr: true,
			errMsg:    "batch is empty",
		},
		{
			name: "batch too large",
			request: &BatchRequest{
				Tools: make([]BatchToolCall, 11), // Default max is 10
			},
			expectErr: true,
			errMsg:    "batch size 11 exceeds maximum 10",
		},
		{
			name: "tool with empty name",
			request: &BatchRequest{
				Tools: []BatchToolCall{
					{Name: "tool1"},
					{Name: ""}, // Empty name
				},
			},
			expectErr: true,
			errMsg:    "tool at index 1 has empty name",
		},
		{
			name: "valid batch",
			request: &BatchRequest{
				Tools: []BatchToolCall{
					{Name: "tool1"},
					{Name: "tool2"},
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.ValidateBatchRequest(tt.request)
			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestBatchExecutor_Execute_Sequential tests sequential execution
func TestBatchExecutor_Execute_Sequential(t *testing.T) {
	registry := tools.NewRegistry()
	logger := observability.NewNoopLogger()

	// Register mock tools
	successTool := tools.ToolDefinition{
		Name:        "success_tool",
		Description: "Always succeeds",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			return map[string]interface{}{"status": "ok"}, nil
		},
	}

	failTool := tools.ToolDefinition{
		Name:        "fail_tool",
		Description: "Always fails",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			return nil, errors.New("intentional failure")
		},
	}

	slowTool := tools.ToolDefinition{
		Name:        "slow_tool",
		Description: "Sleeps for a bit",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			time.Sleep(100 * time.Millisecond)
			return map[string]interface{}{"status": "ok"}, nil
		},
	}

	registry.RegisterRemote(successTool)
	registry.RegisterRemote(failTool)
	registry.RegisterRemote(slowTool)

	executor := NewBatchExecutor(registry, nil, logger)

	parallel := false
	request := &BatchRequest{
		Tools: []BatchToolCall{
			{ID: "1", Name: "success_tool"},
			{ID: "2", Name: "fail_tool"},
			{ID: "3", Name: "success_tool"},
		},
		Parallel: &parallel,
	}

	response, err := executor.Execute(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, response)

	assert.Equal(t, 3, len(response.Results), "Should have 3 results")
	assert.Equal(t, 2, response.SuccessCount, "Should have 2 successes")
	assert.Equal(t, 1, response.ErrorCount, "Should have 1 error")
	assert.False(t, response.Parallel, "Should be sequential")

	// Check individual results
	assert.Equal(t, "success", response.Results[0].Status)
	assert.Equal(t, "1", response.Results[0].ID)
	assert.Equal(t, "success_tool", response.Results[0].Name)

	assert.Equal(t, "error", response.Results[1].Status)
	assert.Equal(t, "2", response.Results[1].ID)
	assert.Contains(t, response.Results[1].Error, "intentional failure")

	assert.Equal(t, "success", response.Results[2].Status)
	assert.Equal(t, "3", response.Results[2].ID)
}

// TestBatchExecutor_Execute_Parallel tests parallel execution
func TestBatchExecutor_Execute_Parallel(t *testing.T) {
	registry := tools.NewRegistry()
	logger := observability.NewNoopLogger()

	// Register slow tools to test parallelism
	sleepDuration := 200 * time.Millisecond
	slowTool := tools.ToolDefinition{
		Name:        "slow_tool",
		Description: "Sleeps for 200ms",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			time.Sleep(sleepDuration)
			return map[string]interface{}{"status": "ok"}, nil
		},
	}

	registry.RegisterRemote(slowTool)

	executor := NewBatchExecutor(registry, nil, logger)

	parallel := true
	request := &BatchRequest{
		Tools: []BatchToolCall{
			{ID: "1", Name: "slow_tool"},
			{ID: "2", Name: "slow_tool"},
			{ID: "3", Name: "slow_tool"},
		},
		Parallel: &parallel,
	}

	start := time.Now()
	response, err := executor.Execute(context.Background(), request)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, response)

	assert.Equal(t, 3, response.SuccessCount, "All tools should succeed")
	assert.True(t, response.Parallel, "Should be parallel")

	// If executed in parallel, total time should be less than 3x the sleep duration
	// With some margin for overhead
	maxExpectedDuration := sleepDuration + (100 * time.Millisecond) // 200ms + 100ms overhead
	assert.Less(t, elapsed, maxExpectedDuration, "Parallel execution should be faster than sequential")
}

// TestBatchExecutor_Execute_PartialFailure tests handling of partial failures
func TestBatchExecutor_Execute_PartialFailure(t *testing.T) {
	registry := tools.NewRegistry()
	logger := observability.NewNoopLogger()

	successTool := tools.ToolDefinition{
		Name:        "success_tool",
		Description: "Always succeeds",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			return map[string]interface{}{"status": "ok"}, nil
		},
	}

	failTool := tools.ToolDefinition{
		Name:        "fail_tool",
		Description: "Always fails",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			return nil, errors.New("intentional failure")
		},
	}

	registry.RegisterRemote(successTool)
	registry.RegisterRemote(failTool)

	executor := NewBatchExecutor(registry, nil, logger)

	request := &BatchRequest{
		Tools: []BatchToolCall{
			{ID: "1", Name: "success_tool"},
			{ID: "2", Name: "fail_tool"},
			{ID: "3", Name: "success_tool"},
			{ID: "4", Name: "fail_tool"},
		},
	}

	response, err := executor.Execute(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, response)

	assert.Equal(t, 4, len(response.Results), "Should have 4 results")
	assert.Equal(t, 2, response.SuccessCount, "Should have 2 successes")
	assert.Equal(t, 2, response.ErrorCount, "Should have 2 errors")

	// Check that successes and failures are in the right places
	assert.Equal(t, "success", response.Results[0].Status)
	assert.Equal(t, "error", response.Results[1].Status)
	assert.Equal(t, "success", response.Results[2].Status)
	assert.Equal(t, "error", response.Results[3].Status)
}

// TestBatchExecutor_Execute_MaxBatchSize tests batch size limits
func TestBatchExecutor_Execute_MaxBatchSize(t *testing.T) {
	registry := tools.NewRegistry()
	logger := observability.NewNoopLogger()

	config := &BatchConfig{
		MaxBatchSize:            3,
		EnableParallelExecution: true,
		ContinueOnError:         true,
		Timeout:                 5 * time.Minute,
		MaxConcurrency:          5,
	}

	executor := NewBatchExecutor(registry, config, logger)

	request := &BatchRequest{
		Tools: []BatchToolCall{
			{Name: "tool1"},
			{Name: "tool2"},
			{Name: "tool3"},
			{Name: "tool4"}, // Exceeds max
		},
	}

	_, err := executor.Execute(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "batch size 4 exceeds maximum 3")
}

// TestBatchExecutor_Execute_Timeout tests batch timeout handling
func TestBatchExecutor_Execute_Timeout(t *testing.T) {
	registry := tools.NewRegistry()
	logger := observability.NewNoopLogger()

	// Create tool that sleeps longer than timeout
	longRunningTool := tools.ToolDefinition{
		Name:        "long_tool",
		Description: "Takes a long time",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			select {
			case <-time.After(2 * time.Second):
				return map[string]interface{}{"status": "ok"}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}

	registry.RegisterRemote(longRunningTool)

	config := &BatchConfig{
		MaxBatchSize:            10,
		EnableParallelExecution: true,
		ContinueOnError:         true,
		Timeout:                 100 * time.Millisecond, // Very short timeout
		MaxConcurrency:          5,
	}

	executor := NewBatchExecutor(registry, config, logger)

	request := &BatchRequest{
		Tools: []BatchToolCall{
			{ID: "1", Name: "long_tool"},
		},
	}

	response, err := executor.Execute(context.Background(), request)
	require.NoError(t, err) // Batch doesn't fail, individual tools do
	require.NotNil(t, response)

	// The tool should have failed due to timeout/context cancellation
	assert.Equal(t, 1, response.ErrorCount)
	// Error could be "context deadline exceeded" or contain "timeout"
	errorMsg := response.Results[0].Error
	assert.True(t,
		errorMsg == "context deadline exceeded" || errorMsg == "context canceled",
		"Expected timeout/cancellation error, got: %s", errorMsg)
}

// TestBatchExecutor_Execute_ContinueOnError tests continue on error behavior
func TestBatchExecutor_Execute_ContinueOnError(t *testing.T) {
	registry := tools.NewRegistry()
	logger := observability.NewNoopLogger()

	successTool := tools.ToolDefinition{
		Name:        "success_tool",
		Description: "Always succeeds",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			return map[string]interface{}{"status": "ok"}, nil
		},
	}

	failTool := tools.ToolDefinition{
		Name:        "fail_tool",
		Description: "Always fails",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			return nil, errors.New("intentional failure")
		},
	}

	registry.RegisterRemote(successTool)
	registry.RegisterRemote(failTool)

	// Test with ContinueOnError = false
	config := &BatchConfig{
		MaxBatchSize:            10,
		EnableParallelExecution: false, // Sequential to test stopping behavior
		ContinueOnError:         false, // Stop on first error
		Timeout:                 5 * time.Minute,
		MaxConcurrency:          5,
	}

	executor := NewBatchExecutor(registry, config, logger)

	parallel := false
	request := &BatchRequest{
		Tools: []BatchToolCall{
			{ID: "1", Name: "success_tool"},
			{ID: "2", Name: "fail_tool"},
			{ID: "3", Name: "success_tool"}, // Should not execute
		},
		Parallel: &parallel,
	}

	response, err := executor.Execute(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, response)

	// Should have stopped after the failure at index 1
	assert.Equal(t, 2, len(response.Results), "Should have stopped after first error")
	assert.Equal(t, "success", response.Results[0].Status)
	assert.Equal(t, "error", response.Results[1].Status)
}

// TestBatchExecutor_Execute_ToolNotFound tests handling of non-existent tools
func TestBatchExecutor_Execute_ToolNotFound(t *testing.T) {
	registry := tools.NewRegistry()
	logger := observability.NewNoopLogger()

	executor := NewBatchExecutor(registry, nil, logger)

	request := &BatchRequest{
		Tools: []BatchToolCall{
			{ID: "1", Name: "nonexistent_tool"},
		},
	}

	response, err := executor.Execute(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, response)

	assert.Equal(t, 1, response.ErrorCount)
	assert.Contains(t, response.Results[0].Error, "tool not found")
}

// TestBatchExecutor_Execute_Concurrency tests concurrency control
func TestBatchExecutor_Execute_Concurrency(t *testing.T) {
	registry := tools.NewRegistry()
	logger := observability.NewNoopLogger()

	// Track concurrent executions
	var currentConcurrent int32
	var maxConcurrent int32
	var mu sync.Mutex

	concurrentTool := tools.ToolDefinition{
		Name:        "concurrent_tool",
		Description: "Tracks concurrency",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			mu.Lock()
			currentConcurrent++
			if currentConcurrent > maxConcurrent {
				maxConcurrent = currentConcurrent
			}
			mu.Unlock()

			time.Sleep(50 * time.Millisecond)

			mu.Lock()
			currentConcurrent--
			mu.Unlock()

			return map[string]interface{}{"status": "ok"}, nil
		},
	}

	registry.RegisterRemote(concurrentTool)

	config := &BatchConfig{
		MaxBatchSize:            10,
		EnableParallelExecution: true,
		ContinueOnError:         true,
		Timeout:                 5 * time.Minute,
		MaxConcurrency:          3, // Limit to 3 concurrent
	}

	executor := NewBatchExecutor(registry, config, logger)

	request := &BatchRequest{
		Tools: []BatchToolCall{
			{ID: "1", Name: "concurrent_tool"},
			{ID: "2", Name: "concurrent_tool"},
			{ID: "3", Name: "concurrent_tool"},
			{ID: "4", Name: "concurrent_tool"},
			{ID: "5", Name: "concurrent_tool"},
		},
	}

	response, err := executor.Execute(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, response)

	assert.Equal(t, 5, response.SuccessCount)
	assert.LessOrEqual(t, maxConcurrent, int32(3), "Should not exceed max concurrency of 3")
}

// TestBatchExecutor_Execute_WithArguments tests passing arguments to tools
func TestBatchExecutor_Execute_WithArguments(t *testing.T) {
	registry := tools.NewRegistry()
	logger := observability.NewNoopLogger()

	echoTool := tools.ToolDefinition{
		Name:        "echo_tool",
		Description: "Returns the input",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var input map[string]interface{}
			if err := json.Unmarshal(args, &input); err != nil {
				return nil, err
			}
			return input, nil
		},
	}

	registry.RegisterRemote(echoTool)

	executor := NewBatchExecutor(registry, nil, logger)

	args1, _ := json.Marshal(map[string]interface{}{"message": "hello"})
	args2, _ := json.Marshal(map[string]interface{}{"message": "world"})

	request := &BatchRequest{
		Tools: []BatchToolCall{
			{ID: "1", Name: "echo_tool", Arguments: args1},
			{ID: "2", Name: "echo_tool", Arguments: args2},
		},
	}

	response, err := executor.Execute(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, response)

	assert.Equal(t, 2, response.SuccessCount)

	// Check that results contain the correct arguments
	result1 := response.Results[0].Result.(map[string]interface{})
	assert.Equal(t, "hello", result1["message"])

	result2 := response.Results[1].Result.(map[string]interface{})
	assert.Equal(t, "world", result2["message"])
}

// TestBatchExecutor_Execute_DurationTracking tests duration tracking for individual tools
func TestBatchExecutor_Execute_DurationTracking(t *testing.T) {
	registry := tools.NewRegistry()
	logger := observability.NewNoopLogger()

	sleepDuration := 100 * time.Millisecond
	timedTool := tools.ToolDefinition{
		Name:        "timed_tool",
		Description: "Sleeps for a specific duration",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			time.Sleep(sleepDuration)
			return map[string]interface{}{"status": "ok"}, nil
		},
	}

	registry.RegisterRemote(timedTool)

	executor := NewBatchExecutor(registry, nil, logger)

	request := &BatchRequest{
		Tools: []BatchToolCall{
			{ID: "1", Name: "timed_tool"},
		},
	}

	response, err := executor.Execute(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, response)

	// Check that duration is tracked and approximately correct
	assert.GreaterOrEqual(t, response.Results[0].Duration, sleepDuration,
		"Duration should be at least the sleep duration")
	assert.Less(t, response.Results[0].Duration, sleepDuration+(50*time.Millisecond),
		"Duration should not be much longer than sleep duration")
}
