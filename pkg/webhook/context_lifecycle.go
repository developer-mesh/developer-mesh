package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/redis"
	"github.com/google/uuid"
	redisclient "github.com/redis/go-redis/v9"
)

// ContextState represents the storage tier of a context
type ContextState string

const (
	StateHot  ContextState = "hot"
	StateWarm ContextState = "warm"
	StateCold ContextState = "cold"

	// Redis sorted set keys for transition tracking
	transitionSetHotToWarm  = "context:transitions:hot_to_warm"
	transitionSetWarmToCold = "context:transitions:warm_to_cold"

	// Redis sorted set for all contexts by tenant
	contextsByTenantSet = "context:index:by_tenant:%s"
	allContextsSet      = "context:index:all"
)

// ContextMetadata contains metadata about a context item
type ContextMetadata struct {
	ID               string       `json:"id"`
	TenantID         string       `json:"tenant_id"`
	State            ContextState `json:"state"`
	CreatedAt        time.Time    `json:"created_at"`
	LastAccessed     time.Time    `json:"last_accessed"`
	AccessCount      int          `json:"access_count"`
	Importance       float64      `json:"importance"` // 0.0 to 1.0
	Size             int64        `json:"size"`
	CompressionRatio float64      `json:"compression_ratio,omitempty"`
	Tags             []string     `json:"tags"`
	SourceType       string       `json:"source_type"` // webhook, manual, system
	SourceID         string       `json:"source_id"`   // webhook event ID, etc.
}

// ContextData represents the actual context data
type ContextData struct {
	Metadata *ContextMetadata       `json:"metadata"`
	Data     map[string]interface{} `json:"data"`
	Summary  string                 `json:"summary,omitempty"`
}

// LifecycleConfig contains configuration for context lifecycle management
type LifecycleConfig struct {
	HotDuration  time.Duration // Time in hot storage (default: 2 hours)
	WarmDuration time.Duration // Time in warm storage (default: 22 hours)

	// Importance thresholds for lifecycle transitions
	HotImportanceThreshold  float64 // Keep in hot if importance > threshold
	WarmImportanceThreshold float64 // Keep in warm if importance > threshold

	// Access count thresholds
	HotAccessThreshold  int // Keep in hot if accessed more than N times
	WarmAccessThreshold int // Keep in warm if accessed more than N times

	// Storage settings
	EnableCompression bool
	CompressionLevel  int // 1-9, higher = better compression

	// Transition settings
	TransitionBatchSize int
	TransitionInterval  time.Duration
}

// DefaultLifecycleConfig returns default lifecycle configuration
func DefaultLifecycleConfig() *LifecycleConfig {
	return &LifecycleConfig{
		HotDuration:             2 * time.Hour,
		WarmDuration:            22 * time.Hour,
		HotImportanceThreshold:  0.8,
		WarmImportanceThreshold: 0.5,
		HotAccessThreshold:      5,
		WarmAccessThreshold:     2,
		EnableCompression:       true,
		CompressionLevel:        6,
		TransitionBatchSize:     100,
		TransitionInterval:      5 * time.Minute,
	}
}

// ContextLifecycleManager manages the lifecycle of contexts
type ContextLifecycleManager struct {
	config         *LifecycleConfig
	redisClient    *redis.StreamsClient
	storageBackend StorageBackend
	compression    CompressionService
	logger         observability.Logger

	// Lifecycle workers
	transitionStop chan struct{}
	wg             sync.WaitGroup

	// Circuit breaker for cold storage
	coldStorageBreaker *redis.CircuitBreaker

	// Batch processing
	batchProcessor *BatchProcessor
	batchStop      chan struct{}

	// Metrics
	metrics         LifecycleMetrics
	metricsRecorder *ContextLifecycleMetrics
}

// LifecycleMetrics tracks lifecycle statistics
type LifecycleMetrics struct {
	mu                sync.RWMutex
	HotContexts       int64
	WarmContexts      int64
	ColdContexts      int64
	TotalTransitions  int64
	CompressionSaved  int64 // Bytes saved through compression
	AverageAccessTime map[ContextState]time.Duration
}

