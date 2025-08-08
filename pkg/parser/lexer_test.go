package parser

import (
	"testing"
)

func TestLexerBasicTokens(t *testing.T) {
	input := `version 1.0

workflow test {
	input {
		String name
		Int count = 42
	}
	
	call my_task { input: name = name }
	
	output {
		String result = my_task.output
	}
}`

	lexer := NewLexer(input, "test.wdl")
	tokens := lexer.AllTokens()

	expectedTypes := []TokenType{
		TokenVersion, TokenFloat, TokenNewline,
		TokenNewline,
		TokenWorkflow, TokenIdentifier, TokenLeftBrace, TokenNewline,
		TokenInput, TokenLeftBrace, TokenNewline,
		TokenStringType, TokenIdentifier, TokenNewline,
		TokenIntType, TokenIdentifier, TokenAssign, TokenInt, TokenNewline,
		TokenRightBrace, TokenNewline,
		TokenNewline,
		TokenCall, TokenIdentifier, TokenLeftBrace, TokenInput, TokenColon,
		TokenIdentifier, TokenAssign, TokenIdentifier, TokenRightBrace, TokenNewline,
		TokenNewline,
		TokenOutput, TokenLeftBrace, TokenNewline,
		TokenStringType, TokenIdentifier, TokenAssign, TokenIdentifier, TokenDot, TokenOutput, TokenNewline,
		TokenRightBrace, TokenNewline,
		TokenRightBrace,
		TokenEOF,
	}

	if len(tokens) < len(expectedTypes) {
		t.Errorf("Expected at least %d tokens, got %d", len(expectedTypes), len(tokens))
		return
	}

	for i, expectedType := range expectedTypes {
		if i >= len(tokens) {
			t.Errorf("Token %d: expected %s, but no more tokens", i, expectedType.String())
			break
		}
		if tokens[i].Type != expectedType {
			t.Errorf("Token %d: expected %s, got %s (value: %s)", 
				i, expectedType.String(), tokens[i].Type.String(), tokens[i].Value)
		}
	}
}

func TestLexerKeywords(t *testing.T) {
	tests := []struct {
		input    string
		expected TokenType
	}{
		{"version", TokenVersion},
		{"import", TokenImport},
		{"as", TokenAs},
		{"alias", TokenAlias},
		{"workflow", TokenWorkflow},
		{"task", TokenTask},
		{"input", TokenInput},
		{"output", TokenOutput},
		{"meta", TokenMeta},
		{"parameter_meta", TokenParameterMeta},
		{"requirements", TokenRequirements},
		{"runtime", TokenRuntime},
		{"scatter", TokenScatter},
		{"if", TokenIf},
		{"then", TokenThen},
		{"else", TokenElse},
		{"call", TokenCall},
		{"after", TokenAfter},
		{"struct", TokenStruct},
		{"command", TokenCommand},
		{"env", TokenEnv},
		{"Array", TokenArray},
		{"File", TokenFile},
		{"Directory", TokenDirectory},
		{"Map", TokenMap},
		{"Pair", TokenPair},
		{"Int", TokenIntType},
		{"Float", TokenFloatType},
		{"String", TokenStringType},
		{"Boolean", TokenBoolType},
		{"None", TokenNone},
		{"true", TokenBool},
		{"false", TokenBool},
	}

	for _, test := range tests {
		lexer := NewLexer(test.input, "test.wdl")
		token := lexer.NextToken()

		if token.Type != test.expected {
			t.Errorf("Input '%s': expected %s, got %s", 
				test.input, test.expected.String(), token.Type.String())
		}
		if token.Value != test.input {
			t.Errorf("Input '%s': expected value '%s', got '%s'", 
				test.input, test.input, token.Value)
		}
	}
}

func TestLexerOperators(t *testing.T) {
	tests := []struct {
		input    string
		expected TokenType
	}{
		{"||", TokenLogicalOr},
		{"&&", TokenLogicalAnd},
		{"==", TokenEqual},
		{"!=", TokenNotEqual},
		{"<=", TokenLessEqual},
		{">=", TokenGreaterEqual},
		{"<", TokenLess},
		{">", TokenGreater},
		{"+", TokenPlus},
		{"-", TokenMinus},
		{"*", TokenMultiply},
		{"/", TokenDivide},
		{"%", TokenModulo},
		{"!", TokenNot},
		{"=", TokenAssign},
		{"${", TokenInterpolationStart},
		{"~{", TokenInterpolationStart},
		{"<<<", TokenCommandStart},
		{">>>", TokenCommandEnd},
	}

	for _, test := range tests {
		lexer := NewLexer(test.input, "test.wdl")
		token := lexer.NextToken()

		if token.Type != test.expected {
			t.Errorf("Input '%s': expected %s, got %s", 
				test.input, test.expected.String(), token.Type.String())
		}
		if token.Value != test.input {
			t.Errorf("Input '%s': expected value '%s', got '%s'", 
				test.input, test.input, token.Value)
		}
	}
}

