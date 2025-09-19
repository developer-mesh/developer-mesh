package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/clients"
	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	ws "github.com/developer-mesh/developer-mesh/pkg/models/websocket"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	agentRepository "github.com/developer-mesh/developer-mesh/pkg/repository/agent"
	"github.com/developer-mesh/developer-mesh/pkg/services"
	"go.opentelemetry.io/otel/attribute"
)

type Server struct {
	connections map[string]*Connection
	mu          sync.RWMutex
	handlers    map[string]interface{} // Can be MessageHandler or MessageHandlerWithPostAction

	auth           *auth.Service
	metrics        observability.MetricsClient
	logger         observability.Logger
	tracingHandler *TracingHandler

	config Config

	// Dependencies
	toolRegistry        ToolRegistry
	contextManager      ContextManager
	eventBus            EventBus
	conversationManager *ConversationSessionManager
	subscriptionManager *SubscriptionManager
	workflowEngine      *WorkflowEngine
	agentRegistry       AgentRegistryInterface
	taskManager         *TaskManager
	workspaceManager    *WorkspaceManager
	notificationManager *NotificationManager

	// REST API client for proxying tool requests
	restAPIClient clients.RESTAPIClient

	// Service layer dependencies
	taskService      services.TaskService
	workflowService  services.WorkflowService
	workspaceService services.WorkspaceService
	documentService  services.DocumentService
	conflictService  services.ConflictResolutionService

	// Security components
	sessionManager  *SessionManager
	ipRateLimiter   *IPRateLimiter
	antiReplayCache *AntiReplayCache

	// Performance components
	connectionPool *ConnectionPoolManager
	batchManager   *BatchManager

	// Metrics
	metricsCollector *MetricsCollector

	// Server start time
	startTime time.Time

	// MCP Protocol handler
	mcpHandler interface{} // Will be set to *api.MCPProtocolHandler to avoid circular import
}

type Config struct {
	MaxConnections  int           `mapstructure:"max_connections"`
	ReadBufferSize  int           `mapstructure:"read_buffer_size"`
	WriteBufferSize int           `mapstructure:"write_buffer_size"`
	PingInterval    time.Duration `mapstructure:"ping_interval"`
	PongTimeout     time.Duration `mapstructure:"pong_timeout"`
	MaxMessageSize  int64         `mapstructure:"max_message_size"`

	// Security settings
	Security  SecurityConfig    `mapstructure:"security"`
	RateLimit RateLimiterConfig `mapstructure:"rate_limit"`

	// Version information
	Version   string `mapstructure:"-"`
	BuildTime string `mapstructure:"-"`
	GitCommit string `mapstructure:"-"`
}

// Connection wraps the WebSocket connection and adds our metadata
type Connection struct {
	*ws.Connection
	conn      *websocket.Conn
	send      chan []byte
	afterSend chan *PostActionConfig // Channel for post-response actions
	hub       *Server
	mu        sync.RWMutex
	state     *ConnectionState

	// Connection lifecycle management
	closeOnce sync.Once
	closed    chan struct{}
	wg        sync.WaitGroup
}

