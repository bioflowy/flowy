package expr

import (
	"fmt"
	"strings"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/types"
	"github.com/bioflowy/flowy/pkg/values"
)

// Identifier represents a variable reference expression
type Identifier struct {
	baseExpr
	Name      string
	Namespace []string // For namespaced identifiers like call.output
}

// NewIdentifier creates a new identifier expression
func NewIdentifier(name string, pos errors.SourcePosition) *Identifier {
	return &Identifier{
		baseExpr:  NewBaseExpr(pos),
		Name:      name,
		Namespace: nil,
	}
}

// NewNamespacedIdentifier creates a new namespaced identifier expression
func NewNamespacedIdentifier(namespace []string, name string, pos errors.SourcePosition) *Identifier {
	return &Identifier{
		baseExpr:  NewBaseExpr(pos),
		Name:      name,
		Namespace: namespace,
	}
}

func (i *Identifier) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	if typeEnv == nil {
		return nil, &errors.UnknownIdentifier{
			ValidationError: errors.NewValidationErrorFromPos(i.pos, fmt.Sprintf("unknown identifier: %s", i.fullName())),
		}
	}

	// Look up the identifier in the type environment
	var fullName string
	if len(i.Namespace) == 0 {
		fullName = i.Name
	} else {
		fullName = strings.Join(i.Namespace, ".") + "." + i.Name
	}

	value, err := typeEnv.Resolve(fullName)
	if err != nil {
		return nil, &errors.UnknownIdentifier{
			ValidationError: errors.NewValidationErrorFromPos(i.pos, fmt.Sprintf("unknown identifier: %s", i.fullName())),
		}
	}

	return value, nil
}

func (i *Identifier) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	if valueEnv == nil {
		return nil, &errors.UnknownIdentifier{
			ValidationError: errors.NewValidationErrorFromPos(i.pos, fmt.Sprintf("unknown identifier: %s", i.fullName())),
		}
	}

	// Look up the identifier in the value environment
	var fullName string
	if len(i.Namespace) == 0 {
		fullName = i.Name
	} else {
		fullName = strings.Join(i.Namespace, ".") + "." + i.Name
	}

	value, err := valueEnv.Resolve(fullName)
	if err != nil {
		return nil, &errors.UnknownIdentifier{
			ValidationError: errors.NewValidationErrorFromPos(i.pos, fmt.Sprintf("unknown identifier: %s", i.fullName())),
		}
	}

	return value, nil
}

func (i *Identifier) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	identType, err := i.InferType(typeEnv, stdlib)
	if err != nil {
		return err
	}

	helper := TypeCheckHelper{}
	return helper.CheckCoercion(identType, expectedType, i.pos)
}

func (i *Identifier) fullName() string {
	if len(i.Namespace) == 0 {
		return i.Name
	}
	return strings.Join(i.Namespace, ".") + "." + i.Name
}

func (i *Identifier) String() string {
	return i.fullName()
}

// GetAttr represents member access expression (obj.member)
type GetAttr struct {
	baseExpr
	Object Expr
	Attr   string
}

// NewGetAttr creates a new member access expression
func NewGetAttr(object Expr, attr string, pos errors.SourcePosition) *GetAttr {
	return &GetAttr{
		baseExpr: NewBaseExpr(pos),
		Object:   object,
		Attr:     attr,
	}
}

func (g *GetAttr) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	objectType, err := g.Object.InferType(typeEnv, stdlib)
	if err != nil {
		return nil, err
	}

	// Handle different object types
	switch obj := objectType.(type) {
	case *types.StructInstanceType:
		// Struct member access
		members := obj.Members()
		if memberType, ok := members[g.Attr]; ok {
			return memberType, nil
		}
		return nil, &errors.NoSuchMember{
			ValidationError: errors.NewValidationErrorFromPos(g.pos, fmt.Sprintf("no such member: %s", g.Attr)),
		}

	case *types.PairType:
		// Pair member access (left, right)
		switch g.Attr {
		case "left":
			return obj.LeftType(), nil
		case "right":
			return obj.RightType(), nil
		default:
			return nil, &errors.NoSuchMember{
				ValidationError: errors.NewValidationErrorFromPos(g.pos, fmt.Sprintf("no such member: %s", g.Attr)),
			}
		}

	default:
		return nil, &errors.InvalidType{
			ValidationError: errors.NewValidationErrorFromPos(g.pos, fmt.Sprintf("not an object: %s", objectType.String())),
		}
	}
}