// StorageBackend defines the interface for cold storage
type StorageBackend interface {
	Store(ctx context.Context, key string, data []byte) error
	Retrieve(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
}

// CompressionService defines the interface for compression
type CompressionService interface {
	Compress(data interface{}) ([]byte, error)
	CompressWithSemantics(data []byte) ([]byte, float64, error)
	Decompress(data []byte) ([]byte, error)
}

// NewContextLifecycleManager creates a new lifecycle manager
func NewContextLifecycleManager(
	config *LifecycleConfig,
	redisClient *redis.StreamsClient,
	storageBackend StorageBackend,
	compression CompressionService,
	logger observability.Logger,
) *ContextLifecycleManager {
	if config == nil {
		config = DefaultLifecycleConfig()
	}

	manager := &ContextLifecycleManager{
		config:         config,
		redisClient:    redisClient,
		storageBackend: storageBackend,
		compression:    compression,
		logger:         logger,
		transitionStop: make(chan struct{}),
		batchStop:      make(chan struct{}),
		metrics: LifecycleMetrics{
			AverageAccessTime: make(map[ContextState]time.Duration),
		},
		metricsRecorder: NewContextLifecycleMetrics(nil),
	}

	// Initialize circuit breaker for cold storage
	manager.coldStorageBreaker = redis.NewCircuitBreaker(
		&redis.CircuitBreakerConfig{
			FailureThreshold:  3,
			SuccessThreshold:  2,
			Timeout:           30 * time.Second,
			MaxTimeout:        2 * time.Minute,
			TimeoutMultiplier: 1.5,
		},
		logger,
	)

	// Initialize batch processor
	manager.batchProcessor = NewBatchProcessor(
		manager,
		100,            // Batch size
		10*time.Second, // Flush interval
	)

	// Start lifecycle workers
	manager.startWorkers()

	return manager
}

// StoreContext stores a new context in hot storage
func (m *ContextLifecycleManager) StoreContext(ctx context.Context, tenantID string, data map[string]interface{}, metadata *ContextMetadata) error {
	if metadata == nil {
		metadata = &ContextMetadata{
			ID:        uuid.New().String(),
			TenantID:  tenantID,
			State:     StateHot,
			CreatedAt: time.Now(),
		}
	}

	// Set initial metadata
	metadata.LastAccessed = time.Now()
	metadata.State = StateHot

	// Calculate size
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal context data: %w", err)
	}
	metadata.Size = int64(len(jsonData))

	// Create context data
	contextData := &ContextData{
		Metadata: metadata,
		Data:     data,
	}

	// Store in hot storage (Redis)
	hotKey := fmt.Sprintf("context:hot:%s:%s", tenantID, metadata.ID)

	contextJSON, err := json.Marshal(contextData)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	client := m.redisClient.GetClient()
	if err := client.Set(ctx, hotKey, contextJSON, m.config.HotDuration).Err(); err != nil {
		return fmt.Errorf("failed to store in hot storage: %w", err)
	}

	// Store metadata separately for efficient queries
	metaKey := fmt.Sprintf("context:meta:%s:%s", tenantID, metadata.ID)
	metaJSON, _ := json.Marshal(metadata)
	client.Set(ctx, metaKey, metaJSON, 0) // No expiration for metadata

	// Add to transition tracking sorted set
	transitionTime := time.Now().Add(m.config.HotDuration).Unix()
	transitionKey := fmt.Sprintf("%s:%s", tenantID, metadata.ID)
	client.ZAdd(ctx, transitionSetHotToWarm, redisclient.Z{
		Score:  float64(transitionTime),
		Member: transitionKey,
	})

	// Add to context index sorted sets for efficient searching
	contextIndexKey := fmt.Sprintf("%s:%s", tenantID, metadata.ID)
	createdTime := metadata.CreatedAt.Unix()

	// Add to tenant-specific index
	tenantSetKey := fmt.Sprintf(contextsByTenantSet, tenantID)
	client.ZAdd(ctx, tenantSetKey, redisclient.Z{
		Score:  float64(createdTime),
		Member: contextIndexKey,
	})

	// Add to global index
	client.ZAdd(ctx, allContextsSet, redisclient.Z{
		Score:  float64(createdTime),
		Member: contextIndexKey,
	})

	// Update metrics
	m.metrics.mu.Lock()
	m.metrics.HotContexts++
	m.metrics.mu.Unlock()

	m.logger.Debug("Stored context in hot storage", map[string]interface{}{
		"context_id": metadata.ID,
		"tenant_id":  tenantID,
		"size":       metadata.Size,
		"importance": metadata.Importance,
	})

	return nil
}

// GetContext retrieves a context from any storage tier
func (m *ContextLifecycleManager) GetContext(ctx context.Context, tenantID, contextID string) (*ContextData, error) {
	start := time.Now()

	// Try hot storage first
	hotKey := fmt.Sprintf("context:hot:%s:%s", tenantID, contextID)
	client := m.redisClient.GetClient()

	hotData, err := client.Get(ctx, hotKey).Bytes()
	if err == nil {
		// Found in hot storage
		var contextData ContextData
		if err := json.Unmarshal(hotData, &contextData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal hot context: %w", err)
		}

		m.updateAccessMetrics(StateHot, time.Since(start))
		m.updateAccessCount(ctx, tenantID, contextID)
		return &contextData, nil
	}

	// Try warm storage
	warmKey := fmt.Sprintf("context:warm:%s:%s", tenantID, contextID)
	warmData, err := client.Get(ctx, warmKey).Bytes()
	if err == nil {
		// Found in warm storage - decompress
		decompressed, err := m.compression.Decompress(warmData)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress warm context: %w", err)
		}

		var contextData ContextData
		if err := json.Unmarshal(decompressed, &contextData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal warm context: %w", err)
		}

		m.updateAccessMetrics(StateWarm, time.Since(start))
		m.updateAccessCount(ctx, tenantID, contextID)

		// Consider promoting back to hot if frequently accessed
		if contextData.Metadata.AccessCount > m.config.HotAccessThreshold {
			go m.promoteToHot(context.Background(), tenantID, contextID, &contextData)
		}

		return &contextData, nil
	}

	// Try cold storage
	return m.retrieveFromCold(ctx, tenantID, contextID)
}

