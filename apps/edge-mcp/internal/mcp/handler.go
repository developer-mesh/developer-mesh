package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/auth"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/cache"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/core"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/metrics"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/middleware"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/platform"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tracing"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/validation"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/harness"
	"github.com/developer-mesh/developer-mesh/pkg/utils"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// MCPMessage represents a JSON-RPC message in the MCP protocol
type MCPMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// SemanticContextManager defines the interface for semantic context operations
// This interface matches the repository.SemanticContextManager but uses interface{} for flexibility
type SemanticContextManager interface {
	CreateContext(ctx context.Context, req interface{}) (interface{}, error)
	GetContext(ctx context.Context, contextID string, opts interface{}) (interface{}, error)
	UpdateContext(ctx context.Context, contextID string, update interface{}) error
	GetRelevantContext(ctx context.Context, contextID string, query string, maxTokens int) (interface{}, error)
	CompactContext(ctx context.Context, contextID string, strategy string) error
	SearchContext(ctx context.Context, query string, contextID string, limit int) (interface{}, error)
}

// Handler manages MCP protocol connections
type Handler struct {
	tools          *tools.Registry
	cache          cache.Cache
	coreClient     *core.Client
	authenticator  auth.Authenticator
	sessions       map[string]*Session
	sessionsMu     sync.RWMutex
	logger         observability.Logger
	refreshManager *tools.RefreshManager
	metrics        *metrics.Metrics
	spanHelper     *tracing.SpanHelper
	streamManager  *StreamManager          // Stream manager for response streaming
	batchExecutor  *BatchExecutor          // Batch executor for batching tool calls
	rateLimiter    *middleware.RateLimiter // Rate limiter for request throttling
	validator      *validation.Validator   // Input validator for security and validation

	// Semantic context manager for enhanced context operations (optional)
	// This is interface{} to support dynamic typing - will be type asserted when used
	semanticContextMgr interface{}

	// Request tracking for cancellation
	activeRequests map[interface{}]context.CancelFunc
	requestsMu     sync.RWMutex

	// Goroutine tracking for cleanup
	activeRefreshes sync.WaitGroup
}

// Session represents an MCP session
type Session struct {
	ID              string
	ConnectionID    string
	Initialized     bool
	TenantID        string
	EdgeMCPID       string
	CoreSession     string // Core Platform session ID for context sync
	CreatedAt       time.Time
	LastActivity    time.Time
	PassthroughAuth *models.PassthroughAuthBundle // User-specific credentials for pass-through
}

// NewHandler creates a new MCP handler
func NewHandler(
	toolRegistry *tools.Registry,
	cache cache.Cache,
	coreClient *core.Client,
	authenticator auth.Authenticator,
	logger observability.Logger,
	metricsCollector *metrics.Metrics,
	tracerProvider *tracing.TracerProvider,
	semanticContextMgr interface{}, // Optional semantic context manager for enhanced context operations
) *Handler {
	var spanHelper *tracing.SpanHelper
	if tracerProvider != nil {
		spanHelper = tracing.NewSpanHelper(tracerProvider)
	}

	// Create stream manager with default config
	streamConfig := DefaultStreamConfig()
	streamManager := NewStreamManager(logger, streamConfig)

	// Create batch executor with default config
	batchConfig := DefaultBatchConfig()
	batchExecutor := NewBatchExecutor(toolRegistry, batchConfig, logger)

	// Create rate limiter with default config
	rateLimitConfig := middleware.DefaultRateLimitConfig()
	rateLimiter := middleware.NewRateLimiter(rateLimitConfig, logger, metricsCollector)

	// Create input validator with default config
	validatorConfig := validation.DefaultConfig()
	validator := validation.NewValidator(validatorConfig, logger)

	h := &Handler{
		tools:              toolRegistry,
		cache:              cache,
		coreClient:         coreClient,
		authenticator:      authenticator,
		sessions:           make(map[string]*Session),
		logger:             logger,
		metrics:            metricsCollector,
		spanHelper:         spanHelper,
		streamManager:      streamManager,
		batchExecutor:      batchExecutor,
		rateLimiter:        rateLimiter,
		validator:          validator,
		semanticContextMgr: semanticContextMgr,
		activeRequests:     make(map[interface{}]context.CancelFunc),
	}

	// Setup refresh manager if core client is available
	if coreClient != nil {
		refreshConfig := tools.DefaultRefreshConfig()
		h.refreshManager = tools.NewRefreshManager(
			toolRegistry,
			refreshConfig,
			logger,
			func(ctx context.Context) ([]tools.ToolDefinition, error) {
				return coreClient.FetchRemoteTools(ctx)
			},
		)
		// Start automatic refresh
		h.refreshManager.Start(context.Background())
	}

	return h
}

// HandleConnection handles a WebSocket connection
func (h *Handler) HandleConnection(conn *websocket.Conn, r *http.Request) {
	sessionID := uuid.New().String()

	// Extract passthrough authentication from headers
	passthroughAuth := h.extractPassthroughAuth(r)

	session := &Session{
		ID:              sessionID,
		ConnectionID:    uuid.New().String(),
		CreatedAt:       time.Now(),
		LastActivity:    time.Now(),
		PassthroughAuth: passthroughAuth,
	}

	h.sessionsMu.Lock()
	h.sessions[sessionID] = session
	h.sessionsMu.Unlock()

	// Create connection context with session tracking
	ctx, cancel := context.WithCancel(context.Background())
	ctx = observability.WithSessionID(ctx, sessionID)
	defer cancel() // Ensure context is cancelled

	defer func() {
		// Clean up session
		h.sessionsMu.Lock()
		delete(h.sessions, sessionID)
		h.sessionsMu.Unlock()

		// Cancel context to stop all goroutines
		cancel()

		// Close connection
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()

	// Start ping ticker with proper cleanup
	pingDone := make(chan struct{})
	go h.pingLoop(ctx, conn, pingDone)
	defer func() {
		cancel()   // Signal ping loop to stop
		<-pingDone // Wait for ping loop to finish
	}()

	// Message handling loop
	for {
		var msg MCPMessage
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
				h.logger.Error("WebSocket error", map[string]interface{}{
					"error":      err.Error(),
					"session_id": sessionID,
				})
			}
			break
		}

		// Generate unique request ID for this message
		requestID := observability.GenerateRequestID()
		msgCtx := observability.WithRequestID(ctx, requestID)

		// Add tenant ID to context if available
		if session.TenantID != "" {
			msgCtx = observability.WithTenantID(msgCtx, session.TenantID)
		}

		// Create request-scoped logger with context fields
		reqLogger := observability.LoggerFromContext(msgCtx, h.logger)

		// Update activity
		h.sessionsMu.Lock()
		if s, exists := h.sessions[sessionID]; exists {
			s.LastActivity = time.Now()
		}
		h.sessionsMu.Unlock()

		// Handle message
		response, err := h.handleMessage(sessionID, &msg)
		if err != nil {
			var errResp *models.ErrorResponse
			if errors.As(err, &errResp) {
				response = &MCPMessage{
					JSONRPC: "2.0",
					ID:      msg.ID,
					Error:   ToMCPError(errResp),
				}
			} else {
				// Fallback for non-structured errors - convert to semantic error
				response = &MCPMessage{
					JSONRPC: "2.0",
					ID:      msg.ID,
					Error: ToMCPError(
						errorTemplates.InternalError("unknown", err),
					),
				}
			}
		}

		if response != nil {
			// Check if response should be streamed based on size
			shouldStream := false
			var responseBytes []byte

			// Try to marshal the response to check size
			if responseBytes, err := json.Marshal(response); err == nil {
				shouldStream = ShouldStream(responseBytes, StreamThreshold)
			}

			if shouldStream && msg.ID != nil {
				// Use streaming for large responses
				reqLogger.Info("Streaming large response", map[string]interface{}{
					"request_id":    msg.ID,
					"response_size": len(responseBytes),
				})

				// Create stream for this request
				stream, err := h.streamManager.CreateStream(msg.ID, conn)
				if err != nil {
					reqLogger.Error("Failed to create stream", map[string]interface{}{
						"error": err.Error(),
					})
					// Fallback to non-streaming
					if err := wsjson.Write(msgCtx, conn, response); err != nil {
						reqLogger.Error("Failed to write response", map[string]interface{}{
							"error": err.Error(),
						})
						break
					}
				} else {
					// Stream the chunked content
					if err := stream.SendChunkedContent(msg.ID, responseBytes, "application/json"); err != nil {
						reqLogger.Error("Failed to stream content", map[string]interface{}{
							"error": err.Error(),
						})
					}

					// Send final response confirmation
					finalResponse := &MCPMessage{
						JSONRPC: "2.0",
						ID:      msg.ID,
						Result: map[string]interface{}{
							"streamed":        true,
							"chunks_complete": true,
						},
					}
					if err := stream.SendFinalResponse(msg.ID, finalResponse.Result); err != nil {
						reqLogger.Error("Failed to send final response", map[string]interface{}{
							"error": err.Error(),
						})
					}

					// Close the stream
					_ = h.streamManager.CloseStream(msg.ID)
				}
			} else {
				// Regular non-streaming response
				if err := wsjson.Write(msgCtx, conn, response); err != nil {
					reqLogger.Error("Failed to write response", map[string]interface{}{
						"error": err.Error(),
					})
					break
				}
			}
		}
	}
}

