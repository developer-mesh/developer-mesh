package adaptive

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// APISIntegration integrates APIL with the existing MCP server
type APISIntegration struct {
	mu      sync.RWMutex
	logger  observability.Logger
	metrics observability.MetricsClient

	// Core APIL components
	telemetry    *ProtocolTelemetry
	selfHealing  *SelfHealingController
	router       *AdaptiveRouter
	orchestrator *MigrationOrchestrator

	// Integration points
	mcpHandler MCPHandler
	wsServer   WebSocketServer

	// State
	enabled bool
	ready   bool
}

// MCPHandler interface for MCP protocol handler
type MCPHandler interface {
	HandleMessage(conn *websocket.Conn, connID string, tenantID string, message []byte) error
	RemoveSession(connID string)
}

// WebSocketServer interface for WebSocket server
type WebSocketServer interface {
	RegisterMiddleware(middleware Middleware)
	GetConnections() map[string]*websocket.Conn
}

// Middleware for intercepting messages
type Middleware func(ctx context.Context, msg []byte, next func(context.Context, []byte) error) error

// NewAPISIntegration creates APIL integration
func NewAPISIntegration(
	logger observability.Logger,
	metrics observability.MetricsClient,
	mcpHandler MCPHandler,
	wsServer WebSocketServer,
) *APISIntegration {
	integration := &APISIntegration{
		logger:       logger,
		metrics:      metrics,
		telemetry:    NewProtocolTelemetry(logger, metrics),
		selfHealing:  NewSelfHealingController(logger, metrics),
		router:       NewAdaptiveRouter(logger, metrics),
		orchestrator: NewMigrationOrchestrator(logger, metrics),
		mcpHandler:   mcpHandler,
		wsServer:     wsServer,
	}

	return integration
}

// Initialize initializes the APIL system
func (ai *APISIntegration) Initialize(ctx context.Context) error {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	ai.logger.Info("Initializing Adaptive Protocol Intelligence Layer", nil)

	// Register middleware for message interception
	ai.wsServer.RegisterMiddleware(ai.messageMiddleware)

	// Start telemetry collection
	go ai.telemetry.continuousAnalysis()

	// Start self-healing monitoring
	go ai.selfHealing.continuousMonitoring()

	// Start router optimization
	go ai.router.continuousOptimization()

	// Mark as ready
	ai.ready = true
	ai.enabled = true

	ai.logger.Info("APIL initialized successfully", nil)

	return nil
}

// StartMigration starts the protocol migration
func (ai *APISIntegration) StartMigration(ctx context.Context) error {
	if !ai.ready {
		return fmt.Errorf("APIL not initialized")
	}

	ai.logger.Info("Starting protocol migration with APIL", nil)

	return ai.orchestrator.StartMigration(ctx)
}

// messageMiddleware intercepts and processes messages
func (ai *APISIntegration) messageMiddleware(ctx context.Context, msgBytes []byte, next func(context.Context, []byte) error) error {
	if !ai.enabled {
		return next(ctx, msgBytes)
	}

	startTime := time.Now()

	// Create message object for routing
	msg := Message{
		ID:        generateMessageID(),
		Size:      len(msgBytes),
		Timestamp: startTime,
		Content:   msgBytes,
	}

	// Determine protocol type
	if isMCPMessage(msgBytes) {
		msg.Protocol = "mcp"
		msg.Type = extractMCPMethod(msgBytes)
	} else {
		msg.Protocol = "custom"
		msg.Type = extractCustomType(msgBytes)
	}

	// Record telemetry
	defer func() {
		latency := time.Since(startTime)
		ai.telemetry.RecordMessage(ctx, msg.Protocol, msg.Type, msg.Size, latency)
	}()

	// Get routing decision
	decision, err := ai.router.Route(ctx, msg)
	if err != nil {
		ai.logger.Error("Routing failed", map[string]interface{}{
			"error":  err.Error(),
			"msg_id": msg.ID,
		})
		// Fall back to normal processing
		return next(ctx, msgBytes)
	}

	// Apply routing decision
	switch decision.RouteType {
	case RouteToCache:
		// Check cache
		if cached := ai.router.cache.Get(msg.ID); cached != nil {
			// Return cached response
			return nil
		}
		return next(ctx, msgBytes)

	case RouteToBatcher:
		// Add to batch
		ai.router.batcher.Add(msg)
		return nil // Will be processed in batch

	case RouteToOptimizer:
		// Optimize before processing
		optimized := ai.optimizeMessage(msg)
		return next(ctx, optimized)

	case RouteToCircuitBreaker:
		// Circuit breaker is open
		return fmt.Errorf("circuit breaker open for route")

	default:
		// Normal processing
		return next(ctx, msgBytes)
	}
}

