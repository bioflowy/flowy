// Package server registry handlers: push, pull, list, tag, and delete for WDL registry.
package server

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/wdl/parser"
)

// computeDigest computes the SHA-256 digest of content and returns it in
// "sha256:<hex>" format.
func computeDigest(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("sha256:%x", h)
}

// classifyWDL parses the given WDL content and returns "workflow" if the document
// defines at least one workflow, "tool" if it defines at least one task, or an
// error if the content cannot be parsed or defines neither.
func classifyWDL(name, content string) (string, error) {
	doc, err := parser.Parse(name, []byte(content))
	if err != nil {
		return "", fmt.Errorf("WDL parse error: %w", err)
	}
	if len(doc.Workflows) > 0 {
		return "workflow", nil
	}
	if len(doc.Tasks) > 0 {
		return "tool", nil
	}
	return "", fmt.Errorf("WDL file defines neither a task nor a workflow")
}

// validTagName reports whether s is a valid registry tag name.
// Tags must be between 1 and 128 characters and must not contain '@' or ':'.
func validTagName(s string) bool {
	return len(s) >= 1 && len(s) <= 128 && !strings.ContainsAny(s, "@:")
}

// handleRegistryPush handles POST /projects/{slug}/registry.
// Executors push WDL content; the server computes the SHA-256 digest and
// classifies the content as "tool" or "workflow" automatically.
// Returns 201 for new versions and 200 for idempotent re-pushes of identical
// content. Returns 422 for WDL parse errors and 409 for kind mismatches.
func (s *Server) handleRegistryPush(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req api.RegistryPushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !validSlug(req.Name) {
		writeError(w, http.StatusBadRequest, "invalid entry name: must be lowercase alphanumeric with hyphens, 1-64 chars")
		return
	}

	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	kind, err := classifyWDL(req.Name, req.Content)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	digest := computeDigest(req.Content)

	version, created, err := s.store.PushRegistryVersion(r.Context(), proj.ID, req.Name, kind, digest, req.Content)
	if err != nil {
		if strings.Contains(err.Error(), "kind") {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to push registry version")
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}

	writeJSON(w, status, api.RegistryPushResponse{
		Name:      req.Name,
		Digest:    version.Digest,
		Kind:      kind,
		VersionID: version.ID,
	})
}

// handleRegistryPull handles GET /projects/{slug}/registry/{name}.
// Callers must specify either ?tag= or ?digest= to select a version.
// Returns 404 with a specific error message when the tag or digest is not found.
func (s *Server) handleRegistryPull(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	name := chi.URLParam(r, "name")
	tag := r.URL.Query().Get("tag")
	digest := r.URL.Query().Get("digest")

	if tag == "" && digest == "" {
		writeError(w, http.StatusBadRequest, "must specify ?tag= or ?digest= query parameter")
		return
	}
	if tag != "" && digest != "" {
		writeError(w, http.StatusBadRequest, "specify either ?tag= or ?digest=, not both")
		return
	}

	if tag != "" {
		v, entry, err := s.store.GetRegistryVersionByTag(r.Context(), proj.ID, name, tag)
		if err != nil {
			writeError(w, http.StatusNotFound, fmt.Sprintf("tag %q not found for entry %q", tag, name))
			return
		}
		writeJSON(w, http.StatusOK, api.RegistryPullResponse{
			Name:    entry.Name,
			Digest:  v.Digest,
			Kind:    entry.Kind,
			Content: v.WDLContent,
		})
		return
	}

	// digest lookup
	v, entry, err := s.store.GetRegistryVersionByDigest(r.Context(), proj.ID, name, digest)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("digest %q not found for entry %q", digest, name))
		return
	}
	writeJSON(w, http.StatusOK, api.RegistryPullResponse{
		Name:    entry.Name,
		Digest:  v.Digest,
		Kind:    entry.Kind,
		Content: v.WDLContent,
	})
}

// handleRegistryList handles GET /projects/{slug}/registry.
// Stub: will be implemented in Plan 03.
func (s *Server) handleRegistryList(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

// handleRegistryListVersions handles GET /projects/{slug}/registry/{name}/versions.
// Stub: will be implemented in Plan 03.
func (s *Server) handleRegistryListVersions(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

// handleRegistryDelete handles DELETE /projects/{slug}/registry/{name}.
// Stub: will be implemented in Plan 03.
func (s *Server) handleRegistryDelete(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

// handleRegistryTag handles POST /projects/{slug}/registry/{name}/tags.
// Stub: will be implemented in Plan 03.
func (s *Server) handleRegistryTag(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

// handleRegistryDeleteTag handles DELETE /projects/{slug}/registry/{name}/tags/{tag}.
// Stub: will be implemented in Plan 03.
func (s *Server) handleRegistryDeleteTag(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}
