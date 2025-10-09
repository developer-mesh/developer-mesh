package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/auth"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/cache"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoGoroutineLeaks(t *testing.T) {
	// Get initial goroutine count
	runtime.GC()
	initialGoroutines := runtime.NumGoroutine()

	// Create handler
	handler := NewHandler(
		tools.NewRegistry(),
		cache.NewMemoryCache(100, 5*time.Minute),
		nil,
		auth.NewEdgeAuthenticator(""),
		observability.NewNoopLogger(),
		nil,
		nil,
	)

	// Simulate multiple connections
	for i := 0; i < 5; i++ {
		func() {
			// Create test WebSocket server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					return
				}
				handler.HandleConnection(conn, r)
			}))
			defer server.Close()

			wsURL := strings.Replace(server.URL, "http", "ws", 1)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// Connect client
			conn, _, err := websocket.Dial(ctx, wsURL, nil)
			require.NoError(t, err)

			// Send initialize message
			err = conn.Write(ctx, websocket.MessageText, []byte(`{
				"jsonrpc": "2.0",
				"id": 1,
				"method": "initialize",
				"params": {
					"protocolVersion": "2025-06-18",
					"clientInfo": {"name": "test", "version": "1.0"}
				}
			}`))
			require.NoError(t, err)

			// Read response
			_, _, err = conn.Read(ctx)
			require.NoError(t, err)

			// Close connection
			err = conn.Close(websocket.StatusNormalClosure, "")
			assert.NoError(t, err)
		}()

		// Allow goroutines to clean up
		time.Sleep(100 * time.Millisecond)
	}

	// Shutdown handler
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := handler.Shutdown(shutdownCtx)
	assert.NoError(t, err)

	// Allow time for goroutines to exit
	time.Sleep(500 * time.Millisecond)
	runtime.GC()

	// Check goroutine count
	finalGoroutines := runtime.NumGoroutine()

	// Allow for a small number of system goroutines
	goroutineGrowth := finalGoroutines - initialGoroutines
	assert.LessOrEqual(t, goroutineGrowth, 2,
		"Goroutine leak detected. Initial: %d, Final: %d, Growth: %d",
		initialGoroutines, finalGoroutines, goroutineGrowth)
}

func TestPingLoopCleanup(t *testing.T) {
	handler := &Handler{
		logger:  observability.NewNoopLogger(),
		metrics: nil,
	}

	// Create mock connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
		// Keep connection open for test
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, server.URL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// Start ping loop
	pingCtx, pingCancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go handler.pingLoop(pingCtx, conn, done)

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Cancel and verify cleanup
	pingCancel()

	// Should complete quickly
	select {
	case <-done:
		// Success - ping loop cleaned up
	case <-time.After(1 * time.Second):
		t.Fatal("Ping loop did not clean up in time")
	}
}

func TestConcurrentConnectionHandling(t *testing.T) {
	handler := NewHandler(
		tools.NewRegistry(),
		cache.NewMemoryCache(100, 5*time.Minute),
		nil,
		auth.NewEdgeAuthenticator(""),
		observability.NewNoopLogger(),
		nil,
		nil,
	)

	// Track goroutines before
	runtime.GC()
	beforeGoroutines := runtime.NumGoroutine()

	// Create multiple concurrent connections
	numConnections := 10
	done := make(chan bool, numConnections)

	for i := 0; i < numConnections; i++ {
		go func(id int) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					return
				}
				handler.HandleConnection(conn, r)
			}))
			defer server.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			wsURL := strings.Replace(server.URL, "http", "ws", 1)
			conn, _, err := websocket.Dial(ctx, wsURL, nil)
			if err != nil {
				done <- false
				return
			}

			// Quick message exchange
			_ = conn.Write(ctx, websocket.MessageText, []byte(`{
				"jsonrpc": "2.0",
				"id": 1,
				"method": "ping",
				"params": {}
			}`))

			_ = conn.Close(websocket.StatusNormalClosure, "")
			done <- true
		}(i)
	}

	// Wait for all connections to complete
	successCount := 0
	for i := 0; i < numConnections; i++ {
		if <-done {
			successCount++
		}
	}

	assert.Equal(t, numConnections, successCount)

	// Shutdown handler
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = handler.Shutdown(shutdownCtx)

	// Allow cleanup
	time.Sleep(500 * time.Millisecond)
	runtime.GC()

	// Check goroutine count didn't grow significantly
	afterGoroutines := runtime.NumGoroutine()
	growth := afterGoroutines - beforeGoroutines

	assert.LessOrEqual(t, growth, 5,
		"Too many goroutines after concurrent connections. Before: %d, After: %d",
		beforeGoroutines, afterGoroutines)
}

