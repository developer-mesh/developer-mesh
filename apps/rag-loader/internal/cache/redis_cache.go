// Package cache provides caching implementations for the RAG loader
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

var (
	// ErrCacheMiss is returned when a cache key is not found
	ErrCacheMiss = errors.New("cache miss")

	// ErrCacheInvalid is returned when cached data is invalid
	ErrCacheInvalid = errors.New("invalid cached data")
)

// CacheConfig configures the cache behavior
type CacheConfig struct {
	// Enabled determines if caching is enabled
	Enabled bool

	// DefaultTTL is the default time-to-live for cache entries
	DefaultTTL time.Duration

	// MaxMemoryMB is the maximum memory to use for caching
	MaxMemoryMB int

	// EvictionPolicy determines how to evict entries when full
	EvictionPolicy string

	// KeyPrefix is prepended to all cache keys
	KeyPrefix string
}

// DefaultCacheConfig returns sensible defaults
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Enabled:        true,
		DefaultTTL:     24 * time.Hour,
		MaxMemoryMB:    1024, // 1GB
		EvictionPolicy: "allkeys-lru",
		KeyPrefix:      "rag:",
	}
}

// RedisCache implements caching using Redis
type RedisCache struct {
	client *redis.Client
	config CacheConfig
	logger observability.Logger

	// Metrics
	hits   int64
	misses int64
}

// NewRedisCache creates a new Redis cache
func NewRedisCache(client *redis.Client, config CacheConfig, logger observability.Logger) *RedisCache {
	return &RedisCache{
		client: client,
		config: config,
		logger: logger.WithPrefix("redis-cache"),
	}
}

// Get retrieves a value from the cache
func (rc *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	if !rc.config.Enabled {
		return nil, ErrCacheMiss
	}

	fullKey := rc.makeKey(key)

	val, err := rc.client.Get(ctx, fullKey).Bytes()
	if err == redis.Nil {
		rc.misses++
		return nil, ErrCacheMiss
	}
	if err != nil {
		rc.logger.Error("Cache get error", map[string]interface{}{
			"key":   key,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("cache get error: %w", err)
	}

	rc.hits++
	return val, nil
}

// Set stores a value in the cache
func (rc *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if !rc.config.Enabled {
		return nil
	}

	fullKey := rc.makeKey(key)

	if ttl == 0 {
		ttl = rc.config.DefaultTTL
	}

	err := rc.client.Set(ctx, fullKey, value, ttl).Err()
	if err != nil {
		rc.logger.Error("Cache set error", map[string]interface{}{
			"key":   key,
			"error": err.Error(),
		})
		return fmt.Errorf("cache set error: %w", err)
	}

	return nil
}

// Delete removes a value from the cache
func (rc *RedisCache) Delete(ctx context.Context, key string) error {
	if !rc.config.Enabled {
		return nil
	}

	fullKey := rc.makeKey(key)

	err := rc.client.Del(ctx, fullKey).Err()
	if err != nil {
		rc.logger.Error("Cache delete error", map[string]interface{}{
			"key":   key,
			"error": err.Error(),
		})
		return fmt.Errorf("cache delete error: %w", err)
	}

	return nil
}

// Exists checks if a key exists in the cache
func (rc *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	if !rc.config.Enabled {
		return false, nil
	}

	fullKey := rc.makeKey(key)

	count, err := rc.client.Exists(ctx, fullKey).Result()
	if err != nil {
		return false, fmt.Errorf("cache exists error: %w", err)
	}

	return count > 0, nil
}

// GetJSON retrieves and unmarshals a JSON value from the cache
func (rc *RedisCache) GetJSON(ctx context.Context, key string, dest interface{}) error {
	data, err := rc.Get(ctx, key)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, dest); err != nil {
		rc.logger.Error("Cache unmarshal error", map[string]interface{}{
			"key":   key,
			"error": err.Error(),
		})
		return fmt.Errorf("%w: %v", ErrCacheInvalid, err)
	}

	return nil
}

// SetJSON marshals and stores a JSON value in the cache
func (rc *RedisCache) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal error: %w", err)
	}

	return rc.Set(ctx, key, data, ttl)
}

