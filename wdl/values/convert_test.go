package values_test

import (
	"testing"

	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// TestValueToInterface_Primitives checks all primitive value types.
func TestValueToInterface_Primitives(t *testing.T) {
	tests := []struct {
		name     string
		input    values.Value
		expected interface{}
	}{
		{
			name:     "StringValue",
			input:    values.StringValue{Type: types.StringType{}, Value: "hello"},
			expected: "hello",
		},
		{
			name:     "IntValue",
			input:    values.IntValue{Type: types.IntType{}, Value: 42},
			expected: int64(42),
		},
		{
			name:     "FloatValue",
			input:    values.FloatValue{Type: types.FloatType{}, Value: 3.14},
			expected: 3.14,
		},
		{
			name:     "BooleanValue true",
			input:    values.BooleanValue{Type: types.BooleanType{}, Value: true},
			expected: true,
		},
		{
			name:     "BooleanValue false",
			input:    values.BooleanValue{Type: types.BooleanType{}, Value: false},
			expected: false,
		},
		{
			name:     "FileValue",
			input:    values.FileValue{Type: types.FileType{}, Value: "/path/to/file"},
			expected: "/path/to/file",
		},
		{
			name:     "DirectoryValue",
			input:    values.DirectoryValue{Type: types.DirectoryType{}, Value: "/path/to/dir"},
			expected: "/path/to/dir",
		},
		{
			name:     "NullValue",
			input:    values.NullValue{},
			expected: nil,
		},
		{
			name:     "nil Value",
			input:    nil,
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := values.ValueToInterface(tc.input)
			if got != tc.expected {
				t.Errorf("ValueToInterface(%T) = %v (%T), want %v (%T)",
					tc.input, got, got, tc.expected, tc.expected)
			}
		})
	}
}

// TestValueToInterface_Array checks ArrayValue conversion.
func TestValueToInterface_Array(t *testing.T) {
	input := values.ArrayValue{
		Type: types.ArrayType{ItemType: types.StringType{}},
		Items: []values.Value{
			values.StringValue{Type: types.StringType{}, Value: "a"},
			values.IntValue{Type: types.IntType{}, Value: 1},
		},
	}

	got := values.ValueToInterface(input)
	arr, ok := got.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", got)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr))
	}
	if arr[0] != "a" {
		t.Errorf("arr[0] = %v, want %q", arr[0], "a")
	}
	if arr[1] != int64(1) {
		t.Errorf("arr[1] = %v, want %d", arr[1], 1)
	}
}

// TestValueToInterface_Struct checks StructValue conversion.
func TestValueToInterface_Struct(t *testing.T) {
	input := values.StructValue{
		Fields: map[string]values.Value{
			"x": values.IntValue{Type: types.IntType{}, Value: 1},
		},
	}

	got := values.ValueToInterface(input)
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", got)
	}
	if m["x"] != int64(1) {
		t.Errorf("m[x] = %v, want %d", m["x"], 1)
	}
}

// TestValueToInterface_Pair checks PairValue conversion.
func TestValueToInterface_Pair(t *testing.T) {
	input := values.PairValue{
		Left:  values.IntValue{Type: types.IntType{}, Value: 1},
		Right: values.StringValue{Type: types.StringType{}, Value: "a"},
	}

	got := values.ValueToInterface(input)
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", got)
	}
	if m["left"] != int64(1) {
		t.Errorf("m[left] = %v, want %d", m["left"], 1)
	}
	if m["right"] != "a" {
		t.Errorf("m[right] = %v, want %q", m["right"], "a")
	}
}

// TestValueToInterface_Map checks MapValue conversion (keys converted via Sprintf).
func TestValueToInterface_Map(t *testing.T) {
	input := values.MapValue{
		Type: types.MapType{KeyType: types.StringType{}, ValType: types.IntType{}},
		Entries: []values.MapEntry{
			{Key: values.StringValue{Type: types.StringType{}, Value: "count"}, Value: values.IntValue{Type: types.IntType{}, Value: 5}},
		},
	}

	got := values.ValueToInterface(input)
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", got)
	}
	if m["count"] != int64(5) {
		t.Errorf("m[count] = %v, want %d", m["count"], 5)
	}
}

// TestValuesToJSON checks conversion of a map of WDL values to JSON-compatible map.
func TestValuesToJSON(t *testing.T) {
	input := map[string]values.Value{
		"k": values.StringValue{Type: types.StringType{}, Value: "v"},
		"n": values.IntValue{Type: types.IntType{}, Value: 42},
	}

	got, err := values.ValuesToJSON(input)
	if err != nil {
		t.Fatalf("ValuesToJSON returned error: %v", err)
	}
	if got["k"] != "v" {
		t.Errorf("got[k] = %v, want %q", got["k"], "v")
	}
	if got["n"] != int64(42) {
		t.Errorf("got[n] = %v, want %d", got["n"], 42)
	}
}
