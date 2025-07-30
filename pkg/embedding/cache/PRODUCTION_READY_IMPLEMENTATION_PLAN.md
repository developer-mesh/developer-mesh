# Production-Ready Semantic Cache Implementation Plan

## Overview
This plan addresses all critical issues, high-priority improvements, and medium-priority enhancements identified in the principal engineer review. The implementation follows project best practices and provides detailed steps for Opus 4 to implement without ambiguity.

## Phase 1: Critical Issues (Must Complete Before Merge)

### 1.1 Remove TODO in semantic_cache.go:621

**Current State**: 
```go
// TODO: Implement LRU eviction
```

**Implementation**:
The LRU eviction is already implemented via the LRU manager. This TODO is outdated.

```go
// File: pkg/embedding/cache/semantic_cache.go
// Line: 621

// Remove the TODO comment and update the function documentation
func (c *SemanticCache) maintainCacheSize() error {
    // LRU eviction is handled by the LRU manager in TenantAwareCache
    // This method is kept for backwards compatibility
    return nil
}
```

### 1.2 Fix Vector Store Test Mocks

**Current Issue**: Tests expect []byte but implementation uses []float32

**Implementation**:

```go
// File: pkg/embedding/cache/vector_store_test.go

func TestVectorStore_StoreCacheEmbedding(t *testing.T) {
    db, mock := setupMockDB(t)
    defer func() { _ = db.Close() }()

    logger := observability.NewLogger("test")
    metrics := observability.NewMetricsClient()
    store := cache.NewVectorStore(db, logger, metrics)

    tenantID := uuid.New()
    cacheKey := "test_key"
    queryHash := "test_hash"
    embedding := []float32{0.1, 0.2, 0.3}

    // Convert float32 slice to PostgreSQL array format
    embeddingArray := pq.Array(embedding)
    
    mock.ExpectExec("INSERT INTO cache_metadata").
        WithArgs(
            tenantID,
            cacheKey,
            queryHash,
            embeddingArray, // Use pq.Array for proper type conversion
        ).
        WillReturnResult(sqlmock.NewResult(1, 1))

    err := store.StoreCacheEmbedding(context.Background(), tenantID, cacheKey, queryHash, embedding)
    assert.NoError(t, err)
    assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVectorStore_FindSimilarQueries(t *testing.T) {
    db, mock := setupMockDB(t)
    defer func() { _ = db.Close() }()

    // ... setup code ...

    embedding := []float32{0.1, 0.2, 0.3}
    embeddingArray := pq.Array(embedding)

    rows := sqlmock.NewRows([]string{"cache_key", "query_hash", "similarity", "hit_count", "last_accessed_at"}).
        AddRow("key1", "hash1", 0.95, 10, time.Now()).
        AddRow("key2", "hash2", 0.85, 5, time.Now())

    mock.ExpectQuery("SELECT cache_key, query_hash").
        WithArgs(tenantID, embeddingArray, threshold, limit).
        WillReturnRows(rows)

    results, err := store.FindSimilarQueries(context.Background(), tenantID, embedding, threshold, limit)
    require.NoError(t, err)
    assert.Len(t, results, 2)
    assert.Equal(t, "key1", results[0].CacheKey)
    assert.Equal(t, float32(0.95), results[0].Similarity)
}
```

### 1.3 Add Structured Logging with Sensitive Data Redaction

**Implementation**:

```go
// File: pkg/embedding/cache/logging.go
package cache

import (
    "regexp"
    "strings"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
)

// SensitiveDataRedactor redacts sensitive information from log messages
type SensitiveDataRedactor struct {
    patterns []*regexp.Regexp
}

// NewSensitiveDataRedactor creates a new redactor with default patterns
func NewSensitiveDataRedactor() *SensitiveDataRedactor {
    patterns := []*regexp.Regexp{
        regexp.MustCompile(`(?i)(api[_-]?key|apikey)(["\s:=]+)([^"\s,}]+)`),
        regexp.MustCompile(`(?i)(password|passwd|pwd)(["\s:=]+)([^"\s,}]+)`),
        regexp.MustCompile(`(?i)(secret[_-]?key|secret)(["\s:=]+)([^"\s,}]+)`),
        regexp.MustCompile(`(?i)(token|access[_-]?token|refresh[_-]?token)(["\s:=]+)([^"\s,}]+)`),
        regexp.MustCompile(`(?i)(private[_-]?key)(["\s:=]+)([^"\s,}]+)`),
        regexp.MustCompile(`\b\d{3}-?\d{2}-?\d{4}\b`), // SSN
        regexp.MustCompile(`\b\d{13,19}\b`), // Credit card
    }
    
    return &SensitiveDataRedactor{
        patterns: patterns,
    }
}

// Redact removes sensitive data from a string
func (r *SensitiveDataRedactor) Redact(input string) string {
    result := input
    for _, pattern := range r.patterns {
        result = pattern.ReplaceAllString(result, "${1}${2}[REDACTED]")
    }
    return result
}

// RedactMap redacts sensitive data from a map
func (r *SensitiveDataRedactor) RedactMap(data map[string]interface{}) map[string]interface{} {
    result := make(map[string]interface{})
    sensitiveKeys := []string{
        "api_key", "apikey", "api-key",
        "password", "passwd", "pwd",
        "secret", "secret_key", "secret-key",
        "token", "access_token", "refresh_token",
        "private_key", "private-key",
        "ssn", "social_security_number",
        "credit_card", "card_number",
    }
    
    for k, v := range data {
        lowerKey := strings.ToLower(k)
        isSensitive := false
        
        for _, sensitiveKey := range sensitiveKeys {
            if strings.Contains(lowerKey, sensitiveKey) {
                isSensitive = true
                break
            }
        }
        
        if isSensitive {
            result[k] = "[REDACTED]"
        } else if str, ok := v.(string); ok {
            result[k] = r.Redact(str)
        } else {
            result[k] = v
        }
    }
    
    return result
}

// SafeLogger wraps the observability logger with redaction
type SafeLogger struct {
    logger   observability.Logger
    redactor *SensitiveDataRedactor
}

// NewSafeLogger creates a logger that redacts sensitive data
func NewSafeLogger(name string) *SafeLogger {
    return &SafeLogger{
        logger:   observability.NewLogger(name),
        redactor: NewSensitiveDataRedactor(),
    }
}

// Error logs an error with redacted fields
func (l *SafeLogger) Error(msg string, fields map[string]interface{}) {
    l.logger.Error(l.redactor.Redact(msg), l.redactor.RedactMap(fields))
}

// Warn logs a warning with redacted fields
func (l *SafeLogger) Warn(msg string, fields map[string]interface{}) {
    l.logger.Warn(l.redactor.Redact(msg), l.redactor.RedactMap(fields))
}

// Info logs info with redacted fields
func (l *SafeLogger) Info(msg string, fields map[string]interface{}) {
    l.logger.Info(l.redactor.Redact(msg), l.redactor.RedactMap(fields))
}

// Debug logs debug with redacted fields
func (l *SafeLogger) Debug(msg string, fields map[string]interface{}) {
    l.logger.Debug(l.redactor.Redact(msg), l.redactor.RedactMap(fields))
}
```

Update all cache components to use SafeLogger:

```go
// File: pkg/embedding/cache/semantic_cache.go
// Update NewSemanticCache function

if logger == nil {
    logger = NewSafeLogger("embedding.cache")
}

// File: pkg/embedding/cache/tenant_cache.go
// Update NewTenantAwareCache function

if logger == nil {
    logger = NewSafeLogger("embedding.cache.tenant")
}
```

