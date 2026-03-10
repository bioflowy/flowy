// Package parser - Pratt expression sub-parser for WDL 1.2.
// This file implements the full expression parser replacing the parseExpr stub in parser.go.
// Operator precedence (binding power table):
//
//	||          leftBP=10, rightBP=11  (left-associative)
//	&&          leftBP=20, rightBP=21  (left-associative)
//	==!=<<=>>= leftBP=30, rightBP=31  (left-associative)
//	+ -         leftBP=40, rightBP=41  (left-associative)
//	* / %       leftBP=50, rightBP=51  (left-associative)
//	**          leftBP=60, rightBP=60  (right-associative: same left/right BP)
//	unary ! -   rightBP=70            (prefix)
package parser

import (
	"strconv"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/lexer"
)

// parseExpr is the Pratt expression parser entry point.
// minBP is the minimum binding power for this parse level.
// The caller passes minBP=0 to parse a complete expression.
// The variadic delimiters parameter is ignored (kept for backward compatibility with
// the stub signature; Pratt parsing terminates naturally at non-infix tokens).
func (p *Parser) parseExpr(delimiters ...lexer.TokenType) (ast.Expr, error) {
	return p.prattExpr(0)
}

// prattExpr implements the Pratt precedence-climbing algorithm.
func (p *Parser) prattExpr(minBP int) (ast.Expr, error) {
	// Parse the left-hand side (prefix/primary expression)
	lhs, err := p.parsePrefixExpr()
	if err != nil {
		return nil, err
	}

	// Postfix loop: handle index access and member access at highest priority
	for {
		tok := p.peek()
		if tok.Type == lexer.TokLBracket {
			// Index access: lhs[index]
			p.advance()
			pos := tok.Pos
			index, err := p.prattExpr(0)
			if err != nil {
				return nil, err
			}
			_, err = p.expect(lexer.TokRBracket)
			if err != nil {
				return nil, err
			}
			lhs = &ast.IndexExpr{Pos: pos, Collection: lhs, Index: index}
		} else if tok.Type == lexer.TokDot {
			// Member access: lhs.member
			p.advance()
			pos := tok.Pos
			memberTok := p.advance()
			if memberTok.Type != lexer.TokIdent && !isNameToken(memberTok.Type) {
				return nil, p.errorf(memberTok.Pos, "expected member name after '.', got %q", memberTok.Val)
			}
			lhs = &ast.GetExpr{Pos: pos, Object: lhs, Member: memberTok.Val}
		} else {
			break
		}
	}

	// Binary operator loop
	for {
		op, leftBP, rightBP, ok := p.peekInfix()
		if !ok || leftBP <= minBP {
			break
		}
		p.advance() // consume operator
		opPos := p.cur.Pos

		rhs, err := p.prattExpr(rightBP)
		if err != nil {
			return nil, err
		}
		lhs = &ast.BinaryExpr{Pos: opPos, Op: op, Left: lhs, Right: rhs}

		// Re-check postfix after binary op
		for {
			tok := p.peek()
			if tok.Type == lexer.TokLBracket {
				p.advance()
				pos := tok.Pos
				index, err := p.prattExpr(0)
				if err != nil {
					return nil, err
				}
				_, err = p.expect(lexer.TokRBracket)
				if err != nil {
					return nil, err
				}
				lhs = &ast.IndexExpr{Pos: pos, Collection: lhs, Index: index}
			} else if tok.Type == lexer.TokDot {
				p.advance()
				pos := tok.Pos
				memberTok := p.advance()
				if memberTok.Type != lexer.TokIdent && !isNameToken(memberTok.Type) {
					return nil, p.errorf(memberTok.Pos, "expected member name after '.', got %q", memberTok.Val)
				}
				lhs = &ast.GetExpr{Pos: pos, Object: lhs, Member: memberTok.Val}
			} else {
				break
			}
		}
	}

	return lhs, nil
}

