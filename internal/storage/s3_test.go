// Package storage — unit tests for S3Backend using mock S3 client and
// transfermanager, covering S3-01 through S3-05 requirements without
// requiring real AWS or MinIO infrastructure.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// ---------------------------------------------------------------------------
// Mock S3 client
// ---------------------------------------------------------------------------

// mockS3Client implements S3APIClient for unit testing.
// Each method delegates to the corresponding function field; if nil a default
// (empty output, nil error for PutObject; empty output for List; error for
// GetObject) is used so individual tests only need to set the field they care
// about.
type mockS3Client struct {
	putObjectFn   func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	getObjectFn   func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	listObjectsFn func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)

	// lastPutInput records the most recent PutObjectInput for assertions.
	lastPutInput *s3.PutObjectInput
	// putCallCount records how many times PutObject was called.
	putCallCount int
}

func (m *mockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	m.lastPutInput = params
	m.putCallCount++
	if m.putObjectFn != nil {
		return m.putObjectFn(ctx, params, optFns...)
	}
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if m.getObjectFn != nil {
		return m.getObjectFn(ctx, params, optFns...)
	}
	return nil, fmt.Errorf("GetObject: no mock configured")
}

func (m *mockS3Client) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	if m.listObjectsFn != nil {
		return m.listObjectsFn(ctx, params, optFns...)
	}
	return &s3.ListObjectsV2Output{}, nil
}

// ---------------------------------------------------------------------------
// Mock transfer manager
// ---------------------------------------------------------------------------

// mockTransferManager implements TransferManagerUploader for unit testing.
// It records calls so tests can assert the upload was routed through it.
type mockTransferManager struct {
	uploadFn func(ctx context.Context, input *transfermanager.UploadObjectInput, opts ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error)

	// lastInput records the most recent UploadObjectInput for assertions.
	lastInput *transfermanager.UploadObjectInput
	// callCount records how many times UploadObject was called.
	callCount int
	// capturedBody holds the bytes read from the last call's Body for assertions.
	capturedBody []byte
}

func (m *mockTransferManager) UploadObject(ctx context.Context, input *transfermanager.UploadObjectInput, opts ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error) {
	m.lastInput = input
	m.callCount++

	// Read and capture body bytes so tests can assert on content.
	if input.Body != nil {
		data, err := io.ReadAll(input.Body)
		if err != nil {
			return nil, fmt.Errorf("mockTransferManager: read body: %w", err)
		}
		m.capturedBody = data
		// Restore so any other reader in the pipeline can still read (not
		// strictly necessary for tests, but keeps mock behaviour clean).
		input.Body = bytes.NewReader(data)
	}

	if m.uploadFn != nil {
		return m.uploadFn(ctx, input, opts...)
	}
	return &transfermanager.UploadObjectOutput{}, nil
}

// ---------------------------------------------------------------------------
// Test helper
// ---------------------------------------------------------------------------

// newTestBackend creates an S3Backend with injected mocks.
func newTestBackend(bucket, prefix string, threshold int64, s3c *mockS3Client, tm *mockTransferManager) *S3Backend {
	return newS3BackendForTest(bucket, prefix, threshold, s3c, tm)
}

// readCloser wraps a byte slice as an io.ReadCloser for use in mock GetObject
// responses.
func readCloser(data []byte) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(data))
}

// ---------------------------------------------------------------------------
// Task 1: Core upload / download tests
// ---------------------------------------------------------------------------

// TestS3Backend_UploadFile verifies that UploadFile routes data through the
// transfer manager with the correct bucket and key (S3-01).
func TestS3Backend_UploadFile(t *testing.T) {
	mock := &mockS3Client{}
	tm := &mockTransferManager{}
	b := newTestBackend("test-bucket", "pfx", 0, mock, tm)

	key := "pfx/runs/r1/inputs/file.txt"
	data := []byte("hello")
	if err := b.UploadFile(key, data); err != nil {
		t.Fatalf("UploadFile returned unexpected error: %v", err)
	}

	if tm.callCount != 1 {
		t.Errorf("expected 1 transfermanager call, got %d", tm.callCount)
	}
	if got := aws.ToString(tm.lastInput.Bucket); got != "test-bucket" {
		t.Errorf("bucket: want %q, got %q", "test-bucket", got)
	}
	if got := aws.ToString(tm.lastInput.Key); got != key {
		t.Errorf("key: want %q, got %q", key, got)
	}
	if !bytes.Equal(tm.capturedBody, data) {
		t.Errorf("body: want %q, got %q", data, tm.capturedBody)
	}
}

