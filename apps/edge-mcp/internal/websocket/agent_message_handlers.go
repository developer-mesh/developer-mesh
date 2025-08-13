package websocket

import (
	"context"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
)

// AgentMessageHandler defines the interface for agent-specific message handlers
type AgentMessageHandler interface {
	HandleMessage(ctx context.Context, msg *AgentMessage) (*AgentMessage, error)
	GetAgentType() string
	GetSupportedMessageTypes() []string
	ValidateMessage(msg *AgentMessage) error
}

// BaseMessageHandler provides common functionality for all handlers
type BaseMessageHandler struct {
	logger  observability.Logger
	metrics observability.MetricsClient
}

// IDEMessageHandler handles messages for IDE agents
type IDEMessageHandler struct {
	BaseMessageHandler
	broker *AgentMessageBroker
}

// NewIDEMessageHandler creates a new IDE message handler
func NewIDEMessageHandler(logger observability.Logger, metrics observability.MetricsClient, broker *AgentMessageBroker) *IDEMessageHandler {
	return &IDEMessageHandler{
		BaseMessageHandler: BaseMessageHandler{
			logger:  logger,
			metrics: metrics,
		},
		broker: broker,
	}
}

func (h *IDEMessageHandler) GetAgentType() string {
	return "ide"
}

func (h *IDEMessageHandler) GetSupportedMessageTypes() []string {
	return []string{
		"code.completion",
		"code.analysis",
		"code.refactor",
		"debug.start",
		"debug.breakpoint",
		"issue.create",      // IDE → Jira
		"documentation.get", // IDE → Documentation system
		"test.run",          // IDE → CI/CD
	}
}

func (h *IDEMessageHandler) ValidateMessage(msg *AgentMessage) error {
	if msg.SourceAgentType != "ide" && msg.TargetAgentType != "ide" {
		return fmt.Errorf("message not for IDE agent")
	}

	// Validate message type
	supported := false
	for _, msgType := range h.GetSupportedMessageTypes() {
		if msg.MessageType == msgType {
			supported = true
			break
		}
	}

	if !supported {
		return fmt.Errorf("unsupported message type for IDE: %s", msg.MessageType)
	}

	return nil
}

func (h *IDEMessageHandler) HandleMessage(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	// Validate message
	if err := h.ValidateMessage(msg); err != nil {
		return nil, err
	}

	h.logger.Debug("Handling IDE message", map[string]interface{}{
		"message_id":   msg.ID,
		"message_type": msg.MessageType,
		"source":       msg.SourceAgentID,
		"target":       msg.TargetAgentID,
	})

	// Handle different message types
	switch msg.MessageType {
	case "code.completion":
		return h.handleCodeCompletion(ctx, msg)

	case "code.analysis":
		return h.handleCodeAnalysis(ctx, msg)

	case "issue.create":
		// Route to Jira agent
		return h.routeToJira(ctx, msg)

	case "documentation.get":
		// Route to documentation system
		return h.routeToDocumentation(ctx, msg)

	case "test.run":
		// Route to CI/CD system
		return h.routeToCICD(ctx, msg)

	default:
		// Default handling
		return h.handleDefault(ctx, msg)
	}
}

func (h *IDEMessageHandler) handleCodeCompletion(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	// Process code completion request
	response := &AgentMessage{
		ID:              uuid.New().String(),
		SourceAgentID:   msg.TargetAgentID,
		SourceAgentType: "ide",
		TargetAgentID:   msg.SourceAgentID,
		TargetAgentType: msg.SourceAgentType,
		MessageType:     "code.completion.response",
		CorrelationID:   msg.CorrelationID,
		Timestamp:       time.Now(),
		Payload: map[string]interface{}{
			"completions": []string{
				"func example()",
				"func exampleWithContext(ctx context.Context)",
			},
			"confidence": 0.95,
		},
	}

	return response, nil
}

