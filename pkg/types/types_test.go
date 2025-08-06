package types

import (
	"testing"
)

func TestBaseInterface(t *testing.T) {
	// Test basic types implement Base interface
	var _ Base = &IntType{}
	var _ Base = &StringType{}
	var _ Base = &BooleanType{}
	var _ Base = &FloatType{}
	var _ Base = &FileType{}
	var _ Base = &DirectoryType{}
	var _ Base = &ArrayType{}
	var _ Base = &MapType{}
	var _ Base = &PairType{}
	var _ Base = &AnyType{}
}

func TestPrimitiveTypes(t *testing.T) {
	tests := []struct {
		name     string
		typ      Base
		expected string
		optional bool
	}{
		{"Int", NewInt(false), "Int", false},
		{"Int?", NewInt(true), "Int?", true},
		{"String", NewString(false), "String", false},
		{"String?", NewString(true), "String?", true},
		{"Boolean", NewBoolean(false), "Boolean", false},
		{"Boolean?", NewBoolean(true), "Boolean?", true},
		{"Float", NewFloat(false), "Float", false},
		{"Float?", NewFloat(true), "Float?", true},
		{"File", NewFile(false), "File", false},
		{"File?", NewFile(true), "File?", true},
		{"Directory", NewDirectory(false), "Directory", false},
		{"Directory?", NewDirectory(true), "Directory?", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.typ.String() != tt.expected {
				t.Errorf("Expected string representation '%s', got '%s'", tt.expected, tt.typ.String())
			}
			if tt.typ.Optional() != tt.optional {
				t.Errorf("Expected optional=%t, got %t", tt.optional, tt.typ.Optional())
			}
		})
	}
}

func TestAnyType(t *testing.T) {
	any1 := NewAny(false, false)
	any2 := NewAny(true, true) // None literal

	if any1.String() != "Any" {
		t.Errorf("Expected 'Any', got '%s'", any1.String())
	}
	if any2.String() != "None" {
		t.Errorf("Expected 'None', got '%s'", any2.String())
	}
}

func TestArrayType(t *testing.T) {
	intArray := NewArray(NewInt(false), false, false)
	stringArrayOptional := NewArray(NewString(false), true, true)

	if intArray.String() != "Array[Int]" {
		t.Errorf("Expected 'Array[Int]', got '%s'", intArray.String())
	}
	if stringArrayOptional.String() != "Array[String]?+" {
		t.Errorf("Expected 'Array[String]?+', got '%s'", stringArrayOptional.String())
	}

	if intArray.ItemType().String() != "Int" {
		t.Errorf("Expected item type 'Int', got '%s'", intArray.ItemType().String())
	}
}

func TestMapType(t *testing.T) {
	stringToInt := NewMap(NewString(false), NewInt(false), false)
	optionalMap := NewMap(NewString(false), NewInt(false), true)

	if stringToInt.String() != "Map[String,Int]" {
		t.Errorf("Expected 'Map[String,Int]', got '%s'", stringToInt.String())
	}
	if optionalMap.String() != "Map[String,Int]?" {
		t.Errorf("Expected 'Map[String,Int]?', got '%s'", optionalMap.String())
	}

	if stringToInt.KeyType().String() != "String" {
		t.Errorf("Expected key type 'String', got '%s'", stringToInt.KeyType().String())
	}
	if stringToInt.ValueType().String() != "Int" {
		t.Errorf("Expected value type 'Int', got '%s'", stringToInt.ValueType().String())
	}
}

func TestPairType(t *testing.T) {
	stringIntPair := NewPair(NewString(false), NewInt(false), false)
	optionalPair := NewPair(NewString(false), NewInt(false), true)

	if stringIntPair.String() != "Pair[String,Int]" {
		t.Errorf("Expected 'Pair[String,Int]', got '%s'", stringIntPair.String())
	}
	if optionalPair.String() != "Pair[String,Int]?" {
		t.Errorf("Expected 'Pair[String,Int]?', got '%s'", optionalPair.String())
	}

	if stringIntPair.LeftType().String() != "String" {
		t.Errorf("Expected left type 'String', got '%s'", stringIntPair.LeftType().String())
	}
	if stringIntPair.RightType().String() != "Int" {
		t.Errorf("Expected right type 'Int', got '%s'", stringIntPair.RightType().String())
	}
}

