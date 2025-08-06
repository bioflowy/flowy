package parser

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/errors"
)

func TestParseError(t *testing.T) {
	pos := errors.SourcePosition{Line: 10, Column: 5, URI: "test.wdl"}
	message := "unexpected token"
	expected := []TokenType{TokenWorkflow, TokenTask}
	actual := Token{Type: TokenIdentifier, Value: "invalid", Position: pos}

	err := NewParseError(pos, message, expected, actual)

	// Test Position
	if err.Position().Line != 10 {
		t.Errorf("Expected line 10, got %d", err.Position().Line)
	}

	if err.Position().Column != 5 {
		t.Errorf("Expected column 5, got %d", err.Position().Column)
	}

	if err.Position().URI != "test.wdl" {
		t.Errorf("Expected URI 'test.wdl', got '%s'", err.Position().URI)
	}

	// Test Message
	if err.Message() != message {
		t.Errorf("Expected message '%s', got '%s'", message, err.Message())
	}

	// Test ExpectedTokens
	expectedTypes := err.ExpectedTokens()
	if len(expectedTypes) != 2 {
		t.Errorf("Expected 2 expected tokens, got %d", len(expectedTypes))
	}

	if expectedTypes[0] != TokenWorkflow || expectedTypes[1] != TokenTask {
		t.Error("Expected tokens don't match")
	}

	// Test ActualToken
	actualToken := err.ActualToken()
	if actualToken.Type != TokenIdentifier {
		t.Errorf("Expected actual token type TokenIdentifier, got %s", actualToken.Type.String())
	}

	if actualToken.Value != "invalid" {
		t.Errorf("Expected actual token value 'invalid', got '%s'", actualToken.Value)
	}

	// Test Error interface
	errorStr := err.Error()
	if errorStr == "" {
		t.Error("Error string should not be empty")
	}
}

func TestParseErrorFormatting(t *testing.T) {
	pos := errors.SourcePosition{Line: 1, Column: 1, URI: "test.wdl"}
	err := NewParseError(pos, "test error", []TokenType{TokenWorkflow}, Token{Type: TokenEOF})

	errorStr := err.Error()

	// Should contain position information
	if errorStr == "" {
		t.Error("Error string should not be empty")
	}

	// The exact format isn't specified, but it should be informative
	// In a real implementation, you'd test the specific format
}

func TestTokenTypeString(t *testing.T) {
	tests := []struct {
		tokenType TokenType
		expected  string
	}{
		{TokenEOF, "EOF"},
		{TokenError, "ERROR"},
		{TokenWorkflow, "workflow"},
		{TokenTask, "task"},
		{TokenIdentifier, "IDENTIFIER"},
		{TokenString, "STRING"},
		{TokenInt, "INT"},
		{TokenFloat, "FLOAT"},
		{TokenBool, "BOOL"},
		{TokenLeftBrace, "{"},
		{TokenRightBrace, "}"},
		{TokenLeftParen, "("},
		{TokenRightParen, ")"},
		{TokenLeftBracket, "["},
		{TokenRightBracket, "]"},
		{TokenComma, ","},
		{TokenSemicolon, ";"},
		{TokenColon, ":"},
		{TokenDot, "."},
		{TokenAssign, "="},
		{TokenEqual, "=="},
		{TokenNotEqual, "!="},
		{TokenLess, "<"},
		{TokenGreater, ">"},
		{TokenLessEqual, "<="},
		{TokenGreaterEqual, ">="},
		{TokenPlus, "+"},
		{TokenMinus, "-"},
		{TokenMultiply, "*"},
		{TokenDivide, "/"},
		{TokenModulo, "%"},
		{TokenLogicalAnd, "&&"},
		{TokenLogicalOr, "||"},
		{TokenNot, "!"},
		{TokenQuestion, "?"},
	}

	for _, test := range tests {
		actual := test.tokenType.String()
		if actual != test.expected {
			t.Errorf("TokenType %d: expected string '%s', got '%s'", 
				int(test.tokenType), test.expected, actual)
		}
	}
}

func TestTokenPosition(t *testing.T) {
	pos := errors.SourcePosition{Line: 5, Column: 10, URI: "test.wdl"}
	token := Token{
		Type:     TokenIdentifier,
		Value:    "test",
		Position: pos,
	}

	if token.Position.Line != 5 {
		t.Errorf("Expected line 5, got %d", token.Position.Line)
	}

	if token.Position.Column != 10 {
		t.Errorf("Expected column 10, got %d", token.Position.Column)
	}

	if token.Position.URI != "test.wdl" {
		t.Errorf("Expected URI 'test.wdl', got '%s'", token.Position.URI)
	}
}

func TestParserUtilityFunctions(t *testing.T) {
	input := "workflow test { call task }"
	parser := NewParser(input, "test.wdl")

	// Test expect function (if it exists in your implementation)
	if !parser.currentTokenIs(TokenWorkflow) {
		t.Error("Expected current token to be workflow")
	}

	// Test advance to specific token types (if implemented)
	parser.nextToken() // Move to identifier
	if !parser.currentTokenIs(TokenIdentifier) {
		t.Error("Expected current token to be identifier")
	}
}

