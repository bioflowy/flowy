//! Array manipulation functions for WDL standard library

use super::Function;
use crate::error::WdlError;
use crate::types::Type;
use crate::value::{Value, ValueBase};

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
                message: "length() expects Array, String, or Map argument".to_string(),
            }),
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        match &args[0] {
            Value::Array { values, .. } => Ok(Value::int(values.len() as i64)),
            Value::String { value, .. } => Ok(Value::int(value.len() as i64)),
            Value::Map { pairs, .. } => Ok(Value::int(pairs.len() as i64)),
            _ => Err(WdlError::RuntimeError {
                message: "length() expects Array, String, or Map argument".to_string(),
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
        if args.is_empty() || args.len() > 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: if args.is_empty() { 1 } else { 2 },
                actual: args.len(),
            });
        }

        if let Type::Array { item_type, .. } = &args[0] {
            if args.len() == 2 {
                // With fallback argument, return the common type of array items and fallback
                let fallback_type = &args[1];
                // For simplicity, return the fallback type (should be same as item type)
                Ok(fallback_type.clone())
            } else {
                // Return the non-optional version of the item type
                Ok(item_type.clone().with_optional(false))
            }
        } else {
            Err(WdlError::RuntimeError {
                message: "select_first() expects Array as first argument".to_string(),
            })
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if let Value::Array { values, .. } = &args[0] {
            for value in values {
                if !matches!(value, Value::Null) {
                    return Ok(value.clone());
                }
            }

            // No non-null values found, check if fallback is provided
            if args.len() == 2 {
                Ok(args[1].clone())
            } else {
                Err(WdlError::RuntimeError {
                    message: "select_first() found no non-null values and no fallback provided"
                        .to_string(),
                })
            }
        } else {
            Err(WdlError::RuntimeError {
                message: "select_first() expects Array as first argument".to_string(),
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
                message: "select_all() expects Array argument".to_string(),
            })
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if let Value::Array { values, wdl_type } = &args[0] {
            let non_null_values: Vec<Value> = values
                .iter()
                .filter(|v| !matches!(v, Value::Null))
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
                message: "select_all() expects Array argument".to_string(),
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
                    message: "flatten() expects Array[Array[T]] argument".to_string(),
                })
            }
        } else {
            Err(WdlError::RuntimeError {
                message: "flatten() expects Array argument".to_string(),
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
                        message: "flatten() expects Array[Array[T]]".to_string(),
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
                message: "flatten() type error".to_string(),
            })
        } else {
            Err(WdlError::RuntimeError {
                message: "flatten() expects Array argument".to_string(),
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
                    message: "range() expects non-negative integer".to_string(),
                });
            }

            let values: Vec<Value> = (0..n).map(Value::int).collect();
            Ok(Value::array(Type::int(false), values))
        } else {
            Err(WdlError::RuntimeError {
                message: "range() expects Int argument".to_string(),
            })
        }
    }
}

/// Prefix function - prepends a prefix to each array element
pub struct PrefixFunction;

impl Function for PrefixFunction {
    fn name(&self) -> &str {
        "prefix"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }

        // First argument should be String (prefix)
        if !matches!(args[0], Type::String { .. }) {
            return Err(WdlError::RuntimeError {
                message: "prefix() first argument must be String".to_string(),
            });
        }

        // Second argument should be Array
        if let Type::Array { nonempty, .. } = &args[1] {
            Ok(Type::array(Type::string(false), false, *nonempty))
        } else {
            Err(WdlError::RuntimeError {
                message: "prefix() second argument must be Array".to_string(),
            })
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let prefix = match &args[0] {
            Value::String { value, .. } => value,
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "prefix() first argument must be String".to_string(),
                })
            }
        };

        if let Value::Array { values, .. } = &args[1] {
            let prefixed_values: Vec<Value> = values
                .iter()
                .map(|v| {
                    let string_value = match v {
                        Value::String { value, .. } => value.clone(),
                        Value::Int { value, .. } => value.to_string(),
                        Value::Float { value, .. } => value.to_string(),
                        Value::Boolean { value, .. } => value.to_string(),
                        _ => format!("{}", v),
                    };
                    Value::string(format!("{}{}", prefix, string_value))
                })
                .collect();

            Ok(Value::array(Type::string(false), prefixed_values))
        } else {
            Err(WdlError::RuntimeError {
                message: "prefix() second argument must be Array".to_string(),
            })
        }
    }
}