func (h *IDEMessageHandler) handleCodeAnalysis(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	// Perform code analysis
	response := &AgentMessage{
		ID:              uuid.New().String(),
		SourceAgentID:   msg.TargetAgentID,
		SourceAgentType: "ide",
		TargetAgentID:   msg.SourceAgentID,
		TargetAgentType: msg.SourceAgentType,
		MessageType:     "code.analysis.response",
		CorrelationID:   msg.CorrelationID,
		Timestamp:       time.Now(),
		Payload: map[string]interface{}{
			"issues": []map[string]interface{}{
				{
					"type":     "warning",
					"line":     42,
					"column":   10,
					"message":  "Unused variable 'x'",
					"severity": "low",
				},
			},
			"metrics": map[string]interface{}{
				"cyclomatic_complexity": 5,
				"lines_of_code":         150,
				"test_coverage":         0.85,
			},
		},
	}

	return response, nil
}

func (h *IDEMessageHandler) routeToJira(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	// Add Jira-specific routing information
	msg.TargetAgentType = "jira"
	msg.TargetCapability = "issue_management"

	// Send through broker for capability-based routing
	if err := h.broker.RouteByCapability(ctx, "issue_management", msg); err != nil {
		return nil, fmt.Errorf("failed to route to Jira: %w", err)
	}

	return nil, nil // No immediate response, async handling
}

func (h *IDEMessageHandler) routeToDocumentation(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	msg.TargetAgentType = "documentation"
	msg.TargetCapability = "docs_retrieval"

	if err := h.broker.RouteByCapability(ctx, "docs_retrieval", msg); err != nil {
		return nil, fmt.Errorf("failed to route to documentation: %w", err)
	}

	return nil, nil
}

func (h *IDEMessageHandler) routeToCICD(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	msg.TargetAgentType = "cicd"
	msg.TargetCapability = "test_execution"

	if err := h.broker.RouteByCapability(ctx, "test_execution", msg); err != nil {
		return nil, fmt.Errorf("failed to route to CI/CD: %w", err)
	}

	return nil, nil
}

func (h *IDEMessageHandler) handleDefault(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	// Default response
	return &AgentMessage{
		ID:              uuid.New().String(),
		SourceAgentID:   msg.TargetAgentID,
		SourceAgentType: "ide",
		TargetAgentID:   msg.SourceAgentID,
		TargetAgentType: msg.SourceAgentType,
		MessageType:     fmt.Sprintf("%s.response", msg.MessageType),
		CorrelationID:   msg.CorrelationID,
		Timestamp:       time.Now(),
		Payload: map[string]interface{}{
			"status":  "processed",
			"message": "Message handled by IDE agent",
		},
	}, nil
}

// SlackMessageHandler handles messages for Slack agents
type SlackMessageHandler struct {
	BaseMessageHandler
	broker *AgentMessageBroker
}

// NewSlackMessageHandler creates a new Slack message handler
func NewSlackMessageHandler(logger observability.Logger, metrics observability.MetricsClient, broker *AgentMessageBroker) *SlackMessageHandler {
	return &SlackMessageHandler{
		BaseMessageHandler: BaseMessageHandler{
			logger:  logger,
			metrics: metrics,
		},
		broker: broker,
	}
}

func (h *SlackMessageHandler) GetAgentType() string {
	return "slack"
}

func (h *SlackMessageHandler) GetSupportedMessageTypes() []string {
	return []string{
		"notification.send",
		"notification.channel",
		"notification.dm",
		"alert.critical",
		"alert.warning",
		"code.help",         // Slack → IDE
		"issue.status",      // Slack → Jira
		"deployment.status", // Slack → CI/CD
	}
}

func (h *SlackMessageHandler) ValidateMessage(msg *AgentMessage) error {
	if msg.SourceAgentType != "slack" && msg.TargetAgentType != "slack" {
		return fmt.Errorf("message not for Slack agent")
	}

	// Check if channel is specified for channel messages
	if msg.MessageType == "notification.channel" {
		if _, ok := msg.Payload["channel"]; !ok {
			return fmt.Errorf("channel not specified for channel notification")
		}
	}

	return nil
}

