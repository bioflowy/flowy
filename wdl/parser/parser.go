// Package parser implements a WDL 1.2 hand-written parser.
// Entry point: Parse(filename string, src []byte) (*ast.Document, error)
// Expression parsing is handled by the Pratt sub-parser in parse_expr.go.
package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/lexer"
)

// ParseError is a structured parse error with precise source location information.
// Error() produces rustc-style output for clear error messages.
type ParseError struct {
	File       string
	Line       int
	Col        int
	Message    string
	SourceLine string
}

// Error returns a rustc-style formatted error message:
//
//	error: {message}
//	  --> {file}:{line}:{col}
//	   |
//	{line} | {sourceLine}
//	   |  {caret}
func (e *ParseError) Error() string {
	caret := strings.Repeat(" ", e.Col-1) + "^"
	lineStr := fmt.Sprintf("%d", e.Line)
	padding := strings.Repeat(" ", len(lineStr))
	return fmt.Sprintf("error: %s\n  --> %s:%d:%d\n%s |\n%s | %s\n%s |  %s",
		e.Message,
		e.File, e.Line, e.Col,
		padding,
		lineStr, e.SourceLine,
		padding, caret,
	)
}

// Parser wraps a lexer and provides a recursive descent parser for WDL 1.2.
type Parser struct {
	lx          *lexer.Lexer
	cur         lexer.Token // most recently consumed token
	peeked      lexer.Token // one-token lookahead
	hasPeeked   bool
	sourceLines []string // source split by newline (for SourceLine in errors)
	file        string
}

// newParser creates a new Parser for the given source file.
func newParser(file string, src []byte) *Parser {
	lines := strings.Split(string(src), "\n")
	p := &Parser{
		lx:          lexer.NewLexer(file, src),
		sourceLines: lines,
		file:        file,
	}
	return p
}

// importAlias represents an "alias OldName as NewName" clause in an import statement.
type importAlias struct {
	OldName string
	NewName string
}

// Parse parses a WDL document from src and returns the AST root.
// file is used in error messages and Pos fields.
// Import statements are recursively resolved: imported tasks, workflows, and
// structs are merged into the returned Document. Circular imports and
// duplicate names produce a *ParseError.
func Parse(file string, src []byte) (*ast.Document, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, &ParseError{File: file, Line: 1, Col: 1, Message: fmt.Sprintf("cannot resolve path: %v", err)}
	}
	return parseFile(abs, src, map[string]bool{}, []string{})
}

// parseFile parses a WDL file and recursively resolves imports.
// path must be an absolute path. src, if non-nil, is used as the file content
// (for the root file); otherwise the file is read from disk.
// visited tracks absolute paths already being parsed (cycle detection).
// chain is the import chain of absolute paths for error messages.
func parseFile(path string, src []byte, visited map[string]bool, chain []string) (*ast.Document, error) {
	if visited[path] {
		// Build cycle message: chain[0] → chain[1] → ... → path
		chainStr := strings.Join(chain, " → ") + " → " + path
		return nil, &ParseError{
			File:    path,
			Line:    1,
			Col:     1,
			Message: "circular import: " + chainStr,
		}
	}
	visited[path] = true
	defer func() { delete(visited, path) }()

	var content []byte
	if src != nil {
		content = src
	} else {
		var err error
		content, err = os.ReadFile(path)
		if err != nil {
			return nil, &ParseError{File: path, Line: 1, Col: 1, Message: fmt.Sprintf("cannot read file: %v", err)}
		}
	}

	p := newParser(path, content)
	doc, importStmts, err := p.parseDocumentWithImports()
	if err != nil {
		return nil, err
	}

	// Resolve imports
	dir := filepath.Dir(path)
	for _, imp := range importStmts {
		importedPath := filepath.Join(dir, imp.path)
		importedAbs, err := filepath.Abs(importedPath)
		if err != nil {
			return nil, &ParseError{File: path, Line: 1, Col: 1, Message: fmt.Sprintf("cannot resolve import path %q: %v", imp.path, err)}
		}

		importedDoc, err := parseFile(importedAbs, nil, visited, append(chain, path))
		if err != nil {
			return nil, err
		}

		// Apply aliases: rename structs in imported doc before merging
		for _, alias := range imp.aliases {
			for _, s := range importedDoc.Structs {
				if s.Name == alias.OldName {
					s.Name = alias.NewName
				}
			}
		}

		// Merge tasks
		for _, task := range importedDoc.Tasks {
			for _, existing := range doc.Tasks {
				if existing.Name == task.Name {
					return nil, &ParseError{
						File:    path,
						Line:    1,
						Col:     1,
						Message: fmt.Sprintf("duplicate task name: %s (imported from %s)", task.Name, importedAbs),
					}
				}
			}
			doc.Tasks = append(doc.Tasks, task)
		}

		// Merge workflows
		for _, wf := range importedDoc.Workflows {
			for _, existing := range doc.Workflows {
				if existing.Name == wf.Name {
					return nil, &ParseError{
						File:    path,
						Line:    1,
						Col:     1,
						Message: fmt.Sprintf("duplicate workflow name: %s (imported from %s)", wf.Name, importedAbs),
					}
				}
			}
			doc.Workflows = append(doc.Workflows, wf)
		}

		// Merge structs
		for _, s := range importedDoc.Structs {
			for _, existing := range doc.Structs {
				if existing.Name == s.Name {
					return nil, &ParseError{
						File:    path,
						Line:    1,
						Col:     1,
						Message: fmt.Sprintf("duplicate struct name: %s (imported from %s)", s.Name, importedAbs),
					}
				}
			}
			doc.Structs = append(doc.Structs, s)
		}
	}

	return doc, nil
}

