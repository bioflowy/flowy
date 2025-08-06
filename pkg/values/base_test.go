package values

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/types"
)

func TestBaseInterface(t *testing.T) {
	// Test that all value types implement Base interface
	var _ Base = &BooleanValue{}
	var _ Base = &IntValue{}
	var _ Base = &FloatValue{}
	var _ Base = &StringValue{}
	var _ Base = &FileValue{}
	var _ Base = &DirectoryValue{}
	var _ Base = &ArrayValue{}
	var _ Base = &MapValue{}
	var _ Base = &PairValue{}
	var _ Base = &Null{}
	var _ Base = &StructValue{}
}

func TestNullValue(t *testing.T) {
	null := NewNull(types.NewInt(true))

	// Test basic properties
	if null.Value() != nil {
		t.Error("Null value should have nil value")
	}

	if null.String() != "null" {
		t.Errorf("Expected 'null', got '%s'", null.String())
	}

	jsonData := null.JSON()
	if string(jsonData) != "null" {
		t.Errorf("Expected JSON 'null', got '%s'", string(jsonData))
	}
}

func TestNullCoercion(t *testing.T) {
	null := NewNull(types.NewInt(true))

	// Null coerces to optional types
	optionalInt := types.NewInt(true)
	coerced, err := null.Coerce(optionalInt)
	if err != nil {
		t.Fatalf("Null should coerce to optional type: %v", err)
	}
	if !coerced.Equal(null) {
		t.Error("Null coercion to optional should return equivalent value")
	}

	// Null should not coerce to non-optional types
	requiredInt := types.NewInt(false)
	_, err = null.Coerce(requiredInt)
	if err == nil {
		t.Error("Null should not coerce to required type")
	}

	// Null should not coerce to non-optional String
	requiredString := types.NewString(false)
	_, err = null.Coerce(requiredString)
	if err == nil {
		t.Error("Null should not coerce to required String")
	}
}

func TestValueEquality(t *testing.T) {
	// Test equality between values of same type
	bool1 := NewBoolean(true, false)
	bool2 := NewBoolean(true, false)
	bool3 := NewBoolean(false, false)

	if !bool1.Equal(bool2) {
		t.Error("Equal boolean values should be equal")
	}

	if bool1.Equal(bool3) {
		t.Error("Different boolean values should not be equal")
	}

	// Test that different types cannot be compared for equality
	// This would normally be caught by static type checking in WDL
	intVal := NewInt(42, false)
	if bool1.Equal(intVal) {
		t.Error("Different types should not be equal")
	}
}

func TestValueCoercionToString(t *testing.T) {
	// Test that all values can coerce to String
	boolVal := NewBoolean(true, false)
	stringType := types.NewString(false)

	coerced, err := boolVal.Coerce(stringType)
	if err != nil {
		t.Fatalf("Boolean should coerce to String: %v", err)
	}

	if stringVal, ok := coerced.(*StringValue); ok {
		if stringVal.Value().(string) != "true" {
			t.Errorf("Expected 'true', got '%s'", stringVal.Value().(string))
		}
	} else {
		t.Error("Coerced value should be StringValue")
	}
}

func TestValueChildren(t *testing.T) {
	// Test that primitive values have no additional methods specific to composite types
	intVal := NewInt(42, false)
	// Primitive values don't have children - this is handled by type system
	if intVal.Type().Parameters() != nil && len(intVal.Type().Parameters()) != 0 {
		t.Error("Primitive types should have no parameters")
	}

	// Test that composite values have children (will be tested in composite_test.go)
}

func TestScalarToArrayCoercion(t *testing.T) {
	// In WDL type system, scalar values do NOT automatically coerce to arrays
	// This would be handled by the expression evaluator, not the type system
	intVal := NewInt(42, false)
	arrayType := types.NewArray(types.NewInt(false), false, false)

	// This should fail - scalar to array coercion is not automatic in WDL
	_, err := intVal.Coerce(arrayType)
	if err == nil {
		t.Error("Int should NOT automatically coerce to Array[Int] in WDL type system")
	}
}