func TestTypeEquality(t *testing.T) {
	int1 := NewInt(false)
	int2 := NewInt(false)
	intOptional := NewInt(true)
	str1 := NewString(false)

	if !int1.Equal(int2) {
		t.Error("Expected equal Int types to be equal")
	}
	if !int1.Equal(intOptional) {
		t.Error("Expected Int and Int? to be equal (ignoring optional)")
	}
	if int1.Equal(str1) {
		t.Error("Expected Int and String to not be equal")
	}
}

func TestTypeCoercion(t *testing.T) {
	intType := NewInt(false)
	floatType := NewFloat(false)
	stringType := NewString(false)
	boolType := NewBoolean(false)
	fileType := NewFile(false)

	// Int coerces to Float
	if !intType.Coerces(floatType, true) {
		t.Error("Expected Int to coerce to Float")
	}

	// Int coerces to String
	if !intType.Coerces(stringType, true) {
		t.Error("Expected Int to coerce to String")
	}

	// Boolean coerces to String
	if !boolType.Coerces(stringType, true) {
		t.Error("Expected Boolean to coerce to String")
	}

	// Float coerces to String
	if !floatType.Coerces(stringType, true) {
		t.Error("Expected Float to coerce to String")
	}

	// File coerces to String
	if !fileType.Coerces(stringType, true) {
		t.Error("Expected File to coerce to String")
	}

	// String coerces to File
	if !stringType.Coerces(fileType, true) {
		t.Error("Expected String to coerce to File")
	}

	// String coerces to Int
	if !stringType.Coerces(intType, true) {
		t.Error("Expected String to coerce to Int")
	}

	// String coerces to Float
	if !stringType.Coerces(floatType, true) {
		t.Error("Expected String to coerce to Float")
	}
}

func TestOptionalCoercion(t *testing.T) {
	intType := NewInt(false)
	intOptional := NewInt(true)

	// T coerces to T?
	if !intType.Coerces(intOptional, true) {
		t.Error("Expected Int to coerce to Int?")
	}

	// T? does not coerce to T (with check_quant=true)
	if intOptional.Coerces(intType, true) {
		t.Error("Expected Int? not to coerce to Int with quantifier check")
	}

	// T? coerces to T (with check_quant=false)
	if !intOptional.Coerces(intType, false) {
		t.Error("Expected Int? to coerce to Int without quantifier check")
	}
}

func TestArrayCoercion(t *testing.T) {
	intType := NewInt(false)
	stringType := NewString(false)
	intArray := NewArray(intType, false, false)
	intArrayPlus := NewArray(intType, false, true)
	stringArray := NewArray(stringType, false, false)

	// Array[T]+ coerces to Array[T]
	if !intArrayPlus.Coerces(intArray, true) {
		t.Error("Expected Array[Int]+ to coerce to Array[Int]")
	}

	// Array[T] doesn't coerce to Array[T]+ (with check_quant=true)
	if intArray.Coerces(intArrayPlus, true) {
		t.Error("Expected Array[Int] not to coerce to Array[Int]+ with quantifier check")
	}

	// Array[T] coerces to Array[T]+ (with check_quant=false)
	if !intArray.Coerces(intArrayPlus, false) {
		t.Error("Expected Array[Int] to coerce to Array[Int]+ without quantifier check")
	}

	// Array[Int] doesn't coerce to Array[String]
	if intArray.Coerces(stringArray, true) {
		t.Error("Expected Array[Int] not to coerce to Array[String]")
	}

	// T coerces to Array[T] (with check_quant=false)
	if !intType.Coerces(intArray, false) {
		t.Error("Expected Int to coerce to Array[Int] without quantifier check")
	}

	// T doesn't coerce to Array[T] (with check_quant=true)
	if intType.Coerces(intArray, true) {
		t.Error("Expected Int not to coerce to Array[Int] with quantifier check")
	}
}