/// Suffix function - appends a suffix to each array element
pub struct SuffixFunction;

impl Function for SuffixFunction {
    fn name(&self) -> &str {
        "suffix"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }

        // First argument should be String (suffix)
        if !matches!(args[0], Type::String { .. }) {
            return Err(WdlError::RuntimeError {
                message: "suffix() first argument must be String".to_string(),
            });
        }

        // Second argument should be Array
        if let Type::Array { nonempty, .. } = &args[1] {
            Ok(Type::array(Type::string(false), false, *nonempty))
        } else {
            Err(WdlError::RuntimeError {
                message: "suffix() second argument must be Array".to_string(),
            })
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let suffix = match &args[0] {
            Value::String { value, .. } => value,
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "suffix() first argument must be String".to_string(),
                })
            }
        };

        if let Value::Array { values, .. } = &args[1] {
            let suffixed_values: Vec<Value> = values
                .iter()
                .map(|v| {
                    let string_value = match v {
                        Value::String { value, .. } => value.clone(),
                        Value::Int { value, .. } => value.to_string(),
                        Value::Float { value, .. } => value.to_string(),
                        Value::Boolean { value, .. } => value.to_string(),
                        _ => format!("{}", v),
                    };
                    Value::string(format!("{}{}", string_value, suffix))
                })
                .collect();

            Ok(Value::array(Type::string(false), suffixed_values))
        } else {
            Err(WdlError::RuntimeError {
                message: "suffix() second argument must be Array".to_string(),
            })
        }
    }
}

/// Quote function - wraps each array element in double quotes
pub struct QuoteFunction;

impl Function for QuoteFunction {
    fn name(&self) -> &str {
        "quote"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        if let Type::Array { nonempty, .. } = &args[0] {
            Ok(Type::array(Type::string(false), false, *nonempty))
        } else {
            Err(WdlError::RuntimeError {
                message: "quote() expects Array argument".to_string(),
            })
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if let Value::Array { values, .. } = &args[0] {
            let quoted_values: Vec<Value> = values
                .iter()
                .map(|v| {
                    let string_value = match v {
                        Value::String { value, .. } => value.clone(),
                        Value::Int { value, .. } => value.to_string(),
                        Value::Float { value, .. } => value.to_string(),
                        Value::Boolean { value, .. } => value.to_string(),
                        _ => format!("{}", v),
                    };
                    Value::string(format!("\"{}\"", string_value))
                })
                .collect();

            Ok(Value::array(Type::string(false), quoted_values))
        } else {
            Err(WdlError::RuntimeError {
                message: "quote() expects Array argument".to_string(),
            })
        }
    }
}

/// Single quote function - wraps each array element in single quotes
pub struct SquoteFunction;

impl Function for SquoteFunction {
    fn name(&self) -> &str {
        "squote"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        if let Type::Array { nonempty, .. } = &args[0] {
            Ok(Type::array(Type::string(false), false, *nonempty))
        } else {
            Err(WdlError::RuntimeError {
                message: "squote() expects Array argument".to_string(),
            })
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if let Value::Array { values, .. } = &args[0] {
            let quoted_values: Vec<Value> = values
                .iter()
                .map(|v| {
                    let string_value = match v {
                        Value::String { value, .. } => value.clone(),
                        Value::Int { value, .. } => value.to_string(),
                        Value::Float { value, .. } => value.to_string(),
                        Value::Boolean { value, .. } => value.to_string(),
                        _ => format!("{}", v),
                    };
                    Value::string(format!("'{}'", string_value))
                })
                .collect();

            Ok(Value::array(Type::string(false), quoted_values))
        } else {
            Err(WdlError::RuntimeError {
                message: "squote() expects Array argument".to_string(),
            })
        }
    }
}

pub struct ZipFunction;

impl Function for ZipFunction {
    fn name(&self) -> &str {
        "zip"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }

