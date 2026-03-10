package ast_test

import (
	"testing"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/types"
)

// TestASTStructsCompile verifies that all exported AST types are accessible
// and have the expected fields/shapes. This is a compile-time check encoded
// as a runtime test so we can use go test for verification.
func TestASTStructsCompile(t *testing.T) {
	// Pos embeds File, Line, Col
	pos := ast.Pos{File: "/abs/path/file.wdl", Line: 1, Col: 1}
	if pos.File == "" {
		t.Error("Pos.File should be set")
	}

	// Document
	doc := &ast.Document{
		Pos:     pos,
		Version: "1.2",
	}
	if doc.Version != "1.2" {
		t.Error("Document.Version should be set")
	}

	// Task
	task := &ast.Task{
		Pos:  pos,
		Name: "my_task",
	}
	if task.Name != "my_task" {
		t.Error("Task.Name should be set")
	}

	// Workflow
	wf := &ast.Workflow{
		Pos:  pos,
		Name: "my_workflow",
	}
	if wf.Name != "my_workflow" {
		t.Error("Workflow.Name should be set")
	}

	// Decl with types.Type
	decl := &ast.Decl{
		Pos:  pos,
		Type: types.IntType{},
		Name: "x",
	}
	if decl.Name != "x" {
		t.Error("Decl.Name should be set")
	}
	if decl.Type.TypeName() != "Int" {
		t.Error("Decl.Type should be IntType")
	}

	// StructDef
	sd := &ast.StructDef{
		Pos:  pos,
		Name: "MyStruct",
	}
	if sd.Name != "MyStruct" {
		t.Error("StructDef.Name should be set")
	}

	// Call
	call := &ast.Call{
		Pos:    pos,
		Callee: []string{"my_task"},
		Alias:  "t1",
		After:  []string{"other"},
		Inputs: map[string]ast.Expr{},
	}
	if call.Alias != "t1" {
		t.Error("Call.Alias should be set")
	}

	// Scatter
	scatter := &ast.Scatter{
		Pos:      pos,
		Variable: "item",
	}
	if scatter.Variable != "item" {
		t.Error("Scatter.Variable should be set")
	}

	// Conditional
	cond := &ast.Conditional{
		Pos: pos,
	}
	_ = cond

	// WorkflowNode interface — Decl, Call, Scatter, Conditional all implement it
	var _ ast.WorkflowNode = &ast.Decl{}
	var _ ast.WorkflowNode = &ast.Call{}
	var _ ast.WorkflowNode = &ast.Scatter{}
	var _ ast.WorkflowNode = &ast.Conditional{}

	// Workflow.Body uses WorkflowNode slice
	wf.Body = []ast.WorkflowNode{decl, call, scatter, cond}
	if len(wf.Body) != 4 {
		t.Error("Workflow.Body should hold WorkflowNode values")
	}

	// Document collects tasks, workflows, structs
	doc.Tasks = []*ast.Task{task}
	doc.Workflows = []*ast.Workflow{wf}
	doc.Structs = []*ast.StructDef{sd}

	// StringPart interface — RawStringPart and PlaceholderPart implement it
	var _ ast.StringPart = &ast.RawStringPart{}
	var _ ast.StringPart = &ast.PlaceholderPart{}

	raw := &ast.RawStringPart{Val: "hello"}
	if raw.Val != "hello" {
		t.Error("RawStringPart.Val should be set")
	}
	ph := &ast.PlaceholderPart{
		Pos:     pos,
		Options: map[string]string{"sep": ","},
	}
	_ = ph

	// CommandStyle constants
	var bs ast.CommandStyle = ast.CommandBrace
	var hs ast.CommandStyle = ast.CommandHeredoc
	if bs == hs {
		t.Error("CommandBrace and CommandHeredoc should be distinct")
	}
}

