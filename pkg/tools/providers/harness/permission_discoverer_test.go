package harness

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/stretchr/testify/assert"
)

func TestNewHarnessPermissionDiscoverer(t *testing.T) {
	logger := &observability.NoopLogger{}
	discoverer := NewHarnessPermissionDiscoverer(logger)

	assert.NotNil(t, discoverer)
	assert.NotNil(t, discoverer.baseDiscoverer)
	assert.NotNil(t, discoverer.logger)
	assert.NotNil(t, discoverer.httpClient)
}

func TestHarnessPermissionDiscoverer_DiscoverPermissions(t *testing.T) {
	tests := []struct {
		name        string
		apiKey      string
		responses   map[string]*httpResponse
		wantScopes  []string
		wantModules map[string]bool
	}{
		{
			name:   "successful discovery with all modules",
			apiKey: "pat.account123.key456",
			responses: map[string]*httpResponse{
				"/gateway/ng/api/user/currentUser": {
					status: http.StatusOK,
					body: `{
						"data": {
							"email": "user@example.com",
							"name": "Test User",
							"uuid": "user-uuid",
							"defaultAccountId": "account123"
						}
					}`,
				},
				"/pipeline/api/pipelines/list": {
					status: http.StatusOK,
					body:   `{"pipelines": []}`,
				},
				"/v1/orgs": {
					status: http.StatusOK,
					body: `{
						"orgs": [
							{"identifier": "org1", "name": "Org 1"},
							{"identifier": "org2", "name": "Org 2"}
						]
					}`,
				},
				"/ng/api/connectors/listV2": {
					status: http.StatusOK,
					body:   `{"connectors": []}`,
				},
				"/cv/api/monitored-service/list": {
					status: http.StatusForbidden,
					body:   `{"message": "Forbidden"}`,
				},
			},
			wantScopes: []string{"module:pipeline", "module:project", "module:connector", "org:org1", "org:org2"},
			wantModules: map[string]bool{
				"pipeline":  true,
				"project":   true,
				"connector": true,
				"cv":        false,
			},
		},
		{
			name:   "api key without account ID",
			apiKey: "some-other-format-key",
			responses: map[string]*httpResponse{
				"/gateway/ng/api/user/currentUser": {
					status: http.StatusOK,
					body: `{
						"data": {
							"email": "user@example.com",
							"name": "Test User",
							"defaultAccountId": "default-account"
						}
					}`,
				},
			},
			wantScopes: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check auth header
				assert.Equal(t, tt.apiKey, r.Header.Get("x-api-key"))

				// Find matching response
				for path, resp := range tt.responses {
					if r.URL.Path == path || containsString(r.URL.String(), path) {
						w.WriteHeader(resp.status)
						if _, err := w.Write([]byte(resp.body)); err != nil {
							t.Errorf("Failed to write response: %v", err)
						}
						return
					}
				}

				// Default response
				w.WriteHeader(http.StatusNotFound)
				if _, err := w.Write([]byte(`{"message": "Not found"}`)); err != nil {
					t.Errorf("Failed to write response: %v", err)
				}
			}))
			defer server.Close()

			logger := &observability.NoopLogger{}
			discoverer := NewHarnessPermissionDiscoverer(logger)

			// Override the HTTP client to use test server
			discoverer.httpClient = &http.Client{
				Transport: &testTransport{
					server: server,
				},
			}

			ctx := context.Background()
			perms, err := discoverer.DiscoverPermissions(ctx, tt.apiKey)

			assert.NoError(t, err)
			assert.NotNil(t, perms)

			// Check discovered scopes
			if len(tt.wantScopes) > 0 {
				for _, scope := range tt.wantScopes {
					assert.Contains(t, perms.Scopes, scope, "Expected scope %s to be discovered", scope)
				}
			}

			// Check enabled modules
			if tt.wantModules != nil {
				for module, enabled := range tt.wantModules {
					assert.Equal(t, enabled, perms.EnabledModules[module],
						"Module %s should be enabled=%v", module, enabled)
				}
			}
		})
	}
}

