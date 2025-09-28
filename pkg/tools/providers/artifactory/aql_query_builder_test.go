package artifactory

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewAQLQueryBuilder(t *testing.T) {
	builder := NewAQLQueryBuilder()

	assert.NotNil(t, builder)
	assert.Equal(t, "items", builder.findType)
	assert.Equal(t, 100, builder.limit)
	assert.Equal(t, 0, builder.offset)
	assert.Empty(t, builder.criteria)
	assert.Empty(t, builder.includeFields)
	assert.Empty(t, builder.sortBy)
}

func TestFindTypes(t *testing.T) {
	tests := []struct {
		name     string
		method   func(*AQLQueryBuilder) *AQLQueryBuilder
		expected string
	}{
		{
			name:     "Find items",
			method:   (*AQLQueryBuilder).FindItems,
			expected: "items",
		},
		{
			name:     "Find builds",
			method:   (*AQLQueryBuilder).FindBuilds,
			expected: "builds",
		},
		{
			name:     "Find entries",
			method:   (*AQLQueryBuilder).FindEntries,
			expected: "entries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewAQLQueryBuilder()
			result := tt.method(builder)
			assert.Equal(t, builder, result) // Check fluent interface
			assert.Equal(t, tt.expected, builder.findType)
		})
	}
}

func TestFindItemsByName(t *testing.T) {
	builder := NewAQLQueryBuilder().FindItemsByName("*.jar")

	assert.Len(t, builder.criteria, 1)
	assert.Contains(t, builder.criteria[0], "name")

	nameMatch := builder.criteria[0]["name"].(map[string]string)
	assert.Equal(t, "*.jar", nameMatch["$match"])
}

func TestFindItemsByRepo(t *testing.T) {
	builder := NewAQLQueryBuilder().FindItemsByRepo("libs-release")

	assert.Len(t, builder.criteria, 1)
	assert.Equal(t, "libs-release", builder.criteria[0]["repo"])
}

func TestFindItemsByPath(t *testing.T) {
	builder := NewAQLQueryBuilder().FindItemsByPath("/com/example/*")

	assert.Len(t, builder.criteria, 1)
	assert.Contains(t, builder.criteria[0], "path")

	pathMatch := builder.criteria[0]["path"].(map[string]string)
	assert.Equal(t, "/com/example/*", pathMatch["$match"])
}

func TestFindItemsByProperty(t *testing.T) {
	builder := NewAQLQueryBuilder().FindItemsByProperty("build.number", "123")

	assert.Len(t, builder.criteria, 1)
	assert.Equal(t, "123", builder.criteria[0]["@build.number"])
}

func TestFindItemsByChecksum(t *testing.T) {
	tests := []struct {
		checksumType string
		checksum     string
		expectedKey  string
	}{
		{"sha1", "abc123", "actual_sha1"},
		{"sha256", "def456", "actual_sha256"},
		{"md5", "ghi789", "actual_md5"},
	}

	for _, tt := range tests {
		t.Run(tt.checksumType, func(t *testing.T) {
			builder := NewAQLQueryBuilder().FindItemsByChecksum(tt.checksumType, tt.checksum)

			assert.Len(t, builder.criteria, 1)
			assert.Equal(t, tt.checksum, builder.criteria[0][tt.expectedKey])
		})
	}
}

func TestFindItemsBySize(t *testing.T) {
	tests := []struct {
		name     string
		operator string
		size     int64
	}{
		{"Greater than", "$gt", 1000},
		{"Less than", "$lt", 500},
		{"Greater or equal", "$gte", 1000},
		{"Less or equal", "$lte", 500},
		{"Equal", "$eq", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewAQLQueryBuilder().FindItemsBySize(tt.operator, tt.size)

			assert.Len(t, builder.criteria, 1)
			sizeOp := builder.criteria[0]["size"].(map[string]int64)
			assert.Equal(t, tt.size, sizeOp[tt.operator])
		})
	}
}

