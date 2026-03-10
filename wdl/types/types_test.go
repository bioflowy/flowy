package types_test

import (
	"testing"

	"github.com/bioflowy/flowy/wdl/types"
)

func TestTypeName(t *testing.T) {
	tests := []struct {
		name     string
		typ      types.Type
		expected string
	}{
		{"Int", types.IntType{}, "Int"},
		{"Int?", types.IntType{Optional: true}, "Int?"},
		{"Boolean", types.BooleanType{}, "Boolean"},
		{"Float", types.FloatType{}, "Float"},
		{"String", types.StringType{}, "String"},
		{"File", types.FileType{}, "File"},
		{"Directory", types.DirectoryType{}, "Directory"},
		{"Array[String]", types.ArrayType{ItemType: types.StringType{}}, "Array[String]"},
		{"Array[String]+", types.ArrayType{ItemType: types.StringType{}, Nonempty: true}, "Array[String]+"},
		{"Array[String]+?", types.ArrayType{ItemType: types.StringType{}, Nonempty: true, Optional: true}, "Array[String]+?"},
		{"Map[String,Int]", types.MapType{KeyType: types.StringType{}, ValType: types.IntType{}}, "Map[String,Int]"},
		{"Pair[Int,Boolean]", types.PairType{LeftType: types.IntType{}, RightType: types.BooleanType{}}, "Pair[Int,Boolean]"},
		{"None", types.AnyType{Optional: true}, "None"},
		{"Any", types.AnyType{}, "Any"},
		{"StructType", types.StructType{Name: "Sample"}, "Sample"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.typ.TypeName()
			if got != tt.expected {
				t.Errorf("TypeName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestWithOptional(t *testing.T) {
	base := types.IntType{}
	opt := base.WithOptional(true)

	if !opt.IsOptional() {
		t.Error("opt.IsOptional() = false, want true")
	}
	if base.IsOptional() {
		t.Error("base.IsOptional() = true after WithOptional(true) — original was mutated!")
	}
}

func TestIsOptional(t *testing.T) {
	tests := []struct {
		name     string
		typ      types.Type
		expected bool
	}{
		{"IntType not optional", types.IntType{Optional: false}, false},
		{"FloatType optional", types.FloatType{Optional: true}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.typ.IsOptional()
			if got != tt.expected {
				t.Errorf("IsOptional() = %v, want %v", got, tt.expected)
			}
		})
	}
}
