package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	ws "github.com/developer-mesh/developer-mesh/pkg/models/websocket"
)

// ConnectionMode represents the type of connection
type ConnectionMode int

const (
	ModeStandardMCP ConnectionMode = iota
	ModeClaudeCode
	ModeAgent
	ModeIDE
)

// String returns the string representation of ConnectionMode
func (m ConnectionMode) String() string {
	switch m {
	case ModeClaudeCode:
		return "claude-code"
	case ModeAgent:
		return "agent"
	case ModeIDE:
		return "ide"
	default:
		return "standard-mcp"
	}
}

// ConnectionState tracks additional connection state
type ConnectionState struct {
	BinaryMode           bool
	CompressionThreshold int
	MaxTokens            int
	CurrentTokenUsage    int
	ActiveSessionID      string
	PreviousSessionID    string
	SystemPromptTokens   int
	ConversationTokens   int
	ToolTokens           int
	Claims               *auth.Claims   // Authentication claims
	ConnectionMode       ConnectionMode // Type of connection
}

// RateLimiter implements token bucket algorithm
type RateLimiter struct {
	mu        sync.Mutex
	tokens    float64
	capacity  float64
	rate      float64
	lastCheck time.Time
}

func NewRateLimiter(rate, capacity float64) *RateLimiter {
	return &RateLimiter{
		tokens:    capacity,
		capacity:  capacity,
		rate:      rate,
		lastCheck: time.Now(),
	}
}

func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastCheck).Seconds()
	r.lastCheck = now

	// Add tokens based on elapsed time
	r.tokens += elapsed * r.rate
	if r.tokens > r.capacity {
		r.tokens = r.capacity
	}

	// Check if we have tokens
	if r.tokens >= 1.0 {
		r.tokens -= 1.0
		return true
	}

	return false
}

