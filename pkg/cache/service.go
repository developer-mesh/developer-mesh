package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

type Service struct {
	db     *sqlx.DB
	redis  *redis.Client
	logger observability.Logger
}

func NewService(db *sqlx.DB, redis *redis.Client, logger observability.Logger) *Service {
	return &Service{
		db:     db,
		redis:  redis,
		logger: logger,
	}
}

type CacheEntry struct {
	KeyHash      string          `db:"key_hash"`
	TenantID     string          `db:"tenant_id"`
	ResponseData json.RawMessage `db:"response_data"`
	FromCache    bool            `db:"from_cache"`
	HitCount     int             `db:"hit_count"`
	CreatedAt    time.Time       `db:"created_at"`
}

type ExecutionRequest struct {
	TenantID   string
	ToolID     string
	Action     string
	Parameters map[string]interface{}
	TTLSeconds int
}

func (s *Service) GetOrCompute(
	ctx context.Context,
	req *ExecutionRequest,
	computeFn func(context.Context) (interface{}, error),
) (interface{}, error) {
	startTime := time.Now()

	// Generate cache key
	cacheKey := s.generateCacheKey(req)
	parametersHash := s.hashParameters(req.Parameters)

	// Try L1 cache (Redis) first for hot data (if Redis is available)
	redisKey := fmt.Sprintf("cache:%s:%s", req.TenantID, cacheKey)
	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, redisKey).Result(); err == nil && cached != "" {
			s.logger.Debug("Cache hit from Redis", map[string]interface{}{
				"tenant_id": req.TenantID,
				"tool_id":   req.ToolID,
				"action":    req.Action,
				"cache_key": cacheKey,
				"latency":   time.Since(startTime).Milliseconds(),
			})

			// Update stats
			go s.updateStats(ctx, req.TenantID, true, time.Since(startTime).Milliseconds())

			// Try to unmarshal as ToolExecutionResponse first
			var toolResponse models.ToolExecutionResponse
			if err := json.Unmarshal([]byte(cached), &toolResponse); err == nil {
				// Add cache metadata to the response
				toolResponse.FromCache = true
				toolResponse.CacheHit = true
				toolResponse.CacheLevel = "L1_redis"
				s.logger.Info("Returning cached ToolExecutionResponse from Redis", map[string]interface{}{
					"from_cache":  toolResponse.FromCache,
					"cache_hit":   toolResponse.CacheHit,
					"cache_level": toolResponse.CacheLevel,
				})
				return &toolResponse, nil
			}

			// Fallback to map if not a ToolExecutionResponse
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(cached), &result); err == nil {
				// Add cache metadata
				result["from_cache"] = true
				result["cache_hit"] = true
				result["cache_level"] = "L1_redis"
				return result, nil
			}
		}
	}

	// Try L2 cache (PostgreSQL)
	var entry CacheEntry
	err := s.db.GetContext(ctx, &entry, `
		SELECT * FROM mcp.get_or_create_cache_entry(
			$1, $2, $3, $4, $5, NULL, NULL, NULL, $6
		)
	`, cacheKey, req.TenantID, req.ToolID, req.Action, parametersHash, req.TTLSeconds)

	if err == nil && entry.FromCache {
		s.logger.Debug("Cache hit from PostgreSQL", map[string]interface{}{
			"tenant_id": req.TenantID,
			"tool_id":   req.ToolID,
			"action":    req.Action,
			"cache_key": cacheKey,
			"hit_count": entry.HitCount,
			"latency":   time.Since(startTime).Milliseconds(),
		})

		// Populate Redis for next time (write-through) if Redis is available
		if s.redis != nil {
			go func() {
				ttl := time.Duration(req.TTLSeconds) * time.Second
				if ttl == 0 {
					ttl = time.Hour
				}
				_ = s.redis.Set(context.Background(), redisKey, string(entry.ResponseData), ttl).Err()
			}()
		}

		// Update stats
		go s.updateStats(ctx, req.TenantID, true, time.Since(startTime).Milliseconds())

		// Try to unmarshal as ToolExecutionResponse first
		var toolResponse models.ToolExecutionResponse
		if err := json.Unmarshal(entry.ResponseData, &toolResponse); err == nil {
			// Add cache metadata to the response
			toolResponse.FromCache = true
			toolResponse.CacheHit = true
			toolResponse.CacheLevel = "L2_postgres"
			toolResponse.HitCount = entry.HitCount
			s.logger.Info("Returning cached ToolExecutionResponse from PostgreSQL", map[string]interface{}{
				"from_cache":  toolResponse.FromCache,
				"cache_hit":   toolResponse.CacheHit,
				"cache_level": toolResponse.CacheLevel,
				"hit_count":   toolResponse.HitCount,
			})
			return &toolResponse, nil
		}

		// Fallback to map if not a ToolExecutionResponse
		var result map[string]interface{}
		if err := json.Unmarshal(entry.ResponseData, &result); err == nil {
			// Add cache metadata
			result["from_cache"] = true
			result["cache_hit"] = true
			result["cache_level"] = "L2_postgres"
			result["hit_count"] = entry.HitCount
			return result, nil
		}
	}

	// Cache miss - compute the value
	s.logger.Debug("Cache miss, computing value", map[string]interface{}{
		"tenant_id": req.TenantID,
		"tool_id":   req.ToolID,
		"action":    req.Action,
		"cache_key": cacheKey,
	})

	computeStart := time.Now()
	value, err := computeFn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to compute value: %w", err)
	}
	computeTime := time.Since(computeStart)

	// Convert to JSON for storage
	jsonData, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	// Store in both caches
	go s.storeInCaches(ctx, req, cacheKey, parametersHash, jsonData)

	// Update stats
	go s.updateStats(ctx, req.TenantID, false, time.Since(startTime).Milliseconds())

	// Add metadata to response based on type
	switch v := value.(type) {
	case *models.ToolExecutionResponse:
		// Add cache metadata to ToolExecutionResponse
		v.FromCache = false
		v.CacheHit = false
		s.logger.Info("Returning computed ToolExecutionResponse (cache miss)", map[string]interface{}{
			"from_cache": v.FromCache,
			"cache_hit":  v.CacheHit,
		})
		return v, nil
	case map[string]interface{}:
		// Add cache metadata to map
		v["from_cache"] = false
		v["cache_hit"] = false
		v["compute_time_ms"] = computeTime.Milliseconds()
		return v, nil
	default:
		// Return as-is
		return value, nil
	}
}

