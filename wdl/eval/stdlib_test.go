package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// writeFile is a test helper that writes content to a named file under dir.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	return path
}

// callStdlib invokes a registered stdlib function directly, bypassing Eval().
func callStdlib(t *testing.T, name string, args []values.Value, ctx ExecContext) (values.Value, error) {
	t.Helper()
	fn, ok := stdlibFuncs[name]
	if !ok {
		t.Fatalf("stdlib function %q not registered", name)
	}
	return fn(args, ctx)
}

// TestStdlibFileIO tests file I/O standard library functions (STDLIB-01).
// Covers: read_string, read_int, read_float, read_boolean, read_lines, read_tsv,
// read_map, read_json, write_lines, write_tsv, write_map, write_json.
func TestStdlibFileIO(t *testing.T) {
	ctx := ExecContext{}
	dir := t.TempDir()

	// --- read_string ---
	t.Run("read_string strips trailing newline", func(t *testing.T) {
		p := writeFile(t, dir, "str.txt", "hello\n")
		got, err := callStdlib(t, "read_string", []values.Value{values.FileValue{Value: p}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		sv, ok := got.(values.StringValue)
		if !ok {
			t.Fatalf("expected StringValue, got %T", got)
		}
		if sv.Value != "hello" {
			t.Errorf("got %q, want %q", sv.Value, "hello")
		}
	})

	t.Run("read_string no trailing newline", func(t *testing.T) {
		p := writeFile(t, dir, "str2.txt", "world")
		got, err := callStdlib(t, "read_string", []values.Value{values.FileValue{Value: p}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		sv := got.(values.StringValue)
		if sv.Value != "world" {
			t.Errorf("got %q, want %q", sv.Value, "world")
		}
	})

	// --- read_int ---
	t.Run("read_int", func(t *testing.T) {
		p := writeFile(t, dir, "int.txt", "42\n")
		got, err := callStdlib(t, "read_int", []values.Value{values.FileValue{Value: p}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		iv := got.(values.IntValue)
		if iv.Value != 42 {
			t.Errorf("got %d, want 42", iv.Value)
		}
	})

	// --- read_float ---
	t.Run("read_float", func(t *testing.T) {
		p := writeFile(t, dir, "float.txt", "3.14\n")
		got, err := callStdlib(t, "read_float", []values.Value{values.FileValue{Value: p}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fv := got.(values.FloatValue)
		if fv.Value != 3.14 {
			t.Errorf("got %v, want 3.14", fv.Value)
		}
	})

	// --- read_boolean ---
	t.Run("read_boolean true", func(t *testing.T) {
		p := writeFile(t, dir, "bool_true.txt", "true\n")
		got, err := callStdlib(t, "read_boolean", []values.Value{values.FileValue{Value: p}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		bv := got.(values.BooleanValue)
		if !bv.Value {
			t.Error("expected true")
		}
	})

	t.Run("read_boolean false", func(t *testing.T) {
		p := writeFile(t, dir, "bool_false.txt", "false\n")
		got, err := callStdlib(t, "read_boolean", []values.Value{values.FileValue{Value: p}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		bv := got.(values.BooleanValue)
		if bv.Value {
			t.Error("expected false")
		}
	})

	// --- read_lines ---
	t.Run("read_lines with trailing newline", func(t *testing.T) {
		p := writeFile(t, dir, "lines.txt", "a\nb\nc\n")
		got, err := callStdlib(t, "read_lines", []values.Value{values.FileValue{Value: p}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av := got.(values.ArrayValue)
		if len(av.Items) != 3 {
			t.Fatalf("expected 3 items, got %d", len(av.Items))
		}
		want := []string{"a", "b", "c"}
		for i, item := range av.Items {
			sv := item.(values.StringValue)
			if sv.Value != want[i] {
				t.Errorf("item %d: got %q, want %q", i, sv.Value, want[i])
			}
		}
	})

	t.Run("read_lines no trailing newline", func(t *testing.T) {
		p := writeFile(t, dir, "lines2.txt", "a\nb")
		got, err := callStdlib(t, "read_lines", []values.Value{values.FileValue{Value: p}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av := got.(values.ArrayValue)
		if len(av.Items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(av.Items))
		}
	})

	// --- read_tsv ---
	t.Run("read_tsv", func(t *testing.T) {
		p := writeFile(t, dir, "data.tsv", "a\tb\tc\nd\te\tf\n")
		got, err := callStdlib(t, "read_tsv", []values.Value{values.FileValue{Value: p}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av := got.(values.ArrayValue)
		if len(av.Items) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(av.Items))
		}
		row0 := av.Items[0].(values.ArrayValue)
		if len(row0.Items) != 3 {
			t.Fatalf("row 0: expected 3 fields, got %d", len(row0.Items))
		}
		if row0.Items[0].(values.StringValue).Value != "a" {
			t.Errorf("row0[0]: got %q, want %q", row0.Items[0].(values.StringValue).Value, "a")
		}
		if row0.Items[2].(values.StringValue).Value != "c" {
			t.Errorf("row0[2]: got %q, want %q", row0.Items[2].(values.StringValue).Value, "c")
		}
	})

	// --- read_map ---
	t.Run("read_map", func(t *testing.T) {
		p := writeFile(t, dir, "map.tsv", "key1\tval1\nkey2\tval2\n")
		got, err := callStdlib(t, "read_map", []values.Value{values.FileValue{Value: p}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		mv := got.(values.MapValue)
		if len(mv.Entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(mv.Entries))
		}
		if mv.Entries[0].Key.(values.StringValue).Value != "key1" {
			t.Errorf("entry 0 key: got %q, want %q", mv.Entries[0].Key.(values.StringValue).Value, "key1")
		}
		if mv.Entries[0].Value.(values.StringValue).Value != "val1" {
			t.Errorf("entry 0 value: got %q, want %q", mv.Entries[0].Value.(values.StringValue).Value, "val1")
		}
	})

	// --- read_json ---
	t.Run("read_json object", func(t *testing.T) {
		p := writeFile(t, dir, "obj.json", `{"x": 1}`)
		got, err := callStdlib(t, "read_json", []values.Value{values.FileValue{Value: p}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		mv, ok := got.(values.MapValue)
		if !ok {
			t.Fatalf("expected MapValue, got %T", got)
		}
		if len(mv.Entries) != 1 {
			t.Errorf("expected 1 entry, got %d", len(mv.Entries))
		}
	})

	t.Run("read_json array", func(t *testing.T) {
		p := writeFile(t, dir, "arr.json", `[1, 2, 3]`)
		got, err := callStdlib(t, "read_json", []values.Value{values.FileValue{Value: p}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av, ok := got.(values.ArrayValue)
		if !ok {
			t.Fatalf("expected ArrayValue, got %T", got)
		}
		if len(av.Items) != 3 {
			t.Errorf("expected 3 items, got %d", len(av.Items))
		}
	})

	t.Run("read_json string", func(t *testing.T) {
		p := writeFile(t, dir, "str.json", `"hello"`)
		got, err := callStdlib(t, "read_json", []values.Value{values.FileValue{Value: p}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		sv, ok := got.(values.StringValue)
		if !ok {
			t.Fatalf("expected StringValue, got %T", got)
		}
		if sv.Value != "hello" {
			t.Errorf("got %q, want %q", sv.Value, "hello")
		}
	})

	t.Run("read_json boolean", func(t *testing.T) {
		p := writeFile(t, dir, "bool.json", `true`)
		got, err := callStdlib(t, "read_json", []values.Value{values.FileValue{Value: p}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		bv, ok := got.(values.BooleanValue)
		if !ok {
			t.Fatalf("expected BooleanValue, got %T", got)
		}
		if !bv.Value {
			t.Error("expected true")
		}
	})

	t.Run("read_json integer", func(t *testing.T) {
		p := writeFile(t, dir, "int.json", `42`)
		got, err := callStdlib(t, "read_json", []values.Value{values.FileValue{Value: p}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		iv, ok := got.(values.IntValue)
		if !ok {
			t.Fatalf("expected IntValue for whole number JSON, got %T", got)
		}
		if iv.Value != 42 {
			t.Errorf("got %d, want 42", iv.Value)
		}
	})

	t.Run("read_json float", func(t *testing.T) {
		p := writeFile(t, dir, "float.json", `3.14`)
		got, err := callStdlib(t, "read_json", []values.Value{values.FileValue{Value: p}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fv, ok := got.(values.FloatValue)
		if !ok {
			t.Fatalf("expected FloatValue for fractional JSON, got %T", got)
		}
		if fv.Value != 3.14 {
			t.Errorf("got %v, want 3.14", fv.Value)
		}
	})

	// --- write_lines ---
	t.Run("write_lines produces file with lines", func(t *testing.T) {
		items := []values.Value{
			values.StringValue{Type: types.StringType{}, Value: "line1"},
			values.StringValue{Type: types.StringType{}, Value: "line2"},
			values.StringValue{Type: types.StringType{}, Value: "line3"},
		}
		arr := values.ArrayValue{
			Type:  types.ArrayType{ItemType: types.StringType{}},
			Items: items,
		}
		got, err := callStdlib(t, "write_lines", []values.Value{arr}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fv, ok := got.(values.FileValue)
		if !ok {
			t.Fatalf("expected FileValue, got %T", got)
		}
		data, readErr := os.ReadFile(fv.Value)
		if readErr != nil {
			t.Fatalf("cannot read output file: %v", readErr)
		}
		want := "line1\nline2\nline3\n"
		if string(data) != want {
			t.Errorf("got %q, want %q", string(data), want)
		}
	})

	// --- write_tsv ---
	t.Run("write_tsv produces file with TSV content", func(t *testing.T) {
		row0 := values.ArrayValue{
			Type:  types.ArrayType{ItemType: types.StringType{}},
			Items: []values.Value{values.StringValue{Value: "a"}, values.StringValue{Value: "b"}},
		}
		row1 := values.ArrayValue{
			Type:  types.ArrayType{ItemType: types.StringType{}},
			Items: []values.Value{values.StringValue{Value: "c"}, values.StringValue{Value: "d"}},
		}
		arr := values.ArrayValue{
			Type:  types.ArrayType{ItemType: types.ArrayType{ItemType: types.StringType{}}},
			Items: []values.Value{row0, row1},
		}
		got, err := callStdlib(t, "write_tsv", []values.Value{arr}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fv := got.(values.FileValue)
		data, _ := os.ReadFile(fv.Value)
		want := "a\tb\nc\td\n"
		if string(data) != want {
			t.Errorf("got %q, want %q", string(data), want)
		}
	})

	// --- write_map ---
	t.Run("write_map produces key-tab-value per line", func(t *testing.T) {
		mv := values.MapValue{
			Type: types.MapType{KeyType: types.StringType{}, ValType: types.StringType{}},
			Entries: []values.MapEntry{
				{Key: values.StringValue{Value: "k1"}, Value: values.StringValue{Value: "v1"}},
				{Key: values.StringValue{Value: "k2"}, Value: values.StringValue{Value: "v2"}},
			},
		}
		got, err := callStdlib(t, "write_map", []values.Value{mv}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fv := got.(values.FileValue)
		data, _ := os.ReadFile(fv.Value)
		want := "k1\tv1\nk2\tv2\n"
		if string(data) != want {
			t.Errorf("got %q, want %q", string(data), want)
		}
	})

	// --- write_json ---
	t.Run("write_json writes valid JSON", func(t *testing.T) {
		arr := values.ArrayValue{
			Type:  types.ArrayType{ItemType: types.IntType{}},
			Items: []values.Value{values.IntValue{Value: 1}, values.IntValue{Value: 2}},
		}
		got, err := callStdlib(t, "write_json", []values.Value{arr}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fv := got.(values.FileValue)
		data, _ := os.ReadFile(fv.Value)
		var decoded []interface{}
		if jsonErr := json.Unmarshal(data, &decoded); jsonErr != nil {
			t.Fatalf("output is not valid JSON: %v", jsonErr)
		}
		if len(decoded) != 2 {
			t.Errorf("expected 2 elements, got %d", len(decoded))
		}
	})
}

// TestStdlibFilesystem tests filesystem standard library functions (STDLIB-02).
// Covers: glob, size, basename, dirname, stdout, stderr.
func TestStdlibFilesystem(t *testing.T) {
	ctx := ExecContext{}
	dir := t.TempDir()

	// --- glob ---
	t.Run("glob matches txt files", func(t *testing.T) {
		workDir := t.TempDir()
		writeFile(t, workDir, "a.txt", "a")
		writeFile(t, workDir, "b.txt", "b")
		writeFile(t, workDir, "c.dat", "c")
		globCtx := ExecContext{WorkDir: workDir}
		got, err := callStdlib(t, "glob", []values.Value{values.StringValue{Value: "*.txt"}}, globCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av := got.(values.ArrayValue)
		if len(av.Items) != 2 {
			t.Errorf("expected 2 matches, got %d", len(av.Items))
		}
		for _, item := range av.Items {
			_, ok := item.(values.FileValue)
			if !ok {
				t.Errorf("expected FileValue items, got %T", item)
			}
		}
	})

	t.Run("glob with empty WorkDir returns EvalError", func(t *testing.T) {
		_, err := callStdlib(t, "glob", []values.Value{values.StringValue{Value: "*.txt"}}, ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	// --- basename ---
	t.Run("basename returns filename", func(t *testing.T) {
		got, err := callStdlib(t, "basename", []values.Value{values.FileValue{Value: "/a/b/c.txt"}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		sv := got.(values.StringValue)
		if sv.Value != "c.txt" {
			t.Errorf("got %q, want %q", sv.Value, "c.txt")
		}
	})

	t.Run("basename on StringValue", func(t *testing.T) {
		got, err := callStdlib(t, "basename", []values.Value{values.StringValue{Value: "/x/y/z.wdl"}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		sv := got.(values.StringValue)
		if sv.Value != "z.wdl" {
			t.Errorf("got %q, want %q", sv.Value, "z.wdl")
		}
	})

	// --- dirname ---
	t.Run("dirname returns directory", func(t *testing.T) {
		got, err := callStdlib(t, "dirname", []values.Value{values.FileValue{Value: "/a/b/c.txt"}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		sv := got.(values.StringValue)
		if sv.Value != "/a/b" {
			t.Errorf("got %q, want %q", sv.Value, "/a/b")
		}
	})

	// --- size ---
	t.Run("size default bytes", func(t *testing.T) {
		p := writeFile(t, dir, "sizeme.txt", "hello")
		got, err := callStdlib(t, "size", []values.Value{values.FileValue{Value: p}}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fv := got.(values.FloatValue)
		if fv.Value != 5.0 {
			t.Errorf("got %v, want 5.0", fv.Value)
		}
	})

	t.Run("size KiB unit", func(t *testing.T) {
		// write 1024 bytes
		data := make([]byte, 1024)
		for i := range data {
			data[i] = 'x'
		}
		p := filepath.Join(dir, "kib.bin")
		if err := os.WriteFile(p, data, 0644); err != nil {
			t.Fatalf("writeFile: %v", err)
		}
		got, err := callStdlib(t, "size", []values.Value{
			values.FileValue{Value: p},
			values.StringValue{Value: "KiB"},
		}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fv := got.(values.FloatValue)
		if fv.Value != 1.0 {
			t.Errorf("got %v, want 1.0", fv.Value)
		}
	})

	// --- stdout ---
	t.Run("stdout returns FileValue", func(t *testing.T) {
		stdoutCtx := ExecContext{StdoutPath: "/tmp/out"}
		got, err := callStdlib(t, "stdout", []values.Value{}, stdoutCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fv := got.(values.FileValue)
		if fv.Value != "/tmp/out" {
			t.Errorf("got %q, want %q", fv.Value, "/tmp/out")
		}
	})

	t.Run("stdout with empty StdoutPath returns EvalError", func(t *testing.T) {
		_, err := callStdlib(t, "stdout", []values.Value{}, ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	// --- stderr ---
	t.Run("stderr returns FileValue", func(t *testing.T) {
		stderrCtx := ExecContext{StderrPath: "/tmp/err"}
		got, err := callStdlib(t, "stderr", []values.Value{}, stderrCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fv := got.(values.FileValue)
		if fv.Value != "/tmp/err" {
			t.Errorf("got %q, want %q", fv.Value, "/tmp/err")
		}
	})

	t.Run("stderr with empty StderrPath returns EvalError", func(t *testing.T) {
		_, err := callStdlib(t, "stderr", []values.Value{}, ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// TestStdlibArray tests array standard library functions (STDLIB-03).
// Covers: length, flatten, transpose, zip, cross, prefix, suffix, quote,
// squote, unzip, select_first, select_all, chunk.
func TestStdlibArray(t *testing.T) {
	ctx := ExecContext{}

	intArr := func(vals ...int64) values.ArrayValue {
		items := make([]values.Value, len(vals))
		for i, v := range vals {
			items[i] = values.IntValue{Type: types.IntType{}, Value: v}
		}
		return values.ArrayValue{Type: types.ArrayType{ItemType: types.IntType{}}, Items: items}
	}
	strArr := func(vals ...string) values.ArrayValue {
		items := make([]values.Value, len(vals))
		for i, v := range vals {
			items[i] = values.StringValue{Type: types.StringType{}, Value: v}
		}
		return values.ArrayValue{Type: types.ArrayType{ItemType: types.StringType{}}, Items: items}
	}
	pairVal := func(l, r values.Value) values.PairValue {
		return values.PairValue{
			Type:  types.PairType{LeftType: l.ValueType(), RightType: r.ValueType()},
			Left:  l,
			Right: r,
		}
	}

	// --- length ---
	t.Run("length non-empty", func(t *testing.T) {
		got, err := callStdlib(t, "length", []values.Value{intArr(1, 2, 3)}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		iv := got.(values.IntValue)
		if iv.Value != 3 {
			t.Errorf("got %d, want 3", iv.Value)
		}
	})
	t.Run("length empty", func(t *testing.T) {
		got, err := callStdlib(t, "length", []values.Value{intArr()}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		iv := got.(values.IntValue)
		if iv.Value != 0 {
			t.Errorf("got %d, want 0", iv.Value)
		}
	})

	// --- flatten ---
	t.Run("flatten [[1,2],[3,4]]", func(t *testing.T) {
		inner1 := intArr(1, 2)
		inner2 := intArr(3, 4)
		outer := values.ArrayValue{
			Type:  types.ArrayType{ItemType: types.ArrayType{ItemType: types.IntType{}}},
			Items: []values.Value{inner1, inner2},
		}
		got, err := callStdlib(t, "flatten", []values.Value{outer}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av := got.(values.ArrayValue)
		if len(av.Items) != 4 {
			t.Fatalf("expected 4 items, got %d", len(av.Items))
		}
		for i, want := range []int64{1, 2, 3, 4} {
			if av.Items[i].(values.IntValue).Value != want {
				t.Errorf("items[%d]=%d, want %d", i, av.Items[i].(values.IntValue).Value, want)
			}
		}
	})
	t.Run("flatten with empty sub-array", func(t *testing.T) {
		outer := values.ArrayValue{
			Type:  types.ArrayType{ItemType: types.ArrayType{ItemType: types.IntType{}}},
			Items: []values.Value{intArr(1), intArr(), intArr(2, 3)},
		}
		got, err := callStdlib(t, "flatten", []values.Value{outer}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av := got.(values.ArrayValue)
		if len(av.Items) != 3 {
			t.Fatalf("expected 3 items, got %d", len(av.Items))
		}
	})

	// --- transpose ---
	t.Run("transpose [[1,2],[3,4]]", func(t *testing.T) {
		outer := values.ArrayValue{
			Type:  types.ArrayType{ItemType: types.ArrayType{ItemType: types.IntType{}}},
			Items: []values.Value{intArr(1, 2), intArr(3, 4)},
		}
		got, err := callStdlib(t, "transpose", []values.Value{outer}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av := got.(values.ArrayValue)
		if len(av.Items) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(av.Items))
		}
		row0 := av.Items[0].(values.ArrayValue)
		row1 := av.Items[1].(values.ArrayValue)
		if row0.Items[0].(values.IntValue).Value != 1 || row0.Items[1].(values.IntValue).Value != 3 {
			t.Errorf("row0=%v, want [1,3]", row0.Items)
		}
		if row1.Items[0].(values.IntValue).Value != 2 || row1.Items[1].(values.IntValue).Value != 4 {
			t.Errorf("row1=%v, want [2,4]", row1.Items)
		}
	})

	// --- zip ---
	t.Run("zip [1,2] [a,b]", func(t *testing.T) {
		got, err := callStdlib(t, "zip", []values.Value{intArr(1, 2), strArr("a", "b")}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av := got.(values.ArrayValue)
		if len(av.Items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(av.Items))
		}
		p0 := av.Items[0].(values.PairValue)
		if p0.Left.(values.IntValue).Value != 1 || p0.Right.(values.StringValue).Value != "a" {
			t.Errorf("pair[0]=%v, want (1,a)", p0)
		}
	})

	// --- cross ---
	t.Run("cross [1,2] [a,b]", func(t *testing.T) {
		got, err := callStdlib(t, "cross", []values.Value{intArr(1, 2), strArr("a", "b")}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av := got.(values.ArrayValue)
		if len(av.Items) != 4 {
			t.Fatalf("expected 4 items (Cartesian), got %d", len(av.Items))
		}
		// order: (1,a),(1,b),(2,a),(2,b)
		expected := []struct{ l int64; r string }{{1, "a"}, {1, "b"}, {2, "a"}, {2, "b"}}
		for i, e := range expected {
			p := av.Items[i].(values.PairValue)
			if p.Left.(values.IntValue).Value != e.l || p.Right.(values.StringValue).Value != e.r {
				t.Errorf("item[%d]=(%v,%v), want (%d,%s)", i, p.Left, p.Right, e.l, e.r)
			}
		}
	})

	// --- prefix ---
	t.Run("prefix", func(t *testing.T) {
		got, err := callStdlib(t, "prefix", []values.Value{
			values.StringValue{Type: types.StringType{}, Value: "--"},
			strArr("a", "b"),
		}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av := got.(values.ArrayValue)
		if len(av.Items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(av.Items))
		}
		if av.Items[0].(values.StringValue).Value != "--a" {
			t.Errorf("got %q, want %q", av.Items[0].(values.StringValue).Value, "--a")
		}
		if av.Items[1].(values.StringValue).Value != "--b" {
			t.Errorf("got %q, want %q", av.Items[1].(values.StringValue).Value, "--b")
		}
	})

	// --- suffix ---
	t.Run("suffix", func(t *testing.T) {
		got, err := callStdlib(t, "suffix", []values.Value{
			values.StringValue{Type: types.StringType{}, Value: ".txt"},
			strArr("a", "b"),
		}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av := got.(values.ArrayValue)
		if av.Items[0].(values.StringValue).Value != "a.txt" {
			t.Errorf("got %q, want %q", av.Items[0].(values.StringValue).Value, "a.txt")
		}
	})

	// --- quote ---
	t.Run("quote", func(t *testing.T) {
		got, err := callStdlib(t, "quote", []values.Value{strArr("a", "b")}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av := got.(values.ArrayValue)
		if av.Items[0].(values.StringValue).Value != `"a"` {
			t.Errorf("got %q, want %q", av.Items[0].(values.StringValue).Value, `"a"`)
		}
		if av.Items[1].(values.StringValue).Value != `"b"` {
			t.Errorf("got %q, want %q", av.Items[1].(values.StringValue).Value, `"b"`)
		}
	})

	// --- squote ---
	t.Run("squote", func(t *testing.T) {
		got, err := callStdlib(t, "squote", []values.Value{strArr("a", "b")}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av := got.(values.ArrayValue)
		if av.Items[0].(values.StringValue).Value != "'a'" {
			t.Errorf("got %q, want 'a'", av.Items[0].(values.StringValue).Value)
		}
	})

	// --- unzip ---
	t.Run("unzip", func(t *testing.T) {
		p1 := pairVal(values.IntValue{Type: types.IntType{}, Value: 1}, values.StringValue{Type: types.StringType{}, Value: "a"})
		p2 := pairVal(values.IntValue{Type: types.IntType{}, Value: 2}, values.StringValue{Type: types.StringType{}, Value: "b"})
		pairArr := values.ArrayValue{
			Type:  types.ArrayType{ItemType: p1.ValueType()},
			Items: []values.Value{p1, p2},
		}
		got, err := callStdlib(t, "unzip", []values.Value{pairArr}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		pv := got.(values.PairValue)
		lefts := pv.Left.(values.ArrayValue)
		rights := pv.Right.(values.ArrayValue)
		if len(lefts.Items) != 2 || len(rights.Items) != 2 {
			t.Fatalf("expected 2 lefts and 2 rights, got %d and %d", len(lefts.Items), len(rights.Items))
		}
		if lefts.Items[0].(values.IntValue).Value != 1 {
			t.Errorf("lefts[0]=%v, want 1", lefts.Items[0])
		}
		if rights.Items[1].(values.StringValue).Value != "b" {
			t.Errorf("rights[1]=%v, want b", rights.Items[1])
		}
	})

	// --- select_all ---
	t.Run("select_all filters nulls", func(t *testing.T) {
		arr := values.ArrayValue{
			Type: types.ArrayType{ItemType: types.IntType{}},
			Items: []values.Value{
				values.IntValue{Type: types.IntType{}, Value: 1},
				values.NullValue{},
				values.IntValue{Type: types.IntType{}, Value: 2},
				values.NullValue{},
			},
		}
		got, err := callStdlib(t, "select_all", []values.Value{arr}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av := got.(values.ArrayValue)
		if len(av.Items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(av.Items))
		}
		if av.Items[0].(values.IntValue).Value != 1 || av.Items[1].(values.IntValue).Value != 2 {
			t.Errorf("got %v, want [1,2]", av.Items)
		}
	})

	// --- select_first ---
	t.Run("select_first returns first non-null", func(t *testing.T) {
		arr := values.ArrayValue{
			Type: types.ArrayType{ItemType: types.IntType{}},
			Items: []values.Value{
				values.NullValue{},
				values.NullValue{},
				values.IntValue{Type: types.IntType{}, Value: 3},
			},
		}
		got, err := callStdlib(t, "select_first", []values.Value{arr}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		iv := got.(values.IntValue)
		if iv.Value != 3 {
			t.Errorf("got %d, want 3", iv.Value)
		}
	})
	t.Run("select_first all null returns EvalError", func(t *testing.T) {
		arr := values.ArrayValue{
			Type:  types.ArrayType{ItemType: types.IntType{}},
			Items: []values.Value{values.NullValue{}, values.NullValue{}},
		}
		_, err := callStdlib(t, "select_first", []values.Value{arr}, ctx)
		if err == nil {
			t.Fatal("expected EvalError for all-null array")
		}
	})

	// --- chunk ---
	t.Run("chunk [1,2,3,4,5] by 2", func(t *testing.T) {
		got, err := callStdlib(t, "chunk", []values.Value{
			intArr(1, 2, 3, 4, 5),
			values.IntValue{Type: types.IntType{}, Value: 2},
		}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av := got.(values.ArrayValue)
		if len(av.Items) != 3 {
			t.Fatalf("expected 3 chunks, got %d", len(av.Items))
		}
		c0 := av.Items[0].(values.ArrayValue)
		c1 := av.Items[1].(values.ArrayValue)
		c2 := av.Items[2].(values.ArrayValue)
		if len(c0.Items) != 2 || len(c1.Items) != 2 || len(c2.Items) != 1 {
			t.Errorf("chunk sizes: got %d,%d,%d want 2,2,1", len(c0.Items), len(c1.Items), len(c2.Items))
		}
	})
	t.Run("chunk exact fit", func(t *testing.T) {
		got, err := callStdlib(t, "chunk", []values.Value{
			intArr(1, 2, 3),
			values.IntValue{Type: types.IntType{}, Value: 3},
		}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av := got.(values.ArrayValue)
		if len(av.Items) != 1 {
			t.Fatalf("expected 1 chunk, got %d", len(av.Items))
		}
	})
	t.Run("chunk empty array", func(t *testing.T) {
		got, err := callStdlib(t, "chunk", []values.Value{
			intArr(),
			values.IntValue{Type: types.IntType{}, Value: 2},
		}, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av := got.(values.ArrayValue)
		if len(av.Items) != 0 {
			t.Fatalf("expected 0 chunks, got %d", len(av.Items))
		}
	})
	t.Run("chunk n=0 returns EvalError", func(t *testing.T) {
		_, err := callStdlib(t, "chunk", []values.Value{
			intArr(1, 2, 3),
			values.IntValue{Type: types.IntType{}, Value: 0},
		}, ctx)
		if err == nil {
			t.Fatal("expected EvalError for n=0")
		}
	})
	t.Run("chunk n negative returns EvalError", func(t *testing.T) {
		_, err := callStdlib(t, "chunk", []values.Value{
			intArr(1, 2, 3),
			values.IntValue{Type: types.IntType{}, Value: -1},
		}, ctx)
		if err == nil {
			t.Fatal("expected EvalError for n<0")
		}
	})
}

// TestStdlibString tests string standard library functions (STDLIB-04).
// Covers: sub, sep, find, matches, quote, squote.
func TestStdlibString(t *testing.T) {
	t.Skip("not yet implemented")
}

// TestStdlibUtility tests utility standard library functions (STDLIB-05).
// Covers: defined, select_first, select_all, basename, flatten.
func TestStdlibUtility(t *testing.T) {
	t.Skip("not yet implemented")
}

// TestStdlibNumeric tests numeric standard library functions (STDLIB-06).
// Covers: floor, ceil, round, min, max.
func TestStdlibNumeric(t *testing.T) {
	t.Skip("not yet implemented")
}

// TestRoundHalfUp tests the round() function's half-up semantics (STDLIB-06).
// Spec requirement: round(-2.5) must return -2 (round half away from zero), not -3.
func TestRoundHalfUp(t *testing.T) {
	t.Skip("not yet implemented")
}
