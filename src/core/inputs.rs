use crate::env::Bindings;
use crate::tree::{Document, Task, Workflow};
use crate::types::Type;
use crate::value::Value;
use crate::WdlError;
use std::cell::RefCell;
use std::path::PathBuf;

thread_local! {
    static INPUT_BASE_DIR: RefCell<Option<PathBuf>> = RefCell::new(None);
}

/// Guard returned by [`set_input_base_dir`] to restore the previous base directory when dropped.
pub struct BaseDirGuard {
    previous: Option<PathBuf>,
}

impl Drop for BaseDirGuard {
    fn drop(&mut self) {
        INPUT_BASE_DIR.with(|cell| {
            *cell.borrow_mut() = self.previous.take();
        });
    }
}

/// Set the base directory used to resolve relative `File` inputs for the current thread.
///
/// The previous base directory is restored when the returned guard is dropped.
pub fn set_input_base_dir(base_dir: Option<PathBuf>) -> BaseDirGuard {
    let previous = INPUT_BASE_DIR.with(|cell| {
        let mut borrow = cell.borrow_mut();
        std::mem::replace(&mut *borrow, base_dir)
    });

    BaseDirGuard { previous }
}

/// Convert JSON inputs into bindings suitable for executing a workflow or task.
pub fn bindings_from_json_for_document(
    json: serde_json::Value,
    document: &Document,
) -> Result<Bindings<Value>, WdlError> {
    if let Some(ref workflow) = document.workflow {
        json_to_bindings_with_types(json, workflow)
    } else if document.tasks.len() == 1 {
        let task = &document.tasks[0];
        json_to_bindings_with_task_types(json, task)
    } else {
        json_to_bindings(json)
    }
}

/// Convert JSON inputs into bindings for a specific task, using the task's type declarations when possible.
pub fn bindings_from_json_for_task(
    json: serde_json::Value,
    task: &Task,
) -> Result<Bindings<Value>, WdlError> {
    json_to_bindings_with_task_types(json, task)
}

fn resolve_file_path(path: &str) -> Result<PathBuf, WdlError> {
    let path_buf = PathBuf::from(path);

    let resolved = if path_buf.is_absolute() {
        path_buf
    } else {
        let base_dir = INPUT_BASE_DIR.with(|cell| cell.borrow().clone());
        let base = if let Some(dir) = base_dir {
            dir
        } else {
            std::env::current_dir().map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to get current directory: {}", e),
            })?
        };
        base.join(&path_buf)
    };

    if !resolved.exists() {
        return Err(WdlError::RuntimeError {
            message: format!("Input file not found: {}", path),
        });
    }

    resolved.canonicalize().map_err(|e| WdlError::RuntimeError {
        message: format!("Failed to resolve file path '{}': {}", path, e),
    })
}

fn json_to_bindings_with_types(
    json: serde_json::Value,
    workflow: &Workflow,
) -> Result<Bindings<Value>, WdlError> {
    let mut bindings = Bindings::new();

    if let serde_json::Value::Object(map) = json {
        for (key, json_value) in map {
            let input_name = if key.starts_with(&format!("{}.", workflow.name)) {
                key.strip_prefix(&format!("{}.", workflow.name))
                    .unwrap()
                    .to_string()
            } else {
                key.clone()
            };

            let input_type = workflow
                .inputs
                .iter()
                .find(|decl| decl.name == input_name)
                .map(|decl| &decl.decl_type);

            let wdl_value = if let Some(ty) = input_type {
                json_to_value_typed(json_value, ty)?
            } else {
                json_to_value(json_value)?
            };

            bindings = bindings.bind(input_name, wdl_value, None);
        }
    }

    Ok(bindings)
}

