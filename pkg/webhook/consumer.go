package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/redis"
	redisclient "github.com/go-redis/redis/v8"
)

// ConsumerConfig contains configuration for the webhook consumer
type ConsumerConfig struct {
	// Consumer group settings
	ConsumerGroup string
	ConsumerID    string
	StreamKey     string

	// Processing settings
	BatchSize        int64
	ClaimMinIdleTime time.Duration
	MaxRetries       int
	BlockTimeout     time.Duration

	// Worker settings
	NumWorkers     int
	ProcessTimeout time.Duration

	// Dead letter settings
	DeadLetterStream string
	MaxDeadLetterAge time.Duration
}

// DefaultConsumerConfig returns default consumer configuration
func DefaultConsumerConfig() *ConsumerConfig {
	return &ConsumerConfig{
		ConsumerGroup:    "webhook-consumers",
		StreamKey:        "webhook:events",
		BatchSize:        10,
		ClaimMinIdleTime: 30 * time.Second,
		MaxRetries:       3,
		BlockTimeout:     5 * time.Second,
		NumWorkers:       5,
		ProcessTimeout:   30 * time.Second,
		DeadLetterStream: "webhook:events:dlq",
		MaxDeadLetterAge: 7 * 24 * time.Hour,
	}
}

// WebhookConsumer consumes webhook events from Redis Streams
type WebhookConsumer struct {
	config         *ConsumerConfig
	redisClient    *redis.StreamsClient
	deduplicator   *Deduplicator
	schemaRegistry *SchemaRegistry
	lifecycle      *ContextLifecycleManager
	logger         observability.Logger

	// Event processor
	processor EventProcessor

	// Worker management
	workers sync.WaitGroup
	stopCh  chan struct{}

	// Metrics
	metrics ConsumerMetrics

	// Circuit breakers per tool
	toolBreakers map[string]*redis.CircuitBreaker
	breakerMu    sync.RWMutex
}

// EventProcessor defines the interface for processing webhook events
type EventProcessor interface {
	ProcessEvent(ctx context.Context, event *WebhookEvent) error
	GetToolID() string
}

// ConsumerMetrics tracks consumer statistics
type ConsumerMetrics struct {
	mu                 sync.RWMutex
	EventsProcessed    int64
	EventsFailed       int64
	EventsRetried      int64
	EventsDeadLettered int64
	ProcessingTime     map[string]time.Duration
	LastProcessedTime  time.Time
}

// NewWebhookConsumer creates a new webhook consumer
func NewWebhookConsumer(
	config *ConsumerConfig,
	redisClient *redis.StreamsClient,
	deduplicator *Deduplicator,
	schemaRegistry *SchemaRegistry,
	lifecycle *ContextLifecycleManager,
	processor EventProcessor,
	logger observability.Logger,
) (*WebhookConsumer, error) {
	if config == nil {
		config = DefaultConsumerConfig()
	}

	// Generate consumer ID if not provided
	if config.ConsumerID == "" {
		config.ConsumerID = fmt.Sprintf("consumer-%s-%d", processor.GetToolID(), time.Now().Unix())
	}

	consumer := &WebhookConsumer{
		config:         config,
		redisClient:    redisClient,
		deduplicator:   deduplicator,
		schemaRegistry: schemaRegistry,
		lifecycle:      lifecycle,
		processor:      processor,
		logger:         logger,
		stopCh:         make(chan struct{}),
		toolBreakers:   make(map[string]*redis.CircuitBreaker),
		metrics: ConsumerMetrics{
			ProcessingTime: make(map[string]time.Duration),
		},
	}

	// Create consumer group if it doesn't exist
	if err := consumer.ensureConsumerGroup(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure consumer group: %w", err)
	}

	return consumer, nil
}

// ensureConsumerGroup ensures the consumer group exists
func (c *WebhookConsumer) ensureConsumerGroup(ctx context.Context) error {
	client := c.redisClient.GetClient()

	// Try to create the consumer group
	err := client.XGroupCreateMkStream(ctx, c.config.StreamKey, c.config.ConsumerGroup, "$").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	// Also create group for dead letter stream
	err = client.XGroupCreateMkStream(ctx, c.config.DeadLetterStream, c.config.ConsumerGroup, "$").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("failed to create dead letter consumer group: %w", err)
	}

	c.logger.Info("Consumer group ready", map[string]interface{}{
		"consumer_group": c.config.ConsumerGroup,
		"stream_key":     c.config.StreamKey,
		"consumer_id":    c.config.ConsumerID,
	})

	return nil
}

