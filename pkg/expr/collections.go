package expr

import (
	"fmt"
	"strings"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/types"
	"github.com/bioflowy/flowy/pkg/values"
)

// ArrayLiteral represents an array literal expression [expr1, expr2, ...]
type ArrayLiteral struct {
	baseExpr
	Items []Expr
}

// NewArrayLiteral creates a new array literal
func NewArrayLiteral(items []Expr, pos errors.SourcePosition) *ArrayLiteral {
	return &ArrayLiteral{
		baseExpr: NewBaseExpr(pos),
		Items:    items,
	}
}

func (a *ArrayLiteral) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	if len(a.Items) == 0 {
		// Empty array - type is Array[Any]
		return types.NewArray(types.NewAny(false, false), false, false), nil
	}

	// Infer types of all items and unify them
	var itemTypes []types.Base
	for _, item := range a.Items {
		itemType, err := item.InferType(typeEnv, stdlib)
		if err != nil {
			return nil, err
		}
		itemTypes = append(itemTypes, itemType)
	}

	// Unify all item types to get the common element type
	helper := InferTypeHelper{}
	elementType, err := helper.UnifyTypes(itemTypes, a.pos)
	if err != nil {
		return nil, err
	}

	return types.NewArray(elementType, false, false), nil
}

func (a *ArrayLiteral) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	// First infer the element type for the result array
	var elementType types.Base = types.NewAny(false, false)
	if len(a.Items) > 0 {
		// We need to infer the type to create the right array type
		arrayType, err := a.InferType(nil, stdlib) // Type env not needed for literals
		if err != nil {
			return nil, err
		}
		if at, ok := arrayType.(*types.ArrayType); ok {
			elementType = at.ItemType()
		}
	}

	// Create the array and evaluate each item
	result := values.NewArray(elementType, false, false)
	for _, item := range a.Items {
		val, err := item.Eval(valueEnv, stdlib)
		if err != nil {
			return nil, err
		}
		result.Add(val)
	}

	return result, nil
}

func (a *ArrayLiteral) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	arrayType, err := a.InferType(typeEnv, stdlib)
	if err != nil {
		return err
	}

	helper := TypeCheckHelper{}
	if err := helper.CheckCoercion(arrayType, expectedType, a.pos); err != nil {
		return err
	}

	// Type check each item
	var expectedItemType types.Base
	if expected, ok := expectedType.(*types.ArrayType); ok {
		expectedItemType = expected.ItemType()
	} else if inferred, ok := arrayType.(*types.ArrayType); ok {
		expectedItemType = inferred.ItemType()
	} else {
		expectedItemType = types.NewAny(false, false)
	}

	for _, item := range a.Items {
		if err := item.TypeCheck(expectedItemType, typeEnv, stdlib); err != nil {
			return err
		}
	}

	return nil
}

func (a *ArrayLiteral) Children() []Expr {
	return a.Items
}

func (a *ArrayLiteral) String() string {
	var items []string
	for _, item := range a.Items {
		items = append(items, item.String())
	}
	return "[" + strings.Join(items, ", ") + "]"
}

// PairLiteral represents a pair literal expression (expr1, expr2)
type PairLiteral struct {
	baseExpr
	Left  Expr
	Right Expr
}

// NewPairLiteral creates a new pair literal
func NewPairLiteral(left, right Expr, pos errors.SourcePosition) *PairLiteral {
	return &PairLiteral{
		baseExpr: NewBaseExpr(pos),
		Left:     left,
		Right:    right,
	}
}

func (p *PairLiteral) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	leftType, err := p.Left.InferType(typeEnv, stdlib)
	if err != nil {
		return nil, err
	}

	rightType, err := p.Right.InferType(typeEnv, stdlib)
	if err != nil {
		return nil, err
	}

	return types.NewPair(leftType, rightType, false), nil
}

func (p *PairLiteral) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	leftVal, err := p.Left.Eval(valueEnv, stdlib)
	if err != nil {
		return nil, err
	}

	rightVal, err := p.Right.Eval(valueEnv, stdlib)
	if err != nil {
		return nil, err
	}

	return values.NewPair(leftVal.Type(), rightVal.Type(), leftVal, rightVal, false), nil
}