// TestExprInterface verifies the Expr interface and all expression node types.
func TestExprInterface(t *testing.T) {
	pos := ast.Pos{File: "/file.wdl", Line: 1, Col: 1}

	// All expr node types must implement Expr
	var _ ast.Expr = &ast.BoolLit{}
	var _ ast.Expr = &ast.IntLit{}
	var _ ast.Expr = &ast.FloatLit{}
	var _ ast.Expr = &ast.NullLit{}
	var _ ast.Expr = &ast.StringExpr{}
	var _ ast.Expr = &ast.CommandExpr{}
	var _ ast.Expr = &ast.ArrayExpr{}
	var _ ast.Expr = &ast.MapExpr{}
	var _ ast.Expr = &ast.PairExpr{}
	var _ ast.Expr = &ast.StructExpr{}
	var _ ast.Expr = &ast.IfThenElse{}
	var _ ast.Expr = &ast.BinaryExpr{}
	var _ ast.Expr = &ast.UnaryExpr{}
	var _ ast.Expr = &ast.IndexExpr{}
	var _ ast.Expr = &ast.GetExpr{}
	var _ ast.Expr = &ast.ApplyExpr{}
	var _ ast.Expr = &ast.IdentExpr{}

	// Spot-check field presence
	bl := &ast.BoolLit{Pos: pos, Value: true}
	if !bl.Value {
		t.Error("BoolLit.Value should be true")
	}

	il := &ast.IntLit{Pos: pos, Value: 42}
	if il.Value != 42 {
		t.Error("IntLit.Value should be 42")
	}

	fl := &ast.FloatLit{Pos: pos, Value: 3.14}
	if fl.Value != 3.14 {
		t.Error("FloatLit.Value should be 3.14")
	}

	id := &ast.IdentExpr{Pos: pos, Name: "myVar"}
	if id.Name != "myVar" {
		t.Error("IdentExpr.Name should be set")
	}

	bin := &ast.BinaryExpr{
		Pos:   pos,
		Op:    1, // TokenType = int alias
		Left:  &ast.IntLit{Pos: pos, Value: 1},
		Right: &ast.IntLit{Pos: pos, Value: 2},
	}
	if bin.Op != 1 {
		t.Error("BinaryExpr.Op should be set")
	}

	un := &ast.UnaryExpr{
		Pos:     pos,
		Op:      2,
		Operand: &ast.BoolLit{Pos: pos, Value: false},
	}
	_ = un

	idx := &ast.IndexExpr{
		Pos:        pos,
		Collection: &ast.IdentExpr{Pos: pos, Name: "arr"},
		Index:      &ast.IntLit{Pos: pos, Value: 0},
	}
	_ = idx

	get := &ast.GetExpr{
		Pos:    pos,
		Object: &ast.IdentExpr{Pos: pos, Name: "obj"},
		Member: "field",
	}
	if get.Member != "field" {
		t.Error("GetExpr.Member should be set")
	}

	apply := &ast.ApplyExpr{
		Pos:  pos,
		Name: "length",
		Args: []ast.Expr{&ast.IdentExpr{Pos: pos, Name: "s"}},
	}
	if apply.Name != "length" {
		t.Error("ApplyExpr.Name should be set")
	}

	ite := &ast.IfThenElse{
		Pos:  pos,
		Cond: &ast.BoolLit{Pos: pos, Value: true},
		Then: &ast.IntLit{Pos: pos, Value: 1},
		Else: &ast.IntLit{Pos: pos, Value: 0},
	}
	_ = ite

	arr := &ast.ArrayExpr{
		Pos:   pos,
		Items: []ast.Expr{&ast.IntLit{Pos: pos, Value: 1}},
	}
	if len(arr.Items) != 1 {
		t.Error("ArrayExpr.Items should have one element")
	}

	mapExpr := &ast.MapExpr{
		Pos: pos,
		Items: []ast.MapItem{
			{Key: &ast.StringExpr{Pos: pos}, Value: &ast.IntLit{Pos: pos, Value: 1}},
		},
	}
	if len(mapExpr.Items) != 1 {
		t.Error("MapExpr.Items should have one element")
	}

	pair := &ast.PairExpr{
		Pos:   pos,
		Left:  &ast.IntLit{Pos: pos, Value: 1},
		Right: &ast.IntLit{Pos: pos, Value: 2},
	}
	_ = pair

	structExpr := &ast.StructExpr{
		Pos:      pos,
		TypeName: "MyStruct",
		Members:  []ast.StructMember{{Name: "x", Value: &ast.IntLit{Pos: pos, Value: 5}}},
	}
	if structExpr.TypeName != "MyStruct" {
		t.Error("StructExpr.TypeName should be set")
	}

	null := &ast.NullLit{Pos: pos}
	_ = null

	// StringExpr with parts
	strExpr := &ast.StringExpr{
		Pos: pos,
		Parts: []ast.StringPart{
			&ast.RawStringPart{Val: "hello "},
			&ast.PlaceholderPart{Pos: pos, Expr: &ast.IdentExpr{Pos: pos, Name: "name"}},
		},
	}
	if len(strExpr.Parts) != 2 {
		t.Error("StringExpr.Parts should have 2 elements")
	}

	// CommandExpr
	cmdExpr := &ast.CommandExpr{
		Pos:   pos,
		Style: ast.CommandHeredoc,
		Parts: []ast.StringPart{
			&ast.RawStringPart{Val: "echo hello"},
		},
	}
	if cmdExpr.Style != ast.CommandHeredoc {
		t.Error("CommandExpr.Style should be CommandHeredoc")
	}
}

// TestPosFileIsAbsolute documents the expectation that Pos.File holds an absolute path.
func TestPosFileIsAbsolute(t *testing.T) {
	task := &ast.Task{
		Pos:  ast.Pos{File: "/abs/path/to/file.wdl", Line: 5, Col: 1},
		Name: "my_task",
	}
	if task.Pos.File[0] != '/' {
		t.Errorf("Pos.File should be absolute path, got: %s", task.Pos.File)
	}
	if task.Pos.Line != 5 {
		t.Error("Pos.Line should be 5")
	}
}
