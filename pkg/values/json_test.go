package values

import (
	"encoding/json"
	"testing"

	"github.com/bioflowy/flowy/pkg/types"
)

func TestFromJSON(t *testing.T) {
	// Test parsing Boolean
	boolType := types.NewBoolean(false)
	boolData := json.RawMessage("true")
	val, err := FromJSON(boolData, boolType)
	if err != nil {
		t.Fatalf("Failed to parse boolean: %v", err)
	}
	if boolVal, ok := val.(*BooleanValue); ok {
		if boolVal.Value().(bool) != true {
			t.Error("Expected true")
		}
	} else {
		t.Error("Expected BooleanValue")
	}

	// Test parsing Int
	intType := types.NewInt(false)
	intData := json.RawMessage("42")
	val, err = FromJSON(intData, intType)
	if err != nil {
		t.Fatalf("Failed to parse int: %v", err)
	}
	if intVal, ok := val.(*IntValue); ok {
		if intVal.Value().(int64) != 42 {
			t.Errorf("Expected 42, got %v", intVal.Value())
		}
	} else {
		t.Error("Expected IntValue")
	}

	// Test parsing Float
	floatType := types.NewFloat(false)
	floatData := json.RawMessage("3.14")
	val, err = FromJSON(floatData, floatType)
	if err != nil {
		t.Fatalf("Failed to parse float: %v", err)
	}
	if floatVal, ok := val.(*FloatValue); ok {
		if floatVal.Value().(float64) != 3.14 {
			t.Errorf("Expected 3.14, got %v", floatVal.Value())
		}
	} else {
		t.Error("Expected FloatValue")
	}

	// Test parsing String
	stringType := types.NewString(false)
	stringData := json.RawMessage(`"hello"`)
	val, err = FromJSON(stringData, stringType)
	if err != nil {
		t.Fatalf("Failed to parse string: %v", err)
	}
	if strVal, ok := val.(*StringValue); ok {
		if strVal.Value().(string) != "hello" {
			t.Errorf("Expected 'hello', got '%s'", strVal.Value())
		}
	} else {
		t.Error("Expected StringValue")
	}

	// Test parsing null
	optionalIntType := types.NewInt(true)
	nullData := json.RawMessage("null")
	val, err = FromJSON(nullData, optionalIntType)
	if err != nil {
		t.Fatalf("Failed to parse null: %v", err)
	}
	if _, ok := val.(*Null); !ok {
		t.Error("Expected Null value")
	}

	// Test parsing null for non-optional type should fail
	nonOptionalIntType := types.NewInt(false)
	_, err = FromJSON(nullData, nonOptionalIntType)
	if err == nil {
		t.Error("Parsing null for non-optional type should fail")
	}
}

func TestFromJSONArray(t *testing.T) {
	intType := types.NewInt(false)
	arrayType := types.NewArray(intType, false, false)
	arrayData := json.RawMessage("[1, 2, 3]")

	val, err := FromJSON(arrayData, arrayType)
	if err != nil {
		t.Fatalf("Failed to parse array: %v", err)
	}

	if arrayVal, ok := val.(*ArrayValue); ok {
		if len(arrayVal.Items()) != 3 {
			t.Errorf("Expected 3 items, got %d", len(arrayVal.Items()))
		}
		// Check first item
		if intVal, ok := arrayVal.Items()[0].(*IntValue); ok {
			if intVal.Value().(int64) != 1 {
				t.Errorf("Expected first item to be 1, got %v", intVal.Value())
			}
		} else {
			t.Error("First item should be IntValue")
		}
	} else {
		t.Error("Expected ArrayValue")
	}

	// Test non-empty array constraint
	nonemptyArrayType := types.NewArray(intType, false, true)
	emptyArrayData := json.RawMessage("[]")
	_, err = FromJSON(emptyArrayData, nonemptyArrayType)
	if err == nil {
		t.Error("Empty array should fail for non-empty array type")
	}
}

