//! TSV helper functions for the WDL standard library
//!
//! This module contains implementations for `write_tsv` and `read_tsv`,
//! including the full WDL 1.2 behaviour for optional headers and object
//! materialisation.

use crate::env::Bindings;
use crate::error::WdlError;
use crate::expr::{Expression, ExpressionBase};
use crate::parser::keywords::is_valid_identifier;
use crate::stdlib::{Function, PathMapper, StdLib};
use crate::types::Type;
use crate::value::{Value, ValueBase};
use std::collections::HashMap;
use std::fs;
use std::io::Write;
use std::os::unix::fs::PermissionsExt;

/// Create write_tsv function:
///   * write_tsv(Array[Array[String]])
///   * write_tsv(Array[Array[String]], true, Array[String])
///   * write_tsv(Array[Struct], [Boolean, [Array[String]]])
pub fn create_write_tsv_function(
    path_mapper: Box<dyn PathMapper>,
    write_dir: String,
) -> Box<dyn Function> {
    Box::new(WriteTsvFunction {
        path_mapper,
        write_dir,
    })
}

/// Create read_tsv function with the overloads defined by the WDL 1.2 spec
pub fn create_read_tsv_function(path_mapper: Box<dyn PathMapper>) -> Box<dyn Function> {
    Box::new(ReadTsvFunction { path_mapper })
}

struct WriteTsvFunction {
    path_mapper: Box<dyn PathMapper>,
    write_dir: String,
}

impl Function for WriteTsvFunction {
    fn name(&self) -> &str {
        "write_tsv"
    }

    fn infer_type(
        &self,
        args: &mut [Expression],
        type_env: &Bindings<Type>,
        stdlib: &StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        if args.is_empty() {
            return Err(WdlError::RuntimeError {
                message: "write_tsv(): expected at least 1 argument".to_string(),
            });
        }

        let first_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        match &first_type {
            Type::Array { item_type, .. } => match item_type.as_ref() {
                Type::Array { .. } => match args.len() {
                    1 => Ok(Type::file(false)),
                    3 => {
                        let flag_type = args[1].infer_type(type_env, stdlib, struct_typedefs)?;
                        if !flag_type.coerces(&Type::boolean(false), true) {
                            return Err(WdlError::RuntimeError {
                                message: "write_tsv(): second argument must be true".to_string(),
                            });
                        }
                        let header_type = args[2].infer_type(type_env, stdlib, struct_typedefs)?;
                        if !matches!(header_type, Type::Array { .. }) {
                            return Err(WdlError::RuntimeError {
                                message: format!(
                                    "write_tsv(): header must be Array[String], got {}",
                                    header_type
                                ),
                            });
                        }
                        Ok(Type::file(false))
                    }
                    n => Err(WdlError::RuntimeError {
                        message: format!(
                            "write_tsv(): expected 1 or 3 arguments for Array[Array[String]] input, got {}",
                            n
                        ),
                    }),
                },
                Type::StructInstance { .. } => {
                    if args.len() > 1 {
                        let flag_type = args[1].infer_type(type_env, stdlib, struct_typedefs)?;
                        if !flag_type.coerces(&Type::boolean(false), true) {
                            return Err(WdlError::RuntimeError {
                                message: "write_tsv(): second argument must be Boolean".to_string(),
                            });
                        }
                    }
                    if args.len() == 3 {
                        let header_type = args[2].infer_type(type_env, stdlib, struct_typedefs)?;
                        if !matches!(header_type, Type::Array { .. }) {
                            return Err(WdlError::RuntimeError {
                                message: format!(
                                    "write_tsv(): header must be Array[String], got {}",
                                    header_type
                                ),
                            });
                        }
                    }
                    if args.len() > 3 {
                        return Err(WdlError::RuntimeError {
                            message: "write_tsv(): too many arguments".to_string(),
                        });
                    }
                    Ok(Type::file(false))
                }
                other => Err(WdlError::RuntimeError {
                    message: format!("write_tsv(): unsupported inner type {}", other),
                }),
            },
            _ => Err(WdlError::RuntimeError {
                message: format!("write_tsv(): expected Array value, got {}", first_type),
            }),
        }
    }

