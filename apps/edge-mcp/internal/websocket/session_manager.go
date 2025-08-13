package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
)

// ConversationSessionManager manages WebSocket conversation sessions
type ConversationSessionManager struct {
	sessions sync.Map // map[string]*Session
	cache    cache.Cache
	logger   observability.Logger
	metrics  observability.MetricsClient
}

// NewConversationSessionManager creates a new conversation session manager
func NewConversationSessionManager(cache cache.Cache, logger observability.Logger, metrics observability.MetricsClient) *ConversationSessionManager {
	return &ConversationSessionManager{
		cache:   cache,
		logger:  logger,
		metrics: metrics,
	}
}

// Session represents a conversation session
type Session struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	AgentID         string                 `json:"agent_id"`
	TenantID        string                 `json:"tenant_id"`
	AgentProfile    map[string]interface{} `json:"agent_profile"`
	State           map[string]interface{} `json:"state"`
	Messages        []SessionMessage       `json:"messages"`
	TokenCount      int                    `json:"token_count"`
	Persistent      bool                   `json:"persistent"`
	ParentSessionID string                 `json:"parent_session_id,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	ExpiresAt       time.Time              `json:"expires_at,omitempty"`
	Tags            []string               `json:"tags"`
	Metrics         *SessionMetrics        `json:"metrics,omitempty"`
}

// SessionMessage represents a message in a session
type SessionMessage struct {
	ID         string                 `json:"id"`
	Role       string                 `json:"role"`
	Content    string                 `json:"content"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	TokenCount int                    `json:"token_count"`
	Timestamp  time.Time              `json:"timestamp"`
}

// SessionConfig represents session creation configuration
type SessionConfig struct {
	ID             string
	Name           string
	AgentID        string
	TenantID       string
	AgentProfile   map[string]interface{}
	InitialContext map[string]interface{}
	State          map[string]interface{}
	Persistent     bool
	TTL            time.Duration
	TrackMetrics   bool
	Tags           []string
}

// SessionMetrics tracks session usage metrics
type SessionMetrics struct {
	Duration       time.Duration  `json:"duration"`
	OperationCount int            `json:"operation_count"`
	TokenUsage     int            `json:"token_usage"`
	ToolUsage      map[string]int `json:"tool_usage"`
	ErrorCount     int            `json:"error_count"`
	CreatedAt      time.Time      `json:"created_at"`
	LastActivity   time.Time      `json:"last_activity"`
}

// CreateSession creates a new session
func (sm *ConversationSessionManager) CreateSession(ctx context.Context, config *SessionConfig) (*Session, error) {
	session := &Session{
		ID:           config.ID,
		Name:         config.Name,
		AgentID:      config.AgentID,
		TenantID:     config.TenantID,
		AgentProfile: config.AgentProfile,
		State:        config.State,
		Messages:     []SessionMessage{},
		TokenCount:   0,
		Persistent:   config.Persistent,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Tags:         config.Tags,
	}

	if config.TTL > 0 {
		session.ExpiresAt = time.Now().Add(config.TTL)
	}

	if config.TrackMetrics {
		session.Metrics = &SessionMetrics{
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
			ToolUsage:    make(map[string]int),
		}
	}

	// Store in memory
	sm.sessions.Store(session.ID, session)

	// Store in cache if persistent
	if session.Persistent {
		if err := sm.persistSession(ctx, session); err != nil {
			sm.logger.Error("Failed to persist session", map[string]interface{}{
				"session_id": session.ID,
				"error":      err.Error(),
			})
		}
	}

	sm.metrics.IncrementCounter("sessions_created", 1)
	return session, nil
}

// GetSession retrieves a session
func (sm *ConversationSessionManager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	// Check memory first
	if val, ok := sm.sessions.Load(sessionID); ok {
		return val.(*Session), nil
	}

	// Check cache for persistent sessions
	session, err := sm.loadSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Store in memory for faster access
	sm.sessions.Store(sessionID, session)
	return session, nil
}

// UpdateSessionState updates the session state
func (sm *ConversationSessionManager) UpdateSessionState(ctx context.Context, sessionID string, state map[string]interface{}) (*Session, error) {
	session, err := sm.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	session.State = state
	session.UpdatedAt = time.Now()

	if session.Persistent {
		if err := sm.persistSession(ctx, session); err != nil {
			return nil, err
		}
	}

	return session, nil
}

// AddMessage adds a message to the session
func (sm *ConversationSessionManager) AddMessage(ctx context.Context, sessionID string, messageData map[string]interface{}) (*SessionMessage, error) {
	session, err := sm.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	message := SessionMessage{
		ID:        uuid.New().String(),
		Role:      messageData["role"].(string),
		Content:   messageData["content"].(string),
		Timestamp: time.Now(),
	}

	// Calculate token count (simplified - in production use proper tokenizer)
	message.TokenCount = len(message.Content) / 4

	session.Messages = append(session.Messages, message)
	session.TokenCount += message.TokenCount
	session.UpdatedAt = time.Now()

	if session.Metrics != nil {
		session.Metrics.LastActivity = time.Now()
		session.Metrics.TokenUsage += message.TokenCount
	}

	if session.Persistent {
		if err := sm.persistSession(ctx, session); err != nil {
			return nil, err
		}
	}

	return &message, nil
}

// GetMessages retrieves messages with pagination
func (s *Session) GetMessages(limit, offset int) []SessionMessage {
	if limit <= 0 {
		limit = len(s.Messages)
	}

	if offset >= len(s.Messages) {
		return []SessionMessage{}
	}

	end := offset + limit
	if end > len(s.Messages) {
		end = len(s.Messages)
	}

	return s.Messages[offset:end]
}

