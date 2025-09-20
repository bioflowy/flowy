//! Array manipulation functions for WDL standard library
//!
//! This module provides array manipulation functions as defined in the WDL specification.

use crate::error::WdlError;
use crate::expr::ExpressionBase;
use crate::stdlib::create_static_function;
use crate::types::{unify_types, Type};
use crate::value::Value;
use crate::value::ValueBase;

/// Create the prefix function
///
/// Prepends a prefix string to each element in a string array.
///
/// **Parameters**
/// 1. `String`: the prefix to prepend
/// 2. `Array[String]`: the array of strings
///
/// **Returns**: Array[String] with prefix prepended to each element
pub fn create_prefix_function() -> Box<dyn crate::stdlib::Function> {
    create_static_function(
        "prefix".to_string(),
        vec![
            Type::string(false),
            Type::array(Type::string(false), false, false),
        ], // prefix, array
        Type::array(Type::string(false), false, false), // returns Array[String]
        |args: &[Value]| -> Result<Value, WdlError> {
            let prefix = args[0].as_string().unwrap();
            let array = args[1].as_array().unwrap();

            let mut result_values = Vec::new();
            for value in array {
                match value.as_string() {
                    Some(s) => {
                        result_values.push(Value::string(format!("{}{}", prefix, s)));
                    }
                    None => {
                        return Err(WdlError::RuntimeError {
                            message: "prefix(): array must contain only String values".to_string(),
                        });
                    }
                }
            }

            Ok(Value::array(Type::string(false), result_values))
        },
    )
}

/// Create the suffix function
///
/// Appends a suffix string to each element in a string array.
///
/// **Parameters**
/// 1. `String`: the suffix to append
/// 2. `Array[String]`: the array of strings
///
/// **Returns**: Array[String] with suffix appended to each element
pub fn create_suffix_function() -> Box<dyn crate::stdlib::Function> {
    create_static_function(
        "suffix".to_string(),
        vec![
            Type::string(false),
            Type::array(Type::string(false), false, false),
        ], // suffix, array
        Type::array(Type::string(false), false, false), // returns Array[String]
        |args: &[Value]| -> Result<Value, WdlError> {
            let suffix = args[0].as_string().unwrap();
            let array = args[1].as_array().unwrap();

            let mut result_values = Vec::new();
            for value in array {
                match value.as_string() {
                    Some(s) => {
                        result_values.push(Value::string(format!("{}{}", s, suffix)));
                    }
                    None => {
                        return Err(WdlError::RuntimeError {
                            message: "suffix(): array must contain only String values".to_string(),
                        });
                    }
                }
            }

            Ok(Value::array(Type::string(false), result_values))
        },
    )
}

/// Create the length function
///
/// Returns the number of elements in an array, map, object, or string.
///
/// **Parameters**
/// 1. `Array[X]|Map[X, Y]|Object|String`: the value to measure
///
/// **Returns**: Int representing the number of elements/characters
pub fn create_length_function() -> Box<dyn crate::stdlib::Function> {
    Box::new(LengthFunction)
}

/// Length function implementation that supports Array, Map, Object, and String
struct LengthFunction;

impl crate::stdlib::Function for LengthFunction {
    fn name(&self) -> &str {
        "length"
    }

