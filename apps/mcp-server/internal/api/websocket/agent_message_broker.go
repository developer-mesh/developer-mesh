package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/redis"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	redisclient "github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// AgentMessageBroker handles message routing between universal agents
type AgentMessageBroker struct {
	// Redis Streams client
	streamsClient *redis.StreamsClient

	// Repositories
	manifestRepo repository.AgentManifestRepository

	// Components
	logger  observability.Logger
	metrics observability.MetricsClient

	// Configuration
	config *BrokerConfig

	// Worker management
	workers  sync.WaitGroup
	stopCh   chan struct{}
	stopOnce sync.Once

	// Routing tables
	// capabilityRoutes sync.Map // capability -> []agent_id // TODO: Implement capability-based routing
	agentStreams sync.Map // agent_id -> stream_key
	routingRules sync.Map // rule_id -> RoutingRule

	// Consumer groups
	consumerGroups map[string]string // stream -> consumer_group
	consumerID     string

	// Metrics tracking
	messageMetrics *MessageMetrics
	metricsMu      sync.RWMutex
}

// BrokerConfig contains configuration for the message broker
type BrokerConfig struct {
	// Stream configuration
	BaseStreamKey    string
	ConsumerGroup    string
	DeadLetterStream string

	// Processing configuration
	BatchSize        int64
	BlockTimeout     time.Duration
	ProcessTimeout   time.Duration
	MaxRetries       int
	ClaimMinIdleTime time.Duration

	// Worker configuration
	NumWorkers        int
	RoutingWorkers    int
	CapabilityWorkers int

	// Stream management
	StreamTTL       time.Duration
	MaxStreamLength int64
	TrimApproximate bool
}

// DefaultBrokerConfig returns default configuration
func DefaultBrokerConfig() *BrokerConfig {
	return &BrokerConfig{
		BaseStreamKey:     "agent:messages",
		ConsumerGroup:     "agent-message-brokers",
		DeadLetterStream:  "agent:messages:dlq",
		BatchSize:         10,
		BlockTimeout:      5 * time.Second,
		ProcessTimeout:    30 * time.Second,
		MaxRetries:        3,
		ClaimMinIdleTime:  30 * time.Second,
		NumWorkers:        5,
		RoutingWorkers:    3,
		CapabilityWorkers: 2,
		StreamTTL:         24 * time.Hour,
		MaxStreamLength:   100000,
		TrimApproximate:   true,
	}
}

// AgentMessage represents a message between agents
type AgentMessage struct {
	ID               string                 `json:"id"`
	SourceAgentID    string                 `json:"source_agent_id"`
	SourceAgentType  string                 `json:"source_agent_type"`
	TargetAgentID    string                 `json:"target_agent_id,omitempty"`
	TargetAgentType  string                 `json:"target_agent_type,omitempty"`
	TargetCapability string                 `json:"target_capability,omitempty"`
	MessageType      string                 `json:"message_type"`
	Payload          map[string]interface{} `json:"payload"`
	Context          map[string]interface{} `json:"context,omitempty"`
	CorrelationID    string                 `json:"correlation_id,omitempty"`
	ReplyTo          string                 `json:"reply_to,omitempty"`
	Priority         int                    `json:"priority"`
	TTL              int64                  `json:"ttl,omitempty"`
	Timestamp        time.Time              `json:"timestamp"`
	RetryCount       int                    `json:"retry_count"`
}

// RoutingRule defines how messages are routed
type RoutingRule struct {
	ID         string
	Name       string
	SourceType string // agent type or "*"
	TargetType string // agent type or "*"
	Capability string // required capability
	Priority   int
	Conditions map[string]interface{} // additional routing conditions
	Active     bool
	CreatedAt  time.Time
}

// MessageMetrics tracks broker metrics
type MessageMetrics struct {
	MessagesRouted     int64
	MessagesFailed     int64
	MessagesDeadLetter int64
	RoutingLatency     map[string]time.Duration
	AgentDeliveries    map[string]int64
	CapabilityHits     map[string]int64
	LastProcessedTime  time.Time
}