// importStatement holds the parsed data from a single import statement.
type importStatement struct {
	path      string        // relative file path from string literal
	namespace string        // optional "as" namespace
	aliases   []importAlias // optional "alias X as Y" clauses
}

// peek returns the next token without consuming it.
func (p *Parser) peek() lexer.Token {
	if !p.hasPeeked {
		p.peeked = p.lx.Next()
		p.hasPeeked = true
	}
	return p.peeked
}

// advance consumes and returns the next token.
func (p *Parser) advance() lexer.Token {
	if p.hasPeeked {
		p.cur = p.peeked
		p.hasPeeked = false
	} else {
		p.cur = p.lx.Next()
	}
	return p.cur
}

// expect consumes the next token and returns an error if it is not of type tt.
func (p *Parser) expect(tt lexer.TokenType) (lexer.Token, error) {
	tok := p.advance()
	if tok.Type != tt {
		return tok, p.errorf(tok.Pos, "expected token type %d, got %d (%q)", tt, tok.Type, tok.Val)
	}
	return tok, nil
}

// expectName consumes the next token as a name (identifier).
// It accepts TokIdent as well as any keyword token that may legally appear as a struct/task/workflow
// name — specifically type-keyword identifiers like "Pair", "Map", "Array", "File", etc.
// This is needed because WDL type keywords are valid as user-defined struct names.
func (p *Parser) expectName() (lexer.Token, error) {
	tok := p.advance()
	if isNameToken(tok.Type) {
		return tok, nil
	}
	return tok, p.errorf(tok.Pos, "expected identifier, got %q", tok.Val)
}

// isNameToken returns true if tok is a valid identifier token (including type keywords
// that users commonly use as struct/task/workflow names).
func isNameToken(tt lexer.TokenType) bool {
	switch tt {
	case lexer.TokIdent,
		// Type keywords that are valid as user-defined names in declaration context
		lexer.TokBoolean, lexer.TokIntType, lexer.TokFloatType, lexer.TokStringType,
		lexer.TokFileType, lexer.TokDirectoryType,
		lexer.TokArray, lexer.TokMap, lexer.TokPair, lexer.TokObject,
		// Other keywords usable as variable names in expression context
		lexer.TokEnv:
		return true
	}
	return false
}

// errorf creates a *ParseError at pos with a formatted message.
// SourceLine is retrieved from the source lines slice.
func (p *Parser) errorf(pos ast.Pos, format string, args ...any) *ParseError {
	msg := fmt.Sprintf(format, args...)
	sourceLine := ""
	if pos.Line >= 1 && pos.Line <= len(p.sourceLines) {
		sourceLine = p.sourceLines[pos.Line-1]
	}
	return &ParseError{
		File:       pos.File,
		Line:       pos.Line,
		Col:        pos.Col,
		Message:    msg,
		SourceLine: sourceLine,
	}
}

// parseDocument parses the top-level WDL document.
// Import statements are left unresolved (returned as stubs with error).
// Use parseDocumentWithImports for full import-aware parsing.
func (p *Parser) parseDocument() (*ast.Document, error) {
	doc, _, err := p.parseDocumentWithImports()
	return doc, err
}

