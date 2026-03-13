package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bioflowy/flowy/internal/config"
)

func writeYAML(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "flowy.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
base_dir: /custom/path
db_driver: sqlite
db_dsn: file:/custom/path/flowy.db
server:
  listen_addr: ":9090"
  heartbeat_timeout_sec: 120
daemon:
  server_url: "http://server:9090"
  heartbeat_sec: 45
  poll_timeout_sec: 60
client:
  server_url: "http://server:9090"
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.BaseDir != "/custom/path" {
		t.Errorf("BaseDir = %q, want %q", cfg.BaseDir, "/custom/path")
	}
	if cfg.DBDriver != "sqlite" {
		t.Errorf("DBDriver = %q, want %q", cfg.DBDriver, "sqlite")
	}
	if cfg.DBDSN != "file:/custom/path/flowy.db" {
		t.Errorf("DBDSN = %q, want %q", cfg.DBDSN, "file:/custom/path/flowy.db")
	}
	if cfg.Server.ListenAddr != ":9090" {
		t.Errorf("Server.ListenAddr = %q, want %q", cfg.Server.ListenAddr, ":9090")
	}
	if cfg.Server.HeartbeatTimeoutSec != 120 {
		t.Errorf("Server.HeartbeatTimeoutSec = %d, want %d", cfg.Server.HeartbeatTimeoutSec, 120)
	}
	if cfg.Daemon.ServerURL != "http://server:9090" {
		t.Errorf("Daemon.ServerURL = %q, want %q", cfg.Daemon.ServerURL, "http://server:9090")
	}
	if cfg.Daemon.HeartbeatSec != 45 {
		t.Errorf("Daemon.HeartbeatSec = %d, want %d", cfg.Daemon.HeartbeatSec, 45)
	}
	if cfg.Daemon.PollTimeoutSec != 60 {
		t.Errorf("Daemon.PollTimeoutSec = %d, want %d", cfg.Daemon.PollTimeoutSec, 60)
	}
	if cfg.Client.ServerURL != "http://server:9090" {
		t.Errorf("Client.ServerURL = %q, want %q", cfg.Client.ServerURL, "http://server:9090")
	}
}

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `{}`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	wantBase, _ := filepath.Abs("./flowy-data")
	if cfg.BaseDir != wantBase {
		t.Errorf("BaseDir = %q, want %q", cfg.BaseDir, wantBase)
	}
	if cfg.DBDriver != "sqlite" {
		t.Errorf("DBDriver = %q, want %q", cfg.DBDriver, "sqlite")
	}
	if cfg.Server.ListenAddr != ":8124" {
		t.Errorf("Server.ListenAddr = %q, want %q", cfg.Server.ListenAddr, ":8124")
	}
	if cfg.Server.HeartbeatTimeoutSec != 60 {
		t.Errorf("Server.HeartbeatTimeoutSec = %d, want %d", cfg.Server.HeartbeatTimeoutSec, 60)
	}
	if cfg.Daemon.ServerURL != "http://localhost:8124" {
		t.Errorf("Daemon.ServerURL = %q, want %q", cfg.Daemon.ServerURL, "http://localhost:8124")
	}
	if cfg.Daemon.HeartbeatSec != 30 {
		t.Errorf("Daemon.HeartbeatSec = %d, want %d", cfg.Daemon.HeartbeatSec, 30)
	}
	if cfg.Daemon.PollTimeoutSec != 30 {
		t.Errorf("Daemon.PollTimeoutSec = %d, want %d", cfg.Daemon.PollTimeoutSec, 30)
	}
	if cfg.Client.ServerURL != "http://localhost:8124" {
		t.Errorf("Client.ServerURL = %q, want %q", cfg.Client.ServerURL, "http://localhost:8124")
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := config.Load("/nonexistent/path/flowy.yaml")
	if err == nil {
		t.Error("Load() expected error for missing file, got nil")
	}
}

func TestLoadMalformed(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `{invalid yaml: [unclosed`)

	_, err := config.Load(path)
	if err == nil {
		t.Error("Load() expected error for malformed YAML, got nil")
	}
}

