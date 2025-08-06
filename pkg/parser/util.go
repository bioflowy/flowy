package parser

import (
	"strconv"
	"strings"
)

// parseOptional parses an optional element using the provided parse function
func (p *Parser) parseOptional(parseFunc func() (interface{}, bool)) interface{} {
	if result, ok := parseFunc(); ok {
		return result
	}
	return nil
}

// parseList parses a comma-separated list of elements
func (p *Parser) parseList(
	terminator TokenType,
	parseElement func() (interface{}, bool),
	allowTrailingComma bool,
) ([]interface{}, bool) {
	var elements []interface{}

	// Handle empty list
	if p.currentTokenIs(terminator) {
		return elements, true
	}

	// Parse first element
	if elem, ok := parseElement(); ok {
		elements = append(elements, elem)
	} else {
		return nil, false
	}

	// Parse remaining elements
	for p.currentTokenIs(TokenComma) {
		p.nextToken() // consume comma

		// Check for trailing comma
		if p.currentTokenIs(terminator) {
			if allowTrailingComma {
				break
			} else {
				p.addError(p.expectError(terminator))
				return nil, false
			}
		}

		if elem, ok := parseElement(); ok {
			elements = append(elements, elem)
		} else {
			return nil, false
		}
	}

	return elements, true
}

// parseDelimitedList parses a list enclosed in delimiters
func (p *Parser) parseDelimitedList(
	openDelim, closeDelim TokenType,
	parseElement func() (interface{}, bool),
	allowTrailingComma bool,
) ([]interface{}, bool) {
	if !p.consume(openDelim) {
		return nil, false
	}

	elements, ok := p.parseList(closeDelim, parseElement, allowTrailingComma)
	if !ok {
		return nil, false
	}

	if !p.consume(closeDelim) {
		return nil, false
	}

	return elements, true
}

// parseStringList parses a comma-separated list of strings
func (p *Parser) parseStringList(
	terminator TokenType,
	allowTrailingComma bool,
) ([]string, bool) {
	elements, ok := p.parseList(terminator, func() (interface{}, bool) {
		if str, ok := p.parseStringValue(); ok {
			return str, true
		}
		return nil, false
	}, allowTrailingComma)

	if !ok {
		return nil, false
	}

	// Convert interface{} slice to string slice
	var strings []string
	for _, elem := range elements {
		strings = append(strings, elem.(string))
	}

	return strings, true
}

// parseIdentifierList parses a comma-separated list of identifiers
func (p *Parser) parseIdentifierList(
	terminator TokenType,
	allowTrailingComma bool,
) ([]string, bool) {
	elements, ok := p.parseList(terminator, func() (interface{}, bool) {
		if ident, ok := p.parseIdentifier(); ok {
			return ident, true
		}
		return nil, false
	}, allowTrailingComma)

	if !ok {
		return nil, false
	}

	// Convert interface{} slice to string slice
	var identifiers []string
	for _, elem := range elements {
		identifiers = append(identifiers, elem.(string))
	}

	return identifiers, true
}

// parseInt parses an integer token
func (p *Parser) parseInt() (int64, bool) {
	if !p.currentTokenIs(TokenInt) {
		p.addError(p.expectError(TokenInt))
		return 0, false
	}

	value, err := strconv.ParseInt(p.currentToken.Value, 10, 64)
	if err != nil {
		p.addError(NewParseError(
			p.currentToken.Position,
			"invalid integer literal: "+p.currentToken.Value,
			[]TokenType{TokenInt},
			p.currentToken,
		))
		return 0, false
	}

	p.nextToken()
	return value, true
}

// parseFloat parses a float token
func (p *Parser) parseFloat() (float64, bool) {
	if !p.currentTokenIs(TokenFloat) {
		p.addError(p.expectError(TokenFloat))
		return 0, false
	}

	value, err := strconv.ParseFloat(p.currentToken.Value, 64)
	if err != nil {
		p.addError(NewParseError(
			p.currentToken.Position,
			"invalid float literal: "+p.currentToken.Value,
			[]TokenType{TokenFloat},
			p.currentToken,
		))
		return 0, false
	}

	p.nextToken()
	return value, true
}

// parseNumber parses either an int or float token
func (p *Parser) parseNumber() (interface{}, TokenType, bool) {
	if p.currentTokenIs(TokenInt) {
		if value, ok := p.parseInt(); ok {
			return value, TokenInt, true
		}
	} else if p.currentTokenIs(TokenFloat) {
		if value, ok := p.parseFloat(); ok {
			return value, TokenFloat, true
		}
	} else {
		p.addError(p.expectError(TokenInt, TokenFloat))
	}
	return nil, TokenError, false
}

// parseBool parses a boolean token
func (p *Parser) parseBool() (bool, bool) {
	if !p.currentTokenIs(TokenBool) {
		p.addError(p.expectError(TokenBool))
		return false, false
	}

	value := p.currentToken.Value == "true"
	p.nextToken()
	return value, true
}

// parseBlock parses a block enclosed in braces
func (p *Parser) parseBlock(parseContent func() bool) bool {
	if !p.consume(TokenLeftBrace) {
		return false
	}

	p.skipCommentsAndNewlines()

	if parseContent != nil && !parseContent() {
		// Try to recover by finding the closing brace
		p.skipTo(TokenRightBrace)
	}

	p.skipCommentsAndNewlines()

	if !p.consume(TokenRightBrace) {
		return false
	}

	return true
}

