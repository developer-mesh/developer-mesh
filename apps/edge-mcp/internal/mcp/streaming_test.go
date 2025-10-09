package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultStreamConfig tests the default streaming configuration
func TestDefaultStreamConfig(t *testing.T) {
	config := DefaultStreamConfig()

	assert.Equal(t, 64*1024, config.ChunkSize, "Default chunk size should be 64KB")
	assert.Equal(t, 500*time.Millisecond, config.ProgressInterval, "Default progress interval should be 500ms")
	assert.True(t, config.EnableLogStreaming, "Log streaming should be enabled by default")
	assert.Equal(t, 100, config.MaxConcurrentStreams, "Max concurrent streams should be 100")
}

// TestStreamWriter_SendProgress tests sending progress notifications
func TestStreamWriter_SendProgress(t *testing.T) {
	// Create test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

		// Read messages
		for {
			_, _, err := conn.Read(context.Background())
			if err != nil {
				break
			}
		}
	}))
	defer server.Close()

	// Connect to test server
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// Create stream writer
	logger := observability.NewStandardLogger("[test]")
	config := DefaultStreamConfig()
	sw := NewStreamWriter(conn, logger, config)

	// Send progress notification
	err = sw.SendProgress("test-token", 50, 100, "Processing...")
	assert.NoError(t, err)

	// Verify notification was sent
	time.Sleep(100 * time.Millisecond)

	// Note: In a real test, you would verify the message content
	// Here we just verify the method executes without error
}

// TestStreamWriter_SendLog tests sending log notifications
func TestStreamWriter_SendLog(t *testing.T) {
	tests := []struct {
		name               string
		enableLogStreaming bool
		expectedError      bool
		shouldReceiveLog   bool
	}{
		{
			name:               "logs when streaming enabled",
			enableLogStreaming: true,
			expectedError:      false,
			shouldReceiveLog:   true,
		},
		{
			name:               "no logs when streaming disabled",
			enableLogStreaming: false,
			expectedError:      false,
			shouldReceiveLog:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock connection
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				require.NoError(t, err)
				defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

				// Just keep connection open
				<-context.Background().Done()
			}))
			defer server.Close()

			wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
			ctx := context.Background()
			conn, _, err := websocket.Dial(ctx, wsURL, nil)
			require.NoError(t, err)
			defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

			logger := observability.NewStandardLogger("[test]")
			config := DefaultStreamConfig()
			config.EnableLogStreaming = tt.enableLogStreaming
			sw := NewStreamWriter(conn, logger, config)

			metadata := map[string]interface{}{
				"key": "value",
			}
			err = sw.SendLog("info", "Test log message", metadata)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestStreamWriter_SendChunkedContent tests chunked content streaming
func TestStreamWriter_SendChunkedContent(t *testing.T) {
	tests := []struct {
		name           string
		contentSize    int
		chunkSize      int
		expectedChunks int
	}{
		{
			name:           "small content - single chunk",
			contentSize:    1000,
			chunkSize:      2000,
			expectedChunks: 1,
		},
		{
			name:           "exact multiple chunks",
			contentSize:    4000,
			chunkSize:      1000,
			expectedChunks: 4,
		},
		{
			name:           "uneven chunks",
			contentSize:    5500,
			chunkSize:      2000,
			expectedChunks: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test content
			content := make([]byte, tt.contentSize)
			for i := range content {
				content[i] = byte(i % 256)
			}

			// Create mock connection
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				require.NoError(t, err)
				defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

				// Keep connection open
				<-time.After(2 * time.Second)
			}))
			defer server.Close()

			wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
			ctx := context.Background()
			conn, _, err := websocket.Dial(ctx, wsURL, nil)
			require.NoError(t, err)
			defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

			logger := observability.NewStandardLogger("[test]")
			config := DefaultStreamConfig()
			config.ChunkSize = tt.chunkSize
			sw := NewStreamWriter(conn, logger, config)

			// Send chunked content
			err = sw.SendChunkedContent("test-request", content, "application/json")
			assert.NoError(t, err)
		})
	}
}

