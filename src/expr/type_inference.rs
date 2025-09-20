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
                    Self::infer_map_type(pairs.as_slice(), struct_typedefs)?
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
                let container_type = expr.get_type().ok_or_else(|| {
                    WdlError::static_type_mismatch(
                        pos.clone(),
                        "Array or Map".to_string(),
                        "Unknown".to_string(),
                        "Cannot infer type for subscript operation".to_string(),
                    )
                })?;

                Self::infer_subscript_type(container_type, index, pos)?
            }

            Expression::Get {
                expr, field, pos, ..
            } => {
                let container_type = expr.get_type().ok_or_else(|| {
                    WdlError::static_type_mismatch(
                        pos.clone(),
                        "Object, Pair, or Struct".to_string(),
                        "Unknown".to_string(),
                        "Cannot infer type for member access".to_string(),
                    )
                })?;

                Self::infer_get_type(container_type, field, pos, struct_typedefs)?
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
                let result_type = if branch_types.iter().any(|ty| {
                    matches!(
                        ty,
                        Type::Any {
                            optional: false,
                            ..
                        }
                    )
                }) {
                    Type::any()
                } else if branch_types
                    .iter()
                    .all(|ty| matches!(ty, Type::Any { optional: true, .. }))
                {
                    Type::any().with_optional(true)
                } else {
                    let unified_type = crate::types::unify_types(branch_types, true, false);

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
                };

                result_type
            }

            Expression::Apply {
                function_name,
                arguments,
                ..
            } => {
                // Use stdlib function infer_type
                if let Some(func) = stdlib.get_function(function_name) {
                    let mut arg_expressions: Vec<Expression> = arguments.clone();
                    func.infer_type(&mut arg_expressions, type_env, stdlib, struct_typedefs)?
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

        self.store_inferred_type(&inferred_type);
        Ok(inferred_type)
    }

    fn store_inferred_type(&mut self, ty: &Type) {
        let clone = ty.clone();
        match self {
            Expression::Boolean { inferred_type, .. }
            | Expression::Int { inferred_type, .. }
            | Expression::Float { inferred_type, .. }
            | Expression::String { inferred_type, .. }
            | Expression::Null { inferred_type, .. }
            | Expression::Array { inferred_type, .. }
            | Expression::Pair { inferred_type, .. }
            | Expression::Map { inferred_type, .. }
            | Expression::Struct { inferred_type, .. }
            | Expression::Ident { inferred_type, .. }
            | Expression::At { inferred_type, .. }
            | Expression::Get { inferred_type, .. }
            | Expression::IfThenElse { inferred_type, .. }
            | Expression::Apply { inferred_type, .. }
            | Expression::BinaryOp { inferred_type, .. }
            | Expression::UnaryOp { inferred_type, .. } => *inferred_type = Some(clone),
        }
    }

    fn infer_map_type(
        pairs: &[(Expression, Expression)],
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        let mut could_be_struct = true;
        let mut member_types: HashMap<String, Type> = HashMap::new();

        for (key_expr, value_expr) in pairs {
            match key_expr {
                Expression::String { parts, .. } if parts.len() == 1 => {
                    if let StringPart::Text(key_name) = &parts[0] {
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
                }
                _ => {
                    could_be_struct = false;
                    break;
                }
            }
        }

        if could_be_struct {
            for struct_def in struct_typedefs {
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

                if matches && member_types.len() <= struct_def.members.len() {
                    return Ok(Type::StructInstance {
                        type_name: struct_def.name.clone(),
                        members: Some(struct_def.members.clone()),
                        optional: false,
                    });
                }
            }
        }

        let key_types: Vec<&Type> = pairs.iter().filter_map(|(k, _)| k.get_type()).collect();
        let value_types: Vec<&Type> = pairs.iter().filter_map(|(_, v)| v.get_type()).collect();
        let unified_key_type = crate::types::unify_types(key_types, true, false);
        let unified_value_type = crate::types::unify_types(value_types, true, false);
        Ok(Type::map(unified_key_type, unified_value_type, false))
    }

    fn infer_subscript_type(
        container_type: &Type,
        index: &Expression,
        pos: &crate::error::SourcePosition,
    ) -> Result<Type, WdlError> {
        match container_type {
            Type::Array { item_type, .. } => {
                if let Some(index_type) = index.get_type() {
                    if !index_type.coerces(&Type::int(false), true) {
                        return Err(WdlError::static_type_mismatch(
                            HasSourcePosition::source_position(index).clone(),
                            "Int".to_string(),
                            index_type.to_string(),
                            "Array index must be Int".to_string(),
                        ));
                    }
                }
                Ok(item_type.as_ref().clone())
            }
            Type::Map { value_type, .. } => Ok(value_type.as_ref().clone()),
            other => Err(WdlError::static_type_mismatch(
                pos.clone(),
                "Array or Map".to_string(),
                other.to_string(),
                "Subscript operation requires array or map type".to_string(),
            )),
        }
    }

    fn infer_get_type(
        expr_type: &Type,
        field: &str,
        pos: &crate::error::SourcePosition,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        match expr_type {
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
                    if let Some(field_type) = members.get(field) {
                        Ok(Type::Array {
                            item_type: Box::new(field_type.clone()),
                            optional: *optional,
                            nonempty: *nonempty,
                        })
                    } else {
                        Err(WdlError::no_such_member_error(
                            pos.clone(),
                            field.to_string(),
                        ))
                    }
                } else {
                    Err(WdlError::static_type_mismatch(
                        pos.clone(),
                        "Object, Pair, or Struct".to_string(),
                        expr_type.to_string(),
                        "Member access on array only allowed for scattered call outputs"
                            .to_string(),
                    ))
                }
            }
            Type::Pair {
                left_type,
                right_type,
                ..
            } => match field {
                "left" => Ok(left_type.as_ref().clone()),
                "right" => Ok(right_type.as_ref().clone()),
                _ => Err(WdlError::no_such_member_error(
                    pos.clone(),
                    field.to_string(),
                )),
            },
            Type::Object { members, .. } => members
                .get(field)
                .cloned()
                .ok_or_else(|| WdlError::no_such_member_error(pos.clone(), field.to_string())),
            Type::StructInstance {
                type_name, members, ..
            } => Self::resolve_struct_member_type(
                type_name,
                members.as_ref(),
                field,
                pos,
                struct_typedefs,
            ),
            _ => Err(WdlError::static_type_mismatch(
                pos.clone(),
                "Object, Pair, or Struct".to_string(),
                expr_type.to_string(),
                "Member access requires object, pair, or struct type".to_string(),
            )),
        }
    }

    fn resolve_struct_member_type(
        type_name: &str,
        members: Option<&HashMap<String, Type>>,
        field: &str,
        pos: &crate::error::SourcePosition,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        if let Some(member_types) = members {
            return member_types
                .get(field)
                .cloned()
                .ok_or_else(|| WdlError::no_such_member_error(pos.clone(), field.to_string()));
        }

        let struct_def = struct_typedefs
            .iter()
            .find(|s| s.name == type_name)
            .ok_or_else(|| {
                WdlError::static_type_mismatch(
                    pos.clone(),
                    "Struct with known members".to_string(),
                    type_name.to_string(),
                    "Cannot access field on struct without member information".to_string(),
                )
            })?;

        struct_def
            .members
            .get(field)
            .cloned()
            .ok_or_else(|| WdlError::no_such_member_error(pos.clone(), field.to_string()))
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