// TransitionToWarm transitions a context from hot to warm storage
func (m *ContextLifecycleManager) TransitionToWarm(ctx context.Context, tenantID, contextID string) error {
	// Retrieve from hot storage
	hotKey := fmt.Sprintf("context:hot:%s:%s", tenantID, contextID)
	client := m.redisClient.GetClient()

	hotData, err := client.Get(ctx, hotKey).Bytes()
	if err != nil {
		return fmt.Errorf("context not found in hot storage: %w", err)
	}

	var contextData ContextData
	if err := json.Unmarshal(hotData, &contextData); err != nil {
		return fmt.Errorf("failed to unmarshal context: %w", err)
	}

	// Check if should skip transition based on importance or access
	if contextData.Metadata.Importance > m.config.HotImportanceThreshold ||
		contextData.Metadata.AccessCount > m.config.HotAccessThreshold {
		// Extend hot storage TTL
		client.Expire(ctx, hotKey, m.config.HotDuration)
		return nil
	}

	// Compress for warm storage
	compressed, ratio, err := m.compression.CompressWithSemantics(hotData)
	if err != nil {
		return fmt.Errorf("failed to compress context: %w", err)
	}

	// Update metadata
	contextData.Metadata.State = StateWarm
	contextData.Metadata.CompressionRatio = ratio

	// Store in warm tier
	warmKey := fmt.Sprintf("context:warm:%s:%s", tenantID, contextID)
	if err := client.Set(ctx, warmKey, compressed, m.config.WarmDuration).Err(); err != nil {
		return fmt.Errorf("failed to store in warm storage: %w", err)
	}

	// Update metadata
	metaKey := fmt.Sprintf("context:meta:%s:%s", tenantID, contextID)
	metaJSON, _ := json.Marshal(contextData.Metadata)
	client.Set(ctx, metaKey, metaJSON, 0)

	// Remove from hot storage
	client.Del(ctx, hotKey)

	// Update metrics
	m.metrics.mu.Lock()
	m.metrics.HotContexts--
	m.metrics.WarmContexts++
	m.metrics.CompressionSaved += int64(len(hotData)) - int64(len(compressed))
	m.metrics.mu.Unlock()

	m.logger.Debug("Transitioned context to warm storage", map[string]interface{}{
		"context_id":        contextID,
		"tenant_id":         tenantID,
		"compression_ratio": ratio,
		"space_saved":       len(hotData) - len(compressed),
	})

	return nil
}

// TransitionToCold transitions a context from warm to cold storage
func (m *ContextLifecycleManager) TransitionToCold(ctx context.Context, tenantID, contextID string) error {
	// This would implement the full cold storage transition
	// Including AI summarization and S3 archival
	// For now, we'll implement a simplified version

	warmKey := fmt.Sprintf("context:warm:%s:%s", tenantID, contextID)
	client := m.redisClient.GetClient()

	warmData, err := client.Get(ctx, warmKey).Bytes()
	if err != nil {
		return fmt.Errorf("context not found in warm storage: %w", err)
	}

	// Archive to cold storage
	archiveKey := fmt.Sprintf("contexts/%s/%s/%s.json.gz",
		tenantID,
		time.Now().Format("2006/01/02"),
		contextID,
	)

	if err := m.storageBackend.Store(ctx, archiveKey, warmData); err != nil {
		return fmt.Errorf("failed to archive to cold storage: %w", err)
	}

	// Update metadata
	metaKey := fmt.Sprintf("context:meta:%s:%s", tenantID, contextID)
	var metadata ContextMetadata
	if metaData, err := client.Get(ctx, metaKey).Bytes(); err == nil {
		_ = json.Unmarshal(metaData, &metadata)
		metadata.State = StateCold
		metaJSON, _ := json.Marshal(metadata)
		client.Set(ctx, metaKey, metaJSON, 0)
	}

	// Remove from warm storage
	client.Del(ctx, warmKey)

	// Update metrics
	m.metrics.mu.Lock()
	m.metrics.WarmContexts--
	m.metrics.ColdContexts++
	m.metrics.mu.Unlock()

	return nil
}

