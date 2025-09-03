//! WDL values instantiated at runtime
//!
//! Each value is represented by a Rust enum corresponding to the WDL value types.
//! Values carry both their runtime data and their associated WDL type information.

use crate::error::{SourcePosition, WdlError};
use crate::types::Type;
use once_cell::sync::Lazy;
use serde::{Deserialize, Serialize};
use serde_json::{Map as JsonMap, Value as JsonValue};
use std::collections::{HashMap, HashSet};
use std::fmt;

/// Base trait for WDL runtime values
pub trait ValueBase {
    /// Get the WDL type of this value
    fn wdl_type(&self) -> &Type;

    /// Convert to JSON representation
    fn to_json(&self) -> JsonValue;

    /// Coerce this value to a desired type
    fn coerce(&self, desired_type: &Type) -> Result<Value, WdlError>;

    /// Get all child values (for compound types)
    fn children(&self) -> Vec<&Value> {
        Vec::new()
    }
}

/// WDL runtime value
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum Value {
    /// Null value (None in WDL)
    Null,

    /// Boolean value
    Boolean { value: bool, wdl_type: Type },

    /// Integer value  
    Int { value: i64, wdl_type: Type },

    /// Float value
    Float { value: f64, wdl_type: Type },

    /// String value
    String { value: String, wdl_type: Type },

    /// File value (string with File type)
    File { value: String, wdl_type: Type },

    /// Directory value (string with Directory type)
    Directory { value: String, wdl_type: Type },

    /// Array value
    Array { values: Vec<Value>, wdl_type: Type },

    /// Map value (key-value pairs)
    Map {
        pairs: Vec<(Value, Value)>,
        wdl_type: Type,
    },

    /// Pair value
    Pair {
        left: Box<Value>,
        right: Box<Value>,
        wdl_type: Type,
    },

    /// Struct value
    Struct {
        members: HashMap<String, Value>,
        extra_keys: HashSet<String>,
        wdl_type: Type,
    },
}

impl Value {
    /// Create a new Null value
    pub fn null() -> Self {
        Value::Null
    }

    /// Create a new Boolean value
    pub fn boolean(value: bool) -> Self {
        Value::Boolean {
            value,
            wdl_type: Type::boolean(false),
        }
    }

    /// Create a new Int value
    pub fn int(value: i64) -> Self {
        Value::Int {
            value,
            wdl_type: Type::int(false),
        }
    }

    /// Create a new Float value
    pub fn float(value: f64) -> Self {
        Value::Float {
            value,
            wdl_type: Type::float(false),
        }
    }

    /// Create a new String value
    pub fn string(value: String) -> Self {
        Value::String {
            value,
            wdl_type: Type::string(false),
        }
    }

    /// Create a new File value
    pub fn file(value: String) -> Result<Self, WdlError> {
        // Validate file path - no trailing slashes
        if value != value.trim_end_matches('/') {
            return Err(WdlError::validation_error(
                SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                format!("Invalid file path: {}", value),
            ));
        }

        Ok(Value::File {
            value,
            wdl_type: Type::file(false),
        })
    }

    /// Create a new Directory value
    pub fn directory(value: String) -> Self {
        Value::Directory {
            value,
            wdl_type: Type::directory(false),
        }
    }

    /// Create a new Array value
    pub fn array(item_type: Type, values: Vec<Value>) -> Self {
        let nonempty = !values.is_empty();
        Value::Array {
            values,
            wdl_type: Type::array(item_type, false, nonempty),
        }
    }

    /// Create a new Map value
    pub fn map(key_type: Type, value_type: Type, pairs: Vec<(Value, Value)>) -> Self {
        Value::Map {
            pairs,
            wdl_type: Type::map(key_type, value_type, false),
        }
    }

    /// Create a new Pair value
    pub fn pair(left_type: Type, right_type: Type, left: Value, right: Value) -> Self {
        Value::Pair {
            left: Box::new(left),
            right: Box::new(right),
            wdl_type: Type::pair(left_type, right_type, false),
        }
    }