// readPump pumps messages from the websocket connection to the hub
func (c *Connection) readPump() {
	c.wg.Add(1)
	defer func() {
		c.wg.Done()
		_ = c.Close()
	}()

	ctx := context.Background()

	// Create rate limiter (1000 messages per minute with burst of 100)
	rateLimiter := NewRateLimiter(1000.0/60.0, 100)

	for {
		// Check if connection is still active
		if !c.IsActive() {
			return
		}

		// Read message using pooled object
		msg := GetMessage()
		defer PutMessage(msg)

		// Protect conn access
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn == nil {
			return
		}

		// Read MCP protocol message (only protocol supported)
		msgType, data, readErr := conn.Read(ctx)
		if readErr == nil {
			// Only handle MCP protocol messages (text format)
			if msgType != websocket.MessageText {
				readErr = fmt.Errorf("unsupported message type: expected text, got %v", msgType)
			} else if !strings.Contains(string(data), `"jsonrpc":"2.0"`) && !strings.Contains(string(data), `"jsonrpc": "2.0"`) {
				readErr = fmt.Errorf("invalid protocol: only MCP (JSON-RPC 2.0) messages are supported")
			} else {
				// Handle MCP protocol message
				if c.hub != nil && c.hub.mcpHandler != nil {
					// Use reflection to call HandleMessage on the MCP handler
					// This avoids circular import issues
					if handler, ok := c.hub.mcpHandler.(interface {
						HandleMessage(*websocket.Conn, string, string, []byte) error
					}); ok {
						// Route to MCP handler
						if err := handler.HandleMessage(conn, c.ID, c.TenantID, data); err != nil {
							c.hub.logger.Error("MCP handler error", map[string]interface{}{
								"error":         err.Error(),
								"connection_id": c.ID,
							})
						}
						// MCP messages are handled, continue to next message
						continue
					}
				} else {
					readErr = fmt.Errorf("MCP handler not configured")
				}
			}
		}
		if readErr != nil {
			// Check for normal closure or EOF (expected for short-lived connections)
			if websocket.CloseStatus(readErr) == websocket.StatusNormalClosure {
				return
			}
			// Check for EOF which is expected for request-response pattern connections
			if strings.Contains(readErr.Error(), "EOF") || strings.Contains(readErr.Error(), "failed to read frame header: EOF") {
				// Log as debug instead of error for expected disconnections
				if c.hub != nil && c.hub.logger != nil && c.Connection != nil {
					c.hub.logger.Debug("Connection closed (expected for request-response pattern)", map[string]interface{}{
						"connection_id": c.ID,
					})
				}
				return
			}
			// Log actual errors
			if c.hub != nil && c.hub.logger != nil && c.Connection != nil {
				c.hub.logger.Error("Read error", map[string]interface{}{
					"error":         readErr.Error(),
					"connection_id": c.ID,
				})
			}
			return
		}

		// Update last activity
		c.LastPing = time.Now()

		// Rate limiting is handled within the MCP handler
		if !rateLimiter.Allow() {
			if c.hub != nil && c.hub.metricsCollector != nil {
				c.hub.metricsCollector.RecordError("rate_limit")
			}
			// Send rate limit error as MCP error response
			// The MCP handler will handle this appropriately
			continue
		}
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Connection) writePump() {
	c.wg.Add(1)

	// Set default ping interval if hub is nil or config is missing
	pingInterval := 30 * time.Second
	if c.hub != nil && c.hub.config.PingInterval > 0 {
		pingInterval = c.hub.config.PingInterval
	}
	ticker := time.NewTicker(pingInterval)

	defer func() {
		ticker.Stop()
		c.wg.Done()
		_ = c.Close()
	}()

	ctx := context.Background()

	for {
		select {
		case <-c.closed:
			// Connection is closing
			return
		case message, ok := <-c.send:
			if !ok {
				// The hub closed the channel
				return
			}

			// Set timeout for write operation
			writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			// Protect conn access
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()

			if conn == nil {
				if c.hub != nil && c.hub.logger != nil {
					connID := "unknown"
					if c.Connection != nil {
						connID = c.ID
					}
					c.hub.logger.Error("Connection is nil", map[string]interface{}{
						"connection_id": connID,
					})
				}
				return
			}

			var err error
			if c.IsBinaryMode() {
				// For binary mode, we need to parse the JSON message first
				var msg ws.Message
				if jsonErr := json.Unmarshal(message, &msg); jsonErr == nil {
					encoder := NewBinaryEncoder(1024)
					if binaryData, encodeErr := encoder.Encode(&msg); encodeErr == nil {
						err = conn.Write(writeCtx, websocket.MessageBinary, binaryData)
					} else {
						// Fall back to text on encoding error
						err = conn.Write(writeCtx, websocket.MessageText, message)
					}
				} else {
					// Fall back to text on parse error
					err = conn.Write(writeCtx, websocket.MessageText, message)
				}
			} else {
				// Send as text
				err = conn.Write(writeCtx, websocket.MessageText, message)
			}

			if err != nil {
				if c.hub != nil && c.hub.logger != nil {
					connID := "unknown"
					if c.Connection != nil {
						connID = c.ID
					}
					c.hub.logger.Error("Write error", map[string]interface{}{
						"error":         err.Error(),
						"connection_id": connID,
					})
				}
				return
			}

			// Record sent message
			if c.hub != nil && c.hub.metricsCollector != nil && c.Connection != nil {
				c.hub.metricsCollector.RecordMessage("sent", "response", c.TenantID, 0)
			}

			// Check for any post-send actions (non-blocking)
			select {
			case postAction := <-c.afterSend:
				if postAction != nil && postAction.Action != nil {
					if postAction.Synchronous {
						// Execute synchronously for protocol switching
						if c.hub != nil && c.hub.logger != nil {
							c.hub.logger.Debug("Executing synchronous post-action", map[string]interface{}{
								"connection_id": c.ID,
							})
						}
						// Execute with panic recovery
						func() {
							defer func() {
								if r := recover(); r != nil {
									if c.hub != nil && c.hub.logger != nil {
										c.hub.logger.Error("Synchronous post-action panic", map[string]interface{}{
											"error":         fmt.Sprintf("%v", r),
											"connection_id": c.ID,
										})
									}
								}
							}()
							postAction.Action()
						}()
					} else {
						// Execute asynchronously in a goroutine
						go func() {
							defer func() {
								if r := recover(); r != nil {
									if c.hub != nil && c.hub.logger != nil {
										c.hub.logger.Error("Async post-action panic", map[string]interface{}{
											"error":         fmt.Sprintf("%v", r),
											"connection_id": c.ID,
										})
									}
								}
							}()
							postAction.Action()
						}()
					}
				}
			default:
				// No action to execute
			}

		case <-ticker.C:
			// Send ping to detect disconnected clients
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()

			if conn != nil {
				pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				if err := conn.Ping(pingCtx); err != nil {
					cancel()
					if c.hub != nil && c.hub.logger != nil && c.Connection != nil {
						c.hub.logger.Error("Ping error", map[string]interface{}{
							"error":         err.Error(),
							"connection_id": c.ID,
						})
					}
					return
				}
				cancel()
			}
		}
	}
}

// sendError sends an error message to the client
func (c *Connection) sendError(requestID string, code int, message string, data interface{}) {
	errorMsg := GetMessage()
	defer PutMessage(errorMsg)

	errorMsg.ID = requestID
	errorMsg.Type = ws.MessageTypeError
	errorMsg.Error = &ws.Error{
		Code:    code,
		Message: message,
		Data:    data,
	}

	response, err := json.Marshal(errorMsg)
	if err != nil {
		if c.hub != nil && c.hub.logger != nil {
			c.hub.logger.Error("Failed to marshal error message", map[string]interface{}{
				"error": err.Error(),
			})
		}
		return
	}

	select {
	case c.send <- response:
	default:
		// Channel full, log and drop
		if c.hub != nil && c.hub.logger != nil && c.Connection != nil {
			c.hub.logger.Warn("Failed to send error message - channel full", map[string]interface{}{
				"connection_id": c.ID,
			})
		}
		if c.hub != nil && c.hub.metricsCollector != nil {
			c.hub.metricsCollector.RecordMessageDropped("channel_full")
		}
	}
}

// SendMessage sends a message to the client
func (c *Connection) SendMessage(msg *ws.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case c.send <- data:
		return nil
	default:
		return ErrChannelFull
	}
}