    fn infer_type(
        &self,
        args: &mut [crate::expr::Expression],
        type_env: &crate::env::Bindings<Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        // Check argument count
        if args.len() != 1 {
            let pos = if args.is_empty() {
                crate::error::SourcePosition::new(
                    "unknown".to_string(),
                    "unknown".to_string(),
                    0,
                    0,
                    0,
                    0,
                )
            } else {
                args[0].source_position().clone()
            };
            return Err(WdlError::Validation {
                pos,
                message: format!("Function 'length' expects 1 argument, got {}", args.len()),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        // Infer the type of the argument
        let arg_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;

        // Check if the argument type is supported
        match &arg_type {
            Type::Array { .. }
            | Type::Map { .. }
            | Type::StructInstance { .. }
            | Type::Object { .. }
            | Type::String { .. } => Ok(Type::int(false)),
            other => Err(WdlError::Validation {
                pos: args[0].source_position().clone(),
                message: format!(
                    "Function 'length' argument 1 expects type Array[Any]|Map[Any,Any]|Object|String, got {}",
                    other
                ),
                source_text: None,
                declared_wdl_version: None,
            }),
        }
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        env: &crate::env::Bindings<Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::RuntimeError {
                message: format!("Function 'length' expects 1 argument, got {}", args.len()),
            });
        }

        // Evaluate the argument
        let arg_value = args[0].eval(env, stdlib)?;

        // Calculate length based on value type
        match &arg_value {
            Value::Array { values, .. } => Ok(Value::int(values.len() as i64)),
            Value::Map { pairs, .. } => Ok(Value::int(pairs.len() as i64)),
            Value::Struct { members, .. } => Ok(Value::int(members.len() as i64)),
            Value::String { value, .. } => Ok(Value::int(value.len() as i64)),
            other => Err(WdlError::RuntimeError {
                message: format!(
                    "Function 'length' argument 1 expects type Array[Any]|Map[Any,Any]|Object|String, got {}",
                    other.wdl_type()
                ),
            }),
        }
    }
}

/// Create the range function
///
/// Creates an array of integers from 0 to n-1.
///
/// **Parameters**
/// 1. `Int`: the number of elements (n)
///
/// **Returns**: Array[Int] containing [0, 1, 2, ..., n-1]
pub fn create_range_function() -> Box<dyn crate::stdlib::Function> {
    create_static_function(
        "range".to_string(),
        vec![Type::int(false)],                      // Int
        Type::array(Type::int(false), false, false), // returns Array[Int]
        |args: &[Value]| -> Result<Value, WdlError> {
            let n = args[0].as_int().unwrap();

            if n < 0 {
                return Err(WdlError::RuntimeError {
                    message: "range(): argument must be non-negative".to_string(),
                });
            }

            let mut result_values = Vec::new();
            for i in 0..n {
                result_values.push(Value::int(i));
            }

            Ok(Value::array(Type::int(false), result_values))
        },
    )
}

/// SelectFirst function implementation
///
/// Returns the first non-null element in the array.
/// The return type is inferred from the array's item type.
pub struct SelectFirstFunction;

impl crate::stdlib::Function for SelectFirstFunction {
    fn name(&self) -> &str {
        "select_first"
    }

    fn infer_type(
        &self,
        args: &mut [crate::expr::Expression],
        type_env: &crate::env::Bindings<crate::types::Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        if args.is_empty() || args.len() > 2 {
            return Err(WdlError::RuntimeError {
                message: format!(
                    "select_first(): expected 1 or 2 arguments, got {}",
                    args.len()
                ),
            });
        }

        let array_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        let fallback_type = if args.len() == 2 {
            Some(args[1].infer_type(type_env, stdlib, struct_typedefs)?)
        } else {
            None
        };

        select_first_result_type(&array_type, fallback_type.as_ref())
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        env: &crate::env::Bindings<crate::value::Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        if args.is_empty() || args.len() > 2 {
            return Err(WdlError::RuntimeError {
                message: format!(
                    "select_first(): expected 1 or 2 arguments, got {}",
                    args.len()
                ),
            });
        }

        let array_value = args[0].eval(env, stdlib)?;
        let array = match array_value.as_array() {
            Some(arr) => arr,
            None => {
                return Err(WdlError::RuntimeError {
                    message: "select_first(): argument must be an array".to_string(),
                })
            }
        };

        let fallback_value = if args.len() == 2 {
            Some(args[1].eval(env, stdlib)?)
        } else {
            None
        };

        let array_type = array_value.wdl_type().clone();
        let fallback_type = fallback_value
            .as_ref()
            .map(|value| value.wdl_type().clone());
        let result_type = select_first_result_type(&array_type, fallback_type.as_ref())?;

        for value in array {
            if !value.is_null() {
                return value.coerce(&result_type);
            }
        }

        if let Some(fallback) = fallback_value {
            return fallback.coerce(&result_type);
        }

        Err(WdlError::RuntimeError {
            message: "select_first(): all elements in array are null".to_string(),
        })
    }
}

/// Create the select_first function
pub fn create_select_first_function() -> Box<dyn crate::stdlib::Function> {
    Box::new(SelectFirstFunction)
}

fn select_first_result_type(
    array_type: &Type,
    fallback_type: Option<&Type>,
) -> Result<Type, WdlError> {
    let item_type = match array_type {
        Type::Array { item_type, .. } => item_type.as_ref().clone(),
        _ => {
            return Err(WdlError::RuntimeError {
                message: "select_first(): argument must be an array".to_string(),
            })
        }
    };

    let mut element_type = item_type.clone().with_optional(false);

    if let Some(fallback) = fallback_type {
        let fallback_non_optional = fallback.clone().with_optional(false);

        if !element_type.coerces(&fallback_non_optional, true)
            && !fallback_non_optional.coerces(&element_type, true)
        {
            return Err(WdlError::RuntimeError {
                message: format!(
                    "select_first(): fallback type {} incompatible with array element type {}",
                    fallback, item_type
                ),
            });
        }

        let unified = unify_types(vec![&element_type, &fallback_non_optional], true, false);
        element_type = unified.with_optional(false);
    }

    Ok(element_type)
}

/// Contains function implementation
pub struct ContainsFunction;

impl crate::stdlib::Function for ContainsFunction {
    fn name(&self) -> &str {
        "contains"
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
                message: format!("contains(): expected 2 arguments, got {}", args.len()),
            });
        }

        let array_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        let value_type = args[1].infer_type(type_env, stdlib, struct_typedefs)?;

        // Reuse select_first compatibility logic (ignoring the resulting type)
        select_first_result_type(&array_type, Some(&value_type))?;

        Ok(Type::boolean(false))
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        env: &crate::env::Bindings<crate::value::Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::RuntimeError {
                message: format!("contains(): expected 2 arguments, got {}", args.len()),
            });
        }

        let array_value = args[0].eval(env, stdlib)?;
        let array = match array_value.as_array() {
            Some(arr) => arr,
            None => {
                return Err(WdlError::RuntimeError {
                    message: "contains(): first argument must be an array".to_string(),
                })
            }
        };

        let search_value_raw = args[1].eval(env, stdlib)?;

        let array_type = array_value.wdl_type().clone();
        let array_item_type = match &array_type {
            Type::Array { item_type, .. } => item_type.as_ref().clone(),
            _ => unreachable!("array_value must be an array"),
        };

        let search_value_type = search_value_raw.wdl_type().clone();
        let comparison_type = select_first_result_type(&array_type, Some(&search_value_type))?;

        let allow_null = array_item_type.is_optional();
        let search_is_null = search_value_raw.is_null();

        if search_is_null && !allow_null {
            return Err(WdlError::RuntimeError {
                message: "contains(): cannot search for None in non-optional array".to_string(),
            });
        }

        let search_value = if search_is_null {
            Value::null()
        } else {
            search_value_raw.coerce(&comparison_type)?
        };

        for element in array {
            if element.is_null() {
                if search_is_null {
                    return Ok(Value::boolean(true));
                }
                continue;
            }

            let coerced_element = element.coerce(&comparison_type)?;
            if coerced_element == search_value {
                return Ok(Value::boolean(true));
            }
        }

        Ok(Value::boolean(false))
    }
}

