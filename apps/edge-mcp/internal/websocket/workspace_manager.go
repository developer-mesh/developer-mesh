package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	ws "github.com/developer-mesh/developer-mesh/pkg/models/websocket"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
)

// WorkspaceManager manages collaborative workspaces
type WorkspaceManager struct {
	workspaces sync.Map // workspace ID -> Workspace
	members    sync.Map // agent ID -> []workspace IDs
	logger     observability.Logger
	metrics    observability.MetricsClient
	server     *Server // Reference to send broadcasts
}

// NewWorkspaceManager creates a new workspace manager
func NewWorkspaceManager(logger observability.Logger, metrics observability.MetricsClient, server *Server) *WorkspaceManager {
	return &WorkspaceManager{
		logger:  logger,
		metrics: metrics,
		server:  server,
	}
}

// WorkspaceConfig represents workspace creation config
type WorkspaceConfig struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"` // private, team, public
	OwnerID     string   `json:"owner_id"`
	TenantID    string   `json:"tenant_id"`
	Members     []string `json:"members"`
}

// Workspace represents a collaborative workspace
type Workspace struct {
	ID             string                      `json:"id"`
	Name           string                      `json:"name"`
	Description    string                      `json:"description"`
	Type           string                      `json:"type"`
	OwnerID        string                      `json:"owner_id"`
	TenantID       string                      `json:"tenant_id"`
	Members        map[string]*WorkspaceMember `json:"members"`
	State          map[string]interface{}      `json:"state"`
	StateVersion   int                         `json:"state_version"`
	StateUpdatedAt time.Time                   `json:"state_updated_at"`
	StateUpdatedBy string                      `json:"state_updated_by"`
	CreatedAt      time.Time                   `json:"created_at"`
	UpdatedAt      time.Time                   `json:"updated_at"`
}

// WorkspaceMember represents a workspace member
type WorkspaceMember struct {
	ID       string    `json:"id"`
	AgentID  string    `json:"agent_id"`
	Role     string    `json:"role"` // member, moderator, admin
	JoinedAt time.Time `json:"joined_at"`
}

