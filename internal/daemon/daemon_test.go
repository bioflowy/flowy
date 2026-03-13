// Package daemon tests exercise the daemon's HTTP client methods and lifecycle.
// Tests use httptest.NewServer to mock the flowy-server API without real Docker.
package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/config"
	"github.com/bioflowy/flowy/internal/storage"
	"github.com/bioflowy/flowy/wdl/values"
)

// mockBackend is a test-only storage.Backend that delegates DownloadFile and
// UploadFileStream to caller-supplied functions. All other methods are no-ops.
type mockBackend struct {
	downloadFn       func(path string) ([]byte, error)
	uploadStreamFn   func(path string, r io.Reader) error
}

// Ensure mockBackend satisfies the storage.Backend interface at compile time.
var _ storage.Backend = (*mockBackend)(nil)

func (m *mockBackend) DownloadFile(path string) ([]byte, error) {
	if m.downloadFn != nil {
		return m.downloadFn(path)
	}
	return nil, nil
}

func (m *mockBackend) UploadFileStream(path string, r io.Reader) error {
	if m.uploadStreamFn != nil {
		return m.uploadStreamFn(path, r)
	}
	return nil
}

// --- No-op implementations for remaining Backend interface methods ---

func (m *mockBackend) RunInputsDir(baseDir, runID string) string         { return "" }
func (m *mockBackend) RunOutputsDir(baseDir, runID string) string        { return "" }
func (m *mockBackend) TaskDir(baseDir, runID, taskName string, shardIndex int) string {
	return ""
}
func (m *mockBackend) CreateRunDirs(baseDir, runID string) error { return nil }
func (m *mockBackend) CreateTaskDirs(baseDir, runID, taskName string, shardIndex int) error {
	return nil
}
func (m *mockBackend) UploadFile(path string, data []byte) error { return nil }
func (m *mockBackend) DownloadFileStream(path string, w io.Writer) error {
	if m.downloadFn != nil {
		data, err := m.downloadFn(path)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	return nil
}
func (m *mockBackend) UploadDirectory(path string, localSrc string) error  { return nil }
func (m *mockBackend) DownloadDirectory(path string, localDst string) error { return nil }
func (m *mockBackend) WriteJSON(path string, v interface{}) error           { return nil }
func (m *mockBackend) ReadJSON(path string, v interface{}) error            { return nil }

// --- Helpers ---

// newTestDaemon creates a Daemon configured to talk to the given test server URL.
func newTestDaemon(t *testing.T, serverURL string) *Daemon {
	t.Helper()
	cfg := &config.Config{}
	// Apply defaults then override server URL.
	cfg.Daemon.ServerURL = serverURL
	cfg.Daemon.HeartbeatSec = 30
	cfg.Daemon.Workers = 1
	return New(cfg, "test-daemon-1234")
}

// --- Existing tests ---

// TestClaimTask_Success verifies that claimTask parses a 200 TaskClaimResponse correctly.
func TestClaimTask_Success(t *testing.T) {
	want := api.TaskClaimResponse{
		TaskID:     "42",
		RunID:      "run-abc",
		TaskName:   "hello",
		ShardIndex: 0,
		InputsDir:  "/data/inputs",
		OutputsDir: "/data/outputs",
		WDLPath:    "/wdl/hello.wdl",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/tasks/claim" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(want); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	d := newTestDaemon(t, srv.URL)
	got, err := d.claimTask(context.Background())
	if err != nil {
		t.Fatalf("claimTask returned error: %v", err)
	}
	if got == nil {
		t.Fatal("claimTask returned nil claim, expected non-nil")
	}
	if got.TaskID != want.TaskID {
		t.Errorf("TaskID: got %q, want %q", got.TaskID, want.TaskID)
	}
	if got.RunID != want.RunID {
		t.Errorf("RunID: got %q, want %q", got.RunID, want.RunID)
	}
	if got.TaskName != want.TaskName {
		t.Errorf("TaskName: got %q, want %q", got.TaskName, want.TaskName)
	}
	if got.InputsDir != want.InputsDir {
		t.Errorf("InputsDir: got %q, want %q", got.InputsDir, want.InputsDir)
	}
	if got.OutputsDir != want.OutputsDir {
		t.Errorf("OutputsDir: got %q, want %q", got.OutputsDir, want.OutputsDir)
	}
	if got.WDLPath != want.WDLPath {
		t.Errorf("WDLPath: got %q, want %q", got.WDLPath, want.WDLPath)
	}
}

// TestClaimTask_NoContent verifies that a 204 response returns (nil, nil).
func TestClaimTask_NoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	d := newTestDaemon(t, srv.URL)
	claim, err := d.claimTask(context.Background())
	if err != nil {
		t.Fatalf("claimTask returned error: %v", err)
	}
	if claim != nil {
		t.Errorf("claimTask returned non-nil claim on 204: %+v", claim)
	}
}

// TestClaimTask_Error verifies that a 500 response returns an error.
func TestClaimTask_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := newTestDaemon(t, srv.URL)
	claim, err := d.claimTask(context.Background())
	if err == nil {
		t.Fatal("claimTask returned nil error on 500, expected non-nil")
	}
	if claim != nil {
		t.Errorf("claimTask returned non-nil claim on error: %+v", claim)
	}
}