// NewAgentMessageBroker creates a new message broker
func NewAgentMessageBroker(
	streamsClient *redis.StreamsClient,
	manifestRepo repository.AgentManifestRepository,
	logger observability.Logger,
	metrics observability.MetricsClient,
	config *BrokerConfig,
) (*AgentMessageBroker, error) {
	if config == nil {
		config = DefaultBrokerConfig()
	}

	// Generate unique consumer ID
	consumerID := fmt.Sprintf("broker-%s-%d", uuid.New().String()[:8], time.Now().Unix())

	broker := &AgentMessageBroker{
		streamsClient:  streamsClient,
		manifestRepo:   manifestRepo,
		logger:         logger,
		metrics:        metrics,
		config:         config,
		consumerID:     consumerID,
		stopCh:         make(chan struct{}),
		consumerGroups: make(map[string]string),
		messageMetrics: &MessageMetrics{
			RoutingLatency:  make(map[string]time.Duration),
			AgentDeliveries: make(map[string]int64),
			CapabilityHits:  make(map[string]int64),
		},
	}

	// Initialize consumer groups
	if err := broker.initializeStreams(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize streams: %w", err)
	}

	// Load routing rules
	broker.loadDefaultRoutingRules()

	return broker, nil
}

// Start starts the message broker workers
func (b *AgentMessageBroker) Start() error {
	b.logger.Info("Starting agent message broker", map[string]interface{}{
		"num_workers":        b.config.NumWorkers,
		"routing_workers":    b.config.RoutingWorkers,
		"capability_workers": b.config.CapabilityWorkers,
		"consumer_id":        b.consumerID,
	})

	// Start main message workers
	for i := 0; i < b.config.NumWorkers; i++ {
		b.workers.Add(1)
		go b.messageWorker(i)
	}

	// Start routing workers
	for i := 0; i < b.config.RoutingWorkers; i++ {
		b.workers.Add(1)
		go b.routingWorker(i)
	}

	// Start capability-based routing workers
	for i := 0; i < b.config.CapabilityWorkers; i++ {
		b.workers.Add(1)
		go b.capabilityWorker(i)
	}

	// Start pending message claimer
	b.workers.Add(1)
	go b.claimPendingMessages()

	// Start metrics reporter
	b.workers.Add(1)
	go b.reportMetrics()

	// Start stream maintenance
	b.workers.Add(1)
	go b.maintainStreams()

	return nil
}

// Stop gracefully stops the message broker
func (b *AgentMessageBroker) Stop() {
	b.stopOnce.Do(func() {
		b.logger.Info("Stopping agent message broker", nil)
		close(b.stopCh)
		b.workers.Wait()
		b.logger.Info("Agent message broker stopped", nil)
	})
}

// SendMessage sends a message to the broker for routing
func (b *AgentMessageBroker) SendMessage(ctx context.Context, msg *AgentMessage) error {
	// Validate message
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	// Set TTL if not specified
	if msg.TTL == 0 {
		msg.TTL = time.Now().Add(b.config.StreamTTL).Unix()
	}

	// Marshal message
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Determine target stream based on routing
	streamKey := b.determineTargetStream(msg)

	// Add to stream
	values := map[string]interface{}{
		"message":        string(msgBytes),
		"source_agent":   msg.SourceAgentID,
		"target_agent":   msg.TargetAgentID,
		"capability":     msg.TargetCapability,
		"priority":       msg.Priority,
		"message_type":   msg.MessageType,
		"correlation_id": msg.CorrelationID,
	}

	messageID, err := b.streamsClient.AddToStream(ctx, streamKey, values)
	if err != nil {
		return fmt.Errorf("failed to add message to stream: %w", err)
	}

	b.logger.Debug("Message sent to broker", map[string]interface{}{
		"message_id":   messageID,
		"source_agent": msg.SourceAgentID,
		"target_agent": msg.TargetAgentID,
		"capability":   msg.TargetCapability,
		"stream":       streamKey,
	})

	// Update metrics
	b.metricsMu.Lock()
	b.messageMetrics.MessagesRouted++
	b.metricsMu.Unlock()

	b.metrics.IncrementCounter("agent_messages_sent", 1)

	return nil
}

// RouteByCapability routes a message to agents with specific capability
func (b *AgentMessageBroker) RouteByCapability(ctx context.Context, capability string, msg *AgentMessage) error {
	// Find agents with the capability
	agents, err := b.discoverAgentsByCapability(ctx, capability)
	if err != nil {
		return fmt.Errorf("failed to discover agents with capability %s: %w", capability, err)
	}

	if len(agents) == 0 {
		return fmt.Errorf("no agents found with capability: %s", capability)
	}

	// Route to all capable agents or select based on load
	targetAgent := b.selectBestAgent(agents, msg)

	msg.TargetAgentID = targetAgent
	msg.TargetCapability = capability

	return b.SendMessage(ctx, msg)
}

