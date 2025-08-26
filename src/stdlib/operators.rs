//! Binary and unary operators for WDL standard library

use crate::error::WdlError;
use crate::types::Type;
use crate::value::Value;
use super::Function;

/// Addition operator (+)
pub struct AddOperator;

impl Function for AddOperator {
    fn name(&self) -> &str { "_add" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }
        
        match (&args[0], &args[1]) {
            (Type::Int { .. }, Type::Int { .. }) => Ok(Type::int(false)),
            (Type::Float { .. }, _) | (_, Type::Float { .. }) => Ok(Type::float(false)),
            (Type::String { .. }, _) | (_, Type::String { .. }) => Ok(Type::string(false)),
            _ => Err(WdlError::RuntimeError {
                message: format!("Cannot add {:?} and {:?}", args[0], args[1]),
            }),
        }
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        match (&args[0], &args[1]) {
            (Value::Int { value: a, .. }, Value::Int { value: b, .. }) => {
                Ok(Value::int(a + b))
            }
            (Value::String { value: a, .. }, b) => {
                let b_str = match b {
                    Value::String { value, .. } => value.clone(),
                    Value::Int { value, .. } => value.to_string(),
                    Value::Float { value, .. } => format!("{:.6}", value),
                    _ => return Err(WdlError::RuntimeError {
                        message: format!("Cannot concatenate String with {:?}", b),
                    }),
                };
                Ok(Value::string(format!("{}{}", a, b_str)))
            }
            (a, Value::String { value: b, .. }) => {
                let a_str = match a {
                    Value::Int { value, .. } => value.to_string(),
                    Value::Float { value, .. } => format!("{:.6}", value),
                    _ => return Err(WdlError::RuntimeError {
                        message: format!("Cannot concatenate {:?} with String", a),
                    }),
                };
                Ok(Value::string(format!("{}{}", a_str, b)))
            }
            _ => {
                let a_float = args[0].as_float().or_else(|| args[0].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot add non-numeric values"),
                    })?;
                let b_float = args[1].as_float().or_else(|| args[1].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot add non-numeric values"),
                    })?;
                Ok(Value::float(a_float + b_float))
            }
        }
    }
}

/// Subtraction operator (-)
pub struct SubtractOperator;

impl Function for SubtractOperator {
    fn name(&self) -> &str { "_sub" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }
        
        match (&args[0], &args[1]) {
            (Type::Int { .. }, Type::Int { .. }) => Ok(Type::int(false)),
            (Type::Float { .. }, _) | (_, Type::Float { .. }) => Ok(Type::float(false)),
            _ => Err(WdlError::RuntimeError {
                message: format!("Cannot subtract {:?} and {:?}", args[0], args[1]),
            }),
        }
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        match (&args[0], &args[1]) {
            (Value::Int { value: a, .. }, Value::Int { value: b, .. }) => {
                Ok(Value::int(a - b))
            }
            _ => {
                let a_float = args[0].as_float().or_else(|| args[0].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot subtract non-numeric values"),
                    })?;
                let b_float = args[1].as_float().or_else(|| args[1].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot subtract non-numeric values"),
                    })?;
                Ok(Value::float(a_float - b_float))
            }
        }
    }
}

/// Multiplication operator (*)
pub struct MultiplyOperator;

impl Function for MultiplyOperator {
    fn name(&self) -> &str { "_mul" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }
        
        match (&args[0], &args[1]) {
            (Type::Int { .. }, Type::Int { .. }) => Ok(Type::int(false)),
            (Type::Float { .. }, _) | (_, Type::Float { .. }) => Ok(Type::float(false)),
            _ => Err(WdlError::RuntimeError {
                message: format!("Cannot multiply {:?} and {:?}", args[0], args[1]),
            }),
        }
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        match (&args[0], &args[1]) {
            (Value::Int { value: a, .. }, Value::Int { value: b, .. }) => {
                Ok(Value::int(a * b))
            }
            _ => {
                let a_float = args[0].as_float().or_else(|| args[0].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot multiply non-numeric values"),
                    })?;
                let b_float = args[1].as_float().or_else(|| args[1].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot multiply non-numeric values"),
                    })?;
                Ok(Value::float(a_float * b_float))
            }
        }
    }
}