    fn eval(
        &self,
        args: &[Expression],
        env: &Bindings<Value>,
        stdlib: &StdLib,
    ) -> Result<Value, WdlError> {
        if args.is_empty() {
            return Err(WdlError::RuntimeError {
                message: "write_tsv(): expected at least 1 argument".to_string(),
            });
        }

        let data_value = args[0].eval(env, stdlib)?;
        let data_rows = data_value
            .as_array()
            .ok_or_else(|| WdlError::RuntimeError {
                message: "write_tsv(): first argument must be an array".to_string(),
            })?;

        let all_arrays = data_rows.iter().all(|row| row.as_array().is_some());
        let all_structs = data_rows
            .iter()
            .all(|row| matches!(row, Value::Struct { .. }));

        let (rows, header) = if all_arrays && (args.len() == 1 || args.len() == 3) {
            write_tsv_from_string_arrays(args, env, stdlib, &data_value)?
        } else if all_structs && (1..=3).contains(&args.len()) {
            write_tsv_from_structs(args, env, stdlib, &data_value)?
        } else {
            return Err(WdlError::RuntimeError {
                message: "write_tsv(): unsupported argument combination".to_string(),
            });
        };

        let target_dir = if let Some(task_dir) = stdlib.task_dir() {
            task_dir.join("work").join("write_")
        } else {
            std::path::PathBuf::from(&self.write_dir)
        };
        std::fs::create_dir_all(&target_dir).map_err(|e| WdlError::RuntimeError {
            message: format!(
                "Failed to create directory '{}': {}",
                target_dir.display(),
                e
            ),
        })?;

        let mut temp_file =
            tempfile::NamedTempFile::new_in(&target_dir).map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to create temporary file: {}", e),
            })?;

        if let Some(header) = header {
            write_tsv_row(&header, temp_file.as_file_mut())?;
        }

        for row in rows {
            write_tsv_row(&row, temp_file.as_file_mut())?;
        }

        temp_file.flush().map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to flush TSV file: {}", e),
        })?;

        let temp_path = temp_file.path().to_path_buf();
        temp_file
            .persist(&temp_path)
            .map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to persist temporary file: {}", e),
            })?;

        let mut perms = std::fs::metadata(&temp_path)
            .map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to get file metadata: {}", e),
            })?
            .permissions();
        perms.set_mode(0o660);
        std::fs::set_permissions(&temp_path, perms).map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to set file permissions: {}", e),
        })?;

        let virtual_name = self
            .path_mapper
            .virtualize_filename(&temp_path)
            .map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to virtualize filename: {}", e),
            })?;

        Value::file(virtual_name)
    }
}

struct ReadTsvFunction {
    path_mapper: Box<dyn PathMapper>,
}

impl Function for ReadTsvFunction {
    fn name(&self) -> &str {
        "read_tsv"
    }

    fn infer_type(
        &self,
        args: &mut [Expression],
        type_env: &Bindings<Type>,
        stdlib: &StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        if args.is_empty() {
            return Err(WdlError::RuntimeError {
                message: "read_tsv(): expected at least 1 argument".to_string(),
            });
        }
        if args.len() > 3 {
            return Err(WdlError::RuntimeError {
                message: "read_tsv(): expected 1 to 3 arguments".to_string(),
            });
        }

        let file_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        if !file_type.coerces(&Type::file(false), true) {
            return Err(WdlError::RuntimeError {
                message: format!("read_tsv(): expected File, got {}", file_type),
            });
        }

        if args.len() >= 2 {
            let bool_type = args[1].infer_type(type_env, stdlib, struct_typedefs)?;
            if !bool_type.coerces(&Type::boolean(false), true) {
                return Err(WdlError::RuntimeError {
                    message: "read_tsv(): second argument must be Boolean".to_string(),
                });
            }
        }

        if args.len() == 3 {
            let header_type = args[2].infer_type(type_env, stdlib, struct_typedefs)?;
            match header_type {
                Type::Array { item_type, .. }
                    if matches!(*item_type, Type::String { .. } | Type::Any { .. }) => {}
                _ => {
                    return Err(WdlError::RuntimeError {
                        message: format!(
                            "read_tsv(): header must be Array[String], got {}",
                            header_type
                        ),
                    });
                }
            }
        }

        let array_of_arrays =
            Type::array(Type::array(Type::string(false), false, false), false, false);
        let object_item_type = Type::object(HashMap::new());
        let array_of_objects = Type::array(object_item_type, false, false);

        match args.len() {
            1 => Ok(array_of_arrays),
            2 => {
                let literal = args[1].literal();
                if let Some(Value::Boolean { value, .. }) = literal {
                    if value {
                        Ok(array_of_objects)
                    } else {
                        Ok(array_of_arrays)
                    }
                } else {
                    Err(WdlError::RuntimeError {
                        message: "read_tsv(): second argument must be a Boolean literal when header names are not provided".to_string(),
                    })
                }
            }
            3 => Ok(array_of_objects),
            _ => unreachable!(),
        }
    }