// messageWorker processes messages from the main stream
func (b *AgentMessageBroker) messageWorker(workerID int) {
	defer b.workers.Done()

	b.logger.Info("Message worker started", map[string]interface{}{
		"worker_id": workerID,
	})

	for {
		select {
		case <-b.stopCh:
			b.logger.Info("Message worker stopping", map[string]interface{}{
				"worker_id": workerID,
			})
			return
		default:
			b.processMessages(workerID, b.config.BaseStreamKey)
		}
	}
}

// routingWorker handles direct agent-to-agent routing
func (b *AgentMessageBroker) routingWorker(workerID int) {
	defer b.workers.Done()

	routingStream := fmt.Sprintf("%s:routing", b.config.BaseStreamKey)

	for {
		select {
		case <-b.stopCh:
			return
		default:
			b.processMessages(workerID, routingStream)
		}
	}
}

// capabilityWorker handles capability-based routing
func (b *AgentMessageBroker) capabilityWorker(workerID int) {
	defer b.workers.Done()

	capabilityStream := fmt.Sprintf("%s:capability", b.config.BaseStreamKey)

	for {
		select {
		case <-b.stopCh:
			return
		default:
			b.processCapabilityMessages(workerID, capabilityStream)
		}
	}
}

// processMessages processes messages from a stream
func (b *AgentMessageBroker) processMessages(workerID int, streamKey string) {
	ctx, cancel := context.WithTimeout(context.Background(), b.config.BlockTimeout)
	defer cancel()

	client := b.streamsClient.GetClient()
	consumerGroup := b.getConsumerGroup(streamKey)

	// Read messages from the stream
	messages, err := client.XReadGroup(ctx, &redisclient.XReadGroupArgs{
		Group:    consumerGroup,
		Consumer: b.consumerID,
		Streams:  []string{streamKey, ">"},
		Count:    b.config.BatchSize,
		Block:    b.config.BlockTimeout,
	}).Result()

	if err != nil {
		if err != redisclient.Nil {
			b.logger.Error("Failed to read messages", map[string]interface{}{
				"error":     err.Error(),
				"worker_id": workerID,
				"stream":    streamKey,
			})
		}
		return
	}

	// Process each message
	for _, stream := range messages {
		for _, message := range stream.Messages {
			b.processMessage(workerID, streamKey, message)
		}
	}
}

// processCapabilityMessages processes capability-based routing messages
func (b *AgentMessageBroker) processCapabilityMessages(workerID int, streamKey string) {
	ctx, cancel := context.WithTimeout(context.Background(), b.config.BlockTimeout)
	defer cancel()

	client := b.streamsClient.GetClient()
	consumerGroup := b.getConsumerGroup(streamKey)

	// Read messages
	messages, err := client.XReadGroup(ctx, &redisclient.XReadGroupArgs{
		Group:    consumerGroup,
		Consumer: b.consumerID,
		Streams:  []string{streamKey, ">"},
		Count:    b.config.BatchSize,
		Block:    b.config.BlockTimeout,
	}).Result()

	if err != nil {
		if err != redisclient.Nil {
			b.logger.Error("Failed to read capability messages", map[string]interface{}{
				"error":     err.Error(),
				"worker_id": workerID,
			})
		}
		return
	}

	// Process each message
	for _, stream := range messages {
		for _, message := range stream.Messages {
			b.processCapabilityMessage(workerID, message)
		}
	}
}

