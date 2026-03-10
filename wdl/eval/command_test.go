package eval

import (
	"testing"

	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// TestCommandDedent tests the sentinel-before-indent dedent algorithm (EXPR-07).
// Covers: computeMinIndent and applyDedentAndSubstitute pure functions.
func TestCommandDedent(t *testing.T) {
	t.Run("computeMinIndent: 4-space indent", func(t *testing.T) {
		got := computeMinIndent("    echo hello\n    echo world\n")
		if got != 4 {
			t.Errorf("got %d, want 4", got)
		}
	})

	t.Run("computeMinIndent: empty lines excluded", func(t *testing.T) {
		got := computeMinIndent("    echo hello\n\n    echo world\n")
		if got != 4 {
			t.Errorf("got %d, want 4", got)
		}
	})

	t.Run("computeMinIndent: whitespace-only lines excluded", func(t *testing.T) {
		got := computeMinIndent("    echo hello\n   \n    echo world\n")
		if got != 4 {
			t.Errorf("got %d, want 4", got)
		}
	})

	t.Run("computeMinIndent: zero indent", func(t *testing.T) {
		got := computeMinIndent("echo hello\n")
		if got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})

	t.Run("computeMinIndent: empty string returns 0", func(t *testing.T) {
		got := computeMinIndent("")
		if got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})

	t.Run("computeMinIndent: mixed indent uses minimum", func(t *testing.T) {
		got := computeMinIndent("    echo a\n  echo b\n      echo c\n")
		if got != 2 {
			t.Errorf("got %d, want 2", got)
		}
	})

	t.Run("applyDedentAndSubstitute: strips indent", func(t *testing.T) {
		sentinel := "    echo hello\n    echo world\n"
		got := applyDedentAndSubstitute(sentinel, 4, nil, "~{}")
		want := "echo hello\necho world\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("applyDedentAndSubstitute: substitutes markers in order", func(t *testing.T) {
		sentinel := "    echo ~{}\n    echo ~{}\n"
		got := applyDedentAndSubstitute(sentinel, 4, []string{"alice", "bob"}, "~{}")
		want := "echo alice\necho bob\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("applyDedentAndSubstitute: whitespace-only line preserved as-is", func(t *testing.T) {
		// Whitespace-only line is not stripped (it's excluded from min-indent computation,
		// but in dedent application it should not have leading spaces stripped beyond its content)
		sentinel := "    echo hello\n   \n    echo world\n"
		got := applyDedentAndSubstitute(sentinel, 4, nil, "~{}")
		want := "echo hello\n   \necho world\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("applyDedentAndSubstitute: zero indent no-op", func(t *testing.T) {
		sentinel := "echo hello\necho world\n"
		got := applyDedentAndSubstitute(sentinel, 0, nil, "~{}")
		want := "echo hello\necho world\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// TestCommandDedentEdgeCases tests edge cases for the command block dedent algorithm.
func TestCommandDedentEdgeCases(t *testing.T) {
	t.Run("valueToString: NullValue without default returns empty string", func(t *testing.T) {
		got := valueToString(values.NullValue{}, nil)
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("valueToString: NullValue with default returns default", func(t *testing.T) {
		opts := map[string]string{"default": "MISSING"}
		got := valueToString(values.NullValue{}, opts)
		if got != "MISSING" {
			t.Errorf("got %q, want %q", got, "MISSING")
		}
	})

	t.Run("valueToString: BooleanValue true without option", func(t *testing.T) {
		got := valueToString(values.BooleanValue{Type: types.BooleanType{}, Value: true}, nil)
		if got != "true" {
			t.Errorf("got %q, want %q", got, "true")
		}
	})

	t.Run("valueToString: BooleanValue false without option", func(t *testing.T) {
		got := valueToString(values.BooleanValue{Type: types.BooleanType{}, Value: false}, nil)
		if got != "false" {
			t.Errorf("got %q, want %q", got, "false")
		}
	})

	t.Run("valueToString: BooleanValue true with 'true' option", func(t *testing.T) {
		opts := map[string]string{"true": "yes"}
		got := valueToString(values.BooleanValue{Type: types.BooleanType{}, Value: true}, opts)
		if got != "yes" {
			t.Errorf("got %q, want %q", got, "yes")
		}
	})

	t.Run("valueToString: BooleanValue false with 'false' option", func(t *testing.T) {
		opts := map[string]string{"false": "no"}
		got := valueToString(values.BooleanValue{Type: types.BooleanType{}, Value: false}, opts)
		if got != "no" {
			t.Errorf("got %q, want %q", got, "no")
		}
	})

	t.Run("valueToString: IntValue", func(t *testing.T) {
		got := valueToString(values.IntValue{Type: types.IntType{}, Value: 42}, nil)
		if got != "42" {
			t.Errorf("got %q, want %q", got, "42")
		}
	})

	t.Run("valueToString: FloatValue", func(t *testing.T) {
		got := valueToString(values.FloatValue{Type: types.FloatType{}, Value: 3.14}, nil)
		if got != "3.14" {
			t.Errorf("got %q, want %q", got, "3.14")
		}
	})

	t.Run("valueToString: StringValue", func(t *testing.T) {
		got := valueToString(values.StringValue{Type: types.StringType{}, Value: "hello"}, nil)
		if got != "hello" {
			t.Errorf("got %q, want %q", got, "hello")
		}
	})

	t.Run("valueToString: FileValue", func(t *testing.T) {
		got := valueToString(values.FileValue{Type: types.FileType{}, Value: "/tmp/file.txt"}, nil)
		if got != "/tmp/file.txt" {
			t.Errorf("got %q, want %q", got, "/tmp/file.txt")
		}
	})

	t.Run("valueToString: ArrayValue with sep option", func(t *testing.T) {
		arr := values.ArrayValue{
			Type: types.ArrayType{ItemType: types.StringType{}},
			Items: []values.Value{
				values.StringValue{Type: types.StringType{}, Value: "a"},
				values.StringValue{Type: types.StringType{}, Value: "b"},
				values.StringValue{Type: types.StringType{}, Value: "c"},
			},
		}
		opts := map[string]string{"sep": ","}
		got := valueToString(arr, opts)
		if got != "a,b,c" {
			t.Errorf("got %q, want %q", got, "a,b,c")
		}
	})

	t.Run("valueToString: ArrayValue without sep option uses space", func(t *testing.T) {
		arr := values.ArrayValue{
			Type: types.ArrayType{ItemType: types.IntType{}},
			Items: []values.Value{
				values.IntValue{Type: types.IntType{}, Value: 1},
				values.IntValue{Type: types.IntType{}, Value: 2},
			},
		}
		got := valueToString(arr, nil)
		if got != "1 2" {
			t.Errorf("got %q, want %q", got, "1 2")
		}
	})

	t.Run("applyDedentAndSubstitute: empty parts list produces empty output", func(t *testing.T) {
		got := applyDedentAndSubstitute("", 0, nil, "~{}")
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})
}
