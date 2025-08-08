package parser

import (
	"github.com/bioflowy/flowy/pkg/expr"
	"github.com/bioflowy/flowy/pkg/tree"
)

// parseDeclaration parses any declaration according to WDL grammar:
// ?any_decl: unbound_decl | bound_decl
func (p *Parser) parseDeclaration() (*tree.Decl, bool) {
	pos := p.currentPosition()

	// Parse optional env modifier for task declarations
	// isEnv := false
	if p.currentTokenIs(TokenEnv) {
		// isEnv = true
		p.nextToken()
		// Note: We'll store this information but pkg/tree doesn't currently 
		// have a field for env declarations, so this is for future extension
	}

	// Parse type
	declType, ok := p.parseType()
	if !ok {
		return nil, false
	}

	// Parse identifier
	name, ok := p.parseIdentifier()
	if !ok {
		return nil, false
	}

	// Check for initialization expression
	var initExpr expr.Expr
	if p.currentTokenIs(TokenAssign) {
		p.nextToken() // consume '='
		
		// Parse initialization expression
		// Note: parseExpression will be implemented in expressions.go
		initExpr, ok = p.parseExpression()
		if !ok {
			// For now, create a placeholder
			p.addError(NewParseError(
				p.currentPosition(),
				"expression parsing not yet implemented",
				[]TokenType{},
				p.currentToken,
			))
			return nil, false
		}
	}

	return tree.NewDecl(name, declType, initExpr, pos), true
}

// parseUnboundDeclaration parses an unbound declaration:
// unbound_decl: type CNAME -> decl
func (p *Parser) parseUnboundDeclaration() (*tree.Decl, bool) {
	pos := p.currentPosition()

	// Parse type
	declType, ok := p.parseType()
	if !ok {
		return nil, false
	}

	// Parse identifier
	name, ok := p.parseIdentifier()
	if !ok {
		return nil, false
	}

	// Unbound declarations should not have assignments
	if p.currentTokenIs(TokenAssign) {
		p.addError(NewParseError(
			p.currentPosition(),
			"unbound declaration cannot have initialization",
			[]TokenType{},
			p.currentToken,
		))
		return nil, false
	}

	// No initialization for unbound declarations
	return tree.NewDecl(name, declType, nil, pos), true
}

// parseBoundDeclaration parses a bound declaration:
// bound_decl: type CNAME "=" expr -> decl
func (p *Parser) parseBoundDeclaration() (*tree.Decl, bool) {
	pos := p.currentPosition()

	// Parse type
	declType, ok := p.parseType()
	if !ok {
		return nil, false
	}

	// Parse identifier
	name, ok := p.parseIdentifier()
	if !ok {
		return nil, false
	}

	// Expect assignment
	if !p.consume(TokenAssign) {
		return nil, false
	}

	// Parse initialization expression
	initExpr, ok := p.parseExpression()
	if !ok {
		return nil, false
	}

	return tree.NewDecl(name, declType, initExpr, pos), true
}

// parseInputDeclarations parses input declarations:
// input_decls: "input" "{" any_decl* "}"
func (p *Parser) parseInputDeclarations() (*tree.Input, bool) {
	pos := p.currentPosition()

	if !p.consume(TokenInput) {
		return nil, false
	}

	var declarations []*tree.Decl
	ok := p.parseBlock(func() bool {
		for !p.currentTokenIs(TokenRightBrace) && !p.IsAtEnd() {
			if decl, ok := p.parseDeclaration(); ok {
				declarations = append(declarations, decl)
			} else {
				// Try to recover
				p.synchronize()
				break
			}
			
			p.skipCommentsAndNewlines()
		}
		return true
	})

	if !ok {
		return nil, false
	}

	return tree.NewInput(declarations, pos), true
}

// parseTaskInputDeclarations parses task input declarations with optional env:
// task_input_decls: "input" "{" task_env_decl* "}" -> input_decls
// task_env_decl: ENV? any_decl
func (p *Parser) parseTaskInputDeclarations() (*tree.Input, bool) {
	// For now, this is the same as regular input declarations
	// The env modifier is parsed but not yet stored in the tree structure
	return p.parseInputDeclarations()
}

// parseOutputDeclarations parses output declarations:
// output_decls: "output" "{" bound_decl* "}"
func (p *Parser) parseOutputDeclarations() (*tree.Output, bool) {
	pos := p.currentPosition()

	if !p.consume(TokenOutput) {
		return nil, false
	}

	var declarations []*tree.Decl
	ok := p.parseBlock(func() bool {
		for !p.currentTokenIs(TokenRightBrace) && !p.IsAtEnd() {
			// Output declarations must be bound (have initialization)
			if decl, ok := p.parseBoundDeclaration(); ok {
				declarations = append(declarations, decl)
			} else {
				// Try to recover
				p.synchronize()
				break
			}
			
			p.skipCommentsAndNewlines()
		}
		return true
	})

	if !ok {
		return nil, false
	}

	return tree.NewOutput(declarations, pos), true
}

