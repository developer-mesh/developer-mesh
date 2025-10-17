package api

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CreateSourceRequest is the request body for creating a new data source
type CreateSourceRequest struct {
	SourceID    string            `json:"source_id" binding:"required,min=3,max=255"`
	SourceType  string            `json:"source_type" binding:"required,oneof=github_org github_repo gitlab confluence slack s3 jira notion"`
	Config      json.RawMessage   `json:"config" binding:"required"`
	Credentials map[string]string `json:"credentials" binding:"required"`
	Schedule    string            `json:"schedule,omitempty"` // Cron expression, optional
	Description string            `json:"description,omitempty"`
}

// UpdateSourceRequest is the request body for updating an existing source
type UpdateSourceRequest struct {
	Config      json.RawMessage   `json:"config,omitempty"`
	Credentials map[string]string `json:"credentials,omitempty"`
	Schedule    string            `json:"schedule,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
	Description string            `json:"description,omitempty"`
}

// SourceResponse is the response for a single source
type SourceResponse struct {
	ID          uuid.UUID       `json:"id"`
	SourceID    string          `json:"source_id"`
	SourceType  string          `json:"source_type"`
	Config      json.RawMessage `json:"config"`
	Schedule    string          `json:"schedule,omitempty"`
	Enabled     bool            `json:"enabled"`
	SyncStatus  string          `json:"sync_status"`
	LastSyncAt  *time.Time      `json:"last_sync_at,omitempty"`
	NextSyncAt  *time.Time      `json:"next_sync_at,omitempty"`
	Description string          `json:"description,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// SyncJobResponse is the response for a sync job
type SyncJobResponse struct {
	ID                 uuid.UUID  `json:"id"`
	SourceID           string     `json:"source_id"`
	JobType            string     `json:"job_type"`
	Status             string     `json:"status"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	DocumentsProcessed int        `json:"documents_processed"`
	DocumentsAdded     int        `json:"documents_added"`
	DocumentsUpdated   int        `json:"documents_updated"`
	DocumentsDeleted   int        `json:"documents_deleted"`
	ErrorsCount        int        `json:"errors_count"`
	ErrorMessage       string     `json:"error_message,omitempty"`
}

// Source-specific configuration structures

// GitHubOrgConfig defines configuration for GitHub organization source
type GitHubOrgConfig struct {
	Org             string   `json:"org" binding:"required"`
	BaseURL         string   `json:"base_url,omitempty"` // GitHub Enterprise URL (optional, defaults to github.com)
	IncludeArchived bool     `json:"include_archived"`
	IncludeForks    bool     `json:"include_forks"`
	IncludePatterns []string `json:"include_patterns"`
	ExcludePatterns []string `json:"exclude_patterns"`
	Repos           []string `json:"repos,omitempty"` // Optional: specific repos only
}

// GitHubRepoConfig defines configuration for single GitHub repository source
type GitHubRepoConfig struct {
	Owner           string   `json:"owner" binding:"required"`
	Repo            string   `json:"repo" binding:"required"`
	Branch          string   `json:"branch,omitempty"`
	BaseURL         string   `json:"base_url,omitempty"` // GitHub Enterprise URL (optional, defaults to github.com)
	IncludePatterns []string `json:"include_patterns"`
	ExcludePatterns []string `json:"exclude_patterns"`
}

// ConfluenceConfig defines configuration for Confluence source
type ConfluenceConfig struct {
	BaseURL  string   `json:"base_url" binding:"required,url"`
	SpaceKey string   `json:"space_key" binding:"required"`
	Labels   []string `json:"labels,omitempty"`
}

// SlackConfig defines configuration for Slack source
type SlackConfig struct {
	WorkspaceID string   `json:"workspace_id" binding:"required"`
	Channels    []string `json:"channels,omitempty"`  // Empty means all accessible channels
	DaysBack    int      `json:"days_back,omitempty"` // How many days of history to fetch
}

// S3Config defines configuration for S3 bucket source
type S3Config struct {
	Bucket          string   `json:"bucket" binding:"required"`
	Prefix          string   `json:"prefix,omitempty"`
	Region          string   `json:"region,omitempty"`
	IncludePatterns []string `json:"include_patterns"`
	ExcludePatterns []string `json:"exclude_patterns"`
}

// JiraConfig defines configuration for Jira source
type JiraConfig struct {
	BaseURL    string   `json:"base_url" binding:"required,url"`
	ProjectKey string   `json:"project_key" binding:"required"`
	IssueTypes []string `json:"issue_types,omitempty"`
	JQL        string   `json:"jql,omitempty"` // Custom JQL query
}

// NotionConfig defines configuration for Notion source
type NotionConfig struct {
	WorkspaceID string   `json:"workspace_id" binding:"required"`
	DatabaseIDs []string `json:"database_ids,omitempty"`
	PageIDs     []string `json:"page_ids,omitempty"`
}
