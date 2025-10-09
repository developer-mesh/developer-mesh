package resilience

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/pkg/errors"
)

// Bulkhead errors
var (
	ErrBulkheadFull            = errors.New("bulkhead is full, cannot acquire resource")
	ErrBulkheadQueueFull       = errors.New("bulkhead queue is full, request rejected")
	ErrBulkheadTimeout         = errors.New("timeout waiting for bulkhead resource")
	ErrBulkheadContextCanceled = errors.New("context canceled while waiting for bulkhead resource")
)

// BulkheadConfig holds configuration for a bulkhead
type BulkheadConfig struct {
	// MaxConcurrentCalls is the maximum number of concurrent calls allowed
	MaxConcurrentCalls int

	// MaxQueueDepth is the maximum number of calls that can be queued
	// when all resources are in use. Set to 0 to disable queueing.
	MaxQueueDepth int

	// QueueTimeout is the maximum time a call can wait in the queue
	QueueTimeout time.Duration

	// RateLimitConfig is the rate limiting configuration for this bulkhead
	// If nil, no rate limiting is applied
	RateLimitConfig *RateLimiterConfig

	// EnableBackpressure enables backpressure when queue is full
	// If true, requests are rejected when queue is full
	// If false, requests wait indefinitely (subject to context timeout)
	EnableBackpressure bool
}

// Bulkhead implements the bulkhead pattern for resource isolation
type Bulkhead struct {
	name   string
	config BulkheadConfig

	// Semaphore for concurrent call limiting
	semaphore chan struct{}

	// Queue for pending operations
	queue chan *queuedOperation

	// Rate limiter (optional)
	rateLimiter *RateLimiter

	// Metrics
	activeRequests    atomic.Int64
	queuedRequests    atomic.Int64
	totalRequests     atomic.Int64
	rejectedRequests  atomic.Int64
	completedRequests atomic.Int64
	timedOutRequests  atomic.Int64

	// Observability
	logger  observability.Logger
	metrics observability.MetricsClient

	// Lifecycle management
	closed atomic.Bool
	wg     sync.WaitGroup
}

// queuedOperation represents an operation waiting in the queue
type queuedOperation struct {
	ctx       context.Context
	operation func(context.Context) (interface{}, error)
	result    chan operationResult
	queuedAt  time.Time
}

// operationResult holds the result of an operation
type operationResult struct {
	value interface{}
	err   error
}

// NewBulkhead creates a new bulkhead with the given configuration
func NewBulkhead(name string, config BulkheadConfig, logger observability.Logger, metrics observability.MetricsClient) *Bulkhead {
	// Apply defaults
	if config.MaxConcurrentCalls <= 0 {
		config.MaxConcurrentCalls = 10
	}
	if config.MaxQueueDepth < 0 {
		config.MaxQueueDepth = 0
	}
	if config.QueueTimeout <= 0 {
		config.QueueTimeout = 30 * time.Second
	}

	b := &Bulkhead{
		name:      name,
		config:    config,
		semaphore: make(chan struct{}, config.MaxConcurrentCalls),
		logger:    logger,
		metrics:   metrics,
	}

	// Initialize queue if configured
	if config.MaxQueueDepth > 0 {
		b.queue = make(chan *queuedOperation, config.MaxQueueDepth)
		// Start queue processor
		b.wg.Add(1)
		go b.processQueue()
	}

	// Initialize rate limiter if configured
	if config.RateLimitConfig != nil {
		b.rateLimiter = NewRateLimiter(name, *config.RateLimitConfig)
	}

	return b
}

