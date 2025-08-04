package auth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockOAuthProvider implements OAuthProvider for testing
type MockOAuthProvider struct {
	*auth.BaseOAuthProvider
	mockServer *httptest.Server
}

// NewMockOAuthProvider creates a new mock OAuth provider
func NewMockOAuthProvider(t *testing.T) *MockOAuthProvider {
	provider := &MockOAuthProvider{
		BaseOAuthProvider: &auth.BaseOAuthProvider{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
		},
	}

	// Create mock server
	provider.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			provider.handleTokenRequest(w, r)
		case "/oauth/userinfo":
			provider.handleUserInfo(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	provider.AuthURL = provider.mockServer.URL + "/oauth/authorize"
	provider.TokenURL = provider.mockServer.URL + "/oauth/token"
	provider.UserInfoURL = provider.mockServer.URL + "/oauth/userinfo"

	return provider
}

// Close closes the mock server
func (m *MockOAuthProvider) Close() {
	m.mockServer.Close()
}

// ExchangeCode mocks code exchange
func (m *MockOAuthProvider) ExchangeCode(ctx context.Context, code, redirectURI string) (*auth.OAuthToken, error) {
	if code == "valid-code" {
		return &auth.OAuthToken{
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
			TokenType:    "Bearer",
			ExpiresAt:    time.Now().Add(time.Hour),
		}, nil
	}
	return nil, fmt.Errorf("invalid code")
}

// ExchangeCodeWithPKCE mocks PKCE code exchange
func (m *MockOAuthProvider) ExchangeCodeWithPKCE(ctx context.Context, code, redirectURI, codeVerifier string) (*auth.OAuthToken, error) {
	if code == "valid-code" && codeVerifier == "verifier" {
		return &auth.OAuthToken{
			AccessToken:  "new-access-token-pkce",
			RefreshToken: "new-refresh-token-pkce",
			TokenType:    "Bearer",
			ExpiresAt:    time.Now().Add(time.Hour),
		}, nil
	}
	return nil, fmt.Errorf("invalid code or verifier")
}

// RefreshToken mocks token refresh
func (m *MockOAuthProvider) RefreshToken(ctx context.Context, refreshToken string) (*auth.OAuthToken, error) {
	if refreshToken == "valid-refresh-token" {
		return &auth.OAuthToken{
			AccessToken:  "refreshed-access-token",
			RefreshToken: "new-refresh-token",
			TokenType:    "Bearer",
			ExpiresAt:    time.Now().Add(time.Hour),
		}, nil
	}
	if refreshToken == "expired-refresh-token" {
		return nil, fmt.Errorf("refresh token expired")
	}
	return nil, fmt.Errorf("invalid refresh token")
}

// ValidateToken mocks token validation
func (m *MockOAuthProvider) ValidateToken(ctx context.Context, accessToken string) (*auth.OAuthUserInfo, error) {
	switch accessToken {
	case "valid-access-token":
		return &auth.OAuthUserInfo{
			ID:    "user-123",
			Email: "user@example.com",
			Name:  "Test User",
		}, nil
	case "expired-token":
		return nil, fmt.Errorf("token expired")
	default:
		return nil, fmt.Errorf("invalid token")
	}
}

// Mock server handlers
func (m *MockOAuthProvider) handleTokenRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	code := r.FormValue("code")
	grantType := r.FormValue("grant_type")

	var response map[string]interface{}

	switch {
	case code == "valid-code" && grantType == "authorization_code":
		response = map[string]interface{}{
			"access_token":  "new-access-token",
			"refresh_token": "new-refresh-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
		}
	case code == "invalid-grant":
		w.WriteHeader(http.StatusBadRequest)
		response = map[string]interface{}{
			"error":             "invalid_grant",
			"error_description": "The provided authorization grant is invalid",
		}
	default:
		w.WriteHeader(http.StatusBadRequest)
		response = map[string]interface{}{
			"error": "invalid_request",
		}
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (m *MockOAuthProvider) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")

	switch authHeader {
	case "Bearer valid-access-token":
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "user-123",
			"email": "user@example.com",
			"name":  "Test User",
		}); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	case "Bearer expired-token":
		w.WriteHeader(http.StatusUnauthorized)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "token_expired",
		}); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	default:
		w.WriteHeader(http.StatusUnauthorized)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "invalid_token",
		}); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}