func TestArrayToStringCoercion(t *testing.T) {
	intType := NewInt(false)
	stringType := NewString(false)
	intArray := NewArray(intType, false, false)
	
	// Array[T] coerces to String if T coerces to String
	if !intArray.Coerces(stringType, true) {
		t.Error("Expected Array[Int] to coerce to String")
	}
}

func TestEquatable(t *testing.T) {
	intType := NewInt(false)
	intOptional := NewInt(true)
	floatType := NewFloat(false)
	stringType := NewString(false)
	anyType := NewAny(false, false)

	// Same types are equatable (ignoring optional)
	if !intType.Equatable(intOptional, false) {
		t.Error("Expected Int and Int? to be equatable")
	}

	// Int and Float are equatable at top level
	if !intType.Equatable(floatType, false) {
		t.Error("Expected Int and Float to be equatable")
	}

	// Any is equatable with everything
	if !intType.Equatable(anyType, false) {
		t.Error("Expected Int and Any to be equatable")
	}

	// Different types are not equatable
	if intType.Equatable(stringType, false) {
		t.Error("Expected Int and String not to be equatable")
	}
}

func TestComparable(t *testing.T) {
	intType := NewInt(false)
	intOptional := NewInt(true)
	floatType := NewFloat(false)
	stringType := NewString(false)
	boolType := NewBoolean(false)
	arrayType := NewArray(intType, false, false)

	// Int and Float are comparable
	if !intType.Comparable(floatType, true) {
		t.Error("Expected Int and Float to be comparable")
	}

	// Same primitive types are comparable
	if !stringType.Comparable(stringType, true) {
		t.Error("Expected String to be comparable with itself")
	}

	// Optional types are not comparable (with check_quant=true)
	if intOptional.Comparable(intType, true) {
		t.Error("Expected Int? not to be comparable with quantifier check")
	}

	// Optional types are comparable (with check_quant=false)
	if !intOptional.Comparable(intType, false) {
		t.Error("Expected Int? to be comparable without quantifier check")
	}

	// Composite types are not comparable
	if arrayType.Comparable(arrayType, true) {
		t.Error("Expected Array[Int] not to be comparable")
	}

	// Int and Boolean are not comparable
	if intType.Comparable(boolType, true) {
		t.Error("Expected Int and Boolean not to be comparable")
	}
}

func TestTypeCopy(t *testing.T) {
	intType := NewInt(false)
	
	// Copy with same optional
	intCopy := intType.Copy(nil)
	if intCopy.Optional() != false {
		t.Error("Expected copied type to have same optional setting")
	}
	
	// Copy with different optional
	intOptionalCopy := intType.Copy(&[]bool{true}[0])
	if intOptionalCopy.Optional() != true {
		t.Error("Expected copied type to have new optional setting")
	}
	
	// Original should be unchanged
	if intType.Optional() != false {
		t.Error("Expected original type to be unchanged")
	}
}

func TestUnifyFunction(t *testing.T) {
	intType := NewInt(false)
	floatType := NewFloat(false)
	stringType := NewString(false)
	anyType := NewAny(false, false)

	// Unify same types
	unified, err := Unify(intType, intType)
	if err != nil {
		t.Fatalf("Expected to unify Int with Int, got error: %v", err)
	}
	if unified.String() != "Int" {
		t.Errorf("Expected unified type 'Int', got '%s'", unified.String())
	}

	// Unify Int and Float -> Float
	unified, err = Unify(intType, floatType)
	if err != nil {
		t.Fatalf("Expected to unify Int with Float, got error: %v", err)
	}
	if unified.String() != "Float" {
		t.Errorf("Expected unified type 'Float', got '%s'", unified.String())
	}

	// Unify with Any
	unified, err = Unify(intType, anyType)
	if err != nil {
		t.Fatalf("Expected to unify Int with Any, got error: %v", err)
	}
	if unified.String() != "Int" {
		t.Errorf("Expected unified type 'Int', got '%s'", unified.String())
	}

	// Cannot unify incompatible types
	_, err = Unify(intType, stringType)
	if err == nil {
		t.Error("Expected error when unifying Int with String")
	}
}