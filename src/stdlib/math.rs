//! Mathematical functions for WDL standard library

use crate::error::WdlError;
use crate::types::Type;
use crate::value::Value;
use super::Function;

/// Floor function - returns the largest integer less than or equal to the given float
pub struct FloorFunction;

impl Function for FloorFunction {
    fn name(&self) -> &str { "floor" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }
        if !matches!(args[0], Type::Float { .. }) {
            return Err(WdlError::TypeMismatch {
                expected: Type::float(false),
                actual: args[0].clone(),
            });
        }
        Ok(Type::int(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if let Some(f) = args[0].as_float() {
            Ok(Value::int(f.floor() as i64))
        } else {
            Err(WdlError::RuntimeError {
                message: format!("floor() expects Float argument"),
            })
        }
    }
}

/// Ceiling function - returns the smallest integer greater than or equal to the given float
pub struct CeilFunction;

impl Function for CeilFunction {
    fn name(&self) -> &str { "ceil" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }
        if !matches!(args[0], Type::Float { .. }) {
            return Err(WdlError::TypeMismatch {
                expected: Type::float(false),
                actual: args[0].clone(),
            });
        }
        Ok(Type::int(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if let Some(f) = args[0].as_float() {
            Ok(Value::int(f.ceil() as i64))
        } else {
            Err(WdlError::RuntimeError {
                message: format!("ceil() expects Float argument"),
            })
        }
    }
}

/// Round function - rounds a float to the nearest integer (half-up)
pub struct RoundFunction;

impl Function for RoundFunction {
    fn name(&self) -> &str { "round" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }
        if !matches!(args[0], Type::Float { .. }) {
            return Err(WdlError::TypeMismatch {
                expected: Type::float(false),
                actual: args[0].clone(),
            });
        }
        Ok(Type::int(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if let Some(f) = args[0].as_float() {
            // Round half up like Python's round
            Ok(Value::int((f + 0.5).floor() as i64))
        } else {
            Err(WdlError::RuntimeError {
                message: format!("round() expects Float argument"),
            })
        }
    }
}

/// Min function - returns the minimum of two numeric values
pub struct MinFunction;

impl Function for MinFunction {
    fn name(&self) -> &str { "min" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }
        
        // Both arguments must be numeric
        let is_numeric = |t: &Type| matches!(t, Type::Int { .. } | Type::Float { .. });
        if !is_numeric(&args[0]) || !is_numeric(&args[1]) {
            return Err(WdlError::RuntimeError {
                message: format!("min() expects numeric arguments"),
            });
        }
        
        // Return Float if either argument is Float
        if matches!(args[0], Type::Float { .. }) || matches!(args[1], Type::Float { .. }) {
            Ok(Type::float(false))
        } else {
            Ok(Type::int(false))
        }
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let (a, b) = (&args[0], &args[1]);
        
        match (a, b) {
            (Value::Int { value: a, .. }, Value::Int { value: b, .. }) => {
                Ok(Value::int((*a).min(*b)))
            }
            _ => {
                // Convert to float for comparison
                let a_float = a.as_float().or_else(|| a.as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("min() expects numeric arguments"),
                    })?;
                let b_float = b.as_float().or_else(|| b.as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("min() expects numeric arguments"),
                    })?;
                Ok(Value::float(a_float.min(b_float)))
            }
        }
    }
}

/// Max function - returns the maximum of two numeric values
pub struct MaxFunction;

impl Function for MaxFunction {
    fn name(&self) -> &str { "max" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }
        
        // Both arguments must be numeric
        let is_numeric = |t: &Type| matches!(t, Type::Int { .. } | Type::Float { .. });
        if !is_numeric(&args[0]) || !is_numeric(&args[1]) {
            return Err(WdlError::RuntimeError {
                message: format!("max() expects numeric arguments"),
            });
        }
        
        // Return Float if either argument is Float
        if matches!(args[0], Type::Float { .. }) || matches!(args[1], Type::Float { .. }) {
            Ok(Type::float(false))
        } else {
            Ok(Type::int(false))
        }
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let (a, b) = (&args[0], &args[1]);
        
        match (a, b) {
            (Value::Int { value: a, .. }, Value::Int { value: b, .. }) => {
                Ok(Value::int((*a).max(*b)))
            }
            _ => {
                // Convert to float for comparison
                let a_float = a.as_float().or_else(|| a.as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("max() expects numeric arguments"),
                    })?;
                let b_float = b.as_float().or_else(|| b.as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("max() expects numeric arguments"),
                    })?;
                Ok(Value::float(a_float.max(b_float)))
            }
        }
    }
}