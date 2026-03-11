package eval

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// init registers all STDLIB-01 (file I/O) and STDLIB-02 (filesystem) functions
// into the stdlibFuncs dispatch map defined in stdlib.go.
func init() {
	// STDLIB-01: read functions
	stdlibFuncs["read_lines"]   = stdReadLines
	stdlibFuncs["read_tsv"]     = stdReadTsv
	stdlibFuncs["read_map"]     = stdReadMap
	stdlibFuncs["read_json"]    = stdReadJson
	stdlibFuncs["read_boolean"] = stdReadBoolean
	stdlibFuncs["read_int"]     = stdReadInt
	stdlibFuncs["read_float"]   = stdReadFloat
	stdlibFuncs["read_string"]  = stdReadString
	// STDLIB-01: write functions
	stdlibFuncs["write_lines"] = stdWriteLines
	stdlibFuncs["write_tsv"]   = stdWriteTsv
	stdlibFuncs["write_map"]   = stdWriteMap
	stdlibFuncs["write_json"]  = stdWriteJson
	// STDLIB-02: filesystem functions
	stdlibFuncs["glob"]     = stdGlob
	stdlibFuncs["basename"] = stdBasename
	stdlibFuncs["dirname"]  = stdDirname
	stdlibFuncs["size"]     = stdSize
	stdlibFuncs["stdout"]   = stdStdout
	stdlibFuncs["stderr"]   = stdStderr
}

// fileArg extracts the host filesystem path from args[0], which must be FileValue or StringValue.
func fileArg(args []values.Value) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("expected at least 1 argument")
	}
	switch v := args[0].(type) {
	case values.FileValue:
		return v.Value, nil
	case values.StringValue:
		return v.Value, nil
	default:
		return "", fmt.Errorf("expected File argument, got %T", args[0])
	}
}

// stdReadLines implements read_lines(File) -> Array[String].
// Reads the file, trims a trailing newline, then splits on "\n".
// Each resulting line becomes a StringValue.
func stdReadLines(args []values.Value, ctx ExecContext) (values.Value, error) {
	path, err := fileArg(args)
	if err != nil {
		return nil, &EvalError{Message: "read_lines: " + err.Error()}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("read_lines: cannot read file %q: %v", path, err)}
	}
	content := strings.TrimRight(string(data), "\n")
	var lines []string
	if content == "" {
		lines = []string{}
	} else {
		lines = strings.Split(content, "\n")
	}
	items := make([]values.Value, len(lines))
	for i, l := range lines {
		items[i] = values.StringValue{Type: types.StringType{}, Value: l}
	}
	return values.ArrayValue{
		Type:  types.ArrayType{ItemType: types.StringType{}},
		Items: items,
	}, nil
}

// stdReadTsv implements read_tsv(File) -> Array[Array[String]].
// Each non-empty line is split by "\t" into fields; each row is an ArrayValue of StringValues.
func stdReadTsv(args []values.Value, ctx ExecContext) (values.Value, error) {
	path, err := fileArg(args)
	if err != nil {
		return nil, &EvalError{Message: "read_tsv: " + err.Error()}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("read_tsv: cannot read file %q: %v", path, err)}
	}
	rawLines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	var rows []values.Value
	for _, line := range rawLines {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		items := make([]values.Value, len(fields))
		for i, f := range fields {
			items[i] = values.StringValue{Type: types.StringType{}, Value: f}
		}
		rows = append(rows, values.ArrayValue{
			Type:  types.ArrayType{ItemType: types.StringType{}},
			Items: items,
		})
	}
	if rows == nil {
		rows = []values.Value{}
	}
	return values.ArrayValue{
		Type:  types.ArrayType{ItemType: types.ArrayType{ItemType: types.StringType{}}},
		Items: rows,
	}, nil
}

