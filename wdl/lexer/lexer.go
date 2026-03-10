// Package lexer implements a WDL 1.2 tokenizer using Rob Pike's state-function pattern.
// The lexer handles three modes:
//   - Normal WDL tokenization
//   - Command-brace mode: both ~{} and ${} are placeholder delimiters
//   - Command-heredoc mode: only ~{} is a placeholder; ${...} passes through as raw text
package lexer

import (
	"strings"

	"github.com/bioflowy/flowy/wdl/ast"
)

// TokenType identifies the kind of token produced by the lexer.
// Using int (matching ast.TokenType = int) prevents circular imports.
type TokenType = ast.TokenType

// Token is a single lexeme emitted by the lexer.
type Token struct {
	Type TokenType
	Val  string
	Pos  ast.Pos
}

// Token type constants. Using iota starting at 1 so the zero value is always invalid.
const (
	TokEOF   TokenType = iota + 1 // end of input
	TokError                       // unrecognized input; Val contains error message
	TokIdent                       // identifier or unrecognized name

	// Literals
	TokIntLit   // integer literal
	TokFloatLit // floating-point literal
	TokStringOpen
	TokStringClose

	// Command-mode tokens
	TokCommandContent  // raw text inside a command block or string
	TokPlaceholderOpen // ~{ or ${ (brace mode); ~{ only (heredoc mode)
	TokPlaceholderClose
	TokHeredocOpen  // <<<
	TokHeredocClose // >>>

	// Keywords
	TokVersion
	TokTask
	TokWorkflow
	TokStruct
	TokImport
	TokAs
	TokAlias
	TokInput
	TokOutput
	TokCommand
	TokRuntime
	TokRequirements
	TokHints
	TokMeta
	TokParamMeta
	TokCall
	TokScatter
	TokIn
	TokIf
	TokThen
	TokElse
	TokAfter
	TokEnv
	TokBoolean // Boolean type keyword
	TokIntType // Int type keyword
	TokFloatType
	TokStringType
	TokFileType
	TokDirectoryType
	TokArray
	TokMap
	TokPair
	TokObject
	TokNull
	TokTrue
	TokFalse

	// Operators
	TokAssign  // =
	TokPlus    // +
	TokMinus   // -
	TokStar    // *
	TokSlash   // /
	TokPercent // %
	TokStarStar // **
	TokEqEq    // ==
	TokNotEq   // !=
	TokLt      // <
	TokLtEq    // <=
	TokGt      // >
	TokGtEq    // >=
	TokAnd     // &&
	TokOr      // ||
	TokNot     // !
	TokDot     // .
	TokQuestion // ?
	TokArrow   // ->
	TokTilde   // ~

	// Punctuation
	TokLBrace   // {
	TokRBrace   // }
	TokLParen   // (
	TokRParen   // )
	TokLBracket // [
	TokRBracket // ]
	TokComma    // ,
	TokColon    // :
)

// keywords maps WDL keyword strings to their token types.
var keywords = map[string]TokenType{
	"version":    TokVersion,
	"task":       TokTask,
	"workflow":   TokWorkflow,
	"struct":     TokStruct,
	"import":     TokImport,
	"as":         TokAs,
	"alias":      TokAlias,
	"input":      TokInput,
	"output":     TokOutput,
	"command":    TokCommand,
	"runtime":    TokRuntime,
	"requirements": TokRequirements,
	"hints":      TokHints,
	"meta":       TokMeta,
	"parameter_meta": TokParamMeta,
	"call":       TokCall,
	"scatter":    TokScatter,
	"in":         TokIn,
	"if":         TokIf,
	"then":       TokThen,
	"else":       TokElse,
	"after":      TokAfter,
	"env":        TokEnv,
	"Boolean":    TokBoolean,
	"Int":        TokIntType,
	"Float":      TokFloatType,
	"String":     TokStringType,
	"File":       TokFileType,
	"Directory":  TokDirectoryType,
	"Array":      TokArray,
	"Map":        TokMap,
	"Pair":       TokPair,
	"Object":     TokObject,
	"null":       TokNull,
	"true":       TokTrue,
	"false":      TokFalse,
}

// lexMode identifies which scanning mode the lexer is currently in.
type lexMode int