// startWorkers starts the background workers for lifecycle management
func (m *ContextLifecycleManager) startWorkers() {
	// Transition worker
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.transitionWorker()
	}()

	// Metrics update worker
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.metricsUpdateWorker()
	}()
}

// transitionWorker handles automatic lifecycle transitions
func (m *ContextLifecycleManager) transitionWorker() {
	ticker := time.NewTicker(m.config.TransitionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.processTransitions()
		case <-m.transitionStop:
			return
		}
	}
}

// metricsUpdateWorker periodically updates storage tier metrics
func (m *ContextLifecycleManager) metricsUpdateWorker() {
	ticker := time.NewTicker(30 * time.Second) // Update every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.updateStorageTierMetrics()
		case <-m.transitionStop:
			return
		}
	}
}

// updateStorageTierMetrics updates the storage tier distribution metrics
func (m *ContextLifecycleManager) updateStorageTierMetrics() {
	m.metrics.mu.RLock()
	hotCount := m.metrics.HotContexts
	warmCount := m.metrics.WarmContexts
	coldCount := m.metrics.ColdContexts
	m.metrics.mu.RUnlock()

	if m.metricsRecorder != nil {
		m.metricsRecorder.UpdateStorageTierCounts(hotCount, warmCount, coldCount)
	}
}

// processTransitions processes pending lifecycle transitions
func (m *ContextLifecycleManager) processTransitions() {
	ctx := context.Background()
	client := m.redisClient.GetClient()

	now := time.Now().Unix()

	// Process hot -> warm transitions
	m.processBatchTransitions(ctx, client, transitionSetHotToWarm, now, m.transitionHotToWarm)

	// Process warm -> cold transitions
	m.processBatchTransitions(ctx, client, transitionSetWarmToCold, now, m.transitionWarmToCold)
}

// processBatchTransitions processes a batch of transitions from a sorted set
func (m *ContextLifecycleManager) processBatchTransitions(
	ctx context.Context,
	client interface{},
	setKey string,
	currentTime int64,
	transitionFunc func(context.Context, string, string) error,
) {
	redisClient := m.redisClient.GetClient()

	// Get contexts ready for transition (score <= current time)
	results, err := redisClient.ZRangeByScoreWithScores(ctx, setKey, &redisclient.ZRangeBy{
		Min:   "0",
		Max:   fmt.Sprintf("%d", currentTime),
		Count: int64(m.config.TransitionBatchSize),
	}).Result()

	if err != nil {
		m.logger.Error("Failed to query transition set", map[string]interface{}{
			"set_key": setKey,
			"error":   err.Error(),
		})
		return
	}

	if len(results) == 0 {
		return
	}

	// Process transitions in parallel with concurrency limit
	sem := make(chan struct{}, 10) // Max 10 concurrent transitions
	var wg sync.WaitGroup

	for _, result := range results {
		// Parse tenant ID and context ID
		parts := strings.Split(result.Member.(string), ":")
		if len(parts) != 2 {
			continue
		}

		tenantID := parts[0]
		contextID := parts[1]

		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		go func(tID, cID string) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			// Process transition with timeout
			transitionCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			if err := transitionFunc(transitionCtx, tID, cID); err != nil {
				m.logger.Error("Failed to process transition", map[string]interface{}{
					"tenant_id":  tID,
					"context_id": cID,
					"set_key":    setKey,
					"error":      err.Error(),
				})
				// Don't remove from set on failure - will retry next time
			} else {
				// Remove from transition set on success
				redisClient.ZRem(ctx, setKey, fmt.Sprintf("%s:%s", tID, cID))
			}
		}(tenantID, contextID)
	}

	wg.Wait()
}

// transitionHotToWarm wraps TransitionToWarm with proper error handling
func (m *ContextLifecycleManager) transitionHotToWarm(ctx context.Context, tenantID, contextID string) error {
	// Acquire lock before transition
	lock, err := m.AcquireContextLock(ctx, tenantID, contextID)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer func() {
		if err := lock.Release(ctx); err != nil {
			m.logger.Warn("Failed to release lock in transitionHotToWarm", map[string]interface{}{
				"error":      err.Error(),
				"context_id": contextID,
				"tenant_id":  tenantID,
			})
		}
	}()

	return m.TransitionToWarm(ctx, tenantID, contextID)
}

// transitionWarmToCold wraps TransitionToCold with proper error handling
func (m *ContextLifecycleManager) transitionWarmToCold(ctx context.Context, tenantID, contextID string) error {
	// Acquire lock before transition
	lock, err := m.AcquireContextLock(ctx, tenantID, contextID)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer func() {
		if err := lock.Release(ctx); err != nil {
			m.logger.Warn("Failed to release lock in transitionWarmToCold", map[string]interface{}{
				"error":      err.Error(),
				"context_id": contextID,
				"tenant_id":  tenantID,
			})
		}
	}()

	return m.TransitionToCold(ctx, tenantID, contextID)
}

