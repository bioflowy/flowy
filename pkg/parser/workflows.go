package parser

import (
	"github.com/bioflowy/flowy/pkg/expr"
	"github.com/bioflowy/flowy/pkg/tree"
)

// parseWorkflow parses a WDL workflow according to the grammar:
// workflow: "workflow" CNAME "{" workflow_element* "}"
func (p *Parser) parseWorkflow() (*tree.Workflow, bool) {
	pos := p.currentPosition()

	if !p.consume(TokenWorkflow) {
		return nil, false
	}

	// Parse workflow name
	workflowName, ok := p.parseIdentifier()
	if !ok {
		return nil, false
	}

	// Parse workflow body
	var inputs []*tree.Decl
	var outputs []*tree.Decl
	var body []tree.WorkflowNode

	ok = p.parseBlock(func() bool {
		for !p.currentTokenIs(TokenRightBrace) && !p.isAtEnd() {
			p.skipCommentsAndNewlines()

			if element, ok := p.parseWorkflowElement(); ok {
				switch elem := element.(type) {
				case *tree.Input:
					inputs = append(inputs, elem.Decls...)
				case *tree.Output:
					outputs = append(outputs, elem.Decls...)
				case tree.WorkflowNode:
					body = append(body, elem)
				}
			} else {
				// Try to recover
				p.synchronize()
				return false
			}

			p.skipCommentsAndNewlines()
		}
		return true
	})

	if !ok {
		return nil, false
	}

	return tree.NewWorkflow(workflowName, inputs, body, outputs, pos), true
}

// parseWorkflowElement parses any workflow element:
// ?workflow_element: input_decls | any_decl | call | scatter | conditional | workflow_outputs | meta_section
func (p *Parser) parseWorkflowElement() (interface{}, bool) {
	switch p.currentToken.Type {
	case TokenInput:
		return p.parseInputDeclarations()
	case TokenOutput:
		return p.parseOutputDeclarations()
	case TokenCall:
		return p.parseCall()
	case TokenScatter:
		return p.parseScatter()
	case TokenIf:
		return p.parseConditional()
	case TokenMeta, TokenParameterMeta:
		// Meta sections in workflows - for now we'll skip them
		// as pkg/tree doesn't have workflow-level metadata
		_, ok := p.parseMetaSection()
		if !ok {
			return nil, false
		}
		// Return a dummy value to indicate success but nothing to add
		return nil, true
	default:
		// Try to parse as a declaration
		if p.isDeclarationStart() {
			if decl, ok := p.parseDeclaration(); ok {
				return decl, true
			}
		}

		p.addError(NewParseError(
			p.currentPosition(),
			"expected workflow element",
			[]TokenType{TokenInput, TokenOutput, TokenCall, TokenScatter, TokenIf, TokenMeta},
			p.currentToken,
		))
		return nil, false
	}
}

// parseCall parses a workflow call:
// call: "call" namespaced_ident ("after" CNAME)* _call_body? -> call
//     | "call" namespaced_ident "as" CNAME ("after" CNAME)* _call_body? -> call_as
func (p *Parser) parseCall() (*tree.Call, bool) {
	pos := p.currentPosition()

	if !p.consume(TokenCall) {
		return nil, false
	}

	// Parse callee name (namespaced identifier)
	callee, ok := p.parseNamespacedIdentifier()
	if !ok {
		return nil, false
	}

	// Check for alias
	var alias *string
	if p.currentTokenIs(TokenAs) {
		p.nextToken() // consume "as"
		
		aliasName, ok := p.parseIdentifier()
		if !ok {
			return nil, false
		}
		alias = &aliasName
	}

	// Parse "after" dependencies
	var afterDeps []string
	for p.currentTokenIs(TokenAfter) {
		p.nextToken() // consume "after"
		
		depName, ok := p.parseIdentifier()
		if !ok {
			return nil, false
		}
		afterDeps = append(afterDeps, depName)
	}

	// Parse optional call body (inputs)
	inputs := make(map[string]expr.Expr)
	if p.currentTokenIs(TokenLeftBrace) {
		if callInputs, ok := p.parseCallBody(); ok {
			inputs = callInputs
		} else {
			return nil, false
		}
	}

	// Create call node
	callName := callee
	if alias != nil {
		callName = *alias
	}

	call := tree.NewCall(callName, callee, alias, inputs, pos)
	call.SetAfter(afterDeps)

	return call, true
}

