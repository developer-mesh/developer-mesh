# OAuth Providers Implementation Guide

> **Status**: Implementation Guide
> **Complexity**: Medium
> **Estimated Effort**: 1-2 weeks per provider
> **Dependencies**: OAuth2 libraries, session management

## Overview

This guide provides comprehensive instructions for implementing OAuth providers in the Developer Mesh platform. Currently, only the OAuth interface exists without concrete implementations. This guide covers implementing popular OAuth providers like Google, GitHub, Microsoft, and generic OIDC.

## Current State

```go
// Only interface exists in pkg/auth/oauth_provider.go
type OAuthProvider interface {
    GetAuthorizationURL(state string) string
    ExchangeCodeForToken(ctx context.Context, code string) (*TokenResponse, error)
    RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error)
    GetUserInfo(ctx context.Context, token string) (*UserInfo, error)
}
```

## Implementation Architecture

### High-Level Flow
```
User → Login Page → OAuth Provider → Callback → Token Exchange → User Creation/Login → JWT Token
```

## Provider Implementations

### 1. Google OAuth Provider

#### 1.1 Configuration
```go
// pkg/auth/providers/google_oauth.go
package providers

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "strings"
    "time"
    
    "github.com/developer-mesh/developer-mesh/pkg/auth"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
)

type GoogleOAuthProvider struct {
    clientID     string
    clientSecret string
    redirectURI  string
    scopes       []string
    logger       observability.Logger
    httpClient   *http.Client
}

func NewGoogleOAuthProvider(clientID, clientSecret, redirectURI string) *GoogleOAuthProvider {
    return &GoogleOAuthProvider{
        clientID:     clientID,
        clientSecret: clientSecret,
        redirectURI:  redirectURI,
        scopes: []string{
            "openid",
            "email",
            "profile",
        },
        logger: observability.NewLogger("google-oauth"),
        httpClient: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}
```

#### 1.2 Authorization URL Generation
```go
func (g *GoogleOAuthProvider) GetAuthorizationURL(state string) string {
    params := url.Values{
        "client_id":     {g.clientID},
        "redirect_uri":  {g.redirectURI},
        "response_type": {"code"},
        "scope":         {strings.Join(g.scopes, " ")},
        "state":         {state},
        "access_type":   {"offline"}, // Request refresh token
        "prompt":        {"consent"}, // Force consent to get refresh token
    }
    
    return fmt.Sprintf("https://accounts.google.com/o/oauth2/v2/auth?%s", params.Encode())
}
```

