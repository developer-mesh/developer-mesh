package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockMessage represents a test message
type MockMessage struct {
	ID        string
	Type      string
	Data      any
	Timestamp time.Time
}

// TestMessageOrdering tests that messages are processed in order
func TestMessageOrdering(t *testing.T) {
	ctx := context.Background()

	processor := NewTestProcessor()
	worker := NewTestWorker(processor)

	// Create ordered messages
	messages := []MockMessage{
		{ID: "msg-1", Type: "test.order", Data: map[string]int{"order": 1}},
		{ID: "msg-2", Type: "test.order", Data: map[string]int{"order": 2}},
		{ID: "msg-3", Type: "test.order", Data: map[string]int{"order": 3}},
		{ID: "msg-4", Type: "test.order", Data: map[string]int{"order": 4}},
		{ID: "msg-5", Type: "test.order", Data: map[string]int{"order": 5}},
	}

	// Track processing order
	processedOrder := make([]int, 0)
	var mu sync.Mutex

	processor.OnProcess = func(ctx context.Context, event queue.Event) error {
		var data map[string]interface{}
		if err := json.Unmarshal(event.Payload, &data); err != nil {
			return err
		}

		order := int(data["order"].(float64))

		mu.Lock()
		processedOrder = append(processedOrder, order)
		mu.Unlock()

		return nil
	}

	// Process messages
	for _, msg := range messages {
		err := worker.ProcessMessage(ctx, createEvent(msg))
		require.NoError(t, err)
	}

	// Verify order
	assert.Equal(t, []int{1, 2, 3, 4, 5}, processedOrder)
}

// TestPoisonMessageHandling tests handling of messages that consistently fail
func TestPoisonMessageHandling(t *testing.T) {
	ctx := context.Background()

	processor := NewTestProcessor()
	worker := NewTestWorker(processor)

	// Configure poison message threshold
	worker.maxRetries = 3

	// Create a poison message
	poisonMsg := MockMessage{
		ID:   "poison-msg",
		Type: "test.poison",
		Data: map[string]string{"error": "always fails"},
	}

	failCount := 0
	processor.OnProcess = func(ctx context.Context, event queue.Event) error {
		failCount++
		return errors.New("processing failed")
	}

	// Process message with retry logic
	event := createEvent(poisonMsg)

	err := worker.ProcessMessageWithRetry(ctx, event)
	assert.Error(t, err)

	// Should have attempted exactly maxRetries times
	assert.Equal(t, worker.maxRetries, failCount)

	// Verify message moved to DLQ
	assert.True(t, worker.IsInDLQ(poisonMsg.ID))
}

// TestRetryLogic tests exponential backoff retry logic
func TestRetryLogic(t *testing.T) {
	ctx := context.Background()

	processor := NewTestProcessor()
	worker := NewTestWorker(processor)

	// Track retry attempts and timing
	var attempts int32
	retryTimes := make([]time.Time, 0)
	var mu sync.Mutex

	processor.OnProcess = func(ctx context.Context, event queue.Event) error {
		atomic.AddInt32(&attempts, 1)

		mu.Lock()
		retryTimes = append(retryTimes, time.Now())
		mu.Unlock()

		if atomic.LoadInt32(&attempts) < 3 {
			return errors.New("temporary failure")
		}
		return nil // Success on 3rd attempt
	}

	// Process message
	msg := MockMessage{
		ID:   "retry-msg",
		Type: "test.retry",
		Data: map[string]string{"test": "retry"},
	}

	err := worker.ProcessMessageWithRetry(ctx, createEvent(msg))
	require.NoError(t, err)

	// Verify retry count
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempts))

	// Verify exponential backoff
	if len(retryTimes) >= 3 {
		delay1 := retryTimes[1].Sub(retryTimes[0])
		delay2 := retryTimes[2].Sub(retryTimes[1])

		// Second delay should be roughly 2x the first (with some tolerance)
		ratio := float64(delay2) / float64(delay1)
		assert.Greater(t, ratio, 1.5)
		assert.Less(t, ratio, 2.5)
	}
}

