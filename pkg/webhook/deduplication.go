package webhook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/bits-and-blooms/bloom/v3"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/redis"
)

// DeduplicationConfig contains configuration for the deduplication system
type DeduplicationConfig struct {
	// Window configurations per tool type
	WindowConfigs map[string]WindowConfig

	// Default window configuration
	DefaultWindow WindowConfig

	// Bloom filter settings
	BloomFilterSize      uint
	BloomFilterHashFuncs uint
	BloomRotationPeriod  time.Duration

	// Redis key prefix
	RedisKeyPrefix string
}

// WindowConfig defines deduplication window settings
type WindowConfig struct {
	Duration time.Duration
	MaxSize  int64 // Maximum number of entries to track
}

// DefaultDeduplicationConfig returns default configuration
func DefaultDeduplicationConfig() *DeduplicationConfig {
	return &DeduplicationConfig{
		WindowConfigs: map[string]WindowConfig{
			"github": {
				Duration: 5 * time.Minute,
				MaxSize:  10000,
			},
			"gitlab": {
				Duration: 5 * time.Minute,
				MaxSize:  10000,
			},
			"jira": {
				Duration: 10 * time.Minute,
				MaxSize:  5000,
			},
		},
		DefaultWindow: WindowConfig{
			Duration: 5 * time.Minute,
			MaxSize:  10000,
		},
		BloomFilterSize:      1000000, // 1M entries
		BloomFilterHashFuncs: 7,
		BloomRotationPeriod:  24 * time.Hour,
		RedisKeyPrefix:       "webhook:dedup:",
	}
}

// Deduplicator handles webhook event deduplication
type Deduplicator struct {
	config      *DeduplicationConfig
	redisClient *redis.StreamsClient
	logger      observability.Logger

	// Current and previous bloom filters for rotation
	currentBloom  *bloom.BloomFilter
	previousBloom *bloom.BloomFilter
	bloomMu       sync.RWMutex
	lastRotation  time.Time

	// Metrics
	metrics DeduplicationMetrics
}

// DeduplicationMetrics tracks deduplication statistics
type DeduplicationMetrics struct {
	mu                  sync.RWMutex
	TotalChecked        int64
	Duplicates          int64
	UniqueEvents        int64
	BloomFalsePositives int64
}