func TestLoadPartial(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
server:
  listen_addr: ":7070"
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	// Explicitly set section should be preserved
	if cfg.Server.ListenAddr != ":7070" {
		t.Errorf("Server.ListenAddr = %q, want %q", cfg.Server.ListenAddr, ":7070")
	}
	// Unset sections should get defaults
	wantBase, _ := filepath.Abs("./flowy-data")
	if cfg.BaseDir != wantBase {
		t.Errorf("BaseDir = %q, want %q", cfg.BaseDir, wantBase)
	}
	if cfg.Daemon.ServerURL != "http://localhost:8124" {
		t.Errorf("Daemon.ServerURL = %q, want %q", cfg.Daemon.ServerURL, "http://localhost:8124")
	}
	if cfg.Client.ServerURL != "http://localhost:8124" {
		t.Errorf("Client.ServerURL = %q, want %q", cfg.Client.ServerURL, "http://localhost:8124")
	}
	// Unset server field should get default
	if cfg.Server.HeartbeatTimeoutSec != 60 {
		t.Errorf("Server.HeartbeatTimeoutSec = %d, want %d", cfg.Server.HeartbeatTimeoutSec, 60)
	}
}

func TestDBDSN(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
base_dir: /mydata
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	want := "file:/mydata/flowy.db?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	got := config.DefaultDSN(cfg)
	if got != want {
		t.Errorf("DefaultDSN() = %q, want %q", got, want)
	}
}

func TestDBDSNOverride(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
db_dsn: "file:/custom.db"
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	// When db_dsn is explicitly set, DefaultDSN should return it as-is
	got := config.DefaultDSN(cfg)
	if got != "file:/custom.db" {
		t.Errorf("DefaultDSN() = %q, want %q", got, "file:/custom.db")
	}
}

func TestLoadStorageConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
storage:
  type: s3
  bucket: my-workflow-bucket
  endpoint: http://localhost:9000
  region: us-east-1
  prefix: flowy-runs/
  multipart_threshold: 52428800
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.Storage.Type != "s3" {
		t.Errorf("Storage.Type = %q, want %q", cfg.Storage.Type, "s3")
	}
	if cfg.Storage.Bucket != "my-workflow-bucket" {
		t.Errorf("Storage.Bucket = %q, want %q", cfg.Storage.Bucket, "my-workflow-bucket")
	}
	if cfg.Storage.Endpoint != "http://localhost:9000" {
		t.Errorf("Storage.Endpoint = %q, want %q", cfg.Storage.Endpoint, "http://localhost:9000")
	}
	if cfg.Storage.Region != "us-east-1" {
		t.Errorf("Storage.Region = %q, want %q", cfg.Storage.Region, "us-east-1")
	}
	if cfg.Storage.Prefix != "flowy-runs/" {
		t.Errorf("Storage.Prefix = %q, want %q", cfg.Storage.Prefix, "flowy-runs/")
	}
	if cfg.Storage.MultipartThreshold != 52428800 {
		t.Errorf("Storage.MultipartThreshold = %d, want %d", cfg.Storage.MultipartThreshold, 52428800)
	}
}

func TestLoadStorageDefaults(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `{}`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.Storage.Type != "local" {
		t.Errorf("Storage.Type = %q, want %q", cfg.Storage.Type, "local")
	}
	const wantThreshold = int64(104857600)
	if cfg.Storage.MultipartThreshold != wantThreshold {
		t.Errorf("Storage.MultipartThreshold = %d, want %d", cfg.Storage.MultipartThreshold, wantThreshold)
	}
}

func TestValidateLocalOK(t *testing.T) {
	cfg := &config.Config{}
	cfg.Storage.Type = "local"
	if err := config.Validate(cfg); err != nil {
		t.Errorf("Validate() unexpected error for type=local: %v", err)
	}
}

func TestValidateS3OK(t *testing.T) {
	cfg := &config.Config{}
	cfg.Storage.Type = "s3"
	cfg.Storage.Bucket = "b"
	if err := config.Validate(cfg); err != nil {
		t.Errorf("Validate() unexpected error for type=s3 with bucket: %v", err)
	}
}

func TestValidateS3MissingBucket(t *testing.T) {
	cfg := &config.Config{}
	cfg.Storage.Type = "s3"
	cfg.Storage.Bucket = ""
	err := config.Validate(cfg)
	if err == nil {
		t.Error("Validate() expected error for type=s3 with empty bucket, got nil")
	}
	if err != nil && !containsAll(err.Error(), "storage.bucket") {
		t.Errorf("Validate() error = %q, want it to mention %q", err.Error(), "storage.bucket")
	}
}

func TestValidateUnknownType(t *testing.T) {
	cfg := &config.Config{}
	cfg.Storage.Type = "gcs"
	err := config.Validate(cfg)
	if err == nil {
		t.Error("Validate() expected error for unknown storage type, got nil")
	}
	if err != nil && !containsAll(err.Error(), "not supported", "local, s3") {
		t.Errorf("Validate() error = %q, want it to mention %q and %q", err.Error(), "not supported", "local, s3")
	}
}

// containsAll checks that s contains all of the provided substrings.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