        if let (
            Type::Array {
                item_type: left_type,
                nonempty: left_nonempty,
                ..
            },
            Type::Array {
                item_type: right_type,
                nonempty: right_nonempty,
                ..
            },
        ) = (&args[0], &args[1])
        {
            let pair_type = Type::pair((**left_type).clone(), (**right_type).clone(), false);
            Ok(Type::array(
                pair_type,
                false,
                *left_nonempty && *right_nonempty,
            ))
        } else {
            Err(WdlError::RuntimeError {
                message: "zip() expects two Array arguments".to_string(),
            })
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if let (
            Value::Array {
                values: left_values,
                ..
            },
            Value::Array {
                values: right_values,
                ..
            },
        ) = (&args[0], &args[1])
        {
            let pairs: Vec<Value> = left_values
                .iter()
                .zip(right_values.iter())
                .map(|(left, right)| {
                    Value::pair(
                        left.wdl_type().clone(),
                        right.wdl_type().clone(),
                        left.clone(),
                        right.clone(),
                    )
                })
                .collect();

            if let Some(first_pair) = pairs.first() {
                Ok(Value::array(first_pair.wdl_type().clone(), pairs))
            } else {
                // Empty arrays case
                let pair_type = Type::pair(Type::any(), Type::any(), false);
                Ok(Value::array(pair_type, pairs))
            }
        } else {
            Err(WdlError::RuntimeError {
                message: "zip() expects two Array arguments".to_string(),
            })
        }
    }
}

pub struct CrossFunction;

impl Function for CrossFunction {
    fn name(&self) -> &str {
        "cross"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }

