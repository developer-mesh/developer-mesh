//go:build integration

package jira

import (
	"context"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
)

func TestJiraProviderBasicIntegration(t *testing.T) {
	logger := &observability.NoopLogger{}

	t.Run("ProviderInitialization", func(t *testing.T) {
		provider := NewJiraProvider(logger, "test-tenant")
		assert.NotNil(t, provider)
		assert.Equal(t, "jira", provider.GetProviderName())
	})

	t.Run("ToolDefinitions", func(t *testing.T) {
		provider := NewJiraProvider(logger, "test-tenant")

		// Test tool definitions are loaded
		toolDefs := provider.GetToolDefinitions()
		assert.NotEmpty(t, toolDefs)

		// Check for key toolsets
		toolNames := make([]string, 0, len(toolDefs))
		for _, tool := range toolDefs {
			toolNames = append(toolNames, tool.Name)
		}

		assert.Contains(t, toolNames, "jira_issues")
		assert.Contains(t, toolNames, "jira_projects")
	})

	t.Run("OperationMappings", func(t *testing.T) {
		provider := NewJiraProvider(logger, "test-tenant")

		// Test operation mappings
		mappings := provider.GetOperationMappings()
		assert.NotEmpty(t, mappings)

		// Check for essential operations
		assert.Contains(t, mappings, "issues/create")
		assert.Contains(t, mappings, "issues/get")
		assert.Contains(t, mappings, "issues/update")
		assert.Contains(t, mappings, "issues/delete")
		assert.Contains(t, mappings, "issues/search")
	})

	t.Run("HealthStatus", func(t *testing.T) {
		provider := NewJiraProvider(logger, "test-tenant")

		// Test health status
		status := provider.GetHealthStatus()
		assert.NotNil(t, status)
		// Initially false until first health check
		assert.False(t, status.Healthy)
	})

	t.Run("CacheManager", func(t *testing.T) {
		provider := NewJiraProvider(logger, "test-tenant")

		// Check if cache manager is initialized
		if provider.cacheManager != nil {
			t.Log("Cache manager is initialized")

			// Test cache operations
			// Check if cache is working by verifying IsCacheable
			cacheable := provider.cacheManager.IsCacheable("GET", "issues/get")
			assert.True(t, cacheable)

			cacheable = provider.cacheManager.IsCacheable("POST", "issues/create")
			assert.False(t, cacheable)
		} else {
			t.Log("Cache manager not initialized")
		}
	})

	t.Run("SecurityManager", func(t *testing.T) {
		provider := NewJiraProvider(logger, "test-tenant")

		// Check if security manager is initialized
		if provider.securityMgr != nil {
			t.Log("Security manager is initialized")

			// Test PII detection
			testData := []byte(`{"email": "user@example.com"}`)
			piiTypes, err := provider.securityMgr.DetectPII(testData)
			assert.NoError(t, err)
			assert.NotEmpty(t, piiTypes)
		} else {
			t.Log("Security manager not initialized")
		}
	})

	t.Run("ObservabilityManager", func(t *testing.T) {
		provider := NewJiraProvider(logger, "test-tenant")

		// Check if observability manager is initialized
		if provider.observabilityMgr != nil {
			t.Log("Observability manager is initialized")

			// Test debug mode
			debugMode := provider.IsDebugMode()
			assert.False(t, debugMode) // Should be false by default

			// Test metrics
			metrics := provider.observabilityMgr.GetObservabilityMetrics()
			assert.NotEmpty(t, metrics)
		} else {
			t.Log("Observability manager not initialized")
		}
	})

	t.Run("ToolsetManagement", func(t *testing.T) {
		provider := NewJiraProvider(logger, "test-tenant")

		// Test enabling/disabling toolsets
		err := provider.EnableToolset("issues")
		assert.NoError(t, err)
		assert.True(t, provider.IsToolsetEnabled("issues"))

		err = provider.DisableToolset("issues")
		assert.NoError(t, err)
		assert.False(t, provider.IsToolsetEnabled("issues"))

		// Get enabled toolsets
		enabledToolsets := provider.GetEnabledToolsets()
		assert.NotNil(t, enabledToolsets)
	})

	t.Run("ContextConfiguration", func(t *testing.T) {
		provider := NewJiraProvider(logger, "test-tenant")
		provider.domain = "https://test.atlassian.net"

		ctx := context.Background()

		// Test context configuration (this should not panic)
		provider.ConfigureFromContext(ctx)

		// Test read-only mode check
		isReadOnly := provider.IsReadOnlyMode(ctx)
		assert.False(t, isReadOnly) // Should be false by default

		// Test write operation check
		isWriteOp := provider.IsWriteOperation("issues/create")
		assert.True(t, isWriteOp)

		isWriteOp = provider.IsWriteOperation("issues/get")
		assert.False(t, isWriteOp)
	})

	t.Run("AI OptimizedDefinitions", func(t *testing.T) {
		t.Skip("AI definitions not fully implemented yet")
		// TODO: Enable this test once AI definitions are fully implemented
	})
}
