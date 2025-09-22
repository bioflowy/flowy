//! WDL Map standard library functions
//!
//! This module provides map manipulation functions similar to miniwdl's Map functions.

use crate::error::WdlError;
use crate::expr::ExpressionBase;
use crate::stdlib::Function;
use crate::types::Type;
use crate::value::{Value, ValueBase};
use indexmap::IndexMap;
use std::collections::HashMap;

/// Keys function implementation
///
/// Returns all keys from a Map, Struct, or Object as an Array.
/// Signature: keys(Map[K, V]) -> Array[K]
///           keys(Struct|Object) -> Array[String]
pub struct KeysFunction;

impl crate::stdlib::Function for KeysFunction {
    fn name(&self) -> &str {
        "keys"
    }

    fn infer_type(
        &self,
        args: &mut [crate::expr::Expression],
        type_env: &crate::env::Bindings<crate::types::Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::RuntimeError {
                message: format!("keys(): expected 1 argument, got {}", args.len()),
            });
        }

        let arg_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        match &arg_type {
            Type::Map { key_type, .. } => {
                // Return Array[K] where K is the key type
                Ok(Type::array(key_type.as_ref().clone(), false, false))
            }
            Type::StructInstance { .. } | Type::Object { .. } => {
                // For Struct and Object, keys are always strings
                Ok(Type::array(Type::string(false), false, false))
            }
            _ => Err(WdlError::RuntimeError {
                message: format!(
                    "keys(): argument must be a Map, Struct, or Object, got {}",
                    arg_type
                ),
            }),
        }
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        env: &crate::env::Bindings<crate::value::Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::RuntimeError {
                message: format!("keys(): expected 1 argument, got {}", args.len()),
            });
        }

        let arg_value = args[0].eval(env, stdlib)?;

        match &arg_value {
            Value::Map { pairs, .. } => {
                let mut keys = Vec::new();
                for (key, _) in pairs {
                    keys.push(key.clone());
                }

                // Infer the key type from the first key, or use the map's key type
                let key_type = if let Some(first_key) = keys.first() {
                    first_key.wdl_type().clone()
                } else {
                    Type::string(false) // Default to String if empty map
                };

                Ok(Value::array(key_type, keys))
            }
            Value::Struct { members, .. } => {
                // For structs, return field names as string keys
                let mut keys = Vec::new();
                for field_name in members.keys() {
                    keys.push(Value::string(field_name.clone()));
                }

                // Sort keys for consistent ordering (like struct definition order)
                keys.sort_by(|a, b| {
                    let a_str = a.as_string().unwrap_or("");
                    let b_str = b.as_string().unwrap_or("");
                    a_str.cmp(b_str)
                });

                Ok(Value::array(Type::string(false), keys))
            }
            _ => Err(WdlError::RuntimeError {
                message: format!(
                    "keys(): argument must be a Map, Struct, or Object, got {}",
                    arg_value.wdl_type()
                ),
            }),
        }
    }
}

/// Create the keys function
pub fn create_keys_function() -> Box<dyn crate::stdlib::Function> {
    Box::new(KeysFunction)
}

/// AsPairs function implementation
///
/// Converts a Map into an Array of Pairs.
/// Signature: as_pairs(Map[K, V]) -> Array[Pair[K, V]]
pub struct AsPairsFunction;

impl crate::stdlib::Function for AsPairsFunction {
    fn name(&self) -> &str {
        "as_pairs"
    }