// stdReadMap implements read_map(File) -> Map[String, String].
// Each line contains "key\tvalue"; builds MapValue with StringValue keys and values.
func stdReadMap(args []values.Value, ctx ExecContext) (values.Value, error) {
	path, err := fileArg(args)
	if err != nil {
		return nil, &EvalError{Message: "read_map: " + err.Error()}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("read_map: cannot read file %q: %v", path, err)}
	}
	rawLines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	var entries []values.MapEntry
	for _, line := range rawLines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			return nil, &EvalError{Message: fmt.Sprintf("read_map: malformed line (expected key<TAB>value): %q", line)}
		}
		entries = append(entries, values.MapEntry{
			Key:   values.StringValue{Type: types.StringType{}, Value: parts[0]},
			Value: values.StringValue{Type: types.StringType{}, Value: parts[1]},
		})
	}
	if entries == nil {
		entries = []values.MapEntry{}
	}
	return values.MapValue{
		Type:    types.MapType{KeyType: types.StringType{}, ValType: types.StringType{}},
		Entries: entries,
	}, nil
}

// stdReadJson implements read_json(File) -> Value.
// Unmarshals the JSON file into the appropriate WDL value type.
func stdReadJson(args []values.Value, ctx ExecContext) (values.Value, error) {
	path, err := fileArg(args)
	if err != nil {
		return nil, &EvalError{Message: "read_json: " + err.Error()}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("read_json: cannot read file %q: %v", path, err)}
	}
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("read_json: invalid JSON in %q: %v", path, err)}
	}
	return jsonToValue(raw), nil
}

// jsonToValue recursively converts a json.Unmarshal result to a WDL values.Value.
func jsonToValue(v interface{}) values.Value {
	switch x := v.(type) {
	case bool:
		return values.BooleanValue{Type: types.BooleanType{}, Value: x}
	case float64:
		// JSON numbers are float64; treat as IntValue when it is a whole number.
		if math.Trunc(x) == x && !math.IsInf(x, 0) {
			return values.IntValue{Type: types.IntType{}, Value: int64(x)}
		}
		return values.FloatValue{Type: types.FloatType{}, Value: x}
	case string:
		return values.StringValue{Type: types.StringType{}, Value: x}
	case []interface{}:
		items := make([]values.Value, len(x))
		for i, elem := range x {
			items[i] = jsonToValue(elem)
		}
		var itemType types.Type = types.AnyType{}
		if len(items) > 0 {
			itemType = items[0].ValueType()
		}
		return values.ArrayValue{
			Type:  types.ArrayType{ItemType: itemType},
			Items: items,
		}
	case map[string]interface{}:
		entries := make([]values.MapEntry, 0, len(x))
		for k, val := range x {
			entries = append(entries, values.MapEntry{
				Key:   values.StringValue{Type: types.StringType{}, Value: k},
				Value: jsonToValue(val),
			})
		}
		return values.MapValue{
			Type:    types.MapType{KeyType: types.StringType{}, ValType: types.AnyType{}},
			Entries: entries,
		}
	default:
		// nil or unknown types become NullValue
		return values.NullValue{}
	}
}

// stdReadBoolean implements read_boolean(File) -> Boolean.
func stdReadBoolean(args []values.Value, ctx ExecContext) (values.Value, error) {
	path, err := fileArg(args)
	if err != nil {
		return nil, &EvalError{Message: "read_boolean: " + err.Error()}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("read_boolean: cannot read file %q: %v", path, err)}
	}
	b, err := strconv.ParseBool(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("read_boolean: invalid boolean in %q: %v", path, err)}
	}
	return values.BooleanValue{Type: types.BooleanType{}, Value: b}, nil
}

// stdReadInt implements read_int(File) -> Int.
func stdReadInt(args []values.Value, ctx ExecContext) (values.Value, error) {
	path, err := fileArg(args)
	if err != nil {
		return nil, &EvalError{Message: "read_int: " + err.Error()}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("read_int: cannot read file %q: %v", path, err)}
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("read_int: invalid integer in %q: %v", path, err)}
	}
	return values.IntValue{Type: types.IntType{}, Value: n}, nil
}

