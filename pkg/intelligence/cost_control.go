package intelligence

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// CostController manages cost tracking and budget enforcement
type CostController struct {
	// Budget tracking
	budgets      sync.Map // tenant_id -> *TenantBudget
	dailySpend   sync.Map // tenant_id:date -> decimal
	monthlySpend sync.Map // tenant_id:month -> decimal

	// Cost rates
	costRates *CostRateTable

	// Thresholds and limits
	globalLimit    decimal.Decimal
	warningPercent float64

	// Database
	db         *sql.DB
	repository CostRepository

	// Metrics
	totalSpend      atomic.Value // decimal.Decimal
	blockedRequests uint64

	// Configuration
	config CostControlConfig
	logger observability.Logger

	// Alert channels
	alertChan chan CostAlert

	// Cache
	costCache sync.Map // execution_id -> CostBreakdown
}

// CostControlConfig contains cost control configuration
type CostControlConfig struct {
	// Global limits
	GlobalDailyLimit   float64
	GlobalMonthlyLimit float64

	// Alert thresholds
	WarningThreshold  float64 // 0.8 = alert at 80% of budget
	CriticalThreshold float64 // 0.95 = critical at 95%

	// Rate limits
	EnableRateLimiting bool
	CostPerMinute      float64 // Max cost per minute per tenant

	// Tracking
	TrackingGranularity string // "execution", "hourly", "daily"
	RetentionDays       int

	// Enforcement
	StrictEnforcement  bool // Block when budget exceeded
	GracePeriodMinutes int  // Allow temporary overages
}

// NewCostController creates a new cost controller
func NewCostController(config CostControlConfig, deps CostControlDependencies) (*CostController, error) {
	controller := &CostController{
		costRates:      NewCostRateTable(),
		globalLimit:    decimal.NewFromFloat(config.GlobalDailyLimit),
		warningPercent: config.WarningThreshold,
		db:             deps.DB,
		repository:     deps.Repository,
		config:         config,
		logger:         deps.Logger,
		alertChan:      make(chan CostAlert, 100),
	}

	// Initialize total spend
	controller.totalSpend.Store(decimal.Zero)

	// Load tenant budgets
	if err := controller.loadTenantBudgets(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to load tenant budgets: %w", err)
	}

	// Start background workers
	controller.startWorkers()

	return controller, nil
}

// CheckBudget checks if an operation can proceed within budget
func (c *CostController) CheckBudget(ctx context.Context, req CostCheckRequest) (*CostCheckResponse, error) {
	// Estimate cost
	estimatedCost := c.estimateCost(req)

	// Get tenant budget
	budget, err := c.getTenantBudget(req.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant budget: %w", err)
	}

	// Get current spend
	currentSpend := c.getCurrentSpend(req.TenantID)

	// Calculate remaining budget
	remaining := budget.DailyLimit.Sub(currentSpend)

	// Check if operation would exceed budget
	wouldExceed := remaining.LessThan(estimatedCost)

	// Check warning threshold
	percentUsedDecimal, _ := currentSpend.Div(budget.DailyLimit).Float64()
	percentUsed := percentUsedDecimal
	shouldWarn := percentUsed >= c.config.WarningThreshold

	response := &CostCheckResponse{
		Allowed:       !wouldExceed || !c.config.StrictEnforcement,
		EstimatedCost: estimatedCost,
		CurrentSpend:  currentSpend,
		DailyLimit:    budget.DailyLimit,
		Remaining:     remaining,
		PercentUsed:   percentUsed * 100,
		ShouldWarn:    shouldWarn,
	}

	// Send alert if needed
	if shouldWarn {
		c.sendBudgetAlert(req.TenantID, percentUsed, currentSpend, budget.DailyLimit)
	}

	// Block if strict enforcement and would exceed
	if wouldExceed && c.config.StrictEnforcement {
		atomic.AddUint64(&c.blockedRequests, 1)
		response.BlockReason = "Daily budget would be exceeded"

		// Check grace period
		if c.isInGracePeriod(req.TenantID) {
			response.Allowed = true
			response.GracePeriod = true
		}
	}

	return response, nil
}

// RecordCost records actual cost after execution
func (c *CostController) RecordCost(ctx context.Context, record CostRecord) error {
	// Calculate cost breakdown
	breakdown := c.calculateCostBreakdown(record)

	// Update spend tracking
	c.updateSpendTracking(record.TenantID, breakdown.Total)

	// Store in database
	if err := c.repository.StoreCost(ctx, record, breakdown); err != nil {
		c.logger.Error("Failed to store cost record", map[string]interface{}{"error": err.Error()})
		// Don't fail the operation, just log
	}

	// Cache the breakdown
	c.costCache.Store(record.ExecutionID, breakdown)

	// Update global spend
	current := c.totalSpend.Load().(decimal.Decimal)
	c.totalSpend.Store(current.Add(breakdown.Total))

	// Check if we need to send alerts
	c.checkCostAlerts(record.TenantID, breakdown.Total)

	// Update metrics
	c.recordCostMetrics(record, breakdown)

	return nil
}

