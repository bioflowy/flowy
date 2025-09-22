//! I/O functions for the WDL standard library
//!
//! This module provides file reading and writing functions similar to miniwdl's I/O functions.

use crate::env::Bindings;
use crate::error::{SourcePosition, WdlError};
use crate::expr::ExpressionBase;
use crate::stdlib::Function;
use crate::types::Type;
use crate::value::{Value, ValueBase};
use serde_json::Value as JsonValue;
use std::io::Write;
use std::os::unix::fs::PermissionsExt;
use std::path::Path;
use std::{
    collections::{HashMap, HashSet},
    fs,
};

/// Write file function that follows the original WriteFileFunction pattern
/// Uses eval_with_stdlib for proper file handling in different contexts
pub struct WriteFileFunction<F>
where
    F: Fn(&Value, &mut dyn Write) -> Result<(), WdlError> + Send + Sync + 'static,
{
    name: String,
    argument_type: Type,
    serializer: Box<F>,
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
    write_dir: String,
}

impl<F> Function for WriteFileFunction<F>
where
    F: Fn(&Value, &mut dyn Write) -> Result<(), WdlError> + Send + Sync + 'static,
{
    fn infer_type(
        &self,
        args: &mut [crate::expr::Expression],
        type_env: &crate::env::Bindings<Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        if args.len() != 1 {
            let pos = if args.is_empty() {
                SourcePosition::new("unknown".to_string(), "unknown".to_string(), 0, 0, 0, 0)
            } else {
                args[0].source_position().clone()
            };
            return Err(WdlError::Validation {
                pos,
                message: format!("{} expects 1 argument, got {}", self.name, args.len()),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        let arg_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        if !arg_type.coerces(&self.argument_type, true) {
            let pos = args[0].source_position().clone();
            return Err(WdlError::Validation {
                pos,
                message: format!(
                    "{} expects {}, got {}",
                    self.name, self.argument_type, arg_type
                ),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        Ok(Type::file(false))
    }

    fn name(&self) -> &str {
        &self.name
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        env: &crate::env::Bindings<Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::RuntimeError {
                message: format!("{} expects 1 argument, got {}", self.name, args.len()),
            });
        }

        let value = args[0].eval(env, stdlib)?;

        // Use task directory if available, otherwise use write_dir
        let target_dir = if let Some(task_dir) = stdlib.task_dir() {
            task_dir.join("work").join("write_")
        } else {
            std::path::PathBuf::from(&self.write_dir)
        };

        // Create target directory if it doesn't exist
        std::fs::create_dir_all(&target_dir).map_err(|e| WdlError::RuntimeError {
            message: format!(
                "Failed to create directory '{}': {}",
                target_dir.display(),
                e
            ),
        })?;

        // Create a temporary file in the target directory
        let mut temp_file =
            tempfile::NamedTempFile::new_in(&target_dir).map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to create temporary file: {}", e),
            })?;

        let temp_path = temp_file.path().to_path_buf();

        // Serialize the value to the file and handle persistence
        let virtual_filename = {
            (self.serializer)(&value, &mut temp_file)?;
            temp_file.flush().map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to flush file: {}", e),
            })?;

            // Set file permissions (equivalent to chmod 0o660)
            let mut perms = std::fs::metadata(&temp_path)
                .map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to get file metadata: {}", e),
                })?
                .permissions();
            perms.set_mode(0o660);
            std::fs::set_permissions(&temp_path, perms).map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to set file permissions: {}", e),
            })?;

            // Persist the temp file to prevent automatic deletion
            temp_file
                .persist(&temp_path)
                .map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to persist temporary file: {}", e),
                })?;

            // Virtualize the filename using PathMapper
            let virtual_name = self.path_mapper.virtualize_filename(&temp_path)?;

            // Debug output to see the actual file paths
            if std::env::var("RUST_BACKTRACE").is_ok() || std::env::var("DEBUG").is_ok() {
                eprintln!(
                    "DEBUG: {} created file: real_path={:?}, virtual_name={}",
                    self.name, temp_path, virtual_name
                );
            }

            virtual_name
        };

        Value::file(virtual_filename)
    }
}

/// Create a read function implementation based on a parse function
///
/// This is similar to miniwdl's _read() method that generates read_* function
/// implementations based on a parse function.
///
/// # Arguments
/// * `name` - Function name (e.g., "read_string", "read_int")
/// * `return_type` - The expected return type
/// * `parse` - Function that parses the file content string into a Value
/// * `path_mapper` - PathMapper instance for file virtualization
///
/// # Returns
/// A Function implementation that reads files and parses their content
fn create_read_function<F>(
    name: String,
    return_type: Type,
    parse: F,
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
) -> Box<dyn Function>
where
    F: Fn(&str) -> Result<Value, WdlError> + Send + Sync + 'static,
{
    use crate::stdlib::create_static_function;

    create_static_function(name, vec![Type::file(false)], return_type, move |args| {
        // args[0] is guaranteed to be a File value due to create_static_function's type checking
        let file_value = &args[0];

        // Get the filename from the File value
        let virtual_filename = file_value
            .as_string()
            .ok_or_else(|| WdlError::RuntimeError {
                message: "Invalid file value".to_string(),
            })?;

        // Use PathMapper to devirtualize the filename
        let real_filename = path_mapper.devirtualize_filename(virtual_filename)?;

        // Read the file content
        let content = fs::read_to_string(&real_filename).map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to read file '{}': {}", real_filename.display(), e),
        })?;

        // Parse the content using the provided parse function
        parse(&content)
    })
}

/// Create a write function implementation based on a serialize function
///
/// This is similar to miniwdl's _write() method that generates write_* function
/// implementations based on a serialize function.
///
/// # Arguments
/// * `name` - Function name (e.g., "write_lines", "write_tsv")
/// * `argument_type` - The expected argument type
/// * `serialize` - Function that serializes the value to bytes
/// * `path_mapper` - PathMapper instance for file virtualization
/// * `write_dir` - Directory where files should be written
///
/// # Returns
/// A Function implementation that writes values to files
fn create_write_function<F>(
    name: String,
    argument_type: Type,
    serialize: F,
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
    write_dir: String,
) -> Box<dyn Function>
where
    F: Fn(&Value, &mut dyn Write) -> Result<(), WdlError> + Send + Sync + 'static,
{
    use crate::stdlib::create_static_function;

    Box::new(WriteFileFunction {
        name,
        argument_type,
        serializer: Box::new(serialize),
        path_mapper,
        write_dir,
    })
}

