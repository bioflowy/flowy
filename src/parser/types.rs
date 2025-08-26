//! Token-based type parsing for WDL

use super::parser_utils::{parse_separated, ParseResult};
use super::token_stream::TokenStream;
use super::tokens::Token;
use crate::error::WdlError;
use crate::types::Type;

/// Parse a primitive type
pub fn parse_primitive_type(stream: &mut TokenStream) -> ParseResult<Type> {
    match stream.peek_token() {
        Some(Token::Keyword(kw)) => {
            let type_name = kw.clone();
            match type_name.as_str() {
                "String" => {
                    stream.next();
                    Ok(Type::string(false))
                }
                "Int" => {
                    stream.next();
                    Ok(Type::int(false))
                }
                "Float" => {
                    stream.next();
                    Ok(Type::float(false))
                }
                "Boolean" => {
                    stream.next();
                    Ok(Type::boolean(false))
                }
                "File" => {
                    stream.next();
                    Ok(Type::file(false))
                }
                "Directory" => {
                    stream.next();
                    Ok(Type::directory(false))
                }
                "None" => {
                    stream.next();
                    Ok(Type::none())
                }
                _ => Err(WdlError::syntax_error(
                    stream.current_position(),
                    format!("Unknown primitive type: {}", type_name),
                    "1.0".to_string(),
                    None,
                )),
            }
        }
        _ => Err(WdlError::syntax_error(
            stream.current_position(),
            "Expected type name".to_string(),
            "1.0".to_string(),
            None,
        )),
    }
}

/// Parse an array type like Array[String] or Array[Array[Int]]
pub fn parse_array_type(stream: &mut TokenStream) -> ParseResult<Type> {
    // Expect "Array" keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "Array" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected 'Array' keyword".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    }

    stream.expect(Token::LeftBracket)?;
    let inner_type = parse_type(stream)?;
    stream.expect(Token::RightBracket)?;

    // Check for non-empty array marker (+)
    let nonempty = if stream.peek_token() == Some(Token::Plus) {
        stream.next();
        true
    } else {
        false
    };

    Ok(Type::array(inner_type, false, nonempty))
}

/// Parse a map type like Map[String, Int]
pub fn parse_map_type(stream: &mut TokenStream) -> ParseResult<Type> {
    // Expect "Map" keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "Map" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected 'Map' keyword".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    }

    stream.expect(Token::LeftBracket)?;
    let key_type = parse_type(stream)?;
    stream.expect(Token::Comma)?;
    let value_type = parse_type(stream)?;
    stream.expect(Token::RightBracket)?;

    Ok(Type::map(key_type, value_type, false))
}

/// Parse a pair type like Pair[String, Int]
pub fn parse_pair_type(stream: &mut TokenStream) -> ParseResult<Type> {
    // Expect "Pair" keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "Pair" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected 'Pair' keyword".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    }

    stream.expect(Token::LeftBracket)?;
    let left_type = parse_type(stream)?;
    stream.expect(Token::Comma)?;
    let right_type = parse_type(stream)?;
    stream.expect(Token::RightBracket)?;

    Ok(Type::pair(left_type, right_type, false))
}

/// Parse an object type
pub fn parse_object_type(stream: &mut TokenStream) -> ParseResult<Type> {
    // Expect "Object" keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "Object" => {
            stream.next();
            use std::collections::HashMap;
            Ok(Type::object(HashMap::new()))
        }
        _ => Err(WdlError::syntax_error(
            stream.current_position(),
            "Expected 'Object' keyword".to_string(),
            "1.0".to_string(),
            None,
        )),
    }
}

/// Parse a struct type (e.g., MyStruct)
pub fn parse_struct_type(stream: &mut TokenStream) -> ParseResult<Type> {
    // Struct types start with an uppercase identifier
    match stream.peek_token() {
        Some(Token::Identifier(name)) => {
            // Check if it starts with uppercase
            if name.chars().next().map_or(false, |c| c.is_uppercase()) {
                let type_name = name.clone(); // We need to clone here since we'll consume the token
                stream.next();
                Ok(Type::struct_instance(type_name, false))
            } else {
                let pos = stream.current_position();
                Err(WdlError::syntax_error(
                    pos,
                    format!("Struct type names must start with uppercase: {}", name),
                    "1.0".to_string(),
                    None,
                ))
            }
        }
        _ => Err(WdlError::syntax_error(
            stream.current_position(),
            "Expected struct type name".to_string(),
            "1.0".to_string(),
            None,
        )),
    }
}

