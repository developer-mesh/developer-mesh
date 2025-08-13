package websocket

import (
	"bytes"
	"sync"
	"time"

	"github.com/coder/websocket"
	ws "github.com/developer-mesh/developer-mesh/pkg/models/websocket"
)

// Object pools for zero-allocation design
var (
	// Message pool for WebSocket messages
	messagePool = sync.Pool{
		New: func() interface{} {
			return &ws.Message{}
		},
	}

	// Buffer pool for binary encoding
	bufferPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, 4096))
		},
	}

	// Connection pool for reusing connection objects
	connectionPool = sync.Pool{
		New: func() interface{} {
			// Create a connection with initialized embedded ws.Connection
			wsConn := &ws.Connection{
				ID:        "", // Will be set when connection is used
				AgentID:   "",
				TenantID:  "",
				CreatedAt: time.Time{},
				LastPing:  time.Time{},
			}
			// Initialize state to prevent nil pointer
			wsConn.State.Store(ws.ConnectionStateClosed)

			return &Connection{
				Connection: wsConn,
				send:       make(chan []byte, 256),
				afterSend:  make(chan *PostActionConfig, 32), // Buffered to prevent blocking
				closed:     make(chan struct{}),
			}
		},
	}

	// Binary header pool
	binaryHeaderPool = sync.Pool{
		New: func() interface{} {
			return &ws.BinaryHeader{}
		},
	}

	// Byte slice pools for different sizes
	smallBytePool = sync.Pool{
		New: func() interface{} {
			b := make([]byte, 256)
			return &b
		},
	}

	mediumBytePool = sync.Pool{
		New: func() interface{} {
			b := make([]byte, 4096)
			return &b
		},
	}

	largeBytePool = sync.Pool{
		New: func() interface{} {
			b := make([]byte, 65536)
			return &b
		},
	}
)

// GetMessage retrieves a message from the pool
func GetMessage() *ws.Message {
	return messagePool.Get().(*ws.Message)
}

// PutMessage returns a message to the pool
func PutMessage(msg *ws.Message) {
	// Reset the message
	msg.ID = ""
	msg.Type = 0
	msg.Method = ""
	msg.Params = nil
	msg.Result = nil
	msg.Error = nil

	messagePool.Put(msg)
}