// Execute executes the given operation with bulkhead protection
func (b *Bulkhead) Execute(ctx context.Context, operation func(context.Context) (interface{}, error)) (interface{}, error) {
	if b.closed.Load() {
		return nil, errors.New("bulkhead is closed")
	}

	b.totalRequests.Add(1)
	b.recordMetric("bulkhead_requests_total", 1, map[string]string{"bulkhead": b.name})

	// Check rate limit if configured
	if b.rateLimiter != nil {
		if !b.rateLimiter.Allow() {
			b.rejectedRequests.Add(1)
			b.recordMetric("bulkhead_rate_limited_total", 1, map[string]string{"bulkhead": b.name})
			return nil, fmt.Errorf("rate limit exceeded for bulkhead %s", b.name)
		}
	}

	// Try to acquire resource immediately
	select {
	case b.semaphore <- struct{}{}:
		// Resource acquired, execute immediately
		return b.executeWithResource(ctx, operation)
	default:
		// Resource not available, try to queue or reject
		return b.handleResourceUnavailable(ctx, operation)
	}
}

// executeWithResource executes the operation while holding a resource
func (b *Bulkhead) executeWithResource(ctx context.Context, operation func(context.Context) (interface{}, error)) (interface{}, error) {
	defer func() {
		<-b.semaphore // Release resource
		b.activeRequests.Add(-1)
		b.recordMetric("bulkhead_active_requests", float64(b.activeRequests.Load()), map[string]string{"bulkhead": b.name})
	}()

	b.activeRequests.Add(1)
	b.recordMetric("bulkhead_active_requests", float64(b.activeRequests.Load()), map[string]string{"bulkhead": b.name})

	start := time.Now()
	result, err := operation(ctx)
	duration := time.Since(start)

	b.completedRequests.Add(1)
	b.recordMetric("bulkhead_completed_total", 1, map[string]string{"bulkhead": b.name})
	b.recordMetric("bulkhead_execution_duration_seconds", duration.Seconds(), map[string]string{"bulkhead": b.name})

	if err != nil {
		b.recordMetric("bulkhead_errors_total", 1, map[string]string{"bulkhead": b.name})
	}

	return result, err
}

// handleResourceUnavailable handles the case when all resources are in use
func (b *Bulkhead) handleResourceUnavailable(ctx context.Context, operation func(context.Context) (interface{}, error)) (interface{}, error) {
	// If queueing is disabled, reject immediately
	if b.config.MaxQueueDepth == 0 {
		b.rejectedRequests.Add(1)
		b.recordMetric("bulkhead_rejected_total", 1, map[string]string{"bulkhead": b.name, "reason": "no_queue"})
		return nil, ErrBulkheadFull
	}

	// Try to queue the operation
	queuedOp := &queuedOperation{
		ctx:       ctx,
		operation: operation,
		result:    make(chan operationResult, 1),
		queuedAt:  time.Now(),
	}

	// Apply backpressure if enabled
	if b.config.EnableBackpressure {
		select {
		case b.queue <- queuedOp:
			// Successfully queued
			b.queuedRequests.Add(1)
			b.recordMetric("bulkhead_queued_requests", float64(b.queuedRequests.Load()), map[string]string{"bulkhead": b.name})
		default:
			// Queue is full, reject with backpressure
			b.rejectedRequests.Add(1)
			b.recordMetric("bulkhead_rejected_total", 1, map[string]string{"bulkhead": b.name, "reason": "queue_full"})
			return nil, ErrBulkheadQueueFull
		}
	} else {
		// Block until can queue (may block indefinitely if queue stays full)
		select {
		case b.queue <- queuedOp:
			b.queuedRequests.Add(1)
			b.recordMetric("bulkhead_queued_requests", float64(b.queuedRequests.Load()), map[string]string{"bulkhead": b.name})
		case <-ctx.Done():
			b.rejectedRequests.Add(1)
			b.recordMetric("bulkhead_rejected_total", 1, map[string]string{"bulkhead": b.name, "reason": "context_canceled"})
			return nil, ErrBulkheadContextCanceled
		}
	}

	// Wait for result with timeout
	timeout := time.NewTimer(b.config.QueueTimeout)
	defer timeout.Stop()

	select {
	case result := <-queuedOp.result:
		queueWaitTime := time.Since(queuedOp.queuedAt)
		b.recordMetric("bulkhead_queue_wait_seconds", queueWaitTime.Seconds(), map[string]string{"bulkhead": b.name})
		return result.value, result.err
	case <-timeout.C:
		b.timedOutRequests.Add(1)
		b.recordMetric("bulkhead_timeouts_total", 1, map[string]string{"bulkhead": b.name})
		return nil, ErrBulkheadTimeout
	case <-ctx.Done():
		b.rejectedRequests.Add(1)
		b.recordMetric("bulkhead_rejected_total", 1, map[string]string{"bulkhead": b.name, "reason": "context_canceled"})
		return nil, ctx.Err()
	}
}

