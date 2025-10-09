package common

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PrettyPrint prints a JSON object in a formatted way
func PrettyPrint(v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Printf("Error pretty printing: %v\n", err)
		return
	}
	fmt.Println(string(b))
}

// PrintSection prints a section header
func PrintSection(title string) {
	fmt.Printf("\n%s\n%s\n", title, strings.Repeat("=", len(title)))
}

// PrintSubsection prints a subsection header
func PrintSubsection(title string) {
	fmt.Printf("\n%s\n%s\n", title, strings.Repeat("-", len(title)))
}

// PrintSuccess prints a success message
func PrintSuccess(message string) {
	fmt.Printf("✓ %s\n", message)
}

// PrintError prints an error message
func PrintError(message string, err error) {
	fmt.Printf("✗ %s: %v\n", message, err)
}

// PrintInfo prints an informational message
func PrintInfo(message string) {
	fmt.Printf("ℹ %s\n", message)
}

// ParseToolResult parses a tool result into a target struct
func ParseToolResult(result json.RawMessage, target interface{}) error {
	return json.Unmarshal(result, target)
}

// FormatDuration formats a duration in milliseconds to a human-readable string
func FormatDuration(ms float64) string {
	d := time.Duration(ms * float64(time.Millisecond))
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fμs", float64(d)/float64(time.Microsecond))
	} else if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d)/float64(time.Millisecond))
	} else {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

// RetryWithBackoff retries a function with exponential backoff
func RetryWithBackoff(fn func() error, maxRetries int, initialDelay time.Duration) error {
	var err error
	delay := initialDelay

	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		if i < maxRetries-1 {
			PrintInfo(fmt.Sprintf("Retry %d/%d after %v: %v", i+1, maxRetries, delay, err))
			time.Sleep(delay)
			delay *= 2 // Exponential backoff
		}
	}

	return fmt.Errorf("failed after %d retries: %w", maxRetries, err)
}

// ExtractErrorCode extracts the error code from an MCP error
func ExtractErrorCode(err error) (int, string) {
	if err == nil {
		return 0, ""
	}

	errStr := err.Error()
	if strings.Contains(errStr, "RPC error") {
		// Format: "RPC error CODE: MESSAGE"
		parts := strings.SplitN(errStr, ":", 2)
		if len(parts) == 2 {
			var code int
			fmt.Sscanf(parts[0], "RPC error %d", &code)
			message := strings.TrimSpace(parts[1])
			return code, message
		}
	}

	return -1, errStr
}

// IsRetryableError determines if an error should be retried
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	code, _ := ExtractErrorCode(err)

	// Retryable error codes
	switch code {
	case -32000: // Server error (may be transient)
	case 429: // Rate limit exceeded
	case 503: // Service unavailable
	case 504: // Gateway timeout
		return true
	}

	// Check for network errors
	errStr := err.Error()
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") {
		return true
	}

	return false
}

// HandleRateLimitError handles rate limit errors with proper backoff
func HandleRateLimitError(err error) time.Duration {
	errStr := err.Error()

	// Try to extract retry_after from error message
	// Format might include: "retry_after: 30"
	if strings.Contains(errStr, "retry_after") {
		var retryAfter int
		fmt.Sscanf(errStr, "retry_after: %d", &retryAfter)
		if retryAfter > 0 {
			return time.Duration(retryAfter) * time.Second
		}
	}

	// Default exponential backoff
	return 5 * time.Second
}

// FilterTools filters tools by category
func FilterTools(tools []Tool, category string) []Tool {
	var filtered []Tool
	for _, tool := range tools {
		if tool.Category == category {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

// FilterToolsByTag filters tools by tag
func FilterToolsByTag(tools []Tool, tag string) []Tool {
	var filtered []Tool
	for _, tool := range tools {
		for _, t := range tool.Tags {
			if t == tag {
				filtered = append(filtered, tool)
				break
			}
		}
	}
	return filtered
}

// FindTool finds a tool by name
func FindTool(tools []Tool, name string) *Tool {
	for _, tool := range tools {
		if tool.Name == name {
			return &tool
		}
	}
	return nil
}

// FuzzySearchTools searches for tools by name or description
func FuzzySearchTools(tools []Tool, query string) []Tool {
	query = strings.ToLower(query)
	var matches []Tool

	for _, tool := range tools {
		if strings.Contains(strings.ToLower(tool.Name), query) ||
			strings.Contains(strings.ToLower(tool.Description), query) {
			matches = append(matches, tool)
		}
	}

	return matches
}

// SummarizeBatchResult creates a human-readable summary of batch results
func SummarizeBatchResult(result *BatchResult) string {
	mode := "sequential"
	if result.Parallel {
		mode = "parallel"
	}

	return fmt.Sprintf(
		"Batch execution (%s): %d total, %d succeeded, %d failed, duration: %s",
		mode,
		len(result.Results),
		result.SuccessCount,
		result.ErrorCount,
		FormatDuration(result.DurationMS),
	)
}

// ValidateArguments validates that all required arguments are present
func ValidateArguments(args map[string]interface{}, required []string) error {
	for _, key := range required {
		if _, ok := args[key]; !ok {
			return fmt.Errorf("missing required argument: %s", key)
		}
	}
	return nil
}

// PrintToolList prints a formatted list of tools
func PrintToolList(tools []Tool) {
	fmt.Printf("\nAvailable Tools (%d total)\n", len(tools))
	fmt.Println(strings.Repeat("=", 80))

	// Group by category
	categories := make(map[string][]Tool)
	for _, tool := range tools {
		category := tool.Category
		if category == "" {
			category = "uncategorized"
		}
		categories[category] = append(categories[category], tool)
	}

	// Print each category
	for category, tools := range categories {
		fmt.Printf("\n%s (%d tools):\n", strings.ToUpper(category), len(tools))
		for _, tool := range tools {
			tags := ""
			if len(tool.Tags) > 0 {
				tags = fmt.Sprintf(" [%s]", strings.Join(tool.Tags, ", "))
			}
			fmt.Printf("  • %-40s %s%s\n", tool.Name, tool.Description, tags)
		}
	}
}