// parseDocumentWithImports parses the top-level WDL document and returns
// both the document and the list of import statements found (unresolved).
func (p *Parser) parseDocumentWithImports() (*ast.Document, []importStatement, error) {
	doc := &ast.Document{
		Pos: ast.Pos{File: p.file, Line: 1, Col: 1},
	}
	var imports []importStatement

	// Parse optional version declaration
	if p.peek().Type == lexer.TokVersion {
		version, err := p.parseVersion()
		if err != nil {
			return nil, nil, err
		}
		doc.Version = version
	}

	// Parse document elements
	for p.peek().Type != lexer.TokEOF {
		tok := p.peek()
		switch tok.Type {
		case lexer.TokTask:
			task, err := p.parseTask()
			if err != nil {
				return nil, nil, err
			}
			doc.Tasks = append(doc.Tasks, task)
		case lexer.TokWorkflow:
			wf, err := p.parseWorkflow()
			if err != nil {
				return nil, nil, err
			}
			doc.Workflows = append(doc.Workflows, wf)
		case lexer.TokStruct:
			sd, err := p.parseStruct()
			if err != nil {
				return nil, nil, err
			}
			doc.Structs = append(doc.Structs, sd)
		case lexer.TokImport:
			imp, err := p.parseImport()
			if err != nil {
				return nil, nil, err
			}
			imports = append(imports, imp)
		case lexer.TokError:
			p.advance()
			return nil, nil, p.errorf(tok.Pos, "lexer error: %s", tok.Val)
		default:
			p.advance()
			return nil, nil, p.errorf(tok.Pos, "unexpected token %q at top level", tok.Val)
		}
	}

	return doc, imports, nil
}

// parseImport parses an import statement:
//
//	"import" STRING_LITERAL ["as" CNAME] ("alias" CNAME "as" CNAME)*
func (p *Parser) parseImport() (importStatement, error) {
	_, err := p.expect(lexer.TokImport)
	if err != nil {
		return importStatement{}, err
	}

	// Parse the string literal (import path)
	// String is tokenized as TokStringOpen [TokCommandContent]* TokStringClose
	_, err = p.expect(lexer.TokStringOpen)
	if err != nil {
		return importStatement{}, err
	}
	var sb strings.Builder
	for p.peek().Type == lexer.TokCommandContent {
		sb.WriteString(p.advance().Val)
	}
	_, err = p.expect(lexer.TokStringClose)
	if err != nil {
		return importStatement{}, err
	}
	importPath := sb.String()

	imp := importStatement{path: importPath}

	// Optional "as namespace"
	if p.peek().Type == lexer.TokAs {
		p.advance()
		nsTok, err := p.expectName()
		if err != nil {
			return importStatement{}, err
		}
		imp.namespace = nsTok.Val
	}

	// Zero or more "alias OldName as NewName"
	for p.peek().Type == lexer.TokAlias {
		p.advance()
		oldTok, err := p.expectName()
		if err != nil {
			return importStatement{}, err
		}
		_, err = p.expect(lexer.TokAs)
		if err != nil {
			return importStatement{}, err
		}
		newTok, err := p.expectName()
		if err != nil {
			return importStatement{}, err
		}
		imp.aliases = append(imp.aliases, importAlias{OldName: oldTok.Val, NewName: newTok.Val})
	}

	return imp, nil
}

// parseVersion parses "version 1.2" and returns the version string.
func (p *Parser) parseVersion() (string, error) {
	_, err := p.expect(lexer.TokVersion)
	if err != nil {
		return "", err
	}
	// Version value is an identifier like "1.2" — consume as ident
	tok := p.advance()
	if tok.Type != lexer.TokIdent && tok.Type != lexer.TokFloatLit && tok.Type != lexer.TokIntLit {
		// "1.2" may be tokenized as float literal; "1" as int; "draft-2" as ident
		// Accept any non-EOF token as the version string
		if tok.Type == lexer.TokEOF {
			return "", p.errorf(tok.Pos, "expected version string after 'version'")
		}
	}
	return tok.Val, nil
}