// processMessage processes a single message
func (b *AgentMessageBroker) processMessage(workerID int, streamKey string, message redisclient.XMessage) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), b.config.ProcessTimeout)
	defer cancel()

	// Extract message data
	msgData, ok := message.Values["message"].(string)
	if !ok {
		b.logger.Error("Invalid message format", map[string]interface{}{
			"message_id": message.ID,
		})
		b.acknowledgeMessage(ctx, streamKey, message.ID)
		return
	}

	// Unmarshal agent message
	var agentMsg AgentMessage
	if err := json.Unmarshal([]byte(msgData), &agentMsg); err != nil {
		b.logger.Error("Failed to unmarshal message", map[string]interface{}{
			"message_id": message.ID,
			"error":      err.Error(),
		})
		b.handleProcessingError(ctx, streamKey, message, &agentMsg, err)
		return
	}

	// Check TTL
	if agentMsg.TTL > 0 && time.Now().Unix() > agentMsg.TTL {
		b.logger.Info("Message expired", map[string]interface{}{
			"message_id": agentMsg.ID,
			"ttl":        agentMsg.TTL,
		})
		b.acknowledgeMessage(ctx, streamKey, message.ID)
		return
	}

	// Route the message
	if err := b.routeMessage(ctx, &agentMsg); err != nil {
		b.handleProcessingError(ctx, streamKey, message, &agentMsg, err)
		return
	}

	// Acknowledge the message
	b.acknowledgeMessage(ctx, streamKey, message.ID)

	// Update metrics
	b.updateMetrics(&agentMsg, time.Since(start))
}

// processCapabilityMessage processes a capability-based routing message
func (b *AgentMessageBroker) processCapabilityMessage(workerID int, message redisclient.XMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), b.config.ProcessTimeout)
	defer cancel()

	// Extract message and capability
	msgData, _ := message.Values["message"].(string)
	capability, _ := message.Values["capability"].(string)

	var agentMsg AgentMessage
	if err := json.Unmarshal([]byte(msgData), &agentMsg); err != nil {
		b.logger.Error("Failed to unmarshal capability message", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Find agents with capability
	agents, err := b.discoverAgentsByCapability(ctx, capability)
	if err != nil || len(agents) == 0 {
		b.logger.Warn("No agents found for capability", map[string]interface{}{
			"capability": capability,
		})
		b.moveToDeadLetter(ctx, message, &agentMsg, fmt.Errorf("no agents with capability: %s", capability))
		return
	}

	// Route to best agent
	targetAgent := b.selectBestAgent(agents, &agentMsg)
	agentMsg.TargetAgentID = targetAgent

	// Deliver to target agent
	if err := b.deliverToAgent(ctx, targetAgent, &agentMsg); err != nil {
		b.logger.Error("Failed to deliver to agent", map[string]interface{}{
			"target_agent": targetAgent,
			"error":        err.Error(),
		})
		b.moveToDeadLetter(ctx, message, &agentMsg, err)
		return
	}

	// Update capability metrics
	b.metricsMu.Lock()
	b.messageMetrics.CapabilityHits[capability]++
	b.metricsMu.Unlock()
}

// Helper methods

func (b *AgentMessageBroker) initializeStreams(ctx context.Context) error {
	// Create main stream consumer group
	streams := []string{
		b.config.BaseStreamKey,
		fmt.Sprintf("%s:routing", b.config.BaseStreamKey),
		fmt.Sprintf("%s:capability", b.config.BaseStreamKey),
		b.config.DeadLetterStream,
	}

	for _, stream := range streams {
		consumerGroup := fmt.Sprintf("%s-%s", b.config.ConsumerGroup, stream)
		err := b.streamsClient.CreateConsumerGroupMkStream(ctx, stream, consumerGroup, "$")
		if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
			return fmt.Errorf("failed to create consumer group for %s: %w", stream, err)
		}
		b.consumerGroups[stream] = consumerGroup
	}

	b.logger.Info("Initialized message broker streams", map[string]interface{}{
		"streams": streams,
	})

	return nil
}

func (b *AgentMessageBroker) loadDefaultRoutingRules() {
	// Load some default routing rules
	rules := []RoutingRule{
		{
			ID:         "ide-to-jira",
			Name:       "IDE to Jira routing",
			SourceType: "ide",
			TargetType: "jira",
			Capability: "issue_management",
			Priority:   10,
			Active:     true,
			CreatedAt:  time.Now(),
		},
		{
			ID:         "slack-to-ide",
			Name:       "Slack to IDE routing",
			SourceType: "slack",
			TargetType: "ide",
			Capability: "code_assistance",
			Priority:   10,
			Active:     true,
			CreatedAt:  time.Now(),
		},
		{
			ID:         "monitoring-to-slack",
			Name:       "Monitoring alerts to Slack",
			SourceType: "monitoring",
			TargetType: "slack",
			Capability: "notifications",
			Priority:   5,
			Active:     true,
			CreatedAt:  time.Now(),
		},
	}

	for _, rule := range rules {
		b.routingRules.Store(rule.ID, rule)
	}
}

