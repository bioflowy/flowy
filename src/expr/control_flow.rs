//! Control flow expressions (conditionals, function applications)

use super::Expression;
use crate::error::SourcePosition;

impl Expression {
    /// Create a new function application
    pub fn apply(pos: SourcePosition, function_name: String, arguments: Vec<Expression>) -> Self {
        Expression::Apply {
            pos,
            function_name,
            arguments,
            inferred_type: None,
        }
    }

    /// Create a new conditional expression
    pub fn if_then_else(
        pos: SourcePosition,
        condition: Expression,
        true_expr: Expression,
        false_expr: Expression,
    ) -> Self {
        Expression::IfThenElse {
            pos,
            condition: Box::new(condition),
            true_expr: Box::new(true_expr),
            false_expr: Box::new(false_expr),
            inferred_type: None,
        }
    }

    /// Create a new array/map subscript access expression
    pub fn at(pos: SourcePosition, expr: Expression, index: Expression) -> Self {
        Expression::At {
            pos,
            expr: Box::new(expr),
            index: Box::new(index),
            inferred_type: None,
        }
    }

    /// Create a new object member access expression
    pub fn get(pos: SourcePosition, expr: Expression, field: String) -> Self {
        Expression::Get {
            pos,
            expr: Box::new(expr),
            field,
            inferred_type: None,
        }
    }
}