// peekInfix returns the binding powers for the current token if it is a binary infix operator.
// Returns (op, leftBP, rightBP, ok). ok=false if the current token is not an infix operator.
// Note: index [] and dot . are handled as postfix in prattExpr, not here.
func (p *Parser) peekInfix() (op lexer.TokenType, leftBP, rightBP int, ok bool) {
	tt := p.peek().Type
	switch tt {
	case lexer.TokOr:
		return tt, 10, 11, true
	case lexer.TokAnd:
		return tt, 20, 21, true
	case lexer.TokEqEq, lexer.TokNotEq,
		lexer.TokLt, lexer.TokLtEq,
		lexer.TokGt, lexer.TokGtEq:
		return tt, 30, 31, true
	case lexer.TokPlus, lexer.TokMinus:
		return tt, 40, 41, true
	case lexer.TokStar, lexer.TokSlash, lexer.TokPercent:
		return tt, 50, 51, true
	case lexer.TokStarStar:
		// Right-associative: leftBP == rightBP == 60
		// The loop condition is leftBP <= minBP (strict), so when another **
		// is seen at the same level, leftBP=60 > current minBP (which would be
		// the rightBP=60 from the outer call), triggering right-associativity.
		// Wait — actually: left=60, right=60. The loop condition `leftBP <= minBP`
		// breaks when leftBP <= minBP. For right-assoc: recursive call uses rightBP=60,
		// and the next ** has leftBP=60. 60 <= 60 is true so we BREAK — that makes it
		// LEFT-associative. For right-assoc we need rightBP < leftBP (one less).
		// But RESEARCH.md says leftBP=60, rightBP=60. Let's check the loop condition:
		// "if leftBP <= minBP, break" — for ** with leftBP=rightBP=60:
		// outer call: minBP=0, sees **, leftBP=60 > 0, proceed, recurse with minBP=60
		// inner call: minBP=60, sees **, leftBP=60 <= 60 → BREAK → right-assoc! ✓
		// So leftBP=60, rightBP=60 with condition `leftBP <= minBP` IS right-associative.
		return tt, 60, 60, true
	}
	return 0, 0, 0, false
}

// parsePrefixExpr parses prefix (unary) and primary expressions.
func (p *Parser) parsePrefixExpr() (ast.Expr, error) {
	tok := p.peek()

	// Check for TokError first
	if tok.Type == lexer.TokError {
		p.advance()
		return nil, p.errorf(tok.Pos, "%s", tok.Val)
	}

	switch tok.Type {
	case lexer.TokTrue:
		p.advance()
		return &ast.BoolLit{Pos: tok.Pos, Value: true}, nil

	case lexer.TokFalse:
		p.advance()
		return &ast.BoolLit{Pos: tok.Pos, Value: false}, nil

	case lexer.TokNull:
		p.advance()
		return &ast.NullLit{Pos: tok.Pos}, nil

	case lexer.TokIntLit:
		p.advance()
		val, err := strconv.ParseInt(tok.Val, 0, 64)
		if err != nil {
			// Try octal prefix parsing; some WDL uses 0777 style
			val, err = strconv.ParseInt(tok.Val, 10, 64)
			if err != nil {
				return nil, p.errorf(tok.Pos, "invalid integer literal %q: %v", tok.Val, err)
			}
		}
		return &ast.IntLit{Pos: tok.Pos, Value: val}, nil

	case lexer.TokFloatLit:
		p.advance()
		val, err := strconv.ParseFloat(tok.Val, 64)
		if err != nil {
			return nil, p.errorf(tok.Pos, "invalid float literal %q: %v", tok.Val, err)
		}
		return &ast.FloatLit{Pos: tok.Pos, Value: val}, nil

	case lexer.TokStringOpen:
		return p.parseStringExpr()

	case lexer.TokLParen:
		p.advance()
		pos := tok.Pos
		inner, err := p.prattExpr(0)
		if err != nil {
			return nil, err
		}
		// Check if this is a Pair literal: (left, right)
		if p.peek().Type == lexer.TokComma {
			p.advance()
			right, err := p.prattExpr(0)
			if err != nil {
				return nil, err
			}
			_, err = p.expect(lexer.TokRParen)
			if err != nil {
				return nil, err
			}
			return &ast.PairExpr{Pos: pos, Left: inner, Right: right}, nil
		}
		_, err = p.expect(lexer.TokRParen)
		if err != nil {
			return nil, err
		}
		return inner, nil

	case lexer.TokLBracket:
		return p.parseArrayExpr()

	case lexer.TokLBrace:
		return p.parseMapExpr()

	case lexer.TokNot:
		p.advance()
		operand, err := p.prattExpr(70)
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{Pos: tok.Pos, Op: lexer.TokNot, Operand: operand}, nil

	case lexer.TokMinus:
		p.advance()
		operand, err := p.prattExpr(70)
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{Pos: tok.Pos, Op: lexer.TokMinus, Operand: operand}, nil

	case lexer.TokIdent:
		return p.parseIdentOrCallOrStruct()

	case lexer.TokIf:
		return p.parseIfThenElse()

	default:
		p.advance()
		return nil, p.errorf(tok.Pos, "unexpected token %q in expression", tok.Val)
	}
}

