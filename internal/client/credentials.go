// Package client provides credential storage and loading for the flowy CLI.
// Credentials are stored in ~/.flowy/credentials.yaml with restricted permissions.
package client

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Credentials holds the authentication information for a flowy-server connection.
type Credentials struct {
	Server string `yaml:"server"`
	Token  string `yaml:"token"`
}

// CredentialsPath returns the path to the credentials file: ~/.flowy/credentials.yaml.
func CredentialsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("credentials: cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".flowy", "credentials.yaml"), nil
}

// LoadCredentials reads and returns the Credentials from ~/.flowy/credentials.yaml.
// Returns a descriptive error when the file does not exist, directing the user
// to run `flowy login`.
func LoadCredentials() (*Credentials, error) {
	path, err := CredentialsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("credentials: no credentials found — run `flowy login --server <url>` to authenticate")
		}
		return nil, fmt.Errorf("credentials: reading credentials file: %w", err)
	}

	var creds Credentials
	if err := yaml.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("credentials: parsing credentials file: %w", err)
	}
	if creds.Server == "" {
		return nil, fmt.Errorf("credentials: server URL is empty in credentials file — run `flowy login --server <url>`")
	}
	if creds.Token == "" {
		return nil, fmt.Errorf("credentials: token is empty in credentials file — run `flowy login --server <url>`")
	}
	return &creds, nil
}

// SaveCredentials writes the given Credentials to ~/.flowy/credentials.yaml.
// The ~/.flowy/ directory is created with mode 0700 if it does not exist.
// The credentials file is written with mode 0600 (owner read/write only).
func SaveCredentials(creds *Credentials) error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("credentials: creating config directory %q: %w", dir, err)
	}

	data, err := yaml.Marshal(creds)
	if err != nil {
		return fmt.Errorf("credentials: marshaling credentials: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("credentials: writing credentials file: %w", err)
	}
	// Unconditionally enforce 0600: os.WriteFile only applies the mode on
	// file creation, so an existing file with broader permissions would
	// otherwise keep its old mode after an overwrite.
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("credentials: setting credentials file permissions: %w", err)
	}
	return nil
}
