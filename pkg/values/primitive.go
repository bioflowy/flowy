package values

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/bioflowy/flowy/pkg/types"
)

// BooleanValue represents a WDL Boolean value
type BooleanValue struct {
	baseValue
	value bool
}

// NewBoolean creates a new Boolean value
func NewBoolean(value bool, optional bool) *BooleanValue {
	return &BooleanValue{
		baseValue: baseValue{typ: types.NewBoolean(optional)},
		value:     value,
	}
}

func (b *BooleanValue) Value() interface{} {
	return b.value
}

func (b *BooleanValue) JSON() json.RawMessage {
	data, _ := json.Marshal(b.value)
	return json.RawMessage(data)
}

func (b *BooleanValue) Equal(other Base) bool {
	if otherBool, ok := other.(*BooleanValue); ok {
		return b.value == otherBool.value
	}
	return false
}

func (b *BooleanValue) String() string {
	if b.value {
		return "true"
	}
	return "false"
}

func (b *BooleanValue) Coerce(targetType types.Base) (Base, error) {
	switch targetType.(type) {
	case *types.BooleanType:
		return b, nil
	case *types.StringType:
		return NewString(b.String(), targetType.Optional()), nil
	default:
		return nil, fmt.Errorf("cannot coerce Boolean to %s", targetType.String())
	}
}

// IntValue represents a WDL Int value
type IntValue struct {
	baseValue
	value int64
}

// NewInt creates a new Int value
func NewInt(value int64, optional bool) *IntValue {
	return &IntValue{
		baseValue: baseValue{typ: types.NewInt(optional)},
		value:     value,
	}
}

func (i *IntValue) Value() interface{} {
	return i.value
}

func (i *IntValue) JSON() json.RawMessage {
	data, _ := json.Marshal(i.value)
	return json.RawMessage(data)
}

func (i *IntValue) Equal(other Base) bool {
	switch o := other.(type) {
	case *IntValue:
		return i.value == o.value
	case *FloatValue:
		return float64(i.value) == o.value
	default:
		return false
	}
}

func (i *IntValue) String() string {
	return strconv.FormatInt(i.value, 10)
}

func (i *IntValue) Coerce(targetType types.Base) (Base, error) {
	switch targetType.(type) {
	case *types.IntType:
		return i, nil
	case *types.FloatType:
		return NewFloat(float64(i.value), targetType.Optional()), nil
	case *types.StringType:
		return NewString(i.String(), targetType.Optional()), nil
	default:
		return nil, fmt.Errorf("cannot coerce Int to %s", targetType.String())
	}
}

// FloatValue represents a WDL Float value
type FloatValue struct {
	baseValue
	value float64
}

// NewFloat creates a new Float value
func NewFloat(value float64, optional bool) *FloatValue {
	return &FloatValue{
		baseValue: baseValue{typ: types.NewFloat(optional)},
		value:     value,
	}
}

func (f *FloatValue) Value() interface{} {
	return f.value
}

func (f *FloatValue) JSON() json.RawMessage {
	data, _ := json.Marshal(f.value)
	return json.RawMessage(data)
}

func (f *FloatValue) Equal(other Base) bool {
	switch o := other.(type) {
	case *FloatValue:
		return f.value == o.value
	case *IntValue:
		return f.value == float64(o.value)
	default:
		return false
	}
}

func (f *FloatValue) String() string {
	return strconv.FormatFloat(f.value, 'f', -1, 64)
}

func (f *FloatValue) Coerce(targetType types.Base) (Base, error) {
	switch targetType.(type) {
	case *types.FloatType:
		return f, nil
	case *types.StringType:
		return NewString(f.String(), targetType.Optional()), nil
	default:
		return nil, fmt.Errorf("cannot coerce Float to %s", targetType.String())
	}
}

// StringValue represents a WDL String value
type StringValue struct {
	baseValue
	value string
}

// NewString creates a new String value
func NewString(value string, optional bool) *StringValue {
	return &StringValue{
		baseValue: baseValue{typ: types.NewString(optional)},
		value:     value,
	}
}

func (s *StringValue) Value() interface{} {
	return s.value
}

func (s *StringValue) JSON() json.RawMessage {
	data, _ := json.Marshal(s.value)
	return json.RawMessage(data)
}

func (s *StringValue) Equal(other Base) bool {
	if otherString, ok := other.(*StringValue); ok {
		return s.value == otherString.value
	}
	return false
}

func (s *StringValue) String() string {
	return s.value
}

func (s *StringValue) Coerce(targetType types.Base) (Base, error) {
	switch targetType.(type) {
	case *types.StringType:
		return s, nil
	case *types.IntType:
		intVal, err := strconv.ParseInt(s.value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse '%s' as Int: %v", s.value, err)
		}
		return NewInt(intVal, targetType.Optional()), nil
	case *types.FloatType:
		floatVal, err := strconv.ParseFloat(s.value, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse '%s' as Float: %v", s.value, err)
		}
		return NewFloat(floatVal, targetType.Optional()), nil
	case *types.FileType:
		return NewFile(s.value, targetType.Optional()), nil
	case *types.DirectoryType:
		return NewDirectory(s.value, targetType.Optional()), nil
	default:
		return nil, fmt.Errorf("cannot coerce String to %s", targetType.String())
	}
}

// FileValue represents a WDL File value
type FileValue struct {
	baseValue
	value string
}

// NewFile creates a new File value
func NewFile(value string, optional bool) *FileValue {
	return &FileValue{
		baseValue: baseValue{typ: types.NewFile(optional)},
		value:     value,
	}
}

func (f *FileValue) Value() interface{} {
	return f.value
}

func (f *FileValue) JSON() json.RawMessage {
	data, _ := json.Marshal(f.value)
	return json.RawMessage(data)
}

func (f *FileValue) Equal(other Base) bool {
	if otherFile, ok := other.(*FileValue); ok {
		return f.value == otherFile.value
	}
	return false
}

func (f *FileValue) String() string {
	return f.value
}

func (f *FileValue) Coerce(targetType types.Base) (Base, error) {
	switch targetType.(type) {
	case *types.FileType:
		return f, nil
	case *types.StringType:
		return NewString(f.value, targetType.Optional()), nil
	default:
		return nil, fmt.Errorf("cannot coerce File to %s", targetType.String())
	}
}

// DirectoryValue represents a WDL Directory value
type DirectoryValue struct {
	baseValue
	value string
}

// NewDirectory creates a new Directory value
func NewDirectory(value string, optional bool) *DirectoryValue {
	return &DirectoryValue{
		baseValue: baseValue{typ: types.NewDirectory(optional)},
		value:     value,
	}
}

func (d *DirectoryValue) Value() interface{} {
	return d.value
}

func (d *DirectoryValue) JSON() json.RawMessage {
	data, _ := json.Marshal(d.value)
	return json.RawMessage(data)
}

func (d *DirectoryValue) Equal(other Base) bool {
	if otherDir, ok := other.(*DirectoryValue); ok {
		return d.value == otherDir.value
	}
	return false
}

func (d *DirectoryValue) String() string {
	return d.value
}

func (d *DirectoryValue) Coerce(targetType types.Base) (Base, error) {
	switch targetType.(type) {
	case *types.DirectoryType:
		return d, nil
	case *types.StringType:
		return NewString(d.value, targetType.Optional()), nil
	default:
		return nil, fmt.Errorf("cannot coerce Directory to %s", targetType.String())
	}
}