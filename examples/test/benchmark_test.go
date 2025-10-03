package test

import (
	"context"
	"testing"

	"github.com/developer-mesh/developer-mesh/examples/common"
)

// BenchmarkSingleToolCall benchmarks a single tool call
func BenchmarkSingleToolCall(b *testing.B) {
	client, err := common.NewClient(nil)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.CallTool(ctx, "agent_list", map[string]interface{}{})
	}
}

// BenchmarkBatchParallel benchmarks parallel batch execution
func BenchmarkBatchParallel(b *testing.B) {
	client, err := common.NewClient(nil)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	tools := []common.BatchToolCall{
		{ID: "1", Name: "agent_list", Arguments: map[string]interface{}{}},
		{ID: "2", Name: "task_list", Arguments: map[string]interface{}{}},
		{ID: "3", Name: "github_list_repositories", Arguments: map[string]interface{}{"type": "owner"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.BatchCallTools(ctx, tools, true)
	}
}

// BenchmarkBatchSequential benchmarks sequential batch execution
func BenchmarkBatchSequential(b *testing.B) {
	client, err := common.NewClient(nil)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	tools := []common.BatchToolCall{
		{ID: "1", Name: "agent_list", Arguments: map[string]interface{}{}},
		{ID: "2", Name: "task_list", Arguments: map[string]interface{}{}},
		{ID: "3", Name: "github_list_repositories", Arguments: map[string]interface{}{"type": "owner"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.BatchCallTools(ctx, tools, false)
	}
}

// BenchmarkListTools benchmarks tool listing
func BenchmarkListTools(b *testing.B) {
	client, err := common.NewClient(nil)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.ListTools(ctx)
	}
}

// BenchmarkContextUpdate benchmarks context updates
func BenchmarkContextUpdate(b *testing.B) {
	client, err := common.NewClient(nil)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	contextData := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
		"key3": true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.UpdateContext(ctx, contextData, false)
	}
}

// BenchmarkContextGet benchmarks context retrieval
func BenchmarkContextGet(b *testing.B) {
	client, err := common.NewClient(nil)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Set initial context
	_ = client.UpdateContext(ctx, map[string]interface{}{"key": "value"}, false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.GetContext(ctx)
	}
}

// BenchmarkFuzzySearch benchmarks fuzzy tool search
func BenchmarkFuzzySearch(b *testing.B) {
	client, err := common.NewClient(nil)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	tools, err := client.ListTools(ctx)
	if err != nil {
		b.Fatalf("Failed to list tools: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = common.FuzzySearchTools(tools, "github")
	}
}
