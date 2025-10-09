package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/utils"
)

// StreamConfig defines configuration for streaming responses
type StreamConfig struct {
	// ChunkSize is the maximum size of each chunk in bytes
	ChunkSize int
	// ProgressInterval is how often to send progress updates
	ProgressInterval time.Duration
	// EnableLogStreaming enables streaming of execution logs
	EnableLogStreaming bool
	// MaxConcurrentStreams limits the number of concurrent streams
	MaxConcurrentStreams int
}

// DefaultStreamConfig returns the default streaming configuration
func DefaultStreamConfig() StreamConfig {
	return StreamConfig{
		ChunkSize:            64 * 1024, // 64KB chunks
		ProgressInterval:     500 * time.Millisecond,
		EnableLogStreaming:   true,
		MaxConcurrentStreams: 100,
	}
}

// ProgressNotification represents a progress update notification
type ProgressNotification struct {
	Token      string  `json:"token"`      // Unique token for this operation
	Percentage float64 `json:"percentage"` // Progress percentage (0-100)
	Message    string  `json:"message"`    // Human-readable progress message
	Current    int64   `json:"current"`    // Current progress value
	Total      int64   `json:"total"`      // Total value (for percentage calculation)
}

// LogNotification represents a log message notification
type LogNotification struct {
	Level     string                 `json:"level"`     // Log level (debug, info, warn, error)
	Message   string                 `json:"message"`   // Log message
	Timestamp time.Time              `json:"timestamp"` // When the log was generated
	Metadata  map[string]interface{} `json:"metadata"`  // Additional metadata
}

