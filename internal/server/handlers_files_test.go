package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/config"
	"github.com/bioflowy/flowy/internal/server"
	"github.com/bioflowy/flowy/internal/storage"
	"github.com/bioflowy/flowy/internal/store"
	"github.com/bioflowy/flowy/internal/store/sqlite"
)

type mockFileBackend struct {
	uploaded  map[string][]byte
	statSizes map[string]int64
	statErrs  map[string]error
}

func newMockFileBackend() *mockFileBackend {
	return &mockFileBackend{
		uploaded:  make(map[string][]byte),
		statSizes: make(map[string]int64),
		statErrs:  make(map[string]error),
	}
}

func (m *mockFileBackend) RunInputsDir(baseDir, runID string) string  { return "" }
func (m *mockFileBackend) RunOutputsDir(baseDir, runID string) string { return "" }
func (m *mockFileBackend) TaskDir(baseDir, runID, taskName string, shardIndex int) string {
	return ""
}
func (m *mockFileBackend) CreateRunDirs(baseDir, runID string) error { return nil }
func (m *mockFileBackend) CreateTaskDirs(baseDir, runID, taskName string, shardIndex int) error {
	return nil
}
func (m *mockFileBackend) UploadFile(path string, data []byte) error {
	m.uploaded[path] = append([]byte(nil), data...)
	m.statSizes[path] = int64(len(data))
	return nil
}
func (m *mockFileBackend) DownloadFile(path string) ([]byte, error) {
	data, ok := m.uploaded[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}
func (m *mockFileBackend) UploadDirectory(path string, localSrc string) error   { return nil }
func (m *mockFileBackend) DownloadDirectory(path string, localDst string) error { return nil }
func (m *mockFileBackend) WriteJSON(path string, v interface{}) error           { return nil }
func (m *mockFileBackend) ReadJSON(path string, v interface{}) error            { return nil }
func (m *mockFileBackend) UploadFileStream(path string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.uploaded[path] = data
	m.statSizes[path] = int64(len(data))
	return nil
}
func (m *mockFileBackend) DownloadFileStream(path string, w io.Writer) error {
	data, err := m.DownloadFile(path)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, bytes.NewReader(data))
	return err
}
func (m *mockFileBackend) Stat(path string) (int64, error) {
	if err, ok := m.statErrs[path]; ok {
		return 0, err
	}
	size, ok := m.statSizes[path]
	if !ok {
		return 0, os.ErrNotExist
	}
	return size, nil
}

func newAuthTestServerWithBackend(
	t *testing.T,
	backend storage.Backend,
	importRoot string,
) (*server.Server, *sqlite.SQLiteStore, func()) {
	t.Helper()

	st, err := sqlite.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}

	baseDir := t.TempDir()
	cfg := &config.Config{
		BaseDir:    baseDir,
		ImportRoot: importRoot,
		Server: config.ServerConfig{
			ListenAddr:          ":0",
			HeartbeatTimeoutSec: 60,
		},
		Auth: config.AuthConfig{
			Enabled:     true,
			JWTSecret:   string(testJWTSecret),
			DaemonToken: testDaemonToken,
		},
	}
	if backend == nil {
		backend = storage.NewBackend(cfg)
	}

	srv := server.New(cfg, st, backend)
	return srv, st, func() { st.Close() }
}

func setupManagedFileTest(
	t *testing.T,
	backend storage.Backend,
	importRoot string,
) (*server.Server, *sqlite.SQLiteStore, string, string, string, func()) {
	t.Helper()

	srv, st, cleanup := newAuthTestServerWithBackend(t, backend, importRoot)

	createTestUser(t, st, "exec-1", "executor_user", "pass", "executor")
	createTestUser(t, st, "view-1", "viewer_user", "pass", "user")

	proj := createRegistryTestProject(t, st, "exec-1")
	addProjectMember(t, st, proj.ID, "view-1", "viewer")

	execToken := issueTestToken(t, "exec-1", "executor_user", "executor")
	viewToken := issueTestToken(t, "view-1", "viewer_user", "user")

	return srv, st, proj.Slug, execToken, viewToken, cleanup
}