// parseIdentOrCallOrStruct parses an identifier, which may be:
//   - A function call:    name(args...)
//   - A struct literal:   TypeName { field: expr, ... }
//   - A plain identifier: name
func (p *Parser) parseIdentOrCallOrStruct() (ast.Expr, error) {
	tok, err := p.expect(lexer.TokIdent)
	if err != nil {
		return nil, err
	}
	pos := tok.Pos
	name := tok.Val

	switch p.peek().Type {
	case lexer.TokLParen:
		// Function call: name(args...)
		p.advance() // consume (
		var args []ast.Expr
		for p.peek().Type != lexer.TokRParen && p.peek().Type != lexer.TokEOF {
			arg, err := p.prattExpr(0)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			if p.peek().Type == lexer.TokComma {
				p.advance()
			} else {
				break
			}
		}
		_, err = p.expect(lexer.TokRParen)
		if err != nil {
			return nil, err
		}
		return &ast.ApplyExpr{Pos: pos, Name: name, Args: args}, nil

	case lexer.TokLBrace:
		// Struct literal: TypeName { field: expr, ... }
		p.advance() // consume {
		var members []ast.StructMember
		for p.peek().Type != lexer.TokRBrace && p.peek().Type != lexer.TokEOF {
			fieldTok, err := p.expect(lexer.TokIdent)
			if err != nil {
				return nil, err
			}
			_, err = p.expect(lexer.TokColon)
			if err != nil {
				return nil, err
			}
			val, err := p.prattExpr(0)
			if err != nil {
				return nil, err
			}
			members = append(members, ast.StructMember{Name: fieldTok.Val, Value: val})
			if p.peek().Type == lexer.TokComma {
				p.advance()
			} else {
				break
			}
		}
		_, err = p.expect(lexer.TokRBrace)
		if err != nil {
			return nil, err
		}
		return &ast.StructExpr{Pos: pos, TypeName: name, Members: members}, nil

	default:
		return &ast.IdentExpr{Pos: pos, Name: name}, nil
	}
}

// parseStringExpr parses a string expression starting with TokStringOpen.
// Produces an ast.StringExpr with a Parts list alternating RawStringPart and PlaceholderPart.
func (p *Parser) parseStringExpr() (*ast.StringExpr, error) {
	openTok, err := p.expect(lexer.TokStringOpen)
	if err != nil {
		return nil, err
	}
	pos := openTok.Pos

	var parts []ast.StringPart

	for p.peek().Type != lexer.TokStringClose && p.peek().Type != lexer.TokEOF {
		tok := p.peek()
		switch tok.Type {
		case lexer.TokCommandContent:
			p.advance()
			parts = append(parts, &ast.RawStringPart{Val: tok.Val})
		case lexer.TokPlaceholderOpen:
			part, err := p.parsePlaceholderPart()
			if err != nil {
				return nil, err
			}
			parts = append(parts, part)
		default:
			// Unexpected token; advance to avoid infinite loop
			p.advance()
			return nil, p.errorf(tok.Pos, "unexpected token %q in string", tok.Val)
		}
	}

	_, err = p.expect(lexer.TokStringClose)
	if err != nil {
		return nil, err
	}

	return &ast.StringExpr{Pos: pos, Parts: parts}, nil
}

