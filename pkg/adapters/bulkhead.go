package adapters

import (
	"context"
	"fmt"
	"sync"
)

// Bulkhead implements the bulkhead pattern for fault isolation
// It limits concurrent requests and queues excess requests
type Bulkhead struct {
	semaphore chan struct{}
	queue     chan request
	closed    bool
	mu        sync.Mutex
}

type request struct {
	ctx    context.Context
	result chan error
}

// NewBulkhead creates a new bulkhead with specified limits
func NewBulkhead(maxConcurrent, queueSize int) *Bulkhead {
	b := &Bulkhead{
		semaphore: make(chan struct{}, maxConcurrent),
		queue:     make(chan request, queueSize),
	}

	// Pre-fill semaphore
	for i := 0; i < maxConcurrent; i++ {
		b.semaphore <- struct{}{}
	}

	// Start queue processor
	go b.processQueue()

	return b
}

// Acquire attempts to acquire a slot in the bulkhead
func (b *Bulkhead) Acquire(ctx context.Context) error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return fmt.Errorf("bulkhead is closed")
	}
	b.mu.Unlock()

	select {
	case <-b.semaphore:
		// Acquired immediately
		return nil
	case <-ctx.Done():
		// Context cancelled
		return ctx.Err()
	default:
		// Try to queue the request
		req := request{
			ctx:    ctx,
			result: make(chan error, 1),
		}

		select {
		case b.queue <- req:
			// Queued successfully, wait for result
			select {
			case err := <-req.result:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		default:
			// Queue is full
			return fmt.Errorf("bulkhead queue is full")
		}
	}
}

// Release releases a slot in the bulkhead
func (b *Bulkhead) Release() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.mu.Unlock()

	select {
	case b.semaphore <- struct{}{}:
		// Released
	default:
		// Semaphore is full (shouldn't happen)
	}
}

// processQueue processes queued requests
func (b *Bulkhead) processQueue() {
	for req := range b.queue {
		// Wait for a slot to become available
		select {
		case <-b.semaphore:
			// Acquired a slot for the queued request
			req.result <- nil
		case <-req.ctx.Done():
			// Request context cancelled
			req.result <- req.ctx.Err()
		}
	}
}

// Close closes the bulkhead
func (b *Bulkhead) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}

	b.closed = true
	close(b.queue)

	// Drain semaphore
	for {
		select {
		case <-b.semaphore:
		default:
			return
		}
	}
}

// Stats returns current bulkhead statistics
func (b *Bulkhead) Stats() BulkheadStats {
	b.mu.Lock()
	defer b.mu.Unlock()

	return BulkheadStats{
		ActiveRequests: cap(b.semaphore) - len(b.semaphore),
		QueuedRequests: len(b.queue),
		MaxConcurrent:  cap(b.semaphore),
		MaxQueueSize:   cap(b.queue),
		Closed:         b.closed,
	}
}

// BulkheadStats contains bulkhead statistics
type BulkheadStats struct {
	ActiveRequests int  `json:"active_requests"`
	QueuedRequests int  `json:"queued_requests"`
	MaxConcurrent  int  `json:"max_concurrent"`
	MaxQueueSize   int  `json:"max_queue_size"`
	Closed         bool `json:"closed"`
}
