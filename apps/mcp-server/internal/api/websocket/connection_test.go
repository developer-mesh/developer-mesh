package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	ws "github.com/developer-mesh/developer-mesh/pkg/models/websocket"
)

// TestConnectionReadPump tests the connection read pump
func TestConnectionReadPump(t *testing.T) {
	// Create test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer func() {
			if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
				t.Logf("Error closing connection: %v", err)
			}
		}()

		// Create mock logger
		mockLogger := &MockLogger{}
		mockLogger.On("Error", mock.Anything, mock.Anything).Return()
		mockLogger.On("Warn", mock.Anything, mock.Anything).Return()
		mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
		mockLogger.On("Info", mock.Anything, mock.Anything).Return()

		// Create mock hub
		mockHub := &Server{
			connections:      make(map[string]*Connection),
			mu:               sync.RWMutex{},
			logger:           mockLogger,
			metricsCollector: NewMetricsCollector(nil),
			config: Config{
				PingInterval: 30 * time.Second,
				PongTimeout:  60 * time.Second,
			},
		}

		// Create test connection
		testConn := NewConnection("test-conn", conn, mockHub)
		testConn.AgentID = "agent-1"
		testConn.TenantID = "tenant-1"
		testConn.SetState(ws.ConnectionStateConnected)

		// Run read pump in goroutine
		go testConn.readPump()

		// Send test message
		msg := ws.Message{
			ID:     "test-msg",
			Type:   ws.MessageTypeRequest,
			Method: "test.method",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = wsjson.Write(ctx, conn, msg)
		assert.NoError(t, err)

		// Wait for processing
		time.Sleep(100 * time.Millisecond)

		// Close connection
		if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
			t.Logf("Error closing connection: %v", err)
		}
	}))
	defer server.Close()

	// Connect as client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)
	defer func() {
		if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
			t.Logf("Error closing connection: %v", err)
		}
	}()

	// Send message
	msg := ws.Message{
		ID:     "client-msg",
		Type:   ws.MessageTypeRequest,
		Method: "client.test",
	}
	err = wsjson.Write(ctx, conn, msg)
	assert.NoError(t, err)
}

// TestConnectionWritePump tests the connection write pump
func TestConnectionWritePump(t *testing.T) {
	// Create test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer func() {
			if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
				t.Logf("Error closing connection: %v", err)
			}
		}()

		// Create mock logger
		mockLogger := &MockLogger{}
		mockLogger.On("Error", mock.Anything, mock.Anything).Return()
		mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
		mockLogger.On("Info", mock.Anything, mock.Anything).Return()
		mockLogger.On("Warn", mock.Anything, mock.Anything).Return()

		// Create mock hub
		mockHub := &Server{
			logger:           mockLogger,
			metricsCollector: NewMetricsCollector(nil),
			config: Config{
				PingInterval: 100 * time.Millisecond,
				PongTimeout:  200 * time.Millisecond,
			},
		}

		// Create test connection
		testConn := NewConnection("test-conn", conn, mockHub)
		testConn.TenantID = "tenant-1"

		// Run write pump
		go testConn.writePump()

		// Send test message through channel
		testMsg := ws.Message{
			ID:   "response-1",
			Type: ws.MessageTypeResponse,
			Result: map[string]interface{}{
				"status": "ok",
			},
		}

		data, err := json.Marshal(testMsg)
		require.NoError(t, err)

		testConn.send <- data

		// Wait for message to be sent
		time.Sleep(50 * time.Millisecond)

		// Close send channel to stop write pump
		close(testConn.send)
	}))
	defer server.Close()

	// Connect as client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)
	defer func() {
		if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
			t.Logf("Error closing connection: %v", err)
		}
	}()

	// Read message sent by server
	var msg ws.Message
	err = wsjson.Read(ctx, conn, &msg)
	assert.NoError(t, err)
	assert.Equal(t, "response-1", msg.ID)
	assert.Equal(t, ws.MessageTypeResponse, msg.Type)
}

