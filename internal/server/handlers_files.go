package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/storage"
	"github.com/bioflowy/flowy/internal/store"
)

func managedFileToResponse(f *store.ManagedFile) api.ManagedFileResponse {
	return api.ManagedFileResponse{
		ID:         f.ID,
		Path:       f.Path,
		FileType:   f.FileType,
		Label:      f.Label,
		SourceType: f.SourceType,
		RunID:      f.RunID,
		Size:       f.Size,
		CreatedAt:  f.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// handleListFiles handles GET /projects/{slug}/files.
func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	files, err := s.store.ListManagedFiles(r.Context(), proj.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("list managed files: %v", err))
		return
	}

	resp := api.ListManagedFilesResponse{
		Files: make([]api.ManagedFileResponse, 0, len(files)),
	}
	for i := range files {
		resp.Files = append(resp.Files, managedFileToResponse(&files[i]))
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleDeleteFile handles DELETE /projects/{slug}/files/{id}.
func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	fileID := chi.URLParam(r, "id")
	mf, err := s.store.GetManagedFileByID(r.Context(), fileID)
	if err != nil || mf.ProjectID != proj.ID {
		writeError(w, http.StatusNotFound, fmt.Sprintf("managed file not found: %s", fileID))
		return
	}
	if err := s.store.DeleteManagedFile(r.Context(), fileID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("delete managed file: %v", err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleImportFile handles POST /projects/{slug}/files.
// It accepts multipart/form-data for browser uploads and JSON for URI imports.
func (s *Server) handleImportFile(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		s.handleImportFileBrowser(w, r, proj)
		return
	}
	s.handleImportFileURI(w, r, proj)
}

func (s *Server) handleImportFileBrowser(w http.ResponseWriter, r *http.Request, proj *store.Project) {
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

	filename := filepath.Base(header.Filename)
	fileType := strings.TrimSpace(r.FormValue("file_type"))
	uploadID := uuid.New().String()
	storagePath := filepath.Join(s.cfg.BaseDir, "projects", proj.Slug, "uploads", uploadID, filename)

	expectedPrefix := filepath.Clean(s.cfg.BaseDir) + string(filepath.Separator)
	if !strings.HasPrefix(storagePath, expectedPrefix) {
		writeError(w, http.StatusBadRequest, "invalid upload path")
		return
	}

	if err := s.backend.UploadFileStream(storagePath, file); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store uploaded file")
		return
	}

	size, err := s.backend.Stat(storagePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to stat uploaded file")
		return
	}

	mf, err := s.createManagedFileRecord(r.Context(), proj.ID, storagePath, filename, fileType, "upload", size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("create managed file: %v", err))
		return
	}

	writeJSON(w, http.StatusCreated, api.ImportFileResponse{File: managedFileToResponse(mf)})
}

func (s *Server) handleImportFileURI(w http.ResponseWriter, r *http.Request, proj *store.Project) {
	var req api.ImportFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	if strings.TrimSpace(req.URI) == "" {
		writeError(w, http.StatusBadRequest, "uri is required")
		return
	}
	req.FileType = strings.TrimSpace(req.FileType)

	if strings.HasPrefix(req.URI, "s3://") {
		s.handleImportS3URI(w, r, proj, req.URI, req.FileType)
		return
	}
	s.handleImportLocalPath(w, r, proj, req.URI, req.FileType)
}

func (s *Server) handleImportS3URI(w http.ResponseWriter, r *http.Request, proj *store.Project, uri string, fileType string) {
	bucket, key, err := storage.ParseS3URI(uri)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.cfg.Storage.Bucket != "" && bucket != s.cfg.Storage.Bucket {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("s3 bucket %q is not configured", bucket))
		return
	}

	size, err := s.backend.Stat(key)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid s3 uri: %v", err))
		return
	}

	mf, err := s.createManagedFileRecord(r.Context(), proj.ID, uri, filepath.Base(key), fileType, "import", size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("create managed file: %v", err))
		return
	}

	writeJSON(w, http.StatusCreated, api.ImportFileResponse{File: managedFileToResponse(mf)})
}

func (s *Server) handleImportLocalPath(w http.ResponseWriter, r *http.Request, proj *store.Project, uri string, fileType string) {
	if strings.TrimSpace(s.cfg.ImportRoot) == "" {
		writeError(w, http.StatusBadRequest, "local path import not configured")
		return
	}

	importRoot, err := filepath.Abs(s.cfg.ImportRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve import root")
		return
	}
	targetPath, err := filepath.Abs(uri)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid local path: %v", err))
		return
	}
	if !isWithinRoot(importRoot, targetPath) {
		writeError(w, http.StatusBadRequest, "local path must be within import_root")
		return
	}

	size, err := s.backend.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusBadRequest, "local path does not exist")
			return
		}
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid local path: %v", err))
		return
	}

	mf, err := s.createManagedFileRecord(r.Context(), proj.ID, targetPath, filepath.Base(targetPath), fileType, "import", size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("create managed file: %v", err))
		return
	}

	writeJSON(w, http.StatusCreated, api.ImportFileResponse{File: managedFileToResponse(mf)})
}

func (s *Server) createManagedFileRecord(
	ctx context.Context,
	projectID string,
	path string,
	label string,
	fileType string,
	sourceType string,
	size int64,
) (*store.ManagedFile, error) {
	mf := &store.ManagedFile{
		ID:         uuid.New().String(),
		ProjectID:  projectID,
		Path:       path,
		FileType:   fileType,
		Label:      label,
		SourceType: sourceType,
		Size:       size,
	}
	if err := s.store.CreateManagedFile(ctx, mf); err != nil {
		return nil, err
	}

	stored, err := s.store.GetManagedFileByID(ctx, mf.ID)
	if err != nil {
		log.Printf("managed file created but could not be reloaded: id=%s err=%v", mf.ID, err)
		mf.CreatedAt = mf.CreatedAt.UTC()
		return mf, nil
	}
	return stored, nil
}

func isWithinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
