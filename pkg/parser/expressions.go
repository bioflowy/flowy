package parser

import (
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/expr"
)

// parseExpression parses a WDL expression according to the grammar:
// ?expr: expr_infix
func (p *Parser) parseExpression() (expr.Expr, bool) {
	return p.parseExprInfix()
}

// parseExprInfix parses infix expressions:
// ?expr_infix: expr_infix0
func (p *Parser) parseExprInfix() (expr.Expr, bool) {
	return p.parseExprInfix0()
}

// parseExprInfix0 parses logical OR expressions (lowest precedence):
// ?expr_infix0: expr_infix0 "||" expr_infix1 -> lor
//             | expr_infix1
func (p *Parser) parseExprInfix0() (expr.Expr, bool) {
	left, ok := p.parseExprInfix1()
	if !ok {
		return nil, false
	}

	for p.currentTokenIs(TokenLogicalOr) {
		pos := p.currentPosition()
		operator := "||"
		p.nextToken() // consume ||

		right, ok := p.parseExprInfix1()
		if !ok {
			return nil, false
		}

		left = expr.NewBinaryOp(left, operator, right, pos)
	}

	return left, true
}

// parseExprInfix1 parses logical AND expressions:
// ?expr_infix1: expr_infix1 "&&" expr_infix2 -> land
//             | expr_infix2
func (p *Parser) parseExprInfix1() (expr.Expr, bool) {
	left, ok := p.parseExprInfix2()
	if !ok {
		return nil, false
	}

	for p.currentTokenIs(TokenLogicalAnd) {
		pos := p.currentPosition()
		operator := "&&"
		p.nextToken() // consume &&

		right, ok := p.parseExprInfix2()
		if !ok {
			return nil, false
		}

		left = expr.NewBinaryOp(left, operator, right, pos)
	}

	return left, true
}

// parseExprInfix2 parses comparison expressions:
// ?expr_infix2: expr_infix2 "==" expr_infix3 -> eqeq
//             | expr_infix2 "!=" expr_infix3 -> neq
//             | expr_infix2 "<=" expr_infix3 -> lte
//             | expr_infix2 ">=" expr_infix3 -> gte
//             | expr_infix2 "<" expr_infix3 -> lt
//             | expr_infix2 ">" expr_infix3 -> gt
//             | expr_infix3
func (p *Parser) parseExprInfix2() (expr.Expr, bool) {
	left, ok := p.parseExprInfix3()
	if !ok {
		return nil, false
	}

	for p.isComparisonOperator() {
		pos := p.currentPosition()
		operator := p.currentToken.Value
		p.nextToken() // consume operator

		right, ok := p.parseExprInfix3()
		if !ok {
			return nil, false
		}

		left = expr.NewBinaryOp(left, operator, right, pos)
	}

	return left, true
}

// parseExprInfix3 parses addition/subtraction expressions:
// ?expr_infix3: expr_infix3 "+" expr_infix4 -> add
//             | expr_infix3 "-" expr_infix4 -> sub
//             | expr_infix4
func (p *Parser) parseExprInfix3() (expr.Expr, bool) {
	left, ok := p.parseExprInfix4()
	if !ok {
		return nil, false
	}

	for p.currentTokenIs(TokenPlus) || p.currentTokenIs(TokenMinus) {
		pos := p.currentPosition()
		operator := p.currentToken.Value
		p.nextToken() // consume operator

		right, ok := p.parseExprInfix4()
		if !ok {
			return nil, false
		}

		left = expr.NewBinaryOp(left, operator, right, pos)
	}

	return left, true
}

// parseExprInfix4 parses multiplication/division/modulo expressions (highest precedence):
// ?expr_infix4: expr_infix4 "*" expr_infix5 -> mul
//             | expr_infix4 "/" expr_infix5 -> div
//             | expr_infix4 "%" expr_infix5 -> rem
//             | expr_infix5
func (p *Parser) parseExprInfix4() (expr.Expr, bool) {
	left, ok := p.parseExprInfix5()
	if !ok {
		return nil, false
	}

	for p.currentTokenIs(TokenMultiply) || p.currentTokenIs(TokenDivide) || p.currentTokenIs(TokenModulo) {
		pos := p.currentPosition()
		operator := p.currentToken.Value
		p.nextToken() // consume operator

		right, ok := p.parseExprInfix5()
		if !ok {
			return nil, false
		}

		left = expr.NewBinaryOp(left, operator, right, pos)
	}

	return left, true
}

