package intelligence

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/attribute"
)

// Core execution types

type ExecutionRequest struct {
	ToolID   uuid.UUID
	AgentID  uuid.UUID
	TenantID uuid.UUID
	Action   string
	Params   map[string]interface{}
	Mode     ExecutionMode
	Metadata map[string]interface{}
}

type ExecutionResponse struct {
	ExecutionID     uuid.UUID
	ToolResult      interface{}
	Intelligence    IntelligenceMetadata
	ContextID       uuid.UUID
	RelatedContexts []uuid.UUID
	Metrics         ExecutionMetrics
}

type ExecutionMetrics struct {
	ExecutionTimeMs      int64
	EmbeddingTimeMs      int64
	IntelligenceDeferred bool
	TotalTokens          int
	TotalCostUSD         float64
	Queued               bool
}

// Intelligence types

type IntelligenceMetadata struct {
	ContentType    ContentType
	Entities       []Entity
	Topics         []Topic
	Keywords       []string
	Summary        string
	Sentiment      SentimentAnalysis
	Language       string
	Classification DataClassification
}

type ContentType string

const (
	ContentTypeText          ContentType = "text"
	ContentTypeCode          ContentType = "code"
	ContentTypeJSON          ContentType = "json"
	ContentTypeHTML          ContentType = "html"
	ContentTypeMarkdown      ContentType = "markdown"
	ContentTypeAPI           ContentType = "api_response"
	ContentTypeDocumentation ContentType = "documentation"
	ContentTypeUnknown       ContentType = "unknown"
)

type Entity struct {
	Type       string // person, organization, location, etc.
	Value      string
	Confidence float64
	Metadata   map[string]interface{}
}

type Topic struct {
	Name     string
	Score    float64
	Keywords []string
}

type SentimentAnalysis struct {
	Polarity     float64 // -1 (negative) to 1 (positive)
	Subjectivity float64 // 0 (objective) to 1 (subjective)
	Confidence   float64
}

// Checkpoint and recovery types

type ExecutionCheckpoint struct {
	ID         uuid.UUID
	StartTime  time.Time
	Request    ExecutionRequest
	Stages     map[string]StageCheckpoint
	LastUpdate time.Time
}

type StageCheckpoint struct {
	Name       string
	Status     StageStatus
	StartTime  time.Time
	EndTime    *time.Time
	InputData  interface{}
	OutputData interface{}
	Error      error
}

type StageStatus string

const (
	StageStatusPending   StageStatus = "pending"
	StageStatusRunning   StageStatus = "running"
	StageStatusCompleted StageStatus = "completed"
	StageStatusFailed    StageStatus = "failed"
	StageStatusSkipped   StageStatus = "skipped"
)

type CompensationFunc func(context.Context) error

// Content analysis types

type ContentAnalysis struct {
	ContentType ContentType
	Metadata    *ContentMetadata
	Entities    []Entity
	Topics      []Topic
	Keywords    []string
	Summary     string
	Language    string
}

type ContentMetadata struct {
	Size        int
	LineCount   int
	WordCount   int
	CharCount   int
	HasCode     bool
	HasMarkdown bool
	HasJSON     bool
	HasHTML     bool
	HasPII      bool
	HasSecrets  bool
}

type IntelligenceResult struct {
	Metadata          IntelligenceMetadata
	EmbeddingID       *uuid.UUID
	RelatedContexts   []uuid.UUID
	SemanticLinks     []SemanticLink
	EmbeddingDuration time.Duration
	TokensUsed        int
	Cost              float64
}

type SemanticLink struct {
	TargetID     uuid.UUID
	Relationship string
	Confidence   float64
}

// Tool execution types

type ToolResult struct {
	Data     interface{}
	Duration time.Duration
	Error    error
}

// Cost control types

type CostCheckRequest struct {
	TenantID        uuid.UUID
	ToolExecution   bool
	ToolType        string
	EmbeddingTokens int
	EmbeddingModel  string
	AnalysisTokens  int
	AnalysisModel   string
	StorageMB       float64
}