func NewServer(auth *auth.Service, metrics observability.MetricsClient, logger observability.Logger, config Config) *Server {
	// Create tracer function for tracing handler
	var tracerFunc observability.StartSpanFunc = func(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, observability.Span) {
		// This would use the global tracer or one passed in config
		// For now, using a no-op implementation
		return ctx, &NoOpSpan{}
	}

	// Set default MaxMessageSize if not configured
	if config.MaxMessageSize <= 0 {
		config.MaxMessageSize = 1048576 // 1MB default
		if logger != nil {
			logger.Warn("MaxMessageSize not configured, using default", map[string]interface{}{
				"default_size": config.MaxMessageSize,
				"size_kb":      config.MaxMessageSize / 1024,
			})
		}
	}

	s := &Server{
		connections:    make(map[string]*Connection),
		handlers:       make(map[string]interface{}),
		auth:           auth,
		metrics:        metrics,
		logger:         logger,
		tracingHandler: NewTracingHandler(tracerFunc, metrics, logger),
		config:         config,
		startTime:      time.Now(),
	}

	// Initialize security components
	s.sessionManager = NewSessionManager()

	// Initialize IP rate limiter if enabled
	if config.RateLimit.PerIP {
		s.ipRateLimiter = NewIPRateLimiter(&config.RateLimit)
	}

	// Initialize anti-replay cache
	s.antiReplayCache = NewAntiReplayCache(5 * time.Minute)

	// Initialize connection pool for performance
	s.connectionPool = NewConnectionPoolManager(config.MaxConnections)

	// Initialize batch manager for message batching
	batchConfig := DefaultBatchConfig()
	s.batchManager = NewBatchManager(batchConfig, logger, metrics)

	// Initialize metrics collector
	s.metricsCollector = NewMetricsCollector(metrics)

	// Initialize notification manager first as other managers depend on it
	s.notificationManager = NewNotificationManager(logger, metrics)

	// Initialize new managers (these would typically be injected as dependencies)
	s.subscriptionManager = NewSubscriptionManager(logger, metrics)
	// Initialize workflow engine with nil services for now - will be set later
	s.workflowEngine = NewWorkflowEngine(logger, metrics, nil, nil)
	s.agentRegistry = NewAgentRegistry(logger, metrics)
	s.taskManager = NewTaskManager(logger, metrics)
	s.workspaceManager = NewWorkspaceManager(logger, metrics, s)

	// Connect notification manager with subscription manager
	s.notificationManager.SetSubscriptionManager(s.subscriptionManager)

	// Set notification manager in workflow engine
	s.workflowEngine.SetNotificationManager(s.notificationManager)

	// Initialize conversation manager with a simple in-memory cache
	// In production, this would be injected with a proper cache implementation
	inMemoryCache := NewInMemoryCache()
	s.conversationManager = NewConversationSessionManager(inMemoryCache, logger, metrics)

	// Register handlers
	s.RegisterHandlers()

	return s
}

// extractPassthroughAuth extracts passthrough authentication from request headers
func (s *Server) extractPassthroughAuth(r *http.Request) *models.PassthroughAuthBundle {
	bundle := &models.PassthroughAuthBundle{
		Credentials: make(map[string]*models.PassthroughCredential),
	}

	// Check for X-Passthrough-Auth header (JSON encoded bundle)
	if authHeader := r.Header.Get("X-Passthrough-Auth"); authHeader != "" {
		var tempBundle models.PassthroughAuthBundle
		if err := json.Unmarshal([]byte(authHeader), &tempBundle); err == nil {
			return &tempBundle
		}
	}

	// Check for individual service credentials
	// GitHub Token
	if githubToken := r.Header.Get("X-GitHub-Token"); githubToken != "" {
		bundle.Credentials["github"] = &models.PassthroughCredential{
			Type:  "bearer",
			Token: githubToken,
		}
	}

	// Harness Token
	if harnessToken := r.Header.Get("X-Harness-Token"); harnessToken != "" {
		bundle.Credentials["harness"] = &models.PassthroughCredential{
			Type:  "x-api-key",
			Token: harnessToken,
		}
	}

	// Generic API Key
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		bundle.Credentials["default"] = &models.PassthroughCredential{
			Type:  "api-key",
			Token: apiKey,
		}
	}

	// OAuth Bearer Token
	if bearerToken := r.Header.Get("X-Bearer-Token"); bearerToken != "" {
		bundle.Credentials["oauth"] = &models.PassthroughCredential{
			Type:  "bearer",
			Token: bearerToken,
		}
	}

	// Return nil if no credentials were found
	if len(bundle.Credentials) == 0 {
		return nil
	}

	return bundle
}

