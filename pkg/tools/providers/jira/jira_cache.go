package jira

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// JiraCacheConfig holds Jira-specific caching settings
type JiraCacheConfig struct {
	// Enable caching for read operations
	EnableResponseCaching bool `yaml:"enable_response_caching" json:"enable_response_caching"`

	// Default TTL for cached responses
	DefaultTTL time.Duration `yaml:"default_ttl" json:"default_ttl"`

	// TTL overrides for specific operation types
	OperationTTLs map[string]time.Duration `yaml:"operation_ttls" json:"operation_ttls"`

	// Enable ETags support
	EnableETags bool `yaml:"enable_etags" json:"enable_etags"`

	// Maximum cache size in MB (0 = unlimited)
	MaxCacheSizeMB int `yaml:"max_cache_size_mb" json:"max_cache_size_mb"`

	// Cache invalidation patterns
	InvalidationPatterns []string `yaml:"invalidation_patterns" json:"invalidation_patterns"`

	// Operations that should be cached (empty = all GET operations)
	CacheableOperations []string `yaml:"cacheable_operations" json:"cacheable_operations"`

	// Operations that should never be cached
	NonCacheableOperations []string `yaml:"non_cacheable_operations" json:"non_cacheable_operations"`
}

// CacheEntry represents a cached response
type CacheEntry struct {
	Key          string                 `json:"key"`
	URL          string                 `json:"url"`
	Method       string                 `json:"method"`
	Operation    string                 `json:"operation"`
	StatusCode   int                    `json:"status_code"`
	Headers      map[string]string      `json:"headers"`
	Body         []byte                 `json:"body"`
	ETag         string                 `json:"etag,omitempty"`
	LastModified string                 `json:"last_modified,omitempty"`
	CachedAt     time.Time              `json:"cached_at"`
	ExpiresAt    time.Time              `json:"expires_at"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// CacheKeyBuilder generates cache keys for requests
type CacheKeyBuilder struct {
	includeParams  bool
	includeHeaders []string
}

// NewCacheKeyBuilder creates a new cache key builder
func NewCacheKeyBuilder() *CacheKeyBuilder {
	return &CacheKeyBuilder{
		includeParams:  true,
		includeHeaders: []string{"Authorization", "Accept"}, // Include auth and accept headers in key
	}
}

// BuildKey builds a cache key from request details
func (ckb *CacheKeyBuilder) BuildKey(method, url, operation string, headers map[string]string) string {
	hasher := md5.New()

	// Include method and URL
	hasher.Write([]byte(method + ":" + url))

	// Include operation if provided
	if operation != "" {
		hasher.Write([]byte(":op:" + operation))
	}

	// Include selected headers
	for _, header := range ckb.includeHeaders {
		if value, exists := headers[header]; exists {
			hasher.Write([]byte(":hdr:" + header + "=" + value))
		}
	}

	return hex.EncodeToString(hasher.Sum(nil))
}

// JiraCacheRepository defines the interface for Jira response caching
type JiraCacheRepository interface {
	// Get retrieves a cached response
	Get(ctx context.Context, key string) (*CacheEntry, error)

	// Set stores a response with TTL
	Set(ctx context.Context, entry *CacheEntry) error

	// Invalidate removes cached entries by key
	Invalidate(ctx context.Context, key string) error

	// InvalidateByPattern removes cached entries matching a pattern
	InvalidateByPattern(ctx context.Context, pattern string) error

	// InvalidateByOperation removes cached entries for a specific operation
	InvalidateByOperation(ctx context.Context, operation string) error

	// GetStats returns cache statistics
	GetStats(ctx context.Context) (CacheStats, error)

	// Clear removes all cached entries
	Clear(ctx context.Context) error
}

// CacheStats represents cache statistics
type CacheStats struct {
	TotalEntries   int64     `json:"total_entries"`
	TotalSizeBytes int64     `json:"total_size_bytes"`
	HitCount       int64     `json:"hit_count"`
	MissCount      int64     `json:"miss_count"`
	HitRatio       float64   `json:"hit_ratio"`
	OldestEntry    time.Time `json:"oldest_entry"`
	NewestEntry    time.Time `json:"newest_entry"`
	ExpiredEntries int64     `json:"expired_entries"`
}

// InMemoryJiraCacheRepository is an in-memory implementation for development/testing
type InMemoryJiraCacheRepository struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	stats   CacheStats
}

// NewInMemoryJiraCacheRepository creates a new in-memory cache repository
func NewInMemoryJiraCacheRepository() JiraCacheRepository {
	return &InMemoryJiraCacheRepository{
		entries: make(map[string]*CacheEntry),
		stats:   CacheStats{},
	}
}

// Get retrieves a cached entry
func (r *InMemoryJiraCacheRepository) Get(ctx context.Context, key string) (*CacheEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.entries[key]
	if !exists {
		r.stats.MissCount++
		return nil, fmt.Errorf("cache entry not found")
	}

	// Check if entry is expired
	if time.Now().After(entry.ExpiresAt) {
		delete(r.entries, key)
		r.stats.MissCount++
		r.stats.ExpiredEntries++
		return nil, fmt.Errorf("cache entry expired")
	}

	r.stats.HitCount++
	r.updateHitRatio()
	return entry, nil
}

// Set stores a cache entry
func (r *InMemoryJiraCacheRepository) Set(ctx context.Context, entry *CacheEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries[entry.Key] = entry
	r.stats.TotalEntries = int64(len(r.entries))

	// Update size statistics
	entrySize := len(entry.Body) + len(entry.Key) + len(entry.URL)
	r.stats.TotalSizeBytes += int64(entrySize)

	// Update oldest/newest
	if r.stats.OldestEntry.IsZero() || entry.CachedAt.Before(r.stats.OldestEntry) {
		r.stats.OldestEntry = entry.CachedAt
	}
	if r.stats.NewestEntry.IsZero() || entry.CachedAt.After(r.stats.NewestEntry) {
		r.stats.NewestEntry = entry.CachedAt
	}

	return nil
}

// Invalidate removes a specific cache entry
func (r *InMemoryJiraCacheRepository) Invalidate(ctx context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.entries, key)
	r.stats.TotalEntries = int64(len(r.entries))
	return nil
}

// InvalidateByPattern removes entries matching a pattern
func (r *InMemoryJiraCacheRepository) InvalidateByPattern(ctx context.Context, pattern string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	keysToDelete := []string{}

	for key := range r.entries {
		if matched, _ := filepath.Match(pattern, key); matched {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		delete(r.entries, key)
	}

	r.stats.TotalEntries = int64(len(r.entries))
	return nil
}

// InvalidateByOperation removes entries for a specific operation
func (r *InMemoryJiraCacheRepository) InvalidateByOperation(ctx context.Context, operation string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	keysToDelete := []string{}

	for key, entry := range r.entries {
		if entry.Operation == operation {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		delete(r.entries, key)
	}

	r.stats.TotalEntries = int64(len(r.entries))
	return nil
}

// GetStats returns current cache statistics
func (r *InMemoryJiraCacheRepository) GetStats(ctx context.Context) (CacheStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.updateHitRatio()
	return r.stats, nil
}

// Clear removes all cache entries
func (r *InMemoryJiraCacheRepository) Clear(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = make(map[string]*CacheEntry)
	r.stats = CacheStats{}
	return nil
}

// updateHitRatio calculates the current hit ratio
func (r *InMemoryJiraCacheRepository) updateHitRatio() {
	total := r.stats.HitCount + r.stats.MissCount
	if total > 0 {
		r.stats.HitRatio = float64(r.stats.HitCount) / float64(total)
	}
}

// JiraCacheManager provides comprehensive caching for Jira operations
type JiraCacheManager struct {
	config     JiraCacheConfig
	repository JiraCacheRepository
	keyBuilder *CacheKeyBuilder
	logger     observability.Logger
	metrics    observability.MetricsClient

	// Operation-specific invalidation rules
	invalidationRules map[string][]string
}

// NewJiraCacheManager creates a new Jira cache manager
func NewJiraCacheManager(config JiraCacheConfig, repository JiraCacheRepository, logger observability.Logger) *JiraCacheManager {
	metrics := observability.DefaultMetricsClient

	jcm := &JiraCacheManager{
		config:            config,
		repository:        repository,
		keyBuilder:        NewCacheKeyBuilder(),
		logger:            logger,
		metrics:           metrics,
		invalidationRules: make(map[string][]string),
	}

	// Setup default invalidation rules
	jcm.setupDefaultInvalidationRules()

	return jcm
}

// setupDefaultInvalidationRules sets up operation-based cache invalidation rules
func (jcm *JiraCacheManager) setupDefaultInvalidationRules() {
	// When issues are created/updated, invalidate related caches
	jcm.invalidationRules["create_issue"] = []string{
		"search_issues", "get_projects", "get_project_*",
	}

	jcm.invalidationRules["update_issue"] = []string{
		"get_issue_*", "search_issues", "get_issue_transitions_*",
	}

	jcm.invalidationRules["delete_issue"] = []string{
		"get_issue_*", "search_issues",
	}

	// When comments are added/updated, invalidate issue caches
	jcm.invalidationRules["add_comment"] = []string{
		"get_issue_*", "get_comments_*",
	}

	jcm.invalidationRules["update_comment"] = []string{
		"get_issue_*", "get_comments_*",
	}

	// When workflows/transitions change, invalidate related caches
	jcm.invalidationRules["transition_issue"] = []string{
		"get_issue_*", "get_issue_transitions_*", "search_issues",
	}
}

// IsCacheable determines if an operation should be cached
func (jcm *JiraCacheManager) IsCacheable(method, operation string) bool {
	if !jcm.config.EnableResponseCaching {
		return false
	}

	// Only cache GET requests by default
	if method != "GET" {
		return false
	}

	// Check if operation is explicitly non-cacheable
	for _, nonCacheable := range jcm.config.NonCacheableOperations {
		if operation == nonCacheable {
			return false
		}
	}

	// If cacheable operations are specified, only cache those
	if len(jcm.config.CacheableOperations) > 0 {
		for _, cacheable := range jcm.config.CacheableOperations {
			if operation == cacheable {
				return true
			}
		}
		return false
	}

	// Cache all other GET operations by default
	return true
}

// GetTTLForOperation returns the TTL for a specific operation
func (jcm *JiraCacheManager) GetTTLForOperation(operation string) time.Duration {
	if ttl, exists := jcm.config.OperationTTLs[operation]; exists {
		return ttl
	}
	return jcm.config.DefaultTTL
}

// Get attempts to retrieve a cached response
func (jcm *JiraCacheManager) Get(ctx context.Context, method, url, operation string, headers map[string]string) (*CacheEntry, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		if jcm.metrics != nil {
			jcm.metrics.RecordCacheOperation("get", true, duration.Seconds())
		}
	}()

	if !jcm.IsCacheable(method, operation) {
		return nil, fmt.Errorf("operation not cacheable")
	}

	key := jcm.keyBuilder.BuildKey(method, url, operation, headers)

	entry, err := jcm.repository.Get(ctx, key)
	if err != nil {
		if jcm.metrics != nil {
			jcm.metrics.RecordCacheOperation("miss", true, 0)
		}
		return nil, err
	}

	if jcm.metrics != nil {
		jcm.metrics.RecordCacheOperation("hit", true, 0)
	}

	jcm.logger.Debug("Cache hit", map[string]interface{}{
		"operation": operation,
		"key":       key,
		"url":       url,
		"cached_at": entry.CachedAt,
	})

	return entry, nil
}

// Set stores a response in the cache
func (jcm *JiraCacheManager) Set(ctx context.Context, method, url, operation string, headers map[string]string, resp *http.Response, body []byte) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		if jcm.metrics != nil {
			jcm.metrics.RecordCacheOperation("set", true, duration.Seconds())
		}
	}()

	if !jcm.IsCacheable(method, operation) {
		return fmt.Errorf("operation not cacheable")
	}

	key := jcm.keyBuilder.BuildKey(method, url, operation, headers)
	ttl := jcm.GetTTLForOperation(operation)

	// Extract response headers
	respHeaders := make(map[string]string)
	for name, values := range resp.Header {
		if len(values) > 0 {
			respHeaders[name] = values[0]
		}
	}

	// Build cache entry
	entry := &CacheEntry{
		Key:        key,
		URL:        url,
		Method:     method,
		Operation:  operation,
		StatusCode: resp.StatusCode,
		Headers:    respHeaders,
		Body:       body,
		CachedAt:   time.Now(),
		ExpiresAt:  time.Now().Add(ttl),
		Metadata:   make(map[string]interface{}),
	}

	// Extract ETags if enabled and present
	if jcm.config.EnableETags {
		if etag := resp.Header.Get("ETag"); etag != "" {
			entry.ETag = etag
		}
		if lastModified := resp.Header.Get("Last-Modified"); lastModified != "" {
			entry.LastModified = lastModified
		}
	}

	// Add operation-specific metadata
	entry.Metadata["operation"] = operation
	entry.Metadata["ttl_seconds"] = ttl.Seconds()

	err := jcm.repository.Set(ctx, entry)
	if err != nil {
		jcm.logger.Warn("Failed to cache response", map[string]interface{}{
			"operation": operation,
			"key":       key,
			"error":     err.Error(),
		})
		return err
	}

	jcm.logger.Debug("Response cached", map[string]interface{}{
		"operation":  operation,
		"key":        key,
		"ttl":        ttl,
		"expires_at": entry.ExpiresAt,
		"has_etag":   entry.ETag != "",
	})

	return nil
}

// InvalidateByOperation invalidates cache entries for operations that modify data
func (jcm *JiraCacheManager) InvalidateByOperation(ctx context.Context, operation string) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		if jcm.metrics != nil {
			jcm.metrics.RecordCacheOperation("invalidate", true, duration.Seconds())
		}
	}()

	// Find operations to invalidate based on rules
	operationsToInvalidate := []string{operation}
	if rules, exists := jcm.invalidationRules[operation]; exists {
		operationsToInvalidate = append(operationsToInvalidate, rules...)
	}

	invalidatedCount := 0
	for _, op := range operationsToInvalidate {
		err := jcm.repository.InvalidateByOperation(ctx, op)
		if err != nil {
			jcm.logger.Warn("Failed to invalidate cache for operation", map[string]interface{}{
				"operation": op,
				"error":     err.Error(),
			})
		} else {
			invalidatedCount++
		}
	}

	jcm.logger.Debug("Cache invalidated by operation", map[string]interface{}{
		"trigger_operation":      operation,
		"invalidated_operations": operationsToInvalidate,
		"invalidated_count":      invalidatedCount,
	})

	return nil
}

// AddConditionalHeaders adds ETag/If-None-Match headers for conditional requests
func (jcm *JiraCacheManager) AddConditionalHeaders(req *http.Request, entry *CacheEntry) {
	if !jcm.config.EnableETags || entry == nil {
		return
	}

	if entry.ETag != "" {
		req.Header.Set("If-None-Match", entry.ETag)
	}

	if entry.LastModified != "" {
		req.Header.Set("If-Modified-Since", entry.LastModified)
	}
}

// HandleConditionalResponse handles 304 Not Modified responses
func (jcm *JiraCacheManager) HandleConditionalResponse(resp *http.Response, entry *CacheEntry) (*http.Response, []byte, bool) {
	if !jcm.config.EnableETags || resp.StatusCode != http.StatusNotModified || entry == nil {
		return resp, nil, false
	}

	// Update cache entry expiration
	ttl := jcm.GetTTLForOperation(entry.Operation)
	entry.ExpiresAt = time.Now().Add(ttl)

	// Create a proper 200 OK response from cached entry
	cachedResp := &http.Response{
		Status:        "200 OK",
		StatusCode:    http.StatusOK,
		Proto:         resp.Proto,
		ProtoMajor:    resp.ProtoMajor,
		ProtoMinor:    resp.ProtoMinor,
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(string(entry.Body))),
		ContentLength: int64(len(entry.Body)),
		Request:       resp.Request,
	}

	// Copy relevant headers from cached entry
	for key, value := range entry.Headers {
		cachedResp.Header.Set(key, value)
	}

	// Ensure Content-Type is set
	if cachedResp.Header.Get("Content-Type") == "" {
		cachedResp.Header.Set("Content-Type", "application/json")
	}

	jcm.logger.Debug("Reconstructed cached response for 304 Not Modified", map[string]interface{}{
		"operation": entry.Operation,
		"url":       entry.URL,
		"etag":      entry.ETag,
	})

	// Close the original 304 response body
	if resp.Body != nil {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail - this is cleanup
			if jcm.logger != nil {
				jcm.logger.Warn("Failed to close response body", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}
	}

	return cachedResp, entry.Body, true
}

// GetCacheStats returns current cache statistics
func (jcm *JiraCacheManager) GetCacheStats(ctx context.Context) (CacheStats, error) {
	return jcm.repository.GetStats(ctx)
}

// GetDefaultJiraCacheConfig returns default cache configuration
func GetDefaultJiraCacheConfig() JiraCacheConfig {
	return JiraCacheConfig{
		EnableResponseCaching: true,
		DefaultTTL:            5 * time.Minute,
		OperationTTLs: map[string]time.Duration{
			// Issue operations
			"get_issue":             10 * time.Minute,
			"search_issues":         2 * time.Minute,
			"get_issue_transitions": 30 * time.Minute,

			// Project operations
			"get_project":  30 * time.Minute,
			"get_projects": 15 * time.Minute,

			// Workflow operations (more stable)
			"get_workflows": 1 * time.Hour,
			"get_workflow":  1 * time.Hour,

			// Comments (frequently updated)
			"get_comments": 1 * time.Minute,

			// User/meta operations (fairly stable)
			"get_current_user": 10 * time.Minute,
			"get_server_info":  1 * time.Hour,
		},
		EnableETags:    true,
		MaxCacheSizeMB: 100, // 100MB default
		InvalidationPatterns: []string{
			"*issue*", // Invalidate anything with "issue" in operation name
		},
		CacheableOperations: []string{}, // Empty means all GET operations
		NonCacheableOperations: []string{
			"get_current_user_permissions", // User-specific, session-dependent
			"search_users",                 // Search results change frequently
		},
	}
}
