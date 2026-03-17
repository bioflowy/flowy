// Package server middleware: JWT and daemon auth middleware for the flowy server.
package server

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/bioflowy/flowy/internal/auth"
	"github.com/bioflowy/flowy/internal/store"
)

// projectContextKey is the unexported type for project context keys.
type projectContextKey int

// projectKey is the context key used to store the resolved *store.Project.
const projectKey projectContextKey = 0

// roleRank maps project role names to their numeric rank for comparison.
// Higher rank = more privileged.
var roleRank = map[string]int{
	"viewer":   1,
	"executor": 2,
	"admin":    3,
}

// roleAtLeast returns true if actual satisfies the minRole requirement.
func roleAtLeast(actual, minRole string) bool {
	return roleRank[actual] >= roleRank[minRole]
}

// projectContext is a middleware that resolves the {slug} URL parameter to a
// *store.Project and injects it into the request context. Returns 404 if the
// project is not found or is inactive.
func (s *Server) projectContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		proj, err := s.store.GetProjectBySlug(r.Context(), slug)
		if err != nil || proj == nil || !proj.IsActive {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		ctx := context.WithValue(r.Context(), projectKey, proj)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// projectFromContext extracts the *store.Project injected by projectContext.
// Returns nil if no project is present in the context.
func projectFromContext(ctx context.Context) *store.Project {
	v := ctx.Value(projectKey)
	if v == nil {
		return nil
	}
	p, _ := v.(*store.Project)
	return p
}

// projectRoleRequired returns a middleware that enforces per-project RBAC.
// Global admins (claims.Role == "admin") bypass the membership check entirely.
// For all other users, the project must be in context (injected by projectContext)
// and the user must have a project role satisfying minRole.
func (s *Server) projectRoleRequired(minRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFromContext(r.Context())
			if claims == nil {
				writeError(w, http.StatusUnauthorized, "not authenticated")
				return
			}

			// Global admin bypasses project membership check (per research pitfall 2).
			if claims.Role == "admin" {
				next.ServeHTTP(w, r)
				return
			}

			proj := projectFromContext(r.Context())
			if proj == nil {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}

			member, err := s.store.GetProjectMember(r.Context(), proj.ID, claims.UserID)
			if err != nil || member == nil {
				writeError(w, http.StatusForbidden, "not a project member")
				return
			}

			if !roleAtLeast(member.Role, minRole) {
				writeError(w, http.StatusForbidden, "insufficient project role")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

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