fn json_to_bindings_with_task_types(
    json: serde_json::Value,
    task: &Task,
) -> Result<Bindings<Value>, WdlError> {
    let mut bindings = Bindings::new();

    if let serde_json::Value::Object(map) = json {
        for (key, json_value) in map {
            let input_name = if key.starts_with(&format!("{}.", task.name)) {
                key.strip_prefix(&format!("{}.", task.name))
                    .unwrap()
                    .to_string()
            } else {
                key.clone()
            };

            let input_type = task
                .inputs
                .iter()
                .find(|decl| decl.name == input_name)
                .map(|decl| &decl.decl_type);

            let wdl_value = if let Some(ty) = input_type {
                json_to_value_typed(json_value, ty)?
            } else {
                json_to_value(json_value)?
            };

            bindings = bindings.bind(input_name, wdl_value, None);
        }
    }

    Ok(bindings)
}

fn json_to_value_typed(json: serde_json::Value, wdl_type: &Type) -> Result<Value, WdlError> {
    match (json, wdl_type) {
        (serde_json::Value::String(s), Type::File { optional }) => {
            let resolved_path = resolve_file_path(&s)?;
            Value::file(resolved_path.to_string_lossy().to_string()).map_err(|e| {
                WdlError::RuntimeError {
                    message: e.to_string(),
                }
            })
        }
        (serde_json::Value::String(s), Type::String { optional }) => Ok(Value::String {
            value: s,
            wdl_type: Type::String {
                optional: *optional,
            },
        }),
        (serde_json::Value::Bool(b), Type::Boolean { optional }) => Ok(Value::Boolean {
            value: b,
            wdl_type: Type::Boolean {
                optional: *optional,
            },
        }),
        (serde_json::Value::Number(n), Type::Int { optional }) => {
            if let Some(i) = n.as_i64() {
                Ok(Value::Int {
                    value: i,
                    wdl_type: Type::Int {
                        optional: *optional,
                    },
                })
            } else {
                Err(WdlError::RuntimeError {
                    message: format!("Cannot convert {} to Int", n),
                })
            }
        }
        (serde_json::Value::Number(n), Type::Float { optional }) => {
            if let Some(f) = n.as_f64() {
                Ok(Value::Float {
                    value: f,
                    wdl_type: Type::Float {
                        optional: *optional,
                    },
                })
            } else {
                Err(WdlError::RuntimeError {
                    message: format!("Cannot convert {} to Float", n),
                })
            }
        }
        (
            serde_json::Value::Array(arr),
            Type::Array {
                item_type,
                optional,
                nonempty,
            },
        ) => {
            let values: Result<Vec<_>, _> = arr
                .into_iter()
                .map(|v| json_to_value_typed(v, item_type))
                .collect();
            Ok(Value::Array {
                values: values?,
                wdl_type: Type::Array {
                    item_type: item_type.clone(),
                    optional: *optional,
                    nonempty: *nonempty,
                },
            })
        }
        (
            serde_json::Value::Object(obj),
            Type::Map {
                key_type,
                value_type,
                optional,
                ..
            },
        ) => {
            let pairs: Result<Vec<_>, _> = obj
                .into_iter()
                .map(|(k, v)| {
                    let wdl_key = json_to_value_typed(serde_json::Value::String(k), key_type)?;
                    let wdl_value = json_to_value_typed(v, value_type)?;
                    Ok((wdl_key, wdl_value))
                })
                .collect();
            Ok(Value::Map {
                pairs: pairs?,
                wdl_type: Type::Map {
                    key_type: key_type.clone(),
                    value_type: value_type.clone(),
                    optional: *optional,
                    literal_keys: None,
                },
            })
        }
        (
            serde_json::Value::Array(mut arr),
            Type::Pair {
                left_type,
                right_type,
                optional,
            },
        ) if arr.len() == 2 => {
            let right = arr.pop().unwrap();
            let left = arr.pop().unwrap();
            let left_value = json_to_value_typed(left, left_type)?;
            let right_value = json_to_value_typed(right, right_type)?;
            Ok(Value::Pair {
                left: Box::new(left_value),
                right: Box::new(right_value),
                wdl_type: Type::Pair {
                    left_type: left_type.clone(),
                    right_type: right_type.clone(),
                    optional: *optional,
                },
            })
        }
        (
            serde_json::Value::Object(obj),
            Type::StructInstance {
                type_name,
                members,
                optional,
                ..
            },
        ) => {
            if let Some(ref struct_members) = members {
                let mut wdl_members = std::collections::HashMap::new();

                for (member_name, member_type) in struct_members {
                    if let Some(json_value) = obj.get(member_name) {
                        let wdl_value = json_to_value_typed(json_value.clone(), member_type)?;
                        wdl_members.insert(member_name.clone(), wdl_value);
                    } else if !member_type.is_optional() {
                        return Err(WdlError::RuntimeError {
                            message: format!("Missing required struct member: {}", member_name),
                        });
                    }
                }

                Ok(Value::Struct {
                    members: wdl_members,
                    extra_keys: std::collections::HashSet::new(),
                    wdl_type: Type::StructInstance {
                        type_name: type_name.clone(),
                        members: members.clone(),
                        optional: *optional,
                    },
                })
            } else {
                Err(WdlError::RuntimeError {
                    message: format!("Struct type {} has no member information", type_name),
                })
            }
        }
        (json_val, _) => json_to_value(json_val),
    }
}

