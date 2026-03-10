// Package eval — command.go implements command template expansion and string interpolation.
// This includes the sentinel-before-indent dedent algorithm (EXPR-07) and placeholder
// substitution (EXPR-06) for both CommandExpr and StringExpr evaluation.
package eval

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// computeMinIndent computes the minimum leading whitespace (number of leading space/tab chars)
// across all non-empty, non-whitespace-only lines in s.
// Empty lines and lines containing only whitespace are excluded from the computation.
// Returns 0 if s is empty or all lines are whitespace-only.
func computeMinIndent(s string) int {
	lines := strings.Split(s, "\n")
	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			// Skip empty and whitespace-only lines
			continue
		}
		indent := 0
		for _, ch := range line {
			if ch == ' ' || ch == '\t' {
				indent++
			} else {
				break
			}
		}
		if minIndent < 0 || indent < minIndent {
			minIndent = indent
		}
	}
	if minIndent < 0 {
		return 0
	}
	return minIndent
}

// applyDedentAndSubstitute strips minIndent leading characters from every non-empty line
// in sentinel, then replaces each occurrence of marker ("~{}") with the next element
// from parts in order (left-to-right).
// Whitespace-only lines are preserved as-is without stripping.
func applyDedentAndSubstitute(sentinel string, minIndent int, parts []string, marker string) string {
	if sentinel == "" {
		return ""
	}

	// Strip indentation from each line
	lines := strings.Split(sentinel, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			// Preserve whitespace-only lines unchanged (no stripping)
			continue
		}
		if minIndent > 0 && len(line) >= minIndent {
			lines[i] = line[minIndent:]
		} else if minIndent > 0 {
			lines[i] = ""
		}
	}
	dedented := strings.Join(lines, "\n")

	// Substitute markers in order
	if len(parts) == 0 || !strings.Contains(dedented, marker) {
		return dedented
	}

	var sb strings.Builder
	idx := 0
	remaining := dedented
	for {
		pos := strings.Index(remaining, marker)
		if pos < 0 || idx >= len(parts) {
			sb.WriteString(remaining)
			break
		}
		sb.WriteString(remaining[:pos])
		sb.WriteString(parts[idx])
		idx++
		remaining = remaining[pos+len(marker):]
	}
	return sb.String()
}

// valueToString serializes a WDL value to its command-line string representation,
// applying placeholder option semantics (sep, default, true, false).
func valueToString(v values.Value, opts map[string]string) string {
	switch val := v.(type) {
	case values.NullValue:
		if opts != nil {
			if def, ok := opts["default"]; ok {
				return def
			}
		}
		return ""
	case values.BooleanValue:
		if val.Value {
			if opts != nil {
				if s, ok := opts["true"]; ok {
					return s
				}
			}
			return "true"
		}
		if opts != nil {
			if s, ok := opts["false"]; ok {
				return s
			}
		}
		return "false"
	case values.IntValue:
		return strconv.FormatInt(val.Value, 10)
	case values.FloatValue:
		return strconv.FormatFloat(val.Value, 'g', -1, 64)
	case values.StringValue:
		return val.Value
	case values.FileValue:
		return val.Value
	case values.DirectoryValue:
		return val.Value
	case values.ArrayValue:
		sep := " "
		if opts != nil {
			if s, ok := opts["sep"]; ok {
				sep = s
			}
		}
		parts := make([]string, len(val.Items))
		for i, item := range val.Items {
			parts[i] = valueToString(item, nil)
		}
		return strings.Join(parts, sep)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// evalString evaluates a StringExpr by substituting placeholder values directly
// (no dedent — strings use direct interpolation without indentation stripping).
func evalString(parts []ast.StringPart, env *Env, ctx ExecContext) (string, error) {
	var sb strings.Builder
	for _, part := range parts {
		switch p := part.(type) {
		case *ast.RawStringPart:
			sb.WriteString(p.Val)
		case *ast.PlaceholderPart:
			v, err := Eval(p.Expr, env, ctx)
			if err != nil {
				return "", err
			}
			sb.WriteString(valueToString(v, p.Options))
		}
	}
	return sb.String(), nil
}

// evalCommand evaluates a CommandExpr using the sentinel-before-indent algorithm:
//  1. Sentinel phase: iterate Parts; for RawStringPart write .Val to sentinelBuf;
//     for PlaceholderPart: Eval .Expr, collect result in evaledParts, write "~{}" sentinel
//  2. Compute min-indent on the sentinel string (not the expanded values)
//  3. Apply dedent and substitute: strip indentation then replace sentinels with stringified values
func evalCommand(e *ast.CommandExpr, env *Env, ctx ExecContext) (values.Value, error) {
	// Phase 1: build sentinel string and evaluate placeholder expressions
	var sentinelBuf strings.Builder
	var evaledParts []string

	for _, part := range e.Parts {
		switch p := part.(type) {
		case *ast.RawStringPart:
			sentinelBuf.WriteString(p.Val)
		case *ast.PlaceholderPart:
			v, err := Eval(p.Expr, env, ctx)
			if err != nil {
				return nil, err
			}
			// Write sentinel marker — the actual value will replace this after dedenting
			sentinelBuf.WriteString("~{}")
			// Options are applied before dedent: stringify with options now, substitute after
			evaledParts = append(evaledParts, valueToString(v, p.Options))
		}
	}

	// Phase 2: compute min-indent on sentinel string (sentinel values, not expanded text)
	minIndent := computeMinIndent(sentinelBuf.String())

	// Phase 3: apply dedent and substitute
	result := applyDedentAndSubstitute(sentinelBuf.String(), minIndent, evaledParts, "~{}")

	return values.StringValue{Type: types.StringType{}, Value: result}, nil
}
