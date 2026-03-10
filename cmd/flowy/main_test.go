package main

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// testdataDir returns the absolute path to the testdata directory for this test package.
// Go tests run with cwd set to the package directory, so testdata/ is directly accessible.
func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata")
}

// runArgs is a helper that calls run() with given args and captures stdout/stderr.
func runArgs(args []string) (code int, stdout, stderr string) {
	var outBuf, errBuf bytes.Buffer
	code = run(args, &outBuf, &errBuf)
	return code, outBuf.String(), errBuf.String()
}

// Test 1: --version flag.
func TestVersion(t *testing.T) {
	code, stdout, _ := runArgs([]string{"--version"})
	if code != 0 {
		t.Errorf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "flowy") {
		t.Errorf("expected stdout to contain 'flowy', got %q", stdout)
	}
	if !strings.Contains(stdout, "dev") {
		t.Errorf("expected stdout to contain 'dev', got %q", stdout)
	}
}

// Test 2: No arguments prints usage to stderr and exits 2.
func TestNoArgs(t *testing.T) {
	code, _, stderr := runArgs([]string{})
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("expected stderr to contain 'Usage:', got %q", stderr)
	}
}

// Test 3: Unknown subcommand exits 2 with "unknown subcommand".
func TestUnknownSubcommand(t *testing.T) {
	code, _, stderr := runArgs([]string{"unknown"})
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "unknown subcommand") {
		t.Errorf("expected 'unknown subcommand' in stderr, got %q", stderr)
	}
}

// Test 4: check with a valid WDL file exits 0 and stdout contains ": OK".
func TestCheckValid(t *testing.T) {
	validPath := filepath.Join(testdataDir(), "valid.wdl")
	code, stdout, stderr := runArgs([]string{"check", validPath})
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}
	if !strings.Contains(stdout, ": OK") {
		t.Errorf("expected stdout to contain ': OK', got %q", stdout)
	}
}

// Test 5: check with an invalid WDL file exits 1 and stderr contains rustc-style error.
func TestCheckInvalid(t *testing.T) {
	invalidPath := filepath.Join(testdataDir(), "invalid.wdl")
	code, _, stderr := runArgs([]string{"check", invalidPath})
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	// Rustc-style: file:line:col
	if !strings.Contains(stderr, "invalid.wdl:") {
		t.Errorf("expected rustc-style error with file:line:col in stderr, got %q", stderr)
	}
}

// Test 6: check with --path flag resolves imports from search directory.
func TestCheckWithPath(t *testing.T) {
	importPath := filepath.Join(testdataDir(), "with_import.wdl")
	libDir := filepath.Join(testdataDir(), "importlib")
	code, stdout, stderr := runArgs([]string{"check", "--path", libDir, importPath})
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}
	if !strings.Contains(stdout, ": OK") {
		t.Errorf("expected stdout to contain ': OK', got %q", stdout)
	}
}

// Test 7: check with nonexistent file exits 1 with error message.
func TestCheckNonexistent(t *testing.T) {
	code, _, stderr := runArgs([]string{"check", "nonexistent.wdl"})
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if stderr == "" {
		t.Error("expected non-empty stderr for missing file")
	}
}
