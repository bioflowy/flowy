package eval

import (
	"fmt"

	"github.com/bioflowy/flowy/wdl/ast"
)

// EvalError represents a runtime evaluation error, mirroring checker.TypeError.
type EvalError struct {
	Pos     ast.Pos
	Message string
}

func (e *EvalError) Error() string {
	return fmt.Sprintf("eval error: %s\n  --> %s:%d:%d",
		e.Message, e.Pos.File, e.Pos.Line, e.Pos.Col)
}
