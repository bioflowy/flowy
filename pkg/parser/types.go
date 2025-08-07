package parser

import (
	"github.com/bioflowy/flowy/pkg/types"
)

// parseType parses a WDL type according to the grammar:
// type: CNAME _quant?
//       | CNAME "[" type ["," type] "]" _quant?
func (p *Parser) parseType() (types.Base, bool) {
	// Parse base type name
	typeName, ok := p.parseTypeName()
	if !ok {
		return nil, false
	}

	// Check for type parameters (e.g., Array[Int], Map[String, Int])
	var baseType types.Base
	if p.currentTokenIs(TokenLeftBracket) {
		baseType, ok = p.parseParameterizedType(typeName)
		if !ok {
			return nil, false
		}
	} else {
		baseType, ok = p.createPrimitiveType(typeName)
		if !ok {
			return nil, false
		}
	}

	// Parse optional quantifiers
	optional, nonempty := p.parseQuantifiers()

	return p.applyQuantifiers(baseType, optional, nonempty), true
}

// parseTypeName parses a type name (identifier or type keyword)
func (p *Parser) parseTypeName() (string, bool) {
	var typeName string

	switch p.currentToken.Type {
	// Primitive type keywords
	case TokenIntType:
		typeName = "Int"
	case TokenFloatType:
		typeName = "Float"
	case TokenStringType:
		typeName = "String"
	case TokenBoolType:
		typeName = "Boolean"
	case TokenFile:
		typeName = "File"
	case TokenDirectory:
		typeName = "Directory"
	// Compound type keywords
	case TokenArray:
		typeName = "Array"
	case TokenMap:
		typeName = "Map"
	case TokenPair:
		typeName = "Pair"
	// Custom type (struct name)
	case TokenIdentifier:
		typeName = p.currentToken.Value
	default:
		p.addError(p.expectError(
			TokenIntType, TokenFloatType, TokenStringType, TokenBoolType,
			TokenFile, TokenDirectory, TokenArray, TokenMap, TokenPair,
			TokenIdentifier,
		))
		return "", false
	}

	p.nextToken()
	return typeName, true
}

// parseParameterizedType parses a type with parameters like Array[T] or Map[K,V]
func (p *Parser) parseParameterizedType(typeName string) (types.Base, bool) {
	if !p.consume(TokenLeftBracket) {
		return nil, false
	}

	switch typeName {
	case "Array":
		return p.parseArrayType()
	case "Map":
		return p.parseMapType()
	case "Pair":
		return p.parsePairType()
	default:
		p.addError(NewParseError(
			p.currentPosition(),
			"type "+typeName+" does not take parameters",
			[]TokenType{TokenRightBracket},
			p.currentToken,
		))
		return nil, false
	}
}

// parseArrayType parses Array[T]
func (p *Parser) parseArrayType() (types.Base, bool) {
	elementType, ok := p.parseType()
	if !ok {
		return nil, false
	}

	if !p.consume(TokenRightBracket) {
		return nil, false
	}

	return types.NewArray(elementType, false, false), true
}

// parseMapType parses Map[K,V]
func (p *Parser) parseMapType() (types.Base, bool) {
	keyType, ok := p.parseType()
	if !ok {
		return nil, false
	}

	if !p.consume(TokenComma) {
		return nil, false
	}

	valueType, ok := p.parseType()
	if !ok {
		return nil, false
	}

	if !p.consume(TokenRightBracket) {
		return nil, false
	}

	return types.NewMap(keyType, valueType, false), true
}

// parsePairType parses Pair[L,R]
func (p *Parser) parsePairType() (types.Base, bool) {
	leftType, ok := p.parseType()
	if !ok {
		return nil, false
	}

	if !p.consume(TokenComma) {
		return nil, false
	}

	rightType, ok := p.parseType()
	if !ok {
		return nil, false
	}

	if !p.consume(TokenRightBracket) {
		return nil, false
	}

	return types.NewPair(leftType, rightType, false), true
}

// createPrimitiveType creates a primitive type from a type name
func (p *Parser) createPrimitiveType(typeName string) (types.Base, bool) {
	switch typeName {
	case "Int":
		return types.NewInt(false), true
	case "Float":
		return types.NewFloat(false), true
	case "String":
		return types.NewString(false), true
	case "Boolean":
		return types.NewBoolean(false), true
	case "File":
		return types.NewFile(false), true
	case "Directory":
		return types.NewDirectory(false), true
	default:
		// Assume it's a struct type
		return types.NewStructInstance(typeName, map[string]types.Base{}, false), true
	}
}