// Start starts the consumer workers
func (c *WebhookConsumer) Start() error {
	c.logger.Info("Starting webhook consumer", map[string]interface{}{
		"num_workers": c.config.NumWorkers,
		"consumer_id": c.config.ConsumerID,
	})

	// Start worker goroutines
	for i := 0; i < c.config.NumWorkers; i++ {
		c.workers.Add(1)
		go c.worker(i)
	}

	// Start pending message claimer
	c.workers.Add(1)
	go c.claimPendingMessages()

	// Start metrics reporter
	c.workers.Add(1)
	go c.reportMetrics()

	return nil
}

// Stop gracefully stops the consumer
func (c *WebhookConsumer) Stop() {
	c.logger.Info("Stopping webhook consumer", nil)
	close(c.stopCh)
	c.workers.Wait()
	c.logger.Info("Webhook consumer stopped", nil)
}

// worker is the main worker loop
func (c *WebhookConsumer) worker(workerID int) {
	defer c.workers.Done()

	c.logger.Info("Worker started", map[string]interface{}{
		"worker_id": workerID,
	})

	for {
		select {
		case <-c.stopCh:
			c.logger.Info("Worker stopping", map[string]interface{}{
				"worker_id": workerID,
			})
			return
		default:
			c.consumeMessages(workerID)
		}
	}
}

// consumeMessages consumes and processes messages
func (c *WebhookConsumer) consumeMessages(workerID int) {
	ctx, cancel := context.WithTimeout(context.Background(), c.config.BlockTimeout)
	defer cancel()

	client := c.redisClient.GetClient()

	// Read messages from the stream
	messages, err := client.XReadGroup(ctx, &redisclient.XReadGroupArgs{
		Group:    c.config.ConsumerGroup,
		Consumer: c.config.ConsumerID,
		Streams:  []string{c.config.StreamKey, ">"},
		Count:    c.config.BatchSize,
		Block:    c.config.BlockTimeout,
	}).Result()

	if err != nil {
		if err != redisclient.Nil { // redis.Nil means no messages available
			c.logger.Error("Failed to read messages", map[string]interface{}{
				"error":     err.Error(),
				"worker_id": workerID,
			})
		}
		return
	}

	// Process each message
	for _, stream := range messages {
		for _, message := range stream.Messages {
			c.processMessage(workerID, message)
		}
	}
}

// processMessage processes a single message
func (c *WebhookConsumer) processMessage(workerID int, message redisclient.XMessage) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), c.config.ProcessTimeout)
	defer cancel()

	c.logger.Debug("Processing message", map[string]interface{}{
		"message_id": message.ID,
		"worker_id":  workerID,
	})

	// Extract event data
	eventData, ok := message.Values["event"].(string)
	if !ok {
		c.logger.Error("Invalid message format", map[string]interface{}{
			"message_id": message.ID,
		})
		c.acknowledgeMessage(ctx, message.ID)
		return
	}

	// Unmarshal webhook event
	var event WebhookEvent
	if err := json.Unmarshal([]byte(eventData), &event); err != nil {
		c.logger.Error("Failed to unmarshal event", map[string]interface{}{
			"message_id": message.ID,
			"error":      err.Error(),
		})
		c.handleProcessingError(ctx, message, &event, err)
		return
	}

	// Check circuit breaker for this tool
	breaker := c.getToolBreaker(event.ToolId)

	err := breaker.Execute(ctx, func() error {
		// Marshal payload for deduplication
		payloadBytes, err := json.Marshal(event.Payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}

		// Check for duplicates
		dedupResult, err := c.deduplicator.ProcessEvent(
			ctx,
			event.ToolId,
			event.ToolType,
			event.EventType,
			payloadBytes,
		)
		if err != nil {
			return fmt.Errorf("deduplication check failed: %w", err)
		}

		if dedupResult.IsDuplicate {
			c.logger.Info("Duplicate event detected", map[string]interface{}{
				"message_id":  message.ID,
				"event_id":    event.EventId,
				"original_id": dedupResult.OriginalEventID,
			})
			return nil // Not an error, just skip processing
		}

		// Process the event
		if err := c.processor.ProcessEvent(ctx, &event); err != nil {
			return fmt.Errorf("event processing failed: %w", err)
		}

		// Store context if needed
		if event.ProcessingInfo != nil && event.ProcessingInfo.GeneratedContext != nil {
			contextData := make(map[string]interface{})
			if err := json.Unmarshal([]byte(event.ProcessingInfo.GeneratedContext.Value), &contextData); err == nil {
				metadata := &ContextMetadata{
					TenantID:   event.TenantId,
					SourceType: "webhook",
					SourceID:   event.EventId,
					Importance: 0.7, // Default importance for webhook events
					Tags:       []string{event.ToolType, event.EventType},
				}

				if err := c.lifecycle.StoreContext(ctx, event.TenantId, contextData, metadata); err != nil {
					c.logger.Error("Failed to store context", map[string]interface{}{
						"error":    err.Error(),
						"event_id": event.EventId,
					})
				}
			}
		}

		return nil
	})

	if err != nil {
		c.handleProcessingError(ctx, message, &event, err)
		return
	}

	// Acknowledge the message
	c.acknowledgeMessage(ctx, message.ID)

	// Update metrics
	c.updateMetrics(event.EventType, time.Since(start))
}