// BranchSession creates a new session branching from a parent
func (sm *ConversationSessionManager) BranchSession(ctx context.Context, parentID string, branchPoint int, branchName string) (*Session, error) {
	parent, err := sm.GetSession(ctx, parentID)
	if err != nil {
		return nil, err
	}

	branch := &Session{
		ID:              uuid.New().String(),
		Name:            branchName,
		AgentID:         parent.AgentID,
		TenantID:        parent.TenantID,
		AgentProfile:    parent.AgentProfile,
		State:           parent.State,
		Messages:        []SessionMessage{},
		TokenCount:      0,
		Persistent:      parent.Persistent,
		ParentSessionID: parentID,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		Tags:            append([]string{"branch"}, parent.Tags...),
	}

	// Copy messages up to branch point
	if branchPoint > 0 && branchPoint <= len(parent.Messages) {
		branch.Messages = make([]SessionMessage, branchPoint)
		copy(branch.Messages, parent.Messages[:branchPoint])

		// Calculate token count
		for _, msg := range branch.Messages {
			branch.TokenCount += msg.TokenCount
		}
	}

	// Store branch
	sm.sessions.Store(branch.ID, branch)

	if branch.Persistent {
		if err := sm.persistSession(ctx, branch); err != nil {
			return nil, err
		}
	}

	sm.metrics.IncrementCounter("sessions_branched", 1)
	return branch, nil
}

// RecoverSession recovers a persistent session
func (sm *ConversationSessionManager) RecoverSession(ctx context.Context, sessionID string) (*Session, error) {
	session, err := sm.loadSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to recover session: %w", err)
	}

	// Update last activity
	session.UpdatedAt = time.Now()
	if session.Metrics != nil {
		session.Metrics.LastActivity = time.Now()
	}

	// Store in memory
	sm.sessions.Store(sessionID, session)

	sm.metrics.IncrementCounter("sessions_recovered", 1)
	return session, nil
}

// ExportSession exports session data
func (sm *ConversationSessionManager) ExportSession(ctx context.Context, sessionID, format string, include []string) (map[string]interface{}, string, error) {
	session, err := sm.GetSession(ctx, sessionID)
	if err != nil {
		return nil, "", err
	}

	export := make(map[string]interface{})
	export["session_id"] = session.ID

	for _, item := range include {
		switch item {
		case "messages":
			export["messages"] = session.Messages
		case "state":
			export["state"] = session.State
		case "metadata":
			export["metadata"] = map[string]interface{}{
				"created_at":    session.CreatedAt,
				"last_activity": session.UpdatedAt,
				"message_count": len(session.Messages),
				"token_count":   session.TokenCount,
			}
		case "metrics":
			if session.Metrics != nil {
				export["metrics"] = session.Metrics
			}
		}
	}

	// Generate download URL (in production, upload to S3)
	downloadURL := fmt.Sprintf("/api/v1/sessions/%s/export.%s", sessionID, format)

	return map[string]interface{}{"export": export}, downloadURL, nil
}

// ListSessions lists sessions for an agent
func (sm *ConversationSessionManager) ListSessions(ctx context.Context, agentID string, filter map[string]interface{}, sortBy string, limit, offset int) ([]map[string]interface{}, int, error) {
	var sessions []map[string]interface{}
	total := 0

	sm.sessions.Range(func(key, value interface{}) bool {
		session := value.(*Session)
		if session.AgentID != agentID {
			return true
		}

		// Apply filters
		if filter != nil {
			if tags, ok := filter["tags"].([]string); ok {
				hasTag := false
				for _, tag := range tags {
					for _, sTag := range session.Tags {
						if tag == sTag {
							hasTag = true
							break
						}
					}
				}
				if !hasTag {
					return true
				}
			}
		}

		total++

		// Skip if before offset
		if total <= offset {
			return true
		}

		// Stop if limit reached
		if limit > 0 && len(sessions) >= limit {
			return false
		}

		sessionData := map[string]interface{}{
			"session_id":    session.ID,
			"name":          session.Name,
			"created_at":    session.CreatedAt,
			"updated_at":    session.UpdatedAt,
			"message_count": len(session.Messages),
			"token_count":   session.TokenCount,
			"tags":          session.Tags,
		}

		sessions = append(sessions, sessionData)
		return true
	})

	return sessions, total, nil
}

// GetSessionMetrics retrieves session metrics
func (sm *ConversationSessionManager) GetSessionMetrics(ctx context.Context, sessionID string) (*SessionMetrics, error) {
	session, err := sm.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if session.Metrics == nil {
		return nil, fmt.Errorf("metrics not tracked for session")
	}

	// Update duration
	session.Metrics.Duration = time.Since(session.Metrics.CreatedAt)

	return session.Metrics, nil
}

// IsExpired checks if the session is expired
func (s *Session) IsExpired() bool {
	if s.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(s.ExpiresAt)
}

// Helper methods for persistence

func (sm *ConversationSessionManager) persistSession(ctx context.Context, session *Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("session:%s", session.ID)
	return sm.cache.Set(ctx, key, data, 24*time.Hour)
}

func (sm *ConversationSessionManager) loadSession(ctx context.Context, sessionID string) (*Session, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	var session Session

	if err := sm.cache.Get(ctx, key, &session); err != nil {
		return nil, err
	}

	return &session, nil
}
