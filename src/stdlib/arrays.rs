//! Array manipulation functions for WDL standard library

use super::Function;
use crate::error::WdlError;
use crate::types::Type;
use crate::value::Value;

/// Length function - returns the length of arrays, strings, or maps
pub struct LengthFunction;

impl Function for LengthFunction {
    fn name(&self) -> &str {
        "length"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        match &args[0] {
            Type::Array { .. } | Type::String { .. } | Type::Map { .. } => Ok(Type::int(false)),
            _ => Err(WdlError::RuntimeError {
                message: format!("length() expects Array, String, or Map argument"),
            }),
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        match &args[0] {
            Value::Array { values, .. } => Ok(Value::int(values.len() as i64)),
            Value::String { value, .. } => Ok(Value::int(value.len() as i64)),
            Value::Map { pairs, .. } => Ok(Value::int(pairs.len() as i64)),
            _ => Err(WdlError::RuntimeError {
                message: format!("length() expects Array, String, or Map argument"),
            }),
        }
    }
}

/// Select first non-null element from an array
pub struct SelectFirstFunction;

impl Function for SelectFirstFunction {
    fn name(&self) -> &str {
        "select_first"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        if let Type::Array { item_type, .. } = &args[0] {
            // Return the non-optional version of the item type
            Ok(item_type.clone().with_optional(false))
        } else {
            Err(WdlError::RuntimeError {
                message: format!("select_first() expects Array argument"),
            })
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if let Value::Array { values, .. } = &args[0] {
            for value in values {
                if !matches!(value, Value::Null { .. }) {
                    return Ok(value.clone());
                }
            }
            Err(WdlError::RuntimeError {
                message: format!("select_first() found no non-null values"),
            })
        } else {
            Err(WdlError::RuntimeError {
                message: format!("select_first() expects Array argument"),
            })
        }
    }
}

/// Select all non-null elements from an array
pub struct SelectAllFunction;

impl Function for SelectAllFunction {
    fn name(&self) -> &str {
        "select_all"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        if let Type::Array { item_type, .. } = &args[0] {
            // Return array of non-optional items
            Ok(Type::array(
                item_type.clone().with_optional(false),
                false,
                true,
            ))
        } else {
            Err(WdlError::RuntimeError {
                message: format!("select_all() expects Array argument"),
            })
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if let Value::Array { values, wdl_type } = &args[0] {
            let non_null_values: Vec<Value> = values
                .iter()
                .filter(|v| !matches!(v, Value::Null { .. }))
                .cloned()
                .collect();

            if let Type::Array { item_type, .. } = wdl_type {
                Ok(Value::array(
                    item_type.clone().with_optional(false),
                    non_null_values,
                ))
            } else {
                unreachable!()
            }
        } else {
            Err(WdlError::RuntimeError {
                message: format!("select_all() expects Array argument"),
            })
        }
    }
}

/// Flatten a 2D array into a 1D array
pub struct FlattenFunction;

impl Function for FlattenFunction {
    fn name(&self) -> &str {
        "flatten"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        if let Type::Array { item_type, .. } = &args[0] {
            if let Type::Array {
                item_type: inner_type,
                ..
            } = item_type.as_ref()
            {
                // Array[Array[T]] -> Array[T]
                Ok(Type::array(*inner_type.clone(), false, false))
            } else {
                Err(WdlError::RuntimeError {
                    message: format!("flatten() expects Array[Array[T]] argument"),
                })
            }
        } else {
            Err(WdlError::RuntimeError {
                message: format!("flatten() expects Array argument"),
            })
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if let Value::Array { values, wdl_type } = &args[0] {
            let mut flattened = Vec::new();

            for value in values {
                if let Value::Array { values: inner, .. } = value {
                    flattened.extend(inner.clone());
                } else {
                    return Err(WdlError::RuntimeError {
                        message: format!("flatten() expects Array[Array[T]]"),
                    });
                }
            }

            if let Type::Array { item_type, .. } = wdl_type {
                if let Type::Array {
                    item_type: inner_type,
                    ..
                } = item_type.as_ref()
                {
                    return Ok(Value::array(*inner_type.clone(), flattened));
                }
            }

            Err(WdlError::RuntimeError {
                message: format!("flatten() type error"),
            })
        } else {
            Err(WdlError::RuntimeError {
                message: format!("flatten() expects Array argument"),
            })
        }
    }
}

/// Generate a range of integers from 0 to n-1
pub struct RangeFunction;

impl Function for RangeFunction {
    fn name(&self) -> &str {
        "range"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        if !matches!(args[0], Type::Int { .. }) {
            return Err(WdlError::TypeMismatch {
                expected: Type::int(false),
                actual: args[0].clone(),
            });
        }

        Ok(Type::array(Type::int(false), false, true))
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if let Some(n) = args[0].as_int() {
            if n < 0 {
                return Err(WdlError::RuntimeError {
                    message: format!("range() expects non-negative integer"),
                });
            }

            let values: Vec<Value> = (0..n).map(Value::int).collect();
            Ok(Value::array(Type::int(false), values))
        } else {
            Err(WdlError::RuntimeError {
                message: format!("range() expects Int argument"),
            })
        }
    }
}