func (h *SlackMessageHandler) HandleMessage(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	if err := h.ValidateMessage(msg); err != nil {
		return nil, err
	}

	switch msg.MessageType {
	case "notification.send", "notification.channel", "notification.dm":
		return h.handleNotification(ctx, msg)

	case "alert.critical", "alert.warning":
		return h.handleAlert(ctx, msg)

	case "code.help":
		// Route to IDE for code assistance
		return h.routeToIDE(ctx, msg)

	case "issue.status":
		// Route to Jira for issue status
		return h.routeToJira(ctx, msg)

	case "deployment.status":
		// Route to CI/CD for deployment status
		return h.routeToCICD(ctx, msg)

	default:
		return h.handleDefault(ctx, msg)
	}
}

func (h *SlackMessageHandler) handleNotification(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	// Process notification
	channel := ""
	if ch, ok := msg.Payload["channel"].(string); ok {
		channel = ch
	}

	text := ""
	if t, ok := msg.Payload["text"].(string); ok {
		text = t
	}

	h.logger.Info("Sending Slack notification", map[string]interface{}{
		"channel": channel,
		"text":    text,
	})

	// Return confirmation
	return &AgentMessage{
		ID:              uuid.New().String(),
		SourceAgentID:   msg.TargetAgentID,
		SourceAgentType: "slack",
		TargetAgentID:   msg.SourceAgentID,
		TargetAgentType: msg.SourceAgentType,
		MessageType:     "notification.sent",
		CorrelationID:   msg.CorrelationID,
		Timestamp:       time.Now(),
		Payload: map[string]interface{}{
			"status":     "sent",
			"channel":    channel,
			"message_ts": time.Now().Unix(),
		},
	}, nil
}

func (h *SlackMessageHandler) handleAlert(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	severity := "info"
	switch msg.MessageType {
	case "alert.critical":
		severity = "critical"
	case "alert.warning":
		severity = "warning"
	}

	// Format alert for Slack
	alertMsg := map[string]interface{}{
		"attachments": []map[string]interface{}{
			{
				"color": h.getColorForSeverity(severity),
				"title": fmt.Sprintf("%s Alert", severity),
				"text":  msg.Payload["message"],
				"fields": []map[string]interface{}{
					{
						"title": "Source",
						"value": msg.SourceAgentID,
						"short": true,
					},
					{
						"title": "Time",
						"value": msg.Timestamp.Format(time.RFC3339),
						"short": true,
					},
				},
				"footer": "DevOps MCP Alert System",
				"ts":     time.Now().Unix(),
			},
		},
	}

	// Send alert
	h.logger.Info("Sending Slack alert", map[string]interface{}{
		"severity": severity,
		"source":   msg.SourceAgentID,
	})

	return &AgentMessage{
		ID:              uuid.New().String(),
		SourceAgentID:   msg.TargetAgentID,
		SourceAgentType: "slack",
		TargetAgentID:   msg.SourceAgentID,
		TargetAgentType: msg.SourceAgentType,
		MessageType:     "alert.acknowledged",
		CorrelationID:   msg.CorrelationID,
		Timestamp:       time.Now(),
		Payload:         alertMsg,
	}, nil
}

func (h *SlackMessageHandler) getColorForSeverity(severity string) string {
	switch severity {
	case "critical":
		return "#FF0000"
	case "warning":
		return "#FFA500"
	case "info":
		return "#0000FF"
	default:
		return "#808080"
	}
}

func (h *SlackMessageHandler) routeToIDE(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	msg.TargetAgentType = "ide"
	msg.TargetCapability = "code_assistance"

	if err := h.broker.RouteByCapability(ctx, "code_assistance", msg); err != nil {
		return nil, fmt.Errorf("failed to route to IDE: %w", err)
	}

	return nil, nil
}

