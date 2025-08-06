package values

import (
	"encoding/json"
	"fmt"
)

// ToJSON converts a WDL value to JSON
func ToJSON(value Base) (json.RawMessage, error) {
	if value == nil {
		return json.RawMessage("null"), nil
	}
	return value.JSON(), nil
}

// ParseJSON parses JSON string to a WDL value
func ParseJSON(jsonStr string) (Base, error) {
	var data json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %v", err)
	}
	return inferValueFromJSON(data)
}

// MarshalJSON marshals a WDL value to JSON bytes
func MarshalJSON(value Base) ([]byte, error) {
	if value == nil {
		return []byte("null"), nil
	}
	return value.JSON(), nil
}

// UnmarshalJSON unmarshals JSON bytes to a WDL value
func UnmarshalJSON(data []byte) (Base, error) {
	return inferValueFromJSON(json.RawMessage(data))
}