func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check IP rate limit
	if s.ipRateLimiter != nil {
		clientIP := s.getClientIP(r)
		if !s.ipRateLimiter.Allow(clientIP) {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
	}

	// Authenticate request
	claims, err := s.authenticateRequest(r)
	if err != nil {
		s.logger.Error("WebSocket authentication failed", map[string]interface{}{
			"error":       err.Error(),
			"remote_addr": r.RemoteAddr,
			"path":        r.URL.Path,
		})
		s.metricsCollector.RecordConnectionFailure("auth_failed")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check connection limit
	if s.ConnectionCount() >= s.config.MaxConnections {
		s.metricsCollector.RecordConnectionFailure("max_connections")
		http.Error(w, "Too Many Connections", http.StatusServiceUnavailable)
		return
	}

	// Accept WebSocket connection
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols: []string{"mcp.v1"},
	})
	if err != nil {
		s.logger.Error("WebSocket accept failed", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Set connection limits
	conn.SetReadLimit(s.config.MaxMessageSize)

	// Extract passthrough authentication from headers
	passthroughAuth := s.extractPassthroughAuth(r)
	if passthroughAuth != nil {
		s.logger.Debug("Passthrough authentication extracted", map[string]interface{}{
			"services": len(passthroughAuth.Credentials),
		})
	}

	// Get connection from pool
	connection := s.connectionPool.Get()

	// Generate a unique connection ID
	connectionID := uuid.New().String()

	s.logger.Debug("WebSocket connection configured", map[string]interface{}{
		"connection_id":    connectionID,
		"max_message_size": s.config.MaxMessageSize,
		"max_message_kb":   s.config.MaxMessageSize / 1024,
	})

	// Generate agent ID - use UserID if available and not zero UUID
	agentID := claims.UserID
	zeroUUID := "00000000-0000-0000-0000-000000000000"
	if agentID == "" || agentID == zeroUUID {
		// Generate a new UUID for agents without explicit user ID or with zero UUID
		agentID = uuid.New().String()
		s.logger.Info("Generated new agent ID", map[string]interface{}{
			"connection_id":    connectionID,
			"agent_id":         agentID,
			"tenant_id":        claims.TenantID,
			"original_user_id": claims.UserID,
		})
	}

	// Initialize connection - reuse existing ws.Connection if available
	if connection.Connection == nil {
		// This should not happen with properly initialized pool, but handle it gracefully
		connection.Connection = &ws.Connection{}
		connection.State.Store(ws.ConnectionStateClosed)
	}

	// Update connection properties
	connection.ID = connectionID
	connection.AgentID = agentID

	// Ensure we have a valid tenant ID
	if claims != nil && claims.TenantID != "" {
		connection.TenantID = claims.TenantID
	} else {
		// Use a default tenant ID for development/testing
		connection.TenantID = "00000000-0000-0000-0000-000000000001"
		s.logger.Warn("No tenant ID in claims, using default", map[string]interface{}{
			"connection_id": connectionID,
			"agent_id":      agentID,
		})
	}

	connection.CreatedAt = time.Now()
	connection.LastPing = time.Now()

	connection.conn = conn
	connection.hub = s

	// Ensure channels are initialized
	if connection.send == nil {
		connection.send = make(chan []byte, 256)
	}
	if connection.afterSend == nil {
		connection.afterSend = make(chan *PostActionConfig, 32) // Buffered to prevent blocking
	}
	if connection.closed == nil {
		connection.closed = make(chan struct{})
	}

	// Set initial state
	connection.SetState(ws.ConnectionStateConnecting)

	// Initialize connection state with authentication claims
	if connection.state == nil {
		connection.state = &ConnectionState{}
	}
	connection.state.Claims = claims
	connection.state.PassthroughAuth = passthroughAuth

	// Detect connection mode based on headers and user agent
	connectionMode := s.detectConnectionMode(r)
	connection.state.ConnectionMode = connectionMode

	// Log connection mode
	s.logger.Info("Connection mode detected", map[string]interface{}{
		"connection_id": connection.ID,
		"mode":          connectionMode.String(),
		"tenant_id":     connection.TenantID,
		"user_agent":    r.Header.Get("User-Agent"),
	})

	// Register connection
	s.addConnection(connection)

	// Generate session key for HMAC signatures
	if s.config.Security.HMACSignatures {
		_, err := s.sessionManager.GenerateSessionKey(connection.ID)
		if err != nil {
			s.logger.Warn("Failed to generate session key", map[string]interface{}{
				"connection_id": connection.ID,
				"error":         err.Error(),
			})
		}
	}

	// Set connected state
	connection.SetState(ws.ConnectionStateConnected)

	// Start connection handlers
	go connection.writePump()
	go connection.readPump()

	s.logger.Info("WebSocket connection established", map[string]interface{}{
		"connection_id": connection.ID,
		"agent_id":      connection.AgentID,
		"tenant_id":     connection.TenantID,
	})

	// Record connection metrics
	s.metricsCollector.RecordConnection(connection.TenantID)
}

// detectConnectionMode detects the type of connection based on headers
func (s *Server) detectConnectionMode(r *http.Request) ConnectionMode {
	userAgent := r.Header.Get("User-Agent")

	// Check for Claude Code
	if strings.Contains(userAgent, "Claude-Code") ||
		r.Header.Get("X-Claude-Code-Version") != "" ||
		r.Header.Get("X-Client-Name") == "claude-code" {
		s.logger.Info("Claude Code connection detected", map[string]interface{}{
			"user_agent": userAgent,
			"version":    r.Header.Get("X-Claude-Code-Version"),
		})
		return ModeClaudeCode
	}

	// Check for IDE connection (Cursor, VS Code, etc.)
	if strings.Contains(userAgent, "Cursor") ||
		strings.Contains(userAgent, "VSCode") ||
		strings.Contains(userAgent, "Visual-Studio-Code") ||
		r.Header.Get("X-IDE-Name") != "" {
		s.logger.Info("IDE connection detected", map[string]interface{}{
			"user_agent": userAgent,
			"ide_name":   r.Header.Get("X-IDE-Name"),
		})
		return ModeIDE
	}

	// Check for agent connection
	if r.Header.Get("X-Agent-ID") != "" ||
		r.Header.Get("X-Agent-Type") != "" ||
		strings.Contains(userAgent, "DevMesh-Agent") {
		s.logger.Info("Agent connection detected", map[string]interface{}{
			"user_agent": userAgent,
			"agent_id":   r.Header.Get("X-Agent-ID"),
			"agent_type": r.Header.Get("X-Agent-Type"),
		})
		return ModeAgent
	}

	// Check for MCP client in user agent
	if strings.Contains(strings.ToLower(userAgent), "mcp") ||
		r.Header.Get("X-MCP-Version") != "" {
		s.logger.Info("Standard MCP connection detected", map[string]interface{}{
			"user_agent":  userAgent,
			"mcp_version": r.Header.Get("X-MCP-Version"),
		})
		return ModeStandardMCP
	}

	// Default to standard MCP
	s.logger.Debug("Defaulting to standard MCP connection", map[string]interface{}{
		"user_agent": userAgent,
	})
	return ModeStandardMCP
}

// ConnectionCount returns the current number of active connections
func (s *Server) ConnectionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.connections)
}

