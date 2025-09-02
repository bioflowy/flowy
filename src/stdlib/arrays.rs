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
                message: "select_first() expects Array argument".to_string(),
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
            Err(WdlError::RuntimeError {
                message: "select_first() found no non-null values".to_string(),
            })
        } else {
            Err(WdlError::RuntimeError {
                message: "select_first() expects Array argument".to_string(),
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
