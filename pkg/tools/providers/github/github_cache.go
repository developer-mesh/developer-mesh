package github

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// GitHubCache provides thread-safe caching for GitHub API responses
// NOTE: The official GitHub MCP server does not implement caching.
// This is an optional enhancement for better performance.
type GitHubCache struct {
	mu    sync.RWMutex
	items map[string]*CacheItem
	ttl   time.Duration

	// Metrics for monitoring
	hits   int64
	misses int64

	// Control cleanup goroutine
	stopCleanup chan struct{}
}

// CacheItem represents a cached value with expiration
type CacheItem struct {
	Value     interface{}
	ExpiresAt time.Time
}

// CacheConfig holds cache configuration options
type CacheConfig struct {
	// DefaultTTL is the default time-to-live for cache entries
	DefaultTTL time.Duration

	// MaxSize is the maximum number of items in cache (0 = unlimited)
	MaxSize int

	// CleanupInterval is how often to run cleanup of expired items
	CleanupInterval time.Duration
}

// DefaultCacheConfig returns sensible defaults for caching
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		DefaultTTL:      5 * time.Minute,  // 5 minutes default
		MaxSize:         10000,            // Max 10k items
		CleanupInterval: 10 * time.Minute, // Cleanup every 10 minutes
	}
}

// NewGitHubCache creates a new cache instance
func NewGitHubCache(config *CacheConfig) *GitHubCache {
	if config == nil {
		config = DefaultCacheConfig()
	}

	cache := &GitHubCache{
		items:       make(map[string]*CacheItem),
		ttl:         config.DefaultTTL,
		stopCleanup: make(chan struct{}),
	}

	// Start cleanup goroutine
	go cache.cleanupExpired(config.CleanupInterval)
	return cache
}

// Get retrieves a value from the cache
func (c *GitHubCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok {
		c.misses++
		return nil, false
	}

	// Check if expired
	if time.Now().After(item.ExpiresAt) {
		c.misses++
		return nil, false
	}

	c.hits++
	return item.Value, true
}

// Set stores a value in the cache with optional TTL
func (c *GitHubCache) Set(key string, value interface{}, ttl ...time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiry := c.ttl
	if len(ttl) > 0 && ttl[0] > 0 {
		expiry = ttl[0]
	}

	c.items[key] = &CacheItem{
		Value:     value,
		ExpiresAt: time.Now().Add(expiry),
	}
}

// Delete removes a key from the cache
func (c *GitHubCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Clear removes all items from the cache
func (c *GitHubCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*CacheItem)
}

// Size returns the number of items in the cache
func (c *GitHubCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Stats returns cache statistics
func (c *GitHubCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(c.hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"hits":      c.hits,
		"misses":    c.misses,
		"hit_rate":  hitRate,
		"size":      len(c.items),
		"total_ops": total,
	}
}

// InvalidatePattern removes all keys matching a pattern
func (c *GitHubCache) InvalidatePattern(pattern string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for key := range c.items {
		if strings.Contains(key, pattern) {
			delete(c.items, key)
			count++
		}
	}
	return count
}

// cleanupExpired removes expired items periodically
func (c *GitHubCache) cleanupExpired(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.removeExpired()
		case <-c.stopCleanup:
			return
		}
	}
}

// removeExpired removes all expired items
func (c *GitHubCache) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, item := range c.items {
		if now.After(item.ExpiresAt) {
			delete(c.items, key)
		}
	}
}

// Stop gracefully stops the cache cleanup goroutine
func (c *GitHubCache) Stop() {
	close(c.stopCleanup)
}

// Cache key builders for consistent keys

// BuildCacheKey creates a consistent cache key from resource and parameters
func BuildCacheKey(resource string, params ...string) string {
	parts := append([]string{"github", resource}, params...)
	return strings.Join(parts, ":")
}

// BuildRepositoryCacheKey creates a cache key for repository operations
func BuildRepositoryCacheKey(owner, repo, operation string, params ...string) string {
	parts := append([]string{"github", "repo", owner, repo, operation}, params...)
	return strings.Join(parts, ":")
}

// BuildIssueCacheKey creates a cache key for issue operations
func BuildIssueCacheKey(owner, repo string, issueNum int, operation string) string {
	return fmt.Sprintf("github:issue:%s:%s:%d:%s", owner, repo, issueNum, operation)
}

// BuildPullRequestCacheKey creates a cache key for PR operations
func BuildPullRequestCacheKey(owner, repo string, prNum int, operation string) string {
	return fmt.Sprintf("github:pr:%s:%s:%d:%s", owner, repo, prNum, operation)
}

// BuildUserCacheKey creates a cache key for user operations
func BuildUserCacheKey(username, operation string) string {
	return fmt.Sprintf("github:user:%s:%s", username, operation)
}

// BuildOrgCacheKey creates a cache key for organization operations
func BuildOrgCacheKey(org, operation string) string {
	return fmt.Sprintf("github:org:%s:%s", org, operation)
}

// BuildSearchCacheKey creates a cache key for search operations
func BuildSearchCacheKey(searchType, query string, page int) string {
	// Hash the query to keep key reasonable length
	queryHash := fmt.Sprintf("%x", query)
	if len(queryHash) > 20 {
		queryHash = queryHash[:20]
	}
	return fmt.Sprintf("github:search:%s:%s:%d", searchType, queryHash, page)
}

// Cache TTL recommendations for different operations
const (
	// Short TTL for frequently changing data
	TTLShort = 1 * time.Minute

	// Medium TTL for moderately stable data
	TTLMedium = 5 * time.Minute

	// Long TTL for rarely changing data
	TTLLong = 15 * time.Minute

	// Extra long TTL for very stable data
	TTLExtraLong = 1 * time.Hour
)

// GetRecommendedTTL returns recommended TTL for different operation types
func GetRecommendedTTL(operationType string) time.Duration {
	switch operationType {
	// Very short TTL for things that change frequently
	case "notifications", "workflow_runs", "workflow_jobs":
		return TTLShort

	// Medium TTL for standard operations
	case "issues", "pulls", "commits", "branches":
		return TTLMedium

	// Long TTL for stable data
	case "repositories", "organizations", "users", "teams":
		return TTLLong

	// Extra long TTL for very stable data
	case "licenses", "gitignore_templates", "languages":
		return TTLExtraLong

	default:
		return TTLMedium
	}
}