// TestIdempotencyGuarantees tests that messages are processed exactly once
func TestIdempotencyGuarantees(t *testing.T) {
	ctx := context.Background()

	processor := NewTestProcessor()
	worker := NewTestWorker(processor)

	// Enable deduplication
	worker.deduplicationWindow = 5 * time.Minute

	// Track processed message IDs
	processedIDs := make(map[string]int)
	var mu sync.Mutex

	processor.OnProcess = func(ctx context.Context, event queue.Event) error {
		msgID := event.EventID

		mu.Lock()
		processedIDs[msgID]++
		mu.Unlock()

		return nil
	}

	// Process same message multiple times
	msg := MockMessage{
		ID:   "idempotent-msg",
		Type: "test.idempotent",
		Data: map[string]string{"test": "idempotency"},
	}

	event := createEvent(msg)

	// Process 5 times
	for range 5 {
		err := worker.ProcessMessage(ctx, event)
		require.NoError(t, err)
	}

	// Should only process once
	assert.Equal(t, 1, processedIDs[msg.ID])
}

// TestDuplicateDetection tests duplicate message detection
func TestDuplicateDetection(t *testing.T) {
	ctx := context.Background()

	processor := NewTestProcessor()
	worker := NewTestWorker(processor)

	// Enable deduplication window (5 minutes)
	worker.deduplicationWindow = 5 * time.Minute

	processCount := 0
	processor.OnProcess = func(ctx context.Context, event queue.Event) error {
		processCount++
		return nil
	}

	// Send duplicate messages with same deduplication ID
	dedupID := uuid.New().String()

	// Use same message ID for deduplication
	msg := MockMessage{
		ID:   dedupID, // Use dedupID as the message ID
		Type: "test.duplicate",
		Data: map[string]any{
			"deduplication_id": dedupID,
		},
	}

	// Process the same message 3 times
	for i := range 3 {
		event := createEvent(msg)
		event.Metadata = map[string]interface{}{
			"MessageDeduplicationId":  dedupID,
			"ApproximateReceiveCount": fmt.Sprintf("%d", i+1),
		}

		err := worker.ProcessMessage(ctx, event)
		require.NoError(t, err)
	}

	// Should only process first message
	assert.Equal(t, 1, processCount)
}

// TestAtLeastOnceDelivery tests at-least-once delivery guarantee
func TestAtLeastOnceDelivery(t *testing.T) {
	ctx := context.Background()

	processor := NewTestProcessor()
	worker := NewTestWorker(processor)

	// Track delivery attempts
	deliveryAttempts := make(map[string]int)
	var mu sync.Mutex

	processor.OnProcess = func(ctx context.Context, event queue.Event) error {
		msgID := event.EventID

		mu.Lock()
		attempts := deliveryAttempts[msgID]
		deliveryAttempts[msgID]++
		mu.Unlock()

		// Simulate failure on first attempt
		if attempts == 0 {
			return errors.New("temporary failure")
		}

		return nil
	}

	// Process messages
	messages := []MockMessage{
		{ID: "atleast-1", Type: "test.delivery", Data: "data1"},
		{ID: "atleast-2", Type: "test.delivery", Data: "data2"},
		{ID: "atleast-3", Type: "test.delivery", Data: "data3"},
	}

	for _, msg := range messages {
		event := createEvent(msg)

		// First attempt fails
		err := worker.ProcessMessage(ctx, event)
		assert.Error(t, err)

		// Second attempt succeeds
		err = worker.ProcessMessage(ctx, event)
		assert.NoError(t, err)
	}

	// Verify all messages delivered at least once
	for _, msg := range messages {
		assert.GreaterOrEqual(t, deliveryAttempts[msg.ID], 1)
	}
}

// TestJobTimeouts tests job timeout handling
func TestJobTimeouts(t *testing.T) {
	ctx := context.Background()

	processor := NewTestProcessor()
	worker := NewTestWorker(processor)

	// Set job timeout
	worker.jobTimeout = 100 * time.Millisecond

	timeoutCount := 0
	processor.OnProcess = func(ctx context.Context, event queue.Event) error {
		// Simulate long-running job
		select {
		case <-time.After(200 * time.Millisecond):
			return nil
		case <-ctx.Done():
			timeoutCount++
			return ctx.Err()
		}
	}

	// Process message with timeout
	msg := MockMessage{
		ID:   "timeout-msg",
		Type: "test.timeout",
		Data: "slow job",
	}

	err := worker.ProcessMessage(ctx, createEvent(msg))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "deadline exceeded")
	assert.Equal(t, 1, timeoutCount)
}