// CreateWorkspace creates a new workspace
func (wm *WorkspaceManager) CreateWorkspace(ctx context.Context, config *WorkspaceConfig) (*Workspace, error) {
	workspace := &Workspace{
		ID:          uuid.New().String(),
		Name:        config.Name,
		Description: config.Description,
		Type:        config.Type,
		OwnerID:     config.OwnerID,
		TenantID:    config.TenantID,
		Members:     make(map[string]*WorkspaceMember),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Add owner as admin
	ownerMember := &WorkspaceMember{
		ID:       uuid.New().String(),
		AgentID:  config.OwnerID,
		Role:     "admin",
		JoinedAt: time.Now(),
	}
	workspace.Members[config.OwnerID] = ownerMember

	// Add initial members
	for _, memberID := range config.Members {
		member := &WorkspaceMember{
			ID:       uuid.New().String(),
			AgentID:  memberID,
			Role:     "member",
			JoinedAt: time.Now(),
		}
		workspace.Members[memberID] = member
		wm.addMemberToIndex(memberID, workspace.ID)
	}

	// Store workspace
	wm.workspaces.Store(workspace.ID, workspace)
	wm.addMemberToIndex(config.OwnerID, workspace.ID)

	wm.metrics.IncrementCounter("workspaces_created", 1)
	wm.logger.Info("Workspace created", map[string]interface{}{
		"workspace_id": workspace.ID,
		"name":         workspace.Name,
		"type":         workspace.Type,
		"owner":        config.OwnerID,
	})

	return workspace, nil
}

// JoinWorkspace adds an agent to a workspace
func (wm *WorkspaceManager) JoinWorkspace(ctx context.Context, workspaceID, agentID, role string) (*WorkspaceMember, error) {
	val, ok := wm.workspaces.Load(workspaceID)
	if !ok {
		return nil, fmt.Errorf("workspace not found: %s", workspaceID)
	}

	workspace := val.(*Workspace)

	// Check if already a member
	if _, exists := workspace.Members[agentID]; exists {
		return nil, fmt.Errorf("already a member of workspace")
	}

	// Create member
	member := &WorkspaceMember{
		ID:       uuid.New().String(),
		AgentID:  agentID,
		Role:     role,
		JoinedAt: time.Now(),
	}

	// Log current members before broadcast
	wm.logger.Debug("Current workspace members before join", map[string]interface{}{
		"workspace_id": workspaceID,
		"members": func() []string {
			ids := make([]string, 0, len(workspace.Members))
			for id := range workspace.Members {
				ids = append(ids, id)
			}
			return ids
		}(),
		"joining_agent": agentID,
	})

	// Add the new member first
	workspace.Members[agentID] = member
	workspace.UpdatedAt = time.Now()

	// Broadcast join event to all members except the one who just joined
	// Pass the joining agent ID in the data so broadcast can exclude them
	wm.broadcastEvent(workspaceID, "member_joined", map[string]interface{}{
		"agent_id":      agentID,
		"role":          role,
		"exclude_agent": agentID, // Production pattern: explicit exclusion
	})

	// Log members after adding
	wm.logger.Debug("Workspace members after join", map[string]interface{}{
		"workspace_id": workspaceID,
		"members": func() []string {
			ids := make([]string, 0, len(workspace.Members))
			for id := range workspace.Members {
				ids = append(ids, id)
			}
			return ids
		}(),
		"new_member": agentID,
	})

	// Update index
	wm.addMemberToIndex(agentID, workspaceID)

	wm.metrics.IncrementCounter("workspace_members_joined", 1)
	return member, nil
}

// LeaveWorkspace removes an agent from a workspace
func (wm *WorkspaceManager) LeaveWorkspace(ctx context.Context, workspaceID, agentID string) error {
	val, ok := wm.workspaces.Load(workspaceID)
	if !ok {
		return fmt.Errorf("workspace not found: %s", workspaceID)
	}

	workspace := val.(*Workspace)

	// Check if member exists
	if _, exists := workspace.Members[agentID]; !exists {
		return fmt.Errorf("not a member of workspace")
	}

	// Don't allow owner to leave
	if agentID == workspace.OwnerID {
		return fmt.Errorf("owner cannot leave workspace")
	}

	// Remove member
	delete(workspace.Members, agentID)
	workspace.UpdatedAt = time.Now()

	// Update index
	wm.removeMemberFromIndex(agentID, workspaceID)

	// Broadcast leave event
	wm.broadcastEvent(workspaceID, "member_left", map[string]interface{}{
		"agent_id": agentID,
	})

	wm.metrics.IncrementCounter("workspace_members_left", 1)
	return nil
}

// BroadcastToWorkspace sends a message to all workspace members
func (wm *WorkspaceManager) BroadcastToWorkspace(ctx context.Context, workspaceID, senderID, event string, data map[string]interface{}) ([]string, error) {
	val, ok := wm.workspaces.Load(workspaceID)
	if !ok {
		return nil, fmt.Errorf("workspace not found: %s", workspaceID)
	}

	workspace := val.(*Workspace)

	// Prepare broadcast message
	message := map[string]interface{}{
		"type":         "workspace_event",
		"workspace_id": workspaceID,
		"event":        event,
		"sender_id":    senderID,
		"data":         data,
		"timestamp":    time.Now().Format(time.RFC3339),
	}

	msgBytes, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}

	// Send to all members except sender
	var recipients []string
	for agentID := range workspace.Members {
		if agentID != senderID {
			recipients = append(recipients, agentID)
			wm.server.SendToAgent(agentID, msgBytes)
		}
	}

	wm.metrics.IncrementCounter("workspace_broadcasts", 1)
	return recipients, nil
}

