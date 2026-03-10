package parser_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bioflowy/flowy/wdl/parser"
)

// TestParseWithSearchPaths tests the ParseWithSearchPaths function which extends
// import resolution to search additional directories beyond the source file directory.
func TestParseWithSearchPaths(t *testing.T) {
	// Helper: create a temp file with WDL content.
	writeFile := func(dir, name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("writeFile(%q): %v", path, err)
		}
		return path
	}

	t.Run("nil_search_paths_same_as_parse", func(t *testing.T) {
		// Test 1: ParseWithSearchPaths with nil searchPaths behaves identically to Parse.
		dir := t.TempDir()
		src := "version 1.2\ntask hello { command {} }\n"
		path := writeFile(dir, "main.wdl", src)

		doc1, err1 := parser.Parse(path, nil)
		doc2, err2 := parser.ParseWithSearchPaths(path, nil, nil)

		if err1 != nil || err2 != nil {
			t.Fatalf("Parse err=%v, ParseWithSearchPaths err=%v", err1, err2)
		}
		if len(doc1.Tasks) != len(doc2.Tasks) {
			t.Errorf("task count mismatch: Parse=%d, ParseWithSearchPaths=%d", len(doc1.Tasks), len(doc2.Tasks))
		}
		if doc1.Tasks[0].Name != doc2.Tasks[0].Name {
			t.Errorf("task name mismatch: %q vs %q", doc1.Tasks[0].Name, doc2.Tasks[0].Name)
		}
	})

	t.Run("resolve_import_from_search_path", func(t *testing.T) {
		// Test 2: ParseWithSearchPaths resolves an import from a secondary search
		// directory when the file is NOT in the same directory as the source.
		mainDir := t.TempDir()
		libDir := t.TempDir()

		// Put imported file in libDir only (not in mainDir).
		writeFile(libDir, "lib.wdl", "version 1.2\ntask libTask { command {} }\n")

		// Main file imports lib.wdl which is only in libDir.
		mainSrc := "version 1.2\nimport \"lib.wdl\"\n"
		mainPath := writeFile(mainDir, "main.wdl", mainSrc)

		doc, err := parser.ParseWithSearchPaths(mainPath, nil, []string{libDir})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(doc.Tasks) != 1 || doc.Tasks[0].Name != "libTask" {
			t.Errorf("expected libTask to be imported, got tasks=%v", doc.Tasks)
		}
	})

	t.Run("primary_dir_takes_precedence", func(t *testing.T) {
		// Test 3: ParseWithSearchPaths tries filepath.Dir(file) first before search paths.
		mainDir := t.TempDir()
		secondaryDir := t.TempDir()

		// Put two different versions of shared.wdl in each dir.
		writeFile(mainDir, "shared.wdl", "version 1.2\ntask primaryTask { command {} }\n")
		writeFile(secondaryDir, "shared.wdl", "version 1.2\ntask secondaryTask { command {} }\n")

		mainSrc := "version 1.2\nimport \"shared.wdl\"\n"
		mainPath := writeFile(mainDir, "main.wdl", mainSrc)

		doc, err := parser.ParseWithSearchPaths(mainPath, nil, []string{secondaryDir})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should find primaryTask from mainDir (primary directory wins).
		if len(doc.Tasks) != 1 || doc.Tasks[0].Name != "primaryTask" {
			t.Errorf("expected primaryTask from primary dir, got tasks=%v", doc.Tasks)
		}
	})

	t.Run("import_not_found_in_any_path_returns_error", func(t *testing.T) {
		// Test 4: ParseWithSearchPaths returns "cannot read file" error when import
		// is not found in any search path.
		mainDir := t.TempDir()
		extraDir := t.TempDir()

		mainSrc := "version 1.2\nimport \"nonexistent.wdl\"\n"
		mainPath := writeFile(mainDir, "main.wdl", mainSrc)

		_, err := parser.ParseWithSearchPaths(mainPath, nil, []string{extraDir})
		if err == nil {
			t.Fatal("expected error for missing import, got nil")
		}
		if !strings.Contains(err.Error(), "cannot read file") {
			t.Errorf("expected 'cannot read file' in error, got: %v", err)
		}
	})
}
