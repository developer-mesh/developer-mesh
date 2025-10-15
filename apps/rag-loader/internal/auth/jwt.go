package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTClaims contains the claims extracted from the JWT token
type JWTClaims struct {
	TenantID string   `json:"tenant_id"`
	UserID   string   `json:"user_id"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

// JWTValidator handles JWT validation and token generation
type JWTValidator struct {
	secretKey []byte
	issuer    string
}

// NewJWTValidator creates a new JWT validator
func NewJWTValidator(secretKey []byte, issuer string) *JWTValidator {
	return &JWTValidator{
		secretKey: secretKey,
		issuer:    issuer,
	}
}

// ValidateJWT validates the JWT token and returns claims
func (v *JWTValidator) ValidateJWT(authHeader string) (*JWTClaims, error) {
	// Extract token from "Bearer <token>" format
	tokenString, err := extractBearerToken(authHeader)
	if err != nil {
		return nil, err
	}

	// Parse token with claims
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing algorithm
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.secretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Extract and validate claims
	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	// Validate standard claims
	if err := v.validateStandardClaims(claims); err != nil {
		return nil, err
	}

	// Validate tenant ID format
	if _, err := uuid.Parse(claims.TenantID); err != nil {
		return nil, fmt.Errorf("invalid tenant ID format: %w", err)
	}

	// Validate user ID format
	if _, err := uuid.Parse(claims.UserID); err != nil {
		return nil, fmt.Errorf("invalid user ID format: %w", err)
	}

	return claims, nil
}

// GenerateToken generates a new JWT token for testing purposes
func (v *JWTValidator) GenerateToken(tenantID, userID, email string, roles []string) (string, error) {
	claims := JWTClaims{
		TenantID: tenantID,
		UserID:   userID,
		Email:    email,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    v.issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(v.secretKey)
}

// extractBearerToken extracts the token from "Bearer <token>" header
func extractBearerToken(authHeader string) (string, error) {
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", errors.New("invalid authorization header format")
	}
	return parts[1], nil
}

// validateStandardClaims validates the standard JWT claims
func (v *JWTValidator) validateStandardClaims(claims *JWTClaims) error {
	now := time.Now()

	// Check expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(now) {
		return errors.New("token has expired")
	}

	// Check not before
	if claims.NotBefore != nil && claims.NotBefore.After(now) {
		return errors.New("token not yet valid")
	}

	// Check issuer if configured
	if v.issuer != "" && claims.Issuer != v.issuer {
		return fmt.Errorf("invalid issuer: expected %s, got %s", v.issuer, claims.Issuer)
	}

	return nil
}
