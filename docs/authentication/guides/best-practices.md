<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:31:08
Verification Script: update-docs-parallel.sh
Batch: ac
-->

# Authentication & Authorization Best Practices

> **Purpose**: Security patterns and implementation guidelines for Developer Mesh
> **Audience**: Developers implementing auth features or using the auth system
> **Scope**: API keys, JWT tokens, OAuth, session management, and security patterns

## Overview

This guide provides comprehensive best practices for implementing and using authentication and authorization in the Developer Mesh platform. It covers security patterns, common pitfalls, and practical recommendations based on industry standards and real-world experience.

## Core Principles

### 1. Defense in Depth
- **Multiple Layers**: Never rely on a single security measure
- **Fail Secure**: Default to denying access when uncertain
- **Least Privilege**: Grant minimum necessary permissions
- **Zero Trust**: Verify every request, trust nothing implicitly

### 2. Separation of Concerns
- **Authentication**: Who are you? (Identity verification)
- **Authorization**: What can you do? (Permission checking)
- **Audit**: What did you do? (Activity logging)

## Authentication Best Practices

### API Key Management

#### DO:
```go
// Generate cryptographically secure keys
func GenerateAPIKey() (string, error) {
    bytes := make([]byte, 32)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(bytes), nil
}

// Store hashed keys only
hashedKey := sha256.Sum256([]byte(apiKey))
db.StoreAPIKey(hex.EncodeToString(hashedKey[:]))

// Implement key rotation
type APIKey struct {
    ID           uuid.UUID
    HashedKey    string
    ExpiresAt    time.Time
    RotatedFrom  *uuid.UUID // Link to previous key
    LastUsedAt   time.Time
}

// Rate limit by key
rateLimiter := rate.NewLimiter(100, 10) // 100 req/s, burst 10
if !rateLimiter.Allow() {
    return errors.New("rate limit exceeded")
}
```

#### DON'T:
```go
// ❌ Don't store plain text keys
db.StoreAPIKey(plainTextKey)

// ❌ Don't use predictable keys
apiKey := fmt.Sprintf("key_%d", userID)

// ❌ Don't skip expiration
key := &APIKey{} // No ExpiresAt set

// ❌ Don't log full keys
logger.Info("API key used", map[string]interface{}{
    "key": apiKey, // Never log full key
})
```

#### Key Rotation Strategy:
```go
func RotateAPIKey(ctx context.Context, oldKeyID uuid.UUID) (*APIKey, error) {
    // 1. Generate new key
    newKey, err := GenerateAPIKey()
    if err != nil {
        return nil, err
    }
    
    // 2. Create with grace period
    apiKey := &APIKey{
        ID:          uuid.New(),
        HashedKey:   hashKey(newKey),
        ExpiresAt:   time.Now().Add(90 * 24 * time.Hour),
        RotatedFrom: &oldKeyID,
    }
    
    // 3. Keep old key active for grace period (7 days)
    db.ExtendKeyExpiration(oldKeyID, 7*24*time.Hour)
    
    // 4. Notify user
    notifyKeyRotation(userEmail, oldKeyID, apiKey.ID)
    
    return apiKey, nil
}
```

### JWT Token Security

#### Secure Implementation:
```go
// Use strong secret keys (min 256 bits)
var jwtSecret = os.Getenv("JWT_SECRET") // Must be at least 32 bytes

// Short expiration times
const (
    AccessTokenExpiry  = 15 * time.Minute
    RefreshTokenExpiry = 7 * 24 * time.Hour
)

// Include essential claims only
type Claims struct {
    jwt.StandardClaims
    UserID   string   `json:"sub"`
    TenantID string   `json:"tenant"`
    Scopes   []string `json:"scopes,omitempty"`
}

// Validate all claims
func ValidateToken(tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        // Verify signing algorithm
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return []byte(jwtSecret), nil
    })
    
    if err != nil {
        return nil, err
    }
    
    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, errors.New("invalid token")
    }
    
    // Additional validation
    if claims.ExpiresAt < time.Now().Unix() {
        return nil, errors.New("token expired")
    }
    
    if claims.IssuedAt > time.Now().Unix() {
        return nil, errors.New("token used before issued")
    }
    
    return claims, nil
}
```

