package expr

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/types"
	"github.com/bioflowy/flowy/pkg/values"
)

// Test Identifier

func TestIdentifierSimple(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	identifier := NewIdentifier("myVar", pos)

	// Create environments
	typeEnv := env.NewBindings[types.Base]()
	typeEnv = typeEnv.Bind("myVar", types.NewString(false), nil)

	valueEnv := env.NewBindings[values.Base]()
	valueEnv = valueEnv.Bind("myVar", values.NewString("hello", false), nil)

	// Test type inference
	inferredType, err := identifier.InferType(typeEnv, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "String" {
		t.Errorf("Expected String type, got %s", inferredType.String())
	}

	// Test evaluation
	value, err := identifier.Eval(valueEnv, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	stringValue, ok := value.(*values.StringValue)
	if !ok {
		t.Errorf("Expected StringValue, got %T", value)
	}
	if stringValue.Value().(string) != "hello" {
		t.Errorf("Expected 'hello', got %s", stringValue.Value().(string))
	}

	// Test string representation
	if identifier.String() != "myVar" {
		t.Errorf("Expected 'myVar', got %s", identifier.String())
	}

	// Test children (should be empty)
	children := identifier.Children()
	if len(children) != 0 {
		t.Errorf("Expected no children, got %d", len(children))
	}
}

func TestIdentifierNamespaced(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	identifier := NewNamespacedIdentifier([]string{"task", "output"}, "result", pos)

	// Create environments
	typeEnv := env.NewBindings[types.Base]()
	typeEnv = typeEnv.Bind("task.output.result", types.NewInt(false), nil)

	valueEnv := env.NewBindings[values.Base]()
	valueEnv = valueEnv.Bind("task.output.result", values.NewInt(42, false), nil)

	// Test type inference
	inferredType, err := identifier.InferType(typeEnv, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Int" {
		t.Errorf("Expected Int type, got %s", inferredType.String())
	}

	// Test evaluation
	value, err := identifier.Eval(valueEnv, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	intValue, ok := value.(*values.IntValue)
	if !ok {
		t.Errorf("Expected IntValue, got %T", value)
	}
	if intValue.Value().(int64) != 42 {
		t.Errorf("Expected 42, got %d", intValue.Value().(int64))
	}

	// Test string representation
	if identifier.String() != "task.output.result" {
		t.Errorf("Expected 'task.output.result', got %s", identifier.String())
	}
}

func TestIdentifierUnknown(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	identifier := NewIdentifier("unknownVar", pos)

	// Empty environments
	typeEnv := env.NewBindings[types.Base]()
	valueEnv := env.NewBindings[values.Base]()

	// Test type inference - should fail
	_, err := identifier.InferType(typeEnv, nil)
	if err == nil {
		t.Errorf("Expected type inference to fail for unknown identifier")
	}

	// Test evaluation - should fail
	_, err = identifier.Eval(valueEnv, nil)
	if err == nil {
		t.Errorf("Expected evaluation to fail for unknown identifier")
	}
}

// Test GetAttr

func TestGetAttrStruct(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create person.name
	personVar := NewIdentifier("person", pos)
	getAttr := NewGetAttr(personVar, "name", pos)

	// Create struct type and value
	memberTypes := map[string]types.Base{
		"name": types.NewString(false),
		"age":  types.NewInt(false),
	}
	structType := types.NewStructInstance("Person", memberTypes, false)

	memberValues := map[string]values.Base{
		"name": values.NewString("John", false),
		"age":  values.NewInt(30, false),
	}
	structValue := values.NewStruct("Person", memberTypes, memberValues, false)

	// Create environments
	typeEnv := env.NewBindings[types.Base]()
	typeEnv = typeEnv.Bind("person", structType, nil)

	valueEnv := env.NewBindings[values.Base]()
	valueEnv = valueEnv.Bind("person", structValue, nil)

	// Test type inference
	inferredType, err := getAttr.InferType(typeEnv, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "String" {
		t.Errorf("Expected String type, got %s", inferredType.String())
	}

	// Test evaluation
	value, err := getAttr.Eval(valueEnv, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	stringValue, ok := value.(*values.StringValue)
	if !ok {
		t.Errorf("Expected StringValue, got %T", value)
	}
	if stringValue.Value().(string) != "John" {
		t.Errorf("Expected 'John', got %s", stringValue.Value().(string))
	}

	// Test string representation
	if getAttr.String() != "person.name" {
		t.Errorf("Expected 'person.name', got %s", getAttr.String())
	}

	// Test children
	children := getAttr.Children()
	if len(children) != 1 {
		t.Errorf("Expected 1 child, got %d", len(children))
	}
	if children[0] != personVar {
		t.Errorf("Expected child to be the person identifier")
	}
}

func TestGetAttrPair(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create pair.left
	pairVar := NewIdentifier("pair", pos)
	getAttr := NewGetAttr(pairVar, "left", pos)

	// Create pair type and value
	pairType := types.NewPair(types.NewInt(false), types.NewString(false), false)
	pairValue := values.NewPair(types.NewInt(false), types.NewString(false),
		values.NewInt(42, false), values.NewString("test", false), false)

	// Create environments
	typeEnv := env.NewBindings[types.Base]()
	typeEnv = typeEnv.Bind("pair", pairType, nil)

	valueEnv := env.NewBindings[values.Base]()
	valueEnv = valueEnv.Bind("pair", pairValue, nil)

	// Test type inference
	inferredType, err := getAttr.InferType(typeEnv, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Int" {
		t.Errorf("Expected Int type, got %s", inferredType.String())
	}

	// Test evaluation
	value, err := getAttr.Eval(valueEnv, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	intValue, ok := value.(*values.IntValue)
	if !ok {
		t.Errorf("Expected IntValue, got %T", value)
	}
	if intValue.Value().(int64) != 42 {
		t.Errorf("Expected 42, got %d", intValue.Value().(int64))
	}
}

func TestGetAttrNoSuchMember(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create person.nonexistent
	personVar := NewIdentifier("person", pos)
	getAttr := NewGetAttr(personVar, "nonexistent", pos)

	// Create struct type and value
	memberTypes := map[string]types.Base{
		"name": types.NewString(false),
	}
	structType := types.NewStructInstance("Person", memberTypes, false)

	// Create environments
	typeEnv := env.NewBindings[types.Base]()
	typeEnv = typeEnv.Bind("person", structType, nil)

	// Test type inference - should fail
	_, err := getAttr.InferType(typeEnv, nil)
	if err == nil {
		t.Errorf("Expected type inference to fail for nonexistent member")
	}
}

// Test GetIndex

func TestGetIndexArray(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create arr[1]
	arrayVar := NewIdentifier("arr", pos)
	indexExpr := NewIntLiteral(1, pos)
	getIndex := NewGetIndex(arrayVar, indexExpr, pos)

	// Create array type and value
	arrayType := types.NewArray(types.NewString(false), false, false)
	arrayValue := values.NewArray(types.NewString(false), false, false)
	arrayValue.Add(values.NewString("first", false))
	arrayValue.Add(values.NewString("second", false))
	arrayValue.Add(values.NewString("third", false))

	// Create environments
	typeEnv := env.NewBindings[types.Base]()
	typeEnv = typeEnv.Bind("arr", arrayType, nil)

	valueEnv := env.NewBindings[values.Base]()
	valueEnv = valueEnv.Bind("arr", arrayValue, nil)

	// Test type inference
	inferredType, err := getIndex.InferType(typeEnv, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "String" {
		t.Errorf("Expected String type, got %s", inferredType.String())
	}

	// Test evaluation
	value, err := getIndex.Eval(valueEnv, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	stringValue, ok := value.(*values.StringValue)
	if !ok {
		t.Errorf("Expected StringValue, got %T", value)
	}
	if stringValue.Value().(string) != "second" {
		t.Errorf("Expected 'second', got %s", stringValue.Value().(string))
	}

	// Test string representation
	if getIndex.String() != "arr[1]" {
		t.Errorf("Expected 'arr[1]', got %s", getIndex.String())
	}

	// Test children
	children := getIndex.Children()
	if len(children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(children))
	}
}

func TestGetIndexMap(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create dict["key"]
	mapVar := NewIdentifier("dict", pos)
	keyExpr := NewStringLiteral("key", pos)
	getIndex := NewGetIndex(mapVar, keyExpr, pos)

	// Create map type and value
	mapType := types.NewMap(types.NewString(false), types.NewInt(false), false)
	mapValue := values.NewMap(types.NewString(false), types.NewInt(false), false)
	mapValue.Set("key", values.NewInt(42, false))
	mapValue.Set("other", values.NewInt(100, false))

	// Create environments
	typeEnv := env.NewBindings[types.Base]()
	typeEnv = typeEnv.Bind("dict", mapType, nil)

	valueEnv := env.NewBindings[values.Base]()
	valueEnv = valueEnv.Bind("dict", mapValue, nil)

	// Test type inference
	inferredType, err := getIndex.InferType(typeEnv, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Int" {
		t.Errorf("Expected Int type, got %s", inferredType.String())
	}

	// Test evaluation
	value, err := getIndex.Eval(valueEnv, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	intValue, ok := value.(*values.IntValue)
	if !ok {
		t.Errorf("Expected IntValue, got %T", value)
	}
	if intValue.Value().(int64) != 42 {
		t.Errorf("Expected 42, got %d", intValue.Value().(int64))
	}
}

func TestGetIndexOutOfBounds(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create arr[10] (out of bounds)
	arrayVar := NewIdentifier("arr", pos)
	indexExpr := NewIntLiteral(10, pos)
	getIndex := NewGetIndex(arrayVar, indexExpr, pos)

	// Create small array
	arrayValue := values.NewArray(types.NewString(false), false, false)
	arrayValue.Add(values.NewString("only", false))

	// Create environments
	valueEnv := env.NewBindings[values.Base]()
	valueEnv = valueEnv.Bind("arr", arrayValue, nil)

	// Test evaluation - should fail
	_, err := getIndex.Eval(valueEnv, nil)
	if err == nil {
		t.Errorf("Expected evaluation to fail for out of bounds access")
	}
}

// Test Slice

func TestSliceBasic(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create arr[1:3]
	arrayVar := NewIdentifier("arr", pos)
	startExpr := NewIntLiteral(1, pos)
	endExpr := NewIntLiteral(3, pos)
	slice := NewSlice(arrayVar, startExpr, endExpr, pos)

	// Create array type and value
	arrayType := types.NewArray(types.NewString(false), false, false)
	arrayValue := values.NewArray(types.NewString(false), false, false)
	for i, str := range []string{"a", "b", "c", "d", "e"} {
		arrayValue.Add(values.NewString(str, false))
		_ = i // suppress unused variable warning
	}

	// Create environments
	typeEnv := env.NewBindings[types.Base]()
	typeEnv = typeEnv.Bind("arr", arrayType, nil)

	valueEnv := env.NewBindings[values.Base]()
	valueEnv = valueEnv.Bind("arr", arrayValue, nil)

	// Test type inference
	inferredType, err := slice.InferType(typeEnv, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Array[String]" {
		t.Errorf("Expected Array[String] type, got %s", inferredType.String())
	}

	// Test evaluation
	value, err := slice.Eval(valueEnv, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	resultArray, ok := value.(*values.ArrayValue)
	if !ok {
		t.Errorf("Expected ArrayValue, got %T", value)
	}

	items := resultArray.Items()
	if len(items) != 2 { // arr[1:3] should include indices 1 and 2
		t.Errorf("Expected 2 items in slice, got %d", len(items))
	}

	// Check values
	expectedValues := []string{"b", "c"}
	for i, expected := range expectedValues {
		if i < len(items) {
			stringVal, ok := items[i].(*values.StringValue)
			if !ok {
				t.Errorf("Expected StringValue at index %d, got %T", i, items[i])
			} else if stringVal.Value().(string) != expected {
				t.Errorf("Expected '%s' at index %d, got %s", expected, i, stringVal.Value().(string))
			}
		}
	}

	// Test string representation
	if slice.String() != "arr[1:3]" {
		t.Errorf("Expected 'arr[1:3]', got %s", slice.String())
	}

	// Test children
	children := slice.Children()
	if len(children) != 3 { // array, start, end
		t.Errorf("Expected 3 children, got %d", len(children))
	}
}

func TestSlicePartial(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create arr[1:] (from index 1 to end)
	arrayVar := NewIdentifier("arr", pos)
	startExpr := NewIntLiteral(1, pos)
	slice := NewSlice(arrayVar, startExpr, nil, pos)

	// Create array value
	arrayValue := values.NewArray(types.NewString(false), false, false)
	for _, str := range []string{"a", "b", "c"} {
		arrayValue.Add(values.NewString(str, false))
	}

	// Create environments
	valueEnv := env.NewBindings[values.Base]()
	valueEnv = valueEnv.Bind("arr", arrayValue, nil)

	// Test evaluation
	value, err := slice.Eval(valueEnv, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	resultArray, ok := value.(*values.ArrayValue)
	if !ok {
		t.Errorf("Expected ArrayValue, got %T", value)
	}

	items := resultArray.Items()
	if len(items) != 2 { // Should include "b" and "c"
		t.Errorf("Expected 2 items in slice, got %d", len(items))
	}

	// Test string representation
	if slice.String() != "arr[1:]" {
		t.Errorf("Expected 'arr[1:]', got %s", slice.String())
	}
}

// Test type checking

func TestReferenceTypeCheck(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create environments
	typeEnv := env.NewBindings[types.Base]()
	typeEnv = typeEnv.Bind("stringVar", types.NewString(false), nil)
	typeEnv = typeEnv.Bind("intArray", types.NewArray(types.NewInt(false), false, false), nil)

	tests := []struct {
		name         string
		expr         Expr
		expectedType types.Base
		shouldFail   bool
	}{
		{
			name:         "String identifier to String",
			expr:         NewIdentifier("stringVar", pos),
			expectedType: types.NewString(false),
			shouldFail:   false,
		},
		{
			name:         "String identifier to Int (should fail)",
			expr:         NewIdentifier("stringVar", pos),
			expectedType: types.NewBoolean(false), // Boolean can't coerce from String
			shouldFail:   true,
		},
		{
			name:         "Array[Int] to Array[Int]",
			expr:         NewIdentifier("intArray", pos),
			expectedType: types.NewArray(types.NewInt(false), false, false),
			shouldFail:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.expr.TypeCheck(tt.expectedType, typeEnv, nil)

			if tt.shouldFail && err == nil {
				t.Errorf("Expected type check to fail but it succeeded")
			}

			if !tt.shouldFail && err != nil {
				t.Errorf("Expected type check to succeed but got error: %v", err)
			}
		})
	}
}