// TestOAuthProviderComprehensive tests OAuth provider functionality
func TestOAuthProviderComprehensive(t *testing.T) {
	provider := NewMockOAuthProvider(t)
	defer provider.Close()

	t.Run("Authorization Flow", func(t *testing.T) {
		// Test authorization URL generation
		authURL := provider.GetAuthorizationURL("test-state", "http://localhost/callback")
		assert.Contains(t, authURL, "client_id=test-client-id")
		assert.Contains(t, authURL, "state=test-state")
		assert.Contains(t, authURL, "redirect_uri=http%3A%2F%2Flocalhost%2Fcallback")

		// Test PKCE support
		authURLWithPKCE := provider.GetAuthorizationURLWithPKCE("test-state", "http://localhost/callback", "challenge")
		assert.Contains(t, authURLWithPKCE, "code_challenge=challenge")
		assert.Contains(t, authURLWithPKCE, "code_challenge_method=S256")
	})

	t.Run("Token Exchange", func(t *testing.T) {
		ctx := context.Background()

		// Test successful token exchange
		token, err := provider.ExchangeCode(ctx, "valid-code", "http://localhost/callback")
		require.NoError(t, err)
		assert.NotEmpty(t, token.AccessToken)
		assert.NotEmpty(t, token.RefreshToken)
		assert.True(t, token.ExpiresAt.After(time.Now()))

		// Test with PKCE
		tokenWithPKCE, err := provider.ExchangeCodeWithPKCE(ctx, "valid-code", "http://localhost/callback", "verifier")
		require.NoError(t, err)
		assert.NotEmpty(t, tokenWithPKCE.AccessToken)
	})

	t.Run("Token Refresh", func(t *testing.T) {
		ctx := context.Background()

		// Test successful refresh
		newToken, err := provider.RefreshToken(ctx, "valid-refresh-token")
		require.NoError(t, err)
		assert.NotEmpty(t, newToken.AccessToken)
		assert.NotEqual(t, "valid-refresh-token", newToken.AccessToken)

		// Test expired refresh token
		_, err = provider.RefreshToken(ctx, "expired-refresh-token")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("Token Validation", func(t *testing.T) {
		ctx := context.Background()

		// Test valid token
		userInfo, err := provider.ValidateToken(ctx, "valid-access-token")
		require.NoError(t, err)
		assert.NotEmpty(t, userInfo.ID)
		assert.NotEmpty(t, userInfo.Email)

		// Test invalid token
		_, err = provider.ValidateToken(ctx, "invalid-token")
		assert.Error(t, err)

		// Test expired token
		_, err = provider.ValidateToken(ctx, "expired-token")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("State Validation", func(t *testing.T) {
		assert.True(t, provider.ValidateState("valid-state", "valid-state"))
		assert.False(t, provider.ValidateState("state1", "state2"))
	})
}

// MockCache implements a simple in-memory cache for testing
type MockCache struct {
	data map[string]interface{}
}

func NewMockCache() *MockCache {
	return &MockCache{
		data: make(map[string]interface{}),
	}
}

func (m *MockCache) Get(ctx context.Context, key string, value interface{}) error {
	if v, ok := m.data[key]; ok {
		// Simple type assertion - in real implementation would use reflection
		switch dst := value.(type) {
		case *bool:
			if b, ok := v.(bool); ok {
				*dst = b
				return nil
			}
		case *int:
			if i, ok := v.(int); ok {
				*dst = i
				return nil
			}
		case *string:
			if s, ok := v.(string); ok {
				*dst = s
				return nil
			}
		}
	}
	return fmt.Errorf("key not found")
}

func (m *MockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.data[key] = value
	return nil
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *MockCache) Exists(ctx context.Context, key string) (bool, error) {
	_, ok := m.data[key]
	return ok, nil
}

func (m *MockCache) Flush(ctx context.Context) error {
	m.data = make(map[string]interface{})
	return nil
}

func (m *MockCache) Close() error {
	return nil
}

// TestAuthenticationIntegration tests the full authentication stack
func TestAuthenticationIntegration(t *testing.T) {
	// Setup
	logger := observability.NewNoopLogger()
	metrics := observability.NewNoOpMetricsClient()
	testCache := NewMockCache()

	// Create base service
	config := auth.DefaultConfig()
	config.JWTSecret = "test-secret"
	config.CacheEnabled = false // Disable cache for testing
	baseService := auth.NewService(config, nil, testCache, logger)

	// Create rate limiter
	rateLimiter := auth.NewRateLimiter(testCache, logger, &auth.RateLimiterConfig{
		Enabled:       true,
		MaxAttempts:   3,
		WindowSize:    1 * time.Minute,
		LockoutPeriod: 5 * time.Minute,
	})

	// Create metrics and audit
	metricsCollector := auth.NewMetricsCollector(metrics)
	auditLogger := auth.NewAuditLogger(logger)

	// Create auth middleware
	authMiddleware := auth.NewAuthMiddleware(baseService, rateLimiter, metricsCollector, auditLogger)

	// Test API key validation with rate limiting
	t.Run("API Key Rate Limiting", func(t *testing.T) {
		ctx := context.Background()

		// Create test API key
		tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		userID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
		apiKey, err := baseService.CreateAPIKey(ctx, tenantID, userID, "test-key", []string{"read"}, nil)
		require.NoError(t, err)

		// Add IP to context
		ctx = context.WithValue(ctx, auth.ContextKeyIPAddress, "127.0.0.1")

		// Successful validations
		for i := 0; i < 3; i++ {
			user, err := authMiddleware.ValidateAPIKeyWithMetrics(ctx, apiKey.Key)
			require.NoError(t, err)
			assert.Equal(t, userID, user.ID)
		}

		// Failed validations (wrong key)
		for i := 0; i < 3; i++ {
			_, err := authMiddleware.ValidateAPIKeyWithMetrics(ctx, "wrong-key")
			assert.Error(t, err)
		}

		// Should be rate limited now
		_, err = authMiddleware.ValidateAPIKeyWithMetrics(ctx, "wrong-key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rate limit")
	})

	// Test middleware integration
	t.Run("Middleware Integration", func(t *testing.T) {
		// Create test handler that simulates auth failures
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate authentication failure
			w.WriteHeader(http.StatusUnauthorized)
		})

		// Wrap with auth context middleware
		wrapped := auth.HTTPAuthMiddleware()(handler)

		// Wrap with rate limit middleware
		wrapped = auth.RateLimitMiddleware(rateLimiter, logger)(wrapped)

		// Test requests
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest("POST", "/auth/login", nil)
			req.RemoteAddr = "127.0.0.1:12345"
			req.Header.Set("X-Forwarded-For", "192.168.1.1")

			rr := httptest.NewRecorder()
			wrapped.ServeHTTP(rr, req)

			if i < 3 {
				// First 3 requests should get through to handler (auth failure)
				assert.Equal(t, http.StatusUnauthorized, rr.Code)
			} else {
				// After 3 failed attempts, should be rate limited
				assert.Equal(t, http.StatusTooManyRequests, rr.Code)
				assert.NotEmpty(t, rr.Header().Get("Retry-After"))
			}
		}
	})
}
