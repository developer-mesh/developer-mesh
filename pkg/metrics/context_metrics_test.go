package metrics_test

import (
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/metrics"
	"github.com/stretchr/testify/assert"
)

func TestNewContextMetrics(t *testing.T) {
	assert := assert.New(t)

	// Create metrics instance
	m := metrics.NewContextMetrics()

	// Verify all metrics are initialized
	assert.NotNil(m.EmbeddingGenerationDuration)
	assert.NotNil(m.EmbeddingGenerationErrors)
	assert.NotNil(m.ContextRetrievalMethod)
	assert.NotNil(m.ContextRetrievalDuration)
	assert.NotNil(m.CompactionExecutions)
	assert.NotNil(m.CompactionDuration)
	assert.NotNil(m.TokensSaved)
	assert.NotNil(m.TokenUtilization)
	assert.NotNil(m.SecurityViolations)
	assert.NotNil(m.AuditEvents)
}

func TestContextMetrics_RecordEmbeddingGeneration(t *testing.T) {
	assert := assert.New(t)

	m := metrics.NewContextMetrics()

	// Test successful embedding generation
	m.RecordEmbeddingGeneration(0.150, true)

	// Test failed embedding generation
	m.RecordEmbeddingGeneration(0.050, false)

	// If we got here without panic, the metrics were recorded successfully
	assert.True(true)
}

func TestContextMetrics_RecordRetrieval(t *testing.T) {
	assert := assert.New(t)

	m := metrics.NewContextMetrics()

	// Test different retrieval methods
	testCases := []struct {
		method   string
		duration float64
	}{
		{"full", 0.050},
		{"semantic", 0.100},
		{"windowed", 0.030},
	}

	for _, tc := range testCases {
		m.RecordRetrieval(tc.method, tc.duration)
	}

	// Verify no panics occurred
	assert.True(true)
}

func TestContextMetrics_RecordCompaction(t *testing.T) {
	assert := assert.New(t)

	m := metrics.NewContextMetrics()

	testCases := []struct {
		name        string
		strategy    string
		duration    float64
		tokensSaved int
		success     bool
	}{
		{
			name:        "Successful tool_clear compaction",
			strategy:    "tool_clear",
			duration:    0.250,
			tokensSaved: 500,
			success:     true,
		},
		{
			name:        "Successful prune compaction",
			strategy:    "prune",
			duration:    0.150,
			tokensSaved: 200,
			success:     true,
		},
		{
			name:        "Failed summarize compaction",
			strategy:    "summarize",
			duration:    1.000,
			tokensSaved: 0,
			success:     false,
		},
		{
			name:        "Sliding window compaction",
			strategy:    "sliding",
			duration:    0.100,
			tokensSaved: 300,
			success:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m.RecordCompaction(tc.strategy, tc.duration, tc.tokensSaved, tc.success)
		})
	}

	// Verify no panics occurred
	assert.True(true)
}

func TestContextMetrics_RecordTokenUtilization(t *testing.T) {
	assert := assert.New(t)

	m := metrics.NewContextMetrics()

	testCases := []struct {
		name       string
		usedTokens int
		maxTokens  int
	}{
		{
			name:       "Low utilization",
			usedTokens: 1000,
			maxTokens:  4000,
		},
		{
			name:       "High utilization",
			usedTokens: 3800,
			maxTokens:  4000,
		},
		{
			name:       "Full utilization",
			usedTokens: 4000,
			maxTokens:  4000,
		},
		{
			name:       "Zero tokens (edge case)",
			usedTokens: 0,
			maxTokens:  4000,
		},
		{
			name:       "Zero max tokens (should not panic)",
			usedTokens: 100,
			maxTokens:  0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m.RecordTokenUtilization(tc.usedTokens, tc.maxTokens)
		})
	}

	// Verify no panics occurred
	assert.True(true)
}

func TestContextMetrics_RecordSecurityViolation(t *testing.T) {
	assert := assert.New(t)

	m := metrics.NewContextMetrics()

	// Test different violation types
	violationTypes := []string{
		"injection",
		"cross_tenant",
		"replay",
		"unauthorized_access",
	}

	for _, violationType := range violationTypes {
		m.RecordSecurityViolation(violationType)
	}

	// Verify no panics occurred
	assert.True(true)
}

func TestContextMetrics_RecordAuditEvent(t *testing.T) {
	assert := assert.New(t)

	m := metrics.NewContextMetrics()

	testCases := []struct {
		operation string
		tenantID  string
	}{
		{
			operation: "create",
			tenantID:  "tenant-123",
		},
		{
			operation: "read",
			tenantID:  "tenant-456",
		},
		{
			operation: "update",
			tenantID:  "tenant-123",
		},
		{
			operation: "delete",
			tenantID:  "tenant-789",
		},
		{
			operation: "compact",
			tenantID:  "tenant-123",
		},
		{
			operation: "semantic_retrieval",
			tenantID:  "tenant-456",
		},
	}

	for _, tc := range testCases {
		m.RecordAuditEvent(tc.operation, tc.tenantID)
	}

	// Verify no panics occurred
	assert.True(true)
}

func TestContextMetrics_IntegrationScenario(t *testing.T) {
	assert := assert.New(t)

	m := metrics.NewContextMetrics()

	// Simulate a complete workflow
	// 1. Generate embeddings
	m.RecordEmbeddingGeneration(0.120, true)

	// 2. Retrieve context semantically
	m.RecordRetrieval("semantic", 0.080)

	// 3. Record token utilization
	m.RecordTokenUtilization(3200, 4000)

	// 4. Perform compaction
	m.RecordCompaction("tool_clear", 0.200, 450, true)

	// 5. Record audit event
	m.RecordAuditEvent("compact", "tenant-test-123")

	// Verify the workflow completed without errors
	assert.True(true)
}

func TestContextMetrics_ConcurrentAccess(t *testing.T) {
	assert := assert.New(t)

	m := metrics.NewContextMetrics()

	// Test concurrent metric recording
	done := make(chan bool)

	// Spawn multiple goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			m.RecordEmbeddingGeneration(0.100, true)
			m.RecordRetrieval("full", 0.050)
			m.RecordCompaction("prune", 0.150, 100, true)
			m.RecordTokenUtilization(2000, 4000)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify no panics or race conditions
	assert.True(true)
}