/// Create read_string function: read_string(File) -> String
/// Reads a file and returns its content as a string, with trailing newline removed
pub fn create_read_string_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
) -> Box<dyn Function> {
    create_read_function(
        "read_string".to_string(),
        Type::string(false),
        |content: &str| {
            // Remove trailing newline like miniwdl does
            let trimmed = if let Some(stripped) = content.strip_suffix('\n') {
                stripped
            } else {
                content
            };
            Ok(Value::string(trimmed.to_string()))
        },
        path_mapper,
    )
}

/// Create read_lines function: read_lines(File) -> Array[String]
/// Reads a file and returns its content as an array of lines
pub fn create_read_lines_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
) -> Box<dyn Function> {
    create_read_function(
        "read_lines".to_string(),
        Type::array(Type::string(false), false, false),
        |content: &str| {
            // Split content into lines and convert to Value array
            let lines: Vec<Value> = content
                .lines()
                .map(|line| Value::string(line.to_string()))
                .collect();

            Ok(Value::array(Type::string(false), lines))
        },
        path_mapper,
    )
}

/// Create read_map function: read_map(File) -> Map[String, String]
/// Reads a two-column TSV file into an ordered Map
pub fn create_read_map_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
) -> Box<dyn Function> {
    create_read_function(
        "read_map".to_string(),
        Type::map(Type::string(false), Type::string(false), false),
        |content: &str| parse_map_content(content),
        path_mapper,
    )
}

/// Create read_object function: read_object(File) -> Object
/// Reads a two-row TSV file into an Object value
pub fn create_read_object_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
) -> Box<dyn Function> {
    create_read_function(
        "read_object".to_string(),
        Type::object(HashMap::new()),
        |content: &str| match parse_objects_rows(content, "read_object")? {
            Some((header, mut rows)) => {
                if rows.len() != 1 {
                    return Err(WdlError::RuntimeError {
                        message: "read_object(): file must have exactly one object".to_string(),
                    });
                }
                let object_type = build_object_type(&header);
                let values = rows.remove(0);
                Ok(build_object_value(&header, values, &object_type))
            }
            None => Err(WdlError::RuntimeError {
                message: "read_object(): file must have exactly one object".to_string(),
            }),
        },
        path_mapper,
    )
}

/// Create read_objects function: read_objects(File) -> Array[Object]
/// Reads a TSV file into an array of Object values
pub fn create_read_objects_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
) -> Box<dyn Function> {
    create_read_function(
        "read_objects".to_string(),
        Type::array(Type::object(HashMap::new()), false, false),
        |content: &str| match parse_objects_rows(content, "read_objects")? {
            Some((header, rows)) => {
                let object_type = build_object_type(&header);
                if rows.is_empty() {
                    return Ok(Value::array(object_type, Vec::new()));
                }

                let objects: Vec<Value> = rows
                    .into_iter()
                    .map(|row| build_object_value(&header, row, &object_type))
                    .collect();

                Ok(Value::array(object_type, objects))
            }
            None => Ok(Value::array(Type::object(HashMap::new()), Vec::new())),
        },
        path_mapper,
    )
}

/// Create read_json function: read_json(File) -> Any
/// Reads a file containing JSON and converts it to a WDL value
pub fn create_read_json_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
) -> Box<dyn Function> {
    create_read_function(
        "read_json".to_string(),
        Type::any(),
        |content: &str| {
            let json_value: JsonValue =
                serde_json::from_str(content).map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to parse JSON: {}", e),
                })?;
            Ok(Value::from_json(json_value))
        },
        path_mapper,
    )
}

fn parse_map_content(content: &str) -> Result<Value, WdlError> {
    let mut pairs: Vec<(Value, Value)> = Vec::new();
    let mut seen_keys: HashSet<String> = HashSet::new();

    if content.is_empty() {
        return Ok(Value::map(Type::string(false), Type::string(false), pairs));
    }

    for (index, raw_line) in content.lines().enumerate() {
        let line = raw_line.trim_end_matches('\r');
        let mut columns = line.splitn(3, '\t');

        let key = columns.next().unwrap_or("");
        let value = columns.next();
        let extra = columns.next();

        if key.is_empty() && value.is_none() {
            continue; // skip blank lines
        }

        let value = match value {
            Some(v) if extra.is_none() => v,
            Some(_) | None => {
                return Err(WdlError::RuntimeError {
                    message: format!(
                        "read_map(): line {} does not contain exactly two columns",
                        index + 1
                    ),
                });
            }
        };

        if !seen_keys.insert(key.to_string()) {
            return Err(WdlError::RuntimeError {
                message: format!("read_map(): duplicate key '{}' detected", key),
            });
        }

        pairs.push((
            Value::string(key.to_string()),
            Value::string(value.to_string()),
        ));
    }

    Ok(Value::map(Type::string(false), Type::string(false), pairs))
}

fn parse_objects_rows(
    content: &str,
    function_name: &str,
) -> Result<Option<(Vec<String>, Vec<Vec<String>>)>, WdlError> {
    let rows = parse_tsv_rows(content);
    if rows.is_empty() {
        return Ok(None);
    }

    let mut iter = rows.into_iter();
    let header = iter.next().unwrap();
    if header.is_empty() {
        return Ok(None);
    }

    let mut seen = HashSet::new();
    for name in &header {
        if name.is_empty() || !seen.insert(name.clone()) {
            return Err(WdlError::RuntimeError {
                message: format!(
                    "{}(): file has empty or duplicate column names",
                    function_name
                ),
            });
        }
    }

    let header_len = header.len();
    let mut data_rows = Vec::new();

    for row in iter {
        if row.len() != header_len {
            return Err(WdlError::RuntimeError {
                message: format!("{}(): file's tab-separated lines are ragged", function_name),
            });
        }
        data_rows.push(row);
    }

    Ok(Some((header, data_rows)))
}