func TestFindItemsByTime(t *testing.T) {
	now := time.Now()

	t.Run("Modified since", func(t *testing.T) {
		builder := NewAQLQueryBuilder().FindItemsModifiedSince(now)

		assert.Len(t, builder.criteria, 1)
		modifiedOp := builder.criteria[0]["modified"].(map[string]string)
		assert.Contains(t, modifiedOp, "$gt")
		assert.Contains(t, modifiedOp["$gt"], now.Format("2006-01-02"))
	})

	t.Run("Created before", func(t *testing.T) {
		builder := NewAQLQueryBuilder().FindItemsCreatedBefore(now)

		assert.Len(t, builder.criteria, 1)
		createdOp := builder.criteria[0]["created"].(map[string]string)
		assert.Contains(t, createdOp, "$lt")
		assert.Contains(t, createdOp["$lt"], now.Format("2006-01-02"))
	})
}

func TestFindItemsByType(t *testing.T) {
	tests := []string{"file", "folder"}

	for _, fileType := range tests {
		t.Run(fileType, func(t *testing.T) {
			builder := NewAQLQueryBuilder().FindItemsByType(fileType)

			assert.Len(t, builder.criteria, 1)
			assert.Equal(t, fileType, builder.criteria[0]["type"])
		})
	}
}

func TestInclude(t *testing.T) {
	builder := NewAQLQueryBuilder().
		Include("name", "repo").
		Include("path", "size")

	assert.Equal(t, []string{"name", "repo", "path", "size"}, builder.includeFields)
}

func TestSort(t *testing.T) {
	builder := NewAQLQueryBuilder().
		Sort("size", false).
		Sort("modified", true)

	assert.Len(t, builder.sortBy, 2)
	assert.Equal(t, "size", builder.sortBy[0].Field)
	assert.False(t, builder.sortBy[0].Asc)
	assert.Equal(t, "modified", builder.sortBy[1].Field)
	assert.True(t, builder.sortBy[1].Asc)
}

func TestLimitAndOffset(t *testing.T) {
	builder := NewAQLQueryBuilder().Limit(50).Offset(100)

	assert.Equal(t, 50, builder.limit)
	assert.Equal(t, 100, builder.offset)
}

func TestBuildSimpleQuery(t *testing.T) {
	query, err := NewAQLQueryBuilder().
		FindItemsByRepo("libs-release").
		Build()

	assert.NoError(t, err)
	assert.Contains(t, query, "items.find(")
	assert.Contains(t, query, "libs-release")
}

func TestBuildComplexQuery(t *testing.T) {
	query, err := NewAQLQueryBuilder().
		FindItemsByRepo("libs-release").
		FindItemsByName("*.jar").
		FindItemsBySize("$gt", 1000000).
		Include("name", "repo", "path", "size").
		Sort("size", false).
		Limit(20).
		Offset(40).
		Build()

	assert.NoError(t, err)
	assert.Contains(t, query, "items.find(")
	assert.Contains(t, query, "$and")
	assert.Contains(t, query, "libs-release")
	assert.Contains(t, query, "*.jar")
	assert.Contains(t, query, ".include(")
	assert.Contains(t, query, ".sort(")
	assert.Contains(t, query, ".limit(20)")
	assert.Contains(t, query, ".offset(40)")
}

