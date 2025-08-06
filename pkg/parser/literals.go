package parser

import (
	"github.com/bioflowy/flowy/pkg/expr"
)

// parseLiteral parses a literal value according to WDL grammar:
// ?literal: "true"-> boolean_true
//         | "false" -> boolean_false
//         | "None" -> null
//         | INT -> int
//         | SIGNED_INT -> int
//         | FLOAT -> float
//         | SIGNED_FLOAT -> float
func (p *Parser) parseLiteral() (expr.Expr, bool) {
	pos := p.currentPosition()

	switch p.currentToken.Type {
	case TokenBool:
		return p.parseBooleanLiteral()
	case TokenNull:
		return p.parseNullLiteral()
	case TokenInt:
		return p.parseIntLiteral()
	case TokenFloat:
		return p.parseFloatLiteral()
	case TokenString:
		return p.parseStringLiteral()
	default:
		p.addError(NewParseError(
			pos,
			"expected literal value",
			[]TokenType{TokenBool, TokenNull, TokenInt, TokenFloat, TokenString},
			p.currentToken,
		))
		return nil, false
	}
}

// parseBooleanLiteral parses a boolean literal (true or false)
func (p *Parser) parseBooleanLiteral() (expr.Expr, bool) {
	if !p.currentTokenIs(TokenBool) {
		p.addError(p.expectError(TokenBool))
		return nil, false
	}

	pos := p.currentPosition()
	value := p.currentToken.Value == "true"
	p.nextToken()

	return expr.NewBooleanLiteral(value, pos), true
}

// parseNullLiteral parses the None literal
func (p *Parser) parseNullLiteral() (expr.Expr, bool) {
	if !p.currentTokenIs(TokenNull) {
		p.addError(p.expectError(TokenNull))
		return nil, false
	}

	pos := p.currentPosition()
	p.nextToken()

	return expr.NewNullLiteral(pos), true
}

// parseIntLiteral parses an integer literal
func (p *Parser) parseIntLiteral() (expr.Expr, bool) {
	if !p.currentTokenIs(TokenInt) {
		p.addError(p.expectError(TokenInt))
		return nil, false
	}

	pos := p.currentPosition()
	value, ok := p.parseInt()
	if !ok {
		return nil, false
	}

	return expr.NewIntLiteral(value, pos), true
}

// parseFloatLiteral parses a float literal
func (p *Parser) parseFloatLiteral() (expr.Expr, bool) {
	if !p.currentTokenIs(TokenFloat) {
		p.addError(p.expectError(TokenFloat))
		return nil, false
	}

	pos := p.currentPosition()
	value, ok := p.parseFloat()
	if !ok {
		return nil, false
	}

	return expr.NewFloatLiteral(value, pos), true
}

// parseStringLiteral parses a string literal (without interpolation)
// This corresponds to: string_literal: ESCAPED_STRING | ESCAPED_STRING1
func (p *Parser) parseStringLiteral() (expr.Expr, bool) {
	if !p.currentTokenIs(TokenString) {
		p.addError(p.expectError(TokenString))
		return nil, false
	}

	pos := p.currentPosition()
	value := p.currentToken.Value
	p.nextToken()

	return expr.NewStringLiteral(value, pos), true
}

// parseNumberLiteral parses either an integer or float literal
func (p *Parser) parseNumberLiteral() (expr.Expr, bool) {
	switch p.currentToken.Type {
	case TokenInt:
		return p.parseIntLiteral()
	case TokenFloat:
		return p.parseFloatLiteral()
	default:
		p.addError(p.expectError(TokenInt, TokenFloat))
		return nil, false
	}
}

// isLiteralToken returns true if the current token is a literal
func (p *Parser) isLiteralToken() bool {
	return p.isLiteral(p.currentToken.Type)
}

// parsePrimitiveLiteral parses primitive literals (excluding strings with interpolation)
func (p *Parser) parsePrimitiveLiteral() (expr.Expr, bool) {
	pos := p.currentPosition()

	switch p.currentToken.Type {
	case TokenBool:
		return p.parseBooleanLiteral()
	case TokenNull:
		return p.parseNullLiteral()
	case TokenInt:
		return p.parseIntLiteral()
	case TokenFloat:
		return p.parseFloatLiteral()
	default:
		p.addError(NewParseError(
			pos,
			"expected primitive literal",
			[]TokenType{TokenBool, TokenNull, TokenInt, TokenFloat},
			p.currentToken,
		))
		return nil, false
	}
}

// parseSignedNumber parses a number that may have a leading sign
// This handles SIGNED_INT and SIGNED_FLOAT from the grammar
func (p *Parser) parseSignedNumber() (expr.Expr, bool) {
	pos := p.currentPosition()
	negative := false

	// Handle optional sign
	if p.currentTokenIs(TokenMinus) {
		negative = true
		p.nextToken()
	} else if p.currentTokenIs(TokenPlus) {
		// Positive sign, just consume it
		p.nextToken()
	}

	// Parse the number
	switch p.currentToken.Type {
	case TokenInt:
		value, ok := p.parseInt()
		if !ok {
			return nil, false
		}
		if negative {
			value = -value
		}
		return expr.NewIntLiteral(value, pos), true

	case TokenFloat:
		value, ok := p.parseFloat()
		if !ok {
			return nil, false
		}
		if negative {
			value = -value
		}
		return expr.NewFloatLiteral(value, pos), true

	default:
		p.addError(p.expectError(TokenInt, TokenFloat))
		return nil, false
	}
}

// parseAnyLiteral parses any literal value, including signed numbers
func (p *Parser) parseAnyLiteral() (expr.Expr, bool) {
	switch p.currentToken.Type {
	case TokenMinus, TokenPlus:
		// Could be a signed number
		return p.parseSignedNumber()
	case TokenBool:
		return p.parseBooleanLiteral()
	case TokenNull:
		return p.parseNullLiteral()
	case TokenInt:
		return p.parseIntLiteral()
	case TokenFloat:
		return p.parseFloatLiteral()
	case TokenString:
		return p.parseStringLiteral()
	default:
		pos := p.currentPosition()
		p.addError(NewParseError(
			pos,
			"expected literal value",
			[]TokenType{TokenBool, TokenNull, TokenInt, TokenFloat, TokenString, TokenMinus, TokenPlus},
			p.currentToken,
		))
		return nil, false
	}
}

// isNumericLiteral returns true if current token is a numeric literal
func (p *Parser) isNumericLiteral() bool {
	return p.currentTokenIs(TokenInt) || p.currentTokenIs(TokenFloat)
}

// isBooleanLiteral returns true if current token is a boolean literal
func (p *Parser) isBooleanLiteral() bool {
	return p.currentTokenIs(TokenBool)
}

// isNullLiteral returns true if current token is the null literal
func (p *Parser) isNullLiteral() bool {
	return p.currentTokenIs(TokenNull)
}

// isStringLiteral returns true if current token is a string literal
func (p *Parser) isStringLiteral() bool {
	return p.currentTokenIs(TokenString)
}

// parseConstantExpression parses a literal that can be used as a constant
// Used in contexts where only compile-time constants are allowed
func (p *Parser) parseConstantExpression() (expr.Expr, bool) {
	// In WDL, constants are just literals
	return p.parseAnyLiteral()
}