// parseCallBody parses the call body:
// _call_body: "{" call_inputs? "}"
func (p *Parser) parseCallBody() (map[string]expr.Expr, bool) {
	if !p.consume(TokenLeftBrace) {
		return nil, false
	}

	inputs, ok := p.parseCallInputs()
	if !ok {
		return nil, false
	}

	if !p.consume(TokenRightBrace) {
		return nil, false
	}

	return inputs, true
}

// parseCallInputs parses call inputs:
// call_inputs: input_colon? [call_input ("," call_input)*] ","?
func (p *Parser) parseCallInputs() (map[string]expr.Expr, bool) {
	inputs := make(map[string]expr.Expr)

	// Handle optional "input:" prefix
	if p.currentTokenIs(TokenInput) && p.peekTokenIs(TokenColon) {
		p.nextToken() // consume "input"
		p.nextToken() // consume ":"
	}

	// Handle empty input list
	if p.currentTokenIs(TokenRightBrace) {
		return inputs, true
	}

	// Parse first input
	key, value, ok := p.parseCallInput()
	if !ok {
		return nil, false
	}
	inputs[key] = value

	// Parse remaining inputs
	for p.currentTokenIs(TokenComma) {
		p.nextToken() // consume comma

		// Check for trailing comma
		if p.currentTokenIs(TokenRightBrace) {
			break
		}

		key, value, ok := p.parseCallInput()
		if !ok {
			return nil, false
		}
		inputs[key] = value
	}

	return inputs, true
}

// parseCallInput parses a single call input:
// call_input: CNAME ["=" expr]
func (p *Parser) parseCallInput() (string, expr.Expr, bool) {
	// Parse input name
	inputName, ok := p.parseIdentifier()
	if !ok {
		return "", nil, false
	}

	// Check for value assignment
	if p.currentTokenIs(TokenAssign) {
		p.nextToken() // consume "="
		
		value, ok := p.parseExpression()
		if !ok {
			return "", nil, false
		}
		
		return inputName, value, true
	} else {
		// No explicit value - create identifier reference
		pos := p.currentPosition()
		return inputName, expr.NewIdentifier(inputName, pos), true
	}
}

// parseScatter parses a scatter block:
// scatter: "scatter" "(" CNAME "in" expr ")" "{" inner_workflow_element* "}"
func (p *Parser) parseScatter() (*tree.Scatter, bool) {
	pos := p.currentPosition()

	if !p.consume(TokenScatter) {
		return nil, false
	}

	if !p.consume(TokenLeftParen) {
		return nil, false
	}

	// Parse scatter variable
	variable, ok := p.parseIdentifier()
	if !ok {
		return nil, false
	}

	// Parse "in"
	if !p.currentTokenIs(TokenIdentifier) || p.currentToken.Value != "in" {
		p.addError(NewParseError(
			p.currentPosition(),
			"expected 'in' in scatter statement",
			[]TokenType{TokenIdentifier},
			p.currentToken,
		))
		return nil, false
	}
	p.nextToken() // consume "in"

	// Parse collection expression
	collection, ok := p.parseExpression()
	if !ok {
		return nil, false
	}

	if !p.consume(TokenRightParen) {
		return nil, false
	}

	// Parse scatter body
	var body []tree.WorkflowNode

	ok = p.parseBlock(func() bool {
		for !p.currentTokenIs(TokenRightBrace) && !p.isAtEnd() {
			p.skipCommentsAndNewlines()

			if element, ok := p.parseInnerWorkflowElement(); ok {
				if element != nil {
					body = append(body, element)
				}
			} else {
				return false
			}

			p.skipCommentsAndNewlines()
		}
		return true
	})

	if !ok {
		return nil, false
	}

	return tree.NewScatter(variable, collection, body, pos), true
}

