package tools

import "context"

// Tool represents a tool instance with its definition and handler
type Tool struct {
	Definition ToolDefinition
	Handler    ToolHandler
}

// ToolDefinition represents a tool that can be used by an AI model
type ToolDefinition struct {
	// Name is the unique identifier for this tool
	Name string `json:"name"`

	// Description is a human-readable description of the tool
	Description string `json:"description"`

	// Parameters defines the schema for the tool's parameters
	Parameters ParameterSchema `json:"parameters"`

	// Returns defines the schema for the tool's return value
	Returns ReturnSchema `json:"returns,omitempty"`

	// Examples provides usage examples for the tool
	Examples []ToolExample `json:"examples,omitempty"`

	// Tags for categorizing and filtering tools
	Tags []string `json:"tags,omitempty"`

	// IsAsync indicates if the tool is asynchronous
	IsAsync bool `json:"is_async,omitempty"`
}

// ParameterSchema defines the schema for a tool's parameters
type ParameterSchema struct {
	// Type is the schema type (usually "object")
	Type string `json:"type"`

	// Properties defines the parameters as property schemas
	Properties map[string]PropertySchema `json:"properties"`

	// Required lists the names of required parameters
	Required []string `json:"required,omitempty"`
}

// PropertySchema defines the schema for a parameter property
type PropertySchema struct {
	// Type is the data type of the property
	Type string `json:"type"`

	// Description of the property
	Description string `json:"description,omitempty"`

	// Enum defines allowed values
	Enum []interface{} `json:"enum,omitempty"`

	// Items defines schema for array items
	Items *PropertySchema `json:"items,omitempty"`

	// Properties for nested objects
	Properties map[string]PropertySchema `json:"properties,omitempty"`

	// Required fields for nested objects
	Required []string `json:"required,omitempty"`

	// Default value
	Default interface{} `json:"default,omitempty"`

	// Pattern for string validation
	Pattern string `json:"pattern,omitempty"`

	// Minimum value for numbers
	Minimum *float64 `json:"minimum,omitempty"`

	// Maximum value for numbers
	Maximum *float64 `json:"maximum,omitempty"`
}

// ReturnSchema defines the schema for a tool's return value
type ReturnSchema struct {
	// Type is the data type of the return value
	Type string `json:"type"`

	// Description of the return value
	Description string `json:"description,omitempty"`

	// Properties for object return types
	Properties map[string]PropertySchema `json:"properties,omitempty"`

	// Items for array return types
	Items *PropertySchema `json:"items,omitempty"`
}

// ToolExample provides an example of how to use a tool
type ToolExample struct {
	// Description of the example
	Description string `json:"description"`

	// Input parameters for the example
	Input map[string]interface{} `json:"input"`

	// Expected output from the example
	Output interface{} `json:"output,omitempty"`
}

// ToolHandler is a function that handles tool execution
type ToolHandler func(ctx context.Context, params map[string]interface{}) (interface{}, error)
