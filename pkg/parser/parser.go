package parser

import (
	"fmt"
	"strings"

	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/tree"
)

// ParseError represents a WDL parsing error
type ParseError struct {
	*errors.SyntaxError
	Expected []TokenType
	Got      Token
}

func NewParseError(pos errors.SourcePosition, message string, expected []TokenType, got Token) *ParseError {
	return &ParseError{
		SyntaxError: errors.NewSyntaxError(pos, message, "1.0", nil),
		Expected:    expected,
		Got:         got,
	}
}

func (e *ParseError) Error() string {
	if len(e.Expected) > 0 {
		var expectedStrs []string
		for _, t := range e.Expected {
			expectedStrs = append(expectedStrs, t.String())
		}
		return fmt.Sprintf("expected %s, got %s at %s:%d:%d",
			strings.Join(expectedStrs, " or "),
			e.Got.Type.String(),
			e.Pos.URI,
			e.Pos.Line,
			e.Pos.Column)
	}
	return fmt.Sprintf("parse error: %s at %s:%d:%d",
		e.Message,
		e.Pos.URI,
		e.Pos.Line,
		e.Pos.Column)
}

// Parser represents a WDL parser
type Parser struct {
	lexer        *Lexer
	currentToken Token
	peekToken    Token
	errors       []*ParseError
	uri          string
}

// CurrentToken returns the current token
func (p *Parser) CurrentToken() Token {
	return p.currentToken
}

// NewParser creates a new WDL parser
func NewParser(input, uri string) *Parser {
	lexer := NewLexer(input, uri)
	parser := &Parser{
		lexer:  lexer,
		errors: make([]*ParseError, 0),
		uri:    uri,
	}

	// Read first two tokens
	parser.nextToken()
	parser.nextToken()

	return parser
}

// nextToken advances to the next token
// NextToken advances to the next token
func (p *Parser) NextToken() {
	p.currentToken = p.peekToken
	p.peekToken = p.lexer.NextToken()
}

// nextToken is an internal alias for NextToken
func (p *Parser) nextToken() {
	p.NextToken()
}

// Errors returns all parse errors encountered
func (p *Parser) Errors() []*ParseError {
	return p.errors
}

// HasErrors returns true if any errors were encountered
func (p *Parser) HasErrors() bool {
	return len(p.errors) > 0
}

// addError adds a parse error
func (p *Parser) addError(err *ParseError) {
	p.errors = append(p.errors, err)
}

// expectError creates an error for unexpected token
func (p *Parser) expectError(expected ...TokenType) *ParseError {
	return NewParseError(
		p.currentToken.Position,
		fmt.Sprintf("unexpected token %s", p.currentToken.Type.String()),
		expected,
		p.currentToken,
	)
}

// currentTokenIs returns true if current token matches expected type
func (p *Parser) currentTokenIs(t TokenType) bool {
	return p.currentToken.Type == t
}

// peekTokenIs returns true if peek token matches expected type
func (p *Parser) peekTokenIs(t TokenType) bool {
	return p.peekToken.Type == t
}

// match returns true and advances if current token matches expected type
func (p *Parser) match(expected TokenType) bool {
	if p.currentTokenIs(expected) {
		p.nextToken()
		return true
	}
	return false
}

// consume expects and consumes a token of the specified type
func (p *Parser) consume(expected TokenType) bool {
	if p.currentTokenIs(expected) {
		p.nextToken()
		return true
	}
	
	p.addError(p.expectError(expected))
	return false
}

// consumeAny expects and consumes any of the specified token types
func (p *Parser) consumeAny(expected ...TokenType) bool {
	for _, tokenType := range expected {
		if p.currentTokenIs(tokenType) {
			p.nextToken()
			return true
		}
	}
	
	p.addError(p.expectError(expected...))
	return false
}

// skipTo skips tokens until we find one of the expected types or EOF
func (p *Parser) skipTo(tokens ...TokenType) {
	for !p.currentTokenIs(TokenEOF) {
		for _, tokenType := range tokens {
			if p.currentTokenIs(tokenType) {
				return
			}
		}
		p.nextToken()
	}
}

// synchronize attempts to recover from parse errors by skipping to a safe point
func (p *Parser) synchronize() {
	p.nextToken()

	for !p.currentTokenIs(TokenEOF) {
		if p.currentTokenIs(TokenSemicolon) {
			p.nextToken()
			return
		}

		switch p.currentToken.Type {
		case TokenTask, TokenWorkflow, TokenStruct, TokenImport, TokenVersion:
			return
		}

		p.nextToken()
	}
}

// IsAtEnd returns true if at end of input
func (p *Parser) IsAtEnd() bool {
	return p.currentTokenIs(TokenEOF)
}

// currentPosition returns current token's position
func (p *Parser) currentPosition() errors.SourcePosition {
	return p.currentToken.Position
}

// parseIdentifier parses an identifier token
func (p *Parser) parseIdentifier() (string, bool) {
	if p.currentTokenIs(TokenIdentifier) {
		value := p.currentToken.Value
		p.nextToken()
		return value, true
	}
	
	p.addError(p.expectError(TokenIdentifier))
	return "", false
}