### 1.4 Configure Redis Connection Pools

**Implementation**:

```go
// File: pkg/embedding/cache/config.go

// Add to Config struct
type Config struct {
    // ... existing fields ...
    
    // Connection pool configuration
    ConnectionPool ConnectionPoolConfig `json:"connection_pool" yaml:"connection_pool"`
}

type ConnectionPoolConfig struct {
    // Maximum number of socket connections
    PoolSize int `json:"pool_size" yaml:"pool_size"`
    // Minimum number of idle connections
    MinIdleConns int `json:"min_idle_conns" yaml:"min_idle_conns"`
    // Maximum number of retries
    MaxRetries int `json:"max_retries" yaml:"max_retries"`
    // Dial timeout
    DialTimeout time.Duration `json:"dial_timeout" yaml:"dial_timeout"`
    // Read timeout
    ReadTimeout time.Duration `json:"read_timeout" yaml:"read_timeout"`
    // Write timeout
    WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`
    // Connection age timeout
    MaxConnAge time.Duration `json:"max_conn_age" yaml:"max_conn_age"`
    // Idle timeout
    IdleTimeout time.Duration `json:"idle_timeout" yaml:"idle_timeout"`
}

// DefaultConnectionPoolConfig returns production-ready pool configuration
func DefaultConnectionPoolConfig() ConnectionPoolConfig {
    return ConnectionPoolConfig{
        PoolSize:     10 * runtime.GOMAXPROCS(0), // 10 connections per CPU
        MinIdleConns: runtime.GOMAXPROCS(0),      // 1 idle connection per CPU
        MaxRetries:   3,
        DialTimeout:  5 * time.Second,
        ReadTimeout:  3 * time.Second,
        WriteTimeout: 3 * time.Second,
        MaxConnAge:   0, // No maximum age
        IdleTimeout:  5 * time.Minute,
    }
}

// Update DefaultConfig to include pool config
func DefaultConfig() *Config {
    return &Config{
        // ... existing fields ...
        ConnectionPool: DefaultConnectionPoolConfig(),
    }
}
```

Create a Redis client factory:

```go
// File: pkg/embedding/cache/redis_factory.go
package cache

import (
    "github.com/go-redis/redis/v8"
)

// NewRedisClient creates a Redis client with proper connection pool configuration
func NewRedisClient(addr string, config ConnectionPoolConfig) *redis.Client {
    return redis.NewClient(&redis.Options{
        Addr:         addr,
        PoolSize:     config.PoolSize,
        MinIdleConns: config.MinIdleConns,
        MaxRetries:   config.MaxRetries,
        DialTimeout:  config.DialTimeout,
        ReadTimeout:  config.ReadTimeout,
        WriteTimeout: config.WriteTimeout,
        MaxConnAge:   config.MaxConnAge,
        IdleTimeout:  config.IdleTimeout,
        // Use exponential backoff for retries
        MaxRetryBackoff: 512 * time.Millisecond,
        MinRetryBackoff: 8 * time.Millisecond,
    })
}
```

## Phase 2: High Priority Improvements

### 2.1 Per-Tenant Encryption Keys with Rotation Support

**Implementation**:

```go
// File: pkg/embedding/cache/encryption/tenant_key_manager.go
package encryption

import (
    "context"
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "sync"
    "time"
    
    "github.com/google/uuid"
    "github.com/developer-mesh/developer-mesh/pkg/repository"
    "github.com/developer-mesh/developer-mesh/pkg/security"
)

// TenantKey represents an encryption key for a tenant
type TenantKey struct {
    TenantID    uuid.UUID `db:"tenant_id"`
    KeyID       string    `db:"key_id"`
    EncryptedKey string   `db:"encrypted_key"`
    CreatedAt   time.Time `db:"created_at"`
    ExpiresAt   time.Time `db:"expires_at"`
    IsActive    bool      `db:"is_active"`
}

// TenantKeyManager manages per-tenant encryption keys
type TenantKeyManager struct {
    repo            repository.TenantKeyRepository
    masterKeyID     string
    encryptionSvc   *security.EncryptionService
    keyCache        sync.Map // map[tenantID]map[keyID]*decryptedKey
    rotationPeriod  time.Duration
    mu              sync.RWMutex
}

// NewTenantKeyManager creates a new tenant key manager
func NewTenantKeyManager(
    repo repository.TenantKeyRepository,
    masterKeyID string,
    rotationPeriod time.Duration,
) *TenantKeyManager {
    return &TenantKeyManager{
        repo:           repo,
        masterKeyID:    masterKeyID,
        encryptionSvc:  security.NewEncryptionService(masterKeyID),
        rotationPeriod: rotationPeriod,
    }
}

// GetOrCreateKey gets the active encryption key for a tenant
func (m *TenantKeyManager) GetOrCreateKey(ctx context.Context, tenantID uuid.UUID) (string, error) {
    // Check cache first
    if cached, ok := m.keyCache.Load(tenantID); ok {
        keys := cached.(map[string]string)
        for _, key := range keys {
            return key, nil
        }
    }
    
    // Get active key from database
    activeKey, err := m.repo.GetActiveKey(ctx, tenantID)
    if err == nil && activeKey != nil {
        decrypted, err := m.decryptKey(activeKey.EncryptedKey)
        if err != nil {
            return "", fmt.Errorf("failed to decrypt tenant key: %w", err)
        }
        
        // Cache the key
        m.cacheKey(tenantID, activeKey.KeyID, decrypted)
        return decrypted, nil
    }
    
    // Create new key if none exists
    return m.createNewKey(ctx, tenantID)
}

// RotateKey creates a new key for the tenant and marks old keys as inactive
func (m *TenantKeyManager) RotateKey(ctx context.Context, tenantID uuid.UUID) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // Create new key
    newKey, err := m.createNewKey(ctx, tenantID)
    if err != nil {
        return fmt.Errorf("failed to create new key: %w", err)
    }
    
    // Mark old keys as inactive
    if err := m.repo.DeactivateKeys(ctx, tenantID); err != nil {
        return fmt.Errorf("failed to deactivate old keys: %w", err)
    }
    
    // Clear cache for this tenant
    m.keyCache.Delete(tenantID)
    
    return nil
}

// createNewKey generates a new encryption key for a tenant
func (m *TenantKeyManager) createNewKey(ctx context.Context, tenantID uuid.UUID) (string, error) {
    // Generate 256-bit key
    keyBytes := make([]byte, 32)
    if _, err := rand.Read(keyBytes); err != nil {
        return "", fmt.Errorf("failed to generate key: %w", err)
    }
    
    key := base64.StdEncoding.EncodeToString(keyBytes)
    keyID := uuid.New().String()
    
    // Encrypt key with master key
    encrypted, err := m.encryptionSvc.EncryptCredential(key, m.masterKeyID)
    if err != nil {
        return "", fmt.Errorf("failed to encrypt key: %w", err)
    }
    
    // Store in database
    tenantKey := &TenantKey{
        TenantID:     tenantID,
        KeyID:        keyID,
        EncryptedKey: base64.StdEncoding.EncodeToString(encrypted),
        CreatedAt:    time.Now(),
        ExpiresAt:    time.Now().Add(m.rotationPeriod),
        IsActive:     true,
    }
    
    if err := m.repo.CreateKey(ctx, tenantKey); err != nil {
        return "", fmt.Errorf("failed to store key: %w", err)
    }
    
    // Cache the key
    m.cacheKey(tenantID, keyID, key)
    
    return key, nil
}