// GetConnectionPassthroughAuth returns the passthrough auth for a connection
func (s *Server) GetConnectionPassthroughAuth(connID string) *models.PassthroughAuthBundle {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if conn, ok := s.connections[connID]; ok && conn.state != nil {
		return conn.state.PassthroughAuth
	}
	return nil
}

// addConnection registers a new connection
func (s *Server) addConnection(conn *Connection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connections[conn.ID] = conn

	// Register with notification manager
	if s.notificationManager != nil {
		s.notificationManager.RegisterConnection(conn)
	}

	// Increment metrics
	s.metrics.IncrementCounter("websocket_connections_total", 1)
	s.metrics.RecordGauge("websocket_connections_active", float64(len(s.connections)), nil)
}

// removeConnection unregisters a connection
func (s *Server) removeConnection(conn *Connection) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.connections[conn.ID]; ok {
		delete(s.connections, conn.ID)

		// Unregister from notification manager
		if s.notificationManager != nil {
			s.notificationManager.UnregisterConnection(conn.ID)
		}

		// Unsubscribe from all subscriptions
		if s.subscriptionManager != nil {
			_ = s.subscriptionManager.UnsubscribeAll(conn.ID)
		}

		// Clean up session key
		if s.sessionManager != nil {
			s.sessionManager.RemoveSessionKey(conn.ID)
		}

		// Update metrics
		s.metrics.RecordGauge("websocket_connections_active", float64(len(s.connections)), nil)

		s.logger.Info("WebSocket connection closed", map[string]interface{}{
			"connection_id": conn.ID,
			"agent_id":      conn.AgentID,
		})

		// Record disconnection metrics
		duration := time.Since(conn.CreatedAt)
		s.metricsCollector.RecordDisconnection(conn.TenantID, duration)

		// Return connection to pool after cleanup
		s.connectionPool.Put(conn)
	}
}