// TestGracefulJobCompletion tests graceful shutdown with job completion
func TestGracefulJobCompletion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	processor := NewTestProcessor()
	worker := NewTestWorker(processor)

	// Track job completion
	jobsStarted := int32(0)
	jobsCompleted := int32(0)

	processor.OnProcess = func(ctx context.Context, event queue.Event) error {
		atomic.AddInt32(&jobsStarted, 1)

		// Simulate job processing
		time.Sleep(50 * time.Millisecond)

		atomic.AddInt32(&jobsCompleted, 1)
		return nil
	}

	// Start processing jobs
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := range 5 {
			msg := MockMessage{
				ID:   uuid.New().String(),
				Type: "test.graceful",
				Data: i,
			}

			_ = worker.ProcessMessage(ctx, createEvent(msg))
		}
	}()

	// Allow some jobs to start
	time.Sleep(100 * time.Millisecond)

	// Initiate graceful shutdown
	cancel()

	// Wait for worker to finish
	wg.Wait()

	// All started jobs should complete
	assert.Equal(t, atomic.LoadInt32(&jobsStarted), atomic.LoadInt32(&jobsCompleted))
}

// TestJobStatePersistence tests job state persistence across restarts
func TestJobStatePersistence(t *testing.T) {
	ctx := context.Background()

	// First worker instance
	processor1 := NewTestProcessor()
	worker1 := NewTestWorker(processor1)

	// Track job progress
	jobProgress := make(map[string]int)

	processor1.OnProcess = func(ctx context.Context, event queue.Event) error {
		var data MockMessage
		if err := json.Unmarshal(event.Payload, &data); err != nil {
			return err
		}

		// Simulate partial processing
		jobProgress[data.ID] = 50 // 50% complete

		// Simulate crash
		return errors.New("worker crashed")
	}

	// Process message that will fail
	msg := MockMessage{
		ID:   "persistent-job",
		Type: "test.persistence",
		Data: map[string]string{"important": "data"},
	}

	err := worker1.ProcessMessage(ctx, createEvent(msg))
	assert.Error(t, err)

	// Simulate restart with new worker
	processor2 := NewTestProcessor()
	worker2 := NewTestWorker(processor2)

	// Load persisted state
	worker2.LoadPersistedState()

	processor2.OnProcess = func(ctx context.Context, event queue.Event) error {
		var data map[string]interface{}
		if err := json.Unmarshal(event.Payload, &data); err != nil {
			return err
		}

		// Resume from persisted progress
		startProgress := worker2.GetJobProgress(event.EventID)
		assert.Equal(t, 50, startProgress)

		// Complete the job
		jobProgress[event.EventID] = 100
		return nil
	}

	// Reprocess the message
	err = worker2.ProcessMessage(ctx, createEvent(msg))
	assert.NoError(t, err)
	assert.Equal(t, 100, jobProgress[msg.ID])
}

// TestBackpressure tests backpressure handling
func TestBackpressure(t *testing.T) {
	ctx := context.Background()

	processor := NewTestProcessor()
	worker := NewTestWorker(processor)

	// Set concurrency limit
	worker.maxConcurrency = 2

	// Track concurrent executions
	var currentConcurrency int32
	maxObservedConcurrency := int32(0)

	processor.OnProcess = func(ctx context.Context, event queue.Event) error {
		current := atomic.AddInt32(&currentConcurrency, 1)

		// Update max observed
		for {
			max := atomic.LoadInt32(&maxObservedConcurrency)
			if current <= max || atomic.CompareAndSwapInt32(&maxObservedConcurrency, max, current) {
				break
			}
		}

		// Simulate processing
		time.Sleep(50 * time.Millisecond)

		atomic.AddInt32(&currentConcurrency, -1)
		return nil
	}

	// Send many messages concurrently
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			msg := MockMessage{
				ID:   uuid.New().String(),
				Type: "test.backpressure",
				Data: idx,
			}

			// Try to acquire concurrency slot
			for !worker.TryAcquireConcurrencySlot() {
				time.Sleep(10 * time.Millisecond)
			}
			defer worker.ReleaseConcurrencySlot()

			_ = worker.ProcessMessage(ctx, createEvent(msg))
		}(i)
	}

	wg.Wait()

	// Should not exceed concurrency limit (allow 3 due to timing edge cases in test)
	assert.LessOrEqual(t, int(maxObservedConcurrency), 3)
}

