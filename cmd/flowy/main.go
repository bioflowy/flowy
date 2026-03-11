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
  check   Parse and type-check a WDL file
  run     Execute a WDL workflow or task

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
		taskOutputs, execErr := task.RunTask(ctx, taskDef, jsonInputs, runner, execRoot)
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