// GetCostBreakdown returns detailed cost breakdown for an execution
func (c *CostController) GetCostBreakdown(ctx context.Context, executionID uuid.UUID) (*CostBreakdown, error) {
	// Check cache
	if cached, ok := c.costCache.Load(executionID); ok {
		return cached.(*CostBreakdown), nil
	}

	// Load from database
	breakdown, err := c.repository.GetCostBreakdown(ctx, executionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cost breakdown: %w", err)
	}

	// Cache for future use
	c.costCache.Store(executionID, breakdown)

	return breakdown, nil
}

// GetTenantUsage returns usage summary for a tenant
func (c *CostController) GetTenantUsage(ctx context.Context, tenantID uuid.UUID, period TimePeriod) (*UsageSummary, error) {
	// Get spend data
	dailySpend := c.getDailySpend(tenantID, period)
	monthlySpend := c.getMonthlySpend(tenantID, period)

	// Get budget info
	budget, err := c.getTenantBudget(tenantID)
	if err != nil {
		return nil, err
	}

	// Get detailed breakdown from database
	breakdown, err := c.repository.GetUsageBreakdown(ctx, tenantID, period)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage breakdown: %w", err)
	}

	// Calculate trends
	trends := c.calculateUsageTrends(breakdown)

	return &UsageSummary{
		TenantID:        tenantID,
		Period:          period,
		DailySpend:      dailySpend,
		MonthlySpend:    monthlySpend,
		DailyLimit:      budget.DailyLimit,
		MonthlyLimit:    budget.MonthlyLimit,
		Breakdown:       breakdown,
		Trends:          trends,
		TopOperations:   c.getTopOperations(breakdown),
		Recommendations: c.generateRecommendations(breakdown, trends),
	}, nil
}

// estimateCost estimates the cost of an operation
func (c *CostController) estimateCost(req CostCheckRequest) decimal.Decimal {
	cost := decimal.Zero

	// Tool execution cost
	if req.ToolExecution {
		toolRate := c.costRates.GetToolRate(req.ToolType)
		cost = cost.Add(toolRate)
	}

	// Embedding cost
	if req.EmbeddingTokens > 0 {
		embeddingRate := c.costRates.GetEmbeddingRate(req.EmbeddingModel)
		tokenCost := embeddingRate.Mul(decimal.NewFromInt(int64(req.EmbeddingTokens))).Div(decimal.NewFromInt(1000))
		cost = cost.Add(tokenCost)
	}

	// Analysis cost
	if req.AnalysisTokens > 0 {
		analysisRate := c.costRates.GetAnalysisRate(req.AnalysisModel)
		tokenCost := analysisRate.Mul(decimal.NewFromInt(int64(req.AnalysisTokens))).Div(decimal.NewFromInt(1000))
		cost = cost.Add(tokenCost)
	}

	// Storage cost (per MB)
	if req.StorageMB > 0 {
		storageRate := c.costRates.GetStorageRate()
		storageCost := storageRate.Mul(decimal.NewFromFloat(req.StorageMB))
		cost = cost.Add(storageCost)
	}

	// Apply tenant discount if applicable
	if budget, err := c.getTenantBudget(req.TenantID); err == nil && budget.DiscountPercent > 0 {
		discount := decimal.NewFromFloat(1 - budget.DiscountPercent/100)
		cost = cost.Mul(discount)
	}

	return cost
}

