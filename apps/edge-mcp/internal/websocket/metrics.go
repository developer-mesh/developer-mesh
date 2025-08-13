package websocket

import (
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// MetricsCollector collects WebSocket metrics
type MetricsCollector struct {
	client observability.MetricsClient
	mu     sync.RWMutex

	// Connection metrics
	totalConnections  uint64
	activeConnections uint64
	failedConnections uint64

	// Message metrics
	messagesReceived uint64
	messagesSent     uint64
	messagesDropped  uint64
	batchesSent      uint64

	// Performance metrics
	messageLatencies    []float64
	batchLatencies      []float64
	connectionDurations []time.Duration

	// Error metrics
	authErrors      uint64
	rateLimitErrors uint64
	protocolErrors  uint64

	// Binary protocol metrics
	binaryMessages     uint64
	jsonMessages       uint64
	compressedMessages uint64

	// Per-tenant metrics
	tenantConnections map[string]uint64
	tenantMessages    map[string]uint64
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(client observability.MetricsClient) *MetricsCollector {
	mc := &MetricsCollector{
		client:              client,
		messageLatencies:    make([]float64, 0, 1000),
		batchLatencies:      make([]float64, 0, 1000),
		connectionDurations: make([]time.Duration, 0, 1000),
		tenantConnections:   make(map[string]uint64),
		tenantMessages:      make(map[string]uint64),
	}

	// Start periodic metrics export
	go mc.exportMetrics()

	return mc
}

// RecordConnection records a new connection
func (mc *MetricsCollector) RecordConnection(tenantID string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.totalConnections++
	mc.activeConnections++
	mc.tenantConnections[tenantID]++

	if mc.client != nil {
		mc.client.IncrementCounter("websocket_connections_total", 1)
		mc.client.RecordGauge("websocket_connections_active", float64(mc.activeConnections), nil)
		mc.client.RecordGauge("websocket_connections_tenant", float64(mc.tenantConnections[tenantID]),
			map[string]string{"tenant_id": tenantID})
	}
}

// RecordDisconnection records a connection close
func (mc *MetricsCollector) RecordDisconnection(tenantID string, duration time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.activeConnections--
	if count, ok := mc.tenantConnections[tenantID]; ok && count > 0 {
		mc.tenantConnections[tenantID]--
	}

	mc.connectionDurations = append(mc.connectionDurations, duration)
	if len(mc.connectionDurations) > 1000 {
		mc.connectionDurations = mc.connectionDurations[1:]
	}

	if mc.client != nil {
		mc.client.RecordGauge("websocket_connections_active", float64(mc.activeConnections), nil)
		mc.client.RecordHistogram("websocket_connection_duration_seconds", duration.Seconds(), nil)
	}
}

// RecordConnectionFailure records a failed connection attempt
func (mc *MetricsCollector) RecordConnectionFailure(reason string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.failedConnections++

	if mc.client != nil {
		mc.client.IncrementCounter("websocket_connections_failed_total", 1)
		mc.client.IncrementCounterWithLabels("websocket_connection_failures", 1,
			map[string]string{"reason": reason})
	}
}

// RecordMessage records message metrics
func (mc *MetricsCollector) RecordMessage(direction string, messageType string, tenantID string, latency time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if direction == "received" {
		mc.messagesReceived++
	} else {
		mc.messagesSent++
	}

	mc.tenantMessages[tenantID]++

	latencySeconds := latency.Seconds()
	mc.messageLatencies = append(mc.messageLatencies, latencySeconds)
	if len(mc.messageLatencies) > 1000 {
		mc.messageLatencies = mc.messageLatencies[1:]
	}

	if mc.client != nil {
		mc.client.IncrementCounterWithLabels("websocket_messages_total", 1, map[string]string{
			"direction": direction,
			"type":      messageType,
			"tenant_id": tenantID,
		})
		mc.client.RecordHistogram("websocket_message_latency_seconds", latencySeconds,
			map[string]string{"type": messageType})
	}
}

// RecordBatch records batch metrics
func (mc *MetricsCollector) RecordBatch(size int, latency time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.batchesSent++

	latencySeconds := latency.Seconds()
	mc.batchLatencies = append(mc.batchLatencies, latencySeconds)
	if len(mc.batchLatencies) > 1000 {
		mc.batchLatencies = mc.batchLatencies[1:]
	}

	if mc.client != nil {
		mc.client.IncrementCounter("websocket_batches_sent_total", 1)
		mc.client.RecordHistogram("websocket_batch_size", float64(size), nil)
		mc.client.RecordHistogram("websocket_batch_latency_seconds", latencySeconds, nil)
	}
}

// RecordProtocolUsage records protocol type usage
func (mc *MetricsCollector) RecordProtocolUsage(protocol string, compressed bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if protocol == "binary" {
		mc.binaryMessages++
	} else {
		mc.jsonMessages++
	}

	if compressed {
		mc.compressedMessages++
	}

	if mc.client != nil {
		mc.client.IncrementCounterWithLabels("websocket_protocol_messages_total", 1, map[string]string{
			"protocol":   protocol,
			"compressed": boolToString(compressed),
		})
	}
}

// RecordError records error metrics
func (mc *MetricsCollector) RecordError(errorType string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	switch errorType {
	case "auth":
		mc.authErrors++
	case "rate_limit":
		mc.rateLimitErrors++
	case "protocol":
		mc.protocolErrors++
	}

	if mc.client != nil {
		mc.client.IncrementCounterWithLabels("websocket_errors_total", 1,
			map[string]string{"type": errorType})
	}
}

// RecordMessageDropped records dropped messages
func (mc *MetricsCollector) RecordMessageDropped(reason string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.messagesDropped++

	if mc.client != nil {
		mc.client.IncrementCounterWithLabels("websocket_messages_dropped_total", 1,
			map[string]string{"reason": reason})
	}
}

// GetStats returns current statistics
func (mc *MetricsCollector) GetStats() WebSocketStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	stats := WebSocketStats{
		TotalConnections:   mc.totalConnections,
		ActiveConnections:  mc.activeConnections,
		FailedConnections:  mc.failedConnections,
		MessagesReceived:   mc.messagesReceived,
		MessagesSent:       mc.messagesSent,
		MessagesDropped:    mc.messagesDropped,
		BatchesSent:        mc.batchesSent,
		BinaryMessages:     mc.binaryMessages,
		JSONMessages:       mc.jsonMessages,
		CompressedMessages: mc.compressedMessages,
		AuthErrors:         mc.authErrors,
		RateLimitErrors:    mc.rateLimitErrors,
		ProtocolErrors:     mc.protocolErrors,
	}

	// Calculate averages
	if len(mc.messageLatencies) > 0 {
		sum := 0.0
		for _, v := range mc.messageLatencies {
			sum += v
		}
		stats.AvgMessageLatency = sum / float64(len(mc.messageLatencies))
	}

	if len(mc.connectionDurations) > 0 {
		sum := time.Duration(0)
		for _, v := range mc.connectionDurations {
			sum += v
		}
		stats.AvgConnectionDuration = sum / time.Duration(len(mc.connectionDurations))
	}

	// Copy tenant stats
	stats.TenantStats = make(map[string]TenantStats)
	for tenantID, connections := range mc.tenantConnections {
		stats.TenantStats[tenantID] = TenantStats{
			Connections: connections,
			Messages:    mc.tenantMessages[tenantID],
		}
	}

	return stats
}

// exportMetrics periodically exports metrics
func (mc *MetricsCollector) exportMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := mc.GetStats()

		if mc.client != nil {
			// Export aggregate metrics
			mc.client.RecordGauge("websocket_stats_total_connections", float64(stats.TotalConnections), nil)
			mc.client.RecordGauge("websocket_stats_active_connections", float64(stats.ActiveConnections), nil)
			mc.client.RecordGauge("websocket_stats_messages_received", float64(stats.MessagesReceived), nil)
			mc.client.RecordGauge("websocket_stats_messages_sent", float64(stats.MessagesSent), nil)
			mc.client.RecordGauge("websocket_stats_avg_latency_ms", stats.AvgMessageLatency*1000, nil)

			// Protocol distribution
			totalProtocolMessages := float64(stats.BinaryMessages + stats.JSONMessages)
			if totalProtocolMessages > 0 {
				binaryRatio := float64(stats.BinaryMessages) / totalProtocolMessages
				mc.client.RecordGauge("websocket_protocol_binary_ratio", binaryRatio, nil)
			}

			// Error rates
			mc.client.RecordGauge("websocket_error_rate_auth", float64(stats.AuthErrors), nil)
			mc.client.RecordGauge("websocket_error_rate_limit", float64(stats.RateLimitErrors), nil)
			mc.client.RecordGauge("websocket_error_rate_protocol", float64(stats.ProtocolErrors), nil)
		}
	}
}

