//! Arithmetic and comparison operators for WDL standard library

use crate::env::Bindings;
use crate::error::{SourcePosition, WdlError};
use crate::expr::{Expression, ExpressionBase};
use crate::stdlib::{evaluate_and_coerce_args, Function};
use crate::types::Type;
use crate::value::Value;

/// Arithmetic operator implementation that handles Int/Float type inference
pub struct ArithmeticOperator {
    name: String,
    operation: Box<dyn Fn(&Value, &Value) -> Result<Value, WdlError> + Send + Sync>,
}

/// Comparison operator implementation that handles type comparisons and returns boolean
pub struct ComparisonOperator {
    name: String,
    operation: Box<dyn Fn(&Value, &Value) -> Result<Value, WdlError> + Send + Sync>,
}

/// Equality operator implementation that handles equality/inequality testing
pub struct EqualityOperator {
    name: String,
    #[allow(dead_code)]
    negate: bool,
    operation: Box<dyn Fn(&Value, &Value) -> Result<Value, WdlError> + Send + Sync>,
}

/// Logical operator implementation for && and || with short-circuit evaluation
pub struct LogicalOperator {
    name: String,
    is_and: bool, // true for &&, false for ||
}

/// Logical NOT operator implementation for !
pub struct LogicalNotOperator;

/// Numeric negation operator implementation for unary minus
pub struct NegateOperator;

impl LogicalOperator {
    pub fn new_and() -> Self {
        Self {
            name: "_and".to_string(),
            is_and: true,
        }
    }

    pub fn new_or() -> Self {
        Self {
            name: "_or".to_string(),
            is_and: false,
        }
    }
}

impl Function for LogicalOperator {
    fn name(&self) -> &str {
        &self.name
    }