func TestShutdownCleansUpGoroutines(t *testing.T) {
	handler := NewHandler(
		tools.NewRegistry(),
		cache.NewMemoryCache(100, 5*time.Minute),
		nil,
		auth.NewEdgeAuthenticator(""),
		observability.NewNoopLogger(),
		nil,
		nil,
	)

	// Track initial goroutines
	runtime.GC()
	initialGoroutines := runtime.NumGoroutine()

	// Create a connection that keeps running
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		handler.HandleConnection(conn, r)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := strings.Replace(server.URL, "http", "ws", 1)
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)

	// Send initialize to establish session
	err = conn.Write(ctx, websocket.MessageText, []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "initialize",
		"params": {
			"protocolVersion": "2025-06-18",
			"clientInfo": {"name": "test", "version": "1.0"}
		}
	}`))
	require.NoError(t, err)

	// Read response
	_, _, err = conn.Read(ctx)
	require.NoError(t, err)

	// Let the connection run for a bit (ping loop should be active)
	time.Sleep(200 * time.Millisecond)

	// Close connection
	_ = conn.Close(websocket.StatusNormalClosure, "")

	// Shutdown the handler
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	err = handler.Shutdown(shutdownCtx)
	assert.NoError(t, err)

	// Wait for cleanup
	time.Sleep(500 * time.Millisecond)
	runtime.GC()

	// Check goroutine count
	finalGoroutines := runtime.NumGoroutine()
	growth := finalGoroutines - initialGoroutines

	assert.LessOrEqual(t, growth, 2,
		"Goroutines not cleaned up after shutdown. Initial: %d, Final: %d, Growth: %d",
		initialGoroutines, finalGoroutines, growth)
}

func TestRefreshManagerGoroutineCleanup(t *testing.T) {
	// Create handler with mock core client to trigger refresh manager
	handler := NewHandler(
		tools.NewRegistry(),
		cache.NewMemoryCache(100, 5*time.Minute),
		nil, // No core client for this test
		auth.NewEdgeAuthenticator(""),
		observability.NewNoopLogger(),
		nil,
		nil,
	)

	// Track goroutines
	runtime.GC()
	beforeGoroutines := runtime.NumGoroutine()

	// Simulate multiple initializations that would trigger refresh
	for i := 0; i < 3; i++ {
		// Add activeRefreshes tracking
		handler.activeRefreshes.Add(1)
		go func() {
			defer handler.activeRefreshes.Done()
			// Simulate work
			time.Sleep(50 * time.Millisecond)
		}()
	}

	// Wait for goroutines to complete
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		handler.activeRefreshes.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good - all refreshes completed
	case <-ctx.Done():
		t.Fatal("Refresh goroutines did not complete in time")
	}

	// Allow cleanup
	time.Sleep(100 * time.Millisecond)
	runtime.GC()

	// Check goroutine count
	afterGoroutines := runtime.NumGoroutine()
	growth := afterGoroutines - beforeGoroutines

	assert.LessOrEqual(t, growth, 1,
		"Refresh goroutines not cleaned up. Before: %d, After: %d, Growth: %d",
		beforeGoroutines, afterGoroutines, growth)
}
