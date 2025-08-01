package interfaces

import (
	"context"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/repository/types"
	"github.com/google/uuid"
)

// WorkspaceFilters defines filtering options for workspace queries
type WorkspaceFilters struct {
	Type          []string
	Visibility    []string
	OwnerID       *string
	MemberID      *string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	IsActive      *bool
	Limit         int
	Offset        int
	SortBy        string
	SortOrder     types.SortOrder
}

// WorkspaceStats represents workspace usage statistics
type WorkspaceStats struct {
	TotalMembers     int64
	ActiveMembers    int64
	TotalDocuments   int64
	TotalOperations  int64
	StorageUsedBytes int64
	LastActivityAt   time.Time
}

// WorkspaceActivity represents activity metrics for a workspace
type WorkspaceActivity struct {
	WorkspaceID      uuid.UUID
	WorkspaceName    string
	TotalMembers     int
	ActiveMembers    int
	TotalTasks       int64
	CompletedTasks   int64
	ActiveDocuments  int
	LastActivityTime time.Time
	ActivitySummary  map[string]int64
}

// WorkspaceRepository defines the interface for workspace persistence
type WorkspaceRepository interface {
	// Transaction support
	WithTx(tx types.Transaction) WorkspaceRepository
	BeginTx(ctx context.Context, opts *types.TxOptions) (types.Transaction, error)

	// Basic CRUD operations
	Create(ctx context.Context, workspace *models.Workspace) error
	Get(ctx context.Context, id uuid.UUID) (*models.Workspace, error)
	Update(ctx context.Context, workspace *models.Workspace) error
	Delete(ctx context.Context, id uuid.UUID) error
	SoftDelete(ctx context.Context, id uuid.UUID) error

	// Query operations
	List(ctx context.Context, tenantID uuid.UUID, filters WorkspaceFilters) ([]*models.Workspace, error)
	ListByOwner(ctx context.Context, ownerID string) ([]*models.Workspace, error)
	ListByMember(ctx context.Context, memberID string) ([]*models.Workspace, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]*models.Workspace, error)
	ListByAgent(ctx context.Context, agentID string) ([]*models.Workspace, error)
	ListPublic(ctx context.Context) ([]*models.Workspace, error)
	Search(ctx context.Context, query string, filters WorkspaceFilters) ([]*models.Workspace, error)
	SearchWorkspaces(ctx context.Context, query string, filters WorkspaceFilters) ([]*models.Workspace, error)

	// Member operations
	AddMember(ctx context.Context, member *models.WorkspaceMember) error
	RemoveMember(ctx context.Context, workspaceID uuid.UUID, agentID string) error
	UpdateMemberRole(ctx context.Context, workspaceID uuid.UUID, agentID string, role string) error
	UpdateMemberPermissions(ctx context.Context, workspaceID uuid.UUID, agentID string, permissions []string) error
	GetMember(ctx context.Context, workspaceID uuid.UUID, agentID string) (*models.WorkspaceMember, error)
	ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]*models.WorkspaceMember, error)
	GetMemberActivity(ctx context.Context, workspaceID uuid.UUID) ([]*models.MemberActivity, error)
	UpdateMemberActivity(ctx context.Context, workspaceID uuid.UUID, agentID string) error

	// State management
	UpdateState(ctx context.Context, workspaceID uuid.UUID, state map[string]interface{}, version int64) error
	GetState(ctx context.Context, workspaceID uuid.UUID) (map[string]interface{}, int64, error)
	MergeState(ctx context.Context, workspaceID uuid.UUID, remoteState *models.WorkspaceState) error
	GetStateHistory(ctx context.Context, workspaceID uuid.UUID, limit int) ([]*models.StateSnapshot, error)
	RestoreState(ctx context.Context, workspaceID uuid.UUID, snapshotID uuid.UUID) error
	LockWorkspace(ctx context.Context, workspaceID uuid.UUID, agentID string, duration time.Duration) error
	UnlockWorkspace(ctx context.Context, workspaceID uuid.UUID, agentID string) error

	// Activity tracking
	RecordActivity(ctx context.Context, workspaceID uuid.UUID, activity *models.WorkspaceActivity) error
	GetRecentActivity(ctx context.Context, workspaceID uuid.UUID, limit int) ([]*models.WorkspaceActivity, error)

	// Analytics
	GetWorkspaceStats(ctx context.Context, workspaceID uuid.UUID) (*WorkspaceStats, error)
	GetStats(ctx context.Context, workspaceID uuid.UUID) (*models.WorkspaceStats, error)
	GetActiveWorkspaces(ctx context.Context, since time.Time) ([]*models.Workspace, error)
	GetCollaborationMetrics(ctx context.Context, workspaceID uuid.UUID, period time.Duration) (*models.CollaborationMetrics, error)

	// Maintenance
	PurgeInactiveWorkspaces(ctx context.Context, inactiveSince time.Duration) (int64, error)
	ValidateWorkspaceIntegrity(ctx context.Context, workspaceID uuid.UUID) error

	// Presence management
	GetPresence(ctx context.Context, workspaceID uuid.UUID) ([]*models.MemberPresence, error)
	UpdatePresence(ctx context.Context, workspaceID uuid.UUID, agentID string, status string) error
}
