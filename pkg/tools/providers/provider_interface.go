package providers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

// StandardToolProvider defines the interface for all standard tool providers
// (GitHub, GitLab, Jira, etc.) as specified in STANDARD_TOOLS_INTEGRATION_PLAN.md
type StandardToolProvider interface {
	// GetProviderName returns the unique name of the provider (e.g., "github", "gitlab")
	GetProviderName() string

	// GetSupportedVersions returns the API versions this provider supports
	GetSupportedVersions() []string

	// GetToolDefinitions returns the list of tool definitions this provider offers
	GetToolDefinitions() []ToolDefinition

	// ValidateCredentials validates the provided credentials for this provider
	ValidateCredentials(ctx context.Context, creds map[string]string) error

	// ExecuteOperation executes a specific operation with the given parameters
	ExecuteOperation(ctx context.Context, op string, params map[string]interface{}) (interface{}, error)

	// GetOperationMappings returns the operation ID to API endpoint mappings
	GetOperationMappings() map[string]OperationMapping

	// GetDefaultConfiguration returns the default configuration for this provider
	GetDefaultConfiguration() ProviderConfig

	// GetAIOptimizedDefinitions returns AI-optimized tool definitions for better agent comprehension
	GetAIOptimizedDefinitions() []AIOptimizedToolDefinition

	// GetOpenAPISpec returns the OpenAPI specification for permission filtering
	// This is required for the PermissionDiscoverer to filter operations based on OAuth scopes
	GetOpenAPISpec() (*openapi3.T, error)

	// GetEmbeddedSpecVersion returns the version of the embedded OpenAPI spec for cache validation
	GetEmbeddedSpecVersion() string

	// HealthCheck verifies the provider is accessible and functioning
	HealthCheck(ctx context.Context) error

	// Close cleans up any resources (connections, clients, etc.)
	Close() error
}

// ToolDefinition defines a tool provided by a StandardToolProvider
type ToolDefinition struct {
	Name        string         `json:"name"`
	DisplayName string         `json:"displayName"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Operation   OperationDef   `json:"operation"`
	Parameters  []ParameterDef `json:"parameters"`
	Examples    []ExampleDef   `json:"examples,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
}

// OperationDef defines an operation that can be executed
type OperationDef struct {
	ID           string            `json:"id"`
	Method       string            `json:"method"`
	PathTemplate string            `json:"pathTemplate"`
	Headers      map[string]string `json:"headers,omitempty"`
}

// ParameterDef defines a parameter for a tool
type ParameterDef struct {
	Name        string      `json:"name"`
	In          string      `json:"in"` // path, query, header, body
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Description string      `json:"description"`
	Default     interface{} `json:"default,omitempty"`
	Examples    []string    `json:"examples,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
}

// ExampleDef provides an example of tool usage
type ExampleDef struct {
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      interface{}            `json:"output,omitempty"`
}

// OperationHandler is a function type for handling internal operations
type OperationHandler func(ctx context.Context, params map[string]interface{}) (interface{}, error)

// OperationMapping maps a simplified operation name to its full specification
type OperationMapping struct {
	OperationID    string           `json:"operationId"`
	Method         string           `json:"method"`
	PathTemplate   string           `json:"pathTemplate"`
	RequiredParams []string         `json:"requiredParams"`
	OptionalParams []string         `json:"optionalParams,omitempty"`
	BodySchema     *json.RawMessage `json:"bodySchema,omitempty"`
	ResponseSchema *json.RawMessage `json:"responseSchema,omitempty"`
	Handler        OperationHandler `json:"-"` // For INTERNAL method type operations
}

// ProviderConfig contains configuration for a provider
type ProviderConfig struct {
	BaseURL         string                 `json:"baseUrl"`
	AuthType        string                 `json:"authType"`
	RequiredScopes  []string               `json:"requiredScopes,omitempty"`
	RateLimits      RateLimitConfig        `json:"rateLimits,omitempty"`
	OperationGroups []OperationGroup       `json:"operationGroups,omitempty"`
	CustomMappings  map[string]string      `json:"customMappings,omitempty"`
	DefaultHeaders  map[string]string      `json:"defaultHeaders,omitempty"`
	Timeout         time.Duration          `json:"timeout,omitempty"`
	RetryPolicy     *RetryPolicy           `json:"retryPolicy,omitempty"`
	HealthEndpoint  string                 `json:"healthEndpoint,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// RateLimitConfig defines rate limiting configuration
type RateLimitConfig struct {
	RequestsPerHour    int `json:"requestsPerHour,omitempty"`
	RequestsPerMinute  int `json:"requestsPerMinute,omitempty"`
	ConcurrentRequests int `json:"concurrentRequests,omitempty"`
}

// OperationGroup groups related operations together
type OperationGroup struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Operations  []string `json:"operations"`
}

// RetryPolicy defines retry configuration
type RetryPolicy struct {
	MaxRetries       int           `json:"maxRetries"`
	InitialDelay     time.Duration `json:"initialDelay"`
	MaxDelay         time.Duration `json:"maxDelay"`
	Multiplier       float64       `json:"multiplier"`
	RetryableErrors  []string      `json:"retryableErrors,omitempty"`
	RetryOnTimeout   bool          `json:"retryOnTimeout"`
	RetryOnRateLimit bool          `json:"retryOnRateLimit"`
}

