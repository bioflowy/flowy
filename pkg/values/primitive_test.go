package values

import (
	"encoding/json"
	"testing"

	"github.com/bioflowy/flowy/pkg/types"
)

func TestBooleanValue(t *testing.T) {
	// Test true value
	trueVal := NewBoolean(true, false)
	if trueVal.Value().(bool) != true {
		t.Error("Expected true value")
	}
	if trueVal.String() != "true" {
		t.Errorf("Expected 'true', got '%s'", trueVal.String())
	}
	
	// Test false value
	falseVal := NewBoolean(false, false)
	if falseVal.Value().(bool) != false {
		t.Error("Expected false value")
	}
	if falseVal.String() != "false" {
		t.Errorf("Expected 'false', got '%s'", falseVal.String())
	}
	
	// Test JSON
	jsonData := trueVal.JSON()
	var jsonBool bool
	if err := json.Unmarshal(jsonData, &jsonBool); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	if jsonBool != true {
		t.Error("Expected JSON true")
	}
}

func TestIntValue(t *testing.T) {
	intVal := NewInt(42, false)
	
	// Test value
	if intVal.Value().(int64) != 42 {
		t.Errorf("Expected value 42, got %v", intVal.Value())
	}
	
	// Test string
	if intVal.String() != "42" {
		t.Errorf("Expected '42', got '%s'", intVal.String())
	}
	
	// Test JSON
	jsonData := intVal.JSON()
	var jsonInt int64
	if err := json.Unmarshal(jsonData, &jsonInt); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	if jsonInt != 42 {
		t.Errorf("Expected JSON 42, got %d", jsonInt)
	}
}

func TestFloatValue(t *testing.T) {
	floatVal := NewFloat(3.14159, false)
	
	// Test value
	if floatVal.Value().(float64) != 3.14159 {
		t.Errorf("Expected value 3.14159, got %v", floatVal.Value())
	}
	
	// Test string
	if floatVal.String() != "3.14159" {
		t.Errorf("Expected '3.14159', got '%s'", floatVal.String())
	}
	
	// Test JSON
	jsonData := floatVal.JSON()
	var jsonFloat float64
	if err := json.Unmarshal(jsonData, &jsonFloat); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	if jsonFloat != 3.14159 {
		t.Errorf("Expected JSON 3.14159, got %f", jsonFloat)
	}
}

func TestStringValue(t *testing.T) {
	strVal := NewString("hello world", false)
	
	// Test value
	if strVal.Value().(string) != "hello world" {
		t.Errorf("Expected value 'hello world', got %v", strVal.Value())
	}
	
	// Test string
	if strVal.String() != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", strVal.String())
	}
	
	// Test JSON
	jsonData := strVal.JSON()
	var jsonStr string
	if err := json.Unmarshal(jsonData, &jsonStr); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	if jsonStr != "hello world" {
		t.Errorf("Expected JSON 'hello world', got '%s'", jsonStr)
	}
}

func TestFileValue(t *testing.T) {
	fileVal := NewFile("/path/to/file.txt", false)
	
	// Test value
	if fileVal.Value().(string) != "/path/to/file.txt" {
		t.Errorf("Expected value '/path/to/file.txt', got %v", fileVal.Value())
	}
	
	// Test string
	if fileVal.String() != "/path/to/file.txt" {
		t.Errorf("Expected '/path/to/file.txt', got '%s'", fileVal.String())
	}
}

func TestDirectoryValue(t *testing.T) {
	dirVal := NewDirectory("/path/to/dir", false)
	
	// Test value
	if dirVal.Value().(string) != "/path/to/dir" {
		t.Errorf("Expected value '/path/to/dir', got %v", dirVal.Value())
	}
	
	// Test string
	if dirVal.String() != "/path/to/dir" {
		t.Errorf("Expected '/path/to/dir', got '%s'", dirVal.String())
	}
}

func TestPrimitiveCoercion(t *testing.T) {
	// Int to Float
	intVal := NewInt(42, false)
	floatType := types.NewFloat(false)
	coerced, err := intVal.Coerce(floatType)
	if err != nil {
		t.Fatalf("Int should coerce to Float: %v", err)
	}
	if floatVal, ok := coerced.(*FloatValue); ok {
		if floatVal.Value().(float64) != 42.0 {
			t.Errorf("Expected 42.0, got %v", floatVal.Value())
		}
	} else {
		t.Error("Coerced value should be FloatValue")
	}
	
	// String to Int
	strVal := NewString("123", false)
	intType := types.NewInt(false)
	coerced, err = strVal.Coerce(intType)
	if err != nil {
		t.Fatalf("String '123' should coerce to Int: %v", err)
	}
	if intCoerced, ok := coerced.(*IntValue); ok {
		if intCoerced.Value().(int64) != 123 {
			t.Errorf("Expected 123, got %v", intCoerced.Value())
		}
	} else {
		t.Error("Coerced value should be IntValue")
	}
	
	// String to Float
	strVal = NewString("3.14", false)
	coerced, err = strVal.Coerce(floatType)
	if err != nil {
		t.Fatalf("String '3.14' should coerce to Float: %v", err)
	}
	if floatCoerced, ok := coerced.(*FloatValue); ok {
		if floatCoerced.Value().(float64) != 3.14 {
			t.Errorf("Expected 3.14, got %v", floatCoerced.Value())
		}
	} else {
		t.Error("Coerced value should be FloatValue")
	}
	
	// File to String
	fileVal := NewFile("/path/to/file", false)
	stringType := types.NewString(false)
	coerced, err = fileVal.Coerce(stringType)
	if err != nil {
		t.Fatalf("File should coerce to String: %v", err)
	}
	if strCoerced, ok := coerced.(*StringValue); ok {
		if strCoerced.Value().(string) != "/path/to/file" {
			t.Errorf("Expected '/path/to/file', got '%s'", strCoerced.Value())
		}
	} else {
		t.Error("Coerced value should be StringValue")
	}
}

func TestPrimitiveEquality(t *testing.T) {
	// Same values are equal
	int1 := NewInt(42, false)
	int2 := NewInt(42, false)
	int3 := NewInt(43, false)
	
	if !int1.Equal(int2) {
		t.Error("Same integer values should be equal")
	}
	
	if int1.Equal(int3) {
		t.Error("Different integer values should not be equal")
	}
	
	// Int and Float with same value
	float1 := NewFloat(42.0, false)
	if !int1.Equal(float1) {
		t.Error("Int 42 and Float 42.0 should be equal")
	}
	
	// Float and Int with same value
	if !float1.Equal(int1) {
		t.Error("Float 42.0 and Int 42 should be equal")
	}
}

func TestInvalidCoercion(t *testing.T) {
	// Boolean to Int should fail
	boolVal := NewBoolean(true, false)
	intType := types.NewInt(false)
	_, err := boolVal.Coerce(intType)
	if err == nil {
		t.Error("Boolean should not coerce to Int")
	}
	
	// Invalid string to Int should fail
	strVal := NewString("not a number", false)
	_, err = strVal.Coerce(intType)
	if err == nil {
		t.Error("Invalid string should not coerce to Int")
	}
	
	// Float to Int should fail
	floatVal := NewFloat(3.14, false)
	_, err = floatVal.Coerce(intType)
	if err == nil {
		t.Error("Float should not coerce to Int")
	}
}