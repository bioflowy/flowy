// Command flowy is the CLI entry point for the flowy WDL executor.
// Usage: flowy <command> [options]
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/client"
	"github.com/bioflowy/flowy/internal/config"
	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/checker"
	"github.com/bioflowy/flowy/wdl/parser"
)

// version is set at build time via ldflags. Default is "dev".
var version = "dev"

// run is the testable main function. args is os.Args[1:].
// stdin is used by the login subcommand to read the username.
// Returns exit code: 0 = success, 1 = execution error, 2 = usage error.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}

	switch args[0] {
	case "--version":
		fmt.Fprintf(stdout, "flowy %s\n", version)
		return 0
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "run":
		return runRun(args[1:], stdout, stderr)
	case "submit":
		return runSubmit(args[1:], stdout, stderr)
	case "status":
		return runStatus(args[1:], stdout, stderr)
	case "outputs":
		return runOutputs(args[1:], stdout, stderr)
	case "login":
		return runLogin(args[1:], stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand: %q\n\n", args[0])
		usage(stderr)
		return 2
	}
}

// main is the real entry point.
func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// usage writes the help text to w.
func usage(w io.Writer) {
	fmt.Fprint(w, `Usage: flowy <command> [options]

Commands:
  login    Authenticate with a flowy-server
  check    Parse and type-check a WDL file
  run      Execute a WDL workflow or task
  submit   Submit a WDL workflow to a flowy-server
  status   Get the status of a submitted workflow run
  outputs  Get the outputs of a completed workflow run

Flags:
  --version  Print version and exit
`)
}

// runCheck implements the "check" subcommand.
// It parses and type-checks a WDL file, printing rustc-style diagnostics on error.
func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pathFlag := fs.String("path", "", "additional import search directory")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: check requires exactly one WDL file argument")
		fmt.Fprintln(stderr, "Usage: flowy check [--path <dir>] <file.wdl>")
		return 2
	}

	file := fs.Arg(0)

	// Parse the WDL file.
	var doc *ast.Document
	var parseErr error

	if *pathFlag != "" {
		doc, parseErr = parser.ParseWithSearchPaths(file, nil, []string{*pathFlag})
	} else {
		doc, parseErr = parser.Parse(file, nil)
	}

	if parseErr != nil {
		// ParseError already formats as rustc-style via Error().
		fmt.Fprintln(stderr, parseErr.Error())
		return 1
	}

	// Type-check the document.
	_, typeErrors := checker.Check(doc)

	if len(typeErrors) > 0 {
		for _, err := range typeErrors {
			if te, ok := err.(*checker.TypeError); ok {
				// Read source line for the error position.
				srcLine := readSourceLine(te.Pos.File, te.Pos.Line)
				printRustcError(stderr, te.Pos.File, te.Pos.Line, te.Pos.Col, te.Message, srcLine)
			} else {
				fmt.Fprintln(stderr, err.Error())
			}
		}
		return 1
	}

	// Success: print "filename: OK"
	fmt.Fprintf(stdout, "%s: OK\n", filepath.Base(file))
	return 0
}