// NewDeduplicator creates a new deduplicator
func NewDeduplicator(config *DeduplicationConfig, redisClient *redis.StreamsClient, logger observability.Logger) (*Deduplicator, error) {
	if config == nil {
		config = DefaultDeduplicationConfig()
	}

	d := &Deduplicator{
		config:       config,
		redisClient:  redisClient,
		logger:       logger,
		lastRotation: time.Now(),
	}

	// Initialize bloom filters
	d.currentBloom = bloom.NewWithEstimates(config.BloomFilterSize, float64(config.BloomFilterHashFuncs)/100.0)
	d.previousBloom = bloom.NewWithEstimates(config.BloomFilterSize, float64(config.BloomFilterHashFuncs)/100.0)

	// Load bloom filters from Redis if they exist
	if err := d.loadBloomFilters(); err != nil {
		logger.Warn("Failed to load bloom filters from Redis, starting fresh", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Start rotation routine
	go d.rotationLoop()

	return d, nil
}

// GenerateMessageID generates a composite message ID for deduplication
func (d *Deduplicator) GenerateMessageID(toolID, eventType string, payload []byte) string {
	// Create hash of payload
	hasher := sha256.New()
	hasher.Write(payload)
	payloadHash := hex.EncodeToString(hasher.Sum(nil))

	// Create composite ID
	return fmt.Sprintf("%s:%s:%s", toolID, eventType, payloadHash[:16])
}

// CheckDuplicate checks if an event is a duplicate
func (d *Deduplicator) CheckDuplicate(ctx context.Context, messageID, toolType string) (bool, string, error) {
	d.metrics.mu.Lock()
	d.metrics.TotalChecked++
	d.metrics.mu.Unlock()

	// Check bloom filter first (fast path)
	d.bloomMu.RLock()
	inCurrent := d.currentBloom.Test([]byte(messageID))
	inPrevious := d.previousBloom.Test([]byte(messageID))
	d.bloomMu.RUnlock()

	if !inCurrent && !inPrevious {
		// Definitely not a duplicate
		d.recordUnique(messageID)
		return false, "", nil
	}

	// Possible duplicate, check Redis for certainty
	window := d.getWindow(toolType)
	redisKey := d.config.RedisKeyPrefix + messageID

	// Try to set with NX (only if not exists) and expiration
	client := d.redisClient.GetClient()
	result := client.SetNX(ctx, redisKey, time.Now().Unix(), window.Duration)

	if result.Err() != nil {
		return false, "", fmt.Errorf("failed to check duplicate in Redis: %w", result.Err())
	}

	isDuplicate := !result.Val() // SetNX returns false if key exists

	if isDuplicate {
		// It's a duplicate, get the original timestamp
		originalTime := client.Get(ctx, redisKey).Val()

		d.metrics.mu.Lock()
		d.metrics.Duplicates++
		d.metrics.mu.Unlock()

		d.logger.Debug("Duplicate event detected", map[string]interface{}{
			"message_id":    messageID,
			"tool_type":     toolType,
			"original_time": originalTime,
		})

		return true, messageID, nil
	}

	// Not a duplicate, but bloom filter thought it might be
	if inCurrent || inPrevious {
		d.metrics.mu.Lock()
		d.metrics.BloomFalsePositives++
		d.metrics.mu.Unlock()
	}

	d.recordUnique(messageID)
	return false, "", nil
}

// recordUnique records a unique message
func (d *Deduplicator) recordUnique(messageID string) {
	// Add to bloom filter
	d.bloomMu.Lock()
	d.currentBloom.Add([]byte(messageID))
	d.bloomMu.Unlock()

	d.metrics.mu.Lock()
	d.metrics.UniqueEvents++
	d.metrics.mu.Unlock()
}

// getWindow returns the deduplication window for a tool type
func (d *Deduplicator) getWindow(toolType string) WindowConfig {
	if window, exists := d.config.WindowConfigs[toolType]; exists {
		return window
	}
	return d.config.DefaultWindow
}

// rotationLoop handles bloom filter rotation
func (d *Deduplicator) rotationLoop() {
	ticker := time.NewTicker(1 * time.Hour) // Check every hour
	defer ticker.Stop()

	for range ticker.C {
		if time.Since(d.lastRotation) >= d.config.BloomRotationPeriod {
			d.rotateBloomFilters()
		}
	}
}

// rotateBloomFilters rotates the bloom filters
func (d *Deduplicator) rotateBloomFilters() {
	d.bloomMu.Lock()
	defer d.bloomMu.Unlock()

	// Save current bloom to Redis before rotation
	if err := d.saveBloomFilter("current", d.currentBloom); err != nil {
		d.logger.Error("Failed to save current bloom filter", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Rotate filters
	d.previousBloom = d.currentBloom
	d.currentBloom = bloom.NewWithEstimates(d.config.BloomFilterSize, float64(d.config.BloomFilterHashFuncs)/100.0)
	d.lastRotation = time.Now()

	d.logger.Info("Rotated bloom filters", map[string]interface{}{
		"timestamp": d.lastRotation,
	})

	// Save new state
	if err := d.saveBloomFilter("previous", d.previousBloom); err != nil {
		d.logger.Error("Failed to save previous bloom filter", map[string]interface{}{
			"error": err.Error(),
		})
	}
}

// saveBloomFilter saves a bloom filter to Redis
func (d *Deduplicator) saveBloomFilter(name string, bf *bloom.BloomFilter) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	data, err := bf.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal bloom filter: %w", err)
	}

	key := fmt.Sprintf("%sbloom:%s", d.config.RedisKeyPrefix, name)
	client := d.redisClient.GetClient()

	return client.Set(ctx, key, data, 48*time.Hour).Err() // Keep for 2 days
}

// loadBloomFilters loads bloom filters from Redis
func (d *Deduplicator) loadBloomFilters() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := d.redisClient.GetClient()

	// Load current bloom
	currentKey := fmt.Sprintf("%sbloom:current", d.config.RedisKeyPrefix)
	currentData, err := client.Get(ctx, currentKey).Bytes()
	if err == nil {
		bf := bloom.New(d.config.BloomFilterSize, d.config.BloomFilterHashFuncs)
		if err := bf.UnmarshalBinary(currentData); err == nil {
			d.currentBloom = bf
			d.logger.Info("Loaded current bloom filter from Redis", nil)
		}
	}

	// Load previous bloom
	previousKey := fmt.Sprintf("%sbloom:previous", d.config.RedisKeyPrefix)
	previousData, err := client.Get(ctx, previousKey).Bytes()
	if err == nil {
		bf := bloom.New(d.config.BloomFilterSize, d.config.BloomFilterHashFuncs)
		if err := bf.UnmarshalBinary(previousData); err == nil {
			d.previousBloom = bf
			d.logger.Info("Loaded previous bloom filter from Redis", nil)
		}
	}

	return nil
}

// GetMetrics returns deduplication metrics
func (d *Deduplicator) GetMetrics() map[string]interface{} {
	d.metrics.mu.RLock()
	defer d.metrics.mu.RUnlock()

	duplicateRate := float64(0)
	if d.metrics.TotalChecked > 0 {
		duplicateRate = float64(d.metrics.Duplicates) / float64(d.metrics.TotalChecked) * 100
	}

	falsePositiveRate := float64(0)
	if d.metrics.UniqueEvents > 0 {
		falsePositiveRate = float64(d.metrics.BloomFalsePositives) / float64(d.metrics.UniqueEvents) * 100
	}

	return map[string]interface{}{
		"total_checked":         d.metrics.TotalChecked,
		"duplicates":            d.metrics.Duplicates,
		"unique_events":         d.metrics.UniqueEvents,
		"duplicate_rate":        duplicateRate,
		"bloom_false_positives": d.metrics.BloomFalsePositives,
		"false_positive_rate":   falsePositiveRate,
		"bloom_size":            d.config.BloomFilterSize,
		"last_rotation":         d.lastRotation,
	}
}

// DeduplicationResult contains the result of a deduplication check
type DeduplicationResult struct {
	IsDuplicate     bool
	OriginalEventID string
	MessageID       string
	CheckDuration   time.Duration
}

// ProcessEvent performs full deduplication check for an event
func (d *Deduplicator) ProcessEvent(ctx context.Context, toolID, toolType, eventType string, payload []byte) (*DeduplicationResult, error) {
	start := time.Now()

	// Generate message ID
	messageID := d.GenerateMessageID(toolID, eventType, payload)

	// Check for duplicate
	isDuplicate, originalID, err := d.CheckDuplicate(ctx, messageID, toolType)
	if err != nil {
		return nil, err
	}

	result := &DeduplicationResult{
		IsDuplicate:     isDuplicate,
		OriginalEventID: originalID,
		MessageID:       messageID,
		CheckDuration:   time.Since(start),
	}

	return result, nil
}
