package values

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bioflowy/flowy/pkg/types"
)

// ArrayValue represents a WDL Array value
type ArrayValue struct {
	baseValue
	items []Base
}

// NewArray creates a new Array value
func NewArray(itemType types.Base, optional bool, nonempty bool) *ArrayValue {
	return &ArrayValue{
		baseValue: baseValue{typ: types.NewArray(itemType, optional, nonempty)},
		items:     []Base{},
	}
}

// NewArrayWithItems creates a new Array value with initial items
func NewArrayWithItems(itemType types.Base, items []Base, optional bool, nonempty bool) *ArrayValue {
	return &ArrayValue{
		baseValue: baseValue{typ: types.NewArray(itemType, optional, nonempty)},
		items:     items,
	}
}

func (a *ArrayValue) Value() interface{} {
	result := make([]interface{}, len(a.items))
	for i, item := range a.items {
		result[i] = item.Value()
	}
	return result
}

func (a *ArrayValue) JSON() json.RawMessage {
	jsonItems := make([]json.RawMessage, len(a.items))
	for i, item := range a.items {
		jsonItems[i] = item.JSON()
	}
	data, _ := json.Marshal(jsonItems)
	return json.RawMessage(data)
}

func (a *ArrayValue) Equal(other Base) bool {
	if otherArray, ok := other.(*ArrayValue); ok {
		if len(a.items) != len(otherArray.items) {
			return false
		}
		for i := range a.items {
			if !a.items[i].Equal(otherArray.items[i]) {
				return false
			}
		}
		return true
	}
	return false
}

