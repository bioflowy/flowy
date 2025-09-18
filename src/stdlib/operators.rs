//! Arithmetic and comparison operators for WDL standard library

use crate::stdlib::{Function, evaluate_and_coerce_args};
use crate::error::{WdlError, SourcePosition};
use crate::expr::{Expression, ExpressionBase};
use crate::env::Bindings;
use crate::types::Type;
use crate::value::Value;

/// Arithmetic operator implementation that handles Int/Float type inference
pub struct ArithmeticOperator {
    name: String,
    operation: Box<dyn Fn(&Value, &Value) -> Result<Value, WdlError> + Send + Sync>,
}

impl ArithmeticOperator {
    /// Create a new arithmetic operator
    pub fn new<F>(name: String, operation: F) -> Self
    where
        F: Fn(&Value, &Value) -> Result<Value, WdlError> + Send + Sync + 'static,
    {
        Self {
            name,
            operation: Box::new(operation),
        }
    }

    /// Infer the result type based on operand types
    /// Returns Int if both operands are Int, Float if either operand is Float
    fn infer_result_type(&self, left_type: &Type, right_type: &Type) -> Type {
        if matches!(left_type, Type::Float { .. }) || matches!(right_type, Type::Float { .. }) {
            Type::float(false)
        } else {
            Type::int(false)
        }
    }

    /// Check if types are numeric (Int or Float)
    fn is_numeric_type(&self, ty: &Type) -> bool {
        matches!(ty, Type::Int { .. } | Type::Float { .. })
    }
}

