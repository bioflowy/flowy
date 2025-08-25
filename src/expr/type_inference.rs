//! Type inference logic for expressions

use super::{Expression, ExpressionBase, StringPart, BinaryOperator, UnaryOperator};
use crate::error::{WdlError, HasSourcePosition, MultiErrorContext};
use crate::env::Bindings;
use crate::types::Type;
use std::collections::HashMap;

impl Expression {
    /// Infer the type of this expression and store it
    pub fn infer_type(&mut self, type_env: &Bindings<Type>) -> Result<Type, WdlError> {
        // First, recursively infer types for all children
        let mut errors = MultiErrorContext::new();
        
        match self {
            Expression::String { parts, .. } => {
                for part in parts {
                    if let StringPart::Placeholder { expr, .. } = part {
                        errors.try_with(|| expr.infer_type(type_env));
                    }
                }
            }
            Expression::Array { items, .. } => {
                for item in items {
                    errors.try_with(|| item.infer_type(type_env));
                }
            }
            Expression::Pair { left, right, .. } => {
                errors.try_with(|| left.infer_type(type_env));
                errors.try_with(|| right.infer_type(type_env));
            }
            Expression::Map { pairs, .. } => {
                for (k, v) in pairs {
                    errors.try_with(|| k.infer_type(type_env));
                    errors.try_with(|| v.infer_type(type_env));
                }
            }
            Expression::Struct { members, .. } => {
                for (_, expr) in members {
                    errors.try_with(|| expr.infer_type(type_env));
                }
            }
            Expression::Get { expr, index, .. } => {
                errors.try_with(|| expr.infer_type(type_env));
                errors.try_with(|| index.infer_type(type_env));
            }
            Expression::IfThenElse { condition, true_expr, false_expr, .. } => {
                errors.try_with(|| condition.infer_type(type_env));
                errors.try_with(|| true_expr.infer_type(type_env));
                errors.try_with(|| false_expr.infer_type(type_env));
            }
            Expression::Apply { arguments, .. } => {
                for arg in arguments {
                    errors.try_with(|| arg.infer_type(type_env));
                }
            }
            Expression::BinaryOp { left, right, .. } => {
                errors.try_with(|| left.infer_type(type_env));
                errors.try_with(|| right.infer_type(type_env));
            }
            Expression::UnaryOp { operand, .. } => {
                errors.try_with(|| operand.infer_type(type_env));
            }
            _ => {} // Literals don't need child processing
        }
        
        errors.maybe_raise()?;
        
        // Now infer our own type
        let inferred_type = match self {
            Expression::Boolean { .. } => Type::boolean(false),
            Expression::Int { .. } => Type::int(false),
            Expression::Float { .. } => Type::float(false),
            Expression::String { .. } => Type::string(false),
            Expression::Null { .. } => Type::any().with_optional(true),
            
            Expression::Array { items, .. } => {
                if items.is_empty() {
                    Type::array(Type::any(), false, false)
                } else {
                    // Unify all item types
                    let item_types: Vec<&Type> = items
                        .iter()
                        .filter_map(|item| item.get_type())
                        .collect();
                    let unified_type = crate::types::unify_types(item_types, true, false);
                    Type::array(unified_type, false, !items.is_empty())
                }
            }
            
            Expression::Pair { left, right, .. } => {
                let left_type = left.get_type().cloned().unwrap_or_else(|| Type::any());
                let right_type = right.get_type().cloned().unwrap_or_else(|| Type::any());
                Type::pair(left_type, right_type, false)
            }
            
            Expression::Map { pairs, .. } => {
                if pairs.is_empty() {
                    Type::map(Type::any(), Type::any(), false)
                } else {
                    let key_types: Vec<&Type> = pairs
                        .iter()
                        .filter_map(|(k, _)| k.get_type())
                        .collect();
                    let value_types: Vec<&Type> = pairs
                        .iter()
                        .filter_map(|(_, v)| v.get_type())
                        .collect();
                    
                    let unified_key_type = crate::types::unify_types(key_types, true, false);
                    let unified_value_type = crate::types::unify_types(value_types, true, false);
                    Type::map(unified_key_type, unified_value_type, false)
                }
            }
            
            Expression::Struct { members, .. } => {
                let member_types: HashMap<String, Type> = members
                    .iter()
                    .map(|(name, expr)| {
                        let ty = expr.get_type().cloned().unwrap_or_else(|| Type::any());
                        (name.clone(), ty)
                    })
                    .collect();
                Type::object(member_types)
            }
            
            Expression::Ident { name, .. } => {
                type_env.resolve(name).cloned().unwrap_or_else(|| {
                    // This will be caught as an error in evaluation
                    Type::any()
                })
            }
            
            Expression::Get { expr, .. } => {
                if let Some(expr_type) = expr.get_type() {
                    match expr_type {
                        Type::Array { item_type, .. } => item_type.as_ref().clone(),
                        Type::Map { value_type, .. } => value_type.as_ref().clone(),
                        _ => Type::any(),
                    }
                } else {
                    Type::any()
                }
            }
            
            Expression::IfThenElse { condition, true_expr, false_expr, .. } => {
                // Check condition is boolean
                if let Some(cond_type) = condition.get_type() {
                    if !cond_type.coerces(&Type::boolean(false), true) {
                        return Err(WdlError::static_type_mismatch(
                            HasSourcePosition::source_position(&**condition).clone(),
                            "Boolean".to_string(),
                            cond_type.to_string(),
                            "If condition must be Boolean".to_string(),
                        ));
                    }
                }
                
                // Unify true and false branch types
                let true_type = true_expr.get_type().cloned().unwrap_or_else(|| Type::any());
                let false_type = false_expr.get_type().cloned().unwrap_or_else(|| Type::any());
                crate::types::unify_types(vec![&true_type, &false_type], true, false)
            }
            
            Expression::Apply { function_name, .. } => {
                // For now, just return Any for function applications
                // A full implementation would have a function registry
                match function_name.as_str() {
                    "length" => Type::int(false),
                    "defined" => Type::boolean(false),
                    _ => Type::any(),
                }
            }
            
            Expression::BinaryOp { op, left, right, .. } => {
                let left_type = left.get_type().cloned().unwrap_or_else(|| Type::any());
                let right_type = right.get_type().cloned().unwrap_or_else(|| Type::any());
                
                match op {
                    BinaryOperator::Add | BinaryOperator::Subtract |
                    BinaryOperator::Multiply | BinaryOperator::Divide |
                    BinaryOperator::Modulo => {
                        // Arithmetic operations
                        if left_type.coerces(&Type::int(false), true) && 
                           right_type.coerces(&Type::int(false), true) {
                            Type::int(false)
                        } else {
                            Type::float(false)
                        }
                    }
                    BinaryOperator::Equal | BinaryOperator::NotEqual |
                    BinaryOperator::Less | BinaryOperator::LessEqual |
                    BinaryOperator::Greater | BinaryOperator::GreaterEqual => {
                        // Comparison operations
                        Type::boolean(false)
                    }
                    BinaryOperator::And | BinaryOperator::Or => {
                        // Logical operations
                        Type::boolean(false)
                    }
                }
            }
            
            Expression::UnaryOp { op, operand, .. } => {
                match op {
                    UnaryOperator::Not => Type::boolean(false),
                    UnaryOperator::Negate => {
                        let operand_type = operand.get_type().cloned().unwrap_or_else(|| Type::any());
                        if operand_type.coerces(&Type::int(false), true) {
                            Type::int(false)
                        } else {
                            Type::float(false)
                        }
                    }
                }
            }
        };
        
        // Store the inferred type
        match self {
            Expression::Boolean { inferred_type: ref mut t, .. } => *t = Some(inferred_type.clone()),
            Expression::Int { inferred_type: ref mut t, .. } => *t = Some(inferred_type.clone()),
            Expression::Float { inferred_type: ref mut t, .. } => *t = Some(inferred_type.clone()),
            Expression::String { inferred_type: ref mut t, .. } => *t = Some(inferred_type.clone()),
            Expression::Null { inferred_type: ref mut t, .. } => *t = Some(inferred_type.clone()),
            Expression::Array { inferred_type: ref mut t, .. } => *t = Some(inferred_type.clone()),
            Expression::Pair { inferred_type: ref mut t, .. } => *t = Some(inferred_type.clone()),
            Expression::Map { inferred_type: ref mut t, .. } => *t = Some(inferred_type.clone()),
            Expression::Struct { inferred_type: ref mut t, .. } => *t = Some(inferred_type.clone()),
            Expression::Ident { inferred_type: ref mut t, .. } => *t = Some(inferred_type.clone()),
            Expression::Get { inferred_type: ref mut t, .. } => *t = Some(inferred_type.clone()),
            Expression::IfThenElse { inferred_type: ref mut t, .. } => *t = Some(inferred_type.clone()),
            Expression::Apply { inferred_type: ref mut t, .. } => *t = Some(inferred_type.clone()),
            Expression::BinaryOp { inferred_type: ref mut t, .. } => *t = Some(inferred_type.clone()),
            Expression::UnaryOp { inferred_type: ref mut t, .. } => *t = Some(inferred_type.clone()),
        }
        
        Ok(inferred_type)
    }
}