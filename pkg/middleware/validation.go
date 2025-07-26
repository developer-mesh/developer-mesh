package middleware

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// ValidationMiddleware provides input validation for API requests
type ValidationMiddleware struct {
	validator *validator.Validate
}

// NewValidationMiddleware creates a new validation middleware
func NewValidationMiddleware() *ValidationMiddleware {
	v := validator.New()

	// Register custom validators
	if err := v.RegisterValidation("url", validateURL); err != nil {
		// Log error but continue - validation will fail if used
		panic(fmt.Sprintf("failed to register url validator: %v", err))
	}
	if err := v.RegisterValidation("tool_name", validateToolName); err != nil {
		panic(fmt.Sprintf("failed to register tool_name validator: %v", err))
	}
	if err := v.RegisterValidation("auth_type", validateAuthType); err != nil {
		panic(fmt.Sprintf("failed to register auth_type validator: %v", err))
	}
	_ = v.RegisterValidation("no_sql", validateNoSQL)
	_ = v.RegisterValidation("safe_json", validateSafeJSON)

	return &ValidationMiddleware{
		validator: v,
	}
}

// ValidateRequest validates incoming requests
func (vm *ValidationMiddleware) ValidateRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Validate headers
		if err := vm.validateHeaders(c); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		// Validate query parameters
		if err := vm.validateQueryParams(c); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		// Validate path parameters
		if err := vm.validatePathParams(c); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		// Validate content type for POST/PUT
		if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH" {
			contentType := c.GetHeader("Content-Type")
			if !strings.HasPrefix(contentType, "application/json") {
				c.JSON(http.StatusBadRequest, gin.H{"error": "content-type must be application/json"})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// ValidateStruct validates a struct using the validator
func (vm *ValidationMiddleware) ValidateStruct(s interface{}) error {
	return vm.validator.Struct(s)
}

// validateHeaders validates request headers
func (vm *ValidationMiddleware) validateHeaders(c *gin.Context) error {
	// Check for injection attempts in headers
	headers := []string{"X-Tenant-ID", "X-Request-ID", "X-Tool-Name"}
	for _, header := range headers {
		value := c.GetHeader(header)
		if value != "" && containsSQLInjection(value) {
			return fmt.Errorf("invalid header value: %s", header)
		}
	}

	// Validate tenant ID format if present
	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID != "" && !isValidUUID(tenantID) {
		return fmt.Errorf("invalid tenant ID format")
	}

	return nil
}

// validateQueryParams validates query parameters
func (vm *ValidationMiddleware) validateQueryParams(c *gin.Context) error {
	// Check common query params
	params := c.Request.URL.Query()

	// Validate pagination
	if limit := params.Get("limit"); limit != "" {
		if !isValidNumber(limit, 1, 1000) {
			return fmt.Errorf("invalid limit parameter")
		}
	}

	if offset := params.Get("offset"); offset != "" {
		if !isValidNumber(offset, 0, 1000000) {
			return fmt.Errorf("invalid offset parameter")
		}
	}

	// Check for SQL injection in all params
	for key, values := range params {
		for _, value := range values {
			if containsSQLInjection(value) {
				return fmt.Errorf("invalid query parameter: %s", key)
			}
		}
	}

	return nil
}

// validatePathParams validates path parameters
func (vm *ValidationMiddleware) validatePathParams(c *gin.Context) error {
	// Validate tool ID
	if toolID := c.Param("toolId"); toolID != "" {
		if !isValidUUID(toolID) && !isValidIdentifier(toolID) {
			return fmt.Errorf("invalid tool ID format")
		}
	}

	// Validate action name
	if action := c.Param("action"); action != "" {
		if !isValidIdentifier(action) {
			return fmt.Errorf("invalid action name")
		}
	}

	// Validate session ID
	if sessionID := c.Param("sessionId"); sessionID != "" {
		if !isValidUUID(sessionID) {
			return fmt.Errorf("invalid session ID format")
		}
	}

	return nil
}

// Custom validators

func validateURL(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	u, err := url.Parse(value)
	if err != nil {
		return false
	}

	// Only allow http and https
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	// Must have a host
	if u.Host == "" {
		return false
	}

	// No user info allowed (prevents credential leaking)
	if u.User != nil {
		return false
	}

	return true
}

func validateToolName(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	// Tool names must be alphanumeric with dashes/underscores
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, value)
	return matched && len(value) >= 3 && len(value) <= 50
}

func validateAuthType(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	validTypes := []string{"token", "bearer", "api_key", "basic", "oauth2", "custom"}
	for _, t := range validTypes {
		if value == t {
			return true
		}
	}
	return false
}

func validateNoSQL(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	return !containsSQLInjection(value)
}

func validateSafeJSON(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	// Check for potential JSON injection patterns
	dangerous := []string{"__proto__", "constructor", "prototype"}
	valueLower := strings.ToLower(value)
	for _, pattern := range dangerous {
		if strings.Contains(valueLower, pattern) {
			return false
		}
	}
	return true
}

// Helper functions

func containsSQLInjection(s string) bool {
	// Common SQL injection patterns
	patterns := []string{
		"'",
		"\"",
		";",
		"--",
		"/*",
		"*/",
		"xp_",
		"sp_",
		"exec",
		"execute",
		"insert",
		"select",
		"delete",
		"update",
		"drop",
		"create",
		"alter",
		"union",
		"group by",
		"order by",
		"having",
		"1=1",
		"1 = 1",
		"or 1",
		"' or",
		"\" or",
	}

	sLower := strings.ToLower(s)
	for _, pattern := range patterns {
		if strings.Contains(sLower, pattern) {
			return true
		}
	}
	return false
}

func isValidUUID(s string) bool {
	uuidRegex := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	return uuidRegex.MatchString(s)
}

func isValidIdentifier(s string) bool {
	// Identifiers can contain alphanumeric, underscore, dash
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, s)
	return matched && len(s) > 0 && len(s) <= 100
}

func isValidNumber(s string, min, max int) bool {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return err == nil && n >= min && n <= max
}
