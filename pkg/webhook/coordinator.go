package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/redis"
)

// CoordinatorConfig contains configuration for the consumer coordinator
type CoordinatorConfig struct {
	// Coordination settings
	HeartbeatInterval time.Duration
	ConsumerTimeout   time.Duration
	RebalanceInterval time.Duration

	// Leader election
	LeaderKey string
	LeaderTTL time.Duration

	// Stream distribution
	MinConsumersPerStream int
	MaxConsumersPerStream int

	// Health check settings
	HealthCheckInterval time.Duration
	UnhealthyThreshold  int
}

// DefaultCoordinatorConfig returns default coordinator configuration
func DefaultCoordinatorConfig() *CoordinatorConfig {
	return &CoordinatorConfig{
		HeartbeatInterval:     10 * time.Second,
		ConsumerTimeout:       30 * time.Second,
		RebalanceInterval:     60 * time.Second,
		LeaderKey:             "webhook:coordinator:leader",
		LeaderTTL:             15 * time.Second,
		MinConsumersPerStream: 2,
		MaxConsumersPerStream: 10,
		HealthCheckInterval:   30 * time.Second,
		UnhealthyThreshold:    3,
	}
}

// ConsumerInfo represents information about a consumer
type ConsumerInfo struct {
	ID              string          `json:"id"`
	GroupID         string          `json:"group_id"`
	Hostname        string          `json:"hostname"`
	StartTime       time.Time       `json:"start_time"`
	LastHeartbeat   time.Time       `json:"last_heartbeat"`
	State           ConsumerState   `json:"state"`
	AssignedStreams []string        `json:"assigned_streams"`
	ProcessingStats ProcessingStats `json:"processing_stats"`
	HealthStatus    HealthStatus    `json:"health_status"`
}

// ConsumerState represents the state of a consumer
type ConsumerState string

const (
	ConsumerStateActive   ConsumerState = "active"
	ConsumerStateIdle     ConsumerState = "idle"
	ConsumerStateStopping ConsumerState = "stopping"
	ConsumerStateDead     ConsumerState = "dead"
)

// ProcessingStats contains consumer processing statistics
type ProcessingStats struct {
	EventsProcessed   int64         `json:"events_processed"`
	EventsFailed      int64         `json:"events_failed"`
	AverageLatency    time.Duration `json:"average_latency"`
	LastProcessedTime time.Time     `json:"last_processed_time"`
}

