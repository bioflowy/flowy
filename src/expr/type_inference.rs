//! Type inference for WDL expressions

use super::{BinaryOperator, Expression, ExpressionBase, StringPart, UnaryOperator};
use crate::env::Bindings;
use crate::error::{HasSourcePosition, MultiErrorContext, WdlError};
use crate::types::Type;
use std::collections::HashMap;

impl Expression {
    /// Infer the type of this expression and store it
    pub fn infer_type(
        &mut self,
        type_env: &Bindings<Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        // First, recursively infer types for all children
        let mut errors = MultiErrorContext::new();

        match self {
            Expression::String { parts, .. } => {
                for part in parts {
                    if let StringPart::Placeholder { expr, .. } = part {
                        errors.try_with(|| expr.infer_type(type_env, stdlib, struct_typedefs));
                    }
                }
            }
            Expression::Array { items, .. } => {
                for item in items {
                    errors.try_with(|| item.infer_type(type_env, stdlib, struct_typedefs));
                }
            }
            Expression::Pair { left, right, .. } => {
                errors.try_with(|| left.infer_type(type_env, stdlib, struct_typedefs));
                errors.try_with(|| right.infer_type(type_env, stdlib, struct_typedefs));
            }
            Expression::Map { pairs, .. } => {
                for (k, v) in pairs.iter_mut() {
                    errors.try_with(|| k.infer_type(type_env, stdlib, struct_typedefs));
                    errors.try_with(|| v.infer_type(type_env, stdlib, struct_typedefs));
                }
            }
            Expression::Struct { members, .. } => {
                for (_, expr) in members {
                    errors.try_with(|| expr.infer_type(type_env, stdlib, struct_typedefs));
                }
            }
            Expression::At { expr, index, .. } => {
                errors.try_with(|| expr.infer_type(type_env, stdlib, struct_typedefs));
                errors.try_with(|| index.infer_type(type_env, stdlib, struct_typedefs));
            }
            Expression::Get { expr, .. } => {
                errors.try_with(|| expr.infer_type(type_env, stdlib, struct_typedefs));
            }
            Expression::IfThenElse {
                condition,
                true_expr,
                false_expr,
                ..
            } => {
                errors.try_with(|| condition.infer_type(type_env, stdlib, struct_typedefs));
                errors.try_with(|| true_expr.infer_type(type_env, stdlib, struct_typedefs));
                errors.try_with(|| false_expr.infer_type(type_env, stdlib, struct_typedefs));
            }
            Expression::Apply { arguments, .. } => {
                for arg in arguments {
                    errors.try_with(|| arg.infer_type(type_env, stdlib, struct_typedefs));
                }
            }
            Expression::BinaryOp { left, right, .. } => {
                errors.try_with(|| left.infer_type(type_env, stdlib, struct_typedefs));
                errors.try_with(|| right.infer_type(type_env, stdlib, struct_typedefs));
            }
            Expression::UnaryOp { operand, .. } => {
                errors.try_with(|| operand.infer_type(type_env, stdlib, struct_typedefs));
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
                    // First, check if this could be a struct literal
                    // All keys must be string literals for struct consideration
                    let mut member_types: HashMap<String, Type> = HashMap::new();
                    let mut could_be_struct = true;

                    for (key_expr, value_expr) in pairs.iter() {
                        // Check if key is a string literal
                        if let Expression::String { parts, .. } = key_expr {
                            if parts.len() == 1 {
                                if let crate::expr::StringPart::Text(key_name) = &parts[0] {
                                    if let Some(value_type) = value_expr.get_type() {
                                        member_types.insert(key_name.clone(), value_type.clone());
                                    } else {
                                        could_be_struct = false;
                                        break;
                                    }
                                } else {
                                    could_be_struct = false;
                                    break;
                                }
                            } else {
                                could_be_struct = false;
                                break;
                            }
                        } else {
                            could_be_struct = false;
                            break;
                        }
                    }

                    // If this could be a struct, try to match with struct_typedefs
                    let mut found_struct_type: Option<Type> = None;
                    if could_be_struct {
                        for struct_def in struct_typedefs {
                            // Check if all provided members match a known struct type
                            let mut matches = true;
                            for (provided_name, provided_type) in &member_types {
                                if let Some(expected_type) = struct_def.members.get(provided_name) {
                                    if !provided_type.coerces(expected_type, true) {
                                        matches = false;
                                        break;
                                    }
                                } else {
                                    matches = false;
                                    break;
                                }
                            }

                            // If this struct definition matches, use it
                            if matches && member_types.len() <= struct_def.members.len() {
                                found_struct_type = Some(Type::StructInstance {
                                    type_name: struct_def.name.clone(),
                                    members: Some(struct_def.members.clone()),
                                    optional: false,
                                });
                                break;
                            }
                        }
                    }

                    // Use struct type if found, otherwise fall back to Map
                    if let Some(struct_type) = found_struct_type {
                        struct_type
                    } else {
                        // No struct match found, treat as regular map
                        let key_types: Vec<&Type> =
                            pairs.iter().filter_map(|(k, _)| k.get_type()).collect();
                        let value_types: Vec<&Type> =
                            pairs.iter().filter_map(|(_, v)| v.get_type()).collect();

                        let unified_key_type = crate::types::unify_types(key_types, true, false);
                        let unified_value_type =
                            crate::types::unify_types(value_types, true, false);
                        Type::map(unified_key_type, unified_value_type, false)
                    }
                }
            }

            Expression::Struct {
                type_name, members, ..
            } => Self::infer_struct_type(type_name, members, type_env, stdlib, struct_typedefs)?,

            Expression::Ident { name, pos, .. } => type_env
                .resolve(name)
                .cloned()
                .ok_or_else(|| WdlError::unknown_identifier_error(pos.clone(), name.to_string()))?,

            Expression::At {
                expr, index, pos, ..
            } => {
                // Array/Map subscript access
                if let Some(expr_type) = expr.get_type() {
                    match expr_type {
                        Type::Array { item_type, .. } => item_type.as_ref().clone(),
                        Type::Map { value_type, .. } => value_type.as_ref().clone(),
                        _ => {
                            return Err(WdlError::static_type_mismatch(
                                pos.clone(),
                                "Array or Map".to_string(),
                                expr_type.to_string(),
                                "Subscript operation requires array or map type".to_string(),
                            ));
                        }
                    }
                } else {
                    return Err(WdlError::static_type_mismatch(
                        pos.clone(),
                        "Array or Map".to_string(),
                        "Unknown".to_string(),
                        "Cannot infer type for subscript operation".to_string(),
                    ));
                }
            }

            Expression::Get {
                expr, field, pos, ..
            } => {
                // Object member access
                if let Some(expr_type) = expr.get_type() {
                    match expr_type {
                        // Special case: Array of call outputs (scatter context)
                        Type::Array {
                            item_type,
                            optional,
                            nonempty,
                        } => {
                            if let Type::Object {
                                members,
                                is_call_output: true,
                            } = item_type.as_ref()
                            {
                                // This is an array of call outputs from a scatter
                                // call.output syntax should return Array[OutputType]
                                if let Some(field_type) = members.get(field) {
                                    Type::Array {
                                        item_type: Box::new(field_type.clone()),
                                        optional: *optional,
                                        nonempty: *nonempty,
                                    }
                                } else {
                                    return Err(WdlError::no_such_member_error(
                                        pos.clone(),
                                        field.clone(),
                                    ));
                                }
                            } else {
                                return Err(WdlError::static_type_mismatch(
                                    pos.clone(),
                                    "Object, Pair, or Struct".to_string(),
                                    expr_type.to_string(),
                                    "Member access on array only allowed for scattered call outputs".to_string(),
                                ));
                            }
                        }
                        Type::Pair {
                            left_type,
                            right_type,
                            ..
                        } => {
                            // Pair field access: pair.left or pair.right
                            match field.as_str() {
                                "left" => left_type.as_ref().clone(),
                                "right" => right_type.as_ref().clone(),
                                _ => {
                                    return Err(WdlError::no_such_member_error(
                                        pos.clone(),
                                        field.clone(),
                                    ));
                                }
                            }
                        }
                        Type::Object { members, .. } => {
                            // Object field access: obj.field
                            if let Some(field_type) = members.get(field) {
                                field_type.clone()
                            } else {
                                return Err(WdlError::no_such_member_error(
                                    pos.clone(),
                                    field.clone(),
                                ));
                            }
                        }
                        Type::StructInstance { members, .. } => {
                            // Struct field access - now with proper struct resolution
                            if let Some(ref member_types) = members {
                                if let Some(field_type) = member_types.get(field) {
                                    field_type.clone()
                                } else {
                                    return Err(WdlError::no_such_member_error(
                                        pos.clone(),
                                        field.clone(),
                                    ));
                                }
                            } else {
                                // Try to resolve struct type using struct_typedefs
                                if let Type::StructInstance { type_name, .. } = expr_type {
                                    if let Some(struct_def) =
                                        struct_typedefs.iter().find(|s| s.name == *type_name)
                                    {
                                        if let Some(field_type) = struct_def.members.get(field) {
                                            return Ok(field_type.clone());
                                        }
                                    }
                                }
                                return Err(WdlError::static_type_mismatch(
                                    pos.clone(),
                                    "Struct with known members".to_string(),
                                    expr_type.to_string(),
                                    "Cannot access field on struct without member information"
                                        .to_string(),
                                ));
                            }
                        }
                        _ => {
                            return Err(WdlError::static_type_mismatch(
                                pos.clone(),
                                "Object, Pair, or Struct".to_string(),
                                expr_type.to_string(),
                                "Member access requires object, pair, or struct type".to_string(),
                            ));
                        }
                    }
                } else {
                    return Err(WdlError::static_type_mismatch(
                        pos.clone(),
                        "Object, Pair, or Struct".to_string(),
                        "Unknown".to_string(),
                        "Cannot infer type for member access".to_string(),
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
                    func.infer_type(&arg_types)?
                } else {
                    return Err(WdlError::RuntimeError {
                        message: format!("Unknown function: {}", function_name),
                    });
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
            Expression::At {
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

    fn infer_struct_type(
        type_name: &Option<String>,
        members: &mut [(String, Expression)],
        type_env: &Bindings<Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        // First, infer types for all member expressions
        let mut member_types = std::collections::HashMap::new();
        for (name, expr) in members {
            let member_type = expr.infer_type(type_env, stdlib, struct_typedefs)?;
            member_types.insert(name.clone(), member_type);
        }

        match type_name {
            // 1. Named struct literal: MyStruct { ... } - validate against specific struct definition
            Some(struct_name) => {
                if let Some(struct_def) = struct_typedefs.iter().find(|s| s.name == *struct_name) {
                    // Validate that all provided members match the struct definition
                    for (provided_name, provided_type) in &member_types {
                        if let Some(expected_type) = struct_def.members.get(provided_name) {
                            if !provided_type.coerces(expected_type, true) {
                                return Err(WdlError::Validation {
                                    pos: crate::error::SourcePosition::new(
                                        "<unknown>".to_string(),
                                        "<unknown>".to_string(),
                                        0,
                                        0,
                                        0,
                                        0,
                                    ),
                                    message: format!(
                                    "Member '{}' of struct '{}' has type '{}' but expected '{}'",
                                    provided_name, struct_name, provided_type, expected_type
                                ),
                                    source_text: None,
                                    declared_wdl_version: None,
                                });
                            }
                        } else {
                            return Err(WdlError::Validation {
                                pos: crate::error::SourcePosition::new(
                                    "<unknown>".to_string(),
                                    "<unknown>".to_string(),
                                    0,
                                    0,
                                    0,
                                    0,
                                ),
                                message: format!(
                                    "Member '{}' is not defined in struct '{}'",
                                    provided_name, struct_name
                                ),
                                source_text: None,
                                declared_wdl_version: None,
                            });
                        }
                    }

                    Ok(Type::StructInstance {
                        type_name: struct_def.name.clone(),
                        members: Some(struct_def.members.clone()),
                        optional: false,
                    })
                } else {
                    Err(WdlError::Validation {
                        pos: crate::error::SourcePosition::new(
                            "<unknown>".to_string(),
                            "<unknown>".to_string(),
                            0,
                            0,
                            0,
                            0,
                        ),
                        message: format!("Undefined struct type: {}", struct_name),
                        source_text: None,
                        declared_wdl_version: None,
                    })
                }
            }

            // 2. Anonymous object literal: object { ... } - create Object type directly
            None => {
                // Per WDL 1.2 spec: object { ... } syntax creates Object type
                // We don't try to match against struct definitions for anonymous object literals
                Ok(Type::object(member_types))
            }
        }
    }
}