// updateAccessMetrics updates access time metrics
func (m *ContextLifecycleManager) updateAccessMetrics(state ContextState, duration time.Duration) {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()

	current := m.metrics.AverageAccessTime[state]
	// Simple moving average
	m.metrics.AverageAccessTime[state] = (current + duration) / 2
}

// updateAccessCount updates the access count for a context
func (m *ContextLifecycleManager) updateAccessCount(ctx context.Context, tenantID, contextID string) {
	metaKey := fmt.Sprintf("context:meta:%s:%s", tenantID, contextID)
	client := m.redisClient.GetClient()

	var metadata ContextMetadata
	if data, err := client.Get(ctx, metaKey).Bytes(); err == nil {
		if json.Unmarshal(data, &metadata) == nil {
			metadata.AccessCount++
			metadata.LastAccessed = time.Now()

			metaJSON, _ := json.Marshal(metadata)
			client.Set(ctx, metaKey, metaJSON, 0)
		}
	}
}

// promoteToHot promotes a context back to hot storage
func (m *ContextLifecycleManager) promoteToHot(ctx context.Context, tenantID, contextID string, contextData *ContextData) {
	// Use a separate context with timeout derived from parent
	promotionCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Acquire distributed lock
	lock, err := m.AcquireContextLock(promotionCtx, tenantID, contextID)
	if err != nil {
		m.logger.Error("Failed to acquire lock for promotion", map[string]interface{}{
			"context_id": contextID,
			"tenant_id":  tenantID,
			"error":      err.Error(),
		})
		return
	}
	defer func() {
		if err := lock.Release(promotionCtx); err != nil {
			m.logger.Warn("Failed to release promotion lock", map[string]interface{}{
				"context_id": contextID,
				"error":      err.Error(),
			})
		}
	}()

	client := m.redisClient.GetClient()

	// Check if already in hot storage (double-check with lock)
	hotKey := fmt.Sprintf("context:hot:%s:%s", tenantID, contextID)
	exists, _ := client.Exists(promotionCtx, hotKey).Result()
	if exists > 0 {
		m.logger.Debug("Context already in hot storage", map[string]interface{}{
			"context_id": contextID,
		})
		return
	}

	// Update metadata
	contextData.Metadata.State = StateHot
	contextData.Metadata.LastAccessed = time.Now()

	// Marshal context data
	contextJSON, err := json.Marshal(contextData)
	if err != nil {
		m.logger.Error("Failed to marshal context for promotion", map[string]interface{}{
			"context_id": contextID,
			"error":      err.Error(),
		})
		return
	}

	// Start a Redis transaction
	pipe := client.TxPipeline()

	// Store in hot tier
	pipe.Set(promotionCtx, hotKey, contextJSON, m.config.HotDuration)

	// Update metadata
	metaKey := fmt.Sprintf("context:meta:%s:%s", tenantID, contextID)
	metaJSON, _ := json.Marshal(contextData.Metadata)
	pipe.Set(promotionCtx, metaKey, metaJSON, 0)

	// Remove from warm tier
	warmKey := fmt.Sprintf("context:warm:%s:%s", tenantID, contextID)
	pipe.Del(promotionCtx, warmKey)

	// Remove from transition set
	pipe.ZRem(promotionCtx, transitionSetHotToWarm, fmt.Sprintf("%s:%s", tenantID, contextID))

	// Execute transaction
	_, err = pipe.Exec(promotionCtx)
	if err != nil {
		m.logger.Error("Failed to promote context to hot storage", map[string]interface{}{
			"context_id": contextID,
			"tenant_id":  tenantID,
			"error":      err.Error(),
		})
		return
	}

	// Update metrics
	m.metrics.mu.Lock()
	m.metrics.HotContexts++
	m.metrics.WarmContexts--
	m.metrics.TotalTransitions++
	m.metrics.mu.Unlock()

	m.logger.Info("Successfully promoted context to hot storage", map[string]interface{}{
		"context_id":   contextID,
		"tenant_id":    tenantID,
		"access_count": contextData.Metadata.AccessCount,
	})
}

