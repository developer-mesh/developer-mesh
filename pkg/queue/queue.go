package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/redis"
)

// Event represents a webhook event in the queue
type Event struct {
	EventID     string                 `json:"event_id"`
	EventType   string                 `json:"event_type"`
	RepoName    string                 `json:"repo_name"`
	SenderName  string                 `json:"sender_name"`
	Payload     json.RawMessage        `json:"payload"`
	AuthContext *EventAuthContext      `json:"auth_context,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// EventAuthContext contains authentication context for queue events
type EventAuthContext struct {
	TenantID       string                 `json:"tenant_id"`
	PrincipalID    string                 `json:"principal_id"`
	PrincipalType  string                 `json:"principal_type"`
	InstallationID *int64                 `json:"installation_id,omitempty"`
	AppID          *int64                 `json:"app_id,omitempty"`
	Permissions    []string               `json:"permissions,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// SQSEvent type for backward compatibility
type SQSEvent struct {
	DeliveryID  string            `json:"delivery_id"`
	EventType   string            `json:"event_type"`
	RepoName    string            `json:"repo_name"`
	SenderName  string            `json:"sender_name"`
	Payload     json.RawMessage   `json:"payload"`
	AuthContext *EventAuthContext `json:"auth_context,omitempty"`
}

// Client represents the Redis-based queue client
type Client struct {
	streamsClient *redis.StreamsClient
	streamName    string
	consumerGroup string
	logger        observability.Logger
}

// Config holds configuration for the queue client
type Config struct {
	Logger observability.Logger
}

// NewClient creates a new Redis-based queue client
func NewClient(_ context.Context, config *Config) (*Client, error) {
	if config == nil {
		config = &Config{}
	}

	logger := config.Logger
	if logger == nil {
		logger = observability.NewNoopLogger()
	}

	// Get Redis configuration from environment
	addresses := []string{"localhost:6379"}
	if redisAddr := os.Getenv("REDIS_ADDRESS"); redisAddr != "" {
		addresses = []string{redisAddr}
	}

	password := os.Getenv("REDIS_PASSWORD")

	streamName := "webhook-events"
	if s := os.Getenv("REDIS_STREAM_NAME"); s != "" {
		streamName = s
	}

	consumerGroup := "webhook-processors"
	if g := os.Getenv("REDIS_CONSUMER_GROUP"); g != "" {
		consumerGroup = g
	}

	// Create Redis Streams client
	streamsClient, err := redis.NewStreamsClient(&redis.StreamsConfig{
		Addresses:  addresses,
		Password:   password,
		PoolSize:   10,
		MaxRetries: 3,
	}, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis Streams client: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create consumer group if it doesn't exist
	if err := streamsClient.CreateConsumerGroupMkStream(ctx, streamName, consumerGroup, "0"); err != nil {
		// Ignore error if group already exists
		logger.Info("Consumer group may already exist", map[string]interface{}{"error": err.Error()})
	}

	client := &Client{
		streamsClient: streamsClient,
		streamName:    streamName,
		consumerGroup: consumerGroup,
		logger:        logger,
	}

	return client, nil
}

// EnqueueEvent sends an event to the Redis stream
func (c *Client) EnqueueEvent(ctx context.Context, event Event) error {
	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Marshal complex fields to JSON
	authJSON := ""
	if event.AuthContext != nil {
		data, err := json.Marshal(event.AuthContext)
		if err != nil {
			return fmt.Errorf("failed to marshal auth context: %w", err)
		}
		authJSON = string(data)
	}

	metadataJSON := ""
	if event.Metadata != nil {
		data, err := json.Marshal(event.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataJSON = string(data)
	}

	payloadJSON := ""
	if event.Payload != nil {
		payloadJSON = string(event.Payload)
	}

	// Add to stream with automatic ID generation
	messageID, err := c.streamsClient.AddToStream(ctx, c.streamName, map[string]interface{}{
		"event_id":     event.EventID,
		"event_type":   event.EventType,
		"repo_name":    event.RepoName,
		"sender_name":  event.SenderName,
		"payload":      payloadJSON,
		"auth_context": authJSON,
		"timestamp":    event.Timestamp.Format(time.RFC3339),
		"metadata":     metadataJSON,
	})
	if err != nil {
		return fmt.Errorf("failed to add event to stream: %w", err)
	}

	c.logger.Info("Event enqueued", map[string]interface{}{
		"message_id": messageID,
		"event_id":   event.EventID,
		"event_type": event.EventType,
	})

	return nil
}

// ReceiveEvents receives events from the Redis stream
func (c *Client) ReceiveEvents(ctx context.Context, maxMessages int32, waitSeconds int32) ([]Event, []string, error) {
	consumerName := fmt.Sprintf("consumer-%d", time.Now().UnixNano())

	streams := []string{c.streamName, ">"}
	results, err := c.streamsClient.ReadFromConsumerGroup(
		ctx,
		c.consumerGroup,
		consumerName,
		streams,
		int64(maxMessages),
		time.Duration(waitSeconds)*time.Second,
		false,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read from stream: %w", err)
	}

	var events []Event
	var receipts []string

	for _, stream := range results {
		for _, message := range stream.Messages {
			// Parse event from Redis message
			event := Event{}

			if val, ok := message.Values["event_id"].(string); ok {
				event.EventID = val
			}
			if val, ok := message.Values["event_type"].(string); ok {
				event.EventType = val
			}
			if val, ok := message.Values["repo_name"].(string); ok {
				event.RepoName = val
			}
			if val, ok := message.Values["sender_name"].(string); ok {
				event.SenderName = val
			}

			// Parse JSON fields
			if val, ok := message.Values["payload"].(string); ok {
				event.Payload = json.RawMessage(val)
			}

			if val, ok := message.Values["auth_context"].(string); ok && val != "" {
				var authContext EventAuthContext
				if err := json.Unmarshal([]byte(val), &authContext); err == nil {
					event.AuthContext = &authContext
				}
			}

			if val, ok := message.Values["timestamp"].(string); ok {
				if t, err := time.Parse(time.RFC3339, val); err == nil {
					event.Timestamp = t
				}
			}

			if val, ok := message.Values["metadata"].(string); ok && val != "" {
				var metadata map[string]interface{}
				if err := json.Unmarshal([]byte(val), &metadata); err == nil {
					event.Metadata = metadata
				}
			}

			events = append(events, event)
			receipts = append(receipts, message.ID)
		}
	}

	return events, receipts, nil
}

// DeleteMessage acknowledges a message from the Redis stream
func (c *Client) DeleteMessage(ctx context.Context, receiptHandle string) error {
	return c.streamsClient.AckMessages(ctx, c.streamName, c.consumerGroup, receiptHandle)
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.streamsClient.Close()
}

// Health checks the health of the Redis connection
func (c *Client) Health(ctx context.Context) error {
	if !c.streamsClient.IsHealthy() {
		return errors.New("redis client is not healthy")
	}

	return nil
}

// GetQueueDepth returns the approximate number of messages in the queue
func (c *Client) GetQueueDepth(ctx context.Context) (int64, error) {
	info, err := c.streamsClient.GetStreamInfo(ctx, c.streamName)
	if err != nil {
		return 0, fmt.Errorf("failed to get stream info: %w", err)
	}
	return info.Length, nil
}
