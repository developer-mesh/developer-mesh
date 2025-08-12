package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// Prompt represents a reusable prompt template
type Prompt struct {
	ID          string           `json:"id" db:"id"`
	TenantID    string           `json:"tenant_id" db:"tenant_id"`
	Name        string           `json:"name" db:"name"`
	Description string           `json:"description" db:"description"`
	Arguments   []PromptArgument `json:"arguments" db:"arguments"`
	Template    string           `json:"template" db:"template"`
	Category    string           `json:"category,omitempty" db:"category"`
	Tags        []string         `json:"tags,omitempty" db:"tags"`
	Metadata    JSONMap          `json:"metadata,omitempty" db:"metadata"`
	CreatedAt   time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at" db:"updated_at"`
}

// PromptArgument represents an argument in a prompt template
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

// PromptListRequest represents parameters for listing prompts
type PromptListRequest struct {
	TenantID string   `json:"tenant_id"`
	Category string   `json:"category,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Limit    int      `json:"limit,omitempty"`
	Offset   int      `json:"offset,omitempty"`
}

// PromptCreateRequest represents a request to create a new prompt
type PromptCreateRequest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Arguments   []PromptArgument       `json:"arguments,omitempty"`
	Template    string                 `json:"template"`
	Category    string                 `json:"category,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// PromptUpdateRequest represents a request to update a prompt
type PromptUpdateRequest struct {
	Name        *string                `json:"name,omitempty"`
	Description *string                `json:"description,omitempty"`
	Arguments   []PromptArgument       `json:"arguments,omitempty"`
	Template    *string                `json:"template,omitempty"`
	Category    *string                `json:"category,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// PromptRenderRequest represents a request to render a prompt with arguments
type PromptRenderRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// PromptRenderResponse represents the response from rendering a prompt
type PromptRenderResponse struct {
	Messages []PromptMessage `json:"messages"`
}

// PromptMessage represents a message in the rendered prompt
type PromptMessage struct {
	Role    string `json:"role"` // "user", "assistant", "system"
	Content string `json:"content"`
}

// PromptArgumentList is a custom type for storing prompt arguments as JSON
type PromptArgumentList []PromptArgument

// Value implements the driver.Valuer interface
func (p PromptArgumentList) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	return json.Marshal(p)
}

// Scan implements the sql.Scanner interface
func (p *PromptArgumentList) Scan(value interface{}) error {
	if value == nil {
		*p = nil
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		data = []byte("[]")
	}

	return json.Unmarshal(data, p)
}
