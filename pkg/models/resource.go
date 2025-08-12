package models

import (
	"time"
)

// Resource represents a resource that can be accessed by AI agents
type Resource struct {
	ID          string    `json:"id" db:"id"`
	TenantID    string    `json:"tenant_id" db:"tenant_id"`
	URI         string    `json:"uri" db:"uri"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description,omitempty" db:"description"`
	MimeType    string    `json:"mimeType" db:"mime_type"`
	Content     string    `json:"content,omitempty" db:"content"`
	Metadata    JSONMap   `json:"metadata,omitempty" db:"metadata"`
	Tags        []string  `json:"tags,omitempty" db:"tags"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// ResourceContent represents the content of a resource
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
	Content  string `json:"content"`
}

// ResourceListRequest represents parameters for listing resources
type ResourceListRequest struct {
	TenantID string                 `json:"tenant_id"`
	Filter   map[string]interface{} `json:"filter,omitempty"`
	Limit    int                    `json:"limit,omitempty"`
	Offset   int                    `json:"offset,omitempty"`
}

// ResourceCreateRequest represents a request to create a new resource
type ResourceCreateRequest struct {
	URI         string                 `json:"uri"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	MimeType    string                 `json:"mimeType"`
	Content     string                 `json:"content"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
}

// ResourceUpdateRequest represents a request to update a resource
type ResourceUpdateRequest struct {
	Name        *string                `json:"name,omitempty"`
	Description *string                `json:"description,omitempty"`
	MimeType    *string                `json:"mimeType,omitempty"`
	Content     *string                `json:"content,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
}

// ResourceSubscription represents a subscription to resource changes
type ResourceSubscription struct {
	ID         string    `json:"id" db:"id"`
	TenantID   string    `json:"tenant_id" db:"tenant_id"`
	ResourceID string    `json:"resource_id" db:"resource_id"`
	AgentID    string    `json:"agent_id" db:"agent_id"`
	Events     []string  `json:"events" db:"events"` // "created", "updated", "deleted"
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}
