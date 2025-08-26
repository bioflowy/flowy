//! Type checking and validation functions for WDL standard library

use super::Function;
use crate::error::WdlError;
use crate::types::Type;
use crate::value::Value;

/// Defined function - checks if a value is not null
pub struct DefinedFunction;

impl Function for DefinedFunction {
    fn name(&self) -> &str {
        "defined"
    }

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
        Ok(Value::boolean(!matches!(args[0], Value::Null)))
    }
}
