package worker

import (
	"context"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/queue"
)

var (
	idempotencyTTL = 24 * time.Hour
)

type QueueClient interface {
	ReceiveEvents(ctx context.Context, maxMessages int32, waitSeconds int32) ([]queue.Event, []string, error)
	DeleteMessage(ctx context.Context, receiptHandle string) error
}

type RedisIdempotency interface {
	Exists(ctx context.Context, key string) (int64, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}

func RunWorker(ctx context.Context, queueClient QueueClient, redisClient RedisIdempotency, processFunc func(queue.Event) error) error {
	for {
		events, handles, err := queueClient.ReceiveEvents(ctx, 5, 10)
		if err != nil {
			return err
		}
		for i, event := range events {
			idKey := "github:webhook:processed:" + event.EventID
			exists, err := redisClient.Exists(ctx, idKey)
			if err != nil {
				continue
			}
			if exists == 1 {
				_ = queueClient.DeleteMessage(ctx, handles[i]) // Already processed, delete
				continue
			}
			err = processFunc(event)
			if err == nil {
				_ = redisClient.Set(ctx, idKey, "1", idempotencyTTL)
				_ = queueClient.DeleteMessage(ctx, handles[i])
			}
		}
	}
}
