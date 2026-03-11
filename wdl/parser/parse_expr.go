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
	"strings"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/lexer"
)

// unescapeString processes WDL string escape sequences.
// WDL 1.2 supports: \n (newline), \t (tab), \r (CR), \\ (backslash), \" (double-quote).
// Unknown escape sequences are preserved as-is (backslash + char).
func unescapeString(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s // fast path: no escapes
	}
	var sb strings.Builder
	sb.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case 'r':
				sb.WriteByte('\r')
			case '\\':
				sb.WriteByte('\\')
			case '"':
				sb.WriteByte('"')
			case '\'':
				sb.WriteByte('\'')
			default:
				// Unknown escape: preserve as-is
				sb.WriteByte('\\')
				sb.WriteByte(s[i+1])
			}
			i += 2
		} else {
			sb.WriteByte(s[i])
			i++
		}
	}
	return sb.String()
}

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

	// Binary operator loop.
	// Break condition: leftBP <= minBP.
	// Left-associative: leftBP < rightBP, so recursive call with rightBP causes next
	//   same-priority op to break (leftBP <= rightBP is true).
	// Right-associative (**): leftBP=60, rightBP=59. Recursive call uses minBP=59.
	//   Next ** has leftBP=60. 60 <= 59 is false → loop continues → right-assoc.
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
		// Right-associative exponentiation.
		// leftBP=60, rightBP=59: the recursive call uses minBP=59.
		// The next ** has leftBP=60. 60 <= 59 is false → loop continues → right-assoc.
		// For left-assoc operators: leftBP=N, rightBP=N+1 so recursive minBP=N+1;
		//   next same op has leftBP=N; N <= N+1 → break → left-assoc.
		return tt, 60, 59, true
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

	case lexer.TokHeredocOpen:
		return p.parseMultilineStringExpr()

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
		// rightBP=55: lower than ** leftBP=60 so unary ! binds less tightly than **
		// i.e., !x**2 → !(x**2). Higher than *, /, % (leftBP=50) so !x*y → (!x)*y.
		operand, err := p.prattExpr(55)
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{Pos: tok.Pos, Op: lexer.TokNot, Operand: operand}, nil

	case lexer.TokMinus:
		p.advance()
		// rightBP=55: lower than ** leftBP=60 so unary - binds less tightly than **
		// i.e., -x**2 → -(x**2). This matches Python/math convention.
		operand, err := p.prattExpr(55)
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{Pos: tok.Pos, Op: lexer.TokMinus, Operand: operand}, nil

	case lexer.TokIdent:
		return p.parseIdentOrCallOrStruct()

	case lexer.TokIf:
		return p.parseIfThenElse()

	default:
		// Handle keyword tokens that are valid as identifiers in expression context.
		// For example, "env" is TokEnv but may appear as a variable name.
		if isNameToken(tok.Type) {
			p.advance()
			return &ast.IdentExpr{Pos: tok.Pos, Name: tok.Val}, nil
		}
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
			// Process WDL string escape sequences (\n, \t, \\, etc.).
			// Command blocks (heredoc/brace) should NOT be unescaped here —
			// they are processed only as raw shell text.
			parts = append(parts, &ast.RawStringPart{Val: unescapeString(tok.Val)})
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
	// Note: "true" and "false" are lexed as TokTrue/TokFalse keywords, not TokIdent.
	options := make(map[string]string)
	for {
		tok := p.peek()
		var optName string
		switch tok.Type {
		case lexer.TokIdent:
			if tok.Val == "sep" || tok.Val == "default" {
				optName = tok.Val
			}
		case lexer.TokTrue:
			optName = "true"
		case lexer.TokFalse:
			optName = "false"
		}
		if optName == "" {
			break
		}

		// Speculatively consume the token to check for "="
		saved := p.peek()
		p.advance()
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
			// Not an option — this was actually the expression; build AST from the consumed token.
			var lhs ast.Expr
			switch saved.Type {
			case lexer.TokTrue:
				lhs = &ast.BoolLit{Pos: saved.Pos, Value: true}
			case lexer.TokFalse:
				lhs = &ast.BoolLit{Pos: saved.Pos, Value: false}
			default:
				lhs = &ast.IdentExpr{Pos: saved.Pos, Name: saved.Val}
			}
			expr, err := p.finishExprFromIdent(lhs)
			if err != nil {
				return nil, err
			}
			_, err = p.expect(lexer.TokPlaceholderClose)
			if err != nil {
				return nil, err
			}
			return &ast.PlaceholderPart{Pos: pos, Options: options, Expr: expr}, nil
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

// parseMultilineStringExpr parses a multiline string expression starting with TokHeredocOpen (<<<).
// Produces an ast.StringExpr with whitespace stripping applied per WDL 1.2 spec.
func (p *Parser) parseMultilineStringExpr() (*ast.StringExpr, error) {
	openTok, err := p.expect(lexer.TokHeredocOpen)
	if err != nil {
		return nil, err
	}
	pos := openTok.Pos

	var parts []ast.StringPart

	for p.peek().Type != lexer.TokHeredocClose && p.peek().Type != lexer.TokEOF {
		tok := p.peek()
		switch tok.Type {
		case lexer.TokCommandContent:
			p.advance()
			// Do NOT unescape — multiline strings preserve raw content
			parts = append(parts, &ast.RawStringPart{Val: tok.Val})
		case lexer.TokPlaceholderOpen:
			part, err := p.parsePlaceholderPart()
			if err != nil {
				return nil, err
			}
			parts = append(parts, part)
		default:
			p.advance()
			return nil, p.errorf(tok.Pos, "unexpected token %q in multiline string", tok.Val)
		}
	}

	_, err = p.expect(lexer.TokHeredocClose)
	if err != nil {
		return nil, err
	}

	parts = stripMultilineWhitespace(parts)

	return &ast.StringExpr{Pos: pos, Parts: parts}, nil
}

// stripMultilineWhitespace applies the WDL 1.2 whitespace stripping algorithm
// for multiline string literals. The algorithm has 5 steps:
//  1. Process line continuations (backslash before newline)
//  2. Strip everything after <<< up to and including the first newline
//  3. Strip everything before >>> up to and including the preceding newline
//  4. Calculate common leading whitespace from non-blank lines
//  5. Remove common leading whitespace from all lines
func stripMultilineWhitespace(parts []ast.StringPart) []ast.StringPart {
	// Step 1: Process line continuations
	parts = processLineContinuations(parts)

	// Step 2: Strip content after opening <<< up to first newline
	parts = stripAfterOpen(parts)

	// Step 3: Strip content before closing >>> after last newline
	parts = stripBeforeClose(parts)

	// Step 4 & 5: Calculate and remove common leading whitespace
	parts = removeCommonIndent(parts)

	// Remove empty raw parts
	var cleaned []ast.StringPart
	for _, p := range parts {
		if raw, ok := p.(*ast.RawStringPart); ok && raw.Val == "" {
			continue
		}
		cleaned = append(cleaned, p)
	}
	if len(cleaned) == 0 {
		return []ast.StringPart{&ast.RawStringPart{Val: ""}}
	}
	return cleaned
}

// processLineContinuations handles backslash-newline continuations in RawStringParts.
// Per WDL 1.2 spec:
//   - Each pair of consecutive backslashes (\\) is an escaped backslash producing a single \.
//   - A line is continued only if it ends in an odd number of backslashes before a newline.
//   - Removing a line continuation means removing the last \, the newline, and all subsequent
//     whitespace before the next non-whitespace character or end of line.
func processLineContinuations(parts []ast.StringPart) []ast.StringPart {
	result := make([]ast.StringPart, 0, len(parts))
	for _, p := range parts {
		raw, ok := p.(*ast.RawStringPart)
		if !ok {
			result = append(result, p)
			continue
		}
		val := raw.Val
		var sb strings.Builder
		sb.Grow(len(val))
		i := 0
		for i < len(val) {
			if val[i] == '\\' {
				// Count consecutive backslashes
				bsStart := i
				for i < len(val) && val[i] == '\\' {
					i++
				}
				bsCount := i - bsStart
				if i < len(val) && val[i] == '\n' {
					// Backslashes before newline: each pair → single backslash
					for j := 0; j < bsCount/2; j++ {
						sb.WriteByte('\\')
					}
					if bsCount%2 == 1 {
						// Odd: last backslash is a line continuation
						i++ // skip \n
						// Skip leading whitespace on next line
						for i < len(val) && (val[i] == ' ' || val[i] == '\t') {
							i++
						}
					} else {
						// Even: no continuation, preserve newline
						// The backslashes were already written as escaped pairs above
					}
				} else {
					// Backslashes not before newline: preserve as-is
					sb.WriteString(val[bsStart:i])
				}
			} else {
				sb.WriteByte(val[i])
				i++
			}
		}
		result = append(result, &ast.RawStringPart{Val: sb.String()})
	}
	return result
}

// stripAfterOpen removes whitespace after <<< up to and including the first newline (if any).
// If there is no newline, only leading whitespace is removed.
func stripAfterOpen(parts []ast.StringPart) []ast.StringPart {
	if len(parts) == 0 {
		return parts
	}
	raw, ok := parts[0].(*ast.RawStringPart)
	if !ok {
		return parts
	}
	idx := strings.IndexByte(raw.Val, '\n')
	if idx >= 0 {
		// Check if everything before the newline is whitespace
		before := raw.Val[:idx]
		allWS := true
		for _, c := range before {
			if c != ' ' && c != '\t' {
				allWS = false
				break
			}
		}
		if allWS {
			parts[0] = &ast.RawStringPart{Val: raw.Val[idx+1:]}
		}
	} else {
		// No newline: strip leading whitespace only
		trimmed := strings.TrimLeft(raw.Val, " \t")
		parts[0] = &ast.RawStringPart{Val: trimmed}
	}
	return parts
}

// stripBeforeClose removes whitespace before >>> up to and including the preceding newline (if any).
// If there is no newline, only trailing whitespace is removed.
func stripBeforeClose(parts []ast.StringPart) []ast.StringPart {
	if len(parts) == 0 {
		return parts
	}
	// Find the last RawStringPart
	for i := len(parts) - 1; i >= 0; i-- {
		raw, ok := parts[i].(*ast.RawStringPart)
		if !ok {
			continue
		}
		idx := strings.LastIndexByte(raw.Val, '\n')
		if idx >= 0 {
			// Check if everything after the last \n is whitespace
			trailing := raw.Val[idx+1:]
			allWhitespace := true
			for _, c := range trailing {
				if c != ' ' && c != '\t' {
					allWhitespace = false
					break
				}
			}
			if allWhitespace {
				// Remove the newline and trailing whitespace
				parts[i] = &ast.RawStringPart{Val: raw.Val[:idx]}
			}
		} else {
			// No newline: strip trailing whitespace only
			trimmed := strings.TrimRight(raw.Val, " \t")
			parts[i] = &ast.RawStringPart{Val: trimmed}
		}
		break
	}
	return parts
}

// removeCommonIndent calculates the minimum leading whitespace across all non-blank lines
// and removes it from each line. Lines are tracked across part boundaries.
func removeCommonIndent(parts []ast.StringPart) []ast.StringPart {
	// First pass: find common indent by examining all lines across parts.
	// A "line" starts at the beginning or after a \n in a RawStringPart.
	// We only measure indent from RawStringPart content at line beginnings.

	minIndent := -1 // -1 means not yet set
	atLineStart := true
	currentIndent := 0
	lineHasContent := false

	for _, p := range parts {
		raw, ok := p.(*ast.RawStringPart)
		if !ok {
			// Placeholder at line start means indent is whatever we've accumulated
			if atLineStart {
				lineHasContent = true
				if minIndent == -1 || currentIndent < minIndent {
					minIndent = currentIndent
				}
				atLineStart = false
			}
			continue
		}
		for _, c := range raw.Val {
			if c == '\n' {
				// End of line. If line had content, its indent was already recorded.
				// If line was blank (atLineStart && !lineHasContent), skip it.
				if atLineStart && lineHasContent {
					// This shouldn't happen normally
				}
				atLineStart = true
				currentIndent = 0
				lineHasContent = false
			} else if atLineStart && (c == ' ' || c == '\t') {
				currentIndent++
			} else {
				if atLineStart {
					lineHasContent = true
					if minIndent == -1 || currentIndent < minIndent {
						minIndent = currentIndent
					}
					atLineStart = false
				}
			}
		}
	}

	if minIndent <= 0 {
		return parts
	}

	// Second pass: remove minIndent chars from the beginning of each line
	result := make([]ast.StringPart, 0, len(parts))
	atLineStart = true
	charsToStrip := minIndent

	for _, p := range parts {
		raw, ok := p.(*ast.RawStringPart)
		if !ok {
			atLineStart = false
			charsToStrip = 0
			result = append(result, p)
			continue
		}

		var sb strings.Builder
		sb.Grow(len(raw.Val))
		for _, c := range raw.Val {
			if c == '\n' {
				sb.WriteByte('\n')
				atLineStart = true
				charsToStrip = minIndent
			} else if atLineStart && charsToStrip > 0 && (c == ' ' || c == '\t') {
				charsToStrip--
				if charsToStrip == 0 {
					atLineStart = false
				}
			} else {
				atLineStart = false
				charsToStrip = 0
				sb.WriteRune(c)
			}
		}
		result = append(result, &ast.RawStringPart{Val: sb.String()})
	}

	return result
}
