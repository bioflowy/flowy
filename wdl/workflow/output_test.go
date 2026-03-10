package workflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bioflowy/flowy/wdl/task"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// readJSONFile reads a JSON file and returns its parsed content.
func readJSONFile(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading JSON file %q: %v", path, err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("parsing JSON file %q: %v", path, err)
	}
	return result
}

// TestWriteOutputsJSON_Simple: workflow "myWf" with output "result"=IntValue(42)
// -> file contains {"myWf.result": 42}
func TestWriteOutputsJSON_Simple(t *testing.T) {
	dir := t.TempDir()
	destPath := filepath.Join(dir, "outputs.json")

	outputs := map[string]values.Value{
		"result": values.IntValue{Value: 42},
	}

	if err := writeOutputsJSON("myWf", outputs, dir, destPath); err != nil {
		t.Fatalf("writeOutputsJSON: %v", err)
	}

	got := readJSONFile(t, destPath)
	v, ok := got["myWf.result"]
	if !ok {
		t.Fatal("expected key 'myWf.result' in outputs.json")
	}
	// JSON numbers unmarshal as float64.
	if v.(float64) != 42 {
		t.Errorf("expected 42, got %v", v)
	}
}

// TestWriteOutputsJSON_FileRelativePath: FileValue("/tmp/exec/work/out.txt") with execRoot="/tmp/exec"
// -> value is "work/out.txt" (relative).
func TestWriteOutputsJSON_FileRelativePath(t *testing.T) {
	dir := t.TempDir()
	destPath := filepath.Join(dir, "outputs.json")
	execRoot := "/tmp/exec"

	outputs := map[string]values.Value{
		"outfile": values.FileValue{Value: "/tmp/exec/work/out.txt"},
	}

	if err := writeOutputsJSON("wf", outputs, execRoot, destPath); err != nil {
		t.Fatalf("writeOutputsJSON: %v", err)
	}

	got := readJSONFile(t, destPath)
	v, ok := got["wf.outfile"]
	if !ok {
		t.Fatal("expected key 'wf.outfile' in outputs.json")
	}
	if v.(string) != "work/out.txt" {
		t.Errorf("expected 'work/out.txt', got %q", v)
	}
}

// TestWriteOutputsJSON_DirectoryRelativePath: DirectoryValue absolute -> relative.
func TestWriteOutputsJSON_DirectoryRelativePath(t *testing.T) {
	dir := t.TempDir()
	destPath := filepath.Join(dir, "outputs.json")
	execRoot := "/data/run"

	outputs := map[string]values.Value{
		"outdir": values.DirectoryValue{Value: "/data/run/results/dir"},
	}

	if err := writeOutputsJSON("wf", outputs, execRoot, destPath); err != nil {
		t.Fatalf("writeOutputsJSON: %v", err)
	}

	got := readJSONFile(t, destPath)
	v, ok := got["wf.outdir"]
	if !ok {
		t.Fatal("expected key 'wf.outdir' in outputs.json")
	}
	if v.(string) != "results/dir" {
		t.Errorf("expected 'results/dir', got %q", v)
	}
}

// TestWriteOutputsJSON_Array: ArrayValue of StringValues -> JSON array of strings.
func TestWriteOutputsJSON_Array(t *testing.T) {
	dir := t.TempDir()
	destPath := filepath.Join(dir, "outputs.json")

	outputs := map[string]values.Value{
		"names": values.ArrayValue{
			Type: types.ArrayType{ItemType: types.StringType{}},
			Items: []values.Value{
				values.StringValue{Value: "alice"},
				values.StringValue{Value: "bob"},
			},
		},
	}

	if err := writeOutputsJSON("wf", outputs, "/exec", destPath); err != nil {
		t.Fatalf("writeOutputsJSON: %v", err)
	}

	got := readJSONFile(t, destPath)
	v, ok := got["wf.names"]
	if !ok {
		t.Fatal("expected key 'wf.names' in outputs.json")
	}
	arr, ok := v.([]interface{})
	if !ok {
		t.Fatalf("expected array, got %T", v)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 items, got %d", len(arr))
	}
	if arr[0].(string) != "alice" || arr[1].(string) != "bob" {
		t.Errorf("expected [alice, bob], got %v", arr)
	}
}

