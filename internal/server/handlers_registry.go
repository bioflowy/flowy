// Package server registry handlers: push, pull, list, tag, and delete for WDL registry.
package server

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/wdl/ast"
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

	entry, err := s.store.GetRegistryEntry(r.Context(), proj.ID, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load registry entry")
		return
	}
	if err := s.store.UpsertRegistryTag(r.Context(), entry.ID, "latest", version.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to assign latest tag")
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
// Returns summary rows for all active entries in the project. Accessible by viewers.
func (s *Server) handleRegistryList(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	items, err := s.store.ListRegistryEntries(r.Context(), proj.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list registry entries")
		return
	}

	resp := make([]api.RegistryEntryResponse, 0, len(items))
	for _, item := range items {
		tags := item.Tags
		if tags == nil {
			tags = []string{}
		}
		resp = append(resp, api.RegistryEntryResponse{
			Name:         item.Name,
			Kind:         item.Kind,
			VersionCount: item.VersionCount,
			Tags:         tags,
			CreatedAt:    item.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleRegistryListVersions handles GET /projects/{slug}/registry/{name}/versions.
// Returns all versions for the named entry with their associated tag names. Accessible by viewers.
func (s *Server) handleRegistryListVersions(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	name := chi.URLParam(r, "name")

	items, err := s.store.ListRegistryVersions(r.Context(), proj.ID, name)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("entry %q not found", name))
		return
	}

	resp := make([]api.RegistryVersionResponse, 0, len(items))
	for _, item := range items {
		tags := item.Tags
		if tags == nil {
			tags = []string{}
		}
		resp = append(resp, api.RegistryVersionResponse{
			ID:        item.ID,
			Digest:    item.Digest,
			Tags:      tags,
			CreatedAt: item.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleRegistryDelete handles DELETE /projects/{slug}/registry/{name}.
// Soft-deletes the named entry. Admin only.
func (s *Server) handleRegistryDelete(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	name := chi.URLParam(r, "name")

	if err := s.store.DeleteRegistryEntry(r.Context(), proj.ID, name); err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("entry %q not found", name))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleRegistryTag handles POST /projects/{slug}/registry/{name}/tags.
// Creates or reassigns a tag to the version identified by digest. Admin only.
func (s *Server) handleRegistryTag(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	name := chi.URLParam(r, "name")

	var req api.RegistryTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !validTagName(req.Tag) {
		writeError(w, http.StatusBadRequest, "invalid tag name: must be 1-128 chars, no '@' or ':'")
		return
	}

	if req.Digest == "" {
		writeError(w, http.StatusBadRequest, "digest is required")
		return
	}

	entry, err := s.store.GetRegistryEntry(r.Context(), proj.ID, name)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("entry %q not found", name))
		return
	}

	version, _, err := s.store.GetRegistryVersionByDigest(r.Context(), proj.ID, name, req.Digest)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("version with digest %q not found for entry %q", req.Digest, name))
		return
	}

	if err := s.store.UpsertRegistryTag(r.Context(), entry.ID, req.Tag, version.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to upsert tag")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"tag":    req.Tag,
		"digest": req.Digest,
		"entry":  name,
	})
}

// handleRegistryDeleteTag handles DELETE /projects/{slug}/registry/{name}/tags/{tag}.
// Removes the named tag; the version is not affected. Admin only.
func (s *Server) handleRegistryDeleteTag(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	name := chi.URLParam(r, "name")
	tag := chi.URLParam(r, "tag")

	entry, err := s.store.GetRegistryEntry(r.Context(), proj.ID, name)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("entry %q not found", name))
		return
	}

	if err := s.store.DeleteRegistryTag(r.Context(), entry.ID, tag); err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("tag %q not found for entry %q", tag, name))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleRegistryInputs handles GET /projects/{slug}/registry/{name}/inputs?tag=<tag> or ?digest=<digest>.
// Returns the declared inputs for the WDL workflow or task stored under the selected version.
// Accessible by project viewers.
func (s *Server) handleRegistryInputs(w http.ResponseWriter, r *http.Request) {
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

	var vContent string
	if digest != "" {
		v, _, err := s.store.GetRegistryVersionByDigest(r.Context(), proj.ID, name, digest)
		if err != nil {
			writeError(w, http.StatusNotFound, fmt.Sprintf("digest %q not found for entry %q", digest, name))
			return
		}
		vContent = v.WDLContent
	} else {
		v, _, err := s.store.GetRegistryVersionByTag(r.Context(), proj.ID, name, tag)
		if err != nil {
			if strings.HasPrefix(tag, "sha256:") {
				v, _, err = s.store.GetRegistryVersionByDigest(r.Context(), proj.ID, name, tag)
				if err != nil {
					writeError(w, http.StatusNotFound, fmt.Sprintf("digest %q not found for entry %q", tag, name))
					return
				}
				vContent = v.WDLContent
			} else {
				writeError(w, http.StatusNotFound, fmt.Sprintf("tag %q not found for entry %q", tag, name))
				return
			}
		} else {
			vContent = v.WDLContent
		}
	}

	doc, err := parser.Parse(name, []byte(vContent))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "WDL parse error")
		return
	}

	inputs := extractInputs(doc)
	writeJSON(w, http.StatusOK, api.RegistryInputsResponse{Inputs: inputs})
}

// extractInputs extracts the declared input list from the first workflow or task in doc.
func extractInputs(doc *ast.Document) []api.RegistryInputItem {
	var decls []*ast.Decl
	var paramMeta map[string]any
	if len(doc.Workflows) > 0 {
		decls = doc.Workflows[0].Inputs
		paramMeta = doc.Workflows[0].ParamMeta
	} else if len(doc.Tasks) > 0 {
		decls = doc.Tasks[0].Inputs
		paramMeta = doc.Tasks[0].ParamMeta
	}

	items := make([]api.RegistryInputItem, 0, len(decls))
	for _, d := range decls {
		items = append(items, api.RegistryInputItem{
			Name:     d.Name,
			Type:     d.Type.TypeName(),
			Optional: d.Type.IsOptional(),
			Default:  declDefaultString(d.Expr),
			FileType: parameterMetaType(paramMeta, d.Name),
		})
	}
	return items
}

func parameterMetaType(paramMeta map[string]any, name string) *string {
	if len(paramMeta) == 0 {
		return nil
	}
	rawField, ok := paramMeta[name]
	if !ok {
		return nil
	}
	fieldMap, ok := rawField.(map[string]any)
	if !ok {
		return nil
	}
	value, ok := fieldMap["type"].(string)
	if !ok || value == "" {
		return nil
	}
	return &value
}

// declDefaultString converts a simple literal AST expression to its string representation.
// Returns nil for complex or nil expressions (interpolated strings, function calls, etc.).
func declDefaultString(e ast.Expr) *string {
	if e == nil {
		return nil
	}
	var s string
	switch v := e.(type) {
	case *ast.IntLit:
		s = strconv.FormatInt(v.Value, 10)
	case *ast.FloatLit:
		s = strconv.FormatFloat(v.Value, 'f', -1, 64)
	case *ast.BoolLit:
		if v.Value {
			s = "true"
		} else {
			s = "false"
		}
	case *ast.NullLit:
		s = "null"
	case *ast.StringExpr:
		if len(v.Parts) == 0 {
			s = ""
		} else if len(v.Parts) == 1 {
			raw, ok := v.Parts[0].(*ast.RawStringPart)
			if !ok {
				return nil // interpolated — cannot reduce to plain string
			}
			s = raw.Val
		} else {
			return nil // multi-part interpolated string
		}
	default:
		return nil
	}
	return &s
}
