// Package auth provides JWT token issuance/validation and bcrypt password
// helpers for the flowy server. It is a leaf package with no dependency on
// internal/store, using a minimal Store interface to avoid import cycles.
package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// DefaultBcryptCost is the bcrypt cost used for production password hashing.
// Use bcrypt.MinCost in tests for speed.
const DefaultBcryptCost = 12

// contextKey is an unexported type for context keys to avoid collisions.
type contextKey int

// UserClaimsKey is the context key under which *Claims is stored by JWTMiddleware.
const UserClaimsKey contextKey = iota

// Claims holds the JWT payload for an authenticated user.
type Claims struct {
	UserID   string `json:"sub"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Store is the minimal interface required by ResolveJWTSecret.
// Implemented by *sqlite.SQLiteStore — defined here to avoid import cycles.
type Store interface {
	GetServerConfig(ctx context.Context, key string) (string, error)
	SetServerConfig(ctx context.Context, key, value string) error
}

// IssueToken creates and signs a JWT with HS256 for the given user, valid for 7 days.
// Returns the signed token string or an error.
func IssueToken(secret []byte, userID, username, role string) (string, error) {
	return IssueTokenWithExpiry(secret, userID, username, role, 7*24*time.Hour)
}

// IssueTokenWithExpiry creates a JWT with the specified expiry duration.
// Negative durations produce already-expired tokens (useful in tests).
func IssueTokenWithExpiry(secret []byte, userID, username, role string, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// ValidateToken parses and validates a JWT signed with the given secret.
// Returns the claims on success, or an error if the token is expired, malformed,
// or signed with a different secret.
func ValidateToken(secret []byte, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}

// HashPassword returns a bcrypt hash of password with the given cost.
// Use DefaultBcryptCost in production; bcrypt.MinCost in tests.
func HashPassword(password string, cost int) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword verifies that password matches the bcrypt hash.
// Returns nil on success, non-nil error on mismatch.
func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// ResolveJWTSecret returns the effective JWT signing secret following this priority:
//  1. jwtSecretCfg (non-empty string from config file) — returned as-is.
//  2. DB key "jwt_secret" (previously auto-generated) — returned if found.
//  3. Auto-generate: 32 random bytes, hex-encoded, persisted via SetServerConfig.
func ResolveJWTSecret(ctx context.Context, jwtSecretCfg string, st Store) ([]byte, error) {
	if jwtSecretCfg != "" {
		return []byte(jwtSecretCfg), nil
	}

	// Try DB.
	stored, err := st.GetServerConfig(ctx, "jwt_secret")
	if err == nil {
		return []byte(stored), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("resolve jwt secret: get server config: %w", err)
	}

	// Auto-generate.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("resolve jwt secret: generate random bytes: %w", err)
	}
	hexSecret := hex.EncodeToString(raw)
	if err := st.SetServerConfig(ctx, "jwt_secret", hexSecret); err != nil {
		return nil, fmt.Errorf("resolve jwt secret: persist generated secret: %w", err)
	}
	return []byte(hexSecret), nil
}
