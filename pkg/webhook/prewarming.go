package webhook

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/redis"
)

// PrewarmingStrategy defines the interface for pre-warming strategies
type PrewarmingStrategy interface {
	PredictContextsToWarm(ctx context.Context, currentContext *ContextData) ([]*PredictionResult, error)
	UpdateModel(ctx context.Context, accessPattern AccessPattern) error
	GetStrategyName() string
}

// PrewarmingConfig contains configuration for the pre-warming engine
type PrewarmingConfig struct {
	// Prediction settings
	MaxPredictions      int           // Maximum contexts to pre-warm
	ConfidenceThreshold float64       // Minimum confidence for pre-warming
	LookbackWindow      time.Duration // Time window for pattern analysis

	// Resource limits
	MaxMemoryUsage     int64         // Maximum memory for pre-warmed contexts
	MaxConcurrentWarms int           // Maximum concurrent pre-warming operations
	WarmingTimeout     time.Duration // Timeout for warming operations

	// Model settings
	ModelUpdateInterval time.Duration // How often to update prediction models
	MinDataPoints       int           // Minimum data points for predictions

	// Strategies
	EnabledStrategies []string // "sequential", "similarity", "temporal", "collaborative"
}

// DefaultPrewarmingConfig returns default configuration
func DefaultPrewarmingConfig() *PrewarmingConfig {
	return &PrewarmingConfig{
		MaxPredictions:      10,
		ConfidenceThreshold: 0.7,
		LookbackWindow:      24 * time.Hour,
		MaxMemoryUsage:      100 * 1024 * 1024, // 100MB
		MaxConcurrentWarms:  5,
		WarmingTimeout:      5 * time.Second,
		ModelUpdateInterval: 1 * time.Hour,
		MinDataPoints:       10,
		EnabledStrategies:   []string{"sequential", "similarity", "temporal"},
	}
}

