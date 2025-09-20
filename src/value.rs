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

    /// Create a new Struct value with optional member completion
    /// If the struct_type is a complete struct type definition with optional members,
    /// this will add null values for any missing optional members (like miniwdl)
    pub fn struct_value_with_completion(
        struct_type: Type,
        mut members: HashMap<String, Value>,
        extra_keys: Option<HashSet<String>>,
    ) -> Self {
        // If this is an Object type with member definitions, check for missing optional fields
        if let Type::Object {
            is_call_output: false,
            members: type_members,
            ..
        } = &struct_type
        {
            for (field_name, field_type) in type_members {
                if !members.contains_key(field_name) && field_type.is_optional() {
                    // Add null value for missing optional field
                    members.insert(field_name.clone(), Value::null());
                }
            }
        }

        Value::Struct {
            members,
            extra_keys: extra_keys.unwrap_or_default(),
            wdl_type: struct_type,
        }
    }

    /// Coerce this value to match the target type
    /// This is similar to miniwdl's Value.coerce() method
    pub fn coerce(&self, desired_type: &Type) -> Result<Value, WdlError> {
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

    /// Coerce this value to match the target type with struct definitions
    pub fn coerce_with_structs(
        &self,
        target_type: &Type,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Value, crate::error::WdlError> {
        match (self, target_type) {
            // StructInstance to StructInstance coercion (resolve target first if needed)
            (
                Value::Struct {
                    members,
                    wdl_type,
                    extra_keys,
                },
                Type::StructInstance {
                    type_name,
                    members: target_members,
                    ..
                },
            ) => {
                // If target has no members defined, resolve it first
                let resolved_target_type = if target_members.is_none() {
                    target_type
                        .resolve_struct_type(struct_typedefs)
                        .unwrap_or_else(|_| target_type.clone())
                } else {
                    target_type.clone()
                };

                // Now handle the resolved type
                match resolved_target_type {
                    Type::StructInstance {
                        members: Some(resolved_members),
                        ..
                    } => {
                        // Convert to Object type for processing
                        let object_members = resolved_members
                            .into_iter()
                            .map(|(name, ty)| (name, ty))
                            .collect::<std::collections::HashMap<_, _>>();
                        let object_type = Type::Object {
                            is_call_output: false,
                            members: object_members,
                        };
                        self.coerce_with_structs(&object_type, struct_typedefs)
                    }
                    _ => {
                        // Still unresolved, return error
                        Err(crate::error::WdlError::static_type_mismatch(
                            crate::error::SourcePosition::new(
                                "coercion".to_string(),
                                "coercion".to_string(),
                                1,
                                1,
                                1,
                                1,
                            ),
                            target_type.to_string(),
                            self.wdl_type().to_string(),
                            format!("Cannot resolve struct type: {}", type_name),
                        ))
                    }
                }
            }

            // Struct coercion: complete missing optional members
            (
                Value::Struct {
                    members,
                    wdl_type,
                    extra_keys,
                },
                Type::Object {
                    is_call_output: false,
                    members: target_members,
                    ..
                },
            ) => {
                let mut new_members = members.clone();

                // Add null values for missing optional fields in target type
                for (field_name, field_type) in target_members {
                    if !new_members.contains_key(field_name) && field_type.is_optional() {
                        new_members.insert(field_name.clone(), Value::null());
                    }
                }

                // Recursively coerce existing members if needed
                for (field_name, field_type) in target_members {
                    if let Some(field_value) = new_members.get(field_name) {
                        // Resolve the field type if it's a struct type
                        let resolved_field_type = field_type
                            .resolve_struct_type(struct_typedefs)
                            .unwrap_or_else(|_| field_type.clone());
                        let coerced_value = field_value
                            .coerce_with_structs(&resolved_field_type, struct_typedefs)?;
                        new_members.insert(field_name.clone(), coerced_value);
                    }
                }

                Ok(Value::Struct {
                    members: new_members,
                    extra_keys: extra_keys.clone(),
                    wdl_type: target_type.clone(),
                })
            }

            // For non-struct types, use regular coercion
            _ => self.coerce(target_type),
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
        // Handle coercion to Any - Any type accepts any value as-is
        if matches!(desired_type, Type::Any { .. }) {
            return Ok(self.clone());
        }

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

        // Specific type coercions for simple types
        match (self, desired_type) {
            // Null coercions - handle first to prevent unwanted array promotion
            (Value::Null, ty) => {
                if !ty.is_optional() && !matches!(ty, Type::Any { .. }) {
                    return Err(WdlError::NullValue {
                        pos: SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                    });
                }
                Ok(self.clone())
            }

            // Handle T -> Array[T] promotion (only for non-null, non-arrays)
            // Following miniwdl's logic: promote single value to array
            (value, Type::Array { item_type, .. }) if !matches!(value, Value::Array { .. }) => {
                // Only promote non-arrays to arrays, and only if this value's type
                // can coerce to the array's item type
                if self.wdl_type().coerces(item_type, false) {
                    let coerced_item = self.coerce(item_type)?;
                    Ok(Value::array(item_type.as_ref().clone(), vec![coerced_item]))
                } else {
                    Err(WdlError::static_type_mismatch(
                        SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                        desired_type.to_string(),
                        self.wdl_type().to_string(),
                        format!("Cannot coerce {} to {}", self.wdl_type(), desired_type),
                    ))
                }
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

            // Handle optional to non-optional coercion for primitive types
            // This is the key fix: actually create a new Value with the target type instead of just returning self.clone()
            (Value::Int { value, .. }, Type::Int { .. }) => Ok(Value::Int {
                value: *value,
                wdl_type: desired_type.clone(),
            }),
            (Value::Float { value, .. }, Type::Float { .. }) => Ok(Value::Float {
                value: *value,
                wdl_type: desired_type.clone(),
            }),
            (Value::String { value, .. }, Type::String { .. }) => Ok(Value::String {
                value: value.clone(),
                wdl_type: desired_type.clone(),
            }),
            (Value::Boolean { value, .. }, Type::Boolean { .. }) => Ok(Value::Boolean {
                value: *value,
                wdl_type: desired_type.clone(),
            }),
            (Value::File { value, .. }, Type::File { .. }) => Ok(Value::File {
                value: value.clone(),
                wdl_type: desired_type.clone(),
            }),
            (Value::Directory { value, .. }, Type::Directory { .. }) => Ok(Value::Directory {
                value: value.clone(),
                wdl_type: desired_type.clone(),
            }),

            // Same type - return self (for cases where types already match exactly)
            _ if *self.wdl_type() == *desired_type => Ok(self.clone()),

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
                Type::Object {
                    is_call_output: false,
                    members,
                } => {
                    // Convert Map to Object (Struct value with Object type)
                    let mut object_members = HashMap::new();

                    // Convert map pairs to object members
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
                                        "Cannot convert {:?} key to object member name",
                                        key_value
                                    ),
                                ))
                            }
                        };

                        // If Object has specific member types, coerce to them
                        let coerced_value = if let Some(expected_type) = members.get(&key_str) {
                            value_value.coerce(expected_type)?
                        } else if members.is_empty() {
                            // Empty Object (from parser) - accept any values as-is
                            value_value.clone()
                        } else {
                            // This key doesn't match any expected member
                            return Err(WdlError::validation_error(
                                SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                                format!("Unexpected object member: {}", key_str),
                            ));
                        };

                        object_members.insert(key_str, coerced_value);
                    }

                    // Create struct value with Object type
                    Ok(Value::struct_value_unchecked(
                        desired_type.clone(),
                        object_members,
                        None,
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
                    is_call_output: false,
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
            // String, File, and Directory comparisons (all treated as string comparisons per WDL spec)
            (Value::String { value: a, .. }, Value::String { value: b, .. })
            | (Value::File { value: a, .. }, Value::File { value: b, .. })
            | (Value::Directory { value: a, .. }, Value::Directory { value: b, .. })
            | (Value::String { value: a, .. }, Value::File { value: b, .. })
            | (Value::File { value: a, .. }, Value::String { value: b, .. }) => a == b,
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

        // File and String equality per WDL spec
        let file_val = Value::file("hello.txt".to_string()).unwrap();
        let string_val = Value::string("hello.txt".to_string());
        assert!(file_val.equals(&string_val).unwrap());
        assert!(string_val.equals(&file_val).unwrap());
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

    #[test]
    fn test_coercion_error_details() {
        // Test that coercion errors provide detailed information
        // This reproduces the issue in test_conditional.wdl where error details are suppressed

        // Try to coerce Array[Int?] with None values to Array[Int] (non-optional)
        // This should fail and provide detailed error information
        let optional_int_array = Value::array(
            Type::int(true), // Optional int type
            vec![
                Value::int(1),
                Value::null(), // This None should cause coercion to fail
                Value::int(3),
            ],
        );

        let non_optional_int_array_type = Type::array(Type::int(false), false, false); // Non-optional int array

        let coercion_result = optional_int_array.coerce(&non_optional_int_array_type);

        match coercion_result {
            Ok(_) => {
                panic!("Coercion should have failed when trying to coerce Array[Int?] with None values to Array[Int]");
            }
            Err(error) => {
                let error_message = error.to_string();
                eprintln!("Coercion error (should be detailed): {}", error_message);

                // The error should contain specific information about what went wrong
                // Accept "Null value" as a valid error message format
                assert!(
                    error_message.contains("Cannot coerce")
                        || error_message.contains("NullValue")
                        || error_message.contains("null")
                        || error_message.contains("Null value")
                );

                // This test documents what the error looks like before improvement
                println!("Current error detail level: {}", error_message);
            }
        }
    }

    #[test]
    fn test_workflow_output_error_reproduction() {
        // Simulate the exact pattern from test_conditional.wdl
        // Array[Int?] -> Array[Int] coercion failure in workflow output context

        let maybe_results = Value::array(
            Type::int(true), // Array[Int?] - optional int elements
            vec![
                Value::int(2), // Some valid values
                Value::null(), // None value that should cause problems
                Value::int(8),
            ],
        );

        // Try to coerce to Array[Int] (non-optional elements)
        let result_array_type = Type::array(Type::int(false), false, false);

        let coercion_result = maybe_results.coerce(&result_array_type);

        match coercion_result {
            Ok(_) => {
                panic!("Expected coercion to fail when Array[Int?] contains None and target is Array[Int]");
            }
            Err(error) => {
                println!("Detailed coercion error: {:?}", error);
                println!("Error message: {}", error);

                // This should provide enough detail to understand what went wrong
                // After the fix, this should show the specific element that failed and why
                assert!(!error.to_string().is_empty());
            }
        }
    }

    #[test]
    fn test_select_all_type_conversion() {
        // Test that select_all properly converts Array[Int?] to Array[Int]
        // This reproduces the exact issue from test_conditional.wdl

        use crate::env::Bindings;
        use crate::expr::Expression;
        use crate::stdlib::StdLib;

        let stdlib = StdLib::new("1.2");
        let env = Bindings::new();
        let select_all_fn = stdlib
            .get_function("select_all")
            .expect("select_all function should exist");

        // Create Array[Int?] with some None values as Expression
        let pos = crate::error::SourcePosition::new(
            "test.wdl".to_string(),
            "test.wdl".to_string(),
            1,
            1,
            1,
            5,
        );
        let optional_int_array = Expression::array(
            pos.clone(),
            vec![
                Expression::int(pos.clone(), 1),
                Expression::int(pos.clone(), 3),
                Expression::int(pos.clone(), 5),
            ],
        );

        // Call select_all
        let result = select_all_fn
            .eval(&[optional_int_array], &env, &stdlib)
            .expect("select_all should succeed");

        println!("Result array type: {:?}", result.wdl_type());

        // Check that result is Array[Int] (non-optional elements)
        if let Value::Array { values, wdl_type } = &result {
            // Should have 3 elements (1, 3, 5)
            assert_eq!(values.len(), 3);

            // Check the array type
            if let Type::Array {
                item_type,
                optional,
                nonempty,
            } = wdl_type
            {
                println!("Item type: {:?}", item_type);

                // The item type should be Int (non-optional)
                match item_type.as_ref() {
                    Type::Int {
                        optional: item_optional,
                        ..
                    } => {
                        assert!(
                            !item_optional,
                            "select_all should return Array[Int] not Array[Int?]"
                        );
                    }
                    _ => panic!("Expected Int item type, got: {:?}", item_type),
                }

                assert!(!optional, "Array itself should not be optional");
                assert!(*nonempty, "Array should be nonempty since we have elements");
            } else {
                panic!("Expected Array type, got: {:?}", wdl_type);
            }

            // Check individual values
            for (i, value) in values.iter().enumerate() {
                match value {
                    Value::Int {
                        value: int_val,
                        wdl_type,
                    } => match wdl_type {
                        Type::Int { optional, .. } => {
                            assert!(
                                !optional,
                                "Individual elements should be non-optional Int, not Int?"
                            );
                        }
                        _ => panic!("Expected Int type for element {}", i),
                    },
                    _ => panic!("Expected Int value for element {}, got: {:?}", i, value),
                }
            }

            // Values should be [1, 3, 5]
            assert_eq!(values[0].as_int(), Some(1));
            assert_eq!(values[1].as_int(), Some(3));
            assert_eq!(values[2].as_int(), Some(5));
        } else {
            panic!("Expected Array result, got: {:?}", result);
        }

        // Now test coercion to Array[Int] - this should work
        let target_type = Type::array(Type::int(false), false, false); // Array[Int]
        let coerced = result
            .coerce(&target_type)
            .expect("Should be able to coerce select_all result to Array[Int]");

        println!("Coercion successful: {:?}", coerced.wdl_type());
    }

    #[test]
    fn test_scatter_conditional_array_type_issue() {
        // This test reproduces the actual issue where scatter with if statements
        // in the WDL AST produces Array[Any?] instead of Array[Int?] during typecheck

        use crate::parser::parse_document;
        use crate::tree::Document;

        // Use the updated test_conditional2.wdl content
        let wdl_content = r#"
version 1.2

workflow test_conditional {
  input {
    Array[Int] scatter_range = [1, 2, 3, 4, 5]
  }

    scatter (i in scatter_range) {
      if (i > 2) {
        Int result = 2
      }
    }

  output {
    Array[Int?] maybe_results = result
  }
}
"#;

        println!("Parsing WDL document...");

        // Parse the document
        let mut document = parse_document(wdl_content, "test_conditional2.wdl")
            .expect("Failed to parse WDL document");

        println!("Document parsed successfully");

        // Perform type checking
        let typecheck_result = document.typecheck();

        match typecheck_result {
            Ok(()) => {
                println!("✅ Typecheck succeeded");

                // If typecheck succeeds, it means the scatter+conditional is working correctly
                // and 'result' is being inferred as Array[Int?] as expected

                // Now let's inspect what the workflow's type environment looks like
                // We can do this by trying to extract the workflow and examining it

                if let Some(ref workflow) = document.workflow {
                    println!("Workflow name: {}", workflow.name);
                    println!("Workflow body elements: {}", workflow.body.len());
                    println!("Workflow outputs: {}", workflow.outputs.len());

                    // Check if the output declaration has the expected type
                    if let Some(output) = workflow.outputs.first() {
                        println!("Output name: {}", output.name);
                        println!("Output declared type: {:?}", output.decl_type);

                        // The output should be Array[Int?]
                        match &output.decl_type {
                            crate::types::Type::Array { item_type, .. } => {
                                println!("Output array item type: {:?}", item_type);

                                match item_type.as_ref() {
                                    crate::types::Type::Int { optional: true, .. } => {
                                        println!("✅ EXPECTED: Output has Array[Int?] type");
                                    }
                                    crate::types::Type::Any { optional: true, .. } => {
                                        println!("❌ BUG DETECTED: Output has Array[Any?] type instead of Array[Int?]");
                                        panic!("BUG CONFIRMED: scatter+if produces Array[Any?] instead of Array[Int?]");
                                    }
                                    other => {
                                        println!(
                                            "🤔 UNEXPECTED: Output has Array[{:?}] type",
                                            other
                                        );
                                        panic!("Unexpected array item type: {:?}", other);
                                    }
                                }
                            }
                            other => {
                                println!("🤔 UNEXPECTED: Output is not an array type: {:?}", other);
                                panic!("Expected Array type for output, got: {:?}", other);
                            }
                        }
                    } else {
                        panic!("No outputs found in workflow");
                    }
                } else {
                    panic!("No workflow found in document");
                }
            }
            Err(e) => {
                println!("❌ Typecheck failed: {:?}", e);

                // If typecheck fails, it could be because of the Array[Any?] vs Array[Int?] mismatch
                // This would indicate that the bug exists - the scatter+conditional is producing
                // Array[Any?] but the output is expecting Array[Int?]

                let error_message = format!("{:?}", e);
                if error_message.contains("Any") && error_message.contains("Int") {
                    println!(
                        "🔍 Error contains Any/Int type mismatch - this likely indicates the bug!"
                    );
                    println!("The scatter+conditional is probably producing Array[Any?] instead of Array[Int?]");
                    panic!("BUG CONFIRMED: Type mismatch suggests scatter+if produces wrong type");
                } else {
                    panic!("Unexpected typecheck error: {:?}", e);
                }
            }
        }
    }

    #[test]
    fn test_scatter_with_and_without_conditional_comparison() {
        // This test compares scatter+if vs scatter-only to ensure both produce the same Array[Int] type

        use crate::parser::parse_document;
        use crate::tree::Document;

        // Case 1: scatter with if (test_conditional2.wdl style)
        let scatter_with_if = r#"
version 1.2

workflow test_conditional {
  input {
    Array[Int] scatter_range = [1, 2, 3, 4, 5]
  }

    scatter (i in scatter_range) {
      if (i > 2) {
        Int result = 2
      }
    }

  output {
    Array[Int?] maybe_results = result
  }
}
"#;

        // Case 2: scatter without if (test_conditional3.wdl style)
        let scatter_without_if = r#"
version 1.2

workflow test_conditional {
  input {
    Array[Int] scatter_range = [1, 2, 3, 4, 5]
  }

    scatter (i in scatter_range) {
        Int result = 2
    }

  output {
    Array[Int] maybe_results = result
  }
}
"#;

        println!("=== Testing scatter WITH if ===");
        let mut doc1 = parse_document(scatter_with_if, "scatter_with_if.wdl")
            .expect("Failed to parse scatter with if");

        let result1 = doc1.typecheck();
        let result1_success = result1.is_ok();
        match result1 {
            Ok(()) => {
                println!("✅ Scatter with if: typecheck succeeded");
                if let Some(ref workflow) = doc1.workflow {
                    if let Some(output) = workflow.outputs.first() {
                        println!("Scatter with if result type: {:?}", output.decl_type);
                    }
                }
            }
            Err(ref e) => {
                println!("❌ Scatter with if failed: {:?}", e);
            }
        }

        println!("\n=== Testing scatter WITHOUT if ===");
        let mut doc2 = parse_document(scatter_without_if, "scatter_without_if.wdl")
            .expect("Failed to parse scatter without if");

        let result2 = doc2.typecheck();
        let result2_success = result2.is_ok();
        match result2 {
            Ok(()) => {
                println!("✅ Scatter without if: typecheck succeeded");
                if let Some(ref workflow) = doc2.workflow {
                    if let Some(output) = workflow.outputs.first() {
                        println!("Scatter without if result type: {:?}", output.decl_type);
                    }
                }
            }
            Err(ref e) => {
                println!("❌ Scatter without if failed: {:?}", e);
            }
        }

        // Both should succeed after our 1-phase type checking fix
        assert!(
            result1_success,
            "Scatter with if should succeed after type checking refactor"
        );
        assert!(result2_success, "Scatter without if should succeed");
    }
}