func (b *AgentMessageBroker) determineTargetStream(msg *AgentMessage) string {
	// Determine which stream to use based on message type
	if msg.TargetAgentID != "" {
		// Direct routing
		return fmt.Sprintf("%s:routing", b.config.BaseStreamKey)
	} else if msg.TargetCapability != "" {
		// Capability-based routing
		return fmt.Sprintf("%s:capability", b.config.BaseStreamKey)
	}

	// Default stream
	return b.config.BaseStreamKey
}

func (b *AgentMessageBroker) getConsumerGroup(streamKey string) string {
	if group, ok := b.consumerGroups[streamKey]; ok {
		return group
	}
	return b.config.ConsumerGroup
}

func (b *AgentMessageBroker) routeMessage(ctx context.Context, msg *AgentMessage) error {
	// If target agent specified, deliver directly
	if msg.TargetAgentID != "" {
		return b.deliverToAgent(ctx, msg.TargetAgentID, msg)
	}

	// If capability specified, route by capability
	if msg.TargetCapability != "" {
		return b.RouteByCapability(ctx, msg.TargetCapability, msg)
	}

	// Apply routing rules
	rule := b.findMatchingRule(msg)
	if rule != nil {
		return b.applyRoutingRule(ctx, rule, msg)
	}

	return fmt.Errorf("no routing path found for message")
}

func (b *AgentMessageBroker) deliverToAgent(ctx context.Context, agentID string, msg *AgentMessage) error {
	// Get agent's dedicated stream
	streamKey := b.getAgentStream(agentID)

	// Add message to agent's stream
	values := map[string]interface{}{
		"message":        msg,
		"delivered_at":   time.Now().Unix(),
		"correlation_id": msg.CorrelationID,
	}

	_, err := b.streamsClient.AddToStream(ctx, streamKey, values)
	if err != nil {
		return fmt.Errorf("failed to deliver to agent %s: %w", agentID, err)
	}

	// Update delivery metrics
	b.metricsMu.Lock()
	b.messageMetrics.AgentDeliveries[agentID]++
	b.metricsMu.Unlock()

	return nil
}

func (b *AgentMessageBroker) getAgentStream(agentID string) string {
	// Check cached stream
	if stream, ok := b.agentStreams.Load(agentID); ok {
		return stream.(string)
	}

	// Create agent-specific stream
	streamKey := fmt.Sprintf("%s:agent:%s", b.config.BaseStreamKey, agentID)
	b.agentStreams.Store(agentID, streamKey)

	return streamKey
}

func (b *AgentMessageBroker) discoverAgentsByCapability(ctx context.Context, capability string) ([]string, error) {
	// Get manifests with the capability
	// For now, use capability as both type and name
	manifests, err := b.manifestRepo.ListManifestsByCapability(ctx, capability, capability)
	if err != nil {
		return nil, err
	}

	var agentIDs []string
	for _, manifest := range manifests {
		// Check if any registrations are healthy
		registrations, err := b.manifestRepo.ListRegistrationsByManifest(ctx, manifest.ID)
		if err != nil {
			continue
		}

		for _, reg := range registrations {
			// Check if agent is healthy
			if reg.HealthStatus == models.RegistrationHealthHealthy {
				agentIDs = append(agentIDs, manifest.AgentID)
				break // Only need one healthy registration per manifest
			}
		}
	}

	return agentIDs, nil
}

func (b *AgentMessageBroker) selectBestAgent(agents []string, msg *AgentMessage) string {
	if len(agents) == 0 {
		return ""
	}

	// Simple round-robin for now
	// In production, consider load, health, and affinity

	// If high priority, select least loaded agent
	if msg.Priority > 5 {
		return b.selectLeastLoadedAgent(agents)
	}

	// Default to first available
	return agents[0]
}

func (b *AgentMessageBroker) selectLeastLoadedAgent(agents []string) string {
	// Track agent load and select least loaded
	// For now, simple selection
	if len(agents) > 0 {
		return agents[0]
	}
	return ""
}

