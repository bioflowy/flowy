package task_test

import (
	"testing"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/task"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

func TestBindInputs_String(t *testing.T) {
	taskAST := &ast.Task{
		Name: "test_task",
		Inputs: []*ast.Decl{
			{Name: "sample_name", Type: types.StringType{}},
		},
	}
	jsonInputs := map[string]interface{}{
		"sample_name": "NA12878",
	}
	pm := task.NewPathMap()
	env, _, err := task.BindInputs(taskAST, jsonInputs, pm)
	if err != nil {
		t.Fatalf("BindInputs error: %v", err)
	}
	v, ok := env.Lookup("sample_name")
	if !ok {
		t.Fatal("sample_name not found in env")
	}
	sv, ok := v.(values.StringValue)
	if !ok {
		t.Fatalf("expected StringValue, got %T", v)
	}
	if sv.Value != "NA12878" {
		t.Errorf("sample_name = %q, want %q", sv.Value, "NA12878")
	}
}

func TestBindInputs_FileType(t *testing.T) {
	taskAST := &ast.Task{
		Name: "test_task",
		Inputs: []*ast.Decl{
			{Name: "bam_file", Type: types.FileType{}},
		},
	}
	jsonInputs := map[string]interface{}{
		"bam_file": "/host/data/input.bam",
	}
	pm := task.NewPathMap()
	env, _, err := task.BindInputs(taskAST, jsonInputs, pm)
	if err != nil {
		t.Fatalf("BindInputs error: %v", err)
	}
	v, ok := env.Lookup("bam_file")
	if !ok {
		t.Fatal("bam_file not found in env")
	}
	fv, ok := v.(values.FileValue)
	if !ok {
		t.Fatalf("expected FileValue, got %T", v)
	}
	if fv.Value != "/host/data/input.bam" {
		t.Errorf("bam_file.Value = %q, want %q", fv.Value, "/host/data/input.bam")
	}
	// Verify PathMap was populated
	binds := pm.Binds()
	if len(binds) == 0 {
		t.Error("expected PathMap to have binds after File input, got none")
	}
}

func TestBindInputs_MissingRequired(t *testing.T) {
	taskAST := &ast.Task{
		Name: "test_task",
		Inputs: []*ast.Decl{
			{Name: "required_file", Type: types.FileType{}, Expr: nil},
		},
	}
	jsonInputs := map[string]interface{}{} // missing required input
	pm := task.NewPathMap()
	_, _, err := task.BindInputs(taskAST, jsonInputs, pm)
	if err == nil {
		t.Error("expected error for missing required input, got nil")
	}
}

func TestBindInputs_MissingOptional(t *testing.T) {
	taskAST := &ast.Task{
		Name: "test_task",
		Inputs: []*ast.Decl{
			{Name: "opt_file", Type: types.FileType{Optional: true}, Expr: nil},
		},
	}
	jsonInputs := map[string]interface{}{} // missing optional input
	pm := task.NewPathMap()
	env, _, err := task.BindInputs(taskAST, jsonInputs, pm)
	if err != nil {
		t.Fatalf("BindInputs error for optional input: %v", err)
	}
	v, ok := env.Lookup("opt_file")
	if !ok {
		t.Fatal("opt_file not found in env (should be NullValue)")
	}
	if _, ok := v.(values.NullValue); !ok {
		t.Errorf("optional missing input should be NullValue, got %T", v)
	}
}

func TestBindInputs_DefaultExpression(t *testing.T) {
	// Input with a default expression (string literal "default_value")
	defaultExpr := &ast.StringExpr{
		Parts: []ast.StringPart{
			&ast.RawStringPart{Val: "default_value"},
		},
	}
	taskAST := &ast.Task{
		Name: "test_task",
		Inputs: []*ast.Decl{
			{Name: "with_default", Type: types.StringType{}, Expr: defaultExpr},
		},
	}
	jsonInputs := map[string]interface{}{} // not provided, should use default
	pm := task.NewPathMap()
	env, _, err := task.BindInputs(taskAST, jsonInputs, pm)
	if err != nil {
		t.Fatalf("BindInputs error: %v", err)
	}
	v, ok := env.Lookup("with_default")
	if !ok {
		t.Fatal("with_default not found in env")
	}
	sv, ok := v.(values.StringValue)
	if !ok {
		t.Fatalf("expected StringValue, got %T", v)
	}
	if sv.Value != "default_value" {
		t.Errorf("with_default = %q, want %q", sv.Value, "default_value")
	}
}

func TestBindInputs_EnvModifier(t *testing.T) {
	taskAST := &ast.Task{
		Name: "test_task",
		Inputs: []*ast.Decl{
			{Name: "SAMPLE_ID", Type: types.StringType{}, Env: true},
		},
	}
	jsonInputs := map[string]interface{}{
		"SAMPLE_ID": "NA12878",
	}
	pm := task.NewPathMap()
	_, envVars, err := task.BindInputs(taskAST, jsonInputs, pm)
	if err != nil {
		t.Fatalf("BindInputs error: %v", err)
	}
	found := false
	for _, ev := range envVars {
		if ev == "SAMPLE_ID=NA12878" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'SAMPLE_ID=NA12878' in envVars, got %v", envVars)
	}
}

func TestBindInputs_IntType(t *testing.T) {
	taskAST := &ast.Task{
		Name: "test_task",
		Inputs: []*ast.Decl{
			{Name: "count", Type: types.IntType{}},
		},
	}
	jsonInputs := map[string]interface{}{
		"count": float64(42), // JSON numbers are float64
	}
	pm := task.NewPathMap()
	env, _, err := task.BindInputs(taskAST, jsonInputs, pm)
	if err != nil {
		t.Fatalf("BindInputs error: %v", err)
	}
	v, ok := env.Lookup("count")
	if !ok {
		t.Fatal("count not found in env")
	}
	iv, ok := v.(values.IntValue)
	if !ok {
		t.Fatalf("expected IntValue, got %T", v)
	}
	if iv.Value != 42 {
		t.Errorf("count = %d, want 42", iv.Value)
	}
}

func TestBindInputs_DirectoryType(t *testing.T) {
	taskAST := &ast.Task{
		Name: "test_task",
		Inputs: []*ast.Decl{
			{Name: "ref_dir", Type: types.DirectoryType{}},
		},
	}
	jsonInputs := map[string]interface{}{
		"ref_dir": "/host/refdata/",
	}
	pm := task.NewPathMap()
	env, _, err := task.BindInputs(taskAST, jsonInputs, pm)
	if err != nil {
		t.Fatalf("BindInputs error: %v", err)
	}
	v, ok := env.Lookup("ref_dir")
	if !ok {
		t.Fatal("ref_dir not found in env")
	}
	dv, ok := v.(values.DirectoryValue)
	if !ok {
		t.Fatalf("expected DirectoryValue, got %T", v)
	}
	if dv.Value != "/host/refdata/" {
		t.Errorf("ref_dir.Value = %q, want %q", dv.Value, "/host/refdata/")
	}
	// Verify PathMap was populated
	binds := pm.Binds()
	if len(binds) == 0 {
		t.Error("expected PathMap to have binds after Directory input, got none")
	}
}
