//! Expression evaluation logic

use super::{BinaryOperator, Expression, ExpressionBase, StringPart, UnaryOperator};
use crate::env::Bindings;
use crate::error::{HasSourcePosition, SourcePosition, WdlError};
use crate::types::Type;
use crate::value::{Value, ValueBase};
use std::collections::HashMap;

impl ExpressionBase for Expression {
    fn source_position(&self) -> &SourcePosition {
        HasSourcePosition::source_position(self)
    }

    fn infer_type(&mut self, type_env: &Bindings<Type>) -> Result<Type, WdlError> {
        // Delegate to the implementation in type_inference module
        Expression::infer_type(self, type_env)
    }

    fn get_type(&self) -> Option<&Type> {
        match self {
            Expression::Boolean { inferred_type, .. } => inferred_type.as_ref(),
            Expression::Int { inferred_type, .. } => inferred_type.as_ref(),
            Expression::Float { inferred_type, .. } => inferred_type.as_ref(),
            Expression::String { inferred_type, .. } => inferred_type.as_ref(),
            Expression::Null { inferred_type, .. } => inferred_type.as_ref(),
            Expression::Array { inferred_type, .. } => inferred_type.as_ref(),
            Expression::Pair { inferred_type, .. } => inferred_type.as_ref(),
            Expression::Map { inferred_type, .. } => inferred_type.as_ref(),
            Expression::Struct { inferred_type, .. } => inferred_type.as_ref(),
            Expression::Ident { inferred_type, .. } => inferred_type.as_ref(),
            Expression::Get { inferred_type, .. } => inferred_type.as_ref(),
            Expression::IfThenElse { inferred_type, .. } => inferred_type.as_ref(),
            Expression::Apply { inferred_type, .. } => inferred_type.as_ref(),
            Expression::BinaryOp { inferred_type, .. } => inferred_type.as_ref(),
            Expression::UnaryOp { inferred_type, .. } => inferred_type.as_ref(),
        }
    }

    fn typecheck(&self, expected: &Type) -> Result<(), WdlError> {
        if let Some(actual) = self.get_type() {
            actual.check_coercion(expected, true)?;
        }
        Ok(())
    }