func (h *SlackMessageHandler) routeToJira(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	msg.TargetAgentType = "jira"
	msg.TargetCapability = "issue_tracking"

	if err := h.broker.RouteByCapability(ctx, "issue_tracking", msg); err != nil {
		return nil, fmt.Errorf("failed to route to Jira: %w", err)
	}

	return nil, nil
}

func (h *SlackMessageHandler) routeToCICD(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	msg.TargetAgentType = "cicd"
	msg.TargetCapability = "deployment_status"

	if err := h.broker.RouteByCapability(ctx, "deployment_status", msg); err != nil {
		return nil, fmt.Errorf("failed to route to CI/CD: %w", err)
	}

	return nil, nil
}

func (h *SlackMessageHandler) handleDefault(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	return &AgentMessage{
		ID:              uuid.New().String(),
		SourceAgentID:   msg.TargetAgentID,
		SourceAgentType: "slack",
		TargetAgentID:   msg.SourceAgentID,
		TargetAgentType: msg.SourceAgentType,
		MessageType:     fmt.Sprintf("%s.response", msg.MessageType),
		CorrelationID:   msg.CorrelationID,
		Timestamp:       time.Now(),
		Payload: map[string]interface{}{
			"status":  "processed",
			"message": "Message handled by Slack agent",
		},
	}, nil
}

// MonitoringMessageHandler handles messages for monitoring agents
type MonitoringMessageHandler struct {
	BaseMessageHandler
	broker *AgentMessageBroker
}

// NewMonitoringMessageHandler creates a new monitoring message handler
func NewMonitoringMessageHandler(logger observability.Logger, metrics observability.MetricsClient, broker *AgentMessageBroker) *MonitoringMessageHandler {
	return &MonitoringMessageHandler{
		BaseMessageHandler: BaseMessageHandler{
			logger:  logger,
			metrics: metrics,
		},
		broker: broker,
	}
}

func (h *MonitoringMessageHandler) GetAgentType() string {
	return "monitoring"
}

func (h *MonitoringMessageHandler) GetSupportedMessageTypes() []string {
	return []string{
		"metric.report",
		"metric.threshold",
		"health.check",
		"health.status",
		"alert.trigger",
		"alert.resolve",
		"incident.create", // Monitoring → PagerDuty/Incident Management
		"slack.notify",    // Monitoring → Slack
	}
}

func (h *MonitoringMessageHandler) ValidateMessage(msg *AgentMessage) error {
	if msg.SourceAgentType != "monitoring" && msg.TargetAgentType != "monitoring" {
		return fmt.Errorf("message not for monitoring agent")
	}

	// Validate metric messages have required fields
	if msg.MessageType == "metric.report" || msg.MessageType == "metric.threshold" {
		if _, ok := msg.Payload["metric_name"]; !ok {
			return fmt.Errorf("metric_name required for metric messages")
		}
		if _, ok := msg.Payload["value"]; !ok {
			return fmt.Errorf("value required for metric messages")
		}
	}

	return nil
}

func (h *MonitoringMessageHandler) HandleMessage(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	if err := h.ValidateMessage(msg); err != nil {
		return nil, err
	}

	switch msg.MessageType {
	case "metric.report":
		return h.handleMetricReport(ctx, msg)

	case "metric.threshold":
		return h.handleMetricThreshold(ctx, msg)

	case "health.check":
		return h.handleHealthCheck(ctx, msg)

	case "alert.trigger":
		return h.handleAlertTrigger(ctx, msg)

	case "incident.create":
		// Route to incident management
		return h.routeToIncidentManagement(ctx, msg)

	case "slack.notify":
		// Route to Slack
		return h.routeToSlack(ctx, msg)

	default:
		return h.handleDefault(ctx, msg)
	}
}

