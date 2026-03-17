// Package server middleware: JWT and daemon auth middleware for the flowy server.
package server

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/bioflowy/flowy/internal/auth"
)

// JWTMiddleware returns a middleware that validates the Bearer JWT in the
// Authorization header. On success, it injects *auth.Claims into the request
// context via auth.UserClaimsKey. On failure, it returns 401 JSON.
func JWTMiddleware(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractBearerToken(r)
			if tokenStr == "" {
				writeError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}
			claims, err := auth.ValidateToken(secret, tokenStr)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}
			ctx := context.WithValue(r.Context(), auth.UserClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// DaemonAuthMiddleware returns a middleware that validates the Bearer token in
// the Authorization header against the configured daemon token.
// Uses constant-time comparison to prevent timing attacks.
func DaemonAuthMiddleware(daemonToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				writeError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}
			if subtle.ConstantTimeCompare([]byte(token), []byte(daemonToken)) != 1 {
				writeError(w, http.StatusUnauthorized, "invalid daemon token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// roleRequired returns a middleware that checks the authenticated user's role.
// It must be chained after JWTMiddleware (claims must be in context).
// Returns 403 if the role does not match the required role.
func roleRequired(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFromContext(r.Context())
			if claims == nil || claims.Role != role {
				writeError(w, http.StatusForbidden, "forbidden: insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// claimsFromContext extracts *auth.Claims from the request context.
// Returns nil if no claims are present (e.g., unauthenticated request).
func claimsFromContext(ctx context.Context) *auth.Claims {
	v := ctx.Value(auth.UserClaimsKey)
	if v == nil {
		return nil
	}
	c, _ := v.(*auth.Claims)
	return c
}

// extractBearerToken extracts the token from the Authorization header.
// Returns empty string if the header is missing or not in "Bearer <token>" format.
func extractBearerToken(r *http.Request) string {
	hdr := r.Header.Get("Authorization")
	if !strings.HasPrefix(hdr, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(hdr, "Bearer ")
}
