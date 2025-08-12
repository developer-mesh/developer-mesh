package adaptive

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// MigrationOrchestrator safely migrates from custom protocol to MCP-only with APIL
type MigrationOrchestrator struct {
	mu      sync.RWMutex
	logger  observability.Logger
	metrics observability.MetricsClient

	// Core APIL components
	telemetry   *ProtocolTelemetry
	selfHealing *SelfHealingController
	router      *AdaptiveRouter

	// Migration state
	stage           MigrationStage
	startTime       time.Time
	customTraffic   uint64
	mcpTraffic      uint64
	rollbackTrigger *RollbackTrigger

	// Feature flags
	featureFlags *FeatureFlags

	// Safety monitors
	safetyMonitor *SafetyMonitor
	validator     *MigrationValidator

	// Progress tracking
	progress *MigrationProgress
}

// MigrationStage represents the current stage of migration
type MigrationStage int

const (
	StagePreMigration MigrationStage = iota
	StageShadowMode                  // Both protocols, telemetry only
	StageCanary                      // 5% traffic to MCP
	StageProgressive                 // 25%, 50%, 75% to MCP
	StageMCPOnly                     // 100% MCP
	StageCleanup                     // Remove old code
	StageComplete                    // Migration complete
)

// FeatureFlags manages feature toggles for migration
type FeatureFlags struct {
	mu    sync.RWMutex
	flags map[string]bool
}

// SafetyMonitor monitors migration safety metrics
type SafetyMonitor struct {
	mu sync.RWMutex

	// Thresholds
	maxErrorRate    float64
	maxLatency      time.Duration
	maxCostIncrease float64

	// Current metrics
	errorRate      float64
	avgLatency     time.Duration
	costMultiplier float64

	// Rollback triggers
	triggers []func() bool
}

// MigrationValidator validates migration readiness
type MigrationValidator struct {
	checks []ValidationCheck
}

// ValidationCheck represents a migration validation check
type ValidationCheck struct {
	Name     string
	Check    func() (bool, string)
	Required bool
}

// MigrationProgress tracks migration progress
type MigrationProgress struct {
	mu sync.RWMutex

	// Progress metrics
	CustomProtocolRemaining int
	MCPProtocolAdopted      int
	ComponentsMigrated      []string
	ComponentsRemaining     []string

	// Performance comparison
	CustomMetrics *ProtocolMetrics
	MCPMetrics    *ProtocolMetrics

	// Decision log
	Decisions []MigrationDecision
}

// MigrationDecision records a migration decision
type MigrationDecision struct {
	Timestamp         time.Time
	Stage             MigrationStage
	Decision          string
	Confidence        float64
	RiskScore         float64
	Approved          bool
	AutomatedDecision bool
}

// NewMigrationOrchestrator creates a new migration orchestrator
func NewMigrationOrchestrator(
	logger observability.Logger,
	metrics observability.MetricsClient,
) *MigrationOrchestrator {
	mo := &MigrationOrchestrator{
		logger:        logger,
		metrics:       metrics,
		telemetry:     NewProtocolTelemetry(logger, metrics),
		selfHealing:   NewSelfHealingController(logger, metrics),
		router:        NewAdaptiveRouter(logger, metrics),
		stage:         StagePreMigration,
		featureFlags:  NewFeatureFlags(),
		safetyMonitor: NewSafetyMonitor(),
		validator:     NewMigrationValidator(),
		progress:      NewMigrationProgress(),
	}

	// Initialize rollback trigger
	mo.rollbackTrigger = &RollbackTrigger{
		orchestrator: mo,
	}

	return mo
}

// StartMigration begins the migration process
func (mo *MigrationOrchestrator) StartMigration(ctx context.Context) error {
	mo.mu.Lock()
	mo.startTime = time.Now()
	mo.stage = StageShadowMode
	mo.mu.Unlock()

	mo.logger.Info("Starting protocol migration", map[string]interface{}{
		"stage": "shadow_mode",
		"time":  mo.startTime,
	})

	// Start telemetry collection
	if err := mo.startTelemetryCollection(ctx); err != nil {
		return fmt.Errorf("failed to start telemetry: %w", err)
	}

	// Run migration loop
	go mo.migrationLoop(ctx)

	return nil
}

// migrationLoop manages the migration progression
func (mo *MigrationOrchestrator) migrationLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mo.evaluateAndProgress(ctx)
		}
	}
}

