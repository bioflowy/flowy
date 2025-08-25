//! WDL expressions composing literal values, arithmetic, comparison, conditionals,
//! string interpolation, arrays & maps, and function applications.
//!
//! The abstract syntax tree (AST) for any expression is represented by an enum
//! and associated structs. Expressions can be evaluated to Values given suitable
//! environment bindings.

use crate::error::{SourcePosition, WdlError, HasSourcePosition, SourceNode};
use crate::env::Bindings;
use crate::types::Type;
use crate::value::Value;
use std::collections::HashMap;
use std::fmt;
use serde::{Deserialize, Serialize};

// Re-export submodules
pub mod literals;
pub mod operations;
pub mod control_flow;
pub mod collections;
pub mod evaluation;
pub mod type_inference;

/// Base trait for WDL expressions
pub trait ExpressionBase {
    /// Get the source position of this expression
    fn source_position(&self) -> &SourcePosition;
    
    /// Infer the type of this expression
    fn infer_type(&mut self, type_env: &Bindings<Type>) -> Result<Type, WdlError>;
    
    /// Get the inferred type (must call infer_type first)
    fn get_type(&self) -> Option<&Type>;
    
    /// Type-check this expression against an expected type
    fn typecheck(&self, expected: &Type) -> Result<(), WdlError>;
    
    /// Evaluate this expression in the given environment with standard library
    fn eval(&self, env: &Bindings<Value>, stdlib: &crate::stdlib::StdLib) -> Result<Value, WdlError>;
    
    /// Get all child expressions
    fn children(&self) -> Vec<&Expression>;
    
    /// Check if this is a literal constant
    fn literal(&self) -> Option<Value>;
}

/// WDL expression AST node
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum Expression {
    /// Boolean literal (true/false)
    Boolean {
        pos: SourcePosition,
        value: bool,
        inferred_type: Option<Type>,
    },
    
    /// Integer literal
    Int {
        pos: SourcePosition,
        value: i64,
        inferred_type: Option<Type>,
    },
    
    /// Float literal
    Float {
        pos: SourcePosition,
        value: f64,
        inferred_type: Option<Type>,
    },
    
    /// String literal (may contain placeholders)
    String {
        pos: SourcePosition,
        parts: Vec<StringPart>,
        inferred_type: Option<Type>,
    },
    
    /// Null literal (None in WDL)
    Null {
        pos: SourcePosition,
        inferred_type: Option<Type>,
    },
    
    /// Array literal [item1, item2, ...]
    Array {
        pos: SourcePosition,
        items: Vec<Expression>,
        inferred_type: Option<Type>,
    },
    
    /// Pair literal (left, right)
    Pair {
        pos: SourcePosition,
        left: Box<Expression>,
        right: Box<Expression>,
        inferred_type: Option<Type>,
    },
    
    /// Map literal {key1: value1, key2: value2, ...}
    Map {
        pos: SourcePosition,
        pairs: Vec<(Expression, Expression)>,
        inferred_type: Option<Type>,
    },
    
    /// Struct literal {member1: value1, member2: value2, ...}
    Struct {
        pos: SourcePosition,
        members: Vec<(String, Expression)>,
        inferred_type: Option<Type>,
    },
    
    /// Variable identifier reference
    Ident {
        pos: SourcePosition,
        name: String,
        inferred_type: Option<Type>,
    },
    
    /// Array/map access: expr[index]
    Get {
        pos: SourcePosition,
        expr: Box<Expression>,
        index: Box<Expression>,
        inferred_type: Option<Type>,
    },
    
    /// Conditional expression: if condition then true_expr else false_expr
    IfThenElse {
        pos: SourcePosition,
        condition: Box<Expression>,
        true_expr: Box<Expression>,
        false_expr: Box<Expression>,
        inferred_type: Option<Type>,
    },
    
    /// Function application: function_name(arg1, arg2, ...)
    Apply {
        pos: SourcePosition,
        function_name: String,
        arguments: Vec<Expression>,
        inferred_type: Option<Type>,
    },
    
    /// Binary operations: +, -, *, /, %, ==, !=, <, <=, >, >=, &&, ||
    BinaryOp {
        pos: SourcePosition,
        op: BinaryOperator,
        left: Box<Expression>,
        right: Box<Expression>,
        inferred_type: Option<Type>,
    },
    
    /// Unary operations: !, -
    UnaryOp {
        pos: SourcePosition,
        op: UnaryOperator,
        operand: Box<Expression>,
        inferred_type: Option<Type>,
    },
}