/// Division operator (/)
pub struct DivideOperator;

impl Function for DivideOperator {
    fn name(&self) -> &str { "_div" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }
        
        match (&args[0], &args[1]) {
            (Type::Int { .. }, Type::Int { .. }) => Ok(Type::int(false)),
            (Type::Float { .. }, _) | (_, Type::Float { .. }) => Ok(Type::float(false)),
            _ => Err(WdlError::RuntimeError {
                message: format!("Cannot divide {:?} and {:?}", args[0], args[1]),
            }),
        }
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        match (&args[0], &args[1]) {
            (Value::Int { value: a, .. }, Value::Int { value: b, .. }) => {
                if *b == 0 {
                    return Err(WdlError::RuntimeError {
                        message: format!("Division by zero"),
                    });
                }
                Ok(Value::int(a / b))
            }
            _ => {
                let a_float = args[0].as_float().or_else(|| args[0].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot divide non-numeric values"),
                    })?;
                let b_float = args[1].as_float().or_else(|| args[1].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot divide non-numeric values"),
                    })?;
                if b_float == 0.0 {
                    return Err(WdlError::RuntimeError {
                        message: format!("Division by zero"),
                    });
                }
                Ok(Value::float(a_float / b_float))
            }
        }
    }
}

/// Remainder operator (%)
pub struct RemainderOperator;

impl Function for RemainderOperator {
    fn name(&self) -> &str { "_rem" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }
        
        if !matches!(args[0], Type::Int { .. }) || !matches!(args[1], Type::Int { .. }) {
            return Err(WdlError::RuntimeError {
                message: format!("Remainder operator requires Int arguments"),
            });
        }
        
        Ok(Type::int(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        match (&args[0], &args[1]) {
            (Value::Int { value: a, .. }, Value::Int { value: b, .. }) => {
                if *b == 0 {
                    return Err(WdlError::RuntimeError {
                        message: format!("Division by zero in remainder"),
                    });
                }
                Ok(Value::int(a % b))
            }
            _ => Err(WdlError::RuntimeError {
                message: format!("Remainder operator requires Int arguments"),
            }),
        }
    }
}

/// Equality operator (==)
pub struct EqualOperator;

impl Function for EqualOperator {
    fn name(&self) -> &str { "_eqeq" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }
        Ok(Type::boolean(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        Ok(Value::boolean(args[0] == args[1]))
    }
}

/// Inequality operator (!=)
pub struct NotEqualOperator;

impl Function for NotEqualOperator {
    fn name(&self) -> &str { "_neq" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }
        Ok(Type::boolean(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        Ok(Value::boolean(args[0] != args[1]))
    }
}

/// Less than operator (<)
pub struct LessThanOperator;

impl Function for LessThanOperator {
    fn name(&self) -> &str { "_lt" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }
        Ok(Type::boolean(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        match (&args[0], &args[1]) {
            (Value::Int { value: a, .. }, Value::Int { value: b, .. }) => {
                Ok(Value::boolean(a < b))
            }
            (Value::String { value: a, .. }, Value::String { value: b, .. }) => {
                Ok(Value::boolean(a < b))
            }
            _ => {
                let a_float = args[0].as_float().or_else(|| args[0].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot compare non-numeric/string values"),
                    })?;
                let b_float = args[1].as_float().or_else(|| args[1].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot compare non-numeric/string values"),
                    })?;
                Ok(Value::boolean(a_float < b_float))
            }
        }
    }
}

/// Less than or equal operator (<=)
pub struct LessThanEqualOperator;

impl Function for LessThanEqualOperator {
    fn name(&self) -> &str { "_lte" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }
        Ok(Type::boolean(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        match (&args[0], &args[1]) {
            (Value::Int { value: a, .. }, Value::Int { value: b, .. }) => {
                Ok(Value::boolean(a <= b))
            }
            (Value::String { value: a, .. }, Value::String { value: b, .. }) => {
                Ok(Value::boolean(a <= b))
            }
            _ => {
                let a_float = args[0].as_float().or_else(|| args[0].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot compare non-numeric/string values"),
                    })?;
                let b_float = args[1].as_float().or_else(|| args[1].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot compare non-numeric/string values"),
                    })?;
                Ok(Value::boolean(a_float <= b_float))
            }
        }
    }
}

/// Greater than operator (>)
pub struct GreaterThanOperator;

impl Function for GreaterThanOperator {
    fn name(&self) -> &str { "_gt" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }
        Ok(Type::boolean(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        match (&args[0], &args[1]) {
            (Value::Int { value: a, .. }, Value::Int { value: b, .. }) => {
                Ok(Value::boolean(a > b))
            }
            (Value::String { value: a, .. }, Value::String { value: b, .. }) => {
                Ok(Value::boolean(a > b))
            }
            _ => {
                let a_float = args[0].as_float().or_else(|| args[0].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot compare non-numeric/string values"),
                    })?;
                let b_float = args[1].as_float().or_else(|| args[1].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot compare non-numeric/string values"),
                    })?;
                Ok(Value::boolean(a_float > b_float))
            }
        }
    }
}