    fn eval(
        &self,
        args: &[Expression],
        env: &Bindings<Value>,
        stdlib: &StdLib,
    ) -> Result<Value, WdlError> {
        if args.is_empty() || args.len() > 3 {
            return Err(WdlError::RuntimeError {
                message: "read_tsv(): expected 1 to 3 arguments".to_string(),
            });
        }

        let file_value = args[0].eval(env, stdlib)?;
        let virtual_filename = file_value
            .as_string()
            .ok_or_else(|| WdlError::RuntimeError {
                message: "read_tsv(): expected File argument".to_string(),
            })?;

        let real_path = self
            .path_mapper
            .devirtualize_filename(virtual_filename)
            .map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to resolve filename '{}': {}", virtual_filename, e),
            })?;

        let content = fs::read_to_string(&real_path).map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to read TSV file '{}': {}", real_path.display(), e),
        })?;

        let mut rows = parse_tsv_content(&content);

        let header_flag = if args.len() >= 2 {
            let flag_value = args[1].eval(env, stdlib)?;
            parse_bool_value(flag_value, "read_tsv(): second argument", false)?
        } else {
            false
        };

        let provided_header = if args.len() == 3 {
            let header_value = args[2].eval(env, stdlib)?;
            Some(parse_string_array_value(
                header_value,
                "read_tsv(): header",
            )?)
        } else {
            None
        };

        if provided_header.is_none() && !header_flag {
            return Ok(build_array_of_arrays(rows));
        }

        let (field_names, data_rows) = if let Some(names) = provided_header {
            validate_field_names(&names, stdlib.wdl_version())?;

            if header_flag {
                if rows.is_empty() {
                    return Err(WdlError::RuntimeError {
                        message: "read_tsv(): TSV file is empty but a header was requested"
                            .to_string(),
                    });
                }
                let header = rows.remove(0);
                if header.len() != names.len() {
                    return Err(WdlError::RuntimeError {
                        message: format!(
                            "read_tsv(): header override length {} does not match file header length {}",
                            names.len(),
                            header.len()
                        ),
                    });
                }
            }

            (names, rows)
        } else {
            if rows.is_empty() {
                return Err(WdlError::RuntimeError {
                    message: "read_tsv(): TSV file is empty so no header can be read".to_string(),
                });
            }
            let header = rows.remove(0);
            validate_field_names(&header, stdlib.wdl_version())?;
            (header, rows)
        };

        ensure_uniform_row_length(&field_names, &data_rows, "read_tsv")?;

        Ok(build_array_of_objects(field_names, data_rows))
    }
}

fn write_tsv_from_string_arrays(
    args: &[Expression],
    env: &Bindings<Value>,
    stdlib: &StdLib,
    data_value: &Value,
) -> Result<(Vec<Vec<String>>, Option<Vec<String>>), WdlError> {
    let rows = value_rows_from_array(data_value)?;

    match args.len() {
        1 => Ok((rows, None)),
        3 => {
            let flag_value = args[1].eval(env, stdlib)?;
            let header_value = args[2].eval(env, stdlib)?;
            let flag = parse_bool_value(flag_value, "write_tsv(): second argument", false)?;
            if !flag {
                return Err(WdlError::RuntimeError {
                    message: "write_tsv(): second argument must be true when header is provided"
                        .to_string(),
                });
            }
            let header = parse_string_array_value(header_value, "write_tsv(): header")?;
            ensure_header_matches_rows(&header, &rows)?;
            Ok((rows, Some(header)))
        }
        n => Err(WdlError::RuntimeError {
            message: format!(
                "write_tsv(): expected 1 or 3 arguments for Array[Array[String]] input, got {}",
                n
            ),
        }),
    }
}