// GetConnection retrieves a connection by ID
func (s *Server) GetConnection(id string) (*Connection, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conn, ok := s.connections[id]
	return conn, ok
}

// Broadcast sends a message to all connections for a specific tenant
func (s *Server) Broadcast(tenantID string, message []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, conn := range s.connections {
		if conn.TenantID == tenantID {
			select {
			case conn.send <- message:
			default:
				// Channel full, skip this connection
				s.logger.Warn("Skipping broadcast to connection - channel full", map[string]interface{}{
					"connection_id": conn.ID,
				})
			}
		}
	}
}

// SendToAgent sends a message to all connections for a specific agent
func (s *Server) SendToAgent(agentID string, message []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, conn := range s.connections {
		if conn.AgentID == agentID {
			select {
			case conn.send <- message:
			default:
				// Channel full, skip this connection
				s.logger.Warn("Skipping message to connection - channel full", map[string]interface{}{
					"connection_id": conn.ID,
				})
			}
		}
	}
}

// SetToolRegistry sets the tool registry for the server
func (s *Server) SetToolRegistry(registry ToolRegistry) {
	s.toolRegistry = registry
}

// SetRESTClient sets the REST API client for proxying tool requests
func (s *Server) SetRESTClient(client clients.RESTAPIClient) {
	if client != nil {
		s.restAPIClient = client
		s.logger.Info("REST API client configured for WebSocket server", nil)
	}
}

// SetMCPHandler sets the MCP protocol handler
func (s *Server) SetMCPHandler(handler interface{}) {
	s.mcpHandler = handler
	s.logger.Info("MCP protocol handler configured for WebSocket server", nil)
}

// SetContextManager sets the context manager for the server
func (s *Server) SetContextManager(manager ContextManager) {
	s.contextManager = manager
}

// SetEventBus sets the event bus for the server
func (s *Server) SetEventBus(bus EventBus) {
	s.eventBus = bus
}

// SetConversationSessionManager sets the conversation session manager
func (s *Server) SetConversationSessionManager(manager *ConversationSessionManager) {
	s.conversationManager = manager
}

// SetWorkflowService sets the workflow service for the server
func (s *Server) SetWorkflowService(service services.WorkflowService) {
	s.workflowService = service
	// Update workflow engine if it exists
	if s.workflowEngine != nil {
		s.workflowEngine = NewWorkflowEngine(s.logger, s.metrics, service, s.taskService)
		s.workflowEngine.SetNotificationManager(s.notificationManager)
	}
}

// SetTaskService sets the task service for the server
func (s *Server) SetTaskService(service services.TaskService) {
	s.taskService = service
	// Update workflow engine if it exists
	if s.workflowEngine != nil {
		s.workflowEngine = NewWorkflowEngine(s.logger, s.metrics, s.workflowService, service)
		s.workflowEngine.SetNotificationManager(s.notificationManager)
	}
}

// SetServices sets all the services at once
func (s *Server) SetServices(taskService services.TaskService, workflowService services.WorkflowService,
	workspaceService services.WorkspaceService, documentService services.DocumentService,
	conflictService services.ConflictResolutionService, agentRepo agentRepository.Repository,
	cache cache.Cache) {
	s.taskService = taskService
	s.workflowService = workflowService
	s.workspaceService = workspaceService
	s.documentService = documentService
	s.conflictService = conflictService

	// Replace in-memory agent registry with database-backed one if repository is available
	if agentRepo != nil && cache != nil {
		s.agentRegistry = NewDBAgentRegistry(agentRepo, cache, s.logger, s.metrics)
		s.logger.Info("Using database-backed agent registry", nil)
	} else {
		s.logger.Warn("Agent repository or cache not available, using in-memory agent registry", nil)
	}

	// Reinitialize workflow engine with real services
	if workflowService != nil && taskService != nil {
		s.workflowEngine = NewWorkflowEngine(s.logger, s.metrics, workflowService, taskService)
		s.workflowEngine.SetNotificationManager(s.notificationManager)
	}
}