// StreamWriter manages streaming responses over WebSocket
type StreamWriter struct {
	conn       *websocket.Conn
	mu         sync.Mutex
	logger     observability.Logger
	config     StreamConfig
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// NewStreamWriter creates a new stream writer for a WebSocket connection
func NewStreamWriter(conn *websocket.Conn, logger observability.Logger, config StreamConfig) *StreamWriter {
	ctx, cancel := context.WithCancel(context.Background())
	return &StreamWriter{
		conn:       conn,
		logger:     logger,
		config:     config,
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

// SendProgress sends a progress notification to the client
func (sw *StreamWriter) SendProgress(token string, current, total int64, message string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	percentage := float64(0)
	if total > 0 {
		percentage = (float64(current) / float64(total)) * 100
	}

	notification := &MCPMessage{
		JSONRPC: "2.0",
		Method:  "$/progress",
		Params: json.RawMessage(mustMarshal(&ProgressNotification{
			Token:      token,
			Percentage: percentage,
			Message:    message,
			Current:    current,
			Total:      total,
		})),
	}

	if err := wsjson.Write(sw.ctx, sw.conn, notification); err != nil {
		sw.logger.Error("Failed to send progress notification", map[string]interface{}{
			"error": err.Error(),
			"token": token,
		})
		return fmt.Errorf("failed to send progress notification: %w", err)
	}

	return nil
}

// SendLog sends a log message notification to the client
func (sw *StreamWriter) SendLog(level, message string, metadata map[string]interface{}) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if !sw.config.EnableLogStreaming {
		return nil // Log streaming disabled
	}

	notification := &MCPMessage{
		JSONRPC: "2.0",
		Method:  "$/logMessage",
		Params: json.RawMessage(mustMarshal(&LogNotification{
			Level:     level,
			Message:   message,
			Timestamp: time.Now(),
			Metadata:  metadata,
		})),
	}

	if err := wsjson.Write(sw.ctx, sw.conn, notification); err != nil {
		sw.logger.Warn("Failed to send log notification", map[string]interface{}{
			"error": err.Error(),
			"level": level,
		})
		return fmt.Errorf("failed to send log notification: %w", err)
	}

	return nil
}

// SendChunkedContent sends large content in chunks with progress updates
func (sw *StreamWriter) SendChunkedContent(requestID interface{}, content []byte, contentType string) error {
	totalChunks := (len(content) + sw.config.ChunkSize - 1) / sw.config.ChunkSize
	token := fmt.Sprintf("chunk_%v", requestID)

	sw.logger.Info("Streaming chunked content", map[string]interface{}{
		"request_id":   requestID,
		"total_bytes":  len(content),
		"total_chunks": totalChunks,
		"chunk_size":   sw.config.ChunkSize,
	})

	// Send initial progress
	if err := sw.SendProgress(token, 0, int64(totalChunks), "Starting content streaming"); err != nil {
		return err
	}

	// Send content in chunks
	for i := 0; i < totalChunks; i++ {
		// Check for cancellation
		select {
		case <-sw.ctx.Done():
			sw.logger.Info("Stream cancelled", map[string]interface{}{
				"request_id":      requestID,
				"chunks_sent":     i,
				"chunks_total":    totalChunks,
				"bytes_sent":      i * sw.config.ChunkSize,
				"bytes_remaining": len(content) - (i * sw.config.ChunkSize),
			})
			return fmt.Errorf("stream cancelled: %w", sw.ctx.Err())
		default:
		}

		start := i * sw.config.ChunkSize
		end := start + sw.config.ChunkSize
		if end > len(content) {
			end = len(content)
		}

		chunk := content[start:end]

		// Create chunk response
		chunkResponse := &MCPMessage{
			JSONRPC: "2.0",
			Method:  "$/contentChunk",
			Params: json.RawMessage(mustMarshal(map[string]interface{}{
				"requestId":   requestID,
				"chunk":       i,
				"totalChunks": totalChunks,
				"contentType": contentType,
				"data":        string(chunk),
				"isLast":      i == totalChunks-1,
			})),
		}

		sw.mu.Lock()
		if err := wsjson.Write(sw.ctx, sw.conn, chunkResponse); err != nil {
			sw.mu.Unlock()
			sw.logger.Error("Failed to send content chunk", map[string]interface{}{
				"error":      err.Error(),
				"request_id": requestID,
				"chunk":      i,
			})
			return fmt.Errorf("failed to send chunk %d: %w", i, err)
		}
		sw.mu.Unlock()

		// Send progress update
		if err := sw.SendProgress(token, int64(i+1), int64(totalChunks),
			fmt.Sprintf("Sent chunk %d of %d", i+1, totalChunks)); err != nil {
			return err
		}

		sw.logger.Debug("Sent content chunk", map[string]interface{}{
			"request_id": requestID,
			"chunk":      i,
			"chunk_size": len(chunk),
			"progress":   fmt.Sprintf("%.1f%%", float64(i+1)/float64(totalChunks)*100),
		})
	}

	// Send completion progress
	if err := sw.SendProgress(token, int64(totalChunks), int64(totalChunks), "Content streaming complete"); err != nil {
		return err
	}

	sw.logger.Info("Completed chunked content streaming", map[string]interface{}{
		"request_id":  requestID,
		"total_bytes": len(content),
		"chunks_sent": totalChunks,
	})

	return nil
}

// SendFinalResponse sends the final response after streaming is complete
func (sw *StreamWriter) SendFinalResponse(requestID interface{}, result interface{}) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	response := &MCPMessage{
		JSONRPC: "2.0",
		ID:      requestID,
		Result:  result,
	}

	if err := wsjson.Write(sw.ctx, sw.conn, response); err != nil {
		sw.logger.Error("Failed to send final response", map[string]interface{}{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return fmt.Errorf("failed to send final response: %w", err)
	}

	return nil
}

// Cancel cancels the stream and any in-progress operations
func (sw *StreamWriter) Cancel() {
	sw.cancelFunc()
	sw.logger.Debug("Stream cancelled", nil)
}

// Close closes the stream writer and releases resources
func (sw *StreamWriter) Close() error {
	sw.cancelFunc()
	return nil
}

// StreamManager manages multiple concurrent streaming operations
type StreamManager struct {
	streams map[interface{}]*StreamWriter
	mu      sync.RWMutex
	config  StreamConfig
	logger  observability.Logger
}

// NewStreamManager creates a new stream manager
func NewStreamManager(logger observability.Logger, config StreamConfig) *StreamManager {
	return &StreamManager{
		streams: make(map[interface{}]*StreamWriter),
		config:  config,
		logger:  logger,
	}
}

// CreateStream creates a new stream for a request
func (sm *StreamManager) CreateStream(requestID interface{}, conn *websocket.Conn) (*StreamWriter, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check concurrent stream limit
	if len(sm.streams) >= sm.config.MaxConcurrentStreams {
		return nil, fmt.Errorf("max concurrent streams (%d) reached", sm.config.MaxConcurrentStreams)
	}

	// Check if stream already exists
	if _, exists := sm.streams[requestID]; exists {
		return nil, fmt.Errorf("stream for request %v already exists", requestID)
	}

	stream := NewStreamWriter(conn, sm.logger, sm.config)
	sm.streams[requestID] = stream

	sm.logger.Debug("Created new stream", map[string]interface{}{
		"request_id":      requestID,
		"active_streams":  len(sm.streams),
		"max_streams":     sm.config.MaxConcurrentStreams,
		"stream_capacity": fmt.Sprintf("%.1f%%", float64(len(sm.streams))/float64(sm.config.MaxConcurrentStreams)*100),
	})

	return stream, nil
}

// GetStream retrieves an existing stream
func (sm *StreamManager) GetStream(requestID interface{}) (*StreamWriter, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stream, exists := sm.streams[requestID]
	return stream, exists
}

// CloseStream closes and removes a stream
func (sm *StreamManager) CloseStream(requestID interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	stream, exists := sm.streams[requestID]
	if !exists {
		return fmt.Errorf("stream for request %v not found", requestID)
	}

	if err := stream.Close(); err != nil {
		sm.logger.Warn("Error closing stream", map[string]interface{}{
			"error":      err.Error(),
			"request_id": requestID,
		})
	}

	delete(sm.streams, requestID)

	sm.logger.Debug("Closed stream", map[string]interface{}{
		"request_id":     requestID,
		"active_streams": len(sm.streams),
	})

	return nil
}

// CloseAll closes all active streams
func (sm *StreamManager) CloseAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for requestID, stream := range sm.streams {
		if err := stream.Close(); err != nil {
			sm.logger.Warn("Error closing stream during shutdown", map[string]interface{}{
				"error":      err.Error(),
				"request_id": requestID,
			})
		}
	}

	sm.streams = make(map[interface{}]*StreamWriter)

	sm.logger.Info("Closed all streams", nil)
}

// StreamCount returns the number of active streams
func (sm *StreamManager) StreamCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.streams)
}