// TestS3Backend_UploadFile_Error verifies that upload errors are wrapped with
// the "s3:" prefix.
func TestS3Backend_UploadFile_Error(t *testing.T) {
	tm := &mockTransferManager{
		uploadFn: func(_ context.Context, _ *transfermanager.UploadObjectInput, _ ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error) {
			return nil, fmt.Errorf("access denied")
		},
	}
	b := newTestBackend("test-bucket", "", 0, &mockS3Client{}, tm)

	err := b.UploadFile("some/key", []byte("data"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !contains(err.Error(), "s3:") {
		t.Errorf("error should contain %q prefix, got: %v", "s3:", err)
	}
	if !contains(err.Error(), "access denied") {
		t.Errorf("error should contain original message, got: %v", err)
	}
}

// TestS3Backend_DownloadFile verifies that GetObject body bytes are returned
// unchanged (S3-02).
func TestS3Backend_DownloadFile(t *testing.T) {
	want := []byte("world")
	mock := &mockS3Client{
		getObjectFn: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return &s3.GetObjectOutput{Body: readCloser(want)}, nil
		},
	}
	b := newTestBackend("bucket", "", 0, mock, &mockTransferManager{})

	got, err := b.DownloadFile("any/key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("body: want %q, got %q", want, got)
	}
}

// TestS3Backend_DownloadFile_Error verifies that GetObject errors are wrapped.
func TestS3Backend_DownloadFile_Error(t *testing.T) {
	mock := &mockS3Client{
		getObjectFn: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return nil, fmt.Errorf("internal S3 error")
		},
	}
	b := newTestBackend("bucket", "", 0, mock, &mockTransferManager{})

	_, err := b.DownloadFile("some/key")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !contains(err.Error(), "s3:") {
		t.Errorf("error should contain %q prefix, got: %v", "s3:", err)
	}
}

// TestS3Backend_DownloadFile_NotFound verifies that NoSuchKey is mapped to
// os.ErrNotExist for cross-backend error consistency (S3-02).
func TestS3Backend_DownloadFile_NotFound(t *testing.T) {
	mock := &mockS3Client{
		getObjectFn: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return nil, &types.NoSuchKey{}
		},
	}
	b := newTestBackend("bucket", "", 0, mock, &mockTransferManager{})

	_, err := b.DownloadFile("missing/key")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected errors.Is(err, os.ErrNotExist), got: %v", err)
	}
}

// TestS3Backend_UploadFileStream verifies that streaming upload routes through
// the transfer manager with correct key and body (S3-01).
func TestS3Backend_UploadFileStream(t *testing.T) {
	tm := &mockTransferManager{}
	b := newTestBackend("bucket", "pre", 0, &mockS3Client{}, tm)

	key := "pre/runs/r2/inputs/data.bin"
	payload := []byte("stream-data")
	if err := b.UploadFileStream(key, bytes.NewReader(payload)); err != nil {
		t.Fatalf("UploadFileStream returned unexpected error: %v", err)
	}

	if tm.callCount != 1 {
		t.Errorf("expected 1 call, got %d", tm.callCount)
	}
	if got := aws.ToString(tm.lastInput.Key); got != key {
		t.Errorf("key: want %q, got %q", key, got)
	}
	if !bytes.Equal(tm.capturedBody, payload) {
		t.Errorf("body: want %q, got %q", payload, tm.capturedBody)
	}
}

// TestS3Backend_DownloadFileStream verifies that the object body is written to
// the provided io.Writer (S3-02).
func TestS3Backend_DownloadFileStream(t *testing.T) {
	want := []byte("streamed-bytes")
	mock := &mockS3Client{
		getObjectFn: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return &s3.GetObjectOutput{Body: readCloser(want)}, nil
		},
	}
	b := newTestBackend("bucket", "", 0, mock, &mockTransferManager{})

	var buf bytes.Buffer
	if err := b.DownloadFileStream("any/key", &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("written bytes: want %q, got %q", want, buf.Bytes())
	}
}

// ---------------------------------------------------------------------------
// Task 2: Key layout, scatter, directory, and JSON tests
// ---------------------------------------------------------------------------

// TestS3Backend_TaskDir verifies that TaskDir produces the expected S3 key
// with prefix, runID, taskName, and shardIndex (S3-05).
func TestS3Backend_TaskDir(t *testing.T) {
	b := newTestBackend("bucket", "myprefix", 0, &mockS3Client{}, &mockTransferManager{})
	got := b.TaskDir("ignored", "run-123", "my_task", 0)
	want := "myprefix/runs/run-123/tasks/my_task_0"
	if got != want {
		t.Errorf("TaskDir: want %q, got %q", want, got)
	}
}

