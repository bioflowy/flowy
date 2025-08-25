//! WDL Standard Library implementation
//! 
//! This module provides the standard library functions and operators for WDL,
//! similar to miniwdl's StdLib.py

use crate::error::WdlError;
use crate::types::Type;
use crate::value::Value;
// Note: Bindings available if needed
use std::collections::HashMap;

/// Function trait for standard library functions
pub trait Function: Send + Sync {
    /// Check argument types and return the result type
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError>;
    
    /// Evaluate the function with given arguments
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError>;
    
    /// Get the function name
    fn name(&self) -> &str;
}

/// Standard library containing all built-in functions and operators
pub struct StdLib {
    functions: HashMap<String, Box<dyn Function>>,
    #[allow(dead_code)]
    wdl_version: String,
}

impl StdLib {
    /// Create a new standard library instance for the given WDL version
    pub fn new(wdl_version: &str) -> Self {
        let mut stdlib = StdLib {
            functions: HashMap::new(),
            wdl_version: wdl_version.to_string(),
        };
        
        // Register built-in functions
        stdlib.register_builtin_functions();
        stdlib.register_operators();
        
        stdlib
    }
    
    /// Get a function by name
    pub fn get_function(&self, name: &str) -> Option<&dyn Function> {
        self.functions.get(name).map(|f| f.as_ref())
    }
    
    /// Register built-in functions
    fn register_builtin_functions(&mut self) {
        // Math functions
        self.register(FloorFunction);
        self.register(CeilFunction);
        self.register(RoundFunction);
        self.register(MinFunction);
        self.register(MaxFunction);
        
        // Array/String functions
        self.register(LengthFunction);
        self.register(SelectFirstFunction);
        self.register(SelectAllFunction);
        self.register(FlattenFunction);
        self.register(RangeFunction);
        
        // String functions
        self.register(SubFunction);
        self.register(BasenameFunction);
        self.register(SepFunction);
        
        // Type functions
        self.register(DefinedFunction);
    }
    
    /// Register operators as functions
    fn register_operators(&mut self) {
        // Arithmetic operators
        self.register(AddOperator);
        self.register(SubtractOperator);
        self.register(MultiplyOperator);
        self.register(DivideOperator);
        self.register(RemainderOperator);
        
        // Comparison operators
        self.register(EqualOperator);
        self.register(NotEqualOperator);
        self.register(LessThanOperator);
        self.register(LessThanEqualOperator);
        self.register(GreaterThanOperator);
        self.register(GreaterThanEqualOperator);
        
        // Logical operators
        self.register(LogicalAndOperator);
        self.register(LogicalOrOperator);
        self.register(LogicalNotOperator);
    }
    
    /// Register a function
    fn register<F: Function + 'static>(&mut self, func: F) {
        self.functions.insert(func.name().to_string(), Box::new(func));
    }
}

// Math functions

struct FloorFunction;
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

struct CeilFunction;
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

struct RoundFunction;
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

struct MinFunction;
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

struct MaxFunction;
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

// Array/String functions

struct LengthFunction;
impl Function for LengthFunction {
    fn name(&self) -> &str { "length" }
    
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

struct SelectFirstFunction;
impl Function for SelectFirstFunction {
    fn name(&self) -> &str { "select_first" }
    
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

struct SelectAllFunction;
impl Function for SelectAllFunction {
    fn name(&self) -> &str { "select_all" }
    
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
            Ok(Type::array(item_type.clone().with_optional(false), false, true))
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

struct FlattenFunction;
impl Function for FlattenFunction {
    fn name(&self) -> &str { "flatten" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }
        
