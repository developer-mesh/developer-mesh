package utils

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetryWithBackoff_Success(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2,
	}

	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 2 {
			return errors.New("temporary error")
		}
		return nil // Success on second attempt
	}

	// Set retry condition
	config.RetryIf = func(err error) bool {
		return err.Error() == "temporary error"
	}

	result, err := RetryWithBackoff(context.Background(), config, fn)

	assert.NoError(t, err)
	assert.Equal(t, 2, result.Attempts)
	assert.Equal(t, 2, attempts)
}

func TestRetryWithBackoff_AllAttemptsFail(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		RetryIf:      func(err error) bool { return true },
	}

	attempts := 0
	fn := func() error {
		attempts++
		return errors.New("persistent error")
	}

	result, err := RetryWithBackoff(context.Background(), config, fn)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all 3 attempts failed")
	assert.Equal(t, 3, result.Attempts)
	assert.Equal(t, 3, attempts)
}

func TestRetryWithBackoff_NonRetryableError(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 3,
		RetryIf: func(err error) bool {
			return err.Error() != "fatal error"
		},
	}

	attempts := 0
	fn := func() error {
		attempts++
		return errors.New("fatal error")
	}

	result, err := RetryWithBackoff(context.Background(), config, fn)

	assert.Error(t, err)
	assert.Equal(t, 1, result.Attempts)
	assert.Equal(t, 1, attempts) // Should not retry
}

func TestRetryWithBackoff_ExponentialBackoff(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:  4,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2,
		JitterFactor: 0, // No jitter for predictable testing
	}

	attempts := 0
	delays := []time.Duration{}
	lastTime := time.Now()

	fn := func() error {
		attempts++
		now := time.Now()
		if attempts > 1 {
			// Capture delay between attempts (not on first attempt)
			delays = append(delays, now.Sub(lastTime))
		}
		lastTime = now
		return errors.New("error")
	}

	config.RetryIf = func(err error) bool { return true }

	_, _ = RetryWithBackoff(context.Background(), config, fn)

	// Should have 4 attempts total
	assert.Equal(t, 4, attempts)

	// Verify exponential backoff
	assert.Len(t, delays, 3) // 3 delays between 4 attempts

	// First retry delay: ~100ms
	assert.InDelta(t, 100, delays[0].Milliseconds(), 20)

	// Second retry delay: ~200ms (100ms * 2)
	assert.InDelta(t, 200, delays[1].Milliseconds(), 20)

	// Third retry delay: ~400ms (100ms * 2^2)
	assert.InDelta(t, 400, delays[2].Milliseconds(), 20)
}

func TestRetryableError_Interface(t *testing.T) {
	// Test NetworkError
	netErr := NetworkError{Message: "connection reset"}
	assert.True(t, netErr.IsRetryable())
	assert.Contains(t, netErr.Error(), "connection reset")

	// Test HTTPError - retryable
	httpErr503 := HTTPError{StatusCode: 503, Message: "Service Unavailable"}
	assert.True(t, httpErr503.IsRetryable())

	// Test HTTPError - not retryable
	httpErr400 := HTTPError{StatusCode: 400, Message: "Bad Request"}
	assert.False(t, httpErr400.IsRetryable())

	// Test HTTPError - rate limit (retryable)
	httpErr429 := HTTPError{StatusCode: 429, Message: "Too Many Requests"}
	assert.True(t, httpErr429.IsRetryable())
}

func TestCalculateDelay(t *testing.T) {
	config := &RetryConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2,
		JitterFactor: 0,
	}

	// Test exponential growth
	delay1 := calculateDelay(1, config)
	assert.Equal(t, 100*time.Millisecond, delay1)

	delay2 := calculateDelay(2, config)
	assert.Equal(t, 200*time.Millisecond, delay2)

	delay3 := calculateDelay(3, config)
	assert.Equal(t, 400*time.Millisecond, delay3)

	// Test max delay cap
	delay10 := calculateDelay(10, config)
	assert.Equal(t, 5*time.Second, delay10)
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 3, config.MaxAttempts)
	assert.Equal(t, 1*time.Second, config.InitialDelay)
	assert.Equal(t, 30*time.Second, config.MaxDelay)
	assert.Equal(t, 2.0, config.Multiplier)
	assert.Equal(t, 0.1, config.JitterFactor)
}

func TestRetryWithBackoff_WithRetryableErrors(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialDelay:    10 * time.Millisecond,
		RetryableErrors: []error{ErrTimeout, ErrRateLimit},
	}

	attempts := 0
	fn := func() error {
		attempts++
		switch attempts {
		case 1:
			return ErrTimeout
		case 2:
			return ErrRateLimit
		}
		return nil // Success on third attempt
	}

	result, err := RetryWithBackoff(context.Background(), config, fn)

	assert.NoError(t, err)
	assert.Equal(t, 3, result.Attempts)
	assert.Equal(t, 3, attempts)
}

func TestRetryWithBackoff_WithNetworkError(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
	}

	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 3 {
			return NetworkError{Message: "connection refused"}
		}
		return nil // Success on third attempt
	}

	result, err := RetryWithBackoff(context.Background(), config, fn)

	assert.NoError(t, err)
	assert.Equal(t, 3, result.Attempts)
	assert.Equal(t, 3, attempts)
}

func TestRetryWithBackoff_WithHTTPError(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
	}

	// Test retryable HTTP error
	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 2 {
			return HTTPError{StatusCode: 503, Message: "Service Unavailable"}
		}
		return nil
	}

	result, err := RetryWithBackoff(context.Background(), config, fn)
	assert.NoError(t, err)
	assert.Equal(t, 2, result.Attempts)

	// Test non-retryable HTTP error
	attempts = 0
	fn = func() error {
		attempts++
		return HTTPError{StatusCode: 400, Message: "Bad Request"}
	}

	result, err = RetryWithBackoff(context.Background(), config, fn)
	assert.Error(t, err)
	assert.Equal(t, 1, result.Attempts) // Should not retry
}

func TestCalculateDelay_WithJitter(t *testing.T) {
	config := &RetryConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2,
		JitterFactor: 0.2, // 20% jitter
	}

	// Run multiple times to test jitter
	for i := 0; i < 10; i++ {
		delay := calculateDelay(2, config)
		// With 20% jitter, delay should be 200ms ± 40ms
		assert.InDelta(t, 200, delay.Milliseconds(), 40)
	}
}

func TestRetryWithBackoff_NilConfig(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 2 {
			return errors.New("error")
		}
		return nil
	}

	// Pass nil config to use defaults
	result, err := RetryWithBackoff(context.Background(), nil, fn)

	// Should use default config with no RetryIf function, so won't retry
	assert.Error(t, err)
	assert.Equal(t, 1, result.Attempts)
}

func TestRetryWithBackoff_SuccessOnFirstAttempt(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
	}

	attempts := 0
	fn := func() error {
		attempts++
		return nil // Success immediately
	}

	result, err := RetryWithBackoff(context.Background(), config, fn)

	assert.NoError(t, err)
	assert.Equal(t, 1, result.Attempts)
	assert.Equal(t, 1, attempts)
	assert.Greater(t, result.TotalDuration.Nanoseconds(), int64(0))
}