// pingLoop handles WebSocket ping/pong with proper cleanup
func (h *Handler) pingLoop(ctx context.Context, conn *websocket.Conn, done chan struct{}) {
	defer close(done) // Signal completion when exiting

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Set deadline for ping
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Ping(pingCtx)
			cancel()

			if err != nil {
				h.logger.Debug("Ping failed, closing connection", map[string]interface{}{
					"error": err.Error(),
				})
				return
			}
		case <-ctx.Done():
			h.logger.Debug("Ping loop stopped due to context cancellation", nil)
			return
		}
	}
}

// Shutdown gracefully shuts down the MCP handler
func (h *Handler) Shutdown(ctx context.Context) error {
	h.logger.Info("Shutting down MCP handler", nil)

	// Cancel all active requests
	h.requestsMu.Lock()
	for _, cancel := range h.activeRequests {
		cancel()
	}
	h.requestsMu.Unlock()

	// Close all active streams
	if h.streamManager != nil {
		h.streamManager.CloseAll()
	}

	// Close rate limiter
	if h.rateLimiter != nil {
		h.rateLimiter.Close()
	}

	// Wait for active refreshes with timeout
	done := make(chan struct{})
	go func() {
		h.activeRefreshes.Wait()
		close(done)
	}()

	select {
	case <-done:
		h.logger.Info("All refresh operations completed", nil)
	case <-ctx.Done():
		h.logger.Warn("Shutdown timeout, some operations may be incomplete", nil)
	}

	// Close all sessions
	h.sessionsMu.Lock()
	for id := range h.sessions {
		delete(h.sessions, id)
	}
	h.sessionsMu.Unlock()

	return nil
}

// HandleStdio handles MCP protocol over stdin/stdout for Claude Code integration
func (h *Handler) HandleStdio() {
	sessionID := uuid.New().String()

	// Log all environment variables that might contain tokens (for debugging)
	// Write to stderr so it appears in Claude logs
	fmt.Fprintf(os.Stderr, "[edge-mcp] Checking environment variables:\n")
	fmt.Fprintf(os.Stderr, "  HARNESS_TOKEN: %v\n", os.Getenv("HARNESS_TOKEN") != "")
	fmt.Fprintf(os.Stderr, "  HARNESS_API_KEY: %v\n", os.Getenv("HARNESS_API_KEY") != "")
	fmt.Fprintf(os.Stderr, "  GITHUB_TOKEN: %v (len=%d)\n", os.Getenv("GITHUB_TOKEN") != "", len(os.Getenv("GITHUB_TOKEN")))
	fmt.Fprintf(os.Stderr, "  AWS_ACCESS_KEY_ID: %v\n", os.Getenv("AWS_ACCESS_KEY_ID") != "")
	fmt.Fprintf(os.Stderr, "  DEV_MESH_URL: %s\n", os.Getenv("DEV_MESH_URL"))
	fmt.Fprintf(os.Stderr, "  DEV_MESH_API_KEY: %v\n", os.Getenv("DEV_MESH_API_KEY") != "")

	h.logger.Info("Checking environment variables for tokens", map[string]interface{}{
		"has_HARNESS_TOKEN":    os.Getenv("HARNESS_TOKEN") != "",
		"has_HARNESS_API_KEY":  os.Getenv("HARNESS_API_KEY") != "",
		"has_GITHUB_TOKEN":     os.Getenv("GITHUB_TOKEN") != "",
		"has_AWS_ACCESS_KEY":   os.Getenv("AWS_ACCESS_KEY_ID") != "",
		"has_DEV_MESH_URL":     os.Getenv("DEV_MESH_URL") != "",
		"has_DEV_MESH_API_KEY": os.Getenv("DEV_MESH_API_KEY") != "",
	})

	// Extract passthrough authentication from environment variables
	passthroughAuth := h.extractPassthroughAuthFromEnv()

	session := &Session{
		ID:              sessionID,
		ConnectionID:    uuid.New().String(),
		CreatedAt:       time.Now(),
		LastActivity:    time.Now(),
		PassthroughAuth: passthroughAuth,
	}

	h.sessionsMu.Lock()
	h.sessions[sessionID] = session
	h.sessionsMu.Unlock()

	defer func() {
		h.sessionsMu.Lock()
		delete(h.sessions, sessionID)
		h.sessionsMu.Unlock()
	}()

	// Create JSON encoder/decoder for stdio
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	// Message handling loop
	for {
		var msg MCPMessage
		if err := decoder.Decode(&msg); err != nil {
			// EOF is expected when stdin is closed
			if err == io.EOF || strings.Contains(err.Error(), "file already closed") {
				h.logger.Debug("Stdio connection closed", nil)
				break
			}
			h.logger.Error("Failed to decode message from stdin", map[string]interface{}{
				"error": err.Error(),
			})
			// Send error response
			errorResponse := &MCPMessage{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &MCPError{
					Code:    -32700,
					Message: "Parse error",
					Data:    err.Error(),
				},
			}
			_ = encoder.Encode(errorResponse)
			continue
		}

		// Update activity
		h.sessionsMu.Lock()
		if s, exists := h.sessions[sessionID]; exists {
			s.LastActivity = time.Now()
		}
		h.sessionsMu.Unlock()

		// Handle message
		response, err := h.handleMessage(sessionID, &msg)
		if err != nil {
			var errResp *models.ErrorResponse
			if errors.As(err, &errResp) {
				response = &MCPMessage{
					JSONRPC: "2.0",
					ID:      msg.ID,
					Error:   ToMCPError(errResp),
				}
			} else {
				// Fallback for non-structured errors - convert to semantic error
				response = &MCPMessage{
					JSONRPC: "2.0",
					ID:      msg.ID,
					Error: ToMCPError(
						errorTemplates.InternalError("unknown", err),
					),
				}
			}
		}

		// Check for shutdown
		if msg.Method == "shutdown" {
			if response != nil {
				_ = encoder.Encode(response)
			}
			h.logger.Info("Received shutdown request, exiting stdio mode", nil)
			break
		}

		if response != nil {
			if err := encoder.Encode(response); err != nil {
				h.logger.Error("Failed to write response to stdout", map[string]interface{}{
					"error": err.Error(),
				})
				break
			}
		}
	}
}