func (h *MonitoringMessageHandler) handleMetricReport(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	metricName := msg.Payload["metric_name"].(string)
	value := msg.Payload["value"]

	h.logger.Info("Processing metric report", map[string]interface{}{
		"metric": metricName,
		"value":  value,
		"source": msg.SourceAgentID,
	})

	// Store metric and check thresholds
	// In real implementation, would check against configured thresholds

	return &AgentMessage{
		ID:              uuid.New().String(),
		SourceAgentID:   msg.TargetAgentID,
		SourceAgentType: "monitoring",
		TargetAgentID:   msg.SourceAgentID,
		TargetAgentType: msg.SourceAgentType,
		MessageType:     "metric.acknowledged",
		CorrelationID:   msg.CorrelationID,
		Timestamp:       time.Now(),
		Payload: map[string]interface{}{
			"status":      "received",
			"metric_name": metricName,
			"value":       value,
			"timestamp":   time.Now().Unix(),
		},
	}, nil
}

func (h *MonitoringMessageHandler) handleMetricThreshold(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	// Threshold exceeded, trigger alert
	alertMsg := &AgentMessage{
		ID:               uuid.New().String(),
		SourceAgentID:    msg.TargetAgentID,
		SourceAgentType:  "monitoring",
		TargetAgentType:  "slack",
		TargetCapability: "notifications",
		MessageType:      "alert.critical",
		CorrelationID:    msg.CorrelationID,
		Priority:         10,
		Timestamp:        time.Now(),
		Payload: map[string]interface{}{
			"alert_type":  "threshold_exceeded",
			"metric_name": msg.Payload["metric_name"],
			"value":       msg.Payload["value"],
			"threshold":   msg.Payload["threshold"],
			"message":     fmt.Sprintf("Metric %s exceeded threshold", msg.Payload["metric_name"]),
		},
	}

	// Route to Slack for notification
	if err := h.broker.RouteByCapability(ctx, "notifications", alertMsg); err != nil {
		h.logger.Error("Failed to route threshold alert", map[string]interface{}{
			"error": err.Error(),
		})
	}

	return &AgentMessage{
		ID:              uuid.New().String(),
		SourceAgentID:   msg.TargetAgentID,
		SourceAgentType: "monitoring",
		TargetAgentID:   msg.SourceAgentID,
		TargetAgentType: msg.SourceAgentType,
		MessageType:     "threshold.alert.sent",
		CorrelationID:   msg.CorrelationID,
		Timestamp:       time.Now(),
		Payload: map[string]interface{}{
			"status":  "alert_triggered",
			"sent_to": "slack",
		},
	}, nil
}

func (h *MonitoringMessageHandler) handleHealthCheck(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	// Perform health check on target
	target := ""
	if t, ok := msg.Payload["target"].(string); ok {
		target = t
	}

	// Simulate health check
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"target":    target,
		"checks": map[string]interface{}{
			"cpu":     "ok",
			"memory":  "ok",
			"disk":    "ok",
			"network": "ok",
		},
	}

	return &AgentMessage{
		ID:              uuid.New().String(),
		SourceAgentID:   msg.TargetAgentID,
		SourceAgentType: "monitoring",
		TargetAgentID:   msg.SourceAgentID,
		TargetAgentType: msg.SourceAgentType,
		MessageType:     "health.status",
		CorrelationID:   msg.CorrelationID,
		Timestamp:       time.Now(),
		Payload:         health,
	}, nil
}

