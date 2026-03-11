package eval

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// init registers all STDLIB-04 (string) functions into the stdlibFuncs dispatch map.
func init() {
	stdlibFuncs["sub"]        = stdSub
	stdlibFuncs["find"]       = stdFind
	stdlibFuncs["matches"]    = stdMatches
	stdlibFuncs["join_paths"] = stdJoinPaths
}

// wdlReplacementToGo converts a WDL replacement string to Go's regexp replacement syntax.
// WDL uses \N (backslash + digit) for backreferences; Go uses ${N}.
// A literal backslash in WDL replacement is written as \\ (already processed by unescapeString
// into a single \). We convert remaining \N sequences to ${N}.
func wdlReplacementToGo(repl string) string {
	var sb strings.Builder
	sb.Grow(len(repl))
	i := 0
	for i < len(repl) {
		if repl[i] == '\\' && i+1 < len(repl) && repl[i+1] >= '0' && repl[i+1] <= '9' {
			// \N → ${N}
			sb.WriteString("${")
			sb.WriteByte(repl[i+1])
			sb.WriteByte('}')
			i += 2
		} else if repl[i] == '$' {
			// Escape literal $ to avoid Go interpreting it as a group reference
			sb.WriteString("$$")
			i++
		} else {
			sb.WriteByte(repl[i])
			i++
		}
	}
	return sb.String()
}

// stdSub implements sub(String input, String pattern, String replacement) -> String.
// Replaces all POSIX ERE matches of pattern in input with replacement.
// WDL backreferences in replacement use \N syntax (e.g. \1, \2).
func stdSub(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 3 {
		return nil, &EvalError{Message: "sub: expected (String, String, String) arguments"}
	}
	input, ok1 := args[0].(values.StringValue)
	pattern, ok2 := args[1].(values.StringValue)
	replacement, ok3 := args[2].(values.StringValue)
	if !ok1 || !ok2 || !ok3 {
		return nil, &EvalError{Message: "sub: all arguments must be StringValue"}
	}
	re, err := regexp.Compile(pattern.Value)
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("sub: invalid pattern %q: %v", pattern.Value, err)}
	}
	// Convert WDL \N backreferences to Go ${N} syntax.
	goRepl := wdlReplacementToGo(replacement.Value)
	result := re.ReplaceAllString(input.Value, goRepl)
	return values.StringValue{Type: types.StringType{}, Value: result}, nil
}

// stdFind implements find(String input, String pattern) -> String?.
// Returns the first match of pattern in input, or NullValue if no match.
func stdFind(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 2 {
		return nil, &EvalError{Message: "find: expected (String, String) arguments"}
	}
	input, ok1 := args[0].(values.StringValue)
	pattern, ok2 := args[1].(values.StringValue)
	if !ok1 || !ok2 {
		return nil, &EvalError{Message: "find: both arguments must be StringValue"}
	}
	re, err := regexp.Compile(pattern.Value)
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("find: invalid pattern %q: %v", pattern.Value, err)}
	}
	match := re.FindString(input.Value)
	if match == "" {
		return values.NullValue{}, nil
	}
	return values.StringValue{Type: types.StringType{}, Value: match}, nil
}

// stdMatches implements matches(String input, String pattern) -> Boolean.
// Returns true if the entire input string matches the pattern (whole-string match).
func stdMatches(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 2 {
		return nil, &EvalError{Message: "matches: expected (String, String) arguments"}
	}
	input, ok1 := args[0].(values.StringValue)
	pattern, ok2 := args[1].(values.StringValue)
	if !ok1 || !ok2 {
		return nil, &EvalError{Message: "matches: both arguments must be StringValue"}
	}
	// Anchor pattern to require whole-string match.
	anchored := "^" + pattern.Value + "$"
	re, err := regexp.Compile(anchored)
	if err != nil {
		return nil, &EvalError{Message: fmt.Sprintf("matches: invalid pattern %q: %v", pattern.Value, err)}
	}
	return values.BooleanValue{Type: types.BooleanType{}, Value: re.MatchString(input.Value)}, nil
}

// stdJoinPaths implements join_paths(...) -> File.
// Three overloads dispatched at runtime by argument types:
//
//	(FileValue, StringValue)  -> filepath.Join(arg0, arg1)
//	(FileValue, ArrayValue)   -> filepath.Join(arg0, items...)
//	(ArrayValue)              -> filepath.Join(items...)
func stdJoinPaths(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 1 {
		return nil, &EvalError{Message: "join_paths: expected at least 1 argument"}
	}
	var parts []string

	switch a0 := args[0].(type) {
	case values.FileValue:
		parts = append(parts, a0.Value)
		if len(args) < 2 {
			return nil, &EvalError{Message: "join_paths: (File, ...) overload requires a second argument"}
		}
		switch a1 := args[1].(type) {
		case values.StringValue:
			parts = append(parts, a1.Value)
		case values.ArrayValue:
			for _, item := range a1.Items {
				sv, ok := item.(values.StringValue)
				if !ok {
					return nil, &EvalError{Message: fmt.Sprintf("join_paths: Array items must be StringValue, got %T", item)}
				}
				parts = append(parts, sv.Value)
			}
		default:
			return nil, &EvalError{Message: fmt.Sprintf("join_paths: second argument must be String or Array[String], got %T", args[1])}
		}
	case values.StringValue:
		parts = append(parts, a0.Value)
		if len(args) < 2 {
			return nil, &EvalError{Message: "join_paths: (String, ...) overload requires a second argument"}
		}
		switch a1 := args[1].(type) {
		case values.StringValue:
			parts = append(parts, a1.Value)
		case values.ArrayValue:
			for _, item := range a1.Items {
				sv, ok := item.(values.StringValue)
				if !ok {
					return nil, &EvalError{Message: fmt.Sprintf("join_paths: Array items must be StringValue, got %T", item)}
				}
				parts = append(parts, sv.Value)
			}
		default:
			return nil, &EvalError{Message: fmt.Sprintf("join_paths: second argument must be String or Array[String], got %T", args[1])}
		}
	case values.DirectoryValue:
		parts = append(parts, a0.Value)
		if len(args) < 2 {
			return nil, &EvalError{Message: "join_paths: (Directory, ...) overload requires a second argument"}
		}
		switch a1 := args[1].(type) {
		case values.StringValue:
			parts = append(parts, a1.Value)
		case values.ArrayValue:
			for _, item := range a1.Items {
				sv, ok := item.(values.StringValue)
				if !ok {
					return nil, &EvalError{Message: fmt.Sprintf("join_paths: Array items must be StringValue, got %T", item)}
				}
				parts = append(parts, sv.Value)
			}
		default:
			return nil, &EvalError{Message: fmt.Sprintf("join_paths: second argument must be String or Array[String], got %T", args[1])}
		}
	case values.ArrayValue:
		for _, item := range a0.Items {
			sv, ok := item.(values.StringValue)
			if !ok {
				return nil, &EvalError{Message: fmt.Sprintf("join_paths: Array items must be StringValue, got %T", item)}
			}
			parts = append(parts, sv.Value)
		}
	default:
		return nil, &EvalError{Message: fmt.Sprintf("join_paths: first argument must be File or Array[String], got %T", args[0])}
	}

	return values.FileValue{Type: types.FileType{}, Value: filepath.Join(parts...)}, nil
}
