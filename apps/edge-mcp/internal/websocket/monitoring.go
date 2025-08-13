package websocket

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	// "github.com/coder/websocket" // Reserved for dashboard WebSocket connections
)

// MonitoringEndpoints provides HTTP endpoints for WebSocket monitoring
type MonitoringEndpoints struct {
	server *Server
}

// NewMonitoringEndpoints creates new monitoring endpoints
func NewMonitoringEndpoints(server *Server) *MonitoringEndpoints {
	return &MonitoringEndpoints{
		server: server,
	}
}

// RegisterRoutes registers monitoring routes with gin
func (m *MonitoringEndpoints) RegisterRoutes(router *gin.RouterGroup) {
	monitor := router.Group("/websocket")
	{
		monitor.GET("/stats", m.handleStats)
		monitor.GET("/connections", m.handleConnections)
		monitor.GET("/health", m.handleHealth)
		monitor.GET("/metrics", m.handleMetrics)
	}
}

// handleStats returns WebSocket statistics
func (m *MonitoringEndpoints) handleStats(c *gin.Context) {
	stats := m.server.metricsCollector.GetStats()

	// Add server-level stats
	uptime := time.Since(m.server.startTime)
	response := gin.H{
		"server": gin.H{
			"active_connections": m.server.ConnectionCount(),
			"max_connections":    m.server.config.MaxConnections,
			"uptime":             uptime.String(),
			"uptime_seconds":     uptime.Seconds(),
			"started_at":         m.server.startTime.Format(time.RFC3339),
			"version":            m.server.config.Version,
			"build_time":         m.server.config.BuildTime,
			"git_commit":         m.server.config.GitCommit,
		},
		"connections": gin.H{
			"total":        stats.TotalConnections,
			"active":       stats.ActiveConnections,
			"failed":       stats.FailedConnections,
			"avg_duration": stats.AvgConnectionDuration.String(),
		},
		"messages": gin.H{
			"received":       stats.MessagesReceived,
			"sent":           stats.MessagesSent,
			"dropped":        stats.MessagesDropped,
			"batches":        stats.BatchesSent,
			"avg_latency_ms": stats.AvgMessageLatency * 1000,
		},
		"protocols": gin.H{
			"binary":     stats.BinaryMessages,
			"json":       stats.JSONMessages,
			"compressed": stats.CompressedMessages,
		},
		"errors": gin.H{
			"auth":       stats.AuthErrors,
			"rate_limit": stats.RateLimitErrors,
			"protocol":   stats.ProtocolErrors,
		},
		"tenants": stats.TenantStats,
	}

	c.JSON(http.StatusOK, response)
}