/// Parse a WDL type
pub fn parse_type(stream: &mut TokenStream) -> ParseResult<Type> {
    // Try to parse the base type
    let base_type = match stream.peek_token() {
        Some(Token::Keyword(kw)) => {
            match kw.as_str() {
                "Array" => parse_array_type(stream)?,
                "Map" => parse_map_type(stream)?,
                "Pair" => parse_pair_type(stream)?,
                "Object" => parse_object_type(stream)?,
                "String" | "Int" | "Float" | "Boolean" | "File" | "Directory" | "None" => {
                    parse_primitive_type(stream)?
                }
                _ => {
                    // Could be a struct type that happens to be a keyword
                    parse_struct_type(stream)?
                }
            }
        }
        Some(Token::Identifier(_)) => parse_struct_type(stream)?,
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected type".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    };

    // Check for optional suffix (?)
    if stream.peek_token() == Some(Token::Question) {
        stream.next();

        // Set optional flag on the type
        let optional_type = match base_type {
            Type::Boolean { .. } => Type::boolean(true),
            Type::Int { .. } => Type::int(true),
            Type::Float { .. } => Type::float(true),
            Type::String { .. } => Type::string(true),
            Type::File { .. } => Type::file(true),
            Type::Directory { .. } => Type::directory(true),
            Type::Array {
                item_type,
                nonempty,
                ..
            } => Type::Array {
                item_type,
                optional: true,
                nonempty,
            },
            Type::Map {
                key_type,
                value_type,
                literal_keys,
                ..
            } => Type::Map {
                key_type,
                value_type,
                optional: true,
                literal_keys,
            },
            Type::Pair {
                left_type,
                right_type,
                ..
            } => Type::Pair {
                left_type,
                right_type,
                optional: true,
            },
            Type::StructInstance {
                type_name, members, ..
            } => Type::StructInstance {
                type_name,
                members,
                optional: true,
            },
            _ => base_type,
        };

        Ok(optional_type)
    } else {
        Ok(base_type)
    }
}

/// Parse a list of types (for function signatures, etc.)
pub fn parse_type_list(stream: &mut TokenStream) -> ParseResult<Vec<Type>> {
    parse_separated(stream, Token::Comma, parse_type)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::parser::token_stream::TokenStream;

    #[test]
    fn test_parse_primitive_types() {
        let test_cases = vec![
            ("String", "String"),
            ("Int", "Int"),
            ("Float", "Float"),
            ("Boolean", "Boolean"),
            ("File", "File"),
        ];

        for (input_str, expected_name) in test_cases {
            let mut stream = TokenStream::new(input_str, "1.0").unwrap();
            let result = parse_primitive_type(&mut stream);
            assert!(result.is_ok(), "Failed to parse {}", input_str);
            let type_ = result.unwrap();
            assert_eq!(type_.to_string(), expected_name);
        }
    }

    #[test]
    fn test_parse_array_type() {
        let mut stream = TokenStream::new("Array[String]", "1.0").unwrap();
        let result = parse_array_type(&mut stream);
        assert!(result.is_ok());
        let type_ = result.unwrap();
        assert_eq!(type_.to_string(), "Array[String]");

        // Nested array
        let mut stream = TokenStream::new("Array[Array[Int]]", "1.0").unwrap();
        let result = parse_type(&mut stream);
        assert!(result.is_ok());
        let type_ = result.unwrap();
        assert_eq!(type_.to_string(), "Array[Array[Int]]");
    }

    #[test]
    fn test_parse_map_type() {
        let mut stream = TokenStream::new("Map[String, Int]", "1.0").unwrap();
        let result = parse_map_type(&mut stream);
        assert!(result.is_ok());
        let type_ = result.unwrap();
        assert_eq!(type_.to_string(), "Map[String,Int]");

        // Without spaces
        let mut stream = TokenStream::new("Map[String,Int]", "1.0").unwrap();
        let result = parse_map_type(&mut stream);
        assert!(result.is_ok());
    }

    #[test]
    fn test_parse_pair_type() {
        let mut stream = TokenStream::new("Pair[String, Float]", "1.0").unwrap();
        let result = parse_pair_type(&mut stream);
        assert!(result.is_ok());
        let type_ = result.unwrap();
        assert_eq!(type_.to_string(), "Pair[String,Float]");
    }

    #[test]
    fn test_parse_optional_type() {
        let mut stream = TokenStream::new("String?", "1.0").unwrap();
        let result = parse_type(&mut stream);
        assert!(result.is_ok());
        let type_ = result.unwrap();
        assert_eq!(type_.to_string(), "String?");

        // Optional array
        let mut stream = TokenStream::new("Array[Int]?", "1.0").unwrap();
        let result = parse_type(&mut stream);
        assert!(result.is_ok());
        let type_ = result.unwrap();
        assert_eq!(type_.to_string(), "Array[Int]?");
    }

    #[test]
    fn test_parse_complex_types() {
        // Map with array values
        let mut stream = TokenStream::new("Map[String, Array[File]]", "1.0").unwrap();
        let result = parse_type(&mut stream);
        assert!(result.is_ok());
        let type_ = result.unwrap();
        assert_eq!(type_.to_string(), "Map[String,Array[File]]");

        // Optional map with optional values
        let mut stream = TokenStream::new("Map[String, Int?]?", "1.0").unwrap();
        let result = parse_type(&mut stream);
        assert!(result.is_ok());
        let type_ = result.unwrap();
        assert_eq!(type_.to_string(), "Map[String,Int?]?");
    }

    #[test]
    fn test_parse_struct_type() {
        let mut stream = TokenStream::new("MyStruct", "1.0").unwrap();
        let result = parse_struct_type(&mut stream);
        assert!(result.is_ok());
        let type_ = result.unwrap();
        assert_eq!(type_.to_string(), "MyStruct");

        // Optional struct
        let mut stream = TokenStream::new("MyStruct?", "1.0").unwrap();
        let result = parse_type(&mut stream);
        assert!(result.is_ok());
        let type_ = result.unwrap();
        assert_eq!(type_.to_string(), "MyStruct?");
    }

    #[test]
    fn test_parse_type_list() {
        let mut stream = TokenStream::new("String, Int, Array[File]", "1.0").unwrap();
        let result = parse_type_list(&mut stream);
        assert!(result.is_ok());
        let types = result.unwrap();
        assert_eq!(types.len(), 3);
        assert_eq!(types[0].to_string(), "String");
        assert_eq!(types[1].to_string(), "Int");
        assert_eq!(types[2].to_string(), "Array[File]");
    }
}
