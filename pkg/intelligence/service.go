package intelligence

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"github.com/sony/gobreaker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"
)

// ExecutionMode defines how the execution should be processed
type ExecutionMode string

const (
	ModeSync   ExecutionMode = "sync"   // Wait for everything
	ModeAsync  ExecutionMode = "async"  // Return immediately, process in background
	ModeHybrid ExecutionMode = "hybrid" // Return tool result, async intelligence
)

// ResilientExecutionService provides fault-tolerant tool execution with intelligence
type ResilientExecutionService struct {
	// Core services
	toolExecutor     ToolExecutor
	contentAnalyzer  ContentAnalyzer
	embeddingService EmbeddingService
	securityLayer    *SecurityLayer
	semanticGraph    SemanticGraphService
	costController   *CostController

	// Persistence
	db         *sql.DB
	cache      CacheService
	eventStore EventStore

	// Resilience
	circuitBreaker *gobreaker.CircuitBreaker
	rateLimiter    *rate.Limiter
	semaphore      *semaphore.Weighted
	retryPolicy    backoff.BackOff

	// Observability
	tracer  trace.Tracer
	meter   metric.Meter
	logger  observability.Logger
	metrics observability.MetricsClient

	// Configuration
	config *ServiceConfig
	slo    *SLOConfig

	// State management
	checkpoints   sync.Map // map[uuid.UUID]*ExecutionCheckpoint
	compensations sync.Map // map[uuid.UUID][]CompensationFunc
}

// ServiceConfig contains all configuration for the service
type ServiceConfig struct {
	// Execution modes
	DefaultMode         ExecutionMode
	EnableAsyncFallback bool

	// Performance
	MaxConcurrency int64
	TimeoutSeconds int
	CacheEnabled   bool
	CacheTTL       time.Duration

	// Resilience
	CircuitBreakerConfig CircuitBreakerConfig
	RetryConfig          RetryConfig
	RateLimitConfig      RateLimitConfig

	// Security
	SecurityConfig SecurityConfig

	// Cost
	CostThresholdUSD float64
	DailyBudgetUSD   float64
}

// SLOConfig defines service level objectives
type SLOConfig struct {
	P50LatencyMs    float64 // 200ms
	P99LatencyMs    float64 // 500ms
	ErrorRatePct    float64 // 0.1%
	AvailabilityPct float64 // 99.9%
}

// NewResilientExecutionService creates a production-ready execution service
func NewResilientExecutionService(config *ServiceConfig, deps ServiceDependencies) (*ResilientExecutionService, error) {
	// Initialize tracer
	tracer := otel.Tracer("intelligence.execution")

	// Initialize meter
	meter := otel.Meter("intelligence.execution")

	// Create circuit breaker
	cbSettings := gobreaker.Settings{
		Name:        "execution-service",
		MaxRequests: uint32(config.CircuitBreakerConfig.MaxRequests),
		Interval:    config.CircuitBreakerConfig.Interval,
		Timeout:     config.CircuitBreakerConfig.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			deps.Logger.Info("Circuit breaker state changed", map[string]interface{}{
				"name": name,
				"from": from,
				"to":   to,
			})
		},
	}

	cb := gobreaker.NewCircuitBreaker(cbSettings)

	// Create rate limiter
	rateLimiter := rate.NewLimiter(
		rate.Limit(config.RateLimitConfig.RequestsPerSecond),
		config.RateLimitConfig.BurstSize,
	)

	// Create semaphore for concurrency control
	sem := semaphore.NewWeighted(config.MaxConcurrency)

	// Create retry policy
	retryPolicy := backoff.NewExponentialBackOff()
	retryPolicy.MaxElapsedTime = 30 * time.Second

	// Create security layer
	securityLayer := NewSecurityLayer(config.SecurityConfig)

	// Create cost controller
	costConfig := CostControlConfig{
		GlobalDailyLimit:   config.DailyBudgetUSD,
		GlobalMonthlyLimit: config.DailyBudgetUSD * 30, // Approximate monthly
		WarningThreshold:   0.8,
		CriticalThreshold:  0.95,
	}
	costDeps := CostControlDependencies{
		DB:         deps.DB,
		Repository: nil, // Would be passed in from deps
		Logger:     deps.Logger,
	}
	costController, _ := NewCostController(costConfig, costDeps)

	return &ResilientExecutionService{
		toolExecutor:     deps.ToolExecutor,
		contentAnalyzer:  deps.ContentAnalyzer,
		embeddingService: deps.EmbeddingService,
		securityLayer:    securityLayer,
		semanticGraph:    deps.SemanticGraph,
		costController:   costController,
		db:               deps.DB,
		cache:            deps.Cache,
		eventStore:       deps.EventStore,
		circuitBreaker:   cb,
		rateLimiter:      rateLimiter,
		semaphore:        sem,
		retryPolicy:      retryPolicy,
		tracer:           tracer,
		meter:            meter,
		logger:           deps.Logger,
		metrics:          deps.MetricsClient,
		config:           config,
		slo: &SLOConfig{
			P50LatencyMs:    200,
			P99LatencyMs:    500,
			ErrorRatePct:    0.1,
			AvailabilityPct: 99.9,
		},
	}, nil
}