// applyQuantifiers applies quantifiers to a base type
func (p *Parser) applyQuantifiers(baseType types.Base, optional, nonempty bool) types.Base {
	result := baseType

	// Apply nonempty quantifier if needed
	if nonempty {
		// For arrays, apply nonempty constraint
		if arrayType, ok := result.(*types.ArrayType); ok {
			result = types.NewArray(arrayType.ItemType(), false, true) // nonempty = true
		}
		// Note: nonempty doesn't apply to other types in WDL
	}

	// Apply optional quantifier if needed
	if optional {
		result = result.Copy(&optional)
	}

	return result
}

// parseTypeList parses a comma-separated list of types
func (p *Parser) parseTypeList(terminator TokenType) ([]types.Base, bool) {
	var typeList []types.Base

	// Handle empty list
	if p.currentTokenIs(terminator) {
		return typeList, true
	}

	// Parse first type
	firstType, ok := p.parseType()
	if !ok {
		return nil, false
	}
	typeList = append(typeList, firstType)

	// Parse remaining types
	for p.currentTokenIs(TokenComma) {
		p.nextToken() // consume comma

		// Check for trailing comma
		if p.currentTokenIs(terminator) {
			break
		}

		nextType, ok := p.parseType()
		if !ok {
			return nil, false
		}
		typeList = append(typeList, nextType)
	}

	return typeList, true
}

// isTypeStart returns true if current token can start a type
func (p *Parser) isTypeStart() bool {
	return p.isTypeKeyword(p.currentToken.Type) || p.currentTokenIs(TokenIdentifier)
}

// parseOptionalType parses an optional type (may be nil)
func (p *Parser) parseOptionalType() types.Base {
	if p.isTypeStart() {
		if t, ok := p.parseType(); ok {
			return t
		}
	}
	return nil
}

// parseStructMemberType parses a type for struct members (unbound declarations)
func (p *Parser) parseStructMemberType() (types.Base, bool) {
	// Struct members can't have initialization, so just parse the type
	return p.parseType()
}

// parseVariableType parses a type for variable declarations
func (p *Parser) parseVariableType() (types.Base, bool) {
	// Variable declarations can have any type
	return p.parseType()
}

// parseParameterType parses a type for function/task parameters
func (p *Parser) parseParameterType() (types.Base, bool) {
	// Parameter types are just regular types
	return p.parseType()
}

// parseFunctionReturnType parses a return type for functions
func (p *Parser) parseFunctionReturnType() (types.Base, bool) {
	// Function return types are just regular types
	return p.parseType()
}

// validateTypeName validates that a type name is valid
func (p *Parser) validateTypeName(name string) bool {
	// Check if it's a valid identifier
	if !isValidIdentifier(name) {
		return false
	}

	// Check if it's a reserved keyword that shouldn't be used as a type name
	switch name {
	case "version", "import", "as", "alias", "workflow", "task", 
		 "input", "output", "meta", "parameter_meta", "requirements", 
		 "runtime", "scatter", "if", "then", "else", "call", "after",
		 "struct", "command", "env", "left", "right", "object":
		return false
	default:
		return true
	}
}

// parseGenericType parses a potentially generic type (for future extension)
func (p *Parser) parseGenericType() (types.Base, bool) {
	// For now, just parse a regular type
	// This could be extended for generic types in the future
	return p.parseType()
}

// parseCompleteType parses a complete type expression, including all quantifiers
func (p *Parser) parseCompleteType() (types.Base, bool) {
	return p.parseType()
}

// isBuiltinType returns true if the type name is a built-in WDL type
func (p *Parser) isBuiltinType(typeName string) bool {
	switch typeName {
	case "Int", "Float", "String", "Boolean", "File", "Directory",
		 "Array", "Map", "Pair":
		return true
	default:
		return false
	}
}

// getPrimitiveTypeToken returns the token type for a primitive type name
func (p *Parser) getPrimitiveTypeToken(typeName string) TokenType {
	switch typeName {
	case "Int":
		return TokenIntType
	case "Float":
		return TokenFloatType
	case "String":
		return TokenStringType
	case "Boolean":
		return TokenBoolType
	case "File":
		return TokenFile
	case "Directory":
		return TokenDirectory
	case "Array":
		return TokenArray
	case "Map":
		return TokenMap
	case "Pair":
		return TokenPair
	default:
		return TokenIdentifier
	}
}