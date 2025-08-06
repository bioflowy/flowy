package types

import (
	"fmt"
	"strings"
)

// ArrayType represents the WDL Array type
type ArrayType struct {
	baseType
	itemType   Base
	nonempty   bool // Array[T]+ if true
}

func NewArray(itemType Base, optional bool, nonempty bool) *ArrayType {
	return &ArrayType{
		baseType: baseType{optional: optional},
		itemType: itemType,
		nonempty: nonempty,
	}
}

func (a *ArrayType) String() string {
	result := fmt.Sprintf("Array[%s]", a.itemType.String())
	if a.optional {
		result += "?"
	}
	if a.nonempty {
		result += "+"
	}
	return result
}

func (a *ArrayType) Copy(optional *bool) Base {
	result := &ArrayType{
		baseType: a.baseType,
		itemType: a.itemType,
		nonempty: a.nonempty,
	}
	if optional != nil {
		result.baseType.optional = *optional
	}
	return result
}

func (a *ArrayType) ItemType() Base {
	return a.itemType
}

func (a *ArrayType) NonEmpty() bool {
	return a.nonempty
}

func (a *ArrayType) Parameters() []Base {
	return []Base{a.itemType}
}

func (a *ArrayType) Coerces(target Base, checkQuant bool) bool {
	return a.Check(target, checkQuant) == nil
}

func (a *ArrayType) Check(target Base, checkQuant bool) error {
	if targetArray, ok := target.(*ArrayType); ok {
		// Check array compatibility - Array[T]+ can coerce to Array[T], but not vice versa when checkQuant=true
		if !a.nonempty && targetArray.nonempty && checkQuant {
			return fmt.Errorf("cannot coerce Array[T] to Array[T]+")
		}
		// Check item type compatibility - must be exact match for arrays
		if !a.itemType.Equal(targetArray.itemType) {
			return fmt.Errorf("array item types do not match")
		}
		return a.checkOptional(target, checkQuant)
	}
	
	// Array[T] coerces to String if T coerces to String
	if _, ok := target.(*StringType); ok {
		if err := a.itemType.Check(NewString(false), checkQuant); err != nil {
			return fmt.Errorf("Array[%s] cannot coerce to String", a.itemType.String())
		}
		return a.checkOptional(target, checkQuant)
	}
	
	if isAny(target) {
		return a.checkOptional(target, checkQuant)
	}
	
	return fmt.Errorf("cannot coerce %s to %s", a.String(), target.String())
}

func (a *ArrayType) Equatable(other Base, compound bool) bool {
	if otherArray, ok := other.(*ArrayType); ok {
		return a.itemType.Equatable(otherArray.itemType, true)
	}
	return isAny(other)
}

func (a *ArrayType) Equal(other Base) bool {
	if otherArray, ok := other.(*ArrayType); ok {
		return a.itemType.Equal(otherArray.itemType) && a.nonempty == otherArray.nonempty
	}
	return false
}

func (a *ArrayType) Comparable(other Base, checkQuant bool) bool {
	return false // Arrays are not comparable
}

// MapType represents the WDL Map type
type MapType struct {
	baseType
	keyType   Base
	valueType Base
}

func NewMap(keyType Base, valueType Base, optional bool) *MapType {
	return &MapType{
		baseType:  baseType{optional: optional},
		keyType:   keyType,
		valueType: valueType,
	}
}

func (m *MapType) String() string {
	result := fmt.Sprintf("Map[%s,%s]", m.keyType.String(), m.valueType.String())
	if m.optional {
		result += "?"
	}
	return result
}

func (m *MapType) Copy(optional *bool) Base {
	result := &MapType{
		baseType:  m.baseType,
		keyType:   m.keyType,
		valueType: m.valueType,
	}
	if optional != nil {
		result.baseType.optional = *optional
	}
	return result
}

func (m *MapType) KeyType() Base {
	return m.keyType
}

func (m *MapType) ValueType() Base {
	return m.valueType
}

