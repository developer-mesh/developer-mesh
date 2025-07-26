package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthCheckManager(t *testing.T) {
	logger := &mockLogger{}
	cache := &mockCache{}
	handler := &mockOpenAPIHandler{}
	metrics := &mockMetricsClient{}
	manager := NewHealthCheckManager(cache, handler, logger, metrics)

	t.Run("CheckHealth_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "healthy", "version": "1.0.0"}`))
		}))
		defer server.Close()

		config := ToolConfig{
			Name:    "test-tool",
			BaseURL: server.URL,
			HealthConfig: &HealthCheckConfig{
				Mode:     "on_demand",
				HealthEndpoint: "/health",
				CheckTimeout:   5 * time.Second,
			},
		}

		status, err := manager.CheckHealth(context.Background(), config, true)
		require.NoError(t, err)
		assert.True(t, status.IsHealthy)
		// Message field was removed, check IsHealthy instead
		assert.Contains(t, status.Details, "version")
		assert.Equal(t, "1.0.0", status.Details["version"])
		assert.NotZero(t, status.ResponseTime)
	})

	t.Run("CheckHealth_Failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "database connection failed"}`))
		}))
		defer server.Close()

		config := ToolConfig{
			Name:    "test-tool",
			BaseURL: server.URL,
			HealthConfig: &HealthCheckConfig{
				Mode:     "on_demand",
				HealthEndpoint: "/health",
				CheckTimeout:   5 * time.Second,
			},
		}

		status, err := manager.CheckHealth(context.Background(), config, true)
		require.NoError(t, err) // CheckHealth doesn't return error for unhealthy status
		assert.False(t, status.IsHealthy)
		assert.Contains(t, status.Error, "500")
	})

	t.Run("CheckHealth_Timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond) // Simulate slow response
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := ToolConfig{
			Name:    "test-tool",
			BaseURL: server.URL,
			HealthConfig: &HealthCheckConfig{
				Mode:     "on_demand",
				HealthEndpoint: "/health",
				CheckTimeout:   50 * time.Millisecond, // Very short timeout
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
		defer cancel()

		status, err := manager.CheckHealth(ctx, config, true)
		require.NoError(t, err)
		assert.False(t, status.IsHealthy)
		assert.Contains(t, status.Error, "timeout")
	})

	t.Run("CheckHealth_WithAuth", func(t *testing.T) {
		expectedToken := "test-token"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "Bearer "+expectedToken {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status": "healthy"}`))
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		}))
		defer server.Close()

		config := ToolConfig{
			Name:    "test-tool",
			BaseURL: server.URL,
			HealthConfig: &HealthCheckConfig{
				Mode:     "on_demand",
				HealthEndpoint: "/health",
				CheckTimeout:   5 * time.Second,
			},
			Credential: &models.TokenCredential{
				Type:  "token",
				Token: expectedToken,
			},
		}

		status, err := manager.CheckHealth(context.Background(), config, true)
		require.NoError(t, err)
		assert.True(t, status.IsHealthy)
	})

	t.Run("CheckHealth_Caching", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "healthy"}`))
		}))
		defer server.Close()

		config := ToolConfig{
			ID:      "test-tool-123",
			Name:    "test-tool",
			BaseURL: server.URL,
			HealthConfig: &HealthCheckConfig{
				Mode:     "periodic",
				HealthEndpoint: "/health",
				CheckTimeout:   5 * time.Second,
			},
		}

		// First call - should hit the server
		status1, err := manager.CheckHealth(context.Background(), config, false)
		require.NoError(t, err)
		assert.True(t, status1.IsHealthy)
		assert.Equal(t, 1, callCount)

		// Second call - should use cache
		status2, err := manager.CheckHealth(context.Background(), config, false)
		require.NoError(t, err)
		assert.True(t, status2.IsHealthy)
		assert.Equal(t, 1, callCount) // No additional call

		// Third call with force - should hit the server
		status3, err := manager.CheckHealth(context.Background(), config, true)
		require.NoError(t, err)
		assert.True(t, status3.IsHealthy)
		assert.Equal(t, 2, callCount) // Additional call
	})

	t.Run("GetCachedStatus", func(t *testing.T) {
		config := ToolConfig{
			ID:   "test-tool-cached",
			Name: "test-tool",
		}

		// No cached status initially
		_, exists := manager.GetCachedStatus(context.Background(), config)
		assert.False(t, exists)

		// Add to cache
		status := &HealthStatus{
			IsHealthy:   true,
			Error:       "", // healthy status has no error
			LastChecked: time.Now(),
		}
		// Store in cache using the Set method
		ctx := context.Background()
		manager.cache.Set(ctx, config.ID, status, 5*time.Minute)

		// Should find cached status
		cacheKey := fmt.Sprintf("health:%s:%s", config.TenantID, config.ID)
		var cachedStatus *HealthStatus
		err := manager.cache.Get(ctx, cacheKey, &cachedStatus)
		assert.NoError(t, err)
		assert.NotNil(t, cachedStatus)
		assert.Equal(t, status.IsHealthy, cachedStatus.IsHealthy)
		assert.Equal(t, status.Error, cachedStatus.Error)
	})

	t.Run("InvalidateCache", func(t *testing.T) {
		config := ToolConfig{
			ID:   "test-tool-invalidate",
			Name: "test-tool",
		}

		// Add to cache
		status := &HealthStatus{
			IsHealthy:   true,
			Error:       "", // healthy status has no error
			LastChecked: time.Now(),
		}
		// Store in cache using the Set method
		ctx := context.Background()
		manager.cache.Set(ctx, config.ID, status, 5*time.Minute)

		// Verify it's cached
		_, exists := manager.GetCachedStatus(context.Background(), config)
		assert.True(t, exists)

		// Invalidate
		manager.InvalidateCache(context.Background(), config)

		// Should no longer be cached
		_, exists = manager.GetCachedStatus(context.Background(), config)
		assert.False(t, exists)
	})

	t.Run("DefaultHealthEndpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		config := ToolConfig{
			Name:    "test-tool",
			BaseURL: server.URL,
			// No HealthConfig - should use defaults
		}

		status, err := manager.CheckHealth(context.Background(), config, true)
		require.NoError(t, err)
		assert.True(t, status.IsHealthy)
	})
}

func TestHealthCheckScheduler(t *testing.T) {
	logger := &mockLogger{}
	cache := &mockCache{}
	handler := &mockOpenAPIHandler{}
	metrics := &mockMetricsClient{}
	manager := NewHealthCheckManager(cache, handler, logger, metrics)
	db := &mockHealthCheckDB{}
	scheduler := NewHealthCheckScheduler(manager, db, logger, 100*time.Millisecond)

	t.Run("StartStop", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := scheduler.Start(ctx)
		require.NoError(t, err)

		// Verify it's running
		scheduler.mu.RLock()
		running := scheduler.running
		scheduler.mu.RUnlock()
		assert.True(t, running)

		// Stop
		scheduler.Stop()

		// Verify it's stopped
		scheduler.mu.RLock()
		running = scheduler.running
		scheduler.mu.RUnlock()
		assert.False(t, running)
	})

	t.Run("AddRemoveTools", func(t *testing.T) {
		tool1 := ToolConfig{
			ID:   "tool-1",
			Name: "Tool 1",
		}
		tool2 := ToolConfig{
			ID:   "tool-2",
			Name: "Tool 2",
		}

		// Add tools
		scheduler.AddTool(tool1)
		scheduler.AddTool(tool2)

		// Verify tools are tracked
		tools := scheduler.GetScheduledTools()
		assert.Len(t, tools, 2)

		// Remove a tool
		scheduler.RemoveTool(tool1.ID)

		// Verify tool is removed
		tools = scheduler.GetScheduledTools()
		assert.Len(t, tools, 1)
		assert.Equal(t, tool2.ID, tools[0].ID)
	})

	t.Run("PerformHealthChecks", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "healthy"}`))
		}))
		defer server.Close()

		// Add tools to scheduler
		tool := ToolConfig{
			ID:       "test-tool",
			TenantID: "tenant-123",
			Name:     "Test Tool",
			BaseURL:  server.URL,
			HealthConfig: &HealthCheckConfig{
				Mode:     "periodic",
				HealthEndpoint: "/health",
			},
		}
		scheduler.AddTool(tool)

		// Perform health checks
		ctx := context.Background()
		scheduler.performHealthChecks(ctx)

		// Verify database was updated
		assert.Len(t, db.updates, 1)
		assert.Equal(t, tool.TenantID, db.updates[0].tenantID)
		assert.Equal(t, tool.ID, db.updates[0].toolID)
		assert.True(t, db.updates[0].status.IsHealthy)
	})
}