const (
	modeNormal       lexMode = iota
	modeCommandBrace         // inside command { ... }
	modeCommandHeredoc       // inside command <<< ... >>>
	modePlaceholder          // inside ~{ or ${ placeholder
	modeString               // inside "..." string
)

// stateFn represents the current state of the lexer as a function.
// It returns the next stateFn to call.
type stateFn func(*Lexer) stateFn

// Lexer is a WDL 1.2 tokenizer.
type Lexer struct {
	file  string    // absolute path of source file (for Pos)
	src   []byte    // full source input
	start int       // start position of current token
	pos   int       // current read position
	line  int       // current line number (1-based)
	col   int       // current column number (1-based)

	// position of current token start (for Pos field)
	tokLine int
	tokCol  int

	// mode stack: top is current mode
	modeStack []lexMode

	// braceDepth tracks nested { } inside a placeholder to prevent premature close
	braceDepth int

	// pending holds tokens ready to be returned by Next()
	pending []Token
}

// NewLexer creates a new Lexer for the given source.
// file should be an absolute path and is used in emitted token positions.
func NewLexer(file string, src []byte) *Lexer {
	l := &Lexer{
		file:      file,
		src:       src,
		line:      1,
		col:       1,
		modeStack: []lexMode{modeNormal},
	}
	return l
}

// Next returns the next token from the source.
// After TokEOF is returned, subsequent calls continue to return TokEOF.
func (l *Lexer) Next() Token {
	for len(l.pending) == 0 {
		state := l.currentStateFn()
		if state == nil {
			// emit EOF
			l.emit(TokEOF, "")
			break
		}
		nextState := state(l)
		if nextState == nil {
			// state signaled done; emit EOF if nothing pending
			if len(l.pending) == 0 {
				l.emit(TokEOF, "")
			}
		}
		// loop continues until we have a pending token or are done
		_ = nextState
	}
	if len(l.pending) == 0 {
		return Token{Type: TokEOF, Pos: ast.Pos{File: l.file, Line: l.line, Col: l.col}}
	}
	tok := l.pending[0]
	l.pending = l.pending[1:]
	return tok
}

// currentStateFn returns the stateFn for the current top-of-stack mode.
func (l *Lexer) currentStateFn() stateFn {
	if l.pos >= len(l.src) {
		return nil
	}
	switch l.currentMode() {
	case modeNormal:
		return lexNormal
	case modeCommandBrace:
		return lexCommandBrace
	case modeCommandHeredoc:
		return lexCommandHeredoc
	case modePlaceholder:
		return lexNormal // re-use normal tokenizer inside placeholders
	case modeString:
		return lexString
	default:
		return lexNormal
	}
}

// currentMode returns the top of the mode stack.
func (l *Lexer) currentMode() lexMode {
	if len(l.modeStack) == 0 {
		return modeNormal
	}
	return l.modeStack[len(l.modeStack)-1]
}

// pushMode pushes a new mode onto the stack.
func (l *Lexer) pushMode(m lexMode) {
	l.modeStack = append(l.modeStack, m)
}

// popMode pops the top mode from the stack.
func (l *Lexer) popMode() {
	if len(l.modeStack) > 1 {
		l.modeStack = l.modeStack[:len(l.modeStack)-1]
	}
}

// peek returns the byte at pos+n without advancing.
// Returns 0 if out of bounds.
func (l *Lexer) peek(n int) byte {
	idx := l.pos + n
	if idx >= len(l.src) {
		return 0
	}
	return l.src[idx]
}

// advance consumes one byte and returns it. Updates line/col tracking.
func (l *Lexer) advance() byte {
	if l.pos >= len(l.src) {
		return 0
	}
	b := l.src[l.pos]
	l.pos++
	if b == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return b
}

// markStart records the current position as the start of the next token.
func (l *Lexer) markStart() {
	l.start = l.pos
	l.tokLine = l.line
	l.tokCol = l.col
}

// emit creates a token from start..pos and appends it to pending.
func (l *Lexer) emit(typ TokenType, val string) {
	tok := Token{
		Type: typ,
		Val:  val,
		Pos:  ast.Pos{File: l.file, Line: l.tokLine, Col: l.tokCol},
	}
	l.pending = append(l.pending, tok)
	l.start = l.pos
}

// emitCurrent emits the text accumulated from l.start to l.pos as the given type.
func (l *Lexer) emitCurrent(typ TokenType) {
	val := string(l.src[l.start:l.pos])
	l.emit(typ, val)
}

