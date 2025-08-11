package health

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Status represents the health status of a component
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// Check represents a single health check
type Check struct {
	Name        string                 `json:"name"`
	Status      Status                 `json:"status"`
	Message     string                 `json:"message,omitempty"`
	LastChecked time.Time              `json:"last_checked"`
	Duration    time.Duration          `json:"duration_ms"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// HealthCheck interface for individual health checks
type HealthCheck interface {
	Name() string
	Check(ctx context.Context) error
}

// HealthChecker manages and executes health checks
type HealthChecker struct {
	checks  map[string]HealthCheck
	results map[string]*Check
	mu      sync.RWMutex

	metrics observability.MetricsClient
	logger  observability.Logger

	// Configuration
	checkInterval time.Duration
	timeout       time.Duration
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(logger observability.Logger, metrics observability.MetricsClient) *HealthChecker {
	return &HealthChecker{
		checks:        make(map[string]HealthCheck),
		results:       make(map[string]*Check),
		metrics:       metrics,
		logger:        logger,
		checkInterval: 30 * time.Second,
		timeout:       5 * time.Second,
	}
}

// RegisterCheck registers a new health check
func (h *HealthChecker) RegisterCheck(name string, check HealthCheck) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.checks[name] = check
	h.logger.Info("Registered health check", map[string]interface{}{
		"check": name,
	})
}

// RunChecks executes all registered health checks
func (h *HealthChecker) RunChecks(ctx context.Context) map[string]*Check {
	h.mu.RLock()
	checks := make(map[string]HealthCheck, len(h.checks))
	for name, check := range h.checks {
		checks[name] = check
	}
	h.mu.RUnlock()

	results := make(map[string]*Check)
	var wg sync.WaitGroup
	resultsChan := make(chan struct {
		name  string
		check *Check
	}, len(checks))

	for name, check := range checks {
		wg.Add(1)
		go func(n string, c HealthCheck) {
			defer wg.Done()

			checkCtx, cancel := context.WithTimeout(ctx, h.timeout)
			defer cancel()

			start := time.Now()
			err := c.Check(checkCtx)
			duration := time.Since(start)

			result := &Check{
				Name:        n,
				LastChecked: time.Now(),
				Duration:    duration,
				Metadata:    make(map[string]interface{}),
			}

			if err != nil {
				result.Status = StatusUnhealthy
				result.Message = err.Error()
			} else {
				result.Status = StatusHealthy
			}

			// Record metrics
			h.recordMetrics(n, result)

			resultsChan <- struct {
				name  string
				check *Check
			}{name: n, check: result}
		}(name, check)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	for r := range resultsChan {
		results[r.name] = r.check
	}

	// Update cached results
	h.mu.Lock()
	h.results = results
	h.mu.Unlock()

	return results
}

// GetResults returns the latest health check results
func (h *HealthChecker) GetResults() map[string]*Check {
	h.mu.RLock()
	defer h.mu.RUnlock()

	results := make(map[string]*Check, len(h.results))
	for k, v := range h.results {
		results[k] = v
	}
	return results
}

// IsHealthy returns true if all checks are healthy
func (h *HealthChecker) IsHealthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, check := range h.results {
		if check.Status != StatusHealthy {
			return false
		}
	}
	return true
}

// StartBackgroundChecks starts periodic health checks
func (h *HealthChecker) StartBackgroundChecks(ctx context.Context) {
	ticker := time.NewTicker(h.checkInterval)
	defer ticker.Stop()

	// Run initial check
	h.RunChecks(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.RunChecks(ctx)
		}
	}
}

func (h *HealthChecker) recordMetrics(name string, check *Check) {
	// Record check status
	statusValue := 0.0
	if check.Status == StatusHealthy {
		statusValue = 1.0
	}

	h.metrics.RecordGauge("health_check_status", statusValue, map[string]string{
		"component": name,
	})

	// Record check duration
	h.metrics.RecordHistogram("health_check_duration_seconds", check.Duration.Seconds(), map[string]string{
		"component": name,
	})
}

// DatabaseHealthCheck checks database connectivity
type DatabaseHealthCheck struct {
	db   *sql.DB
	name string
}

// NewDatabaseHealthCheck creates a new database health check
func NewDatabaseHealthCheck(name string, db *sql.DB) *DatabaseHealthCheck {
	return &DatabaseHealthCheck{
		db:   db,
		name: name,
	}
}

func (d *DatabaseHealthCheck) Name() string {
	return d.name
}

func (d *DatabaseHealthCheck) Check(ctx context.Context) error {
	// Check basic connectivity
	if err := d.db.PingContext(ctx); err != nil {
		return errors.Wrap(err, "database ping failed")
	}

	// Check pgvector extension
	var extensionExists bool
	query := `SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'vector')`
	if err := d.db.QueryRowContext(ctx, query).Scan(&extensionExists); err != nil {
		return errors.Wrap(err, "failed to check pgvector extension")
	}

	if !extensionExists {
		return errors.New("pgvector extension not installed")
	}

	return nil
}

// RedisHealthCheck checks Redis connectivity
type RedisHealthCheck struct {
	client *redis.Client
	name   string
}

// NewRedisHealthCheck creates a new Redis health check
func NewRedisHealthCheck(name string, client *redis.Client) *RedisHealthCheck {
	return &RedisHealthCheck{
		client: client,
		name:   name,
	}
}

func (r *RedisHealthCheck) Name() string {
	return r.name
}

func (r *RedisHealthCheck) Check(ctx context.Context) error {
	// Ping Redis
	if err := r.client.Ping(ctx).Err(); err != nil {
		return errors.Wrap(err, "redis ping failed")
	}

	// Check memory usage
	info, err := r.client.Info(ctx, "memory").Result()
	if err != nil {
		return errors.Wrap(err, "failed to get redis info")
	}

	// Basic check that we got some info back
	if len(info) == 0 {
		return errors.New("redis returned empty info")
	}

	return nil
}

// S3HealthCheck checks S3 bucket accessibility
type S3HealthCheck struct {
	client *s3.Client
	bucket string
	name   string
}

// NewS3HealthCheck creates a new S3 health check
func NewS3HealthCheck(name string, client *s3.Client, bucket string) *S3HealthCheck {
	return &S3HealthCheck{
		client: client,
		bucket: bucket,
		name:   name,
	}
}

func (s *S3HealthCheck) Name() string {
	return s.name
}

func (s *S3HealthCheck) Check(ctx context.Context) error {
	// Check bucket exists and is accessible
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: &s.bucket,
	})

	if err != nil {
		return errors.Wrapf(err, "failed to access S3 bucket %s", s.bucket)
	}

	return nil
}

// QueueHealthCheck checks Redis queue accessibility
type QueueHealthCheck struct {
	name  string
	queue interface {
		Health(context.Context) error
		GetQueueDepth(context.Context) (int64, error)
	}
}

// NewQueueHealthCheck creates a new queue health check
func NewQueueHealthCheck(name string, queue interface {
	Health(context.Context) error
	GetQueueDepth(context.Context) (int64, error)
}) *QueueHealthCheck {
	return &QueueHealthCheck{
		name:  name,
		queue: queue,
	}
}

func (q *QueueHealthCheck) Name() string {
	return q.name
}

func (q *QueueHealthCheck) Check(ctx context.Context) error {
	// Check basic health
	if err := q.queue.Health(ctx); err != nil {
		return errors.Wrapf(err, "queue health check failed")
	}

	// Check queue depth
	depth, err := q.queue.GetQueueDepth(ctx)
	if err != nil {
		// Log but don't fail health check
		fmt.Printf("Failed to get queue depth: %v\n", err)
		return nil
	}

	// Just log if queue depth is high, don't fail the health check
	if depth > 10000 {
		fmt.Printf("Queue depth is high: %d messages\n", depth)
	}

	return nil
}

// BedrockHealthCheck checks AWS Bedrock service availability
type BedrockHealthCheck struct {
	// In a real implementation, this would use the Bedrock client
	endpoint string
	name     string
}

// NewBedrockHealthCheck creates a new Bedrock health check
func NewBedrockHealthCheck(name string, endpoint string) *BedrockHealthCheck {
	return &BedrockHealthCheck{
		endpoint: endpoint,
		name:     name,
	}
}

func (b *BedrockHealthCheck) Name() string {
	return b.name
}

func (b *BedrockHealthCheck) Check(ctx context.Context) error {
	// In production, this would make a minimal API call to Bedrock
	// For now, we'll just check if the endpoint is configured
	if b.endpoint == "" {
		return errors.New("bedrock endpoint not configured")
	}

	// TODO: Add actual Bedrock API health check when client is available
	return nil
}

// ServiceHealthCheck checks if a dependent service is healthy
type ServiceHealthCheck struct {
	name      string
	checkFunc func(ctx context.Context) error
}

// NewServiceHealthCheck creates a new service health check
func NewServiceHealthCheck(name string, checkFunc func(ctx context.Context) error) *ServiceHealthCheck {
	return &ServiceHealthCheck{
		name:      name,
		checkFunc: checkFunc,
	}
}

func (s *ServiceHealthCheck) Name() string {
	return s.name
}

func (s *ServiceHealthCheck) Check(ctx context.Context) error {
	return s.checkFunc(ctx)
}

// AggregatedHealth represents the overall health status
type AggregatedHealth struct {
	Status      Status            `json:"status"`
	Message     string            `json:"message,omitempty"`
	Checks      map[string]*Check `json:"checks"`
	LastChecked time.Time         `json:"last_checked"`
	Version     string            `json:"version,omitempty"`
	Uptime      time.Duration     `json:"uptime_seconds,omitempty"`
}

// GetAggregatedHealth returns the aggregated health status
func (h *HealthChecker) GetAggregatedHealth() *AggregatedHealth {
	checks := h.GetResults()

	status := StatusHealthy
	var unhealthyCount int
	var degradedCount int

	for _, check := range checks {
		switch check.Status {
		case StatusUnhealthy:
			unhealthyCount++
		case StatusDegraded:
			degradedCount++
		}
	}

	message := ""
	if unhealthyCount > 0 {
		status = StatusUnhealthy
		message = fmt.Sprintf("%d components unhealthy", unhealthyCount)
	} else if degradedCount > 0 {
		status = StatusDegraded
		message = fmt.Sprintf("%d components degraded", degradedCount)
	}

	return &AggregatedHealth{
		Status:      status,
		Message:     message,
		Checks:      checks,
		LastChecked: time.Now(),
	}
}