    fn infer_type(
        &self,
        args: &mut [crate::expr::Expression],
        type_env: &crate::env::Bindings<crate::types::Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::RuntimeError {
                message: format!("as_pairs(): expected 1 argument, got {}", args.len()),
            });
        }

        let arg_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        match &arg_type {
            Type::Map {
                key_type,
                value_type,
                ..
            } => {
                // Return Array[Pair[K, V]]
                let pair_type = Type::pair(
                    key_type.as_ref().clone(),
                    value_type.as_ref().clone(),
                    false,
                );
                Ok(Type::array(pair_type, false, false))
            }
            _ => Err(WdlError::RuntimeError {
                message: "as_pairs(): argument must be a Map".to_string(),
            }),
        }
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        env: &crate::env::Bindings<crate::value::Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::RuntimeError {
                message: format!("as_pairs(): expected 1 argument, got {}", args.len()),
            });
        }

        let map_value = args[0].eval(env, stdlib)?;
        let pairs = match &map_value {
            Value::Map { pairs, .. } => pairs,
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "as_pairs(): argument must be a Map".to_string(),
                })
            }
        };

        let mut pair_values = Vec::new();
        for (key, value) in pairs {
            let key_type = key.wdl_type().clone();
            let value_type = value.wdl_type().clone();
            let pair_value = Value::pair(key_type, value_type, key.clone(), value.clone());
            pair_values.push(pair_value);
        }

        // Infer the pair type from the first pair, or construct from map type
        let pair_type = if let Some(first_pair) = pair_values.first() {
            first_pair.wdl_type().clone()
        } else {
            // Default pair type for empty map
            Type::pair(Type::string(false), Type::any(), false)
        };

        Ok(Value::array(pair_type, pair_values))
    }
}

/// Create the as_pairs function
pub fn create_as_pairs_function() -> Box<dyn crate::stdlib::Function> {
    Box::new(AsPairsFunction)
}

/// AsMap function implementation
///
/// Converts an Array of Pairs into a Map, requiring unique keys.
/// Signature: as_map(Array[Pair[K, V]]) -> Map[K, V]
pub struct AsMapFunction;

impl crate::stdlib::Function for AsMapFunction {
    fn name(&self) -> &str {
        "as_map"
    }

    fn infer_type(
        &self,
        args: &mut [crate::expr::Expression],
        type_env: &crate::env::Bindings<crate::types::Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::RuntimeError {
                message: format!("as_map(): expected 1 argument, got {}", args.len()),
            });
        }

        let arg_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        match &arg_type {
            Type::Array { item_type, .. } => {
                match item_type.as_ref() {
                    Type::Pair {
                        left_type,
                        right_type,
                        ..
                    } => {
                        // Return Map[K, V] where K is left_type and V is right_type
                        Ok(Type::map(
                            left_type.as_ref().clone(),
                            right_type.as_ref().clone(),
                            false,
                        ))
                    }
                    _ => Err(WdlError::RuntimeError {
                        message: "as_map(): argument must be an Array[Pair]".to_string(),
                    }),
                }
            }
            _ => Err(WdlError::RuntimeError {
                message: "as_map(): argument must be an Array[Pair]".to_string(),
            }),
        }
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        env: &crate::env::Bindings<crate::value::Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::RuntimeError {
                message: format!("as_map(): expected 1 argument, got {}", args.len()),
            });
        }

        let array_value = args[0].eval(env, stdlib)?;
        let array = match array_value.as_array() {
            Some(arr) => arr,
            None => {
                return Err(WdlError::RuntimeError {
                    message: "as_map(): argument must be an Array".to_string(),
                })
            }
        };

        let mut map_pairs = Vec::new();
        let mut seen_keys = HashMap::new();

        for (i, pair_value) in array.iter().enumerate() {
            let (key, value) = match pair_value {
                Value::Pair { left, right, .. } => (left.as_ref(), right.as_ref()),
                _ => {
                    return Err(WdlError::RuntimeError {
                        message: "as_map(): all array elements must be Pairs".to_string(),
                    })
                }
            };

            // Check for duplicate keys using string representation
            let key_str = format!("{:?}", key);
            if let Some(existing_index) = seen_keys.get(&key_str) {
                return Err(WdlError::RuntimeError {
                    message: format!(
                        "as_map(): duplicate key at indices {} and {}",
                        existing_index, i
                    ),
                });
            }
            seen_keys.insert(key_str, i);

            map_pairs.push((key.clone(), value.clone()));
        }

        // Infer map type from first pair if available
        let (key_type, value_type) = if let Some((key, value)) = map_pairs.first() {
            (key.wdl_type().clone(), value.wdl_type().clone())
        } else {
            (Type::string(false), Type::any())
        };

        Ok(Value::map(key_type, value_type, map_pairs))
    }
}