// optimizeMessage optimizes a message before processing
func (ai *APISIntegration) optimizeMessage(msg Message) []byte {
	// Apply optimizations
	content := msg.Content.([]byte)

	// Compress if beneficial
	if msg.Size > 10240 { // > 10KB
		if compressed := ai.router.compressor.Compress(content); len(compressed) < len(content) {
			return compressed
		}
	}

	return content
}

// HandleFailure handles system failures
func (ai *APISIntegration) HandleFailure(ctx context.Context, component string, err error) {
	failure := FailureEvent{
		Type:      "system_failure",
		Component: component,
		Error:     err,
		Timestamp: time.Now(),
	}

	// Let self-healing handle it
	if healErr := ai.selfHealing.HandleFailure(ctx, failure); healErr != nil {
		ai.logger.Error("Self-healing failed", map[string]interface{}{
			"error":     healErr.Error(),
			"component": component,
		})
	}
}

// GetMetrics returns APIL metrics
func (ai *APISIntegration) GetMetrics() APISMetrics {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	// Gather metrics from all components
	return APISMetrics{
		Telemetry:   ai.telemetry.GetMetrics(),
		SelfHealing: ai.selfHealing.GetMetrics(),
		Router:      ai.router.GetMetrics(),
		Migration:   ai.orchestrator.GetMetrics(),
	}
}

// GetMigrationStatus returns current migration status
func (ai *APISIntegration) GetMigrationStatus() MigrationStatus {
	return ai.orchestrator.GetStatus()
}

// EnableAdaptiveRouting enables adaptive routing
func (ai *APISIntegration) EnableAdaptiveRouting(enabled bool) {
	ai.mu.Lock()
	ai.enabled = enabled
	ai.mu.Unlock()

	status := "disabled"
	if enabled {
		status = "enabled"
	}

	ai.logger.Info("Adaptive routing "+status, nil)
}

// Helper functions

func generateMessageID() string {
	return fmt.Sprintf("msg-%d", time.Now().UnixNano())
}

func isMCPMessage(data []byte) bool {
	// Check for JSON-RPC 2.0 signature
	return len(data) > 20 &&
		(contains(data, []byte(`"jsonrpc":"2.0"`)) ||
			contains(data, []byte(`"jsonrpc": "2.0"`)))
}

func extractMCPMethod(data []byte) string {
	// Simple extraction - would use proper JSON parsing in production
	if contains(data, []byte(`"method":"initialize"`)) {
		return "initialize"
	}
	if contains(data, []byte(`"method":"tools/`)) {
		return "tools"
	}
	if contains(data, []byte(`"method":"resources/`)) {
		return "resources"
	}
	return "unknown"
}

func extractCustomType(data []byte) string {
	// Extract custom protocol type
	if contains(data, []byte(`"type":"agent.`)) {
		return "agent"
	}
	if contains(data, []byte(`"type":"workflow.`)) {
		return "workflow"
	}
	if contains(data, []byte(`"type":"task.`)) {
		return "task"
	}
	return "unknown"
}

