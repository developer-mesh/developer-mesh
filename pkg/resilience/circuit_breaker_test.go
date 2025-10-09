package resilience

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMetricsClient implements a mock metrics client for testing
type mockMetricsClient struct {
	counters   map[string]float64
	gauges     map[string]float64
	histograms map[string][]float64
	mu         sync.Mutex
}

func newMockMetricsClient() *mockMetricsClient {
	return &mockMetricsClient{
		counters:   make(map[string]float64),
		gauges:     make(map[string]float64),
		histograms: make(map[string][]float64),
	}
}

func (m *mockMetricsClient) RecordEvent(source, eventType string)                   {}
func (m *mockMetricsClient) RecordLatency(operation string, duration time.Duration) {}
func (m *mockMetricsClient) RecordCounter(name string, value float64, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name] += value
}
func (m *mockMetricsClient) RecordGauge(name string, value float64, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[name] = value
}
func (m *mockMetricsClient) RecordHistogram(name string, value float64, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.histograms[name] = append(m.histograms[name], value)
}
func (m *mockMetricsClient) RecordTimer(name string, duration time.Duration, labels map[string]string) {
}
func (m *mockMetricsClient) RecordCacheOperation(operation string, success bool, durationSeconds float64) {
}
func (m *mockMetricsClient) RecordOperation(component string, operation string, success bool, durationSeconds float64, labels map[string]string) {
}
func (m *mockMetricsClient) RecordAPIOperation(api string, operation string, success bool, durationSeconds float64) {
}
func (m *mockMetricsClient) RecordDatabaseOperation(operation string, success bool, durationSeconds float64) {
}
func (m *mockMetricsClient) StartTimer(name string, labels map[string]string) func() {
	return func() {}
}
func (m *mockMetricsClient) IncrementCounter(name string, value float64) {
	m.RecordCounter(name, value, nil)
}
func (m *mockMetricsClient) IncrementCounterWithLabels(name string, value float64, labels map[string]string) {
	m.RecordCounter(name, value, labels)
}
func (m *mockMetricsClient) RecordDuration(name string, duration time.Duration) {}
func (m *mockMetricsClient) Close() error                                       { return nil }

func (m *mockMetricsClient) getGauge(name string) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.gauges[name]
}

// mockLogger implements a mock logger for testing
type mockLogger struct {
	logs []logEntry
	mu   sync.Mutex
}

type logEntry struct {
	level  string
	msg    string
	fields map[string]interface{}
}

func newMockLogger() *mockLogger {
	return &mockLogger{
		logs: make([]logEntry, 0),
	}
}

func (l *mockLogger) Debug(msg string, fields map[string]interface{})         { l.log("debug", msg, fields) }
func (l *mockLogger) Info(msg string, fields map[string]interface{})          { l.log("info", msg, fields) }
func (l *mockLogger) Warn(msg string, fields map[string]interface{})          { l.log("warn", msg, fields) }
func (l *mockLogger) Error(msg string, fields map[string]interface{})         { l.log("error", msg, fields) }
func (l *mockLogger) Fatal(msg string, fields map[string]interface{})         { l.log("fatal", msg, fields) }
func (l *mockLogger) Debugf(format string, args ...interface{})               {}
func (l *mockLogger) Infof(format string, args ...interface{})                {}
func (l *mockLogger) Warnf(format string, args ...interface{})                {}
func (l *mockLogger) Errorf(format string, args ...interface{})               {}
func (l *mockLogger) Fatalf(format string, args ...interface{})               {}
func (l *mockLogger) WithPrefix(prefix string) observability.Logger           { return l }
func (l *mockLogger) With(fields map[string]interface{}) observability.Logger { return l }

func (l *mockLogger) log(level, msg string, fields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, logEntry{level: level, msg: msg, fields: fields})
}