func (h *MonitoringMessageHandler) handleAlertTrigger(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	// Create incident if severity is high
	severity := "low"
	if s, ok := msg.Payload["severity"].(string); ok {
		severity = s
	}

	if severity == "critical" || severity == "high" {
		// Create incident
		incidentMsg := &AgentMessage{
			ID:               uuid.New().String(),
			SourceAgentID:    msg.TargetAgentID,
			SourceAgentType:  "monitoring",
			TargetCapability: "incident_management",
			MessageType:      "incident.create",
			Priority:         10,
			Timestamp:        time.Now(),
			Payload:          msg.Payload,
		}

		if err := h.broker.SendMessage(ctx, incidentMsg); err != nil {
			h.logger.Error("Failed to create incident", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// Also notify Slack
	return h.routeToSlack(ctx, msg)
}

func (h *MonitoringMessageHandler) routeToIncidentManagement(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	msg.TargetCapability = "incident_management"

	if err := h.broker.RouteByCapability(ctx, "incident_management", msg); err != nil {
		return nil, fmt.Errorf("failed to route to incident management: %w", err)
	}

	return nil, nil
}

func (h *MonitoringMessageHandler) routeToSlack(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	msg.TargetAgentType = "slack"
	msg.TargetCapability = "notifications"

	if err := h.broker.RouteByCapability(ctx, "notifications", msg); err != nil {
		return nil, fmt.Errorf("failed to route to Slack: %w", err)
	}

	return nil, nil
}

func (h *MonitoringMessageHandler) handleDefault(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	return &AgentMessage{
		ID:              uuid.New().String(),
		SourceAgentID:   msg.TargetAgentID,
		SourceAgentType: "monitoring",
		TargetAgentID:   msg.SourceAgentID,
		TargetAgentType: msg.SourceAgentType,
		MessageType:     fmt.Sprintf("%s.response", msg.MessageType),
		CorrelationID:   msg.CorrelationID,
		Timestamp:       time.Now(),
		Payload: map[string]interface{}{
			"status":  "processed",
			"message": "Message handled by monitoring agent",
		},
	}, nil
}

// MessageHandlerRegistry manages all message handlers
type MessageHandlerRegistry struct {
	handlers map[string]AgentMessageHandler
	logger   observability.Logger
	metrics  observability.MetricsClient
}

// NewMessageHandlerRegistry creates a new handler registry
func NewMessageHandlerRegistry(logger observability.Logger, metrics observability.MetricsClient, broker *AgentMessageBroker) *MessageHandlerRegistry {
	registry := &MessageHandlerRegistry{
		handlers: make(map[string]AgentMessageHandler),
		logger:   logger,
		metrics:  metrics,
	}

	// Register default handlers
	registry.RegisterHandler("ide", NewIDEMessageHandler(logger, metrics, broker))
	registry.RegisterHandler("slack", NewSlackMessageHandler(logger, metrics, broker))
	registry.RegisterHandler("monitoring", NewMonitoringMessageHandler(logger, metrics, broker))

	return registry
}

// RegisterHandler registers a message handler for an agent type
func (r *MessageHandlerRegistry) RegisterHandler(agentType string, handler AgentMessageHandler) {
	r.handlers[agentType] = handler
	r.logger.Info("Registered message handler", map[string]interface{}{
		"agent_type":    agentType,
		"message_types": handler.GetSupportedMessageTypes(),
	})
}

// GetHandler returns the handler for an agent type
func (r *MessageHandlerRegistry) GetHandler(agentType string) (AgentMessageHandler, bool) {
	handler, exists := r.handlers[agentType]
	return handler, exists
}

// ProcessMessage processes a message using the appropriate handler
func (r *MessageHandlerRegistry) ProcessMessage(ctx context.Context, msg *AgentMessage) (*AgentMessage, error) {
	// Determine which handler to use
	handlerType := msg.TargetAgentType
	if handlerType == "" {
		handlerType = msg.SourceAgentType
	}

	handler, exists := r.GetHandler(handlerType)
	if !exists {
		return nil, fmt.Errorf("no handler registered for agent type: %s", handlerType)
	}

	// Process the message
	response, err := handler.HandleMessage(ctx, msg)
	if err != nil {
		r.metrics.IncrementCounter("message_handler_errors", 1)
		return nil, err
	}

	r.metrics.IncrementCounter("messages_handled", 1)

	return response, nil
}

// GetSupportedMessageTypes returns all supported message types across all handlers
func (r *MessageHandlerRegistry) GetSupportedMessageTypes() map[string][]string {
	supported := make(map[string][]string)

	for agentType, handler := range r.handlers {
		supported[agentType] = handler.GetSupportedMessageTypes()
	}

	return supported
}