// runRun implements the "run" subcommand using server-based execution.
// It resolves the WDL file path, loads inputs, submits to the server,
// polls until terminal state, then prints outputs on success.
//
// All referenced files (WDL and input data) must already reside under the
// shared base_dir or be resolvable relative to the WDL file's parent directory
// (per WDL 1.2 spec). See docs/adr/0001-remove-heuristic-file-staging-from-client.md.
//
// Exit codes: 0 = success, 1 = execution error, 2 = usage error.
func runRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)

	inputFlag := fs.String("i", "", "JSON input file path")
	// Also support --input as alias.
	inputLongFlag := fs.String("input", "", "JSON input file path (alias for -i)")
	taskFlag := fs.String("task", "", "run only the named task via POST /tasks/run")
	pathFlag := fs.String("path", "", "additional import search directory")
	verboseFlag := fs.Bool("verbose", false, "accepted for compatibility (no-op in server mode)")
	outputFlag := fs.String("output", "", "write outputs JSON to specified file path instead of stdout")
	serverFlag := fs.String("server", "", "server URL override")
	configFlag := fs.String("config", "", "config file path")
	quietFlag := fs.Bool("quiet", false, "suppress status lines on stderr during polling")
	storageFlag := fs.String("storage", "", "storage backend: local or s3")
	projectFlag := fs.String("project", "", "project slug for registry-ref runs")

	// Reorder args so flags after positional args are parsed correctly.
	// Go's flag package stops parsing at the first non-flag argument,
	// but spectool places --task after the WDL file path.
	args = reorderArgs(args)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: run requires exactly one WDL file argument")
		fmt.Fprintln(stderr, "Usage: flowy run [--server <url>] [--project <slug>] [--storage <type>] [-i <inputs>] [--task <name>] [--path <dir>] [--output <file>] [--quiet] <file.wdl|name:tag>")
		return 2
	}

	// --verbose is a no-op in server mode.
	if *verboseFlag {
		fmt.Fprintln(stderr, "note: --verbose not yet supported in server mode")
	}

	// Resolve effective input flag (--input or -i).
	effectiveInput := *inputFlag
	if effectiveInput == "" {
		effectiveInput = *inputLongFlag
	}

	wdlArg := fs.Arg(0)
	cfg := loadConfigOptional(*configFlag, stderr)

	// Validate storage backend selection.
	effectiveStorageType := resolveStorageType(*storageFlag, cfg)
	if err := validateStorageType(effectiveStorageType); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if effectiveStorageType == "s3" && cfg.Storage.Bucket == "" {
		fmt.Fprintln(stderr, "error: --storage s3 requires storage.bucket to be set in config")
		return 1
	}

	serverURL := resolveServerURL(*serverFlag, cfg)

	// Resolve project slug from flag or config default.
	projectSlug := *projectFlag
	if projectSlug == "" {
		projectSlug = cfg.Client.DefaultProject
	}

	// Set up signal-aware context so Ctrl+C interrupts immediately.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Generate a client-side run ID.
	runID := uuid.New().String()

	// Detect whether the argument is a registry reference or a file path.
	var absWDL string
	var registryRef string
	var err error
	if isRegistryRef(wdlArg) {
		registryRef = wdlArg
		if projectSlug == "" {
			fmt.Fprintln(stderr, "error: registry reference requires --project flag or default_project in config")
			return 1
		}
	} else {
		absWDL, err = filepath.Abs(wdlArg)
		if err != nil {
			fmt.Fprintf(stderr, "error: resolving WDL path %q: %v\n", wdlArg, err)
			return 1
		}
	}

	// Load input JSON if provided.
	jsonInputs := map[string]interface{}{}
	if effectiveInput != "" {
		data, err := os.ReadFile(effectiveInput)
		if err != nil {
			fmt.Fprintf(stderr, "error: reading inputs file %q: %v\n", effectiveInput, err)
			return 1
		}
		if err := json.Unmarshal(data, &jsonInputs); err != nil {
			fmt.Fprintf(stderr, "error: parsing inputs JSON %q: %v\n", effectiveInput, err)
			return 1
		}
	}

	// Collect search paths.
	var searchPaths []string
	if *pathFlag != "" {
		searchPaths = []string{*pathFlag}
	}

	c := client.New(serverURL)
	var submittedRunID string

	if *taskFlag != "" {
		// Task mode: submit via POST /tasks/run (or project-scoped /projects/{slug}/tasks/run).
		req := api.TaskRunRequest{
			WDLPath:     absWDL,
			RegistryRef: registryRef,
			TaskName:    *taskFlag,
			InputsJSON:  jsonInputs,
			RunID:       runID,
			SearchPaths: searchPaths,
		}
		var resp *api.TaskRunResponse
		if registryRef != "" {
			resp, err = c.TaskRunInProject(ctx, projectSlug, req)
		} else {
			resp, err = c.TaskRun(ctx, req)
		}
		if err != nil {
			fmt.Fprintf(stderr, "error: cannot reach server at %s: %v\n", serverURL, err)
			return 1
		}
		submittedRunID = resp.RunID
	} else {
		// Workflow mode: submit via POST /workflows (or project-scoped /projects/{slug}/workflows).
		req := api.SubmitRequest{
			RunID:       runID,
			WDLPath:     absWDL,
			RegistryRef: registryRef,
			InputsJSON:  jsonInputs,
			SearchPaths: searchPaths,
		}
		var resp *api.SubmitResponse
		if registryRef != "" {
			resp, err = c.SubmitToProject(ctx, projectSlug, req)
		} else {
			resp, err = c.Submit(ctx, req)
		}
		if err != nil {
			fmt.Fprintf(stderr, "error: cannot reach server at %s: %v\n", serverURL, err)
			return 1
		}
		submittedRunID = resp.RunID
	}

	// Poll until terminal state.
	finalState, err := pollUntilDone(ctx, c, submittedRunID, projectSlug, *quietFlag, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "error: polling run %s: %v\n", submittedRunID, err)
		return 1
	}

	if finalState != "COMPLETED" {
		fmt.Fprintf(stderr, "error: workflow %s (run: %s)\n", finalState, submittedRunID)
		return 1
	}

	// Fetch outputs.
	var outputsResp *api.OutputsResponse
	if projectSlug != "" {
		outputsResp, err = c.OutputsInProject(ctx, projectSlug, submittedRunID)
	} else {
		outputsResp, err = c.Outputs(ctx, submittedRunID)
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: fetching outputs for run %s: %v\n", submittedRunID, err)
		return 1
	}

	jsonBytes, err := json.MarshalIndent(outputsResp.Outputs, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "error: serializing outputs: %v\n", err)
		return 1
	}

	// Write outputs JSON to --output file or stdout.
	if *outputFlag != "" {
		if err := os.WriteFile(*outputFlag, jsonBytes, 0644); err != nil {
			fmt.Fprintf(stderr, "error: writing output file %q: %v\n", *outputFlag, err)
			return 1
		}
	} else {
		fmt.Fprintf(stdout, "%s\n", jsonBytes)
	}
	return 0
}