// MGet retrieves multiple values from the cache
func (rc *RedisCache) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	if !rc.config.Enabled {
		return nil, nil
	}

	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = rc.makeKey(key)
	}

	vals, err := rc.client.MGet(ctx, fullKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("cache mget error: %w", err)
	}

	result := make(map[string][]byte)
	for i, val := range vals {
		if val != nil {
			if strVal, ok := val.(string); ok {
				result[keys[i]] = []byte(strVal)
				rc.hits++
			}
		} else {
			rc.misses++
		}
	}

	return result, nil
}

// MSet stores multiple values in the cache
func (rc *RedisCache) MSet(ctx context.Context, kvPairs map[string][]byte, ttl time.Duration) error {
	if !rc.config.Enabled {
		return nil
	}

	if ttl == 0 {
		ttl = rc.config.DefaultTTL
	}

	// Use pipeline for efficiency
	pipe := rc.client.Pipeline()

	for key, value := range kvPairs {
		fullKey := rc.makeKey(key)
		pipe.Set(ctx, fullKey, value, ttl)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		rc.logger.Error("Cache mset error", map[string]interface{}{
			"count": len(kvPairs),
			"error": err.Error(),
		})
		return fmt.Errorf("cache mset error: %w", err)
	}

	return nil
}

// Clear removes all cache entries with the configured prefix
func (rc *RedisCache) Clear(ctx context.Context) error {
	if !rc.config.Enabled {
		return nil
	}

	pattern := rc.config.KeyPrefix + "*"

	iter := rc.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := rc.client.Del(ctx, iter.Val()).Err(); err != nil {
			rc.logger.Error("Cache clear error", map[string]interface{}{
				"key":   iter.Val(),
				"error": err.Error(),
			})
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("cache clear error: %w", err)
	}

	rc.logger.Info("Cache cleared", nil)
	return nil
}

// Stats returns cache statistics
func (rc *RedisCache) Stats() map[string]interface{} {
	total := rc.hits + rc.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(rc.hits) / float64(total)
	}

	return map[string]interface{}{
		"hits":      rc.hits,
		"misses":    rc.misses,
		"total":     total,
		"hit_rate":  hitRate,
		"miss_rate": 1.0 - hitRate,
	}
}

// makeKey creates a full cache key with prefix
func (rc *RedisCache) makeKey(key string) string {
	return rc.config.KeyPrefix + key
}

// DocumentCache provides specialized caching for documents
type DocumentCache struct {
	cache  *RedisCache
	logger observability.Logger
}

// NewDocumentCache creates a new document cache
func NewDocumentCache(cache *RedisCache, logger observability.Logger) *DocumentCache {
	return &DocumentCache{
		cache:  cache,
		logger: logger.WithPrefix("doc-cache"),
	}
}