// processQueue processes queued operations
func (b *Bulkhead) processQueue() {
	defer b.wg.Done()

	for queuedOp := range b.queue {
		// Wait for resource to become available
		select {
		case b.semaphore <- struct{}{}:
			// Resource acquired, execute the operation
			b.queuedRequests.Add(-1)
			b.recordMetric("bulkhead_queued_requests", float64(b.queuedRequests.Load()), map[string]string{"bulkhead": b.name})

			go func(op *queuedOperation) {
				result, err := b.executeWithResource(op.ctx, op.operation)
				op.result <- operationResult{value: result, err: err}
				close(op.result)
			}(queuedOp)
		case <-queuedOp.ctx.Done():
			// Context canceled while waiting
			b.queuedRequests.Add(-1)
			b.rejectedRequests.Add(1)
			queuedOp.result <- operationResult{err: ErrBulkheadContextCanceled}
			close(queuedOp.result)
		}
	}
}

// GetStats returns current bulkhead statistics
func (b *Bulkhead) GetStats() BulkheadStats {
	return BulkheadStats{
		Name:              b.name,
		ActiveRequests:    b.activeRequests.Load(),
		QueuedRequests:    b.queuedRequests.Load(),
		TotalRequests:     b.totalRequests.Load(),
		RejectedRequests:  b.rejectedRequests.Load(),
		CompletedRequests: b.completedRequests.Load(),
		TimedOutRequests:  b.timedOutRequests.Load(),
		MaxConcurrent:     int64(b.config.MaxConcurrentCalls),
		MaxQueueDepth:     int64(b.config.MaxQueueDepth),
	}
}

// BulkheadStats holds bulkhead statistics
type BulkheadStats struct {
	Name              string
	ActiveRequests    int64
	QueuedRequests    int64
	TotalRequests     int64
	RejectedRequests  int64
	CompletedRequests int64
	TimedOutRequests  int64
	MaxConcurrent     int64
	MaxQueueDepth     int64
}

// Close closes the bulkhead and waits for all operations to complete
func (b *Bulkhead) Close() error {
	if b.closed.Swap(true) {
		return errors.New("bulkhead already closed")
	}

	// Close queue if it exists
	if b.queue != nil {
		close(b.queue)
		b.wg.Wait()
	}

	return nil
}

// recordMetric is a helper to safely record metrics
func (b *Bulkhead) recordMetric(name string, value float64, labels map[string]string) {
	if b.metrics != nil {
		b.metrics.RecordGauge(name, value, labels)
	}
}

// BulkheadManager manages multiple bulkheads for different resources/tenants
type BulkheadManager struct {
	bulkheads map[string]*Bulkhead
	configs   map[string]BulkheadConfig
	mutex     sync.RWMutex
	logger    observability.Logger
	metrics   observability.MetricsClient
}

// NewBulkheadManager creates a new bulkhead manager
func NewBulkheadManager(defaultConfigs map[string]BulkheadConfig, logger observability.Logger, metrics observability.MetricsClient) *BulkheadManager {
	manager := &BulkheadManager{
		bulkheads: make(map[string]*Bulkhead),
		configs:   make(map[string]BulkheadConfig),
		logger:    logger,
		metrics:   metrics,
	}

	// Store configs and create bulkheads
	for name, config := range defaultConfigs {
		manager.configs[name] = config
		manager.bulkheads[name] = NewBulkhead(name, config, logger, metrics)
	}

	return manager
}