// skipWhitespace advances past spaces, tabs, newlines, and carriage returns.
func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.src) {
		b := l.src[l.pos]
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			l.advance()
		} else {
			break
		}
	}
}

// skipLineComment advances past a # line comment.
func (l *Lexer) skipLineComment() {
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		l.advance()
	}
}

// isIdentStart returns true if b can start a WDL identifier.
func isIdentStart(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

// isIdentContinue returns true if b can continue a WDL identifier.
func isIdentContinue(b byte) bool {
	return isIdentStart(b) || (b >= '0' && b <= '9')
}

// isDigit returns true if b is an ASCII decimal digit.
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// lexNormal handles the normal WDL tokenization mode.
// It also handles the modePlaceholder mode (re-entrant normal lexing inside placeholders).
func lexNormal(l *Lexer) stateFn {
	l.skipWhitespace()
	if l.pos >= len(l.src) {
		return nil
	}

	// Skip comments
	if l.src[l.pos] == '#' {
		l.advance()
		l.skipLineComment()
		l.markStart()
		return lexNormal
	}

	l.markStart()
	b := l.src[l.pos]

	// String literal
	if b == '"' {
		l.advance()
		l.emit(TokStringOpen, "\"")
		l.markStart()
		l.pushMode(modeString)
		return nil // will be picked up by currentStateFn -> lexString
	}

	// Identifiers and keywords
	if isIdentStart(b) {
		return lexIdent(l)
	}

	// Numeric literals
	if isDigit(b) || (b == '-' && l.pos+1 < len(l.src) && isDigit(l.src[l.pos+1])) {
		return lexNumber(l)
	}

	// Operators and punctuation
	l.advance()
	switch b {
	case '=':
		if l.peek(0) == '=' {
			l.advance()
			l.emitCurrent(TokEqEq)
		} else {
			l.emitCurrent(TokAssign)
		}
	case '+':
		l.emitCurrent(TokPlus)
	case '-':
		if l.peek(0) == '>' {
			l.advance()
			l.emitCurrent(TokArrow)
		} else {
			l.emitCurrent(TokMinus)
		}
	case '*':
		if l.peek(0) == '*' {
			l.advance()
			l.emitCurrent(TokStarStar)
		} else {
			l.emitCurrent(TokStar)
		}
	case '/':
		l.emitCurrent(TokSlash)
	case '%':
		l.emitCurrent(TokPercent)
	case '!':
		if l.peek(0) == '=' {
			l.advance()
			l.emitCurrent(TokNotEq)
		} else {
			l.emitCurrent(TokNot)
		}
	case '<':
		if l.peek(0) == '=' {
			l.advance()
			l.emitCurrent(TokLtEq)
		} else if l.peek(0) == '<' && l.peek(1) == '<' {
			l.advance()
			l.advance()
			l.emitCurrent(TokHeredocOpen)
		} else {
			l.emitCurrent(TokLt)
		}
	case '>':
		if l.peek(0) == '=' {
			l.advance()
			l.emitCurrent(TokGtEq)
		} else if l.peek(0) == '>' && l.peek(1) == '>' {
			l.advance()
			l.advance()
			l.emitCurrent(TokHeredocClose)
		} else {
			l.emitCurrent(TokGt)
		}
	case '&':
		if l.peek(0) == '&' {
			l.advance()
			l.emitCurrent(TokAnd)
		} else {
			l.emit(TokError, "unexpected '&' (use '&&')")
		}
	case '|':
		if l.peek(0) == '|' {
			l.advance()
			l.emitCurrent(TokOr)
		} else {
			l.emit(TokError, "unexpected '|' (use '||')")
		}
	case '.':
		l.emitCurrent(TokDot)
	case '?':
		l.emitCurrent(TokQuestion)
	case '~':
		l.emitCurrent(TokTilde)
	case '{':
		if l.currentMode() == modePlaceholder {
			l.braceDepth++
			l.emitCurrent(TokLBrace)
		} else {
			l.emitCurrent(TokLBrace)
		}
	case '}':
		mode := l.currentMode()
		if mode == modePlaceholder {
			if l.braceDepth > 0 {
				l.braceDepth--
				l.emitCurrent(TokRBrace)
			} else {
				// close placeholder
				l.emit(TokPlaceholderClose, "}")
				l.popMode() // back to command or string mode
			}
		} else {
			l.emitCurrent(TokRBrace)
		}
	case '(':
		l.emitCurrent(TokLParen)
	case ')':
		l.emitCurrent(TokRParen)
	case '[':
		l.emitCurrent(TokLBracket)
	case ']':
		l.emitCurrent(TokRBracket)
	case ',':
		l.emitCurrent(TokComma)
	case ':':
		l.emitCurrent(TokColon)
	default:
		l.emit(TokError, "unexpected character: "+string(b))
	}
	return nil
}

// lexIdent lexes an identifier or keyword.
func lexIdent(l *Lexer) stateFn {
	for l.pos < len(l.src) && isIdentContinue(l.src[l.pos]) {
		l.advance()
	}
	word := string(l.src[l.start:l.pos])
	typ, ok := keywords[word]
	if !ok {
		typ = TokIdent
	}

	l.emitCurrent(typ)

	// After the "command" keyword, peek at the next non-whitespace to determine mode.
	if typ == TokCommand {
		lexCommandTransition(l)
	}

	return nil
}

// lexCommandTransition is called after the "command" keyword is emitted.
// It skips whitespace and peeks for { or <<< to determine the command mode.
func lexCommandTransition(l *Lexer) {
	l.skipWhitespace()
	l.markStart()

	if l.pos >= len(l.src) {
		return
	}

	b := l.src[l.pos]
	if b == '{' {
		l.advance()
		l.emitCurrent(TokLBrace)
		l.markStart()
		l.pushMode(modeCommandBrace)
	} else if b == '<' && l.peek(1) == '<' && l.peek(2) == '<' {
		l.advance()
		l.advance()
		l.advance()
		l.emitCurrent(TokHeredocOpen)
		l.markStart()
		l.pushMode(modeCommandHeredoc)
	}
	// else: leave in normal mode; parser will handle the error
}

// lexNumber lexes an integer or float literal.
func lexNumber(l *Lexer) stateFn {
	isFloat := false

	// optional leading minus (only if we confirmed next char is a digit in lexNormal)
	// Actually, we don't handle leading minus here — lexNormal only calls lexNumber when
	// the current byte IS a digit. Minus is handled as TokMinus.
	for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
		l.advance()
	}
	// Check for fractional part
	if l.pos < len(l.src) && l.src[l.pos] == '.' {
		isFloat = true
		l.advance()
		for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
			l.advance()
		}
	}
	// Check for exponent
	if l.pos < len(l.src) && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
		isFloat = true
		l.advance()
		if l.pos < len(l.src) && (l.src[l.pos] == '+' || l.src[l.pos] == '-') {
			l.advance()
		}
		for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
			l.advance()
		}
	}
	if isFloat {
		l.emitCurrent(TokFloatLit)
	} else {
		l.emitCurrent(TokIntLit)
	}
	return nil
}

