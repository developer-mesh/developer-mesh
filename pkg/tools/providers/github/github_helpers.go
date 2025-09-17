package github

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ToBoolPtr converts a bool to a *bool pointer
func ToBoolPtr(b bool) *bool {
	return &b
}

// ToStringPtr converts a string to a *string pointer
// Returns nil if the string is empty
func ToStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ToIntPtr converts an int to a *int pointer
func ToIntPtr(i int) *int {
	return &i
}

// marshalJSON safely marshals data to JSON string
func marshalJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error": "failed to marshal: %v"}`, err)
	}
	return string(data)
}

// extractString safely extracts a string from params
func extractString(params map[string]interface{}, key string) string {
	if v, ok := params[key].(string); ok {
		return v
	}
	return ""
}

// extractInt safely extracts an int from params
func extractInt(params map[string]interface{}, key string) int {
	if v, ok := params[key].(float64); ok {
		return int(v)
	}
	if v, ok := params[key].(int); ok {
		return v
	}
	// Try to parse from string
	if v, ok := params[key].(string); ok {
		if intVal, err := strconv.Atoi(v); err == nil {
			return intVal
		}
	}
	// Try int64 (sometimes JSON unmarshaling uses int64)
	if v, ok := params[key].(int64); ok {
		return int(v)
	}
	return 0
}

// extractBool safely extracts a bool from params
func extractBool(params map[string]interface{}, key string) bool {
	if v, ok := params[key].(bool); ok {
		return v
	}
	return false
}

// BuildSearchQuery builds a GitHub search query from filters
func BuildSearchQuery(filters map[string]interface{}) string {
	parts := []string{}

	// Add type qualifiers
	if repo, ok := filters["repo"].(string); ok {
		parts = append(parts, fmt.Sprintf("repo:%s", repo))
	}
	if lang, ok := filters["language"].(string); ok {
		parts = append(parts, fmt.Sprintf("language:%s", lang))
	}
	if user, ok := filters["user"].(string); ok {
		parts = append(parts, fmt.Sprintf("user:%s", user))
	}
	if org, ok := filters["org"].(string); ok {
		parts = append(parts, fmt.Sprintf("org:%s", org))
	}
	if state, ok := filters["state"].(string); ok {
		parts = append(parts, fmt.Sprintf("state:%s", state))
	}
	if typ, ok := filters["type"].(string); ok {
		parts = append(parts, fmt.Sprintf("type:%s", typ))
	}

	// Add the main query
	if q, ok := filters["q"].(string); ok {
		parts = append(parts, q)
	} else if query, ok := filters["query"].(string); ok {
		parts = append(parts, query)
	}

	return strings.Join(parts, " ")
}

// ErrorResult creates an error ToolResult
func ErrorResult(format string, args ...interface{}) *ToolResult {
	var message string
	if len(args) > 0 {
		message = fmt.Sprintf(format, args...)
	} else {
		message = format
	}

	return &ToolResult{
		Content: nil,
		IsError: true,
		Error:   message,
	}
}

// SuccessResult creates a success ToolResult
func SuccessResult(content interface{}) *ToolResult {
	return &ToolResult{
		Content: content,
		IsError: false,
	}
}
