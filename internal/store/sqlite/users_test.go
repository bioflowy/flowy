package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/bioflowy/flowy/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// newTestUser creates a User with all required fields set for testing.
// It hashes the given password using bcrypt.MinCost for speed.
func newTestUser(t *testing.T, id, username, password string) *store.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword: %v", err)
	}
	return &store.User{
		ID:           id,
		Username:     username,
		PasswordHash: string(hash),
		DisplayName:  username + " Display",
		Email:        username + "@example.com",
		GlobalRole:   "user",
		IsActive:     true,
	}
}

// TestCreateUser_and_GetByUsername verifies that a created user can be retrieved
// by username and that all fields are round-tripped correctly.
func TestCreateUser_and_GetByUsername(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	u := newTestUser(t, "user-001", "alice", "s3cr3t")
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := s.GetUserByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("ID: got %q, want %q", got.ID, u.ID)
	}
	if got.Username != u.Username {
		t.Errorf("Username: got %q, want %q", got.Username, u.Username)
	}
	if got.PasswordHash != u.PasswordHash {
		t.Errorf("PasswordHash: got %q, want %q", got.PasswordHash, u.PasswordHash)
	}
	if got.DisplayName != u.DisplayName {
		t.Errorf("DisplayName: got %q, want %q", got.DisplayName, u.DisplayName)
	}
	if got.Email != u.Email {
		t.Errorf("Email: got %q, want %q", got.Email, u.Email)
	}
	if got.GlobalRole != u.GlobalRole {
		t.Errorf("GlobalRole: got %q, want %q", got.GlobalRole, u.GlobalRole)
	}
	if !got.IsActive {
		t.Error("IsActive: got false, want true")
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

// TestCreateUser_DuplicateUsername verifies that creating two users with the
// same username returns an error on the second attempt.
func TestCreateUser_DuplicateUsername(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	u1 := newTestUser(t, "user-dup-01", "bob", "pass1")
	if err := s.CreateUser(ctx, u1); err != nil {
		t.Fatalf("first CreateUser: %v", err)
	}

	u2 := newTestUser(t, "user-dup-02", "bob", "pass2")
	if err := s.CreateUser(ctx, u2); err == nil {
		t.Error("expected error for duplicate username, got nil")
	}
}

// TestGetUserByID verifies that a user can be retrieved by their UUID.
func TestGetUserByID(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	u := newTestUser(t, "user-byid-01", "carol", "pass")
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := s.GetUserByID(ctx, "user-byid-01")
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.Username != "carol" {
		t.Errorf("Username: got %q, want %q", got.Username, "carol")
	}
}

// TestListUsers verifies that ListUsers returns all created users in order.
func TestListUsers(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	u1 := newTestUser(t, "user-list-01", "dave", "pass1")
	u2 := newTestUser(t, "user-list-02", "eve", "pass2")
	for _, u := range []*store.User{u1, u2} {
		if err := s.CreateUser(ctx, u); err != nil {
			t.Fatalf("CreateUser %q: %v", u.Username, err)
		}
	}

	users, err := s.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

// TestDeactivateUser verifies that DeactivateUser sets is_active=false and
// that the deactivated user is no longer returned by GetUserByUsername.
func TestDeactivateUser(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	u := newTestUser(t, "user-deact-01", "frank", "pass")
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := s.DeactivateUser(ctx, u.ID); err != nil {
		t.Fatalf("DeactivateUser: %v", err)
	}

	// GetUserByUsername only returns active users, so this should fail.
	_, err := s.GetUserByUsername(ctx, "frank")
	if err == nil {
		t.Error("expected error for deactivated user via GetUserByUsername, got nil")
	}

	// GetUserByID still returns the record, but is_active should be false.
	got, err := s.GetUserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetUserByID after deactivation: %v", err)
	}
	if got.IsActive {
		t.Error("IsActive: got true after DeactivateUser, want false")
	}
}

// TestCountUsers verifies that CountUsers returns the correct total.
func TestCountUsers(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	count, err := s.CountUsers(ctx)
	if err != nil {
		t.Fatalf("CountUsers (empty): %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	for i, name := range []string{"grace", "heidi"} {
		u := newTestUser(t, "user-count-0"+string(rune('1'+i)), name, "pass")
		if err := s.CreateUser(ctx, u); err != nil {
			t.Fatalf("CreateUser %q: %v", name, err)
		}
	}

	count, err = s.CountUsers(ctx)
	if err != nil {
		t.Fatalf("CountUsers: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

// TestServerConfig verifies SetServerConfig and GetServerConfig round-trip,
// and that GetServerConfig returns an error for a missing key.
func TestServerConfig(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// Missing key should return sql.ErrNoRows.
	_, err := s.GetServerConfig(ctx, "jwt_secret")
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}

	// Set a key and retrieve it.
	if err := s.SetServerConfig(ctx, "jwt_secret", "supersecret"); err != nil {
		t.Fatalf("SetServerConfig: %v", err)
	}
	val, err := s.GetServerConfig(ctx, "jwt_secret")
	if err != nil {
		t.Fatalf("GetServerConfig: %v", err)
	}
	if val != "supersecret" {
		t.Errorf("GetServerConfig: got %q, want %q", val, "supersecret")
	}

	// Upsert: set same key with new value.
	if err := s.SetServerConfig(ctx, "jwt_secret", "newvalue"); err != nil {
		t.Fatalf("SetServerConfig (upsert): %v", err)
	}
	val, err = s.GetServerConfig(ctx, "jwt_secret")
	if err != nil {
		t.Fatalf("GetServerConfig (after upsert): %v", err)
	}
	if val != "newvalue" {
		t.Errorf("GetServerConfig (after upsert): got %q, want %q", val, "newvalue")
	}
}