// ListMembers lists all members of a workspace
func (wm *WorkspaceManager) ListMembers(ctx context.Context, workspaceID string) ([]map[string]interface{}, error) {
	val, ok := wm.workspaces.Load(workspaceID)
	if !ok {
		return nil, fmt.Errorf("workspace not found: %s", workspaceID)
	}

	workspace := val.(*Workspace)

	var members []map[string]interface{}
	for _, member := range workspace.Members {
		members = append(members, map[string]interface{}{
			"agent_id":  member.AgentID,
			"role":      member.Role,
			"joined_at": member.JoinedAt,
		})
	}

	return members, nil
}

// IsMember checks if an agent is a member of a workspace
func (wm *WorkspaceManager) IsMember(ctx context.Context, workspaceID, agentID string) (bool, error) {
	val, ok := wm.workspaces.Load(workspaceID)
	if !ok {
		return false, fmt.Errorf("workspace not found: %s", workspaceID)
	}

	workspace := val.(*Workspace)
	_, exists := workspace.Members[agentID]
	return exists, nil
}

// GetAgentWorkspaces returns all workspaces for an agent
func (wm *WorkspaceManager) GetAgentWorkspaces(agentID string) []string {
	val, ok := wm.members.Load(agentID)
	if !ok {
		return []string{}
	}

	return val.([]string)
}

// Helper methods

func (wm *WorkspaceManager) addMemberToIndex(agentID, workspaceID string) {
	val, ok := wm.members.Load(agentID)
	var workspaces []string
	if ok {
		workspaces = val.([]string)
	}

	// Add if not already present
	found := false
	for _, id := range workspaces {
		if id == workspaceID {
			found = true
			break
		}
	}

	if !found {
		workspaces = append(workspaces, workspaceID)
		wm.members.Store(agentID, workspaces)
	}
}

func (wm *WorkspaceManager) removeMemberFromIndex(agentID, workspaceID string) {
	val, ok := wm.members.Load(agentID)
	if !ok {
		return
	}

	workspaces := val.([]string)
	newWorkspaces := make([]string, 0, len(workspaces))

	for _, id := range workspaces {
		if id != workspaceID {
			newWorkspaces = append(newWorkspaces, id)
		}
	}

	if len(newWorkspaces) > 0 {
		wm.members.Store(agentID, newWorkspaces)
	} else {
		wm.members.Delete(agentID)
	}
}

func (wm *WorkspaceManager) broadcastEvent(workspaceID, event string, data map[string]interface{}) {
	// Get workspace to find all members
	val, ok := wm.workspaces.Load(workspaceID)
	if !ok {
		wm.logger.Warn("Cannot broadcast event: workspace not found", map[string]interface{}{
			"workspace_id": workspaceID,
			"event":        event,
		})
		return
	}

	workspace := val.(*Workspace)

	// Check if we should exclude a specific agent (production pattern)
	var excludeAgent string
	if exclude, ok := data["exclude_agent"].(string); ok {
		excludeAgent = exclude
		// Remove from notification data as it's internal only
		delete(data, "exclude_agent")
	}

	// Build notification message
	notification := map[string]interface{}{
		"workspace_id":   workspaceID,
		"workspace_name": workspace.Name,
		"event_type":     event,
		"timestamp":      time.Now().Format(time.RFC3339),
	}

	// Merge event data
	for k, v := range data {
		notification[k] = v
	}

	// Send notification to all members
	if wm.server != nil && wm.server.notificationManager != nil {
		// Map event names to expected notification methods
		method := ""
		switch event {
		case "member_joined":
			method = "workspace.member.joined"
		case "member_left":
			method = "workspace.member.left"
		case "state_updated":
			method = "workspace.state.updated"
		case "document_created":
			method = "document.created"
		default:
			method = "workspace." + event
		}

		// Send to all workspace members
		wm.logger.Debug("Broadcasting event to workspace members", map[string]interface{}{
			"workspace_id": workspaceID,
			"event":        event,
			"member_count": len(workspace.Members),
			"members": func() []string {
				ids := make([]string, 0, len(workspace.Members))
				for id := range workspace.Members {
					ids = append(ids, id)
				}
				return ids
			}(),
		})

		for agentID := range workspace.Members {
			// Skip if this is the excluded agent (production pattern)
			if excludeAgent != "" && agentID == excludeAgent {
				wm.logger.Debug("Skipping notification for excluded agent", map[string]interface{}{
					"workspace_id": workspaceID,
					"agent_id":     agentID,
					"event":        event,
				})
				continue
			}

			wm.logger.Debug("Sending notification to member", map[string]interface{}{
				"workspace_id": workspaceID,
				"agent_id":     agentID,
				"event":        event,
			})

			// Find connection for this agent
			wm.server.mu.RLock()
			found := false
			for _, conn := range wm.server.connections {
				if conn.AgentID == agentID {
					found = true
					// Send notification directly to connection
					msg := &ws.Message{
						Type:   ws.MessageTypeNotification,
						Method: method,
						Params: notification,
					}

					// Send via connection's send channel
					if err := conn.SendMessage(msg); err != nil {
						wm.logger.Warn("Failed to send workspace notification", map[string]interface{}{
							"agent_id": agentID,
							"error":    err.Error(),
						})
					} else {
						wm.logger.Debug("Sent workspace notification", map[string]interface{}{
							"agent_id":     agentID,
							"method":       method,
							"workspace_id": workspaceID,
						})
					}
				}
			}
			wm.server.mu.RUnlock()

			if !found {
				wm.logger.Warn("No connection found for workspace member", map[string]interface{}{
					"agent_id":          agentID,
					"workspace_id":      workspaceID,
					"total_connections": len(wm.server.connections),
				})
			}
		}
	}

	wm.logger.Info("Workspace event broadcast", map[string]interface{}{
		"workspace_id": workspaceID,
		"event":        event,
		"data":         data,
		"member_count": len(workspace.Members),
	})
}