        if let Type::Array { item_type, .. } = &args[0] {
            if let Type::Array { item_type: inner_type, .. } = item_type.as_ref() {
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
                if let Type::Array { item_type: inner_type, .. } = item_type.as_ref() {
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

struct RangeFunction;
impl Function for RangeFunction {
    fn name(&self) -> &str { "range" }
    
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

// String functions

struct SubFunction;
impl Function for SubFunction {
    fn name(&self) -> &str { "sub" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 3 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 3,
                actual: args.len(),
            });
        }
        
        for (i, arg) in args.iter().enumerate() {
            if !matches!(arg, Type::String { .. }) {
                return Err(WdlError::RuntimeError {
                    message: format!("sub() argument {} must be String", i + 1),
                });
            }
        }
        
        Ok(Type::string(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let input = args[0].as_string().ok_or_else(|| WdlError::RuntimeError {
            message: format!("sub() first argument must be String"),
        })?;
        let pattern = args[1].as_string().ok_or_else(|| WdlError::RuntimeError {
            message: format!("sub() second argument must be String"),
        })?;
        let replacement = args[2].as_string().ok_or_else(|| WdlError::RuntimeError {
            message: format!("sub() third argument must be String"),
        })?;
        
        // For now, use simple string replacement
        // TODO: Add regex support
        Ok(Value::string(input.replace(&pattern, &replacement)))
    }
}

struct BasenameFunction;
impl Function for BasenameFunction {
    fn name(&self) -> &str { "basename" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() < 1 || args.len() > 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1, // or 2
                actual: args.len(),
            });
        }
        
        if !matches!(args[0], Type::String { .. } | Type::File { .. }) {
            return Err(WdlError::RuntimeError {
                message: format!("basename() first argument must be String or File"),
            });
        }
        
        if args.len() == 2 && !matches!(args[1], Type::String { optional: true, .. }) {
            return Err(WdlError::RuntimeError {
                message: format!("basename() second argument must be String?"),
            });
        }
        
        Ok(Type::string(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let path = args[0].as_string().ok_or_else(|| WdlError::RuntimeError {
            message: format!("basename() first argument must be String"),
        })?;
        
        let base = path.rsplit('/').next().unwrap_or(&path);
        
        if args.len() == 2 {
            if let Some(suffix) = args[1].as_string() {
                if base.ends_with(&suffix) {
                    let trimmed = &base[..base.len() - suffix.len()];
                    return Ok(Value::string(trimmed.to_string()));
                }
            }
        }
        
        Ok(Value::string(base.to_string()))
    }
}

struct SepFunction;
impl Function for SepFunction {
    fn name(&self) -> &str { "sep" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }
        
        if !matches!(args[0], Type::String { .. }) {
            return Err(WdlError::RuntimeError {
                message: format!("sep() first argument must be String"),
            });
        }
        
        if !matches!(args[1], Type::Array { .. }) {
            return Err(WdlError::RuntimeError {
                message: format!("sep() second argument must be Array"),
            });
        }
        
        Ok(Type::string(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let separator = args[0].as_string().ok_or_else(|| WdlError::RuntimeError {
            message: format!("sep() first argument must be String"),
        })?;
        
        if let Value::Array { values, .. } = &args[1] {
            let strings: Result<Vec<String>, _> = values
                .iter()
                .map(|v| v.as_string().map(|s| s.to_string()).ok_or_else(|| WdlError::RuntimeError {
                    message: format!("sep() array elements must be String"),
                }))
                .collect();
            
            Ok(Value::string(strings?.join(&separator)))
        } else {
            Err(WdlError::RuntimeError {
                message: format!("sep() second argument must be Array"),
            })
        }
    }
}

// Type functions

struct DefinedFunction;
impl Function for DefinedFunction {
    fn name(&self) -> &str { "defined" }
    
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
        Ok(Value::boolean(!matches!(args[0], Value::Null { .. })))
    }
}

// Operators

struct AddOperator;
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
                // Convert to float if either is float
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

struct SubtractOperator;
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

struct MultiplyOperator;
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

struct DivideOperator;
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

struct RemainderOperator;
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

// Comparison operators

struct EqualOperator;
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

struct NotEqualOperator;
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

struct LessThanOperator;
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
                // Try numeric comparison
                let a_float = args[0].as_float().or_else(|| args[0].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot compare {:?} and {:?}", args[0], args[1]),
                    })?;
                let b_float = args[1].as_float().or_else(|| args[1].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot compare {:?} and {:?}", args[0], args[1]),
                    })?;
                Ok(Value::boolean(a_float < b_float))
            }
        }
    }
}