func (g *GetAttr) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	objectValue, err := g.Object.Eval(valueEnv, stdlib)
	if err != nil {
		return nil, err
	}

	// Handle different value types
	switch obj := objectValue.(type) {
	case *values.StructValue:
		// Struct member access
		if value, ok := obj.Get(g.Attr); ok {
			return value, nil
		}
		return nil, &errors.NoSuchMember{
			ValidationError: errors.NewValidationErrorFromPos(g.pos, fmt.Sprintf("no such member: %s", g.Attr)),
		}

	case *values.PairValue:
		// Pair member access
		switch g.Attr {
		case "left":
			return obj.Left(), nil
		case "right":
			return obj.Right(), nil
		default:
			return nil, &errors.NoSuchMember{
				ValidationError: errors.NewValidationErrorFromPos(g.pos, fmt.Sprintf("no such member: %s", g.Attr)),
			}
		}

	case *values.ObjectValue:
		// Legacy object member access
		if value, ok := obj.Get(g.Attr); ok {
			return value, nil
		}
		return nil, &errors.NoSuchMember{
			ValidationError: errors.NewValidationErrorFromPos(g.pos, fmt.Sprintf("no such member: %s", g.Attr)),
		}

	default:
		return nil, &errors.InvalidType{
			ValidationError: errors.NewValidationErrorFromPos(g.pos, fmt.Sprintf("not an object: %s", objectValue.Type().String())),
		}
	}
}

func (g *GetAttr) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	attrType, err := g.InferType(typeEnv, stdlib)
	if err != nil {
		return err
	}

	helper := TypeCheckHelper{}
	return helper.CheckCoercion(attrType, expectedType, g.pos)
}

func (g *GetAttr) Children() []Expr {
	return []Expr{g.Object}
}

func (g *GetAttr) String() string {
	return fmt.Sprintf("%s.%s", g.Object.String(), g.Attr)
}

// GetIndex represents indexed access expression (obj[index])
type GetIndex struct {
	baseExpr
	Object Expr
	Index  Expr
}

// NewGetIndex creates a new indexed access expression
func NewGetIndex(object, index Expr, pos errors.SourcePosition) *GetIndex {
	return &GetIndex{
		baseExpr: NewBaseExpr(pos),
		Object:   object,
		Index:    index,
	}
}

func (g *GetIndex) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	objectType, err := g.Object.InferType(typeEnv, stdlib)
	if err != nil {
		return nil, err
	}

	indexType, err := g.Index.InferType(typeEnv, stdlib)
	if err != nil {
		return nil, err
	}

	// Handle different collection types
	switch obj := objectType.(type) {
	case *types.ArrayType:
		// Array indexing - index must be Int
		if err := indexType.Check(types.NewInt(false), true); err != nil {
			return nil, &errors.InvalidType{
				ValidationError: errors.NewValidationErrorFromPos(g.pos, fmt.Sprintf("type mismatch: expected %s, got %s", "Int", indexType.String())),
			}
		}
		return obj.ItemType(), nil

	case *types.MapType:
		// Map lookup - index must coerce to key type
		if err := indexType.Check(obj.KeyType(), true); err != nil {
			return nil, &errors.InvalidType{
				ValidationError: errors.NewValidationErrorFromPos(g.pos, fmt.Sprintf("type mismatch: expected %s, got %s", obj.KeyType().String(), indexType.String())),
			}
		}
		return obj.ValueType(), nil

	default:
		return nil, &errors.InvalidType{
			ValidationError: errors.NewValidationErrorFromPos(g.pos, fmt.Sprintf("not an object: %s", objectType.String())),
		}
	}
}

func (g *GetIndex) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	objectValue, err := g.Object.Eval(valueEnv, stdlib)
	if err != nil {
		return nil, err
	}

	indexValue, err := g.Index.Eval(valueEnv, stdlib)
	if err != nil {
		return nil, err
	}

	// Handle different collection types
	switch obj := objectValue.(type) {
	case *values.ArrayValue:
		// Array indexing
		intIndex, err := indexValue.Coerce(types.NewInt(false))
		if err != nil {
			return nil, errors.NewEvalErrorFromPos(g.pos, "array index must be integer: "+err.Error())
		}

		idx := intIndex.(*values.IntValue).Value().(int64)
		items := obj.Items()

		if idx < 0 || idx >= int64(len(items)) {
			return nil, errors.NewEvalErrorFromPos(g.pos, fmt.Sprintf("array index out of bounds: %d", idx))
		}

		return items[idx], nil

	case *values.MapValue:
		// Map lookup
		keyValue, err := indexValue.Coerce(types.NewString(false))
		if err != nil {
			return nil, errors.NewEvalErrorFromPos(g.pos, "map key must be string: "+err.Error())
		}

		key := keyValue.(*values.StringValue).Value().(string)
		if value, ok := obj.Get(key); ok {
			return value, nil
		}

		return nil, errors.NewEvalErrorFromPos(g.pos, fmt.Sprintf("key not found in map: %s", key))

	default:
		return nil, &errors.InvalidType{
			ValidationError: errors.NewValidationErrorFromPos(g.pos, fmt.Sprintf("not an object: %s", objectValue.Type().String())),
		}
	}
}

