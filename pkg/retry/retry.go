package retry

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// Policy defines the retry policy interface
type Policy interface {
	Execute(ctx context.Context, fn func(ctx context.Context) error) error
	NextDelay(attempt int) time.Duration
}

// Config contains retry configuration
type Config struct {
	InitialInterval time.Duration
	MaxInterval     time.Duration
	MaxElapsedTime  time.Duration
	Multiplier      float64
	MaxRetries      int
}

// ExponentialBackoff implements exponential backoff retry policy
type ExponentialBackoff struct {
	config Config
}

// NewExponentialBackoff creates a new exponential backoff retry policy
func NewExponentialBackoff(config Config) Policy {
	if config.InitialInterval <= 0 {
		config.InitialInterval = 100 * time.Millisecond
	}
	if config.MaxInterval <= 0 {
		config.MaxInterval = 30 * time.Second
	}
	if config.MaxElapsedTime <= 0 {
		config.MaxElapsedTime = 5 * time.Minute
	}
	if config.Multiplier <= 1.0 {
		config.Multiplier = 2.0
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 10
	}

	return &ExponentialBackoff{config: config}
}

// Execute executes the function with exponential backoff retry
func (e *ExponentialBackoff) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	start := time.Now()
	attempt := 0

	for {
		err := fn(ctx)
		if err == nil {
			return nil
		}

		attempt++

		// Check if we've exceeded max retries
		if e.config.MaxRetries > 0 && attempt >= e.config.MaxRetries {
			return err
		}

		// Check if we've exceeded max elapsed time
		if time.Since(start) >= e.config.MaxElapsedTime {
			return err
		}

		// Check if context is cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}

		delay := e.NextDelay(attempt)
		select {
		case <-time.After(delay):
			// Continue to next retry
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// NextDelay calculates the next delay with jitter
func (e *ExponentialBackoff) NextDelay(attempt int) time.Duration {
	delay := float64(e.config.InitialInterval) * math.Pow(e.config.Multiplier, float64(attempt-1))

	// Cap at max interval
	if delay > float64(e.config.MaxInterval) {
		delay = float64(e.config.MaxInterval)
	}

	// Add jitter (±20%)
	jitter := delay * 0.2 * (rand.Float64()*2 - 1)
	delay += jitter

	return time.Duration(delay)
}

// FixedDelay implements fixed delay retry policy
type FixedDelay struct {
	delay      time.Duration
	maxRetries int
}

// NewFixedDelay creates a new fixed delay retry policy
func NewFixedDelay(delay time.Duration, maxRetries int) Policy {
	return &FixedDelay{
		delay:      delay,
		maxRetries: maxRetries,
	}
}

// Execute executes the function with fixed delay retry
func (f *FixedDelay) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	attempt := 0

	for {
		err := fn(ctx)
		if err == nil {
			return nil
		}

		attempt++

		// Check if we've exceeded max retries
		if f.maxRetries > 0 && attempt >= f.maxRetries {
			return err
		}

		// Check if context is cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}

		select {
		case <-time.After(f.delay):
			// Continue to next retry
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// NextDelay returns the fixed delay
func (f *FixedDelay) NextDelay(attempt int) time.Duration {
	return f.delay
}
