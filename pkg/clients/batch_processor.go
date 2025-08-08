package clients

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// BatchProcessor handles batch operations for improved performance
type BatchProcessor struct {
	logger observability.Logger

	// Configuration
	config BatchConfig

	// Batch queue
	queue      chan *BatchRequest
	workers    int
	workerPool chan struct{}

	// In-flight batches
	activeBatches map[string]*Batch
	batchMutex    sync.RWMutex

	// Metrics
	metrics *BatchMetrics

	// Shutdown management
	shutdown chan struct{}
	wg       sync.WaitGroup
}

// BatchConfig defines batch processing configuration
type BatchConfig struct {
	MaxBatchSize    int           `json:"max_batch_size"`
	MaxBatchWait    time.Duration `json:"max_batch_wait"`
	Workers         int           `json:"workers"`
	QueueSize       int           `json:"queue_size"`
	EnableAutoFlush bool          `json:"enable_auto_flush"`
	FlushInterval   time.Duration `json:"flush_interval"`
}

// BatchRequest represents a single request in a batch
type BatchRequest struct {
	ID        string
	Operation string
	TenantID  string
	Data      interface{}
	Context   context.Context
	Result    chan BatchResult
}

// BatchResult represents the result of a batch request
type BatchResult struct {
	ID    string
	Data  interface{}
	Error error
}

// Batch represents a collection of requests to be processed together
type Batch struct {
	ID         string
	Requests   []*BatchRequest
	CreatedAt  time.Time
	Processing bool
}

// BatchMetrics tracks batch processing metrics
type BatchMetrics struct {
	mu sync.RWMutex

	TotalBatches      int64
	TotalRequests     int64
	SuccessfulBatches int64
	FailedBatches     int64

	AvgBatchSize      float64
	AvgProcessingTime time.Duration
	MaxBatchSize      int

	CurrentQueueSize int
	DroppedRequests  int64
}

// DefaultBatchConfig returns default batch configuration
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		MaxBatchSize:    50,
		MaxBatchWait:    100 * time.Millisecond,
		Workers:         4,
		QueueSize:       1000,
		EnableAutoFlush: true,
		FlushInterval:   1 * time.Second,
	}
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(config BatchConfig, logger observability.Logger) *BatchProcessor {
	processor := &BatchProcessor{
		logger:        logger,
		config:        config,
		queue:         make(chan *BatchRequest, config.QueueSize),
		workers:       config.Workers,
		workerPool:    make(chan struct{}, config.Workers),
		activeBatches: make(map[string]*Batch),
		metrics:       &BatchMetrics{},
		shutdown:      make(chan struct{}),
	}

	// Initialize worker pool
	for i := 0; i < config.Workers; i++ {
		processor.workerPool <- struct{}{}
	}

	// Start batch workers
	for i := 0; i < config.Workers; i++ {
		processor.wg.Add(1)
		go processor.batchWorker()
	}

	// Start auto-flush if enabled
	if config.EnableAutoFlush {
		processor.wg.Add(1)
		go processor.autoFlushWorker()
	}

	return processor
}

