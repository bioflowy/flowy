//! Type inference for WDL expressions

use super::{BinaryOperator, Expression, ExpressionBase, StringPart, UnaryOperator};
use crate::env::Bindings;
use crate::error::{HasSourcePosition, MultiErrorContext, WdlError};
use crate::types::Type;
use std::collections::HashMap;

impl Expression {
    /// Extract field name from index expression for object field access
    fn extract_field_name(index: &Expression) -> Option<String> {
        match index {
            Expression::String { parts, .. } => {
                // Handle string literals like obj["field"] or obj.field (parsed as string)
                if parts.len() == 1 {
                    if let crate::expr::StringPart::Text(text) = &parts[0] {
                        Some(text.clone())
                    } else {
                        None
                    }
                } else {
                    None
                }
            }
            _ => None,
        }
    }
    /// Infer the type of this expression and store it
    pub fn infer_type(
        &mut self,
        type_env: &Bindings<Type>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Type, WdlError> {
        // First, recursively infer types for all children
        let mut errors = MultiErrorContext::new();

        match self {
            Expression::String { parts, .. } => {
                for part in parts {
                    if let StringPart::Placeholder { expr, .. } = part {
                        errors.try_with(|| expr.infer_type(type_env, stdlib));
                    }
                }
            }
            Expression::Array { items, .. } => {
                for item in items {
                    errors.try_with(|| item.infer_type(type_env, stdlib));
                }
            }
            Expression::Pair { left, right, .. } => {
                errors.try_with(|| left.infer_type(type_env, stdlib));
                errors.try_with(|| right.infer_type(type_env, stdlib));
            }
            Expression::Map { pairs, .. } => {
                for (k, v) in pairs {
                    errors.try_with(|| k.infer_type(type_env, stdlib));
                    errors.try_with(|| v.infer_type(type_env, stdlib));
                }
            }
            Expression::Struct { members, .. } => {
                for (_, expr) in members {
                    errors.try_with(|| expr.infer_type(type_env, stdlib));
                }
            }
            Expression::Get { expr, index, .. } => {
                errors.try_with(|| expr.infer_type(type_env, stdlib));
                errors.try_with(|| index.infer_type(type_env, stdlib));
            }
            Expression::IfThenElse {
                condition,
                true_expr,
                false_expr,
                ..
            } => {
                errors.try_with(|| condition.infer_type(type_env, stdlib));
                errors.try_with(|| true_expr.infer_type(type_env, stdlib));
                errors.try_with(|| false_expr.infer_type(type_env, stdlib));
            }
            Expression::Apply { arguments, .. } => {
                for arg in arguments {
                    errors.try_with(|| arg.infer_type(type_env, stdlib));
                }
            }
            Expression::BinaryOp { left, right, .. } => {
                errors.try_with(|| left.infer_type(type_env, stdlib));
                errors.try_with(|| right.infer_type(type_env, stdlib));
            }
            Expression::UnaryOp { operand, .. } => {
                errors.try_with(|| operand.infer_type(type_env, stdlib));
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
                    let item_types: Vec<&Type> =
                        items.iter().filter_map(|item| item.get_type()).collect();
                    let unified_type = crate::types::unify_types(item_types, true, false);
                    Type::array(unified_type, false, !items.is_empty())
                }
            }

            Expression::Pair { left, right, .. } => {
                let left_type = left.get_type().cloned().unwrap_or_else(Type::any);
                let right_type = right.get_type().cloned().unwrap_or_else(Type::any);
                Type::pair(left_type, right_type, false)
            }

            Expression::Map { pairs, .. } => {
                if pairs.is_empty() {
                    Type::map(Type::any(), Type::any(), false)
                } else {
                    let key_types: Vec<&Type> =
                        pairs.iter().filter_map(|(k, _)| k.get_type()).collect();
                    let value_types: Vec<&Type> =
                        pairs.iter().filter_map(|(_, v)| v.get_type()).collect();

                    let unified_key_type = crate::types::unify_types(key_types, true, false);
                    let unified_value_type = crate::types::unify_types(value_types, true, false);
                    Type::map(unified_key_type, unified_value_type, false)
                }
            }

            Expression::Struct { members, .. } => {
                let mut member_types: HashMap<String, Type> = HashMap::new();

                // Add types for explicitly provided members
                for (name, expr) in members {
                    let ty = expr.get_type().cloned().unwrap_or_else(Type::any);
                    member_types.insert(name.clone(), ty);
                }

                // For struct literals, we need to check if there's a known struct type
                // that matches the provided fields and add missing optional fields
                // For now, we'll create the type based on provided members only
                // The evaluation step will handle adding missing optional fields

                Type::object(member_types)
            }

            Expression::Ident { name, pos, .. } => type_env
                .resolve(name)
                .cloned()
                .ok_or_else(|| WdlError::unknown_identifier_error(pos.clone(), name.to_string()))?,

            Expression::Get {
                expr, index, pos, ..
            } => {
                if let Some(expr_type) = expr.get_type() {
                    match expr_type {
                        Type::Array { item_type, .. } => item_type.as_ref().clone(),
                        Type::Map { value_type, .. } => value_type.as_ref().clone(),
                        Type::Object { members, .. } => {
                            // Object field access: obj.field or obj["field"]
                            // Extract field name from index expression
                            if let Some(field_name) = Self::extract_field_name(index) {
                                if let Some(field_type) = members.get(&field_name) {
                                    field_type.clone()
                                } else {
                                    return Err(WdlError::no_such_member_error(
                                        pos.clone(),
                                        field_name,
                                    ));
                                }
                            } else {
                                return Err(WdlError::static_type_mismatch(
                                    pos.clone(),
                                    "String literal".to_string(),
                                    "Dynamic expression".to_string(),
                                    "Object field access requires a string literal".to_string(),
                                ));
                            }
                        }
                        _ => {
                            return Err(WdlError::static_type_mismatch(
                                pos.clone(),
                                "Array, Map, or Object".to_string(),
                                expr_type.to_string(),
                                "Get operation requires indexable type".to_string(),
                            ));
                        }
                    }
                } else {
                    return Err(WdlError::static_type_mismatch(
                        pos.clone(),
                        "Array, Map, or Object".to_string(),
                        "Unknown".to_string(),
                        "Cannot infer type for get operation".to_string(),
                    ));
                }
            }

            Expression::IfThenElse {
                condition,
                true_expr,
                false_expr,
                ..
            } => {
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

                // Get branch types
                let true_type = true_expr.get_type().cloned().unwrap_or_else(Type::any);
                let false_type = false_expr.get_type().cloned().unwrap_or_else(Type::any);
                // Handle cases similar to miniwdl logic
                let branch_types = vec![&true_type, &false_type];

                // Check if any branch is Any (non-optional) - like read_json()
                if branch_types.iter().any(|ty| {
                    matches!(
                        ty,
                        Type::Any {
                            optional: false,
                            ..
                        }
                    )
                }) {
                    return Ok(Type::any());
                }

                // Check if both branches are Any (optional) - both are None
                if branch_types
                    .iter()
                    .all(|ty| matches!(ty, Type::Any { optional: true, .. }))
                {
                    return Ok(Type::any().with_optional(true));
                }

                // Try to unify the types
                let unified_type = crate::types::unify_types(branch_types, true, false);

                // Check if unification failed (returns Any when it can't unify)
                if matches!(unified_type, Type::Any { .. })
                    && !matches!(true_type, Type::Any { .. })
                    && !matches!(false_type, Type::Any { .. })
                {
                    return Err(WdlError::static_type_mismatch(
                        HasSourcePosition::source_position(self).clone(),
                        true_type.to_string(),
                        false_type.to_string(),
                        "Unable to unify consequent & alternative types".to_string(),
                    ));
                }

                unified_type
            }

            Expression::Apply {
                function_name,
                arguments,
                ..
            } => {
                // Use stdlib function infer_type
                if let Some(func) = stdlib.get_function(function_name) {
                    let arg_types: Vec<Type> = arguments
                        .iter()
                        .filter_map(|arg| arg.get_type().cloned())
                        .collect();
                    match func.infer_type(&arg_types) {
                        Ok(typ) => typ,
                        Err(_) => Type::any(), // Fall back to Any if inference fails
                    }
                } else {
                    Type::any() // Unknown function
                }
            }

            Expression::BinaryOp {
                op, left, right, ..
            } => {
                let left_type = left.get_type().cloned().unwrap_or_else(Type::any);
                let right_type = right.get_type().cloned().unwrap_or_else(Type::any);

                match op {
                    BinaryOperator::Add
                    | BinaryOperator::Subtract
                    | BinaryOperator::Multiply
                    | BinaryOperator::Divide
                    | BinaryOperator::Modulo => {
                        // Arithmetic operations
                        if left_type.coerces(&Type::int(false), true)
                            && right_type.coerces(&Type::int(false), true)
                        {
                            Type::int(false)
                        } else {
                            Type::float(false)
                        }
                    }
                    BinaryOperator::Equal
                    | BinaryOperator::NotEqual
                    | BinaryOperator::Less
                    | BinaryOperator::LessEqual
                    | BinaryOperator::Greater
                    | BinaryOperator::GreaterEqual => {
                        // Comparison operations
                        Type::boolean(false)
                    }
                    BinaryOperator::And | BinaryOperator::Or => {
                        // Logical operations
                        Type::boolean(false)
                    }
                }
            }

            Expression::UnaryOp { op, operand, .. } => match op {
                UnaryOperator::Not => Type::boolean(false),
                UnaryOperator::Negate => {
                    let operand_type = operand.get_type().cloned().unwrap_or_else(Type::any);
                    if operand_type.coerces(&Type::int(false), true) {
                        Type::int(false)
                    } else {
                        Type::float(false)
                    }
                }
            },
        };

        // Store the inferred type in the expression
        let computed_type = inferred_type.clone();
        match self {
            Expression::Boolean {
                inferred_type: stored_type,
                ..
            } => *stored_type = Some(computed_type.clone()),
            Expression::Int {
                inferred_type: stored_type,
                ..
            } => *stored_type = Some(computed_type.clone()),
            Expression::Float {
                inferred_type: stored_type,
                ..
            } => *stored_type = Some(computed_type.clone()),
            Expression::String {
                inferred_type: stored_type,
                ..
            } => *stored_type = Some(computed_type.clone()),
            Expression::Null {
                inferred_type: stored_type,
                ..
            } => *stored_type = Some(computed_type.clone()),
            Expression::Array {
                inferred_type: stored_type,
                ..
            } => *stored_type = Some(computed_type.clone()),
            Expression::Pair {
                inferred_type: stored_type,
                ..
            } => *stored_type = Some(computed_type.clone()),
            Expression::Map {
                inferred_type: stored_type,
                ..
            } => *stored_type = Some(computed_type.clone()),
            Expression::Struct {
                inferred_type: stored_type,
                ..
            } => *stored_type = Some(computed_type.clone()),
            Expression::Ident {
                inferred_type: stored_type,
                ..
            } => *stored_type = Some(computed_type.clone()),
            Expression::Get {
                inferred_type: stored_type,
                ..
            } => *stored_type = Some(computed_type.clone()),
            Expression::IfThenElse {
                inferred_type: stored_type,
                ..
            } => *stored_type = Some(computed_type.clone()),
            Expression::Apply {
                inferred_type: stored_type,
                ..
            } => *stored_type = Some(computed_type.clone()),
            Expression::BinaryOp {
                inferred_type: stored_type,
                ..
            } => *stored_type = Some(computed_type.clone()),
            Expression::UnaryOp {
                inferred_type: stored_type,
                ..
            } => *stored_type = Some(computed_type),
        }

        Ok(inferred_type)
    }
}