// HealthStatus represents consumer health
type HealthStatus struct {
	Healthy             bool      `json:"healthy"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	LastError           string    `json:"last_error,omitempty"`
	LastCheckTime       time.Time `json:"last_check_time"`
}

// StreamAssignment represents stream assignment to consumers
type StreamAssignment struct {
	StreamKey    string    `json:"stream_key"`
	ConsumerIDs  []string  `json:"consumer_ids"`
	LastModified time.Time `json:"last_modified"`
}

// ConsumerCoordinator coordinates multiple webhook consumers
type ConsumerCoordinator struct {
	config      *CoordinatorConfig
	redisClient *redis.StreamsClient
	logger      observability.Logger

	// Local consumer info
	localConsumer *ConsumerInfo

	// Coordination state
	isLeader    bool
	leaderID    string
	consumers   map[string]*ConsumerInfo
	assignments map[string]*StreamAssignment
	mu          sync.RWMutex

	// Background tasks
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewConsumerCoordinator creates a new consumer coordinator
func NewConsumerCoordinator(
	config *CoordinatorConfig,
	redisClient *redis.StreamsClient,
	consumerInfo *ConsumerInfo,
	logger observability.Logger,
) *ConsumerCoordinator {
	if config == nil {
		config = DefaultCoordinatorConfig()
	}

	return &ConsumerCoordinator{
		config:        config,
		redisClient:   redisClient,
		localConsumer: consumerInfo,
		logger:        logger,
		consumers:     make(map[string]*ConsumerInfo),
		assignments:   make(map[string]*StreamAssignment),
		stopCh:        make(chan struct{}),
	}
}

// Start starts the coordinator
func (c *ConsumerCoordinator) Start() error {
	c.logger.Info("Starting consumer coordinator", map[string]interface{}{
		"consumer_id": c.localConsumer.ID,
		"group_id":    c.localConsumer.GroupID,
	})

	// Register this consumer
	if err := c.registerConsumer(); err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	// Start background tasks
	c.wg.Add(1)
	go c.heartbeatLoop()

	c.wg.Add(1)
	go c.leaderElectionLoop()

	c.wg.Add(1)
	go c.healthCheckLoop()

	return nil
}

// Stop stops the coordinator
func (c *ConsumerCoordinator) Stop() {
	c.logger.Info("Stopping consumer coordinator", nil)

	// Deregister consumer
	c.deregisterConsumer()

	close(c.stopCh)
	c.wg.Wait()
}

// registerConsumer registers this consumer in Redis
func (c *ConsumerCoordinator) registerConsumer() error {
	ctx := context.Background()
	client := c.redisClient.GetClient()

	// Update consumer info
	c.localConsumer.State = ConsumerStateActive
	c.localConsumer.LastHeartbeat = time.Now()

	// Serialize consumer info
	data, err := json.Marshal(c.localConsumer)
	if err != nil {
		return fmt.Errorf("failed to marshal consumer info: %w", err)
	}

	// Store in Redis with TTL
	key := fmt.Sprintf("webhook:consumers:%s", c.localConsumer.ID)
	if err := client.Set(ctx, key, data, c.config.ConsumerTimeout).Err(); err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	// Add to consumer set
	setKey := fmt.Sprintf("webhook:consumer_groups:%s", c.localConsumer.GroupID)
	if err := client.SAdd(ctx, setKey, c.localConsumer.ID).Err(); err != nil {
		return fmt.Errorf("failed to add to consumer set: %w", err)
	}

	c.logger.Info("Consumer registered", map[string]interface{}{
		"consumer_id": c.localConsumer.ID,
	})

	return nil
}

// deregisterConsumer removes this consumer from Redis
func (c *ConsumerCoordinator) deregisterConsumer() {
	ctx := context.Background()
	client := c.redisClient.GetClient()

	// Update state
	c.localConsumer.State = ConsumerStateDead

	// Remove from Redis
	key := fmt.Sprintf("webhook:consumers:%s", c.localConsumer.ID)
	client.Del(ctx, key)

	// Remove from consumer set
	setKey := fmt.Sprintf("webhook:consumer_groups:%s", c.localConsumer.GroupID)
	client.SRem(ctx, setKey, c.localConsumer.ID)

	c.logger.Info("Consumer deregistered", map[string]interface{}{
		"consumer_id": c.localConsumer.ID,
	})
}

// heartbeatLoop sends periodic heartbeats
func (c *ConsumerCoordinator) heartbeatLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.sendHeartbeat()
		}
	}
}

// sendHeartbeat sends a heartbeat for this consumer
func (c *ConsumerCoordinator) sendHeartbeat() {
	ctx := context.Background()
	client := c.redisClient.GetClient()

	// Update heartbeat time
	c.localConsumer.LastHeartbeat = time.Now()

	// Serialize consumer info
	data, err := json.Marshal(c.localConsumer)
	if err != nil {
		c.logger.Error("Failed to marshal consumer info", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Update in Redis
	key := fmt.Sprintf("webhook:consumers:%s", c.localConsumer.ID)
	if err := client.Set(ctx, key, data, c.config.ConsumerTimeout).Err(); err != nil {
		c.logger.Error("Failed to send heartbeat", map[string]interface{}{
			"error": err.Error(),
		})
	}
}

// leaderElectionLoop handles leader election
func (c *ConsumerCoordinator) leaderElectionLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.LeaderTTL / 2)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.tryBecomeLeader()
			if c.isLeader {
				c.performLeaderDuties()
			}
		}
	}
}

// tryBecomeLeader attempts to become the leader
func (c *ConsumerCoordinator) tryBecomeLeader() {
	ctx := context.Background()
	client := c.redisClient.GetClient()

	// Try to acquire leader lock
	result := client.SetNX(ctx, c.config.LeaderKey, c.localConsumer.ID, c.config.LeaderTTL)
	if result.Val() {
		c.mu.Lock()
		c.isLeader = true
		c.leaderID = c.localConsumer.ID
		c.mu.Unlock()

		c.logger.Info("Became leader", map[string]interface{}{
			"consumer_id": c.localConsumer.ID,
		})
		return
	}

	// Check if we're already the leader
	currentLeader, err := client.Get(ctx, c.config.LeaderKey).Result()
	if err == nil && currentLeader == c.localConsumer.ID {
		// Extend leader TTL
		client.Expire(ctx, c.config.LeaderKey, c.config.LeaderTTL)

		c.mu.Lock()
		c.isLeader = true
		c.leaderID = c.localConsumer.ID
		c.mu.Unlock()
	} else {
		c.mu.Lock()
		c.isLeader = false
		c.leaderID = currentLeader
		c.mu.Unlock()
	}
}

// performLeaderDuties performs leader-specific tasks
func (c *ConsumerCoordinator) performLeaderDuties() {
	// Discover all consumers
	c.discoverConsumers()

	// Check for dead consumers
	c.cleanupDeadConsumers()

	// Rebalance if needed
	if c.shouldRebalance() {
		c.rebalanceStreams()
	}
}

// discoverConsumers discovers all active consumers
func (c *ConsumerCoordinator) discoverConsumers() {
	ctx := context.Background()
	client := c.redisClient.GetClient()

	// Get all consumer IDs from the group
	setKey := fmt.Sprintf("webhook:consumer_groups:%s", c.localConsumer.GroupID)
	consumerIDs, err := client.SMembers(ctx, setKey).Result()
	if err != nil {
		c.logger.Error("Failed to get consumer list", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Get info for each consumer
	c.mu.Lock()
	defer c.mu.Unlock()

	activeConsumers := make(map[string]*ConsumerInfo)

	for _, id := range consumerIDs {
		key := fmt.Sprintf("webhook:consumers:%s", id)
		data, err := client.Get(ctx, key).Bytes()
		if err != nil {
			continue // Consumer might be dead
		}

		var info ConsumerInfo
		if err := json.Unmarshal(data, &info); err != nil {
			c.logger.Error("Failed to unmarshal consumer info", map[string]interface{}{
				"consumer_id": id,
				"error":       err.Error(),
			})
			continue
		}

		// Check if consumer is still alive
		if time.Since(info.LastHeartbeat) <= c.config.ConsumerTimeout {
			activeConsumers[id] = &info
		}
	}

	c.consumers = activeConsumers
}

// cleanupDeadConsumers removes dead consumers
func (c *ConsumerCoordinator) cleanupDeadConsumers() {
	ctx := context.Background()
	client := c.redisClient.GetClient()

	c.mu.RLock()
	consumers := make(map[string]*ConsumerInfo)
	for k, v := range c.consumers {
		consumers[k] = v
	}
	c.mu.RUnlock()

	for id, info := range consumers {
		if time.Since(info.LastHeartbeat) > c.config.ConsumerTimeout {
			c.logger.Info("Removing dead consumer", map[string]interface{}{
				"consumer_id":    id,
				"last_heartbeat": info.LastHeartbeat,
			})

			// Remove from Redis
			key := fmt.Sprintf("webhook:consumers:%s", id)
			client.Del(ctx, key)

			// Remove from consumer set
			setKey := fmt.Sprintf("webhook:consumer_groups:%s", info.GroupID)
			client.SRem(ctx, setKey, id)

			// Remove from local map
			c.mu.Lock()
			delete(c.consumers, id)
			c.mu.Unlock()
		}
	}
}

// shouldRebalance determines if streams should be rebalanced
func (c *ConsumerCoordinator) shouldRebalance() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check if we have the minimum number of consumers
	if len(c.consumers) < c.config.MinConsumersPerStream {
		return false
	}

	// Check for uneven distribution
	streamCounts := make(map[string]int)
	for _, consumer := range c.consumers {
		for _, stream := range consumer.AssignedStreams {
			streamCounts[stream]++
		}
	}

	// Check if any stream has too many or too few consumers
	for _, count := range streamCounts {
		if count < c.config.MinConsumersPerStream || count > c.config.MaxConsumersPerStream {
			return true
		}
	}

	return false
}

// rebalanceStreams redistributes streams among consumers
func (c *ConsumerCoordinator) rebalanceStreams() {
	c.logger.Info("Rebalancing streams", map[string]interface{}{
		"num_consumers": len(c.consumers),
	})

	ctx := context.Background()
	client := c.redisClient.GetClient()

	// Get all available streams
	streams := c.getAvailableStreams()

	// Calculate assignments
	assignments := c.calculateAssignments(streams)

	// Store assignments
	for stream, assignment := range assignments {
		data, err := json.Marshal(assignment)
		if err != nil {
			continue
		}

		key := fmt.Sprintf("webhook:stream_assignments:%s", stream)
		client.Set(ctx, key, data, 0)
	}

	// Notify consumers of new assignments
	c.notifyAssignmentChanges(assignments)

	c.mu.Lock()
	c.assignments = assignments
	c.mu.Unlock()
}

// getAvailableStreams gets all available webhook streams
func (c *ConsumerCoordinator) getAvailableStreams() []string {
	// In a real implementation, this would discover streams dynamically
	// For now, return a static list
	return []string{
		"webhook:events:github",
		"webhook:events:gitlab",
		"webhook:events:jira",
		"webhook:events:jenkins",
	}
}

// calculateAssignments calculates optimal stream assignments
func (c *ConsumerCoordinator) calculateAssignments(streams []string) map[string]*StreamAssignment {
	c.mu.RLock()
	numConsumers := len(c.consumers)
	consumerList := make([]string, 0, numConsumers)
	for id := range c.consumers {
		consumerList = append(consumerList, id)
	}
	c.mu.RUnlock()

	assignments := make(map[string]*StreamAssignment)

	// Simple round-robin assignment
	for i, stream := range streams {
		assignment := &StreamAssignment{
			StreamKey:    stream,
			ConsumerIDs:  make([]string, 0, c.config.MinConsumersPerStream),
			LastModified: time.Now(),
		}

		// Assign minimum consumers per stream
		for j := 0; j < c.config.MinConsumersPerStream && j < numConsumers; j++ {
			consumerIdx := (i + j) % numConsumers
			assignment.ConsumerIDs = append(assignment.ConsumerIDs, consumerList[consumerIdx])
		}

		assignments[stream] = assignment
	}

	return assignments
}

// notifyAssignmentChanges notifies consumers of assignment changes
func (c *ConsumerCoordinator) notifyAssignmentChanges(assignments map[string]*StreamAssignment) {
	ctx := context.Background()
	client := c.redisClient.GetClient()

	// Publish assignment change event
	event := map[string]interface{}{
		"type":        "assignment_change",
		"timestamp":   time.Now().Unix(),
		"assignments": assignments,
	}

	data, _ := json.Marshal(event)
	client.Publish(ctx, "webhook:coordinator:events", data)
}

// healthCheckLoop performs health checks on consumers
func (c *ConsumerCoordinator) healthCheckLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.performHealthChecks()
		}
	}
}

// performHealthChecks checks the health of all consumers
func (c *ConsumerCoordinator) performHealthChecks() {
	c.mu.RLock()
	consumers := make([]*ConsumerInfo, 0, len(c.consumers))
	for _, consumer := range c.consumers {
		consumers = append(consumers, consumer)
	}
	c.mu.RUnlock()

	for _, consumer := range consumers {
		c.checkConsumerHealth(consumer)
	}
}

// checkConsumerHealth checks the health of a single consumer
func (c *ConsumerCoordinator) checkConsumerHealth(consumer *ConsumerInfo) {
	// Check if consumer is processing events
	if time.Since(consumer.ProcessingStats.LastProcessedTime) > 5*time.Minute {
		consumer.HealthStatus.ConsecutiveFailures++
		consumer.HealthStatus.LastError = "No events processed in 5 minutes"
	} else {
		consumer.HealthStatus.ConsecutiveFailures = 0
		consumer.HealthStatus.LastError = ""
	}

	// Update health status
	consumer.HealthStatus.Healthy = consumer.HealthStatus.ConsecutiveFailures < c.config.UnhealthyThreshold
	consumer.HealthStatus.LastCheckTime = time.Now()

	// Log unhealthy consumers
	if !consumer.HealthStatus.Healthy {
		c.logger.Warn("Consumer unhealthy", map[string]interface{}{
			"consumer_id": consumer.ID,
			"failures":    consumer.HealthStatus.ConsecutiveFailures,
			"error":       consumer.HealthStatus.LastError,
		})
	}
}

// GetAssignments returns current stream assignments
func (c *ConsumerCoordinator) GetAssignments() map[string]*StreamAssignment {
	c.mu.RLock()
	defer c.mu.RUnlock()

	assignments := make(map[string]*StreamAssignment)
	for k, v := range c.assignments {
		assignments[k] = v
	}

	return assignments
}

// GetConsumers returns information about all consumers
func (c *ConsumerCoordinator) GetConsumers() map[string]*ConsumerInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	consumers := make(map[string]*ConsumerInfo)
	for k, v := range c.consumers {
		consumers[k] = v
	}

	return consumers
}

// IsLeader returns whether this coordinator is the leader
func (c *ConsumerCoordinator) IsLeader() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isLeader
}

// GetLeaderID returns the current leader ID
func (c *ConsumerCoordinator) GetLeaderID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.leaderID
}
