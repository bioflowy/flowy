package parser

import (
	"github.com/bioflowy/flowy/pkg/expr"
	"github.com/bioflowy/flowy/pkg/tree"
	"github.com/bioflowy/flowy/pkg/values"
)

// parseTask parses a WDL task according to the grammar:
// task: "task" CNAME "{" task_section* command task_section* "}"
func (p *Parser) parseTask() (*tree.Task, bool) {
	pos := p.currentPosition()

	if !p.consume(TokenTask) {
		return nil, false
	}

	// Parse task name
	taskName, ok := p.parseIdentifier()
	if !ok {
		return nil, false
	}

	// Parse task body
	var inputs []*tree.Decl
	var outputs []*tree.Decl
	var command expr.Expr
	runtime := make(map[string]expr.Expr)
	meta := make(map[string]interface{})
	parameterMeta := make(map[string]interface{})

	ok = p.parseBlock(func() bool {
		// Parse task sections and command
		for !p.currentTokenIs(TokenRightBrace) && !p.IsAtEnd() {
			p.skipCommentsAndNewlines()

			switch p.currentToken.Type {
			case TokenInput:
				if inputSection, ok := p.parseTaskInputDeclarations(); ok {
					inputs = append(inputs, inputSection.Decls...)
				} else {
					return false
				}

			case TokenOutput:
				if outputSection, ok := p.parseOutputDeclarations(); ok {
					outputs = append(outputs, outputSection.Decls...)
				} else {
					return false
				}

			case TokenCommand:
				if cmd, ok := p.parseTaskCommand(); ok {
					command = cmd
				} else {
					return false
				}

			case TokenMeta:
				if metaData, ok := p.parseMetaSection(); ok {
					for k, v := range metaData {
						meta[k] = v
					}
				} else {
					return false
				}

			case TokenParameterMeta:
				if paramData, ok := p.parseParameterMetaSection(); ok {
					for k, v := range paramData {
						parameterMeta[k] = v
					}
				} else {
					return false
				}

			case TokenRequirements, TokenRuntime:
				if reqData, ok := p.parseRequirementsSection(); ok {
					for k, v := range reqData {
						runtime[k] = v
					}
				} else {
					return false
				}

			default:
				// Try to parse as a non-input declaration (env declarations)
				if p.isDeclarationStart() {
					if decl, ok := p.parseDeclaration(); ok {
						// This is a non-input declaration, could be added to a separate list
						// For now, we'll skip it as pkg/tree doesn't have a specific place for it
						_ = decl
					} else {
						return false
					}
				} else {
					p.addError(NewParseError(
						p.currentPosition(),
						"unexpected token in task body",
						[]TokenType{TokenInput, TokenOutput, TokenCommand, TokenMeta, TokenParameterMeta, TokenRequirements, TokenRuntime},
						p.currentToken,
					))
					p.synchronize()
					return false
				}
			}

			p.skipCommentsAndNewlines()
		}

		return true
	})

	if !ok {
		return nil, false
	}

	// Create task with command expression
	var taskCommand *tree.TaskCommand
	if command != nil {
		taskCommand = tree.NewTaskCommand(command, pos)
	}

	task := tree.NewTask(taskName, inputs, outputs, taskCommand, pos)

	// Set metadata
	if len(runtime) > 0 {
		task.Runtime = runtime
	}
	if len(meta) > 0 {
		task.Meta = meta
	}
	if len(parameterMeta) > 0 {
		task.ParameterMeta = parameterMeta
	}

	return task, true
}

// parseTaskCommand parses a task command:
// ?command: "command" (command1 | command2)
func (p *Parser) parseTaskCommand() (expr.Expr, bool) {
	if !p.consume(TokenCommand) {
		return nil, false
	}

	// Parse the command block
	return p.parseCommandBlock()
}

// parseMetaSection parses a meta section:
// meta_section: ("meta" | "parameter_meta") meta_object
func (p *Parser) parseMetaSection() (map[string]interface{}, bool) {
	if !p.consume(TokenMeta) {
		return nil, false
	}

	return p.parseMetaObject()
}

// parseParameterMetaSection parses a parameter_meta section
func (p *Parser) parseParameterMetaSection() (map[string]interface{}, bool) {
	if !p.consume(TokenParameterMeta) {
		return nil, false
	}

	return p.parseMetaObject()
}

