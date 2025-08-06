package expr

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/types"
	"github.com/bioflowy/flowy/pkg/values"
)

// Test ArrayLiteral

func TestArrayLiteralEmpty(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	arrayLit := NewArrayLiteral([]Expr{}, pos)
	
	// Test type inference - empty array should be Array[Any]
	inferredType, err := arrayLit.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Array[Any]" {
		t.Errorf("Expected Array[Any] type, got %s", inferredType.String())
	}
	
	// Test evaluation
	value, err := arrayLit.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	arrayValue, ok := value.(*values.ArrayValue)
	if !ok {
		t.Errorf("Expected ArrayValue, got %T", value)
	}
	if len(arrayValue.Items()) != 0 {
		t.Errorf("Expected empty array, got %d items", len(arrayValue.Items()))
	}
}

func TestArrayLiteralHomogeneous(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// Create [1, 2, 3]
	items := []Expr{
		NewIntLiteral(1, pos),
		NewIntLiteral(2, pos),
		NewIntLiteral(3, pos),
	}
	arrayLit := NewArrayLiteral(items, pos)
	
	// Test type inference
	inferredType, err := arrayLit.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Array[Int]" {
		t.Errorf("Expected Array[Int] type, got %s", inferredType.String())
	}
	
	// Test evaluation
	value, err := arrayLit.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	arrayValue, ok := value.(*values.ArrayValue)
	if !ok {
		t.Errorf("Expected ArrayValue, got %T", value)
	}
	
	items2 := arrayValue.Items()
	if len(items2) != 3 {
		t.Errorf("Expected 3 items, got %d", len(items2))
	}
	
	// Check values
	for i, expected := range []int64{1, 2, 3} {
		intVal, ok := items2[i].(*values.IntValue)
		if !ok {
			t.Errorf("Expected IntValue at index %d, got %T", i, items2[i])
		} else if intVal.Value().(int64) != expected {
			t.Errorf("Expected %d at index %d, got %d", expected, i, intVal.Value().(int64))
		}
	}
	
	// Test string representation
	if arrayLit.String() != "[1, 2, 3]" {
		t.Errorf("Expected '[1, 2, 3]', got %s", arrayLit.String())
	}
	
	// Test children
	children := arrayLit.Children()
	if len(children) != 3 {
		t.Errorf("Expected 3 children, got %d", len(children))
	}
}

func TestArrayLiteralMixed(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// Create [1, 2.5] - Int and Float should unify to Float
	items := []Expr{
		NewIntLiteral(1, pos),
		NewFloatLiteral(2.5, pos),
	}
	arrayLit := NewArrayLiteral(items, pos)
	
	// Test type inference
	inferredType, err := arrayLit.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Array[Float]" {
		t.Errorf("Expected Array[Float] type, got %s", inferredType.String())
	}
}

func TestArrayLiteralIncompatibleTypes(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// Create [1, "hello"] - Int and String can't unify
	items := []Expr{
		NewIntLiteral(1, pos),
		NewStringLiteral("hello", pos),
	}
	arrayLit := NewArrayLiteral(items, pos)
	
	// Test type inference - should fail
	_, err := arrayLit.InferType(nil, nil)
	if err == nil {
		t.Errorf("Expected type inference to fail for incompatible array item types")
	}
}

// Test PairLiteral

func TestPairLiteral(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	leftExpr := NewIntLiteral(42, pos)
	rightExpr := NewStringLiteral("hello", pos)
	pairLit := NewPairLiteral(leftExpr, rightExpr, pos)
	
	// Test type inference
	inferredType, err := pairLit.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Pair[Int,String]" {
		t.Errorf("Expected Pair[Int,String] type, got %s", inferredType.String())
	}
	
	// Test evaluation
	value, err := pairLit.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	pairValue, ok := value.(*values.PairValue)
	if !ok {
		t.Errorf("Expected PairValue, got %T", value)
	}
	
	// Check left value
	leftVal, ok := pairValue.Left().(*values.IntValue)
	if !ok {
		t.Errorf("Expected IntValue for left, got %T", pairValue.Left())
	} else if leftVal.Value().(int64) != 42 {
		t.Errorf("Expected 42 for left, got %d", leftVal.Value().(int64))
	}
	
	// Check right value
	rightVal, ok := pairValue.Right().(*values.StringValue)
	if !ok {
		t.Errorf("Expected StringValue for right, got %T", pairValue.Right())
	} else if rightVal.Value().(string) != "hello" {
		t.Errorf("Expected 'hello' for right, got %s", rightVal.Value().(string))
	}
	
	// Test string representation
	if pairLit.String() != "(42, \"hello\")" {
		t.Errorf("Expected '(42, \"hello\")', got %s", pairLit.String())
	}
	
	// Test children
	children := pairLit.Children()
	if len(children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(children))
	}
}