    /// Create a new Struct value with validation
    pub fn struct_value(
        struct_type: Type,
        mut members: HashMap<String, Value>,
        extra_keys: Option<HashSet<String>>,
    ) -> Result<Self, WdlError> {
        let mut final_extra_keys = extra_keys.unwrap_or_default();

        // If struct type has resolved members, validate and fill in missing optional members
        if let Type::StructInstance {
            members: Some(struct_members),
            ..
        } = &struct_type
        {
            // Fill in null for any omitted optional members
            for (name, member_type) in struct_members {
                if !members.contains_key(name) {
                    if member_type.is_optional() {
                        members.insert(name.clone(), Value::null());
                    } else {
                        return Err(WdlError::validation_error(
                            SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                            format!("Missing required struct member: {}", name),
                        ));
                    }
                }
            }

            // Track extra keys that don't correspond to struct members
            for key in members.keys() {
                if !struct_members.contains_key(key) {
                    final_extra_keys.insert(key.clone());
                }
            }
        }

        Ok(Value::Struct {
            members,
            extra_keys: final_extra_keys,
            wdl_type: struct_type,
        })
    }

    /// Create a new Struct value without validation (for backward compatibility)
    pub fn struct_value_unchecked(
        struct_type: Type,
        members: HashMap<String, Value>,
        extra_keys: Option<HashSet<String>>,
    ) -> Self {
        Value::Struct {
            members,
            extra_keys: extra_keys.unwrap_or_default(),
            wdl_type: struct_type,
        }
    }

    /// Create a Value from JSON, inferring the type
    pub fn from_json(json_value: JsonValue) -> Self {
        match json_value {
            JsonValue::Null => Value::null(),
            JsonValue::Bool(b) => Value::boolean(b),
            JsonValue::Number(n) => {
                if let Some(i) = n.as_i64() {
                    Value::int(i)
                } else if let Some(f) = n.as_f64() {
                    Value::float(f)
                } else {
                    // Fallback to string representation
                    Value::string(n.to_string())
                }
            }
            JsonValue::String(s) => Value::string(s),
            JsonValue::Array(arr) => {
                let values: Vec<Value> = arr.into_iter().map(Value::from_json).collect();
                // Infer item type from first element, or use Any if empty
                let item_type = values
                    .first()
                    .map(|v| v.wdl_type().clone())
                    .unwrap_or_else(Type::any);
                Value::array(item_type, values)
            }
            JsonValue::Object(obj) => {
                let members: HashMap<String, Value> = obj
                    .into_iter()
                    .map(|(k, v)| (k, Value::from_json(v)))
                    .collect();

                // Create an Object type for JSON objects
                let member_types: HashMap<String, Type> = members
                    .iter()
                    .map(|(k, v)| (k.clone(), v.wdl_type().clone()))
                    .collect();

                Value::struct_value_unchecked(Type::object(member_types), members, None)
            }
        }
    }

    /// Convert a Value with a specific type constraint from JSON
    pub fn from_json_with_type(ty: &Type, json_value: JsonValue) -> Result<Self, WdlError> {
        let value = Value::from_json(json_value);
        value.coerce(ty)
    }
}

// Static instance for Null type
static NULL_TYPE: Lazy<Type> = Lazy::new(|| Type::any().with_optional(true));

impl ValueBase for Value {
    fn wdl_type(&self) -> &Type {
        match self {
            Value::Null => &NULL_TYPE,
            Value::Boolean { wdl_type, .. } => wdl_type,
            Value::Int { wdl_type, .. } => wdl_type,
            Value::Float { wdl_type, .. } => wdl_type,
            Value::String { wdl_type, .. } => wdl_type,
            Value::File { wdl_type, .. } => wdl_type,
            Value::Directory { wdl_type, .. } => wdl_type,
            Value::Array { wdl_type, .. } => wdl_type,
            Value::Map { wdl_type, .. } => wdl_type,
            Value::Pair { wdl_type, .. } => wdl_type,
            Value::Struct { wdl_type, .. } => wdl_type,
        }
    }