fn parse_tsv_rows(content: &str) -> Vec<Vec<String>> {
    content
        .lines()
        .map(|line| line.trim_end_matches('\r'))
        .filter(|line| !line.is_empty())
        .map(|line| line.split('\t').map(|cell| cell.to_string()).collect())
        .collect()
}

fn build_object_type(header: &[String]) -> Type {
    let member_types: HashMap<String, Type> = header
        .iter()
        .cloned()
        .map(|name| (name, Type::string(false)))
        .collect();
    Type::object(member_types)
}

fn build_object_value(header: &[String], row: Vec<String>, object_type: &Type) -> Value {
    let members: HashMap<String, Value> = header
        .iter()
        .cloned()
        .zip(row.into_iter().map(Value::string))
        .collect();

    Value::struct_value_unchecked(object_type.clone(), members, None)
}

/// Create read_int function: read_int(File) -> Int
/// Reads a file and parses its content as an integer
pub fn create_read_int_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
) -> Box<dyn Function> {
    create_read_function(
        "read_int".to_string(),
        Type::int(false),
        |content: &str| {
            let trimmed = content.trim();
            let value = trimmed.parse::<i64>().map_err(|e| WdlError::RuntimeError {
                message: format!("Cannot parse '{}' as integer: {}", trimmed, e),
            })?;
            Ok(Value::int(value))
        },
        path_mapper,
    )
}

/// Create read_float function: read_float(File) -> Float
/// Reads a file and parses its content as a float
pub fn create_read_float_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
) -> Box<dyn Function> {
    create_read_function(
        "read_float".to_string(),
        Type::float(false),
        |content: &str| {
            let trimmed = content.trim();
            let value = trimmed.parse::<f64>().map_err(|e| WdlError::RuntimeError {
                message: format!("Cannot parse '{}' as float: {}", trimmed, e),
            })?;
            Ok(Value::float(value))
        },
        path_mapper,
    )
}

/// Create read_boolean function: read_boolean(File) -> Boolean
/// Reads a file and parses its content as a boolean
pub fn create_read_boolean_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
) -> Box<dyn Function> {
    create_read_function(
        "read_boolean".to_string(),
        Type::boolean(false),
        |content: &str| {
            let trimmed = content.trim().to_lowercase();
            let value = match trimmed.as_str() {
                "true" | "1" => true,
                "false" | "0" => false,
                _ => {
                    return Err(WdlError::RuntimeError {
                        message: format!("Cannot parse '{}' as boolean", trimmed),
                    });
                }
            };
            Ok(Value::boolean(value))
        },
        path_mapper,
    )
}

/// Create write_lines function: write_lines(Array[String]) -> File
/// Writes an array of strings to a file, one line per string
pub fn create_write_lines_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
    write_dir: String,
) -> Box<dyn Function> {
    create_write_function(
        "write_lines".to_string(),
        Type::array(Type::string(false), false, false),
        |value: &Value, file: &mut dyn Write| {
            let array = value.as_array().ok_or_else(|| WdlError::RuntimeError {
                message: "Expected array value for write_lines".to_string(),
            })?;

            for item in array {
                let line = item.as_string().ok_or_else(|| WdlError::RuntimeError {
                    message: "All array items must be strings for write_lines".to_string(),
                })?;
                file.write_all(line.as_bytes())
                    .map_err(|e| WdlError::RuntimeError {
                        message: format!("Failed to write line: {}", e),
                    })?;
                file.write_all(b"\n").map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to write newline: {}", e),
                })?;
            }
            Ok(())
        },
        path_mapper,
        write_dir,
    )
}

/// Create write_map function: write_map(Map[String, String]) -> File
/// Writes a map as tab-separated key-value pairs
pub fn create_write_map_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
    write_dir: String,
) -> Box<dyn Function> {
    create_write_function(
        "write_map".to_string(),
        Type::map(Type::string(false), Type::string(false), false),
        |value: &Value, file: &mut dyn Write| {
            let pairs = match value {
                Value::Map { pairs, .. } => pairs,
                _ => {
                    return Err(WdlError::RuntimeError {
                        message: "Expected map value for write_map".to_string(),
                    });
                }
            };

            for (key, val) in pairs {
                let key_str = key.as_string().ok_or_else(|| WdlError::RuntimeError {
                    message: "Map keys must be strings for write_map".to_string(),
                })?;
                let val_str = val.as_string().ok_or_else(|| WdlError::RuntimeError {
                    message: "Map values must be strings for write_map".to_string(),
                })?;

                // Check for forbidden characters like miniwdl does
                if key_str.contains('\t')
                    || key_str.contains('\n')
                    || val_str.contains('\t')
                    || val_str.contains('\n')
                {
                    return Err(WdlError::RuntimeError {
                        message:
                            "write_map(): keys & values must not contain tab or newline characters"
                                .to_string(),
                    });
                }

                let line = format!("{}\t{}\n", key_str, val_str);
                file.write_all(line.as_bytes())
                    .map_err(|e| WdlError::RuntimeError {
                        message: format!("Failed to write map line: {}", e),
                    })?;
            }
            Ok(())
        },
        path_mapper,
        write_dir,
    )
}

fn primitive_value_to_string(value: &Value) -> Option<String> {
    match value {
        Value::Boolean { value, .. } => Some(value.to_string()),
        Value::Int { value, .. } => Some(value.to_string()),
        Value::Float { value, .. } => Some(value.to_string()),
        Value::String { value, .. }
        | Value::File { value, .. }
        | Value::Directory { value, .. } => Some(value.clone()),
        _ => None,
    }
}

fn extract_struct_members<'a>(
    value: &'a Value,
    function_name: &str,
) -> Result<(&'a HashMap<String, Value>, &'a Type), WdlError> {
    match value {
        Value::Struct {
            members, wdl_type, ..
        } => Ok((members, wdl_type)),
        _ => Err(WdlError::RuntimeError {
            message: format!("{}(): expected Object or Struct value", function_name),
        }),
    }
}

