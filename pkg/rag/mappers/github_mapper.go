// Package mappers provides source-specific document mapping functionality
package mappers

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/rag/models"
)

// GitHubMapper maps GitHub-specific data to unified document model
type GitHubMapper struct {
	// Configuration for scoring weights
	baseScoreBoost float64
}

// NewGitHubMapper creates a new GitHub mapper
func NewGitHubMapper() *GitHubMapper {
	return &GitHubMapper{
		baseScoreBoost: 0.1,
	}
}

// CalculateBaseScore calculates base score for GitHub files
func (m *GitHubMapper) CalculateBaseScore(doc *models.Document) float64 {
	score := 0.5 // Default base score

	title := strings.ToLower(doc.Title)

	// Boost for README files
	if strings.Contains(title, "readme") {
		score += 0.3
	}

	// Boost for documentation files
	if strings.HasSuffix(title, ".md") {
		score += 0.1
	}

	// Boost for main directories
	if strings.HasPrefix(title, "pkg/") {
		score += 0.15
	} else if strings.HasPrefix(title, "cmd/") {
		score += 0.1
	} else if strings.HasPrefix(title, "docs/") {
		score += 0.2
	}

	// Boost for configuration files
	if strings.HasSuffix(title, ".yaml") || strings.HasSuffix(title, ".yml") {
		score += 0.05
	}

	// Boost for important code files
	if strings.HasSuffix(title, ".go") {
		score += 0.05
	}

	// Penalize test files
	if strings.Contains(title, "_test.go") {
		score -= 0.2
	}

	// Penalize vendor and generated files
	if strings.Contains(title, "vendor/") || strings.Contains(title, "node_modules/") {
		score -= 0.3
	}

	return math.Min(math.Max(score, 0.0), 1.0)
}

// CalculateAuthorityScore calculates authority score based on file metadata
func (m *GitHubMapper) CalculateAuthorityScore(metadata map[string]interface{}) float64 {
	score := 0.5 // Default authority

	// Check if it's from a main branch
	if branch, ok := metadata["branch"].(string); ok {
		if branch == "main" || branch == "master" {
			score += 0.2
		}
	}

	// Check file path for authority indicators
	if path, ok := metadata["path"].(string); ok {
		// Root-level files are more authoritative
		if !strings.Contains(path, "/") {
			score += 0.15
		}

		// Architecture and design docs are authoritative
		if strings.Contains(path, "architecture") || strings.Contains(path, "design") {
			score += 0.2
		}
	}

	return math.Min(score, 1.0)
}

// CalculatePopularityScore calculates popularity based on file characteristics
func (m *GitHubMapper) CalculatePopularityScore(metadata map[string]interface{}) float64 {
	score := 0.5 // Base popularity

	// Larger files might be more comprehensive (but not always better)
	if size, ok := metadata["size"].(int); ok {
		if size > 5000 && size < 50000 {
			score += 0.1
		}
	}

	return math.Min(score, 1.0)
}

// EnrichDocument enriches a GitHub document with calculated scores
func (m *GitHubMapper) EnrichDocument(doc *models.Document) {
	// Calculate scoring components
	doc.BaseScore = m.CalculateBaseScore(doc)
	doc.AuthorityScore = m.CalculateAuthorityScore(doc.Metadata)
	doc.PopularityScore = m.CalculatePopularityScore(doc.Metadata)

	// Freshness is set based on last commit time (should be in metadata)
	if updatedAt, ok := doc.Metadata["updated_at"].(time.Time); ok {
		daysSinceUpdate := time.Since(updatedAt).Hours() / 24
		if daysSinceUpdate < 7 {
			doc.FreshnessScore = 1.0
		} else if daysSinceUpdate < 30 {
			doc.FreshnessScore = 0.8
		} else if daysSinceUpdate < 90 {
			doc.FreshnessScore = 0.6
		} else {
			doc.FreshnessScore = 0.4
		}
	} else {
		// Default freshness if no timestamp
		doc.FreshnessScore = 0.5
	}
}

// FormatFileContent formats GitHub file content for better embedding
func (m *GitHubMapper) FormatFileContent(doc *models.Document) string {
	var builder strings.Builder

	// Add context header
	builder.WriteString(fmt.Sprintf("# File: %s\n", doc.Title))
	builder.WriteString("Source: GitHub Repository\n")

	if url, ok := doc.Metadata["url"].(string); ok {
		builder.WriteString(fmt.Sprintf("URL: %s\n", url))
	}

	builder.WriteString("\n")

	// Add file content
	builder.WriteString(doc.Content)

	return builder.String()
}

// ExtractMetadata extracts structured metadata from GitHub file
func (m *GitHubMapper) ExtractMetadata(doc *models.Document) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Copy existing metadata
	for k, v := range doc.Metadata {
		metadata[k] = v
	}

	// Add derived metadata
	metadata["source_type"] = "github"
	metadata["file_extension"] = m.getFileExtension(doc.Title)
	metadata["is_documentation"] = m.isDocumentation(doc.Title)
	metadata["is_code"] = m.isCode(doc.Title)
	metadata["language"] = m.detectLanguage(doc.Title)

	return metadata
}

// getFileExtension extracts file extension
func (m *GitHubMapper) getFileExtension(title string) string {
	parts := strings.Split(title, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ""
}

// isDocumentation checks if file is documentation
func (m *GitHubMapper) isDocumentation(title string) bool {
	lower := strings.ToLower(title)
	return strings.HasSuffix(lower, ".md") ||
		strings.Contains(lower, "readme") ||
		strings.Contains(lower, "docs/")
}

// isCode checks if file is code
func (m *GitHubMapper) isCode(title string) bool {
	lower := strings.ToLower(title)
	codeExtensions := []string{".go", ".js", ".ts", ".py", ".java", ".cpp", ".c", ".rs"}
	for _, ext := range codeExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// detectLanguage detects programming language from file extension
func (m *GitHubMapper) detectLanguage(title string) string {
	lower := strings.ToLower(title)

	languageMap := map[string]string{
		".go":   "go",
		".js":   "javascript",
		".ts":   "typescript",
		".py":   "python",
		".java": "java",
		".cpp":  "cpp",
		".c":    "c",
		".rs":   "rust",
		".rb":   "ruby",
		".php":  "php",
		".sh":   "shell",
		".yaml": "yaml",
		".yml":  "yaml",
		".json": "json",
		".md":   "markdown",
	}

	for ext, lang := range languageMap {
		if strings.HasSuffix(lower, ext) {
			return lang
		}
	}

	return "unknown"
}