// SendNotification sends a notification to the client
func (c *Connection) SendNotification(method string, params interface{}) error {
	msg := GetMessage()
	defer PutMessage(msg)

	msg.ID = uuid.New().String()
	msg.Type = ws.MessageTypeNotification
	msg.Method = method
	msg.Params = params

	return c.SendMessage(msg)
}

// Close closes the connection gracefully
func (c *Connection) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		// Signal closure to all goroutines
		close(c.closed)

		// Set state to closing (with nil check)
		if c.Connection != nil {
			c.SetState(ws.ConnectionStateClosing)
		}

		// Remove from hub (with nil check)
		if c.hub != nil {
			c.hub.removeConnection(c)
		}

		// Close the websocket connection
		c.mu.Lock()
		if c.conn != nil {
			closeErr = c.conn.Close(websocket.StatusNormalClosure, "Connection closed by server")
			c.conn = nil
		}
		c.mu.Unlock()

		// Wait for goroutines to finish
		c.wg.Wait()

		// Close afterSend channel
		if c.afterSend != nil {
			close(c.afterSend)
		}

		// Set final state (with nil check)
		if c.Connection != nil {
			c.SetState(ws.ConnectionStateClosed)
		}
	})
	return closeErr
}

// GetTenantUUID returns the tenant ID as a UUID
func (c *Connection) GetTenantUUID() uuid.UUID {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tenantUUID, err := uuid.Parse(c.TenantID)
	if err != nil {
		// Return zero UUID if parsing fails
		return uuid.UUID{}
	}
	return tenantUUID
}