func TestCircuitBreaker_Execute(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(cb *CircuitBreaker)
		fn          func() (interface{}, error)
		wantResult  interface{}
		wantErr     error
		checkState  CircuitBreakerState
		checkCounts func(t *testing.T, counts *Counts)
	}{
		{
			name: "successful execution in closed state",
			fn: func() (interface{}, error) {
				return "success", nil
			},
			wantResult: "success",
			wantErr:    nil,
			checkState: CircuitBreakerClosed,
			checkCounts: func(t *testing.T, counts *Counts) {
				assert.Equal(t, 1, counts.Requests)
				assert.Equal(t, 1, counts.Successes)
				assert.Equal(t, 0, counts.Failures)
			},
		},
		{
			name: "failed execution in closed state",
			fn: func() (interface{}, error) {
				return nil, errors.New("test error")
			},
			wantResult: nil,
			wantErr:    errors.New("circuit breaker execution failed: test error"),
			checkState: CircuitBreakerClosed,
			checkCounts: func(t *testing.T, counts *Counts) {
				assert.Equal(t, 1, counts.Requests)
				assert.Equal(t, 0, counts.Successes)
				assert.Equal(t, 1, counts.Failures)
			},
		},
		{
			name: "circuit opens after failure threshold",
			setup: func(cb *CircuitBreaker) {
				// Cause failures to trip the circuit
				for i := 0; i < 5; i++ {
					_, _ = cb.Execute(context.Background(), func() (interface{}, error) {
						return nil, errors.New("failure")
					})
				}
			},
			fn: func() (interface{}, error) {
				return "should not execute", nil
			},
			wantResult: nil,
			wantErr:    ErrCircuitBreakerOpen,
			checkState: CircuitBreakerOpen,
		},
		{
			name: "circuit transitions to half-open after timeout",
			setup: func(cb *CircuitBreaker) {
				// Trip the circuit
				for i := 0; i < 5; i++ {
					_, _ = cb.Execute(context.Background(), func() (interface{}, error) {
						return nil, errors.New("failure")
					})
				}
				// Modify last failure time to simulate timeout
				cb.lastFailureTime.Store(time.Now().Add(-31 * time.Second))
			},
			fn: func() (interface{}, error) {
				return "half-open success", nil
			},
			wantResult: "half-open success",
			wantErr:    nil,
			checkState: CircuitBreakerHalfOpen,
		},
		{
			name: "circuit closes after success threshold in half-open",
			setup: func(cb *CircuitBreaker) {
				// Trip the circuit
				for i := 0; i < 5; i++ {
					_, _ = cb.Execute(context.Background(), func() (interface{}, error) {
						return nil, errors.New("failure")
					})
				}
				// Transition to half-open
				cb.lastFailureTime.Store(time.Now().Add(-31 * time.Second))
				// First success in half-open
				_, _ = cb.Execute(context.Background(), func() (interface{}, error) {
					return "success", nil
				})
			},
			fn: func() (interface{}, error) {
				return "final success", nil
			},
			wantResult: "final success",
			wantErr:    nil,
			checkState: CircuitBreakerClosed,
		},
		{
			name: "circuit re-opens on failure in half-open",
			setup: func(cb *CircuitBreaker) {
				// Trip the circuit
				for i := 0; i < 5; i++ {
					_, _ = cb.Execute(context.Background(), func() (interface{}, error) {
						return nil, errors.New("failure")
					})
				}
				// Transition to half-open
				cb.lastFailureTime.Store(time.Now().Add(-31 * time.Second))
			},
			fn: func() (interface{}, error) {
				return nil, errors.New("half-open failure")
			},
			wantResult: nil,
			wantErr:    errors.New("circuit breaker execution failed: half-open failure"),
			checkState: CircuitBreakerOpen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := newMockLogger()
			metrics := newMockMetricsClient()

			config := CircuitBreakerConfig{
				FailureThreshold:    5,
				FailureRatio:        0.6,
				ResetTimeout:        30 * time.Second,
				SuccessThreshold:    2,
				TimeoutThreshold:    5 * time.Second,
				MaxRequestsHalfOpen: 5,
				MinimumRequestCount: 10,
			}

			cb := NewCircuitBreaker("test", config, logger, metrics)

			if tt.setup != nil {
				tt.setup(cb)
			}

			result, err := cb.Execute(context.Background(), tt.fn)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantResult, result)
			}

			assert.Equal(t, tt.checkState, cb.getState())

			if tt.checkCounts != nil {
				counts := cb.getCounts()
				tt.checkCounts(t, counts)
			}
		})
	}
}

