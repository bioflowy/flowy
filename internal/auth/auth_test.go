package auth_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/bioflowy/flowy/internal/auth"
)

// --- Fake Store for testing ResolveJWTSecret ---

type fakeStore struct {
	data map[string]string
}

func newFakeStore() *fakeStore {
	return &fakeStore{data: make(map[string]string)}
}

func (f *fakeStore) GetServerConfig(_ context.Context, key string) (string, error) {
	v, ok := f.data[key]
	if !ok {
		return "", sql.ErrNoRows
	}
	return v, nil
}

func (f *fakeStore) SetServerConfig(_ context.Context, key, value string) error {
	f.data[key] = value
	return nil
}

// --- Tests ---

func TestIssueAndValidateToken(t *testing.T) {
	secret := []byte("test-secret")
	token, err := auth.IssueToken(secret, "user-id-1", "alice", "admin")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := auth.ValidateToken(secret, token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.UserID != "user-id-1" {
		t.Errorf("UserID: got %q, want %q", claims.UserID, "user-id-1")
	}
	if claims.Username != "alice" {
		t.Errorf("Username: got %q, want %q", claims.Username, "alice")
	}
	if claims.Role != "admin" {
		t.Errorf("Role: got %q, want %q", claims.Role, "admin")
	}
}

func TestValidateToken_Expired(t *testing.T) {
	secret := []byte("test-secret")
	// Issue a token that is already expired.
	token, err := auth.IssueTokenWithExpiry(secret, "user-id-1", "alice", "user", -1*time.Hour)
	if err != nil {
		t.Fatalf("IssueTokenWithExpiry: %v", err)
	}

	_, err = auth.ValidateToken(secret, token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	secretA := []byte("secret-a")
	secretB := []byte("secret-b")

	token, err := auth.IssueToken(secretA, "user-id-1", "alice", "user")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	_, err = auth.ValidateToken(secretB, token)
	if err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestValidateToken_Malformed(t *testing.T) {
	secret := []byte("test-secret")
	_, err := auth.ValidateToken(secret, "not.a.valid.jwt")
	if err == nil {
		t.Fatal("expected error for malformed token, got nil")
	}
}

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := auth.HashPassword("mysecretpassword", bcrypt.MinCost)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}

	// Correct password should succeed.
	if err := auth.CheckPassword(hash, "mysecretpassword"); err != nil {
		t.Errorf("CheckPassword correct: %v", err)
	}

	// Wrong password should fail.
	if err := auth.CheckPassword(hash, "wrongpassword"); err == nil {
		t.Error("expected error for wrong password, got nil")
	}
}

func TestResolveJWTSecret_FromConfig(t *testing.T) {
	st := newFakeStore()
	secret, err := auth.ResolveJWTSecret(context.Background(), "my-config-secret", st)
	if err != nil {
		t.Fatalf("ResolveJWTSecret: %v", err)
	}
	if string(secret) != "my-config-secret" {
		t.Errorf("expected %q, got %q", "my-config-secret", string(secret))
	}
	// Store should not have been touched.
	if _, dbErr := st.GetServerConfig(context.Background(), "jwt_secret"); !errors.Is(dbErr, sql.ErrNoRows) {
		t.Error("expected store to be untouched when config secret is set")
	}
}

func TestResolveJWTSecret_AutoGenerate(t *testing.T) {
	st := newFakeStore()
	secret, err := auth.ResolveJWTSecret(context.Background(), "", st)
	if err != nil {
		t.Fatalf("ResolveJWTSecret: %v", err)
	}
	if len(secret) == 0 {
		t.Fatal("expected non-empty secret")
	}

	// Should have been persisted.
	stored, dbErr := st.GetServerConfig(context.Background(), "jwt_secret")
	if dbErr != nil {
		t.Fatalf("GetServerConfig: %v", dbErr)
	}
	if stored == "" {
		t.Error("expected persisted secret to be non-empty")
	}

	// Second call should return the same secret.
	secret2, err := auth.ResolveJWTSecret(context.Background(), "", st)
	if err != nil {
		t.Fatalf("second ResolveJWTSecret: %v", err)
	}
	if string(secret) != string(secret2) {
		t.Errorf("expected same secret on second call: %q != %q", string(secret), string(secret2))
	}
}

func TestResolveJWTSecret_FromDB(t *testing.T) {
	st := newFakeStore()
	// Pre-seed the store with a known secret.
	_ = st.SetServerConfig(context.Background(), "jwt_secret", "db-stored-secret")

	secret, err := auth.ResolveJWTSecret(context.Background(), "", st)
	if err != nil {
		t.Fatalf("ResolveJWTSecret: %v", err)
	}
	if string(secret) != "db-stored-secret" {
		t.Errorf("expected %q, got %q", "db-stored-secret", string(secret))
	}
}
