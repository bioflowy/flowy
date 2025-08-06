package types

import "fmt"

// Unify attempts to find a common type for two given types
// This is used for type inference in expressions like conditionals
func Unify(left Base, right Base) (Base, error) {
	// If either is Any, return the other
	if leftAny, ok := left.(*AnyType); ok {
		if leftAny.isNull {
			// None literal - return right type made optional
			return right.Copy(&[]bool{true}[0]), nil
		}
		return right, nil
	}
	if rightAny, ok := right.(*AnyType); ok {
		if rightAny.isNull {
			// None literal - return left type made optional
			return left.Copy(&[]bool{true}[0]), nil
		}
		return left, nil
	}

	// Same types unify to themselves
	if left.String() == right.String() {
		return left, nil
	}

	// Int and Float unify to Float
	if isInt(left) && isFloat(right) {
		optional := left.Optional() || right.Optional()
		return NewFloat(optional), nil
	}
	if isFloat(left) && isInt(right) {
		optional := left.Optional() || right.Optional()
		return NewFloat(optional), nil
	}

	// Array unification
	if leftArray, ok := left.(*ArrayType); ok {
		if rightArray, ok := right.(*ArrayType); ok {
			unified, err := Unify(leftArray.itemType, rightArray.itemType)
			if err != nil {
				return nil, err
			}
			optional := leftArray.Optional() || rightArray.Optional()
			nonempty := leftArray.nonempty && rightArray.nonempty
			return NewArray(unified, optional, nonempty), nil
		}
	}

	// Map unification
	if leftMap, ok := left.(*MapType); ok {
		if rightMap, ok := right.(*MapType); ok {
			keyUnified, err := Unify(leftMap.keyType, rightMap.keyType)
			if err != nil {
				return nil, err
			}
			valueUnified, err := Unify(leftMap.valueType, rightMap.valueType)
			if err != nil {
				return nil, err
			}
			optional := leftMap.Optional() || rightMap.Optional()
			return NewMap(keyUnified, valueUnified, optional), nil
		}
	}

	// Pair unification
	if leftPair, ok := left.(*PairType); ok {
		if rightPair, ok := right.(*PairType); ok {
			leftUnified, err := Unify(leftPair.leftType, rightPair.leftType)
			if err != nil {
				return nil, err
			}
			rightUnified, err := Unify(leftPair.rightType, rightPair.rightType)
			if err != nil {
				return nil, err
			}
			optional := leftPair.Optional() || rightPair.Optional()
			return NewPair(leftUnified, rightUnified, optional), nil
		}
	}

	// Struct unification - only if same type ID
	if leftStruct, ok := left.(*StructInstanceType); ok {
		if rightStruct, ok := right.(*StructInstanceType); ok {
			if leftStruct.typeID == rightStruct.typeID {
				optional := leftStruct.Optional() || rightStruct.Optional()
				return leftStruct.Copy(&optional), nil
			}
		}
	}

	return nil, fmt.Errorf("cannot unify %s with %s", left.String(), right.String())
}

// Helper functions for type checking
func isInt(t Base) bool {
	_, ok := t.(*IntType)
	return ok
}

func isFloat(t Base) bool {
	_, ok := t.(*FloatType)
	return ok
}

func isString(t Base) bool {
	_, ok := t.(*StringType)
	return ok
}

func isBool(t Base) bool {
	_, ok := t.(*BooleanType)
	return ok
}

func isFile(t Base) bool {
	_, ok := t.(*FileType)
	return ok
}

func isDirectory(t Base) bool {
	_, ok := t.(*DirectoryType)
	return ok
}

func isArray(t Base) bool {
	_, ok := t.(*ArrayType)
	return ok
}

func isMap(t Base) bool {
	_, ok := t.(*MapType)
	return ok
}

func isPair(t Base) bool {
	_, ok := t.(*PairType)
	return ok
}

func isStruct(t Base) bool {
	_, ok := t.(*StructInstanceType)
	return ok
}

func isObject(t Base) bool {
	_, ok := t.(*ObjectType)
	return ok
}