func createManagedFileRecord(t *testing.T, st *sqlite.SQLiteStore, projectID, path, label, fileType string, size int64) *store.ManagedFile {
	t.Helper()

	mf := &store.ManagedFile{
		ID:         uuid.New().String(),
		ProjectID:  projectID,
		Path:       path,
		Label:      label,
		FileType:   fileType,
		SourceType: "import",
		Size:       size,
	}
	if err := st.CreateManagedFile(context.Background(), mf); err != nil {
		t.Fatalf("CreateManagedFile: %v", err)
	}
	got, err := st.GetManagedFileByID(context.Background(), mf.ID)
	if err != nil {
		t.Fatalf("GetManagedFileByID: %v", err)
	}
	return got
}

func TestHandleListFiles_Empty(t *testing.T) {
	srv, _, slug, execToken, _, cleanup := setupManagedFileTest(t, nil, "")
	defer cleanup()

	w := doAuthRequest(t, srv, "GET", "/projects/"+slug+"/files", nil, execToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.ListManagedFilesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Files == nil {
		t.Fatal("expected non-nil files slice")
	}
	if len(resp.Files) != 0 {
		t.Fatalf("expected empty list, got %d items", len(resp.Files))
	}
}

func TestHandleListFiles_AuthAndFields(t *testing.T) {
	srv, st, slug, _, viewToken, cleanup := setupManagedFileTest(t, nil, "")
	defer cleanup()

	proj, err := st.GetProjectBySlug(context.Background(), slug)
	if err != nil {
		t.Fatalf("GetProjectBySlug: %v", err)
	}
	mf := createManagedFileRecord(t, st, proj.ID, "/tmp/example.fastq", "example.fastq", "fastq", 42)

	wNoAuth := doAuthRequest(t, srv, "GET", "/projects/"+slug+"/files", nil, "")
	if wNoAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", wNoAuth.Code)
	}

	w := doAuthRequest(t, srv, "GET", "/projects/"+slug+"/files", nil, viewToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.ListManagedFilesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(resp.Files))
	}
	got := resp.Files[0]
	if got.ID != mf.ID || got.Path != mf.Path || got.FileType != mf.FileType || got.Label != mf.Label || got.Size != mf.Size {
		t.Fatalf("unexpected file response: %+v", got)
	}
	if got.SourceType != "import" {
		t.Fatalf("unexpected source_type: %+v", got)
	}
	if got.CreatedAt == "" {
		t.Fatal("expected created_at")
	}
}

func TestHandleDeleteFile(t *testing.T) {
	srv, st, slug, execToken, _, cleanup := setupManagedFileTest(t, nil, "")
	defer cleanup()

	proj, err := st.GetProjectBySlug(context.Background(), slug)
	if err != nil {
		t.Fatalf("GetProjectBySlug: %v", err)
	}
	mf := createManagedFileRecord(t, st, proj.ID, "/tmp/delete-me.txt", "delete-me.txt", "", 12)

	w := doAuthRequest(t, srv, "DELETE", "/projects/"+slug+"/files/"+mf.ID, nil, execToken)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d (body: %s)", w.Code, w.Body.String())
	}

	if _, err := st.GetManagedFileByID(context.Background(), mf.ID); err == nil {
		t.Fatal("expected managed file to be deleted")
	}
}

func TestHandleDeleteFile_NonexistentAndCrossProject(t *testing.T) {
	srv, st, slug, execToken, _, cleanup := setupManagedFileTest(t, nil, "")
	defer cleanup()

	w := doAuthRequest(t, srv, "DELETE", "/projects/"+slug+"/files/nonexistent", nil, execToken)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent file, got %d", w.Code)
	}

	otherProj := createRegistryTestProject(t, st, "exec-1")
	mf := createManagedFileRecord(t, st, otherProj.ID, "/tmp/other.txt", "other.txt", "", 7)

	w = doAuthRequest(t, srv, "DELETE", "/projects/"+slug+"/files/"+mf.ID, nil, execToken)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-project file, got %d", w.Code)
	}
}