// Mock implementations for testing

type mockLogger struct {
	logs []map[string]interface{}
}

func (m *mockLogger) Info(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, fields)
}

func (m *mockLogger) Error(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, fields)
}

func (m *mockLogger) Debug(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, fields)
}

func (m *mockLogger) Warn(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, fields)
}

func (m *mockLogger) Fatal(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, fields)
}

func (m *mockLogger) Infof(format string, args ...interface{}) {
	// Mock implementation
}

func (m *mockLogger) Errorf(format string, args ...interface{}) {
	// Mock implementation
}

func (m *mockLogger) Debugf(format string, args ...interface{}) {
	// Mock implementation
}

func (m *mockLogger) Warnf(format string, args ...interface{}) {
	// Mock implementation
}

func (m *mockLogger) Fatalf(format string, args ...interface{}) {
	// Mock implementation
}

func (m *mockLogger) WithPrefix(prefix string) observability.Logger {
	return m
}

func (m *mockLogger) With(fields map[string]interface{}) observability.Logger {
	return m
}

type mockHealthCheckDB struct {
	tools   []ToolConfig
	updates []healthUpdate
}

type healthUpdate struct {
	tenantID string
	toolID   string
	status   *HealthStatus
}

func (m *mockHealthCheckDB) GetActiveToolsForHealthCheck(ctx context.Context) ([]ToolConfig, error) {
	return m.tools, nil
}

