package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/developer-mesh/developer-mesh/pkg/observability"

	// Import removed - not used directly
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStreamsClient(t *testing.T) {
	logger := observability.NewNoopLogger()

	t.Run("Creates client with default config", func(t *testing.T) {
		mr, err := miniredis.Run()
		require.NoError(t, err)
		defer mr.Close()

		config := &StreamsConfig{
			Addresses:   []string{mr.Addr()},
			PoolTimeout: 5 * time.Second,
		}

		client, err := NewStreamsClient(config, logger)
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.True(t, client.IsHealthy())
	})

	t.Run("Handles connection errors", func(t *testing.T) {
		config := &StreamsConfig{
			Addresses:   []string{"invalid:6379"},
			PoolTimeout: 1 * time.Second,
		}

		client, err := NewStreamsClient(config, logger)
		assert.Error(t, err) // Client creation fails due to connection error
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "failed to connect to Redis")
	})
}

func TestStreamsClient_AddToStream(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	logger := observability.NewNoopLogger()
	config := &StreamsConfig{
		Addresses:   []string{mr.Addr()},
		PoolTimeout: 5 * time.Second,
	}

	client, err := NewStreamsClient(config, logger)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Adds message to stream", func(t *testing.T) {
		values := map[string]interface{}{
			"field1": "value1",
			"field2": "value2",
		}

		id, err := client.AddToStream(ctx, "test-stream", values)
		assert.NoError(t, err)
		assert.NotEmpty(t, id)
	})

	t.Run("Handles unhealthy connection", func(t *testing.T) {
		// Close the client to make it unhealthy
		_ = client.Close()

		values := map[string]interface{}{
			"field1": "value1",
		}

		_, err := client.AddToStream(ctx, "test-stream", values)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "closed")
	})
}

func TestStreamsClient_ReadFromStream(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	logger := observability.NewNoopLogger()
	config := &StreamsConfig{
		Addresses:   []string{mr.Addr()},
		PoolTimeout: 5 * time.Second,
	}

	client, err := NewStreamsClient(config, logger)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Reads messages from stream", func(t *testing.T) {
		// Add a message first
		values := map[string]interface{}{
			"test": "data",
		}
		_, err := client.AddToStream(ctx, "read-test", values)
		require.NoError(t, err)

		// Read messages
		messages, err := client.ReadFromStream(ctx, []string{"read-test"}, 10, 0)
		assert.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, "read-test", messages[0].Stream)
	})
}

func TestStreamsClient_ConsumerGroup(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	logger := observability.NewNoopLogger()
	config := &StreamsConfig{
		Addresses:   []string{mr.Addr()},
		PoolTimeout: 5 * time.Second,
	}

	client, err := NewStreamsClient(config, logger)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Creates consumer group", func(t *testing.T) {
		err := client.CreateConsumerGroupMkStream(ctx, "group-test", "test-group", "$")
		assert.NoError(t, err)
	})

	t.Run("Reads from consumer group", func(t *testing.T) {
		// Add a message
		values := map[string]interface{}{
			"data": "test",
		}
		_, err := client.AddToStream(ctx, "group-test", values)
		require.NoError(t, err)

		// Read from consumer group
		messages, err := client.ReadFromConsumerGroup(ctx, "test-group", "consumer-1", []string{"group-test"}, 10, 0, false)
		assert.NoError(t, err)
		assert.NotEmpty(t, messages)
	})
}

func TestStreamsClient_HealthCheck(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	logger := observability.NewNoopLogger()
	config := &StreamsConfig{
		Addresses:   []string{mr.Addr()},
		PoolTimeout: 5 * time.Second,
	}

	client, err := NewStreamsClient(config, logger)
	require.NoError(t, err)

	t.Run("Health check maintains healthy status", func(t *testing.T) {
		// Wait for health checks to run
		time.Sleep(300 * time.Millisecond)
		assert.True(t, client.IsHealthy())
	})

	t.Run("Health check detects unhealthy connection", func(t *testing.T) {
		// Close miniredis to simulate failure
		mr.Close()

		// Wait for health check to detect failure
		time.Sleep(300 * time.Millisecond)
		assert.False(t, client.IsHealthy())
	})
}

func TestStreamsClient_ReconnectMechanism(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	logger := observability.NewNoopLogger()
	config := &StreamsConfig{
		Addresses:   []string{mr.Addr()},
		PoolTimeout: 5 * time.Second,
		MaxRetries:  3,
	}

	client, err := NewStreamsClient(config, logger)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Reconnects after connection loss", func(t *testing.T) {
		// Initial connection should work
		assert.True(t, client.IsHealthy())

		// Save the address before closing
		addr := mr.Addr()

		// Close miniredis to simulate connection loss
		mr.Close()
		time.Sleep(100 * time.Millisecond)
		assert.False(t, client.IsHealthy())

		// Restart miniredis on same address
		mr2 := miniredis.NewMiniRedis()
		err := mr2.StartAddr(addr)
		require.NoError(t, err)
		defer mr2.Close()

		// Wait for reconnection
		time.Sleep(200 * time.Millisecond)

		// Should be healthy again
		assert.True(t, client.IsHealthy())

		// Operations should work
		values := map[string]interface{}{
			"test": "reconnect",
		}
		_, err = client.AddToStream(ctx, "reconnect-test", values)
		assert.NoError(t, err)
	})
}