// lexCommandBrace handles scanning inside a brace-style command block.
// Both ~{ and ${ are placeholder delimiters.
// The command ends when an unmatched } is found.
func lexCommandBrace(l *Lexer) stateFn {
	l.markStart()
	// Scan raw content until we hit ~, $, or }
	for l.pos < len(l.src) {
		b := l.src[l.pos]

		if b == '~' && l.peek(1) == '{' {
			// Emit any accumulated content first
			if l.pos > l.start {
				l.emitCurrent(TokCommandContent)
			}
			l.markStart()
			// Emit ~{
			l.advance() // ~
			l.advance() // {
			l.emitCurrent(TokPlaceholderOpen)
			l.markStart()
			l.braceDepth = 0
			l.pushMode(modePlaceholder)
			return nil
		} else if b == '$' && l.peek(1) == '{' {
			// Emit any accumulated content first
			if l.pos > l.start {
				l.emitCurrent(TokCommandContent)
			}
			l.markStart()
			// Emit ${
			l.advance() // $
			l.advance() // {
			l.emitCurrent(TokPlaceholderOpen)
			l.markStart()
			l.braceDepth = 0
			l.pushMode(modePlaceholder)
			return nil
		} else if b == '}' {
			// End of command block
			if l.pos > l.start {
				l.emitCurrent(TokCommandContent)
			}
			l.markStart()
			l.advance()
			l.emitCurrent(TokRBrace)
			l.popMode() // back to normal
			return nil
		} else {
			l.advance()
		}
	}
	// End of input inside command block
	if l.pos > l.start {
		l.emitCurrent(TokCommandContent)
	}
	return nil
}