func contains(data, substr []byte) bool {
	if len(substr) > len(data) {
		return false
	}
	for i := 0; i <= len(data)-len(substr); i++ {
		if match(data[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func match(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Metrics types

type APISMetrics struct {
	Telemetry   TelemetryMetrics
	SelfHealing SelfHealingMetrics
	Router      RouterMetrics
	Migration   MigrationMetrics
}

type TelemetryMetrics struct {
	MessagesProcessed uint64
	PatternsDetected  int
	AnomaliesFound    int
}

type SelfHealingMetrics struct {
	IncidentsHandled    int
	RecoveriesSuccess   int
	RecoveriesFailed    int
	CircuitBreakersOpen int
}

type RouterMetrics struct {
	RoutingDecisions  uint64
	CacheHits         uint64
	CacheMisses       uint64
	BatchedMessages   uint64
	OptimizedMessages uint64
}

type MigrationMetrics struct {
	Stage              string
	CustomTraffic      uint64
	MCPTraffic         uint64
	ProgressPercentage float64
}

type MigrationStatus struct {
	Stage      string
	StartTime  time.Time
	Duration   time.Duration
	Safe       bool
	Confidence float64
	Progress   float64
}

// Component metric getters

func (pt *ProtocolTelemetry) GetMetrics() TelemetryMetrics {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	total := uint64(0)
	for _, pm := range pt.protocolUsage {
		total += pm.TotalMessages
	}

	return TelemetryMetrics{
		MessagesProcessed: total,
		PatternsDetected:  len(pt.messagePatterns.sequences),
	}
}

func (shc *SelfHealingController) GetMetrics() SelfHealingMetrics {
	shc.mu.RLock()
	defer shc.mu.RUnlock()

	openBreakers := 0
	for _, breaker := range shc.circuitNetwork.breakers {
		if breaker.state == CircuitOpen {
			openBreakers++
		}
	}

	return SelfHealingMetrics{
		IncidentsHandled:    len(shc.healingDecisions),
		CircuitBreakersOpen: openBreakers,
	}
}

func (ar *AdaptiveRouter) GetMetrics() RouterMetrics {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	return RouterMetrics{
		RoutingDecisions: uint64(len(ar.decisions)),
	}
}

func (mo *MigrationOrchestrator) GetMetrics() MigrationMetrics {
	mo.mu.RLock()
	defer mo.mu.RUnlock()

	stageName := ""
	switch mo.stage {
	case StageShadowMode:
		stageName = "shadow_mode"
	case StageCanary:
		stageName = "canary"
	case StageProgressive:
		stageName = "progressive"
	case StageMCPOnly:
		stageName = "mcp_only"
	case StageCleanup:
		stageName = "cleanup"
	case StageComplete:
		stageName = "complete"
	default:
		stageName = "pre_migration"
	}

	return MigrationMetrics{
		Stage:              stageName,
		CustomTraffic:      atomic.LoadUint64(&mo.customTraffic),
		MCPTraffic:         atomic.LoadUint64(&mo.mcpTraffic),
		ProgressPercentage: mo.getProgress(),
	}
}

func (mo *MigrationOrchestrator) GetStatus() MigrationStatus {
	mo.mu.RLock()
	defer mo.mu.RUnlock()

	return MigrationStatus{
		Stage:      mo.getStageName(),
		StartTime:  mo.startTime,
		Duration:   time.Since(mo.startTime),
		Safe:       mo.safetyMonitor.IsHealthy(),
		Confidence: mo.getConfidence(),
		Progress:   mo.getProgress(),
	}
}

func (mo *MigrationOrchestrator) getStageName() string {
	stages := map[MigrationStage]string{
		StagePreMigration: "pre_migration",
		StageShadowMode:   "shadow_mode",
		StageCanary:       "canary",
		StageProgressive:  "progressive",
		StageMCPOnly:      "mcp_only",
		StageCleanup:      "cleanup",
		StageComplete:     "complete",
	}
	return stages[mo.stage]
}

func (mo *MigrationOrchestrator) getConfidence() float64 {
	// Calculate confidence based on current metrics
	return 0.85 // Simplified
}

func (mo *MigrationOrchestrator) getProgress() float64 {
	// Calculate progress percentage
	switch mo.stage {
	case StagePreMigration:
		return 0
	case StageShadowMode:
		return 10
	case StageCanary:
		return 25
	case StageProgressive:
		return 50
	case StageMCPOnly:
		return 85
	case StageCleanup:
		return 95
	case StageComplete:
		return 100
	default:
		return 0
	}
}

// Stub implementations for missing methods

func (ab *AdaptiveBatcher) Compress(data []byte) []byte {
	// Compression implementation
	return data
}

func (pc *PredictiveCache) Compress(data []byte) []byte {
	// Compression implementation
	return data
}

type SmartCompressor struct{}

func (sc *SmartCompressor) Compress(data []byte) []byte {
	// Smart compression implementation
	return data
}