/// Create the as_map function
pub fn create_as_map_function() -> Box<dyn crate::stdlib::Function> {
    Box::new(AsMapFunction)
}

/// CollectByKey function implementation
///
/// Groups values by their keys, creating a Map where each key maps to an Array of values.
/// Signature: collect_by_key(Array[Pair[K, V]]) -> Map[K, Array[V]]
pub struct CollectByKeyFunction;

impl crate::stdlib::Function for CollectByKeyFunction {
    fn name(&self) -> &str {
        "collect_by_key"
    }

    fn infer_type(
        &self,
        args: &mut [crate::expr::Expression],
        type_env: &crate::env::Bindings<crate::types::Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::RuntimeError {
                message: format!("collect_by_key(): expected 1 argument, got {}", args.len()),
            });
        }

        let arg_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        match &arg_type {
            Type::Array { item_type, .. } => {
                match item_type.as_ref() {
                    Type::Pair {
                        left_type,
                        right_type,
                        ..
                    } => {
                        // Return Map[K, Array[V]] where K is left_type and V is right_type
                        let value_array_type =
                            Type::array(right_type.as_ref().clone(), false, false);
                        Ok(Type::map(
                            left_type.as_ref().clone(),
                            value_array_type,
                            false,
                        ))
                    }
                    _ => Err(WdlError::RuntimeError {
                        message: "collect_by_key(): argument must be an Array[Pair]".to_string(),
                    }),
                }
            }
            _ => Err(WdlError::RuntimeError {
                message: "collect_by_key(): argument must be an Array[Pair]".to_string(),
            }),
        }
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        env: &crate::env::Bindings<crate::value::Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::RuntimeError {
                message: format!("collect_by_key(): expected 1 argument, got {}", args.len()),
            });
        }

        let array_value = args[0].eval(env, stdlib)?;
        let array = match array_value.as_array() {
            Some(arr) => arr,
            None => {
                return Err(WdlError::RuntimeError {
                    message: "collect_by_key(): argument must be an Array".to_string(),
                })
            }
        };

        let mut grouped: IndexMap<String, (Value, Vec<Value>)> = IndexMap::new();

        for pair_value in array {
            let (key, value) = match pair_value {
                Value::Pair { left, right, .. } => (left.as_ref(), right.as_ref()),
                _ => {
                    return Err(WdlError::RuntimeError {
                        message: "collect_by_key(): all array elements must be Pairs".to_string(),
                    })
                }
            };

            // Use string representation as HashMap key
            let key_str = format!("{:?}", key);
            grouped
                .entry(key_str)
                .or_insert_with(|| (key.clone(), Vec::new()))
                .1
                .push(value.clone());
        }

        // Convert grouped HashMap to map pairs
        let mut map_pairs = Vec::new();
        let (key_type, value_type) =
            if let Some((_, (first_key, first_values))) = grouped.iter().next() {
                let value_item_type = if let Some(first_value) = first_values.first() {
                    first_value.wdl_type().clone()
                } else {
                    Type::any()
                };
                (first_key.wdl_type().clone(), value_item_type)
            } else {
                (Type::string(false), Type::any())
            };

        for (_, (key, values)) in grouped {
            let value_array = Value::array(value_type.clone(), values);
            map_pairs.push((key, value_array));
        }

        let array_value_type = Type::array(value_type, false, false);
        Ok(Value::map(key_type, array_value_type, map_pairs))
    }
}

/// Create the collect_by_key function
pub fn create_collect_by_key_function() -> Box<dyn crate::stdlib::Function> {
    Box::new(CollectByKeyFunction)
}