// parseParenthesized parses content enclosed in parentheses
func (p *Parser) parseParenthesized(parseContent func() bool) bool {
	if !p.consume(TokenLeftParen) {
		return false
	}

	p.skipCommentsAndNewlines()

	if parseContent != nil && !parseContent() {
		// Try to recover by finding the closing paren
		p.skipTo(TokenRightParen)
	}

	p.skipCommentsAndNewlines()

	if !p.consume(TokenRightParen) {
		return false
	}

	return true
}

// parseBracketed parses content enclosed in square brackets
func (p *Parser) parseBracketed(parseContent func() bool) bool {
	if !p.consume(TokenLeftBracket) {
		return false
	}

	p.skipCommentsAndNewlines()

	if parseContent != nil && !parseContent() {
		// Try to recover by finding the closing bracket
		p.skipTo(TokenRightBracket)
	}

	p.skipCommentsAndNewlines()

	if !p.consume(TokenRightBracket) {
		return false
	}

	return true
}

// isQuantifier returns true if token is a type quantifier
func (p *Parser) isQuantifier(token TokenType) bool {
	switch token {
	case TokenQuestion, TokenPlusQuantifier:
		return true
	default:
		return false
	}
}

// parseQuantifiers parses optional type quantifiers (? and +)
func (p *Parser) parseQuantifiers() (optional bool, nonempty bool) {
	for {
		if p.currentTokenIs(TokenQuestion) {
			optional = true
			p.nextToken()
		} else if p.currentTokenIs(TokenPlusQuantifier) {
			nonempty = true
			p.nextToken()
		} else {
			break
		}
	}
	return
}

// expectOneOf expects one of the specified tokens and consumes it
func (p *Parser) expectOneOf(tokens ...TokenType) (TokenType, bool) {
	for _, token := range tokens {
		if p.currentTokenIs(token) {
			p.nextToken()
			return token, true
		}
	}
	
	p.addError(p.expectError(tokens...))
	return TokenError, false
}

// isBlockStart returns true if current token can start a block
func (p *Parser) isBlockStart() bool {
	switch p.currentToken.Type {
	case TokenLeftBrace, TokenTask, TokenWorkflow, TokenStruct,
		 TokenInput, TokenOutput, TokenMeta, TokenParameterMeta,
		 TokenRequirements, TokenRuntime, TokenCommand:
		return true
	default:
		return false
	}
}

// isStatementStart returns true if current token can start a statement
func (p *Parser) isStatementStart() bool {
	switch p.currentToken.Type {
	case TokenIdentifier:
		return true
	default:
		return p.isTypeKeyword(p.currentToken.Type) || p.isBlockStart()
	}
}

// isExpressionStart returns true if current token can start an expression
func (p *Parser) isExpressionStart() bool {
	switch p.currentToken.Type {
	case TokenIdentifier, TokenLeftParen, TokenLeftBracket,
		 TokenNot, TokenMinus, TokenPlus, TokenIf:
		return true
	default:
		return p.isLiteral(p.currentToken.Type)
	}
}

// skipUntilRecovery skips tokens until we reach a recovery point
func (p *Parser) skipUntilRecovery() {
	depth := 0
	
	for !p.isAtEnd() {
		switch p.currentToken.Type {
		case TokenLeftBrace:
			depth++
		case TokenRightBrace:
			if depth > 0 {
				depth--
			} else {
				return // Found recovery point
			}
		case TokenTask, TokenWorkflow, TokenStruct, TokenImport, TokenVersion:
			if depth == 0 {
				return // Found recovery point at top level
			}
		case TokenSemicolon:
			if depth == 0 {
				p.nextToken() // consume semicolon
				return
			}
		}
		p.nextToken()
	}
}

// joinIdentifiers joins a list of identifiers with dots
func joinIdentifiers(identifiers []string) string {
	return strings.Join(identifiers, ".")
}

// splitNamespace splits a namespaced identifier into parts
func splitNamespace(namespaced string) []string {
	return strings.Split(namespaced, ".")
}

// isValidIdentifier checks if a string is a valid WDL identifier
func isValidIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	
	// First character must be letter or underscore
	first := rune(s[0])
	if !(first >= 'a' && first <= 'z') && !(first >= 'A' && first <= 'Z') && first != '_' {
		return false
	}
	
	// Rest can be letters, digits, or underscores
	for _, ch := range s[1:] {
		if !(ch >= 'a' && ch <= 'z') && !(ch >= 'A' && ch <= 'Z') && 
		   !(ch >= '0' && ch <= '9') && ch != '_' {
			return false
		}
	}
	
	return true
}

// precedenceOf returns the precedence level for an operator token
func precedenceOf(tokenType TokenType) int {
	switch tokenType {
	case TokenLogicalOr:
		return 1
	case TokenLogicalAnd:
		return 2
	case TokenEqual, TokenNotEqual, TokenLess, TokenGreater, 
		 TokenLessEqual, TokenGreaterEqual:
		return 3
	case TokenPlus, TokenMinus:
		return 4
	case TokenMultiply, TokenDivide, TokenModulo:
		return 5
	default:
		return 0
	}
}

// isLeftAssociative returns true if operator is left-associative
func isLeftAssociative(tokenType TokenType) bool {
	// All WDL binary operators are left-associative
	switch tokenType {
	case TokenLogicalOr, TokenLogicalAnd,
		 TokenEqual, TokenNotEqual, TokenLess, TokenGreater, 
		 TokenLessEqual, TokenGreaterEqual,
		 TokenPlus, TokenMinus, TokenMultiply, TokenDivide, TokenModulo:
		return true
	default:
		return false
	}
}