fn write_tsv_from_structs(
    args: &[Expression],
    env: &Bindings<Value>,
    stdlib: &StdLib,
    data_value: &Value,
) -> Result<(Vec<Vec<String>>, Option<Vec<String>>), WdlError> {
    let (rows, inferred_header) = value_rows_from_structs(data_value)?;

    let flag = if args.len() > 1 {
        parse_bool_value(
            args[1].eval(env, stdlib)?,
            "write_tsv(): second argument",
            true,
        )?
    } else {
        true
    };

    if args.len() == 3 {
        let header_values = args[2].eval(env, stdlib)?;
        if !flag {
            return Err(WdlError::RuntimeError {
                message: "write_tsv(): header array provided but second argument was false"
                    .to_string(),
            });
        }
        let header = parse_string_array_value(header_values, "write_tsv(): header")?;
        ensure_header_matches_rows(&header, &rows)?;
        Ok((rows, Some(header)))
    } else if flag {
        let header = inferred_header.ok_or_else(|| WdlError::RuntimeError {
            message: "write_tsv(): cannot infer header from empty struct array".to_string(),
        })?;
        ensure_header_matches_rows(&header, &rows)?;
        Ok((rows, Some(header)))
    } else {
        Ok((rows, None))
    }
}

fn value_rows_from_array(value: &Value) -> Result<Vec<Vec<String>>, WdlError> {
    let rows = value.as_array().ok_or_else(|| WdlError::RuntimeError {
        message: "write_tsv(): expected Array[Array[String]]".to_string(),
    })?;

    let mut table_rows = Vec::with_capacity(rows.len());
    let mut row_width: Option<usize> = None;

    for row in rows {
        let cells = row.as_array().ok_or_else(|| WdlError::RuntimeError {
            message: "write_tsv(): expected Array[Array[String]]".to_string(),
        })?;

        let mut row_values = Vec::with_capacity(cells.len());
        for cell in cells {
            let cell_str = cell.as_string().ok_or_else(|| WdlError::RuntimeError {
                message: "write_tsv(): array elements must be strings".to_string(),
            })?;
            row_values.push(cell_str.to_string());
        }

        if let Some(width) = row_width {
            if width != row_values.len() {
                return Err(WdlError::RuntimeError {
                    message: "write_tsv(): rows must all be the same length".to_string(),
                });
            }
        } else {
            row_width = Some(row_values.len());
        }

        table_rows.push(row_values);
    }

    Ok(table_rows)
}

fn value_rows_from_structs(
    value: &Value,
) -> Result<(Vec<Vec<String>>, Option<Vec<String>>), WdlError> {
    let rows = value.as_array().ok_or_else(|| WdlError::RuntimeError {
        message: "write_tsv(): expected Array[Struct]".to_string(),
    })?;

    let mut inferred_header: Option<Vec<String>> = None;
    let mut table_rows = Vec::with_capacity(rows.len());

    for row in rows {
        if let Value::Struct {
            members, wdl_type, ..
        } = row
        {
            if inferred_header.is_none() {
                let header = match wdl_type {
                    Type::StructInstance {
                        members: Some(def), ..
                    } => def.keys().cloned().collect(),
                    _ => members.keys().cloned().collect(),
                };
                inferred_header = Some(header);
            }

            let header = inferred_header.as_ref().unwrap();
            let mut row_values = Vec::with_capacity(header.len());
            for key in header {
                if let Some(val) = members.get(key) {
                    row_values.push(val.as_string().unwrap_or_default().to_string());
                } else {
                    row_values.push(String::new());
                }
            }
            table_rows.push(row_values);
        } else {
            return Err(WdlError::RuntimeError {
                message: "write_tsv(): expected Array[Struct]".to_string(),
            });
        }
    }

    Ok((table_rows, inferred_header))
}