// Close gracefully shuts down the server
func (s *Server) Close() error {
	s.mu.Lock()

	// Collect all connections to close
	conns := make([]*Connection, 0, len(s.connections))
	for _, conn := range s.connections {
		conns = append(conns, conn)
	}

	// Clear connections map early to prevent new operations
	s.connections = make(map[string]*Connection)
	s.mu.Unlock()

	// Close all connections (without holding the lock)
	for _, conn := range conns {
		// This will trigger the graceful close process
		_ = conn.Close()
	}

	// Stop the connection pool maintenance goroutine
	if s.connectionPool != nil {
		s.connectionPool.Stop()
	}

	return nil
}

// authenticateRequest validates the request and returns auth claims
func (s *Server) authenticateRequest(r *http.Request) (*auth.Claims, error) {
	// Check if auth service is available
	if s.auth == nil {
		s.logger.Error("Auth service not initialized", nil)
		return nil, errors.New("authentication service unavailable")
	}

	// Log incoming headers for debugging
	s.logger.Debug("WebSocket auth request headers", map[string]interface{}{
		"authorization": r.Header.Get("Authorization"),
		"x-api-key":     r.Header.Get("X-API-Key"),
		"user-agent":    r.Header.Get("User-Agent"),
	})

	// Check Authorization header first
	authHeader := r.Header.Get("Authorization")

	// If no Authorization header, check custom API key header (e.g., X-API-Key)
	if authHeader == "" && s.auth.GetConfig() != nil && s.auth.GetConfig().APIKeyHeader != "" {
		customHeader := s.auth.GetConfig().APIKeyHeader
		authHeader = r.Header.Get(customHeader)
		if authHeader != "" {
			s.logger.Debug("Using custom API key header", map[string]interface{}{
				"header": customHeader,
			})
		}
	}

	if authHeader == "" {
		s.logger.Warn("No authentication header found", nil)
		return nil, auth.ErrNoAPIKey
	}

	// Extract token (with or without Bearer prefix)
	token := authHeader
	if strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimPrefix(authHeader, "Bearer ")
	}

	// Try to validate as JWT first (has two dots)
	if strings.Count(token, ".") == 2 {
		user, err := s.auth.ValidateJWT(r.Context(), token)
		if err == nil {
			// Convert User to Claims
			claims := &auth.Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Subject: user.ID.String(),
				},
				TenantID: user.TenantID.String(),
				UserID:   user.ID.String(),
				Scopes:   user.Scopes,
			}

			// If TenantID is empty, check X-Tenant-ID header (for e2e tests)
			if claims.TenantID == "" {
				if tenantID := r.Header.Get("X-Tenant-ID"); tenantID != "" {
					claims.TenantID = tenantID
					s.logger.Debug("Using X-Tenant-ID header for JWT", map[string]interface{}{
						"tenant_id": tenantID,
						"user_id":   user.ID.String(),
					})
				}
			}

			return claims, nil
		}
		// If JWT validation fails, fall through to try as API key
	}

	// Try as API key
	user, err := s.auth.ValidateAPIKey(r.Context(), token)
	if err != nil {
		keyPrefix := token
		if len(keyPrefix) > 8 {
			keyPrefix = keyPrefix[:8]
		}
		s.logger.Warn("API key validation failed", map[string]interface{}{
			"error":      err.Error(),
			"key_prefix": keyPrefix,
		})
		return nil, err
	}

	// Convert User to Claims
	claims := &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: user.ID.String(),
		},
		TenantID: user.TenantID.String(),
		UserID:   user.ID.String(),
		Scopes:   user.Scopes,
	}

	// If TenantID is empty, check X-Tenant-ID header (for e2e tests)
	if claims.TenantID == "" {
		if tenantID := r.Header.Get("X-Tenant-ID"); tenantID != "" {
			claims.TenantID = tenantID
			s.logger.Debug("Using X-Tenant-ID header", map[string]interface{}{
				"tenant_id": tenantID,
				"user_id":   user.ID.String(),
			})
		}
	}

	return claims, nil
}

// getClientIP extracts the client IP address from the request
func (s *Server) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
		return r.RemoteAddr[:idx]
	}

	return r.RemoteAddr
}
