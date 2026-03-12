// convert.go provides utilities for converting WDL Values to JSON-compatible Go types.
package values

import "fmt"

// ValueToInterface converts a single WDL Value to a JSON-compatible interface{}.
// nil input and NullValue both return nil.
// Unknown value types are rendered as strings via fmt.Sprintf.
func ValueToInterface(v Value) interface{} {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case StringValue:
		return val.Value
	case IntValue:
		return val.Value
	case FloatValue:
		return val.Value
	case BooleanValue:
		return val.Value
	case FileValue:
		return val.Value
	case DirectoryValue:
		return val.Value
	case NullValue:
		return nil
	case ArrayValue:
		items := make([]interface{}, len(val.Items))
		for i, item := range val.Items {
			items[i] = ValueToInterface(item)
		}
		return items
	case MapValue:
		m := make(map[string]interface{}, len(val.Entries))
		for _, entry := range val.Entries {
			key := fmt.Sprintf("%v", ValueToInterface(entry.Key))
			m[key] = ValueToInterface(entry.Value)
		}
		return m
	case StructValue:
		m := make(map[string]interface{}, len(val.Fields))
		for k, fv := range val.Fields {
			m[k] = ValueToInterface(fv)
		}
		return m
	case PairValue:
		return map[string]interface{}{
			"left":  ValueToInterface(val.Left),
			"right": ValueToInterface(val.Right),
		}
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ValuesToJSON converts a map of WDL runtime values to JSON-compatible Go types.
// Unknown value types are rendered as strings via fmt.Sprintf.
func ValuesToJSON(outputs map[string]Value) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(outputs))
	for k, v := range outputs {
		result[k] = ValueToInterface(v)
	}
	return result, nil
}