func TestFromJSONMap(t *testing.T) {
	stringType := types.NewString(false)
	intType := types.NewInt(false)
	mapType := types.NewMap(stringType, intType, false)
	mapData := json.RawMessage(`{"a": 1, "b": 2, "c": 3}`)

	val, err := FromJSON(mapData, mapType)
	if err != nil {
		t.Fatalf("Failed to parse map: %v", err)
	}

	if mapVal, ok := val.(*MapValue); ok {
		if len(mapVal.Entries()) != 3 {
			t.Errorf("Expected 3 entries, got %d", len(mapVal.Entries()))
		}
		// Check specific entry
		if val, ok := mapVal.Get("b"); ok {
			if intVal, ok := val.(*IntValue); ok {
				if intVal.Value().(int64) != 2 {
					t.Errorf("Expected b:2, got %v", intVal.Value())
				}
			} else {
				t.Error("Value should be IntValue")
			}
		} else {
			t.Error("Expected to find key 'b'")
		}
	} else {
		t.Error("Expected MapValue")
	}
}

func TestFromJSONPair(t *testing.T) {
	stringType := types.NewString(false)
	intType := types.NewInt(false)
	pairType := types.NewPair(stringType, intType, false)
	pairData := json.RawMessage(`["hello", 42]`)

	val, err := FromJSON(pairData, pairType)
	if err != nil {
		t.Fatalf("Failed to parse pair: %v", err)
	}

	if pairVal, ok := val.(*PairValue); ok {
		if strVal, ok := pairVal.Left().(*StringValue); ok {
			if strVal.Value().(string) != "hello" {
				t.Errorf("Expected left 'hello', got '%s'", strVal.Value())
			}
		} else {
			t.Error("Left should be StringValue")
		}
		if intVal, ok := pairVal.Right().(*IntValue); ok {
			if intVal.Value().(int64) != 42 {
				t.Errorf("Expected right 42, got %v", intVal.Value())
			}
		} else {
			t.Error("Right should be IntValue")
		}
	} else {
		t.Error("Expected PairValue")
	}

	// Test invalid pair (wrong number of elements)
	invalidPairData := json.RawMessage(`["hello"]`)
	_, err = FromJSON(invalidPairData, pairType)
	if err == nil {
		t.Error("Pair with 1 element should fail")
	}
}

func TestFromJSONStruct(t *testing.T) {
	memberTypes := map[string]types.Base{
		"name": types.NewString(false),
		"age":  types.NewInt(false),
		"city": types.NewString(true), // optional
	}
	structType := types.NewStructInstance("Person", memberTypes, false)
	structData := json.RawMessage(`{"name": "Alice", "age": 30}`)

	val, err := FromJSON(structData, structType)
	if err != nil {
		t.Fatalf("Failed to parse struct: %v", err)
	}

	if structVal, ok := val.(*StructValue); ok {
		if nameVal, ok := structVal.Get("name"); ok {
			if strVal, ok := nameVal.(*StringValue); ok {
				if strVal.Value().(string) != "Alice" {
					t.Errorf("Expected name 'Alice', got '%s'", strVal.Value())
				}
			} else {
				t.Error("Name should be StringValue")
			}
		} else {
			t.Error("Expected to find member 'name'")
		}
		if ageVal, ok := structVal.Get("age"); ok {
			if intVal, ok := ageVal.(*IntValue); ok {
				if intVal.Value().(int64) != 30 {
					t.Errorf("Expected age 30, got %v", intVal.Value())
				}
			} else {
				t.Error("Age should be IntValue")
			}
		} else {
			t.Error("Expected to find member 'age'")
		}
		// Optional field not provided is OK
		if _, ok := structVal.Get("city"); ok {
			t.Error("Optional field 'city' should not be present")
		}
	} else {
		t.Error("Expected StructValue")
	}

	// Test missing required field
	invalidStructData := json.RawMessage(`{"name": "Alice"}`)
	_, err = FromJSON(invalidStructData, structType)
	if err == nil {
		t.Error("Struct with missing required field 'age' should fail")
	}
}

