package redis

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
)

func TestNewCircuitBreaker(t *testing.T) {
	logger := observability.NewNoopLogger()

	t.Run("Creates circuit breaker with default config", func(t *testing.T) {
		cb := NewCircuitBreaker(nil, logger)
		assert.NotNil(t, cb)
		assert.Equal(t, StateClosed, cb.GetState())
	})

	t.Run("Creates circuit breaker with custom config", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			FailureThreshold:  10,
			SuccessThreshold:  3,
			Timeout:           1 * time.Minute,
			MaxTimeout:        10 * time.Minute,
			TimeoutMultiplier: 3.0,
		}
		cb := NewCircuitBreaker(config, logger)
		assert.NotNil(t, cb)
		assert.Equal(t, config.FailureThreshold, cb.config.FailureThreshold)
	})
}

func TestCircuitBreaker_Execute_Success(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := &CircuitBreakerConfig{
		FailureThreshold:  3,
		SuccessThreshold:  2,
		Timeout:           100 * time.Millisecond,
		MaxTimeout:        1 * time.Second,
		TimeoutMultiplier: 2.0,
	}
	cb := NewCircuitBreaker(config, logger)
	ctx := context.Background()

	t.Run("Executes successful operation", func(t *testing.T) {
		var called bool
		err := cb.Execute(ctx, func() error {
			called = true
			return nil
		})
		assert.NoError(t, err)
		assert.True(t, called)
		assert.Equal(t, StateClosed, cb.GetState())
	})

	t.Run("Remains closed with successful operations", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			err := cb.Execute(ctx, func() error {
				return nil
			})
			assert.NoError(t, err)
		}
		assert.Equal(t, StateClosed, cb.GetState())
	})
}

func TestCircuitBreaker_Execute_Failure(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := &CircuitBreakerConfig{
		FailureThreshold:  3,
		SuccessThreshold:  2,
		Timeout:           100 * time.Millisecond,
		MaxTimeout:        1 * time.Second,
		TimeoutMultiplier: 2.0,
	}
	cb := NewCircuitBreaker(config, logger)
	ctx := context.Background()

	t.Run("Opens after failure threshold", func(t *testing.T) {
		testErr := errors.New("test error")

		// Fail until threshold
		for i := 0; i < config.FailureThreshold; i++ {
			err := cb.Execute(ctx, func() error {
				return testErr
			})
			assert.Error(t, err)
		}

		// Circuit should be open
		assert.Equal(t, StateOpen, cb.GetState())

		// Should fail fast when open
		var called bool
		err := cb.Execute(ctx, func() error {
			called = true
			return nil
		})
		assert.Error(t, err)
		assert.Equal(t, ErrCircuitOpen, err)
		assert.False(t, called)
	})
}

