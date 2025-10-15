package github

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/rag/models"
)

func TestNewCrawler(t *testing.T) {
	tenantID := uuid.New()
	config := Config{
		Owner:  "test-owner",
		Repo:   "test-repo",
		Branch: "main",
		Token:  "",
	}

	crawler, err := NewCrawler(tenantID, config)
	require.NoError(t, err)
	assert.NotNil(t, crawler)
	assert.Equal(t, tenantID, crawler.tenantID)
	assert.Equal(t, "github_test-owner_test-repo", crawler.ID())
}

func TestCrawler_ID(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{
			name: "simple repo",
			config: Config{
				Owner: "owner1",
				Repo:  "repo1",
			},
			want: "github_owner1_repo1",
		},
		{
			name: "org repo",
			config: Config{
				Owner: "my-org",
				Repo:  "my-project",
			},
			want: "github_my-org_my-project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crawler, err := NewCrawler(uuid.New(), tt.config)
			require.NoError(t, err)
			assert.Equal(t, tt.want, crawler.ID())
		})
	}
}

func TestCrawler_Type(t *testing.T) {
	crawler, err := NewCrawler(uuid.New(), Config{Owner: "test", Repo: "test"})
	require.NoError(t, err)
	assert.Equal(t, models.SourceTypeGitHub, crawler.Type())
}

func TestCrawler_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Owner: "test-owner",
				Repo:  "test-repo",
			},
			wantErr: false,
		},
		{
			name: "missing owner",
			config: Config{
				Repo: "test-repo",
			},
			wantErr: true,
		},
		{
			name: "missing repo",
			config: Config{
				Owner: "test-owner",
			},
			wantErr: true,
		},
		{
			name:    "empty config",
			config:  Config{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crawler, err := NewCrawler(uuid.New(), tt.config)
			require.NoError(t, err)

			err = crawler.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCrawler_shouldProcess(t *testing.T) {
	tests := []struct {
		name            string
		includePatterns []string
		excludePatterns []string
		path            string
		want            bool
	}{
		{
			name:            "no patterns - include all",
			includePatterns: []string{},
			excludePatterns: []string{},
			path:            "src/main.go",
			want:            true,
		},
		{
			name:            "include go files",
			includePatterns: []string{"*.go"},
			excludePatterns: []string{},
			path:            "main.go",
			want:            true,
		},
		{
			name:            "exclude test files",
			includePatterns: []string{"*.go"},
			excludePatterns: []string{"*_test.go"},
			path:            "main_test.go",
			want:            false,
		},
		{
			name:            "exclude vendor directory",
			includePatterns: []string{},
			excludePatterns: []string{"vendor/**"},
			path:            "vendor/pkg/lib.go",
			want:            false,
		},
		{
			name:            "include markdown",
			includePatterns: []string{"*.md"},
			excludePatterns: []string{},
			path:            "README.md",
			want:            true,
		},
		{
			name:            "complex pattern",
			includePatterns: []string{"*.go", "*.md"},
			excludePatterns: []string{"*_test.go", ".git/**"},
			path:            "pkg/service.go",
			want:            true,
		},
		{
			name:            "excluded git directory",
			includePatterns: []string{"*"},
			excludePatterns: []string{".git/**"},
			path:            ".git/config",
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crawler, err := NewCrawler(uuid.New(), Config{
				Owner:           "test",
				Repo:            "test",
				IncludePatterns: tt.includePatterns,
				ExcludePatterns: tt.excludePatterns,
			})
			require.NoError(t, err)

			got := crawler.shouldProcess(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCrawler_calculateBaseScore(t *testing.T) {
	crawler, err := NewCrawler(uuid.New(), Config{Owner: "test", Repo: "test"})
	require.NoError(t, err)

	tests := []struct {
		name string
		doc  *models.Document
		want float64
	}{
		{
			name: "README file - high score",
			doc: &models.Document{
				Title: "README.md",
			},
			want: 0.8, // 0.5 base + 0.3 README boost
		},
		{
			name: "API interface file - high score",
			doc: &models.Document{
				Title: "pkg/api/interface.go",
			},
			want: 0.75, // 0.5 base + 0.1 pkg + 0.15 interface
		},
		{
			name: "Documentation file",
			doc: &models.Document{
				Title: "docs/guide.md",
			},
			want: 0.7, // 0.5 base + 0.2 doc
		},
		{
			name: "Regular source file",
			doc: &models.Document{
				Title: "internal/util.go",
			},
			want: 0.5, // Base score only
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := crawler.calculateBaseScore(tt.doc)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCrawler_calculateQualityScore(t *testing.T) {
	crawler, err := NewCrawler(uuid.New(), Config{Owner: "test", Repo: "test"})
	require.NoError(t, err)

	tests := []struct {
		name string
		doc  *models.Document
		want float64
	}{
		{
			name: "well-documented markdown",
			doc: &models.Document{
				Title:   "guide.md",
				Content: strings.Repeat("# Header\n\n```go\ncode\n```\n\n", 10), // ~200 chars
			},
			want: 0.7, // 0.5 base + 0.1 headers + 0.1 code (length threshold not met)
		},
		{
			name: "go file with comments",
			doc: &models.Document{
				Title:   "main.go",
				Content: strings.Repeat("// Comment\nfunc test() {}\n", 50), // ~1000 chars
			},
			want: 0.8, // 0.5 base + 0.2 length + 0.1 comments
		},
		{
			name: "too short content",
			doc: &models.Document{
				Title:   "small.go",
				Content: "package main",
			},
			want: 0.5, // Base only
		},
		{
			name: "very long file",
			doc: &models.Document{
				Title:   "large.go",
				Content: strings.Repeat("x", 100000),
			},
			want: 0.6, // 0.5 base + 0.1 for long content
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := crawler.calculateQualityScore(tt.doc)
			assert.InDelta(t, tt.want, got, 0.01) // Allow small floating-point differences
		})
	}
}

func TestCrawler_GetMetadata(t *testing.T) {
	config := Config{
		Owner:           "test-owner",
		Repo:            "test-repo",
		Branch:          "main",
		IncludePatterns: []string{"*.go", "*.md"},
		ExcludePatterns: []string{"*_test.go"},
	}

	crawler, err := NewCrawler(uuid.New(), config)
	require.NoError(t, err)

	metadata := crawler.GetMetadata()

	assert.Equal(t, "test-owner", metadata["owner"])
	assert.Equal(t, "test-repo", metadata["repo"])
	assert.Equal(t, "main", metadata["branch"])
	assert.Equal(t, config.IncludePatterns, metadata["include_patterns"])
	assert.Equal(t, config.ExcludePatterns, metadata["exclude_patterns"])
}
