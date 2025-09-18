//! Mathematical functions for WDL standard library

use crate::stdlib::{create_static_function, Function};
use crate::types::Type;
use crate::value::Value;
use crate::error::WdlError;

/// Create floor function: floor(Float) -> Int
/// Converts a floating point number to integer by rounding down
pub fn create_floor_function() -> Box<dyn Function> {
    create_static_function(
        "floor".to_string(),
        vec![Type::float(false)],
        Type::int(false),
        |args| {
            let value = args[0].as_float().ok_or_else(|| WdlError::RuntimeError {
                message: "floor() expected float argument".to_string(),
            })?;
            Ok(Value::int(value.floor() as i64))
        }
    )
}

/// Create ceil function: ceil(Float) -> Int
/// Converts a floating point number to integer by rounding up
pub fn create_ceil_function() -> Box<dyn Function> {
    create_static_function(
        "ceil".to_string(),
        vec![Type::float(false)],
        Type::int(false),
        |args| {
            let value = args[0].as_float().ok_or_else(|| WdlError::RuntimeError {
                message: "ceil() expected float argument".to_string(),
            })?;
            Ok(Value::int(value.ceil() as i64))
        }
    )
}

/// Create round function: round(Float) -> Int
/// Converts a floating point number to integer by rounding to nearest integer
/// Uses "round half up" behavior (0.5 rounds to 1, -0.5 rounds to 0)
pub fn create_round_function() -> Box<dyn Function> {
    create_static_function(
        "round".to_string(),
        vec![Type::float(false)],
        Type::int(false),
        |args| {
            let value = args[0].as_float().ok_or_else(|| WdlError::RuntimeError {
                message: "round() expected float argument".to_string(),
            })?;
            // Implement "round half up" behavior like miniwdl
            let rounded = if value >= 0.0 {
                (value + 0.5).floor()
            } else {
                (value - 0.5).ceil()
            };
            Ok(Value::int(rounded as i64))
        }
    )
}

/// Create min function that works with both Int and Float
/// Similar to miniwdl's _ArithmeticOperator for min
pub fn create_min_function() -> Box<dyn Function> {
    use crate::stdlib::operators::create_arithmetic_operator;
    create_arithmetic_operator(
        "min".to_string(),
        |left, right| left.min(right),    // i64 min
        |left, right| left.min(right),    // f64 min
    )
}

/// Create max function that works with both Int and Float
/// Similar to miniwdl's _ArithmeticOperator for max
pub fn create_max_function() -> Box<dyn Function> {
    use crate::stdlib::operators::create_arithmetic_operator;
    create_arithmetic_operator(
        "max".to_string(),
        |left, right| left.max(right),    // i64 max
        |left, right| left.max(right),    // f64 max
    )
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::expr::Expression;
    use crate::error::SourcePosition;
    use crate::env::Bindings;
    use crate::stdlib::StdLib;

    #[test]
    fn test_floor_function() {
        let floor_fn = create_floor_function();
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);

        // Test positive number
        let args = vec![Expression::float(pos.clone(), 3.7)];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");
        let result = floor_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 3);

        // Test negative number
        let args = vec![Expression::float(pos.clone(), -2.3)];
        let result = floor_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), -3);
    }

    #[test]
    fn test_ceil_function() {
        let ceil_fn = create_ceil_function();
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);

        // Test positive number
        let args = vec![Expression::float(pos.clone(), 3.2)];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");
        let result = ceil_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 4);

        // Test negative number
        let args = vec![Expression::float(pos.clone(), -2.7)];
        let result = ceil_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), -2);
    }

    #[test]
    fn test_round_function() {
        let round_fn = create_round_function();
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        // Test normal rounding
        let args = vec![Expression::float(pos.clone(), 3.4)];
        let result = round_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 3);

        let args = vec![Expression::float(pos.clone(), 3.6)];
        let result = round_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 4);

        // Test "round half up" behavior
        let args = vec![Expression::float(pos.clone(), 0.5)];
        let result = round_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 1);

        let args = vec![Expression::float(pos.clone(), -0.5)];
        let result = round_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 0);
    }

    #[test]
    fn test_min_function() {
        let min_fn = create_min_function();
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        // Test integer min
        let args = vec![
            Expression::int(pos.clone(), 5),
            Expression::int(pos.clone(), 3),
        ];
        let result = min_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 3);

        // Test float min
        let args = vec![
            Expression::float(pos.clone(), 5.5),
            Expression::float(pos.clone(), 3.2),
        ];
        let result = min_fn.eval(&args, &env, &stdlib).unwrap();
        assert!((result.as_float().unwrap() - 3.2).abs() < f64::EPSILON);

        // Test mixed types (should return float)
        let args = vec![
            Expression::int(pos.clone(), 5),
            Expression::float(pos.clone(), 3.2),
        ];
        let result = min_fn.eval(&args, &env, &stdlib).unwrap();
        assert!((result.as_float().unwrap() - 3.2).abs() < f64::EPSILON);
    }

    #[test]
    fn test_max_function() {
        let max_fn = create_max_function();
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        // Test integer max
        let args = vec![
            Expression::int(pos.clone(), 5),
            Expression::int(pos.clone(), 8),
        ];
        let result = max_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 8);

        // Test float max
        let args = vec![
            Expression::float(pos.clone(), 5.5),
            Expression::float(pos.clone(), 3.2),
        ];
        let result = max_fn.eval(&args, &env, &stdlib).unwrap();
        assert!((result.as_float().unwrap() - 5.5).abs() < f64::EPSILON);

        // Test mixed types (should return float)
        let args = vec![
            Expression::int(pos.clone(), 5),
            Expression::float(pos.clone(), 7.8),
        ];
        let result = max_fn.eval(&args, &env, &stdlib).unwrap();
        assert!((result.as_float().unwrap() - 7.8).abs() < f64::EPSILON);
    }
}