// extractPassthroughAuthFromEnv extracts passthrough auth from environment variables
func (h *Handler) extractPassthroughAuthFromEnv() *models.PassthroughAuthBundle {
	bundle := &models.PassthroughAuthBundle{
		Credentials: make(map[string]*models.PassthroughCredential),
	}

	// Check for common service tokens in environment
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		bundle.Credentials["github"] = &models.PassthroughCredential{
			Type:  "bearer",
			Token: token,
		}
		h.logger.Info("Found GitHub passthrough token in environment", map[string]interface{}{
			"token_len": len(token),
		})
	}

	if accessKey := os.Getenv("AWS_ACCESS_KEY_ID"); accessKey != "" {
		if secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY"); secretKey != "" {
			bundle.Credentials["aws"] = &models.PassthroughCredential{
				Type: "aws_signature",
				Properties: map[string]string{
					"access_key_id":     accessKey,
					"secret_access_key": secretKey,
				},
			}
			if region := os.Getenv("AWS_REGION"); region != "" {
				bundle.Credentials["aws"].Properties["region"] = region
			}
			h.logger.Debug("Found AWS passthrough credentials in environment", nil)
		}
	}

	// Check for Harness token (try both possible env var names)
	harnessToken := os.Getenv("HARNESS_TOKEN")
	fmt.Fprintf(os.Stderr, "[edge-mcp] Checking HARNESS_TOKEN: found=%v, len=%d\n", harnessToken != "", len(harnessToken))
	h.logger.Info("Checking HARNESS_TOKEN environment variable", map[string]interface{}{
		"found":     harnessToken != "",
		"token_len": len(harnessToken),
	})
	if harnessToken == "" {
		harnessToken = os.Getenv("HARNESS_API_KEY")
		fmt.Fprintf(os.Stderr, "[edge-mcp] Checking HARNESS_API_KEY: found=%v, len=%d\n", harnessToken != "", len(harnessToken))
		h.logger.Info("Checking HARNESS_API_KEY environment variable", map[string]interface{}{
			"found":     harnessToken != "",
			"token_len": len(harnessToken),
		})
	}
	if harnessToken != "" {
		cred := &models.PassthroughCredential{
			Type:       "api_key",
			Token:      harnessToken,
			Properties: make(map[string]string),
		}

		// Discover Harness permissions and store them in credential properties
		if perms := h.discoverHarnessPermissions(context.Background(), harnessToken); perms != nil {
			// Store permissions as JSON in properties
			if permsJSON, err := json.Marshal(perms); err == nil {
				cred.Properties["permissions"] = string(permsJSON)
				fmt.Fprintf(os.Stderr, "[edge-mcp] Discovered Harness permissions: %d modules enabled\n", len(perms.EnabledModules))
				h.logger.Info("Discovered Harness permissions for passthrough auth", map[string]interface{}{
					"modules_count":   len(perms.EnabledModules),
					"resources_count": len(perms.ResourceAccess),
				})
			}
		}

		bundle.Credentials["harness"] = cred
		fmt.Fprintf(os.Stderr, "[edge-mcp] Added Harness token to passthrough bundle (len=%d)\n", len(harnessToken))
		h.logger.Info("Found Harness passthrough token in environment", map[string]interface{}{
			"token_len": len(harnessToken),
		})
	} else {
		fmt.Fprintf(os.Stderr, "[edge-mcp] WARNING: No Harness token found in environment\n")
		h.logger.Warn("No Harness token found in environment", nil)
	}

	if len(bundle.Credentials) == 0 {
		return nil
	}

	// Debug: Log what credentials are in the bundle
	fmt.Fprintf(os.Stderr, "[edge-mcp] Passthrough bundle contains %d credentials:\n", len(bundle.Credentials))
	for provider, cred := range bundle.Credentials {
		tokenLen := 0
		if cred.Token != "" {
			tokenLen = len(cred.Token)
		}
		fmt.Fprintf(os.Stderr, "  - %s: type=%s, token_len=%d\n", provider, cred.Type, tokenLen)
	}

	return bundle
}

// handleMessage processes an MCP message
func (h *Handler) handleMessage(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	// Get session for rate limiting
	h.sessionsMu.RLock()
	session, exists := h.sessions[sessionID]
	h.sessionsMu.RUnlock()

	if !exists {
		return nil, NewProtocolError(msg.Method, "Session not found",
			"The session does not exist or has expired")
	}

	// Extract tool name for per-tool rate limiting
	var toolName string
	if msg.Method == "tools/call" {
		var params struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(msg.Params, &params); err == nil {
			toolName = params.Name
		}
	}

	// Check rate limits (skip for initialize and ping methods)
	if h.rateLimiter != nil && msg.Method != "initialize" && msg.Method != "initialized" && msg.Method != "ping" {
		result := h.rateLimiter.CheckRateLimit(context.Background(), session.TenantID, toolName)
		if !result.Allowed {
			// Rate limit exceeded - return error with rate limit info
			errorData := h.rateLimiter.CreateRateLimitError(result)
			return &MCPMessage{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Error: &MCPError{
					Code:    429, // Too Many Requests
					Message: fmt.Sprintf("Rate limit exceeded: %s", result.LimitType),
					Data:    errorData,
				},
			}, nil
		}
	}

	switch msg.Method {
	case "initialize":
		return h.handleInitialize(sessionID, msg)
	case "initialized":
		return h.handleInitialized(sessionID, msg)
	case "ping":
		return h.handlePing(msg)
	case "shutdown":
		return h.handleShutdown(sessionID, msg)
	case "tools/list":
		return h.handleToolsList(sessionID, msg)
	case "tools/call":
		return h.handleToolCall(sessionID, msg)
	case "tools/batch":
		return h.handleBatchToolCall(sessionID, msg)
	case "resources/list":
		return h.handleResourcesList(sessionID, msg)
	case "resources/read":
		return h.handleResourceRead(sessionID, msg)
	case "prompts/list":
		return h.handlePromptsList(sessionID, msg)
	case "logging/setLevel":
		return h.handleLoggingSetLevel(sessionID, msg)
	case "$/cancelRequest":
		return h.handleCancelRequest(sessionID, msg)
	default:
		return nil, NewProtocolError(msg.Method, "Method not found",
			fmt.Sprintf("The method '%s' is not supported by this server", msg.Method))
	}
}