#### Token Storage:
```go
// Client-side storage recommendations
const TokenStorage = {
    // For web apps: Use httpOnly, secure cookies
    SetToken: (token) => {
        document.cookie = `auth_token=${token}; path=/; secure; httpOnly; samesite=strict`;
    },
    
    // For mobile/desktop: Use secure storage
    SetTokenSecure: async (token) => {
        await SecureStore.setItemAsync('auth_token', token);
    },
    
    // Never store in localStorage for sensitive data
    SetTokenInsecure: (token) => {
        localStorage.setItem('auth_token', token); // ❌ Vulnerable to XSS
    }
}
```

### OAuth Security

#### State Parameter:
```go
// Generate unpredictable state
func GenerateOAuthState() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    
    state := base64.URLEncoding.EncodeToString(b)
    
    // Store with expiration
    cache.Set(fmt.Sprintf("oauth:state:%s", state), true, 10*time.Minute)
    
    return state, nil
}

// Validate state parameter
func ValidateOAuthState(state string) error {
    key := fmt.Sprintf("oauth:state:%s", state)
    
    // Check existence
    exists, err := cache.Exists(key)
    if err != nil || !exists {
        return errors.New("invalid or expired state")
    }
    
    // Delete after use (one-time use)
    cache.Delete(key)
    
    return nil
}
```

#### PKCE Implementation:
```go
// For public clients (mobile, SPA)
type PKCEChallenge struct {
    Verifier  string
    Challenge string
    Method    string
}

func GeneratePKCE() (*PKCEChallenge, error) {
    // Generate code verifier
    verifier := make([]byte, 32)
    if _, err := rand.Read(verifier); err != nil {
        return nil, err
    }
    
    verifierStr := base64.URLEncoding.EncodeToString(verifier)
    
    // Generate code challenge
    h := sha256.Sum256([]byte(verifierStr))
    challenge := base64.URLEncoding.EncodeToString(h[:])
    
    return &PKCEChallenge{
        Verifier:  verifierStr,
        Challenge: challenge,
        Method:    "S256",
    }, nil
}
```

## Authorization Best Practices

### Permission Checking

#### Effective Patterns:
```go
// Context-based authorization
func (s *Service) GetResource(ctx context.Context, resourceID string) (*Resource, error) {
    // Extract user from context
    user, err := auth.UserFromContext(ctx)
    if err != nil {
        return nil, errors.New("unauthorized")
    }
    
    // Load resource
    resource, err := s.repo.GetResource(ctx, resourceID)
    if err != nil {
        return nil, err
    }
    
    // Check ownership
    if resource.OwnerID != user.ID && resource.TenantID != user.TenantID {
        return nil, errors.New("forbidden")
    }
    
    // Check specific permission
    if !s.authorizer.Can(ctx, user, "read", resource) {
        return nil, errors.New("forbidden")
    }
    
    return resource, nil
}

// Middleware pattern
func RequirePermission(permission string) gin.HandlerFunc {
    return func(c *gin.Context) {
        user := c.MustGet("user").(*User)
        resource := c.Param("resource")
        
        allowed, err := authorizer.CheckPermission(c.Request.Context(), 
            user, resource, permission)
        
        if err != nil || !allowed {
            c.AbortWithStatus(http.StatusForbidden)
            return
        }
        
        c.Next()
    }
}
```

#### Resource-based Authorization:
```go
// Implement resource ownership
type Resource struct {
    ID       uuid.UUID
    OwnerID  uuid.UUID
    TenantID uuid.UUID
    Public   bool
    ShareIDs []uuid.UUID // Users with shared access
}

func (r *Resource) CanAccess(user *User) bool {
    // Owner always has access
    if r.OwnerID == user.ID {
        return true
    }
    
    // Same tenant access (if configured)
    if r.TenantID == user.TenantID && user.HasRole("tenant_member") {
        return true
    }
    
    // Public resources
    if r.Public {
        return true
    }
    
    // Shared access
    for _, shareID := range r.ShareIDs {
        if shareID == user.ID {
            return true
        }
    }
    
    return false
}
```

## Session Management

### Secure Session Implementation

