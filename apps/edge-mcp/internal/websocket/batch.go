package websocket

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	ws "github.com/developer-mesh/developer-mesh/pkg/models/websocket"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// BatchProcessor handles message batching for improved throughput
type BatchProcessor struct {
	mu              sync.Mutex
	pendingMessages []*BatchMessage
	batchSize       int
	maxBatchSize    int
	flushInterval   time.Duration
	flushTimer      *time.Timer
	sendFunc        func([]byte) error
	logger          observability.Logger
	metrics         observability.MetricsClient

	// Binary mode
	binaryMode bool

	// Statistics
	totalBatches  uint64
	totalMessages uint64
	batchSizes    []int
}

// BatchMessage wraps a message with metadata
type BatchMessage struct {
	ConnectionID string
	Message      []byte
	Timestamp    time.Time
}

// BatchConfig configures the batch processor
type BatchConfig struct {
	BatchSize     int           // Target batch size
	MaxBatchSize  int           // Maximum batch size
	FlushInterval time.Duration // Max time to wait before flushing
	BinaryMode    bool          // Use binary protocol for batching
}

// DefaultBatchConfig returns default batch configuration
func DefaultBatchConfig() *BatchConfig {
	return &BatchConfig{
		BatchSize:     10,
		MaxBatchSize:  100,
		FlushInterval: 10 * time.Millisecond,
		BinaryMode:    true,
	}
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(
	config *BatchConfig,
	sendFunc func([]byte) error,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *BatchProcessor {
	bp := &BatchProcessor{
		pendingMessages: make([]*BatchMessage, 0, config.MaxBatchSize),
		batchSize:       config.BatchSize,
		maxBatchSize:    config.MaxBatchSize,
		flushInterval:   config.FlushInterval,
		sendFunc:        sendFunc,
		logger:          logger,
		metrics:         metrics,
		binaryMode:      config.BinaryMode,
		batchSizes:      make([]int, 0, 1000),
	}

	// Start flush timer
	bp.resetTimer()

	return bp
}

// Add adds a message to the batch
func (bp *BatchProcessor) Add(connectionID string, message []byte) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	// Create batch message
	batchMsg := &BatchMessage{
		ConnectionID: connectionID,
		Message:      message,
		Timestamp:    time.Now(),
	}

	bp.pendingMessages = append(bp.pendingMessages, batchMsg)
	bp.totalMessages++

	// Check if we should flush
	if len(bp.pendingMessages) >= bp.batchSize {
		return bp.flushLocked()
	}

	return nil
}

// Flush forces a flush of pending messages
func (bp *BatchProcessor) Flush() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	return bp.flushLocked()
}

// flushLocked flushes pending messages (must be called with lock held)
func (bp *BatchProcessor) flushLocked() error {
	if len(bp.pendingMessages) == 0 {
		return nil
	}

	// Stop the timer
	if bp.flushTimer != nil {
		bp.flushTimer.Stop()
	}

	start := time.Now()

	// Create batch
	var batchData []byte
	var err error

	if bp.binaryMode {
		batchData, err = bp.createBinaryBatch()
	} else {
		batchData, err = bp.createJSONBatch()
	}

	if err != nil {
		bp.logger.Error("Failed to create batch", map[string]interface{}{
			"error": err.Error(),
			"size":  len(bp.pendingMessages),
		})
		return err
	}

	// Send batch
	if err := bp.sendFunc(batchData); err != nil {
		bp.logger.Error("Failed to send batch", map[string]interface{}{
			"error": err.Error(),
			"size":  len(bp.pendingMessages),
		})
		return err
	}

	// Update statistics
	batchSize := len(bp.pendingMessages)
	bp.totalBatches++
	bp.batchSizes = append(bp.batchSizes, batchSize)
	if len(bp.batchSizes) > 1000 {
		bp.batchSizes = bp.batchSizes[1:]
	}

	// Record metrics
	if bp.metrics != nil {
		bp.metrics.RecordHistogram("websocket_batch_size", float64(batchSize), nil)
		bp.metrics.RecordHistogram("websocket_batch_latency_seconds", time.Since(start).Seconds(), nil)
		bp.metrics.IncrementCounter("websocket_batches_sent_total", 1)
	}

	// Clear pending messages
	bp.pendingMessages = bp.pendingMessages[:0]

	// Reset timer
	bp.resetTimer()

	return nil
}