// stdReadFloat implements read_float(File) -> Float.
func stdReadFloat(args []values.Value, ctx ExecContext) (values.Value, error) {
	path, err := fileArg(args)
	if err != nil {
		return nil, &EvalError{Message: "read_float: " + err.Error()}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("read_float: cannot read file %q: %v", path, err)}
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("read_float: invalid float in %q: %v", path, err)}
	}
	return values.FloatValue{Type: types.FloatType{}, Value: f}, nil
}

// stdReadString implements read_string(File) -> String.
// Returns the file content with surrounding whitespace (including trailing newline) stripped.
func stdReadString(args []values.Value, ctx ExecContext) (values.Value, error) {
	path, err := fileArg(args)
	if err != nil {
		return nil, &EvalError{Message: "read_string: " + err.Error()}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("read_string: cannot read file %q: %v", path, err)}
	}
	return values.StringValue{Type: types.StringType{}, Value: strings.TrimSpace(string(data))}, nil
}

// stdWriteLines implements write_lines(Array[String]) -> File.
// Writes each string on its own line using os.MkdirTemp + os.WriteFile.
func stdWriteLines(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 1 {
		return nil, &EvalError{Message: "write_lines: expected Array[String] argument"}
	}
	av, ok := args[0].(values.ArrayValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("write_lines: expected ArrayValue, got %T", args[0])}
	}
	var sb strings.Builder
	for _, item := range av.Items {
		sv, ok := item.(values.StringValue)
		if !ok {
			return nil, &EvalError{Message: fmt.Sprintf("write_lines: expected StringValue items, got %T", item)}
		}
		sb.WriteString(sv.Value)
		sb.WriteByte('\n')
	}
	dir, err := os.MkdirTemp("", "wdl-write-*")
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("write_lines: cannot create temp dir: %v", err)}
	}
	path := filepath.Join(dir, "lines.txt")
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("write_lines: cannot write file: %v", err)}
	}
	return values.FileValue{Type: types.FileType{}, Value: path}, nil
}

// stdWriteTsv implements write_tsv(Array[Array[String]]) -> File.
// Writes each row as tab-separated fields, one row per line.
func stdWriteTsv(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 1 {
		return nil, &EvalError{Message: "write_tsv: expected Array[Array[String]] argument"}
	}
	av, ok := args[0].(values.ArrayValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("write_tsv: expected ArrayValue, got %T", args[0])}
	}
	var sb strings.Builder
	for _, rowVal := range av.Items {
		row, ok := rowVal.(values.ArrayValue)
		if !ok {
			return nil, &EvalError{Message: fmt.Sprintf("write_tsv: expected ArrayValue rows, got %T", rowVal)}
		}
		fields := make([]string, len(row.Items))
		for i, item := range row.Items {
			sv, ok := item.(values.StringValue)
			if !ok {
				return nil, &EvalError{Message: fmt.Sprintf("write_tsv: expected StringValue fields, got %T", item)}
			}
			fields[i] = sv.Value
		}
		sb.WriteString(strings.Join(fields, "\t"))
		sb.WriteByte('\n')
	}
	dir, err := os.MkdirTemp("", "wdl-write-*")
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("write_tsv: cannot create temp dir: %v", err)}
	}
	path := filepath.Join(dir, "tsv.tsv")
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("write_tsv: cannot write file: %v", err)}
	}
	return values.FileValue{Type: types.FileType{}, Value: path}, nil
}

// stdWriteMap implements write_map(Map[String, String]) -> File.
// Writes each entry as "key\tvalue\n".
func stdWriteMap(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 1 {
		return nil, &EvalError{Message: "write_map: expected Map argument"}
	}
	mv, ok := args[0].(values.MapValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("write_map: expected MapValue, got %T", args[0])}
	}
	var sb strings.Builder
	for _, entry := range mv.Entries {
		ks, ok1 := entry.Key.(values.StringValue)
		vs, ok2 := entry.Value.(values.StringValue)
		if !ok1 || !ok2 {
			return nil, &EvalError{Message: "write_map: Map keys and values must be String"}
		}
		sb.WriteString(ks.Value)
		sb.WriteByte('\t')
		sb.WriteString(vs.Value)
		sb.WriteByte('\n')
	}
	dir, err := os.MkdirTemp("", "wdl-write-*")
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("write_map: cannot create temp dir: %v", err)}
	}
	path := filepath.Join(dir, "map.tsv")
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("write_map: cannot write file: %v", err)}
	}
	return values.FileValue{Type: types.FileType{}, Value: path}, nil
}

