package worker

import (
	"context"
	"math"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
)

// RetryConfig defines configuration for retry behavior
type RetryConfig struct {
	MaxRetries      int
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Multiplier      float64
	MaxElapsedTime  time.Duration
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:      5,
		InitialInterval: 1 * time.Second,
		MaxInterval:     5 * time.Minute,
		Multiplier:      2.0,
		MaxElapsedTime:  30 * time.Minute,
	}
}

// RetryHandler handles retries with exponential backoff
type RetryHandler struct {
	config  *RetryConfig
	logger  observability.Logger
	dlq     DLQHandler
	metrics *MetricsCollector
}

// NewRetryHandler creates a new retry handler
func NewRetryHandler(config *RetryConfig, logger observability.Logger, dlq DLQHandler, metrics *MetricsCollector) *RetryHandler {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &RetryHandler{
		config:  config,
		logger:  logger,
		dlq:     dlq,
		metrics: metrics,
	}
}

// ExecuteWithRetry executes a function with exponential backoff retry
func (r *RetryHandler) ExecuteWithRetry(ctx context.Context, event queue.Event, fn func() error) error {
	// Create exponential backoff configuration
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = r.config.InitialInterval
	b.MaxInterval = r.config.MaxInterval
	b.Multiplier = r.config.Multiplier
	b.MaxElapsedTime = r.config.MaxElapsedTime

	// Wrap with max retries (ensure positive value)
	maxRetries := r.config.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	// #nosec G115 -- maxRetries is validated to be non-negative above, safe for uint64 conversion
	backoffWithRetries := backoff.WithMaxRetries(b, uint64(maxRetries))

	// Track retry attempts
	attempt := 0

	operation := func() error {
		attempt++

		r.logger.Debug("Executing operation", map[string]interface{}{
			"event_id": event.EventID,
			"attempt":  attempt,
		})

		err := fn()

		if err != nil {
			// Check if error is retryable
			if !r.isRetryableError(err) {
				r.logger.Error("Non-retryable error encountered", map[string]interface{}{
					"event_id": event.EventID,
					"attempt":  attempt,
					"error":    err.Error(),
				})
				// Don't retry, but send to DLQ
				return backoff.Permanent(err)
			}

			r.logger.Warn("Retryable error encountered", map[string]interface{}{
				"event_id":    event.EventID,
				"attempt":     attempt,
				"max_retries": r.config.MaxRetries,
				"error":       err.Error(),
			})

			// Record retry attempt
			if r.metrics != nil {
				r.metrics.RecordRetryAttempt(ctx, event.EventID, attempt, err.Error())
			}

			return err
		}

		return nil
	}

	// Execute with backoff
	err := backoff.Retry(operation, backoff.WithContext(backoffWithRetries, ctx))

	if err != nil {
		// Max retries exceeded or permanent error - send to DLQ
		r.logger.Error("Max retries exceeded or permanent error", map[string]interface{}{
			"event_id":       event.EventID,
			"total_attempts": attempt,
			"error":          err.Error(),
		})

		if r.dlq != nil {
			if dlqErr := r.dlq.SendToDLQ(ctx, event, err); dlqErr != nil {
				r.logger.Error("Failed to send event to DLQ", map[string]interface{}{
					"event_id": event.EventID,
					"error":    dlqErr.Error(),
				})
			}
		}

		return err
	}

	if attempt > 1 {
		r.logger.Info("Operation succeeded after retries", map[string]interface{}{
			"event_id":       event.EventID,
			"total_attempts": attempt,
		})
	}

	return nil
}

// isRetryableError determines if an error should trigger a retry
func (r *RetryHandler) isRetryableError(err error) bool {
	// TODO: Add more sophisticated error classification
	// For now, consider these errors as non-retryable:
	nonRetryableErrors := []string{
		"validation failed",
		"invalid payload",
		"tool not found",
		"tool is not active",
		"webhook config is disabled",
		"unauthorized",
		"forbidden",
	}

	errMsg := err.Error()
	for _, nonRetryable := range nonRetryableErrors {
		if contains(errMsg, nonRetryable) {
			return false
		}
	}

	// All other errors are retryable (network issues, timeouts, etc.)
	return true
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(containsHelper(s, substr) || contains(s[1:], substr)))
}

func containsHelper(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i < len(substr); i++ {
		if s[i] != substr[i] && s[i]+32 != substr[i] && s[i]-32 != substr[i] {
			return false
		}
	}
	return true
}

// GetBackoffDuration calculates the backoff duration for a given attempt
func (r *RetryHandler) GetBackoffDuration(attempt int) time.Duration {
	if attempt <= 0 {
		return r.config.InitialInterval
	}

	duration := float64(r.config.InitialInterval) * math.Pow(r.config.Multiplier, float64(attempt-1))

	if duration > float64(r.config.MaxInterval) {
		return r.config.MaxInterval
	}

	return time.Duration(duration)
}