// handleInitialize handles the initialize request
func (h *Handler) handleInitialize(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
		ClientInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Type    string `json:"type,omitempty"`
		} `json:"clientInfo"`
		Credentials *models.PassthroughAuthBundle `json:"credentials,omitempty"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil, NewProtocolError("initialize", "Invalid initialize params",
			fmt.Sprintf("Failed to parse initialize parameters: %v", err))
	}

	// Validate protocol version
	if err := h.validator.ValidateMCPProtocolVersion(params.ProtocolVersion); err != nil {
		h.validator.LogValidationFailure(context.Background(), err, map[string]interface{}{
			"method":     "initialize",
			"version":    params.ProtocolVersion,
			"session":    sessionID,
			"message_id": msg.ID,
		})
		errResp := h.validator.ToErrorResponse(err, "initialize")
		return nil, errResp
	}

	// Validate client info
	clientInfoMap := map[string]interface{}{
		"name":    params.ClientInfo.Name,
		"version": params.ClientInfo.Version,
	}
	if params.ClientInfo.Type != "" {
		clientInfoMap["type"] = params.ClientInfo.Type
	}
	if err := h.validator.ValidateClientInfo(clientInfoMap); err != nil {
		h.validator.LogValidationFailure(context.Background(), err, map[string]interface{}{
			"method":     "initialize",
			"session":    sessionID,
			"message_id": msg.ID,
		})
		errResp := h.validator.ToErrorResponse(err, "initialize")
		return nil, errResp
	}

	// Update session
	h.sessionsMu.Lock()
	if session, exists := h.sessions[sessionID]; exists {
		session.Initialized = true

		// Store credentials in session if provided
		if params.Credentials != nil {
			session.PassthroughAuth = params.Credentials
			h.logger.Debug("Stored passthrough credentials in session", map[string]interface{}{
				"session_id":      sessionID,
				"num_credentials": len(params.Credentials.Credentials),
			})
		}

		// If connected to Core Platform, create a linked session
		if h.coreClient != nil {
			coreSessionID, err := h.coreClient.CreateSession(
				context.Background(),
				params.ClientInfo.Name,
				params.ClientInfo.Type,
			)
			if err != nil {
				h.logger.Warn("Failed to create Core Platform session", map[string]interface{}{
					"error": err.Error(),
				})
			} else {
				session.CoreSession = coreSessionID
			}

			// Trigger tool refresh on new connection
			if h.refreshManager != nil {
				// Create a context with timeout for refresh
				refreshCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

				// Track the goroutine
				h.activeRefreshes.Add(1)

				go func() {
					defer h.activeRefreshes.Done()
					defer cancel()

					h.logger.Debug("Refreshing tools on new connection", map[string]interface{}{
						"client": params.ClientInfo.Name,
					})

					h.refreshManager.OnReconnect(refreshCtx)
				}()
			}
		}
	}
	h.sessionsMu.Unlock()

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"protocolVersion": params.ProtocolVersion,
			"serverInfo": map[string]interface{}{
				"name":    "edge-mcp",
				"version": "1.0.0",
			},
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{
					"listChanged": true,
				},
				"resources": map[string]interface{}{
					"subscribe":   false, // Edge MCP doesn't support subscriptions
					"listChanged": false,
				},
				"prompts": map[string]interface{}{},
				"logging": map[string]interface{}{},
			},
		},
	}, nil
}

// handleInitialized handles the initialized notification
func (h *Handler) handleInitialized(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	// Client confirms initialization complete
	h.sessionsMu.Lock()
	if session, exists := h.sessions[sessionID]; exists {
		session.Initialized = true
	}
	h.sessionsMu.Unlock()

	// No response for notifications
	return nil, nil
}

// handlePing handles ping requests
func (h *Handler) handlePing(msg *MCPMessage) (*MCPMessage, error) {
	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  map[string]interface{}{},
	}, nil
}

// discoverHarnessPermissions discovers Harness permissions for a given API key
func (h *Handler) discoverHarnessPermissions(ctx context.Context, apiKey string) *harness.HarnessPermissions {
	// Create permission discoverer
	discoverer := harness.NewHarnessPermissionDiscoverer(h.logger)

	// Log discovery attempt with redacted key
	h.logger.Debug("Attempting Harness permission discovery", map[string]interface{}{
		"key_hint": utils.RedactString(apiKey),
	})

	// Discover permissions - pass the actual key to the discoverer
	permissions, err := discoverer.DiscoverPermissions(ctx, apiKey)
	if err != nil {
		// Sanitize error message to ensure it doesn't contain the API key
		sanitizedError := strings.ReplaceAll(err.Error(), apiKey, utils.RedactString(apiKey))
		h.logger.Warn("Failed to discover Harness permissions", map[string]interface{}{
			"error": sanitizedError,
		})
		return nil
	}

	// Log success without exposing sensitive data
	h.logger.Info("Successfully discovered Harness permissions", map[string]interface{}{
		"account_id":      permissions.AccountID,
		"modules_count":   len(permissions.EnabledModules),
		"resources_count": len(permissions.ResourceAccess),
	})

	return permissions
}

// filterToolsByPermissions filters tools based on session permissions
func (h *Handler) filterToolsByPermissions(allTools []tools.ToolDefinition, session *Session) []tools.ToolDefinition {
	if session == nil || session.PassthroughAuth == nil {
		// No filtering if no session or passthrough auth
		return allTools
	}

	// Check for Harness permissions in the passthrough bundle
	var harnessPermissions *harness.HarnessPermissions
	if harnessCred, ok := session.PassthroughAuth.Credentials["harness"]; ok && harnessCred != nil {
		if harnessCred.Properties != nil {
			if permsJSON, ok := harnessCred.Properties["permissions"]; ok && permsJSON != "" {
				// Decode the permissions from JSON
				if err := json.Unmarshal([]byte(permsJSON), &harnessPermissions); err == nil {
					h.logger.Debug("Found Harness permissions in session", map[string]interface{}{
						"session_id":    session.ID,
						"modules_count": len(harnessPermissions.EnabledModules),
					})
				}
			}
		}
	}

	// If no Harness permissions, return all tools
	if harnessPermissions == nil {
		return allTools
	}

	// Filter Harness tools based on permissions
	filteredTools := make([]tools.ToolDefinition, 0, len(allTools))
	for _, tool := range allTools {
		// Check if it's a Harness tool
		if strings.HasPrefix(tool.Name, "harness_") {
			// Extract the operation name from the tool name
			// e.g., "harness_pipelines_list" -> "pipelines/list"
			operationName := strings.TrimPrefix(tool.Name, "harness_")
			operationName = strings.ReplaceAll(operationName, "_", "/")

			// Check if this operation is allowed based on permissions
			if h.isHarnessOperationAllowed(operationName, harnessPermissions) {
				filteredTools = append(filteredTools, tool)
			} else {
				h.logger.Debug("Filtering out Harness tool due to permissions", map[string]interface{}{
					"tool_name": tool.Name,
					"operation": operationName,
				})
			}
		} else {
			// Non-Harness tools pass through
			filteredTools = append(filteredTools, tool)
		}
	}

	h.logger.Info("Filtered tools based on Harness permissions", map[string]interface{}{
		"total_tools":      len(allTools),
		"filtered_tools":   len(filteredTools),
		"harness_filtered": len(allTools) - len(filteredTools),
	})

	return filteredTools
}

// isHarnessOperationAllowed checks if a Harness operation is allowed based on permissions
func (h *Handler) isHarnessOperationAllowed(operation string, permissions *harness.HarnessPermissions) bool {
	// Parse the operation to get module and action
	parts := strings.Split(operation, "/")
	if len(parts) < 2 {
		return true // Allow operations without clear module/action structure
	}

	module := parts[0]

	// Check if the module is enabled
	moduleEnabled := false
	for moduleKey, isEnabled := range permissions.EnabledModules {
		if isEnabled && strings.EqualFold(module, moduleKey) {
			moduleEnabled = true
			break
		}
	}

	if !moduleEnabled {
		// Special cases: some operations use different module names
		// Map operation prefixes to module names
		moduleMap := map[string]string{
			"pipelines":       "ci",
			"executions":      "ci",
			"projects":        "core",
			"orgs":            "core",
			"connectors":      "core",
			"secrets":         "core",
			"delegates":       "core",
			"templates":       "ci",
			"triggers":        "ci",
			"services":        "cd",
			"environments":    "cd",
			"infrastructures": "cd",
			"manifests":       "cd",
			"gitops":          "gitops",
			"featureflags":    "cf",
			"ccm":             "ccm",
			"cv":              "cv",
			"chaos":           "chaos",
			"sto":             "sto",
			"idp":             "idp",
			"iacm":            "iacm",
			"ssca":            "ssca",
		}

		if mappedModule, ok := moduleMap[module]; ok {
			// Check if this module is enabled in permissions
			if isEnabled, hasModule := permissions.EnabledModules[mappedModule]; hasModule && isEnabled {
				moduleEnabled = true
			}
		}
	}

	return moduleEnabled
}

// handleShutdown handles shutdown requests
func (h *Handler) handleShutdown(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	// Clean up session
	h.sessionsMu.Lock()
	if session, exists := h.sessions[sessionID]; exists {
		// If connected to Core Platform, close the linked session
		if h.coreClient != nil && session.CoreSession != "" {
			_ = h.coreClient.CloseSession(context.Background(), session.CoreSession)
		}
	}
	delete(h.sessions, sessionID)
	h.sessionsMu.Unlock()

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  map[string]interface{}{},
	}, nil
}

// handleToolsList handles tools/list requests
func (h *Handler) handleToolsList(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	// Get all tools from registry
	allTools := h.tools.ListAll()

	// Get session to check for Harness permissions
	h.sessionsMu.RLock()
	session := h.sessions[sessionID]
	h.sessionsMu.RUnlock()

	// Filter tools based on session permissions
	filteredTools := h.filterToolsByPermissions(allTools, session)

	toolList := make([]map[string]interface{}, 0, len(filteredTools))
	for _, tool := range filteredTools {
		toolList = append(toolList, map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		})
	}

	h.logger.Info("Listed tools for session", map[string]interface{}{
		"session_id":     sessionID,
		"total_tools":    len(allTools),
		"filtered_tools": len(filteredTools),
	})

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"tools": toolList,
		},
	}, nil
}

// handleToolCall handles tools/call requests
func (h *Handler) handleToolCall(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil, NewValidationError("params", fmt.Sprintf("Invalid tool call parameters: %v", err)).
			WithOperation("tools/call").
			WithRequestID(fmt.Sprintf("%v", msg.ID))
	}

	// Validate tool name format
	if err := h.validator.ValidateToolName(params.Name); err != nil {
		// Log validation failure
		h.validator.LogValidationFailure(context.Background(), err, map[string]interface{}{
			"method":     "tools/call",
			"tool":       params.Name,
			"session":    sessionID,
			"message_id": msg.ID,
		})
		// Convert to error response
		errResp := h.validator.ToErrorResponse(err, "tools/call")
		return nil, errResp
	}

	// CRITICAL: Handle context operations specially for sync with Core Platform and semantic operations
	if params.Name == "context.update" || params.Name == "context.append" || params.Name == "context.get" ||
		params.Name == "context.compact" || params.Name == "context.search" {
		return h.handleContextOperation(sessionID, msg.ID, params.Name, params.Arguments)
	}

	// Create cancellable context for tool execution
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure cancel is always called to prevent context leak

	// Generate request ID for this tool execution and add to context
	requestID := observability.GenerateRequestID()
	ctx = observability.WithRequestID(ctx, requestID)

	// Add session tracking to context
	ctx = observability.WithSessionID(ctx, sessionID)
	ctx = observability.WithOperation(ctx, fmt.Sprintf("tools/call:%s", params.Name))

	// Add passthrough auth to context if available
	h.sessionsMu.RLock()
	session := h.sessions[sessionID]
	var passthroughAuth *models.PassthroughAuthBundle
	var tenantID string
	if session != nil {
		tenantID = session.TenantID
		if tenantID != "" {
			ctx = observability.WithTenantID(ctx, tenantID)
		}
		if session.PassthroughAuth != nil {
			passthroughAuth = session.PassthroughAuth
			// Add passthrough auth to context for remote tool execution
			ctx = context.WithValue(ctx, core.PassthroughAuthKey, passthroughAuth)
		}
	}
	h.sessionsMu.RUnlock()

	// Create request-scoped logger with context fields
	reqLogger := observability.LoggerFromContext(ctx, h.logger)

	// Start distributed tracing span for tool execution
	var span trace.Span
	if h.spanHelper != nil {
		ctx, span = h.spanHelper.StartToolExecutionSpan(ctx, params.Name, sessionID, tenantID)
		defer span.End()
	}

	// Track the request for potential cancellation (only if ID is present)
	if msg.ID != nil {
		h.trackRequest(msg.ID, cancel)
		defer h.untrackRequest(msg.ID)
	}

	// Tool execution audit log - START
	startTime := time.Now()
	reqLogger.Info("Tool execution started", map[string]interface{}{
		"tool":       params.Name,
		"session_id": sessionID,
	})

	// Execute tool with cancellable context (includes passthrough auth if available)
	result, err := h.tools.Execute(ctx, params.Name, params.Arguments)

	// Tool execution audit log - END
	duration := time.Since(startTime)
	if err != nil {
		// Record error in span
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		reqLogger.Error("Tool execution failed", map[string]interface{}{
			"tool":        params.Name,
			"session_id":  sessionID,
			"duration_ms": duration.Milliseconds(),
			"error":       err.Error(),
		})
	} else {
		// Set success status on span
		if span != nil {
			span.SetStatus(codes.Ok, "Tool execution completed successfully")
		}

		reqLogger.Info("Tool execution completed", map[string]interface{}{
			"tool":        params.Name,
			"session_id":  sessionID,
			"duration_ms": duration.Milliseconds(),
		})
	}
	if err != nil {
		// Check for special error types and enhance with AI-friendly information
		var toolNotFoundErr *tools.ToolNotFoundError
		var toolConfigErr *tools.ToolConfigError

		if errors.As(err, &toolNotFoundErr) {
			// Get available categories for suggestions
			allTools := h.tools.ListAll()
			categories := make([]string, 0)
			categoryMap := make(map[string]bool)
			for _, t := range allTools {
				if t.Category != "" && !categoryMap[t.Category] {
					categoryMap[t.Category] = true
					categories = append(categories, t.Category)
				}
			}

			return nil, errorTemplates.ToolNotFound(params.Name, categories).
				WithOperation(fmt.Sprintf("tools/call:%s", params.Name)).
				WithRequestID(fmt.Sprintf("%v", msg.ID)).
				WithMetadata("available_tools_count", len(allTools))

		} else if errors.As(err, &toolConfigErr) {
			return nil, errorTemplates.InternalError(
				fmt.Sprintf("tools/call:%s", params.Name),
				err,
			).WithRequestID(fmt.Sprintf("%v", msg.ID))
		}

		// Default tool execution error with possible alternatives
		var alternatives []string
		if tool, exists := h.tools.Get(params.Name); exists {
			alternatives = tool.Alternatives
		}

		return nil, NewToolExecutionErrorWithAlternatives(params.Name, err, alternatives).
			WithRequestID(fmt.Sprintf("%v", msg.ID))
	}

	// Record execution with Core Platform if connected
	if h.coreClient != nil {
		coreSessionID := ""
		if session != nil {
			coreSessionID = session.CoreSession
		}

		if coreSessionID != "" {
			_ = h.coreClient.RecordToolExecution(
				context.Background(),
				coreSessionID,
				params.Name,
				params.Arguments,
				result,
			)
		}
	}

	// Format result as MCP content
	content := []map[string]interface{}{
		{
			"type": "text",
			"text": fmt.Sprintf("%v", result),
		},
	}

	// If result is already structured, use it directly
	if resultMap, ok := result.(map[string]interface{}); ok {
		if resultContent, ok := resultMap["content"]; ok {
			content = resultContent.([]map[string]interface{})
		}
	}

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"content": content,
		},
	}, nil
}

// handleBatchToolCall handles tools/batch requests
func (h *Handler) handleBatchToolCall(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	var batchRequest BatchRequest

	if err := json.Unmarshal(msg.Params, &batchRequest); err != nil {
		return nil, NewValidationError("params", fmt.Sprintf("Invalid batch tool call parameters: %v", err)).
			WithOperation("tools/batch").
			WithRequestID(fmt.Sprintf("%v", msg.ID))
	}

	// Validate batch request
	if err := h.batchExecutor.ValidateBatchRequest(&batchRequest); err != nil {
		return nil, NewValidationError("batch", err.Error()).
			WithOperation("tools/batch").
			WithRequestID(fmt.Sprintf("%v", msg.ID))
	}

	// Create cancellable context for batch execution
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure cancel is always called to prevent context leak

	// Generate request ID for this batch execution and add to context
	requestID := observability.GenerateRequestID()
	ctx = observability.WithRequestID(ctx, requestID)

	// Add session tracking to context
	ctx = observability.WithSessionID(ctx, sessionID)
	ctx = observability.WithOperation(ctx, "tools/batch")

	// Add passthrough auth to context if available
	h.sessionsMu.RLock()
	session := h.sessions[sessionID]
	var tenantID string
	if session != nil {
		tenantID = session.TenantID
		if tenantID != "" {
			ctx = observability.WithTenantID(ctx, tenantID)
		}
		if session.PassthroughAuth != nil {
			// Add passthrough auth to context for remote tool execution
			ctx = context.WithValue(ctx, core.PassthroughAuthKey, session.PassthroughAuth)
		}
	}
	h.sessionsMu.RUnlock()

	// Create request-scoped logger with context fields
	reqLogger := observability.LoggerFromContext(ctx, h.logger)

	// Start distributed tracing span for batch execution
	var span trace.Span
	if h.spanHelper != nil {
		ctx, span = h.spanHelper.StartToolExecutionSpan(ctx, "batch", sessionID, tenantID)
		defer span.End()
	}

	// Track the request for potential cancellation (only if ID is present)
	if msg.ID != nil {
		h.trackRequest(msg.ID, cancel)
		defer h.untrackRequest(msg.ID)
	}

	// Batch execution audit log - START
	startTime := time.Now()
	reqLogger.Info("Batch execution started", map[string]interface{}{
		"batch_size": len(batchRequest.Tools),
		"session_id": sessionID,
		"parallel":   batchRequest.Parallel,
	})

	// Execute batch
	batchResponse, err := h.batchExecutor.Execute(ctx, &batchRequest)

	// Batch execution audit log - END
	duration := time.Since(startTime)
	if err != nil {
		// Record error in span
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		reqLogger.Error("Batch execution failed", map[string]interface{}{
			"batch_size":  len(batchRequest.Tools),
			"session_id":  sessionID,
			"duration_ms": duration.Milliseconds(),
			"error":       err.Error(),
		})

		return nil, errorTemplates.InternalError("tools/batch", err).
			WithRequestID(fmt.Sprintf("%v", msg.ID)).
			WithMetadata("batch_size", len(batchRequest.Tools))
	}

	// Set success status on span
	if span != nil {
		span.SetStatus(codes.Ok, "Batch execution completed")
	}

	reqLogger.Info("Batch execution completed", map[string]interface{}{
		"batch_size":    len(batchRequest.Tools),
		"session_id":    sessionID,
		"duration_ms":   duration.Milliseconds(),
		"success_count": batchResponse.SuccessCount,
		"error_count":   batchResponse.ErrorCount,
		"parallel":      batchResponse.Parallel,
	})

	// Record execution with Core Platform if connected
	if h.coreClient != nil {
		coreSessionID := ""
		if session != nil {
			coreSessionID = session.CoreSession
		}

		if coreSessionID != "" {
			// Record batch execution summary
			batchSummary := map[string]interface{}{
				"type":          "batch",
				"tools":         batchRequest.Tools,
				"success_count": batchResponse.SuccessCount,
				"error_count":   batchResponse.ErrorCount,
				"duration_ms":   batchResponse.TotalDuration.Milliseconds(),
			}
			_ = h.coreClient.RecordToolExecution(
				context.Background(),
				coreSessionID,
				"tools/batch",
				nil,
				batchSummary,
			)
		}
	}

	// Format batch response as MCP content
	// Convert results to a more readable format
	contentText := fmt.Sprintf("Batch execution completed: %d/%d tools succeeded",
		batchResponse.SuccessCount, len(batchRequest.Tools))

	content := []map[string]interface{}{
		{
			"type": "text",
			"text": contentText,
		},
	}

	// Return structured batch response
	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"content":        content,
			"batch_results":  batchResponse.Results,
			"success_count":  batchResponse.SuccessCount,
			"error_count":    batchResponse.ErrorCount,
			"total_duration": batchResponse.TotalDuration.Milliseconds(),
			"parallel":       batchResponse.Parallel,
		},
	}, nil
}

// handleContextOperation handles context operations with semantic awareness
// Story 6.1: Integrates semantic context manager for enhanced context operations
func (h *Handler) handleContextOperation(sessionID string, msgID interface{}, operation string, args json.RawMessage) (*MCPMessage, error) {
	h.sessionsMu.RLock()
	session := h.sessions[sessionID]
	coreContextID := ""
	if session != nil {
		coreContextID = session.CoreSession
	}
	h.sessionsMu.RUnlock()

	var result interface{}
	var err error

	// Use semantic context manager if available (preferred)
	if semanticMgr, ok := h.semanticContextMgr.(SemanticContextManager); ok && semanticMgr != nil {
		h.logger.Debug("Using semantic context manager", map[string]interface{}{
			"operation":  operation,
			"session_id": sessionID,
		})

		switch operation {
		case "context.update":
			var updateParams struct {
				Content         string                 `json:"content"`
				Role            string                 `json:"role,omitempty"`
				ImportanceScore float64                `json:"importance_score,omitempty"`
				Metadata        map[string]interface{} `json:"metadata,omitempty"`
			}
			if err := json.Unmarshal(args, &updateParams); err != nil {
				return nil, NewValidationError("arguments", fmt.Sprintf("Invalid context update data: %v", err)).
					WithOperation("context.update")
			}

			// Set default role if not provided
			if updateParams.Role == "" {
				updateParams.Role = "user"
			}

			// Add importance score to metadata if provided
			if updateParams.Metadata == nil {
				updateParams.Metadata = make(map[string]interface{})
			}
			if updateParams.ImportanceScore > 0 {
				updateParams.Metadata["importance_score"] = updateParams.ImportanceScore
			}

			// Create update request - using map for interface compatibility
			update := map[string]interface{}{
				"role":     updateParams.Role,
				"content":  updateParams.Content,
				"metadata": updateParams.Metadata,
			}

			err = semanticMgr.UpdateContext(context.Background(), coreContextID, update)
			if err == nil {
				result = map[string]interface{}{
					"success":           true,
					"embedding_enabled": true,
					"context_id":        coreContextID,
				}
			}

		case "context.get":
			var getParams struct {
				RelevanceQuery string `json:"relevance_query,omitempty"`
				MaxTokens      int    `json:"max_tokens,omitempty"`
			}
			if err := json.Unmarshal(args, &getParams); err != nil {
				// If no params, just retrieve normally
				getParams = struct {
					RelevanceQuery string `json:"relevance_query,omitempty"`
					MaxTokens      int    `json:"max_tokens,omitempty"`
				}{}
			}

			// Use semantic retrieval if query provided
			if getParams.RelevanceQuery != "" {
				h.logger.Info("Semantic context retrieval requested", map[string]interface{}{
					"query":      getParams.RelevanceQuery,
					"max_tokens": getParams.MaxTokens,
				})
				result, err = semanticMgr.GetRelevantContext(
					context.Background(),
					coreContextID,
					getParams.RelevanceQuery,
					getParams.MaxTokens,
				)
			} else {
				// Standard retrieval with optional options
				opts := map[string]interface{}{}
				if getParams.MaxTokens > 0 {
					opts["max_tokens"] = getParams.MaxTokens
				}
				result, err = semanticMgr.GetContext(context.Background(), coreContextID, opts)
			}

		case "context.compact":
			var compactParams struct {
				Strategy string `json:"strategy"` // summarize, prune, semantic, sliding, tool_clear
			}
			if err := json.Unmarshal(args, &compactParams); err != nil {
				return nil, NewValidationError("arguments", fmt.Sprintf("Invalid compact parameters: %v", err)).
					WithOperation("context.compact")
			}

			// Default to summarize strategy
			if compactParams.Strategy == "" {
				compactParams.Strategy = "summarize"
			}

			h.logger.Info("Context compaction requested", map[string]interface{}{
				"strategy":   compactParams.Strategy,
				"context_id": coreContextID,
			})

			err = semanticMgr.CompactContext(context.Background(), coreContextID, compactParams.Strategy)
			if err == nil {
				result = map[string]interface{}{
					"success":  true,
					"strategy": compactParams.Strategy,
				}
			}

		case "context.search":
			var searchParams struct {
				Query string `json:"query"`
				Limit int    `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(args, &searchParams); err != nil {
				return nil, NewValidationError("arguments", fmt.Sprintf("Invalid search parameters: %v", err)).
					WithOperation("context.search")
			}

			// Default limit
			if searchParams.Limit == 0 {
				searchParams.Limit = 10
			}

			h.logger.Info("Semantic context search requested", map[string]interface{}{
				"query": searchParams.Query,
				"limit": searchParams.Limit,
			})

			result, err = semanticMgr.SearchContext(
				context.Background(),
				searchParams.Query,
				coreContextID,
				searchParams.Limit,
			)

		case "context.append":
			// Append is similar to update but maintains separate items
			var appendParams struct {
				Content         string                 `json:"content"`
				Role            string                 `json:"role,omitempty"`
				ImportanceScore float64                `json:"importance_score,omitempty"`
				Metadata        map[string]interface{} `json:"metadata,omitempty"`
			}
			if err := json.Unmarshal(args, &appendParams); err != nil {
				return nil, NewValidationError("arguments", fmt.Sprintf("Invalid append data: %v", err)).
					WithOperation("context.append")
			}

			// Set default role
			if appendParams.Role == "" {
				appendParams.Role = "assistant"
			}

			// Add importance score to metadata
			if appendParams.Metadata == nil {
				appendParams.Metadata = make(map[string]interface{})
			}
			if appendParams.ImportanceScore > 0 {
				appendParams.Metadata["importance_score"] = appendParams.ImportanceScore
			}

			// Use update with append flag in metadata
			appendParams.Metadata["append"] = true

			update := map[string]interface{}{
				"role":     appendParams.Role,
				"content":  appendParams.Content,
				"metadata": appendParams.Metadata,
			}

			err = semanticMgr.UpdateContext(context.Background(), coreContextID, update)
			if err == nil {
				result = map[string]interface{}{
					"success":  true,
					"appended": true,
				}
			}

		default:
			return nil, NewProtocolError(operation, "Unsupported operation",
				fmt.Sprintf("Operation '%s' is not supported", operation))
		}

	} else if h.coreClient != nil {
		// Fall back to legacy Core Platform client
		h.logger.Debug("Using legacy Core Platform client", map[string]interface{}{
			"operation":  operation,
			"session_id": sessionID,
		})

		if coreContextID == "" {
			return nil, errorTemplates.UninitializedSession().
				WithDetails("Core Platform session must be established before context operations")
		}

		switch operation {
		case "context.update":
			var contextUpdate map[string]interface{}
			if err := json.Unmarshal(args, &contextUpdate); err != nil {
				return nil, NewValidationError("arguments", fmt.Sprintf("Invalid context update data: %v", err)).
					WithOperation("context.update")
			}

			err = h.coreClient.UpdateContext(context.Background(), coreContextID, contextUpdate)
			if err == nil {
				// Cache locally for performance
				_ = h.cache.Set(context.Background(), fmt.Sprintf("context:%s", sessionID), contextUpdate, 5*time.Minute)
				result = map[string]interface{}{"success": true}
			}

		case "context.get":
			// Try cache first
			var cached map[string]interface{}
			if err := h.cache.Get(context.Background(), fmt.Sprintf("context:%s", sessionID), &cached); err == nil {
				result = cached
			} else {
				// Fetch from Core Platform
				result, err = h.coreClient.GetContext(context.Background(), coreContextID)
				if err == nil {
					// Cache the result
					_ = h.cache.Set(context.Background(), fmt.Sprintf("context:%s", sessionID), result, 5*time.Minute)
				}
			}

		case "context.append":
			var appendData map[string]interface{}
			if err := json.Unmarshal(args, &appendData); err != nil {
				return nil, NewValidationError("arguments", fmt.Sprintf("Invalid append data: %v", err)).
					WithOperation("context.append")
			}

			err = h.coreClient.AppendContext(context.Background(), coreContextID, appendData)
			if err == nil {
				result = map[string]interface{}{"success": true}
			}

		case "context.search":
			// Parse search parameters
			var searchParams struct {
				Query string `json:"query"`
				Limit int    `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(args, &searchParams); err != nil {
				return nil, NewValidationError("arguments", fmt.Sprintf("Invalid search parameters: %v", err)).
					WithOperation("context.search")
			}

			// Default limit
			if searchParams.Limit == 0 {
				searchParams.Limit = 10
			}

			h.logger.Info("Semantic context search via Core Platform", map[string]interface{}{
				"query":      searchParams.Query,
				"limit":      searchParams.Limit,
				"context_id": coreContextID,
			})

			// Call Core Platform's search endpoint
			result, err = h.coreClient.SearchContext(context.Background(), coreContextID, searchParams.Query, searchParams.Limit)

		case "context.compact":
			return nil, NewProtocolError(operation, "Operation not supported",
				fmt.Sprintf("Operation '%s' requires semantic context manager", operation))

		default:
			return nil, NewProtocolError(operation, "Unsupported operation",
				fmt.Sprintf("Operation '%s' is not supported", operation))
		}
	} else {
		// Neither semantic manager nor core client available
		return nil, NewProtocolError(operation, "Context backend not available",
			"Context operations require either semantic context manager or Core Platform connection")
	}

	if err != nil {
		return nil, errorTemplates.InternalError(operation, err).
			WithSuggestion("Retry the operation or check connectivity")
	}

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msgID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("%v", result),
				},
			},
		},
	}, nil
}

// handleResourcesList handles resources/list requests
func (h *Handler) handleResourcesList(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	resources := []map[string]interface{}{
		{
			"uri":         "edge://system/info",
			"name":        "System Information",
			"description": "Edge MCP system information",
			"mimeType":    "application/json",
		},
		{
			"uri":         "edge://platform/info",
			"name":        "Platform Information",
			"description": "Operating system and platform capabilities",
			"mimeType":    "application/json",
		},
		{
			"uri":         "edge://tools/list",
			"name":        "Available Tools",
			"description": "List of available tools",
			"mimeType":    "application/json",
		},
	}

	// Add Core Platform resources if connected
	if h.coreClient != nil {
		resources = append(resources, map[string]interface{}{
			"uri":         "core://connection/status",
			"name":        "Core Connection Status",
			"description": "Status of Core Platform connection",
			"mimeType":    "application/json",
		})
	}

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"resources": resources,
		},
	}, nil
}

// handleResourceRead handles resources/read requests
func (h *Handler) handleResourceRead(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	var params struct {
		URI string `json:"uri"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil, NewValidationError("params", fmt.Sprintf("Invalid resource read parameters: %v", err)).
			WithOperation("resources/read")
	}

	var content interface{}

	switch params.URI {
	case "edge://system/info":
		content = map[string]interface{}{
			"version":        "1.0.0",
			"core_connected": h.coreClient != nil,
			"tools_count":    h.tools.Count(),
			"cache_size":     h.cache.Size(),
		}

	case "edge://platform/info":
		content = platform.GetInfo()

	case "edge://tools/list":
		tools := h.tools.ListAll()
		toolNames := make([]string, 0, len(tools))
		for _, tool := range tools {
			toolNames = append(toolNames, tool.Name)
		}
		content = toolNames

	case "core://connection/status":
		if h.coreClient != nil {
			content = h.coreClient.GetStatus()
		} else {
			content = map[string]interface{}{
				"connected": false,
				"error":     "Core Platform not configured",
			}
		}

	default:
		return nil, errorTemplates.ResourceNotFound("resource", params.URI, []string{"resources/list"}).
			WithOperation("resources/read")
	}

	contentJSON, _ := json.Marshal(content)

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"uri":      params.URI,
					"mimeType": "application/json",
					"text":     string(contentJSON),
				},
			},
		},
	}, nil
}