// parseExprInfix5 delegates to expr_core:
// ?expr_infix5: expr_core
func (p *Parser) parseExprInfix5() (expr.Expr, bool) {
	return p.parseExprCore()
}

// parseExprCore parses core expressions:
// ?expr_core: "(" expr ")"
//           | literal
//           | string
//           | "!" expr_core -> negate
//           | "[" [expr ("," expr)*] ","? "]" -> array
//           | expr_core "[" expr "]" -> at
//           | "(" expr "," expr ")" -> pair
//           | "{" [map_kv ("," map_kv)*] ","? "}" -> map
//           | "if" expr "then" expr "else" expr -> ifthenelse
//           | CNAME "(" [expr ("," expr)*] ")" -> apply
//           | CNAME "{" [object_kv ("," object_kv)* ","?] "}" -> obj
//           | CNAME -> left_name
//           | expr_core "." CNAME -> get_name
func (p *Parser) parseExprCore() (expr.Expr, bool) {
	pos := p.currentPosition()

	switch p.currentToken.Type {
	case TokenLeftParen:
		return p.parseParenthesizedExpression()
	
	case TokenNot:
		return p.parseUnaryExpression()
		
	case TokenLeftBracket:
		return p.parseArrayLiteral()
		
	case TokenLeftBrace:
		return p.parseMapLiteral()
		
	case TokenIf:
		return p.parseIfThenElse()
		
	case TokenIdentifier:
		return p.parseIdentifierExpression()
		
	default:
		// Try to parse as literal
		if p.isLiteralToken() {
			return p.parseAnyLiteral()
		}
		
		// Try to parse as string
		if p.isStringStart() {
			return p.parseString()
		}
		
		// Try to parse keywords as identifiers in expression context
		if p.canBeIdentifier(p.currentToken.Type) {
			return p.parseKeywordAsIdentifier()
		}
		
		p.addError(NewParseError(
			pos,
			"expected expression",
			[]TokenType{TokenLeftParen, TokenNot, TokenLeftBracket, TokenLeftBrace, TokenIf, TokenIdentifier},
			p.currentToken,
		))
		return nil, false
	}
}

// parseParenthesizedExpression parses "(" expr ")"
func (p *Parser) parseParenthesizedExpression() (expr.Expr, bool) {
	if !p.consume(TokenLeftParen) {
		return nil, false
	}

	// Check if this might be a pair literal: (expr, expr)
	expr1, ok := p.parseExpression()
	if !ok {
		return nil, false
	}

	if p.currentTokenIs(TokenComma) {
		// This is a pair literal
		p.nextToken() // consume comma
		
		expr2, ok := p.parseExpression()
		if !ok {
			return nil, false
		}
		
		if !p.consume(TokenRightParen) {
			return nil, false
		}
		
		return expr.NewPairLiteral(expr1, expr2, expr1.Pos()), true
	} else {
		// Regular parenthesized expression
		if !p.consume(TokenRightParen) {
			return nil, false
		}
		
		return expr1, true
	}
}

// parseUnaryExpression parses "!" expr_core -> negate
func (p *Parser) parseUnaryExpression() (expr.Expr, bool) {
	pos := p.currentPosition()

	if !p.consume(TokenNot) {
		return nil, false
	}

	operand, ok := p.parseExprCore()
	if !ok {
		return nil, false
	}

	return expr.NewUnaryOp("!", operand, pos), true
}

// parseArrayLiteral parses "[" [expr ("," expr)*] ","? "]" -> array
func (p *Parser) parseArrayLiteral() (expr.Expr, bool) {
	pos := p.currentPosition()

	if !p.consume(TokenLeftBracket) {
		return nil, false
	}

	var elements []expr.Expr

	// Handle empty array
	if p.currentTokenIs(TokenRightBracket) {
		p.nextToken()
		return expr.NewArrayLiteral(elements, pos), true
	}

	// Parse first element
	firstElement, ok := p.parseExpression()
	if !ok {
		return nil, false
	}
	elements = append(elements, firstElement)

	// Parse remaining elements
	for p.currentTokenIs(TokenComma) {
		p.nextToken() // consume comma

		// Check for trailing comma
		if p.currentTokenIs(TokenRightBracket) {
			break
		}

		element, ok := p.parseExpression()
		if !ok {
			return nil, false
		}
		elements = append(elements, element)
	}

	if !p.consume(TokenRightBracket) {
		return nil, false
	}

	return expr.NewArrayLiteral(elements, pos), true
}

