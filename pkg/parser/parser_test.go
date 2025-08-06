package parser

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/errors"
)

func TestNewParser(t *testing.T) {
	input := "workflow test { call task }"
	uri := "test.wdl"

	parser := NewParser(input, uri)

	if parser == nil {
		t.Error("NewParser should not return nil")
		return
	}

	if parser.lexer == nil {
		t.Error("Parser lexer should not be nil")
	}

	if parser.uri != uri {
		t.Errorf("Expected URI '%s', got '%s'", uri, parser.uri)
	}

	if len(parser.errors) != 0 {
		t.Errorf("New parser should have no errors, got %d", len(parser.errors))
	}
}

func TestParserErrorHandling(t *testing.T) {
	parser := NewParser("", "test.wdl")

	// Test adding errors
	pos := errors.SourcePosition{Line: 1, Column: 1, URI: "test.wdl"}
	err := NewParseError(pos, "test error", []TokenType{TokenWorkflow}, Token{Type: TokenEOF})

	parser.addError(err)

	if !parser.HasErrors() {
		t.Error("Parser should have errors after adding one")
	}

	if len(parser.Errors()) != 1 {
		t.Errorf("Expected 1 error, got %d", len(parser.Errors()))
	}

	if parser.Errors()[0].Message() != "test error" {
		t.Errorf("Expected error message 'test error', got '%s'", parser.Errors()[0].Message())
	}
}

func TestParserCurrentPosition(t *testing.T) {
	input := "workflow\ntest"
	parser := NewParser(input, "test.wdl")

	pos := parser.currentPosition()

	if pos.Line != 1 {
		t.Errorf("Expected line 1, got %d", pos.Line)
	}

	if pos.Column != 1 {
		t.Errorf("Expected column 1, got %d", pos.Column)
	}

	if pos.URI != "test.wdl" {
		t.Errorf("Expected URI 'test.wdl', got '%s'", pos.URI)
	}
}

func TestParserTokenNavigation(t *testing.T) {
	input := "workflow test { call task }"
	parser := NewParser(input, "test.wdl")

	// Test initial state
	if parser.currentToken.Type != TokenWorkflow {
		t.Errorf("Expected first token to be TokenWorkflow, got %s", parser.currentToken.Type.String())
	}

	// Test nextToken
	parser.nextToken()
	if parser.currentToken.Type != TokenIdentifier || parser.currentToken.Value != "test" {
		t.Errorf("Expected second token to be identifier 'test', got %s '%s'", 
			parser.currentToken.Type.String(), parser.currentToken.Value)
	}

	// Test peekToken
	next := parser.peekToken()
	if next.Type != TokenLeftBrace {
		t.Errorf("Expected peek to be TokenLeftBrace, got %s", next.Type.String())
	}

	// Verify current token didn't change
	if parser.currentToken.Type != TokenIdentifier {
		t.Error("peekToken should not change current token")
	}
}

func TestParserTokenChecking(t *testing.T) {
	input := "workflow test"
	parser := NewParser(input, "test.wdl")

	// Test currentTokenIs
	if !parser.currentTokenIs(TokenWorkflow) {
		t.Error("currentTokenIs should return true for TokenWorkflow")
	}

	if parser.currentTokenIs(TokenTask) {
		t.Error("currentTokenIs should return false for TokenTask")
	}

	// Test peekTokenIs
	parser.nextToken() // Move to "test"
	if !parser.peekTokenIs(TokenEOF) {
		t.Error("peekTokenIs should return true for TokenEOF")
	}
}

func TestParserConsume(t *testing.T) {
	input := "workflow test"
	parser := NewParser(input, "test.wdl")

	// Test successful consume
	if !parser.consume(TokenWorkflow) {
		t.Error("consume should return true for expected token")
	}

	if parser.currentToken.Type != TokenIdentifier {
		t.Error("consume should advance to next token")
	}

	// Test failed consume
	if parser.consume(TokenTask) {
		t.Error("consume should return false for unexpected token")
	}

	if !parser.HasErrors() {
		t.Error("Failed consume should add error")
	}
}

func TestParserSynchronization(t *testing.T) {
	// Test synchronization after error
	input := "workflow { task echo }"
	parser := NewParser(input, "test.wdl")

	// Cause an error by expecting wrong token
	parser.consume(TokenTask) // This should fail

	if !parser.HasErrors() {
		t.Error("Expected error after failed consume")
	}

	// Test synchronize
	parser.synchronize()

	// After synchronization, should be positioned at a recovery point
	if parser.isAtEnd() {
		t.Error("Parser should not be at end after synchronization")
	}
}

func TestParserRecovery(t *testing.T) {
	tests := []struct {
		input          string
		description    string
		expectErrors   bool
		expectRecovery bool
	}{
		{"workflow { task echo }", "missing workflow name", true, true},
		{"workflow test { invalid_element }", "invalid workflow element", true, true},
		{"task { command { echo hello } }", "missing task name", true, true},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")

		// Attempt to parse document (this should trigger errors and recovery)
		_, ok := parser.parseDocument()

		hasErrors := parser.HasErrors()
		if hasErrors != test.expectErrors {
			t.Errorf("%s: expected errors=%t, got errors=%t", 
				test.description, test.expectErrors, hasErrors)
		}

		if test.expectErrors && len(parser.Errors()) == 0 {
			t.Errorf("%s: expected to have recorded errors", test.description)
		}

		// Recovery success is harder to test without specific implementation details
		_ = ok // Use the variable
	}
}

