//! Expression evaluation logic

use super::{Expression, ExpressionBase, StringPart, BinaryOperator, UnaryOperator};
use crate::error::{SourcePosition, WdlError, HasSourcePosition};
use crate::env::Bindings;
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
    
    fn eval(&self, env: &Bindings<Value>) -> Result<Value, WdlError> {
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
                            let val = expr.eval(env)?;
                            if val.is_null() {
                                if let Some(default) = options.get("default") {
                                    result.push_str(default);
                                }
                                // Otherwise add nothing for null values
                            } else {
                                // For string interpolation, extract the raw value without quotes
                                match &val {
                                    Value::String { value, .. } |
                                    Value::File { value, .. } |
                                    Value::Directory { value, .. } => {
                                        result.push_str(&value);
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
                    values.push(item.eval(env)?);
                }
                let item_type = if let Some(first) = values.first() {
                    first.wdl_type().clone()
                } else {
                    Type::any()
                };
                Ok(Value::array(item_type, values))
            }
            
            Expression::Pair { left, right, .. } => {
                let left_val = left.eval(env)?;
                let right_val = right.eval(env)?;
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
                    let key = k_expr.eval(env)?;
                    let value = v_expr.eval(env)?;
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
                    member_values.insert(name.clone(), expr.eval(env)?);
                }
                
                let member_types: HashMap<String, Type> = member_values
                    .iter()
                    .map(|(k, v)| (k.clone(), v.wdl_type().clone()))
                    .collect();
                
                Ok(Value::struct_value(Type::object(member_types), member_values, None))
            }
            
            Expression::Ident { name, .. } => {
                env.resolve(name).cloned().ok_or_else(|| {
                    WdlError::validation_error(
                        HasSourcePosition::source_position(self).clone(),
                        format!("Unknown identifier: {}", name),
                    )
                })
            }
            
            Expression::Get { expr, index, .. } => {
                let container = expr.eval(env)?;
                let idx = index.eval(env)?;
                
                match (&container, &idx) {
                    (Value::Array { values, .. }, Value::Int { value: i, .. }) => {
                        let index = *i as usize;
                        if index < values.len() {
                            Ok(values[index].clone())
                        } else {
                            Err(WdlError::OutOfBounds { pos: HasSourcePosition::source_position(self).clone() })
                        }
                    }
                    _ => Err(WdlError::validation_error(
                        HasSourcePosition::source_position(self).clone(),
                        "Invalid array/map access".to_string(),
                    )),
                }
            }
            
            Expression::IfThenElse { condition, true_expr, false_expr, .. } => {
                let cond_val = condition.eval(env)?;
                if let Some(cond_bool) = cond_val.as_bool() {
                    if cond_bool {
                        true_expr.eval(env)
                    } else {
                        false_expr.eval(env)
                    }
                } else {
                    Err(WdlError::validation_error(
                        HasSourcePosition::source_position(&**condition).clone(),
                        "If condition must be Boolean".to_string(),
                    ))
                }
            }
            
            Expression::Apply { function_name, arguments, .. } => {
                // Basic function implementations
                match function_name.as_str() {
                    "length" => {
                        if arguments.len() != 1 {
                            return Err(WdlError::validation_error(
                                HasSourcePosition::source_position(self).clone(),
                                format!("length() expects 1 argument, got {}", arguments.len()),
                            ));
                        }
                        let arg = arguments[0].eval(env)?;
                        match arg {
                            Value::Array { values, .. } => Ok(Value::int(values.len() as i64)),
                            Value::String { value, .. } => Ok(Value::int(value.len() as i64)),
                            _ => Err(WdlError::validation_error(
                                HasSourcePosition::source_position(self).clone(),
                                "length() requires Array or String argument".to_string(),
                            )),
                        }
                    }
                    "defined" => {
                        if arguments.len() != 1 {
                            return Err(WdlError::validation_error(
                                HasSourcePosition::source_position(self).clone(),
                                format!("defined() expects 1 argument, got {}", arguments.len()),
                            ));
                        }
                        let arg = arguments[0].eval(env)?;
                        Ok(Value::boolean(!arg.is_null()))
                    }
                    _ => Err(WdlError::validation_error(
                        HasSourcePosition::source_position(self).clone(),
                        format!("Unknown function: {}", function_name),
                    )),
                }
            }
            
            Expression::BinaryOp { op, left, right, .. } => {
                let left_val = left.eval(env)?;
                let right_val = right.eval(env)?;
                
                match op {
                    BinaryOperator::Add => {
                        match (&left_val, &right_val) {
                            (Value::Int { value: a, .. }, Value::Int { value: b, .. }) => {
                                Ok(Value::int(a + b))
                            }
                            (Value::Float { value: a, .. }, Value::Float { value: b, .. }) => {
                                Ok(Value::float(a + b))
                            }
                            (Value::Int { value: a, .. }, Value::Float { value: b, .. }) => {
                                Ok(Value::float((*a) as f64 + b))
                            }
                            (Value::Float { value: a, .. }, Value::Int { value: b, .. }) => {
                                Ok(Value::float(a + (*b) as f64))
                            }
                            (Value::String { value: a, .. }, Value::String { value: b, .. }) => {
                                Ok(Value::string(format!("{}{}", a, b)))
                            }
                            _ => Err(WdlError::validation_error(
                                HasSourcePosition::source_position(self).clone(),
                                "Invalid operands for addition".to_string(),
                            )),
                        }
                    }
                    BinaryOperator::Equal => {
                        Ok(Value::boolean(left_val.equals(&right_val).unwrap_or(false)))
                    }
                    BinaryOperator::And => {
                        let left_bool = left_val.as_bool().ok_or_else(|| {
                            WdlError::validation_error(
                                HasSourcePosition::source_position(&**left).clone(),
                                "Left operand must be Boolean".to_string(),
                            )
                        })?;
                        let right_bool = right_val.as_bool().ok_or_else(|| {
                            WdlError::validation_error(
                                HasSourcePosition::source_position(&**right).clone(),
                                "Right operand must be Boolean".to_string(),
                            )
                        })?;
                        Ok(Value::boolean(left_bool && right_bool))
                    }
                    // Add other operators as needed
                    _ => Err(WdlError::validation_error(
                        HasSourcePosition::source_position(self).clone(),
                        format!("Operator {:?} not yet implemented", op),
                    )),
                }
            }
            
            Expression::UnaryOp { op, operand, .. } => {
                let operand_val = operand.eval(env)?;
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
                    UnaryOperator::Negate => {
                        match operand_val {
                            Value::Int { value, .. } => Ok(Value::int(-value)),
                            Value::Float { value, .. } => Ok(Value::float(-value)),
                            _ => Err(WdlError::validation_error(
                                HasSourcePosition::source_position(&**operand).clone(),
                                "Operand must be numeric".to_string(),
                            )),
                        }
                    }
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