// WebSocketStats contains WebSocket statistics
type WebSocketStats struct {
	// Connection stats
	TotalConnections      uint64
	ActiveConnections     uint64
	FailedConnections     uint64
	AvgConnectionDuration time.Duration

	// Message stats
	MessagesReceived  uint64
	MessagesSent      uint64
	MessagesDropped   uint64
	BatchesSent       uint64
	AvgMessageLatency float64 // seconds

	// Protocol stats
	BinaryMessages     uint64
	JSONMessages       uint64
	CompressedMessages uint64

	// Error stats
	AuthErrors      uint64
	RateLimitErrors uint64
	ProtocolErrors  uint64

	// Per-tenant stats
	TenantStats map[string]TenantStats
}

// TenantStats contains per-tenant statistics
type TenantStats struct {
	Connections uint64
	Messages    uint64
}

// Helper function
func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// Collaboration-specific metrics

// RecordTaskCreated records task creation metrics
func (mc *MetricsCollector) RecordTaskCreated(tenantID string, taskType string, priority string) {
	if mc.client != nil {
		mc.client.IncrementCounterWithLabels("mcp_task_created_total", 1, map[string]string{
			"tenant_id": tenantID,
			"task_type": taskType,
			"priority":  priority,
		})
	}
}

// RecordTaskDelegated records task delegation metrics
func (mc *MetricsCollector) RecordTaskDelegated(tenantID string, fromAgent string, toAgent string, delegationType string) {
	if mc.client != nil {
		mc.client.IncrementCounterWithLabels("mcp_task_delegated_total", 1, map[string]string{
			"tenant_id":       tenantID,
			"from_agent":      fromAgent,
			"to_agent":        toAgent,
			"delegation_type": delegationType,
		})
	}
}

