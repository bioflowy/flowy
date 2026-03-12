// Command flowy is the CLI entry point for the flowy WDL executor.
// Usage: flowy <command> [options]
package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/bioflowy/flowy/internal/client"
	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/config"
	"github.com/bioflowy/flowy/internal/store"
	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/checker"
	"github.com/bioflowy/flowy/wdl/parser"
	"github.com/bioflowy/flowy/wdl/task"
	"github.com/bioflowy/flowy/wdl/workflow"
)

// version is set at build time via ldflags. Default is "dev".
var version = "dev"

// newRunner is a package-level factory for creating ContainerRunner instances.
// It can be overridden in tests to inject a mock runner without subprocess.
var newRunner func() task.ContainerRunner = func() task.ContainerRunner {
	return task.NewDockerRunner()
}

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

// runRun implements the "run" subcommand.
// It parses a WDL file, type-checks it, then executes either a named task
// or the first workflow using the provided ContainerRunner.
//
// Exit codes: 0 = success, 1 = execution error, 2 = usage error.
func runRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)

	inputFlag := fs.String("input", "", "JSON input file path")
	taskFlag := fs.String("task", "", "run only the named task (skip workflow)")
	dirFlag := fs.String("dir", "", "execution directory (must be empty if it exists)")
	pathFlag := fs.String("path", "", "additional import search directory")
	verboseFlag := fs.Bool("verbose", false, "print task stdout/stderr after execution")
	outputFlag := fs.String("output", "", "write outputs JSON to specified file path instead of stdout")

	// Reorder args so flags after positional args are parsed correctly.
	// Go's flag package stops parsing at the first non-flag argument,
	// but spectool places --task after the WDL file path.
	args = reorderArgs(args)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: run requires exactly one WDL file argument")
		fmt.Fprintln(stderr, "Usage: flowy run [--input <file>] [--task <name>] [--dir <dir>] [--path <dir>] [--verbose] [--output <file>] <file.wdl>")
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
		fmt.Fprintln(stderr, parseErr.Error())
		return 1
	}

	// Type-check the document.
	_, typeErrors := checker.Check(doc)
	if len(typeErrors) > 0 {
		for _, err := range typeErrors {
			if te, ok := err.(*checker.TypeError); ok {
				srcLine := readSourceLine(te.Pos.File, te.Pos.Line)
				printRustcError(stderr, te.Pos.File, te.Pos.Line, te.Pos.Col, te.Message, srcLine)
			} else {
				fmt.Fprintln(stderr, err.Error())
			}
		}
		return 1
	}

	// Load JSON inputs.
	jsonInputs := map[string]interface{}{}
	if *inputFlag != "" {
		data, err := os.ReadFile(*inputFlag)
		if err != nil {
			fmt.Fprintf(stderr, "error: reading input file %q: %v\n", *inputFlag, err)
			return 1
		}
		if err := json.Unmarshal(data, &jsonInputs); err != nil {
			fmt.Fprintf(stderr, "error: parsing input JSON %q: %v\n", *inputFlag, err)
			return 1
		}
	}

	// Set up execution directory.
	execRoot, err := setupExecDir(*dirFlag)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	// Print execution directory before any task execution.
	fmt.Fprintf(stderr, "Execution directory: %s\n", execRoot)

	// Create the runner.
	runner := newRunner()
	ctx := context.Background()

	// Execute.
	var jsonBytes []byte
	if *taskFlag != "" {
		// Find the named task.
		var taskDef *ast.Task
		for _, t := range doc.Tasks {
			if t.Name == *taskFlag {
				taskDef = t
				break
			}
		}
		if taskDef == nil {
			fmt.Fprintf(stderr, "error: task %q not found in %s\n", *taskFlag, file)
			return 1
		}
		// Build struct definitions map for struct literal evaluation.
		structDefs := make(map[string]*ast.StructDef, len(doc.Structs))
		for _, s := range doc.Structs {
			structDefs[s.Name] = s
		}
		taskOutputs, execErr := task.RunTask(ctx, taskDef, jsonInputs, runner, execRoot, structDefs)
		if execErr != nil {
			fmt.Fprintf(stderr, "error: %v\n", execErr)
			fmt.Fprintf(stderr, "See %s/error.json for details\n", execRoot)
			return 1
		}
		// Serialize task outputs with taskName.outputName keys.
		var serErr error
		jsonBytes, serErr = workflow.SerializeOutputs(*taskFlag, taskOutputs)
		if serErr != nil {
			fmt.Fprintf(stderr, "error: serializing task outputs: %v\n", serErr)
			return 1
		}
		// Write outputs.json to execRoot for task runs.
		if err := os.WriteFile(filepath.Join(execRoot, "outputs.json"), jsonBytes, 0644); err != nil {
			fmt.Fprintf(stderr, "error: writing outputs.json: %v\n", err)
			return 1
		}
	} else {
		// Execute the first workflow.
		if len(doc.Workflows) == 0 {
			fmt.Fprintf(stderr, "error: no workflow defined in %s (use --task to run a task)\n", file)
			return 1
		}
		wfName := doc.Workflows[0].Name
		wfOutputs, execErr := workflow.RunWorkflow(ctx, doc, wfName, jsonInputs, runner, execRoot)
		if execErr != nil {
			fmt.Fprintf(stderr, "error: %v\n", execErr)
			fmt.Fprintf(stderr, "See %s/error.json for details\n", execRoot)
			return 1
		}
		// Serialize workflow outputs with wfName.outputName keys.
		// RunWorkflow already wrote outputs.json internally; we just need JSON for stdout.
		var serErr error
		jsonBytes, serErr = workflow.SerializeOutputs(wfName, wfOutputs)
		if serErr != nil {
			fmt.Fprintf(stderr, "error: serializing workflow outputs: %v\n", serErr)
			return 1
		}
	}

	// Print verbose output if requested.
	if *verboseFlag {
		printVerboseOutput(execRoot, stderr)
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

// setupExecDir prepares the execution directory.
// If dirFlag is set, it validates the directory is empty (or doesn't exist) and creates it.
// If dirFlag is empty, it generates a unique temp directory with "flowy-" prefix.
func setupExecDir(dirFlag string) (string, error) {
	if dirFlag != "" {
		// Check if the directory exists and is non-empty.
		entries, err := os.ReadDir(dirFlag)
		if err == nil && len(entries) > 0 {
			return "", fmt.Errorf("execution directory %q is not empty; use an empty or new directory", dirFlag)
		}
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("checking execution directory %q: %v", dirFlag, err)
		}
		if err := os.MkdirAll(dirFlag, 0755); err != nil {
			return "", fmt.Errorf("creating execution directory %q: %v", dirFlag, err)
		}
		return dirFlag, nil
	}

	// Generate a unique temp directory with "flowy-" prefix.
	suffix, err := randomHex(4)
	if err != nil {
		return "", fmt.Errorf("generating execution directory name: %v", err)
	}
	dirName := fmt.Sprintf("flowy-%s-%s", time.Now().Format("20060102"), suffix)
	execRoot := filepath.Join(os.TempDir(), dirName)
	if err := os.MkdirAll(execRoot, 0755); err != nil {
		return "", fmt.Errorf("creating execution directory %q: %v", execRoot, err)
	}
	return execRoot, nil
}

// printVerboseOutput walks the execution directory and prints stdout/stderr
// files from task subdirectories to w. This is the post-hoc approach:
// output appears after all tasks finish (no API changes to inner packages).
func printVerboseOutput(execRoot string, w io.Writer) {
	entries, err := os.ReadDir(execRoot)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		taskDir := filepath.Join(execRoot, entry.Name())

		// Print stdout.
		stdoutPath := filepath.Join(taskDir, "stdout.txt")
		if data, err := os.ReadFile(stdoutPath); err == nil && len(data) > 0 {
			fmt.Fprintf(w, "--- %s stdout ---\n", entry.Name())
			w.Write(data) //nolint:errcheck
		}

		// Print stderr.
		stderrPath := filepath.Join(taskDir, "stderr.txt")
		if data, err := os.ReadFile(stderrPath); err == nil && len(data) > 0 {
			fmt.Fprintf(w, "--- %s stderr ---\n", entry.Name())
			w.Write(data) //nolint:errcheck
		}
	}
}

// randomHex generates n random bytes and returns their hex encoding.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
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
		"--input": true, "--task": true, "--dir": true,
		"--path": true, "--output": true,
		"-input": true, "-task": true, "-dir": true,
		"-path": true, "-output": true,
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