    fn infer_type(
        &self,
        args: &mut [crate::expr::Expression],
        type_env: &crate::env::Bindings<Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        if args.len() != 2 {
            let pos = if args.is_empty() {
                SourcePosition::new("unknown".to_string(), "unknown".to_string(), 0, 0, 0, 0)
            } else {
                args[0].source_position().clone()
            };
            return Err(WdlError::Validation {
                pos,
                message: format!(
                    "Logical operator '{}' requires exactly 2 arguments, got {}",
                    self.name,
                    args.len()
                ),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        // Both arguments must be Boolean
        let left_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        let right_type = args[1].infer_type(type_env, stdlib, struct_typedefs)?;

        // Check if types can be coerced to Boolean
        let bool_type = Type::boolean(false);
        if !bool_type.coerces(&left_type, true) {
            return Err(WdlError::Validation {
                pos: args[0].source_position().clone(),
                message: format!(
                    "Left operand of '{}' must be Boolean, got {}",
                    self.name, left_type
                ),
                source_text: None,
                declared_wdl_version: None,
            });
        }
        if !bool_type.coerces(&right_type, true) {
            return Err(WdlError::Validation {
                pos: args[1].source_position().clone(),
                message: format!(
                    "Right operand of '{}' must be Boolean, got {}",
                    self.name, right_type
                ),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        Ok(bool_type)
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        env: &crate::env::Bindings<Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::RuntimeError {
                message: format!(
                    "Logical operator '{}' requires exactly 2 arguments, got {}",
                    self.name,
                    args.len()
                ),
            });
        }

        // Evaluate left operand
        let left = args[0].eval(env, stdlib)?;
        let left_bool = left.as_bool().ok_or_else(|| WdlError::RuntimeError {
            message: format!("Left operand of '{}' must be Boolean", self.name),
        })?;

        // Short-circuit evaluation
        if self.is_and {
            // For &&: if left is false, return false without evaluating right
            if !left_bool {
                return Ok(Value::boolean(false));
            }
            // If left is true, evaluate and return right operand
            let right = args[1].eval(env, stdlib)?;
            let right_bool = right.as_bool().ok_or_else(|| WdlError::RuntimeError {
                message: format!("Right operand of '{}' must be Boolean", self.name),
            })?;
            Ok(Value::boolean(right_bool))
        } else {
            // For ||: if left is true, return true without evaluating right
            if left_bool {
                return Ok(Value::boolean(true));
            }
            // If left is false, evaluate and return right operand
            let right = args[1].eval(env, stdlib)?;
            let right_bool = right.as_bool().ok_or_else(|| WdlError::RuntimeError {
                message: format!("Right operand of '{}' must be Boolean", self.name),
            })?;
            Ok(Value::boolean(right_bool))
        }
    }
}

impl Function for LogicalNotOperator {
    fn name(&self) -> &str {
        "_not"
    }

    fn infer_type(
        &self,
        args: &mut [crate::expr::Expression],
        type_env: &crate::env::Bindings<Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        if args.len() != 1 {
            let pos = if args.is_empty() {
                SourcePosition::new("unknown".to_string(), "unknown".to_string(), 0, 0, 0, 0)
            } else {
                args[0].source_position().clone()
            };
            return Err(WdlError::Validation {
                pos,
                message: format!(
                    "Logical NOT operator '!' requires exactly 1 argument, got {}",
                    args.len()
                ),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        // Argument must be Boolean
        let arg_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        let bool_type = Type::boolean(false);

        if !bool_type.coerces(&arg_type, true) {
            return Err(WdlError::Validation {
                pos: args[0].source_position().clone(),
                message: format!("Operand of '!' must be Boolean, got {}", arg_type),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        Ok(bool_type)
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        env: &crate::env::Bindings<Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::RuntimeError {
                message: format!(
                    "Logical NOT operator '!' requires exactly 1 argument, got {}",
                    args.len()
                ),
            });
        }

        let operand = args[0].eval(env, stdlib)?;
        let bool_val = operand.as_bool().ok_or_else(|| WdlError::RuntimeError {
            message: "Operand of '!' must be Boolean".to_string(),
        })?;

        Ok(Value::boolean(!bool_val))
    }
}

impl Function for NegateOperator {
    fn name(&self) -> &str {
        "_neg"
    }

    fn infer_type(
        &self,
        args: &mut [crate::expr::Expression],
        type_env: &crate::env::Bindings<Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        if args.len() != 1 {
            let pos = if args.is_empty() {
                SourcePosition::new("unknown".to_string(), "unknown".to_string(), 0, 0, 0, 0)
            } else {
                args[0].source_position().clone()
            };
            return Err(WdlError::Validation {
                pos,
                message: format!(
                    "Negation operator '-' requires exactly 1 argument, got {}",
                    args.len()
                ),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        // Argument must be Int or Float
        let arg_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;

        match &arg_type {
            Type::Int { .. } => Ok(Type::int(false)),
            Type::Float { .. } => Ok(Type::float(false)),
            _ => {
                // Check if it can be coerced to numeric
                let int_type = Type::int(false);
                let float_type = Type::float(false);
                if int_type.coerces(&arg_type, true) {
                    Ok(Type::int(false))
                } else if float_type.coerces(&arg_type, true) {
                    Ok(Type::float(false))
                } else {
                    Err(WdlError::Validation {
                        pos: args[0].source_position().clone(),
                        message: format!("Operand of '-' must be Int or Float, got {}", arg_type),
                        source_text: None,
                        declared_wdl_version: None,
                    })
                }
            }
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
                message: format!(
                    "Negation operator '-' requires exactly 1 argument, got {}",
                    args.len()
                ),
            });
        }

        let operand = args[0].eval(env, stdlib)?;

        match operand {
            Value::Int { value, .. } => Ok(Value::int(-value)),
            Value::Float { value, .. } => Ok(Value::float(-value)),
            _ => Err(WdlError::RuntimeError {
                message: format!("Cannot negate non-numeric value: {:?}", operand),
            }),
        }
    }
}

impl EqualityOperator {
    /// Create a new equality operator
    pub fn new<F>(name: String, negate: bool, operation: F) -> Self
    where
        F: Fn(&Value, &Value) -> Result<Value, WdlError> + Send + Sync + 'static,
    {
        Self {
            name,
            negate,
            operation: Box::new(operation),
        }
    }

    /// Check if types are equatable (can be compared for equality)
    fn are_equatable_types(&self, left_type: &Type, right_type: &Type) -> bool {
        // Two types are equatable if they can be coerced to a common type
        // This follows WDL's equatable logic from miniwdl

        match (left_type, right_type) {
            // Same base types are equatable
            (Type::Int { .. }, Type::Int { .. }) => true,
            (Type::Float { .. }, Type::Float { .. }) => true,
            (Type::String { .. }, Type::String { .. }) => true,
            (Type::Boolean { .. }, Type::Boolean { .. }) => true,
            (Type::File { .. }, Type::File { .. }) => true,
            (Type::Directory { .. }, Type::Directory { .. }) => true,

            // Int and Float are equatable (numeric coercion)
            (Type::Int { .. }, Type::Float { .. }) => true,
            (Type::Float { .. }, Type::Int { .. }) => true,

            // Arrays are equatable if their item types are equatable
            (
                Type::Array {
                    item_type: left_item,
                    ..
                },
                Type::Array {
                    item_type: right_item,
                    ..
                },
            ) => self.are_equatable_types(left_item, right_item),

            // Maps are equatable if their key and value types are equatable
            (
                Type::Map {
                    key_type: left_key,
                    value_type: left_val,
                    ..
                },
                Type::Map {
                    key_type: right_key,
                    value_type: right_val,
                    ..
                },
            ) => {
                self.are_equatable_types(left_key, right_key)
                    && self.are_equatable_types(left_val, right_val)
            }

            // Pairs are equatable if both components are equatable
            (
                Type::Pair {
                    left_type: ll,
                    right_type: lr,
                    ..
                },
                Type::Pair {
                    left_type: rl,
                    right_type: rr,
                    ..
                },
            ) => self.are_equatable_types(ll, rl) && self.are_equatable_types(lr, rr),

            // StructInstances with same type name are equatable
            (
                Type::StructInstance {
                    type_name: left_name,
                    ..
                },
                Type::StructInstance {
                    type_name: right_name,
                    ..
                },
            ) => left_name == right_name,

            // Any type is equatable with any other type
            (Type::Any { .. }, _) => true,
            (_, Type::Any { .. }) => true,

            // Different base types are not equatable
            _ => false,
        }
    }
}
impl Function for EqualityOperator {
    fn infer_type(
        &self,
        args: &mut [Expression],
        type_env: &Bindings<Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
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
                    "Equality operator '{}' expects 2 arguments, got {}",
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

        // Check that operands are equatable
        if !self.are_equatable_types(&left_type, &right_type) {
            return Err(WdlError::Validation {
                pos: args[0].source_position().clone(),
                message: format!("Cannot test equality of {} and {}", left_type, right_type),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        // All equality operators return Boolean
        Ok(Type::boolean(false))
    }

    fn eval(
        &self,
        args: &[Expression],
        env: &Bindings<Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        // Check argument count
        if args.len() != 2 {
            return Err(WdlError::RuntimeError {
                message: format!(
                    "Equality operator '{}' expects 2 arguments, got {}",
                    self.name,
                    args.len()
                ),
            });
        }

        // Evaluate operands
        let left_value = args[0].eval(env, stdlib)?;
        let right_value = args[1].eval(env, stdlib)?;

        // Delegate to the operation function which handles equality logic
        (self.operation)(&left_value, &right_value)
    }

    fn name(&self) -> &str {
        &self.name
    }
}

impl ComparisonOperator {
    /// Create a new comparison operator
    pub fn new<F>(name: String, operation: F) -> Self
    where
        F: Fn(&Value, &Value) -> Result<Value, WdlError> + Send + Sync + 'static,
    {
        Self {
            name,
            operation: Box::new(operation),
        }
    }

    /// Check if types are comparable (both numeric or both strings)
    fn are_comparable_types(&self, left_type: &Type, right_type: &Type) -> bool {
        let is_left_numeric = matches!(left_type, Type::Int { .. } | Type::Float { .. });
        let is_right_numeric = matches!(right_type, Type::Int { .. } | Type::Float { .. });
        let is_left_string = matches!(left_type, Type::String { .. });
        let is_right_string = matches!(right_type, Type::String { .. });

        (is_left_numeric && is_right_numeric) || (is_left_string && is_right_string)
    }
}
impl Function for ComparisonOperator {
    fn infer_type(
        &self,
        args: &mut [Expression],
        type_env: &Bindings<Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
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
                    "Comparison operator '{}' expects 2 arguments, got {}",
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

        // Check that operands are comparable
        if !self.are_comparable_types(&left_type, &right_type) {
            return Err(WdlError::Validation {
                pos: args[0].source_position().clone(),
                message: format!(
                    "Incomparable operands to {} operator: {} and {}",
                    self.name, left_type, right_type
                ),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        // All comparison operators return Boolean
        Ok(Type::boolean(false))
    }

    fn eval(
        &self,
        args: &[Expression],
        env: &Bindings<Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        // Check argument count
        if args.len() != 2 {
            return Err(WdlError::RuntimeError {
                message: format!(
                    "Comparison operator '{}' expects 2 arguments, got {}",
                    self.name,
                    args.len()
                ),
            });
        }

        // Evaluate operands
        let left_value = args[0].eval(env, stdlib)?;
        let right_value = args[1].eval(env, stdlib)?;

        // Delegate to the operation function which handles comparison logic
        (self.operation)(&left_value, &right_value)
    }

    fn name(&self) -> &str {
        &self.name
    }
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
    fn infer_type(
        &self,
        args: &mut [Expression],
        type_env: &Bindings<Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
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

    fn eval(
        &self,
        args: &[Expression],
        env: &Bindings<Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
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
/// use miniwdl_rust::stdlib::operators::create_arithmetic_operator;
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
        if let (
            Value::Int {
                value: left_int, ..
            },
            Value::Int {
                value: right_int, ..
            },
        ) = (left, right)
        {
            // Both are Int, use integer operation
            let result = int_op(*left_int, *right_int);
            Ok(Value::int(result))
        } else {
            // At least one is Float or can be converted to Float, use float operation
            let left_float = left.as_float().ok_or_else(|| WdlError::RuntimeError {
                message: format!(
                    "Cannot convert left operand to number for {} operation",
                    name_clone
                ),
            })?;
            let right_float = right.as_float().ok_or_else(|| WdlError::RuntimeError {
                message: format!(
                    "Cannot convert right operand to number for {} operation",
                    name_clone
                ),
            })?;
            let result = float_op(left_float, right_float);
            Ok(Value::float(result))
        }
    };

    Box::new(ArithmeticOperator::new(name, operation))
}

/// Create a comparison operator function
///
/// Takes a function name and comparison operation closures for different types.
/// The operator automatically handles type coercion and returns Boolean results.
///
/// # Arguments
/// * `name` - The operator name (e.g., "_lt", "_le", "_gt", "_ge")
/// * `int_op` - Closure for i64 comparisons (left, right) -> bool
/// * `float_op` - Closure for f64 comparisons (left, right) -> bool
/// * `string_op` - Closure for String comparisons (left, right) -> bool
///
/// # Type Inference Rules
/// - If both operands are Int, use int_op
/// - If either operand is Float, convert both to Float and use float_op
/// - If both operands are String, use string_op
/// - Result is always Boolean
///
/// # Example
/// ```rust
/// use miniwdl_rust::stdlib::operators::create_comparison_operator;
/// let lt_fn = create_comparison_operator(
///     "_lt".to_string(),
///     |l, r| l < r,        // i64 comparison
///     |l, r| l < r,        // f64 comparison
///     |l, r| l < r,        // String comparison
/// );
/// ```
pub fn create_comparison_operator<IntOp, FloatOp, StringOp>(
    name: String,
    int_op: IntOp,
    float_op: FloatOp,
    string_op: StringOp,
) -> Box<dyn Function>
where
    IntOp: Fn(i64, i64) -> bool + Send + Sync + 'static,
    FloatOp: Fn(f64, f64) -> bool + Send + Sync + 'static,
    StringOp: Fn(&str, &str) -> bool + Send + Sync + 'static,
{
    let name_clone = name.clone();
    let operation = move |left: &Value, right: &Value| -> Result<Value, WdlError> {
        // Check if both values are strings
        if let (Some(left_str), Some(right_str)) = (left.as_string(), right.as_string()) {
            let result = string_op(left_str, right_str);
            return Ok(Value::boolean(result));
        }

        // Check if both values are Int for precision
        if let (
            Value::Int {
                value: left_int, ..
            },
            Value::Int {
                value: right_int, ..
            },
        ) = (left, right)
        {
            let result = int_op(*left_int, *right_int);
            return Ok(Value::boolean(result));
        }

        // Try to convert to Float for numeric comparison
        if let (Some(left_float), Some(right_float)) = (left.as_float(), right.as_float()) {
            let result = float_op(left_float, right_float);
            return Ok(Value::boolean(result));
        }

        // If we get here, the types are incomparable
        Err(WdlError::RuntimeError {
            message: format!(
                "Cannot compare incompatible types in {} operation",
                name_clone
            ),
        })
    };

    Box::new(ComparisonOperator::new(name, operation))
}

/// Create the less-than (_lt) operator function
pub fn create_lt_function() -> Box<dyn Function> {
    create_comparison_operator(
        "_lt".to_string(),
        |l, r| l < r, // i64 comparison
        |l, r| l < r, // f64 comparison
        |l, r| l < r, // String comparison
    )
}

/// Create the less-than-or-equal (_lte) operator function
pub fn create_lte_function() -> Box<dyn Function> {
    create_comparison_operator(
        "_lte".to_string(),
        |l, r| l <= r, // i64 comparison
        |l, r| l <= r, // f64 comparison
        |l, r| l <= r, // String comparison
    )
}

/// Create the greater-than (_gt) operator function
pub fn create_gt_function() -> Box<dyn Function> {
    create_comparison_operator(
        "_gt".to_string(),
        |l, r| l > r, // i64 comparison
        |l, r| l > r, // f64 comparison
        |l, r| l > r, // String comparison
    )
}

/// Create the greater-than-or-equal (_gte) operator function
pub fn create_gte_function() -> Box<dyn Function> {
    create_comparison_operator(
        "_gte".to_string(),
        |l, r| l >= r, // i64 comparison
        |l, r| l >= r, // f64 comparison
        |l, r| l >= r, // String comparison
    )
}

/// Create an equality operator function
///
/// Handles both equality (==) and inequality (!=) operations.
/// Automatically handles type coercion and equality testing for compatible types.
///
/// # Arguments
/// * `name` - The operator name (e.g., "_eqeq", "_neq")
/// * `negate` - If true, negates the equality result (for != operator)
///
/// # Type Rules
/// - Two values are equatable if their types can be coerced to a common type
/// - Int and Float are equatable through coercion
/// - Optional types are handled appropriately
/// - Arrays, Maps, and Pairs are equatable if their components are equatable
/// - Result is always Boolean
///
/// # Example
/// ```rust
/// use miniwdl_rust::stdlib::operators::create_equal_operator;
/// let eq_fn = create_equal_operator("_eqeq".to_string(), false);  // == operator
/// let neq_fn = create_equal_operator("_neq".to_string(), true);   // != operator
/// ```
pub fn create_equal_operator(name: String, negate: bool) -> Box<dyn Function> {
    let name_clone = name.clone();
    let operation = move |left: &Value, right: &Value| -> Result<Value, WdlError> {
        // Use Value's built-in equality comparison
        // This handles all the type coercion logic automatically
        let are_equal = match (left, right) {
            // Handle numeric comparisons with coercion
            (Value::Int { value: l, .. }, Value::Float { value: r, .. }) => (*l as f64) == *r,
            (Value::Float { value: l, .. }, Value::Int { value: r, .. }) => *l == (*r as f64),
            // For all other cases, use direct equality
            _ => left == right,
        };

        // Apply negation if this is the != operator
        let result = if negate { !are_equal } else { are_equal };
        Ok(Value::boolean(result))
    };

    Box::new(EqualityOperator::new(name, negate, operation))
}

/// Create the equality (_eqeq) operator function
pub fn create_eqeq_function() -> Box<dyn Function> {
    create_equal_operator("_eqeq".to_string(), false)
}

/// Create the inequality (_neq) operator function  
pub fn create_neq_function() -> Box<dyn Function> {
    create_equal_operator("_neq".to_string(), true)
}

/// Create a subtraction operator function
pub fn create_sub_function() -> Box<dyn Function> {
    create_arithmetic_operator(
        "_sub".to_string(),
        |l, r| l - r, // i64 subtraction
        |l, r| l - r, // f64 subtraction
    )
}

/// Create a multiplication operator function
pub fn create_mul_function() -> Box<dyn Function> {
    create_arithmetic_operator(
        "_mul".to_string(),
        |l, r| l * r, // i64 multiplication
        |l, r| l * r, // f64 multiplication
    )
}

/// Create a division operator function
pub fn create_div_function() -> Box<dyn Function> {
    create_arithmetic_operator(
        "_div".to_string(),
        |l, r| l / r, // i64 division (truncating)
        |l, r| l / r, // f64 division
    )
}

/// Create a remainder (modulo) operator function
pub fn create_rem_function() -> Box<dyn Function> {
    create_arithmetic_operator(
        "_rem".to_string(),
        |l, r| l % r, // i64 modulo
        |l, r| l % r, // f64 modulo
    )
}

/// Add operator implementation that supports both arithmetic and string concatenation
/// Similar to miniwdl's _AddOperator
pub struct AddOperator {
    name: String,
}

impl Default for AddOperator {
    fn default() -> Self {
        Self::new()
    }
}

impl AddOperator {
    pub fn new() -> Self {
        Self {
            name: "_add".to_string(),
        }
    }
}

impl Function for AddOperator {
    fn infer_type(
        &self,
        args: &mut [Expression],
        type_env: &Bindings<Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
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
            if left_type.coerces(&Type::string(false), true)
                && right_type.coerces(&Type::string(false), true)
            {
                return Ok(Type::string(false));
            } else {
                return Err(WdlError::Validation {
                    pos: args[0].source_position().clone(),
                    message: format!("Cannot add/concatenate {} and {}", left_type, right_type),
                    source_text: None,
                    declared_wdl_version: None,
                });
            }
        }

        // Neither operand is a string, check for numeric addition
        if matches!(left_type, Type::Int { .. } | Type::Float { .. })
            && matches!(right_type, Type::Int { .. } | Type::Float { .. })
        {
            // Return Float if either operand is Float, otherwise Int
            if matches!(left_type, Type::Float { .. }) || matches!(right_type, Type::Float { .. }) {
                Ok(Type::float(false))
            } else {
                Ok(Type::int(false))
            }
        } else {
            Err(WdlError::Validation {
                pos: args[0].source_position().clone(),
                message: format!("Cannot add/concatenate {} and {}", left_type, right_type),
                source_text: None,
                declared_wdl_version: None,
            })
        }
    }

    fn eval(
        &self,
        args: &[Expression],
        env: &Bindings<Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
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
        if matches!(left_value, Value::String { .. }) || matches!(right_value, Value::String { .. })
        {
            // String concatenation
            let left_str =
                left_value
                    .coerce(&Type::string(false))
                    .map_err(|_| WdlError::RuntimeError {
                        message: "Cannot coerce left operand to string for concatenation"
                            .to_string(),
                    })?;
            let right_str =
                right_value
                    .coerce(&Type::string(false))
                    .map_err(|_| WdlError::RuntimeError {
                        message: "Cannot coerce right operand to string for concatenation"
                            .to_string(),
                    })?;

            let left_string = left_str.as_string().ok_or_else(|| WdlError::RuntimeError {
                message: "Invalid left operand for string concatenation".to_string(),
            })?;
            let right_string = right_str
                .as_string()
                .ok_or_else(|| WdlError::RuntimeError {
                    message: "Invalid right operand for string concatenation".to_string(),
                })?;

            return Ok(Value::string(format!("{}{}", left_string, right_string)));
        }

        // Check if both values are numeric for arithmetic addition
        if (matches!(left_value, Value::Int { .. } | Value::Float { .. }))
            && (matches!(right_value, Value::Int { .. } | Value::Float { .. }))
        {
            // Numeric addition - check if both are Int variants for precision
            if matches!(left_value, Value::Int { .. }) && matches!(right_value, Value::Int { .. }) {
                let left_int = left_value.as_int().unwrap();
                let right_int = right_value.as_int().unwrap();
                Ok(Value::int(left_int + right_int))
            } else {
                // At least one is Float, use float arithmetic
                let left_float = left_value
                    .as_float()
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: "Cannot convert left operand to number for addition".to_string(),
                    })?;
                let right_float = right_value
                    .as_float()
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: "Cannot convert right operand to number for addition".to_string(),
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

/// Create a logical AND operator function (&&)
pub fn create_logical_and_function() -> Box<dyn Function> {
    Box::new(LogicalOperator::new_and())
}

/// Create a logical OR operator function (||)
pub fn create_logical_or_function() -> Box<dyn Function> {
    Box::new(LogicalOperator::new_or())
}

/// Create a logical NOT operator function (!)
pub fn create_logical_not_function() -> Box<dyn Function> {
    Box::new(LogicalNotOperator)
}

/// Create a numeric negation operator function (unary -)
pub fn create_negate_function() -> Box<dyn Function> {
    Box::new(NegateOperator)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::env::Bindings;
    use crate::error::SourcePosition;
    use crate::expr::Expression;
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