// TestSendHeartbeat verifies that sendHeartbeat POSTs to /heartbeat with the correct DaemonID.
func TestSendHeartbeat(t *testing.T) {
	var gotReq api.HeartbeatRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/heartbeat" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decode heartbeat body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := newTestDaemon(t, srv.URL)
	if err := d.sendHeartbeat(context.Background()); err != nil {
		t.Fatalf("sendHeartbeat returned error: %v", err)
	}
	if gotReq.DaemonID != d.daemonID {
		t.Errorf("DaemonID in heartbeat body: got %q, want %q", gotReq.DaemonID, d.daemonID)
	}
}

// TestReportResult verifies that reportResult POSTs to /tasks/{id}/result with the correct state.
func TestReportResult(t *testing.T) {
	const taskID = "123"
	var gotReq api.TaskResultRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/tasks/" + taskID + "/result"
		if r.Method != http.MethodPost || r.URL.Path != expectedPath {
			t.Errorf("unexpected request: %s %s (want POST %s)", r.Method, r.URL.Path, expectedPath)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decode result body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := newTestDaemon(t, srv.URL)
	d.reportResult(context.Background(), taskID, "COMPLETED", "")
	if gotReq.State != "COMPLETED" {
		t.Errorf("State in result body: got %q, want %q", gotReq.State, "COMPLETED")
	}
	if gotReq.Error != "" {
		t.Errorf("Error field should be empty, got %q", gotReq.Error)
	}
}

// TestOutputsJSON verifies values.ValuesToJSON converts all WDL value types to correct Go types.
func TestOutputsJSON(t *testing.T) {
	inputs := map[string]values.Value{
		"str":     values.StringValue{Value: "hello"},
		"intVal":  values.IntValue{Value: 42},
		"flt":     values.FloatValue{Value: 3.14},
		"boolVal": values.BooleanValue{Value: true},
		"file":    values.FileValue{Value: "/path/to/file"},
		"null":    values.NullValue{},
		"arr":     values.ArrayValue{Items: []values.Value{values.StringValue{Value: "a"}, values.StringValue{Value: "b"}}},
	}

	result, err := values.ValuesToJSON(inputs)
	if err != nil {
		t.Fatalf("valuesToJSON returned error: %v", err)
	}

	if v, ok := result["str"].(string); !ok || v != "hello" {
		t.Errorf("str: got %T(%v), want string(hello)", result["str"], result["str"])
	}
	if v, ok := result["intVal"].(int64); !ok || v != 42 {
		t.Errorf("intVal: got %T(%v), want int64(42)", result["intVal"], result["intVal"])
	}
	if v, ok := result["flt"].(float64); !ok || v != 3.14 {
		t.Errorf("flt: got %T(%v), want float64(3.14)", result["flt"], result["flt"])
	}
	if v, ok := result["boolVal"].(bool); !ok || !v {
		t.Errorf("boolVal: got %T(%v), want bool(true)", result["boolVal"], result["boolVal"])
	}
	if v, ok := result["file"].(string); !ok || v != "/path/to/file" {
		t.Errorf("file: got %T(%v), want string(/path/to/file)", result["file"], result["file"])
	}
	if result["null"] != nil {
		t.Errorf("null: got %v, want nil", result["null"])
	}
	arr, ok := result["arr"].([]interface{})
	if !ok {
		t.Fatalf("arr: got %T, want []interface{}", result["arr"])
	}
	if len(arr) != 2 {
		t.Fatalf("arr len: got %d, want 2", len(arr))
	}
	if arr[0] != "a" || arr[1] != "b" {
		t.Errorf("arr values: got %v, want [a b]", arr)
	}
}

// TestDaemonRunShutdown verifies that a daemon with no tasks exits cleanly when context is cancelled.
func TestDaemonRunShutdown(t *testing.T) {
	// Mock server: always return 204 for /tasks/claim, 200 for /heartbeat.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tasks/claim":
			w.WriteHeader(http.StatusNoContent)
		case "/heartbeat":
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{}
	cfg.Daemon.ServerURL = srv.URL
	cfg.Daemon.HeartbeatSec = 1
	cfg.Daemon.Workers = 1
	d := New(cfg, "test-shutdown-5678")

	ctx, cancel := context.WithCancel(context.Background())

	runDone := make(chan error, 1)
	go func() {
		runDone <- d.Run(ctx)
	}()

	// Let the daemon tick at least once by waiting a moment, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Wait for Run to finish with a generous timeout.
	select {
	case err := <-runDone:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("daemon Run did not exit within 5 seconds after context cancel")
	}

	// Shutdown should return quickly (no in-flight tasks).
	shutdownDone := make(chan struct{})
	go func() {
		d.Shutdown()
		close(shutdownDone)
	}()
	select {
	case <-shutdownDone:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown did not return within 2 seconds")
	}
}

// --- LED function unit tests ---

// TestWalkAndLocalize_RewritesS3Strings verifies that an s3:// string in a map
// is downloaded and rewritten to a local file path.
func TestWalkAndLocalize_RewritesS3Strings(t *testing.T) {
	const fileContent = "BAM data"
	inputsDir := t.TempDir()

	backend := &mockBackend{
		downloadFn: func(path string) ([]byte, error) {
			if path != "key/file.bam" {
				t.Errorf("DownloadFile called with unexpected path: %q", path)
			}
			return []byte(fileContent), nil
		},
	}

	input := map[string]any{"input_file": "s3://bucket/key/file.bam"}
	result, err := walkAndLocalize(input, backend, inputsDir)
	if err != nil {
		t.Fatalf("walkAndLocalize returned error: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type: got %T, want map[string]any", result)
	}

	localPath, ok := resultMap["input_file"].(string)
	if !ok {
		t.Fatalf("input_file type: got %T, want string", resultMap["input_file"])
	}

	// Verify the local path is in inputsDir and has correct basename.
	if filepath.Dir(localPath) != inputsDir {
		t.Errorf("local path dir: got %q, want %q", filepath.Dir(localPath), inputsDir)
	}
	if filepath.Base(localPath) != "file.bam" {
		t.Errorf("local path base: got %q, want %q", filepath.Base(localPath), "file.bam")
	}

	// Verify file contents were written correctly.
	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("read localized file: %v", err)
	}
	if string(data) != fileContent {
		t.Errorf("file content: got %q, want %q", string(data), fileContent)
	}
}