#### Session Creation:
```go
type SessionManager struct {
    store      SessionStore
    encryptor  Encryptor
    maxAge     time.Duration
    secure     bool
}

func (sm *SessionManager) CreateSession(ctx context.Context, user *User) (*Session, error) {
    // Generate secure session ID
    sessionID, err := generateSecureID()
    if err != nil {
        return nil, err
    }
    
    // Create session data
    session := &Session{
        ID:        sessionID,
        UserID:    user.ID,
        TenantID:  user.TenantID,
        CreatedAt: time.Now(),
        ExpiresAt: time.Now().Add(sm.maxAge),
        IPAddress: getClientIP(ctx),
        UserAgent: getUserAgent(ctx),
        Data:      make(map[string]interface{}),
    }
    
    // Encrypt sensitive data
    encrypted, err := sm.encryptor.Encrypt(session)
    if err != nil {
        return nil, err
    }
    
    // Store session
    if err := sm.store.Set(ctx, sessionID, encrypted, sm.maxAge); err != nil {
        return nil, err
    }
    
    return session, nil
}
```

#### Session Validation:
```go
func (sm *SessionManager) ValidateSession(ctx context.Context, sessionID string) (*Session, error) {
    // Retrieve from store
    encrypted, err := sm.store.Get(ctx, sessionID)
    if err != nil {
        return nil, errors.New("session not found")
    }
    
    // Decrypt session
    session, err := sm.encryptor.Decrypt(encrypted)
    if err != nil {
        return nil, errors.New("invalid session data")
    }
    
    // Validate expiration
    if time.Now().After(session.ExpiresAt) {
        sm.store.Delete(ctx, sessionID)
        return nil, errors.New("session expired")
    }
    
    // Validate consistency
    if !sm.validateConsistency(ctx, session) {
        return nil, errors.New("session validation failed")
    }
    
    // Extend session
    session.LastAccessedAt = time.Now()
    sm.store.Extend(ctx, sessionID, sm.maxAge)
    
    return session, nil
}

func (sm *SessionManager) validateConsistency(ctx context.Context, session *Session) bool {
    // Check IP address change (configurable)
    if sm.checkIPBinding && session.IPAddress != getClientIP(ctx) {
        return false
    }
    
    // Check user agent change
    if sm.checkUserAgent && session.UserAgent != getUserAgent(ctx) {
        return false
    }
    
    return true
}
```

### Concurrent Session Management:
```go
type ConcurrentSessionManager struct {
    maxSessions int
    strategy    SessionStrategy
}

type SessionStrategy string

const (
    StrategyDenyNew      SessionStrategy = "deny_new"
    StrategyInvalidateOld SessionStrategy = "invalidate_old"
    StrategyWarnUser     SessionStrategy = "warn_user"
)

func (csm *ConcurrentSessionManager) OnSessionCreate(ctx context.Context, userID uuid.UUID, newSession *Session) error {
    // Get active sessions
    sessions, err := csm.GetActiveSessions(ctx, userID)
    if err != nil {
        return err
    }
    
    if len(sessions) >= csm.maxSessions {
        switch csm.strategy {
        case StrategyDenyNew:
            return errors.New("maximum concurrent sessions reached")
            
        case StrategyInvalidateOld:
            // Invalidate oldest session
            oldest := csm.findOldestSession(sessions)
            csm.InvalidateSession(ctx, oldest.ID)
            csm.notifySessionInvalidated(userID, oldest)
            
        case StrategyWarnUser:
            // Allow but warn user
            csm.warnUserAboutSessions(userID, sessions)
        }
    }
    
    return nil
}
```

## Security Headers

### Essential Security Headers:
```go
func SecurityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Prevent XSS attacks
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-XSS-Protection", "1; mode=block")
        
        // Prevent clickjacking
        c.Header("X-Frame-Options", "DENY")
        c.Header("Content-Security-Policy", "frame-ancestors 'none'")
        
        // Force HTTPS
        c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        
        // Disable caching for sensitive endpoints
        if strings.Contains(c.Request.URL.Path, "/auth") {
            c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
            c.Header("Pragma", "no-cache")
        }
        
        c.Next()
    }
}
```

## Rate Limiting

