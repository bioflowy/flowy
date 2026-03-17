package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/bioflowy/flowy/internal/auth"
	"github.com/bioflowy/flowy/internal/store"
	"github.com/google/uuid"
	"golang.org/x/term"
)

// errUsersExist is returned by runInitAdmin when users are already present in the DB.
// It is a sentinel so callers can distinguish this condition from other errors.
var errUsersExist = errors.New("users already exist. Use the API to manage users")

// InitAdminConfig holds the parameters for the --init-admin operation.
type InitAdminConfig struct {
	Username   string
	Password   string
	BcryptCost int
}

// createUserArgs groups the fields passed to InitAdminStore.CreateUser.
// Using a struct rather than *store.User keeps the interface testable without
// importing the full store package in tests (though here we accept store.User
// since tests share the same package).
type createUserArgs struct {
	id           string
	username     string
	passwordHash string
	globalRole   string
	isActive     bool
}

// InitAdminStore is the minimal store interface needed by runInitAdmin.
// It is a subset of store.Store, defined here for testability.
type InitAdminStore interface {
	CountUsers(ctx context.Context) (int, error)
	CreateUser(ctx context.Context, u *createUserArgs) error
}

// storeAdapter wraps a full store.Store so it satisfies InitAdminStore.
type storeAdapter struct {
	st store.Store
}

func (a *storeAdapter) CountUsers(ctx context.Context) (int, error) {
	return a.st.CountUsers(ctx)
}

func (a *storeAdapter) CreateUser(ctx context.Context, u *createUserArgs) error {
	return a.st.CreateUser(ctx, &store.User{
		ID:           u.id,
		Username:     u.username,
		PasswordHash: u.passwordHash,
		GlobalRole:   u.globalRole,
		IsActive:     u.isActive,
	})
}

// runInitAdmin creates the initial admin user. It is called from main when
// --init-admin is passed. The HTTP server is NOT started in this path.
//
// Steps:
//  1. Validate that username is non-empty.
//  2. Check that no users exist yet (chicken-and-egg guard).
//  3. If password is empty, prompt interactively (hidden input).
//  4. Hash the password with bcrypt.
//  5. Create the user with global_role="admin" and is_active=true.
func runInitAdmin(ctx context.Context, st InitAdminStore, iac InitAdminConfig) error {
	if iac.Username == "" {
		return errors.New("username is required")
	}

	count, err := st.CountUsers(ctx)
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return errUsersExist
	}

	password := iac.Password
	if password == "" {
		pw, err := promptPassword("Enter admin password: ")
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}
		confirm, err := promptPassword("Confirm admin password: ")
		if err != nil {
			return fmt.Errorf("read password confirmation: %w", err)
		}
		if pw != confirm {
			return errors.New("passwords do not match")
		}
		password = pw
	}

	hash, err := auth.HashPassword(password, iac.BcryptCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	id := uuid.New().String()
	if err := st.CreateUser(ctx, &createUserArgs{
		id:           id,
		username:     iac.Username,
		passwordHash: hash,
		globalRole:   "admin",
		isActive:     true,
	}); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Admin user created: %s\n", iac.Username)
	return nil
}

// promptPassword prints prompt to stderr and reads a password without terminal echo.
// It uses golang.org/x/term so it works on Linux/macOS/Windows.
func promptPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr) // newline after hidden input
	if err != nil {
		return "", err
	}
	return string(pw), nil
}
