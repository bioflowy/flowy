package parser

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/bioflowy/flowy/pkg/errors"
)

// TokenType represents the type of a WDL token
type TokenType int

const (
	// Special tokens
	TokenEOF TokenType = iota
	TokenError
	TokenComment
	TokenNewline

	// Literals
	TokenInt
	TokenFloat
	TokenString
	TokenBool
	TokenIdentifier

	// Keywords
	TokenVersion
	TokenImport
	TokenAs
	TokenAlias
	TokenWorkflow
	TokenTask
	TokenInput
	TokenOutput
	TokenMeta
	TokenParameterMeta
	TokenRequirements
	TokenRuntime
	TokenScatter
	TokenIf
	TokenThen
	TokenElse
	TokenCall
	TokenAfter
	TokenStruct
	TokenCommand
	TokenEnv
	TokenLeft
	TokenRight
	TokenObject

	// Type keywords
	TokenArray
	TokenFile
	TokenDirectory
	TokenMap
	TokenPair
	TokenIntType
	TokenFloatType
	TokenStringType
	TokenBoolType
	TokenNone

	// Operators
	TokenLogicalOr     // ||
	TokenLogicalAnd    // &&
	TokenEqual         // ==
	TokenNotEqual      // !=
	TokenLessEqual     // <=
	TokenGreaterEqual  // >=
	TokenLess          // <
	TokenGreater       // >
	TokenPlus          // +
	TokenMinus         // -
	TokenMultiply      // *
	TokenDivide        // /
	TokenModulo        // %
	TokenNot           // !
	TokenAssign        // =

	// Delimiters
	TokenLeftBrace     // {
	TokenRightBrace    // }
	TokenLeftBracket   // [
	TokenRightBracket  // ]
	TokenLeftParen     // (
	TokenRightParen    // )
	TokenComma         // ,
	TokenColon         // :
	TokenSemicolon     // ;
	TokenDot           // .
	TokenQuestion      // ?
	TokenPlusQuantifier // +

	// String interpolation
	TokenInterpolationStart  // ${ or ~{
	TokenCommandStart        // <<<
	TokenCommandEnd          // >>>
	TokenMultiStringStart    // <<<
	TokenMultiStringEnd      // >>>

	// String fragments (used in string interpolation)
	TokenStringFragment
	TokenCommandFragment
)

// Token represents a WDL token
type Token struct {
	Type     TokenType
	Value    string
	Position errors.SourcePosition
}