    fn to_json(&self) -> JsonValue {
        match self {
            Value::Null => JsonValue::Null,
            Value::Boolean { value, .. } => JsonValue::Bool(*value),
            Value::Int { value, .. } => JsonValue::Number((*value).into()),
            Value::Float { value, .. } => {
                JsonValue::Number(serde_json::Number::from_f64(*value).unwrap_or_else(|| 0.into()))
            }
            Value::String { value, .. }
            | Value::File { value, .. }
            | Value::Directory { value, .. } => JsonValue::String(value.clone()),
            Value::Array { values, .. } => {
                JsonValue::Array(values.iter().map(|v| v.to_json()).collect())
            }
            Value::Map { pairs, .. } => {
                // Try to convert to JSON object by stringifying keys
                let mut obj = JsonMap::new();
                for (k, v) in pairs {
                    let key_str = match k {
                        Value::String { value, .. } => value.clone(),
                        Value::Int { value, .. } => value.to_string(),
                        Value::Float { value, .. } => value.to_string(),
                        Value::Boolean { value, .. } => value.to_string(),
                        Value::File { value, .. } | Value::Directory { value, .. } => value.clone(),
                        _ => format!("{:?}", k), // Fallback for complex types
                    };
                    obj.insert(key_str, v.to_json());
                }
                JsonValue::Object(obj)
            }
            Value::Pair { left, right, .. } => {
                let mut obj = JsonMap::new();
                obj.insert("left".to_string(), left.to_json());
                obj.insert("right".to_string(), right.to_json());
                JsonValue::Object(obj)
            }
            Value::Struct { members, .. } => {
                let obj: JsonMap<String, JsonValue> = members
                    .iter()
                    .map(|(k, v)| (k.clone(), v.to_json()))
                    .collect();
                JsonValue::Object(obj)
            }
        }
    }

    fn coerce(&self, desired_type: &Type) -> Result<Value, WdlError> {
        // Early return if types already match
        if *self.wdl_type() == *desired_type {
            return Ok(self.clone());
        }

        // Dispatch to type-specific methods
        match self {
            Value::Array { .. } => self.coerce_array(desired_type),
            Value::Map { .. } => self.coerce_map(desired_type),
            Value::Pair { .. } => self.coerce_pair(desired_type),
            Value::Struct { .. } => self.coerce_struct(desired_type),
            _ => self.coerce_base(desired_type),
        }
    }
}

