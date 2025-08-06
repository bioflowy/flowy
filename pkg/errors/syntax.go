package errors

// SyntaxError represents a failure to lex/parse a WDL document
type SyntaxError struct {
	Pos                SourcePosition
	Message            string
	WDLVersion         string
	DeclaredWDLVersion *string
}

func NewSyntaxError(pos SourcePosition, message string, wdlVersion string, declaredWDLVersion *string) *SyntaxError {
	return &SyntaxError{
		Pos:                pos,
		Message:            message,
		WDLVersion:         wdlVersion,
		DeclaredWDLVersion: declaredWDLVersion,
	}
}

func (e *SyntaxError) Error() string {
	return e.Message
}

// BadCharacterEncoding represents invalid escape sequence in string literal
type BadCharacterEncoding struct {
	Pos SourcePosition
}

func NewBadCharacterEncoding(pos SourcePosition) *BadCharacterEncoding {
	return &BadCharacterEncoding{Pos: pos}
}

func (e *BadCharacterEncoding) Error() string {
	return "Invalid character encoding"
}

// ImportError represents failure to open/retrieve an imported WDL document
type ImportError struct {
	Pos       SourcePosition
	ImportURI string
	Message   string
}

func NewImportError(pos SourcePosition, importURI string, message string) *ImportError {
	msg := "Failed to import " + importURI
	if message != "" {
		msg = msg + ", " + message
	}
	return &ImportError{
		Pos:       pos,
		ImportURI: importURI,
		Message:   msg,
	}
}

func (e *ImportError) Error() string {
	return e.Message
}