// pollUntilDone polls the server for run status until a terminal state is reached.
// It prints state changes to stderr (unless quiet is true).
// Returns the terminal state string or an error on context cancellation.
func pollUntilDone(ctx context.Context, c *client.Client, runID string, projectSlug string, quiet bool, stderr io.Writer) (string, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	shortID := runID
	if len(runID) >= 8 {
		shortID = runID[:8]
	}

	lastState := ""
	poll := func() (string, bool) {
		var resp *api.StatusResponse
		var err error
		if projectSlug != "" {
			resp, err = c.StatusInProject(ctx, projectSlug, runID)
		} else {
			resp, err = c.Status(ctx, runID)
		}
		if err != nil {
			// Transient network issue — warn and continue.
			fmt.Fprintf(stderr, "warning: polling run %s: %v\n", shortID, err)
			return "", false
		}
		state := resp.State
		if state != lastState {
			lastState = state
			if !quiet {
				fmt.Fprintf(stderr, "run %s: %s\n", shortID, state)
			}
		}
		switch state {
		case "COMPLETED", "FAILED", "ABORTED":
			return state, true
		}
		return "", false
	}

	// Immediate first poll to avoid waiting for the ticker.
	if state, done := poll(); done {
		return state, nil
	}

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			if state, done := poll(); done {
				return state, nil
			}
		}
	}
}

// printRustcError writes a rustc-style error diagnostic to w.
func printRustcError(w io.Writer, file string, line, col int, message, sourceLine string) {
	caret := strings.Repeat(" ", col-1) + "^"
	lineStr := fmt.Sprintf("%d", line)
	padding := strings.Repeat(" ", len(lineStr))
	fmt.Fprintf(w, "error: %s\n  --> %s:%d:%d\n%s |\n%s | %s\n%s |  %s\n",
		message,
		file, line, col,
		padding,
		lineStr, sourceLine,
		padding, caret,
	)
}

// readSourceLine reads a specific line (1-indexed) from a file.
// Returns empty string on any error.
func readSourceLine(file string, line int) string {
	f, err := os.Open(file)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	current := 0
	for scanner.Scan() {
		current++
		if current == line {
			return scanner.Text()
		}
	}
	return ""
}