fn determine_struct_member_order(
    members: &HashMap<String, Value>,
    value_type: &Type,
) -> Vec<String> {
    let mut ordered_keys: Vec<String> = Vec::new();
    let mut seen = HashSet::new();

    match value_type {
        Type::StructInstance {
            members: Some(struct_members),
            ..
        } => {
            for key in struct_members.keys() {
                if members.contains_key(key) && seen.insert(key.clone()) {
                    ordered_keys.push(key.clone());
                }
            }
        }
        Type::Object {
            members: type_members,
            ..
        } => {
            if !type_members.is_empty() {
                let mut sorted: Vec<String> = type_members.keys().cloned().collect();
                sorted.sort();
                for key in sorted {
                    if members.contains_key(&key) && seen.insert(key.clone()) {
                        ordered_keys.push(key);
                    }
                }
            }
        }
        _ => {}
    }

    let mut remaining: Vec<String> = members
        .keys()
        .filter(|k| !seen.contains(*k))
        .cloned()
        .collect();
    remaining.sort();
    ordered_keys.extend(remaining);

    ordered_keys
}

fn serialize_struct_members(
    members: &HashMap<String, Value>,
    ordered_keys: &[String],
    function_name: &str,
) -> Result<Vec<String>, WdlError> {
    let mut value_parts = Vec::with_capacity(ordered_keys.len());

    for key in ordered_keys {
        let member_value = members.get(key).ok_or_else(|| WdlError::RuntimeError {
            message: format!("{}(): missing value for member '{}'", function_name, key),
        })?;

        let serialized =
            primitive_value_to_string(member_value).ok_or_else(|| WdlError::RuntimeError {
                message: format!(
                    "{}(): member '{}' must be a primitive value",
                    function_name, key
                ),
            })?;

        if serialized.contains('\n') || serialized.contains('\r') {
            return Err(WdlError::RuntimeError {
                message: format!(
                    "{}(): value for '{}' must not contain newline characters",
                    function_name, key
                ),
            });
        }

        value_parts.push(serialized);
    }

    if members.len() != ordered_keys.len() {
        let key_set: HashSet<&String> = ordered_keys.iter().collect();
        let extra_keys: Vec<&String> = members.keys().filter(|k| !key_set.contains(*k)).collect();
        if !extra_keys.is_empty() {
            return Err(WdlError::RuntimeError {
                message: format!(
                    "{}(): unexpected members found: {}",
                    function_name,
                    extra_keys
                        .iter()
                        .map(|k| k.as_str())
                        .collect::<Vec<&str>>()
                        .join(", ")
                ),
            });
        }
    }

    Ok(value_parts)
}

/// Create write_object function: write_object(Object|Struct) -> File
pub fn create_write_object_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
    write_dir: String,
) -> Box<dyn Function> {
    create_write_function(
        "write_object".to_string(),
        Type::any(),
        |value: &Value, file: &mut dyn Write| {
            let (members, value_type) = extract_struct_members(value, "write_object")?;
            let ordered_keys = determine_struct_member_order(members, value_type);
            let value_parts = serialize_struct_members(members, &ordered_keys, "write_object")?;

            file.write_all(ordered_keys.join("\t").as_bytes())
                .map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to write object header: {}", e),
                })?;
            file.write_all(b"\n").map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to write newline: {}", e),
            })?;

            file.write_all(value_parts.join("\t").as_bytes())
                .map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to write object values: {}", e),
                })?;
            file.write_all(b"\n").map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to write newline: {}", e),
            })
        },
        path_mapper,
        write_dir,
    )
}

/// Create write_objects function: write_objects(Array[Struct|Object]) -> File
pub fn create_write_objects_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
    write_dir: String,
) -> Box<dyn Function> {
    create_write_function(
        "write_objects".to_string(),
        Type::array(Type::any(), false, false),
        |value: &Value, file: &mut dyn Write| {
            let array_values = value.as_array().ok_or_else(|| WdlError::RuntimeError {
                message: "write_objects(): expected Array value".to_string(),
            })?;

            if array_values.is_empty() {
                return Ok(());
            }

            let mut header_keys: Vec<String> = Vec::new();
            let mut rows: Vec<String> = Vec::with_capacity(array_values.len());

            for (idx, element) in array_values.iter().enumerate() {
                let (members, value_type) = extract_struct_members(element, "write_objects")?;
                let current_keys = determine_struct_member_order(members, value_type);

                if idx == 0 {
                    header_keys = current_keys.clone();
                } else if header_keys.len() != current_keys.len()
                    || !header_keys
                        .iter()
                        .zip(current_keys.iter())
                        .all(|(a, b)| a == b)
                {
                    return Err(WdlError::RuntimeError {
                        message: format!(
                            "write_objects(): array elements must have the same member names; element {} differs",
                            idx
                        ),
                    });
                }

                let row_parts = serialize_struct_members(members, &header_keys, "write_objects")?;
                rows.push(row_parts.join("\t"));
            }

            file.write_all(header_keys.join("\t").as_bytes())
                .map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to write objects header: {}", e),
                })?;
            file.write_all(b"\n").map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to write newline: {}", e),
            })?;

            for row in rows {
                file.write_all(row.as_bytes())
                    .map_err(|e| WdlError::RuntimeError {
                        message: format!("Failed to write objects row: {}", e),
                    })?;
                file.write_all(b"\n").map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to write newline: {}", e),
                })?;
            }

            Ok(())
        },
        path_mapper,
        write_dir,
    )
}

/// Create write_json function: write_json(Any) -> File
/// Writes any value as JSON
pub fn create_write_json_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
    write_dir: String,
) -> Box<dyn Function> {
    create_write_function(
        "write_json".to_string(),
        Type::any(),
        |value: &Value, file: &mut dyn Write| {
            let json_value = value.to_json();
            let json_string =
                serde_json::to_string(&json_value).map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to serialize value as JSON: {}", e),
                })?;

            file.write_all(json_string.as_bytes())
                .map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to write JSON: {}", e),
                })?;
            Ok(())
        },
        path_mapper,
        write_dir,
    )
}