// stdWriteJson implements write_json(Value) -> File.
// Serializes the WDL value to JSON using valueToJson helper.
func stdWriteJson(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 1 {
		return nil, &EvalError{Message: "write_json: expected argument"}
	}
	if err := validateJsonSerializable(args[0]); err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("write_json: %v", err)}
	}
	raw := valueToJson(args[0])
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("write_json: cannot marshal value: %v", err)}
	}
	dir, err := os.MkdirTemp("", "wdl-write-*")
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("write_json: cannot create temp dir: %v", err)}
	}
	path := filepath.Join(dir, "data.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("write_json: cannot write file: %v", err)}
	}
	return values.FileValue{Type: types.FileType{}, Value: path}, nil
}

// validateJsonSerializable checks that a value can be serialized to JSON.
// Returns an error if any Map has non-String keys.
func validateJsonSerializable(v values.Value) error {
	switch x := v.(type) {
	case values.ArrayValue:
		for _, item := range x.Items {
			if err := validateJsonSerializable(item); err != nil {
				return err
			}
		}
	case values.MapValue:
		for _, entry := range x.Entries {
			if _, ok := entry.Key.(values.StringValue); !ok {
				return fmt.Errorf("Map keys must be String for JSON serialization, got %T", entry.Key)
			}
			if err := validateJsonSerializable(entry.Value); err != nil {
				return err
			}
		}
	case values.PairValue:
		if err := validateJsonSerializable(x.Left); err != nil {
			return err
		}
		if err := validateJsonSerializable(x.Right); err != nil {
			return err
		}
	case values.StructValue:
		for _, val := range x.Fields {
			if err := validateJsonSerializable(val); err != nil {
				return err
			}
		}
	}
	return nil
}

// valueToJson converts a WDL values.Value to a json-serializable interface{}.
// This is the inverse of jsonToValue.
func valueToJson(v values.Value) interface{} {
	switch x := v.(type) {
	case values.BooleanValue:
		return x.Value
	case values.IntValue:
		return x.Value
	case values.FloatValue:
		return x.Value
	case values.StringValue:
		return x.Value
	case values.FileValue:
		return x.Value
	case values.DirectoryValue:
		return x.Value
	case values.ArrayValue:
		result := make([]interface{}, len(x.Items))
		for i, item := range x.Items {
			result[i] = valueToJson(item)
		}
		return result
	case values.MapValue:
		result := make(map[string]interface{}, len(x.Entries))
		for _, entry := range x.Entries {
			key := fmt.Sprintf("%v", valueToJson(entry.Key))
			result[key] = valueToJson(entry.Value)
		}
		return result
	case values.NullValue:
		return nil
	default:
		return fmt.Sprintf("%v", v)
	}
}

// stdGlob implements glob(String) -> Array[File].
// Returns FileValue for each path matched by filepath.Glob within ctx.WorkDir.
// Returns EvalError when ctx.WorkDir is empty.
func stdGlob(args []values.Value, ctx ExecContext) (values.Value, error) {
	if ctx.WorkDir == "" {
		return nil, &EvalError{Message: "glob() requires a task execution context (WorkDir is empty)"}
	}
	if len(args) < 1 {
		return nil, &EvalError{Message: "glob: expected String pattern argument"}
	}
	pattern, ok := args[0].(values.StringValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("glob: expected StringValue, got %T", args[0])}
	}
	matches, err := filepath.Glob(filepath.Join(ctx.WorkDir, pattern.Value))
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("glob: invalid pattern %q: %v", pattern.Value, err)}
	}
	var items []values.Value
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil || info.IsDir() {
			continue
		}
		items = append(items, values.FileValue{Type: types.FileType{}, Value: m})
	}
	return values.ArrayValue{
		Type:  types.ArrayType{ItemType: types.FileType{}},
		Items: items,
	}, nil
}