// GetBuffer retrieves a buffer from the pool
func GetBuffer() *bytes.Buffer {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// PutBuffer returns a buffer to the pool
func PutBuffer(buf *bytes.Buffer) {
	// Only put back reasonable sized buffers
	if buf.Cap() > 1024*1024 { // 1MB
		return
	}
	bufferPool.Put(buf)
}

// GetBinaryHeader retrieves a binary header from the pool
func GetBinaryHeader() *ws.BinaryHeader {
	return binaryHeaderPool.Get().(*ws.BinaryHeader)
}

// PutBinaryHeader returns a binary header to the pool
func PutBinaryHeader(header *ws.BinaryHeader) {
	// Reset the header
	*header = ws.BinaryHeader{}
	binaryHeaderPool.Put(header)
}

// GetByteSlice retrieves a byte slice from the appropriate pool
func GetByteSlice(size int) *[]byte {
	switch {
	case size <= 256:
		return smallBytePool.Get().(*[]byte)
	case size <= 4096:
		return mediumBytePool.Get().(*[]byte)
	case size <= 65536:
		return largeBytePool.Get().(*[]byte)
	default:
		// For very large sizes, allocate directly
		b := make([]byte, size)
		return &b
	}
}

// PutByteSlice returns a byte slice to the appropriate pool
func PutByteSlice(b *[]byte) {
	if b == nil {
		return
	}

	size := cap(*b)
	switch {
	case size <= 256:
		smallBytePool.Put(b)
	case size <= 4096:
		mediumBytePool.Put(b)
	case size <= 65536:
		largeBytePool.Put(b)
		// Don't pool very large slices
	}
}

// GetConnection retrieves a connection from the sync.Pool
func GetConnection() *Connection {
	// Always create a new connection for WebSocket
	// WebSocket connections are stateful and should not be reused
	wsConn := &ws.Connection{
		ID:        "",
		AgentID:   "",
		TenantID:  "",
		CreatedAt: time.Time{},
		LastPing:  time.Time{},
	}
	wsConn.State.Store(ws.ConnectionStateClosed)

	return &Connection{
		Connection: wsConn,
		conn:       nil,
		hub:        nil,
		state:      nil,
		send:       make(chan []byte, 256),
		afterSend:  make(chan *PostActionConfig, 32), // Buffered to prevent blocking
		closed:     make(chan struct{}),
		mu:         sync.RWMutex{},
		closeOnce:  sync.Once{},
		wg:         sync.WaitGroup{},
	}
}

// PutConnection returns a connection to the sync.Pool
func PutConnection(conn *Connection) {
	if conn == nil {
		return
	}

	// Close the websocket connection if it exists
	if conn.conn != nil {
		_ = conn.conn.Close(websocket.StatusNormalClosure, "")
	}

	// Close channels if they're open
	if conn.closed != nil {
		select {
		case <-conn.closed:
			// Already closed
		default:
			close(conn.closed)
		}
	}

	// Close send channel
	if conn.send != nil {
		close(conn.send)
		// Drain any remaining messages
		for range conn.send {
		}
	}

	// Don't return to pool - WebSocket connections should not be reused
	// The connection will be garbage collected
}

// ConnectionPoolManager manages a pool of pre-allocated connections
type ConnectionPoolManager struct {
	pool        chan *Connection
	size        int
	maxSize     int
	minSize     int
	idleTimeout time.Duration
	mu          sync.Mutex
	done        chan struct{}
	stopOnce    sync.Once

	// Metrics
	created   uint64
	destroyed uint64
	borrowed  uint64
	returned  uint64

	// Track idle connections
	idleTracker map[*Connection]time.Time
	trackerMu   sync.RWMutex
}

// NewConnectionPoolManager creates a new connection pool manager
func NewConnectionPoolManager(size int) *ConnectionPoolManager {
	minSize := size / 4
	if minSize < 10 {
		minSize = 10
	}

	maxSize := size * 2
	if maxSize > 10000 {
		maxSize = 10000
	}

	manager := &ConnectionPoolManager{
		pool:        make(chan *Connection, size),
		size:        size,
		minSize:     minSize,
		maxSize:     maxSize,
		idleTimeout: 5 * time.Minute,
		idleTracker: make(map[*Connection]time.Time),
		done:        make(chan struct{}),
	}

	// Don't pre-allocate connections since WebSocket connections should not be reused

	// Start pool maintenance goroutine
	go manager.maintain()

	return manager
}

// Get retrieves a connection from the pool
func (m *ConnectionPoolManager) Get() *Connection {
	m.mu.Lock()
	m.borrowed++
	m.created++
	m.mu.Unlock()

	// ALWAYS create a new connection for WebSocket
	// WebSocket connections are stateful and should never be reused
	wsConn := &ws.Connection{
		ID:        "",
		AgentID:   "",
		TenantID:  "",
		CreatedAt: time.Time{},
		LastPing:  time.Time{},
	}
	wsConn.State.Store(ws.ConnectionStateClosed)

	return &Connection{
		Connection: wsConn,
		send:       make(chan []byte, 256),
		afterSend:  make(chan *PostActionConfig, 32), // Buffered to prevent blocking
		closed:     make(chan struct{}),
	}
}

// Put returns a connection to the pool
func (m *ConnectionPoolManager) Put(conn *Connection) {
	m.mu.Lock()
	m.returned++
	m.destroyed++
	m.mu.Unlock()

	// Close the underlying websocket connection if it exists
	if conn.conn != nil {
		// Ignore error as connection might already be closed
		_ = conn.conn.Close(websocket.StatusNormalClosure, "")
	}

	// Always destroy WebSocket connections - they should never be reused
	// Close channels to free resources
	if conn.send != nil {
		close(conn.send)
	}
	if conn.closed != nil {
		select {
		case <-conn.closed:
			// Already closed
		default:
			close(conn.closed)
		}
	}

	// Clear all references
	conn.conn = nil
	conn.hub = nil
	conn.state = nil
	conn.Connection = nil
}

// Stats returns pool statistics
func (m *ConnectionPoolManager) Stats() (available, size int) {
	return len(m.pool), m.size
}

// DetailedStats returns detailed pool statistics
func (m *ConnectionPoolManager) DetailedStats() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	available := len(m.pool)

	// Calculate in-use count safely
	var inUse int64
	if m.borrowed >= m.returned {
		diff := m.borrowed - m.returned
		// Check if diff fits in int64
		const maxInt64 = 9223372036854775807 // math.MaxInt64
		if diff <= maxInt64 {
			inUse = int64(diff)
		} else {
			inUse = maxInt64
		}
	}

	return map[string]interface{}{
		"size":         m.size,
		"min_size":     m.minSize,
		"max_size":     m.maxSize,
		"available":    available,
		"in_use":       inUse,
		"created":      m.created,
		"destroyed":    m.destroyed,
		"borrowed":     m.borrowed,
		"returned":     m.returned,
		"idle_timeout": m.idleTimeout.String(),
		"utilization":  float64(inUse) / float64(m.size),
	}
}