// loadConfigOptional loads config from the given path. If path is empty, it tries
// "./flowy.yaml". If neither exists, it falls back to config.Default().
func loadConfigOptional(path string, stderr io.Writer) *config.Config {
	if path != "" {
		cfg, err := config.Load(path)
		if err != nil {
			fmt.Fprintf(stderr, "warning: loading config %q: %v — using defaults\n", path, err)
			return config.Default()
		}
		return cfg
	}
	// Try the default config path.
	cfg, err := config.Load("./flowy.yaml")
	if err == nil {
		return cfg
	}
	return config.Default()
}

// resolveServerURL returns the --server flag value if non-empty, otherwise
// falls back to cfg.Client.ServerURL.
func resolveServerURL(flagValue string, cfg *config.Config) string {
	if flagValue != "" {
		return flagValue
	}
	return cfg.Client.ServerURL
}

// resolveStorageType returns the --storage flag value if non-empty, otherwise
// falls back to cfg.Storage.Type.
func resolveStorageType(flagValue string, cfg *config.Config) string {
	if flagValue != "" {
		return flagValue
	}
	return cfg.Storage.Type
}

// validateStorageType returns an error if t is not a recognised storage backend name.
func validateStorageType(t string) error {
	switch t {
	case "local", "s3":
		return nil
	default:
		return fmt.Errorf("unknown storage backend %q (valid values: local, s3)", t)
	}
}

// runSubmit implements the "submit" subcommand.
// It resolves the WDL file path and submits the workflow to the flowy-server.
// All referenced files must already reside under the shared base_dir or be
// resolvable relative to the WDL file's parent directory.
func runSubmit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("submit", flag.ContinueOnError)
	fs.SetOutput(stderr)

	inputFlag := fs.String("i", "", "JSON inputs file path")
	serverFlag := fs.String("server", "", "server URL override")
	configFlag := fs.String("config", "", "config file path")
	pathFlag := fs.String("path", "", "additional import search directory")
	storageFlag := fs.String("storage", "", "storage backend: local or s3")
	projectFlag := fs.String("project", "", "project slug for registry-ref runs")

	args = reorderArgs(args)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: submit requires exactly one WDL file argument")
		fmt.Fprintln(stderr, "Usage: flowy submit [--server <url>] [--project <slug>] [--storage <type>] [-i <inputs>] [--path <dir>] <file.wdl|name:tag>")
		return 2
	}

	wdlArg := fs.Arg(0)
	cfg := loadConfigOptional(*configFlag, stderr)

	// Validate storage backend selection.
	effectiveStorageType := resolveStorageType(*storageFlag, cfg)
	if err := validateStorageType(effectiveStorageType); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if effectiveStorageType == "s3" && cfg.Storage.Bucket == "" {
		fmt.Fprintln(stderr, "error: --storage s3 requires storage.bucket to be set in config")
		return 1
	}

	serverURL := resolveServerURL(*serverFlag, cfg)

	// Resolve project slug from flag or config default.
	projectSlug := *projectFlag
	if projectSlug == "" {
		projectSlug = cfg.Client.DefaultProject
	}

	// Generate a client-side run ID.
	runID := uuid.New().String()

	// Detect whether the argument is a registry reference or a file path.
	var absWDL string
	var registryRef string
	if isRegistryRef(wdlArg) {
		registryRef = wdlArg
		if projectSlug == "" {
			fmt.Fprintln(stderr, "error: registry reference requires --project flag or default_project in config")
			return 1
		}
	} else {
		var err error
		absWDL, err = filepath.Abs(wdlArg)
		if err != nil {
			fmt.Fprintf(stderr, "error: resolving WDL path %q: %v\n", wdlArg, err)
			return 1
		}
	}

	// Load input JSON if provided.
	jsonInputs := map[string]interface{}{}
	if *inputFlag != "" {
		data, err := os.ReadFile(*inputFlag)
		if err != nil {
			fmt.Fprintf(stderr, "error: reading inputs file %q: %v\n", *inputFlag, err)
			return 1
		}
		if err := json.Unmarshal(data, &jsonInputs); err != nil {
			fmt.Fprintf(stderr, "error: parsing inputs JSON %q: %v\n", *inputFlag, err)
			return 1
		}
	}

	// Collect search paths.
	var searchPaths []string
	if *pathFlag != "" {
		searchPaths = []string{*pathFlag}
	}

	// Submit to server (project-scoped if registry ref, otherwise direct).
	req := api.SubmitRequest{
		RunID:       runID,
		WDLPath:     absWDL,
		RegistryRef: registryRef,
		InputsJSON:  jsonInputs,
		SearchPaths: searchPaths,
	}

	c := client.New(serverURL)
	var resp *api.SubmitResponse
	var err error
	if registryRef != "" {
		resp, err = c.SubmitToProject(context.Background(), projectSlug, req)
	} else {
		resp, err = c.Submit(context.Background(), req)
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: submitting workflow: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, resp.RunID)
	return 0
}

