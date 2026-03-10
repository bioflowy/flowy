package values_test

import (
	"testing"

	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// Compile-time interface satisfaction check: NullValue{} must satisfy Value.
var _ values.Value = values.NullValue{}

func TestValueType(t *testing.T) {
	tests := []struct {
		name     string
		val      values.Value
		wantType types.Type
	}{
		{
			name:     "BooleanValue",
			val:      values.BooleanValue{Type: types.BooleanType{}},
			wantType: types.BooleanType{},
		},
		{
			name:     "IntValue",
			val:      values.IntValue{Type: types.IntType{}},
			wantType: types.IntType{},
		},
		{
			name:     "FloatValue",
			val:      values.FloatValue{Type: types.FloatType{}},
			wantType: types.FloatType{},
		},
		{
			name:     "StringValue",
			val:      values.StringValue{Type: types.StringType{}},
			wantType: types.StringType{},
		},
		{
			name:     "FileValue",
			val:      values.FileValue{Type: types.FileType{}},
			wantType: types.FileType{},
		},
		{
			name:     "DirectoryValue",
			val:      values.DirectoryValue{Type: types.DirectoryType{}},
			wantType: types.DirectoryType{},
		},
		{
			name:     "ArrayValue",
			val:      values.ArrayValue{Type: types.ArrayType{ItemType: types.IntType{}}},
			wantType: types.ArrayType{ItemType: types.IntType{}},
		},
		{
			name:     "MapValue",
			val:      values.MapValue{Type: types.MapType{KeyType: types.StringType{}, ValType: types.IntType{}}},
			wantType: types.MapType{KeyType: types.StringType{}, ValType: types.IntType{}},
		},
		{
			name:     "PairValue",
			val:      values.PairValue{Type: types.PairType{LeftType: types.IntType{}, RightType: types.BooleanType{}}},
			wantType: types.PairType{LeftType: types.IntType{}, RightType: types.BooleanType{}},
		},
		{
			name: "StructValue",
			val: values.StructValue{
				Type: types.StructType{Name: "S", Members: map[string]types.Type{"x": types.IntType{}}},
			},
			wantType: nil, // checked by TypeName below
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.val.ValueType()
			if tt.name == "StructValue" {
				// Check via TypeName since map equality is tricky
				if got.TypeName() != "S" {
					t.Errorf("StructValue ValueType().TypeName() = %q, want %q", got.TypeName(), "S")
				}
				return
			}
			if got != tt.wantType {
				t.Errorf("ValueType() = %v (%T), want %v (%T)", got, got, tt.wantType, tt.wantType)
			}
		})
	}
}

func TestArrayValue_Typed(t *testing.T) {
	intArr := values.ArrayValue{Type: types.ArrayType{ItemType: types.IntType{}}}
	strArr := values.ArrayValue{Type: types.ArrayType{ItemType: types.StringType{}}}

	intName := intArr.ValueType().TypeName()
	strName := strArr.ValueType().TypeName()

	if intName == strName {
		t.Errorf("ArrayValue with Int and String should have different TypeName(), both got %q", intName)
	}
	if intName != "Array[Int]" {
		t.Errorf("int array TypeName() = %q, want %q", intName, "Array[Int]")
	}
	if strName != "Array[String]" {
		t.Errorf("str array TypeName() = %q, want %q", strName, "Array[String]")
	}
}

func TestNullValue(t *testing.T) {
	nv := values.NullValue{}
	vt := nv.ValueType()

	if !vt.IsOptional() {
		t.Error("NullValue.ValueType().IsOptional() = false, want true")
	}
	if vt.TypeName() != "None" {
		t.Errorf("NullValue.ValueType().TypeName() = %q, want %q", vt.TypeName(), "None")
	}
}