func (p *PairLiteral) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	pairType, err := p.InferType(typeEnv, stdlib)
	if err != nil {
		return err
	}

	helper := TypeCheckHelper{}
	if err := helper.CheckCoercion(pairType, expectedType, p.pos); err != nil {
		return err
	}

	// Type check components
	if expected, ok := expectedType.(*types.PairType); ok {
		if err := p.Left.TypeCheck(expected.LeftType(), typeEnv, stdlib); err != nil {
			return err
		}
		if err := p.Right.TypeCheck(expected.RightType(), typeEnv, stdlib); err != nil {
			return err
		}
	}

	return nil
}

func (p *PairLiteral) Children() []Expr {
	return []Expr{p.Left, p.Right}
}

func (p *PairLiteral) String() string {
	return fmt.Sprintf("(%s, %s)", p.Left.String(), p.Right.String())
}

// MapLiteral represents a map literal expression {key: value, ...}
type MapLiteral struct {
	baseExpr
	Items []MapItem
}

// MapItem represents a key-value pair in a map literal
type MapItem struct {
	Key   Expr
	Value Expr
}

// NewMapLiteral creates a new map literal
func NewMapLiteral(items []MapItem, pos errors.SourcePosition) *MapLiteral {
	return &MapLiteral{
		baseExpr: NewBaseExpr(pos),
		Items:    items,
	}
}

func (m *MapLiteral) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	if len(m.Items) == 0 {
		// Empty map - type is Map[String, Any] by default
		return types.NewMap(types.NewString(false), types.NewAny(false, false), false), nil
	}

	// Infer types of all keys and values separately
	var keyTypes, valueTypes []types.Base
	for _, item := range m.Items {
		keyType, err := item.Key.InferType(typeEnv, stdlib)
		if err != nil {
			return nil, err
		}
		keyTypes = append(keyTypes, keyType)

		valueType, err := item.Value.InferType(typeEnv, stdlib)
		if err != nil {
			return nil, err
		}
		valueTypes = append(valueTypes, valueType)
	}

	// Unify key and value types
	helper := InferTypeHelper{}
	keyType, err := helper.UnifyTypes(keyTypes, m.pos)
	if err != nil {
		return nil, err
	}

	valueType, err := helper.UnifyTypes(valueTypes, m.pos)
	if err != nil {
		return nil, err
	}

	return types.NewMap(keyType, valueType, false), nil
}

func (m *MapLiteral) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	// Infer the key/value types for the result map
	var keyType types.Base = types.NewString(false)       // Default
	var valueType types.Base = types.NewAny(false, false) // Default

	if len(m.Items) > 0 {
		mapType, err := m.InferType(nil, stdlib)
		if err != nil {
			return nil, err
		}
		if mt, ok := mapType.(*types.MapType); ok {
			keyType = mt.KeyType()
			valueType = mt.ValueType()
		}
	}

	result := values.NewMap(keyType, valueType, false)

	for _, item := range m.Items {
		keyVal, err := item.Key.Eval(valueEnv, stdlib)
		if err != nil {
			return nil, err
		}

		valueVal, err := item.Value.Eval(valueEnv, stdlib)
		if err != nil {
			return nil, err
		}

		// Keys must be string-like for map access
		keyStr, err := keyVal.Coerce(types.NewString(false))
		if err != nil {
			return nil, errors.NewEvalError(nil, "map key must be string-coercible: "+err.Error())
		}

		keyString := keyStr.(*values.StringValue).Value().(string)
		result.Set(keyString, valueVal)
	}

	return result, nil
}