impl Function for ArithmeticOperator {
    fn infer_type(&self, args: &mut [Expression], type_env: &Bindings<Type>, stdlib: &crate::stdlib::StdLib, struct_typedefs: &[crate::tree::StructTypeDef]) -> Result<Type, WdlError> {
        // Check argument count
        if args.len() != 2 {
            let pos = if args.is_empty() {
                SourcePosition::new("unknown".to_string(), "unknown".to_string(), 0, 0, 0, 0)
            } else {
                args[0].source_position().clone()
            };
            return Err(WdlError::Validation {
                pos,
                message: format!(
                    "Arithmetic operator '{}' expects 2 arguments, got {}",
                    self.name,
                    args.len()
                ),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        // Infer types of both operands
        let left_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        let right_type = args[1].infer_type(type_env, stdlib, struct_typedefs)?;

        // Check that both operands are numeric
        if !self.is_numeric_type(&left_type) {
            return Err(WdlError::Validation {
                pos: args[0].source_position().clone(),
                message: format!(
                    "Non-numeric operand to {} operator: expected Int or Float, got {}",
                    self.name, left_type
                ),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        if !self.is_numeric_type(&right_type) {
            return Err(WdlError::Validation {
                pos: args[1].source_position().clone(),
                message: format!(
                    "Non-numeric operand to {} operator: expected Int or Float, got {}",
                    self.name, right_type
                ),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        // Return the inferred result type
        Ok(self.infer_result_type(&left_type, &right_type))
    }

    fn eval(&self, args: &[Expression], env: &Bindings<Value>, stdlib: &crate::stdlib::StdLib) -> Result<Value, WdlError> {
        // Check argument count
        if args.len() != 2 {
            return Err(WdlError::RuntimeError {
                message: format!(
                    "Arithmetic operator '{}' expects 2 arguments, got {}",
                    self.name,
                    args.len()
                ),
            });
        }

        // Evaluate operands
        let left_value = args[0].eval(env, stdlib)?;
        let right_value = args[1].eval(env, stdlib)?;

        // Delegate to the operation function which handles type logic
        (self.operation)(&left_value, &right_value)
    }

    fn name(&self) -> &str {
        &self.name
    }
}

/// Create an arithmetic operator function
///
/// Takes a function name and binary math operation closures that handle type inference automatically.
/// The operator automatically handles Int/Float type inference following WDL semantics.
///
/// # Arguments
/// * `name` - The operator name (e.g., "_add", "_sub", "_mul", "_div")
/// * `int_op` - Closure for i64 operations (left, right) -> result
/// * `float_op` - Closure for f64 operations (left, right) -> result
///
/// # Type Inference Rules
/// - If both operands are Int, result is Int using int_op
/// - If either operand is Float, result is Float using float_op
/// - Both operands must be numeric (Int or Float)
///
/// # Example
/// ```rust
/// let add_fn = create_arithmetic_operator(
///     "_add".to_string(),
///     |l, r| l + r,    // i64 addition
///     |l, r| l + r,    // f64 addition
/// );
/// ```
pub fn create_arithmetic_operator<IntOp, FloatOp>(
    name: String,
    int_op: IntOp,
    float_op: FloatOp,
) -> Box<dyn Function>
where
    IntOp: Fn(i64, i64) -> i64 + Send + Sync + 'static,
    FloatOp: Fn(f64, f64) -> f64 + Send + Sync + 'static,
{
    let name_clone = name.clone();
    let operation = move |left: &Value, right: &Value| -> Result<Value, WdlError> {
        // Check if both values are Int variants for precision
        if let (Value::Int { value: left_int, .. }, Value::Int { value: right_int, .. }) = (left, right) {
            // Both are Int, use integer operation
            let result = int_op(*left_int, *right_int);
            Ok(Value::int(result))
        } else {
            // At least one is Float or can be converted to Float, use float operation
            let left_float = left.as_float().ok_or_else(|| WdlError::RuntimeError {
                message: format!("Cannot convert left operand to number for {} operation", name_clone),
            })?;
            let right_float = right.as_float().ok_or_else(|| WdlError::RuntimeError {
                message: format!("Cannot convert right operand to number for {} operation", name_clone),
            })?;
            let result = float_op(left_float, right_float);
            Ok(Value::float(result))
        }
    };

    Box::new(ArithmeticOperator::new(name, operation))
}

/// Create a subtraction operator function
pub fn create_sub_function() -> Box<dyn Function> {
    create_arithmetic_operator(
        "_sub".to_string(),
        |l, r| l - r,    // i64 subtraction
        |l, r| l - r,    // f64 subtraction
    )
}

/// Create a multiplication operator function
pub fn create_mul_function() -> Box<dyn Function> {
    create_arithmetic_operator(
        "_mul".to_string(),
        |l, r| l * r,    // i64 multiplication
        |l, r| l * r,    // f64 multiplication
    )
}

/// Create a division operator function
pub fn create_div_function() -> Box<dyn Function> {
    create_arithmetic_operator(
        "_div".to_string(),
        |l, r| l / r,    // i64 division (truncating)
        |l, r| l / r,    // f64 division
    )
}

/// Add operator implementation that supports both arithmetic and string concatenation
/// Similar to miniwdl's _AddOperator
pub struct AddOperator {
    name: String,
}

impl AddOperator {
    pub fn new() -> Self {
        Self {
            name: "_add".to_string(),
        }
    }
}

impl Function for AddOperator {
    fn infer_type(&self, args: &mut [Expression], type_env: &Bindings<Type>, stdlib: &crate::stdlib::StdLib, struct_typedefs: &[crate::tree::StructTypeDef]) -> Result<Type, WdlError> {
        // Check argument count
        if args.len() != 2 {
            let pos = if args.is_empty() {
                SourcePosition::new("unknown".to_string(), "unknown".to_string(), 0, 0, 0, 0)
            } else {
                args[0].source_position().clone()
            };
            return Err(WdlError::Validation {
                pos,
                message: format!("Add operator expects 2 arguments, got {}", args.len()),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        // Infer types of both operands
        let left_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        let right_type = args[1].infer_type(type_env, stdlib, struct_typedefs)?;

        // Check for string concatenation first
        if matches!(left_type, Type::String { .. }) || matches!(right_type, Type::String { .. }) {
            // At least one operand is a string, check if both can be coerced to string
            if left_type.coerces(&Type::string(false), true) && right_type.coerces(&Type::string(false), true) {
                return Ok(Type::string(false));
            } else {
                return Err(WdlError::Validation {
                    pos: args[0].source_position().clone(),
                    message: format!(
                        "Cannot add/concatenate {} and {}",
                        left_type, right_type
                    ),
                    source_text: None,
                    declared_wdl_version: None,
                });
            }
        }

        // Neither operand is a string, check for numeric addition
        if matches!(left_type, Type::Int { .. } | Type::Float { .. }) &&
           matches!(right_type, Type::Int { .. } | Type::Float { .. }) {
            // Return Float if either operand is Float, otherwise Int
            if matches!(left_type, Type::Float { .. }) || matches!(right_type, Type::Float { .. }) {
                Ok(Type::float(false))
            } else {
                Ok(Type::int(false))
            }
        } else {
            Err(WdlError::Validation {
                pos: args[0].source_position().clone(),
                message: format!(
                    "Cannot add/concatenate {} and {}",
                    left_type, right_type
                ),
                source_text: None,
                declared_wdl_version: None,
            })
        }
    }

    fn eval(&self, args: &[Expression], env: &Bindings<Value>, stdlib: &crate::stdlib::StdLib) -> Result<Value, WdlError> {
        // Check argument count
        if args.len() != 2 {
            return Err(WdlError::RuntimeError {
                message: format!("Add operator expects 2 arguments, got {}", args.len()),
            });
        }

        // Evaluate operands
        let left_value = args[0].eval(env, stdlib)?;
        let right_value = args[1].eval(env, stdlib)?;

        // Check for string concatenation first
        if matches!(left_value, Value::String { .. }) || matches!(right_value, Value::String { .. }) {
            // String concatenation
            let left_str = left_value.coerce(&Type::string(false)).map_err(|_| {
                WdlError::RuntimeError {
                    message: "Cannot coerce left operand to string for concatenation".to_string(),
                }
            })?;
            let right_str = right_value.coerce(&Type::string(false)).map_err(|_| {
                WdlError::RuntimeError {
                    message: "Cannot coerce right operand to string for concatenation".to_string(),
                }
            })?;

            let left_string = left_str.as_string().ok_or_else(|| {
                WdlError::RuntimeError {
                    message: "Invalid left operand for string concatenation".to_string(),
                }
            })?;
            let right_string = right_str.as_string().ok_or_else(|| {
                WdlError::RuntimeError {
                    message: "Invalid right operand for string concatenation".to_string(),
                }
            })?;

            return Ok(Value::string(format!("{}{}", left_string, right_string)));
        }

        // Check if both values are numeric for arithmetic addition
        if (matches!(left_value, Value::Int { .. } | Value::Float { .. })) &&
           (matches!(right_value, Value::Int { .. } | Value::Float { .. })) {
            // Numeric addition - check if both are Int variants for precision
            if matches!(left_value, Value::Int { .. }) && matches!(right_value, Value::Int { .. }) {
                let left_int = left_value.as_int().unwrap();
                let right_int = right_value.as_int().unwrap();
                Ok(Value::int(left_int + right_int))
            } else {
                // At least one is Float, use float arithmetic
                let left_float = left_value.as_float().ok_or_else(|| {
                    WdlError::RuntimeError {
                        message: "Cannot convert left operand to number for addition".to_string(),
                    }
                })?;
                let right_float = right_value.as_float().ok_or_else(|| {
                    WdlError::RuntimeError {
                        message: "Cannot convert right operand to number for addition".to_string(),
                    }
                })?;
                Ok(Value::float(left_float + right_float))
            }
        } else {
            Err(WdlError::RuntimeError {
                message: "Cannot add/concatenate the given operand types".to_string(),
            })
        }
    }

    fn name(&self) -> &str {
        &self.name
    }
}

/// Create an add operator function that supports both arithmetic and string concatenation
pub fn create_add_function() -> Box<dyn Function> {
    Box::new(AddOperator::new())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::expr::Expression;
    use crate::error::SourcePosition;
    use crate::env::Bindings;
    use crate::stdlib::StdLib;

    #[test]
    fn test_add_operator() {
        let add_fn = create_arithmetic_operator("_add".to_string(), |l, r| l + r, |l, r| l + r);
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);

        // Test integer addition
        let args = vec![
            Expression::int(pos.clone(), 5),
            Expression::int(pos.clone(), 3),
        ];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");
        let result = add_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 8);

        // Test float addition
        let args = vec![
            Expression::float(pos.clone(), 5.5),
            Expression::float(pos.clone(), 3.2),
        ];
        let result = add_fn.eval(&args, &env, &stdlib).unwrap();
        assert!((result.as_float().unwrap() - 8.7).abs() < f64::EPSILON);

        // Test mixed int/float addition (should return float)
        let args = vec![
            Expression::int(pos.clone(), 5),
            Expression::float(pos.clone(), 3.2),
        ];
        let result = add_fn.eval(&args, &env, &stdlib).unwrap();
        assert!((result.as_float().unwrap() - 8.2).abs() < f64::EPSILON);
    }

    #[test]
    fn test_subtract_operator() {
        let sub_fn = create_arithmetic_operator("_sub".to_string(), |l, r| l - r, |l, r| l - r);
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);

        let args = vec![
            Expression::int(pos.clone(), 10),
            Expression::int(pos.clone(), 3),
        ];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");
        let result = sub_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 7);
    }

    #[test]
    fn test_multiply_operator() {
        let mul_fn = create_arithmetic_operator("_mul".to_string(), |l, r| l * r, |l, r| l * r);
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);

        let args = vec![
            Expression::int(pos.clone(), 4),
            Expression::int(pos.clone(), 3),
        ];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");
        let result = mul_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 12);
    }

    #[test]
    fn test_divide_operator() {
        let div_fn = create_arithmetic_operator("_div".to_string(), |l, r| l / r, |l, r| l / r);
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);

        // Test integer division (should truncate)
        let args = vec![
            Expression::int(pos.clone(), 10),
            Expression::int(pos.clone(), 3),
        ];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");
        let result = div_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 3); // Truncated division

        // Test float division
        let args = vec![
            Expression::float(pos.clone(), 10.0),
            Expression::float(pos.clone(), 3.0),
        ];
        let result = div_fn.eval(&args, &env, &stdlib).unwrap();
        assert!((result.as_float().unwrap() - (10.0 / 3.0)).abs() < f64::EPSILON);
    }

    #[test]
    fn test_wrong_argument_count() {
        let add_fn = create_arithmetic_operator("_add".to_string(), |l, r| l + r, |l, r| l + r);
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);

        // Test with wrong number of arguments
        let args = vec![Expression::int(pos, 5)]; // Only one argument
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");
        let result = add_fn.eval(&args, &env, &stdlib);
        assert!(result.is_err());
    }
}