// parseNamespacedIdentifier parses a namespaced identifier (e.g., "lib.task")
func (p *Parser) parseNamespacedIdentifier() (string, bool) {
	parts := []string{}
	
	// First identifier
	if ident, ok := p.parseIdentifier(); ok {
		parts = append(parts, ident)
	} else {
		return "", false
	}
	
	// Additional parts separated by dots
	for p.currentTokenIs(TokenDot) {
		p.nextToken() // consume '.'
		
		if ident, ok := p.parseIdentifier(); ok {
			parts = append(parts, ident)
		} else {
			p.addError(p.expectError(TokenIdentifier))
			return "", false
		}
	}
	
	return strings.Join(parts, "."), true
}

// parseStringValue parses a string token value
func (p *Parser) parseStringValue() (string, bool) {
	if p.currentTokenIs(TokenString) {
		value := p.currentToken.Value
		p.nextToken()
		return value, true
	}
	
	p.addError(p.expectError(TokenString))
	return "", false
}

// skipNewlines skips any newline tokens
func (p *Parser) skipNewlines() {
	for p.currentTokenIs(TokenNewline) {
		p.nextToken()
	}
}

// skipCommentsAndNewlines skips comments and newlines
func (p *Parser) skipCommentsAndNewlines() {
	for p.currentTokenIs(TokenComment) || p.currentTokenIs(TokenNewline) {
		p.nextToken()
	}
}

// isKeyword returns true if token is a WDL keyword
func (p *Parser) isKeyword(token TokenType) bool {
	switch token {
	case TokenVersion, TokenImport, TokenAs, TokenAlias,
		 TokenWorkflow, TokenTask, TokenInput, TokenOutput,
		 TokenMeta, TokenParameterMeta, TokenRequirements, TokenRuntime,
		 TokenScatter, TokenIf, TokenThen, TokenElse,
		 TokenCall, TokenAfter, TokenStruct, TokenCommand, TokenEnv,
		 TokenLeft, TokenRight, TokenObject,
		 TokenArray, TokenFile, TokenDirectory, TokenMap, TokenPair,
		 TokenIntType, TokenFloatType, TokenStringType, TokenBoolType, TokenNone:
		return true
	default:
		return false
	}
}

// isTypeKeyword returns true if token is a WDL type keyword
func (p *Parser) isTypeKeyword(token TokenType) bool {
	switch token {
	case TokenArray, TokenFile, TokenDirectory, TokenMap, TokenPair,
		 TokenIntType, TokenFloatType, TokenStringType, TokenBoolType:
		return true
	default:
		return false
	}
}

// isLiteral returns true if token is a literal value
func (p *Parser) isLiteral(token TokenType) bool {
	switch token {
	case TokenInt, TokenFloat, TokenString, TokenBool, TokenNone:
		return true
	default:
		return false
	}
}

// isUnaryOperator returns true if token is a unary operator
func (p *Parser) isUnaryOperator(token TokenType) bool {
	switch token {
	case TokenNot, TokenMinus, TokenPlus:
		return true
	default:
		return false
	}
}

// isBinaryOperator returns true if token is a binary operator
func (p *Parser) isBinaryOperator(token TokenType) bool {
	switch token {
	case TokenLogicalOr, TokenLogicalAnd,
		 TokenEqual, TokenNotEqual, TokenLessEqual, TokenGreaterEqual, TokenLess, TokenGreater,
		 TokenPlus, TokenMinus, TokenMultiply, TokenDivide, TokenModulo:
		return true
	default:
		return false
	}
}

// getOperatorPrecedence returns the precedence of binary operators (higher number = higher precedence)
func (p *Parser) getOperatorPrecedence(token TokenType) int {
	switch token {
	case TokenLogicalOr:
		return 1
	case TokenLogicalAnd:
		return 2
	case TokenEqual, TokenNotEqual, TokenLessEqual, TokenGreaterEqual, TokenLess, TokenGreater:
		return 3
	case TokenPlus, TokenMinus:
		return 4
	case TokenMultiply, TokenDivide, TokenModulo:
		return 5
	default:
		return 0
	}
}

// recovery point detection
func (p *Parser) isRecoveryPoint() bool {
	switch p.currentToken.Type {
	case TokenTask, TokenWorkflow, TokenStruct, TokenImport, TokenVersion, TokenRightBrace:
		return true
	default:
		return false
	}
}

// ParseDocument is the main entry point for parsing a WDL document
func (p *Parser) ParseDocument() (*tree.Document, error) {
	p.skipCommentsAndNewlines()
	
	if p.IsAtEnd() {
		return nil, NewParseError(
			p.currentPosition(),
			"empty document",
			[]TokenType{TokenVersion, TokenImport, TokenTask, TokenWorkflow, TokenStruct},
			p.currentToken,
		)
	}
	
	doc, ok := p.parseDocument()
	if !ok {
		// Return first error if parsing failed
		if len(p.errors) > 0 {
			return nil, p.errors[0]
		}
		return nil, NewParseError(
			p.currentPosition(),
			"failed to parse document",
			[]TokenType{},
			p.currentToken,
		)
	}
	
	// Validate document structure
	if !p.validateDocumentStructure(doc) {
		if len(p.errors) > 0 {
			return nil, p.errors[len(p.errors)-1]
		}
		return nil, NewParseError(
			doc.SourcePosition(),
			"invalid document structure",
			[]TokenType{},
			Token{Type: TokenEOF},
		)
	}
	
	return doc, nil
}

// Debug helper to print current parser state
func (p *Parser) debugState() string {
	return fmt.Sprintf("Current: %s(%s), Peek: %s(%s)", 
		p.currentToken.Type.String(), p.currentToken.Value,
		p.peekToken.Type.String(), p.peekToken.Value)
}