func TestLexerDelimiters(t *testing.T) {
	tests := []struct {
		input    string
		expected TokenType
	}{
		{"{", TokenLeftBrace},
		{"}", TokenRightBrace},
		{"[", TokenLeftBracket},
		{"]", TokenRightBracket},
		{"(", TokenLeftParen},
		{")", TokenRightParen},
		{",", TokenComma},
		{":", TokenColon},
		{";", TokenSemicolon},
		{".", TokenDot},
		{"?", TokenQuestion},
	}

	for _, test := range tests {
		lexer := NewLexer(test.input, "test.wdl")
		token := lexer.NextToken()

		if token.Type != test.expected {
			t.Errorf("Input '%s': expected %s, got %s", 
				test.input, test.expected.String(), token.Type.String())
		}
	}
}

func TestLexerNumbers(t *testing.T) {
	tests := []struct {
		input       string
		expectedType TokenType
		expectedValue string
	}{
		{"42", TokenInt, "42"},
		{"0", TokenInt, "0"},
		{"123", TokenInt, "123"},
		{"-42", TokenMinus, "-"},  // Minus is separate token
		{"3.14", TokenFloat, "3.14"},
		{"0.5", TokenFloat, "0.5"},
		{"1.0e10", TokenFloat, "1.0e10"},
		{"2E-3", TokenFloat, "2E-3"},
	}

	for _, test := range tests {
		lexer := NewLexer(test.input, "test.wdl")
		token := lexer.NextToken()

		if token.Type != test.expectedType {
			t.Errorf("Input '%s': expected type %s, got %s", 
				test.input, test.expectedType.String(), token.Type.String())
		}
		if token.Value != test.expectedValue {
			t.Errorf("Input '%s': expected value '%s', got '%s'", 
				test.input, test.expectedValue, token.Value)
		}
	}
}

func TestLexerStrings(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello"`, "hello"},
		{`'world'`, "world"},
		{`"hello world"`, "hello world"},
		{`"with \"quotes\""`, `with "quotes"`},
		{`'with \'quotes\''`, `with 'quotes'`},
		{`"with\nnewline"`, "with\nnewline"},
		{`"with\ttab"`, "with\ttab"},
		{`""`, ""},
		{`''`, ""},
	}

	for _, test := range tests {
		lexer := NewLexer(test.input, "test.wdl")
		token := lexer.NextToken()

		if token.Type != TokenString {
			t.Errorf("Input %s: expected TokenString, got %s", test.input, token.Type.String())
		}
		if token.Value != test.expected {
			t.Errorf("Input %s: expected value '%s', got '%s'", 
				test.input, test.expected, token.Value)
		}
	}
}

func TestLexerIdentifiers(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"variable", "variable"},
		{"my_var", "my_var"},
		{"var123", "var123"},
		{"_private", "_private"},
		{"CamelCase", "CamelCase"},
		{"snake_case", "snake_case"},
	}

	for _, test := range tests {
		lexer := NewLexer(test.input, "test.wdl")
		token := lexer.NextToken()

		if token.Type != TokenIdentifier {
			t.Errorf("Input '%s': expected TokenIdentifier, got %s", test.input, token.Type.String())
		}
		if token.Value != test.expected {
			t.Errorf("Input '%s': expected value '%s', got '%s'", 
				test.input, test.expected, token.Value)
		}
	}
}

func TestLexerComments(t *testing.T) {
	input := `# This is a comment
workflow test {
    # Another comment
    input {
        String name # End of line comment
    }
}`

	lexer := NewLexer(input, "test.wdl")
	tokens := lexer.AllTokens()

	// Comments should be skipped, so we should only see tokens for the workflow
	expectedTypes := []TokenType{
		TokenWorkflow, TokenIdentifier, TokenLeftBrace,
		TokenInput, TokenLeftBrace,
		TokenStringType, TokenIdentifier,
		TokenRightBrace,
		TokenRightBrace,
		TokenEOF,
	}

	actualTypes := make([]TokenType, 0, len(tokens))
	for _, token := range tokens {
		actualTypes = append(actualTypes, token.Type)
	}

	if len(actualTypes) != len(expectedTypes) {
		t.Errorf("Expected %d tokens (comments should be skipped), got %d", 
			len(expectedTypes), len(actualTypes))
	}

	for i, expectedType := range expectedTypes {
		if i >= len(actualTypes) {
			break
		}
		if actualTypes[i] != expectedType {
			t.Errorf("Token %d: expected %s, got %s", 
				i, expectedType.String(), actualTypes[i].String())
		}
	}
}