// Execute performs tool execution with intelligence processing
func (s *ResilientExecutionService) Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResponse, error) {
	// Start root span
	ctx, span := s.tracer.Start(ctx, "intelligence.execute",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("execution.mode", string(req.Mode)),
			attribute.String("tool.id", req.ToolID.String()),
			attribute.String("agent.id", req.AgentID.String()),
			attribute.String("tenant.id", req.TenantID.String()),
		),
	)
	defer span.End()

	// Record start time for metrics
	start := time.Now()

	// Create execution ID
	execID := uuid.New()
	span.SetAttributes(attribute.String("execution.id", execID.String()))

	// Apply timeout
	timeout := time.Duration(s.config.TimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Check rate limit
	if err := s.rateLimiter.Wait(ctx); err != nil {
		if s.metrics != nil {
			s.metrics.IncrementCounter("rate_limit_exceeded", 1)
		}
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	// Acquire semaphore for concurrency control
	if err := s.semaphore.Acquire(ctx, 1); err != nil {
		if s.metrics != nil {
			s.metrics.IncrementCounter("concurrency_limit_exceeded", 1)
		}
		return nil, fmt.Errorf("concurrency limit exceeded: %w", err)
	}
	defer s.semaphore.Release(1)

	// Create execution checkpoint
	checkpoint := &ExecutionCheckpoint{
		ID:        execID,
		StartTime: start,
		Request:   req,
		Stages:    make(map[string]StageCheckpoint),
	}
	s.checkpoints.Store(execID, checkpoint)
	defer s.checkpoints.Delete(execID)

	// Initialize compensation stack
	var compensations []CompensationFunc
	s.compensations.Store(execID, compensations)
	defer s.compensations.Delete(execID)

	// Execute with circuit breaker
	result, err := s.circuitBreaker.Execute(func() (interface{}, error) {
		// Choose execution mode
		switch req.Mode {
		case ModeAsync:
			return s.executeAsync(ctx, execID, req, checkpoint)
		case ModeHybrid:
			return s.executeHybrid(ctx, execID, req, checkpoint)
		default:
			return s.executeSync(ctx, execID, req, checkpoint)
		}
	})

	// Record metrics
	duration := time.Since(start)
	s.recordExecutionMetrics(duration, req, err)

	// Check SLO violations
	if duration > time.Duration(s.slo.P99LatencyMs)*time.Millisecond {
		if s.metrics != nil {
			s.metrics.RecordHistogram("slo_violation_duration_ms", float64(duration.Milliseconds()), map[string]string{
				"violation_type": "latency",
			})
		}
		s.logger.Warn("SLO violation: latency", map[string]interface{}{
			"duration_ms": duration.Milliseconds(),
			"slo_ms":      s.slo.P99LatencyMs,
		})
	}

	if err != nil {
		// Execute compensations on failure
		s.executeCompensations(ctx, execID)
		span.RecordError(err)
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	return result.(*ExecutionResponse), nil
}

// executeSync performs synchronous execution with all intelligence processing
func (s *ResilientExecutionService) executeSync(
	ctx context.Context,
	execID uuid.UUID,
	req ExecutionRequest,
	checkpoint *ExecutionCheckpoint,
) (*ExecutionResponse, error) {
	ctx, span := s.tracer.Start(ctx, "execute.sync")
	defer span.End()

	// Stage 1: Security checks
	if err := s.executeSecurityStage(ctx, execID, req, checkpoint); err != nil {
		return nil, fmt.Errorf("security check failed: %w", err)
	}

	// Stage 2: Cost pre-check
	if err := s.executeCostCheckStage(ctx, execID, req, checkpoint); err != nil {
		return nil, fmt.Errorf("cost check failed: %w", err)
	}

	// Stage 3: Tool execution
	toolResult, err := s.executeToolStage(ctx, execID, req, checkpoint)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	// Stage 4: Content analysis
	analysis, err := s.executeAnalysisStage(ctx, execID, toolResult, checkpoint)
	if err != nil {
		// Don't fail on analysis errors - log and continue
		s.logger.Error("Content analysis failed", map[string]interface{}{
			"error": err.Error(),
		})
		analysis = &ContentAnalysis{
			ContentType: ContentTypeUnknown,
			Metadata:    &ContentMetadata{},
		}
	}

	// Stage 5: Intelligence processing
	intelligence, err := s.executeIntelligenceStage(ctx, execID, toolResult, analysis, checkpoint)
	if err != nil {
		// Don't fail on intelligence errors - log and continue
		s.logger.Error("Intelligence processing failed", map[string]interface{}{
			"error": err.Error(),
		})
		intelligence = &IntelligenceResult{
			Metadata: IntelligenceMetadata{
				ContentType: analysis.ContentType,
			},
		}
	}

	// Stage 6: Semantic graph update
	contextID, err := s.executeSemanticStage(ctx, execID, intelligence, checkpoint)
	if err != nil {
		// Don't fail on semantic errors - log and continue
		s.logger.Error("Semantic graph update failed", map[string]interface{}{
			"error": err.Error(),
		})
		contextID = uuid.New()
	}

	// Stage 7: Persistence
	if err := s.executePersistenceStage(ctx, execID, req, toolResult, intelligence, contextID, checkpoint); err != nil {
		// Log but don't fail
		s.logger.Error("Persistence failed", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Build response
	return &ExecutionResponse{
		ExecutionID:     execID,
		ToolResult:      toolResult.Data,
		Intelligence:    intelligence.Metadata,
		ContextID:       contextID,
		RelatedContexts: intelligence.RelatedContexts,
		Metrics: ExecutionMetrics{
			ExecutionTimeMs: toolResult.Duration.Milliseconds(),
			EmbeddingTimeMs: intelligence.EmbeddingDuration.Milliseconds(),
			TotalTokens:     intelligence.TokensUsed,
			TotalCostUSD:    intelligence.Cost,
		},
	}, nil
}

// executeAsync performs asynchronous execution
func (s *ResilientExecutionService) executeAsync(
	ctx context.Context,
	execID uuid.UUID,
	req ExecutionRequest,
	checkpoint *ExecutionCheckpoint,
) (*ExecutionResponse, error) {
	// Queue for background processing
	if err := s.eventStore.PublishEvent(ctx, Event{
		ID:          uuid.New(),
		Type:        EventExecutionQueued,
		ExecutionID: execID,
		Timestamp:   time.Now(),
		Data:        req,
	}); err != nil {
		return nil, fmt.Errorf("failed to queue execution: %w", err)
	}

	// Start background processing
	go func() {
		bgCtx := context.Background()
		bgCtx, cancel := context.WithTimeout(bgCtx, 5*time.Minute)
		defer cancel()

		if _, err := s.executeSync(bgCtx, execID, req, checkpoint); err != nil {
			s.logger.Error("Async execution failed", map[string]interface{}{
				"execution_id": execID.String(),
				"error":        err.Error(),
			})
		}
	}()

	// Return immediately with execution ID
	return &ExecutionResponse{
		ExecutionID: execID,
		Metrics: ExecutionMetrics{
			Queued: true,
		},
	}, nil
}

// executeHybrid performs hybrid execution (sync tool, async intelligence)
func (s *ResilientExecutionService) executeHybrid(
	ctx context.Context,
	execID uuid.UUID,
	req ExecutionRequest,
	checkpoint *ExecutionCheckpoint,
) (*ExecutionResponse, error) {
	// Execute tool synchronously
	toolResult, err := s.executeToolStage(ctx, execID, req, checkpoint)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	// Queue intelligence processing
	go func() {
		bgCtx := context.Background()
		bgCtx, cancel := context.WithTimeout(bgCtx, 2*time.Minute)
		defer cancel()

		// Run remaining stages
		analysis, _ := s.executeAnalysisStage(bgCtx, execID, toolResult, checkpoint)
		intelligence, _ := s.executeIntelligenceStage(bgCtx, execID, toolResult, analysis, checkpoint)
		contextID, _ := s.executeSemanticStage(bgCtx, execID, intelligence, checkpoint)
		_ = s.executePersistenceStage(bgCtx, execID, req, toolResult, intelligence, contextID, checkpoint)
	}()

	// Return tool result immediately
	return &ExecutionResponse{
		ExecutionID: execID,
		ToolResult:  toolResult.Data,
		Metrics: ExecutionMetrics{
			ExecutionTimeMs:      toolResult.Duration.Milliseconds(),
			IntelligenceDeferred: true,
		},
	}, nil
}

// recordExecutionMetrics records detailed execution metrics
func (s *ResilientExecutionService) recordExecutionMetrics(duration time.Duration, req ExecutionRequest, err error) {
	labels := map[string]string{
		"tool_id": req.ToolID.String(),
		"action":  req.Action,
		"mode":    string(req.Mode),
		"success": fmt.Sprintf("%t", err == nil),
	}

	if s.metrics != nil {
		s.metrics.RecordHistogram("execution_duration_ms", float64(duration.Milliseconds()), labels)
	}

	if s.metrics != nil {
		if err != nil {
			s.metrics.IncrementCounter("execution_errors", 1)
		} else {
			s.metrics.IncrementCounter("execution_success", 1)
		}
	}
}

// executeCompensations runs all registered compensation functions
func (s *ResilientExecutionService) executeCompensations(ctx context.Context, execID uuid.UUID) {
	if val, ok := s.compensations.Load(execID); ok {
		compensations := val.([]CompensationFunc)

		// Execute in reverse order
		for i := len(compensations) - 1; i >= 0; i-- {
			if err := compensations[i](ctx); err != nil {
				s.logger.Error("Compensation failed", map[string]interface{}{
					"execution_id": execID.String(),
					"index":        i,
					"error":        err.Error(),
				})
			}
		}
	}
}
