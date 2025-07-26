package tools

import (
	"fmt"
	"net/http"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/getkin/kin-openapi/openapi3"
)

// DynamicAuthenticator handles authentication based on OpenAPI security schemes
type DynamicAuthenticator struct{}

// NewDynamicAuthenticator creates a new dynamic authenticator
func NewDynamicAuthenticator() *DynamicAuthenticator {
	return &DynamicAuthenticator{}
}

// ApplyAuthentication applies authentication based on credential configuration
func (a *DynamicAuthenticator) ApplyAuthentication(req *http.Request, creds *models.TokenCredential) error {
	if creds == nil {
		return fmt.Errorf("no credentials provided")
	}

	// Apply based on credential type and configuration
	switch creds.Type {
	case "bearer":
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", creds.Token))

	case "api_key":
		// API key can be in header or query
		if creds.HeaderName != "" {
			req.Header.Set(creds.HeaderName, creds.Token)
		} else if creds.QueryParam != "" {
			q := req.URL.Query()
			q.Set(creds.QueryParam, creds.Token)
			req.URL.RawQuery = q.Encode()
		} else {
			// Default to X-API-Key header
			req.Header.Set("X-API-Key", creds.Token)
		}

	case "basic":
		req.SetBasicAuth(creds.Username, creds.Password)

	case "custom_header":
		if creds.HeaderName == "" {
			return fmt.Errorf("header name required for custom header auth")
		}
		value := creds.Token
		if creds.HeaderPrefix != "" {
			value = creds.HeaderPrefix + " " + value
		}
		req.Header.Set(creds.HeaderName, value)

	case "oauth2":
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", creds.Token))

	default:
		return fmt.Errorf("unsupported authentication type: %s", creds.Type)
	}

	return nil
}

// ExtractSecuritySchemes extracts security schemes from OpenAPI spec
func (a *DynamicAuthenticator) ExtractSecuritySchemes(spec *openapi3.T) map[string]SecurityScheme {
	schemes := make(map[string]SecurityScheme)

	if spec == nil || spec.Components == nil {
		return schemes
	}

	// Extract security schemes from components
	for name, schemeRef := range spec.Components.SecuritySchemes {
		if schemeRef == nil || schemeRef.Value == nil {
			continue
		}

		scheme := schemeRef.Value
		secScheme := SecurityScheme{
			Name:        name,
			Type:        scheme.Type,
			Description: scheme.Description,
		}

		// Handle different security scheme types
		switch scheme.Type {
		case "apiKey":
			secScheme.In = scheme.In
			secScheme.ParamName = scheme.Name

		case "http":
			secScheme.Scheme = scheme.Scheme
			secScheme.BearerFormat = scheme.BearerFormat

		case "oauth2":
			if scheme.Flows != nil {
				secScheme.OAuth2Flows = extractOAuth2Flows(scheme.Flows)
			}

		case "openIdConnect":
			secScheme.OpenIDConnectURL = scheme.OpenIdConnectUrl
		}

		schemes[name] = secScheme
	}

	return schemes
}

// SecurityScheme represents an OpenAPI security scheme
type SecurityScheme struct {
	Type             string                `json:"type"`
	Name             string                `json:"name"`
	Description      string                `json:"description,omitempty"`
	In               string                `json:"in,omitempty"`               // For apiKey
	ParamName        string                `json:"paramName,omitempty"`        // For apiKey
	Scheme           string                `json:"scheme,omitempty"`           // For http
	BearerFormat     string                `json:"bearerFormat,omitempty"`     // For http bearer
	OAuth2Flows      map[string]OAuth2Flow `json:"flows,omitempty"`            // For oauth2
	OpenIDConnectURL string                `json:"openIdConnectUrl,omitempty"` // For openIdConnect
}

// OAuth2Flow represents an OAuth2 flow
type OAuth2Flow struct {
	AuthorizationURL string            `json:"authorizationUrl,omitempty"`
	TokenURL         string            `json:"tokenUrl,omitempty"`
	RefreshURL       string            `json:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes,omitempty"`
}

