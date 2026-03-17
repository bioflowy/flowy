// Package server auth handlers: login, user CRUD, me, and password change.
package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/auth"
	"github.com/bioflowy/flowy/internal/store"
)

// handleLogin handles POST /auth/login.
// Returns a signed JWT on valid credentials, 401 on invalid credentials or inactive user.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req api.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Use GetUserByUsernameAny to look up regardless of active state, so we can
	// distinguish "inactive account" (account disabled) from "not found" (invalid credentials).
	user, err := s.store.GetUserByUsernameAny(r.Context(), req.Username)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !user.IsActive {
		writeError(w, http.StatusUnauthorized, "account disabled")
		return
	}

	if err := auth.CheckPassword(user.PasswordHash, req.Password); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := auth.IssueToken(s.jwtSecret, user.ID, user.Username, user.GlobalRole)
	if err != nil {
		log.Printf("handleLogin: issue token: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	writeJSON(w, http.StatusOK, api.LoginResponse{Token: token})
}

// handleCreateUser handles POST /users (admin only).
// Creates a new user with the given credentials and returns the UserResponse with 201.
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req api.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}

	hash, err := auth.HashPassword(req.Password, auth.DefaultBcryptCost)
	if err != nil {
		log.Printf("handleCreateUser: hash password: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	role := req.GlobalRole
	if role == "" {
		role = "user"
	}

	u := &store.User{
		ID:           uuid.New().String(),
		Username:     req.Username,
		PasswordHash: hash,
		DisplayName:  req.DisplayName,
		Email:        req.Email,
		GlobalRole:   role,
		IsActive:     true,
	}

	if err := s.store.CreateUser(r.Context(), u); err != nil {
		log.Printf("handleCreateUser: create user: %v", err)
		writeError(w, http.StatusConflict, "username already exists")
		return
	}

	writeJSON(w, http.StatusCreated, userToResponse(u))
}

// handleListUsers handles GET /users (admin only).
// Returns a JSON array of all users.
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		log.Printf("handleListUsers: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	resp := make([]api.UserResponse, len(users))
	for i, u := range users {
		u := u // avoid loop variable capture
		resp[i] = userToResponse(&u)
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleDeleteUser handles DELETE /users/{id} (admin only).
// Soft-deletes the user and returns 204.
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "user id is required")
		return
	}
	if err := s.store.DeactivateUser(r.Context(), id); err != nil {
		log.Printf("handleDeleteUser: deactivate user %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleGetMe handles GET /users/me.
// Returns the authenticated user's profile.
func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	user, err := s.store.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		log.Printf("handleGetMe: get user %s: %v", claims.UserID, err)
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, userToResponse(user))
}

// handleChangePassword handles PUT /users/me/password.
// Verifies the current password and replaces it with the new password.
// Returns 204 on success.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req api.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.store.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		log.Printf("handleChangePassword: get user %s: %v", claims.UserID, err)
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	if err := auth.CheckPassword(user.PasswordHash, req.CurrentPassword); err != nil {
		writeError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	newHash, err := auth.HashPassword(req.NewPassword, auth.DefaultBcryptCost)
	if err != nil {
		log.Printf("handleChangePassword: hash new password: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	user.PasswordHash = newHash
	if err := s.store.UpdateUser(r.Context(), user); err != nil {
		log.Printf("handleChangePassword: update user %s: %v", claims.UserID, err)
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// userToResponse converts a store.User to an api.UserResponse.
func userToResponse(u *store.User) api.UserResponse {
	return api.UserResponse{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		GlobalRole:  u.GlobalRole,
		IsActive:    u.IsActive,
		CreatedAt:   u.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   u.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// handleLoginDisabled is registered as POST /auth/login when auth is disabled.
// It returns 200 with an empty token to indicate auth is not required.
// This allows clients to call /auth/login unconditionally.
func (s *Server) handleLoginDisabled(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.LoginResponse{Token: ""})
}

// bcryptCostError is used to detect bcrypt cost errors (not exported by package).
var _ = errors.New // ensure errors is used
var _ = bcrypt.MinCost