// parseMapLiteral parses "{" [map_kv ("," map_kv)*] ","? "}" -> map
func (p *Parser) parseMapLiteral() (expr.Expr, bool) {
	pos := p.currentPosition()

	if !p.consume(TokenLeftBrace) {
		return nil, false
	}

	var items []expr.MapItem

	// Handle empty map
	if p.currentTokenIs(TokenRightBrace) {
		p.nextToken()
		return expr.NewMapLiteral(items, pos), true
	}

	// Parse first key-value pair
	firstItem, ok := p.parseMapKeyValue()
	if !ok {
		return nil, false
	}
	items = append(items, firstItem)

	// Parse remaining items
	for p.currentTokenIs(TokenComma) {
		p.nextToken() // consume comma

		// Check for trailing comma
		if p.currentTokenIs(TokenRightBrace) {
			break
		}

		item, ok := p.parseMapKeyValue()
		if !ok {
			return nil, false
		}
		items = append(items, item)
	}

	if !p.consume(TokenRightBrace) {
		return nil, false
	}

	return expr.NewMapLiteral(items, pos), true
}

// parseMapKeyValue parses map_kv: map_key ":" expr
func (p *Parser) parseMapKeyValue() (expr.MapItem, bool) {
	// Parse key (map_key: expr_core)
	key, ok := p.parseExprCore()
	if !ok {
		return expr.MapItem{}, false
	}

	if !p.consume(TokenColon) {
		return expr.MapItem{}, false
	}

	// Parse value
	value, ok := p.parseExpression()
	if !ok {
		return expr.MapItem{}, false
	}

	return expr.MapItem{Key: key, Value: value}, true
}

// parseIfThenElse parses "if" expr "then" expr "else" expr -> ifthenelse
func (p *Parser) parseIfThenElse() (expr.Expr, bool) {
	pos := p.currentPosition()

	if !p.consume(TokenIf) {
		return nil, false
	}

	condition, ok := p.parseExpression()
	if !ok {
		return nil, false
	}

	if !p.consume(TokenThen) {
		return nil, false
	}

	thenExpr, ok := p.parseExpression()
	if !ok {
		return nil, false
	}

	if !p.consume(TokenElse) {
		return nil, false
	}

	elseExpr, ok := p.parseExpression()
	if !ok {
		return nil, false
	}

	return expr.NewIfThenElse(condition, thenExpr, elseExpr, pos), true
}

// parseIdentifierExpression parses identifier-based expressions:
// CNAME "(" [expr ("," expr)*] ")" -> apply
// CNAME "{" [object_kv ("," object_kv)* ","?] "}" -> obj
// CNAME -> left_name
// expr_core "." CNAME -> get_name
func (p *Parser) parseIdentifierExpression() (expr.Expr, bool) {
	pos := p.currentPosition()
	name := p.currentToken.Value
	p.nextToken()

	switch p.currentToken.Type {
	case TokenLeftParen:
		// Function application: CNAME "(" [expr ("," expr)*] ")"
		return p.parseFunctionApplication(name, pos)
		
	case TokenLeftBrace:
		// Struct literal: CNAME "{" [object_kv ("," object_kv)* ","?] "}"
		return p.parseStructLiteral(name, pos)
		
	default:
		// Simple identifier: CNAME -> left_name
		// But we need to check for postfix operations like member access or array indexing
		return p.parsePostfixExpression(expr.NewIdentifier(name, pos))
	}
}

// parseFunctionApplication parses function calls
func (p *Parser) parseFunctionApplication(funcName string, pos errors.SourcePosition) (expr.Expr, bool) {
	if !p.consume(TokenLeftParen) {
		return nil, false
	}

	var args []expr.Expr

	// Handle empty argument list
	if p.currentTokenIs(TokenRightParen) {
		p.nextToken()
		return expr.NewApply(funcName, args, pos), true
	}

	// Parse first argument
	firstArg, ok := p.parseExpression()
	if !ok {
		return nil, false
	}
	args = append(args, firstArg)

	// Parse remaining arguments
	for p.currentTokenIs(TokenComma) {
		p.nextToken() // consume comma

		arg, ok := p.parseExpression()
		if !ok {
			return nil, false
		}
		args = append(args, arg)
	}

	if !p.consume(TokenRightParen) {
		return nil, false
	}

	return expr.NewApply(funcName, args, pos), true
}

