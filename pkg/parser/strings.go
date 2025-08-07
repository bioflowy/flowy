package parser

import (
	"strings"

	"github.com/bioflowy/flowy/pkg/expr"
)

// parseString parses a WDL string according to the grammar:
// ?string: string1 | string2 | multistring
func (p *Parser) parseString() (expr.Expr, bool) {
	switch p.currentToken.Type {
	case TokenString:
		// Simple string literal (no interpolation detected by lexer)
		return p.parseStringLiteral()
	case TokenMultiStringStart:
		return p.parseMultiString()
	default:
		// Check if this might be a string with interpolation
		return p.parseInterpolatedString()
	}
}

// parseInterpolatedString parses a string with interpolation
// This is a simplified implementation - full string interpolation parsing
// would require more sophisticated lexer support
func (p *Parser) parseInterpolatedString() (expr.Expr, bool) {
	pos := p.currentPosition()
	
	// For now, if we see a string token, just parse it as a literal
	if p.currentTokenIs(TokenString) {
		return p.parseStringLiteral()
	}
	
	// TODO: Implement full interpolated string parsing when lexer supports it
	p.addError(NewParseError(
		pos,
		"interpolated string parsing not yet fully implemented",
		[]TokenType{TokenString},
		p.currentToken,
	))
	return nil, false
}

// parseMultiString parses a multi-line string:
// multistring: /<<</ (COMMAND2_FRAGMENT? "~{" placeholder "}")* COMMAND2_FRAGMENT? />>>/ -> string
func (p *Parser) parseMultiString() (expr.Expr, bool) {
	pos := p.currentPosition()
	
	if !p.currentTokenIs(TokenMultiStringStart) {
		p.addError(p.expectError(TokenMultiStringStart))
		return nil, false
	}
	p.nextToken() // consume <<<
	
	// For now, parse the content as string fragments until we see >>>
	var parts []string
	var interpolations []expr.Expr
	
	for !p.currentTokenIs(TokenMultiStringEnd) && !p.isAtEnd() {
		switch p.currentToken.Type {
		case TokenStringFragment:
			parts = append(parts, p.currentToken.Value)
			p.nextToken()
		case TokenCommandFragment:
			parts = append(parts, p.currentToken.Value)
			p.nextToken()
		case TokenInterpolationStart:
			// Parse interpolation
			if placeholder, ok := p.parsePlaceholder(); ok {
				interpolations = append(interpolations, placeholder)
				parts = append(parts, "") // Empty string as placeholder
			} else {
				return nil, false
			}
		default:
			p.addError(p.expectError(TokenMultiStringEnd, TokenStringFragment, TokenInterpolationStart))
			return nil, false
		}
	}
	
	if !p.consume(TokenMultiStringEnd) {
		return nil, false
	}
	
	// Combine all parts into a single string value for now
	combined := strings.Join(parts, "")
	return expr.NewMultilineString(combined, interpolations, pos), true
}

// parseCommand parses a command string:
// ?command: "command" (command1 | command2)
func (p *Parser) parseCommand() (expr.Expr, bool) {
	// pos := p.currentPosition()  // unused
	
	if !p.consume(TokenCommand) {
		return nil, false
	}
	
	switch p.currentToken.Type {
	case TokenLeftBrace:
		return p.parseCommand1()
	case TokenCommandStart:
		return p.parseCommand2()
	default:
		p.addError(p.expectError(TokenLeftBrace, TokenCommandStart))
		return nil, false
	}
}

// parseCommand1 parses command style 1:
// command1: "{" (COMMAND1_FRAGMENT? _EITHER_DELIM placeholder "}")* COMMAND1_FRAGMENT? "}" -> command
func (p *Parser) parseCommand1() (expr.Expr, bool) {
	pos := p.currentPosition()
	
	if !p.consume(TokenLeftBrace) {
		return nil, false
	}
	
	var parts []string
	var interpolations []expr.Expr
	
	for !p.currentTokenIs(TokenRightBrace) && !p.isAtEnd() {
		switch p.currentToken.Type {
		case TokenCommandFragment:
			parts = append(parts, p.currentToken.Value)
			p.nextToken()
		case TokenInterpolationStart:
			if placeholder, ok := p.parsePlaceholder(); ok {
				interpolations = append(interpolations, placeholder)
				parts = append(parts, "") // Empty string as placeholder
			} else {
				return nil, false
			}
		default:
			p.addError(p.expectError(TokenRightBrace, TokenCommandFragment, TokenInterpolationStart))
			return nil, false
		}
	}
	
	if !p.consume(TokenRightBrace) {
		return nil, false
	}
	
	combined := strings.Join(parts, "")
	return expr.NewTaskCommand(combined, interpolations, pos), true
}

