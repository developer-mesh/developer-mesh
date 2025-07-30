package cache

import "time"

// Cache key patterns
const (
	// Key separators
	KeySeparator = ":"

	// Key prefixes
	QueryKeyPrefix    = "q"
	MetadataKeyPrefix = "m"
	StatsKeyPrefix    = "s"
	LockKeyPrefix     = "lock"
	TenantKeyPrefix   = "t"

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
	DefaultMaxCacheSize    = 1000              // Default entries
)

// Time constants
const (
	// Timeouts
	DefaultGetTimeout      = 1 * time.Second
	DefaultSetTimeout      = 2 * time.Second
	DefaultDeleteTimeout   = 1 * time.Second
	DefaultShutdownTimeout = 30 * time.Second
	DefaultHealthTimeout   = 5 * time.Second

	// Intervals
	DefaultFlushInterval    = 100 * time.Millisecond
	DefaultCleanupInterval  = 1 * time.Hour
	DefaultRotationInterval = 30 * 24 * time.Hour // 30 days
	DefaultCheckPeriod      = 5 * time.Second

	// TTLs
	DefaultCacheTTL    = 24 * time.Hour
	DefaultConfigTTL   = 5 * time.Minute
	DefaultLockTTL     = 30 * time.Second
	DefaultFallbackTTL = 15 * time.Minute
)

// Validation constants
const (
	// Query validation
	MaxQueryLength        = 1000
	MaxEmbeddingDimension = 1536 // OpenAI ada-002 dimension

	// Result limits
	MaxSearchResults = 100
	MaxBatchQueries  = 100
	DefaultLimit     = 10

	// Similarity thresholds
	DefaultSimilarityThreshold = 0.95
	MinSimilarityThreshold     = 0.0
	MaxSimilarityThreshold     = 1.0
)

// Eviction constants
const (
	// Eviction percentages
	DefaultEvictionPercentage = 0.1 // 10%
	MaxEvictionPercentage     = 0.5 // 50%
)

// Redis operation constants
const (
	// Scan count for Redis SCAN operations
	DefaultScanCount = 100

	// Pipeline size
	DefaultPipelineSize = 10

	// Lock retry
	LockMaxRetries     = 10
	LockRetryDelay     = 50 * time.Millisecond
	LockAcquireTimeout = 5 * time.Second
)

// Vector store constants
const (
	// Index configuration
	DefaultIVFFlatLists = 100

	// Maintenance
	VacuumBatchSize = 1000
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
		config.MaxCandidates = DefaultLimit
	}

	if config.SimilarityThreshold == 0 {
		config.SimilarityThreshold = DefaultSimilarityThreshold
	}

	if config.Prefix == "" {
		config.Prefix = "semantic_cache"
	}

	if config.MaxCacheSize <= 0 {
		config.MaxCacheSize = DefaultMaxCacheSize
	}

	// Apply performance defaults
	if config.PerformanceConfig == nil {
		config.PerformanceConfig = &PerformanceConfig{}
	}

	perf := config.PerformanceConfig

	// TrackerBufferSize is not in PerformanceConfig
	// It would be part of a separate LRU configuration

	if perf.BatchSize <= 0 {
		perf.BatchSize = DefaultBatchSize
	}

	if perf.FlushInterval <= 0 {
		perf.FlushInterval = DefaultFlushInterval
	}

	if perf.CompressionThreshold <= 0 {
		perf.CompressionThreshold = DefaultCompressionThreshold
	}

	// Timeout defaults
	if perf.GetTimeout <= 0 {
		perf.GetTimeout = DefaultGetTimeout
	}

	if perf.SetTimeout <= 0 {
		perf.SetTimeout = DefaultSetTimeout
	}

	if perf.DeleteTimeout <= 0 {
		perf.DeleteTimeout = DefaultDeleteTimeout
	}

	// ShutdownTimeout is not in PerformanceConfig

	return config
}
