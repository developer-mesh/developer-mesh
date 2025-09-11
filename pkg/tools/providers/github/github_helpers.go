package github

import (
	"encoding/json"
	"fmt"
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

// extractStringSlice extracts a string slice from interface{}
func extractStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	
	switch val := v.(type) {
	case []string:
		return val
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	default:
		return nil
	}
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

// convertToMinimalResponse converts GitHub SDK objects to minimal formats
func convertToMinimalResponse(data interface{}) interface{} {
	// This would implement minimal response conversion
	// For now, we'll just return the data as-is
	// In production, this would strip unnecessary fields
	return data
}

// isDirectory checks if a path is likely a directory
func isDirectory(path string) bool {
	// Simple heuristic: directories don't have file extensions
	// and don't end with common file patterns
	if path == "" || path == "/" {
		return true
	}
	
	parts := strings.Split(path, "/")
	lastPart := parts[len(parts)-1]
	
	// Check for file extension
	if strings.Contains(lastPart, ".") {
		return false
	}
	
	return true
}

// detectMimeType detects MIME type from file path
func detectMimeType(path string) string {
	lowerPath := strings.ToLower(path)
	
	switch {
	case strings.HasSuffix(lowerPath, ".json"):
		return "application/json"
	case strings.HasSuffix(lowerPath, ".yaml") || strings.HasSuffix(lowerPath, ".yml"):
		return "application/yaml"
	case strings.HasSuffix(lowerPath, ".xml"):
		return "application/xml"
	case strings.HasSuffix(lowerPath, ".html"):
		return "text/html"
	case strings.HasSuffix(lowerPath, ".css"):
		return "text/css"
	case strings.HasSuffix(lowerPath, ".js"):
		return "application/javascript"
	case strings.HasSuffix(lowerPath, ".ts"):
		return "application/typescript"
	case strings.HasSuffix(lowerPath, ".go"):
		return "text/x-go"
	case strings.HasSuffix(lowerPath, ".py"):
		return "text/x-python"
	case strings.HasSuffix(lowerPath, ".java"):
		return "text/x-java"
	case strings.HasSuffix(lowerPath, ".md"):
		return "text/markdown"
	case strings.HasSuffix(lowerPath, ".txt"):
		return "text/plain"
	case strings.HasSuffix(lowerPath, ".png"):
		return "image/png"
	case strings.HasSuffix(lowerPath, ".jpg") || strings.HasSuffix(lowerPath, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lowerPath, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lowerPath, ".svg"):
		return "image/svg+xml"
	default:
		return "text/plain"
	}
}