// handleConnections returns active connection details
func (m *MonitoringEndpoints) handleConnections(c *gin.Context) {
	m.server.mu.RLock()
	defer m.server.mu.RUnlock()

	connections := make([]gin.H, 0, len(m.server.connections))
	for _, conn := range m.server.connections {
		connections = append(connections, gin.H{
			"id":         conn.ID,
			"agent_id":   conn.AgentID,
			"tenant_id":  conn.TenantID,
			"state":      conn.GetState(),
			"created_at": conn.CreatedAt,
			"last_ping":  conn.LastPing,
			"duration":   time.Since(conn.CreatedAt).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"count":       len(connections),
		"connections": connections,
	})
}

// handleHealth returns WebSocket server health
func (m *MonitoringEndpoints) handleHealth(c *gin.Context) {
	health := gin.H{
		"status": "healthy",
		"version": gin.H{
			"version":    m.server.config.Version,
			"build_time": m.server.config.BuildTime,
			"git_commit": m.server.config.GitCommit,
		},
		"uptime": time.Since(m.server.startTime).String(),
		"checks": gin.H{
			"connections":     m.checkConnectionHealth(),
			"rate_limiter":    m.checkRateLimiterHealth(),
			"memory_pool":     m.checkMemoryPoolHealth(),
			"batch_processor": m.checkBatchProcessorHealth(),
		},
	}

	// Determine overall health
	healthy := true
	for _, check := range health["checks"].(gin.H) {
		if checkMap, ok := check.(gin.H); ok {
			if status, ok := checkMap["status"].(string); ok && status != "healthy" {
				healthy = false
				break
			}
		}
	}

	if !healthy {
		health["status"] = "degraded"
		c.JSON(http.StatusServiceUnavailable, health)
	} else {
		c.JSON(http.StatusOK, health)
	}
}

// handleMetrics returns Prometheus-compatible metrics
func (m *MonitoringEndpoints) handleMetrics(c *gin.Context) {
	stats := m.server.metricsCollector.GetStats()

	// Format as Prometheus metrics
	metrics := []string{
		"# HELP websocket_connections_total Total number of WebSocket connections",
		"# TYPE websocket_connections_total counter",
		formatMetric("websocket_connections_total", float64(stats.TotalConnections)),

		"# HELP websocket_connections_active Current number of active connections",
		"# TYPE websocket_connections_active gauge",
		formatMetric("websocket_connections_active", float64(stats.ActiveConnections)),

		"# HELP websocket_messages_total Total number of messages",
		"# TYPE websocket_messages_total counter",
		formatMetric("websocket_messages_total{direction=\"received\"}", float64(stats.MessagesReceived)),
		formatMetric("websocket_messages_total{direction=\"sent\"}", float64(stats.MessagesSent)),

		"# HELP websocket_message_latency_seconds Message processing latency",
		"# TYPE websocket_message_latency_seconds histogram",
		formatMetric("websocket_message_latency_seconds", stats.AvgMessageLatency),

		"# HELP websocket_errors_total Total number of errors",
		"# TYPE websocket_errors_total counter",
		formatMetric("websocket_errors_total{type=\"auth\"}", float64(stats.AuthErrors)),
		formatMetric("websocket_errors_total{type=\"rate_limit\"}", float64(stats.RateLimitErrors)),
		formatMetric("websocket_errors_total{type=\"protocol\"}", float64(stats.ProtocolErrors)),

		"# HELP websocket_server_uptime_seconds Server uptime in seconds",
		"# TYPE websocket_server_uptime_seconds gauge",
		formatMetric("websocket_server_uptime_seconds", time.Since(m.server.startTime).Seconds()),
	}

	// Add pool stats
	if m.server.connectionPool != nil {
		available, size := m.server.connectionPool.Stats()
		metrics = append(metrics,
			"# HELP websocket_pool_available Available connections in pool",
			"# TYPE websocket_pool_available gauge",
			formatMetric("websocket_pool_available", float64(available)),
			formatMetric("websocket_pool_size", float64(size)),
		)
	}

	c.String(http.StatusOK, joinMetrics(metrics))
}

// checkConnectionHealth checks connection subsystem health
func (m *MonitoringEndpoints) checkConnectionHealth() gin.H {
	active := m.server.ConnectionCount()
	max := m.server.config.MaxConnections
	utilization := float64(active) / float64(max)

	status := "healthy"
	if utilization > 0.9 {
		status = "warning"
	} else if active >= max {
		status = "critical"
	}

	return gin.H{
		"status":      status,
		"active":      active,
		"max":         max,
		"utilization": utilization,
	}
}

// checkRateLimiterHealth checks rate limiter health
func (m *MonitoringEndpoints) checkRateLimiterHealth() gin.H {
	stats := m.server.metricsCollector.GetStats()
	errorRate := float64(0)

	total := stats.MessagesReceived
	if total > 0 {
		errorRate = float64(stats.RateLimitErrors) / float64(total)
	}

	status := "healthy"
	if errorRate > 0.1 {
		status = "warning"
	} else if errorRate > 0.2 {
		status = "critical"
	}

	return gin.H{
		"status":     status,
		"error_rate": errorRate,
		"errors":     stats.RateLimitErrors,
	}
}

// checkMemoryPoolHealth checks memory pool health
func (m *MonitoringEndpoints) checkMemoryPoolHealth() gin.H {
	allocations, frees, inUse := globalMemoryPool.Stats()

	status := "healthy"
	if inUse > 10000 {
		status = "warning"
	} else if inUse > 50000 {
		status = "critical"
	}

	return gin.H{
		"status":      status,
		"allocations": allocations,
		"frees":       frees,
		"in_use":      inUse,
	}
}

// checkBatchProcessorHealth checks batch processor health
func (m *MonitoringEndpoints) checkBatchProcessorHealth() gin.H {
	// This would check batch processor stats
	// For now, return healthy
	return gin.H{
		"status": "healthy",
	}
}

// formatMetric formats a metric for Prometheus
func formatMetric(name string, value float64) string {
	return name + " " + formatFloat(value)
}

// formatFloat formats a float for Prometheus
func formatFloat(v float64) string {
	// Simple float formatting
	return fmt.Sprintf("%g", v)
}

// joinMetrics joins metrics with newlines
func joinMetrics(metrics []string) string {
	result := ""
	for _, m := range metrics {
		result += m + "\n"
	}
	return result
}

// WebSocketDashboard provides a real-time dashboard
type WebSocketDashboard struct {
	server *Server
	hub    *DashboardHub
}

// DashboardHub manages dashboard WebSocket connections
type DashboardHub struct {
	connections map[string]*DashboardConnection
	broadcast   chan []byte
	register    chan *DashboardConnection
	unregister  chan *DashboardConnection
	mu          sync.RWMutex
}

// DashboardConnection represents a dashboard WebSocket connection
type DashboardConnection struct {
	id string
	// conn field reserved for WebSocket connection
	// conn   *websocket.Conn
	send chan []byte
	// hub field reserved for dashboard hub reference
	// hub    *DashboardHub
}

// NewWebSocketDashboard creates a new dashboard
func NewWebSocketDashboard(server *Server) *WebSocketDashboard {
	hub := &DashboardHub{
		connections: make(map[string]*DashboardConnection),
		broadcast:   make(chan []byte),
		register:    make(chan *DashboardConnection),
		unregister:  make(chan *DashboardConnection),
	}

	go hub.run()

	return &WebSocketDashboard{
		server: server,
		hub:    hub,
	}
}

// run manages the dashboard hub
func (h *DashboardHub) run() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			h.connections[conn.id] = conn
			h.mu.Unlock()

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.connections[conn.id]; ok {
				delete(h.connections, conn.id)
				close(conn.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for _, conn := range h.connections {
				select {
				case conn.send <- message:
				default:
					// Skip slow connections
				}
			}
			h.mu.RUnlock()

		case <-ticker.C:
			// Broadcast stats periodically
			// This would send real-time stats to dashboard
		}
	}
}

// Server uptime tracking is already implemented:
// - Server struct has startTime field initialized in NewServer()
// - Uptime is calculated as time.Since(m.server.startTime) in handleStats()
