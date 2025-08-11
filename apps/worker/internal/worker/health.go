package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/database"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/redis/go-redis/v9"
)

// HealthStatus represents the health status of a component
type HealthStatus struct {
	Status    string                 `json:"status"` // healthy, degraded, unhealthy
	Message   string                 `json:"message,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// HealthChecker performs health checks on various components
type HealthChecker struct {
	db          *database.Database
	redisClient *redis.Client
	queueClient *queue.Client
	metrics     *MetricsCollector
	logger      observability.Logger
	mu          sync.RWMutex
	statuses    map[string]*HealthStatus
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(
	db *database.Database,
	redisClient *redis.Client,
	queueClient *queue.Client,
	metrics *MetricsCollector,
	logger observability.Logger,
) *HealthChecker {
	return &HealthChecker{
		db:          db,
		redisClient: redisClient,
		queueClient: queueClient,
		metrics:     metrics,
		logger:      logger,
		statuses:    make(map[string]*HealthStatus),
	}
}

// CheckHealth performs health checks on all components
func (h *HealthChecker) CheckHealth(ctx context.Context) map[string]*HealthStatus {
	results := make(map[string]*HealthStatus)

	// Check database
	results["database"] = h.checkDatabase(ctx)

	// Check Redis
	results["redis"] = h.checkRedis(ctx)

	// Check queue
	results["queue"] = h.checkQueue(ctx)

	// Check worker process
	results["worker"] = h.checkWorker(ctx)

	// Update cached statuses
	h.mu.Lock()
	h.statuses = results
	h.mu.Unlock()

	return results
}

// checkDatabase checks database connectivity and performance
func (h *HealthChecker) checkDatabase(ctx context.Context) *HealthStatus {
	start := time.Now()
	defer func() {
		h.metrics.RecordHealthCheck("database", true, time.Since(start))
	}()

	status := &HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Check database ping
	db := h.db.GetDB()
	if err := db.PingContext(ctx); err != nil {
		status.Status = "unhealthy"
		status.Message = fmt.Sprintf("Database ping failed: %v", err)
		h.metrics.RecordHealthCheck("database", false, time.Since(start))
		return status
	}

	// Check connection pool stats
	stats := db.Stats()
	status.Details["open_connections"] = stats.OpenConnections
	status.Details["in_use"] = stats.InUse
	status.Details["idle"] = stats.Idle

	// Check if we're running low on connections
	if float64(stats.InUse) > float64(stats.MaxOpenConnections)*0.8 {
		status.Status = "degraded"
		status.Message = "High database connection usage"
	}

	// Check query performance
	var result int
	err := db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	queryDuration := time.Since(start)
	status.Details["query_duration_ms"] = queryDuration.Milliseconds()

	if err != nil {
		status.Status = "unhealthy"
		status.Message = fmt.Sprintf("Health check query failed: %v", err)
	} else if queryDuration > 100*time.Millisecond {
		status.Status = "degraded"
		status.Message = "Slow database response"
	}

	return status
}

// checkRedis checks Redis connectivity and performance
func (h *HealthChecker) checkRedis(ctx context.Context) *HealthStatus {
	start := time.Now()
	defer func() {
		h.metrics.RecordHealthCheck("redis", true, time.Since(start))
	}()

	status := &HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Check Redis ping
	pingStart := time.Now()
	if err := h.redisClient.Ping(ctx).Err(); err != nil {
		status.Status = "unhealthy"
		status.Message = fmt.Sprintf("Redis ping failed: %v", err)
		h.metrics.RecordHealthCheck("redis", false, time.Since(start))
		return status
	}

	pingDuration := time.Since(pingStart)
	status.Details["ping_duration_ms"] = pingDuration.Milliseconds()

	if pingDuration > 50*time.Millisecond {
		status.Status = "degraded"
		status.Message = "Slow Redis response"
	}

	// Get Redis info
	_, err := h.redisClient.Info(ctx).Result()
	if err == nil {
		// Parse some basic info (simplified)
		status.Details["info"] = "available"
	}

	return status
}

// checkQueue checks queue connectivity and depth
func (h *HealthChecker) checkQueue(ctx context.Context) *HealthStatus {
	start := time.Now()
	defer func() {
		h.metrics.RecordHealthCheck("queue", true, time.Since(start))
	}()

	status := &HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Check queue health
	if err := h.queueClient.Health(ctx); err != nil {
		status.Status = "unhealthy"
		status.Message = fmt.Sprintf("Queue health check failed: %v", err)
		h.metrics.RecordHealthCheck("queue", false, time.Since(start))
		return status
	}

	// Get queue depth
	depth, err := h.queueClient.GetQueueDepth(ctx)
	if err != nil {
		status.Status = "degraded"
		status.Message = fmt.Sprintf("Failed to get queue depth: %v", err)
	} else {
		status.Details["queue_depth"] = depth
		h.metrics.RecordQueueDepth(depth)

		// Alert if queue is getting too deep
		if depth > 1000 {
			status.Status = "degraded"
			status.Message = "High queue depth"
		}
	}

	return status
}

// checkWorker checks the worker process health
func (h *HealthChecker) checkWorker(ctx context.Context) *HealthStatus {
	status := &HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Add runtime stats
	perfMon := &PerformanceMonitor{metrics: h.metrics}
	stats := perfMon.GetRuntimeStats()

	status.Details["runtime"] = stats

	// Check memory usage
	if memStats, ok := stats["memory"].(map[string]interface{}); ok {
		if allocMB, ok := memStats["alloc_mb"].(uint64); ok && allocMB > 500 {
			status.Status = "degraded"
			status.Message = "High memory usage"
		}
	}

	// Check goroutine count
	if rtStats, ok := stats["runtime"].(map[string]interface{}); ok {
		if goroutines, ok := rtStats["goroutines"].(int); ok && goroutines > 1000 {
			status.Status = "degraded"
			status.Message = "High goroutine count"
		}
	}

	return status
}

// ServeHTTP implements http.Handler for health check endpoint
func (h *HealthChecker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Perform health checks
	results := h.CheckHealth(ctx)

	// Determine overall status
	overallStatus := "healthy"
	unhealthyComponents := []string{}
	degradedComponents := []string{}

	for component, status := range results {
		switch status.Status {
		case "unhealthy":
			overallStatus = "unhealthy"
			unhealthyComponents = append(unhealthyComponents, component)
		case "degraded":
			if overallStatus == "healthy" {
				overallStatus = "degraded"
			}
			degradedComponents = append(degradedComponents, component)
		}
	}

	response := map[string]interface{}{
		"status":     overallStatus,
		"timestamp":  time.Now(),
		"components": results,
	}

	if len(unhealthyComponents) > 0 {
		response["unhealthy_components"] = unhealthyComponents
	}
	if len(degradedComponents) > 0 {
		response["degraded_components"] = degradedComponents
	}

	// Set appropriate HTTP status code
	statusCode := http.StatusOK
	switch overallStatus {
	case "unhealthy":
		statusCode = http.StatusServiceUnavailable
	case "degraded":
		statusCode = http.StatusOK // Still return 200 for degraded
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode health response", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Log health check
	h.logger.Debug("Health check completed", map[string]interface{}{
		"overall_status": overallStatus,
		"status_code":    statusCode,
	})
}

// StartHealthEndpoint starts the health check HTTP endpoint
func (h *HealthChecker) StartHealthEndpoint(addr string) error {
	mux := http.NewServeMux()
	mux.Handle("/health", h)
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			h.logger.Error("Failed to write health response", map[string]interface{}{
				"error": err.Error(),
			})
		}
	})

	h.logger.Info("Starting health check endpoint", map[string]interface{}{
		"address": addr,
	})

	return http.ListenAndServe(addr, mux)
}