type CostCheckResponse struct {
	Allowed       bool
	EstimatedCost decimal.Decimal
	CurrentSpend  decimal.Decimal
	DailyLimit    decimal.Decimal
	Remaining     decimal.Decimal
	PercentUsed   float64
	ShouldWarn    bool
	BlockReason   string
	GracePeriod   bool
}

type CostBreakdown struct {
	ExecutionID   uuid.UUID
	TenantID      uuid.UUID
	Timestamp     time.Time
	ToolCost      decimal.Decimal
	EmbeddingCost decimal.Decimal
	AnalysisCost  decimal.Decimal
	StorageCost   decimal.Decimal
	Discount      decimal.Decimal
	Total         decimal.Decimal
}

type TenantBudget struct {
	TenantID        uuid.UUID
	DailyLimit      decimal.Decimal
	MonthlyLimit    decimal.Decimal
	WarningPercent  float64
	DiscountPercent float64
	GracePeriod     time.Duration
}

type CostRateTable struct {
	ToolRates      map[string]decimal.Decimal
	EmbeddingRates map[string]decimal.Decimal
	AnalysisRates  map[string]decimal.Decimal
	StorageRate    decimal.Decimal
}

func NewCostRateTable() *CostRateTable {
	return &CostRateTable{
		ToolRates: map[string]decimal.Decimal{
			"api_call":     decimal.NewFromFloat(0.001),
			"web_scrape":   decimal.NewFromFloat(0.002),
			"code_execute": decimal.NewFromFloat(0.005),
		},
		EmbeddingRates: map[string]decimal.Decimal{
			"text-embedding-3-small":       decimal.NewFromFloat(0.00002),
			"text-embedding-3-large":       decimal.NewFromFloat(0.00013),
			"amazon.titan-embed-text-v2:0": decimal.NewFromFloat(0.00002),
		},
		AnalysisRates: map[string]decimal.Decimal{
			"gpt-4":    decimal.NewFromFloat(0.03),
			"gpt-3.5":  decimal.NewFromFloat(0.002),
			"claude-3": decimal.NewFromFloat(0.025),
		},
		StorageRate: decimal.NewFromFloat(0.0001), // per MB
	}
}

func (c *CostRateTable) GetToolRate(toolType string) decimal.Decimal {
	if rate, ok := c.ToolRates[toolType]; ok {
		return rate
	}
	return c.ToolRates["api_call"] // default
}

func (c *CostRateTable) GetEmbeddingRate(model string) decimal.Decimal {
	if rate, ok := c.EmbeddingRates[model]; ok {
		return rate
	}
	return decimal.NewFromFloat(0.00002) // default
}

func (c *CostRateTable) GetAnalysisRate(model string) decimal.Decimal {
	if rate, ok := c.AnalysisRates[model]; ok {
		return rate
	}
	return decimal.NewFromFloat(0.002) // default
}

func (c *CostRateTable) GetStorageRate() decimal.Decimal {
	return c.StorageRate
}

type UsageSummary struct {
	TenantID        uuid.UUID
	Period          TimePeriod
	DailySpend      decimal.Decimal
	MonthlySpend    decimal.Decimal
	DailyLimit      decimal.Decimal
	MonthlyLimit    decimal.Decimal
	Breakdown       *UsageBreakdown
	Trends          *UsageTrends
	TopOperations   []OperationUsage
	Recommendations []string
}

type TimePeriod struct {
	Start time.Time
	End   time.Time
}

type UsageBreakdown struct {
	ToolCosts      decimal.Decimal
	EmbeddingCosts decimal.Decimal
	AnalysisCosts  decimal.Decimal
	StorageCosts   decimal.Decimal
	TotalCost      decimal.Decimal
}

type UsageTrends struct {
	DailyGrowthRate   float64
	WeeklyGrowthRate  float64
	MonthlyGrowthRate float64
	PeakHour          int
	PeakDay           string
}

type OperationUsage struct {
	Operation   string
	Count       int
	TotalCost   decimal.Decimal
	AverageCost decimal.Decimal
}

type CostAlert struct {
	Level    AlertLevel
	TenantID uuid.UUID
	Message  string
	Spend    decimal.Decimal
	Limit    decimal.Decimal
}