impl Value {
    /// Base coercion method for simple types and common cases
    fn coerce_base(&self, desired_type: &Type) -> Result<Value, WdlError> {
        // Handle coercion to String - almost everything can be coerced to string
        if let Type::String { .. } = desired_type {
            let str_repr = match self {
                Value::Null => {
                    // For optional types, preserve null instead of converting to empty string
                    if desired_type.is_optional() {
                        return Ok(self.clone());
                    } else {
                        "".to_string()
                    }
                }
                Value::Boolean { value, .. } => value.to_string(),
                Value::Int { value, .. } => value.to_string(),
                Value::Float { value, .. } => format!("{:.6}", value),
                Value::String { value, .. }
                | Value::File { value, .. }
                | Value::Directory { value, .. } => value.clone(),
                Value::Array { values, .. } => {
                    let items: Vec<String> = values.iter().map(|v| format!("{}", v)).collect();
                    format!("[{}]", items.join(", "))
                }
                Value::Map { pairs, .. } => {
                    let items: Vec<String> =
                        pairs.iter().map(|(k, v)| format!("{}: {}", k, v)).collect();
                    format!("{{{}}}", items.join(", "))
                }
                Value::Pair { left, right, .. } => {
                    format!("({},{})", left, right)
                }
                Value::Struct { members, .. } => {
                    let items: Vec<String> = members
                        .iter()
                        .map(|(k, v)| format!("{}: {}", k, v))
                        .collect();
                    format!("{{{}}}", items.join(", "))
                }
            };
            return Ok(Value::string(str_repr));
        }

        // Handle T -> Array[T] promotion (only for non-arrays)
        // Following miniwdl's logic: promote single value to array
        if let Type::Array { item_type, .. } = desired_type {
            if !matches!(self, Value::Array { .. }) {
                // Only promote non-arrays to arrays, and only if this value's type
                // can coerce to the array's item type
                if self.wdl_type().coerces(item_type, false) {
                    let coerced_item = self.coerce(item_type)?;
                    return Ok(Value::array(item_type.as_ref().clone(), vec![coerced_item]));
                }
            }
        }

        // Specific type coercions for simple types
        match (self, desired_type) {
            // Null coercions
            (Value::Null, ty) => {
                if !ty.is_optional() && !matches!(ty, Type::Any { .. }) {
                    return Err(WdlError::NullValue {
                        pos: SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                    });
                }
                Ok(self.clone())
            }

            // Int to Float coercion
            (Value::Int { value, .. }, Type::Float { .. }) => Ok(Value::float(*value as f64)),

            // String coercions to other types
            (Value::String { value, .. }, Type::File { .. }) => Value::file(value.clone()),
            (Value::String { value, .. }, Type::Directory { .. }) => {
                Ok(Value::directory(value.clone()))
            }
            (Value::String { value, .. }, Type::Int { .. }) => match value.parse::<i64>() {
                Ok(i) => Ok(Value::int(i)),
                Err(_) => Err(WdlError::Eval {
                    pos: SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                    message: format!("Cannot coerce '{}' to Int", value),
                }),
            },
            (Value::String { value, .. }, Type::Float { .. }) => match value.parse::<f64>() {
                Ok(f) => Ok(Value::float(f)),
                Err(_) => Err(WdlError::Eval {
                    pos: SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                    message: format!("Cannot coerce '{}' to Float", value),
                }),
            },

            // Same type - return self (this handles cases not caught by early return)
            _ if self.wdl_type().coerces(desired_type, true) => Ok(self.clone()),

            // Coercion not possible
            _ => Err(WdlError::static_type_mismatch(
                SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                desired_type.to_string(),
                self.wdl_type().to_string(),
                format!("Cannot coerce {} to {}", self.wdl_type(), desired_type),
            )),
        }
    }

    /// Array-specific coercion following miniwdl's pattern
    fn coerce_array(&self, desired_type: &Type) -> Result<Value, WdlError> {
        if let Value::Array { values, wdl_type } = self {
            match desired_type {
                Type::Array {
                    item_type,
                    nonempty,
                    ..
                } => {
                    // Check nonempty constraint
                    if *nonempty && values.is_empty() {
                        return Err(WdlError::EmptyArray {
                            pos: SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                        });
                    }

                    // Extract current item type
                    let current_item_type = match wdl_type {
                        Type::Array {
                            item_type: current_item,
                            ..
                        } => current_item,
                        _ => unreachable!("Array value must have Array type"),
                    };

                    // If item types are the same, return self (already handled by early return, but for safety)
                    if **current_item_type == **item_type {
                        return Ok(self.clone());
                    }

                    // Otherwise coerce each element
                    let coerced_values: Result<Vec<_>, _> =
                        values.iter().map(|v| v.coerce(item_type)).collect();

                    Ok(Value::array(item_type.as_ref().clone(), coerced_values?))
                }
                // Handle coercion to String (array representation)
                Type::String { .. } => self.coerce_base(desired_type),
                // Other coercions fall back to base
                _ => self.coerce_base(desired_type),
            }
        } else {
            unreachable!("coerce_array called on non-array value")
        }
    }