// String returns a string representation of the token type
func (t TokenType) String() string {
	switch t {
	case TokenEOF:
		return "EOF"
	case TokenError:
		return "ERROR"
	case TokenComment:
		return "COMMENT"
	case TokenNewline:
		return "NEWLINE"
	case TokenInt:
		return "INT"
	case TokenFloat:
		return "FLOAT"
	case TokenString:
		return "STRING"
	case TokenBool:
		return "BOOL"
	case TokenIdentifier:
		return "IDENTIFIER"
	case TokenVersion:
		return "version"
	case TokenImport:
		return "import"
	case TokenAs:
		return "as"
	case TokenAlias:
		return "alias"
	case TokenWorkflow:
		return "workflow"
	case TokenTask:
		return "task"
	case TokenInput:
		return "input"
	case TokenOutput:
		return "output"
	case TokenMeta:
		return "meta"
	case TokenParameterMeta:
		return "parameter_meta"
	case TokenRequirements:
		return "requirements"
	case TokenRuntime:
		return "runtime"
	case TokenScatter:
		return "scatter"
	case TokenIf:
		return "if"
	case TokenThen:
		return "then"
	case TokenElse:
		return "else"
	case TokenCall:
		return "call"
	case TokenAfter:
		return "after"
	case TokenStruct:
		return "struct"
	case TokenCommand:
		return "command"
	case TokenEnv:
		return "env"
	case TokenLeft:
		return "left"
	case TokenRight:
		return "right"
	case TokenObject:
		return "object"
	case TokenArray:
		return "Array"
	case TokenFile:
		return "File"
	case TokenDirectory:
		return "Directory"
	case TokenMap:
		return "Map"
	case TokenPair:
		return "Pair"
	case TokenIntType:
		return "Int"
	case TokenFloatType:
		return "Float"
	case TokenStringType:
		return "String"
	case TokenBoolType:
		return "Boolean"
	case TokenNone:
		return "None"
	case TokenLogicalOr:
		return "||"
	case TokenLogicalAnd:
		return "&&"
	case TokenEqual:
		return "=="
	case TokenNotEqual:
		return "!="
	case TokenLessEqual:
		return "<="
	case TokenGreaterEqual:
		return ">="
	case TokenLess:
		return "<"
	case TokenGreater:
		return ">"
	case TokenPlus:
		return "+"
	case TokenMinus:
		return "-"
	case TokenMultiply:
		return "*"
	case TokenDivide:
		return "/"
	case TokenModulo:
		return "%"
	case TokenNot:
		return "!"
	case TokenAssign:
		return "="
	case TokenLeftBrace:
		return "{"
	case TokenRightBrace:
		return "}"
	case TokenLeftBracket:
		return "["
	case TokenRightBracket:
		return "]"
	case TokenLeftParen:
		return "("
	case TokenRightParen:
		return ")"
	case TokenComma:
		return ","
	case TokenColon:
		return ":"
	case TokenSemicolon:
		return ";"
	case TokenDot:
		return "."
	case TokenQuestion:
		return "?"
	case TokenPlusQuantifier:
		return "+"
	case TokenInterpolationStart:
		return "${" // or ~{
	case TokenCommandStart:
		return "<<<"
	case TokenCommandEnd:
		return ">>>"
	case TokenMultiStringStart:
		return "<<<"
	case TokenMultiStringEnd:
		return ">>>"
	case TokenStringFragment:
		return "STRING_FRAGMENT"
	case TokenCommandFragment:
		return "COMMAND_FRAGMENT"
	default:
		return fmt.Sprintf("TOKEN(%d)", int(t))
	}
}

// LexerMode represents the current mode of the lexer
type LexerMode int

const (
	LexerModeNormal LexerMode = iota
	LexerModeCommand1 // Inside { } command block
	LexerModeCommand2 // Inside <<< >>> command block
	LexerModeString   // Inside string with interpolation
)

// Lexer represents the WDL lexer
type Lexer struct {
	input      string
	position   int
	line       int
	column     int
	uri        string
	keywords   map[string]TokenType
	mode       LexerMode
	modeStack  []LexerMode // Stack for nested modes
	braceDepth int         // Track brace depth for command blocks
	lastToken  TokenType   // Track the last token for context-sensitive lexing
}

// NewLexer creates a new WDL lexer
func NewLexer(input, uri string) *Lexer {
	keywords := map[string]TokenType{
		"version":        TokenVersion,
		"import":         TokenImport,
		"as":             TokenAs,
		"alias":          TokenAlias,
		"workflow":       TokenWorkflow,
		"task":           TokenTask,
		"input":          TokenInput,
		"output":         TokenOutput,
		"meta":           TokenMeta,
		"parameter_meta": TokenParameterMeta,
		"requirements":   TokenRequirements,
		"runtime":        TokenRuntime,
		"scatter":        TokenScatter,
		"if":             TokenIf,
		"then":           TokenThen,
		"else":           TokenElse,
		"call":           TokenCall,
		"after":          TokenAfter,
		"struct":         TokenStruct,
		"command":        TokenCommand,
		"env":            TokenEnv,
		"left":           TokenLeft,
		"right":          TokenRight,
		"object":         TokenObject,
		// Type keywords
		"Array":     TokenArray,
		"File":      TokenFile,
		"Directory": TokenDirectory,
		"Map":       TokenMap,
		"Pair":      TokenPair,
		"Int":       TokenIntType,
		"Float":     TokenFloatType,
		"String":    TokenStringType,
		"Boolean":   TokenBoolType,
		"None":      TokenNone,
		// Boolean literals
		"true":  TokenBool,
		"false": TokenBool,
	}

	return &Lexer{
		input:      input,
		position:   0,
		line:       1,
		column:     1,
		uri:        uri,
		keywords:   keywords,
		mode:       LexerModeNormal,
		modeStack:  []LexerMode{},
		braceDepth: 0,
	}
}