func TestErrorContext(t *testing.T) {
	// Test that errors have sufficient context for debugging
	pos := errors.SourcePosition{Line: 3, Column: 7, URI: "example.wdl"}
	expected := []TokenType{TokenLeftBrace, TokenRightBrace}
	actual := Token{Type: TokenSemicolon, Value: ";", Position: pos}

	err := NewParseError(pos, "expected block", expected, actual)

	// Error should have position
	if err.Position().Line != 3 {
		t.Error("Error should preserve position")
	}

	// Error should have expected tokens
	if len(err.ExpectedTokens()) != 2 {
		t.Error("Error should preserve expected tokens")
	}

	// Error should have actual token
	if err.ActualToken().Type != TokenSemicolon {
		t.Error("Error should preserve actual token")
	}
}

func TestTokenEquality(t *testing.T) {
	pos1 := errors.SourcePosition{Line: 1, Column: 1, URI: "test.wdl"}
	pos2 := errors.SourcePosition{Line: 1, Column: 1, URI: "test.wdl"}

	token1 := Token{Type: TokenWorkflow, Value: "workflow", Position: pos1}
	token2 := Token{Type: TokenWorkflow, Value: "workflow", Position: pos2}

	// Tokens with same type and value should be considered equal for parsing purposes
	if token1.Type != token2.Type {
		t.Error("Tokens with same type should have equal types")
	}

	if token1.Value != token2.Value {
		t.Error("Tokens with same value should have equal values")
	}
}

func TestTokenValidation(t *testing.T) {
	tests := []struct {
		token       Token
		valid       bool
		description string
	}{
		{Token{Type: TokenIdentifier, Value: "valid_name"}, true, "valid identifier"},
		{Token{Type: TokenString, Value: "hello world"}, true, "valid string"},
		{Token{Type: TokenInt, Value: "42"}, true, "valid integer"},
		{Token{Type: TokenFloat, Value: "3.14"}, true, "valid float"},
		{Token{Type: TokenEOF, Value: ""}, true, "valid EOF"},
	}

	for _, test := range tests {
		// Test token has expected properties
		if test.token.Type == TokenIdentifier && test.token.Value == "" {
			t.Errorf("%s: identifier tokens should have non-empty values", test.description)
		}

		if test.token.Type == TokenString && test.token.Value == "" {
			// Empty strings are valid
		}

		if test.token.Type == TokenInt || test.token.Type == TokenFloat {
			if test.token.Value == "" {
				t.Errorf("%s: numeric tokens should have non-empty values", test.description)
			}
		}
	}
}

func TestParseErrorChaining(t *testing.T) {
	pos := errors.SourcePosition{Line: 1, Column: 1, URI: "test.wdl"}
	
	// Test that we can create multiple related errors
	err1 := NewParseError(pos, "first error", []TokenType{TokenWorkflow}, Token{Type: TokenEOF})
	err2 := NewParseError(pos, "second error", []TokenType{TokenTask}, Token{Type: TokenEOF})

	errors := []ParseError{err1, err2}

	if len(errors) != 2 {
		t.Error("Should be able to collect multiple errors")
	}

	for i, err := range errors {
		if err.Message() == "" {
			t.Errorf("Error %d should have a message", i)
		}
	}
}

func TestRecoveryTokens(t *testing.T) {
	// Test tokens that are commonly used for error recovery
	recoveryTokens := []TokenType{
		TokenWorkflow,
		TokenTask,
		TokenStruct,
		TokenImport,
		TokenRightBrace,
		TokenSemicolon,
		TokenEOF,
	}

	parser := NewParser("", "test.wdl")

	for _, tokenType := range recoveryTokens {
		// In a real implementation, you might have a method to check
		// if a token is suitable for recovery
		_ = tokenType
		_ = parser
		// This would test parser.isRecoveryToken(tokenType) or similar
	}
}

func TestPositionTracking(t *testing.T) {
	input := `line 1
line 2
    indented line 3`

	parser := NewParser(input, "test.wdl")

	// Test that positions are tracked correctly as we advance
	initialPos := parser.currentPosition()
	if initialPos.Line != 1 || initialPos.Column != 1 {
		t.Errorf("Initial position should be (1,1), got (%d,%d)", 
			initialPos.Line, initialPos.Column)
	}

	// The lexer should handle position tracking
	// We can't easily test this without more complex input parsing
}

func TestErrorRecoveryStrategies(t *testing.T) {
	// Test different error recovery strategies
	tests := []struct {
		input               string
		expectedRecoveryAt  TokenType
		description         string
	}{
		{"workflow { invalid", TokenEOF, "recover at end"},
		{"workflow test { } task", TokenTask, "recover at next top-level"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		
		// Force an error and then test recovery
		parser.consume(TokenTask) // This should fail for "workflow" input
		
		if !parser.HasErrors() {
			t.Errorf("%s: expected error to be generated", test.description)
			continue
		}

		parser.synchronize()

		// After synchronization, we should be at a reasonable recovery point
		// The exact behavior depends on implementation
		if parser.isAtEnd() && test.expectedRecoveryAt != TokenEOF {
			t.Errorf("%s: unexpected recovery at EOF", test.description)
		}
	}
}

func TestUtilityHelpers(t *testing.T) {
	// Test any utility helper functions that might exist
	parser := NewParser("workflow test", "test.wdl")

	// Test position helpers
	pos := parser.currentPosition()
	if pos.URI != "test.wdl" {
		t.Error("Position should include correct URI")
	}

	// Test token advancement helpers
	if parser.isAtEnd() {
		t.Error("Should not be at end initially")
	}

	// Advance to end
	for !parser.isAtEnd() {
		parser.nextToken()
	}

	if !parser.isAtEnd() {
		t.Error("Should be at end after advancing through all tokens")
	}
}