// decryptKey decrypts a key using the master key
func (m *TenantKeyManager) decryptKey(encryptedKey string) (string, error) {
    encrypted, err := base64.StdEncoding.DecodeString(encryptedKey)
    if err != nil {
        return "", fmt.Errorf("failed to decode key: %w", err)
    }
    
    decrypted, err := m.encryptionSvc.DecryptCredential(encrypted, m.masterKeyID)
    if err != nil {
        return "", fmt.Errorf("failed to decrypt key: %w", err)
    }
    
    return decrypted, nil
}

// cacheKey stores a decrypted key in the cache
func (m *TenantKeyManager) cacheKey(tenantID uuid.UUID, keyID, key string) {
    keys, _ := m.keyCache.LoadOrStore(tenantID, make(map[string]string))
    keyMap := keys.(map[string]string)
    keyMap[keyID] = key
}

// StartRotationScheduler starts a background job to rotate expired keys
func (m *TenantKeyManager) StartRotationScheduler(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Hour)
    go func() {
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                m.rotateExpiredKeys(ctx)
            }
        }
    }()
}

// rotateExpiredKeys finds and rotates all expired keys
func (m *TenantKeyManager) rotateExpiredKeys(ctx context.Context) {
    expiredKeys, err := m.repo.GetExpiredKeys(ctx)
    if err != nil {
        // Log error but don't stop
        return
    }
    
    for _, key := range expiredKeys {
        if err := m.RotateKey(ctx, key.TenantID); err != nil {
            // Log error but continue with other tenants
            continue
        }
    }
}
```

Update TenantAwareCache to use per-tenant keys:

```go
// File: pkg/embedding/cache/tenant_cache.go

// Add to TenantAwareCache struct
type TenantAwareCache struct {
    // ... existing fields ...
    keyManager *encryption.TenantKeyManager
}

// Update NewTenantAwareCache
func NewTenantAwareCache(
    baseCache *SemanticCache,
    configRepo repository.TenantConfigRepository,
    keyRepo repository.TenantKeyRepository, // Add this parameter
    rateLimiter *middleware.RateLimiter,
    masterKeyID string, // Changed from encryptionKey
    logger observability.Logger,
    metrics observability.MetricsClient,
) *TenantAwareCache {
    // ... existing code ...
    
    // Create key manager
    keyManager := encryption.NewTenantKeyManager(
        keyRepo,
        masterKeyID,
        30 * 24 * time.Hour, // 30 day rotation period
    )
    
    cache := &TenantAwareCache{
        // ... existing fields ...
        keyManager: keyManager,
        // Remove encryptionService - we'll create per-request
    }
    
    // Start key rotation scheduler
    keyManager.StartRotationScheduler(context.Background())
    
    return cache
}

// Update encryption methods to use tenant-specific keys
func (tc *TenantAwareCache) getEncryptionService(ctx context.Context, tenantID uuid.UUID) (*security.EncryptionService, error) {
    key, err := tc.keyManager.GetOrCreateKey(ctx, tenantID)
    if err != nil {
        return nil, fmt.Errorf("failed to get tenant key: %w", err)
    }
    
    return security.NewEncryptionService(key), nil
}
```

### 2.2 Configurable Buffer Sizes and Timeouts

**Implementation**:

```go
// File: pkg/embedding/cache/config.go

// Update Config struct
type Config struct {
    // ... existing fields ...
    
    // Performance tuning
    Performance PerformanceConfig `json:"performance" yaml:"performance"`
}

type PerformanceConfig struct {
    // LRU tracker buffer size
    TrackerBufferSize int `json:"tracker_buffer_size" yaml:"tracker_buffer_size"`
    
    // Batch processing configuration
    BatchSize     int           `json:"batch_size" yaml:"batch_size"`
    FlushInterval time.Duration `json:"flush_interval" yaml:"flush_interval"`
    
    // Operation timeouts
    GetTimeout    time.Duration `json:"get_timeout" yaml:"get_timeout"`
    SetTimeout    time.Duration `json:"set_timeout" yaml:"set_timeout"`
    DeleteTimeout time.Duration `json:"delete_timeout" yaml:"delete_timeout"`
    
    // Shutdown timeout
    ShutdownTimeout time.Duration `json:"shutdown_timeout" yaml:"shutdown_timeout"`
    
    // Compression threshold
    CompressionThreshold int `json:"compression_threshold" yaml:"compression_threshold"`
}

// DefaultPerformanceConfig returns performance configuration based on system resources
func DefaultPerformanceConfig() PerformanceConfig {
    numCPU := runtime.GOMAXPROCS(0)
    
    return PerformanceConfig{
        TrackerBufferSize:    10000 * numCPU, // Scale with CPU count
        BatchSize:           100,
        FlushInterval:       100 * time.Millisecond,
        GetTimeout:          1 * time.Second,
        SetTimeout:          2 * time.Second,
        DeleteTimeout:       1 * time.Second,
        ShutdownTimeout:     30 * time.Second,
        CompressionThreshold: 1024, // 1KB
    }
}

// PerformanceProfile defines preset configurations
type PerformanceProfile string

const (
    PerformanceProfileLowLatency  PerformanceProfile = "low-latency"
    PerformanceProfileHighThroughput PerformanceProfile = "high-throughput"
    PerformanceProfileBalanced    PerformanceProfile = "balanced"
)

// GetPerformanceConfig returns configuration for a specific profile
func GetPerformanceConfig(profile PerformanceProfile) PerformanceConfig {
    base := DefaultPerformanceConfig()
    
    switch profile {
    case PerformanceProfileLowLatency:
        base.TrackerBufferSize = 1000
        base.BatchSize = 10
        base.FlushInterval = 10 * time.Millisecond
        base.GetTimeout = 500 * time.Millisecond
        base.SetTimeout = 1 * time.Second
        
    case PerformanceProfileHighThroughput:
        base.TrackerBufferSize = 100000
        base.BatchSize = 1000
        base.FlushInterval = 500 * time.Millisecond
        base.GetTimeout = 5 * time.Second
        base.SetTimeout = 10 * time.Second
        
    case PerformanceProfileBalanced:
        // Use defaults
    }
    
    return base
}
```

Update components to use configurable values:

```go
// File: pkg/embedding/cache/lru/tracker.go

// Update NewAsyncTracker
func NewAsyncTracker(
    client RedisClient,
    config *Config,
    logger observability.Logger,
    metrics observability.MetricsClient,
) *AsyncTracker {
    // Use configured buffer size
    bufferSize := config.Performance.TrackerBufferSize
    if bufferSize <= 0 {
        bufferSize = 10000 // fallback
    }
    
    t := &AsyncTracker{
        client:        client,
        config:        config,
        updates:       make(chan accessUpdate, bufferSize),
        pendingAccess: make(map[tenantKey][]time.Time),
        logger:        logger,
        metrics:       metrics,
        stopCh:        make(chan struct{}),
    }
    
    // ... rest of initialization
}

// File: pkg/embedding/cache/compression.go

// Update NewCompressionService
func NewCompressionService(encryptionKey string, threshold int) *CompressionService {
    if threshold <= 0 {
        threshold = 1024 // default 1KB
    }
    
    return &CompressionService{
        encryptionService: security.NewEncryptionService(encryptionKey),
        compressionLevel:  gzip.BestSpeed,
        minSizeBytes:      threshold,
    }
}

// File: pkg/embedding/cache/semantic_cache.go

// Add timeout enforcement to operations
func (c *SemanticCache) Get(ctx context.Context, query string, embedding []float32) (*CacheEntry, error) {
    // Apply timeout from config
    timeoutCtx, cancel := context.WithTimeout(ctx, c.config.Performance.GetTimeout)
    defer cancel()
    
    // ... rest of method uses timeoutCtx
}
```

### 2.3 Configurable Retry and Circuit Breaker Policies

**Implementation**:

```go
// File: pkg/embedding/cache/resilience/config.go
package resilience

