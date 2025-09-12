package github

import (
	"fmt"
	"net/http"

	"github.com/google/go-github/v74/github"
)

// GitHubAPIError represents a GitHub API error
type GitHubAPIError struct {
	Message  string
	Response *github.Response
	Err      error
}

// Error implements the error interface
func (e *GitHubAPIError) Error() string {
	if e.Response != nil {
		return fmt.Sprintf("%s (status: %d)", e.Message, e.Response.StatusCode)
	}
	return e.Message
}

// NewGitHubAPIError creates a new GitHub API error
func newGitHubAPIError(message string, resp *github.Response, err error) *GitHubAPIError {
	return &GitHubAPIError{
		Message:  message,
		Response: resp,
		Err:      err,
	}
}

// NewToolError creates a ToolResult for an error
func NewToolError(message string) *ToolResult {
	return &ToolResult{
		Content: nil,
		IsError: true,
		Error:   message,
	}
}

// NewToolResult creates a successful ToolResult
func NewToolResult(content interface{}) *ToolResult {
	return &ToolResult{
		Content: content,
		IsError: false,
	}
}

// NewGitHubAPIErrorResponse returns a ToolResult for GitHub API errors
func NewGitHubAPIErrorResponse(message string, resp *github.Response, err error) *ToolResult {
	apiErr := newGitHubAPIError(message, resp, err)

	// Check for specific GitHub error types
	if resp != nil {
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return NewToolError(fmt.Sprintf("Authentication failed: %s", message))
		case http.StatusForbidden:
			if resp.Rate.Remaining == 0 {
				return NewToolError(fmt.Sprintf("Rate limit exceeded. Resets at %v", resp.Rate.Reset))
			}
			return NewToolError(fmt.Sprintf("Permission denied: %s", message))
		case http.StatusNotFound:
			return NewToolError(fmt.Sprintf("Resource not found: %s", message))
		case http.StatusUnprocessableEntity:
			return NewToolError(fmt.Sprintf("Validation failed: %s", message))
		default:
			return NewToolError(fmt.Sprintf("%s (status: %d)", message, resp.StatusCode))
		}
	}

	// Generic error
	if err != nil {
		return NewToolError(fmt.Sprintf("%s: %v", message, err))
	}

	return NewToolError(apiErr.Error())
}