        if let (
            Type::Array {
                item_type: left_type,
                ..
            },
            Type::Array {
                item_type: right_type,
                ..
            },
        ) = (&args[0], &args[1])
        {
            let pair_type = Type::pair((**left_type).clone(), (**right_type).clone(), false);
            Ok(Type::array(pair_type, false, false))
        } else {
            Err(WdlError::RuntimeError {
                message: "cross() expects two Array arguments".to_string(),
            })
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if let (
            Value::Array {
                values: left_values,
                ..
            },
            Value::Array {
                values: right_values,
                ..
            },
        ) = (&args[0], &args[1])
        {
            let mut cross_product = Vec::new();

            for left_val in left_values {
                for right_val in right_values {
                    cross_product.push(Value::pair(
                        left_val.wdl_type().clone(),
                        right_val.wdl_type().clone(),
                        left_val.clone(),
                        right_val.clone(),
                    ));
                }
            }

            if let Some(first_pair) = cross_product.first() {
                Ok(Value::array(first_pair.wdl_type().clone(), cross_product))
            } else {
                // Empty arrays case
                let pair_type = Type::pair(Type::any(), Type::any(), false);
                Ok(Value::array(pair_type, cross_product))
            }
        } else {
            Err(WdlError::RuntimeError {
                message: "cross() expects two Array arguments".to_string(),
            })
        }
    }
}

pub struct TransposeFunction;

impl Function for TransposeFunction {
    fn name(&self) -> &str {
        "transpose"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        if let Type::Array {
            item_type,
            nonempty,
            ..
        } = &args[0]
        {
            if let Type::Array {
                item_type: inner_type,
                ..
            } = item_type.as_ref()
            {
                // Array[Array[T]] -> Array[Array[T]] (same type structure)
                Ok(Type::array(
                    Type::array((**inner_type).clone(), false, false),
                    false,
                    *nonempty,
                ))
            } else {
                Err(WdlError::RuntimeError {
                    message: "transpose() expects Array[Array[T]] argument".to_string(),
                })
            }
        } else {
            Err(WdlError::RuntimeError {
                message: "transpose() expects Array argument".to_string(),
            })
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if let Value::Array { values, .. } = &args[0] {
            if values.is_empty() {
                return Ok(Value::array(
                    Type::array(Type::any(), false, false),
                    Vec::new(),
                ));
            }

            // Get the length of the first inner array to determine dimensions
            let first_inner_len = if let Value::Array {
                values: first_inner,
                ..
            } = &values[0]
            {
                first_inner.len()
            } else {
                return Err(WdlError::RuntimeError {
                    message: "transpose() expects Array[Array[T]]".to_string(),
                });
            };

            // Create transposed matrix
            let mut transposed = vec![Vec::new(); first_inner_len];

            for row in values {
                if let Value::Array {
                    values: row_values, ..
                } = row
                {
                    if row_values.len() != first_inner_len {
                        return Err(WdlError::RuntimeError {
                            message:
                                "transpose() requires all inner arrays to have the same length"
                                    .to_string(),
                        });
                    }
                    for (col_idx, value) in row_values.iter().enumerate() {
                        transposed[col_idx].push(value.clone());
                    }
                } else {
                    return Err(WdlError::RuntimeError {
                        message: "transpose() expects Array[Array[T]]".to_string(),
                    });
                }
            }

            // Convert back to Value arrays
            let transposed_arrays: Vec<Value> = transposed
                .into_iter()
                .map(|col| {
                    let item_type = if let Some(first) = col.first() {
                        first.wdl_type().clone()
                    } else {
                        Type::any()
                    };
                    Value::array(item_type, col)
                })
                .collect();

            let inner_array_type = if let Some(first) = transposed_arrays.first() {
                first.wdl_type().clone()
            } else {
                Type::array(Type::any(), false, false)
            };

            Ok(Value::array(inner_array_type, transposed_arrays))
        } else {
            Err(WdlError::RuntimeError {
                message: "transpose() expects Array argument".to_string(),
            })
        }
    }
}

pub struct UnzipFunction;

impl Function for UnzipFunction {
    fn name(&self) -> &str {
        "unzip"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        if let Type::Array {
            item_type,
            nonempty,
            ..
        } = &args[0]
        {
            if let Type::Pair {
                left_type,
                right_type,
                ..
            } = item_type.as_ref()
            {
                // Array[Pair[L,R]] -> Pair[Array[L], Array[R]]
                let left_array = Type::array((**left_type).clone(), false, *nonempty);
                let right_array = Type::array((**right_type).clone(), false, *nonempty);
                Ok(Type::pair(left_array, right_array, false))
            } else {
                Err(WdlError::RuntimeError {
                    message: "unzip() expects Array[Pair[L,R]] argument".to_string(),
                })
            }
        } else {
            Err(WdlError::RuntimeError {
                message: "unzip() expects Array argument".to_string(),
            })
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if let Value::Array { values, .. } = &args[0] {
            let mut left_values = Vec::new();
            let mut right_values = Vec::new();

            for pair in values {
                if let Value::Pair { left, right, .. } = pair {
                    left_values.push(left.as_ref().clone());
                    right_values.push(right.as_ref().clone());
                } else {
                    return Err(WdlError::RuntimeError {
                        message: "unzip() expects Array[Pair[L,R]]".to_string(),
                    });
                }
            }

            let left_item_type = if let Some(first) = left_values.first() {
                first.wdl_type().clone()
            } else {
                Type::any()
            };

            let right_item_type = if let Some(first) = right_values.first() {
                first.wdl_type().clone()
            } else {
                Type::any()
            };

            let left_array = Value::array(left_item_type, left_values);
            let right_array = Value::array(right_item_type, right_values);

            Ok(Value::pair(
                left_array.wdl_type().clone(),
                right_array.wdl_type().clone(),
                left_array,
                right_array,
            ))
        } else {
            Err(WdlError::RuntimeError {
                message: "unzip() expects Array argument".to_string(),
            })
        }
    }
}

/// Get keys from a map
pub struct KeysFunction;

impl Function for KeysFunction {
    fn name(&self) -> &str {
        "keys"
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
            Type::Map { key_type, .. } => {
                // Map[X, Y] -> Array[X]
                Ok(Type::array(key_type.as_ref().clone(), false, false))
            }
            Type::StructInstance { .. } => {
                // Struct -> Array[String]
                Ok(Type::array(Type::string(false), false, false))
            }
            _ => Err(WdlError::RuntimeError {
                message: "keys() expects Map or Struct argument".to_string(),
            }),
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        match &args[0] {
            Value::Map {
                pairs, wdl_type, ..
            } => {
                let keys: Vec<Value> = pairs.iter().map(|(k, _)| k.clone()).collect();

                // Get key type from map type
                let key_type = if let Type::Map { key_type, .. } = wdl_type {
                    key_type.as_ref().clone()
                } else {
                    Type::any()
                };

                Ok(Value::array(key_type, keys))
            }
            Value::Struct { members, .. } => {
                // Get struct member names as string keys
                let keys: Vec<Value> = members
                    .keys()
                    .map(|name| Value::string(name.clone()))
                    .collect();

                Ok(Value::array(Type::string(false), keys))
            }
            _ => Err(WdlError::RuntimeError {
                message: "keys() expects Map or Struct argument".to_string(),
            }),
        }
    }
}

/// Get values from a map
pub struct ValuesFunction;

impl Function for ValuesFunction {
    fn name(&self) -> &str {
        "values"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        if let Type::Map { value_type, .. } = &args[0] {
            Ok(Type::array(value_type.as_ref().clone(), false, false))
        } else {
            Err(WdlError::RuntimeError {
                message: "values() expects Map argument".to_string(),
            })
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if let Value::Map {
            pairs, wdl_type, ..
        } = &args[0]
        {
            let values: Vec<Value> = pairs.iter().map(|(_, v)| v.clone()).collect();

            // Get value type from map type
            let value_type = if let Type::Map { value_type, .. } = wdl_type {
                value_type.as_ref().clone()
            } else {
                Type::any()
            };

            Ok(Value::array(value_type, values))
        } else {
            Err(WdlError::RuntimeError {
                message: "values() expects Map argument".to_string(),
            })
        }
    }
}
/// Check if a map/object contains a specific key
pub struct ContainsKeyFunction;

/// Convert Map[X,Y] to Array[Pair[X,Y]] - WDL 1.2 as_pairs function
pub struct AsPairsFunction;

/// Convert Array[Pair[X,Y]] to Map[X,Y] - WDL 1.2 as_map function
pub struct AsMapFunction;

impl Function for AsPairsFunction {
    fn name(&self) -> &str {
        "as_pairs"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: "as_pairs".to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        match &args[0] {
            Type::Map {
                key_type,
                value_type,
                ..
            } => {
                let pair_type = Type::pair(
                    key_type.as_ref().clone(),
                    value_type.as_ref().clone(),
                    false,
                );
                Ok(Type::array(pair_type, false, false))
            }
            _ => Err(WdlError::RuntimeError {
                message: "as_pairs() expects Map argument".to_string(),
            }),
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::RuntimeError {
                message: "as_pairs() expects exactly 1 argument".to_string(),
            });
        }