/// Create the contains function
pub fn create_contains_function() -> Box<dyn crate::stdlib::Function> {
    Box::new(ContainsFunction)
}

/// SelectAll function implementation
///
/// Returns a new array with all non-null elements from the input array.
/// The return type is inferred from the array's item type.
pub struct SelectAllFunction;

impl crate::stdlib::Function for SelectAllFunction {
    fn name(&self) -> &str {
        "select_all"
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
                message: format!("select_all(): expected 1 argument, got {}", args.len()),
            });
        }

        let arg_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        match &arg_type {
            Type::Array { item_type, .. } => {
                // Return Array[T] where T is the item type without optional flag
                Ok(Type::array(
                    item_type.as_ref().clone().with_optional(false),
                    false,
                    false,
                ))
            }
            _ => Err(WdlError::RuntimeError {
                message: "select_all(): argument must be an array".to_string(),
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
                message: format!("select_all(): expected 1 argument, got {}", args.len()),
            });
        }

        let array_value = args[0].eval(env, stdlib)?;
        let array = match array_value.as_array() {
            Some(arr) => arr,
            None => {
                return Err(WdlError::RuntimeError {
                    message: "select_all(): argument must be an array".to_string(),
                })
            }
        };

        let mut result_values = Vec::new();
        for value in array {
            if !value.is_null() {
                result_values.push(value.clone());
            }
        }

        // Infer the result type from the first non-null element, or use the input type
        let result_item_type = if let Some(first) = result_values.first() {
            first.wdl_type().clone()
        } else {
            // If all elements are null, use the original item type without optional
            let mut args_copy = args.to_vec();
            let arg_type = args_copy[0].infer_type(&crate::env::Bindings::new(), stdlib, &[])?;
            match &arg_type {
                Type::Array { item_type, .. } => item_type.as_ref().clone().with_optional(false),
                _ => Type::any(),
            }
        };

        Ok(Value::array(result_item_type, result_values))
    }
}

/// Create the select_all function
pub fn create_select_all_function() -> Box<dyn crate::stdlib::Function> {
    Box::new(SelectAllFunction)
}

/// Flatten function implementation
///
/// Converts a 2D array into a 1D array by combining all sub-arrays.
/// The return type is inferred from the nested array's item type.
pub struct FlattenFunction;