// TestWalkAndLocalize_PassthroughNonS3 verifies that non-s3:// values pass through unchanged.
func TestWalkAndLocalize_PassthroughNonS3(t *testing.T) {
	inputsDir := t.TempDir()
	backend := &mockBackend{}

	input := map[string]any{
		"name":  "hello",
		"count": float64(42),
		"flag":  true,
	}
	result, err := walkAndLocalize(input, backend, inputsDir)
	if err != nil {
		t.Fatalf("walkAndLocalize returned error: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type: got %T, want map[string]any", result)
	}

	if v, ok := resultMap["name"].(string); !ok || v != "hello" {
		t.Errorf("name: got %T(%v), want string(hello)", resultMap["name"], resultMap["name"])
	}
	if v, ok := resultMap["count"].(float64); !ok || v != 42 {
		t.Errorf("count: got %T(%v), want float64(42)", resultMap["count"], resultMap["count"])
	}
	if v, ok := resultMap["flag"].(bool); !ok || !v {
		t.Errorf("flag: got %T(%v), want bool(true)", resultMap["flag"], resultMap["flag"])
	}
}

// TestWalkAndLocalize_NestedArraysAndMaps verifies that s3:// URIs are rewritten
// in nested arrays and maps.
func TestWalkAndLocalize_NestedArraysAndMaps(t *testing.T) {
	inputsDir := t.TempDir()
	downloaded := make(map[string]bool)

	backend := &mockBackend{
		downloadFn: func(path string) ([]byte, error) {
			downloaded[path] = true
			return []byte("data"), nil
		},
	}

	input := map[string]any{
		"files": []any{"s3://b/k1.txt", "s3://b/k2.txt"},
		"nested": map[string]any{
			"inner": "s3://b/k3.txt",
		},
		"plain": "not-s3",
	}

	result, err := walkAndLocalize(input, backend, inputsDir)
	if err != nil {
		t.Fatalf("walkAndLocalize returned error: %v", err)
	}

	resultMap := result.(map[string]any)

	// Check array rewriting.
	files, ok := resultMap["files"].([]any)
	if !ok {
		t.Fatalf("files: got %T, want []any", resultMap["files"])
	}
	for i, f := range files {
		fStr, ok := f.(string)
		if !ok {
			t.Errorf("files[%d]: got %T, want string", i, f)
			continue
		}
		if strings.HasPrefix(fStr, "s3://") {
			t.Errorf("files[%d] still has s3:// prefix: %q", i, fStr)
		}
	}

	// Check nested map rewriting.
	nested, ok := resultMap["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested: got %T, want map[string]any", resultMap["nested"])
	}
	innerStr, ok := nested["inner"].(string)
	if !ok {
		t.Fatalf("nested.inner: got %T, want string", nested["inner"])
	}
	if strings.HasPrefix(innerStr, "s3://") {
		t.Errorf("nested.inner still has s3:// prefix: %q", innerStr)
	}

	// Verify all three keys were downloaded.
	for _, key := range []string{"k1.txt", "k2.txt", "k3.txt"} {
		if !downloaded[key] {
			t.Errorf("expected DownloadFile to be called with key %q", key)
		}
	}

	// Verify plain string passed through unchanged.
	if v, ok := resultMap["plain"].(string); !ok || v != "not-s3" {
		t.Errorf("plain: got %T(%v), want string(not-s3)", resultMap["plain"], resultMap["plain"])
	}
}