// Test MapLiteral

func TestMapLiteralEmpty(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	mapLit := NewMapLiteral([]MapItem{}, pos)
	
	// Test type inference - empty map should be Map[String, Any]
	inferredType, err := mapLit.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Map[String,Any]" {
		t.Errorf("Expected Map[String,Any] type, got %s", inferredType.String())
	}
	
	// Test evaluation
	value, err := mapLit.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	_, ok := value.(*values.MapValue)
	if !ok {
		t.Errorf("Expected MapValue, got %T", value)
	}
	
	// Map should be empty - we don't have a direct way to check size, 
	// so we'll trust the evaluation worked if it didn't error
}

func TestMapLiteralSimple(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// Create {"name": "John", "age": "30"}
	items := []MapItem{
		{Key: NewStringLiteral("name", pos), Value: NewStringLiteral("John", pos)},
		{Key: NewStringLiteral("age", pos), Value: NewStringLiteral("30", pos)},
	}
	mapLit := NewMapLiteral(items, pos)
	
	// Test type inference
	inferredType, err := mapLit.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Map[String,String]" {
		t.Errorf("Expected Map[String,String] type, got %s", inferredType.String())
	}
	
	// Test evaluation
	value, err := mapLit.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	mapValue, ok := value.(*values.MapValue)
	if !ok {
		t.Errorf("Expected MapValue, got %T", value)
	}
	
	// Check values
	nameValue, ok := mapValue.Get("name")
	if !ok {
		t.Errorf("Expected to find 'name' key in map")
	} else {
		nameStr, ok := nameValue.(*values.StringValue)
		if !ok {
			t.Errorf("Expected StringValue for name, got %T", nameValue)
		} else if nameStr.Value().(string) != "John" {
			t.Errorf("Expected 'John' for name, got %s", nameStr.Value().(string))
		}
	}
	
	// Test string representation
	expected := "{\"name\": \"John\", \"age\": \"30\"}"
	if mapLit.String() != expected {
		t.Errorf("Expected '%s', got %s", expected, mapLit.String())
	}
}

func TestMapLiteralMixedValues(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// Create {"count": 42, "name": "test"} - mixed Int and String values should unify to Any
	items := []MapItem{
		{Key: NewStringLiteral("count", pos), Value: NewIntLiteral(42, pos)},
		{Key: NewStringLiteral("name", pos), Value: NewStringLiteral("test", pos)},
	}
	mapLit := NewMapLiteral(items, pos)
	
	// Test type inference - value types are incompatible so should fail
	_, err := mapLit.InferType(nil, nil)
	if err == nil {
		t.Errorf("Expected type inference to fail for incompatible map value types")
	}
}

// Test StructLiteral

