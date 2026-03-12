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
	"github.com/bioflowy/flowy/internal/store"
	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/checker"
	"github.com/bioflowy/flowy/wdl/parser"
)

// version is set at build time via ldflags. Default is "dev".
var version = "dev"

// run is the testable main function. args is os.Args[1:].
// Returns exit code: 0 = success, 1 = execution error, 2 = usage error.
func run(args []string, stdout, stderr io.Writer) int {
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
	default:
		fmt.Fprintf(stderr, "unknown subcommand: %q\n\n", args[0])
		usage(stderr)
		return 2
	}
}

// main is the real entry point.
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// usage writes the help text to w.
func usage(w io.Writer) {
	fmt.Fprint(w, `Usage: flowy <command> [options]

Commands:
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
// It stages the WDL file and inputs to the shared filesystem, submits to the server,
// polls until terminal state, then prints outputs on success.
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
	baseDirFlag := fs.String("base-dir", "", "base directory for staging (overrides config)")
	quietFlag := fs.Bool("quiet", false, "suppress status lines on stderr during polling")

	// Reorder args so flags after positional args are parsed correctly.
	// Go's flag package stops parsing at the first non-flag argument,
	// but spectool places --task after the WDL file path.
	args = reorderArgs(args)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: run requires exactly one WDL file argument")
		fmt.Fprintln(stderr, "Usage: flowy run [--server <url>] [-i <inputs>] [--task <name>] [--path <dir>] [--output <file>] [--quiet] <file.wdl>")
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
	if *baseDirFlag != "" {
		cfg.BaseDir = *baseDirFlag
	}
	serverURL := resolveServerURL(*serverFlag, cfg)

	// Set up signal-aware context so Ctrl+C interrupts immediately.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Generate a client-side run ID for pre-submission staging.
	runID := uuid.New().String()

	// Create staging directories.
	if err := store.CreateRunDirs(cfg.BaseDir, runID); err != nil {
		fmt.Fprintf(stderr, "error: creating staging dirs: %v\n", err)
		return 1
	}

	inputsDir := store.RunInputsDir(cfg.BaseDir, runID)
	cleanupOnError := func() {
		os.RemoveAll(filepath.Join(cfg.BaseDir, "runs", runID)) //nolint:errcheck
	}

	// Stage the WDL file if not already under baseDir.
	absWDL, err := filepath.Abs(wdlArg)
	if err != nil {
		fmt.Fprintf(stderr, "error: resolving WDL path %q: %v\n", wdlArg, err)
		cleanupOnError()
		return 1
	}
	absBase, err := filepath.Abs(cfg.BaseDir)
	if err != nil {
		fmt.Fprintf(stderr, "error: resolving base dir %q: %v\n", cfg.BaseDir, err)
		cleanupOnError()
		return 1
	}
	stagedWDLPath := absWDL
	if !strings.HasPrefix(absWDL, absBase+string(os.PathSeparator)) && absWDL != absBase {
		stagedWDLPath, err = stageFile(absWDL, inputsDir)
		if err != nil {
			fmt.Fprintf(stderr, "error: staging WDL file: %v\n", err)
			cleanupOnError()
			return 1
		}
	}

	// Load and stage input files.
	jsonInputs := map[string]interface{}{}
	if effectiveInput != "" {
		data, err := os.ReadFile(effectiveInput)
		if err != nil {
			fmt.Fprintf(stderr, "error: reading inputs file %q: %v\n", effectiveInput, err)
			cleanupOnError()
			return 1
		}
		if err := json.Unmarshal(data, &jsonInputs); err != nil {
			fmt.Fprintf(stderr, "error: parsing inputs JSON %q: %v\n", effectiveInput, err)
			cleanupOnError()
			return 1
		}
		jsonInputs, err = stageInputFiles(jsonInputs, inputsDir, absBase)
		if err != nil {
			fmt.Fprintf(stderr, "error: staging input files: %v\n", err)
			cleanupOnError()
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
		// Task mode: submit via POST /tasks/run.
		req := api.TaskRunRequest{
			WDLPath:     stagedWDLPath,
			TaskName:    *taskFlag,
			InputsJSON:  jsonInputs,
			RunID:       runID,
			SearchPaths: searchPaths,
		}
		resp, err := c.TaskRun(ctx, req)
		if err != nil {
			fmt.Fprintf(stderr, "error: cannot reach server at %s: %v\n", serverURL, err)
			cleanupOnError()
			return 1
		}
		submittedRunID = resp.RunID
	} else {
		// Workflow mode: submit via POST /workflows.
		req := api.SubmitRequest{
			RunID:       runID,
			WDLPath:     stagedWDLPath,
			InputsJSON:  jsonInputs,
			SearchPaths: searchPaths,
		}
		resp, err := c.Submit(ctx, req)
		if err != nil {
			fmt.Fprintf(stderr, "error: cannot reach server at %s: %v\n", serverURL, err)
			cleanupOnError()
			return 1
		}
		submittedRunID = resp.RunID
	}

	// Poll until terminal state.
	finalState, err := pollUntilDone(ctx, c, submittedRunID, *quietFlag, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "error: polling run %s: %v\n", submittedRunID, err)
		return 1
	}

	if finalState != "COMPLETED" {
		fmt.Fprintf(stderr, "error: workflow %s (run: %s)\n", finalState, submittedRunID)
		return 1
	}

	// Fetch outputs.
	outputsResp, err := c.Outputs(ctx, submittedRunID)
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
func pollUntilDone(ctx context.Context, c *client.Client, runID string, quiet bool, stderr io.Writer) (string, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	shortID := runID
	if len(runID) >= 8 {
		shortID = runID[:8]
	}

	lastState := ""
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			resp, err := c.Status(ctx, runID)
			if err != nil {
				// Transient network issue — warn and continue.
				fmt.Fprintf(stderr, "warning: polling run %s: %v\n", shortID, err)
				continue
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

// runSubmit implements the "submit" subcommand.
// It stages local input files and the WDL file to the shared filesystem,
// then submits the workflow to the flowy-server.
func runSubmit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("submit", flag.ContinueOnError)
	fs.SetOutput(stderr)

	inputFlag := fs.String("i", "", "JSON inputs file path")
	serverFlag := fs.String("server", "", "server URL override")
	configFlag := fs.String("config", "", "config file path")
	baseDirFlag := fs.String("base-dir", "", "base directory for staging (overrides config)")
	pathFlag := fs.String("path", "", "additional import search directory")

	args = reorderArgs(args)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: submit requires exactly one WDL file argument")
		fmt.Fprintln(stderr, "Usage: flowy submit [--server <url>] [-i <inputs>] [--path <dir>] [--base-dir <dir>] <file.wdl>")
		return 2
	}

	wdlArg := fs.Arg(0)
	cfg := loadConfigOptional(*configFlag, stderr)
	if *baseDirFlag != "" {
		cfg.BaseDir = *baseDirFlag
	}
	serverURL := resolveServerURL(*serverFlag, cfg)

	// Generate a client-side run ID for pre-submission staging.
	runID := uuid.New().String()

	// Create staging directories.
	if err := store.CreateRunDirs(cfg.BaseDir, runID); err != nil {
		fmt.Fprintf(stderr, "error: creating staging dirs: %v\n", err)
		return 1
	}

	inputsDir := store.RunInputsDir(cfg.BaseDir, runID)
	cleanupOnError := func() {
		os.RemoveAll(filepath.Join(cfg.BaseDir, "runs", runID)) //nolint:errcheck
	}

	// Stage the WDL file if not already under baseDir.
	absWDL, err := filepath.Abs(wdlArg)
	if err != nil {
		fmt.Fprintf(stderr, "error: resolving WDL path %q: %v\n", wdlArg, err)
		cleanupOnError()
		return 1
	}
	absBase, err := filepath.Abs(cfg.BaseDir)
	if err != nil {
		fmt.Fprintf(stderr, "error: resolving base dir %q: %v\n", cfg.BaseDir, err)
		cleanupOnError()
		return 1
	}
	stagedWDLPath := absWDL
	if !strings.HasPrefix(absWDL, absBase+string(os.PathSeparator)) && absWDL != absBase {
		stagedWDLPath, err = stageFile(absWDL, inputsDir)
		if err != nil {
			fmt.Fprintf(stderr, "error: staging WDL file: %v\n", err)
			cleanupOnError()
			return 1
		}
	}

	// Load and stage input files.
	jsonInputs := map[string]interface{}{}
	if *inputFlag != "" {
		data, err := os.ReadFile(*inputFlag)
		if err != nil {
			fmt.Fprintf(stderr, "error: reading inputs file %q: %v\n", *inputFlag, err)
			cleanupOnError()
			return 1
		}
		if err := json.Unmarshal(data, &jsonInputs); err != nil {
			fmt.Fprintf(stderr, "error: parsing inputs JSON %q: %v\n", *inputFlag, err)
			cleanupOnError()
			return 1
		}
		jsonInputs, err = stageInputFiles(jsonInputs, inputsDir, absBase)
		if err != nil {
			fmt.Fprintf(stderr, "error: staging input files: %v\n", err)
			cleanupOnError()
			return 1
		}
	}

	// Collect search paths.
	var searchPaths []string
	if *pathFlag != "" {
		searchPaths = []string{*pathFlag}
	}

	// Submit to server.
	req := api.SubmitRequest{
		RunID:       runID,
		WDLPath:     stagedWDLPath,
		InputsJSON:  jsonInputs,
		SearchPaths: searchPaths,
	}

	c := client.New(serverURL)
	resp, err := c.Submit(context.Background(), req)
	if err != nil {
		fmt.Fprintf(stderr, "error: submitting workflow: %v\n", err)
		cleanupOnError()
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

// stageInputFiles returns a new map where string values that (a) resolve to
// existing local files and (b) are not already under baseDir are copied to
// inputsDir and replaced with their new staged path.
// Non-string values and non-existent paths are left unchanged.
func stageInputFiles(inputs map[string]interface{}, inputsDir, baseDir string) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(inputs))
	for k, v := range inputs {
		strVal, ok := v.(string)
		if !ok {
			result[k] = v
			continue
		}

		absPath, err := filepath.Abs(strVal)
		if err != nil {
			result[k] = v
			continue
		}

		// Check if the file exists.
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			result[k] = v
			continue
		}

		// Check if it is already under baseDir.
		absBase, err := filepath.Abs(baseDir)
		if err != nil {
			result[k] = v
			continue
		}
		if strings.HasPrefix(absPath, absBase+string(os.PathSeparator)) || absPath == absBase {
			result[k] = v
			continue
		}

		// Stage the file.
		stagedPath, err := stageFile(absPath, inputsDir)
		if err != nil {
			return nil, fmt.Errorf("stageInputFiles: staging %q: %w", strVal, err)
		}
		result[k] = stagedPath
	}
	return result, nil
}

// stageFile copies a single file to destDir, preserving the filename.
// Returns the absolute path to the staged file.
func stageFile(srcPath, destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("stageFile: create dest dir %q: %w", destDir, err)
	}
	destPath := filepath.Join(destDir, filepath.Base(srcPath))
	src, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("stageFile: open source %q: %w", srcPath, err)
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("stageFile: create dest %q: %w", destPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("stageFile: copy %q -> %q: %w", srcPath, destPath, err)
	}
	return destPath, nil
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
		"--server": true, "--config": true, "--base-dir": true,
		"-server": true, "-config": true, "-base-dir": true,
		"-i": true, "--i": true,
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