// TestS3Backend_TaskDir_ScatterDistinct verifies that different shardIndex
// values produce different keys, preventing scatter collision (S3-05).
func TestS3Backend_TaskDir_ScatterDistinct(t *testing.T) {
	b := newTestBackend("bucket", "myprefix", 0, &mockS3Client{}, &mockTransferManager{})
	key0 := b.TaskDir("ignored", "run-123", "scatter_task", 0)
	key3 := b.TaskDir("ignored", "run-123", "scatter_task", 3)

	if key0 == key3 {
		t.Errorf("scatter keys must be distinct; both returned %q", key0)
	}
	if !hasSuffix(key0, "_0") {
		t.Errorf("shard 0 key should end with _0, got %q", key0)
	}
	if !hasSuffix(key3, "_3") {
		t.Errorf("shard 3 key should end with _3, got %q", key3)
	}
}

// TestS3Backend_RunInputsDir verifies the RunInputsDir key format.
func TestS3Backend_RunInputsDir(t *testing.T) {
	b := newTestBackend("bucket", "myprefix", 0, &mockS3Client{}, &mockTransferManager{})
	got := b.RunInputsDir("ignored", "run-123")
	want := "myprefix/runs/run-123/inputs"
	if got != want {
		t.Errorf("RunInputsDir: want %q, got %q", want, got)
	}
}

// TestS3Backend_RunOutputsDir verifies the RunOutputsDir key format.
func TestS3Backend_RunOutputsDir(t *testing.T) {
	b := newTestBackend("bucket", "myprefix", 0, &mockS3Client{}, &mockTransferManager{})
	got := b.RunOutputsDir("ignored", "run-123")
	want := "myprefix/runs/run-123/outputs"
	if got != want {
		t.Errorf("RunOutputsDir: want %q, got %q", want, got)
	}
}

// TestS3Backend_KeyJoin_EmptyPrefix verifies that an empty prefix produces
// keys without a leading slash.
func TestS3Backend_KeyJoin_EmptyPrefix(t *testing.T) {
	b := newTestBackend("bucket", "", 0, &mockS3Client{}, &mockTransferManager{})
	got := b.TaskDir("ignored", "run-123", "task", 0)
	want := "runs/run-123/tasks/task_0"
	if got != want {
		t.Errorf("KeyJoin empty prefix: want %q, got %q", want, got)
	}
	if len(got) > 0 && got[0] == '/' {
		t.Errorf("key should not have leading slash, got %q", got)
	}
}

// TestS3Backend_CreateRunDirs_Noop verifies that CreateRunDirs returns nil
// without calling PutObject.
func TestS3Backend_CreateRunDirs_Noop(t *testing.T) {
	mock := &mockS3Client{}
	b := newTestBackend("bucket", "pfx", 0, mock, &mockTransferManager{})

	if err := b.CreateRunDirs("base", "run-999"); err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
	if mock.putCallCount != 0 {
		t.Errorf("expected no PutObject calls, got %d", mock.putCallCount)
	}
}

// TestS3Backend_CreateTaskDirs_Noop verifies that CreateTaskDirs returns nil
// without calling PutObject.
func TestS3Backend_CreateTaskDirs_Noop(t *testing.T) {
	mock := &mockS3Client{}
	b := newTestBackend("bucket", "pfx", 0, mock, &mockTransferManager{})

	if err := b.CreateTaskDirs("base", "run-999", "task", 0); err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
	if mock.putCallCount != 0 {
		t.Errorf("expected no PutObject calls, got %d", mock.putCallCount)
	}
}

// TestS3Backend_WriteJSON verifies that WriteJSON marshals the struct and
// uploads it via the transfer manager.
func TestS3Backend_WriteJSON(t *testing.T) {
	type payload struct{ Name string }

	tm := &mockTransferManager{}
	b := newTestBackend("bucket", "", 0, &mockS3Client{}, tm)

	key := "runs/r1/result.json"
	if err := b.WriteJSON(key, payload{Name: "alice"}); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	if tm.callCount != 1 {
		t.Errorf("expected 1 transfermanager call, got %d", tm.callCount)
	}
	if got := aws.ToString(tm.lastInput.Key); got != key {
		t.Errorf("key: want %q, got %q", key, got)
	}
	// Verify body contains expected JSON field.
	if !contains(string(tm.capturedBody), `"alice"`) {
		t.Errorf("JSON body should contain alice, got %q", string(tm.capturedBody))
	}
}