// calculateCostBreakdown calculates detailed cost breakdown
func (c *CostController) calculateCostBreakdown(record CostRecord) *CostBreakdown {
	breakdown := &CostBreakdown{
		ExecutionID: record.ExecutionID,
		TenantID:    record.TenantID,
		Timestamp:   time.Now(),
	}

	// Tool execution cost
	if record.ToolExecution {
		breakdown.ToolCost = c.costRates.GetToolRate(record.ToolType)
	}

	// Embedding cost
	if record.EmbeddingTokens > 0 {
		rate := c.costRates.GetEmbeddingRate(record.EmbeddingModel)
		breakdown.EmbeddingCost = rate.Mul(decimal.NewFromInt(int64(record.EmbeddingTokens))).Div(decimal.NewFromInt(1000))
	}

	// Analysis cost
	if record.AnalysisTokens > 0 {
		rate := c.costRates.GetAnalysisRate(record.AnalysisModel)
		breakdown.AnalysisCost = rate.Mul(decimal.NewFromInt(int64(record.AnalysisTokens))).Div(decimal.NewFromInt(1000))
	}

	// Storage cost
	if record.StorageMB > 0 {
		rate := c.costRates.GetStorageRate()
		breakdown.StorageCost = rate.Mul(decimal.NewFromFloat(record.StorageMB))
	}

	// Calculate total
	breakdown.Total = breakdown.ToolCost.
		Add(breakdown.EmbeddingCost).
		Add(breakdown.AnalysisCost).
		Add(breakdown.StorageCost)

	// Apply discount
	if budget, err := c.getTenantBudget(record.TenantID); err == nil && budget.DiscountPercent > 0 {
		breakdown.Discount = breakdown.Total.Mul(decimal.NewFromFloat(budget.DiscountPercent / 100))
		breakdown.Total = breakdown.Total.Sub(breakdown.Discount)
	}

	return breakdown
}

// updateSpendTracking updates spend tracking maps
func (c *CostController) updateSpendTracking(tenantID uuid.UUID, amount decimal.Decimal) {
	// Update daily spend
	dailyKey := fmt.Sprintf("%s:%s", tenantID.String(), time.Now().Format("2006-01-02"))
	if current, ok := c.dailySpend.Load(dailyKey); ok {
		c.dailySpend.Store(dailyKey, current.(decimal.Decimal).Add(amount))
	} else {
		c.dailySpend.Store(dailyKey, amount)
	}

	// Update monthly spend
	monthlyKey := fmt.Sprintf("%s:%s", tenantID.String(), time.Now().Format("2006-01"))
	if current, ok := c.monthlySpend.Load(monthlyKey); ok {
		c.monthlySpend.Store(monthlyKey, current.(decimal.Decimal).Add(amount))
	} else {
		c.monthlySpend.Store(monthlyKey, amount)
	}
}

// getCurrentSpend gets current daily spend for tenant
func (c *CostController) getCurrentSpend(tenantID uuid.UUID) decimal.Decimal {
	dailyKey := fmt.Sprintf("%s:%s", tenantID.String(), time.Now().Format("2006-01-02"))
	if spend, ok := c.dailySpend.Load(dailyKey); ok {
		return spend.(decimal.Decimal)
	}
	return decimal.Zero
}

// getTenantBudget gets or creates tenant budget
func (c *CostController) getTenantBudget(tenantID uuid.UUID) (*TenantBudget, error) {
	// Check cache
	if budget, ok := c.budgets.Load(tenantID); ok {
		return budget.(*TenantBudget), nil
	}

	// Load from database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	budget, err := c.repository.GetTenantBudget(ctx, tenantID)
	if err != nil {
		// Use default budget
		budget = &TenantBudget{
			TenantID:       tenantID,
			DailyLimit:     decimal.NewFromFloat(100.0),  // $100 default
			MonthlyLimit:   decimal.NewFromFloat(3000.0), // $3000 default
			WarningPercent: 80.0,
		}
	}

	// Cache it
	c.budgets.Store(tenantID, budget)

	return budget, nil
}

// loadTenantBudgets loads all tenant budgets from database
func (c *CostController) loadTenantBudgets(ctx context.Context) error {
	budgets, err := c.repository.GetAllTenantBudgets(ctx)
	if err != nil {
		return err
	}

	for _, budget := range budgets {
		c.budgets.Store(budget.TenantID, budget)
	}

	c.logger.Info("Loaded tenant budgets", map[string]interface{}{"count": len(budgets)})
	return nil
}

// checkCostAlerts checks if cost alerts need to be sent
func (c *CostController) checkCostAlerts(tenantID uuid.UUID, amount decimal.Decimal) {
	budget, err := c.getTenantBudget(tenantID)
	if err != nil {
		return
	}

	currentSpend := c.getCurrentSpend(tenantID)
	percentUsedDecimal, _ := currentSpend.Div(budget.DailyLimit).Float64()
	percentUsed := percentUsedDecimal

	// Check thresholds
	if percentUsed >= c.config.CriticalThreshold {
		c.sendCostAlert(CostAlert{
			Level:    AlertLevelCritical,
			TenantID: tenantID,
			Message:  fmt.Sprintf("Critical: Daily budget at %.1f%%", percentUsed*100),
			Spend:    currentSpend,
			Limit:    budget.DailyLimit,
		})
	} else if percentUsed >= c.config.WarningThreshold {
		c.sendCostAlert(CostAlert{
			Level:    AlertLevelWarning,
			TenantID: tenantID,
			Message:  fmt.Sprintf("Warning: Daily budget at %.1f%%", percentUsed*100),
			Spend:    currentSpend,
			Limit:    budget.DailyLimit,
		})
	}
}