impl crate::stdlib::Function for FlattenFunction {
    fn name(&self) -> &str {
        "flatten"
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
                message: format!("flatten(): expected 1 argument, got {}", args.len()),
            });
        }

        let arg_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        match &arg_type {
            Type::Array { item_type, .. } => {
                match item_type.as_ref() {
                    Type::Array {
                        item_type: inner_item_type,
                        ..
                    } => {
                        // Return Array[T] where T is the inner array's item type
                        Ok(Type::array(inner_item_type.as_ref().clone(), false, false))
                    }
                    _ => Err(WdlError::RuntimeError {
                        message: "flatten(): argument must be an array of arrays".to_string(),
                    }),
                }
            }
            _ => Err(WdlError::RuntimeError {
                message: "flatten(): argument must be an array".to_string(),
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
                message: format!("flatten(): expected 1 argument, got {}", args.len()),
            });
        }

        let array_value = args[0].eval(env, stdlib)?;
        let outer_array = match array_value.as_array() {
            Some(arr) => arr,
            None => {
                return Err(WdlError::RuntimeError {
                    message: "flatten(): argument must be an array".to_string(),
                })
            }
        };

        let mut result_values = Vec::new();
        let mut result_item_type = Type::any();

        for value in outer_array {
            match value.as_array() {
                Some(inner_array) => {
                    for inner_value in inner_array {
                        // Use the type of the first inner value as the result type
                        if result_values.is_empty() {
                            result_item_type = inner_value.wdl_type().clone();
                        }
                        result_values.push(inner_value.clone());
                    }
                }
                None => {
                    return Err(WdlError::RuntimeError {
                        message: "flatten(): array must contain only Array elements".to_string(),
                    });
                }
            }
        }

        // If no elements were flattened, infer type from the function signature
        if result_values.is_empty() {
            let mut args_copy = args.to_vec();
            result_item_type =
                match self.infer_type(&mut args_copy, &crate::env::Bindings::new(), stdlib, &[])? {
                    Type::Array { item_type, .. } => item_type.as_ref().clone(),
                    _ => Type::any(),
                };
        }

        Ok(Value::array(result_item_type, result_values))
    }
}

/// Create the flatten function
pub fn create_flatten_function() -> Box<dyn crate::stdlib::Function> {
    Box::new(FlattenFunction)
}

/// Zip function implementation
///
/// Pairs corresponding elements from two arrays.
/// The return type is inferred from the input array types.
pub struct ZipFunction;

impl crate::stdlib::Function for ZipFunction {
    fn name(&self) -> &str {
        "zip"
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
                message: format!("zip(): expected 2 arguments, got {}", args.len()),
            });
        }

        let arg0_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        let arg1_type = args[1].infer_type(type_env, stdlib, struct_typedefs)?;

        let (item_type0, nonempty0) = match &arg0_type {
            Type::Array {
                item_type,
                nonempty,
                ..
            } => (item_type.as_ref().clone(), *nonempty),
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "zip(): first argument must be an array".to_string(),
                })
            }
        };

        let (item_type1, nonempty1) = match &arg1_type {
            Type::Array {
                item_type,
                nonempty,
                ..
            } => (item_type.as_ref().clone(), *nonempty),
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "zip(): second argument must be an array".to_string(),
                })
            }
        };

        // Return Array[Pair[A, B]] where A and B are the item types
        Ok(Type::array(
            Type::pair(item_type0, item_type1, false),
            false,
            nonempty0 || nonempty1, // nonempty if either array is nonempty
        ))
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        env: &crate::env::Bindings<crate::value::Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::RuntimeError {
                message: format!("zip(): expected 2 arguments, got {}", args.len()),
            });
        }

        let array1_value = args[0].eval(env, stdlib)?;
        let array2_value = args[1].eval(env, stdlib)?;

        let array1 = match array1_value.as_array() {
            Some(arr) => arr,
            None => {
                return Err(WdlError::RuntimeError {
                    message: "zip(): first argument must be an array".to_string(),
                })
            }
        };

        let array2 = match array2_value.as_array() {
            Some(arr) => arr,
            None => {
                return Err(WdlError::RuntimeError {
                    message: "zip(): second argument must be an array".to_string(),
                })
            }
        };

        if array1.len() != array2.len() {
            return Err(WdlError::RuntimeError {
                message: format!(
                    "zip(): arrays must have same length, got {} and {}",
                    array1.len(),
                    array2.len()
                ),
            });
        }

        let mut result_values = Vec::new();
        for (left, right) in array1.iter().zip(array2.iter()) {
            let pair = Value::pair(
                left.wdl_type().clone(),
                right.wdl_type().clone(),
                left.clone(),
                right.clone(),
            );
            result_values.push(pair);
        }

        // Infer result item type from the first pair if available
        let result_item_type = if let Some(first) = result_values.first() {
            first.wdl_type().clone()
        } else {
            Type::pair(Type::any(), Type::any(), false)
        };

        Ok(Value::array(result_item_type, result_values))
    }
}

