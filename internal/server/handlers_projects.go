// Package server project handlers: CRUD endpoints for project management.
package server

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/google/uuid"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/store"
)

// slugPattern matches valid project slugs: lowercase alphanumeric and hyphens,
// no leading/trailing hyphens, between 1 and 64 characters total.
var slugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$`)

// validSlug reports whether s is a valid project slug.
func validSlug(s string) bool {
	return len(s) >= 1 && len(s) <= 64 && slugPattern.MatchString(s)
}

// projectToResponse converts a store.Project to an api.ProjectResponse.
func projectToResponse(p *store.Project) api.ProjectResponse {
	return api.ProjectResponse{
		ID:          p.ID,
		Slug:        p.Slug,
		Description: p.Description,
		IsActive:    p.IsActive,
		CreatedAt:   p.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   p.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// handleCreateProject handles POST /projects.
// Any authenticated user can create a project and automatically becomes admin.
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req api.CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !validSlug(req.Slug) {
		writeError(w, http.StatusBadRequest, "invalid slug: must be lowercase alphanumeric with hyphens, 1-64 chars, no leading/trailing hyphens")
		return
	}

	now := time.Now().UTC()
	proj := &store.Project{
		ID:          uuid.New().String(),
		Slug:        req.Slug,
		Description: req.Description,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.store.CreateProject(r.Context(), proj); err != nil {
		log.Printf("handleCreateProject: create project %q: %v", req.Slug, err)
		writeError(w, http.StatusConflict, "slug already exists")
		return
	}

	// Auto-add creator as project admin.
	member := &store.ProjectMember{
		ProjectID: proj.ID,
		UserID:    claims.UserID,
		Role:      "admin",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.AddMember(r.Context(), member); err != nil {
		log.Printf("handleCreateProject: add creator as admin for project %q: %v", proj.Slug, err)
		writeError(w, http.StatusInternalServerError, "failed to add project creator as admin")
		return
	}

	writeJSON(w, http.StatusCreated, projectToResponse(proj))
}

// handleListProjects handles GET /projects.
// Global admins see all projects; other users see only projects they are members of.
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var projects []store.Project
	var err error

	if claims.Role == "admin" {
		projects, err = s.store.ListAllProjects(r.Context())
	} else {
		projects, err = s.store.ListProjectsForUser(r.Context(), claims.UserID)
	}

	if err != nil {
		log.Printf("handleListProjects: list projects for user %s: %v", claims.UserID, err)
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}

	resp := make([]api.ProjectResponse, len(projects))
	for i, p := range projects {
		p := p // avoid loop variable capture
		resp[i] = projectToResponse(&p)
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleGetProject handles GET /projects/{slug}.
// Returns the project details to members (enforced by projectRoleRequired middleware).
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	writeJSON(w, http.StatusOK, projectToResponse(proj))
}

// handleUpdateProject handles PUT /projects/{slug}.
// Updates the project description (admin only, enforced by projectRoleRequired middleware).
func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req api.UpdateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	proj.Description = req.Description
	proj.UpdatedAt = time.Now().UTC()

	if err := s.store.UpdateProject(r.Context(), proj); err != nil {
		log.Printf("handleUpdateProject: update project %q: %v", proj.Slug, err)
		writeError(w, http.StatusInternalServerError, "failed to update project")
		return
	}

	writeJSON(w, http.StatusOK, projectToResponse(proj))
}

// handleDeleteProject handles DELETE /projects/{slug}.
// Soft-deletes the project (admin only, enforced by projectRoleRequired middleware).
func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	if err := s.store.DeactivateProject(r.Context(), proj.ID); err != nil {
		log.Printf("handleDeleteProject: deactivate project %q: %v", proj.Slug, err)
		writeError(w, http.StatusInternalServerError, "failed to delete project")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