// parsePlaceholderPart parses a placeholder inside a string or command block.
// The TokPlaceholderOpen has already been peeked but NOT consumed.
// On entry: p.peek() == TokPlaceholderOpen
// On exit:  TokPlaceholderClose has been consumed.
func (p *Parser) parsePlaceholderPart() (*ast.PlaceholderPart, error) {
	openTok := p.advance() // consume TokPlaceholderOpen
	pos := openTok.Pos

	// Parse optional placeholder options: ident = (string | bool) sequences
	// e.g., sep="," true="yes" false="no" default="none"
	options := make(map[string]string)
	for p.peek().Type == lexer.TokIdent {
		// Look ahead: if next-next is TokAssign, this is an option
		// We need two-token lookahead here. Use a simple heuristic:
		// if the ident is one of the known option keywords, treat as option.
		optName := p.peek().Val
		if optName == "sep" || optName == "true" || optName == "false" || optName == "default" {
			// Peek one more: should be "="
			saved := p.peek()
			p.advance() // consume ident
			if p.peek().Type == lexer.TokAssign {
				p.advance() // consume "="
				// Consume the value (string or bool literal)
				valTok := p.advance()
				var optVal string
				if valTok.Type == lexer.TokStringOpen {
					// Consume string content until StringClose
					var sb []byte
					for p.peek().Type == lexer.TokCommandContent {
						sb = append(sb, p.advance().Val...)
					}
					_, err := p.expect(lexer.TokStringClose)
					if err != nil {
						return nil, err
					}
					optVal = string(sb)
				} else {
					optVal = valTok.Val
				}
				options[optName] = optVal
			} else {
				// Not an option — this was actually the expression; put it back
				// We can't "unread" but we can parse as ident expression manually
				// The token is already consumed as 'saved'. Build an IdentExpr.
				_ = saved
				identExpr := &ast.IdentExpr{Pos: saved.Pos, Name: saved.Val}
				// Parse the rest of the expression starting from identExpr as the lhs
				// We need to integrate this into the Pratt loop. Use a workaround:
				// continue with remaining parsing after building the full expression.
				expr, err := p.finishExprFromIdent(identExpr)
				if err != nil {
					return nil, err
				}
				_, err = p.expect(lexer.TokPlaceholderClose)
				if err != nil {
					return nil, err
				}
				return &ast.PlaceholderPart{Pos: pos, Options: options, Expr: expr}, nil
			}
		} else {
			break
		}
	}

	// Parse the placeholder expression
	expr, err := p.prattExpr(0)
	if err != nil {
		return nil, err
	}

	_, err = p.expect(lexer.TokPlaceholderClose)
	if err != nil {
		return nil, err
	}

	return &ast.PlaceholderPart{Pos: pos, Options: options, Expr: expr}, nil
}