// parseConditional parses a conditional block:
// conditional: "if" "(" expr ")" "{" inner_workflow_element* "}"
func (p *Parser) parseConditional() (*tree.Conditional, bool) {
	pos := p.currentPosition()

	if !p.consume(TokenIf) {
		return nil, false
	}

	if !p.consume(TokenLeftParen) {
		return nil, false
	}

	// Parse condition expression
	condition, ok := p.parseExpression()
	if !ok {
		return nil, false
	}

	if !p.consume(TokenRightParen) {
		return nil, false
	}

	// Parse conditional body
	var body []tree.WorkflowNode

	ok = p.parseBlock(func() bool {
		for !p.currentTokenIs(TokenRightBrace) && !p.isAtEnd() {
			p.skipCommentsAndNewlines()

			if element, ok := p.parseInnerWorkflowElement(); ok {
				if element != nil {
					body = append(body, element)
				}
			} else {
				return false
			}

			p.skipCommentsAndNewlines()
		}
		return true
	})

	if !ok {
		return nil, false
	}

	return tree.NewConditional(condition, body, pos), true
}

// parseInnerWorkflowElement parses inner workflow elements:
// ?inner_workflow_element: any_decl | call | scatter | conditional
func (p *Parser) parseInnerWorkflowElement() (tree.WorkflowNode, bool) {
	switch p.currentToken.Type {
	case TokenCall:
		return p.parseCall()
	case TokenScatter:
		return p.parseScatter()
	case TokenIf:
		return p.parseConditional()
	default:
		// Try to parse as a declaration
		if p.isDeclarationStart() {
			return p.parseDeclaration()
		}

		p.addError(NewParseError(
			p.currentPosition(),
			"expected inner workflow element",
			[]TokenType{TokenCall, TokenScatter, TokenIf},
			p.currentToken,
		))
		return nil, false
	}
}

// isWorkflowElementStart returns true if current token can start a workflow element
func (p *Parser) isWorkflowElementStart() bool {
	switch p.currentToken.Type {
	case TokenInput, TokenOutput, TokenCall, TokenScatter, TokenIf, TokenMeta, TokenParameterMeta:
		return true
	default:
		return p.isDeclarationStart()
	}
}

// isInnerWorkflowElementStart returns true if current token can start an inner workflow element
func (p *Parser) isInnerWorkflowElementStart() bool {
	switch p.currentToken.Type {
	case TokenCall, TokenScatter, TokenIf:
		return true
	default:
		return p.isDeclarationStart()
	}
}

// parseWorkflowList parses a list of workflows
func (p *Parser) parseWorkflowList() ([]*tree.Workflow, bool) {
	var workflows []*tree.Workflow

	for p.currentTokenIs(TokenWorkflow) {
		if workflow, ok := p.parseWorkflow(); ok {
			workflows = append(workflows, workflow)
		} else {
			return nil, false
		}
		
		p.skipCommentsAndNewlines()
	}

	return workflows, true
}

// validateWorkflowStructure performs basic validation of workflow structure
func (p *Parser) validateWorkflowStructure(workflow *tree.Workflow) bool {
	// Basic validation - workflows should have at least some body elements
	if len(workflow.Body) == 0 {
		p.addError(NewParseError(
			workflow.SourcePosition(),
			"workflow body is empty",
			[]TokenType{},
			Token{Type: TokenEOF},
		))
		return false
	}

	return true
}

// parseCallList parses multiple call statements
func (p *Parser) parseCallList() ([]*tree.Call, bool) {
	var calls []*tree.Call

	for p.currentTokenIs(TokenCall) {
		if call, ok := p.parseCall(); ok {
			calls = append(calls, call)
		} else {
			return nil, false
		}
		
		p.skipCommentsAndNewlines()
	}

	return calls, true
}