        match &args[0] {
            Value::Map { pairs, wdl_type } => {
                // Extract key and value types from the map type
                if let Type::Map {
                    key_type,
                    value_type,
                    ..
                } = wdl_type
                {
                    // Create Pair type for the array elements
                    let pair_type = Type::pair(
                        key_type.as_ref().clone(),
                        value_type.as_ref().clone(),
                        false,
                    );

                    // Convert each (key, value) pair to a Pair value
                    let mut pair_values: Vec<Value> = Vec::new();
                    for (key, value) in pairs {
                        // Use correct argument order: left_type, right_type, left, right
                        let pair_value = Value::pair(
                            key_type.as_ref().clone(),
                            value_type.as_ref().clone(),
                            key.clone(),
                            value.clone(),
                        );
                        pair_values.push(pair_value);
                    }

                    // Create Array[Pair[X,Y]] type
                    let array_type = Type::array(pair_type, false, false);
                    Ok(Value::Array {
                        values: pair_values,
                        wdl_type: array_type,
                    })
                } else {
                    Err(WdlError::RuntimeError {
                        message: "as_pairs() argument must have Map type".to_string(),
                    })
                }
            }
            _ => Err(WdlError::RuntimeError {
                message: "as_pairs() expects Map argument".to_string(),
            }),
        }
    }
}
impl Function for AsMapFunction {
    fn name(&self) -> &str {
        "as_map"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: "as_map".to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        match &args[0] {
            Type::Array { item_type, .. } => {
                // Check if it's Array[Pair[X,Y]]
                if let Type::Pair {
                    left_type,
                    right_type,
                    ..
                } = item_type.as_ref()
                {
                    // Return Map[X,Y]
                    Ok(Type::map(
                        left_type.as_ref().clone(),
                        right_type.as_ref().clone(),
                        false,
                    ))
                } else {
                    Err(WdlError::RuntimeError {
                        message: "as_map() expects Array[Pair[X,Y]] argument".to_string(),
                    })
                }
            }
            _ => Err(WdlError::RuntimeError {
                message: "as_map() expects Array argument".to_string(),
            }),
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::RuntimeError {
                message: "as_map() expects exactly 1 argument".to_string(),
            });
        }