    /// Map-specific coercion following miniwdl's pattern
    fn coerce_map(&self, desired_type: &Type) -> Result<Value, WdlError> {
        if let Value::Map { pairs, .. } = self {
            match desired_type {
                Type::Map {
                    key_type,
                    value_type,
                    ..
                } => {
                    let coerced_pairs: Result<Vec<_>, _> = pairs
                        .iter()
                        .map(|(k, v)| Ok((k.coerce(key_type)?, v.coerce(value_type)?)))
                        .collect();

                    Ok(Value::map(
                        key_type.as_ref().clone(),
                        value_type.as_ref().clone(),
                        coerced_pairs?,
                    ))
                }
                Type::StructInstance {
                    members: Some(struct_members),
                    ..
                } => {
                    // Convert map to struct - validate that map keys match struct members
                    let mut struct_values = HashMap::new();
                    let mut extra_keys = HashSet::new();

                    // Convert map pairs to string keys for lookup
                    for (key_value, value_value) in pairs {
                        let key_str = match key_value {
                            Value::String { value, .. } => value.clone(),
                            Value::Int { value, .. } => value.to_string(),
                            Value::Float { value, .. } => value.to_string(),
                            Value::Boolean { value, .. } => value.to_string(),
                            Value::File { value, .. } | Value::Directory { value, .. } => {
                                value.clone()
                            }
                            _ => {
                                return Err(WdlError::validation_error(
                                    SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                                    format!(
                                        "Cannot convert {:?} key to struct member name",
                                        key_value
                                    ),
                                ))
                            }
                        };

                        if let Some(expected_type) = struct_members.get(&key_str) {
                            // This key matches a struct member - coerce the value
                            match value_value.coerce(expected_type) {
                                Ok(coerced_value) => {
                                    struct_values.insert(key_str, coerced_value);
                                }
                                Err(e) => {
                                    return Err(WdlError::validation_error(
                                        SourcePosition::new(
                                            "".to_string(),
                                            "".to_string(),
                                            0,
                                            0,
                                            0,
                                            0,
                                        ),
                                        format!(
                                            "Cannot coerce value for struct member {}: {}",
                                            key_str, e
                                        ),
                                    ))
                                }
                            }
                        } else {
                            // This key doesn't match any struct member - this is an error
                            extra_keys.insert(key_str);
                        }
                    }

                    // Check if we have extra keys that don't belong to the struct
                    if !extra_keys.is_empty() {
                        return Err(WdlError::validation_error(
                            SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                            format!(
                                "Map keys {:?} do not match struct members {:?}",
                                extra_keys.iter().collect::<Vec<_>>(),
                                struct_members.keys().collect::<Vec<_>>()
                            ),
                        ));
                    }

                    // Use the validated constructor
                    Ok(Value::struct_value(
                        desired_type.clone(),
                        struct_values,
                        Some(extra_keys),
                    )?)
                }
                Type::StructInstance { members: None, .. } => {
                    // Cannot coerce to unresolved struct - need struct definition
                    Err(WdlError::validation_error(
                        SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                        "Cannot coerce Map to unresolved struct type. Struct definition needed."
                            .to_string(),
                    ))
                }
                // Handle coercion to String (map representation)
                Type::String { .. } => self.coerce_base(desired_type),
                // Other coercions fall back to base
                _ => self.coerce_base(desired_type),
            }
        } else {
            unreachable!("coerce_map called on non-map value")
        }
    }

    /// Pair-specific coercion following miniwdl's pattern
    fn coerce_pair(&self, desired_type: &Type) -> Result<Value, WdlError> {
        if let Value::Pair { left, right, .. } = self {
            match desired_type {
                Type::Pair {
                    left_type,
                    right_type,
                    ..
                } => Ok(Value::pair(
                    left_type.as_ref().clone(),
                    right_type.as_ref().clone(),
                    left.coerce(left_type)?,
                    right.coerce(right_type)?,
                )),
                // Handle coercion to String (pair representation)
                Type::String { .. } => self.coerce_base(desired_type),
                // Other coercions fall back to base
                _ => self.coerce_base(desired_type),
            }
        } else {
            unreachable!("coerce_pair called on non-pair value")
        }
    }