/// Greater than or equal operator (>=)
pub struct GreaterThanEqualOperator;

impl Function for GreaterThanEqualOperator {
    fn name(&self) -> &str { "_gte" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }
        Ok(Type::boolean(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        match (&args[0], &args[1]) {
            (Value::Int { value: a, .. }, Value::Int { value: b, .. }) => {
                Ok(Value::boolean(a >= b))
            }
            (Value::String { value: a, .. }, Value::String { value: b, .. }) => {
                Ok(Value::boolean(a >= b))
            }
            _ => {
                let a_float = args[0].as_float().or_else(|| args[0].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot compare non-numeric/string values"),
                    })?;
                let b_float = args[1].as_float().or_else(|| args[1].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot compare non-numeric/string values"),
                    })?;
                Ok(Value::boolean(a_float >= b_float))
            }
        }
    }
}

/// Logical AND operator (&&)
pub struct LogicalAndOperator;

impl Function for LogicalAndOperator {
    fn name(&self) -> &str { "_and" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }
        Ok(Type::boolean(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let a_bool = args[0].as_bool().ok_or_else(|| WdlError::RuntimeError {
            message: format!("Logical AND requires Boolean arguments"),
        })?;
        let b_bool = args[1].as_bool().ok_or_else(|| WdlError::RuntimeError {
            message: format!("Logical AND requires Boolean arguments"),
        })?;
        Ok(Value::boolean(a_bool && b_bool))
    }
}

/// Logical OR operator (||)
pub struct LogicalOrOperator;

impl Function for LogicalOrOperator {
    fn name(&self) -> &str { "_or" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }
        Ok(Type::boolean(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let a_bool = args[0].as_bool().ok_or_else(|| WdlError::RuntimeError {
            message: format!("Logical OR requires Boolean arguments"),
        })?;
        let b_bool = args[1].as_bool().ok_or_else(|| WdlError::RuntimeError {
            message: format!("Logical OR requires Boolean arguments"),
        })?;
        Ok(Value::boolean(a_bool || b_bool))
    }
}

/// Logical NOT operator (!)
pub struct LogicalNotOperator;

impl Function for LogicalNotOperator {
    fn name(&self) -> &str { "_not" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }
        Ok(Type::boolean(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let bool_val = args[0].as_bool().ok_or_else(|| WdlError::RuntimeError {
            message: format!("Logical NOT requires Boolean argument"),
        })?;
        Ok(Value::boolean(!bool_val))
    }
}