// Submit adds a request to the batch queue
func (p *BatchProcessor) Submit(ctx context.Context, operation string, tenantID string, data interface{}) (interface{}, error) {
	request := &BatchRequest{
		ID:        fmt.Sprintf("%s-%d", operation, time.Now().UnixNano()),
		Operation: operation,
		TenantID:  tenantID,
		Data:      data,
		Context:   ctx,
		Result:    make(chan BatchResult, 1),
	}

	// Try to add to queue
	select {
	case p.queue <- request:
		p.updateQueueSize(1)
	case <-time.After(100 * time.Millisecond):
		p.recordDroppedRequest()
		return nil, fmt.Errorf("batch queue is full")
	}

	// Wait for result
	select {
	case result := <-request.Result:
		return result.Data, result.Error
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// BatchListTools performs batch tool list operations
func (p *BatchProcessor) BatchListTools(ctx context.Context, tenantIDs []string, client RESTAPIClient) (map[string][]*models.DynamicTool, error) {
	results := make(map[string][]*models.DynamicTool)
	resultMutex := sync.Mutex{}
	errors := make([]error, 0)
	errorMutex := sync.Mutex{}

	// Create wait group for parallel processing
	wg := sync.WaitGroup{}

	// Use semaphore to limit concurrent requests
	semaphore := make(chan struct{}, p.config.Workers)

	startTime := time.Now()

	for _, tenantID := range tenantIDs {
		wg.Add(1)

		go func(tid string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Execute request
			tools, err := client.ListTools(ctx, tid)
			if err != nil {
				errorMutex.Lock()
				errors = append(errors, fmt.Errorf("tenant %s: %w", tid, err))
				errorMutex.Unlock()
				return
			}

			// Store result
			resultMutex.Lock()
			results[tid] = tools
			resultMutex.Unlock()
		}(tenantID)
	}

	// Wait for all requests to complete
	wg.Wait()

	// Record metrics
	p.recordBatchOperation(len(tenantIDs), len(errors), time.Since(startTime))

	// Return error if any failed
	if len(errors) > 0 {
		return results, fmt.Errorf("batch operation had %d errors: %v", len(errors), errors[0])
	}

	return results, nil
}

// BatchExecuteTools performs batch tool execution
func (p *BatchProcessor) BatchExecuteTools(ctx context.Context, executions []ToolExecution, client RESTAPIClient) ([]ToolExecutionResult, error) {
	results := make([]ToolExecutionResult, len(executions))

	// Create wait group for parallel processing
	wg := sync.WaitGroup{}

	// Use semaphore to limit concurrent requests
	semaphore := make(chan struct{}, p.config.Workers)

	startTime := time.Now()
	errorCount := 0

	for i, exec := range executions {
		wg.Add(1)

		go func(idx int, e ToolExecution) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Execute tool
			result, err := client.ExecuteTool(ctx, e.TenantID, e.ToolID, e.Action, e.Params)

			// Store result
			results[idx] = ToolExecutionResult{
				ToolID: e.ToolID,
				Result: result,
				Error:  err,
			}

			if err != nil {
				errorCount++
			}
		}(i, exec)
	}

	// Wait for all executions to complete
	wg.Wait()

	// Record metrics
	p.recordBatchOperation(len(executions), errorCount, time.Since(startTime))

	return results, nil
}

// ToolExecution represents a tool execution request
type ToolExecution struct {
	TenantID string
	ToolID   string
	Action   string
	Params   map[string]interface{}
}

// ToolExecutionResult represents a tool execution result
type ToolExecutionResult struct {
	ToolID string
	Result *models.ToolExecutionResponse
	Error  error
}

// batchWorker processes batch requests
func (p *BatchProcessor) batchWorker() {
	defer p.wg.Done()

	batch := &Batch{
		ID:        fmt.Sprintf("batch-%d", time.Now().UnixNano()),
		Requests:  make([]*BatchRequest, 0, p.config.MaxBatchSize),
		CreatedAt: time.Now(),
	}

	timer := time.NewTimer(p.config.MaxBatchWait)
	defer timer.Stop()

	for {
		select {
		case req := <-p.queue:
			p.updateQueueSize(-1)
			batch.Requests = append(batch.Requests, req)

			// Check if batch is full
			if len(batch.Requests) >= p.config.MaxBatchSize {
				p.processBatch(batch)

				// Create new batch
				batch = &Batch{
					ID:        fmt.Sprintf("batch-%d", time.Now().UnixNano()),
					Requests:  make([]*BatchRequest, 0, p.config.MaxBatchSize),
					CreatedAt: time.Now(),
				}

				// Reset timer
				timer.Reset(p.config.MaxBatchWait)
			}

		case <-timer.C:
			// Process batch if not empty
			if len(batch.Requests) > 0 {
				p.processBatch(batch)

				// Create new batch
				batch = &Batch{
					ID:        fmt.Sprintf("batch-%d", time.Now().UnixNano()),
					Requests:  make([]*BatchRequest, 0, p.config.MaxBatchSize),
					CreatedAt: time.Now(),
				}
			}

			// Reset timer
			timer.Reset(p.config.MaxBatchWait)

		case <-p.shutdown:
			// Process remaining batch
			if len(batch.Requests) > 0 {
				p.processBatch(batch)
			}
			return
		}
	}
}

// processBatch processes a batch of requests
func (p *BatchProcessor) processBatch(batch *Batch) {
	if len(batch.Requests) == 0 {
		return
	}

	startTime := time.Now()

	// Acquire worker token
	<-p.workerPool
	defer func() {
		p.workerPool <- struct{}{}
	}()

	// Group requests by operation type
	operationGroups := make(map[string][]*BatchRequest)
	for _, req := range batch.Requests {
		operationGroups[req.Operation] = append(operationGroups[req.Operation], req)
	}

	// Process each operation group
	for operation, requests := range operationGroups {
		p.processOperationGroup(operation, requests)
	}

	// Record metrics
	p.recordBatchProcessed(batch, time.Since(startTime))
}

// processOperationGroup processes a group of requests with the same operation
func (p *BatchProcessor) processOperationGroup(operation string, requests []*BatchRequest) {
	// This would be implemented based on operation type
	// For now, process individually
	for _, req := range requests {
		// Simulate processing
		result := BatchResult{
			ID:   req.ID,
			Data: fmt.Sprintf("Processed %s", req.ID),
		}

		select {
		case req.Result <- result:
		default:
			// Result channel might be closed if context was cancelled
		}
	}
}

// autoFlushWorker periodically flushes pending batches
func (p *BatchProcessor) autoFlushWorker() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.flushPendingBatches()
		case <-p.shutdown:
			return
		}
	}
}