/// Create stdout function: stdout() -> File
/// Returns a file representing standard output (only available in task context)
pub fn create_stdout_function() -> Box<dyn Function> {
    Box::new(StdoutFunction)
}

/// Stdout function implementation that returns the task's stdout.txt file
struct StdoutFunction;

impl Function for StdoutFunction {
    fn name(&self) -> &str {
        "stdout"
    }

    fn infer_type(
        &self,
        args: &mut [crate::expr::Expression],
        _type_env: &crate::env::Bindings<crate::types::Type>,
        _stdlib: &crate::stdlib::StdLib,
        _struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<crate::types::Type, crate::error::WdlError> {
        if !args.is_empty() {
            return Err(crate::error::WdlError::Validation {
                pos: args[0].source_position().clone(),
                message: "stdout() takes no arguments".to_string(),
                source_text: None,
                declared_wdl_version: None,
            });
        }
        Ok(crate::types::Type::file(false))
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        _env: &crate::env::Bindings<crate::value::Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<crate::value::Value, crate::error::WdlError> {
        if !args.is_empty() {
            return Err(crate::error::WdlError::RuntimeError {
                message: "stdout() takes no arguments".to_string(),
            });
        }

        // Use path mapper to resolve stdout path
        let real_path = stdlib.path_mapper().devirtualize_filename("stdout.txt")?;
        let virtual_path = stdlib.path_mapper().virtualize_filename(&real_path)?;
        crate::value::Value::file(virtual_path)
    }
}

/// Create stderr function: stderr() -> File
/// Returns a file representing standard error (only available in task context)
pub fn create_stderr_function() -> Box<dyn Function> {
    Box::new(StderrFunction)
}

/// Stderr function implementation that returns the task's stderr.txt file
struct StderrFunction;

impl Function for StderrFunction {
    fn name(&self) -> &str {
        "stderr"
    }

    fn infer_type(
        &self,
        args: &mut [crate::expr::Expression],
        _type_env: &crate::env::Bindings<crate::types::Type>,
        _stdlib: &crate::stdlib::StdLib,
        _struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<crate::types::Type, crate::error::WdlError> {
        if !args.is_empty() {
            return Err(crate::error::WdlError::Validation {
                pos: args[0].source_position().clone(),
                message: "stderr() takes no arguments".to_string(),
                source_text: None,
                declared_wdl_version: None,
            });
        }
        Ok(crate::types::Type::file(false))
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        _env: &crate::env::Bindings<crate::value::Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<crate::value::Value, crate::error::WdlError> {
        if !args.is_empty() {
            return Err(crate::error::WdlError::RuntimeError {
                message: "stderr() takes no arguments".to_string(),
            });
        }

        // Use path mapper to resolve stderr path
        let real_path = stdlib.path_mapper().devirtualize_filename("stderr.txt")?;
        let virtual_path = stdlib.path_mapper().virtualize_filename(&real_path)?;
        crate::value::Value::file(virtual_path)
    }
}

/// Create glob function: glob(pattern: String) -> Array[File]
/// Returns an array of files matching the given glob pattern
pub fn create_glob_function(path_mapper: Box<dyn crate::stdlib::PathMapper>) -> Box<dyn Function> {
    use crate::stdlib::create_static_function;

    create_static_function(
        "glob".to_string(),
        vec![Type::string(false)],
        Type::array(Type::file(false), false, false),
        move |args| {
            let pattern = args[0].as_string().ok_or_else(|| WdlError::RuntimeError {
                message: "glob() requires a string pattern".to_string(),
            })?;

            // Resolve relative globs against the task work directory when available
            let resolved_pattern = {
                let pattern_path = std::path::Path::new(pattern);
                if pattern_path.is_absolute() {
                    pattern.to_string()
                } else if let Some(task_mapper) = path_mapper
                    .as_any()
                    .downcast_ref::<crate::stdlib::TaskPathMapper>()
                {
                    task_mapper
                        .task_dir()
                        .join("work")
                        .join(pattern_path)
                        .to_string_lossy()
                        .to_string()
                } else {
                    std::env::current_dir()
                        .map(|cwd| cwd.join(pattern_path))
                        .map_err(|e| WdlError::RuntimeError {
                            message: format!("Failed to resolve glob pattern: {}", e),
                        })?
                        .to_string_lossy()
                        .to_string()
                }
            };

            // Use glob crate to find matching files
            let glob_result =
                glob::glob(&resolved_pattern).map_err(|e| WdlError::RuntimeError {
                    message: format!("Invalid glob pattern '{}': {}", resolved_pattern, e),
                })?;

            let mut files = Vec::new();
            for entry in glob_result {
                match entry {
                    Ok(path) => {
                        if path.is_dir() {
                            // Skip directories per WDL spec
                            continue;
                        }
                        // Convert path to string and virtualize through PathMapper
                        let path_str = path.to_string_lossy().to_string();
                        match path_mapper.virtualize_filename(&path) {
                            Ok(virtual_path) => {
                                files.push(Value::file(virtual_path)?);
                            }
                            Err(_) => {
                                // If virtualization fails, use the original path
                                files.push(Value::file(path_str)?);
                            }
                        }
                    }
                    Err(e) => {
                        return Err(WdlError::RuntimeError {
                            message: format!("Glob error: {}", e),
                        });
                    }
                }
            }

            Ok(Value::array(Type::file(false), files))
        },
    )
}

/// Create size function with multiple signatures:
/// - size(file: File?) -> Float
/// - size(file: File?, unit: String) -> Float
/// - size(files: Array[File?]) -> Float
/// - size(files: Array[File?], unit: String) -> Float
///
/// Returns the size of file(s) in bytes or specified unit
pub fn create_size_function(path_mapper: Box<dyn crate::stdlib::PathMapper>) -> Box<dyn Function> {
    use crate::env::Bindings;
    use crate::expr::Expression;
    use crate::stdlib::Function;

    struct SizeFunction {
        path_mapper: Box<dyn crate::stdlib::PathMapper>,
    }

    impl SizeFunction {
        fn type_may_contain_paths(ty: &Type) -> bool {
            match ty {
                Type::File { .. } | Type::Directory { .. } => true,
                Type::Array { item_type, .. } => Self::type_may_contain_paths(item_type),
                Type::Pair {
                    left_type,
                    right_type,
                    ..
                } => {
                    Self::type_may_contain_paths(left_type)
                        || Self::type_may_contain_paths(right_type)
                }
                Type::Map {
                    key_type,
                    value_type,
                    ..
                } => {
                    Self::type_may_contain_paths(key_type)
                        || Self::type_may_contain_paths(value_type)
                }
                Type::StructInstance { members, .. } => members
                    .as_ref()
                    .map_or(true, |m| m.values().any(Self::type_may_contain_paths)),
                Type::Object { members, .. } => {
                    if members.is_empty() {
                        true
                    } else {
                        members.values().any(Self::type_may_contain_paths)
                    }
                }
                Type::Any { .. } => true,
                _ => false,
            }
        }

        fn path_size(&self, virtual_path: &str) -> Result<f64, WdlError> {
            let real_path = self
                .path_mapper
                .devirtualize_filename(virtual_path)
                .map_err(|e| WdlError::RuntimeError {
                    message: format!("Cannot resolve path '{}': {}", virtual_path, e),
                })?;

            let metadata = fs::metadata(&real_path).map_err(|e| WdlError::RuntimeError {
                message: format!("Cannot get metadata for '{}': {}", virtual_path, e),
            })?;

            if metadata.is_dir() {
                let bytes = directory_size(&real_path).map_err(|e| WdlError::RuntimeError {
                    message: format!("Cannot get size of directory '{}': {}", virtual_path, e),
                })?;
                Ok(bytes as f64)
            } else {
                Ok(metadata.len() as f64)
            }
        }

        fn value_size(&self, value: &Value) -> Result<f64, WdlError> {
            match value {
                Value::Null
                | Value::Boolean { .. }
                | Value::Int { .. }
                | Value::Float { .. }
                | Value::String { .. } => Ok(0.0),
                Value::File { value, .. } | Value::Directory { value, .. } => self.path_size(value),
                Value::Array { values, .. } => {
                    let mut total = 0.0;
                    for element in values {
                        total += self.value_size(element)?;
                    }
                    Ok(total)
                }
                Value::Map { pairs, .. } => {
                    let mut total = 0.0;
                    for (key, val) in pairs {
                        total += self.value_size(key)?;
                        total += self.value_size(val)?;
                    }
                    Ok(total)
                }
                Value::Pair { left, right, .. } => {
                    Ok(self.value_size(left)? + self.value_size(right)?)
                }
                Value::Struct { members, .. } => {
                    let mut total = 0.0;
                    for member_value in members.values() {
                        total += self.value_size(member_value)?;
                    }
                    Ok(total)
                }
            }
        }
    }

    impl Function for SizeFunction {
        fn name(&self) -> &str {
            "size"
        }

        fn infer_type(
            &self,
            args: &mut [Expression],
            type_env: &Bindings<Type>,
            stdlib: &crate::stdlib::StdLib,
            struct_typedefs: &[crate::tree::StructTypeDef],
        ) -> Result<Type, WdlError> {
            if args.is_empty() || args.len() > 2 {
                return Err(WdlError::Validation {
                    pos: if args.is_empty() {
                        SourcePosition::new(
                            "unknown".to_string(),
                            "unknown".to_string(),
                            0,
                            0,
                            0,
                            0,
                        )
                    } else {
                        args[0].source_position().clone()
                    },
                    message: format!("size() expects 1 or 2 arguments, got {}", args.len()),
                    source_text: None,
                    declared_wdl_version: None,
                });
            }

            // Infer the first argument type and ensure it can contain paths
            let first_arg_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
            if !Self::type_may_contain_paths(&first_arg_type) {
                return Err(WdlError::Validation {
                    pos: args[0].source_position().clone(),
                    message: format!(
                        "size() first argument must be or contain File/Directory types, got {}",
                        first_arg_type
                    ),
                    source_text: None,
                    declared_wdl_version: None,
                });
            }

            // Check the optional second argument (unit)
            if args.len() > 1 {
                let unit_type = args[1].infer_type(type_env, stdlib, struct_typedefs)?;
                if !matches!(unit_type, Type::String { .. }) {
                    return Err(WdlError::Validation {
                        pos: args[1].source_position().clone(),
                        message: format!("size() unit argument must be String, got {}", unit_type),
                        source_text: None,
                        declared_wdl_version: None,
                    });
                }
            }

            Ok(Type::float(false))
        }

        fn eval(
            &self,
            args: &[Expression],
            env: &Bindings<Value>,
            stdlib: &crate::stdlib::StdLib,
        ) -> Result<Value, WdlError> {
            if args.is_empty() || args.len() > 2 {
                return Err(WdlError::RuntimeError {
                    message: format!("size() expects 1 or 2 arguments, got {}", args.len()),
                });
            }

            // Define byte unit conversion factors
            let get_unit_factor = |unit: &str| -> Result<f64, WdlError> {
                let normalized = unit.trim();
                if normalized.is_empty() {
                    return Ok(1.0);
                }

                let upper = normalized.to_uppercase();
                match upper.as_str() {
                    "B" | "BYTE" | "BYTES" => Ok(1.0),
                    "K" | "KB" | "KILOBYTE" | "KILOBYTES" => Ok(1_000.0),
                    "M" | "MB" | "MEGABYTE" | "MEGABYTES" => Ok(1_000_000.0),
                    "G" | "GB" | "GIGABYTE" | "GIGABYTES" => Ok(1_000_000_000.0),
                    "T" | "TB" | "TERABYTE" | "TERABYTES" => Ok(1_000_000_000_000.0),
                    "KI" | "KIB" | "KIBIBYTE" | "KIBIBYTES" => Ok(1024.0),
                    "MI" | "MIB" | "MEBIBYTE" | "MEBIBYTES" => Ok(1024.0 * 1024.0),
                    "GI" | "GIB" | "GIBIBYTE" | "GIBIBYTES" => Ok(1024.0 * 1024.0 * 1024.0),
                    "TI" | "TIB" | "TEBIBYTE" | "TEBIBYTES" => {
                        Ok(1024.0 * 1024.0 * 1024.0 * 1024.0)
                    }
                    _ => Err(WdlError::RuntimeError {
                        message: format!("Unknown size unit: {}", unit),
                    }),
                }
            };

            // Evaluate arguments
            let first_arg = args[0].eval(env, stdlib)?;

            // Get unit factor if provided
            let unit_factor = if args.len() > 1 {
                let unit_arg = args[1].eval(env, stdlib)?;
                let unit = unit_arg.as_string().ok_or_else(|| WdlError::RuntimeError {
                    message: "size() unit argument must be a string".to_string(),
                })?;
                get_unit_factor(unit)?
            } else {
                1.0 // Default to bytes
            };

            let total_size = self.value_size(&first_arg)?;

            Ok(Value::float(total_size / unit_factor))
        }
    }

    Box::new(SizeFunction { path_mapper })
}

fn directory_size(path: &Path) -> std::io::Result<u64> {
    let mut total = 0u64;
    for entry in fs::read_dir(path)? {
        let entry = entry?;
        let metadata = entry.metadata()?;
        let file_type = metadata.file_type();

        if file_type.is_symlink() {
            continue;
        }

        if file_type.is_dir() {
            total += directory_size(&entry.path())?;
        } else if file_type.is_file() {
            total += metadata.len();
        }
    }

    Ok(total)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::env::Bindings;
    use crate::error::{SourcePosition, WdlError};
    use crate::expr::{Expression, StringPart};
    use crate::stdlib::{DefaultPathMapper, StdLib, TaskPathMapper};
    use std::fs;
    use std::io::Write;
    use tempfile::{NamedTempFile, TempDir};

    #[test]
    fn test_read_string_valid() {
        let mut temp_file = NamedTempFile::new().unwrap();
        writeln!(temp_file, "hello world").unwrap();
        let temp_path = temp_file.path().to_str().unwrap();

        let path_mapper = Box::new(DefaultPathMapper);
        let read_string_fn = create_read_string_function(path_mapper);

        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let file_expr = Expression::string(pos, vec![StringPart::Text(temp_path.to_string())]);
        let args = vec![file_expr];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        let result = read_string_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_string().unwrap(), "hello world");
    }

    #[test]
    fn test_read_int_valid() {
        let mut temp_file = NamedTempFile::new().unwrap();
        writeln!(temp_file, "42").unwrap();
        let temp_path = temp_file.path().to_str().unwrap();

        let path_mapper = Box::new(DefaultPathMapper);
        let read_int_fn = create_read_int_function(path_mapper);

        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let file_expr = Expression::string(pos, vec![StringPart::Text(temp_path.to_string())]);
        let args = vec![file_expr];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        let result = read_int_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 42);
    }

    #[test]
    #[allow(clippy::approx_constant)]
    fn test_read_float_valid() {
        let mut temp_file = NamedTempFile::new().unwrap();
        writeln!(temp_file, "3.14").unwrap();
        let temp_path = temp_file.path().to_str().unwrap();

        let path_mapper = Box::new(DefaultPathMapper);
        let read_float_fn = create_read_float_function(path_mapper);

        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let file_expr = Expression::string(pos, vec![StringPart::Text(temp_path.to_string())]);
        let args = vec![file_expr];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        let result = read_float_fn.eval(&args, &env, &stdlib).unwrap();
        assert!((result.as_float().unwrap() - 3.14).abs() < f64::EPSILON);
    }

    #[test]
    fn test_read_boolean_valid() {
        let mut temp_file = NamedTempFile::new().unwrap();
        writeln!(temp_file, "true").unwrap();
        let temp_path = temp_file.path().to_str().unwrap();

        let path_mapper = Box::new(DefaultPathMapper);
        let read_boolean_fn = create_read_boolean_function(path_mapper);

        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let file_expr = Expression::string(pos, vec![StringPart::Text(temp_path.to_string())]);
        let args = vec![file_expr];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        let result = read_boolean_fn.eval(&args, &env, &stdlib).unwrap();
        assert!(result.as_bool().unwrap());
    }

    #[test]
    fn test_read_object_basic() {
        let mut temp_file = NamedTempFile::new().unwrap();
        writeln!(temp_file, "key_0\tkey_1").unwrap();
        write!(temp_file, "value_0\tvalue_1").unwrap();
        let temp_path = temp_file.path().to_str().unwrap();

        let path_mapper = Box::new(DefaultPathMapper);
        let read_object_fn = create_read_object_function(path_mapper);

        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let file_expr = Expression::string(pos, vec![StringPart::Text(temp_path.to_string())]);
        let args = vec![file_expr];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        let result = read_object_fn.eval(&args, &env, &stdlib).unwrap();
        let members = result.as_struct().unwrap();
        assert_eq!(members.len(), 2);
        assert_eq!(
            members.get("key_0").unwrap().as_string().unwrap(),
            "value_0"
        );
        assert_eq!(
            members.get("key_1").unwrap().as_string().unwrap(),
            "value_1"
        );
    }

    #[test]
    fn test_read_objects_basic() {
        let mut temp_file = NamedTempFile::new().unwrap();
        writeln!(temp_file, "key_0\tkey_1").unwrap();
        writeln!(temp_file, "value_A0\tvalue_A1").unwrap();
        writeln!(temp_file, "value_B0\tvalue_B1").unwrap();
        write!(temp_file, "value_C0\tvalue_C1").unwrap();
        let temp_path = temp_file.path().to_str().unwrap();

        let path_mapper = Box::new(DefaultPathMapper);
        let read_objects_fn = create_read_objects_function(path_mapper);

        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let file_expr = Expression::string(pos, vec![StringPart::Text(temp_path.to_string())]);
        let args = vec![file_expr];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        let result = read_objects_fn.eval(&args, &env, &stdlib).unwrap();
        let array = result.as_array().unwrap();
        assert_eq!(array.len(), 3);

        let first = array[0].as_struct().unwrap();
        assert_eq!(first.get("key_0").unwrap().as_string().unwrap(), "value_A0");
        assert_eq!(first.get("key_1").unwrap().as_string().unwrap(), "value_A1");

        let third = array[2].as_struct().unwrap();
        assert_eq!(third.get("key_0").unwrap().as_string().unwrap(), "value_C0");
        assert_eq!(third.get("key_1").unwrap().as_string().unwrap(), "value_C1");
    }

    #[test]
    fn test_write_objects_basic() {
        let temp_dir = TempDir::new().unwrap();
        let path_mapper = Box::new(DefaultPathMapper);
        let write_objects_fn = create_write_objects_function(
            path_mapper,
            temp_dir.path().to_string_lossy().to_string(),
        );

        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 1);

        let obj_exprs = vec![
            Expression::struct_expr(
                pos.clone(),
                vec![
                    (
                        "key_1".to_string(),
                        Expression::string_literal(pos.clone(), "value_1".to_string()),
                    ),
                    (
                        "key_2".to_string(),
                        Expression::string_literal(pos.clone(), "value_2".to_string()),
                    ),
                    (
                        "key_3".to_string(),
                        Expression::string_literal(pos.clone(), "value_3".to_string()),
                    ),
                ],
            ),
            Expression::struct_expr(
                pos.clone(),
                vec![
                    (
                        "key_1".to_string(),
                        Expression::string_literal(pos.clone(), "value_4".to_string()),
                    ),
                    (
                        "key_2".to_string(),
                        Expression::string_literal(pos.clone(), "value_5".to_string()),
                    ),
                    (
                        "key_3".to_string(),
                        Expression::string_literal(pos.clone(), "value_6".to_string()),
                    ),
                ],
            ),
            Expression::struct_expr(
                pos.clone(),
                vec![
                    (
                        "key_1".to_string(),
                        Expression::string_literal(pos.clone(), "value_7".to_string()),
                    ),
                    (
                        "key_2".to_string(),
                        Expression::string_literal(pos.clone(), "value_8".to_string()),
                    ),
                    (
                        "key_3".to_string(),
                        Expression::string_literal(pos.clone(), "value_9".to_string()),
                    ),
                ],
            ),
        ];

        let array_expr = Expression::array(pos.clone(), obj_exprs);
        let args = vec![array_expr];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        let result = write_objects_fn.eval(&args, &env, &stdlib).unwrap();
        let filepath = result.as_string().unwrap();
        let content = fs::read_to_string(filepath).unwrap();

        assert_eq!(
            content,
            "key_1\tkey_2\tkey_3\nvalue_1\tvalue_2\tvalue_3\nvalue_4\tvalue_5\tvalue_6\nvalue_7\tvalue_8\tvalue_9\n"
        );
    }

    #[test]
    fn test_write_objects_mismatched_members() {
        let temp_dir = TempDir::new().unwrap();
        let path_mapper = Box::new(DefaultPathMapper);
        let write_objects_fn = create_write_objects_function(
            path_mapper,
            temp_dir.path().to_string_lossy().to_string(),
        );

        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 1);

        let array_expr = Expression::array(
            pos.clone(),
            vec![
                Expression::struct_expr(
                    pos.clone(),
                    vec![
                        (
                            "key_1".to_string(),
                            Expression::string_literal(pos.clone(), "value_1".to_string()),
                        ),
                        (
                            "key_2".to_string(),
                            Expression::string_literal(pos.clone(), "value_2".to_string()),
                        ),
                    ],
                ),
                Expression::struct_expr(
                    pos.clone(),
                    vec![
                        (
                            "key_1".to_string(),
                            Expression::string_literal(pos.clone(), "value_3".to_string()),
                        ),
                        (
                            "key_3".to_string(),
                            Expression::string_literal(pos.clone(), "value_4".to_string()),
                        ),
                    ],
                ),
            ],
        );

        let args = vec![array_expr];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        let err = write_objects_fn.eval(&args, &env, &stdlib).unwrap_err();
        match err {
            WdlError::RuntimeError { message } => {
                assert!(message.contains("array elements must have the same member names"));
            }
            other => panic!("Expected RuntimeError, got {:?}", other),
        }
    }

