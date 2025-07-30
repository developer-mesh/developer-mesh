package cache

import "errors"

var (
	// Tenant errors
	ErrInvalidTenantID = errors.New("invalid tenant ID format")
	ErrTenantNotFound  = errors.New("tenant configuration not found")

	// Feature errors
	ErrFeatureDisabled = errors.New("feature disabled for tenant")

	// Quota errors
	ErrQuotaExceeded  = errors.New("cache quota exceeded")
	ErrEvictionFailed = errors.New("failed to evict entries")

	// Cache operation errors
	ErrCacheMiss             = errors.New("cache miss")
	ErrInvalidQuery          = errors.New("invalid query")
	ErrInvalidEmbedding      = errors.New("invalid embedding")
	ErrSerializationFailed   = errors.New("serialization failed")
	ErrDeserializationFailed = errors.New("deserialization failed")

	// Storage errors
	ErrStorageUnavailable = errors.New("storage backend unavailable")
	ErrStorageTimeout     = errors.New("storage operation timeout")

	// Configuration errors
	ErrInvalidConfig  = errors.New("invalid configuration")
	ErrConfigNotFound = errors.New("configuration not found")
)