func (b *AgentMessageBroker) findMatchingRule(msg *AgentMessage) *RoutingRule {
	var bestRule *RoutingRule
	highestPriority := -1

	b.routingRules.Range(func(key, value interface{}) bool {
		rule := value.(RoutingRule)

		if !rule.Active {
			return true
		}

		// Check if rule matches
		if (rule.SourceType == "*" || rule.SourceType == msg.SourceAgentType) &&
			(rule.TargetType == "*" || rule.TargetType == msg.TargetAgentType) {
			if rule.Priority > highestPriority {
				bestRule = &rule
				highestPriority = rule.Priority
			}
		}

		return true
	})

	return bestRule
}

func (b *AgentMessageBroker) applyRoutingRule(ctx context.Context, rule *RoutingRule, msg *AgentMessage) error {
	// Apply the routing rule
	if rule.Capability != "" {
		msg.TargetCapability = rule.Capability
		return b.RouteByCapability(ctx, rule.Capability, msg)
	}

	if rule.TargetType != "" && rule.TargetType != "*" {
		// Find agents of target type
		agents, err := b.discoverAgentsByType(ctx, rule.TargetType)
		if err != nil {
			return err
		}

		if len(agents) > 0 {
			msg.TargetAgentID = agents[0]
			return b.deliverToAgent(ctx, agents[0], msg)
		}
	}

	return fmt.Errorf("routing rule could not be applied")
}

func (b *AgentMessageBroker) discoverAgentsByType(ctx context.Context, agentType string) ([]string, error) {
	manifests, err := b.manifestRepo.ListManifestsByType(ctx, agentType)
	if err != nil {
		return nil, err
	}

	var agentIDs []string
	for _, manifest := range manifests {
		// Check if any registrations are healthy
		registrations, err := b.manifestRepo.ListRegistrationsByManifest(ctx, manifest.ID)
		if err != nil {
			continue
		}

		for _, reg := range registrations {
			if reg.HealthStatus == models.RegistrationHealthHealthy {
				agentIDs = append(agentIDs, manifest.AgentID)
				break
			}
		}
	}

	return agentIDs, nil
}

func (b *AgentMessageBroker) acknowledgeMessage(ctx context.Context, streamKey string, messageID string) {
	consumerGroup := b.getConsumerGroup(streamKey)
	if err := b.streamsClient.AckMessages(ctx, streamKey, consumerGroup, messageID); err != nil {
		b.logger.Error("Failed to acknowledge message", map[string]interface{}{
			"message_id": messageID,
			"error":      err.Error(),
		})
	}
}

func (b *AgentMessageBroker) handleProcessingError(ctx context.Context, streamKey string, message redisclient.XMessage, agentMsg *AgentMessage, err error) {
	agentMsg.RetryCount++

	b.logger.Error("Failed to process message", map[string]interface{}{
		"message_id":  agentMsg.ID,
		"error":       err.Error(),
		"retry_count": agentMsg.RetryCount,
	})

	if agentMsg.RetryCount >= b.config.MaxRetries {
		b.moveToDeadLetter(ctx, message, agentMsg, err)
	} else {
		// Requeue for retry by not acknowledging
		b.metricsMu.Lock()
		b.messageMetrics.MessagesFailed++
		b.metricsMu.Unlock()
	}
}

func (b *AgentMessageBroker) moveToDeadLetter(ctx context.Context, message redisclient.XMessage, agentMsg *AgentMessage, processingError error) {
	client := b.streamsClient.GetClient()

	// Add to dead letter stream
	dlqEntry := map[string]interface{}{
		"message":      message.Values["message"],
		"original_id":  message.ID,
		"failed_at":    time.Now().Unix(),
		"error":        processingError.Error(),
		"retry_count":  agentMsg.RetryCount,
		"source_agent": agentMsg.SourceAgentID,
		"target_agent": agentMsg.TargetAgentID,
	}

	if _, err := client.XAdd(ctx, &redisclient.XAddArgs{
		Stream: b.config.DeadLetterStream,
		Values: dlqEntry,
	}).Result(); err != nil {
		b.logger.Error("Failed to add to dead letter queue", map[string]interface{}{
			"message_id": agentMsg.ID,
			"error":      err.Error(),
		})
	}

	// Update metrics
	b.metricsMu.Lock()
	b.messageMetrics.MessagesDeadLetter++
	b.metricsMu.Unlock()
}