import (
    "time"
    "github.com/developer-mesh/developer-mesh/pkg/resilience"
)

// ResilienceConfig configures retry and circuit breaker behavior
type ResilienceConfig struct {
    Retry          RetryConfig          `json:"retry" yaml:"retry"`
    CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker" yaml:"circuit_breaker"`
}

// RetryConfig configures retry behavior
type RetryConfig struct {
    MaxAttempts     int           `json:"max_attempts" yaml:"max_attempts"`
    InitialInterval time.Duration `json:"initial_interval" yaml:"initial_interval"`
    MaxInterval     time.Duration `json:"max_interval" yaml:"max_interval"`
    Multiplier      float64       `json:"multiplier" yaml:"multiplier"`
    MaxElapsedTime  time.Duration `json:"max_elapsed_time" yaml:"max_elapsed_time"`
    
    // Retry only on specific errors
    RetryableErrors []string `json:"retryable_errors" yaml:"retryable_errors"`
}

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
    // Number of consecutive failures before opening
    FailureThreshold int `json:"failure_threshold" yaml:"failure_threshold"`
    
    // Success threshold to close circuit
    SuccessThreshold int `json:"success_threshold" yaml:"success_threshold"`
    
    // Timeout before attempting half-open
    Timeout time.Duration `json:"timeout" yaml:"timeout"`
    
    // Maximum number of requests in half-open state
    MaxRequests int `json:"max_requests" yaml:"max_requests"`
}

// DefaultResilienceConfig returns production-ready resilience configuration
func DefaultResilienceConfig() ResilienceConfig {
    return ResilienceConfig{
        Retry: RetryConfig{
            MaxAttempts:     3,
            InitialInterval: 100 * time.Millisecond,
            MaxInterval:     2 * time.Second,
            Multiplier:      2.0,
            MaxElapsedTime:  10 * time.Second,
            RetryableErrors: []string{
                "connection refused",
                "i/o timeout",
                "context deadline exceeded",
            },
        },
        CircuitBreaker: CircuitBreakerConfig{
            FailureThreshold: 5,
            SuccessThreshold: 2,
            Timeout:         30 * time.Second,
            MaxRequests:     10,
        },
    }
}

// BuildRetryPolicy creates a retry policy from configuration
func BuildRetryPolicy(config RetryConfig) *resilience.RetryPolicy {
    return &resilience.RetryPolicy{
        MaxAttempts:     config.MaxAttempts,
        InitialInterval: config.InitialInterval,
        MaxInterval:     config.MaxInterval,
        Multiplier:      config.Multiplier,
        MaxElapsedTime:  config.MaxElapsedTime,
        RetryIf: func(err error) bool {
            if err == nil {
                return false
            }
            
            errStr := err.Error()
            for _, retryable := range config.RetryableErrors {
                if strings.Contains(errStr, retryable) {
                    return true
                }
            }
            
            return false
        },
    }
}

// BuildCircuitBreaker creates a circuit breaker from configuration
func BuildCircuitBreaker(name string, config CircuitBreakerConfig) *resilience.CircuitBreaker {
    return resilience.NewCircuitBreaker(
        name,
        config.FailureThreshold,
        config.SuccessThreshold,
        config.Timeout,
        resilience.WithMaxRequests(config.MaxRequests),
    )
}
```

Update ResilientRedisClient to use configuration:

```go
// File: pkg/embedding/cache/redis_client.go

// Update NewResilientRedisClient
func NewResilientRedisClient(
    client *redis.Client,
    config ResilienceConfig, // Add parameter
    logger observability.Logger,
    metrics observability.MetricsClient,
) *ResilientRedisClient {
    retryPolicy := resilience.BuildRetryPolicy(config.Retry)
    circuitBreaker := resilience.BuildCircuitBreaker("redis_cache", config.CircuitBreaker)
    
    return &ResilientRedisClient{
        client:         client,
        retryPolicy:    retryPolicy,
        circuitBreaker: circuitBreaker,
        logger:         logger,
        metrics:        metrics,
    }
}
```

### 2.4 Health Check Endpoints

**Implementation**:

```go
// File: pkg/embedding/cache/health/health_check.go
package health

import (
    "context"
    "time"
    
    "github.com/developer-mesh/developer-mesh/pkg/embedding/cache"
)

// HealthStatus represents the health of the cache system
type HealthStatus struct {
    Status     string                 `json:"status"` // healthy, degraded, unhealthy
    Components map[string]ComponentHealth `json:"components"`
    Timestamp  time.Time             `json:"timestamp"`
}

// ComponentHealth represents health of a single component
type ComponentHealth struct {
    Status  string                 `json:"status"`
    Message string                 `json:"message,omitempty"`
    Details map[string]interface{} `json:"details,omitempty"`
}

// CacheHealthChecker checks the health of cache components
type CacheHealthChecker struct {
    cache       *cache.TenantAwareCache
    redisClient cache.RedisClient
    vectorStore cache.VectorStore
}

// NewCacheHealthChecker creates a new health checker
func NewCacheHealthChecker(
    cache *cache.TenantAwareCache,
    redisClient cache.RedisClient,
    vectorStore cache.VectorStore,
) *CacheHealthChecker {
    return &CacheHealthChecker{
        cache:       cache,
        redisClient: redisClient,
        vectorStore: vectorStore,
    }
}

// Check performs a comprehensive health check
func (c *CacheHealthChecker) Check(ctx context.Context) (*HealthStatus, error) {
    health := &HealthStatus{
        Status:     "healthy",
        Components: make(map[string]ComponentHealth),
        Timestamp:  time.Now(),
    }
    
    // Check Redis
    redisHealth := c.checkRedis(ctx)
    health.Components["redis"] = redisHealth
    if redisHealth.Status != "healthy" {
        health.Status = "degraded"
    }
    
    // Check Vector Store
    if c.vectorStore != nil {
        vectorHealth := c.checkVectorStore(ctx)
        health.Components["vector_store"] = vectorHealth
        if vectorHealth.Status != "healthy" {
            health.Status = "degraded"
        }
    }
    
    // Check LRU Manager
    lruHealth := c.checkLRUManager(ctx)
    health.Components["lru_manager"] = lruHealth
    if lruHealth.Status != "healthy" {
        health.Status = "degraded"
    }
    
    // Check Circuit Breaker
    cbHealth := c.checkCircuitBreaker()
    health.Components["circuit_breaker"] = cbHealth
    if cbHealth.Status == "open" {
        health.Status = "unhealthy"
    }
    
    return health, nil
}

// checkRedis verifies Redis connectivity
func (c *CacheHealthChecker) checkRedis(ctx context.Context) ComponentHealth {
    start := time.Now()
    
    // Try a simple ping
    err := c.redisClient.Execute(ctx, func() (interface{}, error) {
        // Implement ping in RedisClient interface
        return nil, c.redisClient.(*cache.ResilientRedisClient).Ping(ctx)
    })
    
    latency := time.Since(start)
    
    if err != nil {
        return ComponentHealth{
            Status:  "unhealthy",
            Message: err.Error(),
            Details: map[string]interface{}{
                "latency_ms": latency.Milliseconds(),
            },
        }
    }
    
    status := "healthy"
    if latency > 100*time.Millisecond {
        status = "degraded"
    }
    
    return ComponentHealth{
        Status: status,
        Details: map[string]interface{}{
            "latency_ms": latency.Milliseconds(),
        },
    }
}