// PrewarmingEngine predicts and pre-warms contexts
type PrewarmingEngine struct {
	config      *PrewarmingConfig
	lifecycle   *ContextLifecycleManager
	relevance   *RelevanceService
	redisClient *redis.StreamsClient
	logger      observability.Logger

	// Strategies
	strategies map[string]PrewarmingStrategy

	// Access pattern tracking
	accessLog *AccessLog

	// Resource tracking
	memoryUsage  int64
	warmingQueue chan *WarmingTask

	// Metrics
	metrics PrewarmingMetrics

	// Control
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// PrewarmingMetrics tracks pre-warming statistics
type PrewarmingMetrics struct {
	mu                    sync.RWMutex
	TotalPredictions      int64
	SuccessfulPredictions int64
	TotalPrewarmed        int64
	CacheHitRate          float64
	AverageWarmingTime    time.Duration
	MemorySaved           int64
}

// AccessPattern represents a context access pattern
type AccessPattern struct {
	UserID          string
	ContextID       string
	AccessTime      time.Time
	PreviousContext string
	SessionID       string
	AccessDuration  time.Duration
}

// AccessLog tracks access patterns
type AccessLog struct {
	mu       sync.RWMutex
	patterns []AccessPattern
	index    map[string][]int // contextID -> pattern indices
}

// PredictionResult represents a pre-warming prediction
type PredictionResult struct {
	ContextID  string  `json:"context_id"`
	Confidence float64 `json:"confidence"`
	Strategy   string  `json:"strategy"`
	Reason     string  `json:"reason"`
	Priority   int     `json:"priority"`
}

// WarmingTask represents a context warming task
type WarmingTask struct {
	ContextID   string
	TenantID    string
	Priority    int
	RequestedAt time.Time
	Strategy    string
}

// NewPrewarmingEngine creates a new pre-warming engine
func NewPrewarmingEngine(
	config *PrewarmingConfig,
	lifecycle *ContextLifecycleManager,
	relevance *RelevanceService,
	redisClient *redis.StreamsClient,
	logger observability.Logger,
) (*PrewarmingEngine, error) {
	if config == nil {
		config = DefaultPrewarmingConfig()
	}

	engine := &PrewarmingEngine{
		config:      config,
		lifecycle:   lifecycle,
		relevance:   relevance,
		redisClient: redisClient,
		logger:      logger,
		strategies:  make(map[string]PrewarmingStrategy),
		accessLog: &AccessLog{
			patterns: make([]AccessPattern, 0, 1000),
			index:    make(map[string][]int),
		},
		warmingQueue: make(chan *WarmingTask, config.MaxPredictions*2),
		stopCh:       make(chan struct{}),
	}

	// Initialize strategies
	if err := engine.initializeStrategies(); err != nil {
		return nil, fmt.Errorf("failed to initialize strategies: %w", err)
	}

	return engine, nil
}

// Start starts the pre-warming engine
func (e *PrewarmingEngine) Start() error {
	e.logger.Info("Starting pre-warming engine", map[string]interface{}{
		"strategies":      e.config.EnabledStrategies,
		"max_predictions": e.config.MaxPredictions,
	})

	// Start warming workers
	for i := 0; i < e.config.MaxConcurrentWarms; i++ {
		e.wg.Add(1)
		go e.warmingWorker(i)
	}

	// Start model update routine
	e.wg.Add(1)
	go e.modelUpdateLoop()

	// Start metrics reporter
	e.wg.Add(1)
	go e.metricsReporter()

	return nil
}

// Stop stops the pre-warming engine
func (e *PrewarmingEngine) Stop() {
	e.logger.Info("Stopping pre-warming engine", nil)
	close(e.stopCh)
	close(e.warmingQueue)
	e.wg.Wait()
}

// OnContextAccess handles context access events for pattern learning
func (e *PrewarmingEngine) OnContextAccess(pattern AccessPattern) {
	// Record access pattern
	e.accessLog.mu.Lock()
	e.accessLog.patterns = append(e.accessLog.patterns, pattern)
	e.accessLog.index[pattern.ContextID] = append(e.accessLog.index[pattern.ContextID], len(e.accessLog.patterns)-1)

	// Trim old patterns
	if len(e.accessLog.patterns) > 10000 {
		e.accessLog.patterns = e.accessLog.patterns[5000:]
		// Rebuild index
		e.accessLog.index = make(map[string][]int)
		for i, p := range e.accessLog.patterns {
			e.accessLog.index[p.ContextID] = append(e.accessLog.index[p.ContextID], i)
		}
	}
	e.accessLog.mu.Unlock()

	// Trigger predictions for next contexts
	go e.predictAndWarm(pattern)
}

// predictAndWarm predicts and pre-warms contexts
func (e *PrewarmingEngine) predictAndWarm(pattern AccessPattern) {
	ctx := context.Background()

	// Get current context
	currentContext, err := e.lifecycle.GetContext(ctx, pattern.UserID, pattern.ContextID)
	if err != nil {
		e.logger.Warn("Failed to get current context", map[string]interface{}{
			"context_id": pattern.ContextID,
			"error":      err.Error(),
		})
		return
	}

	// Collect predictions from all strategies
	allPredictions := make([]*PredictionResult, 0)
	for name, strategy := range e.strategies {
		predictions, err := strategy.PredictContextsToWarm(ctx, currentContext)
		if err != nil {
			e.logger.Warn("Strategy prediction failed", map[string]interface{}{
				"strategy": name,
				"error":    err.Error(),
			})
			continue
		}
		allPredictions = append(allPredictions, predictions...)
	}

	// Deduplicate and rank predictions
	rankedPredictions := e.rankPredictions(allPredictions)

	// Update metrics
	e.metrics.mu.Lock()
	e.metrics.TotalPredictions += int64(len(rankedPredictions))
	e.metrics.mu.Unlock()

	// Queue warming tasks for high-confidence predictions
	for _, pred := range rankedPredictions {
		if pred.Confidence >= e.config.ConfidenceThreshold {
			task := &WarmingTask{
				ContextID:   pred.ContextID,
				TenantID:    pattern.UserID,
				Priority:    pred.Priority,
				RequestedAt: time.Now(),
				Strategy:    pred.Strategy,
			}

			select {
			case e.warmingQueue <- task:
				// Queued successfully
			default:
				// Queue full, skip
				e.logger.Warn("Warming queue full, skipping context", map[string]interface{}{
					"context_id": pred.ContextID,
				})
			}
		}
	}
}

// rankPredictions deduplicates and ranks predictions
func (e *PrewarmingEngine) rankPredictions(predictions []*PredictionResult) []*PredictionResult {
	// Deduplicate by context ID, keeping highest confidence
	predMap := make(map[string]*PredictionResult)
	for _, pred := range predictions {
		if existing, ok := predMap[pred.ContextID]; !ok || pred.Confidence > existing.Confidence {
			predMap[pred.ContextID] = pred
		}
	}

	// Convert to slice
	ranked := make([]*PredictionResult, 0, len(predMap))
	for _, pred := range predMap {
		ranked = append(ranked, pred)
	}

	// Sort by confidence and priority
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Confidence != ranked[j].Confidence {
			return ranked[i].Confidence > ranked[j].Confidence
		}
		return ranked[i].Priority > ranked[j].Priority
	})

	// Limit to max predictions
	if len(ranked) > e.config.MaxPredictions {
		ranked = ranked[:e.config.MaxPredictions]
	}

	return ranked
}