/// Create the zip function
pub fn create_zip_function() -> Box<dyn crate::stdlib::Function> {
    Box::new(ZipFunction)
}

/// Cross function implementation
///
/// Creates all possible pairs between elements of two arrays.
/// The return type is inferred from the input array types.
pub struct CrossFunction;

impl crate::stdlib::Function for CrossFunction {
    fn name(&self) -> &str {
        "cross"
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
                message: format!("cross(): expected 2 arguments, got {}", args.len()),
            });
        }

        let arg0_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        let arg1_type = args[1].infer_type(type_env, stdlib, struct_typedefs)?;

        let (item_type0, nonempty0) = match &arg0_type {
            Type::Array {
                item_type,
                nonempty,
                ..
            } => (item_type.as_ref().clone(), *nonempty),
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "cross(): first argument must be an array".to_string(),
                })
            }
        };

        let (item_type1, nonempty1) = match &arg1_type {
            Type::Array {
                item_type,
                nonempty,
                ..
            } => (item_type.as_ref().clone(), *nonempty),
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "cross(): second argument must be an array".to_string(),
                })
            }
        };

        // Return Array[Pair[A, B]] where A and B are the item types
        Ok(Type::array(
            Type::pair(item_type0, item_type1, false),
            false,
            nonempty0 || nonempty1, // nonempty if either array is nonempty
        ))
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        env: &crate::env::Bindings<crate::value::Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::RuntimeError {
                message: format!("cross(): expected 2 arguments, got {}", args.len()),
            });
        }

        let array1_value = args[0].eval(env, stdlib)?;
        let array2_value = args[1].eval(env, stdlib)?;

        let array1 = match array1_value.as_array() {
            Some(arr) => arr,
            None => {
                return Err(WdlError::RuntimeError {
                    message: "cross(): first argument must be an array".to_string(),
                })
            }
        };

        let array2 = match array2_value.as_array() {
            Some(arr) => arr,
            None => {
                return Err(WdlError::RuntimeError {
                    message: "cross(): second argument must be an array".to_string(),
                })
            }
        };

        let mut result_values = Vec::new();
        for left in array1.iter() {
            for right in array2.iter() {
                let pair = Value::pair(
                    left.wdl_type().clone(),
                    right.wdl_type().clone(),
                    left.clone(),
                    right.clone(),
                );
                result_values.push(pair);
            }
        }

        // Infer result item type from the first pair if available
        let result_item_type = if let Some(first) = result_values.first() {
            first.wdl_type().clone()
        } else {
            Type::pair(Type::any(), Type::any(), false)
        };

        Ok(Value::array(result_item_type, result_values))
    }
}

/// Create the cross function
pub fn create_cross_function() -> Box<dyn crate::stdlib::Function> {
    Box::new(CrossFunction)
}

/// Create the quote function
///
/// Wraps each element in an array with double quotes.
///
/// **Parameters**
/// 1. `Array[T]`: array of elements to quote
///
/// **Returns**: Array[String] with each element wrapped in double quotes
pub fn create_quote_function() -> Box<dyn crate::stdlib::Function> {
    create_static_function(
        "quote".to_string(),
        vec![Type::array(Type::any(), false, false)], // Array[Any]
        Type::array(Type::string(false), false, false), // returns Array[String]
        |args: &[Value]| -> Result<Value, WdlError> {
            let array = args[0].as_array().unwrap();

            let mut result_values = Vec::new();
            for value in array {
                // Convert value to string and wrap with double quotes
                let str_value = match value {
                    Value::String { value, .. } => value.clone(),
                    Value::Int { value, .. } => value.to_string(),
                    Value::Float { value, .. } => value.to_string(),
                    Value::Boolean { value, .. } => value.to_string(),
                    Value::File { value, .. } => value.clone(),
                    Value::Directory { value, .. } => value.clone(),
                    Value::Null => "null".to_string(),
                    _ => {
                        return Err(WdlError::RuntimeError {
                            message: format!(
                                "quote(): cannot convert value to string: {:?}",
                                value
                            ),
                        });
                    }
                };
                result_values.push(Value::string(format!("\"{}\"", str_value)));
            }

            Ok(Value::array(Type::string(false), result_values))
        },
    )
}