// parseStruct parses a struct definition:
// struct: "struct" CNAME "{" unbound_decl* "}"
func (p *Parser) parseStruct() (*tree.StructTypeDef, bool) {
	pos := p.currentPosition()

	if !p.consume(TokenStruct) {
		return nil, false
	}

	// Parse struct name
	name, ok := p.parseIdentifier()
	if !ok {
		return nil, false
	}

	var members []*tree.StructMember
	ok = p.parseBlock(func() bool {
		for !p.currentTokenIs(TokenRightBrace) && !p.IsAtEnd() {
			// Parse struct member (unbound declaration)
			if member, ok := p.parseStructMember(); ok {
				members = append(members, member)
			} else {
				// Try to recover
				p.synchronize()
				break
			}
			
			p.skipCommentsAndNewlines()
		}
		return true
	})

	if !ok {
		return nil, false
	}

	return tree.NewStructTypeDef(name, members, pos), true
}

// parseStructMember parses a struct member (unbound declaration)
func (p *Parser) parseStructMember() (*tree.StructMember, bool) {
	pos := p.currentPosition()

	// Parse type
	memberType, ok := p.parseType()
	if !ok {
		return nil, false
	}

	// Parse member name
	name, ok := p.parseIdentifier()
	if !ok {
		return nil, false
	}

	return tree.NewStructMember(name, memberType, pos), true
}

// parseDeclarationList parses a list of declarations
func (p *Parser) parseDeclarationList(terminator TokenType) ([]*tree.Decl, bool) {
	var declarations []*tree.Decl

	for !p.currentTokenIs(terminator) && !p.IsAtEnd() {
		if decl, ok := p.parseDeclaration(); ok {
			declarations = append(declarations, decl)
		} else {
			return nil, false
		}
		
		p.skipCommentsAndNewlines()
	}

	return declarations, true
}

// parseUnboundDeclarationList parses a list of unbound declarations
func (p *Parser) parseUnboundDeclarationList(terminator TokenType) ([]*tree.Decl, bool) {
	var declarations []*tree.Decl

	for !p.currentTokenIs(terminator) && !p.IsAtEnd() {
		if decl, ok := p.parseUnboundDeclaration(); ok {
			declarations = append(declarations, decl)
		} else {
			return nil, false
		}
		
		p.skipCommentsAndNewlines()
	}

	return declarations, true
}

// parseBoundDeclarationList parses a list of bound declarations
func (p *Parser) parseBoundDeclarationList(terminator TokenType) ([]*tree.Decl, bool) {
	var declarations []*tree.Decl

	for !p.currentTokenIs(terminator) && !p.IsAtEnd() {
		if decl, ok := p.parseBoundDeclaration(); ok {
			declarations = append(declarations, decl)
		} else {
			return nil, false
		}
		
		p.skipCommentsAndNewlines()
	}

	return declarations, true
}

// parseStructMemberList parses a list of struct members
func (p *Parser) parseStructMemberList(terminator TokenType) ([]*tree.StructMember, bool) {
	var members []*tree.StructMember

	for !p.currentTokenIs(terminator) && !p.IsAtEnd() {
		if member, ok := p.parseStructMember(); ok {
			members = append(members, member)
		} else {
			return nil, false
		}
		
		p.skipCommentsAndNewlines()
	}

	return members, true
}

// isDeclarationStart returns true if current token can start a declaration
func (p *Parser) isDeclarationStart() bool {
	// Declarations start with types or env keyword
	return p.isTypeStart() || p.currentTokenIs(TokenEnv)
}

// parseVariableDeclaration parses a variable declaration in various contexts
func (p *Parser) parseVariableDeclaration() (*tree.Decl, bool) {
	return p.parseDeclaration()
}

// validateDeclarationName validates that a declaration name is valid
func (p *Parser) validateDeclarationName(name string) bool {
	// Check if it's a valid identifier
	if !isValidIdentifier(name) {
		return false
	}

	// Check if it conflicts with reserved keywords
	switch name {
	case "true", "false", "None":
		return false // These are literal values
	case "version", "import", "as", "alias", "workflow", "task", 
		 "scatter", "if", "then", "else", "call", "struct":
		return false // These are core language keywords that cannot be variable names
	default:
		return true
	}
}