// checkVectorStore verifies pgvector connectivity
func (c *CacheHealthChecker) checkVectorStore(ctx context.Context) ComponentHealth {
    start := time.Now()
    
    // Try a simple query
    err := c.vectorStore.HealthCheck(ctx)
    
    latency := time.Since(start)
    
    if err != nil {
        return ComponentHealth{
            Status:  "unhealthy",
            Message: err.Error(),
            Details: map[string]interface{}{
                "latency_ms": latency.Milliseconds(),
            },
        }
    }
    
    return ComponentHealth{
        Status: "healthy",
        Details: map[string]interface{}{
            "latency_ms": latency.Milliseconds(),
        },
    }
}

// checkLRUManager verifies LRU manager is functioning
func (c *CacheHealthChecker) checkLRUManager(ctx context.Context) ComponentHealth {
    if c.cache.GetLRUManager() == nil {
        return ComponentHealth{
            Status:  "disabled",
            Message: "LRU manager not configured",
        }
    }
    
    // Get stats to verify it's working
    stats := c.cache.GetLRUManager().GetStats()
    
    return ComponentHealth{
        Status: "healthy",
        Details: map[string]interface{}{
            "tracking_enabled": stats["tracking_enabled"],
            "tenants_tracked":  stats["tenants_tracked"],
        },
    }
}

// checkCircuitBreaker checks circuit breaker state
func (c *CacheHealthChecker) checkCircuitBreaker() ComponentHealth {
    // Get circuit breaker state from resilient client
    state := c.redisClient.(*cache.ResilientRedisClient).GetCircuitBreakerState()
    
    return ComponentHealth{
        Status: string(state),
        Details: map[string]interface{}{
            "state": state,
        },
    }
}
```

Add health check endpoint to router:

```go
// File: pkg/embedding/cache/integration/router.go

// Add to SetupRoutes
readGroup.GET("/health", cr.handleHealthCheck)
readGroup.GET("/health/detailed", middleware.RequireTenantMiddleware(), cr.handleDetailedHealthCheck)

// Add handler methods
func (cr *CacheRouter) handleHealthCheck(c *gin.Context) {
    checker := health.NewCacheHealthChecker(
        cr.tenantCache,
        cr.tenantCache.GetRedisClient(),
        cr.tenantCache.GetVectorStore(),
    )
    
    ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
    defer cancel()
    
    status, err := checker.Check(ctx)
    if err != nil {
        c.JSON(500, gin.H{"error": "health check failed", "details": err.Error()})
        return
    }
    
    httpStatus := 200
    if status.Status == "unhealthy" {
        httpStatus = 503
    } else if status.Status == "degraded" {
        httpStatus = 207
    }
    
    c.JSON(httpStatus, status)
}
```

## Phase 3: Medium Priority Enhancements

### 3.1 Define Interfaces for Main Components

**Implementation**:

```go
// File: pkg/embedding/cache/interfaces.go
package cache

import (
    "context"
    "time"
    
    "github.com/google/uuid"
)

// Cache defines the interface for semantic cache operations
type Cache interface {
    // Core operations
    Get(ctx context.Context, query string, embedding []float32) (*CacheEntry, error)
    Set(ctx context.Context, query string, embedding []float32, results []CachedSearchResult) error
    Delete(ctx context.Context, query string) error
    
    // Batch operations
    GetBatch(ctx context.Context, queries []string, embeddings [][]float32) ([]*CacheEntry, error)
    
    // Management
    Clear(ctx context.Context) error
    GetStats(ctx context.Context) (*CacheStats, error)
    Shutdown(ctx context.Context) error
}

// TenantCache extends Cache with tenant-aware operations
type TenantCache interface {
    Cache
    
    // Tenant-specific operations
    GetWithTenant(ctx context.Context, tenantID uuid.UUID, query string, embedding []float32) (*CacheEntry, error)
    SetWithTenant(ctx context.Context, tenantID uuid.UUID, query string, embedding []float32, results []CachedSearchResult) error
    DeleteWithTenant(ctx context.Context, tenantID uuid.UUID, query string) error
    
    // Tenant management
    ClearTenant(ctx context.Context, tenantID uuid.UUID) error
    GetTenantStats(ctx context.Context, tenantID uuid.UUID) (*TenantCacheStats, error)
    GetTenantConfig(ctx context.Context, tenantID uuid.UUID) (*TenantCacheConfig, error)
    
    // LRU operations
    GetLRUManager() LRUManager
}

// LRUManager defines the interface for LRU eviction management
type LRUManager interface {
    // Eviction operations
    EvictForTenant(ctx context.Context, tenantID uuid.UUID, targetBytes int64) error
    EvictGlobal(ctx context.Context, targetBytes int64) error
    
    // Tracking operations
    TrackAccess(tenantID uuid.UUID, key string)
    GetAccessScore(ctx context.Context, tenantID uuid.UUID, key string) (float64, error)
    GetLRUKeys(ctx context.Context, tenantID uuid.UUID, limit int) ([]string, error)
    
    // Statistics
    GetStats() map[string]interface{}
    GetTenantStats(ctx context.Context, tenantID uuid.UUID) (*LRUStats, error)
}

// CacheStore defines low-level cache storage operations
type CacheStore interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    TTL(ctx context.Context, key string) (time.Duration, error)
    
    // Batch operations
    MGet(ctx context.Context, keys []string) ([][]byte, error)
    MSet(ctx context.Context, items map[string][]byte, ttl time.Duration) error
    
    // Pattern operations
    Keys(ctx context.Context, pattern string) ([]string, error)
    DeletePattern(ctx context.Context, pattern string) error
}

// VectorSearchEngine defines vector similarity search operations
type VectorSearchEngine interface {
    // Storage operations
    StoreEmbedding(ctx context.Context, id string, embedding []float32, metadata map[string]interface{}) error
    DeleteEmbedding(ctx context.Context, id string) error
    
    // Search operations
    SearchSimilar(ctx context.Context, embedding []float32, threshold float32, limit int) ([]SearchResult, error)
    SearchSimilarWithFilter(ctx context.Context, embedding []float32, threshold float32, limit int, filter map[string]interface{}) ([]SearchResult, error)
    
    // Management
    GetIndexStats(ctx context.Context) (*IndexStats, error)
    OptimizeIndex(ctx context.Context) error
}

// CompressionEngine defines compression operations
type CompressionEngine interface {
    Compress(data []byte) ([]byte, error)
    Decompress(data []byte) ([]byte, error)
    IsCompressed(data []byte) bool
    GetCompressionRatio(data []byte) (float64, error)
}

// EncryptionEngine defines encryption operations
type EncryptionEngine interface {
    Encrypt(data []byte, context string) ([]byte, error)
    Decrypt(data []byte, context string) ([]byte, error)
    RotateKey(oldContext, newContext string) error
}
```

Update implementations to use interfaces:

```go
// File: pkg/embedding/cache/semantic_cache.go

// Ensure SemanticCache implements Cache interface
var _ Cache = (*SemanticCache)(nil)

// File: pkg/embedding/cache/tenant_cache.go

// Ensure TenantAwareCache implements TenantCache interface
var _ TenantCache = (*TenantAwareCache)(nil)

// Add GetRedisClient method
func (tc *TenantAwareCache) GetRedisClient() RedisClient {
    return tc.baseCache.redis
}

// Add GetVectorStore method
func (tc *TenantAwareCache) GetVectorStore() VectorStore {
    return tc.baseCache.vectorStore
}
```

### 3.2 Add pgvector Indexes

**Implementation**:

```sql
-- File: migrations/20240130_add_pgvector_indexes.up.sql