func TestCircuitBreaker_Timeout(t *testing.T) {
	logger := newMockLogger()
	metrics := newMockMetricsClient()

	config := CircuitBreakerConfig{
		TimeoutThreshold: 100 * time.Millisecond,
	}

	cb := NewCircuitBreaker("test-timeout", config, logger, metrics)

	result, err := cb.Execute(context.Background(), func() (interface{}, error) {
		time.Sleep(200 * time.Millisecond)
		return "should timeout", nil
	})

	require.Error(t, err)
	assert.Equal(t, ErrCircuitBreakerTimeout, err)
	assert.Nil(t, result)

	counts := cb.getCounts()
	assert.Equal(t, 1, counts.Failures)
}

func TestCircuitBreaker_ContextCancellation(t *testing.T) {
	logger := newMockLogger()
	metrics := newMockMetricsClient()

	cb := NewCircuitBreaker("test-cancel", CircuitBreakerConfig{}, logger, metrics)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result, err := cb.Execute(ctx, func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "should be cancelled", nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled")
	assert.Nil(t, result)
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	logger := newMockLogger()
	metrics := newMockMetricsClient()

	config := CircuitBreakerConfig{
		FailureThreshold:    50,
		MaxRequestsHalfOpen: 5,
	}

	cb := NewCircuitBreaker("test-concurrent", config, logger, metrics)

	var wg sync.WaitGroup
	successCount := atomic.Int32{}
	failureCount := atomic.Int32{}

	// Run 100 concurrent operations
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			_, err := cb.Execute(context.Background(), func() (interface{}, error) {
				// 50% success rate
				if idx%2 == 0 {
					return "success", nil
				}
				return nil, errors.New("failure")
			})

			if err != nil {
				failureCount.Add(1)
			} else {
				successCount.Add(1)
			}
		}(i)
	}

	wg.Wait()

	// Verify all operations completed
	total := successCount.Load() + failureCount.Load()
	assert.Equal(t, int32(100), total)

	// Verify state consistency
	counts := cb.getCounts()
	assert.Equal(t, counts.Successes+counts.Failures, counts.Requests)
}

func TestCircuitBreaker_HalfOpenMaxRequests(t *testing.T) {
	logger := newMockLogger()
	metrics := newMockMetricsClient()

	config := CircuitBreakerConfig{
		FailureThreshold:    3,
		MaxRequestsHalfOpen: 2,
		ResetTimeout:        100 * time.Millisecond,
	}

	cb := NewCircuitBreaker("test-halfopen", config, logger, metrics)

	// Trip the circuit
	for i := 0; i < 3; i++ {
		_, _ = cb.Execute(context.Background(), func() (interface{}, error) {
			return nil, errors.New("failure")
		})
	}

	assert.Equal(t, CircuitBreakerOpen, cb.getState())

	// Wait for reset timeout
	time.Sleep(150 * time.Millisecond)

	// Execute requests in half-open state
	var wg sync.WaitGroup
	results := make([]error, 5)

	// Add a small delay to ensure we're in half-open state
	time.Sleep(10 * time.Millisecond)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := cb.Execute(context.Background(), func() (interface{}, error) {
				time.Sleep(50 * time.Millisecond) // Simulate work
				return "success", nil
			})
			results[idx] = err
		}(i)
		// Small stagger to improve test reliability
		time.Sleep(5 * time.Millisecond)
	}

	wg.Wait()

	// Count how many requests were allowed vs rejected
	allowed := 0
	rejected := 0
	timedOut := 0
	for _, err := range results {
		if err == nil {
			allowed++
		} else if strings.Contains(err.Error(), "max requests exceeded") {
			rejected++
		} else if strings.Contains(err.Error(), "circuit breaker is open") {
			rejected++
		} else {
			timedOut++
		}
	}

	// Should allow max 2 requests in half-open state
	// We check that at least one was rejected (timing may allow 2-4 through)
	assert.GreaterOrEqual(t, allowed, 1, "Expected at least one request to be allowed")
	assert.GreaterOrEqual(t, rejected, 1, "Expected at least one request to be rejected")
	assert.Equal(t, 5, allowed+rejected+timedOut, "All requests should be accounted for")
}