func (m *MapLiteral) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	mapType, err := m.InferType(typeEnv, stdlib)
	if err != nil {
		return err
	}

	helper := TypeCheckHelper{}
	if err := helper.CheckCoercion(mapType, expectedType, m.pos); err != nil {
		return err
	}

	// Type check each key-value pair
	var expectedKeyType, expectedValueType types.Base
	if expected, ok := expectedType.(*types.MapType); ok {
		expectedKeyType = expected.KeyType()
		expectedValueType = expected.ValueType()
	} else if inferred, ok := mapType.(*types.MapType); ok {
		expectedKeyType = inferred.KeyType()
		expectedValueType = inferred.ValueType()
	} else {
		expectedKeyType = types.NewString(false)
		expectedValueType = types.NewAny(false, false)
	}

	for _, item := range m.Items {
		if err := item.Key.TypeCheck(expectedKeyType, typeEnv, stdlib); err != nil {
			return err
		}
		if err := item.Value.TypeCheck(expectedValueType, typeEnv, stdlib); err != nil {
			return err
		}
	}

	return nil
}

func (m *MapLiteral) Children() []Expr {
	var children []Expr
	for _, item := range m.Items {
		children = append(children, item.Key, item.Value)
	}
	return children
}

func (m *MapLiteral) String() string {
	var items []string
	for _, item := range m.Items {
		items = append(items, fmt.Sprintf("%s: %s", item.Key.String(), item.Value.String()))
	}
	return "{" + strings.Join(items, ", ") + "}"
}

// StructLiteral represents a struct literal expression {member: value, ...}
type StructLiteral struct {
	baseExpr
	TypeName string
	Members  []StructMember
}

// StructMember represents a member in a struct literal
type StructMember struct {
	Name  string
	Value Expr
}

// NewStructLiteral creates a new struct literal
func NewStructLiteral(typeName string, members []StructMember, pos errors.SourcePosition) *StructLiteral {
	return &StructLiteral{
		baseExpr: NewBaseExpr(pos),
		TypeName: typeName,
		Members:  members,
	}
}

func (s *StructLiteral) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	// Build member types map
	memberTypes := make(map[string]types.Base)
	for _, member := range s.Members {
		memberType, err := member.Value.InferType(typeEnv, stdlib)
		if err != nil {
			return nil, err
		}
		memberTypes[member.Name] = memberType
	}

	return types.NewStructInstance(s.TypeName, memberTypes, false), nil
}

func (s *StructLiteral) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	// First infer the member types
	structType, err := s.InferType(nil, stdlib)
	if err != nil {
		return nil, err
	}

	memberTypes := structType.(*types.StructInstanceType).Members()
	memberValues := make(map[string]values.Base)

	for _, member := range s.Members {
		val, err := member.Value.Eval(valueEnv, stdlib)
		if err != nil {
			return nil, err
		}
		memberValues[member.Name] = val
	}

	return values.NewStruct(s.TypeName, memberTypes, memberValues, false), nil
}

func (s *StructLiteral) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	structType, err := s.InferType(typeEnv, stdlib)
	if err != nil {
		return err
	}

	helper := TypeCheckHelper{}
	if err := helper.CheckCoercion(structType, expectedType, s.pos); err != nil {
		return err
	}

	// Type check each member
	var expectedMemberTypes map[string]types.Base
	if expected, ok := expectedType.(*types.StructInstanceType); ok {
		expectedMemberTypes = expected.Members()
	} else if inferred, ok := structType.(*types.StructInstanceType); ok {
		expectedMemberTypes = inferred.Members()
	}

	for _, member := range s.Members {
		var expectedMemberType types.Base = types.NewAny(false, false)
		if expectedMemberTypes != nil {
			if t, ok := expectedMemberTypes[member.Name]; ok {
				expectedMemberType = t
			}
		}
		if err := member.Value.TypeCheck(expectedMemberType, typeEnv, stdlib); err != nil {
			return err
		}
	}

	return nil
}

func (s *StructLiteral) Children() []Expr {
	var children []Expr
	for _, member := range s.Members {
		children = append(children, member.Value)
	}
	return children
}

func (s *StructLiteral) String() string {
	var members []string
	for _, member := range s.Members {
		members = append(members, fmt.Sprintf("%s: %s", member.Name, member.Value.String()))
	}
	return fmt.Sprintf("%s{%s}", s.TypeName, strings.Join(members, ", "))
}