// handleProcessingError handles errors during message processing
func (c *WebhookConsumer) handleProcessingError(ctx context.Context, message redisclient.XMessage, event *WebhookEvent, err error) {
	retryCount := c.getRetryCount(message)

	c.logger.Error("Failed to process message", map[string]interface{}{
		"message_id":  message.ID,
		"event_id":    event.EventId,
		"error":       err.Error(),
		"retry_count": retryCount,
		"max_retries": c.config.MaxRetries,
	})

	if retryCount >= c.config.MaxRetries {
		// Move to dead letter queue
		c.moveToDeadLetter(ctx, message, event, err)
	} else {
		// Requeue for retry by not acknowledging
		c.metrics.mu.Lock()
		c.metrics.EventsRetried++
		c.metrics.mu.Unlock()
	}
}

// acknowledgeMessage acknowledges a processed message
func (c *WebhookConsumer) acknowledgeMessage(ctx context.Context, messageID string) {
	client := c.redisClient.GetClient()
	if err := client.XAck(ctx, c.config.StreamKey, c.config.ConsumerGroup, messageID).Err(); err != nil {
		c.logger.Error("Failed to acknowledge message", map[string]interface{}{
			"message_id": messageID,
			"error":      err.Error(),
		})
	}
}

// moveToDeadLetter moves a message to the dead letter queue
func (c *WebhookConsumer) moveToDeadLetter(ctx context.Context, message redisclient.XMessage, event *WebhookEvent, processingError error) {
	client := c.redisClient.GetClient()

	// Add additional metadata
	dlqEntry := map[string]interface{}{
		"event":          message.Values["event"],
		"original_id":    message.ID,
		"failed_at":      time.Now().Unix(),
		"error":          processingError.Error(),
		"consumer_group": c.config.ConsumerGroup,
		"consumer_id":    c.config.ConsumerID,
		"retry_count":    c.getRetryCount(message),
	}

	// Add to dead letter stream
	if _, err := client.XAdd(ctx, &redisclient.XAddArgs{
		Stream: c.config.DeadLetterStream,
		Values: dlqEntry,
	}).Result(); err != nil {
		c.logger.Error("Failed to add to dead letter queue", map[string]interface{}{
			"message_id": message.ID,
			"error":      err.Error(),
		})
		return
	}

	// Acknowledge the original message
	c.acknowledgeMessage(ctx, message.ID)

	// Update metrics
	c.metrics.mu.Lock()
	c.metrics.EventsDeadLettered++
	c.metrics.mu.Unlock()

	c.logger.Info("Message moved to dead letter queue", map[string]interface{}{
		"message_id": message.ID,
		"event_id":   event.EventId,
	})
}

// claimPendingMessages claims and processes pending messages
func (c *WebhookConsumer) claimPendingMessages() {
	defer c.workers.Done()

	ticker := time.NewTicker(c.config.ClaimMinIdleTime)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.processPendingMessages()
		}
	}
}

