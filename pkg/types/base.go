// Package types provides WDL data types
package types

import "fmt"

// Base represents the abstract base interface for all WDL types
type Base interface {
	// String returns the string representation of the type
	String() string
	
	// Optional returns true if the type has the optional quantifier (T?)
	Optional() bool
	
	// Copy creates a copy of the type, possibly with different optional setting
	Copy(optional *bool) Base
	
	// Equal checks if two types are equal (ignoring optional quantifier in some cases)
	Equal(other Base) bool
	
	// Coerces checks if this type can be coerced to the target type
	Coerces(target Base, checkQuant bool) bool
	
	// Check verifies this type can be coerced to target, raises error if not
	Check(target Base, checkQuant bool) error
	
	// Equatable checks if values of this type can be compared with == operator
	Equatable(other Base, compound bool) bool
	
	// Comparable checks if values of this type can be compared with <, >, etc.
	Comparable(other Base, checkQuant bool) bool
	
	// Parameters returns the type parameters (for composite types)
	Parameters() []Base
	
	// checkOptional performs optional quantifier checking
	checkOptional(target Base, checkQuant bool) error
}

// baseType provides common functionality for all types
type baseType struct {
	optional bool
}

func (b *baseType) Optional() bool {
	return b.optional
}

func (b *baseType) checkOptional(target Base, checkQuant bool) error {
	if checkQuant && b.optional && !target.Optional() && !isAny(target) {
		return fmt.Errorf("cannot coerce optional type to non-optional type")
	}
	return nil
}

func (b *baseType) Parameters() []Base {
	return []Base{}
}

func (b *baseType) Equal(other Base) bool {
	// This will be overridden by concrete types
	// Default implementation for embedded baseType
	return false
}

// Helper function to check if a type is Any
func isAny(t Base) bool {
	_, ok := t.(*AnyType)
	return ok
}

// AnyType represents a symbolic type that coerces to any other type
type AnyType struct {
	baseType
	isNull bool // true for None literals
}

// NewAny creates a new Any type
func NewAny(optional bool, isNull bool) *AnyType {
	return &AnyType{
		baseType: baseType{optional: isNull}, // True only for None literals
		isNull:   isNull,
	}
}

func (a *AnyType) String() string {
	if a.isNull {
		return "None"
	}
	return "Any"
}

func (a *AnyType) Copy(optional *bool) Base {
	result := &AnyType{
		baseType: a.baseType,
		isNull:   a.isNull,
	}
	if optional != nil {
		result.baseType.optional = *optional
	}
	return result
}

func (a *AnyType) Coerces(target Base, checkQuant bool) bool {
	return a.Check(target, checkQuant) == nil
}

func (a *AnyType) Check(target Base, checkQuant bool) error {
	return a.checkOptional(target, checkQuant)
}

func (a *AnyType) Equatable(other Base, compound bool) bool {
	return true // Any is equatable with everything
}

func (a *AnyType) Equal(other Base) bool {
	// Any equals Any, but not specific types
	_, ok := other.(*AnyType)
	return ok
}

func (a *AnyType) Comparable(other Base, checkQuant bool) bool {
	// Any is not comparable in the traditional sense
	return false
}