// stdBasename implements basename(File, String?) -> String.
// Returns the base name (final path element) of the file path.
// If a second argument (suffix string) is provided, it is stripped from the result.
// Accepts FileValue or StringValue for the first argument.
func stdBasename(args []values.Value, ctx ExecContext) (values.Value, error) {
	path, err := fileArg(args)
	if err != nil {
		return nil, &EvalError{Message: "basename: " + err.Error()}
	}
	base := filepath.Base(path)
	// Optional second argument: suffix to strip.
	if len(args) >= 2 {
		var suffix string
		switch s := args[1].(type) {
		case values.StringValue:
			suffix = s.Value
		default:
			return nil, &EvalError{Message: fmt.Sprintf("basename: second argument must be String, got %T", args[1])}
		}
		base = strings.TrimSuffix(base, suffix)
	}
	return values.StringValue{Type: types.StringType{}, Value: base}, nil
}

// stdDirname implements dirname(File) -> String.
// Returns the directory component of the file path.
// Accepts FileValue or StringValue input.
func stdDirname(args []values.Value, ctx ExecContext) (values.Value, error) {
	path, err := fileArg(args)
	if err != nil {
		return nil, &EvalError{Message: "dirname: " + err.Error()}
	}
	return values.StringValue{Type: types.StringType{}, Value: filepath.Dir(path)}, nil
}

// sizeUnits maps WDL unit strings to their divisors relative to bytes.
var sizeUnits = map[string]float64{
	"bytes": 1,
	"kB":    1_000,
	"MB":    1_000_000,
	"GB":    1_000_000_000,
	"TB":    1_000_000_000_000,
	"KiB":   1024,
	"MiB":   1024 * 1024,
	"GiB":   1024 * 1024 * 1024,
	"TiB":   1024 * 1024 * 1024 * 1024,
}

// stdSize implements size(File, String?) -> Float.
// Returns the file size in the specified unit (default: bytes).
func stdSize(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 1 {
		return nil, &EvalError{Message: "size: expected File argument"}
	}
	fv, ok := args[0].(values.FileValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("size: expected FileValue, got %T", args[0])}
	}
	info, err := os.Stat(fv.Value)
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("size: cannot stat %q: %v", fv.Value, err)}
	}
	byteCount := float64(info.Size())

	factor := 1.0
	if len(args) >= 2 {
		unitVal, ok := args[1].(values.StringValue)
		if !ok {
			return nil, &EvalError{Message: fmt.Sprintf("size: unit must be String, got %T", args[1])}
		}
		f, known := sizeUnits[unitVal.Value]
		if !known {
			return nil, &EvalError{Message: fmt.Sprintf("size: unknown unit %q", unitVal.Value)}
		}
		factor = f
	}

	return values.FloatValue{Type: types.FloatType{}, Value: byteCount / factor}, nil
}

// stdStdout implements stdout() -> File.
// Returns FileValue with the path from ExecContext.StdoutPath.
// Returns EvalError when StdoutPath is empty.
func stdStdout(args []values.Value, ctx ExecContext) (values.Value, error) {
	if ctx.StdoutPath == "" {
		return nil, &EvalError{Message: "stdout() requires a task execution context (StdoutPath is empty)"}
	}
	return values.FileValue{Type: types.FileType{}, Value: ctx.StdoutPath}, nil
}

// stdStderr implements stderr() -> File.
// Returns FileValue with the path from ExecContext.StderrPath.
// Returns EvalError when StderrPath is empty.
func stdStderr(args []values.Value, ctx ExecContext) (values.Value, error) {
	if ctx.StderrPath == "" {
		return nil, &EvalError{Message: "stderr() requires a task execution context (StderrPath is empty)"}
	}
	return values.FileValue{Type: types.FileType{}, Value: ctx.StderrPath}, nil
}
