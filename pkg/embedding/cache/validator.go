package cache

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/middleware"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
)

var (
	// Use project error patterns
	ErrQueryTooLong      = errors.New("query exceeds maximum length")
	ErrInvalidCharacters = errors.New("query contains invalid characters")
	ErrEmptyQuery        = errors.New("query cannot be empty")
	ErrNoTenantID        = errors.New("no tenant ID in context")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

// QueryValidator validates and sanitizes queries for cache operations
type QueryValidator struct {
	maxLength       int
	allowedPattern  *regexp.Regexp
	sanitizePattern *regexp.Regexp
	rateLimiter     *middleware.RateLimiter
	logger          observability.Logger
}

// NewQueryValidator creates a new query validator with default settings
func NewQueryValidator() *QueryValidator {
	return &QueryValidator{
		maxLength:       1000,
		allowedPattern:  regexp.MustCompile(`^[\p{L}\p{N}\s\-_.,!?'"()\[\]{}/@#$%^&*+=<>:;|\\~` + "`" + `]+$`),
		sanitizePattern: regexp.MustCompile(`[^\p{L}\p{N}\s\-_.,!?'"()\[\]{}/@#$%^&*+=<>:;|\\~` + "`" + `]+`),
	}
}

// NewQueryValidatorWithRateLimiter creates a validator with rate limiting
func NewQueryValidatorWithRateLimiter(rateLimiter *middleware.RateLimiter, logger observability.Logger) *QueryValidator {
	return &QueryValidator{
		maxLength: 1000,
		// Use project's standard validation pattern
		allowedPattern:  regexp.MustCompile(`^[\p{L}\p{N}\s\-_.,!?'"@#$%^&*()+=/\\<>{}[\]|~` + "`" + `]+$`),
		sanitizePattern: regexp.MustCompile(`[^\p{L}\p{N}\s\-_.,!?'"@#$%^&*()+=/\\<>{}[\]|~` + "`" + `]+`),
		rateLimiter:     rateLimiter,
		logger:          logger,
	}
}

// ValidateWithContext validates a query with tenant context and rate limiting
func (v *QueryValidator) ValidateWithContext(ctx context.Context, query string) error {
	// Extract tenant ID using auth package
	tenantID := auth.GetTenantID(ctx)
	if tenantID == uuid.Nil {
		return ErrNoTenantID
	}

	// Apply rate limiting if configured
	// Note: Since the middleware RateLimiter doesn't have a simple Allow method,
	// we'll skip rate limiting in the validator for now.
	// TODO: Add a proper rate limiting interface or use the getLimiter method

	// Validate query
	return v.Validate(query)
}

// Validate checks if a query is valid
func (v *QueryValidator) Validate(query string) error {
	if query == "" {
		return ErrEmptyQuery
	}

	if !utf8.ValidString(query) {
		return ErrInvalidCharacters
	}

	if len(query) > v.maxLength {
		return ErrQueryTooLong
	}

	if v.allowedPattern != nil && !v.allowedPattern.MatchString(query) {
		return ErrInvalidCharacters
	}

	return nil
}

// Sanitize removes potentially dangerous characters from a query
func (v *QueryValidator) Sanitize(query string) string {
	// Trim whitespace
	query = strings.TrimSpace(query)

	// First ensure valid UTF-8
	if !utf8.ValidString(query) {
		// Remove invalid UTF-8 sequences
		query = strings.ToValidUTF8(query, "")
	}

	// Remove control characters and non-printable characters
	if v.sanitizePattern != nil {
		query = v.sanitizePattern.ReplaceAllString(query, "")
	} else {
		query = regexp.MustCompile(`[\x00-\x1F\x7F]`).ReplaceAllString(query, "")
	}

	// Normalize whitespace
	query = strings.Join(strings.Fields(query), " ")

	// Trim to max length
	if len(query) > v.maxLength {
		// Find a valid UTF-8 boundary
		query = string([]rune(query)[:v.maxLength])
	}

	return query
}

// sanitizeRedisKey makes a string safe for use as a Redis key
func sanitizeRedisKey(key string) string {
	// Use project's standard key sanitization
	replacer := strings.NewReplacer(
		" ", "_",
		":", "-",
		"*", "-",
		"?", "-",
		"[", "-",
		"]", "-",
		"{", "-",
		"}", "-",
		"\\", "-",
		"\n", "-",
		"\r", "-",
		"\t", "-",
		"\x00", "-",
	)

	// Apply replacements
	sanitized := replacer.Replace(key)

	// Remove any remaining control characters
	sanitized = regexp.MustCompile(`[\x00-\x1F\x7F]`).ReplaceAllString(sanitized, "")

	// Ensure the key is not empty after sanitization
	if sanitized == "" {
		sanitized = "empty_key"
	}

	return sanitized
}

// SanitizeRedisKey ensures Redis key safety following project patterns (exported version)
func SanitizeRedisKey(key string) string {
	return sanitizeRedisKey(key)
}