// Custom errors
var (
	ErrChannelFull = &ws.Error{
		Code:    ws.ErrCodeServerError,
		Message: "Message channel full",
	}
)

// Extended Connection methods for new features

// SetBinaryMode enables/disables binary protocol for the connection
func (c *Connection) SetBinaryMode(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == nil {
		c.state = &ConnectionState{}
	}
	c.state.BinaryMode = enabled
}

// IsBinaryMode returns whether binary mode is enabled
func (c *Connection) IsBinaryMode() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.state == nil {
		return false
	}
	return c.state.BinaryMode
}

// SetCompressionThreshold sets the message size threshold for compression
func (c *Connection) SetCompressionThreshold(threshold int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == nil {
		c.state = &ConnectionState{}
	}
	c.state.CompressionThreshold = threshold
}

// GetCompressionThreshold returns the compression threshold
func (c *Connection) GetCompressionThreshold() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.state == nil {
		return 0
	}
	return c.state.CompressionThreshold
}

// Token management methods

// SetMaxTokens sets the maximum token window for the connection
func (c *Connection) SetMaxTokens(maxTokens int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == nil {
		c.state = &ConnectionState{}
	}
	c.state.MaxTokens = maxTokens
}

// GetMaxTokens returns the maximum token window
func (c *Connection) GetMaxTokens() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.state == nil {
		return 200000 // Default for Claude 3 Opus
	}
	if c.state.MaxTokens == 0 {
		return 200000
	}
	return c.state.MaxTokens
}

// GetTokenUsage returns current token usage
func (c *Connection) GetTokenUsage() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.state == nil {
		return 0
	}
	return c.state.CurrentTokenUsage
}

// UpdateTokenUsage updates the current token usage
func (c *Connection) UpdateTokenUsage(tokens int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == nil {
		c.state = &ConnectionState{}
	}
	c.state.CurrentTokenUsage = tokens
}

// GetSystemPromptTokens returns system prompt token count
func (c *Connection) GetSystemPromptTokens() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.state == nil {
		return 0
	}
	return c.state.SystemPromptTokens
}

// GetConversationTokens returns conversation token count
func (c *Connection) GetConversationTokens() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.state == nil {
		return 0
	}
	return c.state.ConversationTokens
}

// GetToolTokens returns tool token count
func (c *Connection) GetToolTokens() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.state == nil {
		return 0
	}
	return c.state.ToolTokens
}

// Session management methods

// SetActiveSession sets the active session for the connection
func (c *Connection) SetActiveSession(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == nil {
		c.state = &ConnectionState{}
	}
	c.state.PreviousSessionID = c.state.ActiveSessionID
	c.state.ActiveSessionID = sessionID
}

// GetActiveSession returns the active session ID
func (c *Connection) GetActiveSession() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.state == nil {
		return ""
	}
	return c.state.ActiveSessionID
}

// GetPreviousSession returns the previous session ID
func (c *Connection) GetPreviousSession() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.state == nil {
		return ""
	}
	return c.state.PreviousSessionID
}

// IsActive safely checks if the connection is active
func (c *Connection) IsActive() bool {
	// Check if connection is nil first
	if c == nil || c.Connection == nil {
		return false
	}
	// Use the embedded Connection's IsActive method
	return c.Connection.IsActive()
}

// SetState safely sets the connection state
func (c *Connection) SetState(state ws.ConnectionState) {
	// Check if connection is nil first
	if c == nil || c.Connection == nil {
		return
	}
	// Use the embedded Connection's SetState method
	c.Connection.SetState(state)
}

// GetState safely gets the connection state
func (c *Connection) GetState() ws.ConnectionState {
	// Check if connection is nil first
	if c == nil || c.Connection == nil {
		return ws.ConnectionStateClosed
	}
	// Use the embedded Connection's GetState method
	return c.Connection.GetState()
}