func TestInferValueFromJSON(t *testing.T) {
	// Test inferring boolean
	boolData := json.RawMessage("true")
	val, err := inferValueFromJSON(boolData)
	if err != nil {
		t.Fatalf("Failed to infer boolean: %v", err)
	}
	if _, ok := val.(*BooleanValue); !ok {
		t.Error("Expected BooleanValue")
	}

	// Test inferring integer
	intData := json.RawMessage("42")
	val, err = inferValueFromJSON(intData)
	if err != nil {
		t.Fatalf("Failed to infer int: %v", err)
	}
	if _, ok := val.(*IntValue); !ok {
		t.Error("Expected IntValue")
	}

	// Test inferring float
	floatData := json.RawMessage("3.14")
	val, err = inferValueFromJSON(floatData)
	if err != nil {
		t.Fatalf("Failed to infer float: %v", err)
	}
	if _, ok := val.(*FloatValue); !ok {
		t.Error("Expected FloatValue")
	}

	// Test inferring string
	stringData := json.RawMessage(`"hello"`)
	val, err = inferValueFromJSON(stringData)
	if err != nil {
		t.Fatalf("Failed to infer string: %v", err)
	}
	if _, ok := val.(*StringValue); !ok {
		t.Error("Expected StringValue")
	}

	// Test inferring array
	arrayData := json.RawMessage("[1, 2, 3]")
	val, err = inferValueFromJSON(arrayData)
	if err != nil {
		t.Fatalf("Failed to infer array: %v", err)
	}
	if arrayVal, ok := val.(*ArrayValue); ok {
		if len(arrayVal.Items()) != 3 {
			t.Errorf("Expected 3 items, got %d", len(arrayVal.Items()))
		}
	} else {
		t.Error("Expected ArrayValue")
	}

	// Test inferring object
	objectData := json.RawMessage(`{"a": 1, "b": "hello"}`)
	val, err = inferValueFromJSON(objectData)
	if err != nil {
		t.Fatalf("Failed to infer object: %v", err)
	}
	if _, ok := val.(*ObjectValue); !ok {
		t.Error("Expected ObjectValue")
	}

	// Test inferring null
	nullData := json.RawMessage("null")
	val, err = inferValueFromJSON(nullData)
	if err != nil {
		t.Fatalf("Failed to infer null: %v", err)
	}
	if _, ok := val.(*Null); !ok {
		t.Error("Expected Null")
	}
}

func TestToJSON(t *testing.T) {
	// Test Boolean to JSON
	boolVal := NewBoolean(true, false)
	jsonData, err := ToJSON(boolVal)
	if err != nil {
		t.Fatalf("Failed to convert boolean to JSON: %v", err)
	}
	if string(jsonData) != "true" {
		t.Errorf("Expected JSON 'true', got '%s'", string(jsonData))
	}

	// Test Int to JSON
	intVal := NewInt(42, false)
	jsonData, err = ToJSON(intVal)
	if err != nil {
		t.Fatalf("Failed to convert int to JSON: %v", err)
	}
	if string(jsonData) != "42" {
		t.Errorf("Expected JSON '42', got '%s'", string(jsonData))
	}

	// Test String to JSON
	strVal := NewString("hello", false)
	jsonData, err = ToJSON(strVal)
	if err != nil {
		t.Fatalf("Failed to convert string to JSON: %v", err)
	}
	if string(jsonData) != `"hello"` {
		t.Errorf("Expected JSON '\"hello\"', got '%s'", string(jsonData))
	}

	// Test null to JSON
	nullVal := NewNull(types.NewInt(true))
	jsonData, err = ToJSON(nullVal)
	if err != nil {
		t.Fatalf("Failed to convert null to JSON: %v", err)
	}
	if string(jsonData) != "null" {
		t.Errorf("Expected JSON 'null', got '%s'", string(jsonData))
	}
}