### Intelligent Rate Limiting:
```go
type RateLimiter struct {
    limiters sync.Map
    config   RateLimitConfig
}

type RateLimitConfig struct {
    // Different limits for different operations
    LoginLimit      rate.Limit // Strict for login attempts
    APILimit        rate.Limit // Normal for API calls
    PublicLimit     rate.Limit // Restrictive for public endpoints
    
    // Burst allowance
    LoginBurst      int
    APIBurst        int
    PublicBurst     int
}

func (rl *RateLimiter) CheckLimit(ctx context.Context, key string, operation string) error {
    limiter := rl.getLimiter(key, operation)
    
    if !limiter.Allow() {
        // Log rate limit violation
        rl.logViolation(ctx, key, operation)
        
        // Progressive penalties
        rl.applyPenalty(key, operation)
        
        return errors.New("rate limit exceeded")
    }
    
    return nil
}

func (rl *RateLimiter) applyPenalty(key string, operation string) {
    if operation == "login" {
        // Exponential backoff for login attempts
        violations := rl.getViolationCount(key)
        penaltyDuration := time.Duration(math.Pow(2, float64(violations))) * time.Second
        rl.blockKey(key, penaltyDuration)
    }
}
```

## Password Security (Future Implementation)

### If Adding Password Auth:
```go
// Use Argon2id for password hashing
import "golang.org/x/crypto/argon2"

type PasswordHasher struct {
    time    uint32
    memory  uint32
    threads uint8
    keyLen  uint32
}

func NewPasswordHasher() *PasswordHasher {
    return &PasswordHasher{
        time:    1,
        memory:  64 * 1024, // 64 MB
        threads: 4,
        keyLen:  32,
    }
}

func (ph *PasswordHasher) Hash(password string) (string, error) {
    // Generate salt
    salt := make([]byte, 16)
    if _, err := rand.Read(salt); err != nil {
        return "", err
    }
    
    // Hash password
    hash := argon2.IDKey([]byte(password), salt, ph.time, ph.memory, ph.threads, ph.keyLen)
    
    // Encode for storage
    return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
        argon2.Version, ph.memory, ph.time, ph.threads,
        base64.RawStdEncoding.EncodeToString(salt),
        base64.RawStdEncoding.EncodeToString(hash),
    ), nil
}

// Password requirements
func ValidatePassword(password string) error {
    if len(password) < 12 {
        return errors.New("password must be at least 12 characters")
    }
    
    // Check complexity (configurable)
    var hasUpper, hasLower, hasDigit, hasSpecial bool
    for _, char := range password {
        switch {
        case unicode.IsUpper(char):
            hasUpper = true
        case unicode.IsLower(char):
            hasLower = true
        case unicode.IsDigit(char):
            hasDigit = true
        case unicode.IsPunct(char) || unicode.IsSymbol(char):
            hasSpecial = true
        }
    }
    
    if !hasUpper || !hasLower || !hasDigit {
        return errors.New("password must contain uppercase, lowercase, and digits")
    }
    
    return nil
}
```

## Common Security Vulnerabilities

### 1. Timing Attacks
```go
// ❌ Vulnerable to timing attacks
func validateAPIKey(provided, stored string) bool {
    return provided == stored // Comparison time varies
}

// ✅ Constant-time comparison
func validateAPIKey(provided, stored string) bool {
    return subtle.ConstantTimeCompare([]byte(provided), []byte(stored)) == 1
}
```

### 2. SQL Injection
```go
// ❌ Vulnerable to SQL injection
query := fmt.Sprintf("SELECT * FROM users WHERE id = '%s'", userID)

// ✅ Use parameterized queries
query := "SELECT * FROM users WHERE id = $1"
db.QueryRow(query, userID)
```

### 3. Insecure Direct Object References
```go
// ❌ No authorization check
func GetDocument(c *gin.Context) {
    docID := c.Param("id")
    doc, _ := db.GetDocument(docID)
    c.JSON(200, doc)
}

// ✅ Verify access rights
func GetDocument(c *gin.Context) {
    user := c.MustGet("user").(*User)
    docID := c.Param("id")
    
    doc, err := db.GetDocument(docID)
    if err != nil {
        c.JSON(404, gin.H{"error": "not found"})
        return
    }
    
    if !doc.CanAccess(user) {
        c.JSON(403, gin.H{"error": "forbidden"})
        return
    }
    
    c.JSON(200, doc)
}
```

## Monitoring and Alerting