func (s *Service) generateCacheKey(req *ExecutionRequest) string {
	h := sha256.New()
	h.Write([]byte(req.TenantID))
	h.Write([]byte(req.ToolID))
	h.Write([]byte(req.Action))

	// Canonical JSON for consistent hashing
	if canonical, err := json.Marshal(req.Parameters); err == nil {
		h.Write(canonical)
	}

	return hex.EncodeToString(h.Sum(nil))
}

func (s *Service) hashParameters(params map[string]interface{}) string {
	h := sha256.New()
	if canonical, err := json.Marshal(params); err == nil {
		h.Write(canonical)
	}
	return hex.EncodeToString(h.Sum(nil))[:16] // Truncate for readability
}

func (s *Service) storeInCaches(ctx context.Context, req *ExecutionRequest, cacheKey, parametersHash string, data []byte) {
	// Use background context for async cache operations to avoid cancellation
	backgroundCtx := context.Background()

	// Store in PostgreSQL (already done via get_or_create_cache_entry)
	_, err := s.db.ExecContext(backgroundCtx, `
		SELECT * FROM mcp.get_or_create_cache_entry(
			$1, $2, $3, $4, $5, $6, NULL, NULL, $7
		)
	`, cacheKey, req.TenantID, req.ToolID, req.Action, parametersHash, data, req.TTLSeconds)

	if err != nil {
		s.logger.Warn("Failed to store in PostgreSQL cache", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": req.TenantID,
			"tool_id":   req.ToolID,
		})
	}

	// Store in Redis (if available)
	if s.redis != nil {
		redisKey := fmt.Sprintf("cache:%s:%s", req.TenantID, cacheKey)
		ttl := time.Duration(req.TTLSeconds) * time.Second
		if ttl == 0 {
			ttl = time.Hour
		}

		if err := s.redis.Set(backgroundCtx, redisKey, data, ttl).Err(); err != nil {
			s.logger.Warn("Failed to store in Redis cache", map[string]interface{}{
				"error":     err.Error(),
				"tenant_id": req.TenantID,
				"tool_id":   req.ToolID,
			})
		}
	}
}

func (s *Service) updateStats(ctx context.Context, tenantID string, isHit bool, responseTimeMs int64) {
	// Use background context to avoid cancellation
	backgroundCtx := context.Background()
	_, err := s.db.ExecContext(backgroundCtx, `
		SELECT mcp.update_cache_stats($1, $2, $3, 0, 0)
	`, tenantID, isHit, responseTimeMs)

	if err != nil {
		s.logger.Warn("Failed to update cache statistics", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID,
		})
	}
}

func (s *Service) InvalidatePattern(ctx context.Context, tenantID, pattern string) error {
	// Invalidate Redis entries (if Redis is available)
	if s.redis != nil {
		redisPattern := fmt.Sprintf("cache:%s:%s*", tenantID, pattern)
		iter := s.redis.Scan(ctx, 0, redisPattern, 0).Iterator()
		for iter.Next(ctx) {
			if err := s.redis.Del(ctx, iter.Val()).Err(); err != nil {
				s.logger.Warn("Failed to delete Redis key", map[string]interface{}{
					"error": err.Error(),
					"key":   iter.Val(),
				})
			}
		}
	}

	// Invalidate PostgreSQL entries (mark as expired)
	_, err := s.db.ExecContext(ctx, `
		UPDATE mcp.cache_entries 
		SET expires_at = NOW() 
		WHERE tenant_id = $1 AND key_hash LIKE $2
	`, tenantID, pattern+"%")

	return err
}

func (s *Service) GetStats(ctx context.Context, tenantID string) (map[string]interface{}, error) {
	var stats struct {
		TotalRequests   int     `db:"total_requests"`
		CacheHits       int     `db:"cache_hits"`
		CacheMisses     int     `db:"cache_misses"`
		CacheHitRate    float64 `db:"cache_hit_rate"`
		AvgResponseTime float64 `db:"avg_response_time_ms"`
		BytesSaved      int64   `db:"bytes_saved"`
		APICallsSaved   int     `db:"api_calls_saved"`
	}

	err := s.db.GetContext(ctx, &stats, `
		SELECT 
			total_requests,
			cache_hits,
			cache_misses,
			cache_hit_rate,
			avg_response_time_ms,
			bytes_saved,
			api_calls_saved
		FROM mcp.cache_performance
		WHERE tenant_id = $1 AND date = CURRENT_DATE
		LIMIT 1
	`, tenantID)

	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_requests":       stats.TotalRequests,
		"cache_hits":           stats.CacheHits,
		"cache_misses":         stats.CacheMisses,
		"cache_hit_rate":       stats.CacheHitRate,
		"avg_response_time_ms": stats.AvgResponseTime,
		"bytes_saved":          stats.BytesSaved,
		"api_calls_saved":      stats.APICallsSaved,
	}, nil
}
