package health

import (
	"context"
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// QueueHealthChecker interface for queue health checks
type QueueHealthChecker interface {
	Health(ctx context.Context) error
	GetQueueDepth(ctx context.Context) (int64, error)
}

// RedisQueueHealthCheck implements health checks for Redis-based queues
type RedisQueueHealthCheck struct {
	queueClient QueueHealthChecker
	logger      observability.Logger
}

// NewRedisQueueHealthCheck creates a new Redis queue health check
func NewRedisQueueHealthCheck(queueClient QueueHealthChecker, logger observability.Logger) *RedisQueueHealthCheck {
	return &RedisQueueHealthCheck{
		queueClient: queueClient,
		logger:      logger,
	}
}

// Check performs the health check
func (r *RedisQueueHealthCheck) Check(ctx context.Context) error {
	// Check queue health
	if err := r.queueClient.Health(ctx); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}

	// Check queue depth
	depth, err := r.queueClient.GetQueueDepth(ctx)
	if err != nil {
		r.logger.Warn("Failed to get queue depth", map[string]interface{}{
			"error": err.Error(),
		})
	} else {
		r.logger.Info("Queue depth check", map[string]interface{}{
			"queue_depth": depth,
		})

		// Log warning if queue is getting too deep
		if depth > 1000 {
			r.logger.Warn("Queue depth is high", map[string]interface{}{
				"queue_depth": depth,
			})
		}
	}

	return nil
}

// Name returns the name of this health check
func (r *RedisQueueHealthCheck) Name() string {
	return "redis-queue"
}
