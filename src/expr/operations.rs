//! Binary and unary operation expressions

use super::{BinaryOperator, Expression, UnaryOperator};
use crate::error::SourcePosition;

impl Expression {
    /// Create a new binary operation
    pub fn binary_op(
        pos: SourcePosition,
        op: BinaryOperator,
        left: Expression,
        right: Expression,
    ) -> Self {
        Expression::BinaryOp {
            pos,
            op,
            left: Box::new(left),
            right: Box::new(right),
            inferred_type: None,
        }
    }

    /// Create a new unary operation
    pub fn unary_op(pos: SourcePosition, op: UnaryOperator, operand: Expression) -> Self {
        Expression::UnaryOp {
            pos,
            op,
            operand: Box::new(operand),
            inferred_type: None,
        }
    }
}
