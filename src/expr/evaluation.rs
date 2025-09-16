//! Expression evaluation logic

use super::{BinaryOperator, Expression, ExpressionBase, StringPart, StringType, UnaryOperator};
use crate::env::Bindings;
use crate::error::{HasSourcePosition, SourcePosition, WdlError};
use crate::types::Type;
use crate::value::{Value, ValueBase};
use std::collections::HashMap;
use std::result;

impl ExpressionBase for Expression {
    fn source_position(&self) -> &SourcePosition {
        HasSourcePosition::source_position(self)
    }

    fn infer_type(
        &mut self,
        type_env: &Bindings<Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        // Delegate to the implementation in type_inference module
        Expression::infer_type(self, type_env, stdlib, struct_typedefs)
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
            Expression::At { inferred_type, .. } => inferred_type.as_ref(),
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

            Expression::String {
                parts, string_type, ..
            } => {
                let result = match string_type {
                    StringType::MultiLine => {
                        // Process multiline strings with special handling
                        eval_multiline_string(parts, env, stdlib)?
                    }
                    StringType::TaskCommand => {
                        // Process task commands with dedent but no escape removal
                        eval_task_command(parts, env, stdlib)?
                    }
                    StringType::Regular => {
                        // Regular string processing
                        eval_regular_string(parts, env, stdlib)?
                    }
                };
                Ok(result)
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

            Expression::Map {
                pairs,
                inferred_type,
                ..
            } => {
                let mut map_pairs = Vec::new();
                for (k_expr, v_expr) in pairs {
                    let key = k_expr.eval(env, stdlib)?;
                    let value = v_expr.eval(env, stdlib)?;
                    map_pairs.push((key, value));
                }

                // Check if this Map should be evaluated as a struct based on inferred type
                if let Some(Type::StructInstance {
                    type_name, members, ..
                }) = inferred_type
                {
                    if let Some(struct_members) = members {
                        // This Map should be converted to a struct
                        let mut member_values = HashMap::new();

                        // Convert map pairs to member values
                        for (key_val, value_val) in map_pairs {
                            if let Value::String {
                                value: key_string, ..
                            } = key_val
                            {
                                member_values.insert(key_string, value_val);
                            }
                        }

                        // Create struct value with the inferred type
                        let struct_type = Type::StructInstance {
                            type_name: type_name.clone(),
                            members: Some(struct_members.clone()),
                            optional: false,
                        };

                        return Ok(Value::struct_value_with_completion(
                            struct_type,
                            member_values,
                            None,
                        ));
                    }
                }

                // Regular Map evaluation
                let (key_type, value_type) = if let Some((k, v)) = map_pairs.first() {
                    (k.wdl_type().clone(), v.wdl_type().clone())
                } else {
                    (Type::any(), Type::any())
                };

                Ok(Value::map(key_type, value_type, map_pairs))
            }

            Expression::Struct {
                members,
                inferred_type,
                ..
            } => {
                let mut member_values = HashMap::new();

                // First, add all explicitly provided members
                for (name, expr) in members {
                    member_values.insert(name.clone(), expr.eval(env, stdlib)?);
                }

                // Create the struct type based on the members we have
                let member_types: HashMap<String, Type> = member_values
                    .iter()
                    .map(|(k, v)| (k.clone(), v.wdl_type().clone()))
                    .collect();

                let struct_type = if let Some(inferred) = inferred_type {
                    // Use the inferred type if available, which may have more complete information
                    inferred.clone()
                } else {
                    // Fallback to creating type from available members
                    Type::object(member_types)
                };

                // Use the new function that completes missing optional members
                let result = Value::struct_value_with_completion(struct_type, member_values, None);

                Ok(result)
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
                            if let Value::Struct { members, .. } = container_value {
                                if let Some(member_value) = members.get(member) {
                                    return Ok(member_value.clone());
                                }
                            }
                        }
                    }
                }