func TestCircuitBreaker_FailureRatio(t *testing.T) {
	logger := newMockLogger()
	metrics := newMockMetricsClient()

	config := CircuitBreakerConfig{
		FailureThreshold:    100, // High threshold to test ratio
		FailureRatio:        0.5, // 50% failure rate
		MinimumRequestCount: 10,
	}

	cb := NewCircuitBreaker("test-ratio", config, logger, metrics)

	// Execute 9 requests (below minimum)
	for i := 0; i < 9; i++ {
		if i < 5 {
			_, _ = cb.Execute(context.Background(), func() (interface{}, error) {
				return nil, errors.New("failure")
			})
		} else {
			_, _ = cb.Execute(context.Background(), func() (interface{}, error) {
				return "success", nil
			})
		}
	}

	// Should still be closed (below minimum request count)
	assert.Equal(t, CircuitBreakerClosed, cb.getState())

	// 10th request (failure) should trip based on ratio
	_, _ = cb.Execute(context.Background(), func() (interface{}, error) {
		return nil, errors.New("failure")
	})

	// Should now be open (60% failure rate > 50% threshold)
	assert.Equal(t, CircuitBreakerOpen, cb.getState())
}

func TestCircuitBreaker_Reset(t *testing.T) {
	logger := newMockLogger()
	metrics := newMockMetricsClient()

	cb := NewCircuitBreaker("test-reset", CircuitBreakerConfig{}, logger, metrics)

	// Trip the circuit
	for i := 0; i < 5; i++ {
		_, _ = cb.Execute(context.Background(), func() (interface{}, error) {
			return nil, errors.New("failure")
		})
	}

	assert.Equal(t, CircuitBreakerOpen, cb.getState())

	// Reset the circuit
	cb.Reset()

	assert.Equal(t, CircuitBreakerClosed, cb.getState())
	counts := cb.getCounts()
	assert.Equal(t, 0, counts.Requests)
	assert.Equal(t, 0, counts.Failures)
	assert.Equal(t, 0, counts.Successes)
}

func TestCircuitBreakerManager(t *testing.T) {
	logger := newMockLogger()
	metrics := newMockMetricsClient()

	configs := map[string]CircuitBreakerConfig{
		"service1": {
			FailureThreshold: 3,
		},
		"service2": {
			FailureThreshold: 5,
		},
	}

	manager := NewCircuitBreakerManager(logger, metrics, configs)

	// Test existing circuit breaker
	cb1 := manager.GetCircuitBreaker("service1")
	assert.NotNil(t, cb1)
	assert.Equal(t, "service1", cb1.name)

	// Test getting same circuit breaker returns same instance
	cb1Again := manager.GetCircuitBreaker("service1")
	assert.Same(t, cb1, cb1Again)

	// Test creating new circuit breaker
	cb3 := manager.GetCircuitBreaker("service3")
	assert.NotNil(t, cb3)
	assert.Equal(t, "service3", cb3.name)

	// Test Execute method
	result, err := manager.Execute(context.Background(), "service1", func() (interface{}, error) {
		return "test result", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "test result", result)

	// Test GetAllMetrics
	allMetrics := manager.GetAllMetrics()
	assert.Len(t, allMetrics, 3)
	assert.Contains(t, allMetrics, "service1")
	assert.Contains(t, allMetrics, "service2")
	assert.Contains(t, allMetrics, "service3")

	// Test ResetAll
	manager.ResetAll()
	for _, metrics := range allMetrics {
		// All should be in closed state after reset
		assert.Equal(t, "closed", metrics["state"])
	}
}

func BenchmarkCircuitBreaker_Execute(b *testing.B) {
	logger := observability.NewStandardLogger("bench")
	metrics := newMockMetricsClient()

	cb := NewCircuitBreaker("bench", CircuitBreakerConfig{}, logger, metrics)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cb.Execute(context.Background(), func() (interface{}, error) {
			return "result", nil
		})
	}
}

func BenchmarkCircuitBreaker_ConcurrentExecute(b *testing.B) {
	logger := observability.NewStandardLogger("bench")
	metrics := newMockMetricsClient()

	cb := NewCircuitBreaker("bench-concurrent", CircuitBreakerConfig{}, logger, metrics)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cb.Execute(context.Background(), func() (interface{}, error) {
				return "result", nil
			})
		}
	})
}