    fn eval(
        &self,
        env: &Bindings<Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        match self {
            Expression::Boolean { value, .. } => Ok(Value::boolean(*value)),
            Expression::Int { value, .. } => Ok(Value::int(*value)),
            Expression::Float { value, .. } => Ok(Value::float(*value)),
            Expression::String { parts, .. } => {
                let mut result = String::new();
                for part in parts {
                    match part {
                        StringPart::Text(text) => result.push_str(text),
                        StringPart::Placeholder { expr, options } => {
                            let val = expr.eval(env, stdlib)?;
                            if val.is_null() {
                                if let Some(default) = options.get("default") {
                                    result.push_str(default);
                                }
                                // Otherwise add nothing for null values
                            } else {
                                // For string interpolation, extract the raw value without quotes
                                match &val {
                                    Value::String { value, .. }
                                    | Value::File { value, .. }
                                    | Value::Directory { value, .. } => {
                                        result.push_str(value);
                                    }
                                    _ => {
                                        result.push_str(&format!("{}", val));
                                    }
                                }
                            }
                        }
                    }
                }
                Ok(Value::string(result))
            }
            Expression::Null { .. } => Ok(Value::null()),

            Expression::Array { items, .. } => {
                let mut values = Vec::new();
                for item in items {
                    values.push(item.eval(env, stdlib)?);
                }
                let item_type = if let Some(first) = values.first() {
                    first.wdl_type().clone()
                } else {
                    Type::any()
                };
                Ok(Value::array(item_type, values))
            }

            Expression::Pair { left, right, .. } => {
                let left_val = left.eval(env, stdlib)?;
                let right_val = right.eval(env, stdlib)?;
                Ok(Value::pair(
                    left_val.wdl_type().clone(),
                    right_val.wdl_type().clone(),
                    left_val,
                    right_val,
                ))
            }

            Expression::Map { pairs, .. } => {
                let mut map_pairs = Vec::new();
                for (k_expr, v_expr) in pairs {
                    let key = k_expr.eval(env, stdlib)?;
                    let value = v_expr.eval(env, stdlib)?;
                    map_pairs.push((key, value));
                }

                let (key_type, value_type) = if let Some((k, v)) = map_pairs.first() {
                    (k.wdl_type().clone(), v.wdl_type().clone())
                } else {
                    (Type::any(), Type::any())
                };

                Ok(Value::map(key_type, value_type, map_pairs))
            }

            Expression::Struct { members, .. } => {
                let mut member_values = HashMap::new();
                for (name, expr) in members {
                    member_values.insert(name.clone(), expr.eval(env, stdlib)?);
                }

                let member_types: HashMap<String, Type> = member_values
                    .iter()
                    .map(|(k, v)| (k.clone(), v.wdl_type().clone()))
                    .collect();

                Ok(Value::struct_value(
                    Type::object(member_types),
                    member_values,
                    None,
                ))
            }

            Expression::Ident { name, .. } => {
                // First try direct resolution
                if let Some(value) = env.resolve(name) {
                    return Ok(value.clone());
                }

                // If not found and contains dot, try to resolve as member access
                if name.contains('.') {
                    let mut parts = name.splitn(2, '.');
                    if let (Some(prefix), Some(member)) = (parts.next(), parts.next()) {
                        // Try to resolve the prefix
                        if let Some(container_value) = env.resolve(prefix) {
                            match container_value {
                                Value::Struct { members, .. } => {
                                    if let Some(member_value) = members.get(member) {
                                        return Ok(member_value.clone());
                                    }
                                }
                                _ => {}
                            }
                        }
                    }
                }

                Err(WdlError::validation_error(
                    HasSourcePosition::source_position(self).clone(),
                    format!("Unknown identifier: {}", name),
                ))
            }

            Expression::Get { expr, index, .. } => {
                // Special case: If this is a member access like hello.message,
                // try to resolve it as a qualified name first
                if let Expression::Ident {
                    name: container_name,
                    ..
                } = expr.as_ref()
                {
                    // Try to extract member name from index
                    let member_name = match index.as_ref() {
                        Expression::Ident { name, .. } => Some(name.clone()),
                        Expression::String { parts, .. } => {
                            // Extract text from string parts
                            if parts.len() == 1 {
                                if let StringPart::Text(text) = &parts[0] {
                                    Some(text.clone())
                                } else {
                                    None
                                }
                            } else {
                                None
                            }
                        }
                        _ => None,
                    };

                    if let Some(member) = member_name {
                        let qualified_name = format!("{}.{}", container_name, member);
                        if let Some(value) = env.resolve(&qualified_name) {
                            return Ok(value.clone());
                        }
                    }
                }
                // Normal Get evaluation for arrays and maps
                let container = expr.eval(env, stdlib)?;
                let idx = index.eval(env, stdlib)?;

                match (&container, &idx) {
                    (Value::Array { values, .. }, Value::Int { value: i, .. }) => {
                        let index = *i as usize;
                        if index < values.len() {
                            Ok(values[index].clone())
                        } else {
                            Err(WdlError::OutOfBounds {
                                pos: HasSourcePosition::source_position(self).clone(),
                            })
                        }
                    }
                    (Value::Map { pairs, .. }, Value::String { value: key, .. }) => {
                        for (map_key, map_value) in pairs {
                            if let Value::String { value: key_str, .. } = map_key {
                                if key_str == key {
                                    return Ok(map_value.clone());
                                }
                            }
                        }
                        Err(WdlError::validation_error(
                            HasSourcePosition::source_position(self).clone(),
                            format!("Key '{}' not found in map", key),
                        ))
                    }
                    (Value::Struct { members, .. }, Value::String { value: member, .. }) => {
                        if let Some(value) = members.get(member) {
                            Ok(value.clone())
                        } else {
                            Err(WdlError::validation_error(
                                HasSourcePosition::source_position(self).clone(),
                                format!("Member '{}' not found in struct", member),
                            ))
                        }
                    }
                    _ => Err(WdlError::validation_error(
                        HasSourcePosition::source_position(self).clone(),
                        "Invalid array/map/struct access".to_string(),
                    )),
                }
            }

            Expression::IfThenElse {
                condition,
                true_expr,
                false_expr,
                ..
            } => {
                let cond_val = condition.eval(env, stdlib)?;
                if let Some(cond_bool) = cond_val.as_bool() {
                    if cond_bool {
                        true_expr.eval(env, stdlib)
                    } else {
                        false_expr.eval(env, stdlib)
                    }
                } else {
                    Err(WdlError::validation_error(
                        HasSourcePosition::source_position(&**condition).clone(),
                        "If condition must be Boolean".to_string(),
                    ))
                }
            }

            Expression::Apply {
                function_name,
                arguments,
                ..
            } => {
                // Evaluate arguments first
                let mut eval_args = Vec::new();
                for arg in arguments {
                    eval_args.push(arg.eval(env, stdlib)?);
                }

                // Look up function in stdlib
                if let Some(function) = stdlib.get_function(function_name) {
                    function.eval(&eval_args).map_err(|e| {
                        // Convert WdlError to include position information
                        match e {
                            WdlError::RuntimeError { message } => WdlError::validation_error(
                                HasSourcePosition::source_position(self).clone(),
                                message,
                            ),
                            WdlError::ArgumentCountMismatch {
                                function,
                                expected,
                                actual,
                            } => WdlError::validation_error(
                                HasSourcePosition::source_position(self).clone(),
                                format!(
                                    "{}() expects {} arguments, got {}",
                                    function, expected, actual
                                ),
                            ),
                            other => other,
                        }
                    })
                } else {
                    Err(WdlError::validation_error(
                        HasSourcePosition::source_position(self).clone(),
                        format!("Unknown function: {}", function_name),
                    ))
                }
            }

            Expression::BinaryOp {
                op, left, right, ..
            } => {
                let left_val = left.eval(env, stdlib)?;
                let right_val = right.eval(env, stdlib)?;

                // Convert operator to stdlib function name
                let function_name = match op {
                    BinaryOperator::Add => "_add",
                    BinaryOperator::Subtract => "_sub",
                    BinaryOperator::Multiply => "_mul",
                    BinaryOperator::Divide => "_div",
                    BinaryOperator::Modulo => "_rem",
                    BinaryOperator::Equal => "_eqeq",
                    BinaryOperator::NotEqual => "_neq",
                    BinaryOperator::Less => "_lt",
                    BinaryOperator::LessEqual => "_lte",
                    BinaryOperator::Greater => "_gt",
                    BinaryOperator::GreaterEqual => "_gte",
                    BinaryOperator::And => "_and",
                    BinaryOperator::Or => "_or",
                };

                // Call the stdlib operator function
                if let Some(function) = stdlib.get_function(function_name) {
                    function.eval(&[left_val, right_val]).map_err(|e| match e {
                        WdlError::RuntimeError { message } => WdlError::validation_error(
                            HasSourcePosition::source_position(self).clone(),
                            message,
                        ),
                        other => other,
                    })
                } else {
                    // This should never happen if stdlib is properly initialized
                    Err(WdlError::validation_error(
                        HasSourcePosition::source_position(self).clone(),
                        format!(
                            "Binary operator function '{}' not found in stdlib",
                            function_name
                        ),
                    ))
                }
            }

            Expression::UnaryOp { op, operand, .. } => {
                let operand_val = operand.eval(env, stdlib)?;
                match op {
                    UnaryOperator::Not => {
                        let bool_val = operand_val.as_bool().ok_or_else(|| {
                            WdlError::validation_error(
                                HasSourcePosition::source_position(&**operand).clone(),
                                "Operand must be Boolean".to_string(),
                            )
                        })?;
                        Ok(Value::boolean(!bool_val))
                    }
                    UnaryOperator::Negate => match operand_val {
                        Value::Int { value, .. } => Ok(Value::int(-value)),
                        Value::Float { value, .. } => Ok(Value::float(-value)),
                        _ => Err(WdlError::validation_error(
                            HasSourcePosition::source_position(&**operand).clone(),
                            "Operand must be numeric".to_string(),
                        )),
                    },
                }
            }
        }
    }

    fn children(&self) -> Vec<&Expression> {
        let mut children = Vec::new();

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
            Expression::IfThenElse {
                condition,
                true_expr,
                false_expr,
                ..
            } => {
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

    fn literal(&self) -> Option<Value> {
        match self {
            Expression::Boolean { value, .. } => Some(Value::boolean(*value)),
            Expression::Int { value, .. } => Some(Value::int(*value)),
            Expression::Float { value, .. } => Some(Value::float(*value)),
            Expression::String { parts, .. } => {
                // Only return literal value if all parts are text
                let mut result = String::new();
                for part in parts {
                    match part {
                        StringPart::Text(text) => result.push_str(text),
                        StringPart::Placeholder { .. } => return None,
                    }
                }
                Some(Value::string(result))
            }
            Expression::Null { .. } => Some(Value::null()),
            _ => None,
        }
    }
}