// StreamingLogger wraps a logger to send logs to a stream
type StreamingLogger struct {
	baseLogger observability.Logger
	stream     *StreamWriter
}

// NewStreamingLogger creates a logger that sends logs to a stream
func NewStreamingLogger(baseLogger observability.Logger, stream *StreamWriter) *StreamingLogger {
	return &StreamingLogger{
		baseLogger: baseLogger,
		stream:     stream,
	}
}

// Debug logs a debug message and sends it to the stream
func (sl *StreamingLogger) Debug(msg string, fields map[string]interface{}) {
	// Redact sensitive data before logging anywhere (server or client)
	redactedFields := utils.RedactSensitiveData(fields)
	sl.baseLogger.Debug(msg, redactedFields)
	_ = sl.stream.SendLog("debug", msg, redactedFields)
}

// Info logs an info message and sends it to the stream
func (sl *StreamingLogger) Info(msg string, fields map[string]interface{}) {
	// Redact sensitive data before logging anywhere (server or client)
	redactedFields := utils.RedactSensitiveData(fields)
	sl.baseLogger.Info(msg, redactedFields)
	_ = sl.stream.SendLog("info", msg, redactedFields)
}

// Warn logs a warning message and sends it to the stream
func (sl *StreamingLogger) Warn(msg string, fields map[string]interface{}) {
	// Redact sensitive data before logging anywhere (server or client)
	redactedFields := utils.RedactSensitiveData(fields)
	sl.baseLogger.Warn(msg, redactedFields)
	_ = sl.stream.SendLog("warn", msg, redactedFields)
}

// Error logs an error message and sends it to the stream
func (sl *StreamingLogger) Error(msg string, fields map[string]interface{}) {
	// Redact sensitive data before logging anywhere (server or client)
	redactedFields := utils.RedactSensitiveData(fields)
	sl.baseLogger.Error(msg, redactedFields)
	_ = sl.stream.SendLog("error", msg, redactedFields)
}

// Fatal logs a fatal message and sends it to the stream
func (sl *StreamingLogger) Fatal(msg string, fields map[string]interface{}) {
	// Redact sensitive data before logging anywhere (server or client)
	redactedFields := utils.RedactSensitiveData(fields)
	sl.baseLogger.Fatal(msg, redactedFields)
	_ = sl.stream.SendLog("fatal", msg, redactedFields)
}

// Debugf logs a formatted debug message
func (sl *StreamingLogger) Debugf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	sl.Debug(msg, nil)
}

// Infof logs a formatted info message
func (sl *StreamingLogger) Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	sl.Info(msg, nil)
}

// Warnf logs a formatted warning message
func (sl *StreamingLogger) Warnf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	sl.Warn(msg, nil)
}

// Errorf logs a formatted error message
func (sl *StreamingLogger) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	sl.Error(msg, nil)
}

// Fatalf logs a formatted fatal message
func (sl *StreamingLogger) Fatalf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	sl.Fatal(msg, nil)
}

// With returns a new logger with additional fields
func (sl *StreamingLogger) With(fields map[string]interface{}) observability.Logger {
	return &StreamingLogger{
		baseLogger: sl.baseLogger.With(fields),
		stream:     sl.stream,
	}
}

// WithPrefix returns a new logger with a prefix
func (sl *StreamingLogger) WithPrefix(prefix string) observability.Logger {
	return &StreamingLogger{
		baseLogger: sl.baseLogger.WithPrefix(prefix),
		stream:     sl.stream,
	}
}

// mustMarshal marshals v to JSON or panics
func mustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal: %v", err))
	}
	return data
}

// ShouldStream determines if a response should be streamed based on size
func ShouldStream(content []byte, threshold int) bool {
	return len(content) > threshold
}

// StreamThreshold is the default threshold for streaming responses (32KB)
const StreamThreshold = 32 * 1024