// extractOAuth2Flows extracts OAuth2 flows from OpenAPI spec
func extractOAuth2Flows(flows *openapi3.OAuthFlows) map[string]OAuth2Flow {
	result := make(map[string]OAuth2Flow)

	if flows.Implicit != nil {
		result["implicit"] = OAuth2Flow{
			AuthorizationURL: flows.Implicit.AuthorizationURL,
			Scopes:           flows.Implicit.Scopes,
		}
	}

	if flows.Password != nil {
		result["password"] = OAuth2Flow{
			TokenURL: flows.Password.TokenURL,
			Scopes:   flows.Password.Scopes,
		}
	}

	if flows.ClientCredentials != nil {
		result["clientCredentials"] = OAuth2Flow{
			TokenURL: flows.ClientCredentials.TokenURL,
			Scopes:   flows.ClientCredentials.Scopes,
		}
	}

	if flows.AuthorizationCode != nil {
		result["authorizationCode"] = OAuth2Flow{
			AuthorizationURL: flows.AuthorizationCode.AuthorizationURL,
			TokenURL:         flows.AuthorizationCode.TokenURL,
			RefreshURL:       flows.AuthorizationCode.RefreshURL,
			Scopes:           flows.AuthorizationCode.Scopes,
		}
	}

	return result
}

// DetermineAuthType determines the best auth type from OpenAPI security schemes
func (a *DynamicAuthenticator) DetermineAuthType(schemes map[string]SecurityScheme) string {
	// Priority order for auth types
	for _, scheme := range schemes {
		switch scheme.Type {
		case "http":
			switch scheme.Scheme {
			case "bearer":
				return "bearer"
			case "basic":
				return "basic"
			}
		case "apiKey":
			return "api_key"
		case "oauth2":
			return "oauth2"
		}
	}

	// Default to bearer token
	return "bearer"
}

// BuildCredentialTemplate builds a credential template from security schemes
func (a *DynamicAuthenticator) BuildCredentialTemplate(schemes map[string]SecurityScheme) CredentialTemplate {
	template := CredentialTemplate{
		SupportedTypes: []string{},
		Fields:         []CredentialField{},
	}

	// Analyze schemes to determine supported types and required fields
	for _, scheme := range schemes {
		switch scheme.Type {
		case "http":
			switch scheme.Scheme {
			case "bearer":
				template.SupportedTypes = append(template.SupportedTypes, "bearer")
				template.Fields = append(template.Fields, CredentialField{
					Name:        "token",
					Type:        "string",
					Required:    true,
					Description: "Bearer token for authentication",
				})
			case "basic":
				template.SupportedTypes = append(template.SupportedTypes, "basic")
				template.Fields = append(template.Fields,
					CredentialField{
						Name:        "username",
						Type:        "string",
						Required:    true,
						Description: "Username for basic authentication",
					},
					CredentialField{
						Name:        "password",
						Type:        "string",
						Required:    true,
						Sensitive:   true,
						Description: "Password for basic authentication",
					},
				)
			}

		case "apiKey":
			template.SupportedTypes = append(template.SupportedTypes, "api_key")

			field := CredentialField{
				Name:        "token",
				Type:        "string",
				Required:    true,
				Description: fmt.Sprintf("API key for %s authentication", scheme.Name),
			}

			// Add metadata about where the key goes
			switch scheme.In {
			case "header":
				field.Metadata = map[string]string{
					"header_name": scheme.ParamName,
				}
			case "query":
				field.Metadata = map[string]string{
					"query_param": scheme.ParamName,
				}
			}

			template.Fields = append(template.Fields, field)

		case "oauth2":
			template.SupportedTypes = append(template.SupportedTypes, "oauth2")
			template.Fields = append(template.Fields, CredentialField{
				Name:        "token",
				Type:        "string",
				Required:    true,
				Description: "OAuth2 access token",
			})

			// Add OAuth2 metadata
			if flows, ok := scheme.OAuth2Flows["clientCredentials"]; ok {
				template.OAuth2Config = &OAuth2Config{
					TokenURL: flows.TokenURL,
					Scopes:   flows.Scopes,
				}
			}
		}
	}

	// Remove duplicates
	template.SupportedTypes = removeDuplicates(template.SupportedTypes)

	return template
}

// CredentialTemplate describes how to configure credentials for a tool
type CredentialTemplate struct {
	SupportedTypes []string          `json:"supported_types"`
	Fields         []CredentialField `json:"fields"`
	OAuth2Config   *OAuth2Config     `json:"oauth2_config,omitempty"`
}

// CredentialField describes a credential field
type CredentialField struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Required    bool              `json:"required"`
	Sensitive   bool              `json:"sensitive"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// OAuth2Config contains OAuth2 configuration
type OAuth2Config struct {
	TokenURL string            `json:"token_url"`
	Scopes   map[string]string `json:"scopes"`
}

// removeDuplicates removes duplicate strings from a slice
func removeDuplicates(items []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}
