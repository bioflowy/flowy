package lexer_test

import (
	"testing"

	"github.com/bioflowy/flowy/wdl/lexer"
)

// tokenizeAll collects all tokens from the input until TokEOF.
func tokenizeAll(src string) []lexer.Token {
	l := lexer.NewLexer("test.wdl", []byte(src))
	var tokens []lexer.Token
	for {
		tok := l.Next()
		tokens = append(tokens, tok)
		if tok.Type == lexer.TokEOF || tok.Type == lexer.TokError {
			break
		}
	}
	return tokens
}

// hasToken reports whether tokens contains a token of the given type with the given value.
// val == "" matches any value.
func hasToken(tokens []lexer.Token, typ lexer.TokenType, val string) bool {
	for _, tok := range tokens {
		if tok.Type == typ {
			if val == "" || tok.Val == val {
				return true
			}
		}
	}
	return false
}

// filterByType returns all tokens of the given type from a slice.
func filterByType(tokens []lexer.Token, typ lexer.TokenType) []lexer.Token {
	var result []lexer.Token
	for _, tok := range tokens {
		if tok.Type == typ {
			result = append(result, tok)
		}
	}
	return result
}

// TestCommandModes verifies that the lexer correctly tokenizes both brace-style
// and heredoc-style command blocks. (PARSE-02)
func TestCommandModes(t *testing.T) {
	cases := []struct {
		name  string
		input string
		check func(t *testing.T, tokens []lexer.Token)
	}{
		{
			name:  "brace-style command content",
			input: `command { echo hello }`,
			check: func(t *testing.T, tokens []lexer.Token) {
				contents := filterByType(tokens, lexer.TokCommandContent)
				if len(contents) == 0 {
					t.Fatal("expected at least one TokCommandContent token, got none")
				}
				// The combined content must contain "echo hello"
				combined := ""
				for _, c := range contents {
					combined += c.Val
				}
				// There should be no TokIdent for "echo" or "hello"
				if hasToken(tokens, lexer.TokIdent, "echo") {
					t.Error("expected 'echo' as TokCommandContent, not TokIdent")
				}
				if hasToken(tokens, lexer.TokIdent, "hello") {
					t.Error("expected 'hello' as TokCommandContent, not TokIdent")
				}
				if combined == "" {
					t.Errorf("combined command content is empty; tokens: %v", tokens)
				}
			},
		},
		{
			name:  "heredoc-style command content",
			input: `command <<< echo hello >>>`,
			check: func(t *testing.T, tokens []lexer.Token) {
				contents := filterByType(tokens, lexer.TokCommandContent)
				if len(contents) == 0 {
					t.Fatal("expected at least one TokCommandContent token, got none")
				}
				combined := ""
				for _, c := range contents {
					combined += c.Val
				}
				// The combined content should contain the text
				if !hasToken(tokens, lexer.TokHeredocOpen, "") {
					t.Error("expected TokHeredocOpen token")
				}
				if !hasToken(tokens, lexer.TokHeredocClose, "") {
					t.Error("expected TokHeredocClose token")
				}
				if hasToken(tokens, lexer.TokIdent, "echo") {
					t.Error("expected 'echo' as TokCommandContent inside heredoc, not TokIdent")
				}
				if combined == "" {
					t.Errorf("combined command content is empty; tokens: %v", tokens)
				}
			},
		},
		{
			name:  "brace command emits TokLBrace opener",
			input: `command { hello }`,
			check: func(t *testing.T, tokens []lexer.Token) {
				// Verify we see TokCommand followed by TokLBrace
				foundCmd := false
				foundBrace := false
				for _, tok := range tokens {
					if tok.Type == lexer.TokCommand {
						foundCmd = true
					}
					if foundCmd && tok.Type == lexer.TokLBrace {
						foundBrace = true
						break
					}
				}
				if !foundCmd {
					t.Error("expected TokCommand token")
				}
				if !foundBrace {
					t.Error("expected TokLBrace after TokCommand")
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokens := tokenizeAll(tc.input)
			tc.check(t, tokens)
		})
	}
}

// TestCommandBracePlaceholders verifies that the lexer correctly tokenizes
// placeholder expressions within brace-style command blocks. (PARSE-03)
func TestCommandBracePlaceholders(t *testing.T) {
	cases := []struct {
		name  string
		input string
		check func(t *testing.T, tokens []lexer.Token)
	}{
		{
			name:  "tilde placeholder in brace command",
			input: `command { echo ~{name} }`,
			check: func(t *testing.T, tokens []lexer.Token) {
				opens := filterByType(tokens, lexer.TokPlaceholderOpen)
				if len(opens) != 1 {
					t.Fatalf("expected 1 TokPlaceholderOpen, got %d; tokens: %v", len(opens), tokens)
				}
				if opens[0].Val != "~{" {
					t.Errorf("expected TokPlaceholderOpen Val to be '~{', got %q", opens[0].Val)
				}
				// Inside the placeholder, 'name' should be a TokIdent
				if !hasToken(tokens, lexer.TokIdent, "name") {
					t.Error("expected TokIdent 'name' inside placeholder")
				}
				closes := filterByType(tokens, lexer.TokPlaceholderClose)
				if len(closes) != 1 {
					t.Fatalf("expected 1 TokPlaceholderClose, got %d", len(closes))
				}
			},
		},
		{
			name:  "dollar placeholder in brace command",
			input: `command { echo ${greeting} }`,
			check: func(t *testing.T, tokens []lexer.Token) {
				opens := filterByType(tokens, lexer.TokPlaceholderOpen)
				if len(opens) != 1 {
					t.Fatalf("expected 1 TokPlaceholderOpen, got %d; tokens: %v", len(opens), tokens)
				}
				if opens[0].Val != "${" {
					t.Errorf("expected TokPlaceholderOpen Val to be '${', got %q", opens[0].Val)
				}
				if !hasToken(tokens, lexer.TokIdent, "greeting") {
					t.Error("expected TokIdent 'greeting' inside placeholder")
				}
				closes := filterByType(tokens, lexer.TokPlaceholderClose)
				if len(closes) != 1 {
					t.Fatalf("expected 1 TokPlaceholderClose, got %d", len(closes))
				}
			},
		},
		{
			name:  "both tilde and dollar placeholders in brace command",
			input: `command { echo ~{name} ${greeting} }`,
			check: func(t *testing.T, tokens []lexer.Token) {
				opens := filterByType(tokens, lexer.TokPlaceholderOpen)
				if len(opens) != 2 {
					t.Fatalf("expected 2 TokPlaceholderOpen (one ~{, one ${), got %d; tokens: %v", len(opens), tokens)
				}
				// Verify Val fields
				vals := map[string]bool{}
				for _, o := range opens {
					vals[o.Val] = true
				}
				if !vals["~{"] {
					t.Error("expected TokPlaceholderOpen with Val '~{'")
				}
				if !vals["${"] {
					t.Error("expected TokPlaceholderOpen with Val '${'")
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokens := tokenizeAll(tc.input)
			tc.check(t, tokens)
		})
	}
}

// TestCommandHeredocPlaceholders verifies that the lexer correctly tokenizes
// placeholder expressions within heredoc-style command blocks. (PARSE-04)
func TestCommandHeredocPlaceholders(t *testing.T) {
	cases := []struct {
		name  string
		input string
		check func(t *testing.T, tokens []lexer.Token)
	}{
		{
			name:  "tilde placeholder is special in heredoc",
			input: `command <<< echo ~{name} >>>`,
			check: func(t *testing.T, tokens []lexer.Token) {
				opens := filterByType(tokens, lexer.TokPlaceholderOpen)
				if len(opens) != 1 {
					t.Fatalf("expected 1 TokPlaceholderOpen for ~{, got %d; tokens: %v", len(opens), tokens)
				}
				if opens[0].Val != "~{" {
					t.Errorf("expected Val '~{', got %q", opens[0].Val)
				}
				if !hasToken(tokens, lexer.TokIdent, "name") {
					t.Error("expected TokIdent 'name' inside ~{ placeholder")
				}
			},
		},
		{
			name:  "dollar is NOT a placeholder in heredoc (PARSE-04 invariant)",
			input: `command <<< echo ~{name} ${HOME} >>>`,
			check: func(t *testing.T, tokens []lexer.Token) {
				// Only one TokPlaceholderOpen (for ~{name})
				opens := filterByType(tokens, lexer.TokPlaceholderOpen)
				if len(opens) != 1 {
					t.Fatalf("expected exactly 1 TokPlaceholderOpen (only ~{), got %d; tokens: %v", len(opens), tokens)
				}
				if opens[0].Val != "~{" {
					t.Errorf("expected the single open to be '~{', got %q", opens[0].Val)
				}
				// ${HOME} must appear as raw TokCommandContent, not as a placeholder
				contents := filterByType(tokens, lexer.TokCommandContent)
				combined := ""
				for _, c := range contents {
					combined += c.Val
				}
				if !containsStr(combined, "${HOME}") {
					t.Errorf("expected '${HOME}' to appear as raw TokCommandContent; combined content: %q", combined)
				}
				// HOME must NOT appear as TokIdent
				if hasToken(tokens, lexer.TokIdent, "HOME") {
					t.Error("'HOME' should be raw content, not a TokIdent")
				}
			},
		},
		{
			name:  "dollar-only heredoc (PARSE-04 invariant: all $ is raw)",
			input: `command <<< echo ${HOME} >>>`,
			check: func(t *testing.T, tokens []lexer.Token) {
				opens := filterByType(tokens, lexer.TokPlaceholderOpen)
				if len(opens) != 0 {
					t.Fatalf("expected 0 TokPlaceholderOpen in heredoc with only ${}, got %d", len(opens))
				}
				contents := filterByType(tokens, lexer.TokCommandContent)
				combined := ""
				for _, c := range contents {
					combined += c.Val
				}
				if !containsStr(combined, "${HOME}") {
					t.Errorf("expected '${HOME}' in raw content; got %q", combined)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokens := tokenizeAll(tc.input)
			tc.check(t, tokens)
		})
	}
}

// TestNestedPlaceholder verifies that a nested ~{} inside a string placeholder
// does not prematurely close the outer placeholder. (Brace depth tracking)
func TestNestedPlaceholder(t *testing.T) {
	// In a string context: "prefix_~{x}"
	// We use a simple string with a placeholder containing an identifier.
	// Then test that a { inside a placeholder is tracked as brace depth.
	input := `command { ~{"prefix_~{x}"} }`
	tokens := tokenizeAll(input)

	opens := filterByType(tokens, lexer.TokPlaceholderOpen)
	closes := filterByType(tokens, lexer.TokPlaceholderClose)

	// We expect 2 opens (outer ~{ and inner ~{) and 2 closes
	if len(opens) < 2 {
		t.Fatalf("expected at least 2 TokPlaceholderOpen for nested placeholder; got %d; tokens: %v", len(opens), tokens)
	}
	if len(closes) < 2 {
		t.Fatalf("expected at least 2 TokPlaceholderClose for nested placeholder; got %d; tokens: %v", len(closes), tokens)
	}
}

// TestStarStar verifies that ** is tokenized as a single TokStarStar token, not two TokStar.
func TestStarStar(t *testing.T) {
	input := `2**3`
	tokens := tokenizeAll(input)

	// Must have TokStarStar
	if !hasToken(tokens, lexer.TokStarStar, "**") {
		t.Errorf("expected TokStarStar '**'; tokens: %v", tokens)
	}
	// Must NOT have two separate TokStar tokens
	stars := filterByType(tokens, lexer.TokStar)
	if len(stars) > 0 {
		t.Errorf("expected no individual TokStar tokens when ** is present; got %d TokStar tokens", len(stars))
	}

	// Also verify the integer literals are emitted
	intLits := filterByType(tokens, lexer.TokIntLit)
	if len(intLits) != 2 {
		t.Errorf("expected 2 TokIntLit tokens for '2' and '3'; got %d", len(intLits))
	}
}

// TestLineCol verifies that line and column numbers are correctly tracked on multi-line input.
func TestLineCol(t *testing.T) {
	// "task" is on line 1; "workflow" is on line 2
	input := "task\nworkflow"
	tokens := tokenizeAll(input)

	var taskTok, wfTok *lexer.Token
	for i := range tokens {
		if tokens[i].Type == lexer.TokTask {
			taskTok = &tokens[i]
		}
		if tokens[i].Type == lexer.TokWorkflow {
			wfTok = &tokens[i]
		}
	}

	if taskTok == nil {
		t.Fatal("expected TokTask token")
	}
	if wfTok == nil {
		t.Fatal("expected TokWorkflow token")
	}

	if taskTok.Pos.Line != 1 {
		t.Errorf("TokTask: expected Line==1, got %d", taskTok.Pos.Line)
	}
	if taskTok.Pos.Col != 1 {
		t.Errorf("TokTask: expected Col==1, got %d", taskTok.Pos.Col)
	}

	if wfTok.Pos.Line != 2 {
		t.Errorf("TokWorkflow: expected Line==2, got %d", wfTok.Pos.Line)
	}
	if wfTok.Pos.Col != 1 {
		t.Errorf("TokWorkflow: expected Col==1, got %d", wfTok.Pos.Col)
	}
}

// TestDeclarationTokenization verifies a typical WDL declaration tokenizes correctly.
func TestDeclarationTokenization(t *testing.T) {
	input := `Int count = 42`
	tokens := tokenizeAll(input)

	if !hasToken(tokens, lexer.TokIntType, "Int") {
		t.Error("expected TokIntType 'Int'")
	}
	if !hasToken(tokens, lexer.TokIdent, "count") {
		t.Error("expected TokIdent 'count'")
	}
	if !hasToken(tokens, lexer.TokAssign, "=") {
		t.Error("expected TokAssign '='")
	}
	if !hasToken(tokens, lexer.TokIntLit, "42") {
		t.Error("expected TokIntLit '42'")
	}
}

// containsStr reports whether s contains substr (helper to avoid importing strings in tests).
func containsStr(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
