package confluence

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
)

func TestHealthCheck(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		wantErr     bool
		errContains string
	}{
		{
			name:       "successful health check - 200 OK",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "successful health check - 401 Unauthorized (API accessible)",
			statusCode: http.StatusUnauthorized,
			wantErr:    false,
		},
		{
			name:        "failed health check - 500 Internal Server Error",
			statusCode:  http.StatusInternalServerError,
			wantErr:     true,
			errContains: "unexpected status",
		},
		{
			name:        "failed health check - 404 Not Found",
			statusCode:  http.StatusNotFound,
			wantErr:     true,
			errContains: "unexpected status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check that the request is to the health check endpoint
				assert.Contains(t, r.URL.Path, "/wiki/rest/api/space")
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Accept"))

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					_, _ = w.Write([]byte(`{"results":[]}`))
				}
			}))
			defer server.Close()

			// Create provider
			logger := &observability.NoopLogger{}
			provider := NewConfluenceProvider(logger, "test-domain")

			// Override HTTP client to use test server
			provider.httpClient = &http.Client{
				Transport: &testTransport{
					serverURL:  server.URL,
					httpClient: server.Client(),
				},
			}

			// Test health check
			err := provider.HealthCheck(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHealthCheckNetworkError(t *testing.T) {
	// Create provider
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	// Override HTTP client to simulate network error
	provider.httpClient = &http.Client{
		Transport: &errorTransport{
			err: assert.AnError,
		},
	}

	// Test health check
	err := provider.HealthCheck(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}

func TestClose(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	// Test Close method
	err := provider.Close()
	assert.NoError(t, err)
}

// errorTransport is a mock transport that always returns an error
type errorTransport struct {
	err error
}

func (t *errorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, t.err
}