// handlePromptsList handles prompts/list requests
func (h *Handler) handlePromptsList(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	// Edge MCP doesn't provide prompts
	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"prompts": []interface{}{},
		},
	}, nil
}

// handleLoggingSetLevel handles logging/setLevel requests
func (h *Handler) handleLoggingSetLevel(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	var params struct {
		Level string `json:"level"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil, NewValidationError("params", fmt.Sprintf("Invalid logging parameters: %v", err)).
			WithOperation("logging/setLevel")
	}

	// Map MCP log levels to observability log levels
	levelMap := map[string]observability.LogLevel{
		"debug":   observability.LogLevelDebug,
		"info":    observability.LogLevelInfo,
		"warning": observability.LogLevelWarn,
		"warn":    observability.LogLevelWarn,
		"error":   observability.LogLevelError,
	}

	newLevel, ok := levelMap[params.Level]
	if !ok {
		validLevels := []string{"debug", "info", "warning", "warn", "error"}
		return nil, NewValidationError("level", fmt.Sprintf("Invalid log level '%s'. Valid levels: %v", params.Level, validLevels)).
			WithOperation("logging/setLevel")
	}

	// Create a new logger with the specified level if StandardLogger
	if stdLogger, ok := h.logger.(*observability.StandardLogger); ok {
		h.logger = stdLogger.WithLevel(newLevel)
		h.logger.Info("Log level changed", map[string]interface{}{
			"new_level": params.Level,
		})
	}

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  map[string]interface{}{},
	}, nil
}

// handleCancelRequest handles $/cancelRequest requests
func (h *Handler) handleCancelRequest(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
	var params struct {
		ID interface{} `json:"id"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil, NewValidationError("params", fmt.Sprintf("Invalid cancel request parameters: %v", err)).
			WithOperation("$/cancelRequest")
	}

	// Look up and cancel the request
	h.requestsMu.Lock()
	cancel, exists := h.activeRequests[params.ID]
	if exists {
		delete(h.activeRequests, params.ID)
	}
	h.requestsMu.Unlock()

	if exists {
		// Cancel the request context
		cancel()
		h.logger.Info("Request cancelled", map[string]interface{}{
			"request_id": params.ID,
			"session_id": sessionID,
		})
	} else {
		h.logger.Warn("Request not found for cancellation", map[string]interface{}{
			"request_id": params.ID,
			"session_id": sessionID,
		})
	}

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  map[string]interface{}{},
	}, nil
}