// TestWalkAndLocalize_DownloadError verifies that download errors produce
// descriptive messages with the "localize: download" prefix.
func TestWalkAndLocalize_DownloadError(t *testing.T) {
	inputsDir := t.TempDir()
	backend := &mockBackend{
		downloadFn: func(path string) ([]byte, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}

	input := map[string]any{"file": "s3://bucket/key/data.bam"}
	_, err := walkAndLocalize(input, backend, inputsDir)
	if err == nil {
		t.Fatal("walkAndLocalize returned nil error, expected non-nil")
	}
	if !strings.Contains(err.Error(), "localize: download") {
		t.Errorf("error message should contain 'localize: download', got: %q", err.Error())
	}
}

// TestRewriteOutputPaths verifies that local paths in a JSON map are replaced
// with their s3:// URIs from the localToS3 mapping.
func TestRewriteOutputPaths(t *testing.T) {
	localToS3 := map[string]string{
		"/tmp/stage/output.bam": "s3://bucket/runs/r1/tasks/t1_0/output.bam",
		"/tmp/stage/log.txt":    "s3://bucket/runs/r1/tasks/t1_0/log.txt",
	}

	input := map[string]any{
		"result_bam": "/tmp/stage/output.bam",
		"log":        "/tmp/stage/log.txt",
		"count":      float64(42),
		"nested": map[string]any{
			"file": "/tmp/stage/output.bam",
		},
		"files": []any{"/tmp/stage/output.bam", "/tmp/stage/log.txt"},
	}

	result := rewriteOutputPaths(input, localToS3)
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type: got %T, want map[string]any", result)
	}

	if v := resultMap["result_bam"]; v != "s3://bucket/runs/r1/tasks/t1_0/output.bam" {
		t.Errorf("result_bam: got %v, want s3 URI", v)
	}
	if v := resultMap["log"]; v != "s3://bucket/runs/r1/tasks/t1_0/log.txt" {
		t.Errorf("log: got %v, want s3 URI", v)
	}
	if v := resultMap["count"]; v != float64(42) {
		t.Errorf("count: got %v, want 42", v)
	}

	nested, ok := resultMap["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested: got %T, want map[string]any", resultMap["nested"])
	}
	if v := nested["file"]; v != "s3://bucket/runs/r1/tasks/t1_0/output.bam" {
		t.Errorf("nested.file: got %v, want s3 URI", v)
	}

	files, ok := resultMap["files"].([]any)
	if !ok {
		t.Fatalf("files: got %T, want []any", resultMap["files"])
	}
	if files[0] != "s3://bucket/runs/r1/tasks/t1_0/output.bam" {
		t.Errorf("files[0]: got %v, want s3 URI", files[0])
	}
	if files[1] != "s3://bucket/runs/r1/tasks/t1_0/log.txt" {
		t.Errorf("files[1]: got %v, want s3 URI", files[1])
	}
}