func TestStreamsClient_StreamOperations(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	logger := observability.NewNoopLogger()
	config := &StreamsConfig{
		Addresses:   []string{mr.Addr()},
		PoolTimeout: 5 * time.Second,
	}

	client, err := NewStreamsClient(config, logger)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Acknowledges messages", func(t *testing.T) {
		// Create stream and consumer group
		err := client.CreateConsumerGroupMkStream(ctx, "ack-test", "ack-group", "$")
		require.NoError(t, err)

		// Add message
		values := map[string]interface{}{"data": "test"}
		msgID, err := client.AddToStream(ctx, "ack-test", values)
		require.NoError(t, err)

		// Read with consumer group
		messages, err := client.ReadFromConsumerGroup(ctx, "ack-group", "consumer-1", []string{"ack-test"}, 10, 0, false)
		require.NoError(t, err)
		require.NotEmpty(t, messages)

		// Acknowledge message
		err = client.AckMessages(ctx, "ack-test", "ack-group", msgID)
		assert.NoError(t, err)
	})

	t.Run("Claims pending messages", func(t *testing.T) {
		// Create stream and consumer group
		err := client.CreateConsumerGroupMkStream(ctx, "claim-test", "claim-group", "$")
		require.NoError(t, err)

		// Add message
		values := map[string]interface{}{"data": "test"}
		msgID, err := client.AddToStream(ctx, "claim-test", values)
		require.NoError(t, err)

		// Read with first consumer (don't ack)
		messages, err := client.ReadFromConsumerGroup(ctx, "claim-group", "consumer-1", []string{"claim-test"}, 10, 0, false)
		require.NoError(t, err)
		require.NotEmpty(t, messages)

		// Try to claim with second consumer
		claimed, err := client.ClaimMessages(ctx, "claim-test", "claim-group", "consumer-2", 0, msgID)
		assert.NoError(t, err)
		assert.NotEmpty(t, claimed)
	})

	t.Run("Gets pending messages info", func(t *testing.T) {
		// Create stream and consumer group
		err := client.CreateConsumerGroupMkStream(ctx, "pending-test", "pending-group", "$")
		require.NoError(t, err)

		// Add message
		values := map[string]interface{}{"data": "test"}
		_, err = client.AddToStream(ctx, "pending-test", values)
		require.NoError(t, err)

		// Read without acknowledging
		messages, err := client.ReadFromConsumerGroup(ctx, "pending-group", "consumer-1", []string{"pending-test"}, 10, 0, false)
		require.NoError(t, err)
		require.NotEmpty(t, messages)

		// Get pending messages
		redisClient := client.GetClient()
		pending, err := redisClient.XPending(ctx, "pending-test", "pending-group").Result()
		assert.NoError(t, err)
		assert.NotNil(t, pending)
		assert.Equal(t, int64(1), pending.Count)
	})
}

func TestStreamsClient_GetClient(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	logger := observability.NewNoopLogger()
	config := &StreamsConfig{
		Addresses:   []string{mr.Addr()},
		PoolTimeout: 5 * time.Second,
	}

	client, err := NewStreamsClient(config, logger)
	require.NoError(t, err)

	t.Run("Returns underlying Redis client", func(t *testing.T) {
		redisClient := client.GetClient()
		assert.NotNil(t, redisClient)

		// Test that the client works
		ctx := context.Background()
		err := redisClient.Ping(ctx).Err()
		assert.NoError(t, err)
	})
}

func TestStreamsClient_ErrorHandling(t *testing.T) {
	logger := observability.NewNoopLogger()

	t.Run("Handles nil config", func(t *testing.T) {
		client, err := NewStreamsClient(nil, logger)
		assert.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("Handles empty addresses", func(t *testing.T) {
		config := &StreamsConfig{
			Addresses: []string{},
		}
		client, err := NewStreamsClient(config, logger)
		assert.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("Handles invalid mode", func(t *testing.T) {
		// Test with invalid sentinel config
		config := &StreamsConfig{
			Addresses:       []string{"localhost:6379"},
			SentinelEnabled: true,
			MasterName:      "", // Invalid: empty master name
		}
		client, err := NewStreamsClient(config, logger)
		assert.Error(t, err)
		assert.Nil(t, client)
	})
}