// trackRequest registers a request for potential cancellation
func (h *Handler) trackRequest(id interface{}, cancel context.CancelFunc) {
	h.requestsMu.Lock()
	h.activeRequests[id] = cancel
	h.requestsMu.Unlock()
}

// untrackRequest removes a request from tracking
func (h *Handler) untrackRequest(id interface{}) {
	h.requestsMu.Lock()
	delete(h.activeRequests, id)
	h.requestsMu.Unlock()
}

// extractPassthroughAuth extracts user-specific credentials from request headers and environment
func (h *Handler) extractPassthroughAuth(r *http.Request) *models.PassthroughAuthBundle {
	bundle := &models.PassthroughAuthBundle{
		Credentials:   make(map[string]*models.PassthroughCredential),
		CustomHeaders: make(map[string]string),
	}

	// Extract GitHub Personal Access Token (from header or environment)
	token := r.Header.Get("X-GitHub-Token")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
		if token == "" {
			token = os.Getenv("GITHUB_PAT") // Alternative env var
		}
	}
	if token != "" {
		bundle.Credentials["github"] = &models.PassthroughCredential{
			Type:  "bearer",
			Token: token,
		}
		h.logger.Debug("Found GitHub passthrough token", nil)
	}

	// Extract generic user token (can be used for any service)
	userToken := r.Header.Get("X-User-Token")
	if userToken == "" {
		userToken = os.Getenv("USER_TOKEN")
	}
	if userToken != "" {
		bundle.Credentials["*"] = &models.PassthroughCredential{
			Type:  "bearer",
			Token: userToken,
		}
		h.logger.Debug("Found generic user passthrough token", nil)
	}

	// Extract AWS credentials (from headers or environment)
	accessKey := r.Header.Get("X-AWS-Access-Key")
	if accessKey == "" {
		accessKey = os.Getenv("AWS_ACCESS_KEY_ID")
	}

	if accessKey != "" {
		awsCred := &models.PassthroughCredential{
			Type:       "aws_signature",
			Properties: make(map[string]string),
		}
		awsCred.Properties["access_key"] = accessKey

		secretKey := r.Header.Get("X-AWS-Secret-Key")
		if secretKey == "" {
			secretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
		}
		if secretKey != "" {
			awsCred.Properties["secret_key"] = secretKey
		}

		sessionToken := r.Header.Get("X-AWS-Session-Token")
		if sessionToken == "" {
			sessionToken = os.Getenv("AWS_SESSION_TOKEN")
		}
		if sessionToken != "" {
			awsCred.Properties["session_token"] = sessionToken
		}

		region := r.Header.Get("X-AWS-Region")
		if region == "" {
			region = os.Getenv("AWS_REGION")
			if region == "" {
				region = os.Getenv("AWS_DEFAULT_REGION")
			}
		}
		if region != "" {
			awsCred.Properties["region"] = region
		}

		bundle.Credentials["aws"] = awsCred
		h.logger.Debug("Found AWS passthrough credentials", nil)
	}

	// Extract service-specific tokens (pattern: X-Service-{ServiceName}-Token)
	for key, values := range r.Header {
		if strings.HasPrefix(key, "X-Service-") && strings.HasSuffix(key, "-Token") && len(values) > 0 {
			// Extract service name from header
			// e.g., "X-Service-Slack-Token" -> "slack"
			serviceName := strings.ToLower(
				strings.TrimSuffix(
					strings.TrimPrefix(key, "X-Service-"),
					"-Token",
				),
			)
			bundle.Credentials[serviceName] = &models.PassthroughCredential{
				Type:  "bearer",
				Token: values[0],
			}
			h.logger.Debug("Found service-specific passthrough token", map[string]interface{}{
				"service": serviceName,
			})
		}
	}

	// Extract custom headers for advanced use cases
	for key, values := range r.Header {
		if strings.HasPrefix(key, "X-Custom-Auth-") && len(values) > 0 {
			customKey := strings.TrimPrefix(key, "X-Custom-Auth-")
			bundle.CustomHeaders[customKey] = values[0]
		}
	}

	// Check for common service tokens in environment variables
	commonServices := map[string][]string{
		"slack":     {"SLACK_TOKEN", "SLACK_API_TOKEN"},
		"jira":      {"JIRA_TOKEN", "JIRA_API_TOKEN", "ATLASSIAN_TOKEN"},
		"gitlab":    {"GITLAB_TOKEN", "GITLAB_PAT"},
		"bitbucket": {"BITBUCKET_TOKEN", "BITBUCKET_APP_PASSWORD"},
		"discord":   {"DISCORD_TOKEN", "DISCORD_BOT_TOKEN"},
		"openai":    {"OPENAI_API_KEY", "OPENAI_TOKEN"},
		"anthropic": {"ANTHROPIC_API_KEY", "CLAUDE_API_KEY"},
	}

	for service, envVars := range commonServices {
		// Skip if we already have a credential for this service
		if _, exists := bundle.Credentials[service]; exists {
			continue
		}

		// Check each possible environment variable
		for _, envVar := range envVars {
			if token := os.Getenv(envVar); token != "" {
				bundle.Credentials[service] = &models.PassthroughCredential{
					Type:  "bearer",
					Token: token,
				}
				h.logger.Debug("Found service token from environment", map[string]interface{}{
					"service": service,
					"env_var": envVar,
				})
				break
			}
		}
	}

	// If no passthrough credentials were found, return nil
	if len(bundle.Credentials) == 0 && len(bundle.CustomHeaders) == 0 {
		return nil
	}

	h.logger.Info("Extracted passthrough authentication", map[string]interface{}{
		"credentials_count": len(bundle.Credentials),
		"custom_headers":    len(bundle.CustomHeaders),
	})

	return bundle
}