// GetBulkhead gets or creates a bulkhead for the given resource
func (m *BulkheadManager) GetBulkhead(name string) *Bulkhead {
	m.mutex.RLock()
	bulkhead, exists := m.bulkheads[name]
	m.mutex.RUnlock()

	if exists {
		return bulkhead
	}

	// Create new bulkhead with default config
	config, exists := m.configs[name]
	if !exists {
		// Use default configuration
		config = BulkheadConfig{
			MaxConcurrentCalls: 10,
			MaxQueueDepth:      100,
			QueueTimeout:       30 * time.Second,
			EnableBackpressure: true,
		}
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check again in case it was created while waiting for lock
	bulkhead, exists = m.bulkheads[name]
	if exists {
		return bulkhead
	}

	// Create new bulkhead
	bulkhead = NewBulkhead(name, config, m.logger, m.metrics)
	m.bulkheads[name] = bulkhead

	return bulkhead
}

// Execute executes an operation through the specified bulkhead
func (m *BulkheadManager) Execute(ctx context.Context, bulkheadName string, operation func(context.Context) (interface{}, error)) (interface{}, error) {
	bulkhead := m.GetBulkhead(bulkheadName)
	return bulkhead.Execute(ctx, operation)
}

// GetAllStats returns statistics for all bulkheads
func (m *BulkheadManager) GetAllStats() map[string]BulkheadStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := make(map[string]BulkheadStats)
	for name, bulkhead := range m.bulkheads {
		stats[name] = bulkhead.GetStats()
	}

	return stats
}

// Close closes all bulkheads
func (m *BulkheadManager) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var errs []error
	for name, bulkhead := range m.bulkheads {
		if err := bulkhead.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close bulkhead %s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing bulkheads: %v", errs)
	}

	return nil
}

// DefaultBulkheadConfigs provides default configurations for common resources
var DefaultBulkheadConfigs = map[string]BulkheadConfig{
	"github_api": {
		MaxConcurrentCalls: 50,
		MaxQueueDepth:      200,
		QueueTimeout:       60 * time.Second,
		EnableBackpressure: true,
		RateLimitConfig: &RateLimiterConfig{
			Limit:       100,
			Period:      time.Minute,
			BurstFactor: 2,
		},
	},
	"harness_api": {
		MaxConcurrentCalls: 30,
		MaxQueueDepth:      100,
		QueueTimeout:       45 * time.Second,
		EnableBackpressure: true,
		RateLimitConfig: &RateLimiterConfig{
			Limit:       50,
			Period:      time.Minute,
			BurstFactor: 2,
		},
	},
	"database": {
		MaxConcurrentCalls: 100,
		MaxQueueDepth:      500,
		QueueTimeout:       10 * time.Second,
		EnableBackpressure: true,
	},
	"cache": {
		MaxConcurrentCalls: 200,
		MaxQueueDepth:      1000,
		QueueTimeout:       5 * time.Second,
		EnableBackpressure: true,
	},
	"agent_execution": {
		MaxConcurrentCalls: 20,
		MaxQueueDepth:      50,
		QueueTimeout:       120 * time.Second,
		EnableBackpressure: true,
		RateLimitConfig: &RateLimiterConfig{
			Limit:       30,
			Period:      time.Minute,
			BurstFactor: 1,
		},
	},
	"workflow_execution": {
		MaxConcurrentCalls: 15,
		MaxQueueDepth:      30,
		QueueTimeout:       180 * time.Second,
		EnableBackpressure: true,
		RateLimitConfig: &RateLimiterConfig{
			Limit:       20,
			Period:      time.Minute,
			BurstFactor: 1,
		},
	},
}
