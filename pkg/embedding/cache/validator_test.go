package cache

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryValidator_Validate(t *testing.T) {
	validator := NewQueryValidator()

	tests := []struct {
		name    string
		query   string
		wantErr error
	}{
		{
			name:    "empty query",
			query:   "",
			wantErr: ErrEmptyQuery,
		},
		{
			name:    "query too long",
			query:   strings.Repeat("a", 1001),
			wantErr: ErrQueryTooLong,
		},
		{
			name:    "valid query",
			query:   "How to implement Redis cache?",
			wantErr: nil,
		},
		{
			name:    "query with special characters",
			query:   "SELECT * FROM users; DROP TABLE users;--",
			wantErr: nil, // We allow most characters, sanitization handles safety
		},
		{
			name:    "query with emojis",
			query:   "How to cache data? 🤔",
			wantErr: nil,
		},
		{
			name:    "query with newlines",
			query:   "Multi\nline\nquery",
			wantErr: nil,
		},
		{
			name:    "query with tabs",
			query:   "Query\twith\ttabs",
			wantErr: nil,
		},
		{
			name:    "query with mixed languages",
			query:   "How to implement 缓存 cache?",
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.query)
			if tt.wantErr != nil {
				assert.Equal(t, tt.wantErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestQueryValidator_Sanitize(t *testing.T) {
	validator := NewQueryValidator()

	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "normal query",
			query:    "How to implement Redis cache?",
			expected: "How to implement Redis cache?",
		},
		{
			name:     "query with control characters",
			query:    "Query\x00with\x01control\x1Fchars",
			expected: "Querywithcontrolchars",
		},
		{
			name:     "query with spaces to trim",
			query:    "  Query with spaces  ",
			expected: "Query with spaces",
		},
		{
			name:     "query longer than max",
			query:    strings.Repeat("a", 1100),
			expected: strings.Repeat("a", 1000),
		},
		{
			name:     "query with newlines",
			query:    "Multi\nline\nquery",
			expected: "Multi line query",
		},
		{
			name:     "empty after sanitization",
			query:    "\x00\x01\x02",
			expected: "",
		},
		{
			name:     "invalid UTF-8",
			query:    "Valid text \xc3\x28 invalid UTF-8",
			expected: "Valid text ( invalid UTF-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Sanitize(tt.query)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeRedisKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "normal key",
			key:      "simple_key_123",
			expected: "simple_key_123",
		},
		{
			name:     "key with spaces",
			key:      "key with spaces",
			expected: "key_with_spaces",
		},
		{
			name:     "key with colons",
			key:      "namespace:key:value",
			expected: "namespace-key-value",
		},
		{
			name:     "key with wildcards",
			key:      "key*with?wildcards[1-9]",
			expected: "key-with-wildcards-1-9-",
		},
		{
			name:     "key with braces",
			key:      "key{hash}value",
			expected: "key-hash-value",
		},
		{
			name:     "key with newlines",
			key:      "key\nwith\nnewlines",
			expected: "key-with-newlines",
		},
		{
			name:     "empty key",
			key:      "",
			expected: "empty_key",
		},
		{
			name:     "key with backslashes",
			key:      "path\\to\\file",
			expected: "path-to-file",
		},
		{
			name:     "key with control chars",
			key:      "key\x00with\x01control",
			expected: "key-withcontrol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeRedisKey(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func BenchmarkQueryValidator_Validate(b *testing.B) {
	validator := NewQueryValidator()
	query := "How to implement a distributed cache system with Redis?"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Validate(query)
	}
}

func BenchmarkQueryValidator_Sanitize(b *testing.B) {
	validator := NewQueryValidator()
	query := "Query with \x00 control chars \n and spaces   "

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Sanitize(query)
	}
}

func BenchmarkSanitizeRedisKey(b *testing.B) {
	key := "namespace:key:with:special*chars?and[brackets]"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sanitizeRedisKey(key)
	}
}