// currentPosition returns the current source position
func (l *Lexer) currentPosition() errors.SourcePosition {
	return errors.SourcePosition{
		URI:     l.uri,
		Line:    l.line,
		Column:  l.column,
		EndLine: l.line,
	}
}

// peek returns the current character without consuming it
func (l *Lexer) peek() byte {
	if l.position >= len(l.input) {
		return 0
	}
	return l.input[l.position]
}

// peekNext returns the next character without consuming it
func (l *Lexer) peekNext() byte {
	if l.position+1 >= len(l.input) {
		return 0
	}
	return l.input[l.position+1]
}

// advance consumes the current character and advances position
func (l *Lexer) advance() byte {
	if l.position >= len(l.input) {
		return 0
	}

	ch := l.input[l.position]
	l.position++

	if ch == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}

	return ch
}

// skipWhitespace skips whitespace characters
func (l *Lexer) skipWhitespace() {
	for l.position < len(l.input) {
		ch := l.peek()
		if ch == ' ' || ch == '\t' || ch == '\r' {
			l.advance()
		} else {
			break
		}
	}
}

// skipComment skips a line comment starting with #
func (l *Lexer) skipComment() {
	// Skip the # character
	l.advance()

	// Skip until end of line
	for l.position < len(l.input) {
		ch := l.peek()
		if ch == '\n' || ch == '\r' {
			break
		}
		l.advance()
	}
}

// readIdentifier reads an identifier or keyword
func (l *Lexer) readIdentifier() string {
	start := l.position

	// First character must be letter or underscore
	if !unicode.IsLetter(rune(l.peek())) && l.peek() != '_' {
		return ""
	}

	for l.position < len(l.input) {
		ch := l.peek()
		if unicode.IsLetter(rune(ch)) || unicode.IsDigit(rune(ch)) || ch == '_' {
			l.advance()
		} else {
			break
		}
	}

	return l.input[start:l.position]
}

// readNumber reads an integer or float literal
func (l *Lexer) readNumber() (string, TokenType) {
	start := l.position
	tokenType := TokenInt

	// Handle optional sign
	if l.peek() == '+' || l.peek() == '-' {
		l.advance()
	}

	// Read digits
	hasDigits := false
	for l.position < len(l.input) && unicode.IsDigit(rune(l.peek())) {
		l.advance()
		hasDigits = true
	}

	// Check for decimal point
	if l.peek() == '.' && l.position+1 < len(l.input) && unicode.IsDigit(rune(l.input[l.position+1])) {
		tokenType = TokenFloat
		l.advance() // consume '.'

		// Read fractional digits
		for l.position < len(l.input) && unicode.IsDigit(rune(l.peek())) {
			l.advance()
		}
	}

	// Check for scientific notation
	if l.peek() == 'e' || l.peek() == 'E' {
		tokenType = TokenFloat
		l.advance()

		if l.peek() == '+' || l.peek() == '-' {
			l.advance()
		}

		for l.position < len(l.input) && unicode.IsDigit(rune(l.peek())) {
			l.advance()
		}
	}

	if !hasDigits && tokenType == TokenInt {
		// Only a sign, not a valid number
		return "", TokenError
	}

	return l.input[start:l.position], tokenType
}

// readString reads a string literal (either single or double quoted)
func (l *Lexer) readString(quote byte) string {
	// start := l.position  // unused
	l.advance() // consume opening quote

	var result strings.Builder
	escaped := false

	for l.position < len(l.input) {
		ch := l.peek()

		if escaped {
			l.advance()
			switch ch {
			case 'n':
				result.WriteByte('\n')
			case 't':
				result.WriteByte('\t')
			case 'r':
				result.WriteByte('\r')
			case '\\':
				result.WriteByte('\\')
			case '\'':
				result.WriteByte('\'')
			case '"':
				result.WriteByte('"')
			default:
				result.WriteByte(ch)
			}
			escaped = false
		} else {
			if ch == '\\' {
				escaped = true
				l.advance()
			} else if ch == quote {
				l.advance() // consume closing quote
				break
			} else {
				result.WriteByte(ch)
				l.advance()
			}
		}
	}

	return result.String()
}