type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// Configuration types

type CircuitBreakerConfig struct {
	MaxRequests uint32
	Interval    time.Duration
	Timeout     time.Duration
}

type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
}

type RateLimitConfig struct {
	RequestsPerSecond float64
	BurstSize         int
}

// Metrics types

type ExecutionMetricsCollector struct {
	totalExecutions  atomic.Uint64
	failedExecutions atomic.Uint64
	totalDuration    atomic.Value // time.Duration
	totalTokens      atomic.Uint64
	totalCost        atomic.Value // float64
}

func NewExecutionMetricsCollector(meter interface{}) *ExecutionMetricsCollector {
	collector := &ExecutionMetricsCollector{}
	collector.totalDuration.Store(time.Duration(0))
	collector.totalCost.Store(float64(0))
	return collector
}

func (c *ExecutionMetricsCollector) GetStats() map[string]interface{} {
	total := c.totalExecutions.Load()
	failed := c.failedExecutions.Load()

	return map[string]interface{}{
		"total":        total,
		"failed":       failed,
		"success_rate": float64(total-failed) / float64(total) * 100,
		"avg_duration": c.totalDuration.Load().(time.Duration) / time.Duration(total),
		"total_tokens": c.totalTokens.Load(),
		"total_cost":   c.totalCost.Load().(float64),
	}
}

type CostMetricsCollector struct {
	dailySpend   atomic.Value // decimal.Decimal
	monthlySpend atomic.Value // decimal.Decimal
	overBudget   atomic.Uint64
}

func NewCostMetricsCollector(meter interface{}) *CostMetricsCollector {
	collector := &CostMetricsCollector{}
	collector.dailySpend.Store(decimal.Zero)
	collector.monthlySpend.Store(decimal.Zero)
	return collector
}

func (c *CostMetricsCollector) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"daily_spend":   c.dailySpend.Load().(decimal.Decimal).String(),
		"monthly_spend": c.monthlySpend.Load().(decimal.Decimal).String(),
		"over_budget":   c.overBudget.Load(),
	}
}

type PerformanceMetricsCollector struct {
	cacheHits   atomic.Uint64
	cacheMisses atomic.Uint64
	gauges      sync.Map // string -> float64
}

func NewPerformanceMetricsCollector(meter interface{}) *PerformanceMetricsCollector {
	return &PerformanceMetricsCollector{}
}

func (c *PerformanceMetricsCollector) UpdateGauge(name string, value float64, labels ...attribute.KeyValue) {
	c.gauges.Store(name, value)
}

func (c *PerformanceMetricsCollector) GetStats() map[string]interface{} {
	hits := c.cacheHits.Load()
	misses := c.cacheMisses.Load()

	stats := map[string]interface{}{
		"cache_hits":   hits,
		"cache_misses": misses,
		"hit_rate":     float64(hits) / float64(hits+misses) * 100,
	}

	c.gauges.Range(func(key, value interface{}) bool {
		stats[key.(string)] = value
		return true
	})

	return stats
}

type SecurityMetricsCollector struct {
	piiDetected     atomic.Uint64
	secretsDetected atomic.Uint64
	blocked         atomic.Uint64
}

func NewSecurityMetricsCollector(meter interface{}) *SecurityMetricsCollector {
	return &SecurityMetricsCollector{}
}

func (c *SecurityMetricsCollector) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"pii_detected":     c.piiDetected.Load(),
		"secrets_detected": c.secretsDetected.Load(),
		"blocked":          c.blocked.Load(),
	}
}

// SLO monitoring types

type SLOTargets struct {
	LatencyP50   time.Duration
	LatencyP99   time.Duration
	ErrorRate    float64
	Availability float64
}

type SLOMonitor struct {
	targets    SLOTargets
	violations atomic.Uint64
	logger     observability.Logger
}

func NewSLOMonitor(targets SLOTargets, logger observability.Logger) *SLOMonitor {
	return &SLOMonitor{
		targets: targets,
		logger:  logger,
	}
}