// retrieveFromCold retrieves a context from cold storage
func (m *ContextLifecycleManager) retrieveFromCold(ctx context.Context, tenantID, contextID string) (*ContextData, error) {
	start := time.Now()

	// Use circuit breaker for cold storage access
	var contextData *ContextData
	err := m.coldStorageBreaker.Execute(ctx, func() error {
		// Try multiple possible archive paths (by date)
		possiblePaths := m.generateColdStoragePaths(tenantID, contextID)

		for _, archivePath := range possiblePaths {
			data, err := m.storageBackend.Retrieve(ctx, archivePath)
			if err != nil {
				continue // Try next path
			}

			// Decompress the data
			decompressed, err := m.compression.Decompress(data)
			if err != nil {
				m.logger.Warn("Failed to decompress cold storage data", map[string]interface{}{
					"path":  archivePath,
					"error": err.Error(),
				})
				continue
			}

			// Unmarshal the context
			var retrievedData ContextData
			if err := json.Unmarshal(decompressed, &retrievedData); err != nil {
				m.logger.Warn("Failed to unmarshal cold storage data", map[string]interface{}{
					"path":  archivePath,
					"error": err.Error(),
				})
				continue
			}

			contextData = &retrievedData

			// Update access metrics
			m.updateAccessMetrics(StateCold, time.Since(start))

			// Log successful retrieval
			m.logger.Info("Retrieved context from cold storage", map[string]interface{}{
				"context_id": contextID,
				"tenant_id":  tenantID,
				"path":       archivePath,
				"duration":   time.Since(start).String(),
			})

			// Promote to warm tier for faster subsequent access
			go m.promoteFromColdToWarm(context.Background(), tenantID, contextID, contextData)

			return nil
		}

		return fmt.Errorf("context not found in cold storage after checking %d paths", len(possiblePaths))
	})

	if err != nil {
		// Check if we can reconstruct from AI summary
		if summaryData, summaryErr := m.reconstructFromSummary(ctx, tenantID, contextID); summaryErr == nil {
			return summaryData, nil
		}

		return nil, fmt.Errorf("context not found in any storage tier: %w", err)
	}

	return contextData, nil
}

// generateColdStoragePaths generates possible archive paths for a context
func (m *ContextLifecycleManager) generateColdStoragePaths(tenantID, contextID string) []string {
	paths := make([]string, 0, 7)

	// Check last 7 days of archives
	for i := 0; i < 7; i++ {
		date := time.Now().AddDate(0, 0, -i)
		path := fmt.Sprintf("contexts/%s/%s/%s.json.gz",
			tenantID,
			date.Format("2006/01/02"),
			contextID,
		)
		paths = append(paths, path)
	}

	// Also check a flat structure (legacy support)
	paths = append(paths, fmt.Sprintf("contexts/%s/%s.json.gz", tenantID, contextID))

	return paths
}

// promoteFromColdToWarm promotes a context from cold to warm storage
func (m *ContextLifecycleManager) promoteFromColdToWarm(ctx context.Context, tenantID, contextID string, contextData *ContextData) {
	// Update metadata
	contextData.Metadata.State = StateWarm
	contextData.Metadata.LastAccessed = time.Now()

	// Compress for warm storage
	jsonData, _ := json.Marshal(contextData)
	compressed, ratio, err := m.compression.CompressWithSemantics(jsonData)
	if err != nil {
		m.logger.Error("Failed to compress context for warm promotion", map[string]interface{}{
			"context_id": contextID,
			"error":      err.Error(),
		})
		return
	}

	contextData.Metadata.CompressionRatio = ratio

	// Store in warm tier
	client := m.redisClient.GetClient()
	warmKey := fmt.Sprintf("context:warm:%s:%s", tenantID, contextID)
	if err := client.Set(ctx, warmKey, compressed, m.config.WarmDuration).Err(); err != nil {
		m.logger.Error("Failed to store in warm tier after cold retrieval", map[string]interface{}{
			"context_id": contextID,
			"error":      err.Error(),
		})
		return
	}

	// Update metadata
	metaKey := fmt.Sprintf("context:meta:%s:%s", tenantID, contextID)
	metaJSON, _ := json.Marshal(contextData.Metadata)
	client.Set(ctx, metaKey, metaJSON, 0)

	// Add to warm->cold transition tracking
	transitionTime := time.Now().Add(m.config.WarmDuration).Unix()
	transitionKey := fmt.Sprintf("%s:%s", tenantID, contextID)
	client.ZAdd(ctx, transitionSetWarmToCold, redisclient.Z{
		Score:  float64(transitionTime),
		Member: transitionKey,
	})

	m.logger.Info("Promoted context from cold to warm storage", map[string]interface{}{
		"context_id": contextID,
		"tenant_id":  tenantID,
	})
}

// reconstructFromSummary attempts to reconstruct context from AI-generated summary
func (m *ContextLifecycleManager) reconstructFromSummary(ctx context.Context, tenantID, contextID string) (*ContextData, error) {
	// This is a placeholder for AI-based reconstruction
	// In a real implementation, this would:
	// 1. Retrieve the summary from cold storage
	// 2. Use an LLM to expand the summary back to structured context
	// 3. Return the reconstructed context with a flag indicating it's reconstructed

	return nil, fmt.Errorf("AI reconstruction not implemented")
}