func (m *mockHealthCheckDB) UpdateToolHealthStatus(ctx context.Context, tenantID, toolID string, status *HealthStatus) error {
	m.updates = append(m.updates, healthUpdate{
		tenantID: tenantID,
		toolID:   toolID,
		status:   status,
	})
	return nil
}

// mockCache implements cache.Cache for testing
type mockCache struct {
	data map[string]interface{}
	mu   sync.RWMutex
}

func (m *mockCache) Get(ctx context.Context, key string, value interface{}) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.data == nil {
		return fmt.Errorf("key not found")
	}
	val, ok := m.data[key]
	if !ok {
		return fmt.Errorf("key not found")
	}
	// Use reflection to set the value
	if val != nil && value != nil {
		// This is a simplified version - in real implementation would use reflection
		if hp, ok := value.(**HealthStatus); ok {
			if hs, ok := val.(*HealthStatus); ok {
				*hp = hs
			}
		}
	}
	return nil
}

func (m *mockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = make(map[string]interface{})
	}
	m.data[key] = value
	return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data != nil {
		delete(m.data, key)
	}
	return nil
}

func (m *mockCache) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]interface{})
}

func (m *mockCache) Close() error {
	return nil
}

func (m *mockCache) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.data == nil {
		return false, nil
	}
	_, ok := m.data[key]
	return ok, nil
}

func (m *mockCache) Flush(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]interface{})
	return nil
}

// mockOpenAPIHandler implements OpenAPIHandler for testing
type mockOpenAPIHandler struct{}

func (m *mockOpenAPIHandler) TestConnection(ctx context.Context, config ToolConfig) error {
	return nil
}

func (m *mockOpenAPIHandler) AuthenticateRequest(req *http.Request, creds *models.TokenCredential, schemes map[string]SecurityScheme) error {
	return nil
}

func (m *mockOpenAPIHandler) DiscoverAPIs(ctx context.Context, config ToolConfig) (*DiscoveryResult, error) {
	return &DiscoveryResult{
		Status: DiscoveryStatusSuccess,
	}, nil
}

func (m *mockOpenAPIHandler) ExtractSecuritySchemes(spec *openapi3.T) map[string]SecurityScheme {
	return make(map[string]SecurityScheme)
}

func (m *mockOpenAPIHandler) GenerateTools(config ToolConfig, spec *openapi3.T) ([]*Tool, error) {
	return []*Tool{}, nil
}

// mockMetricsClient implements observability.MetricsClient for testing
type mockMetricsClient struct{}

func (m *mockMetricsClient) RecordEvent(source, eventType string) {}
func (m *mockMetricsClient) RecordLatency(operation string, duration time.Duration) {}
func (m *mockMetricsClient) RecordCounter(name string, value float64, labels map[string]string) {}
func (m *mockMetricsClient) RecordGauge(name string, value float64, labels map[string]string) {}
func (m *mockMetricsClient) RecordHistogram(name string, value float64, labels map[string]string) {}
func (m *mockMetricsClient) RecordTimer(name string, duration time.Duration, labels map[string]string) {}
func (m *mockMetricsClient) RecordCacheOperation(operation string, success bool, durationSeconds float64) {}
func (m *mockMetricsClient) RecordOperation(component string, operation string, success bool, durationSeconds float64, labels map[string]string) {}
func (m *mockMetricsClient) RecordAPIOperation(api string, operation string, success bool, durationSeconds float64) {}
func (m *mockMetricsClient) RecordDatabaseOperation(operation string, success bool, durationSeconds float64) {}
func (m *mockMetricsClient) StartTimer(name string, labels map[string]string) func() { return func() {} }
func (m *mockMetricsClient) IncrementCounter(name string, value float64) {}
func (m *mockMetricsClient) IncrementCounterWithLabels(name string, value float64, labels map[string]string) {}
func (m *mockMetricsClient) RecordDuration(name string, duration time.Duration) {}
func (m *mockMetricsClient) Close() error { return nil }