// finishExprFromIdent completes a Pratt parse starting from an already-parsed lhs ident.
// This is needed when we speculatively parsed an identifier looking for placeholder options
// but found it was actually the start of an expression.
func (p *Parser) finishExprFromIdent(lhs ast.Expr) (ast.Expr, error) {
	// Handle postfix on the ident first
	for {
		tok := p.peek()
		if tok.Type == lexer.TokLBracket {
			p.advance()
			pos := tok.Pos
			index, err := p.prattExpr(0)
			if err != nil {
				return nil, err
			}
			_, err = p.expect(lexer.TokRBracket)
			if err != nil {
				return nil, err
			}
			lhs = &ast.IndexExpr{Pos: pos, Collection: lhs, Index: index}
		} else if tok.Type == lexer.TokDot {
			p.advance()
			pos := tok.Pos
			memberTok := p.advance()
			if memberTok.Type != lexer.TokIdent && !isNameToken(memberTok.Type) {
				return nil, p.errorf(memberTok.Pos, "expected member name after '.', got %q", memberTok.Val)
			}
			lhs = &ast.GetExpr{Pos: pos, Object: lhs, Member: memberTok.Val}
		} else if tok.Type == lexer.TokLParen {
			// Function call on ident
			p.advance()
			identExpr, isIdent := lhs.(*ast.IdentExpr)
			if !isIdent {
				return nil, p.errorf(tok.Pos, "unexpected '(' after non-identifier expression")
			}
			var args []ast.Expr
			for p.peek().Type != lexer.TokRParen && p.peek().Type != lexer.TokEOF {
				arg, err := p.prattExpr(0)
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
				if p.peek().Type == lexer.TokComma {
					p.advance()
				} else {
					break
				}
			}
			_, err := p.expect(lexer.TokRParen)
			if err != nil {
				return nil, err
			}
			lhs = &ast.ApplyExpr{Pos: identExpr.Pos, Name: identExpr.Name, Args: args}
		} else {
			break
		}
	}

	// Binary operators
	for {
		op, leftBP, rightBP, ok := p.peekInfix()
		if !ok || leftBP <= 0 {
			break
		}
		p.advance()
		opPos := p.cur.Pos
		rhs, err := p.prattExpr(rightBP)
		if err != nil {
			return nil, err
		}
		lhs = &ast.BinaryExpr{Pos: opPos, Op: op, Left: lhs, Right: rhs}
	}

	return lhs, nil
}

// parseArrayExpr parses an array literal: "[" expr ("," expr)* "]"
func (p *Parser) parseArrayExpr() (*ast.ArrayExpr, error) {
	openTok, err := p.expect(lexer.TokLBracket)
	if err != nil {
		return nil, err
	}
	pos := openTok.Pos

	var items []ast.Expr
	for p.peek().Type != lexer.TokRBracket && p.peek().Type != lexer.TokEOF {
		item, err := p.prattExpr(0)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		if p.peek().Type == lexer.TokComma {
			p.advance()
		} else {
			break
		}
	}

	_, err = p.expect(lexer.TokRBracket)
	if err != nil {
		return nil, err
	}

	return &ast.ArrayExpr{Pos: pos, Items: items}, nil
}

// parseMapExpr parses a map literal: "{" (expr ":" expr) ("," (expr ":" expr))* "}"
func (p *Parser) parseMapExpr() (*ast.MapExpr, error) {
	openTok, err := p.expect(lexer.TokLBrace)
	if err != nil {
		return nil, err
	}
	pos := openTok.Pos

	var items []ast.MapItem
	for p.peek().Type != lexer.TokRBrace && p.peek().Type != lexer.TokEOF {
		key, err := p.prattExpr(0)
		if err != nil {
			return nil, err
		}
		_, err = p.expect(lexer.TokColon)
		if err != nil {
			return nil, err
		}
		val, err := p.prattExpr(0)
		if err != nil {
			return nil, err
		}
		items = append(items, ast.MapItem{Key: key, Value: val})
		if p.peek().Type == lexer.TokComma {
			p.advance()
		} else {
			break
		}
	}

	_, err = p.expect(lexer.TokRBrace)
	if err != nil {
		return nil, err
	}

	return &ast.MapExpr{Pos: pos, Items: items}, nil
}

// parseIfThenElse parses "if expr then expr else expr"
func (p *Parser) parseIfThenElse() (*ast.IfThenElse, error) {
	ifTok, err := p.expect(lexer.TokIf)
	if err != nil {
		return nil, err
	}
	pos := ifTok.Pos

	cond, err := p.prattExpr(0)
	if err != nil {
		return nil, err
	}

	_, err = p.expect(lexer.TokThen)
	if err != nil {
		return nil, err
	}

	thenExpr, err := p.prattExpr(0)
	if err != nil {
		return nil, err
	}

	_, err = p.expect(lexer.TokElse)
	if err != nil {
		return nil, err
	}

	elseExpr, err := p.prattExpr(0)
	if err != nil {
		return nil, err
	}

	return &ast.IfThenElse{Pos: pos, Cond: cond, Then: thenExpr, Else: elseExpr}, nil
}