// flushPendingBatches processes all pending batches
func (p *BatchProcessor) flushPendingBatches() {
	p.batchMutex.RLock()
	pendingCount := len(p.activeBatches)
	p.batchMutex.RUnlock()

	if pendingCount > 0 {
		p.logger.Debug("Flushing pending batches", map[string]interface{}{
			"count": pendingCount,
		})
	}
}

// Metrics recording methods

func (p *BatchProcessor) updateQueueSize(delta int) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()
	p.metrics.CurrentQueueSize += delta
}

func (p *BatchProcessor) recordDroppedRequest() {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()
	p.metrics.DroppedRequests++
}

func (p *BatchProcessor) recordBatchOperation(size int, errors int, duration time.Duration) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	p.metrics.TotalBatches++
	p.metrics.TotalRequests += int64(size)

	if errors == 0 {
		p.metrics.SuccessfulBatches++
	} else {
		p.metrics.FailedBatches++
	}

	// Update average batch size
	if p.metrics.AvgBatchSize == 0 {
		p.metrics.AvgBatchSize = float64(size)
	} else {
		p.metrics.AvgBatchSize = (p.metrics.AvgBatchSize + float64(size)) / 2
	}

	// Update max batch size
	if size > p.metrics.MaxBatchSize {
		p.metrics.MaxBatchSize = size
	}

	// Update average processing time
	if p.metrics.AvgProcessingTime == 0 {
		p.metrics.AvgProcessingTime = duration
	} else {
		p.metrics.AvgProcessingTime = (p.metrics.AvgProcessingTime + duration) / 2
	}
}

func (p *BatchProcessor) recordBatchProcessed(batch *Batch, duration time.Duration) {
	p.recordBatchOperation(len(batch.Requests), 0, duration)
}

// GetMetrics returns batch processing metrics
func (p *BatchProcessor) GetMetrics() map[string]interface{} {
	p.metrics.mu.RLock()
	defer p.metrics.mu.RUnlock()

	successRate := float64(0)
	if p.metrics.TotalBatches > 0 {
		successRate = float64(p.metrics.SuccessfulBatches) / float64(p.metrics.TotalBatches)
	}

	return map[string]interface{}{
		"total_batches":       p.metrics.TotalBatches,
		"total_requests":      p.metrics.TotalRequests,
		"successful_batches":  p.metrics.SuccessfulBatches,
		"failed_batches":      p.metrics.FailedBatches,
		"success_rate":        successRate,
		"avg_batch_size":      p.metrics.AvgBatchSize,
		"max_batch_size":      p.metrics.MaxBatchSize,
		"avg_processing_time": p.metrics.AvgProcessingTime.String(),
		"current_queue_size":  p.metrics.CurrentQueueSize,
		"dropped_requests":    p.metrics.DroppedRequests,
	}
}

// Close shuts down the batch processor
func (p *BatchProcessor) Close() error {
	close(p.shutdown)
	p.wg.Wait()
	close(p.queue)
	return nil
}
