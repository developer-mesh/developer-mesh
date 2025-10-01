package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock tool provider for testing
type mockToolProvider struct {
	tools []ToolDefinition
}

func (m *mockToolProvider) GetDefinitions() []ToolDefinition {
	return m.tools
}

func TestRegistry_RegisterAndList(t *testing.T) {
	// Setup
	registry := NewRegistry()

	// Create mock tools
	mockProvider := &mockToolProvider{
		tools: []ToolDefinition{
			{
				Name:        "test_tool_1",
				Description: "First test tool",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"input": map[string]interface{}{
							"type": "string",
						},
					},
				},
				Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
					return map[string]string{"result": "success"}, nil
				},
			},
			{
				Name:        "test_tool_2",
				Description: "Second test tool",
				InputSchema: map[string]interface{}{
					"type": "object",
				},
				Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
					return "tool 2 result", nil
				},
			},
		},
	}

	// Test registration
	registry.Register(mockProvider)

	// Test listing
	allTools := registry.ListAll()
	assert.Len(t, allTools, 2, "Should have 2 tools registered")

	// Verify tool details
	toolNames := make(map[string]bool)
	for _, tool := range allTools {
		toolNames[tool.Name] = true
	}
	assert.True(t, toolNames["test_tool_1"], "Should have test_tool_1")
	assert.True(t, toolNames["test_tool_2"], "Should have test_tool_2")

	// Test count
	assert.Equal(t, 2, registry.Count())
	assert.Equal(t, 2, registry.Size())
}

func TestRegistry_ExecuteTool_Success(t *testing.T) {
	// Setup
	registry := NewRegistry()

	// Create a tool that echoes input
	echoTool := ToolDefinition{
		Name:        "echo_tool",
		Description: "Echoes the input",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var input map[string]interface{}
			if err := json.Unmarshal(args, &input); err != nil {
				return nil, err
			}
			return map[string]interface{}{
				"echoed": input["message"],
			}, nil
		},
	}

	// Register directly
	registry.RegisterRemote(echoTool)

	// Test execution
	testArgs := json.RawMessage(`{"message": "Hello World"}`)
	result, err := registry.Execute(context.Background(), "echo_tool", testArgs)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, result)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok, "Result should be a map")
	assert.Equal(t, "Hello World", resultMap["echoed"])
}

func TestRegistry_ExecuteTool_NotFound(t *testing.T) {
	// Setup
	registry := NewRegistry()

	// Try to execute non-existent tool
	_, err := registry.Execute(context.Background(), "non_existent_tool", nil)

	// Verify error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tool not found: non_existent_tool")
}

func TestRegistry_ExecuteTool_HandlerError(t *testing.T) {
	// Setup
	registry := NewRegistry()

	// Create a tool that always fails
	failingTool := ToolDefinition{
		Name:        "failing_tool",
		Description: "Always fails",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			return nil, errors.New("tool execution failed: database connection lost")
		},
	}

	registry.RegisterRemote(failingTool)

	// Test execution
	_, err := registry.Execute(context.Background(), "failing_tool", nil)

	// Verify error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tool execution failed: database connection lost")
}