// GetDocumentHash retrieves a document hash from cache
func (dc *DocumentCache) GetDocumentHash(ctx context.Context, sourceID, documentID string) (string, error) {
	key := fmt.Sprintf("doc:hash:%s:%s", sourceID, documentID)

	data, err := dc.cache.Get(ctx, key)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// SetDocumentHash stores a document hash in cache
func (dc *DocumentCache) SetDocumentHash(ctx context.Context, sourceID, documentID, hash string) error {
	key := fmt.Sprintf("doc:hash:%s:%s", sourceID, documentID)
	return dc.cache.Set(ctx, key, []byte(hash), 7*24*time.Hour) // 7 days
}

// CheckDuplicate checks if a document is a duplicate based on content hash
func (dc *DocumentCache) CheckDuplicate(ctx context.Context, content string) (bool, string, error) {
	// Calculate content hash
	hash := calculateContentHash(content)

	// Check if hash exists in cache
	key := fmt.Sprintf("doc:content:%s", hash)
	exists, err := dc.cache.Exists(ctx, key)
	if err != nil {
		return false, "", err
	}

	if exists {
		dc.logger.Debug("Duplicate document detected", map[string]interface{}{
			"hash": hash,
		})
	}

	return exists, hash, nil
}

// MarkProcessed marks a document as processed
func (dc *DocumentCache) MarkProcessed(ctx context.Context, hash string) error {
	key := fmt.Sprintf("doc:content:%s", hash)
	return dc.cache.Set(ctx, key, []byte("1"), 30*24*time.Hour) // 30 days
}

// EmbeddingCache provides specialized caching for embeddings
type EmbeddingCache struct {
	cache  *RedisCache
	logger observability.Logger
}

// NewEmbeddingCache creates a new embedding cache
func NewEmbeddingCache(cache *RedisCache, logger observability.Logger) *EmbeddingCache {
	return &EmbeddingCache{
		cache:  cache,
		logger: logger.WithPrefix("emb-cache"),
	}
}

// GetEmbedding retrieves a cached embedding
func (ec *EmbeddingCache) GetEmbedding(ctx context.Context, content string) ([]float32, error) {
	hash := calculateContentHash(content)
	key := fmt.Sprintf("emb:%s", hash)

	var embedding []float32
	if err := ec.cache.GetJSON(ctx, key, &embedding); err != nil {
		return nil, err
	}

	ec.logger.Debug("Embedding cache hit", map[string]interface{}{
		"hash": hash,
	})

	return embedding, nil
}

// SetEmbedding stores an embedding in cache
func (ec *EmbeddingCache) SetEmbedding(ctx context.Context, content string, embedding []float32) error {
	hash := calculateContentHash(content)
	key := fmt.Sprintf("emb:%s", hash)

	return ec.cache.SetJSON(ctx, key, embedding, 7*24*time.Hour) // 7 days
}

// calculateContentHash computes SHA-256 hash of content
func calculateContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// BloomFilter provides a probabilistic check for document existence
type BloomFilter struct {
	cache  *RedisCache
	size   int64
	hashes int
	logger observability.Logger
}

// NewBloomFilter creates a new Bloom filter
func NewBloomFilter(cache *RedisCache, size int64, hashes int, logger observability.Logger) *BloomFilter {
	return &BloomFilter{
		cache:  cache,
		size:   size,
		hashes: hashes,
		logger: logger.WithPrefix("bloom-filter"),
	}
}

// Add adds an item to the Bloom filter
func (bf *BloomFilter) Add(ctx context.Context, item string) error {
	key := "bloom:filter"

	for i := 0; i < bf.hashes; i++ {
		hash := bf.hash(item, i)
		if err := bf.cache.client.SetBit(ctx, key, hash, 1).Err(); err != nil {
			return fmt.Errorf("bloom filter add error: %w", err)
		}
	}

	return nil
}

// Contains checks if an item might be in the Bloom filter
func (bf *BloomFilter) Contains(ctx context.Context, item string) (bool, error) {
	key := "bloom:filter"

	for i := 0; i < bf.hashes; i++ {
		hash := bf.hash(item, i)
		bit, err := bf.cache.client.GetBit(ctx, key, hash).Result()
		if err != nil {
			return false, fmt.Errorf("bloom filter contains error: %w", err)
		}
		if bit == 0 {
			return false, nil
		}
	}

	return true, nil
}

// hash computes a hash position for the Bloom filter
func (bf *BloomFilter) hash(item string, seed int) int64 {
	h := sha256.New()
	h.Write([]byte(item))
	h.Write([]byte{byte(seed)})
	hashBytes := h.Sum(nil)

	// Convert first 8 bytes to int64
	var hashInt int64
	for i := 0; i < 8 && i < len(hashBytes); i++ {
		hashInt = (hashInt << 8) | int64(hashBytes[i])
	}

	if hashInt < 0 {
		hashInt = -hashInt
	}

	return hashInt % bf.size
}