func (g *GetIndex) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	indexType, err := g.InferType(typeEnv, stdlib)
	if err != nil {
		return err
	}

	helper := TypeCheckHelper{}
	return helper.CheckCoercion(indexType, expectedType, g.pos)
}

func (g *GetIndex) Children() []Expr {
	return []Expr{g.Object, g.Index}
}

func (g *GetIndex) String() string {
	return fmt.Sprintf("%s[%s]", g.Object.String(), g.Index.String())
}

// Slice represents array slicing expression (array[start:end])
type Slice struct {
	baseExpr
	Array Expr
	Start Expr // Can be nil for [:end]
	End   Expr // Can be nil for [start:]
}

// NewSlice creates a new slice expression
func NewSlice(array, start, end Expr, pos errors.SourcePosition) *Slice {
	return &Slice{
		baseExpr: NewBaseExpr(pos),
		Array:    array,
		Start:    start,
		End:      end,
	}
}

func (s *Slice) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	arrayType, err := s.Array.InferType(typeEnv, stdlib)
	if err != nil {
		return nil, err
	}

	// Must be an array type
	if _, ok := arrayType.(*types.ArrayType); !ok {
		return nil, &errors.InvalidType{
			ValidationError: errors.NewValidationErrorFromPos(s.pos, fmt.Sprintf("type mismatch: expected %s, got %s", "Array[T]", arrayType.String())),
		}
	}

	// Check index types if present
	if s.Start != nil {
		startType, err := s.Start.InferType(typeEnv, stdlib)
		if err != nil {
			return nil, err
		}
		if err := startType.Check(types.NewInt(false), true); err != nil {
			return nil, &errors.InvalidType{
				ValidationError: errors.NewValidationErrorFromPos(s.pos, fmt.Sprintf("type mismatch: expected %s, got %s", "Int", startType.String())),
			}
		}
	}

	if s.End != nil {
		endType, err := s.End.InferType(typeEnv, stdlib)
		if err != nil {
			return nil, err
		}
		if err := endType.Check(types.NewInt(false), true); err != nil {
			return nil, &errors.InvalidType{
				ValidationError: errors.NewValidationErrorFromPos(s.pos, fmt.Sprintf("type mismatch: expected %s, got %s", "Int", endType.String())),
			}
		}
	}

	// Result is same array type
	return arrayType, nil
}

func (s *Slice) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	arrayValue, err := s.Array.Eval(valueEnv, stdlib)
	if err != nil {
		return nil, err
	}

	arrayVal, ok := arrayValue.(*values.ArrayValue)
	if !ok {
		return nil, errors.NewEvalErrorFromPos(s.pos, "slice requires array value")
	}

	items := arrayVal.Items()
	length := int64(len(items))

	// Evaluate indices
	startIdx := int64(0)
	if s.Start != nil {
		startValue, err := s.Start.Eval(valueEnv, stdlib)
		if err != nil {
			return nil, err
		}
		startInt, err := startValue.Coerce(types.NewInt(false))
		if err != nil {
			return nil, errors.NewEvalErrorFromPos(s.pos, "slice start must be integer: "+err.Error())
		}
		startIdx = startInt.(*values.IntValue).Value().(int64)
	}

	endIdx := length
	if s.End != nil {
		endValue, err := s.End.Eval(valueEnv, stdlib)
		if err != nil {
			return nil, err
		}
		endInt, err := endValue.Coerce(types.NewInt(false))
		if err != nil {
			return nil, errors.NewEvalErrorFromPos(s.pos, "slice end must be integer: "+err.Error())
		}
		endIdx = endInt.(*values.IntValue).Value().(int64)
	}

	// Bounds checking
	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx > length {
		endIdx = length
	}
	if startIdx > endIdx {
		startIdx = endIdx
	}

	// Create sliced array
	arrayType := arrayVal.Type().(*types.ArrayType)
	result := values.NewArray(arrayType.ItemType(), arrayType.Optional(), false) // Sliced array is not necessarily non-empty

	for i := startIdx; i < endIdx; i++ {
		result.Add(items[i])
	}

	return result, nil
}

func (s *Slice) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	sliceType, err := s.InferType(typeEnv, stdlib)
	if err != nil {
		return err
	}

	helper := TypeCheckHelper{}
	return helper.CheckCoercion(sliceType, expectedType, s.pos)
}

func (s *Slice) Children() []Expr {
	children := []Expr{s.Array}
	if s.Start != nil {
		children = append(children, s.Start)
	}
	if s.End != nil {
		children = append(children, s.End)
	}
	return children
}

func (s *Slice) String() string {
	startStr := ""
	if s.Start != nil {
		startStr = s.Start.String()
	}
	endStr := ""
	if s.End != nil {
		endStr = s.End.String()
	}
	return fmt.Sprintf("%s[%s:%s]", s.Array.String(), startStr, endStr)
}
