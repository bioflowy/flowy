package parser_test

import (
	"testing"

	"github.com/bioflowy/flowy/wdl/parser"
)

// TestParserCompile verifies that the parser package exports the expected API surface.
// This is a TDD RED test that fails until parser.go and parse_types.go are implemented.
func TestParserCompile(t *testing.T) {
	// Verify ParseError is exported and implements error interface
	var _ error = (*parser.ParseError)(nil)

	// Verify Parse function is exported
	doc, err := parser.Parse("test.wdl", []byte("version 1.2\n"))
	_ = doc
	_ = err
}