func TestCircuitBreaker_HalfOpen_Recovery(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := &CircuitBreakerConfig{
		FailureThreshold:  2,
		SuccessThreshold:  2,
		Timeout:           50 * time.Millisecond,
		MaxTimeout:        1 * time.Second,
		TimeoutMultiplier: 2.0,
	}
	cb := NewCircuitBreaker(config, logger)
	ctx := context.Background()

	t.Run("Transitions to half-open and recovers", func(t *testing.T) {
		// Open the circuit
		for i := 0; i < config.FailureThreshold; i++ {
			_ = cb.Execute(ctx, func() error {
				return errors.New("fail")
			})
		}
		assert.Equal(t, StateOpen, cb.GetState())

		// Wait for timeout
		time.Sleep(config.Timeout + 10*time.Millisecond)

		// First success moves to half-open
		err := cb.Execute(ctx, func() error {
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, StateHalfOpen, cb.GetState())

		// More successes close the circuit
		for i := 1; i < config.SuccessThreshold; i++ {
			err = cb.Execute(ctx, func() error {
				return nil
			})
			assert.NoError(t, err)
		}
		assert.Equal(t, StateClosed, cb.GetState())
	})
}

func TestCircuitBreaker_HalfOpen_Failure(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := &CircuitBreakerConfig{
		FailureThreshold:  2,
		SuccessThreshold:  2,
		Timeout:           50 * time.Millisecond,
		MaxTimeout:        1 * time.Second,
		TimeoutMultiplier: 2.0,
	}
	cb := NewCircuitBreaker(config, logger)
	ctx := context.Background()

	t.Run("Returns to open on half-open failure", func(t *testing.T) {
		// Open the circuit
		for i := 0; i < config.FailureThreshold; i++ {
			_ = cb.Execute(ctx, func() error {
				return errors.New("fail")
			})
		}
		assert.Equal(t, StateOpen, cb.GetState())

		// Wait for timeout
		time.Sleep(config.Timeout + 10*time.Millisecond)

		// Fail in half-open state
		err := cb.Execute(ctx, func() error {
			return errors.New("fail again")
		})
		assert.Error(t, err)
		assert.Equal(t, StateOpen, cb.GetState())

		// Timeout should have increased
		stats := cb.GetStats()
		currentTimeout := stats["current_timeout"].(time.Duration)
		assert.True(t, currentTimeout > config.Timeout)
	})
}

func TestCircuitBreaker_ExecuteWithResult(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultCircuitBreakerConfig()
	cb := NewCircuitBreaker(config, logger)
	ctx := context.Background()

	t.Run("Returns result on success", func(t *testing.T) {
		expectedResult := "test result"
		result, err := cb.ExecuteWithResult(ctx, func() (interface{}, error) {
			return expectedResult, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("Returns error on failure", func(t *testing.T) {
		testErr := errors.New("test error")
		result, err := cb.ExecuteWithResult(ctx, func() (interface{}, error) {
			return nil, testErr
		})
		assert.Error(t, err)
		assert.Equal(t, testErr, err)
		assert.Nil(t, result)
	})

	t.Run("Fails fast when open", func(t *testing.T) {
		// Open the circuit
		for i := 0; i < config.FailureThreshold; i++ {
			_, _ = cb.ExecuteWithResult(ctx, func() (interface{}, error) {
				return nil, errors.New("fail")
			})
		}

		result, err := cb.ExecuteWithResult(ctx, func() (interface{}, error) {
			return "should not execute", nil
		})
		assert.Error(t, err)
		assert.Equal(t, ErrCircuitOpen, err)
		assert.Nil(t, result)
	})
}

func TestCircuitBreaker_Concurrent(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := &CircuitBreakerConfig{
		FailureThreshold:  5,
		SuccessThreshold:  3,
		Timeout:           100 * time.Millisecond,
		MaxTimeout:        1 * time.Second,
		TimeoutMultiplier: 2.0,
	}
	cb := NewCircuitBreaker(config, logger)
	ctx := context.Background()

	t.Run("Handles concurrent executions", func(t *testing.T) {
		var wg sync.WaitGroup
		var successCount, failureCount int32

		// Run concurrent operations
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				err := cb.Execute(ctx, func() error {
					if i%3 == 0 {
						return errors.New("fail")
					}
					return nil
				})
				if err != nil {
					atomic.AddInt32(&failureCount, 1)
				} else {
					atomic.AddInt32(&successCount, 1)
				}
			}(i)
		}

		wg.Wait()

		// Should have processed all requests
		total := atomic.LoadInt32(&successCount) + atomic.LoadInt32(&failureCount)
		assert.Equal(t, int32(100), total)
	})
}

func TestCircuitBreaker_GetStats(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := &CircuitBreakerConfig{
		FailureThreshold:  3,
		SuccessThreshold:  2,
		Timeout:           100 * time.Millisecond,
		MaxTimeout:        1 * time.Second,
		TimeoutMultiplier: 2.0,
	}
	cb := NewCircuitBreaker(config, logger)
	ctx := context.Background()

	t.Run("Returns accurate statistics", func(t *testing.T) {
		// Initial stats
		stats := cb.GetStats()
		assert.Equal(t, "closed", stats["state"])
		assert.Equal(t, 0, stats["failures"])
		assert.Equal(t, 0, stats["successes"])

		// Add some failures
		for i := 0; i < 2; i++ {
			_ = cb.Execute(ctx, func() error {
				return errors.New("fail")
			})
		}

		stats = cb.GetStats()
		assert.Equal(t, 2, stats["failures"])

		// Add a success
		_ = cb.Execute(ctx, func() error {
			return nil
		})

		stats = cb.GetStats()
		assert.Equal(t, 0, stats["failures"]) // Reset on success in closed state
		assert.Contains(t, stats, "generation")
		assert.Contains(t, stats, "current_timeout")
	})
}

func TestCircuitBreaker_TimeoutBackoff(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := &CircuitBreakerConfig{
		FailureThreshold:  2,
		SuccessThreshold:  1,
		Timeout:           50 * time.Millisecond,
		MaxTimeout:        200 * time.Millisecond,
		TimeoutMultiplier: 2.0,
	}
	cb := NewCircuitBreaker(config, logger)
	ctx := context.Background()

	t.Run("Increases timeout with backoff", func(t *testing.T) {
		timeouts := []time.Duration{}

		for i := 0; i < 3; i++ {
			// Open the circuit
			for j := 0; j < config.FailureThreshold; j++ {
				_ = cb.Execute(ctx, func() error {
					return errors.New("fail")
				})
			}

			// Record timeout
			stats := cb.GetStats()
			timeouts = append(timeouts, stats["current_timeout"].(time.Duration))

			// Wait and fail in half-open
			time.Sleep(timeouts[i] + 10*time.Millisecond)
			_ = cb.Execute(ctx, func() error {
				return errors.New("fail")
			})
		}

		// Check that timeouts increased with multiplier
		assert.True(t, timeouts[1] > timeouts[0])
		assert.True(t, timeouts[2] > timeouts[1])
		assert.True(t, timeouts[2] <= config.MaxTimeout)
	})
}

func TestCircuitBreaker_ContextCancellation(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultCircuitBreakerConfig()
	cb := NewCircuitBreaker(config, logger)

	t.Run("Respects context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := cb.Execute(ctx, func() error {
			return nil
		})
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("Respects context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		time.Sleep(10 * time.Millisecond) // Ensure timeout

		err := cb.Execute(ctx, func() error {
			return nil
		})
		assert.Error(t, err)
	})
}