        match &args[0] {
            Value::Array { values, wdl_type } => {
                // Extract the array element type
                if let Type::Array { item_type, .. } = wdl_type {
                    // Verify it's Pair type
                    if let Type::Pair {
                        left_type,
                        right_type,
                        ..
                    } = item_type.as_ref()
                    {
                        // Convert each Pair value to (key, value) for the map
                        let mut map_pairs: Vec<(Value, Value)> = Vec::new();
                        let mut seen_keys = std::collections::HashSet::new();

                        for pair_value in values {
                            if let Value::Pair { left, right, .. } = pair_value {
                                // Check for duplicate keys
                                let key_str = format!("{:?}", left);
                                if seen_keys.contains(&key_str) {
                                    return Err(WdlError::RuntimeError {
                                        message: format!("as_map() duplicate key: {:?}", left),
                                    });
                                }
                                seen_keys.insert(key_str);

                                map_pairs.push((left.as_ref().clone(), right.as_ref().clone()));
                            } else {
                                return Err(WdlError::RuntimeError {
                                    message: "as_map() expects Array[Pair[X,Y]]".to_string(),
                                });
                            }
                        }

                        // Create Map[X,Y] type
                        let map_type = Type::map(
                            left_type.as_ref().clone(),
                            right_type.as_ref().clone(),
                            false,
                        );

                        Ok(Value::Map {
                            pairs: map_pairs,
                            wdl_type: map_type,
                        })
                    } else {
                        Err(WdlError::RuntimeError {
                            message: "as_map() expects Array[Pair[X,Y]]".to_string(),
                        })
                    }
                } else {
                    Err(WdlError::RuntimeError {
                        message: "as_map() argument must have Array type".to_string(),
                    })
                }
            }
            _ => Err(WdlError::RuntimeError {
                message: "as_map() expects Array argument".to_string(),
            }),
        }
    }
}

impl Function for ContainsKeyFunction {
    fn name(&self) -> &str {
        "contains_key"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }

        // All overloads return Boolean
        Ok(Type::boolean(false))
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }

        match (&args[0], &args[1]) {
            // contains_key(Map[P, Y], P) - check if map contains key
            (Value::Map { pairs, .. }, key_value) => {
                let contains = pairs.iter().any(|(k, _)| k == key_value);
                Ok(Value::boolean(contains))
            }

            // contains_key(Object, String) - check if object has member
            (Value::Struct { members, .. }, Value::String { value: key, .. }) => {
                let contains = members.contains_key(key);
                Ok(Value::boolean(contains))
            }

            // contains_key(Map/Struct/Object, Array[String]) - compound key check
            (container, Value::Array { values, .. }) => {
                // Convert array to string keys
                let mut keys: Vec<String> = Vec::new();
                for val in values {
                    match val {
                        Value::String { value, .. } => keys.push(value.clone()),
                        _ => {
                            return Err(WdlError::RuntimeError {
                                message: "contains_key with Array argument requires Array[String]"
                                    .to_string(),
                            });
                        }
                    }
                }

                // Check compound key recursively
                let contains = self.check_compound_key(container, &keys)?;
                Ok(Value::boolean(contains))
            }

            _ => Err(WdlError::RuntimeError {
                message: format!(
                    "contains_key() unsupported argument types: {:?} and {:?}",
                    args[0].wdl_type(),
                    args[1].wdl_type()
                ),
            }),
        }
    }
}

impl ContainsKeyFunction {
    /// Check for compound key presence recursively
    fn check_compound_key(&self, container: &Value, keys: &[String]) -> Result<bool, WdlError> {
        if keys.is_empty() {
            return Ok(true);
        }

        let first_key = &keys[0];
        let remaining_keys = &keys[1..];

        match container {
            Value::Map { pairs, .. } => {
                // Look for the first key in the map
                for (k, v) in pairs {
                    if let Value::String { value: key_str, .. } = k {
                        if key_str == first_key {
                            if remaining_keys.is_empty() {
                                return Ok(true);
                            } else {
                                return self.check_compound_key(v, remaining_keys);
                            }
                        }
                    }
                }
                Ok(false)
            }

            Value::Struct { members, .. } => {
                if let Some(value) = members.get(first_key) {
                    if remaining_keys.is_empty() {
                        Ok(true)
                    } else {
                        self.check_compound_key(value, remaining_keys)
                    }
                } else {
                    Ok(false)
                }
            }

            _ => Ok(false),
        }
    }
}
#[cfg(test)]
mod contains_key_tests {
    use super::*;
    use crate::types::Type;
    use crate::value::Value;
    use std::collections::HashMap;