func (a *ArrayValue) String() string {
	parts := make([]string, len(a.items))
	for i, item := range a.items {
		parts[i] = item.String()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func (a *ArrayValue) Coerce(targetType types.Base) (Base, error) {
	switch target := targetType.(type) {
	case *types.ArrayType:
		// Check if item types are compatible
		if arrayType, ok := a.typ.(*types.ArrayType); ok {
			if err := arrayType.ItemType().Check(target.ItemType(), true); err != nil {
				return nil, fmt.Errorf("array item types incompatible: %v", err)
			}
		}
		// Coerce each item if needed
		coercedItems := make([]Base, len(a.items))
		for i, item := range a.items {
			coerced, err := item.Coerce(target.ItemType())
			if err != nil {
				return nil, fmt.Errorf("cannot coerce array item %d: %v", i, err)
			}
			coercedItems[i] = coerced
		}
		return NewArrayWithItems(target.ItemType(), coercedItems, target.Optional(), target.NonEmpty()), nil
	case *types.StringType:
		// Array[T] coerces to String if T coerces to String
		parts := make([]string, len(a.items))
		for i, item := range a.items {
			coerced, err := item.Coerce(types.NewString(false))
			if err != nil {
				return nil, fmt.Errorf("cannot coerce array item %d to String: %v", i, err)
			}
			parts[i] = coerced.String()
		}
		return NewString(strings.Join(parts, ""), target.Optional()), nil
	default:
		return nil, fmt.Errorf("cannot coerce Array to %s", targetType.String())
	}
}

// Items returns the array items
func (a *ArrayValue) Items() []Base {
	return a.items
}

// Add adds an item to the array
func (a *ArrayValue) Add(item Base) {
	a.items = append(a.items, item)
}

// MapValue represents a WDL Map value
type MapValue struct {
	baseValue
	entries map[string]Base
}

// NewMap creates a new Map value
func NewMap(keyType types.Base, valueType types.Base, optional bool) *MapValue {
	return &MapValue{
		baseValue: baseValue{typ: types.NewMap(keyType, valueType, optional)},
		entries:   make(map[string]Base),
	}
}

// NewMapWithEntries creates a new Map value with initial entries
func NewMapWithEntries(keyType types.Base, valueType types.Base, entries map[string]Base, optional bool) *MapValue {
	return &MapValue{
		baseValue: baseValue{typ: types.NewMap(keyType, valueType, optional)},
		entries:   entries,
	}
}

func (m *MapValue) Value() interface{} {
	result := make(map[string]interface{})
	for k, v := range m.entries {
		result[k] = v.Value()
	}
	return result
}

func (m *MapValue) JSON() json.RawMessage {
	jsonMap := make(map[string]json.RawMessage)
	for k, v := range m.entries {
		jsonMap[k] = v.JSON()
	}
	data, _ := json.Marshal(jsonMap)
	return json.RawMessage(data)
}

func (m *MapValue) Equal(other Base) bool {
	if otherMap, ok := other.(*MapValue); ok {
		if len(m.entries) != len(otherMap.entries) {
			return false
		}
		for k, v := range m.entries {
			if otherV, ok := otherMap.entries[k]; !ok || !v.Equal(otherV) {
				return false
			}
		}
		return true
	}
	return false
}

func (m *MapValue) String() string {
	parts := []string{}
	for k, v := range m.entries {
		parts = append(parts, fmt.Sprintf("%s: %s", k, v.String()))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func (m *MapValue) Coerce(targetType types.Base) (Base, error) {
	switch target := targetType.(type) {
	case *types.MapType:
		// Check if key and value types are compatible
		if mapType, ok := m.typ.(*types.MapType); ok {
			if err := mapType.KeyType().Check(target.KeyType(), true); err != nil {
				return nil, fmt.Errorf("map key types incompatible: %v", err)
			}
			if err := mapType.ValueType().Check(target.ValueType(), true); err != nil {
				return nil, fmt.Errorf("map value types incompatible: %v", err)
			}
		}
		// Coerce each entry if needed
		coercedEntries := make(map[string]Base)
		for k, v := range m.entries {
			coerced, err := v.Coerce(target.ValueType())
			if err != nil {
				return nil, fmt.Errorf("cannot coerce map value for key %s: %v", k, err)
			}
			coercedEntries[k] = coerced
		}
		return NewMapWithEntries(target.KeyType(), target.ValueType(), coercedEntries, target.Optional()), nil
	default:
		return nil, fmt.Errorf("cannot coerce Map to %s", targetType.String())
	}
}

// Get returns the value for the given key
func (m *MapValue) Get(key string) (Base, bool) {
	val, ok := m.entries[key]
	return val, ok
}

// Set sets the value for the given key
func (m *MapValue) Set(key string, value Base) {
	m.entries[key] = value
}

// Entries returns all map entries
func (m *MapValue) Entries() map[string]Base {
	return m.entries
}

// PairValue represents a WDL Pair value
type PairValue struct {
	baseValue
	left  Base
	right Base
}

// NewPair creates a new Pair value
func NewPair(leftType types.Base, rightType types.Base, left Base, right Base, optional bool) *PairValue {
	return &PairValue{
		baseValue: baseValue{typ: types.NewPair(leftType, rightType, optional)},
		left:      left,
		right:     right,
	}
}

func (p *PairValue) Value() interface{} {
	return []interface{}{p.left.Value(), p.right.Value()}
}

func (p *PairValue) JSON() json.RawMessage {
	data, _ := json.Marshal([]json.RawMessage{p.left.JSON(), p.right.JSON()})
	return json.RawMessage(data)
}

func (p *PairValue) Equal(other Base) bool {
	if otherPair, ok := other.(*PairValue); ok {
		return p.left.Equal(otherPair.left) && p.right.Equal(otherPair.right)
	}
	return false
}

func (p *PairValue) String() string {
	return fmt.Sprintf("(%s, %s)", p.left.String(), p.right.String())
}

func (p *PairValue) Coerce(targetType types.Base) (Base, error) {
	switch target := targetType.(type) {
	case *types.PairType:
		// Coerce left and right values
		coercedLeft, err := p.left.Coerce(target.LeftType())
		if err != nil {
			return nil, fmt.Errorf("cannot coerce pair left value: %v", err)
		}
		coercedRight, err := p.right.Coerce(target.RightType())
		if err != nil {
			return nil, fmt.Errorf("cannot coerce pair right value: %v", err)
		}
		return NewPair(target.LeftType(), target.RightType(), coercedLeft, coercedRight, target.Optional()), nil
	default:
		return nil, fmt.Errorf("cannot coerce Pair to %s", targetType.String())
	}
}

// Left returns the left value of the pair
func (p *PairValue) Left() Base {
	return p.left
}

// Right returns the right value of the pair
func (p *PairValue) Right() Base {
	return p.right
}

// StructValue represents a WDL struct instance value
type StructValue struct {
	baseValue
	typeName string
	members  map[string]Base
}

// NewStruct creates a new Struct value
func NewStruct(typeName string, memberTypes map[string]types.Base, members map[string]Base, optional bool) *StructValue {
	return &StructValue{
		baseValue: baseValue{typ: types.NewStructInstance(typeName, memberTypes, optional)},
		typeName:  typeName,
		members:   members,
	}
}

func (s *StructValue) Value() interface{} {
	result := make(map[string]interface{})
	for k, v := range s.members {
		result[k] = v.Value()
	}
	return result
}

func (s *StructValue) JSON() json.RawMessage {
	jsonMap := make(map[string]json.RawMessage)
	for k, v := range s.members {
		jsonMap[k] = v.JSON()
	}
	data, _ := json.Marshal(jsonMap)
	return json.RawMessage(data)
}

func (s *StructValue) Equal(other Base) bool {
	if otherStruct, ok := other.(*StructValue); ok {
		if s.typeName != otherStruct.typeName {
			return false
		}
		if len(s.members) != len(otherStruct.members) {
			return false
		}
		for k, v := range s.members {
			if otherV, ok := otherStruct.members[k]; !ok || !v.Equal(otherV) {
				return false
			}
		}
		return true
	}
	return false
}

func (s *StructValue) String() string {
	parts := []string{}
	for k, v := range s.members {
		parts = append(parts, fmt.Sprintf("%s: %s", k, v.String()))
	}
	return fmt.Sprintf("%s{%s}", s.typeName, strings.Join(parts, ", "))
}

func (s *StructValue) Coerce(targetType types.Base) (Base, error) {
	switch target := targetType.(type) {
	case *types.StructInstanceType:
		// Check if struct types match
		if s.typeName != target.TypeName() {
			return nil, fmt.Errorf("cannot coerce struct %s to %s", s.typeName, target.TypeName())
		}
		return s, nil
	default:
		return nil, fmt.Errorf("cannot coerce Struct to %s", targetType.String())
	}
}

// Get returns the value for the given member
func (s *StructValue) Get(member string) (Base, bool) {
	val, ok := s.members[member]
	return val, ok
}

// Members returns all struct members
func (s *StructValue) Members() map[string]Base {
	return s.members
}

// ObjectValue represents a WDL Object value (legacy)
type ObjectValue struct {
	baseValue
	members map[string]Base
}

// NewObject creates a new Object value
func NewObject(members map[string]Base, optional bool) *ObjectValue {
	return &ObjectValue{
		baseValue: baseValue{typ: types.NewObject(optional)},
		members:   members,
	}
}

func (o *ObjectValue) Value() interface{} {
	result := make(map[string]interface{})
	for k, v := range o.members {
		result[k] = v.Value()
	}
	return result
}

func (o *ObjectValue) JSON() json.RawMessage {
	jsonMap := make(map[string]json.RawMessage)
	for k, v := range o.members {
		jsonMap[k] = v.JSON()
	}
	data, _ := json.Marshal(jsonMap)
	return json.RawMessage(data)
}

func (o *ObjectValue) Equal(other Base) bool {
	if otherObject, ok := other.(*ObjectValue); ok {
		if len(o.members) != len(otherObject.members) {
			return false
		}
		for k, v := range o.members {
			if otherV, ok := otherObject.members[k]; !ok || !v.Equal(otherV) {
				return false
			}
		}
		return true
	}
	return false
}

func (o *ObjectValue) String() string {
	parts := []string{}
	for k, v := range o.members {
		parts = append(parts, fmt.Sprintf("%s: %s", k, v.String()))
	}
	return "object{" + strings.Join(parts, ", ") + "}"
}

func (o *ObjectValue) Coerce(targetType types.Base) (Base, error) {
	switch targetType.(type) {
	case *types.ObjectType:
		return o, nil
	default:
		return nil, fmt.Errorf("cannot coerce Object to %s", targetType.String())
	}
}

// Get returns the value for the given member
func (o *ObjectValue) Get(member string) (Base, bool) {
	val, ok := o.members[member]
	return val, ok
}

// Members returns all object members
func (o *ObjectValue) Members() map[string]Base {
	return o.members
}