// Stop gracefully stops the lifecycle manager
func (m *ContextLifecycleManager) Stop() {
	// Stop batch processor
	if m.batchProcessor != nil {
		m.batchProcessor.Stop()
	}

	// Stop transition workers
	close(m.transitionStop)
	m.wg.Wait()
}

// GetMetrics returns lifecycle metrics
func (m *ContextLifecycleManager) GetMetrics() map[string]interface{} {
	m.metrics.mu.RLock()
	defer m.metrics.mu.RUnlock()

	return map[string]interface{}{
		"hot_contexts":        m.metrics.HotContexts,
		"warm_contexts":       m.metrics.WarmContexts,
		"cold_contexts":       m.metrics.ColdContexts,
		"total_transitions":   m.metrics.TotalTransitions,
		"compression_saved":   m.metrics.CompressionSaved,
		"average_access_time": m.metrics.AverageAccessTime,
	}
}

// DeleteContext deletes a context from all storage tiers
func (m *ContextLifecycleManager) DeleteContext(ctx context.Context, tenantID, contextID string) error {
	client := m.redisClient.GetClient()

	// Use pipeline for atomic deletion
	pipe := client.TxPipeline()

	// Delete from all storage tiers
	hotKey := fmt.Sprintf("context:hot:%s:%s", tenantID, contextID)
	warmKey := fmt.Sprintf("context:warm:%s:%s", tenantID, contextID)
	metaKey := fmt.Sprintf("context:meta:%s:%s", tenantID, contextID)

	pipe.Del(ctx, hotKey, warmKey, metaKey)

	// Remove from transition tracking sets
	transitionKey := fmt.Sprintf("%s:%s", tenantID, contextID)
	pipe.ZRem(ctx, transitionSetHotToWarm, transitionKey)
	pipe.ZRem(ctx, transitionSetWarmToCold, transitionKey)

	// Remove from index sorted sets
	contextIndexKey := fmt.Sprintf("%s:%s", tenantID, contextID)
	tenantSetKey := fmt.Sprintf(contextsByTenantSet, tenantID)
	pipe.ZRem(ctx, tenantSetKey, contextIndexKey)
	pipe.ZRem(ctx, allContextsSet, contextIndexKey)

	// Execute pipeline
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete context from Redis: %w", err)
	}

	// Delete from cold storage (best effort)
	possiblePaths := m.generateColdStoragePaths(tenantID, contextID)
	for _, path := range possiblePaths {
		_ = m.storageBackend.Delete(ctx, path)
	}

	return nil
}

// calculateImportance calculates the importance score for a context
func (m *ContextLifecycleManager) calculateImportance(context *AgentContext) float64 {
	var score float64

	// Recency score (0.0 to 0.4)
	age := time.Since(context.UpdatedAt)
	if age < 1*time.Hour {
		score += 0.4
	} else if age < 6*time.Hour {
		score += 0.3
	} else if age < 24*time.Hour {
		score += 0.2
	} else if age < 7*24*time.Hour {
		score += 0.1
	}

	// Activity score based on conversation length (0.0 to 0.3)
	convLen := len(context.ConversationHistory)
	if convLen > 10 {
		score += 0.3
	} else if convLen > 5 {
		score += 0.2
	} else if convLen > 0 {
		score += 0.1
	}

	// Variables score (0.0 to 0.3)
	varCount := len(context.Variables)
	if varCount > 5 {
		score += 0.3
	} else if varCount > 2 {
		score += 0.2
	} else if varCount > 0 {
		score += 0.1
	}

	return score
}