func (m *MapType) Parameters() []Base {
	return []Base{m.keyType, m.valueType}
}

func (m *MapType) Coerces(target Base, checkQuant bool) bool {
	return m.Check(target, checkQuant) == nil
}

func (m *MapType) Check(target Base, checkQuant bool) error {
	if targetMap, ok := target.(*MapType); ok {
		if err := m.keyType.Check(targetMap.keyType, checkQuant); err != nil {
			return err
		}
		if err := m.valueType.Check(targetMap.valueType, checkQuant); err != nil {
			return err
		}
		return m.checkOptional(target, checkQuant)
	}
	
	if isAny(target) {
		return m.checkOptional(target, checkQuant)
	}
	
	return fmt.Errorf("cannot coerce %s to %s", m.String(), target.String())
}

func (m *MapType) Equatable(other Base, compound bool) bool {
	if otherMap, ok := other.(*MapType); ok {
		return m.keyType.Equatable(otherMap.keyType, true) && m.valueType.Equatable(otherMap.valueType, true)
	}
	return isAny(other)
}

func (m *MapType) Equal(other Base) bool {
	if otherMap, ok := other.(*MapType); ok {
		return m.keyType.Equal(otherMap.keyType) && m.valueType.Equal(otherMap.valueType)
	}
	return false
}

func (m *MapType) Comparable(other Base, checkQuant bool) bool {
	return false // Maps are not comparable
}

// PairType represents the WDL Pair type
type PairType struct {
	baseType
	leftType  Base
	rightType Base
}

func NewPair(leftType Base, rightType Base, optional bool) *PairType {
	return &PairType{
		baseType:  baseType{optional: optional},
		leftType:  leftType,
		rightType: rightType,
	}
}

func (p *PairType) String() string {
	result := fmt.Sprintf("Pair[%s,%s]", p.leftType.String(), p.rightType.String())
	if p.optional {
		result += "?"
	}
	return result
}

func (p *PairType) Copy(optional *bool) Base {
	result := &PairType{
		baseType:  p.baseType,
		leftType:  p.leftType,
		rightType: p.rightType,
	}
	if optional != nil {
		result.baseType.optional = *optional
	}
	return result
}

func (p *PairType) LeftType() Base {
	return p.leftType
}

func (p *PairType) RightType() Base {
	return p.rightType
}

func (p *PairType) Parameters() []Base {
	return []Base{p.leftType, p.rightType}
}

func (p *PairType) Coerces(target Base, checkQuant bool) bool {
	return p.Check(target, checkQuant) == nil
}

func (p *PairType) Check(target Base, checkQuant bool) error {
	if targetPair, ok := target.(*PairType); ok {
		if err := p.leftType.Check(targetPair.leftType, checkQuant); err != nil {
			return err
		}
		if err := p.rightType.Check(targetPair.rightType, checkQuant); err != nil {
			return err
		}
		return p.checkOptional(target, checkQuant)
	}
	
	if isAny(target) {
		return p.checkOptional(target, checkQuant)
	}
	
	return fmt.Errorf("cannot coerce %s to %s", p.String(), target.String())
}

func (p *PairType) Equatable(other Base, compound bool) bool {
	if otherPair, ok := other.(*PairType); ok {
		return p.leftType.Equatable(otherPair.leftType, true) && p.rightType.Equatable(otherPair.rightType, true)
	}
	return isAny(other)
}

func (p *PairType) Equal(other Base) bool {
	if otherPair, ok := other.(*PairType); ok {
		return p.leftType.Equal(otherPair.leftType) && p.rightType.Equal(otherPair.rightType)
	}
	return false
}

func (p *PairType) Comparable(other Base, checkQuant bool) bool {
	return false // Pairs are not comparable
}

// StructInstanceType represents a WDL struct instance type
type StructInstanceType struct {
	baseType
	typeName  string
	members   map[string]Base
	typeID    string
}

