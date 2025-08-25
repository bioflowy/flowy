//! Literal expression constructors and utilities

use super::{Expression, StringPart};
use crate::error::SourcePosition;

impl Expression {
    /// Create a new Boolean expression
    pub fn boolean(pos: SourcePosition, value: bool) -> Self {
        Expression::Boolean {
            pos,
            value,
            inferred_type: None,
        }
    }
    
    /// Create a new Int expression
    pub fn int(pos: SourcePosition, value: i64) -> Self {
        Expression::Int {
            pos,
            value,
            inferred_type: None,
        }
    }
    
    /// Create a new Float expression
    pub fn float(pos: SourcePosition, value: f64) -> Self {
        Expression::Float {
            pos,
            value,
            inferred_type: None,
        }
    }
    
    /// Create a new String expression
    pub fn string(pos: SourcePosition, parts: Vec<StringPart>) -> Self {
        Expression::String {
            pos,
            parts,
            inferred_type: None,
        }
    }
    
    /// Create a new simple string literal
    pub fn string_literal(pos: SourcePosition, value: String) -> Self {
        Expression::String {
            pos,
            parts: vec![StringPart::Text(value)],
            inferred_type: None,
        }
    }
    
    /// Create a new Null expression
    pub fn null(pos: SourcePosition) -> Self {
        Expression::Null {
            pos,
            inferred_type: None,
        }
    }
    
    /// Create a new Ident expression
    pub fn ident(pos: SourcePosition, name: String) -> Self {
        Expression::Ident {
            pos,
            name,
            inferred_type: None,
        }
    }
}