// GetWorkspaceState retrieves the current state of a workspace
func (wm *WorkspaceManager) GetWorkspaceState(ctx context.Context, workspaceID string) (*WorkspaceState, error) {
	val, ok := wm.workspaces.Load(workspaceID)
	if !ok {
		return nil, fmt.Errorf("workspace not found: %s", workspaceID)
	}

	workspace := val.(*Workspace)

	return &WorkspaceState{
		Data:      workspace.State,
		Version:   workspace.StateVersion,
		UpdatedAt: workspace.StateUpdatedAt,
		UpdatedBy: workspace.StateUpdatedBy,
	}, nil
}

// UpdateWorkspaceState updates the workspace state with versioning
func (wm *WorkspaceManager) UpdateWorkspaceState(ctx context.Context, workspaceID, agentID string, state map[string]interface{}, version int) (*WorkspaceState, error) {
	val, ok := wm.workspaces.Load(workspaceID)
	if !ok {
		return nil, fmt.Errorf("workspace not found: %s", workspaceID)
	}

	workspace := val.(*Workspace)

	// Check if agent is a member
	if _, exists := workspace.Members[agentID]; !exists {
		return nil, fmt.Errorf("agent is not a member of workspace")
	}

	// Check version for optimistic concurrency control
	if version != 0 && version != workspace.StateVersion {
		return nil, fmt.Errorf("version mismatch: expected %d, got %d", workspace.StateVersion, version)
	}

	// Update state
	workspace.State = state
	workspace.StateVersion++
	workspace.StateUpdatedAt = time.Now()
	workspace.StateUpdatedBy = agentID

	wm.workspaces.Store(workspaceID, workspace)

	// Broadcast state update to all members
	wm.broadcastEvent(workspaceID, "state_updated", map[string]interface{}{
		"updated_by": agentID,
		"version":    workspace.StateVersion,
	})

	return &WorkspaceState{
		Data:      workspace.State,
		Version:   workspace.StateVersion,
		UpdatedAt: workspace.StateUpdatedAt,
		UpdatedBy: workspace.StateUpdatedBy,
	}, nil
}

// WorkspaceState represents the current state of a workspace
type WorkspaceState struct {
	Data      map[string]interface{} `json:"data"`
	Version   int                    `json:"version"`
	UpdatedAt time.Time              `json:"updated_at"`
	UpdatedBy string                 `json:"updated_by"`
}
