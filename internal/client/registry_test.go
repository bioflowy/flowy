package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/client"
)

// TestRegistryPush_Success tests that RegistryPush sends POST /projects/{slug}/registry
// and returns RegistryPushResponse on 201.
func TestRegistryPush_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/myproj/registry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		// Verify Authorization header is set when using NewWithToken.
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		var req api.RegistryPushRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, api.RegistryPushResponse{
			Name:      req.Name,
			Digest:    "sha256:abc123",
			Kind:      "workflow",
			VersionID: "v1",
		})
	})

	srv := newTestServerRaw(t, mux)
	c := client.NewWithToken(srv.URL, "test-token")

	resp, err := c.RegistryPush(context.Background(), "myproj", "hello", "version 1.2\nworkflow test {}")
	if err != nil {
		t.Fatalf("RegistryPush: unexpected error: %v", err)
	}
	if resp.Name != "hello" {
		t.Errorf("RegistryPush: got Name %q, want %q", resp.Name, "hello")
	}
	if resp.Digest != "sha256:abc123" {
		t.Errorf("RegistryPush: got Digest %q, want %q", resp.Digest, "sha256:abc123")
	}
}

// TestRegistryPush_Unauthorized tests that RegistryPush returns ErrUnauthorized on 401.
func TestRegistryPush_Unauthorized(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/myproj/registry", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusUnauthorized, api.ErrorResponse{Error: "unauthorized"})
	})

	srv := newTestServerRaw(t, mux)
	c := client.NewWithToken(srv.URL, "bad-token")

	_, err := c.RegistryPush(context.Background(), "myproj", "hello", "content")
	if err == nil {
		t.Fatal("RegistryPush: expected error, got nil")
	}
	if err != client.ErrUnauthorized {
		t.Errorf("RegistryPush: got error %v, want ErrUnauthorized", err)
	}
}

// TestRegistryPush_Forbidden tests that RegistryPush returns ErrForbidden on 403.
func TestRegistryPush_Forbidden(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/myproj/registry", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusForbidden, api.ErrorResponse{Error: "forbidden"})
	})

	srv := newTestServerRaw(t, mux)
	c := client.NewWithToken(srv.URL, "no-perm-token")

	_, err := c.RegistryPush(context.Background(), "myproj", "hello", "content")
	if err == nil {
		t.Fatal("RegistryPush: expected error, got nil")
	}
	if err != client.ErrForbidden {
		t.Errorf("RegistryPush: got error %v, want ErrForbidden", err)
	}
}

// TestRegistryList_Success tests that RegistryList sends GET /projects/{slug}/registry
// and returns a list of entries on 200.
func TestRegistryList_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/myproj/registry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		entries := []api.RegistryEntryResponse{
			{Name: "hello", Kind: "workflow", VersionCount: 2, Tags: []string{"latest", "v1.0"}},
			{Name: "greet", Kind: "task", VersionCount: 1, Tags: []string{"stable"}},
		}
		writeJSON(w, http.StatusOK, entries)
	})

	srv := newTestServerRaw(t, mux)
	c := client.NewWithToken(srv.URL, "test-token")

	entries, err := c.RegistryList(context.Background(), "myproj")
	if err != nil {
		t.Fatalf("RegistryList: unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("RegistryList: got %d entries, want 2", len(entries))
	}
	if entries[0].Name != "hello" {
		t.Errorf("RegistryList: got entries[0].Name %q, want %q", entries[0].Name, "hello")
	}
}

// TestRegistryList_Unauthorized tests that RegistryList returns ErrUnauthorized on 401.
func TestRegistryList_Unauthorized(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/myproj/registry", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusUnauthorized, api.ErrorResponse{Error: "unauthorized"})
	})

	srv := newTestServerRaw(t, mux)
	c := client.NewWithToken(srv.URL, "bad-token")

	_, err := c.RegistryList(context.Background(), "myproj")
	if err == nil {
		t.Fatal("RegistryList: expected error, got nil")
	}
	if err != client.ErrUnauthorized {
		t.Errorf("RegistryList: got error %v, want ErrUnauthorized", err)
	}
}

// TestRegistryPull_ByTag tests that RegistryPull sends GET /projects/{slug}/registry/{name}?tag={tag}.
func TestRegistryPull_ByTag(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/myproj/registry/hello", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Query().Get("tag") != "latest" {
			http.Error(w, "expected tag=latest", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, api.RegistryPullResponse{
			Name:    "hello",
			Digest:  "sha256:abc123",
			Kind:    "workflow",
			Content: "version 1.2\nworkflow test {}",
		})
	})

	srv := newTestServerRaw(t, mux)
	c := client.NewWithToken(srv.URL, "test-token")

	resp, err := c.RegistryPull(context.Background(), "myproj", "hello", "latest", "")
	if err != nil {
		t.Fatalf("RegistryPull: unexpected error: %v", err)
	}
	if resp.Content != "version 1.2\nworkflow test {}" {
		t.Errorf("RegistryPull: got Content %q, want %q", resp.Content, "version 1.2\nworkflow test {}")
	}
}