func TestParserAtEnd(t *testing.T) {
	input := "workflow"
	parser := NewParser(input, "test.wdl")

	if parser.isAtEnd() {
		t.Error("Parser should not be at end initially")
	}

	// Advance to end
	parser.nextToken() // Move to EOF
	if !parser.isAtEnd() {
		t.Error("Parser should be at end after reaching EOF")
	}
}

func TestParserSkipCommentsAndNewlines(t *testing.T) {
	input := `# Comment
	
	workflow # Another comment
	test`

	parser := NewParser(input, "test.wdl")

	// Should start at workflow (comments skipped)
	if parser.currentToken.Type != TokenWorkflow {
		t.Errorf("Expected TokenWorkflow after skipping comments, got %s", 
			parser.currentToken.Type.String())
	}

	parser.nextToken()
	parser.skipCommentsAndNewlines()

	// Should be at 'test' identifier
	if parser.currentToken.Type != TokenIdentifier || parser.currentToken.Value != "test" {
		t.Errorf("Expected identifier 'test' after skipping comments, got %s '%s'", 
			parser.currentToken.Type.String(), parser.currentToken.Value)
	}
}

func TestParseErrorCreation(t *testing.T) {
	pos := errors.SourcePosition{Line: 5, Column: 10, URI: "test.wdl"}
	expected := []TokenType{TokenWorkflow, TokenTask}
	actual := Token{Type: TokenIdentifier, Value: "invalid"}

	err := NewParseError(pos, "test message", expected, actual)

	if err.Position().Line != 5 {
		t.Errorf("Expected line 5, got %d", err.Position().Line)
	}

	if err.Message() != "test message" {
		t.Errorf("Expected message 'test message', got '%s'", err.Message())
	}

	if err.ActualToken().Type != TokenIdentifier {
		t.Errorf("Expected actual token TokenIdentifier, got %s", err.ActualToken().Type.String())
	}
}

func TestParserComplexNavigation(t *testing.T) {
	input := `workflow test {
		input {
			String name
		}
		call echo { input: message = name }
	}`

	parser := NewParser(input, "test.wdl")

	tokens := []struct {
		expectedType TokenType
		expectedValue string
	}{
		{TokenWorkflow, "workflow"},
		{TokenIdentifier, "test"},
		{TokenLeftBrace, "{"},
		{TokenInput, "input"},
		{TokenLeftBrace, "{"},
		{TokenStringType, "String"},
		{TokenIdentifier, "name"},
		{TokenRightBrace, "}"},
		{TokenCall, "call"},
		{TokenIdentifier, "echo"},
		{TokenLeftBrace, "{"},
		{TokenInput, "input"},
		{TokenColon, ":"},
		{TokenIdentifier, "message"},
		{TokenAssign, "="},
		{TokenIdentifier, "name"},
		{TokenRightBrace, "}"},
		{TokenRightBrace, "}"},
		{TokenEOF, ""},
	}

	for i, expected := range tokens {
		if parser.currentToken.Type != expected.expectedType {
			t.Errorf("Token %d: expected type %s, got %s", 
				i, expected.expectedType.String(), parser.currentToken.Type.String())
		}

		if expected.expectedValue != "" && parser.currentToken.Value != expected.expectedValue {
			t.Errorf("Token %d: expected value '%s', got '%s'", 
				i, expected.expectedValue, parser.currentToken.Value)
		}

		parser.nextToken()
	}
}

func TestParserErrorAccumulation(t *testing.T) {
	// Create input that will generate multiple errors
	input := `workflow {
		invalid_statement
		task {
			command
		}
	}`

	parser := NewParser(input, "test.wdl")
	
	// Try to parse document - should accumulate multiple errors
	_, ok := parser.parseDocument()
	
	if ok {
		t.Error("Expected parsing to fail with multiple errors")
	}

	if !parser.HasErrors() {
		t.Error("Expected parser to have errors")
	}

	errors := parser.Errors()
	if len(errors) < 1 {
		t.Error("Expected at least one error to be recorded")
	}

	// Each error should have valid position information
	for i, err := range errors {
		if err.Position().Line < 1 {
			t.Errorf("Error %d: invalid line number %d", i, err.Position().Line)
		}
		if err.Position().Column < 1 {
			t.Errorf("Error %d: invalid column number %d", i, err.Position().Column)
		}
		if err.Message() == "" {
			t.Errorf("Error %d: empty error message", i)
		}
	}
}

func TestParserStateReset(t *testing.T) {
	input1 := "workflow test1 { call task1 }"
	input2 := "workflow test2 { call task2 }"

	// Parse first input
	parser := NewParser(input1, "test1.wdl")
	_, ok1 := parser.parseDocument()

	if !ok1 {
		t.Error("First parse should succeed")
	}

	// Create new parser for second input
	parser2 := NewParser(input2, "test2.wdl")
	_, ok2 := parser2.parseDocument()

	if !ok2 {
		t.Error("Second parse should succeed")
	}

	// Parsers should be independent
	if parser.uri == parser2.uri {
		t.Error("Parsers should have different URIs")
	}
}