// processPendingMessages processes pending messages from other consumers
func (c *WebhookConsumer) processPendingMessages() {
	ctx := context.Background()
	client := c.redisClient.GetClient()

	// Get pending messages
	pending, err := client.XPendingExt(ctx, &redisclient.XPendingExtArgs{
		Stream: c.config.StreamKey,
		Group:  c.config.ConsumerGroup,
		Start:  "-",
		End:    "+",
		Count:  c.config.BatchSize,
	}).Result()

	if err != nil {
		c.logger.Error("Failed to get pending messages", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Claim old messages
	for _, msg := range pending {
		if msg.Idle >= c.config.ClaimMinIdleTime {
			c.claimAndProcessMessage(ctx, msg.ID)
		}
	}
}

// claimAndProcessMessage claims and processes a single pending message
func (c *WebhookConsumer) claimAndProcessMessage(ctx context.Context, messageID string) {
	client := c.redisClient.GetClient()

	// Claim the message
	messages, err := client.XClaim(ctx, &redisclient.XClaimArgs{
		Stream:   c.config.StreamKey,
		Group:    c.config.ConsumerGroup,
		Consumer: c.config.ConsumerID,
		MinIdle:  c.config.ClaimMinIdleTime,
		Messages: []string{messageID},
	}).Result()

	if err != nil {
		c.logger.Error("Failed to claim message", map[string]interface{}{
			"message_id": messageID,
			"error":      err.Error(),
		})
		return
	}

	// Process claimed messages
	for _, message := range messages {
		c.processMessage(-1, message) // -1 indicates claimed message
	}
}

// getRetryCount gets the retry count from message metadata
func (c *WebhookConsumer) getRetryCount(message redisclient.XMessage) int {
	if val, ok := message.Values["retry_count"]; ok {
		if count, ok := val.(int); ok {
			return count
		}
	}
	return 0
}

// getToolBreaker gets or creates a circuit breaker for a tool
func (c *WebhookConsumer) getToolBreaker(toolID string) *redis.CircuitBreaker {
	c.breakerMu.RLock()
	breaker, exists := c.toolBreakers[toolID]
	c.breakerMu.RUnlock()

	if exists {
		return breaker
	}

	// Create new breaker
	c.breakerMu.Lock()
	defer c.breakerMu.Unlock()

	// Double-check after acquiring write lock
	if breaker, exists := c.toolBreakers[toolID]; exists {
		return breaker
	}

	config := &redis.CircuitBreakerConfig{
		FailureThreshold:  5,
		SuccessThreshold:  2,
		Timeout:           30 * time.Second,
		MaxTimeout:        5 * time.Minute,
		TimeoutMultiplier: 2.0,
	}

	breaker = redis.NewCircuitBreaker(config, c.logger)
	c.toolBreakers[toolID] = breaker

	return breaker
}

// updateMetrics updates processing metrics
func (c *WebhookConsumer) updateMetrics(eventType string, duration time.Duration) {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()

	c.metrics.EventsProcessed++
	c.metrics.LastProcessedTime = time.Now()

	// Update average processing time
	if current, exists := c.metrics.ProcessingTime[eventType]; exists {
		c.metrics.ProcessingTime[eventType] = (current + duration) / 2
	} else {
		c.metrics.ProcessingTime[eventType] = duration
	}
}

// reportMetrics periodically reports consumer metrics
func (c *WebhookConsumer) reportMetrics() {
	defer c.workers.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.logMetrics()
		}
	}
}

// logMetrics logs current metrics
func (c *WebhookConsumer) logMetrics() {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()

	c.logger.Info("Consumer metrics", map[string]interface{}{
		"events_processed":     c.metrics.EventsProcessed,
		"events_failed":        c.metrics.EventsFailed,
		"events_retried":       c.metrics.EventsRetried,
		"events_dead_lettered": c.metrics.EventsDeadLettered,
		"last_processed":       c.metrics.LastProcessedTime,
		"consumer_id":          c.config.ConsumerID,
	})
}

// GetMetrics returns current consumer metrics
func (c *WebhookConsumer) GetMetrics() map[string]interface{} {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()

	return map[string]interface{}{
		"events_processed":     c.metrics.EventsProcessed,
		"events_failed":        c.metrics.EventsFailed,
		"events_retried":       c.metrics.EventsRetried,
		"events_dead_lettered": c.metrics.EventsDeadLettered,
		"processing_time":      c.metrics.ProcessingTime,
		"last_processed":       c.metrics.LastProcessedTime,
		"num_workers":          c.config.NumWorkers,
		"consumer_id":          c.config.ConsumerID,
		"consumer_group":       c.config.ConsumerGroup,
	}
}