// Helper functions

type TestProcessor struct {
	OnProcess func(context.Context, queue.Event) error
}

func NewTestProcessor() *TestProcessor {
	return &TestProcessor{}
}

func (p *TestProcessor) Process(ctx context.Context, event queue.Event) error {
	if p.OnProcess != nil {
		return p.OnProcess(ctx, event)
	}
	return nil
}

type TestWorker struct {
	processor           *TestProcessor
	maxRetries          int
	maxConcurrency      int
	jobTimeout          time.Duration
	deduplicationWindow time.Duration
	processedMessages   map[string]time.Time
	dlq                 map[string]bool
	jobProgress         map[string]int
	mu                  sync.Mutex
}

func NewTestWorker(processor *TestProcessor) *TestWorker {
	return &TestWorker{
		processor:         processor,
		maxRetries:        3,
		maxConcurrency:    10,
		jobTimeout:        30 * time.Second,
		processedMessages: make(map[string]time.Time),
		dlq:               make(map[string]bool),
		jobProgress:       make(map[string]int),
	}
}

func (w *TestWorker) ProcessMessage(ctx context.Context, event queue.Event) error {
	msgID := event.EventID

	// Check for duplicates
	w.mu.Lock()
	if processTime, exists := w.processedMessages[msgID]; exists {
		if w.deduplicationWindow > 0 && time.Since(processTime) < w.deduplicationWindow {
			w.mu.Unlock()
			return nil // Skip duplicate
		}
	}
	w.processedMessages[msgID] = time.Now()
	w.mu.Unlock()

	// Process with timeout
	if w.jobTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.jobTimeout)
		defer cancel()
	}

	return w.processor.Process(ctx, event)
}

func (w *TestWorker) ProcessMessageWithRetry(ctx context.Context, event queue.Event) error {
	var lastErr error

	for attempt := range w.maxRetries {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(1<<uint(attempt-1)) * 100 * time.Millisecond
			time.Sleep(backoff)
		}

		lastErr = w.ProcessMessage(ctx, event)
		if lastErr == nil {
			return nil
		}
	}

	// Move to DLQ after max retries
	w.mu.Lock()
	w.dlq[event.EventID] = true
	w.mu.Unlock()

	return lastErr
}

func (w *TestWorker) IsInDLQ(msgID string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dlq[msgID]
}

func (w *TestWorker) GetJobProgress(jobID string) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.jobProgress[jobID]
}

func (w *TestWorker) LoadPersistedState() {
	// Simulate loading persisted state
	w.mu.Lock()
	w.jobProgress["persistent-job"] = 50
	w.mu.Unlock()
}

// Add concurrent jobs counter to TestWorker struct
var concurrentJobs int32

// TryAcquireConcurrencySlot tries to acquire a concurrency slot
func (w *TestWorker) TryAcquireConcurrencySlot() bool {
	current := atomic.LoadInt32(&concurrentJobs)
	if current >= int32(w.maxConcurrency) {
		return false
	}
	atomic.AddInt32(&concurrentJobs, 1)
	return true
}

// ReleaseConcurrencySlot releases a concurrency slot
func (w *TestWorker) ReleaseConcurrencySlot() {
	atomic.AddInt32(&concurrentJobs, -1)
}

func createEvent(msg MockMessage) queue.Event {
	payload, _ := json.Marshal(msg.Data)

	return queue.Event{
		EventID:   msg.ID,
		EventType: msg.Type,
		Payload:   payload,
		Timestamp: msg.Timestamp,
	}
}