fn write_tsv_row(row: &[String], file: &mut dyn Write) -> Result<(), WdlError> {
    for value in row {
        if value.contains('\n') {
            return Err(WdlError::RuntimeError {
                message: "write_tsv(): values must not contain newline characters".to_string(),
            });
        }
    }

    let line = row.join("\t");
    file.write_all(line.as_bytes())
        .map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to write TSV row: {}", e),
        })?;
    file.write_all(b"\n").map_err(|e| WdlError::RuntimeError {
        message: format!("Failed to write TSV row: {}", e),
    })
}

fn parse_bool_value(value: Value, context: &str, default: bool) -> Result<bool, WdlError> {
    match value {
        Value::Boolean { value, .. } => Ok(value),
        Value::Null => Ok(default),
        other => Err(WdlError::RuntimeError {
            message: format!("{} must be Boolean, got {}", context, other),
        }),
    }
}

fn parse_string_array_value(value: Value, context: &str) -> Result<Vec<String>, WdlError> {
    let array = value.as_array().ok_or_else(|| WdlError::RuntimeError {
        message: format!("{} must be Array[String]", context),
    })?;

    array
        .iter()
        .map(|cell| {
            cell.as_string()
                .map(|s| s.to_string())
                .ok_or_else(|| WdlError::RuntimeError {
                    message: format!("{} must contain only strings", context),
                })
        })
        .collect()
}

fn ensure_header_matches_rows(header: &[String], rows: &[Vec<String>]) -> Result<(), WdlError> {
    if let Some(first_row) = rows.first() {
        if header.len() != first_row.len() {
            return Err(WdlError::RuntimeError {
                message: format!(
                    "write_tsv(): header length {} does not match row length {}",
                    header.len(),
                    first_row.len()
                ),
            });
        }
    }

    Ok(())
}

fn parse_tsv_content(content: &str) -> Vec<Vec<String>> {
    if content.is_empty() {
        return Vec::new();
    }

    content
        .lines()
        .map(|line| {
            line.split('\t')
                .map(|cell| cell.trim_end_matches('\r').to_string())
                .collect()
        })
        .collect()
}

fn build_array_of_arrays(rows: Vec<Vec<String>>) -> Value {
    let string_type = Type::string(false);
    let row_type = Type::array(string_type.clone(), false, false);
    let row_values: Vec<Value> = rows
        .into_iter()
        .map(|row| {
            let cells: Vec<Value> = row.into_iter().map(Value::string).collect();
            Value::array(string_type.clone(), cells)
        })
        .collect();

    Value::array(row_type, row_values)
}

fn build_array_of_objects(field_names: Vec<String>, rows: Vec<Vec<String>>) -> Value {
    let member_types: HashMap<String, Type> = field_names
        .iter()
        .map(|name| (name.clone(), Type::string(false)))
        .collect();
    let object_type = Type::object(member_types.clone());

    let values: Vec<Value> = rows
        .into_iter()
        .map(|row| {
            let members: HashMap<String, Value> = field_names
                .iter()
                .cloned()
                .zip(row.into_iter().map(Value::string))
                .collect();
            Value::struct_value_unchecked(object_type.clone(), members, None)
        })
        .collect();

    Value::array(object_type, values)
}

fn ensure_uniform_row_length(
    header: &[String],
    rows: &[Vec<String>],
    context: &str,
) -> Result<(), WdlError> {
    for (index, row) in rows.iter().enumerate() {
        if row.len() != header.len() {
            return Err(WdlError::RuntimeError {
                message: format!(
                    "{}: row {} has length {} but expected {}",
                    context,
                    index + 1,
                    row.len(),
                    header.len()
                ),
            });
        }
    }
    Ok(())
}

fn validate_field_names(names: &[String], wdl_version: &str) -> Result<(), WdlError> {
    for name in names {
        if !is_valid_identifier(name, wdl_version) {
            return Err(WdlError::RuntimeError {
                message: format!("read_tsv(): '{}' is not a valid object field name", name),
            });
        }
    }
    Ok(())
}
