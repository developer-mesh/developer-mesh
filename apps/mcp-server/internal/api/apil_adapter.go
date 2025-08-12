package api

import (
	"context"

	gorillaws "github.com/coder/websocket"
	ws "github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/api/websocket"
	"github.com/developer-mesh/developer-mesh/pkg/protocol/adaptive"
)

// WebSocketServerAdapter adapts the MCP WebSocket server to the APIL WebSocketServer interface
type WebSocketServerAdapter struct {
	server *ws.Server
}

// NewWebSocketServerAdapter creates a new adapter
func NewWebSocketServerAdapter(server *ws.Server) adaptive.WebSocketServer {
	return &WebSocketServerAdapter{
		server: server,
	}
}

// RegisterMiddleware registers a middleware function
func (a *WebSocketServerAdapter) RegisterMiddleware(middleware adaptive.Middleware) {
	// Convert APIL middleware to WebSocket server middleware
	wsMiddleware := func(conn *gorillaws.Conn, msg []byte) ([]byte, error) {
		// Create a simple next function that returns the original message
		next := func(ctx context.Context, data []byte) error {
			// In the actual implementation, this would process the message
			return nil
		}

		// Call the APIL middleware
		err := middleware(context.Background(), msg, next)
		if err != nil {
			return nil, err
		}

		return msg, nil
	}

	// Note: The actual WebSocket server would need to support middleware registration
	// For now, we'll store it internally or use a different approach
	_ = wsMiddleware
}

// GetConnections returns active WebSocket connections
func (a *WebSocketServerAdapter) GetConnections() map[string]*gorillaws.Conn {
	// The WebSocket server would need to expose this method
	// For now, return an empty map
	return make(map[string]*gorillaws.Conn)
}

// MCPHandlerAdapter adapts MCPProtocolHandler to the APIL MCPHandler interface
type MCPHandlerAdapter struct {
	handler *MCPProtocolHandler
}

// NewMCPHandlerAdapter creates a new adapter
func NewMCPHandlerAdapter(handler *MCPProtocolHandler) adaptive.MCPHandler {
	return &MCPHandlerAdapter{
		handler: handler,
	}
}

// HandleMessage processes an MCP message
func (a *MCPHandlerAdapter) HandleMessage(conn *gorillaws.Conn, connID string, tenantID string, message []byte) error {
	return a.handler.HandleMessage(conn, connID, tenantID, message)
}

// RemoveSession removes a session when connection closes
func (a *MCPHandlerAdapter) RemoveSession(connID string) {
	a.handler.RemoveSession(connID)
}