// warmingWorker processes warming tasks
func (e *PrewarmingEngine) warmingWorker(id int) {
	defer e.wg.Done()

	for {
		select {
		case <-e.stopCh:
			return
		case task, ok := <-e.warmingQueue:
			if !ok {
				return
			}
			e.processWarmingTask(task)
		}
	}
}

// processWarmingTask warms a context
func (e *PrewarmingEngine) processWarmingTask(task *WarmingTask) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), e.config.WarmingTimeout)
	defer cancel()

	// Check if context is already in hot storage
	hotKey := fmt.Sprintf("context:hot:%s:%s", task.TenantID, task.ContextID)
	client := e.redisClient.GetClient()

	exists, err := client.Exists(ctx, hotKey).Result()
	if err == nil && exists > 0 {
		// Already in hot storage
		return
	}

	// Get context from warm or cold storage
	contextData, err := e.lifecycle.GetContext(ctx, task.TenantID, task.ContextID)
	if err != nil {
		e.logger.Warn("Failed to get context for warming", map[string]interface{}{
			"context_id": task.ContextID,
			"error":      err.Error(),
		})
		return
	}

	// Check memory usage
	contextSize := int64(contextData.Metadata.Size)
	if e.memoryUsage+contextSize > e.config.MaxMemoryUsage {
		e.logger.Warn("Memory limit reached, skipping pre-warming", map[string]interface{}{
			"context_id":    task.ContextID,
			"current_usage": e.memoryUsage,
			"context_size":  contextSize,
		})
		return
	}

	// Move to hot storage
	if contextData.Metadata.State != StateHot {
		// The context is now in hot storage from the GetContext call
		// Update memory tracking
		e.memoryUsage += contextSize

		// Update metrics
		e.metrics.mu.Lock()
		e.metrics.TotalPrewarmed++
		e.metrics.AverageWarmingTime = (e.metrics.AverageWarmingTime + time.Since(start)) / 2
		e.metrics.mu.Unlock()

		e.logger.Debug("Pre-warmed context", map[string]interface{}{
			"context_id":   task.ContextID,
			"strategy":     task.Strategy,
			"warming_time": time.Since(start),
		})
	}
}

// initializeStrategies initializes prediction strategies
func (e *PrewarmingEngine) initializeStrategies() error {
	for _, strategyName := range e.config.EnabledStrategies {
		switch strategyName {
		case "sequential":
			e.strategies[strategyName] = NewSequentialStrategy(e.accessLog, e.logger)
		case "similarity":
			e.strategies[strategyName] = NewSimilarityStrategy(e.relevance, e.logger)
		case "temporal":
			e.strategies[strategyName] = NewTemporalStrategy(e.accessLog, e.config.LookbackWindow, e.logger)
		case "collaborative":
			e.strategies[strategyName] = NewCollaborativeStrategy(e.accessLog, e.logger)
		default:
			e.logger.Warn("Unknown pre-warming strategy", map[string]interface{}{
				"strategy": strategyName,
			})
		}
	}

	if len(e.strategies) == 0 {
		return fmt.Errorf("no valid strategies enabled")
	}

	return nil
}

// modelUpdateLoop periodically updates prediction models
func (e *PrewarmingEngine) modelUpdateLoop() {
	defer e.wg.Done()

	ticker := time.NewTicker(e.config.ModelUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.updateModels()
		}
	}
}

// updateModels updates all strategy models
func (e *PrewarmingEngine) updateModels() {
	ctx := context.Background()

	// Get recent access patterns
	e.accessLog.mu.RLock()
	patterns := make([]AccessPattern, len(e.accessLog.patterns))
	copy(patterns, e.accessLog.patterns)
	e.accessLog.mu.RUnlock()

	// Update each strategy
	for name, strategy := range e.strategies {
		for _, pattern := range patterns {
			if err := strategy.UpdateModel(ctx, pattern); err != nil {
				e.logger.Warn("Failed to update strategy model", map[string]interface{}{
					"strategy": name,
					"error":    err.Error(),
				})
			}
		}
	}
}