// TestWriteOutputsJSON_Null: NullValue -> null in JSON.
func TestWriteOutputsJSON_Null(t *testing.T) {
	dir := t.TempDir()
	destPath := filepath.Join(dir, "outputs.json")

	outputs := map[string]values.Value{
		"maybe": values.NullValue{},
	}

	if err := writeOutputsJSON("wf", outputs, "/exec", destPath); err != nil {
		t.Fatalf("writeOutputsJSON: %v", err)
	}

	// We need to read raw JSON to check for null.
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading outputs.json: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parsing outputs.json: %v", err)
	}
	v, ok := raw["wf.maybe"]
	if !ok {
		t.Fatal("expected key 'wf.maybe' in outputs.json")
	}
	if v != nil {
		t.Errorf("expected null, got %v", v)
	}
}

// TestWriteOutputsJSON_MultipleOutputs: two outputs -> both present with workflow_name prefix.
func TestWriteOutputsJSON_MultipleOutputs(t *testing.T) {
	dir := t.TempDir()
	destPath := filepath.Join(dir, "outputs.json")

	outputs := map[string]values.Value{
		"x": values.IntValue{Value: 1},
		"y": values.StringValue{Value: "hello"},
	}

	if err := writeOutputsJSON("myWorkflow", outputs, "/exec", destPath); err != nil {
		t.Fatalf("writeOutputsJSON: %v", err)
	}

	got := readJSONFile(t, destPath)
	if _, ok := got["myWorkflow.x"]; !ok {
		t.Error("expected key 'myWorkflow.x'")
	}
	if _, ok := got["myWorkflow.y"]; !ok {
		t.Error("expected key 'myWorkflow.y'")
	}
}

// TestWriteErrorJSON: TaskError{TaskName:"foo", ExitCode:1}
// -> {"error": "...", "task": "foo", "exit_code": 1}
func TestWriteErrorJSON(t *testing.T) {
	dir := t.TempDir()
	destPath := filepath.Join(dir, "error.json")

	taskErr := &task.TaskError{
		TaskName: "foo",
		ExitCode: 1,
		Stderr:   []byte("something failed"),
	}

	if err := writeErrorJSON(taskErr, destPath); err != nil {
		t.Fatalf("writeErrorJSON: %v", err)
	}

	got := readJSONFile(t, destPath)

	if got["task"].(string) != "foo" {
		t.Errorf("expected task='foo', got %v", got["task"])
	}
	exitCode, ok := got["exit_code"]
	if !ok {
		t.Fatal("expected 'exit_code' in error.json")
	}
	if exitCode.(float64) != 1 {
		t.Errorf("expected exit_code=1, got %v", exitCode)
	}
	if got["error"] == nil || got["error"].(string) == "" {
		t.Error("expected non-empty 'error' message")
	}
}

// TestWriteErrorJSON_NonTaskError: generic error
// -> {"error": "msg", "task": "", "exit_code": -1}
func TestWriteErrorJSON_NonTaskError(t *testing.T) {
	dir := t.TempDir()
	destPath := filepath.Join(dir, "error.json")

	genericErr := &testError{msg: "generic failure"}

	if err := writeErrorJSON(genericErr, destPath); err != nil {
		t.Fatalf("writeErrorJSON: %v", err)
	}

	got := readJSONFile(t, destPath)

	if got["task"].(string) != "" {
		t.Errorf("expected task='', got %v", got["task"])
	}
	exitCode := got["exit_code"].(float64)
	if exitCode != -1 {
		t.Errorf("expected exit_code=-1, got %v", exitCode)
	}
	if got["error"].(string) != "generic failure" {
		t.Errorf("expected error='generic failure', got %v", got["error"])
	}
}

// testError is a generic error type for testing writeErrorJSON with non-TaskError.
type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
