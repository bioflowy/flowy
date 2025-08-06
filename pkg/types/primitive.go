package types

import "fmt"

// BooleanType represents the WDL Boolean type
type BooleanType struct {
	baseType
}

func NewBoolean(optional bool) *BooleanType {
	return &BooleanType{baseType: baseType{optional: optional}}
}

func (b *BooleanType) String() string {
	if b.optional {
		return "Boolean?"
	}
	return "Boolean"
}

func (b *BooleanType) Copy(optional *bool) Base {
	result := &BooleanType{baseType: b.baseType}
	if optional != nil {
		result.baseType.optional = *optional
	}
	return result
}

func (b *BooleanType) Coerces(target Base, checkQuant bool) bool {
	return b.Check(target, checkQuant) == nil
}

func (b *BooleanType) Check(target Base, checkQuant bool) error {
	if _, ok := target.(*StringType); ok {
		return b.checkOptional(target, checkQuant)
	}
	if _, ok := target.(*BooleanType); ok {
		return b.checkOptional(target, checkQuant)
	}
	if isAny(target) {
		return b.checkOptional(target, checkQuant)
	}
	return fmt.Errorf("cannot coerce Boolean to %s", target.String())
}

func (b *BooleanType) Equatable(other Base, compound bool) bool {
	_, isBool := other.(*BooleanType)
	return isBool || isAny(other)
}

func (b *BooleanType) Equal(other Base) bool {
	_, ok := other.(*BooleanType)
	return ok
}

func (b *BooleanType) Comparable(other Base, checkQuant bool) bool {
	if checkQuant && (b.optional || other.Optional()) {
		return false
	}
	_, isBool := other.(*BooleanType)
	return isBool
}

// IntType represents the WDL Int type
type IntType struct {
	baseType
}

func NewInt(optional bool) *IntType {
	return &IntType{baseType: baseType{optional: optional}}
}

func (i *IntType) String() string {
	if i.optional {
		return "Int?"
	}
	return "Int"
}

func (i *IntType) Copy(optional *bool) Base {
	result := &IntType{baseType: i.baseType}
	if optional != nil {
		result.baseType.optional = *optional
	}
	return result
}

func (i *IntType) Coerces(target Base, checkQuant bool) bool {
	return i.Check(target, checkQuant) == nil
}

func (i *IntType) Check(target Base, checkQuant bool) error {
	switch target.(type) {
	case *IntType, *FloatType, *StringType:
		return i.checkOptional(target, checkQuant)
	case *ArrayType:
		if !checkQuant {
			// T can coerce to Array[T] in some contexts
			if targetArray := target.(*ArrayType); targetArray.itemType.Equal(i) {
				return i.checkOptional(target, checkQuant)
			}
		}
	}
	if isAny(target) {
		return i.checkOptional(target, checkQuant)
	}
	return fmt.Errorf("cannot coerce Int to %s", target.String())
}

func (i *IntType) Equatable(other Base, compound bool) bool {
	switch other.(type) {
	case *IntType:
		return true
	case *FloatType:
		return !compound // Int/Float coercion allowed at top level only
	}
	return isAny(other)
}

func (i *IntType) Equal(other Base) bool {
	_, ok := other.(*IntType)
	return ok
}

func (i *IntType) Comparable(other Base, checkQuant bool) bool {
	if checkQuant && (i.optional || other.Optional()) {
		return false
	}
	switch other.(type) {
	case *IntType, *FloatType:
		return true
	}
	return false
}

// FloatType represents the WDL Float type
type FloatType struct {
	baseType
}

func NewFloat(optional bool) *FloatType {
	return &FloatType{baseType: baseType{optional: optional}}
}

func (f *FloatType) String() string {
	if f.optional {
		return "Float?"
	}
	return "Float"
}

func (f *FloatType) Copy(optional *bool) Base {
	result := &FloatType{baseType: f.baseType}
	if optional != nil {
		result.baseType.optional = *optional
	}
	return result
}

func (f *FloatType) Coerces(target Base, checkQuant bool) bool {
	return f.Check(target, checkQuant) == nil
}

func (f *FloatType) Check(target Base, checkQuant bool) error {
	switch target.(type) {
	case *FloatType, *StringType:
		return f.checkOptional(target, checkQuant)
	}
	if isAny(target) {
		return f.checkOptional(target, checkQuant)
	}
	return fmt.Errorf("cannot coerce Float to %s", target.String())
}

func (f *FloatType) Equatable(other Base, compound bool) bool {
	switch other.(type) {
	case *FloatType:
		return true
	case *IntType:
		return !compound // Int/Float coercion allowed at top level only
	}
	return isAny(other)
}