/// Create the squote function
///
/// Wraps each element in an array with single quotes.
///
/// **Parameters**
/// 1. `Array[T]`: array of elements to quote
///
/// **Returns**: Array[String] with each element wrapped in single quotes
pub fn create_squote_function() -> Box<dyn crate::stdlib::Function> {
    create_static_function(
        "squote".to_string(),
        vec![Type::array(Type::any(), false, false)], // Array[Any]
        Type::array(Type::string(false), false, false), // returns Array[String]
        |args: &[Value]| -> Result<Value, WdlError> {
            let array = args[0].as_array().unwrap();

            let mut result_values = Vec::new();
            for value in array {
                // Convert value to string and wrap with single quotes
                let str_value = match value {
                    Value::String { value, .. } => value.clone(),
                    Value::Int { value, .. } => value.to_string(),
                    Value::Float { value, .. } => value.to_string(),
                    Value::Boolean { value, .. } => value.to_string(),
                    Value::File { value, .. } => value.clone(),
                    Value::Directory { value, .. } => value.clone(),
                    Value::Null => "null".to_string(),
                    _ => {
                        return Err(WdlError::RuntimeError {
                            message: format!(
                                "squote(): cannot convert value to string: {:?}",
                                value
                            ),
                        });
                    }
                };
                result_values.push(Value::string(format!("'{}'", str_value)));
            }

            Ok(Value::array(Type::string(false), result_values))
        },
    )
}

/// Unzip function implementation
///
/// Separates a list of pairs into two separate arrays.
/// The return type is inferred from the pair types.
pub struct UnzipFunction;

impl crate::stdlib::Function for UnzipFunction {
    fn name(&self) -> &str {
        "unzip"
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
                message: format!("unzip(): expected 1 argument, got {}", args.len()),
            });
        }

        let arg_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        match &arg_type {
            Type::Array {
                item_type,
                nonempty,
                ..
            } => {
                match item_type.as_ref() {
                    Type::Pair {
                        left_type,
                        right_type,
                        ..
                    } => {
                        // Return Pair[Array[A], Array[B]] where A and B are the pair's component types
                        Ok(Type::pair(
                            Type::array(left_type.as_ref().clone(), false, *nonempty),
                            Type::array(right_type.as_ref().clone(), false, *nonempty),
                            false,
                        ))
                    }
                    _ => Err(WdlError::RuntimeError {
                        message: "unzip(): argument must be an array of pairs".to_string(),
                    }),
                }
            }
            _ => Err(WdlError::RuntimeError {
                message: "unzip(): argument must be an array".to_string(),
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
                message: format!("unzip(): expected 1 argument, got {}", args.len()),
            });
        }

        let array_value = args[0].eval(env, stdlib)?;
        let pair_array = match array_value.as_array() {
            Some(arr) => arr,
            None => {
                return Err(WdlError::RuntimeError {
                    message: "unzip(): argument must be an array".to_string(),
                })
            }
        };

        let mut left_values = Vec::new();
        let mut right_values = Vec::new();
        let mut left_type = Type::any();
        let mut right_type = Type::any();

        for value in pair_array {
            match value {
                Value::Pair { left, right, .. } => {
                    // Use types from the first pair
                    if left_values.is_empty() {
                        left_type = left.wdl_type().clone();
                        right_type = right.wdl_type().clone();
                    }
                    left_values.push(left.as_ref().clone());
                    right_values.push(right.as_ref().clone());
                }
                _ => {
                    return Err(WdlError::RuntimeError {
                        message: "unzip(): array must contain only Pair elements".to_string(),
                    });
                }
            }
        }

        let left_array = Value::array(left_type.clone(), left_values);
        let right_array = Value::array(right_type.clone(), right_values);

        Ok(Value::pair(
            Type::array(left_type, false, false),
            Type::array(right_type, false, false),
            left_array,
            right_array,
        ))
    }
}

/// Create the unzip function
pub fn create_unzip_function() -> Box<dyn crate::stdlib::Function> {
    Box::new(UnzipFunction)
}

/// Transpose function implementation
///
/// Transposes a 2D array (matrix) by swapping rows and columns.
/// The return type is inferred from the nested array's item type.
pub struct TransposeFunction;