func TestBuildSimpleMethod(t *testing.T) {
	t.Run("Single criterion", func(t *testing.T) {
		query, err := NewAQLQueryBuilder().
			FindItemsByRepo("docker-local").
			BuildSimple()

		assert.NoError(t, err)
		assert.Equal(t, `items.find({"repo": "docker-local"})`, query)
	})

	t.Run("With limit", func(t *testing.T) {
		query, err := NewAQLQueryBuilder().
			FindItemsByRepo("npm-local").
			Limit(10).
			BuildSimple()

		assert.NoError(t, err)
		assert.Equal(t, `items.find({"repo": "npm-local"}).limit(10)`, query)
	})

	t.Run("Falls back to Build for complex queries", func(t *testing.T) {
		query, err := NewAQLQueryBuilder().
			FindItemsByRepo("libs-release").
			FindItemsByName("*.jar").
			BuildSimple()

		assert.NoError(t, err)
		assert.Contains(t, query, "$and") // Complex query uses Build()
	})
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*AQLQueryBuilder)
		expectedErr string
	}{
		{
			name: "Invalid find type",
			setup: func(b *AQLQueryBuilder) {
				b.findType = "invalid"
			},
			expectedErr: "invalid find type",
		},
		{
			name: "Negative limit",
			setup: func(b *AQLQueryBuilder) {
				b.limit = -1
			},
			expectedErr: "limit cannot be negative",
		},
		{
			name: "Excessive limit",
			setup: func(b *AQLQueryBuilder) {
				b.limit = 10001
			},
			expectedErr: "limit exceeds maximum",
		},
		{
			name: "Negative offset",
			setup: func(b *AQLQueryBuilder) {
				b.offset = -1
			},
			expectedErr: "offset cannot be negative",
		},
		{
			name: "Empty sort field",
			setup: func(b *AQLQueryBuilder) {
				b.sortBy = []SortField{{Field: "", Asc: true}}
			},
			expectedErr: "sort field cannot be empty",
		},
		{
			name: "Empty include field",
			setup: func(b *AQLQueryBuilder) {
				b.includeFields = []string{"name", ""}
			},
			expectedErr: "include field cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewAQLQueryBuilder()
			tt.setup(builder)

			_, err := builder.Build()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestAddCustomCriterion(t *testing.T) {
	builder := NewAQLQueryBuilder().
		AddCustomCriterion(map[string]interface{}{
			"custom_field": map[string]string{"$regex": "pattern.*"},
		})

	assert.Len(t, builder.criteria, 1)
	assert.Contains(t, builder.criteria[0], "custom_field")
}

func TestGetCommonAQLExamples(t *testing.T) {
	examples := GetCommonAQLExamples()

	assert.NotEmpty(t, examples)

	// Test that each example builder can produce a valid query
	for name, builderFunc := range examples {
		t.Run(name, func(t *testing.T) {
			builder := builderFunc()
			query, err := builder.Build()

			assert.NoError(t, err)
			assert.NotEmpty(t, query)
			assert.Contains(t, query, ".find(")
		})
	}
}

func TestValidateAQLQuery(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid items query",
			query:       `items.find({"repo": "libs-release"})`,
			expectError: false,
		},
		{
			name:        "Valid builds query",
			query:       `builds.find({"name": "my-build"})`,
			expectError: false,
		},
		{
			name:        "Valid entries query",
			query:       `entries.find({"archive.entry.name": "*.class"})`,
			expectError: false,
		},
		{
			name:        "Empty query",
			query:       "",
			expectError: true,
			errorMsg:    "cannot be empty",
		},
		{
			name:        "Missing find method",
			query:       `items({"repo": "libs-release"})`,
			expectError: true,
			errorMsg:    "must contain .find()",
		},
		{
			name:        "Invalid find type",
			query:       `artifacts.find({"repo": "libs-release"})`,
			expectError: true,
			errorMsg:    "must start with items.find(), builds.find(), or entries.find()",
		},
		{
			name:        "Unbalanced parentheses",
			query:       `items.find({"repo": "libs-release"}`,
			expectError: true,
			errorMsg:    "unbalanced parentheses",
		},
		{
			name:        "Unbalanced braces",
			query:       `items.find({"repo": "libs-release")`,
			expectError: true,
			errorMsg:    "unbalanced braces",
		},
		{
			name:        "Double comma",
			query:       `items.find({"repo": "libs-release",, "name": "*.jar"})`,
			expectError: true,
			errorMsg:    "double comma",
		},
		{
			name:        "Double braces",
			query:       `items.find({{"repo": "libs-release"})`,
			expectError: true,
			errorMsg:    "double braces",
		},
		{
			name:        "With includes and sort",
			query:       `items.find({"repo": "libs-release"}).include("name", "size").sort({"$desc": ["size"]})`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAQLQuery(tt.query)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFluentInterface(t *testing.T) {
	// Test that all methods return the builder for chaining
	builder := NewAQLQueryBuilder()

	result := builder.
		FindItems().
		FindItemsByName("*.jar").
		FindItemsByRepo("libs-release").
		FindItemsByPath("/com/*").
		FindItemsByProperty("key", "value").
		FindItemsByChecksum("sha1", "abc").
		FindItemsBySize("$gt", 1000).
		FindItemsModifiedSince(time.Now()).
		FindItemsCreatedBefore(time.Now()).
		FindItemsByType("file").
		Include("name").
		Sort("size", true).
		Limit(10).
		Offset(5).
		AddCustomCriterion(map[string]interface{}{"custom": "value"})

	assert.Equal(t, builder, result)
}
