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
)

// ContextState represents the storage tier of a context
type ContextState string

const (
	StateHot  ContextState = "hot"
	StateWarm ContextState = "warm"
	StateCold ContextState = "cold"
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

	// Metrics
	metrics LifecycleMetrics
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
		metrics: LifecycleMetrics{
			AverageAccessTime: make(map[ContextState]time.Duration),
		},
	}

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

// processTransitions processes pending lifecycle transitions
func (m *ContextLifecycleManager) processTransitions() {
	ctx := context.Background()
	client := m.redisClient.GetClient()

	// Find contexts ready for transition
	// This is simplified - in production, use Redis sorted sets for efficient querying

	// Process hot -> warm transitions
	hotPattern := "context:hot:*"
	hotKeys, _ := client.Keys(ctx, hotPattern).Result()

	for _, key := range hotKeys {
		ttl, _ := client.TTL(ctx, key).Result()
		if ttl < 30*time.Minute && ttl > 0 {
			// Extract tenant ID and context ID from key
			// Format: context:hot:tenantID:contextID
			parts := strings.Split(key, ":")
			if len(parts) == 4 {
				tenantID := parts[2]
				contextID := parts[3]
				_ = m.TransitionToWarm(ctx, tenantID, contextID)
			}
		}
	}

	// Similar logic for warm -> cold transitions
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
	// Implementation for promoting frequently accessed contexts back to hot storage
	m.logger.Info("Promoting context to hot storage", map[string]interface{}{
		"context_id":   contextID,
		"tenant_id":    tenantID,
		"access_count": contextData.Metadata.AccessCount,
	})
}

// retrieveFromCold retrieves a context from cold storage
func (m *ContextLifecycleManager) retrieveFromCold(ctx context.Context, tenantID, contextID string) (*ContextData, error) {
	// This would implement retrieval from S3/cold storage
	// Including reconstruction from AI summaries if needed
	return nil, fmt.Errorf("context not found in any storage tier")
}

// Stop gracefully stops the lifecycle manager
func (m *ContextLifecycleManager) Stop() {
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

	// Delete from all tiers
	hotKey := fmt.Sprintf("context:hot:%s:%s", tenantID, contextID)
	warmKey := fmt.Sprintf("context:warm:%s:%s", tenantID, contextID)
	metaKey := fmt.Sprintf("context:meta:%s:%s", tenantID, contextID)

	// Delete from Redis
	client.Del(ctx, hotKey, warmKey, metaKey)

	// Delete from cold storage
	archiveKey := fmt.Sprintf("contexts/%s/%s.json.gz", tenantID, contextID)
	_ = m.storageBackend.Delete(ctx, archiveKey)

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
	var results []*AgentContext
	client := m.redisClient.GetClient()

	// Simple implementation - scan metadata keys
	pattern := "context:meta:*"
	if criteria.TenantID != "" {
		pattern = fmt.Sprintf("context:meta:%s:*", criteria.TenantID)
	}

	keys, err := client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	for _, metaKey := range keys {
		// Extract IDs from key
		parts := strings.Split(metaKey, ":")
		if len(parts) != 4 {
			continue
		}

		tenantID := parts[2]
		contextID := parts[3]

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

		// Apply filters
		if criteria.ToolID != "" {
			if toolID, ok := contextData.Data["tool_id"].(string); !ok || toolID != criteria.ToolID {
				continue
			}
		}

		if !criteria.StartTime.IsZero() && contextData.Metadata.CreatedAt.Before(criteria.StartTime) {
			continue
		}

		if !criteria.EndTime.IsZero() && contextData.Metadata.CreatedAt.After(criteria.EndTime) {
			continue
		}

		results = append(results, agentContext)
	}

	return results, nil
}