impl crate::stdlib::Function for TransposeFunction {
    fn name(&self) -> &str {
        "transpose"
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
                message: format!("transpose(): expected 1 argument, got {}", args.len()),
            });
        }

        let arg_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        match &arg_type {
            Type::Array { item_type, .. } => {
                match item_type.as_ref() {
                    Type::Array {
                        item_type: inner_item_type,
                        ..
                    } => {
                        // Return Array[Array[T]] where T is the inner array's item type (same structure)
                        Ok(Type::array(
                            Type::array(inner_item_type.as_ref().clone(), false, false),
                            false,
                            false,
                        ))
                    }
                    _ => Err(WdlError::RuntimeError {
                        message: "transpose(): argument must be an array of arrays".to_string(),
                    }),
                }
            }
            _ => Err(WdlError::RuntimeError {
                message: "transpose(): argument must be an array".to_string(),
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
                message: format!("transpose(): expected 1 argument, got {}", args.len()),
            });
        }

        let array_value = args[0].eval(env, stdlib)?;
        let matrix = match array_value.as_array() {
            Some(arr) => arr,
            None => {
                return Err(WdlError::RuntimeError {
                    message: "transpose(): argument must be an array".to_string(),
                })
            }
        };

        if matrix.is_empty() {
            return Ok(Value::array(
                Type::array(Type::any(), false, false),
                Vec::new(),
            ));
        }

        // Get the first row to determine column count and item type
        let first_row = match matrix[0].as_array() {
            Some(row) => row,
            None => {
                return Err(WdlError::RuntimeError {
                    message: "transpose(): array must contain only Array elements".to_string(),
                })
            }
        };

        let col_count = first_row.len();
        let item_type = if col_count > 0 {
            first_row[0].wdl_type().clone()
        } else {
            Type::any()
        };

        // Validate all rows have the same length
        for (i, row_value) in matrix.iter().enumerate() {
            match row_value.as_array() {
                Some(row) => {
                    if row.len() != col_count {
                        return Err(WdlError::RuntimeError {
                            message: format!("transpose(): ragged input matrix - row {} has {} elements, expected {}", i, row.len(), col_count),
                        });
                    }
                }
                None => {
                    return Err(WdlError::RuntimeError {
                        message: "transpose(): array must contain only Array elements".to_string(),
                    });
                }
            }
        }

        // Transpose the matrix
        let mut transposed_values = Vec::new();
        for col in 0..col_count {
            let mut column_values = Vec::new();
            for row_value in matrix {
                let row = row_value.as_array().unwrap(); // Already validated above
                column_values.push(row[col].clone());
            }
            transposed_values.push(Value::array(item_type.clone(), column_values));
        }

        Ok(Value::array(
            Type::array(item_type, false, false),
            transposed_values,
        ))
    }
}