func TestHarnessPermissionDiscoverer_FilterOperationsByPermissions(t *testing.T) {
	logger := &observability.NoopLogger{}
	discoverer := NewHarnessPermissionDiscoverer(logger)

	permissions := &HarnessPermissions{
		DiscoveredPermissions: &tools.DiscoveredPermissions{
			Scopes: []string{"module:pipeline", "module:project"},
		},
		EnabledModules: map[string]bool{
			"pipeline":  true,
			"project":   true,
			"connector": false,
			"gitops":    false,
		},
		ResourceAccess: map[string][]string{
			"pipeline": {"create", "list", "execute"},
			"project":  {"list", "read"},
		},
	}

	operationMappings := map[string]interface{}{
		"pipelines/list":     struct{}{},
		"pipelines/create":   struct{}{},
		"pipelines/delete":   struct{}{},
		"pipelines/execute":  struct{}{},
		"projects/list":      struct{}{},
		"projects/create":    struct{}{},
		"connectors/list":    struct{}{},
		"gitops/agents/list": struct{}{},
		"unknown/operation":  struct{}{},
	}

	allowed := discoverer.FilterOperationsByPermissions(operationMappings, permissions)

	// Check allowed operations
	assert.True(t, allowed["pipelines/list"], "pipelines/list should be allowed")
	assert.True(t, allowed["pipelines/create"], "pipelines/create should be allowed")
	assert.False(t, allowed["pipelines/delete"], "pipelines/delete should not be allowed (no delete permission)")
	assert.True(t, allowed["pipelines/execute"], "pipelines/execute should be allowed")
	assert.True(t, allowed["projects/list"], "projects/list should be allowed")
	assert.False(t, allowed["projects/create"], "projects/create should not be allowed (no create permission)")
	assert.False(t, allowed["connectors/list"], "connectors/list should not be allowed (module disabled)")
	assert.False(t, allowed["gitops/agents/list"], "gitops/agents/list should not be allowed (module disabled)")
}

func TestHarnessPermissionDiscoverer_probeEndpoint(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		method       string
		responseCode int
		expectAccess bool
	}{
		{
			name:         "successful GET",
			endpoint:     "/test/endpoint",
			method:       "GET",
			responseCode: http.StatusOK,
			expectAccess: true,
		},
		{
			name:         "successful POST",
			endpoint:     "/test/endpoint",
			method:       "POST",
			responseCode: http.StatusOK,
			expectAccess: true,
		},
		{
			name:         "forbidden access",
			endpoint:     "/test/endpoint",
			method:       "GET",
			responseCode: http.StatusForbidden,
			expectAccess: false,
		},
		{
			name:         "bad request but has access",
			endpoint:     "/test/endpoint",
			method:       "GET",
			responseCode: http.StatusBadRequest,
			expectAccess: true,
		},
		{
			name:         "not found but has access",
			endpoint:     "/test/endpoint",
			method:       "GET",
			responseCode: http.StatusNotFound,
			expectAccess: true,
		},
		{
			name:         "server error",
			endpoint:     "/test/endpoint",
			method:       "GET",
			responseCode: http.StatusInternalServerError,
			expectAccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.method, r.Method)
				assert.Equal(t, "test-api-key", r.Header.Get("x-api-key"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				w.WriteHeader(tt.responseCode)
				if _, err := w.Write([]byte(`{}`)); err != nil {
					t.Errorf("Failed to write response: %v", err)
				}
			}))
			defer server.Close()

			logger := &observability.NoopLogger{}
			discoverer := NewHarnessPermissionDiscoverer(logger)

			// Override the HTTP client to use test server
			discoverer.httpClient = &http.Client{
				Transport: &testTransport{
					server: server,
				},
			}

			ctx := context.Background()
			hasAccess := discoverer.probeEndpoint(ctx, tt.endpoint, tt.method, "test-api-key", "account123")

			assert.Equal(t, tt.expectAccess, hasAccess,
				"Expected access=%v for status %d", tt.expectAccess, tt.responseCode)
		})
	}
}

// Helper types and functions

type httpResponse struct {
	status int
	body   string
}

type testTransport struct {
	server *httptest.Server
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the URL to point to test server
	req.URL.Scheme = "http"
	req.URL.Host = t.server.Listener.Addr().String()
	return http.DefaultTransport.RoundTrip(req)
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[len(s)-len(substr):] == substr ||
			s[:len(substr)] == substr ||
			len(s) > len(substr) && containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 1; i < len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