/// Parts of a string literal (literal text or placeholder)
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum StringPart {
    /// Literal text
    Text(String),
    /// Expression placeholder ~{expr}
    Placeholder {
        expr: Box<Expression>,
        options: HashMap<String, String>, // sep, true, false, default options
    },
}

/// Binary operators
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum BinaryOperator {
    // Arithmetic
    Add,
    Subtract,
    Multiply,
    Divide,
    Modulo,
    
    // Comparison  
    Equal,
    NotEqual,
    Less,
    LessEqual,
    Greater,
    GreaterEqual,
    
    // Logical
    And,
    Or,
}

/// Unary operators
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum UnaryOperator {
    /// Logical NOT (!)
    Not,
    /// Numeric negation (-)
    Negate,
}

impl HasSourcePosition for Expression {
    fn source_position(&self) -> &SourcePosition {
        match self {
            Expression::Boolean { pos, .. } => pos,
            Expression::Int { pos, .. } => pos,
            Expression::Float { pos, .. } => pos,
            Expression::String { pos, .. } => pos,
            Expression::Null { pos, .. } => pos,
            Expression::Array { pos, .. } => pos,
            Expression::Pair { pos, .. } => pos,
            Expression::Map { pos, .. } => pos,
            Expression::Struct { pos, .. } => pos,
            Expression::Ident { pos, .. } => pos,
            Expression::Get { pos, .. } => pos,
            Expression::IfThenElse { pos, .. } => pos,
            Expression::Apply { pos, .. } => pos,
            Expression::BinaryOp { pos, .. } => pos,
            Expression::UnaryOp { pos, .. } => pos,
        }
    }
    
    fn set_source_position(&mut self, new_pos: SourcePosition) {
        match self {
            Expression::Boolean { pos, .. } => *pos = new_pos,
            Expression::Int { pos, .. } => *pos = new_pos,
            Expression::Float { pos, .. } => *pos = new_pos,
            Expression::String { pos, .. } => *pos = new_pos,
            Expression::Null { pos, .. } => *pos = new_pos,
            Expression::Array { pos, .. } => *pos = new_pos,
            Expression::Pair { pos, .. } => *pos = new_pos,
            Expression::Map { pos, .. } => *pos = new_pos,
            Expression::Struct { pos, .. } => *pos = new_pos,
            Expression::Ident { pos, .. } => *pos = new_pos,
            Expression::Get { pos, .. } => *pos = new_pos,
            Expression::IfThenElse { pos, .. } => *pos = new_pos,
            Expression::Apply { pos, .. } => *pos = new_pos,
            Expression::BinaryOp { pos, .. } => *pos = new_pos,
            Expression::UnaryOp { pos, .. } => *pos = new_pos,
        }
    }
}

impl SourceNode for Expression {
    fn parent(&self) -> Option<&dyn SourceNode> {
        // For simplicity, we don't track parent relationships in this implementation
        None
    }
    
    fn set_parent(&mut self, _parent: Option<&dyn SourceNode>) {
        // For simplicity, we don't track parent relationships in this implementation
    }
    
    fn children(&self) -> Vec<&dyn SourceNode> {
        let mut children: Vec<&dyn SourceNode> = Vec::new();
        
        match self {
            Expression::String { parts, .. } => {
                for part in parts {
                    if let StringPart::Placeholder { expr, .. } = part {
                        children.push(expr.as_ref());
                    }
                }
            }
            Expression::Array { items, .. } => {
                for item in items {
                    children.push(item);
                }
            }
            Expression::Pair { left, right, .. } => {
                children.push(left.as_ref());
                children.push(right.as_ref());
            }
            Expression::Map { pairs, .. } => {
                for (k, v) in pairs {
                    children.push(k);
                    children.push(v);
                }
            }
            Expression::Struct { members, .. } => {
                for (_, expr) in members {
                    children.push(expr);
                }
            }
            Expression::Get { expr, index, .. } => {
                children.push(expr.as_ref());
                children.push(index.as_ref());
            }
            Expression::IfThenElse { condition, true_expr, false_expr, .. } => {
                children.push(condition.as_ref());
                children.push(true_expr.as_ref());
                children.push(false_expr.as_ref());
            }
            Expression::Apply { arguments, .. } => {
                for arg in arguments {
                    children.push(arg);
                }
            }
            Expression::BinaryOp { left, right, .. } => {
                children.push(left.as_ref());
                children.push(right.as_ref());
            }
            Expression::UnaryOp { operand, .. } => {
                children.push(operand.as_ref());
            }
            _ => {} // Literals have no children
        }
        
        children
    }
}