                Err(WdlError::unknown_identifier_error(
                    HasSourcePosition::source_position(self).clone(),
                    name.clone(),
                ))
            }

            Expression::At { expr, index, .. } => {
                // Array/Map subscript access
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
                    (Value::Map { pairs, wdl_type }, _) => {
                        // Maps can have any type as key, not just String
                        // For proper map access, we need to try type coercion for the key

                        // First, try direct comparison (fast path)
                        for (map_key, map_value) in pairs {
                            if map_key == &idx {
                                return Ok(map_value.clone());
                            }
                        }

                        // If direct comparison failed, try coercing the access key to match map key types
                        if let Type::Map { key_type, .. } = wdl_type {
                            // Try to coerce the access key to the map's key type
                            if let Ok(coerced_key) = idx.coerce(key_type) {
                                for (map_key, map_value) in pairs {
                                    if map_key == &coerced_key {
                                        return Ok(map_value.clone());
                                    }
                                }
                            }
                        }

                        Err(WdlError::validation_error(
                            HasSourcePosition::source_position(self).clone(),
                            "Key not found in map".to_string(),
                        ))
                    }
                    _ => Err(WdlError::validation_error(
                        HasSourcePosition::source_position(self).clone(),
                        format!(
                            "Invalid subscript operation on {:?} with index {:?}",
                            container, idx
                        ),
                    )),
                }
            }

            Expression::Get { expr, field, .. } => {
                // Special case: If this is a member access like hello.message,
                // try to resolve it as a qualified name first
                if let Expression::Ident {
                    name: container_name,
                    ..
                } = expr.as_ref()
                {
                    let qualified_name = format!("{}.{}", container_name, field);
                    if let Some(value) = env.resolve(&qualified_name) {
                        return Ok(value.clone());
                    }
                }
                // Normal Get evaluation for member access
                let container = expr.eval(env, stdlib)?;

                match &container {
                    Value::Struct { members, .. } => {
                        if let Some(value) = members.get(field) {
                            Ok(value.clone())
                        } else {
                            Err(WdlError::validation_error(
                                HasSourcePosition::source_position(self).clone(),
                                format!("Member '{}' not found in struct", field),
                            ))
                        }
                    }
                    Value::Pair { left, right, .. } => match field.as_str() {
                        "left" => Ok(left.as_ref().clone()),
                        "right" => Ok(right.as_ref().clone()),
                        _ => Err(WdlError::validation_error(
                            HasSourcePosition::source_position(self).clone(),
                            format!(
                                "Pair has no member '{}'. Valid members are 'left' and 'right'",
                                field
                            ),
                        )),
                    },
                    _ => Err(WdlError::validation_error(
                        HasSourcePosition::source_position(self).clone(),
                        format!(
                            "Invalid member access on {:?} with field {}",
                            container, field
                        ),
                    )),
                }
            }

            Expression::IfThenElse {
                condition,
                true_expr,
                false_expr,
                inferred_type,
                ..
            } => {
                let cond_val = condition.eval(env, stdlib)?;
                if let Some(cond_bool) = cond_val.as_bool() {
                    let result_value = if cond_bool {
                        true_expr.eval(env, stdlib)?
                    } else {
                        false_expr.eval(env, stdlib)?
                    };

                    // Apply type coercion to the inferred type if available
                    if let Some(target_type) = inferred_type {
                        result_value.coerce(target_type)
                    } else {
                        Ok(result_value)
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
                inferred_type,
                ..
            } => {
                // Evaluate arguments first
                let mut eval_args = Vec::new();
                for arg in arguments {
                    eval_args.push(arg.eval(env, stdlib)?);
                }

                // Look up function in stdlib
                if let Some(function) = stdlib.get_function(function_name) {
                    let result_value =
                        function.eval_with_stdlib(&eval_args, stdlib).map_err(|e| {
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
                        })?;

                    // Apply type coercion to the inferred type if available
                    if let Some(target_type) = inferred_type {
                        result_value.coerce(target_type)
                    } else {
                        Ok(result_value)
                    }
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
                    function
                        .eval_with_stdlib(&[left_val, right_val], stdlib)
                        .map_err(|e| match e {
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

                let function_name = match op {
                    UnaryOperator::Not => "_not",
                    UnaryOperator::Negate => "_neg",
                };

                if let Some(function) = stdlib.get_function(function_name) {
                    function
                        .eval_with_stdlib(&[operand_val], stdlib)
                        .map_err(|e| match e {
                            WdlError::RuntimeError { message } => WdlError::validation_error(
                                HasSourcePosition::source_position(self).clone(),
                                message,
                            ),
                            other => other,
                        })
                } else {
                    Err(WdlError::validation_error(
                        HasSourcePosition::source_position(self).clone(),
                        format!(
                            "Unary operator function '{}' not found in stdlib",
                            function_name
                        ),
                    ))
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
            Expression::At { expr, index, .. } => {
                children.push(expr.as_ref());
                children.push(index.as_ref());
            }
            Expression::Get { expr, .. } => {
                children.push(expr.as_ref());
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
            Expression::String {
                parts, string_type, ..
            } => {
                // Only return literal value if all parts are text
                if parts
                    .iter()
                    .any(|p| matches!(p, StringPart::Placeholder { .. }))
                {
                    return None;
                }

                // Apply appropriate processing based on string type
                let result = match string_type {
                    StringType::MultiLine => {
                        // For literals (no placeholders), we can simplify to text processing
                        // First remove escaped newlines
                        let mut processed_parts = Vec::new();
                        for part in parts {
                            if let StringPart::Text(text) = part {
                                let processed = remove_escaped_newlines(text);
                                processed_parts.push(StringPart::Text(processed));
                            }
                        }

                        // Apply trimming and dedenting
                        let trimmed_dedented = process_multiline_parts(&processed_parts);

                        // Join text parts and decode escape sequences
                        let mut result = String::new();
                        for part in trimmed_dedented {
                            if let StringPart::Text(text) = part {
                                let decoded = decode_escape_sequences(&text);
                                result.push_str(&decoded);
                            }
                        }
                        result
                    }
                    StringType::TaskCommand => {
                        // Apply dedent to task command parts (they're all text at this point)
                        let dedented_parts = dedent_parts(parts);
                        // Join all text parts back together
                        let mut result = String::new();
                        for part in dedented_parts {
                            if let StringPart::Text(text) = part {
                                result.push_str(&text);
                            }
                        }
                        result
                    }
                    StringType::Regular => {
                        // Just join the text parts
                        let text_parts: Vec<String> = parts
                            .iter()
                            .filter_map(|p| match p {
                                StringPart::Text(text) => Some(text.clone()),
                                _ => None,
                            })
                            .collect();
                        text_parts.join("")
                    }
                };

                Some(Value::string(result))
            }
            Expression::Null { .. } => Some(Value::null()),
            _ => None,
        }
    }
}

/// Helper function to evaluate a placeholder and return its string representation
fn evaluate_placeholder(
    expr: &Expression,
    options: &std::collections::HashMap<String, String>,
    env: &Bindings<Value>,
    stdlib: &crate::stdlib::StdLib,
) -> Result<String, WdlError> {
    // Per WDL spec: If an expression within a placeholder evaluates to None,
    // or causes an error, then the placeholder is replaced by the empty string.
    let val = match expr.eval(env, stdlib) {
        Ok(value) => value,
        Err(_) => {
            // Return empty string for any evaluation error
            return Ok(String::new());
        }
    };

    if val.is_null() {
        if let Some(default) = options.get("default") {
            return Ok(default.clone());
        }
        // Otherwise return empty string for null values
        return Ok(String::new());
    }

    // Handle true/false options for Boolean values first
    if let Value::Boolean {
        value: bool_val, ..
    } = &val
    {
        if *bool_val {
            if let Some(true_option) = options.get("true") {
                return Ok(true_option.clone());
            }
        } else if let Some(false_option) = options.get("false") {
            return Ok(false_option.clone());
        }
    }

    // Handle sep option for arrays
    if let Some(sep) = options.get("sep") {
        match &val {
            Value::Array { values, .. } => {
                let string_values: Vec<String> = values
                    .iter()
                    .map(|v| match v {
                        Value::String { value, .. }
                        | Value::File { value, .. }
                        | Value::Directory { value, .. } => value.clone(),
                        _ => format!("{}", v),
                    })
                    .collect();
                return Ok(string_values.join(sep));
            }
            _ => {
                // For non-arrays, just convert to string (sep has no effect)
                return Ok(match &val {
                    Value::String { value, .. }
                    | Value::File { value, .. }
                    | Value::Directory { value, .. } => value.clone(),
                    _ => format!("{}", val),
                });
            }
        }
    }

    // No special options - use default string conversion
    Ok(match &val {
        Value::String { value, .. }
        | Value::File { value, .. }
        | Value::Directory { value, .. } => value.clone(),
        _ => format!("{}", val),
    })
}

/// Helper function to evaluate regular strings
fn eval_regular_string(
    parts: &[StringPart],
    env: &Bindings<Value>,
    stdlib: &crate::stdlib::StdLib,
) -> Result<Value, WdlError> {
    let mut result = String::new();
    for part in parts {
        match part {
            StringPart::Text(text) => result.push_str(text),
            StringPart::Placeholder { expr, options } => {
                let placeholder_text = evaluate_placeholder(expr, options, env, stdlib)?;
                result.push_str(&placeholder_text);
            }
        }
    }
    Ok(Value::string(result))
}

fn eval_multiline_string(
    parts: &[StringPart],
    env: &Bindings<Value>,
    stdlib: &crate::stdlib::StdLib,
) -> Result<Value, WdlError> {
    // Step 1: Process parts to handle escaped newlines (keep placeholders intact)
    let mut processed_parts = Vec::new();
    for part in parts {
        match part {
            StringPart::Text(text) => {
                // Remove escaped newlines and following whitespace
                let processed = remove_escaped_newlines(text);
                processed_parts.push(StringPart::Text(processed));
            }
            StringPart::Placeholder { .. } => {
                // Keep placeholders as-is during this phase
                processed_parts.push(part.clone());
            }
        }
    }

    // Step 2: Apply trimming and dedenting with placeholders still as placeholders
    let trimmed_dedented = process_multiline_parts(&processed_parts);

    // Step 3: Now evaluate placeholders and decode escape sequences
    let mut result = String::new();
    for part in trimmed_dedented {
        match part {
            StringPart::Text(text) => {
                // Decode escape sequences for text parts
                let decoded = decode_escape_sequences(&text);
                result.push_str(&decoded);
            }
            StringPart::Placeholder { expr, options } => {
                // Evaluate placeholder
                let placeholder_text = evaluate_placeholder(&expr, &options, env, stdlib)?;
                result.push_str(&placeholder_text);
            }
        }
    }

    Ok(Value::string(result))
}

/// Helper function to evaluate task command strings
fn eval_task_command(
    parts: &[StringPart],
    env: &Bindings<Value>,
    stdlib: &crate::stdlib::StdLib,
) -> Result<Value, WdlError> {
    // First evaluate placeholders but keep structure as StringPart
    let mut evaluated_parts = Vec::new();
    for part in parts {
        match part {
            StringPart::Text(text) => {
                // Task commands don't decode escape sequences
                // They are passed as-is to the shell
                evaluated_parts.push(StringPart::Text(text.clone()));
            }
            StringPart::Placeholder { expr, options } => {
                let placeholder_text = evaluate_placeholder(expr, options, env, stdlib)?;
                evaluated_parts.push(StringPart::Text(placeholder_text));
            }
        }
    }

    // Apply dedenting (without trimming)
    let dedented_parts = dedent_parts(&evaluated_parts);

    // Join all text parts back together
    let mut result = String::new();
    for part in dedented_parts {
        if let StringPart::Text(text) = part {
            result.push_str(&text);
        }
    }

    Ok(Value::string(result))
}

/// Process multiline parts: trim whitespace and apply dedent while keeping placeholders
fn process_multiline_parts(parts: &[StringPart]) -> Vec<StringPart> {
    // If there are no parts, return empty
    if parts.is_empty() {
        return Vec::new();
    }

    let mut processed_parts = parts.to_vec();

    // Trim whitespace from the left of the first line
    if let Some(StringPart::Text(first_text)) = processed_parts.first_mut() {
        // Find first newline
        if let Some(newline_pos) = first_text.find('\n') {
            let first_line = &first_text[..newline_pos];
            if first_line.trim().is_empty() {
                // Remove the entire first line if it's only whitespace
                *first_text = first_text[newline_pos + 1..].to_string();
            } else {
                // Just trim leading whitespace from first line
                let trimmed = first_line.trim_start();
                *first_text = format!("{}{}", trimmed, &first_text[newline_pos..]);
            }
        } else {
            // Single line - just trim leading whitespace
            *first_text = first_text.trim_start().to_string();
        }
    }

    // Trim whitespace from the right of the last line
    if let Some(StringPart::Text(last_text)) = processed_parts.last_mut() {
        if let Some(last_newline_pos) = last_text.rfind('\n') {
            let last_line = &last_text[last_newline_pos + 1..];
            if last_line.trim().is_empty() {
                // Remove the entire last line if it's only whitespace
                *last_text = last_text[..last_newline_pos].to_string();
            } else {
                // Just trim trailing whitespace from last line
                let trimmed = last_line.trim_end();
                *last_text = format!("{}{}", &last_text[..last_newline_pos + 1], trimmed);
            }
        } else {
            // Single line - just trim trailing whitespace
            *last_text = last_text.trim_end().to_string();
        }
    }

    // Now apply dedent with placeholders still as placeholders
    dedent_parts(&processed_parts)
}

/// Remove escaped newlines and following whitespace
fn remove_escaped_newlines(text: &str) -> String {
    let mut result = String::new();
    let mut chars = text.chars().peekable();
    let mut backslash_count = 0;

    while let Some(ch) = chars.next() {
        if ch == '\\' {
            backslash_count += 1;
            result.push(ch);
        } else if ch == '\n' {
            if backslash_count % 2 == 1 {
                // Odd number of backslashes - newline is escaped
                // Remove the last backslash
                result.pop();
                // Skip any following whitespace
                while let Some(&next_ch) = chars.peek() {
                    if next_ch == ' ' || next_ch == '\t' {
                        chars.next();
                    } else {
                        break;
                    }
                }
            } else {
                // Even number of backslashes - newline is not escaped
                result.push(ch);
            }
            backslash_count = 0;
        } else {
            result.push(ch);
            backslash_count = 0;
        }
    }

    result
}

/// Decode escape sequences in a string (for multiline strings)
fn decode_escape_sequences(text: &str) -> String {
    let mut result = String::new();
    let mut chars = text.chars();

    while let Some(ch) = chars.next() {
        if ch == '\\' {
            if let Some(next_ch) = chars.next() {
                match next_ch {
                    'n' => result.push('\n'),
                    't' => result.push('\t'),
                    'r' => result.push('\r'),
                    '\\' => result.push('\\'),
                    '\'' => result.push('\''),
                    '"' => result.push('"'),
                    _ => {
                        // Unknown escape sequence - keep as is
                        result.push('\\');
                        result.push(next_ch);
                    }
                }
            } else {
                // Backslash at end of string
                result.push('\\');
            }
        } else {
            result.push(ch);
        }
    }

    result
}

#[cfg(test)]
fn dedent(text: &str) -> String {
    // Convert string to StringPart array and back
    let parts = vec![StringPart::Text(text.to_string())];
    let dedented_parts = dedent_parts(&parts);

    // Join all text parts back together
    let mut result = String::new();
    for part in dedented_parts {
        if let StringPart::Text(text) = part {
            result.push_str(&text);
        }
    }
    result
}

fn dedent_parts(parts: &[StringPart]) -> Vec<StringPart> {
    // Detect common leading whitespace on the non-blank lines
    // Build a pseudo string where placeholders are replaced with "~{}"
    let pseudo = parts
        .iter()
        .map(|part| match part {
            StringPart::Text(text) => text.clone(),
            StringPart::Placeholder { .. } => "~{}".to_string(),
        })
        .collect::<Vec<_>>()
        .join("");

    let mut common_ws: Option<usize> = None;
    for line in pseudo.split('\n') {
        let line_ws = line.len() - line.trim_start().len();
        if line_ws < line.len() {
            // This line has non-whitespace content
            common_ws = Some(match common_ws {
                None => line_ws,
                Some(current) => current.min(line_ws),
            });
        }
    }

    // Remove the common leading whitespace, passing through placeholders
    let common_ws = common_ws.unwrap_or(0);
    if common_ws == 0 {
        return parts.to_vec();
    }

    let mut parts2 = Vec::new();
    let mut at_new_line = true;

    for part in parts {
        match part {
            StringPart::Placeholder { .. } => {
                // Placeholder - pass through unchanged
                at_new_line = false;
                parts2.push(part.clone());
            }
            StringPart::Text(text) => {
                let lines: Vec<&str> = text.split('\n').collect();
                let mut lines2 = Vec::new();

                for (i, line) in lines.iter().enumerate() {
                    if at_new_line {
                        // Remove common whitespace from the beginning of the line
                        // For lines shorter than common_ws (blank lines with some whitespace),
                        // we remove what whitespace is there
                        let chars_to_remove = common_ws.min(line.len());
                        lines2.push(&line[chars_to_remove..]);
                    } else {
                        // Not at new line (after placeholder) - keep line as is
                        lines2.push(line);
                    }

                    // We're at a new line for the next iteration unless this is the last line
                    // and the text doesn't end with a newline
                    at_new_line = i < lines.len() - 1 || text.ends_with('\n');
                }

                let dedented_text = lines2.join("\n");
                parts2.push(StringPart::Text(dedented_text));

                // Update at_new_line based on whether this text part ends with newline
                at_new_line = text.ends_with('\n');
            }
        }
    }

    parts2
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::error::SourcePosition;

    fn make_placeholder(expr_text: &str) -> StringPart {
        StringPart::Placeholder {
            expr: Box::new(Expression::String {
                parts: vec![StringPart::Text(expr_text.to_string())],
                string_type: StringType::Regular,
                inferred_type: None,
                pos: SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1),
            }),
            options: HashMap::new(),
        }
    }

    #[test]
    fn test_dedent_parts_no_indentation() {
        let parts = vec![StringPart::Text("line1\nline2\nline3".to_string())];

        let result = dedent_parts(&parts);
        assert_eq!(result.len(), 1);

        if let StringPart::Text(text) = &result[0] {
            assert_eq!(text, "line1\nline2\nline3");
        } else {
            panic!("Expected Text variant");
        }
    }

    #[test]
    fn test_dedent_parts_with_common_indentation() {
        let parts = vec![StringPart::Text(
            "    line1\n    line2\n    line3".to_string(),
        )];

        let result = dedent_parts(&parts);
        assert_eq!(result.len(), 1);

        if let StringPart::Text(text) = &result[0] {
            assert_eq!(text, "line1\nline2\nline3");
        } else {
            panic!("Expected Text variant");
        }
    }

    #[test]
    fn test_dedent_parts_with_varying_indentation() {
        let parts = vec![StringPart::Text(
            "    line1\n  line2\n      line3".to_string(),
        )];

        let result = dedent_parts(&parts);
        assert_eq!(result.len(), 1);

        if let StringPart::Text(text) = &result[0] {
            assert_eq!(text, "  line1\nline2\n    line3");
        } else {
            panic!("Expected Text variant");
        }
    }

    #[test]
    fn test_dedent_parts_with_placeholder() {
        let parts = vec![
            StringPart::Text("    echo ".to_string()),
            make_placeholder("value"),
            StringPart::Text("\n    line2".to_string()),
        ];

        let result = dedent_parts(&parts);
        assert_eq!(result.len(), 3);

        // First part should have indentation removed
        if let StringPart::Text(text) = &result[0] {
            assert_eq!(text, "echo ");
        } else {
            panic!("Expected Text variant at index 0");
        }

        // Placeholder should be unchanged
        assert!(matches!(&result[1], StringPart::Placeholder { .. }));

        // Third part should have indentation removed from the new line
        if let StringPart::Text(text) = &result[2] {
            assert_eq!(text, "\nline2");
        } else {
            panic!("Expected Text variant at index 2");
        }
    }

    #[test]
    fn test_dedent_parts_placeholder_prevents_dedent_on_same_line() {
        // When a placeholder is on a line, text after it on the same line
        // should not be dedented
        let parts = vec![
            StringPart::Text("    line1\n    ".to_string()),
            make_placeholder("value"),
            StringPart::Text("  text_after_placeholder\n    line3".to_string()),
        ];

        let result = dedent_parts(&parts);

        if let StringPart::Text(text) = &result[0] {
            assert_eq!(text, "line1\n");
        } else {
            panic!("Expected Text variant at index 0");
        }

        // Placeholder unchanged
        assert!(matches!(&result[1], StringPart::Placeholder { .. }));

        // Text after placeholder on same line keeps its spacing
        if let StringPart::Text(text) = &result[2] {
            assert_eq!(text, "  text_after_placeholder\nline3");
        } else {
            panic!("Expected Text variant at index 2");
        }
    }

    #[test]
    fn test_dedent_parts_blank_lines() {
        let parts = vec![StringPart::Text("    line1\n\n    line3".to_string())];

        let result = dedent_parts(&parts);
        assert_eq!(result.len(), 1);

        if let StringPart::Text(text) = &result[0] {
            assert_eq!(text, "line1\n\nline3");
        } else {
            panic!("Expected Text variant");
        }
    }

    #[test]
    fn test_dedent_parts_blank_lines_with_whitespace() {
        // Blank lines that contain only whitespace should have
        // common whitespace removed, leaving just newline
        let parts = vec![StringPart::Text(
            "    line1\n        \n    line2".to_string(),
        )];

        let result = dedent_parts(&parts);
        assert_eq!(result.len(), 1);

        if let StringPart::Text(text) = &result[0] {
            // The middle line had 8 spaces, common indent is 4
            // So it should become 4 spaces after dedenting, not empty
            assert_eq!(
                text, "line1\n    \nline2",
                "Blank line with 8 spaces should have 4 spaces after removing common indent of 4"
            );
        } else {
            panic!("Expected Text variant");
        }
    }

    #[test]
    fn test_dedent_parts_blank_lines_with_less_whitespace_than_common() {
        // Blank lines with less whitespace than common should become empty
        let parts = vec![StringPart::Text("    line1\n  \n    line2".to_string())];

        let result = dedent_parts(&parts);
        assert_eq!(result.len(), 1);

        if let StringPart::Text(text) = &result[0] {
            // The middle line has 2 spaces, common indent is 4
            // According to spec, it should be trimmed to just newline
            assert_eq!(
                text, "line1\n\nline2",
                "Blank line with 2 spaces should become empty when common indent is 4"
            );
        } else {
            panic!("Expected Text variant");
        }
    }

    #[test]
    fn test_dedent_parts_multiple_placeholders() {
        let parts = vec![
            StringPart::Text("    echo ".to_string()),
            make_placeholder("val1"),
            StringPart::Text(" ".to_string()),
            make_placeholder("val2"),
            StringPart::Text("\n    line2".to_string()),
        ];

        let result = dedent_parts(&parts);
        assert_eq!(result.len(), 5);

        if let StringPart::Text(text) = &result[0] {
            assert_eq!(text, "echo ");
        } else {
            panic!("Expected Text variant at index 0");
        }

        // Both placeholders should be unchanged
        assert!(matches!(&result[1], StringPart::Placeholder { .. }));
        assert!(matches!(&result[3], StringPart::Placeholder { .. }));

        // Last part with new line should be dedented
        if let StringPart::Text(text) = &result[4] {
            assert_eq!(text, "\nline2");
        } else {
            panic!("Expected Text variant at index 4");
        }
    }

    #[test]
    fn test_dedent_helper_function() {
        // Test the simple dedent helper that converts string to StringPart and back
        let text = "    line1\n    line2\n    line3";
        let result = dedent(text);
        assert_eq!(result, "line1\nline2\nline3");
    }

    #[test]
    fn test_dedent_parts_preserves_trailing_newline() {
        let parts = vec![StringPart::Text("    line1\n    line2\n".to_string())];

        let result = dedent_parts(&parts);
        assert_eq!(result.len(), 1);

        if let StringPart::Text(text) = &result[0] {
            assert_eq!(text, "line1\nline2\n");
        } else {
            panic!("Expected Text variant");
        }
    }

    #[test]
    fn test_multiline_string_processing_order() {
        // Test that multiline strings process in correct order:
        // 1. Remove escaped newlines
        // 2. Trim first/last lines
        // 3. Dedent (with placeholders as placeholders)
        // 4. Evaluate placeholders

        // This test simulates what eval_multiline_string should do
        use crate::env::Bindings;
        use crate::stdlib::StdLib;
        use crate::value::Value;

        // Create test environment with a variable
        let env =
            Bindings::new().bind("name".to_string(), Value::string("World".to_string()), None);
        let _stdlib = StdLib::new("1.2");

        // Test case: multiline string with placeholder that affects dedenting
        let parts = vec![
            StringPart::Text("    Hello ".to_string()),
            StringPart::Placeholder {
                expr: Box::new(Expression::Ident {
                    name: "name".to_string(),
                    inferred_type: None,
                    pos: SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1),
                }),
                options: HashMap::new(),
            },
            StringPart::Text("\n    Line 2".to_string()),
        ];

        // Step 1: Process escaped newlines (skipped in this test - no escaped newlines)

        // Step 2: Trim first/last lines would happen here in full implementation

        // Step 3: Dedent with placeholders still as placeholders
        let dedented = dedent_parts(&parts);

        // Verify dedenting happened with placeholder in place
        assert_eq!(dedented.len(), 3);
        if let StringPart::Text(text) = &dedented[0] {
            assert_eq!(text, "Hello ");
        }
        // Placeholder should be unchanged
        assert!(matches!(&dedented[1], StringPart::Placeholder { .. }));
        if let StringPart::Text(text) = &dedented[2] {
            assert_eq!(text, "\nLine 2");
        }

        // Step 4: Now evaluate placeholders
        let mut final_result = String::new();
        for part in dedented {
            match part {
                StringPart::Text(text) => final_result.push_str(&text),
                StringPart::Placeholder { expr, options } => {
                    // In real implementation, would call evaluate_placeholder
                    // For this test, we know it should evaluate to "World"
                    final_result.push_str("World");
                }
            }
        }

        assert_eq!(final_result, "Hello World\nLine 2");
    }
}