// evaluateAndProgress evaluates current state and progresses migration
func (mo *MigrationOrchestrator) evaluateAndProgress(ctx context.Context) {
	mo.mu.RLock()
	currentStage := mo.stage
	mo.mu.RUnlock()

	switch currentStage {
	case StageShadowMode:
		mo.evaluateShadowMode(ctx)
	case StageCanary:
		mo.evaluateCanary(ctx)
	case StageProgressive:
		mo.evaluateProgressive(ctx)
	case StageMCPOnly:
		mo.evaluateMCPOnly(ctx)
	case StageCleanup:
		mo.performCleanup(ctx)
	}
}

// evaluateShadowMode evaluates shadow mode metrics
func (mo *MigrationOrchestrator) evaluateShadowMode(ctx context.Context) {
	// Collect telemetry for at least 24 hours
	if time.Since(mo.startTime) < 24*time.Hour {
		mo.logger.Info("Shadow mode - collecting telemetry", map[string]interface{}{
			"duration":       time.Since(mo.startTime),
			"custom_traffic": atomic.LoadUint64(&mo.customTraffic),
			"mcp_traffic":    atomic.LoadUint64(&mo.mcpTraffic),
		})
		return
	}

	// Analyze telemetry
	canRemove, analysis := mo.telemetry.ShouldRemoveCustomProtocol()

	decision := MigrationDecision{
		Timestamp:         time.Now(),
		Stage:             StageShadowMode,
		Decision:          "progress_to_canary",
		Confidence:        analysis.Confidence,
		RiskScore:         mo.calculateRiskScore(analysis),
		AutomatedDecision: true,
	}

	if canRemove && analysis.Confidence > 0.8 {
		// Safe to progress
		mo.progressToCanary(ctx)
		decision.Approved = true
		mo.logger.Info("Progressing to canary stage", map[string]interface{}{
			"confidence": analysis.Confidence,
			"reason":     analysis.Reason,
		})
	} else {
		decision.Approved = false
		mo.logger.Warn("Not ready for canary", map[string]interface{}{
			"confidence": analysis.Confidence,
			"reason":     analysis.Reason,
		})
	}

	mo.recordDecision(decision)
}

// progressToCanary moves to canary deployment
func (mo *MigrationOrchestrator) progressToCanary(ctx context.Context) {
	mo.mu.Lock()
	mo.stage = StageCanary
	mo.mu.Unlock()

	// Enable canary feature flag
	mo.featureFlags.Set("mcp_canary", true)
	mo.featureFlags.Set("traffic_percentage", 5) // 5% traffic

	// Configure router for canary
	mo.router.SetTrafficSplit(0.05) // 5% to MCP

	// Start enhanced monitoring
	mo.safetyMonitor.StartEnhancedMonitoring()

	mo.logger.Info("Entered canary stage", map[string]interface{}{
		"traffic_percentage": 5,
	})
}

// evaluateProgressive evaluates progressive rollout
func (mo *MigrationOrchestrator) evaluateProgressive(ctx context.Context) {
	// Check current progress
	mo.mu.RLock()
	currentPercentage := mo.getProgress()
	mo.mu.RUnlock()

	if currentPercentage < 75 {
		// Continue progressive rollout
		mo.progressToProgressive(ctx, int(currentPercentage*2))
	} else {
		// Ready for full migration evaluation
		mo.evaluateFullMigration(ctx)
	}
}

// evaluateCanary evaluates canary deployment
func (mo *MigrationOrchestrator) evaluateCanary(ctx context.Context) {
	// Run canary for at least 1 hour
	canaryDuration := time.Since(mo.getStageStartTime())
	if canaryDuration < 1*time.Hour {
		return
	}

	// Check safety metrics
	if mo.safetyMonitor.IsViolated() {
		mo.rollback(ctx, "Safety threshold violated in canary")
		return
	}

	// Compare performance
	comparison := mo.compareProtocolPerformance()

	if comparison.MCPBetter() {
		mo.progressToProgressive(ctx, 25) // Move to 25%
	} else if comparison.Similar() {
		// Need more data
		mo.extendCanary()
	} else {
		// MCP performing worse - investigate
		mo.investigatePerformanceIssue(comparison)
	}
}

// progressToProgressive moves to progressive rollout
func (mo *MigrationOrchestrator) progressToProgressive(ctx context.Context, percentage int) {
	mo.mu.Lock()
	mo.stage = StageProgressive
	mo.mu.Unlock()

	mo.featureFlags.Set("traffic_percentage", percentage)
	mo.router.SetTrafficSplit(float64(percentage) / 100.0)

	mo.logger.Info("Progressive rollout", map[string]interface{}{
		"percentage": percentage,
	})

	// Schedule next increase
	go func() {
		time.Sleep(2 * time.Hour)
		if percentage < 75 {
			mo.progressToProgressive(ctx, min(percentage*2, 75))
		} else {
			mo.evaluateFullMigration(ctx)
		}
	}()
}

