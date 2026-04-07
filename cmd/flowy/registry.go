package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/bioflowy/flowy/internal/client"
)

// loadCredsAndClient loads credentials from ~/.flowy/credentials.yaml and returns
// a configured Client. On failure it prints a human-readable error and returns nil, 1.
func loadCredsAndClient(stderr io.Writer) (*client.Client, int) {
	creds, err := client.LoadCredentials()
	if err != nil {
		fmt.Fprintln(stderr, "Not authenticated. Run `flowy login --server <url>` first.")
		return nil, 1
	}
	return client.NewWithToken(creds.Server, creds.Token), 0
}

// handleClientError maps sentinel errors to human-readable messages.
// Returns exit code 1 for all cases.
func handleClientError(err error, stderr io.Writer) int {
	switch {
	case errors.Is(err, client.ErrUnauthorized):
		fmt.Fprintln(stderr, "Token expired or invalid. Run `flowy login` to authenticate.")
	case errors.Is(err, client.ErrForbidden):
		fmt.Fprintln(stderr, "Permission denied. Required role not granted for this operation.")
	default:
		fmt.Fprintln(stderr, err.Error())
	}
	return 1
}

// runRegistry dispatches to the appropriate registry subcommand.
func runRegistry(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, `Usage: flowy registry <subcommand> [options]

Subcommands:
  push        Push a WDL file to the registry
  list        List registry entries
  pull        Pull WDL content from the registry
  tag         Create a tag for a registry version
  delete-tag  Delete a tag from a registry entry
  delete      Delete a registry entry
`)
		return 2
	}

	switch args[0] {
	case "push":
		return runRegistryPush(args[1:], stdout, stderr)
	case "list":
		return runRegistryList(args[1:], stdout, stderr)
	case "pull":
		return runRegistryPull(args[1:], stdout, stderr)
	case "tag":
		return runRegistryTag(args[1:], stdout, stderr)
	case "delete-tag":
		return runRegistryDeleteTag(args[1:], stdout, stderr)
	case "delete":
		return runRegistryDelete(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown registry subcommand: %q\n", args[0])
		return 2
	}
}

// runRegistryPush implements "flowy registry push --project <slug> --name <name> <file.wdl>".
func runRegistryPush(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("registry push", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectFlag := fs.String("project", "", "project slug (required)")
	nameFlag := fs.String("name", "", "registry entry name (required)")

	args = reorderArgs(args)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *projectFlag == "" {
		fmt.Fprintln(stderr, "error: --project is required")
		fmt.Fprintln(stderr, "Usage: flowy registry push --project <slug> --name <name> <file.wdl>")
		return 2
	}
	if *nameFlag == "" {
		fmt.Fprintln(stderr, "error: --name is required")
		fmt.Fprintln(stderr, "Usage: flowy registry push --project <slug> --name <name> <file.wdl>")
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: registry push requires exactly one WDL file argument")
		fmt.Fprintln(stderr, "Usage: flowy registry push --project <slug> --name <name> <file.wdl>")
		return 2
	}

	filePath := fs.Arg(0)
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: reading WDL file %q: %v\n", filePath, err)
		return 1
	}

	c, code := loadCredsAndClient(stderr)
	if code != 0 {
		return code
	}

	resp, err := c.RegistryPush(context.Background(), *projectFlag, *nameFlag, string(content))
	if err != nil {
		return handleClientError(err, stderr)
	}

	fmt.Fprintf(stdout, "Pushed %s digest:%s\n", resp.Name, resp.Digest)
	return 0
}