-- Create GiST index for vector similarity search
CREATE INDEX idx_cache_metadata_embedding_vector 
ON cache_metadata 
USING ivfflat (embedding vector_cosine_ops)
WITH (lists = 100);

-- Create composite index for tenant-scoped searches
CREATE INDEX idx_cache_metadata_tenant_embedding 
ON cache_metadata (tenant_id, embedding vector_cosine_ops);

-- Create index for access pattern queries
CREATE INDEX idx_cache_metadata_tenant_accessed 
ON cache_metadata (tenant_id, last_accessed_at DESC);

-- Create index for hit count queries
CREATE INDEX idx_cache_metadata_tenant_hits 
ON cache_metadata (tenant_id, hit_count DESC);

-- Create partial index for active entries
CREATE INDEX idx_cache_metadata_active 
ON cache_metadata (tenant_id, cache_key) 
WHERE is_active = true;

-- Analyze table to update statistics
ANALYZE cache_metadata;
```

Update VectorStore to use optimized queries:

```go
// File: pkg/embedding/cache/vector_store.go

// Add query hints for index usage
const findSimilarQuery = `
    SELECT /*+ IndexScan(cache_metadata idx_cache_metadata_tenant_embedding) */
        cache_key, 
        query_hash, 
        1 - (embedding <=> $2::vector) as similarity,
        hit_count,
        last_accessed_at
    FROM cache_metadata
    WHERE tenant_id = $1
        AND is_active = true
        AND 1 - (embedding <=> $2::vector) > $3
    ORDER BY embedding <=> $2::vector
    LIMIT $4
`

// Add index maintenance
func (v *VectorStore) OptimizeIndexes(ctx context.Context) error {
    // Vacuum and analyze for better performance
    queries := []string{
        "VACUUM ANALYZE cache_metadata",
        "REINDEX INDEX CONCURRENTLY idx_cache_metadata_embedding_vector",
    }
    
    for _, query := range queries {
        if _, err := v.db.ExecContext(ctx, query); err != nil {
            return fmt.Errorf("failed to optimize indexes: %w", err)
        }
    }
    
    return nil
}
```

### 3.3 Implement Tenant Config Caching

**Implementation**:

```go
// File: pkg/embedding/cache/tenant/config_cache.go
package tenant

import (
    "context"
    "sync"
    "time"
    
    "github.com/google/uuid"
    "github.com/developer-mesh/developer-mesh/pkg/models"
    "github.com/developer-mesh/developer-mesh/pkg/repository"
)

// ConfigCache caches tenant configurations with TTL
type ConfigCache struct {
    repo       repository.TenantConfigRepository
    cache      sync.Map // map[uuid.UUID]*cachedConfig
    ttl        time.Duration
    mu         sync.RWMutex
}

type cachedConfig struct {
    config    *TenantCacheConfig
    expiresAt time.Time
}

// NewConfigCache creates a new configuration cache
func NewConfigCache(repo repository.TenantConfigRepository, ttl time.Duration) *ConfigCache {
    if ttl <= 0 {
        ttl = 5 * time.Minute // default TTL
    }
    
    return &ConfigCache{
        repo: repo,
        ttl:  ttl,
    }
}

// Get retrieves a tenant configuration from cache or database
func (c *ConfigCache) Get(ctx context.Context, tenantID uuid.UUID) (*TenantCacheConfig, error) {
    // Check cache first
    if cached, ok := c.cache.Load(tenantID); ok {
        config := cached.(*cachedConfig)
        if time.Now().Before(config.expiresAt) {
            return config.config, nil
        }
        // Expired, delete it
        c.cache.Delete(tenantID)
    }
    
    // Load from database
    dbConfig, err := c.repo.GetByTenantID(ctx, tenantID.String())
    if err != nil {
        return nil, err
    }
    
    // Parse to cache config
    cacheConfig := ParseFromTenantConfig(dbConfig)
    
    // Cache it
    c.cache.Store(tenantID, &cachedConfig{
        config:    cacheConfig,
        expiresAt: time.Now().Add(c.ttl),
    })
    
    return cacheConfig, nil
}

// Invalidate removes a tenant configuration from cache
func (c *ConfigCache) Invalidate(tenantID uuid.UUID) {
    c.cache.Delete(tenantID)
}

// InvalidateAll clears the entire cache
func (c *ConfigCache) InvalidateAll() {
    c.cache.Range(func(key, value interface{}) bool {
        c.cache.Delete(key)
        return true
    })
}

// StartCleanup starts a background goroutine to clean expired entries
func (c *ConfigCache) StartCleanup(ctx context.Context) {
    ticker := time.NewTicker(c.ttl)
    go func() {
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                c.cleanupExpired()
            }
        }
    }()
}

// cleanupExpired removes expired entries from cache
func (c *ConfigCache) cleanupExpired() {
    now := time.Now()
    c.cache.Range(func(key, value interface{}) bool {
        config := value.(*cachedConfig)
        if now.After(config.expiresAt) {
            c.cache.Delete(key)
        }
        return true
    })
}
```

Update TenantAwareCache to use config cache:

```go
// File: pkg/embedding/cache/tenant_cache.go

// Add to TenantAwareCache struct
type TenantAwareCache struct {
    // ... existing fields ...
    configCache *tenant.ConfigCache
}

// Update NewTenantAwareCache
func NewTenantAwareCache(
    // ... existing parameters ...
) *TenantAwareCache {
    // ... existing code ...
    
    // Create config cache
    configCache := tenant.NewConfigCache(configRepo, 5*time.Minute)
    configCache.StartCleanup(context.Background())
    
    cache := &TenantAwareCache{
        // ... existing fields ...
        configCache: configCache,
    }
    
    return cache
}

// Update getTenantConfig to use cache
func (tc *TenantAwareCache) getTenantConfig(ctx context.Context, tenantID uuid.UUID) (*tenant.TenantCacheConfig, error) {
    return tc.configCache.Get(ctx, tenantID)
}
```

### 3.4 Extract Magic Numbers to Configuration

**Implementation**:

```go
// File: pkg/embedding/cache/constants.go
package cache

import "time"

// Cache key patterns
const (
    // Key separators
    KeySeparator = ":"
    
    // Key prefixes
    QueryKeyPrefix     = "q"
    MetadataKeyPrefix  = "m"
    StatsKeyPrefix     = "s"
    LockKeyPrefix      = "lock"
    
    // Default key names
    EmptyKeyName = "empty_key"
)

// Size constants
const (
    // Compression threshold
    DefaultCompressionThreshold = 1024 // 1KB
    
    // Buffer sizes
    DefaultTrackerBufferSize = 10000
    MinTrackerBufferSize     = 100
    MaxTrackerBufferSize     = 1000000
    
    // Batch sizes
    DefaultBatchSize = 100
    MinBatchSize     = 1
    MaxBatchSize     = 10000
    
    // Cache limits
    DefaultMaxCacheEntries = 10000
    DefaultMaxCacheBytes   = 100 * 1024 * 1024 // 100MB
)

// Time constants
const (
    // Timeouts
    DefaultGetTimeout      = 1 * time.Second
    DefaultSetTimeout      = 2 * time.Second
    DefaultDeleteTimeout   = 1 * time.Second
    DefaultShutdownTimeout = 30 * time.Second
    
    // Intervals
    DefaultFlushInterval    = 100 * time.Millisecond
    DefaultCleanupInterval  = 1 * time.Hour
    DefaultRotationInterval = 30 * 24 * time.Hour // 30 days
    
    // TTLs
    DefaultCacheTTL    = 24 * time.Hour
    DefaultConfigTTL   = 5 * time.Minute
    DefaultLockTTL     = 30 * time.Second
)