    /// Struct-specific coercion following miniwdl's pattern
    fn coerce_struct(&self, desired_type: &Type) -> Result<Value, WdlError> {
        if let Value::Struct {
            members,
            extra_keys,
            ..
        } = self
        {
            match desired_type {
                Type::StructInstance {
                    members: target_members,
                    ..
                } => {
                    // Coerce each member to the target member type
                    let mut coerced_members = HashMap::new();

                    if let Some(target_members) = target_members {
                        for (name, target_type) in target_members {
                            if let Some(current_value) = members.get(name) {
                                coerced_members
                                    .insert(name.clone(), current_value.coerce(target_type)?);
                            } else if !target_type.is_optional() {
                                return Err(WdlError::validation_error(
                                    SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                                    format!("Missing required struct member: {}", name),
                                ));
                            } else {
                                // Optional member missing - insert null
                                coerced_members.insert(name.clone(), Value::null());
                            }
                        }
                    } else {
                        // For StructInstance with no specific members (like plain Object type),
                        // preserve all original members
                        coerced_members = members.clone();
                    }

                    Value::struct_value(
                        desired_type.clone(),
                        coerced_members,
                        Some(extra_keys.clone()),
                    )
                }
                Type::Object {
                    members: target_members,
                } => {
                    // Coerce to Object type
                    let mut coerced_members = HashMap::new();

                    if target_members.is_empty() {
                        // For plain Object type with no specific members, preserve all original members
                        coerced_members = members.clone();
                    } else {
                        // For Object with specific member constraints, enforce them
                        for (name, target_type) in target_members {
                            if let Some(current_value) = members.get(name) {
                                coerced_members
                                    .insert(name.clone(), current_value.coerce(target_type)?);
                            } else if !target_type.is_optional() {
                                return Err(WdlError::validation_error(
                                    SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                                    format!("Missing required object member: {}", name),
                                ));
                            } else {
                                // Optional member missing - insert null
                                coerced_members.insert(name.clone(), Value::null());
                            }
                        }
                    }

                    Value::struct_value(
                        desired_type.clone(),
                        coerced_members,
                        Some(extra_keys.clone()),
                    )
                }
                // Handle coercion to String (struct representation)
                Type::String { .. } => self.coerce_base(desired_type),
                // Other coercions fall back to base
                _ => self.coerce_base(desired_type),
            }
        } else {
            unreachable!("coerce_struct called on non-struct value")
        }
    }
}

impl fmt::Display for Value {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Value::Null => write!(f, "None"),
            Value::Boolean { value, .. } => write!(f, "{}", if *value { "true" } else { "false" }),
            Value::Int { value, .. } => write!(f, "{}", value),
            Value::Float { value, .. } => write!(f, "{:.6}", value),
            Value::String { value, .. }
            | Value::File { value, .. }
            | Value::Directory { value, .. } => write!(f, "\"{}\"", value),
            Value::Array { values, .. } => {
                let items: Vec<String> = values.iter().map(|v| v.to_string()).collect();
                write!(f, "[{}]", items.join(", "))
            }
            Value::Map { pairs, .. } => {
                let items: Vec<String> =
                    pairs.iter().map(|(k, v)| format!("{}: {}", k, v)).collect();
                write!(f, "{{{}}}", items.join(", "))
            }
            Value::Pair { left, right, .. } => {
                write!(f, "({}, {})", left, right)
            }
            Value::Struct { members, .. } => {
                let items: Vec<String> = members
                    .iter()
                    .map(|(k, v)| format!("{}: {}", k, v))
                    .collect();
                write!(f, "{{{}}}", items.join(", "))
            }
        }
    }
}