// TestRegistryPull_ByDigest tests that RegistryPull uses ?digest= when digest is provided.
func TestRegistryPull_ByDigest(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/myproj/registry/hello", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("digest") != "sha256:abc123" {
			http.Error(w, "expected digest param", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, api.RegistryPullResponse{
			Name:    "hello",
			Digest:  "sha256:abc123",
			Kind:    "workflow",
			Content: "version 1.2\nworkflow test {}",
		})
	})

	srv := newTestServerRaw(t, mux)
	c := client.NewWithToken(srv.URL, "test-token")

	resp, err := c.RegistryPull(context.Background(), "myproj", "hello", "", "sha256:abc123")
	if err != nil {
		t.Fatalf("RegistryPull: unexpected error: %v", err)
	}
	if resp.Digest != "sha256:abc123" {
		t.Errorf("RegistryPull: got Digest %q, want %q", resp.Digest, "sha256:abc123")
	}
}

// TestRegistryPull_Unauthorized tests that RegistryPull returns ErrUnauthorized on 401.
func TestRegistryPull_Unauthorized(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/myproj/registry/hello", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusUnauthorized, api.ErrorResponse{Error: "unauthorized"})
	})

	srv := newTestServerRaw(t, mux)
	c := client.NewWithToken(srv.URL, "bad-token")

	_, err := c.RegistryPull(context.Background(), "myproj", "hello", "latest", "")
	if err == nil {
		t.Fatal("RegistryPull: expected error, got nil")
	}
	if err != client.ErrUnauthorized {
		t.Errorf("RegistryPull: got error %v, want ErrUnauthorized", err)
	}
}

// TestRegistryTag_Success tests that RegistryTag sends POST /projects/{slug}/registry/{name}/tags.
func TestRegistryTag_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/myproj/registry/hello/tags", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		var req api.RegistryTagRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		if req.Tag == "" || req.Digest == "" {
			http.Error(w, "missing tag or digest", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := newTestServerRaw(t, mux)
	c := client.NewWithToken(srv.URL, "test-token")

	err := c.RegistryTag(context.Background(), "myproj", "hello", "v1.0", "sha256:abc123")
	if err != nil {
		t.Fatalf("RegistryTag: unexpected error: %v", err)
	}
}

// TestRegistryTag_Unauthorized tests that RegistryTag returns ErrUnauthorized on 401.
func TestRegistryTag_Unauthorized(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/myproj/registry/hello/tags", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusUnauthorized, api.ErrorResponse{Error: "unauthorized"})
	})

	srv := newTestServerRaw(t, mux)
	c := client.NewWithToken(srv.URL, "bad-token")

	err := c.RegistryTag(context.Background(), "myproj", "hello", "v1.0", "sha256:abc123")
	if err == nil {
		t.Fatal("RegistryTag: expected error, got nil")
	}
	if err != client.ErrUnauthorized {
		t.Errorf("RegistryTag: got error %v, want ErrUnauthorized", err)
	}
}

// TestRegistryTag_Forbidden tests that RegistryTag returns ErrForbidden on 403.
func TestRegistryTag_Forbidden(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/myproj/registry/hello/tags", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusForbidden, api.ErrorResponse{Error: "forbidden"})
	})

	srv := newTestServerRaw(t, mux)
	c := client.NewWithToken(srv.URL, "no-perm-token")

	err := c.RegistryTag(context.Background(), "myproj", "hello", "v1.0", "sha256:abc123")
	if err == nil {
		t.Fatal("RegistryTag: expected error, got nil")
	}
	if err != client.ErrForbidden {
		t.Errorf("RegistryTag: got error %v, want ErrForbidden", err)
	}
}

// TestRegistryDeleteTag_Success tests that RegistryDeleteTag sends DELETE .../tags/{tag} and returns nil on 204.
func TestRegistryDeleteTag_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/myproj/registry/hello/tags/v1.0", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	srv := newTestServerRaw(t, mux)
	c := client.NewWithToken(srv.URL, "test-token")

	err := c.RegistryDeleteTag(context.Background(), "myproj", "hello", "v1.0")
	if err != nil {
		t.Fatalf("RegistryDeleteTag: unexpected error: %v", err)
	}
}

// TestRegistryDelete_Success tests that RegistryDelete sends DELETE .../registry/{name} and returns nil on 204.
func TestRegistryDelete_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/myproj/registry/hello", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	srv := newTestServerRaw(t, mux)
	c := client.NewWithToken(srv.URL, "test-token")

	err := c.RegistryDelete(context.Background(), "myproj", "hello")
	if err != nil {
		t.Fatalf("RegistryDelete: unexpected error: %v", err)
	}
}

// newTestServerRaw creates an httptest server with the given mux (does not create a client).
func newTestServerRaw(t *testing.T, mux *http.ServeMux) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}
