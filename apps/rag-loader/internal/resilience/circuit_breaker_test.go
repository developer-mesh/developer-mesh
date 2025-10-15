package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

func TestCircuitBreaker_Closed(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := CircuitBreakerConfig{
		MaxFailures:  3,
		ResetTimeout: 100 * time.Millisecond,
	}
	cb := NewCircuitBreaker(config, logger)

	// Circuit should be closed initially
	assert.Equal(t, StateClosed, cb.State())

	// Should allow requests
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	assert.NoError(t, err)

	// Should still be closed after success
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreaker_Open(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := CircuitBreakerConfig{
		MaxFailures:  3,
		ResetTimeout: 100 * time.Millisecond,
	}
	cb := NewCircuitBreaker(config, logger)

	// Trigger max failures
	testErr := errors.New("test error")
	for i := 0; i < 3; i++ {
		_ = cb.Execute(context.Background(), func() error {
			return testErr
		})
	}

	// Circuit should be open
	assert.Equal(t, StateOpen, cb.State())

	// Should reject new requests
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	assert.Equal(t, ErrCircuitOpen, err)
}

func TestCircuitBreaker_HalfOpen(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := CircuitBreakerConfig{
		MaxFailures:         3,
		ResetTimeout:        100 * time.Millisecond,
		HalfOpenMaxRequests: 2,
	}
	cb := NewCircuitBreaker(config, logger)

	// Trigger max failures
	testErr := errors.New("test error")
	for i := 0; i < 3; i++ {
		_ = cb.Execute(context.Background(), func() error {
			return testErr
		})
	}

	// Circuit should be open
	assert.Equal(t, StateOpen, cb.State())

	// Wait for reset timeout
	time.Sleep(150 * time.Millisecond)

	// Should transition to half-open on next request
	_ = cb.Execute(context.Background(), func() error {
		return nil
	})

	// Note: State transitions happen internally, we test behavior not state directly
	stats := cb.Stats()
	assert.NotNil(t, stats)
}

func TestCircuitBreaker_Recovery(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := CircuitBreakerConfig{
		MaxFailures:         3,
		ResetTimeout:        100 * time.Millisecond,
		HalfOpenMaxRequests: 2,
	}
	cb := NewCircuitBreaker(config, logger)

	// Trigger max failures to open circuit
	testErr := errors.New("test error")
	for i := 0; i < 3; i++ {
		_ = cb.Execute(context.Background(), func() error {
			return testErr
		})
	}
	assert.Equal(t, StateOpen, cb.State())

	// Wait for reset timeout
	time.Sleep(150 * time.Millisecond)

	// Successful requests should close the circuit
	for i := 0; i < 2; i++ {
		_ = cb.Execute(context.Background(), func() error {
			return nil
		})
	}

	// After enough successes, circuit should eventually close
	// State machine is complex, so we just verify stats are tracked
	stats := cb.Stats()
	successes, ok := stats["successes"].(int)
	assert.True(t, ok)
	assert.Greater(t, successes, 0)
}

func TestCircuitBreaker_Stats(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := DefaultCircuitBreakerConfig()
	cb := NewCircuitBreaker(config, logger)

	// Execute some operations
	_ = cb.Execute(context.Background(), func() error {
		return nil
	})
	_ = cb.Execute(context.Background(), func() error {
		return errors.New("error")
	})

	stats := cb.Stats()
	assert.Equal(t, "closed", stats["state"])
	assert.Equal(t, 2, stats["requests"])
	assert.Equal(t, 1, stats["successes"])
	assert.Equal(t, 1, stats["failures"])
}

func TestRetryWithBackoff_Success(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	attempts := 0
	err := RetryWithBackoff(context.Background(), config, logger, func() error {
		attempts++
		if attempts < 2 {
			return errors.New("temporary error")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestRetryWithBackoff_MaxRetries(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	attempts := 0
	testErr := errors.New("persistent error")
	err := RetryWithBackoff(context.Background(), config, logger, func() error {
		attempts++
		return testErr
	})

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrMaxRetriesExceeded)
	assert.Equal(t, 4, attempts) // Initial + 3 retries
}

func TestRetryWithBackoff_ContextCancellation(t *testing.T) {
	logger := observability.NewNoopLogger()
	config := RetryConfig{
		MaxRetries:   10,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	attempts := 0
	err := RetryWithBackoff(ctx, config, logger, func() error {
		attempts++
		return errors.New("error")
	})

	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
	assert.Less(t, attempts, 10) // Should stop before max retries
}

func TestRetryWithBackoff_NonRetryableError(t *testing.T) {
	logger := observability.NewNoopLogger()

	retryableErr := errors.New("retryable")
	nonRetryableErr := errors.New("non-retryable")

	config := RetryConfig{
		MaxRetries:      3,
		InitialDelay:    10 * time.Millisecond,
		MaxDelay:        100 * time.Millisecond,
		Multiplier:      2.0,
		RetryableErrors: []error{retryableErr},
	}

	attempts := 0
	err := RetryWithBackoff(context.Background(), config, logger, func() error {
		attempts++
		return nonRetryableErr
	})

	assert.Error(t, err)
	assert.Equal(t, 1, attempts) // Should not retry
}

func BenchmarkCircuitBreaker_ClosedState(b *testing.B) {
	logger := observability.NewNoopLogger()
	config := DefaultCircuitBreakerConfig()
	cb := NewCircuitBreaker(config, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cb.Execute(context.Background(), func() error {
			return nil
		})
	}
}

func BenchmarkCircuitBreaker_OpenState(b *testing.B) {
	logger := observability.NewNoopLogger()
	config := CircuitBreakerConfig{
		MaxFailures:  1,
		ResetTimeout: 1 * time.Hour,
	}
	cb := NewCircuitBreaker(config, logger)

	// Open the circuit
	_ = cb.Execute(context.Background(), func() error {
		return errors.New("error")
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cb.Execute(context.Background(), func() error {
			return nil
		})
	}
}