// createBinaryBatch creates a binary batch
func (bp *BatchProcessor) createBinaryBatch() ([]byte, error) {
	buf := GetBuffer()
	defer PutBuffer(buf)

	// Write batch header
	header := GetBinaryHeader()
	defer PutBinaryHeader(header)

	header.Magic = ws.MagicNumber
	header.Version = 1
	header.Type = uint8(ws.MessageTypeBatch)
	header.SetFlag(ws.FlagBatch)
	header.DataSize = 0 // Will calculate later

	// Reserve space for header
	headerSize := 24
	buf.Write(make([]byte, headerSize))

	// Write message count
	msgCount := len(bp.pendingMessages)
	if msgCount > int(^uint32(0)) {
		return nil, fmt.Errorf("message count exceeds uint32 max: %d", msgCount)
	}
	if err := binary.Write(buf, binary.BigEndian, uint32(msgCount)); err != nil { // #nosec G115 - Bounds checked above
		bp.logger.Error("Failed to write message count", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	// Write each message
	for _, msg := range bp.pendingMessages {
		// Write message length
		msgLen := len(msg.Message)
		if msgLen > int(^uint32(0)) {
			return nil, fmt.Errorf("message length exceeds uint32 max: %d", msgLen)
		}
		if err := binary.Write(buf, binary.BigEndian, uint32(msgLen)); err != nil { // #nosec G115 - Bounds checked above
			bp.logger.Error("Failed to write message length", map[string]interface{}{
				"error": err.Error(),
			})
			return nil, err
		}
		// Write message data
		buf.Write(msg.Message)
	}

	// Update header with actual data size
	data := buf.Bytes()
	dataSize := len(data) - headerSize
	if dataSize < 0 || dataSize > int(^uint32(0)) {
		return nil, fmt.Errorf("data size out of bounds: %d", dataSize)
	}
	header.DataSize = uint32(dataSize) // #nosec G115 - Bounds checked above

	// Write header to beginning of buffer
	headerBuf := bytes.NewBuffer(data[:0])
	if err := ws.WriteBinaryHeader(headerBuf, header); err != nil {
		bp.logger.Error("Failed to write binary header", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())

	return result, nil
}

// createJSONBatch creates a JSON batch
func (bp *BatchProcessor) createJSONBatch() ([]byte, error) {
	// Create batch message
	batch := &ws.Message{
		ID:     generateBatchID(),
		Type:   ws.MessageTypeBatch,
		Method: "batch",
		Params: map[string]interface{}{
			"messages": bp.pendingMessages,
			"count":    len(bp.pendingMessages),
		},
	}

	return json.Marshal(batch)
}

// resetTimer resets the flush timer
func (bp *BatchProcessor) resetTimer() {
	if bp.flushTimer != nil {
		bp.flushTimer.Stop()
	}

	bp.flushTimer = time.AfterFunc(bp.flushInterval, func() {
		if err := bp.Flush(); err != nil {
			bp.logger.Error("Failed to flush batch", map[string]interface{}{
				"error": err.Error(),
			})
		}
	})
}

// Close closes the batch processor
func (bp *BatchProcessor) Close() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	// Stop timer
	if bp.flushTimer != nil {
		bp.flushTimer.Stop()
	}

	// Flush any remaining messages
	return bp.flushLocked()
}

// Stats returns batch processor statistics
func (bp *BatchProcessor) Stats() BatchStats {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	stats := BatchStats{
		TotalBatches:  bp.totalBatches,
		TotalMessages: bp.totalMessages,
		PendingCount:  len(bp.pendingMessages),
	}

	// Calculate average batch size
	if len(bp.batchSizes) > 0 {
		sum := 0
		for _, size := range bp.batchSizes {
			sum += size
		}
		stats.AvgBatchSize = float64(sum) / float64(len(bp.batchSizes))
	}

	return stats
}

// BatchStats contains batch processor statistics
type BatchStats struct {
	TotalBatches  uint64
	TotalMessages uint64
	PendingCount  int
	AvgBatchSize  float64
}

// generateBatchID generates a unique batch ID
func generateBatchID() string {
	return "batch-" + time.Now().Format("20060102150405.000")
}

// ConnectionBatcher manages batching for a specific connection
type ConnectionBatcher struct {
	connectionID string
	processor    *BatchProcessor
	// mu field reserved for future thread-safe operations
	// mu           sync.Mutex
}

// NewConnectionBatcher creates a new connection batcher
func NewConnectionBatcher(connectionID string, processor *BatchProcessor) *ConnectionBatcher {
	return &ConnectionBatcher{
		connectionID: connectionID,
		processor:    processor,
	}
}

// Send adds a message to the batch for this connection
func (cb *ConnectionBatcher) Send(message []byte) error {
	return cb.processor.Add(cb.connectionID, message)
}

// Flush forces a flush of pending messages
func (cb *ConnectionBatcher) Flush() error {
	return cb.processor.Flush()
}

// BatchManager manages batch processors for all connections
type BatchManager struct {
	processors map[string]*BatchProcessor
	config     *BatchConfig
	logger     observability.Logger
	metrics    observability.MetricsClient
	mu         sync.RWMutex
}

// NewBatchManager creates a new batch manager
func NewBatchManager(
	config *BatchConfig,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *BatchManager {
	return &BatchManager{
		processors: make(map[string]*BatchProcessor),
		config:     config,
		logger:     logger,
		metrics:    metrics,
	}
}

// GetProcessor gets or creates a batch processor for a connection
func (bm *BatchManager) GetProcessor(connectionID string, sendFunc func([]byte) error) *BatchProcessor {
	bm.mu.RLock()
	processor, exists := bm.processors[connectionID]
	bm.mu.RUnlock()

	if exists {
		return processor
	}

	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Double-check after acquiring write lock
	if processor, exists := bm.processors[connectionID]; exists {
		return processor
	}

	// Create new processor
	processor = NewBatchProcessor(bm.config, sendFunc, bm.logger, bm.metrics)
	bm.processors[connectionID] = processor

	return processor
}

// RemoveProcessor removes a batch processor for a connection
func (bm *BatchManager) RemoveProcessor(connectionID string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	processor, exists := bm.processors[connectionID]
	if !exists {
		return nil
	}

	// Close processor (flushes pending messages)
	if err := processor.Close(); err != nil {
		return err
	}

	delete(bm.processors, connectionID)
	return nil
}

// FlushAll flushes all batch processors
func (bm *BatchManager) FlushAll() error {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	for _, processor := range bm.processors {
		if err := processor.Flush(); err != nil {
			return err
		}
	}

	return nil
}

// Close closes all batch processors
func (bm *BatchManager) Close() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	for id, processor := range bm.processors {
		if err := processor.Close(); err != nil {
			bm.logger.Error("Failed to close batch processor", map[string]interface{}{
				"connection_id": id,
				"error":         err.Error(),
			})
		}
	}

	bm.processors = make(map[string]*BatchProcessor)
	return nil
}