// parseCommand2 parses command style 2:
// command2: "<<<" (COMMAND2_FRAGMENT? "~{" placeholder "}")* COMMAND2_FRAGMENT? ">>>" -> command
func (p *Parser) parseCommand2() (expr.Expr, bool) {
	pos := p.currentPosition()
	
	if !p.consume(TokenCommandStart) {
		return nil, false
	}
	
	var parts []string
	var interpolations []expr.Expr
	
	for !p.currentTokenIs(TokenCommandEnd) && !p.isAtEnd() {
		switch p.currentToken.Type {
		case TokenCommandFragment:
			parts = append(parts, p.currentToken.Value)
			p.nextToken()
		case TokenInterpolationStart:
			// Only ~{ is allowed in command2 style, but lexer doesn't distinguish
			if placeholder, ok := p.parsePlaceholder(); ok {
				interpolations = append(interpolations, placeholder)
				parts = append(parts, "") // Empty string as placeholder
			} else {
				return nil, false
			}
		default:
			p.addError(p.expectError(TokenCommandEnd, TokenCommandFragment, TokenInterpolationStart))
			return nil, false
		}
	}
	
	if !p.consume(TokenCommandEnd) {
		return nil, false
	}
	
	combined := strings.Join(parts, "")
	return expr.NewTaskCommand(combined, interpolations, pos), true
}

// parsePlaceholder parses a placeholder:
// placeholder: placeholder_option* expr
func (p *Parser) parsePlaceholder() (expr.Expr, bool) {
	pos := p.currentPosition()
	
	if !p.consume(TokenInterpolationStart) {
		return nil, false
	}
	
	// Parse placeholder options
	options := make(map[string]string)
	for p.currentTokenIs(TokenIdentifier) && p.peekTokenIs(TokenAssign) {
		optName := p.currentToken.Value
		p.nextToken() // consume option name
		p.nextToken() // consume =
		
		// Parse option value
		var optValue string
		if p.currentTokenIs(TokenString) {
			optValue = p.currentToken.Value
			p.nextToken()
		} else if p.currentTokenIs(TokenInt) {
			optValue = p.currentToken.Value
			p.nextToken()
		} else if p.currentTokenIs(TokenFloat) {
			optValue = p.currentToken.Value
			p.nextToken()
		} else {
			p.addError(p.expectError(TokenString, TokenInt, TokenFloat))
			return nil, false
		}
		
		options[optName] = optValue
	}
	
	// Parse the expression inside the placeholder
	expression, ok := p.parseExpression()
	if !ok {
		return nil, false
	}
	
	if !p.consume(TokenRightBrace) {
		return nil, false
	}
	
	// Convert options map to PlaceholderOptions
	var placeholderOpts *expr.PlaceholderOptions
	if len(options) > 0 {
		placeholderOpts = &expr.PlaceholderOptions{}
		// TODO: Properly convert options map to PlaceholderOptions struct
	}
	
	return expr.NewPlaceholder(expression, placeholderOpts, pos), true
}

// parseStringWithInterpolation parses a string that may contain interpolation
// This is a placeholder for future full implementation
func (p *Parser) parseStringWithInterpolation() (expr.Expr, bool) {
	pos := p.currentPosition()
	
	// For now, just parse as a regular string literal
	if p.currentTokenIs(TokenString) {
		value := p.currentToken.Value
		p.nextToken()
		
		// Check if the string contains interpolation markers
		if strings.Contains(value, "${") || strings.Contains(value, "~{") {
			// This should be parsed as an interpolated string, but our
			// current lexer doesn't handle this properly
			// For now, return as a simple string
			return expr.NewStringLiteral(value, pos), true
		}
		
		return expr.NewStringLiteral(value, pos), true
	}
	
	p.addError(p.expectError(TokenString))
	return nil, false
}

// isStringStart returns true if current token can start a string
func (p *Parser) isStringStart() bool {
	switch p.currentToken.Type {
	case TokenString, TokenMultiStringStart, TokenCommandStart:
		return true
	default:
		return false
	}
}

// parseStringFragment parses a string fragment (part of an interpolated string)
func (p *Parser) parseStringFragment() (string, bool) {
	if p.currentTokenIs(TokenStringFragment) {
		value := p.currentToken.Value
		p.nextToken()
		return value, true
	}
	
	if p.currentTokenIs(TokenCommandFragment) {
		value := p.currentToken.Value
		p.nextToken()
		return value, true
	}
	
	p.addError(p.expectError(TokenStringFragment, TokenCommandFragment))
	return "", false
}

// parseInterpolationExpression parses the expression inside ${ } or ~{ }
func (p *Parser) parseInterpolationExpression() (expr.Expr, bool) {
	// This is just a regular expression
	return p.parseExpression()
}

// validateStringOptions validates placeholder options
func (p *Parser) validateStringOptions(options map[string]string) bool {
	// Common WDL placeholder options
	validOptions := map[string]bool{
		"sep":     true,  // separator for arrays
		"true":    true,  // value when boolean is true
		"false":   true,  // value when boolean is false
		"default": true,  // default value
	}
	
	for optName := range options {
		if !validOptions[optName] {
			return false
		}
	}
	
	return true
}

// isInterpolationStart returns true if current position starts an interpolation
func (p *Parser) isInterpolationStart() bool {
	return p.currentTokenIs(TokenInterpolationStart)
}

// parseCommandBlock parses either style of command block
func (p *Parser) parseCommandBlock() (expr.Expr, bool) {
	switch p.currentToken.Type {
	case TokenLeftBrace:
		return p.parseCommand1()
	case TokenCommandStart:
		return p.parseCommand2()
	default:
		p.addError(p.expectError(TokenLeftBrace, TokenCommandStart))
		return nil, false
	}
}