// lexCommandHeredoc handles scanning inside a heredoc-style command block (<<< ... >>>).
// Only ~{ is a placeholder delimiter; ${...} passes through as raw text.
// The command ends when >>> is found.
func lexCommandHeredoc(l *Lexer) stateFn {
	l.markStart()
	for l.pos < len(l.src) {
		b := l.src[l.pos]

		if b == '~' && l.peek(1) == '{' {
			// Emit any accumulated content first
			if l.pos > l.start {
				l.emitCurrent(TokCommandContent)
			}
			l.markStart()
			l.advance() // ~
			l.advance() // {
			l.emitCurrent(TokPlaceholderOpen)
			l.markStart()
			l.braceDepth = 0
			l.pushMode(modePlaceholder)
			return nil
		} else if b == '>' && l.peek(1) == '>' && l.peek(2) == '>' {
			// End of heredoc block
			if l.pos > l.start {
				l.emitCurrent(TokCommandContent)
			}
			l.markStart()
			l.advance() // >
			l.advance() // >
			l.advance() // >
			l.emitCurrent(TokHeredocClose)
			l.popMode() // back to normal
			return nil
		} else {
			// All other characters (including $ and ${) are raw content
			l.advance()
		}
	}
	// End of input inside heredoc
	if l.pos > l.start {
		l.emitCurrent(TokCommandContent)
	}
	return nil
}

// lexString handles scanning inside a double-quoted string.
// Both ~{ and ${ are placeholder delimiters inside strings.
func lexString(l *Lexer) stateFn {
	l.markStart()
	for l.pos < len(l.src) {
		b := l.src[l.pos]

		if b == '"' {
			// End of string
			if l.pos > l.start {
				l.emitCurrent(TokCommandContent)
			}
			l.markStart()
			l.advance()
			l.emit(TokStringClose, "\"")
			l.popMode()
			return nil
		} else if b == '~' && l.peek(1) == '{' {
			if l.pos > l.start {
				l.emitCurrent(TokCommandContent)
			}
			l.markStart()
			l.advance() // ~
			l.advance() // {
			l.emitCurrent(TokPlaceholderOpen)
			l.markStart()
			l.braceDepth = 0
			l.pushMode(modePlaceholder)
			return nil
		} else if b == '$' && l.peek(1) == '{' {
			if l.pos > l.start {
				l.emitCurrent(TokCommandContent)
			}
			l.markStart()
			l.advance() // $
			l.advance() // {
			l.emitCurrent(TokPlaceholderOpen)
			l.markStart()
			l.braceDepth = 0
			l.pushMode(modePlaceholder)
			return nil
		} else if b == '\\' {
			// escape sequence: consume backslash and next char as raw content
			l.advance()
			if l.pos < len(l.src) {
				l.advance()
			}
		} else {
			l.advance()
		}
	}
	// Unterminated string
	if l.pos > l.start {
		l.emitCurrent(TokCommandContent)
	}
	return nil
}

// tokenizeAll is a helper (for use in tests) that drains the lexer to EOF.
func tokenizeAll(file string, src []byte) []Token {
	l := NewLexer(file, src)
	var tokens []Token
	for {
		tok := l.Next()
		tokens = append(tokens, tok)
		if tok.Type == TokEOF || tok.Type == TokError {
			break
		}
	}
	return tokens
}

// HasTokenOfType is a helper that reports whether a token slice contains
// a token of the given type with the given value.
// val == "" matches any value.
func HasTokenOfType(tokens []Token, typ TokenType, val string) bool {
	for _, tok := range tokens {
		if tok.Type == typ {
			if val == "" || tok.Val == val {
				return true
			}
		}
	}
	return false
}

// FilterByType returns all tokens of the given type from a slice.
func FilterByType(tokens []Token, typ TokenType) []Token {
	var result []Token
	for _, tok := range tokens {
		if tok.Type == typ {
			result = append(result, tok)
		}
	}
	return result
}

// JoinContent joins the Val fields of all TokCommandContent tokens in order.
func JoinContent(tokens []Token) string {
	var b strings.Builder
	for _, tok := range tokens {
		if tok.Type == TokCommandContent {
			b.WriteString(tok.Val)
		}
	}
	return b.String()
}