// AIOptimizedToolDefinition provides AI-optimized tool definitions
// for better agent comprehension as specified in the plan
type AIOptimizedToolDefinition struct {
	// Core identification
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Category    string `json:"category"`
	Subcategory string `json:"subcategory,omitempty"`

	// Rich descriptions
	Description   string    `json:"description"`
	DetailedHelp  string    `json:"detailedHelp,omitempty"`
	UsageExamples []Example `json:"examples,omitempty"`

	// AI hints for better understanding
	SemanticTags  []string `json:"semanticTags,omitempty"`
	CommonPhrases []string `json:"commonPhrases,omitempty"`
	RelatedTools  []string `json:"relatedTools,omitempty"`

	// Simplified parameter schema
	InputSchema  AIParameterSchema `json:"inputSchema"`
	OutputSchema *ResponseSchema   `json:"outputSchema,omitempty"`

	// Capabilities and constraints
	Capabilities *ToolCapabilities `json:"capabilities,omitempty"`

	// Progressive disclosure levels
	ComplexityLevel string `json:"complexityLevel,omitempty"` // simple, moderate, complex
}

// AIParameterSchema provides AI-friendly parameter schema
type AIParameterSchema struct {
	Type       string                      `json:"type"`
	Properties map[string]AIPropertySchema `json:"properties"`
	Required   []string                    `json:"required,omitempty"`
	Examples   []map[string]interface{}    `json:"examples,omitempty"`
	AIHints    *AIParameterHints           `json:"aiHints,omitempty"`
}

// AIPropertySchema defines a property in an AI-friendly way
type AIPropertySchema struct {
	Type         string                      `json:"type"`
	Description  string                      `json:"description"`
	Examples     []interface{}               `json:"examples,omitempty"`
	Aliases      []string                    `json:"aliases,omitempty"`
	SmartDefault string                      `json:"smartDefault,omitempty"`
	Properties   map[string]AIPropertySchema `json:"properties,omitempty"` // For nested objects
	Required     []string                    `json:"required,omitempty"`   // For nested objects
	MinLength    int                         `json:"minLength,omitempty"`
	MaxLength    int                         `json:"maxLength,omitempty"`
	ItemType     string                      `json:"itemType,omitempty"` // For arrays
	Template     string                      `json:"template,omitempty"` // For strings with structure
}

// AIParameterHints provides hints to help AI understand parameters
type AIParameterHints struct {
	ParameterGrouping       map[string][]string      `json:"parameterGrouping,omitempty"`
	SmartDefaults           map[string]string        `json:"smartDefaults,omitempty"`
	ConditionalRequirements []ConditionalRequirement `json:"conditionalRequirements,omitempty"`
}

// ConditionalRequirement defines conditional parameter requirements
type ConditionalRequirement struct {
	If   string `json:"if"`
	Then string `json:"then"`
}

// Example provides a usage example
type Example struct {
	Scenario       string                 `json:"scenario"`
	Input          map[string]interface{} `json:"input"`
	ExpectedOutput interface{}            `json:"expectedOutput,omitempty"`
	Explanation    string                 `json:"explanation,omitempty"`
}

// ResponseSchema defines the expected response structure
type ResponseSchema struct {
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	Examples    []interface{}          `json:"examples,omitempty"`
}

// ToolCapabilities declares what a tool can and cannot do
type ToolCapabilities struct {
	Capabilities []Capability       `json:"capabilities"`
	Limitations  []Limitation       `json:"limitations,omitempty"`
	RateLimits   *RateLimitInfo     `json:"rateLimits,omitempty"`
	DataAccess   *DataAccessPattern `json:"dataAccess,omitempty"`
}

// Capability describes what the tool can do
type Capability struct {
	Action      string   `json:"action"`                // create, read, update, delete, list
	Resource    string   `json:"resource"`              // issues, pull_requests, repositories
	Constraints []string `json:"constraints,omitempty"` // own_repos_only, public_only
}

// Limitation describes what the tool cannot do
type Limitation struct {
	Description string `json:"description"`
	Workaround  string `json:"workaround,omitempty"`
}

// RateLimitInfo provides rate limit information
type RateLimitInfo struct {
	RequestsPerHour   int    `json:"requestsPerHour,omitempty"`
	RequestsPerMinute int    `json:"requestsPerMinute,omitempty"`
	Description       string `json:"description,omitempty"`
}

// DataAccessPattern describes how the tool accesses data
type DataAccessPattern struct {
	Pagination       bool     `json:"pagination"`
	MaxResults       int      `json:"maxResults,omitempty"`
	SupportedFilters []string `json:"supportedFilters,omitempty"`
	SupportedSorts   []string `json:"supportedSorts,omitempty"`
}

// ProviderError represents an error from a provider
type ProviderError struct {
	Provider    string         `json:"provider"`
	Code        string         `json:"code"`
	Message     string         `json:"message"`
	StatusCode  int            `json:"statusCode,omitempty"`
	RetryAfter  *time.Duration `json:"retryAfter,omitempty"`
	IsRetryable bool           `json:"isRetryable"`
}

func (e *ProviderError) Error() string {
	return e.Message
}