// metricsReporter reports metrics periodically
func (e *PrewarmingEngine) metricsReporter() {
	defer e.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.reportMetrics()
		}
	}
}

// reportMetrics logs current metrics
func (e *PrewarmingEngine) reportMetrics() {
	metrics := e.GetMetrics()
	e.logger.Info("Pre-warming engine metrics", metrics)
}

// GetMetrics returns pre-warming engine metrics
func (e *PrewarmingEngine) GetMetrics() map[string]interface{} {
	e.metrics.mu.RLock()
	defer e.metrics.mu.RUnlock()

	hitRate := float64(0)
	if e.metrics.TotalPredictions > 0 {
		hitRate = float64(e.metrics.SuccessfulPredictions) / float64(e.metrics.TotalPredictions) * 100
	}

	return map[string]interface{}{
		"total_predictions":      e.metrics.TotalPredictions,
		"successful_predictions": e.metrics.SuccessfulPredictions,
		"prediction_hit_rate":    hitRate,
		"total_prewarmed":        e.metrics.TotalPrewarmed,
		"average_warming_time":   e.metrics.AverageWarmingTime,
		"current_memory_usage":   e.memoryUsage,
		"memory_saved":           e.metrics.MemorySaved,
		"enabled_strategies":     e.config.EnabledStrategies,
	}
}

// SequentialStrategy predicts based on sequential access patterns
type SequentialStrategy struct {
	accessLog   *AccessLog
	logger      observability.Logger
	transitions map[string]map[string]int // from -> to -> count
	mu          sync.RWMutex
}

// NewSequentialStrategy creates a new sequential strategy
func NewSequentialStrategy(accessLog *AccessLog, logger observability.Logger) *SequentialStrategy {
	return &SequentialStrategy{
		accessLog:   accessLog,
		logger:      logger,
		transitions: make(map[string]map[string]int),
	}
}

// PredictContextsToWarm predicts next contexts based on sequences
func (s *SequentialStrategy) PredictContextsToWarm(ctx context.Context, currentContext *ContextData) ([]*PredictionResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	predictions := make([]*PredictionResult, 0)

	// Get transition probabilities for current context
	if nextContexts, ok := s.transitions[currentContext.Metadata.ID]; ok {
		total := 0
		for _, count := range nextContexts {
			total += count
		}

		// Create predictions
		for nextID, count := range nextContexts {
			confidence := float64(count) / float64(total)
			predictions = append(predictions, &PredictionResult{
				ContextID:  nextID,
				Confidence: confidence,
				Strategy:   "sequential",
				Reason:     fmt.Sprintf("Followed %d times out of %d", count, total),
				Priority:   count,
			})
		}
	}

	// Sort by confidence
	sort.Slice(predictions, func(i, j int) bool {
		return predictions[i].Confidence > predictions[j].Confidence
	})

	// Return top predictions
	if len(predictions) > 5 {
		predictions = predictions[:5]
	}

	return predictions, nil
}

// UpdateModel updates the sequential model
func (s *SequentialStrategy) UpdateModel(ctx context.Context, pattern AccessPattern) error {
	if pattern.PreviousContext == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Update transition count
	if _, ok := s.transitions[pattern.PreviousContext]; !ok {
		s.transitions[pattern.PreviousContext] = make(map[string]int)
	}
	s.transitions[pattern.PreviousContext][pattern.ContextID]++

	return nil
}

// GetStrategyName returns the strategy name
func (s *SequentialStrategy) GetStrategyName() string {
	return "sequential"
}

// SimilarityStrategy predicts based on content similarity
type SimilarityStrategy struct {
	relevance *RelevanceService
	logger    observability.Logger
}

// NewSimilarityStrategy creates a new similarity strategy
func NewSimilarityStrategy(relevance *RelevanceService, logger observability.Logger) *SimilarityStrategy {
	return &SimilarityStrategy{
		relevance: relevance,
		logger:    logger,
	}
}

// PredictContextsToWarm predicts similar contexts to warm
func (s *SimilarityStrategy) PredictContextsToWarm(ctx context.Context, currentContext *ContextData) ([]*PredictionResult, error) {
	// This would use the relevance service to find similar contexts
	// For now, return empty predictions
	return []*PredictionResult{}, nil
}

// UpdateModel updates the similarity model (no-op for this strategy)
func (s *SimilarityStrategy) UpdateModel(ctx context.Context, pattern AccessPattern) error {
	return nil
}