// cleanupIdleConnections removes connections that have been idle too long
func (m *ConnectionPoolManager) cleanupIdleConnections() {
	m.trackerMu.Lock()
	defer m.trackerMu.Unlock()

	now := time.Now()
	toRemove := make([]*Connection, 0)

	// Find connections that have been idle too long
	for conn, idleTime := range m.idleTracker {
		if now.Sub(idleTime) > m.idleTimeout {
			toRemove = append(toRemove, conn)
		}
	}

	// Remove idle connections from the pool
	for _, conn := range toRemove {
		// Try to remove from pool by draining it temporarily
		removed := false
		poolSize := len(m.pool)
		tempConns := make([]*Connection, 0, poolSize)

		// Drain the pool
	drainLoop:
		for i := 0; i < poolSize; i++ {
			select {
			case c := <-m.pool:
				if c == conn {
					removed = true
					// Destroy this connection
					if c.send != nil {
						close(c.send)
					}
					m.mu.Lock()
					m.destroyed++
					m.mu.Unlock()
				} else {
					tempConns = append(tempConns, c)
				}
			default:
				break drainLoop
			}
		}

		// Put non-idle connections back
		for _, c := range tempConns {
			select {
			case m.pool <- c:
			default:
				// This shouldn't happen but handle gracefully
				if c.send != nil {
					close(c.send)
				}
			}
		}

		// Remove from tracker if we found and removed it
		if removed {
			delete(m.idleTracker, conn)
		}
	}
}

// maintain performs periodic pool maintenance
func (m *ConnectionPoolManager) maintain() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			// Don't maintain a minimum pool size - WebSocket connections should not be reused
			// Just clean up idle connections
			m.cleanupIdleConnections()
		}
	}
}

// Stop gracefully stops the pool maintenance
func (m *ConnectionPoolManager) Stop() {
	m.stopOnce.Do(func() {
		close(m.done)
		m.Shutdown()
	})
}

// Shutdown gracefully shuts down the pool
func (m *ConnectionPoolManager) Shutdown() {
	// Close all connections in the pool
	close(m.pool)

	for conn := range m.pool {
		if conn.conn != nil {
			_ = conn.conn.Close(websocket.StatusGoingAway, "server shutdown")
		}
		if conn.send != nil {
			close(conn.send)
		}
	}
}

// Memory pool for reducing GC pressure
type MemoryPool struct {
	allocations uint64
	frees       uint64
	inUse       uint64
	mu          sync.Mutex
}

var globalMemoryPool = &MemoryPool{}

// TrackAllocation tracks memory allocation
func (mp *MemoryPool) TrackAllocation() {
	mp.mu.Lock()
	mp.allocations++
	mp.inUse++
	mp.mu.Unlock()
}

// TrackFree tracks memory free
func (mp *MemoryPool) TrackFree() {
	mp.mu.Lock()
	mp.frees++
	mp.inUse--
	mp.mu.Unlock()
}

// Stats returns memory pool statistics
func (mp *MemoryPool) Stats() (allocations, frees, inUse uint64) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	return mp.allocations, mp.frees, mp.inUse
}