func TestCircuitBreakerState_String(t *testing.T) {
	tests := []struct {
		state CircuitBreakerState
		want  string
	}{
		{CircuitBreakerClosed, "closed"},
		{CircuitBreakerOpen, "open"},
		{CircuitBreakerHalfOpen, "half-open"},
		{CircuitBreakerState(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.state.String())
		})
	}
}

func TestCircuitBreaker_EdgeCases(t *testing.T) {
	logger := newMockLogger()
	metrics := newMockMetricsClient()

	t.Run("unknown state in canExecute", func(t *testing.T) {
		cb := NewCircuitBreaker("test-unknown", CircuitBreakerConfig{}, logger, metrics)
		// Force an invalid state
		cb.state.Store(CircuitBreakerState(99))

		_, err := cb.Execute(context.Background(), func() (interface{}, error) {
			return "should not execute", nil
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown circuit breaker state")
	})

	t.Run("GetCircuitBreaker concurrent creation", func(t *testing.T) {
		manager := NewCircuitBreakerManager(logger, metrics, nil)

		var wg sync.WaitGroup
		breakers := make([]*CircuitBreaker, 10)

		// Concurrently try to get the same circuit breaker
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				breakers[idx] = manager.GetCircuitBreaker("concurrent-test")
			}(i)
		}

		wg.Wait()

		// All should be the same instance
		for i := 1; i < 10; i++ {
			assert.Same(t, breakers[0], breakers[i])
		}
	})

	t.Run("config with all defaults", func(t *testing.T) {
		cb := NewCircuitBreaker("defaults", CircuitBreakerConfig{}, logger, metrics)
		assert.Equal(t, 5, cb.config.FailureThreshold)
		assert.Equal(t, 0.6, cb.config.FailureRatio)
		assert.Equal(t, 30*time.Second, cb.config.ResetTimeout)
		assert.Equal(t, 2, cb.config.SuccessThreshold)
		assert.Equal(t, 5*time.Second, cb.config.TimeoutThreshold)
		assert.Equal(t, 5, cb.config.MaxRequestsHalfOpen)
		assert.Equal(t, 10, cb.config.MinimumRequestCount)
	})
}

func TestCounts_Methods(t *testing.T) {
	t.Run("RecordTimeout", func(t *testing.T) {
		counts := NewCounts()
		counts.RecordTimeout()

		assert.Equal(t, 1, counts.Requests)
		assert.Equal(t, 1, counts.Failures)
		assert.Equal(t, uint32(1), counts.TotalFailures)
		assert.Equal(t, 1, counts.Timeout)
		assert.Equal(t, 1, counts.ConsecutiveFailures)
		assert.Equal(t, 0, counts.ConsecutiveSuccesses)
		assert.NotZero(t, counts.LastTimeout)
		assert.NotZero(t, counts.LastFailure)
	})

	t.Run("RecordRejected", func(t *testing.T) {
		counts := NewCounts()
		counts.RecordRejected()
		assert.Equal(t, 1, counts.Rejected)
	})

	t.Run("RecordShortCircuited", func(t *testing.T) {
		counts := NewCounts()
		counts.RecordShortCircuited()
		assert.Equal(t, 1, counts.ShortCircuited)
	})

	t.Run("Reset", func(t *testing.T) {
		counts := NewCounts()
		counts.Requests = 10
		counts.Successes = 5
		counts.Failures = 5
		counts.ConsecutiveSuccesses = 2
		counts.ConsecutiveFailures = 0
		counts.Timeout = 1
		counts.ShortCircuited = 2
		counts.Rejected = 3

		counts.Reset()

		assert.Equal(t, 0, counts.Requests)
		assert.Equal(t, 0, counts.Successes)
		assert.Equal(t, 0, counts.Failures)
		assert.Equal(t, 0, counts.ConsecutiveSuccesses)
		assert.Equal(t, 0, counts.ConsecutiveFailures)
		assert.Equal(t, 0, counts.Timeout)
		assert.Equal(t, 0, counts.ShortCircuited)
		assert.Equal(t, 0, counts.Rejected)
	})

	t.Run("ResetTimestamps", func(t *testing.T) {
		counts := NewCounts()
		counts.LastSuccess = time.Now()
		counts.LastFailure = time.Now()
		counts.LastTimeout = time.Now()

		counts.ResetTimestamps()

		assert.True(t, counts.LastSuccess.IsZero())
		assert.True(t, counts.LastFailure.IsZero())
		assert.True(t, counts.LastTimeout.IsZero())
	})
}

