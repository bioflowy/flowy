package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/bioflowy/flowy/internal/client"
)

// readPasswordFn is a package-level variable so tests can override it
// without needing a real terminal.
var readPasswordFn = func() ([]byte, error) {
	return term.ReadPassword(int(syscall.Stdin))
}

// runLogin implements the "login" subcommand.
// It prompts for username and password, authenticates with the flowy-server,
// and saves the returned JWT to ~/.flowy/credentials.yaml.
//
// Exit codes: 0 = success, 1 = execution error, 2 = usage error.
func runLogin(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(stderr)
	serverFlag := fs.String("server", "", "flowy-server URL (e.g. http://localhost:8124)")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *serverFlag == "" {
		fmt.Fprintln(stderr, "error: --server flag is required")
		fmt.Fprintln(stderr, "Usage: flowy login --server <url>")
		return 2
	}

	// Prompt for username (reads from stdin for testability).
	fmt.Fprint(stdout, "Username: ")
	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(stderr, "error: reading username: %v\n", err)
		} else {
			fmt.Fprintln(stderr, "error: unexpected EOF reading username")
		}
		return 1
	}
	username := strings.TrimSpace(scanner.Text())

	// Prompt for password (uses readPasswordFn so tests can override).
	fmt.Fprint(stdout, "Password: ")
	passwordBytes, err := readPasswordFn()
	fmt.Fprintln(stdout) // newline after password prompt
	if err != nil {
		fmt.Fprintf(stderr, "error: reading password: %v\n", err)
		return 1
	}
	password := string(passwordBytes)
	// Zero the byte slice so the plaintext password is not retained in memory
	// longer than necessary (e.g. visible in heap dumps or core files).
	for i := range passwordBytes {
		passwordBytes[i] = 0
	}

	// Authenticate with the server.
	c := client.New(*serverFlag)
	token, err := c.Login(context.Background(), username, password)
	if err != nil {
		errMsg := err.Error()
		if strings.HasPrefix(errMsg, "cannot connect") {
			fmt.Fprintf(stderr, "Cannot connect to server at %s: %v\n", *serverFlag, err)
		} else {
			fmt.Fprintf(stderr, "Login failed: %s\n", errMsg)
		}
		return 1
	}

	// Handle auth-disabled case: server returns empty token.
	if token == "" {
		fmt.Fprintln(stderr, "Warning: server has authentication disabled — no credentials saved.")
		return 0
	}

	// Save credentials.
	creds := &client.Credentials{
		Server: *serverFlag,
		Token:  token,
	}
	if err := client.SaveCredentials(creds); err != nil {
		fmt.Fprintf(stderr, "error: saving credentials: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "Login successful.")
	return 0
}