func (b *AgentMessageBroker) updateMetrics(msg *AgentMessage, duration time.Duration) {
	b.metricsMu.Lock()
	defer b.metricsMu.Unlock()

	b.messageMetrics.LastProcessedTime = time.Now()

	// Update routing latency
	key := fmt.Sprintf("%s->%s", msg.SourceAgentType, msg.TargetAgentType)
	if current, exists := b.messageMetrics.RoutingLatency[key]; exists {
		b.messageMetrics.RoutingLatency[key] = (current + duration) / 2
	} else {
		b.messageMetrics.RoutingLatency[key] = duration
	}
}

// Maintenance workers

func (b *AgentMessageBroker) claimPendingMessages() {
	defer b.workers.Done()

	ticker := time.NewTicker(b.config.ClaimMinIdleTime)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.processPendingMessages()
		}
	}
}

func (b *AgentMessageBroker) processPendingMessages() {
	ctx := context.Background()
	client := b.streamsClient.GetClient()

	for streamKey, consumerGroup := range b.consumerGroups {
		// Get pending messages
		pending, err := client.XPendingExt(ctx, &redisclient.XPendingExtArgs{
			Stream: streamKey,
			Group:  consumerGroup,
			Start:  "-",
			End:    "+",
			Count:  b.config.BatchSize,
		}).Result()

		if err != nil {
			b.logger.Error("Failed to get pending messages", map[string]interface{}{
				"stream": streamKey,
				"error":  err.Error(),
			})
			continue
		}

		// Claim old messages
		for _, msg := range pending {
			if msg.Idle >= b.config.ClaimMinIdleTime {
				b.claimAndProcessMessage(ctx, streamKey, msg.ID)
			}
		}
	}
}

func (b *AgentMessageBroker) claimAndProcessMessage(ctx context.Context, streamKey string, messageID string) {
	messages, err := b.streamsClient.ClaimMessages(
		ctx,
		streamKey,
		b.getConsumerGroup(streamKey),
		b.consumerID,
		b.config.ClaimMinIdleTime,
		messageID,
	)

	if err != nil {
		b.logger.Error("Failed to claim message", map[string]interface{}{
			"message_id": messageID,
			"error":      err.Error(),
		})
		return
	}

	// Process claimed messages
	for _, message := range messages {
		b.processMessage(-1, streamKey, message)
	}
}

func (b *AgentMessageBroker) maintainStreams() {
	defer b.workers.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.trimStreams()
		}
	}
}

func (b *AgentMessageBroker) trimStreams() {
	ctx := context.Background()

	// Trim all managed streams
	for streamKey := range b.consumerGroups {
		if err := b.streamsClient.TrimStream(ctx, streamKey, b.config.MaxStreamLength, b.config.TrimApproximate); err != nil {
			b.logger.Error("Failed to trim stream", map[string]interface{}{
				"stream": streamKey,
				"error":  err.Error(),
			})
		}
	}
}

func (b *AgentMessageBroker) reportMetrics() {
	defer b.workers.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.logMetrics()
		}
	}
}

func (b *AgentMessageBroker) logMetrics() {
	b.metricsMu.RLock()
	defer b.metricsMu.RUnlock()

	b.logger.Info("Message broker metrics", map[string]interface{}{
		"messages_routed":      b.messageMetrics.MessagesRouted,
		"messages_failed":      b.messageMetrics.MessagesFailed,
		"messages_dead_letter": b.messageMetrics.MessagesDeadLetter,
		"last_processed":       b.messageMetrics.LastProcessedTime,
		"consumer_id":          b.consumerID,
	})
}

// GetMetrics returns current broker metrics
func (b *AgentMessageBroker) GetMetrics() map[string]interface{} {
	b.metricsMu.RLock()
	defer b.metricsMu.RUnlock()

	return map[string]interface{}{
		"messages_routed":      b.messageMetrics.MessagesRouted,
		"messages_failed":      b.messageMetrics.MessagesFailed,
		"messages_dead_letter": b.messageMetrics.MessagesDeadLetter,
		"routing_latency":      b.messageMetrics.RoutingLatency,
		"agent_deliveries":     b.messageMetrics.AgentDeliveries,
		"capability_hits":      b.messageMetrics.CapabilityHits,
		"last_processed":       b.messageMetrics.LastProcessedTime,
		"num_workers":          b.config.NumWorkers,
		"consumer_id":          b.consumerID,
	}
}
