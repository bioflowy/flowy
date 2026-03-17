// Package server membership handlers: add, list, update, and remove project members.
package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/store"
)

// validProjectRole reports whether role is one of the accepted project roles.
func validProjectRole(role string) bool {
	switch role {
	case "admin", "executor", "viewer":
		return true
	default:
		return false
	}
}

// memberToResponse converts a store.ProjectMember to an api.MemberResponse.
// username is the resolved username for the member's user ID.
func memberToResponse(m *store.ProjectMember, username string) api.MemberResponse {
	return api.MemberResponse{
		UserID:    m.UserID,
		Username:  username,
		Role:      m.Role,
		CreatedAt: m.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: m.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// handleAddMember handles POST /projects/{slug}/members.
// Adds a user by username with the specified role; returns 201 with MemberResponse.
func (s *Server) handleAddMember(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req api.AddMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !validProjectRole(req.Role) {
		writeError(w, http.StatusBadRequest, "invalid role: must be admin, executor, or viewer")
		return
	}

	user, err := s.store.GetUserByUsername(r.Context(), req.Username)
	if err != nil || user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	now := time.Now().UTC()
	member := &store.ProjectMember{
		ProjectID: proj.ID,
		UserID:    user.ID,
		Role:      req.Role,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.store.AddMember(r.Context(), member); err != nil {
		log.Printf("handleAddMember: add member %q to project %q: %v", req.Username, proj.Slug, err)
		writeError(w, http.StatusConflict, "user is already a member of this project")
		return
	}

	writeJSON(w, http.StatusCreated, memberToResponse(member, user.Username))
}

// handleListMembers handles GET /projects/{slug}/members.
// Returns all members with resolved usernames; returns 200 with []MemberResponse.
func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	members, err := s.store.ListMembers(r.Context(), proj.ID)
	if err != nil {
		log.Printf("handleListMembers: list members for project %q: %v", proj.Slug, err)
		writeError(w, http.StatusInternalServerError, "failed to list members")
		return
	}

	resp := make([]api.MemberResponse, 0, len(members))
	for _, m := range members {
		m := m // avoid loop variable capture
		user, err := s.store.GetUserByID(r.Context(), m.UserID)
		if err != nil {
			log.Printf("handleListMembers: get user %q: %v", m.UserID, err)
			continue
		}
		resp = append(resp, memberToResponse(&m, user.Username))
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleUpdateMember handles PUT /projects/{slug}/members/{username}.
// Changes the role of an existing project member; returns 200 with updated MemberResponse.
func (s *Server) handleUpdateMember(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	username := chi.URLParam(r, "username")
	user, err := s.store.GetUserByUsername(r.Context(), username)
	if err != nil || user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	var req api.UpdateMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !validProjectRole(req.Role) {
		writeError(w, http.StatusBadRequest, "invalid role: must be admin, executor, or viewer")
		return
	}

	if err := s.store.UpdateMemberRole(r.Context(), proj.ID, user.ID, req.Role); err != nil {
		log.Printf("handleUpdateMember: update role for user %q in project %q: %v", username, proj.Slug, err)
		writeError(w, http.StatusInternalServerError, "failed to update member role")
		return
	}

	// Retrieve updated member record to build response.
	member, err := s.store.GetProjectMember(r.Context(), proj.ID, user.ID)
	if err != nil {
		log.Printf("handleUpdateMember: get updated member %q in project %q: %v", username, proj.Slug, err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve updated member")
		return
	}

	writeJSON(w, http.StatusOK, memberToResponse(member, user.Username))
}

// handleRemoveMember handles DELETE /projects/{slug}/members/{username}.
// Removes a member from the project; returns 204 on success, 400 if target is last admin.
func (s *Server) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	username := chi.URLParam(r, "username")
	user, err := s.store.GetUserByUsername(r.Context(), username)
	if err != nil || user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	if err := s.store.RemoveMemberIfNotLastAdmin(r.Context(), proj.ID, user.ID); err != nil {
		log.Printf("handleRemoveMember: remove member %q from project %q: %v", username, proj.Slug, err)
		writeError(w, http.StatusBadRequest, "cannot remove last admin")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