func TestHandleDeleteFile_ViewerForbidden(t *testing.T) {
	srv, st, slug, _, viewToken, cleanup := setupManagedFileTest(t, nil, "")
	defer cleanup()

	proj, err := st.GetProjectBySlug(context.Background(), slug)
	if err != nil {
		t.Fatalf("GetProjectBySlug: %v", err)
	}
	mf := createManagedFileRecord(t, st, proj.ID, "/tmp/viewer.txt", "viewer.txt", "", 8)

	w := doAuthRequest(t, srv, "DELETE", "/projects/"+slug+"/files/"+mf.ID, nil, viewToken)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestHandleImportFile_BrowserUpload(t *testing.T) {
	backend := newMockFileBackend()
	srv, st, slug, execToken, _, cleanup := setupManagedFileTest(t, backend, "")
	defer cleanup()

	w := doMultipartUploadWithFields(
		t,
		srv,
		"/projects/"+slug+"/files",
		"file",
		"reads.fastq",
		"acgt",
		map[string]string{"file_type": "fastq"},
		execToken,
	)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.ImportFileResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.File.Label != "reads.fastq" {
		t.Fatalf("label: got %q", resp.File.Label)
	}
	if resp.File.Size != 4 {
		t.Fatalf("size: got %d", resp.File.Size)
	}
	if resp.File.FileType != "fastq" {
		t.Fatalf("file_type: got %q", resp.File.FileType)
	}
	if resp.File.SourceType != "upload" {
		t.Fatalf("source_type: got %q", resp.File.SourceType)
	}
	if !strings.Contains(resp.File.Path, "/projects/"+slug+"/uploads/") || !strings.HasSuffix(resp.File.Path, "/reads.fastq") {
		t.Fatalf("unexpected path: %q", resp.File.Path)
	}

	proj, err := st.GetProjectBySlug(context.Background(), slug)
	if err != nil {
		t.Fatalf("GetProjectBySlug: %v", err)
	}
	files, err := st.ListManagedFiles(context.Background(), proj.ID)
	if err != nil {
		t.Fatalf("ListManagedFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 managed file, got %d", len(files))
	}
	if files[0].FileType != "fastq" {
		t.Fatalf("stored file_type: got %q", files[0].FileType)
	}
	if files[0].SourceType != "upload" {
		t.Fatalf("stored source_type: got %q", files[0].SourceType)
	}
}

func TestHandleImportFile_BrowserUploadPathTraversal(t *testing.T) {
	backend := newMockFileBackend()
	srv, _, slug, execToken, _, cleanup := setupManagedFileTest(t, backend, "")
	defer cleanup()

	w := doMultipartUpload(t, srv, "/projects/"+slug+"/files", "file", "../../etc/passwd", "evil", execToken)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.ImportFileResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.File.Label != "passwd" {
		t.Fatalf("expected sanitized label passwd, got %q", resp.File.Label)
	}
}

func TestHandleImportFile_S3URI(t *testing.T) {
	backend := newMockFileBackend()
	backend.statSizes["path/to/file.bam"] = 123
	srv, st, slug, execToken, _, cleanup := setupManagedFileTest(t, backend, "")
	defer cleanup()

	w := doAuthRequest(t, srv, "POST", "/projects/"+slug+"/files", api.ImportFileRequest{
		URI:      "s3://bucket/path/to/file.bam",
		FileType: "bam",
	}, execToken)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.ImportFileResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.File.Path != "s3://bucket/path/to/file.bam" || resp.File.Label != "file.bam" || resp.File.Size != 123 {
		t.Fatalf("unexpected response: %+v", resp.File)
	}
	if resp.File.FileType != "bam" {
		t.Fatalf("file_type: got %q", resp.File.FileType)
	}
	if resp.File.SourceType != "import" {
		t.Fatalf("source_type: got %q", resp.File.SourceType)
	}

	proj, err := st.GetProjectBySlug(context.Background(), slug)
	if err != nil {
		t.Fatalf("GetProjectBySlug: %v", err)
	}
	files, err := st.ListManagedFiles(context.Background(), proj.ID)
	if err != nil {
		t.Fatalf("ListManagedFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 managed file, got %d", len(files))
	}
	if files[0].FileType != "bam" {
		t.Fatalf("stored file_type: got %q", files[0].FileType)
	}
	if files[0].SourceType != "import" {
		t.Fatalf("stored source_type: got %q", files[0].SourceType)
	}
}

func TestHandleImportFile_S3URINotFound(t *testing.T) {
	backend := newMockFileBackend()
	backend.statErrs["nonexistent"] = os.ErrNotExist
	srv, _, slug, execToken, _, cleanup := setupManagedFileTest(t, backend, "")
	defer cleanup()

	w := doAuthRequest(t, srv, "POST", "/projects/"+slug+"/files", api.ImportFileRequest{
		URI: "s3://bucket/nonexistent",
	}, execToken)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestHandleImportFile_LocalPath(t *testing.T) {
	backend := newMockFileBackend()
	importRoot := filepath.Join(t.TempDir(), "imports")
	targetPath := filepath.Join(importRoot, "sample.fastq")
	backend.statSizes[targetPath] = 777

	srv, st, slug, execToken, _, cleanup := setupManagedFileTest(t, backend, importRoot)
	defer cleanup()

	w := doAuthRequest(t, srv, "POST", "/projects/"+slug+"/files", api.ImportFileRequest{
		URI:      targetPath,
		FileType: "fastq",
	}, execToken)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.ImportFileResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.File.Path != targetPath || resp.File.Label != "sample.fastq" || resp.File.Size != 777 {
		t.Fatalf("unexpected response: %+v", resp.File)
	}
	if resp.File.FileType != "fastq" {
		t.Fatalf("file_type: got %q", resp.File.FileType)
	}
	if resp.File.SourceType != "import" {
		t.Fatalf("source_type: got %q", resp.File.SourceType)
	}

	proj, err := st.GetProjectBySlug(context.Background(), slug)
	if err != nil {
		t.Fatalf("GetProjectBySlug: %v", err)
	}
	files, err := st.ListManagedFiles(context.Background(), proj.ID)
	if err != nil {
		t.Fatalf("ListManagedFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 managed file, got %d", len(files))
	}
	if files[0].FileType != "fastq" {
		t.Fatalf("stored file_type: got %q", files[0].FileType)
	}
	if files[0].SourceType != "import" {
		t.Fatalf("stored source_type: got %q", files[0].SourceType)
	}
}

func TestHandleImportFile_LocalPathOutsideImportRoot(t *testing.T) {
	backend := newMockFileBackend()
	importRoot := filepath.Join(t.TempDir(), "imports")
	targetPath := filepath.Join(t.TempDir(), "outside.fastq")
	backend.statSizes[targetPath] = 1

	srv, _, slug, execToken, _, cleanup := setupManagedFileTest(t, backend, importRoot)
	defer cleanup()

	w := doAuthRequest(t, srv, "POST", "/projects/"+slug+"/files", api.ImportFileRequest{
		URI: targetPath,
	}, execToken)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestHandleImportFile_LocalPathImportRootUnset(t *testing.T) {
	backend := newMockFileBackend()
	targetPath := filepath.Join(t.TempDir(), "sample.fastq")
	backend.statSizes[targetPath] = 1

	srv, _, slug, execToken, _, cleanup := setupManagedFileTest(t, backend, "")
	defer cleanup()

	w := doAuthRequest(t, srv, "POST", "/projects/"+slug+"/files", api.ImportFileRequest{
		URI: targetPath,
	}, execToken)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestHandleImportFile_ViewerForbidden(t *testing.T) {
	backend := newMockFileBackend()
	srv, _, slug, _, viewToken, cleanup := setupManagedFileTest(t, backend, "")
	defer cleanup()

	w := doAuthRequest(t, srv, "POST", "/projects/"+slug+"/files", api.ImportFileRequest{
		URI: "s3://bucket/path/to/file.bam",
	}, viewToken)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}