func NewStructInstance(typeName string, members map[string]Base, optional bool) *StructInstanceType {
	return &StructInstanceType{
		baseType: baseType{optional: optional},
		typeName: typeName,
		members:  members,
		typeID:   generateStructTypeID(typeName, members),
	}
}

func (s *StructInstanceType) String() string {
	result := s.typeName
	if s.optional {
		result += "?"
	}
	return result
}

func (s *StructInstanceType) Copy(optional *bool) Base {
	result := &StructInstanceType{
		baseType: s.baseType,
		typeName: s.typeName,
		members:  s.members,
		typeID:   s.typeID,
	}
	if optional != nil {
		result.baseType.optional = *optional
	}
	return result
}

func (s *StructInstanceType) TypeName() string {
	return s.typeName
}

func (s *StructInstanceType) Members() map[string]Base {
	return s.members
}

func (s *StructInstanceType) TypeID() string {
	return s.typeID
}

func (s *StructInstanceType) Coerces(target Base, checkQuant bool) bool {
	return s.Check(target, checkQuant) == nil
}

func (s *StructInstanceType) Check(target Base, checkQuant bool) error {
	if targetStruct, ok := target.(*StructInstanceType); ok {
		if s.typeID != targetStruct.typeID {
			return fmt.Errorf("struct types do not match")
		}
		return s.checkOptional(target, checkQuant)
	}
	
	if isAny(target) {
		return s.checkOptional(target, checkQuant)
	}
	
	return fmt.Errorf("cannot coerce %s to %s", s.String(), target.String())
}

func (s *StructInstanceType) Equatable(other Base, compound bool) bool {
	if otherStruct, ok := other.(*StructInstanceType); ok {
		return s.typeID == otherStruct.typeID
	}
	return isAny(other)
}

func (s *StructInstanceType) Equal(other Base) bool {
	if otherStruct, ok := other.(*StructInstanceType); ok {
		return s.typeID == otherStruct.typeID
	}
	return false
}

func (s *StructInstanceType) Comparable(other Base, checkQuant bool) bool {
	return false // Structs are not comparable
}

// ObjectType represents a WDL Object type (legacy)
type ObjectType struct {
	baseType
}

func NewObject(optional bool) *ObjectType {
	return &ObjectType{baseType: baseType{optional: optional}}
}

func (o *ObjectType) String() string {
	result := "Object"
	if o.optional {
		result += "?"
	}
	return result
}

func (o *ObjectType) Copy(optional *bool) Base {
	result := &ObjectType{baseType: o.baseType}
	if optional != nil {
		result.baseType.optional = *optional
	}
	return result
}

func (o *ObjectType) Coerces(target Base, checkQuant bool) bool {
	return o.Check(target, checkQuant) == nil
}

func (o *ObjectType) Check(target Base, checkQuant bool) error {
	if _, ok := target.(*ObjectType); ok {
		return o.checkOptional(target, checkQuant)
	}
	
	if isAny(target) {
		return o.checkOptional(target, checkQuant)
	}
	
	return fmt.Errorf("cannot coerce %s to %s", o.String(), target.String())
}

func (o *ObjectType) Equatable(other Base, compound bool) bool {
	_, isObject := other.(*ObjectType)
	return isObject || isAny(other)
}

func (o *ObjectType) Equal(other Base) bool {
	_, ok := other.(*ObjectType)
	return ok
}

func (o *ObjectType) Comparable(other Base, checkQuant bool) bool {
	return false // Objects are not comparable
}

// generateStructTypeID generates a unique ID for struct types
func generateStructTypeID(typeName string, members map[string]Base) string {
	var parts []string
	parts = append(parts, typeName)
	
	// Sort member names for consistent ID generation
	var memberNames []string
	for name := range members {
		memberNames = append(memberNames, name)
	}
	
	// Simple string concatenation for ID (could be made more sophisticated)
	for _, name := range memberNames {
		parts = append(parts, fmt.Sprintf("%s:%s", name, members[name].String()))
	}
	
	return strings.Join(parts, "|")
}