package github

import (
	"github.com/shurcooL/githubv4"
)

// PaginationParams holds pagination parameters for REST API
type PaginationParams struct {
	Page    int
	PerPage int
	After   string // For cursor pagination
}

// CursorPaginationParams holds cursor-based pagination parameters
type CursorPaginationParams struct {
	PerPage int
	After   string
}

// ParamDef defines a parameter
type ParamDef struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Min         int
	Max         int
	Default     interface{}
}

// WithPagination returns parameter definitions for pagination
func WithPagination() []ParamDef {
	return []ParamDef{
		{
			Name:        "page",
			Type:        "number",
			Description: "Page number to retrieve",
			Min:         1,
			Default:     1,
		},
		{
			Name:        "perPage",
			Type:        "number",
			Description: "Number of results per page",
			Min:         1,
			Max:         100,
			Default:     30,
		},
	}
}

// WithCursorPagination returns parameter definitions for cursor-based pagination
func WithCursorPagination() []ParamDef {
	return []ParamDef{
		{
			Name:        "after",
			Type:        "string",
			Description: "Cursor for pagination",
		},
		{
			Name:        "perPage",
			Type:        "number",
			Description: "Number of results per page",
			Min:         1,
			Max:         100,
			Default:     30,
		},
	}
}

// ExtractPagination extracts pagination parameters from a map
func ExtractPagination(params map[string]interface{}) PaginationParams {
	// Use extractInt for robust type conversion
	page := extractInt(params, "page")
	if page == 0 {
		page = 1 // Default to first page
	}

	// Check for both per_page (snake_case) and perPage (camelCase) for compatibility
	perPage := extractInt(params, "per_page")
	if perPage == 0 {
		// Fallback to camelCase for backwards compatibility
		perPage = extractInt(params, "perPage")
	}
	if perPage == 0 {
		perPage = 30 // Default
	} else if perPage > 100 {
		perPage = 100 // Max allowed by GitHub
	}

	after := extractString(params, "after")

	return PaginationParams{
		Page:    page,
		PerPage: perPage,
		After:   after,
	}
}

// ExtractCursorPagination extracts cursor pagination parameters
func ExtractCursorPagination(params map[string]interface{}) CursorPaginationParams {
	// Check for both per_page (snake_case) and perPage (camelCase) for compatibility
	perPage := extractInt(params, "per_page")
	if perPage == 0 {
		// Fallback to camelCase for backwards compatibility
		perPage = extractInt(params, "perPage")
	}
	if perPage == 0 {
		perPage = 30 // Default
	} else if perPage > 100 {
		perPage = 100 // Max allowed by GitHub
	}

	after := extractString(params, "after")

	return CursorPaginationParams{
		PerPage: perPage,
		After:   after,
	}
}

// ToGraphQLParams converts cursor pagination to GraphQL variables
func (p CursorPaginationParams) ToGraphQLParams() map[string]interface{} {
	vars := map[string]interface{}{
		"first": githubv4.Int(p.PerPage),
	}
	if p.After != "" {
		after := githubv4.String(p.After)
		vars["after"] = &after
	} else {
		vars["after"] = (*githubv4.String)(nil)
	}
	return vars
}

// PageInfo represents pagination information
type PageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
	TotalCount  int    `json:"totalCount,omitempty"`
}

// PaginatedResponse wraps a response with pagination info
type PaginatedResponse struct {
	Data     interface{} `json:"data"`
	PageInfo PageInfo    `json:"pageInfo"`
}