// parseStruct parses "struct Name { unbound_decl* }" into *ast.StructDef.
func (p *Parser) parseStruct() (*ast.StructDef, error) {
	tok, err := p.expect(lexer.TokStruct)
	if err != nil {
		return nil, err
	}
	pos := tok.Pos

	nameTok, err := p.expectName()
	if err != nil {
		return nil, err
	}

	_, err = p.expect(lexer.TokLBrace)
	if err != nil {
		return nil, err
	}

	sd := &ast.StructDef{
		Pos:  pos,
		Name: nameTok.Val,
	}

	for p.peek().Type != lexer.TokRBrace && p.peek().Type != lexer.TokEOF {
		decl, err := p.parseDecl(false)
		if err != nil {
			return nil, err
		}
		sd.Members = append(sd.Members, decl)
	}

	_, err = p.expect(lexer.TokRBrace)
	if err != nil {
		return nil, err
	}

	return sd, nil
}

// parseTask parses "task Name { task_section* command task_section* }".
func (p *Parser) parseTask() (*ast.Task, error) {
	tok, err := p.expect(lexer.TokTask)
	if err != nil {
		return nil, err
	}
	pos := tok.Pos

	nameTok, err := p.expectName()
	if err != nil {
		return nil, err
	}

	_, err = p.expect(lexer.TokLBrace)
	if err != nil {
		return nil, err
	}

	task := &ast.Task{
		Pos:          pos,
		Name:         nameTok.Val,
		Runtime:      make(map[string]ast.Expr),
		Requirements: make(map[string]ast.Expr),
		Hints:        make(map[string]any),
		Meta:         make(map[string]any),
		ParamMeta:    make(map[string]any),
	}

	for p.peek().Type != lexer.TokRBrace && p.peek().Type != lexer.TokEOF {
		if err := p.parseTaskSection(task); err != nil {
			return nil, err
		}
	}

	_, err = p.expect(lexer.TokRBrace)
	if err != nil {
		return nil, err
	}

	return task, nil
}

// parseTaskSection dispatches on the current keyword to parse a task section.
func (p *Parser) parseTaskSection(task *ast.Task) error {
	tok := p.peek()
	switch tok.Type {
	case lexer.TokInput:
		p.advance()
		_, err := p.expect(lexer.TokLBrace)
		if err != nil {
			return err
		}
		for p.peek().Type != lexer.TokRBrace && p.peek().Type != lexer.TokEOF {
			decl, err := p.parseDecl(false)
			if err != nil {
				return err
			}
			task.Inputs = append(task.Inputs, decl)
		}
		_, err = p.expect(lexer.TokRBrace)
		return err

	case lexer.TokOutput:
		p.advance()
		_, err := p.expect(lexer.TokLBrace)
		if err != nil {
			return err
		}
		for p.peek().Type != lexer.TokRBrace && p.peek().Type != lexer.TokEOF {
			decl, err := p.parseDecl(true)
			if err != nil {
				return err
			}
			task.Outputs = append(task.Outputs, decl)
		}
		_, err = p.expect(lexer.TokRBrace)
		return err

	case lexer.TokCommand:
		cmd, err := p.parseCommandBlock()
		if err != nil {
			return err
		}
		task.Command = cmd
		return nil

	case lexer.TokRuntime:
		p.advance()
		_, err := p.expect(lexer.TokLBrace)
		if err != nil {
			return err
		}
		for p.peek().Type != lexer.TokRBrace && p.peek().Type != lexer.TokEOF {
			keyTok := p.advance()
			if keyTok.Type != lexer.TokIdent {
				return p.errorf(keyTok.Pos, "expected identifier in runtime block, got %q", keyTok.Val)
			}
			_, err := p.expect(lexer.TokColon)
			if err != nil {
				return err
			}
			expr, err := p.parseExpr(lexer.TokComma, lexer.TokRBrace)
			if err != nil {
				return err
			}
			// consume optional comma
			if p.peek().Type == lexer.TokComma {
				p.advance()
			}
			task.Runtime[keyTok.Val] = expr
		}
		_, err = p.expect(lexer.TokRBrace)
		return err

	case lexer.TokRequirements:
		p.advance()
		_, err := p.expect(lexer.TokLBrace)
		if err != nil {
			return err
		}
		for p.peek().Type != lexer.TokRBrace && p.peek().Type != lexer.TokEOF {
			keyTok := p.advance()
			if keyTok.Type != lexer.TokIdent {
				return p.errorf(keyTok.Pos, "expected identifier in requirements block, got %q", keyTok.Val)
			}
			_, err := p.expect(lexer.TokColon)
			if err != nil {
				return err
			}
			expr, err := p.parseExpr(lexer.TokComma, lexer.TokRBrace)
			if err != nil {
				return err
			}
			if p.peek().Type == lexer.TokComma {
				p.advance()
			}
			task.Requirements[keyTok.Val] = expr
		}
		_, err = p.expect(lexer.TokRBrace)
		return err

	case lexer.TokHints:
		p.advance()
		_, err := p.expect(lexer.TokLBrace)
		if err != nil {
			return err
		}
		vals, err := p.parseMetaBody()
		if err != nil {
			return err
		}
		task.Hints = vals
		_, err = p.expect(lexer.TokRBrace)
		return err

	case lexer.TokMeta:
		p.advance()
		_, err := p.expect(lexer.TokLBrace)
		if err != nil {
			return err
		}
		vals, err := p.parseMetaBody()
		if err != nil {
			return err
		}
		task.Meta = vals
		_, err = p.expect(lexer.TokRBrace)
		return err

	case lexer.TokParamMeta:
		p.advance()
		_, err := p.expect(lexer.TokLBrace)
		if err != nil {
			return err
		}
		vals, err := p.parseMetaBody()
		if err != nil {
			return err
		}
		task.ParamMeta = vals
		_, err = p.expect(lexer.TokRBrace)
		return err

	default:
		// Private intermediate declaration
		decl, err := p.parseDecl(true)
		if err != nil {
			return err
		}
		task.Decls = append(task.Decls, decl)
		return nil
	}
}