/// Utility functions for working with values
impl Value {
    /// Check if this value equals another value
    pub fn equals(&self, other: &Value) -> Result<bool, WdlError> {
        if !self.wdl_type().equatable(other.wdl_type(), false) {
            return Err(WdlError::validation_error(
                SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                format!(
                    "Cannot compare {} with {}",
                    self.wdl_type(),
                    other.wdl_type()
                ),
            ));
        }

        Ok(match (self, other) {
            (Value::Null, Value::Null) => true,
            (Value::Boolean { value: a, .. }, Value::Boolean { value: b, .. }) => a == b,
            (Value::Int { value: a, .. }, Value::Int { value: b, .. }) => a == b,
            (Value::Float { value: a, .. }, Value::Float { value: b, .. }) => a == b,
            (Value::Int { value: a, .. }, Value::Float { value: b, .. }) => (*a as f64) == *b,
            (Value::Float { value: a, .. }, Value::Int { value: b, .. }) => *a == (*b as f64),
            (Value::String { value: a, .. }, Value::String { value: b, .. })
            | (Value::File { value: a, .. }, Value::File { value: b, .. })
            | (Value::Directory { value: a, .. }, Value::Directory { value: b, .. }) => a == b,
            (Value::Array { values: a, .. }, Value::Array { values: b, .. }) => {
                if a.len() != b.len() {
                    false
                } else {
                    a.iter()
                        .zip(b.iter())
                        .all(|(x, y)| x.equals(y).unwrap_or(false))
                }
            }
            (
                Value::Pair {
                    left: a_left,
                    right: a_right,
                    ..
                },
                Value::Pair {
                    left: b_left,
                    right: b_right,
                    ..
                },
            ) => a_left.equals(b_left).unwrap_or(false) && a_right.equals(b_right).unwrap_or(false),
            (Value::Map { pairs: a, .. }, Value::Map { pairs: b, .. }) => {
                if a.len() != b.len() {
                    false
                } else {
                    // For maps, we need to compare key-value pairs
                    // Since the order might be different, we need a more sophisticated comparison
                    // For now, let's do a simple ordered comparison
                    a.iter().zip(b.iter()).all(|(a_pair, b_pair)| {
                        let (a_key, a_val) = a_pair;
                        let (b_key, b_val) = b_pair;
                        a_key.equals(b_key).unwrap_or(false) && a_val.equals(b_val).unwrap_or(false)
                    })
                }
            }
            _ => false,
        })
    }

    /// Get the raw value for primitive types
    pub fn as_bool(&self) -> Option<bool> {
        match self {
            Value::Boolean { value, .. } => Some(*value),
            _ => None,
        }
    }

    pub fn as_int(&self) -> Option<i64> {
        match self {
            Value::Int { value, .. } => Some(*value),
            _ => None,
        }
    }

    pub fn as_float(&self) -> Option<f64> {
        match self {
            Value::Float { value, .. } => Some(*value),
            Value::Int { value, .. } => Some(*value as f64),
            _ => None,
        }
    }

    pub fn as_string(&self) -> Option<&str> {
        match self {
            Value::String { value, .. }
            | Value::File { value, .. }
            | Value::Directory { value, .. } => Some(value),
            _ => None,
        }
    }

    pub fn as_array(&self) -> Option<&[Value]> {
        match self {
            Value::Array { values, .. } => Some(values),
            _ => None,
        }
    }

    pub fn as_struct(&self) -> Option<&HashMap<String, Value>> {
        match self {
            Value::Struct { members, .. } => Some(members),
            _ => None,
        }
    }