// parseMetaObject parses a meta object:
// meta_object: "{" [meta_kv (","? meta_kv)*] ","? "}"
func (p *Parser) parseMetaObject() (map[string]interface{}, bool) {
	if !p.consume(TokenLeftBrace) {
		return nil, false
	}

	p.skipCommentsAndNewlines()

	meta := make(map[string]interface{})

	// Handle empty meta object
	if p.currentTokenIs(TokenRightBrace) {
		p.nextToken()
		return meta, true
	}

	// Parse first key-value pair
	key, value, ok := p.parseMetaKeyValue()
	if !ok {
		return nil, false
	}
	meta[key] = value

	// Parse remaining key-value pairs
	for {
		p.skipCommentsAndNewlines()
		
		// Check if we're done
		if p.currentTokenIs(TokenRightBrace) {
			break
		}
		
		// Optional comma before next key-value pair
		if p.currentTokenIs(TokenComma) {
			p.nextToken() // consume comma
			p.skipCommentsAndNewlines()
			
			// Check for trailing comma
			if p.currentTokenIs(TokenRightBrace) {
				break
			}
		}
		
		// Try to parse another key-value pair
		if p.currentTokenIs(TokenIdentifier) || p.canBeMetaKey(p.currentToken.Type) {
			key, value, ok := p.parseMetaKeyValue()
			if !ok {
				return nil, false
			}
			meta[key] = value
		} else {
			break
		}
	}

	p.skipCommentsAndNewlines()

	if !p.consume(TokenRightBrace) {
		return nil, false
	}

	return meta, true
}

// parseMetaKeyValue parses meta_kv: CNAME ":" meta_value
func (p *Parser) parseMetaKeyValue() (string, interface{}, bool) {
	// Parse key - can be identifier or many keywords
	var key string
	if p.currentTokenIs(TokenIdentifier) {
		key = p.currentToken.Value
		p.nextToken()
	} else if p.canBeMetaKey(p.currentToken.Type) {
		key = p.currentToken.Value
		p.nextToken()
	} else {
		p.addError(p.expectError(TokenIdentifier))
		return "", nil, false
	}

	if !p.consume(TokenColon) {
		return "", nil, false
	}

	// Parse value
	value, ok := p.parseMetaValue()
	if !ok {
		return "", nil, false
	}

	return key, value, true
}

// parseMetaValue parses meta_value:
// ?meta_value: literal | string_literal
//            | meta_object
//            | "[" [meta_value ("," meta_value)*] ","? "]" -> meta_array
func (p *Parser) parseMetaValue() (interface{}, bool) {
	switch p.currentToken.Type {
	case TokenLeftBrace:
		// Nested meta object
		return p.parseMetaObject()

	case TokenLeftBracket:
		// Meta array
		return p.parseMetaArray()

	case TokenString:
		// String literal
		value := p.currentToken.Value
		p.nextToken()
		return value, true

	default:
		// Try to parse as literal
		if p.isLiteralToken() {
			if literal, ok := p.parseAnyLiteral(); ok {
				// Convert expr.Expr to Go value
				if lit, ok := literal.Literal(); ok {
					switch v := lit.(type) {
					case *values.BooleanValue:
						return v.Value().(bool), true
					case *values.IntValue:
						return v.Value().(int64), true
					case *values.FloatValue:
						return v.Value().(float64), true
					case *values.StringValue:
						return v.Value().(string), true
					case *values.Null:
						return nil, true
					default:
						return lit, true
					}
				}
			}
		}

		p.addError(NewParseError(
			p.currentPosition(),
			"expected meta value",
			[]TokenType{TokenLeftBrace, TokenLeftBracket, TokenString, TokenInt, TokenFloat, TokenBool, TokenNone},
			p.currentToken,
		))
		return nil, false
	}
}

// parseMetaArray parses a meta array: "[" [meta_value ("," meta_value)*] ","? "]"
func (p *Parser) parseMetaArray() ([]interface{}, bool) {
	if !p.consume(TokenLeftBracket) {
		return nil, false
	}

	var values []interface{}

	// Handle empty array
	if p.currentTokenIs(TokenRightBracket) {
		p.nextToken()
		return values, true
	}

	// Parse first value
	firstValue, ok := p.parseMetaValue()
	if !ok {
		return nil, false
	}
	values = append(values, firstValue)

	// Parse remaining values
	for p.currentTokenIs(TokenComma) {
		p.nextToken() // consume comma

		// Check for trailing comma
		if p.currentTokenIs(TokenRightBracket) {
			break
		}

		value, ok := p.parseMetaValue()
		if !ok {
			return nil, false
		}
		values = append(values, value)
	}

	if !p.consume(TokenRightBracket) {
		return nil, false
	}

	return values, true
}