impl fmt::Display for Expression {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Expression::Boolean { value, .. } => write!(f, "{}", if *value { "true" } else { "false" }),
            Expression::Int { value, .. } => write!(f, "{}", value),
            Expression::Float { value, .. } => write!(f, "{}", value),
            Expression::String { parts, .. } => {
                write!(f, "\"")?;
                for part in parts {
                    match part {
                        StringPart::Text(text) => write!(f, "{}", text)?,
                        StringPart::Placeholder { expr, .. } => write!(f, "~{{{}}}", expr)?,
                    }
                }
                write!(f, "\"")
            }
            Expression::Null { .. } => write!(f, "None"),
            Expression::Array { items, .. } => {
                write!(f, "[")?;
                for (i, item) in items.iter().enumerate() {
                    if i > 0 { write!(f, ", ")?; }
                    write!(f, "{}", item)?;
                }
                write!(f, "]")
            }
            Expression::Pair { left, right, .. } => {
                write!(f, "({}, {})", left, right)
            }
            Expression::Map { pairs, .. } => {
                write!(f, "{{")?;
                for (i, (k, v)) in pairs.iter().enumerate() {
                    if i > 0 { write!(f, ", ")?; }
                    write!(f, "{}: {}", k, v)?;
                }
                write!(f, "}}")
            }
            Expression::Struct { members, .. } => {
                write!(f, "{{")?;
                for (i, (name, expr)) in members.iter().enumerate() {
                    if i > 0 { write!(f, ", ")?; }
                    write!(f, "{}: {}", name, expr)?;
                }
                write!(f, "}}")
            }
            Expression::Ident { name, .. } => write!(f, "{}", name),
            Expression::Get { expr, index, .. } => write!(f, "{}[{}]", expr, index),
            Expression::IfThenElse { condition, true_expr, false_expr, .. } => {
                write!(f, "if {} then {} else {}", condition, true_expr, false_expr)
            }
            Expression::Apply { function_name, arguments, .. } => {
                write!(f, "{}(", function_name)?;
                for (i, arg) in arguments.iter().enumerate() {
                    if i > 0 { write!(f, ", ")?; }
                    write!(f, "{}", arg)?;
                }
                write!(f, ")")
            }
            Expression::BinaryOp { op, left, right, .. } => {
                let op_str = match op {
                    BinaryOperator::Add => "+",
                    BinaryOperator::Subtract => "-",
                    BinaryOperator::Multiply => "*",
                    BinaryOperator::Divide => "/",
                    BinaryOperator::Modulo => "%",
                    BinaryOperator::Equal => "==",
                    BinaryOperator::NotEqual => "!=",
                    BinaryOperator::Less => "<",
                    BinaryOperator::LessEqual => "<=",
                    BinaryOperator::Greater => ">",
                    BinaryOperator::GreaterEqual => ">=",
                    BinaryOperator::And => "&&",
                    BinaryOperator::Or => "||",
                };
                write!(f, "{} {} {}", left, op_str, right)
            }
            Expression::UnaryOp { op, operand, .. } => {
                let op_str = match op {
                    UnaryOperator::Not => "!",
                    UnaryOperator::Negate => "-",
                };
                write!(f, "{}{}", op_str, operand)
            }
        }
    }
}

impl Expression {
    /// Convenience method to get source position
    pub fn pos(&self) -> &SourcePosition {
        HasSourcePosition::source_position(self)
    }
}

// Tests module
#[cfg(test)]
pub mod tests;

#[cfg(test)]
mod basic_tests {
    use super::*;
    use crate::env::Bindings;
    use crate::types::Type;
    use crate::value::Value;
    use std::collections::HashMap;

    #[test]
    fn test_literal_expressions() {
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        
        // Test Boolean
        let bool_expr = Expression::boolean(pos.clone(), true);
        assert_eq!(bool_expr.literal(), Some(Value::boolean(true)));
        
        // Test Int
        let int_expr = Expression::int(pos.clone(), 42);
        assert_eq!(int_expr.literal(), Some(Value::int(42)));
        
        // Test Float
        let float_expr = Expression::float(pos.clone(), 3.14);
        assert_eq!(float_expr.literal(), Some(Value::float(3.14)));
        
        // Test String
        let string_expr = Expression::string_literal(pos.clone(), "hello".to_string());
        assert_eq!(string_expr.literal(), Some(Value::string("hello".to_string())));
        
        // Test Null
        let null_expr = Expression::null(pos.clone());
        assert_eq!(null_expr.literal(), Some(Value::null()));
    }
    