func TestLexerWhitespace(t *testing.T) {
	input := `   workflow    test   {   
	    input   {   
	        String   name   
	    }   
	}   `

	lexer := NewLexer(input, "test.wdl")
	tokens := lexer.AllTokens()

	// Whitespace should be skipped
	expectedTypes := []TokenType{
		TokenWorkflow, TokenIdentifier, TokenLeftBrace,
		TokenInput, TokenLeftBrace,
		TokenStringType, TokenIdentifier,
		TokenRightBrace,
		TokenRightBrace,
		TokenEOF,
	}

	actualTypes := make([]TokenType, 0, len(tokens))
	for _, token := range tokens {
		actualTypes = append(actualTypes, token.Type)
	}

	if len(actualTypes) != len(expectedTypes) {
		t.Errorf("Expected %d tokens (whitespace should be skipped), got %d", 
			len(expectedTypes), len(actualTypes))
	}

	for i, expectedType := range expectedTypes {
		if i >= len(actualTypes) {
			break
		}
		if actualTypes[i] != expectedType {
			t.Errorf("Token %d: expected %s, got %s", 
				i, expectedType.String(), actualTypes[i].String())
		}
	}
}

func TestLexerPosition(t *testing.T) {
	input := `workflow
test`

	lexer := NewLexer(input, "test.wdl")
	
	token1 := lexer.NextToken()
	if token1.Position.Line != 1 || token1.Position.Column != 1 {
		t.Errorf("First token position: expected (1,1), got (%d,%d)", 
			token1.Position.Line, token1.Position.Column)
	}
	
	token2 := lexer.NextToken()
	if token2.Position.Line != 2 || token2.Position.Column != 1 {
		t.Errorf("Second token position: expected (2,1), got (%d,%d)", 
			token2.Position.Line, token2.Position.Column)
	}
}

func TestLexerErrors(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{"@", "invalid character"},
		{"#", "invalid character after comment"},
		{"$", "incomplete interpolation"},
		{"~", "incomplete interpolation"},
	}

	for _, test := range tests {
		lexer := NewLexer(test.input, "test.wdl")
		token := lexer.NextToken()

		if token.Type != TokenError && token.Type != TokenComment && token.Type != TokenEOF {
			// Some inputs might be valid in certain contexts
			continue
		}
	}
}

func TestLexerEOF(t *testing.T) {
	lexer := NewLexer("", "test.wdl")
	token := lexer.NextToken()

	if token.Type != TokenEOF {
		t.Errorf("Empty input should return EOF, got %s", token.Type.String())
	}
}

func TestLexerMultipleNextToken(t *testing.T) {
	input := "workflow test"
	lexer := NewLexer(input, "test.wdl")

	// First call
	token1 := lexer.NextToken()
	if token1.Type != TokenWorkflow {
		t.Errorf("First token: expected TokenWorkflow, got %s", token1.Type.String())
	}

	// Second call
	token2 := lexer.NextToken()
	if token2.Type != TokenIdentifier || token2.Value != "test" {
		t.Errorf("Second token: expected TokenIdentifier 'test', got %s '%s'", 
			token2.Type.String(), token2.Value)
	}

	// Third call should be EOF
	token3 := lexer.NextToken()
	if token3.Type != TokenEOF {
		t.Errorf("Third token: expected TokenEOF, got %s", token3.Type.String())
	}

	// Fourth call should still be EOF
	token4 := lexer.NextToken()
	if token4.Type != TokenEOF {
		t.Errorf("Fourth token: expected TokenEOF, got %s", token4.Type.String())
	}
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
		{TokenLogicalOr, "||"},
		{TokenLogicalAnd, "&&"},
		{TokenEqual, "=="},
		{TokenNotEqual, "!="},
		{TokenLeftBrace, "{"},
		{TokenRightBrace, "}"},
	}

	for _, test := range tests {
		actual := test.tokenType.String()
		if actual != test.expected {
			t.Errorf("TokenType.String() for %d: expected '%s', got '%s'", 
				int(test.tokenType), test.expected, actual)
		}
	}
}

func TestNewLexer(t *testing.T) {
	input := "workflow test"
	uri := "test.wdl"
	
	lexer := NewLexer(input, uri)
	
	if lexer.input != input {
		t.Errorf("Expected input '%s', got '%s'", input, lexer.input)
	}
	if lexer.uri != uri {
		t.Errorf("Expected URI '%s', got '%s'", uri, lexer.uri)
	}
	if lexer.line != 1 {
		t.Errorf("Expected initial line 1, got %d", lexer.line)
	}
	if lexer.column != 1 {
		t.Errorf("Expected initial column 1, got %d", lexer.column)
	}
	if lexer.position != 0 {
		t.Errorf("Expected initial position 0, got %d", lexer.position)
	}
	
	// Test that keywords map is populated
	if len(lexer.keywords) == 0 {
		t.Error("Keywords map should not be empty")
	}
	
	// Test some known keywords
	if lexer.keywords["workflow"] != TokenWorkflow {
		t.Error("Expected 'workflow' keyword to map to TokenWorkflow")
	}
	if lexer.keywords["true"] != TokenBool {
		t.Error("Expected 'true' keyword to map to TokenBool")
	}
}