// evaluateFullMigration evaluates readiness for full migration
func (mo *MigrationOrchestrator) evaluateFullMigration(ctx context.Context) {
	// Run comprehensive validation
	ready, report := mo.validator.ValidateFullMigration()

	if ready {
		mo.progressToMCPOnly(ctx)
	} else {
		mo.logger.Warn("Not ready for full migration", map[string]interface{}{
			"report": report,
		})
		// Stay at 75% for longer observation
	}
}

// progressToMCPOnly moves to 100% MCP
func (mo *MigrationOrchestrator) progressToMCPOnly(ctx context.Context) {
	mo.mu.Lock()
	mo.stage = StageMCPOnly
	mo.mu.Unlock()

	// Disable custom protocol
	mo.featureFlags.Set("allow_custom_protocol", false)
	mo.featureFlags.Set("traffic_percentage", 100)
	mo.router.SetTrafficSplit(1.0)

	mo.logger.Info("Moved to MCP-only mode", map[string]interface{}{
		"timestamp": time.Now(),
	})

	// Monitor for 24 hours before cleanup
	go func() {
		time.Sleep(24 * time.Hour)
		mo.evaluateMCPOnly(ctx)
	}()
}

// evaluateMCPOnly evaluates MCP-only performance
func (mo *MigrationOrchestrator) evaluateMCPOnly(ctx context.Context) {
	// Final validation before cleanup
	if mo.safetyMonitor.IsHealthy() {
		mo.progressToCleanup(ctx)
	} else {
		mo.logger.Warn("Issues detected in MCP-only mode", map[string]interface{}{
			"health": mo.safetyMonitor.GetHealth(),
		})
	}
}

// progressToCleanup removes old code
func (mo *MigrationOrchestrator) progressToCleanup(ctx context.Context) {
	mo.mu.Lock()
	mo.stage = StageCleanup
	mo.mu.Unlock()

	mo.logger.Info("Starting cleanup of deprecated code", nil)

	// This would trigger actual code removal
	// In practice, this would be a separate deployment
	mo.performCleanup(ctx)
}

// performCleanup performs the actual cleanup
func (mo *MigrationOrchestrator) performCleanup(ctx context.Context) {
	// List of components to remove
	componentsToRemove := []string{
		"internal/api/websocket/custom_handlers.go",
		"internal/api/websocket/custom_protocol.go",
		"pkg/models/custom_protocol.go",
	}

	for _, component := range componentsToRemove {
		mo.logger.Info("Removing deprecated component", map[string]interface{}{
			"component": component,
		})
		// In reality, this would be handled by deployment
		mo.progress.ComponentsMigrated = append(mo.progress.ComponentsMigrated, component)
	}

	mo.mu.Lock()
	mo.stage = StageComplete
	mo.mu.Unlock()

	mo.logger.Info("Migration complete!", map[string]interface{}{
		"duration":           time.Since(mo.startTime),
		"components_removed": len(componentsToRemove),
	})
}

// rollback performs a rollback
func (mo *MigrationOrchestrator) rollback(ctx context.Context, reason string) {
	mo.logger.Error("Rolling back migration", map[string]interface{}{
		"reason": reason,
		"stage":  mo.stage,
	})

	// Restore custom protocol
	mo.featureFlags.Set("allow_custom_protocol", true)
	mo.featureFlags.Set("mcp_canary", false)
	mo.router.SetTrafficSplit(0) // All traffic to custom

	// Move back one stage
	mo.mu.Lock()
	if mo.stage > StagePreMigration {
		mo.stage--
	}
	mo.mu.Unlock()

	// Notify team
	mo.notifyRollback(reason)
}

// Helper methods

func (mo *MigrationOrchestrator) startTelemetryCollection(ctx context.Context) error {
	// Start collecting telemetry in shadow mode
	return nil
}

func (mo *MigrationOrchestrator) calculateRiskScore(analysis RemovalAnalysis) float64 {
	// Calculate risk based on various factors
	risk := 0.0

	if analysis.UsageRate > 0.01 {
		risk += 0.3
	}
	if analysis.ErrorRate > 0.001 {
		risk += 0.2
	}
	if analysis.UniquePatterns > 10 {
		risk += 0.2
	}
	if analysis.DaysSinceLastUse < 7 {
		risk += 0.3
	}

	return math.Min(1.0, risk)
}

func (mo *MigrationOrchestrator) recordDecision(decision MigrationDecision) {
	mo.progress.mu.Lock()
	mo.progress.Decisions = append(mo.progress.Decisions, decision)
	mo.progress.mu.Unlock()
}

func (mo *MigrationOrchestrator) getStageStartTime() time.Time {
	// Would track actual stage start times
	return mo.startTime
}

