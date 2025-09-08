//! Collection expressions (arrays, pairs, maps, structs)

use super::Expression;
use crate::error::SourcePosition;

impl Expression {
    /// Create a new Array expression
    pub fn array(pos: SourcePosition, items: Vec<Expression>) -> Self {
        Expression::Array {
            pos,
            items,
            inferred_type: None,
        }
    }

    /// Create a new Pair expression
    pub fn pair(pos: SourcePosition, left: Expression, right: Expression) -> Self {
        Expression::Pair {
            pos,
            left: Box::new(left),
            right: Box::new(right),
            inferred_type: None,
        }
    }

    /// Create a new Map expression
    pub fn map(pos: SourcePosition, pairs: Vec<(Expression, Expression)>) -> Self {
        Expression::Map {
            pos,
            pairs,
            inferred_type: None,
        }
    }

    /// Create a new anonymous Struct expression (for object { ... })
    pub fn struct_expr(pos: SourcePosition, members: Vec<(String, Expression)>) -> Self {
        Expression::Struct {
            pos,
            type_name: None, // For anonymous object literals
            members,
            inferred_type: None,
        }
    }

    /// Create a new named Struct expression (for MyStruct { ... })
    pub fn named_struct_expr(pos: SourcePosition, type_name: String, members: Vec<(String, Expression)>) -> Self {
        Expression::Struct {
            pos,
            type_name: Some(type_name), // For named struct literals
            members,
            inferred_type: None,
        }
    }
}
