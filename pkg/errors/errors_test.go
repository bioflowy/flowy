package errors

import (
	"testing"
)

func TestSourcePosition(t *testing.T) {
	pos := SourcePosition{
		URI:       "test.wdl",
		AbsPath:   "/path/to/test.wdl",
		Line:      10,
		Column:    5,
		EndLine:   10,
		EndColumn: 15,
	}

	if pos.URI != "test.wdl" {
		t.Errorf("Expected URI 'test.wdl', got '%s'", pos.URI)
	}
	if pos.Line != 10 {
		t.Errorf("Expected Line 10, got %d", pos.Line)
	}
	if pos.Column != 5 {
		t.Errorf("Expected Column 5, got %d", pos.Column)
	}
}

func TestSyntaxError(t *testing.T) {
	pos := SourcePosition{
		URI:       "test.wdl",
		AbsPath:   "/path/to/test.wdl",
		Line:      1,
		Column:    1,
		EndLine:   1,
		EndColumn: 10,
	}

	declaredVersion := "1.0"
	err := NewSyntaxError(pos, "test syntax error", "1.0", &declaredVersion)

	if err.Pos != pos {
		t.Error("Expected position to match")
	}
	if err.WDLVersion != "1.0" {
		t.Errorf("Expected WDL version '1.0', got '%s'", err.WDLVersion)
	}
	if err.DeclaredWDLVersion == nil || *err.DeclaredWDLVersion != "1.0" {
		t.Error("Expected declared WDL version '1.0'")
	}
	if err.Error() != "test syntax error" {
		t.Errorf("Expected error message 'test syntax error', got '%s'", err.Error())
	}
}

func TestImportError(t *testing.T) {
	pos := SourcePosition{
		URI:     "test.wdl",
		AbsPath: "/path/to/test.wdl",
		Line:    5,
		Column:  1,
	}

	err := NewImportError(pos, "imported.wdl", "file not found")

	if err.Pos != pos {
		t.Error("Expected position to match")
	}

	expectedMsg := "Failed to import imported.wdl, file not found"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestSourceNode(t *testing.T) {
	pos1 := SourcePosition{
		AbsPath: "/path/to/test.wdl",
		Line:    1,
		Column:  1,
	}
	pos2 := SourcePosition{
		AbsPath: "/path/to/test.wdl",
		Line:    2,
		Column:  1,
	}

	node1 := &BaseSourceNode{Pos: pos1}
	node2 := &BaseSourceNode{Pos: pos2}

	// Test comparison
	if !node1.Less(node2) {
		t.Error("Expected node1 < node2")
	}
	if node2.Less(node1) {
		t.Error("Expected !(node2 < node1)")
	}

	// Test equality
	node3 := &BaseSourceNode{Pos: pos1}
	if !node1.Equal(node3) {
		t.Error("Expected node1 == node3")
	}
}

func TestValidationError(t *testing.T) {
	pos := SourcePosition{
		URI:     "test.wdl",
		AbsPath: "/path/to/test.wdl",
		Line:    10,
		Column:  5,
	}
	node := &BaseSourceNode{Pos: pos}

	err := NewValidationError(node, "test validation error")

	if err.Node != node {
		t.Error("Expected node to match")
	}
	if err.Pos != pos {
		t.Error("Expected position to match")
	}
	if err.Error() != "test validation error" {
		t.Errorf("Expected error message 'test validation error', got '%s'", err.Error())
	}
}

func TestSpecificValidationErrors(t *testing.T) {
	pos := SourcePosition{URI: "test.wdl", AbsPath: "/path/to/test.wdl", Line: 1, Column: 1}
	node := &BaseSourceNode{Pos: pos}

	// Test NoSuchTask
	taskErr := NewNoSuchTask(node, "missing_task")
	expected := "No such task/workflow: missing_task"
	if taskErr.Error() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, taskErr.Error())
	}

	// Test NoSuchFunction
	funcErr := NewNoSuchFunction(node, "missing_func")
	expected = "No such function: missing_func"
	if funcErr.Error() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, funcErr.Error())
	}

	// Test NoSuchMember
	memberErr := NewNoSuchMember(node, "missing_member")
	expected = "No such member 'missing_member'"
	if memberErr.Error() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, memberErr.Error())
	}
}

func TestMultipleValidationErrors(t *testing.T) {
	pos := SourcePosition{URI: "test.wdl", AbsPath: "/path/to/test.wdl", Line: 1, Column: 1}
	node := &BaseSourceNode{Pos: pos}

	err1 := NewNoSuchTask(node, "task1")
	err2 := NewNoSuchFunction(node, "func1")

	multiErr := NewMultipleValidationErrors(err1, err2)

	if len(multiErr.Exceptions) != 2 {
		t.Errorf("Expected 2 exceptions, got %d", len(multiErr.Exceptions))
	}
}

func TestRuntimeError(t *testing.T) {
	moreInfo := map[string]interface{}{
		"task_id": "task123",
		"log_url": "http://logs.example.com/task123",
	}

	err := NewRuntimeError("test runtime error", moreInfo)

	if err.Error() != "test runtime error" {
		t.Errorf("Expected 'test runtime error', got '%s'", err.Error())
	}
	if err.MoreInfo["task_id"] != "task123" {
		t.Error("Expected more_info to contain task_id")
	}
}

func TestEvalError(t *testing.T) {
	pos := SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	node := &BaseSourceNode{Pos: pos}

	err := NewEvalError(node, "evaluation failed")

	if err.Node != node {
		t.Error("Expected node to match")
	}
	if err.Pos != pos {
		t.Error("Expected position to match")
	}
	if err.Error() != "evaluation failed" {
		t.Errorf("Expected 'evaluation failed', got '%s'", err.Error())
	}
}

func TestSpecificEvalErrors(t *testing.T) {
	pos := SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	node := &BaseSourceNode{Pos: pos}

	// Test OutOfBounds
	boundsErr := NewOutOfBounds(node, "index out of bounds")
	if boundsErr.Error() != "index out of bounds" {
		t.Errorf("Expected 'index out of bounds', got '%s'", boundsErr.Error())
	}

	// Test EmptyArray
	emptyErr := NewEmptyArray(node)
	expected := "Empty array for Array+ input/declaration"
	if emptyErr.Error() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, emptyErr.Error())
	}

	// Test NullValue
	nullErr := NewNullValue(node)
	expected = "Null value"
	if nullErr.Error() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, nullErr.Error())
	}
}
