// Package values provides WDL value types
package values

import (
	"encoding/json"
	"fmt"

	"github.com/bioflowy/flowy/pkg/types"
)

// Base represents the abstract base interface for all WDL values
type Base interface {
	// Type returns the WDL type of this value
	Type() types.Base

	// Value returns the Go value representation
	Value() interface{}

	// JSON returns the JSON representation of this value
	JSON() json.RawMessage

	// Equal checks if two values are equal
	Equal(other Base) bool

	// String returns the string representation of the value
	String() string

	// Coerce attempts to coerce this value to the target type
	Coerce(targetType types.Base) (Base, error)
}

// baseValue provides common functionality for all values
type baseValue struct {
	typ types.Base
}

func (b *baseValue) Type() types.Base {
	return b.typ
}

// Null represents a null/None value
type Null struct {
	baseValue
}

// NewNull creates a new null value
func NewNull(typ types.Base) *Null {
	return &Null{
		baseValue: baseValue{typ: typ},
	}
}

func (n *Null) Value() interface{} {
	return nil
}

func (n *Null) JSON() json.RawMessage {
	return json.RawMessage("null")
}

func (n *Null) Equal(other Base) bool {
	_, ok := other.(*Null)
	return ok
}

func (n *Null) String() string {
	return "null"
}

func (n *Null) Coerce(targetType types.Base) (Base, error) {
	if !targetType.Optional() {
		return nil, fmt.Errorf("cannot coerce null to non-optional type %s", targetType.String())
	}
	return NewNull(targetType), nil
}