// Validation constants
const (
    // Query validation
    MaxQueryLength        = 1000
    MaxEmbeddingDimension = 1536 // OpenAI ada-002 dimension
    
    // Result limits
    MaxSearchResults = 100
    MaxBatchQueries  = 100
)

// GetConfigWithDefaults fills in default values for missing configuration
func GetConfigWithDefaults(config *Config) *Config {
    if config == nil {
        config = &Config{}
    }
    
    // Apply defaults
    if config.TTL <= 0 {
        config.TTL = DefaultCacheTTL
    }
    
    if config.MaxCandidates <= 0 {
        config.MaxCandidates = 10
    }
    
    if config.SimilarityThreshold == 0 {
        config.SimilarityThreshold = 0.95
    }
    
    if config.Prefix == "" {
        config.Prefix = "semantic_cache"
    }
    
    // Performance defaults
    if config.Performance.TrackerBufferSize <= 0 {
        config.Performance.TrackerBufferSize = DefaultTrackerBufferSize
    }
    
    if config.Performance.BatchSize <= 0 {
        config.Performance.BatchSize = DefaultBatchSize
    }
    
    if config.Performance.FlushInterval <= 0 {
        config.Performance.FlushInterval = DefaultFlushInterval
    }
    
    if config.Performance.CompressionThreshold <= 0 {
        config.Performance.CompressionThreshold = DefaultCompressionThreshold
    }
    
    // Timeout defaults
    if config.Performance.GetTimeout <= 0 {
        config.Performance.GetTimeout = DefaultGetTimeout
    }
    
    if config.Performance.SetTimeout <= 0 {
        config.Performance.SetTimeout = DefaultSetTimeout
    }
    
    if config.Performance.DeleteTimeout <= 0 {
        config.Performance.DeleteTimeout = DefaultDeleteTimeout
    }
    
    if config.Performance.ShutdownTimeout <= 0 {
        config.Performance.ShutdownTimeout = DefaultShutdownTimeout
    }
    
    return config
}
```

Update code to use constants:

```go
// File: pkg/embedding/cache/semantic_cache.go

// Replace magic numbers
func (c *SemanticCache) getCacheKey(normalized string) string {
    return fmt.Sprintf("%s%s%s%s%s", c.config.Prefix, KeySeparator, QueryKeyPrefix, KeySeparator, SanitizeRedisKey(normalized))
}

// File: pkg/embedding/cache/lru/tracker.go

// Use constants for validation
func NewAsyncTracker(
    client RedisClient,
    config *Config,
    logger observability.Logger,
    metrics observability.MetricsClient,
) *AsyncTracker {
    bufferSize := config.Performance.TrackerBufferSize
    
    // Validate buffer size
    if bufferSize < MinTrackerBufferSize {
        bufferSize = MinTrackerBufferSize
    } else if bufferSize > MaxTrackerBufferSize {
        bufferSize = MaxTrackerBufferSize
    }
    
    // ... rest of implementation
}
```

## Phase 4: Low Priority Nice-to-Haves

### 4.1 Degraded Mode for Redis Failures

**Implementation**:

```go
// File: pkg/embedding/cache/fallback/memory_cache.go
package fallback

import (
    "context"
    "sync"
    "time"
    
    "github.com/google/uuid"
    "github.com/developer-mesh/developer-mesh/pkg/embedding/cache"
)

// MemoryCache provides in-memory caching when Redis is unavailable
type MemoryCache struct {
    entries  sync.Map // map[string]*memoryCacheEntry
    maxSize  int
    mu       sync.RWMutex
    size     int
}

type memoryCacheEntry struct {
    entry     *cache.CacheEntry
    expiresAt time.Time
}

// NewMemoryCache creates a fallback memory cache
func NewMemoryCache(maxSize int) *MemoryCache {
    if maxSize <= 0 {
        maxSize = 1000 // Default to 1000 entries
    }
    
    mc := &MemoryCache{
        maxSize: maxSize,
    }
    
    // Start cleanup routine
    go mc.cleanupLoop()
    
    return mc
}

// Get retrieves an entry from memory cache
func (mc *MemoryCache) Get(ctx context.Context, key string) (*cache.CacheEntry, error) {
    if val, ok := mc.entries.Load(key); ok {
        entry := val.(*memoryCacheEntry)
        if time.Now().Before(entry.expiresAt) {
            return entry.entry, nil
        }
        mc.entries.Delete(key)
    }
    
    return nil, nil
}

// Set stores an entry in memory cache
func (mc *MemoryCache) Set(ctx context.Context, key string, entry *cache.CacheEntry, ttl time.Duration) error {
    mc.mu.Lock()
    defer mc.mu.Unlock()
    
    // Check size limit
    if mc.size >= mc.maxSize {
        // Evict random entry (simple strategy)
        mc.evictOne()
    }
    
    mc.entries.Store(key, &memoryCacheEntry{
        entry:     entry,
        expiresAt: time.Now().Add(ttl),
    })
    mc.size++
    
    return nil
}

// evictOne removes a random entry
func (mc *MemoryCache) evictOne() {
    mc.entries.Range(func(key, value interface{}) bool {
        mc.entries.Delete(key)
        mc.size--
        return false // Stop after first deletion
    })
}

// cleanupLoop periodically removes expired entries
func (mc *MemoryCache) cleanupLoop() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        now := time.Now()
        mc.entries.Range(func(key, value interface{}) bool {
            entry := value.(*memoryCacheEntry)
            if now.After(entry.expiresAt) {
                mc.entries.Delete(key)
                mc.mu.Lock()
                mc.size--
                mc.mu.Unlock()
            }
            return true
        })
    }
}
```

Update SemanticCache to use fallback:

```go
// File: pkg/embedding/cache/semantic_cache.go

// Add to SemanticCache struct
type SemanticCache struct {
    // ... existing fields ...
    fallbackCache *fallback.MemoryCache
    degradedMode  atomic.Bool
}

// Update Get method to use fallback
func (c *SemanticCache) Get(ctx context.Context, query string, embedding []float32) (*CacheEntry, error) {
    // Check if in degraded mode
    if c.degradedMode.Load() && c.fallbackCache != nil {
        return c.fallbackCache.Get(ctx, c.getCacheKey(query))
    }
    
    // Try Redis with circuit breaker
    entry, err := c.getFromRedis(ctx, query, embedding)
    if err != nil {
        // Check if circuit breaker is open
        if c.isCircuitBreakerOpen() {
            c.enterDegradedMode()
            if c.fallbackCache != nil {
                return c.fallbackCache.Get(ctx, c.getCacheKey(query))
            }
        }
        return nil, err
    }
    
    return entry, nil
}

// enterDegradedMode switches to fallback mode
func (c *SemanticCache) enterDegradedMode() {
    if c.degradedMode.CompareAndSwap(false, true) {
        c.logger.Warn("Entering degraded mode - using in-memory cache", nil)
        
        // Start recovery checker
        go c.checkRecovery()
    }
}

// checkRecovery periodically checks if Redis is available again
func (c *SemanticCache) checkRecovery() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        err := c.redis.Execute(ctx, func() (interface{}, error) {
            return nil, c.redis.Ping(ctx)
        })
        cancel()
        
        if err == nil {
            c.degradedMode.Store(false)
            c.logger.Info("Exiting degraded mode - Redis connection restored", nil)
            return
        }
    }
}
```

### 4.2 Comprehensive Godoc

Add comprehensive documentation to all exported types and methods:

```go
// File: pkg/embedding/cache/doc.go