// NextToken returns the next token from the input
func (l *Lexer) NextToken() Token {
	// Handle special modes
	if l.mode == LexerModeCommand1 {
		return l.nextTokenCommand1()
	} else if l.mode == LexerModeCommand2 {
		return l.nextTokenCommand2()
	}

	for {
		l.skipWhitespace()

		if l.position >= len(l.input) {
			return Token{TokenEOF, "", l.currentPosition()}
		}

		pos := l.currentPosition()
		ch := l.peek()

		switch ch {
		case '\n':
			l.advance()
			return Token{TokenNewline, "\n", pos}

		case '#':
			l.skipComment()
			continue // Skip comments and try next token

		// Two-character operators
		case '|':
			if l.peekNext() == '|' {
				l.advance()
				l.advance()
				return Token{TokenLogicalOr, "||", pos}
			}
			l.advance()
			return Token{TokenError, string(ch), pos}

		case '&':
			if l.peekNext() == '&' {
				l.advance()
				l.advance()
				return Token{TokenLogicalAnd, "&&", pos}
			}
			l.advance()
			return Token{TokenError, string(ch), pos}

		case '=':
			if l.peekNext() == '=' {
				l.advance()
				l.advance()
				return Token{TokenEqual, "==", pos}
			}
			l.advance()
			return Token{TokenAssign, "=", pos}

		case '!':
			if l.peekNext() == '=' {
				l.advance()
				l.advance()
				return Token{TokenNotEqual, "!=", pos}
			}
			l.advance()
			return Token{TokenNot, "!", pos}

		case '<':
			if l.peekNext() == '=' {
				l.advance()
				l.advance()
				return Token{TokenLessEqual, "<=", pos}
			} else if l.peekNext() == '<' && l.position+2 < len(l.input) && l.input[l.position+2] == '<' {
				l.advance()
				l.advance()
				l.advance()
				return Token{TokenCommandStart, "<<<", pos}
			}
			l.advance()
			return Token{TokenLess, "<", pos}

		case '>':
			if l.peekNext() == '=' {
				l.advance()
				l.advance()
				return Token{TokenGreaterEqual, ">=", pos}
			} else if l.peekNext() == '>' && l.position+2 < len(l.input) && l.input[l.position+2] == '>' {
				l.advance()
				l.advance()
				l.advance()
				return Token{TokenCommandEnd, ">>>", pos}
			}
			l.advance()
			return Token{TokenGreater, ">", pos}

		// String interpolation starts
		case '$':
			if l.peekNext() == '{' {
				l.advance()
				l.advance()
				return Token{TokenInterpolationStart, "${", pos}
			}
			l.advance()
			return Token{TokenError, string(ch), pos}

		case '~':
			if l.peekNext() == '{' {
				l.advance()
				l.advance()
				return Token{TokenInterpolationStart, "~{", pos}
			}
			l.advance()
			return Token{TokenError, string(ch), pos}

		// Single-character tokens
		case '+':
			l.advance()
			return Token{TokenPlus, "+", pos}
		case '-':
			// Check if this starts a number
			if unicode.IsDigit(rune(l.peekNext())) {
				value, tokenType := l.readNumber()
				if tokenType == TokenError {
					return Token{TokenError, value, pos}
				}
				return Token{tokenType, value, pos}
			}
			l.advance()
			return Token{TokenMinus, "-", pos}
		case '*':
			l.advance()
			return Token{TokenMultiply, "*", pos}
		case '/':
			l.advance()
			return Token{TokenDivide, "/", pos}
		case '%':
			l.advance()
			return Token{TokenModulo, "%", pos}
		case '{':
			l.advance()
			// Check if this is the start of a command block
			if l.lastToken == TokenCommand {
				l.mode = LexerModeCommand1
				l.braceDepth = 1
			}
			l.lastToken = TokenLeftBrace
			return Token{TokenLeftBrace, "{", pos}
		case '}':
			l.advance()
			// Check if this ends an interpolation and we need to return to command mode
			if len(l.modeStack) > 0 {
				// Pop mode from stack
				l.mode = l.modeStack[len(l.modeStack)-1]
				l.modeStack = l.modeStack[:len(l.modeStack)-1]
				l.lastToken = TokenRightBrace
				return Token{TokenRightBrace, "}", pos}
			}
			l.lastToken = TokenRightBrace
			return Token{TokenRightBrace, "}", pos}
		case '[':
			l.advance()
			return Token{TokenLeftBracket, "[", pos}
		case ']':
			l.advance()
			return Token{TokenRightBracket, "]", pos}
		case '(':
			l.advance()
			return Token{TokenLeftParen, "(", pos}
		case ')':
			l.advance()
			return Token{TokenRightParen, ")", pos}
		case ',':
			l.advance()
			return Token{TokenComma, ",", pos}
		case ':':
			l.advance()
			return Token{TokenColon, ":", pos}
		case ';':
			l.advance()
			return Token{TokenSemicolon, ";", pos}
		case '.':
			l.advance()
			return Token{TokenDot, ".", pos}
		case '?':
			l.advance()
			return Token{TokenQuestion, "?", pos}

		// String literals
		case '"', '\'':
			value := l.readString(ch)
			return Token{TokenString, value, pos}

		default:
			// Numbers
			if unicode.IsDigit(rune(ch)) {
				value, tokenType := l.readNumber()
				if tokenType == TokenError {
					return Token{TokenError, value, pos}
				}
				return Token{tokenType, value, pos}
			}

			// Identifiers and keywords
			if unicode.IsLetter(rune(ch)) || ch == '_' {
				value := l.readIdentifier()
				if tokenType, isKeyword := l.keywords[value]; isKeyword {
					// Special handling for command keyword
					if tokenType == TokenCommand {
						// Store the token but don't change mode yet
						// Mode will change when we see the opening brace
						l.lastToken = tokenType
					}
					return Token{tokenType, value, pos}
				}
				return Token{TokenIdentifier, value, pos}
			}

			// Unknown character
			l.advance()
			return Token{TokenError, string(ch), pos}
		}
	}
}