func (m *SLOMonitor) CheckExecution(duration time.Duration, err error) {
	// Check latency SLO
	if duration > m.targets.LatencyP99 {
		m.violations.Add(1)
		m.logger.Warn("SLO violation: latency", map[string]interface{}{
			"duration": duration,
			"target":   m.targets.LatencyP99,
		})
	}

	// Check error rate would be done over a window
	if err != nil {
		m.logger.Warn("Execution error", map[string]interface{}{
			"error": err.Error(),
		})
	}
}

func (m *SLOMonitor) Start() {
	// Start monitoring loop
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		violations := m.violations.Load()
		if violations > 0 {
			m.logger.Warn("SLO violations detected", map[string]interface{}{
				"count": violations,
			})
			m.violations.Store(0) // Reset
		}
	}
}

// Alert types

type Alert struct {
	Severity AlertSeverity
	Title    string
	Message  string
	Labels   []attribute.KeyValue
}

type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

type AlertChannel interface {
	Send(alert Alert) error
}

type AlertManager struct {
	channels []AlertChannel
	logger   interface{}
}

func NewAlertManager(channels []AlertChannel, logger interface{}) *AlertManager {
	return &AlertManager{
		channels: channels,
		logger:   logger,
	}
}

func (m *AlertManager) SendAlert(alert Alert) {
	for _, channel := range m.channels {
		if err := channel.Send(alert); err != nil {
			// Log error but continue with other channels
			_ = err
		}
	}
}

// Interface definitions

type ToolExecutor interface {
	Execute(ctx context.Context, toolID uuid.UUID, action string, params map[string]interface{}) (*ToolResult, error)
}

type ContentAnalyzer interface {
	Analyze(ctx context.Context, content []byte) (*ContentAnalysis, error)
}

// EmbeddingService is a wrapper interface for the embedding service
type EmbeddingService interface {
	GenerateEmbedding(ctx context.Context, content string, metadata map[string]interface{}) (*uuid.UUID, error)
}

type SemanticGraphService interface {
	AddNode(ctx context.Context, nodeID uuid.UUID, metadata map[string]interface{}) error
	CreateRelationship(ctx context.Context, source, target uuid.UUID, relationship string) error
	FindRelated(ctx context.Context, nodeID uuid.UUID, maxDistance int) ([]uuid.UUID, error)
}

type CacheService interface {
	Get(ctx context.Context, key string) (interface{}, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

type EventStore interface {
	PublishEvent(ctx context.Context, event Event) error
	Subscribe(ctx context.Context, eventType string) (<-chan Event, error)
}

type Event struct {
	ID          uuid.UUID
	Type        string
	ExecutionID uuid.UUID
	Timestamp   time.Time
	Data        interface{}
}

const (
	EventExecutionQueued   = "execution.queued"
	EventExecutionStarted  = "execution.started"
	EventExecutionComplete = "execution.complete"
	EventExecutionFailed   = "execution.failed"
)

type CostRepository interface {
	StoreCost(ctx context.Context, record CostRecord, breakdown *CostBreakdown) error
	GetCostBreakdown(ctx context.Context, executionID uuid.UUID) (*CostBreakdown, error)
	GetUsageBreakdown(ctx context.Context, tenantID uuid.UUID, period TimePeriod) (*UsageBreakdown, error)
	GetTenantBudget(ctx context.Context, tenantID uuid.UUID) (*TenantBudget, error)
	GetAllTenantBudgets(ctx context.Context) ([]*TenantBudget, error)
	IsInGracePeriod(ctx context.Context, tenantID uuid.UUID) (bool, error)
	StoreAlert(ctx context.Context, alert CostAlert) error
}

// Logger is an alias for the observability.Logger interface
type Logger = observability.Logger

type AuditLogger interface {
	LogSecurityEvent(ctx context.Context, event SecurityAuditEvent)
}

// Helper functions

func timeNow() time.Time {
	return time.Now()
}

// sha256Hash computes SHA256 hash of data
// Reserved for future security features
// func sha256Hash(data string) string {
// 	// Implementation would use crypto/sha256
// 	return ""
// }