// Package cache provides a high-performance, tenant-isolated semantic cache
// for embedding-based search results.
//
// Overview
//
// The semantic cache is designed to accelerate AI/ML applications by caching
// search results based on query similarity rather than exact matches. It uses
// vector embeddings to find semantically similar queries and return cached
// results when appropriate.
//
// Key Features:
//   - Tenant isolation with per-tenant encryption
//   - LRU eviction based on configurable limits
//   - Vector similarity search using pgvector
//   - Automatic compression for large entries
//   - Circuit breaker pattern for resilience
//   - Comprehensive metrics and monitoring
//
// Architecture
//
// The cache is organized in layers:
//   - SemanticCache: Core caching logic with Redis storage
//   - TenantAwareCache: Adds multi-tenancy and encryption
//   - LRU Manager: Handles eviction based on access patterns
//   - Vector Store: PostgreSQL with pgvector for similarity search
//   - Compression Service: Reduces storage for large entries
//
// Usage Example
//
//   // Create cache with configuration
//   config := cache.DefaultConfig()
//   config.SimilarityThreshold = 0.95
//   
//   redisClient := redis.NewClient(&redis.Options{
//       Addr: "localhost:6379",
//   })
//   
//   semanticCache, err := cache.NewSemanticCache(
//       redisClient,
//       config,
//       logger,
//   )
//   if err != nil {
//       log.Fatal(err)
//   }
//   
//   // Create tenant-aware wrapper
//   tenantCache := cache.NewTenantAwareCache(
//       semanticCache,
//       configRepo,
//       keyRepo,
//       rateLimiter,
//       masterKeyID,
//       logger,
//       metrics,
//   )
//   
//   // Use the cache
//   ctx := context.WithValue(context.Background(), "tenant_id", tenantID)
//   
//   // Store results
//   err = tenantCache.Set(ctx, query, embedding, results)
//   
//   // Retrieve results
//   entry, err := tenantCache.Get(ctx, query, embedding)
//   if err == nil && entry != nil {
//       // Cache hit - use cached results
//       return entry.Results
//   }
//
// Configuration
//
// The cache behavior can be configured through the Config struct:
//
//   config := &cache.Config{
//       TTL:                 24 * time.Hour,
//       SimilarityThreshold: 0.95,
//       MaxCandidates:       10,
//       EnableCompression:   true,
//       EnableMetrics:       true,
//   }
//
// For production environments, use performance profiles:
//
//   config.Performance = cache.GetPerformanceConfig(
//       cache.PerformanceProfileHighThroughput,
//   )
//
// Monitoring
//
// The cache exposes Prometheus metrics:
//   - cache_hits_total: Number of cache hits
//   - cache_misses_total: Number of cache misses
//   - cache_operation_duration_seconds: Operation latency
//   - cache_entries: Number of entries per tenant
//   - cache_bytes: Storage used per tenant
//
// Health checks are available at /health endpoint.
//
// Best Practices
//
// 1. Set appropriate similarity thresholds based on your use case
// 2. Monitor cache hit rates and adjust TTL accordingly
// 3. Use tenant isolation for multi-tenant applications
// 4. Enable compression for large result sets
// 5. Configure circuit breakers for resilience
// 6. Implement key rotation for security
//
// Security
//
// The cache provides several security features:
//   - Per-tenant encryption with unique keys
//   - Automatic detection and encryption of sensitive data
//   - Key rotation support
//   - Audit logging for compliance
//   - Input validation and sanitization
//
package cache
```

### 4.3 Audit Logging

**Implementation**:

```go
// File: pkg/embedding/cache/audit/audit_logger.go
package audit

import (
    "context"
    "time"
    
    "github.com/google/uuid"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
)

// AuditEvent represents a cache audit event
type AuditEvent struct {
    EventID     string                 `json:"event_id"`
    EventType   string                 `json:"event_type"`
    TenantID    uuid.UUID             `json:"tenant_id"`
    UserID      string                 `json:"user_id,omitempty"`
    Operation   string                 `json:"operation"`
    Resource    string                 `json:"resource"`
    Result      string                 `json:"result"`
    Duration    time.Duration         `json:"duration_ms"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
    Timestamp   time.Time             `json:"timestamp"`
}

// AuditLogger logs cache operations for compliance
type AuditLogger struct {
    logger observability.Logger
    buffer chan *AuditEvent
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logger observability.Logger) *AuditLogger {
    al := &AuditLogger{
        logger: logger,
        buffer: make(chan *AuditEvent, 1000),
    }
    
    go al.processEvents()
    
    return al
}

// LogOperation logs a cache operation
func (al *AuditLogger) LogOperation(
    ctx context.Context,
    operation string,
    resource string,
    duration time.Duration,
    err error,
) {
    tenantID := auth.GetTenantID(ctx)
    userID := auth.GetUserID(ctx)
    
    result := "success"
    if err != nil {
        result = "failure"
    }
    
    event := &AuditEvent{
        EventID:   uuid.New().String(),
        EventType: "cache_operation",
        TenantID:  tenantID,
        UserID:    userID,
        Operation: operation,
        Resource:  resource,
        Result:    result,
        Duration:  duration,
        Timestamp: time.Now(),
    }
    
    if err != nil {
        event.Metadata = map[string]interface{}{
            "error": err.Error(),
        }
    }
    
    select {
    case al.buffer <- event:
    default:
        // Buffer full, log directly
        al.logEvent(event)
    }
}

// processEvents processes buffered audit events
func (al *AuditLogger) processEvents() {
    for event := range al.buffer {
        al.logEvent(event)
    }
}

// logEvent logs a single audit event
func (al *AuditLogger) logEvent(event *AuditEvent) {
    al.logger.Info("AUDIT", map[string]interface{}{
        "event_id":    event.EventID,
        "event_type":  event.EventType,
        "tenant_id":   event.TenantID.String(),
        "user_id":     event.UserID,
        "operation":   event.Operation,
        "resource":    event.Resource,
        "result":      event.Result,
        "duration_ms": event.Duration.Milliseconds(),
        "metadata":    event.Metadata,
        "timestamp":   event.Timestamp,
    })
}
```

## Implementation Order

1. **Day 1-2**: Complete Phase 1 (Critical Issues)
   - Fix TODO comment
   - Fix vector store tests
   - Implement structured logging
   - Configure connection pools

2. **Day 3-5**: Implement Phase 2 (High Priority)
   - Per-tenant encryption keys
   - Configurable parameters
   - Retry and circuit breaker configuration
   - Health check endpoints

3. **Day 6-7**: Implement Phase 3 (Medium Priority)
   - Define interfaces
   - Add pgvector indexes
   - Implement config caching
   - Extract magic numbers

4. **Day 8**: Implement Phase 4 (Nice-to-Haves)
   - Degraded mode
   - Documentation
   - Audit logging

## Testing Strategy

1. **Unit Tests**: Update existing tests for new interfaces
2. **Integration Tests**: Test end-to-end flows with new features
3. **Performance Tests**: Verify no regression with changes
4. **Security Tests**: Validate encryption and key rotation

## Rollout Plan

Since this is a greenfield deployment:

1. Deploy all changes at once
2. Enable monitoring and alerting
3. Run smoke tests in production
4. Monitor metrics for 24 hours
5. Enable for pilot tenants first
6. Roll out to all tenants

## Success Metrics

- Zero critical security findings
- Cache hit rate > 60%
- p99 latency < 100ms
- Zero data breaches
- 99.9% availability

This comprehensive plan provides all the implementation details needed to bring the semantic cache to production-ready status while following project best practices.