// parseCommandBlock parses a command block (brace or heredoc style).
// The "command" keyword has already been handled by the lexer's auto-mode detection
// which emits TokCommand followed by TokLBrace or TokHeredocOpen.
func (p *Parser) parseCommandBlock() (*ast.CommandExpr, error) {
	tok, err := p.expect(lexer.TokCommand)
	if err != nil {
		return nil, err
	}
	pos := tok.Pos

	openTok := p.advance()
	var style ast.CommandStyle
	var closeType lexer.TokenType

	switch openTok.Type {
	case lexer.TokLBrace:
		style = ast.CommandBrace
		closeType = lexer.TokRBrace
	case lexer.TokHeredocOpen:
		style = ast.CommandHeredoc
		closeType = lexer.TokHeredocClose
	default:
		return nil, p.errorf(openTok.Pos, "expected { or <<< after command, got %q", openTok.Val)
	}

	cmd := &ast.CommandExpr{
		Pos:   pos,
		Style: style,
	}

	// Collect parts until close token
	for p.peek().Type != closeType && p.peek().Type != lexer.TokEOF {
		t := p.peek()
		switch t.Type {
		case lexer.TokCommandContent:
			p.advance()
			cmd.Parts = append(cmd.Parts, &ast.RawStringPart{Val: t.Val})
		case lexer.TokPlaceholderOpen:
			p.advance()
			partPos := t.Pos
			// Stub: consume tokens until TokPlaceholderClose
			expr, err := p.parseExpr(lexer.TokPlaceholderClose)
			if err != nil {
				return nil, err
			}
			_, err = p.expect(lexer.TokPlaceholderClose)
			if err != nil {
				return nil, err
			}
			cmd.Parts = append(cmd.Parts, &ast.PlaceholderPart{
				Pos:  partPos,
				Expr: expr,
			})
		default:
			// Unexpected token in command block; treat as raw content
			p.advance()
			cmd.Parts = append(cmd.Parts, &ast.RawStringPart{Val: t.Val})
		}
	}

	_, err = p.expect(closeType)
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

// parseWorkflow parses "workflow Name { workflow_element* }".
func (p *Parser) parseWorkflow() (*ast.Workflow, error) {
	tok, err := p.expect(lexer.TokWorkflow)
	if err != nil {
		return nil, err
	}
	pos := tok.Pos

	nameTok, err := p.expectName()
	if err != nil {
		return nil, err
	}

	_, err = p.expect(lexer.TokLBrace)
	if err != nil {
		return nil, err
	}

	wf := &ast.Workflow{
		Pos:       pos,
		Name:      nameTok.Val,
		Meta:      make(map[string]any),
		ParamMeta: make(map[string]any),
	}

	for p.peek().Type != lexer.TokRBrace && p.peek().Type != lexer.TokEOF {
		tok := p.peek()
		switch tok.Type {
		case lexer.TokInput:
			p.advance()
			_, err = p.expect(lexer.TokLBrace)
			if err != nil {
				return nil, err
			}
			for p.peek().Type != lexer.TokRBrace && p.peek().Type != lexer.TokEOF {
				// WDL 1.2: workflow inputs may have optional default values (bound=true)
				decl, err := p.parseDecl(true)
				if err != nil {
					return nil, err
				}
				wf.Inputs = append(wf.Inputs, decl)
			}
			_, err = p.expect(lexer.TokRBrace)
			if err != nil {
				return nil, err
			}

		case lexer.TokOutput:
			p.advance()
			_, err = p.expect(lexer.TokLBrace)
			if err != nil {
				return nil, err
			}
			for p.peek().Type != lexer.TokRBrace && p.peek().Type != lexer.TokEOF {
				decl, err := p.parseDecl(true)
				if err != nil {
					return nil, err
				}
				wf.Outputs = append(wf.Outputs, decl)
			}
			_, err = p.expect(lexer.TokRBrace)
			if err != nil {
				return nil, err
			}

		case lexer.TokMeta:
			p.advance()
			_, err = p.expect(lexer.TokLBrace)
			if err != nil {
				return nil, err
			}
			vals, err := p.parseMetaBody()
			if err != nil {
				return nil, err
			}
			wf.Meta = vals
			_, err = p.expect(lexer.TokRBrace)
			if err != nil {
				return nil, err
			}

		case lexer.TokParamMeta:
			p.advance()
			_, err = p.expect(lexer.TokLBrace)
			if err != nil {
				return nil, err
			}
			vals, err := p.parseMetaBody()
			if err != nil {
				return nil, err
			}
			wf.ParamMeta = vals
			_, err = p.expect(lexer.TokRBrace)
			if err != nil {
				return nil, err
			}

		default:
			node, err := p.parseWorkflowBodyElement()
			if err != nil {
				return nil, err
			}
			if node != nil {
				wf.Body = append(wf.Body, node)
			}
		}
	}

	_, err = p.expect(lexer.TokRBrace)
	if err != nil {
		return nil, err
	}

	return wf, nil
}

// parseWorkflowBodyElement parses a single body element (scatter, conditional, call, or decl).
func (p *Parser) parseWorkflowBodyElement() (ast.WorkflowNode, error) {
	tok := p.peek()
	switch tok.Type {
	case lexer.TokScatter:
		return p.parseScatter()
	case lexer.TokIf:
		return p.parseConditional()
	case lexer.TokCall:
		return p.parseCall()
	default:
		return p.parseDecl(true)
	}
}

// parseScatter parses "scatter (variable in expr) { workflow_element* }".
func (p *Parser) parseScatter() (*ast.Scatter, error) {
	tok, err := p.expect(lexer.TokScatter)
	if err != nil {
		return nil, err
	}
	pos := tok.Pos

	_, err = p.expect(lexer.TokLParen)
	if err != nil {
		return nil, err
	}

	varTok, err := p.expect(lexer.TokIdent)
	if err != nil {
		return nil, err
	}

	_, err = p.expect(lexer.TokIn)
	if err != nil {
		return nil, err
	}

	expr, err := p.parseExpr(lexer.TokRParen)
	if err != nil {
		return nil, err
	}

	_, err = p.expect(lexer.TokRParen)
	if err != nil {
		return nil, err
	}

	_, err = p.expect(lexer.TokLBrace)
	if err != nil {
		return nil, err
	}

	sc := &ast.Scatter{
		Pos:      pos,
		Variable: varTok.Val,
		Expr:     expr,
	}

	for p.peek().Type != lexer.TokRBrace && p.peek().Type != lexer.TokEOF {
		node, err := p.parseWorkflowBodyElement()
		if err != nil {
			return nil, err
		}
		if node != nil {
			sc.Body = append(sc.Body, node)
		}
	}

	_, err = p.expect(lexer.TokRBrace)
	if err != nil {
		return nil, err
	}

	return sc, nil
}

// parseConditional parses "if (expr) { workflow_element* }".
func (p *Parser) parseConditional() (*ast.Conditional, error) {
	tok, err := p.expect(lexer.TokIf)
	if err != nil {
		return nil, err
	}
	pos := tok.Pos

	_, err = p.expect(lexer.TokLParen)
	if err != nil {
		return nil, err
	}

	expr, err := p.parseExpr(lexer.TokRParen)
	if err != nil {
		return nil, err
	}

	_, err = p.expect(lexer.TokRParen)
	if err != nil {
		return nil, err
	}

	_, err = p.expect(lexer.TokLBrace)
	if err != nil {
		return nil, err
	}

	cond := &ast.Conditional{
		Pos:  pos,
		Expr: expr,
	}

	for p.peek().Type != lexer.TokRBrace && p.peek().Type != lexer.TokEOF {
		node, err := p.parseWorkflowBodyElement()
		if err != nil {
			return nil, err
		}
		if node != nil {
			cond.Body = append(cond.Body, node)
		}
	}

	_, err = p.expect(lexer.TokRBrace)
	if err != nil {
		return nil, err
	}

	return cond, nil
}

// parseCall parses "call CNAME [as CNAME] [after CNAME (, CNAME)*] [{ input: ... }]".
func (p *Parser) parseCall() (*ast.Call, error) {
	tok, err := p.expect(lexer.TokCall)
	if err != nil {
		return nil, err
	}
	pos := tok.Pos

	// Parse callee: one or more CNAME separated by dots
	callee := []string{}
	nameTok, err := p.expect(lexer.TokIdent)
	if err != nil {
		return nil, err
	}
	callee = append(callee, nameTok.Val)
	for p.peek().Type == lexer.TokDot {
		p.advance()
		nameTok, err = p.expect(lexer.TokIdent)
		if err != nil {
			return nil, err
		}
		callee = append(callee, nameTok.Val)
	}

	call := &ast.Call{
		Pos:    pos,
		Callee: callee,
		Inputs: make(map[string]ast.Expr),
	}

	// Optional alias
	if p.peek().Type == lexer.TokAs {
		p.advance()
		aliasTok, err := p.expect(lexer.TokIdent)
		if err != nil {
			return nil, err
		}
		call.Alias = aliasTok.Val
	}

	// Optional after clause
	if p.peek().Type == lexer.TokAfter {
		p.advance()
		afterTok, err := p.expect(lexer.TokIdent)
		if err != nil {
			return nil, err
		}
		call.After = append(call.After, afterTok.Val)
		for p.peek().Type == lexer.TokComma {
			p.advance()
			afterTok, err = p.expect(lexer.TokIdent)
			if err != nil {
				return nil, err
			}
			call.After = append(call.After, afterTok.Val)
		}
	}

	// Optional input block
	if p.peek().Type == lexer.TokLBrace {
		p.advance()
		// Expect "input:"
		if p.peek().Type == lexer.TokInput {
			p.advance()
			_, err = p.expect(lexer.TokColon)
			if err != nil {
				return nil, err
			}
			// Parse key = expr pairs
			for p.peek().Type != lexer.TokRBrace && p.peek().Type != lexer.TokEOF {
				keyTok := p.advance()
				if keyTok.Type != lexer.TokIdent {
					return nil, p.errorf(keyTok.Pos, "expected identifier in call input, got %q", keyTok.Val)
				}
				_, err = p.expect(lexer.TokAssign)
				if err != nil {
					return nil, err
				}
				expr, err := p.parseExpr(lexer.TokComma, lexer.TokRBrace)
				if err != nil {
					return nil, err
				}
				if p.peek().Type == lexer.TokComma {
					p.advance()
				}
				call.Inputs[keyTok.Val] = expr
			}
		}
		_, err = p.expect(lexer.TokRBrace)
		if err != nil {
			return nil, err
		}
	}

	return call, nil
}

// parseDecl parses "Type Name [= Expr]" into *ast.Decl.
// If bound=false, the expression is not expected (struct member / input stub).
// Supports optional "env" modifier before the type.
func (p *Parser) parseDecl(bound bool) (*ast.Decl, error) {
	pos := p.peek().Pos

	// Check for optional "env" modifier
	envFlag := false
	if p.peek().Type == lexer.TokEnv {
		p.advance()
		envFlag = true
	}

	typ, err := p.parseType()
	if err != nil {
		return nil, err
	}

	nameTok, err := p.expect(lexer.TokIdent)
	if err != nil {
		return nil, err
	}

	decl := &ast.Decl{
		Pos:  pos,
		Type: typ,
		Name: nameTok.Val,
		Env:  envFlag,
	}

	if bound && p.peek().Type == lexer.TokAssign {
		p.advance()
		expr, err := p.parseExpr(lexer.TokRBrace, lexer.TokEOF)
		if err != nil {
			return nil, err
		}
		decl.Expr = expr
	}

	return decl, nil
}

// parseMetaBody parses JSON-like key-value pairs until the closing "}" is seen.
// It does NOT consume the closing brace; the caller does that.
func (p *Parser) parseMetaBody() (map[string]any, error) {
	result := make(map[string]any)

	for p.peek().Type != lexer.TokRBrace && p.peek().Type != lexer.TokEOF {
		keyTok := p.advance()
		if keyTok.Type != lexer.TokIdent {
			return nil, p.errorf(keyTok.Pos, "expected identifier in meta block, got %q", keyTok.Val)
		}
		_, err := p.expect(lexer.TokColon)
		if err != nil {
			return nil, err
		}
		val, err := p.parseMetaValue()
		if err != nil {
			return nil, err
		}
		result[keyTok.Val] = val

		// consume optional comma
		if p.peek().Type == lexer.TokComma {
			p.advance()
		}
	}

	return result, nil
}

// parseMetaValue parses a JSON-like meta value (string, number, bool, null, array, object).
func (p *Parser) parseMetaValue() (any, error) {
	tok := p.peek()
	switch tok.Type {
	case lexer.TokStringOpen:
		// Consume string: TokStringOpen [TokCommandContent] TokStringClose
		p.advance()
		var sb strings.Builder
		for p.peek().Type == lexer.TokCommandContent {
			sb.WriteString(p.advance().Val)
		}
		_, err := p.expect(lexer.TokStringClose)
		if err != nil {
			return nil, err
		}
		return sb.String(), nil

	case lexer.TokIntLit:
		p.advance()
		var val int64
		fmt.Sscanf(tok.Val, "%d", &val)
		return val, nil

	case lexer.TokFloatLit:
		p.advance()
		var val float64
		fmt.Sscanf(tok.Val, "%f", &val)
		return val, nil

	case lexer.TokMinus:
		p.advance()
		inner := p.peek()
		if inner.Type == lexer.TokIntLit {
			p.advance()
			var val int64
			fmt.Sscanf(inner.Val, "%d", &val)
			return -val, nil
		} else if inner.Type == lexer.TokFloatLit {
			p.advance()
			var val float64
			fmt.Sscanf(inner.Val, "%f", &val)
			return -val, nil
		}
		return nil, p.errorf(inner.Pos, "expected number after '-' in meta value")

	case lexer.TokTrue:
		p.advance()
		return true, nil

	case lexer.TokFalse:
		p.advance()
		return false, nil

	case lexer.TokNull:
		p.advance()
		return nil, nil

	case lexer.TokLBracket:
		p.advance()
		var items []any
		for p.peek().Type != lexer.TokRBracket && p.peek().Type != lexer.TokEOF {
			val, err := p.parseMetaValue()
			if err != nil {
				return nil, err
			}
			items = append(items, val)
			if p.peek().Type == lexer.TokComma {
				p.advance()
			}
		}
		_, err := p.expect(lexer.TokRBracket)
		if err != nil {
			return nil, err
		}
		return items, nil

	case lexer.TokLBrace:
		p.advance()
		nested := make(map[string]any)
		for p.peek().Type != lexer.TokRBrace && p.peek().Type != lexer.TokEOF {
			keyTok := p.advance()
			if keyTok.Type != lexer.TokIdent && keyTok.Type != lexer.TokStringOpen {
				return nil, p.errorf(keyTok.Pos, "expected key in meta object, got %q", keyTok.Val)
			}
			var key string
			if keyTok.Type == lexer.TokStringOpen {
				var sb strings.Builder
				for p.peek().Type == lexer.TokCommandContent {
					sb.WriteString(p.advance().Val)
				}
				_, err := p.expect(lexer.TokStringClose)
				if err != nil {
					return nil, err
				}
				key = sb.String()
			} else {
				key = keyTok.Val
			}
			_, err := p.expect(lexer.TokColon)
			if err != nil {
				return nil, err
			}
			val, err := p.parseMetaValue()
			if err != nil {
				return nil, err
			}
			nested[key] = val
			if p.peek().Type == lexer.TokComma {
				p.advance()
			}
		}
		_, err := p.expect(lexer.TokRBrace)
		if err != nil {
			return nil, err
		}
		return nested, nil

	default:
		p.advance()
		return nil, p.errorf(tok.Pos, "unexpected token %q in meta value", tok.Val)
	}
}

// parseType is defined in parse_types.go.
// parseExpr is defined in parse_expr.go (Pratt expression parser).