// SearchContexts searches for contexts matching the given criteria
func (m *ContextLifecycleManager) SearchContexts(ctx context.Context, criteria *ContextSearchCriteria) ([]*AgentContext, error) {
	startTime := time.Now()
	var results []*AgentContext
	client := m.redisClient.GetClient()

	// Determine which sorted set to query
	var setKey string
	if criteria.TenantID != "" {
		setKey = fmt.Sprintf(contextsByTenantSet, criteria.TenantID)
	} else {
		setKey = allContextsSet
	}

	// Build score range for time-based filtering
	minScore := "-inf"
	maxScore := "+inf"

	if !criteria.StartTime.IsZero() {
		minScore = fmt.Sprintf("%d", criteria.StartTime.Unix())
	}
	if !criteria.EndTime.IsZero() {
		maxScore = fmt.Sprintf("%d", criteria.EndTime.Unix())
	}

	// Query sorted set with score range
	members, err := client.ZRangeByScore(ctx, setKey, &redisclient.ZRangeBy{
		Min:   minScore,
		Max:   maxScore,
		Count: 1000, // Limit results
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to query context index: %w", err)
	}

	// Process each member
	for _, member := range members {
		// Extract IDs from member (format: "tenantID:contextID")
		parts := strings.Split(member, ":")
		if len(parts) != 2 {
			continue
		}

		tenantID := parts[0]
		contextID := parts[1]

		// Skip if searching for specific tenant and doesn't match
		if criteria.TenantID != "" && tenantID != criteria.TenantID {
			continue
		}

		// Get the actual context
		contextData, err := m.GetContext(ctx, tenantID, contextID)
		if err != nil || contextData == nil {
			continue
		}

		// Convert to AgentContext
		agentContext := &AgentContext{
			EventID:   contextData.Metadata.ID,
			TenantID:  contextData.Metadata.TenantID,
			CreatedAt: contextData.Metadata.CreatedAt,
			UpdatedAt: contextData.Metadata.LastAccessed,
		}

		// Apply additional filters
		if criteria.ToolID != "" {
			if toolID, ok := contextData.Data["tool_id"].(string); !ok || toolID != criteria.ToolID {
				continue
			}
		}

		results = append(results, agentContext)
	}

	// Record search performance metrics
	if m.metricsRecorder != nil {
		criteriaType := "all"
		if criteria.TenantID != "" {
			criteriaType = "tenant"
		} else if !criteria.StartTime.IsZero() || !criteria.EndTime.IsZero() {
			criteriaType = "time_range"
		} else if criteria.ToolID != "" {
			criteriaType = "tool"
		}

		m.metricsRecorder.RecordSearchPerformance(criteriaType, len(results), time.Since(startTime))
	}

	return results, nil
}

// Story 5.1: Semantic Context Integration
// The following methods integrate semantic context management with lifecycle operations

// PromoteToHot promotes a context to hot storage tier
func (m *ContextLifecycleManager) PromoteToHot(ctx context.Context, tenantID, contextID string) error {
	// Delegate to internal promoteToHot method
	contextData, err := m.GetContext(ctx, tenantID, contextID)
	if err != nil {
		return fmt.Errorf("failed to get context for promotion: %w", err)
	}

	// Call internal promotion method
	go m.promoteToHot(ctx, tenantID, contextID, contextData)

	return nil
}

// PromoteToHotWithEmbeddings promotes context with embeddings to hot tier
func (m *ContextLifecycleManager) PromoteToHotWithEmbeddings(
	ctx context.Context,
	tenantID string,
	contextID string,
	embeddings []byte,
) error {
	// Use existing PromoteToHot method
	if err := m.PromoteToHot(ctx, tenantID, contextID); err != nil {
		return err
	}

	// Additionally store embeddings in hot tier for fast access
	embeddingKey := fmt.Sprintf("embeddings:hot:%s:%s", tenantID, contextID)

	// Store in Redis with same TTL as context
	client := m.redisClient.GetClient()
	if err := client.Set(ctx, embeddingKey, embeddings, m.config.HotDuration).Err(); err != nil {
		m.logger.Warn("Failed to cache embeddings", map[string]interface{}{
			"context_id": contextID,
			"tenant_id":  tenantID,
			"error":      err.Error(),
		})
		// Don't fail the whole operation if caching fails
	}

	return nil
}

// GetWithEmbeddings retrieves context with embeddings from appropriate tier
func (m *ContextLifecycleManager) GetWithEmbeddings(
	ctx context.Context,
	tenantID string,
	contextID string,
) (*ContextData, []byte, error) {
	// Get context using existing method
	contextData, err := m.GetContext(ctx, tenantID, contextID)
	if err != nil {
		return nil, nil, err
	}

	// Try to get embeddings from cache first
	embeddingKey := fmt.Sprintf("embeddings:hot:%s:%s", tenantID, contextID)

	var embeddings []byte
	client := m.redisClient.GetClient()
	embeddingData, err := client.Get(ctx, embeddingKey).Bytes()
	if err == nil {
		// Found in cache
		embeddings = embeddingData
	} else {
		// Not in cache - log but don't fail
		m.logger.Debug("Embeddings not found in cache", map[string]interface{}{
			"context_id": contextID,
			"tenant_id":  tenantID,
		})
		// embeddings will be nil - caller should fetch from database if needed
	}

	return contextData, embeddings, nil
}

// ArchiveToCold archives a context to cold storage
func (m *ContextLifecycleManager) ArchiveToCold(ctx context.Context, tenantID, contextID string) error {
	// Delegate to TransitionToCold
	return m.TransitionToCold(ctx, tenantID, contextID)
}

// CompactAndArchive compacts context before archiving
func (m *ContextLifecycleManager) CompactAndArchive(
	ctx context.Context,
	tenantID string,
	contextID string,
	strategy string,
) error {
	// First compact
	m.logger.Info("Compacting context before archive", map[string]interface{}{
		"context_id": contextID,
		"tenant_id":  tenantID,
		"strategy":   strategy,
	})

	// Note: Actual compaction logic should be called here if available
	// For now, just log the intent and proceed with archival

	// Then archive using existing method
	return m.ArchiveToCold(ctx, tenantID, contextID)
}
