package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGinMiddlewareWithPassthrough(t *testing.T) {
	// Set up
	gin.SetMode(gin.TestMode)
	logger := observability.NewNoopLogger()

	tests := []struct {
		name                 string
		apiKey               string
		keyType              KeyType
		allowedServices      []string
		userToken            string
		tokenProvider        string
		wantStatus           int
		wantPassthrough      bool
		wantError            string
		skipPassthroughCheck bool
	}{
		{
			name:            "non-gateway key - no passthrough",
			apiKey:          "usr_test123",
			keyType:         KeyTypeUser,
			allowedServices: []string{"github"},
			userToken:       "ghp_usertoken",
			tokenProvider:   "github",
			wantStatus:      http.StatusOK,
			wantPassthrough: false,
		},
		{
			name:            "gateway key with valid passthrough",
			apiKey:          "gw_test123",
			keyType:         KeyTypeGateway,
			allowedServices: []string{"github", "gitlab"},
			userToken:       "ghp_usertoken",
			tokenProvider:   "github",
			wantStatus:      http.StatusOK,
			wantPassthrough: true,
		},
		{
			name:            "gateway key without user token",
			apiKey:          "gw_test123",
			keyType:         KeyTypeGateway,
			allowedServices: []string{"github"},
			userToken:       "",
			tokenProvider:   "",
			wantStatus:      http.StatusOK,
			wantPassthrough: false,
		},
		{
			name:                 "gateway key with user token but no provider",
			apiKey:               "gw_test123",
			keyType:              KeyTypeGateway,
			allowedServices:      []string{"github"},
			userToken:            "ghp_usertoken",
			tokenProvider:        "",
			wantStatus:           http.StatusBadRequest,
			wantError:            "X-Token-Provider header required when using X-User-Token",
			skipPassthroughCheck: true,
		},
		{
			name:                 "gateway key with disallowed provider",
			apiKey:               "gw_test123",
			keyType:              KeyTypeGateway,
			allowedServices:      []string{"gitlab"},
			userToken:            "ghp_usertoken",
			tokenProvider:        "github",
			wantStatus:           http.StatusForbidden,
			wantError:            "Provider github not allowed for this gateway key",
			skipPassthroughCheck: true,
		},
		{
			name:                 "invalid API key",
			apiKey:               "invalid_key",
			keyType:              KeyTypeUser,
			allowedServices:      nil,
			userToken:            "",
			tokenProvider:        "",
			wantStatus:           http.StatusUnauthorized,
			wantError:            "Authentication required",
			skipPassthroughCheck: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create auth service with in-memory API key
			service := NewService(DefaultConfig(), nil, nil, logger)

			if tt.apiKey != "invalid_key" {
				// Add API key to in-memory storage
				service.apiKeys[tt.apiKey] = &APIKey{
					Key:             tt.apiKey,
					KeyType:         tt.keyType,
					TenantID:        uuid.MustParse("00000000-0000-0000-0000-000000000001"),
					UserID:          uuid.MustParse("00000000-0000-0000-0000-000000000002"),
					Name:            "Test Key",
					Scopes:          []string{"read", "write"},
					Active:          true,
					AllowedServices: tt.allowedServices,
				}
			}

			// Create router with middleware
			router := gin.New()
			router.Use(service.GinMiddlewareWithPassthrough(TypeAPIKey))

			// Add test endpoint
			router.GET("/test", func(c *gin.Context) {
				// Check if passthrough token is in context
				passthroughToken, hasPassthrough := GetPassthroughToken(c.Request.Context())

				// If not found in request context, check Gin context
				if !hasPassthrough {
					if tokenInterface, exists := c.Get("passthrough_token"); exists {
						if token, ok := tokenInterface.(PassthroughToken); ok {
							passthroughToken = &token
							hasPassthrough = true
						}
					}
				}

				c.JSON(http.StatusOK, gin.H{
					"has_passthrough": hasPassthrough,
					"token":           passthroughToken,
				})
			})

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", "Bearer "+tt.apiKey)
			if tt.userToken != "" {
				req.Header.Set("X-User-Token", tt.userToken)
			}
			if tt.tokenProvider != "" {
				req.Header.Set("X-Token-Provider", tt.tokenProvider)
			}

			// Execute request
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Verify response
			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.wantError != "" {
				assert.Contains(t, w.Body.String(), tt.wantError)
			} else if !tt.skipPassthroughCheck && w.Code == http.StatusOK {
				// Parse response
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				hasPassthrough, _ := response["has_passthrough"].(bool)
				assert.Equal(t, tt.wantPassthrough, hasPassthrough)

				if tt.wantPassthrough {
					token, ok := response["token"].(map[string]interface{})
					require.True(t, ok)
					assert.Equal(t, tt.tokenProvider, token["Provider"])
					assert.Equal(t, tt.userToken, token["Token"])
				}
			}
		})
	}
}

func TestStandardMiddlewareWithPassthrough(t *testing.T) {
	logger := observability.NewNoopLogger()

	tests := []struct {
		name            string
		apiKey          string
		keyType         KeyType
		allowedServices []string
		userToken       string
		tokenProvider   string
		wantStatus      int
		wantPassthrough bool
	}{
		{
			name:            "gateway key with valid passthrough",
			apiKey:          "gw_test123",
			keyType:         KeyTypeGateway,
			allowedServices: []string{"github"},
			userToken:       "ghp_usertoken",
			tokenProvider:   "github",
			wantStatus:      http.StatusOK,
			wantPassthrough: true,
		},
		{
			name:            "non-gateway key",
			apiKey:          "usr_test123",
			keyType:         KeyTypeUser,
			allowedServices: []string{"github"},
			userToken:       "ghp_usertoken",
			tokenProvider:   "github",
			wantStatus:      http.StatusOK,
			wantPassthrough: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create auth service
			service := NewService(DefaultConfig(), nil, nil, logger)

			// Add API key
			service.apiKeys[tt.apiKey] = &APIKey{
				Key:             tt.apiKey,
				KeyType:         tt.keyType,
				TenantID:        uuid.MustParse("00000000-0000-0000-0000-000000000001"),
				UserID:          uuid.MustParse("00000000-0000-0000-0000-000000000002"),
				Name:            "Test Key",
				Scopes:          []string{"read"},
				Active:          true,
				AllowedServices: tt.allowedServices,
			}

			// Create handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				passthroughToken, hasPassthrough := GetPassthroughToken(r.Context())

				response := map[string]interface{}{
					"has_passthrough": hasPassthrough,
				}

				if hasPassthrough {
					response["provider"] = passthroughToken.Provider
					response["has_token"] = passthroughToken.Token != ""
				}

				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			})

			// Wrap with middleware
			middleware := service.StandardMiddlewareWithPassthrough(TypeAPIKey)
			wrappedHandler := middleware(handler)

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", "Bearer "+tt.apiKey)
			if tt.userToken != "" {
				req.Header.Set("X-User-Token", tt.userToken)
			}
			if tt.tokenProvider != "" {
				req.Header.Set("X-Token-Provider", tt.tokenProvider)
			}

			// Execute request
			w := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(w, req)

			// Verify
			assert.Equal(t, tt.wantStatus, w.Code)

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				hasPassthrough, _ := response["has_passthrough"].(bool)
				assert.Equal(t, tt.wantPassthrough, hasPassthrough)
			}
		})
	}
}