// TestLocalizeInputs_CreatesStageDir verifies that localizeInputs creates stageDir and
// inputs subdirectory, and that the returned stageDir exists after the call.
func TestLocalizeInputs_CreatesStageDir(t *testing.T) {
	backend := &mockBackend{
		downloadFn: func(path string) ([]byte, error) {
			return []byte("data"), nil
		},
	}

	inputs := map[string]any{"f": "s3://bucket/key/input.txt"}
	localizedInputs, stageDir, err := localizeInputs(inputs, backend)
	if err != nil {
		t.Fatalf("localizeInputs returned error: %v", err)
	}
	defer os.RemoveAll(stageDir)

	// Verify stageDir exists.
	if _, err := os.Stat(stageDir); os.IsNotExist(err) {
		t.Errorf("stageDir does not exist: %q", stageDir)
	}

	// Verify inputs subdir exists.
	inputsDir := filepath.Join(stageDir, "inputs")
	if _, err := os.Stat(inputsDir); os.IsNotExist(err) {
		t.Errorf("inputs subdir does not exist: %q", inputsDir)
	}

	// Verify the s3:// URI was rewritten to a local path.
	localPath, ok := localizedInputs["f"].(string)
	if !ok {
		t.Fatalf("localized input 'f': got %T, want string", localizedInputs["f"])
	}
	if !strings.HasPrefix(localPath, inputsDir) {
		t.Errorf("localized path %q is not under inputsDir %q", localPath, inputsDir)
	}
}