    /// Check if this is a null value
    pub fn is_null(&self) -> bool {
        matches!(self, Value::Null)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_value_creation() {
        let bool_val = Value::boolean(true);
        assert_eq!(bool_val.as_bool(), Some(true));

        let int_val = Value::int(42);
        assert_eq!(int_val.as_int(), Some(42));

        let float_val = Value::float(std::f64::consts::PI);
        assert_eq!(float_val.as_float(), Some(std::f64::consts::PI));

        let str_val = Value::string("hello".to_string());
        assert_eq!(str_val.as_string(), Some("hello"));
    }

    #[test]
    fn test_json_conversion() {
        let int_val = Value::int(42);
        assert_eq!(int_val.to_json(), json!(42));

        let array_val = Value::array(
            Type::int(false),
            vec![Value::int(1), Value::int(2), Value::int(3)],
        );
        assert_eq!(array_val.to_json(), json!([1, 2, 3]));

        let null_val = Value::null();
        assert_eq!(null_val.to_json(), json!(null));
    }

    #[test]
    fn test_from_json() {
        let json_val = json!({"name": "test", "count": 42, "active": true});
        let value = Value::from_json(json_val);

        if let Value::Struct { members, .. } = value {
            assert_eq!(members.get("name").unwrap().as_string(), Some("test"));
            assert_eq!(members.get("count").unwrap().as_int(), Some(42));
            assert_eq!(members.get("active").unwrap().as_bool(), Some(true));
        } else {
            panic!("Expected struct value");
        }
    }

    #[test]
    fn test_coercion() {
        let int_val = Value::int(42);
        let float_type = Type::float(false);

        let coerced = int_val.coerce(&float_type).unwrap();
        assert_eq!(coerced.as_float(), Some(42.0));

        // String to Int
        let str_val = Value::string("123".to_string());
        let int_type = Type::int(false);
        let coerced_int = str_val.coerce(&int_type).unwrap();
        assert_eq!(coerced_int.as_int(), Some(123));

        // Invalid string to int should fail
        let bad_str = Value::string("not_a_number".to_string());
        assert!(bad_str.coerce(&int_type).is_err());
    }

    #[test]
    fn test_array_coercion() {
        let int_arr = Value::array(Type::int(false), vec![Value::int(1), Value::int(2)]);

        let float_arr_type = Type::array(Type::float(false), false, false);
        let coerced = int_arr.coerce(&float_arr_type).unwrap();

        if let Value::Array { values, .. } = coerced {
            assert_eq!(values[0].as_float(), Some(1.0));
            assert_eq!(values[1].as_float(), Some(2.0));
        } else {
            panic!("Expected array");
        }
    }
    #[test]
    fn test_array_string_coercion() {
        let string_arr = Value::array(
            Type::string(false),
            vec![
                Value::string("string1".to_string()),
                Value::string("string2".to_string()),
            ],
        );

        let string_arr_type = Type::array(Type::string(false), false, false);
        let coerced = string_arr.coerce(&string_arr_type).unwrap();
        if let Value::Array { values, .. } = coerced {
            assert_eq!(values[0].as_string(), Some("string1"));
            assert_eq!(values[1].as_string(), Some("string2"));
        } else {
            panic!("Expected array");
        }
    }

    #[test]
    fn test_equality() {
        let int1 = Value::int(42);
        let int2 = Value::int(42);
        let int3 = Value::int(43);

        assert!(int1.equals(&int2).unwrap());
        assert!(!int1.equals(&int3).unwrap());

        // Int and Float equality
        let float_val = Value::float(42.0);
        assert!(int1.equals(&float_val).unwrap());
    }

    #[test]
    fn test_string_coercion() {
        let int_val = Value::int(42);
        let str_type = Type::string(false);

        let str_val = int_val.coerce(&str_type).unwrap();
        assert_eq!(str_val.as_string(), Some("42"));

        let bool_val = Value::boolean(true);
        let bool_str = bool_val.coerce(&str_type).unwrap();
        assert_eq!(bool_str.as_string(), Some("true"));
    }

    #[test]
    fn test_null_coercion() {
        let null_val = Value::null();
        let optional_int = Type::int(true);
        let non_optional_int = Type::int(false);

        // Null should coerce to optional types
        assert!(null_val.coerce(&optional_int).is_ok());

        // But not to non-optional types
        assert!(null_val.coerce(&non_optional_int).is_err());
    }

    #[test]
    fn test_file_creation() {
        let file_val = Value::file("test.txt".to_string()).unwrap();
        assert_eq!(file_val.as_string(), Some("test.txt"));

        // File with trailing slash should fail
        assert!(Value::file("test/".to_string()).is_err());
    }

    #[test]
    fn test_pair_value() {
        let pair_val = Value::pair(
            Type::int(false),
            Type::string(false),
            Value::int(42),
            Value::string("hello".to_string()),
        );

        let json_repr = pair_val.to_json();
        assert_eq!(json_repr["left"], json!(42));
        assert_eq!(json_repr["right"], json!("hello"));
    }

    #[test]
    fn test_display_formatting() {
        let int_val = Value::int(42);
        assert_eq!(format!("{}", int_val), "42");

        let str_val = Value::string("hello".to_string());
        assert_eq!(format!("{}", str_val), "\"hello\"");

        let null_val = Value::null();
        assert_eq!(format!("{}", null_val), "None");
    }
}