// GetStrategyName returns the strategy name
func (s *SimilarityStrategy) GetStrategyName() string {
	return "similarity"
}

// TemporalStrategy predicts based on time patterns
type TemporalStrategy struct {
	accessLog      *AccessLog
	lookbackWindow time.Duration
	logger         observability.Logger
	hourlyPatterns map[int]map[string]int // hour -> contextID -> count
	mu             sync.RWMutex
}

// NewTemporalStrategy creates a new temporal strategy
func NewTemporalStrategy(accessLog *AccessLog, lookbackWindow time.Duration, logger observability.Logger) *TemporalStrategy {
	return &TemporalStrategy{
		accessLog:      accessLog,
		lookbackWindow: lookbackWindow,
		logger:         logger,
		hourlyPatterns: make(map[int]map[string]int),
	}
}

// PredictContextsToWarm predicts contexts based on time patterns
func (s *TemporalStrategy) PredictContextsToWarm(ctx context.Context, currentContext *ContextData) ([]*PredictionResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	currentHour := time.Now().Hour()
	predictions := make([]*PredictionResult, 0)

	// Look at patterns for current and next hour
	for h := currentHour; h <= currentHour+1; h++ {
		hour := h % 24
		if contexts, ok := s.hourlyPatterns[hour]; ok {
			total := 0
			for _, count := range contexts {
				total += count
			}

			for contextID, count := range contexts {
				confidence := float64(count) / float64(total) * 0.8 // Scale down temporal confidence
				predictions = append(predictions, &PredictionResult{
					ContextID:  contextID,
					Confidence: confidence,
					Strategy:   "temporal",
					Reason:     fmt.Sprintf("Frequently accessed at %d:00", hour),
					Priority:   count,
				})
			}
		}
	}

	return predictions, nil
}

// UpdateModel updates the temporal model
func (s *TemporalStrategy) UpdateModel(ctx context.Context, pattern AccessPattern) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hour := pattern.AccessTime.Hour()
	if _, ok := s.hourlyPatterns[hour]; !ok {
		s.hourlyPatterns[hour] = make(map[string]int)
	}
	s.hourlyPatterns[hour][pattern.ContextID]++

	return nil
}

// GetStrategyName returns the strategy name
func (s *TemporalStrategy) GetStrategyName() string {
	return "temporal"
}

// CollaborativeStrategy predicts based on similar user patterns
type CollaborativeStrategy struct {
	accessLog      *AccessLog
	logger         observability.Logger
	userSimilarity map[string]map[string]float64 // userID -> userID -> similarity
}

// NewCollaborativeStrategy creates a new collaborative strategy
func NewCollaborativeStrategy(accessLog *AccessLog, logger observability.Logger) *CollaborativeStrategy {
	return &CollaborativeStrategy{
		accessLog:      accessLog,
		logger:         logger,
		userSimilarity: make(map[string]map[string]float64),
	}
}

// PredictContextsToWarm predicts contexts based on similar users
func (s *CollaborativeStrategy) PredictContextsToWarm(ctx context.Context, currentContext *ContextData) ([]*PredictionResult, error) {
	// This would analyze similar user patterns
	// For now, return empty predictions
	return []*PredictionResult{}, nil
}

// UpdateModel updates the collaborative model
func (s *CollaborativeStrategy) UpdateModel(ctx context.Context, pattern AccessPattern) error {
	// This would update user similarity scores
	return nil
}

// GetStrategyName returns the strategy name
func (s *CollaborativeStrategy) GetStrategyName() string {
	return "collaborative"
}

// AnalyzePredictionAccuracy analyzes prediction accuracy
func (e *PrewarmingEngine) AnalyzePredictionAccuracy(windowSize time.Duration) *PredictionAccuracyReport {
	report := &PredictionAccuracyReport{
		WindowSize: windowSize,
		Strategies: make(map[string]*StrategyAccuracy),
	}

	// This would analyze historical predictions vs actual access patterns
	// For now, return placeholder report

	return report
}

// PredictionAccuracyReport represents prediction accuracy analysis
type PredictionAccuracyReport struct {
	WindowSize          time.Duration
	TotalPredictions    int64
	AccuratePredictions int64
	OverallAccuracy     float64
	Strategies          map[string]*StrategyAccuracy
}

// StrategyAccuracy represents accuracy for a specific strategy
type StrategyAccuracy struct {
	TotalPredictions    int64
	AccuratePredictions int64
	Accuracy            float64
	AverageConfidence   float64
}