    #[test]
    fn test_expression_evaluation() {
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let mut env = Bindings::new();
        env = env.bind("x".to_string(), Value::int(10), None);
        
        // Test identifier resolution
        let ident_expr = Expression::ident(pos.clone(), "x".to_string());
        let result = ident_expr.eval(&env).unwrap();
        assert_eq!(result.as_int(), Some(10));
        
        // Test binary operation
        let add_expr = Expression::binary_op(
            pos.clone(),
            BinaryOperator::Add,
            Expression::int(pos.clone(), 5),
            Expression::ident(pos.clone(), "x".to_string()),
        );
        
        let result = add_expr.eval(&env).unwrap();
        assert_eq!(result.as_int(), Some(15));
    }
    
    #[test]
    fn test_array_expression() {
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let env = Bindings::new();
        
        let array_expr = Expression::array(pos.clone(), vec![
            Expression::int(pos.clone(), 1),
            Expression::int(pos.clone(), 2),
            Expression::int(pos.clone(), 3),
        ]);
        
        let result = array_expr.eval(&env).unwrap();
        if let Some(values) = result.as_array() {
            assert_eq!(values.len(), 3);
            assert_eq!(values[0].as_int(), Some(1));
            assert_eq!(values[1].as_int(), Some(2));
            assert_eq!(values[2].as_int(), Some(3));
        } else {
            panic!("Expected array result");
        }
    }
    
    #[test]
    fn test_conditional_expression() {
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let env = Bindings::new();
        
        let if_expr = Expression::if_then_else(
            pos.clone(),
            Expression::boolean(pos.clone(), true),
            Expression::int(pos.clone(), 1),
            Expression::int(pos.clone(), 2),
        );
        
        let result = if_expr.eval(&env).unwrap();
        assert_eq!(result.as_int(), Some(1));
        
        let if_expr_false = Expression::if_then_else(
            pos.clone(),
            Expression::boolean(pos.clone(), false),
            Expression::int(pos.clone(), 1),
            Expression::int(pos.clone(), 2),
        );
        
        let result = if_expr_false.eval(&env).unwrap();
        assert_eq!(result.as_int(), Some(2));
    }
    
    #[test]
    fn test_function_application() {
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let env = Bindings::new();
        
        let length_expr = Expression::apply(
            pos.clone(),
            "length".to_string(),
            vec![Expression::array(pos.clone(), vec![
                Expression::int(pos.clone(), 1),
                Expression::int(pos.clone(), 2),
            ])],
        );
        
        let result = length_expr.eval(&env).unwrap();
        assert_eq!(result.as_int(), Some(2));
    }
    
    #[test]
    fn test_type_inference() {
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let type_env = Bindings::new();
        
        let mut int_expr = Expression::int(pos.clone(), 42);
        let inferred_type = int_expr.infer_type(&type_env).unwrap();
        assert_eq!(inferred_type, Type::int(false));
        
        let mut array_expr = Expression::array(pos.clone(), vec![
            Expression::int(pos.clone(), 1),
            Expression::int(pos.clone(), 2),
        ]);
        let inferred_type = array_expr.infer_type(&type_env).unwrap();
        assert_eq!(inferred_type, Type::array(Type::int(false), false, true));
    }
    
    #[test]
    fn test_string_interpolation() {
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let env = Bindings::new().bind("name".to_string(), Value::string("world".to_string()), None);
        
        let string_expr = Expression::string(pos.clone(), vec![
            StringPart::Text("Hello ".to_string()),
            StringPart::Placeholder {
                expr: Box::new(Expression::ident(pos.clone(), "name".to_string())),
                options: HashMap::new(),
            },
            StringPart::Text("!".to_string()),
        ]);
        
        let result = string_expr.eval(&env).unwrap();
        assert_eq!(result.as_string(), Some("Hello world!"));
    }

    #[test]
    fn test_display_formatting() {
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        
        let bool_expr = Expression::boolean(pos.clone(), true);
        assert_eq!(format!("{}", bool_expr), "true");
        
        let add_expr = Expression::binary_op(
            pos.clone(),
            BinaryOperator::Add,
            Expression::int(pos.clone(), 1),
            Expression::int(pos.clone(), 2),
        );
        assert_eq!(format!("{}", add_expr), "1 + 2");
    }
}