// nextTokenCommand1 handles tokenization inside command { } blocks
func (l *Lexer) nextTokenCommand1() Token {
	if l.position >= len(l.input) {
		return Token{TokenEOF, "", l.currentPosition()}
	}

	pos := l.currentPosition()
	
	// Check for interpolation start
	if l.peek() == '$' && l.peekNext() == '{' {
		l.advance()
		l.advance()
		// Switch to normal mode for the interpolation
		l.modeStack = append(l.modeStack, l.mode)
		l.mode = LexerModeNormal
		return Token{TokenInterpolationStart, "${", pos}
	}
	
	// Check for closing brace
	if l.peek() == '}' {
		// Check if this is the end of the command block
		if l.braceDepth == 1 {
			l.advance()
			l.braceDepth = 0
			l.mode = LexerModeNormal
			l.lastToken = TokenRightBrace
			return Token{TokenRightBrace, "}", pos}
		}
	}
	
	// Read command fragment until next interpolation or closing brace
	var fragment strings.Builder
	for l.position < len(l.input) {
		ch := l.peek()
		
		// Check for special sequences
		if ch == '$' && l.peekNext() == '{' {
			break // Start of interpolation
		}
		if ch == '}' && l.braceDepth == 1 {
			break // End of command block
		}
		
		// Include everything else in the fragment
		fragment.WriteByte(ch)
		l.advance()
		
		// Track newlines for position
		if ch == '\n' {
			continue // Include newlines in command fragment
		}
	}
	
	if fragment.Len() > 0 {
		return Token{TokenCommandFragment, fragment.String(), pos}
	}
	
	// Should not reach here in normal cases
	return Token{TokenError, "", pos}
}

// nextTokenCommand2 handles tokenization inside command <<< >>> blocks
func (l *Lexer) nextTokenCommand2() Token {
	// Similar to nextTokenCommand1 but for <<< >>> style
	// For now, just return to normal mode
	l.mode = LexerModeNormal
	return l.NextToken()
}

// AllTokens returns all tokens from the input (useful for testing)
func (l *Lexer) AllTokens() []Token {
	var tokens []Token

	for {
		token := l.NextToken()
		tokens = append(tokens, token)
		if token.Type == TokenEOF || token.Type == TokenError {
			break
		}
	}

	return tokens
}