#### 1.3 Token Exchange
```go
func (g *GoogleOAuthProvider) ExchangeCodeForToken(ctx context.Context, code string) (*auth.TokenResponse, error) {
    ctx, span := observability.StartSpan(ctx, "GoogleOAuth.ExchangeCodeForToken")
    defer span.End()
    
    // Prepare token exchange request
    data := url.Values{
        "client_id":     {g.clientID},
        "client_secret": {g.clientSecret},
        "code":          {code},
        "redirect_uri":  {g.redirectURI},
        "grant_type":    {"authorization_code"},
    }
    
    req, err := http.NewRequestWithContext(
        ctx,
        "POST",
        "https://oauth2.googleapis.com/token",
        strings.NewReader(data.Encode()),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    
    resp, err := g.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("token exchange failed: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("token exchange returned %d", resp.StatusCode)
    }
    
    var tokenResp struct {
        AccessToken  string `json:"access_token"`
        RefreshToken string `json:"refresh_token"`
        ExpiresIn    int    `json:"expires_in"`
        TokenType    string `json:"token_type"`
        IDToken      string `json:"id_token"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }
    
    return &auth.TokenResponse{
        AccessToken:  tokenResp.AccessToken,
        RefreshToken: tokenResp.RefreshToken,
        ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
        TokenType:    tokenResp.TokenType,
        IDToken:      tokenResp.IDToken,
    }, nil
}
```

#### 1.4 User Info Retrieval
```go
func (g *GoogleOAuthProvider) GetUserInfo(ctx context.Context, token string) (*auth.UserInfo, error) {
    ctx, span := observability.StartSpan(ctx, "GoogleOAuth.GetUserInfo")
    defer span.End()
    
    req, err := http.NewRequestWithContext(
        ctx,
        "GET",
        "https://www.googleapis.com/oauth2/v2/userinfo",
        nil,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    
    req.Header.Set("Authorization", "Bearer "+token)
    
    resp, err := g.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to get user info: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("user info request returned %d", resp.StatusCode)
    }
    
    var userInfo struct {
        ID            string `json:"id"`
        Email         string `json:"email"`
        VerifiedEmail bool   `json:"verified_email"`
        Name          string `json:"name"`
        Picture       string `json:"picture"`
        Locale        string `json:"locale"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
        return nil, fmt.Errorf("failed to decode user info: %w", err)
    }
    
    return &auth.UserInfo{
        ID:       userInfo.ID,
        Email:    userInfo.Email,
        Name:     userInfo.Name,
        Picture:  userInfo.Picture,
        Provider: "google",
        Metadata: map[string]interface{}{
            "verified_email": userInfo.VerifiedEmail,
            "locale":         userInfo.Locale,
        },
    }, nil
}
```

### 2. GitHub OAuth Provider

#### 2.1 Implementation
```go
// pkg/auth/providers/github_oauth.go
package providers

type GitHubOAuthProvider struct {
    clientID     string
    clientSecret string
    redirectURI  string
    scopes       []string
    logger       observability.Logger
    httpClient   *http.Client
}

func NewGitHubOAuthProvider(clientID, clientSecret, redirectURI string) *GitHubOAuthProvider {
    return &GitHubOAuthProvider{
        clientID:     clientID,
        clientSecret: clientSecret,
        redirectURI:  redirectURI,
        scopes: []string{
            "read:user",
            "user:email",
        },
        logger: observability.NewLogger("github-oauth"),
        httpClient: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}

func (g *GitHubOAuthProvider) GetAuthorizationURL(state string) string {
    params := url.Values{
        "client_id":    {g.clientID},
        "redirect_uri": {g.redirectURI},
        "scope":        {strings.Join(g.scopes, " ")},
        "state":        {state},
    }
    
    return fmt.Sprintf("https://github.com/login/oauth/authorize?%s", params.Encode())
}

func (g *GitHubOAuthProvider) ExchangeCodeForToken(ctx context.Context, code string) (*auth.TokenResponse, error) {
    data := url.Values{
        "client_id":     {g.clientID},
        "client_secret": {g.clientSecret},
        "code":          {code},
        "redirect_uri":  {g.redirectURI},
    }
    
    req, err := http.NewRequestWithContext(
        ctx,
        "POST",
        "https://github.com/login/oauth/access_token",
        strings.NewReader(data.Encode()),
    )
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.Header.Set("Accept", "application/json")
    
    // ... similar implementation to Google
}

func (g *GitHubOAuthProvider) GetUserInfo(ctx context.Context, token string) (*auth.UserInfo, error) {
    // Get user info
    userReq, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
    userReq.Header.Set("Authorization", "Bearer "+token)
    
    // Get primary email
    emailReq, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/emails", nil)
    emailReq.Header.Set("Authorization", "Bearer "+token)
    
    // ... combine responses
}
```

### 3. Generic OIDC Provider

#### 3.1 Auto-Discovery Implementation
```go
// pkg/auth/providers/oidc_provider.go
package providers

type OIDCProvider struct {
    issuerURL    string
    clientID     string
    clientSecret string
    redirectURI  string
    scopes       []string
    discovery    *OIDCDiscovery
    logger       observability.Logger
}

type OIDCDiscovery struct {
    Issuer                string   `json:"issuer"`
    AuthorizationEndpoint string   `json:"authorization_endpoint"`
    TokenEndpoint         string   `json:"token_endpoint"`
    UserInfoEndpoint      string   `json:"userinfo_endpoint"`
    JWKSUri               string   `json:"jwks_uri"`
    ScopesSupported       []string `json:"scopes_supported"`
}

func NewOIDCProvider(issuerURL, clientID, clientSecret, redirectURI string) (*OIDCProvider, error) {
    provider := &OIDCProvider{
        issuerURL:    issuerURL,
        clientID:     clientID,
        clientSecret: clientSecret,
        redirectURI:  redirectURI,
        scopes:       []string{"openid", "profile", "email"},
        logger:       observability.NewLogger("oidc"),
    }
    
    // Discover endpoints
    if err := provider.discover(); err != nil {
        return nil, fmt.Errorf("OIDC discovery failed: %w", err)
    }
    
    return provider, nil
}

func (o *OIDCProvider) discover() error {
    wellKnownURL := strings.TrimRight(o.issuerURL, "/") + "/.well-known/openid-configuration"
    
    resp, err := http.Get(wellKnownURL)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if err := json.NewDecoder(resp.Body).Decode(&o.discovery); err != nil {
        return err
    }
    
    return nil
}
```

## OAuth Callback Handler

### 4.1 Callback Route Implementation
```go
// apps/rest-api/internal/api/oauth_handlers.go
package api

type OAuthHandler struct {
    authManager    *auth.Manager
    userService    *services.UserService
    sessionService *services.SessionService
    providers      map[string]auth.OAuthProvider
}

// HandleOAuthLogin initiates OAuth flow
func (h *OAuthHandler) HandleOAuthLogin(c *gin.Context) {
    provider := c.Param("provider")
    
    oauthProvider, exists := h.providers[provider]
    if !exists {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown provider"})
        return
    }
    
    // Generate state with CSRF protection
    state := generateState()
    
    // Store state in session/cache
    h.sessionService.StoreState(c, state, provider)
    
    // Redirect to provider
    authURL := oauthProvider.GetAuthorizationURL(state)
    c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// HandleOAuthCallback processes OAuth callback
func (h *OAuthHandler) HandleOAuthCallback(c *gin.Context) {
    provider := c.Param("provider")
    code := c.Query("code")
    state := c.Query("state")
    
    // Verify state
    if !h.sessionService.VerifyState(c, state, provider) {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid state"})
        return
    }
    
    // Get provider
    oauthProvider, exists := h.providers[provider]
    if !exists {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown provider"})
        return
    }
    
    // Exchange code for token
    tokenResp, err := oauthProvider.ExchangeCodeForToken(c.Request.Context(), code)
    if err != nil {
        h.logger.Error("Token exchange failed", map[string]interface{}{
            "error":    err.Error(),
            "provider": provider,
        })
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication failed"})
        return
    }
    
    // Get user info
    userInfo, err := oauthProvider.GetUserInfo(c.Request.Context(), tokenResp.AccessToken)
    if err != nil {
        h.logger.Error("Failed to get user info", map[string]interface{}{
            "error":    err.Error(),
            "provider": provider,
        })
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
        return
    }
    
    // Create or update user
    user, err := h.userService.FindOrCreateByOAuth(c.Request.Context(), userInfo)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
        return
    }
    
    // Store OAuth tokens
    if err := h.userService.StoreOAuthTokens(c.Request.Context(), user.ID, provider, tokenResp); err != nil {
        h.logger.Warn("Failed to store OAuth tokens", map[string]interface{}{
            "error":   err.Error(),
            "user_id": user.ID,
        })
    }
    
    // Generate JWT token
    jwtToken, err := h.authManager.GenerateToken(user)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
        return
    }
    
    // Redirect to frontend with token
    redirectURL := fmt.Sprintf("%s/auth/success?token=%s", h.frontendURL, jwtToken)
    c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}
```

## State Management

### 5.1 CSRF Protection
```go
// pkg/auth/state_manager.go
package auth

import (
    "crypto/rand"
    "encoding/base64"
    "time"
)

type StateManager struct {
    cache  cache.Cache
    logger observability.Logger
}

func (s *StateManager) GenerateState() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    
    state := base64.URLEncoding.EncodeToString(b)
    
    // Store with 10-minute expiration
    key := fmt.Sprintf("oauth:state:%s", state)
    if err := s.cache.Set(key, true, 10*time.Minute); err != nil {
        return "", err
    }
    
    return state, nil
}

func (s *StateManager) VerifyState(state string) bool {
    key := fmt.Sprintf("oauth:state:%s", state)
    
    exists, err := s.cache.Exists(key)
    if err != nil || !exists {
        return false
    }
    
    // Delete after verification (one-time use)
    s.cache.Delete(key)
    
    return true
}
```

## User Management

### 6.1 OAuth User Integration
```go
// pkg/services/user_service.go
func (s *UserService) FindOrCreateByOAuth(ctx context.Context, info *auth.UserInfo) (*models.User, error) {
    ctx, span := s.tracer.Start(ctx, "UserService.FindOrCreateByOAuth")
    defer span.End()
    
    // Try to find existing user
    user, err := s.repo.FindByOAuthID(ctx, info.Provider, info.ID)
    if err == nil {
        // Update user info
        user.Name = info.Name
        user.Picture = info.Picture
        user.UpdatedAt = time.Now()
        
        if err := s.repo.Update(ctx, user); err != nil {
            return nil, err
        }
        
        return user, nil
    }
    
    // Check if email already exists
    if info.Email != "" {
        user, err = s.repo.FindByEmail(ctx, info.Email)
        if err == nil {
            // Link OAuth account
            if err := s.LinkOAuthAccount(ctx, user.ID, info); err != nil {
                return nil, err
            }
            return user, nil
        }
    }
    
    // Create new user
    user = &models.User{
        ID:       uuid.New().String(),
        Email:    info.Email,
        Name:     info.Name,
        Picture:  info.Picture,
        Provider: info.Provider,
        OAuthID:  info.ID,
        TenantID: s.getDefaultTenant(),
        Roles:    []string{"user"},
        Active:   true,
    }
    
    if err := s.repo.Create(ctx, user); err != nil {
        return nil, err
    }
    
    // Create OAuth link
    if err := s.LinkOAuthAccount(ctx, user.ID, info); err != nil {
        s.logger.Warn("Failed to create OAuth link", map[string]interface{}{
            "error":   err.Error(),
            "user_id": user.ID,
        })
    }
    
    return user, nil
}
```

## Configuration

### 7.1 Provider Configuration
```yaml
# configs/oauth.yaml
oauth:
  providers:
    google:
      enabled: true
      client_id: ${GOOGLE_OAUTH_CLIENT_ID}
      client_secret: ${GOOGLE_OAUTH_CLIENT_SECRET}
      redirect_uri: https://api.example.com/auth/callback/google
      
    github:
      enabled: true
      client_id: ${GITHUB_OAUTH_CLIENT_ID}
      client_secret: ${GITHUB_OAUTH_CLIENT_SECRET}
      redirect_uri: https://api.example.com/auth/callback/github
      
    microsoft:
      enabled: false
      client_id: ${MICROSOFT_OAUTH_CLIENT_ID}
      client_secret: ${MICROSOFT_OAUTH_CLIENT_SECRET}
      redirect_uri: https://api.example.com/auth/callback/microsoft
      tenant: common
      
    custom_oidc:
      enabled: false
      issuer_url: https://auth.company.com
      client_id: ${OIDC_CLIENT_ID}
      client_secret: ${OIDC_CLIENT_SECRET}
      redirect_uri: https://api.example.com/auth/callback/oidc
```

### 7.2 Environment Setup
```bash
# .env
GOOGLE_OAUTH_CLIENT_ID=your-client-id.apps.googleusercontent.com
GOOGLE_OAUTH_CLIENT_SECRET=your-client-secret
GITHUB_OAUTH_CLIENT_ID=your-github-client-id
GITHUB_OAUTH_CLIENT_SECRET=your-github-client-secret
```

## Frontend Integration

### 8.1 Login Page
```typescript
// frontend/src/components/LoginPage.tsx
import React from 'react';

const LoginPage: React.FC = () => {
    const handleOAuthLogin = (provider: string) => {
        window.location.href = `/api/v1/auth/login/${provider}`;
    };
    
    return (
        <div className="login-container">
            <h1>Login to Developer Mesh</h1>
            
            <button onClick={() => handleOAuthLogin('google')}>
                Login with Google
            </button>
            
            <button onClick={() => handleOAuthLogin('github')}>
                Login with GitHub
            </button>
            
            <button onClick={() => handleOAuthLogin('microsoft')}>
                Login with Microsoft
            </button>
        </div>
    );
};
```

### 8.2 Success Handler
```typescript
// frontend/src/components/AuthSuccess.tsx
import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';

const AuthSuccess: React.FC = () => {
    const navigate = useNavigate();
    
    useEffect(() => {
        const params = new URLSearchParams(window.location.search);
        const token = params.get('token');
        
        if (token) {
            // Store token
            localStorage.setItem('auth_token', token);
            
            // Redirect to dashboard
            navigate('/dashboard');
        } else {
            navigate('/login');
        }
    }, [navigate]);
    
    return <div>Authenticating...</div>;
};
```

## Security Considerations

### 9.1 Best Practices
```go
// Security middleware
func OAuthSecurityMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Enforce HTTPS
        if c.Request.Header.Get("X-Forwarded-Proto") != "https" {
            c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
                "error": "HTTPS required",
            })
            return
        }
        
        // Add security headers
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-Frame-Options", "DENY")
        c.Header("X-XSS-Protection", "1; mode=block")
        
        c.Next()
    }
}
```

### 9.2 Token Storage
```go
// Secure token storage
type OAuthTokenStore struct {
    db     *sqlx.DB
    crypto *encryption.Service
}

func (s *OAuthTokenStore) StoreTokens(userID string, provider string, tokens *auth.TokenResponse) error {
    // Encrypt sensitive tokens
    encryptedAccess, err := s.crypto.Encrypt(tokens.AccessToken)
    if err != nil {
        return err
    }
    
    encryptedRefresh, err := s.crypto.Encrypt(tokens.RefreshToken)
    if err != nil {
        return err
    }
    
    _, err = s.db.Exec(`
        INSERT INTO oauth_tokens (user_id, provider, access_token, refresh_token, expires_at)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (user_id, provider) DO UPDATE
        SET access_token = $3, refresh_token = $4, expires_at = $5, updated_at = NOW()
    `, userID, provider, encryptedAccess, encryptedRefresh, tokens.ExpiresAt)
    
    return err
}
```

## Testing

### 10.1 Mock Provider for Testing
```go
// pkg/auth/providers/mock_provider.go
type MockOAuthProvider struct {
    Users map[string]*auth.UserInfo
}

func (m *MockOAuthProvider) GetAuthorizationURL(state string) string {
    return fmt.Sprintf("http://localhost:8080/mock-oauth/authorize?state=%s", state)
}

func (m *MockOAuthProvider) ExchangeCodeForToken(ctx context.Context, code string) (*auth.TokenResponse, error) {
    if code == "valid-code" {
        return &auth.TokenResponse{
            AccessToken:  "mock-access-token",
            RefreshToken: "mock-refresh-token",
            ExpiresAt:    time.Now().Add(1 * time.Hour),
        }, nil
    }
    return nil, errors.New("invalid code")
}
```

### 10.2 Integration Tests
```go
func TestOAuthFlow(t *testing.T) {
    // Setup
    handler := setupOAuthHandler()
    router := setupRouter(handler)
    
    // Test login initiation
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/auth/login/google", nil)
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
    location := w.Header().Get("Location")
    assert.Contains(t, location, "accounts.google.com")
    
    // Test callback
    state := extractState(location)
    w = httptest.NewRecorder()
    req, _ = http.NewRequest("GET", fmt.Sprintf("/auth/callback/google?code=test-code&state=%s", state), nil)
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
    assert.Contains(t, w.Header().Get("Location"), "token=")
}
```

## Troubleshooting

### Common Issues

1. **Invalid redirect URI**
   - Ensure redirect URI matches exactly in provider config
   - Check for trailing slashes
   - Verify HTTPS in production

2. **State mismatch**
   - Check cache/session configuration
   - Verify state expiration time
   - Ensure cookies are enabled

3. **Token exchange failures**
   - Verify client secret is correct
   - Check network connectivity
   - Review provider-specific requirements

4. **User info retrieval errors**
   - Ensure requested scopes are granted
   - Check token expiration
   - Verify API endpoints

## Next Steps

1. Choose providers to implement
2. Register OAuth applications with providers
3. Implement provider classes
4. Add callback handlers
5. Create user management integration
6. Add frontend components
7. Comprehensive testing
8. Security audit
9. Documentation updates

## Resources

- [OAuth 2.0 RFC](https://tools.ietf.org/html/rfc6749)
- [OpenID Connect](https://openid.net/connect/)
- [Google OAuth 2.0](https://developers.google.com/identity/protocols/oauth2)
- [GitHub OAuth](https://docs.github.com/en/developers/apps/building-oauth-apps)
- [Microsoft Identity Platform](https://docs.microsoft.com/en-us/azure/active-directory/develop/)