func TestRegistry_ExecuteTool_ContextCancellation(t *testing.T) {
	// Setup
	registry := NewRegistry()

	// Create a slow tool
	slowTool := ToolDefinition{
		Name:        "slow_tool",
		Description: "Takes time to execute",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			select {
			case <-time.After(5 * time.Second):
				return "completed", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}

	registry.RegisterRemote(slowTool)

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Start execution in goroutine
	done := make(chan struct{})
	var execErr error

	go func() {
		_, execErr = registry.Execute(ctx, "slow_tool", nil)
		close(done)
	}()

	// Cancel after short time
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Wait for completion
	<-done

	// Verify cancellation
	assert.Error(t, execErr)
	assert.Equal(t, context.Canceled, execErr)
}

func TestRegistry_ExecuteTool_ParameterValidation(t *testing.T) {
	// Setup
	registry := NewRegistry()

	// Create a tool that validates parameters
	validatingTool := ToolDefinition{
		Name:        "validating_tool",
		Description: "Validates input parameters",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var input map[string]interface{}
			if err := json.Unmarshal(args, &input); err != nil {
				return nil, errors.New("invalid JSON input")
			}

			// Check required field
			if _, ok := input["required_field"]; !ok {
				return nil, errors.New("missing required field: required_field")
			}

			// Check field type
			if _, ok := input["required_field"].(string); !ok {
				return nil, errors.New("required_field must be a string")
			}

			return "validation passed", nil
		},
	}

	registry.RegisterRemote(validatingTool)

	// Test with valid parameters
	validArgs := json.RawMessage(`{"required_field": "value"}`)
	result, err := registry.Execute(context.Background(), "validating_tool", validArgs)
	assert.NoError(t, err)
	assert.Equal(t, "validation passed", result)

	// Test with missing field
	missingArgs := json.RawMessage(`{"other_field": "value"}`)
	_, err = registry.Execute(context.Background(), "validating_tool", missingArgs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required field")

	// Test with invalid JSON
	invalidArgs := json.RawMessage(`{invalid json}`)
	_, err = registry.Execute(context.Background(), "validating_tool", invalidArgs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	// Setup
	registry := NewRegistry()

	// Create a simple tool
	simpleTool := ToolDefinition{
		Name:        "concurrent_tool",
		Description: "For concurrent testing",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			return "success", nil
		},
	}

	registry.RegisterRemote(simpleTool)

	// Test concurrent execution
	numGoroutines := 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			// Execute tool
			result, err := registry.Execute(context.Background(), "concurrent_tool", nil)
			assert.NoError(t, err)
			assert.Equal(t, "success", result)

			// List tools
			tools := registry.ListAll()
			assert.NotEmpty(t, tools)

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestRegistry_ExecuteTool_NoHandler(t *testing.T) {
	// Setup
	registry := NewRegistry()

	// Create a tool without handler
	noHandlerTool := ToolDefinition{
		Name:        "no_handler_tool",
		Description: "Tool without handler",
		Handler:     nil,
	}

	registry.RegisterRemote(noHandlerTool)

	// Test execution
	_, err := registry.Execute(context.Background(), "no_handler_tool", nil)

	// Verify error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no handler configured")
}

func TestRegistry_MultipleProviders(t *testing.T) {
	// Setup
	registry := NewRegistry()

	// First provider
	provider1 := &mockToolProvider{
		tools: []ToolDefinition{
			{
				Name:        "provider1_tool1",
				Description: "Tool from provider 1",
				Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
					return "provider1_result", nil
				},
			},
		},
	}

	// Second provider
	provider2 := &mockToolProvider{
		tools: []ToolDefinition{
			{
				Name:        "provider2_tool1",
				Description: "Tool from provider 2",
				Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
					return "provider2_result", nil
				},
			},
			{
				Name:        "provider2_tool2",
				Description: "Another tool from provider 2",
				Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
					return "provider2_result2", nil
				},
			},
		},
	}

	// Register both providers
	registry.Register(provider1)
	registry.Register(provider2)

	// Verify all tools are registered
	assert.Equal(t, 3, registry.Count())

	// Test execution of tools from different providers
	result1, err := registry.Execute(context.Background(), "provider1_tool1", nil)
	assert.NoError(t, err)
	assert.Equal(t, "provider1_result", result1)

	result2, err := registry.Execute(context.Background(), "provider2_tool1", nil)
	assert.NoError(t, err)
	assert.Equal(t, "provider2_result", result2)
}

func TestRegistry_OverwriteTool(t *testing.T) {
	// Setup
	registry := NewRegistry()

	// Register initial tool
	tool1 := ToolDefinition{
		Name:        "overwrite_tool",
		Description: "Original tool",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			return "original", nil
		},
	}
	registry.RegisterRemote(tool1)

	// Verify original tool
	result, err := registry.Execute(context.Background(), "overwrite_tool", nil)
	assert.NoError(t, err)
	assert.Equal(t, "original", result)

	// Overwrite with new version
	tool2 := ToolDefinition{
		Name:        "overwrite_tool",
		Description: "Updated tool",
		Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			return "updated", nil
		},
	}
	registry.RegisterRemote(tool2)

	// Verify updated tool
	result, err = registry.Execute(context.Background(), "overwrite_tool", nil)
	assert.NoError(t, err)
	assert.Equal(t, "updated", result)

	// Should still have only 1 tool
	assert.Equal(t, 1, registry.Count())
}