// TestConnectionSendError tests error message sending
func TestConnectionSendError(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Error", mock.Anything, mock.Anything).Return()
	mockLogger.On("Warn", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	mockHub := &Server{
		logger:           mockLogger,
		metricsCollector: NewMetricsCollector(nil),
	}

	conn := NewConnection("test-conn", nil, mockHub)

	// Send error
	conn.sendError("req-123", ws.ErrCodeInvalidParams, "Invalid request", nil)

	// Check message was queued
	select {
	case data := <-conn.send:
		var msg ws.Message
		err := json.Unmarshal(data, &msg)
		assert.NoError(t, err)
		assert.Equal(t, "req-123", msg.ID)
		assert.Equal(t, ws.MessageTypeError, msg.Type)
		assert.NotNil(t, msg.Error)
		assert.Equal(t, ws.ErrCodeInvalidParams, msg.Error.Code)
		assert.Equal(t, "Invalid request", msg.Error.Message)
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for error message")
	}

	// Test with full channel
	for i := 0; i < 256; i++ {
		conn.send <- []byte("dummy")
	}

	// Should not block
	conn.sendError("req-456", ws.ErrCodeServerError, "Channel full", nil)
}

// TestConnectionSendMessage tests message sending
func TestConnectionSendMessage(t *testing.T) {
	conn := NewConnection("test-conn", nil, nil)

	msg := &ws.Message{
		ID:   "msg-1",
		Type: ws.MessageTypeResponse,
		Result: map[string]interface{}{
			"data": "test",
		},
	}

	err := conn.SendMessage(msg)
	assert.NoError(t, err)

	// Verify message was queued
	select {
	case data := <-conn.send:
		var decoded ws.Message
		err := json.Unmarshal(data, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, msg.ID, decoded.ID)
	default:
		t.Fatal("No message in send channel")
	}

	// Test with full channel - fill it completely
	for i := 0; i < 256; i++ {
		conn.send <- []byte("blocking")
	}
	err = conn.SendMessage(msg)
	assert.Error(t, err)
}

// TestConnectionSendNotification tests notification sending
func TestConnectionSendNotification(t *testing.T) {
	conn := NewConnection("test-conn", nil, nil)

	err := conn.SendNotification("event.occurred", map[string]interface{}{
		"event": "test",
		"data":  "value",
	})
	assert.NoError(t, err)

	// Verify notification was queued
	select {
	case data := <-conn.send:
		var msg ws.Message
		err := json.Unmarshal(data, &msg)
		assert.NoError(t, err)
		assert.NotEmpty(t, msg.ID)
		assert.Equal(t, ws.MessageTypeNotification, msg.Type)
		assert.Equal(t, "event.occurred", msg.Method)
		assert.NotNil(t, msg.Params)
	default:
		t.Fatal("No notification in send channel")
	}
}

// TestRateLimiterConcurrency tests rate limiter under concurrent access
func TestRateLimiterConcurrency(t *testing.T) {
	limiter := NewRateLimiter(100.0, 10.0) // 100 per second, burst of 10

	var allowed int64
	var wg sync.WaitGroup

	// Run 20 concurrent goroutines
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if limiter.Allow() {
				atomic.AddInt64(&allowed, 1)
			}
		}()
	}

	wg.Wait()

	// Should allow burst of 10, but due to race conditions might allow slightly more
	finalAllowed := atomic.LoadInt64(&allowed)
	assert.LessOrEqual(t, finalAllowed, int64(12))
	assert.GreaterOrEqual(t, finalAllowed, int64(8))
}

// TestConnectionStateManagement tests connection state transitions
func TestConnectionStateManagement(t *testing.T) {
	conn := NewConnection("test-conn", nil, nil)
	conn.SetState(ws.ConnectionStateConnecting)

	// Test state transitions
	assert.Equal(t, ws.ConnectionState(ws.ConnectionStateConnecting), conn.GetState())

	conn.SetState(ws.ConnectionStateConnected)
	assert.Equal(t, ws.ConnectionState(ws.ConnectionStateConnected), conn.GetState())
	assert.True(t, conn.IsActive())

	conn.SetState(ws.ConnectionStateClosing)
	assert.Equal(t, ws.ConnectionState(ws.ConnectionStateClosing), conn.GetState())
	assert.False(t, conn.IsActive())

	conn.SetState(ws.ConnectionStateClosed)
	assert.Equal(t, ws.ConnectionState(ws.ConnectionStateClosed), conn.GetState())
	assert.False(t, conn.IsActive())
}

// TestConnectionClose tests connection closing
func TestConnectionClose(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)

		// Create a minimal mock hub to avoid nil pointer in removeConnection
		mockHub := &Server{
			connections: make(map[string]*Connection),
			mu:          sync.RWMutex{},
		}

		testConn := NewConnection("test-conn", conn, mockHub)
		testConn.SetState(ws.ConnectionStateConnected)

		// Close connection
		err = testConn.Close()
		assert.NoError(t, err)
		// State could be either Closing or Closed depending on timing
		state := testConn.GetState()
		assert.True(t, state == ws.ConnectionStateClosing || state == ws.ConnectionStateClosed,
			"Expected state to be Closing or Closed, got %v", state)
	}))
	defer server.Close()

	// Connect as client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)

	// Wait for server to close connection
	time.Sleep(100 * time.Millisecond)

	// Try to read, should get close error
	var msg ws.Message
	err = wsjson.Read(ctx, conn, &msg)
	assert.Error(t, err)
}