struct LessThanEqualOperator;
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
                // Try numeric comparison
                let a_float = args[0].as_float().or_else(|| args[0].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot compare {:?} and {:?}", args[0], args[1]),
                    })?;
                let b_float = args[1].as_float().or_else(|| args[1].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot compare {:?} and {:?}", args[0], args[1]),
                    })?;
                Ok(Value::boolean(a_float <= b_float))
            }
        }
    }
}

struct GreaterThanOperator;
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
                // Try numeric comparison
                let a_float = args[0].as_float().or_else(|| args[0].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot compare {:?} and {:?}", args[0], args[1]),
                    })?;
                let b_float = args[1].as_float().or_else(|| args[1].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot compare {:?} and {:?}", args[0], args[1]),
                    })?;
                Ok(Value::boolean(a_float > b_float))
            }
        }
    }
}

struct GreaterThanEqualOperator;
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
                // Try numeric comparison
                let a_float = args[0].as_float().or_else(|| args[0].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot compare {:?} and {:?}", args[0], args[1]),
                    })?;
                let b_float = args[1].as_float().or_else(|| args[1].as_int().map(|i| i as f64))
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: format!("Cannot compare {:?} and {:?}", args[0], args[1]),
                    })?;
                Ok(Value::boolean(a_float >= b_float))
            }
        }
    }
}

// Logical operators

struct LogicalAndOperator;
impl Function for LogicalAndOperator {
    fn name(&self) -> &str { "_land" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }
        
        if !matches!(args[0], Type::Boolean { .. }) || !matches!(args[1], Type::Boolean { .. }) {
            return Err(WdlError::RuntimeError {
                message: format!("Logical AND requires Boolean arguments"),
            });
        }
        
        Ok(Type::boolean(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let a = args[0].as_bool().ok_or_else(|| WdlError::RuntimeError {
            message: format!("Logical AND requires Boolean arguments"),
        })?;
        let b = args[1].as_bool().ok_or_else(|| WdlError::RuntimeError {
            message: format!("Logical AND requires Boolean arguments"),
        })?;
        
        Ok(Value::boolean(a && b))
    }
}

struct LogicalOrOperator;
impl Function for LogicalOrOperator {
    fn name(&self) -> &str { "_lor" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }
        
        if !matches!(args[0], Type::Boolean { .. }) || !matches!(args[1], Type::Boolean { .. }) {
            return Err(WdlError::RuntimeError {
                message: format!("Logical OR requires Boolean arguments"),
            });
        }
        
        Ok(Type::boolean(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let a = args[0].as_bool().ok_or_else(|| WdlError::RuntimeError {
            message: format!("Logical OR requires Boolean arguments"),
        })?;
        let b = args[1].as_bool().ok_or_else(|| WdlError::RuntimeError {
            message: format!("Logical OR requires Boolean arguments"),
        })?;
        
        Ok(Value::boolean(a || b))
    }
}

struct LogicalNotOperator;
impl Function for LogicalNotOperator {
    fn name(&self) -> &str { "_negate" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }
        
        if !matches!(args[0], Type::Boolean { .. }) {
            return Err(WdlError::RuntimeError {
                message: format!("Logical NOT requires Boolean argument"),
            });
        }
        
        Ok(Type::boolean(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let a = args[0].as_bool().ok_or_else(|| WdlError::RuntimeError {
            message: format!("Logical NOT requires Boolean argument"),
        })?;
        
        Ok(Value::boolean(!a))
    }
}