// runRegistryList implements "flowy registry list --project <slug>".
func runRegistryList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("registry list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectFlag := fs.String("project", "", "project slug (required)")

	args = reorderArgs(args)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *projectFlag == "" {
		fmt.Fprintln(stderr, "error: --project is required")
		fmt.Fprintln(stderr, "Usage: flowy registry list --project <slug>")
		return 2
	}

	c, code := loadCredsAndClient(stderr)
	if code != 0 {
		return code
	}

	entries, err := c.RegistryList(context.Background(), *projectFlag)
	if err != nil {
		return handleClientError(err, stderr)
	}

	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tKIND\tLATEST TAG\tVERSIONS")
	for _, entry := range entries {
		latestTag := "-"
		if len(entry.Tags) > 0 {
			latestTag = entry.Tags[len(entry.Tags)-1]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", entry.Name, entry.Kind, latestTag, entry.VersionCount)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(stderr, "error: writing output: %v\n", err)
		return 1
	}
	return 0
}

// runRegistryPull implements "flowy registry pull --project <slug> --name <name> --tag <tag>".
func runRegistryPull(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("registry pull", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectFlag := fs.String("project", "", "project slug (required)")
	nameFlag := fs.String("name", "", "registry entry name (required)")
	tagFlag := fs.String("tag", "", "tag to pull")
	digestFlag := fs.String("digest", "", "digest to pull (alternative to --tag)")

	args = reorderArgs(args)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *projectFlag == "" {
		fmt.Fprintln(stderr, "error: --project is required")
		fmt.Fprintln(stderr, "Usage: flowy registry pull --project <slug> --name <name> --tag <tag>|--digest <digest>")
		return 2
	}
	if *nameFlag == "" {
		fmt.Fprintln(stderr, "error: --name is required")
		fmt.Fprintln(stderr, "Usage: flowy registry pull --project <slug> --name <name> --tag <tag>|--digest <digest>")
		return 2
	}
	if *tagFlag == "" && *digestFlag == "" {
		fmt.Fprintln(stderr, "error: must specify --tag or --digest")
		fmt.Fprintln(stderr, "Usage: flowy registry pull --project <slug> --name <name> --tag <tag>|--digest <digest>")
		return 2
	}

	c, code := loadCredsAndClient(stderr)
	if code != 0 {
		return code
	}

	resp, err := c.RegistryPull(context.Background(), *projectFlag, *nameFlag, *tagFlag, *digestFlag)
	if err != nil {
		return handleClientError(err, stderr)
	}

	// Print raw content with no added newline (pipeable).
	fmt.Fprint(stdout, resp.Content)
	return 0
}

// runRegistryTag implements "flowy registry tag --project <slug> --name <name> --tag <tag> --digest <digest>".
func runRegistryTag(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("registry tag", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectFlag := fs.String("project", "", "project slug (required)")
	nameFlag := fs.String("name", "", "registry entry name (required)")
	tagFlag := fs.String("tag", "", "tag name (required)")
	digestFlag := fs.String("digest", "", "version digest (required)")

	args = reorderArgs(args)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *projectFlag == "" || *nameFlag == "" || *tagFlag == "" || *digestFlag == "" {
		fmt.Fprintln(stderr, "error: --project, --name, --tag, and --digest are all required")
		fmt.Fprintln(stderr, "Usage: flowy registry tag --project <slug> --name <name> --tag <tag> --digest <digest>")
		return 2
	}

	c, code := loadCredsAndClient(stderr)
	if code != 0 {
		return code
	}

	if err := c.RegistryTag(context.Background(), *projectFlag, *nameFlag, *tagFlag, *digestFlag); err != nil {
		return handleClientError(err, stderr)
	}

	fmt.Fprintf(stdout, "Tagged %s %s\n", *nameFlag, *tagFlag)
	return 0
}

// runRegistryDeleteTag implements "flowy registry delete-tag --project <slug> --name <name> --tag <tag>".
func runRegistryDeleteTag(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("registry delete-tag", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectFlag := fs.String("project", "", "project slug (required)")
	nameFlag := fs.String("name", "", "registry entry name (required)")
	tagFlag := fs.String("tag", "", "tag to delete (required)")

	args = reorderArgs(args)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *projectFlag == "" || *nameFlag == "" || *tagFlag == "" {
		fmt.Fprintln(stderr, "error: --project, --name, and --tag are all required")
		fmt.Fprintln(stderr, "Usage: flowy registry delete-tag --project <slug> --name <name> --tag <tag>")
		return 2
	}

	c, code := loadCredsAndClient(stderr)
	if code != 0 {
		return code
	}

	if err := c.RegistryDeleteTag(context.Background(), *projectFlag, *nameFlag, *tagFlag); err != nil {
		return handleClientError(err, stderr)
	}

	fmt.Fprintf(stdout, "Deleted tag %s from %s\n", *tagFlag, *nameFlag)
	return 0
}

// runRegistryDelete implements "flowy registry delete --project <slug> --name <name>".
func runRegistryDelete(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("registry delete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectFlag := fs.String("project", "", "project slug (required)")
	nameFlag := fs.String("name", "", "registry entry name (required)")

	args = reorderArgs(args)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *projectFlag == "" || *nameFlag == "" {
		fmt.Fprintln(stderr, "error: --project and --name are required")
		fmt.Fprintln(stderr, "Usage: flowy registry delete --project <slug> --name <name>")
		return 2
	}

	c, code := loadCredsAndClient(stderr)
	if code != 0 {
		return code
	}

	if err := c.RegistryDelete(context.Background(), *projectFlag, *nameFlag); err != nil {
		return handleClientError(err, stderr)
	}

	fmt.Fprintf(stdout, "Deleted %s\n", *nameFlag)
	return 0
}