// RecordTaskCompleted records task completion metrics
func (mc *MetricsCollector) RecordTaskCompleted(tenantID string, agentID string, status string, duration time.Duration) {
	if mc.client != nil {
		mc.client.IncrementCounterWithLabels("mcp_task_completed_total", 1, map[string]string{
			"tenant_id": tenantID,
			"agent_id":  agentID,
			"status":    status,
		})
		mc.client.RecordHistogram("mcp_task_duration_seconds", duration.Seconds(), map[string]string{
			"tenant_id": tenantID,
			"status":    status,
		})
	}
}

// RecordWorkflowStarted records workflow start metrics
func (mc *MetricsCollector) RecordWorkflowStarted(tenantID string, workflowType string) {
	if mc.client != nil {
		mc.client.IncrementCounterWithLabels("mcp_workflow_started_total", 1, map[string]string{
			"tenant_id":     tenantID,
			"workflow_type": workflowType,
		})
	}
}

// RecordWorkflowCompleted records workflow completion metrics
func (mc *MetricsCollector) RecordWorkflowCompleted(tenantID string, workflowType string, status string, duration time.Duration) {
	if mc.client != nil {
		mc.client.IncrementCounterWithLabels("mcp_workflow_completed_total", 1, map[string]string{
			"tenant_id":     tenantID,
			"workflow_type": workflowType,
			"status":        status,
		})
		mc.client.RecordHistogram("mcp_workflow_duration_seconds", duration.Seconds(), map[string]string{
			"tenant_id":     tenantID,
			"workflow_type": workflowType,
		})
	}
}

