//! Expression evaluation logic

use super::{BinaryOperator, Expression, ExpressionBase, StringPart, StringType, UnaryOperator};
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
                    (Value::Map { pairs, .. }, _) => {
                        // Maps can have any type as key, not just String
                        for (map_key, map_value) in pairs {
                            // Compare values directly - this handles all value types
                            if map_key == &idx {
                                return Ok(map_value.clone());
                            }
                        }
                        Err(WdlError::validation_error(
                            HasSourcePosition::source_position(self).clone(),
                            "Key not found in map".to_string(),
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
                    (Value::Pair { left, right, .. }, Value::String { value: member, .. }) => {
                        match member.as_str() {
                            "left" => Ok(left.as_ref().clone()),
                            "right" => Ok(right.as_ref().clone()),
                            _ => Err(WdlError::validation_error(
                                HasSourcePosition::source_position(self).clone(),
                                format!(
                                    "Pair has no member '{}'. Valid members are 'left' and 'right'",
                                    member
                                ),
                            )),
                        }
                    }
                    _ => Err(WdlError::validation_error(
                        HasSourcePosition::source_position(self).clone(),
                        "Invalid array/map/struct/pair access".to_string(),
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
                        eprintln!("DEBUG IfThenElse: target_type = {:?}", target_type);
                        eprintln!(
                            "DEBUG IfThenElse: result_value before coerce = {:?}",
                            result_value
                        );
                        let coerced = result_value.coerce(target_type)?;
                        eprintln!(
                            "DEBUG IfThenElse: result_value after coerce = {:?}",
                            coerced
                        );
                        Ok(coerced)
                    } else {
                        eprintln!("DEBUG IfThenElse: no inferred_type available");
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
                ..
            } => {
                // Evaluate arguments first
                let mut eval_args = Vec::new();
                for arg in arguments {
                    eval_args.push(arg.eval(env, stdlib)?);
                }

                // Look up function in stdlib
                if let Some(function) = stdlib.get_function(function_name) {
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

                // Collect all text parts
                let text_parts: Vec<String> = parts
                    .iter()
                    .filter_map(|p| match p {
                        StringPart::Text(text) => Some(text.clone()),
                        _ => None,
                    })
                    .collect();

                // Apply appropriate processing based on string type
                let result = match string_type {
                    StringType::MultiLine => {
                        // Apply multiline processing to the text
                        process_multiline_text(text_parts)
                    }
                    StringType::TaskCommand => {
                        // Apply dedent to task command text
                        dedent_parts(&text_parts)
                    }
                    StringType::Regular => {
                        // Just join the parts
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
    let val = expr.eval(env, stdlib)?;

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

/// Helper function to evaluate multiline strings
fn eval_multiline_string(
    parts: &[StringPart],
    env: &Bindings<Value>,
    stdlib: &crate::stdlib::StdLib,
) -> Result<Value, WdlError> {
    // First, process parts to handle escaped newlines
    let mut processed_parts = Vec::new();

    for part in parts {
        match part {
            StringPart::Text(text) => {
                // Remove escaped newlines and following whitespace
                let processed = remove_escaped_newlines(text);
                processed_parts.push(StringPart::Text(processed));
            }
            StringPart::Placeholder { .. } => {
                processed_parts.push(part.clone());
            }
        }
    }

    // Evaluate all placeholders
    let mut text_parts = Vec::new();
    for part in &processed_parts {
        match part {
            StringPart::Text(text) => {
                // Decode escape sequences for multiline strings
                let decoded = decode_escape_sequences(text);
                text_parts.push(decoded);
            }
            StringPart::Placeholder { expr, options } => {
                let placeholder_text = evaluate_placeholder(expr, options, env, stdlib)?;
                text_parts.push(placeholder_text);
            }
        }
    }

    // Apply multiline processing
    let result = process_multiline_text(text_parts);
    Ok(Value::string(result))
}

/// Helper function to evaluate task command strings
fn eval_task_command(
    parts: &[StringPart],
    env: &Bindings<Value>,
    stdlib: &crate::stdlib::StdLib,
) -> Result<Value, WdlError> {
    // Process parts - don't remove escaped newlines for task commands
    let mut text_parts = Vec::new();
    for part in parts {
        match part {
            StringPart::Text(text) => {
                // Task commands don't decode escape sequences
                // They are passed as-is to the shell
                text_parts.push(text.clone());
            }
            StringPart::Placeholder { expr, options } => {
                let placeholder_text = evaluate_placeholder(expr, options, env, stdlib)?;
                text_parts.push(placeholder_text);
            }
        }
    }

    // Apply dedenting (without trimming)
    let result = dedent_parts(&text_parts);
    Ok(Value::string(result))
}

/// Process multiline text: trim whitespace and apply dedent
fn process_multiline_text(text_parts: Vec<String>) -> String {
    let mut content = text_parts.join("");

    // Trim whitespace from the left of the first line
    if let Some(newline_pos) = content.find('\n') {
        let first_line = &content[..newline_pos];
        if first_line.trim().is_empty() {
            // Remove the entire first line if it's only whitespace
            content = content[newline_pos + 1..].to_string();
        } else {
            // Just trim leading whitespace from first line
            let trimmed = first_line.trim_start();
            content = format!("{}{}", trimmed, &content[newline_pos..]);
        }
    } else {
        // Single line - just trim leading whitespace
        content = content.trim_start().to_string();
    }

    // Trim whitespace from the right of the last line
    if let Some(last_newline_pos) = content.rfind('\n') {
        let last_line = &content[last_newline_pos + 1..];
        if last_line.trim().is_empty() {
            // Remove the entire last line if it's only whitespace
            content = content[..last_newline_pos].to_string();
        } else {
            // Just trim trailing whitespace from last line
            let trimmed = last_line.trim_end();
            content = format!("{}{}", &content[..last_newline_pos + 1], trimmed);
        }
    } else {
        // Single line - just trim trailing whitespace
        content = content.trim_end().to_string();
    }

    // Now apply dedent
    dedent(&content)
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

/// Dedent text by removing common leading whitespace
fn dedent(text: &str) -> String {
    dedent_parts(&[text.to_string()])
}

/// Dedent multiple text parts
fn dedent_parts(parts: &[String]) -> String {
    let combined = parts.join("");

    if combined.is_empty() {
        return String::new();
    }

    // Check if the original string ends with a newline
    let ends_with_newline = combined.ends_with('\n');

    let lines: Vec<&str> = combined.lines().collect();

    if lines.is_empty() {
        return if ends_with_newline {
            "\n".to_string()
        } else {
            String::new()
        };
    }

    // Find minimum indentation among non-blank lines
    let mut min_indent: Option<usize> = None;
    for line in &lines {
        if !line.trim().is_empty() {
            let indent = line.len() - line.trim_start().len();
            min_indent = Some(match min_indent {
                None => indent,
                Some(current_min) => current_min.min(indent),
            });
        }
    }

    let indent_to_remove = min_indent.unwrap_or(0);

    // Remove the common indentation
    let dedented_lines: Vec<String> = lines
        .iter()
        .map(|line| {
            if line.len() > indent_to_remove {
                line[indent_to_remove..].to_string()
            } else if line.trim().is_empty() {
                // Keep blank lines as empty strings
                String::new()
            } else {
                line.to_string()
            }
        })
        .collect();

    let mut result = dedented_lines.join("\n");

    // Preserve the original trailing newline behavior
    if ends_with_newline && !result.ends_with('\n') {
        result.push('\n');
    }

    result
}
