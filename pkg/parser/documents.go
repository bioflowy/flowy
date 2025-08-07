package parser

import (
	"strings"

	"github.com/bioflowy/flowy/pkg/tree"
)

// parseDocument parses a complete WDL document according to the grammar:
// document: version? document_element*
func (p *Parser) parseDocument() (*tree.Document, bool) {
	pos := p.currentPosition()

	p.skipCommentsAndNewlines()

	// Parse optional version
	// var version string
	if p.currentTokenIs(TokenVersion) {
		if _, ok := p.parseVersion(); ok {
			// version = v
			// Note: Version is parsed but not stored in tree.Document currently
		} else {
			return nil, false
		}
		p.skipCommentsAndNewlines()
	}

	// Parse document elements
	var imports []*tree.Import
	var tasks []*tree.Task
	var workflows []*tree.Workflow
	var structs []*tree.StructTypeDef

	for !p.isAtEnd() {
		p.skipCommentsAndNewlines()
		
		if p.isAtEnd() {
			break
		}

		switch p.currentToken.Type {
		case TokenImport:
			if imp, ok := p.parseImport(); ok {
				imports = append(imports, imp)
			} else {
				return nil, false
			}

		case TokenTask:
			if task, ok := p.parseTask(); ok {
				tasks = append(tasks, task)
			} else {
				return nil, false
			}

		case TokenWorkflow:
			if workflow, ok := p.parseWorkflow(); ok {
				workflows = append(workflows, workflow)
			} else {
				return nil, false
			}

		case TokenStruct:
			if structDef, ok := p.parseStruct(); ok {
				structs = append(structs, structDef)
			} else {
				return nil, false
			}

		default:
			p.addError(NewParseError(
				p.currentPosition(),
				"expected document element (import, task, workflow, struct)",
				[]TokenType{TokenImport, TokenTask, TokenWorkflow, TokenStruct},
				p.currentToken,
			))
			p.synchronize()
			// Try to continue parsing
			continue
		}

		p.skipCommentsAndNewlines()
	}

	// Determine main workflow (if any)
	var mainWorkflow *tree.Workflow
	if len(workflows) > 0 {
		mainWorkflow = workflows[0] // Use first workflow as main
	}

	return tree.NewDocument(imports, mainWorkflow, tasks, structs, pos), true
}

// parseVersion parses a version declaration:
// version: "version" /[^ \t\r\n]+/
func (p *Parser) parseVersion() (string, bool) {
	if !p.consume(TokenVersion) {
		return "", false
	}

	// The version should be the next token
	// In WDL, version is typically like "1.0", "development", etc.
	if p.currentTokenIs(TokenString) {
		version := p.currentToken.Value
		p.nextToken()
		return version, true
	} else if p.currentTokenIs(TokenIdentifier) {
		version := p.currentToken.Value
		p.nextToken()
		return version, true
	} else if p.currentTokenIs(TokenFloat) {
		version := p.currentToken.Value
		p.nextToken()
		return version, true
	}

	p.addError(NewParseError(
		p.currentPosition(),
		"expected version string",
		[]TokenType{TokenString, TokenIdentifier, TokenFloat},
		p.currentToken,
	))
	return "", false
}

// parseImport parses an import statement:
// import_doc: "import" string_literal ["as" CNAME] import_alias*
func (p *Parser) parseImport() (*tree.Import, bool) {
	pos := p.currentPosition()

	if !p.consume(TokenImport) {
		return nil, false
	}

	// Parse import URI
	uri, ok := p.parseStringValue()
	if !ok {
		return nil, false
	}

	// Parse optional namespace alias
	var alias *string
	if p.currentTokenIs(TokenAs) {
		p.nextToken() // consume "as"
		
		aliasName, ok := p.parseIdentifier()
		if !ok {
			return nil, false
		}
		alias = &aliasName
	}

	// Extract namespace from URI if no alias provided
	namespace := extractNamespaceFromURI(uri)
	if alias != nil {
		namespace = *alias
	}

	// Parse import aliases (advanced feature, rarely used)
	// import_alias: "alias" CNAME "as" CNAME
	// For now, we'll skip detailed parsing of these
	for p.currentTokenIs(TokenAlias) {
		p.nextToken() // consume "alias"
		
		// Skip the alias mapping for now
		if p.currentTokenIs(TokenIdentifier) {
			p.nextToken()
		}
		if p.currentTokenIs(TokenAs) {
			p.nextToken()
		}
		if p.currentTokenIs(TokenIdentifier) {
			p.nextToken()
		}
	}

	return tree.NewImport(uri, namespace, alias, pos), true
}

// extractNamespaceFromURI extracts a namespace from a URI
func extractNamespaceFromURI(uri string) string {
	// Remove file extension and path
	parts := strings.Split(uri, "/")
	filename := parts[len(parts)-1]
	
	// Remove .wdl extension
	if strings.HasSuffix(filename, ".wdl") {
		filename = filename[:len(filename)-4]
	}
	
	// Replace invalid characters with underscores
	namespace := strings.ReplaceAll(filename, "-", "_")
	namespace = strings.ReplaceAll(namespace, ".", "_")
	
	return namespace
}

// parseDocumentElement parses any top-level document element:
// ?document_element: import_doc | task | workflow | struct
func (p *Parser) parseDocumentElement() (interface{}, bool) {
	switch p.currentToken.Type {
	case TokenImport:
		return p.parseImport()
	case TokenTask:
		return p.parseTask()
	case TokenWorkflow:
		return p.parseWorkflow()
	case TokenStruct:
		return p.parseStruct()
	default:
		p.addError(NewParseError(
			p.currentPosition(),
			"expected document element",
			[]TokenType{TokenImport, TokenTask, TokenWorkflow, TokenStruct},
			p.currentToken,
		))
		return nil, false
	}
}

// isDocumentElementStart returns true if current token can start a document element
func (p *Parser) isDocumentElementStart() bool {
	switch p.currentToken.Type {
	case TokenImport, TokenTask, TokenWorkflow, TokenStruct:
		return true
	default:
		return false
	}
}

// validateDocumentStructure performs basic validation of document structure
func (p *Parser) validateDocumentStructure(doc *tree.Document) bool {
	// Document should have at least one task or workflow
	if len(doc.Tasks) == 0 && doc.Workflow == nil {
		p.addError(NewParseError(
			doc.SourcePosition(),
			"document must contain at least one task or workflow",
			[]TokenType{TokenTask, TokenWorkflow},
			Token{Type: TokenEOF},
		))
		return false
	}

	return true
}

// parseDocumentElements parses multiple document elements
func (p *Parser) parseDocumentElements() ([]interface{}, bool) {
	var elements []interface{}

	for p.isDocumentElementStart() && !p.isAtEnd() {
		if element, ok := p.parseDocumentElement(); ok {
			elements = append(elements, element)
		} else {
			return nil, false
		}
		
		p.skipCommentsAndNewlines()
	}

	return elements, true
}

// parseOptionalVersion parses an optional version declaration
func (p *Parser) parseOptionalVersion() (string, bool) {
	if p.currentTokenIs(TokenVersion) {
		return p.parseVersion()
	}
	return "", true // No version is OK
}

// isValidWDLVersion checks if the version string is valid
func isValidWDLVersion(version string) bool {
	validVersions := []string{
		"draft-2", "draft-3",
		"1.0", "1.1", "1.2",
		"development",
	}
	
	for _, valid := range validVersions {
		if version == valid {
			return true
		}
	}
	
	return false
}

