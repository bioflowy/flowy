package sqlite_test

import (
	"context"
	"strings"
	"testing"

	"github.com/bioflowy/flowy/internal/store"
)

// TestCreateRunWithRegistryRef verifies that CreateRun persists RegistryRef and
// RegistryDigest fields, and that GetRun retrieves them correctly.
func TestCreateRunWithRegistryRef(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	run := &store.Run{
		ID:             "registry-run-001",
		WDLPath:        "",
		RegistryRef:    "hello:prod",
		RegistryDigest: "sha256:abc123",
		InputsJSON:     `{}`,
		State:          store.RunPending,
	}
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	got, err := s.GetRun(ctx, "registry-run-001")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.RegistryRef != "hello:prod" {
		t.Errorf("RegistryRef: got %q, want %q", got.RegistryRef, "hello:prod")
	}
	if got.RegistryDigest != "sha256:abc123" {
		t.Errorf("RegistryDigest: got %q, want %q", got.RegistryDigest, "sha256:abc123")
	}
}

// TestCreateRunWithEmptyRegistryRef verifies that a file-path run (empty RegistryRef)
// round-trips through the store with empty strings for the registry fields.
func TestCreateRunWithEmptyRegistryRef(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	run := &store.Run{
		ID:             "file-run-001",
		WDLPath:        "/path/to/file.wdl",
		RegistryRef:    "",
		RegistryDigest: "",
		InputsJSON:     `{}`,
		State:          store.RunPending,
	}
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	got, err := s.GetRun(ctx, "file-run-001")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.RegistryRef != "" {
		t.Errorf("RegistryRef: got %q, want empty string", got.RegistryRef)
	}
	if got.RegistryDigest != "" {
		t.Errorf("RegistryDigest: got %q, want empty string", got.RegistryDigest)
	}
}

// TestGetRunByPrefixRegistryFields verifies that GetRunByPrefix returns
// RegistryRef and RegistryDigest correctly.
func TestGetRunByPrefixRegistryFields(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	run := &store.Run{
		ID:             "prefix-registry-run-aabbcc",
		WDLPath:        "",
		RegistryRef:    "align-reads:prod",
		RegistryDigest: "sha256:deadbeef",
		InputsJSON:     `{}`,
		State:          store.RunPending,
	}
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	got, err := s.GetRunByPrefix(ctx, "prefix-registry-run")
	if err != nil {
		t.Fatalf("GetRunByPrefix: %v", err)
	}
	if got.RegistryRef != "align-reads:prod" {
		t.Errorf("RegistryRef: got %q, want %q", got.RegistryRef, "align-reads:prod")
	}
	if !strings.HasPrefix(got.RegistryDigest, "sha256:") {
		t.Errorf("RegistryDigest: got %q, expected sha256: prefix", got.RegistryDigest)
	}
}