// TestS3Backend_ReadJSON verifies that ReadJSON downloads bytes and unmarshals
// them into the target struct.
func TestS3Backend_ReadJSON(t *testing.T) {
	type payload struct{ Name string }

	mock := &mockS3Client{
		getObjectFn: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return &s3.GetObjectOutput{Body: readCloser([]byte(`{"Name":"test"}`))}, nil
		},
	}
	b := newTestBackend("bucket", "", 0, mock, &mockTransferManager{})

	var p payload
	if err := b.ReadJSON("runs/r1/result.json", &p); err != nil {
		t.Fatalf("ReadJSON error: %v", err)
	}
	if p.Name != "test" {
		t.Errorf("Name: want %q, got %q", "test", p.Name)
	}
}

// TestS3Backend_UploadDirectory verifies that UploadDirectory walks the local
// directory and uploads each file with the correct S3 key (S3-01).
func TestS3Backend_UploadDirectory(t *testing.T) {
	// Build a temp directory with two files.
	tmpDir := t.TempDir()
	files := map[string][]byte{
		"a.txt":        []byte("content-a"),
		"sub/b.txt":    []byte("content-b"),
	}
	for rel, data := range files {
		full := filepath.Join(tmpDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	tm := &mockTransferManager{}
	uploadedKeys := map[string][]byte{}
	tm.uploadFn = func(_ context.Context, input *transfermanager.UploadObjectInput, _ ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error) {
		data, _ := io.ReadAll(input.Body)
		uploadedKeys[aws.ToString(input.Key)] = data
		return &transfermanager.UploadObjectOutput{}, nil
	}

	b := newTestBackend("bucket", "", 0, &mockS3Client{}, tm)
	keyPrefix := "uploads/run-1"

	if err := b.UploadDirectory(keyPrefix, tmpDir); err != nil {
		t.Fatalf("UploadDirectory error: %v", err)
	}

	if len(uploadedKeys) != 2 {
		t.Errorf("expected 2 uploads, got %d", len(uploadedKeys))
	}
	for rel, wantData := range files {
		expectedKey := keyPrefix + "/" + rel
		gotData, ok := uploadedKeys[expectedKey]
		if !ok {
			t.Errorf("expected key %q to be uploaded, got keys: %v", expectedKey, keysOf(uploadedKeys))
			continue
		}
		if !bytes.Equal(gotData, wantData) {
			t.Errorf("key %q: body want %q, got %q", expectedKey, wantData, gotData)
		}
	}
}

// TestS3Backend_DownloadDirectory verifies that DownloadDirectory lists objects
// by prefix and downloads each one into the correct local path (S3-02).
func TestS3Backend_DownloadDirectory(t *testing.T) {
	prefix := "downloads/run-2/"
	objects := []struct {
		key     string
		content []byte
	}{
		{prefix + "file1.txt", []byte("alpha")},
		{prefix + "nested/file2.txt", []byte("beta")},
	}

	mock := &mockS3Client{}
	mock.listObjectsFn = func(_ context.Context, params *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
		items := make([]types.Object, len(objects))
		for i, o := range objects {
			key := o.key
			items[i] = types.Object{Key: &key}
		}
		f := false
		return &s3.ListObjectsV2Output{Contents: items, IsTruncated: &f}, nil
	}
	mock.getObjectFn = func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
		key := aws.ToString(params.Key)
		for _, o := range objects {
			if o.key == key {
				return &s3.GetObjectOutput{Body: readCloser(o.content)}, nil
			}
		}
		return nil, fmt.Errorf("key not found: %s", key)
	}

	b := newTestBackend("bucket", "", 0, mock, &mockTransferManager{})
	tmpDir := t.TempDir()
	keyPrefix := "downloads/run-2"

	if err := b.DownloadDirectory(keyPrefix, tmpDir); err != nil {
		t.Fatalf("DownloadDirectory error: %v", err)
	}

	for _, o := range objects {
		rel := o.key[len(prefix):]
		localPath := filepath.Join(tmpDir, filepath.FromSlash(rel))
		got, err := os.ReadFile(localPath)
		if err != nil {
			t.Errorf("expected file %q to exist: %v", localPath, err)
			continue
		}
		if !bytes.Equal(got, o.content) {
			t.Errorf("file %q: want %q, got %q", localPath, o.content, got)
		}
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexStr(s, sub) >= 0)
}

func indexStr(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func keysOf(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