    #[test]
    fn test_write_objects_empty_array() {
        let temp_dir = TempDir::new().unwrap();
        let path_mapper = Box::new(DefaultPathMapper);
        let write_objects_fn = create_write_objects_function(
            path_mapper,
            temp_dir.path().to_string_lossy().to_string(),
        );

        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 1);
        let array_expr = Expression::array(pos.clone(), vec![]);
        let args = vec![array_expr];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        let result = write_objects_fn.eval(&args, &env, &stdlib).unwrap();
        let filepath = result.as_string().unwrap();
        let metadata = fs::metadata(filepath).unwrap();
        assert_eq!(metadata.len(), 0);
    }

    #[test]
    fn test_glob_ignores_directories() {
        let task_dir = TempDir::new().unwrap();
        let work_dir = task_dir.path().join("work");
        fs::create_dir_all(&work_dir).unwrap();
        fs::create_dir(work_dir.join("a_dir")).unwrap();
        fs::write(work_dir.join("a_dir").join("a_inner.txt"), "inner").unwrap();
        fs::write(work_dir.join("a_file_1.txt"), "one").unwrap();
        fs::write(work_dir.join("a_file_2.txt"), "two").unwrap();

        let path_mapper = Box::new(TaskPathMapper::new(task_dir.path().to_path_buf()));
        let glob_fn = create_glob_function(path_mapper);

        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 1);
        let pattern_expr = Expression::string_literal(pos.clone(), "a_*".to_string());
        let args = vec![pattern_expr];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        let result = glob_fn.eval(&args, &env, &stdlib).unwrap();
        let files = result.as_array().unwrap();
        assert_eq!(files.len(), 2);
        let names: Vec<&str> = files
            .iter()
            .map(|value| value.as_string().unwrap())
            .collect();
        assert_eq!(names, vec!["a_file_1.txt", "a_file_2.txt"]);
    }
}