    #[test]
    fn test_contains_key_map() {
        let function = ContainsKeyFunction;

        // Create a map: {"a": 1, "b": 2}
        let map_value = Value::map(
            Type::string(false),
            Type::int(false),
            vec![
                (Value::string("a".to_string()), Value::int(1)),
                (Value::string("b".to_string()), Value::int(2)),
            ],
        );

        // Test existing key
        let result = function
            .eval(&[map_value.clone(), Value::string("a".to_string())])
            .unwrap();
        assert_eq!(result.as_bool().unwrap(), true);

        // Test non-existing key
        let result = function
            .eval(&[map_value.clone(), Value::string("c".to_string())])
            .unwrap();
        assert_eq!(result.as_bool().unwrap(), false);
    }

    #[test]
    fn test_contains_key_struct() {
        let function = ContainsKeyFunction;

        // Create a struct with members
        let mut members = HashMap::new();
        members.insert("name".to_string(), Value::string("John".to_string()));
        members.insert("age".to_string(), Value::int(30));

        let struct_type = Type::StructInstance {
            type_name: "Person".to_string(),
            members: None,
            optional: false,
        };

        let struct_value = Value::struct_value_unchecked(struct_type, members, None);

        // Test existing member
        let result = function
            .eval(&[struct_value.clone(), Value::string("name".to_string())])
            .unwrap();
        assert_eq!(result.as_bool().unwrap(), true);

        // Test non-existing member
        let result = function
            .eval(&[struct_value.clone(), Value::string("email".to_string())])
            .unwrap();
        assert_eq!(result.as_bool().unwrap(), false);
    }

    #[test]
    fn test_contains_key_compound() {
        let function = ContainsKeyFunction;

        // Create nested structure: {"details": {"phone": "123"}}
        let mut inner_members = HashMap::new();
        inner_members.insert(
            "phone".to_string(),
            Value::string("123-456-7890".to_string()),
        );

        let inner_struct_type = Type::StructInstance {
            type_name: "Details".to_string(),
            members: None,
            optional: false,
        };
        let inner_struct = Value::struct_value_unchecked(inner_struct_type, inner_members, None);

        let mut outer_members = HashMap::new();
        outer_members.insert("name".to_string(), Value::string("John".to_string()));
        outer_members.insert("details".to_string(), inner_struct);

        let outer_struct_type = Type::StructInstance {
            type_name: "Person".to_string(),
            members: None,
            optional: false,
        };
        let outer_struct = Value::struct_value_unchecked(outer_struct_type, outer_members, None);

        // Test compound key ["details", "phone"] - should exist
        let keys_array = Value::array(
            Type::string(false),
            vec![
                Value::string("details".to_string()),
                Value::string("phone".to_string()),
            ],
        );

        let result = function.eval(&[outer_struct.clone(), keys_array]).unwrap();
        assert_eq!(result.as_bool().unwrap(), true);

        // Test compound key ["details", "email"] - should not exist
        let keys_array = Value::array(
            Type::string(false),
            vec![
                Value::string("details".to_string()),
                Value::string("email".to_string()),
            ],
        );

        let result = function.eval(&[outer_struct.clone(), keys_array]).unwrap();
        assert_eq!(result.as_bool().unwrap(), false);
    }

    #[test]
    fn test_as_pairs_function_is_implemented() {
        // This test confirms that as_pairs is now implemented
        use crate::stdlib::StdLib;
        let stdlib = StdLib::new("1.2");

        // Try to find the as_pairs function - should return Some now
        let as_pairs_fn = stdlib.get_function("as_pairs");
        assert!(
            as_pairs_fn.is_some(),
            "as_pairs function should be implemented"
        );

        // Verify function name
        if let Some(func) = as_pairs_fn {
            assert_eq!(func.name(), "as_pairs");
        }
    }