func TestStructLiteral(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// Create Person{name: "John", age: 30}
	members := []StructMember{
		{Name: "name", Value: NewStringLiteral("John", pos)},
		{Name: "age", Value: NewIntLiteral(30, pos)},
	}
	structLit := NewStructLiteral("Person", members, pos)
	
	// Test type inference
	inferredType, err := structLit.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	
	// The type should be a StructInstanceType
	structType, ok := inferredType.(*types.StructInstanceType)
	if !ok {
		t.Errorf("Expected StructInstanceType, got %T", inferredType)
	}
	
	// Check member types
	memberTypes := structType.Members()
	if len(memberTypes) != 2 {
		t.Errorf("Expected 2 member types, got %d", len(memberTypes))
	}
	
	if nameType, ok := memberTypes["name"]; !ok {
		t.Errorf("Expected 'name' member type")
	} else if nameType.String() != "String" {
		t.Errorf("Expected String type for name, got %s", nameType.String())
	}
	
	if ageType, ok := memberTypes["age"]; !ok {
		t.Errorf("Expected 'age' member type")
	} else if ageType.String() != "Int" {
		t.Errorf("Expected Int type for age, got %s", ageType.String())
	}
	
	// Test evaluation
	value, err := structLit.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	structValue, ok := value.(*values.StructValue)
	if !ok {
		t.Errorf("Expected StructValue, got %T", value)
	}
	
	// Check member values
	if nameValue, ok := structValue.Get("name"); !ok {
		t.Errorf("Expected 'name' member value")
	} else {
		nameStr, ok := nameValue.(*values.StringValue)
		if !ok {
			t.Errorf("Expected StringValue for name, got %T", nameValue)
		} else if nameStr.Value().(string) != "John" {
			t.Errorf("Expected 'John' for name, got %s", nameStr.Value().(string))
		}
	}
	
	// Test string representation
	expected := "Person{name: \"John\", age: 30}"
	if structLit.String() != expected {
		t.Errorf("Expected '%s', got %s", expected, structLit.String())
	}
	
	// Test children
	children := structLit.Children()
	if len(children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(children))
	}
}

// Test type checking

func TestCollectionTypeCheck(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	tests := []struct {
		name         string
		expr         Expr
		expectedType types.Base
		shouldFail   bool
	}{
		{
			name:         "Array[Int] to Array[Int]",
			expr:         NewArrayLiteral([]Expr{NewIntLiteral(1, pos)}, pos),
			expectedType: types.NewArray(types.NewInt(false), false, false),
			shouldFail:   false,
		},
		{
			name:         "Array[Int] to Array[Float]",
			expr:         NewArrayLiteral([]Expr{NewIntLiteral(1, pos)}, pos),
			expectedType: types.NewArray(types.NewFloat(false), false, false),
			shouldFail:   true, // Arrays require exact type match
		},
		{
			name:         "Pair[Int,String] to Pair[Int,String]",
			expr:         NewPairLiteral(NewIntLiteral(1, pos), NewStringLiteral("test", pos), pos),
			expectedType: types.NewPair(types.NewInt(false), types.NewString(false), false),
			shouldFail:   false,
		},
		{
			name:         "Map[String,Int] to Map[String,Int]",
			expr:         NewMapLiteral([]MapItem{{Key: NewStringLiteral("key", pos), Value: NewIntLiteral(1, pos)}}, pos),
			expectedType: types.NewMap(types.NewString(false), types.NewInt(false), false),
			shouldFail:   false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.expr.TypeCheck(tt.expectedType, nil, nil)
			
			if tt.shouldFail && err == nil {
				t.Errorf("Expected type check to fail but it succeeded")
			}
			
			if !tt.shouldFail && err != nil {
				t.Errorf("Expected type check to succeed but got error: %v", err)
			}
		})
	}
}

// Test literal detection

func TestCollectionLiterals(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// Array literals are not considered literals (contain sub-expressions)
	arrayLit := NewArrayLiteral([]Expr{NewIntLiteral(1, pos)}, pos)
	_, isLiteral := arrayLit.Literal()
	if isLiteral {
		t.Errorf("Expected array literal to not be considered a literal")
	}
	
	// Same for other collection types
	pairLit := NewPairLiteral(NewIntLiteral(1, pos), NewStringLiteral("test", pos), pos)
	_, isLiteral = pairLit.Literal()
	if isLiteral {
		t.Errorf("Expected pair literal to not be considered a literal")
	}
	
	mapLit := NewMapLiteral([]MapItem{{Key: NewStringLiteral("key", pos), Value: NewIntLiteral(1, pos)}}, pos)
	_, isLiteral = mapLit.Literal()
	if isLiteral {
		t.Errorf("Expected map literal to not be considered a literal")
	}
	
	structLit := NewStructLiteral("Test", []StructMember{{Name: "field", Value: NewIntLiteral(1, pos)}}, pos)
	_, isLiteral = structLit.Literal()
	if isLiteral {
		t.Errorf("Expected struct literal to not be considered a literal")
	}
}