/// Create the transpose function
pub fn create_transpose_function() -> Box<dyn crate::stdlib::Function> {
    Box::new(TransposeFunction)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::env::Bindings;
    use crate::expr::ExpressionBase;

    #[test]
    fn test_prefix_function() {
        let prefix_fn = create_prefix_function();
        let env = Bindings::new();
        let stdlib = crate::stdlib::StdLib::new("1.2");

        // Test prefix("pre_", ["a", "b", "c"]) -> ["pre_a", "pre_b", "pre_c"]
        let prefix_expr = crate::expr::Expression::string_literal(
            crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1),
            "pre_".to_string(),
        );
        let array_expr = crate::expr::Expression::array(
            crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1),
            vec![
                crate::expr::Expression::string_literal(
                    crate::error::SourcePosition::new(
                        "test".to_string(),
                        "test".to_string(),
                        1,
                        1,
                        1,
                        1,
                    ),
                    "a".to_string(),
                ),
                crate::expr::Expression::string_literal(
                    crate::error::SourcePosition::new(
                        "test".to_string(),
                        "test".to_string(),
                        1,
                        1,
                        1,
                        1,
                    ),
                    "b".to_string(),
                ),
                crate::expr::Expression::string_literal(
                    crate::error::SourcePosition::new(
                        "test".to_string(),
                        "test".to_string(),
                        1,
                        1,
                        1,
                        1,
                    ),
                    "c".to_string(),
                ),
            ],
        );

        let result = prefix_fn
            .eval(&[prefix_expr, array_expr], &env, &stdlib)
            .unwrap();
        let result_array = result.as_array().unwrap();

        assert_eq!(result_array.len(), 3);
        assert_eq!(result_array[0].as_string().unwrap(), "pre_a");
        assert_eq!(result_array[1].as_string().unwrap(), "pre_b");
        assert_eq!(result_array[2].as_string().unwrap(), "pre_c");
    }

    #[test]
    fn test_suffix_function() {
        let suffix_fn = create_suffix_function();
        let env = Bindings::new();
        let stdlib = crate::stdlib::StdLib::new("1.2");

        // Test suffix("_suf", ["a", "b", "c"]) -> ["a_suf", "b_suf", "c_suf"]
        let suffix_expr = crate::expr::Expression::string_literal(
            crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1),
            "_suf".to_string(),
        );
        let array_expr = crate::expr::Expression::array(
            crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1),
            vec![
                crate::expr::Expression::string_literal(
                    crate::error::SourcePosition::new(
                        "test".to_string(),
                        "test".to_string(),
                        1,
                        1,
                        1,
                        1,
                    ),
                    "a".to_string(),
                ),
                crate::expr::Expression::string_literal(
                    crate::error::SourcePosition::new(
                        "test".to_string(),
                        "test".to_string(),
                        1,
                        1,
                        1,
                        1,
                    ),
                    "b".to_string(),
                ),
                crate::expr::Expression::string_literal(
                    crate::error::SourcePosition::new(
                        "test".to_string(),
                        "test".to_string(),
                        1,
                        1,
                        1,
                        1,
                    ),
                    "c".to_string(),
                ),
            ],
        );

        let result = suffix_fn
            .eval(&[suffix_expr, array_expr], &env, &stdlib)
            .unwrap();
        let result_array = result.as_array().unwrap();

        assert_eq!(result_array.len(), 3);
        assert_eq!(result_array[0].as_string().unwrap(), "a_suf");
        assert_eq!(result_array[1].as_string().unwrap(), "b_suf");
        assert_eq!(result_array[2].as_string().unwrap(), "c_suf");
    }

    #[test]
    fn test_length_function_comprehensive() {
        let length_fn = create_length_function();
        let env = Bindings::new();
        let stdlib = crate::stdlib::StdLib::new("1.2");

        // Test array length - [1, 2, 3] should return 3
        let array_expr = crate::expr::Expression::array(
            crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1),
            vec![
                crate::expr::Expression::int(
                    crate::error::SourcePosition::new(
                        "test".to_string(),
                        "test".to_string(),
                        1,
                        1,
                        1,
                        1,
                    ),
                    1,
                ),
                crate::expr::Expression::int(
                    crate::error::SourcePosition::new(
                        "test".to_string(),
                        "test".to_string(),
                        1,
                        1,
                        1,
                        1,
                    ),
                    2,
                ),
                crate::expr::Expression::int(
                    crate::error::SourcePosition::new(
                        "test".to_string(),
                        "test".to_string(),
                        1,
                        1,
                        1,
                        1,
                    ),
                    3,
                ),
            ],
        );

        let result = length_fn.eval(&[array_expr], &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 3);

        // Test empty array length - [] should return 0
        let empty_array_expr = crate::expr::Expression::array(
            crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1),
            vec![],
        );

        let result = length_fn.eval(&[empty_array_expr], &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 0);

        // Test string length - "ABCDE" should return 5
        let string_expr = crate::expr::Expression::string_literal(
            crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1),
            "ABCDE".to_string(),
        );

        let result = length_fn.eval(&[string_expr], &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 5);

        // Test map length - {"a": 1, "b": 2} should return 2
        let map_expr = crate::expr::Expression::map(
            crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1),
            vec![
                (
                    crate::expr::Expression::string_literal(
                        crate::error::SourcePosition::new(
                            "test".to_string(),
                            "test".to_string(),
                            1,
                            1,
                            1,
                            1,
                        ),
                        "a".to_string(),
                    ),
                    crate::expr::Expression::int(
                        crate::error::SourcePosition::new(
                            "test".to_string(),
                            "test".to_string(),
                            1,
                            1,
                            1,
                            1,
                        ),
                        1,
                    ),
                ),
                (
                    crate::expr::Expression::string_literal(
                        crate::error::SourcePosition::new(
                            "test".to_string(),
                            "test".to_string(),
                            1,
                            1,
                            1,
                            1,
                        ),
                        "b".to_string(),
                    ),
                    crate::expr::Expression::int(
                        crate::error::SourcePosition::new(
                            "test".to_string(),
                            "test".to_string(),
                            1,
                            1,
                            1,
                            1,
                        ),
                        2,
                    ),
                ),
            ],
        );

        let result = length_fn.eval(&[map_expr], &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 2);

        // Test struct/object length - {a: 1, b: "test"} should return 2
        let struct_expr = crate::expr::Expression::struct_expr(
            crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1),
            vec![
                (
                    "a".to_string(),
                    crate::expr::Expression::int(
                        crate::error::SourcePosition::new(
                            "test".to_string(),
                            "test".to_string(),
                            1,
                            1,
                            1,
                            1,
                        ),
                        1,
                    ),
                ),
                (
                    "b".to_string(),
                    crate::expr::Expression::string_literal(
                        crate::error::SourcePosition::new(
                            "test".to_string(),
                            "test".to_string(),
                            1,
                            1,
                            1,
                            1,
                        ),
                        "test".to_string(),
                    ),
                ),
            ],
        );

        let result = length_fn.eval(&[struct_expr], &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 2);
    }
}