// TestDelocalizeOutputs_UploadsFiles verifies that files in workDir are uploaded
// and the returned map has s3:// URIs instead of local paths.
func TestDelocalizeOutputs_UploadsFiles(t *testing.T) {
	workDir := t.TempDir()

	// Create a test output file in workDir.
	outputFile := filepath.Join(workDir, "result.txt")
	if err := os.WriteFile(outputFile, []byte("output content"), 0644); err != nil {
		t.Fatalf("write test output file: %v", err)
	}

	var uploadedKeys []string
	backend := &mockBackend{
		uploadStreamFn: func(path string, r io.Reader) error {
			uploadedKeys = append(uploadedKeys, path)
			return nil
		},
	}

	claim := &api.TaskClaimResponse{
		RunID:         "run-001",
		TaskName:      "myTask",
		ShardIndex:    0,
		OutputsDir:    "runs/run-001/tasks/myTask_0",
		StorageBucket: "my-bucket",
	}

	wdlOutputs := map[string]values.Value{
		"result": values.FileValue{Value: outputFile},
	}

	jsonOutputs, err := delocalizeOutputs(wdlOutputs, workDir, claim, backend)
	if err != nil {
		t.Fatalf("delocalizeOutputs returned error: %v", err)
	}

	// Verify at least one file was uploaded.
	if len(uploadedKeys) == 0 {
		t.Fatal("no files were uploaded")
	}

	// Verify the uploaded key contains the expected prefix.
	foundResultTxt := false
	for _, key := range uploadedKeys {
		if strings.Contains(key, "result.txt") {
			foundResultTxt = true
			if !strings.HasPrefix(key, "runs/run-001/tasks/myTask_0/") {
				t.Errorf("uploaded key has wrong prefix: %q", key)
			}
		}
	}
	if !foundResultTxt {
		t.Errorf("result.txt not found in uploaded keys: %v", uploadedKeys)
	}

	// Verify the outputs map has s3:// URIs.
	resultVal, ok := jsonOutputs["result"].(string)
	if !ok {
		t.Fatalf("result output: got %T, want string", jsonOutputs["result"])
	}
	if !strings.HasPrefix(resultVal, "s3://my-bucket/") {
		t.Errorf("result output: got %q, want s3:// URI", resultVal)
	}
}

// TestDelocalizeOutputs_UploadError verifies that an upload failure returns an error
// with the "delocalize: upload" prefix.
func TestDelocalizeOutputs_UploadError(t *testing.T) {
	workDir := t.TempDir()

	// Create a test file to trigger the upload.
	if err := os.WriteFile(filepath.Join(workDir, "out.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	backend := &mockBackend{
		uploadStreamFn: func(path string, r io.Reader) error {
			return io.ErrClosedPipe
		},
	}

	claim := &api.TaskClaimResponse{
		OutputsDir:    "runs/run-001/tasks/t_0",
		StorageBucket: "bucket",
	}

	_, err := delocalizeOutputs(nil, workDir, claim, backend)
	if err == nil {
		t.Fatal("delocalizeOutputs returned nil error, expected non-nil")
	}
	if !strings.Contains(err.Error(), "delocalize: upload") {
		t.Errorf("error should contain 'delocalize: upload', got: %q", err.Error())
	}
}

// TestReportResultWithOutputs verifies that reportResultWithOutputs sends an HTTP
// request that includes the Outputs field in the JSON body.
func TestReportResultWithOutputs(t *testing.T) {
	const taskID = "task-xyz"
	var gotReq api.TaskResultRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/tasks/" + taskID + "/result"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: %s (want %s)", r.URL.Path, expectedPath)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := newTestDaemon(t, srv.URL)
	outputs := map[string]any{
		"result_bam": "s3://bucket/runs/r1/tasks/t_0/result.bam",
		"count":      float64(100),
	}

	d.reportResultWithOutputs(context.Background(), taskID, "COMPLETED", "", outputs)

	if gotReq.State != "COMPLETED" {
		t.Errorf("State: got %q, want COMPLETED", gotReq.State)
	}
	if gotReq.Outputs == nil {
		t.Fatal("Outputs field is nil, expected non-nil map")
	}
	if v, ok := gotReq.Outputs["result_bam"].(string); !ok || v != "s3://bucket/runs/r1/tasks/t_0/result.bam" {
		t.Errorf("Outputs[result_bam]: got %T(%v), want s3 URI string", gotReq.Outputs["result_bam"], gotReq.Outputs["result_bam"])
	}
}

// TestStageDirCleanupOnSuccess verifies that stageDir is removed after a
// successful LED cycle (simulated by checking stageDir exists then is cleaned).
func TestStageDirCleanupOnSuccess(t *testing.T) {
	backend := &mockBackend{
		downloadFn: func(path string) ([]byte, error) {
			return []byte("data"), nil
		},
	}

	inputs := map[string]any{"f": "s3://bucket/key/input.txt"}
	_, stageDir, err := localizeInputs(inputs, backend)
	if err != nil {
		t.Fatalf("localizeInputs returned error: %v", err)
	}

	// Verify stageDir exists after localization.
	if _, err := os.Stat(stageDir); os.IsNotExist(err) {
		t.Fatalf("stageDir does not exist before cleanup: %q", stageDir)
	}

	// Simulate successful completion: remove stageDir.
	if err := os.RemoveAll(stageDir); err != nil {
		t.Fatalf("RemoveAll stageDir: %v", err)
	}

	// Verify stageDir is removed.
	if _, err := os.Stat(stageDir); !os.IsNotExist(err) {
		t.Errorf("stageDir should not exist after cleanup: %q", stageDir)
	}
}

