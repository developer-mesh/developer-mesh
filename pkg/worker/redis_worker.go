package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
)

// RedisIdempotency interface for idempotency checks
type RedisIdempotency interface {
	Exists(ctx context.Context, key string) (int64, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}

// QueueClient interface for queue operations
type QueueClient interface {
	ReceiveEvents(ctx context.Context, maxMessages int32, waitSeconds int32) ([]queue.Event, []string, error)
	DeleteMessage(ctx context.Context, receiptHandle string) error
}

// RedisWorker represents a worker that processes events from Redis
type RedisWorker struct {
	queueClient    QueueClient
	redisClient    RedisIdempotency
	processor      func(queue.Event) error
	logger         observability.Logger
	consumerName   string
	idempotencyTTL time.Duration
}

// Config holds configuration for the Redis worker
type Config struct {
	QueueClient    QueueClient
	RedisClient    RedisIdempotency
	Processor      func(queue.Event) error
	Logger         observability.Logger
	ConsumerName   string
	IdempotencyTTL time.Duration
}

// NewRedisWorker creates a new Redis worker
func NewRedisWorker(config *Config) (*RedisWorker, error) {
	if config.QueueClient == nil {
		return nil, fmt.Errorf("queue client is required")
	}
	if config.RedisClient == nil {
		return nil, fmt.Errorf("redis client is required")
	}
	if config.Processor == nil {
		return nil, fmt.Errorf("processor function is required")
	}
	if config.Logger == nil {
		config.Logger = observability.NewNoopLogger()
	}
	if config.ConsumerName == "" {
		config.ConsumerName = "worker"
	}
	if config.IdempotencyTTL == 0 {
		config.IdempotencyTTL = 24 * time.Hour
	}

	return &RedisWorker{
		queueClient:    config.QueueClient,
		redisClient:    config.RedisClient,
		processor:      config.Processor,
		logger:         config.Logger,
		consumerName:   config.ConsumerName,
		idempotencyTTL: config.IdempotencyTTL,
	}, nil
}

// Run starts the worker processing loop
func (w *RedisWorker) Run(ctx context.Context) error {
	w.logger.Info("Starting Redis worker", map[string]interface{}{
		"consumer_name": w.consumerName,
	})

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Worker stopping due to context cancellation", nil)
			return ctx.Err()
		default:
			// Continue processing
		}

		// Receive events from Redis
		events, handles, err := w.queueClient.ReceiveEvents(ctx, 10, 5)
		if err != nil {
			w.logger.Error("Failed to receive events", map[string]interface{}{
				"error": err.Error(),
			})
			// Sleep briefly on error to avoid tight loop
			time.Sleep(time.Second)
			continue
		}

		// Process each event
		for i, event := range events {
			if err := w.processEvent(ctx, event, handles[i]); err != nil {
				w.logger.Error("Failed to process event", map[string]interface{}{
					"event_id": event.EventID,
					"error":    err.Error(),
				})
			}
		}
	}
}

// processEvent processes a single event with idempotency checking
func (w *RedisWorker) processEvent(ctx context.Context, event queue.Event, handle string) error {
	// Build idempotency key
	idKey := fmt.Sprintf("webhook:processed:%s", event.EventID)

	// Check if already processed
	exists, err := w.redisClient.Exists(ctx, idKey)
	if err != nil {
		w.logger.Error("Failed to check idempotency", map[string]interface{}{
			"event_id": event.EventID,
			"error":    err.Error(),
		})
		// Continue processing on Redis error
	} else if exists == 1 {
		w.logger.Info("Event already processed, acknowledging", map[string]interface{}{
			"event_id": event.EventID,
		})
		// Acknowledge the message to remove from queue
		return w.queueClient.DeleteMessage(ctx, handle)
	}

	// Start processing timer
	start := time.Now()

	// Process the event
	err = w.processor(event)

	// Record processing duration
	duration := time.Since(start)

	if err != nil {
		w.logger.Error("Event processing failed", map[string]interface{}{
			"event_id":    event.EventID,
			"event_type":  event.EventType,
			"duration_ms": duration.Milliseconds(),
			"error":       err.Error(),
		})
		// Don't acknowledge - let it retry
		return err
	}

	// Mark as processed
	if err := w.redisClient.Set(ctx, idKey, "1", w.idempotencyTTL); err != nil {
		w.logger.Error("Failed to set idempotency key", map[string]interface{}{
			"event_id": event.EventID,
			"error":    err.Error(),
		})
		// Continue - we've already processed successfully
	}

	// Acknowledge the message
	if err := w.queueClient.DeleteMessage(ctx, handle); err != nil {
		w.logger.Error("Failed to acknowledge message", map[string]interface{}{
			"event_id": event.EventID,
			"handle":   handle,
			"error":    err.Error(),
		})
		return err
	}

	w.logger.Info("Event processed successfully", map[string]interface{}{
		"event_id":    event.EventID,
		"event_type":  event.EventType,
		"duration_ms": duration.Milliseconds(),
	})

	return nil
}