func (f *FloatType) Equal(other Base) bool {
	_, ok := other.(*FloatType)
	return ok
}

func (f *FloatType) Comparable(other Base, checkQuant bool) bool {
	if checkQuant && (f.optional || other.Optional()) {
		return false
	}
	switch other.(type) {
	case *IntType, *FloatType:
		return true
	}
	return false
}

// StringType represents the WDL String type
type StringType struct {
	baseType
}

func NewString(optional bool) *StringType {
	return &StringType{baseType: baseType{optional: optional}}
}

func (s *StringType) String() string {
	if s.optional {
		return "String?"
	}
	return "String"
}

func (s *StringType) Copy(optional *bool) Base {
	result := &StringType{baseType: s.baseType}
	if optional != nil {
		result.baseType.optional = *optional
	}
	return result
}

func (s *StringType) Coerces(target Base, checkQuant bool) bool {
	return s.Check(target, checkQuant) == nil
}

func (s *StringType) Check(target Base, checkQuant bool) error {
	switch target.(type) {
	case *StringType, *FileType, *IntType, *FloatType:
		return s.checkOptional(target, checkQuant)
	}
	if isAny(target) {
		return s.checkOptional(target, checkQuant)
	}
	return fmt.Errorf("cannot coerce String to %s", target.String())
}

func (s *StringType) Equatable(other Base, compound bool) bool {
	_, isString := other.(*StringType)
	return isString || isAny(other)
}

func (s *StringType) Equal(other Base) bool {
	_, ok := other.(*StringType)
	return ok
}

func (s *StringType) Comparable(other Base, checkQuant bool) bool {
	if checkQuant && (s.optional || other.Optional()) {
		return false
	}
	_, isString := other.(*StringType)
	return isString
}

// FileType represents the WDL File type
type FileType struct {
	baseType
}

func NewFile(optional bool) *FileType {
	return &FileType{baseType: baseType{optional: optional}}
}

func (f *FileType) String() string {
	if f.optional {
		return "File?"
	}
	return "File"
}

func (f *FileType) Copy(optional *bool) Base {
	result := &FileType{baseType: f.baseType}
	if optional != nil {
		result.baseType.optional = *optional
	}
	return result
}

func (f *FileType) Coerces(target Base, checkQuant bool) bool {
	return f.Check(target, checkQuant) == nil
}

func (f *FileType) Check(target Base, checkQuant bool) error {
	switch target.(type) {
	case *FileType, *StringType:
		return f.checkOptional(target, checkQuant)
	}
	if isAny(target) {
		return f.checkOptional(target, checkQuant)
	}
	return fmt.Errorf("cannot coerce File to %s", target.String())
}

func (f *FileType) Equatable(other Base, compound bool) bool {
	_, isFile := other.(*FileType)
	return isFile || isAny(other)
}

func (f *FileType) Equal(other Base) bool {
	_, ok := other.(*FileType)
	return ok
}

func (f *FileType) Comparable(other Base, checkQuant bool) bool {
	return false // File is not comparable
}

// DirectoryType represents the WDL Directory type
type DirectoryType struct {
	baseType
}

func NewDirectory(optional bool) *DirectoryType {
	return &DirectoryType{baseType: baseType{optional: optional}}
}

func (d *DirectoryType) String() string {
	if d.optional {
		return "Directory?"
	}
	return "Directory"
}

func (d *DirectoryType) Copy(optional *bool) Base {
	result := &DirectoryType{baseType: d.baseType}
	if optional != nil {
		result.baseType.optional = *optional
	}
	return result
}

func (d *DirectoryType) Coerces(target Base, checkQuant bool) bool {
	return d.Check(target, checkQuant) == nil
}

func (d *DirectoryType) Check(target Base, checkQuant bool) error {
	switch target.(type) {
	case *DirectoryType, *StringType:
		return d.checkOptional(target, checkQuant)
	}
	if isAny(target) {
		return d.checkOptional(target, checkQuant)
	}
	return fmt.Errorf("cannot coerce Directory to %s", target.String())
}

func (d *DirectoryType) Equatable(other Base, compound bool) bool {
	_, isDir := other.(*DirectoryType)
	return isDir || isAny(other)
}

func (d *DirectoryType) Equal(other Base) bool {
	_, ok := other.(*DirectoryType)
	return ok
}

func (d *DirectoryType) Comparable(other Base, checkQuant bool) bool {
	return false // Directory is not comparable
}