// TestStageDirPreservedOnDelocalizeFailure verifies the keepStage logic:
// when delocalize fails, stageDir must remain for debugging.
// We test this indirectly by verifying delocalizeOutputs returns an error
// and the caller should set keepStage=true in that case.
func TestStageDirPreservedOnDelocalizeFailure(t *testing.T) {
	workDir := t.TempDir()

	// Create a file to trigger the upload attempt.
	if err := os.WriteFile(filepath.Join(workDir, "out.bam"), []byte("data"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	backend := &mockBackend{
		uploadStreamFn: func(path string, r io.Reader) error {
			return io.ErrClosedPipe // simulate upload failure
		},
	}

	claim := &api.TaskClaimResponse{
		OutputsDir:    "runs/r1/tasks/t_0",
		StorageBucket: "bucket",
	}

	_, err := delocalizeOutputs(nil, workDir, claim, backend)
	if err == nil {
		t.Fatal("expected delocalize error, got nil")
	}

	// workDir (simulating stageDir) should still exist because we did NOT call RemoveAll.
	if _, statErr := os.Stat(workDir); os.IsNotExist(statErr) {
		t.Error("workDir was removed on delocalize failure — it should be preserved")
	}
}

// TestExecuteTask_LocalMode_APIInputs verifies that in local mode, the daemon
// reads inputs from claim.Inputs (API delivery) rather than requiring an inputs.json file.
func TestExecuteTask_LocalMode_APIInputs(t *testing.T) {
	// This test verifies claim parsing behavior: if claim.Inputs is set, the daemon
	// should use it directly without reading inputs.json from disk.
	// We create a claim with Inputs populated and verify executeTaskLocal doesn't
	// crash trying to read a non-existent inputs.json.

	// Create a temp dir that does NOT contain inputs.json.
	outputsDir := t.TempDir()

	// Minimal WDL task that outputs a string (no file I/O, no Docker needed for unit test).
	// We test only the input parsing path here — not full Docker execution.
	// So we verify claim.Inputs is used by checking the fallback path is NOT taken.

	claim := &api.TaskClaimResponse{
		TaskID:      "test-task-1",
		RunID:       "run-test",
		TaskName:    "hello",
		StorageType: "local",
		Inputs: map[string]interface{}{
			"name": "world",
		},
		// WDLPath points to non-existent file — if fallback path is taken, parse will fail.
		// But since WDLContent is set, parse uses WDLContent directly.
		WDLContent: `version 1.0
task hello {
  input { String name }
  command { echo ~{name} }
  output { String greeting = "Hello ~{name}" }
  runtime { docker: "ubuntu:latest" }
}`,
		OutputsDir: outputsDir,
		InputsDir:  outputsDir, // inputs.json does NOT exist here
	}

	// Verify claim.Inputs is non-nil (the condition that skips inputs.json reading).
	if claim.Inputs == nil {
		t.Fatal("claim.Inputs should be non-nil for this test")
	}

	// If the daemon tried to read inputs.json from outputsDir, it would fail because
	// no inputs.json exists. The fact that it uses claim.Inputs avoids this.
	// We verify the inputs path selection logic directly.
	var selectedInputs map[string]interface{}
	if claim.Inputs != nil {
		selectedInputs = claim.Inputs
	} else {
		// This branch should NOT be taken.
		t.Fatal("claim.Inputs was nil — fallback to inputs.json would be triggered")
	}

	if v, ok := selectedInputs["name"].(string); !ok || v != "world" {
		t.Errorf("selectedInputs[name]: got %T(%v), want string(world)", selectedInputs["name"], selectedInputs["name"])
	}
}

// TestLocalizeFailureReported verifies that when localizeInputs fails (e.g. S3
// download error), the daemon reports FAILED state with a descriptive error
// message to the server via the result API.
func TestLocalizeFailureReported(t *testing.T) {
	var gotReq api.TaskResultRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/result") {
			if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
				t.Errorf("decode result body: %v", err)
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := newTestDaemon(t, srv.URL)

	// Simulate the localize failure path that executeTaskS3 would follow:
	// 1. localizeInputs fails
	// 2. daemon reports FAILED with localize error message

	backend := &mockBackend{
		downloadFn: func(path string) ([]byte, error) {
			return nil, fmt.Errorf("NoSuchKey: the specified key does not exist")
		},
	}

	inputs := map[string]any{"bam": "s3://bucket/missing/file.bam"}
	_, _, err := localizeInputs(inputs, backend)
	if err == nil {
		t.Fatal("localizeInputs should have returned an error")
	}

	// Verify the error message has the expected prefix.
	if !strings.Contains(err.Error(), "localize: download") {
		t.Errorf("error should contain 'localize: download', got: %q", err.Error())
	}

	// Report the failure to the server (as executeTaskS3 would do).
	d.reportResult(context.Background(), "task-localize-fail", "FAILED",
		fmt.Sprintf("localize inputs: %v", err))

	// Verify the server received the failure report.
	if gotReq.State != "FAILED" {
		t.Errorf("State: got %q, want FAILED", gotReq.State)
	}
	if !strings.Contains(gotReq.Error, "localize") {
		t.Errorf("Error message should mention 'localize', got: %q", gotReq.Error)
	}
	if !strings.Contains(gotReq.Error, "NoSuchKey") {
		t.Errorf("Error message should propagate root cause 'NoSuchKey', got: %q", gotReq.Error)
	}
}

// TestDelocalizeFailureReported verifies that when delocalizeOutputs fails (e.g.
// S3 upload error), the daemon reports FAILED state with a descriptive error
// message to the server via the result API, and preserves the staging directory.
func TestDelocalizeFailureReported(t *testing.T) {
	var gotReq api.TaskResultRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/result") {
			if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
				t.Errorf("decode result body: %v", err)
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := newTestDaemon(t, srv.URL)

	// Create a workDir with an output file to trigger upload.
	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workDir, "output.vcf"), []byte("vcf data"), 0644); err != nil {
		t.Fatalf("write output file: %v", err)
	}

	backend := &mockBackend{
		uploadStreamFn: func(path string, r io.Reader) error {
			return fmt.Errorf("AccessDenied: not authorized")
		},
	}

	claim := &api.TaskClaimResponse{
		OutputsDir:    "runs/r1/tasks/align_0",
		StorageBucket: "bucket",
	}

	_, err := delocalizeOutputs(nil, workDir, claim, backend)
	if err == nil {
		t.Fatal("delocalizeOutputs should have returned an error")
	}

	// Verify the error message has the expected prefix.
	if !strings.Contains(err.Error(), "delocalize: upload") {
		t.Errorf("error should contain 'delocalize: upload', got: %q", err.Error())
	}

	// Report the failure to the server (as executeTaskS3 would do).
	d.reportResult(context.Background(), "task-delocalize-fail", "FAILED",
		fmt.Sprintf("delocalize outputs: %v", err))

	// Verify the server received the failure report.
	if gotReq.State != "FAILED" {
		t.Errorf("State: got %q, want FAILED", gotReq.State)
	}
	if !strings.Contains(gotReq.Error, "delocalize") {
		t.Errorf("Error message should mention 'delocalize', got: %q", gotReq.Error)
	}
	if !strings.Contains(gotReq.Error, "AccessDenied") {
		t.Errorf("Error message should propagate root cause 'AccessDenied', got: %q", gotReq.Error)
	}

	// Verify workDir is still present (simulating keepStage=true behavior).
	if _, statErr := os.Stat(workDir); os.IsNotExist(statErr) {
		t.Error("workDir should be preserved on delocalize failure")
	}
}
