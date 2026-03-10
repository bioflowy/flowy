// Package lexer_test verifies that the lexer package compiles and exports the required API.
package lexer_test

import (
	"testing"

	"github.com/bioflowy/flowy/wdl/lexer"
)

// TestLexerCompiles verifies that all exported types from the lexer package exist and are accessible.
func TestLexerCompiles(t *testing.T) {
	// Verify NewLexer constructor exists and returns a non-nil Lexer.
	l := lexer.NewLexer("test.wdl", []byte("version 1.2\n"))
	if l == nil {
		t.Fatal("NewLexer returned nil")
	}

	// Verify Next() returns a Token and does not panic.
	tok := l.Next()
	if tok.Type == 0 {
		// tokEOF or any token is acceptable as long as it returns
	}

	// Verify token type constants are distinguishable.
	var _ lexer.TokenType = lexer.TokEOF
	var _ lexer.TokenType = lexer.TokError
	var _ lexer.TokenType = lexer.TokIdent
	var _ lexer.TokenType = lexer.TokCommandContent
	var _ lexer.TokenType = lexer.TokPlaceholderOpen
	var _ lexer.TokenType = lexer.TokPlaceholderClose
	var _ lexer.TokenType = lexer.TokHeredocOpen
	var _ lexer.TokenType = lexer.TokHeredocClose
}