// RecordWorkspaceActivity records workspace activity metrics
func (mc *MetricsCollector) RecordWorkspaceActivity(tenantID string, workspaceID string, activityType string) {
	if mc.client != nil {
		mc.client.IncrementCounterWithLabels("mcp_workspace_activity_total", 1, map[string]string{
			"tenant_id":     tenantID,
			"workspace_id":  workspaceID,
			"activity_type": activityType,
		})
	}
}

// RecordDocumentEdit records document edit metrics
func (mc *MetricsCollector) RecordDocumentEdit(tenantID string, workspaceID string, documentID string, editType string) {
	if mc.client != nil {
		mc.client.IncrementCounterWithLabels("mcp_document_edits_total", 1, map[string]string{
			"tenant_id":    tenantID,
			"workspace_id": workspaceID,
			"document_id":  documentID,
			"edit_type":    editType,
		})
	}
}

// RecordCRDTOperation records CRDT operation metrics
func (mc *MetricsCollector) RecordCRDTOperation(tenantID string, crdtType string, operation string, duration time.Duration) {
	if mc.client != nil {
		mc.client.IncrementCounterWithLabels("mcp_crdt_operations_total", 1, map[string]string{
			"tenant_id": tenantID,
			"crdt_type": crdtType,
			"operation": operation,
		})
		mc.client.RecordHistogram("mcp_crdt_operation_duration_seconds", duration.Seconds(), map[string]string{
			"tenant_id": tenantID,
			"crdt_type": crdtType,
		})
	}
}

// RecordConflictResolution records conflict resolution metrics
func (mc *MetricsCollector) RecordConflictResolution(tenantID string, conflictType string, resolution string) {
	if mc.client != nil {
		mc.client.IncrementCounterWithLabels("mcp_conflicts_resolved_total", 1, map[string]string{
			"tenant_id":     tenantID,
			"conflict_type": conflictType,
			"resolution":    resolution,
		})
	}
}

// RecordAgentCapabilityMatch records agent capability matching metrics
func (mc *MetricsCollector) RecordAgentCapabilityMatch(tenantID string, capability string, matched bool) {
	if mc.client != nil {
		mc.client.IncrementCounterWithLabels("mcp_capability_matches_total", 1, map[string]string{
			"tenant_id":  tenantID,
			"capability": capability,
			"matched":    boolToString(matched),
		})
	}
}

// MetricsRegistry provides centralized metrics registration
type MetricsRegistry struct {
	collectors map[string]*MetricsCollector
	mu         sync.RWMutex
}

// NewMetricsRegistry creates a new metrics registry
func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		collectors: make(map[string]*MetricsCollector),
	}
}

// Register registers a metrics collector
func (mr *MetricsRegistry) Register(name string, collector *MetricsCollector) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.collectors[name] = collector
}

// Get retrieves a metrics collector
func (mr *MetricsRegistry) Get(name string) (*MetricsCollector, bool) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	collector, ok := mr.collectors[name]
	return collector, ok
}

// GetAllStats returns stats from all collectors
func (mr *MetricsRegistry) GetAllStats() map[string]WebSocketStats {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	stats := make(map[string]WebSocketStats)
	for name, collector := range mr.collectors {
		stats[name] = collector.GetStats()
	}

	return stats
}