/// ContainsKey function implementation
///
/// Tests whether a Map contains an entry with the given key.
/// Signature: contains_key(Map[P, Y], P) -> Boolean
pub struct ContainsKeyFunction;

impl crate::stdlib::Function for ContainsKeyFunction {
    fn name(&self) -> &str {
        "contains_key"
    }

    fn infer_type(
        &self,
        args: &mut [crate::expr::Expression],
        type_env: &crate::env::Bindings<crate::types::Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::RuntimeError {
                message: format!("contains_key(): expected 2 arguments, got {}", args.len()),
            });
        }

        let map_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        let key_type = args[1].infer_type(type_env, stdlib, struct_typedefs)?;

        match &map_type {
            Type::Map {
                key_type: map_key_type,
                ..
            } => {
                // Check if the key type is compatible with the map's key type
                if !key_type.coerces(map_key_type, true) {
                    return Err(WdlError::RuntimeError {
                        message: format!(
                            "contains_key(): key type {} is not compatible with map key type {}",
                            key_type, map_key_type
                        ),
                    });
                }
                Ok(Type::boolean(false))
            }
            _ => Err(WdlError::RuntimeError {
                message: "contains_key(): first argument must be a Map".to_string(),
            }),
        }
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        env: &crate::env::Bindings<crate::value::Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::RuntimeError {
                message: format!("contains_key(): expected 2 arguments, got {}", args.len()),
            });
        }

        let map_value = args[0].eval(env, stdlib)?;
        let key_value = args[1].eval(env, stdlib)?;

        let pairs = match &map_value {
            Value::Map { pairs, .. } => pairs,
            Value::Null => {
                return Ok(Value::boolean(false));
            }
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "contains_key(): first argument must be a Map".to_string(),
                })
            }
        };

        // Check if the key exists in the map
        for (map_key, _) in pairs {
            // Use string representation for comparison (same as collect_by_key)
            if format!("{:?}", map_key) == format!("{:?}", key_value) {
                return Ok(Value::boolean(true));
            }
        }

        Ok(Value::boolean(false))
    }
}

/// Create the contains_key function
pub fn create_contains_key_function() -> Box<dyn crate::stdlib::Function> {
    Box::new(ContainsKeyFunction)
}

/// Values function implementation
///
/// Returns an Array of the values from a Map.
/// Signature: values(Map[P, Y]) -> Array[Y]
pub struct ValuesFunction;

impl crate::stdlib::Function for ValuesFunction {
    fn name(&self) -> &str {
        "values"
    }

    fn infer_type(
        &self,
        args: &mut [crate::expr::Expression],
        type_env: &crate::env::Bindings<crate::types::Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::RuntimeError {
                message: format!("values(): expected 1 argument, got {}", args.len()),
            });
        }

        let arg_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        match &arg_type {
            Type::Map { value_type, .. } => {
                // Return Array[Y] where Y is the value type
                Ok(Type::array(value_type.as_ref().clone(), false, false))
            }
            _ => Err(WdlError::RuntimeError {
                message: "values(): argument must be a Map".to_string(),
            }),
        }
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        env: &crate::env::Bindings<crate::value::Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::RuntimeError {
                message: format!("values(): expected 1 argument, got {}", args.len()),
            });
        }

        let map_value = args[0].eval(env, stdlib)?;
        let pairs = match &map_value {
            Value::Map { pairs, .. } => pairs,
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "values(): argument must be a Map".to_string(),
                })
            }
        };

        let mut values = Vec::new();
        for (_, value) in pairs {
            values.push(value.clone());
        }

        // Infer the value type from the first value, or use the map's value type
        let value_type = if let Some(first_value) = values.first() {
            first_value.wdl_type().clone()
        } else {
            Type::any() // Default for empty map
        };

        Ok(Value::array(value_type, values))
    }
}

/// Create the values function
pub fn create_values_function() -> Box<dyn crate::stdlib::Function> {
    Box::new(ValuesFunction)
}