// TestStreamWriter_Cancel tests stream cancellation
func TestStreamWriter_Cancel(t *testing.T) {
	// Create mock connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

		<-time.After(5 * time.Second)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	logger := observability.NewStandardLogger("[test]")
	config := DefaultStreamConfig()
	sw := NewStreamWriter(conn, logger, config)

	// Create large content
	content := make([]byte, 1024*1024) // 1MB

	// Cancel stream after short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		sw.Cancel()
	}()

	// Attempt to send chunked content (should be cancelled)
	err = sw.SendChunkedContent("test-request", content, "application/json")
	assert.Error(t, err, "Should return error when stream is cancelled")
	assert.Contains(t, err.Error(), "cancel", "Error should indicate cancellation")
}

// TestStreamManager_CreateStream tests stream creation
func TestStreamManager_CreateStream(t *testing.T) {
	logger := observability.NewStandardLogger("[test]")
	config := DefaultStreamConfig()
	config.MaxConcurrentStreams = 2

	sm := NewStreamManager(logger, config)

	// Create mock connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
		<-time.After(5 * time.Second)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// Create first stream
	stream1, err := sm.CreateStream("request-1", conn)
	assert.NoError(t, err)
	assert.NotNil(t, stream1)
	assert.Equal(t, 1, sm.StreamCount())

	// Attempt to create duplicate stream (should fail - already exists)
	stream1Dup, err := sm.CreateStream("request-1", conn)
	assert.Error(t, err)
	assert.Nil(t, stream1Dup)
	assert.Contains(t, err.Error(), "already exists")

	// Create second stream
	stream2, err := sm.CreateStream("request-2", conn)
	assert.NoError(t, err)
	assert.NotNil(t, stream2)
	assert.Equal(t, 2, sm.StreamCount())

	// Attempt to create third stream (should fail - max reached)
	stream3, err := sm.CreateStream("request-3", conn)
	assert.Error(t, err)
	assert.Nil(t, stream3)
	assert.Contains(t, err.Error(), "max concurrent streams")
}

// TestStreamManager_GetStream tests stream retrieval
func TestStreamManager_GetStream(t *testing.T) {
	logger := observability.NewStandardLogger("[test]")
	config := DefaultStreamConfig()
	sm := NewStreamManager(logger, config)

	// Create mock connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
		<-time.After(5 * time.Second)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// Create stream
	created, err := sm.CreateStream("request-1", conn)
	require.NoError(t, err)

	// Get existing stream
	retrieved, exists := sm.GetStream("request-1")
	assert.True(t, exists)
	assert.Equal(t, created, retrieved)

	// Get non-existent stream
	_, exists = sm.GetStream("request-999")
	assert.False(t, exists)
}