// runStatus implements the "status" subcommand.
// It queries the server for the current state of a workflow run.
func runStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)

	serverFlag := fs.String("server", "", "server URL override")
	configFlag := fs.String("config", "", "config file path")

	args = reorderArgs(args)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: status requires exactly one run ID argument")
		fmt.Fprintln(stderr, "Usage: flowy status [--server <url>] <run-id>")
		return 2
	}

	runID := fs.Arg(0)
	cfg := loadConfigOptional(*configFlag, stderr)
	serverURL := resolveServerURL(*serverFlag, cfg)

	c := client.New(serverURL)
	resp, err := c.Status(context.Background(), runID)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, resp.State)
	return 0
}

// runOutputs implements the "outputs" subcommand.
// It retrieves the JSON outputs of a completed workflow run.
func runOutputs(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("outputs", flag.ContinueOnError)
	fs.SetOutput(stderr)

	serverFlag := fs.String("server", "", "server URL override")
	configFlag := fs.String("config", "", "config file path")
	outputFlag := fs.String("output", "", "write JSON to file instead of stdout")

	args = reorderArgs(args)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: outputs requires exactly one run ID argument")
		fmt.Fprintln(stderr, "Usage: flowy outputs [--server <url>] [--output <file>] <run-id>")
		return 2
	}

	runID := fs.Arg(0)
	cfg := loadConfigOptional(*configFlag, stderr)
	serverURL := resolveServerURL(*serverFlag, cfg)

	c := client.New(serverURL)
	resp, err := c.Outputs(context.Background(), runID)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	jsonBytes, err := json.MarshalIndent(resp.Outputs, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "error: serializing outputs: %v\n", err)
		return 1
	}

	if *outputFlag != "" {
		if err := os.WriteFile(*outputFlag, jsonBytes, 0644); err != nil {
			fmt.Fprintf(stderr, "error: writing output file %q: %v\n", *outputFlag, err)
			return 1
		}
	} else {
		fmt.Fprintf(stdout, "%s\n", jsonBytes)
	}
	return 0
}

// isRegistryRef reports whether arg looks like a registry reference.
// A registry ref contains "@sha256:" or ":" where the colon is not a
// Windows drive letter (single letter followed by colon).
func isRegistryRef(arg string) bool {
	if strings.Contains(arg, "@sha256:") {
		return true
	}
	idx := strings.Index(arg, ":")
	if idx < 0 {
		return false
	}
	// Drive letters are single alphabetical characters: "C:", "D:", etc.
	if idx == 1 && len(arg) > 1 && isAlpha(arg[0]) {
		return false
	}
	return true
}

// isAlpha reports whether b is an ASCII letter (a-z or A-Z).
func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// reorderArgs moves flags that appear after positional arguments to the front.
// Go's flag package stops parsing at the first non-flag argument, so
// "file.wdl --task name" would silently ignore --task. This function
// reorders to "--task name file.wdl".
func reorderArgs(args []string) []string {
	var flags, positional []string
	knownValueFlags := map[string]bool{
		"--input": true, "--task": true, "--path": true, "--output": true,
		"-input": true, "-task": true, "-path": true, "-output": true,
		// Server-facing subcommand flags.
		"--server": true, "--config": true,
		"-server": true, "-config": true,
		"-i": true, "--i": true,
		// Storage backend selection.
		"--storage": true, "-storage": true,
		// Project selection for registry-ref runs.
		"--project": true, "-project": true,
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			if knownValueFlags[a] && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		} else {
			positional = append(positional, a)
		}
	}
	return append(flags, positional...)
}