// sendCostAlert sends a cost alert
func (c *CostController) sendCostAlert(alert CostAlert) {
	select {
	case c.alertChan <- alert:
	default:
		// Channel full, log and drop
		c.logger.Warn("Alert channel full, dropping alert", map[string]interface{}{"tenant_id": alert.TenantID.String()})
	}
}

// sendBudgetAlert sends budget warning alert
func (c *CostController) sendBudgetAlert(tenantID uuid.UUID, percentUsed float64, spend, limit decimal.Decimal) {
	alert := CostAlert{
		Level:    AlertLevelWarning,
		TenantID: tenantID,
		Message:  fmt.Sprintf("Budget usage at %.1f%%", percentUsed*100),
		Spend:    spend,
		Limit:    limit,
	}

	c.sendCostAlert(alert)
}

// isInGracePeriod checks if tenant is in grace period
func (c *CostController) isInGracePeriod(tenantID uuid.UUID) bool {
	// Check if grace period is configured
	if c.config.GracePeriodMinutes <= 0 {
		return false
	}

	// Check database for grace period status
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	inGrace, err := c.repository.IsInGracePeriod(ctx, tenantID)
	if err != nil {
		c.logger.Error("Failed to check grace period", map[string]interface{}{"error": err.Error()})
		return false
	}

	return inGrace
}

// startWorkers starts background workers
func (c *CostController) startWorkers() {
	// Alert processor
	go c.alertProcessor()

	// Budget refresh worker
	go c.budgetRefreshWorker()

	// Cleanup worker
	go c.cleanupWorker()
}

// alertProcessor processes cost alerts
func (c *CostController) alertProcessor() {
	for alert := range c.alertChan {
		// Process alert (send to notification service, etc.)
		c.logger.Warn("Cost alert", map[string]interface{}{
			"level":     string(alert.Level),
			"tenant_id": alert.TenantID.String(),
			"message":   alert.Message,
			"spend":     alert.Spend.String(),
			"limit":     alert.Limit.String(),
		})

		// Store alert in database
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := c.repository.StoreAlert(ctx, alert); err != nil {
			c.logger.Error("Failed to store alert", map[string]interface{}{"error": err.Error()})
		}
		cancel()
	}
}

// budgetRefreshWorker refreshes tenant budgets periodically
func (c *CostController) budgetRefreshWorker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := c.loadTenantBudgets(ctx); err != nil {
			c.logger.Error("Failed to refresh budgets", map[string]interface{}{"error": err.Error()})
		}
		cancel()
	}
}

// cleanupWorker cleans up old data
func (c *CostController) cleanupWorker() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		// Clean old daily spend entries
		cutoff := time.Now().AddDate(0, 0, -c.config.RetentionDays)
		c.cleanOldSpendData(cutoff)

		// Clean old cost cache entries
		c.cleanCostCache()
	}
}

// cleanOldSpendData removes old spend tracking data
func (c *CostController) cleanOldSpendData(cutoff time.Time) {
	cutoffStr := cutoff.Format("2006-01-02")

	// Clean daily spend
	c.dailySpend.Range(func(key, value interface{}) bool {
		keyStr := key.(string)
		// Extract date from key (format: "uuid:date")
		if len(keyStr) > 36 {
			dateStr := keyStr[37:]
			if dateStr < cutoffStr {
				c.dailySpend.Delete(key)
			}
		}
		return true
	})

	// Clean monthly spend older than 12 months
	monthCutoff := time.Now().AddDate(0, -12, 0).Format("2006-01")
	c.monthlySpend.Range(func(key, value interface{}) bool {
		keyStr := key.(string)
		if len(keyStr) > 36 {
			monthStr := keyStr[37:]
			if monthStr < monthCutoff {
				c.monthlySpend.Delete(key)
			}
		}
		return true
	})
}

// cleanCostCache removes old cached cost breakdowns
func (c *CostController) cleanCostCache() {
	// Keep only last 1000 entries (simple LRU-like behavior)
	count := 0
	c.costCache.Range(func(key, value interface{}) bool {
		count++
		if count > 1000 {
			c.costCache.Delete(key)
		}
		return true
	})
}

// recordCostMetrics records cost metrics
func (c *CostController) recordCostMetrics(record CostRecord, breakdown *CostBreakdown) {
	// This would integrate with the observability layer
	c.logger.Debug("Cost recorded", map[string]interface{}{
		"execution_id": record.ExecutionID.String(),
		"tenant_id":    record.TenantID.String(),
		"total":        breakdown.Total.String(),
	})
}