### Security Events to Monitor:
```go
type SecurityMonitor struct {
    metrics SecurityMetrics
    alerter AlertService
}

func (sm *SecurityMonitor) MonitorAuthEvents() {
    // Failed login attempts
    sm.metrics.TrackFailedLogin = func(userID, ip string) {
        count := sm.getFailedCount(userID, ip, 5*time.Minute)
        if count > 5 {
            sm.alerter.Alert("Multiple failed login attempts", map[string]interface{}{
                "user_id": userID,
                "ip": ip,
                "count": count,
            })
        }
    }
    
    // Privilege escalation attempts
    sm.metrics.TrackPrivilegeEscalation = func(userID, action string) {
        sm.alerter.Alert("Privilege escalation attempt", map[string]interface{}{
            "user_id": userID,
            "action": action,
        })
    }
    
    // Unusual access patterns
    sm.metrics.TrackAccessPattern = func(userID, resource string) {
        if sm.isUnusualAccess(userID, resource) {
            sm.alerter.Alert("Unusual access pattern", map[string]interface{}{
                "user_id": userID,
                "resource": resource,
            })
        }
    }
}
```

## Testing Security

### Security Test Suite:
```go
func TestSecurityVulnerabilities(t *testing.T) {
    t.Run("SQL Injection", func(t *testing.T) {
        maliciousInput := "'; DROP TABLE users; --"
        _, err := api.GetUser(maliciousInput)
        assert.Error(t, err)
        // Verify table still exists
        assert.True(t, tableExists("users"))
    })
    
    t.Run("XSS Prevention", func(t *testing.T) {
        xssPayload := "<script>alert('xss')</script>"
        resp := api.CreateResource(xssPayload)
        assert.NotContains(t, resp.Body, "<script>")
        assert.Contains(t, resp.Body, "&lt;script&gt;")
    })
    
    t.Run("Rate Limiting", func(t *testing.T) {
        for i := 0; i < 100; i++ {
            resp := api.Login("user", "wrong")
            if i > 5 {
                assert.Equal(t, 429, resp.StatusCode)
            }
        }
    })
}
```

## Compliance Considerations

### GDPR Compliance:
```go
// Right to erasure
func (s *UserService) DeleteUserData(ctx context.Context, userID uuid.UUID) error {
    // Start transaction
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // Anonymize audit logs (keep for legal requirements)
    tx.Exec("UPDATE audit_logs SET user_id = NULL, ip_address = '0.0.0.0' WHERE user_id = $1", userID)
    
    // Delete personal data
    tx.Exec("DELETE FROM user_profiles WHERE user_id = $1", userID)
    tx.Exec("DELETE FROM user_sessions WHERE user_id = $1", userID)
    tx.Exec("DELETE FROM api_keys WHERE user_id = $1", userID)
    
    // Delete user account
    tx.Exec("DELETE FROM users WHERE id = $1", userID)
    
    return tx.Commit()
}

// Data portability
func (s *UserService) ExportUserData(ctx context.Context, userID uuid.UUID) ([]byte, error) {
    data := struct {
        Profile   UserProfile   `json:"profile"`
        Sessions  []Session     `json:"sessions"`
        AuditLogs []AuditLog    `json:"audit_logs"`
        APIKeys   []APIKeyMeta  `json:"api_keys"`
    }{}
    
    // Collect all user data
    // ... implementation ...
    
    return json.MarshalIndent(data, "", "  ")
}
```

## Security Checklist

### Development Phase:
- [ ] Use HTTPS everywhere (no HTTP in production)
- [ ] Implement proper authentication for all endpoints
- [ ] Add authorization checks for resource access
- [ ] Use parameterized queries (no string concatenation)
- [ ] Validate and sanitize all inputs
- [ ] Implement rate limiting
- [ ] Add security headers
- [ ] Log security events
- [ ] Handle errors without leaking information
- [ ] Use secure random number generation

### Deployment Phase:
- [ ] Rotate all secrets and keys
- [ ] Enable audit logging
- [ ] Configure firewall rules
- [ ] Set up monitoring and alerting
- [ ] Disable debug mode
- [ ] Remove development endpoints
- [ ] Update dependencies
- [ ] Run security scanning
- [ ] Document security procedures
- [ ] Train team on security practices

### Operational Phase:
- [ ] Regular security audits
- [ ] Penetration testing
- [ ] Dependency updates
- [ ] Key rotation schedule
- [ ] Incident response plan
- [ ] Security training updates
- [ ] Monitor for anomalies
- [ ] Review access logs
- [ ] Update security documentation
- [ ] Practice incident response

## Resources

- [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
- [OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
- [NIST Digital Identity Guidelines](https://pages.nist.gov/800-63-3/)
- [OAuth 2.0 Security Best Practices](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-security-topics)
