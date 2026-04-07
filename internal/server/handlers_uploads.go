package server

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/bioflowy/flowy/internal/api"
)

// handleUpload handles POST /projects/{slug}/uploads.
// Accepts multipart/form-data with a single "file" field.
// Stores the file under {base_dir}/projects/{slug}/uploads/{uuid}/{filename}.
// Returns 201 with {"path": "<server-accessible path>"}.
// Requires executor role.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// Enforce 100 MB body limit BEFORE parsing multipart.
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form (max 100 MB)")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("missing 'file' field in multipart form: %v", err))
		return
	}
	defer file.Close()

	// Sanitize filename to prevent path traversal attacks.
	filename := filepath.Base(header.Filename)
	uploadID := uuid.New().String()
	storagePath := filepath.Join(s.cfg.BaseDir, "projects", proj.Slug, "uploads", uploadID, filename)

	if err := s.backend.UploadFileStream(storagePath, file); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store uploaded file")
		return
	}

	writeJSON(w, http.StatusCreated, api.UploadResponse{Path: storagePath})
}