// parseRequirementsSection parses a requirements section:
// requirements_section: ("requirements" | "runtime") "{" [runtime_kv (","? runtime_kv)*] "}"
func (p *Parser) parseRequirementsSection() (map[string]expr.Expr, bool) {
	if !p.currentTokenIs(TokenRequirements) && !p.currentTokenIs(TokenRuntime) {
		return nil, false
	}
	p.nextToken() // consume requirements or runtime

	if !p.consume(TokenLeftBrace) {
		return nil, false
	}

	requirements := make(map[string]expr.Expr)

	// Handle empty requirements
	if p.currentTokenIs(TokenRightBrace) {
		p.nextToken()
		return requirements, true
	}

	// Parse first key-value pair
	key, value, ok := p.parseRuntimeKeyValue()
	if !ok {
		return nil, false
	}
	requirements[key] = value

	// Parse remaining key-value pairs
	for p.currentTokenIs(TokenComma) {
		p.nextToken() // consume comma

		// Check for trailing comma
		if p.currentTokenIs(TokenRightBrace) {
			break
		}

		key, value, ok := p.parseRuntimeKeyValue()
		if !ok {
			return nil, false
		}
		requirements[key] = value
	}

	if !p.consume(TokenRightBrace) {
		return nil, false
	}

	return requirements, true
}

// parseRuntimeKeyValue parses runtime_kv: CNAME ":" expr
func (p *Parser) parseRuntimeKeyValue() (string, expr.Expr, bool) {
	// Parse key
	key, ok := p.parseIdentifier()
	if !ok {
		return "", nil, false
	}

	if !p.consume(TokenColon) {
		return "", nil, false
	}

	// Parse value expression
	value, ok := p.parseExpression()
	if !ok {
		return "", nil, false
	}

	return key, value, true
}

// parseTaskSection parses any task section
func (p *Parser) parseTaskSection() bool {
	switch p.currentToken.Type {
	case TokenInput:
		_, ok := p.parseTaskInputDeclarations()
		return ok
	case TokenOutput:
		_, ok := p.parseOutputDeclarations()
		return ok
	case TokenMeta:
		_, ok := p.parseMetaSection()
		return ok
	case TokenParameterMeta:
		_, ok := p.parseParameterMetaSection()
		return ok
	case TokenRequirements, TokenRuntime:
		_, ok := p.parseRequirementsSection()
		return ok
	case TokenCommand:
		_, ok := p.parseTaskCommand()
		return ok
	default:
		p.addError(NewParseError(
			p.currentPosition(),
			"expected task section",
			[]TokenType{TokenInput, TokenOutput, TokenMeta, TokenParameterMeta, TokenRequirements, TokenRuntime, TokenCommand},
			p.currentToken,
		))
		return false
	}
}

// isTaskSectionStart returns true if current token can start a task section
func (p *Parser) isTaskSectionStart() bool {
	switch p.currentToken.Type {
	case TokenInput, TokenOutput, TokenMeta, TokenParameterMeta,
		 TokenRequirements, TokenRuntime, TokenCommand:
		return true
	default:
		return false
	}
}

// validateTaskStructure performs basic validation of task structure
func (p *Parser) validateTaskStructure(task *tree.Task) bool {
	// Task must have a command
	if task.Command == nil {
		p.addError(NewParseError(
			task.SourcePosition(),
			"task must have a command section",
			[]TokenType{TokenCommand},
			Token{Type: TokenEOF},
		))
		return false
	}

	return true
}

// parseTaskList parses a list of tasks
func (p *Parser) parseTaskList() ([]*tree.Task, bool) {
	var tasks []*tree.Task

	for p.currentTokenIs(TokenTask) {
		if task, ok := p.parseTask(); ok {
			tasks = append(tasks, task)
		} else {
			return nil, false
		}
		
		p.skipCommentsAndNewlines()
	}

	return tasks, true
}

// canBeMetaKey returns true if a token can be used as a meta key
func (p *Parser) canBeMetaKey(tokenType TokenType) bool {
	switch tokenType {
	case TokenVersion, TokenTask, TokenWorkflow, TokenCall, TokenInput, TokenOutput, 
		 TokenMeta, TokenStruct, TokenCommand, TokenEnv, TokenFile, TokenDirectory,
		 TokenStringType, TokenIntType, TokenFloatType, TokenBoolType:
		// Many keywords can be used as meta keys
		return true
	default:
		return false
	}
}