package parser

import (
	"github.com/bioflowy/flowy/wdl/lexer"
	"github.com/bioflowy/flowy/wdl/types"
)

// parseType parses a WDL type expression and returns the corresponding types.Type.
//
// Grammar:
//
//	type = primitive_type ["?" | "+" ["?"]]
//	      | "Array" "[" type "]" ["+" ["?"]] ["?"]
//	      | "Map" "[" type "," type "]" ["?"]
//	      | "Pair" "[" type "," type "]" ["?"]
//	      | IDENT  // StructType
//	      ["?"]
func (p *Parser) parseType() (types.Type, error) {
	tok := p.peek()

	var baseType types.Type
	var err error

	switch tok.Type {
	case lexer.TokBoolean:
		p.advance()
		baseType = types.BooleanType{}

	case lexer.TokIntType:
		p.advance()
		baseType = types.IntType{}

	case lexer.TokFloatType:
		p.advance()
		baseType = types.FloatType{}

	case lexer.TokStringType:
		p.advance()
		baseType = types.StringType{}

	case lexer.TokFileType:
		p.advance()
		baseType = types.FileType{}

	case lexer.TokDirectoryType:
		p.advance()
		baseType = types.DirectoryType{}

	case lexer.TokArray:
		p.advance()
		_, err = p.expect(lexer.TokLBracket)
		if err != nil {
			return nil, err
		}
		itemType, err := p.parseType()
		if err != nil {
			return nil, err
		}
		_, err = p.expect(lexer.TokRBracket)
		if err != nil {
			return nil, err
		}

		arrType := types.ArrayType{ItemType: itemType}

		// Check for + and/or ?
		// Sequence: Array[T]+ → NonEmpty=true
		//           Array[T]? → Optional=true
		//           Array[T]+? → NonEmpty=true, Optional=true
		if p.peek().Type == lexer.TokPlus {
			p.advance()
			arrType.Nonempty = true
			// Now check for optional after +
			if p.peek().Type == lexer.TokQuestion {
				p.advance()
				arrType.Optional = true
			}
		} else if p.peek().Type == lexer.TokQuestion {
			p.advance()
			arrType.Optional = true
		}

		return arrType, nil

	case lexer.TokMap:
		p.advance()
		_, err = p.expect(lexer.TokLBracket)
		if err != nil {
			return nil, err
		}
		keyType, err := p.parseType()
		if err != nil {
			return nil, err
		}
		_, err = p.expect(lexer.TokComma)
		if err != nil {
			return nil, err
		}
		valType, err := p.parseType()
		if err != nil {
			return nil, err
		}
		_, err = p.expect(lexer.TokRBracket)
		if err != nil {
			return nil, err
		}

		mapType := types.MapType{KeyType: keyType, ValType: valType}
		if p.peek().Type == lexer.TokQuestion {
			p.advance()
			mapType.Optional = true
		}
		return mapType, nil

	case lexer.TokPair:
		p.advance()
		_, err = p.expect(lexer.TokLBracket)
		if err != nil {
			return nil, err
		}
		leftType, err := p.parseType()
		if err != nil {
			return nil, err
		}
		_, err = p.expect(lexer.TokComma)
		if err != nil {
			return nil, err
		}
		rightType, err := p.parseType()
		if err != nil {
			return nil, err
		}
		_, err = p.expect(lexer.TokRBracket)
		if err != nil {
			return nil, err
		}

		pairType := types.PairType{LeftType: leftType, RightType: rightType}
		if p.peek().Type == lexer.TokQuestion {
			p.advance()
			pairType.Optional = true
		}
		return pairType, nil

	case lexer.TokIdent:
		// Struct type by name
		p.advance()
		baseType = types.StructType{Name: tok.Val}

	default:
		return nil, p.errorf(tok.Pos, "expected type, got %q", tok.Val)
	}

	// Handle optional suffix for primitive/struct types
	if p.peek().Type == lexer.TokQuestion {
		p.advance()
		baseType = baseType.WithOptional(true)
	}

	return baseType, nil
}