fn json_to_bindings(json: serde_json::Value) -> Result<Bindings<Value>, WdlError> {
    let mut bindings = Bindings::new();

    if let serde_json::Value::Object(map) = json {
        for (key, value) in map {
            let wdl_value = json_to_value(value)?;
            bindings = bindings.bind(key, wdl_value, None);
        }
    }

    Ok(bindings)
}

fn json_to_value(json: serde_json::Value) -> Result<Value, WdlError> {
    match json {
        serde_json::Value::Null => Ok(Value::Null),
        serde_json::Value::Bool(b) => Ok(Value::boolean(b)),
        serde_json::Value::Number(n) => {
            if let Some(i) = n.as_i64() {
                Ok(Value::int(i))
            } else if let Some(f) = n.as_f64() {
                Ok(Value::float(f))
            } else {
                Err(WdlError::Validation {
                    message: format!("Invalid number: {}", n),
                    pos: crate::SourcePosition::new(String::new(), String::new(), 0, 0, 0, 0),
                    source_text: Some(String::new()),
                    declared_wdl_version: Some("1.0".to_string()),
                })
            }
        }
        serde_json::Value::String(s) => Ok(Value::string(s)),
        serde_json::Value::Array(arr) => {
            let values: Result<Vec<_>, _> = arr.into_iter().map(json_to_value).collect();
            Ok(Value::array(Type::any(), values?))
        }
        serde_json::Value::Object(map) => {
            let mut struct_map = std::collections::HashMap::new();
            for (key, value) in map {
                struct_map.insert(key, json_to_value(value)?);
            }
            Ok(Value::Struct {
                members: struct_map,
                extra_keys: std::collections::HashSet::new(),
                wdl_type: Type::Object {
                    members: std::collections::HashMap::new(),
                    is_call_output: false,
                },
            })
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::parser;

    #[test]
    fn test_bindings_from_json_for_document_with_workflow() {
        let wdl = r#"
        version 1.0
        workflow wf {
            input {
                Int i
                String s
            }
            call t {
                input: x = i
            }

            output {
                Int res = t.y
            }
        }

        task t {
            input {
                Int x
            }
            command <<<
                echo ${x}
            >>>
            output {
                Int y = read_int(stdout())
            }
        }
        "#;

        let document = parser::parse_document(wdl, "1.0").unwrap();
        let json = serde_json::json!({"wf.i": 5, "wf.s": "hello"});
        let bindings = bindings_from_json_for_document(json, &document).unwrap();
        assert!(bindings.has_binding("i"));
        assert!(bindings.has_binding("s"));
    }
}
