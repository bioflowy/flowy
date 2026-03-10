// Command flowy is the CLI entry point for the flowy WDL executor.
// Usage: flowy <command> [options]
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
		// Placeholder: implemented in Plan 02.
		return 0
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