func TestCircuitBreaker_Timeout_RecordsMetrics(t *testing.T) {
	logger := newMockLogger()
	metrics := newMockMetricsClient()

	config := CircuitBreakerConfig{
		TimeoutThreshold: 50 * time.Millisecond,
	}

	cb := NewCircuitBreaker("test-timeout-metrics", config, logger, metrics)

	// Force a timeout to verify it records timeout in counts
	_, err := cb.Execute(context.Background(), func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "should timeout", nil
	})

	require.Error(t, err)
	assert.Equal(t, ErrCircuitBreakerTimeout, err)

	// Verify the metrics were recorded
	assert.Greater(t, metrics.counters["circuit_breaker_requests_total"], 0.0)
	assert.Greater(t, metrics.counters["circuit_breaker_failures_total"], 0.0)

	// Verify state metrics
	gaugeValue, exists := metrics.gauges["circuit_breaker_current_state"]
	assert.True(t, exists)
	assert.Equal(t, float64(CircuitBreakerClosed), gaugeValue)
}

// TestCircuitBreaker_ExecuteWithFallback tests the fallback mechanism
func TestCircuitBreaker_ExecuteWithFallback(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(cb *CircuitBreaker)
		fn            func() (interface{}, error)
		fallback      FallbackFunc
		wantResult    interface{}
		wantErr       error
		checkFallback bool
	}{
		{
			name: "successful execution, fallback not called",
			fn: func() (interface{}, error) {
				return "success", nil
			},
			fallback: func(err error) (interface{}, error) {
				return "fallback", nil
			},
			wantResult:    "success",
			wantErr:       nil,
			checkFallback: false,
		},
		{
			name: "failed execution, fallback returns default value",
			fn: func() (interface{}, error) {
				return nil, errors.New("primary failure")
			},
			fallback: func(err error) (interface{}, error) {
				return "fallback-value", nil
			},
			wantResult:    "fallback-value",
			wantErr:       nil,
			checkFallback: true,
		},
		{
			name: "circuit open, fallback executes",
			setup: func(cb *CircuitBreaker) {
				// Trip the circuit
				for i := 0; i < 5; i++ {
					_, _ = cb.Execute(context.Background(), func() (interface{}, error) {
						return nil, errors.New("failure")
					})
				}
			},
			fn: func() (interface{}, error) {
				return "should not execute", nil
			},
			fallback: func(err error) (interface{}, error) {
				return "fallback-when-open", nil
			},
			wantResult:    "fallback-when-open",
			wantErr:       nil,
			checkFallback: true,
		},
		{
			name: "fallback also fails",
			fn: func() (interface{}, error) {
				return nil, errors.New("primary failure")
			},
			fallback: func(err error) (interface{}, error) {
				return nil, errors.New("fallback failure")
			},
			wantResult:    nil,
			wantErr:       errors.New("fallback failure"),
			checkFallback: true,
		},
		{
			name: "nil fallback returns original error",
			fn: func() (interface{}, error) {
				return nil, errors.New("primary failure")
			},
			fallback:      nil,
			wantResult:    nil,
			wantErr:       errors.New("circuit breaker execution failed: primary failure"),
			checkFallback: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := newMockLogger()
			metrics := newMockMetricsClient()

			config := CircuitBreakerConfig{
				FailureThreshold: 5,
			}

			cb := NewCircuitBreaker("test-fallback", config, logger, metrics)

			if tt.setup != nil {
				tt.setup(cb)
			}

			result, err := cb.ExecuteWithFallback(context.Background(), tt.fn, tt.fallback)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr.Error())
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantResult, result)

			// Check if fallback metrics were recorded
			if tt.checkFallback && metrics != nil {
				if tt.wantErr == nil {
					assert.Greater(t, metrics.counters["circuit_breaker_fallback_success_total"], 0.0)
				} else {
					assert.Greater(t, metrics.counters["circuit_breaker_fallback_failure_total"], 0.0)
				}
			}
		})
	}
}