    #[test]
    fn test_as_pairs_function_expected_behavior() {
        // This test validates the actual behavior of as_pairs function
        use crate::types::Type;
        use crate::value::Value;

        // Create a test map: {"a": 1, "b": 2}
        let pairs = vec![
            (Value::string("a".to_string()), Value::int(1)),
            (Value::string("b".to_string()), Value::int(2)),
        ];
        let test_map = Value::Map {
            pairs,
            wdl_type: Type::map(Type::string(false), Type::int(false), false),
        };

        // Function should now be implemented
        let stdlib = crate::stdlib::StdLib::new("1.2");
        if let Some(as_pairs_fn) = stdlib.get_function("as_pairs") {
            let result = as_pairs_fn.eval(&[test_map]);

            // Should return Array[Pair[String, Int]]
            // Expected: [("a", 1), ("b", 2)]
            assert!(result.is_ok());

            if let Ok(array_value) = result {
                if let Value::Array { values, .. } = array_value {
                    assert_eq!(values.len(), 2);

                    // Check pairs (order may vary for maps, so check both exist)
                    let mut found_a = false;
                    let mut found_b = false;

                    for value in &values {
                        if let Value::Pair { left, right, .. } = value {
                            let key = left.as_string().unwrap();
                            if key == "a" {
                                assert_eq!(right.as_int().unwrap(), 1);
                                found_a = true;
                            } else if key == "b" {
                                assert_eq!(right.as_int().unwrap(), 2);
                                found_b = true;
                            }
                        } else {
                            panic!("Expected Pair value");
                        }
                    }

                    assert!(found_a, "Should find pair ('a', 1)");
                    assert!(found_b, "Should find pair ('b', 2)");
                } else {
                    panic!("Expected Array value");
                }
            } else {
                panic!("Function evaluation should succeed");
            }
        } else {
            panic!("as_pairs function not found in standard library");
        }
    }

    #[test]
    fn test_as_map_function_is_implemented() {
        // Test that as_map function is now implemented
        let stdlib = crate::stdlib::StdLib::new("1.2");

        // Try to get as_map function - should succeed
        let result = stdlib.get_function("as_map");
        assert!(result.is_some(), "as_map function should be implemented");

        // Verify the function name
        if let Some(as_map_fn) = result {
            assert_eq!(as_map_fn.name(), "as_map");
        }
    }

    #[test]
    fn test_as_map_function_expected_behavior() {
        // Test the expected behavior of as_map once implemented
        // This test should fail until as_map is implemented
        let stdlib = crate::stdlib::StdLib::new("1.2");

        if let Some(as_map_fn) = stdlib.get_function("as_map") {
            use crate::types::Type;
            use crate::value::Value;

            // Create Array[Pair[String, Int]] = [("a", 1), ("b", 2)]
            let pair1 = Value::pair(
                Type::string(false),
                Type::int(false),
                Value::string("a".to_string()),
                Value::int(1),
            );
            let pair2 = Value::pair(
                Type::string(false),
                Type::int(false),
                Value::string("b".to_string()),
                Value::int(2),
            );

            let pairs_array = Value::array(
                Type::pair(Type::string(false), Type::int(false), false),
                vec![pair1, pair2],
            );

            // Test type inference
            let inferred_type = as_map_fn.infer_type(&[pairs_array.wdl_type().clone()]);
            assert!(inferred_type.is_ok());

            if let Ok(map_type) = inferred_type {
                assert!(matches!(map_type, Type::Map { .. }));
            }

            // Test evaluation
            let result = as_map_fn.eval(&[pairs_array]);
            assert!(
                result.is_ok(),
                "as_map should successfully convert Array[Pair] to Map"
            );

            if let Ok(map_value) = result {
                // Should be a Map with {"a": 1, "b": 2}
                if let Value::Map { pairs, .. } = map_value {
                    assert_eq!(pairs.len(), 2);
                    // Check the map contains the expected key-value pairs
                    let keys: Vec<String> = pairs
                        .iter()
                        .map(|(k, _)| {
                            if let Value::String { value, .. } = k {
                                value.clone()
                            } else {
                                panic!("Key should be string")
                            }
                        })
                        .collect();
                    assert!(keys.contains(&"a".to_string()));
                    assert!(keys.contains(&"b".to_string()));
                }
            }
        } else {
            panic!("as_map function should be implemented");
        }
    }
}
