// Package config provides YAML-based configuration loading for flowy binaries.
// All three binaries (flowy, flowy-server, flowy-daemon) use this package.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the top-level configuration for all flowy binaries.
type Config struct {
	// BaseDir is the root directory for run data (inputs/outputs/db).
	// Defaults to "./flowy-data".
	BaseDir string `yaml:"base_dir"`

	// DBDriver identifies the database backend. Currently only "sqlite" is supported.
	DBDriver string `yaml:"db_driver"`

	// DBDSN overrides the default SQLite DSN. When empty, DefaultDSN() is used.
	DBDSN string `yaml:"db_dsn"`

	Server ServerConfig `yaml:"server"`
	Daemon DaemonConfig `yaml:"daemon"`
	Client ClientConfig `yaml:"client"`
}

// ServerConfig holds configuration for the flowy-server binary.
type ServerConfig struct {
	// ListenAddr is the TCP address the server listens on. Defaults to ":8124".
	ListenAddr string `yaml:"listen_addr"`

	// HeartbeatTimeoutSec is the number of seconds before a daemon is considered
	// stale if no heartbeat is received. Defaults to 60.
	HeartbeatTimeoutSec int `yaml:"heartbeat_timeout_sec"`
}

// DaemonConfig holds configuration for the flowy-daemon binary.
type DaemonConfig struct {
	// ServerURL is the base URL for the flowy-server HTTP API.
	// Defaults to "http://localhost:8124".
	ServerURL string `yaml:"server_url"`

	// HeartbeatSec is the interval in seconds between heartbeat sends.
	// Defaults to 30.
	HeartbeatSec int `yaml:"heartbeat_sec"`

	// PollTimeoutSec is the long-poll timeout in seconds when waiting for tasks.
	// Defaults to 30.
	PollTimeoutSec int `yaml:"poll_timeout_sec"`

	// Workers is the number of concurrent task execution goroutines. Defaults to 1.
	Workers int `yaml:"workers"`
}

// ClientConfig holds configuration for the flowy CLI client binary.
type ClientConfig struct {
	// ServerURL is the base URL for the flowy-server HTTP API.
	// Defaults to "http://localhost:8124".
	ServerURL string `yaml:"server_url"`
}

// Load reads and parses the YAML config file at path, applies defaults for
// zero-value fields, and returns the resulting Config.
// Returns an error if the file cannot be read or the YAML is malformed.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse %q: %w", path, err)
	}

	applyDefaults(&cfg)
	return &cfg, nil
}

// applyDefaults sets zero-value fields in cfg to their default values.
// BaseDir is resolved to an absolute path so that Docker volume mounts
// and other path consumers always receive absolute paths.
func applyDefaults(cfg *Config) {
	if cfg.BaseDir == "" {
		cfg.BaseDir = "./flowy-data"
	}
	if abs, err := filepath.Abs(cfg.BaseDir); err == nil {
		cfg.BaseDir = abs
	}
	if cfg.DBDriver == "" {
		cfg.DBDriver = "sqlite"
	}
	if cfg.Server.ListenAddr == "" {
		cfg.Server.ListenAddr = ":8124"
	}
	if cfg.Server.HeartbeatTimeoutSec == 0 {
		cfg.Server.HeartbeatTimeoutSec = 60
	}
	if cfg.Daemon.ServerURL == "" {
		cfg.Daemon.ServerURL = "http://localhost:8124"
	}
	if cfg.Daemon.HeartbeatSec == 0 {
		cfg.Daemon.HeartbeatSec = 30
	}
	if cfg.Daemon.PollTimeoutSec == 0 {
		cfg.Daemon.PollTimeoutSec = 30
	}
	if cfg.Daemon.Workers == 0 {
		cfg.Daemon.Workers = 1
	}
	if cfg.Client.ServerURL == "" {
		cfg.Client.ServerURL = "http://localhost:8124"
	}
}

// Default returns a new Config with all fields set to their default values.
// Use this when no config file is present (e.g., the CLI client without a
// config file on a developer workstation).
func Default() *Config {
	cfg := &Config{}
	applyDefaults(cfg)
	return cfg
}

// DefaultDSN returns the database DSN for the given config.
// If DBDSN is already set in cfg, it is returned unchanged.
// Otherwise, a SQLite WAL-mode DSN pointing to {BaseDir}/flowy.db is constructed.
func DefaultDSN(cfg *Config) string {
	if cfg.DBDSN != "" {
		return cfg.DBDSN
	}
	dbPath := filepath.Join(cfg.BaseDir, "flowy.db")
	return fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)",
		dbPath,
	)
}