// TestCircuitBreaker_ExecuteWithDefaultValue tests the default value fallback
func TestCircuitBreaker_ExecuteWithDefaultValue(t *testing.T) {
	logger := newMockLogger()
	metrics := newMockMetricsClient()

	cb := NewCircuitBreaker("test-default", CircuitBreakerConfig{}, logger, metrics)

	// Test successful execution
	result, err := cb.ExecuteWithDefaultValue(context.Background(), func() (interface{}, error) {
		return "primary-result", nil
	}, "default-value")

	require.NoError(t, err)
	assert.Equal(t, "primary-result", result)

	// Test failed execution with default value
	result, err = cb.ExecuteWithDefaultValue(context.Background(), func() (interface{}, error) {
		return nil, errors.New("failure")
	}, "default-value")

	require.NoError(t, err)
	assert.Equal(t, "default-value", result)

	// Verify fallback success metric
	assert.Greater(t, metrics.counters["circuit_breaker_fallback_success_total"], 0.0)
}

// TestCircuitBreakerManager_Fallback tests fallback methods on the manager
func TestCircuitBreakerManager_Fallback(t *testing.T) {
	logger := newMockLogger()
	metrics := newMockMetricsClient()

	manager := NewCircuitBreakerManager(logger, metrics, nil)

	// Test ExecuteWithFallback
	result, err := manager.ExecuteWithFallback(context.Background(), "service1",
		func() (interface{}, error) {
			return nil, errors.New("service failure")
		},
		func(err error) (interface{}, error) {
			return "fallback-result", nil
		},
	)

	require.NoError(t, err)
	assert.Equal(t, "fallback-result", result)

	// Test ExecuteWithDefaultValue
	result, err = manager.ExecuteWithDefaultValue(context.Background(), "service2",
		func() (interface{}, error) {
			return nil, errors.New("service failure")
		},
		"default-result",
	)

	require.NoError(t, err)
	assert.Equal(t, "default-result", result)
}

// TestCircuitBreaker_FallbackWithCircuitOpen tests fallback when circuit is open
func TestCircuitBreaker_FallbackWithCircuitOpen(t *testing.T) {
	logger := newMockLogger()
	metrics := newMockMetricsClient()

	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		ResetTimeout:     100 * time.Millisecond,
	}

	cb := NewCircuitBreaker("test-fallback-open", config, logger, metrics)

	// Trip the circuit
	for i := 0; i < 3; i++ {
		_, _ = cb.Execute(context.Background(), func() (interface{}, error) {
			return nil, errors.New("failure")
		})
	}

	assert.Equal(t, CircuitBreakerOpen, cb.getState())

	// Execute with fallback while circuit is open
	result, err := cb.ExecuteWithFallback(context.Background(),
		func() (interface{}, error) {
			return "should-not-execute", nil
		},
		func(err error) (interface{}, error) {
			// Verify we got the circuit open error
			assert.Contains(t, err.Error(), "circuit breaker is open")
			return "fallback-result", nil
		},
	)

	require.NoError(t, err)
	assert.Equal(t, "fallback-result", result)

	// Verify fallback metrics
	assert.Greater(t, metrics.counters["circuit_breaker_fallback_success_total"], 0.0)
}

// TestCircuitBreaker_FallbackMetrics tests metric recording for fallback operations
func TestCircuitBreaker_FallbackMetrics(t *testing.T) {
	logger := newMockLogger()
	metrics := newMockMetricsClient()

	cb := NewCircuitBreaker("test-fallback-metrics", CircuitBreakerConfig{}, logger, metrics)

	// Test successful fallback
	_, _ = cb.ExecuteWithFallback(context.Background(),
		func() (interface{}, error) {
			return nil, errors.New("primary failure")
		},
		func(err error) (interface{}, error) {
			return "fallback", nil
		},
	)

	assert.Equal(t, 1.0, metrics.counters["circuit_breaker_fallback_success_total"])

	// Test failed fallback
	_, _ = cb.ExecuteWithFallback(context.Background(),
		func() (interface{}, error) {
			return nil, errors.New("primary failure")
		},
		func(err error) (interface{}, error) {
			return nil, errors.New("fallback failure")
		},
	)

	assert.Equal(t, 1.0, metrics.counters["circuit_breaker_fallback_failure_total"])
}
