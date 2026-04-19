package server

func parameterMetaType(paramMeta map[string]any, name string) *string {
	return parameterMetaString(paramMeta, name, "type")
}

func parameterMetaLabel(paramMeta map[string]any, name string) *string {
	return parameterMetaString(paramMeta, name, "label")
}

func parameterMetaString(paramMeta map[string]any, name string, field string) *string {
	if len(paramMeta) == 0 {
		return nil
	}
	rawField, ok := paramMeta[name]
	if !ok {
		return nil
	}
	fieldMap, ok := rawField.(map[string]any)
	if !ok {
		return nil
	}
	value, ok := fieldMap[field].(string)
	if !ok || value == "" {
		return nil
	}
	return &value
}
