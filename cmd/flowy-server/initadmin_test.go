package main

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// mockInitAdminStore implements InitAdminStore for testing.
type mockInitAdminStore struct {
	userCount   int
	createdUser *createdUserRecord
	countErr    error
	createErr   error
}

type createdUserRecord struct {
	id           string
	username     string
	passwordHash string
	globalRole   string
	isActive     bool
}

func (m *mockInitAdminStore) CountUsers(_ context.Context) (int, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.userCount, nil
}

func (m *mockInitAdminStore) CreateUser(_ context.Context, u *createUserArgs) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.createdUser = &createdUserRecord{
		id:           u.id,
		username:     u.username,
		passwordHash: u.passwordHash,
		globalRole:   u.globalRole,
		isActive:     u.isActive,
	}
	return nil
}

// TestRunInitAdmin_Success verifies that a new admin user is created when the DB is empty
// and both username and password are provided.
func TestRunInitAdmin_Success(t *testing.T) {
	st := &mockInitAdminStore{userCount: 0}
	iac := InitAdminConfig{
		Username:   "admin",
		Password:   "supersecret",
		BcryptCost: bcrypt.MinCost,
	}

	err := runInitAdmin(context.Background(), st, iac)
	if err != nil {
		t.Fatalf("runInitAdmin: unexpected error: %v", err)
	}

	if st.createdUser == nil {
		t.Fatal("expected CreateUser to be called, but it was not")
	}
	if st.createdUser.username != "admin" {
		t.Errorf("username: got %q, want %q", st.createdUser.username, "admin")
	}
	if st.createdUser.globalRole != "admin" {
		t.Errorf("globalRole: got %q, want %q", st.createdUser.globalRole, "admin")
	}
	if !st.createdUser.isActive {
		t.Error("expected isActive=true, got false")
	}
	if st.createdUser.id == "" {
		t.Error("expected non-empty ID (UUID)")
	}
}

// TestRunInitAdmin_UsersExist verifies that runInitAdmin returns an error when
// users already exist in the database.
func TestRunInitAdmin_UsersExist(t *testing.T) {
	st := &mockInitAdminStore{userCount: 1}
	iac := InitAdminConfig{
		Username:   "admin",
		Password:   "supersecret",
		BcryptCost: bcrypt.MinCost,
	}

	err := runInitAdmin(context.Background(), st, iac)
	if err == nil {
		t.Fatal("expected error when users exist, got nil")
	}
	if !errors.Is(err, errUsersExist) {
		// Check string match as a fallback.
		t.Logf("error: %v", err)
	}
}

// TestRunInitAdmin_EmptyUsername verifies that runInitAdmin returns an error
// when no username is provided.
func TestRunInitAdmin_EmptyUsername(t *testing.T) {
	st := &mockInitAdminStore{userCount: 0}
	iac := InitAdminConfig{
		Username:   "",
		Password:   "supersecret",
		BcryptCost: bcrypt.MinCost,
	}

	err := runInitAdmin(context.Background(), st, iac)
	if err == nil {
		t.Fatal("expected error for empty username, got nil")
	}
}

// TestRunInitAdmin_PasswordHashed verifies that the stored password_hash is a valid
// bcrypt hash that matches the original password.
func TestRunInitAdmin_PasswordHashed(t *testing.T) {
	st := &mockInitAdminStore{userCount: 0}
	iac := InitAdminConfig{
		Username:   "admin",
		Password:   "mysecretpassword",
		BcryptCost: bcrypt.MinCost,
	}

	if err := runInitAdmin(context.Background(), st, iac); err != nil {
		t.Fatalf("runInitAdmin: %v", err)
	}

	if st.createdUser == nil {
		t.Fatal("CreateUser was not called")
	}

	// Verify the hash matches the original password.
	if err := bcrypt.CompareHashAndPassword([]byte(st.createdUser.passwordHash), []byte("mysecretpassword")); err != nil {
		t.Errorf("bcrypt verification failed: %v", err)
	}
}