// parseStructLiteral parses struct literals
func (p *Parser) parseStructLiteral(typeName string, pos errors.SourcePosition) (expr.Expr, bool) {
	if !p.consume(TokenLeftBrace) {
		return nil, false
	}

	var members []expr.StructMember

	// Handle empty struct
	if p.currentTokenIs(TokenRightBrace) {
		p.nextToken()
		return expr.NewStructLiteral(typeName, members, pos), true
	}

	// Parse first member
	firstMember, ok := p.parseObjectKeyValue()
	if !ok {
		return nil, false
	}
	members = append(members, firstMember)

	// Parse remaining members
	for p.currentTokenIs(TokenComma) {
		p.nextToken() // consume comma

		// Check for trailing comma
		if p.currentTokenIs(TokenRightBrace) {
			break
		}

		member, ok := p.parseObjectKeyValue()
		if !ok {
			return nil, false
		}
		members = append(members, member)
	}

	if !p.consume(TokenRightBrace) {
		return nil, false
	}

	return expr.NewStructLiteral(typeName, members, pos), true
}

// parseObjectKeyValue parses object_kv: CNAME ":" expr | string_literal ":" expr
func (p *Parser) parseObjectKeyValue() (expr.StructMember, bool) {
	var memberName string

	if p.currentTokenIs(TokenIdentifier) {
		memberName = p.currentToken.Value
		p.nextToken()
	} else if p.currentTokenIs(TokenString) {
		memberName = p.currentToken.Value
		p.nextToken()
	} else {
		p.addError(p.expectError(TokenIdentifier, TokenString))
		return expr.StructMember{}, false
	}

	if !p.consume(TokenColon) {
		return expr.StructMember{}, false
	}

	value, ok := p.parseExpression()
	if !ok {
		return expr.StructMember{}, false
	}

	return expr.StructMember{Name: memberName, Value: value}, true
}

// parsePostfixExpression handles postfix operations like member access and array indexing
// expr_core "." CNAME -> get_name
// expr_core "[" expr "]" -> at
func (p *Parser) parsePostfixExpression(baseExpr expr.Expr) (expr.Expr, bool) {
	result := baseExpr

	for {
		switch p.currentToken.Type {
		case TokenDot:
			// Member access: expr_core "." CNAME
			p.nextToken() // consume dot
			
			var memberName string
			if p.currentTokenIs(TokenIdentifier) {
				memberName = p.currentToken.Value
			} else if p.canBeIdentifier(p.currentToken.Type) {
				memberName = p.currentToken.Value
			} else {
				p.addError(p.expectError(TokenIdentifier))
				return nil, false
			}
			
			pos := p.currentPosition()
			p.nextToken()
			
			result = expr.NewGetAttr(result, memberName, pos)
			
		case TokenLeftBracket:
			// Array/map indexing: expr_core "[" expr "]"
			pos := p.currentPosition()
			p.nextToken() // consume [
			
			index, ok := p.parseExpression()
			if !ok {
				return nil, false
			}
			
			if !p.consume(TokenRightBracket) {
				return nil, false
			}
			
			result = expr.NewGetIndex(result, index, pos)
			
		default:
			// No more postfix operations
			return result, true
		}
	}
}

// isComparisonOperator returns true if current token is a comparison operator
func (p *Parser) isComparisonOperator() bool {
	switch p.currentToken.Type {
	case TokenEqual, TokenNotEqual, TokenLess, TokenGreater, TokenLessEqual, TokenGreaterEqual:
		return true
	default:
		return false
	}
}

// canBeIdentifier returns true if a token can be used as an identifier in expression context
func (p *Parser) canBeIdentifier(tokenType TokenType) bool {
	switch tokenType {
	case TokenTask, TokenWorkflow, TokenCall, TokenInput, TokenOutput, TokenMeta, TokenStruct:
		// These keywords can be used as identifiers in expressions
		return true
	default:
		return false
	}
}

// parseKeywordAsIdentifier parses a keyword token as an identifier
func (p *Parser) parseKeywordAsIdentifier() (expr.Expr, bool) {
	pos := p.currentPosition()
	name := p.currentToken.Value
	p.nextToken()
	
	// Create identifier and check for postfix operations
	return p.parsePostfixExpression(expr.NewIdentifier(name, pos))
}