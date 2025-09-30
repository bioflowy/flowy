use crate::env::Bindings;
use crate::value::Value;
use crate::WdlError;

/// Convert bindings into JSON, applying an optional namespace prefix to each key.
pub fn bindings_to_json_with_namespace(
    outputs: &Bindings<Value>,
    namespace: Option<&str>,
) -> Result<serde_json::Value, WdlError> {
    let mut map = serde_json::Map::new();

    let namespace_prefix = if let Some(ns) = namespace {
        if ns.is_empty() {
            String::new()
        } else if ns.ends_with('.') {
            ns.to_string()
        } else {
            format!("{}.", ns)
        }
    } else {
        String::new()
    };

    for binding in outputs.iter() {
        let json_value = value_to_json(binding.value())?;
        let key = if !binding.name().starts_with('_') && !namespace_prefix.is_empty() {
            format!("{}{}", namespace_prefix, binding.name())
        } else {
            binding.name().to_string()
        };
        map.insert(key, json_value);
    }

    Ok(serde_json::Value::Object(map))
}

/// Convert a WDL value into a JSON value.
pub fn value_to_json(value: &Value) -> Result<serde_json::Value, WdlError> {
    match value {
        Value::Null => Ok(serde_json::Value::Null),
        Value::Boolean { value, .. } => Ok(serde_json::Value::Bool(*value)),
        Value::Int { value, .. } => Ok(serde_json::Value::Number((*value).into())),
        Value::Float { value, .. } => serde_json::Number::from_f64(*value)
            .map(serde_json::Value::Number)
            .ok_or_else(|| WdlError::Validation {
                message: format!("Invalid float value: {}", value),
                pos: crate::SourcePosition::new(String::new(), String::new(), 0, 0, 0, 0),
                source_text: Some(String::new()),
                declared_wdl_version: Some("1.0".to_string()),
            }),
        Value::String { value, .. }
        | Value::File { value, .. }
        | Value::Directory { value, .. } => Ok(serde_json::Value::String(value.clone())),
        Value::Array { values, .. } => {
            let arr: Result<Vec<_>, _> = values.iter().map(value_to_json).collect();
            Ok(serde_json::Value::Array(arr?))
        }
        Value::Pair { left, right, .. } => {
            let mut map = serde_json::Map::new();
            map.insert("left".to_string(), value_to_json(left)?);
            map.insert("right".to_string(), value_to_json(right)?);
            Ok(serde_json::Value::Object(map))
        }
        Value::Map { pairs, .. } => {
            let mut map = serde_json::Map::new();
            for (k, v) in pairs {
                let key_str = match k {
                    Value::String { value, .. } => value.clone(),
                    _ => format!("{:?}", k),
                };
                map.insert(key_str, value_to_json(v)?);
            }
            Ok(serde_json::Value::Object(map))
        }
        Value::Struct { members, .. } => {
            let mut map = serde_json::Map::new();
            for (k, v) in members {
                map.insert(k.clone(), value_to_json(v)?);
            }
            Ok(serde_json::Value::Object(map))
        }
    }
}