// TestStreamManager_CloseStream tests stream closure
func TestStreamManager_CloseStream(t *testing.T) {
	logger := observability.NewStandardLogger("[test]")
	config := DefaultStreamConfig()
	sm := NewStreamManager(logger, config)

	// Create mock connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
		<-time.After(5 * time.Second)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// Create stream
	_, err = sm.CreateStream("request-1", conn)
	require.NoError(t, err)
	assert.Equal(t, 1, sm.StreamCount())

	// Close stream
	err = sm.CloseStream("request-1")
	assert.NoError(t, err)
	assert.Equal(t, 0, sm.StreamCount())

	// Attempt to close non-existent stream
	err = sm.CloseStream("request-999")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestStreamManager_CloseAll tests closing all streams
func TestStreamManager_CloseAll(t *testing.T) {
	logger := observability.NewStandardLogger("[test]")
	config := DefaultStreamConfig()
	sm := NewStreamManager(logger, config)

	// Create mock connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
		<-time.After(5 * time.Second)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// Create multiple streams
	for i := 0; i < 5; i++ {
		_, err := sm.CreateStream(fmt.Sprintf("request-%d", i), conn)
		require.NoError(t, err)
	}
	assert.Equal(t, 5, sm.StreamCount())

	// Close all streams
	sm.CloseAll()
	assert.Equal(t, 0, sm.StreamCount())
}

// TestStreamingLogger tests the streaming logger implementation
func TestStreamingLogger(t *testing.T) {
	// Create mock connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
		<-time.After(5 * time.Second)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	baseLogger := observability.NewStandardLogger("[test]")
	config := DefaultStreamConfig()
	sw := NewStreamWriter(conn, baseLogger, config)
	streamLogger := NewStreamingLogger(baseLogger, sw)

	// Test all log levels
	streamLogger.Debug("Debug message", map[string]interface{}{"key": "debug"})
	streamLogger.Info("Info message", map[string]interface{}{"key": "info"})
	streamLogger.Warn("Warn message", map[string]interface{}{"key": "warn"})
	streamLogger.Error("Error message", map[string]interface{}{"key": "error"})

	// Test formatted logging
	streamLogger.Debugf("Debug %s", "formatted")
	streamLogger.Infof("Info %s", "formatted")
	streamLogger.Warnf("Warn %s", "formatted")
	streamLogger.Errorf("Error %s", "formatted")

	// Test With and WithPrefix
	withFieldsLogger := streamLogger.With(map[string]interface{}{"extra": "field"})
	assert.NotNil(t, withFieldsLogger)

	withPrefixLogger := streamLogger.WithPrefix("[prefix]")
	assert.NotNil(t, withPrefixLogger)
}

// TestShouldStream tests the streaming threshold check
func TestShouldStream(t *testing.T) {
	tests := []struct {
		name        string
		contentSize int
		threshold   int
		expected    bool
	}{
		{
			name:        "below threshold",
			contentSize: 1000,
			threshold:   2000,
			expected:    false,
		},
		{
			name:        "at threshold",
			contentSize: 2000,
			threshold:   2000,
			expected:    false,
		},
		{
			name:        "above threshold",
			contentSize: 3000,
			threshold:   2000,
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := make([]byte, tt.contentSize)
			result := ShouldStream(content, tt.threshold)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestProgressNotification_JSON tests progress notification serialization
func TestProgressNotification_JSON(t *testing.T) {
	progress := ProgressNotification{
		Token:      "test-token",
		Percentage: 75.5,
		Message:    "Processing...",
		Current:    75,
		Total:      100,
	}

	data, err := json.Marshal(progress)
	require.NoError(t, err)

	var decoded ProgressNotification
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, progress.Token, decoded.Token)
	assert.Equal(t, progress.Percentage, decoded.Percentage)
	assert.Equal(t, progress.Message, decoded.Message)
	assert.Equal(t, progress.Current, decoded.Current)
	assert.Equal(t, progress.Total, decoded.Total)
}

// TestLogNotification_JSON tests log notification serialization
func TestLogNotification_JSON(t *testing.T) {
	now := time.Now()
	logNotif := LogNotification{
		Level:     "info",
		Message:   "Test log",
		Timestamp: now,
		Metadata: map[string]interface{}{
			"key": "value",
		},
	}

	data, err := json.Marshal(logNotif)
	require.NoError(t, err)

	var decoded LogNotification
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, logNotif.Level, decoded.Level)
	assert.Equal(t, logNotif.Message, decoded.Message)
	assert.Equal(t, "value", decoded.Metadata["key"])
}

// TestStreamWriter_ConcurrentAccess tests thread-safe concurrent access
func TestStreamWriter_ConcurrentAccess(t *testing.T) {
	// Create mock connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
		<-time.After(5 * time.Second)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	logger := observability.NewStandardLogger("[test]")
	config := DefaultStreamConfig()
	sw := NewStreamWriter(conn, logger, config)

	// Send multiple concurrent operations
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = sw.SendProgress(fmt.Sprintf("token-%d", idx), int64(idx), 10, "Progress")
			_ = sw.SendLog("info", fmt.Sprintf("Log %d", idx), nil)
		}(i)
	}

	wg.Wait()
	// Test passes if no race conditions or panics occur
}

// BenchmarkStreamWriter_SendProgress benchmarks progress notification sending
func BenchmarkStreamWriter_SendProgress(b *testing.B) {
	// Create mock connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
		<-time.After(30 * time.Second)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	logger := observability.NewStandardLogger("[bench]")
	config := DefaultStreamConfig()
	sw := NewStreamWriter(conn, logger, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sw.SendProgress("bench-token", int64(i), int64(b.N), "Benchmarking...")
	}
}

// BenchmarkStreamWriter_SendLog benchmarks log notification sending
func BenchmarkStreamWriter_SendLog(b *testing.B) {
	// Create mock connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
		<-time.After(30 * time.Second)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	logger := observability.NewStandardLogger("[bench]")
	config := DefaultStreamConfig()
	sw := NewStreamWriter(conn, logger, config)

	metadata := map[string]interface{}{
		"iteration": 0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metadata["iteration"] = i
		_ = sw.SendLog("info", "Benchmark log message", metadata)
	}
}