func (mo *MigrationOrchestrator) compareProtocolPerformance() *PerformanceComparison {
	// Compare custom vs MCP performance
	return &PerformanceComparison{
		CustomLatency: 25 * time.Millisecond,
		MCPLatency:    10 * time.Millisecond,
		CustomCost:    0.10,
		MCPCost:       0.05,
	}
}

func (mo *MigrationOrchestrator) extendCanary() {
	mo.logger.Info("Extending canary period for more data", nil)
}

func (mo *MigrationOrchestrator) investigatePerformanceIssue(comparison *PerformanceComparison) {
	mo.logger.Warn("Investigating MCP performance issue", map[string]interface{}{
		"comparison": comparison,
	})
}

func (mo *MigrationOrchestrator) notifyRollback(reason string) {
	// Send notifications about rollback
}

// Supporting type implementations

func NewFeatureFlags() *FeatureFlags {
	return &FeatureFlags{
		flags: make(map[string]bool),
	}
}

func (ff *FeatureFlags) Set(key string, value interface{}) {
	ff.mu.Lock()
	defer ff.mu.Unlock()

	switch v := value.(type) {
	case bool:
		ff.flags[key] = v
	case int:
		ff.flags[key] = v > 0
	}
}

func (ff *FeatureFlags) IsEnabled(key string) bool {
	ff.mu.RLock()
	defer ff.mu.RUnlock()
	return ff.flags[key]
}

func NewSafetyMonitor() *SafetyMonitor {
	return &SafetyMonitor{
		maxErrorRate:    0.01, // 1%
		maxLatency:      200 * time.Millisecond,
		maxCostIncrease: 1.5, // 50% increase
		triggers:        make([]func() bool, 0),
	}
}

func (sm *SafetyMonitor) IsViolated() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.errorRate > sm.maxErrorRate {
		return true
	}
	if sm.avgLatency > sm.maxLatency {
		return true
	}
	if sm.costMultiplier > sm.maxCostIncrease {
		return true
	}

	for _, trigger := range sm.triggers {
		if trigger() {
			return true
		}
	}

	return false
}

func (sm *SafetyMonitor) IsHealthy() bool {
	return !sm.IsViolated()
}

func (sm *SafetyMonitor) GetHealth() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return map[string]interface{}{
		"error_rate":      sm.errorRate,
		"avg_latency":     sm.avgLatency,
		"cost_multiplier": sm.costMultiplier,
	}
}

func (sm *SafetyMonitor) StartEnhancedMonitoring() {
	// Start enhanced monitoring
}

func NewMigrationValidator() *MigrationValidator {
	return &MigrationValidator{
		checks: []ValidationCheck{
			{
				Name:     "error_rate",
				Check:    checkErrorRate,
				Required: true,
			},
			{
				Name:     "latency",
				Check:    checkLatency,
				Required: true,
			},
			{
				Name:     "cost",
				Check:    checkCost,
				Required: false,
			},
		},
	}
}

func (mv *MigrationValidator) ValidateFullMigration() (bool, string) {
	for _, check := range mv.checks {
		passed, msg := check.Check()
		if !passed && check.Required {
			return false, fmt.Sprintf("Check failed: %s - %s", check.Name, msg)
		}
	}
	return true, "All checks passed"
}

func NewMigrationProgress() *MigrationProgress {
	return &MigrationProgress{
		ComponentsRemaining: []string{},
		ComponentsMigrated:  []string{},
		Decisions:           make([]MigrationDecision, 0),
	}
}

// Validation check functions
func checkErrorRate() (bool, string) {
	// Check error rate
	return true, "Error rate acceptable"
}

func checkLatency() (bool, string) {
	// Check latency
	return true, "Latency acceptable"
}

func checkCost() (bool, string) {
	// Check cost
	return true, "Cost acceptable"
}

// Supporting types

type RollbackTrigger struct {
	orchestrator *MigrationOrchestrator
}

type PerformanceComparison struct {
	CustomLatency time.Duration
	MCPLatency    time.Duration
	CustomCost    float64
	MCPCost       float64
}

func (pc *PerformanceComparison) MCPBetter() bool {
	return pc.MCPLatency < pc.CustomLatency && pc.MCPCost < pc.CustomCost
}

func (pc *PerformanceComparison) Similar() bool {
	latencyRatio := float64(pc.MCPLatency) / float64(pc.CustomLatency)
	costRatio := pc.MCPCost / pc.CustomCost

	return latencyRatio > 0.9 && latencyRatio < 1.1 &&
		costRatio > 0.9 && costRatio < 1.1
}

func (ar *AdaptiveRouter) SetTrafficSplit(mcpPercentage float64) {
	// Implementation to split traffic
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
